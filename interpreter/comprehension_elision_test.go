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
	"strings"
	"testing"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

func unwrapDecoratedNode(node InterpretableV2) InterpretableV2 {
	for {
		switch n := node.(type) {
		case *ObservableInterpretable:
			node = n.InterpretableV2
		case *plannedInterpretable:
			node = n.InterpretableV2
		case *evalWatch:
			node = n.InterpretableV2
		case *evalWatchAttr:
			node = n.InterpretableAttribute
		default:
			return node
		}
	}
}

func unwrapElidedStep(step InterpretableV2) (elided InterpretableV2, isConditional bool) {
	step = unwrapDecoratedNode(step)
	switch s := step.(type) {
	case *evalAttr:
		if condAttr, ok := s.Attr().(*conditionalAttribute); ok {
			if relAttr, ok := condAttr.truthy.(*relativeAttribute); ok {
				op := unwrapDecoratedNode(relAttr.operand)
				switch op.(type) {
				case *evalListAppend, *evalMapInsert:
					return op, true
				}
			}
		}
	case *evalExhaustiveConditional:
		if relAttr, ok := s.attr.truthy.(*relativeAttribute); ok {
			op := unwrapDecoratedNode(relAttr.operand)
			switch op.(type) {
			case *evalListAppend, *evalMapInsert:
				return op, true
			}
		}
	}
	return step, false
}

