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

// Package matcher provides AST pattern matching capabilities for CEL.
package matcher

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/parser"
)

var patternMacros = []parser.Macro{
	parser.NewReceiverMacro("optional", 0, func(eh parser.ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		return eh.NewMemberCall("atMost", target, eh.NewLiteral(types.Int(1))), nil
	}),
	parser.NewReceiverMacro("star", 0, func(eh parser.ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		return eh.NewMemberCall("atLeast", target, eh.NewLiteral(types.Int(0))), nil
	}),
	parser.NewReceiverMacro("plus", 0, func(eh parser.ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		return eh.NewMemberCall("atLeast", target, eh.NewLiteral(types.Int(1))), nil
	}),
	parser.NewReceiverMacro("repeated", 1, func(eh parser.ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		atLeastCall := eh.NewMemberCall("atLeast", target, args[0])
		return eh.NewMemberCall("atMost", atLeastCall, args[0]), nil
	}),
}

// MatchResult provides access to captured slots for a single match.
type MatchResult interface {
	// Matched returns true if the match succeeded.
	Matched() bool

	// FirstExpr returns the first navigable expression captured by a positional or sequence slot (_0, _1, ...).
	// If the slot matched an empty sequence (e.g. _0.optional() or _0.star() matching 0 elements),
	// or if the slot was not bound, FirstExpr returns (nil, false).
	FirstExpr(slotIndex int) (ast.NavigableExpr, bool)

	// Exprs returns all navigable expressions captured by a slot (_0, _1.star(), etc.).
	// If the slot was not bound, Exprs returns (nil, false).
	Exprs(slotIndex int) ([]ast.NavigableExpr, bool)

	// FirstRawExpr returns the first captured expression as an ast.Expr without forcing NavigableExpr conversion.
	// If the slot matched an empty sequence (e.g. _0.optional() or _0.star() matching 0 elements),
	// or if the slot was not bound, FirstRawExpr returns (nil, false).
	FirstRawExpr(slotIndex int) (ast.Expr, bool)

	// RawExprs returns captured expressions as []ast.Expr without forcing NavigableExpr conversion.
	// If the slot was not bound, RawExprs returns (nil, false).
	RawExprs(slotIndex int) ([]ast.Expr, bool)
}

const (
	maxSlots           = 10
	anonymousSlotIndex = -1
)

type slotKind uint8

const (
	slotUnbound slotKind = iota
	slotSingle
	slotMulti
)

type matchResultImpl struct {
	matched     bool
	targetAST   *ast.AST
	singleSlots [maxSlots]ast.Expr
	multiSlots  [maxSlots][]ast.Expr
	slotKinds   [maxSlots]slotKind
}

func (r *matchResultImpl) Matched() bool {
	return r != nil && r.matched
}

func (r *matchResultImpl) FirstRawExpr(slotIndex int) (ast.Expr, bool) {
	if r == nil || !r.matched || slotIndex < 0 || slotIndex >= maxSlots {
		return nil, false
	}
	switch r.slotKinds[slotIndex] {
	case slotSingle:
		return r.singleSlots[slotIndex], true
	case slotMulti:
		if len(r.multiSlots[slotIndex]) == 0 {
			return nil, false
		}
		return r.multiSlots[slotIndex][0], true
	default:
		return nil, false
	}
}

func (r *matchResultImpl) FirstExpr(slotIndex int) (ast.NavigableExpr, bool) {
	raw, ok := r.FirstRawExpr(slotIndex)
	if !ok || raw == nil {
		return nil, false
	}
	if nav, ok := raw.(ast.NavigableExpr); ok {
		return nav, true
	}
	return ast.NavigateExpr(r.targetAST, raw), true
}

func (r *matchResultImpl) RawExprs(slotIndex int) ([]ast.Expr, bool) {
	if r == nil || !r.matched || slotIndex < 0 || slotIndex >= maxSlots {
		return nil, false
	}
	switch r.slotKinds[slotIndex] {
	case slotSingle:
		return []ast.Expr{r.singleSlots[slotIndex]}, true
	case slotMulti:
		return r.multiSlots[slotIndex], true
	default:
		return nil, false
	}
}

func (r *matchResultImpl) Exprs(slotIndex int) ([]ast.NavigableExpr, bool) {
	raws, ok := r.RawExprs(slotIndex)
	if !ok {
		return nil, false
	}
	res := make([]ast.NavigableExpr, len(raws))
	for i, raw := range raws {
		if nav, ok := raw.(ast.NavigableExpr); ok {
			res[i] = nav
		} else {
			res[i] = ast.NavigateExpr(r.targetAST, raw)
		}
	}
	return res, true
}

type slotDef struct {
	id           int64
	index        int
	minOccurs    int
	maxOccurs    int
	hasMinOccurs bool
	hasMaxOccurs bool
	hasKind      bool
	expectedKind ast.ExprKind
	structType   string
	hasType      bool
	typeName     string
	expectedType *types.Type
}

func (s *slotDef) isVariableCardinality() bool {
	return s.maxOccurs == -1 || s.minOccurs != s.maxOccurs
}

// Pattern represents a compiled CEL AST pattern matcher.
type Pattern struct {
	ast           *ast.AST
	slots         map[int64]*slotDef
	slotByID      []*slotDef
	macroCalls    map[int64]ast.Expr
	hasMacroCalls bool
	typeProvider  types.Provider
}

func (p *Pattern) slot(id int64) *slotDef {
	if id >= 0 && int(id) < len(p.slotByID) {
		return p.slotByID[id]
	}
	if len(p.slots) > 0 {
		return p.slots[id]
	}
	return nil
}

// AST returns the parsed AST representation of the pattern.
func (p *Pattern) AST() *ast.AST {
	return p.ast
}

// CompileOption configures pattern compilation.
type CompileOption func(*compileOptions)

type compileOptions struct {
	typeProvider  types.Provider
	parserOptions []parser.Option
}

// TypeProvider configures the type provider used during pattern compilation to validate type names.
func TypeProvider(provider types.Provider) CompileOption {
	return func(opts *compileOptions) {
		opts.typeProvider = provider
	}
}

