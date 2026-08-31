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
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"google.golang.org/protobuf/proto"

	proto3pb "cel.dev/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	anypb "google.golang.org/protobuf/types/known/anypb"
	dpb "google.golang.org/protobuf/types/known/durationpb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

func TestRegistryCopy(t *testing.T) {
	tests := []struct {
		name string
		reg  *Registry
	}{
		{
			name: "empty",
			reg:  NewEmptyRegistry(),
		},
		{
			name: "populated",
			reg:  newTestRegistry(t),
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			reg2 := tc.reg.Copy()
			if !reflect.DeepEqual(tc.reg, reg2) {
				t.Fatal("type registry copy did not produce equivalent values.")
			}
		})
	}

	t.Run("nil registry", func(t *testing.T) {
		var reg *Registry
		if reg.Copy() != nil {
			t.Error("expected nil registry copy to return nil")
		}
	})
}

func assertShared(t *testing.T, reg *Registry) {
	t.Helper()
	if !reg.shared.Load() {
		t.Errorf("registry.shared = false, want true")
	}
}

func assertUnshared(t *testing.T, reg *Registry) {
	t.Helper()
	if reg.shared.Load() {
		t.Errorf("registry.shared = true, want false")
	}
}

func newSharedRegistryPair(t *testing.T, opts ...RegistryOption) (*Registry, *Registry) {
	t.Helper()
	reg := newTestRegistry(t, opts...)
	copied := reg.Copy()
	assertShared(t, reg)
	assertShared(t, copied)
	return reg, copied
}

func TestRegistrySharedOnCopy(t *testing.T) {
	reg := NewEmptyRegistry()
	assertUnshared(t, reg)

	copied := reg.Copy()
	assertShared(t, reg)
	assertShared(t, copied)

	if !reflect.DeepEqual(reg, copied) {
		t.Errorf("reg.Copy() expected equivalent registries")
	}
}

func TestRegistryUnshared_RegisterTypeOnCopy(t *testing.T) {
	reg, copied := newSharedRegistryPair(t)

	customType := NewObjectType("custom.TypeA")
	if err := copied.RegisterType(customType); err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}

	assertUnshared(t, copied)
	assertShared(t, reg)

	if _, found := copied.FindIdent("custom.TypeA"); !found {
		t.Errorf("copied.FindIdent('custom.TypeA') expected found == true")
	}
	if _, found := reg.FindIdent("custom.TypeA"); found {
		t.Errorf("reg.FindIdent('custom.TypeA') expected found == false after mutating copy")
	}

	// Subsequent mutation on already unshared copy stays unshared
	customTypeB := NewObjectType("custom.TypeB")
	if err := copied.RegisterType(customTypeB); err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}
	assertUnshared(t, copied)
	if _, found := copied.FindIdent("custom.TypeB"); !found {
		t.Errorf("copied.FindIdent('custom.TypeB') expected found == true")
	}
	if _, found := reg.FindIdent("custom.TypeB"); found {
		t.Errorf("reg.FindIdent('custom.TypeB') expected found == false")
	}
}

func TestRegistryUnshared_RegisterTypeOnOriginal(t *testing.T) {
	reg, copied := newSharedRegistryPair(t)

	customType := NewObjectType("custom.TypeOrig")
	if err := reg.RegisterType(customType); err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}

	assertUnshared(t, reg)
	assertShared(t, copied)

	if _, found := reg.FindIdent("custom.TypeOrig"); !found {
		t.Errorf("reg.FindIdent('custom.TypeOrig') expected found == true")
	}
	if _, found := copied.FindIdent("custom.TypeOrig"); found {
		t.Errorf("copied.FindIdent('custom.TypeOrig') expected found == false after mutating original")
	}
}

func TestRegistryUnshared_RegisterMessage(t *testing.T) {
	reg, copied := newSharedRegistryPair(t)

	if err := copied.RegisterMessage(&proto3pb.TestAllTypes{}); err != nil {
		t.Fatalf("RegisterMessage() failed: %v", err)
	}

	assertUnshared(t, copied)
	assertShared(t, reg)

	if _, found := copied.FindStructType("google.expr.proto3.test.TestAllTypes"); !found {
		t.Errorf("copied.FindStructType() expected found == true")
	}
	if _, found := reg.FindStructType("google.expr.proto3.test.TestAllTypes"); found {
		t.Errorf("reg.FindStructType() expected found == false")
	}
}

func TestRegistryUnshared_RegisterDescriptor(t *testing.T) {
	reg, copied := newSharedRegistryPair(t)

	err := copied.RegisterDescriptor(proto3pb.GlobalEnum_GOO.Descriptor().ParentFile())
	if err != nil {
		t.Fatalf("RegisterDescriptor() failed: %v", err)
	}

	assertUnshared(t, copied)
	assertShared(t, reg)

	enumVal := copied.EnumValue("google.expr.proto3.test.GlobalEnum.GOO")
	if IsError(enumVal) || enumVal.(Int) != Int(proto3pb.GlobalEnum_GOO.Number()) {
		t.Errorf("copied.EnumValue() got %v, wanted %v", enumVal, proto3pb.GlobalEnum_GOO.Number())
	}
	origEnumVal := reg.EnumValue("google.expr.proto3.test.GlobalEnum.GOO")
	if !IsError(origEnumVal) {
		t.Errorf("reg.EnumValue() expected error, got %v", origEnumVal)
	}
}

func TestRegistryUnshared_WithJSONFieldNames(t *testing.T) {
	reg, copied := newSharedRegistryPair(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))

	if err := copied.WithJSONFieldNames(true); err != nil {
		t.Fatalf("WithJSONFieldNames() failed: %v", err)
	}

	assertUnshared(t, copied)
	assertShared(t, reg)

	if !copied.JSONFieldNames() {
		t.Errorf("copied.JSONFieldNames() expected true, got false")
	}
	if reg.JSONFieldNames() {
		t.Errorf("reg.JSONFieldNames() expected false, got true")
	}
}

func TestRegistryUnshared_ChainedCopies(t *testing.T) {
	r1 := NewEmptyRegistry()
	r2 := r1.Copy()
	r3 := r2.Copy()

	assertShared(t, r1)
	assertShared(t, r2)
	assertShared(t, r3)

	typeInR2 := NewObjectType("custom.InR2")
	if err := r2.RegisterType(typeInR2); err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}

	assertUnshared(t, r2)
	assertShared(t, r1)
	assertShared(t, r3)

	if _, found := r2.FindIdent("custom.InR2"); !found {
		t.Errorf("r2.FindIdent('custom.InR2') expected found == true")
	}
	if _, found := r1.FindIdent("custom.InR2"); found {
		t.Errorf("r1.FindIdent('custom.InR2') expected found == false")
	}
	if _, found := r3.FindIdent("custom.InR2"); found {
		t.Errorf("r3.FindIdent('custom.InR2') expected found == false")
	}

	typeInR3 := NewObjectType("custom.InR3")
	if err := r3.RegisterType(typeInR3); err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}

	assertUnshared(t, r3)
	if _, found := r3.FindIdent("custom.InR3"); !found {
		t.Errorf("r3.FindIdent('custom.InR3') expected found == true")
	}
	if _, found := r1.FindIdent("custom.InR3"); found {
		t.Errorf("r1.FindIdent('custom.InR3') expected found == false")
	}
	if _, found := r2.FindIdent("custom.InR3"); found {
		t.Errorf("r2.FindIdent('custom.InR3') expected found == false")
	}
}

func TestRegistryConcurrentCopy(t *testing.T) {
	reg := NewEmptyRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reg.Copy()
		}()
	}
	wg.Wait()
}

