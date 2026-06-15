// Copyright 2020 Google LLC
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

package containers

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/cel-go/common/ast"
)

func TestContainers_ResolveCandidateNames(t *testing.T) {
	c, err := NewContainer(Name("a.b.c.M.N"))
	if err != nil {
		t.Fatal(err)
	}
	names := c.ResolveCandidateNames("R.s")
	want := []string{
		"a.b.c.M.N.R.s",
		"a.b.c.M.R.s",
		"a.b.c.R.s",
		"a.b.R.s",
		"a.R.s",
		"R.s",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
}

func TestContainers_ResolveCandidateNames_FullyQualifiedName(t *testing.T) {
	c, err := NewContainer(Name("a.b.c.M.N"))
	if err != nil {
		t.Fatal(err)
	}
	// The leading '.' indicates the name is already fully-qualified.
	names := c.ResolveCandidateNames(".R.s")
	want := []string{"R.s"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
}

func TestContainers_ResolveCandidateNames_EmptyContainer(t *testing.T) {
	names := DefaultContainer.ResolveCandidateNames("R.s")
	want := []string{"R.s"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
}

func TestContainers_Alias(t *testing.T) {
	cont, err := DefaultContainer.Extend(Alias("my.example.pkg.verbose", "bigex"))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	if !reflect.DeepEqual(cont.ResolveCandidateNames("bigex.Execute"), []string{"my.example.pkg.verbose.Execute"}) {
		t.Errorf("ResolveCandidateNames() got %s, wanted %s", cont.ResolveCandidateNames("bigex.Execute"), "my.example.pkg.verbose.Execute")
	}

	cont, err = DefaultContainer.Extend(Alias("really_long_package_name", "short"))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	if !reflect.DeepEqual(cont.ResolveCandidateNames("short"), []string{"really_long_package_name"}) {
		t.Errorf("ResolveCandidateNames() got %s, wanted %s", cont.ResolveCandidateNames("short"), "really_long_package_name")
	}
	if !reflect.DeepEqual(cont.ResolveCandidateNames("short.field"), []string{"really_long_package_name.field"}) {
		t.Errorf("ResolveCandidateNames() got %s, wanted %s", cont.ResolveCandidateNames("short.field"), "really_long_package_name.field")
	}
}

func TestContainers_Abbrevs(t *testing.T) {
	abbr, err := DefaultContainer.Extend(Abbrevs("my.alias.R"))
	if err != nil {
		t.Fatal(err)
	}
	names := abbr.ResolveCandidateNames("R")
	want := []string{
		"my.alias.R",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
	c, err := NewContainer(Name("a.b.c"), Abbrevs("my.alias.R"))
	if err != nil {
		t.Fatal(err)
	}
	names = c.ResolveCandidateNames("R")
	want = []string{
		"my.alias.R",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
	names = c.ResolveCandidateNames("R.S.T")
	want = []string{
		"my.alias.R.S.T",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
	names = c.ResolveCandidateNames("S")
	want = []string{
		"a.b.c.S",
		"a.b.S",
		"a.S",
		"S",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, wanted %v", names, want)
	}
}

func TestContainers_Aliasing_Errors(t *testing.T) {
	type aliasDef struct {
		name  string
		alias string
	}
	tests := []struct {
		container string
		abbrevs   []string
		aliases   []aliasDef
		err       string
	}{
		{
			abbrevs: []string{"my.alias.R", "yer.other.R"},
			err: "abbreviation collides with existing reference: " +
				"name=yer.other.R, abbreviation=R, existing=my.alias.R",
		},
		{
			container: "a.b.c.M.N",
			abbrevs:   []string{"my.alias.a", "yer.other.b"},
			err: "abbreviation collides with container name: name=my.alias.a, " +
				"abbreviation=a, container=a.b.c.M.N",
		},
		{
			abbrevs: []string{".bad"},
			err:     "invalid qualified name: .bad, wanted name of the form 'qualified.name'",
		},
		{
			abbrevs: []string{"bad.alias."},
			err:     "invalid qualified name: bad.alias., wanted name of the form 'qualified.name'",
		},
		{
			abbrevs: []string{"   bad_alias1"},
			err:     "invalid qualified name: bad_alias1, wanted name of the form 'qualified.name'",
		},
		{
			abbrevs: []string{"   bad.alias!  "},
			err:     "invalid qualified name: bad.alias!, wanted name of the form 'qualified.name'",
		},
		{
			aliases: []aliasDef{{name: "my.alias", alias: "b.c"}},
			err:     "alias must be non-empty and simple (not qualified): alias=b.c",
		},
		{
			aliases: []aliasDef{{name: ".my.qual.name", alias: "a'"}},
			err:     "qualified name must not begin with a leading '.': .my.qual.name",
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			opts := []ContainerOption{}
			if tc.container != "" {
				opts = append(opts, Name(tc.container))
			}
			if len(tc.abbrevs) != 0 {
				opts = append(opts, Abbrevs(tc.abbrevs...))
			}
			if len(tc.aliases) != 0 {
				for _, a := range tc.aliases {
					opts = append(opts, Alias(a.name, a.alias))
				}
			}
			_, err := NewContainer(opts...)
			if err == nil {
				t.Fatalf("NewContainer() succeeded, wanted err %s", tc.err)
			}
			if err.Error() != tc.err {
				t.Errorf("NewContainer() got error %s, wanted error %s", err.Error(), tc.err)
			}
		})
	}
}

func TestContainers_Extend_Alias(t *testing.T) {
	c, err := DefaultContainer.Extend(Alias("test.alias", "alias"))
	if err != nil {
		t.Fatal(err)
	}
	if c.AliasSet()["alias"] != "test.alias" {
		t.Errorf("got alias %v wanted 'test.alias'", c.AliasSet())
	}
	c, err = c.Extend(Name("with.container"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != "with.container" {
		t.Errorf("got container name %s, wanted 'with.container'", c.Name())
	}
	if c.AliasSet()["alias"] != "test.alias" {
		t.Errorf("got alias %v wanted 'test.alias'", c.AliasSet())
	}
}

func TestContainers_Extend_Name(t *testing.T) {
	c, err := DefaultContainer.Extend(Name(""))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != "" {
		t.Errorf("got %v, wanted empty name", c.Name())
	}
	c, err = DefaultContainer.Extend(Name("hello.container"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != "hello.container" {
		t.Errorf("got container name %s, wanted 'hello.container'", c.Name())
	}
	c, err = c.Extend(Name("goodbye.container"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != "goodbye.container" {
		t.Errorf("got container name %s, wanted 'goodbye.container'", c.Name())
	}
	c, err = c.Extend(Name(".bad.container"))
	if err == nil {
		t.Errorf("got %v, wanted error", c.Name())
	}
}

func TestContainers_ToQualifiedName(t *testing.T) {
	fac := ast.NewExprFactory()
	ident := fac.NewIdent(1, "var")
	idName, found := ToQualifiedName(ident)
	if !found {
		t.Errorf("got not found from %v expr, wanted found", ident)
	}
	if idName != "var" {
		t.Errorf("got %v, wanted 'var'", idName)
	}
	sel := fac.NewSelect(2, ident, "qualifier")
	qualName, found := ToQualifiedName(sel)
	if !found {
		t.Errorf("got not found from %v expr, wanted found", sel)
	}
	if qualName != "var.qualifier" {
		t.Errorf("got %v, wanted 'var.qualifier'", qualName)
	}

	pres := fac.NewPresenceTest(2, ident, "qualifier")
	_, found = ToQualifiedName(pres)
	if found {
		t.Error("got found, wanted not found for test-only expression")
	}

	unary := fac.NewCall(2, "!_", ident)
	sel = fac.NewSelect(3, unary, "qualifier")
	_, found = ToQualifiedName(sel)
	if found {
		t.Errorf("got found, wanted not found for %v", sel)
	}
}
