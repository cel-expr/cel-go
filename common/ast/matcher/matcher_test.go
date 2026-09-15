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

package matcher_test

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/ast/matcher"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/ext"
	"cel.dev/cel-go/parser"
)

func testAST(t testing.TB, src string, envOpts ...cel.EnvOption) *ast.AST {
	t.Helper()
	opts := append([]cel.EnvOption{
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		cel.EnableIdentifierEscapeSyntax(),
	}, envOpts...)
	env, err := cel.NewEnv(opts...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	if len(envOpts) > 0 {
		ast, iss := env.Compile(src)
		if iss.Err() != nil {
			t.Fatalf("env.Compile(%q) failed: %v", src, iss.Err())
		}
		return ast.NativeRep()
	}
	prs, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
		parser.EnablePrattParser(true),
		parser.EnableIdentEscapeSyntax(true),
		parser.EnableCallEscapeSyntax(true),
		parser.PopulateMacroCalls(true),
		parser.EnableOptionalSyntax(true),
	)
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	parsed, iss := prs.Parse(common.NewTextSource(src))
	if iss != nil && len(iss.GetErrors()) > 0 {
		t.Fatalf("prs.Parse(%q) failed: %v", src, iss.ToDisplayString())
	}
	return parsed
}

func testParsedAST(t testing.TB, src string, envOpts ...cel.EnvOption) *ast.AST {
	t.Helper()
	opts := append([]cel.EnvOption{
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		cel.EnableIdentifierEscapeSyntax(),
	}, envOpts...)
	env, err := cel.NewEnv(opts...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Parse(src)
	if iss.Err() != nil {
		t.Fatalf("env.Parse(%q) failed: %v", src, iss.Err())
	}
	return ast.NativeRep()
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		target     string
		envOpts    []cel.EnvOption
		equivOpts  []ast.EquivOption
		wantMatch  bool
		checkSlots func(t *testing.T, res matcher.MatchResult)
	}{
		// Algebraic simplifications
		{
			name:      "identity addition",
			pattern:   "_1 + 0",
			target:    "(a * b) + 0",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.CallKind || e1.AsCall().FunctionName() != "_*_" {
					t.Errorf("unexpected expr for _1: %v", e1)
				}
			},
		},
		{
			name:      "multiplication by zero",
			pattern:   "_1 * 0",
			target:    "size(items) * 0",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.CallKind || e1.AsCall().FunctionName() != "size" {
					t.Errorf("unexpected expr for _1: %v", e1)
				}
			},
		},
		{
			name:      "self subtraction matching",
			pattern:   "_1 - _1",
			target:    "user.age - user.age",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.SelectKind || e1.AsSelect().FieldName() != "age" {
					t.Errorf("unexpected expr for _1: %v", e1)
				}
			},
		},
		{
			name:      "self subtraction mismatch",
			pattern:   "_1 - _1",
			target:    "user.age - user.id",
			wantMatch: false,
		},
		{
			name:      "logical AND with true",
			pattern:   "_1 && true",
			target:    "(x > 10 && y < 20) && true",
			wantMatch: true,
		},
		{
			name:      "logical OR with false",
			pattern:   "_1 || false",
			target:    "has(user.email) || false",
			wantMatch: true,
		},
		{
			name:      "self equality tautology",
			pattern:   "_1 == _1",
			target:    "request.auth.claims.sub == request.auth.claims.sub",
			wantMatch: true,
		},
		{
			name:      "double negation",
			pattern:   "!(!_1)",
			target:    "!(!isValid)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.IdentKind || e1.AsIdent() != "isValid" {
					t.Errorf("unexpected expr for _1: %v", e1)
				}
			},
		},
		{
			name:      "anonymous slot wildcard",
			pattern:   "_ + _",
			target:    "42 + 100",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if _, ok := res.FirstExpr(0); ok {
					t.Errorf("anonymous wildcard should not bind to slot 0")
				}
			},
		},
		{
			name:      "slot _0 basic match and capture",
			pattern:   "_0 + 0",
			target:    "42 + 0",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e0, ok := res.FirstExpr(0)
				if !ok || e0.Kind() != ast.LiteralKind || e0.AsLiteral().Value() != int64(42) {
					t.Errorf("unexpected expr for _0: %v", e0)
				}
				raw0, okRaw := res.FirstRawExpr(0)
				if !okRaw || raw0 == nil {
					t.Errorf("expected FirstRawExpr(0) to succeed")
				}
			},
		},
		{
			name:      "slot _0 non-linear match",
			pattern:   "_0 == _0",
			target:    "x == x",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e0, ok := res.FirstExpr(0)
				if !ok || e0.Kind() != ast.IdentKind || e0.AsIdent() != "x" {
					t.Errorf("unexpected expr for _0: %v", e0)
				}
			},
		},
		{
			name:      "slot _0 non-linear mismatch",
			pattern:   "_0 == _0",
			target:    "x == y",
			wantMatch: false,
		},
		{
			name:      "slot _0 sequence quantifier star",
			pattern:   "concat(_0.star())",
			target:    "concat('a', 'b')",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				elems, ok := res.Exprs(0)
				if !ok || len(elems) != 2 {
					t.Errorf("unexpected elements for _0: %v", elems)
				}
				raws, okRaw := res.RawExprs(0)
				if !okRaw || len(raws) != 2 {
					t.Errorf("unexpected raw elements for _0: %v", raws)
				}
			},
		},

		// Expression Kind Constraints
		{
			name:      "exprKind ident match",
			pattern:   "_1.exprKind(ident) + 1",
			target:    "x + 1",
			wantMatch: true,
		},
		{
			name:      "exprKind ident mismatch on call",
			pattern:   "_1.exprKind(ident) + 1",
			target:    "foo() + 1",
			wantMatch: false,
		},
		{
			name:      "exprKind call match",
			pattern:   "_1.exprKind(call) + 1",
			target:    "foo() + 1",
			wantMatch: true,
		},
		{
			name:      "exprKind select match",
			pattern:   "_1.exprKind(select) + 1",
			target:    "a.b + 1",
			wantMatch: true,
		},
		{
			name:      "exprKind list match",
			pattern:   "size(_1.exprKind(list))",
			target:    "size([1, 2, 3])",
			wantMatch: true,
		},
		{
			name:      "exprKind list mismatch on ident",
			pattern:   "size(_1.exprKind(list))",
			target:    "size(myList)",
			wantMatch: false,
		},
		{
			name:      "exprKind map match",
			pattern:   "size(_1.exprKind(map))",
			target:    "size({'k': 'v'})",
			wantMatch: true,
		},
		{
			name:      "exprKind struct match",
			pattern:   "_1.exprKind(struct, pkg.MyMessage)",
			target:    "pkg.MyMessage{field: 'hello'}",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.StructKind {
					t.Errorf("expected _1 to be struct, got %v", e1)
				}
			},
		},
		{
			name:      "exprKind struct mismatch on type",
			pattern:   "_1.exprKind(struct, pkg.MyMessage)",
			target:    "pkg.OtherMessage{field: 'hello'}",
			wantMatch: false,
		},

		// Type Constraints
		{
			name:      "type int match",
			pattern:   "_1.type(int) + 1",
			target:    "x + 1",
			envOpts:   []cel.EnvOption{cel.Variable("x", cel.IntType)},
			wantMatch: true,
		},
		{
			name:      "type int mismatch on double",
			pattern:   "_1.type(int) + 1",
			target:    "x + 1.0",
			envOpts:   []cel.EnvOption{cel.Variable("x", cel.DoubleType)},
			wantMatch: false,
		},
		{
			name:      "type string match",
			pattern:   "_1.type(string) + 's'",
			target:    "x + 's'",
			envOpts:   []cel.EnvOption{cel.Variable("x", cel.StringType)},
			wantMatch: true,
		},

		// Sequence Quantifiers - star
		{
			name:      "star matches 0 args",
			pattern:   "concat(_1, _2.star())",
			target:    "concat('a')",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if _, ok := res.FirstExpr(1); !ok {
					t.Errorf("_1 should be set")
				}
				if e2s, ok := res.Exprs(2); !ok || len(e2s) != 0 {
					t.Errorf("_2.star() should have 0 elements, got %v", e2s)
				}
			},
		},
		{
			name:      "star matches 2 args",
			pattern:   "concat(_1, _2.star())",
			target:    "concat('a', 'b', 'c')",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e2s, ok := res.Exprs(2); !ok || len(e2s) != 2 {
					t.Errorf("_2.star() should have 2 elements, got %d", len(e2s))
				}
			},
		},

		// Sequence Quantifiers - plus
		{
			name:      "plus fails on 0 trailing args",
			pattern:   "fn(_1, _2.plus())",
			target:    "fn(1)",
			wantMatch: false,
		},
		{
			name:      "plus matches 1 trailing arg",
			pattern:   "fn(_1, _2.plus())",
			target:    "fn(1, 2)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e2s, ok := res.Exprs(2); !ok || len(e2s) != 1 {
					t.Errorf("expected 1 element for _2, got %d", len(e2s))
				}
			},
		},
		{
			name:      "plus matches 2 trailing args",
			pattern:   "fn(_1, _2.plus())",
			target:    "fn(1, 2, 3)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e2s, ok := res.Exprs(2); !ok || len(e2s) != 2 {
					t.Errorf("expected 2 elements for _2, got %d", len(e2s))
				}
			},
		},

		// Sequence Quantifiers - optional
		{
			name:      "optional matches 0 trailing args",
			pattern:   "slice(_1, _2, _3.optional())",
			target:    "slice(items, 0)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if _, ok := res.FirstExpr(3); ok {
					t.Errorf("_3 should not be present when optional matches 0 items")
				}
			},
		},
		{
			name:      "optional matches 1 trailing arg",
			pattern:   "slice(_1, _2, _3.optional())",
			target:    "slice(items, 0, 10)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e3, ok := res.FirstExpr(3); !ok || e3 == nil {
					t.Errorf("_3 should be present when optional matches 1 item")
				}
			},
		},

		// Sequence Quantifiers - atLeast / atMost
		{
			name:      "atLeast and atMost fails below min",
			pattern:   "format(_1, _2.atLeast(1).atMost(2))",
			target:    "format('fmt')",
			wantMatch: false,
		},
		{
			name:      "atLeast and atMost matches 1 arg",
			pattern:   "format(_1, _2.atLeast(1).atMost(2))",
			target:    "format('fmt', a)",
			wantMatch: true,
		},
		{
			name:      "atLeast and atMost matches 2 args",
			pattern:   "format(_1, _2.atLeast(1).atMost(2))",
			target:    "format('fmt', a, b)",
			wantMatch: true,
		},
		{
			name:      "atLeast and atMost fails above max",
			pattern:   "format(_1, _2.atLeast(1).atMost(2))",
			target:    "format('fmt', a, b, c)",
			wantMatch: false,
		},

		// Sequence Quantifiers in Lists
		{
			name:      "repeated in list fails count mismatch",
			pattern:   "[_1.repeated(2), _2.star()]",
			target:    "[1]",
			wantMatch: false,
		},
		{
			name:      "repeated in list matches exact count",
			pattern:   "[_1.repeated(2), _2.star()]",
			target:    "[1, 2]",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e1s, _ := res.Exprs(1); len(e1s) != 2 {
					t.Errorf("_1.repeated(2) should have 2 elements, got %d", len(e1s))
				}
			},
		},
		{
			name:      "repeated in list with trailing star",
			pattern:   "[_1.repeated(2), _2.star()]",
			target:    "[1, 2, 3, 4]",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e2s, _ := res.Exprs(2); len(e2s) != 2 {
					t.Errorf("_2.star() should have 2 elements, got %d", len(e2s))
				}
			},
		},

		// Maps and Structs
		{
			name:      "map matching",
			pattern:   "{'key': _1}",
			target:    "{'key': 42}",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.LiteralKind || e1.AsLiteral().Value() != int64(42) {
					t.Errorf("expected _1 to be 42, got %v", e1)
				}
			},
		},
		{
			name:      "struct matching",
			pattern:   "pkg.MyMessage{field: _1}",
			target:    "pkg.MyMessage{field: 'hello'}",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.LiteralKind || e1.AsLiteral().Value() != "hello" {
					t.Errorf("expected _1 to be 'hello', got %v", e1)
				}
			},
		},

		// Macro Matching
		{
			name:      "filter macro matching with alpha equivalence",
			pattern:   "_1.filter(x, true)",
			target:    "users.filter(u, true)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.IdentKind || e1.AsIdent() != "users" {
					t.Errorf("expected _1 to be 'users', got %v", e1)
				}
			},
		},
		{
			name:      "all macro matching with alpha equivalence",
			pattern:   "_1.all(x, x > _2)",
			target:    "items.all(it, it > 10)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				if e1, ok := res.FirstExpr(1); !ok || e1.AsIdent() != "items" {
					t.Errorf("expected _1 to be 'items', got %v", e1)
				}
				if e2, ok := res.FirstExpr(2); !ok || e2.AsLiteral().Value() != int64(10) {
					t.Errorf("expected _2 to be 10, got %v", e2)
				}
			},
		},
		{
			name:      "repeated slot with alpha-equivalent comprehensions (default enabled)",
			pattern:   "_1 == _1",
			target:    "[1].exists(x, x > 0) == [1].exists(y, y > 0)",
			wantMatch: true,
		},
		{
			name:      "repeated slot with alpha-equivalent comprehensions (disabled by caller)",
			pattern:   "_1 == _1",
			target:    "[1].exists(x, x > 0) == [1].exists(y, y > 0)",
			equivOpts: []ast.EquivOption{ast.EquivIgnoreIdentifiers(false)},
			wantMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := matcher.MustCompile(tc.pattern)
			tgt := testAST(t, tc.target, tc.envOpts...)
			res, matched := pat.Match(tgt, tc.equivOpts...)
			if matched != tc.wantMatch {
				t.Fatalf("pat.Match(%s) = %v, want %v", tc.target, matched, tc.wantMatch)
			}
			if matched && tc.checkSlots != nil {
				tc.checkSlots(t, res)
			}
		})
	}
}

