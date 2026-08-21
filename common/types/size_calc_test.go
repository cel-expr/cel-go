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
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"

	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestCalculateSize(t *testing.T) {
	adapter := DefaultTypeAdapter

	tests := []struct {
		name string
		val  any
		want uint32
	}{
		{
			name: "aggregate_sizer_list",
			val:  NewRefValList(adapter, []ref.Val{Int(1), Int(2)}),
			want: 3,
		},
		{
			name: "sizer_string",
			val:  String("hello"),
			want: 1, // 5 chars round up to a single 10-char element unit
		},
		{
			name: "sizer_bytes",
			val:  Bytes("world"),
			want: 1,
		},
		{
			name: "err_val",
			val:  NewErr("test error"),
			want: 1,
		},
		{
			name: "unknown_val",
			val:  &Unknown{},
			want: 1,
		},
		{
			name: "type_val",
			val:  IntType,
			want: 1,
		},
		{
			name: "null_val",
			val:  NullValue,
			want: 1,
		},
		{
			name: "scalar_ref_val_int",
			val:  Int(42),
			want: 1,
		},
		{
			name: "scalar_ref_val_double",
			val:  Double(1.5),
			want: 1,
		},
		{
			name: "scalar_ref_val_bool",
			val:  True,
			want: 1,
		},
		{
			name: "scalar_ref_val_timestamp",
			val:  Timestamp{Time: time.Unix(100, 0)},
			want: 1,
		},
		{
			name: "scalar_ref_val_duration",
			val:  Duration{Duration: time.Second},
			want: 1,
		},
		{
			name: "proto_value_string",
			val:  protoreflect.ValueOfString("hello"),
			want: 1,
		},
		{
			name: "proto_value_bytes",
			val:  protoreflect.ValueOfBytes([]byte("world")),
			want: 1,
		},
		{
			name: "proto_value_int",
			val:  protoreflect.ValueOfInt32(42),
			want: 1,
		},
		{
			name: "proto_map_key",
			val:  protoreflect.MapKey(protoreflect.ValueOfString("key")),
			want: 1,
		},
		{
			name: "proto_message",
			val:  &proto3pb.TestAllTypes{SingleString: "hello"},
			want: 2, // 1 (root) + 1 (string unit) = 2
		},
		{
			name: "proto_message_with_list_and_map",
			val: &proto3pb.TestAllTypes{
				RepeatedString:  []string{"a", "b"},
				MapStringString: map[string]string{"k": "v"},
			},
			want: 7,
		},
		{
			name: "protoreflect_message",
			val:  (&proto3pb.TestAllTypes{SingleInt64: 10}).ProtoReflect(),
			want: 2, // 1 (root) + 1 (int64) = 2
		},
		{
			name: "protoreflect_list",
			val:  (&proto3pb.TestAllTypes{RepeatedString: []string{"a", "b"}}).ProtoReflect().Get((&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor().Fields().ByName("repeated_string")).List(),
			want: 3, // 1 (container) + 1("a") + 1("b") = 3
		},
		{
			name: "protoreflect_map",
			val:  (&proto3pb.TestAllTypes{MapStringString: map[string]string{"k": "v"}}).ProtoReflect().Get((&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor().Fields().ByName("map_string_string")).Map(),
			want: 3, // 1 (container) + 1("k") + 1("v") = 3
		},
		{
			name: "nil_proto_message",
			val:  (*proto3pb.TestAllTypes)(nil),
			want: 0,
		},
		{
			name: "reflect_value",
			val:  reflect.ValueOf("reflected"),
			want: 1,
		},
		{
			name: "native_string",
			val:  "hello",
			want: 1,
		},
		{
			name: "native_bytes",
			val:  []byte("world"),
			want: 1,
		},
		{
			name: "native_int",
			val:  42,
			want: 1,
		},
		{
			name: "native_float",
			val:  3.14,
			want: 1,
		},
		{
			name: "native_bool",
			val:  true,
			want: 1,
		},
		{
			name: "native_time",
			val:  time.Now(),
			want: 1,
		},
		{
			name: "native_duration",
			val:  time.Hour,
			want: 1,
		},
		{
			name: "native_nil",
			val:  nil,
			want: 1,
		},
		{
			name: "custom_struct",
			val:  struct{ Name string }{"cel"},
			want: 2, // 1 (root) + 1 ("cel") = 2
		},
		{
			name: "custom_lister",
			val:  proxyLegacyList{proxy: NewRefValList(DefaultTypeAdapter, []ref.Val{String("a"), String("b")})},
			want: 3, // 1 (container) + 1 ("a") + 1 ("b") = 3
		},
		{
			name: "custom_mapper",
			val:  interopFoldableMap{Mapper: NewStringStringMap(DefaultTypeAdapter, map[string]string{"key": "val"})},
			want: 3, // 1 (container) + 1 ("key") + 1 ("val") = 3
		},
		{
			name: "custom_pure_mapper",
			val:  customPureMapper{Mapper: NewStringStringMap(DefaultTypeAdapter, map[string]string{"key": "val"})},
			want: 3, // 1 (container) + 1 ("key") + 1 ("val") = 3
		},
		{
			name: "custom_sizer_struct_field",
			val:  struct{ Sizer traits.Sizer }{Sizer: customSizerVal(42)},
			want: 43, // 1 (struct container) + 42 (custom sizer) = 43
		},
		{
			name: "custom_visitor_struct_field",
			val:  struct{ Visitor customVisitorVal }{Visitor: customVisitorVal{Val: 1}},
			want: 101, // 1 (struct container) + 100 (custom visitor) = 101
		},
		{
			name: "custom_sizer_pointer",
			val:  newCustomSizerPtr(42),
			want: 42,
		},
		{
			name: "custom_visitor_pointer",
			val:  &customVisitorVal{Val: 1},
			want: 100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calculator := NewSizeCalculator()
			if got := calculator.ApproximateAggregateSize(tc.val).Size; got != tc.want {
				t.Errorf("ApproximateAggregateSize(%v) got %d, want %d", tc.val, got, tc.want)
			}
		})
	}
}

type customPureMapper struct {
	traits.Mapper
}

type customSizerVal int

func (c customSizerVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return int(c), nil
}

func (c customSizerVal) ConvertToType(typeVal ref.Type) ref.Val {
	return Int(c)
}

func (c customSizerVal) Equal(other ref.Val) ref.Val {
	return Bool(c == other)
}

func (c customSizerVal) Type() ref.Type {
	return IntType
}

func (c customSizerVal) Value() any {
	return int(c)
}

func (c customSizerVal) Size() ref.Val {
	return Int(c)
}

type customSizerPtr struct {
	val int
}

func (c *customSizerPtr) Size() ref.Val {
	return Int(c.val)
}

func newCustomSizerPtr(v int) *customSizerPtr {
	return &customSizerPtr{val: v}
}

type customVisitorVal struct {
	Val int
}

func (c customVisitorVal) AggregateSize(sizer AggregateSizer) uint32 {
	return 100
}

func TestNativeObjCalculateSizeNil(t *testing.T) {
	nilNative := &nativeObj{}
	if got := nilNative.AggregateSize(NewSizeCalculator()); got != 0 {
		t.Errorf("nil nativeObj.AggregateSize() got %d, want 0", got)
	}
}

func TestSizeCalculatorOptions(t *testing.T) {
	adapter := DefaultTypeAdapter

	var makeNestedList func(depth int) ref.Val
	makeNestedList = func(depth int) ref.Val {
		if depth <= 1 {
			return NewRefValList(adapter, []ref.Val{Int(1)})
		}
		return NewRefValList(adapter, []ref.Val{makeNestedList(depth - 1)})
	}

	t.Run("maxDepth default limit 5", func(t *testing.T) {
		calc := NewSizeCalculator()
		list5 := makeNestedList(4)
		if got := calc.ApproximateAggregateSize(list5); got.LimitExceeded || got.Size == math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for depth 5 got %+v, want within limits", got)
		}

		list6 := makeNestedList(5)
		if got := calc.ApproximateAggregateSize(list6); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for depth 6 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxDepth custom option", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		list2 := makeNestedList(1)
		if got := calc.ApproximateAggregateSize(list2); got.LimitExceeded || got.Size == math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for depth 2 got %+v, want within limits", got)
		}

		list3 := makeNestedList(2)
		if got := calc.ApproximateAggregateSize(list3); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for depth 3 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal default limit 10000", func(t *testing.T) {
		calc := NewSizeCalculator()
		smallElems := make([]ref.Val, 100)
		for i := 0; i < 100; i++ {
			smallElems[i] = Int(i)
		}
		smallList := NewRefValList(adapter, smallElems)
		if got := calc.ApproximateAggregateSize(smallList); got.LimitExceeded || got.Size == math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for 100 elements got %+v, want within limits", got)
		}

		largeElems := make([]ref.Val, 10001)
		for i := 0; i < 10001; i++ {
			largeElems[i] = Int(i)
		}
		largeList := NewRefValList(adapter, largeElems)
		if got := calc.ApproximateAggregateSize(largeList); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for 10001 elements got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal custom option", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(5))
		list4 := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4)})
		if got := calc.ApproximateAggregateSize(list4); got.LimitExceeded || got.Size == math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for 5 nodes got %+v, want within limits", got)
		}

		list5 := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4), Int(5)})
		if got := calc.ApproximateAggregateSize(list5); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for 6 nodes got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("proto depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		msg := &proto3pb.TestAllTypes{
			RepeatedNestedMessage: []*proto3pb.TestAllTypes_NestedMessage{
				{Bb: 42},
			},
		}
		if got := calc.ApproximateAggregateSize(msg); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for proto nested msg got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("cel map depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		nestedMap := NewRefValMap(adapter, map[ref.Val]ref.Val{
			String("k"): NewRefValMap(adapter, map[ref.Val]ref.Val{
				String("subk"): String("subv"),
			}),
		})
		if got := calc.ApproximateAggregateSize(nestedMap); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for nested map got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("native struct depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		type Level3 struct{ Val string }
		type Level2 struct{ L3 Level3 }
		type Level1 struct{ L2 Level2 }

		obj := Level1{L2: Level2{L3: Level3{Val: "deep"}}}
		if got := calc.ApproximateAggregateSize(obj); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for native struct depth > 2 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("native map depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		m := map[string]map[string]string{
			"outer": {"inner": "val"},
		}
		if got := calc.ApproximateAggregateSize(m); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for native nested map depth > 2 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal map limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(3))
		m := NewRefValMap(adapter, map[ref.Val]ref.Val{
			String("k1"): String("v1"),
			String("k2"): String("v2"),
		})
		if got := calc.ApproximateAggregateSize(m); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for map traversal > 3 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal proto limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(2))
		msg := &proto3pb.TestAllTypes{
			SingleString: "hello",
			SingleInt64:  42,
		}
		if got := calc.ApproximateAggregateSize(msg); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for proto traversal > 2 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal native struct limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(2))
		s := struct{ A, B, C int }{A: 1, B: 2, C: 3}
		if got := calc.ApproximateAggregateSize(s); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for native struct traversal > 2 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal native map limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(3))
		m := map[string]int{"a": 1, "b": 2}
		if got := calc.ApproximateAggregateSize(m); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for native map traversal > 3 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("maxTraversal native slice limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(3))
		slice := []string{"a", "b", "c"}
		if got := calc.ApproximateAggregateSize(slice); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize for native slice traversal > 3 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("zero depth limit saturation", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(0))
		if got := calc.ApproximateAggregateSize(Int(42)); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize with depth 0 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})

	t.Run("zero traversal limit saturation", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(0))
		if got := calc.ApproximateAggregateSize(Int(42)); !got.LimitExceeded || got.Size != math.MaxUint32 {
			t.Errorf("ApproximateAggregateSize with traversal 0 got %+v, want MaxUint32 and LimitExceeded", got)
		}
	})
}

