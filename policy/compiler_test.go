// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/google/cel-go/interpreter"
	"github.com/google/go-cmp/cmp"
)

func TestCompile(t *testing.T) {
	for _, tst := range policyTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			r := newRunner(tc.name, tc.expr, tc.parseOpts)
			env, ast, iss := r.compile(t, tc.envOpts, []CompilerOption{})
			if iss.Err() != nil {
				t.Fatalf("Compile(%s) failed: %v", r.name, iss.Err())
			}
			r.setup(t, env, ast)
			r.run(t)
		})
	}
}

func TestCompileError(t *testing.T) {
	for _, tst := range policyErrorTests {
		policy := parsePolicy(t, tst.name, []ParserOption{})
		_, _, iss := compile(t, tst.name, policy, []cel.EnvOption{}, tst.compilerOpts)
		if iss.Err() == nil {
			t.Fatalf("compile(%s) did not error, wanted %s", tst.name, tst.err)
		}
		if iss.Err().Error() != tst.err {
			t.Errorf("compile(%s) got error %s, wanted %s", tst.name, iss.Err().Error(), tst.err)
		}
	}
}

func TestCompiledRuleHasOptionalOutput(t *testing.T) {
	env, err := cel.NewEnv()
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		rule     *CompiledRule
		optional bool
	}{
		{rule: &CompiledRule{}, optional: false},
		{
			rule: &CompiledRule{
				matches: []*CompiledMatch{{}},
			},
			optional: true,
		},
		{
			rule: &CompiledRule{
				matches: []*CompiledMatch{{}},
			},
			optional: true,
		},
		{
			rule: &CompiledRule{
				matches: []*CompiledMatch{{cond: mustCompileExpr(t, env, "true")}},
			},
			optional: false,
		},
		{
			rule: &CompiledRule{
				matches: []*CompiledMatch{{cond: mustCompileExpr(t, env, "1 < 0")}},
			},
			optional: true,
		},
		{
			rule: &CompiledRule{
				matches: []*CompiledMatch{
					{
						cond: mustCompileExpr(t, env, "true"),
						nestedRule: &CompiledRule{
							matches: []*CompiledMatch{{cond: mustCompileExpr(t, env, "1 > 0")}},
						},
					},
					{cond: mustCompileExpr(t, env, "true")},
				},
			},
			optional: false,
		},
	}
	for _, tst := range tests {
		got := tst.rule.HasOptionalOutput()
		if got != tst.optional {
			t.Errorf("rule.HasOptionalOutput() got %v, wanted, %v", got, tst.optional)
		}
	}
}

func TestMaxNestedExpressions_Error(t *testing.T) {
	policyName := "limits"
	wantError := `ERROR: testdata/limits/policy.yaml:15:8: error configuring compiler option: nested expression limit must be non-negative, non-zero value: -1
 | name: "limits"
 | .......^`
	policy := parsePolicy(t, policyName, []ParserOption{})
	_, _, iss := compile(t, policyName, policy, []cel.EnvOption{}, []CompilerOption{MaxNestedExpressions(-1)})
	if iss.Err() == nil {
		t.Fatalf("compile(%s) did not error, wanted %s", policyName, wantError)
	}
	if iss.Err().Error() != wantError {
		t.Errorf("compile(%s) got error %s, wanted %s", policyName, iss.Err().Error(), wantError)
	}
}

func TestWhitespaceHanlding(t *testing.T) {

	testCases := []struct {
		matchID string
		want    string
	}{
		{
			matchID: "folded_unambiguous",
			want:    "a string expression that is folded",
		},
		{
			matchID: "folded_line_break",
			// 8 spaces (4 indents)
			want: "a string expression that\n        is folded",
		},
		{
			matchID: "folded_line_break_indent",
			// 10 spaces (5 indents)
			want: "a string expression that\n          is folded",
		},
		{
			matchID: "literal_unambiguous",
			want:    "a string expression that is a literal block",
		},
		{
			matchID: "literal_line_break",
			// 8 spaces (4 indents)
			want: "a string expression that\n        is a literal block",
		},
		{
			matchID: "literal_line_break_indent",
			// 10 spaces (5 indents)
			want: "a string expression that\n          is a literal block",
		},
	}

	policy := parsePolicy(t, "yaml_parsing", []ParserOption{})
	env, ast, iss := compile(t, "yaml_parsing", policy, []cel.EnvOption{}, []CompilerOption{})
	if iss.Err() != nil {
		t.Fatalf("compile('yaml_parsing') failed, %s", iss.Err().Error())
	}

	p, err := env.PlanProgram(ast.NativeRep())

	if err != nil {
		t.Fatalf("env.PlanProgram() failed, %s", err.Error())
	}

	for _, tst := range testCases {
		t.Run(tst.matchID, func(t *testing.T) {

			in := map[string]any{"match_id": types.String(tst.matchID)}

			result, _, err := p.Eval(in)
			if err != nil {
				t.Fatalf("p.Eval(match_id: '%s') failed, %s", tst.matchID, err.Error())
			}
			s, ok := result.(types.String)
			if !ok {
				t.Fatalf("p.Eval(match_id: '%s') got %v, wanted string", tst.matchID, result)
			}
			if dx := cmp.Diff(tst.want, s.Value()); dx != "" {
				t.Errorf("p.Eval(match_id: '%s') has diffs. (-want, +got): %s", tst.matchID, dx)
			}
		})
	}
}

