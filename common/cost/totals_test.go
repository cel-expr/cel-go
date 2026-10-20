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
	"testing"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// estimateModelCost compiles a single-argument global model with the given sizing strategy and
// returns its cost estimate for an argument of the given size. The model is exempt from minCallCost,
// so that the estimate is the bare quantity's.
func estimateModelCost(t *testing.T, cost QuantityExpr, strategy SizingStrategy, arg SizeEstimate) CostEstimate {
	t.Helper()
	m := Overload("test", argumentsOnly(), EvalCost(cost))
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
	est := m.FunctionEstimatorWithOptions(WithSizingStrategy(strategy))(
		nil, nil, []AstNode{NewAstNode(nil, nil, nil, &arg)})
	return est.CostEstimate
}

// trackModelCost compiles a single-argument global model and returns its tracked cost for the
// given argument value. The model is exempt from minCallCost, so that the cost is the bare
// quantity's.
func trackModelCost(t *testing.T, cost QuantityExpr, strategy SizingStrategy, arg ref.Val) uint64 {
	t.Helper()
	m := Overload("test", argumentsOnly(), EvalCost(cost))
	return *m.FunctionTrackerWithOptions(WithSizingStrategy(strategy))([]ref.Val{arg}, nil)
}

// opaqueSizingStrategy is a custom strategy whose relationship between size and contents is
// unknown to the cost model.
type opaqueSizingStrategy struct{ defaultSizingStrategy }

