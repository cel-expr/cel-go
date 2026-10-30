// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cel

import (
	"context"
	"errors"
	"fmt"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// ConstantFoldingOption defines a functional option for configuring constant folding.
type ConstantFoldingOption func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error)

// MaxConstantFoldIterations limits the number of times literals may be folding during optimization.
//
// Defaults to 100 if not set.
func MaxConstantFoldIterations(limit int) ConstantFoldingOption {
	return func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error) {
		opt.maxFoldIterations = limit
		return opt, nil
	}
}

// MaxComprehensionUnrollSize limits the maximum list literal size for unrolling .exists() and
// .all() comprehensions whose predicates cannot be fully folded to a constant.
//
// Setting a limit less than or equal to 0 disables comprehension unrolling.
// Defaults to 100 if not set.
func MaxComprehensionUnrollSize(limit int) ConstantFoldingOption {
	return func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error) {
		if limit <= 0 {
			opt.maxUnrollSize = 0
		} else {
			opt.maxUnrollSize = limit
		}
		return opt, nil
	}
}

// FoldKnownValues adds an Activation which provides known values for the folding evaluator
//
// Any values the activation provides will be used by the constant folder and turned into
// literals in the AST.
//
// Defaults to the NoVars() Activation
func FoldKnownValues(knownValues Activation) ConstantFoldingOption {
	return func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error) {
		if knownValues != nil {
			opt.knownValues = knownValues
		} else {
			opt.knownValues = NoVars()
		}
		return opt, nil
	}
}

// NewConstantFoldingOptimizer creates an optimizer which inlines constant scalar an aggregate
// literal values within function calls and select statements with their evaluated result.
func NewConstantFoldingOptimizer(opts ...ConstantFoldingOption) (ASTOptimizer, error) {
	folder := &constantFoldingOptimizer{
		maxFoldIterations: defaultMaxConstantFoldIterations,
		maxUnrollSize:     defaultMaxComprehensionUnrollSize,
	}
	var err error
	for _, o := range opts {
		folder, err = o(folder)
		if err != nil {
			return nil, err
		}
	}
	return folder, nil
}

type constantFoldingOptimizer struct {
	maxFoldIterations int
	maxUnrollSize     int
	knownValues       Activation
}

// Optimize queries the expression graph for scalar and aggregate literal expressions within call and
// select statements and then evaluates them and replaces the call site with the literal result.
//
// Note: only values which can be represented as literals in CEL syntax are supported.
func (opt *constantFoldingOptimizer) Optimize(ctx *OptimizerContext, a *ast.AST) *ast.AST {
	// Walk the list of foldable expressions and continue to fold until there are no more folds left.
	// All of the fold candidates returned by the constantExprMatcher should succeed unless there's
	// a logic bug with the selection of expressions.
	if !opt.foldConstants(ctx, a) {
		return a
	}

	// Prune resolved optional elements before attempting comprehension folding or unrolling so
	// aggregate literals with resolved optionals are flattened first.
	pruneOptionalElements(ctx, ast.NavigateAST(a))

	// Once all of the constants have been folded, try to run through the remaining comprehensions.
	// In this case, there's no guarantee they'll run, so we only update the target comprehension
	// node with the literal value if the evaluation succeeds. When evaluation fails with
	// errCannotFold, attempt to unroll eligible .exists() and .all() comprehensions over list
	// literals and re-run constant folding on the unrolled branches.
	for unrollIter := 0; ; unrollIter++ {
		unrolled := false
		root := ast.NavigateAST(a)
		for _, compre := range ast.MatchDescendants(root, ast.KindMatcher(ast.ComprehensionKind)) {
			if err := opt.tryFold(ctx, a, compre); err == nil {
				continue
			}
			if unrollIter < opt.maxFoldIterations && opt.tryUnrollComprehension(ctx, a, compre) {
				unrolled = true
			}
		}
		if !unrolled {
			break
		}
		if !opt.foldConstants(ctx, a) {
			return a
		}
		pruneOptionalElements(ctx, ast.NavigateAST(a))
	}

	// Ensure that all intermediate values in the folded expression can be represented as valid
	// CEL literals within the AST structure. Use `PostOrderVisit` rather than `MatchDescendents`
	// to avoid extra allocations during this final pass through the AST.
	root := ast.NavigateAST(a)
	ast.PostOrderVisit(root, ast.NewExprVisitor(func(e ast.Expr) {
		if e.Kind() != ast.LiteralKind {
			return
		}
		val := e.AsLiteral()
		adapted, err := adaptLiteral(ctx, val)
		if err != nil {
			ctx.ReportErrorAtID(root.ID(), "constant-folding evaluation failed: %v", err.Error())
			return
		}
		ctx.UpdateExpr(e, adapted)
	}))

	return a
}