func TestComprehensionStepElisionPlanning(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)
	partAttrs := NewPartialAttributeFactory(cont, reg, reg)
	partInterp := NewInterpreter(disp, cont, reg, reg, partAttrs)

	t.Run("elided_list_and_map_steps_across_modes", func(t *testing.T) {
		tests := []struct {
			expr       string
			wantList   bool
			wantMap    bool
			wantCond   bool
			wantMapBin bool
		}{
			{
				expr:     `[1, 2, 3].map(x, x + 1)`,
				wantList: true,
			},
			{
				expr:     `[1, 2, 3].filter(x, x > 1)`,
				wantList: true,
				wantCond: true,
			},
			{
				expr:     `compre2(i, v, a, [10, 20], [], true, a + [i + v], a)`,
				wantList: true,
			},
			{
				expr:     `compre2(i, v, a, [10, 20], [], true, i > 0 ? a + [v] : a, a)`,
				wantList: true,
				wantCond: true,
			},
			{
				expr:    `compre2(k, v, a, {1: 2}, {}, true, insert(a, k, v + 1), a)`,
				wantMap: true,
			},
			{
				expr:       `compre2(k, v, a, {1: 2}, {}, true, insert(a, {k: v + 1}), a)`,
				wantMap:    true,
				wantMapBin: true,
			},
			{
				expr:       `compre2(k, v, a, {1: 2}, {}, true, k > 0 ? insert(a, {k: v + 1}) : a, a)`,
				wantMap:    true,
				wantCond:   true,
				wantMapBin: true,
			},
		}

		modes := []struct {
			name   string
			interp Interpreter
			opts   []PlannerOption
		}{
			{name: "standard", interp: interp},
			{name: "optimize", interp: interp, opts: []PlannerOption{Optimize()}},
			{
				name:   "cost_memory_state_exhaustive",
				interp: interp,
				opts: []PlannerOption{
					Optimize(),
					CostObserver(CostTrackerFactory(func() (*CostTracker, error) { return NewCostTracker(nil) })),
					MemoryObserver(MemoryTrackerFactory(func() (*types.MemoryTracker, error) { return types.NewMemoryTracker(), nil })),
					EvalStateObserver(),
					ExhaustiveEval(),
				},
			},
			{
				name:   "partial_attribute_factory",
				interp: partInterp,
				opts:   []PlannerOption{EvalStateObserver()},
			},
		}

		for _, mode := range modes {
			t.Run(mode.name, func(t *testing.T) {
				for _, tc := range tests {
					parsed := parseMutabilityExpr(t, tc.expr)
					prg, err := mode.interp.NewInterpretable(parsed, mode.opts...)
					if err != nil {
						t.Fatalf("NewInterpretable(%q) failed: %v", tc.expr, err)
					}
					fold := unwrapDecoratedNode(prg).(*evalFold)
					leaf, isCond := unwrapElidedStep(fold.step)
					if isCond != tc.wantCond {
						t.Errorf("%q: isCond = %v, want %v", tc.expr, isCond, tc.wantCond)
					}
					_, isList := leaf.(*evalListAppend)
					if isList != tc.wantList {
						t.Errorf("%q: isListAppend = %v, want %v (got %T)", tc.expr, isList, tc.wantList, leaf)
					}
					mapIns, isMap := leaf.(*evalMapInsert)
					if isMap != tc.wantMap {
						t.Errorf("%q: isMapInsert = %v, want %v (got %T)", tc.expr, isMap, tc.wantMap, leaf)
					}
					if isMap && mapIns.binary != tc.wantMapBin {
						t.Errorf("%q: evalMapInsert.binary = %v, want %v", tc.expr, mapIns.binary, tc.wantMapBin)
					}
				}
			})
		}
	})

	t.Run("non_elidable_fallbacks_and_aliasing_guards", func(t *testing.T) {
		fac := ast.NewExprFactory()
		// Optional single-element list: @a + [?x]
		optListCompre := fac.NewComprehension(1,
			fac.NewList(2, []ast.Expr{fac.NewLiteral(3, types.Int(1))}, nil),
			"x", "@a",
			fac.NewList(4, nil, nil),
			fac.NewLiteral(5, types.True),
			fac.NewCall(6, operators.Add,
				fac.NewIdent(7, "@a"),
				fac.NewList(8, []ast.Expr{fac.NewIdent(9, "x")}, []int32{0}),
			),
			fac.NewIdent(10, "@a"),
		)
		optListPrg, err := interp.NewInterpretable(ast.NewAST(optListCompre, ast.NewSourceInfo(nil)))
		if err != nil {
			t.Fatalf("NewInterpretable(optListCompre) failed: %v", err)
		}
		if _, ok := unwrapEvalFold(optListPrg).step.(*evalListAppend); ok {
			t.Errorf("optional list element should not elide to *evalListAppend")
		}

		// Optional map entry: insert(@a, {?k: v})
		optMapCompre := fac.NewComprehensionTwoVar(1,
			fac.NewMap(2, nil),
			"k", "v", "@a",
			fac.NewMap(3, nil),
			fac.NewLiteral(4, types.True),
			fac.NewCall(5, mapInsertFunction,
				fac.NewIdent(6, "@a"),
				fac.NewMap(7, []ast.EntryExpr{
					fac.NewMapEntry(8, fac.NewIdent(9, "k"), fac.NewIdent(10, "v"), true),
				}),
			),
			fac.NewIdent(11, "@a"),
		)
		optMapPrg, err := interp.NewInterpretable(ast.NewAST(optMapCompre, ast.NewSourceInfo(nil)))
		if err != nil {
			t.Fatalf("NewInterpretable(optMapCompre) failed: %v", err)
		}
		if _, ok := unwrapEvalFold(optMapPrg).step.(*evalMapInsert); ok {
			t.Errorf("optional map entry should not elide to *evalMapInsert")
		}

		// Conditional falsy branch not equal to @a (`x != 2 ? (@a + [x]) : [@a, @a]`) does not
		// match the standard filter ternary shape and must not elide the step.
		aliasCompre := fac.NewComprehension(1,
			fac.NewList(2, []ast.Expr{
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.Int(2)),
				fac.NewLiteral(5, types.Int(3)),
			}, nil),
			"x", "@a",
			fac.NewList(6, nil, nil),
			fac.NewLiteral(7, types.True),
			fac.NewCall(8, operators.Conditional,
				fac.NewCall(9, operators.NotEquals, fac.NewIdent(10, "x"), fac.NewLiteral(11, types.Int(2))),
				fac.NewCall(12, operators.Add,
					fac.NewIdent(13, "@a"),
					fac.NewList(14, []ast.Expr{fac.NewIdent(15, "x")}, nil),
				),
				fac.NewList(16, []ast.Expr{fac.NewIdent(17, "@a"), fac.NewIdent(18, "@a")}, nil),
			),
			fac.NewIdent(19, "@a"),
		)
		aliasPrg, err := interp.NewInterpretable(ast.NewAST(aliasCompre, ast.NewSourceInfo(nil)))
		if err != nil {
			t.Fatalf("NewInterpretable(aliasCompre) failed: %v", err)
		}
		aliasFold := unwrapEvalFold(aliasPrg)
		if leaf, isCond := unwrapElidedStep(aliasFold.step); isCond || leaf != aliasFold.step {
			t.Errorf("non-accuVar falsy branch should not elide step, got %T (isCond=%v)", leaf, isCond)
		}
		aliasOut := aliasPrg.Eval(EmptyActivation()).(traits.Lister)
		if aliasOut.Size() != types.Int(3) || aliasOut.Get(types.IntZero).(traits.Lister).Size() != types.Int(1) {
			t.Errorf("aliased accumulator was mutated across iterations: %v", aliasOut)
		}

		// Late-bound cel.@mapInsert overload disables elision.
		lateDisp := NewDispatcher()
		addFunctionBindings(t, lateDisp)
		if err := lateDisp.Add(&functions.Overload{
			Operator:  mapInsertFunction,
			LateBound: true,
		}); err != nil {
			t.Fatalf("lateDisp.Add() failed: %v", err)
		}
		lateInterp := NewInterpreter(lateDisp, cont, reg, reg, attrs)
		latePrg, err := lateInterp.NewInterpretable(parseMutabilityExpr(t, `compre2(k, v, a, {1: 2}, {}, true, insert(a, k, v), a)`))
		if err != nil {
			t.Fatalf("NewInterpretable(lateBound) failed: %v", err)
		}
		if _, ok := unwrapEvalFold(latePrg).step.(*evalMapInsert); ok {
			t.Errorf("late-bound cel.@mapInsert should not elide to *evalMapInsert")
		}

		// Missing operators.Add in Dispatcher must NOT be bypassed by evalListAppend.
		emptyDisp := NewDispatcher()
		noAddInterp := NewInterpreter(emptyDisp, cont, reg, reg, attrs)
		noAddPrg, err := noAddInterp.NewInterpretable(parseMutabilityExpr(t, `[1, 2].map(x, x)`))
		if err != nil {
			t.Fatalf("NewInterpretable(noAdd) failed: %v", err)
		}
		if _, ok := unwrapEvalFold(noAddPrg).step.(*evalListAppend); ok {
			t.Errorf("missing operators.Add should not elide to *evalListAppend")
		}
		noAddOut := noAddPrg.Eval(EmptyActivation())
		if !types.IsError(noAddOut) || !strings.Contains(noAddOut.(*types.Err).Error(), "no such overload") {
			t.Errorf("missing operators.Add Eval() = %v, want 'no such overload' error", noAddOut)
		}

		// Custom specialized operators.Add overload (without AdderType trait) must NOT be bypassed.
		customDisp := NewDispatcher()
		if err := customDisp.Add(&functions.Overload{
			Operator: operators.Add,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return types.NewErr("custom add blocked")
			},
		}); err != nil {
			t.Fatalf("customDisp.Add() failed: %v", err)
		}
		customInterp := NewInterpreter(customDisp, cont, reg, reg, attrs)
		customPrg, err := customInterp.NewInterpretable(parseMutabilityExpr(t, `[1, 2].map(x, x)`))
		if err != nil {
			t.Fatalf("NewInterpretable(customAdd) failed: %v", err)
		}
		if _, ok := unwrapEvalFold(customPrg).step.(*evalListAppend); ok {
			t.Errorf("custom operators.Add should not elide to *evalListAppend")
		}
		customOut := customPrg.Eval(EmptyActivation())
		if !types.IsError(customOut) || !strings.Contains(customOut.(*types.Err).Error(), "custom add blocked") {
			t.Errorf("custom operators.Add Eval() = %v, want 'custom add blocked'", customOut)
		}
	})
}

