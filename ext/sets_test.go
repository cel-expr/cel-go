// Copyright 2023 Google LLC
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

package ext

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestSets(t *testing.T) {
	tests := []struct {
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost cost.CostEstimate
		actualCost    uint64
	}{
		// set containment
		{
			expr:  `sets.contains(x, [1, 2, 3])`,
			vars:  []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
			in:    map[string]any{"x": []int64{5, 4, 3, 2, 1}},
			hints: map[string]uint64{"x": 10},
			// min cost is input 'x' length 0, 10 for list creation, 2 for arg costs
			// max cost is input 'x' length 10, 10 for list creation, 2 for arg costs
			estimatedCost: cost.CostEstimate{Min: 12, Max: 42},
			// actual cost is 'x' length 5 * list literal length 3, 10 for list creation, 2 for arg cost
			actualCost: 27,
		},
		{
			expr: `sets.contains(x, [1, 1, 1, 1, 1])`,
			vars: []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
			in:   map[string]any{"x": []int64{5, 4, 3, 2, 1}},
			// min cost is input 'x' length 0, 10 for list creation, 2 for arg costs
			// max cost is effectively infinite due to missing size hint for 'x'
			estimatedCost: cost.CostEstimate{Min: 12, Max: math.MaxUint64},
			// actual cost is 'x' length 5 * list literal length 5, 10 for list creation, 2 for arg cost
			actualCost: 37,
		},
		{
			expr:          `sets.contains([], [])`,
			estimatedCost: cost.FixedCostEstimate(21),
			actualCost:    21,
		},
		{
			expr:          `sets.contains([1], [])`,
			estimatedCost: cost.FixedCostEstimate(21),
			actualCost:    21,
		},
		{
			expr:          `sets.contains([1], [1])`,
			estimatedCost: cost.FixedCostEstimate(22),
			actualCost:    22,
		},
		{
			expr:          `sets.contains([1], [1, 1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.contains([1, 1], [1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.contains([2, 1], [1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.contains([1, 2, 3, 4], [2, 3])`,
			estimatedCost: cost.FixedCostEstimate(29),
			actualCost:    29,
		},
		{
			expr:          `sets.contains([1], [1.0, 1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.contains([1, 2], [2u, 2.0])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			expr:          `sets.contains([1, 2u], [2, 2.0])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			expr:          `sets.contains([1, 2.0, 3u], [1.0, 2u, 3])`,
			estimatedCost: cost.FixedCostEstimate(30),
			actualCost:    30,
		},
		{
			expr: `sets.contains([[1], [2, 3]], [[2, 3.0]])`,
			// 10 for each list creation, top-level list sizes are 2, 1
			estimatedCost: cost.FixedCostEstimate(53),
			actualCost:    53,
		},
		{
			expr:          `!sets.contains([1], [2])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `!sets.contains([1], [1, 2])`,
			estimatedCost: cost.FixedCostEstimate(24),
			actualCost:    24,
		},
		{
			expr:          `!sets.contains([1], ["1", 1])`,
			estimatedCost: cost.FixedCostEstimate(24),
			actualCost:    24,
		},
		{
			expr:          `!sets.contains([1], [1.1, 1u])`,
			estimatedCost: cost.FixedCostEstimate(24),
			actualCost:    24,
		},

		// set equivalence (note the cost factor is higher as it's basically two contains checks)
		{
			expr:          `sets.equivalent([], [])`,
			estimatedCost: cost.FixedCostEstimate(21),
			actualCost:    21,
		},
		{
			expr:          `sets.equivalent([1], [1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.equivalent([1], [1, 1])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			expr:          `sets.equivalent([1, 1], [1])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			expr:          `sets.equivalent([1], [1u, 1.0])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			expr:          `sets.equivalent([1], [1u, 1.0])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			expr:          `sets.equivalent([1, 2, 3], [3u, 2.0, 1])`,
			estimatedCost: cost.FixedCostEstimate(39),
			actualCost:    39,
		},
		{
			expr:          `sets.equivalent([[1.0], [2, 3]], [[1], [2, 3.0]])`,
			estimatedCost: cost.FixedCostEstimate(69),
			actualCost:    69,
		},
		{
			expr:          `!sets.equivalent([2, 1], [1])`,
			estimatedCost: cost.FixedCostEstimate(26),
			actualCost:    26,
		},
		{
			expr:          `!sets.equivalent([1], [1, 2])`,
			estimatedCost: cost.FixedCostEstimate(26),
			actualCost:    26,
		},
		{
			expr:          `!sets.equivalent([1, 2], [2u, 2, 2.0])`,
			estimatedCost: cost.FixedCostEstimate(34),
			actualCost:    34,
		},
		{
			expr:          `!sets.equivalent([1, 2], [1u, 2, 2.3])`,
			estimatedCost: cost.FixedCostEstimate(34),
			actualCost:    34,
		},

		// set intersection
		{
			expr:          `sets.intersects([1], [1])`,
			estimatedCost: cost.FixedCostEstimate(22),
			actualCost:    22,
		},
		{
			expr:          `sets.intersects([1], [1, 1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.intersects([1, 1], [1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.intersects([2, 1], [1])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.intersects([1], [1, 2])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.intersects([1], [1.0, 2])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `sets.intersects([1, 2], [2u, 2, 2.0])`,
			estimatedCost: cost.FixedCostEstimate(27),
			actualCost:    27,
		},
		{
			expr:          `sets.intersects([1, 2], [1u, 2, 2.3])`,
			estimatedCost: cost.FixedCostEstimate(27),
			actualCost:    27,
		},
		{
			expr:          `sets.intersects([[1], [2, 3]], [[1, 2], [2, 3.0]])`,
			estimatedCost: cost.FixedCostEstimate(65),
			actualCost:    65,
		},
		{
			expr:          `!sets.intersects([], [])`,
			estimatedCost: cost.FixedCostEstimate(22),
			actualCost:    22,
		},
		{
			expr:          `!sets.intersects([1], [])`,
			estimatedCost: cost.FixedCostEstimate(22),
			actualCost:    22,
		},
		{
			expr:          `!sets.intersects([1], [2])`,
			estimatedCost: cost.FixedCostEstimate(23),
			actualCost:    23,
		},
		{
			expr:          `!sets.intersects([1], ["1", 2])`,
			estimatedCost: cost.FixedCostEstimate(24),
			actualCost:    24,
		},
		{
			expr:          `!sets.intersects([1], [1.1, 2u])`,
			estimatedCost: cost.FixedCostEstimate(24),
			actualCost:    24,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			env := testSetsEnv(t, tc.vars...)
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}

			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost)
			}
		})
	}
}

func TestSetsMembershipRewriter(t *testing.T) {
	tests := []struct {
		expr      string
		optimized string
		opts      []cel.EnvOption
		in        map[string]any
		out       ref.Val
	}{
		{
			expr:      `a in [1, 2, 3, 4]`,
			optimized: `a in {1: true, 2: true, 3: true, 4: true}`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
			},
			in: map[string]any{
				"a": 3,
			},
			out: types.True,
		},
		{
			expr:      `a in ['1', '2', '3', 4]`,
			optimized: `a in {"1": true, "2": true, "3": true, 4: true}`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
			},
			in: map[string]any{
				"a": 3,
			},
			out: types.False,
		},
		{
			expr:      `a in [1u, '2', '3', 4]`,
			optimized: `a in {1u: true, "2": true, "3": true, 4: true}`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
			},
			in: map[string]any{
				"a": 4,
			},
			out: types.True,
		},
		{
			expr:      `a in [1u, 2.0, '3', 4]`,
			optimized: `a in [1u, 2.0, "3", 4]`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
			},
			in: map[string]any{
				"a": 4,
			},
			out: types.True,
		},
		{
			expr:      `a in [b, 32]`,
			optimized: `a in [b, 32]`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
				cel.Variable("b", cel.IntType),
			},
			in: map[string]any{
				"a": 4,
				"b": 4,
			},
			out: types.True,
		},
		{
			expr:      `a in {b: c}`,
			optimized: `a in {b: c}`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
				cel.Variable("b", cel.IntType),
				cel.Variable("c", cel.IntType),
			},
			in: map[string]any{
				"a": 4,
				"b": 42,
				"c": 123,
			},
			out: types.False,
		},
		{
			expr:      `a in {3: true}`,
			optimized: `a in {3: true}`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.IntType),
			},
			in: map[string]any{
				"a": 4,
			},
			out: types.False,
		},
		{
			expr:      `a in ["hello", "world"].map(i, i in ["goodbye", "world"], i + i)`,
			optimized: `a in ["hello", "world"].map(i, i in {"goodbye": true, "world": true}, i + i)`,
			opts: []cel.EnvOption{
				cel.Variable("a", cel.StringType),
			},
			in: map[string]any{
				"a": "worldworld",
			},
			out: types.True,
		},
		{
			expr:      `a in [test.GlobalEnum.GOO, test.GlobalEnum.GAR, test.GlobalEnum.GAZ]`,
			optimized: `a in {0: true, 1: true, 2: true}`,
			opts: []cel.EnvOption{
				cel.Container("google.expr.proto3"),
				cel.Variable("a", cel.IntType),
				cel.Types(&proto3pb.TestAllTypes{}),
			},
			in: map[string]any{
				"a": proto3pb.GlobalEnum_GAZ,
			},
			out: types.True,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			env := testSetsEnv(t, tc.opts...)
			var asts []*cel.Ast
			a, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, a)
			setsOpt, err := NewSetMembershipOptimizer()
			if err != nil {
				t.Fatalf("NewSetMembershipOptimizer() failed with error: %v", err)
			}
			opt, err := cel.NewStaticOptimizer(setsOpt)
			if err != nil {
				t.Fatalf("cel.NewStaticOptimizer() failed with error: %v", err)
			}
			optAST, iss := opt.Optimize(env, a)
			if iss.Err() != nil {
				t.Fatalf("opt.Optimize() failed: %v", iss.Err())
			}
			optExpr, err := cel.AstToString(optAST)
			if err != nil {
				t.Fatalf("cel.AstToString() failed :%v", err)
			}
			if tc.optimized != optExpr {
				t.Errorf("got %v, wanted optimized expr %v", optExpr, tc.optimized)
			}
			asts = append(asts, optAST)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				in := tc.in
				if in == nil {
					in = map[string]any{}
				}
				out, _, err := prg.Eval(in)
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out != tc.out {
					t.Errorf("prg.Eval() got %v, wanted %v for expr: %s", out, tc.out, tc.expr)
				}
			}
		})
	}
}

