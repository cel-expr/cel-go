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
	"fmt"

	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/internal/typedef/typedefpb"

	celpb "cel.dev/expr"
)

// TypeDefSetFromProto converts a cel.types.TypeDefSet message to a TypeDefSet.
//
// Returns ErrBadTypeDef if any type definition is invalid, and
// ErrAlreadyExists if type names, field names, or enum constant names are
// duplicated.
func TypeDefSetFromProto(pb *typedefpb.TypeDefSet) (*TypeDefSet, error) {
	s := &TypeDefSet{}
	for i, typeDefPB := range pb.GetTypes() {
		typeDef, err := TypeDefFromProto(typeDefPB)
		if err != nil {
			return nil, fmt.Errorf("types[%d]: %w", i, err)
		}
		if err := s.AddTypeDef(typeDef); err != nil {
			return nil, fmt.Errorf("types[%d]: %w", i, err)
		}
	}
	return s, nil
}

// TypeDefSetToProto converts a TypeDefSet to a cel.types.TypeDefSet message.
// A nil TypeDefSet converts to an empty message.
func TypeDefSetToProto(s *TypeDefSet) (*typedefpb.TypeDefSet, error) {
	typeDefs := s.Types()
	pb := &typedefpb.TypeDefSet{Types: make([]*typedefpb.TypeDef, 0, len(typeDefs))}
	for i, typeDef := range typeDefs {
		typeDefPB, err := TypeDefToProto(typeDef)
		if err != nil {
			return nil, fmt.Errorf("types[%d]: %w", i, err)
		}
		pb.Types = append(pb.Types, typeDefPB)
	}
	return pb, nil
}

// TypeDefFromProto converts a cel.types.TypeDef message to a TypeDef.
//
// Returns ErrBadTypeDef if neither struct_type nor enum_type is set or the
// definition is invalid, and ErrAlreadyExists if field or enum constant names
// are duplicated. Duration constants outside the range of time.Duration and
// timestamp constants outside years 0001-9999 are rejected as invalid.
func TypeDefFromProto(pb *typedefpb.TypeDef) (*TypeDef, error) {
	switch kind := pb.GetKind().(type) {
	case *typedefpb.TypeDef_StructType:
		s, err := structFromProto(kind.StructType)
		if err != nil {
			return nil, err
		}
		return FromStruct(s), nil
	case *typedefpb.TypeDef_EnumType:
		e, err := enumFromProto(kind.EnumType)
		if err != nil {
			return nil, err
		}
		return FromEnum(e), nil
	default:
		return nil, fmt.Errorf("%w: cel.types.TypeDef must specify 'struct_type' or 'enum_type'", ErrBadTypeDef)
	}
}

// TypeDefToProto converts a TypeDef to a cel.types.TypeDef message.
//
// Returns ErrBadTypeDef if t is nil or unspecified, or if it holds a type,
// default value, or aspect property that cannot be represented in protobuf
// form.
func TypeDefToProto(t *TypeDef) (*typedefpb.TypeDef, error) {
	switch t.Kind() {
	case StructKind:
		s, err := structToProto(t.StructType())
		if err != nil {
			return nil, err
		}
		return &typedefpb.TypeDef{Kind: &typedefpb.TypeDef_StructType{StructType: s}}, nil
	case EnumKind:
		e, err := enumToProto(t.EnumType())
		if err != nil {
			return nil, err
		}
		return &typedefpb.TypeDef{Kind: &typedefpb.TypeDef_EnumType{EnumType: e}}, nil
	default:
		return nil, fmt.Errorf("%w: type definition cannot be nil or unspecified", ErrBadTypeDef)
	}
}

func structFromProto(pb *typedefpb.Struct) (*StructTypeDef, error) {
	aspects, err := aspectsFromProto(pb.GetAspects())
	if err != nil {
		return nil, fmt.Errorf("struct %q: %w", pb.GetName(), err)
	}
	s, err := NewStruct(pb.GetName(), StructDoc(pb.GetDoc()), StructAspects(aspects))
	if err != nil {
		return nil, fmt.Errorf("struct %q: %w", pb.GetName(), err)
	}
	for i, fieldPB := range pb.GetFields() {
		field, err := fieldFromProto(fieldPB)
		if err != nil {
			return nil, fmt.Errorf("struct %q fields[%d]: %w", pb.GetName(), i, err)
		}
		if err := s.AddField(field); err != nil {
			return nil, fmt.Errorf("struct %q fields[%d]: %w", pb.GetName(), i, err)
		}
	}
	return s, nil
}

func fieldFromProto(pb *typedefpb.Struct_Field) (*Field, error) {
	fieldType, err := typeFromProto(pb.GetType())
	if err != nil {
		return nil, fmt.Errorf("type: %w", err)
	}
	defaultValue, err := constantFromProto(pb.GetDefaultValue())
	if err != nil {
		return nil, fmt.Errorf("default value: %w", err)
	}
	aspects, err := aspectsFromProto(pb.GetAspects())
	if err != nil {
		return nil, err
	}
	return NewField(pb.GetName(), fieldType,
		FieldDoc(pb.GetDoc()),
		FieldDefaultValue(defaultValue),
		FieldAspects(aspects))
}

