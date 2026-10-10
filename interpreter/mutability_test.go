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

package interpreter

import (
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/parser"
)

// compreMacro exposes a raw comprehension to the parser for testing purposes:
//
//	compre(iterVar, accuVar, iterRange, accuInit, loopCondition, loopStep, result)
var compreMacro = parser.NewGlobalMacro("compre", 7,
	func(eh parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		return eh.NewComprehension(args[2], args[0].AsIdent(), args[1].AsIdent(),
			args[3], args[4], args[5], args[6]), nil
	})

// compre2Macro exposes a raw two-variable comprehension to the parser for testing purposes:
//
//	compre2(iterVar, iterVar2, accuVar, iterRange, accuInit, loopCondition, loopStep, result)
var compre2Macro = parser.NewGlobalMacro("compre2", 8,
	func(eh parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		return eh.NewComprehensionTwoVar(args[3], args[0].AsIdent(), args[1].AsIdent(),
			args[2].AsIdent(), args[4], args[5], args[6], args[7]), nil
	})

// bindMacro mirrors the cel.bind() macro in the ext package: bind(var, init, result)
var bindMacro = parser.NewGlobalMacro("bind", 3,
	func(eh parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		varName := args[0].AsIdent()
		return eh.NewComprehension(eh.NewList(), "#unused", varName,
			args[1], eh.NewLiteral(types.False), eh.NewIdent(varName), args[2]), nil
	})

// insertMacro produces a call to the internal map insertion function: insert(map, key, value)
// or insert(map, entries)
var insertMacro = parser.NewGlobalVarArgMacro("insert",
	func(eh parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		return eh.NewCall(mapInsertFunction, args...), nil
	})

// hiddenTestVars lists the identifiers which parseMutabilityExpr rewrites to '@'-prefixed names
// since hidden names like `@a` cannot be written in CEL source. This allows hand-written test
// comprehensions to use accumulators that look like the hidden `@result` accumulator produced by
// the standard macros.
var hiddenTestVars = map[string]string{"a": "@a", "b": "@b"}

func parseMutabilityExpr(t *testing.T, expr string) *ast.AST {
	t.Helper()
	p, err := parser.NewParser(
		parser.Macros(append(parser.AllMacros, compreMacro, compre2Macro, bindMacro, insertMacro)...),
	)
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	parsed, errs := p.Parse(common.NewTextSource(expr))
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Parse(%q) failed: %v", expr, errs.ToDisplayString())
	}
	hideTestVars(parsed.Expr())
	return parsed
}

// hideTestVars renames identifiers and comprehension variables listed in hiddenTestVars.
func hideTestVars(e ast.Expr) {
	fac := ast.NewExprFactory()
	rename := func(name string) string {
		if hidden, found := hiddenTestVars[name]; found {
			return hidden
		}
		return name
	}
	ast.PostOrderVisit(e, ast.NewExprVisitor(func(e ast.Expr) {
		switch e.Kind() {
		case ast.IdentKind:
			e.SetKindCase(fac.NewIdent(e.ID(), rename(e.AsIdent())))
		case ast.ComprehensionKind:
			c := e.AsComprehension()
			e.SetKindCase(fac.NewComprehensionTwoVar(e.ID(), c.IterRange(),
				rename(c.IterVar()), rename(c.IterVar2()), rename(c.AccuVar()),
				c.AccuInit(), c.LoopCondition(), c.LoopStep(), c.Result()))
		}
	}))
}

