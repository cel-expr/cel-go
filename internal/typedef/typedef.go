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

// Package typedef defines declarative CEL type definitions (structs, enums, and
// type definition sets) and aspect property containers.
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
	// ErrBadTypeDef indicates an invalid type, field, enum constant, or aspect
	// definition (e.g., nil receiver, empty name, or non-scalar constant).
	ErrBadTypeDef = errors.New("invalid typedef")

	// ErrAlreadyExists indicates a duplicate type, field, or enum constant
	// definition.
	ErrAlreadyExists = errors.New("already exists")

	defaultFieldType = &types.Type{}
	emptyAspect      = &Aspect{}
)

// Aspect represents a read-only set of schema-specific or extension properties
// attached to a type definition, field, or enum constant.
type Aspect struct {
	properties map[string]ref.Val
}

// NewAspect creates a new Aspect with a defensive copy of properties.
//
// Supported property values are non-nil CEL scalar constants (types.Null,
// types.Bool, types.Int, types.Uint, types.Double, types.Bytes, types.String,
// types.Duration, and types.Timestamp); nil or any other ref.Val returns
// ErrBadTypeDef.
func NewAspect(properties map[string]ref.Val) (*Aspect, error) {
	if len(properties) == 0 {
		return &Aspect{}, nil
	}
	cloned := make(map[string]ref.Val, len(properties))
	for k, v := range properties {
		if v == nil {
			return nil, fmt.Errorf("aspect property %q: %w: constant value cannot be nil", k, ErrBadTypeDef)
		}
		sanitized, err := sanitizeConstant(v)
		if err != nil {
			return nil, fmt.Errorf("aspect property %q: %w", k, err)
		}
		cloned[k] = sanitized
	}
	return &Aspect{properties: cloned}, nil
}

// Properties returns a defensive copy of the aspect's properties, or nil if
// the aspect is nil or has no properties.
func (a *Aspect) Properties() map[string]ref.Val {
	if a == nil || len(a.properties) == 0 {
		return nil
	}
	cloned := make(map[string]ref.Val, len(a.properties))
	for k, v := range a.properties {
		cloned[k] = cloneConstant(v)
	}
	return cloned
}

// Property looks up a property by key, returning the property value and
// true if the key is present, or (nil, false) otherwise.
func (a *Aspect) Property(key string) (ref.Val, bool) {
	if a == nil || len(a.properties) == 0 {
		return nil, false
	}
	v, ok := a.properties[key]
	if !ok {
		return nil, false
	}
	return cloneConstant(v), true
}

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

// Name returns the underlying struct or enum type's name, or "" if t is nil
// or unspecified.
func (t *TypeDef) Name() string {
	switch t.Kind() {
	case StructKind:
		return t.structType.Name()
	case EnumKind:
		return t.enumType.Name()
	default:
		return ""
	}
}

// Doc returns the underlying struct or enum type's documentation string, or ""
// if t is nil or unspecified.
func (t *TypeDef) Doc() string {
	switch t.Kind() {
	case StructKind:
		return t.structType.Doc()
	case EnumKind:
		return t.enumType.Doc()
	default:
		return ""
	}
}

// Aspects returns a shallow copy of the underlying struct or enum type's
// aspects keyed by aspect name, or nil if t is nil, unspecified, or has no
// aspects.
func (t *TypeDef) Aspects() map[string]*Aspect {
	switch t.Kind() {
	case StructKind:
		return t.structType.Aspects()
	case EnumKind:
		return t.enumType.Aspects()
	default:
		return nil
	}
}

