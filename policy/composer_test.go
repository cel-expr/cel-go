package policy

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/debug"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

func TestCompose(t *testing.T) {
	tests := []struct {
		name         string
		policy       string
		composerOpts []ComposerOption
		wantUnparsed string
		wantEval     string
		checkInfo    bool
	}{
		{
			name: "source_info",
			policy: `name: test_policy
rule:
  match:
    - condition: "2 == 1"
      output: "'hi'"
    - output: "'hello' + ' world'"
`,
			checkInfo: true,
		},
		{
			name: "unnest",
			policy: `name: unnest
rule:
  match:
    - condition: "2 == 1"
      output: "'hi'"
    - output: "'hello'"
`,
			composerOpts: []ComposerOption{ExpressionUnnestHeight(1)},
			checkInfo:    true,
		},
		{
			name: "empty_aggregate",
			policy: `name: empty_nested_match_under_aggregate
rule:
  aggregate:
    - condition: "true"
      rule:
        match: []
`,
			wantUnparsed: "[]",
			wantEval:     "[]",
		},
		{
			name: "conditional_optional_nested",
			policy: `name: conditional_optional_nested
rule:
  match:
    - condition: "2 == 2"
      rule:
        match:
          - condition: "1 == 1"
            output: "'foo'"
    - condition: "true"
      rule:
        match:
          - condition: "3 == 3"
            output: "'bar'"
`,
			wantUnparsed: `(2 == 2) ? ((1 == 1) ? optional.of("foo") : optional.none()) : ((3 == 3) ? optional.of("bar") : optional.none())`,
			wantEval:     `foo`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, compiledRule, compAST := parseAndComposeRule(t, tc.policy, tc.name+".yaml", tc.composerOpts...)
			if tc.checkInfo {
				si := compAST.SourceInfo()
				if si.Location != tc.name+".yaml" {
					t.Errorf("SourceInfo.Location got %q, wanted %s.yaml", si.Location, tc.name)
				}
				verifySourceInfoTransfer(t, compiledRule, compAST)
				if t.Failed() {
					t.Logf("composed AST: %s", debug.ToDebugStringWithIDs(compAST.NativeRep().Expr()))
					t.Logf("SourceInfo: %v", compAST.NativeRep().SourceInfo().OffsetRanges())
				}
			}
			if tc.wantUnparsed != "" {
				exprStr, err := cel.AstToString(compAST)
				if err != nil {
					t.Fatalf("cel.AstToString() failed: %v", err)
				}
				if normalize(exprStr) != normalize(tc.wantUnparsed) {
					t.Errorf("cel.AstToString() got %q, wanted %q", exprStr, tc.wantUnparsed)
				}
			}
			if tc.wantEval != "" {
				prg, err := env.Program(compAST)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				res, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if fmt.Sprintf("%v", res.Value()) != tc.wantEval {
					t.Errorf("eval result got %v, wanted %s", res.Value(), tc.wantEval)
				}
			}
		})
	}
}

type testUnconditionalComposer struct{}

func (t testUnconditionalComposer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	trueCond := ctx.NewLiteral(types.True)
	out1 := ctx.NewLiteral(types.String("first"))
	out2 := ctx.NewLiteral(types.String("second"))

	s := newNonOptionalCompositionStep(ctx, trueCond, out1)
	step := newNonOptionalCompositionStep(ctx, trueCond, out2)

	combined := s.combine(step)
	return ctx.NewAST(combined.expr())
}

// Note: This test case cannot be reached through the policy format (because the compiler
// statically rejects policies with unreachable outputs), but is expressed in code for defense in depth.
func TestNonOptionalCompositionStep_UnconditionalCombine(t *testing.T) {
	env, err := cel.NewEnv()
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	opt, err := cel.NewStaticOptimizer(testUnconditionalComposer{})
	if err != nil {
		t.Fatalf("cel.NewStaticOptimizer() failed: %v", err)
	}
	dummyAST, _ := env.Compile("true")
	resultAST, iss := opt.Optimize(env, dummyAST)
	if iss.Err() != nil {
		t.Fatalf("Optimize() failed: %v", iss.Err())
	}
	exprStr, err := cel.AstToString(resultAST)
	if err != nil {
		t.Fatalf("cel.AstToString() failed: %v", err)
	}
	if exprStr != `"first"` {
		t.Errorf("got %q, wanted \"first\"", exprStr)
	}
}

func TestRuleComposerError(t *testing.T) {
	env, err := cel.NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	_, err = NewRuleComposer(env, ExpressionUnnestHeight(-1))
	if err == nil || !strings.Contains(err.Error(), "invalid unnest") {
		t.Errorf("NewRuleComposer() got %v, wanted 'invalid unnest'", err)
	}
}

