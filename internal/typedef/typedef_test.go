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
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/internal/typedef"
)

type testStructProperties struct {
	CustomName string
}

type testFieldProperties struct {
	JSONName string
}

type testEnumProperties struct {
	AllowAlias bool
}

type testEnumConstantProperties struct {
	CustomName string
}

type fieldSnapshot struct {
	Name            string
	Type            *types.Type
	Doc             string
	HasDefaultValue bool
	DefaultValue    ref.Val
	Aspects         map[string]any
}

type enumConstantSnapshot struct {
	Name    string
	ID      int32
	Doc     string
	Aspects map[string]any
}

var cmpOpts = cmp.Options{
	// Compare both *types.Type and scalar ref.Val values in a single comparer:
	//  1. *types.Type implements ref.Val, so registering separate comparers for
	//     *types.Type and ref.Val causes go-cmp to panic with ambiguous options
	//     on Field.Type. Using IsExactType preserves type parameters (unlike
	//     (*types.Type).Equal, which only checks type names).
	//  2. CEL's Equal performs cross-numeric comparisons (Int(1) == Uint(1) ==
	//     Double(1.0)), whereas schema field defaults must match the exact CEL
	//     scalar type, so a.Type() != b.Type() is rejected first.
	//  3. CEL's Double.Equal follows IEEE-754 (NaN != NaN), so NaN is handled
	//     explicitly to satisfy cmp.Comparer's reflexivity contract.
	cmp.Comparer(func(a, b ref.Val) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		ta, aIsType := a.(*types.Type)
		tb, bIsType := b.(*types.Type)
		if aIsType || bIsType {
			if !aIsType || !bIsType {
				return false
			}
			if ta == nil || tb == nil {
				return ta == tb
			}
			return ta.IsExactType(tb)
		}
		if a.Type() != b.Type() {
			return false
		}
		if da, ok := a.(types.Double); ok && math.IsNaN(float64(da)) {
			db, ok := b.(types.Double)
			return ok && math.IsNaN(float64(db))
		}
		return a.Equal(b) == types.True
	}),
	cmp.Transformer("Field", func(f *typedef.Field) *fieldSnapshot {
		if f == nil {
			return nil
		}
		return &fieldSnapshot{
			Name:            f.Name(),
			Type:            f.Type(),
			Doc:             f.Doc(),
			HasDefaultValue: f.HasDefaultValue(),
			DefaultValue:    f.DefaultValue(),
			Aspects:         f.Aspects(),
		}
	}),
	cmp.Transformer("EnumConstant", func(c *typedef.EnumConstant) *enumConstantSnapshot {
		if c == nil {
			return nil
		}
		return &enumConstantSnapshot{
			Name:    c.Name(),
			ID:      c.ID(),
			Doc:     c.Doc(),
			Aspects: c.Aspects(),
		}
	}),
}

func mustField(t *testing.T, name string, ft *types.Type, opts ...typedef.FieldOpt) *typedef.Field {
	t.Helper()
	f, err := typedef.NewField(name, ft, opts...)
	if err != nil {
		t.Fatalf("NewField(%q) unexpected error: %v", name, err)
	}
	return f
}

func mustEnumConstant(t *testing.T, name string, id int32, opts ...typedef.EnumConstantOpt) *typedef.EnumConstant {
	t.Helper()
	c, err := typedef.NewEnumConstant(name, id, opts...)
	if err != nil {
		t.Fatalf("NewEnumConstant(%q, %d) unexpected error: %v", name, id, err)
	}
	return c
}

func TestKindString(t *testing.T) {
	cases := []struct {
		kind typedef.Kind
		want string
	}{
		{kind: typedef.UnspecifiedKind, want: "unspecified"},
		{kind: typedef.StructKind, want: "struct"},
		{kind: typedef.EnumKind, want: "enum"},
		{kind: typedef.Kind(99), want: "unspecified"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(tc.kind), got, tc.want)
		}
	}
}

func TestZeroValueTypeDef(t *testing.T) {
	var def typedef.TypeDef
	if got := def.Kind(); got != typedef.UnspecifiedKind {
		t.Errorf("def.Kind() = %v, want %v", got, typedef.UnspecifiedKind)
	}
	if def.IsStruct() {
		t.Errorf("def.IsStruct() = true, want false")
	}
	if def.IsEnum() {
		t.Errorf("def.IsEnum() = true, want false")
	}
	if got := def.StructType(); got != nil {
		t.Errorf("def.StructType() = %v, want nil", got)
	}
	if got := def.EnumType(); got != nil {
		t.Errorf("def.EnumType() = %v, want nil", got)
	}
}

