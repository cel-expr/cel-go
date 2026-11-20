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
	"fmt"
	"regexp"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	celast "cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/ext"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// explainPod is a Pod-like object in which containers[1] and initContainers[1] are privileged,
// containers[0] exposes host port 8080, and initContainers[1] exposes host port 22.
func explainPod() map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{"app.kubernetes.io/name": "web", "team": "payments"},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":            "app",
					"securityContext": map[string]any{"privileged": false},
					"ports":           []any{map[string]any{"hostPort": 0}, map[string]any{"hostPort": 8080}},
				},
				map[string]any{
					"name":            "sidecar",
					"securityContext": map[string]any{"privileged": true},
					"ports":           []any{map[string]any{"hostPort": 0}},
				},
			},
			"initContainers": []any{
				map[string]any{"name": "init", "ports": []any{}},
				map[string]any{
					"name":            "setup",
					"securityContext": map[string]any{"privileged": true},
					"ports":           []any{map[string]any{"hostPort": 22}},
				},
			},
		},
	}
}

func explainEval(t *testing.T, expr string, opts ...cel.EvalOption) *cel.Explanation {
	t.Helper()
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType), ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast, cel.EvalOptions(opts...))
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	// Evaluation errors are explained as well, so only a missing EvalDetails is fatal.
	_, det, err := prg.Eval(map[string]any{"object": explainPod()})
	if det == nil {
		t.Fatalf("Eval(%q) failed: %v", expr, err)
	}
	exp, err := cel.ExplainEval(ast, det)
	if err != nil {
		t.Fatalf("cel.ExplainEval() failed: %v", err)
	}
	return exp
}

func violationStrings(vs []cel.Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.String()
	}
	return out
}

