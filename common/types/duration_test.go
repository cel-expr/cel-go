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
	dMax := duration(maxDurationSeconds, 999_999_999)
	dMin := duration(minDurationSeconds, -999_999_999)

	tests := []struct {
		name string
		op   func() ref.Val
		out  any
	}{
		// Addition tests.
		{
			name: "DurationAddSelf",
			op: func() ref.Val {
				return d.Add(d)
			},
			out: duration(15012, 1134),
		},
		{
			name: "DurationMaxAddOneNanoOverflow",
			op: func() ref.Val {
				return dMax.Add(dNano)
			},
			out: errDurationOverflow,
		},
		{
			name: "DurationMaxAddOneSecondOverflow",
			op: func() ref.Val {
				return dMax.Add(dSecond)
			},
			out: errDurationOverflow,
		},
		{
			name: "DurationMinAddMinusOneOverflow",
			op: func() ref.Val {
				return dMin.Add(duration(-1, 0))
			},
			out: errDurationOverflow,
		},

		// Subtraction tests.
		{
			name: "DurationSubSelf",
			op: func() ref.Val {
				return d.Subtract(d)
			},
			out: duration(0, 0),
		},
		{
			name: "DurationMaxSubMinusOneOverflow",
			op: func() ref.Val {
				return dMax.Subtract(duration(0, -1))
			},
			out: errDurationOverflow,
		},
		{
			name: "DurationMinSubOneOverflow",
			op: func() ref.Val {
				return dMin.Subtract(dNano)
			},
			out: errDurationOverflow,
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
	d := Duration{Seconds: 7506}
	lt := Duration{Seconds: -10}
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
	dur := duration(7506, 1000)
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
	wantTD := 7506*time.Second + 1000*time.Nanosecond
	if !reflect.DeepEqual(val, wantTD) {
		t.Errorf("got value %v, wanted %v", val, wantTD)
	}
	// Out of range for time.Duration
	largeDur := Duration{Seconds: maxDurationSeconds}
	if _, err := largeDur.ConvertToNative(reflect.TypeFor[time.Duration]()); err == nil {
		t.Error("expected error converting large Duration to time.Duration, got nil")
	}
}

func TestDurationConvertToNative_Any(t *testing.T) {
	d := duration(7506, 1000)
	val, err := d.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	want, err := anypb.New(&dpb.Duration{Seconds: 7506, Nanos: 1000})
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', wanted %v", val, want)
	}
}

func TestDurationConvertToNative_Error(t *testing.T) {
	val, err := duration(7506, 1000).ConvertToNative(JSONValueType)
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
	val, err := duration(7506, 1000).ConvertToNative(JSONValueType)
	if err != nil {
		t.Error(err)
	}
	want := structpb.NewStringValue("7506.000001s")
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', wanted %v", val, want)
	}
}

func TestDurationConvertToType_Identity(t *testing.T) {
	d := duration(7506, 1000)
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
	neg := duration(1234, 1).Negate()
	want := duration(-1234, -1)
	if neg.Value().(Duration) != want {
		t.Errorf("Got %v, expected %v", neg, want)
	}
	maxDur := Duration{Seconds: maxDurationSeconds, Nanos: 999_999_999}
	minDur := Duration{Seconds: minDurationSeconds, Nanos: -999_999_999}
	if !maxDur.Negate().Equal(minDur).(Bool) {
		t.Errorf("Expected negating %v to yield %v, got %v", maxDur, minDur, maxDur.Negate())
	}
	if !minDur.Negate().Equal(maxDur).(Bool) {
		t.Errorf("Expected negating %v to yield %v, got %v", minDur, maxDur, minDur.Negate())
	}
}

func TestDurationGetHours(t *testing.T) {
	d := duration(7506, 0)
	hr := d.Receive(overloads.TimeGetHours, overloads.DurationToHours, []ref.Val{})
	if !hr.Equal(Int(2)).(Bool) {
		t.Error("Expected 2 hours, got", hr)
	}
}

func TestDurationGetMinutes(t *testing.T) {
	d := duration(7506, 0)
	min := d.Receive(overloads.TimeGetMinutes, overloads.DurationToMinutes, []ref.Val{})
	if !min.Equal(Int(125)).(Bool) {
		t.Error("Expected 125 minutes, got", min)
	}
}

