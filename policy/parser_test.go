// Copyright 2024 Google LLC
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

package policy

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v3"
)

func TestParse(t *testing.T) {
	for _, tst := range policyTests {
		srcFile := readPolicy(t, fmt.Sprintf("testdata/%s/policy.yaml", tst.name))
		parser, err := NewParser(tst.parseOpts...)
		if err != nil {
			t.Fatalf("NewParser() failed: %v", err)
		}
		p, iss := parser.Parse(srcFile)
		if iss.Err() != nil {
			t.Fatalf("parser.Parse() failed: %v", iss.Err())
		}
		if p.Name().Value != tst.name {
			t.Errorf("policy name is %v, wanted %q", p.name, tst.name)
		}
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		txt string
		err string
	}{
		{
			txt: `
name:
  illegal: yaml-type`,
			err: `ERROR: <input>:3:3: got yaml node type tag:yaml.org,2002:map, wanted type(s) [tag:yaml.org,2002:str !txt]
 |   illegal: yaml-type
 | ..^`,
		},
		{
			txt: `
rule:
  custom: yaml-type`,
			err: `ERROR: <input>:3:3: unsupported rule tag: custom
 |   custom: yaml-type
 | ..^`,
		},
		{
			txt: `
inputs:
  - name: a
  - name: b`,
			err: `ERROR: <input>:2:1: unsupported policy tag: inputs
 | inputs:
 | ^`,
		},
		{
			txt: `
rule:
  variables:
    - name: "true"
      alt_name: "bool_true"`,
			err: `ERROR: <input>:5:7: unsupported variable tag: alt_name
 |       alt_name: "bool_true"
 | ......^`,
		},
		{
			txt: `
rule:
  match:
    - name: "true"
      alt_name: "bool_true"`,
			err: `ERROR: <input>:4:7: unsupported match tag: name
 |     - name: "true"
 | ......^
ERROR: <input>:4:7: match does not specify a rule or output
 |     - name: "true"
 | ......^
ERROR: <input>:5:7: unsupported match tag: alt_name
 |       alt_name: "bool_true"
 | ......^`,
		},
		{
			txt: `
- rule:
    id: a`,
			err: `ERROR: <input>:2:1: got yaml node type tag:yaml.org,2002:seq, wanted type(s) [tag:yaml.org,2002:map]
 | - rule:
 | ^`,
		},
		{
			txt: `
rule:
  match:
    - condition: "true"
      output: "world"
      rule:
        match:
          - output: "hello"`,
			err: `ERROR: <input>:6:7: only the rule or the output may be set
 |       rule:
 | ......^`,
		},
		{
			txt: `
rule:
  match:
    - condition: "true"
      rule:
        match:
          - output: "hello"
      output: "world"`,
			err: `ERROR: <input>:8:7: only the rule or the output may be set
 |       output: "world"
 | ......^`,
		},
		{
			txt: `
rule:
  match:
    - condition: "true"
      explanation: "hi"
      rule:
        match:
          - output: "hello"`,
			err: `ERROR: <input>:6:7: explanation can only be set on output match cases, not nested rules
 |       rule:
 | ......^`,
		},
		{
			txt: `
rule:
  match:
    - condition: "true"
      output: "'foo'"
  aggregate:
    - condition: "true"
      output: "'bar'"`,
			err: `ERROR: <input>:6:3: Only one of 'match' or 'aggregate' may be set in a rule
 |   aggregate:
 | ..^`,
		},
		{
			txt: `
rule:
  match:
    - condition: "true"
      rule:
        match:
          - output: "hello"
      explanation: "hi"`,
			err: `ERROR: <input>:8:7: explanation can only be set on output match cases, not nested rules
 |       explanation: "hi"
 | ......^`,
		},
		{
			txt: `
imports:
  - first`,
			err: `ERROR: <input>:3:5: got yaml node type tag:yaml.org,2002:str, wanted type(s) [tag:yaml.org,2002:map]
 |   - first
 | ....^`,
		},
		{
			txt: `
imports:
  first: name`,
			err: `ERROR: <input>:3:3: got yaml node type tag:yaml.org,2002:map, wanted type(s) [tag:yaml.org,2002:seq]
 |   first: name
 | ..^`,
		},
		{
			txt: `
rule:
  - variables: name`,
			err: `ERROR: <input>:3:3: got yaml node type tag:yaml.org,2002:seq, wanted type(s) [tag:yaml.org,2002:map]
 |   - variables: name
 | ..^`,
		},
		{
			txt: `
rule:
  variables: name`,
			err: `ERROR: <input>:3:14: got yaml node type tag:yaml.org,2002:str, wanted type(s) [tag:yaml.org,2002:seq]
 |   variables: name
 | .............^`,
		},
		{
			txt: `
rule:
  variables: 
    - name`,
			err: `ERROR: <input>:4:7: got yaml node type tag:yaml.org,2002:str, wanted type(s) [tag:yaml.org,2002:map]
 |     - name
 | ......^`,
		},
		{
			txt: `
rule:
  match: 
    name: value`,
			err: `ERROR: <input>:4:5: got yaml node type tag:yaml.org,2002:map, wanted type(s) [tag:yaml.org,2002:seq]
 |     name: value
 | ....^`,
		},
		{
			txt: `
rule:
  match: 
    - name`,
			err: `ERROR: <input>:4:7: got yaml node type tag:yaml.org,2002:str, wanted type(s) [tag:yaml.org,2002:map]
 |     - name
 | ......^`,
		},
		{
			txt: `
name: test
rule:
  match:
    - output: 'true'
  aggregate:
    - output: 'true'`,
			err: `ERROR: <input>:6:3: Only one of 'match' or 'aggregate' may be set in a rule
 |   aggregate:
 | ..^`,
		},
		{
			txt: `
name: test
rule:
  aggregate:
    - output: 'true'
  match:
    - output: 'true'`,
			err: `ERROR: <input>:6:3: Only one of 'match' or 'aggregate' may be set in a rule
 |   match:
 | ..^`,
		},
	}

	for _, tst := range tests {
		parser, err := NewParser()
		if err != nil {
			t.Fatalf("NewParser() failed: %v", err)
		}
		_, iss := parser.Parse(StringSource(tst.txt, "<input>"))
		if iss.Err() == nil {
			t.Fatalf("parser.Parse(%q) did not error, wanted %s", tst.txt, tst.err)
		}

		if iss.Err().Error() != tst.err {
			t.Errorf("parser.Parse(%q) got error %v, wanted error %s", tst.txt, iss.Err(), tst.err)
		}
	}
}