func TestExplainViolations(t *testing.T) {
	tests := []struct {
		name string
		expr string
		opts []cel.EvalOption
		want []string
	}{
		{
			// The KubeScape use case: identify every offending element of a joined list by its
			// stable flat index.
			name: "all_over_joined_lists",
			expr: `(object.spec.containers + object.spec.initContainers).all(c,
				!has(c.securityContext) || c.securityContext.privileged == false)`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`(object.spec.containers + object.spec.initContainers)[1]: !has(c.securityContext) -> false`,
				`(object.spec.containers + object.spec.initContainers)[1]: c.securityContext.privileged == false -> false`,
				`(object.spec.containers + object.spec.initContainers)[3]: !has(c.securityContext) -> false`,
				`(object.spec.containers + object.spec.initContainers)[3]: c.securityContext.privileged == false -> false`,
			},
		},
		{
			// Without exhaustive evaluation, all() stops at the first failing element.
			name: "all_short_circuit",
			expr: `(object.spec.containers + object.spec.initContainers).all(c,
				!has(c.securityContext) || c.securityContext.privileged == false)`,
			opts: []cel.EvalOption{cel.OptTraceIterations},
			want: []string{
				`(object.spec.containers + object.spec.initContainers)[1]: !has(c.securityContext) -> false`,
				`(object.spec.containers + object.spec.initContainers)[1]: c.securityContext.privileged == false -> false`,
			},
		},
		{
			name: "nested_all",
			expr: `(object.spec.containers + object.spec.initContainers).all(c,
				c.ports.all(p, p.hostPort == 0))`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`(object.spec.containers + object.spec.initContainers)[0].ports[1]: p.hostPort == 0 -> false`,
				`(object.spec.containers + object.spec.initContainers)[3].ports[0]: p.hostPort == 0 -> false`,
			},
		},
		{
			// A false exists() is explained by every iteration.
			name: "exists_false",
			expr: `object.spec.containers.exists(c, c.name == 'db')`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`object.spec.containers[0]: c.name == "db" -> false`,
				`object.spec.containers[1]: c.name == "db" -> false`,
			},
		},
		{
			// Map iterations are identified by key rather than by ordinal.
			name: "map_keys",
			expr: `object.metadata.labels.all(k, k.startsWith('app.'))`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`object.metadata.labels["team"]: k.startsWith("app.") -> false`,
			},
		},
		{
			name: "logical_and_outside_comprehension",
			expr: `object.spec.containers.size() == 2 && object.spec.initContainers.size() == 1`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`object.spec.initContainers.size() == 1 -> false`,
			},
		},
		{
			name: "conditional_and_bind",
			expr: `cel.bind(cs, object.spec.containers,
				cs.size() > 0 ? cs.all(c, c.securityContext.privileged == false) : true)`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`cs[1]: c.securityContext.privileged == false -> false`,
			},
		},
		{
			// The false branch of a conditional is blamed when the condition is false.
			name: "conditional_false_branch",
			expr: `object.spec.containers.size() > 5 ? true :
				object.spec.containers.all(c, c.securityContext.privileged == false)`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`object.spec.containers[1]: c.securityContext.privileged == false -> false`,
			},
		},
		{
			// A quantifier compared within a leaf is explained by the iterations which produced
			// its value, here the container which matched.
			name: "quantifier_within_leaf",
			expr: `object.spec.containers.exists(c, c.name == 'app') == false`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`object.spec.containers[0]: c.name == "app" -> true, in object.spec.containers.exists(c, c.name == "app") == false -> false`,
			},
		},
		{
			name: "bind_within_leaf",
			expr: `cel.bind(cs, object.spec.containers, cs.filter(c, c.securityContext.privileged)).size() == 0`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`cs[1]: c.securityContext.privileged -> true, in cel.bind(cs, object.spec.containers, cs.filter(c, c.securityContext.privileged)).size() == 0 -> false`,
			},
		},
		{
			// A comprehension in the unevaluated branch of a conditional contributes nothing.
			name: "unevaluated_comprehension_within_leaf",
			expr: `(false ? object.spec.containers.map(c, c.name) : ['x']) == ['y']`,
			opts: []cel.EvalOption{cel.OptTraceIterations},
			want: []string{
				`(false ? object.spec.containers.map(c, c.name) : ["x"]) == ["y"] -> false`,
			},
		},
		{
			// A non-boolean result is explained by the iterations which produced it.
			name: "non_boolean_result",
			expr: `object.spec.containers.map(c, c.name)`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{
				`object.spec.containers[0]: c.name -> "app", in object.spec.containers.map(c, c.name) -> [app, sidecar]`,
				`object.spec.containers[1]: c.name -> "sidecar", in object.spec.containers.map(c, c.name) -> [app, sidecar]`,
			},
		},
		{
			name: "satisfied",
			expr: `object.spec.containers.all(c, has(c.name))`,
			opts: []cel.EvalOption{cel.OptExhaustiveEval, cel.OptTraceIterations},
			want: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exp := explainEval(t, tc.expr, tc.opts...)
			got := violationStrings(exp.Violations())
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("Violations() got:\n%s\nwanted:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestExplainViolationPaths(t *testing.T) {
	exp := explainEval(t,
		`(object.spec.containers + object.spec.initContainers).all(c, c.securityContext.privileged == false)`,
		cel.OptExhaustiveEval, cel.OptTraceIterations)
	if exp.Result() != types.False {
		t.Fatalf("Result() got %v, wanted false", exp.Result())
	}
	var indices []int64
	for _, v := range exp.Violations() {
		if len(v.Path) != 1 {
			t.Fatalf("violation %v got path length %d, wanted 1", v, len(v.Path))
		}
		ref := v.Path[0]
		if ref.Range != "object.spec.containers + object.spec.initContainers" {
			t.Errorf("Range got %q", ref.Range)
		}
		if ref.Value == nil {
			t.Errorf("Value got nil, wanted the element")
		}
		indices = append(indices, int64(ref.Key.(types.Int)))
	}
	// initContainers[0] has no securityContext, so the predicate errors for index 2 as well.
	want := []int64{1, 2, 3}
	if len(indices) != len(want) {
		t.Fatalf("violation indices got %v, wanted %v", indices, want)
	}
	for i := range want {
		if indices[i] != want[i] {
			t.Errorf("violation indices got %v, wanted %v", indices, want)
		}
	}
}

func TestExplainString(t *testing.T) {
	exp := explainEval(t, `object.spec.containers.all(c, c.securityContext.privileged == false)`,
		cel.OptExhaustiveEval, cel.OptTraceIterations)
	want := "result: false\n  object.spec.containers[1]: c.securityContext.privileged == false -> false\n"
	if got := exp.String(); got != want {
		t.Errorf("String() got:\n%s\nwanted:\n%s", got, want)
	}
}

func TestExplainComprehensionKinds(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want []string
		// trackedWant is the expectation with macro call tracking when it differs from want.
		trackedWant []string
	}{
		{
			// The privileged containers are the ones which made the filtered list non-empty.
			name: "filter",
			expr: `object.spec.containers.filter(c, c.securityContext.privileged).size() == 0`,
			want: []string{
				`object.spec.containers[1]: c.securityContext.privileged -> true, in object.spec.containers.filter(c, c.securityContext.privileged).size() == 0 -> false`,
			},
		},
		{
			name: "filter_within_all",
			expr: `(object.spec.containers + object.spec.initContainers).all(c,
				c.ports.filter(p, p.hostPort != 0).size() == 0)`,
			want: []string{
				`(object.spec.containers + object.spec.initContainers)[0].ports[1]: p.hostPort != 0 -> true, in c.ports.filter(p, p.hostPort != 0).size() == 0 -> false`,
				`(object.spec.containers + object.spec.initContainers)[3].ports[0]: p.hostPort != 0 -> true, in c.ports.filter(p, p.hostPort != 0).size() == 0 -> false`,
			},
		},
		{
			// Every element of a mapped list came from an iteration.
			name: "map",
			expr: `object.spec.containers.map(c, c.name) == ['app']`,
			want: []string{
				`object.spec.containers[0]: c.name -> "app", in object.spec.containers.map(c, c.name) == ["app"] -> false`,
				`object.spec.containers[1]: c.name -> "sidecar", in object.spec.containers.map(c, c.name) == ["app"] -> false`,
			},
		},
		{
			// Only the iterations admitted by the filter contributed elements.
			name: "map_filter",
			expr: `object.spec.containers.map(c, c.name != 'app', c.name) == []`,
			want: []string{
				`object.spec.containers[1]: c.name -> "sidecar", in object.spec.containers.map(c, c.name != "app", c.name) == [] -> false`,
			},
		},
		{
			// More than one match: the matching iterations are responsible.
			name: "exists_one_too_many",
			expr: `object.spec.containers.exists_one(c, has(c.ports))`,
			want: []string{
				`object.spec.containers[0]: has(c.ports) -> true`,
				`object.spec.containers[1]: has(c.ports) -> true`,
			},
		},
		{
			// No match: every iteration is responsible.
			name: "exists_one_none",
			expr: `object.spec.containers.exists_one(c, c.name == 'db')`,
			want: []string{
				`object.spec.containers[0]: c.name == "db" -> false`,
				`object.spec.containers[1]: c.name == "db" -> false`,
			},
		},
		{
			name: "two_var_all",
			expr: `object.spec.containers.all(i, c, i == 0 || !c.securityContext.privileged)`,
			want: []string{
				`object.spec.containers[1]: i == 0 -> false`,
				`object.spec.containers[1]: !c.securityContext.privileged -> false`,
			},
		},
		{
			name: "two_var_exists_one",
			expr: `object.spec.containers.existsOne(i, c, i >= 0)`,
			want: []string{
				`object.spec.containers[0]: i >= 0 -> true`,
				`object.spec.containers[1]: i >= 0 -> true`,
			},
		},
		{
			name: "transform_list",
			expr: `object.spec.containers.transformList(i, c, i > 0, c.name) == []`,
			want: []string{
				`object.spec.containers[1]: c.name -> "sidecar", in object.spec.containers.transformList(i, c, i > 0, c.name) == [] -> false`,
			},
		},
		{
			name: "transform_map",
			expr: `object.metadata.labels.transformMap(k, v, k != 'team', v) == {}`,
			want: []string{
				`object.metadata.labels["app.kubernetes.io/name"]: v -> "web", in object.metadata.labels.transformMap(k, v, k != "team", v) == {} -> false`,
			},
		},
		{
			name: "transform_map_entry",
			expr: `object.metadata.labels.transformMapEntry(k, v, k == 'team', {v: k}) == {}`,
			want: []string{
				`object.metadata.labels["team"]: k == "team" -> true, in object.metadata.labels.transformMapEntry(k, v, k == "team", {v: k}) == {} -> false`,
			},
		},
		{
			// A conditional step which does not fall back to the accumulator is not a filter, and
			// so the comprehension is unrecognized.
			name: "custom_conditional_comprehension",
			expr: `object.spec.containers.lastPorts(c, c.ports.size()) == 0`,
			want: []string{
				`object.spec.containers[0]: (c.ports.size() > 0) ? c.ports.size() : 0 -> 2, in <expr> -> false`,
				`object.spec.containers[1]: (c.ports.size() > 0) ? c.ports.size() : 0 -> 1, in <expr> -> false`,
			},
			trackedWant: []string{
				`object.spec.containers[0]: (c.ports.size() > 0) ? c.ports.size() : 0 -> 2, in object.spec.containers.lastPorts(c, c.ports.size()) == 0 -> false`,
				`object.spec.containers[1]: (c.ports.size() > 0) ? c.ports.size() : 0 -> 1, in object.spec.containers.lastPorts(c, c.ports.size()) == 0 -> false`,
			},
		},
		{
			// The semantics of a custom comprehension are unknown, so every iteration is reported
			// with the value of its loop step. Without macro call tracking the custom macro cannot
			// be unparsed, so the context is identified by expression id.
			name: "custom_comprehension",
			expr: `object.spec.containers.portCount(c, c.ports.size()) == 0`,
			want: []string{
				`object.spec.containers[0]: @count + c.ports.size() -> 2, in <expr> -> false`,
				`object.spec.containers[1]: @count + c.ports.size() -> 3, in <expr> -> false`,
			},
			trackedWant: []string{
				`object.spec.containers[0]: @count + c.ports.size() -> 2, in object.spec.containers.portCount(c, c.ports.size()) == 0 -> false`,
				`object.spec.containers[1]: @count + c.ports.size() -> 3, in object.spec.containers.portCount(c, c.ports.size()) == 0 -> false`,
			},
		},
	}
	portCount := cel.ReceiverMacro("portCount", 2,
		func(mef cel.MacroExprFactory, target celast.Expr, args []celast.Expr) (celast.Expr, *cel.Error) {
			return mef.NewComprehension(target, args[0].AsIdent(), "@count",
				mef.NewLiteral(types.IntZero),
				mef.NewLiteral(types.True),
				mef.NewCall(operators.Add, mef.NewIdent("@count"), args[1]),
				mef.NewIdent("@count")), nil
		})
	lastPorts := cel.ReceiverMacro("lastPorts", 2,
		func(mef cel.MacroExprFactory, target celast.Expr, args []celast.Expr) (celast.Expr, *cel.Error) {
			return mef.NewComprehension(target, args[0].AsIdent(), "@last",
				mef.NewLiteral(types.IntZero),
				mef.NewLiteral(types.True),
				mef.NewCall(operators.Conditional,
					mef.NewCall(operators.Greater, mef.Copy(args[1]), mef.NewLiteral(types.IntZero)),
					mef.Copy(args[1]), mef.NewLiteral(types.IntZero)),
				mef.NewIdent("@last")), nil
		})
	// Comprehensions are classified from their expansion, or with macro call tracking, from the
	// recorded macro call. Both must explain the standard macros identically.
	for _, tracked := range []bool{false, true} {
		opts := []cel.EnvOption{cel.Variable("object", cel.DynType), ext.Bindings(),
			ext.TwoVarComprehensions(), cel.Macros(portCount, lastPorts)}
		if tracked {
			opts = append(opts, cel.EnableMacroCallTracking())
		}
		env, err := cel.NewEnv(opts...)
		if err != nil {
			t.Fatalf("NewEnv() failed: %v", err)
		}
		for _, tc := range tests {
			t.Run(fmt.Sprintf("%s/tracked=%t", tc.name, tracked), func(t *testing.T) {
				want := tc.want
				if tracked && tc.trackedWant != nil {
					want = tc.trackedWant
				}
				exp := explainWith(t, env, tc.expr)
				got := violationStrings(exp.Violations())
				// Expression ids are not stable across parser changes, so elide them before comparing.
				for i, g := range got {
					got[i] = exprIDPattern.ReplaceAllString(g, "<expr>")
				}
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("Violations() got:\n%s\nwanted:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
				}
			})
		}
	}
}

