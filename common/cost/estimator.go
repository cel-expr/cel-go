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

package cost

import (
	"math"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
)

// Estimator provides CallCost and Size estimates for cost calculation.
type Estimator interface {

	// EstimateCallCost returns the CallEstimate for a function, overload and target and arguments.
	//
	// If nil is returned, the cost of the function is calculated by the OverloadCostEstimate if
	// provided, or standard CEL function cost estimation.
	EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate

	// EstimateSize returns the SizeEstimate of an AstNode.
	//
	// If nil is returned, the size of the node is calculated by the SizingStrategy.
	EstimateSize(element AstNode) *SizeEstimate
}

// AstNode represents an AST node for estimating cost.
type AstNode interface {
	// Path returns a field path through the provided type declarations to the type of the AstNode, or nil if the AstNode does not
	// represent type directly reachable from the provided type declarations.
	// The first path element is a variable. All subsequent path elements are one of: field name, '@items', '@keys', '@values'.
	Path() []string

	// Type returns the deduced type of the AstNode.
	Type() *types.Type

	// Expr returns the expression of the AstNode.
	Expr() ast.Expr

	// ComputedSize returns a size estimate of the AstNode derived from information available in the CEL expression.
	// For constants and inline list and map declarations, the exact size is returned. For concatenated list, strings
	// and bytes, the size is derived from the size estimates of the operands. nil is returned if there is no
	// computed size available.
	ComputedSize() *SizeEstimate
}

// astNode implements the AstNode interface.
type astNode struct {
	path        []string
	t           *types.Type
	expr        ast.Expr
	derivedSize *SizeEstimate
}

// Path returns the field path to the node, if known.
func (e astNode) Path() []string {
	return e.path
}

// Type returns the type of the node.
func (e astNode) Type() *types.Type {
	return e.t
}

// Expr returns the underlying CEL expression.
func (e astNode) Expr() ast.Expr {
	return e.expr
}

// ComputedSize returns the precomputed or derived size estimate of the node.
func (e astNode) ComputedSize() *SizeEstimate {
	return e.derivedSize
}

// NewAstNode returns an AstNode with the given expression, path, type, and derived size.
func NewAstNode(expr ast.Expr, path []string, t *types.Type, derivedSize *SizeEstimate) AstNode {
	return astNode{
		path:        path,
		t:           t,
		expr:        expr,
		derivedSize: derivedSize,
	}
}

// Option configures flags which affect cost computations.
type Option func(*coster) error

// CostOption is an alias for Option.
//
// Deprecated: use Option.
type CostOption = Option

// PresenceTestHasCost determines whether presence testing has a cost of one or zero.
//
// Defaults to presence test has a cost of one.
func PresenceTestHasCost(hasCost bool) Option {
	return func(c *coster) error {
		if hasCost {
			c.presenceTestCost = selectAndIdentCost
			return nil
		}
		c.presenceTestCost = FixedCostEstimate(0)
		return nil
	}
}

// FunctionEstimator provides a CallEstimate given the target and arguments for a specific function, overload pair.
type FunctionEstimator func(estimator Estimator, target *AstNode, args []AstNode) *CallEstimate

// OverloadCostEstimate binds a FunctionEstimator to a specific function overload ID.
//
// When a OverloadCostEstimate is provided, it will override the cost calculation of the CostEstimator provided to
// the Cost() call.
func OverloadCostEstimate(overloadID string, functionCoster FunctionEstimator) Option {
	return func(c *coster) error {
		c.overloadEstimators[overloadID] = functionCoster
		return nil
	}
}

// EstimateSizingStrategy configures a custom SizingStrategy for cost estimation.
func EstimateSizingStrategy(strategy SizingStrategy) Option {
	return func(c *coster) error {
		if strategy == nil {
			strategy = DefaultSizingStrategy()
		}
		c.sizingStrategy = strategy
		return nil
	}
}

// EstimateModelVersion pins cost estimation to a revision of the cost model's rules.
//
// Defaults to defaultModelVersion. Pin only to hold estimates stable against a previously recorded
// baseline; see ModelVersion for why an older revision is always the less accurate choice.
func EstimateModelVersion(version uint32) Option {
	return func(c *coster) error {
		c.modelVersion = version
		return nil
	}
}

