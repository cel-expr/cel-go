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
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"

	anypb "google.golang.org/protobuf/types/known/anypb"
	dpb "google.golang.org/protobuf/types/known/durationpb"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

func TestBaseListAdd_Empty(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewDynamicList(reg, []bool{true})
	if list.Add(NewDynamicList(reg, []bool{})) != list {
		t.Error("Adding an empty list created new list.")
	}
	if NewDynamicList(reg, []string{}).Add(list) != list {
		t.Error("Adding list to empty created a new list.")
	}
}

func TestBaseListAdd_Error(t *testing.T) {
	if !IsError(NewDynamicList(newTestRegistry(t), []bool{}).Add(String("error"))) {
		t.Error("Addind a non-list value to a list unexpected succeeds.")
	}
}

func TestBaseListContains(t *testing.T) {
	list := NewDynamicList(newTestRegistry(t), []float32{1.0, 2.0, 3.0})
	tests := []struct {
		in  ref.Val
		out ref.Val
	}{
		{
			in:  Double(math.NaN()),
			out: False,
		},
		{
			in:  Double(5),
			out: False,
		},
		{
			in:  Double(3),
			out: True,
		},
		{
			in:  Uint(3),
			out: True,
		},
		{
			in:  Int(3),
			out: True,
		},
		{
			in:  Int(3),
			out: True,
		},
		{
			in:  Int(0),
			out: False,
		},
		{
			in:  String("3"),
			out: False,
		},
	}
	for _, tc := range tests {
		got := list.Contains(tc.in)
		if !reflect.DeepEqual(got, tc.out) {
			t.Errorf("list.Contains(%v) returned %v, wanted %v", tc.in, got, tc.out)
		}
	}
}

func TestBaseListConvertToNative(t *testing.T) {
	list := NewDynamicList(newTestRegistry(t), []float64{1.0, 2.0})
	if protoList, err := list.ConvertToNative(reflect.TypeOf([]float32{})); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(protoList, []float32{1.0, 2.0}) {
		t.Errorf("Could not convert to []float32: %v", protoList)
	}
}

func TestBaseListConvertToNative_Any(t *testing.T) {
	list := NewDynamicList(newTestRegistry(t), []float64{1.0, 2.0})
	val, err := list.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	jsonVal := &structpb.ListValue{}
	err = protojson.Unmarshal([]byte("[1.0, 2.0]"), jsonVal)
	if err != nil {
		t.Fatalf("protojson.Unmarshal() failed: %v", err)
	}
	want, err := anypb.New(jsonVal)
	if err != nil {
		t.Fatalf("anypb.New() failed: %v", err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got %v, wanted %v", val, want)
	}
}

func TestBaseListConvertToNative_Json(t *testing.T) {
	list := NewDynamicList(newTestRegistry(t), []float64{1.0, 2.0})
	val, err := list.ConvertToNative(JSONListType)
	if err != nil {
		t.Error(err)
	}
	want := &structpb.ListValue{}
	err = protojson.Unmarshal([]byte("[1.0, 2.0]"), want)
	if err != nil {
		t.Fatalf("protojson.Unmarshal() failed: %v", err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got %v, wanted %v", val, want)
	}
}

func TestBaseListConvertToType(t *testing.T) {
	list := NewDynamicList(newTestRegistry(t), []string{"h", "e", "l", "l", "o"})
	if list.ConvertToType(ListType) != list {
		t.Error("List was not convertible to itself.")
	}
	if list.ConvertToType(TypeType) != ListType {
		t.Error("Unable to obtain the proper type from the list.")
	}
	if !IsError(list.ConvertToType(MapType)) {
		t.Error("List was able to convert to unexpected type.")
	}
}

func TestBaseListEqual(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []string{"h", "e", "l", "l", "o"})
	if listA.Equal(listA) != True {
		t.Error("listA.Equal(listA) did not return true.")
	}
	listB := NewDynamicList(reg, []string{"h", "e", "l", "p", "!"})
	if listA.Equal(listB) != False {
		t.Error("listA.Equal(listB) did not return false.")
	}
	listC := reg.NativeToValue([]any{"h", "e", "l", "l", String("o")})
	if listA.Equal(listC) != True {
		t.Error("listA.Equal(listC) did not return true.")
	}
	listD := reg.NativeToValue([]any{"h", "e", 1, "p", "!"})
	if listA.Equal(listD) != False {
		t.Error("listA.Equal(listD) did not return true")
	}
	if IsError(listB.Equal(listD)) {
		t.Error("listA.Equal(listD) errored, wanted 'false'")
	}
}

func TestBaseListGet(t *testing.T) {
	validateList123(t, NewDynamicList(newTestRegistry(t), []int32{1, 2, 3}))
}

func TestBaseListString(t *testing.T) {
	l := DefaultTypeAdapter.NativeToValue([]any{1, "hello", 2.1, true, []string{"world"}})
	want := `[1, hello, 2.1, true, [world]]`
	if fmt.Sprintf("%v", l) != want {
		t.Errorf("l.String() got %v, wanted %v", l, want)
	}
}

func TestConcatListString(t *testing.T) {
	l := DefaultTypeAdapter.NativeToValue([]any{1, "hello", 2.1, true}).(traits.Lister)
	c := l.Add(DefaultTypeAdapter.NativeToValue([]string{"world"}))
	want := `[1, hello, 2.1, true, world]`
	if fmt.Sprintf("%v", c) != want {
		t.Errorf("c.String() got %v, wanted %v", c, want)
	}
}

func TestListIsZeroValue(t *testing.T) {
	tests := []struct {
		val         any
		isZeroValue bool
	}{
		{
			val:         []string{},
			isZeroValue: true,
		},
		{
			val:         []int{},
			isZeroValue: true,
		},
		{
			val:         []any{},
			isZeroValue: true,
		},
		{
			val:         &structpb.ListValue{},
			isZeroValue: true,
		},
		{
			val:         []ref.Val{},
			isZeroValue: true,
		},
		{
			val:         DefaultTypeAdapter.NativeToValue([]ref.Val{}).(traits.Lister).Add(DefaultTypeAdapter.NativeToValue([]ref.Val{})),
			isZeroValue: true,
		},
		{
			val:         []string{""},
			isZeroValue: false,
		},
		{
			val:         []bool{false},
			isZeroValue: false,
		},
		{
			val:         []any{0},
			isZeroValue: false,
		},
		{
			val:         &structpb.ListValue{Values: []*structpb.Value{structpb.NewBoolValue(false)}},
			isZeroValue: false,
		},
		{
			val:         []ref.Val{Double(0.0)},
			isZeroValue: false,
		},
		{
			val:         DefaultTypeAdapter.NativeToValue([]ref.Val{IntOne}).(traits.Lister).Add(DefaultTypeAdapter.NativeToValue([]ref.Val{})),
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

func TestValueListGet(t *testing.T) {
	validateList123(t, NewRefValList(newTestRegistry(t), []ref.Val{Int(1), Int(2), Int(3)}))
}

func TestBaseListIterator(t *testing.T) {
	validateIterator123(t, NewDynamicList(newTestRegistry(t), []int32{1, 2, 3}))
}

func TestValueListValue_Iterator(t *testing.T) {
	validateIterator123(t, NewRefValList(newTestRegistry(t), []ref.Val{Int(1), Int(2), Int(3)}))
}

func TestBaseListNestedList(t *testing.T) {
	reg := newTestRegistry(t)
	listUint32 := []uint32{1, 2}
	nestedUint32 := NewDynamicList(reg, []any{listUint32})
	listUint64 := []uint64{1, 2}
	nestedUint64 := NewDynamicList(reg, []any{listUint64})
	if nestedUint32.Equal(nestedUint64) != True {
		t.Error("Could not find nested list")
	}
	if nestedUint32.Contains(NewDynamicList(reg, listUint64)) != True ||
		nestedUint64.Contains(NewDynamicList(reg, listUint32)) != True {
		t.Error("Could not find type compatible nested lists")
	}
}

func TestBaseListSize(t *testing.T) {
	reg := newTestRegistry(t)
	listUint32 := []uint32{1, 2}
	nestedUint32 := NewDynamicList(reg, []any{listUint32})
	if nestedUint32.Size() != IntOne {
		t.Error("List indicates the incorrect size.")
	}
	if nestedUint32.Get(IntZero).(traits.Sizer).Size() != Int(2) {
		t.Error("Nested list indicates the incorrect size.")
	}
}

func TestMutableListGet(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewMutableList(reg)
	listB := NewStringList(reg, []string{"item"})
	listA = listA.Add(listB).(*mutableList)

	itemVal := listA.Get(Int(0))

	if itemVal.Value().(string) != "item" {
		t.Error("MutableList get returned invalid item.")
	}
}

func TestConcatListAdd(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewStringList(reg, []string{"3"})
	list := listA.Add(listB).(traits.Lister).Add(listA).
		Value().([]ref.Val)
	expected := []ref.Val{
		Double(1.0),
		Double(2.0),
		String("3"),
		Double(1.0),
		Double(2.0)}
	if len(list) != len(expected) {
		t.Errorf("Got '%v', expected '%v'", list, expected)
	} else {
		for i := 0; i < len(list); i++ {
			if expected[i] != list[i] {
				t.Errorf("elem[%d] Got '%v', expected '%v'",
					i, list[i], expected[i])
			}
		}
	}
	// Zero length input list
	listConcat := listA.Add(listB).(traits.Lister)
	same := listConcat.Add(NewStringList(reg, []string{}))
	if !reflect.DeepEqual(listConcat, same) {
		t.Error("Adding an empty list to a concat list did not return the concat list")
	}
	// Zero length operand list
	same = NewDynamicList(reg, []bool{}).Add(listConcat)
	if !reflect.DeepEqual(listConcat, same) {
		t.Error("Adding a concat list to an empty list did not return the concat list")
	}
}

func TestConcatListConvertToNative_Json(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []string{"3"})
	list := listA.Add(listB)
	jsonVal, err := list.ConvertToNative(JSONValueType)
	if err != nil {
		t.Fatalf("Got error '%v', expected value", err)
	}
	jsonBytes, err := protojson.Marshal(jsonVal.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal(%v) failed: %v", jsonVal, err)
	}
	jsonTxt := string(jsonBytes)
	outList := []any{}
	err = json.Unmarshal(jsonBytes, &outList)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", jsonTxt, err)
	}
	if !reflect.DeepEqual(outList, []any{1.0, 2.0, "3"}) {
		t.Errorf("got json '%v', expected %v", outList, []any{1.0, 2.0, "3"})
	}
	// Test proto3 to JSON conversion.
	listC := NewDynamicList(reg, []*dpb.Duration{{Seconds: 100}})
	listConcat := listA.Add(listC)
	jsonVal, err = listConcat.ConvertToNative(JSONValueType)
	if err != nil {
		t.Fatal(err)
	}
	jsonBytes, err = protojson.Marshal(jsonVal.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal(%v) failed: %v", jsonVal, err)
	}
	jsonTxt = string(jsonBytes)
	outList = []any{}
	err = json.Unmarshal(jsonBytes, &outList)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", jsonTxt, err)
	}
	if !reflect.DeepEqual(outList, []any{1.0, 2.0, "100s"}) {
		t.Errorf("got json '%v', expected %v", outList, []any{1.0, 2.0, "100s"})
	}
}

func TestConcatListConvertToNativeListInterface(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewStringList(reg, []string{"3.0"})
	list := listA.Add(listB)
	iface, err := list.ConvertToNative(reflect.TypeOf([]any{}))
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, list)
	}
	want := []any{1.0, 2.0, "3.0"}
	if !reflect.DeepEqual(iface, want) {
		t.Errorf("Got '%v', expected '%v'", iface, want)
	}
}

