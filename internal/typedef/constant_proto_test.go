// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package typedef

import (
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"

	celpb "cel.dev/expr"
)

func TestConstantProtoRoundTrip(t *testing.T) {
	tests := []struct {
		textproto string
		want      ref.Val
	}{
		{textproto: ``, want: nil},
		{textproto: `null_value: NULL_VALUE`, want: types.NullValue},
		{textproto: `bool_value: true`, want: types.True},
		{textproto: `int64_value: -123`, want: types.Int(-123)},
		{textproto: `uint64_value: 456`, want: types.Uint(456)},
		{textproto: `double_value: 3.25`, want: types.Double(3.25)},
		{textproto: `string_value: "hello"`, want: types.String("hello")},
		{textproto: `bytes_value: "\001\002\003"`, want: types.Bytes{1, 2, 3}},
		{textproto: `duration_value { seconds: 10 nanos: 5 }`, want: types.Duration{Duration: 10*time.Second + 5}},
		{textproto: `duration_value { seconds: -10 nanos: -5 }`, want: types.Duration{Duration: -10*time.Second - 5}},
		{textproto: `timestamp_value { seconds: 1791158400 nanos: 7 }`, want: types.Timestamp{Time: time.Unix(1791158400, 7).UTC()}},
	}
	for _, tc := range tests {
		t.Run(tc.textproto, func(t *testing.T) {
			input := &celpb.Constant{}
			if err := prototext.Unmarshal([]byte(tc.textproto), input); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			got, err := constantFromProto(input)
			if err != nil {
				t.Fatalf("constantFromProto() failed: %v", err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("constantFromProto() = %v, want nil", got)
				}
			} else if got == nil || got.Equal(tc.want) != types.True {
				t.Fatalf("constantFromProto() = %v, want %v", got, tc.want)
			}
			roundTrip, err := constantToProto(got)
			if err != nil {
				t.Fatalf("constantToProto() failed: %v", err)
			}
			if !proto.Equal(roundTrip, input) {
				t.Errorf("constantToProto(constantFromProto(%v)) = %v, want %v", input, roundTrip, input)
			}
		})
	}
}

func TestConstantFromProtoErrors(t *testing.T) {
	tests := []struct {
		name string
		in   *celpb.Constant
	}{
		{
			name: "invalid duration",
			in:   &celpb.Constant{ConstantKind: &celpb.Constant_DurationValue{DurationValue: &durationpb.Duration{Seconds: 1, Nanos: -1}}},
		},
		{
			name: "duration overflows time.Duration",
			in:   &celpb.Constant{ConstantKind: &celpb.Constant_DurationValue{DurationValue: &durationpb.Duration{Seconds: 315576000000}}},
		},
		{
			name: "negative duration overflows time.Duration",
			in:   &celpb.Constant{ConstantKind: &celpb.Constant_DurationValue{DurationValue: &durationpb.Duration{Seconds: -9223372037}}},
		},
		{
			name: "invalid timestamp",
			in:   &celpb.Constant{ConstantKind: &celpb.Constant_TimestampValue{TimestampValue: &timestamppb.Timestamp{Seconds: 253402300800}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := constantFromProto(tc.in); !errors.Is(err, ErrBadTypeDef) {
				t.Errorf("constantFromProto() got error %v, want ErrBadTypeDef", err)
			}
		})
	}
}

func TestConstantToProtoErrors(t *testing.T) {
	tests := []struct {
		name string
		in   ref.Val
	}{
		{name: "invalid timestamp", in: types.Timestamp{Time: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}},
		{name: "unsupported value", in: types.NewErr("unsupported")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := constantToProto(tc.in); !errors.Is(err, ErrBadTypeDef) {
				t.Errorf("constantToProto() got error %v, want ErrBadTypeDef", err)
			}
		})
	}
}