// Aspect looks up a single aspect on the underlying struct or enum type by
// aspect name.
func (t *TypeDef) Aspect(name string) (*Aspect, bool) {
	switch t.Kind() {
	case StructKind:
		return t.structType.Aspect(name)
	case EnumKind:
		return t.enumType.Aspect(name)
	default:
		return nil, false
	}
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

// TypeDefSet represents a collection of unique TypeDefs (structs and enums)
// indexed by their fully-qualified CEL type name.
type TypeDefSet struct {
	types       []*TypeDef
	typesByName map[string]*TypeDef
}

// NewTypeDefSet creates a new TypeDefSet containing typeDefs in the given
// order. Returns an error if any TypeDef is invalid or has a duplicate name.
func NewTypeDefSet(typeDefs ...*TypeDef) (*TypeDefSet, error) {
	s := &TypeDefSet{}
	for _, td := range typeDefs {
		if err := s.AddTypeDef(td); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// AddTypeDef adds a TypeDef to the set.
//
// Returns ErrBadTypeDef if s is nil, typeDef is nil or unspecified, or the
// type name is empty, and ErrAlreadyExists if a TypeDef with the same name is
// already present.
func (s *TypeDefSet) AddTypeDef(typeDef *TypeDef) error {
	if s == nil {
		return fmt.Errorf("%w: cannot add type to nil TypeDefSet", ErrBadTypeDef)
	}
	if typeDef == nil || typeDef.Kind() == UnspecifiedKind {
		return fmt.Errorf("%w: type definition cannot be nil or unspecified", ErrBadTypeDef)
	}
	typeName := typeDef.Name()
	if typeName == "" {
		return fmt.Errorf("%w: type name cannot be empty", ErrBadTypeDef)
	}
	if s.typesByName == nil {
		s.typesByName = make(map[string]*TypeDef)
	}
	if _, exists := s.typesByName[typeName]; exists {
		return fmt.Errorf("%w: type %q", ErrAlreadyExists, typeName)
	}
	s.typesByName[typeName] = typeDef
	s.types = append(s.types, typeDef)
	return nil
}

// FindTypeDef looks up a TypeDef by its fully-qualified name, returning the
// TypeDef and true if found, or (nil, false) otherwise.
func (s *TypeDefSet) FindTypeDef(typeName string) (*TypeDef, bool) {
	if s == nil || len(s.typesByName) == 0 {
		return nil, false
	}
	td, ok := s.typesByName[typeName]
	return td, ok
}

// Types returns the TypeDefs in the order they were added, or nil if s is nil
// or empty.
func (s *TypeDefSet) Types() []*TypeDef {
	if s == nil || len(s.types) == 0 {
		return nil
	}
	return slices.Clone(s.types)
}

// StructTypeDef represents a declarative struct type with typed fields and
// optional aspects.
type StructTypeDef struct {
	name         string
	doc          string
	fields       []*Field
	fieldsByName map[string]*Field
	aspects      map[string]*Aspect
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

// Fields returns the fields in the order they were added, or nil if s is nil
// or empty.
func (s *StructTypeDef) Fields() []*Field {
	if s == nil || len(s.fields) == 0 {
		return nil
	}
	return slices.Clone(s.fields)
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (s *StructTypeDef) Aspects() map[string]*Aspect {
	if s == nil || len(s.aspects) == 0 {
		return nil
	}
	return maps.Clone(s.aspects)
}

// Aspect looks up a single aspect by aspect name.
func (s *StructTypeDef) Aspect(name string) (*Aspect, bool) {
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
	fieldName := field.Name()
	if fieldName == "" {
		return fmt.Errorf("%w: field name cannot be empty", ErrBadTypeDef)
	}
	if s.fieldsByName == nil {
		s.fieldsByName = make(map[string]*Field)
	}
	if _, exists := s.fieldsByName[fieldName]; exists {
		return fmt.Errorf("%w: field %q", ErrAlreadyExists, fieldName)
	}
	s.fieldsByName[fieldName] = field
	s.fields = append(s.fields, field)
	return nil
}

// FindField looks up a field by name, returning the Field and true if found,
// or (nil, false) otherwise.
func (s *StructTypeDef) FindField(fieldName string) (*Field, bool) {
	if s == nil || len(s.fieldsByName) == 0 {
		return nil, false
	}
	f, ok := s.fieldsByName[fieldName]
	return f, ok
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

// StructAspects merges a map of named aspects into a StructTypeDef.
func StructAspects(aspects map[string]*Aspect) StructOpt {
	return func(s *StructTypeDef) error {
		s.aspects = mergeAspects(s.aspects, aspects)
		return nil
	}
}

// StructAspect sets a single named aspect on a StructTypeDef.
func StructAspect(name string, aspect *Aspect) StructOpt {
	return func(s *StructTypeDef) error {
		s.aspects = setAspect(s.aspects, name, aspect)
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
	aspects      map[string]*Aspect
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
	return cloneConstant(f.defaultValue)
}

// HasDefaultValue reports whether a default value is set on the field.
func (f *Field) HasDefaultValue() bool {
	return f != nil && f.defaultValue != nil
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (f *Field) Aspects() map[string]*Aspect {
	if f == nil || len(f.aspects) == 0 {
		return nil
	}
	return maps.Clone(f.aspects)
}

// Aspect looks up a single aspect by aspect name.
func (f *Field) Aspect(name string) (*Aspect, bool) {
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
		sanitized, err := sanitizeConstant(val)
		if err != nil {
			return fmt.Errorf("field default value: %w", err)
		}
		f.defaultValue = sanitized
		return nil
	}
}

// FieldAspects merges a map of named aspects into a Field.
func FieldAspects(aspects map[string]*Aspect) FieldOpt {
	return func(f *Field) error {
		f.aspects = mergeAspects(f.aspects, aspects)
		return nil
	}
}

// FieldAspect sets a single named aspect on a Field.
func FieldAspect(name string, aspect *Aspect) FieldOpt {
	return func(f *Field) error {
		f.aspects = setAspect(f.aspects, name, aspect)
		return nil
	}
}

// EnumTypeDef represents a named enumeration type with named and numbered
// constants and optional aspects.
type EnumTypeDef struct {
	name            string
	doc             string
	constants       []*EnumConstant
	constantsByName map[string]*EnumConstant
	aspects         map[string]*Aspect
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

// Constants returns the constants in the order they were added, or nil if e is
// nil or empty.
func (e *EnumTypeDef) Constants() []*EnumConstant {
	if e == nil || len(e.constants) == 0 {
		return nil
	}
	return slices.Clone(e.constants)
}

// Aspects returns a shallow copy of the aspects (e.g. schema-specific
// properties) keyed by aspect name.
func (e *EnumTypeDef) Aspects() map[string]*Aspect {
	if e == nil || len(e.aspects) == 0 {
		return nil
	}
	return maps.Clone(e.aspects)
}

// Aspect looks up a single aspect by aspect name.
func (e *EnumTypeDef) Aspect(name string) (*Aspect, bool) {
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
	constantName := constant.Name()
	if constantName == "" {
		return fmt.Errorf("%w: enum constant name cannot be empty", ErrBadTypeDef)
	}
	if e.constantsByName == nil {
		e.constantsByName = make(map[string]*EnumConstant)
	}
	if _, exists := e.constantsByName[constantName]; exists {
		return fmt.Errorf("%w: enum constant %q", ErrAlreadyExists, constantName)
	}
	e.constantsByName[constantName] = constant
	e.constants = append(e.constants, constant)
	return nil
}

// FindConstant looks up an enum constant by name, returning the EnumConstant
// and true if found, or (nil, false) otherwise.
func (e *EnumTypeDef) FindConstant(constantName string) (*EnumConstant, bool) {
	if e == nil || len(e.constantsByName) == 0 {
		return nil, false
	}
	c, ok := e.constantsByName[constantName]
	return c, ok
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

// EnumAspects merges a map of named aspects into an EnumTypeDef.
func EnumAspects(aspects map[string]*Aspect) EnumOpt {
	return func(e *EnumTypeDef) error {
		e.aspects = mergeAspects(e.aspects, aspects)
		return nil
	}
}

// EnumAspect sets a single named aspect on an EnumTypeDef.
func EnumAspect(name string, aspect *Aspect) EnumOpt {
	return func(e *EnumTypeDef) error {
		e.aspects = setAspect(e.aspects, name, aspect)
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
	aspects map[string]*Aspect
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
func (c *EnumConstant) Aspects() map[string]*Aspect {
	if c == nil || len(c.aspects) == 0 {
		return nil
	}
	return maps.Clone(c.aspects)
}

// Aspect looks up a single aspect by aspect name.
func (c *EnumConstant) Aspect(name string) (*Aspect, bool) {
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

// EnumConstantAspects merges a map of named aspects into an EnumConstant.
func EnumConstantAspects(aspects map[string]*Aspect) EnumConstantOpt {
	return func(c *EnumConstant) error {
		c.aspects = mergeAspects(c.aspects, aspects)
		return nil
	}
}

// EnumConstantAspect sets a single named aspect on an EnumConstant.
func EnumConstantAspect(name string, aspect *Aspect) EnumConstantOpt {
	return func(c *EnumConstant) error {
		c.aspects = setAspect(c.aspects, name, aspect)
		return nil
	}
}

func sanitizeConstant(val ref.Val) (ref.Val, error) {
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
		return nil, fmt.Errorf("%w: unsupported constant value %T", ErrBadTypeDef, val)
	}
}

func cloneConstant(val ref.Val) ref.Val {
	if b, ok := val.(types.Bytes); ok {
		return slices.Clone(b)
	}
	return val
}

func mergeAspects(dst, src map[string]*Aspect) map[string]*Aspect {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]*Aspect, len(src))
	}
	for name, aspect := range src {
		if aspect == nil {
			aspect = emptyAspect
		}
		dst[name] = aspect
	}
	return dst
}

func setAspect(dst map[string]*Aspect, name string, aspect *Aspect) map[string]*Aspect {
	if dst == nil {
		dst = make(map[string]*Aspect, 1)
	}
	if aspect == nil {
		aspect = emptyAspect
	}
	dst[name] = aspect
	return dst
}
