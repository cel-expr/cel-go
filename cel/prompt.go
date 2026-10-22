// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cel

import (
	_ "embed"
	"sort"
	"strings"
	"text/template"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/stdlib"
	"cel.dev/cel-go/common/types"
)

//go:embed templates/authoring.tmpl
var authoringPrompt string

// splitImpl splits a string into a list of strings.
//
// Normalizes extracted comments (trim common prefix whitespace and extra trailing newlines).
func splitImpl(str string) []string {
	str = strings.TrimRight(str, " \n\t\r")
	out := strings.Split(str, "\n")
	if len(out) == 0 {
		return nil
	}
	negative := strings.TrimLeft(out[0], " \t")
	lenNegative := len(negative)
	lenOut := len(out[0])
	if lenNegative == lenOut {
		return out
	}
	prefix := out[0][:lenOut-lenNegative]
	trimmed := make([]string, len(out))
	for i, line := range out {
		if line == "" {
			trimmed[i] = ""
			continue
		}
		if !strings.HasPrefix(line, prefix) {
			return out
		}
		trimmed[i] = strings.TrimPrefix(line, prefix)
	}

	return trimmed
}

// PromptOption configures behavior for an authoring Prompt.
type PromptOption func(*Prompt) (*Prompt, error)

// PromptVerbose configures whether standard library definitions and generic LLM prompt scaffolding
// (Persona, FormatRules, GeneralUsage) are included in the prompt.
// By default, these are omitted to produce a compact prompt.
func PromptVerbose(verbose bool) PromptOption {
	return func(p *Prompt) (*Prompt, error) {
		p.verbose = verbose
		return p, nil
	}
}

// PromptFieldPaths configures whether reachable field paths on struct variables are documented in the prompt.
func PromptFieldPaths(fieldPaths bool) PromptOption {
	return func(p *Prompt) (*Prompt, error) {
		p.fieldPaths = fieldPaths
		return p, nil
	}
}

// AuthoringPrompt creates a prompt template from a CEL environment for the purpose of AI-assisted authoring.
//
// By default, generic LLM prompt scaffolding (Persona, FormatRules, GeneralUsage) and unmodified
// standard library functions and macros are omitted from the rendered prompt unless
// PromptVerbose(true) is specified.
func AuthoringPrompt(env *Env, opts ...PromptOption) (*Prompt, error) {
	funcMap := template.FuncMap{
		"split":          splitImpl,
		"newlineToSpace": func(str string) string { return strings.ReplaceAll(str, "\n", " ") },
	}
	tmpl := template.New("cel").Funcs(funcMap)
	tmpl, err := tmpl.Parse(authoringPrompt)
	if err != nil {
		return nil, err
	}
	p := &Prompt{
		tmpl: tmpl,
		env:  env,
	}
	for _, opt := range opts {
		p, err = opt(p)
		if err != nil {
			return nil, err
		}
	}
	if p.verbose {
		p.Persona = defaultPersona
		p.FormatRules = defaultFormatRules
		p.GeneralUsage = defaultGeneralUsage
	}
	return p, nil
}

// AuthoringPromptWithFieldPaths creates a prompt template from a CEL environment for the purpose of AI-assisted authoring.
// Includes documentation for all of the reachable field paths in the environment.
func AuthoringPromptWithFieldPaths(env *Env, opts ...PromptOption) (*Prompt, error) {
	allOpts := append([]PromptOption{PromptFieldPaths(true)}, opts...)
	return AuthoringPrompt(env, allOpts...)
}

// Prompt represents the core components of an LLM prompt based on a CEL environment.
//
// All fields of the prompt may be overwritten / modified with support for rendering the
// prompt to a human-readable string.
type Prompt struct {
	// Persona indicates something about the kind of user making the request
	Persona string

	// FormatRules indicate how the LLM should generate its output
	FormatRules string

	// GeneralUsage specifies additional context on how CEL should be used.
	GeneralUsage string

	// tmpl is the text template base-configuration for rendering text.
	tmpl *template.Template

	// fieldPaths is a flag to include reachable field paths in the prompt.
	fieldPaths bool

	// verbose is a flag to include standard library functions, macros, and prompt scaffolding in the prompt.
	verbose bool

	// env reference used to collect variables, functions, and macros available to the prompt.
	env *Env
}

type promptVariable struct {
	*common.Doc
	FieldPaths []*common.Doc
}

type promptInst struct {
	*Prompt

	Container  string
	Imports    []string
	Variables  []*promptVariable
	Macros     []*common.Doc
	Functions  []*common.Doc
	UserPrompt string
}

func isStdLibSubsetted(e *Env) bool {
	if lib, ok := e.libraries["cel.lib.std"]; ok {
		if subsetter, ok := lib.(LibrarySubsetter); ok {
			subset := subsetter.LibrarySubset()
			return subset != nil && !subset.Disabled
		}
	}
	return false
}