type nestedNative struct {
	NestedList []string
	NestedMap  map[string]int
}

type rootNative struct {
	Name     string
	Count    int
	Children []nestedNative
}

func BenchmarkCalculateSizeAmortized(b *testing.B) {
	adapter := DefaultTypeAdapter

	flatList := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4), Int(5), Int(6), Int(7), Int(8)})
	nestedList := NewRefValList(adapter, []ref.Val{
		String("hello"),
		NewRefValList(adapter, []ref.Val{String("nested1"), String("nested2")}),
		NewRefValMap(adapter, map[ref.Val]ref.Val{String("k1"): String("v1")}),
	})
	flatMap := NewRefValMap(adapter, map[ref.Val]ref.Val{
		String("k1"): Int(1),
		String("k2"): Int(2),
		String("k3"): Int(3),
	})
	protoMsg := &proto3pb.TestAllTypes{
		SingleString:   "hello world",
		SingleInt64:    42,
		RepeatedString: []string{"first", "second", "third"},
		MapStringString: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	nativeData := &rootNative{
		Name:  "parent",
		Count: 100,
		Children: []nestedNative{
			{NestedList: []string{"a", "b", "c"}, NestedMap: map[string]int{"k1": 1, "k2": 2}},
			{NestedList: []string{"d", "e"}, NestedMap: map[string]int{"k3": 3}},
		},
	}
	nativeVal := adapter.NativeToValue(nativeData)

	benchmarks := []struct {
		name string
		val  any
	}{
		{name: "scalar_int", val: Int(42)},
		{name: "scalar_string", val: String("hello world this is a test string")},
		{name: "native_string", val: "hello world this is a test string"},
		{name: "native_bytes", val: []byte("hello world this is a test string")},
		{name: "list_flat", val: flatList},
		{name: "list_nested", val: nestedList},
		{name: "map_flat", val: flatMap},
		{name: "custom_list_flat", val: proxyLegacyList{proxy: flatList}},
		{name: "custom_list_nested", val: proxyLegacyList{proxy: nestedList}},
		{name: "custom_map_flat", val: interopFoldableMap{Mapper: flatMap}},
		{name: "proto_message", val: protoMsg},
		{name: "proto_obj", val: adapter.NativeToValue(protoMsg)},
		{name: "native_obj", val: nativeVal},
		{name: "native_struct", val: nativeData},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			calculator := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calculator.ApproximateAggregateSize(bm.val)
			}
		})
	}
}

