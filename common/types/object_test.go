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
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

func TestNewProtoObject(t *testing.T) {
	reg := newTestRegistry(t)
	parsedExpr := &exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3}}}
	reg.RegisterMessage(parsedExpr)
	obj := reg.NativeToValue(parsedExpr).(traits.Indexer)
	si := obj.Get(String("source_info")).(traits.Indexer)
	lo := si.Get(String("line_offsets")).(traits.Indexer)
	if lo.Get(Int(2)).Equal(Int(3)) != True {
		t.Errorf("Could not select fields by their proto type names")
	}
	expr := obj.Get(String("expr")).(traits.Indexer)
	call := expr.Get(String("call_expr")).(traits.Indexer)
	if call.Get(String("function")).Equal(String("")) != True {
		t.Errorf("Could not traverse through default values for unset fields")
	}
}

func TestProtoObjectConvertToNative(t *testing.T) {
	msg := &exprpb.ParsedExpr{
		Expr: &exprpb.Expr{
			Id: 1,
			ExprKind: &exprpb.Expr_ConstExpr{
				ConstExpr: &exprpb.Constant{
					ConstantKind: &exprpb.Constant_BoolValue{
						BoolValue: true,
					},
				},
			},
		},
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3}}}

	tests := []struct {
		opts     []RegistryOption
		fieldMap func(reg *Registry) map[string]ref.Val
		outValue map[string]any
	}{
		{
			opts: []RegistryOption{ProtoTypeDefs(&exprpb.Expr{}), JSONFieldNames(true)},
			fieldMap: func(reg *Registry) map[string]ref.Val {
				return map[string]ref.Val{
					"expr":       reg.NativeToValue(msg.GetExpr()),
					"sourceInfo": reg.NativeToValue(msg.GetSourceInfo()),
				}
			},
			outValue: map[string]any{
				"expr": map[string]any{
					// The id is encoded to a string because it is int64 type, though
					// this is not exactly what's documented in protojson serialization
					// and so could signal a bug in the json-handling for int values
					// as the serialization to int or string is based on the value precision.
					"id": "1",
					"constExpr": map[string]any{
						"boolValue": true,
					},
				},
				"sourceInfo": map[string]any{
					"lineOffsets": []any{1.0, 2.0, 3.0},
				},
			},
		},
		{
			opts: []RegistryOption{ProtoTypeDefs(&exprpb.Expr{}), JSONFieldNames(false)},
			fieldMap: func(reg *Registry) map[string]ref.Val {
				return map[string]ref.Val{
					"expr":        reg.NativeToValue(msg.GetExpr()),
					"source_info": reg.NativeToValue(msg.GetSourceInfo()),
				}
			},
			outValue: map[string]any{
				"expr": map[string]any{
					"id": "1",
					"constExpr": map[string]any{
						"boolValue": true,
					},
				},
				"sourceInfo": map[string]any{
					"lineOffsets": []any{1.0, 2.0, 3.0},
				},
			},
		},
	}

	for _, tst := range tests {
		reg := newTestRegistry(t, tst.opts...)
		objVal := reg.NativeToValue(msg)

		// Proto Message
		val, err := objVal.ConvertToNative(reflect.TypeOf(&exprpb.ParsedExpr{}))
		if err != nil {
			t.Error(err)
		}
		if !proto.Equal(val.(proto.Message), msg) {
			t.Errorf("Messages were not equal, expect '%v', got '%v'", objVal.Value(), msg)
		}

		// Dynamic protobuf
		dynPB := reg.NewValue(
			string(msg.ProtoReflect().Descriptor().FullName()),
			tst.fieldMap(reg),
		)
		if IsError(dynPB) {
			t.Fatalf("reg.NewValue() failed: %v", dynPB)
		}
		dynVal := reg.NativeToValue(dynPB)
		val, err = dynVal.ConvertToNative(reflect.TypeOf(msg))
		if err != nil {
			t.Fatalf("dynVal.ConvertToNative() failed: %v", err)
		}
		if !proto.Equal(val.(proto.Message), msg) {
			t.Errorf("Messages were not equal, expect '%v', got '%v'", objVal.Value(), msg)
		}

		// google.protobuf.Any
		anyVal, err := objVal.ConvertToNative(anyValueType)
		if err != nil {
			t.Fatalf("objVal.ConvertToNative() failed: %v", err)
		}
		anyMsg := anyVal.(*anypb.Any)
		unpackedAny, err := anyMsg.UnmarshalNew()
		if err != nil {
			t.Fatalf("UnmarshalNew() failed: %v", err)
		}
		if !proto.Equal(unpackedAny, objVal.Value().(proto.Message)) {
			t.Errorf("Messages were not equal, expect '%v', got '%v'", objVal.Value(), unpackedAny)
		}

		// JSON
		jsonVal, err := objVal.ConvertToNative(JSONValueType)
		if err != nil {
			t.Fatalf("objVal.ConvertToNative(%v) failed: %v", JSONValueType, err)
		}
		jsonBytes, err := protojson.Marshal(jsonVal.(proto.Message))
		jsonTxt := string(jsonBytes)
		if err != nil {
			t.Fatalf("protojson.Marshal(%v) failed: %v", jsonVal, err)
		}
		outMap := map[string]any{}
		err = json.Unmarshal(jsonBytes, &outMap)
		if err != nil {
			t.Fatalf("json.Unmarshal(%q) failed: %v", jsonTxt, err)
		}
		want := tst.outValue
		if !reflect.DeepEqual(outMap, want) {
			t.Errorf("got json '%v', expected %v", outMap, want)
		}
	}
}

