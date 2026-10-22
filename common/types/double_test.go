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

package types

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"

	anypb "google.golang.org/protobuf/types/known/anypb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

func TestDoubleAdd(t *testing.T) {
	if !Double(4).Add(Double(-3.5)).Equal(Double(0.5)).(Bool) {
		t.Error("Adding two doubles did not match expected value.")
	}
	if !IsError(Double(-1).Add(String("-1"))) {
		t.Error("Adding non-double to double was not an error.")
	}
}

func TestDoubleCompare(t *testing.T) {
	tests := []struct {
		a   ref.Val
		b   ref.Val
		out ref.Val
	}{
		{
			a:   Double(42),
			b:   Double(42),
			out: IntZero,
		},
		{
			a:   Double(42),
			b:   Uint(42),
			out: IntZero,
		},
		{
			a:   Double(42),
			b:   Int(42),
			out: IntZero,
		},
		{
			a:   Double(-1300),
			b:   Double(204),
			out: IntNegOne,
		},
		{
			a:   Double(-1300),
			b:   Uint(204),
			out: IntNegOne,
		},
		{
			a:   Double(203.9),
			b:   Int(204),
			out: IntNegOne,
		},
		{
			a:   Double(1300),
			b:   Uint(math.MaxInt64) + 1,
			out: IntNegOne,
		},
		{
			a:   Double(204),
			b:   Uint(205),
			out: IntNegOne,
		},
		{
			a:   Double(204),
			b:   Double(math.MaxInt64) + 1025.0,
			out: IntNegOne,
		},
		{
			a:   Double(204),
			b:   Double(math.NaN()),
			out: NewErr("NaN values cannot be ordered"),
		},
		{
			a:   Double(math.NaN()),
			b:   Double(204),
			out: NewErr("NaN values cannot be ordered"),
		},
		{
			a:   Double(204),
			b:   Double(-1300),
			out: IntOne,
		},
		{
			a:   Double(204),
			b:   Uint(10),
			out: IntOne,
		},
		{
			a:   Double(204.1),
			b:   Int(204),
			out: IntOne,
		},
		{
			// 2^53+1 has no double representation, so it must not compare equal to 2^53.
			a:   Double(1 << 53),
			b:   Int(1<<53) + 1,
			out: IntNegOne,
		},
		{
			a:   Double(1 << 53),
			b:   Uint(1<<53) + 1,
			out: IntNegOne,
		},
		{
			// MaxInt64 rounds up to 2^63 as a double, but remains below it as an int.
			a:   Double(1 << 63),
			b:   Int(math.MaxInt64),
			out: IntOne,
		},
		{
			a:   Double(1),
			b:   String("1"),
			out: NoSuchOverloadErr(),
		},
	}
	for _, tc := range tests {
		comparer := tc.a.(traits.Comparer)
		got := comparer.Compare(tc.b)
		if !reflect.DeepEqual(got, tc.out) {
			t.Errorf("%v.Compare(%v) got %v, wanted %v", tc.a, tc.b, got, tc.out)
		}
	}
}

func TestDoubleConvertToNative_Any(t *testing.T) {
	val, err := Double(math.MaxFloat64).ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	want, err := anypb.New(wrapperspb.Double(1.7976931348623157e+308))
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', wanted %v", val, want)
	}
}

func TestDoubleConvertToNative_Error(t *testing.T) {
	val, err := Double(-10000).ConvertToNative(reflect.TypeOf(""))
	if err == nil {
		t.Errorf("Got '%v', expected error", val)
	}
}

func TestDoubleConvertToNative_Float32(t *testing.T) {
	val, err := Double(3.1415).ConvertToNative(reflect.TypeOf(float32(0)))
	if err != nil {
		t.Error(err)
	} else if val.(float32) != 3.1415 {
		t.Errorf("Got '%v', wanted 3.1415", val)
	}
}

func TestDoubleConvertToNative_Float64(t *testing.T) {
	val, err := Double(30000000.1).ConvertToNative(reflect.TypeOf(float64(0)))
	if err != nil {
		t.Error(err)
	} else if val.(float64) != 30000000.1 {
		t.Errorf("Got '%v', wanted 330000000.1", val)
	}
}