func (opt *constantFoldingOptimizer) foldConstants(ctx *OptimizerContext, a *ast.AST) bool {
	constantExprMatcherCapture := func(e ast.NavigableExpr) bool { return opt.constantExprMatcher(ctx, a, e) }
	foldableExprs := ast.MatchDescendants(ast.NavigateAST(a), constantExprMatcherCapture)
	foldCount := 0
	for len(foldableExprs) != 0 && foldCount < opt.maxFoldIterations {
		for _, fold := range foldableExprs {
			// If the expression could be folded because it's a non-strict call, and the
			// branches are pruned, continue to the next fold.
			if fold.Kind() == ast.CallKind && maybePruneBranches(ctx, a, fold) {
				continue
			}
			// Async and late-bound function calls cannot be folded.
			if fold.Kind() == ast.CallKind && (isLateBoundFunctionCall(ctx, fold) || isAsyncFunctionCall(ctx, fold)) {
				continue
			}
			// Otherwise, assume all context is needed to evaluate the expression.
			err := opt.tryFold(ctx, a, fold)
			// Ignore errors for identifiers or subexpressions that cannot be folded, since there is no guarantee that the environment
			// has a value for them.
			if err != nil && fold.Kind() != ast.IdentKind && !errors.Is(err, errCannotFold) {
				ctx.ReportErrorAtID(fold.ID(), "constant-folding evaluation failed: %v", err.Error())
				return false
			}
		}
		foldCount++
		foldableExprs = ast.MatchDescendants(ast.NavigateAST(a), constantExprMatcherCapture)
	}
	return true
}

func (opt *constantFoldingOptimizer) tryUnrollComprehension(ctx *OptimizerContext, a *ast.AST, compNode ast.NavigableExpr) bool {
	if opt.maxUnrollSize <= 0 {
		return false
	}
	comp := compNode.AsComprehension()
	if comp.IterVar2() != "" {
		return false
	}

	if comp.AccuInit().Kind() != ast.LiteralKind {
		return false
	}
	initBool, ok := comp.AccuInit().AsLiteral().(types.Bool)
	if !ok {
		return false
	}
	isAll := initBool == types.True

	if comp.Result().Kind() != ast.IdentKind || comp.Result().AsIdent() != comp.AccuVar() {
		return false
	}

	if !isStandardQuantifierCondition(comp.LoopCondition(), comp.AccuVar(), isAll) {
		return false
	}

	predTemplate, expectedOp, ok := extractQuantifierPredicate(comp.LoopStep(), comp.AccuVar(), isAll)
	if !ok || !isBoolType(a, predTemplate) {
		return false
	}

	if hasLateBoundOrAsyncCall(ctx, compNode) {
		return false
	}

	iterRange := comp.IterRange()
	if iterRange.Kind() == ast.LiteralKind && iterRange.AsLiteral().Type() == types.ListType {
		lister, ok := iterRange.AsLiteral().(traits.Lister)
		if !ok {
			return false
		}
		sz, ok := lister.Size().(types.Int)
		if !ok || int(sz) > opt.maxUnrollSize {
			return false
		}
		elems := make([]ast.Expr, int(sz))
		it := lister.Iterator()
		for idx := 0; it.HasNext() == types.True; idx++ {
			elems[idx] = ctx.NewLiteral(it.Next())
		}
		ctx.UpdateExpr(iterRange, ctx.NewList(elems, []int32{}))
		iterRange = comp.IterRange()
	}

	if iterRange.Kind() != ast.ListKind {
		return false
	}
	list := iterRange.AsList()
	if len(list.OptionalIndices()) != 0 || list.Size() > opt.maxUnrollSize {
		return false
	}

	if list.Size() == 0 {
		lit := ctx.NewLiteral(initBool)
		clearSubtreeMacroCalls(ctx, compNode)
		ctx.UpdateExpr(compNode, lit)
		if a != nil {
			delete(a.ReferenceMap(), compNode.ID())
			a.SetType(compNode.ID(), types.BoolType)
		}
		return true
	}

	boundVars := collectBoundComprehensionVars(comp, predTemplate)
	elems := list.Elements()
	allElemsConstant := true
	for _, elem := range elems {
		if navElem, ok := elem.(ast.NavigableExpr); !ok || !constantMatcher(navElem) {
			allElemsConstant = false
		}
		freeInElem := map[string]bool{}
		collectFreeIdents(elem, nil, freeInElem)
		for identName := range freeInElem {
			if boundVars[identName] {
				return false
			}
		}
	}

	freeInPred := map[string]bool{}
	collectFreeIdents(predTemplate, nil, freeInPred)
	if freeInPred[comp.AccuVar()] {
		return false
	}
	iterVarFree := freeInPred[comp.IterVar()]
	if !iterVarFree && !allElemsConstant {
		return false
	}

	var preds []ast.Expr
	if !iterVarFree {
		preds = []ast.Expr{copySubExpr(ctx, a, predTemplate)}
	} else {
		preds = make([]ast.Expr, len(elems))
		for i, elem := range elems {
			predCopy := copySubExpr(ctx, a, predTemplate)
			substituteIterVar(ctx, a, predCopy, comp.IterVar(), elem)
			preds[i] = predCopy
		}
	}

	combined := buildBalancedLogicalTree(ctx, a, expectedOp, preds)
	clearSubtreeMacroCalls(ctx, compNode)
	ctx.UpdateExpr(compNode, combined)
	updateMetadata(a, compNode.ID(), combined.ID())
	if a != nil {
		a.SetType(compNode.ID(), types.BoolType)
	}
	return true
}

func clearSubtreeMacroCalls(ctx *OptimizerContext, expr ast.Expr) {
	var clearRec func(e ast.Expr)
	clearRec = func(e ast.Expr) {
		ast.PostOrderVisit(e, ast.NewExprVisitor(func(node ast.Expr) {
			if call, found := ctx.MacroCalls()[node.ID()]; found {
				ctx.ClearMacroCall(node.ID())
				clearRec(call)
			}
		}))
	}
	clearRec(expr)
}

