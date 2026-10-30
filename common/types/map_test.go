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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types/pb"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"

	proto3pb "cel.dev/cel-go/test/proto3pb"
	anypb "google.golang.org/protobuf/types/known/anypb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

type testStruct struct {
	M            string
	Details      []string
	ExtraDetails string
}

func TestMapContains(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	reflectMap := reg.NativeToValue(map[any]any{
		int64(1):  "hello",
		uint64(2): "world",
	}).(traits.Mapper)

	refValMap := reg.NativeToValue(map[ref.Val]ref.Val{
		Int(1):  String("hello"),
		Uint(2): String("world"),
	}).(traits.Mapper)

	msg := &proto3pb.TestAllTypes{
		MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
			1: {},
			2: {},
		},
	}
	pbMsg := reg.NativeToValue(msg).(traits.Indexer)
	protoMap := pbMsg.Get(String("map_int64_nested_type")).(traits.Mapper)

	tests := []struct {
		value any
		out   Bool
	}{
		{value: 1, out: True},
		{value: 1.0, out: True},
		{value: uint(1), out: True},
		{value: 2, out: True},
		{value: 2.0, out: True},
		{value: uint(2), out: True},

		{value: 3, out: False},
		{value: 1.1, out: False},
		{value: 1.1 + math.MaxInt64, out: False},
		{value: 1.1 + math.MaxUint64, out: False},
		{value: "3", out: False},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			v := reg.NativeToValue(tc.value)
			if reflectMap.Contains(v).Equal(tc.out) != True {
				t.Errorf("reflectMap.Contains(%v) got %v, wanted %v", v, tc.out.Negate(), tc.out)
			}
			if refValMap.Contains(v).Equal(tc.out) != True {
				t.Errorf("refValMap.Contains(%v) got %v, wanted %v", v, tc.out.Negate(), tc.out)
			}
			if protoMap.Contains(v).Equal(tc.out) != True {
				t.Errorf("protoMap.Contains(%v) got %v, wanted %v", v, tc.out.Negate(), tc.out)
			}
		})
	}
}

func TestStringMapContains(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	if mapVal.Contains(String("first")) != True {
		t.Error("mapVal.Contains('first') did not return true")
	}
	if mapVal.Contains(String("third")) != False {
		t.Error("mapVal.Contains('third') did not return false")
	}
	if IsError(mapVal.Contains(Int(123))) {
		t.Error("mapVal.Contains(123) errored, wanted false'.")
	}
}

func TestDynamicMapConvertToNative_Any(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[string]float32{
		"nested": {"1": -1.0}})
	val, err := mapVal.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	jsonMap := &structpb.Struct{}
	err = protojson.Unmarshal([]byte(`{"nested":{"1":-1}}`), jsonMap)
	if err != nil {
		t.Fatalf("protojson.Unmarshal() failed: %v", err)
	}
	want, err := anypb.New(jsonMap)
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got %v, wanted %v", val, want)
	}
}

func TestDynamicMapConvertToNative_Error(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[string]float32{
		"nested": {"1": -1.0}})
	val, err := mapVal.ConvertToNative(reflect.TypeOf(""))
	if err == nil {
		t.Errorf("mapVal.ConvertToNative(string) got '%v', expected error", val)
	}
}

func TestDynamicMapConvertToNative_Json(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[string]float32{
		"nested": {"1": -1.0}})
	json, err := mapVal.ConvertToNative(JSONValueType)
	if err != nil {
		t.Error(err)
	}
	jsonBytes, err := protojson.Marshal(json.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal(%v) failed: %v", json, err)
	}
	jsonTxt := string(jsonBytes)
	if jsonTxt != `{"nested":{"1":-1}}` {
		t.Error(jsonTxt)
	}
}

func TestDynamicMapConvertToNative_Struct(t *testing.T) {
	reg := newTestRegistry(t)
	want := testStruct{M: "hello", Details: []string{"world", "universe"}, ExtraDetails: "extra, extra!"}
	tests := []map[string]any{
		{
			"m":             "hello",
			"details":       []string{"world", "universe"},
			"extra_Details": "extra, extra!",
		},
		{
			"M":             "hello",
			"Details":       []string{"world", "universe"},
			"extra_details": "extra, extra!",
		},
		{
			"M":                "hello",
			"Details":          []string{"world", "universe"},
			" extra___details": "extra, extra!",
		},
		{
			"M":                "hello",
			"Details_ ":        []string{"world", "universe"},
			" extra___details": "extra, extra!",
		},
		{
			"_m":               "hello",
			"Details_ ":        []string{"world", "universe"},
			" extra___details": "extra, extra!",
		},
		{
			"_M":               "hello",
			"Details_ ":        []string{"world", "universe"},
			" extra___details": "extra, extra!",
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			mapVal := NewDynamicMap(reg, tc)
			ts, err := mapVal.ConvertToNative(reflect.TypeFor[testStruct]())
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(ts, want) {
				t.Errorf("mapVal.ConvertToNative(struct) got %v, wanted %v", ts, want)
			}
		})
	}
}

func TestDynamicMapConvertToNative_StructPtr(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]any{
		"m":       "hello",
		"details": []string{"world", "universe"},
	})
	ts, err := mapVal.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err != nil {
		t.Error(err)
	}
	want := &testStruct{M: "hello", Details: []string{"world", "universe"}}
	if !reflect.DeepEqual(ts, want) {
		t.Errorf("mapVal.ConvertToNative(struct) got %v, wanted %v", ts, want)
	}
}

func TestDynamicMapConvertToNative_StructPtrPtr(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]any{
		"m":       "hello",
		"details": []string{"world", "universe"},
	})
	ptr := &testStruct{}
	ts, err := mapVal.ConvertToNative(reflect.TypeOf(&ptr))
	if err == nil {
		t.Errorf("Got %v, wanted error", ts)
	}
}

func TestDynamicMapConvertToNative_Struct_InvalidFieldError(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]any{
		"m":       "hello",
		"details": []string{"world", "universe"},
		"invalid": "invalid field",
	})
	ts, err := mapVal.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err == nil {
		t.Errorf("mapVal.ConvertToNative(struct) got %v, wanted error", ts)
	}
}

func TestDynamicMapConvertToNative_Struct_EmptyFieldError(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]any{
		"m":       "hello",
		"details": []string{"world", "universe"},
		"":        "empty field",
	})
	ts, err := mapVal.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err == nil {
		t.Errorf("mapVal.ConvertToNative(struct) got %v, wanted error", ts)
	}
}

func TestDynamicMapConvertToNative_Struct_PrivateFieldError(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]any{
		"message": "hello",
		"details": []string{"world", "universe"},
		"private": "private field",
	})
	ts, err := mapVal.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err == nil {
		t.Errorf("mapVal.ConvertToNative(struct) got %v, wanted error", ts)
	}
}

func TestStringMapConvertToNative(t *testing.T) {
	reg := newTestRegistry(t)
	strMap := map[string]string{
		"first":  "hello",
		"second": "world",
	}
	mapVal := NewStringStringMap(reg, strMap)
	val, err := mapVal.ConvertToNative(reflect.TypeOf(strMap))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative(map[string]string) failed: %v", err)
	}
	if !reflect.DeepEqual(val.(map[string]string), strMap) {
		t.Errorf("got not-equal, wanted equal for %v == %v", val, strMap)
	}
	val, err = mapVal.ConvertToNative(reflect.TypeOf(mapVal))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative(baseMap) failed: %v", err)
	}
	if !reflect.DeepEqual(val, mapVal) {
		t.Errorf("got not-equal, wanted equal for %v == %v", val, mapVal)
	}
	jsonVal, err := mapVal.ConvertToNative(JSONStructType)
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative(jsonStructType) failed: %v", err)
	}
	jsonBytes, err := protojson.Marshal(jsonVal.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal() failed: %v", err)
	}
	jsonTxt := string(jsonBytes)
	outMap := map[string]any{}
	err = json.Unmarshal(jsonBytes, &outMap)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", jsonTxt, err)
	}
	if !reflect.DeepEqual(outMap, map[string]any{
		"first":  "hello",
		"second": "world",
	}) {
		t.Errorf("got json '%v', expected %v", jsonTxt, outMap)
	}
}

func TestDynamicMapConvertToType(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]string{"key": "value"})
	if mapVal.ConvertToType(MapType) != mapVal {
		t.Error("mapVal.ConvertToType(MapType) could not be converted to a map.")
	}
	if mapVal.ConvertToType(TypeType) != MapType {
		t.Error("mapVal.ConvertToType(TypeType) did not return a map type.")
	}
	if !IsError(mapVal.ConvertToType(ListType)) {
		t.Error("mapVal.ConvertToType(ListType) returned a non-error.")
	}
}

func TestStringMapConvertToType(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := reg.NativeToValue(map[string]string{"key": "value"})
	if mapVal.ConvertToType(MapType) != mapVal {
		t.Error("mapVal.ConvertToType(MapType) could not be converted to a map.")
	}
	if mapVal.ConvertToType(TypeType) != MapType {
		t.Error("mapVal.ConvertToType(TypeType) did not return the map type.")
	}
	if !IsError(mapVal.ConvertToType(ListType)) {
		t.Error("mapVal.ConvertToType(ListType) did not error.")
	}
}

