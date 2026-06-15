// Copyright 2020 Google LLC
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
	"time"
	"unicode/utf8"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// TODO: move these tests to a conformance test.
var stringTests = []struct {
	expr      string
	err       string
	parseOnly bool
}{
	// CharAt test.
	{expr: `'tacocat'.charAt(3) == 'o'`},
	{expr: `'tacocat'.charAt(7) == ''`},
	{expr: `'©αT'.charAt(0) == '©' && '©αT'.charAt(1) == 'α' && '©αT'.charAt(2) == 'T'`},
	// Index of search string tests.
	{expr: `'tacocat'.indexOf('') == 0`},
	{expr: `'tacocat'.indexOf('ac') == 1`},
	{expr: `'tacocat'.indexOf('none') == -1`},
	{expr: `'tacocat'.indexOf('', 3) == 3`},
	{expr: `'tacocat'.indexOf('a', 3) == 5`},
	{expr: `'tacocat'.indexOf('at', 3) == 5`},
	{expr: `'ta©o©αT'.indexOf('©') == 2`},
	{expr: `'ta©o©αT'.indexOf('©', 3) == 4`},
	{expr: `'ta©o©αT'.indexOf('©αT', 3) == 4`},
	{expr: `'ta©o©αT'.indexOf('©α', 5) == -1`},
	{expr: `'ijk'.indexOf('k') == 2`},
	{expr: `'hello wello'.indexOf('hello wello') == 0`},
	{expr: `'hello wello'.indexOf('ello', 6) == 7`},
	{expr: `'hello wello'.indexOf('elbo room!!') == -1`},
	{expr: `'hello wello'.indexOf('elbo room!!!') == -1`},
	{expr: `''.lastIndexOf('@@') == -1`},
	{expr: `'tacocat'.lastIndexOf('') == 7`},
	{expr: `'tacocat'.lastIndexOf('at') == 5`},
	{expr: `'tacocat'.lastIndexOf('none') == -1`},
	{expr: `'tacocat'.lastIndexOf('', 3) == 3`},
	{expr: `'tacocat'.lastIndexOf('a', 3) == 1`},
	{expr: `'ta©o©αT'.lastIndexOf('©') == 4`},
	{expr: `'ta©o©αT'.lastIndexOf('©', 3) == 2`},
	{expr: `'ta©o©αT'.lastIndexOf('©α', 4) == 4`},
	{expr: `'hello wello'.lastIndexOf('ello', 6) == 1`},
	{expr: `'hello wello'.lastIndexOf('low') == -1`},
	{expr: `'hello wello'.lastIndexOf('elbo room!!') == -1`},
	{expr: `'hello wello'.lastIndexOf('elbo room!!!') == -1`},
	{expr: `'hello wello'.lastIndexOf('hello wello') == 0`},
	{expr: `'bananananana'.lastIndexOf('nana', 7) == 6`},
	// Lower ASCII tests.
	{expr: `'TacoCat'.lowerAscii() == 'tacocat'`},
	{expr: `'TacoCÆt'.lowerAscii() == 'tacocÆt'`},
	{expr: `'TacoCÆt Xii'.lowerAscii() == 'tacocÆt xii'`},
	// Replace tests
	{expr: `"12 days 12 hours".replace("{0}", "2") == "12 days 12 hours"`},
	{expr: `"{0} days {0} hours".replace("{0}", "2") == "2 days 2 hours"`},
	{expr: `"{0} days {0} hours".replace("{0}", "2", 1).replace("{0}", "23") == "2 days 23 hours"`},
	{expr: `"1 ©αT taco".replace("αT", "o©α") == "1 ©o©α taco"`},
	{expr: `"hello hello".replace("", "_") == "_h_e_l_l_o_ _h_e_l_l_o_"`},
	{expr: `"hello hello".replace("h", "") == "ello ello"`},
	// Split tests.
	{expr: `"hello world".split(" ") == ["hello", "world"]`},
	{expr: `"hello world events!".split(" ", 0) == []`},
	{expr: `"hello world events!".split(" ", 1) == ["hello world events!"]`},
	{expr: `"o©o©o©o".split("©", -1) == ["o", "o", "o", "o"]`},
	// Substring tests.
	{expr: `"tacocat".substring(4) == "cat"`},
	{expr: `"tacocat".substring(7) == ""`},
	{expr: `"tacocat".substring(0, 4) == "taco"`},
	{expr: `"tacocat".substring(4, 4) == ""`},
	{expr: `'ta©o©αT'.substring(2, 6) == "©o©α"`},
	{expr: `'ta©o©αT'.substring(7, 7) == ""`},
	// Trim tests using the unicode standard for whitespace.
	{expr: `" \f\n\r\t\vtext  ".trim() == "text"`},
	{expr: `"\u0085\u00a0\u1680text".trim() == "text"`},
	{expr: `"text\u2000\u2001\u2002\u2003\u2004\u2004\u2006\u2007\u2008\u2009".trim() == "text"`},
	{expr: `"\u200atext\u2028\u2029\u202F\u205F\u3000".trim() == "text"`},
	// Trim test with whitespace-like characters not included.
	{expr: `"\u180etext\u200b\u200c\u200d\u2060\ufeff".trim()
				== "\u180etext\u200b\u200c\u200d\u2060\ufeff"`},
	// Upper ASCII tests.
	{expr: `'tacoCat'.upperAscii() == 'TACOCAT'`},
	{expr: `'tacoCαt'.upperAscii() == 'TACOCαT'`},
	// Reverse tests.
	{expr: `'gums'.reverse() == 'smug'`},
	{expr: `'palindromes'.reverse() == 'semordnilap'`},
	{expr: `'John Smith'.reverse() == 'htimS nhoJ'`},
	{expr: `'u180etext'.reverse() == 'txete081u'`},
	{expr: `'2600+U'.reverse() == 'U+0062'`},
	{expr: `'\u180e\u200b\u200c\u200d\u2060\ufeff'.reverse() == '\ufeff\u2060\u200d\u200c\u200b\u180e'`},
	// Join tests.
	{expr: `['x', 'y'].join() == 'xy'`},
	{expr: `['x', 'y'].join('-') == 'x-y'`},
	{expr: `[].join() == ''`},
	{expr: `[].join('-') == ''`},
	// Escaping tests.
	{expr: `strings.quote("first\nsecond") == "\"first\\nsecond\""`},
	{expr: `strings.quote("bell\a") == "\"bell\\a\""`},
	{expr: `strings.quote("\bbackspace") == "\"\\bbackspace\""`},
	{expr: `strings.quote("\fform feed") == "\"\\fform feed\""`},
	{expr: `strings.quote("carriage \r return") == "\"carriage \\r return\""`},
	{expr: `strings.quote("horizontal tab\t") == "\"horizontal tab\\t\""`},
	{expr: `strings.quote("vertical \v tab") == "\"vertical \\v tab\""`},
	{expr: `strings.quote("double \\\\ slash") == "\"double \\\\\\\\ slash\""`},
	{expr: `strings.quote("two escape sequences \a\n") == "\"two escape sequences \\a\\n\""`},
	{expr: `strings.quote("verbatim") == "\"verbatim\""`},
	{expr: `strings.quote("ends with \\") == "\"ends with \\\\\""`},
	{expr: `strings.quote("\\ starts with") == "\"\\\\ starts with\""`},
	{expr: `strings.quote("printable unicode😀") == "\"printable unicode😀\""`},
	{expr: `strings.quote("mid string \" quote") == "\"mid string \\\" quote\""`},
	{expr: `strings.quote('single-quote with "double quote"') == "\"single-quote with \\\"double quote\\\"\""`},
	{expr: `strings.quote("size('ÿ')") == "\"size('ÿ')\""`},
	{expr: `strings.quote("size('πέντε')") == "\"size('πέντε')\""`},
	{expr: `strings.quote("завтра") == "\"завтра\""`},
	{expr: `strings.quote("\U0001F431\U0001F600\U0001F61B") == "\"\U0001F431\U0001F600\U0001F61B\""`},
	{expr: `strings.quote("ta©o©αT") == "\"ta©o©αT\""`},
	{expr: `strings.quote("") == "\"\""`},
	// Format tests with a non-literal as the format string
	{
		expr: `strings.quote('%s %s').format(['hello', 'world']) == "\"hello world\""`,
	},
	// Error test cases based on checked expression usage.
	{
		expr: `'tacocat'.charAt(30) == ''`,
		err:  "index out of range: 30",
	},
	{expr: `'tacocat'.indexOf('a', 30) == -1`},
	{expr: `'tacocat'.indexOf('', 7) == 7`},
	{expr: `'tacocat'.indexOf('', 30) == 7`},
	{
		expr: `'tacocat'.indexOf('', -1) == 0`,
		err:  "index out of range: -1",
	},
	{
		expr: `'tacocat'.lastIndexOf('a', -1) == -1`,
		err:  "index out of range: -1",
	},
	{expr: `'tacocat'.lastIndexOf('a', 30) == -1`},
	{expr: `'tacocat'.lastIndexOf('', 7) == 7`},
	{expr: `'tacocat'.lastIndexOf('', 30) == 7`},
	{
		expr: `'tacocat'.lastIndexOf('', -1) == 0`,
		err:  "index out of range: -1",
	},
	{
		expr: `"tacocat".substring(40) == "cat"`,
		err:  "index out of range: 40",
	},
	{
		expr: `"tacocat".substring(-1) == "cat"`,
		err:  "index out of range: -1",
	},
	{
		expr: `"tacocat".substring(1, 50) == "cat"`,
		err:  "index out of range: 50",
	},
	{
		expr: `"tacocat".substring(49, 50) == "cat"`,
		err:  "index out of range: 49",
	},
	{
		expr: `"tacocat".substring(4, 3) == ""`,
		err:  "invalid substring range. start: 4, end: 3",
	},
	// Valid parse-only expressions which should generate runtime errors.
	{
		expr:      `42.charAt(2) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'hello'.charAt(true) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `24.indexOf('2') == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'hello'.indexOf(true) == 1`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.indexOf('4', 0) == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'42'.indexOf(4, 0) == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'42'.indexOf('4', '0') == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'42'.indexOf('4', 0, 1) == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.split("2") == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.replace(2, 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace(2, 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.replace("2", "1", 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace(2, "1", 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", 1, 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", "1", "1") == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", "1", 1, false) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.split("") == ["4", "2"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split(2) == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.split("2", "1") == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split(2, 1) == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split("2", "1") == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split("2", 1, 1) == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'hello'.substring(1, 2, 3) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `30.substring(true, 3) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"tacocat".substring(true, 3) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"tacocat".substring(0, false) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
}

func TestStrings(t *testing.T) {
	env, err := cel.NewEnv(Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Strings()) failed: %v", err)
	}
	for i, tst := range stringTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			if !tc.parseOnly {
				cAst, iss := env.Check(pAst)
				if iss.Err() != nil {
					t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
				}
				asts = append(asts, cAst)
			}
			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatal(err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if tc.err != "" {
					if err == nil {
						t.Fatalf("got value %v, wanted error %s for expr: %s",
							out.Value(), tc.err, tc.expr)
					}
					if !strings.Contains(err.Error(), tc.err) {
						t.Errorf("got %q, expected error to contain %q for expr: %s", err, tc.err, tc.expr)
					}
				} else if err != nil {
					t.Fatal(err)
				} else if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestStringsVersions(t *testing.T) {
	versionCases := []struct {
		version            uint32
		supportedFunctions map[string]string
	}{
		{
			version: 0,
			supportedFunctions: map[string]string{
				"chatAt":      "''.charAt(0) == ''",
				"indexOf":     "'a'.indexOf('a') == 0",
				"lastIndexOf": "'a'.lastIndexOf('a') == 0",
				"join":        "['a', 'b'].join() == 'ab'",
				"joinSep":     "['a', 'b'].join('-') == 'a-b'",
				"lowerAscii":  "'a'.lowerAscii() == 'a'",
				"replace":     "'hello hello'.replace('he', 'we') == 'wello wello'",
				"split":       "'hello hello hello'.split(' ') == ['hello', 'hello', 'hello']",
				"substring":   "'tacocat'.substring(4) == 'cat'",
				"trim":        "'  \\ttrim\\n    '.trim() == 'trim'",
				"upperAscii":  "'TacoCat'.upperAscii() == 'TACOCAT'",
			},
		},
		{
			version: 1,
			supportedFunctions: map[string]string{
				"format": "'a %d'.format([1]) == 'a 1'",
				"quote":  `strings.quote('\a \b "double quotes"') == '"\\a \\b \\"double quotes\\""'`,
			},
		},
		{
			version: 3,
			supportedFunctions: map[string]string{
				"reverse": "'taco'.reverse() == 'ocat'",
			},
		},
	}
	for _, lib := range versionCases {
		env, err := cel.NewEnv(Strings(StringsVersion(lib.version)))
		if err != nil {
			t.Fatalf("cel.NewEnv(Strings(StringsVersion(%d))) failed: %v", lib.version, err)
		}
		t.Run(fmt.Sprintf("version=%d", lib.version), func(t *testing.T) {
			for _, tc := range versionCases {
				for name, expr := range tc.supportedFunctions {
					supported := lib.version >= tc.version
					t.Run(fmt.Sprintf("%s-supported=%t", name, supported), func(t *testing.T) {
						ast, iss := env.Compile(expr)
						if supported {
							if iss.Err() != nil {
								t.Errorf("unexpected error: %v", iss.Err())
							}
						} else {
							if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "undeclared reference") {
								t.Errorf("got error %v, wanted error %s for expr: %s, version: %d", iss.Err(), "undeclared reference", expr, tc.version)
							}
							return
						}
						prg, err := env.Program(ast)
						if err != nil {
							t.Fatalf("env.Program() failed: %v", err)
						}
						out, _, err := prg.Eval(cel.NoVars())
						if err != nil {
							t.Fatalf("prg.Eval() failed: %v", err)
						}
						if out != types.True {
							t.Errorf("prg.Eval() got %v, wanted true", out)
						}
					})
				}
			}
		})
	}
}

func TestStringsWithExtension(t *testing.T) {
	env, err := cel.NewEnv(Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Strings()) failed: %v", err)
	}
	_, err = env.Extend(Strings())
	if err != nil {
		t.Fatalf("env.Extend(Strings()) failed: %v", err)
	}
}

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

