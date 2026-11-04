// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interpreter

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/parser"
)

func addMapInsertBinding(t testing.TB, disp Dispatcher) {
	t.Helper()
	err := disp.Add(&functions.Overload{
		Operator: mapInsertFunction,
		Binary: func(targetMap, updateMap ref.Val) ref.Val {
			tm := targetMap.(traits.Mapper)
			um := updateMap.(traits.Mapper)
			it := um.Iterator()
			for it.HasNext() == types.True {
				k := it.Next()
				res := types.InsertMapKeyValue(tm, k, um.Get(k))
				if types.IsError(res) {
					return res
				}
				tm = res.(traits.Mapper)
			}
			return tm
		},
		Function: func(args ...ref.Val) ref.Val {
			return types.InsertMapKeyValue(args[0].(traits.Mapper), args[1], args[2])
		},
	})
	if err != nil {
		t.Fatalf("disp.Add(cel.@mapInsert) failed: %v", err)
	}
}

type iterableOnlyList struct {
	items []int64
}

func (l *iterableOnlyList) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return l.items, nil
}

func (l *iterableOnlyList) ConvertToType(typeVal ref.Type) ref.Val {
	if typeVal == types.TypeType {
		return types.ListType
	}
	return types.NewErr("type conversion error")
}

func (l *iterableOnlyList) Equal(other ref.Val) ref.Val {
	return types.False
}

func (l *iterableOnlyList) Type() ref.Type {
	return types.ListType
}

func (l *iterableOnlyList) Value() any {
	return l.items
}

func (l *iterableOnlyList) Iterator() traits.Iterator {
	return &iterableOnlyIter{items: l.items}
}

type iterableOnlyIter struct {
	items []int64
	idx   int
}

func (it *iterableOnlyIter) ConvertToNative(reflect.Type) (any, error) {
	return nil, nil
}

func (it *iterableOnlyIter) ConvertToType(ref.Type) ref.Val {
	return types.NewErr("type conversion error")
}

func (it *iterableOnlyIter) Equal(ref.Val) ref.Val {
	return types.False
}

func (it *iterableOnlyIter) Type() ref.Type {
	return types.IteratorType
}

func (it *iterableOnlyIter) Value() any {
	return it
}

func (it *iterableOnlyIter) HasNext() ref.Val {
	return types.Bool(it.idx < len(it.items))
}

func (it *iterableOnlyIter) Next() ref.Val {
	v := it.items[it.idx]
	it.idx++
	return types.Int(v)
}

func unwrapEvalFold(v InterpretableV2) *evalFold {
	for v != nil {
		if fold, ok := v.(*evalFold); ok {
			return fold
		}
		if obs, ok := v.(*ObservableInterpretable); ok {
			v = obs.InterpretableV2
			continue
		}
		if pi, ok := v.(*plannedInterpretable); ok {
			v = pi.InterpretableV2
			continue
		}
		if ed, ok := v.(*evalWatch); ok {
			v = ed.InterpretableV2
			continue
		}
		return nil
	}
	return nil
}

