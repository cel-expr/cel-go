// Copyright 2022 Google LLC
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
	"github.com/google/cel-go/common/types"
)

func TestMath(t *testing.T) {
	mathTests := []struct {
		expr string
		in   any
	}{
		// Tests for math.least
		{expr: "math.least(-0.5) == -0.5"},
		{expr: "math.least(-1) == -1"},
		{expr: "math.least(1u) == 1u"},
		{expr: "math.least(42.0, -0.5) == -0.5"},
		{expr: "math.least(-1, 0) == -1"},
		{expr: "math.least(-1, -1) == -1"},
		{expr: "math.least(1u, 42u) == 1u"},
		{expr: "math.least(42.0, -0.5, -0.25) == -0.5"},
		{expr: "math.least(-1, 0, 1) == -1"},
		{expr: "math.least(-1, -1, -1) == -1"},
		{expr: "math.least(1u, 42u, 0u) == 0u"},
		// math.least two arg overloads across type.
		{expr: "math.least(1, 1.0) == 1"},
		{expr: "math.least(1, -2.0) == -2.0"},
		{expr: "math.least(2, 1u) == 1u"},
		{expr: "math.least(1.5, 2) == 1.5"},
		{expr: "math.least(1.5, -2) == -2"},
		{expr: "math.least(2.5, 1u) == 1u"},
		{expr: "math.least(1u, 2) == 1u"},
		{expr: "math.least(1u, -2) == -2"},
		{expr: "math.least(2u, 2.5) == 2u"},
		// math.least with dynamic values across type.
		{expr: "math.least(1u, dyn(42)) == 1"},
		{expr: "math.least(1u, dyn(42), dyn(0.0)) == 0u"},
		// math.least with a list literal.
		{expr: "math.least([1u, 42u, 0u]) == 0u"},
		// math.least with expression arguments.
		{
			expr: "math.least(a, b) == a",
			in: map[string]any{
				"a": 1,
				"b": 2,
			},
		},
		{
			expr: "math.least(numbers) == dyn(a)",
			in: map[string]any{
				"a":       -21,
				"numbers": []float64{-21.0, -10.5, 1.0},
			},
		},

		// Tests for math.greatest
		{expr: "math.greatest(-0.5) == -0.5"},
		{expr: "math.greatest(-1) == -1"},
		{expr: "math.greatest(1u) == 1u"},
		{expr: "math.greatest(42.0, -0.5) == 42.0"},
		{expr: "math.greatest(-1, 0) == 0"},
		{expr: "math.greatest(-1, -1) == -1"},
		{expr: "math.greatest(1u, 42u) == 42u"},
		{expr: "math.greatest(42.0, -0.5, -0.25) == 42.0"},
		{expr: "math.greatest(-1, 0, 1) == 1"},
		{expr: "math.greatest(-1, -1, -1) == -1"},
		{expr: "math.greatest(1u, 42u, 0u) == 42u"},
		// math.greatest two arg overloads across type.
		{expr: "math.greatest(1, 1.0) == 1"},
		{expr: "math.greatest(1, -2.0) == 1"},
		{expr: "math.greatest(2, 1u) == 2"},
		{expr: "math.greatest(1.5, 2) == 2"},
		{expr: "math.greatest(1.5, -2) == 1.5"},
		{expr: "math.greatest(2.5, 1u) == 2.5"},
		{expr: "math.greatest(1u, 2) == 2"},
		{expr: "math.greatest(1u, -2) == 1u"},
		{expr: "math.greatest(2u, 2.5) == 2.5"},
		// math.greatest with dynamic values across type.
		{expr: "math.greatest(1u, dyn(42)) == 42.0"},
		{expr: "math.greatest(1u, dyn(0.0), 0u) == 1"},
		// math.greatest with a list literal
		{expr: "math.greatest([1u, dyn(0.0), 0u]) == 1"},
		// math.greatest with expression arguments.
		{
			expr: "math.greatest(a, b) == b",
			in: map[string]any{
				"a": 1,
				"b": 2,
			},
		},
		{
			expr: "math.greatest(numbers) == dyn(a)",
			in: map[string]any{
				"a":       1,
				"numbers": []float64{-21.0, -10.5, 1.0},
			},
		},

		// Tests for math bitwise operators
		// Signed bitwise ops
		{expr: "math.bitAnd(1, 2) == 0"},
		{expr: "math.bitAnd(1, -1) == 1"},
		{expr: "math.bitAnd(1, 3) == 1"},
		{expr: "math.bitOr(1, 2) == 3"},
		{expr: "math.bitXor(1, 3) == 2"},
		{expr: "math.bitXor(3, 5) == 6"},
		{expr: "math.bitNot(1) == -2"},
		{expr: "math.bitNot(0) == -1"},
		{expr: "math.bitNot(-1) == 0"},
		{expr: "math.bitShiftLeft(1, 2) == 4"},
		{expr: "math.bitShiftLeft(1, 200) == 0"},
		{expr: "math.bitShiftLeft(-1, 200) == 0"},
		{expr: "math.bitShiftRight(1024, 2) == 256"},
		{expr: "math.bitShiftRight(1024, 64) == 0"},
		{expr: "math.bitShiftRight(-1024, 3) == 2305843009213693824"},
		{expr: "math.bitShiftRight(-1024, 64) == 0"},
		// Unsigned bitwise ops
		{expr: "math.bitAnd(1u, 2u) == 0u"},
		{expr: "math.bitAnd(1u, 3u) == 1u"},
		{expr: "math.bitOr(1u, 2u) == 3u"},
		{expr: "math.bitXor(1u, 3u) == 2u"},
		{expr: "math.bitXor(3u, 5u) == 6u"},
		{expr: "math.bitNot(1u) == 18446744073709551614u"},
		{expr: "math.bitNot(0u) == 18446744073709551615u"},
		{expr: "math.bitShiftLeft(1u, 2) == 4u"},
		{expr: "math.bitShiftLeft(1u, 200) == 0u"},
		{expr: "math.bitShiftRight(1024u, 2) == 256u"},
		{expr: "math.bitShiftRight(1024u, 64) == 0u"},

		// Tests for floating point helpers
		{expr: "math.isNaN(0.0/0.0)"},
		{expr: "!math.isNaN(1.0/0.0)"},
		{expr: "math.isFinite(1.0/1.5)"},
		{expr: "!math.isFinite(1.0/0.0)"},
		{expr: "math.isInf(1.0/0.0)"},

		// Tests for rounding functions
		{expr: "math.ceil(1.2) == 2.0"},
		{expr: "math.ceil(-1.2) == -1.0"},
		{expr: "math.floor(1.2) == 1.0"},
		{expr: "math.floor(-1.2) == -2.0"},
		{expr: "math.round(1.2) == 1.0"},
		{expr: "math.round(1.5) == 2.0"},
		{expr: "math.round(-1.5) == -2.0"},
		{expr: "math.isNaN(math.round(0.0/0.0))"},
		{expr: "math.round(-1.2) == -1.0"},
		{expr: "math.trunc(-1.3) == -1.0"},
		{expr: "math.trunc(1.3) == 1.0"},

		// Tests for signedness related functions
		{expr: "math.sign(-42) == -1"},
		{expr: "math.sign(0) == 0"},
		{expr: "math.sign(42) == 1"},
		{expr: "math.sign(0u) == 0u"},
		{expr: "math.sign(42u) == 1u"},
		{expr: "math.sign(-0.3) == -1.0"},
		{expr: "math.sign(0.0) == 0.0"},
		{expr: "math.isNaN(math.sign(0.0/0.0))"},
		{expr: "math.sign(1.0/0.0) == 1.0"},
		{expr: "math.sign(-1.0/0.0) == -1.0"},
		{expr: "math.sign(0.3) == 1.0"},
		{expr: "math.abs(-1) == 1"},
		{expr: "math.abs(1) == 1"},
		{expr: "math.abs(-234.5) == 234.5"},
		{expr: "math.abs(234.5) == 234.5"},

		// Tests for Square root function
		{expr: "math.sqrt(49.0) == 7.0"},
		{expr: "math.sqrt(0) == 0.0"},
		{expr: "math.sqrt(1) == 1.0"},
		{expr: "math.sqrt(25u) == 5.0"},
		{expr: "math.sqrt(82) == 9.055385138137417"},
		{expr: "math.sqrt(985.25) == 31.388692231439016"},
		{expr: "math.isNaN(math.sqrt(-15.34))"},
	}

	env := testMathEnv(t,
		cel.Variable("a", cel.IntType),
		cel.Variable("b", cel.IntType),
		cel.Variable("numbers", cel.ListType(cel.DoubleType)),
	)
	for i, tst := range mathTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
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
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				in := tc.in
				if in == nil {
					in = cel.NoVars()
				}
				out, _, err := prg.Eval(in)
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out.Value() != true {
					t.Errorf("prg.Eval() got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestMathStaticErrors(t *testing.T) {
	mathTests := []struct {
		expr string
		err  string
	}{
		// Tests for math.least
		{
			expr: "math.least()",
			err:  "math.least() requires at least one argument",
		},
		{
			expr: "math.least('hello')",
			err:  "math.least() invalid single argument value",
		},
		{
			expr: "math.least({})",
			err:  "math.least() invalid single argument value",
		},
		{
			expr: "math.least(1, true)",
			err:  "math.least() simple literal arguments must be numeric",
		},
		{
			expr: "math.least(1, 2, true)",
			err:  "math.least() simple literal arguments must be numeric",
		},

		// Tests for math.greatest
		{
			expr: "math.greatest()",
			err:  "math.greatest() requires at least one argument",
		},
		{
			expr: "math.greatest(true)",
			err:  "math.greatest() invalid single argument value",
		},
		{
			expr: "math.greatest([])",
			err:  "math.greatest() invalid single argument value",
		},
		{
			expr: "math.greatest([1, true])",
			err:  "math.greatest() invalid single argument value",
		},
		{
			expr: "math.greatest(1, true)",
			err:  "math.greatest() simple literal arguments must be numeric",
		},
		{
			expr: "math.greatest(1, 2, true)",
			err:  "math.greatest() simple literal arguments must be numeric",
		},
	}

	env := testMathEnv(t)
	for i, tst := range mathTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if iss.Err() == nil || !strings.Contains(iss.Err().Error(), tc.err) {
				t.Errorf("env.Compile(%q) got %v, wanted error %v", tc.expr, iss.Err(), tc.err)
			}
		})
	}
}

