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

package interpreter

import (
	"testing"
	"time"

	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func TestActivation(t *testing.T) {
	act, err := NewActivation(map[string]any{"a": types.True})
	if err != nil {
		t.Fatalf("Got err: %v, wanted activation", err)
	}
	_, err = NewActivation(act)
	if err != nil {
		t.Fatalf("Got err: %v, wanted activation", err)
	}
	act3, err := NewActivation("")
	if err == nil {
		t.Fatalf("Got %v, wanted err", act3)
	}
}

func TestActivation_Resolve(t *testing.T) {
	activation, _ := NewActivation(map[string]any{"a": types.True})
	if val, found := activation.ResolveName("a"); !found || val != types.True {
		t.Error("Activation failed to resolve 'a'")
	}
}

func TestActivation_ResolveLazy(t *testing.T) {
	var v ref.Val
	now := func() ref.Val {
		if v == nil {
			v = types.DefaultTypeAdapter.NativeToValue(time.Now().Unix())
		}
		return v
	}
	a, _ := NewActivation(map[string]any{
		"now": now,
	})
	first, _ := a.ResolveName("now")
	second, _ := a.ResolveName("now")
	if first != second {
		t.Errorf("Got different second, "+
			"expected same as first: 1:%v 2:%v", first, second)
	}
}

func TestActivation_ResolveLazyAny(t *testing.T) {
	var v any
	now := func() any {
		if v == nil {
			v = time.Now().Unix()
		}
		return v
	}
	a, _ := NewActivation(map[string]any{
		"now": now,
	})
	first, _ := a.ResolveName("now")
	second, _ := a.ResolveName("now")
	if first != second {
		t.Errorf("Got different second, "+
			"expected same as first: 1:%v 2:%v", first, second)
	}
}

func TestHierarchicalActivation(t *testing.T) {
	// compose a parent with more properties than the child
	parent, _ := NewActivation(map[string]any{
		"a": types.String("world"),
		"b": types.Int(-42),
	})
	// compose the child such that it shadows the parent
	child, _ := NewActivation(map[string]any{
		"a": types.True,
		"c": types.String("universe"),
	})
	combined := NewHierarchicalActivation(parent, child)

	// Resolve the shadowed child value.
	if val, found := combined.ResolveName("a"); !found || val != types.True {
		t.Error("Activation failed to resolve shadow value of 'a'")
	}
	// Resolve the parent only value.
	if val, found := combined.ResolveName("b"); !found || val.(types.Int) != -42 {
		t.Error("Activation failed to resolve parent value of 'b'")
	}
	// Resolve the child only value.
	if val, found := combined.ResolveName("c"); !found || val.(types.String) != "universe" {
		t.Error("Activation failed to resolve child value of 'c'")
	}
}

func TestAsPartialActivation(t *testing.T) {
	// compose a parent with more properties than the child
	parent, _ := NewPartialActivation(map[string]any{
		"a": types.String("world"),
		"b": types.Int(-42),
	}, NewAttributePattern("c"))
	// compose the child such that it shadows the parent
	child, _ := NewActivation(map[string]any{
		"d": types.String("universe"),
	})
	combined := NewHierarchicalActivation(parent, child)

	// Resolve the shadowed child value.
	if part, found := AsPartialActivation(combined); found {
		if part != parent {
			t.Errorf("AsPartialActivation() got %v, wanted %v", part, parent)
		}
	} else {
		t.Error("AsPartialActivation() failed, did not find parent partial activation")
	}
}

func TestIsLocalVariableNested(t *testing.T) {
	parentAct := EmptyActivation()
	frame := mustNewExecutionFrame(t, parentAct)
	defer frame.Close()

	// Outer comprehension scope (e.g. fold1)
	fold1 := &evalFold{
		accuVar:  "accu1",
		iterVar:  "iter1",
		iterVar2: "iter1_2",
	}
	fld1 := newFolder(fold1, frame)
	defer releaseFolder(fld1)

	// Push outer scope
	frame1 := frame.Push(fld1)
	defer frame1.Pop()

	// Inner comprehension scope (e.g. fold2)
	fold2 := &evalFold{
		accuVar: "accu2",
		iterVar: "iter2",
	}
	fld2 := newFolder(fold2, frame1)
	defer releaseFolder(fld2)

	// Push inner scope
	frame2 := frame1.Push(fld2)
	defer frame2.Pop()

	// Verify localVariableHolder implementations and recursive checks
	tests := []struct {
		name      string
		varName   string
		wantLocal bool
	}{
		{"inner accu", "accu2", true},
		{"inner iter", "iter2", true},
		{"outer accu", "accu1", true},
		{"outer iter", "iter1", true},
		{"outer iter2", "iter1_2", true},
		{"global var", "x", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := frame2.IsLocalVariable(tc.varName); got != tc.wantLocal {
				t.Errorf("IsLocalVariable(%q) = %t, wanted %t", tc.varName, got, tc.wantLocal)
			}
		})
	}
}