func TestComprehensionSlotPlanning(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)

	newInterp := func(attrs AttributeFactory) *exprInterpreter {
		return NewInterpreter(disp, cont, reg, reg, attrs).(*exprInterpreter)
	}

	t.Run("list_and_map_comprehensions_use_slots", func(t *testing.T) {
		attrs := NewAttributeFactory(cont, reg, reg)
		interp := newInterp(attrs)

		expressions := []string{
			`[1, 2, 3].map(x, x + 1)`,
			`[1, 2, 3].filter(x, x > 1)`,
			`compre2(i, v, a, [10, 20], [], true, a + [i + v], a)`,
			`compre2(i, v, a, [10, 20], [], true, i > 0 ? a + [v] : a, a)`,
			`compre2(k, v, a, {1: 2}, {}, true, insert(a, k, v + 1), a)`,
			`compre2(k, v, a, {1: 2}, {}, true, insert(a, {k: v + 1}), a)`,
			`compre2(k, v, a, {1: 2}, {}, true, k > 0 ? insert(a, {k: v + 1}) : a, a)`,
			`[1, 2, 3].all(x, x > 0)`,
		}

		for _, expr := range expressions {
			parsed := parseMutabilityExpr(t, expr)
			prg, err := interp.NewInterpretable(parsed)
			if err != nil {
				t.Fatalf("NewInterpretable(%q) failed: %v", expr, err)
			}
			pi, ok := prg.(*plannedInterpretable)
			if !ok {
				t.Fatalf("NewInterpretable(%q) got %T, wanted *plannedInterpretable", expr, prg)
			}
			fold, ok := pi.InterpretableV2.(*evalFold)
			if !ok {
				t.Fatalf("NewInterpretable(%q) inner got %T, wanted *evalFold", expr, pi.InterpretableV2)
			}
			if fold.accuSlot < 0 || pi.maxSlots <= 0 {
				t.Errorf("%q: fold accuSlot=%d, maxSlots=%d, want accuSlot >= 0 and maxSlots > 0", expr, fold.accuSlot, pi.maxSlots)
			}
			if fold.cond == nil || fold.step == nil || fold.result == nil {
				t.Errorf("%q: expected cond/step/result to be planned, got (%v, %v, %v)",
					expr, fold.cond, fold.step, fold.result)
			}
		}
	})

	t.Run("enabled_with_observers_cost_and_partial_eval", func(t *testing.T) {
		parsed := parseMutabilityExpr(t, `[1, 2, 3].map(x, x + 1)`)
		wantList := types.NewRefValList(reg, []ref.Val{types.Int(2), types.Int(3), types.Int(4)})

		// EvalStateObserver layers on top of slots via evalWatchAttr.
		obsInterp := newInterp(NewAttributeFactory(cont, reg, reg))
		obsPrg, err := obsInterp.NewInterpretable(parsed, EvalStateObserver())
		if err != nil {
			t.Fatalf("NewInterpretable with EvalStateObserver failed: %v", err)
		}
		if fold := unwrapEvalFold(obsPrg); fold == nil || fold.accuSlot < 0 || fold.iterSlot < 0 {
			t.Errorf("EvalStateObserver: fold = %+v, want non-nil with valid slots", fold)
		}
		if got := obsPrg.Eval(EmptyActivation()); got.Equal(wantList) != types.True {
			t.Errorf("EvalStateObserver eval got %v, want %v", got, wantList)
		}

		// MemoryObserver layers on top of slots, sampling inside fold scopes and enforcing limits.
		memPrg, err := obsInterp.NewInterpretable(parsed, MemoryObserver(MemoryTrackerFactory(func() (*types.MemoryTracker, error) {
			return types.NewMemoryTracker(types.MemoryTrackerLimit(1)), nil
		})))
		if err != nil {
			t.Fatalf("NewInterpretable with MemoryObserver failed: %v", err)
		}
		if fold := unwrapEvalFold(memPrg); fold == nil || fold.accuSlot < 0 || fold.iterSlot < 0 {
			t.Errorf("MemoryObserver: fold = %+v, want non-nil with valid slots", fold)
		}
		var memErr any
		func() {
			defer func() { memErr = recover() }()
			_ = memPrg.Eval(EmptyActivation())
		}()
		if cancelled, ok := memErr.(EvalCancelledError); !ok || cancelled.Cause != MemoryLimitExceeded {
			t.Errorf("MemoryObserver with Limit=1 panicked with %v (%T), wanted EvalCancelledError{Cause: MemoryLimitExceeded}", memErr, memErr)
		}

		// ExhaustiveEval keeps slots enabled while marking fold.exhaustive=true.
		exhPrg, err := obsInterp.NewInterpretable(parsed, ExhaustiveEval())
		if err != nil {
			t.Fatalf("NewInterpretable with ExhaustiveEval failed: %v", err)
		}
		if fold := unwrapEvalFold(exhPrg); fold == nil || fold.accuSlot < 0 || !fold.exhaustive {
			t.Errorf("ExhaustiveEval: fold = %+v, want valid slots and exhaustive=true", fold)
		}

		// PartialAttributeFactory keeps slots enabled and isolates local slots from unknown patterns.
		partInterp := newInterp(NewPartialAttributeFactory(cont, reg, reg))
		partPrg, err := partInterp.NewInterpretable(parsed)
		if err != nil {
			t.Fatalf("NewInterpretable with PartialAttributeFactory failed: %v", err)
		}
		if fold := unwrapEvalFold(partPrg); fold == nil || fold.accuSlot < 0 || fold.iterSlot < 0 {
			t.Errorf("PartialAttributeFactory: fold = %+v, want non-nil with valid slots", fold)
		}
		partAct, err := NewPartialActivation(EmptyActivation(), NewAttributePattern("x"))
		if err != nil {
			t.Fatalf("NewPartialActivation() failed: %v", err)
		}
		if got := partPrg.Eval(partAct); got.Equal(wantList) != types.True {
			t.Errorf("PartialAttributeFactory eval got %v, want %v", got, wantList)
		}

		// CostObserver layers on top of slots and charges EvalAttribute for slot reads.
		costInterp := newInterp(NewAttributeFactory(cont, reg, reg))
		var lastTracker *CostTracker
		costPrg, err := costInterp.NewInterpretable(parsed,
			CostObserver(CostTrackerFactory(func() (*CostTracker, error) {
				lastTracker = &CostTracker{}
				return lastTracker, nil
			})))
		if err != nil {
			t.Fatalf("NewInterpretable with CostObserver failed: %v", err)
		}
		if fold := unwrapEvalFold(costPrg); fold == nil || fold.accuSlot < 0 || fold.iterSlot < 0 {
			t.Errorf("CostObserver: fold = %+v, want non-nil with valid slots", fold)
		}
		if got := costPrg.Eval(EmptyActivation()); got.Equal(wantList) != types.True {
			t.Errorf("CostObserver eval got %v, want %v", got, wantList)
		}
		if lastTracker == nil || lastTracker.ActualCost() == 0 {
			t.Errorf("CostObserver ActualCost() = %v, want > 0", lastTracker)
		}
	})

}

