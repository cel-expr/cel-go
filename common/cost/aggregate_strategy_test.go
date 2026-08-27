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

func TestAggregateSizingStrategy_TrackSize(t *testing.T) {
	adapter := types.DefaultTypeAdapter
	strVal := types.String("hello_world")                                          // len 11
	nestedList := adapter.NativeToValue([][]int{{1, 2}, {3, 4}})                   // outer len 2, each inner len 2
	nestedMap := adapter.NativeToValue(map[string][]int{"a": {1, 2}, "b": {3, 4}}) // 2 keys, 2 list values

	tests := []struct {
		name     string
		strat    SizingStrategy
		val      ref.Val
		wantSize uint64
		wantOk   bool
	}{
		{
			name:     "nil_value",
			strat:    AggregateSizingStrategy(),
			val:      nil,
			wantSize: 0,
			wantOk:   false,
		},
		{
			name:     "string_scalar",
			strat:    AggregateSizingStrategy(),
			val:      strVal,
			wantSize: 11,
			wantOk:   true,
		},
		{
			name:     "nested_list_recursive",
			strat:    AggregateSizingStrategy(),
			val:      nestedList,
			wantSize: 7, // 1 (outer list header) + (1 + 2) (inner list 1) + (1 + 2) (inner list 2)
			wantOk:   true,
		},
		{
			name:     "nested_map_recursive",
			strat:    AggregateSizingStrategy(),
			val:      nestedMap,
			wantSize: 9, // 1 (map) + 2 (key strings of len 1) + 2 * (1 + 2) (two inner lists)
			wantOk:   true,
		},
		{
			name:     "custom_string_unit_length",
			strat:    AggregateSizingStrategy(types.SizeCalculatorStringUnitLength(5)),
			val:      strVal,
			wantSize: 3, // (11 + 4) / 5 = 3
			wantOk:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSize, gotOk := tc.strat.TrackSize(nil, tc.val)
			if gotOk != tc.wantOk || gotSize != tc.wantSize {
				t.Errorf("TrackSize(%v) = (%d, %t), want (%d, %t)", tc.val, gotSize, gotOk, tc.wantSize, tc.wantOk)
			}
		})
	}
}

type dynEstimator struct {
	dynElemSize *SizeEstimate
}

func (d dynEstimator) EstimateSize(node AstNode) *SizeEstimate {
	if node != nil && node.Type() == types.DynType && len(node.Path()) == 0 {
		return d.dynElemSize
	}
	return nil
}

func (d dynEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	return nil
}

type customMapEstimator struct {
	mapEst  *SizeEstimate
	listEst *SizeEstimate
}

func (c customMapEstimator) EstimateSize(node AstNode) *SizeEstimate {
	if node != nil && len(node.Path()) > 0 {
		if node.Path()[0] == "list_with_elem_est" {
			if len(node.Path()) == 1 {
				return c.listEst
			}
			if len(node.Path()) == 2 && node.Path()[1] == "@items" {
				sub := ListSizeEstimate(FixedSizeEstimate(2), FixedSizeEstimate(5))
				return &sub
			}
		}
		if node.Path()[0] == "map_with_entries_est" {
			if len(node.Path()) == 1 {
				return c.mapEst
			}
			if len(node.Path()) == 2 && node.Path()[1] == "@keys" {
				sub := ListSizeEstimate(FixedSizeEstimate(2), FixedSizeEstimate(6))
				return &sub
			}
			if len(node.Path()) == 2 && node.Path()[1] == "@values" {
				sub := ListSizeEstimate(FixedSizeEstimate(4), FixedSizeEstimate(8))
				return &sub
			}
		}
	}
	return nil
}

func (c customMapEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	return nil
}

