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

// Package typedef defines declarative CEL type definitions (structs and enums)
// and aspect property sets.
package typedef

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

var (
	// ErrBadTypeDef indicates an invalid type, field, or enum constant
	// definition (e.g., nil receiver, empty name, or non-scalar default).
	ErrBadTypeDef = errors.New("invalid typedef")

	// ErrAlreadyExists indicates a duplicate field or enum constant
	// definition.
	ErrAlreadyExists = errors.New("already exists")

	defaultFieldType = &types.Type{}
)

// Kind indicates whether a TypeDef holds a struct or an enum definition.
type Kind int

const (
	// UnspecifiedKind is returned when a TypeDef is nil or uninitialized.
	UnspecifiedKind Kind = iota
	// StructKind indicates a struct type definition.
	StructKind
	// EnumKind indicates an enum type definition.
	EnumKind
)

// String implements fmt.Stringer.
func (k Kind) String() string {
	switch k {
	case StructKind:
		return "struct"
	case EnumKind:
		return "enum"
	default:
		return "unspecified"
	}
}

// TypeDef is a variant wrapper holding either a StructTypeDef or an
// EnumTypeDef.
//
// Construct instances with FromStruct or FromEnum (or AsTypeDef on a
// StructTypeDef or EnumTypeDef); a zero-value or nil TypeDef has Kind() ==
// UnspecifiedKind.
type TypeDef struct {
	kind       Kind
	structType *StructTypeDef
	enumType   *EnumTypeDef
}

// FromStruct creates a TypeDef holding structType. If structType is nil, an
// empty StructTypeDef is allocated.
func FromStruct(structType *StructTypeDef) *TypeDef {
	if structType == nil {
		structType = &StructTypeDef{}
	}
	return &TypeDef{
		kind:       StructKind,
		structType: structType,
	}
}

// FromEnum creates a TypeDef holding enumType. If enumType is nil, an empty
// EnumTypeDef is allocated.
func FromEnum(enumType *EnumTypeDef) *TypeDef {
	if enumType == nil {
		enumType = &EnumTypeDef{}
	}
	return &TypeDef{
		kind:     EnumKind,
		enumType: enumType,
	}
}

// Kind returns the Kind of the definition (UnspecifiedKind, StructKind, or
// EnumKind).
func (t *TypeDef) Kind() Kind {
	if t == nil {
		return UnspecifiedKind
	}
	return t.kind
}

// IsStruct reports whether the TypeDef holds a StructTypeDef.
func (t *TypeDef) IsStruct() bool {
	return t.Kind() == StructKind
}

// StructType returns the underlying StructTypeDef if IsStruct() is true, or
// nil otherwise.
func (t *TypeDef) StructType() *StructTypeDef {
	if !t.IsStruct() {
		return nil
	}
	return t.structType
}

// IsEnum reports whether the TypeDef holds an EnumTypeDef.
func (t *TypeDef) IsEnum() bool {
	return t.Kind() == EnumKind
}

// EnumType returns the underlying EnumTypeDef if IsEnum() is true, or nil
// otherwise.
func (t *TypeDef) EnumType() *EnumTypeDef {
	if !t.IsEnum() {
		return nil
	}
	return t.enumType
}

// StructTypeDef represents a declarative struct type with typed fields and
// optional aspects.
type StructTypeDef struct {
	name    string
	doc     string
	fields  []*Field
	aspects map[string]any
}

