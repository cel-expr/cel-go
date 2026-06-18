// Copyright 2022 Google LLC
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
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/parser"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func TestTrackCostAdvanced(t *testing.T) {
	var equalCases = []struct {
		in      any
		lhsExpr string
		rhsExpr string
	}{
		{
			lhsExpr: `1`,
			rhsExpr: `2`,
		},
		{
			lhsExpr: `"abc".contains("d")`,
			rhsExpr: `"def".contains("d")`,
		},
		{
			lhsExpr: `1 in [4, 5, 6]`,
			rhsExpr: `2 in [15, 17, 16]`,
		},
	}
	for _, tc := range equalCases {
		t.Run(tc.lhsExpr+" vs "+tc.rhsExpr, func(t *testing.T) {
			ctx := constructActivation(t, tc.in)
			lhsCost, _, err := computeCost(t, tc.lhsExpr, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to eval expression due: %v", err)
			}
			rhsCost, _, err := computeCost(t, tc.rhsExpr, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to eval expression due: %v", err)
			}
			if lhsCost != rhsCost {
				t.Errorf(`Interpreter.Eval(activation Activation) failed return a cost for %s of %d equal to a cost for %s of %d`,
					tc.lhsExpr, lhsCost, tc.rhsExpr, rhsCost)
			}
		})

	}
	var smallerCases = []struct {
		in      any
		lhsExpr string
		rhsExpr string
	}{
		{
			lhsExpr: `1`,
			rhsExpr: `1 + 2`,
		},
		{
			lhsExpr: `"abc".contains("d")`,
			rhsExpr: `"abcdhdflsfiehfieubdkwjbdwgxvuyagwsdwdnw qdbgquyidvbwqi".contains("e")`,
		},
		{
			lhsExpr: `1 in [4, 5, 6]`,
			rhsExpr: `1 in [4, 5, 6, 7, 8, 9]`,
		},
	}
	for _, tc := range smallerCases {
		t.Run(tc.lhsExpr+" vs "+tc.rhsExpr, func(t *testing.T) {
			ctx := constructActivation(t, tc.in)
			lhsCost, _, err := computeCost(t, tc.lhsExpr, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to eval expression due: %v", err)
			}
			rhsCost, _, err := computeCost(t, tc.rhsExpr, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to eval expression due: %v", err)
			}
			if lhsCost >= rhsCost {
				t.Errorf(`Interpreter.Eval(activation Activation) failed return a cost for %s of %d less than the cost for %s of %d`,
					tc.lhsExpr, lhsCost, tc.rhsExpr, rhsCost)
			}
		})
	}
}

func computeCost(t *testing.T, expr string, vars []*decls.VariableDecl, ctx Activation, options []CostTrackerOption) (cost uint64, est checker.CostEstimate, err error) {
	t.Helper()

	s := common.NewTextSource(expr)
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("Failed to initialize parser: %v", err)
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf(`Failed to Parse expression "%s", error: %v`, expr, errs.GetErrors())
	}

	cont := containers.DefaultContainer
	reg := newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	attrs := NewAttributeFactory(cont, reg, reg)
	env := newTestEnv(t, cont, reg)
	err = env.AddIdents(vars...)
	if err != nil {
		t.Fatalf("Failed to initialize env: %v", err)
	}
	costTracker, err := NewCostTracker(&testRuntimeCostEstimator{}, options...)
	if err != nil {
		t.Fatalf("NewCostTracker() failed: %v", err)
	}
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf(`Failed to check expression "%s", error: %v`, expr, errs.GetErrors())
	}
	est, err = checker.Cost(checked, testCostEstimator{}, checker.PresenceTestHasCost(costTracker.presenceTestHasCost))
	if err != nil {
		t.Fatalf("checker.Cost() failed: %v", err)
	}
	interp := newStandardInterpreter(t, cont, reg, reg, attrs)
	prg, err := interp.NewInterpretable(checked,
		CostObserver(CostTrackerFactory(func() (*CostTracker, error) {
			return costTracker, nil
		})))
	if err != nil {
		t.Fatalf(`Failed to check expression "%s", error: %v`, expr, errs.GetErrors())
	}

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case EvalCancelledError:
				err = t
			default:
				err = fmt.Errorf("internal error: %v", r)
			}
		}
	}()
	frame := AsFrame(ctx)
	prg.Exec(frame)
	// TODO: enable this once all attributes are properly pushed and popped from stack.
	//if len(costTracker.stack) != 1 {
	//	t.Fatalf(`Expected resulting stack size to be 1 but got %d: %#+v`, len(costTracker.stack), costTracker.stack)
	//}
	return costTracker.cost, est, err
}

