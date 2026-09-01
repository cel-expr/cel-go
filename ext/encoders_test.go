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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/checker"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	proto2pb "cel.dev/cel-go/test/proto2pb"
	proto3pb "cel.dev/cel-go/test/proto3pb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestEncoders(t *testing.T) {
	var tests = []struct {
		expr      string
		err       string
		parseOnly bool
	}{
		// Base64 Positive & Edge Cases
		{expr: "base64.decode('aGVsbG8=') == b'hello'"},
		{expr: "base64.decode('aGVsbG8') == b'hello'"},
		{expr: "base64.decode('') == b''"},
		{expr: "base64.decode('AQIDBAU=') == b'\\x01\\x02\\x03\\x04\\x05'"},
		{expr: "base64.encode(b'hello') == 'aGVsbG8='"},
		{expr: "base64.encode(b'') == ''"},
		{expr: "base64.encode(b'\\x01\\x02\\x03\\x04\\x05') == 'AQIDBAU='"},
		// Base64 Overload Errors
		{
			expr:      "base64.decode(b'aGVsbG8=') == b'hello'",
			err:       "no such overload",
			parseOnly: true,
		},
		{
			expr:      "base64.encode('hello') == b'aGVsbG8='",
			err:       "no such overload",
			parseOnly: true,
		},
		{expr: "base64.decodeUrl('aGVsbG8=') == b'hello'"},
		{expr: "base64.decodeUrl('aGVsbG8') == b'hello'"},
		{expr: "base64.decodeUrl('____') == b'\\xff\\xff\\xff'"},
		{expr: "base64.decodeUrl('-_==') == b'\\xfb'"},
		{
			expr:      "base64.decodeUrl(b'aGVsbG8=') == b'hello'",
			err:       "no such overload",
			parseOnly: true,
		},
		{expr: "base64.encodeUrl(b'hello') == 'aGVsbG8='"},
		{expr: "base64.encodeUrl(b'\\xff\\xff\\xff') == '____'"},
		{
			expr:      "base64.encodeUrl('hello') == b'aGVsbG8='",
			err:       "no such overload",
			parseOnly: true,
		},
		// Differences between standard Base64 and Base64URL encoding and decoding
		{expr: "base64.encode(b'\\xff\\xff\\xff') == '////'"},
		{expr: "base64.encode(b'\\xff\\xff\\xff') != base64.encodeUrl(b'\\xff\\xff\\xff')"},
		{expr: "base64.encode(b'\\xfb') == '+w=='"},
		{expr: "base64.encodeUrl(b'\\xfb') == '-w=='"},
		{expr: "base64.encode(b'\\xfb') != base64.encodeUrl(b'\\xfb')"},
		{expr: "base64.encode(b'\\xfb\\xef\\xbe') == '++++'"},
		{expr: "base64.encodeUrl(b'\\xfb\\xef\\xbe') == '----'"},
		{expr: "base64.encode(b'\\xfb\\xef\\xbe') != base64.encodeUrl(b'\\xfb\\xef\\xbe')"},
		{expr: "base64.encode(b'\\xfb\\xff\\xfe') == '+//+'"},
		{expr: "base64.encodeUrl(b'\\xfb\\xff\\xfe') == '-__-'"},
		{expr: "base64.encode(b'\\xfb\\xff\\xfe') != base64.encodeUrl(b'\\xfb\\xff\\xfe')"},
		// Base64 decoding fails for characters unique to Base64URL (- and _)
		{
			expr: "base64.decode('____')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decode('----')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decode('-__-')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decode('-w==')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decode('-w')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decode(base64.encodeUrl(b'\\xff\\xff\\xff'))",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decode(base64.encodeUrl(b'\\xfb'))",
			err:  "illegal base64 data",
		},
		// Base64URL decoding fails for characters unique to standard Base64 (+ and /)
		{
			expr: "base64.decodeUrl('////')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decodeUrl('++++')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decodeUrl('+//+')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decodeUrl('+w==')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decodeUrl('+w')",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decodeUrl(base64.encode(b'\\xff\\xff\\xff'))",
			err:  "illegal base64 data",
		},
		{
			expr: "base64.decodeUrl(base64.encode(b'\\xfb'))",
			err:  "illegal base64 data",
		},
		// Respective decoders produce matching byte results for equivalent inputs
		{expr: "base64.decode('////') == b'\\xff\\xff\\xff'"},
		{expr: "base64.decode('+w==') == b'\\xfb'"},
		{expr: "base64.decode('+w') == b'\\xfb'"},
		{expr: "base64.decode('++++') == b'\\xfb\\xef\\xbe'"},
		{expr: "base64.decodeUrl('-w==') == b'\\xfb'"},
		{expr: "base64.decodeUrl('-w') == b'\\xfb'"},
		{expr: "base64.decodeUrl('----') == b'\\xfb\\xef\\xbe'"},
		{expr: "base64.decode('+//+') == b'\\xfb\\xff\\xfe'"},
		{expr: "base64.decodeUrl('-__-') == b'\\xfb\\xff\\xfe'"},
		{expr: "base64.decode('////') == base64.decodeUrl('____')"},
		{expr: "base64.decode('+w==') == base64.decodeUrl('-w==')"},
		{expr: "base64.decode('++++') == base64.decodeUrl('----')"},
		{expr: "base64.decode('+//+') == base64.decodeUrl('-__-')"},
		// JSON Encode Positive & Edge Cases
		{expr: "json.encode('hello') == '\"hello\"'"},
		{expr: "json.encode(123) == '123'"},
		{expr: "json.encode(-42) == '-42'"},
		{expr: "json.encode(0) == '0'"},
		{expr: "json.encode(true) == 'true'"},
		{expr: "json.encode(false) == 'false'"},
		{expr: "json.encode(null) == 'null'"},
		{expr: "json.encode([]) == '[]'"},
		{expr: "json.encode({}) == '{}'"},
		{expr: "json.parse(json.encode([1, 'two', true])) == optional.of([1, 'two', true])"},
		{expr: "json.parse(json.encode({'items': [1, 'two', false]})) == optional.of({'items': [1, 'two', false]})"},
		{expr: "json.parse(json.encode({'a': 1, 'b': 2})) == optional.of({'a': 1, 'b': 2})"},
		{expr: `json.parse(json.encode('hello\nworld\t"')) == optional.of("hello\nworld\t\"")`},
		// JSON Parse Dynamic Positive & Edge Cases
		{expr: `json.parse('"hello"') == optional.of('hello')`},
		{expr: `json.parse('123') == optional.of(123)`},
		{expr: `json.parse('-42') == optional.of(-42)`},
		{expr: `json.parse('0') == optional.of(0)`},
		{expr: `json.parse('123.5') == optional.of(123.5)`},
		{expr: `json.parse('-3.14') == optional.of(-3.14)`},
		{expr: `json.parse('1e2') == optional.of(100.0)`},
		{expr: `json.parse('1.25e2') == optional.of(125.0)`},
		{expr: `json.parse('true') == optional.of(true)`},
		{expr: `json.parse('false') == optional.of(false)`},
		{expr: `json.parse('null') == optional.of(null)`},
		{expr: `json.parse('[]') == optional.of([])`},
		{expr: `json.parse('{}') == optional.of({})`},
		{expr: `json.parse('[1, "two", true]') == optional.of([1, 'two', true])`},
		{expr: `json.parse('{"items":[1,"two",false]}') == optional.of({'items': [1, 'two', false]})`},
		{expr: `json.parse('{"a": [1, {"b": true}, "three"]}') == optional.of({'a': [1, {'b': true}, 'three']})`},
		{expr: `json.parse('"hello\\nworld\\t\\\""') == optional.of("hello\nworld\t\"")`},
		{expr: `json.parse('"\\u003chello\\u003e"') == optional.of('<hello>')`},
		{expr: `json.parse('"🚀"') == optional.of('🚀')`},
		{expr: `json.parse('  \n\t {"a": 1} \r\n ') == optional.of({'a': 1})`},
		{expr: `json.parse('  \t 42 \n ') == optional.of(42)`},
		{expr: `json.parse(json.encode({'a': 1, 'b': 2})) == optional.of({'a': 1, 'b': 2})`},
		// JSON Parse Typed Positive & Edge Cases
		{expr: `json.parse('123', int) == optional.of(123)`},
		{expr: `json.parse('-100', int) == optional.of(-100)`},
		{expr: `json.parse('0', int) == optional.of(0)`},
		{expr: `json.parse('9223372036854775807', int) == optional.of(9223372036854775807)`},
		{expr: `json.parse('-9223372036854775808', int) == optional.of(-9223372036854775808)`},
		{expr: `json.parse('123', uint) == optional.of(123u)`},
		{expr: `json.parse('0', uint) == optional.of(0u)`},
		{expr: `json.parse('18446744073709551615', uint) == optional.of(18446744073709551615u)`},
		{expr: `json.parse('1.5', double) == optional.of(1.5)`},
		{expr: `json.parse('-2.5', double) == optional.of(-2.5)`},
		{expr: `json.parse('42', double) == optional.of(42.0)`},
		{expr: `json.parse('true', bool) == optional.of(true)`},
		{expr: `json.parse('false', bool) == optional.of(false)`},
		{expr: `json.parse('"hello"', string) == optional.of('hello')`},
		{expr: `json.parse('""', string) == optional.of('')`},
		{expr: `json.parse('"aGVsbG8="', bytes) == optional.of(b'hello')`},
		{expr: `json.parse('"AQID"', bytes) == optional.of(b'\x01\x02\x03')`},
		{expr: `json.parse('""', bytes) == optional.of(b'')`},
		{expr: `json.parse('null', null_type) == optional.of(null)`},
		{expr: `json.parse('"2023-01-01T00:00:00Z"', type(timestamp('2023-01-01T00:00:00Z'))) == optional.of(timestamp('2023-01-01T00:00:00Z'))`},
		{expr: `json.parse('"5s"', type(duration('5s'))) == optional.of(duration('5s'))`},
		{expr: `json.parse('"1h30m"', type(duration('5s'))) == optional.of(duration('1h30m'))`},
		{expr: `json.parse('[1, 2, 3]', type([1])) == optional.of([1, 2, 3])`},
		{expr: `json.parse('[]', type([1])) == optional.of([])`},
		{expr: `json.parse('{"a": 1}', type({'': 1})) == optional.of({'a': 1})`},
		{expr: `json.parse('{}', type({'': 1})) == optional.of({})`},
		{expr: `json.parse('  \t 42 \n ', int) == optional.of(42)`},
		// JSON Parse Negative Cases (Malformed Syntax)
		{expr: `json.parse('invalid') == optional.none()`},
		{expr: `json.parse('') == optional.none()`},
		{expr: `json.parse('   ') == optional.none()`},
		{expr: `json.parse('{') == optional.none()`},
		{expr: `json.parse('[1, 2') == optional.none()`},
		{expr: `json.parse('"unterminated') == optional.none()`},
		{expr: `json.parse('{key: "value"}') == optional.none()`},
		{expr: `json.parse("'single_quote'") == optional.none()`},
		{expr: `json.parse('[1, 2,]') == optional.none()`},
		{expr: `json.parse('{"a": 1,}') == optional.none()`},
		{expr: `json.parse('123 abc') == optional.none()`},
		{expr: `json.parse('{"a": 1} {"b": 2}') == optional.none()`},
		{expr: `json.parse('true false') == optional.none()`},
		// JSON Parse Negative Cases (Type Mismatches)
		{expr: `json.parse('"not_an_int"', int) == optional.none()`},
		{expr: `json.parse('1.5', int) == optional.none()`},
		{expr: `json.parse('"123"', int) == optional.none()`},
		{expr: `json.parse('true', int) == optional.none()`},
		{expr: `json.parse('null', int) == optional.none()`},
		{expr: `json.parse('{"a": 1}', int) == optional.none()`},
		{expr: `json.parse('-1', uint) == optional.none()`},
		{expr: `json.parse('1.5', uint) == optional.none()`},
		{expr: `json.parse('"123"', uint) == optional.none()`},
		{expr: `json.parse('null', uint) == optional.none()`},
		{expr: `json.parse('"1.5"', double) == optional.none()`},
		{expr: `json.parse('true', double) == optional.none()`},
		{expr: `json.parse('null', double) == optional.none()`},
		{expr: `json.parse('123', bool) == optional.none()`},
		{expr: `json.parse('"true"', bool) == optional.none()`},
		{expr: `json.parse('null', bool) == optional.none()`},
		{expr: `json.parse('123', string) == optional.none()`},
		{expr: `json.parse('true', string) == optional.none()`},
		{expr: `json.parse('null', string) == optional.none()`},
		{expr: `json.parse('123', bytes) == optional.none()`},
		{expr: `json.parse('"not valid base64 %%%"', bytes) == optional.none()`},
		{expr: `json.parse('123', null_type) == optional.none()`},
		{expr: `json.parse('"null"', null_type) == optional.none()`},
		{expr: `json.parse('123', type(timestamp('2023-01-01T00:00:00Z'))) == optional.none()`},
		{expr: `json.parse('"invalid-time"', type(timestamp('2023-01-01T00:00:00Z'))) == optional.none()`},
		{expr: `json.parse('123', type(duration('5s'))) == optional.none()`},
		{expr: `json.parse('"10x"', type(duration('5s'))) == optional.none()`},
		{expr: `json.parse('123', type([1])) == optional.none()`},
		{expr: `json.parse('{"a": 1}', type([1])) == optional.none()`},
		{expr: `json.parse('123', type({'': 1})) == optional.none()`},
		{expr: `json.parse('[1, 2]', type({'': 1})) == optional.none()`},
	}

	env, err := cel.NewEnv(cel.OptionalTypes(), Encoders())
	if err != nil {
		t.Fatalf("cel.NewEnv(Encoders()) failed: %v", err)
	}
	for i, tst := range tests {
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
					t.Fatalf("env.Program(%s) failed: %v", tc.expr, err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if tc.err != "" {
					if err == nil {
						t.Fatalf("got %v, wanted error %s for expr: %s",
							out.Value(), tc.err, tc.expr)
					}
					if !strings.Contains(err.Error(), tc.err) {
						t.Errorf("got error %v, wanted error %s for expr: %s", err, tc.err, tc.expr)
					}
				} else if err != nil {
					t.Fatalf("prg.Eval() failed for expr %q: %v", tc.expr, err)
				} else if out.Value() != true {
					t.Errorf("got %v, wanted true for expr %q", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestEncodersVersion(t *testing.T) {
	env, err := cel.NewEnv(Encoders(EncodersVersion(0)))
	if err != nil {
		t.Fatalf("EncodersVersion(0) failed: %v", err)
	}
	if _, iss := env.Compile("base64.encode(b'hello')"); iss.Err() != nil {
		t.Fatalf("base64.encode() got %v, wanted no error", iss.Err())
	}
	if _, iss := env.Compile("json.encode('hello')"); iss.Err() == nil {
		t.Fatal("json.encode() got no error, wanted version-gated function to be unavailable")
	}
	if _, iss := env.Compile("base64.encodeUrl(b'hello')"); iss.Err() == nil {
		t.Fatal("base64.encodeUrl() got no error, wanted version-gated function to be unavailable")
	}
	if _, iss := env.Compile("json.parse('\"hello\"')"); iss.Err() == nil {
		t.Fatal("json.parse() got no error, wanted version-gated function to be unavailable")
	}

	env, err = cel.NewEnv(Encoders(EncodersVersion(1)))
	if err != nil {
		t.Fatalf("EncodersVersion(1) failed: %v", err)
	}
	if _, iss := env.Compile("json.encode('hello')"); iss.Err() != nil {
		t.Fatalf("json.encode() got %v, wanted no error", iss.Err())
	}
	if _, iss := env.Compile("base64.encodeUrl(b'hello')"); iss.Err() == nil {
		t.Fatal("base64.encodeUrl() got no error, wanted version-gated function to be unavailable")
	}

	_, err = cel.NewEnv(Encoders(EncodersVersion(2)))
	if err == nil || !strings.Contains(err.Error(), "encoders library requires the optional library") {
		t.Fatalf("EncodersVersion(2) without optional types got %v, wanted encoders library requires the optional library", err)
	}

	env, err = cel.NewEnv(cel.OptionalTypes(), Encoders(EncodersVersion(2)))
	if err != nil {
		t.Fatalf("EncodersVersion(2) failed: %v", err)
	}
	if _, iss := env.Compile("base64.encodeUrl(b'hello')"); iss.Err() != nil {
		t.Fatalf("base64.encodeUrl() got %v, wanted no error", iss.Err())
	}
	if _, iss := env.Compile("base64.decodeUrl('aGVsbG8=')"); iss.Err() != nil {
		t.Fatalf("base64.decodeUrl() got %v, wanted no error", iss.Err())
	}
	if _, iss := env.Compile("json.parse('\"hello\"')"); iss.Err() != nil {
		t.Fatalf("json.parse() got %v, wanted no error", iss.Err())
	}
	if _, iss := env.Compile("json.parse('123', int)"); iss.Err() != nil {
		t.Fatalf("json.parse(str, int) got %v, wanted no error", iss.Err())
	}
}

func TestEncoders_MissingOptionalTypes(t *testing.T) {
	tests := []struct {
		name string
		opts []cel.EnvOption
	}{
		{
			name: "no optional types default version",
			opts: []cel.EnvOption{Encoders()},
		},
		{
			name: "no optional types version 2",
			opts: []cel.EnvOption{Encoders(EncodersVersion(2))},
		},
		{
			name: "optional types after encoders",
			opts: []cel.EnvOption{Encoders(), cel.OptionalTypes()},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cel.NewEnv(tc.opts...)
			if err == nil || !strings.Contains(err.Error(), "encoders library requires the optional library") {
				t.Fatalf("cel.NewEnv() got %v, wanted encoders library requires the optional library", err)
			}
		})
	}
}

func testEncodersCostsEnv(t *testing.T, version int, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{
		cel.OptionalTypes(),
		Encoders(EncodersVersion(uint32(version))),
		cel.EnableMacroCallTracking(),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Encoders()) failed: %v", err)
	}
	return env
}

func TestEncodersCosts(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost checker.CostEstimate
		actualCost    uint64
		version       int
	}{
		{
			name: "encode_bytes_v0",
			expr: "base64.encode(x) == 'aGVsbG8='",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.BytesType),
			},
			in: map[string]any{
				"x": []byte("hello"),
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.FixedCostEstimate(3), // x lookup (1) + encode (1) + == (1) = 3
			actualCost:    3,
			version:       0,
		},
		{
			name: "encode_bytes_v1",
			expr: "base64.encode(x) == 'aGVsbG8='",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.BytesType),
			},
			in: map[string]any{
				"x": []byte("hello"),
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 13}, // x lookup (1) + encode (100 * 0.1 + 1 = 11) + == (1) = 13
			actualCost:    4,                                     // x lookup (1) + encode (ceil(5 * 0.1) + 1 = 2) + == (1) = 4
			version:       1,
		},
		{
			name: "decode_string_v0",
			expr: "base64.decode(x) == b'hello'",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in: map[string]any{
				"x": "aGVsbG8=",
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.FixedCostEstimate(3),
			actualCost:    3,
			version:       0,
		},
		{
			name: "decode_string_v1",
			expr: "base64.decode(x) == b'hello'",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in: map[string]any{
				"x": "aGVsbG8=",
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 13}, // x lookup (1) + decode (100 * 0.1 + 1 = 11) + == (1) = 13
			actualCost:    4,                                     // x lookup (1) + decode (ceil(8 * 0.1) + 1 = 2) + == (1) = 4
			version:       1,
		},
		{
			name:          "encode_bytes_v1_literal",
			expr:          "base64.encode(b'hello') == 'aGVsbG8='",
			estimatedCost: checker.FixedCostEstimate(3),
			actualCost:    3,
			version:       1,
		},
		{
			name:          "decode_string_v1_literal",
			expr:          "base64.decode('aGVsbG8=') == b'hello'",
			estimatedCost: checker.FixedCostEstimate(3),
			actualCost:    3,
			version:       1,
		},
		{
			name:          "encode_empty_bytes_v1_literal",
			expr:          "base64.encode(b'') == ''",
			estimatedCost: checker.FixedCostEstimate(1),
			actualCost:    1,
			version:       1,
		},
		{
			name:          "decode_empty_string_v1_literal",
			expr:          "base64.decode('') == b''",
			estimatedCost: checker.FixedCostEstimate(1),
			actualCost:    1,
			version:       1,
		},
		{
			name:          "encode_non_utf8_bytes_v1_literal",
			expr:          "base64.encode(b'\xff\xfe\xfd') != '////'",
			estimatedCost: checker.FixedCostEstimate(3),
			actualCost:    3,
			version:       1,
		},
		{
			name: "json_encode_dyn",
			expr: "json.encode(x) == '\"hello\"'",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.DynType),
			},
			in: map[string]any{
				"x": "hello",
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 2, Max: math.MaxUint64},
			actualCost:    math.MaxUint64,
			version:       1,
		},
		{
			name: "encode_url_bytes_v2",
			expr: "base64.encodeUrl(x) == '____'",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.BytesType),
			},
			in: map[string]any{
				"x": []byte("\xff\xff\xff"),
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 13},
			actualCost:    4,
			version:       2,
		},
		{
			name: "decode_url_string_v2",
			expr: "base64.decodeUrl(x) == b'hello'",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in: map[string]any{
				"x": "aGVsbG8=",
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 13},
			actualCost:    4,
			version:       2,
		},
		{
			name: "json_parse_string_type",
			expr: "json.parse(x, string) == optional.of('hello')",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in: map[string]any{
				"x": "\"hello\"",
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 4, Max: math.MaxUint64},
			actualCost:    math.MaxUint64,
			version:       2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := testEncodersCostsEnv(t, tc.version, tc.vars...)
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost)
			}
		})
	}
}