func TestComprehensionSlotsEvalAndEdgeCases(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	tests := []struct {
		name    string
		expr    string
		vars    map[string]any
		wantErr string
	}{
		{
			name: "nested_shadowing_and_outer_activation",
			expr: `x + [1, 2, 3].map(x, [10, 20].filter(x, x > 10)[0] + x)[2]`,
			vars: map[string]any{"x": int64(100)},
		},
		{
			name: "sequential_comprehensions_slot_reuse",
			expr: `[1, 2, 3].map(x, x * 2) + [4, 5].filter(x, x > 4) + [6, 7].map(y, y - 1)`,
		},
		{
			name: "empty_list_filter_and_map",
			expr: `[].filter(x, x > 0).size() == 0 && [].map(x, x + 1).size() == 0`,
		},
		{
			name: "empty_map_transform",
			expr: `compre2(k, v, a, {}, {}, true, insert(a, k, v), a).size() == 0 &&
				   compre2(k, v, a, {}, {}, true, insert(a, {k: v}), a).size() == 0`,
		},
		{
			name: "all_filtered_out_list_and_map",
			expr: `[1, 2, 3].filter(x, x > 10).size() == 0 &&
				   compre2(k, v, a, {1: 2, 3: 4}, {}, true, k > 10 ? insert(a, k, v) : a, a).size() == 0 &&
				   compre2(k, v, a, {1: 2, 3: 4}, {}, true, k > 10 ? insert(a, {k: v}) : a, a).size() == 0`,
		},
		{
			name: "two_var_list_and_map_to_list_and_map",
			expr: `compre2(i, v, a, [10, 20, 30], [], true, i >= 1 ? a + [i * 100 + v] : a, a) == [120, 230] &&
				   compre2(k, v, a, {'a': 1, 'b': 2}, {}, true, insert(a, k, v * 10), a) == {'a': 10, 'b': 20}`,
		},
		{
			name: "qualified_slot_field_and_index_access",
			expr: `items.filter(x, has(x.active) && x.active).map(x, x.vals[0] + lookup[x.key])`,
			vars: map[string]any{
				"items": []map[string]any{
					{"active": true, "key": "a", "vals": []int64{10}},
					{"active": false, "key": "b", "vals": []int64{20}},
					{"key": "c", "vals": []int64{30}},
					{"active": true, "key": "d", "vals": []int64{40}},
				},
				"lookup": map[string]int64{"a": 1, "d": 4},
			},
		},
		{
			name:    "list_append_filter_error_propagates",
			expr:    `[1, 0, 2].filter(x, 10 / x > 1)`,
			wantErr: "division by zero",
		},
		{
			name:    "list_append_elem_error_propagates",
			expr:    `[1, 0, 2].map(x, 10 / x)`,
			wantErr: "division by zero",
		},
		{
			name:    "map_insert_filter_error_propagates",
			expr:    `compre2(i, v, a, [1, 0, 2], {}, true, (10 / v > 1) ? insert(a, i, v) : a, a)`,
			wantErr: "division by zero",
		},
		{
			name:    "map_insert_key_or_val_error_propagates",
			expr:    `compre2(i, v, a, [1, 0, 2], {}, true, insert(a, i, 10 / v), a)`,
			wantErr: "division by zero",
		},
		{
			name:    "map_insert_duplicate_key_error_propagates",
			expr:    `compre2(i, v, a, [1, 2], {}, true, insert(a, 1, v), a)`,
			wantErr: "already exists",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parseMutabilityExpr(t, tc.expr)
			act := EmptyActivation()
			if tc.vars != nil {
				var err error
				act, err = NewActivation(tc.vars)
				if err != nil {
					t.Fatalf("NewActivation() failed: %v", err)
				}
			}

			prg, err := interp.NewInterpretable(parsed)
			if err != nil {
				t.Fatalf("NewInterpretable() failed: %v", err)
			}
			got := prg.Eval(act)

			obsPrg, err := interp.NewInterpretable(parsed, EvalStateObserver())
			if err != nil {
				t.Fatalf("NewInterpretable(EvalStateObserver) failed: %v", err)
			}
			want := obsPrg.Eval(act)

			if tc.wantErr != "" {
				if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), tc.wantErr) {
					t.Fatalf("got %v, want error containing %q", got, tc.wantErr)
				}
				if !types.IsError(want) {
					t.Fatalf("reference EvalStateObserver did not error: %v", want)
				}
				return
			}

			if types.IsError(got) {
				t.Fatalf("got unexpected error: %v", got)
			}
			if got.Equal(want) != types.True {
				t.Errorf("slot eval got %v, want %v", got, want)
			}
		})
	}

	t.Run("iterable_only_range_with_slots", func(t *testing.T) {
		parsed := parseMutabilityExpr(t, `seq.map(x, x * 3)`)
		prg, err := interp.NewInterpretable(parsed)
		if err != nil {
			t.Fatalf("NewInterpretable() failed: %v", err)
		}
		act, _ := NewActivation(map[string]any{
			"seq": &iterableOnlyList{items: []int64{1, 2, 3, 4}},
		})
		got := prg.Eval(act)
		want := types.NewRefValList(reg, []ref.Val{types.Int(3), types.Int(6), types.Int(9), types.Int(12)})
		if got.Equal(want) != types.True {
			t.Errorf("iterable-only got %v, want %v", got, want)
		}
	})

	t.Run("interrupt_check_in_fast_path", func(t *testing.T) {
		parsed := parseMutabilityExpr(t, `[1, 2, 3, 4, 5].map(x, x + 1)`)
		prg, err := interp.NewInterpretable(parsed, InterruptableEval())
		if err != nil {
			t.Fatalf("NewInterpretable() failed: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		frame, err := NewExecutionFrame(EmptyActivation())
		if err != nil {
			t.Fatalf("NewExecutionFrame() failed: %v", err)
		}
		defer frame.Close()
		frame.SetContext(ctx, 1)
		got := prg.Exec(frame)
		if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), "operation interrupted") {
			t.Errorf("interrupted fold got %v, want operation interrupted error", got)
		}
	})
}