func TestMathRuntimeErrors(t *testing.T) {
	mathTests := []struct {
		expr string
		err  string
		in   any
	}{
		// Tests for math.least
		{
			expr: "math.least(a, b)",
			err:  "no such overload: math.@min",
			in: map[string]any{
				"a": []int{},
				"b": 1,
			},
		},
		{
			expr: "math.least(b, a)",
			err:  "no such overload: math.@min",
			in: map[string]any{
				"a": []int{},
				"b": 1,
			},
		},
		{
			expr: "math.least(a)",
			err:  "math.@min(list) argument must not be empty",
			in: map[string]any{
				"a": []int{},
			},
		},
		{
			expr: "math.least(a)",
			err:  "no such overload: math.@min",
			in: map[string]any{
				"a": []any{"hello"},
			},
		},
		{
			expr: "math.least(a)",
			err:  "no such overload: math.@min",
			in: map[string]any{
				"a": []any{[]int{}, []int{}},
			},
		},
		{
			expr: "math.least(a)",
			err:  "no such overload: math.@min",
			in: map[string]any{
				"a": []any{1, true, 2},
			},
		},
		{
			expr: "math.least(dyn('string'))",
			err:  "no such overload: math.@min",
		},

		// Tests for math.greatest
		{
			expr: "math.greatest(a, b)",
			err:  "no such overload: math.@max",
			in: map[string]any{
				"a": []int{},
				"b": 1,
			},
		},
		{
			expr: "math.greatest(b, a)",
			err:  "no such overload: math.@max",
			in: map[string]any{
				"a": []int{},
				"b": 1,
			},
		},
		{
			expr: "math.greatest(a)",
			err:  "math.@max(list) argument must not be empty",
			in: map[string]any{
				"a": []int{},
			},
		},
		{
			expr: "math.greatest(a)",
			err:  "no such overload: math.@max",
			in: map[string]any{
				"a": []any{true},
			},
		},
		{
			expr: "math.greatest(a)",
			err:  "no such overload: math.@max",
			in: map[string]any{
				"a": []any{1, true, 2},
			},
		},
		{
			expr: "math.greatest(dyn('string'))",
			err:  "no such overload: math.@max",
		},
		{
			expr: "math.bitShiftLeft(1, -2) == 4",
			err:  "math.bitShiftLeft() negative offset",
		},
		{
			expr: "math.bitShiftLeft(1u, -2) == 0u",
			err:  "math.bitShiftLeft() negative offset",
		},
		{
			expr: "math.bitShiftRight(-1024, -3) == -128",
			err:  "math.bitShiftRight() negative offset",
		},
		{
			expr: "math.bitShiftRight(1024u, -4) == 1u",
			err:  "math.bitShiftRight() negative offset",
		},
		{
			expr: "math.abs(-9223372036854775808)",
			err:  "overflow",
		},
		{
			expr: "math.bitOr(dyn(1.2), 1)",
			err:  "no such overload: math.bitOr(double, int)",
		},
		{
			expr: "math.bitAnd(2u, dyn(''))",
			err:  "no such overload: math.bitAnd(uint, string)",
		},
		{
			expr: "math.bitXor(dyn(1), dyn(1u))",
			err:  "no such overload: math.bitXor(int, uint)",
		},
		{
			expr: "math.bitXor(dyn([]), dyn([1]))",
			err:  "no such overload: math.bitXor(list, list)",
		},
		{
			expr: "math.bitNot(dyn([1]))",
			err:  "no such overload: math.bitNot(list)",
		},
		{
			expr: "math.bitShiftLeft(dyn([1]), 1)",
			err:  "no such overload: math.bitShiftLeft(list, int)",
		},
		{
			expr: "math.bitShiftRight(dyn({}), 1)",
			err:  "no such overload: math.bitShiftRight(map, int)",
		},
		{
			expr: "math.isInf(dyn(1u))",
			err:  "no such overload: math.isInf(uint)",
		},
		{
			expr: "math.isFinite(dyn(1u))",
			err:  "no such overload: math.isFinite(uint)",
		},
		{
			expr: "math.isNaN(dyn(1u))",
			err:  "no such overload: math.isNaN(uint)",
		},
		{
			expr: "math.sign(dyn(''))",
			err:  "no such overload: math.sign(string)",
		},
		{
			expr: "math.abs(dyn(''))",
			err:  "no such overload: math.abs(string)",
		},
		{
			expr: "math.ceil(dyn(''))",
			err:  "no such overload: math.ceil(string)",
		},
		{
			expr: "math.floor(dyn(''))",
			err:  "no such overload: math.floor(string)",
		},
		{
			expr: "math.round(dyn(1))",
			err:  "no such overload: math.round(int)",
		},
		{
			expr: "math.trunc(dyn(1u))",
			err:  "no such overload: math.trunc(uint)",
		},
		{
			expr: "math.sqrt(dyn(''))",
			err:  "no such overload: math.sqrt(string)",
		},
	}

	env := testMathEnv(t,
		cel.Variable("a", cel.DynType),
		cel.Variable("b", cel.IntType),
		cel.Variable("numbers", cel.ListType(cel.DoubleType)),
	)
	for i, tst := range mathTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed with error %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program(ast) failed: %v", err)
			}
			in := tc.in
			if in == nil {
				in = cel.NoVars()
			}
			_, _, err = prg.Eval(in)
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.err)
			}
		})
	}
}