func TestFindAll(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		target    string
		opts      []matcher.MatchOption
		wantCount int
		check     func(t *testing.T, results []matcher.MatchResult)
	}{
		{
			name:      "find all overlapping matches",
			pattern:   "_1 + 0",
			target:    "((a + 0) * (b + 0)) + 0",
			wantCount: 3,
		},
		{
			name:      "find disjoint matches",
			pattern:   "_1 + 0",
			target:    "((a + 0) * (b + 0)) + 0",
			opts:      []matcher.MatchOption{matcher.MatchDisjoint(true)},
			wantCount: 1,
		},
		{
			name:      "find with max depth",
			pattern:   "_1 + 0",
			target:    "((a + 0) * (b + 0)) + 0",
			opts:      []matcher.MatchOption{matcher.MatchMaxDepth(1)},
			wantCount: 1,
		},
		{
			name:      "contextual navigation through Parent",
			pattern:   "_1 + 0",
			target:    "foo((a + 0) * (b + 0))",
			wantCount: 2,
			check: func(t *testing.T, results []matcher.MatchResult) {
				for _, res := range results {
					e1, ok := res.FirstExpr(1)
					if !ok || e1 == nil {
						t.Fatalf("expected FirstExpr(1) to be non-nil")
					}
					parent, ok := e1.Parent()
					if !ok || parent == nil {
						t.Errorf("expected captured node to have a parent")
					}
					if parent.Kind() != ast.CallKind || parent.AsCall().FunctionName() != "_+_" {
						t.Errorf("expected parent to be '+', got %v", parent)
					}
					grandparent, ok := parent.Parent()
					if !ok || grandparent == nil {
						t.Errorf("expected parent node to have a parent")
					}
					if grandparent.Kind() != ast.CallKind || grandparent.AsCall().FunctionName() != "_*_" {
						t.Errorf("expected grandparent to be '*', got %v", grandparent)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := matcher.MustCompile(tc.pattern)
			tgt := testAST(t, tc.target)
			results := pat.FindAll(tgt, tc.opts...)
			if len(results) != tc.wantCount {
				t.Fatalf("pat.FindAll() returned %d matches, want %d", len(results), tc.wantCount)
			}
			if tc.check != nil {
				tc.check(t, results)
			}

			// FindAllExpr check
			exprResults := pat.FindAllExpr(tgt.Expr(), tc.opts...)
			if len(exprResults) != tc.wantCount {
				t.Fatalf("pat.FindAllExpr() returned %d matches, want %d", len(exprResults), tc.wantCount)
			}
		})
	}
}

func TestMatchAll(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		target     string
		stopEarly  bool
		wantVisits int
	}{
		{
			name:       "visit all matches",
			pattern:    "_1 + 0",
			target:     "((a + 0) * (b + 0)) + 0",
			stopEarly:  false,
			wantVisits: 3,
		},
		{
			name:       "early termination after first match",
			pattern:    "_1 + 0",
			target:     "((a + 0) * (b + 0)) + 0",
			stopEarly:  true,
			wantVisits: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := matcher.MustCompile(tc.pattern)
			tgt := testAST(t, tc.target)
			visits := 0
			pat.MatchAll(tgt, func(m matcher.MatchResult) bool {
				visits++
				return !tc.stopEarly
			})
			if visits != tc.wantVisits {
				t.Errorf("MatchAll visited %d nodes, want %d", visits, tc.wantVisits)
			}

			exprVisits := 0
			pat.MatchAllExpr(tgt.Expr(), func(m matcher.MatchResult) bool {
				exprVisits++
				return !tc.stopEarly
			})
			if exprVisits != tc.wantVisits {
				t.Errorf("MatchAllExpr visited %d nodes, want %d", exprVisits, tc.wantVisits)
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"non-trailing star in call", "fn(_1.star(), _2)"},
		{"non-trailing plus in call", "fn(_1.plus(), _2)"},
		{"non-trailing optional in call", "fn(_1.optional(), _2)"},
		{"non-trailing star in list", "[_1.star(), _2]"},
		{"invalid kind in exprKind", "_1.exprKind(invalidKind)"},
		{"extra arg to exprKind for non-struct", "_1.exprKind(ident, extra)"},
		{"duplicate exprKind", "_1.exprKind(ident).exprKind(call)"},
		{"duplicate type", "_1.type(int).type(string)"},
		{"duplicate star", "fn(_1.star().star())"},
		{"duplicate optional", "fn(_1.optional().optional())"},
		{"conflicting quantifiers star and plus", "fn(_1.star().plus())"},
		{"duplicate atLeast", "fn(_1.atLeast(1).atLeast(2))"},
		{"duplicate atMost", "fn(_1.atMost(2).atMost(3))"},
		{"contradictory bounds atLeast > atMost", "fn(_1.atLeast(5).atMost(3))"},
		{"contradictory bounds atMost < atLeast", "fn(_1.atMost(2).atLeast(4))"},
		{"negative atLeast", "_1.atLeast(-1)"},
		{"negative atMost", "_1.atMost(-1)"},
		{"non-literal atLeast", "_1.atLeast(x)"},
		{"non-literal atMost", "_1.atMost(x)"},
		{"no-arg atLeast", "_1.atLeast()"},
		{"extra-arg atLeast", "_1.atLeast(1, 2)"},
		{"no-arg atMost", "_1.atMost()"},
		{"extra-arg atMost", "_1.atMost(1, 2)"},
		{"no-arg exprKind", "_1.exprKind()"},
		{"invalid kind call in exprKind", "_1.exprKind(call(1))"},
		{"no-arg type", "_1.type()"},
		{"invalid type call in type", "_1.type(fn(1))"},
		{"unknown qualified type name", "_1.type(no.such.Type)"},
		{"misspelled primitive type name", "_1.type(itn)"},
		{"unregistered message type name", "_1.type(dev.cel.testing.Missing)"},
		{"invalid struct type call in exprKind", "_1.exprKind(struct, fn(1))"},
		{"parse syntax error", "1 + *"},
		{"invalid slot in map key", "{_1.atLeast(-1): 'b'}"},
		{"invalid slot in map val", "{'a': _1.atLeast(-1)}"},
		{"invalid slot in struct field", "google.protobuf.Duration{seconds: _1.atLeast(-1)}"},
		{"invalid slot in select operand", "_1.atLeast(-1).field"},
		{"slot index out of range _10", "fn(_10)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := matcher.Compile(tc.pattern)
			if err == nil {
				t.Errorf("expected Compile(%q) to fail, but succeeded", tc.pattern)
			}
		})
	}
}

