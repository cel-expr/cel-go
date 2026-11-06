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

package cel_test

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/ext"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

// TestStaticOptimizerMacroCallRetention exercises multi-pass and multi-stage optimizations
// over expressions with nested macros to ensure that macro call metadata which is still
// referenced by the optimized expression is retained, while metadata for macros which have
// been optimized away is pruned.
func TestStaticOptimizerMacroCallRetention(t *testing.T) {
	type stage []string
	inline := stage{"inline"}
	fold := stage{"fold"}
	inlineFold := stage{"inline", "fold"}
	tests := []struct {
		name       string
		expr       string
		stages     []stage
		want       string
		macroCount int
	}{
		{
			name:       "nested macros retained when unfoldable",
			expr:       `xs.all(i, [1, 2].exists(j, j == i))`,
			stages:     []stage{fold},
			want:       `xs.all(i, [1, 2].exists(j, j == i))`,
			macroCount: 2,
		},
		{
			name:       "inner macro folded, outer retained",
			expr:       `xs.all(i, [1, 2].exists(j, j == 2) && i > 0)`,
			stages:     []stage{fold},
			want:       `xs.all(i, true && i > 0)`,
			macroCount: 1,
		},
		{
			name:       "macro hoisted through folded ternary",
			expr:       `[1, 2].exists(j, j == 2) ? xs.map(i, i + 1) : []`,
			stages:     []stage{fold},
			want:       `xs.map(i, i + 1)`,
			macroCount: 1,
		},
		{
			name:       "macro nested in macro target",
			expr:       `[xs.filter(i, i > 0)].all(l, l.size() >= 0) || has(m.k)`,
			stages:     []stage{fold},
			want:       `[xs.filter(i, i > 0)].all(l, l.size() >= 0) || has(m.k)`,
			macroCount: 3,
		},
		{
			name:       "nested orphans pruned, sibling retained",
			expr:       `[[1].exists(y, y == 1)].all(z, z) && xs.exists(i, i > 0)`,
			stages:     []stage{fold},
			want:       `xs.exists(i, i > 0)`,
			macroCount: 1,
		},
		{
			name:       "nested orphans pruned across repeated folding",
			expr:       `[[1].exists(y, y == 1)].all(z, z) && xs.exists(i, [i].all(w, w > 0))`,
			stages:     []stage{fold, fold},
			want:       `xs.exists(i, [i].all(w, w > 0))`,
			macroCount: 2,
		},
		{
			name:       "inlined macro under bind",
			expr:       `a.exists(i, i > 2) && a.size() > 0`,
			stages:     []stage{inline},
			want:       `cel.bind(alpha, [1, 2, 3].map(i, i * 2), alpha.exists(i, i > 2) && alpha.size() > 0)`,
			macroCount: 3,
		},
		{
			name:       "inlined macro under bind then folded",
			expr:       `a.exists(i, i > 2) && a.size() > 0`,
			stages:     []stage{inline, fold},
			want:       `true`,
			macroCount: 0,
		},
		{
			name:       "inlined and folded in one optimizer",
			expr:       `a.exists(i, i > 2) && a.size() > 0`,
			stages:     []stage{inlineFold},
			want:       `true`,
			macroCount: 0,
		},
		{
			name:       "inlined macros chained with presence test",
			expr:       `b.map(k, k + 1).size() > 0 && c`,
			stages:     []stage{inline, fold},
			want:       `xs.filter(j, j > 0).map(k, k + 1).size() > 0 && has(m.k)`,
			macroCount: 3,
		},
		{
			name:       "inlined macro folded within retained macro",
			expr:       `xs.all(v, a.exists(w, w == v)) || c`,
			stages:     []stage{inlineFold},
			want:       `xs.all(v, [2, 4, 6].exists(w, w == v)) || has(m.k)`,
			macroCount: 3,
		},
		{
			name:       "inlined macro folded within retained macro, separate stages",
			expr:       `xs.all(v, a.exists(w, w == v)) || c`,
			stages:     []stage{inline, fold, fold},
			want:       `xs.all(v, [2, 4, 6].exists(w, w == v)) || has(m.k)`,
			macroCount: 3,
		},
		{
			name:       "inlined bind with nested macro pruned",
			expr:       `(a.size() > 10 && a.exists(i, [i].all(j, j > 0))) || xs.exists(x, x == 0)`,
			stages:     []stage{inline, fold},
			want:       `xs.exists(x, x == 0)`,
			macroCount: 1,
		},
		{
			name:       "two-variable comprehension with folded inner macro",
			expr:       `{'a': xs.filter(i, i > 0)}.all(k, v, v.size() >= 0 && [1].exists(z, z == 1))`,
			stages:     []stage{fold},
			want:       `{"a": xs.filter(i, i > 0)}.all(k, v, v.size() >= 0 && true)`,
			macroCount: 2,
		},
		{
			name:       "optional macro with nested macro",
			expr:       `optional.of(xs).optMap(l, l.filter(i, i > 0))`,
			stages:     []stage{fold},
			want:       `optional.of(xs).optMap(l, l.filter(i, i > 0))`,
			macroCount: 2,
		},
		{
			name:       "presence test retained within folded comprehension",
			expr:       `[1].exists(z, z == 1) && has(msg.single_int64) && xs.all(i, has(msg.single_int32))`,
			stages:     []stage{fold},
			want:       `has(msg.single_int64) && xs.all(i, has(msg.single_int32))`,
			macroCount: 3,
		},
	}

	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		ext.Bindings(),
		ext.TwoVarComprehensions(),
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.Variable("msg", cel.ObjectType("google.expr.proto3.test.TestAllTypes")),
		cel.Variable("xs", cel.ListType(cel.IntType)),
		cel.Variable("m", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("a", cel.ListType(cel.IntType)),
		cel.Variable("b", cel.ListType(cel.IntType)),
		cel.Variable("c", cel.BoolType),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	compile := func(t *testing.T, expr string) *cel.Ast {
		t.Helper()
		checked, iss := env.Compile(expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
		}
		return checked
	}
	newOptimizer := func(t *testing.T, name string) cel.ASTOptimizer {
		t.Helper()
		switch name {
		case "fold":
			folder, err := cel.NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			return folder
		case "inline":
			return cel.NewInliningOptimizer(
				cel.NewInlineVariableWithAlias("a", "alpha", compile(t, `[1, 2, 3].map(i, i * 2)`)),
				cel.NewInlineVariable("b", compile(t, `xs.filter(j, j > 0)`)),
				cel.NewInlineVariable("c", compile(t, `has(m.k)`)),
			)
		}
		t.Fatalf("unknown optimizer: %q", name)
		return nil
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			optimized := compile(t, tc.expr)
			checkMacroCallIntegrity(t, env, optimized)
			for _, s := range tc.stages {
				var opts []any
				for _, name := range s {
					opts = append(opts, newOptimizer(t, name))
				}
				opt, err := cel.NewStaticOptimizer(opts...)
				if err != nil {
					t.Fatalf("NewStaticOptimizer() failed: %v", err)
				}
				var iss *cel.Issues
				optimized, iss = opt.Optimize(env, optimized)
				if iss.Err() != nil {
					t.Fatalf("Optimize() failed: %v", iss.Err())
				}
				checkMacroCallIntegrity(t, env, optimized)
			}
			got, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("optimized got %q, wanted %q", got, tc.want)
			}
			if n := len(optimized.NativeRep().SourceInfo().MacroCalls()); n != tc.macroCount {
				t.Errorf("got %d macro calls, wanted %d", n, tc.macroCount)
			}
		})
	}
}