func TestMathNonMatch(t *testing.T) {
	var mathTests = []struct {
		expr string
	}{
		// Even though 'least' is the macro, the call is left unexpanded since the operand is not 'math'.
		{
			expr: `100.least(42) == 42`,
		},
		// Even though 'greatest' is the macro, the call is left unexpanded since the operand is not 'math'.
		{
			expr: `100.greatest(42) == 100`,
		},
	}
	env := testMathEnv(t,
		cel.Function("greatest",
			cel.MemberOverload("int_greatest_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(maxPair))),
		cel.Function("least",
			cel.MemberOverload("int_least_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(minPair))),
	)
	for i, tst := range mathTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
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
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out.Value() != true {
					t.Errorf("prg.Eval() got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestMathWithExtension(t *testing.T) {
	env := testMathEnv(t)
	_, err := env.Extend(Math())
	if err != nil {
		t.Fatalf("env.Extend(Math()) failed: %v", err)
	}
	_, iss := env.Compile("math.least(0, 1, 2) == 0")
	if iss.Err() != nil {
		t.Errorf("env.Compile() failed: %v", iss.Err())
	}
}

func TestMathVersions(t *testing.T) {
	versionCases := []struct {
		version            uint32
		supportedFunctions map[string]string
	}{
		{
			version: 0,
			supportedFunctions: map[string]string{
				"greatest": `math.greatest(1, 2) == 2`,
				"least":    `math.least(2.1, -1.0) == -1.0`,
			},
		},
		{
			version: 1,
			supportedFunctions: map[string]string{
				"ceil":          `math.ceil(1.5) == 2.0`,
				"floor":         `math.floor(1.2) == 1.0`,
				"round":         `math.round(1.5) == 2.0`,
				"trunc":         `math.trunc(1.222) == 1.0`,
				"isInf":         `!math.isInf(0.0)`,
				"isNaN":         `math.isNaN(0.0/0.0)`,
				"isFinite":      `math.isFinite(0.0)`,
				"abs":           `math.abs(1.2) == 1.2`,
				"sign":          `math.sign(-1) == -1`,
				"bitAnd":        `math.bitAnd(1, 2) == 0`,
				"bitOr":         `math.bitOr(1, 2) == 3`,
				"bitXor":        `math.bitXor(1, 3) == 2`,
				"bitNot":        `math.bitNot(-1) == 0`,
				"bitShiftLeft":  `math.bitShiftLeft(4, 2) == 16`,
				"bitShiftRight": `math.bitShiftRight(4, 2) == 1`,
			},
		},
		{
			version: 2,
			supportedFunctions: map[string]string{
				"sqrt":          `math.sqrt(25) == 5.0`,
			},
		},
	}
	for _, lib := range versionCases {
		env, err := cel.NewEnv(Math(MathVersion(lib.version)))
		if err != nil {
			t.Fatalf("cel.NewEnv(Math(MathVersion(%d))) failed: %v", lib.version, err)
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

func testMathEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{Math(), cel.EnableMacroCallTracking()}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Math()) failed: %v", err)
	}
	return env
}

func testMathCostsEnv(t *testing.T, version int, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	var mathOpt cel.EnvOption
	if version > 0 {
		mathOpt = Math(MathVersion(uint32(version)))
	} else {
		mathOpt = Math()
	}
	baseOpts := []cel.EnvOption{
		mathOpt,
		cel.EnableMacroCallTracking(),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Math()) failed: %v", err)
	}
	return env
}

func TestMathCosts(t *testing.T) {
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
			name: "math_greatest_list_v2",
			expr: "math.greatest(x) == 5",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
			},
			in: map[string]any{
				"x": []int64{1, 2, 3, 4, 5},
			},
			hints: map[string]uint64{
				"x": 10,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 3},
			actualCost:    3,
			version:       2,
		},
		{
			name: "math_greatest_list_v3",
			expr: "math.greatest(x) == 5",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
			},
			in: map[string]any{
				"x": []int64{1, 2, 3, 4, 5},
			},
			hints: map[string]uint64{
				"x": 10,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 13},
			actualCost:    8,
			version:       3,
		},
		{
			name: "math_least_list_v2",
			expr: "math.least(x) == -3.0",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.DoubleType)),
			},
			in: map[string]any{
				"x": []float64{-1.0, -2.0, -3.0},
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 3},
			actualCost:    3,
			version:       2,
		},
		{
			name: "math_least_list_v3",
			expr: "math.least(x) == -3.0",
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.DoubleType)),
			},
			in: map[string]any{
				"x": []float64{-1.0, -2.0, -3.0},
			},
			hints: map[string]uint64{
				"x": 100,
			},
			estimatedCost: checker.CostEstimate{Min: 3, Max: 103},
			actualCost:    6,
			version:       3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := testMathCostsEnv(t, tc.version, tc.vars...)
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