func TestComprehensionStepElisionActivationAccu(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)

	accuAttr := &evalAttr{adapter: reg, attr: attrs.MaybeAttribute(1, "accu")}
	elemConst := NewConstValue(2, types.Int(42))
	appendNode := &evalListAppend{
		id:       3,
		listID:   4,
		overload: overloads.AddList,
		accu:     accuAttr,
		elem:     elemConst,
		adapter:  reg,
	}

	// Mutable list accumulator via Activation attribute.
	mutList := types.NewMutableList(reg)
	actMut, _ := NewActivation(map[string]any{"accu": mutList})
	gotMut := toImmutableAccu(appendNode.Eval(actMut))
	if gotMut.Equal(reg.NativeToValue([]int64{42})) != types.True {
		t.Errorf("evalListAppend(mutable attr) = %v, want [42]", gotMut)
	}

	// Immutable list accumulator via Activation attribute (ExhaustiveEval fallback path).
	immList := reg.NativeToValue([]int64{10})
	actImm, _ := NewActivation(map[string]any{"accu": immList})
	gotImm := appendNode.Eval(actImm)
	if gotImm.Equal(reg.NativeToValue([]int64{10, 42})) != types.True {
		t.Errorf("evalListAppend(immutable attr) = %v, want [10, 42]", gotImm)
	}

	// Map insert via Activation attribute (mutable and immutable).
	insertNode := &evalMapInsert{
		id:       5,
		mapID:    6,
		overload: "@mapInsert_map_map",
		accu:     accuAttr,
		key:      NewConstValue(7, types.String("k")),
		val:      NewConstValue(8, types.Int(99)),
		adapter:  reg,
		binary:   true,
	}
	mutMap := types.NewMutableMap(reg, nil)
	actMutMap, _ := NewActivation(map[string]any{"accu": mutMap})
	gotMutMap := toImmutableAccu(insertNode.Eval(actMutMap))
	if gotMutMap.Equal(reg.NativeToValue(map[string]int64{"k": 99})) != types.True {
		t.Errorf("evalMapInsert(mutable attr) = %v, want {k: 99}", gotMutMap)
	}

	immMap := reg.NativeToValue(map[string]int64{"a": 1})
	actImmMap, _ := NewActivation(map[string]any{"accu": immMap})
	gotImmMap := insertNode.Eval(actImmMap)
	if gotImmMap.Equal(reg.NativeToValue(map[string]int64{"a": 1, "k": 99})) != types.True {
		t.Errorf("evalMapInsert(immutable attr) = %v, want {a: 1, k: 99}", gotImmMap)
	}
}

