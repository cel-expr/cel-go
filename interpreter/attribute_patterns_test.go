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

package interpreter

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/types"
)

// attr describes a simplified format for specifying common Attribute and Qualifier values for
// use in pattern matching tests.
type attr struct {
	// unchecked indicates whether the attribute has not been type-checked and thus not gone
	// the variable and function resolution step.
	unchecked bool
	// container simulates the expression container and is only relevant on 'unchecked' test inputs
	// as the container is used to resolve the potential fully qualified variable names represented
	// by an identifier or select expression.
	container string
	// variable name, fully qualified unless the attr is marked as unchecked=true
	name string
	// quals contains a list of static qualifiers.
	quals []any
}

// patternTest describes a pattern, and a set of matches and misses for the pattern to highlight
// what the pattern will and will not match.
type patternTest struct {
	pattern *AttributePattern
	matches []attr
	misses  []attr
}

var patternTests = map[string]patternTest{
	"var": {
		pattern: NewAttributePattern("var"),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{"field"}},
		},
		misses: []attr{
			{name: "ns.var"},
		},
	},
	"var_namespace": {
		pattern: NewAttributePattern("ns.app.var"),
		matches: []attr{
			{name: "ns.app.var"},
			{name: "ns.app.var", quals: []any{int64(0)}},
			{
				name:      "ns",
				quals:     []any{"app", "var", "foo"},
				container: "ns.app",
				unchecked: true,
			},
		},
		misses: []attr{
			{name: "ns.var"},
			{
				name:      "ns",
				quals:     []any{"var"},
				container: "ns.app",
				unchecked: true,
			},
		},
	},
	"var_field": {
		pattern: NewAttributePattern("var").QualString("field"),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{"field"}},
			{name: "var", quals: []any{"field"}, unchecked: true},
			{name: "var", quals: []any{"field", uint64(1)}},
		},
		misses: []attr{
			{name: "var", quals: []any{"other"}},
		},
	},
	"var_index": {
		pattern: NewAttributePattern("var").QualInt(0),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{int64(0)}},
			{name: "var", quals: []any{float64(0)}},
			{name: "var", quals: []any{int64(0), false}},
			{name: "var", quals: []any{uint64(0)}},
		},
		misses: []attr{
			{name: "var", quals: []any{int64(1), false}},
		},
	},
	"var_index_uint": {
		pattern: NewAttributePattern("var").QualUint(1),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{uint64(1)}},
			{name: "var", quals: []any{uint64(1), true}},
			{name: "var", quals: []any{int64(1), false}},
		},
		misses: []attr{
			{name: "var", quals: []any{uint64(0)}},
		},
	},
	"var_index_bool": {
		pattern: NewAttributePattern("var").QualBool(true),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{true}},
			{name: "var", quals: []any{true, "name"}},
		},
		misses: []attr{
			{name: "var", quals: []any{false}},
			{name: "none"},
		},
	},
	"var_wildcard": {
		pattern: NewAttributePattern("ns.var").Wildcard(),
		matches: []attr{
			{name: "ns.var"},
			// The unchecked attributes consider potential namespacing and field selection
			// when testing variable names.
			{
				name:      "var",
				quals:     []any{true},
				container: "ns",
				unchecked: true,
			},
			{
				name:      "var",
				quals:     []any{"name"},
				container: "ns",
				unchecked: true,
			},
			{
				name:      "var",
				quals:     []any{"name"},
				container: "ns",
				unchecked: true,
			},
		},
		misses: []attr{
			{name: "var", quals: []any{false}},
			{name: "none"},
		},
	},
	"var_wildcard_field": {
		pattern: NewAttributePattern("var").Wildcard().QualString("field"),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{true}},
			{name: "var", quals: []any{int64(10), "field"}},
		},
		misses: []attr{
			{name: "var", quals: []any{int64(10), "other"}},
		},
	},
	"var_wildcard_wildcard": {
		pattern: NewAttributePattern("var").Wildcard().Wildcard(),
		matches: []attr{
			{name: "var"},
			{name: "var", quals: []any{true}},
			{name: "var", quals: []any{int64(10), "field"}},
		},
		misses: []attr{
			{name: "none"},
		},
	},
}

