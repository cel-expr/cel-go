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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
)

func TestEncoders(t *testing.T) {
	var tests = []struct {
		expr      string
		err       string
		parseOnly bool
	}{
		{expr: "base64.decode('aGVsbG8=') == b'hello'"},
		{expr: "base64.decode('aGVsbG8') == b'hello'"},
		{
			expr:      "base64.decode(b'aGVsbG8=') == b'hello'",
			err:       "no such overload",
			parseOnly: true,
		},
		{expr: "base64.encode(b'hello') == 'aGVsbG8='"},
		{
			expr:      "base64.encode('hello') == b'aGVsbG8='",
			err:       "no such overload",
			parseOnly: true,
		},
	}

	env, err := cel.NewEnv(Encoders())
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
					t.Fatal(err)
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
					t.Fatal(err)
				} else if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestEncodersVersion(t *testing.T) {
	_, err := cel.NewEnv(Encoders(EncodersVersion(0)))
	if err != nil {
		t.Fatalf("EncodersVersion(0) failed: %v", err)
	}
}

func testEncodersCostsEnv(t *testing.T, version int, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{
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