func TestWhitespaceHandlingErrorPresentation(t *testing.T) {
	policy := parsePolicy(t, "yaml_parsing_cel_error", []ParserOption{})
	_, _, iss := compile(t, "yaml_parsing_cel_error", policy, []cel.EnvOption{}, []CompilerOption{})
	if iss.Err() == nil {
		t.Fatalf("compile('yaml_parsing_cel_error') did not error, wanted error")
	}

	wantErrors := []string{
		`ERROR: testdata/yaml_parsing_cel_error/policy.yaml:11:16: found no matching overload for '_+_' applied to '(string, int)'
 |         ("bar" + 1)
 | ...............^`,
		`ERROR: testdata/yaml_parsing_cel_error/policy.yaml:15:18: found no matching overload for '_+_' applied to '(string, int)'
 |           ("bar" + 1)
 | .................^`,
		`ERROR: testdata/yaml_parsing_cel_error/policy.yaml:19:16: found no matching overload for '_+_' applied to '(string, int)'
 |         ("bar" + 1)
 | ...............^`,
		`ERROR: testdata/yaml_parsing_cel_error/policy.yaml:23:18: found no matching overload for '_+_' applied to '(string, int)'
 |           ("bar" + 1)
 | .................^`,
	}
	found := false
	for _, wantError := range wantErrors {
		if !strings.Contains(iss.Err().Error(), wantError) {
			found = true
			t.Errorf("compile('yaml_parsing_cel_error') does not contain error %s", wantError)
		}
	}
	if found {
		t.Errorf("compile('yaml_parsing_cel_error') missing errors, got: %s", iss.Err().Error())
	}
}

func BenchmarkCompile(b *testing.B) {
	for _, tst := range policyTests {
		r := newRunner(tst.name, tst.expr, tst.parseOpts)
		env, ast, iss := r.compile(b, tst.envOpts, []CompilerOption{})
		if iss.Err() != nil {
			b.Fatalf("Compile() failed: %v", iss.Err())
		}
		r.setup(b, env, ast)
		r.bench(b)
	}
}

func parsePolicySource(t testing.TB, name string, policySource string, parseOpts ...ParserOption) *Policy {
	t.Helper()
	p := StringSource(policySource, name)
	parser, err := NewParser(parseOpts...)
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(p)
	if iss.Err() != nil {
		t.Fatalf("parser.Parse() failed: %v", iss.Err())
	}
	return policy
}

func parseAndCompilePolicy(t testing.TB, name string, policySource string, envOpts []cel.EnvOption, compilerOpts []CompilerOption) (*cel.Env, *cel.Ast, *cel.Issues) {
	t.Helper()
	policy := parsePolicySource(t, name, policySource)
	envOpts = append([]cel.EnvOption{
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		ext.Bindings(),
	}, envOpts...)
	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := Compile(env, policy, compilerOpts...)
	return env, ast, iss
}

func newRunner(name, expr string, parseOpts []ParserOption, envOpts ...cel.EnvOption) *runner {
	return &runner{
		name:      name,
		parseOpts: parseOpts,
		envOpts:   envOpts,
		expr:      expr}
}

type runner struct {
	name      string
	parseOpts []ParserOption
	envOpts   []cel.EnvOption
	env       *cel.Env
	expr      string
	prg       cel.Program
}