func TestDecodeNonBase64Error(t *testing.T) {
	env := testEncodersCostsEnv(t, 1)
	pAst, iss := env.Parse("base64.decode('abc-') == b''")
	if iss.Err() != nil {
		t.Fatalf("env.Parse() failed: %v", iss.Err())
	}
	cAst, iss := env.Check(pAst)
	if iss.Err() != nil {
		t.Fatalf("env.Check() failed: %v", iss.Err())
	}
	testCheckCost(t, env, cAst, nil, checker.FixedCostEstimate(2))
	prgOpts := []cel.ProgramOption{}
	if cAst.IsChecked() {
		prgOpts = append(prgOpts, cel.CostTracking(nil))
	}
	prg, err := env.Program(cAst, prgOpts...)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, _, err = prg.Eval(cel.NoVars())
	if err == nil {
		t.Fatal("expected eval error for non-base64 string decoding, got nil")
	}
}

func TestDecodeNonBase64UrlError(t *testing.T) {
	env := testEncodersCostsEnv(t, 2)
	pAst, iss := env.Parse("base64.decodeUrl('abc!') == b''")
	if iss.Err() != nil {
		t.Fatalf("env.Parse() failed: %v", iss.Err())
	}
	cAst, iss := env.Check(pAst)
	if iss.Err() != nil {
		t.Fatalf("env.Check() failed: %v", iss.Err())
	}
	testCheckCost(t, env, cAst, nil, checker.FixedCostEstimate(2))
	prgOpts := []cel.ProgramOption{}
	if cAst.IsChecked() {
		prgOpts = append(prgOpts, cel.CostTracking(nil))
	}
	prg, err := env.Program(cAst, prgOpts...)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, _, err = prg.Eval(cel.NoVars())
	if err == nil {
		t.Fatal("expected eval error for non-base64url string decoding, got nil")
	}
}