func TestDynamicMapEqual_True(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}})
	if mapVal.Equal(mapVal) != True {
		t.Error("mapVal.Equal(mapVal) did not return true")
	}

	if nestedVal := mapVal.Get(String("nested")); IsError(nestedVal) {
		t.Error(nestedVal)
	} else if mapVal.Equal(nestedVal) == True ||
		nestedVal.Equal(mapVal) == True {
		t.Error("Same length, but different key names did not result in error")
	}
}

func TestStringMapEqual_True(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	if mapVal.Equal(mapVal) != True {
		t.Error("mapVal.Equal(mapVal) did not return true")
	}
	equivDyn := NewDynamicMap(reg, map[string]string{
		"second": "world",
		"first":  "hello"})
	if mapVal.Equal(equivDyn) != True {
		t.Error("mapVal.Equal(equivDyn) did not return true, and was key-order dependent")
	}
	equivJSON := NewJSONStruct(reg, &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"first":  structpb.NewStringValue("hello"),
			"second": structpb.NewStringValue("world"),
		}})
	if mapVal.Equal(equivJSON) != True && equivJSON.Equal(mapVal) != True {
		t.Error("mapVal.Equal(equivJSON) did not return true")
	}
}

func TestDynamicMapEqual_NotTrue(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}})
	other := NewDynamicMap(reg, map[string]map[int64]float64{
		"nested": {1: -1.0, 2: 2.0, 3: 3.14},
		"empty":  {}})
	if mapVal.Equal(other) != False {
		t.Error("mapVal.Equal(other) did not return false.")
	}
	other = NewDynamicMap(reg, map[string]map[int64]float64{
		"nested": {1: -1.0, 2: 2.0, 3: 3.14},
		"absent": {}})
	if mapVal.Equal(other) != False {
		t.Error("mapVal.Equal(other) did not return false.")
	}
	if mapVal.Equal(NullValue) != False {
		t.Errorf("mapVal.Equal(NullValue) returned %v, wanted false", mapVal.Equal(NullValue))
	}
}

func TestStringMapEqual_NotTrue(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	if mapVal.Equal(mapVal) != True {
		t.Error("mapVal.Equal(mapVal) did not return true")
	}
	other := NewStringStringMap(reg, map[string]string{
		"second": "world",
		"first":  "goodbye"})
	if mapVal.Equal(other) != False {
		t.Error("mapVal.Equal(other) with same keys and different values did not return false")
	}
	other = NewStringStringMap(reg, map[string]string{
		"first": "hello"})
	if mapVal.Equal(other) != False {
		t.Error("mapVal.Equal(other) between maps of different size did not return false")
	}
	other = NewStringStringMap(reg, map[string]string{
		"first": "hello",
		"third": "goodbye"})
	if mapVal.Equal(other) != False {
		t.Error("mapVal.Equal(other) between maps with different keys did not return false")
	}
	other = NewDynamicMap(reg, map[string]any{
		"first":  "hello",
		"second": 1})
	if IsError(mapVal.Equal(other)) {
		t.Error("mapVal.Equal(other) between maps with same keys and different value types errored, wanted 'false'")
	}
}

func TestDynamicMapGet(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}})
	nestedVal, ok := mapVal.Get(String("nested")).(traits.Mapper)
	if !ok {
		t.Fatalf("mapVal.Get('nested') got %v, wanted map value", mapVal.Get(String("nested")))
	}
	floatVal := nestedVal.(traits.Indexer).Get(Int(1))
	if floatVal.Equal(Double(-1.0)) != True {
		t.Errorf("nestedVal.Get(1) got %v, wanted -1.0", floatVal)
	}
	err := mapVal.Get(String("absent"))
	if !IsError(err) || err.(*Err).Error() != "no such key: absent" {
		t.Errorf("mapVal.Get('absent') got %v, wanted no such key: absent.", err)
	}
	err = nestedVal.Get(String("bad_key"))
	if !IsError(err) || err.(*Err).Error() != "no such key: bad_key" {
		t.Errorf("nestedVal.Get('bad_key') errored %v, wanted no such key: bad_key.", err)
	}
	empty, ok := mapVal.Get(String("empty")).(traits.Mapper)
	if !ok {
		t.Fatalf("mapVal.Get('empty') got %v, wanted empty map", mapVal.Get(String("empty")))
	}
	err = empty.Get(String("hello"))
	if !IsError(err) || err.(*Err).Error() != "no such key: hello" {
		t.Errorf("empty.Get('hello') got %v, wanted no such key: hello", err)
	}
	err = empty.Get(Double(-1.0))
	if !IsError(err) || err.(*Err).Error() != "no such key: -1" {
		t.Errorf("empty.Get(-1.0) got %v, wanted no such key: -1", err)
	}
}

func TestStringIfaceMapGet(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringInterfaceMap(reg, map[string]any{
		"nested": map[int32]float64{1: -1.0, 2: 2.0},
		"empty":  map[string]any{},
	})
	nestedVal, ok := mapVal.Get(String("nested")).(traits.Mapper)
	if !ok {
		t.Fatalf("mapVal.Get('nested') got %v, wanted map value", mapVal.Get(String("nested")))
	}
	floatVal := nestedVal.(traits.Indexer).Get(Int(1))
	if floatVal.Equal(Double(-1.0)) != True {
		t.Errorf("nestedVal.Get(1) got %v, wanted -1.0", floatVal)
	}
	err := mapVal.Get(String("absent"))
	if !IsError(err) || err.(*Err).Error() != "no such key: absent" {
		t.Errorf("mapVal.Get('absent') got %v, wanted no such key: absent.", err)
	}
	err = nestedVal.Get(String("bad_key"))
	if !IsError(err) || err.(*Err).Error() != "no such key: bad_key" {
		t.Errorf("nestedVal.Get('bad_key') got %v, no such key: bad_key", err)
	}
	empty, ok := mapVal.Get(String("empty")).(traits.Mapper)
	if !ok {
		t.Fatalf("mapVal.Get('empty') got %v, wanted empty map", mapVal.Get(String("empty")))
	}
	err = empty.Get(String("hello"))
	if !IsError(err) || err.(*Err).Error() != "no such key: hello" {
		t.Errorf("empty.Get('hello') got %v, wanted no such key: hello", err)
	}
	err = empty.Get(Double(-1.0))
	if !IsError(err) || err.(*Err).Error() != "no such key: -1" {
		t.Errorf("empty.Get(-1.0) got %v, wanted no such key: -1", err)
	}
}

func TestStringMapGet(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	val := mapVal.Get(String("first"))
	if val.Equal(String("hello")) != True {
		t.Errorf("mapVal.Get('first') '%v', wanted 'hello'", val)
	}
	if !IsError(mapVal.Get(Int(1))) {
		t.Error("mapVal.Get(1) got real value, wanted error")
	}
	if !IsError(mapVal.Get(String("third"))) {
		t.Error("mapVal.Get('third') got real value, wanted error")
	}
}

func TestRefValMapGet(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewRefValMap(reg, map[ref.Val]ref.Val{
		String("nested"): NewRefValMap(reg, map[ref.Val]ref.Val{
			Int(1): Double(-1.0), Int(2): Double(2.0),
		}),
		String("empty"): NewRefValMap(reg, map[ref.Val]ref.Val{}),
	})
	nestedVal, ok := mapVal.Get(String("nested")).(traits.Mapper)
	if !ok {
		t.Fatalf("mapVal.Get('nested') got %v, wanted map value", mapVal.Get(String("nested")))
	}
	floatVal := nestedVal.(traits.Indexer).Get(Int(1))
	if floatVal.Equal(Double(-1.0)) != True {
		t.Errorf("nestedVal.Get(1) got %v, wanted -1.0", floatVal)
	}
	err := mapVal.Get(String("absent"))
	if !IsError(err) || err.(*Err).Error() != "no such key: absent" {
		t.Errorf("mapVal.Get('absent') got %v, wanted no such key: absent.", err)
	}
	err = nestedVal.Get(String("bad_key"))
	if !IsError(err) || err.(*Err).Error() != "no such key: bad_key" {
		t.Errorf("nestedVal.Get('bad_key') got %v, wanted no such key: bad_key.", err)
	}
	empty, ok := mapVal.Get(String("empty")).(traits.Mapper)
	if !ok {
		t.Fatalf("mapVal.Get('empty') got %v, wanted empty map", mapVal.Get(String("empty")))
	}
	err = empty.Get(String("hello"))
	if !IsError(err) || err.(*Err).Error() != "no such key: hello" {
		t.Errorf("empty.Get('hello') got %v, wanted no such key: hello", err)
	}
	err = empty.Get(Double(-1.0))
	if !IsError(err) || err.(*Err).Error() != "no such key: -1" {
		t.Errorf("empty.Get(-1.0) got %v, wanted no such key: -1", err)
	}
}