func enumFromProto(pb *typedefpb.Enum) (*EnumTypeDef, error) {
	aspects, err := aspectsFromProto(pb.GetAspects())
	if err != nil {
		return nil, fmt.Errorf("enum %q: %w", pb.GetName(), err)
	}
	e, err := NewEnum(pb.GetName(), EnumDoc(pb.GetDoc()), EnumAspects(aspects))
	if err != nil {
		return nil, fmt.Errorf("enum %q: %w", pb.GetName(), err)
	}
	for i, constantPB := range pb.GetConstants() {
		constant, err := enumConstantFromProto(constantPB)
		if err != nil {
			return nil, fmt.Errorf("enum %q constants[%d]: %w", pb.GetName(), i, err)
		}
		if err := e.AddConstant(constant); err != nil {
			return nil, fmt.Errorf("enum %q constants[%d]: %w", pb.GetName(), i, err)
		}
	}
	return e, nil
}

func enumConstantFromProto(pb *typedefpb.Enum_EnumConstant) (*EnumConstant, error) {
	aspects, err := aspectsFromProto(pb.GetAspects())
	if err != nil {
		return nil, err
	}
	return NewEnumConstant(pb.GetName(), pb.GetId(),
		EnumConstantDoc(pb.GetDoc()),
		EnumConstantAspects(aspects))
}

func aspectsFromProto(pbs map[string]*typedefpb.Aspect) (map[string]*Aspect, error) {
	if len(pbs) == 0 {
		return nil, nil
	}
	aspects := make(map[string]*Aspect, len(pbs))
	for name, pb := range pbs {
		aspect, err := aspectFromProto(pb)
		if err != nil {
			return nil, fmt.Errorf("aspect %q: %w", name, err)
		}
		aspects[name] = aspect
	}
	return aspects, nil
}

func aspectFromProto(pb *typedefpb.Aspect) (*Aspect, error) {
	props := pb.GetProperties()
	properties := make(map[string]ref.Val, len(props))
	for key, constantPB := range props {
		v, err := constantFromProto(constantPB)
		if err != nil {
			return nil, fmt.Errorf("aspect property %q: %w", key, err)
		}
		properties[key] = v
	}
	return NewAspect(properties)
}

func structToProto(s *StructTypeDef) (*typedefpb.Struct, error) {
	aspects, err := aspectsToProto(s.Aspects())
	if err != nil {
		return nil, fmt.Errorf("struct %q: %w", s.Name(), err)
	}
	fields := s.Fields()
	pb := &typedefpb.Struct{
		Name:    optionalString(s.Name()),
		Doc:     optionalString(s.Doc()),
		Aspects: aspects,
		Fields:  make([]*typedefpb.Struct_Field, 0, len(fields)),
	}
	for _, field := range fields {
		fieldPB, err := fieldToProto(field)
		if err != nil {
			return nil, fmt.Errorf("struct %q field %q: %w", s.Name(), field.Name(), err)
		}
		pb.Fields = append(pb.Fields, fieldPB)
	}
	return pb, nil
}

func fieldToProto(f *Field) (*typedefpb.Struct_Field, error) {
	aspects, err := aspectsToProto(f.Aspects())
	if err != nil {
		return nil, err
	}
	pb := &typedefpb.Struct_Field{
		Name:    proto.String(f.Name()),
		Doc:     optionalString(f.Doc()),
		Aspects: aspects,
	}
	if f.Type().Kind() != types.UnspecifiedKind {
		pb.Type, err = typeToProto(f.Type())
		if err != nil {
			return nil, fmt.Errorf("type: %w", err)
		}
	}
	if f.HasDefaultValue() {
		pb.DefaultValue, err = constantToProto(f.DefaultValue())
		if err != nil {
			return nil, fmt.Errorf("default value: %w", err)
		}
	}
	return pb, nil
}

func enumToProto(e *EnumTypeDef) (*typedefpb.Enum, error) {
	aspects, err := aspectsToProto(e.Aspects())
	if err != nil {
		return nil, fmt.Errorf("enum %q: %w", e.Name(), err)
	}
	constants := e.Constants()
	pb := &typedefpb.Enum{
		Name:      optionalString(e.Name()),
		Doc:       optionalString(e.Doc()),
		Aspects:   aspects,
		Constants: make([]*typedefpb.Enum_EnumConstant, 0, len(constants)),
	}
	for _, constant := range constants {
		constantAspects, err := aspectsToProto(constant.Aspects())
		if err != nil {
			return nil, fmt.Errorf("enum %q constant %q: %w", e.Name(), constant.Name(), err)
		}
		pb.Constants = append(pb.Constants, &typedefpb.Enum_EnumConstant{
			Name:    proto.String(constant.Name()),
			Id:      proto.Int32(constant.ID()),
			Doc:     optionalString(constant.Doc()),
			Aspects: constantAspects,
		})
	}
	return pb, nil
}

func aspectsToProto(aspects map[string]*Aspect) (map[string]*typedefpb.Aspect, error) {
	if len(aspects) == 0 {
		return nil, nil
	}
	pbs := make(map[string]*typedefpb.Aspect, len(aspects))
	for name, aspect := range aspects {
		pb, err := aspectToProto(aspect)
		if err != nil {
			return nil, fmt.Errorf("aspect %q: %w", name, err)
		}
		pbs[name] = pb
	}
	return pbs, nil
}

func aspectToProto(a *Aspect) (*typedefpb.Aspect, error) {
	props := a.Properties()
	if len(props) == 0 {
		return &typedefpb.Aspect{}, nil
	}
	properties := make(map[string]*celpb.Constant, len(props))
	for key, v := range props {
		pb, err := constantToProto(v)
		if err != nil {
			return nil, fmt.Errorf("aspect property %q: %w", key, err)
		}
		properties[key] = pb
	}
	return &typedefpb.Aspect{Properties: properties}, nil
}

// optionalString returns nil for an empty string so that unset names and
// documentation are omitted from the encoded message, matching cel-cpp.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return proto.String(s)
}
