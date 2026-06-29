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
	"testing"
	"time"

	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types/ref"

	"google.golang.org/protobuf/proto"

	anypb "google.golang.org/protobuf/types/known/anypb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestTimestampConvertToType(t *testing.T) {
	ts := Timestamp{Time: time.Unix(7654, 321).UTC()}
	if ts.ConvertToType(TypeType) != TimestampType {
		t.Errorf("ConvertToType(type) failed to return timestamp type: %v", ts.ConvertToType(TypeType))
	}
	if ts.ConvertToType(IntType) != Int(7654) {
		t.Errorf("ConvertToType(int) failed to truncate a timestamp to a unix epoch: %v", ts.ConvertToType(IntType))
	}
	if ts.ConvertToType(StringType) != String("1970-01-01T02:07:34.000000321Z") {
		t.Errorf("ConvertToType(string) failed to convert to a human readable timestamp. "+
			"got %v, wanted: 1970-01-01T02:07:34.000000321Z",
			ts.ConvertToType(StringType))
	}
	if ts.ConvertToType(TimestampType) != ts {
		t.Error("ConvertToType(timestamp) failed an identity conversion")
	}
	if !IsError(ts.ConvertToType(DurationType)) {
		t.Error("ConvertToType(duration) failed to error")
	}
}

