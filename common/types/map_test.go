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

	"github.com/google/cel-go/common/types/pb"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"

	proto3pb "github.com/google/cel-go/test/proto3pb"
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
			want: 9,
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
			want: 0,
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
