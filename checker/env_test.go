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

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/env"
	"cel.dev/cel-go/common/stdlib"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/parser"
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

func TestEnterExitScope(t *testing.T) {
	cat := env.NewCatalog(&env.CatalogSymbol{Name: "test_fn", Kind: env.FunctionKind})
	parent, err := NewEnv(
		containers.DefaultContainer,
		newTestRegistry(t),
		CrossTypeNumericComparisons(true),
		JSONFieldNames(true),
		Catalog(func() *env.Catalog { return cat }),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	child := parent.enterScope()
	if child.jsonFieldNames != true {
		t.Errorf("child.jsonFieldNames = %v, want true", child.jsonFieldNames)
	}
	if len(child.filteredOverloadIDs) != 0 {
		t.Errorf("child.filteredOverloadIDs = %v, want empty", child.filteredOverloadIDs)
	}
	if child.Catalog() != cat {
		t.Errorf("child.Catalog() = %v, want %v", child.Catalog(), cat)
	}

	grandchild := child.enterScope()
	if grandchild.jsonFieldNames != true {
		t.Errorf("grandchild.jsonFieldNames = %v, want true", grandchild.jsonFieldNames)
	}
	if grandchild.Catalog() != cat {
		t.Errorf("grandchild.Catalog() = %v, want %v", grandchild.Catalog(), cat)
	}

	exitedChild := grandchild.exitScope()
	if exitedChild.jsonFieldNames != true {
		t.Errorf("exitedChild.jsonFieldNames = %v, want true", exitedChild.jsonFieldNames)
	}
	if exitedChild.Catalog() != cat {
		t.Errorf("exitedChild.Catalog() = %v, want %v", exitedChild.Catalog(), cat)
	}

	exitedParent := exitedChild.exitScope()
	if exitedParent.jsonFieldNames != true {
		t.Errorf("exitedParent.jsonFieldNames = %v, want true", exitedParent.jsonFieldNames)
	}
	if exitedParent.Catalog() != cat {
		t.Errorf("exitedParent.Catalog() = %v, want %v", exitedParent.Catalog(), cat)
	}
}

func TestTypeErrorsCheckUndeclared(t *testing.T) {
	src := common.NewTextSource("test")
	errs := &typeErrors{errs: common.NewErrors(src)}

	// nil env checks
	if errs.checkUndeclaredIdent(nil, 1, common.NoLocation, "a", "b") {
		t.Errorf("checkUndeclaredIdent(nil, ...) = true, want false")
	}
	if errs.checkUndeclaredFunction(nil, 1, common.NoLocation, "a", "b") {
		t.Errorf("checkUndeclaredFunction(nil, ...) = true, want false")
	}
	if errs.hasDeclaredPrefix(nil, "a") {
		t.Errorf("hasDeclaredPrefix(nil, ...) = true, want false")
	}

	// env without catalog
	noCatEnv, err := NewEnv(containers.DefaultContainer, newTestRegistry(t))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	if errs.checkUndeclaredIdent(noCatEnv, 1, common.NoLocation, "a", "b") {
		t.Errorf("checkUndeclaredIdent(noCatEnv, ...) = true, want false")
	}
	if errs.checkUndeclaredFunction(noCatEnv, 1, common.NoLocation, "a", "b") {
		t.Errorf("checkUndeclaredFunction(noCatEnv, ...) = true, want false")
	}

	// env with catalog and declared prefix
	cat := env.NewCatalog(&env.CatalogSymbol{Name: "a.b", Kind: env.FunctionKind})
	catEnv, err := NewEnv(
		containers.DefaultContainer,
		newTestRegistry(t),
		Catalog(func() *env.Catalog { return cat }),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	catEnv.AddIdents(decls.NewVariable("a", types.IntType))

	// "a" is declared, so "a.b" has declared prefix "a"
	if errs.checkUndeclaredIdent(catEnv, 1, common.NoLocation, "a", "b") {
		t.Errorf("checkUndeclaredIdent(catEnv with 'a' declared, ...) = true, want false")
	}
	if errs.checkUndeclaredFunction(catEnv, 1, common.NoLocation, "a", "b") {
		t.Errorf("checkUndeclaredFunction(catEnv with 'a' declared, ...) = true, want false")
	}

	// single qualifier
	if errs.checkUndeclaredIdent(catEnv, 1, common.NoLocation, "single") {
		t.Errorf("checkUndeclaredIdent with single qualifier = true, want false")
	}
}