// Cost estimates the cost of the parsed and type checked CEL expression.
func Cost(checked *ast.AST, estimator Estimator, opts ...Option) (CostEstimate, error) {
	c := &coster{
		checkedAST:         checked,
		estimator:          estimator,
		overloadEstimators: map[string]FunctionEstimator{},
		exprPaths:          map[int64][]string{},
		localVars:          make(scopes),
		computedSizes:      map[int64]SizeEstimate{},
		presenceTestCost:   FixedCostEstimate(1),
		modelVersion:       defaultModelVersion,
	}
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return CostEstimate{}, err
		}
	}
	if c.sizingStrategy == nil {
		c.sizingStrategy = DefaultSizingStrategy()
	}
	// The package-level estimator cache is built with the default strategy at the latest revision,
	// so it can only be reused when both still hold.
	if c.sizingStrategy != defaultSizing || c.modelVersion != defaultModelVersion {
		c.sizingOverloadEstimators = StandardOverloadEstimatorsWithOptions(
			WithSizingStrategy(c.sizingStrategy),
			ModelVersion(c.modelVersion))
	}
	return c.cost(checked.Expr()), nil
}

// coster performs recursive cost and size calculation on checked AST expressions.
type coster struct {
	// exprPaths maps from Expr Id to field path.
	exprPaths map[int64][]string
	// localVars tracks the local and iteration variables assigned during evaluation.
	localVars scopes
	// computedSizes tracks the computed sizes of call results.
	computedSizes map[int64]SizeEstimate

	checkedAST               *ast.AST
	estimator                Estimator
	sizingStrategy           SizingStrategy
	overloadEstimators       map[string]FunctionEstimator
	sizingOverloadEstimators map[string]FunctionEstimator
	// presenceTestCost will either be a zero or one based on whether has() macros count against cost computations.
	presenceTestCost CostEstimate
	// modelVersion selects the revision of the estimation rules to apply.
	modelVersion uint32
}

// localVar captures the local variable size estimates if they exist for variables
type localVar struct {
	exprID int64
	path   []string
	size   *SizeEstimate
}

// scopes is a stack of variable name to integer id stack to handle scopes created by cel.bind() like macros
type scopes map[string][]*localVar

// push adds a variable name to the scope stack with its path and size estimates.
func (s scopes) push(varName string, expr ast.Expr, path []string, size *SizeEstimate) {
	s[varName] = append(s[varName], &localVar{
		exprID: expr.ID(),
		path:   path,
		size:   size,
	})
}

// pop removes the most recent variable from the scope stack for the given name.
func (s scopes) pop(varName string) {
	varStack := s[varName]
	s[varName] = varStack[:len(varStack)-1]
}

// peek returns the top of the stack for the given variable name.
func (s scopes) peek(varName string) (*localVar, bool) {
	varStack := s[varName]
	if len(varStack) > 0 {
		return varStack[len(varStack)-1], true
	}
	return nil, false
}

// pushIterKey pushes the iteration key or index variable for a comprehension onto the scope stack.
func (c *coster) pushIterKey(varName string, rangeExpr ast.Expr) {
	rangeSize := c.sizeOrUnknown(rangeExpr)
	var size *SizeEstimate
	if rangeSize.Key != nil {
		size = rangeSize.Key
	} else {
		s := FixedSizeEstimate(1)
		size = &s
	}
	path := c.getPath(rangeExpr)
	container := c.getType(rangeExpr).Kind()
	subpath := "@keys"
	if container == types.ListKind {
		subpath = "@indices"
	}
	c.localVars.push(varName, rangeExpr, append(path, subpath), size)
}

// pushIterValue pushes the iteration value variable for a comprehension onto the scope stack.
func (c *coster) pushIterValue(varName string, rangeExpr ast.Expr) {
	rangeSize := c.sizeOrUnknown(rangeExpr)
	var size *SizeEstimate
	if rangeSize.Elem != nil {
		size = rangeSize.Elem
	}
	path := c.getPath(rangeExpr)
	container := c.getType(rangeExpr).Kind()
	subpath := "@values"
	if container == types.ListKind {
		subpath = "@items"
	}
	c.localVars.push(varName, rangeExpr, append(path, subpath), size)
}