func isStandardQuantifierCondition(cond ast.Expr, accuVar string, isAll bool) bool {
	if cond.Kind() != ast.CallKind {
		return false
	}
	condCall := cond.AsCall()
	if condCall.IsMemberFunction() || len(condCall.Args()) != 1 {
		return false
	}
	fn := condCall.FunctionName()
	if fn != operators.NotStrictlyFalse && fn != operators.OldNotStrictlyFalse {
		return false
	}
	arg := condCall.Args()[0]
	if isAll {
		return arg.Kind() == ast.IdentKind && arg.AsIdent() == accuVar
	}
	if arg.Kind() != ast.CallKind {
		return false
	}
	notCall := arg.AsCall()
	if notCall.IsMemberFunction() || notCall.FunctionName() != operators.LogicalNot || len(notCall.Args()) != 1 {
		return false
	}
	innerArg := notCall.Args()[0]
	return innerArg.Kind() == ast.IdentKind && innerArg.AsIdent() == accuVar
}

func extractQuantifierPredicate(step ast.Expr, accuVar string, isAll bool) (ast.Expr, string, bool) {
	if step.Kind() != ast.CallKind {
		return nil, "", false
	}
	stepCall := step.AsCall()
	expectedOp := operators.LogicalOr
	if isAll {
		expectedOp = operators.LogicalAnd
	}
	if stepCall.IsMemberFunction() || stepCall.FunctionName() != expectedOp || len(stepCall.Args()) != 2 {
		return nil, "", false
	}
	args := stepCall.Args()
	if args[0].Kind() == ast.IdentKind && args[0].AsIdent() == accuVar {
		return args[1], expectedOp, true
	}
	if args[1].Kind() == ast.IdentKind && args[1].AsIdent() == accuVar {
		return args[0], expectedOp, true
	}
	return nil, "", false
}

func hasLateBoundOrAsyncCall(ctx *OptimizerContext, expr ast.Expr) bool {
	found := false
	ast.PreOrderVisit(expr, ast.NewExprVisitor(func(e ast.Expr) {
		if e.Kind() == ast.CallKind && (isLateBoundFunctionCall(ctx, e) || isAsyncFunctionCall(ctx, e)) {
			found = true
		}
	}))
	return found
}

func collectBoundComprehensionVars(comp ast.ComprehensionExpr, predTemplate ast.Expr) map[string]bool {
	bound := map[string]bool{
		comp.IterVar(): true,
		comp.AccuVar(): true,
	}
	if comp.IterVar2() != "" {
		bound[comp.IterVar2()] = true
	}
	ast.PreOrderVisit(predTemplate, ast.NewExprVisitor(func(e ast.Expr) {
		if e.Kind() != ast.ComprehensionKind {
			return
		}
		inner := e.AsComprehension()
		bound[inner.IterVar()] = true
		bound[inner.AccuVar()] = true
		if inner.IterVar2() != "" {
			bound[inner.IterVar2()] = true
		}
	}))
	return bound
}

func collectFreeIdents(expr ast.Expr, bound map[string]bool, free map[string]bool) {
	switch expr.Kind() {
	case ast.IdentKind:
		name := expr.AsIdent()
		if !bound[name] {
			free[name] = true
		}
	case ast.SelectKind:
		collectFreeIdents(expr.AsSelect().Operand(), bound, free)
	case ast.CallKind:
		call := expr.AsCall()
		if call.IsMemberFunction() {
			collectFreeIdents(call.Target(), bound, free)
		}
		for _, arg := range call.Args() {
			collectFreeIdents(arg, bound, free)
		}
	case ast.ListKind:
		for _, elem := range expr.AsList().Elements() {
			collectFreeIdents(elem, bound, free)
		}
	case ast.MapKind:
		for _, entry := range expr.AsMap().Entries() {
			mapEntry := entry.AsMapEntry()
			collectFreeIdents(mapEntry.Key(), bound, free)
			collectFreeIdents(mapEntry.Value(), bound, free)
		}
	case ast.StructKind:
		for _, field := range expr.AsStruct().Fields() {
			collectFreeIdents(field.AsStructField().Value(), bound, free)
		}
	case ast.ComprehensionKind:
		comp := expr.AsComprehension()
		collectFreeIdents(comp.IterRange(), bound, free)
		collectFreeIdents(comp.AccuInit(), bound, free)
		innerBound := make(map[string]bool, len(bound)+3)
		for k, v := range bound {
			innerBound[k] = v
		}
		innerBound[comp.IterVar()] = true
		innerBound[comp.AccuVar()] = true
		if comp.IterVar2() != "" {
			innerBound[comp.IterVar2()] = true
		}
		collectFreeIdents(comp.LoopCondition(), innerBound, free)
		collectFreeIdents(comp.LoopStep(), innerBound, free)
		collectFreeIdents(comp.Result(), innerBound, free)
	}
}