func TestRegistryRegisterType(t *testing.T) {
	tests := []struct {
		name    string
		types   []ref.Type
		wantErr bool
	}{
		{
			name: "differing type definitions same name",
			types: []ref.Type{
				NewTypeValue("http.Request", traits.ReceiverType),
				NewObjectType("http.Request", traits.ReceiverType),
			},
			wantErr: true,
		},
		{
			name: "equivalent opaque types no conflict",
			types: []ref.Type{
				NewOpaqueType("http.Request", NewTypeParamType("T")),
				NewOpaqueType("http.Request", NewTypeParamType("V")),
			},
			wantErr: false,
		},
		{
			name: "differing opaque types conflict",
			types: []ref.Type{
				NewOpaqueType("http.Request", NewTypeParamType("T"), NewTypeParamType("V")),
				NewOpaqueType("http.Request", NewTypeParamType("V")),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			reg := newTestRegistry(t)
			err := reg.RegisterType(tc.types...)
			if (err != nil) != tc.wantErr {
				t.Errorf("RegisterType() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRegistryEnumValue(t *testing.T) {
	reg := newTestRegistry(t)
	err := reg.RegisterDescriptor(proto3pb.GlobalEnum_GOO.Descriptor().ParentFile())
	if err != nil {
		t.Fatalf("RegisterDescriptor() failed: %v", err)
	}

	t.Run("EnumValue", func(t *testing.T) {
		enumVal := reg.EnumValue("google.expr.proto3.test.GlobalEnum.GOO")
		if IsError(enumVal) || Int(proto3pb.GlobalEnum_GOO.Number()) != enumVal.(Int) {
			t.Errorf("enum values were not equal between registry and proto: %v", enumVal)
		}
	})

	t.Run("FindIdent", func(t *testing.T) {
		enumVal := reg.EnumValue("google.expr.proto3.test.GlobalEnum.GOO")
		enumVal2, found := reg.FindIdent("google.expr.proto3.test.GlobalEnum.GOO")
		if !found {
			t.Fatal("Ident not found google.expr.proto3.test.GlobalEnum.GOO")
		}
		if enumVal.(Int) != enumVal2.(Int) {
			t.Errorf("got enum value %v, wanted %v", enumVal2, enumVal)
		}
	})
}

func TestRegistryFindStructType(t *testing.T) {
	reg := newTestRegistry(t)
	err := reg.RegisterDescriptor(proto3pb.GlobalEnum_GOO.Descriptor().ParentFile())
	if err != nil {
		t.Fatalf("RegisterDescriptor() failed: %v", err)
	}

	tests := []struct {
		typeName  string
		wantFound bool
	}{
		{
			typeName:  ".google.expr.proto3.test.TestAllTypes",
			wantFound: true,
		},
		{
			typeName:  ".google.expr.proto3.test.TestAllTypesUndefined",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.typeName, func(t *testing.T) {
			exprType, foundType := reg.FindType(tc.typeName)
			celType, foundStruct := reg.FindStructType(tc.typeName)

			if foundType != tc.wantFound {
				t.Errorf("FindType(%q) found = %v, want %v", tc.typeName, foundType, tc.wantFound)
			}
			if foundStruct != tc.wantFound {
				t.Errorf("FindStructType(%q) found = %v, want %v", tc.typeName, foundStruct, tc.wantFound)
			}

			if tc.wantFound {
				exprConvType, err := ExprTypeToType(exprType)
				if err != nil {
					t.Fatalf("ExprTypeToType(%v) failed: %v", exprType, err)
				}
				if !exprConvType.IsExactType(celType) {
					t.Errorf("Got %v type, wanted %v", exprConvType, celType)
				}
			}
		})
	}
}

func TestRegistryFindStructFieldNames(t *testing.T) {
	tests := []struct {
		name           string
		typeName       string
		fields         []string
		jsonFieldNames bool
	}{
		{
			name:     "Reference",
			typeName: "google.api.expr.v1alpha1.Reference",
			fields:   []string{"name", "overload_id", "value"},
		},
		{
			name:     "Decl",
			typeName: "google.api.expr.v1alpha1.Decl",
			fields:   []string{"name", "ident", "function"},
		},
		{
			name:     "invalid type",
			typeName: "invalid.TypeName",
			fields:   []string{},
		},
		{
			name:           "Reference JSON field names",
			typeName:       "google.api.expr.v1alpha1.Reference",
			fields:         []string{"name", "overloadId", "value"},
			jsonFieldNames: true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			reg := newTestRegistry(t,
				ProtoTypeDefs(&exprpb.Decl{}, &exprpb.Reference{}),
				JSONFieldNames(tc.jsonFieldNames))
			fields, _ := reg.FindStructFieldNames(tc.typeName)
			sort.Strings(fields)
			sort.Strings(tc.fields)
			if !reflect.DeepEqual(fields, tc.fields) {
				t.Errorf("got %v, wanted %v", fields, tc.fields)
			}
		})
	}
}

func TestRegistryFindStructFieldType(t *testing.T) {
	msgTypeName := ".google.expr.proto3.test.TestAllTypes"
	tests := []struct {
		typeName       string
		field          string
		found          bool
		jsonFieldNames bool
	}{
		{
			typeName: msgTypeName,
			field:    "single_bool",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "single_nested_message",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "standalone_enum",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "single_duration",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "single_timestamp",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "single_any",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "single_int64_wrapper",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "repeated_bool",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "map_string_string",
			found:    true,
		},
		{
			typeName: msgTypeName,
			field:    "double_bool",
			found:    false,
		},
		{
			typeName: msgTypeName + "Undefined",
			field:    "map_string_string",
			found:    false,
		},
		{
			typeName:       msgTypeName,
			field:          "mapStringString",
			found:          true,
			jsonFieldNames: true,
		},
		{
			typeName:       msgTypeName,
			field:          "map_string_string",
			found:          true,
			jsonFieldNames: true,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%s.%s", tc.typeName, tc.field), func(t *testing.T) {
			reg := newTestRegistry(t, JSONFieldNames(tc.jsonFieldNames))
			err := reg.RegisterDescriptor(proto3pb.GlobalEnum_GOO.Descriptor().ParentFile())
			if err != nil {
				t.Fatalf("RegisterDescriptor() failed: %v", err)
			}
			// When the field is expected to be found, test parity of the results
			if tc.found {
				refField, found := reg.FindFieldType(tc.typeName, tc.field)
				if !found {
					t.Fatalf("FindFieldType() did not find: %s.%s", tc.typeName, tc.field)
				}
				celField, found := reg.FindStructFieldType(tc.typeName, tc.field)
				if !found {
					t.Fatalf("FindStructFieldType() found: %s.%s", tc.typeName, tc.field)
				}
				convCelFieldType, err := ExprTypeToType(refField.Type)
				if err != nil {
					t.Fatalf("ExprTypeToType(%v) failed: %v", refField.Type, err)
				}
				if !convCelFieldType.IsExactType(celField.Type) {
					t.Errorf("Got %v type, wanted %v", convCelFieldType, celField.Type)
				}
				return
			}
			// When the field is not expected to be round ensure both return not found.
			if !tc.found {
				_, found := reg.FindFieldType(tc.typeName, tc.field)
				if found {
					t.Errorf("FindFieldType() found: %s.%s", tc.typeName, tc.field)
				}
				_, found = reg.FindStructFieldType(tc.typeName, tc.field)
				if found {
					t.Errorf("FindStructFieldType() found: %s.%s", tc.typeName, tc.field)
				}
			}
		})
	}
}

func TestRegistryNewValue(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}, &exprpb.SourceInfo{}))
	tests := []struct {
		typeName string
		fields   map[string]ref.Val
		out      proto.Message
	}{
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields:   map[string]ref.Val{},
			out:      &proto3pb.TestAllTypes{},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"standalone_enum": Int(1),
			},
			out: &proto3pb.TestAllTypes{
				StandaloneEnum: proto3pb.TestAllTypes_BAR,
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"single_int32_wrapper": Int(123),
				"single_int64_wrapper": NullValue,
			},
			out: &proto3pb.TestAllTypes{
				SingleInt32Wrapper: wrapperspb.Int32(123),
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"repeated_int64": reg.NativeToValue([]int64{3, 2, 1}),
			},
			out: &proto3pb.TestAllTypes{
				RepeatedInt64: []int64{3, 2, 1},
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"single_nested_enum": Int(2),
			},
			out: &proto3pb.TestAllTypes{
				NestedType: &proto3pb.TestAllTypes_SingleNestedEnum{
					SingleNestedEnum: proto3pb.TestAllTypes_BAZ,
				},
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"single_value": True,
			},
			out: &proto3pb.TestAllTypes{
				SingleValue: structpb.NewBoolValue(true),
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"single_value": reg.NativeToValue([]any{"hello", 10.2}),
			},
			out: &proto3pb.TestAllTypes{
				SingleValue: structpb.NewListValue(
					&structpb.ListValue{
						Values: []*structpb.Value{
							structpb.NewStringValue("hello"),
							structpb.NewNumberValue(10.2),
						},
					},
				),
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"repeated_nested_message": reg.NativeToValue([]any{
					&proto3pb.TestAllTypes_NestedMessage{Bb: 123},
				}),
			},
			out: &proto3pb.TestAllTypes{
				RepeatedNestedMessage: []*proto3pb.TestAllTypes_NestedMessage{{Bb: 123}},
			},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"map_int64_nested_type": reg.NativeToValue(map[int64]any{
					1234: &proto3pb.NestedTestAllTypes{Payload: &proto3pb.TestAllTypes{SingleInt32: 1234}},
				}),
			},
			out: &proto3pb.TestAllTypes{
				MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
					1234: {Payload: &proto3pb.TestAllTypes{SingleInt32: 1234}},
				},
			},
		},
		{
			typeName: "google.api.expr.v1alpha1.SourceInfo",
			fields: map[string]ref.Val{
				"location":     String("TestRegistryNewValue"),
				"line_offsets": reg.NativeToValue([]int64{0, 2}),
				"positions":    reg.NativeToValue(map[int64]int64{1: 2, 2: 4}),
			},
			out: &exprpb.SourceInfo{
				Location:    "TestRegistryNewValue",
				LineOffsets: []int32{0, 2},
				Positions:   map[int64]int32{1: 2, 2: 4},
			},
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			out := reg.NewValue(tc.typeName, tc.fields)
			if IsError(out) {
				t.Fatalf("reg.NewValue(%s, %v) failed: %v", tc.typeName, tc.fields, out)
			}
			if !proto.Equal(tc.out, out.Value().(proto.Message)) {
				t.Errorf("reg.NewValue() got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestRegistryNewValueErrors(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}, &exprpb.SourceInfo{}))
	tests := []struct {
		typeName string
		fields   map[string]ref.Val
		err      string
	}{
		{
			typeName: "google.expr.proto3.test.TestAllType",
			fields:   map[string]ref.Val{},
			err:      "unknown type",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"undefined": Int(1),
			},
			err: "no such field",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"single_int32_wrapper": True,
			},
			err: "type conversion error",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"repeated_int64": reg.NativeToValue([]float64{1.0, 2.3}),
			},
			err: "type conversion error",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"repeated_int64": Int(10),
			},
			err: "unsupported field type",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"map_string_string": NullValue,
			},
			err: "unsupported field type",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"map_string_string": reg.NativeToValue(map[string]int{"hello": 1}),
			},
			err: "type conversion error",
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes",
			fields: map[string]ref.Val{
				"map_string_string": reg.NativeToValue(map[int]int{1: 1}),
			},
			err: "type conversion error",
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			out := reg.NewValue(tc.typeName, tc.fields)
			if !IsError(out) {
				t.Fatalf("reg.NewValue(%s, %v) got %v, wanted error", tc.typeName, tc.fields, out)
			}
			err := out.(*Err)
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("reg.NewValue() got error %v, wanted error %s", err, tc.err)
			}
		})
	}
}

func TestRegistryGetters(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))
	sourceInfo := reg.NewValue(
		"google.api.expr.v1alpha1.SourceInfo",
		map[string]ref.Val{
			"location":     String("TestTypeRegistryGetFieldValue"),
			"line_offsets": NewDynamicList(reg, []int64{0, 2}),
			"positions":    NewDynamicMap(reg, map[int64]int64{1: 2, 2: 4}),
		})
	if IsError(sourceInfo) {
		t.Fatalf("NewValue(SourceInfo) failed: %v", sourceInfo)
	}

	si := sourceInfo.(traits.Indexer)

	t.Run("location", func(t *testing.T) {
		loc := si.Get(String("location"))
		if IsError(loc) {
			t.Fatal(loc)
		}
		if loc.(String) != "TestTypeRegistryGetFieldValue" {
			t.Errorf("Expected %s, got %s", "TestTypeRegistryGetFieldValue", loc)
		}
	})

	t.Run("positions", func(t *testing.T) {
		pos := si.Get(String("positions"))
		if IsError(pos) {
			t.Fatal(pos)
		}
		if pos.Equal(NewDynamicMap(reg, map[int64]int32{1: 2, 2: 4})) != True {
			t.Errorf("Expected map[int64]int32, got %v", pos)
		}
		posKeyVal := pos.(traits.Indexer).Get(Int(1))
		if IsError(posKeyVal) {
			t.Fatal(posKeyVal)
		}
		if posKeyVal.(Int) != 2 {
			t.Error("Expected value to be int64, not int32")
		}
	})

	t.Run("line_offsets", func(t *testing.T) {
		offsets := si.Get(String("line_offsets"))
		if IsError(offsets) {
			t.Fatal(offsets)
		}
		offset1 := offsets.(traits.Lister).Get(Int(1))
		if IsError(offset1) {
			t.Fatal(offset1)
		}
		if offset1.(Int) != 2 {
			t.Errorf("Expected index 1 to be value 2, was %v", offset1)
		}
	})
}

