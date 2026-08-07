// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
)

type SemanticType int

const (
	unspecified SemanticType = iota
	firstMatch
	aggregate
)

// NewPolicy creates a policy object which references a policy source and source information.
func NewPolicy(src *Source, info *ast.SourceInfo) *Policy {
	return &Policy{
		metadata: map[string]any{},
		source:   src,
		info:     info,
		semantic: unspecified,
		imports:  []*Import{},
	}
}

// Policy declares a name, rule, and evaluation semantic for a given expression graph.
type Policy struct {
	name        ValueString
	description ValueString
	imports     []*Import
	rule        *Rule
	semantic    SemanticType
	info        *ast.SourceInfo
	source      *Source

	metadata map[string]any
}

// Semantic returns the evaluation semantic for the policy.
func (p *Policy) Semantic() SemanticType {
	if p.semantic == unspecified {
		return firstMatch
	}
	return p.semantic
}

// SetSemantic configures the evaluation semantic for the policy.
func (p *Policy) SetSemantic(s SemanticType) {
	if p.semantic != unspecified && p.semantic != s {
		return
	}
	p.semantic = s
}

// Source returns the policy file contents as a CEL source object.
func (p *Policy) Source() *Source {
	return p.source
}

// SourceInfo returns the policy file metadata about expression positions.
func (p *Policy) SourceInfo() *ast.SourceInfo {
	return p.info
}

// Imports returns the list of imports associated with the policy.
func (p *Policy) Imports() []*Import {
	return p.imports
}

// Name returns the name of the policy.
func (p *Policy) Name() ValueString {
	return p.name
}

// Description returns the description for the policy.
func (p *Policy) Description() ValueString {
	return p.description
}

// Rule returns the rule entry point of the policy.
func (p *Policy) Rule() *Rule {
	return p.rule
}

// Metadata returns a named metadata object if one exists within the policy.
func (p *Policy) Metadata(name string) (any, bool) {
	value, found := p.metadata[name]
	return value, found
}

// MetadataKeys returns a list of metadata keys set on the policy.
func (p *Policy) MetadataKeys() []string {
	keys := make([]string, 0, len(p.metadata))
	for k := range p.metadata {
		keys = append(keys, k)
	}
	return keys
}

// AddImport adds an import to the policy.
func (p *Policy) AddImport(i *Import) {
	p.imports = append(p.imports, i)
}

// SetName configures the policy name.
func (p *Policy) SetName(name ValueString) {
	p.name = name
}

// SetDescription configures the policy description.
func (p *Policy) SetDescription(description ValueString) {
	p.description = description
}

// SetRule configures the policy rule entry point.
func (p *Policy) SetRule(r *Rule) {
	p.rule = r
}

// SetMetadata updates a named metadata key with the given value.
func (p *Policy) SetMetadata(name string, value any) {
	p.metadata[name] = value
}

// ClearMetadata removes a named metadata key from the policy.
func (p *Policy) ClearMetadata(name string) {
	delete(p.metadata, name)
}

// GetExplanationOutputPolicy returns a copy of the policy, except the output of each match block
// is replaced by the expression in the explanation field.
func (p *Policy) GetExplanationOutputPolicy() *Policy {
	ep := Policy{
		name:     p.name,
		semantic: p.semantic,
		info:     p.info,
		metadata: p.metadata,
		source:   p.source,
	}
	if p.rule != nil {
		ep.rule = p.rule.getExplanationOutputRule()
	}
	return &ep
}

// NewImport creates a new typename import node
func NewImport(exprID int64) *Import {
	return &Import{exprID: exprID}
}

// Import represents an imported type name which is aliased within CEL expressions.
type Import struct {
	exprID int64
	name   ValueString
}

// SourceID returns the source identifier associated with the import.
func (i *Import) SourceID() int64 {
	return i.exprID
}

// Name returns the fully qualified type name.
func (i *Import) Name() ValueString {
	return i.name
}

