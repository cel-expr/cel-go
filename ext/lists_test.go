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
	"fmt"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/types"

	proto2pb "github.com/google/cel-go/test/proto2pb"
)

func TestLists(t *testing.T) {
	listsTests := []struct {
		expr string
		err  string
	}{
		{expr: `lists.range(4) == [0,1,2,3]`},
		{expr: `lists.range(0) == []`},
		{expr: `[5,1,2,3].reverse() == [3,2,1,5]`},
		{expr: `[].reverse() == []`},
		{expr: `[1].reverse() == [1]`},
		{expr: `['are', 'you', 'as', 'bored', 'as', 'I', 'am'].reverse() == ['am', 'I', 'as', 'bored', 'as', 'you', 'are']`},
		{expr: `[false, true, true].reverse().reverse() == [false, true, true]`},
		{expr: `[1,2,3,4].slice(0, 4) == [1,2,3,4]`},
		{expr: `[1,2,3,4].slice(0, 0) == []`},
		{expr: `[1,2,3,4].slice(1, 1) == []`},
		{expr: `[1,2,3,4].slice(4, 4) == []`},
		{expr: `[1,2,3,4].slice(1, 3) == [2, 3]`},
		{expr: `[1,2,3,4].slice(3, 0)`, err: "cannot slice(3, 0), start index must be less than or equal to end index"},
		{expr: `[1,2,3,4].slice(0, 10)`, err: "cannot slice(0, 10), list is length 4"},
		{expr: `[1,2,3,4].slice(-5, 10)`, err: "cannot slice(-5, 10), negative indexes not supported"},
		{expr: `[1,2,3,4].slice(-5, -3)`, err: "cannot slice(-5, -3), negative indexes not supported"},

		{expr: `dyn([]).flatten() == []`},
		{expr: `dyn([1,2,3,4]).flatten() == [1,2,3,4]`},
		{expr: `[1,[2,[3,4]]].flatten() == [1,2,[3,4]]`},
		{expr: `[1,2,[],[],[3,4]].flatten() == [1,2,3,4]`},
		{expr: `[1,[2,[3,4]]].flatten(2) == [1,2,3,4]`},
		{expr: `[1,[2,[3,[4]]]].flatten(-1) == [1,2,3,4]`, err: "level must be non-negative"},
		{expr: `[].sort() == []`},
		{expr: `[1].sort() == [1]`},
		{expr: `[4, 3, 2, 1].sort() == [1, 2, 3, 4]`},
		{expr: `["d", "a", "b", "c"].sort() == ["a", "b", "c", "d"]`},
		{expr: `["d", 3, 2, "c"].sort() == ["a", "b", "c", "d"]`, err: "list elements must have the same type"},
		{expr: `[].sortBy(e, e) == []`},
		{expr: `["a"].sortBy(e, e) == ["a"]`},
		{expr: `[-3, 1, -5, -2, 4].sortBy(e, -(e * e)) == [-5, 4, -3, -2, 1]`},
		{expr: `[-3, 1, -5, -2, 4].map(e, e * 2).sortBy(e, -(e * e)) == [-10, 8, -6, -4, 2]`},
		{expr: `lists.range(3).sortBy(e, -e) == [2, 1, 0]`},
		{expr: `["a", "c", "b", "first"].sortBy(e, e == "first" ? "" : e) == ["first", "a", "b", "c"]`},
		{expr: `[ExampleType{name: 'foo'}, ExampleType{name: 'bar'}, ExampleType{name: 'baz'}].sortBy(e, e.name) == [ExampleType{name: 'bar'}, ExampleType{name: 'baz'}, ExampleType{name: 'foo'}]`},
		{expr: `[].distinct() == []`},
		{expr: `[1].distinct() == [1]`},
		{expr: `[-2, 5, -2, 1, 1, 5, -2, 1].distinct() == [-2, 5, 1]`},
		{expr: `['c', 'a', 'a', 'b', 'a', 'b', 'c', 'c'].distinct() == ['c', 'a', 'b']`},
		{expr: `[1, 2.0, "c", 3, "c", 1].distinct() == [1, 2.0, "c", 3]`},
		{expr: `[1, 1.0, 2].distinct() == [1, 2]`},
		{expr: `[[1], [1], [2]].distinct() == [[1], [2]]`},
		{expr: `[ExampleType{name: 'a'}, ExampleType{name: 'b'}, ExampleType{name: 'a'}].distinct() == [ExampleType{name: 'a'}, ExampleType{name: 'b'}]`},
	}

	env := testListsEnv(t, 0)
	for i, tst := range listsTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
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
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if tc.err != "" {
					if err == nil {
						t.Fatalf("got value %v, wanted error %s for expr: %s",
							out.Value(), tc.err, tc.expr)
					}
					if !strings.Contains(err.Error(), tc.err) {
						t.Errorf("got error %v, wanted error %s for expr: %s", err, tc.err, tc.expr)
					}
				} else if err != nil {
					t.Fatal(err)
				} else if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestListsRuntimeErrors(t *testing.T) {
	env, err := cel.NewEnv(Lists(ListsVersion(1)))
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	listsTests := []struct {
		expr string
		err  string
	}{
		{
			expr: "dyn({}).flatten()",
			err:  "no such overload",
		},
		{
			expr: "dyn({}).flatten(0)",
			err:  "no such overload",
		},
		{
			expr: "[].flatten(-1)",
			err:  "level must be non-negative",
		},
		{
			expr: "[].flatten(dyn('1'))",
			err:  "no such overload",
		},
	}
	for i, tst := range listsTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			_, _, err = prg.Eval(cel.NoVars())
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.err)
			}
		})
	}
}