func TestRuleComposerUnnest(t *testing.T) {
	for _, tst := range composerUnnestTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			r := newRunner(tc.name, tc.expr, []ParserOption{})
			env, rule, iss := r.compileRule(t)
			if iss.Err() != nil {
				t.Fatalf("CompileRule() failed: %v", iss.Err())
			}
			rc, err := NewRuleComposer(env, tc.composerOpts...)
			if err != nil {
				t.Fatalf("NewRuleComposer() failed: %v", err)
			}
			ast, iss := rc.Compose(rule)
			if iss.Err() != nil {
				t.Fatalf("Compose(rule) failed: %v", iss.Err())
			}
			policy := parsePolicy(t, tc.name, []ParserOption{})
			verifySourceInfoCoverage(t, policy, ast)
			unparsed, err := cel.AstToString(ast)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if normalize(unparsed) != normalize(tc.composed) {
				t.Errorf("cel.AstToString() got %s, wanted %s", unparsed, tc.composed)
			}
			if !ast.OutputType().IsEquivalentType(tc.outputType) {
				t.Errorf("ast.OutputType() got %v, wanted %v", ast.OutputType(), tc.outputType)
			}
			r.setup(t, env, ast)
			r.run(t)
		})
	}
}

// verifySourceInfoTransfer checks that each offset range in the compiledRule has a corresponding node in composed
func verifySourceInfoTransfer(t *testing.T, compiledRule *CompiledRule, composed *cel.Ast) {
	t.Helper()
	dstRanges := make(map[ast.OffsetRange]ast.Expr)
	check := func(a *cel.Ast) {
		ast.PostOrderVisit(a.NativeRep().Expr(), &transferChecker{
			t:       t,
			srcInfo: a.NativeRep().SourceInfo(),
			dstInfo: composed.NativeRep().SourceInfo(),
			ranges:  &dstRanges})
	}
	ast.PostOrderVisit(composed.NativeRep().Expr(), &collectRanges{sourceInfo: composed.NativeRep().SourceInfo(), ranges: &dstRanges})
	for _, match := range compiledRule.matches {
		check(match.cond)
		check(match.output.expr)
	}
}

type collectRanges struct {
	sourceInfo *ast.SourceInfo
	ranges     *map[ast.OffsetRange]ast.Expr
}

func (r *collectRanges) VisitExpr(e ast.Expr) {
	if or, found := r.sourceInfo.GetOffsetRange(e.ID()); found {
		(*r.ranges)[or] = e
	}
}

func (r *collectRanges) VisitEntryExpr(ast.EntryExpr) {
}

type transferChecker struct {
	t       *testing.T
	srcInfo *ast.SourceInfo
	dstInfo *ast.SourceInfo
	ranges  *map[ast.OffsetRange]ast.Expr
}

func (c *transferChecker) VisitExpr(srcExpr ast.Expr) {
	if srcRange, haveSrc := c.srcInfo.GetOffsetRange(srcExpr.ID()); haveSrc {
		if srcRange.Start == 0 {
			// Ignore synthetic "true" default condition which has an incorrect source location
			return
		}
		dstExpr, haveDst := (*c.ranges)[srcRange]
		if !haveDst {
			c.t.Errorf("composed node not found for rule node: %s", debug.ToDebugString(srcExpr))
			return
		}

		// Check that the two nodes have the same textual representation
		if dstExpr.Kind() == ast.IdentKind && strings.HasPrefix(dstExpr.AsIdent(), "@index") {
			// Skip the check for unnested vars
			return
		}
		dstStr, err := cel.ExprToString(dstExpr, c.dstInfo)
		if err != nil {
			c.t.Errorf("failed to convert dstExpr")
			return
		}
		srcStr, err := cel.ExprToString(srcExpr, c.srcInfo)
		if err != nil {
			c.t.Errorf("failed to convert srcExpr")
			return
		}
		if srcStr != dstStr {
			c.t.Errorf("mismatched nodes, rule: %s composed: %s", srcStr, dstStr)
		}
	}
}

func (c *transferChecker) VisitEntryExpr(ast.EntryExpr) {
}

func parseAndComposeRule(t testing.TB, policyYAML, filename string, composerOpts ...ComposerOption) (*cel.Env, *CompiledRule, *cel.Ast) {
	t.Helper()
	src := StringSource(policyYAML, filename)
	parser, err := NewParser()
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(src)
	if iss.Err() != nil {
		t.Fatalf("parser.Parse() failed: %v", iss.Err())
	}
	env, err := cel.NewEnv(cel.OptionalTypes(), ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	compiledRule, iss := CompileRule(env, policy)
	if iss.Err() != nil {
		t.Fatalf("CompileRule() failed: %v", iss.Err())
	}
	composer, err := NewRuleComposer(env, composerOpts...)
	if err != nil {
		t.Fatalf("NewRuleComposer() failed: %v", err)
	}
	compAST, iss := composer.Compose(compiledRule)
	if iss.Err() != nil {
		t.Fatalf("composer.Compose() failed: %v", iss.Err())
	}
	return env, compiledRule, compAST
}