func constructActivation(t *testing.T, in any) Activation {
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

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randSeq(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}

type testRuntimeCostEstimator struct {
}

var timeToYearCost uint64 = 7

func (e testRuntimeCostEstimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	argsSize := make([]uint64, len(args))
	for i, arg := range args {
		reflectV := reflect.ValueOf(arg.Value())
		switch reflectV.Kind() {
		// Note that the CEL bytes type is implemented with Go byte slices, therefore also supported by the following
		// code.
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			argsSize[i] = uint64(reflectV.Len())
		default:
			argsSize[i] = 1
		}
	}

	switch overloadID {
	case overloads.TimestampToYear:
		return &timeToYearCost
	default:
		return nil
	}
}

type testCostEstimator struct {
	hints map[string]int64
}

func (tc testCostEstimator) EstimateSize(element checker.AstNode) *checker.SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &checker.SizeEstimate{Min: 0, Max: uint64(l)}
	}
	return nil
}

func (tc testCostEstimator) EstimateCallCost(function, overloadID string, target *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	switch overloadID {
	case overloads.TimestampToYear:
		return &checker.CallEstimate{CostEstimate: checker.FixedCostEstimate(7)}
	}
	return nil
}

func TestRuntimeCost(t *testing.T) {
	allTypes := types.NewObjectType("google.expr.proto3.test.TestAllTypes")
	allList := types.NewListType(allTypes)
	intList := types.NewListType(types.IntType)
	nestedList := types.NewListType(allList)

	allMap := types.NewMapType(types.StringType, allTypes)
	nestedMap := types.NewMapType(types.StringType, allMap)
	cases := []struct {
		name         string
		expr         string
		vars         []*decls.VariableDecl
		want         uint64
		in           any
		testFuncCost bool
		limit        uint64
		options      []CostTrackerOption

		expectExceedsLimit bool
	}{
		{
			name: "const",
			expr: `"Hello World!"`,
			want: 0,
		},
		{
			name: "identity",
			expr: `input`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", intList)},
			want: 1,
			in:   map[string]any{"input": []int{1, 2}},
		},
		{
			name: "select: map",
			expr: `input['key']`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			want: 2,
			in:   map[string]any{"input": map[string]string{"key": "v"}},
		},
		{
			name: "select: array index",
			expr: `input[0]`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewListType(types.StringType))},
			want: 2,
			in:   map[string]any{"input": []string{"v"}},
		},
		{
			name: "select: field",
			expr: `input.single_int32`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", allTypes)},
			want: 2,
			in: map[string]any{
				"input": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
		},
		{
			name: "expr select: map",
			expr: `input['ke' + 'y']`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			want: 3,
			in:   map[string]any{"input": map[string]string{"key": "v"}},
		},
		{
			name: "expr select: array index",
			expr: `input[3-3]`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewListType(types.StringType))},
			want: 3,
			in:   map[string]any{"input": []string{"v"}},
		},
		{
			name:    "select: field test only no has() cost",
			expr:    `has(input.single_int32)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			want:    1,
			options: []CostTrackerOption{PresenceTestHasCost(false)},
			in: map[string]any{
				"input": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
		},
		{
			name: "select: field test only",
			expr: `has(input.single_int32)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			want: 2,
			in: map[string]any{
				"input": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
		},
		{
			name:    "select: non-proto field test has() cost",
			expr:    `has(input.testAttr.nestedAttr)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			want:    3,
			options: []CostTrackerOption{PresenceTestHasCost(true)},
			in: map[string]any{
				"input": map[string]any{
					"testAttr": map[string]any{
						"nestedAttr": "0",
					},
				},
			},
		},
		{
			name:    "select: non-proto field test no has() cost",
			expr:    `has(input.testAttr.nestedAttr)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			want:    2,
			options: []CostTrackerOption{PresenceTestHasCost(false)},
			in: map[string]any{
				"input": map[string]any{
					"testAttr": map[string]any{
						"nestedAttr": "0",
					},
				},
			},
		},
		{
			name: "select: non-proto field test",
			expr: `has(input.testAttr.nestedAttr)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			want: 3,
			in: map[string]any{
				"input": map[string]any{
					"testAttr": map[string]any{
						"nestedAttr": "0",
					},
				},
			},
		},
		{
			name:         "estimated function call",
			expr:         `input.getFullYear()`,
			vars:         []*decls.VariableDecl{decls.NewVariable("input", types.TimestampType)},
			want:         8,
			in:           map[string]any{"input": time.Now()},
			testFuncCost: true,
		},
		{
			name: "create list",
			expr: `[1, 2, 3]`,
			want: 10,
		},
		{
			name: "create struct",
			expr: `google.expr.proto3.test.TestAllTypes{single_int32: 1, single_float: 3.14, single_string: 'str'}`,
			want: 40,
		},
		{
			name: "create map",
			expr: `{"a": 1, "b": 2, "c": 3}`,
			want: 30,
		},
		{
			name: "all comprehension",
			vars: []*decls.VariableDecl{decls.NewVariable("input", allList)},
			expr: `input.all(x, true)`,
			want: 2,
			in: map[string]any{
				"input": []*proto3pb.TestAllTypes{},
			},
		},
		{
			name: "nested all comprehension",
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedList)},
			expr: `input.all(x, x.all(y, true))`,
			want: 2,
			in: map[string]any{
				"input": []*proto3pb.TestAllTypes{},
			},
		},
		{
			name: "all comprehension on literal",
			expr: `[1, 2, 3].all(x, true)`,
			want: 20,
		},
		{
			name: "variable cost function",
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			expr: `input.matches('[0-9]')`,
			want: 103,
			in:   map[string]any{"input": string(randSeq(500))},
		},
		{
			name: "variable cost function with constant",
			expr: `'123'.matches('[0-9]')`,
			want: 2,
		},
		{
			name: "or",
			expr: `false || false`,
			want: 0,
		},
		{
			name: "or short-circuit",
			expr: `true || false`,
			want: 0,
		},

		{
			name: "or accumulated branch cost",
			expr: `a || b || c || d`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.BoolType),
				decls.NewVariable("b", types.BoolType),
				decls.NewVariable("c", types.BoolType),
				decls.NewVariable("d", types.BoolType),
			},
			in: map[string]any{
				"a": false,
				"b": false,
				"c": false,
				"d": false,
			},
			want: 4,
		},
		{
			name: "and",
			expr: `true && false`,
			want: 0,
		},
		{
			name: "and short-circuit",
			expr: `false && true`,
			want: 0,
		},
		{
			name: "and accumulated branch cost",
			expr: `a && b && c && d`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.BoolType),
				decls.NewVariable("b", types.BoolType),
				decls.NewVariable("c", types.BoolType),
				decls.NewVariable("d", types.BoolType),
			},
			in: map[string]any{
				"a": true,
				"b": true,
				"c": true,
				"d": true,
			},
			want: 4,
		},
		{
			name: "lt",
			expr: `1 < 2`,
			want: 1,
		},
		{
			name: "lte",
			expr: `1 <= 2`,
			want: 1,
		},
		{
			name: "eq",
			expr: `1 == 2`,
			want: 1,
		},
		{
			name: "gt",
			expr: `2 > 1`,
			want: 1,
		},
		{
			name: "gte",
			expr: `2 >= 1`,
			want: 1,
		},
		{
			name: "in",
			expr: `2 in [1, 2, 3]`,
			want: 13,
		},
		{
			name: "plus",
			expr: `1 + 1`,
			want: 1,
		},
		{
			name: "minus",
			expr: `1 - 1`,
			want: 1,
		},
		{
			name: "/",
			expr: `1 / 1`,
			want: 1,
		},
		{
			name: "/",
			expr: `1 * 1`,
			want: 1,
		},
		{
			name: "%",
			expr: `1 % 1`,
			want: 1,
		},
		{
			name: "ternary",
			expr: `true ? 1 : 2`,
			want: 0,
		},
		{
			name: "string size",
			expr: `size("123")`,
			want: 1,
		},
		{
			name: "str eq str",
			expr: `'12345678901234567890' == '123456789012345678901234567890'`,
			want: 2,
		},
		{
			name: "bytes to string conversion",
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.BytesType)},
			expr: `string(input)`,
			want: 51,
			in:   map[string]any{"input": randSeq(500)},
		},
		{
			name: "string to bytes conversion",
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			expr: `bytes(input)`,
			want: 51,
			in:   map[string]any{"input": string(randSeq(500))},
		},
		{
			name: "int to string conversion",
			expr: `string(1)`,
			want: 1,
		},
		{
			name: "contains",
			expr: `input.contains(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			want: 2502,
			in:   map[string]any{"input": string(randSeq(500)), "arg1": string(randSeq(500))},
		},
		{
			name: "matches",
			expr: `input.matches('\\d+a\\d+b')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			want: 103,
			in:   map[string]any{"input": string(randSeq(500)), "arg1": string(randSeq(500))},
		},
		{
			name: "startsWith",
			expr: `input.startsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			want: 3,
			in:   map[string]any{"input": "idc", "arg1": string(randSeq(500))},
		},
		{
			name: "endsWith",
			expr: `input.endsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			want: 3,
			in:   map[string]any{"input": "idc", "arg1": string(randSeq(500))},
		},
		{
			name: "size receiver",
			expr: `input.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			want: 2,
			in:   map[string]any{"input": "500", "arg1": "500"},
		},
		{
			name: "size",
			expr: `size(input)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			want: 2,
			in:   map[string]any{"input": "500", "arg1": "500"},
		},
		{
			name: "ternary eval",
			expr: `(x > 2 ? input1 : input2).all(y, true)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("input1", allList),
				decls.NewVariable("input2", allList),
			},
			want: 6,
			in:   map[string]any{"input1": []*proto3pb.TestAllTypes{{}}, "input2": []*proto3pb.TestAllTypes{{}}, "x": 1},
		},
		{
			name: "ternary eval trivial, true",
			expr: `true ? false : 1 > 3`,
			want: 0,
			in:   map[string]any{},
		},
		{
			name: "ternary eval trivial, false",
			expr: `false ? false : 1 > 3`,
			want: 1,
			in:   map[string]any{},
		},
		{
			name: "comprehension over map",
			expr: `input.all(k, input[k].single_int32 > 3)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", allMap),
			},
			want: 9,
			in:   map[string]any{"input": map[string]any{"val": &proto3pb.TestAllTypes{}}},
		},
		{
			name: "comprehension over nested map of maps",
			expr: `input.all(k, input[k].all(x, true))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			want: 2,
			in:   map[string]any{"input": map[string]any{}},
		},
		{
			name: "string size of map keys",
			expr: `input.all(k, k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			want: 2,
			in:   map[string]any{"input": map[string]any{}},
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			want: 2,
			in:   map[string]any{"input": map[string]any{}},
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			want: 2,
			in:   map[string]any{"input": map[string]any{}},
		},
		{
			name: "list concat",
			expr: `(list1 + list2).all(x, true)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			want: 4,
			in:   map[string]any{"list1": []int{}, "list2": []int{}},
		},
		{
			name: "str concat",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			want: 6,
			in:   map[string]any{"str1": "val1", "str2": "val2222222"},
		},
		{
			name: "str concat custom cost tracker",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			options: []CostTrackerOption{
				OverloadCostTracker(overloads.ContainsString,
					func(args []ref.Val, result ref.Val) *uint64 {
						strCost := uint64(math.Ceil(float64(actualSize(args[0])) * 0.2))
						substrCost := uint64(math.Ceil(float64(actualSize(args[1])) * 0.2))
						cost := strCost * substrCost
						return &cost
					}),
			},
			want: 10,
			in:   map[string]any{"str1": "val1", "str2": "val2222222"},
		},
		{
			name: "at limit",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			in:    map[string]any{"str1": "val1", "str2": "val2222222"},
			limit: 6,
			want:  6,
		},
		{
			name: "above limit",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			in:                 map[string]any{"str1": "val1", "str2": "val2222222"},
			limit:              5,
			expectExceedsLimit: true,
		},
		{
			name: "ternary as operand",
			expr: `(1 > 2 ? 5 : 3) > 1`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 2,
		},
		{
			name: "ternary as operand",
			expr: `(1 > 2 || 2 > 1) == true`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 3,
		},
		{
			name: "list map literal",
			expr: `[{'k1': 1}, {'k2': 2}].all(x, true)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 77,
		},
		{
			name: "list map literal",
			expr: `[{'k1': 1}, {'k2': 2}].all(x, true)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 77,
		},
		{
			name: ".filter list literal",
			expr: `[1,2,3,4,5].filter(x, x % 2 == 0)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 62,
		},
		{
			name: ".map list literal",
			expr: `[1,2,3,4,5].map(x, x)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 86,
		},
		{
			name: ".map.filter list literal",
			expr: `[1,2,3,4,5].map(x, x).filter(x, x % 2 == 0)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 138,
		},
		{
			name: ".map.exists list literal",
			expr: `[1,2,3,4,5].map(x, x).exists(x, x == 5) == true`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 118,
		},
		{
			name: ".map.map list literal",
			expr: `[1,2,3,4,5].map(x, x).map(x, x)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 162,
		},
		{
			name: ".map.map list literal",
			expr: `[1,2,3,4,5].map(x, [x, x]).filter(z, z.size() == 2)`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			want: 232,
		},
		{
			name: "comprehension on nested list",
			expr: `[1,2,3,4,5].map(x, [x, x]).all(y, y.all(y, y == 1))`,
			want: 171,
		},
		{
			name: "comprehension size",
			expr: `[1,2,3,4,5].map(x, x).map(x, x) + [1]`,
			want: 173,
		},
		{
			name: "nested comprehension",
			expr: `[1,2,3].all(i, i in [1,2,3].map(j, j + j))`,
			want: 86,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := constructActivation(t, tc.in)
			var costLimit *uint64
			if tc.limit > 0 {
				costLimit = &tc.limit
			}
			options := tc.options
			if costLimit != nil {
				options = append(options, CostTrackerLimit(*costLimit))
			}
			actualCost, est, err := computeCost(t, tc.expr, tc.vars, ctx, options)
			if err != nil {
				if tc.expectExceedsLimit {
					return
				}
				t.Fatalf("Interpreter.Eval(activation Activation) failed due to: %v", err)
			}
			if tc.expectExceedsLimit {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to return a cost exceeded error for limit %d, got cost %d", tc.limit, actualCost)
			}
			if actualCost != tc.want {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to return expected runtime cost %d, got %d", tc.want, actualCost)
			}
			if est.Min > actualCost || est.Max < actualCost {
				t.Fatalf("Interpreter.Eval(activation Activation) failed to return cost in range of estimate cost [%d, %d], got %d",
					est.Min, est.Max, actualCost)
			}
		})
	}
}
