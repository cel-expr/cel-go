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
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types"

	celpb "cel.dev/expr"
)

func TestTypeProtoRoundTrip(t *testing.T) {
	tests := []string{
		``,
		`dyn {}`,
		`null: NULL_VALUE`,
		`primitive: BOOL`,
		`primitive: INT64`,
		`primitive: UINT64`,
		`primitive: DOUBLE`,
		`primitive: STRING`,
		`primitive: BYTES`,
		`wrapper: BOOL`,
		`wrapper: INT64`,
		`wrapper: UINT64`,
		`wrapper: DOUBLE`,
		`wrapper: STRING`,
		`wrapper: BYTES`,
		`well_known: ANY`,
		`well_known: TIMESTAMP`,
		`well_known: DURATION`,
		`list_type { elem_type { primitive: INT64 } }`,
		`list_type { elem_type {} }`,
		`map_type { key_type { primitive: STRING } value_type { primitive: DOUBLE } }`,
		`map_type { key_type {} value_type { primitive: INT64 } }`,
		`function { result_type { primitive: BOOL } arg_types { primitive: INT64 } arg_types { primitive: BYTES } }`,
		`function { result_type { primitive: BOOL } }`,
		`message_type: "com.example.Message"`,
		`type_param: "T"`,
		`type { primitive: UINT64 }`,
		`type {}`,
		`error {}`,
		`abstract_type { name: "MyType" }`,
		`abstract_type { name: "MyType" parameter_types { primitive: STRING } }`,
	}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			input := &celpb.Type{}
			if err := prototext.Unmarshal([]byte(tc), input); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			native, err := typeFromProto(input)
			if err != nil {
				t.Fatalf("typeFromProto() failed: %v", err)
			}
			roundTrip, err := typeToProto(native)
			if err != nil {
				t.Fatalf("typeToProto() failed: %v", err)
			}
			if !proto.Equal(roundTrip, input) {
				t.Errorf("typeToProto(typeFromProto(%v)) = %v, want %v", input, roundTrip, input)
			}
		})
	}
}

func TestTypeFromProto(t *testing.T) {
	tests := []struct {
		name string
		in   *celpb.Type
		want *types.Type
	}{
		{name: "nil", in: nil, want: &types.Type{}},
		{name: "function", in: &celpb.Type{TypeKind: &celpb.Type_Function{Function: &celpb.Type_FunctionType{
			ResultType: &celpb.Type{TypeKind: &celpb.Type_Primitive{Primitive: celpb.Type_BOOL}},
			ArgTypes:   []*celpb.Type{{TypeKind: &celpb.Type_Primitive{Primitive: celpb.Type_INT64}}},
		}}}, want: types.NewOpaqueType("function", types.BoolType, types.IntType)},
		{name: "type type", in: &celpb.Type{TypeKind: &celpb.Type_Type{Type: &celpb.Type{}}}, want: types.TypeType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := typeFromProto(tc.in)
			if err != nil {
				t.Fatalf("typeFromProto() failed: %v", err)
			}
			if !got.IsExactType(tc.want) || got.Kind() != tc.want.Kind() {
				t.Errorf("typeFromProto() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTypeFromProtoWrapperIsNullable(t *testing.T) {
	got, err := typeFromProto(&celpb.Type{TypeKind: &celpb.Type_Wrapper{Wrapper: celpb.Type_STRING}})
	if err != nil {
		t.Fatalf("typeFromProto() failed: %v", err)
	}
	if got.Kind() != types.StringKind || !got.IsAssignableType(types.NullType) {
		t.Errorf("typeFromProto(wrapper: STRING) = %v, want nullable string", got)
	}
}

func TestTypeFromProtoErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "invalid primitive", in: `primitive: 999`, wantErr: "PrimitiveType"},
		{name: "unspecified primitive", in: `primitive: PRIMITIVE_TYPE_UNSPECIFIED`, wantErr: "PrimitiveType"},
		{name: "invalid wrapper", in: `wrapper: 999`, wantErr: "PrimitiveType"},
		{name: "unspecified wrapper", in: `wrapper: PRIMITIVE_TYPE_UNSPECIFIED`, wantErr: "PrimitiveType"},
		{name: "invalid well-known", in: `well_known: 999`, wantErr: "WellKnownType"},
		{name: "unspecified well-known", in: `well_known: WELL_KNOWN_TYPE_UNSPECIFIED`, wantErr: "WellKnownType"},
		{name: "nested list elem", in: `list_type { elem_type { primitive: 999 } }`, wantErr: "PrimitiveType"},
		{name: "nested map key", in: `map_type { key_type { primitive: 999 } }`, wantErr: "PrimitiveType"},
		{name: "nested map value", in: `map_type { value_type { well_known: 999 } }`, wantErr: "WellKnownType"},
		{name: "nested function result", in: `function { result_type { primitive: 999 } }`, wantErr: "PrimitiveType"},
		{name: "nested function arg", in: `function { arg_types { primitive: 999 } }`, wantErr: "PrimitiveType"},
		{name: "nested type param", in: `type { primitive: 999 }`, wantErr: "PrimitiveType"},
		{name: "nested abstract param", in: `abstract_type { name: "A" parameter_types { primitive: 999 } }`, wantErr: "PrimitiveType"},
		{name: "reserved abstract name", in: `abstract_type { name: "function" }`, wantErr: "reserved"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := &celpb.Type{}
			if err := prototext.Unmarshal([]byte(tc.in), in); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			_, err := typeFromProto(in)
			if !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("typeFromProto() got error %v, want ErrBadTypeDef containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestTypeProtoMaxDepth(t *testing.T) {
	pb := &celpb.Type{TypeKind: &celpb.Type_Dyn{}}
	native := types.DynType
	for range maxTypeDepth {
		pb = &celpb.Type{TypeKind: &celpb.Type_ListType_{ListType: &celpb.Type_ListType{ElemType: pb}}}
		native = types.NewListType(native)
	}
	if _, err := typeFromProto(pb); err != nil {
		t.Errorf("typeFromProto() at max depth failed: %v", err)
	}
	if _, err := typeToProto(native); err != nil {
		t.Errorf("typeToProto() at max depth failed: %v", err)
	}

	pb = &celpb.Type{TypeKind: &celpb.Type_ListType_{ListType: &celpb.Type_ListType{ElemType: pb}}}
	native = types.NewListType(native)
	if _, err := typeFromProto(pb); !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), "max depth") {
		t.Errorf("typeFromProto() beyond max depth got error %v, want ErrBadTypeDef", err)
	}
	if _, err := typeToProto(native); !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), "max depth") {
		t.Errorf("typeToProto() beyond max depth got error %v, want ErrBadTypeDef", err)
	}
}