func BenchmarkCalculateSizeDynamic(b *testing.B) {
	adapter := DefaultTypeAdapter

	benchmarks := []struct {
		name  string
		valFn func() any
	}{
		{
			name: "list_flat",
			valFn: func() any {
				return NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4), Int(5), Int(6), Int(7), Int(8)})
			},
		},
		{
			name: "list_nested",
			valFn: func() any {
				return NewRefValList(adapter, []ref.Val{
					String("hello"),
					NewRefValList(adapter, []ref.Val{String("nested1"), String("nested2")}),
					NewRefValMap(adapter, map[ref.Val]ref.Val{String("k1"): String("v1")}),
				})
			},
		},
		{
			name: "map_flat",
			valFn: func() any {
				return NewRefValMap(adapter, map[ref.Val]ref.Val{
					String("k1"): Int(1),
					String("k2"): Int(2),
					String("k3"): Int(3),
				})
			},
		},
		{
			name: "custom_list_flat",
			valFn: func() any {
				return proxyLegacyList{proxy: NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4), Int(5), Int(6), Int(7), Int(8)})}
			},
		},
		{
			name: "custom_list_nested",
			valFn: func() any {
				return proxyLegacyList{proxy: NewRefValList(adapter, []ref.Val{
					String("hello"),
					NewRefValList(adapter, []ref.Val{String("nested1"), String("nested2")}),
					NewRefValMap(adapter, map[ref.Val]ref.Val{String("k1"): String("v1")}),
				})}
			},
		},
		{
			name: "custom_map_flat",
			valFn: func() any {
				return interopFoldableMap{Mapper: NewRefValMap(adapter, map[ref.Val]ref.Val{
					String("k1"): Int(1),
					String("k2"): Int(2),
					String("k3"): Int(3),
				})}
			},
		},
		{
			name: "proto_obj",
			valFn: func() any {
				return adapter.NativeToValue(&proto3pb.TestAllTypes{
					SingleString:   "hello world",
					SingleInt64:    42,
					RepeatedString: []string{"first", "second", "third"},
					MapStringString: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				})
			},
		},
		{
			name: "native_obj",
			valFn: func() any {
				return adapter.NativeToValue(&rootNative{
					Name:  "parent",
					Count: 100,
					Children: []nestedNative{
						{NestedList: []string{"a", "b", "c"}, NestedMap: map[string]int{"k1": 1, "k2": 2}},
						{NestedList: []string{"d", "e"}, NestedMap: map[string]int{"k3": 3}},
					},
				})
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			calculator := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				val := bm.valFn()
				_ = calculator.ApproximateAggregateSize(val)
			}
		})
	}
}

