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

package cel

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	celast "cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/interpreter"
	"cel.dev/cel-go/parser"
)

// Explanation describes an evaluation recorded with OptTraceIterations.
//
// The explanation relates the records of the evaluation back to the expressions in the AST, making
// it possible to determine which comprehension iterations, e.g. which list elements, were
// responsible for a result. An Explanation is not safe for concurrent use.
type Explanation struct {
	ast    *celast.AST
	source string
	exprs  map[int64]celast.Expr
	text   map[int64]string

	// info is a copy of the AST source info with macro calls synthesized for recognized
	// comprehensions so that they unparse as the macro which produced them.
	info *celast.SourceInfo

	// internal holds the ids of expressions which implement comprehension bookkeeping, such as
	// accumulator references and loop conditions, rather than expressions written by the user.
	internal map[int64]bool

	// shapes holds the classification of each comprehension by expression id.
	shapes map[int64]comprehensionShape

	// records are the records of the evaluation. values holds the last value of each expression
	// within each iteration, and iterations the iterations of each comprehension within each
	// iteration, both keyed by expression id and the number of the enclosing iteration record.
	records    []interpreter.TraceRecord
	values     map[traceKey]ref.Val
	iterations map[traceKey][]int
}

type traceKey struct {
	id     int64
	parent int
}

// ExplainEval creates an Explanation for the evaluation of the AST.
//
// The program must have been created from the same AST with the OptTraceIterations evaluation
// option. To identify every iteration which contributed to a result, rather than only the
// iterations evaluated before the result was determined, also specify OptExhaustiveEval.
func ExplainEval(a *Ast, details *EvalDetails) (*Explanation, error) {
	if a == nil {
		return nil, errors.New("explain: nil ast")
	}
	records, ok := interpreter.TraceRecords(details.State())
	if !ok {
		return nil, errors.New("explain: iteration tracing is not enabled, use EvalOptions(OptTraceIterations)")
	}
	native := a.NativeRep()
	e := &Explanation{
		ast:        native,
		exprs:      map[int64]celast.Expr{},
		text:       map[int64]string{},
		info:       celast.CopySourceInfo(native.SourceInfo()),
		internal:   map[int64]bool{},
		shapes:     map[int64]comprehensionShape{},
		records:    records,
		values:     make(map[traceKey]ref.Val, len(records)),
		iterations: map[traceKey][]int{},
	}
	if src := a.Source(); src != nil {
		e.source = src.Content()
	}
	fac := celast.NewExprFactory()
	celast.PostOrderVisit(native.Expr(), celast.NewExprVisitor(func(expr celast.Expr) {
		e.exprs[expr.ID()] = expr
		if expr.Kind() == celast.ComprehensionKind {
			e.indexComprehension(fac, expr)
		}
	}))
	for i, r := range records {
		key := traceKey{id: r.ID, parent: r.Parent}
		if r.IsIteration() {
			e.iterations[key] = append(e.iterations[key], i+1)
		} else {
			e.values[key] = r.Value
		}
	}
	return e, nil
}

// value returns the last value of the expression within the iteration, if any.
func (e *Explanation) value(id int64, trace int) (ref.Val, bool) {
	val := e.values[traceKey{id: id, parent: trace}]
	return val, val != nil
}

// indexComprehension synthesizes a macro call for the comprehension, if it is recognized and was
// not already recorded by the parser, and marks its bookkeeping expressions as internal.
func (e *Explanation) indexComprehension(fac celast.ExprFactory, expr celast.Expr) {
	comp := expr.AsComprehension()
	shape, found := e.recordedShape(expr)
	if !found {
		shape = classifyComprehension(expr)
	}
	e.shapes[expr.ID()] = shape
	macro := shape.macro(fac, expr)
	if macro == nil {
		// Unrecognized comprehension: only accumulator references are known to be bookkeeping.
		celast.PostOrderVisit(expr, celast.NewExprVisitor(func(x celast.Expr) {
			if isAccuIdent(comp, x) {
				e.internal[x.ID()] = true
			}
		}))
		return
	}
	// Every expression in the loop step other than the user-written sub-expressions is bookkeeping,
	// as are the accumulator initializer, loop condition, and result of all but cel.bind(), whose
	// initializer and result are written by the user.
	user := map[int64]bool{}
	for _, u := range []celast.Expr{shape.pred, shape.elem} {
		if u != nil {
			celast.PostOrderVisit(u, celast.NewExprVisitor(func(x celast.Expr) { user[x.ID()] = true }))
		}
	}
	bookkeeping := []celast.Expr{comp.LoopCondition(), comp.LoopStep()}
	if shape.kind == compBind {
		bookkeeping = append(bookkeeping, comp.IterRange())
	} else {
		bookkeeping = append(bookkeeping, comp.AccuInit(), comp.Result())
	}
	for _, b := range bookkeeping {
		celast.PostOrderVisit(b, celast.NewExprVisitor(func(x celast.Expr) {
			if !user[x.ID()] {
				e.internal[x.ID()] = true
			}
		}))
	}
	if _, found := e.info.GetMacroCall(expr.ID()); !found {
		e.info.SetMacroCall(expr.ID(), macro)
	}
}