// ParserOptions supplies additional parser options to use when compiling the pattern.
func ParserOptions(opts ...parser.Option) CompileOption {
	return func(co *compileOptions) {
		co.parserOptions = append(co.parserOptions, opts...)
	}
}

// CompileAST compiles a pre-parsed or pre-compiled AST into a Pattern instance.
func CompileAST(patternAST *ast.AST, opts ...CompileOption) (*Pattern, error) {
	if patternAST == nil {
		return nil, fmt.Errorf("patternAST must not be nil")
	}
	co := &compileOptions{}
	for _, opt := range opts {
		opt(co)
	}
	if co.typeProvider == nil {
		reg, err := types.NewRegistry()
		if err != nil {
			return nil, err
		}
		co.typeProvider = reg
	}

	var macroCalls map[int64]ast.Expr
	hasMacroCalls := false
	if patternAST.SourceInfo() != nil && len(patternAST.SourceInfo().MacroCalls()) > 0 {
		macroCalls = patternAST.SourceInfo().MacroCalls()
		hasMacroCalls = true
	}

	p := &Pattern{
		ast:           patternAST,
		slots:         make(map[int64]*slotDef),
		macroCalls:    macroCalls,
		hasMacroCalls: hasMacroCalls,
		typeProvider:  co.typeProvider,
	}

	if err := p.compileSubtree(patternAST.Expr()); err != nil {
		return nil, err
	}

	var maxID int64 = -1
	for id := range p.slots {
		if id > maxID {
			maxID = id
		}
	}
	if maxID >= 0 && maxID < 10000 {
		p.slotByID = make([]*slotDef, maxID+1)
		for id, s := range p.slots {
			p.slotByID[id] = s
		}
	}

	return p, nil
}

// MustCompileAST compiles an AST into a Pattern and panics if compilation fails.
func MustCompileAST(patternAST *ast.AST, opts ...CompileOption) *Pattern {
	p, err := CompileAST(patternAST, opts...)
	if err != nil {
		panic(err)
	}
	return p
}

// Compile parses and compiles a CEL pattern expression into a Pattern instance.
func Compile(patternSource string, opts ...CompileOption) (*Pattern, error) {
	co := &compileOptions{}
	for _, opt := range opts {
		opt(co)
	}

	allMacros := append(slices.Clone(parser.AllMacros), patternMacros...)
	pOpts := []parser.Option{
		parser.EnablePrattParser(true),
		parser.PopulateMacroCalls(true),
		parser.Macros(allMacros...),
		parser.EnableOptionalSyntax(true),
		parser.EnableVariadicOperatorASTs(true),
		parser.EnableIdentEscapeSyntax(true),
		parser.EnableCallEscapeSyntax(true),
	}
	pOpts = append(pOpts, co.parserOptions...)

	prs, err := parser.NewParser(pOpts...)
	if err != nil {
		return nil, err
	}

	source := common.NewTextSource(patternSource)
	patternAST, errors := prs.Parse(source)
	if errors != nil && len(errors.GetErrors()) > 0 {
		return nil, fmt.Errorf("pattern parse error: %s", errors.ToDisplayString())
	}

	return CompileAST(patternAST, opts...)
}

// MustCompile compiles a pattern and panics if compilation fails.
func MustCompile(patternSource string, opts ...CompileOption) *Pattern {
	p, err := Compile(patternSource, opts...)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *Pattern) compileSubtree(e ast.Expr) error {
	if e == nil {
		return nil
	}

	slot, isSlot, err := parseSlotExpr(e, p.typeProvider)
	if err != nil {
		return err
	}
	if isSlot {
		slot.id = e.ID()
		p.slots[e.ID()] = slot
		return nil
	}

	// Check if this node is a macro call recorded in MacroCalls()
	if _, ok := p.ast.SourceInfo().MacroCalls()[e.ID()]; ok {
		// When a macro call is recorded, its arguments may contain slots.
		macroExpr := p.ast.SourceInfo().MacroCalls()[e.ID()]
		if err := p.compileSubtree(macroExpr); err != nil {
			return err
		}
	}

	switch e.Kind() {
	case ast.CallKind:
		c := e.AsCall()
		if c.IsMemberFunction() {
			if err := p.compileSubtree(c.Target()); err != nil {
				return err
			}
		}
		return p.compileSequenceSubtrees(c.Args(), "argument")
	case ast.ListKind:
		return p.compileSequenceSubtrees(e.AsList().Elements(), "list element")
	case ast.SelectKind:
		return p.compileSubtree(e.AsSelect().Operand())
	case ast.MapKind:
		return p.compileMapSubtrees(e.AsMap().Entries())
	case ast.StructKind:
		for _, field := range e.AsStruct().Fields() {
			sf := field.AsStructField()
			if err := p.compileSubtree(sf.Value()); err != nil {
				return err
			}
		}
	case ast.ComprehensionKind:
		comp := e.AsComprehension()
		if err := p.compileSubtree(comp.IterRange()); err != nil {
			return err
		}
		if err := p.compileSubtree(comp.AccuInit()); err != nil {
			return err
		}
		if err := p.compileSubtree(comp.LoopCondition()); err != nil {
			return err
		}
		if err := p.compileSubtree(comp.LoopStep()); err != nil {
			return err
		}
		if err := p.compileSubtree(comp.Result()); err != nil {
			return err
		}
	}

	return nil
}

func (p *Pattern) compileSequenceSubtrees(elements []ast.Expr, containerName string) error {
	for i, elem := range elements {
		if err := p.compileSubtree(elem); err != nil {
			return err
		}
		if i < len(elements)-1 {
			if s, ok := p.slots[elem.ID()]; ok && s.isVariableCardinality() {
				return fmt.Errorf("variable quantifier is only allowed in trailing %s position: %v", containerName, elem)
			}
		}
	}
	return nil
}