func TestIsMutableAccuSafe(t *testing.T) {
	tests := []struct {
		expr string
		safe bool
	}{
		// Standard macros which build lists and maps are optimized.
		{expr: `[1, 2].map(x, x + 1)`, safe: true},
		{expr: `[1, 2].map(x, x > 1, x + 1)`, safe: true},
		{expr: `[1, 2].filter(x, x > 1)`, safe: true},
		{expr: `[[1], [2]].map(x, x.map(y, y + 1))`, safe: true},
		{expr: `compre(i, a, [1], [], true, a + [i], a)`, safe: true},
		{expr: `compre(i, a, [1], [], true, i > 0 ? a + [i] : a, a)`, safe: true},
		{expr: `compre(i, a, [1], [], true, a + [compre(j, a, [2], [], true, a + [j], a)], a)`, safe: true},
		// Map insertion requires a two-variable comprehension.
		{expr: `compre2(k, v, a, {1: 2}, {}, true, insert(a, k, v), a)`, safe: true},
		{expr: `compre2(k, v, a, {1: 2}, {}, true, insert(a, {k: v}), a)`, safe: true},
		{expr: `compre2(k, v, a, {1: 2}, {}, true, k > 0 ? insert(a, k, v) : a, a)`, safe: true},
		{expr: `compre(i, a, [1], {}, true, insert(a, i, i), a)`, safe: false},

		// Shapes which cel-cpp accepts without inspecting the accumulator's other uses. These
		// match cel-cpp's IsOptimizableListAppend / IsOptimizableMapInsert behavior.
		{expr: `compre(i, a, [1], [], a.size() < 2, a + [i], a)`, safe: true},
		{expr: `compre(i, a, [1], [], true, a + [a], a)`, safe: true},
		{expr: `compre(i, a, [1], [], true, a.size() > 0 ? a + [i] : a, a)`, safe: true},
		{expr: `compre(i, a, [1], [], true, i > 0 ? a + [i] : [a], a)`, safe: true},
		{expr: `compre2(k, v, a, {1: 2}, {}, true, insert(a, k, a), a)`, safe: true},

		// cel.bind style comprehensions where the result is not the bare accumulator.
		{expr: `bind(x, [], [x + [1], x + [2]])`, safe: false},
		{expr: `bind(x, [], x + [1])`, safe: false},
		{expr: `bind(x, [], bind(y, x + [1], x))`, safe: false},
		{expr: `bind(x, {}, [x, x])`, safe: false},
		{expr: `compre(i, a, [1], [], true, a + [i], [a, a])`, safe: false},
		{expr: `compre(i, a, [1], [], true, a + [i], a.size())`, safe: false},
		{expr: `compre(i, a, [1], [], true, a + [i], 1)`, safe: false},
		// Accumulator initializer must be an empty list or map literal.
		{expr: `compre(i, a, [1], [0], true, a + [i], a)`, safe: false},
		{expr: `compre(i, a, [1], dyn([]), true, a + [i], a)`, safe: false},
		{expr: `compre2(k, v, a, {1: 2}, {'x': 1}, true, insert(a, k, v), a)`, safe: false},
		// Loop step must be `accu + [elem]`, optionally as the true branch of a ternary.
		{expr: `compre(i, a, [1], [], true, [i], a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, a + a, a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, a + [i, i], a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, [i] + a, a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, (a + [i]) + [i], a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, {'k': a}.k + [i], a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, i > 0 ? a : a + [i], a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, bind(y, a, y + [i]), a)`, safe: false},
		{expr: `compre(i, a, [1], [], true, compre(j, b, a, [], true, b + [j], b), a)`, safe: false},
		// Empty or shadowed accumulator names disable mutation.
		{expr: `compre(a, a, [1], [], true, a + [1], a)`, safe: false},
		{expr: `compre2(k, a, a, {1: 2}, {}, true, insert(a, k, k), a)`, safe: false},
		// Accumulator names which may be written in CEL source disable mutation, even when the
		// comprehension otherwise matches the cel-cpp shape.
		{expr: `compre(i, acc, [1], [], true, acc + [i], acc)`, safe: false},
		{expr: `compre(i, __result__, [1], [], true, __result__ + [i], __result__)`, safe: false},
		{expr: `compre2(k, v, acc, {1: 2}, {}, true, insert(acc, k, v), acc)`, safe: false},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			parsed := parseMutabilityExpr(t, tc.expr)
			e := parsed.Expr()
			if e.Kind() != ast.ComprehensionKind {
				t.Fatalf("got expr kind %v, wanted comprehension", e.Kind())
			}
			if got := isMutableAccuSafe(e.AsComprehension()); got != tc.safe {
				t.Errorf("isMutableAccuSafe(%s) got %v, wanted %v", tc.expr, got, tc.safe)
			}
		})
	}
}

