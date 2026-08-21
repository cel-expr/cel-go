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
	"strings"
	"testing"

	"cel.dev/cel-go/common"
)

func TestPrattParserSourceInfoPositions(t *testing.T) {
	src := common.NewTextSource("a + b")
	p, err := NewPrattParser()
	if err != nil {
		t.Fatalf("NewPrattParser() failed: %v", err)
	}
	parsed, errs := p.Parse(src)
	if len(errs.GetErrors()) > 0 {
		t.Fatalf("Parse() failed: %s", errs.ToDisplayString())
	}
	sourceInfo := parsed.SourceInfo()
	root := parsed.Expr()
	if sourceInfo.GetStartLocation(root.ID()).Column() != 2 {
		t.Errorf("expected root column 2, got %d", sourceInfo.GetStartLocation(root.ID()).Column())
	}
	args := root.AsCall().Args()
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if sourceInfo.GetStartLocation(args[0].ID()).Column() != 0 {
		t.Errorf("expected arg[0] column 0, got %d", sourceInfo.GetStartLocation(args[0].ID()).Column())
	}
	if sourceInfo.GetStartLocation(args[1].ID()).Column() != 4 {
		t.Errorf("expected arg[1] column 4, got %d", sourceInfo.GetStartLocation(args[1].ID()).Column())
	}
}

func TestPrattParserRecursionDepth(t *testing.T) {
	t.Run("DeeplyNestedBracketsLimitExceeded", func(t *testing.T) {
		p, err := NewPrattParser(MaxRecursionDepth(5))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("[[[[[[1]]]]]]"))
		if len(errs.GetErrors()) == 0 {
			t.Errorf("expected recursion limit error, got none")
		}
	})

	t.Run("IgnoreExtraParens", func(t *testing.T) {
		p, err := NewPrattParser(MaxRecursionDepth(1))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("((((1))))"))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error: %s", errs.ToDisplayString())
		}
	})

	t.Run("DeeplyNestedParens1000", func(t *testing.T) {
		p, err := NewPrattParser(MaxRecursionDepth(1))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		expr1 := strings.Repeat("(", 1000) + "42" + strings.Repeat(")", 1000)
		_, errs := p.Parse(common.NewTextSource(expr1))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error on 1000 parens literal: %s", errs.ToDisplayString())
		}

		expr2 := strings.Repeat("(", 1000) + "1 + 2" + strings.Repeat(")", 1000)
		_, errs = p.Parse(common.NewTextSource(expr2))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error on 1000 parens binary: %s", errs.ToDisplayString())
		}
	})

	t.Run("SequentialScopesDoNotAccumulateDepth", func(t *testing.T) {
		p, err := NewPrattParser(MaxRecursionDepth(2))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("[1] + [2] + [3]"))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error on sequential scopes: %s", errs.ToDisplayString())
		}
	})
}