func (r *runner) compile(t testing.TB, envOpts []cel.EnvOption, compilerOpts []CompilerOption) (*cel.Env, *cel.Ast, *cel.Issues) {
	policy := parsePolicy(t, r.name, r.parseOpts)
	return compile(t, r.name, policy, envOpts, compilerOpts)
}

func (r *runner) compileRule(t testing.TB) (*cel.Env, *CompiledRule, *cel.Issues) {
	t.Helper()
	config := readPolicyConfig(t, fmt.Sprintf("testdata/%s/config.yaml", r.name))
	policy := parsePolicy(t, r.name, r.parseOpts)
	env, err := cel.NewCustomEnv(
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		cel.ExtendedValidations(),
		ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	if len(r.envOpts) > 0 {
		env, err = env.Extend(r.envOpts...)
		if err != nil {
			t.Fatalf("env.Extend() with env options failed: %v", err)
		}
	}
	// Configure declarations
	env, err = env.Extend(FromConfig(config))
	if err != nil {
		t.Fatalf("env.Extend() with config options %v, failed: %v", config, err)
	}
	rule, iss := CompileRule(env, policy)
	return env, rule, iss
}

func (r *runner) setup(t testing.TB, env *cel.Env, ast *cel.Ast) {
	t.Helper()
	pExpr, err := cel.AstToString(ast)
	if err != nil {
		t.Fatalf("cel.AstToString() failed: %v", err)
	}
	_, err = cel.AstToCheckedExpr(ast)
	if err != nil {
		t.Fatalf("cel.AstToCheckedExpr() failed: %v", err)
	}
	if r.expr != "" && normalize(pExpr) != normalize(r.expr) {
		t.Errorf("cel.AstToString() got %s, wanted %s", pExpr, r.expr)
	}
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	r.env = env
	r.prg = prg
}

func (r *runner) run(t *testing.T) {
	tests := readTestSuite(t, fmt.Sprintf("testdata/%s/tests.yaml", r.name))
	for _, s := range tests.Sections {
		section := s.Name
		for _, tst := range s.Tests {
			tc := tst
			t.Run(fmt.Sprintf("%s/%s/%s", r.name, section, tc.Name), func(t *testing.T) {
				input := map[string]any{}
				var err error
				var activation interpreter.Activation
				if tc.InputContext != nil && tc.InputContext.ContextExpr != "" {
					ctxExpr := tc.InputContext.ContextExpr
					ctx, err := r.eval(t, ctxExpr).ConvertToNative(
						reflect.TypeOf(((*proto.Message)(nil))).Elem())
					if err != nil {
						t.Fatalf("context variable is not a valid proto: %v", err)
					}
					activation, err = cel.ContextProtoVars(ctx.(proto.Message))
					if err != nil {
						t.Fatalf("cel.ContextProtoVars() failed: %v", err)
					}
				} else if len(tc.Input) != 0 {
					for k, v := range tc.Input {
						if v.Expr != "" {
							input[k] = r.eval(t, v.Expr)
							continue
						}
						input[k] = v.Value
					}
				}
				if activation == nil {
					activation, err = interpreter.NewActivation(input)
					if err != nil {
						t.Fatalf("interpreter.NewActivation(input) failed: %v", err)
					}
				}
				out, _, err := r.prg.Eval(activation)
				if err != nil {
					t.Fatalf("prg.Eval(input) failed: %v", err)
				}
				var testOut ref.Val
				if tc.Output.Expr != "" {
					testOut = r.eval(t, tc.Output.Expr)
				} else if tc.Output.Value != nil {
					testOut = r.env.CELTypeAdapter().NativeToValue(tc.Output.Value)
				}
				if testOut.Equal(out) == types.True {
					return
				}
				if optOut, ok := out.(*types.Optional); ok {
					if optOut.Equal(types.OptionalNone) == types.True {
						if testOut.Equal(types.OptionalNone) != types.True {
							t.Errorf("policy eval got %v, wanted %v", out, testOut)
						}
					} else if testOut.Equal(optOut.GetValue()) != types.True {
						t.Errorf("policy eval got %v, wanted %v", out, testOut)
					}
				} else {
					t.Errorf("policy eval got %v, wanted %v", out, testOut)
				}
			})
		}
	}
}

func (r *runner) bench(b *testing.B) {
	tests := readTestSuite(b, fmt.Sprintf("testdata/%s/tests.yaml", r.name))
	for _, s := range tests.Sections {
		section := s.Name
		for _, tst := range s.Tests {
			tc := tst
			b.Run(fmt.Sprintf("%s/%s/%s", r.name, section, tc.Name), func(b *testing.B) {
				input := map[string]any{}
				var err error
				var activation interpreter.Activation
				if tc.InputContext != nil && tc.InputContext.ContextExpr != "" {
					ctxExpr := tc.InputContext.ContextExpr
					ctx, err := r.eval(b, ctxExpr).ConvertToNative(
						reflect.TypeOf(((*proto.Message)(nil))).Elem())
					if err != nil {
						b.Fatalf("context variable is not a valid proto: %v", err)
					}
					activation, err = cel.ContextProtoVars(ctx.(proto.Message))
					if err != nil {
						b.Fatalf("cel.ContextProtoVars() failed: %v", err)
					}
				} else if tc.Input != nil {
					for k, v := range tc.Input {
						if v.Expr != "" {
							input[k] = r.eval(b, v.Expr)
							continue
						}
						input[k] = v.Value
					}
				}
				if activation == nil {
					activation, err = interpreter.NewActivation(input)
					if err != nil {
						b.Fatalf("interpreter.NewActivation(input) failed: %v", err)
					}
				}
				for i := 0; i < b.N; i++ {
					_, _, err := r.prg.Eval(activation)
					if err != nil {
						b.Fatalf("policy eval failed: %v", err)
					}
				}
			})
		}
	}
}

func (r *runner) eval(t testing.TB, expr string) ref.Val {
	wantExpr, iss := r.env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%q) failed :%v", expr, iss.Err())
	}
	prg, err := r.env.Program(wantExpr)
	if err != nil {
		t.Fatalf("env.Program(wantExpr) failed: %v", err)
	}
	out, _, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	return out
}