// TestComprehensionAccumulatorImmutability verifies that comprehension variables are not visibly
// mutated during evaluation.
//
// Note: since the mutable accumulator analysis mirrors cel-cpp, hand-crafted comprehensions in
// the standard macro shape with a hidden ('@'-prefixed) accumulator which alias the accumulator,
// e.g. `@a + [@a]`, or which accumulate into it from the loop condition are intentionally not
// covered here. Hidden accumulator names cannot be referenced from CEL source.
func TestComprehensionAccumulatorImmutability(t *testing.T) {
	tests := []string{
		`bind(x, [], [x + [1], x + [2]]) == [[1], [2]]`,
		`bind(x, [], bind(y, x + [1], bind(z, x + [2], [x, y, z]))) == [[], [1], [2]]`,
		`bind(x, [], bind(y, x + [1], x)) == []`,
		`bind(x, [], x + [1] + [2]) == [1, 2]`,
		`bind(x, [], [x + [1]].map(l, l + [2])) == [[1, 2]]`,
		`bind(x, [], [1, 2].map(i, x + [i])) == [[1], [2]]`,
		`bind(x, [], [1, 2].filter(i, (x + [i]).size() > 1)) == []`,
		`compre(i, a, [1, 2], [], true, a + [i], [a, a]) == [[1, 2], [1, 2]]`,
		`compre(i, a, [1, 2], [], true, a + [a.size()], a) == [0, 1]`,
		`compre(i, a, [1, 2], [], true, bind(y, a, y + [i]), a) == [1, 2]`,
		`compre(i, a, [1, 2], [], true, [a + [i]][0], a) == [1, 2]`,
		// Hand-written comprehensions with user-visible accumulator names are never optimized,
		// so aliasing the accumulator is safe.
		`compre(i, acc, [1, 2], [], true, acc + [acc], acc) == [[], [[]]]`,
		`compre(i, acc, [1, 2], [], true, acc + [(acc + [0]).size()], acc) == [1, 2]`,
		`[[1], [2]].map(x, x.map(y, y + 1)) == [[2], [3]]`,
		`[1, 2, 3].filter(x, x > 1) == [2, 3]`,
		`[1, 2, 3].map(x, x % 2 == 1, x * 10) == [10, 30]`,
	}
	// Loop conditions are ignored during exhaustive evaluation, so these are only tested with
	// short-circuiting plans.
	shortCircuitTests := []string{
		`compre(i, a, [1, 2, 3], [], a.size() < 2, a + [i], a) == [1, 2]`,
		`compre(i, acc, [1, 2, 3], [], (acc + [i]).size() < 3, acc + [i], acc) == [1, 2]`,
	}
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	planOpts := map[string][]PlannerOption{
		"default":    {},
		"exhaustive": {ExhaustiveEval()},
		"optimized":  {Optimize()},
	}
	type evalCase struct {
		expr       string
		exhaustive bool
	}
	var cases []evalCase
	for _, expr := range tests {
		cases = append(cases, evalCase{expr: expr, exhaustive: true})
	}
	for _, expr := range shortCircuitTests {
		cases = append(cases, evalCase{expr: expr})
	}
	for _, tc := range cases {
		expr := tc.expr
		for name, opts := range planOpts {
			if name == "exhaustive" && !tc.exhaustive {
				continue
			}
			t.Run(name+"/"+expr, func(t *testing.T) {
				parsed := parseMutabilityExpr(t, expr)
				prg, err := interp.NewInterpretable(parsed, opts...)
				if err != nil {
					t.Fatalf("NewInterpretable() failed: %v", err)
				}
				// Evaluate repeatedly to ensure that pooled state does not leak across evaluations.
				for i := 0; i < 3; i++ {
					out := prg.Eval(EmptyActivation())
					if out != types.True {
						t.Errorf("Eval(%s) got %v, wanted true", expr, out)
					}
				}
			})
		}
	}
}