func TestTimestampOperators(t *testing.T) {
	unixTimestamp := func(epoch int64) Timestamp {
		return timestampOf(time.Unix(epoch, 0).UTC())
	}
	tests := []struct {
		name string
		op   func() ref.Val
		out  any
	}{
		// Addition tests.
		{
			name: "DateAddOneHourMinusOneMilli",
			op: func() ref.Val {
				return unixTimestamp(3506).Add(durationOf(time.Hour - time.Millisecond))
			},
			out: time.Unix(7106, 0).Add(-time.Millisecond).UTC(),
		},
		{
			name: "DateAddOneHourOneNano",
			op: func() ref.Val {
				return unixTimestamp(3506).Add(durationOf(time.Hour + time.Nanosecond))
			},
			out: time.Unix(7106, 1).UTC(),
		},
		{
			name: "IntMaxAddOneSecond",
			op: func() ref.Val {
				return unixTimestamp(math.MaxInt64).Add(durationOf(time.Second))
			},
			out: errIntOverflow,
		},
		{
			name: "MaxTimestampAddOneSecond",
			op: func() ref.Val {
				return unixTimestamp(maxUnixTime).Add(durationOf(time.Second))
			},
			out: errTimestampOverflow,
		},
		{
			name: "MaxIntAddOneViaNanos",
			op: func() ref.Val {
				return timestampOf(time.Unix(math.MaxInt64, 999_999_999).UTC()).Add(durationOf(time.Nanosecond))
			},
			out: errIntOverflow,
		},
		{
			name: "SecondsWithNanosNegative",
			op: func() ref.Val {
				ts1 := unixTimestamp(1).Add(durationOf(time.Nanosecond)).(Timestamp)
				return ts1.Add(durationOf(-999_999_999))
			},
			out: time.Unix(0, 2).UTC(),
		},
		{
			name: "SecondsWithNanosPositive",
			op: func() ref.Val {
				ts1 := unixTimestamp(1).Add(durationOf(999_999_999 * time.Nanosecond)).(Timestamp)
				return ts1.Add(durationOf(999_999_999))
			},
			out: time.Unix(2, 999_999_998).UTC(),
		},
		{
			name: "DateAddDateError",
			op: func() ref.Val {
				return unixTimestamp(1).Add(unixTimestamp(1))
			},
			out: errors.New("no such overload"),
		},

		// Comparison tests.
		{
			name: "DateCompareEqual",
			op: func() ref.Val {
				return unixTimestamp(1).Compare(unixTimestamp(1))
			},
			out: int64(0),
		},
		{
			name: "DateCompareBefore",
			op: func() ref.Val {
				return unixTimestamp(1).Compare(unixTimestamp(200))
			},
			out: int64(-1),
		},
		{
			name: "DateCompareAfter",
			op: func() ref.Val {
				return unixTimestamp(1000).Compare(unixTimestamp(200))
			},
			out: int64(1),
		},
		{
			name: "DateCompareError",
			op: func() ref.Val {
				return unixTimestamp(1000).Compare(durationOf(1000))
			},
			out: errors.New("no such overload"),
		},

		// Time subtraction tests.
		{
			name: "TimeSubOneSecond",
			op: func() ref.Val {
				return unixTimestamp(100).Subtract(unixTimestamp(1))
			},
			out: 99 * time.Second,
		},
		{
			name: "DateSubOneHour",
			op: func() ref.Val {
				return unixTimestamp(3506).Subtract(durationOf(time.Hour))
			},
			out: time.Unix(-94, 0).UTC(),
		},
		{
			name: "MinTimestampSubOneSecond",
			op: func() ref.Val {
				return unixTimestamp(-62135596800).Subtract(durationOf(time.Second))
			},
			out: errTimestampOverflow,
		},
		{
			name: "MinTimestampSubMinusOneViaNanos",
			op: func() ref.Val {
				return timestampOf(time.Unix(-62135596800, 2).UTC()).Subtract(durationOf(-999_999_999 * time.Nanosecond))
			},
			out: time.Unix(-62135596799, 1).UTC(),
		},
		{
			name: "MinIntSubOneViaNanosOverflow",
			op: func() ref.Val {
				return timestampOf(time.Unix(math.MinInt64, 0).UTC()).Subtract(durationOf(time.Nanosecond))
			},
			out: errIntOverflow,
		},
		{
			name: "TimeWithNanosPositive",
			op: func() ref.Val {
				return timestampOf(time.Unix(2, 1)).Subtract(timestampOf(time.Unix(0, 999_999_999)))
			},
			out: time.Second + 2*time.Nanosecond,
		},
		{
			name: "TimeWithNanosNegative",
			op: func() ref.Val {
				return timestampOf(time.Unix(1, 1)).Subtract(timestampOf(time.Unix(2, 999_999_999)))
			},
			out: -2*time.Second + 2*time.Nanosecond,
		},
		{
			name: "MinTimestampMinusOne",
			op: func() ref.Val {
				return unixTimestamp(math.MinInt64).Subtract(unixTimestamp(1))
			},
			out: errIntOverflow,
		},
		{
			name: "DateMinusDateDurationOverflow",
			op: func() ref.Val {
				return unixTimestamp(maxUnixTime).Subtract(unixTimestamp(minUnixTime))
			},
			out: errIntOverflow,
		},
		{
			name: "MinTimestampMinusOneViaNanosScaleOverflow",
			op: func() ref.Val {
				return timestampOf(time.Unix(math.MinInt64, 1)).Subtract(timestampOf(time.Unix(0, -999_999_999)))
			},
			out: errIntOverflow,
		},
		{
			name: "DateSubMinDuration",
			op: func() ref.Val {
				return unixTimestamp(1).Subtract(durationOf(math.MinInt64))
			},
			out: errIntOverflow,
		},
	}
	for _, tst := range tests {
		got := tst.op()
		switch v := got.Value().(type) {
		case time.Time:
			if want, ok := tst.out.(time.Time); !ok || !v.Equal(want) {
				t.Errorf("%s: got %v, wanted %v", tst.name, v, tst.out)
			}
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

func TestTimestampConvertToNative_Any(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0)}
	val, err := ts.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	want, err := anypb.New(tpb.New(ts.Time))
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got '%v', expected '%v'", val, want)
	}
}

