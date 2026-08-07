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
	"reflect"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// IsUnknownOrError returns whether the input element ref.Val is an ErrType or UnknownType.
func IsUnknownOrError(val ref.Val) bool {
	switch val.(type) {
	case *Unknown, *Err:
		return true
	}
	return false
}

// IsPrimitiveType returns whether the input element ref.Val is a primitive type.
// Note, primitive types do not include well-known types such as Duration and Timestamp.
func IsPrimitiveType(val ref.Val) bool {
	switch val.Type() {
	case BoolType, BytesType, DoubleType, IntType, StringType, UintType:
		return true
	}
	return false
}

// Equal returns whether the two ref.Value are heterogeneously equivalent.
func Equal(lhs ref.Val, rhs ref.Val) ref.Val {
	lNull := lhs == NullValue
	rNull := rhs == NullValue
	if lNull || rNull {
		return Bool(lNull == rNull)
	}
	return lhs.Equal(rhs)
}

// SizeCalculator calculates the recursive element size of values.
type SizeCalculator struct {
	version int
}

// NewSizeCalculator returns a new SizeCalculator for the specified version.
func NewSizeCalculator() *SizeCalculator {
	return &SizeCalculator{version: 0}
}

// Version returns the calculation version.
func (s *SizeCalculator) Version() int {
	return s.version
}

// AggregateSize returns the size of the input value, if known.
// Otherwise, a unit size of 1 is returned.
func (s *SizeCalculator) AggregateSize(val any) uint32 {
	switch v := val.(type) {
	case AggregateSizeVisitor:
		return v.AggregateSize(s)
	case traits.Mapper:
		total := uint32(1)
		it := v.Iterator()
		for it.HasNext() == True {
			key := it.Next()
			val, _ := v.Find(key)
			total = safeAddUint32(total, s.AggregateSize(key))
			total = safeAddUint32(total, s.AggregateSize(val))
		}
		return total
	case traits.Lister:
		total := uint32(1)
		it := v.Iterator()
		for it.HasNext() == True {
			total = safeAddUint32(total, s.AggregateSize(it.Next()))
		}
		return total
	case String:
		return safeUint32FromInt(utf8.RuneCountInString(string(v)))
	case Bytes:
		return safeUint32FromInt(len(v))
	case traits.Sizer:
		return safeUint32FromBoxedInt(v.Size().(Int))
	case Bool, Int, Uint, Double, Duration, Timestamp, Null, *Type, *Err, *Unknown:
		return 1
	case ref.Val:
		return s.AggregateSize(v.Value())
	case protoreflect.Value:
		return getProtoValueAggregateSize(s, v)
	case protoreflect.MapKey:
		return getProtoValueAggregateSize(s, v.Value())
	case protoreflect.Message:
		return getProtoMessageAggregateSize(s, v)
	case protoreflect.List:
		return getProtoListAggregateSize(s, v)
	case protoreflect.Map:
		return getProtoMapAggregateSize(s, v)
	case proto.Message:
		if v == nil {
			return 0
		}
		return getProtoMessageAggregateSize(s, v.ProtoReflect())
	case reflect.Value:
		return getReflectValueAggregateSize(s, v)
	case string:
		return safeUint32FromInt(utf8.RuneCountInString(v))
	case []byte:
		return safeUint32FromInt(len(v))
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool, time.Time, time.Duration, nil:
		return 1
	default:
		return getReflectValueAggregateSize(s, reflect.ValueOf(val))
	}
}

func getProtoValueAggregateSize(s AggregateSizer, v protoreflect.Value) uint32 {
	switch val := v.Interface().(type) {
	case string:
		return safeUint32FromInt(utf8.RuneCountInString(val))
	case []byte:
		return safeUint32FromInt(len(val))
	case protoreflect.Message:
		return getProtoMessageAggregateSize(s, val)
	case protoreflect.List:
		return getProtoListAggregateSize(s, val)
	case protoreflect.Map:
		return getProtoMapAggregateSize(s, val)
	default:
		return 1
	}
}

func getProtoFieldAggregateSize(s AggregateSizer, fd protoreflect.FieldDescriptor, v protoreflect.Value) uint32 {
	if fd.IsMap() {
		m := v.Map()
		total := uint32(1)
		valKind := fd.MapValue().Kind()
		m.Range(func(k protoreflect.MapKey, val protoreflect.Value) bool {
			total = safeAddUint32(total, getProtoMapKeyAggregateSize(fd.MapKey(), k))
			total = safeAddUint32(total, getProtoFieldByKindAggregateSize(s, valKind, val))
			return true
		})
		return total
	}
	if fd.IsList() {
		l := v.List()
		elemKind := fd.Kind()
		total := safeAddUint32(1, safeUint32FromInt(l.Len()))
		if elemKind == protoreflect.StringKind {
			total = 1
			for i := 0; i < l.Len(); i++ {
				total = safeAddUint32(total, safeUint32FromInt(utf8.RuneCountInString(l.Get(i).String())))
			}
		} else if elemKind == protoreflect.BytesKind {
			total = 1
			for i := 0; i < l.Len(); i++ {
				total = safeAddUint32(total, safeUint32FromInt(len(l.Get(i).Bytes())))
			}
		} else if elemKind == protoreflect.MessageKind || elemKind == protoreflect.GroupKind {
			total = 1
			for i := 0; i < l.Len(); i++ {
				total = safeAddUint32(total, getProtoMessageAggregateSize(s, l.Get(i).Message()))
			}
		}
		return total
	}
	return getProtoFieldByKindAggregateSize(s, fd.Kind(), v)
}