func TestMapIsZeroValue(t *testing.T) {
	msg := &proto3pb.TestAllTypes{
		MapStringString: map[string]string{
			"hello": "world",
		},
	}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	obj := reg.NativeToValue(msg).(traits.Indexer)

	tests := []struct {
		val         any
		isZeroValue bool
	}{
		{
			val:         map[int]int{},
			isZeroValue: true,
		},
		{
			val:         map[string]any{},
			isZeroValue: true,
		},
		{
			val:         map[string]string{},
			isZeroValue: true,
		},
		{
			val:         map[ref.Val]ref.Val{},
			isZeroValue: true,
		},
		{
			val:         &structpb.Struct{},
			isZeroValue: true,
		},
		{
			val:         obj.Get(String("map_int64_nested_type")),
			isZeroValue: true,
		},
		{
			val:         map[int]int{1: 1},
			isZeroValue: false,
		},
		{
			val:         map[string]any{"hello": []any{}},
			isZeroValue: false,
		},
		{
			val:         map[string]string{"": ""},
			isZeroValue: false,
		},
		{
			val:         map[ref.Val]ref.Val{False: True},
			isZeroValue: false,
		},
		{
			val: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"": structpb.NewNullValue(),
				},
			},
			isZeroValue: false,
		},
		{
			val:         obj.Get(String("map_string_string")),
			isZeroValue: false,
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			v := DefaultTypeAdapter.NativeToValue(tc.val)
			zv, ok := v.(traits.Zeroer)
			if !ok {
				t.Fatalf("%v could not be converted to a zero-valuer type", tc.val)
			}
			if zv.IsZeroValue() != tc.isZeroValue {
				t.Errorf("%v.IsZeroValue() got %t, wanted %t", v, zv.IsZeroValue(), tc.isZeroValue)
			}
		})
	}
}

func TestDynamicMapIterator(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}})
	it := mapVal.Iterator()
	var i = 0
	var fieldNames []any
	for ; it.HasNext() == True; i++ {
		fieldName := it.Next()
		if value := mapVal.Get(fieldName); IsError(value) {
			t.Error(value)
		} else {
			fieldNames = append(fieldNames, fieldName)
		}
	}
	if len(fieldNames) != 2 {
		t.Errorf("Did not find the correct number of fields: %v", fieldNames)
	}
	if it.Next() != nil {
		t.Error("Iterator ran off the end of the field names")
	}
}

func TestStringMapIterator(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	it := mapVal.Iterator()
	var i = 0
	var fieldNames []any
	for ; it.HasNext() == True; i++ {
		fieldName := it.Next()
		if value := mapVal.Get(fieldName); IsError(value) {
			t.Error(value)
		} else {
			fieldNames = append(fieldNames, fieldName)
		}
	}
	if len(fieldNames) != 2 {
		t.Errorf("Did not find the correct number of fields: %v", fieldNames)
	}
	fieldsMap := map[string]bool{
		"first":  false,
		"second": false,
	}
	expectedMap := map[string]bool{
		"first":  true,
		"second": true,
	}
	for _, fieldName := range fieldNames {
		key := string(fieldName.(String))
		if _, found := fieldsMap[key]; found {
			fieldsMap[key] = true
		}
	}
	if !reflect.DeepEqual(fieldsMap, expectedMap) {
		t.Errorf("Got '%v', wanted '%v'", fieldsMap, expectedMap)
	}
	if it.Next() != nil {
		t.Error("Iterator ran off the end of the field names")
	}
}

func TestDynamicMapSize(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewDynamicMap(reg, map[string]int{
		"first":  1,
		"second": 2})
	if mapVal.Size() != Int(2) {
		t.Errorf("mapVal.Size() got '%v', expected 2", mapVal.Size())
	}
}

func TestStringMapSize(t *testing.T) {
	reg := newTestRegistry(t)
	mapVal := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	if mapVal.Size() != Int(2) {
		t.Errorf("mapVal.Size() got '%v', expected 2", mapVal.Size())
	}
}

func TestProtoMap(t *testing.T) {
	strMap := map[string]string{
		"hello":   "world",
		"goodbye": "for now",
		"welcome": "back",
	}
	msg := &proto3pb.TestAllTypes{MapStringString: strMap}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	obj := reg.NativeToValue(msg).(traits.Indexer)

	// Test a simple proto map of string string.
	field := obj.Get(String("map_string_string"))
	mapVal, ok := field.(traits.Mapper)
	if !ok {
		t.Fatalf("obj.Get('map_string_string') did not return map: (%T)%v", field, field)
	}
	// CEL type conversion tests.
	if mapVal.ConvertToType(MapType) != mapVal {
		t.Errorf("mapVal.ConvertToType(MapType) got %v, wanted map type", mapVal.ConvertToType(MapType))
	}
	if mapVal.ConvertToType(TypeType) != MapType {
		t.Errorf("mapVal.ConvertToType(TypeType) got %v, wanted type type", mapVal.ConvertToType(TypeType))
	}
	conv := mapVal.ConvertToType(ListType)
	if !IsError(conv) {
		t.Errorf("mapVal.ConvertToType(ListType) got %v, wanted error", conv)
	}
	// Size test
	if mapVal.Size() != Int(len(strMap)) {
		t.Errorf("mapVal.Size() got %d, wanted %d", mapVal.Size(), len(strMap))
	}
	// Contains, Find, and Get tests.
	for k, v := range strMap {
		if mapVal.Contains(reg.NativeToValue(k)) != True {
			t.Errorf("mapVal.Contains() missing key: %v", k)
		}
		kv := mapVal.Get(reg.NativeToValue(k))
		if kv.Equal(reg.NativeToValue(v)) != True {
			t.Errorf("mapVal.Get(%v) got value %v wanted %v", k, kv, v)
		}
	}
	// Equality test
	refStrMap := reg.NativeToValue(strMap)
	if refStrMap.Equal(mapVal) != True || mapVal.Equal(refStrMap) != True {
		t.Errorf("mapVal.Equal(refStrMap) not equal to itself: ref.Val %v != ref.Val %v", refStrMap, mapVal)
	}
	// Iterator test
	it := mapVal.Iterator()
	mapValCopy := map[ref.Val]ref.Val{}
	for it.HasNext() == True {
		key := it.Next()
		mapValCopy[key] = mapVal.Get(key)
	}
	mapVal2 := reg.NativeToValue(mapValCopy)
	if mapVal2.Equal(mapVal) != True || mapVal.Equal(mapVal2) != True {
		t.Errorf("mapVal.Equal(copy) not equal to original: cel ref.Val %v != ref.Val %v", mapVal2, mapVal)
	}
	if mapVal.Equal(NullValue) != False {
		t.Errorf("mapVal.Equal(NullValue) got %v, wanted false", mapVal.Equal(NullValue))
	}
	convMap, err := mapVal.ConvertToNative(reflect.TypeOf(strMap))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	if !reflect.DeepEqual(strMap, convMap) {
		t.Errorf("mapVal.ConvertToNative() got map %v, wanted %v", convMap, strMap)
	}
	// Inequality tests.
	strNeMap := map[string]string{
		"hello":   "world",
		"goodbye": "forever",
		"welcome": "back",
	}
	mapNeVal := reg.NativeToValue(strNeMap)
	if mapNeVal.Equal(mapVal) != False || mapVal.Equal(mapNeVal) != False {
		t.Error("mapNeVal.Equal(mapVal) returned true, wanted false")
	}
	strNeMap = map[string]string{
		"hello":   "world",
		"goodbe":  "for now",
		"welcome": "back",
	}
	mapNeVal = reg.NativeToValue(strNeMap)
	if mapNeVal.Equal(mapVal) != False || mapVal.Equal(mapNeVal) != False {
		t.Error("mapNeVal.Equal(mapVal) returned true, wanted false")
	}
	mapNeVal = reg.NativeToValue(map[string]string{})
	if mapNeVal.Equal(mapVal) != False || mapVal.Equal(mapNeVal) != False {
		t.Error("mapNeVal.Equal(mapVal) returned true, wanted false")
	}
	mapNeMap := map[int64]int64{
		1: 9,
		2: 1,
		3: 1,
	}
	mapNeVal = reg.NativeToValue(mapNeMap)
	if IsError(mapNeVal.Equal(mapVal)) || IsError(mapVal.Equal(mapNeVal)) {
		t.Error("mapNeVal.Equal(mapVal) returned error, wanted false")
	}
}

func TestProtoMapGet(t *testing.T) {
	strMap := map[string]string{
		"hello":   "world",
		"goodbye": "for now",
		"welcome": "back",
	}
	msg := &proto3pb.TestAllTypes{MapStringString: strMap}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	obj := reg.NativeToValue(msg).(traits.Indexer)
	field := obj.Get(String("map_string_string"))
	mapVal, ok := field.(traits.Mapper)
	if !ok {
		t.Fatalf("obj.Get(map_string_string) failed: %v", field)
	}
	v := mapVal.Get(String("hello"))
	if v.Equal(String("world")) == False {
		t.Errorf("mapVal.Get('hello') got %v, wanted 'world'", v)
	}
	notFound := mapVal.Get(String("not_found"))
	if !IsError(notFound) || !strings.Contains(notFound.(*Err).Error(), "no such key") {
		t.Errorf("mapVal.Get('not_found') got %v, wanted no such key error", notFound)
	}
	badKey := mapVal.Get(Int(42))
	if !IsError(badKey) || !strings.Contains(badKey.(*Err).Error(), "no such key: 42") {
		t.Errorf("mapVal.Get(42) got %v, wanted no such key: 42", badKey)
	}
}