func renameComprehensionAccu(a *ast.AST, newAccu string) *ast.AST {
	fac := ast.NewExprFactory()
	comp := a.Expr().AsComprehension()
	oldAccu := comp.AccuVar()
	var rewrite func(e ast.Expr) ast.Expr
	rewrite = func(e ast.Expr) ast.Expr {
		switch e.Kind() {
		case ast.IdentKind:
			if e.AsIdent() == oldAccu {
				return fac.NewIdent(e.ID(), newAccu)
			}
			return e
		case ast.CallKind:
			c := e.AsCall()
			newArgs := make([]ast.Expr, len(c.Args()))
			for i, arg := range c.Args() {
				newArgs[i] = rewrite(arg)
			}
			if c.IsMemberFunction() {
				return fac.NewMemberCall(e.ID(), c.FunctionName(), rewrite(c.Target()), newArgs...)
			}
			return fac.NewCall(e.ID(), c.FunctionName(), newArgs...)
		default:
			return e
		}
	}
	var rewritten ast.Expr
	if comp.HasIterVar2() {
		rewritten = fac.NewComprehensionTwoVar(
			a.Expr().ID(),
			comp.IterRange(),
			comp.IterVar(),
			comp.IterVar2(),
			newAccu,
			comp.AccuInit(),
			rewrite(comp.LoopCondition()),
			rewrite(comp.LoopStep()),
			rewrite(comp.Result()),
		)
	} else {
		rewritten = fac.NewComprehension(
			a.Expr().ID(),
			comp.IterRange(),
			comp.IterVar(),
			newAccu,
			comp.AccuInit(),
			rewrite(comp.LoopCondition()),
			rewrite(comp.LoopStep()),
			rewrite(comp.Result()),
		)
	}
	return ast.NewAST(rewritten, a.SourceInfo())
}

