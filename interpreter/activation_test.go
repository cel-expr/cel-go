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

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
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