func TestProtoMapString(t *testing.T) {
	strMap := map[string]string{
		"hello": "world",
	}
	reg := newTestRegistry(t)
	m := reg.NativeToValue(strMap)
	want := `{hello: world}`
	if fmt.Sprintf("%v", m) != want {
		t.Errorf("map.String() got %v, wanted %v", m, want)
	}
}

func TestProtoMapConvertToNative(t *testing.T) {
	strMap := map[string]string{
		"hello":   "world",
		"goodbye": "for now",
		"welcome": "back",
	}
	msg := &proto3pb.TestAllTypes{MapStringString: strMap}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	obj := reg.NativeToValue(msg).(traits.Indexer)
	// Test a simple proto map of string string.
	field := obj.Get(String("map_string_string"))
	mapVal, ok := field.(traits.Mapper)
	if !ok {
		t.Fatalf("obj.Get('map_string_string') did not return map: (%T)%v", field, field)
	}
	convMap, err := mapVal.ConvertToNative(reflect.TypeOf(map[string]any{}))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	for k, v := range convMap.(map[string]any) {
		if strMap[k] != v {
			t.Errorf("got differing values for key %q: got %v, wanted: %v", k, strMap[k], v)
		}
	}
	mapVal2 := reg.NativeToValue(convMap)
	if mapVal2.Equal(mapVal) != True || mapVal.Equal(mapVal2) != True {
		t.Errorf("mapVal2.Equal(mapVal) returned false, wanted true")
	}
	convMap, err = mapVal.ConvertToNative(anyValueType)
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	mapVal3 := reg.NativeToValue(convMap)
	if mapVal3.Equal(mapVal) != True || mapVal.Equal(mapVal3) != True {
		t.Errorf("mapVal3.Equal(mapVal) returned false, wanted true")
	}
	convMap, err = mapVal.ConvertToNative(JSONValueType)
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	mapVal4 := reg.NativeToValue(convMap)
	if mapVal4.Equal(mapVal) != True || mapVal.Equal(mapVal4) != True {
		t.Errorf("mapVal4.Equal(mapVal) returned false, wanted true")
	}
	convMap, err = mapVal.ConvertToNative(reflect.TypeOf(&pb.Map{}))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	mapVal5 := reg.NativeToValue(convMap)
	if mapVal5.Equal(mapVal) != True || mapVal.Equal(mapVal5) != True {
		t.Errorf("mapVal5.Equal(mapVal) returned false, wanted true")
	}
	var mapper traits.Mapper = mapVal
	convMap, err = mapVal.ConvertToNative(reflect.TypeOf(mapper))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	mapVal6 := reg.NativeToValue(convMap)
	if mapVal6.Equal(mapVal) != True || mapVal.Equal(mapVal6) != True {
		t.Errorf("mapVal6.Equal(mapVal) returned false, wanted true")
	}
	_, err = mapVal.ConvertToNative(JSONListType)
	if err == nil {
		t.Fatalf("mapVal.ConvertToNative() succeeded for invalid type")
	}
	_, err = mapVal.ConvertToNative(reflect.TypeOf(map[int32]string{}))
	if err == nil {
		t.Fatalf("mapVal.ConvertToNative() succeeded for invalid type")
	}
	_, err = mapVal.ConvertToNative(reflect.TypeOf(map[string]int64{}))
	if err == nil {
		t.Fatalf("mapVal.ConvertToNative() succeeded for invalid type")
	}
}

func TestProtoMapConvertToNative_NestedProto(t *testing.T) {
	nestedTypeMap := map[int64]*proto3pb.NestedTestAllTypes{
		1: {
			Payload: &proto3pb.TestAllTypes{
				SingleBoolWrapper: wrapperspb.Bool(true),
			},
		},
		2: {
			Child: &proto3pb.NestedTestAllTypes{
				Payload: &proto3pb.TestAllTypes{
					SingleTimestamp: tpb.New(time.Now()),
				},
			},
		},
	}
	msg := &proto3pb.TestAllTypes{MapInt64NestedType: nestedTypeMap}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	obj := reg.NativeToValue(msg).(traits.Indexer)
	// Test a simple proto map of string string.
	field := obj.Get(String("map_int64_nested_type"))
	mapVal, ok := field.(traits.Mapper)
	if !ok {
		t.Fatalf("obj.Get('map_int64_nested_type') did not return map: (%T)%v", field, field)
	}
	convMap, err := mapVal.ConvertToNative(reflect.TypeOf(map[int32]any{}))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	for k, v := range convMap.(map[int32]any) {
		if !proto.Equal(nestedTypeMap[int64(k)], v.(proto.Message)) {
			t.Errorf("got differing values for key %q: got %v, wanted: %v", k, nestedTypeMap[int64(k)], v)
		}
	}
	convMap, err = mapVal.ConvertToNative(reflect.TypeOf(map[int32]proto.Message{}))
	if err != nil {
		t.Fatalf("mapVal.ConvertToNative() failed: %v", err)
	}
	for k, v := range convMap.(map[int32]proto.Message) {
		if !proto.Equal(nestedTypeMap[int64(k)], v) {
			t.Errorf("got differing values for key %q: got %v, wanted: %v", k, nestedTypeMap[int64(k)], v)
		}
	}
}

func TestMutableMap(t *testing.T) {
	m := NewMutableMap(
		DefaultTypeAdapter,
		map[ref.Val]ref.Val{String("hello"): String("world")})
	m.Insert(String("goodbye"), String("cruel world"))
	im := m.ToImmutableMap()
	if im.Size() != Int(2) {
		t.Errorf("m.ToImmutableMap() had size %d, wanted 2", im.Size())
	}
	if !IsError(m.Insert(String("goodbye"), String("happy world"))) {
		t.Error("m.Insert('goodbye', 'happy world') suceeded, wanted error")
	}
	m.Insert(String("well"), String("well"))
	if im.Size() != Int(2) {
		t.Errorf("m.Insert() mutated storage for immutable map: had size %d, wanted 2", im.Size())
	}
}

func TestMapFold(t *testing.T) {
	pbDB := pb.NewDb()
	fd, err := pbDB.RegisterMessage(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("pbdb.RegisterMessage(TestAllTypes) failed: %v", err)
	}
	td, found := fd.GetTypeDescription(string((&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor().FullName()))
	if !found {
		t.Fatal("fd.GetTypeDescription() failed")
	}
	mapStrStrFD, found := td.FieldByName("map_string_string")
	if !found {
		t.Fatal("Could not find map_string_string field")
	}

	mapStrDesc := (&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor().Fields().ByName("map_string_string")
	tests := []struct {
		m         any
		folds     int
		foldLimit int
	}{
		{
			m:         map[string]any{"a": 1, "b": 2},
			folds:     2,
			foldLimit: 2,
		},
		{
			m:         map[string]string{"hello": "world"},
			folds:     1,
			foldLimit: 2,
		},
		{
			m:         map[string]string{"hello": "world", "goodbye": "cruel world"},
			folds:     1,
			foldLimit: 1,
		},
		{
			m:         map[ref.Val]ref.Val{},
			folds:     0,
			foldLimit: 20,
		},
		{
			m: map[ref.Val]ref.Val{
				(String("hello")):   String("world"),
				(String("goodbye")): String("cruel world"),
			},
			folds:     1,
			foldLimit: 1,
		},
		{
			m: testCreateStruct(t, map[string]any{
				"hello": []any{},
				"world": map[string]any{},
			}),
			folds:     2,
			foldLimit: 2,
		},
		{
			m: testCreateStruct(t, map[string]any{
				"hello": []any{},
				"world": map[string]any{},
			}),
			folds:     1,
			foldLimit: 1,
		},
		{
			m: (&proto3pb.TestAllTypes{
				MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
					1: {},
					2: {},
					3: {},
				},
			}).GetMapInt64NestedType(),
			folds:     3,
			foldLimit: 3,
		},
		{
			m: (&proto3pb.TestAllTypes{
				MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
					1: {},
					2: {},
					3: {},
				},
			}).GetMapInt64NestedType(),
			folds:     2,
			foldLimit: 2,
		},
		{
			m: &pb.Map{
				Map: (&proto3pb.TestAllTypes{
					MapStringString: map[string]string{
						"1": "one",
						"2": "two",
					},
				}).ProtoReflect().Get(mapStrDesc).Map(),
				KeyType:   mapStrStrFD.KeyType,
				ValueType: mapStrStrFD.ValueType,
			},
			folds:     1,
			foldLimit: 1,
		},
	}
	reg := NewEmptyRegistry()
	for i, tst := range tests {
		tc := tst
		m := reg.NativeToValue(tc.m).(traits.Mapper)
		foldKinds := map[string]traits.Foldable{
			"modern": ToFoldableMap(m),
			"legacy": ToFoldableMap(proxyLegacyMap{proxy: m}),
		}
		for foldKind, foldable := range foldKinds {
			t.Run(fmt.Sprintf("[%d]%s", i, foldKind), func(t *testing.T) {
				f := &testMapFolder{foldLimit: tc.foldLimit}
				foldable.Fold(f)
				if f.folds != tc.folds {
					t.Errorf("m.Fold(f) got %d, wanted %d folds", f.folds, tc.folds)
				}
			})
		}
	}
}

