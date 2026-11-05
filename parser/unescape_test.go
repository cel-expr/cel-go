// Copyright 2018 Google LLC
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
	"errors"
	"strings"
	"testing"
)

func TestUnescape(t *testing.T) {
	tests := []struct {
		in      string
		out     any
		isBytes bool
	}{
		// Simple string unescaping tests.
		{in: `'hello'`, out: `hello`},
		{in: `r'hello'`, out: `hello`},
		{in: `""`, out: ``},
		{in: `"\\\""`, out: `\"`},
		{in: `"\\"`, out: `\`},
		{in: `'''x''x'''`, out: `x''x`},
		{in: `"""x""x"""`, out: `x""x`},
		{in: "'''a\r\nb\rc\nd'''", out: "a\nb\nc\nd"},
		{in: "r'''a\r\nb\rc'''", out: "a\nb\nc"},
		{in: `"hello\nÿ☺"`, out: "hello\nÿ☺"},
		{in: "\"\\n\xff\"", out: "\n\ufffd"},
		{in: `"\303\277"`, out: `Ã¿`},
		{in: `"\377"`, out: `ÿ`},
		{in: `"\u263A\u263A"`, out: `☺☺`},
		{in: `"\a\b\f\n\r\t\v\'\"\\\? Legal escapes"`, out: "\a\b\f\n\r\t\v'\"\\? Legal escapes"},
		// Byte unescaping tests.
		{in: `"abc"`, out: "\x61\x62\x63", isBytes: true},
		{in: `"ÿ"`, out: "\xc3\xbf", isBytes: true},
		{in: `"\303\277"`, out: "\xc3\xbf", isBytes: true},
		{in: `"\377"`, out: "\xff", isBytes: true},
		{in: `"\xff"`, out: "\xff", isBytes: true},
		{in: `"\xc3\xbf"`, out: "\xc3\xbf", isBytes: true},
		{in: `'''"Kim\t"'''`, out: "\x22\x4b\x69\x6d\x09\x22", isBytes: true},
		// Escaping errors.
		{in: `"\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>"`, out: errors.New("unable to unescape string")},
		{in: `"\u00f"`, out: errors.New("unable to unescape string")},
		{in: `"\u00fÿ"`, out: errors.New("unable to unescape string")},
		{in: `"\u00ff"`, out: errors.New("unable to unescape string"), isBytes: true},
		{in: `"\U00ff"`, out: errors.New("unable to unescape string"), isBytes: true},
		{in: `"\26"`, out: errors.New("unable to unescape octal sequence")},
		{in: `"\268"`, out: errors.New("unable to unescape octal sequence")},
		{in: `"\267\"`, out: errors.New(`found '\' as last character`)},
		{in: `'`, out: errors.New("unable to unescape string")},
		{in: `*hello*`, out: errors.New("unable to unescape string")},
		{in: `r'''hello'`, out: errors.New("unable to unescape string")},
		{in: `r"""hello"`, out: errors.New("unable to unescape string")},
		{in: `r"""hello"`, out: errors.New("unable to unescape string")},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.in, func(t *testing.T) {
			got, err := unescape(tc.in, tc.isBytes)
			if err != nil {
				expect, isErr := tc.out.(error)
				if isErr {
					if !strings.Contains(err.Error(), expect.Error()) {
						t.Errorf("unescape(%s, %v) errored with %v, wanted %v", tc.in, tc.isBytes, err, expect)
					}
				} else {
					t.Fatalf("unescape(%s, %v) failed: %v", tc.in, tc.isBytes, err)
				}
			} else if got != tc.out {
				t.Errorf("unescape(%s, %v) got %v, wanted %v", tc.in, tc.isBytes, got, tc.out)
			}
		})
	}
}

func TestUnescapeChar_InvalidLeadingChar(t *testing.T) {
	for _, in := range []string{"", "a", "hello", "123"} {
		_, _, _, err := unescapeChar(in, false)
		if err == nil {
			t.Errorf("unescapeChar(%q, false) expected error, got nil", in)
		}
	}
}

func BenchmarkUnescape(b *testing.B) {
	noEscapes := `"` + strings.Repeat("abcdef01", 512) + `"`
	crlfMultiline := "'''" + strings.Repeat("line\r\n", 1000) + "'''"
	escapedASCII := `"\n` + strings.Repeat("a", 99995) + `"`
	escapedUTF8 := `"\n` + strings.Repeat("\u00e9", 99995) + `"`

	for _, tc := range []struct {
		name string
		in   string
	}{
		{name: "NoEscapes_4KB", in: noEscapes},
		{name: "CRLF_Multiline_6KB", in: crlfMultiline},
		{name: "EscapedASCII_100KB", in: escapedASCII},
		{name: "EscapedUTF8_100KRunes", in: escapedUTF8},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := unescape(tc.in, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