func TestTypeToProto(t *testing.T) {
	tests := []struct {
		name string
		in   *types.Type
		want string
	}{
		{name: "nil", in: nil, want: ``},
		{name: "unspecified", in: &types.Type{}, want: ``},
		{name: "type without param", in: types.TypeType, want: `type {}`},
		{name: "type with param", in: types.NewTypeTypeWithParam(types.IntType), want: `type { primitive: INT64 }`},
		{name: "object", in: types.NewObjectType("com.example.Message"), want: `message_type: "com.example.Message"`},
		{name: "nullable", in: types.NewNullableType(types.IntType), want: `wrapper: INT64`},
		{name: "opaque", in: types.NewOpaqueType("A", types.StringType), want: `abstract_type { name: "A" parameter_types { primitive: STRING } }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := &celpb.Type{}
			if err := prototext.Unmarshal([]byte(tc.want), want); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			got, err := typeToProto(tc.in)
			if err != nil {
				t.Fatalf("typeToProto() failed: %v", err)
			}
			if !proto.Equal(got, want) {
				t.Errorf("typeToProto() = %v, want %v", got, want)
			}
		})
	}
}

// Message types naming protobuf well-known types are canonicalized by
// types.NewObjectType, so they intentionally do not round-trip as
// message_type.
func TestTypeProtoCanonicalizesWellKnownMessageTypes(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: `message_type: "google.protobuf.Int64Value"`, want: `wrapper: INT64`},
		{in: `message_type: "google.protobuf.Duration"`, want: `well_known: DURATION`},
		{in: `message_type: "google.protobuf.Struct"`, want: `map_type { key_type { primitive: STRING } value_type { dyn {} } }`},
		{in: `message_type: "google.protobuf.ListValue"`, want: `list_type { elem_type { dyn {} } }`},
		{in: `message_type: "google.protobuf.Value"`, want: `dyn {}`},
		{in: `message_type: "google.protobuf.NullValue"`, want: `null: NULL_VALUE`},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			in := &celpb.Type{}
			if err := prototext.Unmarshal([]byte(tc.in), in); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			want := &celpb.Type{}
			if err := prototext.Unmarshal([]byte(tc.want), want); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			native, err := typeFromProto(in)
			if err != nil {
				t.Fatalf("typeFromProto() failed: %v", err)
			}
			got, err := typeToProto(native)
			if err != nil {
				t.Fatalf("typeToProto() failed: %v", err)
			}
			if !proto.Equal(got, want) {
				t.Errorf("typeToProto(typeFromProto(%v)) = %v, want %v", in, got, want)
			}
		})
	}
}

func TestTypeToProtoErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      *types.Type
		wantErr string
	}{
		{name: "unknown kind", in: types.UnknownType, wantErr: "unsupported type kind"},
		{name: "function without result", in: types.NewOpaqueType("function"), wantErr: "missing a result type"},
		{name: "nested list elem", in: types.NewListType(types.UnknownType), wantErr: "unsupported type kind"},
		{name: "nested map key", in: types.NewMapType(types.UnknownType, types.IntType), wantErr: "unsupported type kind"},
		{name: "nested map value", in: types.NewMapType(types.IntType, types.UnknownType), wantErr: "unsupported type kind"},
		{name: "nested type param", in: types.NewTypeTypeWithParam(types.UnknownType), wantErr: "unsupported type kind"},
		{name: "nested opaque param", in: types.NewOpaqueType("A", types.UnknownType), wantErr: "unsupported type kind"},
		{name: "nested function arg", in: types.NewOpaqueType("function", types.BoolType, types.UnknownType), wantErr: "unsupported type kind"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := typeToProto(tc.in)
			if !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("typeToProto() got error %v, want ErrBadTypeDef containing %q", err, tc.wantErr)
			}
		})
	}
}
