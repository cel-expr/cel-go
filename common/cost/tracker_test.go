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

package cost_test

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

type testCall struct {
	function   string
	overloadID string
}

func (c testCall) Function() string {
	return c.function
}

func (c testCall) OverloadID() string {
	return c.overloadID
}

func TestCostTracker_BasicOperations(t *testing.T) {
	tracker, err := cost.NewTracker(nil,
		cost.TrackerPresenceTestHasCost(true),
	)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	tests := []struct {
		name     string
		action   func()
		wantCost uint64
	}{
		{
			name: "create_list",
			action: func() {
				tracker.CreateList(1, nil)
			},
			wantCost: cost.ListCreateBaseCost,
		},
		{
			name: "create_map",
			action: func() {
				tracker.CreateMap(2, nil)
			},
			wantCost: cost.ListCreateBaseCost + cost.MapCreateBaseCost,
		},
		{
			name: "create_struct",
			action: func() {
				tracker.CreateStruct(3, nil)
			},
			wantCost: cost.ListCreateBaseCost + cost.MapCreateBaseCost + cost.StructCreateBaseCost,
		},
		{
			name: "eval_attribute",
			action: func() {
				tracker.EvalAttribute(4, false, nil)
			},
			wantCost: cost.ListCreateBaseCost + cost.MapCreateBaseCost + cost.StructCreateBaseCost + cost.SelectAndIdentCost,
		},
		{
			name: "qualify",
			action: func() {
				tracker.Qualify(5)
			},
			wantCost: cost.ListCreateBaseCost + cost.MapCreateBaseCost + cost.StructCreateBaseCost + cost.SelectAndIdentCost + 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.action()
			if tracker.ActualCost() != tc.wantCost {
				t.Errorf("ActualCost() = %d, want %d", tracker.ActualCost(), tc.wantCost)
			}
		})
	}

	if !tracker.PresenceTestHasCost() {
		t.Errorf("PresenceTestHasCost() = false, want true")
	}
}

func TestCostTracker_LimitExceededPanic(t *testing.T) {
	var exceeded bool
	tracker, err := cost.NewTracker(nil,
		cost.TrackerLimit(15),
		cost.TrackerLimitExceededHandler(func() {
			exceeded = true
		}),
	)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	tracker.CreateList(1, nil) // cost = 10 <= 15
	if exceeded {
		t.Errorf("exceeded = true, want false")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on cost limit exceeded")
		}
		if !exceeded {
			t.Errorf("exceeded handler was not called")
		}
	}()

	tracker.CreateList(2, nil) // cost = 20 > 15 -> panic
}

func TestCostTracker_CustomOverloadTracker(t *testing.T) {
	tracker, err := cost.NewTracker(nil,
		cost.OverloadTracker("custom_op", func(args []ref.Val, result ref.Val) *uint64 {
			c := uint64(42)
			return &c
		}),
	)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	call := testCall{function: "custom", overloadID: "custom_op"}
	tracker.EvalZeroArity(nil, 1, call, types.IntZero)
	if tracker.ActualCost() != 42 {
		t.Errorf("ActualCost() = %d, want 42", tracker.ActualCost())
	}
}

func TestCostTracker_CloneStateIsolation(t *testing.T) {
	tracker, err := cost.NewTracker(nil)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}
	tracker.Qualify(1)

	clone, err := tracker.Clone()
	if err != nil {
		t.Fatalf("Clone() failed: %v", err)
	}
	if clone.ActualCost() != 0 {
		t.Errorf("clone.ActualCost() = %d, want 0", clone.ActualCost())
	}

	clone.Qualify(2)
	if clone.ActualCost() != 1 {
		t.Errorf("clone.ActualCost() = %d, want 1", clone.ActualCost())
	}
	if tracker.ActualCost() != 1 {
		t.Errorf("tracker.ActualCost() = %d, want 1", tracker.ActualCost())
	}
}

