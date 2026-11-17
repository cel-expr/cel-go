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
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/ext"
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


func TestExplainResult(t *testing.T) {
	exp := explainEval(t, `object.spec.containers.all(c, has(c.name))`, cel.OptTraceIterations)
	if exp.Result() != types.True {
		t.Errorf("Result() got %v, wanted true", exp.Result())
	}
	exp = explainEval(t, `object.spec.containers.all(c, c.securityContext.privileged == false)`, cel.OptTraceIterations)
	if exp.Result() != types.False {
		t.Errorf("Result() got %v, wanted false", exp.Result())
	}
}

func TestExplainIterationRefFormatting(t *testing.T) {
	ref := cel.IterationRef{Range: "a + b", Key: types.Int(3)}
	if got, want := ref.String(), "(a + b)[3]"; got != want {
		t.Errorf("IterationRef.String() got %q, wanted %q", got, want)
	}
	ref = cel.IterationRef{Range: "m", Key: types.String("k")}
	if got, want := ref.String(), `m["k"]`; got != want {
		t.Errorf("IterationRef.String() got %q, wanted %q", got, want)
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
	if exp.Result() != nil {
		t.Errorf("Result() got %v, wanted nil", exp.Result())
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