// pushIterSingle pushes a single iteration variable (items for list, keys for map) onto the scope stack.
func (c *coster) pushIterSingle(varName string, rangeExpr ast.Expr) {
	rangeSize := c.sizeOrUnknown(rangeExpr)
	var size *SizeEstimate
	subpath := "@keys"
	container := c.getType(rangeExpr).Kind()
	if container == types.ListKind {
		size = rangeSize.Elem
		subpath = "@items"
	} else {
		if rangeSize.Key != nil {
			size = rangeSize.Key
		} else {
			s := FixedSizeEstimate(1)
			size = &s
		}
	}
	path := c.getPath(rangeExpr)
	c.localVars.push(varName, rangeExpr, append(path, subpath), size)
}

// pushLocalVar records a local variable binding with its path and size estimates.
func (c *coster) pushLocalVar(varName string, e ast.Expr) {
	path := c.getPath(e)
	c.localVars.push(varName, e, path, c.computeSize(e))
}

// peekLocalVar looks up the top of the scope stack for a local variable name.
func (c *coster) peekLocalVar(varName string) (*localVar, bool) {
	return c.localVars.peek(varName)
}

// popLocalVar pops the scope stack for a local variable name.
func (c *coster) popLocalVar(varName string) {
	c.localVars.pop(varName)
}

// cost recursively calculates the cost estimate for an expression node.
func (c *coster) cost(e ast.Expr) CostEstimate {
	if e == nil {
		return CostEstimate{}
	}
	switch e.Kind() {
	case ast.LiteralKind:
		return constCost
	case ast.IdentKind:
		return c.costIdent(e)
	case ast.SelectKind:
		return c.costSelect(e)
	case ast.CallKind:
		return c.costCall(e)
	case ast.ListKind:
		return c.costCreateList(e)
	case ast.MapKind:
		return c.costCreateMap(e)
	case ast.StructKind:
		return c.costCreateStruct(e)
	case ast.ComprehensionKind:
		if c.isBind(e) {
			return c.costBind(e)
		}
		return c.costComprehension(e)
	default:
		return CostEstimate{}
	}
}

// costIdent estimates the cost of evaluating an identifier and tracks its path.
func (c *coster) costIdent(e ast.Expr) CostEstimate {
	ident := e.AsIdent()
	if v, ok := c.peekLocalVar(ident); ok {
		c.addPath(e, v.path)
	} else {
		c.addPath(e, []string{ident})
	}
	return selectAndIdentCost
}

// costSelect estimates the cost of a field selection or map key access.
func (c *coster) costSelect(e ast.Expr) CostEstimate {
	sel := e.AsSelect()
	var sum CostEstimate
	if sel.IsTestOnly() {
		sum = sum.Add(c.presenceTestCost)
	} else {
		sum = sum.Add(selectAndIdentCost)
	}
	sum = sum.Add(c.cost(sel.Operand()))
	sum = sum.Add(c.relativeAttributeCost(sel.Operand()))
	targetPath := c.getPath(sel.Operand())
	if len(targetPath) > 0 {
		c.addPath(e, append(targetPath, sel.FieldName()))
	}
	return sum
}

// relativeAttributeCost is the cost of qualifying a value which was computed rather than named.
func (c *coster) relativeAttributeCost(operand ast.Expr) CostEstimate {
	if isAttributeChain(operand) {
		return CostEstimate{}
	}
	return FixedCostEstimate(SelectAndIdentCost)
}

// isAttributeChain reports whether an expression is resolved as part of a single attribute
// during evaluation. A chain begins at an identifier, or at a ternary which selects between
// attributes, and is extended by field selections and index operations, including their
// optional variants.
//
// This predicate must mirror the planner's decision to wrap an operand in a relative
// attribute: any expression which plans to an interpretable attribute extends the chain and
// does not incur an additional attribute resolution cost.
func isAttributeChain(e ast.Expr) bool {
	switch e.Kind() {
	case ast.IdentKind, ast.SelectKind:
		return true
	case ast.CallKind:
		switch e.AsCall().FunctionName() {
		case operators.Index, operators.OptIndex, operators.OptSelect, operators.Conditional:
			return true
		}
	}
	return false
}