// nonFoldableList is a list which supports iteration and indexing, but not folding.
type nonFoldableList struct {
	traits.Lister
}

// nonFoldableMap is a map which supports iteration and lookup, but not folding.
type nonFoldableMap struct {
	traits.Mapper
}

func TestComprehensionSlotsRangesAndAccumulators(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	interp := NewInterpreter(disp, cont, reg, reg, NewAttributeFactory(cont, reg, reg))
	vars := map[string]any{
		"list":     nonFoldableList{types.NewDynamicList(reg, []int64{1, 2, 3}).(traits.Lister)},
		"map":      nonFoldableMap{types.NewDynamicMap(reg, map[int64]int64{1: 10, 2: 20}).(traits.Mapper)},
		"iterable": &iterableOnlyList{items: []int64{1, 2, 3}},
		"num":      types.Int(1),
	}
	tests := []struct {
		name    string
		expr    string
		want    any
		wantErr string
	}{
		{
			name: "non_foldable_list_range",
			expr: `list.map(x, x * 2)`,
			want: []int64{2, 4, 6},
		},
		{
			name: "non_foldable_map_range",
			expr: `compre2(k, v, a, map, 0, true, a + k * v, a)`,
			want: int64(50),
		},
		{
			name: "iterable_range_stops_early",
			expr: `iterable.exists(x, x == 2)`,
			want: true,
		},
		{
			// An iterable yields only values, so it cannot bind two iteration variables.
			name:    "two_var_iterable_range",
			expr:    `compre2(i, v, a, iterable, 0, true, a + v, a)`,
			wantErr: "unsupported comprehension range type",
		},
		{
			name:    "non_iterable_range",
			expr:    `num.all(x, x > 0)`,
			wantErr: "expected iterable type",
		},
		{
			// An error in the loop condition stops the fold, which reports it as an interruption.
			name:    "loop_condition_error",
			expr:    `compre(x, a, [1, 0], 0, 10 / x > 0, a + x, a)`,
			wantErr: "operation interrupted",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := interp.NewInterpretable(parseMutabilityExpr(t, tc.expr))
			if err != nil {
				t.Fatalf("NewInterpretable() failed: %v", err)
			}
			act, err := NewActivation(vars)
			if err != nil {
				t.Fatalf("NewActivation() failed: %v", err)
			}
			got := prg.Eval(act)
			if tc.wantErr != "" {
				if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), tc.wantErr) {
					t.Fatalf("Eval() got %v, wanted error containing %q", got, tc.wantErr)
				}
				return
			}
			if want := reg.NativeToValue(tc.want); got.Equal(want) != types.True {
				t.Errorf("Eval() got %v, wanted %v", got, want)
			}
		})
	}
}