func TestCostTracker_StandardStringFunctionTracking(t *testing.T) {
	adapter := types.DefaultTypeAdapter

	tests := []struct {
		name       string
		overloadID string
		function   string
		target     ref.Val
		arg        ref.Val
		result     ref.Val
		wantCost   uint64
	}{
		{
			name:       "starts_with_string",
			overloadID: overloads.StartsWithString,
			function:   "startsWith",
			target:     types.String("hello world"),
			arg:        types.String("hello"), // len 5 -> ceil(5 * 0.1) = 1
			result:     types.True,
			wantCost:   1,
		},
		{
			name:       "ends_with_string",
			overloadID: overloads.EndsWithString,
			function:   "endsWith",
			target:     types.String("hello world"),
			arg:        types.String("world"), // len 5 -> ceil(5 * 0.1) = 1
			result:     types.True,
			wantCost:   1,
		},
		{
			name:       "contains_string",
			overloadID: overloads.ContainsString,
			function:   "contains",
			target:     types.String("hello world"),
			arg:        types.String("lo wo"), // len 5 -> ceil(11*0.1) * ceil(5*0.1) = 2 * 1 = 2
			result:     types.True,
			wantCost:   2,
		},
		{
			name:       "in_list_string",
			overloadID: overloads.InList,
			function:   "@in",
			target:     types.String("item"),
			arg:        adapter.NativeToValue([]string{"a", "b", "c"}),
			result:     types.False,
			wantCost:   3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracker, err := cost.NewTracker(nil)
			if err != nil {
				t.Fatalf("NewTracker() failed: %v", err)
			}
			call := testCall{function: tc.function, overloadID: tc.overloadID}
			tracker.EvalBinary(nil, 1, call, tc.target, tc.arg, tc.result)
			if tracker.ActualCost() != tc.wantCost {
				t.Errorf("ActualCost() = %d, want %d", tracker.ActualCost(), tc.wantCost)
			}
		})
	}
}

func TestTrackCostAdvanced(t *testing.T) {
	equalCases := []struct {
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
				t.Fatalf("Program.Eval(activation) failed to eval expression due: %v", err)
			}
			rhsCost, _, err := computeCost(t, tc.rhsExpr, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Program.Eval(activation) failed to eval expression due: %v", err)
			}
			if lhsCost != rhsCost {
				t.Errorf(`Program.Eval(activation) failed return a cost for %s of %d equal to a cost for %s of %d`,
					tc.lhsExpr, lhsCost, tc.rhsExpr, rhsCost)
			}
		})

	}
	smallerCases := []struct {
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
				t.Fatalf("Program.Eval(activation) failed to eval expression due: %v", err)
			}
			rhsCost, _, err := computeCost(t, tc.rhsExpr, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Program.Eval(activation) failed to eval expression due: %v", err)
			}
			if lhsCost >= rhsCost {
				t.Errorf(`Program.Eval(activation) failed return a cost for %s of %d less than the cost for %s of %d`,
					tc.lhsExpr, lhsCost, tc.rhsExpr, rhsCost)
			}
		})
	}
}

func computeCost(t *testing.T, expr string, vars []*decls.VariableDecl, ctx cel.Activation, options []cost.TrackerOption) (actualCost uint64, est cost.CostEstimate, err error) {
	t.Helper()

	env, err := testCelEnv.Extend(cel.VariableDecls(vars...))
	if err != nil {
		t.Fatalf("env.Extend() failed: %v", err)
	}
	checked, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
	}

	// The estimate must be configured with the same presence test behavior as the tracker,
	// so derive it from a tracker built with the same options.
	tracker, err := cost.NewTracker(nil, options...)
	if err != nil {
		t.Fatalf("cost.NewTracker() failed: %v", err)
	}
	est, err = cost.Cost(checked.NativeRep(), testTrackerCostEstimator{},
		cost.PresenceTestHasCost(tracker.PresenceTestHasCost()))
	if err != nil {
		t.Fatalf("cost.Cost() failed: %v", err)
	}

	prg, err := env.Program(checked,
		cel.CostTracking(&testRuntimeCostEstimator{}),
		cel.CostTrackerOptions(options...))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	// Program.Eval recovers evaluation panics itself, and attaches the cost tracker to the
	// details even when evaluation fails, so a cost limit breach still reports its cost.
	_, det, err := prg.Eval(ctx)
	if cost := det.ActualCost(); cost != nil {
		actualCost = *cost
	}
	return actualCost, est, err
}

