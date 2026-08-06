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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"

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
		Value().([]any)
	expected := []any{
		float64(1.0),
		float64(2.0),
		string("3"),
		float64(1.0),
		float64(2.0)}
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
			want: 11,
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