func TestCompileOptions(t *testing.T) {
	reg, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}

	pat, err := matcher.Compile("_1 + 0",
		matcher.TypeProvider(reg),
		matcher.ParserOptions(parser.EnableOptionalSyntax(true)),
	)
	if err != nil {
		t.Fatalf("Compile with options failed: %v", err)
	}
	if pat.AST() == nil {
		t.Errorf("expected pat.AST() to be non-nil")
	}

	// MustCompile panic branch
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected MustCompile to panic on invalid pattern")
		}
	}()
	matcher.MustCompile("1 + *")
}

func TestMatchResultAPI(t *testing.T) {
	pat := matcher.MustCompile("_1 + _2")
	tgt := testAST(t, "a + b")
	res, ok := pat.Match(tgt)
	if !ok || !res.Matched() {
		t.Fatalf("expected Match to succeed and Matched() to be true")
	}

	if e1, ok := res.FirstExpr(1); !ok || e1 == nil {
		t.Errorf("expected FirstExpr(1) to succeed")
	}
	if exprs, ok := res.Exprs(1); !ok || len(exprs) != 1 {
		t.Errorf("expected Exprs(1) to return slice of length 1")
	}

	if _, ok := res.FirstExpr(99); ok {
		t.Errorf("expected FirstExpr(99) to fail for unbound slot")
	}
	if _, ok := res.Exprs(99); ok {
		t.Errorf("expected Exprs(99) to fail for unbound slot")
	}

	unmatchedRes, ok := pat.Match(testAST(t, "1 * 2"))
	if ok || unmatchedRes != nil {
		t.Errorf("expected unmatched Match to return nil, false")
	}
}

func TestMatchExprAndNilSafety(t *testing.T) {
	pat := matcher.MustCompile("_1 + 0")
	tgt := testAST(t, "a + 0")

	// Nil safety checks
	if _, ok := pat.Match(nil); ok {
		t.Errorf("pat.Match(nil) should return false")
	}
	if _, ok := pat.MatchExpr(nil); ok {
		t.Errorf("pat.MatchExpr(nil) should return false")
	}
	if res := pat.FindAll(nil); res != nil {
		t.Errorf("pat.FindAll(nil) should return nil")
	}
	if res := pat.FindAllExpr(nil); res != nil {
		t.Errorf("pat.FindAllExpr(nil) should return nil")
	}
	pat.MatchAll(nil, func(m matcher.MatchResult) bool {
		t.Errorf("MatchAll(nil) should not invoke handler")
		return true
	})
	pat.MatchAllExpr(nil, func(m matcher.MatchResult) bool {
		t.Errorf("MatchAllExpr(nil) should not invoke handler")
		return true
	})

	// MatchExpr on ast.Expr and ast.NavigableExpr
	res, ok := pat.MatchExpr(tgt.Expr())
	if !ok || !res.Matched() {
		t.Errorf("pat.MatchExpr(tgt.Expr()) failed")
	}

	nav := ast.NavigateAST(tgt)
	res2, ok := pat.MatchExpr(nav)
	if !ok || !res2.Matched() {
		t.Errorf("pat.MatchExpr(nav) failed")
	}

	// MatchExpr with type on NavigableExpr
	typePat := matcher.MustCompile("_1.type(int)")
	checkedTgt := testAST(t, "10", cel.Variable("x", cel.IntType))
	checkedNav := ast.NavigateAST(checkedTgt)
	if _, ok := typePat.MatchExpr(checkedNav); !ok {
		t.Errorf("typePat.MatchExpr(checkedNav) failed")
	}
}

func TestOptionsCoverage(t *testing.T) {
	pat := matcher.MustCompile("_1 + 0")
	tgt := testAST(t, "((a + 0) * (b + 0)) + 0")

	// MatchDisjoint with default true (no args)
	resDisjoint := pat.FindAll(tgt, matcher.MatchDisjoint())
	if len(resDisjoint) != 1 {
		t.Errorf("FindAll with MatchDisjoint() returned %d, want 1", len(resDisjoint))
	}

	// MatchDisjoint(false)
	resNotDisjoint := pat.FindAll(tgt, matcher.MatchDisjoint(false))
	if len(resNotDisjoint) != 3 {
		t.Errorf("FindAll with MatchDisjoint(false) returned %d, want 3", len(resNotDisjoint))
	}

	// MatchEquivOptions in FindAll
	resEquiv := pat.FindAll(tgt, matcher.MatchEquivOptions(ast.EquivIgnoreIdentifiers(true)))
	if len(resEquiv) != 3 {
		t.Errorf("FindAll with MatchEquivOptions returned %d, want 3", len(resEquiv))
	}
}

