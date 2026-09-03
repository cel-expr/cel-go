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
	"slices"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// DefaultSizingStrategy returns the default SizingStrategy implementation.
func DefaultSizingStrategy() SizingStrategy {
	return defaultSizing
}

// defaultSizingStrategy is the default implementation of SizingStrategy.
type defaultSizingStrategy struct{}

// EstimateSize computes the size estimate for an AST node during cost estimation.
func (defaultSizingStrategy) EstimateSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	if node == nil {
		return SizeEstimate{}, false
	}
	if sz := node.ComputedSize(); sz != nil {
		return *sz, true
	}
	if node.Type() == nil {
		return estimateScalarOrFallback(ctx, node)
	}
	switch node.Type().Kind() {
	case types.ListKind:
		return estimateDefaultListSize(ctx, node)
	case types.MapKind:
		return estimateDefaultMapSize(ctx, node)
	default:
		return estimateScalarOrFallback(ctx, node)
	}
}

// estimateDefaultListSize calculates the size and element size of a list AST node.
func estimateDefaultListSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	elemType := listElemType(node.Type())
	listSize, elemSize := estimateListExpr(ctx, node, elemType)

	if listSize == nil && ctx != nil && ctx.Estimator() != nil {
		listSize = ctx.Estimator().EstimateSize(node)
	}
	if elemSize == nil {
		elemSize = estimateSubpath(ctx, node.Path(), "@items", elemType)
	}
	return combineListSize(listSize, elemSize)
}

// estimateDefaultMapSize calculates the size, key size, and value size of a map AST node.
func estimateDefaultMapSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	keyType, valType := mapKeyValueTypes(node.Type())
	mapSize, keySize, valSize := estimateMapExpr(ctx, node, keyType, valType)

	if mapSize == nil && ctx != nil && ctx.Estimator() != nil {
		mapSize = ctx.Estimator().EstimateSize(node)
	}
	if keySize == nil {
		keySize = estimateSubpath(ctx, node.Path(), "@keys", keyType)
	}
	if valSize == nil {
		valSize = estimateSubpath(ctx, node.Path(), "@values", valType)
	}
	return combineMapSize(mapSize, keySize, valSize)
}

// estimateSubpath attempts to estimate the size of a nested child node by path, falling back to primitive type size.
func estimateSubpath(ctx EstimateContext, basePath []string, subpath string, t *types.Type) *SizeEstimate {
	if len(basePath) > 0 && ctx != nil && ctx.Estimator() != nil {
		childPath := append(slices.Clone(basePath), subpath)
		childNode := NewAstNode(nil, childPath, t, nil)
		if sz := ctx.Estimator().EstimateSize(childNode); sz != nil {
			return sz
		}
	}
	return computeTypeSize(t)
}

// TrackSize computes the actual runtime size of a value.
func (defaultSizingStrategy) TrackSize(ctx TrackContext, value ref.Val) (uint64, bool) {
	if value == nil {
		return 0, false
	}
	return ActualSize(value), true
}

// listElemType extracts the element type of a list, defaulting to DynType if not parameterized.
func listElemType(t *types.Type) *types.Type {
	if len(t.Parameters()) > 0 && t.Parameters()[0] != nil {
		return t.Parameters()[0]
	}
	return types.DynType
}

// mapKeyValueTypes extracts key and value types of a map, defaulting to DynType if not parameterized.
func mapKeyValueTypes(t *types.Type) (keyType, valType *types.Type) {
	keyType = types.DynType
	valType = types.DynType
	if len(t.Parameters()) >= 2 {
		keyType = t.Parameters()[0]
		valType = t.Parameters()[1]
	}
	return keyType, valType
}

// unionAccumulator merges a new size estimate into an existing accumulator pointer.
func unionAccumulator(acc *SizeEstimate, next SizeEstimate) *SizeEstimate {
	if acc == nil {
		return &next
	}
	u := acc.Union(next)
	return &u
}

// estimateConditionalSize computes the union SizeEstimate of the branches in a conditional call.
func estimateConditionalSize(ctx EstimateContext, nodeType *types.Type, args []ast.Expr) (SizeEstimate, bool) {
	if len(args) == 3 {
		tVal := ctx.Size(NewAstNode(args[1], nil, nodeType, nil))
		fVal := ctx.Size(NewAstNode(args[2], nil, nodeType, nil))
		return tVal.Union(fVal), true
	}
	return SizeEstimate{}, false
}

