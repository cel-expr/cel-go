// Copyright 2025 Google LLC
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
	"testing"

	"github.com/google/cel-go/cel"
)

func TestMaps(t *testing.T) {
	mapsTests := []struct {
		expr string
	}{
		{expr: `{}.merge({}) == {}`},
		{expr: `{}.merge({'a': 1}) == {'a': 1}`},
		{expr: `{}.merge({'a': 2.1}) == {'a': 2.1}`},
		{expr: `{}.merge({'a': 'foo'}) == {'a': 'foo'}`},
		{expr: `{'a': 1}.merge({}) == {'a': 1}`},
		{expr: `{'a': 1}.merge({'b': 2}) == {'a': 1, 'b': 2}`},
		{expr: `{'a': 1}.merge({'a': 2, 'b': 2}) == {'a': 2, 'b': 2}`},
	}

	env := testMapsEnv(t)
	for i, tc := range mapsTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatal(err)
				} else if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func testMapsEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{
		Maps(),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Maps()) failed: %v", err)
	}
	return env
}