func TestFromStructNil(t *testing.T) {
	def := typedef.FromStruct(nil)
	if got := def.Kind(); got != typedef.StructKind {
		t.Errorf("def.Kind() = %v, want %v", got, typedef.StructKind)
	}
	if !def.IsStruct() {
		t.Errorf("def.IsStruct() = false, want true")
	}
	if def.IsEnum() {
		t.Errorf("def.IsEnum() = true, want false")
	}
	if got := def.EnumType(); got != nil {
		t.Errorf("def.EnumType() = %v, want nil", got)
	}
	st := def.StructType()
	if st == nil {
		t.Fatalf("def.StructType() = nil, want non-nil empty StructTypeDef")
	}
	if def.StructType() != st {
		t.Errorf("def.StructType() returned distinct pointers across calls")
	}
	if got := st.Name(); got != "" {
		t.Errorf("st.Name() = %q, want \"\"", got)
	}
	if got := st.Doc(); got != "" {
		t.Errorf("st.Doc() = %q, want \"\"", got)
	}
	if got := st.Aspects(); got != nil {
		t.Errorf("st.Aspects() = %v, want nil", got)
	}
	if got := st.Fields(); got != nil {
		t.Errorf("st.Fields() = %v, want nil", got)
	}
	if f, ok := st.FindField("id"); ok || f != nil {
		t.Errorf("st.FindField(\"id\") = (%v, %v), want (nil, false)", f, ok)
	}
	// Mutating the underlying empty struct persists across StructType() calls.
	if err := st.AddField(mustField(t, "id", types.IntType)); err != nil {
		t.Fatalf("st.AddField(id) unexpected error: %v", err)
	}
	if _, ok := def.StructType().FindField("id"); !ok {
		t.Errorf("def.StructType().FindField(\"id\") = false, want true")
	}
}

func TestFromEnumNil(t *testing.T) {
	def := typedef.FromEnum(nil)
	if got := def.Kind(); got != typedef.EnumKind {
		t.Errorf("def.Kind() = %v, want %v", got, typedef.EnumKind)
	}
	if def.IsStruct() {
		t.Errorf("def.IsStruct() = true, want false")
	}
	if !def.IsEnum() {
		t.Errorf("def.IsEnum() = false, want true")
	}
	if got := def.StructType(); got != nil {
		t.Errorf("def.StructType() = %v, want nil", got)
	}
	en := def.EnumType()
	if en == nil {
		t.Fatalf("def.EnumType() = nil, want non-nil empty EnumTypeDef")
	}
	if def.EnumType() != en {
		t.Errorf("def.EnumType() returned distinct pointers across calls")
	}
	if got := en.Name(); got != "" {
		t.Errorf("en.Name() = %q, want \"\"", got)
	}
	if got := en.Doc(); got != "" {
		t.Errorf("en.Doc() = %q, want \"\"", got)
	}
	if got := en.Aspects(); got != nil {
		t.Errorf("en.Aspects() = %v, want nil", got)
	}
	if got := en.Constants(); got != nil {
		t.Errorf("en.Constants() = %v, want nil", got)
	}
	if c, ok := en.FindConstant("ANY"); ok || c != nil {
		t.Errorf("en.FindConstant(\"ANY\") = (%v, %v), want (nil, false)", c, ok)
	}
}