func TestConvertToNative(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))
	parsedExpr := &exprpb.ParsedExpr{}

	tests := []struct {
		name string
		in   ref.Val
		want any
	}{
		// Core type conversion tests.
		{name: "bool to bool", in: True, want: true},
		{name: "bool to ref.Val Bool", in: True, want: True},
		{name: "bool list to []any", in: NewDynamicList(reg, []Bool{True, False}), want: []any{true, false}},
		{name: "bool list to []ref.Val", in: NewDynamicList(reg, []Bool{True, False}), want: []ref.Val{True, False}},
		{name: "int to int32", in: Int(-1), want: int32(-1)},
		{name: "int to int64", in: Int(2), want: int64(2)},
		{name: "int to ref.Val Int", in: Int(-1), want: Int(-1)},
		{name: "int list to []any", in: NewDynamicList(reg, []Int{4}), want: []any{int64(4)}},
		{name: "int list to []ref.Val", in: NewDynamicList(reg, []Int{5}), want: []ref.Val{Int(5)}},
		{name: "uint to uint32", in: Uint(3), want: uint32(3)},
		{name: "uint to uint64", in: Uint(4), want: uint64(4)},
		{name: "uint to ref.Val Uint", in: Uint(5), want: Uint(5)},
		{name: "uint list to []any", in: NewDynamicList(reg, []Uint{4}), want: []any{uint64(4)}},
		{name: "uint list to []ref.Val", in: NewDynamicList(reg, []Uint{5}), want: []ref.Val{Uint(5)}},
		{name: "double to float32", in: Double(5.5), want: float32(5.5)},
		{name: "double to float64", in: Double(-5.5), want: float64(-5.5)},
		{name: "double list to []any", in: NewDynamicList(reg, []Double{-5.5}), want: []any{-5.5}},
		{name: "double list to []ref.Val", in: NewDynamicList(reg, []Double{-5.5}), want: []ref.Val{Double(-5.5)}},
		{name: "double to ref.Val Double", in: Double(-5.5), want: Double(-5.5)},
		{name: "string to string", in: String("hello"), want: "hello"},
		{name: "string to ref.Val String", in: String("hello"), want: String("hello")},
		{name: "null to structpb.NullValue", in: NullValue, want: structpb.NullValue_NULL_VALUE},
		{name: "null to ref.Val NullValue", in: NullValue, want: NullValue},
		{name: "null list to []any", in: NewDynamicList(reg, []Null{NullValue}), want: []any{structpb.NullValue_NULL_VALUE}},
		{name: "null list to []ref.Val", in: NewDynamicList(reg, []Null{NullValue}), want: []ref.Val{NullValue}},
		{name: "bytes to []byte", in: Bytes("world"), want: []byte("world")},
		{name: "bytes to ref.Val Bytes", in: Bytes("world"), want: Bytes("world")},
		{name: "bytes list to []any", in: NewDynamicList(reg, []Bytes{Bytes("hello")}), want: []any{[]byte("hello")}},
		{name: "bytes list to []ref.Val", in: NewDynamicList(reg, []Bytes{Bytes("hello")}), want: []ref.Val{Bytes("hello")}},
		{name: "int64 list to []int32", in: NewDynamicList(reg, []int64{1, 2, 3}), want: []int32{1, 2, 3}},
		{name: "duration to time.Duration", in: Duration{Duration: time.Duration(500)}, want: time.Duration(500)},
		{name: "duration to ref.Val Duration", in: Duration{Duration: time.Duration(500)}, want: Duration{Duration: time.Duration(500)}},
		{name: "timestamp to time.Time", in: Timestamp{Time: time.Unix(12345, 0)}, want: time.Unix(12345, 0)},
		{name: "timestamp to ref.Val Timestamp", in: Timestamp{Time: time.Unix(12345, 0)}, want: Timestamp{Time: time.Unix(12345, 0)}},
		{name: "map[int64]int64 to map[int32]int32", in: NewDynamicMap(reg, map[int64]int64{1: 1, 2: 1, 3: 1}), want: map[int32]int32{1: 1, 2: 1, 3: 1}},

		// Null conversion tests.
		{name: "Null(NULL_VALUE) to structpb.NullValue", in: Null(structpb.NullValue_NULL_VALUE), want: structpb.NullValue_NULL_VALUE},

		// Proto conversion tests.
		{name: "parsedExpr proto to proto message", in: reg.NativeToValue(parsedExpr), want: parsedExpr},

		// Custom scalars
		{name: "int to testInt", in: Int(1), want: testInt(1)},
		{name: "int to testInt8", in: Int(1), want: testInt8(1)},
		{name: "int to testInt16", in: Int(1), want: testInt16(1)},
		{name: "int to testInt32", in: Int(1), want: testInt32(1)},
		{name: "int to testInt64", in: Int(1), want: testInt64(1)},
		{name: "uint to testUint", in: Uint(1), want: testUint(1)},
		{name: "uint to testUint8", in: Uint(1), want: testUint8(1)},
		{name: "uint to testUint16", in: Uint(1), want: testUint16(1)},
		{name: "uint to testUint32", in: Uint(1), want: testUint32(1)},
		{name: "uint to testUint64", in: Uint(1), want: testUint64(1)},
		{name: "double to testFloat32", in: Double(4.5), want: testFloat32(4.5)},
		{name: "double to testFloat64", in: Double(-5.1), want: testFloat64(-5.1)},
		{name: "string to testString", in: String("foo"), want: testString("foo")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectValueToNative(t, tc.in, tc.want)
		})
	}
}

func TestNativeToValue_Any(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))

	nullAny, err := NullValue.ConvertToNative(anyValueType)
	if err != nil {
		t.Fatalf("NullValue.ConvertToNative() failed: %v", err)
	}

	jsonStructAny, err := anypb.New(
		structpb.NewStructValue(
			&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"a": structpb.NewStringValue("world"),
					"b": structpb.NewStringValue("five!"),
				},
			},
		),
	)
	if err != nil {
		t.Fatalf("anypb.New(NewStructValue) failed: %v", err)
	}

	jsonListAny, err := anypb.New(structpb.NewListValue(
		&structpb.ListValue{
			Values: []*structpb.Value{
				structpb.NewStringValue("world"),
				structpb.NewStringValue("five!"),
			},
		},
	))
	if err != nil {
		t.Fatalf("anypb.New(NewListValue) failed: %v", err)
	}

	pbMessage := exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3},
		},
	}
	pbMessageAny, err := anypb.New(&pbMessage)
	if err != nil {
		t.Fatalf("anypb.New(ParsedExpr) failed: %v", err)
	}

	tests := []struct {
		name    string
		in      any
		want    ref.Val
		wantErr bool
	}{
		{
			name: "NullValue",
			in:   nullAny,
			want: NullValue,
		},
		{
			name: "JSON Struct",
			in:   jsonStructAny,
			want: NewJSONStruct(reg, &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"a": structpb.NewStringValue("world"),
					"b": structpb.NewStringValue("five!"),
				},
			}),
		},
		{
			name: "JSON List",
			in:   jsonListAny,
			want: NewJSONList(reg, &structpb.ListValue{
				Values: []*structpb.Value{
					structpb.NewStringValue("world"),
					structpb.NewStringValue("five!"),
				},
			}),
		},
		{
			name: "Proto Message",
			in:   pbMessageAny,
			want: reg.NativeToValue(&pbMessage),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectNativeToValue(t, tc.in, tc.want)
		})
	}
}

func TestNativeToValue_Json(t *testing.T) {
	reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))
	parsedExpr := &exprpb.ParsedExpr{}

	tests := []struct {
		name    string
		in      any
		want    ref.Val
		wantErr bool
	}{
		// Json primitive conversion test.
		{name: "bool value", in: structpb.NewBoolValue(false), want: False},
		{name: "number value", in: structpb.NewNumberValue(1.1), want: Double(1.1)},
		{name: "null value", in: structpb.NewNullValue(), want: Null(structpb.NullValue_NULL_VALUE)},
		{name: "string value", in: structpb.NewStringValue("hello"), want: String("hello")},

		// Json list conversion.
		{
			name: "list value",
			in: structpb.NewListValue(
				&structpb.ListValue{
					Values: []*structpb.Value{
						structpb.NewStringValue("world"),
						structpb.NewStringValue("five!"),
					},
				},
			),
			want: NewJSONList(reg, &structpb.ListValue{
				Values: []*structpb.Value{
					structpb.NewStringValue("world"),
					structpb.NewStringValue("five!"),
				},
			}),
		},

		// Json struct conversion.
		{
			name: "struct value",
			in: structpb.NewStructValue(
				&structpb.Struct{
					Fields: map[string]*structpb.Value{
						"a": structpb.NewStringValue("world"),
						"b": structpb.NewStringValue("five!"),
					},
				},
			),
			want: NewJSONStruct(reg, &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"a": structpb.NewStringValue("world"),
					"b": structpb.NewStringValue("five!"),
				},
			}),
		},

		// Proto conversion test.
		{
			name: "proto message",
			in:   parsedExpr,
			want: reg.NativeToValue(parsedExpr),
		},

		// Go json.Number conversion.
		{name: "json.Number int", in: json.Number("42"), want: Int(42)},
		{name: "json.Number float", in: json.Number("42.5"), want: Double(42.5)},
		{name: "json.Number invalid", in: json.Number("invalid-num"), wantErr: true},

		// Go json.RawMessage conversion.
		{name: "json.RawMessage map", in: json.RawMessage(`{"key":"value"}`), want: NewStringInterfaceMap(reg, map[string]any{"key": "value"})},
		{name: "json.RawMessage string", in: json.RawMessage(`"hello"`), want: String("hello")},
		{name: "json.RawMessage int", in: json.RawMessage(`123`), want: Double(123)},
		{name: "json.RawMessage array", in: json.RawMessage(`["world", 42]`), want: NewDynamicList(reg, []any{"world", float64(42)})},
		{name: "[]json.RawMessage slice", in: []json.RawMessage{json.RawMessage(`"hello"`), json.RawMessage(`123`)}, want: NewDynamicList(reg, []json.RawMessage{json.RawMessage(`"hello"`), json.RawMessage(`123`)})},
		{name: "json.RawMessage invalid", in: json.RawMessage(`invalid-json`), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantErr {
				reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))
				val := reg.NativeToValue(tc.in)
				if !IsError(val) {
					t.Errorf("NativeToValue(%v) = %v, want error", tc.in, val)
				}
				return
			}
			expectNativeToValue(t, tc.in, tc.want)
		})
	}
}

