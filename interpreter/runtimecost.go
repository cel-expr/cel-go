// Copyright 2022 Google LLC
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

package interpreter

import (
	"errors"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// WARNING: Any changes to cost calculations in this file require a corresponding change in checker/cost.go

// ActualCostEstimator provides function call cost estimations at runtime
// CallCost returns an estimated cost for the function overload invocation with the given args, or nil if it has no
// estimate to provide. CEL attempts to provide reasonable estimates for its standard function library, so CallCost
// should typically not need to provide an estimate for CELs standard function.
type ActualCostEstimator interface {
	CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64
}

// costTrackPlanOption modifies the cost tracking factory associatied with the CostObserver
type costTrackPlanOption func(*costTrackerFactory) *costTrackerFactory

// CostTrackerFactory configures the factory method to generate a new cost-tracker per-evaluation.
func CostTrackerFactory(factory func() (*CostTracker, error)) costTrackPlanOption {
	return func(fac *costTrackerFactory) *costTrackerFactory {
		fac.factory = factory
		return fac
	}
}

// CostObserver provides an observer that tracks runtime cost.
func CostObserver(opts ...costTrackPlanOption) PlannerOption {
	ct := &costTrackerFactory{}
	for _, o := range opts {
		ct = o(ct)
	}
	return func(p *planner) (*planner, error) {
		if ct.factory == nil {
			return nil, errors.New("cost tracker factory not configured")
		}
		p.costTrackerFactory = ct.factory
		return p, nil
	}
}

// costTrackerFactory holds a factory for producing new CostTracker instances on each Eval call.
type costTrackerFactory struct {
	factory func() (*CostTracker, error)
}

// InitState produces a CostTracker and bundles it into an Activation in a way which is not visible
// to expression evaluation.
func (ct *costTrackerFactory) InitState(frame *ExecutionFrame) (any, error) {
	if frame.ctx != nil && frame.ctx.costs != nil {
		return frame.ctx.costs, nil
	}
	tracker, err := ct.factory()
	if err != nil {
		return nil, err
	}
	if frame.ctx == nil {
		frame.ctx = evalContextPool.Get().(*evalContext)
	}
	frame.ctx.costs = tracker
	return tracker, nil
}

// GetState extracts the CostTracker from the Activation.
func (ct *costTrackerFactory) GetState(frame *ExecutionFrame) any {
	if frame == nil || frame.ctx == nil {
		return nil
	}
	return frame.ctx.costs
}

// Observe implements the StatefulObserver interface.
func (ct *costTrackerFactory) Observe(vars Activation, id int64, programStep any, val ref.Val) {
}

// CostTrackerOption configures the behavior of CostTracker objects.
type CostTrackerOption func(*CostTracker) error

// CostTrackerLimit sets the runtime limit on the evaluation cost during execution and will terminate the expression
// evaluation if the limit is exceeded.
func CostTrackerLimit(limit uint64) CostTrackerOption {
	return func(tracker *CostTracker) error {
		tracker.Limit = &limit
		return nil
	}
}

// PresenceTestHasCost determines whether presence testing has a cost of one or zero.
// Defaults to presence test has a cost of one.
func PresenceTestHasCost(hasCost bool) CostTrackerOption {
	return func(tracker *CostTracker) error {
		tracker.presenceTestHasCost = hasCost
		return nil
	}
}

// NewCostTracker creates a new CostTracker with a given estimator and a set of functional CostTrackerOption values.
func NewCostTracker(estimator ActualCostEstimator, opts ...CostTrackerOption) (*CostTracker, error) {
	tracker := &CostTracker{
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

// OverloadCostTracker binds an overload ID to a runtime FunctionTracker implementation.
//
// OverloadCostTracker instances augment or override ActualCostEstimator decisions, allowing for  versioned and/or
// optional cost tracking changes.
func OverloadCostTracker(overloadID string, fnTracker FunctionTracker) CostTrackerOption {
	return func(tracker *CostTracker) error {
		tracker.overloadTrackers[overloadID] = fnTracker
		return nil
	}
}

// FunctionTracker computes the actual cost of evaluating the functions with the given arguments and result.
type FunctionTracker func(args []ref.Val, result ref.Val) *uint64

// CostTracker represents the information needed for tracking runtime cost.
type CostTracker struct {
	Estimator           ActualCostEstimator
	overloadTrackers    map[string]FunctionTracker
	Limit               *uint64
	presenceTestHasCost bool

	cost uint64
}

// Clone makes a shallow copy of the tracker.
// The different clones can be used independently from
// each other.
func (c *CostTracker) Clone() (*CostTracker, error) {
	tracker := &CostTracker{
		Estimator:           c.Estimator,
		overloadTrackers:    c.overloadTrackers,
		Limit:               c.Limit,
		presenceTestHasCost: c.presenceTestHasCost,
	}
	return tracker, nil
}

// ActualCost returns the runtime cost
func (c *CostTracker) ActualCost() uint64 {
	return c.cost
}

// CreateList records list literal construction cost.
func (c *CostTracker) CreateList(id int64, res ref.Val) {
	c.cost = cost.SafeAdd(c.cost, common.ListCreateBaseCost)
	c.checkLimit()
}

// CreateMap records map literal construction cost.
func (c *CostTracker) CreateMap(id int64, res ref.Val) {
	c.cost = cost.SafeAdd(c.cost, common.MapCreateBaseCost)
	c.checkLimit()
}

// CreateStruct records struct/object construction cost.
func (c *CostTracker) CreateStruct(id int64, res ref.Val) {
	c.cost = cost.SafeAdd(c.cost, common.StructCreateBaseCost)
	c.checkLimit()
}

// EvalAttribute records attribute resolution cost (ident / select).
func (c *CostTracker) EvalAttribute(id int64, isTestOnly bool, res ref.Val) {
	if !isTestOnly || c.presenceTestHasCost {
		c.cost = cost.SafeAdd(c.cost, common.SelectAndIdentCost)
		c.checkLimit()
	}
}

// Qualify records qualifier cost.
func (c *CostTracker) Qualify(id int64) {
	c.cost = cost.SafeAdd(c.cost, 1)
	c.checkLimit()
}

type costTrackingInterpretable struct {
	InterpretableV2
	factory func() (*CostTracker, error)
}

func (c *costTrackingInterpretable) Exec(frame *ExecutionFrame) ref.Val {
	if frame.CostTracker() == nil {
		tracker, err := c.factory()
		if err != nil {
			return types.NewErr("cost tracker factory: %v", err)
		}
		frame.SetCostTracker(tracker)
	}
	return c.InterpretableV2.Exec(frame)
}

func (c *costTrackingInterpretable) Eval(ctx Activation) ref.Val {
	return c.Exec(AsFrame(ctx))
}

// EvalZeroArity records the cost for a 0-arity call expression.
func (c *CostTracker) EvalZeroArity(vars Activation, id int64, call InterpretableCall, result ref.Val) {
	c.cost = cost.SafeAdd(c.cost, c.costCall(call, nil, result))
	c.checkLimit()
}

// EvalUnary records the cost for a unary call expression.
func (c *CostTracker) EvalUnary(vars Activation, id int64, call InterpretableCall, arg ref.Val, result ref.Val) {
	var buf [1]ref.Val
	buf[0] = arg
	c.cost = cost.SafeAdd(c.cost, c.costCall(call, buf[:], result))
	c.checkLimit()
}

// EvalBinary records the cost for a binary call expression.
func (c *CostTracker) EvalBinary(vars Activation, id int64, call InterpretableCall, lhs, rhs ref.Val, result ref.Val) {
	var buf [2]ref.Val
	buf[0] = lhs
	buf[1] = rhs
	c.cost = cost.SafeAdd(c.cost, c.costCall(call, buf[:], result))
	c.checkLimit()
}

// EvalVarArgs records the cost for a variadic call expression.
func (c *CostTracker) EvalVarArgs(vars Activation, id int64, call InterpretableCall, args []ref.Val, result ref.Val) {
	c.cost = cost.SafeAdd(c.cost, c.costCall(call, args, result))
	c.checkLimit()
}

func (c *CostTracker) checkLimit() {
	if c.Limit != nil && c.cost > *c.Limit {
		panic(EvalCancelledError{Cause: CostLimitExceeded, Message: "operation cancelled: actual cost limit exceeded"})
	}
}

func (c *CostTracker) costCall(call InterpretableCall, args []ref.Val, result ref.Val) uint64 {
	var total uint64
	if len(c.overloadTrackers) != 0 {
		if tracker, found := c.overloadTrackers[call.OverloadID()]; found {
			callCost := tracker(args, result)
			if callCost != nil {
				total = cost.SafeAdd(total, *callCost)
				return total
			}
		}
	}
	if c.Estimator != nil {
		callCost := c.Estimator.CallCost(call.Function(), call.OverloadID(), args, result)
		if callCost != nil {
			total = cost.SafeAdd(total, *callCost)
			return total
		}
	}
	// if user didn't specify, the default way of calculating runtime cost would be used.
	// if user has their own implementation of ActualCostEstimator, make sure to cover the mapping between overloadId and cost calculation
	switch call.OverloadID() {
	// O(n) functions
	case overloads.StartsWithString, overloads.EndsWithString:
		total = cost.SafeAdd(total, cost.SafeMultiplyByFactor(actualSize(args[1]), common.StringTraversalCostFactor))
	case overloads.StringToBytes, overloads.BytesToString, overloads.ExtQuoteString, overloads.ExtFormatString:
		total = cost.SafeAdd(total, cost.SafeMultiplyByFactor(actualSize(args[0]), common.StringTraversalCostFactor))
	case overloads.InList:
		// If a list is composed entirely of constant values this is O(1), but we don't account for that here.
		// We just assume all list containment checks are O(n).
		total = cost.SafeAdd(total, actualSize(args[1]))
	// O(min(m, n)) functions
	case overloads.LessString, overloads.GreaterString, overloads.LessEqualsString, overloads.GreaterEqualsString,
		overloads.LessBytes, overloads.GreaterBytes, overloads.LessEqualsBytes, overloads.GreaterEqualsBytes,
		overloads.Equals, overloads.NotEquals:
		// When we check the equality of 2 scalar values (e.g. 2 integers, 2 floating-point numbers, 2 booleans etc.),
		// the CostTracker.ActualSize() function by definition returns 1 for each operand, resulting in an overall cost
		// of 1.
		lhsSize := actualSize(args[0])
		rhsSize := actualSize(args[1])
		minSize := min(rhsSize, lhsSize)
		total = cost.SafeAdd(total, cost.SafeMultiplyByFactor(minSize, common.StringTraversalCostFactor))
	// O(m+n) functions
	case overloads.AddString, overloads.AddBytes:
		// In the worst case scenario, we would need to reallocate a new backing store and copy both operands over.
		argSize := cost.SafeAdd(actualSize(args[0]), actualSize(args[1]))
		total = cost.SafeAdd(total, cost.SafeMultiplyByFactor(argSize, common.StringTraversalCostFactor))
	// O(nm) functions
	case overloads.Matches, overloads.MatchesString:
		// https://swtch.com/~rsc/regexp/regexp1.html applies to RE2 implementation supported by CEL
		// Add one to string length for purposes of cost calculation to prevent product of string and regex to be 0
		// in case where string is empty but regex is still expensive.
		strCost := cost.SafeMultiplyByFactor(cost.SafeAdd(1, actualSize(args[0])), common.StringTraversalCostFactor)
		// We don't know how many expressions are in the regex, just the string length (a huge
		// improvement here would be to somehow get a count the number of expressions in the regex or
		// how many states are in the regex state machine and use that to measure regex cost).
		// For now, we're making a guess that each expression in a regex is typically at least 4 chars
		// in length.
		regexCost := cost.SafeMultiplyByFactor(actualSize(args[1]), common.RegexStringLengthCostFactor)
		total = cost.SafeAdd(total, cost.SafeMultiply(strCost, regexCost))
	case overloads.ContainsString:
		strCost := cost.SafeMultiplyByFactor(actualSize(args[0]), common.StringTraversalCostFactor)
		substrCost := cost.SafeMultiplyByFactor(actualSize(args[1]), common.StringTraversalCostFactor)
		total = cost.SafeAdd(total, cost.SafeMultiply(strCost, substrCost))

	default:
		// The following operations are assumed to have O(1) complexity.
		// - AddList due to the implementation. Index lookup can be O(c) the
		//    number of concatenated lists, but we don't track that is cost calculations.
		// - Conversions, since none perform a traversal of a type of unbound length.
		// - Computing the size of strings, byte sequences, lists and maps.
		// - Logical operations and all operators on fixed width scalars (comparisons, equality)
		// - Any functions that don't have a declared cost either here or in provided ActualCostEstimator.
		total = cost.SafeAdd(total, 1)

	}
	return total
}

// actualSize returns the size of the value for all traits.Sizer values, a fixed size for all proto-based
// objects, and a size of 1 for all other value types.
func actualSize(value ref.Val) uint64 {
	if sz, ok := value.(traits.Sizer); ok {
		return uint64(sz.Size().(types.Int))
	}
	if opt, ok := value.(*types.Optional); ok && opt.HasValue() {
		return actualSize(opt.GetValue())
	}
	return 1
}
