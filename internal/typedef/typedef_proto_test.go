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

package typedef_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/internal/typedef"
	"cel.dev/cel-go/internal/typedef/typedefpb"
)

// invalidTimestamp is outside the range representable by
// google.protobuf.Timestamp, so encoding it fails.
var invalidTimestamp = types.Timestamp{Time: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}

func TestTypeDefProtoRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		textproto string
		// want is the expected re-encoded textproto; defaults to textproto.
		want string
	}{
		{name: "empty struct", textproto: `struct_type {}`},
		{name: "empty enum", textproto: `enum_type {}`},
		{
			name: "unset field type",
			textproto: `struct_type {
				name: "com.example.UnsetFieldType"
				fields { name: "unset_type_field" type {} }
			}`,
			want: `struct_type {
				name: "com.example.UnsetFieldType"
				fields { name: "unset_type_field" }
			}`,
		},
		{
			name: "unset field default",
			textproto: `struct_type {
				name: "com.example.UnsetFieldDefault"
				fields { name: "unset_default_field" default_value {} }
			}`,
			want: `struct_type {
				name: "com.example.UnsetFieldDefault"
				fields { name: "unset_default_field" }
			}`,
		},
		{
			name: "struct with fields and aspects",
			textproto: `struct_type {
				name: "com.example.Order"
				doc: "Customer order entity."
				fields {
					name: "order_id"
					type { primitive: STRING }
					doc: "Unique order ID."
					aspects {
						key: "proto"
						value {
							properties { key: "id" value { int64_value: 1 } }
							properties { key: "json_name" value { string_value: "orderId" } }
							properties { key: "name" value { string_value: "order_id_proto" } }
							properties { key: "unset" value {} }
						}
					}
				}
				fields {
					name: "empty_aspect_field"
					type { primitive: STRING }
					aspects { key: "proto" value {} }
				}
				aspects {
					key: "custom"
					value { properties { key: "version" value { int64_value: 2 } } }
				}
				aspects {
					key: "proto"
					value { properties { key: "name" value { string_value: "com.example.proto.Order" } } }
				}
			}`,
		},
		{
			name: "enum with constants and aspects",
			textproto: `enum_type {
				name: "com.example.OrderStatus"
				doc: "Lifecycle status of an order."
				constants {
					name: "ORDER_STATUS_ACTIVE"
					id: 1
					doc: "Active status."
					aspects {
						key: "proto"
						value { properties { key: "name" value { string_value: "STATUS_ACTIVE" } } }
					}
				}
				constants {
					name: "ORDER_STATUS_LEGACY"
					id: -1
					aspects { key: "proto" value {} }
				}
				aspects {
					key: "proto"
					value {
						properties { key: "allow_alias" value { bool_value: true } }
						properties { key: "name" value { string_value: "com.example.proto.OrderStatus" } }
					}
				}
			}`,
		},
		{
			name: "enum constant with zero id",
			textproto: `enum_type {
				name: "com.example.Zero"
				constants { name: "ZERO" id: 0 }
			}`,
		},
		{
			name: "enum constant with unset id",
			textproto: `enum_type {
				name: "com.example.Unset"
				constants { name: "UNSET" }
			}`,
			want: `enum_type {
				name: "com.example.Unset"
				constants { name: "UNSET" id: 0 }
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := &typedefpb.TypeDef{}
			if err := prototext.Unmarshal([]byte(tc.textproto), input); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			decoded, err := typedef.TypeDefFromProto(input)
			if err != nil {
				t.Fatalf("TypeDefFromProto() failed: %v", err)
			}
			encoded, err := typedef.TypeDefToProto(decoded)
			if err != nil {
				t.Fatalf("TypeDefToProto() failed: %v", err)
			}
			wantText := tc.want
			if wantText == "" {
				wantText = tc.textproto
			}
			want := &typedefpb.TypeDef{}
			if err := prototext.Unmarshal([]byte(wantText), want); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			if !proto.Equal(encoded, want) {
				t.Errorf("TypeDefToProto(TypeDefFromProto()) = %v, want %v", encoded, want)
			}
		})
	}
}

func TestTypeDefFromProtoDecodesValues(t *testing.T) {
	input := &typedefpb.TypeDef{}
	err := prototext.Unmarshal([]byte(`struct_type {
		name: "com.example.Order"
		doc: "Order."
		fields {
			name: "count"
			type { primitive: INT64 }
			doc: "Item count."
			default_value { int64_value: 3 }
			aspects { key: "proto" value { properties { key: "id" value { int64_value: 1 } } } }
		}
		aspects { key: "proto" value { properties { key: "name" value { string_value: "pb.Order" } } } }
	}`), input)
	if err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	td, err := typedef.TypeDefFromProto(input)
	if err != nil {
		t.Fatalf("TypeDefFromProto() failed: %v", err)
	}
	s := td.StructType()
	if s.Name() != "com.example.Order" || s.Doc() != "Order." {
		t.Errorf("struct name, doc = %q, %q, want %q, %q", s.Name(), s.Doc(), "com.example.Order", "Order.")
	}
	if a, ok := s.Aspect("proto"); !ok {
		t.Error("struct aspect \"proto\" not found")
	} else if v, _ := a.GetProperty("name"); v != types.String("pb.Order") {
		t.Errorf("struct aspect property \"name\" = %v, want %q", v, "pb.Order")
	}
	f, ok := s.FindField("count")
	if !ok {
		t.Fatal("field \"count\" not found")
	}
	if !f.Type().IsExactType(types.IntType) || f.Doc() != "Item count." || f.DefaultValue() != types.Int(3) {
		t.Errorf("field = (%v, %q, %v), want (int, %q, 3)", f.Type(), f.Doc(), f.DefaultValue(), "Item count.")
	}
	if a, ok := f.Aspect("proto"); !ok {
		t.Error("field aspect \"proto\" not found")
	} else if v, _ := a.GetProperty("id"); v != types.Int(1) {
		t.Errorf("field aspect property \"id\" = %v, want 1", v)
	}
}

func TestTypeDefFromProtoErrors(t *testing.T) {
	tests := []struct {
		name      string
		textproto string
		wantErr   error
		wantMsg   string
	}{
		{
			name:    "missing kind",
			wantErr: typedef.ErrBadTypeDef,
			wantMsg: "cel.types.TypeDef must specify 'struct_type' or 'enum_type'",
		},
		{
			name:      "empty field name",
			textproto: `struct_type { name: "com.example.EmptyFieldName" fields { name: "" } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "field name cannot be empty",
		},
		{
			name:      "duplicate fields",
			textproto: `struct_type { name: "com.example.DupFields" fields { name: "dup" } fields { name: "dup" } }`,
			wantErr:   typedef.ErrAlreadyExists,
			wantMsg:   `field "dup"`,
		},
		{
			name:      "empty enum constant name",
			textproto: `enum_type { name: "com.example.EmptyConstantName" constants { name: "" } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "enum constant name cannot be empty",
		},
		{
			name:      "duplicate enum constants",
			textproto: `enum_type { name: "com.example.DupConstants" constants { name: "DUP_VAL" } constants { name: "DUP_VAL" } }`,
			wantErr:   typedef.ErrAlreadyExists,
			wantMsg:   `enum constant "DUP_VAL"`,
		},
		{
			name:      "invalid primitive field type",
			textproto: `struct_type { name: "com.example.BadPrim" fields { name: "f" type { primitive: 999 } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "PrimitiveType",
		},
		{
			name:      "invalid well-known field type",
			textproto: `struct_type { name: "com.example.BadWk" fields { name: "f" type { well_known: 999 } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "WellKnownType",
		},
		{
			name:      "invalid field default",
			textproto: `struct_type { name: "com.example.BadDefault" fields { name: "f" default_value { duration_value { nanos: -1 seconds: 1 } } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "invalid duration",
		},
		{
			name:      "invalid struct aspect",
			textproto: `struct_type { name: "com.example.BadAspect" aspects { key: "proto" value { properties { key: "d" value { timestamp_value { seconds: 253402300800 } } } } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   `aspect "proto"`,
		},
		{
			name:      "invalid field aspect",
			textproto: `struct_type { name: "com.example.BadFieldAspect" fields { name: "f" aspects { key: "proto" value { properties { key: "d" value { timestamp_value { seconds: 253402300800 } } } } } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   `aspect property "d"`,
		},
		{
			name:      "invalid enum aspect",
			textproto: `enum_type { name: "com.example.BadEnumAspect" aspects { key: "proto" value { properties { key: "d" value { timestamp_value { seconds: 253402300800 } } } } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   `aspect "proto"`,
		},
		{
			name:      "invalid enum constant aspect",
			textproto: `enum_type { name: "com.example.BadEnumConstAspect" constants { name: "C" aspects { key: "proto" value { properties { key: "d" value { timestamp_value { seconds: 253402300800 } } } } } } }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   `aspect property "d"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := &typedefpb.TypeDef{}
			if err := prototext.Unmarshal([]byte(tc.textproto), input); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			_, err := typedef.TypeDefFromProto(input)
			if !errors.Is(err, tc.wantErr) || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("TypeDefFromProto() got error %v, want %v containing %q", err, tc.wantErr, tc.wantMsg)
			}
		})
	}
}

func TestTypeDefToProtoErrors(t *testing.T) {
	invalidAspect := map[string]*typedef.Aspect{
		"proto": mustAspect(t, map[string]ref.Val{"d": invalidTimestamp}),
	}
	mustStruct := func(name string, opts ...typedef.StructOpt) *typedef.TypeDef {
		t.Helper()
		s, err := typedef.NewStruct(name, opts...)
		if err != nil {
			t.Fatalf("NewStruct(%q) failed: %v", name, err)
		}
		return s.AsTypeDef()
	}
	mustEnum := func(name string, opts ...typedef.EnumOpt) *typedef.TypeDef {
		t.Helper()
		e, err := typedef.NewEnum(name, opts...)
		if err != nil {
			t.Fatalf("NewEnum(%q) failed: %v", name, err)
		}
		return e.AsTypeDef()
	}
	tests := []struct {
		name    string
		typeDef *typedef.TypeDef
		wantMsg string
	}{
		{name: "nil", typeDef: nil, wantMsg: "nil or unspecified"},
		{name: "unspecified", typeDef: &typedef.TypeDef{}, wantMsg: "nil or unspecified"},
		{
			name:    "unsupported field type",
			typeDef: mustStruct("com.example.BadType", typedef.StructFields(mustField(t, "f", types.UnknownType))),
			wantMsg: "unsupported type kind",
		},
		{
			name: "invalid field default",
			typeDef: mustStruct("com.example.BadFieldDefault", typedef.StructFields(
				mustField(t, "f", types.TimestampType, typedef.FieldDefaultValue(invalidTimestamp)))),
			wantMsg: "default value",
		},
		{
			name:    "invalid struct aspect",
			typeDef: mustStruct("com.example.BadStructAspect", typedef.StructAspects(invalidAspect)),
			wantMsg: `aspect "proto"`,
		},
		{
			name: "invalid field aspect",
			typeDef: mustStruct("com.example.BadFieldAspect", typedef.StructFields(
				mustField(t, "f", nil, typedef.FieldAspects(invalidAspect)))),
			wantMsg: `field "f": aspect "proto"`,
		},
		{
			name:    "invalid enum aspect",
			typeDef: mustEnum("com.example.BadEnumAspect", typedef.EnumAspects(invalidAspect)),
			wantMsg: `aspect "proto"`,
		},
		{
			name: "invalid enum constant aspect",
			typeDef: mustEnum("com.example.BadEnumConstAspect", typedef.EnumConstants(
				mustEnumConstant(t, "C", 0, typedef.EnumConstantAspects(invalidAspect)))),
			wantMsg: `constant "C": aspect "proto"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := typedef.TypeDefToProto(tc.typeDef)
			if !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("TypeDefToProto() got error %v, want ErrBadTypeDef containing %q", err, tc.wantMsg)
			}
		})
	}
}

func TestTypeDefSetProtoTextProtoRoundTrip(t *testing.T) {
	contents, err := os.ReadFile("testdata/cel_types.txtproto")
	if err != nil {
		t.Fatalf("os.ReadFile() failed: %v", err)
	}
	input := &typedefpb.TypeDefSet{}
	if err := prototext.Unmarshal(contents, input); err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	if len(input.GetTypes()) == 0 {
		t.Fatal("testdata/cel_types.txtproto contains no types")
	}
	set, err := typedef.TypeDefSetFromProto(input)
	if err != nil {
		t.Fatalf("TypeDefSetFromProto() failed: %v", err)
	}
	if got, want := len(set.Types()), len(input.GetTypes()); got != want {
		t.Errorf("len(TypeDefSet.Types()) = %d, want %d", got, want)
	}
	if _, found := set.FindTypeDef("com.example.store.Order"); !found {
		t.Error("FindTypeDef(\"com.example.store.Order\") not found")
	}
	roundTrip, err := typedef.TypeDefSetToProto(set)
	if err != nil {
		t.Fatalf("TypeDefSetToProto() failed: %v", err)
	}
	if !proto.Equal(roundTrip, input) {
		t.Errorf("TypeDefSetToProto(TypeDefSetFromProto()) = %v, want %v", roundTrip, input)
	}
}

func TestTypeDefSetToProtoNil(t *testing.T) {
	got, err := typedef.TypeDefSetToProto(nil)
	if err != nil {
		t.Fatalf("TypeDefSetToProto(nil) failed: %v", err)
	}
	if !proto.Equal(got, &typedefpb.TypeDefSet{}) {
		t.Errorf("TypeDefSetToProto(nil) = %v, want empty", got)
	}
}

func TestTypeDefSetProtoErrors(t *testing.T) {
	fromProtoTests := []struct {
		name      string
		textproto string
		wantErr   error
		wantMsg   string
	}{
		{
			name:      "unnamed type",
			textproto: `types { struct_type {} }`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "type name cannot be empty",
		},
		{
			name:      "duplicate type",
			textproto: `types { struct_type { name: "com.example.Dup" } } types { enum_type { name: "com.example.Dup" } }`,
			wantErr:   typedef.ErrAlreadyExists,
			wantMsg:   `type "com.example.Dup"`,
		},
		{
			name:      "invalid element",
			textproto: `types { struct_type { name: "com.example.Valid" } } types {}`,
			wantErr:   typedef.ErrBadTypeDef,
			wantMsg:   "types[1]: invalid typedef: cel.types.TypeDef must specify 'struct_type' or 'enum_type'",
		},
	}
	for _, tc := range fromProtoTests {
		t.Run(tc.name, func(t *testing.T) {
			input := &typedefpb.TypeDefSet{}
			if err := prototext.Unmarshal([]byte(tc.textproto), input); err != nil {
				t.Fatalf("prototext.Unmarshal() failed: %v", err)
			}
			_, err := typedef.TypeDefSetFromProto(input)
			if !errors.Is(err, tc.wantErr) || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("TypeDefSetFromProto() got error %v, want %v containing %q", err, tc.wantErr, tc.wantMsg)
			}
		})
	}

	valid, err := typedef.NewStruct("com.example.Valid")
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}
	badAspect, err := typedef.NewStruct("com.example.BadAspect",
		typedef.StructAspect("proto", mustAspect(t, map[string]ref.Val{"d": invalidTimestamp})))
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}
	set, err := typedef.NewTypeDefSet(valid.AsTypeDef(), badAspect.AsTypeDef())
	if err != nil {
		t.Fatalf("NewTypeDefSet() failed: %v", err)
	}
	if _, err := typedef.TypeDefSetToProto(set); !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), "types[1]") {
		t.Errorf("TypeDefSetToProto() got error %v, want ErrBadTypeDef for types[1]", err)
	}
}