func TestJSONEncodeCostUnbounded(t *testing.T) {
	env, err := cel.NewEnv(Encoders(EncodersVersion(1)))
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("json.encode('hello')")
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	// 1. Check Cost Estimate is unbounded: Max is MaxUint64
	est, err := env.EstimateCost(ast, testCostHintEstimator{})
	if err != nil {
		t.Fatalf("env.EstimateCost() failed: %v", err)
	}
	wantEst := checker.CostEstimate{Min: 0, Max: math.MaxUint64}
	if est != wantEst {
		t.Errorf("env.EstimateCost() got %v, wanted %v", est, wantEst)
	}

	// 2. Check Actual Cost is math.MaxUint64
	prg, err := env.Program(ast, cel.CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, det, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if det.ActualCost() == nil {
		t.Fatal("det.ActualCost() got nil, wanted a value")
	}
	if *det.ActualCost() != math.MaxUint64 {
		t.Errorf("det.ActualCost() got %d, wanted %d", *det.ActualCost(), uint64(math.MaxUint64))
	}
}

func TestJSONParseCostUnbounded(t *testing.T) {
	env, err := cel.NewEnv(cel.OptionalTypes(), Encoders(EncodersVersion(2)))
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("json.parse('\"hello\"')")
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	// 1. Check Cost Estimate is unbounded: Max is MaxUint64
	est, err := env.EstimateCost(ast, testCostHintEstimator{})
	if err != nil {
		t.Fatalf("env.EstimateCost() failed: %v", err)
	}
	wantEst := checker.CostEstimate{Min: 0, Max: math.MaxUint64}
	if est != wantEst {
		t.Errorf("env.EstimateCost() got %v, wanted %v", est, wantEst)
	}

	// 2. Check Actual Cost is math.MaxUint64
	prg, err := env.Program(ast, cel.CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, det, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if det.ActualCost() == nil {
		t.Fatal("det.ActualCost() got nil, wanted a value")
	}
	if *det.ActualCost() != math.MaxUint64 {
		t.Errorf("det.ActualCost() got %d, wanted %d", *det.ActualCost(), uint64(math.MaxUint64))
	}
}

type testNativeUser struct {
	Username string `json:"user_name" cel:"username"`
	Age      int    `json:"age,omitempty" cel:"age"`
	Secret   string `json:"-" cel:"secret"`
}

func TestJSONParseNativeTypes(t *testing.T) {
	nativeType, err := types.NewNativeType(reflect.TypeFor[testNativeUser](), types.ParseStructTag("cel"))
	if err != nil {
		t.Fatalf("types.NewNativeType failed: %v", err)
	}
	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Types(nativeType),
		cel.Variable("userJson", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}
	tests := []struct {
		expr string
		vars map[string]any
	}{
		{
			expr: "json.parse(userJson, ext.testNativeUser).value().username == 'Alice' && json.parse(userJson, ext.testNativeUser).value().age == 25",
			vars: map[string]any{
				"userJson": `{"user_name":"Alice","age":25,"secret":"supersecret"}`,
			},
		},
		{
			expr: "json.parse(userJson, ext.testNativeUser).value().secret == ''",
			vars: map[string]any{
				"userJson": `{"user_name":"Alice","age":25,"secret":"supersecret"}`,
			},
		},
		{
			expr: "json.parse(json.encode(json.parse(userJson, ext.testNativeUser).value()), ext.testNativeUser).value().username == 'Alice' && json.parse(json.encode(json.parse(userJson, ext.testNativeUser).value()), ext.testNativeUser).value().age == 25",
			vars: map[string]any{
				"userJson": `{"user_name":"Alice","age":25,"secret":"supersecret"}`,
			},
		},
		{
			expr: "json.encode(json.parse(userJson, ext.testNativeUser).value()) == '{\"user_name\":\"Alice\"}'",
			vars: map[string]any{
				"userJson": `{"user_name":"Alice"}`,
			},
		},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.vars)
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Value() != true {
				t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
			}
		})
	}
}