func mustCompileExpr(t testing.TB, env *cel.Env, expr string) *cel.Ast {
	t.Helper()
	out, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %v", expr, iss.Err())
	}
	return out
}

func parsePolicy(t testing.TB, name string, parseOpts []ParserOption) *Policy {
	t.Helper()
	srcFile := readPolicy(t, fmt.Sprintf("testdata/%s/policy.yaml", name))
	parser, err := NewParser(parseOpts...)
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(srcFile)
	if iss.Err() != nil {
		t.Fatalf("Parse() failed: %v", iss.Err())
	}
	if policy.name.Value != name {
		t.Errorf("policy name is %v, wanted %q", policy.name, name)
	}
	return policy
}

func compile(t testing.TB, name string, policy *Policy, envOpts []cel.EnvOption, compilerOpts []CompilerOption) (*cel.Env, *cel.Ast, *cel.Issues) {
	config := readPolicyConfig(t, fmt.Sprintf("testdata/%s/config.yaml", name))
	env, err := cel.NewCustomEnv(
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		cel.ExtendedValidations(),
		ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	// Configure any custom environment options.
	env, err = env.Extend(envOpts...)
	if err != nil {
		t.Fatalf("env.Extend() with env options %v, failed: %v", config, err)
	}
	// Configure declarations
	env, err = env.Extend(FromConfig(config))
	if err != nil {
		t.Fatalf("env.Extend() with config options %v, failed: %v", config, err)
	}
	ast, iss := Compile(env, policy, compilerOpts...)
	if iss.Err() == nil {
		verifySourceInfoCoverage(t, policy, ast)
	}
	return env, ast, iss
}

func normalize(s string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(s, " ", ""), "\n", ""),
		"\t", "")
}

func verifySourceInfoCoverage(t testing.TB, policy *Policy, ast *cel.Ast) {
	t.Helper()
	info := ast.SourceInfo()

	exprLines := exprLinesFromPolicy(policy)
	coveredLines := make(map[int]bool)
	ids := ast.NativeRep().IDs()
	for id, offset := range info.GetPositions() {
		if offset <= 0 {
			t.Errorf("id %d has invalid offset %v", id, offset)
		}
		// Check that each position in the SourceInfo corresponds to a valid AST node.
		if !ids[id] {
			t.Errorf("id %d not found in AST", id)
		}
		loc, found := ast.Source().OffsetLocation(offset)
		if found {
			coveredLines[loc.Line()] = true
		} else {
			t.Errorf("invalid source location for offset %d", offset)
		}
	}
	// Verify that each source line inside an expression is covered by the at least one node in the
	// AST.
	for line := range exprLines {
		if !coveredLines[line] {
			t.Errorf("Line %d expected to be covered by SourceInfo, but was not", line)
		}
	}

	if t.Failed() {
		checked, err := cel.AstToCheckedExpr(ast)
		if err != nil {
			t.Logf("cel.AstToCheckedExpr() failed: %v", err)
		} else {
			t.Logf("AST:\n%s", prototext.Format(checked.GetExpr()))
		}
	}
}

