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
	"math"
	"reflect"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

const (
	defaultSizeCalculatorMaxDepth     = 5
	defaultSizeCalculatorMaxTraversal = 10000
)

// SizeCalculatorOption configures a SizeCalculator instance.
type SizeCalculatorOption func(*SizeCalculator)

// SizeCalculatorMaxDepth sets the maximum object depth limit before saturating to math.MaxUint32.
func SizeCalculatorMaxDepth(depth int) SizeCalculatorOption {
	return func(s *SizeCalculator) {
		s.maxDepth = depth
	}
}

// SizeCalculatorMaxTraversal sets the maximum object traversal limit before saturating to math.MaxUint32.
func SizeCalculatorMaxTraversal(traversal int) SizeCalculatorOption {
	return func(s *SizeCalculator) {
		s.maxTraversal = traversal
	}
}

// SizeCalculator calculates the recursive element size of values.
type SizeCalculator struct {
	version      int
	maxDepth     int
	maxTraversal int
}

// NewSizeCalculator returns a new SizeCalculator configured with optional SizeCalculatorOption settings.
func NewSizeCalculator(opts ...SizeCalculatorOption) *SizeCalculator {
	s := &SizeCalculator{
		version:      0,
		maxDepth:     defaultSizeCalculatorMaxDepth,
		maxTraversal: defaultSizeCalculatorMaxTraversal,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Version returns the calculation version.
func (s *SizeCalculator) Version() int {
	return s.version
}

type sizeContext struct {
	calc           *SizeCalculator
	depth          int
	traversalCount *int
}

func (c sizeContext) childContext() sizeContext {
	c.depth++
	return c
}

func (c sizeContext) visitNode() bool {
	*c.traversalCount++
	if *c.traversalCount > c.calc.maxTraversal || c.depth > c.calc.maxDepth {
		return false
	}
	return true
}

// AggregateSize returns the size of the input value, if known.
// Otherwise, a unit size of 1 is returned.
func (s *SizeCalculator) AggregateSize(val any) uint32 {
	traversals := 0
	ctx := sizeContext{
		calc:           s,
		depth:          1,
		traversalCount: &traversals,
	}
	return ctx.AggregateSize(val)
}

// AggregateSize implements the ref.Val interface and allows for the generation of nested
// child context values which are necessary for correct traversal count tracking.
func (c sizeContext) AggregateSize(val any) uint32 {
	if !c.visitNode() {
		return math.MaxUint32
	}
	switch v := val.(type) {
	case AggregateSizeVisitor:
		return v.AggregateSize(c.childContext())
	case traits.Foldable:
		f := foldableAggregateSizer{sizer: c.childContext(), total: 1}
		v.Fold(&f)
		return f.total
	case traits.Mapper:
		total := uint32(1)
		it := v.Iterator()
		childCtx := c.childContext()
		for it.HasNext() == True {
			key := it.Next()
			val, _ := v.Find(key)
			total = safeAddUint32(total, childCtx.AggregateSize(key))
			total = safeAddUint32(total, childCtx.AggregateSize(val))
		}
		return total
	case traits.Lister:
		total := uint32(1)
		it := v.Iterator()
		childCtx := c.childContext()
		for it.HasNext() == True {
			total = safeAddUint32(total, childCtx.AggregateSize(it.Next()))
		}
		return total
	case traits.Sizer:
		return safeUint32FromBoxedInt(v.Size().(Int))
	case Bool, Int, Uint, Double, Duration, Timestamp, Null, *Type, *Err, *Unknown:
		return 1
	case ref.Val:
		return c.AggregateSize(v.Value())
	case protoreflect.Value:
		return c.AggregateSize(v.Interface())
	case protoreflect.MapKey:
		return c.AggregateSize(v.Value().Interface())
	case protoreflect.Message:
		return getProtoMessageAggregateSize(c, v)
	case protoreflect.List:
		return getProtoListAggregateSize(c, v)
	case protoreflect.Map:
		return getProtoMapAggregateSize(c, v)
	case proto.Message:
		if v == nil {
			return 0
		}
		return getProtoMessageAggregateSize(c, v.ProtoReflect())
	case reflect.Value:
		return getReflectValueAggregateSize(c, v)
	case string:
		return safeUint32FromInt(utf8.RuneCountInString(v))
	case []byte:
		return safeUint32FromInt(len(v))
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool, time.Time, time.Duration, nil:
		return 1
	default:
		return getReflectValueAggregateSize(c, reflect.ValueOf(val))
	}
}

func getProtoFieldAggregateSize(c sizeContext, fd protoreflect.FieldDescriptor, v protoreflect.Value) uint32 {
	if !c.visitNode() {
		return math.MaxUint32
	}
	childCtx := c.childContext()
	if fd.IsMap() {
		return getProtoMapAggregateSize(childCtx, v.Map())
	}
	if fd.IsList() {
		return getProtoListAggregateSize(childCtx, v.List())
	}
	return childCtx.AggregateSize(v.Interface())
}

func getProtoMessageAggregateSize(c sizeContext, m protoreflect.Message) uint32 {
	if !m.IsValid() {
		return 0
	}
	if !c.visitNode() {
		return math.MaxUint32
	}
	childCtx := c.childContext()
	total := uint32(1)
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		total = safeAddUint32(total, getProtoFieldAggregateSize(childCtx, fd, v))
		return true
	})
	return total
}