func TestListsVersion(t *testing.T) {
	versionCases := []struct {
		version            uint32
		supportedFunctions map[string]string
	}{
		{
			version: 0,
			supportedFunctions: map[string]string{
				"slice": "[1, 2, 3, 4, 5].slice(2, 4) == [3, 4]",
			},
		},
		{
			version: 1,
			supportedFunctions: map[string]string{
				"flatten": "[[1, 2], [3, 4]].flatten() == [1, 2, 3, 4]",
			},
		},
		{
			version: 2,
			supportedFunctions: map[string]string{
				"distinct": "[1, 2, 2, 1].distinct() == [1, 2]",
				"range":    "lists.range(5) == [0, 1, 2, 3, 4]",
				"reverse":  "[1, 2, 3].reverse() == [3, 2, 1]",
				"sort":     "[2, 1, 3].sort() == [1, 2, 3]",
				"sortBy":   "[{'field': 'lo'}, {'field': 'hi'}].sortBy(m, m.field) == [{'field': 'hi'}, {'field': 'lo'}]",
			},
		},
	}
	for _, lib := range versionCases {
		env, err := cel.NewEnv(Lists(ListsVersion(lib.version)))
		if err != nil {
			t.Fatalf("cel.NewEnv(Lists(ListsVersion(%d))) failed: %v", lib.version, err)
		}
		t.Run(fmt.Sprintf("version=%d", lib.version), func(t *testing.T) {
			for _, tc := range versionCases {
				for name, expr := range tc.supportedFunctions {
					supported := lib.version >= tc.version
					t.Run(fmt.Sprintf("%s-supported=%t", name, supported), func(t *testing.T) {
						ast, iss := env.Compile(expr)
						if supported {
							if iss.Err() != nil {
								t.Errorf("unexpected error: %v", iss.Err())
							}
						} else {
							if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "undeclared reference") {
								t.Errorf("got error %v, wanted error %s for expr: %s, version: %d", iss.Err(), "undeclared reference", expr, tc.version)
							}
							return
						}
						prg, err := env.Program(ast)
						if err != nil {
							t.Fatalf("env.Program() failed: %v", err)
						}
						out, _, err := prg.Eval(cel.NoVars())
						if err != nil {
							t.Fatalf("prg.Eval() failed: %v", err)
						}
						if out != types.True {
							t.Errorf("prg.Eval() got %v, wanted true", out)
						}
					})
				}
			}
		})
	}
}