func unquote(s string) (string, error) {
	r := []rune(sanitize(s))
	if r[0] != '"' || r[len(r)-1] != '"' {
		return "", fmt.Errorf("expected given string to be enclosed in double quotes: %q", r)
	}
	var unquotedStrBuilder strings.Builder
	noQuotes := r[1 : len(r)-1]
	for i := 0; i < len(noQuotes); {
		c := noQuotes[i]
		hasNext := i+1 < len(noQuotes)
		if c == '\\' {
			if hasNext {
				nextChar := noQuotes[i+1]
				switch nextChar {
				case 'a':
					unquotedStrBuilder.WriteRune('\a')
				case 'b':
					unquotedStrBuilder.WriteRune('\b')
				case 'f':
					unquotedStrBuilder.WriteRune('\f')
				case 'n':
					unquotedStrBuilder.WriteRune('\n')
				case 'r':
					unquotedStrBuilder.WriteRune('\r')
				case 't':
					unquotedStrBuilder.WriteRune('\t')
				case 'v':
					unquotedStrBuilder.WriteRune('\v')
				case '\\':
					unquotedStrBuilder.WriteRune('\\')
				case '"':
					unquotedStrBuilder.WriteRune('"')
				default:
					unquotedStrBuilder.WriteRune(c)
					unquotedStrBuilder.WriteRune(nextChar)
				}
				i += 2
				continue
			}
		}
		unquotedStrBuilder.WriteRune(c)
		i++
	}
	return unquotedStrBuilder.String(), nil
}