func TestConcatListConvertToType(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []*dpb.Duration{{Seconds: 100}})
	list := listA.Add(listB)
	if list.ConvertToType(ListType) != list {
		t.Error("List conversion to list failed.")
	}
	if list.ConvertToType(TypeType) != ListType {
		t.Error("List conversion to type failed.")
	}
	if !IsError(list.ConvertToType(MapType)) {
		t.Error("List conversion to map unexpectedly succeeded.")
	}
}

func TestConcatListContains(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []string{"3"})
	listConcat := listA.Add(listB).(traits.Lister)
	if listConcat.Contains(String("3")) != True {
		t.Error("Concatenated list did not contain value in 'next' list.")
	}
	if listConcat.Contains(Double(2.0)) != True {
		t.Error("Concatenated list did not contain value in 'prev' list.")
	}
	homogList := NewDynamicList(reg, []string{"3"}).Add(
		NewStringList(reg, []string{"2", "1"})).(traits.Lister)
	if homogList.Contains(String("4")) != False {
		t.Error("Concatenated homogeneous list did not return false.")
	}
}

func TestConcatListContainsNonBool(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []string{"3"})
	listConcat := listA.Add(listB).(traits.Lister)
	if IsError(listConcat.Contains(String("4"))) {
		t.Error("Contains errored with a not-found element, wanted 'false'")
	}
}

func TestConcatListEqual(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []float64{3.0})
	list := listA.Add(listB)
	// Note the internal type of list raw and concat list are slightly different.
	listRaw := NewDynamicList(reg, []any{float32(1.0), float64(2.0), float64(3.0)})
	if listRaw.Equal(list) != True || list.Equal(listRaw) != True {
		t.Errorf("listRaw.Equal(list) not true, got '%v', expected '%v'", list.Value(), listRaw.Value())
	}
	if list.Equal(listA) == True || listRaw.Equal(listA) == True {
		t.Error("lists of unequal length considered equal")
	}
	listC := reg.NativeToValue([]any{1.0, 3.0, 2.0})
	if list.Equal(listC) != False {
		t.Errorf("list.Equal(listC) got %v, wanted false", list.Equal(listC))
	}
	listD := reg.NativeToValue([]any{1, 2.0, 3.0})
	if list.Equal(listD) != True {
		t.Errorf("list.Equal(listD) got %v, wanted true", list.Equal(listD))
	}
	if list.Equal(NullValue) != False {
		t.Errorf("list.Equal(NullValue) got %v, wanted false", list.Equal(NullValue))
	}
}

func TestConcatListGet(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []float64{3.0})
	list := listA.Add(listB).(traits.Lister)
	if getElem(t, list, Int(0)) != Double(1.0) ||
		getElem(t, list, Uint(1)) != Double(2.0) ||
		getElem(t, list, Double(2.0)) != Double(3.0) {
		t.Errorf("List values by index did not match expectations")
	}
	if val := list.Get(Int(-1)); !IsError(val) {
		t.Errorf("Should not have been able to read a negative index")
	}
	if val := list.Get(Int(3)); !IsError(val) {
		t.Errorf("Should not have been able to read beyond end of list")
	}
}

