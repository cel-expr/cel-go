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

package env_test

import (
	"math"
	"strings"
	"testing"

	"cel.dev/cel-go/common/env"
)

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"size", "size", 0},
		{"siz", "size", 1},          // deletion/insertion
		{"sizee", "size", 1},        // insertion
		{"site", "size", 1},         // substitution
		{"optioanl", "optional", 1}, // transposition
		{"contians", "contains", 1}, // transposition
		{"kitten", "sitting", 3},
		{"hello", "world", math.MaxInt32}, // cost > 5/3: early return math.MaxInt32
		{"abcdefghijk", "abcdefghijk", 0}, // 11 * 11 = 121 <= 4096 iterations
		{"abcdefghijklmnopqrstuvwxyz0123456", "abcdefghijklmnopqrstuvwxyz0123456", 0}, // 33 * 33 = 1089 <= 4096 iterations
		{strings.Repeat("a", 64), strings.Repeat("a", 64), 0},                          // 64 * 64 = 4096 <= 4096 iterations
		{strings.Repeat("a", 65), strings.Repeat("a", 65), math.MaxInt32},              // 65 * 65 = 4225 > 4096 iterations
		{"abcdefghij", "0123456789", math.MaxInt32},                                   // completely different: cost > 10/3: early return math.MaxInt32
		{"abcdefghijklmnop", "abcdefghij", 6},                                          // 16 vs 10: la >= 16 and 16 >= 1.5 * 10
		{"abcdefghij", "abcdefghijklmnop", 6},                                          // 10 vs 16: lb >= 16 and 16 >= 1.5 * 10
		{"abcdefghijklmnop", "abcdef", 10},                                             // 16 vs 6: la >= 16 and 16 >= 1.5 * 6
		{"abcdef", "abcdefghijklmnop", 10},                                             // 6 vs 16: lb >= 16 and 16 >= 1.5 * 6
	}

	for _, tc := range tests {
		got := env.EditDistance(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("EditDistance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCatalogFind(t *testing.T) {
	cat := env.NewCatalog(
		&env.CatalogSymbol{Name: "size", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "contains", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "optional", Kind: env.FunctionKind, Option: "cel.OptionalTypes()"},
		&env.CatalogSymbol{Name: "optional_type", Kind: env.TypeKind, Option: "cel.OptionalTypes()"},
		&env.CatalogSymbol{Name: "optMap", Kind: env.MacroKind, Option: "cel.OptionalTypes()"},
		&env.CatalogSymbol{Name: "username", Kind: env.VariableKind},
		&env.CatalogSymbol{Name: "lowerAscii", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "isInf", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "isNaN", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "isFinite", Kind: env.FunctionKind},
	)

	// Exact matches
	syms := cat.Find("size")
	if len(syms) != 1 || syms[0].Kind != env.FunctionKind {
		t.Errorf("Find('size') failed: syms=%+v", syms)
	}
	syms = cat.Find("optional")
	if len(syms) != 1 || syms[0].Option != "cel.OptionalTypes()" {
		t.Errorf("Find('optional') failed: syms=%+v", syms)
	}
	if syms := cat.Find("unknown_ident_with_no_similarity"); len(syms) != 0 {
		t.Errorf("expected Find('unknown...') to return empty, got %+v", syms)
	}

	// Similar matches (edit distance)
	tests := []struct {
		input     string
		wantNames []string
	}{
		{"siz", []string{"size"}},
		{"contians", []string{"contains"}},
		{"optioanl", []string{"optional"}},
		{"optMpa", []string{"optMap"}},
		{"usrname", []string{"username"}},
		// Fallback: prefix match
		{"lower", []string{"lowerAscii"}},
		// Fallback: prefix match with multiple candidates (ordered shortest to longest)
		{"is", []string{"isInf", "isNaN"}},
	}

	for _, tc := range tests {
		got := cat.Find(tc.input)
		var gotNames []string
		for _, s := range got {
			gotNames = append(gotNames, s.Name)
		}
		if len(gotNames) != len(tc.wantNames) {
			t.Errorf("Find(%q) = %v, want %v", tc.input, gotNames, tc.wantNames)
			continue
		}
		for i := range gotNames {
			if gotNames[i] != tc.wantNames[i] {
				t.Errorf("Find(%q)[%d] = %q, want %q", tc.input, i, gotNames[i], tc.wantNames[i])
			}
		}
	}

	// Distant / completely unrelated string
	if syms := cat.Find("completely_unrelated_long_string"); len(syms) != 0 {
		t.Errorf("expected no match for distant string, got %+v", syms)
	}
}

func TestCatalogEdgeCases(t *testing.T) {
	// Add nil and empty name
	cat := env.NewCatalog()
	cat.Add(nil)
	cat.Add(&env.CatalogSymbol{Name: ""})
	if syms := cat.Find("x"); len(syms) != 0 {
		t.Errorf("expected empty Find on empty catalog, got %+v", syms)
	}

	// Nil catalog Find
	var nilCat *env.Catalog
	if syms := nilCat.Find("test"); syms != nil {
		t.Errorf("expected nil for nil Catalog.Find, got %+v", syms)
	}
	if syms := cat.Find(""); syms != nil {
		t.Errorf("expected nil for empty query Find, got %+v", syms)
	}

	// Nil Config ToCatalog
	var nilConf *env.Config
	if c := nilConf.ToCatalog(); c == nil {
		t.Error("expected non-nil Catalog from nil Config.ToCatalog")
	}

	// Multiple edit distance candidates tied with different lengths (3 items with dist 1)
	tiedCat3 := env.NewCatalog(
		&env.CatalogSymbol{Name: "optD_", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "optC_", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "optA", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "optA", Kind: env.TypeKind}, // duplicate name
	)
	syms := tiedCat3.Find("opt_")
	if len(syms) != 2 {
		t.Fatalf("Find('opt_') returned %d results, want 2", len(syms))
	}
	if syms[0].Name != "optA" {
		t.Errorf("Find('opt_')[0] = %s, want optA", syms[0].Name)
	}

	// Multiple prefix candidates with duplicate names
	prefixCat := env.NewCatalog(
		&env.CatalogSymbol{Name: "test_longest_name", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "test_medium_name", Kind: env.FunctionKind},
		&env.CatalogSymbol{Name: "test_medium_name", Kind: env.TypeKind}, // duplicate name
		&env.CatalogSymbol{Name: "test_short_name", Kind: env.FunctionKind},
	)
	prefixSyms := prefixCat.Find("test")
	if len(prefixSyms) != 2 {
		t.Fatalf("Find('test') returned %d results, want 2", len(prefixSyms))
	}
	if prefixSyms[0].Name != "test_short_name" || prefixSyms[1].Name != "test_medium_name" {
		t.Errorf("Find('test') = [%s, %s], want [test_short_name, test_medium_name]", prefixSyms[0].Name, prefixSyms[1].Name)
	}
}

func TestConfigToCatalog(t *testing.T) {
	conf := env.NewConfig("test-env").
		SetContainer("my.pkg").
		AddImports(env.NewImport("my.other.pkg.User")).
		AddVariables(env.NewVariable("x", env.NewTypeDesc("int"))).
		AddFunctions(env.NewFunction("customFunc"))

	cat := conf.ToCatalog()
	syms := cat.Find("x")
	if len(syms) != 1 || syms[0].Kind != env.VariableKind {
		t.Errorf("Find('x') failed: syms=%+v", syms)
	}
	syms = cat.Find("customFunc")
	if len(syms) != 1 || syms[0].Kind != env.FunctionKind {
		t.Errorf("Find('customFunc') failed: syms=%+v", syms)
	}
	syms = cat.Find("my.pkg")
	if len(syms) != 1 || syms[0].Kind != env.NamespaceKind {
		t.Errorf("Find('my.pkg') failed: syms=%+v", syms)
	}
	syms = cat.Find("my.other.pkg.User")
	if len(syms) != 1 || syms[0].Kind != env.NamespaceKind {
		t.Errorf("Find('my.other.pkg.User') failed: syms=%+v", syms)
	}
}
