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
	"errors"
	"strings"
	"testing"

	"cel.dev/cel-go/checker"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/parser"
	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestMemoryObserver_FactoryNotConfigured(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := newStandardInterpreter(t, cont, reg, reg, attrs)

	s := common.NewTextSource(`1 + 1`)
	p, err := parser.NewParser()
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Parse() failed: %v", errs.GetErrors())
	}
	env := newTestEnv(t, cont, reg)
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Check() failed: %v", errs.GetErrors())
	}

	_, err = interp.NewInterpretable(checked, MemoryObserver())
	if err == nil || !strings.Contains(err.Error(), "memory tracker factory not configured") {
		t.Fatalf("NewInterpretable() got error %v, wanted 'memory tracker factory not configured'", err)
	}
}

func TestMemoryObserver_StateLifecycle(t *testing.T) {
	fac := &memoryTrackerFactory{
		factory: func() (*types.MemoryTracker, error) {
			return types.NewMemoryTracker(), nil
		},
	}

	frame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	defer frame.Close()

	if got := fac.GetState(nil); got != nil {
		t.Errorf("GetState(nil) got %v, wanted nil", got)
	}
	if got := fac.GetState(frame); got != nil {
		t.Errorf("GetState(uninitialized) got %v, wanted nil", got)
	}

	state, err := fac.InitState(frame)
	if err != nil {
		t.Fatalf("InitState() failed: %v", err)
	}
	if state == nil {
		t.Fatal("InitState() returned nil state")
	}

	gotState := fac.GetState(frame)
	if gotState != state {
		t.Errorf("GetState() got %v, wanted %v", gotState, state)
	}

	// Calling InitState a second time returns the existing state
	state2, err := fac.InitState(frame)
	if err != nil {
		t.Fatalf("InitState() second call failed: %v", err)
	}
	if state2 != state {
		t.Errorf("InitState() second call got %v, wanted %v", state2, state)
	}

	// Test factory error propagation
	errFac := &memoryTrackerFactory{
		factory: func() (*types.MemoryTracker, error) {
			return nil, errors.New("factory failure")
		},
	}
	frame2, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	defer frame2.Close()

	_, err = errFac.InitState(frame2)
	if err == nil || !strings.Contains(err.Error(), "factory failure") {
		t.Fatalf("InitState() got error %v, wanted 'factory failure'", err)
	}
}

func TestMemoryObserver_Eval(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		vars     []*decls.VariableDecl
		in       any
		memOpts  []types.MemoryTrackerOption
		wantPeak uint32
	}{
		{
			name:     "ident_attribute",
			expr:     `a`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3}},
			wantPeak: 4,
		},
		{
			name:     "field_selection",
			expr:     `m.vals`,
			vars:     []*decls.VariableDecl{decls.NewVariable("m", types.NewMapType(types.StringType, types.NewListType(types.IntType)))},
			in:       map[string]any{"m": map[string][]int64{"vals": {1, 2, 3}}},
			wantPeak: 4,
		},
		{
			name:     "call_list_concat",
			expr:     `a + a`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2}},
			wantPeak: 6,
		},
		{
			name:     "call_string_concat",
			expr:     `s + s`,
			vars:     []*decls.VariableDecl{decls.NewVariable("s", types.StringType)},
			in:       map[string]any{"s": strings.Repeat("a", 50)},
			wantPeak: 10,
		},
		{
			name:     "list_literal",
			expr:     `[1, 2, 3, 4]`,
			wantPeak: 5,
		},
		{
			name:     "map_literal",
			expr:     `{'k1': 1, 'k2': 2}`,
			wantPeak: 5,
		},
		{
			name:     "comprehension_map",
			expr:     `a.map(x, x * 2)`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3, 4, 5}},
			wantPeak: 6,
		},
		{
			name:     "comprehension_filter",
			expr:     `a.filter(x, x % 2 == 0)`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3, 4, 5}},
			wantPeak: 3,
		},
		{
			name:     "comprehension_exists",
			expr:     `a.exists(x, x == 3)`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3, 4, 5}},
			wantPeak: 1,
		},
		{
			name:     "comprehension_all",
			expr:     `a.all(x, x > 0)`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3, 4, 5}},
			wantPeak: 1,
		},
		{
			name:     "comprehension_sampled",
			expr:     `a.map(x, x * 2)`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3, 4, 5}},
			memOpts:  []types.MemoryTrackerOption{types.MemoryTrackerSampleInterval(3)},
			wantPeak: 6,
		},
		{
			name:     "comprehension_single_element",
			expr:     `[1].map(x, [x, x])`,
			memOpts:  []types.MemoryTrackerOption{types.MemoryTrackerSampleInterval(10)},
			wantPeak: 3,
		},
		{
			name:     "comprehension_nested_sampled",
			expr:     `a.map(x, [x, x]).filter(z, z.size() == 2)`,
			vars:     []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:       map[string]any{"a": []int64{1, 2, 3, 4, 5}},
			memOpts:  []types.MemoryTrackerOption{types.MemoryTrackerSampleInterval(2)},
			wantPeak: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracker, res, err := evalTestMemoryTracker(t, tc.expr, tc.vars, tc.in, tc.memOpts...)
			if err != nil {
				t.Fatalf("evalTestMemoryTracker() failed: %v", err)
			}
			if types.IsError(res) {
				t.Fatalf("eval result error: %v", res)
			}
			if tracker == nil {
				t.Fatal("expected non-nil MemoryTracker")
			}
			if tracker.Peak() < tc.wantPeak {
				t.Errorf("tracker.Peak() got %d, want at least %d", tracker.Peak(), tc.wantPeak)
			}
		})
	}
}