func TestNativeToValue_Wrappers(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want ref.Val
	}{
		{name: "bool wrapper true", in: wrapperspb.Bool(true), want: True},
		{name: "bool wrapper zero value", in: &wrapperspb.BoolValue{}, want: False},
		{name: "bool wrapper nil", in: (*wrapperspb.BoolValue)(nil), want: NullValue},
		{name: "bytes wrapper zero value", in: &wrapperspb.BytesValue{}, want: Bytes{}},
		{name: "bytes wrapper value", in: wrapperspb.Bytes([]byte("hi")), want: Bytes("hi")},
		{name: "bytes wrapper nil", in: (*wrapperspb.BytesValue)(nil), want: NullValue},
		{name: "double wrapper zero value", in: &wrapperspb.DoubleValue{}, want: Double(0.0)},
		{name: "double wrapper value", in: wrapperspb.Double(6.4), want: Double(6.4)},
		{name: "double wrapper nil", in: (*wrapperspb.DoubleValue)(nil), want: NullValue},
		{name: "float wrapper zero value", in: &wrapperspb.FloatValue{}, want: Double(0.0)},
		{name: "float wrapper value", in: wrapperspb.Float(3.0), want: Double(3.0)},
		{name: "float wrapper nil", in: (*wrapperspb.FloatValue)(nil), want: NullValue},
		{name: "int32 wrapper zero value", in: &wrapperspb.Int32Value{}, want: IntZero},
		{name: "int32 wrapper value", in: wrapperspb.Int32(-32), want: Int(-32)},
		{name: "int32 wrapper nil", in: (*wrapperspb.Int32Value)(nil), want: NullValue},
		{name: "int64 wrapper zero value", in: &wrapperspb.Int64Value{}, want: IntZero},
		{name: "int64 wrapper value", in: wrapperspb.Int64(-64), want: Int(-64)},
		{name: "int64 wrapper nil", in: (*wrapperspb.Int64Value)(nil), want: NullValue},
		{name: "string wrapper zero value", in: &wrapperspb.StringValue{}, want: String("")},
		{name: "string wrapper value", in: wrapperspb.String("hello"), want: String("hello")},
		{name: "string wrapper nil", in: (*wrapperspb.StringValue)(nil), want: NullValue},
		{name: "uint32 wrapper zero value", in: &wrapperspb.UInt32Value{}, want: Uint(0)},
		{name: "uint32 wrapper value", in: wrapperspb.UInt32(32), want: Uint(32)},
		{name: "uint32 wrapper nil", in: (*wrapperspb.UInt32Value)(nil), want: NullValue},
		{name: "uint64 wrapper zero value", in: &wrapperspb.UInt64Value{}, want: Uint(0)},
		{name: "uint64 wrapper value", in: wrapperspb.UInt64(64), want: Uint(64)},
		{name: "uint64 wrapper nil", in: (*wrapperspb.UInt64Value)(nil), want: NullValue},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectNativeToValue(t, tc.in, tc.want)
		})
	}
}

func TestNativeToValue_Primitive(t *testing.T) {
	reg := newTestRegistry(t)

	pBool := true
	pDub32 := float32(2.5)
	pDub64 := float64(-1000.2)
	pInt := int(1)
	pInt32 := int32(2)
	pInt64 := int64(-1000)
	pStr := "hello"
	pUint := uint(1)
	pUint32 := uint32(2)
	pUint64 := uint64(1000)

	rBool := True
	rDub := Double(32.1)
	rInt := Int(-12)
	rStr := String("hello")
	rUint := Uint(12405)
	rBytes := Bytes([]byte("hello"))

	tests := []struct {
		name string
		in   any
		want ref.Val
	}{
		// Core type conversions.
		{name: "bool", in: true, want: True},
		{name: "int", in: int(-10), want: Int(-10)},
		{name: "int32", in: int32(-1), want: Int(-1)},
		{name: "int64", in: int64(2), want: Int(2)},
		{name: "uint", in: uint(6), want: Uint(6)},
		{name: "uint32", in: uint32(3), want: Uint(3)},
		{name: "uint64", in: uint64(4), want: Uint(4)},
		{name: "float32", in: float32(5.5), want: Double(5.5)},
		{name: "float64", in: float64(-5.5), want: Double(-5.5)},
		{name: "string", in: "hello", want: String("hello")},
		{name: "bytes slice", in: []byte("world"), want: Bytes("world")},
		{name: "bytes array", in: [4]byte{1, 2, 3, 4}, want: Bytes([]byte{1, 2, 3, 4})},
		{name: "bytes array pointer", in: &[4]byte{1, 2, 3, 4}, want: Bytes([]byte{1, 2, 3, 4})},
		{name: "time duration", in: time.Duration(500), want: Duration{Duration: time.Duration(500)}},
		{name: "time timestamp", in: time.Unix(12345, 0), want: Timestamp{Time: time.Unix(12345, 0)}},
		{name: "proto duration", in: dpb.New(time.Duration(500)), want: Duration{Duration: time.Duration(500)}},
		{name: "proto timestamp", in: tpb.New(time.Unix(12345, 0)), want: Timestamp{Time: time.Unix(12345, 0)}},
		{name: "slice of int32", in: []int32{1, 2, 3}, want: NewDynamicList(reg, []int32{1, 2, 3})},
		{name: "map of int32", in: map[int32]int32{1: 1, 2: 1, 3: 1}, want: NewDynamicMap(reg, map[int32]int32{1: 1, 2: 1, 3: 1})},

		// Pointers to core types.
		{name: "pointer to bool", in: &pBool, want: True},
		{name: "pointer to float32", in: &pDub32, want: Double(2.5)},
		{name: "pointer to float64", in: &pDub64, want: Double(-1000.2)},
		{name: "pointer to int", in: &pInt, want: Int(1)},
		{name: "pointer to int32", in: &pInt32, want: Int(2)},
		{name: "pointer to int64", in: &pInt64, want: Int(-1000)},
		{name: "pointer to string", in: &pStr, want: String("hello")},
		{name: "pointer to uint", in: &pUint, want: Uint(1)},
		{name: "pointer to uint32", in: &pUint32, want: Uint(2)},
		{name: "pointer to uint64", in: &pUint64, want: Uint(1000)},

		// Pointers to ref.Val extensions of core types.
		{name: "pointer to ref.Val bool", in: &rBool, want: True},
		{name: "pointer to ref.Val double", in: &rDub, want: rDub},
		{name: "pointer to ref.Val int", in: &rInt, want: rInt},
		{name: "pointer to ref.Val string", in: &rStr, want: rStr},
		{name: "pointer to ref.Val uint", in: &rUint, want: rUint},
		{name: "pointer to ref.Val bytes", in: &rBytes, want: rBytes},

		// Extensions to core types.
		{name: "custom testBool", in: testBool(true), want: True},
		{name: "custom testInt", in: testInt(1), want: Int(1)},
		{name: "custom testInt8", in: testInt8(1), want: Int(1)},
		{name: "custom testInt16", in: testInt16(1), want: Int(1)},
		{name: "custom testInt32", in: testInt32(1), want: Int(1)},
		{name: "custom testInt64", in: testInt64(-100), want: Int(-100)},
		{name: "custom testUint", in: testUint(1), want: Uint(1)},
		{name: "custom testUint8", in: testUint8(1), want: Uint(1)},
		{name: "custom testUint16", in: testUint16(1), want: Uint(1)},
		{name: "custom testUint32", in: testUint32(2), want: Uint(2)},
		{name: "custom testUint64", in: testUint64(3), want: Uint(3)},
		{name: "custom testFloat32", in: testFloat32(4.5), want: Double(4.5)},
		{name: "custom testFloat64", in: testFloat64(-5.1), want: Double(-5.1)},
		{name: "custom testString", in: testString("foo"), want: String("foo")},

		// Null conversion test.
		{name: "nil", in: nil, want: NullValue},
		{name: "proto null value", in: structpb.NullValue_NULL_VALUE, want: Null(structpb.NullValue_NULL_VALUE)},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			expectNativeToValue(t, tc.in, tc.want)
		})
	}
}

func TestUnsupportedConversion(t *testing.T) {
	reg := newTestRegistry(t)
	if val := reg.NativeToValue(nonConvertible{}); !IsError(val) {
		t.Error("Expected error when converting non-proto struct to proto", val)
	}
}

func expectValueToNative(t *testing.T, in ref.Val, out any) {
	t.Helper()
	if val, err := in.ConvertToNative(reflect.TypeOf(out)); err != nil {
		t.Error(err)
	} else {
		var equals bool
		switch val.(type) {
		case []byte:
			equals = bytes.Equal(val.([]byte), out.([]byte))
		case proto.Message:
			equals = proto.Equal(val.(proto.Message), out.(proto.Message))
		case bool, int32, int64, uint32, uint64, float32, float64, string:
			equals = val == out
		default:
			equals = reflect.DeepEqual(val, out)
		}
		if !equals {
			t.Errorf("Unexpected conversion from expr to proto.\n"+
				"expected: %T, actual: %T", out, val)
		}
	}
}

func expectNativeToValue(t *testing.T, in any, out ref.Val) {
	t.Helper()
	reg := newTestRegistry(t, ProtoTypeDefs(&exprpb.ParsedExpr{}))
	if val := reg.NativeToValue(in); IsError(val) {
		t.Error(val)
	} else {
		if val.Equal(out) != True {
			t.Errorf("Unexpected conversion from expr to proto.\n"+
				"expected: %T, actual: %T", val, out)
		}
	}
}

func BenchmarkNativeToValue(b *testing.B) {
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		b.Fatalf("NewRegistry() failed: %v", err)
	}

	dummyDesc := &testDummyStructDescriptor{
		Type:        NewObjectType("dummy.Struct"),
		reflectType: reflect.TypeOf(dummyNativeStruct{}),
		fieldType:   &FieldType{Type: StringType},
	}
	if err := reg.RegisterType(dummyDesc); err != nil {
		b.Fatalf("RegisterType() failed: %v", err)
	}

	protoMsg := &proto3pb.TestAllTypes{SingleInt32: 42}
	nativeStructVal := dummyNativeStruct{}
	nativeStructPtr := &dummyNativeStruct{}

	inputs := []struct {
		name string
		val  any
	}{
		{name: "bool/true", val: true},
		{name: "int/1", val: 1},
		{name: "int64/3", val: int64(3)},
		{name: "string/hello", val: "hello"},
		{name: "ref.Val/String", val: String("hello world")},
		{name: "ref.Val/Int", val: Int(42)},
		{name: "ref.Val/Bool", val: Bool(true)},
		{name: "proto/TestAllTypes", val: protoMsg},
		{name: "nativeStruct/value", val: nativeStructVal},
		{name: "nativeStruct/pointer", val: nativeStructPtr},
	}

	for _, tc := range inputs {
		input := tc.val
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				reg.NativeToValue(input)
			}
		})
	}
}

func TestRegistryStructTypeDescriptor_FindStructType(t *testing.T) {
	reg := newTestStructTypeRegistry(t)
	tests := []struct {
		name     string
		wantType string
	}{
		{name: "custom.MyStruct", wantType: "type"},
		{name: ".custom.MyStruct", wantType: "type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, found := reg.FindStructType(tc.name)
			if !found || st == nil {
				t.Fatalf("FindStructType(%q) not found", tc.name)
			}
			if st.TypeName() != tc.wantType {
				t.Errorf("FindStructType(%q).TypeName() = %s, want %s", tc.name, st.TypeName(), tc.wantType)
			}
			if st.Parameters()[0].TypeName() != "custom.MyStruct" {
				t.Errorf("FindStructType(%q) TypeName() = %s, want 'custom.MyStruct'", tc.name, st.Parameters()[0].TypeName())
			}
		})
	}
}

