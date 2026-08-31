// Copyright 2025 Google LLC
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

package ext

import (
	"fmt"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
)

func TestRegex(t *testing.T) {
	regexTests := []struct {
		expr string
	}{
		// Tests for replace Function
		{expr: "regex.replace('abc', '^', 'start_') == 'start_abc'"},
		{expr: "regex.replace('abc', '$', '_end') == 'abc_end'"},
		{expr: `regex.replace('a-b', r'\b', '|') == '|a|-|b|'`},
		{expr: `regex.replace('foo bar', '(fo)o (ba)r', r'\2 \1') == 'ba fo'`},
		{expr: `regex.replace('foo bar', 'foo', r'\\') == '\\ bar'`},
		{expr: "regex.replace('banana', 'ana', 'x') == 'bxna'"},
		{expr: `regex.replace('abc', 'b(.)', r'x\1') == 'axc'`},
		{expr: "regex.replace('hello world hello', 'hello', 'hi') == 'hi world hi'"},
		{expr: `regex.replace('ac', 'a(b)?c', r'[\1]') == '[]'`},
		{expr: "regex.replace('apple pie', 'p', 'X') == 'aXXle Xie'"},
		{expr: `regex.replace('remove all spaces', r'\s', '') == 'removeallspaces'`},
		{expr: `regex.replace('digit:99919291992', r'\d+', '3') == 'digit:3'`},
		{expr: `regex.replace('foo bar baz', r'\w+', r'(\0)') == '(foo) (bar) (baz)'`},
		{expr: "regex.replace('', 'a', 'b') == ''"},
		{expr: `regex.replace('User: Alice, Age: 30', r'User: (?P<name>\w+), Age: (?P<age>\d+)', '${name} is ${age} years old') == '${name} is ${age} years old'`},
		{expr: `regex.replace('User: Alice, Age: 30', r'User: (?P<name>\w+), Age: (?P<age>\d+)', r'\1 is \2 years old') == 'Alice is 30 years old'`},
		{expr: "regex.replace('hello ☃', '☃', '❄') == 'hello ❄'"},
		{expr: `regex.replace('id=123', r'id=(?P<value>\d+)', r'value: \1') == 'value: 123'`},
		{expr: "regex.replace('banana', 'a', 'x') == 'bxnxnx'"},
		{expr: `regex.replace(regex.replace('%(foo) %(bar) %2', r'%\((\w+)\)', r'${\1}'),r'%(\d+)', r'$\1') == '${foo} ${bar} $2'`},

		// Tests for replace Function with count variable
		{expr: "regex.replace('banana', 'a', 'x', 0) == 'banana'"},
		{expr: "regex.replace('banana', 'a', 'x', 1) == 'bxnana'"},
		{expr: "regex.replace('banana', 'a', 'x', 2) == 'bxnxna'"},
		{expr: "regex.replace('banana', 'a', 'x', 100) == 'bxnxnx'"},
		{expr: "regex.replace('banana', 'a', 'x', -1) == 'bxnxnx'"},
		{expr: "regex.replace('banana', 'a', 'x', -100) == 'bxnxnx'"},
		{expr: `regex.replace('cat-dog dog-cat cat-dog dog-cat', '(cat)-(dog)', r'\2-\1', 1) == 'dog-cat dog-cat cat-dog dog-cat'`},
		{expr: `regex.replace('cat-dog dog-cat cat-dog dog-cat', '(cat)-(dog)', r'\2-\1', 2) == 'dog-cat dog-cat dog-cat dog-cat'`},
		{expr: `regex.replace('a.b.c', r'\.', '-', 1) == 'a-b.c'`},
		{expr: `regex.replace('a.b.c', r'\.', '-', -1) == 'a-b-c'`},
		{expr: `regex.replace('abc def', r'(abc)', r'\\1') == r'\1 def'`},
		{expr: `regex.replace('abc def', r'(abc)', r'\\2') == r'\2 def'`},
		{expr: `regex.replace('abc def', r'(abc)', r'\\{word}') == '\\{word} def'`},
		{expr: `regex.replace('abc def', r'(abc)', r'\\word') == '\\word def'`},

		// Tests for extract Function
		{expr: "regex.extract('hello world', 'hello(.*)') == optional.of(' world')"},
		{expr: `regex.extract('item-A, item-B', r'item-(\w+)') == optional.of('A')`},
		{expr: `regex.extract('The color is red', r'The color is (\w+)') == optional.of('red')`},
		{expr: `regex.extract('The color is red', r'The color is \w+') == optional.of('The color is red')`},
		{expr: "regex.extract('brand', 'brand') == optional.of('brand')"},
		{expr: "regex.extract('hello world', 'goodbye (.*)') == optional.none()"},
		{expr: "regex.extract('HELLO', 'hello') == optional.none()"},
		{expr: `regex.extract('', r'\w+') == optional.none()`},
		{expr: "regex.extract('4122345432', '22').or(optional.of('777')) == optional.of('22')"},
		{expr: "regex.extract('4122345432', '22').orValue('777') == '22'"},

		// Tests for extractAll Function
		{expr: "regex.extractAll('id:123, id:456', 'assa') == []"},
		{expr: `regex.extractAll('id:123, id:456', r'id:\d+') == ['id:123', 'id:456']`},
		{expr: `regex.extractAll('Files: f_1.txt, f_2.csv', r'f_(\d+)') == ['1', '2']`},
		{expr: "regex.extractAll('testuser@', '(?P<username>.*)@') == ['testuser']"},
		{expr: "regex.extractAll('testuser@gmail.com, a@y.com, 2312321wsamkldjq2w2@sdad.com', '(?P<username>.*)@') == ['testuser@gmail.com, a@y.com, 2312321wsamkldjq2w2']"},
		{expr: `regex.extractAll('testuser@gmail.com, a@y.com, 2312321wsamkldjq2w2@sdad.com', r'(?P<username>\w+)@') == ['testuser', 'a', '2312321wsamkldjq2w2']`},
		{expr: "regex.extractAll('banananana', '(ana)') == ['ana', 'ana']"},
		{expr: `regex.extractAll('item:a1, topic:b2', r'(?:item:|topic:)([a-z]\d)') == ['a1', 'b2']`},
		{expr: "regex.extractAll('val=a, val=, val=c', 'val=([^,]*)') == ['a', 'c']"},
		{expr: "regex.extractAll('key=, key=, key=', 'key=([^,]*)') == []"},
		{expr: `regex.extractAll('a b c', r'(\S*)\s*') == ['a', 'b', 'c']`},
	}

	env := testRegexEnv(t)
	for i, tst := range regexTests {
		tr := tst // capture range variable
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tr.expr)
			if iss.Err() != nil {
				t.Fatalf("Parse(%s) failed: %v", tr.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("Check(%s) failed: %v", tr.expr, iss.Err())
			}
			asts = append(asts, cAst)

			for _, ast := range asts {
				prog, err := env.Program(ast)
				if err != nil {
					t.Fatalf("Program(%s) failed: %v", tr.expr, err)
				}
				out, _, err := prog.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("Eval(%s) failed: %v", tr.expr, err)
				}
				if out.Value() != true {
					t.Errorf("Eval(%s) returned %v, want true", tr.expr, out.Value())
				}
			}
		})
	}
}