func constructActivation(t testing.TB, in any) cel.Activation {
	t.Helper()
	if in == nil {
		return cel.NoVars()
	}
	a, err := cel.NewActivation(in)
	if err != nil {
		t.Fatalf("cel.NewActivation(%v) failed: %v", in, err)
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

type testTrackerCostEstimator struct {
	hints map[string]int64
}

func (tc testTrackerCostEstimator) EstimateSize(element cost.AstNode) *cost.SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &cost.SizeEstimate{Min: 0, Max: uint64(l)}
	}
	return nil
}

func (tc testTrackerCostEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	switch overloadID {
	case overloads.TimestampToYear:
		return &cost.CallEstimate{CostEstimate: cost.FixedCostEstimate(7)}
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
	nestedMapStr := types.NewMapType(types.StringType, types.NewMapType(types.StringType, types.StringType))
	cases := []struct {
		name         string
		expr         string
		vars         []*decls.VariableDecl
		want         uint64
		in           any
		testFuncCost bool
		limit        uint64
		options      []cost.TrackerOption

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
			name: "optional select: map",
			expr: `input.?key`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			want: 2,
			in:   map[string]any{"input": map[string]string{"key": "v"}},
		},
		{
			name: "optional index: map",
			expr: `input[?'key']`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			want: 2,
			in:   map[string]any{"input": map[string]string{"key": "v"}},
		},
		{
			// An optional select extends the attribute chain, so the trailing selection
			// only adds a qualifier cost rather than a second attribute resolution.
			name: "optional select: chained",
			expr: `input.?key.subkey`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedMapStr)},
			want: 3,
			in:   map[string]any{"input": map[string]map[string]string{"key": {"subkey": "v"}}},
		},
		{
			name: "optional index: chained",
			expr: `input[?'key'].subkey`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedMapStr)},
			want: 3,
			in:   map[string]any{"input": map[string]map[string]string{"key": {"subkey": "v"}}},
		},
		{
			// A computed operand requires a relative attribute, which costs an extra
			// attribute resolution on top of the qualifier.
			name: "optional index: map literal",
			expr: `{'key': 'v'}[?'key']`,
			want: 32,
		},
		{
			name: "optional index: list literal",
			expr: `['v'][?0]`,
			want: 12,
		},
		{
			name:    "select: field test only no has() cost",
			expr:    `has(input.single_int32)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			want:    1,
			options: []cost.TrackerOption{cost.TrackerPresenceTestHasCost(false)},
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
			options: []cost.TrackerOption{cost.TrackerPresenceTestHasCost(true)},
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
			options: []cost.TrackerOption{cost.TrackerPresenceTestHasCost(false)},
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
			name: "matches global",
			expr: `matches(input, '\\d+a\\d+b')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			want: 103,
			in:   map[string]any{"input": string(randSeq(500))},
		},
		{
			name: "startsWith",
			expr: `input.startsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			want: 52,
			in:   map[string]any{"input": "idc", "arg1": string(randSeq(500))},
		},
		{
			name: "endsWith",
			expr: `input.endsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			want: 52,
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
			options: []cost.TrackerOption{
				cost.OverloadTracker(overloads.ContainsString,
					func(args []ref.Val, result ref.Val) *uint64 {
						strCost := uint64(math.Ceil(float64(cost.ActualSize(args[0])) * 0.2))
						substrCost := uint64(math.Ceil(float64(cost.ActualSize(args[1])) * 0.2))
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
		// cel.bind runtime cost tracking test cases
		{
			name: "bind: literal init and scalar result",
			expr: `cel.bind(a, 'hello', a + '!')`,
			want: 12,
		},
		{
			name: "bind: nested binds",
			expr: `cel.bind(a, 'hello!', cel.bind(b, 'goodbye', a + ' and, ' + b))`,
			want: 26,
		},
		{
			name: "bind: shadowed bind",
			expr: `cel.bind(a, cel.bind(a, 'world', a + '!'), 'hello ' + a)`,
			want: 25,
		},
		{
			name: "bind: with variable list and index",
			expr: `cel.bind(a, input, a[0])`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", intList)},
			want: 13,
			in:   map[string]any{"input": []int{1, 2}},
		},
		{
			name: "bind: with variable map and index",
			expr: `cel.bind(m, input, m['key'])`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			want: 13,
			in:   map[string]any{"input": map[string]string{"key": "value"}},
		},
		{
			name: "bind: with comprehension over empty list",
			vars: []*decls.VariableDecl{decls.NewVariable("input", allList)},
			expr: `cel.bind(a, input, a.all(x, true))`,
			want: 13,
			in: map[string]any{
				"input": []*proto3pb.TestAllTypes{},
			},
		},
		{
			name: "bind: with list and indexing",
			expr: `cel.bind(a, [1, 2, 3], a[0])`,
			want: 22,
		},
		{
			name: "bind: derived size propagation to comprehension",
			expr: `cel.bind(v, [1, 2, 3], v.all(x, true))`,
			want: 31,
		},
		{
			name:               "bind: limit exceeded",
			expr:               `cel.bind(a, [1, 2, 3], a.all(x, true))`,
			limit:              25,
			expectExceedsLimit: true,
		},

		// Two-variable comprehension runtime cost tracking test cases
		{
			name: "two-var all: list literal",
			expr: `[1, 2, 3].all(i, v, i < v)`,
			want: 29,
		},
		{
			name: "two-var all: list variable early return false",
			vars: []*decls.VariableDecl{decls.NewVariable("input", intList)},
			expr: `input.all(i, v, i > v) == false`,
			want: 11,
			in:   map[string]any{"input": []int{1, 2, 3}},
		},
		{
			name: "two-var all: list variable",
			vars: []*decls.VariableDecl{decls.NewVariable("input", intList)},
			expr: `input.all(i, v, i < 5)`,
			want: 17,
			in:   map[string]any{"input": []int{1, 2, 3}},
		},
		{
			name: "two-var all: map literal early return false",
			expr: `{'hello': 'world'}.all(k, v, k != v) == false`,
			want: 38,
		},
		{
			name: "two-var exists: list literal",
			expr: `[1, 2, 3].exists(i, v, i == 1 && v == 2)`,
			want: 28,
		},
		{
			name: "two-var exists: map literal",
			expr: `{"a": 1}.exists(k, v, k == "a" && v == 1)`,
			want: 39,
		},
		{
			name: "two-var existsOne: list variable",
			vars: []*decls.VariableDecl{decls.NewVariable("input", intList)},
			expr: `input.existsOne(i, v, v == 1)`,
			want: 11,
			in:   map[string]any{"input": []int{1, 2, 3}},
		},
		{
			name: "two-var exists_one: list variable",
			vars: []*decls.VariableDecl{decls.NewVariable("input", intList)},
			expr: `input.exists_one(i, v, v == 1)`,
			want: 11,
			in:   map[string]any{"input": []int{1, 2, 3}},
		},
		{
			name: "two-var transformList: 3-arg list literal",
			expr: `[1, 2, 3].transformList(i, v, i + v)`,
			want: 66,
		},
		{
			name: "two-var transformList: 4-arg with filter list literal",
			expr: `[1, 2, 3].transformList(i, v, i % 2 == 0, i + v)`,
			want: 60,
		},
		{
			name: "two-var transformList: 3-arg map literal",
			expr: `{"a": 1, "b": 2}.transformList(k, v, k)`,
			want: 67,
		},
		{
			name: "two-var transformMap: 3-arg map literal",
			expr: `{"a": 1, "b": 2}.transformMap(k, v, v + 1)`,
			want: 71,
		},
		{
			name: "two-var transformMap: 4-arg with filter map literal",
			expr: `{"a": 1, "b": 2}.transformMap(k, v, v > 1, v + 1)`,
			want: 70,
		},
		{
			name: "two-var transformMapEntry: 3-arg map literal",
			expr: `{"a": 1, "b": 2}.transformMapEntry(k, v, {v: k})`,
			want: 129,
		},
		{
			name: "two-var transformMapEntry: 4-arg with filter map literal",
			expr: `{"a": 1, "b": 2}.transformMapEntry(k, v, v > 1, {v: k})`,
			want: 99,
		},
		{
			name: "two-var nested all",
			expr: `[1, 2].all(i, v, [1, 2].all(j, w, i + j < v + w))`,
			want: 79,
		},
		{
			name: "bind with two-var comprehension",
			expr: `cel.bind(l, [1, 2, 3], l.all(i, v, i < v))`,
			want: 40,
		},
		{
			name: "bind with two-var transformList",
			expr: `cel.bind(m, {"a": 1, "b": 2}, m.transformList(k, v, k))`,
			want: 78,
		},
		{
			name:               "two-var transformList: limit exceeded",
			expr:               `[1, 2, 3, 4, 5].transformList(i, v, i + v)`,
			limit:              50,
			expectExceedsLimit: true,
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
				options = append(options, cost.TrackerLimit(*costLimit))
			}
			actualCost, est, err := computeCost(t, tc.expr, tc.vars, ctx, options)
			if err != nil {
				if tc.expectExceedsLimit {
					return
				}
				t.Fatalf("Program.Eval(activation) failed due to: %v", err)
			}
			if tc.expectExceedsLimit {
				t.Fatalf("Program.Eval(activation) failed to return a cost exceeded error for limit %d, got cost %d", tc.limit, actualCost)
			}
			if actualCost != tc.want {
				t.Fatalf("Program.Eval(activation) failed to return expected runtime cost %d, got %d", tc.want, actualCost)
			}
			if est.Min > actualCost || est.Max < actualCost {
				t.Fatalf("Program.Eval(activation) failed to return cost in range of estimate cost [%d, %d], got %d",
					est.Min, est.Max, actualCost)
			}
		})
	}
}

