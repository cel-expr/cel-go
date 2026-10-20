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

package cost

import (
	"fmt"
	"math"
	"testing"

	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// TestOptionalTrackedSize checks the size each strategy tracks for an optional at each cost model
// version.
//
// From version 1 an optional is 1 plus the size of the value it holds: its length under the
// default strategy, and its aggregate size under the aggregate strategy. An empty optional is 1.
// Version 0 sizes every optional as 1 under the default strategy, as cel-go v0.32.0 charged it;
// the aggregate strategy, which v0.32.0 did not have, is the same at every version.
func TestOptionalTrackedSize(t *testing.T) {
	adapter := types.DefaultTypeAdapter
	threeStrings := types.NewStringList(adapter, []string{"ab", "cde", "f"})
	tests := []struct {
		name string
		val  ref.Val
		// want is the size at version 1 under the default and the aggregate strategies.
		wantDefault, wantAggregate uint64
	}{
		{name: "empty", val: types.OptionalNone, wantDefault: 1, wantAggregate: 1},
		{name: "string", val: types.OptionalOf(types.String("abcd")), wantDefault: 5, wantAggregate: 5},
		{name: "multi-byte string counts code points or bytes", val: types.OptionalOf(types.String("héllo")),
			wantDefault: 6, wantAggregate: 7},
		{name: "empty string", val: types.OptionalOf(types.String("")), wantDefault: 1, wantAggregate: 2},
		{name: "int", val: types.OptionalOf(types.Int(42)), wantDefault: 2, wantAggregate: 2},
		// A list of 3 strings has length 3, and aggregate size 1 + 2 + 3 + 1.
		{name: "list is shallow or aggregate", val: types.OptionalOf(threeStrings), wantDefault: 4, wantAggregate: 8},
		{name: "nested optional", val: types.OptionalOf(types.OptionalOf(types.String("ab"))),
			wantDefault: 4, wantAggregate: 4},
		{name: "nested empty optional", val: types.OptionalOf(types.OptionalNone), wantDefault: 2, wantAggregate: 2},
	}
	for _, tc := range tests {
		for _, v := range []uint32{0, 1, defaultModelVersion} {
			for name, strategy := range map[string]SizingStrategy{"default": DefaultSizingStrategy(), "aggregate": AggregateSizingStrategy()} {
				want := tc.wantAggregate
				if name == "default" {
					want = tc.wantDefault
					if v == 0 {
						want = 1
					}
				}
				t.Run(fmt.Sprintf("%s/%s/v%d", tc.name, name, v), func(t *testing.T) {
					m := Overload("test", argumentsOnly(), EvalCost(Arg(0)))
					tracker := m.FunctionTrackerWithOptions(WithSizingStrategy(strategy), ModelVersion(v))
					if got := *tracker([]ref.Val{tc.val}, nil); got != want {
						t.Errorf("Arg(0) track = %d, want %d", got, want)
					}
				})
			}
		}
	}
}

// TestOptionalQuantity checks the estimate and track of Optional, and that ElemOf recovers the
// value an optional holds.
func TestOptionalQuantity(t *testing.T) {
	val := RangedSizeEstimate(3, 5)
	tests := []struct {
		name     string
		version  uint32
		expr     QuantityExpr
		want     SizeEstimate
		wantElem *SizeEstimate
	}{
		{name: "value", version: 1, expr: Optional(Arg(0)), want: RangedSizeEstimate(4, 6), wantElem: &val},
		{name: "empty", version: 1, expr: Optional(Const(0)), want: FixedSizeEstimate(1), wantElem: &zeroSizeEstimate},
		{name: "element is the value", version: 1, expr: ElemOf(Optional(Arg(0))), want: val},
		{name: "version 0 is 1", version: 0, expr: Optional(Arg(0)), want: FixedSizeEstimate(1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Overload("test", argumentsOnly(), EvalCost(Const(0)), ResultSize(tc.expr))
			est := m.FunctionEstimatorWithOptions(ModelVersion(tc.version))(nil, nil, []AstNode{typedArgNode(types.StringType, val)})
			got := *est.ResultSize
			if got.Min != tc.want.Min || got.Max != tc.want.Max {
				t.Errorf("estimate = [%d, %d], want [%d, %d]", got.Min, got.Max, tc.want.Min, tc.want.Max)
			}
			if tc.wantElem != nil && (got.Elem == nil || *got.Elem != *tc.wantElem) {
				t.Errorf("estimate element = %v, want %v", got.Elem, *tc.wantElem)
			}
		})
	}

	// The track of Optional is 1 plus the value's track, and 1 at version 0.
	m := Overload("test", argumentsOnly(), EvalCost(Optional(Arg(0))))
	for v, want := range map[uint32]uint64{0: 1, 1: 5} {
		if got := *m.FunctionTrackerWithOptions(ModelVersion(v))([]ref.Val{types.String("abcd")}, nil); got != want {
			t.Errorf("Optional(Arg(0)) track at version %d = %d, want %d", v, got, want)
		}
	}

	// ElemOf an optional tracks the value it holds, and nothing for an empty optional.
	elem := Overload("test", argumentsOnly(), EvalCost(ElemOf(Arg(0))))
	for val, want := range map[ref.Val]uint64{types.OptionalOf(types.String("abcd")): 4, types.OptionalNone: 0} {
		if got := *elem.FunctionTrackerWithOptions()([]ref.Val{val}, nil); got != want {
			t.Errorf("ElemOf(Arg(0)) track of %v = %d, want %d", val, got, want)
		}
	}
}

// TestStandardOptionalModels checks the cost and result size the standard models estimate for the
// optional functions at each cost model version.
func TestStandardOptionalModels(t *testing.T) {
	value := typedArgNode(types.StringType, RangedSizeEstimate(3, 5))
	alt := typedArgNode(types.StringType, FixedSizeEstimate(7))
	held := RangedSizeEstimate(3, 5)
	optional := optionalSizeOf(held)
	optTarget := typedArgNode(types.NewOptionalType(types.StringType), optional)
	optAlt := typedArgNode(types.NewOptionalType(types.StringType), optionalSizeOf(FixedSizeEstimate(7)))
	tests := []struct {
		overload string
		target   AstNode
		args     []AstNode
		version  uint32
		wantCost CostEstimate
		// wantSize is nil where the model reports no result size.
		wantSize *SizeEstimate
	}{
		{overload: overloads.OptionalNone, version: 1, wantCost: FixedCostEstimate(1), wantSize: ptr(FixedSizeEstimate(1))},
		{overload: overloads.OptionalNone, version: 0, wantCost: FixedCostEstimate(1)},
		{overload: overloads.OptionalOf, args: []AstNode{value}, version: 1, wantCost: FixedCostEstimate(1), wantSize: ptr(RangedSizeEstimate(4, 6))},
		{overload: overloads.OptionalOf, args: []AstNode{value}, version: 0, wantCost: FixedCostEstimate(1)},
		// ofNonZeroValue may be empty, so its value may be 0.
		{overload: overloads.OptionalOfNonZeroValue, args: []AstNode{value}, version: 1, wantCost: FixedCostEstimate(1), wantSize: ptr(RangedSizeEstimate(1, 6))},
		{overload: overloads.OptionalValue, target: optTarget, version: 1, wantCost: FixedCostEstimate(1), wantSize: &held},
		{overload: overloads.OptionalValue, target: optTarget, version: 0, wantCost: FixedCostEstimate(1)},
		// orValue returns the receiver's value or the alternative; or returns either optional.
		{overload: overloads.OptionalOrValueValue, target: optTarget, args: []AstNode{alt}, version: 1, wantCost: FixedCostEstimate(0), wantSize: ptr(RangedSizeEstimate(3, 7))},
		{overload: overloads.OptionalOrOptional, target: optTarget, args: []AstNode{optAlt}, version: 1, wantCost: FixedCostEstimate(0), wantSize: ptr(RangedSizeEstimate(4, 8))},
		// Before version 1 these were unmodeled calls, which v0.32.0 estimated at 1.
		{overload: overloads.OptionalOrValueValue, target: optTarget, args: []AstNode{alt}, version: 0, wantCost: FixedCostEstimate(1)},
		{overload: overloads.OptionalOrOptional, target: optTarget, args: []AstNode{optAlt}, version: 0, wantCost: FixedCostEstimate(1)},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s/v%d", tc.overload, tc.version), func(t *testing.T) {
			var target *AstNode
			if tc.target != nil {
				target = &tc.target
			}
			est := StandardOverloadEstimatorsWithOptions(ModelVersion(tc.version))[tc.overload](nil, target, tc.args)
			if est.CostEstimate != tc.wantCost {
				t.Errorf("cost = %v, want %v", est.CostEstimate, tc.wantCost)
			}
			switch {
			case tc.wantSize == nil && est.ResultSize != nil:
				t.Errorf("result size = %v, want none", *est.ResultSize)
			case tc.wantSize != nil && est.ResultSize == nil:
				t.Errorf("result size = none, want [%d, %d]", tc.wantSize.Min, tc.wantSize.Max)
			case tc.wantSize != nil && (est.ResultSize.Min != tc.wantSize.Min || est.ResultSize.Max != tc.wantSize.Max):
				t.Errorf("result size = [%d, %d], want [%d, %d]", est.ResultSize.Min, est.ResultSize.Max, tc.wantSize.Min, tc.wantSize.Max)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

// TestComparisonEstimate checks the estimate of a comparison at each cost model version.
//
// Version 0 keeps the v0.32.0 estimate, ceil(0.1 * [m, min(lhs.Max, rhs.Max)]) where m is 1 when
// the smaller maximum is non-zero. From version 1 the estimate is the model's,
// max(1, ceil(0.1 * [min(lhs.Min, rhs.Min), min(lhs.Max, rhs.Max)])), saturating for an unknown
// size: a comparison costs at least minCallCost, even of empty operands.
func TestComparisonEstimate(t *testing.T) {
	unknown := UnknownSizeEstimate()
	scaledUnknown := FixedCostEstimate(math.MaxUint64).MultiplyByCostFactor(StringTraversalCostFactor).Max
	tests := []struct {
		name       string
		lhs, rhs   SizeEstimate
		wantV0     CostEstimate
		wantLatest CostEstimate
	}{
		{name: "possibly empty operand", lhs: RangedSizeEstimate(0, 20), rhs: FixedSizeEstimate(2),
			wantV0: FixedCostEstimate(1), wantLatest: FixedCostEstimate(1)},
		{name: "long operands", lhs: RangedSizeEstimate(17, 34), rhs: FixedSizeEstimate(11),
			wantV0: RangedCostEstimate(1, 2), wantLatest: FixedCostEstimate(2)},
		{name: "empty operands", lhs: FixedSizeEstimate(0), rhs: FixedSizeEstimate(0),
			wantV0: FixedCostEstimate(0), wantLatest: FixedCostEstimate(1)},
		{name: "unknown operands", lhs: unknown, rhs: unknown,
			wantV0: RangedCostEstimate(1, scaledUnknown), wantLatest: RangedCostEstimate(1, math.MaxUint64)},
	}
	for _, overload := range []string{overloads.Equals, overloads.NotEquals, overloads.LessString, overloads.GreaterEqualsBytes} {
		for _, tc := range tests {
			t.Run(overload+"/"+tc.name, func(t *testing.T) {
				args := []AstNode{typedArgNode(types.StringType, tc.lhs), typedArgNode(types.StringType, tc.rhs)}
				for version, want := range map[uint32]CostEstimate{0: tc.wantV0, 1: tc.wantLatest} {
					est := StandardOverloadEstimatorsWithOptions(ModelVersion(version))[overload](nil, nil, args)
					if est.CostEstimate != want {
						t.Errorf("version %d estimate = %v, want %v", version, est.CostEstimate, want)
					}
				}
			})
		}
	}
}