func TestPrattParserMacroCalls(t *testing.T) {
	t.Run("DisabledByDefault", func(t *testing.T) {
		p, err := NewPrattParser(Macros(AllMacros...), PopulateMacroCalls(false))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		parsed, errs := p.Parse(common.NewTextSource("has(a.b)"))
		if len(errs.GetErrors()) > 0 {
			t.Fatalf("unexpected error: %s", errs.ToDisplayString())
		}
		if len(parsed.SourceInfo().MacroCalls()) != 0 {
			t.Errorf("expected 0 macro calls, got %d", len(parsed.SourceInfo().MacroCalls()))
		}
	})

	t.Run("GlobalMacroCallRecorded", func(t *testing.T) {
		p, err := NewPrattParser(Macros(AllMacros...), PopulateMacroCalls(true))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		parsed, errs := p.Parse(common.NewTextSource("has(a.b)"))
		if len(errs.GetErrors()) > 0 {
			t.Fatalf("unexpected error: %s", errs.ToDisplayString())
		}
		macroCalls := parsed.SourceInfo().MacroCalls()
		if len(macroCalls) != 1 {
			t.Fatalf("expected 1 macro call, got %d", len(macroCalls))
		}
	})

	t.Run("ReceiverMacroCallRecorded", func(t *testing.T) {
		p, err := NewPrattParser(Macros(AllMacros...), PopulateMacroCalls(true))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		parsed, errs := p.Parse(common.NewTextSource("[1, 2].exists(x, x > 0)"))
		if len(errs.GetErrors()) > 0 {
			t.Fatalf("unexpected error: %s", errs.ToDisplayString())
		}
		macroCalls := parsed.SourceInfo().MacroCalls()
		if len(macroCalls) != 1 {
			t.Fatalf("expected 1 macro call, got %d", len(macroCalls))
		}
	})

	t.Run("NestedMacroCallsRecorded", func(t *testing.T) {
		p, err := NewPrattParser(Macros(AllMacros...), PopulateMacroCalls(true))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		parsed, errs := p.Parse(common.NewTextSource("[1, 2].all(x, has(x.b))"))
		if len(errs.GetErrors()) > 0 {
			t.Fatalf("unexpected error: %s", errs.ToDisplayString())
		}
		macroCalls := parsed.SourceInfo().MacroCalls()
		if len(macroCalls) != 2 {
			t.Fatalf("expected 2 macro calls, got %d", len(macroCalls))
		}
	})
}

func TestPrattParserErrorRecoveryLimits(t *testing.T) {
	t.Run("LimitZero", func(t *testing.T) {
		p, err := NewPrattParser(ErrorRecoveryLimit(0))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("......"))
		if len(errs.GetErrors()) == 0 {
			t.Errorf("expected error recovery limit error, got none")
		}
	})

	t.Run("LimitOne", func(t *testing.T) {
		p, err := NewPrattParser(ErrorRecoveryLimit(1))
		if err != nil {
			t.Fatalf("NewPrattParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("......"))
		if len(errs.GetErrors()) == 0 {
			t.Errorf("expected error recovery limit error, got none")
		}
	})
}

func TestPrattParserExpressionSizeCodePointLimit(t *testing.T) {
	p, err := NewPrattParser(Macros(AllMacros...), ExpressionSizeCodePointLimit(2))
	if err != nil {
		t.Fatal(err)
	}
	src := common.NewTextSource("foo")
	_, errs := p.Parse(src)
	if got, want := len(errs.GetErrors()), 1; got != want {
		t.Fatalf("got %d errors, want %d errors: %s", got, want, errs.ToDisplayString())
	}
	if got, want := errs.GetErrors()[0].Message, "expression code point size exceeds limit: size: 3, limit 2"; got != want {
		t.Fatalf("got %q, want %q: %s", got, want, errs.GetErrors()[0].ToDisplayString(src))
	}
}

func BenchmarkParsers(b *testing.B) {
	exprs := []string{
		`42`,
		`a > 5 && b < 10 || c == "xyz"`,
		`[1, 2, 3].all(x, x > 0) && [4, 5, 6].exists(y, y == 5)`,
		`pkg.Msg{field1: "value", field2: 123, list_field: [1, 2, 3], map_field: {"a": true, "b": false}}`,
		`a.b.c.d.e.f(1, 2, [3, ?4], {?5: 6}) ? (x + y * z - w / v) : (!p && !q || r.s)`,
	}

	antlrParser, _ := NewParser(Macros(AllMacros...), PopulateMacroCalls(true), EnableOptionalSyntax(true))
	prattParser, _ := NewPrattParser(Macros(AllMacros...), PopulateMacroCalls(true), EnableOptionalSyntax(true))

	for _, expr := range exprs {
		src := common.NewTextSource(expr)

		b.Run("ANTLR/"+expr[:min(len(expr), 20)], func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = antlrParser.Parse(src)
			}
		})

		b.Run("Pratt/"+expr[:min(len(expr), 20)], func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = prattParser.Parse(src)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