func TestMemoryObserver_LimitExceeded(t *testing.T) {
	t.Run("over_limit_panics", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected evaluation to panic on memory limit exceeded")
			}
			evalErr, ok := r.(EvalCancelledError)
			if !ok {
				t.Fatalf("expected EvalCancelledError, got %T: %v", r, r)
			}
			if evalErr.Cause != MemoryLimitExceeded {
				t.Errorf("EvalCancelledError.Cause got %v, want %v", evalErr.Cause, MemoryLimitExceeded)
			}
		}()

		vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
		in := map[string]any{"a": []int64{1, 2, 3}}
		_, _, _ = evalTestMemoryTracker(t, `a + a`, vars, in, types.MemoryTrackerLimit(5))
	})

	t.Run("under_limit_succeeds", func(t *testing.T) {
		vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
		in := map[string]any{"a": []int64{1, 2}}
		tracker, res, err := evalTestMemoryTracker(t, `a + a`, vars, in, types.MemoryTrackerLimit(100))
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if types.IsError(res) {
			t.Fatalf("eval result error: %v", res)
		}
		if tracker.ExceedsLimit() {
			t.Error("tracker.ExceedsLimit() got true, want false")
		}
	})
}

func evalTestMemoryTracker(t testing.TB, expr string, vars []*decls.VariableDecl, in any, memOpts ...types.MemoryTrackerOption) (*types.MemoryTracker, ref.Val, error) {
	t.Helper()
	s := common.NewTextSource(expr)
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("Failed to initialize parser: %v", err)
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Failed to parse expression %q: %v", expr, errs.GetErrors())
	}

	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	env := newTestEnv(t, cont, reg)
	if len(vars) > 0 {
		if err := env.AddIdents(vars...); err != nil {
			t.Fatalf("Failed to add variables: %v", err)
		}
	}
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Failed to check expression %q: %v", expr, errs.GetErrors())
	}

	var tracker *types.MemoryTracker
	factory := func() (*types.MemoryTracker, error) {
		tracker = types.NewMemoryTracker(memOpts...)
		return tracker, nil
	}

	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)
	prg, err := interp.NewInterpretable(checked, Optimize(), MemoryObserver(MemoryTrackerFactory(factory)))
	if err != nil {
		return nil, nil, err
	}

	act := constructTestActivation(t, in)
	res := prg.Eval(act)
	return tracker, res, nil
}

