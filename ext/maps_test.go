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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
)

func TestMapsMerge(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars []cel.EnvOption
		in   map[string]any
	}{
		{
			name: "both empty",
			expr: `{}.merge({}) == {}`,
		},
		{
			name: "disjoint keys",
			expr: `{'a': 1}.merge({'b': 2}) == {'a': 1, 'b': 2}`,
		},
		{
			name: "empty left",
			expr: `{}.merge({'b': 2}) == {'b': 2}`,
		},
		{
			name: "empty right",
			expr: `{'a': 1}.merge({}) == {'a': 1}`,
		},
		{
			name: "conflicting key takes the second value",
			expr: `{'a': 1}.merge({'a': 2}) == {'a': 2}`,
		},
		{
			name: "conflict among other keys",
			expr: `{'a': 1, 'b': 2}.merge({'b': 3, 'c': 4}) == {'a': 1, 'b': 3, 'c': 4}`,
		},
		{
			name: "map values are replaced, not merged",
			expr: `{'a': {'x': 1}}.merge({'a': {'y': 2}}) == {'a': {'y': 2}}`,
		},
		{
			name: "int keys",
			expr: `{1: 'a'}.merge({2: 'b'}) == {1: 'a', 2: 'b'}`,
		},
		{
			name: "inputs are unchanged",
			expr: `{'a': 1}.merge({'a': 2}) == {'a': 2} && {'a': 1} == {'a': 1}`,
		},
		{
			name: "merging a variable",
			expr: `x.merge({'b': 2}) == {'a': 1, 'b': 2}`,
			vars: []cel.EnvOption{cel.Variable("x", cel.MapType(cel.StringType, cel.IntType))},
			in:   map[string]any{"x": map[string]int64{"a": 1}},
		},
		{
			name: "merge is associative when keys are disjoint",
			expr: `{'a': 1}.merge({'b': 2}).merge({'c': 3}) ==
			       {'a': 1}.merge({'b': 2}.merge({'c': 3}))`,
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

func TestMapsMergeTypeChecking(t *testing.T) {
	env, err := cel.NewEnv(Maps())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		name string
		expr string
	}{
		{name: "list argument", expr: `[1].merge([2])`},
		{name: "string argument", expr: `'a'.merge('b')`},
		{name: "mixed arguments", expr: `{'a': 1}.merge([2])`},
		{name: "single argument", expr: `{'a': 1}.merge()`},
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

func TestMapsMergeNonMapArgs(t *testing.T) {
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
	ast, iss := env.Compile(`x.merge(y)`)
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

func TestMapsMergeCost(t *testing.T) {
	env, err := cel.NewEnv(Maps())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile(`{'a': 1, 'b': 2}.merge({'c': 3})`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	est, err := env.EstimateCost(ast, testCostHintEstimator{})
	if err != nil {
		t.Fatalf("env.EstimateCost() failed: %v", err)
	}
	// The estimate must scale with the inputs rather than collapse to the
	// default O(1) bucket a function without an estimator would land in.
	if est.Min <= 1 {
		t.Errorf("env.EstimateCost() min got %d, wanted a size-dependent estimate", est.Min)
	}
	prg, err := env.Program(ast, cel.CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, det, err := prg.Eval(map[string]any{})
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	actual := det.ActualCost()
	if actual == nil {
		t.Fatal("det.ActualCost() was nil, wanted a tracked cost")
	}
	if *actual < est.Min || *actual > est.Max {
		t.Errorf("det.ActualCost() got %d, outside the estimate [%d, %d]", *actual, est.Min, est.Max)
	}
}

func TestMapsMergeResultSize(t *testing.T) {
	// A merge of a 2-entry and a 1-entry map holds 2 entries when every key
	// collides and 3 when none do.
	var target checker.AstNode = testSizedNode{size: 2}
	est := estimateMapsMergeCost(testCostHintEstimator{}, &target, []checker.AstNode{
		testSizedNode{size: 1},
	})
	if est == nil {
		t.Fatal("estimateMapsMergeCost() returned nil")
	}
	if est.ResultSize == nil {
		t.Fatal("estimateMapsMergeCost() ResultSize was nil")
	}
	if est.ResultSize.Min != 2 || est.ResultSize.Max != 3 {
		t.Errorf("ResultSize got [%d, %d], wanted [2, 3]",
			est.ResultSize.Min, est.ResultSize.Max)
	}
}

// testSizedNode is a checker.AstNode whose size is known, used to pin the
// result-size bounds of the merge estimator.
type testSizedNode struct {
	size uint64
}

func (n testSizedNode) Path() []string { return nil }

func (n testSizedNode) Type() *cel.Type { return cel.MapType(cel.StringType, cel.IntType) }

func (n testSizedNode) Expr() ast.Expr { return nil }

func (n testSizedNode) ComputedSize() *checker.SizeEstimate {
	sz := checker.SizeEstimate{Min: n.size, Max: n.size}
	return &sz
}

func TestMapsVersion(t *testing.T) {
	_, err := cel.NewEnv(Maps(MapsVersion(0)))
	if err != nil {
		t.Fatalf("MapsVersion(0) failed: %v", err)
	}
}