// explainWith evaluates the expression exhaustively with tracing against explainPod.
func explainWith(t *testing.T, env *cel.Env, expr string) *cel.Explanation {
	t.Helper()
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval, cel.OptTraceIterations))
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	_, det, err := prg.Eval(map[string]any{"object": explainPod()})
	if err != nil {
		t.Fatalf("Eval() failed: %v", err)
	}
	exp, err := cel.ExplainEval(ast, det)
	if err != nil {
		t.Fatalf("ExplainEval() failed: %v", err)
	}
	return exp
}

func TestExplainRecordedMacroCall(t *testing.T) {
	// An all() whose expansion places the predicate before the accumulator, as other CEL
	// implementations may, is not recognized from its structure. The recorded macro call
	// identifies it and its predicate.
	predFirstAll := cel.ReceiverMacro("all", 2,
		func(mef cel.MacroExprFactory, target celast.Expr, args []celast.Expr) (celast.Expr, *cel.Error) {
			return mef.NewComprehension(target, args[0].AsIdent(), "@result",
				mef.NewLiteral(types.True),
				mef.NewCall(operators.NotStrictlyFalse, mef.NewAccuIdent()),
				mef.NewCall(operators.LogicalAnd, args[1], mef.NewAccuIdent()),
				mef.NewAccuIdent()), nil
		})
	expr := `object.spec.containers.all(c, c.securityContext.privileged == false)`
	tests := []struct {
		name string
		opts []cel.EnvOption
		want []string
	}{
		{
			name: "untracked",
			want: []string{
				`object.spec.containers[0]: c.securityContext.privileged == false && @result -> true, in <expr> -> false`,
				`object.spec.containers[1]: c.securityContext.privileged == false && @result -> false, in <expr> -> false`,
			},
		},
		{
			name: "tracked",
			opts: []cel.EnvOption{cel.EnableMacroCallTracking()},
			want: []string{
				`object.spec.containers[1]: c.securityContext.privileged == false -> false`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]cel.EnvOption{cel.ClearMacros(), cel.Macros(predFirstAll),
				cel.Variable("object", cel.DynType)}, tc.opts...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			got := violationStrings(explainWith(t, env, expr).Violations())
			for i, g := range got {
				got[i] = exprIDPattern.ReplaceAllString(g, "<expr>")
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("Violations() got:\n%s\nwanted:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestExplainMismatchedMacroCall(t *testing.T) {
	// A recorded macro call which does not describe the expansion, e.g. one left behind by
	// rewriting the AST after parsing, is ignored in favor of the expansion. Here the record
	// claims an exists() over a predicate which is not part of the loop step.
	// [1, 2].all(x, x < 2)
	fac := celast.NewExprFactory()
	expr := fac.NewComprehension(1, fac.NewList(2, []celast.Expr{
		fac.NewLiteral(3, types.Int(1)), fac.NewLiteral(4, types.Int(2))}, nil),
		"x", "@result",
		fac.NewLiteral(5, types.True),
		fac.NewCall(6, operators.NotStrictlyFalse, fac.NewAccuIdent(7)),
		fac.NewCall(8, operators.LogicalAnd, fac.NewAccuIdent(9),
			fac.NewCall(10, operators.Less, fac.NewIdent(11, "x"), fac.NewLiteral(12, types.Int(2)))),
		fac.NewAccuIdent(13))
	info := celast.NewSourceInfo(nil)
	info.SetMacroCall(1, fac.NewMemberCall(0, "exists", fac.NewUnspecifiedExpr(2),
		fac.NewIdent(20, "x"), fac.NewIdent(21, "y")))
	pe, err := celast.ToProto(celast.NewAST(expr, info))
	if err != nil {
		t.Fatalf("ToProto() failed: %v", err)
	}
	env, err := cel.NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast := cel.ParsedExprToAst(&exprpb.ParsedExpr{Expr: pe.GetExpr(), SourceInfo: pe.GetSourceInfo()})
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval, cel.OptTraceIterations))
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	_, det, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("Eval() failed: %v", err)
	}
	exp, err := cel.ExplainEval(ast, det)
	if err != nil {
		t.Fatalf("ExplainEval() failed: %v", err)
	}
	got := violationStrings(exp.Violations())
	want := []string{`([1, 2])[1]: x < 2 -> false`}
	for i, g := range got {
		got[i] = exprIDPattern.ReplaceAllString(g, "<expr>")
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Violations() got %v, wanted %v", got, want)
	}
}

func TestExplainViolationContext(t *testing.T) {
	exp := explainEval(t, `object.spec.containers.filter(c, c.securityContext.privileged).size() == 0`,
		cel.OptExhaustiveEval, cel.OptTraceIterations)
	vs := exp.Violations()
	if len(vs) != 1 {
		t.Fatalf("Violations() got %v, wanted one violation", vs)
	}
	v := vs[0]
	if v.Context == nil || v.Context.Value != types.False || len(v.Context.Path) != 0 {
		t.Errorf("Context got %+v, wanted the root comparison", v.Context)
	}
	if v.Path[0].Key != types.Int(1) || v.Value != types.True {
		t.Errorf("violation got %v, wanted containers[1] with a true predicate", v)
	}
}

var exprIDPattern = regexp.MustCompile(`<expr \d+>`)

func TestExplainFormatting(t *testing.T) {
	ref := cel.IterationRef{Range: "a + b", Key: types.Int(3)}
	if got, want := ref.String(), "(a + b)[3]"; got != want {
		t.Errorf("IterationRef.String() got %q, wanted %q", got, want)
	}
	ref = cel.IterationRef{Range: "m", Key: types.String("k")}
	if got, want := ref.String(), `m["k"]`; got != want {
		t.Errorf("IterationRef.String() got %q, wanted %q", got, want)
	}

	// String and error values are rendered distinctly.
	exp := explainEval(t, `object.spec.containers[0].name == 'app' && object.spec.missing == 1`,
		cel.OptExhaustiveEval, cel.OptTraceIterations)
	if got, want := violationStrings(exp.Violations()), []string{
		`object.spec.missing == 1 -> error: no such key: missing`,
	}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Violations() got %v, wanted %v", got, want)
	}
}