func TestQuoteUnquote(t *testing.T) {
	tests := []struct {
		name                  string
		testStr               string
		expectedErr           string
		expectedOutput        string
		expectedRuntimeCost   uint64
		expectedEstimatedCost checker.CostEstimate
		disableQuote          bool
		disableCELEval        bool
	}{
		{
			name:                  "remove quotes only",
			testStr:               "this is a test",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "mid-string newline",
			testStr:               "first\nsecond",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "bell",
			testStr:               "bell\a",
			expectedEstimatedCost: checker.FixedCostEstimate(1),
			expectedRuntimeCost:   1,
		},
		{
			name:                  "backspace",
			testStr:               "\bbackspace",
			expectedEstimatedCost: checker.FixedCostEstimate(1),
			expectedRuntimeCost:   1,
		},
		{
			name:                  "form feed",
			testStr:               "\fform feed",
			expectedEstimatedCost: checker.FixedCostEstimate(1),
			expectedRuntimeCost:   1,
		},
		{
			name:                  "carriage return",
			testStr:               "carriage \r return",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "horizontal tab",
			testStr:               "horizontal \ttab",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "vertical tab",
			testStr:               "vertical \v tab",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "double slash",
			testStr:               "double \\\\ slash",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "two escape sequences",
			testStr:               "two escape sequences \a\n",
			expectedEstimatedCost: checker.FixedCostEstimate(3),
			expectedRuntimeCost:   3,
		},
		{
			name:                  "ends with slash",
			testStr:               "ends with \\",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "starts with slash",
			testStr:               "\\ starts with",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "printable unicode",
			testStr:               "printable unicode😀",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "mid-string quote",
			testStr:               "mid-string \" quote",
			expectedEstimatedCost: checker.FixedCostEstimate(2),
			expectedRuntimeCost:   2,
		},
		{
			name:                  "single-quote with double quote",
			testStr:               `single-quote with "double quote"`,
			expectedEstimatedCost: checker.FixedCostEstimate(4),
			expectedRuntimeCost:   4,
		},
		{
			name:                  "CEL-only escape sequences",
			testStr:               "\\? and \\`",
			expectedEstimatedCost: checker.FixedCostEstimate(1),
			expectedRuntimeCost:   1,
		},
		{
			name:                  "test cost",
			testStr:               "this is a very very very long string used to ensure that cost tracking works",
			expectedEstimatedCost: checker.FixedCostEstimate(8),
			expectedRuntimeCost:   8,
		},
		{
			name:         "missing opening quote",
			testStr:      `only one quote"`,
			expectedErr:  "expected given string to be enclosed in double quotes",
			disableQuote: true,
		},
		{
			name:         "missing closing quote",
			testStr:      `"only one quote`,
			expectedErr:  "expected given string to be enclosed in double quotes",
			disableQuote: true,
		},
		{
			name:           "invalid utf8",
			testStr:        "filler \x9f",
			expectedOutput: "filler " + string(utf8.RuneError),
			// disable CEL eval in order to simulate a string variable with invalid UTF-8
			disableCELEval: true,
		},
		{
			name:           "trailing single slash",
			testStr:        "\"trailing slash \\\"",
			expectedOutput: "trailing slash \\",
			disableQuote:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s string
			if tt.disableQuote {
				s = tt.testStr
			} else {
				if tt.disableCELEval {
					s, _ = quote(tt.testStr)
				} else {
					s = evalWithCEL(tt.testStr, tt.expectedRuntimeCost, tt.expectedEstimatedCost, t)
				}
			}
			output, err := unquote(s)
			if err != nil {
				if tt.expectedErr != "" {
					if !strings.Contains(err.Error(), tt.expectedErr) {
						t.Fatalf("expected error message %q to contain %q", err, tt.expectedErr)
					}
				} else {
					t.Fatalf("unexpected error: %s", err)
				}
			} else {
				if tt.expectedErr != "" {
					t.Fatalf("expected error message with substring %q but no error was seen", tt.expectedErr)
				}
				if tt.expectedOutput != "" {
					if output != tt.expectedOutput {
						t.Fatalf("expected output: %q, got: %q", tt.expectedOutput, output)
					}
				} else if output != tt.testStr {
					t.Fatalf("input-output mismatch: original: %q, quote/unquote: %q", tt.testStr, output)
				}
			}
		})
	}
}