func TestRegexStaticErrors(t *testing.T) {
	tests := []struct {
		expr string
		err  string
	}{
		{
			expr: "regex.replace('abc', '^', 1)",
			err:  "found no matching overload for 'regex.replace' applied to '(string, string, int)'",
		},
		{
			expr: "regex.replace('abc', '^', '1','')",
			err:  "found no matching overload for 'regex.replace' applied to '(string, string, string, string)'",
		},
		{
			expr: "regex.extract('foo bar', 1)",
			err:  "found no matching overload for 'regex.extract' applied to '(string, int)'",
		},
		{
			expr: "regex.extract('foo bar', 1, 'bar')",
			err:  "found no matching overload for 'regex.extract' applied to '(string, int, string)'",
		},
		{
			expr: "regex.extractAll()",
			err:  "found no matching overload for 'regex.extractAll' applied to '()'",
		},
		{
			expr: "regex.replace('banana', 'a', 'x', 18446744073709551615)",
			err:  `ERROR: <input>:1:35: invalid int literal`,
		},
	}
	env := testRegexEnv(t)
	for i, tst := range tests {
		tr := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, iss := env.Compile(tr.expr)
			if iss.Err() == nil || !strings.Contains(iss.Err().Error(), tr.err) {
				t.Errorf("env.Compile(%q) got error %v, wanted %v", tr.expr, iss.Err(), tr.err)
			}
		})
	}
}