func getProtoMapKeyAggregateSize(keyDesc protoreflect.FieldDescriptor, k protoreflect.MapKey) uint32 {
	if keyDesc.Kind() == protoreflect.StringKind {
		return safeUint32FromInt(utf8.RuneCountInString(k.String()))
	}
	return 1
}

func getProtoFieldByKindAggregateSize(s AggregateSizer, kind protoreflect.Kind, v protoreflect.Value) uint32 {
	switch kind {
	case protoreflect.StringKind:
		return safeUint32FromInt(utf8.RuneCountInString(v.String()))
	case protoreflect.BytesKind:
		return safeUint32FromInt(len(v.Bytes()))
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return getProtoMessageAggregateSize(s, v.Message())
	default:
		return 1
	}
}

func getProtoMessageAggregateSize(s AggregateSizer, m protoreflect.Message) uint32 {
	if !m.IsValid() {
		return 0
	}
	total := uint32(1)
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		total = safeAddUint32(total, getProtoFieldAggregateSize(s, fd, v))
		return true
	})
	return total
}

func getProtoListAggregateSize(s AggregateSizer, l protoreflect.List) uint32 {
	if !l.IsValid() {
		return 0
	}
	total := uint32(1)
	for i := 0; i < l.Len(); i++ {
		total = safeAddUint32(total, getProtoValueAggregateSize(s, l.Get(i)))
	}
	return total
}

func getProtoMapAggregateSize(s AggregateSizer, m protoreflect.Map) uint32 {
	if !m.IsValid() {
		return 0
	}
	total := uint32(1)
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		total = safeAddUint32(total, getProtoValueAggregateSize(s, k.Value()))
		total = safeAddUint32(total, getProtoValueAggregateSize(s, v))
		return true
	})
	return total
}

func getReflectValueAggregateSize(s AggregateSizer, fieldVal reflect.Value) uint32 {
	if !fieldVal.IsValid() {
		return 0
	}
	switch fieldVal.Kind() {
	case reflect.String:
		return safeUint32FromInt(utf8.RuneCountInString(fieldVal.String()))
	case reflect.Slice, reflect.Array:
		elemType := fieldVal.Type().Elem()
		if elemType.Kind() == reflect.Uint8 {
			return safeUint32FromInt(fieldVal.Len())
		}
		total := safeAddUint32(1, safeUint32FromInt(fieldVal.Len()))
		switch elemType.Kind() {
		case reflect.String:
			total = 1
			for i := 0; i < fieldVal.Len(); i++ {
				total = safeAddUint32(total, safeUint32FromInt(utf8.RuneCountInString(fieldVal.Index(i).String())))
			}
		case reflect.Struct, reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map, reflect.Interface:
			total = 1
			for i := 0; i < fieldVal.Len(); i++ {
				total = safeAddUint32(total, getReflectValueAggregateSize(s, fieldVal.Index(i)))
			}
		}
		return total
	case reflect.Map:
		total := uint32(1)
		iter := fieldVal.MapRange()
		for iter.Next() {
			total = safeAddUint32(total, getReflectValueAggregateSize(s, iter.Key()))
			total = safeAddUint32(total, getReflectValueAggregateSize(s, iter.Value()))
		}
		return total
	case reflect.Pointer, reflect.Interface:
		if fieldVal.IsNil() {
			return 0
		}
		if fieldVal.CanInterface() {
			if sizer, ok := fieldVal.Interface().(AggregateSizeVisitor); ok {
				return sizer.AggregateSize(s)
			}
			if sizer, ok := fieldVal.Interface().(traits.Sizer); ok {
				return safeUint32FromBoxedInt(sizer.Size().(Int))
			}
		}
		return getReflectValueAggregateSize(s, fieldVal.Elem())
	case reflect.Struct:
		if fieldVal.Type() == timestampType || fieldVal.Type() == durationType {
			return 1
		}
		if fieldVal.CanInterface() {
			if sizer, ok := fieldVal.Interface().(AggregateSizeVisitor); ok {
				return sizer.AggregateSize(s)
			}
			if sizer, ok := fieldVal.Interface().(traits.Sizer); ok {
				return safeUint32FromBoxedInt(sizer.Size().(Int))
			}
		}
		total := uint32(1)
		t := fieldVal.Type()
		numFields := fieldVal.NumField()
		for i := 0; i < numFields; i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			fVal := fieldVal.Field(i)
			if !fVal.IsValid() || fVal.IsZero() {
				continue
			}
			total = safeAddUint32(total, getReflectValueAggregateSize(s, fVal))
		}
		return total
	default:
		return 1
	}
}