func (p *Pattern) compileMapSubtrees(entries []ast.EntryExpr) error {
	for i, entry := range entries {
		me := entry.AsMapEntry()
		key := me.Key()
		val := me.Value()
		if err := p.compileSubtree(key); err != nil {
			return err
		}
		if err := p.compileSubtree(val); err != nil {
			return err
		}
		if i < len(entries)-1 {
			if s, ok := p.slots[key.ID()]; ok && s.isVariableCardinality() {
				return fmt.Errorf("variable quantifier is only allowed in trailing map entry position: %v", key)
			}
			if s, ok := p.slots[val.ID()]; ok && s.isVariableCardinality() {
				return fmt.Errorf("variable quantifier is only allowed in trailing map entry position: %v", val)
			}
		}
	}
	return nil
}

type slotBuilder struct {
	slot *slotDef
}

func newSlotBuilder(index int) *slotBuilder {
	return &slotBuilder{
		slot: &slotDef{
			index:     index,
			minOccurs: 1,
			maxOccurs: 1,
		},
	}
}

func (b *slotBuilder) atLeast(val int64) error {
	if b.slot.hasMinOccurs {
		return fmt.Errorf("atLeast or minimum quantifier already specified on slot")
	}
	if val < 0 {
		return fmt.Errorf("atLeast argument must be a non-negative integer")
	}
	b.slot.minOccurs = int(val)
	if !b.slot.hasMaxOccurs {
		b.slot.maxOccurs = -1
	}
	b.slot.hasMinOccurs = true
	return b.validate()
}

func (b *slotBuilder) atMost(val int64) error {
	if b.slot.hasMaxOccurs {
		return fmt.Errorf("atMost or maximum quantifier already specified on slot")
	}
	if val < 0 {
		return fmt.Errorf("atMost argument must be a non-negative integer")
	}
	b.slot.maxOccurs = int(val)
	if !b.slot.hasMinOccurs {
		b.slot.minOccurs = 0
	}
	b.slot.hasMaxOccurs = true
	return b.validate()
}

func (b *slotBuilder) exprKind(kind ast.ExprKind, structType string) error {
	if b.slot.hasKind {
		return fmt.Errorf("exprKind already specified on slot")
	}
	if structType != "" && kind != ast.StructKind {
		return fmt.Errorf("second argument to exprKind only allowed for struct")
	}
	b.slot.hasKind = true
	b.slot.expectedKind = kind
	b.slot.structType = structType
	return nil
}

func (b *slotBuilder) typeConstraint(typeName string, provider types.Provider) error {
	if b.slot.hasType {
		return fmt.Errorf("type already specified on slot")
	}
	b.slot.hasType = true
	b.slot.typeName = typeName
	if provider == nil {
		return nil
	}
	if val, found := provider.FindIdent(typeName); found {
		if t, ok := val.(*types.Type); ok {
			b.slot.expectedType = t
		}
	}
	if b.slot.expectedType == nil {
		if st, found := provider.FindStructType(typeName); found {
			b.slot.expectedType = st
		}
	}
	// An unresolvable type name would otherwise degrade to a literal comparison
	// against the target's type name, silently producing a pattern which can
	// never match. Report the typo at compile time instead.
	if b.slot.expectedType == nil {
		return fmt.Errorf("unknown type name in type constraint: %s", typeName)
	}
	return nil
}

func (b *slotBuilder) validate() error {
	if b.slot.hasMinOccurs && b.slot.hasMaxOccurs && b.slot.maxOccurs != -1 {
		if b.slot.minOccurs > b.slot.maxOccurs {
			return fmt.Errorf("minOccurs (%d) cannot exceed maxOccurs (%d)", b.slot.minOccurs, b.slot.maxOccurs)
		}
	}
	return nil
}

func (b *slotBuilder) build() *slotDef {
	return b.slot
}

func parseNonNegativeIntLiteral(fnName string, args []ast.Expr) (int64, error) {
	if len(args) != 1 || args[0].Kind() != ast.LiteralKind {
		return 0, fmt.Errorf("%s requires 1 integer literal argument", fnName)
	}
	val, ok := args[0].AsLiteral().Value().(int64)
	if !ok || val < 0 {
		return 0, fmt.Errorf("%s argument must be a non-negative integer", fnName)
	}
	return val, nil
}

func parseSlotExpr(e ast.Expr, provider types.Provider) (*slotDef, bool, error) {
	b, ok, err := parseSlotBuilder(e, provider)
	if err != nil || !ok {
		return nil, ok, err
	}
	return b.build(), true, nil
}

func parseSlotBuilder(e ast.Expr, provider types.Provider) (*slotBuilder, bool, error) {
	if e == nil {
		return nil, false, nil
	}

	if e.Kind() == ast.IdentKind {
		name := e.AsIdent()
		if name == "_" {
			return newSlotBuilder(anonymousSlotIndex), true, nil
		}
		if strings.HasPrefix(name, "_") {
			idxStr := name[1:]
			if idx, err := strconv.Atoi(idxStr); err == nil && idx >= 0 {
				if idx >= maxSlots {
					return nil, false, fmt.Errorf("slot index out of range: %s (must be in range _0.._%d)", name, maxSlots-1)
				}
				return newSlotBuilder(idx), true, nil
			}
		}
		return nil, false, nil
	}

	if e.Kind() == ast.CallKind && e.AsCall().IsMemberFunction() {
		call := e.AsCall()
		fn := call.FunctionName()
		b, isSlot, err := parseSlotBuilder(call.Target(), provider)
		if err != nil || !isSlot {
			return nil, false, err
		}

		switch fn {
		case "atLeast":
			val, err := parseNonNegativeIntLiteral("atLeast", call.Args())
			if err != nil {
				return nil, false, err
			}
			if err := b.atLeast(val); err != nil {
				return nil, false, err
			}
		case "atMost":
			val, err := parseNonNegativeIntLiteral("atMost", call.Args())
			if err != nil {
				return nil, false, err
			}
			if err := b.atMost(val); err != nil {
				return nil, false, err
			}
		case "exprKind":
			if len(call.Args()) < 1 || len(call.Args()) > 2 {
				return nil, false, fmt.Errorf("exprKind requires 1 or 2 arguments")
			}
			kindName, err := toQualifiedName(call.Args()[0])
			if err != nil {
				return nil, false, fmt.Errorf("invalid kind identifier for exprKind: %w", err)
			}
			k, err := parseExprKind(kindName)
			if err != nil {
				return nil, false, err
			}
			var structTypeName string
			if len(call.Args()) == 2 {
				var err error
				structTypeName, err = toQualifiedName(call.Args()[1])
				if err != nil {
					return nil, false, fmt.Errorf("invalid struct type name in exprKind: %w", err)
				}
			}
			if err := b.exprKind(k, structTypeName); err != nil {
				return nil, false, err
			}
		case "type":
			if len(call.Args()) != 1 {
				return nil, false, fmt.Errorf("type requires 1 argument")
			}
			typeName, err := toQualifiedName(call.Args()[0])
			if err != nil {
				return nil, false, fmt.Errorf("invalid type identifier for type: %w", err)
			}
			if err := b.typeConstraint(typeName, provider); err != nil {
				return nil, false, err
			}
		default:
			return nil, false, nil
		}
		return b, true, nil
	}

	return nil, false, nil
}

