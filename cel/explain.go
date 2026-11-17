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
	"strconv"
	"strings"

	celast "cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
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