func TestTimestampConvertToNative(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	val, err := ts.ConvertToNative(timestampValueType)
	if err != nil {
		t.Error(err)
	}
	var want any
	want = tpb.New(ts.Time)
	if !proto.Equal(val.(proto.Message), want.(proto.Message)) {
		t.Errorf("Got '%v', expected '%v'", val, want)
	}
	val, err = ts.ConvertToNative(JSONValueType)
	if err != nil {
		t.Error(err)
	}
	want = structpb.NewStringValue("1970-01-01T02:05:06Z")
	if !proto.Equal(val.(proto.Message), want.(proto.Message)) {
		t.Errorf("Got '%v', expected '%v'", val, want)
	}
	val, err = ts.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	want, err = anypb.New(tpb.New(ts.Time))
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want.(proto.Message)) {
		t.Errorf("Got '%v', expected '%v'", val, want)
	}
	val, err = ts.ConvertToNative(reflect.TypeOf(Timestamp{}))
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(val, ts) {
		t.Errorf("got %v wanted %v", val, ts)
	}
	val, err = ts.ConvertToNative(reflect.TypeOf(time.Now()))
	if err != nil {
		t.Error(err)
	}
	want = time.Unix(7506, 0).UTC()
	if !reflect.DeepEqual(val, want) {
		t.Errorf("got %v wanted %v", val, want)
	}
}

func TestTimestampIsZeroValue(t *testing.T) {
	if (Timestamp{Time: time.Now()}).IsZeroValue() {
		t.Error("Timestamp(Now()).IsZeroValue() returned true, wanted false.")
	}
	if (Timestamp{Time: time.Unix(0, 0)}).IsZeroValue() {
		t.Error("Timestamp(0).IsZeroValue() returned true, wanted false.")
	}
}