func TestStructTypeDefConstructionAndLookup(t *testing.T) {
	person, err := typedef.NewStruct(
		"com.example.Person",
		typedef.StructDoc("User account profile entity."),
		typedef.StructAspects(map[string]any{
			"json": testStructProperties{CustomName: "Person"},
		}),
	)
	if err != nil {
		t.Fatalf("NewStruct() unexpected error: %v", err)
	}

	idField := mustField(t, "id", types.IntType, typedef.FieldDoc("Unique user identifier."))
	displayNameField := mustField(
		t,
		"display_name",
		types.StringType,
		typedef.FieldDoc("Display name."),
		typedef.FieldDefaultValue(types.String("Anonymous")),
		typedef.FieldAspects(map[string]any{
			"json": testFieldProperties{JSONName: "displayName"},
		}),
		typedef.FieldAspect("proto3", testFieldProperties{JSONName: "display_name"}),
	)
	avatarField := mustField(
		t,
		"avatar",
		types.BytesType,
		typedef.FieldDefaultValue(types.Bytes([]byte{0x01, 0x02})),
	)
	tagsField := mustField(t, "tags", types.NewListType(types.StringType))
	untypedField := mustField(t, "untyped", nil)
	if untypedField.Type() == nil || untypedField.Type().Kind() != types.UnspecifiedKind {
		t.Errorf("untypedField.Type() = %v, want non-nil UnspecifiedKind", untypedField.Type())
	}
	var zeroField typedef.Field
	if zeroField.Type() == nil || zeroField.Type().Kind() != types.UnspecifiedKind {
		t.Errorf("zeroField.Type() = %v, want non-nil UnspecifiedKind", zeroField.Type())
	}

	for _, f := range []*typedef.Field{idField, displayNameField, avatarField, tagsField, untypedField} {
		if err := person.AddField(f); err != nil {
			t.Fatalf("person.AddField(%q) unexpected error: %v", f.Name(), err)
		}
	}

	wantFields := []*typedef.Field{idField, displayNameField, avatarField, tagsField, untypedField}
	if diff := cmp.Diff(wantFields, person.Fields(), cmpOpts); diff != "" {
		t.Errorf("person.Fields() mismatch (-want +got):\n%s", diff)
	}

	wantAspects := map[string]any{
		"json": testStructProperties{CustomName: "Person"},
	}
	if diff := cmp.Diff(wantAspects, person.Aspects()); diff != "" {
		t.Errorf("person.Aspects() mismatch (-want +got):\n%s", diff)
	}

	gotNameField, ok := person.FindField("display_name")
	if !ok {
		t.Fatalf("person.FindField(\"display_name\") = false, want true")
	}
	if diff := cmp.Diff(displayNameField, gotNameField, cmpOpts); diff != "" {
		t.Errorf("FindField(\"display_name\") mismatch (-want +got):\n%s", diff)
	}

	if f, ok := person.FindField("nonexistent"); ok || f != nil {
		t.Errorf("person.FindField(\"nonexistent\") = (%v, %v), want (nil, false)", f, ok)
	}

	typeDef := person.AsTypeDef()
	if got := typeDef.Kind(); got != typedef.StructKind {
		t.Errorf("typeDef.Kind() = %v, want %v", got, typedef.StructKind)
	}
	if !typeDef.IsStruct() {
		t.Errorf("typeDef.IsStruct() = false, want true")
	}
	if typeDef.IsEnum() {
		t.Errorf("typeDef.IsEnum() = true, want false")
	}
	if got := typeDef.EnumType(); got != nil {
		t.Errorf("typeDef.EnumType() = %v, want nil", got)
	}
	if got := typeDef.StructType().Name(); got != "com.example.Person" {
		t.Errorf("typeDef.StructType().Name() = %q, want \"com.example.Person\"", got)
	}
	if got := typeDef.StructType().Doc(); got != "User account profile entity." {
		t.Errorf("typeDef.StructType().Doc() = %q, want \"User account profile entity.\"", got)
	}
	if diff := cmp.Diff(wantFields, typeDef.StructType().Fields(), cmpOpts); diff != "" {
		t.Errorf("typeDef.StructType().Fields() mismatch (-want +got):\n%s", diff)
	}
}