func TestActivation_NewActivationNilInput(t *testing.T) {
	if _, err := NewActivation(nil); err == nil {
		t.Error("NewActivation(nil) wanted error, got nil")
	}
}

func TestPartialActivation_NewPartialActivationNilInput(t *testing.T) {
	if _, err := NewPartialActivation(nil); err == nil {
		t.Error("NewPartialActivation(nil) wanted error, got nil")
	}
}

func TestAsPartialActivation_NonPartialActivation(t *testing.T) {
	standardAct, _ := NewActivation(map[string]any{"a": 1})
	if _, found := AsPartialActivation(standardAct); found {
		t.Error("AsPartialActivation(standardAct) wanted false, got true")
	}
}

// funcOf returns a late-bound implementation which reports the tag it was created with, so that
// tests can identify which of several bindings was resolved.
func funcOf(tag string) functions.LateBoundOp {
	return func(overloadID string, args ...ref.Val) ref.Val {
		return types.String(tag)
	}
}

// resolveFunction performs the lookup the way an evaluation does: the activation which supplies
// the bindings is located once, then consulted by name.
func resolveFunction(t *testing.T, vars Activation, name string) (functions.LateBoundOp, bool) {
	t.Helper()
	fa, err := FindFunctionActivation(vars)
	if err != nil {
		t.Fatalf("FindFunctionActivation() failed: %v", err)
	}
	if fa == nil {
		return nil, false
	}
	return fa.ResolveFunction(name)
}