func TestAttributePattern_UnknownResolution(t *testing.T) {
	reg := newTestRegistry(t)
	for nm, tc := range patternTests {
		tst := tc
		t.Run(nm, func(t *testing.T) {
			for i, match := range tst.matches {
				m := match
				t.Run(fmt.Sprintf("match[%d]", i), func(t *testing.T) {
					var err error
					cont := containers.DefaultContainer
					if m.unchecked {
						cont, err = containers.NewContainer(containers.Name(m.container))
						if err != nil {
							t.Fatal(err)
						}
					}
					fac := NewPartialAttributeFactory(cont, reg, reg)
					attr := genAttr(fac, m)
					partVars, _ := NewPartialActivation(EmptyActivation(), tst.pattern)
					val, err := attr.Resolve(partVars)
					if err != nil {
						t.Fatalf("Got error: %s, wanted unknown", err)
					}
					_, isUnk := val.(*types.Unknown)
					if !isUnk {
						t.Fatalf("Got value %v, wanted unknown", val)
					}
				})
			}
			for i, miss := range tst.misses {
				m := miss
				t.Run(fmt.Sprintf("miss[%d]", i), func(t *testing.T) {
					cont := containers.DefaultContainer
					if m.unchecked {
						var err error
						cont, err = containers.NewContainer(containers.Name(m.container))
						if err != nil {
							t.Fatal(err)
						}
					}
					fac := NewPartialAttributeFactory(cont, reg, reg)
					attr := genAttr(fac, m)
					partVars, _ := NewPartialActivation(EmptyActivation(), tst.pattern)
					val, err := attr.Resolve(partVars)
					if err == nil {
						t.Fatalf("Got value: %s, wanted error", val)
					}
				})
			}
		})
	}
}

func TestAttributePattern_CrossReference(t *testing.T) {
	reg := newTestRegistry(t)
	fac := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
	a := fac.AbsoluteAttribute(1, "a")
	b := fac.AbsoluteAttribute(2, "b")
	a.AddQualifier(b)

	// Ensure that var a[b], the dynamic index into var 'a' is the unknown value
	// returned from attribute resolution.
	partVars, _ := NewPartialActivation(
		map[string]any{"a": []int64{1, 2}},
		NewAttributePattern("b"))
	val, err := a.Resolve(partVars)
	if err != nil {
		t.Fatal(err)
	}
	if !types.NewUnknown(2, types.NewAttributeTrail("b")).Contains(val.(*types.Unknown)) {
		t.Errorf("Got %v, wanted unknown attribute id for 'b' (2)", val)
	}

	// Ensure that a[b], the dynamic index into var 'a' is the unknown value
	// returned from attribute resolution. Note, both 'a' and 'b' have unknown attribute
	// patterns specified. This changes the evaluation behavior slightly, but the end
	// result is the same.
	partVars, _ = NewPartialActivation(
		map[string]any{"a": []int64{1, 2}},
		NewAttributePattern("a").QualInt(0),
		NewAttributePattern("b"))
	val, err = a.Resolve(partVars)
	if err != nil {
		t.Fatal(err)
	}
	if !types.NewUnknown(2, types.NewAttributeTrail("b")).Contains(val.(*types.Unknown)) {
		t.Errorf("Got %v, wanted unknown attribute id for 'b' (2)", val)
	}

	// Note, that only 'a[0].c' will result in an unknown result since both 'a' and 'b'
	// have values. However, since the attribute being pattern matched is just 'a.b',
	// the outcome will indicate that 'a[b]' is unknown.
	partVars, _ = NewPartialActivation(
		map[string]any{"a": []int64{1, 2}, "b": 0},
		NewAttributePattern("a").QualInt(0).QualString("c"))
	val, err = a.Resolve(partVars)
	if err != nil {
		t.Fatal(err)
	}
	unkAttr := types.NewAttributeTrail("a")
	types.QualifyAttribute[int64](unkAttr, 0)
	wantUnk := types.NewUnknown(2, unkAttr)
	if !wantUnk.Contains(val.(*types.Unknown)) {
		t.Errorf("Got %v, wanted unknown attribute id for %v", val, wantUnk)
	}

	// Test a positive case that returns a valid value even though the attribugte factory
	// is the partial attribute factory.
	partVars, _ = NewPartialActivation(
		map[string]any{"a": []int64{1, 2}, "b": 0})
	val, err = a.Resolve(partVars)
	if err != nil {
		t.Fatal(err)
	}
	if val != int64(1) {
		t.Errorf("Got %v, wanted 1 for a[b]", val)
	}

	// Ensure the unknown attribute id moves when the attribute becomes more specific.
	partVars, _ = NewPartialActivation(
		map[string]any{"a": []int64{1, 2}, "b": 0},
		NewAttributePattern("a").QualInt(0).QualString("c"))
	// Qualify a[b] with 'c', a[b].c
	c, _ := fac.NewQualifier(nil, 3, "c", false)
	a.AddQualifier(c)
	// The resolve step should return unknown
	val, err = a.Resolve(partVars)
	if err != nil {
		t.Fatal(err)
	}
	unkAttr = types.NewAttributeTrail("a")
	types.QualifyAttribute[int64](unkAttr, 0)
	types.QualifyAttribute[string](unkAttr, "c")
	wantUnk = types.NewUnknown(3, unkAttr)
	if !wantUnk.Contains(val.(*types.Unknown)) {
		t.Errorf("Got %v, wanted unknown attribute id for %v", val, wantUnk)
	}
}