func TestEnumTypeDefConstructionAndLookup(t *testing.T) {
	unspecifiedConst := mustEnumConstant(
		t,
		"ORDER_STATUS_UNSPECIFIED",
		0,
		typedef.EnumConstantDoc("Default unspecified state."),
	)
	shippedConst := mustEnumConstant(
		t,
		"SHIPPED",
		2,
		typedef.EnumConstantDoc("Order has been dispatched."),
		typedef.EnumConstantAspects(map[string]any{
			"json": testEnumConstantProperties{CustomName: "shipped"},
		}),
	)
	// Aliasing numeric IDs is allowed when constant names are unique.
	dispatchedConst := mustEnumConstant(
		t,
		"DISPATCHED",
		2,
		typedef.EnumConstantAspect("proto3", "alias"),
	)

	statusEnum, err := typedef.NewEnum(
		"com.example.OrderStatus",
		typedef.EnumDoc("Lifecycle state of a customer order."),
		typedef.EnumAspects(map[string]any{
			"json": testEnumProperties{AllowAlias: true},
		}),
		typedef.EnumConstants(unspecifiedConst, shippedConst, dispatchedConst),
	)
	if err != nil {
		t.Fatalf("NewEnum() unexpected error: %v", err)
	}

	wantConstants := []*typedef.EnumConstant{unspecifiedConst, shippedConst, dispatchedConst}
	if diff := cmp.Diff(wantConstants, statusEnum.Constants(), cmpOpts); diff != "" {
		t.Errorf("statusEnum.Constants() mismatch (-want +got):\n%s", diff)
	}

	wantAspects := map[string]any{
		"json": testEnumProperties{AllowAlias: true},
	}
	if diff := cmp.Diff(wantAspects, statusEnum.Aspects()); diff != "" {
		t.Errorf("statusEnum.Aspects() mismatch (-want +got):\n%s", diff)
	}

	gotShipped, ok := statusEnum.FindConstant("SHIPPED")
	if !ok {
		t.Fatalf("statusEnum.FindConstant(\"SHIPPED\") = false, want true")
	}
	if diff := cmp.Diff(shippedConst, gotShipped, cmpOpts); diff != "" {
		t.Errorf("FindConstant(\"SHIPPED\") mismatch (-want +got):\n%s", diff)
	}

	if c, ok := statusEnum.FindConstant("UNKNOWN"); ok || c != nil {
		t.Errorf("statusEnum.FindConstant(\"UNKNOWN\") = (%v, %v), want (nil, false)", c, ok)
	}

	var emptyEnum typedef.EnumTypeDef
	if c, ok := emptyEnum.FindConstant("ORDER_STATUS_UNSPECIFIED"); ok || c != nil {
		t.Errorf("emptyEnum.FindConstant() = (%v, %v), want (nil, false)", c, ok)
	}

	typeDef := statusEnum.AsTypeDef()
	if got := typeDef.Kind(); got != typedef.EnumKind {
		t.Errorf("typeDef.Kind() = %v, want %v", got, typedef.EnumKind)
	}
	if typeDef.IsStruct() {
		t.Errorf("typeDef.IsStruct() = true, want false")
	}
	if !typeDef.IsEnum() {
		t.Errorf("typeDef.IsEnum() = false, want true")
	}
	if got := typeDef.StructType(); got != nil {
		t.Errorf("typeDef.StructType() = %v, want nil", got)
	}
	if got := typeDef.EnumType().Name(); got != "com.example.OrderStatus" {
		t.Errorf("typeDef.EnumType().Name() = %q, want \"com.example.OrderStatus\"", got)
	}
	if got := typeDef.EnumType().Doc(); got != "Lifecycle state of a customer order." {
		t.Errorf("typeDef.EnumType().Doc() = %q, want \"Lifecycle state of a customer order.\"", got)
	}
	if diff := cmp.Diff(wantConstants, typeDef.EnumType().Constants(), cmpOpts); diff != "" {
		t.Errorf("typeDef.EnumType().Constants() mismatch (-want +got):\n%s", diff)
	}
}

