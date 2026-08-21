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
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types/ref"
)

// WARNING: Any changes to cost calculations in this file require a corresponding change in estimator.go

// ActualCostEstimator provides function call cost estimations at runtime.
//
// CallCost returns an estimated cost for the function overload invocation with the given args, or nil if it has no
// estimate to provide. CEL attempts to provide reasonable estimates for its standard function library, so CallCost
// should typically not need to provide an estimate for CELs standard function.
type ActualCostEstimator interface {
	CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64
}

// FunctionTracker computes the actual cost of evaluating the functions with the given arguments and result.
type FunctionTracker func(args []ref.Val, result ref.Val) *uint64

// Call represents an invocable function with a name and overload ID.
type Call interface {
	Function() string
	OverloadID() string
}

// TrackerOption configures the behavior of CostTracker objects.
type TrackerOption func(*Tracker) error

// TrackerLimit sets the runtime limit on the evaluation cost during execution and will terminate the expression
// evaluation if the limit is exceeded.
func TrackerLimit(limit uint64) TrackerOption {
	return func(tracker *Tracker) error {
		tracker.Limit = &limit
		return nil
	}
}

// TrackerPresenceTestHasCost determines whether presence testing has a cost of one or zero.
// Defaults to presence test has a cost of one.
func TrackerPresenceTestHasCost(hasCost bool) TrackerOption {
	return func(tracker *Tracker) error {
		tracker.presenceTestHasCost = hasCost
		return nil
	}
}

// TrackerLimitExceededHandler sets a custom handler invoked when the cost limit is exceeded.
func TrackerLimitExceededHandler(handler func()) TrackerOption {
	return func(tracker *Tracker) error {
		tracker.limitExceededHandler = handler
		return nil
	}
}

// OverloadTracker binds an overload ID to a runtime FunctionTracker implementation.
//
// OverloadTracker instances augment or override ActualCostEstimator decisions, allowing for versioned and/or
// optional cost tracking changes.
func OverloadTracker(overloadID string, fnTracker FunctionTracker) TrackerOption {
	return func(tracker *Tracker) error {
		tracker.overloadTrackers[overloadID] = fnTracker
		return nil
	}
}

// LimitExceededError indicates that the actual cost limit was exceeded during evaluation.
type LimitExceededError struct {
	Message string
}

func (e LimitExceededError) Error() string {
	return e.Message
}

// Tracker represents the information needed for tracking runtime cost.
type Tracker struct {
	Estimator            ActualCostEstimator
	overloadTrackers     map[string]FunctionTracker
	Limit                *uint64
	presenceTestHasCost  bool
	limitExceededHandler func()

	cost uint64
}

// NewTracker creates a new Tracker with a given estimator and a set of functional TrackerOption values.
func NewTracker(estimator ActualCostEstimator, opts ...TrackerOption) (*Tracker, error) {
	tracker := &Tracker{
		Estimator:           estimator,
		overloadTrackers:    map[string]FunctionTracker{},
		presenceTestHasCost: true,
	}
	for _, opt := range opts {
		err := opt(tracker)
		if err != nil {
			return nil, err
		}
	}
	return tracker, nil
}

// Clone makes a shallow copy of the tracker.
// The different clones can be used independently from each other.
func (c *Tracker) Clone() (*Tracker, error) {
	tracker := &Tracker{
		Estimator:            c.Estimator,
		overloadTrackers:     c.overloadTrackers,
		Limit:                c.Limit,
		presenceTestHasCost:  c.presenceTestHasCost,
		limitExceededHandler: c.limitExceededHandler,
	}
	return tracker, nil
}

// ActualCost returns the runtime cost.
func (c *Tracker) ActualCost() uint64 {
	return c.cost
}

// PresenceTestHasCost returns whether presence testing has a cost.
func (c *Tracker) PresenceTestHasCost() bool {
	return c.presenceTestHasCost
}

// CreateList records list literal construction cost.
func (c *Tracker) CreateList(id int64, res ref.Val) {
	c.cost = SafeAdd(c.cost, ListCreateBaseCost)
	c.checkLimit()
}

// CreateMap records map literal construction cost.
func (c *Tracker) CreateMap(id int64, res ref.Val) {
	c.cost = SafeAdd(c.cost, MapCreateBaseCost)
	c.checkLimit()
}

// CreateStruct records struct/object construction cost.
func (c *Tracker) CreateStruct(id int64, res ref.Val) {
	c.cost = SafeAdd(c.cost, StructCreateBaseCost)
	c.checkLimit()
}

// EvalAttribute records attribute resolution cost (ident / select).
func (c *Tracker) EvalAttribute(id int64, isTestOnly bool, res ref.Val) {
	if !isTestOnly || c.presenceTestHasCost {
		c.cost = SafeAdd(c.cost, SelectAndIdentCost)
		c.checkLimit()
	}
}

// Qualify records qualifier cost.
func (c *Tracker) Qualify(id int64) {
	c.cost = SafeAdd(c.cost, 1)
	c.checkLimit()
}

// EvalZeroArity records the cost for a 0-arity call expression.
func (c *Tracker) EvalZeroArity(vars any, id int64, call Call, result ref.Val) {
	c.cost = SafeAdd(c.cost, c.CostCall(call, nil, result))
	c.checkLimit()
}

// EvalUnary records the cost for a unary call expression.
func (c *Tracker) EvalUnary(vars any, id int64, call Call, arg ref.Val, result ref.Val) {
	var buf [1]ref.Val
	buf[0] = arg
	c.cost = SafeAdd(c.cost, c.CostCall(call, buf[:], result))
	c.checkLimit()
}