func TestElemTotalEstimate(t *testing.T) {
	tests := []struct {
		name     string
		strategy SizingStrategy
		arg      SizeEstimate
		want     CostEstimate
	}{
		{
			name:     "default strategy multiplies the length by the element size",
			strategy: DefaultSizingStrategy(),
			arg:      ListSizeEstimate(RangedSizeEstimate(2, 5), RangedSizeEstimate(1, 3)),
			want:     RangedCostEstimate(2, 15),
		},
		{
			name:     "default strategy without element metadata is unknown",
			strategy: DefaultSizingStrategy(),
			arg:      FixedSizeEstimate(3),
			want:     UnknownCostEstimate(),
		},
		{
			name:     "default strategy empty list totals zero",
			strategy: DefaultSizingStrategy(),
			arg:      FixedSizeEstimate(0),
			want:     FixedCostEstimate(0),
		},
		{
			// A list of 3 strings of at most 5 bytes has aggregate size in [4, 16]; its elements
			// total its content size.
			name:     "aggregate strategy reads the content size",
			strategy: AggregateSizingStrategy(),
			arg:      ListSizeEstimate(RangedSizeEstimate(4, 16), RangedSizeEstimate(1, 5)),
			want:     RangedCostEstimate(3, 15),
		},
		{
			name:     "aggregate strategy map values are a share of the content",
			strategy: AggregateSizingStrategy(),
			arg:      MapSizeEstimate(RangedSizeEstimate(5, 11), FixedSizeEstimate(1), RangedSizeEstimate(1, 4)),
			want:     RangedCostEstimate(0, 10),
		},
		{
			name:     "aggregate strategy empty list totals zero",
			strategy: AggregateSizingStrategy(),
			arg:      FixedSizeEstimate(1),
			want:     FixedCostEstimate(0),
		},
		{
			name:     "aggregate strategy keeps unknown",
			strategy: AggregateSizingStrategy(),
			arg:      UnknownSizeEstimate(),
			want:     UnknownCostEstimate(),
		},
		{
			name:     "custom strategy is unknown",
			strategy: opaqueSizingStrategy{},
			arg:      ListSizeEstimate(RangedSizeEstimate(2, 5), FixedSizeEstimate(3)),
			want:     UnknownCostEstimate(),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := estimateModelCost(t, ElemTotal(Arg(0)), tc.strategy, tc.arg); got != tc.want {
				t.Errorf("ElemTotal(Arg(0)) estimate = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTotalsOfTypedOperands checks that the totals consult the checked type of their operand: a
// value without elements totals 0, as it tracks, and a map's values are only part of its content,
// whether or not its estimate states a key size.
func TestTotalsOfTypedOperands(t *testing.T) {
	mapType := types.NewMapType(types.StringType, types.IntType)
	tests := []struct {
		name     string
		total    QuantityExpr
		strategy SizingStrategy
		argType  *types.Type
		arg      SizeEstimate
		want     CostEstimate
	}{
		{name: "aggregate string has no elements", total: ElemTotal(Arg(0)), strategy: AggregateSizingStrategy(),
			argType: types.StringType, arg: FixedSizeEstimate(5), want: FixedCostEstimate(0)},
		{name: "default string has no elements", total: ElemTotal(Arg(0)), strategy: DefaultSizingStrategy(),
			argType: types.StringType, arg: FixedSizeEstimate(5), want: FixedCostEstimate(0)},
		{name: "string has no keys", total: KeyTotal(Arg(0)), strategy: AggregateSizingStrategy(),
			argType: types.StringType, arg: FixedSizeEstimate(5), want: FixedCostEstimate(0)},
		{name: "aggregate map values without key metadata", total: ElemTotal(Arg(0)), strategy: AggregateSizingStrategy(),
			argType: mapType, arg: FixedSizeEstimate(9), want: RangedCostEstimate(0, 8)},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := Overload("test", argumentsOnly(), EvalCost(tc.total))
			est := m.FunctionEstimatorWithOptions(WithSizingStrategy(tc.strategy))(
				nil, nil, []AstNode{typedArgNode(tc.argType, tc.arg)})
			if est.CostEstimate != tc.want {
				t.Errorf("estimate = %v, want %v", est.CostEstimate, tc.want)
			}
		})
	}
}

// TestTotalsWithinEstimate checks that the tracked totals lie within the estimates, under each
// built-in strategy, for values whose size estimates are exact.
func TestTotalsWithinEstimate(t *testing.T) {
	adapter := types.DefaultTypeAdapter
	lst := types.NewStringList(adapter, []string{"abcde", "ab", "abc"})
	m := adapter.NativeToValue(map[string]string{"a": "x", "bb": "yy", "ccc": "zzz"})
	for _, strategy := range []SizingStrategy{DefaultSizingStrategy(), AggregateSizingStrategy()} {
		for _, tc := range []struct {
			name string
			expr QuantityExpr
			val  ref.Val
		}{
			{name: "elem total of list", expr: ElemTotal(Arg(0)), val: lst},
			{name: "elem total of map", expr: ElemTotal(Arg(0)), val: m},
			{name: "key total of map", expr: KeyTotal(Arg(0)), val: m},
		} {
			t.Run(tc.name, func(t *testing.T) {
				sz := exactSizeEstimate(strategy, tc.val)
				est := estimateModelCost(t, tc.expr, strategy, sz)
				got := trackModelCost(t, tc.expr, strategy, tc.val)
				if got < est.Min || got > est.Max {
					t.Errorf("track %d outside estimate %v (size %v)", got, est, sz)
				}
			})
		}
	}
}

// exactSizeEstimate reports the size estimate of a list or map value under a strategy, with
// element and key sizes bounded by the smallest and largest present.
func exactSizeEstimate(strategy SizingStrategy, val ref.Val) SizeEstimate {
	size := func(v ref.Val) uint64 {
		n, _ := strategy.TrackSize(nil, v)
		return n
	}
	bounds := func(vals []ref.Val) *SizeEstimate {
		if len(vals) == 0 {
			return nil
		}
		lo, hi := size(vals[0]), size(vals[0])
		for _, v := range vals[1:] {
			lo, hi = min(lo, size(v)), max(hi, size(v))
		}
		b := RangedSizeEstimate(lo, hi)
		return &b
	}
	var keys, elems []ref.Val
	eachElement(val, func(v ref.Val) { elems = append(elems, v) })
	eachKey(val, func(v ref.Val) { keys = append(keys, v) })
	sz := FixedSizeEstimate(size(val))
	sz.Key, sz.Elem = bounds(keys), bounds(elems)
	return sz
}

func TestKeyTotal(t *testing.T) {
	adapter := types.DefaultTypeAdapter
	m := adapter.NativeToValue(map[string]string{"a": "x", "bb": "yy", "ccc": "zzz"})

	t.Run("track sums the keys", func(t *testing.T) {
		if got := trackModelCost(t, KeyTotal(Arg(0)), DefaultSizingStrategy(), m); got != 6 {
			t.Errorf("KeyTotal(Arg(0)) track = %d, want 6", got)
		}
	})
	t.Run("track of a list is zero", func(t *testing.T) {
		lst := types.NewStringList(adapter, []string{"a", "b"})
		if got := trackModelCost(t, KeyTotal(Arg(0)), DefaultSizingStrategy(), lst); got != 0 {
			t.Errorf("KeyTotal(Arg(0)) track = %d, want 0", got)
		}
	})
	t.Run("estimate multiplies length by key size", func(t *testing.T) {
		arg := MapSizeEstimate(RangedSizeEstimate(1, 3), RangedSizeEstimate(1, 4), FixedSizeEstimate(9))
		got := estimateModelCost(t, KeyTotal(Arg(0)), DefaultSizingStrategy(), arg)
		if want := RangedCostEstimate(1, 12); got != want {
			t.Errorf("KeyTotal(Arg(0)) estimate = %v, want %v", got, want)
		}
	})
	t.Run("estimate without key metadata is unknown", func(t *testing.T) {
		got := estimateModelCost(t, KeyTotal(Arg(0)), DefaultSizingStrategy(), FixedSizeEstimate(3))
		if !got.IsUnknown() {
			t.Errorf("KeyTotal(Arg(0)) estimate = %v, want unknown", got)
		}
	})
}
