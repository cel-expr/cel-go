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
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

type testEvalContext struct {
	estimator     Estimator
	strategy      SizingStrategy
	args          []SizeEstimate
	argValues     []uint64
	receiver      *SizeEstimate
	receiverValue *uint64
	result        *SizeEstimate
	targetType    *types.Type
	argTypes      []*types.Type
}

func (t *testEvalContext) ArgValue(index int, defaultVal uint64) uint64 {
	if index < len(t.argValues) {
		return t.argValues[index]
	}
	return defaultVal
}

func (t *testEvalContext) TargetValue(defaultVal uint64) uint64 {
	if t.receiverValue != nil {
		return *t.receiverValue
	}
	return defaultVal
}

func (t *testEvalContext) Arg(index int) (SizeEstimate, bool) {
	if index < len(t.args) {
		return t.args[index], true
	}
	return UnknownSizeEstimate(), false
}

func (t *testEvalContext) Target() (SizeEstimate, bool) {
	if t.receiver != nil {
		return *t.receiver, true
	}
	return UnknownSizeEstimate(), false
}

func (t *testEvalContext) Result() (SizeEstimate, bool) {
	if t.result != nil {
		return *t.result, true
	}
	return UnknownSizeEstimate(), false
}

func (t *testEvalContext) Estimator() Estimator {
	return t.estimator
}

func (t *testEvalContext) Size(node AstNode) SizeEstimate {
	if node == nil {
		return UnknownSizeEstimate()
	}
	if node.ComputedSize() != nil {
		return *node.ComputedSize()
	}
	if t.strategy != nil {
		if sz, ok := t.strategy.EstimateSize(t, node); ok {
			return sz
		}
	}
	if t.estimator != nil {
		if sz := t.estimator.EstimateSize(node); sz != nil {
			return *sz
		}
	}
	if sz := computeTypeSize(node.Type(), versionOf(t)); sz != nil {
		return *sz
	}
	return UnknownSizeEstimate()
}

func (t *testEvalContext) TargetType() (*types.Type, bool) {
	if t.targetType != nil {
		return t.targetType, true
	}
	return nil, false
}

func (t *testEvalContext) ArgType(index int) (*types.Type, bool) {
	if index < len(t.argTypes) {
		return t.argTypes[index], true
	}
	return nil, false
}

// TestMinQuantityModelVersions checks that Min computes the exact interval minimum, while the
// BeforeVersion(1, ...) prior on comparison overloads reproduces the version 0 floor of 1.
func TestMinQuantityModelVersions(t *testing.T) {
	model := Overload("cmp", argumentsOnly(),
		EvalCost(Min(Arg(0), Arg(1))),
		BeforeVersion(1, EvalCost(Ranged(Const(1), Min(Arg(0), Arg(1))))),
	)
	estV0 := model.FunctionEstimatorWithOptions(ModelVersion(0))
	estV1 := model.FunctionEstimatorWithOptions(ModelVersion(1))
	tests := []struct {
		name   string
		args   []SizeEstimate
		wantV0 CostEstimate
		wantV1 CostEstimate
	}{
		{
			name:   "overlapping_intervals",
			args:   []SizeEstimate{RangedSizeEstimate(2, 10), RangedSizeEstimate(3, 5)},
			wantV0: RangedCostEstimate(1, 5),
			wantV1: RangedCostEstimate(2, 5),
		},
		{
			name:   "equal_fixed_operands",
			args:   []SizeEstimate{FixedSizeEstimate(6), FixedSizeEstimate(6)},
			wantV0: RangedCostEstimate(1, 6),
			wantV1: FixedCostEstimate(6),
		},
		{
			name:   "operand_can_be_empty",
			args:   []SizeEstimate{RangedSizeEstimate(0, 4), RangedSizeEstimate(3, 9)},
			wantV0: RangedCostEstimate(1, 4),
			wantV1: RangedCostEstimate(0, 4),
		},
		{
			name:   "zero_upper_bound",
			args:   []SizeEstimate{FixedSizeEstimate(0), RangedSizeEstimate(3, 9)},
			wantV0: FixedCostEstimate(0),
			wantV1: FixedCostEstimate(0),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a0, a1 := tc.args[0], tc.args[1]
			nodes := []AstNode{
				NewAstNode(nil, nil, types.StringType, &a0),
				NewAstNode(nil, nil, types.StringType, &a1),
			}
			gotV0 := estV0(nil, nil, nodes).CostEstimate
			if !reflect.DeepEqual(gotV0, tc.wantV0) {
				t.Errorf("estimate at ModelVersion(0) got %v, wanted %v", gotV0, tc.wantV0)
			}
			gotV1 := estV1(nil, nil, nodes).CostEstimate
			if !reflect.DeepEqual(gotV1, tc.wantV1) {
				t.Errorf("estimate at ModelVersion(1) got %v, wanted %v", gotV1, tc.wantV1)
			}
		})
	}
}