// EvalBinary records the cost for a binary call expression.
func (c *Tracker) EvalBinary(vars any, id int64, call Call, lhs, rhs ref.Val, result ref.Val) {
	var buf [2]ref.Val
	buf[0] = lhs
	buf[1] = rhs
	c.cost = SafeAdd(c.cost, c.CostCall(call, buf[:], result))
	c.checkLimit()
}

// EvalVarArgs records the cost for a variadic call expression.
func (c *Tracker) EvalVarArgs(vars any, id int64, call Call, args []ref.Val, result ref.Val) {
	c.cost = SafeAdd(c.cost, c.CostCall(call, args, result))
	c.checkLimit()
}

func (c *Tracker) checkLimit() {
	if c.Limit != nil && c.cost > *c.Limit {
		if c.limitExceededHandler != nil {
			c.limitExceededHandler()
		}
		panic(LimitExceededError{Message: "operation cancelled: actual cost limit exceeded"})
	}
}

// CostCall calculates the runtime cost for a function call.
func (c *Tracker) CostCall(call Call, args []ref.Val, result ref.Val) uint64 {
	var total uint64
	if len(c.overloadTrackers) != 0 {
		if tracker, found := c.overloadTrackers[call.OverloadID()]; found {
			callCost := tracker(args, result)
			if callCost != nil {
				total = SafeAdd(total, *callCost)
				return total
			}
		}
	}
	if c.Estimator != nil {
		callCost := c.Estimator.CallCost(call.Function(), call.OverloadID(), args, result)
		if callCost != nil {
			total = SafeAdd(total, *callCost)
			return total
		}
	}
	// if user didn't specify, the default way of calculating runtime cost would be used.
	// if user has their own implementation of ActualCostEstimator, make sure to cover the mapping between overloadId and cost calculation
	switch call.OverloadID() {
	// O(n) functions
	case overloads.StartsWithString, overloads.EndsWithString:
		total = SafeAdd(total, SafeMultiplyByFactor(ActualSize(args[1]), StringTraversalCostFactor))
	case overloads.StringToBytes, overloads.BytesToString, overloads.ExtQuoteString, overloads.ExtFormatString:
		total = SafeAdd(total, SafeMultiplyByFactor(ActualSize(args[0]), StringTraversalCostFactor))
	case overloads.InList:
		// If a list is composed entirely of constant values this is O(1), but we don't account for that here.
		// We just assume all list containment checks are O(n).
		total = SafeAdd(total, ActualSize(args[1]))
	// O(min(m, n)) functions
	case overloads.LessString, overloads.GreaterString, overloads.LessEqualsString, overloads.GreaterEqualsString,
		overloads.LessBytes, overloads.GreaterBytes, overloads.LessEqualsBytes, overloads.GreaterEqualsBytes,
		overloads.Equals, overloads.NotEquals:
		// When we check the equality of 2 scalar values (e.g. 2 integers, 2 floating-point numbers, 2 booleans etc.),
		// the CostTracker.ActualSize() function by definition returns 1 for each operand, resulting in an overall cost
		// of 1.
		lhsSize := ActualSize(args[0])
		rhsSize := ActualSize(args[1])
		minSize := min(rhsSize, lhsSize)
		total = SafeAdd(total, SafeMultiplyByFactor(minSize, StringTraversalCostFactor))
	// O(m+n) functions
	case overloads.AddString, overloads.AddBytes:
		// In the worst case scenario, we would need to reallocate a new backing store and copy both operands over.
		argSize := SafeAdd(ActualSize(args[0]), ActualSize(args[1]))
		total = SafeAdd(total, SafeMultiplyByFactor(argSize, StringTraversalCostFactor))
	// O(nm) functions
	case overloads.Matches, overloads.MatchesString:
		// https://swtch.com/~rsc/regexp/regexp1.html applies to RE2 implementation supported by CEL
		// Add one to string length for purposes of cost calculation to prevent product of string and regex to be 0
		// in case where string is empty but regex is still expensive.
		strCost := SafeMultiplyByFactor(SafeAdd(1, ActualSize(args[0])), StringTraversalCostFactor)
		// We don't know how many expressions are in the regex, just the string length (a huge
		// improvement here would be to somehow get a count the number of expressions in the regex or
		// how many states are in the regex state machine and use that to measure regex cost).
		// For now, we're making a guess that each expression in a regex is typically at least 4 chars
		// in length.
		regexCost := SafeMultiplyByFactor(ActualSize(args[1]), RegexStringLengthCostFactor)
		total = SafeAdd(total, SafeMultiply(strCost, regexCost))
	case overloads.ContainsString:
		strCost := SafeMultiplyByFactor(ActualSize(args[0]), StringTraversalCostFactor)
		substrCost := SafeMultiplyByFactor(ActualSize(args[1]), StringTraversalCostFactor)
		total = SafeAdd(total, SafeMultiply(strCost, substrCost))

	default:
		// The following operations are assumed to have O(1) complexity.
		// - AddList due to the implementation. Index lookup can be O(c) the
		//    number of concatenated lists, but we don't track that is cost calculations.
		// - Conversions, since none perform a traversal of a type of unbound length.
		// - Computing the size of strings, byte sequences, lists and maps.
		// - Logical operations and all operators on fixed width scalars (comparisons, equality)
		// - Any functions that don't have a declared cost either here or in provided ActualCostEstimator.
		total = SafeAdd(total, 1)

	}
	return total
}
