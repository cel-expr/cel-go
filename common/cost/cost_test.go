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
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/ext"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestSafeSubtract(t *testing.T) {
	tests := []struct {
		name string
		x, y uint64
		want uint64
	}{
		{name: "zero", x: 0, y: 0, want: 0},
		{name: "simple", x: 5, y: 3, want: 2},
		{name: "underflow to zero", x: 3, y: 5, want: 0},
		{name: "max minus zero", x: math.MaxUint64, y: 0, want: math.MaxUint64},
		{name: "max minus one", x: math.MaxUint64, y: 1, want: math.MaxUint64},
		{name: "max minus max", x: math.MaxUint64, y: math.MaxUint64, want: math.MaxUint64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeSubtract(tc.x, tc.y); got != tc.want {
				t.Errorf("SafeSubtract(%d, %d) got %d, want %d", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

func TestSafeAdd(t *testing.T) {
	tests := []struct {
		name string
		x, y uint64
		rest []uint64
		want uint64
	}{
		{name: "zero", x: 0, y: 0, want: 0},
		{name: "simple", x: 2, y: 3, want: 5},
		{name: "variadic", x: 1, y: 2, rest: []uint64{3, 4}, want: 10},
		{name: "max plus zero", x: math.MaxUint64, y: 0, want: math.MaxUint64},
		{name: "overflow", x: math.MaxUint64, y: 1, want: math.MaxUint64},
		{name: "overflow near max", x: math.MaxUint64 - 5, y: 10, want: math.MaxUint64},
		{name: "overflow in rest", x: 1, y: 2, rest: []uint64{math.MaxUint64}, want: math.MaxUint64},
		{name: "saturated stays saturated", x: math.MaxUint64, y: math.MaxUint64,
			rest: []uint64{math.MaxUint64}, want: math.MaxUint64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeAdd(tc.x, tc.y, tc.rest...); got != tc.want {
				t.Errorf("SafeAdd(%d, %d, %v) got %d, want %d", tc.x, tc.y, tc.rest, got, tc.want)
			}
		})
	}
}

func TestSafeMultiply(t *testing.T) {
	tests := []struct {
		name string
		x, y uint64
		want uint64
	}{
		{name: "zero", x: 0, y: 0, want: 0},
		{name: "max by zero", x: math.MaxUint64, y: 0, want: 0},
		{name: "simple", x: 3, y: 4, want: 12},
		{name: "max by one", x: math.MaxUint64, y: 1, want: math.MaxUint64},
		{name: "overflow", x: math.MaxUint64, y: 2, want: math.MaxUint64},
		{name: "overflow squared", x: math.MaxUint32, y: math.MaxUint32 * 2, want: math.MaxUint64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeMultiply(tc.x, tc.y); got != tc.want {
				t.Errorf("SafeMultiply(%d, %d) got %d, want %d", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

func TestSafeMultiplyByFactor(t *testing.T) {
	tests := []struct {
		name   string
		x      uint64
		factor float64
		want   uint64
	}{
		{name: "zero value", x: 0, factor: 0.1, want: 0},
		{name: "zero factor", x: 100, factor: 0, want: 0},
		{name: "rounds up", x: 15, factor: 0.1, want: 2},
		{name: "exact", x: 10, factor: 0.1, want: 1},
		{name: "whole factor", x: 10, factor: 3, want: 30},
		{name: "max saturates", x: math.MaxUint64, factor: 2, want: math.MaxUint64},
		{name: "max scaled down", x: math.MaxUint64, factor: 0.1, want: 1844674407370955264},
		{name: "negative factor", x: 10, factor: -1, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeMultiplyByFactor(tc.x, tc.factor); got != tc.want {
				t.Errorf("SafeMultiplyByFactor(%d, %f) got %d, want %d", tc.x, tc.factor, got, tc.want)
			}
		})
	}
}

func TestSafeCeil(t *testing.T) {
	tests := []struct {
		name string
		x    float64
		want uint64
	}{
		{name: "zero", x: 0, want: 0},
		{name: "negative", x: -1.5, want: 0},
		{name: "nan", x: math.NaN(), want: 0},
		{name: "fraction", x: 0.1, want: 1},
		{name: "rounds up", x: 2.5, want: 3},
		{name: "whole", x: 3.0, want: 3},
		{name: "infinity", x: math.Inf(1), want: math.MaxUint64},
		{name: "out of range", x: math.Ldexp(1.0, 64), want: math.MaxUint64},
		{name: "largest in range", x: math.Ldexp(1.0, 63), want: 1 << 63},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeCeil(tc.x); got != tc.want {
				t.Errorf("SafeCeil(%f) got %d, want %d", tc.x, got, tc.want)
			}
		})
	}
}

func TestSafeTrunc(t *testing.T) {
	tests := []struct {
		name string
		x    float64
		want uint64
	}{
		{name: "zero", x: 0, want: 0},
		{name: "negative", x: -1.5, want: 0},
		{name: "nan", x: math.NaN(), want: math.MaxUint64},
		{name: "fraction truncates to zero", x: 0.9, want: 0},
		{name: "rounds down", x: 2.5, want: 2},
		{name: "whole", x: 3.0, want: 3},
		{name: "infinity", x: math.Inf(1), want: math.MaxUint64},
		// Out-of-range float to uint64 conversion is platform-defined: amd64 yields
		// 0x8000000000000000 and arm64 saturates. Both must report the saturated value.
		{name: "out of range", x: math.Ldexp(1.0, 64), want: math.MaxUint64},
		{name: "far out of range", x: math.Ldexp(1.0, 96), want: math.MaxUint64},
		{name: "largest in range", x: math.Ldexp(1.0, 63), want: 1 << 63},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeTrunc(tc.x); got != tc.want {
				t.Errorf("SafeTrunc(%f) got %d, want %d", tc.x, got, tc.want)
			}
		})
	}
}

func TestSafeMultiplyByFactorTrunc(t *testing.T) {
	tests := []struct {
		name   string
		x      uint64
		factor float64
		want   uint64
	}{
		{name: "zero value", x: 0, factor: 0.1, want: 0},
		{name: "zero factor", x: 100, factor: 0, want: 0},
		{name: "rounds down", x: 15, factor: 0.1, want: 1},
		{name: "exact", x: 10, factor: 0.1, want: 1},
		{name: "whole factor", x: 10, factor: 3, want: 30},
		{name: "max saturates", x: math.MaxUint64, factor: 2, want: math.MaxUint64},
		{name: "max scaled down", x: math.MaxUint64, factor: 0.1, want: 1844674407370955264},
		{name: "negative factor", x: 10, factor: -1, want: 0},
		// The list and sets trackers scale an already-saturated size by a factor > 1.
		{name: "saturated size", x: math.MaxUint64, factor: 2.1, want: math.MaxUint64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cost.SafeMultiplyByFactorTrunc(tc.x, tc.factor); got != tc.want {
				t.Errorf("SafeMultiplyByFactorTrunc(%d, %f) got %d, want %d", tc.x, tc.factor, got, tc.want)
			}
		})
	}
}

// TestSafeMultiplyByFactorTruncMatchesUnchecked verifies that the saturating helper agrees with a
// direct conversion for every in-range input, so that adopting it cannot change a reported cost.
func TestSafeMultiplyByFactorTruncMatchesUnchecked(t *testing.T) {
	factors := []float64{0.1, 1.0, 2.0, 2.1, 3.0}
	sizes := []uint64{0, 1, 2, 5, 10, 100, 1_000, 65_535, 1 << 32}
	for _, f := range factors {
		for _, sz := range sizes {
			want := uint64(float64(sz) * f)
			if got := cost.SafeMultiplyByFactorTrunc(sz, f); got != want {
				t.Errorf("SafeMultiplyByFactorTrunc(%d, %f) got %d, want %d", sz, f, got, want)
			}
		}
	}
}

func TestSizeEstimate(t *testing.T) {
	s1 := cost.FixedSizeEstimate(5)
	s2 := cost.FixedSizeEstimate(10)

	tests := []struct {
		name string
		got  cost.SizeEstimate
		want cost.SizeEstimate
	}{
		{
			name: "add",
			got:  s1.Add(s2),
			want: cost.SizeEstimate{Min: 15, Max: 15},
		},
		{
			name: "multiply",
			got:  s1.Multiply(s2),
			want: cost.SizeEstimate{Min: 50, Max: 50},
		},
		{
			name: "union",
			got:  s1.Union(s2),
			want: cost.SizeEstimate{Min: 5, Max: 10},
		},
		{
			name: "unknown_size_estimate",
			got:  cost.UnknownSizeEstimate(),
			want: cost.SizeEstimate{Min: 0, Max: math.MaxUint64},
		},
		{
			name: "ranged_size_estimate",
			got:  cost.RangedSizeEstimate(3, 8),
			want: cost.SizeEstimate{Min: 3, Max: 8},
		},
		{
			name: "at_least_one_size",
			got:  cost.AtLeastOneSize(cost.FixedSizeEstimate(0)),
			want: cost.SizeEstimate{Min: 1, Max: 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}

	costTests := []struct {
		name string
		got  cost.CostEstimate
		want cost.CostEstimate
	}{
		{
			name: "multiply_by_cost_factor",
			got:  s1.MultiplyByCostFactor(0.5),
			want: cost.CostEstimate{Min: 3, Max: 3},
		},
		{
			name: "multiply_by_cost",
			got:  s1.MultiplyByCost(cost.FixedCostEstimate(4)),
			want: cost.CostEstimate{Min: 20, Max: 20},
		},
		{
			name: "as_cost",
			got:  s1.AsCost(),
			want: cost.CostEstimate{Min: 5, Max: 5},
		},
	}
	for _, tc := range costTests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
}

func TestCostEstimate(t *testing.T) {
	c1 := cost.FixedCostEstimate(5)
	c2 := cost.FixedCostEstimate(10)
	cr := cost.RangedCostEstimate(5, 10)

	tests := []struct {
		name string
		got  cost.CostEstimate
		want cost.CostEstimate
	}{
		{
			name: "add",
			got:  c1.Add(c2),
			want: cost.FixedCostEstimate(15),
		},
		{
			name: "multiply",
			got:  c1.Multiply(c2),
			want: cost.FixedCostEstimate(50),
		},
		{
			name: "union",
			got:  c1.Union(c2),
			want: cost.RangedCostEstimate(5, 10),
		},
		{
			name: "ranged_cost_estimate",
			got:  cr,
			want: cost.RangedCostEstimate(5, 10),
		},
		{
			name: "multiply_by_cost_factor",
			got:  c1.MultiplyByCostFactor(0.5),
			want: cost.FixedCostEstimate(3),
		},
		{
			name: "unknown_cost_estimate",
			got:  cost.UnknownCostEstimate(),
			want: cost.RangedCostEstimate(0, math.MaxUint64),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
	if got := cost.RangedCostEstimate(3, 8); got.Min != 3 || got.Max != 8 {
		t.Errorf("RangedCostEstimate(3, 8) = %v, want {3, 8}", got)
	}
}

func TestExtCostHelpers(t *testing.T) {
	sz := cost.FixedSizeEstimate(10)

	tests := []struct {
		name     string
		callEst  *cost.CallEstimate
		wantCost cost.CostEstimate
		wantSize *cost.SizeEstimate
	}{
		{
			name: "estimate_string_scan",
			callEst: func() *cost.CallEstimate {
				costEst, resSz := cost.EstimateStringScan(sz)
				return cost.NewCallEstimate(costEst, resSz)
			}(),
			wantCost: cost.FixedCostEstimate(1),
			wantSize: &sz,
		},
		{
			name: "estimate_list_alloc",
			callEst: func() *cost.CallEstimate {
				allocCost, allocSz := cost.EstimateListAlloc(sz, 0.5)
				return cost.NewCallEstimate(allocCost, allocSz)
			}(),
			wantCost: cost.FixedCostEstimate(15),
			wantSize: &sz,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.callEst.CostEstimate != tc.wantCost {
				t.Errorf("CostEstimate = %v, want %v", tc.callEst.CostEstimate, tc.wantCost)
			}
			if (tc.callEst.ResultSize == nil) != (tc.wantSize == nil) ||
				(tc.callEst.ResultSize != nil && *tc.callEst.ResultSize != *tc.wantSize) {
				t.Errorf("ResultSize = %v, want %v", tc.callEst.ResultSize, tc.wantSize)
			}
		})
	}
}

func TestSizeEstimate_Subtract(t *testing.T) {
	tests := []struct {
		name string
		s1   cost.SizeEstimate
		s2   cost.SizeEstimate
		want cost.SizeEstimate
	}{
		{
			name: "ranged_subtract",
			s1:   cost.RangedSizeEstimate(5, 15),
			s2:   cost.RangedSizeEstimate(2, 4),
			want: cost.SizeEstimate{Min: 1, Max: 13},
		},
		{
			name: "underflow_subtract",
			s1:   cost.RangedSizeEstimate(2, 4),
			s2:   cost.RangedSizeEstimate(5, 10),
			want: cost.SizeEstimate{Min: 0, Max: 0},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s1.Subtract(tc.s2); got != tc.want {
				t.Errorf("%v.Subtract(%v) = %v, want %v", tc.s1, tc.s2, got, tc.want)
			}
		})
	}
}

type testHintsEstimator struct {
	hints map[string]uint64
}

func (t testHintsEstimator) EstimateSize(element cost.AstNode) *cost.SizeEstimate {
	if sz, ok := t.hints[strings.Join(element.Path(), ".")]; ok {
		res := cost.FixedSizeEstimate(sz)
		return &res
	}
	return nil
}

func (t testHintsEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	return nil
}

func TestEstimateSize(t *testing.T) {
	compSz := cost.FixedSizeEstimate(42)
	nodeWithComp := cost.NewAstNode(nil, nil, types.IntType, &compSz)
	nodeWithoutComp := cost.NewAstNode(nil, []string{"foo"}, types.IntType, nil)
	nodeUnknown := cost.NewAstNode(nil, []string{"bar"}, types.IntType, nil)
	est := testHintsEstimator{hints: map[string]uint64{"foo": 100}}

	tests := []struct {
		name      string
		estimator cost.Estimator
		node      cost.AstNode
		want      cost.SizeEstimate
	}{
		{
			name:      "nil_node",
			estimator: nil,
			node:      nil,
			want:      cost.UnknownSizeEstimate(),
		},
		{
			name:      "computed_size",
			estimator: nil,
			node:      nodeWithComp,
			want:      compSz,
		},
		{
			name:      "with_estimator",
			estimator: est,
			node:      nodeWithoutComp,
			want:      cost.FixedSizeEstimate(100),
		},
		{
			name:      "estimator_returns_nil",
			estimator: est,
			node:      nodeUnknown,
			want:      cost.UnknownSizeEstimate(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if sz := cost.EstimateSize(tc.estimator, tc.node); sz != tc.want {
				t.Errorf("EstimateSize() = %v, want %v", sz, tc.want)
			}
		})
	}
}

func TestNodeAsUintValue(t *testing.T) {
	fac := ast.NewExprFactory()
	identNode := cost.NewAstNode(fac.NewIdent(1, "x"), nil, types.IntType, nil)
	strNode := cost.NewAstNode(fac.NewLiteral(2, types.String("hello")), nil, types.StringType, nil)
	posIntNode := cost.NewAstNode(fac.NewLiteral(3, types.Int(42)), nil, types.IntType, nil)
	negIntNode := cost.NewAstNode(fac.NewLiteral(4, types.Int(-5)), nil, types.IntType, nil)
	uintNode := cost.NewAstNode(fac.NewLiteral(5, types.Uint(100)), nil, types.UintType, nil)

	tests := []struct {
		name       string
		node       cost.AstNode
		defaultVal uint64
		want       uint64
	}{
		{
			name:       "nil_node",
			node:       nil,
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "non_literal_ident",
			node:       identNode,
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "non_int_literal_string",
			node:       strNode,
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "positive_int_literal",
			node:       posIntNode,
			defaultVal: 99,
			want:       42,
		},
		{
			name:       "negative_int_literal_saturates_zero",
			node:       negIntNode,
			defaultVal: 99,
			want:       0,
		},
		{
			name:       "uint_literal",
			node:       uintNode,
			defaultVal: 99,
			want:       100,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if val := cost.NodeAsUintValue(tc.node, tc.defaultVal); val != tc.want {
				t.Errorf("NodeAsUintValue() = %d, want %d", val, tc.want)
			}
		})
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randSeq(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}

type testRuntimeCostEstimator struct{}

var timeToYearCost uint64 = 7

func (e testRuntimeCostEstimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	argsSize := make([]uint64, len(args))
	for i, arg := range args {
		reflectV := reflect.ValueOf(arg.Value())
		switch reflectV.Kind() {
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
	hints map[string]uint64
}

func (tc testCostEstimator) EstimateSize(element cost.AstNode) *cost.SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		est := cost.RangedSizeEstimate(0, l)
		return &est
	}
	if element.Type() == types.BytesType {
		est := cost.RangedSizeEstimate(0, 12)
		return &est
	}
	return nil
}

func (tc testCostEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	switch overloadID {
	case overloads.TimestampToYear:
		return &cost.CallEstimate{CostEstimate: cost.FixedCostEstimate(7)}
	}
	return nil
}

var testCelEnv = func() *cel.Env {
	env, err := cel.NewEnv(
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.CrossTypeNumericComparisons(true),
		cel.OptionalTypes(),
		ext.Bindings(),
		ext.TwoVarComprehensions(),
		cel.Function("max",
			cel.MemberOverload("list_bytes_max",
				[]*cel.Type{cel.ListType(cel.BytesType)}, cel.BytesType)),
	)
	if err != nil {
		panic(fmt.Sprintf("cel.NewEnv() failed: %v", err))
	}
	return env
}()

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

func estPtr(e cost.CostEstimate) *cost.CostEstimate {
	return &e
}

func trackPtr(u uint64) *uint64 {
	return &u
}

func listElementNode(list cost.AstNode) cost.AstNode {
	if params := list.Type().Parameters(); len(params) > 0 {
		lt := params[0]
		nodePath := list.Path()
		if nodePath != nil {
			path := make([]string, len(nodePath)+1)
			copy(path, nodePath)
			path[len(nodePath)] = "@items"
			return cost.NewAstNode(nil, path, lt, nil)
		}
		return cost.NewAstNode(nil, nil, lt, nil)
	}
	return nil
}

func estimateSize(estimator cost.Estimator, node cost.AstNode) cost.SizeEstimate {
	if l := node.ComputedSize(); l != nil {
		return *l
	}
	if l := estimator.EstimateSize(node); l != nil {
		return *l
	}
	return cost.RangedSizeEstimate(0, math.MaxUint64)
}

func sizeEstimate(estimator cost.Estimator, t cost.AstNode) cost.SizeEstimate {
	if sz := t.ComputedSize(); sz != nil {
		return *sz
	}
	if sz := estimator.EstimateSize(t); sz != nil {
		return *sz
	}
	return cost.RangedSizeEstimate(0, math.MaxUint64)
}

func computeCost(t *testing.T, expr string, vars []*decls.VariableDecl, hints map[string]uint64, ctx cel.Activation, options []cost.TrackerOption) (actualCost uint64, est cost.CostEstimate, err error) {
	t.Helper()

	env, err := testCelEnv.Extend(cel.VariableDecls(vars...))
	if err != nil {
		t.Fatalf("env.Extend() failed: %v", err)
	}
	checked, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
	}

	tracker, err := cost.NewTracker(nil, options...)
	if err != nil {
		t.Fatalf("cost.NewTracker() failed: %v", err)
	}
	costOpts := []cost.Option{
		cost.PresenceTestHasCost(tracker.PresenceTestHasCost()),
	}
	est, err = cost.Cost(checked.NativeRep(), testCostEstimator{hints: hints}, costOpts...)
	if err != nil {
		t.Fatalf("cost.Cost() failed: %v", err)
	}

	prg, err := env.Program(checked,
		cel.CostTracking(&testRuntimeCostEstimator{}),
		cel.CostTrackerOptions(options...))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, det, err := prg.Eval(ctx)
	if c := det.ActualCost(); c != nil {
		actualCost = *c
	}
	return actualCost, est, err
}

func TestCostEstimateAndTracking(t *testing.T) {
	allTypes := types.NewObjectType("google.expr.proto3.test.TestAllTypes")
	allList := types.NewListType(allTypes)
	intList := types.NewListType(types.IntType)
	nestedList := types.NewListType(allList)

	allMap := types.NewMapType(types.StringType, allTypes)
	nestedMap := types.NewMapType(types.StringType, allMap)
	nestedMapStr := types.NewMapType(types.StringType, types.NewMapType(types.StringType, types.StringType))

	zeroCost := cost.CostEstimate{}
	oneCost := cost.FixedCostEstimate(1)

	cases := []struct {
		name               string
		expr               string
		vars               []*decls.VariableDecl
		hints              map[string]uint64
		in                 any
		trackerOptions     []cost.TrackerOption
		estimateOptions    []cost.Option
		limit              uint64
		expectExceedsLimit bool
		wantEst            *cost.CostEstimate
		wantEstV0          *cost.CostEstimate
		wantTrack          *uint64
	}{
		{
			name:      "const",
			expr:      `"Hello World!"`,
			wantEst:   estPtr(zeroCost),
			wantTrack: trackPtr(0),
		},
		{
			name:      "identity",
			expr:      `input`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", intList)},
			in:        map[string]any{"input": []int{1, 2}},
			wantEst:   estPtr(cost.CostEstimate{Min: 1, Max: 1}),
			wantTrack: trackPtr(1),
		},
		{
			name: "select: map",
			expr: `input['key']`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			in:        map[string]any{"input": map[string]string{"key": "v"}},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 2}),
			wantTrack: trackPtr(2),
		},
		{
			name: "select: field",
			expr: `input.single_int32`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", allTypes)},
			in: map[string]any{
				"input": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 2}),
			wantTrack: trackPtr(2),
		},
		{
			name: "select: field test only no has() cost",
			expr: `has(input.single_int32)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			in: map[string]any{
				"input": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			trackerOptions:  []cost.TrackerOption{cost.TrackerPresenceTestHasCost(false)},
			estimateOptions: []cost.Option{cost.PresenceTestHasCost(false)},
			wantEst:         estPtr(cost.CostEstimate{Min: 1, Max: 1}),
			wantTrack:       trackPtr(1),
		},
		{
			name: "select: field test only",
			expr: `has(input.single_int32)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			in: map[string]any{
				"input": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 2}),
			wantTrack: trackPtr(2),
		},
		{
			name: "select: non-proto field test has() cost",
			expr: `has(input.testAttr.nestedAttr)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			in: map[string]any{
				"input": map[string]any{
					"testAttr": map[string]any{
						"nestedAttr": "0",
					},
				},
			},
			trackerOptions:  []cost.TrackerOption{cost.TrackerPresenceTestHasCost(true)},
			estimateOptions: []cost.Option{cost.PresenceTestHasCost(true)},
			wantEst:         estPtr(cost.CostEstimate{Min: 3, Max: 3}),
			wantTrack:       trackPtr(3),
		},
		{
			name: "select: non-proto field test no has() cost",
			expr: `has(input.testAttr.nestedAttr)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			in: map[string]any{
				"input": map[string]any{
					"testAttr": map[string]any{
						"nestedAttr": "0",
					},
				},
			},
			trackerOptions:  []cost.TrackerOption{cost.TrackerPresenceTestHasCost(false)},
			estimateOptions: []cost.Option{cost.PresenceTestHasCost(false)},
			wantEst:         estPtr(cost.CostEstimate{Min: 2, Max: 2}),
			wantTrack:       trackPtr(2),
		},
		{
			name: "select: non-proto field test",
			expr: `has(input.testAttr.nestedAttr)`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			in: map[string]any{
				"input": map[string]any{
					"testAttr": map[string]any{
						"nestedAttr": "0",
					},
				},
			},
			wantEst:   estPtr(cost.CostEstimate{Min: 3, Max: 3}),
			wantTrack: trackPtr(3),
		},
		{
			name:      "estimated function call",
			expr:      `input.getFullYear()`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.TimestampType)},
			in:        map[string]any{"input": time.Now()},
			wantEst:   estPtr(cost.CostEstimate{Min: 8, Max: 8}),
			wantTrack: trackPtr(8),
		},
		{
			name:      "create list",
			expr:      `[1, 2, 3]`,
			wantEst:   estPtr(cost.CostEstimate{Min: 10, Max: 10}),
			wantTrack: trackPtr(10),
		},
		{
			name:      "create struct",
			expr:      `google.expr.proto3.test.TestAllTypes{single_int32: 1, single_float: 3.14, single_string: 'str'}`,
			wantEst:   estPtr(cost.CostEstimate{Min: 40, Max: 40}),
			wantTrack: trackPtr(40),
		},
		{
			name:      "create map",
			expr:      `{"a": 1, "b": 2, "c": 3}`,
			wantEst:   estPtr(cost.CostEstimate{Min: 30, Max: 30}),
			wantTrack: trackPtr(30),
		},
		{
			name:  "all comprehension",
			expr:  `input.all(x, true)`,
			vars:  []*decls.VariableDecl{decls.NewVariable("input", allList)},
			hints: map[string]uint64{"input": 100},
			in: map[string]any{
				"input": []*proto3pb.TestAllTypes{},
			},
			wantEst:   estPtr(cost.RangedCostEstimate(2, 302)),
			wantTrack: trackPtr(2),
		},
		{
			name:  "nested all comprehension",
			expr:  `input.all(x, x.all(y, true))`,
			vars:  []*decls.VariableDecl{decls.NewVariable("input", nestedList)},
			hints: map[string]uint64{"input": 50, "input.@items": 10},
			in: map[string]any{
				"input": []*proto3pb.TestAllTypes{},
			},
			wantEst:   estPtr(cost.RangedCostEstimate(2, 1752)),
			wantTrack: trackPtr(2),
		},
		{
			name:      "all comprehension on literal",
			expr:      `[1, 2, 3].all(x, true)`,
			wantEst:   estPtr(cost.FixedCostEstimate(20)),
			wantTrack: trackPtr(20),
		},
		{
			name:      "variable cost function",
			expr:      `input.matches('[0-9]')`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			hints:     map[string]uint64{"input": 500},
			in:        map[string]any{"input": string(randSeq(500))},
			wantEst:   estPtr(cost.RangedCostEstimate(3, 103)),
			wantTrack: trackPtr(103),
		},
		{
			name:      "variable cost function with constant",
			expr:      `'123'.matches('[0-9]')`,
			wantEst:   estPtr(cost.FixedCostEstimate(2)),
			wantTrack: trackPtr(2),
		},
		{
			name:      "or",
			expr:      `true || false`,
			wantEst:   estPtr(zeroCost),
			wantTrack: trackPtr(0),
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
			wantEst:   estPtr(cost.RangedCostEstimate(1, 4)),
			wantTrack: trackPtr(4),
		},
		{
			name:      "and",
			expr:      `true && false`,
			wantEst:   estPtr(zeroCost),
			wantTrack: trackPtr(0),
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
			wantEst:   estPtr(cost.RangedCostEstimate(1, 4)),
			wantTrack: trackPtr(4),
		},
		{
			name:      "lt",
			expr:      `1 < 2`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "lte",
			expr:      `1 <= 2`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "eq",
			expr:      `1 == 2`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "gt",
			expr:      `2 > 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "gte",
			expr:      `2 >= 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "in",
			expr:      `2 in [1, 2, 3]`,
			wantEst:   estPtr(cost.FixedCostEstimate(13)),
			wantTrack: trackPtr(13),
		},
		{
			name:      "plus",
			expr:      `1 + 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "minus",
			expr:      `1 - 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "/",
			expr:      `1 / 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "/",
			expr:      `1 * 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "%",
			expr:      `1 % 1`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:      "ternary",
			expr:      `true ? 1 : 2`,
			wantEst:   estPtr(zeroCost),
			wantTrack: trackPtr(0),
		},
		{
			name:      "string size",
			expr:      `size("123")`,
			wantEst:   estPtr(oneCost),
			wantTrack: trackPtr(1),
		},
		{
			name:    "bytes size",
			expr:    `size(b"123")`,
			wantEst: estPtr(oneCost),
		},
		{
			name:      "bytes to string conversion",
			expr:      `string(input)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.BytesType)},
			hints:     map[string]uint64{"input": 500},
			in:        map[string]any{"input": randSeq(500)},
			wantEst:   estPtr(cost.CostEstimate{Min: 1, Max: 51}),
			wantTrack: trackPtr(51),
		},
		{
			name:      "bytes to string conversion equality",
			expr:      `string(input) == string(input)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.BytesType)},
			hints:     map[string]uint64{"input": 500},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 152}),
			wantEstV0: estPtr(cost.CostEstimate{Min: 3, Max: 152}),
		},
		{
			name:      "string to bytes conversion",
			expr:      `bytes(input)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			hints:     map[string]uint64{"input": 500},
			in:        map[string]any{"input": string(randSeq(500))},
			wantEst:   estPtr(cost.CostEstimate{Min: 1, Max: 51}),
			wantTrack: trackPtr(51),
		},
		{
			name:      "string to bytes conversion equality",
			expr:      `bytes(input) == bytes(input)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			hints:     map[string]uint64{"input": 500},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 302}),
			wantEstV0: estPtr(cost.CostEstimate{Min: 3, Max: 302}),
		},
		{
			name:      "int to string conversion",
			expr:      `string(1)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 1, Max: 1}),
			wantTrack: trackPtr(1),
		},
		{
			name: "contains",
			expr: `input.contains(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			hints:     map[string]uint64{"input": 500, "arg1": 500},
			in:        map[string]any{"input": string(randSeq(500)), "arg1": string(randSeq(500))},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 2502}),
			wantTrack: trackPtr(2502),
		},
		{
			name: "matches",
			expr: `input.matches('\\d+a\\d+b')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			hints:     map[string]uint64{"input": 500},
			in:        map[string]any{"input": string(randSeq(500)), "arg1": string(randSeq(500))},
			wantEst:   estPtr(cost.CostEstimate{Min: 3, Max: 103}),
			wantTrack: trackPtr(103),
		},
		{
			name: "matches global",
			expr: `matches(input, '\\d+a\\d+b')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			hints:     map[string]uint64{"input": 500},
			in:        map[string]any{"input": string(randSeq(500))},
			wantEst:   estPtr(cost.CostEstimate{Min: 3, Max: 103}),
			wantTrack: trackPtr(103),
		},
		{
			name: "startsWith",
			expr: `input.startsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			hints:     map[string]uint64{"arg1": 500},
			in:        map[string]any{"input": "idc", "arg1": string(randSeq(500))},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 52}),
			wantTrack: trackPtr(52),
		},
		{
			name: "endsWith",
			expr: `input.endsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			hints:     map[string]uint64{"arg1": 500},
			in:        map[string]any{"input": "idc", "arg1": string(randSeq(500))},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 52}),
			wantTrack: trackPtr(52),
		},
		{
			name: "size receiver",
			expr: `input.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			in:        map[string]any{"input": "500", "arg1": "500"},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 2}),
			wantTrack: trackPtr(2),
		},
		{
			name: "size",
			expr: `size(input)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			in:        map[string]any{"input": "500", "arg1": "500"},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 2}),
			wantTrack: trackPtr(2),
		},
		{
			name: "ternary eval",
			expr: `(x > 2 ? input1 : input2).all(y, true)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("input1", allList),
				decls.NewVariable("input2", allList),
			},
			hints:     map[string]uint64{"input1": 1, "input2": 1},
			in:        map[string]any{"input1": []*proto3pb.TestAllTypes{{}}, "input2": []*proto3pb.TestAllTypes{{}}, "x": 1},
			wantEst:   estPtr(cost.CostEstimate{Min: 4, Max: 7}),
			wantTrack: trackPtr(6),
		},
		{
			name: "comprehension over map",
			expr: `input.all(k, input[k].single_int32 > 3)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", allMap),
			},
			hints:     map[string]uint64{"input": 10},
			in:        map[string]any{"input": map[string]any{"val": &proto3pb.TestAllTypes{}}},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 82}),
			wantTrack: trackPtr(9),
		},
		{
			name: "comprehension over nested map of maps",
			expr: `input.all(k, input[k].all(x, true))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:     map[string]uint64{"input": 5, "input.@values": 10},
			in:        map[string]any{"input": map[string]any{}},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 187}),
			wantTrack: trackPtr(2),
		},
		{
			name: "string size of map keys",
			expr: `input.all(k, k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:     map[string]uint64{"input": 5, "input.@keys": 10},
			in:        map[string]any{"input": map[string]any{}},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 32}),
			wantTrack: trackPtr(2),
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:     map[string]uint64{"input": 2, "input.@values": 2, "input.@keys": 5},
			in:        map[string]any{"input": map[string]any{}},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 34}),
			wantTrack: trackPtr(2),
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:     map[string]uint64{"input": 2, "input.@values": 2, "input.@keys": 5},
			in:        map[string]any{"input": map[string]any{}},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 34}),
			wantTrack: trackPtr(2),
		},
		{
			name: "list concat",
			expr: `(list1 + list2).all(x, true)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			hints:     map[string]uint64{"list1": 10, "list2": 10},
			in:        map[string]any{"list1": []int{}, "list2": []int{}},
			wantEst:   estPtr(cost.CostEstimate{Min: 4, Max: 64}),
			wantTrack: trackPtr(4),
		},
		{
			name: "str concat",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			hints:     map[string]uint64{"str1": 10, "str2": 10},
			in:        map[string]any{"str1": "val1", "str2": "val2222222"},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 6}),
			wantTrack: trackPtr(6),
		},
		{
			name: "str concat custom cost estimate",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			hints: map[string]uint64{"str1": 10, "str2": 10},
			in:    map[string]any{"str1": "val1", "str2": "val2222222"},
			trackerOptions: []cost.TrackerOption{
				cost.OverloadTracker(overloads.ContainsString,
					func(args []ref.Val, result ref.Val) *uint64 {
						strCost := uint64(math.Ceil(float64(cost.ActualSize(args[0])) * 0.2))
						substrCost := uint64(math.Ceil(float64(cost.ActualSize(args[1])) * 0.2))
						cost := strCost * substrCost
						return &cost
					}),
			},
			estimateOptions: []cost.Option{
				cost.OverloadCostEstimate(overloads.ContainsString,
					func(estimator cost.Estimator, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
						if target != nil && len(args) == 1 {
							strSize := estimateSize(estimator, *target).MultiplyByCostFactor(0.2)
							subSize := estimateSize(estimator, args[0]).MultiplyByCostFactor(0.2)
							return &cost.CallEstimate{CostEstimate: strSize.Multiply(subSize)}
						}
						return nil
					}),
			},
			wantEst:   estPtr(cost.CostEstimate{Min: 2, Max: 12}),
			wantTrack: trackPtr(10),
		},
		{
			name: "list size comparison",
			expr: `list1.size() == list2.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 5, Max: 5}),
		},
		{
			name: "list size from ternary",
			expr: `x > y ? list1.size() : list2.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("y", types.IntType),
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 5, Max: 5}),
		},
		{
			name: "list size from concat",
			expr: `([x, y] + list1 + list2).size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("y", types.IntType),
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			hints: map[string]uint64{
				"list1": 10,
				"list2": 20,
			},
			wantEst: estPtr(cost.CostEstimate{Min: 17, Max: 17}),
		},
		{
			name: "list cost tracking through comprehension",
			expr: `[list1, list2].exists(l, l.exists(v, v.startsWith('hi')))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.StringType)),
				decls.NewVariable("list2", types.NewListType(types.StringType)),
			},
			hints: map[string]uint64{
				"list1":        10,
				"list1.@items": 64,
				"list2":        20,
				"list2.@items": 128,
			},
			wantEst: estPtr(cost.CostEstimate{Min: 21, Max: 265}),
		},
		{
			name: "str endsWith equality",
			expr: `str1.endsWith("abcdefghijklmnopqrstuvwxyz") == str2.endsWith("abcdefghijklmnopqrstuvwxyz")`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 9, Max: 9}),
		},
		{
			name:    "nested subexpression operators",
			expr:    `((5 != 6) == (1 == 2)) == ((3 <= 4) == (9 != 9))`,
			wantEst: estPtr(cost.CostEstimate{Min: 7, Max: 7}),
		},
		{
			name: "str size estimate",
			expr: `string(timestamp1) == string(timestamp2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("timestamp1", types.TimestampType),
				decls.NewVariable("timestamp2", types.TimestampType),
			},
			wantEst:   estPtr(cost.CostEstimate{Min: 4, Max: 1844674407370955268}),
			wantEstV0: estPtr(cost.CostEstimate{Min: 5, Max: 1844674407370955268}),
		},
		{
			name: "timestamp equality check",
			expr: `timestamp1 == timestamp2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("timestamp1", types.TimestampType),
				decls.NewVariable("timestamp2", types.TimestampType),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name: "duration inequality check",
			expr: `duration1 != duration2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("duration1", types.DurationType),
				decls.NewVariable("duration2", types.DurationType),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name:      ".filter list literal",
			expr:      `[1,2,3,4,5].filter(x, x % 2 == 0)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantEst:   estPtr(cost.CostEstimate{Min: 41, Max: 101}),
			wantTrack: trackPtr(62),
		},
		{
			name:      ".map list literal",
			expr:      `[1,2,3,4,5].map(x, x)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantEst:   estPtr(cost.CostEstimate{Min: 86, Max: 86}),
			wantTrack: trackPtr(86),
		},
		{
			name:      ".map.filter list literal",
			expr:      `[1,2,3,4,5].map(x, x).filter(x, x % 2 == 0)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantEst:   estPtr(cost.CostEstimate{Min: 117, Max: 177}),
			wantTrack: trackPtr(138),
		},
		{
			name:      ".map.exists list literal",
			expr:      `[1,2,3,4,5].map(x, x).exists(x, x == 5) == true`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantEst:   estPtr(cost.CostEstimate{Min: 108, Max: 118}),
			wantTrack: trackPtr(118),
		},
		{
			name:      ".map.map list literal",
			expr:      `[1,2,3,4,5].map(x, x).map(x, x)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantEst:   estPtr(cost.CostEstimate{Min: 162, Max: 162}),
			wantTrack: trackPtr(162),
		},
		{
			name:    ".map list literal selection",
			expr:    `[1,2,3,4,5].map(x, x)[4]`,
			wantEst: estPtr(cost.CostEstimate{Min: 88, Max: 88}),
		},
		{
			name:    "nested array selection",
			expr:    `[[1,2],[1,2],[1,2],[1,2],[1,2]][4]`,
			wantEst: estPtr(cost.CostEstimate{Min: 62, Max: 62}),
		},
		{
			name:    "nested map selection",
			expr:    `{'a': [1,2], 'b': [1,2], 'c': [1,2], 'd': [1,2], 'e': [1,2]}.b`,
			wantEst: estPtr(cost.CostEstimate{Min: 82, Max: 82}),
		},
		{
			name:    "comprehension on nested list",
			expr:    `[[1, 1], [2, 2], [3, 3], [4, 4], [5, 5]].all(y, y.all(y, y == 1))`,
			wantEst: estPtr(cost.CostEstimate{Min: 76, Max: 136}),
		},
		{
			name:      "comprehension on transformed nested list",
			expr:      `[1,2,3,4,5].map(x, [x, x]).all(y, y.all(y, y == 1))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 157, Max: 217}),
			wantTrack: trackPtr(171),
		},
		{
			name:    "comprehension on nested literal list",
			expr:    `["a", "ab", "abc", "abcd", "abcde"].map(x, [x, x]).all(y, y.all(y, y.startsWith('a')))`,
			wantEst: estPtr(cost.CostEstimate{Min: 157, Max: 217}),
		},
		{
			name: "comprehension on nested variable list",
			expr: `input.map(x, [x, x]).all(y, y.all(y, y.startsWith('a')))`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewListType(types.StringType))},
			hints: map[string]uint64{
				"input":        5,
				"input.@items": 10,
			},
			wantEst: estPtr(cost.CostEstimate{Min: 13, Max: 208}),
		},
		{
			name:      "comprehension chaining with concat",
			expr:      `[1,2,3,4,5].map(x, x).map(x, x) + [1]`,
			wantEst:   estPtr(cost.CostEstimate{Min: 173, Max: 173}),
			wantTrack: trackPtr(173),
		},
		{
			name:      "nested comprehension",
			expr:      `[1,2,3].all(i, i in [1,2,3].map(j, j + j))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 20, Max: 230}),
			wantTrack: trackPtr(86),
		},
		{
			name:    "nested dyn comprehension",
			expr:    `dyn([1,2,3]).all(i, i in dyn([1,2,3]).map(j, j + j))`,
			wantEst: estPtr(cost.CostEstimate{Min: 21, Max: 234}),
		},
		{
			name:    "literal map access",
			expr:    `{'hello': 'hi'}['hello'] != {'hello': 'bye'}['hello']`,
			wantEst: estPtr(cost.CostEstimate{Min: 65, Max: 65}),
		},
		{
			name:    "literal list access",
			expr:    `['hello', 'hi'][0] != ['hello', 'bye'][1]`,
			wantEst: estPtr(cost.CostEstimate{Min: 25, Max: 25}),
		},
		{
			name:    "literal map optional access",
			expr:    `{'hello': 'hi'}[?'hello']`,
			wantEst: estPtr(cost.CostEstimate{Min: 32, Max: 32}),
		},
		{
			name:    "literal map optional select",
			expr:    `{'hello': 'hi'}.?hello`,
			wantEst: estPtr(cost.CostEstimate{Min: 32, Max: 32}),
		},
		{
			name:    "literal list optional access",
			expr:    `['hello', 'hi'][?0]`,
			wantEst: estPtr(cost.CostEstimate{Min: 12, Max: 12}),
		},
		{
			name: "optional select chain",
			expr: `self.?val1.val2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.DynType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name: "optional index chain",
			expr: `self[?'val1'].val2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.DynType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name:    "type call",
			expr:    `type(1)`,
			wantEst: estPtr(cost.CostEstimate{Min: 1, Max: 1}),
		},
		{
			name: "type call variable",
			expr: `type(self.val1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.IntType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name: "type call variable equality",
			expr: `type(self.val1) == int`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.IntType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 5, Max: 5}),
		},
		{
			name:    "type literal equality cost",
			expr:    `type(1) == int`,
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name:    "type variable equality cost",
			expr:    `type(1) == int`,
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name: "namespace variable equality",
			expr: `self.val1 == 1.0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self.val1", types.DoubleType),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 2, Max: 2}),
		},
		{
			name: "simple map variable equality",
			expr: `self.val1 == 1.0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.DoubleType)),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 3, Max: 3}),
		},
		{
			name: "date-time math",
			expr: `self.val1 == timestamp('2011-08-18T00:00:00.000+01:00') + duration('19h3m37s10ms')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.TimestampType)),
			},
			wantEst: estPtr(cost.FixedCostEstimate(6)),
		},
		{
			name: "date-time math self-conversion",
			expr: `timestamp(self.val1) == timestamp('2011-08-18T00:00:00.000+01:00') + duration('19h3m37s10ms')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.TimestampType)),
			},
			wantEst: estPtr(cost.FixedCostEstimate(7)),
		},
		{
			name: "boolean vars equal",
			expr: `self.val1 != self.val2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.BoolType)),
			},
			wantEst: estPtr(cost.FixedCostEstimate(5)),
		},
		{
			name: "boolean var equals literal",
			expr: `self.val1 != true`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.BoolType)),
			},
			wantEst: estPtr(cost.FixedCostEstimate(3)),
		},
		{
			name: "double var equals literal",
			expr: `self.val1 == 1.0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.DoubleType)),
			},
			wantEst: estPtr(cost.FixedCostEstimate(3)),
		},
		{
			name: "bytes list max",
			expr: "[bytes('012345678901'), bytes('012345678901'), bytes('012345678901'), bytes('012345678901'), bytes('012345678901')].max()",
			estimateOptions: []cost.Option{
				cost.OverloadCostEstimate("list_bytes_max",
					func(estimator cost.Estimator, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
						if target != nil {
							// Charge 1 cost for comparing each element in the list
							elCost := cost.CostEstimate{Min: 1, Max: 1}
							// If the list contains strings or bytes, add the cost of traversing all the strings/bytes as a way
							// of estimating the additional comparison cost.
							if elNode := listElementNode(*target); elNode != nil {
								k := elNode.Type().Kind()
								if k == types.StringKind || k == types.BytesKind {
									sz := sizeEstimate(estimator, elNode)
									elCost = elCost.Add(sz.MultiplyByCostFactor(cost.StringTraversalCostFactor))
								}
								return &cost.CallEstimate{CostEstimate: sizeEstimate(estimator, *target).MultiplyByCost(elCost)}
							}
						}
						return nil
					}),
			},
			wantEst: estPtr(cost.CostEstimate{Min: 25, Max: 35}),
		},
		{
			name:      "bind: literal init and scalar result",
			expr:      `cel.bind(a, 'hello', a + '!')`,
			wantEst:   estPtr(cost.CostEstimate{Min: 12, Max: 12}),
			wantTrack: trackPtr(12),
		},
		{
			name:      "bind: nested binds",
			expr:      `cel.bind(a, 'hello!', cel.bind(b, 'goodbye', a + ' and, ' + b))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 26, Max: 26}),
			wantTrack: trackPtr(26),
		},
		{
			name:      "bind: shadowed bind",
			expr:      `cel.bind(a, cel.bind(a, 'world', a + '!'), 'hello ' + a)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 25, Max: 25}),
			wantTrack: trackPtr(25),
		},
		{
			name:      "bind: with variable list and index",
			expr:      `cel.bind(a, input, a[0])`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", intList)},
			in:        map[string]any{"input": []int{1, 2}},
			wantEst:   estPtr(cost.CostEstimate{Min: 13, Max: 13}),
			wantTrack: trackPtr(13),
		},
		{
			name:      "bind: with variable map and index",
			expr:      `cel.bind(m, input, m['key'])`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			in:        map[string]any{"input": map[string]string{"key": "value"}},
			wantEst:   estPtr(cost.CostEstimate{Min: 13, Max: 13}),
			wantTrack: trackPtr(13),
		},
		{
			name:  "bind: with comprehension and size hints",
			expr:  `cel.bind(a, input, a.all(x, true))`,
			vars:  []*decls.VariableDecl{decls.NewVariable("input", allList)},
			hints: map[string]uint64{"input": 100},
			in: map[string]any{
				"input": []*proto3pb.TestAllTypes{},
			},
			wantEst:   estPtr(cost.CostEstimate{Min: 13, Max: 313}),
			wantTrack: trackPtr(13),
		},
		{
			name:    "bind: nested with list and size hints",
			expr:    `cel.bind(a, input, a.all(x, x.all(y, true)))`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", nestedList)},
			hints:   map[string]uint64{"input": 50, "input.@items": 10},
			wantEst: estPtr(cost.CostEstimate{Min: 13, Max: 1763}),
		},
		{
			name:    "bind: unused bind variable",
			expr:    `cel.bind(a, [1, 2, 3], 42)`,
			wantEst: estPtr(cost.CostEstimate{Min: 20, Max: 20}),
		},
		{
			name:      "bind: derived size propagation to comprehension",
			expr:      `cel.bind(v, [1, 2, 3], v.all(x, true))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 31, Max: 31}),
			wantTrack: trackPtr(31),
		},
		{
			name:      "two-var all: list literal",
			expr:      `[1, 2, 3].all(i, v, i < v)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 20, Max: 29}),
			wantTrack: trackPtr(29),
		},
		{
			name:    "two-var all: list variable with hints",
			expr:    `input.all(i, v, true)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", allList)},
			hints:   map[string]uint64{"input": 100},
			wantEst: estPtr(cost.CostEstimate{Min: 2, Max: 302}),
		},
		{
			name:    "two-var all: map literal",
			expr:    `{"a": 1, "b": 2}.all(k, v, k != "" && v > 0)`,
			wantEst: estPtr(cost.CostEstimate{Min: 37, Max: 43}),
		},
		{
			name:    "two-var all: map variable with hints",
			expr:    `input.all(k, v, true)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", allMap)},
			hints:   map[string]uint64{"input": 50},
			wantEst: estPtr(cost.CostEstimate{Min: 2, Max: 152}),
		},
		{
			name:      "two-var exists: list literal",
			expr:      `[1, 2, 3].exists(i, v, i == 1 && v == 2)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 23, Max: 35}),
			wantTrack: trackPtr(28),
		},
		{
			name:    "two-var exists: map literal",
			expr:    `{"a": 1, "b": 2}.exists(k, v, k == "a" && v == 1)`,
			wantEst: estPtr(cost.CostEstimate{Min: 39, Max: 47}),
		},
		{
			name:    "two-var existsOne: list literal",
			expr:    `[1, 2, 3].existsOne(i, v, v == 1)`,
			wantEst: estPtr(cost.CostEstimate{Min: 21, Max: 24}),
		},
		{
			name:    "two-var exists_one: list literal",
			expr:    `[1, 2, 3].exists_one(i, v, v == 1)`,
			wantEst: estPtr(cost.CostEstimate{Min: 21, Max: 24}),
		},
		{
			name:      "two-var transformList: 3-arg list literal",
			expr:      `[1, 2, 3].transformList(i, v, i + v)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 66, Max: 66}),
			wantTrack: trackPtr(66),
		},
		{
			name:      "two-var transformList: 4-arg with filter list literal",
			expr:      `[1, 2, 3].transformList(i, v, i % 2 == 0, i + v)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 33, Max: 75}),
			wantTrack: trackPtr(60),
		},
		{
			name:      "two-var transformList: 3-arg map literal",
			expr:      `{"a": 1, "b": 2}.transformList(k, v, k)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 67, Max: 67}),
			wantTrack: trackPtr(67),
		},
		{
			name:      "two-var transformMap: 3-arg map literal",
			expr:      `{"a": 1, "b": 2}.transformMap(k, v, v + 1)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 71, Max: 71}),
			wantTrack: trackPtr(71),
		},
		{
			name:      "two-var transformMap: 4-arg with filter map literal",
			expr:      `{"a": 1, "b": 2}.transformMap(k, v, v > 1, v + 1)`,
			wantEst:   estPtr(cost.CostEstimate{Min: 67, Max: 75}),
			wantTrack: trackPtr(70),
		},
		{
			name:      "two-var transformMapEntry: 3-arg map literal",
			expr:      `{"a": 1, "b": 2}.transformMapEntry(k, v, {v: k})`,
			wantEst:   estPtr(cost.CostEstimate{Min: 129, Max: 129}),
			wantTrack: trackPtr(129),
		},
		{
			name:      "two-var transformMapEntry: 4-arg with filter map literal",
			expr:      `{"a": 1, "b": 2}.transformMapEntry(k, v, v > 1, {v: k})`,
			wantEst:   estPtr(cost.CostEstimate{Min: 67, Max: 133}),
			wantTrack: trackPtr(99),
		},
		{
			name:      "two-var nested all",
			expr:      `[1, 2].all(i, v, [1, 2].all(j, w, i + j < v + w))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 17, Max: 79}),
			wantTrack: trackPtr(79),
		},
		{
			name:      "bind with two-var comprehension",
			expr:      `cel.bind(l, [1, 2, 3], l.all(i, v, i < v))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 31, Max: 40}),
			wantTrack: trackPtr(40),
		},
		{
			name:      "bind with two-var transformList",
			expr:      `cel.bind(m, {"a": 1, "b": 2}, m.transformList(k, v, k))`,
			wantEst:   estPtr(cost.CostEstimate{Min: 78, Max: 78}),
			wantTrack: trackPtr(78),
		},
		{
			name:      "select: array index",
			expr:      `input[0]`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.NewListType(types.StringType))},
			in:        map[string]any{"input": []string{"v"}},
			wantTrack: trackPtr(2),
		},
		{
			name:      "expr select: map",
			expr:      `input['ke' + 'y']`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			in:        map[string]any{"input": map[string]string{"key": "v"}},
			wantTrack: trackPtr(3),
		},
		{
			name:      "expr select: array index",
			expr:      `input[3-3]`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.NewListType(types.StringType))},
			in:        map[string]any{"input": []string{"v"}},
			wantTrack: trackPtr(3),
		},
		{
			name:      "optional select: map",
			expr:      `input.?key`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			in:        map[string]any{"input": map[string]string{"key": "v"}},
			wantTrack: trackPtr(2),
		},
		{
			name:      "optional index: map",
			expr:      `input[?'key']`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			in:        map[string]any{"input": map[string]string{"key": "v"}},
			wantTrack: trackPtr(2),
		},
		{
			name:      "optional select: chained",
			expr:      `input.?key.subkey`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", nestedMapStr)},
			in:        map[string]any{"input": map[string]map[string]string{"key": {"subkey": "v"}}},
			wantTrack: trackPtr(3),
		},
		{
			name:      "optional index: chained",
			expr:      `input[?'key'].subkey`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", nestedMapStr)},
			in:        map[string]any{"input": map[string]map[string]string{"key": {"subkey": "v"}}},
			wantTrack: trackPtr(3),
		},
		{
			name:      "optional index: map literal",
			expr:      `{'key': 'v'}[?'key']`,
			wantTrack: trackPtr(32),
		},
		{
			name:      "optional index: list literal",
			expr:      `['v'][?0]`,
			wantTrack: trackPtr(12),
		},
		{
			name:      "or",
			expr:      `false || false`,
			wantTrack: trackPtr(0),
		},
		{
			name:      "and short-circuit",
			expr:      `false && true`,
			wantTrack: trackPtr(0),
		},
		{
			name:      "str eq str",
			expr:      `'12345678901234567890' == '123456789012345678901234567890'`,
			wantTrack: trackPtr(2),
		},
		{
			name:      "ternary eval trivial, true",
			expr:      `true ? false : 1 > 3`,
			in:        map[string]any{},
			wantTrack: trackPtr(0),
		},
		{
			name:      "ternary eval trivial, false",
			expr:      `false ? false : 1 > 3`,
			in:        map[string]any{},
			wantTrack: trackPtr(1),
		},
		{
			name: "at limit",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			in:        map[string]any{"str1": "val1", "str2": "val2222222"},
			limit:     6,
			wantTrack: trackPtr(6),
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
			name:      "ternary as operand",
			expr:      `(1 > 2 ? 5 : 3) > 1`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantTrack: trackPtr(2),
		},
		{
			name:      "ternary as operand",
			expr:      `(1 > 2 || 2 > 1) == true`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantTrack: trackPtr(3),
		},
		{
			name:      "list map literal",
			expr:      `[{'k1': 1}, {'k2': 2}].all(x, true)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantTrack: trackPtr(77),
		},
		{
			name:      "list map literal",
			expr:      `[{'k1': 1}, {'k2': 2}].all(x, true)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantTrack: trackPtr(77),
		},
		{
			name:      ".map.map list literal",
			expr:      `[1,2,3,4,5].map(x, [x, x]).filter(z, z.size() == 2)`,
			vars:      []*decls.VariableDecl{},
			in:        map[string]any{},
			wantTrack: trackPtr(232),
		},
		{
			name:      "bind: with list and indexing",
			expr:      `cel.bind(a, [1, 2, 3], a[0])`,
			wantTrack: trackPtr(22),
		},
		{
			name:               "bind: limit exceeded",
			expr:               `cel.bind(a, [1, 2, 3], a.all(x, true))`,
			limit:              25,
			expectExceedsLimit: true,
		},
		{
			name:      "two-var all: list variable early return false",
			expr:      `input.all(i, v, i > v) == false`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", intList)},
			in:        map[string]any{"input": []int{1, 2, 3}},
			wantTrack: trackPtr(11),
		},
		{
			name:      "two-var all: list variable",
			expr:      `input.all(i, v, i < 5)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", intList)},
			in:        map[string]any{"input": []int{1, 2, 3}},
			wantTrack: trackPtr(17),
		},
		{
			name:      "two-var all: map literal early return false",
			expr:      `{'hello': 'world'}.all(k, v, k != v) == false`,
			wantTrack: trackPtr(38),
		},
		{
			name:      "two-var exists: map literal",
			expr:      `{"a": 1}.exists(k, v, k == "a" && v == 1)`,
			wantTrack: trackPtr(39),
		},
		{
			name:      "two-var existsOne: list variable",
			expr:      `input.existsOne(i, v, v == 1)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", intList)},
			in:        map[string]any{"input": []int{1, 2, 3}},
			wantTrack: trackPtr(11),
		},
		{
			name:      "two-var exists_one: list variable",
			expr:      `input.exists_one(i, v, v == 1)`,
			vars:      []*decls.VariableDecl{decls.NewVariable("input", intList)},
			in:        map[string]any{"input": []int{1, 2, 3}},
			wantTrack: trackPtr(11),
		},
		{
			name:               "two-var transformList: limit exceeded",
			expr:               `[1, 2, 3, 4, 5].transformList(i, v, i + v)`,
			limit:              50,
			expectExceedsLimit: true,
		},
	}

	for _, tst := range cases {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			if tc.hints == nil {
				tc.hints = map[string]uint64{}
			}
			env, err := testCelEnv.Extend(cel.VariableDecls(tc.vars...))
			if err != nil {
				t.Fatalf("env.Extend() failed: %v", err)
			}
			checked, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}

			costOpts := tc.estimateOptions
			if len(costOpts) == 0 && len(tc.trackerOptions) > 0 {
				tracker, err := cost.NewTracker(nil, tc.trackerOptions...)
				if err == nil {
					costOpts = []cost.Option{cost.PresenceTestHasCost(tracker.PresenceTestHasCost())}
				}
			}
			est, err := cost.Cost(checked.NativeRep(), testCostEstimator{hints: tc.hints}, costOpts...)
			if err != nil {
				t.Fatalf("Cost() failed: %v", err)
			}
			if tc.wantEst != nil {
				if est.Min != tc.wantEst.Min || est.Max != tc.wantEst.Max {
					t.Errorf("estimated cost got [%d, %d], wanted [%d, %d]", est.Min, est.Max, tc.wantEst.Min, tc.wantEst.Max)
				}
				wantLegacy := *tc.wantEst
				if tc.wantEstV0 != nil {
					wantLegacy = *tc.wantEstV0
				}
				legacyOpts := append(slices.Clone(costOpts), cost.EstimateModelVersion(0))
				legacyEst, err := cost.Cost(checked.NativeRep(), testCostEstimator{hints: tc.hints}, legacyOpts...)
				if err != nil {
					t.Fatalf("Cost(version 0) failed: %v", err)
				}
				if legacyEst.Min != wantLegacy.Min || legacyEst.Max != wantLegacy.Max {
					t.Errorf("version 0 estimated cost got [%d, %d], wanted [%d, %d]", legacyEst.Min, legacyEst.Max, wantLegacy.Min, wantLegacy.Max)
				}
			}

			if tc.wantTrack != nil || tc.in != nil || tc.expectExceedsLimit {
				ctx := constructActivation(t, tc.in)
				opts := tc.trackerOptions
				if tc.limit > 0 {
					opts = append(opts, cost.TrackerLimit(tc.limit))
				}
				prg, err := env.Program(checked,
					cel.CostTracking(&testRuntimeCostEstimator{}),
					cel.CostTrackerOptions(opts...))
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				_, det, err := prg.Eval(ctx)
				if err != nil {
					if tc.expectExceedsLimit {
						return
					}
					t.Fatalf("Program.Eval() failed: %v", err)
				}
				if tc.expectExceedsLimit {
					t.Fatalf("expected cost limit exceeded for limit %d, got cost %v", tc.limit, det.ActualCost())
				}
				var actualCost uint64
				if c := det.ActualCost(); c != nil {
					actualCost = *c
				}
				if tc.wantTrack != nil && actualCost != *tc.wantTrack {
					t.Errorf("runtime tracked cost got %d, wanted %d", actualCost, *tc.wantTrack)
				}
				if !tc.expectExceedsLimit {
					if actualCost < est.Min || actualCost > est.Max {
						t.Errorf("tracked cost %d must be in range [%d, %d] (est.Min <= actualCost <= est.Max)", actualCost, est.Min, est.Max)
					}
				}
			}
		})
	}
}

