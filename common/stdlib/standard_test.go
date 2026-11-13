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

package stdlib

import (
	"testing"

	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/stdlib/bindings"
)

func TestBindingsMatchFunctions(t *testing.T) {
	want := map[string]*functions.Overload{}
	for _, fn := range Functions() {
		bs, err := fn.Bindings()
		if err != nil {
			t.Fatalf("%s.Bindings() failed: %v", fn.Name(), err)
		}
		for _, b := range bs {
			want[b.Operator] = b
		}
	}

	got := map[string]*functions.Overload{}
	for _, b := range bindings.Overloads() {
		if _, dup := got[b.Operator]; dup {
			t.Errorf("duplicate binding for operator %q", b.Operator)
		}
		got[b.Operator] = b
	}

	for op := range want {
		if _, ok := got[op]; !ok {
			t.Errorf("missing runtime binding for %q", op)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("bindings count = %d, stdlib.Functions() bindings = %d", len(got), len(want))
	}
	for op, wb := range want {
		gb, ok := got[op]
		if !ok {
			t.Errorf("missing runtime binding for %q", op)
			continue
		}
		if gb.OperandTrait != wb.OperandTrait {
			t.Errorf("%q OperandTrait = %d, want %d", op, gb.OperandTrait, wb.OperandTrait)
		}
		if gb.NonStrict != wb.NonStrict {
			t.Errorf("%q NonStrict = %t, want %t", op, gb.NonStrict, wb.NonStrict)
		}
		if gb.Unary == nil && gb.Binary == nil && gb.Function == nil {
			t.Errorf("%q has no binding", op)
		}
	}
}
