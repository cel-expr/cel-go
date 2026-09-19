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

package cel

import (
	"strings"
	"testing"

	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func TestCostModelTrackerOptions(t *testing.T) {
	tests := []struct {
		name       string
		expr       string
		envOpts    []EnvOption
		in         any
		wantResult ref.Val
		wantCost   uint64
	}{
		{
			name: "custom_global_function_cost_model",
			expr: `custom_len(str)`,
			envOpts: []EnvOption{
				Variable("str", StringType),
				Function("custom_len",
					Overload("custom_len_string", []*Type{StringType}, IntType,
						UnaryBinding(func(val ref.Val) ref.Val {
							return types.Int(len(val.Value().(string)))
						}),
					),
				),
				CostModel(
					cost.Overload("custom_len_string",
						cost.EvalCost(cost.Const(42)),
					),
				),
			},
			in:         map[string]any{"str": "hello"},
			wantResult: types.Int(5),
			// 1 for variable 'str' + 42 for custom_len
			wantCost: 43,
		},
		{
			name: "custom_member_function_cost_model",
			expr: `str.custom_transform("prefix_")`,
			envOpts: []EnvOption{
				Variable("str", StringType),
				Function("custom_transform",
					MemberOverload("string_custom_transform_string", []*Type{StringType, StringType}, StringType,
						BinaryBinding(func(target, arg ref.Val) ref.Val {
							return types.String(arg.Value().(string) + target.Value().(string))
						}),
					),
				),
				CostModel(
					cost.MemberOverload("string_custom_transform_string",
						cost.EvalCost(cost.Sum(cost.Scale(cost.Target(), 2.0), cost.Scale(cost.Arg(0), 1.0))),
					),
				),
			},
			in:         map[string]any{"str": "abc"},
			wantResult: types.String("prefix_abc"),
			// 1 for variable 'str' + target size (3)*2 + arg0 size (7)*1 = 1 + 6 + 7 = 14
			wantCost: 14,
		},
		{
			name: "override_standard_overload_cost_model",
			expr: `str.startsWith("prefix")`,
			envOpts: []EnvOption{
				Variable("str", StringType),
				CostModel(
					cost.MemberOverload(overloads.StartsWithString,
						cost.EvalCost(cost.Const(100)),
					),
				),
			},
			in:         map[string]any{"str": "prefix_test"},
			wantResult: types.True,
			// 1 for variable 'str' + 100 for startsWith
			wantCost: 101,
		},
		{
			name: "multiple_overload_models",
			expr: `str.startsWith("pre") && str.endsWith("fix")`,
			envOpts: []EnvOption{
				Variable("str", StringType),
				CostModel(
					cost.MemberOverload(overloads.StartsWithString,
						cost.EvalCost(cost.Const(10)),
					),
					cost.MemberOverload(overloads.EndsWithString,
						cost.EvalCost(cost.Const(20)),
					),
				),
			},
			in:         map[string]any{"str": "prefix"},
			wantResult: types.True,
			// 1 (str) + 10 (startsWith) + 1 (str) + 20 (endsWith) = 32
			wantCost: 32,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := testEnv(t, tc.envOpts...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}

			prg, err := env.Program(ast, CostTracking(nil))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}

			out, details, err := prg.Eval(tc.in)
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}

			if out.Equal(tc.wantResult) != types.True {
				t.Errorf("prg.Eval() result = %v, want %v", out, tc.wantResult)
			}

			if details.ActualCost() == nil {
				t.Fatalf("details.ActualCost() is nil, expected %d", tc.wantCost)
			}
			if *details.ActualCost() != tc.wantCost {
				t.Errorf("details.ActualCost() = %d, want %d", *details.ActualCost(), tc.wantCost)
			}
		})
	}
}