// Render renders the user prompt with the associated context from the prompt template
// for use with LLM generators.
//
// User-supplied input is passed as template data via the UserPrompt field, which
// Go's text/template renders as a literal string value. Template action delimiters
// such as {{.Persona}} in the user prompt are never evaluated as template directives
// because text/template only executes directives present in the template definition
// itself, not in data values interpolated at render time.
func (p *Prompt) Render(userPrompt string) string {
	var buffer strings.Builder
	container := p.env.Container.Name()
	var imports []string
	for _, imp := range p.env.Container.AliasSet() {
		imports = append(imports, imp)
	}
	sort.Strings(imports)

	filterStdLib := !p.verbose && !isStdLibSubsetted(p.env)

	vars := make([]*promptVariable, len(p.env.Variables()))
	for i, v := range p.env.Variables() {
		vars[i] = &promptVariable{Doc: v.Documentation()}
		if p.fieldPaths && v.Type().Kind() == types.StructKind {
			var fieldPaths []*common.Doc

			paths := fieldPathsForType(p.env.CELTypeProvider(), v.Name(), v.Type())
			if len(paths) < 2 {
				paths = nil
			} else {
				// First path is the variable which is already documented.
				paths = paths[1:]
			}
			for _, path := range paths {
				fieldPaths = append(fieldPaths, path.Documentation())
			}

			sort.SliceStable(fieldPaths, func(i, j int) bool {
				return fieldPaths[i].Name < fieldPaths[j].Name
			})
			vars[i].FieldPaths = fieldPaths
		}
	}
	sort.SliceStable(vars, func(i, j int) bool {
		return vars[i].Name < vars[j].Name
	})

	stdMacroKeys := map[string]bool{}
	if filterStdLib {
		for _, m := range StandardMacros {
			stdMacroKeys[m.MacroKey()] = true
		}
	}
	macs := make([]*common.Doc, 0, len(p.env.Macros()))
	for _, m := range p.env.Macros() {
		if filterStdLib && stdMacroKeys[m.MacroKey()] {
			continue
		}
		macs = append(macs, m.(common.Documentor).Documentation())
	}

	stdOverloads := map[string][]string{}
	if filterStdLib {
		for _, fnDecl := range stdlib.Functions() {
			overloads := make([]string, 0, len(fnDecl.OverloadDecls()))
			for _, o := range fnDecl.OverloadDecls() {
				overloads = append(overloads, o.ID())
			}
			stdOverloads[fnDecl.Name()] = overloads
		}
	}
	funcs := make([]*common.Doc, 0, len(p.env.Functions()))
	for _, f := range p.env.Functions() {
		if _, hidden := hiddenFunctions[f.Name()]; hidden {
			continue
		}
		if filterStdLib {
			if excludedOverloads, found := stdOverloads[f.Name()]; found {
				f = f.Subset(decls.ExcludeOverloads(excludedOverloads...))
				if f == nil {
					continue
				}
			}
		}
		funcs = append(funcs, f.Documentation())
	}
	sort.SliceStable(funcs, func(i, j int) bool {
		return funcs[i].Name < funcs[j].Name
	})
	inst := &promptInst{
		Prompt:     p,
		Container:  container,
		Imports:    imports,
		Variables:  vars,
		Macros:     macs,
		Functions:  funcs,
		UserPrompt: userPrompt,
	}
	p.tmpl.Execute(&buffer, inst)
	return buffer.String()
}

const (
	defaultPersona = `You are a software engineer with expertise in networking and application security
authoring boolean Common Expression Language (CEL) expressions to ensure firewall,
networking, authentication, and data access is only permitted when all conditions
are satisfied.`

	defaultFormatRules = `Output your response as a CEL expression.

Write the expression with the comment on the first line and the expression on the
subsequent lines. Format the expression using 80-character line limits commonly
found in C++ or Java code.`

	defaultGeneralUsage = `CEL supports Protocol Buffer and JSON types, as well as simple types and aggregate types.

Simple types include bool, bytes, double, int, string, and uint:

* double literals must always include a decimal point: 1.0, 3.5, -2.2
* uint literals must be positive values suffixed with a 'u': 42u
* byte literals are strings prefixed with a 'b': b'1235'
* string literals can use either single quotes or double quotes: 'hello', "world"
* string literals can also be treated as raw strings that do not require any
  escaping within the string by using the 'R' prefix: R"""quote: "hi" """

Aggregate types include list and map:

* list literals consist of zero or more values between brackets: "['a', 'b', 'c']"
* map literal consist of colon-separated key-value pairs within braces: "{'key1': 1, 'key2': 2}"
* Only int, uint, string, and bool types are valid map keys.
* Maps containing HTTP headers must always use lower-cased string keys.

Comments start with two-forward slashes followed by text and a newline.`
)

var (
	hiddenFunctions = map[string]bool{
		overloads.DeprecatedIn:        true,
		operators.OldIn:               true,
		operators.OldNotStrictlyFalse: true,
		operators.NotStrictlyFalse:    true,
	}
)