func TestRegexRuntimeErrors(t *testing.T) {
	tests := []struct {
		expr string
		err  string
	}{
		{
			expr: "regex.extract('foo bar', '(')",
			err:  "error parsing regexp: missing closing ): `(`",
		},
		{
			expr: "regex.extractAll('foo bar', '[a-z')",
			err:  "error parsing regexp: missing closing ]: `[a-z`",
		},
		{
			expr: `regex.replace('id=123', r'id=(?P<value>\d+)', r'value: \values')`,
			err:  `invalid replacement string: 'value: \values' \ must be followed by a digit`,
		},
		{
			expr: `regex.replace('test', '(.)', r'\2')`,
			err:  "replacement string references group 2 but regex has only 1 group(s)",
		},
		{
			expr: `regex.replace('id=123', r'id=(?P<value>\d+)', r'value: \')`,
			err:  `invalid replacement string: 'value: \' \ not allowed at end`,
		},
		{
			expr: `regex.extract('phone: 415-5551212', r'phone: ((\d{3})-)?')`,
			err:  `regular expression has more than one capturing group: "phone: ((\\d{3})-)?"`,
		},
		{
			expr: `regex.extractAll('Name: John Doe, Age:321', r'Name: (?P<Name>.*), Age:(?P<Age>\d+)')`,
			err:  `regular expression has more than one capturing group: "Name: (?P<Name>.*), Age:(?P<Age>\\d+)"`,
		},
		{
			expr: `regex.extractAll('testuser@testdomain', '(.*)@([^.]*)')`,
			err:  `regular expression has more than one capturing group: "(.*)@([^.]*)"`,
		},
		{
			expr: `regex.extractAll('The user testuser belongs to testdomain', 'The (user|domain) (?P<Username>.*) belongs (to) (?P<Domain>.*)')`,
			err:  `regular expression has more than one capturing group: "The (user|domain) (?P<Username>.*) belongs (to) (?P<Domain>.*)"`,
		},
	}

	env := testRegexEnv(t)
	for i, tst := range tests {
		tr := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ast, iss := env.Compile(tr.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed with error %v", tr.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program(ast) failed: %v", err)
			}
			in := cel.NoVars()
			_, _, err = prg.Eval(in)
			if err == nil || !strings.Contains(err.Error(), tr.err) {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tr.err)
			}
		})
	}
}

func TestRegexEnvCreationErrors(t *testing.T) {
	tests := []struct {
		name string
		opts []cel.EnvOption
	}{
		{
			name: "no optional types",
			opts: []cel.EnvOption{Regex()},
		},
		{
			name: "optional types after regex",
			opts: []cel.EnvOption{Regex(), cel.OptionalTypes()},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cel.NewEnv(tc.opts...)
			if err == nil || !strings.Contains(err.Error(), "regex library requires the optional library") {
				t.Fatalf("prg.Eval() got %v, wanted regex library requires the optional library", err)
			}
		})
	}
}

func TestRegexVersion(t *testing.T) {
	_, err := cel.NewEnv(cel.OptionalTypes(), Regex(RegexVersion(0)))
	if err != nil {
		t.Fatalf("Regex(0) failed: %v", err)
	}
}

func testRegexEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{
		cel.OptionalTypes(),
		Regex(),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Regex()) failed: %v", err)
	}
	return env
}