func constructTestActivation(t testing.TB, in any) Activation {
	t.Helper()
	if in == nil {
		return EmptyActivation()
	}
	a, err := NewActivation(in)
	if err != nil {
		t.Fatalf("NewActivation(%v) failed: %v", in, err)
	}
	return a
}

func benchmarkMemoryTracker(b *testing.B, expr string, vars []*decls.VariableDecl, in any, enableTracking bool, memOpts ...types.MemoryTrackerOption) {
	b.Helper()
	s := common.NewTextSource(expr)
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		b.Fatalf("Failed to initialize parser: %v", err)
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		b.Fatalf("Failed to parse expression %q: %v", expr, errs.GetErrors())
	}

	cont := containers.DefaultContainer
	reg := newTestRegistry(b)
	attrs := NewAttributeFactory(cont, reg, reg)
	env := newTestEnv(b, cont, reg)
	if len(vars) > 0 {
		if err := env.AddIdents(vars...); err != nil {
			b.Fatalf("Failed to add variables: %v", err)
		}
	}
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		b.Fatalf("Failed to check expression %q: %v", expr, errs.GetErrors())
	}

	disp := NewDispatcher()
	addFunctionBindings(b, disp)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	planOpts := []PlannerOption{Optimize()}
	if enableTracking {
		factory := func() (*types.MemoryTracker, error) {
			return types.NewMemoryTracker(memOpts...), nil
		}
		planOpts = append(planOpts, MemoryObserver(MemoryTrackerFactory(factory)))
	}

	prg, err := interp.NewInterpretable(checked, planOpts...)
	if err != nil {
		b.Fatalf("NewInterpretable() failed: %v", err)
	}

	frame := AsFrame(constructTestActivation(b, in))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prg.Eval(frame)
	}
}

func BenchmarkMemoryTracker_SimpleCall_TrackingDisabled(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5}}
	benchmarkMemoryTracker(b, `a + a`, vars, in, false)
}

func BenchmarkMemoryTracker_SimpleCall_TrackingEnabled(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5}}
	benchmarkMemoryTracker(b, `a + a`, vars, in, true)
}

func BenchmarkMemoryTracker_Constructor_TrackingDisabled(b *testing.B) {
	benchmarkMemoryTracker(b, `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]`, nil, nil, false)
}

func BenchmarkMemoryTracker_Constructor_TrackingEnabled(b *testing.B) {
	benchmarkMemoryTracker(b, `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]`, nil, nil, true)
}

func BenchmarkMemoryTracker_Comprehension_TrackingDisabled(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}
	benchmarkMemoryTracker(b, `a.map(x, x * 2)`, vars, in, false)
}

func BenchmarkMemoryTracker_Comprehension_SampleInterval1(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}
	benchmarkMemoryTracker(b, `a.map(x, x * 2)`, vars, in, true, types.MemoryTrackerSampleInterval(1))
}

func BenchmarkMemoryTracker_Comprehension_SampleInterval5(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}
	benchmarkMemoryTracker(b, `a.map(x, x * 2)`, vars, in, true, types.MemoryTrackerSampleInterval(5))
}

func BenchmarkMemoryTracker_NestedComprehension_TrackingDisabled(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5}}
	benchmarkMemoryTracker(b, `a.map(x, [x, x]).filter(z, z.size() == 2)`, vars, in, false)
}

func BenchmarkMemoryTracker_NestedComprehension_TrackingEnabled(b *testing.B) {
	vars := []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5}}
	benchmarkMemoryTracker(b, `a.map(x, [x, x]).filter(z, z.size() == 2)`, vars, in, true)
}

type optionsScenario struct {
	name      string
	trackMem  bool
	memSample uint32
	trackCost bool
	trackTrac bool
}

