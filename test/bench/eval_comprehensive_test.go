package bench

import (
	"reflect"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/ext"
	proto3pb "cel.dev/cel-go/test/proto3pb"
)

type NativeBenchNested struct {
	NestedName string
}

type NativeBenchStruct struct {
	Int64Val    int64
	StringVal   string
	BoolVal     bool
	PtrString   *string
	Nested      NativeBenchNested
	NestedPtr   *NativeBenchNested
	SliceVal    []string
	IntSlice    []int64
	NestedSlice []*NativeBenchNested
	MapVal      map[string]string
}

func newBenchmarkNativeStruct() *NativeBenchStruct {
	ptrStr := "pointer_val"
	return &NativeBenchStruct{
		Int64Val:    10,
		StringVal:   "hello",
		BoolVal:     true,
		PtrString:   &ptrStr,
		Nested:      NativeBenchNested{NestedName: "nested"},
		NestedPtr:   &NativeBenchNested{NestedName: "nested_ptr"},
		SliceVal:    []string{"first", "second", "third", "fourth", "fifth"},
		IntSlice:    []int64{1, 2, 3, 4, 5},
		NestedSlice: []*NativeBenchNested{{NestedName: "elem1"}, {NestedName: "elem2"}, {NestedName: "elem3"}},
		MapVal:      map[string]string{"key1": "val1", "key2": "val2"},
	}
}

type evalCase struct {
	category string
	name     string
	expr     string
	options  []cel.EnvOption
	in       any
}