func TestRegistryStructTypeDescriptor_FindStructFieldNames(t *testing.T) {
	reg := newTestStructTypeRegistry(t)
	names, found := reg.FindStructFieldNames("custom.MyStruct")
	if !found {
		t.Fatalf("FindStructFieldNames('custom.MyStruct') not found")
	}
	want := []string{"Bar", "Foo"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("FindStructFieldNames() = %v, want %v", names, want)
	}
}

func TestRegistryStructTypeDescriptor_FindStructFieldType(t *testing.T) {
	reg := newTestStructTypeRegistry(t)
	tests := []struct {
		fieldName string
		wantType  *Type
	}{
		{fieldName: "Foo", wantType: StringType},
		{fieldName: "Bar", wantType: IntType},
	}
	for _, tc := range tests {
		t.Run(tc.fieldName, func(t *testing.T) {
			ft, found := reg.FindStructFieldType("custom.MyStruct", tc.fieldName)
			if !found || ft == nil {
				t.Fatalf("FindStructFieldType(%q) not found", tc.fieldName)
			}
			if ft.Type != tc.wantType {
				t.Errorf("FindStructFieldType(%q).Type = %v, want %v", tc.fieldName, ft.Type, tc.wantType)
			}
		})
	}
}

func TestRegistryStructTypeDescriptor_FindIdent(t *testing.T) {
	reg := newTestStructTypeRegistry(t)
	for _, identName := range []string{"custom.MyStruct", ".custom.MyStruct"} {
		ident, found := reg.FindIdent(identName)
		if !found || ident == nil {
			t.Fatalf("FindIdent(%q) not found", identName)
		}
	}
}

func TestRegistryStructTypeDescriptor_NewValue(t *testing.T) {
	reg := newTestStructTypeRegistry(t)
	val := reg.NewValue("custom.MyStruct", map[string]ref.Val{"Foo": String("hello"), "Bar": Int(42)})
	if IsError(val) {
		t.Fatalf("NewValue() failed: %v", val)
	}

	t.Run("Foo", func(t *testing.T) {
		fooVal := val.(traits.Indexer).Get(String("Foo"))
		if fooVal.Equal(String("hello")) != True {
			t.Errorf("Get('Foo') = %v, want 'hello'", fooVal)
		}
	})

	t.Run("Bar", func(t *testing.T) {
		barVal := val.(traits.Indexer).Get(String("Bar"))
		if barVal.Equal(Int(42)) != True {
			t.Errorf("Get('Bar') = %v, want 42", barVal)
		}
	})
}

func TestRegistryStructTypeDescriptor_NativeToValue(t *testing.T) {
	reg := newTestStructTypeRegistry(t)
	tests := []struct {
		name  string
		in    any
		check func(t *testing.T, val ref.Val)
	}{
		{
			name: "struct instance",
			in:   dummyNativeStruct{Foo: "hello", Bar: 42},
			check: func(t *testing.T, val ref.Val) {
				fooVal := val.(traits.Indexer).Get(String("Foo"))
				if fooVal.Equal(String("hello")) != True {
					t.Errorf("Get('Foo') = %v, want 'hello'", fooVal)
				}
			},
		},
		{
			name: "pointer to struct instance",
			in:   &dummyNativeStruct{Foo: "world", Bar: 99},
			check: func(t *testing.T, val ref.Val) {
				barVal := val.(traits.Indexer).Get(String("Bar"))
				if barVal.Equal(Int(99)) != True {
					t.Errorf("Get('Bar') = %v, want 99", barVal)
				}
			},
		},
		{
			name: "slice of struct instances",
			in:   []dummyNativeStruct{{Foo: "e1"}, {Foo: "e2"}},
			check: func(t *testing.T, val ref.Val) {
				lister := val.(traits.Lister)
				if lister.Size().Equal(Int(2)) != True {
					t.Errorf("Size() = %v, want 2", lister.Size())
				}
				e1 := lister.Get(Int(0)).(traits.Indexer).Get(String("Foo"))
				if e1.Equal(String("e1")) != True {
					t.Errorf("element 0 Foo = %v, want 'e1'", e1)
				}
			},
		},
		{
			name: "map of struct instances",
			in:   map[string]dummyNativeStruct{"k1": {Foo: "v1"}},
			check: func(t *testing.T, val ref.Val) {
				mapper := val.(traits.Mapper)
				k1Val := mapper.Get(String("k1")).(traits.Indexer).Get(String("Foo"))
				if k1Val.Equal(String("v1")) != True {
					t.Errorf("map k1 Foo = %v, want 'v1'", k1Val)
				}
			},
		},
		{
			name: "typed nil pointer to struct instance",
			in:   (*dummyNativeStruct)(nil),
			check: func(t *testing.T, val ref.Val) {
				if val != NullValue {
					t.Errorf("NativeToValue((*dummyNativeStruct)(nil)) = %v, want NullValue", val)
				}
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			val := reg.NativeToValue(tc.in)
			if IsError(val) {
				t.Fatalf("NativeToValue(%s) error: %v", tc.name, val)
			}
			tc.check(t, val)
		})
	}
}

func BenchmarkTypeProviderNewValue(b *testing.B) {
	reg, err := NewRegistry(&exprpb.ParsedExpr{})
	if err != nil {
		b.Fatalf("NewRegistry() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		reg.NewValue(
			"google.api.expr.v1.SourceInfo",
			map[string]ref.Val{
				"Location":    String("BenchmarkTypeProvider_NewValue"),
				"LineOffsets": NewDynamicList(reg, []int64{0, 2}),
				"Positions":   NewDynamicMap(reg, map[int64]int64{1: 2, 2: 4}),
			})
	}
}

func BenchmarkTypeProviderCopy(b *testing.B) {
	reg, err := NewRegistry()
	if err != nil {
		b.Fatalf("NewRegistry() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		reg.Copy()
	}
}

// Helper types useful for testing extensions of primitive types.
type nonConvertible struct {
	Field string
}
type testBool bool
type testInt int
type testInt8 int8
type testInt16 int16
type testInt32 int32
type testInt64 int64
type testUint uint
type testUint8 uint8
type testUint16 uint16
type testUint32 uint32
type testUint64 uint64
type testFloat32 float32
type testFloat64 float64
type testString string

func newTestRegistry(t *testing.T, opts ...RegistryOption) *Registry {
	t.Helper()
	var o []any
	for _, opt := range opts {
		o = append(o, opt)
	}
	reg, err := NewRegistry(o...)
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	return reg
}

type dummyNativeStruct struct {
	Foo string
	Bar int64
}

type testStructType struct {
	typeName    string
	reflectType reflect.Type
	fields      map[string]*FieldType
	celType     *Type
}

func (d *testStructType) HasTrait(trait int) bool {
	return d.objectType().HasTrait(trait)
}

func (d *testStructType) TypeName() string {
	return d.typeName
}

func (d *testStructType) objectType() *Type {
	if d.celType == nil {
		d.celType = NewObjectType(d.typeName)
	}
	return d.celType
}

func (d *testStructType) ReflectType() reflect.Type {
	return d.reflectType
}

func (d *testStructType) FieldNames() []string {
	names := make([]string, 0, len(d.fields))
	for name := range d.fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (d *testStructType) FindFieldType(fieldName string) (*FieldType, bool) {
	ft, found := d.fields[fieldName]
	return ft, found
}

func (d *testStructType) NewValue(adapter Adapter, fields map[string]ref.Val) ref.Val {
	if d.reflectType == nil {
		return &testStructVal{
			adapter: adapter,
			st:      d,
			value:   fields,
		}
	}
	refPtr := reflect.New(d.reflectType)
	refVal := refPtr.Elem()
	for fieldName, val := range fields {
		refField := refVal.FieldByName(fieldName)
		if !refField.IsValid() || !refField.CanSet() {
			return NewErr("no such field: %s", fieldName)
		}
		nativeVal, err := val.ConvertToNative(refField.Type())
		if err != nil {
			return NewErrFromString(err.Error())
		}
		refField.Set(reflect.ValueOf(nativeVal))
	}
	var inst any
	if d.reflectType.Kind() == reflect.Pointer {
		inst = refPtr.Interface()
	} else {
		inst = refVal.Interface()
	}
	return d.Adapt(adapter, inst)
}

func (d *testStructType) Adapt(adapter Adapter, value any) ref.Val {
	if value == nil {
		return NullValue
	}
	refVal := reflect.ValueOf(value)
	if refVal.Kind() == reflect.Pointer && refVal.IsNil() {
		return NullValue
	}
	return &testStructVal{
		adapter: adapter,
		st:      d,
		value:   value,
	}
}

type testStructVal struct {
	adapter Adapter
	st      *testStructType
	value   any
}

func (o *testStructVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if reflect.TypeOf(o.value).AssignableTo(typeDesc) {
		return o.value, nil
	}
	if reflect.TypeOf(o).AssignableTo(typeDesc) {
		return o, nil
	}
	return nil, fmt.Errorf("type conversion error for type to '%v'", typeDesc)
}

func (o *testStructVal) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case TypeType:
		return NewTypeTypeWithParam(o.Type().(*Type))
	default:
		if o.Type().TypeName() == typeVal.TypeName() {
			return o
		}
	}
	return NewErr("type conversion error from '%s' to '%s'", o.Type(), typeVal)
}

func (o *testStructVal) Equal(other ref.Val) ref.Val {
	return Bool(reflect.DeepEqual(o.value, other.Value()))
}

func (o *testStructVal) HasTrait(trait int) bool {
	return (traits.FieldTesterType|traits.IndexerType)&trait == trait
}

func (o *testStructVal) Get(index ref.Val) ref.Val {
	fieldName, ok := index.(String)
	if !ok {
		return MaybeNoSuchOverloadErr(index)
	}
	ft, found := o.st.FindFieldType(string(fieldName))
	if !found {
		return NewErr("no such field: %s", index)
	}
	if ft.GetFrom == nil {
		return NewErr("field '%s' is not readable", index)
	}
	fv, err := ft.GetFrom(o.value)
	if err != nil {
		return NewErrFromString(err.Error())
	}
	return o.adapter.NativeToValue(fv)
}

func (o *testStructVal) IsSet(field ref.Val) ref.Val {
	fieldName, ok := field.(String)
	if !ok {
		return MaybeNoSuchOverloadErr(field)
	}
	ft, found := o.st.FindFieldType(string(fieldName))
	if !found {
		return NewErr("no such field: %s", field)
	}
	if ft.IsSet == nil {
		return False
	}
	return Bool(ft.IsSet(o.value))
}

func (o *testStructVal) Type() ref.Type {
	return o.st
}

func (o *testStructVal) Value() any {
	return o.value
}