func BenchmarkCalculateSizeScaled(b *testing.B) {
	adapter := DefaultTypeAdapter
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		// Prepare list elements
		listElems := make([]ref.Val, size)
		for i := 0; i < size; i++ {
			listElems[i] = Int(i)
		}
		builtinList := NewRefValList(adapter, listElems)
		customList := proxyLegacyList{proxy: builtinList}

		// Prepare map entries
		mapEntries := make(map[ref.Val]ref.Val, size)
		for i := 0; i < size; i++ {
			mapEntries[String(fmt.Sprintf("k%d", i))] = Int(i)
		}
		builtinMap := NewRefValMap(adapter, mapEntries)
		customMap := interopFoldableMap{Mapper: builtinMap}

		// Amortized (repeated calculation on memoized vs unmemoized custom instance)
		b.Run(fmt.Sprintf("Amortized/builtin_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(builtinList)
			}
		})
		b.Run(fmt.Sprintf("Amortized/custom_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(customList)
			}
		})
		b.Run(fmt.Sprintf("Amortized/builtin_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(builtinMap)
			}
		})
		b.Run(fmt.Sprintf("Amortized/custom_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(customMap)
			}
		})
		b.Run(fmt.Sprintf("Amortized/custom_pure_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			pureMap := customPureMapper{Mapper: builtinMap}
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(pureMap)
			}
		})

		// First-time / Uncached calculation
		b.Run(fmt.Sprintf("FirstTime/builtin_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				l := NewRefValList(adapter, listElems)
				_ = calc.ApproximateAggregateSize(l)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/custom_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				l := proxyLegacyList{proxy: NewRefValList(adapter, listElems)}
				_ = calc.ApproximateAggregateSize(l)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/builtin_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				m := NewRefValMap(adapter, mapEntries)
				_ = calc.ApproximateAggregateSize(m)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/custom_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				m := interopFoldableMap{Mapper: NewRefValMap(adapter, mapEntries)}
				_ = calc.ApproximateAggregateSize(m)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/custom_pure_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				m := customPureMapper{Mapper: NewRefValMap(adapter, mapEntries)}
				_ = calc.ApproximateAggregateSize(m)
			}
		})
	}

	// Benchmark nested tree complexity (Depth x Width)
	depths := []int{2, 3}
	width := 10
	for _, depth := range depths {
		builtinNested := createNestedList(adapter, depth, width)
		customNested := createNestedCustomList(adapter, depth, width)

		b.Run(fmt.Sprintf("Complexity/builtin_nested_list/Depth=%d_Width=%d", depth, width), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(builtinNested)
			}
		})
		b.Run(fmt.Sprintf("Complexity/custom_nested_list/Depth=%d_Width=%d", depth, width), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.ApproximateAggregateSize(customNested)
			}
		})
	}
}