func TestFunctionActivationComposition(t *testing.T) {
	tests := []struct {
		name string
		// vars builds the activation under test.
		vars func(t *testing.T) Activation
		// wantFuncs maps a function name to the tag of the binding which should be resolved,
		// where an empty tag indicates that the name should not resolve.
		wantFuncs map[string]string
		// wantVars maps a variable name to the value which should be resolved.
		wantVars map[string]any
		// wantPartial indicates whether the activation exposes unknown attribute patterns.
		wantPartial bool
	}{
		{
			name: "functions over a map input",
			vars: func(t *testing.T) Activation {
				return mustFunctionVars(t, map[string]any{"a": 1}, map[string]functions.LateBoundOp{"f": funcOf("f")})
			},
			wantFuncs: map[string]string{"f": "f", "g": ""},
			wantVars:  map[string]any{"a": 1},
		},
		{
			name: "functions over an activation input",
			vars: func(t *testing.T) Activation {
				base := mustActivation(t, map[string]any{"a": 1})
				return mustFunctionVars(t, base, map[string]functions.LateBoundOp{"f": funcOf("f")})
			},
			wantFuncs: map[string]string{"f": "f"},
			wantVars:  map[string]any{"a": 1},
		},
		{
			name: "no functions supplied",
			vars: func(t *testing.T) Activation {
				return mustActivation(t, map[string]any{"a": 1})
			},
			wantFuncs: map[string]string{"f": ""},
			wantVars:  map[string]any{"a": 1},
		},
		{
			name: "empty activation",
			vars: func(t *testing.T) Activation {
				return EmptyActivation()
			},
			wantFuncs: map[string]string{"f": ""},
		},
		{
			name: "partial over functions",
			vars: func(t *testing.T) Activation {
				fnVars := mustFunctionVars(t, map[string]any{"a": 1}, map[string]functions.LateBoundOp{"f": funcOf("f")})
				return mustPartialVars(t, fnVars, NewAttributePattern("b"))
			},
			wantFuncs:   map[string]string{"f": "f"},
			wantVars:    map[string]any{"a": 1},
			wantPartial: true,
		},
		{
			name: "functions over partial",
			vars: func(t *testing.T) Activation {
				partial := mustPartialVars(t, map[string]any{"a": 1}, NewAttributePattern("b"))
				return mustFunctionVars(t, partial, map[string]functions.LateBoundOp{"f": funcOf("f")})
			},
			wantFuncs:   map[string]string{"f": "f"},
			wantVars:    map[string]any{"a": 1},
			wantPartial: true,
		},
		{
			name: "functions in the hierarchical parent",
			vars: func(t *testing.T) Activation {
				parent := mustFunctionVars(t, map[string]any{"a": 1}, map[string]functions.LateBoundOp{"f": funcOf("parent")})
				return NewHierarchicalActivation(parent, mustActivation(t, map[string]any{"b": 2}))
			},
			wantFuncs: map[string]string{"f": "parent"},
			wantVars:  map[string]any{"a": 1, "b": 2},
		},
		{
			name: "functions in the hierarchical child",
			vars: func(t *testing.T) Activation {
				child := mustFunctionVars(t, map[string]any{"b": 2}, map[string]functions.LateBoundOp{"f": funcOf("child")})
				return NewHierarchicalActivation(mustActivation(t, map[string]any{"a": 1}), child)
			},
			wantFuncs: map[string]string{"f": "child"},
			wantVars:  map[string]any{"a": 1, "b": 2},
		},
		{
			name: "functions beneath a hierarchical activation",
			vars: func(t *testing.T) Activation {
				fnVars := mustFunctionVars(t, map[string]any{"a": 1}, map[string]functions.LateBoundOp{"g": funcOf("inner")})
				return NewHierarchicalActivation(
					mustActivation(t, map[string]any{"b": 2}),
					mustPartialVars(t, fnVars, NewAttributePattern("c")))
			},
			wantFuncs:   map[string]string{"g": "inner", "f": ""},
			wantVars:    map[string]any{"a": 1, "b": 2},
			wantPartial: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vars := tc.vars(t)
			for name, want := range tc.wantFuncs {
				fn, found := resolveFunction(t, vars, name)
				if want == "" {
					if found {
						t.Errorf("ResolveFunction(%q) found a binding, wanted none", name)
					}
					continue
				}
				if !found {
					t.Fatalf("ResolveFunction(%q) not found, wanted %q", name, want)
				}
				if got := fn("overload_id"); got.Equal(types.String(want)) != types.True {
					t.Errorf("ResolveFunction(%q) resolved %v, wanted %q", name, got, want)
				}
			}
			for name, want := range tc.wantVars {
				got, found := vars.ResolveName(name)
				if !found {
					t.Errorf("ResolveName(%q) not found, wanted %v", name, want)
					continue
				}
				if got != want {
					t.Errorf("ResolveName(%q) got %v, wanted %v", name, got, want)
				}
			}
			if _, isPartial := AsPartialActivation(vars); isPartial != tc.wantPartial {
				t.Errorf("AsPartialActivation() got %t, wanted %t", isPartial, tc.wantPartial)
			}
		})
	}
}