func newTestStructTypeRegistry(t *testing.T) *Registry {
	t.Helper()
	desc := &testStructType{
		typeName:    "custom.MyStruct",
		reflectType: reflect.TypeOf(dummyNativeStruct{}),
		fields: map[string]*FieldType{
			"Foo": {
				Type: StringType,
				GetFrom: func(obj any) (any, error) {
					if s, ok := obj.(dummyNativeStruct); ok {
						return s.Foo, nil
					}
					if s, ok := obj.(*dummyNativeStruct); ok {
						return s.Foo, nil
					}
					return nil, fmt.Errorf("unexpected type: %T", obj)
				},
				IsSet: func(obj any) bool { return true },
			},
			"Bar": {
				Type: IntType,
				GetFrom: func(obj any) (any, error) {
					if s, ok := obj.(dummyNativeStruct); ok {
						return s.Bar, nil
					}
					if s, ok := obj.(*dummyNativeStruct); ok {
						return s.Bar, nil
					}
					return nil, fmt.Errorf("unexpected type: %T", obj)
				},
				IsSet: func(obj any) bool { return true },
			},
		},
	}
	reg, err := NewRegistry(Types(desc))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	return reg
}

type testCustomProvider struct {
	enumVal    ref.Val
	identVal   ref.Val
	structType *Type
	fieldNames []string
	fieldType  *FieldType
	newValue   ref.Val
}

func (p *testCustomProvider) EnumValue(enumName string) ref.Val {
	if enumName == "custom.Enum.VAL" {
		return p.enumVal
	}
	return NewErr("unknown enum name '%s'", enumName)
}

func (p *testCustomProvider) FindIdent(identName string) (ref.Val, bool) {
	if identName == "customIdent" {
		return p.identVal, true
	}
	return nil, false
}

func (p *testCustomProvider) FindStructType(structType string) (*Type, bool) {
	if structType == "custom.ProviderStruct" {
		return p.structType, true
	}
	return nil, false
}

func (p *testCustomProvider) FindStructFieldNames(structType string) ([]string, bool) {
	if structType == "custom.ProviderStruct" {
		return p.fieldNames, true
	}
	return []string{}, false
}

func (p *testCustomProvider) FindStructFieldType(structType, fieldName string) (*FieldType, bool) {
	if structType == "custom.ProviderStruct" && fieldName == "customField" {
		return p.fieldType, true
	}
	return nil, false
}

func (p *testCustomProvider) NewValue(structType string, fields map[string]ref.Val) ref.Val {
	if structType == "custom.ProviderStruct" {
		return p.newValue
	}
	return NewErr("unknown type '%s'", structType)
}

func (p *testCustomProvider) FindStructFieldDescription(structType, fieldName string) (string, bool) {
	if structType == "custom.ProviderStruct" && fieldName == "customField" {
		return "Custom field documentation", true
	}
	return "", false
}

func (p *testCustomProvider) FindType(structType string) (*exprpb.Type, bool) {
	if structType == "custom.ProviderStruct" {
		return &exprpb.Type{
			TypeKind: &exprpb.Type_MessageType{MessageType: structType},
		}, true
	}
	return nil, false
}

func (p *testCustomProvider) FindFieldType(structType, fieldName string) (*ref.FieldType, bool) {
	if structType == "custom.ProviderStruct" && fieldName == "customField" {
		return &ref.FieldType{
			Type: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
		}, true
	}
	return nil, false
}

type customNativeType struct{}

type testCustomAdapter struct {
	adaptedVal ref.Val
}

func (a *testCustomAdapter) NativeToValue(value any) ref.Val {
	if _, ok := value.(customNativeType); ok {
		return a.adaptedVal
	}
	return UnsupportedRefValConversionErr(value)
}

type testCustomCombined struct {
	testCustomProvider
	testCustomAdapter
}

func TestComposeTypes_SameRegistry(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}

	p, a, err := ComposeTypes(reg, reg, ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	if p != reg || a != reg {
		t.Errorf("ComposeTypes() with same registry instance returned different instances: p=%v, a=%v, wanted reg=%v", p, a, reg)
	}

	// Verify type was registered directly on reg
	_, found := reg.FindStructType("google.expr.proto3.test.TestAllTypes")
	if !found {
		t.Errorf("FindStructType() did not find registered type on same registry instance")
	}
}

func TestComposeTypes_DifferentRegistries(t *testing.T) {
	reg1, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry(reg1) failed: %v", err)
	}
	reg2, err := NewRegistry(ProtoTypeDefs(&exprpb.ParsedExpr{}))
	if err != nil {
		t.Fatalf("NewRegistry(reg2) failed: %v", err)
	}

	p, a, err := ComposeTypes(reg1, reg2, ProtoTypeDefs(&exprpb.SourceInfo{}))
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	if p == reg1 || p == reg2 {
		t.Errorf("ComposeTypes() should return a new composed registry, got p=%v", p)
	}
	if any(p) != any(a) {
		t.Errorf("ComposeTypes() returned different provider and adapter: p=%v, a=%v", p, a)
	}

	composedReg := p.(*Registry)

	t.Run("registered type on composed registry", func(t *testing.T) {
		_, found := composedReg.FindStructType("google.api.expr.v1alpha1.SourceInfo")
		if !found {
			t.Errorf("FindStructType() failed for newly registered type on composed registry")
		}
	})

	t.Run("provider type via proxy", func(t *testing.T) {
		_, found := composedReg.FindStructType("google.expr.proto3.test.TestAllTypes")
		if !found {
			t.Errorf("FindStructType() failed for provider (reg1) type on composed registry")
		}
	})

	t.Run("adapter NativeToValue via proxy", func(t *testing.T) {
		parsedExpr := &exprpb.ParsedExpr{}
		val := composedReg.NativeToValue(parsedExpr)
		if IsError(val) {
			t.Errorf("NativeToValue() failed for adapter (reg2) type on composed registry: %v", val)
		}
	})
}

func TestComposeTypes_RegistryProviderCustomAdapter(t *testing.T) {
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	customAdapt := &testCustomAdapter{adaptedVal: String("adapted_success")}

	p, a, err := ComposeTypes(reg, customAdapt)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	if any(p) != any(a) {
		t.Errorf("ComposeTypes() returned different provider and adapter: p=%v, a=%v", p, a)
	}
	composedReg := p.(*Registry)

	t.Run("provider delegation", func(t *testing.T) {
		_, found := composedReg.FindStructType("google.expr.proto3.test.TestAllTypes")
		if !found {
			t.Errorf("FindStructType() failed to delegate to provider registry")
		}
	})

	t.Run("adapter delegation", func(t *testing.T) {
		val := composedReg.NativeToValue(customNativeType{})
		if IsError(val) || val.(String) != "adapted_success" {
			t.Errorf("NativeToValue() failed to delegate to custom adapter: got %v, wanted adapted_success", val)
		}
	})
}

func TestComposeTypes_CustomProviderRegistryAdapter(t *testing.T) {
	customProv := &testCustomProvider{
		enumVal:    Int(42),
		identVal:   String("ident_ok"),
		structType: NewTypeTypeWithParam(NewObjectType("custom.ProviderStruct")),
		fieldNames: []string{"customField"},
		fieldType:  &FieldType{Type: StringType},
		newValue:   String("new_val_ok"),
	}
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}

	p, a, err := ComposeTypes(customProv, reg)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	if any(p) != any(a) {
		t.Errorf("ComposeTypes() returned different provider and adapter: p=%v, a=%v", p, a)
	}
	composedReg := p.(*Registry)

	t.Run("EnumValue", func(t *testing.T) {
		if enumVal := composedReg.EnumValue("custom.Enum.VAL"); IsError(enumVal) || enumVal.(Int) != 42 {
			t.Errorf("EnumValue() proxy failed: got %v, wanted 42", enumVal)
		}
	})

	t.Run("FindIdent", func(t *testing.T) {
		if ident, found := composedReg.FindIdent("customIdent"); !found || ident.(String) != "ident_ok" {
			t.Errorf("FindIdent() proxy failed: got %v, found %v", ident, found)
		}
	})

	t.Run("FindStructType", func(t *testing.T) {
		if st, found := composedReg.FindStructType("custom.ProviderStruct"); !found || st == nil {
			t.Errorf("FindStructType() proxy failed: got %v, found %v", st, found)
		}
	})

	t.Run("FindStructFieldNames", func(t *testing.T) {
		if fields, found := composedReg.FindStructFieldNames("custom.ProviderStruct"); !found || len(fields) != 1 || fields[0] != "customField" {
			t.Errorf("FindStructFieldNames() proxy failed: got %v, found %v", fields, found)
		}
	})

	t.Run("FindStructFieldType", func(t *testing.T) {
		if ft, found := composedReg.FindStructFieldType("custom.ProviderStruct", "customField"); !found || ft.Type != StringType {
			t.Errorf("FindStructFieldType() proxy failed: got %v, found %v", ft, found)
		}
	})

	t.Run("NewValue", func(t *testing.T) {
		if nv := composedReg.NewValue("custom.ProviderStruct", nil); IsError(nv) || nv.(String) != "new_val_ok" {
			t.Errorf("NewValue() proxy failed: got %v", nv)
		}
	})

	t.Run("NativeToValue adapter proxy", func(t *testing.T) {
		msg := &proto3pb.TestAllTypes{}
		val := composedReg.NativeToValue(msg)
		if IsError(val) {
			t.Errorf("NativeToValue() proxy to registry adapter failed: %v", val)
		}
	})
}

func TestComposeTypes_CustomProviderAndAdapter(t *testing.T) {
	customProv := &testCustomProvider{
		identVal: String("ident_val"),
	}
	customAdapt := &testCustomAdapter{
		adaptedVal: String("adapted_val"),
	}

	p, a, err := ComposeTypes(customProv, customAdapt)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	if any(p) != any(a) {
		t.Errorf("ComposeTypes() returned different provider and adapter: p=%v, a=%v", p, a)
	}
	composedReg := p.(*Registry)

	t.Run("FindIdent", func(t *testing.T) {
		if ident, found := composedReg.FindIdent("customIdent"); !found || ident.(String) != "ident_val" {
			t.Errorf("FindIdent() failed on composed registry: got %v", ident)
		}
	})

	t.Run("NativeToValue", func(t *testing.T) {
		if val := composedReg.NativeToValue(customNativeType{}); IsError(val) || val.(String) != "adapted_val" {
			t.Errorf("NativeToValue() failed on composed registry: got %v", val)
		}
	})
}

func TestComposeTypes_SameCustomInstance(t *testing.T) {
	customBoth := &testCustomCombined{
		testCustomProvider: testCustomProvider{identVal: String("combined_ident")},
		testCustomAdapter:  testCustomAdapter{adaptedVal: String("combined_adapt")},
	}

	p, a, err := ComposeTypes(customBoth, customBoth)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	if any(p) != any(a) {
		t.Errorf("ComposeTypes() returned different provider and adapter: p=%v, a=%v", p, a)
	}
	if _, ok := p.(*Registry); !ok {
		t.Fatalf("ComposeTypes() should return a *Registry instance, got %T", p)
	}

	composedReg := p.(*Registry)

	t.Run("FindIdent", func(t *testing.T) {
		if ident, found := composedReg.FindIdent("customIdent"); !found || ident.(String) != "combined_ident" {
			t.Errorf("FindIdent() failed: got %v", ident)
		}
	})

	t.Run("NativeToValue", func(t *testing.T) {
		if val := composedReg.NativeToValue(customNativeType{}); IsError(val) || val.(String) != "combined_adapt" {
			t.Errorf("NativeToValue() failed: got %v", val)
		}
	})
}