func TestStructuralMatching(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		target    string
		envOpts   []cel.EnvOption
		wantMatch bool
	}{
		// Select matching and mismatch
		{"select match", "_1.field", "a.field", nil, true},
		{"select field mismatch", "_1.fieldA", "a.fieldB", nil, false},

		// Member call target mismatch
		{"member call receiver mismatch", "a.foo()", "b.foo()", nil, false},

		// Map pattern and matching
		{"map pattern matching", "{\"k\": _1}", "{\"k\": 10}", nil, true},
		{"map length mismatch", "{\"k\": _1}", "{\"k\": 10, \"k2\": 20}", nil, false},
		{"map key mismatch", "{\"k1\": _1}", "{\"k2\": 10}", nil, false},
		{"map value mismatch", "{\"k\": 1}", "{\"k\": 2}", nil, false},

		// Struct pattern and matching
		{"struct matching", "google.protobuf.Duration{seconds: _1}", "google.protobuf.Duration{seconds: 10}", []cel.EnvOption{cel.Types(types.DurationType)}, true},
		{"struct field value mismatch", "google.protobuf.Duration{seconds: 1}", "google.protobuf.Duration{seconds: 2}", []cel.EnvOption{cel.Types(types.DurationType)}, false},
		{"struct field name mismatch", "google.protobuf.Duration{seconds: _1}", "google.protobuf.Duration{nanos: 10}", []cel.EnvOption{cel.Types(types.DurationType)}, false},
		{"struct type mismatch", "google.protobuf.Duration{seconds: 10}", "google.protobuf.Timestamp{seconds: 10}", []cel.EnvOption{cel.Types(types.DurationType, types.TimestampType)}, false},
		{"struct field count mismatch", "google.protobuf.Duration{seconds: 10}", "google.protobuf.Duration{seconds: 10, nanos: 20}", []cel.EnvOption{cel.Types(types.DurationType)}, false},

		// exprKind variants
		{"exprKind literal", "_1.exprKind(literal)", "100", nil, true},
		{"exprKind comprehension", "_1.exprKind(comprehension)", "[1].exists(x, x > 0)", nil, true},
		{"exprKind list", "_1.exprKind(list)", "[1, 2]", nil, true},
		{"exprKind map", "_1.exprKind(map)", "{'a': 1}", nil, true},
		{"exprKind struct", "_1.exprKind(struct)", "google.protobuf.Duration{seconds: 10}", []cel.EnvOption{cel.Types(types.DurationType)}, true},

		// type protobuf struct type
		{"type proto struct type match", "_1.type(google.protobuf.Duration)", "d", []cel.EnvOption{cel.Variable("d", cel.DurationType)}, true},
		{"type proto struct type mismatch", "_1.type(google.protobuf.Duration)", "t", []cel.EnvOption{cel.Variable("t", cel.TimestampType)}, false},

		// Sequence matching edge cases
		{"empty args call match", "fn()", "fn()", nil, true},
		{"empty args call mismatch", "fn()", "fn(1)", nil, false},
		{"fixed sequence element mismatch", "fn(_1, 10)", "fn('a', 20)", nil, false},
		{"trailing variable sequence element mismatch", "fn(_1, _2.star())", "fn(1, 2, 3)", nil, true},
		{"trailing variable sequence leading element mismatch", "fn(10, _1.star())", "fn(20, 1, 2)", nil, false},
		{"repeated slot sequence length mismatch", "[_1.repeated(2), _1.repeated(1)]", "[1, 2, 3]", nil, false},
		{"repeated slot sequence element mismatch", "[_1.repeated(2), _1.repeated(2)]", "[1, 2, 1, 3]", nil, false},
		{"trailing variable max occurs exceeded", "fn(1, _1.atMost(2))", "fn(1, 2, 3, 4)", nil, false},
		{"trailing variable not enough leading elements", "fn(1, 2, 3, _1.star())", "fn(1, 2)", nil, false},

		// Two variable comprehension matching
		{"two var comprehension macro", "[1].exists_one(x, x > 0)", "[1].exists_one(y, y > 0)", nil, true},
		{"comprehension structural mismatch", "[1].all(x, x > 0)", "[2].all(x, x > 0)", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := matcher.MustCompile(tc.pattern)
			tgt := testAST(t, tc.target, tc.envOpts...)
			_, matched := pat.Match(tgt)
			if matched != tc.wantMatch {
				t.Fatalf("pat.Match(%s) = %v, want %v", tc.target, matched, tc.wantMatch)
			}
		})
	}
}

func TestTwoVarComprehensions(t *testing.T) {
	twoVarOpt := ext.TwoVarComprehensions()

	tests := []struct {
		name       string
		pattern    string
		target     string
		equivOpts  []ast.EquivOption
		wantMatch  bool
		checkSlots func(t *testing.T, res matcher.MatchResult)
	}{
		{
			name:      "all 2-var macro matching with slot capture and alpha equivalence",
			pattern:   "_1.all(k, v, k != v)",
			target:    "{'a': 'b', 'c': 'd'}.all(key, val, key != val)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.MapKind {
					t.Fatalf("expected _1 to be map, got %v", e1)
				}
			},
		},
		{
			name:      "exists 2-var macro matching with slot capture",
			pattern:   "_1.exists(i, v, v == _2)",
			target:    "[10, 20, 30].exists(idx, elem, elem == 20)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e1, ok := res.FirstExpr(1)
				if !ok || e1.Kind() != ast.ListKind {
					t.Fatalf("expected _1 to be list, got %v", e1)
				}
				e2, ok := res.FirstExpr(2)
				if !ok || e2.AsLiteral().Value() != int64(20) {
					t.Fatalf("expected _2 to be 20, got %v", e2)
				}
			},
		},
		{
			name:      "exists_one 2-var macro matching",
			pattern:   "_1.exists_one(i, v, v > _2)",
			target:    "[5, 15, 25].exists_one(idx, val, val > 20)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e2, ok := res.FirstExpr(2)
				if !ok || e2.AsLiteral().Value() != int64(20) {
					t.Fatalf("expected _2 to be 20, got %v", e2)
				}
			},
		},
		{
			name:      "exists map 2-var macro matching",
			pattern:   "_1.exists(k, v, k == _2)",
			target:    "{'a': 1, 'b': 2}.exists(key, val, key == 'b')",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e2, ok := res.FirstExpr(2)
				if !ok || e2.AsLiteral().Value() != "b" {
					t.Fatalf("expected _2 to be 'b', got %v", e2)
				}
			},
		},
		{
			name:      "transformList 3-arg macro matching",
			pattern:   "_1.transformList(i, v, v + _2)",
			target:    "[1, 2, 3].transformList(idx, val, val + 10)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e2, ok := res.FirstExpr(2)
				if !ok || e2.AsLiteral().Value() != int64(10) {
					t.Fatalf("expected _2 to be 10, got %v", e2)
				}
			},
		},
		{
			name:      "transformList 4-arg macro matching with filter",
			pattern:   "_1.transformList(i, v, i > 0, v + _2)",
			target:    "[1, 2, 3].transformList(idx, val, idx > 0, val + 5)",
			wantMatch: true,
			checkSlots: func(t *testing.T, res matcher.MatchResult) {
				e2, ok := res.FirstExpr(2)
				if !ok || e2.AsLiteral().Value() != int64(5) {
					t.Fatalf("expected _2 to be 5, got %v", e2)
				}
			},
		},
		{
			name:      "transformMap 3-arg macro matching",
			pattern:   "_1.transformMap(k, v, v)",
			target:    "{'a': 'b'}.transformMap(key, val, val)",
			wantMatch: true,
		},
		{
			name:      "transformMap 4-arg macro matching with filter",
			pattern:   "_1.transformMap(k, v, k != 'a', v)",
			target:    "{'a': 'b'}.transformMap(key, val, key != 'a', val)",
			wantMatch: true,
		},
		{
			name:      "transformMapEntry 3-arg macro matching",
			pattern:   "_1.transformMapEntry(k, v, {v: k})",
			target:    "{'greeting': 'hello'}.transformMapEntry(key, val, {val: key})",
			wantMatch: true,
		},
		{
			name:      "transformMapEntry 4-arg macro matching with filter",
			pattern:   "_1.transformMapEntry(k, v, k != 'a', {v: k})",
			target:    "{'greeting': 'hello'}.transformMapEntry(key, val, key != 'a', {val: key})",
			wantMatch: true,
		},
		{
			name:      "2-var comprehension repeated slot alpha equivalent",
			pattern:   "_1 == _1",
			target:    "{'a': 1}.all(k, v, k != '') == {'a': 1}.all(x, y, x != '')",
			wantMatch: true,
		},
		{
			name:      "2-var comprehension repeated slot alpha equivalent disabled",
			pattern:   "_1 == _1",
			target:    "{'a': 1}.all(k, v, k != '') == {'a': 1}.all(x, y, x != '')",
			equivOpts: []ast.EquivOption{ast.EquivIgnoreIdentifiers(false)},
			wantMatch: false,
		},
		{
			name:      "1-var pattern vs 2-var target mismatch",
			pattern:   "[1].all(x, x > 0)",
			target:    "[1].all(i, x, x > 0)",
			wantMatch: false,
		},
		{
			name:      "2-var pattern vs 1-var target mismatch",
			pattern:   "[1].all(i, x, x > 0)",
			target:    "[1].all(x, x > 0)",
			wantMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			patAST := testParsedAST(t, tc.pattern, twoVarOpt)
			tgtAST := testParsedAST(t, tc.target, twoVarOpt)
			pat := matcher.MustCompileAST(patAST)
			res, matched := pat.Match(tgtAST, tc.equivOpts...)
			if matched != tc.wantMatch {
				t.Fatalf("pat.Match(%s) = %v, want %v", tc.target, matched, tc.wantMatch)
			}
			if matched && tc.checkSlots != nil {
				tc.checkSlots(t, res)
			}
		})
	}
}