// estimateListExpr computes the list and element size estimates for literal list expressions
// or container call operations (e.g. Add, Conditional).
func estimateListExpr(ctx EstimateContext, node AstNode, elemType *types.Type) (listSize *SizeEstimate, elemSize *SizeEstimate) {
	if node.Expr() == nil {
		return nil, nil
	}
	switch node.Expr().Kind() {
	case ast.ListKind:
		elements := node.Expr().AsList().Elements()
		l := FixedSizeEstimate(uint64(len(elements)))
		listSize = &l
		for _, elem := range elements {
			elemNode := NewAstNode(elem, nil, elemType, nil)
			elemSize = unionAccumulator(elemSize, ctx.Size(elemNode))
		}
	case ast.CallKind:
		call := node.Expr().AsCall()
		args := call.Args()
		switch call.FunctionName() {
		case operators.Add:
			if len(args) == 2 {
				lhs := ctx.Size(NewAstNode(args[0], nil, node.Type(), nil))
				rhs := ctx.Size(NewAstNode(args[1], nil, node.Type(), nil))
				added := lhs.Add(rhs)
				listSize = &added
				elemSize = added.Elem
			}
		case operators.Conditional:
			if u, ok := estimateConditionalSize(ctx, node.Type(), args); ok {
				listSize = &u
				elemSize = u.Elem
			}
		}
	}
	return listSize, elemSize
}

// estimateMapExpr computes map, key, and value size estimates for literal map expressions
// or container call operations (e.g. Conditional).
func estimateMapExpr(ctx EstimateContext, node AstNode, keyType, valType *types.Type) (mapSize *SizeEstimate, keySize *SizeEstimate, valSize *SizeEstimate) {
	if node.Expr() == nil {
		return nil, nil, nil
	}
	switch node.Expr().Kind() {
	case ast.MapKind:
		entries := node.Expr().AsMap().Entries()
		m := FixedSizeEstimate(uint64(len(entries)))
		mapSize = &m
		for _, entry := range entries {
			mapEntry := entry.AsMapEntry()
			kNode := NewAstNode(mapEntry.Key(), nil, keyType, nil)
			vNode := NewAstNode(mapEntry.Value(), nil, valType, nil)
			keySize = unionAccumulator(keySize, ctx.Size(kNode))
			valSize = unionAccumulator(valSize, ctx.Size(vNode))
		}
	case ast.CallKind:
		call := node.Expr().AsCall()
		if call.FunctionName() == operators.Conditional {
			if u, ok := estimateConditionalSize(ctx, node.Type(), call.Args()); ok {
				mapSize = &u
				keySize = u.Key
				valSize = u.Elem
			}
		}
	}
	return mapSize, keySize, valSize
}

// estimateScalarOrFallback returns size estimates using custom estimator hints or primitive type sizes.
func estimateScalarOrFallback(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	if ctx != nil && ctx.Estimator() != nil {
		if sz := ctx.Estimator().EstimateSize(node); sz != nil {
			return *sz, true
		}
	}
	if node.Type() != nil {
		if sz := computeTypeSize(node.Type()); sz != nil {
			return *sz, true
		}
	}
	return SizeEstimate{}, false
}

// combineListSize constructs a list SizeEstimate combining container length and element size.
func combineListSize(listSize, elemSize *SizeEstimate) (SizeEstimate, bool) {
	if listSize != nil {
		if elemSize != nil {
			listSize.Elem = elemSize
		}
		return *listSize, true
	}
	if elemSize != nil {
		res := UnknownSizeEstimate()
		res.Elem = elemSize
		return res, true
	}
	return SizeEstimate{}, false
}

// combineMapSize constructs a map SizeEstimate combining map size, key size, and value size.
func combineMapSize(mapSize, keySize, valSize *SizeEstimate) (SizeEstimate, bool) {
	if mapSize != nil {
		mapSize.Key = keySize
		mapSize.Elem = valSize
		return *mapSize, true
	}
	if keySize != nil || valSize != nil {
		res := UnknownSizeEstimate()
		res.Key = keySize
		res.Elem = valSize
		return res, true
	}
	return SizeEstimate{}, false
}

var defaultSizing SizingStrategy = defaultSizingStrategy{}
