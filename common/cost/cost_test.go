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

package cost

import (
	"math"
	"testing"
)

func TestSafeAdd(t *testing.T) {
	tests := []struct {
		name string
		x, y uint64
		rest []uint64
		want uint64
	}{
		{name: "zero", x: 0, y: 0, want: 0},
		{name: "simple", x: 2, y: 3, want: 5},
		{name: "variadic", x: 1, y: 2, rest: []uint64{3, 4}, want: 10},
		{name: "max plus zero", x: math.MaxUint64, y: 0, want: math.MaxUint64},
		{name: "overflow", x: math.MaxUint64, y: 1, want: math.MaxUint64},
		{name: "overflow near max", x: math.MaxUint64 - 5, y: 10, want: math.MaxUint64},
		{name: "overflow in rest", x: 1, y: 2, rest: []uint64{math.MaxUint64}, want: math.MaxUint64},
		{name: "saturated stays saturated", x: math.MaxUint64, y: math.MaxUint64,
			rest: []uint64{math.MaxUint64}, want: math.MaxUint64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeAdd(tc.x, tc.y, tc.rest...); got != tc.want {
				t.Errorf("SafeAdd(%d, %d, %v) got %d, want %d", tc.x, tc.y, tc.rest, got, tc.want)
			}
		})
	}
}

func TestSafeMultiply(t *testing.T) {
	tests := []struct {
		name string
		x, y uint64
		want uint64
	}{
		{name: "zero", x: 0, y: 0, want: 0},
		{name: "max by zero", x: math.MaxUint64, y: 0, want: 0},
		{name: "simple", x: 3, y: 4, want: 12},
		{name: "max by one", x: math.MaxUint64, y: 1, want: math.MaxUint64},
		{name: "overflow", x: math.MaxUint64, y: 2, want: math.MaxUint64},
		{name: "overflow squared", x: math.MaxUint32, y: math.MaxUint32 * 2, want: math.MaxUint64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeMultiply(tc.x, tc.y); got != tc.want {
				t.Errorf("SafeMultiply(%d, %d) got %d, want %d", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

func TestSafeMultiplyByFactor(t *testing.T) {
	tests := []struct {
		name   string
		x      uint64
		factor float64
		want   uint64
	}{
		{name: "zero value", x: 0, factor: 0.1, want: 0},
		{name: "zero factor", x: 100, factor: 0, want: 0},
		{name: "rounds up", x: 15, factor: 0.1, want: 2},
		{name: "exact", x: 10, factor: 0.1, want: 1},
		{name: "whole factor", x: 10, factor: 3, want: 30},
		{name: "max saturates", x: math.MaxUint64, factor: 2, want: math.MaxUint64},
		{name: "max scaled down", x: math.MaxUint64, factor: 0.1, want: 1844674407370955264},
		{name: "negative factor", x: 10, factor: -1, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeMultiplyByFactor(tc.x, tc.factor); got != tc.want {
				t.Errorf("SafeMultiplyByFactor(%d, %f) got %d, want %d", tc.x, tc.factor, got, tc.want)
			}
		})
	}
}

func TestSafeCeil(t *testing.T) {
	tests := []struct {
		name string
		x    float64
		want uint64
	}{
		{name: "zero", x: 0, want: 0},
		{name: "negative", x: -1.5, want: 0},
		{name: "nan", x: math.NaN(), want: 0},
		{name: "fraction", x: 0.1, want: 1},
		{name: "rounds up", x: 2.5, want: 3},
		{name: "whole", x: 3.0, want: 3},
		{name: "infinity", x: math.Inf(1), want: math.MaxUint64},
		{name: "out of range", x: math.Ldexp(1.0, 64), want: math.MaxUint64},
		{name: "largest in range", x: math.Ldexp(1.0, 63), want: 1 << 63},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeCeil(tc.x); got != tc.want {
				t.Errorf("SafeCeil(%f) got %d, want %d", tc.x, got, tc.want)
			}
		})
	}
}
