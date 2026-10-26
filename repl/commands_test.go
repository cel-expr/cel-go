// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repl

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func cmdMatches(t testing.TB, got Cmder, expected Cmder) (result bool) {
	t.Helper()

	defer func() {
		// catch type assertion errors
		if v := recover(); v != nil {
			result = false
		}
	}()

	switch want := expected.(type) {
	case *compileCmd:
		gotCompile := got.(*compileCmd)
		return gotCompile.expr == want.expr
	case *evalCmd:
		gotEval := got.(*evalCmd)
		return gotEval.expr == want.expr
	case *delCmd:
		gotDel := got.(*delCmd)
		return gotDel.identifier == want.identifier
	case *simpleCmd:
		gotSimple := got.(*simpleCmd)
		if len(gotSimple.args) != len(want.args) {
			return false
		}
		for i, a := range want.args {
			if gotSimple.args[i] != a {
				return false
			}
		}
		return gotSimple.cmd == want.cmd
	case *letFnCmd:
		gotLetFn := got.(*letFnCmd)
		if gotLetFn.identifier != want.identifier ||
			gotLetFn.src != want.src ||
			!reflect.DeepEqual(gotLetFn.resultType, want.resultType) ||
			len(gotLetFn.params) != len(want.params) {
			return false
		}
		for i, wantP := range want.params {
			if gotLetFn.params[i].identifier != wantP.identifier ||
				!reflect.DeepEqual(gotLetFn.params[i].typeHint, wantP.typeHint) {
				return false
			}
		}
		return true
	case *letVarCmd:
		gotLetVar := got.(*letVarCmd)
		return gotLetVar.identifier == want.identifier &&
			reflect.DeepEqual(gotLetVar.typeHint, want.typeHint) &&
			gotLetVar.src == want.src
	}
	return false
}

func (c *evalCmd) String() string {
	return fmt.Sprintf("%%eval %s", c.expr)
}

func (c *letVarCmd) String() string {
	return fmt.Sprintf("%%let %s : %s = %s", c.identifier, c.typeHint.SpecifierFormat(), c.src)
}

func fmtParam(p letFunctionParam) string {
	return fmt.Sprintf("%s : %s", p.identifier, p.typeHint.SpecifierFormat())
}

func fmtParams(ps []letFunctionParam) string {
	buf := make([]string, len(ps))
	for i, p := range ps {
		buf[i] = fmtParam(p)
	}
	return strings.Join(buf, ", ")
}

func (c *letFnCmd) String() string {
	return fmt.Sprintf("%%let %s (%s) : %s -> %s", c.identifier, fmtParams(c.params), c.resultType.SpecifierFormat(), c.src)
}

func (c *delCmd) String() string {
	return fmt.Sprintf("%%delete %s", c.identifier)
}

func (c *simpleCmd) String() string {
	flagFmt := strings.Join(c.args, " ")
	return fmt.Sprintf("%%%s %s", c.cmd, flagFmt)
}