func TestPopLocalVarBySlot(t *testing.T) {
	// A comprehension may declare several variables with the same name, which may also shadow a
	// variable of an enclosing comprehension. Popping a variable removes the one declared in its
	// slot rather than the innermost variable with its name.
	p := &planBuilder{}
	outer := p.pushLocalVar("x")
	accu := p.pushLocalVar("x")
	iter := p.pushLocalVar("x")
	// Even when popped out of nested order, the variable declared in the slot is the one removed.
	p.popLocalVar(accu)
	if want := []localVar{{"x", outer}, {"x", iter}}; !reflect.DeepEqual(p.localVars, want) {
		t.Errorf("localVars after popping the accumulator got %v, wanted %v", p.localVars, want)
	}
	// Variables without names are not declared, and popping them is a no-op.
	p.popLocalVar(p.pushLocalVar(""))
	p.popLocalVar(iter)
	if want := []localVar{{"x", outer}}; !reflect.DeepEqual(p.localVars, want) {
		t.Errorf("localVars after popping the iteration variable got %v, wanted %v", p.localVars, want)
	}

	// Planning comprehensions whose variables share names leaves no variable declared and frees
	// every slot.
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	interp := NewInterpreter(disp, cont, reg, reg, NewAttributeFactory(cont, reg, reg)).(*exprInterpreter)
	for _, expr := range []string{
		`compre(x, x, [1, 2], 10, true, x + 1, x)`,
		`compre2(k, k, k, [5, 6], 0, true, k + 1, k)`,
		`compre(x, a, [1], 0, true, compre(x, a, [2], a, true, a + x, a), a)`,
	} {
		parsed := parseMutabilityExpr(t, expr)
		pb := &planBuilder{planner: newPlanner(disp, reg, reg, interp.attrFactory, cont, parsed)}
		if _, err := pb.plan(parsed.Expr()); err != nil {
			t.Fatalf("plan(%q) failed: %v", expr, err)
		}
		if len(pb.localVars) != 0 || pb.nextSlot != 0 {
			t.Errorf("plan(%q) left localVars %v and next slot %d, wanted none and 0", expr, pb.localVars, pb.nextSlot)
		}
	}
	// Errors planning any part of a comprehension fail the plan.
	for _, expr := range []string{
		`compre(x, a, [1], Missing{}, true, a, a)`,
		`compre(x, a, Missing{}, 0, true, a, a)`,
		`compre(x, a, [1], 0, Missing{}, a, a)`,
		`compre(x, a, [1], 0, true, Missing{}, a)`,
		`compre(x, a, [1], 0, true, a, Missing{})`,
	} {
		if _, err := interp.NewInterpretable(parseMutabilityExpr(t, expr)); err == nil || !strings.Contains(err.Error(), "unknown type") {
			t.Errorf("NewInterpretable(%q) got error %v, wanted an unknown type error", expr, err)
		}
	}
}

