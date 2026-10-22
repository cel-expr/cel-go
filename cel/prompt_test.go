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
	"errors"
	"strings"
	"sync"
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/env"
	"github.com/google/go-cmp/cmp"

	"google.golang.org/protobuf/proto"
	dpb "google.golang.org/protobuf/types/descriptorpb"
)

//go:embed testdata/basic.prompt.txt
var wantBasicPrompt string

//go:embed testdata/macros.prompt.txt
var wantMacrosPrompt string

//go:embed testdata/standard_env.prompt.txt
var wantStandardEnvPrompt string

//go:embed testdata/field_paths.prompt.txt
var wantFieldPathsPrompt string

//go:embed testdata/test_fds_with_source_info-transitive-descriptor-set-source-info.proto.bin
var testFdsWithSourceInfo []byte

var onceFds sync.Once
var fds *dpb.FileDescriptorSet

func testFds(t *testing.T) *dpb.FileDescriptorSet {
	onceFds.Do(func() {
		fds = &dpb.FileDescriptorSet{}
		err := proto.Unmarshal(testFdsWithSourceInfo, fds)
		if err != nil {
			t.Fatalf("failed to unmarshal testFdsWithSourceInfo: %v", err)
		}
	})
	return fds
}

func TestPromptTemplate(t *testing.T) {
	tests := []struct {
		name       string
		envOpts    []EnvOption
		promptOpts []PromptOption
		out        string
	}{
		{
			name: "basic_compact",
			out:  "<USER_PROMPT>\n",
		},
		{
			name:       "basic_verbose",
			promptOpts: []PromptOption{PromptVerbose(true)},
			out:        wantBasicPrompt,
		},
		{
			name:    "macros_compact",
			envOpts: []EnvOption{Macros(StandardMacros...)},
			out:     "<USER_PROMPT>\n",
		},
		{
			name:       "macros_verbose",
			envOpts:    []EnvOption{Macros(StandardMacros...)},
			promptOpts: []PromptOption{PromptVerbose(true)},
			out:        wantMacrosPrompt,
		},
		{
			name:    "standard_env_compact",
			envOpts: []EnvOption{StdLib()},
			out:     "<USER_PROMPT>\n",
		},
		{
			name:       "standard_env_verbose",
			envOpts:    []EnvOption{StdLib(StdLibSubset(env.NewLibrarySubset().SetDisableMacros(true)))},
			promptOpts: []PromptOption{PromptVerbose(true)},
			out:        wantStandardEnvPrompt,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			envOpts := append([]EnvOption{TypeDescs(testFds(t))}, tc.envOpts...)

			env, err := NewCustomEnv(
				envOpts...,
			)
			if err != nil {
				t.Fatalf("cel.NewCustomEnv() failed: %v", err)
			}
			prompt, err := AuthoringPrompt(env, tc.promptOpts...)
			if err != nil {
				t.Fatalf("cel.AuthoringPrompt() failed: %v", err)
			}
			out := prompt.Render("<USER_PROMPT>")
			if diff := cmp.Diff(tc.out, out); diff != "" {
				t.Errorf("got %s, diff (-want +got): %s", out, diff)
			}
		})
	}
}