// SetName updates the fully qualified type name for the import.
func (i *Import) SetName(name ValueString) {
	i.name = name
}

// NewRule creates a Rule instance.
func NewRule(exprID int64) *Rule {
	return &Rule{
		exprID:    exprID,
		variables: []*Variable{},
		matches:   []*Match{},
		semantic:  unspecified,
	}
}

// Rule declares a rule identifier, description, along with a set of variables and match statements.
type Rule struct {
	exprID      int64
	id          *ValueString
	description *ValueString
	variables   []*Variable
	matches     []*Match
	semantic    SemanticType
}

// Semantic returns the evaluation semantic for the rule.
func (r *Rule) Semantic() SemanticType {
	if r.semantic == unspecified {
		return firstMatch
	}
	return r.semantic
}

// SetSemantic configures the evaluation semantic for the rule.
func (r *Rule) SetSemantic(s SemanticType) {
	if r.semantic != unspecified && r.semantic != s {
		return
	}
	r.semantic = s
}

// SourceID returns the source identifier associated with the rule.
func (r *Rule) SourceID() int64 {
	return r.exprID
}

// ID returns the id value of the rule if it is set.
func (r *Rule) ID() ValueString {
	if r.id != nil {
		return *r.id
	}
	return ValueString{}
}

// Description returns the rule description if it is set.
func (r *Rule) Description() ValueString {
	if r.description != nil {
		return *r.description
	}
	return ValueString{}
}

// Matches returns the ordered set of Match declarations.
func (r *Rule) Matches() []*Match {
	return r.matches[:]
}

// Variables returns the order set of Variable tuples.
func (r *Rule) Variables() []*Variable {
	return r.variables[:]
}

// SetID configures the id for the rule.
func (r *Rule) SetID(id ValueString) {
	r.id = &id
}

// SetDescription configures the description for the rule.
func (r *Rule) SetDescription(desc ValueString) {
	r.description = &desc
}

// AddMatch addes a Match to the rule.
func (r *Rule) AddMatch(m *Match) {
	r.matches = append(r.matches, m)
}

// AddVariable adds a variable to the rule.
func (r *Rule) AddVariable(v *Variable) {
	r.variables = append(r.variables, v)
}

// AddVariables adds a list of variables to the rule.
func (r *Rule) AddVariables(v []*Variable) {
	r.variables = append(r.variables, v...)
}

func (r *Rule) getExplanationOutputRule() *Rule {
	if r == nil {
		return nil
	}
	er := Rule{
		id:          r.id,
		description: r.description,
		semantic:    r.semantic,
	}
	er.AddVariables(r.Variables())
	for _, match := range r.matches {
		em := Match{
			condition: match.condition,
			output:    match.explanation,
			rule:      match.rule.getExplanationOutputRule(),
		}
		er.matches = append(er.matches, &em)
	}
	return &er
}

// NewVariable creates a variable instance.
func NewVariable(exprID int64) *Variable {
	return &Variable{exprID: exprID}
}

// Variable is a named expression which may be referenced in subsequent expressions.
type Variable struct {
	exprID     int64
	name       ValueString
	expression ValueString
}

// Name returns the variable name.
func (v *Variable) Name() ValueString {
	return v.name
}

// Expression returns the variable expression.
func (v *Variable) Expression() ValueString {
	return v.expression
}

// SetName sets the variable name.
func (v *Variable) SetName(name ValueString) {
	v.name = name
}

// SetExpression sets the variable expression.
func (v *Variable) SetExpression(e ValueString) {
	v.expression = e
}

// NewMatch creates a match instance.
func NewMatch(exprID int64) *Match {
	return &Match{exprID: exprID}
}

// Match declares a condition (defaults to true) as well as an output or a rule.
// Either the output or the rule field may be set, but not both.
type Match struct {
	exprID      int64
	condition   ValueString
	output      *ValueString
	explanation *ValueString
	rule        *Rule
}