func parseExprKind(name string) (ast.ExprKind, error) {
	switch name {
	case "ident":
		return ast.IdentKind, nil
	case "call":
		return ast.CallKind, nil
	case "select":
		return ast.SelectKind, nil
	case "list":
		return ast.ListKind, nil
	case "map":
		return ast.MapKind, nil
	case "struct":
		return ast.StructKind, nil
	case "literal":
		return ast.LiteralKind, nil
	case "comprehension":
		return ast.ComprehensionKind, nil
	default:
		return ast.UnspecifiedExprKind, fmt.Errorf("unrecognized expression kind: %s", name)
	}
}

func toQualifiedName(e ast.Expr) (string, error) {
	switch e.Kind() {
	case ast.IdentKind:
		return e.AsIdent(), nil
	case ast.SelectKind:
		op := e.AsSelect().Operand()
		prefix, err := toQualifiedName(op)
		if err != nil {
			return "", err
		}
		return prefix + "." + e.AsSelect().FieldName(), nil
	default:
		return "", fmt.Errorf("expected identifier or qualified name, got %v", e.Kind())
	}
}

// MatchOption configures repeated AST matching behavior.
type MatchOption func(*matchOptions)

type matchOptions struct {
	disjoint  bool
	maxDepth  int
	equivOpts []ast.EquivOption
}

// MatchDisjoint prevents matching inside already matched subtrees during in-order traversal.
func MatchDisjoint(enabled ...bool) MatchOption {
	return func(opts *matchOptions) {
		if len(enabled) == 0 {
			opts.disjoint = true
			return
		}
		opts.disjoint = enabled[0]
	}
}

// MatchMaxDepth limits repeated matching traversal to expressions at depth <= maxDepth.
func MatchMaxDepth(maxDepth int) MatchOption {
	return func(opts *matchOptions) {
		opts.maxDepth = maxDepth
	}
}

// MatchEquivOptions passes equivalence options to the underlying structural equivalence checks.
func MatchEquivOptions(equivOpts ...ast.EquivOption) MatchOption {
	return func(opts *matchOptions) {
		opts.equivOpts = append(opts.equivOpts, equivOpts...)
	}
}

// Match checks if the root of the target AST matches the pattern.
func (p *Pattern) Match(target *ast.AST, opts ...ast.EquivOption) (MatchResult, bool) {
	if target == nil {
		return nil, false
	}
	// Reject incompatible roots before wrapping the AST in a navigable expression,
	// which would otherwise allocate even when the match cannot possibly succeed.
	if p.rootKindMismatch(p.ast.Expr(), target.Expr()) {
		return nil, false
	}
	navRoot := ast.NavigateAST(target)
	return p.matchExprInternal(p.ast.Expr(), navRoot, target, opts)
}

// rootKindMismatch indicates whether the pattern root can be rejected based solely
// on its expression kind, without traversing or navigating the target.
//
// Unconstrained slots and macro calls match across expression kinds, so they are
// never rejected here. Slot type constraints require type metadata from a
// navigated or checked AST and are deferred to the match itself.
func (p *Pattern) rootKindMismatch(patExpr, targetExpr ast.Expr) bool {
	if patExpr == nil || targetExpr == nil {
		return false
	}
	if slot := p.slot(patExpr.ID()); slot != nil {
		return !slotKindMatches(slot, targetExpr)
	}
	if p.hasMacroCalls && p.macroCalls[patExpr.ID()] != nil {
		return false
	}
	return patExpr.Kind() != targetExpr.Kind()
}

// MatchExpr checks if the target expression matches the pattern root.
func (p *Pattern) MatchExpr(target ast.Expr, opts ...ast.EquivOption) (MatchResult, bool) {
	if target == nil {
		return nil, false
	}
	return p.matchExprInternal(p.ast.Expr(), target, nil, opts)
}

// FindAll searches the entire AST and returns all matching sub-expressions.
func (p *Pattern) FindAll(target *ast.AST, opts ...MatchOption) []MatchResult {
	if target == nil {
		return nil
	}
	var results []MatchResult
	p.MatchAll(target, func(m MatchResult) bool {
		results = append(results, m)
		return true
	}, opts...)
	return results
}

// FindAllExpr searches the entire expression tree and returns all matching sub-expressions.
func (p *Pattern) FindAllExpr(target ast.Expr, opts ...MatchOption) []MatchResult {
	if target == nil {
		return nil
	}
	var results []MatchResult
	p.MatchAllExpr(target, func(m MatchResult) bool {
		results = append(results, m)
		return true
	}, opts...)
	return results
}

// MatchAll traverses the AST in-order and invokes handler for each match.
func (p *Pattern) MatchAll(target *ast.AST, handler func(MatchResult) bool, opts ...MatchOption) {
	if target == nil {
		return
	}
	navRoot := ast.NavigateAST(target)
	p.matchAllExpr(navRoot, target, handler, opts...)
}

// MatchAllExpr traverses the expression tree in-order and invokes handler for each match.
func (p *Pattern) MatchAllExpr(target ast.Expr, handler func(MatchResult) bool, opts ...MatchOption) {
	if target == nil {
		return
	}
	p.matchAllExpr(target, nil, handler, opts...)
}