func TestProtoObjectIsSet(t *testing.T) {
	msg := &exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3},
		},
	}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	objVal := reg.NativeToValue(msg).(*protoObj)
	if objVal.IsSet(String("source_info")) != True {
		t.Error("got 'source_info' not set, wanted set")
	}
	if objVal.IsSet(String("expr")) != False {
		t.Error("got 'expr' set, wanted not set")
	}
	if !IsError(objVal.IsSet(String("bad_field"))) {
		t.Error("got 'bad_field' wanted error")
	}
	if !IsError(objVal.IsSet(IntZero)) {
		t.Error("got field '0' wanted error")
	}
}

func TestProtoObjectIsZeroValue(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))
	emptyObj := reg.NativeToValue(&exprpb.ParsedExpr{})
	pb, ok := emptyObj.(traits.Zeroer)
	if !ok {
		t.Fatal("Proto object is not a traits.Zeroer")
	}
	if !pb.IsZeroValue() {
		t.Error("pb.IsZeroValue() got false, wanted true")
	}
	obj := reg.NativeToValue(&exprpb.Expr{ExprKind: &exprpb.Expr_CallExpr{}})
	pb = obj.(traits.Zeroer)
	if pb.IsZeroValue() {
		t.Error("pb.IsZeroValue() got true, wanted false")
	}
}

func TestProtoObjectGet(t *testing.T) {
	msg := &exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3},
		},
	}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	objVal := reg.NativeToValue(msg).(*protoObj)
	if objVal.Get(String("source_info")).Equal(reg.NativeToValue(msg.GetSourceInfo())) != True {
		t.Error("could not get 'source_info'")
	}
	if objVal.Get(String("expr")).Equal(reg.NativeToValue(&exprpb.Expr{})) != True {
		t.Errorf("did not get 'expr' default value: %v", objVal.Get(String("expr")))
	}
	if !IsError(objVal.Get(String("bad_field"))) {
		t.Error("got 'bad_field' wanted error")
	}
	if !IsError(objVal.Get(IntZero)) {
		t.Error("got field '0' wanted error")
	}
}

func TestProtoObjectConvertToType(t *testing.T) {
	msg := &exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3},
		},
	}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	objVal := reg.NativeToValue(msg)
	tv := objVal.Type().(ref.Val)
	if objVal.ConvertToType(TypeType).Equal(tv) != True {
		t.Errorf("got non-type value: %v, wanted objet type", objVal.ConvertToType(TypeType))
	}
	if objVal.ConvertToType(objVal.Type()) != objVal {
		t.Error("identity type conversion failed")
	}
}

func TestProtoObjectCalculateSize(t *testing.T) {
	msg := &exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3},
		},
	}
	reg := newTestRegistry(t, ProtoTypeDefs(msg))
	objVal := reg.NativeToValue(msg)
	sizer, ok := objVal.(AggregateSizeVisitor)
	if !ok {
		t.Fatalf("expected AggregateSizeVisitor implementation for protoObj")
	}
	// 1 (protoObj container) + SourceInfo field (1 container + 1 list container + 3 list elements = 5) = 6
	if got := sizer.AggregateSize(NewSizeCalculator()); got != 6 {
		t.Errorf("got aggregate size %d, want 6", got)
	}

	nilObj := &protoObj{}
	if got := nilObj.AggregateSize(NewSizeCalculator()); got != 0 {
		t.Errorf("got nil protoObj aggregate size %d, want 0", got)
	}
}
