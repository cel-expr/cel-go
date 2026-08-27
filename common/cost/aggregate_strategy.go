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

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// AggregateSizingStrategy returns a SizingStrategy that computes recursive size estimates
// by following paths during cost estimation, and calculates actual runtime size using
// AggregateSize during cost tracking. By default, stringUnitLength is configured to 1
// so that unscaled character/byte lengths are preserved for cost modeling.
func AggregateSizingStrategy(opts ...types.SizeCalculatorOption) SizingStrategy {
	if len(opts) == 0 {
		return defaultAggregateSizing
	}
	defaultOpts := []types.SizeCalculatorOption{types.SizeCalculatorStringUnitLength(1)}
	return &aggregateSizingStrategy{calc: types.NewSizeCalculator(append(defaultOpts, opts...)...)}
}

type aggregateSizingStrategy struct {
	calc *types.SizeCalculator
}

func (a *aggregateSizingStrategy) TrackSize(ctx TrackContext, value ref.Val) (uint64, bool) {
	if value == nil {
		return 0, false
	}
	return uint64(a.calc.AggregateSize(value)), true
}

func (a *aggregateSizingStrategy) EstimateSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
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
		return estimateAggregateListSize(ctx, node)
	case types.MapKind:
		return estimateAggregateMapSize(ctx, node)
	default:
		return estimateScalarOrFallback(ctx, node)
	}
}

func estimateAggregateListSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	elemType := listElemType(node.Type())
	listSize, elemSize := estimateListExpr(ctx, node, elemType)

	if listSize == nil && ctx != nil && ctx.Estimator() != nil {
		listSize = ctx.Estimator().EstimateSize(node)
	}
	if elemSize == nil && listSize != nil && listSize.Elem != nil {
		elemSize = listSize.Elem
	}
	if elemSize == nil || (elemSize.Elem == nil && isContainerKind(elemType.Kind())) {
		if len(node.Path()) > 0 && ctx != nil {
			elemPath := append(slices.Clone(node.Path()), "@items")
			elemNode := NewAstNode(nil, elemPath, elemType, nil)
			sz := ctx.Size(elemNode)
			if sz != UnknownSizeEstimate() {
				if elemSize == nil {
					elemSize = &sz
				} else {
					elemSize.Elem = sz.Elem
					elemSize.Key = sz.Key
				}
			}
		}
	}
	if elemSize == nil {
		elemSize = fallbackElemSize(ctx, elemType)
	}
	return combineListSize(listSize, elemSize)
}

func estimateAggregateMapSize(ctx EstimateContext, node AstNode) (SizeEstimate, bool) {
	keyType, valType := mapKeyValueTypes(node.Type())
	mapSize, keySize, valSize := estimateMapExpr(ctx, node, keyType, valType)

	if mapSize == nil && ctx != nil && ctx.Estimator() != nil {
		mapSize = ctx.Estimator().EstimateSize(node)
	}
	if keySize == nil && mapSize != nil && mapSize.Key != nil {
		keySize = mapSize.Key
	}
	if valSize == nil && mapSize != nil && mapSize.Elem != nil {
		valSize = mapSize.Elem
	}
	if len(node.Path()) > 0 && ctx != nil {
		if keySize == nil || (keySize.Elem == nil && isContainerKind(keyType.Kind())) {
			kPath := append(slices.Clone(node.Path()), "@keys")
			kSz := ctx.Size(NewAstNode(nil, kPath, keyType, nil))
			if kSz != UnknownSizeEstimate() {
				if keySize == nil {
					keySize = &kSz
				} else {
					keySize.Elem = kSz.Elem
					keySize.Key = kSz.Key
				}
			}
		}
		if valSize == nil || (valSize.Elem == nil && isContainerKind(valType.Kind())) {
			vPath := append(slices.Clone(node.Path()), "@values")
			vSz := ctx.Size(NewAstNode(nil, vPath, valType, nil))
			if vSz != UnknownSizeEstimate() {
				if valSize == nil {
					valSize = &vSz
				} else {
					valSize.Elem = vSz.Elem
					valSize.Key = vSz.Key
				}
			}
		}
	}
	if keySize == nil {
		keySize = fallbackElemSize(ctx, keyType)
	}
	if valSize == nil {
		valSize = fallbackElemSize(ctx, valType)
	}
	return combineMapSize(mapSize, keySize, valSize)
}

func isContainerKind(kind types.Kind) bool {
	return kind == types.ListKind || kind == types.MapKind
}

func fallbackElemSize(ctx EstimateContext, elemType *types.Type) *SizeEstimate {
	if sz := computeTypeSize(elemType); sz != nil {
		return sz
	}
	if ctx != nil && ctx.Estimator() != nil {
		elemNode := NewAstNode(nil, nil, elemType, nil)
		return ctx.Estimator().EstimateSize(elemNode)
	}
	return nil
}

var defaultAggregateSizing SizingStrategy = &aggregateSizingStrategy{calc: types.NewSizeCalculator(types.SizeCalculatorStringUnitLength(1))}