func TestTimestampGetDayOfMonth(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := timestampOf(time.Unix(7506, 0).UTC())
	mon := ts.Receive(overloads.TimeGetDayOfMonth, overloads.TimestampToDayOfMonthZeroBased, []ref.Val{})
	if !mon.Equal(Int(0)).(Bool) {
		t.Errorf("ts.getDayOfMonth() got %v, wanted 0", mon)
	}
	// 1969-12-31T19:05:06Z
	monTz := ts.Receive(overloads.TimeGetDayOfMonth, overloads.TimestampToDayOfMonthZeroBasedWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !monTz.Equal(Int(30)).(Bool) {
		t.Errorf("ts.getDayOfMonth() got %v, wanted 30", mon)
	}
	// 1969-12-31T19:05:06Z
	monTz = ts.Receive(overloads.TimeGetDayOfMonth, overloads.TimestampToDayOfMonthZeroBasedWithTz,
		[]ref.Val{String("-07:00")})
	if !monTz.Equal(Int(30)).(Bool) {
		t.Errorf("ts.getDayOfMonth() got %v, wanted 30", mon)
	}

	// 1970-01-01T02:05:06Z
	mon = ts.Receive(overloads.TimeGetDate, overloads.TimestampToDayOfMonthOneBased, []ref.Val{})
	if !mon.Equal(Int(1)).(Bool) {
		t.Errorf("ts.getDate() got %v, wanted 1", mon)
	}
	// 1969-12-31T19:05:06Z
	monTz = ts.Receive(overloads.TimeGetDate, overloads.TimestampToDayOfMonthOneBasedWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !monTz.Equal(Int(31)).(Bool) {
		t.Errorf("ts.getDate() got %v, wanted 31", mon)
	}
	// 1970-01-02T01:05:06Z
	monTz = ts.Receive(overloads.TimeGetDate, overloads.TimestampToDayOfMonthOneBasedWithTz,
		[]ref.Val{String("+23:00")})
	if !monTz.Equal(Int(2)).(Bool) {
		t.Errorf("ts.getDate() got %v, wanted 2", mon)
	}
}

func TestTimestampGetDayOfYear(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := timestampOf(time.Unix(7506, 0).UTC())
	hr := ts.Receive(overloads.TimeGetDayOfYear, overloads.TimestampToDayOfYear, []ref.Val{})
	if !hr.Equal(Int(0)).(Bool) {
		t.Error("Expected 0, got", hr)
	}
	// 1969-12-31T19:05:06Z
	hrTz := ts.Receive(overloads.TimeGetDayOfYear, overloads.TimestampToDayOfYearWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !hrTz.Equal(Int(364)).(Bool) {
		t.Error("Expected 364, got", hrTz)
	}
	hrTz = ts.Receive(overloads.TimeGetDayOfYear, overloads.TimestampToDayOfYearWithTz,
		[]ref.Val{String("-07:00")})
	if !hrTz.Equal(Int(364)).(Bool) {
		t.Error("Expected 364, got", hrTz)
	}
}

func TestTimestampGetFullYear(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	year := ts.Receive(overloads.TimeGetFullYear, overloads.TimestampToYear, []ref.Val{})
	if !year.Equal(Int(1970)).(Bool) {
		t.Errorf("ts.getFullYear() got %v, wanted 1970", year)
	}
	// 1969-12-31T19:05:06Z
	yearTz := ts.Receive(overloads.TimeGetFullYear, overloads.TimestampToYearWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !yearTz.Equal(Int(1969)).(Bool) {
		t.Errorf("ts.getFullYear('America/Phoenix') got %v, wanted 1969", yearTz)
	}
}

func TestTimestampGetMonth(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	hr := ts.Receive(overloads.TimeGetMonth, overloads.TimestampToMonth, []ref.Val{})
	if !hr.Equal(Int(0)).(Bool) {
		t.Errorf("ts.getMonth() got %v, wanted 0", hr)
	}
	// 1969-12-31T19:05:06Z
	hrTz := ts.Receive(overloads.TimeGetMonth, overloads.TimestampToMonthWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !hrTz.Equal(Int(11)).(Bool) {
		t.Errorf("ts.getMonth('America/Phoenix') got %v, wanted 11", hrTz)
	}
}

func TestTimestampGetDayOfWeek(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	day := ts.Receive(overloads.TimeGetDayOfWeek, overloads.TimestampToDayOfWeek, []ref.Val{})
	if !day.Equal(Int(4)).(Bool) {
		t.Errorf("ts.getDayOfWeek() got %v, wanted 4", day)
	}
	// 1969-12-31T19:05:06Z
	dayTz := ts.Receive(overloads.TimeGetDayOfWeek, overloads.TimestampToDayOfWeekWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !dayTz.Equal(Int(3)).(Bool) {
		t.Errorf("ts.getDayOfWeek('America/Phoenix') got %v, wanted 3", dayTz)
	}
}

func TestTimestampGetHours(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	hr := ts.Receive(overloads.TimeGetHours, overloads.TimestampToHours, []ref.Val{})
	if !hr.Equal(Int(2)).(Bool) {
		t.Errorf("ts.getHours() got %v, wanted 2", hr)
	}
	// 1969-12-31T19:05:06Z
	hrTz := ts.Receive(overloads.TimeGetHours, overloads.TimestampToHoursWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !hrTz.Equal(Int(19)).(Bool) {
		t.Errorf("ts.getHours('America/Phoenix') got %v, wanted 19 hours", hrTz)
	}
}

func TestTimestampGetMinutes(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	min := ts.Receive(overloads.TimeGetMinutes, overloads.TimestampToMinutes, []ref.Val{})
	if !min.Equal(Int(5)).(Bool) {
		t.Errorf("ts.getMinutes() got %v, wanted 5 minutes", min)
	}
	// 1969-12-31T19:05:06Z
	minTz := ts.Receive(overloads.TimeGetMinutes, overloads.TimestampToMinutesWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !minTz.Equal(Int(5)).(Bool) {
		t.Errorf("ts.getMinutes('America/Phoenix') got %v, wanted 5 minutes", min)
	}
	// A valid offset still resolves: -08:30 shifts 1970-01-01T02:05:06Z back to 17:35.
	minTz = ts.Receive(overloads.TimeGetMinutes, overloads.TimestampToMinutesWithTz,
		[]ref.Val{String("-08:30")})
	if !minTz.Equal(Int(35)).(Bool) {
		t.Errorf("ts.getMinutes('-08:30') got %v, wanted 35 minutes", minTz)
	}
	// Out-of-range and signed minute offsets are rejected rather than silently accepted.
	for _, tz := range []string{"+00:99", "-00:90", "+05:-30"} {
		if got := ts.Receive(overloads.TimeGetMinutes, overloads.TimestampToMinutesWithTz,
			[]ref.Val{String(tz)}); !IsError(got) {
			t.Errorf("ts.getMinutes(%q) got %v, wanted error", tz, got)
		}
	}
}

func TestTimestampGetSeconds(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 0).UTC()}
	sec := ts.Receive(overloads.TimeGetSeconds, overloads.TimestampToSeconds, []ref.Val{})
	if !sec.Equal(Int(6)).(Bool) {
		t.Errorf("ts.getSeconds() got %v, wanted 6 seconds", sec)
	}
	// 1969-12-31T19:05:06Z
	secTz := ts.Receive(overloads.TimeGetSeconds, overloads.TimestampToSecondsWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !secTz.Equal(Int(6)).(Bool) {
		t.Errorf("ts.getSeconds('America/Phoenix') got %v, wanted 6 seconds", secTz)
	}
}

func TestTimestampGetMilliseconds(t *testing.T) {
	// 1970-01-01T02:05:06Z
	ts := Timestamp{Time: time.Unix(7506, 1000000).UTC()}
	ms := ts.Receive(overloads.TimeGetMilliseconds, overloads.TimestampToMilliseconds, []ref.Val{})
	if !ms.Equal(Int(1)).(Bool) {
		t.Errorf("ts.getMilliseconds() got %v, wanted 1 ms", ms)
	}
	// 1969-12-31T19:05:06Z
	msTz := ts.Receive(overloads.TimeGetMilliseconds, overloads.TimestampToMillisecondsWithTz,
		[]ref.Val{String("America/Phoenix")})
	if !msTz.Equal(Int(1)).(Bool) {
		t.Errorf("ts.getMilliseconds('America/Phoenix') got %v, wanted 1 ms", msTz)
	}
}

func TestIsStrictRFC3339MatchesPattern(t *testing.T) {
	// Exercise the hand-rolled scan against strictRFC3339Pattern, including the
	// boundary cases for each field and a few well-formed timestamps. The two
	// must agree on every input.
	cases := []string{
		"2025-01-01T12:34:56Z",
		"2025-01-01T12:34:56z",
		"2025-01-01t12:34:56Z",
		"2025-01-01T12:34:56.123456789Z",
		"2025-01-01T12:34:56.1Z",
		"2025-01-01T00:00:00+00:00",
		"2025-01-01T23:59:60-08:00",
		"2025-01-01T12:34:56+14:00",
		"2025-01-01T12:34:56+05:30",
		"2025-01-01T20:00:00Z",
		"2025-01-01T23:59:59Z",
		// rejected forms
		"2025-01-01T12:34:56,123Z",
		"2025-01-01T1:34:56Z",
		"2025-01-01T12:3:56Z",
		"2025-01-01T12:34:5Z",
		"2025-01-01T24:00:00Z",
		"2025-01-01T12:60:00Z",
		"2025-01-01T12:34:61Z",
		"2025-01-01T12:34:56.Z",
		"2025-01-01T12:34:56+24:00",
		"2025-01-01T12:34:56+00:60",
		"2025-01-01T12:34:56+0530",
		"2025-01-01T12:34:56",
		"2025-01-01 12:34:56Z",
		"2025-1-01T12:34:56Z",
		"",
		"not-a-timestamp",
	}
	for _, s := range cases {
		if got, want := isStrictRFC3339(s), strictRFC3339Pattern.MatchString(s); got != want {
			t.Errorf("isStrictRFC3339(%q) = %v, strictRFC3339Pattern.MatchString = %v", s, got, want)
		}
	}
}