// costCall estimates the cost of evaluating a function call expression.
func (c *coster) costCall(e ast.Expr) CostEstimate {
	// Dyn is just a way to disable type-checking, so return the cost of 1 with the cost of the argument
	if dynEstimate := c.maybeUnwrapDynCall(e); dynEstimate != nil {
		return *dynEstimate
	}

	// Continue estimating the cost of all other calls.
	call := e.AsCall()
	args := call.Args()
	var sum CostEstimate

	// Index-like operators qualify their first argument, which requires a relative attribute
	// when the operand is a computed value rather than a named one.
	switch call.FunctionName() {
	case operators.Index, operators.OptIndex, operators.OptSelect:
		if len(args) > 0 {
			sum = sum.Add(c.relativeAttributeCost(args[0]))
		}
	}

	argTypes := make([]AstNode, len(args))
	argCosts := make([]CostEstimate, len(args))
	for i, arg := range args {
		argCosts[i] = c.cost(arg)
		argTypes[i] = c.newAstNode(arg)
	}

	overloadIDs := c.checkedAST.GetOverloadIDs(e.ID())
	if len(overloadIDs) == 0 {
		var argCostSum CostEstimate
		for _, a := range argCosts {
			argCostSum = argCostSum.Add(a)
		}
		return FixedCostEstimate(1).Add(sum).Add(argCostSum)
	}
	var targetType *AstNode
	if call.IsMemberFunction() {
		sum = sum.Add(c.cost(call.Target()))
		var t AstNode = c.newAstNode(call.Target())
		targetType = &t
	}
	// Pick a cost estimate range that covers all the overload cost estimation ranges
	fnCost := CostEstimate{Min: uint64(math.MaxUint64), Max: 0}
	var resultSize *SizeEstimate
	for _, overload := range overloadIDs {
		overloadCost := c.functionCost(e, call.FunctionName(), overload, targetType, argTypes, argCosts)
		fnCost = fnCost.Union(overloadCost.CostEstimate)
		resultSize = mergeSizeEstimatePtr(resultSize, overloadCost.ResultSize)
		// build and track the field path for index operations
		switch overload {
		case overloads.IndexList:
			if len(args) > 0 {
				c.addPath(e, append(c.getPath(args[0]), "@items"))
			}
		case overloads.IndexMap:
			if len(args) > 0 {
				c.addPath(e, append(c.getPath(args[0]), "@values"))
			}
		}
		if resultSize == nil {
			resultSize = c.computeSize(e)
		}
	}
	c.setSize(e, resultSize)
	return sum.Add(fnCost)
}

// maybeUnwrapDynCall handles the 'dyn' call wrapper, returning an estimate if matched.
func (c *coster) maybeUnwrapDynCall(e ast.Expr) *CostEstimate {
	call := e.AsCall()
	if call.FunctionName() != "dyn" {
		return nil
	}
	arg := call.Args()[0]
	argCost := c.cost(arg)
	c.copySizeEstimates(e, arg)
	callCost := FixedCostEstimate(1).Add(argCost)
	return &callCost
}

// costCreateList estimates the cost of constructing a list literal.
func (c *coster) costCreateList(e ast.Expr) CostEstimate {
	create := e.AsList()
	var sum CostEstimate
	for _, elem := range create.Elements() {
		sum = sum.Add(c.cost(elem))
	}
	return sum.Add(createListBaseCost)
}

// costCreateMap estimates the cost of constructing a map literal.
func (c *coster) costCreateMap(e ast.Expr) CostEstimate {
	mapVal := e.AsMap()
	var sum CostEstimate
	for _, ent := range mapVal.Entries() {
		entry := ent.AsMapEntry()
		sum = sum.Add(c.cost(entry.Key()))
		sum = sum.Add(c.cost(entry.Value()))
	}
	return sum.Add(createMapBaseCost)
}