// SourceID returns the source identifier associated with the match.
func (m *Match) SourceID() int64 {
	return m.exprID
}

// Condition returns the condition CEL expression.
func (m *Match) Condition() ValueString {
	return m.condition
}

// HasOutput indicates whether the output field is set of the match.
func (m *Match) HasOutput() bool {
	return m.output != nil
}

// Output returns the output expression, or empty expression if output is not set.
func (m *Match) Output() ValueString {
	if m.HasOutput() {
		return *m.output
	}
	return ValueString{}
}

// HasExplanation indicates whether the explanation field is set of the match.
func (m *Match) HasExplanation() bool {
	return m.explanation != nil
}

// Explanation returns the explanation expression, or empty expression if output is not set.
func (m *Match) Explanation() ValueString {
	if m.HasExplanation() {
		return *m.explanation
	}
	return ValueString{}
}

// HasRule indicates whether the rule field is set on a match.
func (m *Match) HasRule() bool {
	return m.rule != nil
}

// Rule returns the rule value, or nil if the rule is not set.
func (m *Match) Rule() *Rule {
	return m.rule
}

// SetCondition sets the CEL condition for the match.
func (m *Match) SetCondition(c ValueString) {
	m.condition = c
}

// SetOutput sets the output expression for the match.
func (m *Match) SetOutput(o ValueString) {
	m.output = &o
}

// SetExplanation sets the explanation expression for the match.
func (m *Match) SetExplanation(e ValueString) {
	m.explanation = &e
}

// SetRule sets the rule for the match.
func (m *Match) SetRule(r *Rule) {
	m.rule = r
}

// ValueString contains an identifier corresponding to source metadata and a simple string.
type ValueString struct {
	ID    int64
	Value string
}

// ParserContext declares a set of interfaces for creating and managing metadata for parsed policies.
type ParserContext interface {
	// NextID returns a monotonically increasing identifier for a source fragment.
	// This ID is implicitly created and tracked within the CollectMetadata method.
	NextID() int64

	// CollectMetadata records the source position information of a given YAML node, and returns
	// the id associated with the source metadata which is returned in the Policy SourceInfo object.
	CollectMetadata(*yaml.Node) int64

	// NewPolicy creates a new Policy instance with an ID associated with the YAML node.
	NewPolicy(*yaml.Node) (*Policy, int64)

	// NewRule creates a new Rule instance with an ID associated with the YAML node.
	NewRule(*yaml.Node) (*Rule, int64)

	// NewVariable creates a new Variable instance with an ID associated with the YAML node.
	NewVariable(*yaml.Node) (*Variable, int64)

	// NewMatch creates a new Match instance with an ID associated with the YAML node.
	NewMatch(*yaml.Node) (*Match, int64)

	// NewString creates a new ValueString from the YAML node.
	NewString(*yaml.Node) ValueString

	// ParsePolicy will parse the target yaml node as though it is the top-level policy.
	ParsePolicy(ParserContext, *yaml.Node) *Policy

	// ParseRule will parse the current yaml node as though it is the entry point to a rule.
	ParseRule(ParserContext, *Policy, *yaml.Node) *Rule

	// ParseMatch  will parse the current yaml node as though it is the entry point to a match.
	ParseMatch(ParserContext, *Policy, *yaml.Node) *Match

	// ParseVariable will parse the current yaml node as though it is the entry point to a variable.
	ParseVariable(ParserContext, *Policy, *yaml.Node) *Variable

	// ReportErrorAtID logs an error during parsing which is included in the issue set returned from
	// a failed parse.
	ReportErrorAtID(id int64, msg string, args ...any)
}