// exprLinesFromPolicy returns a set of line numbers within a policy where expressions (variables,
// conditions, etc.) are defined.
func exprLinesFromPolicy(policy *Policy) map[int]bool {
	lines := make(map[int]bool)
	addExpectedLines := func(vs ValueString) {
		if offset, found := policy.SourceInfo().GetOffsetRange(vs.ID); found {
			startLoc, foundStart := policy.Source().OffsetLocation(offset.Start)
			// Multiline strings can span multiple lines, but the SourceInfo will only contain the start
			// position of the expression. So just skip the check if the expression contains a multiline
			// string literal.
			hasMultiline := strings.Contains(vs.Value, "'''") || strings.Contains(vs.Value, "\"\"\"")
			if foundStart && !hasMultiline {
				numLines := strings.Count(vs.Value, "\n")
				for i := 0; i <= numLines; i++ {
					lines[startLoc.Line()+i] = true
				}
			}
		}
	}
	var traverseRule func(r *Rule)
	traverseRule = func(r *Rule) {
		for _, v := range r.Variables() {
			addExpectedLines(v.Expression())
		}
		for _, m := range r.Matches() {
			if !strings.HasPrefix(m.Condition().Value, "true") {
				addExpectedLines(m.Condition())
			}
			if m.HasOutput() {
				addExpectedLines(m.Output())
			}
			if m.HasRule() {
				traverseRule(m.Rule())
			}
		}
	}
	traverseRule(policy.Rule())
	return lines
}