// costCreateStruct estimates the cost of constructing a struct or message literal.
func (c *coster) costCreateStruct(e ast.Expr) CostEstimate {
	msgVal := e.AsStruct()
	var sum CostEstimate
	for _, ent := range msgVal.Fields() {
		field := ent.AsStructField()
		sum = sum.Add(c.cost(field.Value()))
	}
	return sum.Add(createMessageBaseCost)
}

// costComprehension estimates the cost of evaluating a comprehension loop.
func (c *coster) costComprehension(e ast.Expr) CostEstimate {
	comp := e.AsComprehension()
	var sum CostEstimate
	sum = sum.Add(c.cost(comp.IterRange()))
	sum = sum.Add(c.cost(comp.AccuInit()))
	c.pushLocalVar(comp.AccuVar(), comp.AccuInit())

	// Track the iterRange of each IterVar and AccuVar for field path construction
	if comp.HasIterVar2() {
		c.pushIterKey(comp.IterVar(), comp.IterRange())
		c.pushIterValue(comp.IterVar2(), comp.IterRange())
	} else {
		c.pushIterSingle(comp.IterVar(), comp.IterRange())
	}

	// Determine the cost for each element in the loop
	loopCost := c.cost(comp.LoopCondition())
	stepCost := c.cost(comp.LoopStep())

	// Clear the intermediate variable tracking.
	c.popLocalVar(comp.IterVar())
	if comp.HasIterVar2() {
		c.popLocalVar(comp.IterVar2())
	}

	// Determine the result cost.
	sum = sum.Add(c.cost(comp.Result()))
	c.localVars.pop(comp.AccuVar())

	// Estimate the cost of the loop.
	rangeCnt := c.sizeOrUnknown(comp.IterRange())
	rangeCost := rangeCnt.MultiplyByCost(stepCost.Add(loopCost))
	sum = sum.Add(rangeCost)

	switch k := comp.AccuInit().Kind(); k {
	case ast.LiteralKind:
		c.setSize(e, c.computeSize(comp.AccuInit()))
	case ast.ListKind, ast.MapKind:
		accuSize := rangeCnt
		if stepSize := c.computeSize(comp.LoopStep()); stepSize != nil {
			if stepSize.Elem != nil {
				accuSize.Elem = stepSize.Elem
			}
			if stepSize.Key != nil {
				accuSize.Key = stepSize.Key
			}
		}
		c.setSize(e, &accuSize)
	}
	return sum
}

// isBind returns true if the given expression represents a cel.bind() macro structure.
func (c *coster) isBind(e ast.Expr) bool {
	comp := e.AsComprehension()
	iterRange := comp.IterRange()
	loopCond := comp.LoopCondition()
	return iterRange.Kind() == ast.ListKind && iterRange.AsList().Size() == 0 &&
		loopCond.Kind() == ast.LiteralKind && loopCond.AsLiteral() == types.False &&
		!isAccumulatorVar(comp.AccuVar())
}

// costBind estimates the cost of a cel.bind() variable declaration macro.
func (c *coster) costBind(e ast.Expr) CostEstimate {
	comp := e.AsComprehension()
	var sum CostEstimate
	// Binds are lazily initialized, so we retain the cost of an empty iteration range.
	sum = sum.Add(c.cost(comp.IterRange()))
	sum = sum.Add(c.cost(comp.AccuInit()))

	c.pushLocalVar(comp.AccuVar(), comp.AccuInit())
	sum = sum.Add(c.cost(comp.Result()))
	c.popLocalVar(comp.AccuVar())

	// Associate the bind output size with the result size.
	c.copySizeEstimates(e, comp.Result())
	return sum
}

