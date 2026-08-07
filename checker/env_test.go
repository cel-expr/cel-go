// Copyright 2018 Google LLC
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

package checker

import (
	"strings"
	"testing"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"
)

func TestOverlappingMacro(t *testing.T) {
	env := newStdEnv(t)
	hasFn, err := decls.NewFunction("has",
		decls.Overload("has", []*types.Type{types.StringType}, types.BoolType))
	if err != nil {
		t.Fatalf("decls.NewFunction() failed: %v", err)
	}
	err = env.AddFunctions(hasFn)
	if err == nil {
		t.Error("Got nil, wanted error")
	} else if !strings.Contains(err.Error(), "overlapping macro") {
		t.Errorf("Got %v, wanted overlapping macro error", err)
	}
}

func TestCopyDeclarations(t *testing.T) {
	src := common.NewTextSource(`1 + 2 != 3 - 4`)
	parsedAst, errors := parser.Parse(src)
	if len(errors.GetErrors()) > 0 {
		t.Fatalf("Unexpected parse errors: %v", errors.ToDisplayString())
	}

	env := newStdEnv(t)
	_, errors = Check(parsedAst, src, env)
	if len(errors.GetErrors()) > 0 {
		t.Fatalf("Check(parsedAst, src, env): %v", errors.ToDisplayString())
	}

	copy, err := NewEnv(containers.DefaultContainer, newTestRegistry(t), ValidatedDeclarations(env))
	if err != nil {
		t.Fatalf("NewEnv(container, registry, CopyDeclarations(env)) failed %v: ", err)
	}
	_, errors = Check(parsedAst, src, copy)
	if len(errors.GetErrors()) > 0 {
		t.Fatalf("Check(parsedAst, src, copy): %v", errors.ToDisplayString())
	}
}

func BenchmarkNewStdEnv(b *testing.B) {
	for i := 0; i < b.N; i++ {
		env, err := NewEnv(containers.DefaultContainer, newTestRegistry(b))
		if err != nil {
			b.Fatalf("NewEnv() failed: %v", err)
		}
		err = env.AddFunctions(stdlib.Functions()...)
		if err != nil {
			b.Fatalf("env.AddFunctions(stdlib.Functions()...) failed: %v", err)
		}
	}
}

func newStdEnv(t *testing.T) *Env {
	t.Helper()
	env, err := NewEnv(containers.DefaultContainer, newTestRegistry(t))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	err = env.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Fatalf("env.Add(stdlib.Functions()...) failed: %v", err)
	}
	return env
}

func newTestRegistry(t testing.TB) *types.Registry {
	t.Helper()
	reg, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}
	return reg
}