func copySubExpr(ctx *OptimizerContext, a *ast.AST, subExpr ast.Expr) ast.Expr {
	subInfo := ast.NewSourceInfo(nil)
	var oldIDs []int64
	var collectMacroInfo func(e ast.Expr)
	collectMacroInfo = func(e ast.Expr) {
		ast.PostOrderVisit(e, ast.NewExprVisitor(func(node ast.Expr) {
			id := node.ID()
			if a == nil || a.SourceInfo() == nil {
				return
			}
			if offset, found := a.SourceInfo().GetOffsetRange(id); found {
				subInfo.SetOffsetRange(id, offset)
			}
			if call, found := a.SourceInfo().GetMacroCall(id); found {
				if _, exists := subInfo.GetMacroCall(id); !exists {
					subInfo.SetMacroCall(id, call)
					collectMacroInfo(call)
				}
			}
		}))
	}
	ast.PostOrderVisit(subExpr, ast.NewExprVisitor(func(node ast.Expr) {
		oldIDs = append(oldIDs, node.ID())
	}))
	collectMacroInfo(subExpr)

	copied := ctx.CopyASTAndMetadata(ast.NewAST(subExpr, subInfo))
	if a == nil {
		return copied
	}
	idx := 0
	ast.PostOrderVisit(copied, ast.NewExprVisitor(func(node ast.Expr) {
		oldID := oldIDs[idx]
		idx++
		newID := node.ID()
		if t, found := a.TypeMap()[oldID]; found {
			a.TypeMap()[newID] = t
		}
		if ref, found := a.ReferenceMap()[oldID]; found {
			a.ReferenceMap()[newID] = ref
		}
	}))
	return copied
}

func substituteIterVar(ctx *OptimizerContext, a *ast.AST, expr ast.Expr, iterVar string, elem ast.Expr) {
	switch expr.Kind() {
	case ast.IdentKind:
		if expr.AsIdent() == iterVar {
			elemCopy := copySubExpr(ctx, a, elem)
			ctx.UpdateExpr(expr, elemCopy)
			updateMetadata(a, expr.ID(), elemCopy.ID())
		}
	case ast.SelectKind:
		substituteIterVar(ctx, a, expr.AsSelect().Operand(), iterVar, elem)
	case ast.CallKind:
		call := expr.AsCall()
		if call.IsMemberFunction() {
			substituteIterVar(ctx, a, call.Target(), iterVar, elem)
		}
		for _, arg := range call.Args() {
			substituteIterVar(ctx, a, arg, iterVar, elem)
		}
	case ast.ListKind:
		for _, el := range expr.AsList().Elements() {
			substituteIterVar(ctx, a, el, iterVar, elem)
		}
	case ast.MapKind:
		for _, entry := range expr.AsMap().Entries() {
			mapEntry := entry.AsMapEntry()
			substituteIterVar(ctx, a, mapEntry.Key(), iterVar, elem)
			substituteIterVar(ctx, a, mapEntry.Value(), iterVar, elem)
		}
	case ast.StructKind:
		for _, field := range expr.AsStruct().Fields() {
			substituteIterVar(ctx, a, field.AsStructField().Value(), iterVar, elem)
		}
	case ast.ComprehensionKind:
		inner := expr.AsComprehension()
		substituteIterVar(ctx, a, inner.IterRange(), iterVar, elem)
		substituteIterVar(ctx, a, inner.AccuInit(), iterVar, elem)
		if inner.IterVar() == iterVar || inner.IterVar2() == iterVar || inner.AccuVar() == iterVar {
			return
		}
		substituteIterVar(ctx, a, inner.LoopCondition(), iterVar, elem)
		substituteIterVar(ctx, a, inner.LoopStep(), iterVar, elem)
		substituteIterVar(ctx, a, inner.Result(), iterVar, elem)
	}
}

func buildBalancedLogicalTree(ctx *OptimizerContext, a *ast.AST, op string, preds []ast.Expr) ast.Expr {
	if len(preds) == 1 {
		return preds[0]
	}
	mid := len(preds) / 2
	left := buildBalancedLogicalTree(ctx, a, op, preds[:mid])
	right := buildBalancedLogicalTree(ctx, a, op, preds[mid:])
	call := ctx.NewCall(op, left, right)
	if a != nil {
		a.SetType(call.ID(), types.BoolType)
	}
	return call
}

var errCannotFold = errors.New("subexpression cannot be folded")

// tryFold attempts to evaluate a sub-expression to a literal.
//
// If the evaluation succeeds, the input expr value will be modified to become a literal, otherwise
// the method will return an error.
func (opt *constantFoldingOptimizer) tryFold(ctx *OptimizerContext, a *ast.AST, expr ast.Expr) error {
	activation := opt.knownValues
	if activation == nil {
		activation = NoVars()
	}
	navExpr := expr.(ast.NavigableExpr)
	out, err := evaluateExpr(ctx, a, navExpr, activation)
	if err != nil {
		return err
	}
	// Update the fold expression to be a literal.
	ctx.UpdateExpr(expr, ctx.NewLiteral(out))
	delete(a.ReferenceMap(), expr.ID())
	return nil
}

func evaluateExpr(ctx *OptimizerContext, a *ast.AST, navigableExpr ast.NavigableExpr, activation Activation) (ref.Val, error) {
	partialActivation, err := ctx.PartialVars(activation)
	if err != nil {
		return nil, err
	}
	subAST := &Ast{
		impl: ast.NewCheckedAST(ast.NewAST(navigableExpr, a.SourceInfo()), a.TypeMap(), a.ReferenceMap()),
	}
	prg, err := ctx.Program(subAST)
	if err != nil {
		return nil, errCannotFold
	}
	// Folding will not attempt to call async functions,
	// but the presence of such functions requires the use of `ConcurrentEval` in order to
	// avoid an early return error which blocks async functions from running in `Eval` and
	// `ContextEval` call paths.
	resCh := prg.ConcurrentEval(context.Background(), partialActivation)
	res := <-resCh
	if res.Err != nil || types.IsUnknown(res.Val) {
		return nil, errCannotFold
	}
	return res.Val, nil
}