// checkMacroCallIntegrity validates the consistency between the expression graph and the
// macro call metadata of an optimized AST:
//
//   - every macro-generated node (comprehension or presence test) has a macro call record,
//   - every nested macro reference within a macro call resolves to a macro call record,
//   - every macro call record is reachable from the expression graph, and
//   - unparsing and re-parsing the AST yields the same number of macro calls.
func checkMacroCallIntegrity(t *testing.T, env *cel.Env, a *cel.Ast) {
	t.Helper()
	info := a.NativeRep().SourceInfo()
	macroCalls := info.MacroCalls()

	// Macro-generated nodes within the expression must retain their macro call metadata,
	// either directly or via an enclosing macro expansion (e.g. optMap expands to a
	// conditional whose branches contain a comprehension).
	expr := a.NativeRep().Expr()
	exprIDs := map[int64]struct{}{}
	coveredIDs := map[int64]struct{}{}
	ast.PostOrderVisit(expr, ast.NewExprVisitor(func(e ast.Expr) {
		exprIDs[e.ID()] = struct{}{}
		if _, found := macroCalls[e.ID()]; !found {
			return
		}
		ast.PostOrderVisit(e, ast.NewExprVisitor(func(sub ast.Expr) {
			coveredIDs[sub.ID()] = struct{}{}
		}))
	}))
	ast.PostOrderVisit(expr, ast.NewExprVisitor(func(e ast.Expr) {
		isMacroNode := e.Kind() == ast.ComprehensionKind ||
			(e.Kind() == ast.SelectKind && e.AsSelect().IsTestOnly())
		if _, found := coveredIDs[e.ID()]; isMacroNode && !found {
			t.Errorf("expression id %d (kind %v) is missing its macro call", e.ID(), e.Kind())
		}
	}))

	// Nested macro references within a macro call must resolve to a macro call record.
	for id, call := range macroCalls {
		ast.PostOrderVisit(call, ast.NewExprVisitor(func(e ast.Expr) {
			if e.ID() == 0 || e.Kind() != ast.UnspecifiedExprKind {
				return
			}
			if _, found := macroCalls[e.ID()]; !found {
				t.Errorf("macro call %d has a dangling reference to macro %d", id, e.ID())
			}
		}))
	}

	// All macro calls must be reachable from the expression graph.
	for id := range macroCalls {
		if _, found := exprIDs[id]; !found {
			t.Errorf("macro call %d is not referenced by the expression", id)
		}
	}

	// Round-trip the expression through the unparser to ensure the macro metadata is
	// complete enough to reproduce the same macro structure.
	unparsed, err := cel.AstToString(a)
	if err != nil {
		t.Fatalf("AstToString() failed: %v", err)
	}
	reparsed, iss := env.Compile(unparsed)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", unparsed, iss.Err())
	}
	if want := len(reparsed.NativeRep().SourceInfo().MacroCalls()); len(macroCalls) != want {
		t.Errorf("got %d macro calls, wanted %d to match re-parsed expression %q",
			len(macroCalls), want, unparsed)
	}
}