func createNestedList(adapter Adapter, depth, width int) ref.Val {
	if depth <= 1 {
		elems := make([]ref.Val, width)
		for i := 0; i < width; i++ {
			elems[i] = Int(i)
		}
		return NewRefValList(adapter, elems)
	}
	elems := make([]ref.Val, width)
	for i := 0; i < width; i++ {
		elems[i] = createNestedList(adapter, depth-1, width)
	}
	return NewRefValList(adapter, elems)
}

func createNestedCustomList(adapter Adapter, depth, width int) ref.Val {
	if depth <= 1 {
		elems := make([]ref.Val, width)
		for i := 0; i < width; i++ {
			elems[i] = Int(i)
		}
		return proxyLegacyList{proxy: NewRefValList(adapter, elems)}
	}
	elems := make([]ref.Val, width)
	for i := 0; i < width; i++ {
		elems[i] = createNestedCustomList(adapter, depth-1, width)
	}
	return proxyLegacyList{proxy: NewRefValList(adapter, elems)}
}

func TestSizeCalculatorStringUnitLength(t *testing.T) {
	tests := []struct {
		name string
		opts []SizeCalculatorOption
		val  any
		want uint32
	}{
		{name: "empty_string_unit", val: String(""), want: 1},
		{name: "one_unit_exact", val: String("0123456789"), want: 1},
		{name: "one_unit_plus_one", val: String("0123456789a"), want: 2},
		{name: "three_units", val: String("0123456789012345678901"), want: 3},
		{name: "bytes_two_units", val: Bytes("01234567890"), want: 2},
		{name: "native_string_two_units", val: "01234567890", want: 2},
		{name: "native_bytes_two_units", val: []byte("01234567890"), want: 2},
		{name: "reflect_string_two_units", val: reflect.ValueOf("01234567890"), want: 2},
		{
			name: "unit_length_one",
			opts: []SizeCalculatorOption{SizeCalculatorStringUnitLength(1)},
			val:  String("hello"),
			want: 5,
		},
		{
			name: "unit_length_below_one_clamped",
			opts: []SizeCalculatorOption{SizeCalculatorStringUnitLength(0)},
			val:  String("hello"),
			want: 5,
		},
		{
			name: "unit_length_large",
			opts: []SizeCalculatorOption{SizeCalculatorStringUnitLength(100)},
			val:  String("hello world, hello world, hello world"),
			want: 1,
		},
		{
			// Sizes are measured in bytes, not characters: four 3-byte CJK characters
			// occupy 12 bytes and count as two 10-byte units.
			name: "multibyte_counted_in_bytes",
			val:  String("日本語字"),
			want: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calc := NewSizeCalculator(tc.opts...)
			if got := calc.ApproximateAggregateSize(tc.val).Size; got != tc.want {
				t.Errorf("ApproximateAggregateSize(%v) got %d, want %d", tc.val, got, tc.want)
			}
		})
	}
}