func TestQuantityExprs_Estimate(t *testing.T) {
	elem0 := FixedSizeEstimate(7)
	key1 := FixedSizeEstimate(3)
	elem1 := FixedSizeEstimate(12)
	rcvElem := FixedSizeEstimate(15)
	rcvKey := FixedSizeEstimate(4)

	arg0 := ListSizeEstimate(RangedSizeEstimate(2, 10), elem0)
	arg1 := MapSizeEstimate(RangedSizeEstimate(3, 5), key1, elem1)
	rcv := MapSizeEstimate(RangedSizeEstimate(4, 8), rcvKey, rcvElem)

	receiverVal := uint64(20)
	ctx := &testEvalContext{
		args: []SizeEstimate{
			arg0,
			arg1,
		},
		argValues:     []uint64{3, 10},
		receiver:      &rcv,
		receiverValue: &receiverVal,
		targetType:    types.NewListType(types.StringType),
		argTypes:      []*types.Type{types.NewListType(types.IntType)},
	}

	tests := []struct {
		name     string
		expr     QuantityExpr
		expected SizeEstimate
	}{
		{
			name:     "const_quantity",
			expr:     Const(42),
			expected: FixedSizeEstimate(42),
		},
		{
			name:     "arg_quantity",
			expr:     Arg(0),
			expected: arg0,
		},
		{
			name:     "int_arg_quantity",
			expr:     IntArg(0, 99),
			expected: FixedSizeEstimate(3),
		},
		{
			// A missing argument is not a literal, so it is unbounded (version 1); the default
			// applies only to the track.
			name:     "int_arg_default_quantity",
			expr:     IntArg(5, 99),
			expected: UnknownSizeEstimate(),
		},
		{
			name:     "arg_element_quantity",
			expr:     ArgElem(0),
			expected: elem0,
		},
		{
			name:     "arg_key_quantity",
			expr:     ArgKey(1),
			expected: key1,
		},
		{
			name:     "target_quantity",
			expr:     Target(),
			expected: rcv,
		},
		{
			name:     "int_target_quantity",
			expr:     IntTarget(99),
			expected: FixedSizeEstimate(20),
		},
		{
			name:     "target_element_quantity",
			expr:     TargetElem(),
			expected: rcvElem,
		},
		{
			name:     "target_key_quantity",
			expr:     TargetKey(),
			expected: rcvKey,
		},
		{
			name:     "sum_quantity",
			expr:     Sum(Arg(0), Arg(1)),
			expected: arg0.Add(arg1),
		},
		{
			// The difference carries only the minuend's metadata (version 1).
			name: "sub_quantity",
			expr: Sub(Arg(0), Arg(1)),
			expected: func() SizeEstimate {
				diff := arg0.Subtract(arg1)
				diff.Key, diff.Elem = arg0.Key, arg0.Elem
				return diff
			}(),
		},
		{
			name: "mul_quantity",
			expr: Mul(Arg(0), Arg(1)),
			// The product carries the union of the operands' key and element sizes.
			expected: MapSizeEstimate(RangedSizeEstimate(6, 50), key1, RangedSizeEstimate(7, 12)),
		},
		{
			name:     "scale_quantity",
			expr:     Scale(Arg(0), 0.5),
			expected: ListSizeEstimate(RangedSizeEstimate(1, 5), elem0),
		},
		{
			name:     "string_scan_quantity",
			expr:     StringScan(Const(20)),
			expected: FixedSizeEstimate(2),
		},
		{
			name:     "list_alloc_quantity",
			expr:     ListAlloc(Const(10), 0.5),
			expected: FixedSizeEstimate(ListCreateBaseCost + 5),
		},
		{
			name:     "alloc_list_quantity",
			expr:     AllocList(Const(10), Const(5)),
			expected: FixedSizeEstimate(CallCost + ListCreateBaseCost + 15),
		},
		{
			name:     "traversal_with_alloc_quantity",
			expr:     Traversal(Const(20), 0.5, 10),
			expected: FixedSizeEstimate(20),
		},
		{
			name:     "traversal_without_alloc_quantity",
			expr:     Traversal(Const(20), 0.5, 0),
			expected: FixedSizeEstimate(10),
		},
		{
			name:     "at_least_one_quantity",
			expr:     AtLeastOneQuantity(Const(0)),
			expected: FixedSizeEstimate(1),
		},
		{
			name:     "square_quantity",
			expr:     Square(Arg(1)),
			expected: MapSizeEstimate(RangedSizeEstimate(9, 25), key1, elem1),
		},
		{
			name:     "square_and_scale_quantity",
			expr:     Scale(Square(Arg(1)), 2.0),
			expected: MapSizeEstimate(RangedSizeEstimate(18, 50), key1, elem1),
		},
		{
			name: "min_quantity",
			expr: Min(Arg(0), Arg(1)),
			// The result is one of the two operands, so their metadata is unioned rather than
			// dropped: whichever is selected, its elements are covered.
			expected: MapSizeEstimate(RangedSizeEstimate(2, 5), key1, RangedSizeEstimate(7, 12)),
		},
		{
			name:     "max_quantity",
			expr:     Max(Arg(0), Arg(1)),
			expected: MapSizeEstimate(RangedSizeEstimate(3, 10), key1, RangedSizeEstimate(7, 12)),
		},
		{
			name:     "union_quantity",
			expr:     Union(Arg(0), Arg(1)),
			expected: arg0.Union(arg1),
		},
		{
			name:     "ranged_string_to_bytes",
			expr:     Ranged(Arg(0), Scale(Arg(0), 4.0)),
			expected: ListSizeEstimate(RangedSizeEstimate(2, 40), elem0),
		},
		{
			name:     "ranged_bytes_to_string",
			expr:     Ranged(Scale(Arg(0), 0.25), Arg(0)),
			expected: ListSizeEstimate(RangedSizeEstimate(1, 10), elem0),
		},
		{
			name:     "ranged_expression_composition",
			expr:     Ranged(Sum(Arg(0), Const(2)), Sum(Scale(Arg(0), 2.0), Const(2))),
			expected: ListSizeEstimate(RangedSizeEstimate(4, 22), elem0),
		},
		{
			name:     "at_most_quantity",
			expr:     AtMost(Arg(0)),
			expected: ListSizeEstimate(RangedSizeEstimate(0, 10), elem0),
		},
		{
			name:     "list_quantity",
			expr:     List(AtMost(Arg(0)), Arg(0)),
			expected: ListSizeEstimate(RangedSizeEstimate(0, 10), arg0),
		},
		{
			name:     "list_from_map_keys_clears_key_meta",
			expr:     List(Arg(1), KeyOf(Arg(1))),
			expected: ListSizeEstimate(RangedSizeEstimate(3, 5), key1),
		},
		{
			name:     "element_of_quantity",
			expr:     ElemOf(Arg(0)),
			expected: elem0,
		},
		{
			name:     "key_of_quantity",
			expr:     KeyOf(Arg(1)),
			expected: key1,
		},
		{
			name:     "map_quantity",
			expr:     Map(Arg(1), KeyOf(Arg(1)), ElemOf(Arg(1))),
			expected: arg1,
		},
		{
			name:     "element_total_of_list",
			expr:     ElemTotal(Arg(0)),
			expected: RangedSizeEstimate(2*7, 10*7),
		},
		{
			name:     "element_total_of_map",
			expr:     ElemTotal(Arg(1)),
			expected: RangedSizeEstimate(3*12, 5*12),
		},
		{
			// A constant holds no elements, so its total is exactly 0, as it tracks.
			name:     "element_total_of_scalar",
			expr:     ElemTotal(Const(4)),
			expected: FixedSizeEstimate(0),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.expr.estimate(ctx)
			if got.Min != tc.expected.Min || got.Max != tc.expected.Max {
				t.Errorf("estimate got [%v, %v], wanted [%v, %v]", got.Min, got.Max, tc.expected.Min, tc.expected.Max)
			}
			if (got.Elem == nil) != (tc.expected.Elem == nil) {
				t.Errorf("estimate Elem nil mismatch: got %v, wanted %v", got.Elem, tc.expected.Elem)
			} else if got.Elem != nil && (got.Elem.Min != tc.expected.Elem.Min || got.Elem.Max != tc.expected.Elem.Max) {
				t.Errorf("estimate Elem got %v, wanted %v", got.Elem, tc.expected.Elem)
			}
			if (got.Key == nil) != (tc.expected.Key == nil) {
				t.Errorf("estimate Key nil mismatch: got %v, wanted %v", got.Key, tc.expected.Key)
			} else if got.Key != nil && (got.Key.Min != tc.expected.Key.Min || got.Key.Max != tc.expected.Key.Max) {
				t.Errorf("estimate Key got %v, wanted %v", got.Key, tc.expected.Key)
			}
		})
	}
}

