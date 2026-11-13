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

package examples

import (
	"fmt"
	"log"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/ext"
)

func explainDemoPod() map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app":  "web",
				"team": "payments",
			},
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
				map[string]any{
					"name":            "init",
					"securityContext": map[string]any{"privileged": true},
					"ports":           []any{map[string]any{"hostPort": 22}},
				},
			},
		},
	}
}

// Example_cel_Explain_violations showcases ExplainEval with OptTraceIterations, comparing
// short-circuiting evaluation with OptExhaustiveEval to report all offending elements.
func Example_cel_Explain_violations() {
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	if err != nil {
		log.Fatalf("cel.NewEnv() error: %v", err)
	}

	expr := `(object.spec.containers + object.spec.initContainers).all(c,
		!has(c.securityContext) || c.securityContext.privileged == false)`
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		log.Fatalf("Compile() error: %v", iss.Err())
	}

	// 1. Short-circuiting evaluation: stops at the first failing element.
	prgShort, err := env.Program(ast, cel.EvalOptions(cel.OptTraceIterations))
	if err != nil {
		log.Fatalf("Program() error: %v", err)
	}
	_, detShort, _ := prgShort.Eval(map[string]any{"object": explainDemoPod()})
	expShort, err := cel.ExplainEval(ast, detShort)
	if err != nil {
		log.Fatalf("ExplainEval() error: %v", err)
	}
	fmt.Println("Short-circuit:")
	fmt.Print(expShort.String())

	// 2. Exhaustive evaluation: reports every failing element across the joined list.
	prgFull, err := env.Program(ast, cel.EvalOptions(cel.OptTraceIterations, cel.OptExhaustiveEval))
	if err != nil {
		log.Fatalf("Program() error: %v", err)
	}
	_, detFull, _ := prgFull.Eval(map[string]any{"object": explainDemoPod()})
	expFull, err := cel.ExplainEval(ast, detFull)
	if err != nil {
		log.Fatalf("ExplainEval() error: %v", err)
	}
	fmt.Println("Exhaustive:")
	fmt.Printf("result: %v\n", expFull.Result())
	for _, v := range expFull.Violations() {
		fmt.Printf("  path=%s expr=%q value=%v\n", v.PathString(), v.Expr, v.Value)
	}

	// Output:
	// Short-circuit:
	// result: false
	//   (object.spec.containers + object.spec.initContainers)[1]: !has(c.securityContext) -> false
	//   (object.spec.containers + object.spec.initContainers)[1]: c.securityContext.privileged == false -> false
	// Exhaustive:
	// result: false
	//   path=(object.spec.containers + object.spec.initContainers)[1] expr="!has(c.securityContext)" value=false
	//   path=(object.spec.containers + object.spec.initContainers)[1] expr="c.securityContext.privileged == false" value=false
	//   path=(object.spec.containers + object.spec.initContainers)[2] expr="!has(c.securityContext)" value=false
	//   path=(object.spec.containers + object.spec.initContainers)[2] expr="c.securityContext.privileged == false" value=false
}

// Example_cel_Explain_nestedAndContext showcases nested comprehensions, map key paths,
// and Violation.Context for filter() expressions used inside comparisons.
func Example_cel_Explain_nestedAndContext() {
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType), ext.Bindings())
	if err != nil {
		log.Fatalf("cel.NewEnv() error: %v", err)
	}

	cases := []struct {
		name string
		expr string
	}{
		{
			name: "nested all()",
			expr: `object.spec.containers.all(c, c.ports.all(p, p.hostPort == 0))`,
		},
		{
			name: "map keys",
			expr: `object.metadata.labels.all(k, k.startsWith("app"))`,
		},
		{
			name: "filter() with Context",
			expr: `cel.bind(cs, object.spec.containers, cs.filter(c, c.securityContext.privileged).size() == 0)`,
		},
	}

	input := map[string]any{"object": explainDemoPod()}
	for _, tc := range cases {
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			log.Fatalf("Compile(%s) error: %v", tc.name, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptTraceIterations, cel.OptExhaustiveEval))
		if err != nil {
			log.Fatalf("Program(%s) error: %v", tc.name, err)
		}
		_, det, _ := prg.Eval(input)
		exp, err := cel.ExplainEval(ast, det)
		if err != nil {
			log.Fatalf("ExplainEval(%s) error: %v", tc.name, err)
		}
		fmt.Printf("%s:\n", tc.name)
		for _, v := range exp.Violations() {
			fmt.Printf("  %s\n", v.String())
			if v.Context != nil {
				fmt.Printf("    context: %s -> %v\n", v.Context.Expr, v.Context.Value)
			}
		}
	}

	// Output:
	// nested all():
	//   object.spec.containers[0].ports[1]: p.hostPort == 0 -> false
	// map keys:
	//   object.metadata.labels["team"]: k.startsWith("app") -> false
	// filter() with Context:
	//   cs[1]: c.securityContext.privileged -> true, in cs.filter(c, c.securityContext.privileged).size() == 0 -> false
	//     context: cs.filter(c, c.securityContext.privileged).size() == 0 -> false
}