// compKind identifies the macro which a comprehension expands from.
type compKind int

const (
	compUnknown compKind = iota
	compBind
	compAll
	compExists
	compExistsOne
	compFilter
	// compMap covers map() and transformList(), with an optional filter predicate.
	compMap
	// compMapInsert covers transformMap() and transformMapEntry(), with an optional filter predicate.
	compMapInsert
)

// comprehensionShape describes a comprehension in terms of the macro which produced it.
type comprehensionShape struct {
	kind compKind
	// pred is the per-iteration predicate of quantifiers and filters, if any.
	pred celast.Expr
	// elem is the per-iteration element contributed to the result of map-like comprehensions.
	elem celast.Expr
	// entry indicates a transformMapEntry(), whose element is a map entry rather than a value.
	entry bool
}

// recordedShape classifies a comprehension from the macro call recorded by the parser with
// EnableMacroCallTracking, if any. The recorded arguments keep the ids of the sub-expressions they
// expand to, so the predicate and element are found by their position in the call.
//
// The record is used only when it names a standard or two-variable macro and each of the
// sub-expressions it identifies is part of the loop step, which guards against records which do not
// describe the expansion, e.g. those of an AST rewritten after parsing.
func (e *Explanation) recordedShape(expr celast.Expr) (comprehensionShape, bool) {
	call, found := e.info.GetMacroCall(expr.ID())
	if !found || call.Kind() != celast.CallKind {
		return comprehensionShape{}, false
	}
	comp := expr.AsComprehension()
	name := call.AsCall().FunctionName()
	shape := comprehensionShape{kind: macroKind(name), entry: name == "transformMapEntry"}
	vars := 1
	if comp.HasIterVar2() {
		vars = 2
	}
	args := call.AsCall().Args()
	if shape.kind == compUnknown || len(args) <= vars {
		return comprehensionShape{}, false
	}
	// Each argument after the variables is the step's sub-expression with the same id.
	var parts []celast.Expr
	inStep := map[int64]celast.Expr{}
	celast.PostOrderVisit(comp.LoopStep(), celast.NewExprVisitor(func(x celast.Expr) { inStep[x.ID()] = x }))
	for _, arg := range args[vars:] {
		part, found := inStep[arg.ID()]
		if !found {
			return comprehensionShape{}, false
		}
		parts = append(parts, part)
	}
	mapLike := shape.kind == compMap || shape.kind == compMapInsert
	switch {
	case len(parts) == 1 && mapLike:
		shape.elem = parts[0]
	case len(parts) == 1:
		shape.pred = parts[0]
	case len(parts) == 2 && mapLike:
		shape.pred, shape.elem = parts[0], parts[1]
	default:
		return comprehensionShape{}, false
	}
	return shape, true
}

// macroKind returns the comprehension kind of the named standard or two-variable macro.
func macroKind(name string) compKind {
	switch name {
	case "transformMap", "transformMapEntry":
		return compMapInsert
	}
	for kind, names := range macroNames {
		if name == names[0] || name == names[1] {
			return kind
		}
	}
	return compUnknown
}