func TestJSONParseLimits(t *testing.T) {
	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Variable("largeJson", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("json.parse(largeJson)")
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	largeStr := `"` + strings.Repeat("a", maxJSONSize) + `"`
	_, _, err = prg.Eval(map[string]any{
		"largeJson": largeStr,
	})
	if err == nil {
		t.Fatal("expected size limit error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum allowed limit") {
		t.Errorf("expected exceeds limit error, got: %v", err)
	}
}

func TestJSONParseProtobufTypes(t *testing.T) {
	envProto3, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Container("google.expr.proto3.test"),
		cel.Types(
			&structpb.Struct{},
			&structpb.ListValue{},
			&proto3pb.TestAllTypes{},
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	proto3Tests := []string{
		`json.parse('{"k": "v"}', type(google.protobuf.Struct{})).hasValue()`,
		`json.parse('{"hello": "world"}', type(google.protobuf.Struct{})).value().hello == 'world'`,
		`json.parse('[1, "two"]', type(google.protobuf.ListValue{})).value()[0] == 1`,
		`json.parse('[1, "two"]', type(google.protobuf.ListValue{})).value()[1] == 'two'`,
		`json.parse('invalid', type(google.protobuf.Struct{})) == optional.none()`,
		`json.parse('{"single_int32": 42, "single_string": "cel"}', TestAllTypes).value().single_int32 == 42`,
		`json.parse('{"single_int32": 42, "single_string": "cel"}', TestAllTypes).value().single_string == 'cel'`,
		`json.parse('{"single_int32": 42, "single_string": "cel"}', TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel'}`,
		`json.parse('{"single_int32": 42, "single_string": "cel", "single_bool": false, "single_int64": 0}', TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel'}`,
		`json.parse('{"single_int32": 42, "single_string": "cel", "single_bool": false, "single_int64": 0}', TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel', single_bool: false, single_int64: 0}`,
		`json.parse('invalid', TestAllTypes) == optional.none()`,
		`json.parse(json.encode(TestAllTypes{single_int32: 42, single_string: 'cel'}), TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel'}`,
	}
	for i, expr := range proto3Tests {
		t.Run(fmt.Sprintf("proto3[%d]", i), func(t *testing.T) {
			ast, iss := envProto3.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
			}
			prg, err := envProto3.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Value() != true {
				t.Errorf("got %v, wanted true for expr: %s", out.Value(), expr)
			}
		})
	}

	envProto2, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Container("google.expr.proto2.test"),
		cel.Types(
			&proto2pb.TestAllTypes{},
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	proto2Tests := []string{
		`json.parse('{"single_int32": 42, "single_string": "cel"}', TestAllTypes).value().single_int32 == 42`,
		`json.parse('{"single_int32": 42, "single_string": "cel"}', TestAllTypes).value().single_string == 'cel'`,
		`json.parse('{"single_int32": 42, "single_string": "cel"}', TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel'}`,
		`json.parse('{"single_int32": 42, "single_string": "cel", "single_bool": false, "single_int64": 0}', TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel', single_bool: false, single_int64: 0}`,
		`json.parse('invalid', TestAllTypes) == optional.none()`,
		`json.parse(json.encode(TestAllTypes{single_int32: 42, single_string: 'cel'}), TestAllTypes).value() == TestAllTypes{single_int32: 42, single_string: 'cel'}`,
	}
	for i, expr := range proto2Tests {
		t.Run(fmt.Sprintf("proto2[%d]", i), func(t *testing.T) {
			ast, iss := envProto2.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
			}
			prg, err := envProto2.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Value() != true {
				t.Errorf("got %v, wanted true for expr: %s", out.Value(), expr)
			}
		})
	}
}

func TestEncodersRoundtrip(t *testing.T) {
	nativeType, err := types.NewNativeType(reflect.TypeFor[testNativeUser](), types.ParseStructTag("cel"))
	if err != nil {
		t.Fatalf("types.NewNativeType failed: %v", err)
	}
	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Container("google.expr.proto3.test"),
		cel.Types(
			nativeType,
			&proto3pb.TestAllTypes{},
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	tests := []struct {
		name string
		expr string
	}{
		{name: "string_simple", expr: `json.parse(json.encode('hello world')) == optional.of('hello world')`},
		{name: "string_escapes", expr: `json.parse(json.encode("hello\n\t\"\\world")) == optional.of("hello\n\t\"\\world")`},
		{name: "string_unicode", expr: `json.parse(json.encode('<hello>&"world"')) == optional.of('<hello>&"world"')`},
		{name: "string_emoji", expr: `json.parse(json.encode('🎉 🚀 CEL')) == optional.of('🎉 🚀 CEL')`},
		{name: "string_empty", expr: `json.parse(json.encode('')) == optional.of('')`},
		{name: "int_zero", expr: `json.parse(json.encode(0)) == optional.of(0)`},
		{name: "int_pos", expr: `json.parse(json.encode(42)) == optional.of(42)`},
		{name: "int_neg", expr: `json.parse(json.encode(-42)) == optional.of(-42)`},
		{name: "int_safe_max", expr: `json.parse(json.encode(9007199254740991)) == optional.of(9007199254740991)`},
		{name: "int_safe_min", expr: `json.parse(json.encode(-9007199254740991)) == optional.of(-9007199254740991)`},
		{name: "int_64bit_overflow", expr: `json.encode(9223372036854775807) == '"9223372036854775807"' && int(json.parse(json.encode(9223372036854775807)).value()) == 9223372036854775807`},
		{name: "uint_zero", expr: `json.parse(json.encode(0u), uint) == optional.of(0u)`},
		{name: "uint_val", expr: `json.parse(json.encode(42u), uint) == optional.of(42u)`},
		{name: "uint_safe_max", expr: `json.parse(json.encode(9007199254740991u), uint) == optional.of(9007199254740991u)`},
		{name: "uint_64bit_overflow", expr: `json.encode(18446744073709551615u) == '"18446744073709551615"' && uint(json.parse(json.encode(18446744073709551615u)).value()) == 18446744073709551615u`},
		{name: "double_zero", expr: `json.parse(json.encode(0.0), double) == optional.of(0.0)`},
		{name: "double_pos", expr: `json.parse(json.encode(1.5), double) == optional.of(1.5)`},
		{name: "double_neg", expr: `json.parse(json.encode(-3.14), double) == optional.of(-3.14)`},
		{name: "bool_true", expr: `json.parse(json.encode(true)) == optional.of(true)`},
		{name: "bool_false", expr: `json.parse(json.encode(false)) == optional.of(false)`},
		{name: "null_val", expr: `json.parse(json.encode(null)) == optional.of(null)`},
		{name: "bytes_empty", expr: `json.parse(json.encode(b''), bytes) == optional.of(b'')`},
		{name: "bytes_val", expr: `json.parse(json.encode(b'hello'), bytes) == optional.of(b'hello')`},
		{name: "bytes_binary", expr: `json.parse(json.encode(b'\x00\x01\x02\xff'), bytes) == optional.of(b'\x00\x01\x02\xff')`},
		{name: "timestamp_val", expr: `json.parse(json.encode(timestamp('2023-01-01T00:00:00Z')), type(timestamp('2023-01-01T00:00:00Z'))) == optional.of(timestamp('2023-01-01T00:00:00Z'))`},
		{name: "duration_val", expr: `json.parse(json.encode(duration('5s')), type(duration('5s'))) == optional.of(duration('5s'))`},
		{name: "list_empty", expr: `json.parse(json.encode([])) == optional.of([])`},
		{name: "list_primitives", expr: `json.parse(json.encode([1, 2, 3])) == optional.of([1, 2, 3])`},
		{name: "list_heterogeneous", expr: `json.parse(json.encode([1, 'two', true, null])) == optional.of([1, 'two', true, null])`},
		{name: "list_nested", expr: `json.parse(json.encode([[1, 2], [3, 4]])) == optional.of([[1, 2], [3, 4]])`},
		{name: "map_empty", expr: `json.parse(json.encode({})) == optional.of({})`},
		{name: "map_simple", expr: `json.parse(json.encode({'a': 1, 'b': 2})) == optional.of({'a': 1, 'b': 2})`},
		{name: "map_nested", expr: `json.parse(json.encode({'items': [1, {'nested': true}], 'count': 42})) == optional.of({'items': [1, {'nested': true}], 'count': 42})`},
		{name: "base64_roundtrip_empty", expr: `base64.decode(base64.encode(b'')) == b''`},
		{name: "base64_roundtrip_simple", expr: `base64.decode(base64.encode(b'hello world')) == b'hello world'`},
		{name: "proto_roundtrip", expr: `json.parse(json.encode(TestAllTypes{single_int32: 42, single_string: 'cel'}), TestAllTypes).value().single_int32 == 42`},
		{name: "proto_enum_roundtrip", expr: `json.parse(json.encode(TestAllTypes{single_nested_enum: TestAllTypes.NestedEnum.BAR}), TestAllTypes).value().single_nested_enum == TestAllTypes.NestedEnum.BAR`},
		{name: "proto_nested_roundtrip", expr: `json.parse(json.encode(TestAllTypes{single_nested_message: TestAllTypes.NestedMessage{bb: 100}}), TestAllTypes).value().single_nested_message.bb == 100`},
		{name: "proto_repeated_roundtrip", expr: `json.parse(json.encode(TestAllTypes{repeated_int32: [1, 2, 3]}), TestAllTypes).value().repeated_int32 == [1, 2, 3]`},
		{name: "proto_map_roundtrip", expr: `json.parse(json.encode(TestAllTypes{map_string_string: {'k': 'v'}}), TestAllTypes).value().map_string_string['k'] == 'v'`},
		{name: "proto_structpb_roundtrip", expr: `json.parse(json.encode(TestAllTypes{single_struct: {'hello': 'world'}}), TestAllTypes).value().single_struct['hello'] == 'world'`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Value() != true {
				t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
			}
		})
	}
}

func BenchmarkJSONEncode(b *testing.B) {
	nativeType, _ := types.NewNativeType(reflect.TypeFor[testNativeUser](), types.ParseStructTag("cel"))
	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Types(
			nativeType,
			&proto3pb.TestAllTypes{},
			types.NewObjectType("google.expr.proto3.test.TestAllTypes"),
		),
	)
	if err != nil {
		b.Fatalf("cel.NewEnv failed: %v", err)
	}

	cases := []struct {
		name string
		expr string
	}{
		{name: "ScalarInt", expr: "json.encode(42)"},
		{name: "ScalarString", expr: "json.encode('hello world')"},
		{name: "ListSmall", expr: "json.encode([1, 'two', true, null])"},
		{name: "MapSmall", expr: "json.encode({'a': 1, 'b': 'two', 'c': true})"},
		{name: "MapNested", expr: "json.encode({'items': [1, {'nested': true}], 'count': 42})"},
		{name: "ProtoMessage", expr: "json.encode(google.expr.proto3.test.TestAllTypes{single_int32: 42, single_string: 'cel'})"},
		{name: "NativeStruct", expr: "json.encode(ext.testNativeUser{username: 'Alice', age: 25})"},
	}

	for _, c := range cases {
		ast, iss := env.Compile(c.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", c.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			b.Fatalf("env.Program() failed: %v", err)
		}
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					b.Fatalf("prg.Eval() failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkJSONParse(b *testing.B) {
	nativeType, _ := types.NewNativeType(reflect.TypeFor[testNativeUser](), types.ParseStructTag("cel"))
	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		Encoders(),
		cel.Types(
			nativeType,
			&proto3pb.TestAllTypes{},
			types.NewObjectType("google.expr.proto3.test.TestAllTypes"),
		),
	)
	if err != nil {
		b.Fatalf("cel.NewEnv failed: %v", err)
	}

	cases := []struct {
		name string
		expr string
	}{
		{name: "DynamicScalar", expr: "json.parse('42')"},
		{name: "DynamicString", expr: "json.parse('\"hello world\"')"},
		{name: "DynamicMap", expr: "json.parse('{\"a\": 1, \"b\": \"two\", \"c\": true}')"},
		{name: "DynamicNested", expr: "json.parse('{\"items\": [1, {\"nested\": true}], \"count\": 42}')"},
		{name: "TypedInt", expr: "json.parse('42', int)"},
		{name: "TypedString", expr: "json.parse('\"hello world\"', string)"},
		{name: "TypedList", expr: "json.parse('[1, 2, 3, 4, 5]', type([1]))"},
		{name: "TypedMap", expr: "json.parse('{\"a\": 1, \"b\": 2}', type({'': 1}))"},
		{name: "TypedProto", expr: "json.parse('{\"single_int32\": 42, \"single_string\": \"cel\"}', google.expr.proto3.test.TestAllTypes)"},
		{name: "TypedNativeStruct", expr: "json.parse('{\"user_name\": \"Alice\", \"age\": 25}', ext.testNativeUser)"},
	}

	for _, c := range cases {
		ast, iss := env.Compile(c.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", c.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			b.Fatalf("env.Program() failed: %v", err)
		}
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					b.Fatalf("prg.Eval() failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkBase64(b *testing.B) {
	env, err := cel.NewEnv(cel.OptionalTypes(), Encoders())
	if err != nil {
		b.Fatalf("cel.NewEnv failed: %v", err)
	}

	cases := []struct {
		name string
		expr string
	}{
		{name: "EncodeShort", expr: "base64.encode(b'hello world')"},
		{name: "EncodeBinary", expr: "base64.encode(b'\\x00\\x01\\x02\\x03\\x04\\x05\\x06\\x07\\x08\\x09')"},
		{name: "DecodeShort", expr: "base64.decode('aGVsbG8gd29ybGQ=')"},
		{name: "DecodeBinary", expr: "base64.decode('AAECAwQFBgcICQ==')"},
	}

	for _, c := range cases {
		ast, iss := env.Compile(c.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", c.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			b.Fatalf("env.Program() failed: %v", err)
		}
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					b.Fatalf("prg.Eval() failed: %v", err)
				}
			}
		})
	}
}

