// Copyright 2023 Google LLC
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
	"sync"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
)

var bindingTests = []struct {
	name          string
	expr          string
	vars          []cel.EnvOption
	in            map[string]any
	hints         map[string]uint64
	estimatedCost checker.CostEstimate
	actualCost    uint64
}{
	{
		name: "single bind",
		expr: `cel.bind(a, 'hell' + 'o' + '!', "%s, %s, %s".format([a, a, a])) ==
	                       'hello!, hello!, hello' + '!'`,
		estimatedCost: checker.CostEstimate{Min: 30, Max: 32},
		actualCost:    32,
	},
	{
		name: "multiple binds",
		expr: `cel.bind(a, 'hello!',
		       cel.bind(b, 'goodbye',
				a + ' and, ' + b)) == 'hello! and, goodbye'`,
		estimatedCost: checker.CostEstimate{Min: 27, Max: 28},
		actualCost:    28,
	},
	{
		name: "shadow binds",
		expr: `cel.bind(a,
		       cel.bind(a, 'world', a + '!'),
		   	    'hello ' + a) == 'hello ' + 'world' + '!'`,
		estimatedCost: checker.CostEstimate{Min: 30, Max: 31},
		actualCost:    31,
	},
	{
		name: "nested bind with int list",
		expr: `cel.bind(a, x,
			   cel.bind(b, a[0],
			   cel.bind(c, a[1], b + c))) == 10`,
		vars: []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
		in: map[string]any{
			"x": []int64{3, 7},
		},
		hints: map[string]uint64{
			"x": 3,
		},
		estimatedCost: checker.CostEstimate{Min: 39, Max: 39},
		actualCost:    39,
	},
	{
		name: "nested bind with string list",
		expr: `cel.bind(a, x,
			   cel.bind(b, a[0],
			   cel.bind(c, a[1], b + c))) == "threeseven"`,
		vars: []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
		in: map[string]any{
			"x": []string{"three", "seven"},
		},
		hints: map[string]uint64{
			"x":        3,
			"x.@items": 10,
		},
		estimatedCost: checker.CostEstimate{Min: 38, Max: 40},
		actualCost:    39,
	},
	{
		name: "shadowed binding",
		expr: `cel.bind(x, 0, x == 0)`,
		vars: []cel.EnvOption{cel.Variable("x", cel.StringType)},
		in: map[string]any{
			"cel.example.x": "1",
		},
		estimatedCost: checker.FixedCostEstimate(12),
		actualCost:    12,
	},
	{
		name: "container shadowed binding",
		expr: `cel.bind(x, 0, x == 0)`,
		vars: []cel.EnvOption{
			cel.Container("cel.example"),
			cel.Variable("cel.example.x", cel.StringType),
		},
		in: map[string]any{
			"cel.example.x": "1",
		},
		estimatedCost: checker.FixedCostEstimate(12),
		actualCost:    12,
	},
	{
		name: "shadowing namespace resolution selector",
		expr: `cel.bind(x, {'y': 0}, x.y == 0)`,
		vars: []cel.EnvOption{
			cel.Container("cel.example"),
			cel.Variable("cel.example.x.y", cel.IntType),
		},
		in: map[string]any{
			"cel.example.x.y": 1,
		},
		estimatedCost: checker.FixedCostEstimate(43),
		actualCost:    43,
	},
	{
		name: "shadowing namespace resolution selector with local",
		expr: `cel.bind(x, {'y': 0}, .x.y == x.y)`,
		vars: []cel.EnvOption{
			cel.Variable("x.y", cel.IntType),
		},
		in: map[string]any{
			"x.y": 0,
		},
		estimatedCost: checker.FixedCostEstimate(44),
		actualCost:    44,
	},
	{
		name: "namespace disambiguation",
		expr: `cel.bind(y, 0, .y != y)`,
		vars: []cel.EnvOption{
			cel.Variable("y", cel.IntType),
		},
		in: map[string]any{
			"y": 1,
		},
		estimatedCost: checker.FixedCostEstimate(13),
		actualCost:    13,
	},
	{
		name:          "nesting shadowing",
		expr:          `cel.bind(y, 0, cel.bind(y, 1, y != 0))`,
		estimatedCost: checker.FixedCostEstimate(22),
		actualCost:    22,
	},
}

