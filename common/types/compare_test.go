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

import "testing"

// TestCompareDoubleIntPrecision verifies that compareDoubleInt and
// compareDoubleUint correctly distinguish integer values that are adjacent
// to float64 representable boundaries (> 2^53), where naive float64 casting
// loses precision and can incorrectly equate distinct integers.
func TestCompareDoubleIntPrecision(t *testing.T) {
	tests := []struct {
		d    Double
		i    Int
		want Int
		desc string
	}{
		// Values within safe float64 range — must still work correctly.
		{Double(1.0), Int(1), IntZero, "1.0 == 1"},
		{Double(1.5), Int(1), IntOne, "1.5 > 1"},
		{Double(0.5), Int(1), IntNegOne, "0.5 < 1"},
		{Double(9007199254740992), Int(9007199254740992), IntZero, "2^53 == 2^53"},

		// Precision-loss zone (> 2^53): distinct integers must NOT compare equal.
		// Bug: Double(i) cast loses low bits, collapsing adjacent ints to same float64.
		{Double(9007199254740992), Int(9007199254740993), IntNegOne, "2^53 < 2^53+1"},
		{Double(9007199254740992), Int(9007199254740994), IntNegOne, "2^53 < 2^53+2"},
		{Double(1e17), Int(100000000000000001), IntNegOne, "1e17 < 1e17+1"},
		{Double(1e18), Int(1000000000000000001), IntNegOne, "1e18 < 1e18+1"},

		// Symmetric: int smaller than double.
		{Double(9007199254740993), Int(9007199254740992), IntOne, "2^53+1 > 2^53 (as Double)"},
	}
	for _, tc := range tests {
		got := compareDoubleInt(tc.d, tc.i)
		if got != tc.want {
			t.Errorf("compareDoubleInt(%v, %v): got %v, want %v — %s",
				tc.d, tc.i, got, tc.want, tc.desc)
		}
	}
}

func TestCompareDoubleUintPrecision(t *testing.T) {
	tests := []struct {
		d    Double
		u    Uint
		want Int
		desc string
	}{
		{Double(0), Uint(0), IntZero, "0.0 == 0"},
		{Double(1.0), Uint(1), IntZero, "1.0 == 1"},
		{Double(9007199254740992), Uint(9007199254740992), IntZero, "2^53 == 2^53"},
		{Double(9007199254740992), Uint(9007199254740993), IntNegOne, "2^53 < 2^53+1 (uint)"},
		{Double(1e17), Uint(100000000000000001), IntNegOne, "1e17 < 1e17+1 (uint)"},
	}
	for _, tc := range tests {
		got := compareDoubleUint(tc.d, tc.u)
		if got != tc.want {
			t.Errorf("compareDoubleUint(%v, %v): got %v, want %v — %s",
				tc.d, tc.u, got, tc.want, tc.desc)
		}
	}
}