func TestQuantityExprs_Track(t *testing.T) {
	rcvTrackVal := uint64(20)
	trackCtx := &testTrackContext{
		args:          []uint64{10, 5},
		argValues:     []uint64{3, 10},
		receiver:      8,
		receiverValue: &rcvTrackVal,
	}

	scalarTests := []struct {
		name     string
		expr     QuantityExpr
		expected uint64
	}{
		{name: "const_scalar", expr: Const(42), expected: 42},
		{name: "arg_scalar", expr: Arg(0), expected: 10},
		{name: "int_arg_scalar", expr: IntArg(0, 99), expected: 3},
		{name: "int_arg_default_scalar", expr: IntArg(5, 99), expected: 99},
		{name: "target_scalar", expr: Target(), expected: 8},
		{name: "int_target_scalar", expr: IntTarget(99), expected: 20},
		{name: "sum_scalar", expr: Sum(Arg(0), Arg(1)), expected: 15},
		{name: "sub_scalar", expr: Sub(Arg(0), Arg(1)), expected: 5},
		{name: "sub_scalar_saturates", expr: Sub(Arg(1), Arg(0)), expected: 0},
		{name: "mul_scalar", expr: Mul(Arg(0), Arg(1)), expected: 50},
		{name: "scale_scalar", expr: Scale(Arg(0), 1.5), expected: 15},
		{name: "string_scan_scalar", expr: StringScan(Const(20)), expected: 2},
		{name: "list_alloc_scalar", expr: ListAlloc(Const(10), 0.5), expected: ListCreateBaseCost + 5},
		{name: "traversal_with_alloc_scalar", expr: Traversal(Const(20), 0.5, 10), expected: 20},
		{name: "traversal_without_alloc_scalar", expr: Traversal(Const(20), 0.5, 0), expected: 10},
		{name: "at_least_one_scalar_zero", expr: AtLeastOneQuantity(Const(0)), expected: 1},
		{name: "at_least_one_scalar_nonzero", expr: AtLeastOneQuantity(Const(5)), expected: 5},
		{name: "square_scalar", expr: Square(Arg(1)), expected: 25},
		{name: "min_scalar", expr: Min(Arg(0), Arg(1)), expected: 5},
		{name: "max_scalar", expr: Max(Arg(0), Arg(1)), expected: 10},
		{name: "ranged_scalar", expr: Ranged(Arg(1), Arg(0)), expected: 10},
	}

	for _, tc := range scalarTests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.expr.track(trackCtx)
			if got != tc.expected {
				t.Errorf("track got %v, wanted %v", got, tc.expected)
			}
		})
	}
}

// TestProjectionExprs_Track pins the runtime behaviour of the element and key projections, which
// read the values themselves rather than reporting a placeholder.
func TestProjectionExprs_Track(t *testing.T) {
	adapter := types.DefaultTypeAdapter
	// Sizes: 'a'=1, 'bbbb'=4, 'cc'=2, so the list holds 3 elements totalling 7.
	strList := adapter.NativeToValue([]string{"a", "bbbb", "cc"})
	nestedList := adapter.NativeToValue([][]string{{"a", "bbbb"}, {"cc"}})
	strMap := adapter.NativeToValue(map[string]string{"k": "vvv", "longer": "v"})

	trackCtx := &testTrackContext{
		argVals:   []ref.Val{strList, strMap, nestedList},
		targetVal: strMap,
		resultVal: strList,
	}

	tests := []struct {
		name     string
		expr     QuantityExpr
		expected uint64
	}{
		{name: "elem_of_list_is_largest", expr: ArgElem(0), expected: 4},
		{name: "elem_total_of_list", expr: ElemTotal(Arg(0)), expected: 7},
		{name: "elem_of_map_values", expr: ArgElem(1), expected: 3},
		{name: "elem_total_of_map_values", expr: ElemTotal(Arg(1)), expected: 4},
		{name: "key_of_map_is_largest", expr: ArgKey(1), expected: 6},
		{name: "elem_of_target", expr: TargetElem(), expected: 3},
		{name: "key_of_target", expr: TargetKey(), expected: 6},
		{name: "elem_of_result", expr: ElemOf(Result()), expected: 4},
		// The elements of the inner lists, not of the outer one.
		{name: "elem_of_nested_list", expr: ElemOf(ArgElem(2)), expected: 4},
		// Rejected by Validate, since no single size estimate bounds it; this pins the charge only.
		// A list has no keys, and a string has no elements.
		{name: "key_of_list", expr: ArgKey(0), expected: 0},
		{name: "elem_of_absent_arg", expr: ArgElem(9), expected: 0},
		{name: "elem_total_of_absent_arg", expr: ElemTotal(Arg(9)), expected: 0},
		// Rejected by Validate, so this only pins the charge if a model bypasses registration.
		{name: "elem_of_computed_quantity", expr: ElemOf(Const(4)), expected: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.expr.track(trackCtx); got != tc.expected {
				t.Errorf("track() got %v, wanted %v", got, tc.expected)
			}
		})
	}
}

