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
	"testing"

	"google.golang.org/protobuf/proto"

	proto3pb "cel.dev/cel-go/test/proto3pb"
	dynamicpb "google.golang.org/protobuf/types/dynamicpb"
	anypb "google.golang.org/protobuf/types/known/anypb"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

func TestNullConvertToNative(t *testing.T) {
	tests := []struct {
		goType reflect.Type
		out    any
		err    error
	}{
		{
			goType: JSONValueType,
			out:    structpb.NewNullValue(),
		},
		{
			goType: JSONNullType,
			out:    structpb.NullValue_NULL_VALUE,
		},
		{
			goType: anyValueType,
			out:    testPackAny(t, structpb.NewNullValue()),
		},
		{
			goType: reflect.TypeFor[Null](),
			out:    NullValue,
		},
		{goType: boolWrapperType},
		{goType: byteWrapperType},
		{goType: doubleWrapperType},
		{goType: floatWrapperType},
		{goType: int32WrapperType},
		{goType: int64WrapperType},
		{goType: stringWrapperType},
		{goType: uint32WrapperType},
		{goType: uint64WrapperType},
		{goType: durationValueType},
		{goType: timestampValueType},
		{goType: reflect.TypeFor[*dynamicpb.Message]()},
		{goType: reflect.TypeFor[*proto3pb.TestAllTypes]()},
		{goType: reflect.TypeFor[*proto3pb.TestAllTypes]()},
		{
			goType: reflect.TypeFor[int](),
			err:    errors.New("type conversion error from 'null_type' to 'int'"),
		},
		{
			goType: JSONListType,
			err:    errors.New("type conversion error from 'null_type' to '*structpb.ListValue'"),
		},
		{
			goType: JSONStructType,
			err:    errors.New("type conversion error from 'null_type' to '*structpb.Struct'"),
		},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			out, err := NullValue.ConvertToNative(tc.goType)
			if err != nil {
				if tc.err == nil {
					t.Fatalf("NullValue.ConvertToType(%v) failed: %v", tc.goType, err)
				}
				if tc.err.Error() != err.Error() {
					t.Errorf("NullValue.ConvertToType(%v) got error %v, wanted error %v", tc.goType, err, tc.err)
				}
				return
			}
			pbMsg, isPB := out.(proto.Message)
			if (isPB && !proto.Equal(pbMsg, tc.out.(proto.Message))) || (!isPB && out != tc.out) {
				t.Errorf("NullValue.ConvertToNative(%v) got %v, wanted %v", tc.goType, pbMsg, tc.out)
			}
		})
	}
}

func TestNullConvertToType(t *testing.T) {
	if !NullValue.ConvertToType(NullType).Equal(NullValue).(Bool) {
		t.Error("Failed to get NullType of NullValue.")
	}

	if !NullValue.ConvertToType(StringType).Equal(String("null")).(Bool) {
		t.Error("Failed to get StringType of NullValue.")
	}
	if !NullValue.ConvertToType(TypeType).Equal(NullType).(Bool) {
		t.Error("Failed to convert NullValue to type.")
	}
}

func TestNullEqual(t *testing.T) {
	if !NullValue.Equal(NullValue).(Bool) {
		t.Error("NullValue does not equal to itself.")
	}
}

func TestNullIsZeroValue(t *testing.T) {
	if !NullValue.IsZeroValue() {
		t.Error("NullValue.IsZeroValue() returned false, wanted true")
	}
}

func TestNullType(t *testing.T) {
	if NullValue.Type() != NullType {
		t.Error("NullValue gets incorrect type.")
	}
}

func TestNullValue(t *testing.T) {
	if NullValue.Value() != structpb.NullValue_NULL_VALUE {
		t.Error("NullValue gets incorrect value.")
	}
}

func testPackAny(t *testing.T, val proto.Message) *anypb.Any {
	t.Helper()
	out, err := anypb.New(val)
	if err != nil {
		t.Fatalf("anypb.New(%v) failed: %v", val, err)
	}
	return out
}