func TestTwoVarComprehensionStructuralAST(t *testing.T) {
	fac := ast.NewExprFactory()

	// Build raw AST nodes without macro calls to exercise pure structural ComprehensionTwoVar matching
	iterRange1 := fac.NewList(1, []ast.Expr{fac.NewLiteral(2, types.Int(1))}, nil)
	iterRange2 := fac.NewList(10, []ast.Expr{fac.NewLiteral(11, types.Int(1))}, nil)
	iterRangeDiff := fac.NewList(20, []ast.Expr{fac.NewLiteral(21, types.Int(2))}, nil)

	accuInit1 := fac.NewLiteral(3, types.Int(0))
	accuInit2 := fac.NewLiteral(12, types.Int(0))

	cond1 := fac.NewLiteral(4, types.True)
	cond2 := fac.NewLiteral(13, types.True)

	// Step 1: k1 + v1
	step1 := fac.NewCall(5, "_+_", fac.NewIdent(6, "k1"), fac.NewIdent(7, "v1"))
	// Step 2: k2 + v2 (alpha-equivalent)
	step2 := fac.NewCall(14, "_+_", fac.NewIdent(15, "k2"), fac.NewIdent(16, "v2"))
	// Step Swapped: v2 + k2 (order swapped, not structurally identical args)
	stepSwapped := fac.NewCall(17, "_+_", fac.NewIdent(18, "v2"), fac.NewIdent(19, "k2"))

	res1 := fac.NewIdent(8, "@accu1")
	res2 := fac.NewIdent(20, "@accu2")

	comp1 := fac.NewComprehensionTwoVar(9, iterRange1, "k1", "v1", "@accu1", accuInit1, cond1, step1, res1)
	comp2Alpha := fac.NewComprehensionTwoVar(21, iterRange2, "k2", "v2", "@accu2", accuInit2, cond2, step2, res2)
	compSwappedVars := fac.NewComprehensionTwoVar(22, iterRange2, "k2", "v2", "@accu2", accuInit2, cond2, stepSwapped, res2)
	compDiffRange := fac.NewComprehensionTwoVar(23, iterRangeDiff, "k2", "v2", "@accu2", accuInit2, cond2, step2, res2)

	// 1-var comprehension
	compOneVar := fac.NewComprehension(24, iterRange1, "k1", "@accu1", accuInit1, cond1, step1, res1)

	// Pattern wrapping comp1
	patAST := ast.NewAST(comp1, ast.NewSourceInfo(nil))
	pat := matcher.MustCompileAST(patAST)

	// 1. Alpha-equivalent two-var comprehension matching
	if _, ok := pat.MatchExpr(comp2Alpha); !ok {
		t.Errorf("expected comp1 to match comp2Alpha, but it didn't")
	}

	// 2. Alpha-equivalent disabled -> should fail
	if _, ok := pat.MatchExpr(comp2Alpha, ast.EquivIgnoreIdentifiers(false)); ok {
		t.Errorf("expected comp1 to NOT match comp2Alpha when EquivIgnoreIdentifiers is false")
	}

	// 3. Swapped variable references in step -> should fail
	if _, ok := pat.MatchExpr(compSwappedVars); ok {
		t.Errorf("expected comp1 to NOT match compSwappedVars")
	}

	// 4. Mismatched range -> should fail
	if _, ok := pat.MatchExpr(compDiffRange); ok {
		t.Errorf("expected comp1 to NOT match compDiffRange")
	}

	// 5. Mismatched 1-var vs 2-var -> should fail
	if _, ok := pat.MatchExpr(compOneVar); ok {
		t.Errorf("expected comp1 (two-var) to NOT match compOneVar (one-var)")
	}

	// 6. Test MustCompileAST with nil ast panics
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected MustCompileAST(nil) to panic")
		}
	}()
	matcher.MustCompileAST(nil)
}

func TestMapEntryMatcher(t *testing.T) {
	pat := matcher.MustCompile("{_1.plus(): _2.plus()}")

	t.Run("empty map fails plus", func(t *testing.T) {
		tgt := testAST(t, "{}")
		_, matched := pat.Match(tgt)
		if matched {
			t.Errorf("expected empty map to fail plus quantifier")
		}
	})

	t.Run("single entry map", func(t *testing.T) {
		tgt := testAST(t, "{'a': 1}")
		res, matched := pat.Match(tgt)
		if !matched {
			t.Fatalf("expected single entry map to match")
		}
		keys, ok1 := res.Exprs(1)
		vals, ok2 := res.Exprs(2)
		if !ok1 || len(keys) != 1 || keys[0].AsLiteral().Value() != "a" {
			t.Errorf("unexpected keys: %v, ok: %v", keys, ok1)
		}
		if !ok2 || len(vals) != 1 || vals[0].AsLiteral().Value() != int64(1) {
			t.Errorf("unexpected vals: %v, ok: %v", vals, ok2)
		}
	})

	t.Run("multi entry map", func(t *testing.T) {
		tgt := testAST(t, "{'a': 1, 'b': 2, 'c': 3}")
		res, matched := pat.Match(tgt)
		if !matched {
			t.Fatalf("expected multi entry map to match")
		}
		keys, ok1 := res.Exprs(1)
		vals, ok2 := res.Exprs(2)
		if !ok1 || len(keys) != 3 {
			t.Errorf("expected 3 keys, got %d", len(keys))
		}
		if !ok2 || len(vals) != 3 {
			t.Errorf("expected 3 vals, got %d", len(vals))
		}
	})

	t.Run("star quantifier empty map", func(t *testing.T) {
		patStar := matcher.MustCompile("{_1.star(): _2.star()}")
		tgt := testAST(t, "{}")
		res, matched := patStar.Match(tgt)
		if !matched {
			t.Fatalf("expected star pattern to match empty map")
		}
		keys, ok1 := res.Exprs(1)
		vals, ok2 := res.Exprs(2)
		if !ok1 || len(keys) != 0 {
			t.Errorf("expected 0 keys, got %v, ok: %v", keys, ok1)
		}
		if !ok2 || len(vals) != 0 {
			t.Errorf("expected 0 vals, got %v, ok: %v", vals, ok2)
		}
	})

	t.Run("fixed prefix with trailing star", func(t *testing.T) {
		patPrefix := matcher.MustCompile("{'prefix': 0, _1.star(): _2.star()}")
		tgtMatch := testAST(t, "{'prefix': 0, 'a': 1, 'b': 2}")
		res, matched := patPrefix.Match(tgtMatch)
		if !matched {
			t.Fatalf("expected fixed prefix pattern to match")
		}
		keys, _ := res.Exprs(1)
		vals, _ := res.Exprs(2)
		if len(keys) != 2 || len(vals) != 2 {
			t.Errorf("expected 2 trailing keys and values, got %d keys, %d vals", len(keys), len(vals))
		}

		tgtPrefixOnly := testAST(t, "{'prefix': 0}")
		resOnly, matchedOnly := patPrefix.Match(tgtPrefixOnly)
		if !matchedOnly {
			t.Fatalf("expected fixed prefix only to match star pattern")
		}
		keysOnly, _ := resOnly.Exprs(1)
		if len(keysOnly) != 0 {
			t.Errorf("expected 0 trailing keys, got %d", len(keysOnly))
		}

		tgtMismatch := testAST(t, "{'other': 0, 'a': 1}")
		_, matchedMismatch := patPrefix.Match(tgtMismatch)
		if matchedMismatch {
			t.Errorf("expected prefix mismatch to fail")
		}
	})

	t.Run("fixed value with variable keys", func(t *testing.T) {
		patFixedVal := matcher.MustCompile("{_1.plus(): 0}")
		tgtAll0 := testAST(t, "{'a': 0, 'b': 0}")
		res, matched := patFixedVal.Match(tgtAll0)
		if !matched {
			t.Fatalf("expected all zeros to match")
		}
		keys, _ := res.Exprs(1)
		if len(keys) != 2 {
			t.Errorf("expected 2 keys, got %d", len(keys))
		}

		tgtNotAll0 := testAST(t, "{'a': 0, 'b': 1}")
		_, matchedNot0 := patFixedVal.Match(tgtNotAll0)
		if matchedNot0 {
			t.Errorf("expected mismatch when values are not all 0")
		}
	})

	t.Run("repeated quantifier in map", func(t *testing.T) {
		patRep := matcher.MustCompile("{_1.repeated(2): _2.repeated(2)}")
		tgt2 := testAST(t, "{'a': 1, 'b': 2}")
		res, matched := patRep.Match(tgt2)
		if !matched {
			t.Fatalf("expected repeated(2) to match 2 entries")
		}
		keys, _ := res.Exprs(1)
		vals, _ := res.Exprs(2)
		if len(keys) != 2 || len(vals) != 2 {
			t.Errorf("expected 2 keys and 2 vals, got %d and %d", len(keys), len(vals))
		}

		tgt1 := testAST(t, "{'a': 1}")
		_, matched1 := patRep.Match(tgt1)
		if matched1 {
			t.Errorf("expected repeated(2) to fail on 1 entry")
		}

		tgt3 := testAST(t, "{'a': 1, 'b': 2, 'c': 3}")
		_, matched3 := patRep.Match(tgt3)
		if matched3 {
			t.Errorf("expected repeated(2) to fail on 3 entries")
		}
	})

	t.Run("non-trailing variable entry fails compile", func(t *testing.T) {
		_, err := matcher.Compile("{_1.star(): _2.star(), 'trailing': 1}")
		if err == nil {
			t.Errorf("expected error for non-trailing variable map entry")
		}
	})
}