func TestEncodersStringConformance(t *testing.T) {
	env, err := cel.NewEnv(cel.OptionalTypes(), Encoders())
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	tests := []struct {
		name string
		expr string
	}{
		// Valid escapes in json.parse
		{name: "escaped_quote", expr: `json.parse('"hello \\"world\\""') == optional.of('hello "world"')`},
		{name: "escaped_backslash", expr: `json.parse('"hello\\\\world"') == optional.of('hello\\world')`},
		{name: "escaped_slash", expr: `json.parse('"hello\\/world"') == optional.of('hello/world')`},
		{name: "escaped_backspace", expr: `json.parse('"hello\\bworld"') == optional.of("hello\bworld")`},
		{name: "escaped_formfeed", expr: `json.parse('"hello\\fworld"') == optional.of("hello\fworld")`},
		{name: "escaped_newline", expr: `json.parse('"hello\\nworld"') == optional.of("hello\nworld")`},
		{name: "escaped_carriage_return", expr: `json.parse('"hello\\rworld"') == optional.of("hello\rworld")`},
		{name: "escaped_tab", expr: `json.parse('"hello\\tworld"') == optional.of("hello\tworld")`},
		{name: "escaped_unicode_ascii", expr: `json.parse('"\\u0041\\u0042\\u0043"') == optional.of('ABC')`},
		{name: "escaped_unicode_cjk", expr: `json.parse('"\\u4e16\\u754c"') == optional.of('世界')`},
		{name: "escaped_unicode_surrogates", expr: `json.parse('"\\ud83d\\ude80"') == optional.of('🚀')`},
		{name: "escaped_null_byte", expr: `json.parse('"hello\\u0000world"') == optional.of("hello\x00world")`},
		// Typed string variants
		{name: "typed_escaped_quote", expr: `json.parse('"hello \\"world\\""', string) == optional.of('hello "world"')`},
		{name: "typed_escaped_unicode", expr: `json.parse('"\\u4e16\\u754c"', string) == optional.of('世界')`},
		{name: "typed_escaped_surrogates", expr: `json.parse('"\\ud83d\\ude80"', string) == optional.of('🚀')`},
		// Roundtrip with json.encode
		{name: "encode_quotes", expr: `json.parse(json.encode('"hello"')) == optional.of('"hello"')`},
		{name: "encode_backslashes", expr: `json.parse(json.encode('a\\b\\c')) == optional.of('a\\b\\c')`},
		{name: "encode_newlines_tabs", expr: `json.parse(json.encode("line1\nline2\ttab")) == optional.of("line1\nline2\ttab")`},
		{name: "encode_unicode_cjk", expr: `json.parse(json.encode('你好世界')) == optional.of('你好世界')`},
		{name: "encode_unicode_emojis", expr: `json.parse(json.encode('👋 🌍 🚀')) == optional.of('👋 🌍 🚀')`},
		// Negative / Malformed string parsing
		{name: "invalid_hex_escape", expr: `json.parse('"\\x41"') == optional.none()`},
		{name: "invalid_alert_escape", expr: `json.parse('"\\a"') == optional.none()`},
		{name: "invalid_vertical_tab", expr: `json.parse('"\\v"') == optional.none()`},
		{name: "incomplete_unicode_escape", expr: `json.parse('"\\u123"') == optional.none()`},
		{name: "unclosed_string", expr: `json.parse('"unclosed') == optional.none()`},
		{name: "trailing_data", expr: `json.parse('"valid" trailing') == optional.none()`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Value() != true {
				t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
			}
		})
	}
}