func TestMutableAccumulatorConversion(t *testing.T) {
	reg := newTestRegistry(t)
	emptyList := types.NewDynamicList(reg, []int64{})
	list := types.NewDynamicList(reg, []int64{1})
	emptyMap := types.NewDynamicMap(reg, map[int64]int64{})
	err := types.NewErr("boom")
	unk := types.NewUnknown(1, nil)
	// Only empty lists and maps are replaced by mutable accumulators.
	if _, ok := initMutableAccu(emptyList, reg).(traits.MutableLister); !ok {
		t.Error("initMutableAccu([]) did not return a mutable list")
	}
	if _, ok := initMutableAccu(emptyMap, reg).(traits.MutableMapper); !ok {
		t.Error("initMutableAccu({}) did not return a mutable map")
	}
	for _, v := range []ref.Val{list, types.Int(0)} {
		if got := initMutableAccu(v, reg); got != v {
			t.Errorf("initMutableAccu(%v) got %v, wanted it unchanged", v, got)
		}
	}
	// Mutable results are converted to immutable ones, and any other result is returned unchanged.
	if _, ok := toImmutableAccu(types.NewMutableList(reg)).(traits.MutableLister); ok {
		t.Error("toImmutableAccu(mutable list) returned a mutable list")
	}
	if _, ok := toImmutableAccu(types.NewMutableMap(reg, nil)).(traits.MutableMapper); ok {
		t.Error("toImmutableAccu(mutable map) returned a mutable map")
	}
	for _, v := range []ref.Val{list, types.Int(0), err, unk} {
		if got := toImmutableAccu(v); got != v {
			t.Errorf("toImmutableAccu(%v) got %v, wanted it unchanged", v, got)
		}
	}
}

func TestComprehensionErrorAndUnknownPrecedence(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	act, err := NewActivation(map[string]any{
		"err1": types.NewErr("first-err"),
		"err2": types.NewErr("second-err"),
		"unk1": types.NewUnknown(101, nil),
		"unk2": types.NewUnknown(202, nil),
	})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}

	expressions := []string{
		// Filter error on later element overrides earlier element error (conditional evaluates filter before accu).
		`[1, 2].map(x, x == 1 ? true : err2, err1)`,
		// Filter false preserves earlier element error.
		`[1, 2].map(x, x == 1 ? true : false, err1)`,
		// Filter true when accu is already Err1 short-circuits elem (keeps Err1 over Err2).
		`[1, 2].map(x, true, x == 1 ? err1 : err2)`,
		// Filter unknown overrides earlier element error.
		`[1, 2].map(x, x == 1 ? true : unk2, err1)`,
		// List append: Unknown in accu + Err in elem preserves LHS Unknown.
		`[1, 2].map(x, x == 1 ? unk1 : err2)`,
		// List append: Unknown in accu + Unknown in elem merges Unknowns.
		`[1, 2].map(x, x == 1 ? unk1 : unk2)`,
		// 3-arg map insert (evalVarArgs): Unknown in accu + Err in val yields Err.
		`compre2(i, v, a, [1, 2], {}, true, insert(a, i, v == 1 ? unk1 : err2), a)`,
		// 2-arg map insert (evalBinary): Unknown in accu + Err in entry preserves LHS Unknown.
		`compre2(i, v, a, [1, 2], {}, true, insert(a, {i: v == 1 ? unk1 : err2}), a)`,
		// Map insert: Err1 in key short-circuits val (keeps Err1 over Err2).
		`compre2(i, v, a, [1], {}, true, insert(a, err1, err2), a)`,
		`compre2(i, v, a, [1], {}, true, insert(a, {err1: err2}), a)`,
		// Map insert: Unknowns merge across iterations and key/val.
		`compre2(i, v, a, [1, 2], {}, true, insert(a, v == 1 ? unk1 : i, v == 2 ? unk2 : v), a)`,
	}

	for _, expr := range expressions {
		t.Run(expr, func(t *testing.T) {
			parsed := parseMutabilityExpr(t, expr)
			fastPrg, err := interp.NewInterpretable(parsed)
			if err != nil {
				t.Fatalf("NewInterpretable() failed: %v", err)
			}
			refPrg, err := interp.NewInterpretable(parsed, EvalStateObserver())
			if err != nil {
				t.Fatalf("NewInterpretable(EvalStateObserver) failed: %v", err)
			}
			got := fastPrg.Eval(act)
			want := refPrg.Eval(act)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got (%T)%v, want (%T)%v", got, got, want, want)
			}
		})
	}
}