func TestComprehensionStepElisionCostParity(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)
	partAttrs := NewPartialAttributeFactory(cont, reg, reg)
	interp := NewInterpreter(disp, cont, reg, reg, partAttrs)

	partialAct, err := NewPartialActivation(
		map[string]any{
			"xs": []int64{1, 2, 3, 4},
			"m":  map[int64]int64{1: 10, 2: 20},
		},
		NewAttributePattern("unk"),
	)
	if err != nil {
		t.Fatalf("NewPartialActivation() failed: %v", err)
	}

	expressions := []string{
		`xs.map(x, x + 1)`,
		`xs.map(x, 7)`,
		`xs.filter(x, x % 2 == 0)`,
		`xs.map(x, x == 2 ? unk : x + 1)`,
		`xs.map(x, 1 / (x - 2))`,
		`xs.map(x, x == 1 ? unk : 1 / (x - 2))`,
		`compre2(k, v, a, m, {}, true, insert(a, k, v + 1), a)`,
		`compre2(k, v, a, m, {}, true, insert(a, {k: v + 1}), a)`,
		`compre2(i, v, a, xs, {}, true, insert(a, {1: 2}), a)`,
		`compre2(i, v, a, xs, {}, true, v > 1 ? insert(a, {v: v + 1}) : a, a)`,
		`compre2(i, v, a, xs, {}, true, insert(a, {v: v == 1 ? unk : v}), a)`,
		`compre2(i, v, a, xs, {}, true, insert(a, {v: v == 1 ? unk : 1 / (v - 2)}), a)`,
		`compre2(i, v, a, xs, {}, true, insert(a, v, v == 1 ? unk : 1 / (v - 2)), a)`,
	}

	for _, optimize := range []bool{false, true} {
		for _, exhaustive := range []bool{false, true} {
			for _, customTracker := range []bool{false, true} {
				for _, expr := range expressions {
					parsedElided := parseMutabilityExpr(t, expr)
					// Legacy "__result__" accumulator disables mutableAccu & step elision, giving an exact
					// un-elided reference cost on the identical AST structure.
					parsedUnelided := renameComprehensionAccu(parsedElided, "__result__")

					makeTracker := func(dst **CostTracker) PlannerOption {
						return CostObserver(CostTrackerFactory(func() (*CostTracker, error) {
							var opts []CostTrackerOption
							if customTracker {
								opts = append(opts,
									OverloadCostTracker(overloads.AddList, func(args []ref.Val, res ref.Val) *uint64 {
										rhsSize := uint64(0)
										if l, ok := args[1].(traits.Lister); ok {
											rhsSize = uint64(l.Size().(types.Int))
										}
										c := 3 + rhsSize
										return &c
									}),
									OverloadCostTracker("", func(args []ref.Val, res ref.Val) *uint64 {
										c := uint64(5 + len(args))
										return &c
									}),
								)
							}
							tr, err := NewCostTracker(nil, opts...)
							*dst = tr
							return tr, err
						}))
					}

					var elidedTracker, refTracker *CostTracker
					elidedOpts := []PlannerOption{makeTracker(&elidedTracker)}
					refOpts := []PlannerOption{makeTracker(&refTracker)}
					if optimize {
						elidedOpts = append(elidedOpts, Optimize())
						refOpts = append(refOpts, Optimize())
					}
					if exhaustive {
						elidedOpts = append(elidedOpts, ExhaustiveEval())
						refOpts = append(refOpts, ExhaustiveEval())
					}

					elidedPrg, err := interp.NewInterpretable(parsedElided, elidedOpts...)
					if err != nil {
						t.Fatalf("NewInterpretable(elided %q) failed: %v", expr, err)
					}
					refPrg, err := interp.NewInterpretable(parsedUnelided, refOpts...)
					if err != nil {
						t.Fatalf("NewInterpretable(ref %q) failed: %v", expr, err)
					}

					gotVal := elidedPrg.Eval(partialAct)
					wantVal := refPrg.Eval(partialAct)
					if gotErr, wantErr := types.IsError(gotVal), types.IsError(wantVal); gotErr != wantErr {
						t.Errorf("%q (opt=%v exh=%v custom=%v): val = %v, want %v", expr, optimize, exhaustive, customTracker, gotVal, wantVal)
					} else if !gotErr && !types.IsUnknown(gotVal) && gotVal.Equal(wantVal) != types.True {
						t.Errorf("%q (opt=%v exh=%v custom=%v): val = %v, want %v", expr, optimize, exhaustive, customTracker, gotVal, wantVal)
					}
					if elidedTracker.ActualCost() != refTracker.ActualCost() {
						t.Errorf("%q (opt=%v exh=%v custom=%v): ActualCost = %d, want %d",
							expr, optimize, exhaustive, customTracker, elidedTracker.ActualCost(), refTracker.ActualCost())
					}
				}
			}
		}
	}
}

