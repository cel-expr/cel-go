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

package parser

import (
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/runes"
)

type expectedToken struct {
	kind tokenKind
	text string
}

func TestLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []expectedToken
	}{
		{
			name:     "Empty",
			input:    "",
			expected: []expectedToken{},
		},
		{
			name:  "Whitespace",
			input: " \n  \t\r\f\v",
			expected: []expectedToken{
				{kind: tokWhitespace, text: " \n  \t\r\f\v"},
			},
		},
		{
			name:  "KeywordsAndIdents",
			input: "null false true in as return foo_bar _foo_bar _ `quoted.ident`",
			expected: []expectedToken{
				{kind: tokNull, text: "null"},
				{kind: tokWhitespace, text: " "},
				{kind: tokFalse, text: "false"},
				{kind: tokWhitespace, text: " "},
				{kind: tokTrue, text: "true"},
				{kind: tokWhitespace, text: " "},
				{kind: tokIn, text: "in"},
				{kind: tokWhitespace, text: " "},
				{kind: tokReservedWord, text: "as"},
				{kind: tokWhitespace, text: " "},
				{kind: tokReservedWord, text: "return"},
				{kind: tokWhitespace, text: " "},
				{kind: tokIdent, text: "foo_bar"},
				{kind: tokWhitespace, text: " "},
				{kind: tokIdent, text: "_foo_bar"},
				{kind: tokWhitespace, text: " "},
				{kind: tokIdent, text: "_"},
				{kind: tokWhitespace, text: " "},
				{kind: tokIdent, text: "`quoted.ident`"},
			},
		},
		{
			name:  "Numbers",
			input: "123 45u 0x1A 3.14 .5 1e6 2.5e-3 45U 0x1Au 0x1AU",
			expected: []expectedToken{
				{kind: tokInt, text: "123"},
				{kind: tokWhitespace, text: " "},
				{kind: tokUint, text: "45u"},
				{kind: tokWhitespace, text: " "},
				{kind: tokInt, text: "0x1A"},
				{kind: tokWhitespace, text: " "},
				{kind: tokFloat, text: "3.14"},
				{kind: tokWhitespace, text: " "},
				{kind: tokFloat, text: ".5"},
				{kind: tokWhitespace, text: " "},
				{kind: tokFloat, text: "1e6"},
				{kind: tokWhitespace, text: " "},
				{kind: tokFloat, text: "2.5e-3"},
				{kind: tokWhitespace, text: " "},
				{kind: tokUint, text: "45U"},
				{kind: tokWhitespace, text: " "},
				{kind: tokUint, text: "0x1Au"},
				{kind: tokWhitespace, text: " "},
				{kind: tokUint, text: "0x1AU"},
			},
		},
		{
			name:  "IntEOF",
			input: "123456",
			expected: []expectedToken{
				{kind: tokInt, text: "123456"},
			},
		},
		{
			name:  "HexIntEOF",
			input: "0x1A2B",
			expected: []expectedToken{
				{kind: tokInt, text: "0x1A2B"},
			},
		},
		{
			name:  "FloatPositiveExponentEOF",
			input: "1e+6",
			expected: []expectedToken{
				{kind: tokFloat, text: "1e+6"},
			},
		},
		{
			name:  "FloatEOF",
			input: ".12345",
			expected: []expectedToken{
				{kind: tokFloat, text: ".12345"},
			},
		},
		{
			name:  "IntDotIdent",
			input: "1.foo",
			expected: []expectedToken{
				{kind: tokInt, text: "1"},
				{kind: tokDot, text: "."},
				{kind: tokIdent, text: "foo"},
			},
		},
		{
			name:  "IntDotWhitespace",
			input: "1. ",
			expected: []expectedToken{
				{kind: tokInt, text: "1"},
				{kind: tokDot, text: "."},
				{kind: tokWhitespace, text: " "},
			},
		},
		{
			name:  "IntDotEOF",
			input: "1.",
			expected: []expectedToken{
				{kind: tokInt, text: "1"},
				{kind: tokDot, text: "."},
			},
		},
		{
			name:  "ZeroNumbers",
			input: "0 0u 0x0",
			expected: []expectedToken{
				{kind: tokInt, text: "0"},
				{kind: tokWhitespace, text: " "},
				{kind: tokUint, text: "0u"},
				{kind: tokWhitespace, text: " "},
				{kind: tokInt, text: "0x0"},
			},
		},
		{
			name:  "StringsAndBytes",
			input: "\"hello\" 'world' \"\"\" \"allowed!\" \"\"also allowed\"\" \\\"\"\"also allowed\"\"\\\" \"\"\" r\"raw\" b\"bytes\" rb'\\x00' '''multi\nsingle''' R\"raw_upper\" B\"bytes_upper\" b'''multi\nbytes''' br\"raw_bytes\" `a.b-c/d e`\n\"\\a\\b\\f\\n\\r\\t\\v\\\"\\'\\\\\\?\\` \\x1A \\u00A0 \\U0001F600 \\012\"",
			expected: []expectedToken{
				{kind: tokString, text: "\"hello\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "'world'"},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "\"\"\" \"allowed!\" \"\"also allowed\"\" \\\"\"\"also allowed\"\"\\\" \"\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "r\"raw\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "b\"bytes\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "rb'\\x00'"},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "'''multi\nsingle'''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "R\"raw_upper\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "B\"bytes_upper\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "b'''multi\nbytes'''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "br\"raw_bytes\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokIdent, text: "`a.b-c/d e`"},
				{kind: tokWhitespace, text: "\n"},
				{kind: tokString, text: "\"\\a\\b\\f\\n\\r\\t\\v\\\"\\'\\\\\\?\\` \\x1A \\u00A0 \\U0001F600 \\012\""},
			},
		},
		{
			name:  "EmptyStrings",
			input: "\"\" '' \"\"\"\"\"\" '''''' r\"\" r'' r\"\"\"\"\"\" r'''''' b\"\" b'' b\"\"\"\"\"\" b''''''",
			expected: []expectedToken{
				{kind: tokString, text: "\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "\"\"\"\"\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "''''''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "r\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "r''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "r\"\"\"\"\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokString, text: "r''''''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "b\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "b''"},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "b\"\"\"\"\"\""},
				{kind: tokWhitespace, text: " "},
				{kind: tokBytes, text: "b''''''"},
			},
		},
		{
			name:  "OperatorsAndDelimiters",
			input: ". , + - * / % == != < <= > >= && || ! ? : [] { } ( )",
			expected: []expectedToken{
				{kind: tokDot, text: "."},
				{kind: tokWhitespace, text: " "},
				{kind: tokComma, text: ","},
				{kind: tokWhitespace, text: " "},
				{kind: tokPlus, text: "+"},
				{kind: tokWhitespace, text: " "},
				{kind: tokMinus, text: "-"},
				{kind: tokWhitespace, text: " "},
				{kind: tokAsterisk, text: "*"},
				{kind: tokWhitespace, text: " "},
				{kind: tokSlash, text: "/"},
				{kind: tokWhitespace, text: " "},
				{kind: tokPercent, text: "%"},
				{kind: tokWhitespace, text: " "},
				{kind: tokEqualEqual, text: "=="},
				{kind: tokWhitespace, text: " "},
				{kind: tokExclamationEqual, text: "!="},
				{kind: tokWhitespace, text: " "},
				{kind: tokLess, text: "<"},
				{kind: tokWhitespace, text: " "},
				{kind: tokLessEqual, text: "<="},
				{kind: tokWhitespace, text: " "},
				{kind: tokGreater, text: ">"},
				{kind: tokWhitespace, text: " "},
				{kind: tokGreaterEqual, text: ">="},
				{kind: tokWhitespace, text: " "},
				{kind: tokLogicalAnd, text: "&&"},
				{kind: tokWhitespace, text: " "},
				{kind: tokLogicalOr, text: "||"},
				{kind: tokWhitespace, text: " "},
				{kind: tokExclamation, text: "!"},
				{kind: tokWhitespace, text: " "},
				{kind: tokQuestion, text: "?"},
				{kind: tokWhitespace, text: " "},
				{kind: tokColon, text: ":"},
				{kind: tokWhitespace, text: " "},
				{kind: tokLeftBracket, text: "["},
				{kind: tokRightBracket, text: "]"},
				{kind: tokWhitespace, text: " "},
				{kind: tokLeftBrace, text: "{"},
				{kind: tokWhitespace, text: " "},
				{kind: tokRightBrace, text: "}"},
				{kind: tokWhitespace, text: " "},
				{kind: tokLeftParen, text: "("},
				{kind: tokWhitespace, text: " "},
				{kind: tokRightParen, text: ")"},
			},
		},
		{
			name:  "Comments",
			input: "a\n// comment\nb",
			expected: []expectedToken{
				{kind: tokIdent, text: "a"},
				{kind: tokWhitespace, text: "\n"},
				{kind: tokComment, text: "// comment\n"},
				{kind: tokIdent, text: "b"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := runes.NewBuffer(tc.input)
			lexer := newLexer(buf)
			var tokens []expectedToken
			for {
				tok := lexer.Lex()
				if tok.kind == tokEnd {
					break
				}
				if tok.kind == tokError {
					t.Fatalf("unexpected error token: %v (%s)", tok, lexer.GetError().message)
				}
				text := buf.Slice(int(tok.start), int(tok.end))
				tokens = append(tokens, expectedToken{kind: tok.kind, text: text})
			}

			if len(tokens) != len(tc.expected) {
				t.Fatalf("got %d tokens, expected %d\ngot: %+v\nwant: %+v", len(tokens), len(tc.expected), tokens, tc.expected)
			}
			for i, exp := range tc.expected {
				got := tokens[i]
				if got.kind != exp.kind || got.text != exp.text {
					t.Errorf("token[%d] = {kind: %v, text: %q}, want {kind: %v, text: %q}", i, got.kind, got.text, exp.kind, exp.text)
				}
			}
		})
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:  "UnterminatedString",
			input: "\"unterminated",
			expectedError: "ERROR: <input>:1:1: unterminated string literal\n" +
				" | \"unterminated\n" +
				" | ^",
		},
		{
			name:  "HexMissingDigits",
			input: "0x",
			expectedError: "ERROR: <input>:1:1: integral literal missing digits after hexadecimal separator\n" +
				" | 0x\n" +
				" | ^",
		},
		{
			name:  "UnexpectedChar",
			input: "@",
			expectedError: "ERROR: <input>:1:1: unexpected character\n" +
				" | @\n" +
				" | ^",
		},
		{
			name:  "HexInvalidTrailing",
			input: "0x1A_invalid",
			expectedError: "ERROR: <input>:1:1: int literal has unexpected trailing characters\n" +
				" | 0x1A_invalid\n" +
				" | ^",
		},
		{
			name:  "IntInvalidTrailing",
			input: "123_invalid",
			expectedError: "ERROR: <input>:1:1: int literal has unexpected trailing characters\n" +
				" | 123_invalid\n" +
				" | ^",
		},
		{
			name:  "Int1x0",
			input: "1x0",
			expectedError: "ERROR: <input>:1:1: int literal has unexpected trailing characters\n" +
				" | 1x0\n" +
				" | ^",
		},
		{
			name:  "Int2x",
			input: "2x",
			expectedError: "ERROR: <input>:1:1: int literal has unexpected trailing characters\n" +
				" | 2x\n" +
				" | ^",
		},
		{
			name:  "UnterminatedQuotedIdent",
			input: "`unterminated quoted",
			expectedError: "ERROR: <input>:1:1: unterminated quoted identifier\n" +
				" | `unterminated quoted\n" +
				" | ^",
		},
		{
			name:  "UnterminatedMultiString",
			input: "'''unterminated multi",
			expectedError: "ERROR: <input>:1:1: unterminated string literal\n" +
				" | '''unterminated multi\n" +
				" | ^",
		},
		{
			name:  "UnterminatedRawString",
			input: "r'unterminated raw",
			expectedError: "ERROR: <input>:1:1: unterminated string literal\n" +
				" | r'unterminated raw\n" +
				" | ^",
		},
		{
			name:  "UnterminatedBytes",
			input: "b'unterminated bytes",
			expectedError: "ERROR: <input>:1:1: unterminated bytes literal\n" +
				" | b'unterminated bytes\n" +
				" | ^",
		},
		{
			name:  "ExponentMissingDigits",
			input: "1e",
			expectedError: "ERROR: <input>:1:1: floating point literal missing digits after exponent separator\n" +
				" | 1e\n" +
				" | ^",
		},
		{
			name:  "SingleAmpersand",
			input: "&",
			expectedError: "ERROR: <input>:1:1: unexpected single '&', expected '&&'\n" +
				" | &\n" +
				" | ^",
		},
		{
			name:  "SinglePipe",
			input: "|",
			expectedError: "ERROR: <input>:1:1: unexpected single '|', expected '||'\n" +
				" | |\n" +
				" | ^",
		},
		{
			name:  "EmojiUnexpectedChar",
			input: "\"😀😀😀😀😀\" ~error",
			expectedError: "ERROR: <input>:1:9: unexpected character\n" +
				" | \"😀😀😀😀😀\" ~error\n" +
				" | .．．．．．..^",
		},
		{
			name:  "MultiLineEmojiUnexpectedChar",
			input: "\"😀😀\"\n  ~error",
			expectedError: "ERROR: <input>:2:3: unexpected character\n" +
				" |   ~error\n" +
				" | ..^",
		},
		{
			name:  "UnicodeCJKFollowingExponentError",
			input: "\"𠮷野家\" 1e",
			expectedError: "ERROR: <input>:1:7: floating point literal missing digits after exponent separator\n" +
				" | \"𠮷野家\" 1e\n" +
				" | .．．．..^",
		},
		{
			name:  "SupplementaryPlaneQuotedIdentFollowingError",
			input: "`𠮷_ident_🚀` @",
			expectedError: "ERROR: <input>:1:13: unexpected character\n" +
				" | `𠮷_ident_🚀` @\n" +
				" | .．.......．..^",
		},
		{
			name:  "EmojiInStringErrorRecovery",
			input: "\"✨🌟⭐\" @ 42",
			expectedError: "ERROR: <input>:1:7: unexpected character\n" +
				" | \"✨🌟⭐\" @ 42\n" +
				" | .．．．..^",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := runes.NewBuffer(tc.input)
			lexer := newLexer(buf)
			var tok token
			for {
				tok = lexer.Lex()
				if tok.kind == tokError || tok.kind == tokEnd {
					break
				}
			}
			if tok.kind != tokError {
				t.Fatalf("expected tokError for input %q, got %v", tc.input, tok.kind)
			}

			// Format error message using standard common.Error and common.Source
			textSource := common.NewTextSource(tc.input)
			loc, ok := textSource.OffsetLocation(tok.start)
			if !ok {
				t.Fatalf("textSource.OffsetLocation(%d) not found", tok.start)
			}
			err := common.NewError(0, lexer.GetError().message, loc)
			gotDisplay := err.ToDisplayString(textSource)

			if gotDisplay != tc.expectedError {
				t.Errorf("got error display:\n%s\n\nwant error display:\n%s", gotDisplay, tc.expectedError)
			}
		})
	}
}