// classifyComprehension recognizes the expansions of the standard and two-variable macros.
func classifyComprehension(expr celast.Expr) comprehensionShape {
	comp := expr.AsComprehension()
	if celast.IsBind(expr) {
		return comprehensionShape{kind: compBind}
	}
	if pred := quantifierPredicate(comp); pred != nil {
		if comp.LoopStep().AsCall().FunctionName() == operators.LogicalOr {
			return comprehensionShape{kind: compExists, pred: pred}
		}
		return comprehensionShape{kind: compAll, pred: pred}
	}
	step := comp.LoopStep()
	var pred celast.Expr
	if step.Kind() == celast.CallKind && step.AsCall().FunctionName() == operators.Conditional {
		args := step.AsCall().Args()
		if !isAccuIdent(comp, args[2]) {
			return comprehensionShape{}
		}
		pred, step = args[0], args[1]
		if isAccuIncrement(comp, step) {
			return comprehensionShape{kind: compExistsOne, pred: pred}
		}
	}
	if elem := accuAppendElem(comp, step); elem != nil {
		if pred != nil && !comp.HasIterVar2() && elem.Kind() == celast.IdentKind && elem.AsIdent() == comp.IterVar() {
			return comprehensionShape{kind: compFilter, pred: pred}
		}
		return comprehensionShape{kind: compMap, pred: pred, elem: elem}
	}
	if elem, entry := accuInsertElem(comp, step); elem != nil {
		return comprehensionShape{kind: compMapInsert, pred: pred, elem: elem, entry: entry}
	}
	return comprehensionShape{}
}

// macroNames holds the single-variable and two-variable macro names of each comprehension kind.
var macroNames = map[compKind][2]string{
	compAll:       {"all", "all"},
	compExists:    {"exists", "exists"},
	compExistsOne: {"exists_one", "existsOne"},
	compFilter:    {"filter", "filter"},
	compMap:       {"map", "transformList"},
}

// macro returns the macro call which expands to the comprehension, or nil if it is unrecognized.
func (s comprehensionShape) macro(fac celast.ExprFactory, expr celast.Expr) celast.Expr {
	comp := expr.AsComprehension()
	if s.kind == compBind {
		return fac.NewCall(0, "cel.bind", fac.NewIdent(0, comp.AccuVar()), comp.AccuInit(), comp.Result())
	}
	names, found := macroNames[s.kind]
	switch {
	case s.kind == compMapInsert && s.entry:
		names, found = [2]string{"", "transformMapEntry"}, true
	case s.kind == compMapInsert:
		names, found = [2]string{"", "transformMap"}, true
	}
	if !found {
		return nil
	}
	args := []celast.Expr{fac.NewIdent(0, comp.IterVar())}
	name := names[0]
	if comp.HasIterVar2() {
		args = append(args, fac.NewIdent(0, comp.IterVar2()))
		name = names[1]
	}
	for _, arg := range []celast.Expr{s.pred, s.elem} {
		if arg != nil {
			args = append(args, arg)
		}
	}
	return fac.NewMemberCall(0, name, comp.IterRange(), args...)
}

// accuAppendElem returns elem when e has the form `accu + [elem]`, and nil otherwise.
func accuAppendElem(comp celast.ComprehensionExpr, e celast.Expr) celast.Expr {
	if e.Kind() != celast.CallKind || e.AsCall().FunctionName() != operators.Add {
		return nil
	}
	args := e.AsCall().Args()
	if !isAccuIdent(comp, args[0]) || args[1].Kind() != celast.ListKind {
		return nil
	}
	if elems := args[1].AsList().Elements(); len(elems) == 1 {
		return elems[0]
	}
	return nil
}

// accuInsertElem returns the transform of `cel.@mapInsert(accu, iterVar, transform)` or, with entry
// set, of `cel.@mapInsert(accu, transform)`, and nil otherwise.
func accuInsertElem(comp celast.ComprehensionExpr, e celast.Expr) (celast.Expr, bool) {
	if e.Kind() != celast.CallKind || e.AsCall().FunctionName() != "cel.@mapInsert" {
		return nil, false
	}
	args := e.AsCall().Args()
	if !isAccuIdent(comp, args[0]) {
		return nil, false
	}
	return args[len(args)-1], len(args) == 2
}

// isAccuIncrement reports whether e has the form `accu + 1`.
func isAccuIncrement(comp celast.ComprehensionExpr, e celast.Expr) bool {
	if e.Kind() != celast.CallKind || e.AsCall().FunctionName() != operators.Add {
		return false
	}
	args := e.AsCall().Args()
	return isAccuIdent(comp, args[0]) &&
		args[1].Kind() == celast.LiteralKind && args[1].AsLiteral() == types.IntOne
}

func isAccuIdent(comp celast.ComprehensionExpr, e celast.Expr) bool {
	return e.Kind() == celast.IdentKind && e.AsIdent() == comp.AccuVar()
}

// Result returns the value of the root expression, or nil if it was not observed.
func (e *Explanation) Result() ref.Val {
	val, _ := e.value(e.ast.Expr().ID(), 0)
	return val
}