func (p *Pattern) matchAllExpr(root ast.Expr, targetAST *ast.AST, handler func(MatchResult) bool, opts ...MatchOption) {
	mo := &matchOptions{maxDepth: -1}
	for _, opt := range opts {
		opt(mo)
	}

	var walk func(e ast.Expr, depth int) bool
	walk = func(e ast.Expr, depth int) bool {
		if e == nil {
			return true
		}
		if mo.maxDepth >= 0 && depth > mo.maxDepth {
			return true
		}

		if res, ok := p.matchExprInternal(p.ast.Expr(), e, targetAST, mo.equivOpts); ok {
			if !handler(res) {
				return false
			}
			if mo.disjoint {
				return true
			}
		}

		switch e.Kind() {
		case ast.CallKind:
			c := e.AsCall()
			if c.IsMemberFunction() {
				if !walk(c.Target(), depth+1) {
					return false
				}
			}
			for _, arg := range c.Args() {
				if !walk(arg, depth+1) {
					return false
				}
			}
		case ast.ListKind:
			for _, elem := range e.AsList().Elements() {
				if !walk(elem, depth+1) {
					return false
				}
			}
		case ast.SelectKind:
			if !walk(e.AsSelect().Operand(), depth+1) {
				return false
			}
		case ast.MapKind:
			for _, entry := range e.AsMap().Entries() {
				me := entry.AsMapEntry()
				if !walk(me.Key(), depth+1) || !walk(me.Value(), depth+1) {
					return false
				}
			}
		case ast.StructKind:
			for _, field := range e.AsStruct().Fields() {
				if !walk(field.AsStructField().Value(), depth+1) {
					return false
				}
			}
		case ast.ComprehensionKind:
			comp := e.AsComprehension()
			if !walk(comp.IterRange(), depth+1) ||
				!walk(comp.AccuInit(), depth+1) ||
				!walk(comp.LoopCondition(), depth+1) ||
				!walk(comp.LoopStep(), depth+1) ||
				!walk(comp.Result(), depth+1) {
				return false
			}
		}
		return true
	}

	walk(root, 0)
}

type matchContext struct {
	pattern           *Pattern
	targetAST         *ast.AST
	singleSlots       [maxSlots]ast.Expr
	multiSlots        [maxSlots][]ast.Expr
	slotKinds         [maxSlots]slotKind
	equivOpts         []ast.EquivOption
	ignoreIdentifiers bool
	ignored1          []string
	ignored2          []string
}

func (ctx *matchContext) reset() {
	ctx.pattern = nil
	ctx.targetAST = nil
	for i := range ctx.slotKinds {
		ctx.singleSlots[i] = nil
		ctx.multiSlots[i] = nil
		ctx.slotKinds[i] = slotUnbound
	}
	ctx.equivOpts = nil
	ctx.ignoreIdentifiers = false
	ctx.ignored1 = ctx.ignored1[:0]
	ctx.ignored2 = ctx.ignored2[:0]
}

var matchContextPool = sync.Pool{
	New: func() any {
		return &matchContext{
			ignored1: make([]string, 0, 8),
			ignored2: make([]string, 0, 8),
		}
	},
}

func (ctx *matchContext) pushIgnored(comp1, comp2 ast.ComprehensionExpr) int {
	prev := len(ctx.ignored1)
	if comp1.IterVar() != "" {
		ctx.ignored1 = append(ctx.ignored1, comp1.IterVar())
	}
	if comp1.HasIterVar2() {
		ctx.ignored1 = append(ctx.ignored1, comp1.IterVar2())
	}
	if comp1.AccuVar() != "" {
		ctx.ignored1 = append(ctx.ignored1, comp1.AccuVar())
	}

	if comp2.IterVar() != "" {
		ctx.ignored2 = append(ctx.ignored2, comp2.IterVar())
	}
	if comp2.HasIterVar2() {
		ctx.ignored2 = append(ctx.ignored2, comp2.IterVar2())
	}
	if comp2.AccuVar() != "" {
		ctx.ignored2 = append(ctx.ignored2, comp2.AccuVar())
	}
	return prev
}

func (ctx *matchContext) popIgnored(prev int) {
	ctx.ignored1 = ctx.ignored1[:prev]
	ctx.ignored2 = ctx.ignored2[:prev]
}

func (ctx *matchContext) isIdentEqual(name1, name2 string) bool {
	if len(ctx.ignored1) == 0 && len(ctx.ignored2) == 0 {
		return name1 == name2
	}
	idx1 := lastIgnored(ctx.ignored1, name1)
	idx2 := lastIgnored(ctx.ignored2, name2)
	if idx1 >= 0 || idx2 >= 0 {
		return idx1 == idx2
	}
	return name1 == name2
}

func lastIgnored(stack []string, name string) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == name {
			return i
		}
	}
	return -1
}

var defaultEquivOptions = []ast.EquivOption{
	ast.EquivIgnoreIdentifiers(true),
	ast.EquivMacroCalls(true),
}

func resolveEquivOptions(equivOpts []ast.EquivOption) ([]ast.EquivOption, bool) {
	if len(equivOpts) == 0 {
		return defaultEquivOptions, true
	}
	combined := make([]ast.EquivOption, 0, len(defaultEquivOptions)+len(equivOpts))
	combined = append(combined, defaultEquivOptions...)
	combined = append(combined, equivOpts...)
	ignoreIdents := ast.IsEquivIgnoreIdentifiers(combined...)
	return combined, ignoreIdents
}

func (p *Pattern) matchExprInternal(patExpr, targetExpr ast.Expr, targetAST *ast.AST, equivOpts []ast.EquivOption) (MatchResult, bool) {
	if patExpr == nil || targetExpr == nil {
		return nil, patExpr == targetExpr
	}

	// Fast-path root check: if patExpr is not a slot and not a macro recorded in MacroCalls(),
	// its Kind must match targetExpr.Kind().
	if p.rootKindMismatch(patExpr, targetExpr) {
		return nil, false
	}

	ctx := matchContextPool.Get().(*matchContext)
	ctx.pattern = p
	ctx.targetAST = targetAST
	ctx.equivOpts, ctx.ignoreIdentifiers = resolveEquivOptions(equivOpts)

	if !ctx.matchNode(patExpr, targetExpr) {
		ctx.reset()
		matchContextPool.Put(ctx)
		return nil, false
	}

	res := &matchResultImpl{
		matched:     true,
		targetAST:   targetAST,
		singleSlots: ctx.singleSlots,
		multiSlots:  ctx.multiSlots,
		slotKinds:   ctx.slotKinds,
	}
	ctx.reset()
	matchContextPool.Put(ctx)
	return res, true
}