func TestLexerPositionSaveRestore(t *testing.T) {
	buf := runes.NewBuffer("foo + bar * 42")
	lexer := newLexer(buf)

	tok1 := lexer.Lex()
	if tok1.kind != tokIdent {
		t.Fatalf("tok1 = %v, want tokIdent", tok1.kind)
	}

	tok2 := lexer.Lex()
	if tok2.kind != tokWhitespace {
		t.Fatalf("tok2 = %v, want tokWhitespace", tok2.kind)
	}

	// Save position before '+'
	saved := lexer.SavePosition()

	tok3 := lexer.Lex()
	if tok3.kind != tokPlus {
		t.Fatalf("tok3 = %v, want tokPlus", tok3.kind)
	}

	tok4 := lexer.Lex()
	if tok4.kind != tokWhitespace {
		t.Fatalf("tok4 = %v, want tokWhitespace", tok4.kind)
	}

	tok5 := lexer.Lex()
	if tok5.kind != tokIdent {
		t.Fatalf("tok5 = %v, want tokIdent", tok5.kind)
	}

	// Restore position to before '+'
	lexer.RestorePosition(saved)

	tok3Restored := lexer.Lex()
	if tok3Restored.kind != tokPlus || tok3Restored.start != tok3.start || tok3Restored.end != tok3.end {
		t.Errorf("tok3Restored = %v (%d, %d), want tokPlus (%d, %d)", tok3Restored.kind, tok3Restored.start, tok3Restored.end, tok3.start, tok3.end)
	}

	tok4Restored := lexer.Lex()
	if tok4Restored.kind != tokWhitespace {
		t.Errorf("tok4Restored = %v, want tokWhitespace", tok4Restored.kind)
	}

	tok5Restored := lexer.Lex()
	if tok5Restored.kind != tokIdent {
		t.Errorf("tok5Restored = %v, want tokIdent", tok5Restored.kind)
	}
}