func TestDoubleConvertToNative_Json(t *testing.T) {
	val, err := Double(-1.4).ConvertToNative(JSONValueType)
	pbVal := structpb.NewNumberValue(-1.4)
	if err != nil {
		t.Error(err)
	} else if !proto.Equal(val.(proto.Message), pbVal) {
		t.Errorf("Got '%v', expected -1.4", val)
	}

	val, err = Double(math.NaN()).ConvertToNative(JSONValueType)
	if err != nil {
		t.Error(err)
	} else {
		v := val.(*structpb.Value)
		if !math.IsNaN(v.GetNumberValue()) {
			t.Errorf("Got '%v', expected NaN", val)
		}
	}

	val, err = Double(math.Inf(-1)).ConvertToNative(JSONValueType)
	pbVal = structpb.NewNumberValue(math.Inf(-1))
	if err != nil {
		t.Error(err)
	} else if !proto.Equal(val.(proto.Message), pbVal) {
		t.Errorf("Got '%v', expected -Infinity", val)
	}
	val, err = Double(math.Inf(0)).ConvertToNative(JSONValueType)
	pbVal = structpb.NewNumberValue(math.Inf(0))
	if err != nil {
		t.Error(err)
	} else if !proto.Equal(val.(proto.Message), pbVal) {
		t.Errorf("Got '%v', expected Infinity", val)
	}
}

func TestDoubleConvertToNative_Ptr_Float32(t *testing.T) {
	ptrType := float32(0)
	val, err := Double(3.1415).ConvertToNative(reflect.TypeOf(&ptrType))
	if err != nil {
		t.Error(err)
	} else if *val.(*float32) != 3.1415 {
		t.Errorf("Got '%v', wanted 3.1415", val)
	}
}

func TestDoubleConvertToNative_Ptr_Float64(t *testing.T) {
	ptrType := float64(0)
	val, err := Double(30000000.1).ConvertToNative(reflect.TypeOf(&ptrType))
	if err != nil {
		t.Error(err)
	} else if *val.(*float64) != 30000000.1 {
		t.Errorf("Got '%v', wanted 330000000.1", val)
	}
}

func TestDoubleConvertToNative_Wrapper(t *testing.T) {
	val, err := Double(3.1415).ConvertToNative(floatWrapperType)
	if err != nil {
		t.Error(err)
	}
	want := wrapperspb.Float(3.1415)
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', wanted %v", val, want)
	}

	val, err = Double(math.MaxFloat64).ConvertToNative(doubleWrapperType)
	if err != nil {
		t.Error(err)
	}
	want2 := wrapperspb.Double(1.7976931348623157e+308)
	if !proto.Equal(val.(proto.Message), want2) {
		t.Errorf("Got '%v', wanted %v", val, want2)
	}
}

func TestDoubleConvertToType(t *testing.T) {
	tests := []struct {
		name   string
		in     float64
		toType ref.Type
		out    any
	}{
		{
			name:   "DoubleToDouble",
			in:     float64(-4.2),
			toType: DoubleType,
			out:    float64(-4.2),
		},
		{
			name:   "DoubleToType",
			in:     float64(-4.2),
			toType: TypeType,
			out:    DoubleType.TypeName(),
		},
		{
			name:   "DoubleToInt",
			in:     float64(4.2),
			toType: IntType,
			out:    int64(4),
		},
		{
			name:   "DoubleToIntNaN",
			in:     math.NaN(),
			toType: IntType,
			out:    errIntOverflow,
		},
		{
			name:   "DoubleToIntPosInf",
			in:     math.Inf(1),
			toType: IntType,
			out:    errIntOverflow,
		},
		{
			name:   "DoubleToIntPosOverflow",
			in:     float64(math.MaxInt64),
			toType: IntType,
			out:    errIntOverflow,
		},
		{
			name:   "DoubleToIntNegOverflow",
			in:     float64(math.MinInt64),
			toType: IntType,
			out:    errIntOverflow,
		},
		{
			name:   "DoubleToUint",
			in:     float64(4.7),
			toType: UintType,
			out:    uint64(4),
		},
		{
			name:   "DoubleToUintNaN",
			in:     math.NaN(),
			toType: UintType,
			out:    errUintOverflow,
		},
		{
			name:   "DoubleToUintPosInf",
			in:     math.Inf(1),
			toType: UintType,
			out:    errUintOverflow,
		},
		{
			name:   "DoubleToUintPosOverflow",
			in:     float64(math.MaxUint64),
			toType: UintType,
			out:    errUintOverflow,
		},
		{
			name:   "DoubleToUintNegOverflow",
			in:     float64(-0.1),
			toType: UintType,
			out:    errUintOverflow,
		},
		{
			name:   "DoubleToString",
			in:     float64(4.5),
			toType: StringType,
			out:    "4.5",
		},
		{
			name:   "DoubleToUnsupportedType",
			in:     float64(4),
			toType: MapType,
			out:    errors.New("type conversion error"),
		},
	}
	for _, tst := range tests {
		got := Double(tst.in).ConvertToType(tst.toType).Value()
		var eq bool
		switch gotVal := got.(type) {
		case error:
			eq = strings.Contains(gotVal.Error(), tst.out.(error).Error())
		default:
			eq = reflect.DeepEqual(gotVal, tst.out)
		}
		if !eq {
			t.Errorf("Double(%v).ConvertToType(%v) failed, got: %v, wanted: %v",
				tst.in, tst.toType, got, tst.out)
		}
	}
}