func isLateBoundFunctionCall(ctx *OptimizerContext, expr ast.Expr) bool {
	call := expr.AsCall()
	function := ctx.Functions()[call.FunctionName()]
	if function == nil {
		return false
	}
	return function.HasLateBinding()
}

func isAsyncFunctionCall(ctx *OptimizerContext, expr ast.Expr) bool {
	call := expr.AsCall()
	function := ctx.Functions()[call.FunctionName()]
	if function == nil {
		return false
	}
	return function.IsAsync()
}

// maybePruneBranches inspects the non-strict call expression to determine whether
// a branch can be removed. Evaluation will naturally prune logical and / or calls,
// but conditional will not be pruned cleanly, so this is one small area where the
// constant folding step reimplements a portion of the evaluator.
func maybePruneBranches(ctx *OptimizerContext, a *ast.AST, expr ast.NavigableExpr) bool {
	call := expr.AsCall()
	args := call.Args()
	switch call.FunctionName() {
	case operators.LogicalAnd, operators.LogicalOr:
		return maybeShortcircuitLogic(ctx, a, call.FunctionName(), args, expr)
	case operators.Conditional:
		cond := args[0]
		truthy := args[1]
		falsy := args[2]
		if cond.Kind() != ast.LiteralKind {
			return false
		}
		if cond.AsLiteral() == types.True {
			ctx.UpdateExpr(expr, truthy)
			updateMetadata(a, expr.ID(), truthy.ID())
		} else {
			ctx.UpdateExpr(expr, falsy)
			updateMetadata(a, expr.ID(), falsy.ID())
		}
		return true
	case operators.In:
		haystack := args[1]
		if isCollectionEmpty(haystack) {
			ctx.UpdateExpr(expr, ctx.NewLiteral(types.False))
			delete(a.ReferenceMap(), expr.ID())
			return true
		}
		if collectionContainsNeedle(args[0], haystack) {
			ctx.UpdateExpr(expr, ctx.NewLiteral(types.True))
			delete(a.ReferenceMap(), expr.ID())
			return true
		}
	case operators.Add:
		if len(args) == 2 && args[0].Kind() == ast.ListKind && args[1].Kind() == ast.ListKind {
			leftList := args[0].AsList()
			rightList := args[1].AsList()

			elems := make([]ast.Expr, 0, leftList.Size()+rightList.Size())
			elems = append(elems, leftList.Elements()...)
			elems = append(elems, rightList.Elements()...)

			optIndices := make([]int32, 0, len(leftList.OptionalIndices())+len(rightList.OptionalIndices()))
			optIndices = append(optIndices, leftList.OptionalIndices()...)
			offset := int32(leftList.Size())
			for _, idx := range rightList.OptionalIndices() {
				optIndices = append(optIndices, offset+idx)
			}

			combinedList := ctx.NewList(elems, optIndices)
			ctx.UpdateExpr(expr, combinedList)
			delete(a.ReferenceMap(), expr.ID())
			return true
		}
	}
	return false
}

func maybeShortcircuitLogic(ctx *OptimizerContext, a *ast.AST, function string, args []ast.Expr, expr ast.NavigableExpr) bool {
	shortcircuit := types.False
	skip := types.True
	if function == operators.LogicalOr {
		shortcircuit = types.True
		skip = types.False
	}
	newArgs := []ast.Expr{}
	for _, arg := range args {
		if arg.Kind() != ast.LiteralKind {
			newArgs = append(newArgs, arg)
			continue
		}
		if arg.AsLiteral() == skip {
			continue
		}
		if arg.AsLiteral() == shortcircuit {
			ctx.UpdateExpr(expr, arg)
			updateMetadata(a, expr.ID(), arg.ID())
			return true
		}
	}
	if len(newArgs) == 0 {
		newArgs = append(newArgs, args[0])
	}
	if len(newArgs) == len(args) {
		return false
	}
	if len(newArgs) == 1 {
		if !isBoolType(a, newArgs[0]) {
			return false
		}
		ctx.UpdateExpr(expr, newArgs[0])
		updateMetadata(a, expr.ID(), newArgs[0].ID())
		return true
	}
	ctx.UpdateExpr(expr, ctx.NewCall(function, newArgs...))
	return true
}

func updateMetadata(a *ast.AST, targetID, updatedID int64) {
	if a == nil {
		return
	}
	if ref, found := a.ReferenceMap()[updatedID]; found {
		a.ReferenceMap()[targetID] = ref
	} else {
		delete(a.ReferenceMap(), targetID)
	}
	if t, found := a.TypeMap()[updatedID]; found {
		a.TypeMap()[targetID] = t
	}
}

func isBoolType(a *ast.AST, e ast.Expr) bool {
	if a != nil && a.GetType(e.ID()) == types.BoolType {
		return true
	}
	if e.Kind() == ast.LiteralKind && e.AsLiteral().Type() == types.BoolType {
		return true
	}
	return false
}