func TestListsCosts(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost checker.CostEstimate
		actualCost    uint64
		version       int
	}{
		{
			// (1 array alloc + internal alloc) * 10
			// + size(list)
			// + 2 calls
			name:          "list_range",
			expr:          `lists.range(4) == [0, 1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(26),
			actualCost:    26,
		},
		{
			name:          "list_range_computed",
			expr:          `lists.range(4 / 2) == [0, 1]`,
			estimatedCost: checker.FixedCostEstimate(18446744073709551615),
			actualCost:    25,
		},
		{
			name:          "list_range_var",
			expr:          `lists.range(x) == [0, 1, 2, 3, 4]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.IntType)},
			in:            map[string]any{"x": 5},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: checker.FixedCostEstimate(18446744073709551615),
			actualCost:    28,
		},
		{
			// (3 array allocs + internal alloc) * 10 + size(list) + 2 calls
			name:          "list_flatten_depth_one",
			expr:          `[[1, 2], 3].flatten(1) == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(44),
			actualCost:    44,
			version:       3,
		},
		{
			name:          "list_flatten_depth_one_v4",
			expr:          `[[1, 2], 3].flatten(1) == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			// (3 array allocs + internal alloc) * 10 + size(list) * 2 + 2 calls
			name:          "list_flatten_depth_two",
			expr:          `[[1, 2], 3].flatten(2) == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(46),
			actualCost:    46,
			version:       3,
		},
		{
			name:          "list_flatten_depth_two_v4",
			expr:          `[[1, 2], 3].flatten(2) == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			// (3 array allocs + internal alloc) * 10 + size(list) * 3 + 2 calls
			name:          "list_flatten_depth_three",
			expr:          `[[1, 2], 3].flatten(3) == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(48),
			actualCost:    48,
			version:       3,
		},
		{
			name:          "list_flatten_depth_three_v4",
			expr:          `[[1, 2], 3].flatten(3) == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			name:          "list_flatten",
			expr:          `[[1], 2, 3].flatten() == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(45),
			actualCost:    45,
			version:       3,
		},
		{
			name:          "list_flatten_v4",
			expr:          `[[1], 2, 3].flatten() == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			name:          "list_flatten_var",
			expr:          `x.flatten() == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:            map[string]any{"x": []any{[]any{1}, 2, 3}},
			hints:         map[string]uint64{"x": 3},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 26},
			actualCost:    26,
			version:       3,
		},
		{
			name:          "list_flatten_var_v4_no_item_hint",
			expr:          `x.flatten() == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:            map[string]any{"x": []any{[]any{1}, 2, 3}},
			hints:         map[string]uint64{"x": 3},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 18446744073709551615},
			actualCost:    26,
		},
		{
			name:          "list_flatten_var_v4_with_item_hint",
			expr:          `x.flatten() == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:            map[string]any{"x": []any{[]any{1}, 2, 3}},
			hints:         map[string]uint64{"x": 3, "x.@items": 5},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 38},
			actualCost:    26,
		},
		{
			name:          "list_flatten_depth_var",
			expr:          `[[1, 2], 3].flatten(x) == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.IntType)},
			in:            map[string]any{"x": 5},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: checker.FixedCostEstimate(18446744073709551615),
			actualCost:    53,
			version:       3,
		},
		{
			name:          "list_flatten_depth_var_v4",
			expr:          `[[1, 2], 3].flatten(x) == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.IntType)},
			in:            map[string]any{"x": 5},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: checker.FixedCostEstimate(46),
			actualCost:    46,
		},
		{
			// (2 array allocs + 1 internal) * 10
			// + size(list) * size(list) * 2
			// + 2 calls
			name:          "list_distinct_worst_case",
			expr:          `[1, 2, 3].distinct() == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(50),
			actualCost:    50,
		},
		{
			// (2 array allocs + 1 internal) * 10
			// + size(list) * size(list) * 2
			// + 2 calls
			name:          "list_distinct_best_case",
			expr:          `[1, 1, 1].distinct() == [1]`,
			estimatedCost: checker.FixedCostEstimate(50),
			actualCost:    50,
		},
		{
			// (1 array alloc + 1 internal) * 20
			// + [0, size(x) * size(x)] * 2 --> [0, 18]
			// + 2 calls
			// + 1 ident lookup
			name:          "list_distinct_var",
			expr:          `x.distinct() == ['hello']`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"hello", "hello"}},
			hints:         map[string]uint64{"x": 3, "x.@items": 5},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 41},
			actualCost:    31,
		},
		{
			// allocs: (2 + one internal) * 10
			// max_slice cost: 5
			// lookups: 2
			// calls: 2
			name: "list_slice_var_range",
			expr: `[1, 2, 3, 4, 5].slice(x, y) == [2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.IntType),
				cel.Variable("y", cel.IntType),
			},
			in:            map[string]any{"x": 1, "y": 3},
			estimatedCost: checker.FixedCostEstimate(39),
			actualCost:    36,
		},
		{
			// allocs: (1 + one internal) * 10
			// max_slice cost: 2
			// lookups: 1
			// calls: 2
			name: "list_slice_var_list",
			expr: `z.slice(1, 3) == [2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.IntType),
				cel.Variable("y", cel.IntType),
				cel.Variable("z", cel.ListType(cel.IntType)),
			},
			in:            map[string]any{"x": 1, "y": 3, "z": []int{1, 2, 3, 4, 5, 6, 7}},
			hints:         map[string]uint64{"z": 10},
			estimatedCost: checker.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			// allocs: (1 + one internal) * 10
			// max_slice cost: 10
			// lookups: 3
			// calls: 2
			name: "list_slice_var_list_var_range",
			expr: `z.slice(x, y) == [2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.IntType),
				cel.Variable("y", cel.IntType),
				cel.Variable("z", cel.ListType(cel.IntType)),
			},
			in:            map[string]any{"x": 1, "y": 3, "z": []int{1, 2, 3, 4, 5, 6, 7}},
			hints:         map[string]uint64{"z": 10},
			estimatedCost: checker.FixedCostEstimate(35),
			actualCost:    27,
		},
		{
			name:          "list_slice",
			expr:          `[1, 2, 3].slice(1, 3) == [2, 3]`,
			estimatedCost: checker.FixedCostEstimate(34),
			actualCost:    34,
		},
		{
			// allocs: (2 + one internal) * 10
			// reverse cost: 3
			// calls: 2
			name:          "list_reverse",
			expr:          `[3, 2, 1].reverse() == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(35),
			actualCost:    35,
		},
		{
			// allocs: (1 + one internal) * 10
			// reverse cost: 5
			// lookups: 1
			// calls: 2
			name: "list_var_reverse",
			expr: `x.reverse() == [1, 2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
			},
			in:            map[string]any{"x": []int{3, 2, 1}},
			hints:         map[string]uint64{"x": 5},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 28},
			actualCost:    26,
		},
		{
			// (2 allocs + 1 internal) * 10
			// + size(list) * size(list) * 2
			// + 2 calls
			name:          "list_sort",
			expr:          `[2, 3, 1].sort() == [1, 2, 3]`,
			estimatedCost: checker.FixedCostEstimate(50),
			actualCost:    50,
		},
		{
			// (1 allocs + 1 internal) * 10
			// + [0, size(x) * size(x)] * 2 --> [0, 50]
			// + 2 calls
			// + 1 ident lookup
			name:          "list_sort_var",
			expr:          `x.sort() == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
			in:            map[string]any{"x": []int{3, 2, 1}},
			hints:         map[string]uint64{"x": 5},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 73},
			actualCost:    41,
		},
		{
			name:          "list_sort_var_string",
			expr:          `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:         map[string]uint64{"x": 10, "x.@items": 1},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 223},
			actualCost:    98,
		},
		{
			name:          "list_sort_var_int_empty",
			expr:          `x.sort() == []`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
			in:            map[string]any{"x": []int{}},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: checker.CostEstimate{Min: 22, Max: 222},
			actualCost:    22,
		},
		{
			name:          "list_sortBy",
			expr:          `[{'x':4}, {'x':3}].sortBy(m, m['x']) == [{'x':3}, {'x':4}]`,
			estimatedCost: checker.FixedCostEstimate(211),
			actualCost:    211,
		},
		{
			name:          "list_sortBy_var",
			expr:          `x.sortBy(m, m['x']) == [{'x':3}, {'x':4}]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:            map[string]any{"x": []any{map[string]any{"x": 4}, map[string]any{"x": 3}}},
			hints:         map[string]uint64{"x": 5},
			estimatedCost: checker.CostEstimate{Min: 106, Max: 226},
			actualCost:    142,
		},
		{
			name:          "list_sortBy_var_string",
			expr:          `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:            map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:         map[string]uint64{"x": 3, "x.@items": 1},
			estimatedCost: checker.CostEstimate{Min: 136, Max: 196},
			actualCost:    196,
		},
		{
			name: "list_sort_concat_cost",
			expr: `(x.sort() + y).size() == 20`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.ListType(cel.IntType)),
			},
			in: map[string]any{
				"x": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				"y": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			hints: map[string]uint64{
				"x": 10,
				"y": 10,
			},
			estimatedCost: checker.CostEstimate{Min: 16, Max: 216},
			actualCost:    216,
		},
		{
			name: "list_distinct_concat_cost_with_item_hint",
			expr: `(x.distinct() + y).size() == 20`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.ListType(cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				"y": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
			hints: map[string]uint64{
				"x":        10,
				"x.@items": 1,
				"y":        10,
			},
			estimatedCost: checker.CostEstimate{Min: 16, Max: 216},
			actualCost:    226,
		},
		{
			name: "list_distinct_concat_cost",
			expr: `(x.distinct() + y).size() == 20`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.ListType(cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				"y": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
			hints: map[string]uint64{
				"x": 10,
				"y": 10,
			},
			estimatedCost: checker.CostEstimate{Min: 16, Max: 18446744073709551615},
			actualCost:    226,
		},
		{
			name:          "list_distinct_var_v3",
			expr:          `x.distinct() == ['hello']`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"hello", "hello"}},
			hints:         map[string]uint64{"x": 3},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 42},
			actualCost:    31,
			version:       3,
		},
		{
			name:          "list_distinct_var_v4_no_item_hint",
			expr:          `x.distinct() == ['hello']`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"hello", "hello"}},
			hints:         map[string]uint64{"x": 3},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 18446744073709551615},
			actualCost:    31,
		},
		{
			name:          "list_distinct_var_v4_with_item_hint",
			expr:          `x.distinct() == ['hello']`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"hello", "hello"}},
			hints:         map[string]uint64{"x": 3, "x.@items": 5},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 41},
			actualCost:    31,
		},
		{
			name:          "list_sort_var_string_v3",
			expr:          `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 233},
			actualCost:    98,
			version:       3,
		},
		{
			name:          "list_sort_var_string_v4_no_item_hint",
			expr:          `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 18446744073709551615},
			actualCost:    98,
		},
		{
			name:          "list_sort_var_string_v4_with_item_hint",
			expr:          `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:         map[string]uint64{"x": 10, "x.@items": 1},
			estimatedCost: checker.CostEstimate{Min: 23, Max: 223},
			actualCost:    98,
		},
		{
			name:          "list_sortBy_var_string_v3",
			expr:          `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:            map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:         map[string]uint64{"x": 3},
			estimatedCost: checker.CostEstimate{Min: 136, Max: 197},
			actualCost:    196,
			version:       3,
		},
		{
			name:          "list_sortBy_var_string_v4_no_item_hint",
			expr:          `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:            map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:         map[string]uint64{"x": 3},
			estimatedCost: checker.CostEstimate{Min: 136, Max: 18446744073709551615},
			actualCost:    196,
		},
		{
			name:          "list_sortBy_var_string_v4_with_item_hint",
			expr:          `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:            map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:         map[string]uint64{"x": 3, "x.@items": 1},
			estimatedCost: checker.CostEstimate{Min: 136, Max: 196},
			actualCost:    196,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env := testListsEnv(t, tc.version, tc.vars...)
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

func testListsEnv(t *testing.T, version int, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	var listsOpt cel.EnvOption
	if version > 0 {
		listsOpt = Lists(ListsVersion(uint32(version)))
	} else {
		listsOpt = Lists()
	}
	baseOpts := []cel.EnvOption{
		listsOpt,
		cel.Container("google.expr.proto2.test"),
		cel.Types(&proto2pb.ExampleType{},
			&proto2pb.ExternalMessageType{},
		)}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Lists()) failed: %v", err)
	}
	return env
}

func TestGenRangeMaxSize(t *testing.T) {
	// Negative size should fail.
	_, err := genRange(-1, defaultMaxRangeSize)
	if err == nil {
		t.Error("genRange(-1) should fail")
	}

	// Small size should work.
	val, err := genRange(10, defaultMaxRangeSize)
	if err != nil {
		t.Fatalf("genRange(10) failed: %v", err)
	}
	if val == nil {
		t.Fatal("genRange(10) returned nil")
	}

	// Over the limit should fail, not allocate.
	_, err = genRange(defaultMaxRangeSize+1, defaultMaxRangeSize)
	if err == nil {
		t.Error("genRange(defaultMaxRangeSize+1) should fail")
	}

	// Zero limit disables the check.
	val, err = genRange(100, 0)
	if err != nil {
		t.Fatalf("genRange(100, 0) with disabled limit failed: %v", err)
	}
	if val == nil {
		t.Fatal("genRange(100, 0) returned nil")
	}
}
