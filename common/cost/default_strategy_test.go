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
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func TestDefaultSizingStrategy_TrackSize(t *testing.T) {
	strat := DefaultSizingStrategy()
	adapter := types.DefaultTypeAdapter

	tests := []struct {
		name     string
		val      ref.Val
		wantSize uint64
		wantOk   bool
	}{
		{
			name:     "nil_value",
			val:      nil,
			wantSize: 0,
			wantOk:   false,
		},
		{
			name:     "string_scalar",
			val:      types.String("hello"),
			wantSize: 5,
			wantOk:   true,
		},
		{
			name:     "list_flat_length",
			val:      adapter.NativeToValue([]int{1, 2, 3}),
			wantSize: 3,
			wantOk:   true,
		},
		{
			name:     "map_flat_length",
			val:      adapter.NativeToValue(map[string]int{"a": 1, "b": 2}),
			wantSize: 2,
			wantOk:   true,
		},
		{
			name:     "int_scalar",
			val:      types.Int(42),
			wantSize: 1,
			wantOk:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSize, gotOk := strat.TrackSize(nil, tc.val)
			if gotOk != tc.wantOk || gotSize != tc.wantSize {
				t.Errorf("TrackSize(%v) = (%d, %t), want (%d, %t)", tc.val, gotSize, gotOk, tc.wantSize, tc.wantOk)
			}
		})
	}
}