func getProtoListAggregateSize(c sizeContext, l protoreflect.List) uint32 {
	if !l.IsValid() {
		return 0
	}
	if !c.visitNode() {
		return math.MaxUint32
	}
	childCtx := c.childContext()
	total := uint32(1)
	for i := range l.Len() {
		total = safeAddUint32(total, childCtx.AggregateSize(l.Get(i).Interface()))
	}
	return total
}

func getProtoMapAggregateSize(c sizeContext, m protoreflect.Map) uint32 {
	if !m.IsValid() {
		return 0
	}
	if !c.visitNode() {
		return math.MaxUint32
	}
	childCtx := c.childContext()
	total := uint32(1)
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		total = safeAddUint32(total, childCtx.AggregateSize(k.Value().Interface()))
		total = safeAddUint32(total, childCtx.AggregateSize(v.Interface()))
		return true
	})
	return total
}

func getReflectValueAggregateSize(c sizeContext, fieldVal reflect.Value) uint32 {
	if !fieldVal.IsValid() {
		return 0
	}
	if !c.visitNode() {
		return math.MaxUint32
	}
	childCtx := c.childContext()
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
				total = safeAddUint32(total, childCtx.AggregateSize(fieldVal.Index(i).String()))
			}
		case reflect.Struct, reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map, reflect.Interface:
			total = 1
			for i := 0; i < fieldVal.Len(); i++ {
				total = safeAddUint32(total, getReflectValueAggregateSize(childCtx, fieldVal.Index(i)))
			}
		}
		return total
	case reflect.Map:
		total := uint32(1)
		iter := fieldVal.MapRange()
		for iter.Next() {
			total = safeAddUint32(total, getReflectValueAggregateSize(childCtx, iter.Key()))
			total = safeAddUint32(total, getReflectValueAggregateSize(childCtx, iter.Value()))
		}
		return total
	case reflect.Pointer, reflect.Interface:
		if fieldVal.IsNil() {
			return 0
		}
		if sz, ok := checkCustomSizer(childCtx, fieldVal); ok {
			return sz
		}
		return getReflectValueAggregateSize(c, fieldVal.Elem())
	case reflect.Struct:
		if fieldVal.Type() == timestampType || fieldVal.Type() == durationType {
			return 1
		}
		if sz, ok := checkCustomSizer(childCtx, fieldVal); ok {
			return sz
		}
		total := uint32(1)
		t := fieldVal.Type()
		numFields := fieldVal.NumField()
		for i := range numFields {
			if !t.Field(i).IsExported() {
				continue
			}
			fVal := fieldVal.Field(i)
			if !fVal.IsValid() || fVal.IsZero() {
				continue
			}
			total = safeAddUint32(total, getReflectValueAggregateSize(childCtx, fVal))
		}
		return total
	default:
		return 1
	}
}

func checkCustomSizer(c sizeContext, fieldVal reflect.Value) (uint32, bool) {
	if !fieldVal.CanInterface() {
		return 0, false
	}

	switch sizer := fieldVal.Interface().(type) {
	case AggregateSizeVisitor:
		return sizer.AggregateSize(c), true
	case traits.Sizer:
		return safeUint32FromBoxedInt(sizer.Size().(Int)), true
	default:
		return 0, false
	}
}