func (ctx *matchContext) matchNode(patExpr, targetExpr ast.Expr) bool {
	if patExpr == nil || targetExpr == nil {
		return patExpr == targetExpr
	}
	// If this is a slot `_\d`, then attempt to greedily match the content to the slot
	// expression.
	if slot := ctx.pattern.slot(patExpr.ID()); slot != nil {
		return ctx.matchSlot(slot, targetExpr)
	}

	// If the pattern contains macro calls like `_.exists(_, _)`, then attempt to match
	// the macro call structure rather than the macro-expanded AST.
	if ctx.pattern.hasMacroCalls {
		if patMacro, ok := ctx.pattern.macroCalls[patExpr.ID()]; ok {
			var targetMacro ast.Expr
			if ctx.targetAST != nil && ctx.targetAST.SourceInfo() != nil {
				targetMacro = ctx.targetAST.SourceInfo().MacroCalls()[targetExpr.ID()]
			}
			if targetMacro != nil {
				// Comprehension variable names are only unified when the caller
				// permits alpha-equivalence. Otherwise the iteration variables
				// recorded in the macro call are compared by name like any other
				// identifier.
				if ctx.ignoreIdentifiers &&
					patExpr.Kind() == ast.ComprehensionKind && targetExpr.Kind() == ast.ComprehensionKind {
					prev := ctx.pushIgnored(patExpr.AsComprehension(), targetExpr.AsComprehension())
					defer ctx.popIgnored(prev)
				}
				return ctx.matchNode(patMacro, targetMacro)
			}
		}
	}

	// If expression node kinds don't agree, then this isn't a match.
	if patExpr.Kind() != targetExpr.Kind() {
		return false
	}

	switch patExpr.Kind() {
	case ast.IdentKind:
		return ctx.isIdentEqual(patExpr.AsIdent(), targetExpr.AsIdent())
	case ast.LiteralKind:
		return ast.EquivExpr(patExpr, targetExpr, ctx.equivOpts...)
	case ast.SelectKind:
		pSel := patExpr.AsSelect()
		tSel := targetExpr.AsSelect()
		if pSel.FieldName() != tSel.FieldName() || pSel.IsTestOnly() != tSel.IsTestOnly() {
			return false
		}
		return ctx.matchNode(pSel.Operand(), tSel.Operand())
	case ast.CallKind:
		pCall := patExpr.AsCall()
		tCall := targetExpr.AsCall()
		pArgs := pCall.Args()
		tArgs := tCall.Args()
		pLen := len(pArgs)
		tLen := len(tArgs)

		// Attempt to match qualified function names in the parsed pattern (spread over several exprs)
		// with qualified function names in the target AST which may be a single ident.
		if pCall.FunctionName() != tCall.FunctionName() || pCall.IsMemberFunction() != tCall.IsMemberFunction() {
			if pCall.IsMemberFunction() && !tCall.IsMemberFunction() && pCall.Target() != nil && pCall.Target().Kind() == ast.IdentKind {
				qualName := pCall.Target().AsIdent() + "." + pCall.FunctionName()
				if qualName == tCall.FunctionName() {
					return ctx.matchSequence(pArgs, tArgs)
				}
			}
			return false
		}

		// Fast arity pre-check if pattern call does not end in a variable slot
		if pLen == 0 {
			if tLen != 0 {
				return false
			}
		} else {
			lastSlot := ctx.pattern.slot(pArgs[pLen-1].ID())
			if lastSlot == nil || !lastSlot.isVariableCardinality() {
				if ctx.countFixedElements(pArgs) != tLen {
					return false
				}
			}
		}

		if pCall.IsMemberFunction() {
			if !ctx.matchNode(pCall.Target(), tCall.Target()) {
				return false
			}
		}
		return ctx.matchSequence(pArgs, tArgs)
	case ast.ListKind:
		pList := patExpr.AsList()
		tList := targetExpr.AsList()
		return ctx.matchSequence(pList.Elements(), tList.Elements())
	case ast.MapKind:
		pMap := patExpr.AsMap()
		tMap := targetExpr.AsMap()
		return ctx.matchMapSequence(pMap.Entries(), tMap.Entries())
	case ast.StructKind:
		pStruct := patExpr.AsStruct()
		tStruct := targetExpr.AsStruct()
		if pStruct.TypeName() != tStruct.TypeName() {
			return false
		}
		pFields := pStruct.Fields()
		tFields := tStruct.Fields()
		if len(pFields) != len(tFields) {
			return false
		}
		for i, pf := range pFields {
			tf := tFields[i]
			psf := pf.AsStructField()
			tsf := tf.AsStructField()
			if psf.Name() != tsf.Name() || !ctx.matchNode(psf.Value(), tsf.Value()) {
				return false
			}
		}
		return true
	case ast.ComprehensionKind:
		pComp := patExpr.AsComprehension()
		tComp := targetExpr.AsComprehension()
		if pComp.HasIterVar2() != tComp.HasIterVar2() {
			return false
		}
		if (pComp.IterVar() == "") != (tComp.IterVar() == "") ||
			(pComp.AccuVar() == "") != (tComp.AccuVar() == "") {
			return false
		}
		if !ctx.ignoreIdentifiers {
			if pComp.IterVar() != tComp.IterVar() ||
				pComp.IterVar2() != tComp.IterVar2() ||
				pComp.AccuVar() != tComp.AccuVar() {
				return false
			}
		}
		if !ctx.matchNode(pComp.IterRange(), tComp.IterRange()) ||
			!ctx.matchNode(pComp.AccuInit(), tComp.AccuInit()) {
			return false
		}
		var prev int
		if ctx.ignoreIdentifiers {
			prev = ctx.pushIgnored(pComp, tComp)
		}
		matched := ctx.matchNode(pComp.LoopCondition(), tComp.LoopCondition()) &&
			ctx.matchNode(pComp.LoopStep(), tComp.LoopStep()) &&
			ctx.matchNode(pComp.Result(), tComp.Result())
		if ctx.ignoreIdentifiers {
			ctx.popIgnored(prev)
		}
		return matched
	default:
		return ast.EquivExpr(patExpr, targetExpr, ctx.equivOpts...)
	}
}