func comprehensiveCases() []evalCase {
	return []evalCase{
		// 1. Interpreter - Arithmetic, logic, control
		{"interpreter", "arithmetic", "(x + 2) * 3 - 4 / 2", []cel.EnvOption{cel.Variable("x", cel.IntType)}, map[string]any{"x": int64(10)}},
		{"interpreter", "comparison", "x > 10", []cel.EnvOption{cel.Variable("x", cel.IntType)}, map[string]any{"x": int64(15)}},
		{"interpreter", "logic_and", "x > 10 && y < 20", []cel.EnvOption{cel.Variable("x", cel.IntType), cel.Variable("y", cel.IntType)}, map[string]any{"x": int64(15), "y": int64(10)}},
		{"interpreter", "logic_or", "x < 10 || y < 20", []cel.EnvOption{cel.Variable("x", cel.IntType), cel.Variable("y", cel.IntType)}, map[string]any{"x": int64(15), "y": int64(10)}},
		{"interpreter", "ternary", "x > 10 ? x * 2 : x / 2", []cel.EnvOption{cel.Variable("x", cel.IntType)}, map[string]any{"x": int64(15)}},
		// 2. Interpreter - Collections & lookups
		{"interpreter", "list_const_in", "x in ['a', 'b', 'c', 'd']", []cel.EnvOption{cel.Variable("x", cel.StringType)}, map[string]any{"x": "c"}},
		{"interpreter", "list_var_in", "x in list_var", []cel.EnvOption{cel.Variable("x", cel.StringType), cel.Variable("list_var", cel.ListType(cel.StringType))}, map[string]any{"x": "c", "list_var": []string{"a", "b", "c", "d"}}},
		{"interpreter", "map_lookup", "map_var['b'] == 2", []cel.EnvOption{cel.Variable("map_var", cel.MapType(cel.StringType, cel.IntType))}, map[string]any{"map_var": map[string]any{"a": int64(1), "b": int64(2), "c": int64(3)}}},
		{"interpreter", "map_has", "'b' in map_var", []cel.EnvOption{cel.Variable("map_var", cel.MapType(cel.StringType, cel.IntType))}, map[string]any{"map_var": map[string]any{"a": int64(1), "b": int64(2), "c": int64(3)}}},
		// 3. Interpreter - Strings & types
		{"interpreter", "string_concat", "str1 + ' ' + str2", []cel.EnvOption{cel.Variable("str1", cel.StringType), cel.Variable("str2", cel.StringType)}, map[string]any{"str1": "hello", "str2": "world"}},
		{"interpreter", "string_methods", "str.startsWith('hello') && str.endsWith('world')", []cel.EnvOption{cel.Variable("str", cel.StringType)}, map[string]any{"str": "hello beautiful world"}},
		{"interpreter", "type_conversion", "int('12345') + int(double('67.89')) > 1000", nil, map[string]any{}},
		{"interpreter", "timestamp_duration", "timestamp('2026-09-11T00:00:00Z') + duration('24h') == timestamp('2026-09-12T00:00:00Z')", nil, map[string]any{}},
		// 4. Interpreter - Comprehensions
		{"interpreter", "comprehension_all", "[1, 2, 3, 4, 5].all(x, x > 0)", nil, map[string]any{}},
		{"interpreter", "comprehension_all_var", "list_var.all(x, x > 0)", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType))}, map[string]any{"list_var": []int64{1, 2, 3, 4, 5}}},
		{"interpreter", "comprehension_exists", "[1, 2, 3, 4, 5].exists(x, x == 3)", nil, map[string]any{}},
		{"interpreter", "comprehension_exists_var", "list_var.exists(x, x == 3)", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType))}, map[string]any{"list_var": []int64{1, 2, 3, 4, 5}}},
		{"interpreter", "comprehension_map", "[1, 2, 3, 4, 5].map(x, x * 2)", nil, map[string]any{}},
		{"interpreter", "comprehension_map_var", "list_var.map(x, x * 2)", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType))}, map[string]any{"list_var": []int64{1, 2, 3, 4, 5}}},
		{"interpreter", "comprehension_filter", "[1, 2, 3, 4, 5].filter(x, x > 2)", nil, map[string]any{}},
		{"interpreter", "comprehension_filter_var", "list_var.filter(x, x > 2)", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType))}, map[string]any{"list_var": []int64{1, 2, 3, 4, 5}}},
		{"interpreter", "nested_comprehension", "[1, 2, 3].all(i, [1, 2, 3].exists(j, i + j == 4))", nil, map[string]any{}},
		{"interpreter", "nested_comprehension_var", "list_var.all(i, list_var.exists(j, i + j == 4))", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType))}, map[string]any{"list_var": []int64{1, 2, 3}}},

		// 5. Interpreter - Structs, Protos & Native Types
		{"interpreter", "struct_literal", "google.expr.proto3.test.TestAllTypes{single_int64: 10, single_string: 'hello'}.single_string == 'hello'", []cel.EnvOption{cel.Types(&proto3pb.TestAllTypes{})}, nil},
		{"interpreter", "struct_var_field", "msg.single_int64 == 10 && msg.single_string == 'hello'", []cel.EnvOption{cel.Types(&proto3pb.TestAllTypes{}), cel.Variable("msg", cel.ObjectType("google.expr.proto3.test.TestAllTypes"))}, map[string]any{"msg": &proto3pb.TestAllTypes{SingleInt64: 10, SingleString: "hello"}}},
		{"interpreter", "struct_var_has", "has(msg.single_nested_message)", []cel.EnvOption{cel.Types(&proto3pb.TestAllTypes{}), cel.Variable("msg", cel.ObjectType("google.expr.proto3.test.TestAllTypes"))}, map[string]any{"msg": &proto3pb.TestAllTypes{NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{}}}}},
		{"interpreter", "native_var_primitive_field", "nativeMsg.Int64Val == 10 && nativeMsg.StringVal == 'hello'", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_has_primitive", "has(nativeMsg.Int64Val) && has(nativeMsg.StringVal)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_ptr_field", "nativeMsg.PtrString == 'pointer_val'", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_has_ptr", "has(nativeMsg.PtrString)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_nested_field", "nativeMsg.Nested.NestedName == 'nested' && nativeMsg.NestedPtr.NestedName == 'nested_ptr'", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct](), reflect.TypeFor[NativeBenchNested]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_nested_has", "has(nativeMsg.Nested.NestedName) && has(nativeMsg.NestedPtr.NestedName)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct](), reflect.TypeFor[NativeBenchNested]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_slice_index", "nativeMsg.SliceVal[0] == 'first' && nativeMsg.SliceVal[1] == 'second'", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_var_map_lookup", "nativeMsg.MapVal['key1'] == 'val1'", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_all", "nativeMsg.IntSlice.all(x, x > 0)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_exists", "nativeMsg.IntSlice.exists(x, x == 3)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_map", "nativeMsg.IntSlice.map(x, x * 2)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_filter", "nativeMsg.IntSlice.filter(x, x > 2)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_nested_slice_exists", "nativeMsg.NestedSlice.exists(n, n.NestedName == 'elem2')", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct](), reflect.TypeFor[NativeBenchNested]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_nested_slice_map", "nativeMsg.NestedSlice.map(n, n.NestedName)", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct](), reflect.TypeFor[NativeBenchNested]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},
		{"interpreter", "native_comprehension_string_filter", "nativeMsg.SliceVal.filter(s, s.startsWith('f'))", []cel.EnvOption{ext.NativeTypes(reflect.TypeFor[NativeBenchStruct]()), cel.Variable("nativeMsg", cel.ObjectType("bench.NativeBenchStruct"))}, map[string]any{"nativeMsg": newBenchmarkNativeStruct()}},

		// 6. Ext - Strings
		{"ext_strings", "split", "str.split(',')", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "a,b,c,d,e"}},
		{"ext_strings", "join", "['a', 'b', 'c', 'd'].join('-')", []cel.EnvOption{ext.Strings()}, nil},
		{"ext_strings", "replace", "str.replace('world', 'CEL')", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "hello world"}},
		{"ext_strings", "trim", "str.trim()", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "   hello world   "}},
		{"ext_strings", "lower_ascii", "str.lowerAscii()", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "HeLLo WoRLd"}},
		{"ext_strings", "substring", "str.substring(0, 5)", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "hello world"}},
		{"ext_strings", "charAt", "str.charAt(4)", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "hello"}},
		{"ext_strings", "indexOf", "str.indexOf('world')", []cel.EnvOption{cel.Variable("str", cel.StringType), ext.Strings()}, map[string]any{"str": "hello world"}},

		// 7. Ext - Math
		{"ext_math", "greatest", "math.greatest(1, 9, 3, 7, 5)", []cel.EnvOption{ext.Math()}, nil},
		{"ext_math", "least", "math.least(10, 4, 8, 2, 6)", []cel.EnvOption{ext.Math()}, nil},
		{"ext_math", "bit_and", "math.bitAnd(12, 10)", []cel.EnvOption{ext.Math()}, nil},
		{"ext_math", "bit_or", "math.bitOr(12, 10)", []cel.EnvOption{ext.Math()}, nil},
		{"ext_math", "bit_xor", "math.bitXor(12, 10)", []cel.EnvOption{ext.Math()}, nil},
		{"ext_math", "bit_shift", "math.bitShiftLeft(1, 4) + math.bitShiftRight(32, 2)", []cel.EnvOption{ext.Math()}, nil},

		// 8. Ext - Sets
		{"ext_sets", "contains", "sets.contains([1, 2, 3, 4], [2, 3])", []cel.EnvOption{ext.Sets()}, nil},
		{"ext_sets", "contains_var", "sets.contains(list1, list2)", []cel.EnvOption{cel.Variable("list1", cel.ListType(cel.IntType)), cel.Variable("list2", cel.ListType(cel.IntType)), ext.Sets()}, map[string]any{"list1": []int64{1, 2, 3, 4}, "list2": []int64{2, 3}}},
		{"ext_sets", "intersects", "sets.intersects([1, 2, 3], [3, 4, 5])", []cel.EnvOption{ext.Sets()}, nil},
		{"ext_sets", "intersects_var", "sets.intersects(list1, list2)", []cel.EnvOption{cel.Variable("list1", cel.ListType(cel.IntType)), cel.Variable("list2", cel.ListType(cel.IntType)), ext.Sets()}, map[string]any{"list1": []int64{1, 2, 3}, "list2": []int64{3, 4, 5}}},
		{"ext_sets", "equivalent", "sets.equivalent([1, 2, 3], [3, 1, 2])", []cel.EnvOption{ext.Sets()}, nil},
		{"ext_sets", "equivalent_var", "sets.equivalent(list1, list2)", []cel.EnvOption{cel.Variable("list1", cel.ListType(cel.IntType)), cel.Variable("list2", cel.ListType(cel.IntType)), ext.Sets()}, map[string]any{"list1": []int64{1, 2, 3}, "list2": []int64{3, 1, 2}}},

		// 9. Ext - Lists
		{"ext_lists", "range", "lists.range(10)", []cel.EnvOption{ext.Lists()}, nil},
		{"ext_lists", "flatten", "[1, 2, [3, 4]].flatten()", []cel.EnvOption{ext.Lists()}, nil},
		{"ext_lists", "flatten_var", "list_var.flatten()", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.DynType)), ext.Lists()}, map[string]any{"list_var": []any{int64(1), int64(2), []int64{3, 4}}}},
		{"ext_lists", "slice", "[1, 2, 3, 4, 5].slice(1, 4)", []cel.EnvOption{ext.Lists()}, nil},
		{"ext_lists", "slice_var", "list_var.slice(1, 4)", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType)), ext.Lists()}, map[string]any{"list_var": []int64{1, 2, 3, 4, 5}}},
		{"ext_lists", "reverse", "[1, 2, 3, 4, 5].reverse()", []cel.EnvOption{ext.Lists()}, nil},
		{"ext_lists", "reverse_var", "list_var.reverse()", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType)), ext.Lists()}, map[string]any{"list_var": []int64{1, 2, 3, 4, 5}}},
		{"ext_lists", "sort", "[5, 3, 1, 4, 2].sort()", []cel.EnvOption{ext.Lists()}, nil},
		{"ext_lists", "sort_var", "list_var.sort()", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType)), ext.Lists()}, map[string]any{"list_var": []int64{5, 3, 1, 4, 2}}},
		{"ext_lists", "sortBy", "[-3, 1, -5, -2, 4].sortBy(e, -(e * e))", []cel.EnvOption{ext.Lists()}, nil},
		{"ext_lists", "sortBy_var", "list_var.sortBy(e, -(e * e))", []cel.EnvOption{cel.Variable("list_var", cel.ListType(cel.IntType)), ext.Lists()}, map[string]any{"list_var": []int64{-3, 1, -5, -2, 4}}},

		// 10. Ext - Encoders
		{"ext_encoders", "base64_encode", "base64.encode(b'hello world')", []cel.EnvOption{cel.OptionalTypes(), ext.Encoders()}, nil},
		{"ext_encoders", "base64_decode", "base64.decode('aGVsbG8gd29ybGQ=')", []cel.EnvOption{cel.OptionalTypes(), ext.Encoders()}, nil},

		// 11. Ext - Bindings
		{"ext_bindings", "simple", "cel.bind(x, 10, x * 2 + 5)", []cel.EnvOption{ext.Bindings()}, nil},
		{"ext_bindings", "nested", "cel.bind(x, 10, cel.bind(y, 20, x + y))", []cel.EnvOption{ext.Bindings()}, nil},

		// 12. Ext - TwoVarComprehensions
		{"ext_two_var", "all", "[10, 20, 30].all(i, x, x > i)", []cel.EnvOption{ext.TwoVarComprehensions()}, nil},
		{"ext_two_var", "all_map_var", "map_var.all(k, v, v > 0)", []cel.EnvOption{cel.Variable("map_var", cel.MapType(cel.StringType, cel.IntType)), ext.TwoVarComprehensions()}, map[string]any{"map_var": map[string]int64{"a": 1, "b": 2, "c": 3}}},
		{"ext_two_var", "exists", "{'a': 1, 'b': 2}.exists(k, v, v == 2)", []cel.EnvOption{ext.TwoVarComprehensions()}, nil},
		{"ext_two_var", "exists_map_var", "map_var.exists(k, v, v == 2)", []cel.EnvOption{cel.Variable("map_var", cel.MapType(cel.StringType, cel.IntType)), ext.TwoVarComprehensions()}, map[string]any{"map_var": map[string]int64{"a": 1, "b": 2, "c": 3}}},
	}
}