func TestCostSizingStrategyComparison(t *testing.T) {
	tests := []struct {
		name              string
		expr              string
		envOpts           []EnvOption
		hints             map[string]uint64
		in                any
		wantDefaultEst    cost.CostEstimate
		wantDefaultCost   uint64
		wantAggregateEst  cost.CostEstimate
		wantAggregateCost uint64
	}{
		{
			name: "list_of_strings_in_list",
			expr: `"b" in list`,
			envOpts: []EnvOption{
				Variable("list", ListType(StringType)),
				CostModel(
					cost.Overload(overloads.InList,
						cost.EvalCost(cost.Arg(1)),
					),
				),
			},
			hints: map[string]uint64{"list": 2, "list.@items": 5},
			in:    map[string]any{"list": []string{"hello", "world"}},
			// Default sizing: list item count = 2 -> actual: 1 (ident) + 2 = 3
			wantDefaultEst:  cost.CostEstimate{Min: 1, Max: 3},
			wantDefaultCost: 3,
			// Aggregate sizing: container(1) + "hello"(5) + "world"(5) = 11 -> actual: 1 (ident) + 11 = 12
			wantAggregateEst:  cost.CostEstimate{Min: 2, Max: 12},
			wantAggregateCost: 12,
		},
		{
			name: "map_string_to_string",
			expr: `custom_func(map_val)`,
			envOpts: []EnvOption{
				Variable("map_val", MapType(StringType, StringType)),
				Function("custom_func",
					Overload("custom_func_map", []*Type{MapType(StringType, StringType)}, IntType,
						UnaryBinding(func(val ref.Val) ref.Val {
							return types.Int(1)
						}),
					),
				),
				CostModel(
					cost.Overload("custom_func_map",
						cost.EvalCost(cost.Arg(0)),
					),
				),
			},
			hints: map[string]uint64{"map_val": 2, "map_val.@keys": 2, "map_val.@values": 2},
			in:    map[string]any{"map_val": map[string]string{"k1": "v1", "k2": "v2"}},
			// Default sizing: map entries count = 2 -> actual: 1 (ident) + 2 = 3
			wantDefaultEst:  cost.CostEstimate{Min: 1, Max: 3},
			wantDefaultCost: 3,
			// Aggregate sizing: container(1) + 2 keys(2+2) + 2 values(2+2) = 9 -> actual: 1 + 9 = 10
			wantAggregateEst:  cost.CostEstimate{Min: 2, Max: 10},
			wantAggregateCost: 10,
		},
		{
			name: "nested_list_of_lists",
			expr: `custom_func(nested_list)`,
			envOpts: []EnvOption{
				Variable("nested_list", ListType(ListType(IntType))),
				Function("custom_func",
					Overload("custom_func_nested", []*Type{ListType(ListType(IntType))}, IntType,
						UnaryBinding(func(val ref.Val) ref.Val {
							return types.Int(1)
						}),
					),
				),
				CostModel(
					cost.Overload("custom_func_nested",
						cost.EvalCost(cost.Arg(0)),
					),
				),
			},
			hints: map[string]uint64{"nested_list": 2, "nested_list.@items": 3},
			in:    map[string]any{"nested_list": [][]int{{1, 2}, {3, 4, 5}}},
			// Default sizing: outer list count = 2 -> actual: 1 + 2 = 3
			wantDefaultEst:  cost.CostEstimate{Min: 1, Max: 3},
			wantDefaultCost: 3,
			// Aggregate sizing: container(1) + inner1(1+2) + inner2(1+3) = 8 -> actual: 1 + 8 = 9
			wantAggregateEst:  cost.CostEstimate{Min: 2, Max: 10},
			wantAggregateCost: 9,
		},
		{
			name: "nested_list_of_string_lists",
			expr: `custom_func(nested_list)`,
			envOpts: []EnvOption{
				Variable("nested_list", ListType(ListType(StringType))),
				Function("custom_func",
					Overload("custom_func_nested_str_list", []*Type{ListType(ListType(StringType))}, IntType,
						UnaryBinding(func(val ref.Val) ref.Val {
							return types.Int(1)
						}),
					),
				),
				CostModel(
					cost.Overload("custom_func_nested_str_list",
						cost.EvalCost(cost.Arg(0)),
					),
				),
			},
			hints: map[string]uint64{
				"nested_list":               2,
				"nested_list.@items":        2,
				"nested_list.@items.@items": 4,
			},
			in: map[string]any{"nested_list": [][]string{{"ab", "cd"}, {"efg", "hijk"}}},
			// Default sizing: outer list count = 2 -> actual: 1 + 2 = 3
			wantDefaultEst:  cost.CostEstimate{Min: 1, Max: 3},
			wantDefaultCost: 3,
			// Aggregate sizing: container(1) + inner1(1+2+2) + inner2(1+3+4) = 14 -> actual: 1 + 14 = 15
			wantAggregateEst:  cost.CostEstimate{Min: 2, Max: 20},
			wantAggregateCost: 15,
		},
		{
			name: "nested_map_of_string_lists",
			expr: `custom_func(map_val)`,
			envOpts: []EnvOption{
				Variable("map_val", MapType(StringType, ListType(StringType))),
				Function("custom_func",
					Overload("custom_func_map_list", []*Type{MapType(StringType, ListType(StringType))}, IntType,
						UnaryBinding(func(val ref.Val) ref.Val {
							return types.Int(1)
						}),
					),
				),
				CostModel(
					cost.Overload("custom_func_map_list",
						cost.EvalCost(cost.Arg(0)),
					),
				),
			},
			hints: map[string]uint64{
				"map_val":                2,
				"map_val.@keys":          3,
				"map_val.@values":        2,
				"map_val.@values.@items": 4,
			},
			in: map[string]any{"map_val": map[string][]string{"k1": {"a", "bc"}, "key2": {"def", "ghij"}}},
			// Default sizing: map entries count = 2 -> actual: 1 + 2 = 3
			wantDefaultEst:  cost.CostEstimate{Min: 1, Max: 3},
			wantDefaultCost: 3,
			// Aggregate sizing: container(1) + k1(2) + val1(1+1+2) + k2(4) + val2(1+3+4) = 19 -> actual: 1 + 19 = 20
			wantAggregateEst:  cost.CostEstimate{Min: 2, Max: 26},
			wantAggregateCost: 20,
		},
		{
			name: "nested_map_of_maps",
			expr: `custom_func(map_val)`,
			envOpts: []EnvOption{
				Variable("map_val", MapType(StringType, MapType(StringType, StringType))),
				Function("custom_func",
					Overload("custom_func_map_map", []*Type{MapType(StringType, MapType(StringType, StringType))}, IntType,
						UnaryBinding(func(val ref.Val) ref.Val {
							return types.Int(1)
						}),
					),
				),
				CostModel(
					cost.Overload("custom_func_map_map",
						cost.EvalCost(cost.Arg(0)),
					),
				),
			},
			hints: map[string]uint64{
				"map_val":                 2,
				"map_val.@keys":           2,
				"map_val.@values":         2,
				"map_val.@values.@keys":   2,
				"map_val.@values.@values": 3,
			},
			in: map[string]any{"map_val": map[string]map[string]string{"k1": {"a": "bc"}, "k2": {"d": "efg"}}},
			// Default sizing: map entries count = 2 -> actual: 1 + 2 = 3
			wantDefaultEst:  cost.CostEstimate{Min: 1, Max: 3},
			wantDefaultCost: 3,
			// Aggregate sizing: container(1) + k1(2) + val1(1+1+2) + k2(2) + val2(1+1+3) = 14 -> actual: 1 + 14 = 15
			wantAggregateEst:  cost.CostEstimate{Min: 2, Max: 28},
			wantAggregateCost: 15,
		},
		{
			name: "flat_string_target_equality",
			expr: `str.startsWith("prefix")`,
			envOpts: []EnvOption{
				Variable("str", StringType),
				CostModel(
					cost.MemberOverload(overloads.StartsWithString,
						cost.EvalCost(cost.Target()),
					),
				),
			},
			hints: map[string]uint64{"str": 12},
			in:    map[string]any{"str": "prefix_hello"},
			// Flat strings: both strategies evaluate character length identically (12)
			wantDefaultEst:    cost.CostEstimate{Min: 1, Max: 13},
			wantDefaultCost:   13,
			wantAggregateEst:  cost.CostEstimate{Min: 1, Max: 13},
			wantAggregateCost: 13,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			estimator := testCostEstimator{hints: tc.hints}

			// 1. Assess with DefaultSizingStrategy
			defaultEnv := testEnv(t, append(tc.envOpts, CostSizingStrategy(cost.DefaultSizingStrategy()))...)
			defaultAst, iss := defaultEnv.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("defaultEnv.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			defaultEst, err := defaultEnv.EstimateCost(defaultAst, estimator)
			if err != nil {
				t.Fatalf("defaultEnv.EstimateCost() failed: %v", err)
			}
			if defaultEst.Min != tc.wantDefaultEst.Min || defaultEst.Max != tc.wantDefaultEst.Max {
				t.Errorf("defaultEst = [%d, %d], want [%d, %d]", defaultEst.Min, defaultEst.Max, tc.wantDefaultEst.Min, tc.wantDefaultEst.Max)
			}

			defaultPrg, err := defaultEnv.Program(defaultAst, CostTracking(nil))
			if err != nil {
				t.Fatalf("defaultEnv.Program() failed: %v", err)
			}
			_, defaultDet, err := defaultPrg.Eval(tc.in)
			if err != nil {
				t.Fatalf("defaultPrg.Eval() failed: %v", err)
			}
			if defaultDet.ActualCost() == nil {
				t.Fatalf("defaultDet.ActualCost() is nil, want %d", tc.wantDefaultCost)
			}
			defaultActual := *defaultDet.ActualCost()
			if defaultActual != tc.wantDefaultCost {
				t.Errorf("defaultActual = %d, want %d", defaultActual, tc.wantDefaultCost)
			}
			if defaultActual < defaultEst.Min || defaultActual > defaultEst.Max {
				t.Errorf("defaultActual %d not in range [%d, %d]", defaultActual, defaultEst.Min, defaultEst.Max)
			}

			// 2. Assess with AggregateSizingStrategy
			aggregateEnv := testEnv(t, append(tc.envOpts, CostSizingStrategy(cost.AggregateSizingStrategy()))...)
			aggregateAst, iss := aggregateEnv.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("aggregateEnv.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			aggregateEst, err := aggregateEnv.EstimateCost(aggregateAst, estimator)
			if err != nil {
				t.Fatalf("aggregateEnv.EstimateCost() failed: %v", err)
			}
			if aggregateEst.Min != tc.wantAggregateEst.Min || aggregateEst.Max != tc.wantAggregateEst.Max {
				t.Errorf("aggregateEst = [%d, %d], want [%d, %d]", aggregateEst.Min, aggregateEst.Max, tc.wantAggregateEst.Min, tc.wantAggregateEst.Max)
			}

			aggregatePrg, err := aggregateEnv.Program(aggregateAst, CostTracking(nil))
			if err != nil {
				t.Fatalf("aggregateEnv.Program() failed: %v", err)
			}
			_, aggregateDet, err := aggregatePrg.Eval(tc.in)
			if err != nil {
				t.Fatalf("aggregatePrg.Eval() failed: %v", err)
			}
			if aggregateDet.ActualCost() == nil {
				t.Fatalf("aggregateDet.ActualCost() is nil, want %d", tc.wantAggregateCost)
			}
			aggregateActual := *aggregateDet.ActualCost()
			if aggregateActual != tc.wantAggregateCost {
				t.Errorf("aggregateActual = %d, want %d", aggregateActual, tc.wantAggregateCost)
			}
			if aggregateActual < aggregateEst.Min || aggregateActual > aggregateEst.Max {
				t.Errorf("aggregateActual %d not in range [%d, %d]", aggregateActual, aggregateEst.Min, aggregateEst.Max)
			}
		})
	}
}

func TestCostModelExtendEnv(t *testing.T) {
	parentEnv := testEnv(t,
		Variable("str", StringType),
		CostModel(
			cost.MemberOverload(overloads.StartsWithString,
				cost.EvalCost(cost.Const(50)),
			),
		),
		CostSizingStrategy(cost.AggregateSizingStrategy()),
	)

	childEnv, err := parentEnv.Extend(
		CostModel(
			cost.MemberOverload(overloads.EndsWithString,
				cost.EvalCost(cost.Const(25)),
			),
		),
	)
	if err != nil {
		t.Fatalf("parentEnv.Extend() failed: %v", err)
	}

	ast, iss := childEnv.Compile(`str.startsWith("a") && str.endsWith("z")`)
	if iss.Err() != nil {
		t.Fatalf("childEnv.Compile() failed: %v", iss.Err())
	}

	prg, err := childEnv.Program(ast, CostTracking(nil))
	if err != nil {
		t.Fatalf("childEnv.Program() failed: %v", err)
	}

	_, details, err := prg.Eval(map[string]any{"str": "abc_xyz"})
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}

	if details.ActualCost() == nil {
		t.Fatal("details.ActualCost() is nil")
	}
	// 1 (str) + 50 (startsWith) + 1 (str) + 25 (endsWith) = 77
	wantCost := uint64(77)
	if *details.ActualCost() != wantCost {
		t.Errorf("details.ActualCost() = %d, want %d", *details.ActualCost(), wantCost)
	}
}

