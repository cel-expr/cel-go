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
	"strings"
	"testing"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

type testEvalContext struct {
	estimator  Estimator
	strategy   SizingStrategy
	args       []SizeEstimate
	receiver   *SizeEstimate
	result     *SizeEstimate
	targetType *types.Type
	argTypes   []*types.Type
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
	if sz := computeTypeSize(node.Type()); sz != nil {
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

func TestQuantityExprs_Estimate(t *testing.T) {
	elem0 := FixedSizeEstimate(7)
	key1 := FixedSizeEstimate(3)
	elem1 := FixedSizeEstimate(12)
	rcvElem := FixedSizeEstimate(15)
	rcvKey := FixedSizeEstimate(4)

	arg0 := ListSizeEstimate(RangedSizeEstimate(2, 10), elem0)
	arg1 := MapSizeEstimate(RangedSizeEstimate(3, 5), key1, elem1)
	rcv := MapSizeEstimate(RangedSizeEstimate(4, 8), rcvKey, rcvElem)

	ctx := &testEvalContext{
		args: []SizeEstimate{
			arg0,
			arg1,
		},
		receiver:   &rcv,
		targetType: types.NewListType(types.StringType),
		argTypes:   []*types.Type{types.NewListType(types.IntType)},
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
			name:     "mul_quantity",
			expr:     Mul(Arg(0), Arg(1)),
			expected: RangedSizeEstimate(6, 50),
		},
		{
			name:     "scale_quantity",
			expr:     Scale(Arg(0), 0.5),
			expected: ListSizeEstimate(RangedSizeEstimate(1, 5), elem0),
		},
		{
			name:     "square_quantity",
			expr:     Square(Arg(1)),
			expected: RangedSizeEstimate(9, 25),
		},
		{
			name:     "square_and_scale_quantity",
			expr:     Scale(Square(Arg(1)), 2.0),
			expected: RangedSizeEstimate(18, 50),
		},
		{
			name:     "min_quantity",
			expr:     Min(Arg(0), Arg(1)),
			expected: SizeEstimate{Min: 1, Max: 5},
		},
		{
			name:     "max_quantity",
			expr:     Max(Arg(0), Arg(1)),
			expected: RangedSizeEstimate(3, 10),
		},
		{
			name:     "union_quantity",
			expr:     Union(Arg(0), Arg(1)),
			expected: arg0.Union(arg1),
		},
		{
			name:     "intersect_quantity",
			expr:     Intersect(Arg(0), Arg(1)),
			expected: RangedSizeEstimate(3, 5),
		},
		{
			name:     "ranged_string_to_bytes",
			expr:     Ranged(Arg(0), Scale(Arg(0), 4.0)),
			expected: RangedSizeEstimate(2, 40),
		},
		{
			name:     "ranged_bytes_to_string",
			expr:     Ranged(Scale(Arg(0), 0.25), Arg(0)),
			expected: RangedSizeEstimate(1, 10),
		},
		{
			name:     "ranged_expression_composition",
			expr:     Ranged(Sum(Arg(0), Const(2)), Sum(Scale(Arg(0), 2.0), Const(2))),
			expected: RangedSizeEstimate(4, 22),
		},
		{
			name:     "at_most_quantity",
			expr:     AtMost(Arg(0)),
			expected: RangedSizeEstimate(0, 10),
		},
		{
			name:     "list_quantity",
			expr:     List(AtMost(Arg(0)), Arg(0)),
			expected: ListSizeEstimate(RangedSizeEstimate(0, 10), arg0),
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
	trackCtx := &testTrackContext{
		args:     []uint64{10, 5},
		receiver: 8,
	}

	scalarTests := []struct {
		name     string
		expr     QuantityExpr
		expected uint64
	}{
		{name: "const_scalar", expr: Const(42), expected: 42},
		{name: "arg_scalar", expr: Arg(0), expected: 10},
		{name: "target_scalar", expr: Target(), expected: 8},
		{name: "sum_scalar", expr: Sum(Arg(0), Arg(1)), expected: 15},
		{name: "mul_scalar", expr: Mul(Arg(0), Arg(1)), expected: 50},
		{name: "scale_scalar", expr: Scale(Arg(0), 1.5), expected: 15},
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

type testTrackContext struct {
	args      []uint64
	receiver  uint64
	result    uint64
	estimator ActualCostEstimator
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
		{name: "target_expr", expr: Target(), wantTarget: true},
		{name: "target_elem_expr", expr: TargetElem(), wantTarget: true},
		{name: "target_key_expr", expr: TargetKey(), wantTarget: true},
		{name: "result_expr", expr: Result(), wantTarget: false},
		{name: "elem_of_target", expr: ElemOf(Target()), wantTarget: true},
		{name: "elem_of_arg", expr: ElemOf(Arg(0)), wantTarget: false},
		{name: "key_of_target", expr: KeyOf(Target()), wantTarget: true},
		{name: "key_of_arg", expr: KeyOf(Arg(0)), wantTarget: false},
		{name: "min_with_target", expr: Min(Arg(0), Target()), wantTarget: true},
		{name: "min_without_target", expr: Min(Arg(0), Arg(1)), wantTarget: false},
		{name: "max_with_target", expr: Max(Target(), Arg(0)), wantTarget: true},
		{name: "max_without_target", expr: Max(Arg(0), Arg(1)), wantTarget: false},
		{name: "union_with_target", expr: Union(Arg(0), Target()), wantTarget: true},
		{name: "union_without_target", expr: Union(Arg(0), Arg(1)), wantTarget: false},
		{name: "intersect_with_target", expr: Intersect(Target(), Arg(0)), wantTarget: true},
		{name: "intersect_without_target", expr: Intersect(Arg(0), Arg(1)), wantTarget: false},
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
		{
			name:         "empty_intersect",
			expr:         Intersect(),
			wantEstimate: FixedSizeEstimate(0),
			wantTrack:    0,
		},
		{
			name:         "disjoint_intersect",
			expr:         Intersect(Const(5), Const(10)),
			wantEstimate: FixedSizeEstimate(0),
			wantTrack:    5,
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
			name:      "missing_arg_key",
			expr:      ArgKey(0),
			wantTrack: 1,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != FixedSizeEstimate(1) {
					t.Errorf("got %v, want 1", sz)
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
			wantTrack: 1,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != FixedSizeEstimate(1) {
					t.Errorf("got %v, want 1", sz)
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
			name:      "key_of_scalar_const",
			expr:      KeyOf(Const(5)),
			wantTrack: 1,
			checkEst: func(t *testing.T, sz SizeEstimate) {
				if sz != FixedSizeEstimate(1) {
					t.Errorf("got %v, want 1", sz)
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

func TestModel_ScaleFnAdapters(t *testing.T) {
	typeCtx := &testEvalContext{
		targetType: types.NewListType(types.StringType),
		argTypes: []*types.Type{
			types.NewMapType(types.StringType, types.IntType),
			types.NewOpaqueType("custom", types.DoubleType),
			types.IntType,
			types.NewTypeTypeWithParam(types.StringType),
		},
	}
	emptyCtx := &testEvalContext{}

	tests := []struct {
		name      string
		scaleFn   ScaleFn
		ctx       TypeContext
		wantScale float64
	}{
		{
			name: "arg_type_scale_matched",
			scaleFn: ArgTypeScale(0, func(t *types.Type) float64 {
				if t != nil && t.Kind() == types.MapKind {
					return 2.5
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 2.5,
		},
		{
			name: "arg_type_scale_out_of_bounds",
			scaleFn: ArgTypeScale(10, func(t *types.Type) float64 {
				if t == nil {
					return 0.5
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 0.5,
		},
		{
			name: "arg_elem_type_scale_map_elem",
			scaleFn: ArgElemTypeScale(0, func(t *types.Type) float64 {
				if t == types.IntType {
					return 3.0
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 3.0,
		},
		{
			name: "arg_elem_type_scale_opaque_elem",
			scaleFn: ArgElemTypeScale(1, func(t *types.Type) float64 {
				if t == types.DoubleType {
					return 4.0
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 4.0,
		},
		{
			name: "arg_elem_type_scale_no_param",
			scaleFn: ArgElemTypeScale(2, func(t *types.Type) float64 {
				if t == nil {
					return 5.0
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 5.0,
		},
		{
			name: "arg_elem_type_scale_non_container_param",
			scaleFn: ArgElemTypeScale(3, func(t *types.Type) float64 {
				if t == nil {
					return 99.0
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 99.0,
		},
		{
			name: "arg_elem_type_scale_out_of_bounds",
			scaleFn: ArgElemTypeScale(10, func(t *types.Type) float64 {
				if t == nil {
					return 0.5
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 0.5,
		},
		{
			name: "target_type_scale_matched",
			scaleFn: TargetTypeScale(func(t *types.Type) float64 {
				if t != nil && t.Kind() == types.ListKind {
					return 6.0
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 6.0,
		},
		{
			name: "target_type_scale_empty_ctx",
			scaleFn: TargetTypeScale(func(t *types.Type) float64 {
				if t == nil {
					return 0.5
				}
				return 1.0
			}),
			ctx:       emptyCtx,
			wantScale: 0.5,
		},
		{
			name: "target_elem_type_scale_list_elem",
			scaleFn: TargetElemTypeScale(func(t *types.Type) float64 {
				if t == types.StringType {
					return 7.0
				}
				return 1.0
			}),
			ctx:       typeCtx,
			wantScale: 7.0,
		},
		{
			name: "target_elem_type_scale_empty_ctx",
			scaleFn: TargetElemTypeScale(func(t *types.Type) float64 {
				if t == nil {
					return 0.5
				}
				return 1.0
			}),
			ctx:       emptyCtx,
			wantScale: 0.5,
		},
		{
			name: "target_elem_type_scale_scalar_target",
			scaleFn: TargetElemTypeScale(func(t *types.Type) float64 {
				if t == nil {
					return 0.5
				}
				return 1.0
			}),
			ctx:       &testEvalContext{targetType: types.IntType},
			wantScale: 0.5,
		},
		{
			name: "target_elem_type_scale_opaque_target",
			scaleFn: TargetElemTypeScale(func(t *types.Type) float64 {
				if t == types.BoolType {
					return 8.0
				}
				return 1.0
			}),
			ctx:       &testEvalContext{targetType: types.NewOpaqueType("wrapper", types.BoolType)},
			wantScale: 8.0,
		},
		{
			name: "target_elem_type_scale_map_target",
			scaleFn: TargetElemTypeScale(func(t *types.Type) float64 {
				if t == types.DoubleType {
					return 9.0
				}
				return 1.0
			}),
			ctx:       &testEvalContext{targetType: types.NewMapType(types.StringType, types.DoubleType)},
			wantScale: 9.0,
		},
		{
			name: "target_elem_type_scale_non_container_param",
			scaleFn: TargetElemTypeScale(func(t *types.Type) float64 {
				if t == nil {
					return 99.0
				}
				return 1.0
			}),
			ctx:       &testEvalContext{targetType: types.NewTypeTypeWithParam(types.StringType)},
			wantScale: 99.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if s := tc.scaleFn(tc.ctx); s != tc.wantScale {
				t.Errorf("scale = %v, want %v", s, tc.wantScale)
			}
		})
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
}

func TestModel_TrackerEvalContextMethods(t *testing.T) {
	tCtx := &trackerEvalContext{
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
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}

	nonMemberTCtx := &trackerEvalContext{
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
