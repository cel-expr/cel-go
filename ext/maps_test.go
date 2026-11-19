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

package ext

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
)

func TestMapsPutAll(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars []cel.EnvOption
		in   map[string]any
	}{
		{
			name: "both empty",
			expr: `{}.putAll({}) == {}`,
		},
		{
			name: "disjoint keys",
			expr: `{'a': 1}.putAll({'b': 2}) == {'a': 1, 'b': 2}`,
		},
		{
			name: "empty left",
			expr: `{}.putAll({'b': 2}) == {'b': 2}`,
		},
		{
			name: "empty right",
			expr: `{'a': 1}.putAll({}) == {'a': 1}`,
		},
		{
			name: "conflicting key takes the second value",
			expr: `{'a': 1}.putAll({'a': 2}) == {'a': 2}`,
		},
		{
			name: "conflict among other keys",
			expr: `{'a': 1, 'b': 2}.putAll({'b': 3, 'c': 4}) == {'a': 1, 'b': 3, 'c': 4}`,
		},
		{
			name: "map values are replaced, not combined",
			expr: `{'a': {'x': 1}}.putAll({'a': {'y': 2}}) == {'a': {'y': 2}}`,
		},
		{
			name: "int keys",
			expr: `{1: 'a'}.putAll({2: 'b'}) == {1: 'a', 2: 'b'}`,
		},
		{
			name: "inputs are unchanged",
			expr: `{'a': 1}.putAll({'a': 2}) == {'a': 2} && {'a': 1} == {'a': 1}`,
		},
		{
			name: "putAll with a variable",
			expr: `x.putAll({'b': 2}) == {'a': 1, 'b': 2}`,
			vars: []cel.EnvOption{cel.Variable("x", cel.MapType(cel.StringType, cel.IntType))},
			in:   map[string]any{"x": map[string]int64{"a": 1}},
		},
		{
			name: "putAll is associative when keys are disjoint",
			expr: `{'a': 1}.putAll({'b': 2}).putAll({'c': 3}) ==
			       {'a': 1}.putAll({'b': 2}.putAll({'c': 3}))`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]cel.EnvOption{Maps()}, tc.vars...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
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
			if out != types.True {
				t.Errorf("prg.Eval(%q) got %v, wanted true", tc.expr, out)
			}
		})
	}
}

func TestMapsPutAllTypeChecking(t *testing.T) {
	env, err := cel.NewEnv(Maps())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		name string
		expr string
	}{
		{name: "list argument", expr: `[1].putAll([2])`},
		{name: "string argument", expr: `'a'.putAll('b')`},
		{name: "mixed arguments", expr: `{'a': 1}.putAll([2])`},
		{name: "single argument", expr: `{'a': 1}.putAll()`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if iss.Err() == nil {
				t.Errorf("env.Compile(%q) succeeded, wanted a type-checking error", tc.expr)
			}
		})
	}
}

func TestMapsPutAllNonMapArgs(t *testing.T) {
	env, err := cel.NewEnv(Maps(),
		cel.Variable("x", cel.DynType),
		cel.Variable("y", cel.DynType))
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		name string
		in   map[string]any
	}{
		{name: "left is not a map", in: map[string]any{"x": []int64{1}, "y": map[string]int64{"a": 1}}},
		{name: "right is not a map", in: map[string]any{"x": map[string]int64{"a": 1}, "y": "b"}},
		{name: "neither is a map", in: map[string]any{"x": 1, "y": 2}},
	}
	ast, iss := env.Compile(`x.putAll(y)`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := prg.Eval(tc.in)
			if err == nil {
				t.Errorf("prg.Eval(%v) succeeded, wanted an error", tc.in)
			}
		})
	}
}

func TestMapsCosts(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost cost.CostEstimate
		// estimatedCostV0 is the estimate under cost.ModelVersion0, set only where the revision moved it.
		estimatedCostV0 *cost.CostEstimate
		actualCost      uint64
		// actualCostV0 is the cost tracked under cost.ModelVersion0, set only where the revision moved it.
		actualCostV0 *uint64
	}{
		{
			name:          "map_put_all_literal",
			expr:          `{'a': 1, 'b': 2}.putAll({'c': 3}) == {'a': 1, 'b': 2, 'c': 3}`,
			estimatedCost: cost.FixedCostEstimate(125),
			actualCost:    125,
		},
		{
			name: "map_put_all_var",
			expr: `x.putAll(y) == {'a': 1, 'b': 3, 'c': 4}`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.MapType(cel.StringType, cel.IntType)),
				cel.Variable("y", cel.MapType(cel.StringType, cel.IntType)),
			},
			in:            map[string]any{"x": map[string]int64{"a": 1, "b": 2}, "y": map[string]int64{"b": 3, "c": 4}},
			hints:         map[string]uint64{"x": 3, "y": 2},
			estimatedCost: cost.RangedCostEstimate(64, 69),
			actualCost:    68,
		},
		{
			// The result holds at most 3 + 2 entries, which bounds the iterations of
			// all(). Comprehensions are priced differently at version 0.
			name: "map_put_all_var_result_scanned",
			expr: `x.putAll(y).all(k, k != '')`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.MapType(cel.StringType, cel.IntType)),
				cel.Variable("y", cel.MapType(cel.StringType, cel.IntType)),
			},
			in:              map[string]any{"x": map[string]int64{"a": 1, "b": 2}, "y": map[string]int64{"b": 3, "c": 4}},
			hints:           map[string]uint64{"x": 3, "y": 2},
			estimatedCost:   cost.RangedCostEstimate(34, 64),
			estimatedCostV0: costV0(34, 59),
			actualCost:      53,
			actualCostV0:    actualV0(50),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env, err := cel.NewEnv(append([]cel.EnvOption{Maps()}, tc.vars...)...)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			testCheckCost(t, env, ast, tc.hints, tc.estimatedCost, tc.estimatedCostV0)
			testEvalWithCost(t, env, ast, tc.in, tc.actualCost, ledgerV0(tc.actualCostV0)...)
		})
	}
}

func TestMapsPutAllResultSize(t *testing.T) {
	// putAll over a 2-entry and a 1-entry map holds 2 entries when every key
	// collides and 3 when none do.
	estimate := mapPutAllCostModel("map_put_all_map").FunctionEstimator()
	var target cost.AstNode = testSizedNode{size: 2}
	est := estimate(testCostHintEstimator{}, &target, []cost.AstNode{
		testSizedNode{size: 1},
	})
	if est == nil {
		t.Fatal("estimate() returned nil")
	}
	if est.ResultSize == nil {
		t.Fatal("estimate() ResultSize was nil")
	}
	if est.ResultSize.Min != 2 || est.ResultSize.Max != 3 {
		t.Errorf("ResultSize got [%d, %d], wanted [2, 3]",
			est.ResultSize.Min, est.ResultSize.Max)
	}
}

// testSizedNode is a checker.AstNode whose size is known, used to pin the
// result-size bounds of the putAll estimator.
type testSizedNode struct {
	size uint64
}

func (n testSizedNode) Path() []string { return nil }

func (n testSizedNode) Type() *cel.Type { return cel.MapType(cel.StringType, cel.IntType) }

func (n testSizedNode) Expr() ast.Expr { return nil }

func (n testSizedNode) ComputedSize() *cost.SizeEstimate {
	sz := cost.FixedSizeEstimate(n.size)
	return &sz
}

func TestMapsVersion(t *testing.T) {
	_, err := cel.NewEnv(Maps(MapsVersion(0)))
	if err != nil {
		t.Fatalf("MapsVersion(0) failed: %v", err)
	}
}
