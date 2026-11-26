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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types/ref"

	anypb "google.golang.org/protobuf/types/known/anypb"
	dpb "google.golang.org/protobuf/types/known/durationpb"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

const (
	// minDurationSeconds is the minimum supported duration in seconds (-10000 years).
	minDurationSeconds int64 = -315576000000
	// maxDurationSeconds is the maximum supported duration in seconds (+10000 years).
	maxDurationSeconds int64 = 315576000000
	nanosPerSecond     int64 = 1_000_000_000
)

var errDurationRange = errors.New("duration out of range for time.Duration")

// Duration type that implements ref.Val and supports add, compare, negate,
// and subtract operators. This type is also a receiver which means it can
// participate in dispatch to receiver functions.
type Duration struct {
	Seconds int64
	Nanos   int32
}

func durationOf(d time.Duration) Duration {
	sec := int64(d) / nanosPerSecond
	nsec := int32(int64(d) % nanosPerSecond)
	return Duration{Seconds: sec, Nanos: nsec}
}

func normalizeDuration(sec, nsec int64) (Duration, error) {
	if nsec <= -nanosPerSecond || nsec >= nanosPerSecond {
		var err error
		sec, err = addInt64Checked(sec, nsec/nanosPerSecond)
		if err != nil {
			return Duration{}, errDurationOverflow
		}
		nsec %= nanosPerSecond
	}
	if sec > 0 && nsec < 0 {
		sec--
		nsec += nanosPerSecond
	} else if sec < 0 && nsec > 0 {
		sec++
		nsec -= nanosPerSecond
	}
	if sec < minDurationSeconds || sec > maxDurationSeconds {
		return Duration{}, errDurationOverflow
	}
	return Duration{Seconds: sec, Nanos: int32(nsec)}, nil
}

var (
	durationValueType = reflect.TypeFor[*dpb.Duration]()

	durationZeroArgOverloads = map[string]func(ref.Val) ref.Val{
		overloads.TimeGetHours:        DurationGetHours,
		overloads.TimeGetMinutes:      DurationGetMinutes,
		overloads.TimeGetSeconds:      DurationGetSeconds,
		overloads.TimeGetMilliseconds: DurationGetMilliseconds,
	}
)

// Add implements traits.Adder.Add.
func (d Duration) Add(other ref.Val) ref.Val {
	switch other.Type() {
	case DurationType:
		dur2 := other.(Duration)
		val, err := addDurationChecked(d, dur2)
		if err != nil {
			return WrapErr(err)
		}
		return val
	case TimestampType:
		ts := other.(Timestamp).Time
		val, err := addTimeDurationChecked(ts, d)
		if err != nil {
			return WrapErr(err)
		}
		return timestampOf(val)
	}
	return MaybeNoSuchOverloadErr(other)
}

// Compare implements traits.Comparer.Compare.
func (d Duration) Compare(other ref.Val) ref.Val {
	otherDur, ok := other.(Duration)
	if !ok {
		return MaybeNoSuchOverloadErr(other)
	}
	switch {
	case d.Seconds < otherDur.Seconds:
		return IntNegOne
	case d.Seconds > otherDur.Seconds:
		return IntOne
	case d.Nanos < otherDur.Nanos:
		return IntNegOne
	case d.Nanos > otherDur.Nanos:
		return IntOne
	default:
		return IntZero
	}
}

func (d Duration) toTimeDuration() (time.Duration, error) {
	tsec, err := multiplyInt64Checked(d.Seconds, nanosPerSecond)
	if err != nil {
		return 0, errDurationRange
	}
	nsec, err := addInt64Checked(tsec, int64(d.Nanos))
	if err != nil {
		return 0, errDurationRange
	}
	return time.Duration(nsec), nil
}

// ConvertToNative implements ref.Val.ConvertToNative.
func (d Duration) ConvertToNative(typeDesc reflect.Type) (any, error) {
	// If the duration is already assignable to the desired type return it.
	if reflect.TypeFor[time.Duration]().AssignableTo(typeDesc) {
		return d.toTimeDuration()
	}
	if reflect.TypeFor[Duration]().AssignableTo(typeDesc) {
		return d, nil
	}
	switch typeDesc {
	case anyValueType:
		// Pack the duration as a dpb.Duration into an Any value.
		return anypb.New(&dpb.Duration{Seconds: d.Seconds, Nanos: d.Nanos})
	case durationValueType:
		// Unwrap the CEL value to its underlying proto value.
		return &dpb.Duration{Seconds: d.Seconds, Nanos: d.Nanos}, nil
	case JSONValueType:
		// CEL follows the proto3 to JSON conversion.
		// Note, using jsonpb would wrap the result in extra double quotes.
		v := d.ConvertToType(StringType)
		if IsError(v) {
			return nil, v.(*Err)
		}
		return structpb.NewStringValue(string(v.(String))), nil
	}
	return nil, fmt.Errorf("type conversion error from 'Duration' to '%v'", typeDesc)
}