func TestListPlusMatcher(t *testing.T) {
	pat := matcher.MustCompile("[_1.plus()]")

	t.Run("empty list", func(t *testing.T) {
		tgt := testAST(t, "[]")
		_, matched := pat.Match(tgt)
		if matched {
			t.Errorf("expected empty list to fail plus quantifier")
		}
	})

	t.Run("single element list", func(t *testing.T) {
		tgt := testAST(t, "['hello']")
		res, matched := pat.Match(tgt)
		if !matched {
			t.Fatalf("expected single element list to match")
		}
		elems, ok := res.Exprs(1)
		if !ok || len(elems) != 1 || elems[0].AsLiteral().Value() != "hello" {
			t.Errorf("unexpected elements: %v, ok: %v", elems, ok)
		}
	})

	t.Run("multi element list", func(t *testing.T) {
		tgt := testAST(t, "['hello', 'world', '!']")
		res, matched := pat.Match(tgt)
		if !matched {
			t.Fatalf("expected multi element list to match")
		}
		elems, ok := res.Exprs(1)
		if !ok || len(elems) != 3 {
			t.Errorf("expected 3 elements, got %d, ok: %v", len(elems), ok)
		}
	})

	t.Run("star quantifier empty list", func(t *testing.T) {
		patStar := matcher.MustCompile("[_1.star()]")
		tgt := testAST(t, "[]")
		res, matched := patStar.Match(tgt)
		if !matched {
			t.Fatalf("expected star pattern to match empty list")
		}
		elems, ok := res.Exprs(1)
		if !ok || len(elems) != 0 {
			t.Errorf("expected 0 elements, got %d, ok: %v", len(elems), ok)
		}
	})
}

func TestComprehensionStepRewriteMatcher(t *testing.T) {
	t.Run("list filter step pattern", func(t *testing.T) {
		pat := matcher.MustCompile("_1 ? _2.exprKind(ident) + [_3] : _2")

		t.Run("matches filter comprehension step", func(t *testing.T) {
			// [1, 2, 3].filter(x, x > 0) expands to a comprehension whose LoopStep() is:
			// x > 0 ? __result__ + [x] : __result__
			targetAST := testAST(t, "[1, 2, 3].filter(x, x > 0)")
			comp := targetAST.Expr().AsComprehension()
			res, matched := pat.MatchExpr(comp.LoopStep())
			if !matched {
				t.Fatalf("expected filter loop step to match pattern")
			}

			cond, ok1 := res.FirstExpr(1)
			accu, ok2 := res.FirstExpr(2)
			elem, ok3 := res.FirstExpr(3)

			if !ok1 || cond == nil {
				t.Errorf("expected _1 (condition) to be captured")
			}
			if !ok2 || accu == nil || accu.Kind() != ast.IdentKind || accu.AsIdent() != comp.AccuVar() {
				t.Errorf("expected _2 (accumulator) to match %q, got %v", comp.AccuVar(), accu)
			}
			if !ok3 || elem == nil || elem.Kind() != ast.IdentKind || elem.AsIdent() != "x" {
				t.Errorf("expected _3 (element) to be 'x', got %v", elem)
			}

			// Verify rewrite to _2.insert(_3)
			fac := ast.NewExprFactory()
			rewrittenStep := fac.NewMemberCall(0, "insert", accu, elem)
			if rewrittenStep.Kind() != ast.CallKind || rewrittenStep.AsCall().FunctionName() != "insert" {
				t.Errorf("failed to rewrite step to insert: %v", rewrittenStep)
			}
			if !rewrittenStep.AsCall().IsMemberFunction() || rewrittenStep.AsCall().Target() != accu {
				t.Errorf("rewritten step target mismatch: %v", rewrittenStep)
			}
		})

		t.Run("matches map with filter comprehension step", func(t *testing.T) {
			// [1, 2, 3].map(x, x > 0, x * 2) expands to:
			// x > 0 ? __result__ + [x * 2] : __result__
			targetAST := testAST(t, "[1, 2, 3].map(x, x > 0, x * 2)")
			comp := targetAST.Expr().AsComprehension()
			res, matched := pat.MatchExpr(comp.LoopStep())
			if !matched {
				t.Fatalf("expected map with filter loop step to match pattern")
			}

			cond, ok1 := res.FirstExpr(1)
			accu, ok2 := res.FirstExpr(2)
			elem, ok3 := res.FirstExpr(3)

			if !ok1 || cond == nil || cond.Kind() != ast.CallKind || cond.AsCall().FunctionName() != "_>_" {
				t.Errorf("expected _1 to be 'x > 0', got %v", cond)
			}
			if !ok2 || accu == nil || accu.AsIdent() != comp.AccuVar() {
				t.Errorf("expected _2 to be accumulator %q, got %v", comp.AccuVar(), accu)
			}
			if !ok3 || elem == nil || elem.Kind() != ast.CallKind || elem.AsCall().FunctionName() != "_*_" {
				t.Errorf("expected _3 to be 'x * 2', got %v", elem)
			}

			// Verify rewrite
			fac := ast.NewExprFactory()
			rewrittenStep := fac.NewMemberCall(0, "insert", accu, elem)
			if rewrittenStep.AsCall().Args()[0] != elem {
				t.Errorf("rewritten step arg mismatch: %v", rewrittenStep)
			}
		})

		t.Run("mismatch when false branch does not match accumulator", func(t *testing.T) {
			targetAST := testAST(t, "x > 0 ? __result__ + [x] : other_var")
			_, matched := pat.Match(targetAST)
			if matched {
				t.Errorf("expected mismatch when false branch accumulator differs")
			}
		})

		t.Run("mismatch when step has no condition", func(t *testing.T) {
			targetAST := testAST(t, "[1, 2, 3].map(x, x * 2)")
			comp := targetAST.Expr().AsComprehension()
			_, matched := pat.MatchExpr(comp.LoopStep())
			if matched {
				t.Errorf("expected unconditional map loop step to not match ternary pattern")
			}
		})
	})

	t.Run("map insert step pattern", func(t *testing.T) {
		pat := matcher.MustCompile("_1 ? cel.`@mapInsert`(_2.exprKind(ident), {_3: _4}) : _2.exprKind(ident)")

		t.Run("matches transformMapEntry with filter comprehension step", func(t *testing.T) {
			// {'a': 1}.transformMapEntry(k, v, v > 0, {k: v + 1}) expands to:
			// v > 0 ? cel.@mapInsert(__result__, {k: v + 1}) : __result__
			targetAST := testAST(t, "{'a': 1}.transformMapEntry(k, v, v > 0, {k: v + 1})", ext.TwoVarComprehensions())
			comp := targetAST.Expr().AsComprehension()
			res, matched := pat.MatchExpr(comp.LoopStep())
			if !matched {
				t.Fatalf("expected transformMapEntry loop step to match pattern")
			}

			cond, ok1 := res.FirstExpr(1)
			accu, ok2 := res.FirstExpr(2)
			key, ok3 := res.FirstExpr(3)
			val, ok4 := res.FirstExpr(4)

			if !ok1 || cond == nil || cond.Kind() != ast.CallKind || cond.AsCall().FunctionName() != "_>_" {
				t.Errorf("expected _1 to be 'v > 0', got %v", cond)
			}
			if !ok2 || accu == nil || accu.Kind() != ast.IdentKind || accu.AsIdent() != comp.AccuVar() {
				t.Errorf("expected _2 to be accumulator %q, got %v", comp.AccuVar(), accu)
			}
			if !ok3 || key == nil || key.Kind() != ast.IdentKind || key.AsIdent() != "k" {
				t.Errorf("expected _3 to be key 'k', got %v", key)
			}
			if !ok4 || val == nil || val.Kind() != ast.CallKind || val.AsCall().FunctionName() != "_+_" {
				t.Errorf("expected _4 to be val 'v + 1', got %v", val)
			}

			// Verify rewrite to _2.insert(_3, _4)
			fac := ast.NewExprFactory()
			rewrittenStep := fac.NewMemberCall(0, "insert", accu, key, val)
			if rewrittenStep.Kind() != ast.CallKind || rewrittenStep.AsCall().FunctionName() != "insert" {
				t.Errorf("failed to rewrite step to insert: %v", rewrittenStep)
			}
			if len(rewrittenStep.AsCall().Args()) != 2 || rewrittenStep.AsCall().Args()[0] != key || rewrittenStep.AsCall().Args()[1] != val {
				t.Errorf("rewritten step args mismatch: %v", rewrittenStep)
			}
		})

		t.Run("mismatch when false branch accumulator differs", func(t *testing.T) {
			targetAST := testAST(t, "v > 0 ? cel.`@mapInsert`(__result__, {k: v}) : other_accu")
			_, matched := pat.Match(targetAST)
			if matched {
				t.Errorf("expected mismatch when false branch accumulator differs")
			}
		})

		t.Run("mismatch when map insert has no condition", func(t *testing.T) {
			targetAST := testAST(t, "{'a': 1}.transformMapEntry(k, v, {k: v + 1})", ext.TwoVarComprehensions())
			comp := targetAST.Expr().AsComprehension()
			_, matched := pat.MatchExpr(comp.LoopStep())
			if matched {
				t.Errorf("expected unconditional transformMapEntry to not match ternary pattern")
			}
		})
	})
}