func TestFieldDefaultValues(t *testing.T) {
	validCases := []struct {
		name    string
		val     ref.Val
		wantHas bool
	}{
		{name: "unset_nil", val: nil, wantHas: false},
		{name: "null", val: types.NullValue, wantHas: true},
		{name: "bool", val: types.Bool(true), wantHas: true},
		{name: "int", val: types.Int(-42), wantHas: true},
		{name: "uint", val: types.Uint(100), wantHas: true},
		{name: "double", val: types.Double(3.25), wantHas: true},
		{name: "double_nan", val: types.Double(math.NaN()), wantHas: true},
		{name: "bytes", val: types.Bytes([]byte{0xff, 0x00, 0x01}), wantHas: true},
		{name: "bytes_nil", val: types.Bytes(nil), wantHas: true},
		{name: "bytes_empty", val: types.Bytes{}, wantHas: true},
		{name: "string", val: types.String("hello"), wantHas: true},
		{name: "duration", val: types.Duration{Duration: 90 * time.Second}, wantHas: true},
		{name: "timestamp", val: types.Timestamp{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}, wantHas: true},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := typedef.NewField("f", types.DynType, typedef.FieldDefaultValue(tc.val))
			if err != nil {
				t.Fatalf("NewField() unexpected error: %v", err)
			}
			if got := f.HasDefaultValue(); got != tc.wantHas {
				t.Errorf("f.HasDefaultValue() = %v, want %v", got, tc.wantHas)
			}
			if diff := cmp.Diff(tc.val, f.DefaultValue(), cmpOpts); diff != "" {
				t.Errorf("f.DefaultValue() mismatch (-want +got):\n%s", diff)
			}
		})
	}

	var typedNilErr *types.Err
	var typedNilDuration *types.Duration
	var typedNilTimestamp *types.Timestamp
	invalidCases := []struct {
		name string
		val  ref.Val
	}{
		{name: "list", val: types.DefaultTypeAdapter.NativeToValue([]string{"a"})},
		{name: "map", val: types.DefaultTypeAdapter.NativeToValue(map[string]int{"a": 1})},
		{name: "type_val", val: types.IntType},
		{name: "err_val", val: types.NewErr("boom")},
		{name: "unknown_val", val: types.NewUnknown(1, nil)},
		{name: "optional_none", val: types.OptionalNone},
		{name: "typed_nil_err", val: typedNilErr},
		{name: "typed_nil_duration_ptr", val: typedNilDuration},
		{name: "typed_nil_timestamp_ptr", val: typedNilTimestamp},
	}

	for _, tc := range invalidCases {
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			_, err := typedef.NewField("f", types.DynType, typedef.FieldDefaultValue(tc.val))
			if !errors.Is(err, typedef.ErrBadTypeDef) {
				t.Errorf("NewField(FieldDefaultValue(%T)) error = %v, want ErrBadTypeDef", tc.val, err)
			}
		})
	}
}

func TestNewFieldEmptyName(t *testing.T) {
	if _, err := typedef.NewField("", types.StringType); !errors.Is(err, typedef.ErrBadTypeDef) {
		t.Errorf("NewField(\"\") error = %v, want ErrBadTypeDef", err)
	}
}

func TestAddFieldValidationErrors(t *testing.T) {
	dupStruct, err := typedef.NewStruct(
		"com.example.Dup",
		typedef.StructFields(mustField(t, "id", types.IntType)),
	)
	if err != nil {
		t.Fatalf("NewStruct() unexpected error: %v", err)
	}

	addFieldCases := []struct {
		name    string
		target  *typedef.StructTypeDef
		field   *typedef.Field
		wantErr error
	}{
		{
			name:    "nil_struct_receiver",
			target:  nil,
			field:   mustField(t, "id", types.IntType),
			wantErr: typedef.ErrBadTypeDef,
		},
		{
			name:    "nil_field",
			target:  &typedef.StructTypeDef{},
			field:   nil,
			wantErr: typedef.ErrBadTypeDef,
		},
		{
			name:    "zero_value_field_empty_name",
			target:  &typedef.StructTypeDef{},
			field:   &typedef.Field{},
			wantErr: typedef.ErrBadTypeDef,
		},
		{
			name:    "duplicate_field_name",
			target:  dupStruct,
			field:   mustField(t, "id", types.StringType),
			wantErr: typedef.ErrAlreadyExists,
		},
	}

	for _, tc := range addFieldCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.target.AddField(tc.field); !errors.Is(err, tc.wantErr) {
				t.Errorf("AddField(%v) error = %v, want %v", tc.field, err, tc.wantErr)
			}
		})
	}
}

func TestNewStructDuplicateFields(t *testing.T) {
	_, err := typedef.NewStruct(
		"com.example.Bad",
		typedef.StructFields(
			mustField(t, "dup", types.IntType),
			mustField(t, "dup", types.StringType),
		),
	)
	if !errors.Is(err, typedef.ErrAlreadyExists) {
		t.Errorf("NewStruct(duplicate fields) error = %v, want ErrAlreadyExists", err)
	}
}

func TestNewEnumConstantEmptyName(t *testing.T) {
	if _, err := typedef.NewEnumConstant("", 0); !errors.Is(err, typedef.ErrBadTypeDef) {
		t.Errorf("NewEnumConstant(\"\") error = %v, want ErrBadTypeDef", err)
	}
}