// Violations returns the sub-expressions responsible for the expression not evaluating to true,
// along with the comprehension iterations in which each was evaluated.
//
// Starting from the root, the analysis descends through logical operators, conditionals,
// cel.bind() and cel.@block() to the operands which determined the result, and through all(),
// exists(), and exists_one() macros to the individual iterations whose predicate determined the
// result. For example, the violations of
// `object.spec.containers.all(c, c.securityContext.privileged == false)` are the containers whose
// predicate evaluated to false, identified by their list index. When a violation contains other
// comprehensions, such as a filter() or map(), the iterations which produced their values are
// reported instead, with the violation as their Context.
//
// An empty result indicates that the expression evaluated to true. Iterations which were not
// evaluated because the result was already determined are not reported unless the program was
// created with OptExhaustiveEval.
func (e *Explanation) Violations() []Violation {
	var out []Violation
	e.blame(e.ast.Expr(), 0, true, &out)
	return out
}

// String renders the result and violations of the evaluation.
func (e *Explanation) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "result: %s\n", formatExplainValue(e.Result()))
	for _, v := range e.Violations() {
		fmt.Fprintf(&sb, "  %s\n", v.String())
	}
	return sb.String()
}

// IterationRef identifies a single comprehension iteration.
type IterationRef struct {
	// ComprehensionID is the expression id of the comprehension.
	ComprehensionID int64

	// Range is the source text of the expression being iterated, e.g. `object.spec.containers`.
	Range string

	// Key is the list index or map key of the iteration.
	Key ref.Val

	// Value is the list element or map value of the iteration, if the iteration range was observed.
	Value ref.Val

	// elemVar is the comprehension variable bound to the Value, if any.
	elemVar string
}

// String renders the iteration as an index into the iteration range, e.g. `items[2]`.
func (r IterationRef) String() string {
	return formatIterationPath([]IterationRef{r})
}

// renderKey renders an iteration key as an index.
func renderKey(key ref.Val) string {
	if k, ok := key.(types.String); ok {
		return strconv.Quote(string(k))
	}
	return fmt.Sprintf("%v", key)
}

// formatIterationPath renders an iteration path, substituting the outer iteration for its element
// variable where an inner iteration range is derived from it, e.g. `containers[1].ports[0]` rather
// than `containers[1] > c.ports[0]`.
func formatIterationPath(path []IterationRef) string {
	parts := make([]string, 0, len(path))
	prevVar, prevText := "", ""
	for _, r := range path {
		operand := r.Range
		if strings.ContainsAny(operand, " ?:") {
			operand = "(" + operand + ")"
		}
		if prevVar != "" && (r.Range == prevVar ||
			strings.HasPrefix(r.Range, prevVar+".") || strings.HasPrefix(r.Range, prevVar+"[")) {
			// Replace the trailing element of the path with the derived iteration.
			operand = prevText + strings.TrimPrefix(r.Range, prevVar)
			parts = parts[:len(parts)-1]
		}
		text := operand + "[" + renderKey(r.Key) + "]"
		parts = append(parts, text)
		prevVar, prevText = r.elemVar, text
	}
	return strings.Join(parts, " > ")
}

// Violation identifies a sub-expression which contributed to the expression not evaluating to
// true, and the comprehension iterations in which it was evaluated.
type Violation struct {
	// Path lists the comprehension iterations, from outermost to innermost, in which the
	// sub-expression was evaluated. The path is empty for sub-expressions outside of comprehensions.
	Path []IterationRef

	// ExprID is the expression id of the sub-expression.
	ExprID int64

	// Expr is the source text of the sub-expression.
	Expr string

	// Value is the value of the sub-expression within the iteration.
	Value ref.Val

	// Context is the enclosing expression whose value was derived from this iteration, when the
	// violation was found by tracing the iterations of a filter(), map(), or other comprehension
	// which does not itself produce a boolean, e.g. the `size() == 0` comparison of
	// `items.filter(i, i.bad).size() == 0`. Context is nil when Expr itself determined the result.
	Context *Violation
}

// PathString renders the iteration path of the violation, e.g. `object.spec.containers[1]`.
func (v Violation) PathString() string {
	return formatIterationPath(v.Path)
}

// String renders the violation.
func (v Violation) String() string {
	out := fmt.Sprintf("%s -> %s", v.Expr, formatExplainValue(v.Value))
	if len(v.Path) != 0 {
		out = v.PathString() + ": " + out
	}
	if v.Context != nil {
		out += fmt.Sprintf(", in %s -> %s", v.Context.Expr, formatExplainValue(v.Context.Value))
	}
	return out
}