// TestProjectionExprs_TrackWithoutValues covers a context which reports no values at all, the
// state every TrackContext was in before the value accessors existed.
func TestProjectionExprs_TrackWithoutValues(t *testing.T) {
	trackCtx := &testTrackContext{args: []uint64{10}, receiver: 8}
	for _, expr := range []QuantityExpr{ArgElem(0), ArgKey(0), TargetElem(), TargetKey(), ElemOf(Result())} {
		label, _ := describeQuantityExpr(expr)
		if got := expr.track(trackCtx); got != 0 {
			t.Errorf("%s.track() got %v, wanted 0", label, got)
		}
	}
}

// BenchmarkProjectionTracking measures the scan the element and key projections perform. Unlike
// the other quantity expressions, whose cost is independent of the values, these are linear in the
// size of the container and run once per call that is charged through them.
func BenchmarkProjectionTracking(b *testing.B) {
	adapter := types.DefaultTypeAdapter
	for _, size := range []int{1, 10, 100, 1000} {
		elems := make([]string, size)
		entries := make(map[string]string, size)
		for i := range size {
			elems[i] = strings.Repeat("x", i%16)
			entries[elems[i]+fmt.Sprint(i)] = elems[i]
		}
		listVal := adapter.NativeToValue(elems)
		mapVal := adapter.NativeToValue(entries)

		benchmarks := []struct {
			name string
			expr QuantityExpr
			ctx  TrackContext
		}{
			{name: "elem_of_list", expr: ArgElem(0), ctx: &testTrackContext{argVals: []ref.Val{listVal}}},
			{name: "elem_total_list", expr: ElemTotal(Arg(0)), ctx: &testTrackContext{argVals: []ref.Val{listVal}}},
			{name: "key_of_map", expr: ArgKey(0), ctx: &testTrackContext{argVals: []ref.Val{mapVal}}},
			// The baseline: a size read, which does not scan.
			{name: "arg_size", expr: Arg(0), ctx: &testTrackContext{args: []uint64{uint64(size)}}},
		}
		for _, bm := range benchmarks {
			b.Run(fmt.Sprintf("%s/%d", bm.name, size), func(b *testing.B) {
				for b.Loop() {
					bm.expr.track(bm.ctx)
				}
			})
		}
	}
}

type testTrackContext struct {
	args          []uint64
	argValues     []uint64
	receiver      uint64
	receiverValue *uint64
	result        uint64
	estimator     ActualCostEstimator

	// Values denoted by the context, for the element and key projections. A nil entry stands for
	// an argument, target or result which is absent.
	argVals   []ref.Val
	targetVal ref.Val
	resultVal ref.Val
}

func (t *testTrackContext) ArgValue(index int, defaultVal uint64) uint64 {
	if index < len(t.argValues) {
		return t.argValues[index]
	}
	return defaultVal
}

func (t *testTrackContext) TargetValue(defaultVal uint64) uint64 {
	if t.receiverValue != nil {
		return *t.receiverValue
	}
	return defaultVal
}

func (t *testTrackContext) Arg(index int) uint64 {
	if index < len(t.args) {
		return t.args[index]
	}
	return 0
}

func (t *testTrackContext) Target() uint64 {
	return t.receiver
}

func (t *testTrackContext) Result() uint64 {
	return t.result
}

func (t *testTrackContext) ArgVal(index int) (ref.Val, bool) {
	if index < len(t.argVals) && t.argVals[index] != nil {
		return t.argVals[index], true
	}
	return nil, false
}

func (t *testTrackContext) TargetVal() (ref.Val, bool) {
	if t.targetVal != nil {
		return t.targetVal, true
	}
	return nil, false
}

func (t *testTrackContext) ResultVal() (ref.Val, bool) {
	if t.resultVal != nil {
		return t.resultVal, true
	}
	return nil, false
}

func (t *testTrackContext) Estimator() ActualCostEstimator {
	return t.estimator
}

func (t *testTrackContext) Size(value ref.Val) uint64 {
	return ActualSize(value)
}

func (t *testTrackContext) TargetType() (*types.Type, bool) {
	return nil, false
}

func (t *testTrackContext) ArgType(index int) (*types.Type, bool) {
	return nil, false
}

type customExprWithoutTarget struct{}

func (customExprWithoutTarget) estimate(EstimateContext) SizeEstimate { return FixedSizeEstimate(0) }
func (customExprWithoutTarget) track(TrackContext) uint64             { return 0 }

