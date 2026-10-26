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
	"math"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types/ref"

	anypb "google.golang.org/protobuf/types/known/anypb"
	dpb "google.golang.org/protobuf/types/known/durationpb"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

func TestDurationOperators(t *testing.T) {
	d := duration(7506, 567)
	dSecond := duration(1, 0)
	dNano := duration(0, 1)
	dMax := duration(0, math.MaxInt64)
	dMin := duration(0, math.MinInt64)

	tests := []struct {
		name string
		op   func() ref.Val
		out  any
	}{
		// Addition tests.
		{
			name: "DurationAddSelf",
			op: func() ref.Val {
				return durationOf(d).Add(durationOf(d))
			},
			out: d + d,
		},
		{
			name: "DurationMaxAddOneNanoOverflow",
			op: func() ref.Val {
				return durationOf(dMax).Add(durationOf(dNano))
			},
			out: errIntOverflow,
		},
		{
			name: "DurationMaxAddOneSecondOverflow",
			op: func() ref.Val {
				return durationOf(dMax).Add(durationOf(dSecond))
			},
			out: errIntOverflow,
		},
		{
			name: "DurationMinAddMinusOneOverflow",
			op: func() ref.Val {
				return durationOf(dMin).Add(durationOf(-dSecond))
			},
			out: errIntOverflow,
		},

		// Subtraction tests.
		{
			name: "DurationSubSelf",
			op: func() ref.Val {
				return durationOf(d).Subtract(durationOf(d))
			},
			out: duration(0, 0),
		},
		{
			name: "DurationMaxSubMinusOneOverflow",
			op: func() ref.Val {
				return durationOf(dMax).Subtract(durationOf(-dNano))
			},
			out: errIntOverflow,
		},
		{
			name: "DurationMinSubOneOverflow",
			op: func() ref.Val {
				return durationOf(dMin).Subtract(durationOf(dNano))
			},
			out: errIntOverflow,
		},
	}
	for _, tst := range tests {
		got := tst.op()
		switch v := got.Value().(type) {
		case error:
			if want, ok := tst.out.(error); !ok || v.Error() != want.Error() {
				t.Errorf("%s: got %v, wanted %v", tst.name, v, tst.out)
			}
		default:
			if !reflect.DeepEqual(v, tst.out) {
				t.Errorf("%s: got %v, wanted %v", tst.name, v, tst.out)
			}
		}
	}
}

func TestDurationCompare(t *testing.T) {
	d := Duration{(&dpb.Duration{Seconds: 7506}).AsDuration()}
	lt := Duration{(&dpb.Duration{Seconds: -10}).AsDuration()}
	if d.Compare(lt).(Int) != IntOne {
		t.Error("Larger duration was not considered greater than smaller one.")
	}
	if lt.Compare(d).(Int) != IntNegOne {
		t.Error("Smaller duration was not less than larger one.")
	}
	if d.Compare(d).(Int) != IntZero {
		t.Error("Durations were not considered equal.")
	}
	if !IsError(d.Compare(False)) {
		t.Error("Got comparison result, expected error.")
	}
}

func TestDurationConvertToNative(t *testing.T) {
	dur := Duration{Duration: duration(7506, 1000)}
	val, err := dur.ConvertToNative(reflect.TypeFor[*dpb.Duration]())
	if err != nil ||
		!proto.Equal(val.(proto.Message), &dpb.Duration{Seconds: 7506, Nanos: 1000}) {
		t.Errorf("Got '%v', expected backing proto message value", err)
	}
	val, err = dur.ConvertToNative(reflect.TypeFor[Duration]())
	if err != nil {
		t.Fatalf("ConvertToNative() failed: %v", err)
	}
	if !reflect.DeepEqual(val, dur) {
		t.Errorf("got value %v, wanted %v", val, dur)
	}
	val, err = dur.ConvertToNative(reflect.TypeFor[time.Duration]())
	if err != nil {
		t.Fatalf("ConvertToNative() failed: %v", err)
	}
	if !reflect.DeepEqual(val, dur.Duration) {
		t.Errorf("got value %v, wanted %v", val, dur.Duration)
	}
}

func TestDurationConvertToNative_Any(t *testing.T) {
	d := Duration{Duration: duration(7506, 1000)}
	val, err := d.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	want, err := anypb.New(dpb.New(d.Duration))
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', wanted %v", val, want)
	}
}