func TestDefaultSizingStrategy_EstimateSize_List(t *testing.T) {
	strat := DefaultSizingStrategy()
	evalCtx := &testEvalContext{strategy: strat}
	fac := ast.NewExprFactory()

	listType := types.NewListType(types.StringType)
	elemExpr1 := fac.NewLiteral(1, types.String("first"))
	elemExpr2 := fac.NewLiteral(2, types.String("second"))
	listExpr := fac.NewList(3, []ast.Expr{elemExpr1, elemExpr2}, []int32{})
	emptyListExpr := fac.NewList(4, []ast.Expr{}, []int32{})
	addCallExpr := fac.NewCall(5, operators.Add, listExpr, listExpr)
	condCallExpr := fac.NewCall(6, operators.Conditional, fac.NewLiteral(7, types.True), listExpr, emptyListExpr)

	listCtx := &testEvalContext{
		estimator: testHintsEstimator{
			hints: map[string]uint64{
				"my_list":        10,
				"my_list.@items": 25,
			},
		},
		strategy: strat,
	}

	intListCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: map[string]uint64{"int_list": 7}},
		strategy:  strat,
	}

	elemOnlyCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: map[string]uint64{"elem_only.@items": 15}},
		strategy:  strat,
	}

	tests := []struct {
		name   string
		ctx    EstimateContext
		node   AstNode
		wantOk bool
		check  func(t *testing.T, sz SizeEstimate)
	}{
		{
			name:   "literal_list_elements",
			ctx:    evalCtx,
			node:   NewAstNode(listExpr, nil, listType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 2 || sz.Max != 2 {
					t.Errorf("got [%d, %d], want [2, 2]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "empty_literal_list",
			ctx:    evalCtx,
			node:   NewAstNode(emptyListExpr, nil, listType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 0 || sz.Max != 0 {
					t.Errorf("got [%d, %d], want [0, 0]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "add_call_list",
			ctx:    evalCtx,
			node:   NewAstNode(addCallExpr, nil, listType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 4 || sz.Max != 4 {
					t.Errorf("got [%d, %d], want [4, 4]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "conditional_call_list",
			ctx:    evalCtx,
			node:   NewAstNode(condCallExpr, nil, listType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 0 || sz.Max != 2 {
					t.Errorf("got [%d, %d], want [0, 2]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "one_level_items_path_hint",
			ctx:    listCtx,
			node:   NewAstNode(nil, []string{"my_list"}, listType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Max != 10 || sz.Elem == nil || sz.Elem.Max != 25 {
					t.Errorf("got %v, want Max 10, Elem.Max 25", sz)
				}
			},
		},
		{
			name:   "compute_type_size_element",
			ctx:    intListCtx,
			node:   NewAstNode(nil, []string{"int_list"}, types.NewListType(types.IntType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Max != 7 || sz.Elem == nil || sz.Elem.Max != 1 {
					t.Errorf("got %v, want Max 7, Elem.Max 1", sz)
				}
			},
		},
		{
			name:   "elem_size_only_hint",
			ctx:    elemOnlyCtx,
			node:   NewAstNode(nil, []string{"elem_only"}, listType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Elem == nil || sz.Elem.Max != 15 {
					t.Errorf("got %v, want Elem.Max 15", sz)
				}
			},
		},
		{
			name:   "unknown_list_without_hints",
			ctx:    evalCtx,
			node:   NewAstNode(nil, []string{"unknown_list"}, types.NewListType(types.DynType), nil),
			wantOk: false,
		},
		{
			name:   "unparameterized_list_type",
			ctx:    evalCtx,
			node:   NewAstNode(nil, []string{"unparam_list"}, types.NewListType(nil), nil),
			wantOk: false,
		},
		{
			name:   "nil_type_list_node",
			ctx:    evalCtx,
			node:   NewAstNode(nil, []string{"nil_type_list"}, nil, nil),
			wantOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := strat.EstimateSize(tc.ctx, tc.node)
			if ok != tc.wantOk {
				t.Fatalf("EstimateSize() ok = %t, want %t", ok, tc.wantOk)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestDefaultSizingStrategy_EstimateSize_Map(t *testing.T) {
	strat := DefaultSizingStrategy()
	evalCtx := &testEvalContext{strategy: strat}
	fac := ast.NewExprFactory()

	mapType := types.NewMapType(types.StringType, types.IntType)
	entry1 := fac.NewMapEntry(8, fac.NewLiteral(9, types.String("k1")), fac.NewLiteral(10, types.Int(100)), false)
	entry2 := fac.NewMapEntry(11, fac.NewLiteral(12, types.String("k2")), fac.NewLiteral(13, types.Int(200)), false)
	mapExpr := fac.NewMap(14, []ast.EntryExpr{entry1, entry2})
	emptyMapExpr := fac.NewMap(15, []ast.EntryExpr{})
	condMapExpr := fac.NewCall(16, operators.Conditional, fac.NewLiteral(17, types.True), mapExpr, emptyMapExpr)

	mapCtx := &testEvalContext{
		estimator: testHintsEstimator{
			hints: map[string]uint64{
				"my_map":         8,
				"my_map.@keys":   12,
				"my_map.@values": 30,
			},
		},
		strategy: strat,
	}

	scalarMapCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: map[string]uint64{"scalar_map": 5}},
		strategy:  strat,
	}

	keyOnlyCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: map[string]uint64{"key_only.@keys": 10}},
		strategy:  strat,
	}

	tests := []struct {
		name   string
		ctx    EstimateContext
		node   AstNode
		wantOk bool
		check  func(t *testing.T, sz SizeEstimate)
	}{
		{
			name:   "literal_map_entries",
			ctx:    evalCtx,
			node:   NewAstNode(mapExpr, nil, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 2 || sz.Max != 2 {
					t.Errorf("got [%d, %d], want [2, 2]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "empty_literal_map",
			ctx:    evalCtx,
			node:   NewAstNode(emptyMapExpr, nil, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 0 || sz.Max != 0 {
					t.Errorf("got [%d, %d], want [0, 0]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "conditional_call_map",
			ctx:    evalCtx,
			node:   NewAstNode(condMapExpr, nil, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 0 || sz.Max != 2 {
					t.Errorf("got [%d, %d], want [0, 2]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "one_level_keys_and_values_path_hints",
			ctx:    mapCtx,
			node:   NewAstNode(nil, []string{"my_map"}, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Max != 8 || sz.Key == nil || sz.Key.Max != 12 || sz.Elem == nil || sz.Elem.Max != 30 {
					t.Errorf("got %v, want Max 8, Key 12, Elem 30", sz)
				}
			},
		},
		{
			name:   "compute_type_size_key_and_val",
			ctx:    scalarMapCtx,
			node:   NewAstNode(nil, []string{"scalar_map"}, types.NewMapType(types.IntType, types.BoolType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Max != 5 || sz.Key == nil || sz.Key.Max != 1 || sz.Elem == nil || sz.Elem.Max != 1 {
					t.Errorf("got %v, want Max 5, Key 1, Elem 1", sz)
				}
			},
		},
		{
			name:   "key_only_hint",
			ctx:    keyOnlyCtx,
			node:   NewAstNode(nil, []string{"key_only"}, types.NewMapType(types.StringType, types.DynType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Key == nil || sz.Key.Max != 10 {
					t.Errorf("got %v, want Key.Max 10", sz)
				}
			},
		},
		{
			name:   "unknown_map_without_hints",
			ctx:    evalCtx,
			node:   NewAstNode(nil, []string{"unknown_map"}, types.NewMapType(types.DynType, types.DynType), nil),
			wantOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := strat.EstimateSize(tc.ctx, tc.node)
			if ok != tc.wantOk {
				t.Fatalf("EstimateSize() ok = %t, want %t", ok, tc.wantOk)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestDefaultSizingStrategy_EstimateSize_ScalarAndFallback(t *testing.T) {
	strat := DefaultSizingStrategy()
	evalCtx := &testEvalContext{strategy: strat}

	compSz := FixedSizeEstimate(99)
	compNode := NewAstNode(nil, nil, types.StringType, &compSz)
	hintCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: map[string]uint64{"custom_var": 77, "untyped": 42}},
		strategy:  strat,
	}

	tests := []struct {
		name     string
		ctx      EstimateContext
		node     AstNode
		wantSize SizeEstimate
		wantOk   bool
	}{
		{
			name:     "nil_node",
			ctx:      evalCtx,
			node:     nil,
			wantSize: SizeEstimate{},
			wantOk:   false,
		},
		{
			name:     "computed_size_node",
			ctx:      evalCtx,
			node:     compNode,
			wantSize: compSz,
			wantOk:   true,
		},
		{
			name:     "untyped_node_nil_ctx",
			ctx:      nil,
			node:     NewAstNode(nil, []string{"untyped"}, nil, nil),
			wantSize: SizeEstimate{},
			wantOk:   false,
		},
		{
			name:     "untyped_node_with_hint",
			ctx:      hintCtx,
			node:     NewAstNode(nil, []string{"untyped"}, nil, nil),
			wantSize: FixedSizeEstimate(42),
			wantOk:   true,
		},
		{
			name:     "int_scalar",
			ctx:      evalCtx,
			node:     NewAstNode(nil, nil, types.IntType, nil),
			wantSize: FixedSizeEstimate(1),
			wantOk:   true,
		},
		{
			name:     "string_with_hint",
			ctx:      hintCtx,
			node:     NewAstNode(nil, []string{"custom_var"}, types.StringType, nil),
			wantSize: FixedSizeEstimate(77),
			wantOk:   true,
		},
		{
			name:     "string_scalar_no_hints",
			ctx:      evalCtx,
			node:     NewAstNode(nil, nil, types.StringType, nil),
			wantSize: SizeEstimate{},
			wantOk:   false,
		},
		{
			name:     "dyn_scalar_without_hints",
			ctx:      evalCtx,
			node:     NewAstNode(nil, []string{"dyn_var"}, types.DynType, nil),
			wantSize: SizeEstimate{},
			wantOk:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := strat.EstimateSize(tc.ctx, tc.node)
			if ok != tc.wantOk || got != tc.wantSize {
				t.Errorf("EstimateSize() = (%v, %t), want (%v, %t)", got, ok, tc.wantSize, tc.wantOk)
			}
		})
	}
}