func TestInsertMapKeyValue_MutableMapper(t *testing.T) {
	m := NewMutableMap(DefaultTypeAdapter, map[ref.Val]ref.Val{String("first"): Int(1)})
	modified := InsertMapKeyValue(m, String("second"), Int(2))
	if IsError(modified) {
		t.Fatalf("InsertMapKeyValue() got error: %v, wanted insertion", modified)
	}
	if modified != m {
		t.Fatalf("InsertMapKeyValue() created a new map for a mutable input: %v", modified)
	}
	im := m.ToImmutableMap()
	if _, found := im.Find(String("first")); !found {
		t.Errorf("InsertMapKeyValue() did not preserve entry 'first': %v", im)
	}
	if _, found := im.Find(String("second")); !found {
		t.Errorf("InsertMapKeyValue() did not insert entry 'second': %v", im)
	}
	if !IsError(InsertMapKeyValue(m, String("second"), Int(3))) {
		t.Errorf("InsertMapKeyValue('second', 3) modified the map instead of erroring: %v", m)
	}
}

func TestInsertMapKeyValue_Mapper(t *testing.T) {
	m := NewRefValMap(DefaultTypeAdapter, map[ref.Val]ref.Val{String("first"): Int(1)})
	modified := InsertMapKeyValue(m, String("second"), Int(2))
	if IsError(modified) {
		t.Fatalf("InsertMapKeyValue() got error: %v, wanted insertion", modified)
	}
	if modified == m {
		t.Fatalf("InsertMapKeyValue() modified an immutable input: %v", modified)
	}
	im := modified.(traits.Mapper)
	if _, found := im.Find(String("first")); !found {
		t.Errorf("InsertMapKeyValue() did not preserve entry 'first': %v", im)
	}
	if _, found := im.Find(String("second")); !found {
		t.Errorf("InsertMapKeyValue() did not insert entry 'second': %v", im)
	}
	if !IsError(InsertMapKeyValue(im, String("second"), Int(3))) {
		t.Errorf("InsertMapKeyValue('second', 3) modified the map instead of erroring: %v", m)
	}
}

type testMapFolder struct {
	foldLimit int
	folds     int
}

func (f *testMapFolder) FoldEntry(k, v any) bool {
	if f.foldLimit != 0 {
		if f.folds >= f.foldLimit {
			return false
		}
	}
	f.folds++
	return true
}

func testCreateStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	v, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct(m) failed: %v", err)
	}
	return v
}

// proxyLegacyMap omits the foldable interfaces associated with all core Mapper implementations
type proxyLegacyMap struct {
	proxy traits.Mapper
}

func (m proxyLegacyMap) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return m.proxy.ConvertToNative(typeDesc)
}

func (m proxyLegacyMap) ConvertToType(typeValue ref.Type) ref.Val {
	return m.proxy.ConvertToType(typeValue)
}

func (m proxyLegacyMap) Equal(other ref.Val) ref.Val {
	return m.proxy.Equal(other)
}

func (m proxyLegacyMap) Type() ref.Type {
	return m.proxy.Type()
}

func (m proxyLegacyMap) Value() any {
	return m.proxy.Value()
}

func (m proxyLegacyMap) Contains(value ref.Val) ref.Val {
	return m.proxy.Contains(value)
}

func (m proxyLegacyMap) Find(key ref.Val) (ref.Val, bool) {
	return m.proxy.Find(key)
}

func (m proxyLegacyMap) Get(index ref.Val) ref.Val {
	return m.proxy.Get(index)
}

func (m proxyLegacyMap) Iterator() traits.Iterator {
	return m.proxy.Iterator()
}

func (m proxyLegacyMap) Size() ref.Val {
	return m.proxy.Size()
}