func genAttr(fac AttributeFactory, a attr) Attribute {
	id := int64(1)
	var attr Attribute
	if a.unchecked {
		attr = fac.MaybeAttribute(1, a.name)
	} else {
		attr = fac.AbsoluteAttribute(1, a.name)
	}
	for _, q := range a.quals {
		qual, _ := fac.NewQualifier(nil, id, q, false)
		attr.AddQualifier(qual)
		id++
	}
	return attr
}

func TestAttributePattern_LocallyBound(t *testing.T) {
	reg := newTestRegistry(t)
	fac := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)

	// Create a partial activation that designates "x" and "y" as unknown.
	partAct, err := NewPartialActivation(EmptyActivation(), NewAttributePattern("x"), NewAttributePattern("y"))
	if err != nil {
		t.Fatal(err)
	}

	frame := mustNewExecutionFrame(t, partAct)
	defer frame.Close()

	// Create folder (representing a local scope for variable "x" only)
	fold := &evalFold{
		iterVar: "x",
		adapter: types.DefaultTypeAdapter,
	}
	fld := newFolder(fold, frame)
	defer releaseFolder(fld)

	// Push onto the frame stack to simulate local variable scope
	frame1 := frame.Push(fld)
	defer frame1.Pop()

	// 1. Resolve attribute matching "x".
	// Because "x" is locally bound in fld, it should not resolve to types.Unknown.
	attrX := fac.AbsoluteAttribute(1, "x")
	valX, err := attrX.Resolve(frame1)
	if err != nil {
		t.Fatal(err)
	}
	if _, isUnk := valX.(*types.Unknown); isUnk {
		t.Errorf("Resolve(x) got unknown, wanted value (since x is locally bound)")
	}

	// 2. Resolve attribute matching "y".
	// Because "y" is NOT locally bound in any local scope layer, it should resolve to types.Unknown.
	attrY := fac.AbsoluteAttribute(2, "y")
	valY, err := attrY.Resolve(frame1)
	if err != nil {
		t.Fatal(err)
	}
	if _, isUnk := valY.(*types.Unknown); !isUnk {
		t.Errorf("Resolve(y) got %v, wanted types.Unknown (since y is not locally bound)", valY)
	}
}