func TestPublicAPI(t *testing.T) {
	// Base64 Encode & Decode
	encodedB64 := Base64Encode([]byte("hello world"))
	if encodedB64 != "aGVsbG8gd29ybGQ=" {
		t.Errorf("got %q, wanted %q", encodedB64, "aGVsbG8gd29ybGQ=")
	}
	decodedB64, err := Base64Decode(encodedB64)
	if err != nil || string(decodedB64) != "hello world" {
		t.Errorf("Base64Decode failed: %v, got %q", err, string(decodedB64))
	}

	// JSON Encode
	intVal := types.Int(42)
	encodedInt, err := JSONEncode(intVal)
	if err != nil || encodedInt != "42" {
		t.Errorf("JSONEncode(42) failed: %v, got %q", err, encodedInt)
	}

	mapVal := types.DefaultTypeAdapter.NativeToValue(map[string]any{"hello": "world"})
	encodedMap, err := JSONEncode(mapVal)
	if err != nil || encodedMap != `{"hello":"world"}` {
		t.Errorf("JSONEncode(map) failed: %v, got %q", err, encodedMap)
	}

	// JSON Parse Dynamic
	decodedVal, err := JSONParse(nil, `{"a": 1, "b": "two"}`)
	if err != nil {
		t.Fatalf("JSONParse failed: %v", err)
	}
	if mapper, ok := decodedVal.(traits.Mapper); !ok || mapper.Size().(types.Int) != 2 {
		t.Errorf("unexpected decodedVal: %v", decodedVal)
	}

	// JSON Parse Typed
	typedVal, err := JSONParseWithType(nil, nil, `[1, 2, 3]`, types.NewListType(types.IntType))
	if err != nil {
		t.Fatalf("JSONParseWithType failed: %v", err)
	}
	if lister, ok := typedVal.(traits.Lister); !ok || lister.Size().(types.Int) != 3 {
		t.Errorf("unexpected typedVal: %v", typedVal)
	}

	// JSON Parse with Proto
	reg, err := types.NewRegistry(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("types.NewRegistry failed: %v", err)
	}
	protoVal, err := JSONParseWithType(nil, reg, `{"single_int32": 42}`, types.NewObjectType("google.expr.proto3.test.TestAllTypes"))
	if err != nil {
		t.Fatalf("JSONParseWithType proto failed: %v", err)
	}
	if fieldVal := protoVal.(traits.Indexer).Get(types.String("single_int32")); fieldVal != types.Int(42) {
		t.Errorf("got %v, wanted 42", fieldVal)
	}
	pbMsgAny, err := protoVal.ConvertToNative(reflect.TypeFor[*proto3pb.TestAllTypes]())
	if err != nil {
		t.Fatalf("ConvertToNative failed: %v", err)
	}
	if pbMsg, ok := pbMsgAny.(*proto3pb.TestAllTypes); !ok || pbMsg.GetSingleInt32() != 42 {
		t.Errorf("unexpected pbMsg: %v", pbMsgAny)
	}

	// Max size error checks
	hugeStr := strings.Repeat("x", maxJSONSize+1)
	if _, err := JSONParse(nil, hugeStr); err == nil {
		t.Errorf("expected error on huge string in JSONParse, got nil")
	}
	if _, err := JSONParseWithType(nil, nil, hugeStr, types.IntType); err == nil {
		t.Errorf("expected error on huge string in JSONParseWithType, got nil")
	}

	// Double NaN and Inf encoding errors
	if _, err := JSONEncode(types.Double(math.NaN())); err == nil {
		t.Errorf("expected error encoding NaN, got nil")
	}
	if _, err := JSONEncode(types.Double(math.Inf(1))); err == nil {
		t.Errorf("expected error encoding +Inf, got nil")
	}

	// Map with non-string keys
	intKeyMapVal := types.DefaultTypeAdapter.NativeToValue(map[int]string{1: "one"})
	if enc, err := JSONEncode(intKeyMapVal); err != nil || enc != `{"1":"one"}` {
		t.Errorf("JSONEncode int key map failed: %v, got %q", err, enc)
	}

	// Base64 invalid decode
	if _, err := Base64Decode("!!!"); err == nil {
		t.Errorf("expected Base64Decode error on invalid input, got nil")
	}

	// Unmarshal native struct errors
	if _, ok := jsonUnmarshalNative("{}", reflect.TypeOf(123)); ok {
		t.Errorf("expected jsonUnmarshalNative to fail on non-struct int type")
	}
	if _, ok := jsonUnmarshalNative("invalid", reflect.TypeOf(testNativeUser{})); ok {
		t.Errorf("expected jsonUnmarshalNative to fail on invalid JSON")
	}
}