func TestExecutionFramePooledSlotsBounds(t *testing.T) {
	frame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	slots := frame.EnsureSlots(8)
	slots[0] = types.IntOne
	if len(frame.localSlots) != 8 {
		t.Fatalf("len(frame.localSlots) = %d, want 8", len(frame.localSlots))
	}
	frame.Close()
	// Within maxPooledSlots, backing slice capacity is retained but length is reset to 0 and cleared.
	if len(frame.localSlots) != 0 || cap(frame.localSlots) < 8 {
		t.Errorf("after Close() within bound: len=%d cap=%d, want len=0 cap>=8", len(frame.localSlots), cap(frame.localSlots))
	}

	oversized, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	oversized.EnsureSlots(maxPooledSlots + 1)
	oversized.Close()
	if oversized.localSlots != nil {
		t.Errorf("after Close() exceeding maxPooledSlots: localSlots cap=%d, want nil", cap(oversized.localSlots))
	}

	// EnsureSlots growth preserves existing slot values and reuses capacity when possible.
	growFrame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	slots4 := growFrame.EnsureSlots(4)
	slots4[2] = types.IntOne
	slots6 := growFrame.EnsureSlots(6)
	if len(slots6) != 6 || slots6[2] != types.IntOne {
		t.Fatalf("growing EnsureSlots clobbered existing slot: len=%d, val=%v", len(slots6), slots6)
	}
	growFrame.Close()
	if len(growFrame.localSlots) != 0 || cap(growFrame.localSlots) < 6 {
		t.Errorf("after EnsureSlots growth + Close(): len=%d cap=%d, want len=0 cap>=6", len(growFrame.localSlots), cap(growFrame.localSlots))
	}
	growFrame, err = NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	// Re-slicing within existing capacity in EnsureSlots.
	if got := growFrame.EnsureSlots(4); len(got) != 4 {
		t.Errorf("EnsureSlots(4) within cap got len=%d, want 4", len(got))
	}
	growFrame.Close()

	// evalSlot labels unlabeled *types.Err with node ID and resolves through wrapped Activations.
	reg := newTestRegistry(t)
	slotNode := &evalSlot{
		id:      42,
		slot:    0,
		adapter: reg,
		fac:     NewAttributeFactory(containers.DefaultContainer, reg, reg),
	}
	errFrame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	defer errFrame.Close()
	errFrame.EnsureSlots(1)[0] = types.NewErr("boom")
	gotErr, ok := slotNode.Exec(errFrame).(*types.Err)
	if !ok || gotErr.NodeID() != 42 {
		t.Errorf("slotNode.Exec(unlabeled Err) = %v (id=%d), want *types.Err with node ID 42", gotErr, gotErr.NodeID())
	}
	errFrame.localSlots[0] = types.Int(99)
	if got := slotNode.Eval(errFrame); got != types.Int(99) {
		t.Errorf("slotNode.Eval(frame) = %v, want 99", got)
	}
	wrappedAct := NewHierarchicalActivation(errFrame, EmptyActivation())
	resolved, err := slotNode.Resolve(wrappedAct)
	if err != nil || resolved != types.Int(99) {
		t.Errorf("slotNode.Resolve(wrappedAct) = (%v, %v), want (99, nil)", resolved, err)
	}
	if _, err := slotNode.Resolve(EmptyActivation()); err == nil {
		t.Error("slotNode.Resolve(activation without a frame) succeeded, want an error")
	}

	// An optional qualifier which selects an unknown value yields the unknown rather than an
	// optional wrapping it.
	optQual, err := slotNode.fac.NewQualifier(nil, 43, "a", true)
	if err != nil {
		t.Fatalf("NewQualifier() failed: %v", err)
	}
	optSlot := &evalSlot{id: 44, slot: 0, adapter: reg, fac: slotNode.fac, qualifiers: []Qualifier{optQual}}
	unk := types.NewUnknown(45, nil)
	errFrame.localSlots[0] = map[string]any{"a": unk}
	if got := optSlot.Exec(errFrame); got != unk {
		t.Errorf("optSlot.Exec(unknown field) = %v, want %v", got, unk)
	}
	errFrame.localSlots[0] = map[string]any{"a": int64(1)}
	if got := optSlot.Exec(errFrame); got.Equal(types.OptionalOf(types.Int(1))) != types.True {
		t.Errorf("optSlot.Exec(present field) = %v, want optional.of(1)", got)
	}
}