func (ctx *matchContext) matchSlot(slot *slotDef, targetExpr ast.Expr) bool {
	if !ctx.checkSlotConstraints(slot, targetExpr) {
		return false
	}
	if slot.index == anonymousSlotIndex {
		return true
	}
	if slot.index < 0 || slot.index >= maxSlots {
		return false
	}

	switch ctx.slotKinds[slot.index] {
	case slotUnbound:
		// This is the first occurrence of the slot
		ctx.singleSlots[slot.index] = targetExpr
		ctx.slotKinds[slot.index] = slotSingle
		return true
	case slotSingle:
		// Previously a single capture expression was encountered, check for node equivalence.
		return ast.EquivExpr(ctx.singleSlots[slot.index], targetExpr, ctx.equivOpts...)
	case slotMulti:
		// Multi-capture slots can't unify with single capture slots, so `fn(_1.star()) == _1`
		// would cause problems for the multi-capture expression when compared to the single-capture.
		if len(ctx.multiSlots[slot.index]) != 1 {
			return false
		}
		return ast.EquivExpr(ctx.multiSlots[slot.index][0], targetExpr, ctx.equivOpts...)
	}
	return false
}

func (ctx *matchContext) bindSlotSlice(slot *slotDef, slice []ast.Expr) bool {
	if len(slice) == 1 {
		return ctx.matchSlot(slot, slice[0])
	}
	for _, elem := range slice {
		if !ctx.checkSlotConstraints(slot, elem) {
			return false
		}
	}
	if slot.index == anonymousSlotIndex {
		return true
	}
	if slot.index < 0 || slot.index >= maxSlots {
		return false
	}

	switch ctx.slotKinds[slot.index] {
	case slotUnbound:
		ctx.multiSlots[slot.index] = slice
		ctx.slotKinds[slot.index] = slotMulti
		return true
	case slotSingle:
		// Single-element slot was previously bound, but this slice has len != 1.
		return false
	case slotMulti:
		prev := ctx.multiSlots[slot.index]
		if len(prev) != len(slice) {
			return false
		}
		for i := range prev {
			if !ast.EquivExpr(prev[i], slice[i], ctx.equivOpts...) {
				return false
			}
		}
		return true
	}
	return false
}

func (ctx *matchContext) countFixedElements(elements []ast.Expr) int {
	fixedCount := 0
	for _, pe := range elements {
		if s := ctx.pattern.slot(pe.ID()); s != nil && s.hasMinOccurs && s.minOccurs == s.maxOccurs {
			fixedCount += s.minOccurs
		} else {
			fixedCount++
		}
	}
	return fixedCount
}

// slotKindMatches reports whether the expression satisfies the slot's expression
// kind constraint. Kind constraints are decidable without type metadata, so they
// may be evaluated before an AST is navigated.
func slotKindMatches(slot *slotDef, targetExpr ast.Expr) bool {
	if !slot.hasKind {
		return true
	}
	if targetExpr.Kind() != slot.expectedKind {
		return false
	}
	if slot.structType != "" && targetExpr.Kind() == ast.StructKind {
		return targetExpr.AsStruct().TypeName() == slot.structType
	}
	return true
}

func (ctx *matchContext) checkSlotConstraints(slot *slotDef, targetExpr ast.Expr) bool {
	if !slotKindMatches(slot, targetExpr) {
		return false
	}

	if slot.hasType {
		var targetType *types.Type
		if ctx.targetAST != nil {
			targetType = ctx.targetAST.TypeMap()[targetExpr.ID()]
		}
		if targetType == nil {
			if nav, ok := targetExpr.(ast.NavigableExpr); ok {
				targetType = nav.Type()
			}
		}
		if targetType != nil && slot.expectedType != nil {
			if !targetType.IsEquivalentType(slot.expectedType) {
				return false
			}
		} else if targetType != nil && slot.typeName != "" {
			if targetType.TypeName() != slot.typeName {
				return false
			}
		}
	}

	return true
}

func (ctx *matchContext) matchSequence(patElements, targetElements []ast.Expr) bool {
	pLen := len(patElements)
	tLen := len(targetElements)

	if pLen == 0 {
		return tLen == 0
	}

	lastPat := patElements[pLen-1]
	lastSlot := ctx.pattern.slot(lastPat.ID())
	hasTrailingSlot := lastSlot != nil
	isTrailingVariable := hasTrailingSlot && lastSlot.isVariableCardinality()

	if !isTrailingVariable {
		if ctx.countFixedElements(patElements) != tLen {
			return false
		}

		tIdx := 0
		for _, pe := range patElements {
			if s := ctx.pattern.slot(pe.ID()); s != nil && s.hasMinOccurs && s.minOccurs == s.maxOccurs {
				count := s.minOccurs
				if !ctx.bindSlotSlice(s, targetElements[tIdx:tIdx+count]) {
					return false
				}
				tIdx += count
			} else {
				if !ctx.matchNode(pe, targetElements[tIdx]) {
					return false
				}
				tIdx++
			}
		}
		return true
	}

	// Has trailing variable quantifier on lastPat
	fixedLeadingCount := ctx.countFixedElements(patElements[:pLen-1])
	if tLen < fixedLeadingCount {
		return false
	}

	remCount := tLen - fixedLeadingCount
	if remCount < lastSlot.minOccurs {
		return false
	}
	if lastSlot.maxOccurs != -1 && remCount > lastSlot.maxOccurs {
		return false
	}

	// Match leading fixed pattern elements
	tIdx := 0
	for _, pe := range patElements[:pLen-1] {
		if s := ctx.pattern.slot(pe.ID()); s != nil && s.hasMinOccurs && s.minOccurs == s.maxOccurs {
			count := s.minOccurs
			if !ctx.bindSlotSlice(s, targetElements[tIdx:tIdx+count]) {
				return false
			}
			tIdx += count
		} else {
			if !ctx.matchNode(pe, targetElements[tIdx]) {
				return false
			}
			tIdx++
		}
	}

	// Match trailing slice with lastSlot
	return ctx.bindSlotSlice(lastSlot, targetElements[tIdx:])
}