func TestEncodersTypedCollections(t *testing.T) {
	// Typed Lists via JSONParseWithType
	if val, err := JSONParseWithType(nil, nil, `[1, 2]`, types.NewListType(types.UintType)); err != nil || val.Value().([]uint64)[0] != 1 {
		t.Errorf("uint list parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `[1.5, 2.5]`, types.NewListType(types.DoubleType)); err != nil || val.Value().([]float64)[0] != 1.5 {
		t.Errorf("double list parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `[true, false]`, types.NewListType(types.BoolType)); err != nil || val.Value().([]bool)[0] != true {
		t.Errorf("bool list parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `["a", "b"]`, types.NewListType(types.StringType)); err != nil || val.Value().([]string)[0] != "a" {
		t.Errorf("string list parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `[1, "two"]`, types.NewListType(types.DynType)); err != nil || len(val.Value().([]any)) != 2 {
		t.Errorf("dyn list parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, nil, `["bad"]`, types.NewListType(types.UintType)); err == nil {
		t.Errorf("expected uint list to fail on string element")
	}
	if _, err := JSONParseWithType(nil, nil, `["bad"]`, types.NewListType(types.DoubleType)); err == nil {
		t.Errorf("expected double list to fail on string element")
	}
	if _, err := JSONParseWithType(nil, nil, `["bad"]`, types.NewListType(types.BoolType)); err == nil {
		t.Errorf("expected bool list to fail on string element")
	}
	if _, err := JSONParseWithType(nil, nil, `["bad"]`, types.NewListType(types.IntType)); err == nil {
		t.Errorf("expected int list to fail on string element")
	}
	if _, err := JSONParseWithType(nil, nil, `invalid`, types.NewListType(types.DynType)); err == nil {
		t.Errorf("expected dyn list to fail on invalid JSON")
	}

	// Typed Maps via JSONParseWithType
	if val, err := JSONParseWithType(nil, nil, `{"a": 1, "b": 2}`, types.NewMapType(types.StringType, types.UintType)); err != nil || val.Value().(map[string]uint64)["a"] != 1 {
		t.Errorf("uint map parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `{"a": 1.5}`, types.NewMapType(types.StringType, types.DoubleType)); err != nil || val.Value().(map[string]float64)["a"] != 1.5 {
		t.Errorf("double map parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `{"a": true}`, types.NewMapType(types.StringType, types.BoolType)); err != nil || val.Value().(map[string]bool)["a"] != true {
		t.Errorf("bool map parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `{"a": "b"}`, types.NewMapType(types.StringType, types.StringType)); err != nil || val.Value().(map[string]string)["a"] != "b" {
		t.Errorf("string map parse failed: %v", err)
	}
	if val, err := JSONParseWithType(nil, nil, `{"a": 1}`, types.NewMapType(types.StringType, types.DynType)); err != nil || val.Value().(map[string]any)["a"] == nil {
		t.Errorf("dyn map parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, nil, `{"a": "bad"}`, types.NewMapType(types.StringType, types.UintType)); err == nil {
		t.Errorf("expected uint map to fail on string value")
	}
	if _, err := JSONParseWithType(nil, nil, `{"a": "bad"}`, types.NewMapType(types.StringType, types.DoubleType)); err == nil {
		t.Errorf("expected double map to fail on string value")
	}
	if _, err := JSONParseWithType(nil, nil, `{"a": "bad"}`, types.NewMapType(types.StringType, types.BoolType)); err == nil {
		t.Errorf("expected bool map to fail on string value")
	}
	if _, err := JSONParseWithType(nil, nil, `{"a": "bad"}`, types.NewMapType(types.StringType, types.IntType)); err == nil {
		t.Errorf("expected int map to fail on string value")
	}
	if _, err := JSONParseWithType(nil, nil, `invalid`, types.NewMapType(types.StringType, types.DynType)); err == nil {
		t.Errorf("expected dyn map to fail on invalid JSON")
	}

	// Proto Collections
	reg, _ := types.NewRegistry(&proto3pb.TestAllTypes{})
	if val, err := JSONParseWithType(nil, reg, `[{"single_int32": 42}]`, types.NewListType(types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err != nil || len(val.Value().([]any)) != 1 {
		t.Errorf("proto list parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, reg, `[{"single_int32": "bad"}]`, types.NewListType(types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err == nil {
		t.Errorf("expected proto list to fail on bad field type")
	}
	if _, err := JSONParseWithType(nil, reg, `invalid`, types.NewListType(types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err == nil {
		t.Errorf("expected proto list to fail on invalid JSON")
	}
	if val, err := JSONParseWithType(nil, reg, `{"a": {"single_int32": 42}}`, types.NewMapType(types.StringType, types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err != nil || val.Value().(map[string]any)["a"] == nil {
		t.Errorf("proto map parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, reg, `{"a": {"single_int32": "bad"}}`, types.NewMapType(types.StringType, types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err == nil {
		t.Errorf("expected proto map to fail on bad field type")
	}
	if _, err := JSONParseWithType(nil, reg, `invalid`, types.NewMapType(types.StringType, types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err == nil {
		t.Errorf("expected proto map to fail on invalid JSON")
	}

	// Well-known types via JSONParseWithType
	if _, err := JSONParseWithType(nil, nil, `"hello"`, types.NewObjectType("google.protobuf.Value")); err != nil {
		t.Errorf("Value parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, nil, `{"k": "v"}`, types.NewObjectType("google.protobuf.Struct")); err != nil {
		t.Errorf("Struct parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, nil, `[1, 2]`, types.NewObjectType("google.protobuf.ListValue")); err != nil {
		t.Errorf("ListValue parse failed: %v", err)
	}
	if _, err := JSONParseWithType(nil, nil, `{"bad":}`, types.NewObjectType("google.protobuf.Value")); err == nil {
		t.Errorf("expected Value parse to fail on malformed JSON")
	}
	if _, err := JSONParseWithType(nil, nil, `{"bad":}`, types.NewObjectType("google.protobuf.Struct")); err == nil {
		t.Errorf("expected Struct parse to fail on malformed JSON")
	}
	if _, err := JSONParseWithType(nil, nil, `{"bad":}`, types.NewObjectType("google.protobuf.ListValue")); err == nil {
		t.Errorf("expected ListValue parse to fail on malformed JSON")
	}
}

func TestEncodersEstimatorsAndEdgeCases(t *testing.T) {
	// Test estimator invalid argument guards
	if estimateEncode(nil, nil, []checker.AstNode{}) != nil {
		t.Errorf("expected nil from estimateEncode with empty args")
	}
	if estimateDecode(nil, nil, []checker.AstNode{}) != nil {
		t.Errorf("expected nil from estimateDecode with empty args")
	}
	if estimateJSONEncode(nil, nil, []checker.AstNode{}) != nil {
		t.Errorf("expected nil from estimateJSONEncode with empty args")
	}
	if estimateJSONParse(nil, nil, []checker.AstNode{}) != nil {
		t.Errorf("expected nil from estimateJSONParse with empty args")
	}
	if estimateJSONParse(nil, nil, []checker.AstNode{nil, nil, nil}) != nil {
		t.Errorf("expected nil from estimateJSONParse with 3 args")
	}

	// Test estimateEncodeSize overflow guard
	overflowSz := estimateEncodeSize(checker.SizeEstimate{Min: 0, Max: math.MaxUint64})
	if overflowSz.Max != math.MaxUint64 {
		t.Errorf("expected MaxUint64, got %v", overflowSz.Max)
	}

	// Test jsonParsePrimitive errors
	if _, ok := jsonParsePrimitive("123 trailing", func(t any) (any, bool) { return t, true }); ok {
		t.Errorf("expected jsonParsePrimitive to fail with trailing data")
	}
	if _, ok := jsonParsePrimitive("invalid", func(t any) (any, bool) { return t, true }); ok {
		t.Errorf("expected jsonParsePrimitive to fail with invalid token")
	}
	if _, ok := jsonParsePrimitive("123", func(t any) (any, bool) { return nil, false }); ok {
		t.Errorf("expected jsonParsePrimitive to fail when convert returns false")
	}

	// Test jsonParseDynamic uint overflow / numbers
	if num, ok := jsonParseDynamic("18446744073709551615"); !ok || num != json.Number("18446744073709551615") {
		t.Errorf("expected uint64 number, got %v, ok=%v", num, ok)
	}

	// Test jsonParseStructNative with invalid prototype
	if _, ok := jsonParseStructNative(nil, "{}", "non.existent.Type"); ok {
		t.Errorf("expected jsonParseStructNative to fail on non-existent type with nil provider")
	}

	// Test jsonParseStructNative with erroring provider
	errProvider := &mockErrorProvider{}
	if _, ok := jsonParseStructNative(errProvider, "{}", "error.Type"); ok {
		t.Errorf("expected jsonParseStructNative to fail on provider error")
	}
	nilProvider := &mockNilProvider{}
	if _, ok := jsonParseStructNative(nilProvider, "{}", "nil.Type"); ok {
		t.Errorf("expected jsonParseStructNative to fail on nil value from provider")
	}

	// Test jsonParseStructNative with cached pointer reflect type
	ptrTypeVal := &testNativeUser{Username: "Alice", Age: 30}
	structPrototypeCache.Store("ptr.NativeUser", structPrototype{refType: reflect.TypeOf(ptrTypeVal)})
	if val, ok := jsonParseStructNative(nil, `{"user_name":"Bob","age":25}`, "ptr.NativeUser"); !ok || val == nil {
		t.Errorf("expected jsonParseStructNative to parse cached ptr reflect type")
	}

	// Test direct jsonUnmarshalNative with pointer to struct
	if val, ok := jsonUnmarshalNative(`{"user_name":"Carol","age":35}`, reflect.TypeOf(&testNativeUser{})); !ok || val == nil {
		t.Errorf("expected jsonUnmarshalNative with pointer to succeed")
	}
	if _, ok := jsonUnmarshalNative(`invalid`, reflect.TypeOf(&testNativeUser{})); ok {
		t.Errorf("expected jsonUnmarshalNative with pointer to fail on invalid JSON")
	}

	// Test direct jsonUnmarshalProto
	if msg, ok := jsonUnmarshalProto(`{"single_int32": 99}`, &proto3pb.TestAllTypes{}); !ok || msg == nil {
		t.Errorf("expected jsonUnmarshalProto to succeed")
	}
	if _, ok := jsonUnmarshalProto(`{"single_int32": "bad"}`, &proto3pb.TestAllTypes{}); ok {
		t.Errorf("expected jsonUnmarshalProto to fail on invalid field")
	}

	// Test direct jsonEncodeNativeStruct with nil, proto, pointer, and non-struct
	if _, ok := jsonEncodeNativeStruct(nil); ok {
		t.Errorf("expected jsonEncodeNativeStruct(nil) to return false")
	}
	if _, ok := jsonEncodeNativeStruct(&proto3pb.TestAllTypes{}); ok {
		t.Errorf("expected jsonEncodeNativeStruct(proto) to return false")
	}
	if enc, ok := jsonEncodeNativeStruct(&testNativeUser{Username: "Dave", Age: 40}); !ok || enc != `{"user_name":"Dave","age":40}` {
		t.Errorf("expected jsonEncodeNativeStruct(pointer) to succeed, got %q, ok=%v", enc, ok)
	}
	if _, ok := jsonEncodeNativeStruct(123); ok {
		t.Errorf("expected jsonEncodeNativeStruct(int) to return false")
	}

	// Test direct getProtoMessage
	if msg, ok := getProtoMessage(mockProtoVal{TestAllTypes: &proto3pb.TestAllTypes{SingleInt32: 42}}); !ok || msg == nil {
		t.Errorf("expected getProtoMessage(proto.Message) to succeed")
	}
	if _, ok := getProtoMessage(types.Int(42)); ok {
		t.Errorf("expected getProtoMessage(types.Int) to return false")
	}

	// Test jsonEncodeList and jsonEncodeMap error paths
	invalidList := mockInvalidList{}
	if _, err := jsonEncodeList(invalidList); err == nil {
		t.Errorf("expected jsonEncodeList to fail on non-int size")
	}
	elemErrList := types.DefaultTypeAdapter.NativeToValue([]any{math.NaN()}).(traits.Lister)
	if _, err := jsonEncodeList(elemErrList); err == nil {
		t.Errorf("expected jsonEncodeList to fail on NaN element")
	}

	invalidMap := mockInvalidMap{}
	if _, err := jsonEncodeMap(invalidMap); err == nil {
		t.Errorf("expected jsonEncodeMap to fail on non-int size")
	}
	valErrMap := types.DefaultTypeAdapter.NativeToValue(map[string]any{"a": math.NaN()}).(traits.Mapper)
	if _, err := jsonEncodeMap(valErrMap); err == nil {
		t.Errorf("expected jsonEncodeMap to fail on NaN value")
	}

	// Test jsonEncodeValue error from ConvertToNative
	mockErr := &mockErrVal{}
	if _, err := jsonEncodeValue(mockErr); err == nil {
		t.Errorf("expected jsonEncodeValue to fail on ConvertToNative error")
	}

	// Test jsonParseWithType string limit and scalar types
	hugeStr := strings.Repeat("a", maxJSONSize+1)
	if res := jsonParseWithType(nil, nil, hugeStr, types.IntType); !types.IsError(res) {
		t.Errorf("expected error from jsonParseWithType on oversized string")
	}
	if _, ok := jsonParseValue(nil, nil, `"not-bytes"`, types.BytesType); ok {
		t.Errorf("expected jsonParseValue to fail on invalid bytes")
	}
	if _, ok := jsonParseValue(nil, nil, `"not-timestamp"`, types.TimestampType); ok {
		t.Errorf("expected jsonParseValue to fail on invalid timestamp")
	}
	if _, ok := jsonParseValue(nil, nil, `"not-duration"`, types.DurationType); ok {
		t.Errorf("expected jsonParseValue to fail on invalid duration")
	}
	if _, ok := jsonParseValue(nil, nil, `123`, types.StringType); ok {
		t.Errorf("expected jsonParseValue to fail on string parse of number token")
	}
	if _, ok := jsonParseValue(nil, nil, `123`, types.BytesType); ok {
		t.Errorf("expected jsonParseValue to fail on bytes parse of number token")
	}
	if _, ok := jsonParseValue(nil, nil, `123`, types.TimestampType); ok {
		t.Errorf("expected jsonParseValue to fail on timestamp parse of number token")
	}
	if _, ok := jsonParseValue(nil, nil, `123`, types.DurationType); ok {
		t.Errorf("expected jsonParseValue to fail on duration parse of number token")
	}

	// Test jsonUnmarshalExact with trailing data
	var trailingObj any
	if err := jsonUnmarshalExact("123 trailing", &trailingObj); err == nil {
		t.Errorf("expected jsonUnmarshalExact to fail on trailing token")
	}

	// Test jsonEncodeMap key error
	keyErrMap := mockKeyErrMap{}
	if _, err := jsonEncodeMap(keyErrMap); err == nil {
		t.Errorf("expected jsonEncodeMap to fail on key error")
	}

	// Test jsonParseValue with ListType and MapType singletons (no parameters)
	if val, ok := jsonParseValue(nil, nil, `[1, "two"]`, types.ListType); !ok || val == nil {
		t.Errorf("expected jsonParseValue to succeed for unparameterized ListType")
	}
	if _, ok := jsonParseValue(nil, nil, `invalid`, types.ListType); ok {
		t.Errorf("expected jsonParseValue to fail on invalid json for unparameterized ListType")
	}
	if val, ok := jsonParseValue(nil, nil, `{"a": 1}`, types.MapType); !ok || val == nil {
		t.Errorf("expected jsonParseValue to succeed for unparameterized MapType")
	}
	if _, ok := jsonParseValue(nil, nil, `invalid`, types.MapType); ok {
		t.Errorf("expected jsonParseValue to fail on invalid json for unparameterized MapType")
	}

	// Test jsonParseValue with NullType and TypeType
	if _, ok := jsonParseValue(nil, nil, `"not-null"`, types.NullType); ok {
		t.Errorf("expected jsonParseValue to fail on non-null for NullType")
	}
	if _, ok := jsonParseValue(nil, nil, `"not-type"`, types.TypeType); ok {
		t.Errorf("expected jsonParseValue to fail on non-type for TypeType")
	}

	// Test jsonParseValue with custom StructTypeDescriptor and NonType
	if val, ok := jsonParseValue(nil, nil, `{"user_name":"Eve","age":20}`, mockStructTypeDesc{reflectType: reflect.TypeOf(testNativeUser{})}); !ok || val == nil {
		t.Errorf("expected jsonParseValue to succeed for StructTypeDescriptor")
	}
	if val, ok := jsonParseValue(nil, nil, `{"user_name":"Eve","age":20}`, mockCustomTypeDesc{typeName: "ptr.NativeUser"}); !ok || val == nil {
		t.Errorf("expected jsonParseValue to succeed for custom TypeName")
	}
	if _, ok := jsonParseValue(nil, nil, `{}`, mockCustomNonType{}); ok {
		t.Errorf("expected jsonParseValue to fail for mockCustomNonType")
	}

	// Test jsonParseTypedList & jsonParseTypedMap nested custom type failures
	if _, ok := jsonParseTypedList(nil, nil, `[{"bad":}]`, types.NewObjectType("non.existent")); ok {
		t.Errorf("expected jsonParseTypedList to fail on nested custom type with bad json")
	}
	if _, ok := jsonParseTypedMap(nil, nil, `{"a": {"bad":}}`, types.NewObjectType("non.existent")); ok {
		t.Errorf("expected jsonParseTypedMap to fail on nested custom type with bad json")
	}

	// Test jsonParseStructNative well-known types
	if val, ok := jsonParseStructNative(nil, `"hello"`, "google.protobuf.Value"); !ok || val == nil {
		t.Errorf("expected jsonParseStructNative(Value) to succeed")
	}
	if val, ok := jsonParseStructNative(nil, `{"k":"v"}`, "google.protobuf.Struct"); !ok || val == nil {
		t.Errorf("expected jsonParseStructNative(Struct) to succeed")
	}
	if val, ok := jsonParseStructNative(nil, `[1, 2]`, "google.protobuf.ListValue"); !ok || val == nil {
		t.Errorf("expected jsonParseStructNative(ListValue) to succeed")
	}

	// Test CompileOptions defaults and binary overload invalid typeVal
	lib := &encoderLib{version: 2}
	opts := lib.CompileOptions()
	if len(opts) == 0 {
		t.Errorf("expected CompileOptions to return options")
	}
	env, _ := cel.NewEnv(append([]cel.EnvOption{cel.OptionalTypes()}, opts...)...)
	ast, _ := env.Compile(`json.parse("123")`)
	prg, _ := env.Program(ast)
	if _, _, err := prg.Eval(cel.NoVars()); err != nil {
		t.Fatalf("prg.Eval failed: %v", err)
	}

	// Test jsonParseWithType directly
	if res := jsonParseWithType(nil, nil, "123", types.IntType); res.Value() != int64(123) {
		t.Errorf("expected 123, got %v", res)
	}

	// Test JSONParse invalid JSON
	if _, err := JSONParse(nil, `{"bad":}`); err == nil {
		t.Errorf("expected JSONParse error on invalid JSON")
	}

	// Test Base64Decode padding error
	if _, err := Base64Decode("=aGVsbG8="); err == nil {
		t.Errorf("expected Base64Decode error on padding error")
	}

	// Test string and bytes invalid collections
	if _, err := JSONParseWithType(nil, nil, "invalid", types.NewListType(types.StringType)); err == nil {
		t.Errorf("expected string list parse to fail on invalid JSON")
	}
	if _, err := JSONParseWithType(nil, nil, "invalid", types.NewMapType(types.StringType, types.StringType)); err == nil {
		t.Errorf("expected string map parse to fail on invalid JSON")
	}

	// Test proto message with invalid UTF8 string marshal error
	invalidProto := &proto3pb.TestAllTypes{SingleString: "\xff\xff"}
	if _, err := jsonEncodeValue(mockProtoVal{TestAllTypes: invalidProto}); err == nil {
		t.Errorf("expected jsonEncodeValue error on proto with invalid utf8")
	}
	if _, err := jsonEncodeValue(types.DefaultTypeAdapter.NativeToValue(invalidProto)); err == nil {
		t.Errorf("expected jsonEncodeValue error on proto with invalid utf8")
	}

	// Test ConvertToNative returning non-structpb.Value and invalid UTF-8 Value
	if _, err := jsonEncodeValue(mockNonStructPBVal{}); err == nil {
		t.Errorf("expected jsonEncodeValue error on non-structpb.Value")
	}
	if _, err := jsonEncodeValue(mockInvalidUTF8PBVal{}); err == nil {
		t.Errorf("expected jsonEncodeValue error on invalid UTF-8 Value")
	}

	// Test int typed collections
	if val, err := JSONParseWithType(nil, nil, `[1, 2]`, types.NewListType(types.IntType)); err != nil || len(val.Value().([]int64)) != 2 {
		t.Errorf("expected int list to parse successfully")
	}
	if val, err := JSONParseWithType(nil, nil, `{"a": 1}`, types.NewMapType(types.StringType, types.IntType)); err != nil || val.Value().(map[string]int64)["a"] != 1 {
		t.Errorf("expected int map to parse successfully")
	}
	if _, err := JSONParseWithType(nil, nil, `123`, types.NewMapType(types.StringType, types.NewObjectType("google.expr.proto3.test.TestAllTypes"))); err == nil {
		t.Errorf("expected proto map parse to fail on non-map JSON")
	}
}

type mockNonStructPBVal struct {
	ref.Val
}

func (mockNonStructPBVal) Type() ref.Type {
	return types.NewObjectType("mock.NonStructPBVal")
}

func (mockNonStructPBVal) Value() any {
	return nil
}

func (mockNonStructPBVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return "not-structpb-value", nil
}

type mockInvalidUTF8PBVal struct {
	ref.Val
}

func (mockInvalidUTF8PBVal) Type() ref.Type {
	return types.NewObjectType("mock.InvalidUTF8PBVal")
}

func (mockInvalidUTF8PBVal) Value() any {
	return nil
}

func (mockInvalidUTF8PBVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "\xff\xff"}}, nil
}

type mockInvalidList struct {
	traits.Lister
}

func (mockInvalidList) Size() ref.Val {
	return types.String("invalid-size")
}

type mockInvalidMap struct {
	traits.Mapper
}

func (mockInvalidMap) Size() ref.Val {
	return types.String("invalid-size")
}

type mockKeyErrMap struct {
	traits.Mapper
}

func (mockKeyErrMap) Size() ref.Val {
	return types.Int(1)
}

func (mockKeyErrMap) Iterator() traits.Iterator {
	return &mockKeyErrIterator{first: true}
}

type mockKeyErrIterator struct {
	traits.Iterator
	first bool
}

func (m *mockKeyErrIterator) HasNext() ref.Val {
	if m.first {
		m.first = false
		return types.True
	}
	return types.False
}

func (m *mockKeyErrIterator) Next() ref.Val {
	return types.Double(math.NaN())
}

type mockStructTypeDesc struct {
	types.StructTypeDescriptor
	reflectType reflect.Type
}

func (m mockStructTypeDesc) ReflectType() reflect.Type {
	return m.reflectType
}

func (m mockStructTypeDesc) TypeName() string {
	return "mock.StructType"
}

func (m mockStructTypeDesc) HasTrait(trait int) bool {
	return false
}

type mockCustomTypeDesc struct {
	ref.Type
	typeName string
}

func (m mockCustomTypeDesc) TypeName() string {
	return m.typeName
}

type mockCustomNonType struct {
	ref.Type
}

func (mockCustomNonType) HasTrait(trait int) bool {
	return false
}

func (mockCustomNonType) TypeName() string {
	return "mock.CustomNonType"
}

type mockErrVal struct {
	ref.Val
}

func (mockErrVal) Type() ref.Type {
	return types.NewObjectType("mock.ErrVal")
}

func (mockErrVal) Value() any {
	return nil
}

func (mockErrVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, fmt.Errorf("mock ConvertToNative error")
}

type mockErrorProvider struct {
	types.Provider
}

func (mockErrorProvider) NewValue(typeName string, fields map[string]ref.Val) ref.Val {
	return types.NewErr("mock provider error")
}

type mockNilProvider struct {
	types.Provider
}

type mockNilVal struct {
	ref.Val
}

func (mockNilVal) Value() any {
	return nil
}

func (mockNilProvider) NewValue(typeName string, fields map[string]ref.Val) ref.Val {
	return mockNilVal{}
}

type mockProtoVal struct {
	ref.Val
	*proto3pb.TestAllTypes
}