func TestConcatListIterator(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []float64{3.0})
	list := listA.Add(listB).(traits.Lister)
	it := list.Iterator()
	var i = int64(0)
	for ; it.HasNext() == True; i++ {
		elem := it.Next()
		if getElem(t, list, Int(i)) != elem {
			t.Errorf(
				"List iterator returned incorrect value: list[%d]: %v", i, elem)
		}
	}
	if it.Next() != nil {
		t.Errorf("List iterator attempted to continue beyond list size")
	}
	if i != 3 {
		t.Errorf("Iterator did not iterate until last value")
	}
}

func TestStringListAdd_Empty(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewStringList(reg, []string{"hello"})
	if list.Add(NewStringList(reg, []string{})) != list {
		t.Error("Adding empty lists resulted in new list creation.")
	}
	if NewStringList(reg, []string{}).Add(list) != list {
		t.Error("Adding empty lists resulted in new list creation.")
	}
}

func TestStringListAdd_Error(t *testing.T) {
	reg := newTestRegistry(t)
	if !IsError(NewStringList(reg, []string{}).Add(True)) {
		t.Error("Got list, expected error.")
	}
}

func TestStringListAdd_Heterogenous(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewStringList(reg, []string{"hello"})
	listB := NewDynamicList(reg, []int32{1, 2, 3})
	list := listA.Add(listB).(traits.Lister).Value().([]any)
	expected := []any{"hello", int64(1), int64(2), int64(3)}
	if len(list) != len(expected) {
		t.Errorf("Unexpected list size. Got '%d', expected 4", len(list))
	}
	for i, v := range expected {
		if list[i] != v {
			t.Errorf("elem[%d] Got '%v', expected '%v'", i, list[i], v)
		}
	}
}

func TestStringListAdd_StringLists(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewStringList(reg, []string{"hello"})
	listB := NewStringList(reg, []string{"world", "!"})
	list := listA.Add(listB).(traits.Lister)
	if list.Size() != Int(3) {
		t.Error("Combined list did not have correct size.")
	}
	expected := []string{"hello", "world", "!"}
	for i, v := range expected {
		if list.Get(Int(i)).Equal(String(v)) != True {
			t.Errorf("elem[%d] Got '%v', expected '%v'", i, list.Get(Int(i)), v)
		}
	}
}

func TestStringListConvertToNative(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	val, err := list.ConvertToNative(reflect.TypeOf([]string{}))
	if err != nil {
		t.Error("Unable to convert string list to itself.")
	}
	if !reflect.DeepEqual(val, []string{"h", "e", "l", "p"}) {
		t.Errorf(`Got %v, expected ["h", "e", "l", "p"]`, val)
	}
}

func TestStringListConvertToNative_ListInterface(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	val, err := list.ConvertToNative(reflect.TypeOf([]any{}))
	if err != nil {
		t.Error("Unable to convert string list to itself.")
	}
	want := []any{"h", "e", "l", "p"}
	if !reflect.DeepEqual(val.([]any), want) {
		for i, e := range val.([]any) {
			t.Logf("val[%d] %v(%T)", i, e, e)
		}
		for i, e := range want {
			t.Logf("want[%d] %v(%T)", i, e, e)
		}
		t.Errorf(`Got %v(%T), expected %v(%T)`, val, val, want, want)
	}
}

func TestStringListConvertToNative_Error(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	_, err := list.ConvertToNative(JSONStructType)
	if err == nil {
		t.Error("Conversion of list to unsupported type did not error.")
	}
}

func TestStringListConvertToNative_Json(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	jsonVal, err := list.ConvertToNative(JSONValueType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, jsonVal)
	}
	jsonBytes, err := protojson.Marshal(jsonVal.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal(%v) failed: %v", jsonVal, err)
	}
	jsonTxt := string(jsonBytes)
	outList := []any{}
	err = json.Unmarshal(jsonBytes, &outList)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", jsonTxt, err)
	}
	if !reflect.DeepEqual(outList, []any{"h", "e", "l", "p"}) {
		t.Errorf("got json '%v', expected %v", jsonTxt, outList)
	}

	jsonList, err := list.ConvertToNative(JSONListType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, jsonList)
	}
	jsonListBytes, err := protojson.Marshal(jsonList.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal(%v) failed: %v", jsonVal, err)
	}
	jsonListTxt := string(jsonListBytes)
	if jsonTxt != jsonListTxt {
		t.Errorf("Json value and list value not equal.")
	}
}

func TestStringListGet_OutOfRange(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewStringList(reg, []string{"hello", "world"})
	if !IsError(list.Get(Int(-1))) {
		t.Error("Negative index did not return error.")
	}
	if !IsError(list.Get(Int(2))) {
		t.Error("Index out of range did not return error.")
	}
	if !IsError(list.Get(Double(0.9))) {
		t.Error("Index out of range did not return error.")
	}
	if !IsError(list.Get(String("1"))) {
		t.Error("Invalid index type did not return error.")
	}
}

func TestValueListAdd(t *testing.T) {
	reg := newTestRegistry(t)
	listA := NewRefValList(reg, []ref.Val{String("hello")})
	listB := NewRefValList(reg, []ref.Val{String("world")})
	listConcat := listA.Add(listB).(traits.Lister)
	if listConcat.Contains(String("goodbye")) != False {
		t.Error("Homogeneous concatenated value list did not return false on missing input")
	}
	if listConcat.Contains(String("hello")) != True {
		t.Error("Homogeneous concatenated value list did not return true on valid input")
	}
}

func TestValueListConvertToNative_Json(t *testing.T) {
	reg := newTestRegistry(t)
	list := NewRefValList(reg, []ref.Val{String("hello"), String("world")})
	jsonVal, err := list.ConvertToNative(JSONListType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, jsonVal)
	}
	jsonBytes, err := protojson.Marshal(jsonVal.(proto.Message))
	if err != nil {
		t.Fatalf("protojson.Marshal(%v) failed: %v", jsonVal, err)
	}
	jsonTxt := string(jsonBytes)
	outList := []any{}
	err = json.Unmarshal(jsonBytes, &outList)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", jsonTxt, err)
	}
	if !reflect.DeepEqual(outList, []any{"hello", "world"}) {
		t.Errorf("got json '%v', expected %v", jsonTxt, outList)
	}
}

func TestMutableList(t *testing.T) {
	l := NewMutableList(DefaultTypeAdapter)
	l.Add(NewRefValList(DefaultTypeAdapter, []ref.Val{String("hello")}))
	l.Add(NewRefValList(DefaultTypeAdapter, []ref.Val{String("world")}))
	il := l.ToImmutableList()
	if il.Size() != Int(2) {
		t.Errorf("il.Size() got %d, wanted size 2", il.Size())
	}
	l.Add(NewRefValList(DefaultTypeAdapter, []ref.Val{String("!")}))
	if il.Size() != Int(2) {
		t.Errorf("il.Size() got %d, wanted size 2", il.Size())
	}
}