func TestMapCalculateSize(t *testing.T) {
	adapter := DefaultTypeAdapter

	// Setup helper data
	l := NewRefValList(adapter, []ref.Val{Int(2), Int(3)})
	refValMap := NewRefValMap(adapter, map[ref.Val]ref.Val{
		String("a"): Int(1),
		String("b"): l,
	})

	ifaceMap := NewStringInterfaceMap(adapter, map[string]any{
		"a": int64(1),
		"b": []any{int64(2), int64(3)},
	})

	mutMap := NewMutableMap(adapter, map[ref.Val]ref.Val{
		String("a"): Int(1),
		String("b"): l,
	})
	// Initial evaluation before insert to test aggSize reset
	_ = mutMap.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())
	mutMap.Insert(String("c"), Int(4))

	reg, err := NewRegistry(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	msg := &proto3pb.TestAllTypes{
		MapStringString: map[string]string{
			"a": "b",
			"c": "d",
		},
	}
	pbMsg := reg.NativeToValue(msg).(traits.Indexer)
	pm := pbMsg.Get(String("map_string_string")).(traits.Mapper)

	tests := []struct {
		name string
		val  ref.Val
		want uint32
	}{
		{
			name: "empty_ref_val_map",
			val:  NewRefValMap(adapter, map[ref.Val]ref.Val{}),
			want: 1,
		},
		{
			name: "ref_val_map_nested",
			val:  refValMap,
			want: 7,
		},
		{
			name: "string_interface_map",
			val:  ifaceMap,
			want: 7,
		},
		{
			name: "string_string_map",
			val:  NewStringStringMap(adapter, map[string]string{"k1": "v1", "k2": "v2"}),
			want: 5, // 1 (container) + 4 (single-unit keys and values) = 5
		},
		{
			name: "mutable_map_after_insert",
			val:  mutMap,
			want: 9,
		},
		{
			name: "proto_map",
			val:  pm,
			want: 5,
		},
		{
			name: "nil_proto_map",
			val:  &protoMap{},
			want: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sizer, ok := tc.val.(AggregateSizeVisitor)
			if !ok {
				t.Fatalf("expected AggregateSizeVisitor implementation for %T", tc.val)
			}
			if got := sizer.AggregateSize(NewSizeCalculator()); got != tc.want {
				t.Errorf("got aggregate size %d, want %d", got, tc.want)
			}
			// Caching check (memoized aggSize)
			if got := sizer.AggregateSize(NewSizeCalculator()); got != tc.want {
				t.Errorf("memoized AggregateSize() got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNativeMapFindFastKeys(t *testing.T) {
	adapter := DefaultTypeAdapter

	type stringKeyFinder interface {
		FindStringKey(string) (any, bool)
	}
	type int64KeyFinder interface {
		FindInt64Key(int64) (any, bool)
	}
	type nativeFinder interface {
		FindNative(any) (any, bool)
	}

	strMap := NewMap(adapter, map[string]string{"foo": "bar"})
	refValStrMap := NewRefValMap(adapter, map[ref.Val]ref.Val{String("hello"): String("world")})
	int64Map := NewMap(adapter, map[int64]int64{42: 100})
	intMap := NewMap(adapter, map[int]string{10: "ten"})
	int32Map := NewMap(adapter, map[int32]string{5: "five"})
	refValIntMap := NewRefValMap(adapter, map[ref.Val]ref.Val{
		Int(0):   Int(10),
		Uint(50): String("fifty"),
	})

	stringTests := []struct {
		name      string
		mapper    any
		key       string
		wantVal   any
		wantFound bool
	}{
		{name: "string map hit", mapper: strMap, key: "foo", wantVal: String("bar"), wantFound: true},
		{name: "string map miss", mapper: strMap, key: "missing", wantVal: nil, wantFound: false},
		{name: "ref.Val string map hit", mapper: refValStrMap, key: "hello", wantVal: String("world"), wantFound: true},
		{name: "ref.Val string map miss", mapper: refValStrMap, key: "missing", wantVal: nil, wantFound: false},
		{name: "int64 map wrong type", mapper: int64Map, key: "foo", wantVal: nil, wantFound: false},
	}
	for _, tc := range stringTests {
		t.Run(tc.name, func(t *testing.T) {
			finder, ok := tc.mapper.(stringKeyFinder)
			if !ok {
				t.Fatalf("mapper does not implement stringKeyFinder")
			}
			val, found := finder.FindStringKey(tc.key)
			if found != tc.wantFound || val != tc.wantVal {
				t.Errorf("FindStringKey(%q) got %v, %v, want %v, %v", tc.key, val, found, tc.wantVal, tc.wantFound)
			}
		})
	}

	int64Tests := []struct {
		name      string
		mapper    any
		key       int64
		wantVal   any
		wantFound bool
	}{
		{name: "int64 map hit", mapper: int64Map, key: 42, wantVal: Int(100), wantFound: true},
		{name: "int64 map miss", mapper: int64Map, key: 999, wantVal: nil, wantFound: false},
		{name: "int map hit", mapper: intMap, key: 10, wantVal: String("ten"), wantFound: true},
		{name: "int map miss", mapper: intMap, key: 99, wantVal: nil, wantFound: false},
		{name: "int map underflow", mapper: intMap, key: math.MinInt64, wantVal: nil, wantFound: false},
		{name: "int map overflow", mapper: intMap, key: math.MaxInt64, wantVal: nil, wantFound: false},
		{name: "int32 map hit", mapper: int32Map, key: 5, wantVal: String("five"), wantFound: true},
		{name: "int32 map miss", mapper: int32Map, key: 99, wantVal: nil, wantFound: false},
		{name: "int32 map underflow", mapper: int32Map, key: math.MinInt64, wantVal: nil, wantFound: false},
		{name: "int32 map overflow", mapper: int32Map, key: math.MaxInt64, wantVal: nil, wantFound: false},
		{name: "ref.Val map int hit", mapper: refValIntMap, key: 0, wantVal: Int(10), wantFound: true},
		{name: "ref.Val map uint hit", mapper: refValIntMap, key: 50, wantVal: String("fifty"), wantFound: true},
		{name: "ref.Val map miss", mapper: refValIntMap, key: 999, wantVal: nil, wantFound: false},
		{name: "string map wrong type", mapper: strMap, key: 42, wantVal: nil, wantFound: false},
	}
	for _, tc := range int64Tests {
		t.Run(tc.name, func(t *testing.T) {
			finder, ok := tc.mapper.(int64KeyFinder)
			if !ok {
				t.Fatalf("mapper does not implement int64KeyFinder")
			}
			val, found := finder.FindInt64Key(tc.key)
			if found != tc.wantFound || val != tc.wantVal {
				t.Errorf("FindInt64Key(%d) got %v, %v, want %v, %v", tc.key, val, found, tc.wantVal, tc.wantFound)
			}
		})
	}

	nativeTests := []struct {
		name      string
		mapper    any
		key       any
		wantVal   any
		wantFound bool
	}{
		{name: "native string key", mapper: strMap, key: "foo", wantVal: String("bar"), wantFound: true},
		{name: "native String key", mapper: strMap, key: String("foo"), wantVal: String("bar"), wantFound: true},
		{name: "native Int key", mapper: int64Map, key: Int(42), wantVal: Int(100), wantFound: true},
		{name: "native int64 key", mapper: int64Map, key: int64(42), wantVal: Int(100), wantFound: true},
		{name: "native int key", mapper: intMap, key: int(10), wantVal: String("ten"), wantFound: true},
		{name: "native ref.Val string key", mapper: refValStrMap, key: String("hello"), wantVal: String("world"), wantFound: true},
		{name: "native ref.Val double key", mapper: int64Map, key: Double(42.0), wantVal: Int(100), wantFound: true},
		{name: "native unsupported key type", mapper: strMap, key: true, wantVal: nil, wantFound: false},
	}
	for _, tc := range nativeTests {
		t.Run(tc.name, func(t *testing.T) {
			finder, ok := tc.mapper.(nativeFinder)
			if !ok {
				t.Fatalf("mapper does not implement nativeFinder")
			}
			val, found := finder.FindNative(tc.key)
			if found != tc.wantFound || val != tc.wantVal {
				t.Errorf("FindNative(%v) got %v, %v, want %v, %v", tc.key, val, found, tc.wantVal, tc.wantFound)
			}
		})
	}
}

type testFoldKeyOnly struct {
	keys []any
}

func (f *testFoldKeyOnly) FoldKeyOnly() bool {
	return true
}

func (f *testFoldKeyOnly) FoldEntry(key, val any) bool {
	f.keys = append(f.keys, key)
	return true
}

type testEarlyStopFolder struct {
	count int
	limit int
}

func (f *testEarlyStopFolder) FoldEntry(key, val any) bool {
	f.count++
	return f.count < f.limit
}

func TestBaseMapFullCoverage(t *testing.T) {
	adapter := DefaultTypeAdapter
	st := testCreateStruct(t, map[string]any{
		"k1": "v1",
		"k2": "v2",
	})
	bm := NewJSONStruct(adapter, st)

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "type and value",
			test: func(t *testing.T) {
				if bm.Type() != MapType {
					t.Errorf("Type() got %v, want MapType", bm.Type())
				}
				if bm.Value() != st {
					t.Errorf("Value() got %v, want st", bm.Value())
				}
				if bm.(traits.Zeroer).IsZeroValue() {
					t.Errorf("IsZeroValue() got true, want false")
				}
				if bm.Size() != Int(2) {
					t.Errorf("Size() got %v, want 2", bm.Size())
				}
			},
		},
		{
			name: "string representation",
			test: func(t *testing.T) {
				strRep := bm.(fmt.Stringer).String()
				if !strings.Contains(strRep, "k1") || !strings.Contains(strRep, "v1") {
					t.Errorf("String() got %v, expected k1 and v1", strRep)
				}
			},
		},
		{
			name: "format",
			test: func(t *testing.T) {
				formatted := Format(bm)
				if formatted != `{"k1": "v1", "k2": "v2"}` && formatted != `{"k2": "v2", "k1": "v1"}` {
					t.Errorf("Format(bm) got %v", formatted)
				}
			},
		},
		{
			name: "aggregate size uncached",
			test: func(t *testing.T) {
				sizer := NewSizeCalculator()
				if sz := bm.(AggregateSizeVisitor).AggregateSize(sizer); sz == 0 {
					t.Errorf("AggregateSize() got 0")
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.test)
	}
}

func TestNativeMapAllSpecializedTypes(t *testing.T) {
	adapter := DefaultTypeAdapter
	now := time.Now()

	type customStruct struct {
		Name string
	}

	tests := []struct {
		name   string
		mapper traits.Mapper
	}{
		{name: "string_uint32", mapper: NewMap(adapter, map[string]uint32{"a": 1})},
		{name: "string_uint64", mapper: NewMap(adapter, map[string]uint64{"a": 1})},
		{name: "string_uint", mapper: NewMap(adapter, map[string]uint{"a": 1})},
		{name: "string_float32", mapper: NewMap(adapter, map[string]float32{"a": 1.5})},
		{name: "string_float64", mapper: NewMap(adapter, map[string]float64{"a": 1.5})},
		{name: "string_bool", mapper: NewMap(adapter, map[string]bool{"a": true, "b": false})},
		{name: "string_bytes", mapper: NewMap(adapter, map[string][]byte{"a": []byte("bytes")})},
		{name: "uint_string", mapper: NewMap(adapter, map[uint]string{1: "u"})},
		{name: "uint32_string", mapper: NewMap(adapter, map[uint32]string{2: "u32"})},
		{name: "uint64_string", mapper: NewMap(adapter, map[uint64]string{3: "u64"})},
		{name: "bool_string", mapper: NewMap(adapter, map[bool]string{true: "t", false: "f"})},
		{name: "string_customStruct", mapper: NewMap(adapter, map[string]customStruct{"s": {Name: "cel"}})},
		{name: "time_string", mapper: NewMap(adapter, map[time.Time]string{now: "timeKey"})},
		{name: "int64_int64", mapper: NewMap(adapter, map[int64]int64{100: 200})},
		{name: "int_int", mapper: NewMap(adapter, map[int]int{10: 20})},
		{name: "int32_int32", mapper: NewMap(adapter, map[int32]int32{1: 2})},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.mapper
			if m.Type() != MapType {
				t.Errorf("Type() got %v, want MapType", m.Type())
			}
			if m.Value() == nil {
				t.Errorf("Value() got nil")
			}
			if m.Size() == IntZero {
				t.Errorf("Size() got 0")
			}
			if m.(traits.Zeroer).IsZeroValue() {
				t.Errorf("IsZeroValue() got true")
			}
			if str := m.(fmt.Stringer).String(); str == "" {
				t.Errorf("String() returned empty string")
			}
			if fmtStr := Format(m); fmtStr == "" {
				t.Errorf("Format() returned empty string")
			}
			if sz := m.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator()); sz == 0 {
				t.Errorf("AggregateSize() returned 0")
			}

			// Iterator past end
			it := m.Iterator()
			for it.HasNext() == True {
				_ = it.Next()
			}
			if it.HasNext() != False {
				t.Errorf("expected HasNext == False past end")
			}
			if it.Next() != nil {
				t.Errorf("expected Next == nil past end")
			}

			// FoldKeyOnly
			fko := &testFoldKeyOnly{}
			m.(traits.Foldable).Fold(fko)
			if len(fko.keys) == 0 {
				t.Errorf("FoldKeyOnly returned 0 keys")
			}

			// Fold early stop
			fes := &testEarlyStopFolder{limit: 1}
			m.(traits.Foldable).Fold(fes)
		})
	}
}

func TestNativeMapFindBranches(t *testing.T) {
	adapter := DefaultTypeAdapter
	now := time.Now()

	mStr := NewMap(adapter, map[string]uint32{"a": 1})
	mInt64 := NewMap(adapter, map[int64]string{10: "ten"})
	mInt := NewMap(adapter, map[int]string{20: "twenty"})
	mInt32 := NewMap(adapter, map[int32]string{30: "thirty"})
	mUint64 := NewMap(adapter, map[uint64]string{40: "forty"})
	mUint := NewMap(adapter, map[uint]string{50: "fifty"})
	mUint32 := NewMap(adapter, map[uint32]string{60: "sixty"})
	mBool := NewMap(adapter, map[bool]string{true: "yes", false: "no"})
	mBoolOnlyTrue := NewMap(adapter, map[bool]string{true: "only_true"})
	mRefVal := NewRefValMap(adapter, map[ref.Val]ref.Val{
		Int(1):  String("one"),
		Uint(2): String("two"),
	})
	mTime := NewMap(adapter, map[time.Time]string{now: "timeKey"})
	emptyMap := NewMap(adapter, map[string]string{})

	tests := []struct {
		name      string
		mapper    traits.Mapper
		key       ref.Val
		wantVal   ref.Val
		wantFound bool
	}{
		{name: "empty map miss", mapper: emptyMap, key: String("a"), wantVal: nil, wantFound: false},
		{name: "string hit", mapper: mStr, key: String("a"), wantVal: Uint(1), wantFound: true},
		{name: "string wrong type", mapper: mStr, key: Int(1), wantVal: nil, wantFound: false},
		{name: "string miss", mapper: mStr, key: String("missing"), wantVal: nil, wantFound: false},

		{name: "int64 hit Int", mapper: mInt64, key: Int(10), wantVal: String("ten"), wantFound: true},
		{name: "int64 hit Uint lossless", mapper: mInt64, key: Uint(10), wantVal: String("ten"), wantFound: true},
		{name: "int64 hit Double lossless", mapper: mInt64, key: Double(10.0), wantVal: String("ten"), wantFound: true},
		{name: "int64 wrong type", mapper: mInt64, key: String("wrong"), wantVal: nil, wantFound: false},
		{name: "int64 miss", mapper: mInt64, key: Int(99), wantVal: nil, wantFound: false},

		{name: "int hit Int", mapper: mInt, key: Int(20), wantVal: String("twenty"), wantFound: true},
		{name: "int overflow", mapper: mInt, key: Int(math.MaxInt64), wantVal: nil, wantFound: false},
		{name: "int underflow", mapper: mInt, key: Int(math.MinInt64), wantVal: nil, wantFound: false},
		{name: "int miss", mapper: mInt, key: Int(99), wantVal: nil, wantFound: false},
		{name: "int wrong type", mapper: mInt, key: String("wrong"), wantVal: nil, wantFound: false},

		{name: "int32 hit Int", mapper: mInt32, key: Int(30), wantVal: String("thirty"), wantFound: true},
		{name: "int32 overflow", mapper: mInt32, key: Int(math.MaxInt64), wantVal: nil, wantFound: false},
		{name: "int32 miss", mapper: mInt32, key: Int(99), wantVal: nil, wantFound: false},

		{name: "uint64 hit Uint", mapper: mUint64, key: Uint(40), wantVal: String("forty"), wantFound: true},
		{name: "uint64 hit Int lossless", mapper: mUint64, key: Int(40), wantVal: String("forty"), wantFound: true},
		{name: "uint64 hit Double lossless", mapper: mUint64, key: Double(40.0), wantVal: String("forty"), wantFound: true},
		{name: "uint64 wrong type", mapper: mUint64, key: String("wrong"), wantVal: nil, wantFound: false},
		{name: "uint64 miss", mapper: mUint64, key: Uint(99), wantVal: nil, wantFound: false},

		{name: "uint hit Uint", mapper: mUint, key: Uint(50), wantVal: String("fifty"), wantFound: true},
		{name: "uint overflow", mapper: mUint, key: Uint(math.MaxUint64), wantVal: nil, wantFound: false},
		{name: "uint miss", mapper: mUint, key: Uint(99), wantVal: nil, wantFound: false},
		{name: "uint wrong type", mapper: mUint, key: String("wrong"), wantVal: nil, wantFound: false},

		{name: "uint32 hit Uint", mapper: mUint32, key: Uint(60), wantVal: String("sixty"), wantFound: true},
		{name: "uint32 overflow", mapper: mUint32, key: Uint(math.MaxUint64), wantVal: nil, wantFound: false},
		{name: "uint32 miss", mapper: mUint32, key: Uint(99), wantVal: nil, wantFound: false},

		{name: "bool hit True", mapper: mBool, key: True, wantVal: String("yes"), wantFound: true},
		{name: "bool hit False", mapper: mBool, key: False, wantVal: String("no"), wantFound: true},
		{name: "bool miss False", mapper: mBoolOnlyTrue, key: False, wantVal: nil, wantFound: false},
		{name: "bool wrong type", mapper: mBool, key: String("wrong"), wantVal: nil, wantFound: false},

		{name: "ref.Val Double to Int lossless", mapper: mRefVal, key: Double(1.0), wantVal: String("one"), wantFound: true},
		{name: "ref.Val Double to Uint lossless", mapper: mRefVal, key: Double(2.0), wantVal: String("two"), wantFound: true},
		{name: "ref.Val Uint to Int lossless", mapper: mRefVal, key: Uint(1), wantVal: String("one"), wantFound: true},
		{name: "ref.Val Int to Uint lossless", mapper: mRefVal, key: Int(2), wantVal: String("two"), wantFound: true},
		{name: "ref.Val miss", mapper: mRefVal, key: String("missing"), wantVal: nil, wantFound: false},

		{name: "custom key hit", mapper: mTime, key: adapter.NativeToValue(now), wantVal: String("timeKey"), wantFound: true},
		{name: "custom key miss", mapper: mTime, key: String("missing"), wantVal: nil, wantFound: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, found := tc.mapper.Find(tc.key)
			if found != tc.wantFound || val != tc.wantVal {
				t.Errorf("Find(%v) got %v, %v, want %v, %v", tc.key, val, found, tc.wantVal, tc.wantFound)
			}
		})
	}
}