func TestPromptTemplateFieldPaths(t *testing.T) {
	tests := []struct {
		name       string
		envOpts    []EnvOption
		promptOpts []PromptOption
		out        string
	}{
		{
			name: "standard_env_compact",
			envOpts: []EnvOption{
				VariableWithDoc("team", ObjectType("cel.testdata.Team"),
					common.MultilineDescription("A team of gifted youngsters")),
				StdLib(),
			},
			out: `Only use the following variables, macros, and functions in expressions.

Variables:

* name: ` + "`team`" + `
  type: ` + "`cel.testdata.Team`" + `
  description:
    A team of gifted youngsters

  attributes:
    * path: ` + "`team.members`" + `
      type: ` + "`list(cel.testdata.Mutant)`" + `
      description:
        The members of the team.

    * path: ` + "`team.members[0].level`" + `
      type: ` + "`int`" + `
      description:
        The level of the mutant.
        Values from 1 to 10.

    * path: ` + "`team.members[0].name`" + `
      type: ` + "`string`" + `
      description:
        The display name of the mutant.

    * path: ` + "`team.members[0].super_power`" + `
      type: ` + "`string`" + `
      description:
        The super power of the mutant.
        Matches the IDs in Foo.db.

    * path: ` + "`team.name`" + `
      type: ` + "`string`" + `
      description:
        The name of the team.
        Formated as 'team_<8_digit_hex_number>'.

<USER_PROMPT>
`,
		},
		{
			name: "standard_env_verbose",
			envOpts: []EnvOption{
				VariableWithDoc("team", ObjectType("cel.testdata.Team"),
					common.MultilineDescription("A team of gifted youngsters")),
				StdLib(StdLibSubset(env.NewLibrarySubset().SetDisableMacros(true))),
			},
			promptOpts: []PromptOption{PromptVerbose(true)},
			out:        wantFieldPathsPrompt,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			envOpts := append([]EnvOption{TypeDescs(testFds(t))}, tc.envOpts...)

			env, err := NewCustomEnv(
				envOpts...,
			)
			if err != nil {
				t.Fatalf("cel.NewCustomEnv() failed: %v", err)
			}
			prompt, err := AuthoringPromptWithFieldPaths(env, tc.promptOpts...)
			if err != nil {
				t.Fatalf("cel.AuthoringPromptWithFieldPaths() failed: %v", err)
			}
			out := prompt.Render("<USER_PROMPT>")
			if diff := cmp.Diff(tc.out, out); diff != "" {
				t.Errorf("got %s, diff (-want +got): %s", out, diff)
			}
		})
	}
}

// TestRenderSanitizesTemplateDirectives verifies that user-supplied input containing
// Go text/template action delimiters is rendered as literal text and never executed
// as template directives.
//
// Go's text/template renders struct field values (such as .UserPrompt) as literal
// strings — it does not re-parse or re-evaluate them as template syntax. This means
// a caller passing "{{.Persona}}" as the user prompt will see that exact string in
// the output, not the expanded persona content.
func TestRenderSanitizesTemplateDirectives(t *testing.T) {
	env, err := NewEnv()
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	prompt, err := AuthoringPrompt(env, PromptVerbose(true))
	if err != nil {
		t.Fatalf("cel.AuthoringPrompt() failed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantLiteral string
		wantAbsent  string
	}{
		{
			name:        "template_field_not_executed",
			input:       "{{.Persona}} should be literal",
			wantLiteral: "{{.Persona}} should be literal",
		},
		{
			name:        "closing_delimiters_escaped",
			input:       "hello }} world",
			wantLiteral: "hello }} world",
		},
		{
			name:        "format_rules_field_not_executed",
			input:       "{{.FormatRules}} injected",
			wantLiteral: "{{.FormatRules}} injected",
		},
		{
			name:        "persona_not_duplicated_via_injection",
			input:       "{{.Persona}}",
			wantLiteral: "{{.Persona}}",
			// Persona text appears once legitimately at the top of the rendered output.
			// A second occurrence would mean the injected directive executed and dumped
			// the persona content again into the user prompt section.
			wantAbsent: "software engineer\nauthoring boolean",
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			out := prompt.Render(tc.input)
			if !strings.Contains(out, tc.wantLiteral) {
				t.Errorf("Render(%q):\n  want literal substring: %q\n  got output:\n%s",
					tc.input, tc.wantLiteral, out)
			}
			if tc.wantAbsent != "" {
				if strings.Count(out, tc.wantAbsent) > 1 {
					t.Errorf("Render(%q): template injection detected — %q appears multiple times in output:\n%s",
						tc.input, tc.wantAbsent, out)
				}
			}
		})
	}
}