// ConvertToType implements ref.Val.ConvertToType.
func (d Duration) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case StringType:
		var sb strings.Builder
		d.appendString(&sb)
		return String(sb.String())
	case IntType:
		tsec, err := multiplyInt64Checked(d.Seconds, nanosPerSecond)
		if err != nil {
			return WrapErr(err)
		}
		nsec, err := addInt64Checked(tsec, int64(d.Nanos))
		if err != nil {
			return WrapErr(err)
		}
		return Int(nsec)
	case DurationType:
		return d
	case TypeType:
		return DurationType
	}
	return NewErr("type conversion error from '%s' to '%s'", DurationType, typeVal)
}

// Equal implements ref.Val.Equal.
func (d Duration) Equal(other ref.Val) ref.Val {
	otherDur, ok := other.(Duration)
	return Bool(ok && d.Seconds == otherDur.Seconds && d.Nanos == otherDur.Nanos)
}

// IsZeroValue returns true if the duration value is zero
func (d Duration) IsZeroValue() bool {
	return d.Seconds == 0 && d.Nanos == 0
}

// Negate implements traits.Negater.Negate.
func (d Duration) Negate() ref.Val {
	val, err := negateDurationChecked(d)
	if err != nil {
		return WrapErr(err)
	}
	return val
}

// Receive implements traits.Receiver.Receive.
func (d Duration) Receive(function string, overload string, args []ref.Val) ref.Val {
	if len(args) == 0 {
		if f, found := durationZeroArgOverloads[function]; found {
			return f(d)
		}
	}
	return NoSuchOverloadErr()
}

// Subtract implements traits.Subtractor.Subtract.
func (d Duration) Subtract(subtrahend ref.Val) ref.Val {
	subtraDur, ok := subtrahend.(Duration)
	if !ok {
		return MaybeNoSuchOverloadErr(subtrahend)
	}
	val, err := subtractDurationChecked(d, subtraDur)
	if err != nil {
		return WrapErr(err)
	}
	return val
}

// Type implements ref.Val.Type.
func (d Duration) Type() ref.Type {
	return DurationType
}

// Value implements ref.Val.Value.
func (d Duration) Value() any {
	return d
}

func (d Duration) appendString(sb *strings.Builder) {
	sec := d.Seconds
	nsec := int64(d.Nanos)
	if sec < 0 || nsec < 0 {
		sb.WriteByte('-')
		sec = -sec
		nsec = -nsec
	}
	sb.WriteString(strconv.FormatInt(sec, 10))
	if nsec != 0 {
		sb.WriteByte('.')
		var buf [9]byte
		for i := 8; i >= 0; i-- {
			buf[i] = byte('0' + nsec%10)
			nsec /= 10
		}
		end := 9
		for end > 0 && buf[end-1] == '0' {
			end--
		}
		sb.Write(buf[:end])
	}
	sb.WriteByte('s')
}

func (d Duration) String() string {
	var sb strings.Builder
	d.appendString(&sb)
	return sb.String()
}

func (d Duration) format(sb *strings.Builder) {
	sb.WriteString(`duration("`)
	d.appendString(sb)
	sb.WriteString(`")`)
}

// DurationGetHours returns the duration in hours.
func DurationGetHours(val ref.Val) ref.Val {
	dur, ok := val.(Duration)
	if !ok {
		return MaybeNoSuchOverloadErr(val)
	}
	return Int(dur.Seconds / 3600)
}

// DurationGetMinutes returns duration in minutes.
func DurationGetMinutes(val ref.Val) ref.Val {
	dur, ok := val.(Duration)
	if !ok {
		return MaybeNoSuchOverloadErr(val)
	}
	return Int(dur.Seconds / 60)
}

// DurationGetSeconds returns duration in seconds.
func DurationGetSeconds(val ref.Val) ref.Val {
	dur, ok := val.(Duration)
	if !ok {
		return MaybeNoSuchOverloadErr(val)
	}
	return Int(dur.Seconds)
}

// DurationGetMilliseconds returns duration in milliseconds.
func DurationGetMilliseconds(val ref.Val) ref.Val {
	dur, ok := val.(Duration)
	if !ok {
		return MaybeNoSuchOverloadErr(val)
	}
	return Int(dur.Seconds*1000 + int64(dur.Nanos)/1_000_000)
}