func TestQualifierValueEquals(t *testing.T) {
	tests := []struct {
		name      string
		qualifier interface {
			QualifierValueEquals(any) bool
		}
		cases []struct {
			input any
			want  bool
		}
	}{
		{
			name:      "fieldQualifier",
			qualifier: &fieldQualifier{Name: "foo"},
			cases: []struct {
				input any
				want  bool
			}{
				{input: "foo", want: true},
				{input: "bar", want: false},
				{input: 123, want: false},
				{input: true, want: false},
				{input: nil, want: false},
			},
		},
		{
			name:      "stringQualifier",
			qualifier: &stringQualifier{value: "hello"},
			cases: []struct {
				input any
				want  bool
			}{
				{input: "hello", want: true},
				{input: "world", want: false},
				{input: 123, want: false},
				{input: nil, want: false},
			},
		},
		{
			name:      "boolQualifier",
			qualifier: &boolQualifier{value: true},
			cases: []struct {
				input any
				want  bool
			}{
				{input: true, want: true},
				{input: false, want: false},
				{input: "true", want: false},
				{input: 1, want: false},
				{input: nil, want: false},
			},
		},
		{
			name:      "intQualifier",
			qualifier: &intQualifier{celValue: types.Int(42)},
			cases: []struct {
				input any
				want  bool
			}{
				{input: int64(42), want: true},
				{input: int32(42), want: true},
				{input: uint64(42), want: true},
				{input: float64(42.0), want: true},
				{input: types.Int(42), want: true},
				{input: types.Double(42.0), want: true},
				{input: int64(100), want: false},
				{input: "42", want: false},
				{input: nil, want: false},
			},
		},
		{
			name:      "uintQualifier",
			qualifier: &uintQualifier{celValue: types.Uint(100)},
			cases: []struct {
				input any
				want  bool
			}{
				{input: uint64(100), want: true},
				{input: int64(100), want: true},
				{input: float64(100.0), want: true},
				{input: types.Uint(100), want: true},
				{input: uint64(50), want: false},
				{input: "100", want: false},
				{input: nil, want: false},
			},
		},
		{
			name:      "doubleQualifier",
			qualifier: &doubleQualifier{celValue: types.Double(3.14)},
			cases: []struct {
				input any
				want  bool
			}{
				{input: float64(3.14), want: true},
				{input: types.Double(3.14), want: true},
				{input: float64(2.71), want: false},
				{input: int64(3), want: false},
				{input: "3.14", want: false},
				{input: nil, want: false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, c := range tc.cases {
				if got := tc.qualifier.QualifierValueEquals(c.input); got != c.want {
					t.Errorf("%s.QualifierValueEquals(%v (%T)) = %t, wanted %t", tc.name, c.input, c.input, got, c.want)
				}
			}
		})
	}
}

func TestPartialAttributeFactory_MaybeAttributeGloballyNamespaced(t *testing.T) {
	reg := newTestRegistry(t)
	fac := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
	dotAttr := fac.MaybeAttribute(10, ".global_var")
	if dotAttr == nil {
		t.Error("MaybeAttribute(.global_var) got nil")
	}
}

func TestPartialAttributeFactory_ResolveUnknownQualifier(t *testing.T) {
	reg := newTestRegistry(t)
	fac := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
	partVars, _ := NewPartialActivation(
		map[string]any{"a": map[string]any{"b": 1}},
		NewAttributePattern("a").QualString("b"))
	a := fac.AbsoluteAttribute(1, "a")
	b, _ := fac.NewQualifier(nil, 2, "b", false)
	a.AddQualifier(b)
	val, err := a.Resolve(partVars)
	if err != nil {
		t.Fatal(err)
	}
	unkVal, ok := val.(*types.Unknown)
	if !ok {
		t.Fatalf("Resolve(a.b) got %v (%T), wanted *types.Unknown", val, val)
	}
	unkAttr := types.NewAttributeTrail("a")
	types.QualifyAttribute[string](unkAttr, "b")
	wantUnk := types.NewUnknown(2, unkAttr)
	if !wantUnk.Contains(unkVal) {
		t.Errorf("Resolve(a.b) got unknown %v, wanted %v", unkVal, wantUnk)
	}
}