// TestNativeMapContainsNoAllocs guards against presence tests (`k in m`) regressing to value
// conversion. Values are chosen so that converting them to a ref.Val would allocate: ints above
// the runtime's small-int cache, non-empty strings, and raw structs boxed via packAny.
func TestNativeMapContainsNoAllocs(t *testing.T) {
	type rawStruct struct {
		Name  string
		Count int
	}
	adapter := DefaultTypeAdapter
	tests := []struct {
		name   string
		mapper traits.Mapper
		hit    ref.Val
		miss   ref.Val
	}{
		{
			name:   "string_int64",
			mapper: NewMap(adapter, map[string]int64{"a": 1 << 20}),
			hit:    String("a"),
			miss:   String("b"),
		},
		{
			name:   "string_string",
			mapper: NewMap(adapter, map[string]string{"a": "value"}),
			hit:    String("a"),
			miss:   String("b"),
		},
		{
			name:   "string_struct",
			mapper: NewMap(adapter, map[string]rawStruct{"a": {Name: "n", Count: 1 << 20}}),
			hit:    String("a"),
			miss:   String("b"),
		},
		{
			name:   "int_struct",
			mapper: NewMap(adapter, map[int]rawStruct{1000: {Name: "n", Count: 1 << 20}}),
			hit:    Int(1000),
			miss:   Int(1001),
		},
		{
			name:   "uint64_float64",
			mapper: NewMap(adapter, map[uint64]float64{1000: 1.5}),
			hit:    Uint(1000),
			miss:   Uint(1001),
		},
		{
			name:   "refval_refval",
			mapper: NewRefValMap(adapter, map[ref.Val]ref.Val{Int(1000): String("value")}),
			hit:    Uint(1000), // exercises the cross-numeric fallback lookup
			miss:   Int(1001),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.mapper.Contains(tc.hit); got != True {
				t.Fatalf("Contains(%v) = %v, want true", tc.hit, got)
			}
			if got := tc.mapper.Contains(tc.miss); got != False {
				t.Fatalf("Contains(%v) = %v, want false", tc.miss, got)
			}
			if allocs := testing.AllocsPerRun(100, func() {
				tc.mapper.Contains(tc.hit)
				tc.mapper.Contains(tc.miss)
			}); allocs != 0 {
				t.Errorf("Contains allocated %v allocs/run, want 0", allocs)
			}
		})
	}
}

func TestProtoMapFullCoverage(t *testing.T) {
	reg, err := NewRegistry(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	msg := &proto3pb.TestAllTypes{
		MapStringString: map[string]string{
			"a": "b",
			"c": "d",
		},
	}
	pbMsg := reg.NativeToValue(msg).(traits.Indexer)
	pm := pbMsg.Get(String("map_string_string")).(traits.Mapper)

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "metadata and size",
			test: func(t *testing.T) {
				if pm.Type() != MapType {
					t.Errorf("Type() got %v, want MapType", pm.Type())
				}
				if pm.Value() == nil {
					t.Errorf("Value() got nil")
				}
				if pm.Size() != Int(2) {
					t.Errorf("Size() got %v, want 2", pm.Size())
				}
			},
		},
		{
			name: "fold early stop",
			test: func(t *testing.T) {
				fes := &testEarlyStopFolder{limit: 1}
				pm.(traits.Foldable).Fold(fes)
			},
		},
		{
			name: "iterator past end",
			test: func(t *testing.T) {
				it := pm.Iterator()
				for it.HasNext() == True {
					_ = it.Next()
				}
				if it.HasNext() != False {
					t.Errorf("expected HasNext == False")
				}
				if it.Next() != nil {
					t.Errorf("expected Next == nil")
				}
			},
		},
		{
			name: "convert to native error",
			test: func(t *testing.T) {
				if _, err := pm.ConvertToNative(reflect.TypeOf(123)); err == nil {
					t.Errorf("ConvertToNative(int) expected error, got nil")
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.test)
	}
}

func TestProxyMapFoldVariations(t *testing.T) {
	adapter := DefaultTypeAdapter
	rawMap := NewMap(adapter, map[string]string{"a": "1", "b": "2"})
	proxy := proxyLegacyMap{proxy: rawMap}
	foldable := ToFoldableMap(proxy)

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "normal fold",
			test: func(t *testing.T) {
				fko := &testFoldKeyOnly{}
				foldable.Fold(fko)
				if len(fko.keys) != 2 {
					t.Errorf("Fold got %d keys, want 2", len(fko.keys))
				}
			},
		},
		{
			name: "early stop fold",
			test: func(t *testing.T) {
				fes := &testEarlyStopFolder{limit: 1}
				foldable.Fold(fes)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.test)
	}
}