func TestGetExplanationOutputPolicy(t *testing.T) {
	tst := `
rule:
  match:
    - condition: "false"
      rule:
        match:
          - condition: "1 > 2"
            output: "false"
            explanation: "'bad_inner'"
          - output: "true"
            explanation: "'good_inner'"
    - output: "true"
      explanation: "'good_outer'"
    `

	parser, err := NewParser()
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(StringSource(tst, "<input>"))
	if iss != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	explanationPolicy := policy.GetExplanationOutputPolicy()

	want := "'bad_inner'"
	got := explanationPolicy.Rule().Matches()[0].rule.Matches()[0].output.Value
	if got != want {
		t.Errorf("First inner output = %v, wanted %v", got, want)
	}

	want = "1 > 2"
	got = explanationPolicy.Rule().Matches()[0].rule.Matches()[0].condition.Value
	if got != want {
		t.Errorf("First inner condition = %v, wanted %v", got, want)
	}

	want = "'good_inner'"
	got = explanationPolicy.Rule().Matches()[0].rule.Matches()[1].output.Value
	if got != want {
		t.Errorf("Second inner output = %v, wanted %v", got, want)
	}

	want = "'good_outer'"
	got = explanationPolicy.Rule().Matches()[1].output.Value
	if got != want {
		t.Errorf("Second outer output = %v, wanted %v", got, want)
	}
}

func TestCustomTagVisitor(t *testing.T) {
	tst := `name: "test"
description: |-2
   A test description.
version: 1
version: 2
last-modified: 2026-02-09
rule:
  match:
    - condition: "true"
      output: "true"
`

	handler := &testTagHandler{}
	parser, err := NewParser(CustomTagVisitor(handler))
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(StringSource(tst, "<input>"))
	if iss != nil {
		t.Fatalf("Parse() failed: %v", iss.Err())
	}

	if dx := cmp.Diff(" A test description.", handler.description); dx != "" {
		t.Errorf("handler.description (+got, -want): %s", dx)
	}
	if dx := cmp.Diff(" A test description.", policy.Description().Value); dx != "" {
		t.Errorf("policy.Description() (+got, -want): %s", dx)
	}

	if len(policy.MetadataKeys()) != 3 {
		t.Errorf("policy.MetadataKeys() got %v, wanted 2 keys", policy.MetadataKeys())
	}
	prevVer, found := policy.Metadata("prev.version")
	if !found {
		t.Fatalf("prev.version not found in policy.Metadata")
	}
	if prevVer != "1" {
		t.Errorf("prev.version got %s, wanted 1", prevVer)
	}
}

func TestSimpleVariables(t *testing.T) {
	tst := `name: "test"
rule:
  variables:
    - first: "1.5"
    - second: "2.5"
  match:
    - output: >
        variables.first + variables.second

`

	parser, err := NewParser(SimpleVariables())
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(StringSource(tst, "<input>"))
	if iss != nil {
		t.Fatalf("Parse() failed: %v", iss.Err())
	}
	env, err := cel.NewEnv(ext.Bindings())
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := Compile(env, policy)
	if iss != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	out, _, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out != types.Double(4.0) {
		t.Errorf("got %v, wanted 4.0", out)
	}
}

type testTagHandler struct {
	defaultTagVisitor

	description string
}

func (t *testTagHandler) PolicyTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, p *Policy) {
	if tagName == "description" {
		t.description = node.Value
		return
	}
	// Store the last value for a repeated tag
	if meta, found := p.Metadata(tagName); found {
		p.SetMetadata("prev."+tagName, meta)
		p.SetMetadata(tagName, node.Value)
	} else {
		p.SetMetadata(tagName, node.Value)
	}
}

func TestPolicyAndRuleSemanticMethods(t *testing.T) {
	p := NewPolicy(nil, nil)
	if p.Semantic() != firstMatch {
		t.Errorf("got %v, wanted firstMatch", p.Semantic())
	}
	p.SetSemantic(aggregate)
	if p.Semantic() != aggregate {
		t.Errorf("got %v, wanted aggregate", p.Semantic())
	}
	// Attempt to set conflicting semantic
	p.SetSemantic(firstMatch)
	if p.Semantic() != aggregate {
		t.Errorf("got %v, wanted aggregate after conflicting SetSemantic", p.Semantic())
	}

	r := NewRule(123)
	if r.SourceID() != 123 {
		t.Errorf("got %v, wanted 123", r.SourceID())
	}
	if r.Semantic() != firstMatch {
		t.Errorf("got %v, wanted firstMatch", r.Semantic())
	}
	r.SetSemantic(aggregate)
	if r.Semantic() != aggregate {
		t.Errorf("got %v, wanted aggregate", r.Semantic())
	}
	// Attempt to set conflicting semantic
	r.SetSemantic(firstMatch)
	if r.Semantic() != aggregate {
		t.Errorf("got %v, wanted aggregate after conflicting SetSemantic", r.Semantic())
	}
}