// NewStruct creates a new StructTypeDef with the given name and options.
func NewStruct(name string, opts ...StructOpt) (*StructTypeDef, error) {
	s := &StructTypeDef{
		name: name,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Name returns the struct type's name.
func (s *StructTypeDef) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Doc returns the struct type's documentation string.
func (s *StructTypeDef) Doc() string {
	if s == nil {
		return ""
	}
	return s.doc
}

// Fields returns the struct's fields in declaration order.
func (s *StructTypeDef) Fields() []*Field {
	if s == nil || len(s.fields) == 0 {
		return nil
	}
	return slices.Clone(s.fields)
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (s *StructTypeDef) Aspects() map[string]any {
	if s == nil || len(s.aspects) == 0 {
		return nil
	}
	return maps.Clone(s.aspects)
}

// Aspect looks up a single aspect property set by aspect name.
func (s *StructTypeDef) Aspect(name string) (any, bool) {
	if s == nil || len(s.aspects) == 0 {
		return nil, false
	}
	v, ok := s.aspects[name]
	return v, ok
}

// AddField adds a field to the struct definition.
//
// Field names must be non-empty (ErrBadTypeDef) and unique within the struct
// (ErrAlreadyExists).
func (s *StructTypeDef) AddField(field *Field) error {
	if s == nil {
		return fmt.Errorf("%w: cannot add field to nil StructTypeDef", ErrBadTypeDef)
	}
	if field == nil || field.Name() == "" {
		return fmt.Errorf("%w: field name cannot be empty", ErrBadTypeDef)
	}
	if _, found := s.FindField(field.Name()); found {
		return fmt.Errorf("%w: field %q", ErrAlreadyExists, field.Name())
	}
	s.fields = append(s.fields, field)
	return nil
}

// FindField looks up a field by name, returning the Field and true if found,
// or (nil, false) otherwise.
func (s *StructTypeDef) FindField(fieldName string) (*Field, bool) {
	if s == nil {
		return nil, false
	}
	for _, f := range s.fields {
		if f.Name() == fieldName {
			return f, true
		}
	}
	return nil, false
}

// AsTypeDef wraps the StructTypeDef in a TypeDef, or returns nil if s is nil.
func (s *StructTypeDef) AsTypeDef() *TypeDef {
	if s == nil {
		return nil
	}
	return FromStruct(s)
}

// StructOpt configures a StructTypeDef during construction.
type StructOpt func(*StructTypeDef) error

// StructDoc sets the documentation string on a StructTypeDef.
func StructDoc(doc string) StructOpt {
	return func(s *StructTypeDef) error {
		s.doc = doc
		return nil
	}
}

// StructAspects merges a map of named aspect property sets into a
// StructTypeDef.
func StructAspects(aspects map[string]any) StructOpt {
	return func(s *StructTypeDef) error {
		s.aspects = mergeAspects(s.aspects, aspects)
		return nil
	}
}

// StructAspect sets a single named aspect property set on a StructTypeDef.
func StructAspect(name string, value any) StructOpt {
	return func(s *StructTypeDef) error {
		s.aspects = setAspect(s.aspects, name, value)
		return nil
	}
}

// StructFields adds one or more fields to a StructTypeDef in order.
func StructFields(fields ...*Field) StructOpt {
	return func(s *StructTypeDef) error {
		for _, f := range fields {
			if err := s.AddField(f); err != nil {
				return err
			}
		}
		return nil
	}
}

// Field represents a typed field within a StructTypeDef.
type Field struct {
	name         string
	fieldType    *types.Type
	doc          string
	defaultValue ref.Val
	aspects      map[string]any
}

// NewField creates a new Field with the given non-empty name, CEL type, and
// optional configuration. If fieldType is nil, it defaults to a zero-value
// *types.Type (types.UnspecifiedKind). Returns ErrBadTypeDef if name is empty
// or an option fails validation.
func NewField(name string, fieldType *types.Type, opts ...FieldOpt) (*Field, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: field name cannot be empty", ErrBadTypeDef)
	}
	if fieldType == nil {
		fieldType = defaultFieldType
	}
	f := &Field{
		name:      name,
		fieldType: fieldType,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(f); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// Name returns the field's name.
func (f *Field) Name() string {
	if f == nil {
		return ""
	}
	return f.name
}

// Type returns the field's CEL type, or a zero-value *types.Type
// (types.UnspecifiedKind) if f is nil or uninitialized.
func (f *Field) Type() *types.Type {
	if f == nil || f.fieldType == nil {
		return defaultFieldType
	}
	return f.fieldType
}

// Doc returns the field's documentation string.
func (f *Field) Doc() string {
	if f == nil {
		return ""
	}
	return f.doc
}

// DefaultValue returns the field's optional scalar default value, or nil if
// unset.
//
// When the field type is an enum type, DefaultValue holds a types.String with
// the name of the corresponding EnumConstant.
func (f *Field) DefaultValue() ref.Val {
	if f == nil || f.defaultValue == nil {
		return nil
	}
	if b, ok := f.defaultValue.(types.Bytes); ok {
		return slices.Clone(b)
	}
	return f.defaultValue
}

// HasDefaultValue reports whether a default value is set on the field.
func (f *Field) HasDefaultValue() bool {
	return f != nil && f.defaultValue != nil
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (f *Field) Aspects() map[string]any {
	if f == nil || len(f.aspects) == 0 {
		return nil
	}
	return maps.Clone(f.aspects)
}

// Aspect looks up a single aspect property set by aspect name.
func (f *Field) Aspect(name string) (any, bool) {
	if f == nil || len(f.aspects) == 0 {
		return nil, false
	}
	v, ok := f.aspects[name]
	return v, ok
}

// FieldOpt configures a Field during construction.
type FieldOpt func(*Field) error

// FieldDoc sets the documentation string on a Field.
func FieldDoc(doc string) FieldOpt {
	return func(f *Field) error {
		f.doc = doc
		return nil
	}
}

// FieldDefaultValue sets the optional scalar default value on a Field.
//
// Supported non-nil values are CEL scalar constants (types.Null, types.Bool,
// types.Int, types.Uint, types.Double, types.Bytes, types.String,
// types.Duration, and types.Timestamp); any other ref.Val returns
// ErrBadTypeDef. When the field type is an enum type, val should be a
// types.String holding the name of the corresponding EnumConstant.
func FieldDefaultValue(val ref.Val) FieldOpt {
	return func(f *Field) error {
		sanitized, err := sanitizeDefaultValue(val)
		if err != nil {
			return err
		}
		f.defaultValue = sanitized
		return nil
	}
}

// FieldAspects merges a map of named aspect property sets into a Field.
func FieldAspects(aspects map[string]any) FieldOpt {
	return func(f *Field) error {
		f.aspects = mergeAspects(f.aspects, aspects)
		return nil
	}
}

// FieldAspect sets a single named aspect property set on a Field.
func FieldAspect(name string, value any) FieldOpt {
	return func(f *Field) error {
		f.aspects = setAspect(f.aspects, name, value)
		return nil
	}
}

// EnumTypeDef represents a named enumeration type with named and numbered
// constants and optional aspects.
type EnumTypeDef struct {
	name      string
	doc       string
	constants []*EnumConstant
	aspects   map[string]any
}

// NewEnum creates a new EnumTypeDef with the given name and options.
func NewEnum(name string, opts ...EnumOpt) (*EnumTypeDef, error) {
	e := &EnumTypeDef{
		name: name,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(e); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// Name returns the enum type's name.
func (e *EnumTypeDef) Name() string {
	if e == nil {
		return ""
	}
	return e.name
}

// Doc returns the enum type's documentation string.
func (e *EnumTypeDef) Doc() string {
	if e == nil {
		return ""
	}
	return e.doc
}

// Constants returns the enum's constants in declaration order.
func (e *EnumTypeDef) Constants() []*EnumConstant {
	if e == nil || len(e.constants) == 0 {
		return nil
	}
	return slices.Clone(e.constants)
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (e *EnumTypeDef) Aspects() map[string]any {
	if e == nil || len(e.aspects) == 0 {
		return nil
	}
	return maps.Clone(e.aspects)
}

// Aspect looks up a single aspect property set by aspect name.
func (e *EnumTypeDef) Aspect(name string) (any, bool) {
	if e == nil || len(e.aspects) == 0 {
		return nil, false
	}
	v, ok := e.aspects[name]
	return v, ok
}

// AddConstant adds a constant to the enum definition.
//
// Constant names must be non-empty (ErrBadTypeDef) and unique within the enum
// (ErrAlreadyExists); numeric IDs may alias.
func (e *EnumTypeDef) AddConstant(constant *EnumConstant) error {
	if e == nil {
		return fmt.Errorf("%w: cannot add constant to nil EnumTypeDef", ErrBadTypeDef)
	}
	if constant == nil || constant.Name() == "" {
		return fmt.Errorf("%w: enum constant name cannot be empty", ErrBadTypeDef)
	}
	if _, found := e.FindConstant(constant.Name()); found {
		return fmt.Errorf("%w: enum constant %q", ErrAlreadyExists, constant.Name())
	}
	e.constants = append(e.constants, constant)
	return nil
}

// FindConstant looks up an enum constant by name, returning the EnumConstant
// and true if found, or (nil, false) otherwise.
func (e *EnumTypeDef) FindConstant(constantName string) (*EnumConstant, bool) {
	if e == nil {
		return nil, false
	}
	for _, c := range e.constants {
		if c.Name() == constantName {
			return c, true
		}
	}
	return nil, false
}

// AsTypeDef wraps the EnumTypeDef in a TypeDef, or returns nil if e is nil.
func (e *EnumTypeDef) AsTypeDef() *TypeDef {
	if e == nil {
		return nil
	}
	return FromEnum(e)
}

// EnumOpt configures an EnumTypeDef during construction.
type EnumOpt func(*EnumTypeDef) error

// EnumDoc sets the documentation string on an EnumTypeDef.
func EnumDoc(doc string) EnumOpt {
	return func(e *EnumTypeDef) error {
		e.doc = doc
		return nil
	}
}

// EnumAspects merges a map of named aspect property sets into an EnumTypeDef.
func EnumAspects(aspects map[string]any) EnumOpt {
	return func(e *EnumTypeDef) error {
		e.aspects = mergeAspects(e.aspects, aspects)
		return nil
	}
}

// EnumAspect sets a single named aspect property set on an EnumTypeDef.
func EnumAspect(name string, value any) EnumOpt {
	return func(e *EnumTypeDef) error {
		e.aspects = setAspect(e.aspects, name, value)
		return nil
	}
}

// EnumConstants adds one or more constants to an EnumTypeDef in order.
func EnumConstants(constants ...*EnumConstant) EnumOpt {
	return func(e *EnumTypeDef) error {
		for _, c := range constants {
			if err := e.AddConstant(c); err != nil {
				return err
			}
		}
		return nil
	}
}

// EnumConstant represents a named and numbered constant within an EnumTypeDef.
type EnumConstant struct {
	name    string
	id      int32
	doc     string
	aspects map[string]any
}

// NewEnumConstant creates a new EnumConstant with the given non-empty name,
// numeric ID, and optional configuration. Returns ErrBadTypeDef if name is
// empty or an option fails validation.
func NewEnumConstant(name string, id int32, opts ...EnumConstantOpt) (*EnumConstant, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: enum constant name cannot be empty", ErrBadTypeDef)
	}
	c := &EnumConstant{
		name: name,
		id:   id,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Name returns the enum constant's name.
func (c *EnumConstant) Name() string {
	if c == nil {
		return ""
	}
	return c.name
}

// ID returns the enum constant's numeric identifier.
func (c *EnumConstant) ID() int32 {
	if c == nil {
		return 0
	}
	return c.id
}

// Doc returns the enum constant's documentation string.
func (c *EnumConstant) Doc() string {
	if c == nil {
		return ""
	}
	return c.doc
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (c *EnumConstant) Aspects() map[string]any {
	if c == nil || len(c.aspects) == 0 {
		return nil
	}
	return maps.Clone(c.aspects)
}

// Aspect looks up a single aspect property set by aspect name.
func (c *EnumConstant) Aspect(name string) (any, bool) {
	if c == nil || len(c.aspects) == 0 {
		return nil, false
	}
	v, ok := c.aspects[name]
	return v, ok
}

// EnumConstantOpt configures an EnumConstant during construction.
type EnumConstantOpt func(*EnumConstant) error

// EnumConstantDoc sets the documentation string on an EnumConstant.
func EnumConstantDoc(doc string) EnumConstantOpt {
	return func(c *EnumConstant) error {
		c.doc = doc
		return nil
	}
}

// EnumConstantAspects merges a map of named aspect property sets into an
// EnumConstant.
func EnumConstantAspects(aspects map[string]any) EnumConstantOpt {
	return func(c *EnumConstant) error {
		c.aspects = mergeAspects(c.aspects, aspects)
		return nil
	}
}

// EnumConstantAspect sets a single named aspect property set on an
// EnumConstant.
func EnumConstantAspect(name string, value any) EnumConstantOpt {
	return func(c *EnumConstant) error {
		c.aspects = setAspect(c.aspects, name, value)
		return nil
	}
}

func sanitizeDefaultValue(val ref.Val) (ref.Val, error) {
	if val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case types.Null, types.Bool, types.Int, types.Uint, types.Double,
		types.String, types.Duration, types.Timestamp:
		return v, nil
	case types.Bytes:
		return slices.Clone(v), nil
	default:
		return nil, fmt.Errorf("%w: unsupported field default value %T", ErrBadTypeDef, val)
	}
}

func mergeAspects(dst, src map[string]any) map[string]any {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	maps.Copy(dst, src)
	return dst
}

func setAspect(dst map[string]any, name string, value any) map[string]any {
	if dst == nil {
		dst = make(map[string]any, 1)
	}
	dst[name] = value
	return dst
}