func runOptionsBenchmark(b *testing.B, expr string, vars []*decls.VariableDecl, in any, sc optionsScenario) {
	b.Helper()
	s := common.NewTextSource(expr)
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		b.Fatalf("Failed to initialize parser: %v", err)
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		b.Fatalf("Failed to parse expression %q: %v", expr, errs.GetErrors())
	}

	cont := containers.DefaultContainer
	reg := newTestRegistry(b, types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	attrs := NewAttributeFactory(cont, reg, reg)
	env := newTestEnv(b, cont, reg)
	if len(vars) > 0 {
		if err := env.AddIdents(vars...); err != nil {
			b.Fatalf("Failed to add variables: %v", err)
		}
	}
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		b.Fatalf("Failed to check expression %q: %v", expr, errs.GetErrors())
	}

	disp := NewDispatcher()
	addFunctionBindings(b, disp)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	planOpts := []PlannerOption{Optimize()}
	if sc.trackMem {
		factory := func() (*types.MemoryTracker, error) {
			var memOpts []types.MemoryTrackerOption
			if sc.memSample > 0 {
				memOpts = append(memOpts, types.MemoryTrackerSampleInterval(sc.memSample))
			}
			return types.NewMemoryTracker(memOpts...), nil
		}
		planOpts = append(planOpts, MemoryObserver(MemoryTrackerFactory(factory)))
	}
	if sc.trackCost {
		costFac := func() (*CostTracker, error) {
			return NewCostTracker(nil)
		}
		planOpts = append(planOpts, CostObserver(CostTrackerFactory(costFac)))
	}
	if sc.trackTrac {
		planOpts = append(planOpts, EvalStateObserver())
	}

	prg, err := interp.NewInterpretable(checked, planOpts...)
	if err != nil {
		b.Fatalf("NewInterpretable() failed: %v", err)
	}

	frame := AsFrame(constructTestActivation(b, in))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prg.Eval(frame)
	}
}

func BenchmarkOptionsComparison(b *testing.B) {
	scenarios := []optionsScenario{
		{name: "None"},
		{name: "CostTracking", trackCost: true},
		{name: "MemoryTracking_Sample1", trackMem: true, memSample: 1},
		{name: "MemoryTracking_Sample5", trackMem: true, memSample: 5},
		{name: "Tracing_State", trackTrac: true},
		{name: "Cost_and_Memory", trackCost: true, trackMem: true, memSample: 1},
		{name: "Cost_and_Tracing", trackCost: true, trackTrac: true},
		{name: "Memory_and_Tracing", trackMem: true, memSample: 1, trackTrac: true},
		{name: "All_Options", trackCost: true, trackMem: true, memSample: 1, trackTrac: true},
	}

	workloads := []struct {
		name string
		expr string
		vars []*decls.VariableDecl
		in   any
	}{
		{
			name: "SimpleCall",
			expr: `a + a`,
			vars: []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:   map[string]any{"a": []int64{1, 2, 3, 4, 5}},
		},
		{
			name: "LiteralConstructor",
			expr: `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]`,
		},
		{
			name: "MapComprehension_10",
			expr: `a.map(x, x * 2)`,
			vars: []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:   map[string]any{"a": []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		},
		{
			name: "NestedComprehension_10",
			expr: `a.map(x, [x, x]).filter(z, z.size() == 2)`,
			vars: []*decls.VariableDecl{decls.NewVariable("a", types.NewListType(types.IntType))},
			in:   map[string]any{"a": []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		},
		{
			name: "ProtoFieldAccess",
			expr: `msg.single_int64 + msg.single_int32`,
			vars: []*decls.VariableDecl{decls.NewVariable("msg", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			in:   map[string]any{"msg": &proto3pb.TestAllTypes{SingleInt64: 42, SingleInt32: 10, SingleString: "hello world"}},
		},
		{
			name: "ProtoRepeatedComprehension_10",
			expr: `msg.repeated_int64.map(x, x * 2)`,
			vars: []*decls.VariableDecl{decls.NewVariable("msg", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			in:   map[string]any{"msg": &proto3pb.TestAllTypes{RepeatedInt64: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}},
		},
	}

	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			for _, sc := range scenarios {
				b.Run(sc.name, func(b *testing.B) {
					runOptionsBenchmark(b, w.expr, w.vars, w.in, sc)
				})
			}
		})
	}
}
