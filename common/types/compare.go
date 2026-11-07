// Copyright 2021 Google LLC
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

	"cel.dev/cel-go/common/types/ref"
)

// compareDoubleInt orders a double against an int by their mathematical values.
//
// The int is split from the double rather than converted to one: ints above 2^53 have no exact
// double representation, so converting rounds them to a neighbouring value. Callers screen NaN
// before dispatching here.
func compareDoubleInt(d Double, i Int) Int {
	f := float64(d)
	if f < math.MinInt64 {
		return IntNegOne
	}
	// MaxInt64 rounds up to 2^63 as a double, which is the first value above the int64 range.
	if f >= math.MaxInt64 {
		return IntOne
	}
	// The integral part is now within the int64 range, so it converts exactly.
	whole := math.Trunc(f)
	if Int(whole) < i {
		return IntNegOne
	}
	if Int(whole) > i {
		return IntOne
	}
	// Equal whole parts, so the remaining fraction decides the order.
	return compareDouble(Double(f-whole), 0)
}

func compareIntDouble(i Int, d Double) Int {
	return -compareDoubleInt(d, i)
}

// compareDoubleUint orders a double against a uint by their mathematical values, splitting the
// double instead of converting the uint for the reason given on compareDoubleInt.
func compareDoubleUint(d Double, u Uint) Int {
	f := float64(d)
	if f < 0 {
		return IntNegOne
	}
	if f >= doubleTwoTo64 {
		return IntOne
	}
	whole := math.Trunc(f)
	if Uint(whole) < u {
		return IntNegOne
	}
	if Uint(whole) > u {
		return IntOne
	}
	return compareDouble(Double(f-whole), 0)
}

func compareUintDouble(u Uint, d Double) Int {
	return -compareDoubleUint(d, u)
}

func compareIntUint(i Int, u Uint) Int {
	if i < 0 || u > math.MaxInt64 {
		return IntNegOne
	}
	cmp := i - Int(u)
	if cmp < 0 {
		return IntNegOne
	}
	if cmp > 0 {
		return IntOne
	}
	return IntZero
}

func compareUintInt(u Uint, i Int) Int {
	return -compareIntUint(i, u)
}

func compareDouble(a, b Double) Int {
	if a < b {
		return IntNegOne
	}
	if a > b {
		return IntOne
	}
	return IntZero
}

func compareInt(a, b Int) ref.Val {
	if a < b {
		return IntNegOne
	}
	if a > b {
		return IntOne
	}
	return IntZero
}

func compareUint(a, b Uint) ref.Val {
	if a < b {
		return IntNegOne
	}
	if a > b {
		return IntOne
	}
	return IntZero
}