func TestComprehensiveCases(t *testing.T) {
	for _, tc := range comprehensiveCases() {
		t.Run(tc.category+"/"+tc.name, func(t *testing.T) {
			env, err := cel.NewEnv(tc.options...)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program(%q) failed: %v", tc.expr, err)
			}
			in := tc.in
			if in == nil {
				in = map[string]any{}
			}
			val, _, err := prg.Eval(in)
			if err != nil {
				t.Fatalf("prg.Eval(%q) failed: %v", tc.expr, err)
			}
			if val == nil {
				t.Fatalf("prg.Eval(%q) returned nil", tc.expr)
			}
		})
	}
}

func BenchmarkEvaluationSpeed(b *testing.B) {
	for _, tc := range comprehensiveCases() {
		env, err := cel.NewEnv(tc.options...)
		if err != nil {
			b.Fatalf("cel.NewEnv() failed: %v", err)
		}
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
		if err != nil {
			b.Fatalf("env.Program(%q) failed: %v", tc.expr, err)
		}
		in := tc.in
		if in == nil {
			in = map[string]any{}
		}
		b.Run(tc.category+"/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, err := prg.Eval(in)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCostEstimation(b *testing.B) {
	for _, tc := range comprehensiveCases() {
		env, err := cel.NewEnv(tc.options...)
		if err != nil {
			b.Fatalf("cel.NewEnv() failed: %v", err)
		}
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		b.Run(tc.category+"/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := env.EstimateCost(ast, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCostTracking(b *testing.B) {
	for _, tc := range comprehensiveCases() {
		env, err := cel.NewEnv(tc.options...)
		if err != nil {
			b.Fatalf("cel.NewEnv() failed: %v", err)
		}
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize), cel.CostTracking(nil))
		if err != nil {
			b.Fatalf("env.Program(%q) failed: %v", tc.expr, err)
		}
		in := tc.in
		if in == nil {
			in = map[string]any{}
		}
		b.Run(tc.category+"/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, err := prg.Eval(in)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