func TestPromptCompactContainerImportsAndCustomFunctions(t *testing.T) {
	e, err := NewEnv(
		TypeDescs(testFds(t)),
		Container("cel.testdata"),
		Abbrevs("google.protobuf.Timestamp", "google.protobuf.Duration"),
		VariableWithDoc("request_id", StringType, "Unique identifier for the request"),
		VariableWithDoc("actor", StringType, "Authenticated principal"),
		Function("coalesce",
			FunctionDocs("return the first non-null value"),
			Overload("coalesce_int_wrapper_int",
				[]*Type{NullableType(IntType), IntType}, IntType,
				OverloadExamples(
					`coalesce(null, 1) // 1`,
					`coalesce(2, 1) // 2`)),
		),
		Function("size",
			MemberOverload("team_size",
				[]*Type{ObjectType("cel.testdata.Team")}, IntType,
				OverloadExamples(`team.size() // 5`)),
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	prompt, err := AuthoringPrompt(e)
	if err != nil {
		t.Fatalf("cel.AuthoringPrompt() failed: %v", err)
	}
	got := prompt.Render("<USER_PROMPT>")

	want := `Only use the following variables, macros, and functions in expressions.

Container: ` + "`cel.testdata`" + `

Imports:

* ` + "`google.protobuf.Duration`" + `
* ` + "`google.protobuf.Timestamp`" + `

Variables:

* name: ` + "`actor`" + `
  type: ` + "`string`" + `
  description:
    Authenticated principal

* name: ` + "`request_id`" + `
  type: ` + "`string`" + `
  description:
    Unique identifier for the request


Functions:

* coalesce - return the first non-null value

      coalesce(wrapper(int), int) -> int
        Example: coalesce(null, 1) // 1
        Example: coalesce(2, 1) // 2

* size - compute the size of a list or map, the number of characters in a string, or the number of bytes in a sequence

      cel.testdata.Team.size() -> int
        Example: team.size() // 5

<USER_PROMPT>
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("prompt.Render() diff (-want +got):\n%s", diff)
	}
}

func TestPromptCompactStdLibSubsetted(t *testing.T) {
	conf := env.NewConfig("subsetted_env").
		SetStdLib(
			env.NewLibrarySubset().
				AddIncludedMacros("has").
				AddIncludedFunctions(env.NewFunction("_==_", env.NewOverload("equals", nil, nil))),
		)
	e, err := NewCustomEnv(FromConfig(conf))
	if err != nil {
		t.Fatalf("cel.NewCustomEnv() failed: %v", err)
	}

	prompt, err := AuthoringPrompt(e)
	if err != nil {
		t.Fatalf("cel.AuthoringPrompt() failed: %v", err)
	}
	got := prompt.Render("<USER_PROMPT>")

	want := `Only use the following variables, macros, and functions in expressions.

Macros:

* has macro - check a protocol buffer message for the presence of a field, or check a map for the presence of a string key. Only map accesses using the select notation are supported.

      // true if the 'address' field exists in the 'user' message
      has(user.address)
      // test whether the 'key_name' is set on the map which defines it
      has({'key_name': 'value'}.key_name) // true
      // test whether the 'id' field is set to a non-default value on the Expr{} message literal
      has(Expr{}.id) // false


Functions:

* _==_ - compare two values of the same type for equality

      <A> == <A> -> bool
        Example: 1 == 1 // true
        Example: 'hello' == 'world' // false
        Example: bytes('hello') == b'hello' // true
        Example: duration('1h') == duration('60m') // true
        Example: dyn(3.0) == 3 // true

<USER_PROMPT>
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("prompt.Render() diff (-want +got):\n%s", diff)
	}
}

func TestAuthoringPromptOptionError(t *testing.T) {
	e, err := NewEnv()
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	errOpt := func(p *Prompt) (*Prompt, error) {
		return nil, errors.New("invalid prompt option")
	}
	if _, err := AuthoringPrompt(e, errOpt); err == nil {
		t.Errorf("AuthoringPrompt() got nil error, wanted error")
	}
}