func TestModel_HasTargetInspection(t *testing.T) {
	tests := []struct {
		name       string
		expr       QuantityExpr
		wantTarget bool
	}{
		{name: "nil_expr", expr: nil, wantTarget: false},
		{name: "custom_expr_without_target", expr: customExprWithoutTarget{}, wantTarget: false},
		{name: "const_expr", expr: Const(10), wantTarget: false},
		{name: "arg_expr", expr: Arg(0), wantTarget: false},
		{name: "arg_elem_expr", expr: ArgElem(0), wantTarget: false},
		{name: "arg_key_expr", expr: ArgKey(0), wantTarget: false},
		{name: "int_arg_expr", expr: IntArg(0, 10), wantTarget: false},
		{name: "target_expr", expr: Target(), wantTarget: true},
		{name: "int_target_expr", expr: IntTarget(10), wantTarget: true},
		{name: "target_elem_expr", expr: TargetElem(), wantTarget: true},
		{name: "target_key_expr", expr: TargetKey(), wantTarget: true},
		{name: "result_expr", expr: Result(), wantTarget: false},
		{name: "elem_of_target", expr: ElemOf(Target()), wantTarget: true},
		{name: "elem_of_arg", expr: ElemOf(Arg(0)), wantTarget: false},
		{name: "key_of_target", expr: KeyOf(Target()), wantTarget: true},
		{name: "key_of_arg", expr: KeyOf(Arg(0)), wantTarget: false},
		{name: "sub_with_target_lhs", expr: Sub(Target(), Arg(0)), wantTarget: true},
		{name: "sub_with_target_rhs", expr: Sub(Arg(0), Target()), wantTarget: true},
		{name: "sub_without_target", expr: Sub(Arg(0), Arg(1)), wantTarget: false},
		{name: "at_least_one_with_target", expr: AtLeastOneQuantity(Target()), wantTarget: true},
		{name: "at_least_one_without_target", expr: AtLeastOneQuantity(Arg(0)), wantTarget: false},
		{name: "string_scan_with_target", expr: StringScan(Target()), wantTarget: true},
		{name: "string_scan_without_target", expr: StringScan(Arg(0)), wantTarget: false},
		{name: "list_alloc_with_target", expr: ListAlloc(Target(), 1.0), wantTarget: true},
		{name: "list_alloc_without_target", expr: ListAlloc(Arg(0), 1.0), wantTarget: false},
		{name: "traversal_with_target", expr: Traversal(Target(), 1.0, 10), wantTarget: true},
		{name: "traversal_without_target", expr: Traversal(Arg(0), 1.0, 10), wantTarget: false},
		{name: "min_with_target", expr: Min(Arg(0), Target()), wantTarget: true},
		{name: "min_without_target", expr: Min(Arg(0), Arg(1)), wantTarget: false},
		{name: "max_with_target", expr: Max(Target(), Arg(0)), wantTarget: true},
		{name: "max_without_target", expr: Max(Arg(0), Arg(1)), wantTarget: false},
		{name: "union_with_target", expr: Union(Arg(0), Target()), wantTarget: true},
		{name: "union_without_target", expr: Union(Arg(0), Arg(1)), wantTarget: false},
		{name: "ranged_with_target_lhs", expr: Ranged(Target(), Arg(0)), wantTarget: true},
		{name: "ranged_with_target_rhs", expr: Ranged(Arg(0), Target()), wantTarget: true},
		{name: "ranged_without_target", expr: Ranged(Arg(0), Arg(1)), wantTarget: false},
		{name: "list_with_target_len", expr: List(Target(), Arg(0)), wantTarget: true},
		{name: "list_with_target_elem", expr: List(Arg(0), Target()), wantTarget: true},
		{name: "list_without_target", expr: List(Arg(0), Arg(1)), wantTarget: false},
		{name: "map_with_target_size", expr: Map(Target(), Arg(0), Arg(1)), wantTarget: true},
		{name: "map_with_target_key", expr: Map(Arg(0), Target(), Arg(1)), wantTarget: true},
		{name: "map_with_target_val", expr: Map(Arg(0), Arg(1), Target()), wantTarget: true},
		{name: "map_without_target", expr: Map(Arg(0), Arg(1), Arg(2)), wantTarget: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasTarget(tc.expr); got != tc.wantTarget {
				t.Errorf("hasTarget(%v) = %t, want %t", tc.expr, got, tc.wantTarget)
			}
		})
	}
}

func TestModel_EmptyAndDisjointExpressions(t *testing.T) {
	emptyCtx := &testEvalContext{}
	emptyTrackCtx := &testTrackContext{}

	tests := []struct {
		name         string
		expr         QuantityExpr
		wantEstimate SizeEstimate
		wantTrack    uint64
	}{
		{
			name:         "empty_sum",
			expr:         Sum(),
			wantEstimate: FixedSizeEstimate(0),
			wantTrack:    0,
		},
		{
			name:         "empty_mul",
			expr:         Mul(),
			wantEstimate: FixedSizeEstimate(1),
			wantTrack:    1,
		},
		{
			name:         "empty_union",
			expr:         Union(),
			wantEstimate: FixedSizeEstimate(0),
			wantTrack:    0,
		},
		{
			name:         "union_evaluation",
			expr:         Union(Const(5), Const(10)),
			wantEstimate: RangedSizeEstimate(5, 10),
			wantTrack:    10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotEst := tc.expr.estimate(emptyCtx)
			if gotEst != tc.wantEstimate {
				t.Errorf("estimate = %v, want %v", gotEst, tc.wantEstimate)
			}
			gotTrack := tc.expr.track(emptyTrackCtx)
			if gotTrack != tc.wantTrack {
				t.Errorf("track = %d, want %d", gotTrack, tc.wantTrack)
			}
		})
	}
}