func BenchmarkLoopStepExtraction(b *testing.B) {
	// List filter loop step pattern
	listPat := matcher.MustCompile("_1 ? _2.exprKind(ident) + [_3] : _2")
	filterAST := testAST(b, "[1, 2, 3].filter(x, x > 0)")
	filterStep := filterAST.Expr().AsComprehension().LoopStep()
	mapFilterAST := testAST(b, "[1, 2, 3].map(x, x > 0, x * 2)")
	mapFilterStep := mapFilterAST.Expr().AsComprehension().LoopStep()
	uncondMapAST := testAST(b, "[1, 2, 3].map(x, x * 2)")
	uncondMapStep := uncondMapAST.Expr().AsComprehension().LoopStep()

	// Map insert loop step pattern
	mapPat := matcher.MustCompile("_1 ? cel.`@mapInsert`(_2.exprKind(ident), {_3: _4}) : _2.exprKind(ident)")
	mapEntryFilterAST := testAST(b, "{'a': 1}.transformMapEntry(k, v, v > 0, {k: v + 1})", ext.TwoVarComprehensions())
	mapEntryFilterStep := mapEntryFilterAST.Expr().AsComprehension().LoopStep()
	mapEntryUncondAST := testAST(b, "{'a': 1}.transformMapEntry(k, v, {k: v + 1})", ext.TwoVarComprehensions())
	mapEntryUncondStep := mapEntryUncondAST.Expr().AsComprehension().LoopStep()

	b.Run("ListFilter/MatchExpr", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := listPat.MatchExpr(filterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match")
			}
		}
	})

	b.Run("ListFilter/MatchAndExtract", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := listPat.MatchExpr(filterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match")
			}
			cond, ok1 := res.FirstRawExpr(1)
			accu, ok2 := res.FirstRawExpr(2)
			elem, ok3 := res.FirstRawExpr(3)
			if !ok1 || !ok2 || !ok3 || cond == nil || accu == nil || elem == nil {
				b.Fatalf("failed to extract slots")
			}
		}
	})

	b.Run("ListFilter/MapWithFilter/MatchExpr", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := listPat.MatchExpr(mapFilterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match")
			}
		}
	})

	b.Run("ListFilter/Mismatch/MatchExpr", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, ok := listPat.MatchExpr(uncondMapStep)
			if ok {
				b.Fatalf("expected mismatch")
			}
		}
	})

	b.Run("ListFilter/FindAllAST", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			results := listPat.FindAll(filterAST)
			if len(results) != 1 {
				b.Fatalf("expected 1 match, got %d", len(results))
			}
		}
	})

	b.Run("MapInsert/MatchExpr", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := mapPat.MatchExpr(mapEntryFilterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match")
			}
		}
	})

	b.Run("MapInsert/MatchAndExtract", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := mapPat.MatchExpr(mapEntryFilterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match")
			}
			cond, ok1 := res.FirstRawExpr(1)
			accu, ok2 := res.FirstRawExpr(2)
			key, ok3 := res.FirstRawExpr(3)
			val, ok4 := res.FirstRawExpr(4)
			if !ok1 || !ok2 || !ok3 || !ok4 || cond == nil || accu == nil || key == nil || val == nil {
				b.Fatalf("failed to extract slots")
			}
		}
	})

	b.Run("MapInsert/Mismatch/MatchExpr", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, ok := mapPat.MatchExpr(mapEntryUncondStep)
			if ok {
				b.Fatalf("expected mismatch")
			}
		}
	})

	b.Run("MapInsert/FindAllAST", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			results := mapPat.FindAll(mapEntryFilterAST)
			if len(results) != 1 {
				b.Fatalf("expected 1 match, got %d", len(results))
			}
		}
	})

	b.Run("Compile/ListFilterPattern", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := matcher.Compile("_1 ? _2.exprKind(ident) + [_3] : _2")
			if err != nil {
				b.Fatalf("compile failed: %v", err)
			}
		}
	})

	b.Run("Compile/MapInsertPattern", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := matcher.Compile("_1 ? cel.`@mapInsert`(_2.exprKind(ident), {_3: _4}) : _2.exprKind(ident)")
			if err != nil {
				b.Fatalf("compile failed: %v", err)
			}
		}
	})
}