func TestBindings(t *testing.T) {
	for _, tst := range bindingTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			var asts []*cel.Ast
			opts := append([]cel.EnvOption{Bindings(BindingsVersion(0)), Strings()}, tc.vars...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
			}
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

func TestBindingsNonMatch(t *testing.T) {
	env, err := cel.NewEnv(Bindings(), Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
	}
	nonMatchExpr := `ceel.bind(a, 1, a)`
	ast, iss := env.Parse(nonMatchExpr)
	if iss.Err() != nil {
		t.Fatalf("env.Parse(%v) failed: %v", nonMatchExpr, iss.Err())
	}
	if len(ast.SourceInfo().GetMacroCalls()) != 0 {
		t.Fatalf("env.Parse(%v) performed a macro replacement when none was expected: %v",
			nonMatchExpr, ast.SourceInfo().GetMacroCalls())
	}
}

func TestBindingsInvalidIdent(t *testing.T) {
	env, err := cel.NewEnv(Bindings(), Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
	}
	invalidIdentExpr := `cel.bind(a.b, 1, a.b)`
	wantErr := "ERROR: <input>:1:11: cel.bind() variable names must be simple identifiers"
	_, iss := env.Parse(invalidIdentExpr)
	if !strings.Contains(iss.Err().Error(), wantErr) {
		t.Fatalf("env.Parse(%v) failed: %v", invalidIdentExpr, iss.Err())
	}
}

func BenchmarkBindings(b *testing.B) {
	env, err := cel.NewEnv(Bindings(), Strings())
	if err != nil {
		b.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
	}
	for i, tst := range bindingTests {
		tc := tst
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
		if err != nil {
			b.Fatalf("env.Program(ast, Optimize) failed: %v", err)
		}
		// Benchmark the eval.
		b.Run(fmt.Sprintf("[%d]", i), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				prg.Eval(cel.NoVars())
			}
		})
	}
}