func calculateArgCost(overloadID string, argCosts []CostEstimate) CostEstimate {
	switch overloadID {
	case overloads.LogicalOr, overloads.LogicalAnd:
		if len(argCosts) == 2 {
			return CostEstimate{Min: argCosts[0].Min, Max: argCosts[0].Add(argCosts[1]).Max}
		}
	case overloads.Conditional:
		if len(argCosts) == 3 {
			return argCosts[0].Add(argCosts[1].Union(argCosts[2]))
		}
	case overloads.OptionalOrOptional, overloads.OptionalOrValueValue:
		// The alternative is evaluated only when the receiver is empty, so it contributes to the
		// upper bound alone. These are member overloads, which means the receiver is costed by
		// the caller and argCosts holds only the alternative.
		if len(argCosts) == 1 {
			return CostEstimate{Min: 0, Max: argCosts[0].Max}
		}
	}
	var sum CostEstimate
	for _, a := range argCosts {
		sum = sum.Add(a)
	}
	return sum
}

func (c *coster) functionCost(e ast.Expr, function, overloadID string, target *AstNode, args []AstNode, argCosts []CostEstimate) CallEstimate {
	argCost := calculateArgCost(overloadID, argCosts)
	estCtx := c.getEstimator()
	if len(c.overloadEstimators) != 0 {
		if estimator, found := c.overloadEstimators[overloadID]; found {
			if est := estimator(estCtx, target, args); est != nil {
				return CallEstimate{CostEstimate: est.Add(argCost), ResultSize: est.ResultSize}
			}
		}
	}
	if est := estCtx.EstimateCallCost(function, overloadID, target, args); est != nil {
		return CallEstimate{CostEstimate: est.Add(argCost), ResultSize: est.ResultSize}
	}
	if estimator, found := c.getStandardOverloadEstimators()[overloadID]; found {
		if est := estimator(estCtx, target, args); est != nil {
			return CallEstimate{CostEstimate: est.Add(argCost), ResultSize: est.ResultSize}
		}
	}
	// O(1) functions
	// See CostTracker.costCall for more details about O(1) cost calculations

	// Benchmarks suggest that most of the other operations take +/- 50% of a base cost unit
	// which on an Intel xeon 2.20GHz CPU is 50ns.
	return CallEstimate{CostEstimate: FixedCostEstimate(1).Add(argCost)}
}

// getType returns the deduced type of an expression ID from the checked AST.
func (c *coster) getType(e ast.Expr) *types.Type {
	return c.checkedAST.GetType(e.ID())
}

// getPath returns the tracked field path for an expression node, resolving through local variables if needed.
func (c *coster) getPath(e ast.Expr) []string {
	if e == nil {
		return nil
	}
	if e.Kind() == ast.IdentKind {
		if v, found := c.peekLocalVar(e.AsIdent()); found {
			return v.path[:]
		}
	}
	if path, ok := c.exprPaths[e.ID()]; ok {
		return path
	}
	return nil
}

func (c *coster) getStandardOverloadEstimators() map[string]FunctionEstimator {
	if c.sizingOverloadEstimators != nil {
		return c.sizingOverloadEstimators
	}
	return stdOverloadEstimators
}

// addPath associates an expression ID with its path.
func (c *coster) addPath(e ast.Expr, path []string) {
	c.exprPaths[e.ID()] = path
}

var (
	stdOverloadEstimators = StandardOverloadEstimators()
)

func isAccumulatorVar(name string) bool {
	return name == accumulatorName || name == hiddenAccumulatorName
}

func (c *coster) newAstNode(e ast.Expr) AstNode {
	return astNode{
		path:        c.getPath(e),
		t:           c.getType(e),
		expr:        e,
		derivedSize: c.computeSize(e)}
}

// setSize stores a computed size estimate for an expression ID.
func (c *coster) setSize(e ast.Expr, size *SizeEstimate) {
	if size == nil {
		return
	}
	// Store the computed size with the expression
	c.computedSizes[e.ID()] = *size
}

func (c *coster) sizeOrUnknown(node ast.Expr) SizeEstimate {
	if sz := c.computeSize(node); sz != nil {
		return *sz
	}
	return UnknownSizeEstimate()
}

// copySizeEstimates copies computed sizes and entry sizes from a source expression to a destination expression.
func (c *coster) copySizeEstimates(dst, src ast.Expr) {
	c.setSize(dst, c.computeSize(src))
}

func (c *coster) getSizingStrategy() SizingStrategy {
	if c.sizingStrategy != nil {
		return c.sizingStrategy
	}
	return defaultSizing
}