// TagVisitor declares a set of interfaces for handling custom tags which would otherwise be unsupported
// within the policy, rule, match, or variable objects.
type TagVisitor interface {
	// PolicyTag accepts a parser context, field id, tag name, yaml node, and parent Policy to allow for
	// continued parsing within a custom tag.
	PolicyTag(ParserContext, int64, string, *yaml.Node, *Policy)

	// RuleTag accepts a parser context, field id, tag name, yaml node, as well as the parent policy and
	// current rule to allow for continued parsing within custom tags.
	RuleTag(ParserContext, int64, string, *yaml.Node, *Policy, *Rule)

	// MatchTag accepts a parser context, field id, tag name, yaml node, as well as the parent policy and
	// current match to allow for continued parsing within custom tags.
	MatchTag(ParserContext, int64, string, *yaml.Node, *Policy, *Match)

	// VariableTag accepts a parser context, field id, tag name, yaml node, as well as the parent policy and
	// current variable to allow for continued parsing within custom tags.
	VariableTag(ParserContext, int64, string, *yaml.Node, *Policy, *Variable)
}

// DefaultTagVisitor creates a tag visitor which produces errors for any non-canonical YAML tag.
func DefaultTagVisitor() TagVisitor {
	return defaultTagVisitor{}
}

type defaultTagVisitor struct{}

func (defaultTagVisitor) PolicyTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, p *Policy) {
	if tagName == "description" {
		return
	}
	ctx.ReportErrorAtID(id, "unsupported policy tag: %s", tagName)
}

func (defaultTagVisitor) RuleTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, p *Policy, r *Rule) {
	ctx.ReportErrorAtID(id, "unsupported rule tag: %s", tagName)
}

func (defaultTagVisitor) MatchTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, p *Policy, m *Match) {
	ctx.ReportErrorAtID(id, "unsupported match tag: %s", tagName)
}

func (defaultTagVisitor) VariableTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, p *Policy, v *Variable) {
	ctx.ReportErrorAtID(id, "unsupported variable tag: %s", tagName)
}

// Parser parses policy files into a canonical Policy representation.
type Parser struct {
	TagVisitor

	inlineStyleVariables bool
}

// ParserOption is a function parser option for configuring Parser behavior.
type ParserOption func(*Parser) (*Parser, error)

// SimpleVariables configures the parser to allow for key-value pairs to declare a variable name and
// expression.
//
// For example:
//
//	variables:
//	  - foo: bar
//	  - baz: qux
//
// This is in contrast to the default behavior, which requires the following syntax:
//
//	variables:
//	  - name: foo
//	    expression: bar
//	  - name: baz
//	    expression: qux
func SimpleVariables() ParserOption {
	return func(p *Parser) (*Parser, error) {
		p.inlineStyleVariables = true
		return p, nil
	}
}

// CustomTagVisitor configures the parser to use the given TagVisitor for handling custom tags.
func CustomTagVisitor(visitor TagVisitor) ParserOption {
	return func(p *Parser) (*Parser, error) {
		p.TagVisitor = visitor
		return p, nil
	}
}