func BenchmarkCostTracking(b *testing.B) {
	benchmarks := []struct {
		name string
		expr string
		vars []*decls.VariableDecl
		in   map[string]any
	}{
		{
			name: "simple_comparison",
			expr: "x > 10",
			vars: []*decls.VariableDecl{decls.NewVariable("x", types.IntType)},
			in:   map[string]any{"x": 15},
		},
		{
			name: "function_calls",
			expr: "str.startsWith('hello') && str.endsWith('world')",
			vars: []*decls.VariableDecl{decls.NewVariable("str", types.StringType)},
			in:   map[string]any{"str": "hello beautiful world"},
		},
		{
			name: "comprehension",
			expr: "[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map(x, x * 2).filter(x, x > 10)",
		},
		{
			name: "nested_comprehensions",
			expr: "[1, 2, 3, 4, 5].all(i, [1, 2, 3, 4, 5].exists(j, i + j == 6))",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			env, err := testCelEnv.Extend(cel.VariableDecls(bm.vars...))
			if err != nil {
				b.Fatalf("env.Extend() failed: %v", err)
			}
			checked, iss := env.Compile(bm.expr)
			if iss.Err() != nil {
				b.Fatalf("env.Compile(%q) failed: %v", bm.expr, iss.Err())
			}
			prg, err := env.Program(checked, cel.CostTracking(nil))
			if err != nil {
				b.Fatalf("env.Program(%s) failed: %v", bm.expr, err)
			}

			ctx := constructActivation(b, bm.in)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				prg.Eval(ctx)
			}
		})
	}
}

type testConcurrentSizingStrategy struct{}

func (testConcurrentSizingStrategy) EstimateSize(ctx cost.EstimateContext, node cost.AstNode) (cost.SizeEstimate, bool) {
	return cost.FixedSizeEstimate(10), true
}

func (testConcurrentSizingStrategy) TrackSize(ctx cost.TrackContext, value ref.Val) (uint64, bool) {
	return 10, true
}

func TestTracker_ConcurrentCloneRace(t *testing.T) {
	tracker, err := cost.NewTracker(nil, cost.TrackerSizingStrategy(testConcurrentSizingStrategy{}))
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clone, err := tracker.Clone()
			if err != nil {
				t.Errorf("tracker.Clone() failed: %v", err)
				return
			}
			clone.CostCall(testCall{function: "startsWith", overloadID: overloads.StartsWithString}, []ref.Val{types.String("hello"), types.String("h")}, types.True)
			clone.CostCall(testCall{function: "_==_", overloadID: overloads.Equals}, []ref.Val{types.String("a"), types.String("b")}, types.False)
			clone.CreateList(1, nil)
			if clone.ActualCost() == 0 {
				t.Errorf("clone.ActualCost() should be non-zero")
			}
		}()
	}
	wg.Wait()
}