func TestBlockEval(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		expr ast.Expr
		opts []cel.EnvOption
		in   map[string]any
		out  ref.Val
	}{
		{
			name: "chained block",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "x"),
					fac.NewIdent(4, "@index0"),
					fac.NewIdent(5, "@index1"),
				}, []int32{}),
				fac.NewCall(9, operators.Add,
					fac.NewCall(6, operators.Add,
						fac.NewIdent(7, "@index2"),
						fac.NewIdent(8, "@index1")),
					fac.NewIdent(10, "@index0"),
				),
			),
			opts: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in:  map[string]any{"x": "hello"},
			out: types.String("hellohellohello"),
		},
		{
			name: "empty block",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{}, []int32{}),
				fac.NewCall(3, operators.LogicalNot, fac.NewLiteral(4, types.False)),
			),
			in:  map[string]any{},
			out: types.True,
		},
		{
			name: "mixed block constant values",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewLiteral(3, types.String("hello")),
					fac.NewLiteral(4, types.Int(5)),
				}, []int32{}),
				fac.NewCall(5, operators.Equals,
					fac.NewCall(6, "size",
						fac.NewIdent(7, "@index0")),
					fac.NewIdent(8, "@index1"),
				),
			),
			opts: []cel.EnvOption{
				cel.ExtendedValidations(),
			},
			in:  map[string]any{},
			out: types.True,
		},
		{
			name: "mixed block dynamic values",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "x"),
					fac.NewLiteral(4, types.Int(5)),
				}, []int32{}),
				fac.NewCall(5, operators.Equals,
					fac.NewCall(6, "size",
						fac.NewIdent(7, "@index0")),
					fac.NewIdent(8, "@index1"),
				),
			),
			opts: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
				cel.ExtendedValidations(),
			},
			in:  map[string]any{"x": "goodbye"},
			out: types.False,
		},
		{
			name: "mixed block constant values dyn var",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewLiteral(3, types.String("hello")),
				}, []int32{}),
				fac.NewCall(4, operators.Equals,
					fac.NewCall(5, "size",
						fac.NewIdent(6, "@index0")),
					fac.NewIdent(7, "y"),
				),
			),
			opts: []cel.EnvOption{
				cel.Variable("y", cel.IntType),
				cel.ExtendedValidations(),
			},
			in: map[string]any{
				"y": 5,
			},
			out: types.True,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			blockAST := ast.NewAST(tc.expr, nil)
			opts := append([]cel.EnvOption{Bindings()}, tc.opts...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
			}
			prg, err := env.PlanProgram(blockAST, cel.EvalOptions(cel.OptOptimize))
			if err != nil {
				t.Fatalf("PlanProgram() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.in)
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Equal(tc.out) != types.True {
				t.Errorf("got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestBlockEval_BadPlan(t *testing.T) {
	fac := ast.NewExprFactory()
	blockExpr := fac.NewCall(
		1, "cel.@block",
		fac.NewList(2, []ast.Expr{
			fac.NewIdent(3, "x"),
			fac.NewIdent(4, "@index0"),
		}, []int32{}),
		fac.NewCall(6, operators.Add,
			fac.NewIdent(7, "@index1"),
			fac.NewIdent(8, "@index0")),
		fac.NewIdent(9, "x"),
	)
	blockAST := ast.NewAST(blockExpr, nil)
	env, err := cel.NewEnv(
		Bindings(BindingsVersion(1)),
		cel.Variable("x", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	_, err = env.PlanProgram(blockAST)
	if err == nil {
		t.Fatal("PlanProgram() succeeded, expected error")
	}
}

func TestBlockEval_BadBlock(t *testing.T) {
	fac := ast.NewExprFactory()
	blockExpr := fac.NewCall(
		1, "cel.@block",
		fac.NewCall(2, operators.Add,
			fac.NewIdent(3, "@index1"),
			fac.NewIdent(4, "@index0")),
		fac.NewIdent(5, "x"),
	)
	blockAST := ast.NewAST(blockExpr, nil)
	env, err := cel.NewEnv(
		Bindings(BindingsVersion(1)),
		cel.Variable("x", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	_, err = env.PlanProgram(blockAST)
	if err == nil {
		t.Fatal("PlanProgram() succeeded, expected error")
	}
}

func TestBlockEval_RuntimeErrors(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		expr ast.Expr
	}{
		{
			name: "bad index",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "x"),
					fac.NewIdent(4, "@indexNext"),
				}, []int32{}),
				fac.NewCall(6, operators.Add,
					fac.NewIdent(7, "@indexNext"),
					fac.NewIdent(8, "@index0")),
			),
		},
		{
			name: "infinite recursion",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "@index0"),
					fac.NewIdent(4, "@index0"),
				}, []int32{}),
				fac.NewIdent(10, "@index0"),
			),
		},
		{
			name: "negative index",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "@index-1"),
					fac.NewIdent(4, "@index0"),
				}, []int32{}),
				fac.NewIdent(10, "@index0"),
			),
		},
		{
			name: "out of range index",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "@index100"),
					fac.NewIdent(4, "@index0"),
				}, []int32{}),
				fac.NewIdent(10, "@index0"),
			),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			blockAST := ast.NewAST(tc.expr, nil)
			env, err := cel.NewEnv(
				Bindings(BindingsVersion(1)),
				cel.Variable("x", cel.StringType),
			)
			if err != nil {
				t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
			}
			prg, err := env.PlanProgram(blockAST)
			if err != nil {
				t.Fatalf("PlanProgram() failed: %v", err)
			}
			_, _, err = prg.Eval(map[string]any{"x": "hello"})
			if !strings.Contains(err.Error(), "no such attribute") {
				t.Fatalf("prg.Eval() got %v, expected no such attribute error", err)
			}
		})
	}
}

func TestLegacyBlockEval(t *testing.T) {
	// 1. Cover dynamicBlock.Eval
	db := &dynamicBlock{
		expr: interpreter.NewConstValue(1, types.IntOne),
		slotActivationPool: &sync.Pool{
			New: func() any {
				return &dynamicSlotActivation{}
			},
		},
	}
	res1 := db.Eval(cel.NoVars())
	if res1.Equal(types.IntOne) != types.True {
		t.Errorf("db.Eval() = %v, wanted 1", res1)
	}

	// 2. Cover constantBlock.Eval
	cb := &constantBlock{
		expr: interpreter.NewConstValue(2, types.IntOne),
	}
	res2 := cb.Eval(cel.NoVars())
	if res2.Equal(types.IntOne) != types.True {
		t.Errorf("cb.Eval() = %v, wanted 1", res2)
	}
}