func TestComprehensionSlotAllocations(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	makeList := func(n int) []int64 {
		s := make([]int64, n)
		for i := range n {
			s[i] = int64(i + 1)
		}
		return s
	}

	act10, _ := NewActivation(map[string]any{"xs": makeList(10)})
	act100, _ := NewActivation(map[string]any{"xs": makeList(100)})

	// 1. Predicate comprehension (all/exists) with 0 container construction should have identical
	//    constant allocations for N=10 and N=100 (zero ExecutionFrame pushes, zero Activation folders).
	t.Run("predicate_constant_allocs", func(t *testing.T) {
		parsed := parseMutabilityExpr(t, `xs.all(x, x > 0)`)
		prg, err := interp.NewInterpretable(parsed)
		if err != nil {
			t.Fatalf("NewInterpretable() failed: %v", err)
		}
		allocs10 := testing.AllocsPerRun(100, func() {
			if out := prg.Eval(act10); out != types.True {
				t.Fatalf("got %v, want true", out)
			}
		})
		allocs100 := testing.AllocsPerRun(100, func() {
			if out := prg.Eval(act100); out != types.True {
				t.Fatalf("got %v, want true", out)
			}
		})
		if allocs100-allocs10 > 2 {
			t.Errorf("all(x, x > 0) allocs scaled with N: N=10 had %v, N=100 had %v", allocs10, allocs100)
		}
	})

	// 2. Empty-match filter (list and map) should allocate zero intermediate containers and
	//    have constant O(1) allocations for N=10 and N=100.
	t.Run("empty_match_zero_intermediate_allocs", func(t *testing.T) {
		expressions := []string{
			`xs.filter(x, x < 0)`,
			`compre2(i, v, a, xs, {}, true, v < 0 ? insert(a, i, v) : a, a)`,
			`compre2(i, v, a, xs, {}, true, v < 0 ? insert(a, {i: v}) : a, a)`,
		}
		for _, expr := range expressions {
			parsed := parseMutabilityExpr(t, expr)
			prg, err := interp.NewInterpretable(parsed)
			if err != nil {
				t.Fatalf("NewInterpretable(%q) failed: %v", expr, err)
			}
			allocs10 := testing.AllocsPerRun(100, func() {
				_ = prg.Eval(act10)
			})
			allocs100 := testing.AllocsPerRun(100, func() {
				_ = prg.Eval(act100)
			})
			if allocs100-allocs10 > 2 {
				t.Errorf("%q allocs scaled with N: N=10 had %v, N=100 had %v", expr, allocs10, allocs100)
			}
		}
	})
}

func BenchmarkComprehensionSlots(b *testing.B) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(b)
	attrs := NewAttributeFactory(cont, reg, reg)
	disp := NewDispatcher()
	addFunctionBindings(b, disp)
	addMapInsertBinding(b, disp)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	xs := make([]int64, 50)
	m := make(map[int64]int64, 50)
	for i := range xs {
		xs[i] = int64(i)
		m[int64(i)] = int64(i * 10)
	}
	act, err := NewActivation(map[string]any{"xs": xs, "m": m})
	if err != nil {
		b.Fatalf("NewActivation() failed: %v", err)
	}

	cases := []struct {
		name string
		expr string
	}{
		{name: "all_50", expr: `xs.all(x, x >= 0)`},
		{name: "filter_50", expr: `xs.filter(x, x >= 25)`},
		{name: "map_50", expr: `xs.map(x, x * 2)`},
		{name: "transformMap_50", expr: `compre2(k, v, a, m, {}, true, (k >= 25) ? insert(a, k, v + 1) : a, a)`},
		{name: "transformMapEntry_50", expr: `compre2(k, v, a, m, {}, true, (k >= 25) ? insert(a, {k + 100: v}) : a, a)`},
	}

	p, err := parser.NewParser(
		parser.Macros(append(parser.AllMacros, compreMacro, compre2Macro, insertMacro)...),
	)
	if err != nil {
		b.Fatalf("parser.NewParser() failed: %v", err)
	}

	for _, tc := range cases {
		parsed, errs := p.Parse(common.NewTextSource(tc.expr))
		if len(errs.GetErrors()) != 0 {
			b.Fatalf("Parse(%q) failed: %v", tc.expr, errs.ToDisplayString())
		}
		hideTestVars(parsed.Expr())
		prg, err := interp.NewInterpretable(parsed, Optimize())
		if err != nil {
			b.Fatalf("NewInterpretable(%q) failed: %v", tc.expr, err)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = prg.Eval(act)
			}
		})
	}
}