func TestNewEnumConstantOptionError(t *testing.T) {
	badOpt := func(*typedef.EnumConstant) error {
		return typedef.ErrBadTypeDef
	}
	if _, err := typedef.NewEnumConstant("OK", 0, badOpt); !errors.Is(err, typedef.ErrBadTypeDef) {
		t.Errorf("NewEnumConstant(badOpt) error = %v, want ErrBadTypeDef", err)
	}
}

func TestAddConstantValidationErrors(t *testing.T) {
	dupEnum, err := typedef.NewEnum(
		"com.example.DupEnum",
		typedef.EnumConstants(mustEnumConstant(t, "A", 0)),
	)
	if err != nil {
		t.Fatalf("NewEnum() unexpected error: %v", err)
	}

	addConstCases := []struct {
		name     string
		target   *typedef.EnumTypeDef
		constant *typedef.EnumConstant
		wantErr  error
	}{
		{
			name:     "nil_enum_receiver",
			target:   nil,
			constant: mustEnumConstant(t, "A", 0),
			wantErr:  typedef.ErrBadTypeDef,
		},
		{
			name:     "nil_constant",
			target:   &typedef.EnumTypeDef{},
			constant: nil,
			wantErr:  typedef.ErrBadTypeDef,
		},
		{
			name:     "zero_value_constant_empty_name",
			target:   &typedef.EnumTypeDef{},
			constant: &typedef.EnumConstant{},
			wantErr:  typedef.ErrBadTypeDef,
		},
		{
			name:     "duplicate_constant_name",
			target:   dupEnum,
			constant: mustEnumConstant(t, "A", 1),
			wantErr:  typedef.ErrAlreadyExists,
		},
	}

	for _, tc := range addConstCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.target.AddConstant(tc.constant); !errors.Is(err, tc.wantErr) {
				t.Errorf("AddConstant(%v) error = %v, want %v", tc.constant, err, tc.wantErr)
			}
		})
	}
}

func TestNewEnumDuplicateConstants(t *testing.T) {
	_, err := typedef.NewEnum(
		"com.example.BadEnum",
		typedef.EnumConstants(
			mustEnumConstant(t, "DUP", 0),
			mustEnumConstant(t, "DUP", 1),
		),
	)
	if !errors.Is(err, typedef.ErrAlreadyExists) {
		t.Errorf("NewEnum(duplicate constants) error = %v, want ErrAlreadyExists", err)
	}
}

func TestStructAspectsAndDefensiveCopies(t *testing.T) {
	in := map[string]any{"k1": "v1", "override": "old"}
	s, err := typedef.NewStruct(
		"com.example.Safe",
		nil,
		typedef.StructAspects(nil),
		typedef.StructAspects(in),
		typedef.StructAspect("override", "new"),
		typedef.StructFields(mustField(t, "f1", types.BoolType)),
	)
	if err != nil {
		t.Fatalf("NewStruct() unexpected error: %v", err)
	}
	in["injected"] = true
	out := s.Aspects()
	out["mutated"] = true
	want := map[string]any{"k1": "v1", "override": "new"}
	if diff := cmp.Diff(want, s.Aspects()); diff != "" {
		t.Errorf("s.Aspects() mismatch (-want +got):\n%s", diff)
	}
	if got, ok := s.Aspect("override"); !ok || got != "new" {
		t.Errorf("s.Aspect(\"override\") = (%v, %v), want (\"new\", true)", got, ok)
	}
	if got, ok := s.Aspect("missing"); ok || got != nil {
		t.Errorf("s.Aspect(\"missing\") = (%v, %v), want (nil, false)", got, ok)
	}

	fields := s.Fields()
	fields[0] = nil
	if f, ok := s.FindField("f1"); !ok || f == nil {
		t.Errorf("mutating returned Fields() slice affected StructTypeDef")
	}
}