func TestModel_MissingContextFallbacks(t *testing.T) {
	emptyCtx := &testEvalContext{}
	emptyTrackCtx := &testTrackContext{}

	tests := []struct {
		name      string
		expr      QuantityExpr
		wantTrack uint64
		checkEst  func(t *testing.T, sz SizeEstimate)
	}{
		{
			name:      "missing_arg",
			expr:      Arg(5),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_arg_elem",
			expr:      ArgElem(0),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name: "missing_arg_key",
			expr: ArgKey(0),
			// An absent value has no keys to measure. Before the projections were value-backed
			// this reported a single byte per key regardless of the value.
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_target",
			expr:      Target(),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_target_elem",
			expr:      TargetElem(),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_target_key",
			expr:      TargetKey(),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "elem_of_scalar_const",
			expr:      ElemOf(Const(5)),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name: "key_of_scalar_const",
			expr: KeyOf(Const(5)),
			// A projection over a computed quantity is rejected by OverloadModel.Validate; this
			// pins what a model which bypasses registration is charged.
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_int_arg",
			expr:      IntArg(0, 42),
			wantTrack: 42,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				// A missing value is not a literal, so it is unbounded (version 1).
				if !sz.IsUnknown() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_int_target",
			expr:      IntTarget(42),
			wantTrack: 42,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				// A missing value is not a literal, so it is unbounded (version 1).
				if !sz.IsUnknown() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "missing_result",
			expr:      Result(),
			wantTrack: 0,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != UnknownSizeEstimate() {
					t.Errorf("got %v, want unknown", sz)
				}
			},
		},
		{
			name:      "list_tracking",
			expr:      List(Const(5), Const(10)),
			wantTrack: 5,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 5 || sz.Max != 5 || sz.Elem == nil || sz.Elem.Min != 10 || sz.Elem.Max != 10 {
					t.Errorf("got %v, want List(5, 10)", sz)
				}
			},
		},
		{
			name:      "map_tracking",
			expr:      Map(Const(5), Const(2), Const(3)),
			wantTrack: 5,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 5 || sz.Max != 5 || sz.Key == nil || sz.Key.Min != 2 || sz.Elem == nil || sz.Elem.Min != 3 {
					t.Errorf("got %v, want Map(5, 2, 3)", sz)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotEst := tc.expr.estimate(emptyCtx)
			if tc.checkEst != nil {
				tc.checkEst(t, gotEst)
			}
			gotTrack := tc.expr.track(emptyTrackCtx)
			if gotTrack != tc.wantTrack {
				t.Errorf("track = %d, want %d", gotTrack, tc.wantTrack)
			}
		})
	}

	// Test populated Result()
	resSz := FixedSizeEstimate(42)
	resCtx := &testEvalContext{result: &resSz}
	if got := Result().estimate(resCtx); got != resSz {
		t.Errorf("Result().estimate = %v, want %v", got, resSz)
	}
	resTrackCtx := &testTrackContext{result: 42}
	if got := Result().track(resTrackCtx); got != 42 {
		t.Errorf("Result().track = %v, want 42", got)
	}
}

func TestModel_FunctionEstimatorAndTracker(t *testing.T) {
	mGlobal := Overload("test_fn",
		EvalCost(Const(5)),
		ResultSize(Const(10)),
	)
	estFn := mGlobal.FunctionEstimator()
	callEst := estFn(nil, nil, nil)
	if callEst == nil || callEst.CostEstimate.Min != 5 || callEst.ResultSize == nil || callEst.ResultSize.Min != 10 {
		t.Errorf("FunctionEstimator() = %v, want cost 5 size 10", callEst)
	}
	trackFn := mGlobal.FunctionTracker()
	costVal := trackFn(nil, nil)
	if costVal == nil || *costVal != 5 {
		t.Errorf("FunctionTracker() = %v, want 5", costVal)
	}

	mMember := MemberOverload("test_member", EvalCost(Target()))
	estMemberFn := mMember.FunctionEstimator()
	if estMemberFn(nil, nil, nil) != nil {
		t.Errorf("estMemberFn with nil target should return nil")
	}

	mManualEst := Overload("test_manual_est",
		EvalCost(Const(5)),
		ResultSize(Const(10)),
		BeforeVersion(1, ManualEstimator(func(Estimator, *AstNode, []AstNode) *CallEstimate {
			return NewListCallEstimate(FixedCostEstimate(2), FixedSizeEstimate(7))
		})),
	)
	if got := mManualEst.FunctionEstimatorWithOptions(ModelVersion(0))(nil, nil, nil); got == nil || got.CostEstimate != FixedCostEstimate(13) || got.ResultSize == nil || *got.ResultSize != FixedSizeEstimate(7) {
		t.Errorf("ManualEstimator at ModelVersion(0) = %v, want cost 13 size 7", got)
	}
	if got := mManualEst.FunctionTrackerWithOptions(ModelVersion(0))(nil, nil); got == nil || *got != 5 {
		t.Errorf("FunctionTrackerWithOptions(ModelVersion(0)) with ManualEstimator prior = %v, want 5", got)
	}

	mPriorSizeOnly := Overload("test_prior_size_only",
		EvalCost(Const(5)),
		ResultSize(Const(10)),
		BeforeVersion(1, ResultSize(Const(3))),
	)
	if got := mPriorSizeOnly.FunctionEstimatorWithOptions(ModelVersion(0))(nil, nil, nil); got == nil || got.CostEstimate != FixedCostEstimate(5) || got.ResultSize == nil || *got.ResultSize != FixedSizeEstimate(3) {
		t.Errorf("ResultSize-only prior at ModelVersion(0) = %v, want cost 5 size 3", got)
	}
	if got := mPriorSizeOnly.FunctionTrackerWithOptions(ModelVersion(0))(nil, nil); got == nil || *got != 5 {
		t.Errorf("FunctionTrackerWithOptions(ModelVersion(0)) with ResultSize-only prior = %v, want 5", got)
	}
}