func BenchmarkPlanTimeComprehensionInsertion(b *testing.B) {
	fac := ast.NewExprFactory()

	// 1. List Comprehension: [1, 2, 3].filter(x, x > 0)
	listFilterPattern := matcher.MustCompile("_1 ? _2.exprKind(ident) + [_3] : _2")
	filterAST := testAST(b, "[1, 2, 3].filter(x, x > 0)")
	filterComp := filterAST.Expr().AsComprehension()
	filterStep := filterComp.LoopStep()

	// 2. Map-with-Filter Comprehension: [1, 2, 3].map(x, x > 0, x * 2)
	mapFilterAST := testAST(b, "[1, 2, 3].map(x, x > 0, x * 2)")
	mapFilterComp := mapFilterAST.Expr().AsComprehension()
	mapFilterStep := mapFilterComp.LoopStep()

	// 3. Map Entry with Filter Comprehension: {'a': 1}.transformMapEntry(k, v, v > 0, {k: v + 1})
	mapInsertPattern := matcher.MustCompile("_1 ? cel.`@mapInsert`(_2.exprKind(ident), {_3: _4}) : _2.exprKind(ident)")
	mapEntryFilterAST := testAST(b, "{'a': 1}.transformMapEntry(k, v, v > 0, {k: v + 1})", ext.TwoVarComprehensions())
	mapEntryFilterComp := mapEntryFilterAST.Expr().AsComprehension()
	mapEntryFilterStep := mapEntryFilterComp.LoopStep()

	// 4. Nested Comprehension in filter condition: [1, 2, 3].filter(x, [4, 5, 6].exists(y, y > x))
	// Demonstrates that matching stops at the outer comprehension loop step without descending into nested sub-comprehensions
	nestedFilterAST := testAST(b, "[1, 2, 3].filter(x, [4, 5, 6].exists(y, y > x))")
	nestedFilterComp := nestedFilterAST.Expr().AsComprehension()
	nestedFilterStep := nestedFilterComp.LoopStep()

	// 5. Unconditional loop steps (negative/mismatch fast-path)
	uncondMapAST := testAST(b, "[1, 2, 3].map(x, x * 2)")
	uncondMapComp := uncondMapAST.Expr().AsComprehension()
	uncondMapStep := uncondMapComp.LoopStep()

	uncondMapEntryAST := testAST(b, "{'a': 1}.transformMapEntry(k, v, {k: v + 1})", ext.TwoVarComprehensions())
	uncondMapEntryComp := uncondMapEntryAST.Expr().AsComprehension()
	uncondMapEntryStep := uncondMapEntryComp.LoopStep()

	b.Run("ListFilter/PlanAndExtractCriteria", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := listFilterPattern.MatchExpr(filterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match on list filter loop step")
			}
			cond, ok1 := res.FirstRawExpr(1)
			accu, ok2 := res.FirstRawExpr(2)
			elem, ok3 := res.FirstRawExpr(3)
			if !ok1 || !ok2 || !ok3 || cond == nil || accu == nil || elem == nil {
				b.Fatalf("failed to extract criteria")
			}
			if accu.Kind() != ast.IdentKind || accu.AsIdent() != filterComp.AccuVar() {
				b.Fatalf("accumulator mismatch")
			}
			_ = fac.NewMemberCall(0, "insert", accu, elem)
		}
	})

	b.Run("MapWithFilter/PlanAndExtractCriteria", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := listFilterPattern.MatchExpr(mapFilterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match on map-with-filter loop step")
			}
			cond, ok1 := res.FirstRawExpr(1)
			accu, ok2 := res.FirstRawExpr(2)
			elem, ok3 := res.FirstRawExpr(3)
			if !ok1 || !ok2 || !ok3 || cond == nil || accu == nil || elem == nil {
				b.Fatalf("failed to extract criteria")
			}
			if accu.Kind() != ast.IdentKind || accu.AsIdent() != mapFilterComp.AccuVar() {
				b.Fatalf("accumulator mismatch")
			}
			_ = fac.NewMemberCall(0, "insert", accu, elem)
		}
	})

	b.Run("MapInsert/PlanAndExtractCriteria", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := mapInsertPattern.MatchExpr(mapEntryFilterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match on map insert loop step")
			}
			cond, ok1 := res.FirstRawExpr(1)
			accu, ok2 := res.FirstRawExpr(2)
			key, ok3 := res.FirstRawExpr(3)
			val, ok4 := res.FirstRawExpr(4)
			if !ok1 || !ok2 || !ok3 || !ok4 || cond == nil || accu == nil || key == nil || val == nil {
				b.Fatalf("failed to extract criteria")
			}
			if accu.Kind() != ast.IdentKind || accu.AsIdent() != mapEntryFilterComp.AccuVar() {
				b.Fatalf("accumulator mismatch")
			}
			_ = fac.NewMemberCall(0, "insert", accu, key, val)
		}
	})

	b.Run("NestedComprehension/OuterLoopStepMatchOnly", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res, ok := listFilterPattern.MatchExpr(nestedFilterStep)
			if !ok || !res.Matched() {
				b.Fatalf("expected match on outer loop step")
			}
			cond, ok1 := res.FirstRawExpr(1)
			accu, ok2 := res.FirstRawExpr(2)
			elem, ok3 := res.FirstRawExpr(3)
			if !ok1 || !ok2 || !ok3 || cond == nil || accu == nil || elem == nil {
				b.Fatalf("failed to extract criteria")
			}
			if accu.Kind() != ast.IdentKind || accu.AsIdent() != nestedFilterComp.AccuVar() {
				b.Fatalf("accumulator mismatch")
			}
			_ = fac.NewMemberCall(0, "insert", accu, elem)
		}
	})

	b.Run("ListMismatch/FastPathNoAlloc", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, ok := listFilterPattern.MatchExpr(uncondMapStep)
			if ok {
				b.Fatalf("expected mismatch on unconditional step")
			}
		}
	})

	b.Run("MapMismatch/FastPathNoAlloc", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, ok := mapInsertPattern.MatchExpr(uncondMapEntryStep)
			if ok {
				b.Fatalf("expected mismatch on unconditional map entry step")
			}
		}
	})
}

// testASTNoMacroCalls parses an expression without macro call tracking, forcing
// comprehension patterns down the structural matching path.
func testASTNoMacroCalls(t testing.TB, src string) *ast.AST {
	t.Helper()
	prs, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
		parser.EnablePrattParser(true),
	)
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	parsed, iss := prs.Parse(common.NewTextSource(src))
	if iss != nil && len(iss.GetErrors()) > 0 {
		t.Fatalf("prs.Parse(%q) failed: %v", src, iss.ToDisplayString())
	}
	return parsed
}

func TestTypeConstraintNameResolution(t *testing.T) {
	customReg, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}
	if err := customReg.RegisterType(types.NewOpaqueType("dev.cel.Custom")); err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}

	tests := []struct {
		name    string
		pattern string
		opts    []matcher.CompileOption
		wantErr bool
	}{
		{name: "primitive type", pattern: "_1.type(int)"},
		{name: "qualified well-known type", pattern: "_1.type(google.protobuf.Duration)"},
		{name: "type known to the configured provider",
			pattern: "_1.type(dev.cel.Custom)",
			opts:    []matcher.CompileOption{matcher.TypeProvider(customReg)}},
		{name: "misspelled primitive type", pattern: "_1.type(itn)", wantErr: true},
		{name: "unknown qualified type", pattern: "_1.type(no.such.Type)", wantErr: true},
		{name: "type unknown to the configured provider",
			pattern: "_1.type(dev.cel.Other)",
			opts:    []matcher.CompileOption{matcher.TypeProvider(customReg)},
			wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := matcher.Compile(tc.pattern, tc.opts...)
			if tc.wantErr && err == nil {
				t.Fatalf("Compile(%q) succeeded, expected an unknown type error", tc.pattern)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Compile(%q) failed: %v", tc.pattern, err)
			}
		})
	}
}

func TestIgnoreIdentifiersOptionHonored(t *testing.T) {
	strict := []ast.EquivOption{ast.EquivIgnoreIdentifiers(false)}

	tests := []struct {
		name       string
		pattern    string
		target     string
		macroCalls bool
		equivOpts  []ast.EquivOption
		wantMatch  bool
	}{
		{
			name:       "macro call path unifies iteration variables by default",
			pattern:    "_1.all(x, x > 0)",
			target:     "items.all(it, it > 0)",
			macroCalls: true,
			wantMatch:  true,
		},
		{
			name:       "macro call path honors strict identifiers",
			pattern:    "_1.all(x, x > 0)",
			target:     "items.all(it, it > 0)",
			macroCalls: true,
			equivOpts:  strict,
			wantMatch:  false,
		},
		{
			name:       "macro call path matches identical iteration variables when strict",
			pattern:    "_1.all(x, x > 0)",
			target:     "items.all(x, x > 0)",
			macroCalls: true,
			equivOpts:  strict,
			wantMatch:  true,
		},
		{
			name:      "structural path unifies iteration variables by default",
			pattern:   "_1.all(x, x > 0)",
			target:    "items.all(it, it > 0)",
			wantMatch: true,
		},
		{
			name:      "structural path honors strict identifiers",
			pattern:   "_1.all(x, x > 0)",
			target:    "items.all(it, it > 0)",
			equivOpts: strict,
			wantMatch: false,
		},
		{
			name:      "structural path matches identical iteration variables when strict",
			pattern:   "_1.all(x, x > 0)",
			target:    "items.all(x, x > 0)",
			equivOpts: strict,
			wantMatch: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := matcher.MustCompile(tc.pattern)
			var tgt *ast.AST
			if tc.macroCalls {
				tgt = testAST(t, tc.target)
			} else {
				tgt = testASTNoMacroCalls(t, tc.target)
			}
			if _, matched := pat.Match(tgt, tc.equivOpts...); matched != tc.wantMatch {
				t.Errorf("pat.Match(%q) = %v, want %v", tc.target, matched, tc.wantMatch)
			}
		})
	}
}

func TestMatchRootMismatchIsAllocationFree(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		target  string
	}{
		{name: "call pattern against list", pattern: "_1 + 0", target: "[1, 2]"},
		{name: "call pattern against ident", pattern: "_1 + 0", target: "x"},
		{name: "list pattern against map", pattern: "[_1.star()]", target: "{'a': 1}"},
		{name: "struct pattern against literal", pattern: "_1.exprKind(struct)", target: "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pat := matcher.MustCompile(tc.pattern)
			tgt := testAST(t, tc.target)
			if _, matched := pat.Match(tgt); matched {
				t.Fatalf("pat.Match(%q) matched, want mismatch", tc.target)
			}
			allocs := testing.AllocsPerRun(100, func() {
				pat.Match(tgt)
			})
			if allocs != 0 {
				t.Errorf("pat.Match(%q) allocated %v times on a root mismatch, want 0", tc.target, allocs)
			}
		})
	}
}