func TestCostTrackerOptionsPrecedence(t *testing.T) {
	// Program-level CostTrackerOptions should override/augment environment-level CostModel.
	env := testEnv(t,
		Variable("str", StringType),
		CostModel(
			cost.MemberOverload(overloads.StartsWithString,
				cost.EvalCost(cost.Const(50)),
			),
		),
	)

	ast, iss := env.Compile(`str.startsWith("prefix")`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	overrideCost := uint64(999)
	prg, err := env.Program(ast,
		CostTracking(nil),
		CostTrackerOptions(
			cost.OverloadTracker(overloads.StartsWithString, func(args []ref.Val, result ref.Val) *uint64 {
				return &overrideCost
			}),
		),
	)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	_, details, err := prg.Eval(map[string]any{"str": "prefix_value"})
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}

	if details.ActualCost() == nil {
		t.Fatal("details.ActualCost() is nil")
	}
	// 1 (str) + 999 (program-level override) = 1000
	wantCost := uint64(1000)
	if *details.ActualCost() != wantCost {
		t.Errorf("details.ActualCost() = %d, want %d", *details.ActualCost(), wantCost)
	}
}

func TestCostLimitWithCostModel(t *testing.T) {
	env := testEnv(t,
		Variable("str", StringType),
		CostModel(
			cost.MemberOverload(overloads.StartsWithString,
				cost.EvalCost(cost.Const(100)),
			),
		),
	)

	ast, iss := env.Compile(`str.startsWith("prefix")`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	// Case 1: Limit higher than cost (101) -> succeeds
	prgPass, err := env.Program(ast, CostLimit(150))
	if err != nil {
		t.Fatalf("env.Program(CostLimit(150)) failed: %v", err)
	}
	_, detailsPass, err := prgPass.Eval(map[string]any{"str": "prefix_val"})
	if err != nil {
		t.Fatalf("prgPass.Eval() failed: %v", err)
	}
	if detailsPass.ActualCost() == nil || *detailsPass.ActualCost() != 101 {
		t.Errorf("detailsPass.ActualCost() = %v, want 101", detailsPass.ActualCost())
	}

	// Case 2: Limit lower than cost (101) -> fails
	prgFail, err := env.Program(ast, CostLimit(50))
	if err != nil {
		t.Fatalf("env.Program(CostLimit(50)) failed: %v", err)
	}
	_, _, err = prgFail.Eval(map[string]any{"str": "prefix_val"})
	if err == nil {
		t.Fatal("prgFail.Eval() expected error due to cost limit, got nil")
	}
	if !strings.Contains(err.Error(), "actual cost limit exceeded") {
		t.Errorf("prgFail.Eval() error = %q, want containing 'actual cost limit exceeded'", err.Error())
	}
}

func TestCostModelEstimateAndTrackingAlignment(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		envOpts  []EnvOption
		hints    map[string]uint64
		in       map[string]any
		wantEst  cost.CostEstimate
		wantCost uint64
	}{
		{
			name: "startsWith with custom cost model",
			expr: `str.startsWith("prefix")`,
			envOpts: []EnvOption{
				Variable("str", StringType),
				CostModel(cost.MemberOverload(overloads.StartsWithString,
					cost.EvalCost(cost.Scale(cost.Arg(0), 1.0)),
				)),
			},
			hints:    map[string]uint64{"str": 10},
			in:       map[string]any{"str": "prefix_hello"},
			wantEst:  cost.CostEstimate{Min: 7, Max: 7},
			wantCost: 7,
		},
		{
			name: "contains and string concat",
			expr: `str1.contains(str2 + "!")`,
			envOpts: []EnvOption{
				Variable("str1", StringType),
				Variable("str2", StringType),
			},
			hints:    map[string]uint64{"str1": 20, "str2": 10},
			in:       map[string]any{"str1": "hello_world_test!", "str2": "world"},
			wantEst:  cost.CostEstimate{Min: 3, Max: 8},
			wantCost: 5,
		},
		{
			name: "list all comprehension",
			expr: `list.all(x, x > 0)`,
			envOpts: []EnvOption{
				Variable("list", ListType(IntType)),
			},
			hints:    map[string]uint64{"list": 5},
			in:       map[string]any{"list": []int{1, 2, 3, 4, 5}},
			wantEst:  cost.CostEstimate{Min: 2, Max: 27},
			wantCost: 27,
		},
		{
			name: "ternary expression",
			expr: `cond ? str1.size() : str2.size()`,
			envOpts: []EnvOption{
				Variable("cond", BoolType),
				Variable("str1", StringType),
				Variable("str2", StringType),
			},
			hints:    map[string]uint64{"str1": 10, "str2": 10},
			in:       map[string]any{"cond": true, "str1": "abc", "str2": "def"},
			wantEst:  cost.CostEstimate{Min: 3, Max: 3},
			wantCost: 3,
		},
		{
			name: "nested list comprehension with size hints",
			expr: `list.all(sub, sub.all(x, x > 0))`,
			envOpts: []EnvOption{
				Variable("list", ListType(ListType(IntType))),
			},
			hints:    map[string]uint64{"list": 3, "list.@items": 4},
			in:       map[string]any{"list": [][]int{{1, 2}, {3, 4, 5}, {6}}},
			wantEst:  cost.CostEstimate{Min: 2, Max: 77},
			wantCost: 47,
		},
		{
			name: "nested map comprehension with nested list",
			expr: `map_val.all(k, map_val[k].all(x, x > 0))`,
			envOpts: []EnvOption{
				Variable("map_val", MapType(StringType, ListType(IntType))),
			},
			hints:    map[string]uint64{"map_val": 2, "map_val.@keys": 3, "map_val.@values": 4},
			in:       map[string]any{"map_val": map[string][]int{"a": {1, 2}, "b": {3, 4, 5}}},
			wantEst:  cost.CostEstimate{Min: 2, Max: 56},
			wantCost: 39,
		},
		{
			name: "nested map of maps with nested string size",
			expr: `map_val.all(k, map_val[k].all(sub_k, map_val[k][sub_k].size() > 0))`,
			envOpts: []EnvOption{
				Variable("map_val", MapType(StringType, MapType(StringType, StringType))),
			},
			hints: map[string]uint64{
				"map_val":                 2,
				"map_val.@keys":           2,
				"map_val.@values":         2,
				"map_val.@values.@keys":   2,
				"map_val.@values.@values": 5,
			},
			in:       map[string]any{"map_val": map[string]map[string]string{"k1": {"a": "hello"}, "k2": {"b": "world"}}},
			wantEst:  cost.CostEstimate{Min: 2, Max: 56},
			wantCost: 30,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := testEnv(t, tc.envOpts...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}

			// Estimate cost
			est, err := env.EstimateCost(ast, testCostEstimator{hints: tc.hints})
			if err != nil {
				t.Fatalf("env.EstimateCost() failed: %v", err)
			}
			if est.Min != tc.wantEst.Min || est.Max != tc.wantEst.Max {
				t.Errorf("est = [%d, %d], want [%d, %d]", est.Min, est.Max, tc.wantEst.Min, tc.wantEst.Max)
			}

			// Track actual cost
			prg, err := env.Program(ast, CostTracking(nil))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}

			_, details, err := prg.Eval(tc.in)
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}

			if details.ActualCost() == nil {
				t.Fatal("details.ActualCost() is nil")
			}

			actualCost := *details.ActualCost()
			if actualCost != tc.wantCost {
				t.Errorf("actualCost = %d, want %d", actualCost, tc.wantCost)
			}
			if actualCost < est.Min || actualCost > est.Max {
				t.Errorf("tracked cost %d must be in range [%d, %d] (est.Min <= actualCost <= est.Max)", actualCost, est.Min, est.Max)
			}
		})
	}
}

func TestCostModelNilOrEmpty(t *testing.T) {
	// Verify that cost tracking works as expected when no CostModel or CostSizingStrategy is set
	env := testEnv(t,
		Variable("x", IntType),
	)

	ast, iss := env.Compile(`x + 1`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	prg, err := env.Program(ast, CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	out, details, err := prg.Eval(map[string]any{"x": 10})
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}

	if out.Equal(types.Int(11)) != types.True {
		t.Errorf("got %v, want 11", out)
	}

	if details.ActualCost() == nil {
		t.Fatal("details.ActualCost() is nil")
	}
	if *details.ActualCost() == 0 {
		t.Errorf("details.ActualCost() = 0, want > 0")
	}
}