func TestAggregateSizingStrategy_EstimateSize_List(t *testing.T) {
	strat := AggregateSizingStrategy()
	evalCtx := &testEvalContext{strategy: strat}
	fac := ast.NewExprFactory()

	listType := types.NewListType(types.StringType)
	elemExpr1 := fac.NewLiteral(1, types.String("first"))
	elemExpr2 := fac.NewLiteral(2, types.String("second"))
	listExpr := fac.NewList(3, []ast.Expr{elemExpr1, elemExpr2}, []int32{})
	emptyListExpr := fac.NewList(4, []ast.Expr{}, []int32{})
	addCallExpr := fac.NewCall(5, operators.Add, listExpr, listExpr)
	condCallExpr := fac.NewCall(6, operators.Conditional, fac.NewLiteral(7, types.True), listExpr, emptyListExpr)

	nestedListType := types.NewListType(types.NewListType(types.StringType))
	nestedHints := map[string]uint64{
		"nested":               5,
		"nested.@items":        10,
		"nested.@items.@items": 20,
	}
	nestedCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: nestedHints},
		strategy:  strat,
	}

	dynElemSz := FixedSizeEstimate(8)
	dynListCtx := &testEvalContext{
		estimator: dynEstimator{dynElemSize: &dynElemSz},
		strategy:  strat,
	}

	listWithElem := ListSizeEstimate(FixedSizeEstimate(3), FixedSizeEstimate(2))
	customEstCtx := &testEvalContext{
		estimator: customMapEstimator{listEst: &listWithElem},
		strategy:  strat,
	}

	tests := []struct {
		name     string
		ctx      EstimateContext
		node     AstNode
		wantOk   bool
		check    func(t *testing.T, sz SizeEstimate)
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
			name:   "recursive_items_path_exploration",
			ctx:    nestedCtx,
			node:   NewAstNode(nil, []string{"nested"}, nestedListType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Max != 5 || sz.Elem == nil || sz.Elem.Max != 10 || sz.Elem.Elem == nil || sz.Elem.Elem.Max != 20 {
					t.Errorf("got %v, want Max 5, Elem.Max 10, Elem.Elem.Max 20", sz)
				}
			},
		},
		{
			name:   "compute_type_size_element",
			ctx:    evalCtx,
			node:   NewAstNode(nil, nil, types.NewListType(types.IntType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Elem == nil || sz.Elem.Max != 1 {
					t.Errorf("got %v, want Elem.Max 1", sz)
				}
			},
		},
		{
			name:   "dyn_element_from_estimator",
			ctx:    dynListCtx,
			node:   NewAstNode(nil, []string{"dyn_list"}, types.NewListType(types.DynType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Elem == nil || sz.Elem.Max != 8 {
					t.Errorf("got %v, want Elem.Max 8", sz)
				}
			},
		},
		{
			name:   "list_with_estimator_element_and_nested_sub_element",
			ctx:    customEstCtx,
			node:   NewAstNode(nil, []string{"list_with_elem_est"}, nestedListType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Elem == nil || sz.Elem.Elem == nil || sz.Elem.Elem.Max != 5 {
					t.Errorf("got %v, want Elem.Elem.Max 5", sz)
				}
			},
		},
		{
			name:   "unknown_list_without_hints",
			ctx:    evalCtx,
			node:   NewAstNode(nil, []string{"unknown_list"}, types.NewListType(types.DynType), nil),
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

func TestAggregateSizingStrategy_EstimateSize_Map(t *testing.T) {
	strat := AggregateSizingStrategy()
	evalCtx := &testEvalContext{strategy: strat}
	fac := ast.NewExprFactory()

	mapType := types.NewMapType(types.StringType, types.NewListType(types.StringType))
	elemExpr1 := fac.NewLiteral(1, types.String("first"))
	listExpr := fac.NewList(2, []ast.Expr{elemExpr1}, []int32{})
	entry1 := fac.NewMapEntry(3, fac.NewLiteral(4, types.String("k1")), listExpr, false)
	entry2 := fac.NewMapEntry(5, fac.NewLiteral(6, types.String("k2")), fac.NewLiteral(7, types.Int(20)), false)
	mapExprSingle := fac.NewMap(8, []ast.EntryExpr{entry1})
	mapExprMulti := fac.NewMap(9, []ast.EntryExpr{
		fac.NewMapEntry(10, fac.NewLiteral(11, types.String("k1")), fac.NewLiteral(12, types.Int(10)), false),
		entry2,
	})
	emptyMapExpr := fac.NewMap(13, []ast.EntryExpr{})
	condMapExpr := fac.NewCall(14, operators.Conditional, fac.NewLiteral(15, types.True), mapExprSingle, emptyMapExpr)

	mapHints := map[string]uint64{
		"mm":                5,
		"mm.@keys":          3,
		"mm.@values":        4,
		"mm.@values.@items": 6,
	}
	mapAggCtx := &testEvalContext{
		estimator: testHintsEstimator{hints: mapHints},
		strategy:  strat,
	}

	dynElemSz := FixedSizeEstimate(8)
	dynMapCtx := &testEvalContext{
		estimator: dynEstimator{dynElemSize: &dynElemSz},
		strategy:  strat,
	}

	mapWithEntries := MapSizeEstimate(
		FixedSizeEstimate(5),
		FixedSizeEstimate(2),
		FixedSizeEstimate(4),
	)
	customEstCtx := &testEvalContext{
		estimator: customMapEstimator{mapEst: &mapWithEntries},
		strategy:  strat,
	}
	nestedMapType := types.NewMapType(types.NewListType(types.StringType), types.NewListType(types.StringType))

	tests := []struct {
		name   string
		ctx    EstimateContext
		node   AstNode
		wantOk bool
		check  func(t *testing.T, sz SizeEstimate)
	}{
		{
			name:   "literal_map_single_entry",
			ctx:    evalCtx,
			node:   NewAstNode(mapExprSingle, nil, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 1 || sz.Max != 1 {
					t.Errorf("got [%d, %d], want [1, 1]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "literal_map_multiple_entries",
			ctx:    evalCtx,
			node:   NewAstNode(mapExprMulti, nil, types.NewMapType(types.StringType, types.IntType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 2 || sz.Max != 2 {
					t.Errorf("got [%d, %d], want [2, 2]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "conditional_call_map",
			ctx:    evalCtx,
			node:   NewAstNode(condMapExpr, nil, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Min != 0 || sz.Max != 1 {
					t.Errorf("got [%d, %d], want [0, 1]", sz.Min, sz.Max)
				}
			},
		},
		{
			name:   "recursive_keys_and_values_exploration",
			ctx:    mapAggCtx,
			node:   NewAstNode(nil, []string{"mm"}, mapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Max != 5 || sz.Key == nil || sz.Key.Max != 3 || sz.Elem == nil || sz.Elem.Max != 4 || sz.Elem.Elem == nil || sz.Elem.Elem.Max != 6 {
					t.Errorf("got %v, want Max 5, Key.Max 3, Elem.Max 4 with Elem.Elem.Max 6", sz)
				}
			},
		},
		{
			name:   "compute_type_size_key_and_val",
			ctx:    evalCtx,
			node:   NewAstNode(nil, nil, types.NewMapType(types.IntType, types.BoolType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Key == nil || sz.Key.Max != 1 || sz.Elem == nil || sz.Elem.Max != 1 {
					t.Errorf("got %v, want Key.Max 1, Elem.Max 1", sz)
				}
			},
		},
		{
			name:   "dyn_key_and_val_from_estimator",
			ctx:    dynMapCtx,
			node:   NewAstNode(nil, []string{"dyn_map"}, types.NewMapType(types.DynType, types.DynType), nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Key == nil || sz.Key.Max != 8 || sz.Elem == nil || sz.Elem.Max != 8 {
					t.Errorf("got %v, want Key.Max 8, Elem.Max 8", sz)
				}
			},
		},
		{
			name:   "map_with_estimator_entries_and_nested_sub_items",
			ctx:    customEstCtx,
			node:   NewAstNode(nil, []string{"map_with_entries_est"}, nestedMapType, nil),
			wantOk: true,
			check: func(t *testing.T, sz SizeEstimate) {
				if sz.Key == nil || sz.Key.Elem == nil || sz.Key.Elem.Max != 6 || sz.Elem == nil || sz.Elem.Elem == nil || sz.Elem.Elem.Max != 8 {
					t.Errorf("got %v, want Key.Elem.Max 6, Elem.Elem.Max 8", sz)
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

func TestAggregateSizingStrategy_EstimateSize_ScalarAndFallback(t *testing.T) {
	strat := AggregateSizingStrategy()
	evalCtx := &testEvalContext{strategy: strat}

	compSz := FixedSizeEstimate(55)
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
