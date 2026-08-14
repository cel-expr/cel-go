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
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/google/cel-go/common/types/ref"
	proto3pb "github.com/google/cel-go/test/proto3pb"
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
			want: 5,
		},
		{
			name: "sizer_bytes",
			val:  Bytes("world"),
			want: 5,
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
			want: 5,
		},
		{
			name: "proto_value_bytes",
			val:  protoreflect.ValueOfBytes([]byte("world")),
			want: 5,
		},
		{
			name: "proto_value_int",
			val:  protoreflect.ValueOfInt32(42),
			want: 1,
		},
		{
			name: "proto_map_key",
			val:  protoreflect.MapKey(protoreflect.ValueOfString("key")),
			want: 3,
		},
		{
			name: "proto_message",
			val:  &proto3pb.TestAllTypes{SingleString: "hello"},
			want: 6, // 1 (root) + 5 (string) = 6
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
			want: 9,
		},
		{
			name: "native_string",
			val:  "hello",
			want: 5,
		},
		{
			name: "native_bytes",
			val:  []byte("world"),
			want: 5,
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
			want: 4, // 1 (root) + 3 ("cel") = 4
		},
		{
			name: "custom_lister",
			val:  proxyLegacyList{proxy: NewRefValList(DefaultTypeAdapter, []ref.Val{String("a"), String("b")})},
			want: 3, // 1 (container) + 1 ("a") + 1 ("b") = 3
		},
		{
			name: "custom_mapper",
			val:  interopFoldableMap{Mapper: NewStringStringMap(DefaultTypeAdapter, map[string]string{"key": "val"})},
			want: 7, // 1 (container) + 3 ("key") + 3 ("val") = 7
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calculator := NewSizeCalculator()
			if got := calculator.AggregateSize(tc.val); got != tc.want {
				t.Errorf("AggregateSize(%v) got %d, want %d", tc.val, got, tc.want)
			}
		})
	}
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
		if got := calc.AggregateSize(list5); got == math.MaxUint32 {
			t.Errorf("AggregateSize for depth 5 got MaxUint32, want calculated size")
		}

		list6 := makeNestedList(5)
		if got := calc.AggregateSize(list6); got != math.MaxUint32 {
			t.Errorf("AggregateSize for depth 6 got %d, want MaxUint32", got)
		}
	})

	t.Run("maxDepth custom option", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		list2 := makeNestedList(1)
		if got := calc.AggregateSize(list2); got == math.MaxUint32 {
			t.Errorf("AggregateSize for depth 2 got MaxUint32, want calculated size")
		}

		list3 := makeNestedList(2)
		if got := calc.AggregateSize(list3); got != math.MaxUint32 {
			t.Errorf("AggregateSize for depth 3 got %d, want MaxUint32", got)
		}
	})

	t.Run("maxTraversal default limit 10000", func(t *testing.T) {
		calc := NewSizeCalculator()
		smallElems := make([]ref.Val, 100)
		for i := 0; i < 100; i++ {
			smallElems[i] = Int(i)
		}
		smallList := NewRefValList(adapter, smallElems)
		if got := calc.AggregateSize(smallList); got == math.MaxUint32 {
			t.Errorf("AggregateSize for 100 elements got MaxUint32, want calculated size")
		}

		largeElems := make([]ref.Val, 10001)
		for i := 0; i < 10001; i++ {
			largeElems[i] = Int(i)
		}
		largeList := NewRefValList(adapter, largeElems)
		if got := calc.AggregateSize(largeList); got != math.MaxUint32 {
			t.Errorf("AggregateSize for 10001 elements got %d, want MaxUint32", got)
		}
	})

	t.Run("maxTraversal custom option", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxTraversal(5))
		list4 := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4)})
		if got := calc.AggregateSize(list4); got == math.MaxUint32 {
			t.Errorf("AggregateSize for 5 nodes got MaxUint32, want calculated size")
		}

		list5 := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3), Int(4), Int(5)})
		if got := calc.AggregateSize(list5); got != math.MaxUint32 {
			t.Errorf("AggregateSize for 6 nodes got %d, want MaxUint32", got)
		}
	})

	t.Run("proto depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		msg := &proto3pb.TestAllTypes{
			RepeatedNestedMessage: []*proto3pb.TestAllTypes_NestedMessage{
				{Bb: 42},
			},
		}
		if got := calc.AggregateSize(msg); got != math.MaxUint32 {
			t.Errorf("AggregateSize for proto nested msg got %d, want MaxUint32", got)
		}
	})

	t.Run("cel map depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		nestedMap := NewRefValMap(adapter, map[ref.Val]ref.Val{
			String("k"): NewRefValMap(adapter, map[ref.Val]ref.Val{
				String("subk"): String("subv"),
			}),
		})
		if got := calc.AggregateSize(nestedMap); got != math.MaxUint32 {
			t.Errorf("AggregateSize for nested map got %d, want MaxUint32", got)
		}
	})

	t.Run("native struct depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		type Level3 struct{ Val string }
		type Level2 struct{ L3 Level3 }
		type Level1 struct{ L2 Level2 }

		obj := Level1{L2: Level2{L3: Level3{Val: "deep"}}}
		if got := calc.AggregateSize(obj); got != math.MaxUint32 {
			t.Errorf("AggregateSize for native struct depth > 2 got %d, want MaxUint32", got)
		}
	})

	t.Run("native map depth limit", func(t *testing.T) {
		calc := NewSizeCalculator(SizeCalculatorMaxDepth(2))
		m := map[string]map[string]string{
			"outer": {"inner": "val"},
		}
		if got := calc.AggregateSize(m); got != math.MaxUint32 {
			t.Errorf("AggregateSize for native nested map depth > 2 got %d, want MaxUint32", got)
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
				_ = calculator.AggregateSize(bm.val)
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
				_ = calculator.AggregateSize(val)
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
				_ = calc.AggregateSize(builtinList)
			}
		})
		b.Run(fmt.Sprintf("Amortized/custom_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.AggregateSize(customList)
			}
		})
		b.Run(fmt.Sprintf("Amortized/builtin_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.AggregateSize(builtinMap)
			}
		})
		b.Run(fmt.Sprintf("Amortized/custom_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.AggregateSize(customMap)
			}
		})

		// First-time / Uncached calculation
		b.Run(fmt.Sprintf("FirstTime/builtin_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				l := NewRefValList(adapter, listElems)
				_ = calc.AggregateSize(l)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/custom_list/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				l := proxyLegacyList{proxy: NewRefValList(adapter, listElems)}
				_ = calc.AggregateSize(l)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/builtin_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				m := NewRefValMap(adapter, mapEntries)
				_ = calc.AggregateSize(m)
			}
		})
		b.Run(fmt.Sprintf("FirstTime/custom_map/N=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				m := interopFoldableMap{Mapper: NewRefValMap(adapter, mapEntries)}
				_ = calc.AggregateSize(m)
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
				_ = calc.AggregateSize(builtinNested)
			}
		})
		b.Run(fmt.Sprintf("Complexity/custom_nested_list/Depth=%d_Width=%d", depth, width), func(b *testing.B) {
			b.ReportAllocs()
			calc := NewSizeCalculator()
			for i := 0; i < b.N; i++ {
				_ = calc.AggregateSize(customNested)
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