func TestListFold(t *testing.T) {

	tests := []struct {
		l         any
		folds     int
		foldLimit int
	}{
		{
			l:         []string{"hello", "world"},
			folds:     2,
			foldLimit: 2,
		},
		{
			l:         []string{"hello", "world"},
			folds:     1,
			foldLimit: 1,
		},
		{
			l:         []string{"hello"},
			folds:     1,
			foldLimit: 2,
		},
		{
			l:         []ref.Val{},
			folds:     0,
			foldLimit: 20,
		},
		{
			l: []ref.Val{
				String("hello"),
				String("world"),
				String("goodbye"),
				String("cruel world"),
			},
			folds:     1,
			foldLimit: 1,
		},
		{
			l: []ref.Val{
				String("hello"),
				String("world"),
				String("goodbye"),
				String("cruel world"),
			},
			folds:     4,
			foldLimit: 10,
		},
		{
			l: DefaultTypeAdapter.NativeToValue([]ref.Val{
				String("hello"),
				String("world"),
			}).(traits.Lister).Add(DefaultTypeAdapter.NativeToValue([]ref.Val{
				String("goodbye"),
				String("cruel world"),
			})),
			folds:     4,
			foldLimit: 10,
		},
		{
			l: DefaultTypeAdapter.NativeToValue([]ref.Val{
				String("hello"),
				String("world"),
			}).(traits.Lister).Add(DefaultTypeAdapter.NativeToValue([]ref.Val{
				String("goodbye"),
				String("cruel world"),
			})),
			folds:     3,
			foldLimit: 3,
		},
	}
	reg := NewEmptyRegistry()
	for i, tst := range tests {
		tc := tst
		l := reg.NativeToValue(tc.l).(traits.Lister)
		foldKinds := map[string]traits.Foldable{
			"modern": ToFoldableList(l),
			"legacy": ToFoldableList(proxyLegacyList{proxy: l}),
		}
		for foldKind, foldable := range foldKinds {
			t.Run(fmt.Sprintf("[%d]%s", i, foldKind), func(t *testing.T) {
				f := &testListFolder{foldLimit: tc.foldLimit}
				foldable.Fold(f)
				if f.folds != tc.folds {
					t.Errorf("m.Fold(f) got %d, wanted %d folds", f.folds, tc.folds)
				}
			})
		}
	}
}

type testListFolder struct {
	foldLimit int
	folds     int
}

func (f *testListFolder) FoldEntry(k, v any) bool {
	if f.foldLimit != 0 {
		if f.folds >= f.foldLimit {
			return false
		}
	}
	f.folds++
	return true
}

// proxyLegacyList omits the foldable interfaces associated with all core Lister implementations
type proxyLegacyList struct {
	proxy traits.Lister
}

func (m proxyLegacyList) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return m.proxy.ConvertToNative(typeDesc)
}

func (m proxyLegacyList) ConvertToType(typeValue ref.Type) ref.Val {
	return m.proxy.ConvertToType(typeValue)
}

func (m proxyLegacyList) Equal(other ref.Val) ref.Val {
	return m.proxy.Equal(other)
}

func (m proxyLegacyList) Type() ref.Type {
	return m.proxy.Type()
}

func (m proxyLegacyList) Value() any {
	return m.proxy.Value()
}

func (m proxyLegacyList) Add(other ref.Val) ref.Val {
	return m.proxy.Add(other)
}

func (m proxyLegacyList) Contains(value ref.Val) ref.Val {
	return m.proxy.Contains(value)
}

func (m proxyLegacyList) Get(index ref.Val) ref.Val {
	return m.proxy.Get(index)
}

func (m proxyLegacyList) Iterator() traits.Iterator {
	return m.proxy.Iterator()
}

func (m proxyLegacyList) Size() ref.Val {
	return m.proxy.Size()
}

func getElem(t *testing.T, list traits.Indexer, index ref.Val) any {
	t.Helper()
	val := list.Get(index)
	if IsError(val) {
		t.Errorf("Error reading list index %d, %v", index, val)
		return nil
	}
	return val
}

func validateList123(t *testing.T, list traits.Lister) {
	t.Helper()
	if getElem(t, list, Int(0)) != Int(1) ||
		getElem(t, list, Uint(1)) != Int(2) ||
		getElem(t, list, Double(2.0)) != Int(3) {
		t.Errorf("List values by index did not match expectations")
	}
	if val := list.Get(Int(-1)); !IsError(val) {
		t.Errorf("Should not have been able to read a negative index")
	}
	if val := list.Get(Int(3)); !IsError(val) {
		t.Errorf("Should not have been able to read beyond end of list")
	}
	if !IsError(list.Get(Uint(3))) {
		t.Error("Invalid index type did not result in error")
	}
}

func validateIterator123(t *testing.T, list traits.Lister) {
	t.Helper()
	it := list.Iterator()
	var i = int64(0)
	for ; it.HasNext() == True; i++ {
		elem := it.Next()
		if getElem(t, list, Int(i)) != elem {
			t.Errorf(
				"List iterator returned incorrect value: list[%d]: %v", i, elem)
		}
	}
	if it.Next() != nil {
		t.Errorf("List iterator attempted to continue beyond list size")
	}
	if i != 3 {
		t.Errorf("Iterator did not iterate until last value")
	}
}

func TestConcatListSizeCached(t *testing.T) {
	reg := newTestRegistry(t)
	// Build a deep chain of concat lists
	var list traits.Lister = NewDynamicList(reg, []int64{0})
	for i := 0; i < 200; i++ {
		list = list.Add(NewDynamicList(reg, []int64{0})).(traits.Lister)
	}
	// Size() should return instantly since it's precomputed
	size := list.Size()
	if size != Int(201) {
		t.Errorf("Expected size 201, got %v", size)
	}
	// Call Size() many times to confirm no quadratic behavior
	for i := 0; i < 1000; i++ {
		if list.Size() != Int(201) {
			t.Errorf("Size() returned inconsistent value on call %d", i)
		}
	}
}

