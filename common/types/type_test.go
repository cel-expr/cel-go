// Copyright 2019 Google LLC
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
	"encoding/json"
	"reflect"
	"testing"

	"cel.dev/cel-go/common/types/ref"
)

func TestType_ConvertToType(t *testing.T) {
	stdTypes := []ref.Val{
		BoolType,
		BytesType,
		DoubleType,
		DurationType,
		IntType,
		ListType,
		MapType,
		NullType,
		StringType,
		TimestampType,
		TypeType,
		UintType,
	}
	for _, stdType := range stdTypes {
		cnv := stdType.ConvertToType(TypeType)
		if !cnv.Equal(TypeType).(Bool) {
			t.Errorf("Got %v, wanted 'type'", cnv)
		}
	}
}

func TestType_Type(t *testing.T) {
	if TypeType.Type() != TypeType {
		t.Error("type of type is not type.")
	}
}

func TestType_JSONError(t *testing.T) {
	if _, err := IntType.ConvertToNative(JSONValueType); err == nil {
		t.Error("IntType.ConvertToNative(JSONValueType) succeeded, wanted error")
	}
	if _, err := json.Marshal(IntType); err == nil {
		t.Error("json.Marshal(IntType) succeeded, wanted error")
	}
	nt, err := newNativeType(reflect.TypeFor[struct{ Name string }](), NativeTypeOptions{})
	if err != nil {
		t.Fatalf("newNativeType() failed: %v", err)
	}
	if _, err := nt.ConvertToNative(JSONValueType); err == nil {
		t.Error("NativeType.ConvertToNative(JSONValueType) succeeded, wanted error")
	}
	if _, err := json.Marshal(nt); err == nil {
		t.Error("json.Marshal(NativeType) succeeded, wanted error")
	}
}