// pruneOptionalElements works from the bottom up to resolve optional elements within
// aggregate literals.
//
// Note, many aggregate literals will be resolved as arguments to functions or select
// statements, so this method exists to handle the case where the literal could not be
// fully resolved or exists outside of a call, select, or comprehension context.
func pruneOptionalElements(ctx *OptimizerContext, root ast.NavigableExpr) {
	aggregateLiterals := ast.MatchDescendants(root, aggregateLiteralMatcher)
	for _, lit := range aggregateLiterals {
		switch lit.Kind() {
		case ast.ListKind:
			pruneOptionalListElements(ctx, lit)
		case ast.MapKind:
			pruneOptionalMapEntries(ctx, lit)
		case ast.StructKind:
			pruneOptionalStructFields(ctx, lit)
		}
	}
}

func pruneOptionalListElements(ctx *OptimizerContext, e ast.Expr) {
	l := e.AsList()
	elems := l.Elements()
	optIndices := l.OptionalIndices()
	if len(optIndices) == 0 {
		return
	}
	updatedElems := []ast.Expr{}
	updatedIndices := []int32{}
	newOptIndex := -1
	for i, e := range elems {
		newOptIndex++
		if !l.IsOptional(int32(i)) {
			updatedElems = append(updatedElems, e)
			continue
		}
		if e.Kind() != ast.LiteralKind {
			updatedElems = append(updatedElems, e)
			updatedIndices = append(updatedIndices, int32(newOptIndex))
			continue
		}
		optElemVal, ok := e.AsLiteral().(*types.Optional)
		if !ok {
			updatedElems = append(updatedElems, e)
			updatedIndices = append(updatedIndices, int32(newOptIndex))
			continue
		}
		if !optElemVal.HasValue() {
			newOptIndex-- // Skipping causes the list to get smaller.
			continue
		}
		ctx.UpdateExpr(e, ctx.NewLiteral(optElemVal.GetValue()))
		updatedElems = append(updatedElems, e)
	}
	ctx.UpdateExpr(e, ctx.NewList(updatedElems, updatedIndices))
}

func pruneOptionalMapEntries(ctx *OptimizerContext, e ast.Expr) {
	m := e.AsMap()
	entries := m.Entries()
	updatedEntries := []ast.EntryExpr{}
	modified := false
	for _, e := range entries {
		entry := e.AsMapEntry()
		key := entry.Key()
		val := entry.Value()
		// If the entry is not optional, or the value-side of the optional hasn't
		// been resolved to a literal, then preserve the entry as-is.
		if !entry.IsOptional() || val.Kind() != ast.LiteralKind {
			updatedEntries = append(updatedEntries, e)
			continue
		}
		optElemVal, ok := val.AsLiteral().(*types.Optional)
		if !ok {
			updatedEntries = append(updatedEntries, e)
			continue
		}
		// When the key is not a literal, but the value is, then it needs to be
		// restored to an optional value.
		if key.Kind() != ast.LiteralKind {
			undoOptVal, err := adaptLiteral(ctx, optElemVal)
			if err != nil {
				ctx.ReportErrorAtID(val.ID(), "invalid map value literal %v: %v", optElemVal, err)
			}
			ctx.UpdateExpr(val, undoOptVal)
			updatedEntries = append(updatedEntries, e)
			continue
		}
		modified = true
		if !optElemVal.HasValue() {
			continue
		}
		ctx.UpdateExpr(val, ctx.NewLiteral(optElemVal.GetValue()))
		updatedEntry := ctx.NewMapEntry(key, val, false)
		updatedEntries = append(updatedEntries, updatedEntry)
	}
	if modified {
		ctx.UpdateExpr(e, ctx.NewMap(updatedEntries))
	}
}

func pruneOptionalStructFields(ctx *OptimizerContext, e ast.Expr) {
	s := e.AsStruct()
	fields := s.Fields()
	updatedFields := []ast.EntryExpr{}
	modified := false
	for _, f := range fields {
		field := f.AsStructField()
		val := field.Value()
		if !field.IsOptional() || val.Kind() != ast.LiteralKind {
			updatedFields = append(updatedFields, f)
			continue
		}
		optElemVal, ok := val.AsLiteral().(*types.Optional)
		if !ok {
			updatedFields = append(updatedFields, f)
			continue
		}
		modified = true
		if !optElemVal.HasValue() {
			continue
		}
		ctx.UpdateExpr(val, ctx.NewLiteral(optElemVal.GetValue()))
		updatedField := ctx.NewStructField(field.Name(), val, false)
		updatedFields = append(updatedFields, updatedField)
	}
	if modified {
		ctx.UpdateExpr(e, ctx.NewStruct(s.TypeName(), updatedFields))
	}
}

