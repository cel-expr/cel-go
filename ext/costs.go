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

package ext

import (
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types/ref"
)

var (
	callCostEstimate = cost.CallCostEstimate
	callCost         = cost.CallCost
	listAllocCost    = cost.ListAllocCost
	stringCostFactor = cost.StringCostFactor
)

func estimateStringScan(sz cost.SizeEstimate) (cost.CostEstimate, *cost.SizeEstimate) {
	return cost.EstimateStringScan(sz)
}

func estimateListAlloc(sz cost.SizeEstimate, costFactor float64) (cost.CostEstimate, *cost.SizeEstimate) {
	return cost.EstimateListAlloc(sz, costFactor)
}

// estimateTraversal computes cost as a function of the size of the target object and whether the call allocates memory.
func estimateTraversal(nodeSize cost.SizeEstimate, costFactor float64, allocationCost *cost.CostEstimate) (cost.CostEstimate, *cost.SizeEstimate) {
	return cost.EstimateTraversal(nodeSize, costFactor, allocationCost)
}

func estimateSize(estimator cost.Estimator, node cost.AstNode) cost.SizeEstimate {
	return cost.EstimateSize(estimator, node)
}

func actualSize(value ref.Val) uint64 {
	return cost.ActualSize(value)
}

// nodeAsUintValue returns the value of a literal int node as a uint64, or the default value if the
// node is not a non-negative int literal.
func nodeAsUintValue(node cost.AstNode, defaultVal uint64) uint64 {
	return cost.NodeAsUintValue(node, defaultVal)
}

func callEstimate(c cost.CostEstimate, sz *cost.SizeEstimate) *cost.CallEstimate {
	return cost.NewCallEstimate(c, sz)
}

func rangedSizeEstimate(min, max uint64) cost.SizeEstimate {
	return cost.RangedSizeEstimate(min, max)
}

func fixedSizeEstimate(val uint64) cost.SizeEstimate {
	return cost.FixedSizeEstimate(val)
}

func atLeastOne(size cost.SizeEstimate) cost.SizeEstimate {
	return cost.AtLeastOne(size)
}