// blame appends the violations which explain why expr did not evaluate to want under the trace.
func (e *Explanation) blame(expr celast.Expr, trace int, want bool, out *[]Violation) {
	val, found := e.value(expr.ID(), trace)
	if !found || val == types.Bool(want) {
		return
	}
	before := len(*out)
	if !e.blameOperands(expr, trace, want, out) {
		e.blameLeaf(expr, trace, val, out)
		return
	}
	if len(*out) == before {
		*out = append(*out, e.violation(expr, trace, val))
	}
}

// blameOperands blames the operands or iterations which determined the value of a logical operator,
// conditional, cel.@block(), cel.bind(), or quantifier, and reports false for any other expression.
func (e *Explanation) blameOperands(expr celast.Expr, trace int, want bool, out *[]Violation) bool {
	switch expr.Kind() {
	case celast.CallKind:
		args := expr.AsCall().Args()
		switch expr.AsCall().FunctionName() {
		case operators.LogicalAnd, operators.LogicalOr:
			// Operands which evaluated to want are skipped, so for a false `&&` only the false
			// operands are blamed, while for a false `||` every operand is blamed.
			for _, arg := range args {
				e.blame(arg, trace, want, out)
			}
		case operators.LogicalNot:
			before := len(*out)
			e.blame(args[0], trace, !want, out)
			// When the operand is itself the only violation, the negation is reported instead so
			// the violation reads with the polarity of the source, e.g. `!has(x.y) -> false`.
			if len(*out) == before+1 && (*out)[before].ExprID == args[0].ID() {
				*out = (*out)[:before]
			}
		case operators.Conditional:
			switch cond, _ := e.value(args[0].ID(), trace); cond {
			case types.True:
				e.blame(args[1], trace, want, out)
			case types.False:
				e.blame(args[2], trace, want, out)
			}
		case "cel.@block":
			e.blame(args[1], trace, want, out)
		default:
			return false
		}
	case celast.ComprehensionKind:
		shape := e.shapes[expr.ID()]
		iters := e.iterations[traceKey{id: expr.ID(), parent: trace}]
		switch shape.kind {
		case compBind:
			e.blame(expr.AsComprehension().Result(), trace, want, out)
		case compAll, compExists:
			// Each predicate value was combined into the result with `&&` or `||`, so each
			// iteration whose predicate did not evaluate to want is blamed.
			for _, iter := range iters {
				e.blame(shape.pred, iter, want, out)
			}
		case compExistsOne:
			// When no iteration matched, every iteration is responsible. Otherwise, the matching
			// iterations are, either for being one too many or for being the single match.
			matched := slices.DeleteFunc(slices.Clone(iters), func(iter int) bool {
				v, _ := e.value(shape.pred.ID(), iter)
				return v != types.True
			})
			if len(matched) == 0 {
				for _, iter := range iters {
					e.blame(shape.pred, iter, true, out)
				}
			}
			for _, iter := range matched {
				e.blame(shape.pred, iter, false, out)
			}
		default:
			return false
		}
	default:
		return false
	}
	return true
}

// blameLeaf reports expr as a violation. When expr contains comprehensions, such as the filter() in
// `items.filter(i, i.bad).size() == 0`, the iterations which contributed to the comprehension
// results are reported instead, with expr as their context.
func (e *Explanation) blameLeaf(expr celast.Expr, trace int, val ref.Val, out *[]Violation) {
	leaf := e.violation(expr, trace, val)
	before := len(*out)
	e.contributions(expr, trace, out)
	if len(*out) == before {
		*out = append(*out, leaf)
		return
	}
	for i := before; i < len(*out); i++ {
		if (*out)[i].Context == nil {
			(*out)[i].Context = &leaf
		}
	}
}

