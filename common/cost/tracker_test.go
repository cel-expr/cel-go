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
	"sync"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
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
			lhsCost, _, err := computeCost(t, tc.lhsExpr, nil, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Program.Eval(activation) failed to eval expression due: %v", err)
			}
			rhsCost, _, err := computeCost(t, tc.rhsExpr, nil, nil, ctx, nil)
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
			lhsCost, _, err := computeCost(t, tc.lhsExpr, nil, nil, ctx, nil)
			if err != nil {
				t.Fatalf("Program.Eval(activation) failed to eval expression due: %v", err)
			}
			rhsCost, _, err := computeCost(t, tc.rhsExpr, nil, nil, ctx, nil)
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