func TestLexerErrorRecovery(t *testing.T) {
	buf := runes.NewBuffer("1e, {2 3}")
	lexer := newLexer(buf)

	tok := lexer.Lex()
	if tok.kind != tokError {
		t.Fatalf("tok = %v, want tokError", tok.kind)
	}
	if lexer.GetError().message != "floating point literal missing digits after exponent separator" {
		t.Errorf("got error message %q", lexer.GetError().message)
	}

	tok = lexer.Lex()
	if tok.kind != tokComma {
		t.Errorf("tok = %v, want tokComma", tok.kind)
	}

	tok = lexer.Lex()
	if tok.kind != tokWhitespace {
		t.Errorf("tok = %v, want tokWhitespace", tok.kind)
	}

	tok = lexer.Lex()
	if tok.kind != tokLeftBrace {
		t.Errorf("tok = %v, want tokLeftBrace", tok.kind)
	}

	tok = lexer.Lex()
	if tok.kind != tokInt {
		t.Errorf("tok = %v, want tokInt", tok.kind)
	}
	if tok.start != 5 || tok.end != 6 {
		t.Errorf("tok position = (%d, %d), want (5, 6)", tok.start, tok.end)
	}
}

func BenchmarkLexer(b *testing.B) {
	expr := `a > 5 && b < 10 || c == "xyz" + 42u - 3.14 * (foo.bar(1, 2, [3, ?4]))`
	buf := runes.NewBuffer(expr)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lexer := newLexer(buf)
		for {
			tok := lexer.Lex()
			if tok.kind == tokEnd || tok.kind == tokError {
				break
			}
		}
	}
}