// TestFunctionActivationNestingRejected covers hierarchies which supply late-bound functions from
// more than one activation. Such a hierarchy has no obvious resolution order, so it is reported
// rather than resolved by proximity.
func TestFunctionActivationNestingRejected(t *testing.T) {
	fnA := map[string]functions.LateBoundOp{"f": funcOf("a")}
	fnB := map[string]functions.LateBoundOp{"f": funcOf("b")}

	t.Run("function activation over a function activation", func(t *testing.T) {
		inner := mustFunctionVars(t, map[string]any{"a": 1}, fnA)
		if _, err := NewFunctionActivation(inner, fnB); err == nil {
			t.Error("NewFunctionActivation() over an existing binding wanted error, got nil")
		}
	})
	t.Run("function activation over a partial wrapping one", func(t *testing.T) {
		inner := mustFunctionVars(t, map[string]any{"a": 1}, fnA)
		partial := mustPartialVars(t, inner, NewAttributePattern("b"))
		if _, err := NewFunctionActivation(partial, fnB); err == nil {
			t.Error("NewFunctionActivation() over a wrapped binding wanted error, got nil")
		}
	})
	t.Run("function activation over a hierarchy containing one", func(t *testing.T) {
		inner := NewHierarchicalActivation(
			mustActivation(t, map[string]any{"a": 1}),
			mustFunctionVars(t, map[string]any{"b": 2}, fnA))
		if _, err := NewFunctionActivation(inner, fnB); err == nil {
			t.Error("NewFunctionActivation() over a hierarchy wanted error, got nil")
		}
	})
	t.Run("both sides of a hierarchical activation", func(t *testing.T) {
		vars := NewHierarchicalActivation(
			mustFunctionVars(t, map[string]any{"a": 1}, fnA),
			mustFunctionVars(t, map[string]any{"b": 2}, fnB))
		if _, err := FindFunctionActivation(vars); err == nil {
			t.Error("FindFunctionActivation() over two bindings wanted error, got nil")
		}
	})
}

func TestNewFunctionActivationErrors(t *testing.T) {
	if _, err := NewFunctionActivation(nil, map[string]functions.LateBoundOp{"f": funcOf("f")}); err == nil {
		t.Error("NewFunctionActivation(nil) wanted error, got nil")
	}
	if _, err := NewFunctionActivation(map[string]any{}, map[string]functions.LateBoundOp{"f": nil}); err == nil {
		t.Error("NewFunctionActivation() with a nil binding wanted error, got nil")
	}
}

func TestFunctionActivationBindingsAreCopied(t *testing.T) {
	funcs := map[string]functions.LateBoundOp{"f": funcOf("original")}
	vars, err := NewFunctionActivation(map[string]any{}, funcs)
	if err != nil {
		t.Fatalf("NewFunctionActivation() failed: %v", err)
	}
	// Mutating the input after construction must not affect the activation.
	funcs["f"] = funcOf("mutated")
	funcs["g"] = funcOf("added")

	fn, found := vars.ResolveFunction("f")
	if !found {
		t.Fatal("ResolveFunction('f') not found")
	}
	if got := fn("overload_id"); got.Equal(types.String("original")) != types.True {
		t.Errorf("ResolveFunction('f') resolved %v, wanted 'original'", got)
	}
	if _, found := vars.ResolveFunction("g"); found {
		t.Error("ResolveFunction('g') found a binding added after construction")
	}
}

func mustActivation(t *testing.T, vars map[string]any) Activation {
	t.Helper()
	act, err := NewActivation(vars)
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}
	return act
}

func mustFunctionVars(t *testing.T, vars any, funcs map[string]functions.LateBoundOp) FunctionActivation {
	t.Helper()
	act, err := NewFunctionActivation(vars, funcs)
	if err != nil {
		t.Fatalf("NewFunctionActivation() failed: %v", err)
	}
	return act
}

func mustPartialVars(t *testing.T, vars any, patterns ...*AttributePattern) PartialActivation {
	t.Helper()
	act, err := NewPartialActivation(vars, patterns...)
	if err != nil {
		t.Fatalf("NewPartialActivation() failed: %v", err)
	}
	return act
}
