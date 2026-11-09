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

package types

import (
	"testing"

	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

func TestCompareDoesNotAllocate(t *testing.T) {
	tests := []struct {
		name string
		a    ref.Val
		b    ref.Val
		want Int
	}{
		{"double less", Double(1.5), Double(2.5), IntNegOne},
		{"double greater", Double(2.5), Double(1.5), IntOne},
		{"double equal", Double(2.5), Double(2.5), IntZero},
		{"double int less", Double(1.5), Int(2), IntNegOne},
		{"double uint less", Double(1.5), Uint(2), IntNegOne},
		{"int double less", Int(1), Double(1.5), IntNegOne},
		{"int uint less", Int(-1), Uint(0), IntNegOne},
		{"uint double less", Uint(1), Double(1.5), IntNegOne},
		{"uint int greater", Uint(1), Int(-1), IntOne},
		{"uint int less", Uint(1), Int(2), IntNegOne},
		{"string less", String("a"), String("b"), IntNegOne},
		{"string greater", String("b"), String("a"), IntOne},
		{"bytes less", Bytes("a"), Bytes("b"), IntNegOne},
		{"bytes equal", Bytes("a"), Bytes("a"), IntZero},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmp := tc.a.(traits.Comparer)
			if got := cmp.Compare(tc.b); got != tc.want {
				t.Fatalf("%v.Compare(%v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if allocs := testing.AllocsPerRun(100, func() { cmp.Compare(tc.b) }); allocs != 0 {
				t.Errorf("%v.Compare(%v) allocated %v times, want 0", tc.a, tc.b, allocs)
			}
		})
	}
}

func BenchmarkDoubleCompareLess(b *testing.B) {
	lhs, rhs := Double(1.5), Double(2.5)
	for b.Loop() {
		lhs.Compare(rhs)
	}
}