func (ctx *matchContext) entryFixedCount(pe ast.MapEntry) int {
	kSlot := ctx.pattern.slot(pe.Key().ID())
	vSlot := ctx.pattern.slot(pe.Value().ID())
	kOk := kSlot != nil
	vOk := vSlot != nil
	kCount := 1
	if kOk && kSlot.hasMinOccurs && kSlot.minOccurs == kSlot.maxOccurs {
		kCount = kSlot.minOccurs
	}
	vCount := 1
	if vOk && vSlot.hasMinOccurs && vSlot.minOccurs == vSlot.maxOccurs {
		vCount = vSlot.minOccurs
	}
	if kOk && vOk && kSlot.hasMinOccurs && kSlot.minOccurs == kSlot.maxOccurs && vSlot.hasMinOccurs && vSlot.minOccurs == vSlot.maxOccurs {
		if kCount != vCount {
			return -1
		}
	}
	if kOk && kSlot.hasMinOccurs && kSlot.minOccurs == kSlot.maxOccurs {
		return kCount
	}
	if vOk && vSlot.hasMinOccurs && vSlot.minOccurs == vSlot.maxOccurs {
		return vCount
	}
	return 1
}

func (ctx *matchContext) countFixedMapEntries(entries []ast.EntryExpr) int {
	fixedCount := 0
	for _, entry := range entries {
		pe := entry.AsMapEntry()
		c := ctx.entryFixedCount(pe)
		if c < 0 {
			return -1
		}
		fixedCount += c
	}
	return fixedCount
}

func (ctx *matchContext) matchMapEntryRange(pe ast.MapEntry, targetEntries []ast.EntryExpr) bool {
	if len(targetEntries) == 1 {
		te := targetEntries[0].AsMapEntry()
		if pe.IsOptional() != te.IsOptional() {
			return false
		}
		return ctx.matchNode(pe.Key(), te.Key()) && ctx.matchNode(pe.Value(), te.Value())
	}
	targetKeys := make([]ast.Expr, len(targetEntries))
	targetVals := make([]ast.Expr, len(targetEntries))
	for i, te := range targetEntries {
		tme := te.AsMapEntry()
		if pe.IsOptional() != tme.IsOptional() {
			return false
		}
		targetKeys[i] = tme.Key()
		targetVals[i] = tme.Value()
	}
	if kSlot := ctx.pattern.slot(pe.Key().ID()); kSlot != nil {
		if !ctx.bindSlotSlice(kSlot, targetKeys) {
			return false
		}
	} else {
		for _, k := range targetKeys {
			if !ctx.matchNode(pe.Key(), k) {
				return false
			}
		}
	}
	if vSlot := ctx.pattern.slot(pe.Value().ID()); vSlot != nil {
		if !ctx.bindSlotSlice(vSlot, targetVals) {
			return false
		}
	} else {
		for _, v := range targetVals {
			if !ctx.matchNode(pe.Value(), v) {
				return false
			}
		}
	}
	return true
}

func (ctx *matchContext) matchMapSequence(patEntries, targetEntries []ast.EntryExpr) bool {
	pLen := len(patEntries)
	tLen := len(targetEntries)

	if pLen == 0 {
		return tLen == 0
	}

	lastPat := patEntries[pLen-1].AsMapEntry()
	lastKeySlot := ctx.pattern.slot(lastPat.Key().ID())
	lastValSlot := ctx.pattern.slot(lastPat.Value().ID())
	hasTrailingKeySlot := lastKeySlot != nil
	hasTrailingValSlot := lastValSlot != nil
	isTrailingVariable := (hasTrailingKeySlot && lastKeySlot.isVariableCardinality()) || (hasTrailingValSlot && lastValSlot.isVariableCardinality())

	if !isTrailingVariable {
		if ctx.countFixedMapEntries(patEntries) != tLen {
			return false
		}

		tIdx := 0
		for _, pe := range patEntries {
			pme := pe.AsMapEntry()
			count := ctx.entryFixedCount(pme)
			if count < 0 || tIdx+count > tLen {
				return false
			}
			if !ctx.matchMapEntryRange(pme, targetEntries[tIdx:tIdx+count]) {
				return false
			}
			tIdx += count
		}
		return true
	}

	// Has trailing variable quantifier
	fixedLeadingCount := ctx.countFixedMapEntries(patEntries[:pLen-1])
	if fixedLeadingCount < 0 || tLen < fixedLeadingCount {
		return false
	}

	remCount := tLen - fixedLeadingCount
	if hasTrailingKeySlot {
		if remCount < lastKeySlot.minOccurs {
			return false
		}
		if lastKeySlot.maxOccurs != -1 && remCount > lastKeySlot.maxOccurs {
			return false
		}
	}

	if hasTrailingValSlot {
		if remCount < lastValSlot.minOccurs {
			return false
		}
		if lastValSlot.maxOccurs != -1 && remCount > lastValSlot.maxOccurs {
			return false
		}
	}

	// Match leading fixed pattern entries
	tIdx := 0
	for _, pe := range patEntries[:pLen-1] {
		pme := pe.AsMapEntry()
		count := ctx.entryFixedCount(pme)
		if count < 0 || tIdx+count > tLen {
			return false
		}
		if !ctx.matchMapEntryRange(pme, targetEntries[tIdx:tIdx+count]) {
			return false
		}
		tIdx += count
	}

	// Match trailing map entries
	return ctx.matchMapEntryRange(lastPat, targetEntries[tIdx:])
}