func TestExplainBlock(t *testing.T) {
	// cel.@block([object.spec.containers], @index0.all(c, c.securityContext.privileged == false))
	fac := celast.NewExprFactory()
	expr := fac.NewCall(1, "cel.@block",
		fac.NewList(2, []celast.Expr{
			fac.NewSelect(3, fac.NewSelect(4, fac.NewIdent(5, "object"), "spec"), "containers"),
		}, nil),
		fac.NewComprehension(6, fac.NewIdent(7, "@index0"), "c", "@result",
			fac.NewLiteral(8, types.True),
			fac.NewCall(9, operators.NotStrictlyFalse, fac.NewAccuIdent(10)),
			fac.NewCall(11, operators.LogicalAnd, fac.NewAccuIdent(12),
				fac.NewCall(13, operators.Equals,
					fac.NewSelect(14, fac.NewSelect(15, fac.NewIdent(16, "c"), "securityContext"), "privileged"),
					fac.NewLiteral(17, types.False))),
			fac.NewAccuIdent(18)))
	pe, err := celast.ToProto(celast.NewAST(expr, nil))
	if err != nil {
		t.Fatalf("ToProto() failed: %v", err)
	}
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType), ext.Bindings())
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	// The block is evaluated unchecked since the checker does not resolve @index variables.
	ast := cel.ParsedExprToAst(&exprpb.ParsedExpr{Expr: pe.GetExpr()})
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval, cel.OptTraceIterations))
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	_, det, err := prg.Eval(map[string]any{"object": explainPod()})
	if err != nil {
		t.Fatalf("Eval() failed: %v", err)
	}
	exp, err := cel.ExplainEval(ast, det)
	if err != nil {
		t.Fatalf("ExplainEval() failed: %v", err)
	}
	got := violationStrings(exp.Violations())
	want := []string{`@index0[1]: c.securityContext.privileged == false -> false`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Violations() got %v, wanted %v", got, want)
	}
}