func TestRegexCosts(t *testing.T) {
	tests := []struct {
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost cost.CostEstimate
		actualCost    uint64
	}{
		{
			expr:          `regex.extract('hello world', 'hello (.*)') == optional.of('world')`,
			estimatedCost: cost.CostEstimate{Min: 8, Max: 20},
			actualCost:    8,
		},
		// - Estimated Cost (Min: 5): Derived from fixed costs of scanning 10-char
		//   target string and compiling 2-char regex. Since the inputs are constant,
		//   the Min estimate is very accurate.
		// - Actual Cost (5): Observed cost is the sum of a base call cost (~1), the
		//   search cost, and the allocation cost for the 2-char result string '22'.
		//   It aligns perfectly with the minimum estimate.
		{
			expr:          "regex.extract('4122345432', '22').orValue('777') == '22'",
			estimatedCost: cost.CostEstimate{Min: 4, Max: 14},
			actualCost:    4,
		},
		// .or() condition introduces runtime uncertainty, and since the cost estimator
		// can't know which branch the code will take, it must prepare for the most
		// expensive possible outcome resulting in an estimate.Max of ~ math.MaxUint64.
		{
			expr:          "regex.extract('4122345432', '22').or(optional.of('777')) == optional.of('22')",
			estimatedCost: cost.CostEstimate{Min: 6, Max: 1844674407370955279},
			actualCost:    5,
		},
		{
			expr:          "regex.extract('hello world', 'goodbye (.*)') == optional.none()",
			estimatedCost: cost.CostEstimate{Min: 10, Max: 22},
			actualCost:    8,
		},
		{
			expr:          "regex.extractAll('id:123, id:456', 'assa') == []",
			estimatedCost: cost.CostEstimate{Min: 24, Max: 38},
			actualCost:    23,
		},
		// - Estimated Cost (Min: 25): Cost to scan the 14-char target and compile
		//   5-char regex, plus a worst-case allocation cost for the result list's
		//   contents, which is estimated as the full 14-char size of the target.
		// - Actual Cost (28): Observed cost includes the search cost plus the actual
		//   allocation cost, which is the base list creation cost plus the cost of
		//   allocating the two result strings, totaling 12 chars of content.
		{
			expr:          `regex.extractAll('id:123, id:456', r'id:\d+') == ['id:123', 'id:456']`,
			estimatedCost: cost.CostEstimate{Min: 25, Max: 39},
			actualCost:    27,
		},
		{
			expr:          `regex.extractAll('a b c', r'(\S*)\s*') == ['a', 'b', 'c']`,
			estimatedCost: cost.CostEstimate{Min: 24, Max: 29},
			actualCost:    27,
		},
		{
			expr:          `regex.extractAll('testuser@gmail.com, a@y.com, 2312321wsamkldjq2w2@sdad.com', r'(?P<username>\w+)@') == ['testuser', 'a', '2312321wsamkldjq2w2']`,
			estimatedCost: cost.CostEstimate{Min: 51, Max: 108},
			actualCost:    53,
		},
		{
			expr:          "regex.replace('hello world hello', 'hello', 'hi') == 'hi world hi'",
			estimatedCost: cost.CostEstimate{Min: 22, Max: 40},
			actualCost:    16,
		},
		{
			expr:          `regex.replace('ac', 'a(b)?c', r'[\1]') == '[]'`,
			estimatedCost: cost.CostEstimate{Min: 5, Max: 11},
			actualCost:    4,
		},
		{
			expr:          "regex.replace('apple pie', 'p', 'X') == 'aXXle Xie'",
			estimatedCost: cost.CostEstimate{Min: 11, Max: 11},
			actualCost:    11,
		},
		{
			expr:          "regex.replace('aaaaaa', 'a', '-what-') == '-what--what--what--what--what--what-'",
			estimatedCost: cost.CostEstimate{Min: 8, Max: 41},
			actualCost:    41,
		},
		// --- Constant Cost Cases ---
		// These cases demonstrate that the cost is independent of the `count` arg.
		// - Estimated Cost (9): Estimator does not evaluate the value of `count`.
		//  It assumes the worst case (all possible replacements) for any `count`.
		// 	Since the other inputs are identical, the estimate is constant.
		// - Actual Cost (9): The tracker's cost is also modeled on the worst case
		//  for consistency. The search phase of the operation scans the whole
		//  string for all matches, so the dominant cost is constant.
		{
			expr:          "regex.replace('banana', 'a', 'x', 0) == 'banana'",
			estimatedCost: cost.CostEstimate{Min: 8, Max: 8},
			actualCost:    8,
		},
		{
			expr:          "regex.replace('banana', 'a', 'x', 1) == 'bxnana'",
			estimatedCost: cost.CostEstimate{Min: 8, Max: 8},
			actualCost:    8,
		},
		{
			expr:          "regex.replace('banana', 'a', 'x', 100) == 'bxnxnx'",
			estimatedCost: cost.CostEstimate{Min: 8, Max: 8},
			actualCost:    8,
		},
		{
			expr:          `regex.replace('foo bar', r'(foo bar)', r'\1\1\1\1\1' ) == 'foo barfoo barfoo barfoo barfoo bar'`,
			estimatedCost: cost.CostEstimate{Min: 11, Max: 77},
			actualCost:    42,
		},
		{
			expr:          `regex.replace('foo bar', r'(foo bar)', '') == ''`,
			estimatedCost: cost.CostEstimate{Min: 3, Max: 10},
			actualCost:    3,
		},
	}
	for _, test := range tests {
		tc := test
		t.Run(tc.expr, func(t *testing.T) {
			env := testRegexEnv(t, tc.vars...)
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Parse(%s) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("Check(%s) failed: %v", tc.expr, iss.Err())
			}

			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost)
			}
		})
	}
}