func TestEstimateAggregateSize(t *testing.T) {
	adapter := DefaultTypeAdapter

	t.Run("within_limits", func(t *testing.T) {
		calc := NewSizeCalculator()
		est := calc.ApproximateAggregateSize(NewRefValList(adapter, []ref.Val{Int(1), Int(2)}))
		if est.Size != 3 || est.LimitExceeded {
			t.Errorf("EstimateAggregateSize() got %+v, want {Size: 3, LimitExceeded: false}", est)
		}
	})

	t.Run("traversal_limit_exceeded", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(2))
		list := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3)})
		est := calc.ApproximateAggregateSize(list)
		if est.Size != math.MaxUint32 || !est.LimitExceeded {
			t.Errorf("EstimateAggregateSize() got %+v, want {Size: MaxUint32, LimitExceeded: true}", est)
		}
	})

	t.Run("depth_limit_exceeded", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(1))
		list := NewRefValList(adapter, []ref.Val{NewRefValList(adapter, []ref.Val{Int(1)})})
		est := calc.ApproximateAggregateSize(list)
		if est.Size != math.MaxUint32 || !est.LimitExceeded {
			t.Errorf("EstimateAggregateSize() got %+v, want {Size: MaxUint32, LimitExceeded: true}", est)
		}
	})

	t.Run("saturation_without_limit", func(t *testing.T) {
		// Two custom sizers each reporting MaxUint32 elements saturate the sum without
		// tripping the depth or traversal limits.
		calc := NewSizeCalculator()
		val := struct{ A, B traits.Sizer }{
			A: customSizerVal(math.MaxUint32),
			B: customSizerVal(math.MaxUint32),
		}
		est := calc.ApproximateAggregateSize(val)
		if est.Size != math.MaxUint32 || est.LimitExceeded {
			t.Errorf("EstimateAggregateSize() got %+v, want {Size: MaxUint32, LimitExceeded: false}", est)
		}
	})
}