func TestComprehensionStepElisionAllocations(t *testing.T) {
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	addMapInsertBinding(t, disp)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	xs := make([]int64, 100)
	for i := range xs {
		xs[i] = int64(i)
	}
	act, err := NewActivation(map[string]any{"xs": xs})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}

	// Verify xs.map(x, x) and compre2(..., insert(a, {i: v}), ...) do not allocate an intermediate
	// [x] or {i: v} container per element (~O(log N) slice/bucket growth allocs instead of >= N).
	for _, expr := range []string{
		`xs.map(x, x)`,
		`compre2(i, v, a, xs, {}, true, insert(a, {i: v}), a)`,
	} {
		parsed := parseMutabilityExpr(t, expr)
		prg, err := interp.NewInterpretable(parsed)
		if err != nil {
			t.Fatalf("NewInterpretable(%q) failed: %v", expr, err)
		}
		allocs := testing.AllocsPerRun(100, func() {
			_ = prg.Eval(act)
		})
		if allocs >= 20 {
			t.Errorf("%q allocated %v times for N=100 (expected < 20 without per-step container allocs)", expr, allocs)
		}

		// With standard CostObserver enabled, rVal is not synthesized (only CostTracker.EvalBinary's
		// 1 call-arg buffer per step allocates, saving the >= 2 per-step container allocations).
		costOpt := CostObserver(CostTrackerFactory(func() (*CostTracker, error) {
			return NewCostTracker(nil)
		}))
		costPrg, err := interp.NewInterpretable(parsed, costOpt)
		if err != nil {
			t.Fatalf("NewInterpretable(%q, costOpt) failed: %v", expr, err)
		}
		costAllocs := testing.AllocsPerRun(100, func() {
			_ = costPrg.Eval(act)
		})
		if costAllocs >= 130 {
			t.Errorf("%q (with standard CostObserver) allocated %v times for N=100 (expected < 130 without synthesized container allocs)", expr, costAllocs)
		}
	}
}