func TestFieldAspectsAndBytesDefaultCopy(t *testing.T) {
	in := map[string]any{"k1": "v1", "override": "old"}
	rawBytes := []byte{0x0a, 0x0b}
	f := mustField(
		t,
		"payload",
		types.BytesType,
		nil,
		typedef.FieldAspects(nil),
		typedef.FieldAspects(in),
		typedef.FieldAspect("override", "new"),
		typedef.FieldDefaultValue(types.Bytes(rawBytes)),
	)
	in["injected"] = true
	rawBytes[0] = 0xff

	out := f.Aspects()
	out["mutated"] = true
	want := map[string]any{"k1": "v1", "override": "new"}
	if diff := cmp.Diff(want, f.Aspects()); diff != "" {
		t.Errorf("f.Aspects() mismatch (-want +got):\n%s", diff)
	}
	if got, ok := f.Aspect("override"); !ok || got != "new" {
		t.Errorf("f.Aspect(\"override\") = (%v, %v), want (\"new\", true)", got, ok)
	}
	if got, ok := f.Aspect("missing"); ok || got != nil {
		t.Errorf("f.Aspect(\"missing\") = (%v, %v), want (nil, false)", got, ok)
	}

	gotBytes := f.DefaultValue().(types.Bytes)
	gotBytes[1] = 0xee
	if diff := cmp.Diff(types.Bytes([]byte{0x0a, 0x0b}), f.DefaultValue(), cmpOpts); diff != "" {
		t.Errorf("f.DefaultValue() bytes defensive copy failed (-want +got):\n%s", diff)
	}
}

func TestEnumAndConstantAspectsAndDefensiveCopies(t *testing.T) {
	constIn := map[string]any{"k1": "v1", "override": "old"}
	c := mustEnumConstant(
		t,
		"ZERO",
		0,
		nil,
		typedef.EnumConstantAspects(nil),
		typedef.EnumConstantAspects(constIn),
		typedef.EnumConstantAspect("override", "new"),
	)
	constIn["injected"] = true
	cOut := c.Aspects()
	cOut["mutated"] = true
	want := map[string]any{"k1": "v1", "override": "new"}
	if diff := cmp.Diff(want, c.Aspects()); diff != "" {
		t.Errorf("c.Aspects() mismatch (-want +got):\n%s", diff)
	}
	if got, ok := c.Aspect("override"); !ok || got != "new" {
		t.Errorf("c.Aspect(\"override\") = (%v, %v), want (\"new\", true)", got, ok)
	}
	if got, ok := c.Aspect("missing"); ok || got != nil {
		t.Errorf("c.Aspect(\"missing\") = (%v, %v), want (nil, false)", got, ok)
	}

	enumIn := map[string]any{"k1": "v1", "override": "old"}
	e, err := typedef.NewEnum(
		"com.example.SafeEnum",
		nil,
		typedef.EnumAspects(nil),
		typedef.EnumAspects(enumIn),
		typedef.EnumAspect("override", "new"),
		typedef.EnumConstants(c),
	)
	if err != nil {
		t.Fatalf("NewEnum() unexpected error: %v", err)
	}
	enumIn["injected"] = true
	eOut := e.Aspects()
	eOut["mutated"] = true
	if diff := cmp.Diff(want, e.Aspects()); diff != "" {
		t.Errorf("e.Aspects() mismatch (-want +got):\n%s", diff)
	}
	if got, ok := e.Aspect("override"); !ok || got != "new" {
		t.Errorf("e.Aspect(\"override\") = (%v, %v), want (\"new\", true)", got, ok)
	}
	if got, ok := e.Aspect("missing"); ok || got != nil {
		t.Errorf("e.Aspect(\"missing\") = (%v, %v), want (nil, false)", got, ok)
	}

	consts := e.Constants()
	consts[0] = nil
	if found, ok := e.FindConstant("ZERO"); !ok || found == nil {
		t.Errorf("mutating returned Constants() slice affected EnumTypeDef")
	}
}

func TestNilTypeDef(t *testing.T) {
	var td *typedef.TypeDef
	if got := td.Kind(); got != typedef.UnspecifiedKind {
		t.Errorf("nil TypeDef.Kind() = %v, want %v", got, typedef.UnspecifiedKind)
	}
	if td.IsStruct() {
		t.Errorf("nil TypeDef.IsStruct() = true, want false")
	}
	if td.IsEnum() {
		t.Errorf("nil TypeDef.IsEnum() = true, want false")
	}
	if got := td.StructType(); got != nil {
		t.Errorf("nil TypeDef.StructType() = %v, want nil", got)
	}
	if got := td.EnumType(); got != nil {
		t.Errorf("nil TypeDef.EnumType() = %v, want nil", got)
	}
}