func TestComposeTypes_ErrorCases(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}

	tests := []struct {
		name  string
		types []any
	}{
		{
			name:  "unsupported type",
			types: []any{12345},
		},
		{
			name: "conflicting type definitions",
			types: []any{
				NewTypeValue("http.Request", traits.ReceiverType),
				NewObjectType("http.Request", traits.ReceiverType),
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ComposeTypes(reg, reg, tc.types...)
			if err == nil {
				t.Errorf("ComposeTypes() expected error, got nil")
			}
		})
	}
}

func TestComposeTypes_CopyPreservesProxy(t *testing.T) {
	customProv := &testCustomProvider{identVal: String("copied_ident")}
	customAdapt := &testCustomAdapter{adaptedVal: String("copied_adapt")}

	p, _, err := ComposeTypes(customProv, customAdapt)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}

	reg := p.(*Registry)
	copiedReg := reg.Copy()

	t.Run("FindIdent", func(t *testing.T) {
		if ident, found := copiedReg.FindIdent("customIdent"); !found || ident.(String) != "copied_ident" {
			t.Errorf("Copied registry FindIdent() failed: got %v", ident)
		}
	})

	t.Run("NativeToValue", func(t *testing.T) {
		if val := copiedReg.NativeToValue(customNativeType{}); IsError(val) || val.(String) != "copied_adapt" {
			t.Errorf("Copied registry NativeToValue() failed: got %v", val)
		}
	})
}

func TestRegistryJSONFieldNamesDefault(t *testing.T) {
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	if reg.JSONFieldNames() {
		t.Errorf("JSONFieldNames() default expected false, got true")
	}
}

func TestRegistryWithJSONFieldNames(t *testing.T) {
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	err = reg.WithJSONFieldNames(true)
	if err != nil {
		t.Fatalf("WithJSONFieldNames(true) failed: %v", err)
	}
	if !reg.JSONFieldNames() {
		t.Errorf("JSONFieldNames() after enabling expected true, got false")
	}
}

func TestRegistryWithJSONFieldNamesIdempotent(t *testing.T) {
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	err = reg.WithJSONFieldNames(true)
	if err != nil {
		t.Fatalf("WithJSONFieldNames(true) failed: %v", err)
	}
	err = reg.WithJSONFieldNames(true)
	if err != nil {
		t.Fatalf("WithJSONFieldNames(true) idempotent failed: %v", err)
	}
	if !reg.JSONFieldNames() {
		t.Errorf("JSONFieldNames() expected true, got false")
	}
}

func TestRegistryWithJSONFieldNamesDisabled(t *testing.T) {
	reg, err := NewRegistry(ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}
	err = reg.WithJSONFieldNames(true)
	if err != nil {
		t.Fatalf("WithJSONFieldNames(true) failed: %v", err)
	}
	err = reg.WithJSONFieldNames(false)
	if err != nil {
		t.Fatalf("WithJSONFieldNames(false) failed: %v", err)
	}
	if reg.JSONFieldNames() {
		t.Errorf("JSONFieldNames() after disabling expected false, got true")
	}
}

func TestRegistry_EnumValueEdgeCases(t *testing.T) {
	customProv := &testCustomProvider{
		enumVal: Int(99),
	}
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	err := reg.RegisterDescriptor(proto3pb.GlobalEnum_GOO.Descriptor().ParentFile())
	if err != nil {
		t.Fatalf("RegisterDescriptor() failed: %v", err)
	}

	p, _, err := ComposeTypes(customProv, reg)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	composedReg := p.(*Registry)

	tests := []struct {
		enumName string
		target   *Registry
		wantVal  ref.Val
		isErr    bool
	}{
		{
			enumName: "google.expr.proto3.test.GlobalEnum.GOO",
			target:   reg,
			wantVal:  Int(proto3pb.GlobalEnum_GOO.Number()),
		},
		{
			enumName: "custom.Enum.VAL",
			target:   composedReg,
			wantVal:  Int(99),
		},
		{
			enumName: "non.existent.Enum",
			target:   reg,
			isErr:    true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.enumName, func(t *testing.T) {
			got := tc.target.EnumValue(tc.enumName)
			if tc.isErr {
				if !IsError(got) {
					t.Errorf("EnumValue(%s) expected error, got %v", tc.enumName, got)
				}
			} else {
				if IsError(got) || got.Equal(tc.wantVal) != True {
					t.Errorf("EnumValue(%s) got %v, wanted %v", tc.enumName, got, tc.wantVal)
				}
			}
		})
	}
}

func TestRegistry_FindIdentEdgeCases(t *testing.T) {
	customProv := &testCustomProvider{
		identVal: String("ident_found"),
	}
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	err := reg.RegisterDescriptor(proto3pb.GlobalEnum_GOO.Descriptor().ParentFile())
	if err != nil {
		t.Fatalf("RegisterDescriptor() failed: %v", err)
	}

	p, _, err := ComposeTypes(customProv, reg)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	composedReg := p.(*Registry)

	tests := []struct {
		identName string
		target    *Registry
		wantFound bool
	}{
		{
			identName: "int",
			target:    composedReg,
			wantFound: true,
		},
		{
			identName: "google.expr.proto3.test.GlobalEnum.GOO",
			target:    reg,
			wantFound: true,
		},
		{
			identName: "customIdent",
			target:    composedReg,
			wantFound: true,
		},
		{
			identName: "nonExistentIdent",
			target:    composedReg,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.identName, func(t *testing.T) {
			_, found := tc.target.FindIdent(tc.identName)
			if found != tc.wantFound {
				t.Errorf("FindIdent(%s) found=%v, wanted %v", tc.identName, found, tc.wantFound)
			}
		})
	}
}

func TestRegistry_FindTypeEdgeCases(t *testing.T) {
	customProv := &testCustomProvider{}
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))

	p, _, err := ComposeTypes(reg, &testCustomAdapter{})
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	composedReg := p.(*Registry)

	p2, _, err := ComposeTypes(customProv, reg)
	if err != nil {
		t.Fatalf("ComposeTypes(customProv, reg) failed: %v", err)
	}
	composedReg2 := p2.(*Registry)

	tests := []struct {
		typeName  string
		target    *Registry
		wantFound bool
	}{
		{
			typeName:  "google.expr.proto3.test.TestAllTypes",
			target:    composedReg,
			wantFound: true,
		},
		{
			typeName:  ".google.expr.proto3.test.TestAllTypes",
			target:    composedReg,
			wantFound: true,
		},
		{
			typeName:  "custom.ProviderStruct",
			target:    composedReg2,
			wantFound: true,
		},
		{
			typeName:  "non.existent.Type",
			target:    composedReg,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.typeName, func(t *testing.T) {
			got, found := tc.target.FindType(tc.typeName)
			if found != tc.wantFound {
				t.Errorf("FindType(%s) found=%v, wanted %v", tc.typeName, found, tc.wantFound)
			}
			if found && got == nil {
				t.Errorf("FindType(%s) returned nil type despite found=true", tc.typeName)
			}
		})
	}
}

func TestRegistry_FindFieldTypeEdgeCases(t *testing.T) {
	customProv := &testCustomProvider{}
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))

	p, _, err := ComposeTypes(reg, &testCustomAdapter{})
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	composedReg := p.(*Registry)

	p2, _, err := ComposeTypes(customProv, reg)
	if err != nil {
		t.Fatalf("ComposeTypes(customProv, reg) failed: %v", err)
	}
	composedReg2 := p2.(*Registry)

	tests := []struct {
		structType string
		fieldName  string
		target     *Registry
		wantFound  bool
	}{
		{
			structType: "google.expr.proto3.test.TestAllTypes",
			fieldName:  "single_int32",
			target:     composedReg,
			wantFound:  true,
		},
		{
			structType: ".google.expr.proto3.test.TestAllTypes",
			fieldName:  "single_int32",
			target:     composedReg,
			wantFound:  true,
		},
		{
			structType: "custom.ProviderStruct",
			fieldName:  "customField",
			target:     composedReg2,
			wantFound:  false,
		},
		{
			structType: "google.expr.proto3.test.TestAllTypes",
			fieldName:  "non_existent_field",
			target:     composedReg,
			wantFound:  false,
		},
		{
			structType: "non.existent.Type",
			fieldName:  "some_field",
			target:     composedReg,
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.structType+"."+tc.fieldName, func(t *testing.T) {
			got, found := tc.target.FindFieldType(tc.structType, tc.fieldName)
			if found != tc.wantFound {
				t.Errorf("FindFieldType(%s, %s) found=%v, wanted %v", tc.structType, tc.fieldName, found, tc.wantFound)
			}
			if found && got == nil {
				t.Errorf("FindFieldType(%s, %s) returned nil field type despite found=true", tc.structType, tc.fieldName)
			}
		})
	}
}

func TestRegistry_FindStructFieldDescriptionEdgeCases(t *testing.T) {
	customProv := &testCustomProvider{}
	reg := newTestRegistry(t, ProtoTypeDefs(&proto3pb.TestAllTypes{}))

	p, _, err := ComposeTypes(customProv, reg)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	composedReg := p.(*Registry)

	tests := []struct {
		structType string
		fieldName  string
		wantFound  bool
	}{
		{
			structType: "custom.ProviderStruct",
			fieldName:  "customField",
			wantFound:  true,
		},
		{
			structType: "google.expr.proto3.test.TestAllTypes",
			fieldName:  "non_existent_field",
			wantFound:  false,
		},
		{
			structType: "non.existent.Type",
			fieldName:  "some_field",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.structType+"."+tc.fieldName, func(t *testing.T) {
			_, found := composedReg.FindStructFieldDescription(tc.structType, tc.fieldName)
			if found != tc.wantFound {
				t.Errorf("FindStructFieldDescription(%s, %s) found=%v, wanted %v", tc.structType, tc.fieldName, found, tc.wantFound)
			}
		})
	}
}

type testDummyStructDescriptor struct {
	*Type
	reflectType reflect.Type
	fieldType   *FieldType
}

func (d *testDummyStructDescriptor) ReflectType() reflect.Type {
	return d.reflectType
}

func (d *testDummyStructDescriptor) FieldNames() []string {
	return []string{"DummyField"}
}

func (d *testDummyStructDescriptor) FindFieldType(fieldName string) (*FieldType, bool) {
	if fieldName == "DummyField" {
		return d.fieldType, true
	}
	return nil, false
}

func (d *testDummyStructDescriptor) NewValue(adapter Adapter, fields map[string]ref.Val) ref.Val {
	return String("dummy_struct_new")
}

