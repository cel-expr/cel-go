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

package env

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseTypeDesc(t *testing.T) {
	tcs := []struct {
		text string
		want *TypeDesc
	}{
		{
			"int",
			NewTypeDesc("int"),
		},
		{
			"foo",
			NewTypeDesc("foo"),
		},
		{
			".com.example.Message",
			NewTypeDesc(".com.example.Message"),
		},
		{
			"list<int>",
			NewTypeDesc("list", NewTypeDesc("int")),
		},
		{
			" list < int > ",
			NewTypeDesc("list", NewTypeDesc("int")),
		},
		{
			"map<int, list<string>>",
			NewTypeDesc("map", NewTypeDesc("int"), NewTypeDesc("list", NewTypeDesc("string"))),
		},
		{
			"list<\n\tint\n>",
			NewTypeDesc("list", NewTypeDesc("int")),
		},
		{
			"map<\r\n  string,\r\n  list<string>\r\n>",
			NewTypeDesc("map", NewTypeDesc("string"), NewTypeDesc("list", NewTypeDesc("string"))),
		},
		{
			"map<string,\tint>",
			NewTypeDesc("map", NewTypeDesc("string"), NewTypeDesc("int")),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.text, func(t *testing.T) {
			got, err := ParseTypeDesc(tc.text)
			if err != nil {
				t.Fatalf("ParseTypeDesc(%q) = <err> %v, want nil", tc.text, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ParseTypeDesc(%q) = %s diff (-want +got):\n%s", tc.text, got, diff)
			}
		})
	}
}

func TestParseTypeDescErrors(t *testing.T) {
	tcs := []struct {
		text    string
		wantErr string
	}{
		{
			"",
			"missing identifier at position 0",
		},
		{
			"int int",
			"nexpected character 'i'",
		},
		{
			"int>",
			"unexpected character '>'",
		},
		{
			".foo.",
			"unexpected end of input",
		},
		{
			"..foo",
			"identifier is expected, but '.' was found at position 1",
		},
		{
			"~",
			"unexpected end of input",
		},
		{
			"~1",
			"invalid type parameter identifier '1' at position 1",
		},
		{
			"~elem",
			"invalid type param, must have a single alphabetic character at position 2",
		},
		{
			"list<",
			"missing identifier at position 5",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.text, func(t *testing.T) {
			_, err := ParseTypeDesc(tc.text)
			if err == nil {
				t.Fatalf("ParseTypeDesc(%q) = nil, wanted err %v", tc.text, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("ParseTypeDesc(%q) = %v, wanted err %v", tc.text, err, tc.wantErr)
			}
		})
	}
}

func TestConfigToYAML(t *testing.T) {
	// Parsing will accept shorthand for type specifiers, but we always output the
	// structured map nodes so not fully reversible.
	//
	tcs := []struct {
		name    string
		confIn  *Config
		yamlOut string
	}{
		{
			name: "string variable type",
			confIn: NewConfig("foo").AddVariables(
				NewVariable("foo", NewTypeDesc("int")),
			),
			yamlOut: `name: foo
variables:
    - name: foo
      type_name: int
`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			b, err := ConfigToYAML(tc.confIn)
			if err != nil {
				t.Fatalf("ConfigToYAML() = (err) %v", err)
			}
			fmt.Println(tc.confIn.Variables[0].Name, tc.confIn.Variables[0].TypeName)

			if diff := cmp.Diff(tc.yamlOut, string(b)); diff != "" {
				t.Errorf("ConfigToYAML() has diff (+got -want):\n%s", diff)
			}
		})
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	// Parsing will accept shorthand for type specifiers, but we always output the
	// structured map nodes so not fully reversible.
	//
	tcs := []struct {
		name    string
		yamlIn  string
		yamlOut string
	}{
		{
			name: "string variable type",
			yamlIn: `name: foo
variables:
    - name: foo
      type: int
`,
			yamlOut: `name: foo
variables:
    - name: foo
      type_name: int
`,
		},
		{
			name: "structured variable type",
			yamlIn: `name: foo
variables:
    - name: foo
      type_name: int
`,
			yamlOut: `name: foo
variables:
    - name: foo
      type_name: int
`,
		},
		{
			name: "map specifier type",
			yamlIn: `name: foo
variables:
    - name: foo
      type: map<int, string>
`,
			yamlOut: `name: foo
variables:
    - name: foo
      type_name: map
      params:
        - type_name: int
        - type_name: string
`,
		},
		{
			name: "function",
			yamlIn: `name: foo
functions:
    - name: getOrDefault
      overloads:
          - id: getOrDefault
            target: map<string, ~V>
            params:
                - string
                - ~V
            return: ~V
`,
			yamlOut: `name: foo
functions:
    - name: getOrDefault
      overloads:
        - id: getOrDefault
          target:
            type_name: map
            params:
                - type_name: string
                - type_name: V
                  is_type_param: true
          return:
            type_name: V
            is_type_param: true
`,
		},
		{
			name: "multiline and tab whitespace in types",
			yamlIn: `name: user_env
variables:
  - name: user_lookup_table
    type: >-
      map<
        string,
        list<string>
      >
  - name: permissions
    type: "map<string,\tint>"
`,
			yamlOut: `name: user_env
variables:
    - name: user_lookup_table
      type_name: map
      params:
        - type_name: string
        - type_name: list
          params:
            - type_name: string
    - name: permissions
      type_name: map
      params:
        - type_name: string
        - type_name: int
`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ConfigFromYAML([]byte(tc.yamlIn))
			if err != nil {
				t.Fatalf("ConfigFromYAML() = (err) %v, want nil", err)
			}
			b, err := ConfigToYAML(c)
			if err != nil {
				t.Fatalf("ConfigToYAML() = (err) %v, wanted nil", err)
			}
			if diff := cmp.Diff(tc.yamlOut, string(b)); diff != "" {
				t.Errorf("ConfigToYAML() has diff (+got -want):\n%s", diff)
			}
		})
	}
}