func TestNilStructTypeDef(t *testing.T) {
	var s *typedef.StructTypeDef
	if got := s.Name(); got != "" {
		t.Errorf("nil StructTypeDef.Name() = %q, want \"\"", got)
	}
	if got := s.Doc(); got != "" {
		t.Errorf("nil StructTypeDef.Doc() = %q, want \"\"", got)
	}
	if got := s.Fields(); got != nil {
		t.Errorf("nil StructTypeDef.Fields() = %v, want nil", got)
	}
	if got := s.Aspects(); got != nil {
		t.Errorf("nil StructTypeDef.Aspects() = %v, want nil", got)
	}
	if got, ok := s.Aspect("x"); ok || got != nil {
		t.Errorf("nil StructTypeDef.Aspect(\"x\") = (%v, %v), want (nil, false)", got, ok)
	}
	if got, ok := s.FindField("x"); ok || got != nil {
		t.Errorf("nil StructTypeDef.FindField(\"x\") = (%v, %v), want (nil, false)", got, ok)
	}
	if got := s.AsTypeDef(); got != nil {
		t.Errorf("nil StructTypeDef.AsTypeDef() = %v, want nil", got)
	}
}

func TestNilField(t *testing.T) {
	var f *typedef.Field
	if got := f.Name(); got != "" {
		t.Errorf("nil Field.Name() = %q, want \"\"", got)
	}
	if got := f.Type(); got == nil || got.Kind() != types.UnspecifiedKind {
		t.Errorf("nil Field.Type() = %v, want non-nil UnspecifiedKind", got)
	}
	if got := f.Doc(); got != "" {
		t.Errorf("nil Field.Doc() = %q, want \"\"", got)
	}
	if got := f.DefaultValue(); got != nil {
		t.Errorf("nil Field.DefaultValue() = %v, want nil", got)
	}
	if f.HasDefaultValue() {
		t.Errorf("nil Field.HasDefaultValue() = true, want false")
	}
	if got := f.Aspects(); got != nil {
		t.Errorf("nil Field.Aspects() = %v, want nil", got)
	}
	if got, ok := f.Aspect("x"); ok || got != nil {
		t.Errorf("nil Field.Aspect(\"x\") = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestNilEnumTypeDef(t *testing.T) {
	var e *typedef.EnumTypeDef
	if got := e.Name(); got != "" {
		t.Errorf("nil EnumTypeDef.Name() = %q, want \"\"", got)
	}
	if got := e.Doc(); got != "" {
		t.Errorf("nil EnumTypeDef.Doc() = %q, want \"\"", got)
	}
	if got := e.Constants(); got != nil {
		t.Errorf("nil EnumTypeDef.Constants() = %v, want nil", got)
	}
	if got := e.Aspects(); got != nil {
		t.Errorf("nil EnumTypeDef.Aspects() = %v, want nil", got)
	}
	if got, ok := e.Aspect("x"); ok || got != nil {
		t.Errorf("nil EnumTypeDef.Aspect(\"x\") = (%v, %v), want (nil, false)", got, ok)
	}
	if got, ok := e.FindConstant("x"); ok || got != nil {
		t.Errorf("nil EnumTypeDef.FindConstant(\"x\") = (%v, %v), want (nil, false)", got, ok)
	}
	if got := e.AsTypeDef(); got != nil {
		t.Errorf("nil EnumTypeDef.AsTypeDef() = %v, want nil", got)
	}
}

func TestNilEnumConstant(t *testing.T) {
	var c *typedef.EnumConstant
	if got := c.Name(); got != "" {
		t.Errorf("nil EnumConstant.Name() = %q, want \"\"", got)
	}
	if got := c.ID(); got != 0 {
		t.Errorf("nil EnumConstant.ID() = %d, want 0", got)
	}
	if got := c.Doc(); got != "" {
		t.Errorf("nil EnumConstant.Doc() = %q, want \"\"", got)
	}
	if got := c.Aspects(); got != nil {
		t.Errorf("nil EnumConstant.Aspects() = %v, want nil", got)
	}
	if got, ok := c.Aspect("x"); ok || got != nil {
		t.Errorf("nil EnumConstant.Aspect(\"x\") = (%v, %v), want (nil, false)", got, ok)
	}
}