func (d *testDummyStructDescriptor) Adapt(adapter Adapter, value any) ref.Val {
	return String("dummy_struct_adapted")
}

type testSimpleProvider struct{}

func (p *testSimpleProvider) EnumValue(enumName string) ref.Val          { return NewErr("no enum") }
func (p *testSimpleProvider) FindIdent(identName string) (ref.Val, bool) { return nil, false }
func (p *testSimpleProvider) FindStructType(structType string) (*Type, bool) {
	if structType == "simple.Struct" {
		return NewTypeTypeWithParam(NewObjectType("simple.Struct")), true
	}
	return nil, false
}
func (p *testSimpleProvider) FindStructFieldNames(structType string) ([]string, bool) {
	return nil, false
}
func (p *testSimpleProvider) FindStructFieldType(structType, fieldName string) (*FieldType, bool) {
	if structType == "simple.Struct" && fieldName == "simpleField" {
		return &FieldType{Type: StringType}, true
	}
	return nil, false
}
func (p *testSimpleProvider) NewValue(structType string, fields map[string]ref.Val) ref.Val {
	return NewErr("no value")
}

func TestRegistry_FindStructDescriptorByReflectType(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}

	st := &testDummyStructDescriptor{
		Type:        NewObjectType("dummy.Struct"),
		reflectType: reflect.TypeOf(dummyNativeStruct{}),
		fieldType:   &FieldType{Type: StringType},
	}

	err = reg.RegisterType(st)
	if err != nil {
		t.Fatalf("RegisterType() failed: %v", err)
	}

	t.Run("Pointer lookup when registered as value", func(t *testing.T) {
		val := reg.NativeToValue(&dummyNativeStruct{})
		if IsError(val) || val.(String) != "dummy_struct_adapted" {
			t.Errorf("NativeToValue(&dummyNativeStruct{}) = %v, wanted dummy_struct_adapted", val)
		}
	})

	t.Run("Value lookup when registered as value", func(t *testing.T) {
		val := reg.NativeToValue(dummyNativeStruct{})
		if IsError(val) || val.(String) != "dummy_struct_adapted" {
			t.Errorf("NativeToValue(dummyNativeStruct{}) = %v, wanted dummy_struct_adapted", val)
		}
	})

	t.Run("Direct findStructDescriptorByReflectType tests", func(t *testing.T) {
		pNil, foundNil := reg.findStructDescriptorByReflectType(nil)
		if foundNil || pNil != nil {
			t.Errorf("findStructDescriptorByReflectType(nil) expected false, got %v", foundNil)
		}

		// Manually set pointer-only key in reflectTypes
		reg.reflectTypes[reflect.TypeOf(&dummyNativeStruct{})] = st
		delete(reg.reflectTypes, reflect.TypeOf(dummyNativeStruct{}))

		// Pass value type (kind != Ptr) -> hits else branch looking up PointerTo
		stFound, foundVal := reg.findStructDescriptorByReflectType(reflect.TypeOf(dummyNativeStruct{}))
		if !foundVal || stFound != st {
			t.Errorf("findStructDescriptorByReflectType(value) expected st, got %v", stFound)
		}

		// Manually set value-only key in reflectTypes
		reg.reflectTypes[reflect.TypeOf(dummyNativeStruct{})] = st
		delete(reg.reflectTypes, reflect.TypeOf(&dummyNativeStruct{}))

		// Pass pointer type (kind == Ptr) -> hits Ptr branch looking up Elem
		stFound, foundPtr := reg.findStructDescriptorByReflectType(reflect.TypeOf(&dummyNativeStruct{}))
		if !foundPtr || stFound != st {
			t.Errorf("findStructDescriptorByReflectType(pointer) expected st, got %v", stFound)
		}
	})

	t.Run("FindFieldType on registered StructTypeDescriptor", func(t *testing.T) {
		ft, found := reg.FindFieldType("dummy.Struct", "DummyField")
		if !found || ft == nil {
			t.Fatalf("FindFieldType(dummy.Struct, DummyField) failed")
		}
	})

	t.Run("Invalid field type conversion error", func(t *testing.T) {
		stInvalid := &testDummyStructDescriptor{
			Type:        NewObjectType("invalid.Struct"),
			reflectType: reflect.TypeOf(customNativeType{}),
			fieldType:   &FieldType{Type: &Type{}},
		}
		err := reg.RegisterType(stInvalid)
		if err != nil {
			t.Fatalf("RegisterType(stInvalid) failed: %v", err)
		}
		_, found := reg.FindFieldType("invalid.Struct", "DummyField")
		if found {
			t.Errorf("FindFieldType(invalid.Struct, DummyField) expected false due to TypeToExprType error, got true")
		}
	})
}

func TestRegistry_FindFieldType_SimpleProviderFallback(t *testing.T) {
	baseReg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() failed: %v", err)
	}

	simpleProv := &testSimpleProvider{}
	p, _, err := ComposeTypes(simpleProv, baseReg)
	if err != nil {
		t.Fatalf("ComposeTypes() failed: %v", err)
	}
	composedReg := p.(*Registry)

	t.Run("fallback to FindStructFieldType", func(t *testing.T) {
		ft, found := composedReg.FindFieldType("simple.Struct", "simpleField")
		if !found || ft == nil {
			t.Fatalf("FindFieldType(simple.Struct, simpleField) failed on fallback provider")
		}
	})

	t.Run("FindStructFieldDescription without interface", func(t *testing.T) {
		_, found := composedReg.FindStructFieldDescription("simple.Struct", "simpleField")
		if found {
			t.Errorf("FindStructFieldDescription() on simpleProv expected false, got true")
		}
	})
}

func TestRegistry_NewRegistry_OptionErrors(t *testing.T) {
	optErr := RegistryOption(func(r *Registry) (*Registry, error) {
		return nil, fmt.Errorf("registry option error")
	})

	t.Run("NewProtoRegistry", func(t *testing.T) {
		_, err := NewProtoRegistry(optErr)
		if err == nil || err.Error() != "registry option error" {
			t.Errorf("NewProtoRegistry() expected 'registry option error', got %v", err)
		}
	})

	t.Run("NewRegistry", func(t *testing.T) {
		_, err := NewRegistry(optErr)
		if err == nil || err.Error() != "registry option error" {
			t.Errorf("NewRegistry() expected 'registry option error', got %v", err)
		}
	})
}

func TestRegistry_RegisterTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		targetType *Type
	}{
		{
			name:       "conflicting traits",
			targetType: NewTypeValue("bool", traits.ContainerType),
		},
		{
			name:       "conflicting type definition",
			targetType: NewObjectType("bool"),
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			reg, err := NewRegistry()
			if err != nil {
				t.Fatalf("NewRegistry() failed: %v", err)
			}
			err = reg.RegisterType(tc.targetType)
			if err == nil {
				t.Errorf("RegisterType() expected error, got nil")
			}
		})
	}
}

type sampleTaggedStruct struct {
	Greeting string `cel:"hello_str"`
	Count    int    `cel:"count_int"`
}

func TestRegistry_NativeReflectTypes(t *testing.T) {
	reg, err := NewRegistry(
		ParseStructTags(true),
		reflect.TypeFor[sampleTaggedStruct](),
	)
	if err != nil {
		t.Fatalf("NewRegistry(reflect.Type) failed: %v", err)
	}

	t.Run("FindStructType", func(t *testing.T) {
		st, found := reg.FindStructType("types.sampleTaggedStruct")
		if !found || st == nil {
			t.Fatalf("FindStructType(types.sampleTaggedStruct) not found")
		}
	})

	t.Run("FindStructFieldType with tag", func(t *testing.T) {
		ft, found := reg.FindStructFieldType("types.sampleTaggedStruct", "hello_str")
		if !found || ft == nil {
			t.Fatalf("FindStructFieldType(hello_str) not found")
		}
		if ft.Type != StringType {
			t.Errorf("FindStructFieldType(hello_str) got %v, want StringType", ft.Type)
		}
	})

	t.Run("NativeToValue", func(t *testing.T) {
		inst := sampleTaggedStruct{Greeting: "world", Count: 42}
		val := reg.NativeToValue(&inst)
		if IsError(val) {
			t.Fatalf("NativeToValue() failed: %v", val)
		}
		gotGreeting := val.(traits.Indexer).Get(String("hello_str"))
		if gotGreeting.Equal(String("world")) != True {
			t.Errorf("Get(hello_str) = %v, want 'world'", gotGreeting)
		}
	})
}

func TestParseStructTag(t *testing.T) {
	tests := []struct {
		name        string
		tag         string
		tagName     string
		defaultName string
		want        structTagInfo
	}{
		{
			name:        "no_tag",
			tag:         ``,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "DefaultField", OmitEmpty: false, Skip: false, HasTag: false},
		},
		{
			name:        "hyphen_skip",
			tag:         `json:"-"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "", OmitEmpty: false, Skip: true, HasTag: true},
		},
		{
			name:        "hyphen_with_trailing_comma_skip",
			tag:         `json:"-,"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "", OmitEmpty: false, Skip: true, HasTag: true},
		},
		{
			name:        "hyphen_with_omitempty_skip",
			tag:         `json:"-,omitempty"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "", OmitEmpty: false, Skip: true, HasTag: true},
		},
		{
			name:        "quoted_hyphen_literal",
			tag:         `json:"'-'"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "-", OmitEmpty: false, Skip: false, HasTag: true},
		},
		{
			name:        "quoted_hyphen_with_omitempty",
			tag:         `json:"'-',omitempty"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "-", OmitEmpty: true, Skip: false, HasTag: true},
		},
		{
			name:        "custom_name",
			tag:         `json:"custom_name"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "custom_name", OmitEmpty: false, Skip: false, HasTag: true},
		},
		{
			name:        "custom_name_omitempty",
			tag:         `json:"custom_name,omitempty"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "custom_name", OmitEmpty: true, Skip: false, HasTag: true},
		},
		{
			name:        "empty_name_with_omitempty",
			tag:         `json:",omitempty"`,
			tagName:     "json",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "DefaultField", OmitEmpty: true, Skip: false, HasTag: true},
		},
		{
			name:        "cel_tag_hyphen_skip",
			tag:         `cel:"-"`,
			tagName:     "cel",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "", OmitEmpty: false, Skip: true, HasTag: true},
		},
		{
			name:        "cel_tag_quoted_hyphen",
			tag:         `cel:"'-'"`,
			tagName:     "cel",
			defaultName: "DefaultField",
			want:        structTagInfo{Name: "-", OmitEmpty: false, Skip: false, HasTag: true},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			field := reflect.StructField{
				Name: "DefaultField",
				Tag:  reflect.StructTag(tc.tag),
			}
			got := parseStructTag(field, tc.tagName, tc.defaultName)
			if got != tc.want {
				t.Errorf("parseStructTag(%q, %q, %q) = %+v, want %+v", tc.tag, tc.tagName, tc.defaultName, got, tc.want)
			}
		})
	}
}