// adaptLiteral converts a runtime CEL value to its equivalent literal expression.
//
// For strongly typed values, the type-provider will be used to reconstruct the fields
// which are present in the literal and their equivalent initialization values.
func adaptLiteral(ctx *OptimizerContext, val ref.Val) (ast.Expr, error) {
	switch t := val.Type().(type) {
	case *types.Type:
		switch t {
		case types.BoolType, types.BytesType, types.DoubleType, types.IntType,
			types.NullType, types.StringType, types.UintType:
			return ctx.NewLiteral(val), nil
		case types.DurationType:
			return ctx.NewCall(
				overloads.TypeConvertDuration,
				ctx.NewLiteral(val.ConvertToType(types.StringType)),
			), nil
		case types.TimestampType:
			return ctx.NewCall(
				overloads.TypeConvertTimestamp,
				ctx.NewLiteral(val.ConvertToType(types.StringType)),
			), nil
		case types.OptionalType:
			opt := val.(*types.Optional)
			if !opt.HasValue() {
				return ctx.NewCall("optional.none"), nil
			}
			target, err := adaptLiteral(ctx, opt.GetValue())
			if err != nil {
				return nil, err
			}
			return ctx.NewCall("optional.of", target), nil
		case types.TypeType:
			return ctx.NewIdent(val.(*types.Type).TypeName()), nil
		case types.ListType:
			l, ok := val.(traits.Lister)
			if !ok {
				return nil, fmt.Errorf("failed to adapt %v to literal", val)
			}
			elems := make([]ast.Expr, l.Size().(types.Int))
			idx := 0
			it := l.Iterator()
			for it.HasNext() == types.True {
				elemVal := it.Next()
				elemExpr, err := adaptLiteral(ctx, elemVal)
				if err != nil {
					return nil, err
				}
				elems[idx] = elemExpr
				idx++
			}
			return ctx.NewList(elems, []int32{}), nil
		case types.MapType:
			m, ok := val.(traits.Mapper)
			if !ok {
				return nil, fmt.Errorf("failed to adapt %v to literal", val)
			}
			entries := make([]ast.EntryExpr, m.Size().(types.Int))
			idx := 0
			it := m.Iterator()
			for it.HasNext() == types.True {
				keyVal := it.Next()
				keyExpr, err := adaptLiteral(ctx, keyVal)
				if err != nil {
					return nil, err
				}
				valVal := m.Get(keyVal)
				valExpr, err := adaptLiteral(ctx, valVal)
				if err != nil {
					return nil, err
				}
				entries[idx] = ctx.NewMapEntry(keyExpr, valExpr, false)
				idx++
			}
			return ctx.NewMap(entries), nil
		default:
			provider := ctx.CELTypeProvider()
			fields, found := provider.FindStructFieldNames(t.TypeName())
			if !found {
				return nil, fmt.Errorf("failed to adapt %v to literal", val)
			}
			tester := val.(traits.FieldTester)
			indexer := val.(traits.Indexer)
			fieldInits := []ast.EntryExpr{}
			for _, f := range fields {
				field := types.String(f)
				if tester.IsSet(field) != types.True {
					continue
				}
				fieldVal := indexer.Get(field)
				fieldExpr, err := adaptLiteral(ctx, fieldVal)
				if err != nil {
					return nil, err
				}
				fieldInits = append(fieldInits, ctx.NewStructField(f, fieldExpr, false))
			}
			return ctx.NewStruct(t.TypeName(), fieldInits), nil
		}
	}
	return nil, fmt.Errorf("failed to adapt %v to literal", val)
}

// constantExprMatcher matches calls, select statements, and comprehensions whose arguments
// are all constant scalar or aggregate literal values.
//
// Only comprehensions which are not nested are included as possible constant folds, and only
// if all variables referenced in the comprehension stack exist are only iteration or
// accumulation variables.
func (opt *constantFoldingOptimizer) constantExprMatcher(ctx *OptimizerContext, a *ast.AST, e ast.NavigableExpr) bool {
	switch e.Kind() {
	case ast.CallKind:
		return constantCallMatcher(e)
	case ast.SelectKind:
		sel := e.AsSelect() // guaranteed to be a navigable value
		return constantMatcher(sel.Operand().(ast.NavigableExpr))
	case ast.IdentKind:
		return opt.knownValues != nil && a.ReferenceMap()[e.ID()] != nil && !hasComprehensionVar(e)
	case ast.ComprehensionKind:
		if isNestedComprehension(e) {
			return false
		}
		vars := map[string]bool{}
		constantExprs := true
		visitor := ast.NewExprVisitor(func(e ast.Expr) {
			if e.Kind() == ast.ComprehensionKind {
				nested := e.AsComprehension()
				vars[nested.AccuVar()] = true
				vars[nested.IterVar()] = true
				if nested.IterVar2() != "" {
					vars[nested.IterVar2()] = true
				}
			}
			if e.Kind() == ast.IdentKind && !vars[e.AsIdent()] {
				constantExprs = false
			}
			// Async and late-bound function calls cannot be folded.
			if e.Kind() == ast.CallKind && (isLateBoundFunctionCall(ctx, e) || isAsyncFunctionCall(ctx, e)) {
				constantExprs = false
			}
		})
		ast.PreOrderVisit(e, visitor)
		return constantExprs
	default:
		return false
	}
}

// constantCallMatcher identifies strict and non-strict calls which can be folded.
func constantCallMatcher(e ast.NavigableExpr) bool {
	call := e.AsCall()
	children := e.Children()
	fnName := call.FunctionName()
	if fnName == operators.LogicalAnd {
		for _, child := range children {
			if child.Kind() == ast.LiteralKind {
				return true
			}
		}
	}
	if fnName == operators.LogicalOr {
		for _, child := range children {
			if child.Kind() == ast.LiteralKind {
				return true
			}
		}
	}
	if fnName == operators.Conditional {
		cond := children[0]
		if cond.Kind() == ast.LiteralKind && cond.AsLiteral().Type() == types.BoolType {
			return true
		}
	}
	if fnName == operators.Equals || fnName == operators.NotEquals {
		if hasComprehensionVar(e) {
			return false
		}
		if isExprConstantOfKind(children[0], types.BoolType) || isExprConstantOfKind(children[1], types.BoolType) {
			return true
		}
	}
	if fnName == operators.In {
		if hasComprehensionVar(e) {
			return false
		}
		haystack := children[1]
		if isCollectionEmpty(haystack) || collectionContainsNeedle(children[0], haystack) {
			return true
		}
	}
	if fnName == operators.Add {
		if len(children) == 2 && children[0].Kind() == ast.ListKind && children[1].Kind() == ast.ListKind {
			return true
		}
	}
	// convert all other calls with constant arguments
	for _, child := range children {
		if !constantMatcher(child) {
			return false
		}
	}
	return true
}