type versionedEstimator struct {
	delegate Estimator
	version  uint32
}

func (v versionedEstimator) EstimateSize(element AstNode) *SizeEstimate {
	if v.delegate == nil {
		return nil
	}
	return v.delegate.EstimateSize(element)
}

func (v versionedEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	if v.delegate == nil {
		return nil
	}
	return v.delegate.EstimateCallCost(function, overloadID, target, args)
}

func (v versionedEstimator) modelVersion() uint32 {
	return v.version
}

func (c *coster) getEstimator() Estimator {
	return versionedEstimator{delegate: c.estimator, version: c.modelVersion}
}

func (c *coster) newEstimateContext() *estimatorEvalContext {
	return &estimatorEvalContext{
		coster:    c,
		estimator: c.estimator,
		strategy:  c.getSizingStrategy(),
		version:   c.modelVersion,
	}
}

// computeSize resolves the size estimate for an expression, caching the result when found.
func (c *coster) computeSize(e ast.Expr) *SizeEstimate {
	if size, ok := c.computedSizes[e.ID()]; ok {
		return &size
	}
	if size := computeExprSize(e); size != nil {
		return size
	}
	if e.Kind() == ast.IdentKind {
		varName := e.AsIdent()
		if v, ok := c.peekLocalVar(varName); ok && v.size != nil {
			return v.size
		}
	}
	node := astNode{expr: e, path: c.getPath(e), t: c.getType(e)}
	ctx := c.newEstimateContext()
	if size, ok := c.getSizingStrategy().EstimateSize(ctx, node); ok {
		c.computedSizes[e.ID()] = size
		return &size
	}
	if size := computeTypeSize(c.getType(e)); size != nil {
		return size
	}
	return nil
}

// computeExprSize computes the size estimate from literal and declared container expressions.
func computeExprSize(expr ast.Expr) *SizeEstimate {
	var v uint64
	switch expr.Kind() {
	case ast.LiteralKind:
		switch ck := expr.AsLiteral().(type) {
		case types.String:
			// converting to runes here is an O(n) operation, but
			// this is consistent with how size is computed at runtime,
			// and how the language definition defines string size
			v = uint64(len([]rune(ck)))
		case types.Bytes:
			v = uint64(len(ck))
		case types.Bool, types.Double, types.Duration,
			types.Int, types.Timestamp, types.Uint,
			types.Null:
			v = uint64(1)
		default:
			return nil
		}
	default:
		return nil
	}
	size := FixedSizeEstimate(v)
	return &size
}

// computeTypeSize returns a fixed unit size estimate if the type is a constant-size scalar.
func computeTypeSize(t *types.Type) *SizeEstimate {
	if isScalar(t) {
		size := FixedSizeEstimate(1)
		return &size
	}
	return nil
}

// isScalar returns true if the given type is known to be of a constant size at
// compile time. isScalar will return false for strings (they are variable-width)
// in addition to protobuf.Any and protobuf.Value (their size is not knowable at compile time).
func isScalar(t *types.Type) bool {
	switch t.Kind() {
	case types.BoolKind, types.DoubleKind, types.DurationKind, types.IntKind, types.TimestampKind, types.UintKind, types.TypeKind:
		return true
	case types.OpaqueKind:
		if t.TypeName() == "optional_type" {
			return isScalar(t.Parameters()[0])
		}
	}
	return false
}

var (
	unknownSizeEstimate = SizeEstimate{Min: 0, Max: math.MaxUint64}
	unknownCostEstimate = unknownSizeEstimate.MultiplyByCostFactor(1)

	selectAndIdentCost = FixedCostEstimate(SelectAndIdentCost)
	constCost          = FixedCostEstimate(ConstCost)

	createListBaseCost    = FixedCostEstimate(ListCreateBaseCost)
	createMapBaseCost     = FixedCostEstimate(MapCreateBaseCost)
	createMessageBaseCost = FixedCostEstimate(StructCreateBaseCost)

	accumulatorName       = "__result__"
	hiddenAccumulatorName = "@result"
)