func TestSetsVersion(t *testing.T) {
	_, err := cel.NewEnv(Sets(SetsVersion(0)))
	if err != nil {
		t.Fatalf("SetsVersion(0) failed: %v", err)
	}
}

func testSetsEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{cel.EnableMacroCallTracking(), Sets()}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Sets()) failed: %v", err)
	}
	return env
}

func testCheckCost(t *testing.T, env *cel.Env, ast *cel.Ast, hints map[string]uint64, wantEst cost.CostEstimate) {
	t.Helper()
	if len(hints) == 0 {
		hints = map[string]uint64{}
	}
	est, err := env.EstimateCost(ast, testCostHintEstimator{hints: hints})
	if err != nil {
		t.Fatalf("env.EstimateCost() failed: %v", err)
	}
	if !reflect.DeepEqual(est, wantEst) {
		t.Errorf("env.EstimateCost() got %v, wanted %v", est, wantEst)
	}
}

func testEvalWithCost(t *testing.T, env *cel.Env, ast *cel.Ast, in any, wantCost uint64) {
	t.Helper()
	prgOpts := []cel.ProgramOption{}
	if ast.IsChecked() {
		prgOpts = append(prgOpts, cel.CostTracking(nil))
	}
	prg, err := env.Program(ast, prgOpts...)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	var data any = cel.NoVars()
	if in != nil {
		data = in
	}
	out, det, err := prg.Eval(data)
	if err != nil {
		t.Fatal(err)
	} else if out.Value() != true {
		t.Errorf("got %v, wanted true", out.Value())
	}
	if det.ActualCost() != nil && *det.ActualCost() != wantCost {
		t.Errorf("prg.Eval() had cost %v, wanted %v", *det.ActualCost(), wantCost)
	}
}

type testCostHintEstimator struct {
	hints map[string]uint64
}

func (tc testCostHintEstimator) EstimateSize(element cost.AstNode) *cost.SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &cost.SizeEstimate{Min: 0, Max: l}
	}
	return nil
}

func (testCostHintEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	return nil
}