func TestDurationGetSeconds(t *testing.T) {
	d := duration(7506, 0)
	sec := d.Receive(overloads.TimeGetSeconds, overloads.DurationToSeconds, []ref.Val{})
	if !sec.Equal(Int(7506)).(Bool) {
		t.Error("Expected 7506 seconds, got", sec)
	}
}

func TestDurationGetMilliseconds(t *testing.T) {
	d := duration(7506, 0)
	ms := d.Receive(overloads.TimeGetMilliseconds, overloads.DurationToMilliseconds, []ref.Val{})
	if !ms.Equal(Int(7506000)).(Bool) {
		t.Error("Expected 7506000 milliseconds, got", ms)
	}
}

func TestDurationIsZeroValue(t *testing.T) {
	if (&Duration{Nanos: 1}).IsZeroValue() {
		t.Error("Duration(1).IsZeroValue() returned true, wanted false.")
	}
	if !(&Duration{}).IsZeroValue() {
		t.Error("Duration(0).IsZeroValue() returned false, wanted true")
	}
}

func duration(seconds int64, nanos int32) Duration {
	return Duration{Seconds: seconds, Nanos: nanos}
}

func TestDurationPrecisionUnits(t *testing.T) {
	oneNano := Duration{Nanos: 1}
	tests := []struct {
		name string
		dur  Duration
		fn   func(ref.Val) ref.Val
		want Int
	}{
		{
			name: "4097h-1ns",
			dur:  Duration{Seconds: 4097 * 3600}.Subtract(oneNano).(Duration),
			fn:   DurationGetHours,
			want: 4096,
		},
		{
			name: "2562047h-1ns",
			dur:  Duration{Seconds: 2562047 * 3600}.Subtract(oneNano).(Duration),
			fn:   DurationGetHours,
			want: 2562046,
		},
		{
			name: "262145m-1ns",
			dur:  Duration{Seconds: 262145 * 60}.Subtract(oneNano).(Duration),
			fn:   DurationGetMinutes,
			want: 262144,
		},
		{
			name: "2562047m-1ns",
			dur:  Duration{Seconds: 2562047 * 60}.Subtract(oneNano).(Duration),
			fn:   DurationGetMinutes,
			want: 2562046,
		},
		{
			name: "16777217s-1ns",
			dur:  Duration{Seconds: 16777217}.Subtract(oneNano).(Duration),
			fn:   DurationGetSeconds,
			want: 16777216,
		},
		{
			name: "9223372036s-1ns",
			dur:  Duration{Seconds: 9223372036}.Subtract(oneNano).(Duration),
			fn:   DurationGetSeconds,
			want: 9223372035,
		},
		{
			name: "315576000000s-1ns",
			dur:  Duration{Seconds: 315576000000}.Subtract(oneNano).(Duration),
			fn:   DurationGetSeconds,
			want: 315575999999,
		},
	}
	for _, tc := range tests {
		got := tc.fn(tc.dur)
		if !got.Equal(tc.want).(Bool) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDurationStringFormatting(t *testing.T) {
	tests := []struct {
		dur  Duration
		want string
	}{
		{Duration{}, "0s"},
		{Duration{Seconds: 5}, "5s"},
		{Duration{Seconds: -5}, "-5s"},
		{Duration{Nanos: 500_000_000}, "0.5s"},
		{Duration{Nanos: -500_000_000}, "-0.5s"},
		{Duration{Nanos: -1}, "-0.000000001s"},
		{Duration{Seconds: -5, Nanos: -1}, "-5.000000001s"},
		{Duration{Seconds: 360000000, Nanos: 1}, "360000000.000000001s"},
		{Duration{Seconds: 10000000, Nanos: 1}, "10000000.000000001s"},
		{Duration{Seconds: 9223372036, Nanos: 854_775_807}, "9223372036.854775807s"},
		{Duration{Seconds: 315576000000, Nanos: 999_999_999}, "315576000000.999999999s"},
	}
	for _, tc := range tests {
		got := string(tc.dur.ConvertToType(StringType).(String))
		if got != tc.want {
			t.Errorf("ConvertToType(StringType) for %v = %q, want %q", tc.dur, got, tc.want)
		}
		if tc.dur.String() != tc.want {
			t.Errorf("%v.String() = %q, want %q", tc.dur, tc.dur.String(), tc.want)
		}
	}
}