func TestListCalculateSize(t *testing.T) {
	adapter := DefaultTypeAdapter

	// List literal: [1, [3, 4], [[7, 8], [9, 10]]]
	l1 := NewRefValList(adapter, []ref.Val{Int(3), Int(4)})
	l2_1 := NewRefValList(adapter, []ref.Val{Int(7), Int(8)})
	l2_2 := NewRefValList(adapter, []ref.Val{Int(9), Int(10)})
	l2 := NewRefValList(adapter, []ref.Val{l2_1, l2_2})
	nested := NewRefValList(adapter, []ref.Val{Int(1), l1, l2})

	tests := []struct {
		name string
		val  ref.Val
		want uint32
	}{
		{
			name: "empty_list",
			val:  NewRefValList(adapter, []ref.Val{}),
			want: 1,
		},
		{
			name: "flat_list",
			val:  l1,
			want: 3,
		},
		{
			name: "nested_list",
			val:  nested,
			want: 12,
		},
		{
			name: "concat_list",
			val:  l1.Add(l2_1),
			want: 6,
		},
		{
			name: "string_list",
			val:  NewStringList(adapter, []string{"hello", "world"}),
			want: 3, // 1 (container) + 1 ("hello" unit) + 1 ("world" unit) = 3
		},
		{
			name: "dynamic_list",
			val:  NewDynamicList(adapter, []any{int64(1), []int64{3, 4}}),
			want: 5,
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

func TestMaybeSliceList(t *testing.T) {
	adapter := DefaultTypeAdapter

	t.Run("nil_list", func(t *testing.T) {
		if _, ok := MaybeSliceList(adapter, nil, 0, 0); ok {
			t.Errorf("expected ok = false for nil list")
		}
	})

	t.Run("out_of_bounds", func(t *testing.T) {
		l := NewStringList(adapter, []string{"a", "b", "c"})
		bounds := [][2]int{
			{-1, 2},
			{0, 4},
			{2, 1},
			{0, -1},
		}
		for _, b := range bounds {
			if _, ok := MaybeSliceList(adapter, l, b[0], b[1]); ok {
				t.Errorf("MaybeSliceList(l, %d, %d) expected ok = false", b[0], b[1])
			}
		}
	})

	t.Run("string_slice", func(t *testing.T) {
		l := NewStringList(adapter, []string{"a", "b", "c", "d"})
		sub, ok := MaybeSliceList(adapter, l, 1, 3)
		if !ok {
			t.Fatalf("MaybeSliceList failed")
		}
		if sub.Size() != Int(2) {
			t.Errorf("got size %v, want 2", sub.Size())
		}
		want := []string{"b", "c"}
		if !reflect.DeepEqual(sub.Value(), want) {
			t.Errorf("got value %v, want %v", sub.Value(), want)
		}
	})

	t.Run("int_slice", func(t *testing.T) {
		l := NewDynamicList(adapter, []int{10, 20, 30, 40})
		sub, ok := MaybeSliceList(adapter, l, 0, 2)
		if !ok {
			t.Fatalf("MaybeSliceList failed")
		}
		want := []int{10, 20}
		if !reflect.DeepEqual(sub.Value(), want) {
			t.Errorf("got value %v, want %v", sub.Value(), want)
		}
	})

	t.Run("ref_val_slice", func(t *testing.T) {
		l := NewRefValList(adapter, []ref.Val{String("first"), String("second"), String("third")})
		sub, ok := MaybeSliceList(adapter, l, 1, 2)
		if !ok {
			t.Fatalf("MaybeSliceList failed")
		}
		want := []ref.Val{String("second")}
		if !reflect.DeepEqual(sub.Value(), want) {
			t.Errorf("got value %v, want %v", sub.Value(), want)
		}
	})
}

func TestMaybeReverseList(t *testing.T) {
	adapter := DefaultTypeAdapter

	t.Run("nil_list", func(t *testing.T) {
		if _, ok := MaybeReverseList(adapter, nil); ok {
			t.Errorf("expected ok = false for nil list")
		}
	})

	t.Run("empty_and_single", func(t *testing.T) {
		empty := NewStringList(adapter, []string{})
		revEmpty, ok := MaybeReverseList(adapter, empty)
		if !ok || revEmpty.Size() != Int(0) {
			t.Errorf("MaybeReverseList failed on empty list")
		}

		single := NewStringList(adapter, []string{"single"})
		revSingle, ok := MaybeReverseList(adapter, single)
		if !ok || revSingle.Size() != Int(1) {
			t.Errorf("MaybeReverseList failed on single-element list")
		}
	})

	t.Run("string_slice", func(t *testing.T) {
		l := NewStringList(adapter, []string{"a", "b", "c"})
		rev, ok := MaybeReverseList(adapter, l)
		if !ok {
			t.Fatalf("MaybeReverseList failed")
		}
		want := []string{"c", "b", "a"}
		if !reflect.DeepEqual(rev.Value(), want) {
			t.Errorf("got value %v, want %v", rev.Value(), want)
		}
	})

	t.Run("int_slice", func(t *testing.T) {
		l := NewDynamicList(adapter, []int{1, 2, 3, 4})
		rev, ok := MaybeReverseList(adapter, l)
		if !ok {
			t.Fatalf("MaybeReverseList failed")
		}
		want := []int{4, 3, 2, 1}
		if !reflect.DeepEqual(rev.Value(), want) {
			t.Errorf("got value %v, want %v", rev.Value(), want)
		}
	})

	t.Run("ref_val_slice", func(t *testing.T) {
		l := NewRefValList(adapter, []ref.Val{Int(100), Int(200)})
		rev, ok := MaybeReverseList(adapter, l)
		if !ok {
			t.Fatalf("MaybeReverseList failed")
		}
		want := []ref.Val{Int(200), Int(100)}
		if !reflect.DeepEqual(rev.Value(), want) {
			t.Errorf("got value %v, want %v", rev.Value(), want)
		}
	})
}

type int64Indexer interface {
	GetInt64Index(int64) (any, bool)
}

type dummyAggregateSizer struct{}

func (dummyAggregateSizer) AggregateSize(any) uint32 {
	return 1
}

type mockCustomList struct {
	traits.Lister
	val any
}

func (m mockCustomList) Value() any {
	return m.val
}

type mockContainsErrList struct {
	traits.Lister
	err ref.Val
}

func (m mockContainsErrList) Contains(ref.Val) ref.Val {
	return m.err
}

func (m mockContainsErrList) Size() ref.Val {
	return Int(1)
}

func (m mockContainsErrList) Get(ref.Val) ref.Val {
	return m.err
}

type customOnePtrStruct struct {
	P *int
}

type customMultiWordStruct struct {
	A int
	B string
}

type customZeroStruct struct{}

func TestList_ComprehensiveCoverage(t *testing.T) {
	adapter := DefaultTypeAdapter

	t.Run("NewDynamicList_AllSpecializations", func(t *testing.T) {
		// nil
		nl := NewDynamicList(adapter, nil)
		if nl.Size() != IntZero {
			t.Errorf("expected size 0 for nil, got %v", nl.Size())
		}

		// all slice types
		now := time.Now()
		dur := time.Minute

		vString := NewDynamicList(adapter, []string{"a", "b"})
		vRefVal := NewDynamicList(adapter, []ref.Val{Int(1), String("x")})
		vInt := NewDynamicList(adapter, []int{1, 2})
		vInt64 := NewDynamicList(adapter, []int64{1, 2})
		vInt32 := NewDynamicList(adapter, []int32{1, 2})
		vInt16 := NewDynamicList(adapter, []int16{1, 2})
		vInt8 := NewDynamicList(adapter, []int8{1, 2})
		vUint := NewDynamicList(adapter, []uint{1, 2})
		vUint64 := NewDynamicList(adapter, []uint64{1, 2})
		vUint32 := NewDynamicList(adapter, []uint32{1, 2})
		vUint16 := NewDynamicList(adapter, []uint16{1, 2})
		vUint8 := NewDynamicList(adapter, []uint8{1, 2})
		vFloat64 := NewDynamicList(adapter, []float64{1.0, 2.0})
		vFloat32 := NewDynamicList(adapter, []float32{1.0, 2.0})
		vBool := NewDynamicList(adapter, []bool{true, false})
		vAny := NewDynamicList(adapter, []any{1, "two"})
		vBytes := NewDynamicList(adapter, [][]byte{[]byte("hi"), []byte("bye")})
		vTime := NewDynamicList(adapter, []time.Time{now, now.Add(time.Hour)})
		vDuration := NewDynamicList(adapter, []time.Duration{dur, dur * 2})

		allLists := []traits.Lister{
			vString, vRefVal, vInt, vInt64, vInt32, vInt16, vInt8,
			vUint, vUint64, vUint32, vUint16, vUint8,
			vFloat64, vFloat32, vBool, vAny, vBytes, vTime, vDuration,
		}
		for _, l := range allLists {
			if l.Size() != Int(2) {
				t.Errorf("list %T has unexpected size %v", l, l.Size())
			}
		}

		// Dynamic slice meta: slice of structs
		sVal := []customMultiWordStruct{{1, "a"}, {2, "b"}}
		vStruct := NewDynamicList(adapter, sVal)
		if vStruct.Size() != Int(2) {
			t.Errorf("vStruct size got %v, want 2", vStruct.Size())
		}
		if str := vStruct.(fmt.Stringer).String(); str == "" {
			t.Errorf("vStruct string was empty")
		}
		// ConvertToNative on dynamic slice
		if nativeVal, err := vStruct.ConvertToNative(reflect.TypeOf([]customMultiWordStruct{})); err != nil || !reflect.DeepEqual(nativeVal, sVal) {
			t.Errorf("vStruct ConvertToNative got %v, %v", nativeVal, err)
		}

		// Dynamic slice meta: slice of pointers
		pVal := []*customMultiWordStruct{{1, "a"}, nil, {2, "b"}}
		vPtrStruct := NewDynamicList(adapter, pVal)
		if vPtrStruct.Size() != Int(3) {
			t.Errorf("vPtrStruct size got %v, want 3", vPtrStruct.Size())
		}
		if vPtrStruct.Get(Int(1)) != NullValue {
			t.Errorf("vPtrStruct[1] got %v, want NullValue", vPtrStruct.Get(Int(1)))
		}
		if raw, ok := vPtrStruct.(int64Indexer).GetInt64Index(1); !ok || raw != NullValue {
			t.Errorf("GetInt64Index(1) got %v, %v", raw, ok)
		}
		if raw, ok := vPtrStruct.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
			t.Errorf("GetInt64Index(0) got %v, %v", raw, ok)
		}
		// Fold on slice of pointers
		var pFoldCount int
		vPtrStruct.(traits.Foldable).Fold(&testListFolder{foldLimit: 10, folds: 0})
		vPtrStruct.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			pFoldCount++
			return true
		}})
		if pFoldCount != 3 {
			t.Errorf("vPtrStruct fold count got %d, want 3", pFoldCount)
		}
		// Fold early break on slice of pointers
		vPtrStruct.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})
		// Fold with null pointer early break
		vPtrStruct.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			if v == NullValue {
				return false
			}
			return true
		}})

		// Nil and empty dynamic slices
		var nilPtrSlice []*customMultiWordStruct
		vNilPtrSlice := NewDynamicList(adapter, nilPtrSlice)
		if vNilPtrSlice.Size() != IntZero || !vNilPtrSlice.(traits.Zeroer).IsZeroValue() {
			t.Errorf("vNilPtrSlice expected zero size")
		}
		if vNilPtrSlice.Value() == nil {
			// nativeSliceValue when isNilSlice
		}
		emptyPtrSlice := []*customMultiWordStruct{}
		vEmptyPtrSlice := NewDynamicList(adapter, emptyPtrSlice)
		if vEmptyPtrSlice.Size() != IntZero {
			t.Errorf("vEmptyPtrSlice expected zero size")
		}
		_ = vEmptyPtrSlice.Value()

		var nilStructSlice []customMultiWordStruct
		vNilStructSlice := NewDynamicList(adapter, nilStructSlice)
		_ = vNilStructSlice.Value()

		emptyStructSlice := []customMultiWordStruct{}
		vEmptyStructSlice := NewDynamicList(adapter, emptyStructSlice)
		_ = vEmptyStructSlice.Value()

		// Dynamic slice of struct fold & GetInt64Index
		var structFoldCount int
		vStruct.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			structFoldCount++
			return true
		}})
		if structFoldCount != 2 {
			t.Errorf("structFoldCount got %d, want 2", structFoldCount)
		}
		vStruct.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})
		if raw, ok := vStruct.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
			t.Errorf("vStruct GetInt64Index(0) got %v, %v", raw, ok)
		}

		// Fallback baseList from NewDynamicList (e.g. array)
		arr := [2]int{10, 20}
		vArr := NewDynamicList(adapter, arr)
		if vArr.Size() != Int(2) {
			t.Errorf("vArr size got %v, want 2", vArr.Size())
		}
		if vArr.Get(Int(0)) != Int(10) {
			t.Errorf("vArr[0] got %v, want 10", vArr.Get(Int(0)))
		}
	})

	t.Run("MaybeSliceList_And_MaybeReverseList_Edges", func(t *testing.T) {
		// MaybeSliceList
		if _, ok := MaybeSliceList(adapter, mockCustomList{val: nil}, 0, 0); ok {
			t.Errorf("MaybeSliceList on nil value expected false")
		}
		if _, ok := MaybeSliceList(adapter, mockCustomList{val: 123}, 0, 0); ok {
			t.Errorf("MaybeSliceList on non-slice value expected false")
		}

		// MaybeReverseList
		if _, ok := MaybeReverseList(adapter, mockCustomList{val: nil}); ok {
			t.Errorf("MaybeReverseList on nil value expected false")
		}
		if _, ok := MaybeReverseList(adapter, mockCustomList{val: 123}); ok {
			t.Errorf("MaybeReverseList on non-slice value expected false")
		}
	})

	t.Run("ElemTypePtrFor_And_StructSlices", func(t *testing.T) {
		// zeroStruct
		zList := NewList(adapter, []customZeroStruct{{}})
		if zList.Size() != Int(1) {
			t.Errorf("zList size got %v", zList.Size())
		}
		_ = zList.Get(Int(0))

		// onePtrStruct (direct interface)
		val := 42
		opList := NewList(adapter, []customOnePtrStruct{{P: &val}, {P: nil}})
		if opList.Size() != Int(2) {
			t.Errorf("opList size got %v", opList.Size())
		}
		_ = opList.Get(Int(0))
		if raw, ok := opList.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
			t.Errorf("opList GetInt64Index(0) got %v, %v", raw, ok)
		}
		var opFolds int
		opList.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			opFolds++
			return true
		}})
		if opFolds != 2 {
			t.Errorf("opList folds got %d, want 2", opFolds)
		}
		opList.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})

		// multiWordStruct (indirect interface)
		mwList := NewList(adapter, []customMultiWordStruct{{1, "a"}, {2, "b"}})
		if mwList.Size() != Int(2) {
			t.Errorf("mwList size got %v", mwList.Size())
		}
		_ = mwList.Get(Int(0))
		if raw, ok := mwList.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
			t.Errorf("mwList GetInt64Index(0) got %v, %v", raw, ok)
		}
		var mwFolds int
		mwList.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			mwFolds++
			return true
		}})
		if mwFolds != 2 {
			t.Errorf("mwList folds got %d, want 2", mwFolds)
		}
		mwList.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})
		// AggregateSize with elemTypePtr != nil
		mwList.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())
		mwList.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())
	})

	t.Run("BaseList_Full", func(t *testing.T) {
		arr := [3]any{10, "hello", 20.5}
		bl := NewDynamicList(adapter, arr)
		// Add non-lister
		if !IsError(bl.Add(String("err"))) {
			t.Errorf("baseList.Add non-lister expected error")
		}
		// Add lister
		added := bl.Add(NewDynamicList(adapter, [1]any{30}))
		if added.(traits.Lister).Size() != Int(4) {
			t.Errorf("baseList.Add lister size got %v", added.(traits.Lister).Size())
		}

		// ConvertToNative
		if anyNative, err := bl.ConvertToNative(reflect.TypeFor[any]()); err != nil || len(anyNative.([]any)) != 3 {
			t.Errorf("ConvertToNative any got %v, %v", anyNative, err)
		}
		if blNative, err := bl.ConvertToNative(reflect.TypeOf(bl)); err != nil || blNative != bl {
			t.Errorf("ConvertToNative baseList got %v, %v", blNative, err)
		}
		if _, err := bl.ConvertToNative(reflect.TypeOf(123)); err == nil {
			t.Errorf("ConvertToNative non-slice non-array expected error")
		}
		// Convert to array
		arrType := reflect.ArrayOf(3, reflect.TypeFor[any]())
		if arrVal, err := bl.ConvertToNative(arrType); err != nil || arrVal == nil {
			t.Errorf("ConvertToNative array got %v, %v", arrVal, err)
		}
		// Convert with element error
		if _, err := bl.ConvertToNative(reflect.TypeOf([]int{})); err == nil {
			t.Errorf("ConvertToNative with incompatible element expected error")
		}

		// ConvertToType
		if bl.ConvertToType(ListType) != bl {
			t.Errorf("ConvertToType(ListType) failed")
		}
		if bl.ConvertToType(TypeType) != ListType {
			t.Errorf("ConvertToType(TypeType) failed")
		}
		if !IsError(bl.ConvertToType(MapType)) {
			t.Errorf("ConvertToType(MapType) expected error")
		}

		// Equal
		if bl.Equal(String("not list")) != False {
			t.Errorf("baseList.Equal non-lister want false")
		}
		if bl.Equal(NewDynamicList(adapter, [1]any{10})) != False {
			t.Errorf("baseList.Equal different size want false")
		}
		if bl.Equal(NewDynamicList(adapter, [3]any{10, "world", 20.5})) != False {
			t.Errorf("baseList.Equal different element want false")
		}
		if bl.Equal(NewDynamicList(adapter, [3]any{10, "hello", 20.5})) != True {
			t.Errorf("baseList.Equal identical want true")
		}

		// Fold
		var blFolds int
		bl.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			blFolds++
			return true
		}})
		if blFolds != 3 {
			t.Errorf("baseList fold got %d, want 3", blFolds)
		}
		bl.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})

		// AggregateSize
		bl.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())
		// Memoized hit
		bl.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())
		// Non-cacheable sizer
		bl.(AggregateSizeVisitor).AggregateSize(dummyAggregateSizer{})

		// AggregateSize with value == nil
		nilBL := &baseList{Adapter: adapter, value: nil, size: 0}
		nilBL.AggregateSize(NewSizeCalculator())

		// AggregateSize with value as slice and memoization
		blSlice := &baseList{Adapter: adapter, value: []int{1, 2}, size: 2, get: func(i int) any { return i + 1 }}
		szCalc := NewSizeCalculator()
		szCalc.AggregateSize(blSlice)
		szCalc.AggregateSize(blSlice)

		// ConvertToNative errors for anyValueType and JSONValueType
		badList := NewList(adapter, []any{make(chan int)})
		if _, err := badList.ConvertToNative(anyValueType); err == nil {
			t.Errorf("expected error converting chan list to anyValueType")
		}
		if _, err := badList.ConvertToNative(JSONValueType); err == nil {
			t.Errorf("expected error converting chan list to JSONValueType")
		}

		// Type, String, format, Format
		if bl.Type() != ListType {
			t.Errorf("baseList.Type() got %v", bl.Type())
		}
		if bl.(fmt.Stringer).String() != "[10, hello, 20.5]" {
			t.Errorf("baseList.String() got %v", bl.(fmt.Stringer).String())
		}
		if formatted := Format(bl); formatted != `[10, "hello", 20.5]` {
			t.Errorf("Format(bl) got %v", formatted)
		}
	})

	t.Run("MutableList_Full", func(t *testing.T) {
		m1 := NewMutableList(adapter)
		m2 := NewMutableList(adapter)
		m2.Add(NewRefValList(adapter, []ref.Val{Int(1), Int(2)}))

		// Add *mutableList
		m1.Add(m2)
		if m1.Size() != Int(2) {
			t.Errorf("m1.Size() after adding mutableList got %v", m1.Size())
		}

		// Add traits.Lister
		m1.Add(NewRefValList(adapter, []ref.Val{Int(3)}))
		if m1.Size() != Int(3) {
			t.Errorf("m1.Size() after adding Lister got %v", m1.Size())
		}

		// Add non-lister
		if !IsError(m1.Add(String("err"))) {
			t.Errorf("m1.Add non-lister expected error")
		}

		// ToImmutableList
		imm := m1.ToImmutableList()
		if imm.Size() != Int(3) {
			t.Errorf("imm.Size() got %v", imm.Size())
		}
	})

	t.Run("ConcatList_Full", func(t *testing.T) {
		c1 := NewRefValList(adapter, []ref.Val{Int(1), Int(2)})
		c2 := NewRefValList(adapter, []ref.Val{Int(3), Int(4)})
		concat := c1.Add(c2).(*concatList)

		// Add non-lister
		if !IsError(concat.Add(String("err"))) {
			t.Errorf("concatList.Add non-lister expected error")
		}

		// Contains with error in prev
		errList := mockContainsErrList{err: NewErr("prev err")}
		concatWithErr := newConcatList(adapter, errList, c2).(*concatList)
		if !IsError(concatWithErr.Contains(Int(99))) {
			t.Errorf("concatList.Contains with error in prev expected error")
		}

		// Equal with non-lister, different size, error element
		if concat.Equal(String("err")) != False {
			t.Errorf("concat.Equal non-lister want false")
		}
		if concat.Equal(c1) != False {
			t.Errorf("concat.Equal different size want false")
		}
		errConcat1 := newConcatList(adapter, errList, c2).(*concatList)
		errConcat2 := newConcatList(adapter, errList, c2).(*concatList)
		if !IsError(errConcat1.Equal(errConcat2)) {
			t.Errorf("concat.Equal with error element expected error")
		}

		// Get with error index
		if !IsError(concat.Get(String("bad_index"))) {
			t.Errorf("concat.Get with bad index type expected error")
		}

		// IsZeroValue
		if concat.IsZeroValue() {
			t.Errorf("concat with 4 elems IsZeroValue want false")
		}

		// Fold
		var cFolds int
		concat.Fold(&testFuncFolder{fn: func(k, v any) bool {
			cFolds++
			return true
		}})
		if cFolds != 4 {
			t.Errorf("concat folds got %d, want 4", cFolds)
		}
		concat.Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})

		// AggregateSize memoization
		szCalc := NewSizeCalculator()
		szCalc.AggregateSize(concat)
		szCalc.AggregateSize(concat)

		// Type, Format
		if concat.Type() != ListType {
			t.Errorf("concat.Type() got %v", concat.Type())
		}
		if formatted := Format(concat); formatted != "[1, 2, 3, 4]" {
			t.Errorf("Format(concat) got %v", formatted)
		}
	})

	t.Run("SliceList_AllTypes_Operations", func(t *testing.T) {
		// Test sliceList for all integer / uint / float / bool / time types
		testSliceListType(t, adapter, []string{"hello", "world"}, String("hello"), String("nope"), String("hello"), String("hello"))
		testSliceListType(t, adapter, []int{10, 20}, Int(10), Int(99), Uint(10), Int(10))
		testSliceListType(t, adapter, []int64{10, 20}, Int(10), Int(99), Uint(10), Int(10))
		testSliceListType(t, adapter, []int32{10, 20}, Int(10), Int(99), Uint(10), Int(10))
		testSliceListType(t, adapter, []int16{10, 20}, Int(10), Int(99), Uint(10), Int(10))
		testSliceListType(t, adapter, []int8{10, 20}, Int(10), Int(99), Uint(10), Int(10))
		testSliceListType(t, adapter, []uint{10, 20}, Uint(10), Uint(99), Int(10), Uint(10))
		testSliceListType(t, adapter, []uint64{10, 20}, Uint(10), Uint(99), Int(10), Uint(10))
		testSliceListType(t, adapter, []uint32{10, 20}, Uint(10), Uint(99), Int(10), Uint(10))
		testSliceListType(t, adapter, []uint16{10, 20}, Uint(10), Uint(99), Int(10), Uint(10))
		testSliceListType(t, adapter, []uint8{10, 20}, Uint(10), Uint(99), Int(10), Uint(10))
		testSliceListType(t, adapter, []float64{10.0, 20.0}, Double(10.0), Double(99.0), Int(10), Double(10.0))
		testSliceListType(t, adapter, []float32{10.0, 20.0}, Double(10.0), Double(99.0), Int(10), Double(10.0))
		testSliceListType(t, adapter, []bool{true, false}, True, String("nope"), True, True)
		testSliceListType(t, adapter, [][]byte{[]byte("a"), []byte("b")}, Bytes("a"), Bytes("z"), Bytes("a"), Bytes("a"))
		now := time.Unix(1000, 0)
		testSliceListType(t, adapter, []time.Time{now, now.Add(time.Second)}, Timestamp{Time: now}, Timestamp{Time: now.Add(time.Hour)}, Timestamp{Time: now}, Timestamp{Time: now})
		testSliceListType(t, adapter, []time.Duration{time.Second, time.Minute}, Duration{Duration: time.Second}, Duration{Duration: time.Hour}, Duration{Duration: time.Second}, Duration{Duration: time.Second})

		// bool Contains mismatch with False on [true]
		lBoolOnlyTrue := NewList(adapter, []bool{true})
		if lBoolOnlyTrue.Contains(False) != False {
			t.Errorf("lBoolOnlyTrue Contains False want false")
		}

		// int32 out of range Int contains check
		lInt32 := NewList(adapter, []int32{10, 20})
		if lInt32.Contains(Int(math.MaxInt64)) != False {
			t.Errorf("lInt32 Contains MaxInt64 want false")
		}

		// sliceList Add non-lister
		lStr := NewList(adapter, []string{"hello"})
		if !IsError(lStr.Add(String("err"))) {
			t.Errorf("sliceList.Add non-lister expected error")
		}

		// sliceList Equal non-lister and different slice types
		if lStr.Equal(String("err")) != False {
			t.Errorf("sliceList.Equal non-lister want false")
		}
		if lStr.Equal(NewList(adapter, []any{"hello"})) != True {
			t.Errorf("sliceList.Equal different sliceList type want true")
		}
		if lStr.Equal(NewList(adapter, []any{"bye"})) != False {
			t.Errorf("sliceList.Equal different sliceList type mismatch want false")
		}

		// sliceList ConvertToType
		if lStr.ConvertToType(ListType) != lStr {
			t.Errorf("ConvertToType(ListType) failed")
		}
		if lStr.ConvertToType(TypeType) != ListType {
			t.Errorf("ConvertToType(TypeType) failed")
		}
		if !IsError(lStr.ConvertToType(MapType)) {
			t.Errorf("ConvertToType(MapType) expected error")
		}

		// sliceList GetInt64Index out of bounds & qualifyRawVal
		indexer := lStr.(int64Indexer)
		if _, ok := indexer.GetInt64Index(-1); ok {
			t.Errorf("GetInt64Index(-1) want ok=false")
		}
		if _, ok := indexer.GetInt64Index(5); ok {
			t.Errorf("GetInt64Index(5) want ok=false")
		}
		if raw, ok := indexer.GetInt64Index(0); !ok || raw != String("hello") {
			t.Errorf("GetInt64Index(0) got %v, %v", raw, ok)
		}

		// GetInt64Index on pointer struct list (isQualifyRawStruct = true, elemTypePtr = nil)
		ptrList := NewList(adapter, []*customMultiWordStruct{{A: 1, B: "a"}})
		if raw, ok := ptrList.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
			t.Errorf("GetInt64Index on pointer struct list failed")
		}

		// sliceList generic fallback (e.g. complex128)
		lComplex := NewList(adapter, []complex128{complex(1, 2), complex(3, 4)})
		if lComplex.Size() != Int(2) {
			t.Errorf("lComplex size got %v", lComplex.Size())
		}
		if raw, ok := lComplex.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
			t.Errorf("lComplex GetInt64Index(0) got %v, %v", raw, ok)
		}
		var cplxFolds int
		lComplex.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			cplxFolds++
			return true
		}})
		if cplxFolds != 2 {
			t.Errorf("lComplex folds got %d, want 2", cplxFolds)
		}
		lComplex.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
			return false
		}})
		lComplex.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())
		lComplex.(AggregateSizeVisitor).AggregateSize(NewSizeCalculator())

		// Format
		if formatted := Format(lStr); formatted != `["hello"]` {
			t.Errorf("Format(lStr) got %v", formatted)
		}
	})
}