type testCustomSizingStrategy struct{}

func (testCustomSizingStrategy) EstimateSize(ctx cost.EstimateContext, node cost.AstNode) (cost.SizeEstimate, bool) {
	if node.Path() != nil && len(node.Path()) > 0 && node.Path()[0] == "custom_str" {
		return cost.RangedSizeEstimate(10, 20), true
	}
	if node.Path() != nil && len(node.Path()) > 0 && node.Path()[0] == "custom_list" {
		return cost.ListSizeEstimate(cost.RangedSizeEstimate(1, 5), cost.RangedSizeEstimate(15, 30)), true
	}
	return cost.SizeEstimate{}, false
}

func (testCustomSizingStrategy) TrackSize(ctx cost.TrackContext, value ref.Val) (uint64, bool) {
	return cost.ActualSize(value), true
}

func TestCustomSizingStrategy(t *testing.T) {
	env, err := testCelEnv.Extend(cel.VariableDecls(decls.NewVariable("custom_str", types.StringType)))
	if err != nil {
		t.Fatalf("env.Extend() failed: %v", err)
	}
	checked, iss := env.Compile("custom_str.contains('abc')")
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	res, err := cost.Cost(checked.NativeRep(), nil, cost.EstimateSizingStrategy(testCustomSizingStrategy{}))
	if err != nil {
		t.Fatalf("Cost() failed: %v", err)
	}
	// 'abc' has length 3, cost traversal factor 0.1 -> ceil(3 * 0.1) = 1
	// custom_str has min 10, max 20 -> min ceil(10 * 0.1) = 1, max ceil(20 * 0.1) = 2
	// contains cost: min 1 * 1 = 1, max 2 * 1 = 2
	// ident cost = 1
	// total = ident(1) + call(min 1, max 2) = min 2, max 3
	if res.Min != 2 || res.Max != 3 {
		t.Errorf("got cost %v, wanted {Min: 2, Max: 3}", res)
	}
}