func TestModel_TrackerEvalContextMethods(t *testing.T) {
	tCtx := &trackerEvalContext{
		strategy: defaultSizing,
		isMember: true,
		args: []ref.Val{
			types.String("target_val"),
			types.Int(100),
		},
		result: types.String("result_val"),
	}

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "estimator_is_nil",
			check: func(t *testing.T) {
				if tCtx.Estimator() != nil {
					t.Errorf("Estimator() should be nil")
				}
			},
		},
		{
			name: "target_size",
			check: func(t *testing.T) {
				if sz := tCtx.Target(); sz != 10 {
					t.Errorf("Target() = %d, want 10", sz)
				}
			},
		},
		{
			name: "arg_size",
			check: func(t *testing.T) {
				if sz := tCtx.Arg(0); sz != 1 {
					t.Errorf("Arg(0) = %d, want 1", sz)
				}
			},
		},
		{
			name: "arg_size_out_of_bounds",
			check: func(t *testing.T) {
				if sz := tCtx.Arg(5); sz != 0 {
					t.Errorf("Arg(5) = %d, want 0", sz)
				}
			},
		},
		{
			name: "result_size",
			check: func(t *testing.T) {
				if sz := tCtx.Result(); sz != 10 {
					t.Errorf("Result() = %d, want 10", sz)
				}
			},
		},
		{
			name: "target_type",
			check: func(t *testing.T) {
				if tp, ok := tCtx.TargetType(); !ok || tp == nil {
					t.Errorf("TargetType() = (%v, %t), want string", tp, ok)
				}
			},
		},
		{
			name: "arg_type",
			check: func(t *testing.T) {
				if tp, ok := tCtx.ArgType(0); !ok || tp == nil {
					t.Errorf("ArgType(0) = (%v, %t), want int", tp, ok)
				}
			},
		},
		{
			name: "arg_type_out_of_bounds",
			check: func(t *testing.T) {
				if _, ok := tCtx.ArgType(5); ok {
					t.Errorf("ArgType(5) should return false")
				}
			},
		},
		{
			name: "target_value",
			check: func(t *testing.T) {
				if val := tCtx.TargetValue(99); val != 99 {
					t.Errorf("TargetValue(99) for string target = %d, want 99", val)
				}
			},
		},
		{
			name: "arg_value",
			check: func(t *testing.T) {
				if val := tCtx.ArgValue(0, 99); val != 100 {
					t.Errorf("ArgValue(0, 99) = %d, want 100", val)
				}
				if val := tCtx.ArgValue(5, 99); val != 99 {
					t.Errorf("ArgValue(5, 99) = %d, want 99", val)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}

	nonMemberTCtx := &trackerEvalContext{
		strategy: defaultSizing,
		isMember: false,
		args:     []ref.Val{types.Int(50)},
	}
	if sz := nonMemberTCtx.Target(); sz != 0 {
		t.Errorf("nonMemberTCtx.Target() = %d, want 0", sz)
	}
	if sz := nonMemberTCtx.Result(); sz != 0 {
		t.Errorf("nonMemberTCtx.Result() = %d, want 0", sz)
	}
	if _, ok := nonMemberTCtx.TargetType(); ok {
		t.Errorf("nonMemberTCtx.TargetType() should return false")
	}
	if tp, ok := nonMemberTCtx.ArgType(0); !ok || tp == nil {
		t.Errorf("nonMemberTCtx.ArgType(0) = (%v, %t), want int type", tp, ok)
	}
	if sz := nonMemberTCtx.Size(nil); sz != 0 {
		t.Errorf("nonMemberTCtx.Size(nil) = %d, want 0", sz)
	}

	failingStratTCtx := &trackerEvalContext{
		strategy: failingSizingStrategy{},
	}
	if sz := failingStratTCtx.Size(types.String("fallback")); sz != 8 {
		t.Errorf("failingStratTCtx.Size = %d, want 8", sz)
	}

	nilArgTCtx := &trackerEvalContext{
		strategy: defaultSizing,
		isMember: true,
		args:     []ref.Val{nil, nil},
	}
	if _, ok := nilArgTCtx.TargetType(); ok {
		t.Errorf("nilArgTCtx.TargetType() with nil target should return false")
	}
	if _, ok := nilArgTCtx.ArgType(0); ok {
		t.Errorf("nilArgTCtx.ArgType(0) with nil arg should return false")
	}
}

func TestModel_EstimatorEvalContextMethods(t *testing.T) {
	targetNode := NewAstNode(nil, []string{"tgt"}, types.StringType, nil)
	argNode := NewAstNode(nil, []string{"arg0"}, types.IntType, nil)
	eCtx := &estimatorEvalContext{
		strategy:  failingSizingStrategy{},
		estimator: testHintsEstimator{hints: map[string]uint64{"tgt": 1, "arg0": 12}},
		target:    &targetNode,
		args:      []AstNode{argNode},
		hasTarget: true,
	}

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "estimator_not_nil",
			check: func(t *testing.T) {
				if eCtx.Estimator() == nil {
					t.Errorf("Estimator() should not be nil")
				}
			},
		},
		{
			name: "result_is_unknown",
			check: func(t *testing.T) {
				if _, ok := eCtx.Result(); ok {
					t.Errorf("Result() should return false")
				}
			},
		},
		{
			name: "target_size",
			check: func(t *testing.T) {
				if sz, ok := eCtx.Target(); !ok || sz.Max != 1 {
					t.Errorf("Target() = (%v, %t), want Max 1", sz, ok)
				}
			},
		},
		{
			name: "arg_size",
			check: func(t *testing.T) {
				if sz, ok := eCtx.Arg(0); !ok || sz.Max != 12 {
					t.Errorf("Arg(0) = (%v, %t), want Max 12", sz, ok)
				}
			},
		},
		{
			name: "arg_size_out_of_bounds",
			check: func(t *testing.T) {
				if _, ok := eCtx.Arg(5); ok {
					t.Errorf("Arg(5) should return false")
				}
			},
		},
		{
			name: "target_type",
			check: func(t *testing.T) {
				if tp, ok := eCtx.TargetType(); !ok || tp != types.StringType {
					t.Errorf("TargetType() = (%v, %t), want string", tp, ok)
				}
			},
		},
		{
			name: "arg_type",
			check: func(t *testing.T) {
				if tp, ok := eCtx.ArgType(0); !ok || tp != types.IntType {
					t.Errorf("ArgType(0) = (%v, %t), want int", tp, ok)
				}
			},
		},
		{
			name: "arg_type_out_of_bounds",
			check: func(t *testing.T) {
				if _, ok := eCtx.ArgType(5); ok {
					t.Errorf("ArgType(5) should return false")
				}
			},
		},
		{
			name: "target_value",
			check: func(t *testing.T) {
				if val := eCtx.TargetValue(99); val != 99 {
					t.Errorf("TargetValue(99) = %d, want 99", val)
				}
			},
		},
		{
			name: "arg_value",
			check: func(t *testing.T) {
				if val := eCtx.ArgValue(0, 99); val != 99 {
					t.Errorf("ArgValue(0, 99) = %d, want 99", val)
				}
				if val := eCtx.ArgValue(5, 99); val != 99 {
					t.Errorf("ArgValue(5, 99) = %d, want 99", val)
				}
			},
		},
		{
			name: "size_nil_node",
			check: func(t *testing.T) {
				if sz := eCtx.Size(nil); sz != UnknownSizeEstimate() {
					t.Errorf("Size(nil) = %v, want unknown", sz)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}

	noTargetECtx := &estimatorEvalContext{}
	if _, ok := noTargetECtx.TargetType(); ok {
		t.Errorf("noTargetECtx.TargetType() should return false")
	}
	if _, ok := noTargetECtx.Target(); ok {
		t.Errorf("noTargetECtx.Target() should return false")
	}

	computedSz := FixedSizeEstimate(88)
	nodeWithComputed := NewAstNode(nil, nil, types.IntType, &computedSz)
	if sz := eCtx.Size(nodeWithComputed); sz != computedSz {
		t.Errorf("eCtx.Size(computed) = %v, want %v", sz, computedSz)
	}

	failingStratECtx := &estimatorEvalContext{
		strategy:  failingSizingStrategy{},
		estimator: nil,
	}
	nodeWithoutSize := NewAstNode(nil, []string{"unknown"}, types.IntType, nil)
	if sz := failingStratECtx.Size(nodeWithoutSize); sz != UnknownSizeEstimate() {
		t.Errorf("failingStratECtx.Size = %v, want unknown", sz)
	}

	nilEstECtx := &estimatorEvalContext{
		strategy:  failingSizingStrategy{},
		estimator: testHintsEstimator{hints: nil},
	}
	if sz := nilEstECtx.Size(nodeWithoutSize); sz != UnknownSizeEstimate() {
		t.Errorf("nilEstECtx.Size = %v, want unknown", sz)
	}

	succStratECtx := &estimatorEvalContext{
		strategy: DefaultSizingStrategy(),
	}
	litNode := NewAstNode(nil, nil, types.IntType, nil)
	if sz := succStratECtx.Size(litNode); sz != FixedSizeEstimate(1) {
		t.Errorf("succStratECtx.Size = %v, want 1", sz)
	}
}

type versionRecordingStrategy struct {
	defaultSizingStrategy
	versions []uint32
}

func (s *versionRecordingStrategy) EstimateSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	s.versions = append(s.versions, versionOf(ctx))
	return s.defaultSizingStrategy.EstimateSize(ctx, node)
}

func TestCosterEvalContextVersionWithEstimator(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewCall(2, "size", fac.NewIdent(1, "x"))
	checked := ast.NewCheckedAST(ast.NewAST(expr, nil), map[int64]*types.Type{
		1: types.StringType,
		2: types.IntType,
	}, map[int64]*ast.ReferenceInfo{
		2: ast.NewFunctionReference("size_string"),
	})

	for _, wantVersion := range []uint32{0, 1, defaultModelVersion} {
		strat := &versionRecordingStrategy{}
		rawEstimator := testHintsEstimator{hints: map[string]uint64{"x": 10}}
		est, err := Cost(checked, rawEstimator, WithSizingStrategy(strat), ModelVersion(wantVersion))
		if err != nil {
			t.Fatalf("Cost() failed: %v", err)
		}
		if est != FixedCostEstimate(2) {
			t.Errorf("Cost() = %v, want 2", est)
		}
		if len(strat.versions) == 0 {
			t.Fatalf("expected SizingStrategy.EstimateSize to be called during Cost()")
		}
		for _, got := range strat.versions {
			if got != wantVersion {
				t.Errorf("SizingStrategy observed versionOf(ctx) = %d at ModelVersion(%d), want %d", got, wantVersion, wantVersion)
			}
		}
	}
}

type failingSizingStrategy struct{}

func (failingSizingStrategy) EstimateSize(EstimateContext, AstNode) (SizeEstimate, bool) {
	return SizeEstimate{}, false
}

func (failingSizingStrategy) TrackSize(TrackContext, ref.Val) (uint64, bool) {
	return 0, false
}

type customTrackerSizingStrategy struct {
	defaultSizingStrategy
}

func (customTrackerSizingStrategy) TrackSize(ctx TrackContext, value ref.Val) (uint64, bool) {
	if s, ok := value.(types.String); ok {
		return uint64(len(string(s)) * 2), true
	}
	return ActualSize(value), true
}

type simpleCall struct {
	function   string
	overloadID string
}

func (s simpleCall) Function() string   { return s.function }
func (s simpleCall) OverloadID() string { return s.overloadID }

type testHintsEstimator struct {
	hints map[string]uint64
}

func (t testHintsEstimator) EstimateSize(node AstNode) *SizeEstimate {
	if node == nil || len(node.Path()) == 0 {
		return nil
	}
	key := strings.Join(node.Path(), ".")
	if val, ok := t.hints[key]; ok {
		sz := FixedSizeEstimate(val)
		return &sz
	}
	return nil
}

func (t testHintsEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	return nil
}