// contributions appends the iterations which contributed to the value of each outermost
// comprehension within expr, all of which were evaluated under the trace.
func (e *Explanation) contributions(expr celast.Expr, trace int, out *[]Violation) {
	if expr.Kind() != celast.ComprehensionKind {
		for _, child := range celast.NavigateExpr(e.ast, expr).Children() {
			e.contributions(child, trace, out)
		}
		return
	}
	val, found := e.value(expr.ID(), trace)
	if !found {
		return
	}
	shape := e.shapes[expr.ID()]
	iters := e.iterations[traceKey{id: expr.ID(), parent: trace}]
	switch shape.kind {
	case compBind:
		e.contributions(expr.AsComprehension().Result(), trace, out)
	case compAll, compExists, compExistsOne:
		// Explain the boolean value the quantifier produced.
		e.blame(expr, trace, val != types.True, out)
	case compFilter:
		// The iterations whose predicate held were included in the result.
		for _, iter := range iters {
			e.blame(shape.pred, iter, false, out)
		}
	default:
		// Every iteration, or every iteration whose filter predicate held, contributed to the
		// result. An iteration is identified by the value of its element, or for unrecognized
		// comprehensions and elements which were inserted into the result without being
		// constructed, by its predicate or loop step.
		for _, iter := range iters {
			if shape.pred != nil {
				if v, _ := e.value(shape.pred.ID(), iter); v != types.True {
					continue
				}
			}
			for _, x := range []celast.Expr{shape.elem, shape.pred, expr.AsComprehension().LoopStep()} {
				if x == nil {
					continue
				}
				if v, ok := e.value(x.ID(), iter); ok {
					*out = append(*out, e.violation(x, iter, v))
					break
				}
			}
		}
	}
}

func (e *Explanation) violation(expr celast.Expr, trace int, val ref.Val) Violation {
	return Violation{Path: e.path(trace), ExprID: expr.ID(), Expr: e.exprText(expr.ID()), Value: val}
}

// quantifierPredicate returns the per-iteration predicate of an all() or exists() style
// comprehension, whose loop step has the form `accu && pred` or `accu || pred`.
func quantifierPredicate(comp celast.ComprehensionExpr) celast.Expr {
	step := comp.LoopStep()
	if step.Kind() != celast.CallKind {
		return nil
	}
	fn, args := step.AsCall().FunctionName(), step.AsCall().Args()
	if (fn == operators.LogicalAnd || fn == operators.LogicalOr) && isAccuIdent(comp, args[0]) {
		return args[1]
	}
	return nil
}

// path returns the iterations enclosing and including the trace, from outermost to innermost.
func (e *Explanation) path(trace int) []IterationRef {
	var refs []IterationRef
	for t := trace; t != 0; t = e.records[t-1].Parent {
		refs = append(refs, e.iteration(t))
	}
	slices.Reverse(refs)
	return refs
}

// iteration describes the iteration begun by the record numbered trace.
func (e *Explanation) iteration(trace int) IterationRef {
	rec := e.records[trace-1]
	ref := IterationRef{ComprehensionID: rec.ID, Key: rec.Key}
	expr, found := e.exprs[rec.ID]
	if !found {
		return ref
	}
	comp := expr.AsComprehension()
	ref.Range = e.exprText(comp.IterRange().ID())
	// The element of the iteration is not recorded, but is found from the iteration range, which
	// is recorded before the iteration begins.
	rangeVal, _ := e.value(comp.IterRange().ID(), rec.Parent)
	if container, ok := rangeVal.(traits.Indexer); ok {
		ref.Value = container.Get(rec.Key)
	}
	// The first variable of a single-variable comprehension over a map is bound to the key.
	_, isList := rangeVal.(traits.Lister)
	switch {
	case comp.HasIterVar2():
		ref.elemVar = comp.IterVar2()
	case isList:
		ref.elemVar = comp.IterVar()
	}
	return ref
}

func (e *Explanation) exprText(id int64) string {
	if txt, found := e.text[id]; found {
		return txt
	}
	txt := fmt.Sprintf("<expr %d>", id)
	if expr, found := e.exprs[id]; found {
		if unparsed, err := parser.Unparse(expr, e.info, parser.WrapOnOperators()); err == nil {
			txt = unparsed
		}
	}
	e.text[id] = txt
	return txt
}

// sameExplainValue reports whether two observations are indistinguishable when rendered, treating
// errors with the same message as the same value.
func sameExplainValue(a, b ref.Val) bool {
	if aErr, ok := a.(*types.Err); ok {
		bErr, ok := b.(*types.Err)
		return ok && aErr.Error() == bErr.Error()
	}
	return a.Equal(b) == types.True
}

func formatExplainValue(val ref.Val) string {
	switch v := val.(type) {
	case nil:
		return "<unobserved>"
	case types.String:
		return strconv.Quote(string(v))
	case *types.Err:
		return "error: " + v.Error()
	default:
		return fmt.Sprintf("%v", v)
	}
}
