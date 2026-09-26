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

import "cel.dev/cel-go/common/overloads"

// StandardOverloadModels defines the cost models for standard CEL functions.
var StandardOverloadModels = []OverloadModel{
	// O(1) container index operations
	Overload(overloads.IndexList,
		EvalCost(Const(1)),
		ResultSize(ArgElem(0)),
	),
	Overload(overloads.IndexMap,
		EvalCost(Const(1)),
		ResultSize(ArgElem(0)),
	),

	// O(n) prefix/suffix functions
	MemberOverload(overloads.StartsWithString,
		EvalCost(Scale(Arg(0), StringTraversalCostFactor)),
	),
	MemberOverload(overloads.EndsWithString,
		EvalCost(Scale(Arg(0), StringTraversalCostFactor)),
	),

	// O(n) conversion & format functions
	Overload(overloads.StringToBytes,
		EvalCost(Scale(Arg(0), StringTraversalCostFactor)),
		ResultSize(Ranged(Arg(0), Scale(Arg(0), 4.0))),
	),
	Overload(overloads.BytesToString,
		EvalCost(Scale(Arg(0), StringTraversalCostFactor)),
		ResultSize(Ranged(Scale(Arg(0), 0.25), Arg(0))),
	),
	Overload(overloads.ExtQuoteString,
		EvalCost(Scale(Arg(0), StringTraversalCostFactor)),
		ResultSize(Ranged(Sum(Arg(0), Const(2)), Sum(Scale(Arg(0), 2.0), Const(2)))),
	),
	MemberOverload(overloads.ExtFormatString,
		EvalCost(Scale(Target(), StringTraversalCostFactor)),
	),

	// O(n) containment
	Overload(overloads.InList, EvalCost(Arg(1))),

	// Short-circuiting optional selection. The planner rewrites these calls into a branch which
	// evaluates the alternative only when the receiver is empty, and which is not charged at
	// runtime, so the model reports no cost for the dispatch itself. The alternative's cost is
	// bounded by calculateArgCost.
	MemberOverload(overloads.OptionalOrOptional, EvalCost(Const(0))),
	MemberOverload(overloads.OptionalOrValueValue, EvalCost(Const(0))),

	// O(min(m, n)) comparison / equality
	Overload(overloads.LessString,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.GreaterString,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.LessEqualsString,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.GreaterEqualsString,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.LessBytes,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.GreaterBytes,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.LessEqualsBytes,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.GreaterEqualsBytes,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.Equals,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),
	Overload(overloads.NotEquals,
		EvalCost(Scale(Min(Arg(0), Arg(1)), StringTraversalCostFactor)),
	),

	// O(m+n) string & bytes concatenation
	Overload(overloads.AddString,
		EvalCost(Scale(Sum(Arg(0), Arg(1)), StringTraversalCostFactor)),
		ResultSize(Sum(Arg(0), Arg(1))),
	),
	Overload(overloads.AddBytes,
		EvalCost(Scale(Sum(Arg(0), Arg(1)), StringTraversalCostFactor)),
		ResultSize(Sum(Arg(0), Arg(1))),
	),

	// O(1) list concatenation with size tracking
	Overload(overloads.AddList,
		EvalCost(Const(1)),
		ResultSize(Sum(Arg(0), Arg(1))),
	),

	// O(nm) regex matches
	Overload(overloads.Matches,
		EvalCost(Mul(
			Scale(Sum(Arg(0), Const(1)), StringTraversalCostFactor),
			Scale(Arg(1), RegexStringLengthCostFactor),
		)),
	),
	MemberOverload(overloads.MatchesString,
		EvalCost(Mul(
			Scale(Sum(Target(), Const(1)), StringTraversalCostFactor),
			Scale(Arg(0), RegexStringLengthCostFactor),
		)),
	),

	// O(nm) substring contains
	MemberOverload(overloads.ContainsString,
		EvalCost(Mul(
			Scale(Target(), StringTraversalCostFactor),
			Scale(Arg(0), StringTraversalCostFactor),
		)),
	),

	// The arg cost for logical and conditional operators is special-cased for
	// short-circuiting (see CalculateArgCost in estimator.go), so the arg cost is 0.

	// Logical short-circuiting operators
	Overload(overloads.LogicalOr, EvalCost(Const(0))),
	Overload(overloads.LogicalAnd, EvalCost(Const(0))),

	// Conditional operator
	Overload(overloads.Conditional,
		EvalCost(Const(0)),
		ResultSize(Union(Arg(1), Arg(2))),
	),
}

// StandardOverloadEstimators returns the map of FunctionEstimator instances for standard overloads.
func StandardOverloadEstimators() map[string]FunctionEstimator {
	return StandardOverloadEstimatorsWithOptions()
}

// StandardOverloadEstimatorsWithOptions returns the map of FunctionEstimator instances for standard
// overloads, configured by the supplied options.
func StandardOverloadEstimatorsWithOptions(opts ...ModelOption) map[string]FunctionEstimator {
	estimators := make(map[string]FunctionEstimator, len(StandardOverloadModels))
	for _, m := range StandardOverloadModels {
		estimators[m.ID] = m.FunctionEstimatorWithOptions(opts...)
	}
	return estimators
}

// StandardOverloadTrackers returns the map of FunctionTracker instances for standard overloads.
func StandardOverloadTrackers() map[string]FunctionTracker {
	return StandardOverloadTrackersWithOptions()
}

// StandardOverloadTrackersWithOptions returns the map of FunctionTracker instances for standard
// overloads, configured by the supplied options.
func StandardOverloadTrackersWithOptions(opts ...ModelOption) map[string]FunctionTracker {
	trackers := make(map[string]FunctionTracker, len(StandardOverloadModels))
	for _, m := range StandardOverloadModels {
		trackers[m.ID] = m.FunctionTrackerWithOptions(opts...)
	}
	return trackers
}