func TestAggregateSizeConcurrentAccess(t *testing.T) {
	// Immutable lists and maps may be shared across concurrent evaluations; the aggregate
	// size memoization must be race-free (validated under `go test -race`).
	adapter := DefaultTypeAdapter
	sharedList := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3)})
	sharedMap := NewRefValMap(adapter, map[ref.Val]ref.Val{String("k"): String("v")})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			calc := NewSizeCalculator()
			if got := calc.ApproximateAggregateSize(sharedList).Size; got != 4 {
				t.Errorf("ApproximateAggregateSize(list) got %d, want 4", got)
			}
			if got := calc.ApproximateAggregateSize(sharedMap).Size; got != 3 {
				t.Errorf("ApproximateAggregateSize(map) got %d, want 3", got)
			}
		}()
	}
	wg.Wait()
}

func TestAggregateSizeAbortedComputationNotMemoized(t *testing.T) {
	// A sizing aborted at the calculator's limits depends on where in the traversal the
	// value was encountered and must not be memoized: a later sizing within limits must
	// return the true size with no limit-exceeded signal.
	adapter := DefaultTypeAdapter

	t.Run("list", func(t *testing.T) {
		shared := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4), Int(5)})
		strict := NewSizeCalculator(SizeCalculatorMaxTraversal(2))
		if est := strict.ApproximateAggregateSize(shared); !est.LimitExceeded {
			t.Fatalf("strict EstimateAggregateSize() got %+v, want LimitExceeded", est)
		}
		generous := NewSizeCalculator()
		est := generous.ApproximateAggregateSize(shared)
		if est.Size != 6 || est.LimitExceeded {
			t.Errorf("generous EstimateAggregateSize() got %+v, want {Size: 6, LimitExceeded: false}", est)
		}
	})

	t.Run("map", func(t *testing.T) {
		shared := NewRefValMap(adapter, map[ref.Val]ref.Val{
			Int(1): Int(2),
			Int(3): Int(4),
		})
		strict := NewSizeCalculator(SizeCalculatorMaxTraversal(2))
		if est := strict.ApproximateAggregateSize(shared); !est.LimitExceeded {
			t.Fatalf("strict EstimateAggregateSize() got %+v, want LimitExceeded", est)
		}
		generous := NewSizeCalculator()
		est := generous.ApproximateAggregateSize(shared)
		if est.Size != 5 || est.LimitExceeded {
			t.Errorf("generous EstimateAggregateSize() got %+v, want {Size: 5, LimitExceeded: false}", est)
		}
	})

	t.Run("completed_computation_is_memoized", func(t *testing.T) {
		shared := NewRefValList(adapter, []ref.Val{Int(1), Int(2)})
		calc := NewSizeCalculator()
		if got := calc.ApproximateAggregateSize(shared).Size; got != 3 {
			t.Fatalf("ApproximateAggregateSize() got %d, want 3", got)
		}
		// A subsequent sizing under a stricter budget serves the memoized total rather
		// than recomputing (and aborting).
		strict := NewSizeCalculator(SizeCalculatorMaxTraversal(2))
		est := strict.ApproximateAggregateSize(shared)
		if est.Size != 3 || est.LimitExceeded {
			t.Errorf("strict EstimateAggregateSize() after memoization got %+v, want {Size: 3, LimitExceeded: false}", est)
		}
	})
}