func TestDurationConvertToNative_Error(t *testing.T) {
	val, err := Duration{Duration: duration(7506, 1000)}.ConvertToNative(JSONValueType)
	if err != nil {
		t.Errorf("Got error: '%v', expected value", err)
	}
	json := val.(*structpb.Value)
	want := structpb.NewStringValue("7506.000001s")
	if !proto.Equal(json, want) {
		t.Errorf("Got %v, wanted %v", json, want)
	}
}

func TestDurationConvertToNative_Json(t *testing.T) {
	val, err := Duration{Duration: duration(7506, 1000)}.ConvertToNative(JSONValueType)
	if err != nil {
		t.Error(err)
	}
	want := structpb.NewStringValue("7506.000001s")
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', wanted %v", val, want)
	}
}

func TestDurationConvertToType_Identity(t *testing.T) {
	d := Duration{Duration: duration(7506, 1000)}
	str := d.ConvertToType(StringType).(String)
	if str != "7506.000001s" {
		t.Errorf("Got '%v', wanted 7506.000001s", str)
	}
	i := d.ConvertToType(IntType).(Int)
	if i != Int(7506000001000) {
		t.Errorf("Got '%v', wanted 7506000001000", i)
	}
	if !d.ConvertToType(DurationType).Equal(d).(Bool) {
		t.Errorf("Got '%v', wanted identity", d.ConvertToType(DurationType))
	}
	if d.ConvertToType(TypeType) != DurationType {
		t.Errorf("Got '%v', expected duration type", d.ConvertToType(TypeType))
	}
	if !IsError(d.ConvertToType(UintType)) {
		t.Errorf("Got value, expected error.")
	}
}

func TestDurationNegate(t *testing.T) {
	neg := Duration{Duration: duration(1234, 1)}.Negate()
	want := duration(-1234, -1)
	if neg.Value().(time.Duration) != want {
		t.Errorf("Got %v, expected %v", neg, want)
	}
	if v := time.Duration(math.MinInt64); !IsError(durationOf(v).Negate()) {
		t.Errorf("Expected negating %d to result in overflow.", v)
	}
	if v := time.Duration(math.MaxInt64); !durationOf(v).Negate().Equal(durationOf(time.Duration(math.MinInt64 + 1))).(Bool) {
		t.Errorf("Expected negating %d to yield %d", v, time.Duration(math.MinInt64+1))
	}
}

func TestDurationGetHours(t *testing.T) {
	d := Duration{Duration: duration(7506, 0)}
	hr := d.Receive(overloads.TimeGetHours, overloads.DurationToHours, []ref.Val{})
	if !hr.Equal(Int(2)).(Bool) {
		t.Error("Expected 2 hours, got", hr)
	}
}

func TestDurationGetMinutes(t *testing.T) {
	d := Duration{Duration: duration(7506, 0)}
	min := d.Receive(overloads.TimeGetMinutes, overloads.DurationToMinutes, []ref.Val{})
	if !min.Equal(Int(125)).(Bool) {
		t.Error("Expected 5 minutes, got", min)
	}
}

func TestDurationGetSeconds(t *testing.T) {
	d := Duration{Duration: duration(7506, 0)}
	sec := d.Receive(overloads.TimeGetSeconds, overloads.DurationToSeconds, []ref.Val{})
	if !sec.Equal(Int(7506)).(Bool) {
		t.Error("Expected 6 seconds, got", sec)
	}
}

func TestDurationGetMilliseconds(t *testing.T) {
	d := Duration{Duration: duration(7506, 0)}
	sec := d.Receive(overloads.TimeGetMilliseconds, overloads.DurationToMilliseconds, []ref.Val{})
	if !sec.Equal(Int(7506000)).(Bool) {
		t.Error("Expected 6 seconds, got", sec)
	}
}

func TestDurationIsZeroValue(t *testing.T) {
	if (&Duration{Duration: time.Duration(1)}).IsZeroValue() {
		t.Error("Duration(1).IsZeroValue() returned true, wanted false.")
	}
	if !(&Duration{Duration: time.Duration(0)}).IsZeroValue() {
		t.Error("Duration(0).IsZeroValue() returned false, wanted true")
	}
}

func duration(seconds, nanos int64) time.Duration {
	return time.Duration(seconds)*time.Second + time.Duration(nanos)
}