// NewParser creates a new Parser object with a set of functional options.
func NewParser(opts ...ParserOption) (*Parser, error) {
	p := &Parser{TagVisitor: defaultTagVisitor{}}
	var err error
	for _, o := range opts {
		p, err = o(p)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Parse generates an internal parsed policy representation from a YAML input file.
// The internal representation ensures that CEL expressions are tracked relative to
// where they occur within the file, thus making error messages relative to the whole
// file rather than the individual expression.
func (parser *Parser) Parse(src *Source) (*Policy, *cel.Issues) {
	info := ast.NewSourceInfo(src)
	errs := common.NewErrors(src)
	iss := cel.NewIssuesWithSourceInfo(errs, info)
	p := newParserImpl(parser.TagVisitor, info, src, iss, parser.inlineStyleVariables)
	policy := p.parseYAML(src)
	if iss.Err() != nil {
		return nil, iss
	}
	return policy, nil
}

// normalizeEntry extracts a key, value pair as the next two elements from the
// content slice. val source position information is normalized depending on style.
func normalizeEntry(content []*yaml.Node, i int) (key *yaml.Node, val *yaml.Node) {
	key = content[i]
	val = content[i+1]
	if val.Style == yaml.FoldedStyle || val.Style == yaml.LiteralStyle {
		val.Line++
		val.Column = key.Column + 1
	}
	return key, val
}

func (p *parserImpl) parseYAML(src *Source) *Policy {
	// Parse yaml representation from the source to an object model.
	var docNode yaml.Node
	err := sourceToYAML(src, &docNode)
	if err != nil {
		p.iss.ReportErrorAtID(0, "%s", err.Error())
		return nil
	}
	// Entry point always has a single Content node
	return p.ParsePolicy(p, docNode.Content[0])
}

func sourceToYAML(src *Source, docNode *yaml.Node) error {
	err := yaml.Unmarshal([]byte(src.Content()), docNode)
	if err != nil {
		return err
	}
	if docNode.Kind != yaml.DocumentNode {
		return fmt.Errorf("got yaml node of kind %v, wanted mapping node", docNode.Kind)
	}
	return nil
}

func newParserImpl(visitor TagVisitor, info *ast.SourceInfo, src *Source, iss *cel.Issues, arrayStyleVariables bool) *parserImpl {
	return &parserImpl{
		visitor:              visitor,
		info:                 info,
		src:                  src,
		iss:                  iss,
		inlineStyleVariables: arrayStyleVariables,
	}
}

type parserImpl struct {
	id      int64
	visitor TagVisitor
	info    *ast.SourceInfo
	src     *Source
	iss     *cel.Issues

	YAMLHelper
	inlineStyleVariables bool
}

// NextID returns a monotonically increasing identifier for a source fragment.
// This ID is implicitly created and tracked within the CollectMetadata method.
func (p *parserImpl) NextID() int64 {
	p.id++
	return p.id
}

// NewPolicy creates a new Policy instance with an ID associated with the YAML node.
func (p *parserImpl) NewPolicy(node *yaml.Node) (*Policy, int64) {
	policy := NewPolicy(p.src, p.info)
	id := p.CollectMetadata(node)
	return policy, id
}

// NewRule creates a new Rule instance with an ID associated with the YAML node.
func (p *parserImpl) NewRule(node *yaml.Node) (*Rule, int64) {
	id := p.CollectMetadata(node)
	r := NewRule(id)
	return r, id
}

// NewVariable creates a new Variable instance with an ID associated with the YAML node.
func (p *parserImpl) NewVariable(node *yaml.Node) (*Variable, int64) {
	id := p.CollectMetadata(node)
	v := NewVariable(id)
	return v, id
}

// NewMatch creates a new Match instance with an ID associated with the YAML node.
func (p *parserImpl) NewMatch(node *yaml.Node) (*Match, int64) {
	id := p.CollectMetadata(node)
	m := NewMatch(id)
	return m, id
}

// NewString creates a new ValueString from the YAML node.
func (p *parserImpl) NewString(node *yaml.Node) ValueString {
	id := p.CollectMetadata(node)
	nodeType := p.assertYAMLType(id, node, yamlString, yamlText)
	if nodeType == nil {
		return ValueString{ID: id, Value: "*error*"}
	}
	if *nodeType == yamlText {
		return ValueString{ID: id, Value: node.Value}
	}
	if node.Style == yaml.FoldedStyle || node.Style == yaml.LiteralStyle {
		col := node.Column
		line := node.Line
		txt, found := p.src.Snippet(line)
		indent := ""
		for len(indent) < col-1 {
			indent += " "
		}
		var raw strings.Builder
		for found && strings.HasPrefix(txt, indent) {
			line++
			raw.WriteString(txt)
			txt, found = p.src.Snippet(line)
			if found && strings.HasPrefix(txt, indent) {
				raw.WriteString("\n")
			}
		}
		offset := p.info.OffsetRanges()[p.id]
		offsetStart := offset.Start - (int32(node.Column) - 1)
		p.info.SetOffsetRange(p.id, ast.OffsetRange{Start: offsetStart, Stop: offsetStart})
		return ValueString{ID: id, Value: raw.String()}
	}
	return ValueString{ID: id, Value: node.Value}
}

// newStrictString creates a new ValueString from the YAML node, but as a string with no special
// source position information. Intended for use in descriptions, where the string is just
// a human-readable string for presentation.
func (p *parserImpl) newStrictString(node *yaml.Node) ValueString {
	id := p.CollectMetadata(node)
	nodeType := p.assertYAMLType(id, node, yamlString, yamlText)
	if nodeType == nil {
		return ValueString{ID: id, Value: "*error*"}
	}
	return ValueString{ID: id, Value: node.Value}
}

// CollectMetadata records the source position information of a given YAML node, and returns
// the id associated with the source metadata which is returned in the Policy SourceInfo object.
func (p *parserImpl) CollectMetadata(node *yaml.Node) int64 {
	id := p.NextID()
	line := node.Line
	col := int32(node.Column)
	switch node.Style {
	case yaml.DoubleQuotedStyle, yaml.SingleQuotedStyle:
		col++
	}
	offsetStart := int32(0)
	if line > 1 {
		offsetStart = p.info.LineOffsets()[line-2]
	}
	p.info.SetOffsetRange(id, ast.OffsetRange{Start: offsetStart + col - 1, Stop: offsetStart + col - 1})
	return id
}

// ParsePolicy will parse the target yaml node as though it is the top-level policy.
func (p *parserImpl) ParsePolicy(ctx ParserContext, node *yaml.Node) *Policy {
	ctx.CollectMetadata(node)
	policy, id := ctx.NewPolicy(node)
	if p.assertYAMLType(id, node, yamlMap) == nil || !p.checkMapValid(ctx, id, node) {
		return policy
	}
	p.RangeMap(node, func(key, val *yaml.Node) bool {
		keyID := p.CollectMetadata(key)
		fieldName := key.Value
		switch fieldName {
		case "imports":
			p.parseImports(ctx, policy, val)
		case "name":
			policy.SetName(ctx.NewString(val))
		case "description":
			policy.SetDescription(p.newStrictString(val))
			// Since the description field was not supported initially, some
			// clients rely on the ability to intercept it.
			p.visitor.PolicyTag(ctx, keyID, fieldName, val, policy)
		case "rule":
			policy.SetRule(p.ParseRule(ctx, policy, val))
		default:
			p.visitor.PolicyTag(ctx, keyID, fieldName, val, policy)
		}
		return true
	})
	return policy
}

func (p *parserImpl) parseImports(ctx ParserContext, policy *Policy, node *yaml.Node) {
	id := ctx.CollectMetadata(node)
	if p.assertYAMLType(id, node, yamlList) == nil {
		return
	}
	for _, val := range node.Content {
		policy.AddImport(p.parseImport(ctx, policy, val))
	}
}

func (p *parserImpl) parseImport(ctx ParserContext, _ *Policy, node *yaml.Node) *Import {
	id := ctx.CollectMetadata(node)
	imp := NewImport(id)
	if p.assertYAMLType(id, node, yamlMap) == nil || !p.checkMapValid(ctx, id, node) {
		return imp
	}
	p.RangeMap(node, func(key *yaml.Node, val *yaml.Node) bool {
		ctx.CollectMetadata(key)
		fieldName := key.Value
		switch fieldName {
		case "name":
			imp.SetName(ctx.NewString(val))
		}
		return true
	})
	return imp
}

// ParseRule will parse the current yaml node as though it is the entry point to a rule.
func (p *parserImpl) ParseRule(ctx ParserContext, policy *Policy, node *yaml.Node) *Rule {
	r, id := ctx.NewRule(node)
	if p.assertYAMLType(id, node, yamlMap) == nil || !p.checkMapValid(ctx, id, node) {
		return r
	}
	p.RangeMap(node, func(key, val *yaml.Node) bool {
		tagID := ctx.CollectMetadata(key)
		fieldName := key.Value
		switch fieldName {
		case "id":
			r.SetID(ctx.NewString(val))
		case "description":
			r.SetDescription(ctx.NewString(val))
		case "variables":
			p.parseVariables(ctx, policy, r, val)
		case "match", "aggregate":
			sem := firstMatch
			if fieldName == "aggregate" {
				sem = aggregate
			}
			if r.semantic != unspecified && r.semantic != sem {
				p.ReportErrorAtID(tagID, "Only one of 'match' or 'aggregate' may be set in a rule")
			} else {
				r.SetSemantic(sem)
				policy.SetSemantic(sem)
				p.parseMatches(ctx, policy, r, val)
			}
		default:
			p.visitor.RuleTag(ctx, tagID, fieldName, val, policy, r)
		}
		return true
	})
	return r
}

func (p *parserImpl) parseVariables(ctx ParserContext, policy *Policy, r *Rule, node *yaml.Node) {
	id := ctx.CollectMetadata(node)
	if p.assertYAMLType(id, node, yamlList) == nil {
		return
	}
	p.RangeList(node, func(i int, val *yaml.Node) bool {
		r.AddVariable(p.ParseVariable(ctx, policy, val))
		return true
	})
}

// ParseVariable will parse the current yaml node as though it is the entry point to a variable.
func (p *parserImpl) ParseVariable(ctx ParserContext, policy *Policy, node *yaml.Node) *Variable {
	v, id := ctx.NewVariable(node)
	if p.assertYAMLType(id, node, yamlMap) == nil || !p.checkMapValid(ctx, id, node) {
		return v
	}
	if p.inlineStyleVariables {
		return p.parseVariableInline(ctx, policy, v, node)
	}
	return p.parseVariableObject(ctx, policy, v, node)
}

func (p *parserImpl) parseVariableInline(ctx ParserContext, policy *Policy, v *Variable, node *yaml.Node) *Variable {
	iterations := 0
	p.RangeMap(node, func(key, val *yaml.Node) bool {
		keyVal := ctx.NewString(key)
		v.SetName(keyVal)
		v.SetExpression(ctx.NewString(val))
		iterations++
		if iterations > 1 {
			p.ReportErrorAtID(keyVal.ID, "only one variable may be defined inline")
			return false
		}
		return true
	})
	return v
}

func (p *parserImpl) parseVariableObject(ctx ParserContext, policy *Policy, v *Variable, node *yaml.Node) *Variable {
	p.RangeMap(node, func(key, val *yaml.Node) bool {
		keyID := ctx.CollectMetadata(key)
		fieldName := key.Value
		switch fieldName {
		case "name":
			v.SetName(ctx.NewString(val))
		case "expression":
			v.SetExpression(ctx.NewString(val))
		default:
			p.visitor.VariableTag(ctx, keyID, fieldName, val, policy, v)
		}
		return true
	})
	return v
}

func (p *parserImpl) parseMatches(ctx ParserContext, policy *Policy, r *Rule, node *yaml.Node) {
	id := ctx.CollectMetadata(node)
	if p.assertYAMLType(id, node, yamlList) == nil {
		return
	}
	for _, val := range node.Content {
		r.AddMatch(p.parseMatchInternal(ctx, policy, r, val))
	}
}

// ParseMatch  will parse the current yaml node as though it is the entry point to a match.
func (p *parserImpl) ParseMatch(ctx ParserContext, policy *Policy, node *yaml.Node) *Match {
	return p.parseMatchInternal(ctx, policy, nil, node)
}

func (p *parserImpl) parseMatchInternal(ctx ParserContext, policy *Policy, r *Rule, node *yaml.Node) *Match {
	m, id := ctx.NewMatch(node)
	if p.assertYAMLType(id, node, yamlMap) == nil || !p.checkMapValid(ctx, id, node) {
		return m
	}
	ruleSem := firstMatch
	if r != nil {
		ruleSem = r.Semantic()
	} else {
		ruleSem = policy.Semantic()
	}
	isAggregate := ruleSem == aggregate
	m.SetCondition(ValueString{ID: ctx.NextID(), Value: "true"})
	p.RangeMap(node, func(key, val *yaml.Node) bool {
		keyID := ctx.CollectMetadata(key)
		fieldName := key.Value
		switch fieldName {
		case "condition":
			m.SetCondition(ctx.NewString(val))
		case "output", "emit":
			if fieldName == "output" && isAggregate {
				p.ReportErrorAtID(keyID, "Rule aggregate requires 'emit' tag instead of 'output'")
			} else if fieldName == "emit" && !isAggregate {
				p.ReportErrorAtID(keyID, "Rule match requires 'output' tag instead of 'emit'")
			}
			if m.HasRule() {
				p.ReportErrorAtID(keyID, "only the rule or the output may be set")
			}
			m.SetOutput(ctx.NewString(val))
		case "explanation":
			if m.HasRule() {
				p.ReportErrorAtID(keyID, "explanation can only be set on output match cases, not nested rules")
			}
			m.SetExplanation(ctx.NewString(val))
		case "rule", "match", "aggregate":
			if m.HasOutput() {
				p.ReportErrorAtID(keyID, "only the rule or the output may be set")
			}
			if m.HasExplanation() {
				p.ReportErrorAtID(keyID, "explanation can only be set on output match cases, not nested rules")
			}
			m.SetRule(p.ParseRule(ctx, policy, val))
		default:
			p.visitor.MatchTag(ctx, keyID, fieldName, val, policy, m)
		}
		return true
	})
	if !m.HasOutput() && !m.HasRule() {
		p.ReportErrorAtID(id, "match does not specify a rule or output")
	}
	return m
}

func (p *parserImpl) assertYAMLType(id int64, node *yaml.Node, nodeTypes ...yamlNodeType) *yamlNodeType {
	nt, found := yamlTypes[node.LongTag()]
	if !found {
		p.ReportErrorAtID(id, "unsupported yaml tag type: %v", node.LongTag())
		return nil
	}
	for _, nodeType := range nodeTypes {
		if nt == nodeType {
			return &nt
		}
	}
	p.ReportErrorAtID(id, "got yaml node type %v, wanted type(s) %v", node.LongTag(), nodeTypes)
	return nil
}

// ReportErrorAtID logs an error during parsing which is included in the issue set returned from
// a failed parse.
func (p *parserImpl) ReportErrorAtID(id int64, format string, args ...any) {
	p.iss.ReportErrorAtID(id, format, args...)
}

func (p *parserImpl) checkMapValid(ctx ParserContext, id int64, node *yaml.Node) bool {
	valid := len(node.Content)%2 == 0
	if !valid {
		ctx.ReportErrorAtID(id, "mismatched key-value pairs in map")
	}
	return valid
}

type yamlNodeType int

const (
	yamlText yamlNodeType = iota + 1
	yamlBool
	yamlNull
	yamlString
	yamlInt
	yamlDouble
	yamlList
	yamlMap
	yamlTimestamp
)

func (yt yamlNodeType) String() string {
	for k, v := range yamlTypes {
		if v == yt {
			return k
		}
	}
	return fmt.Sprintf("%d", yt)
}

var (
	// yamlTypes map of the long tag names supported by the Go YAML v3 library.
	yamlTypes = map[string]yamlNodeType{
		"!txt":                        yamlText,
		"tag:yaml.org,2002:bool":      yamlBool,
		"tag:yaml.org,2002:null":      yamlNull,
		"tag:yaml.org,2002:str":       yamlString,
		"tag:yaml.org,2002:int":       yamlInt,
		"tag:yaml.org,2002:float":     yamlDouble,
		"tag:yaml.org,2002:seq":       yamlList,
		"tag:yaml.org,2002:map":       yamlMap,
		"tag:yaml.org,2002:timestamp": yamlTimestamp,
	}
)