func TestExplainMismatchedAst(t *testing.T) {
	// Explaining an evaluation with the AST of a different expression must not panic: records for
	// unknown expressions are reported by id, and their iterations are left without a range.
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	evaluated, iss := env.Compile(`object.spec.containers.all(c, has(c.name))`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	other, iss := env.Compile(`true`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(evaluated, cel.EvalOptions(cel.OptTraceIterations))
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	_, det, err := prg.Eval(map[string]any{"object": explainPod()})
	if err != nil {
		t.Fatalf("Eval() failed: %v", err)
	}
	exp, err := cel.ExplainEval(other, det)
	if err != nil {
		t.Fatalf("ExplainEval() failed: %v", err)
	}
	if got, want := exp.String(), "result: <unobserved>\n"; got != want {
		t.Errorf("String() got %q, wanted %q", got, want)
	}
}

func TestExplainEvalRequiresTracing(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType), ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile(`object.spec.containers.all(c, has(c.name))`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	for _, opts := range [][]cel.EvalOption{{}, {cel.OptTrackState}, {cel.OptExhaustiveEval}} {
		prg, err := env.Program(ast, cel.EvalOptions(opts...))
		if err != nil {
			t.Fatalf("Program() failed: %v", err)
		}
		_, det, err := prg.Eval(map[string]any{"object": explainPod()})
		if err != nil {
			t.Fatalf("Eval() failed: %v", err)
		}
		if _, err := cel.ExplainEval(ast, det); err == nil {
			t.Errorf("cel.ExplainEval() with options %v succeeded, wanted error", opts)
		}
	}
	if _, err := cel.ExplainEval(nil, nil); err == nil {
		t.Error("cel.ExplainEval(nil, nil) succeeded, wanted error")
	}
}