func TestParse(t *testing.T) {
	var testCases = []struct {
		commandLine string
		wantCmd     Cmder
		wantErr     error
	}{
		{
			commandLine: "%let x = 1",
			wantCmd: &letVarCmd{
				identifier: "x",
				typeHint:   nil,
				src:        "1",
			},
		},
		{
			commandLine: "%let com.google.x = 1",
			wantCmd: &letVarCmd{
				identifier: "com.google.x",
				typeHint:   nil,
				src:        "1",
			},
		},
		{
			commandLine: "%let x: int = 1",
			wantCmd: &letVarCmd{
				identifier: "x",
				typeHint:   mustParseType(t, "int"),
				src:        "1",
			},
		},
		{
			commandLine: `%compile x + 2`,
			wantCmd:     &compileCmd{expr: "x + 2"},
		},
		{
			commandLine: `%eval x + 2`,
			wantCmd:     &evalCmd{expr: "x + 2"},
		},
		{
			commandLine: "x + 2",
			wantCmd:     &evalCmd{expr: "x + 2"},
		},
		{
			commandLine: `%exit`,
			wantCmd:     &simpleCmd{cmd: "exit"},
		},
		{
			commandLine: `%help`,
			wantErr: errors.New(`Compile emits a textproto representation of the compiled expression.
            %compile <expr>

            Parse emits a textproto representation of the parsed expression.
            %parse <expr>
            
            Declare introduces a variable or function for type checking, but
            doesn't define a value for it:
            %declare <identifier> : <type>
            %declare <identifier> (<param_identifier> : <param_type>, ...) : <result-type>
            
            Delete removes a variable or function declaration from the evaluation context.
            %delete <identifier>
            
            Let introduces a variable or function defined by a sub-CEL expression.
            %let <identifier> (: <type>)? = <expr>
            %let <identifier> (<param_identifier> : <param_type>, ...) : <result-type> -> <expr>

            Status prints the current state of the REPL session
            %status
            %status --yaml

            Config loads a canned REPL state from a config file
            %configure """%let foo : int = 42"""
            %configure --yaml --file 'path/to/env.yaml'

            Option enables a CEL environment option which enables configuration and
            optional language features.
            %option --container 'google.protobuf'
            %option --extension 'all'

            LoadDescriptors loads a protobuf descriptor file (google.protobuf.FileDescriptorSet)
            from disk or from a predefined package. Supported packages are "cel-spec-test-types"
            (TestAllTypes) and "google-rpc" (AttributeContext).
            %load_descriptors 'path/to/descriptor_set.binarypb'
            %load_descriptors --pkg 'cel-spec-test-types'
            
            Help prints usage information for the commands supported by the REPL.
            %help
            
            Exit terminates the REPL.
            %exit`),
		},
		{
			commandLine: `%arbitrary --flag --another-flag 'string literal\n'`,
			wantCmd: &simpleCmd{cmd: "arbitrary",
				args: []string{
					"--flag", "--another-flag", "string literal\\n",
				},
			},
		},
		{
			commandLine: "   ",
			wantCmd:     &simpleCmd{cmd: "null"},
		},
		{
			commandLine: `%delete x`,
			wantCmd:     &delCmd{identifier: "x"},
		},
		{
			commandLine: `%delete com.google.x`,
			wantCmd:     &delCmd{identifier: "com.google.x"},
		},
		{
			commandLine: `%declare x: int`,
			wantCmd: &letVarCmd{
				identifier: "x",
				typeHint:   mustParseType(t, "int"),
				src:        "",
			},
		},
		{
			commandLine: `%let fn (x : int) : int -> x + 2`,
			wantCmd: &letFnCmd{
				identifier: "fn",
				params: []letFunctionParam{
					{identifier: "x", typeHint: mustParseType(t, "int")},
				},
				resultType: mustParseType(t, "int"),
				src:        "x + 2",
			},
		},
		{
			commandLine: `%let int.plus (x : int) : int -> this + x`,
			wantCmd: &letFnCmd{
				identifier: "int.plus",
				params: []letFunctionParam{
					{identifier: "x", typeHint: mustParseType(t, "int")},
				},
				resultType: mustParseType(t, "int"),
				src:        "this + x",
			},
		},
		{
			commandLine: `%let fn () : int -> 2 + 2`,
			wantCmd: &letFnCmd{
				identifier: "fn",
				params:     []letFunctionParam{},
				resultType: mustParseType(t, "int"),
				src:        "2 + 2",
			},
		},
		{
			commandLine: `%let fn (x:int, y :int) : int -> x + y`,
			wantCmd: &letFnCmd{
				identifier: "fn",
				params: []letFunctionParam{
					{identifier: "x", typeHint: mustParseType(t, "int")},
					{identifier: "y", typeHint: mustParseType(t, "int")},
				},
				resultType: mustParseType(t, "int"),
				src:        "x + y",
			},
		},
		{
			commandLine: `%declare fn (x : int) : int`,
			wantCmd: &letFnCmd{
				identifier: "fn",
				params: []letFunctionParam{
					{identifier: "x", typeHint: mustParseType(t, "int")},
				},
				resultType: mustParseType(t, "int"),
				src:        "",
			},
		},
		{
			commandLine: `%eval --parse-only -- foo`,
			wantCmd: &evalCmd{
				parseOnly: true,
				expr:      `foo`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.commandLine, func(t *testing.T) {
			cmd, err := Parse(tc.commandLine)
			if err != nil {
				if tc.wantErr != nil && stripWhitespace(tc.wantErr.Error()) == stripWhitespace(err.Error()) {
					return
				}
				t.Errorf("Parse(\"%s\") failed: %s", tc.commandLine, err)
			}
			if cmd == nil || !cmdMatches(t, cmd, tc.wantCmd) {
				t.Errorf("Parse('%s') got (%s) wanted (%s)", tc.commandLine,
					cmd, tc.wantCmd)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	var testCases = []struct {
		commandLine string
	}{
		{
			// not an identifier
			commandLine: "%let 123 = 1",
		},
		{
			// no assignment
			commandLine: "%let x: int",
		},
		{
			// no assignment
			commandLine: "%let fn() : int",
		},
		{
			// missing types
			commandLine: "%let fn (x) -> x + 1",
		},
		{
			// missing arg id
			commandLine: "%let fn (: int) : int -> x + 1",
		},
		{
			// type required for declare
			commandLine: "%declare x",
		},
		{
			// not an identifier
			commandLine: "%declare 123",
		},
		{
			// not an identifier
			commandLine: "%declare 1() : int",
		},
		{
			// not an identifier
			commandLine: "%delete 123",
		},
		{
			commandLine: "%eval --badflag foo",
		},
		{
			commandLine: "%eval --bad-flag foo",
		},
		{
			commandLine: "%eval -badflag foo",
		},
		{
			commandLine: "%eval -bad-flag foo",
		},
		{
			commandLine: "%eval --parse-only foo",
		},
		{
			commandLine: "%eval -parse-only foo",
		},
	}
	for _, tc := range testCases {
		_, err := Parse(tc.commandLine)
		if err == nil {
			t.Errorf("Parse(\"%s\") ok wanted error", tc.commandLine)
		}
	}
}