func TestRegexProgramSizeLimit(t *testing.T) {
	overloads := []struct {
		name string
		expr string
	}{
		{
			name: "matches",
			expr: `'a1'.matches(pat)`,
		},
		{
			name: "regex.extract",
			expr: `regex.extract('a1', pat)`,
		},
		{
			name: "regex.extractAll",
			expr: `regex.extractAll('a1', pat)`,
		},
		{
			name: "regex.replace 3-arg",
			expr: `regex.replace('a1', pat, 'x')`,
		},
		{
			name: "regex.replace 4-arg",
			expr: `regex.replace('a1', pat, 'x', 1)`,
		},
	}

	t.Run("ExceedsLimit", func(t *testing.T) {
		for _, tc := range overloads {
			t.Run(tc.name, func(t *testing.T) {
				prg, err := cel.Compile(tc.expr,
					cel.OptionalTypes(),
					Regex(),
					cel.RegexProgramSizeLimit(5),
					cel.Variable("pat", cel.StringType),
				)
				if err != nil {
					t.Fatalf("cel.Compile(%s) failed: %v", tc.expr, err)
				}
				_, _, err = prg.Eval(map[string]any{"pat": "(a|b)*[0-9]+"})
				if err == nil {
					t.Fatalf("expected runtime error for regex program size exceeding limit")
				}
				if !strings.Contains(err.Error(), "regex program size 8 exceeds limit of 5") {
					t.Fatalf("got error %v, expected error containing 'regex program size 8 exceeds limit of 5'", err)
				}
			})
		}
	})

	t.Run("WithinLimit", func(t *testing.T) {
		for _, tc := range overloads {
			t.Run(tc.name, func(t *testing.T) {
				prg, err := cel.Compile(tc.expr,
					cel.OptionalTypes(),
					Regex(),
					cel.RegexProgramSizeLimit(10),
					cel.Variable("pat", cel.StringType),
				)
				if err != nil {
					t.Fatalf("cel.Compile(%s) failed: %v", tc.expr, err)
				}
				val, _, err := prg.Eval(map[string]any{"pat": "(a|b)*[0-9]+"})
				if err != nil {
					t.Fatalf("prg.Eval(%s) unexpected error: %v", tc.expr, err)
				}
				if val == nil {
					t.Fatalf("prg.Eval(%s) returned nil result", tc.expr)
				}
			})
		}
	})
}