func TestDoubleDivide(t *testing.T) {
	if !Double(3).Divide(Double(1.5)).Equal(Double(2)).(Bool) {
		t.Error("Dividing two doubles did not match expectations.")
	}
	var z float64 // Avoid 0.0 since const div by zero is an error.
	if !Double(1.1).Divide(Double(0)).Equal(Double(1.1 / z)).(Bool) {
		t.Error("Division by zero did not match infinity.")
	}
	if !IsError(Double(1.1).Divide(IntNegOne)) {
		t.Error("Division permitted without express type-conversion.")
	}
}

func TestDoubleEqual(t *testing.T) {
	tests := []struct {
		a   ref.Val
		b   ref.Val
		out ref.Val
	}{
		{
			a:   Double(-10),
			b:   Double(-10),
			out: True,
		},
		{
			a:   Double(-10),
			b:   Double(10),
			out: False,
		},
		{
			a:   Double(10),
			b:   Uint(10),
			out: True,
		},
		{
			a:   Double(9),
			b:   Uint(10),
			out: False,
		},
		{
			a:   Double(10),
			b:   Int(10),
			out: True,
		},
		{
			a:   Double(10),
			b:   Int(-15),
			out: False,
		},
		{
			a:   Double(math.NaN()),
			b:   Int(10),
			out: False,
		},
		{
			a:   Double(1 << 53),
			b:   Int(1<<53) + 1,
			out: False,
		},
		{
			a:   Double(1 << 53),
			b:   Uint(1<<53) + 1,
			out: False,
		},
		{
			a:   Double(1 << 63),
			b:   Int(math.MaxInt64),
			out: False,
		},
		{
			a:   Double(math.MaxUint64),
			b:   Uint(math.MaxUint64),
			out: False,
		},
	}
	for _, tc := range tests {
		got := tc.a.Equal(tc.b)
		if !reflect.DeepEqual(got, tc.out) {
			t.Errorf("%v.Equal(%v) got %v, wanted %v", tc.a, tc.b, got, tc.out)
		}
	}
}

func TestDoubleIsZeroValue(t *testing.T) {
	if Double(math.Inf(1)).IsZeroValue() {
		t.Error("Double(+infinity).IsZeroValue() returned true, wanted false.")
	}
	if Double(math.Inf(-1)).IsZeroValue() {
		t.Error("Double(-infinity).IsZeroValue() returned true, wanted false.")
	}
	if Double(math.NaN()).IsZeroValue() {
		t.Error("Double(NaN).IsZeroValue() returned true, wanted false.")
	}
	if !Double(0.0).IsZeroValue() {
		t.Error("Double(0.0).IsZeroValue() returned false, wanted true")
	}
}

func TestDoubleMultiply(t *testing.T) {
	if !Double(1.1).Multiply(Double(-1.2)).Equal(Double(-1.32)).(Bool) {
		t.Error("Multiplying two doubles did not match expectations.")
	}
	if !IsError(Double(1.1).Multiply(IntNegOne)) {
		t.Error("Multiplication permitted without express type-conversion.")
	}
}

func TestDoubleNegate(t *testing.T) {
	if !Double(1.1).Negate().Equal(Double(-1.1)).(Bool) {
		t.Error("Negating double value did not succeed")
	}
}

func TestDoubleSubtract(t *testing.T) {
	if !Double(4).Subtract(Double(-3.5)).Equal(Double(7.5)).(Bool) {
		t.Error("Subtracting two doubles did not match expected value.")
	}
	if !IsError(Double(1.1).Subtract(IntNegOne)) {
		t.Error("Subtraction permitted without express type-conversion.")
	}
}