func isCollectionEmpty(e ast.Expr) bool {
	switch e.Kind() {
	case ast.ListKind:
		return e.AsList().Size() == 0
	case ast.MapKind:
		return e.AsMap().Size() == 0
	default:
		return false
	}
}

func exprEqualsNeedle(needle, target ast.Expr) bool {
	if needle.Kind() == ast.LiteralKind && target.Kind() == ast.LiteralKind {
		return target.AsLiteral().Equal(needle.AsLiteral()) == types.True
	}
	if needle.Kind() == ast.IdentKind && target.Kind() == ast.IdentKind {
		return target.AsIdent() == needle.AsIdent()
	}
	return false
}

func collectionContainsNeedle(needle, haystack ast.Expr) bool {
	if needle.Kind() != ast.LiteralKind && !isSelfEqualIdent(needle) {
		return false
	}
	switch haystack.Kind() {
	case ast.ListKind:
		list := haystack.AsList()
		for i, elem := range list.Elements() {
			if list.IsOptional(int32(i)) {
				if needle.Kind() == ast.LiteralKind && elem.Kind() == ast.LiteralKind {
					if opt, ok := elem.AsLiteral().(*types.Optional); ok && opt.HasValue() {
						if opt.GetValue().Equal(needle.AsLiteral()) == types.True {
							return true
						}
					}
				}
				continue
			}
			if exprEqualsNeedle(needle, elem) {
				return true
			}
		}
	case ast.MapKind:
		for _, entryExpr := range haystack.AsMap().Entries() {
			entry := entryExpr.AsMapEntry()
			if entry.IsOptional() {
				if val := entry.Value(); val.Kind() == ast.LiteralKind {
					if opt, ok := val.AsLiteral().(*types.Optional); ok && opt.HasValue() {
						if exprEqualsNeedle(needle, entry.Key()) {
							return true
						}
					}
				}
				continue
			}
			if exprEqualsNeedle(needle, entry.Key()) {
				return true
			}
		}
	}
	return false
}

// isSelfEqualIdent indicates whether the expression is an identifier whose static type
// guarantees that its runtime value is equal to itself.
//
// Matching an identifier against a list element by name only proves list membership when the
// value the name resolves to is self-equal. A double may be NaN, which is not equal to itself,
// and dyn, abstract, and struct types may all hold a NaN at runtime, so the check is limited
// to the scalar types which cannot, and to the aggregate types whose type parameters are
// themselves self-equal.
func isSelfEqualIdent(e ast.Expr) bool {
	if e.Kind() != ast.IdentKind {
		return false
	}
	nav, ok := e.(ast.NavigableExpr)
	if !ok {
		return false
	}
	return isSelfEqualType(nav.Type())
}

// isSelfEqualType indicates whether all runtime values of the given type are equal to themselves.
func isSelfEqualType(t *types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case types.BoolKind, types.BytesKind, types.DurationKind, types.IntKind,
		types.NullTypeKind, types.StringKind, types.TimestampKind, types.TypeKind,
		types.UintKind:
		return true
	case types.ListKind, types.MapKind:
		// Aggregates compare element-wise, so they are self-equal exactly when their type
		// parameters are. A list(dyn) or map(string, double) may still contain a NaN.
		for _, p := range t.Parameters() {
			if !isSelfEqualType(p) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isExprConstantOfKind(e ast.Expr, t *types.Type) bool {
	return e.Kind() == ast.LiteralKind && e.AsLiteral().Type() == t
}

func hasComprehensionVar(e ast.NavigableExpr) bool {
	idents := ast.MatchDescendants(e, ast.KindMatcher(ast.IdentKind))
	for _, identNode := range idents {
		identName := identNode.AsIdent()
		curr := identNode
		parent, found := curr.Parent()
		for found {
			if parent.Kind() == ast.ComprehensionKind {
				compre := parent.AsComprehension()
				if (compre.AccuVar() == identName || compre.IterVar() == identName || compre.IterVar2() == identName) &&
					curr.ID() != compre.IterRange().ID() && curr.ID() != compre.AccuInit().ID() {
					return true
				}
			}
			curr = parent
			parent, found = parent.Parent()
		}
	}
	return false
}

func isNestedComprehension(e ast.NavigableExpr) bool {
	parent, found := e.Parent()
	for found {
		if parent.Kind() == ast.ComprehensionKind {
			return true
		}
		parent, found = parent.Parent()
	}
	return false
}

func aggregateLiteralMatcher(e ast.NavigableExpr) bool {
	return e.Kind() == ast.ListKind || e.Kind() == ast.MapKind || e.Kind() == ast.StructKind
}

var (
	constantMatcher = ast.ConstantValueMatcher()
)

const (
	defaultMaxConstantFoldIterations  = 100
	defaultMaxComprehensionUnrollSize = 100
)
