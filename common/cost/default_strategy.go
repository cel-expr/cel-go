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

type defaultSizingStrategy struct{}

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

func estimateDefaultListSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	elemType := listElemType(node.Type())
	listSize, elemSize := estimateListExpr(ctx, node, elemType)

	if listSize == nil && ctx != nil && ctx.Estimator() != nil {
		listSize = ctx.Estimator().EstimateSize(node)
	}
	if elemSize == nil {
		if len(node.Path()) > 0 && ctx != nil && ctx.Estimator() != nil {
			elemPath := append(slices.Clone(node.Path()), "@items")
			elemNode := NewAstNode(nil, elemPath, elemType, nil)
			elemSize = ctx.Estimator().EstimateSize(elemNode)
		}
		if elemSize == nil {
			elemSize = computeTypeSize(elemType)
		}
	}
	return combineListSize(listSize, elemSize)
}

func estimateDefaultMapSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	keyType, valType := mapKeyValueTypes(node.Type())
	mapSize, keySize, valSize := estimateMapExpr(ctx, node, keyType, valType)

	if mapSize == nil && ctx != nil && ctx.Estimator() != nil {
		mapSize = ctx.Estimator().EstimateSize(node)
	}
	if len(node.Path()) > 0 && ctx != nil && ctx.Estimator() != nil {
		if keySize == nil {
			kPath := append(slices.Clone(node.Path()), "@keys")
			keySize = ctx.Estimator().EstimateSize(NewAstNode(nil, kPath, keyType, nil))
		}
		if valSize == nil {
			vPath := append(slices.Clone(node.Path()), "@values")
			valSize = ctx.Estimator().EstimateSize(NewAstNode(nil, vPath, valType, nil))
		}
	}
	if keySize == nil {
		keySize = computeTypeSize(keyType)
	}
	if valSize == nil {
		valSize = computeTypeSize(valType)
	}
	return combineMapSize(mapSize, keySize, valSize)
}

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
			sz := ctx.Size(elemNode)
			if elemSize == nil {
				elemSize = &sz
			} else {
				u := elemSize.Union(sz)
				elemSize = &u
			}
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
			if len(args) == 3 {
				tVal := ctx.Size(NewAstNode(args[1], nil, node.Type(), nil))
				fVal := ctx.Size(NewAstNode(args[2], nil, node.Type(), nil))
				u := tVal.Union(fVal)
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
			kSz := ctx.Size(kNode)
			vSz := ctx.Size(vNode)
			if keySize == nil {
				keySize = &kSz
			} else {
				u := keySize.Union(kSz)
				keySize = &u
			}
			if valSize == nil {
				valSize = &vSz
			} else {
				u := valSize.Union(vSz)
				valSize = &u
			}
		}
	case ast.CallKind:
		call := node.Expr().AsCall()
		args := call.Args()
		if call.FunctionName() == operators.Conditional && len(args) == 3 {
			tVal := ctx.Size(NewAstNode(args[1], nil, node.Type(), nil))
			fVal := ctx.Size(NewAstNode(args[2], nil, node.Type(), nil))
			u := tVal.Union(fVal)
			mapSize = &u
			keySize = u.Key
			valSize = u.Elem
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
