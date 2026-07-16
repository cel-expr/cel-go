// Copyright 2026 Google LLC
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
	"fmt"
	"reflect"

	"github.com/google/cel-go/common/types/ref"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Enum represents a protobuf enum value as a first-class CEL value with its
// own type identity, used when strong enum handling is enabled on the
// registry. The value tracks the fully qualified enum type name along with
// the numeric value of the enum.
type Enum struct {
	celType *Type
	value   int32
}

// NewEnumValue creates an Enum value from a fully qualified enum type name
// and the numeric value of the enum.
func NewEnumValue(typeName string, value int32) Enum {
	return Enum{
		celType: NewOpaqueType(typeName),
		value:   value,
	}
}

// ConvertToNative implements ref.Val.ConvertToNative.
func (e Enum) ConvertToNative(typeDesc reflect.Type) (any, error) {
	switch typeDesc.Kind() {
	case reflect.Int32:
		if typeDesc == reflect.TypeOf(protoreflect.EnumNumber(0)) {
			return protoreflect.EnumNumber(e.value), nil
		}
		return reflect.ValueOf(e.value).Convert(typeDesc).Interface(), nil
	case reflect.Int64:
		return reflect.ValueOf(int64(e.value)).Convert(typeDesc).Interface(), nil
	case reflect.Interface:
		ev := e.Value()
		if reflect.TypeOf(ev).Implements(typeDesc) {
			return ev, nil
		}
		if reflect.TypeOf(e).Implements(typeDesc) {
			return e, nil
		}
	}
	return nil, fmt.Errorf("type conversion error from '%s' to '%v'", e.celType.TypeName(), typeDesc)
}

// ConvertToType implements ref.Val.ConvertToType.
func (e Enum) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case IntType:
		return Int(e.value)
	case TypeType:
		return e.celType
	}
	if typeVal.TypeName() == e.celType.TypeName() {
		return e
	}
	return NewErr("type conversion error from '%s' to '%s'", e.celType.TypeName(), typeVal.TypeName())
}

// Equal implements ref.Val.Equal.
func (e Enum) Equal(other ref.Val) ref.Val {
	o, ok := other.(Enum)
	if !ok {
		return False
	}
	return Bool(e.celType.TypeName() == o.celType.TypeName() && e.value == o.value)
}

// Type implements ref.Val.Type.
func (e Enum) Type() ref.Type {
	return e.celType
}

// Value implements ref.Val.Value.
func (e Enum) Value() any {
	return int64(e.value)
}
