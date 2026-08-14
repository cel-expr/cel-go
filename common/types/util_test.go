// Copyright 2022 Google LLC
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
	"math"
	"testing"
)

func TestSafeUint32Helpers(t *testing.T) {
	// safeAddUint32
	if got := safeAddUint32(10, 20); got != 30 {
		t.Errorf("safeAddUint32(10, 20) got %d, want 30", got)
	}
	if got := safeAddUint32(math.MaxUint32-5, 10); got != math.MaxUint32 {
		t.Errorf("safeAddUint32(overflow) got %d, want MaxUint32", got)
	}

	// safeUint32FromInt
	if got := safeUint32FromInt(42); got != 42 {
		t.Errorf("safeUint32FromInt(42) got %d, want 42", got)
	}
	if got := safeUint32FromInt(-1); got != math.MaxUint32 {
		t.Errorf("safeUint32FromInt(-1) got %d, want MaxUint32", got)
	}
	if got := safeUint32FromInt(int(uint64(math.MaxUint32) + 100)); got != math.MaxUint32 {
		t.Errorf("safeUint32FromInt(overflow) got %d, want MaxUint32", got)
	}

	// safeUint32FromBoxedInt
	if got := safeUint32FromBoxedInt(Int(42)); got != 42 {
		t.Errorf("safeUint32FromBoxedInt(42) got %d, want 42", got)
	}
	if got := safeUint32FromBoxedInt(Int(-1)); got != math.MaxUint32 {
		t.Errorf("safeUint32FromBoxedInt(-1) got %d, want MaxUint32", got)
	}
	if got := safeUint32FromBoxedInt(Int(int64(math.MaxUint32) + 100)); got != math.MaxUint32 {
		t.Errorf("safeUint32FromBoxedInt(overflow) got %d, want MaxUint32", got)
	}
}

func BenchmarkIsUnknownOrError(b *testing.B) {
	err := NewErr("test")
	unk := &Unknown{}
	for i := 0; i < b.N; i++ {
		if !(IsUnknownOrError(unk) && IsUnknownOrError(err) && !IsUnknownOrError(IntOne)) {
			b.Fatal("IsUnknownOrError() provided an incorrect result.")
		}
	}
}