type noopCostEstimator struct{}

func (e *noopCostEstimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	return nil
}

func (e *noopCostEstimator) EstimateCallCost(function, overloadID string, target *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	return nil
}

func (e *noopCostEstimator) EstimateSize(element checker.AstNode) *checker.SizeEstimate {
	return nil
}

func evalWithCEL(input string, expectedRuntimeCost uint64, expectedEstimatedCost checker.CostEstimate, t *testing.T) string {
	env, err := cel.NewEnv(Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	expr := fmt.Sprintf(`strings.quote(%q)`, input)
	parsedAst, issues := env.Parse(expr)
	if issues.Err() != nil {
		t.Fatalf("env.Parse() failed: %v", issues.Err())
	}
	checkedAst, issues := env.Check(parsedAst)
	if issues.Err() != nil {
		t.Fatalf("env.Check() failed: %v", issues.Err())
	}

	costTracker := &noopCostEstimator{}
	actualEstimatedCost, err := env.EstimateCost(checkedAst, costTracker)
	if err != nil {
		t.Fatal(err)
	}
	if expectedEstimatedCost.Min != 0 && expectedEstimatedCost.Max != 0 {
		if actualEstimatedCost.Min != expectedEstimatedCost.Min && actualEstimatedCost.Max != expectedEstimatedCost.Max {
			t.Fatalf("expected estimated cost range to be %v, was %v", expectedEstimatedCost, actualEstimatedCost)
		}
	}

	program, err := env.Program(checkedAst, cel.CostTracking(costTracker))
	if err != nil {
		t.Fatal(err)
	}
	out, evalDetails, err := program.Eval(cel.NoVars())
	if err != nil {
		t.Fatal(err)
	}
	if evalDetails == nil {
		t.Fatal("evalDetails could not be calculated")
	} else if evalDetails.ActualCost() == nil {
		t.Fatal("could not calculate runtime cost")
	}
	if expectedRuntimeCost != 0 {
		if *evalDetails.ActualCost() != expectedRuntimeCost {
			t.Fatalf("expected runtime cost of %d, got %d", expectedRuntimeCost, *evalDetails.ActualCost())
		}
		if expectedEstimatedCost.Min != 0 && expectedEstimatedCost.Max != 0 {
			if *evalDetails.ActualCost() < expectedEstimatedCost.Min || *evalDetails.ActualCost() > expectedEstimatedCost.Max {
				t.Fatalf("runtime cost %d outside of expected estimated cost range %v", *evalDetails.ActualCost(), expectedEstimatedCost)
			}
		}
	}
	if out.Type() != types.StringType {
		t.Fatalf("expected expr output to be a string, got %s", out.Type().TypeName())
	}
	return out.Value().(string)
}

func TestFunctionsForVersions(t *testing.T) {
	tests := []struct {
		version             uint32
		introducedFunctions []string
	}{
		{
			version:             0,
			introducedFunctions: []string{"lastIndexOf", "lowerAscii", "split", "trim", "join", "charAt", "indexOf", "replace", "substring", "upperAscii"},
		},
		{
			version:             1,
			introducedFunctions: []string{"strings.quote", "format"},
		},
		{
			version:             2,
			introducedFunctions: []string{}, // join changed, no functions added
		},
		{
			version:             3,
			introducedFunctions: []string{"reverse"},
		},
	}
	var functions []string
	for _, tt := range tests {
		functions = append(functions, tt.introducedFunctions...)
		t.Run(fmt.Sprintf("version %d", tt.version), func(t *testing.T) {
			e, err := cel.NewCustomEnv(Strings(StringsVersion(tt.version)))
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			if len(functions) != len(e.Functions()) {
				var functionNames []string
				for name := range e.Functions() {
					functionNames = append(functionNames, name)
				}
				t.Fatalf("Expected functions: %#v, got %#v", functions, functionNames)
			}
			for _, expected := range functions {
				if !e.HasFunction(expected) {
					t.Errorf("Expected HasFunction() to return true for '%s'", expected)
				}

				if _, ok := e.Functions()[expected]; !ok {
					t.Errorf("Expected Functions() to include '%s'", expected)
				}
			}
		})
	}
}

func FuzzQuote(f *testing.F) {
	tests := []string{
		"this is a test",
		`only one quote"`,
		`"only one quote`,
		"first\nsecond",
		"bell\a",
		"\bbackspace",
		"\fform feed",
		"carriage \r return",
		"horizontal \ttab",
		"vertical \v tab",
		"double \\\\ slash",
		"two escape sequences \a\n",
		"ends with \\",
		"\\ starts with",
		"printable unicode😀",
		"mid-string \" quote",
		"\\? and \\`",
		"filler \x9f",
		"size('ÿ')",
		"size('πέντε')",
		"завтра",
		"\U0001F431\U0001F600\U0001F61B",
		"ta©o©αT",
	}
	for _, tc := range tests {
		f.Add(tc)
	}
	f.Fuzz(func(t *testing.T, s string) {
		quoted, err := quote(s)
		if err != nil {
			if utf8.ValidString(s) {
				t.Errorf("unexpected error: %s", err)
			}
		} else {
			unquoted, err := unquote(quoted)
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			} else if s != sanitize(s) {
				if unquoted != sanitize(s) {
					t.Errorf("input-output mismatch on test case containing invalid UTF-8: sanitized original: %q, quoted: %q, quote/unquote: %q", sanitize(s), quoted, unquoted)
				}
			} else if unquoted != s {
				t.Errorf("input-output mismatch: original: %q, quoted: %q, quote/unquote: %q", s, quoted, unquoted)
			}
		}
	})
}