func TestMapConvertToNativeErrorCases(t *testing.T) {
	adapter := DefaultTypeAdapter
	m := NewMap(adapter, map[string]string{"a": "hello"})
	intKeyMap := NewMap(adapter, map[int]int{1: 2})
	strValForIntField := NewMap(adapter, map[string]string{"Age": "not_an_int"})
	mBadKey := NewRefValMap(adapter, map[ref.Val]ref.Val{
		NewList(adapter, []int{1}): String("val"),
	})

	type testStructWithField struct {
		Age int
	}

	tests := []struct {
		name      string
		mapper    traits.Mapper
		target    reflect.Type
		wantError bool
	}{
		{name: "target not a map", mapper: m, target: reflect.TypeOf("string"), wantError: true},
		{name: "value conversion error", mapper: m, target: reflect.TypeOf(map[string]int{}), wantError: true},
		{name: "key conversion error", mapper: m, target: reflect.TypeOf(map[int]string{}), wantError: true},
		{name: "convert to map[any]any", mapper: m, target: reflect.TypeOf(map[any]any{}), wantError: false},
		{name: "convert to any", mapper: m, target: reflect.TypeFor[any](), wantError: false},
		{name: "non-string key to anyValueType", mapper: intKeyMap, target: anyValueType, wantError: true},
		{name: "non-string key to JSONStructType", mapper: intKeyMap, target: JSONStructType, wantError: true},
		{name: "struct field type mismatch", mapper: strValForIntField, target: reflect.TypeFor[testStructWithField](), wantError: true},
		{name: "struct unconvertible list key", mapper: mBadKey, target: reflect.TypeFor[struct{ Val string }](), wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.mapper.ConvertToNative(tc.target)
			if tc.wantError && err == nil {
				t.Errorf("ConvertToNative(%v) expected error, got nil", tc.target)
			}
			if !tc.wantError && (err != nil || v == nil) {
				t.Errorf("ConvertToNative(%v) unexpected error: %v", tc.target, err)
			}
		})
	}
}

func TestMapEdgeCasesRemainingCoverage(t *testing.T) {
	adapter := DefaultTypeAdapter
	st := testCreateStruct(t, map[string]any{"x": 1})
	bm := NewJSONStruct(adapter, st)
	mTwo := NewMap(adapter, map[string]string{"a": "1", "b": "2"})

	reg, _ := NewRegistry(&proto3pb.TestAllTypes{})
	msg := &proto3pb.TestAllTypes{
		MapStringString: map[string]string{"k": "v"},
	}
	pbMsg := reg.NativeToValue(msg).(traits.Indexer)
	pm := pbMsg.Get(String("map_string_string")).(traits.Mapper)

	type rawStructVal struct {
		Number int
	}
	mRawStruct := NewMap(adapter, map[string]rawStructVal{"key": {Number: 42}})

	type keyOnlyFolder interface {
		traits.Folder
		FoldKeyOnly() bool
	}

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "equal with different keys",
			test: func(t *testing.T) {
				mDiff1 := NewMap(adapter, map[string]string{"a": "1"})
				mDiff2 := NewMap(adapter, map[string]string{"b": "1"})
				if eq := mDiff1.Equal(mDiff2); eq != False {
					t.Errorf("Equal on diff keys got %v, want False", eq)
				}
			},
		},
		{
			name: "equal with non-mapper",
			test: func(t *testing.T) {
				if eq := mTwo.Equal(Int(123)); eq != False {
					t.Errorf("Equal(Int) got %v, want False", eq)
				}
			},
		},
		{
			name: "foldKeyOnly early break nativeMap",
			test: func(t *testing.T) {
				var kof keyOnlyFolder = &testKeyOnlyEarly{limit: 1}
				mTwo.(traits.Foldable).Fold(kof)
			},
		},
		{
			name: "foldKeyOnly early break interopFoldableMap",
			test: func(t *testing.T) {
				var kof keyOnlyFolder = &testKeyOnlyEarly{limit: 1}
				proxyTwo := ToFoldableMap(proxyLegacyMap{proxy: mTwo})
				proxyTwo.Fold(kof)
			},
		},
		{
			name: "baseMap aggregate size caching",
			test: func(t *testing.T) {
				calc := NewSizeCalculator()
				sz1 := calc.AggregateSize(bm)
				sz2 := calc.AggregateSize(bm) // Cache hit (lines 311-313, 325-328)
				if sz1 == 0 || sz1 != sz2 {
					t.Errorf("calc.AggregateSize(bm) got %d, %d", sz1, sz2)
				}

				bmNilValue := &baseMap{
					Adapter: adapter,
					size:    0,
					mapAccessor: &nativeMap[string, string]{
						Adapter: adapter,
						mapVal:  map[string]string{},
					},
				}
				_ = calc.AggregateSize(bmNilValue)
			},
		},
		{
			name: "protoMap aggregate size caching",
			test: func(t *testing.T) {
				calc := NewSizeCalculator()
				psz1 := calc.AggregateSize(pm)
				psz2 := calc.AggregateSize(pm) // Cache hit (lines 1363-1365)
				if psz1 == 0 || psz1 != psz2 {
					t.Errorf("calc.AggregateSize(pm) got %d, %d", psz1, psz2)
				}
			},
		},
		{
			name: "protoMap convert to native errors",
			test: func(t *testing.T) {
				msgIntKey := &proto3pb.TestAllTypes{
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{1: {}},
				}
				pbMsgIntKey := reg.NativeToValue(msgIntKey).(traits.Indexer)
				pmIntKey := pbMsgIntKey.Get(String("map_int64_nested_type")).(traits.Mapper)
				if _, err := pmIntKey.ConvertToNative(anyValueType); err == nil {
					t.Errorf("pmIntKey ConvertToNative(anyValueType) expected error")
				}
				if _, err := pmIntKey.ConvertToNative(JSONStructType); err == nil {
					t.Errorf("pmIntKey ConvertToNative(JSONStructType) expected error")
				}
			},
		},
		{
			name: "stringKeyIterator past end",
			test: func(t *testing.T) {
				stIter := bm.Iterator()
				for stIter.HasNext() == True {
					_ = stIter.Next()
				}
				if stIter.Next() != nil {
					t.Errorf("stIter Next() past end expected nil")
				}
			},
		},
		{
			name: "valToQualifyAny with qualifyRawVal == true",
			test: func(t *testing.T) {
				if val, found := mRawStruct.(interface{ FindStringKey(string) (any, bool) }).FindStringKey("key"); !found || val.(rawStructVal).Number != 42 {
					t.Errorf("FindStringKey on raw struct map got %v, %v", val, found)
				}
			},
		},
		{
			name: "non-caching sizer",
			test: func(t *testing.T) {
				type nonCachingSizer struct {
					AggregateSizer
				}
				ncSizer := nonCachingSizer{AggregateSizer: NewSizeCalculator()}
				bmUncached := NewJSONStruct(adapter, st)
				_ = bmUncached.(AggregateSizeVisitor).AggregateSize(ncSizer)
				_ = pm.(AggregateSizeVisitor).AggregateSize(ncSizer)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.test)
	}
}

type testKeyOnlyEarly struct {
	count int
	limit int
}

func (f *testKeyOnlyEarly) FoldKeyOnly() bool {
	return true
}

func (f *testKeyOnlyEarly) FoldEntry(key, val any) bool {
	f.count++
	return f.count < f.limit
}

func TestCustomAdapterMap_Issue1594(t *testing.T) {
	customAdapt := customVerAdapter{Adapter: DefaultTypeAdapter}

	mapsToTest := []struct {
		name string
		m    traits.Mapper
	}{
		{
			name: "NewMap_StringStruct",
			m:    NewMap(customAdapt, map[string]customVerStruct{"a": {10}, "b": {20}}),
		},
		{
			name: "NewMap_CustomKeyStructVal",
			m:    NewMap(customAdapt, map[customVerStruct]customVerStruct{{1}: {10}, {2}: {20}}),
		},
		{
			name: "NewDynamicMap_StringStruct",
			m:    NewDynamicMap(customAdapt, map[string]customVerStruct{"a": {10}, "b": {20}}),
		},
	}

	for _, tc := range mapsToTest {
		t.Run(tc.name, func(t *testing.T) {
			var foldCount int
			tc.m.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
				if _, ok := k.(ref.Val); !ok {
					t.Fatalf("FoldEntry received unadapted key %T (%v), wanted ref.Val", k, k)
				}
				if _, ok := v.(ref.Val); !ok {
					t.Fatalf("FoldEntry received unadapted val %T (%v), wanted ref.Val", v, v)
				}
				foldCount++
				return true
			}})
			if foldCount != 2 {
				t.Errorf("Fold count = %d, want 2", foldCount)
			}
			// Early termination in Fold
			tc.m.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
				return false
			}})
			if sk, ok := tc.m.(interface{ FindStringKey(string) (any, bool) }); ok {
				if got, found := sk.FindStringKey("a"); found && got != Int(10) {
					t.Errorf("FindStringKey(\"a\") = %v, want Int(10)", got)
				}
			}
		})
	}
}

