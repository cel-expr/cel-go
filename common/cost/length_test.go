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

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// typedArgNode returns an AstNode with the given checked type and size estimate.
func typedArgNode(t *types.Type, sz SizeEstimate) AstNode {
	return NewAstNode(nil, nil, t, &sz)
}

func TestLengthEstimate(t *testing.T) {
	tests := []struct {
		name     string
		strategy SizingStrategy
		argType  *types.Type
		arg      SizeEstimate
		want     CostEstimate
	}{
		{name: "default string", strategy: DefaultSizingStrategy(), argType: types.StringType,
			arg: RangedSizeEstimate(3, 9), want: RangedCostEstimate(3, 9)},
		{name: "aggregate string takes 1 to 4 bytes per code point", strategy: AggregateSizingStrategy(),
			argType: types.StringType, arg: RangedSizeEstimate(9, 20), want: RangedCostEstimate(3, 20)},
		{name: "aggregate string of size 1 may be empty", strategy: AggregateSizingStrategy(),
			argType: types.StringType, arg: FixedSizeEstimate(1), want: RangedCostEstimate(0, 1)},
		{name: "aggregate bytes", strategy: AggregateSizingStrategy(), argType: types.BytesType,
			arg: RangedSizeEstimate(5, 8), want: RangedCostEstimate(5, 8)},
		{name: "aggregate list", strategy: AggregateSizingStrategy(), argType: types.NewListType(types.IntType),
			arg: ListSizeEstimate(FixedSizeEstimate(4), FixedSizeEstimate(1)), want: RangedCostEstimate(3, 4)},
		{name: "int has length 1", strategy: AggregateSizingStrategy(), argType: types.IntType,
			arg: FixedSizeEstimate(1), want: FixedCostEstimate(1)},
		{name: "aggregate dyn", strategy: AggregateSizingStrategy(), argType: types.DynType,
			arg: RangedSizeEstimate(2, 7), want: RangedCostEstimate(0, 7)},
		{name: "custom strategy", strategy: opaqueSizingStrategy{}, argType: types.StringType,
			arg: FixedSizeEstimate(4), want: UnknownCostEstimate()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Overload("test", argumentsOnly(), EvalCost(Length(Arg(0))))
			if err := m.Validate(); err != nil {
				t.Fatalf("Validate() failed: %v", err)
			}
			est := m.FunctionEstimatorWithOptions(WithSizingStrategy(tc.strategy))(
				nil, nil, []AstNode{typedArgNode(tc.argType, tc.arg)})
			if est.CostEstimate != tc.want {
				t.Errorf("Length(Arg(0)) estimate = %v, want %v", est.CostEstimate, tc.want)
			}
		})
	}
}

// TestLengthEstimateLiteral checks that the length of a literal is estimated exactly under every
// strategy, as Size() reports it at runtime: code points for a string, rather than its UTF-8 bytes.
func TestLengthEstimateLiteral(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		lit  ref.Val
		typ  *types.Type
		want uint64
	}{
		{name: "multi-byte string counts code points", lit: types.String("héllo"), typ: types.StringType, want: 5},
		{name: "empty string", lit: types.String(""), typ: types.StringType, want: 0},
		{name: "bytes", lit: types.Bytes("héllo"), typ: types.BytesType, want: 6},
		{name: "int", lit: types.Int(42), typ: types.IntType, want: 1},
	}
	for _, tc := range tests {
		for _, strategy := range []SizingStrategy{DefaultSizingStrategy(), AggregateSizingStrategy()} {
			node := NewAstNode(fac.NewLiteral(1, tc.lit), nil, tc.typ, nil)
			want := FixedCostEstimate(tc.want)
			global := Overload("test", argumentsOnly(), EvalCost(Length(Arg(0)))).FunctionEstimatorWithOptions(WithSizingStrategy(strategy))
			if got := global(nil, nil, []AstNode{node}).CostEstimate; got != want {
				t.Errorf("%s: Length(Arg(0)) estimate = %v, want %v", tc.name, got, want)
			}
			member := MemberOverload("test", argumentsOnly(), EvalCost(Length(Target()))).FunctionEstimatorWithOptions(WithSizingStrategy(strategy))
			if got := member(nil, &node, nil).CostEstimate; got != want {
				t.Errorf("%s: Length(Target()) estimate = %v, want %v", tc.name, got, want)
			}
			if got := trackModelCost(t, Length(Arg(0)), strategy, tc.lit); got != tc.want {
				t.Errorf("%s: Length(Arg(0)) track = %d, want %d", tc.name, got, tc.want)
			}
		}
	}
}

// TestLengthTrack checks that Length charges the same under every strategy: the length cost.ActualSize
// reports.
func TestLengthTrack(t *testing.T) {
	adapter := types.DefaultTypeAdapter
	tests := []struct {
		name string
		val  ref.Val
		want uint64
	}{
		{name: "string counts code points", val: types.String("héllo"), want: 5},
		{name: "bytes", val: types.Bytes("héllo"), want: 6},
		{name: "list", val: types.NewStringList(adapter, []string{"abc", "de"}), want: 2},
		{name: "map", val: adapter.NativeToValue(map[string]int{"a": 1}), want: 1},
		{name: "int", val: types.Int(42), want: 1},
		{name: "error has no length", val: types.NewErr("failed"), want: 0},
		{name: "optional has the length of its value", val: types.OptionalOf(types.String("abc")), want: 3},
		{name: "empty optional", val: types.OptionalNone, want: 0},
	}
	for _, tc := range tests {
		for _, strategy := range []SizingStrategy{DefaultSizingStrategy(), AggregateSizingStrategy()} {
			if got := trackModelCost(t, Length(Arg(0)), strategy, tc.val); got != tc.want {
				t.Errorf("%s: Length(Arg(0)) track = %d, want %d", tc.name, got, tc.want)
			}
		}
	}
}