func TestStringCostTracking(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		estimatedCost checker.CostEstimate
		actualCost    uint64
	}{
		{
			name:          "charAt",
			expr:          `"hello world".charAt(0)`,
			estimatedCost: checker.FixedCostEstimate(4),
			actualCost:    4,
		},
		{
			name:          "indexOf",
			expr:          `"hello world".indexOf("world")`,
			estimatedCost: checker.FixedCostEstimate(7),
			actualCost:    7,
		},
		{
			name:          "lastIndexOf",
			expr:          `"hello world".lastIndexOf("o")`,
			estimatedCost: checker.FixedCostEstimate(3),
			actualCost:    3,
		},
		{
			name:          "lowerAscii",
			expr:          `"HELLO".lowerAscii()`,
			estimatedCost: checker.FixedCostEstimate(7),
			actualCost:    7,
		},
		{
			name:          "upperAscii",
			expr:          `"hello".upperAscii()`,
			estimatedCost: checker.FixedCostEstimate(7),
			actualCost:    7,
		},
		{
			name:          "replace",
			expr:          `"hello world".replace("world", "CEL")`,
			estimatedCost: checker.CostEstimate{Min: 11, Max: 55},
			actualCost:    16,
		},
		{
			name:          "replace_exponential_growth",
			expr:          `"A".replace("", "AAAAAAAAAA").replace("", "AAAAAAAAAA")`,
			estimatedCost: checker.CostEstimate{Min: 6, Max: 281},
			actualCost:    268,
		},
		{
			name:          "split",
			expr:          `"a,b,c,d,e".split(",")`,
			estimatedCost: checker.CostEstimate{Min: 12, Max: 21},
			actualCost:    17,
		},
		{
			name:          "substring",
			expr:          `"hello world".substring(0, 5)`,
			estimatedCost: checker.FixedCostEstimate(8),
			actualCost:    8,
		},
		{
			name:          "join",
			expr:          `["a", "b", "c", "d", "e"].join("-")`,
			estimatedCost: checker.CostEstimate{Min: 12, Max: 23},
			actualCost:    21,
		},
		{
			name:          "trim",
			expr:          `"  hello  ".trim()`,
			estimatedCost: checker.CostEstimate{Min: 2, Max: 11},
			actualCost:    7,
		},
		{
			name:          "reverse",
			expr:          `"hello".reverse()`,
			estimatedCost: checker.FixedCostEstimate(7),
			actualCost:    7,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env, err := cel.NewEnv(Strings(StringsVersion(5)))
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			testCheckCost(t, env, ast, nil, tc.estimatedCost)
			prg, err := env.Program(ast, cel.CostTracking(nil))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			_, det, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			cost := det.ActualCost()
			if cost == nil {
				t.Fatal("cost tracking returned nil")
			}
			if *cost != tc.actualCost {
				t.Errorf("cost for %q = %d, want at least %d", tc.expr, *cost, tc.actualCost)
			}
		})
	}
}

func TestStringCostLimitEnforced(t *testing.T) {
	env, err := cel.NewEnv(Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	// Chained replaces that produce exponential output.
	// Without cost tracking, this would be charged ~6. With tracking, the cost
	// scales with output size and should exceed any reasonable limit.
	expr := `"A".replace("", "AAAAAAAAAA").replace("", "AAAAAAAAAA").replace("", "AAAAAAAAAA").replace("", "AAAAAAAAAA").replace("", "AAAAAAAAAA").replace("", "AAAAAAAAAA")`
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, cel.CostLimit(1000), cel.CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, _, err = prg.Eval(cel.NoVars())
	if err == nil {
		t.Error("expected cost limit exceeded error for exponential string growth, got nil")
	}
}