func TestCompileYAMLPolicy_Aggregate(t *testing.T) {
	type testEval struct {
		input  map[string]any
		output ref.Val
	}
	tests := []struct {
		name             string
		policy           string
		envOpts          []cel.EnvOption
		expectedUnparsed string
		evals            []testEval
		wantErr          string
	}{
		{
			name: "eval_aggregate",
			policy: `name: "aggregate_policy"
rule:
  aggregate:
    - condition: 'true'
      output: '"PII"'
    - condition: 'true'
      output: '"CONFIDENTIAL"'`,
			expectedUnparsed: `["PII"] + ["CONFIDENTIAL"]`,
			evals: []testEval{
				{
					input:  map[string]any{},
					output: types.NewStringList(types.DefaultTypeAdapter, []string{"PII", "CONFIDENTIAL"}),
				},
			},
		},
		{
			name: "aggregate_with_block_variables",
			policy: `name: "block_policy"
rule:
  variables:
    - name: val1
      expression: '"PII"'
    - name: val2
      expression: '"CONFIDENTIAL"'
  aggregate:
    - condition: 'true'
      output: 'variables.val1'
    - condition: 'true'
      output: 'variables.val2'`,
			expectedUnparsed: `cel.@block(["PII", "CONFIDENTIAL"], [@index0] + [@index1])`,
			evals: []testEval{
				{
					input:  map[string]any{},
					output: types.NewStringList(types.DefaultTypeAdapter, []string{"PII", "CONFIDENTIAL"}),
				},
			},
		},
		{
			name: "aggregate_conditions_and_block_variables",
			policy: `name: "cse_policy"
rule:
  variables:
    - name: threshold
      expression: "5"
  aggregate:
    - condition: "size(resource.payload) > variables.threshold"
      output: '"CSE1"'
    - condition: "size(resource.payload) > variables.threshold"
      output: '"CSE2"'
    - condition: 'true'
      output: '"ALWAYS"'`,
			envOpts: []cel.EnvOption{
				cel.Variable("resource", cel.MapType(cel.StringType, cel.ListType(cel.IntType))),
			},
			expectedUnparsed: `cel.@block([5], ((size(resource.payload) > @index0) ? ["CSE1"] : []) + (((size(resource.payload) > @index0) ? ["CSE2"] : []) + ["ALWAYS"]))`,
			evals: []testEval{
				{
					input: map[string]any{
						"resource": map[string]any{
							"payload": []int64{1, 2, 3, 4, 5, 6},
						},
					},
					output: types.NewStringList(types.DefaultTypeAdapter, []string{"CSE1", "CSE2", "ALWAYS"}),
				},
				{
					input: map[string]any{
						"resource": map[string]any{
							"payload": []int64{1, 2, 3},
						},
					},
					output: types.NewStringList(types.DefaultTypeAdapter, []string{"ALWAYS"}),
				},
			},
		},
		{
			name: "aggregate_macros_preserved",
			policy: `name: aggregate_macros_preserved
rule:
  variables:
    - name: min_val
      expression: "10"
  aggregate:
    - condition: "cond"
      rule:
        match:
          - condition: "true"
            output: "payload.filter(x, x > variables.min_val).exists(y, y % 2 == 0)"
    - condition: "true"
      output: "payload.all(x, x > 0)"`,
			envOpts: []cel.EnvOption{
				cel.Variable("cond", cel.BoolType),
				cel.Variable("payload", cel.ListType(cel.IntType)),
			},
			expectedUnparsed: `cel.@block([10], (cond ? [payload.filter(x, x > @index0).exists(y, y % 2 == 0)] : []) + [payload.all(x, x > 0)])`,
		},
		{
			name: "nested_aggregate_throws",
			policy: `name: nested_aggregate
rule:
  aggregate:
    - condition: 'true'
      rule:
        aggregate:
          - condition: 'true'
            output: "'foo'"`,
			wantErr: "nested aggregate rules are not allowed",
		},
		{
			name: "nested_aggregate_with_match_throws",
			policy: `name: nested_aggregate_with_match
rule:
  aggregate:
    - condition: 'true'
      rule:
        match:
          - condition: 'true'
            rule:
              aggregate:
                - condition: 'true'
                  output: "'foo'"`,
			wantErr: "nested aggregate rules are not allowed",
		},
		{
			name: "aggregate_under_match_success",
			policy: `name: aggregate_under_match
rule:
  match:
    - condition: 'true'
      rule:
        aggregate:
          - condition: 'true'
            output: "'foo'"`,
			expectedUnparsed: `["foo"]`,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env, ast, iss := parseAndCompilePolicy(t, tc.name, tc.policy, tc.envOpts, nil)
			if tc.wantErr != "" {
				if iss.Err() == nil {
					t.Fatalf("Compile() succeeded, wanted error %q", tc.wantErr)
				}
				if !strings.Contains(iss.Err().Error(), tc.wantErr) {
					t.Errorf("Compile() got %v, wanted error containing %q", iss.Err(), tc.wantErr)
				}
				return
			}

			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}

			unparsed, err := cel.AstToString(ast)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if tc.expectedUnparsed != "" && normalize(unparsed) != normalize(tc.expectedUnparsed) {
				t.Errorf("cel.AstToString() got %s, wanted %s", unparsed, tc.expectedUnparsed)
			}

			_, err = cel.AstToCheckedExpr(ast)
			if err != nil {
				t.Fatalf("cel.AstToCheckedExpr() failed: %v", err)
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program(ast) failed: %v", err)
			}

			for _, ev := range tc.evals {
				out, _, err := prg.Eval(ev.input)
				if err != nil {
					t.Fatalf("prg.Eval(%v) failed: %v", ev.input, err)
				}
				if out.Equal(ev.output) != types.True {
					t.Errorf("prg.Eval(%v) got %v, wanted %v", ev.input, out, ev.output)
				}
			}
		})
	}
}

func TestCompiledRuleSemantic(t *testing.T) {
	policySource := `name: aggregate_semantic
rule:
  aggregate:
    - condition: 'true'
      output: "'foo'"`
	policy := parsePolicySource(t, "aggregate_semantic", policySource)
	env, err := cel.NewEnv()
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	compiledRule, iss := CompileRule(env, policy)
	if iss.Err() != nil {
		t.Fatalf("CompileRule() failed: %v", iss.Err())
	}
	if compiledRule.Semantic() != aggregate {
		t.Errorf("got %v, wanted aggregate", compiledRule.Semantic())
	}
}

func TestCompileYAMLPolicy_ConditionAlwaysFalse(t *testing.T) {
	policySource := `name: condition_always_false
rule:
  aggregate:
    - condition: 'false'
      output: "'foo'"`
	_, _, iss := parseAndCompilePolicy(t, "condition_always_false", policySource, nil, nil)
	if iss.Err() == nil {
		t.Fatalf("Compile() succeeded, wanted error")
	}
	if !strings.Contains(iss.Err().Error(), "Condition is always false") {
		t.Errorf("Compile() got %v, wanted 'Condition is always false'", iss.Err())
	}
}