type testFuncFolder struct {
	fn func(k, v any) bool
}

func (f *testFuncFolder) FoldEntry(k, v any) bool {
	return f.fn(k, v)
}

func testSliceListType[T any](t *testing.T, adapter Adapter, slice []T, matchVal, mismatchVal, altMatchVal, get0Val ref.Val) {
	t.Helper()
	l := NewList(adapter, slice)

	// Size, IsZeroValue
	if l.Size() != Int(len(slice)) {
		t.Errorf("Size() got %v, want %d", l.Size(), len(slice))
	}
	if l.(traits.Zeroer).IsZeroValue() {
		t.Errorf("IsZeroValue() got true, want false")
	}

	// Get & GetInt64Index
	if Equal(l.Get(Int(0)), get0Val) != True {
		t.Errorf("Get(0) got %v, want %v", l.Get(Int(0)), get0Val)
	}
	if raw, ok := l.(int64Indexer).GetInt64Index(0); !ok || raw == nil {
		t.Errorf("GetInt64Index(0) got %v, %v", raw, ok)
	}

	// Contains
	if l.Contains(matchVal) != True {
		t.Errorf("Contains(%v) got false, want true", matchVal)
	}
	if l.Contains(mismatchVal) != False {
		t.Errorf("Contains(%v) got true, want false", mismatchVal)
	}
	if altMatchVal != nil && l.Contains(altMatchVal) != True {
		t.Errorf("Contains(%v) alt got false, want true", altMatchVal)
	}

	// Fold
	var folds int
	l.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
		folds++
		return true
	}})
	if folds != len(slice) {
		t.Errorf("Fold() count got %d, want %d", folds, len(slice))
	}
	// Early break fold
	l.(traits.Foldable).Fold(&testFuncFolder{fn: func(k, v any) bool {
		return false
	}})

	// Equal
	if l.Equal(l) != True {
		t.Errorf("Equal(l) got false, want true")
	}
	lSame := NewList(adapter, slice)
	if l.Equal(lSame) != True {
		t.Errorf("Equal(lSame) got false, want true")
	}
	if len(slice) > 1 {
		diffSlice := make([]T, len(slice))
		copy(diffSlice, slice)
		diffSlice[0], diffSlice[1] = diffSlice[1], diffSlice[0]
		lDiff := NewList(adapter, diffSlice)
		if l.Equal(lDiff) != False {
			t.Errorf("Equal(lDiff) got true, want false")
		}
	}

	// AggregateSize
	if sizer, ok := l.(AggregateSizeVisitor); ok {
		sizer.AggregateSize(NewSizeCalculator())
		sizer.AggregateSize(NewSizeCalculator())
	}

	// ConvertToNative
	if native, err := l.ConvertToNative(reflect.TypeOf(slice)); err != nil || !reflect.DeepEqual(native, slice) {
		t.Errorf("ConvertToNative got %v, %v", native, err)
	}

	// Iterator
	it := l.Iterator()
	var itCount int
	for it.HasNext() == True {
		_ = it.Next()
		itCount++
	}
	if itCount != len(slice) {
		t.Errorf("Iterator count got %d, want %d", itCount, len(slice))
	}
	if it.Next() != nil {
		t.Errorf("Iterator Next past end returned non-nil")
	}
}
