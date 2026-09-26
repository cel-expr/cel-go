// Copyright 2023 Google LLC
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

package ext

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"

	proto2pb "cel.dev/cel-go/test/proto2pb"
	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestLists(t *testing.T) {
	listsTests := []struct {
		expr string
		err  string
	}{
		{expr: `lists.range(4) == [0,1,2,3]`},
		{expr: `lists.range(0) == []`},
		{expr: `[5,1,2,3].reverse() == [3,2,1,5]`},
		{expr: `[].reverse() == []`},
		{expr: `[1].reverse() == [1]`},
		{expr: `['are', 'you', 'as', 'bored', 'as', 'I', 'am'].reverse() == ['am', 'I', 'as', 'bored', 'as', 'you', 'are']`},
		{expr: `[false, true, true].reverse().reverse() == [false, true, true]`},
		{expr: `[1,2,3,4].slice(0, 4) == [1,2,3,4]`},
		{expr: `[1,2,3,4].slice(0, 0) == []`},
		{expr: `[1,2,3,4].slice(1, 1) == []`},
		{expr: `[1,2,3,4].slice(4, 4) == []`},
		{expr: `[1,2,3,4].slice(1, 3) == [2, 3]`},
		{expr: `[1,2,3,4].slice(3, 0)`, err: "cannot slice(3, 0), start index must be less than or equal to end index"},
		{expr: `[1,2,3,4].slice(0, 10)`, err: "cannot slice(0, 10), list is length 4"},
		{expr: `[1,2,3,4].slice(-5, 10)`, err: "cannot slice(-5, 10), negative indexes not supported"},
		{expr: `[1,2,3,4].slice(-5, -3)`, err: "cannot slice(-5, -3), negative indexes not supported"},

		{expr: `dyn([]).flatten() == []`},
		{expr: `dyn([1,2,3,4]).flatten() == [1,2,3,4]`},
		{expr: `[1,[2,[3,4]]].flatten() == [1,2,[3,4]]`},
		{expr: `[1,2,[],[],[3,4]].flatten() == [1,2,3,4]`},
		{expr: `[1,[2,[3,4]]].flatten(2) == [1,2,3,4]`},
		{expr: `[1,[2,[3,[4]]]].flatten(-1) == [1,2,3,4]`, err: "level must be non-negative"},
		{expr: `[].sort() == []`},
		{expr: `[1].sort() == [1]`},
		{expr: `[4, 3, 2, 1].sort() == [1, 2, 3, 4]`},
		{expr: `["d", "a", "b", "c"].sort() == ["a", "b", "c", "d"]`},
		{expr: `["d", 3, 2, "c"].sort() == ["a", "b", "c", "d"]`, err: "list elements must have the same type"},
		{expr: `[].sortBy(e, e) == []`},
		{expr: `["a"].sortBy(e, e) == ["a"]`},
		{expr: `[-3, 1, -5, -2, 4].sortBy(e, -(e * e)) == [-5, 4, -3, -2, 1]`},
		{expr: `[-3, 1, -5, -2, 4].map(e, e * 2).sortBy(e, -(e * e)) == [-10, 8, -6, -4, 2]`},
		{expr: `lists.range(3).sortBy(e, -e) == [2, 1, 0]`},
		{expr: `["a", "c", "b", "first"].sortBy(e, e == "first" ? "" : e) == ["first", "a", "b", "c"]`},
		{expr: `[ExampleType{name: 'foo'}, ExampleType{name: 'bar'}, ExampleType{name: 'baz'}].sortBy(e, e.name) == [ExampleType{name: 'bar'}, ExampleType{name: 'baz'}, ExampleType{name: 'foo'}]`},
		{expr: `[].distinct() == []`},
		{expr: `[1].distinct() == [1]`},
		{expr: `[-2, 5, -2, 1, 1, 5, -2, 1].distinct() == [-2, 5, 1]`},
		{expr: `['c', 'a', 'a', 'b', 'a', 'b', 'c', 'c'].distinct() == ['c', 'a', 'b']`},
		{expr: `[1, 2.0, "c", 3, "c", 1].distinct() == [1, 2.0, "c", 3]`},
		{expr: `[1, 1.0, 2].distinct() == [1, 2]`},
		{expr: `[[1], [1], [2]].distinct() == [[1], [2]]`},
		{expr: `[ExampleType{name: 'a'}, ExampleType{name: 'b'}, ExampleType{name: 'a'}].distinct() == [ExampleType{name: 'a'}, ExampleType{name: 'b'}]`},

		{expr: `[].hasOnly([])`},
		{expr: `[].hasOnly([1, 2])`},
		{expr: `![1].hasOnly([])`},
		{expr: `[1, 1, 2].hasOnly([1, 2, 3])`},
		{expr: `[1, 2, 3].hasOnly([1, 2, 3])`},
		{expr: `![1, 4].hasOnly([1, 2, 3])`},
		{expr: `[1, 2.0, 3u].hasOnly([1.0, 2u, 3])`},
		{expr: `[[1], [2, 3]].hasOnly([[2, 3], [1], [4]])`},
		{expr: `['a', 'b'].hasOnly(['b', 'a', 'c'])`},
		{expr: `[ExampleType{name: 'a'}].hasOnly([ExampleType{name: 'a'}, ExampleType{name: 'b'}])`},

		{expr: `![].hasAny([])`},
		{expr: `![].hasAny([1, 2])`},
		{expr: `![1].hasAny([])`},
		{expr: `[1, 2, 3].hasAny([3, 4])`},
		{expr: `![1, 2].hasAny([3, 4])`},
		{expr: `[1].hasAny([1u, 1.0])`},
		{expr: `[[1], [2, 3]].hasAny([[1, 2], [2, 3.0]])`},
		{expr: `['a', 'b'].hasAny(['c', 'b'])`},

		{expr: `[].hasAll([])`},
		{expr: `[1, 2, 3].hasAll([])`},
		{expr: `![].hasAll([1])`},
		{expr: `[1, 2, 3, 4].hasAll([2, 3])`},
		{expr: `[1, 2, 3].hasAll([2, 2, 2])`},
		{expr: `![1, 2, 3].hasAll([2, 4])`},
		{expr: `[1, 2.0, 3u].hasAll([1.0, 2u, 3])`},
		{expr: `[[1], [2, 3]].hasAll([[2, 3.0]])`},
		{expr: `[ExampleType{name: 'a'}, ExampleType{name: 'b'}].hasAll([ExampleType{name: 'b'}])`},

		{expr: `[].hasExactly([])`},
		{expr: `![].hasExactly([1])`},
		{expr: `![1].hasExactly([])`},
		{expr: `[1].hasExactly([1, 1])`},
		{expr: `[1, 1].hasExactly([1])`},
		{expr: `[1].hasExactly([1u, 1.0])`},
		{expr: `[1, 2, 3].hasExactly([3u, 2.0, 1])`},
		{expr: `![1, 2].hasExactly([1, 2, 3])`},
		{expr: `![1, 2, 3].hasExactly([1, 2])`},
		{expr: `[[1], [2, 3]].hasExactly([[2, 3.0], [1]])`},
		{expr: `['a', 'b', 'a'].hasExactly(['b', 'a'])`},

		{expr: `dyn(1).hasOnly([1])`, err: "no such overload: hasOnly(int, list)"},
		{expr: `[1].hasAny(dyn(1))`, err: "no such overload: hasAny(list, int)"},
		{expr: `dyn([1]).hasAll(dyn('a'))`, err: "no such overload: hasAll(list, string)"},
		{expr: `dyn({}).hasExactly([1])`, err: "no such overload: hasExactly(map, list)"},

		// Slice and reverse of protobuf typed inputs (TestAllTypes)
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}].slice(0, 3) == [TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}]`},
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}].slice(1, 3) == [TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}]`},
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}].slice(0, 0) == []`},
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}].slice(1, 1) == []`},
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}].slice(3, 3) == []`},
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 3}].reverse() == [TestAllTypes{single_int32: 3}, TestAllTypes{single_int32: 2}, TestAllTypes{single_int32: 1}]`},
		{expr: `[TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}].reverse().reverse() == [TestAllTypes{single_int32: 1}, TestAllTypes{single_int32: 2}]`},
		{expr: `[TestAllTypes{single_int32: 1}].reverse() == [TestAllTypes{single_int32: 1}]`},

		// Slice and reverse of native structs (via cel.NativeTypes)
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}].slice(0, 3) == [ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}]`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}].slice(0, 2) == [ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}]`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}].slice(1, 3) == [ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}]`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}].slice(1, 1) == []`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}].slice(3, 3) == []`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'c'}].reverse() == [ext.TestNestedType{NestedCustomName: 'c'}, ext.TestNestedType{NestedCustomName: 'b'}, ext.TestNestedType{NestedCustomName: 'a'}]`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}].reverse().reverse() == [ext.TestNestedType{NestedCustomName: 'a'}, ext.TestNestedType{NestedCustomName: 'b'}]`},
		{expr: `[ext.TestNestedType{NestedCustomName: 'a'}].reverse() == [ext.TestNestedType{NestedCustomName: 'a'}]`},
		// Slice and reverse of optional values (via cel.OptionalTypes)
		{expr: `[optional.of(1), optional.of(2), optional.none()].slice(0, 3) == [optional.of(1), optional.of(2), optional.none()]`},
		{expr: `[optional.of(1), optional.of(2), optional.none()].slice(0, 2) == [optional.of(1), optional.of(2)]`},
		{expr: `[optional.of(1), optional.of(2), optional.none()].slice(1, 3) == [optional.of(2), optional.none()]`},
		{expr: `[optional.of(1), optional.of(2), optional.none()].slice(1, 1) == []`},
		{expr: `[optional.of(1), optional.of(2), optional.none()].reverse() == [optional.none(), optional.of(2), optional.of(1)]`},
		{expr: `[optional.of(1), optional.of(2), optional.none()].reverse().reverse() == [optional.of(1), optional.of(2), optional.none()]`},
		{expr: `[optional.of(1)].reverse() == [optional.of(1)]`},
		{expr: `[optional.none()].reverse() == [optional.none()]`},
		{expr: `[optional.of('a'), optional.none(), optional.of('b')].slice(0, 2).reverse() == [optional.none(), optional.of('a')]`},

		// Slice and reverse of concatList (list + list)
		{expr: `([1, 2, 3] + [4, 5, 6]).slice(0, 6) == [1, 2, 3, 4, 5, 6]`},
		{expr: `([1, 2, 3] + [4, 5, 6]).slice(1, 5) == [2, 3, 4, 5]`},
		{expr: `([1, 2, 3] + [4, 5, 6]).slice(0, 0) == []`},
		{expr: `([1, 2, 3] + [4, 5, 6]).slice(3, 3) == []`},
		{expr: `([1, 2, 3] + [4, 5, 6]).reverse() == [6, 5, 4, 3, 2, 1]`},
		{expr: `([1, 2, 3] + [4, 5, 6]).reverse().reverse() == [1, 2, 3, 4, 5, 6]`},
		{expr: `([1, 2, 3] + [4, 5, 6]).slice(1, 5).reverse() == [5, 4, 3, 2]`},
		{expr: `([optional.of(1), optional.none()] + [optional.of(2)]).slice(1, 3) == [optional.none(), optional.of(2)]`},
		{expr: `([optional.of(1), optional.none()] + [optional.of(2)]).reverse() == [optional.of(2), optional.none(), optional.of(1)]`},
	}

	env := testListsEnv(t, 0, cel.OptionalTypes())
	for i, tst := range listsTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if tc.err != "" {
					if err == nil {
						t.Fatalf("got value %v, wanted error %s for expr: %s",
							out.Value(), tc.err, tc.expr)
					}
					if !strings.Contains(err.Error(), tc.err) {
						t.Errorf("got error %v, wanted error %s for expr: %s", err, tc.err, tc.expr)
					}
				} else if err != nil {
					t.Fatal(err)
				} else if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestListsSliceAndReverseNativeAndProto(t *testing.T) {
	env, err := cel.NewEnv(
		Lists(),
		cel.OptionalTypes(),
		cel.Types(
			&proto2pb.TestAllTypes{},
			&proto3pb.TestAllTypes{},
		),
		NativeTypes(
			reflect.TypeFor[TestNestedType](),
			reflect.TypeFor[TestAllTypes](),
		),
		cel.Variable("proto2_list", cel.ListType(cel.ObjectType("google.expr.proto2.test.TestAllTypes"))),
		cel.Variable("proto3_list", cel.ListType(cel.ObjectType("google.expr.proto3.test.TestAllTypes"))),
		cel.Variable("native_nested_list", cel.ListType(cel.ObjectType("ext.TestNestedType"))),
		cel.Variable("native_all_list", cel.ListType(cel.ObjectType("ext.TestAllTypes"))),
		cel.Variable("string_list", cel.ListType(cel.StringType)),
		cel.Variable("int_list", cel.ListType(cel.IntType)),
		cel.Variable("uint_list", cel.ListType(cel.UintType)),
		cel.Variable("double_list", cel.ListType(cel.DoubleType)),
		cel.Variable("bool_list", cel.ListType(cel.BoolType)),
		cel.Variable("bytes_list", cel.ListType(cel.BytesType)),
		cel.Variable("dyn_list", cel.ListType(cel.DynType)),
		cel.Variable("optional_int_list", cel.ListType(cel.OptionalType(cel.IntType))),
		cel.Variable("optional_string_list", cel.ListType(cel.OptionalType(cel.StringType))),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	proto2Val := []*proto2pb.TestAllTypes{
		{SingleInt32: proto.Int32(10), SingleString: proto.String("first")},
		{SingleInt32: proto.Int32(20), SingleString: proto.String("second")},
		{SingleInt32: proto.Int32(30), SingleString: proto.String("third")},
	}
	proto3Val := []*proto3pb.TestAllTypes{
		{SingleInt32: 10, SingleString: "first"},
		{SingleInt32: 20, SingleString: "second"},
		{SingleInt32: 30, SingleString: "third"},
	}
	nativeNestedPtrs := []*TestNestedType{
		{NestedCustomName: "first"},
		{NestedCustomName: "second"},
		{NestedCustomName: "third"},
	}
	nativeNestedValues := []TestNestedType{
		{NestedCustomName: "first"},
		{NestedCustomName: "second"},
		{NestedCustomName: "third"},
	}
	nativeAllPtrs := []*TestAllTypes{
		{Int32Val: 10, StringVal: "first"},
		{Int32Val: 20, StringVal: "second"},
		{Int32Val: 30, StringVal: "third"},
	}
	nativeAllValues := []TestAllTypes{
		{Int32Val: 10, StringVal: "first"},
		{Int32Val: 20, StringVal: "second"},
		{Int32Val: 30, StringVal: "third"},
	}
	stringVal := []string{"first", "second", "third"}
	intVal := []int64{10, 20, 30}
	uintVal := []uint64{10, 20, 30}
	doubleVal := []float64{1.5, 2.5, 3.5}
	boolVal := []bool{true, false, true}
	bytesVal := [][]byte{[]byte("first"), []byte("second"), []byte("third")}
	dynVal := []any{"first", int64(20), true}
	optionalIntVal := []any{types.OptionalOf(types.Int(10)), types.OptionalOf(types.Int(20)), types.OptionalNone}
	optionalStringVal := []any{types.OptionalOf(types.String("first")), types.OptionalNone, types.OptionalOf(types.String("third"))}

	tests := []struct {
		name string
		expr string
		vars map[string]any
		err  string
	}{
		// Protobuf v2
		{name: "proto2_slice_all", expr: `proto2_list.slice(0, 3).size() == 3`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_slice_sub", expr: `proto2_list.slice(1, 3).size() == 2 && proto2_list.slice(1, 3)[0].single_int32 == 20 && proto2_list.slice(1, 3)[1].single_int32 == 30`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_slice_empty_start", expr: `proto2_list.slice(0, 0) == []`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_slice_empty_mid", expr: `proto2_list.slice(1, 1) == []`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_slice_empty_end", expr: `proto2_list.slice(3, 3) == []`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_reverse", expr: `proto2_list.reverse()[0].single_int32 == 30 && proto2_list.reverse()[2].single_int32 == 10`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_reverse_double", expr: `proto2_list.reverse().reverse() == proto2_list`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_slice_and_reverse", expr: `proto2_list.slice(0, 2).reverse()[0].single_int32 == 20`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "proto2_slice_invalid_order", expr: `proto2_list.slice(2, 1)`, vars: map[string]any{"proto2_list": proto2Val}, err: "start index must be less than or equal to end index"},
		{name: "proto2_slice_out_of_bounds", expr: `proto2_list.slice(0, 4)`, vars: map[string]any{"proto2_list": proto2Val}, err: "list is length 3"},
		{name: "proto2_slice_negative", expr: `proto2_list.slice(-1, 2)`, vars: map[string]any{"proto2_list": proto2Val}, err: "negative indexes not supported"},

		// Protobuf v3
		{name: "proto3_slice_all", expr: `proto3_list.slice(0, 3).size() == 3`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_slice_sub", expr: `proto3_list.slice(1, 3).size() == 2 && proto3_list.slice(1, 3)[0].single_int32 == 20 && proto3_list.slice(1, 3)[1].single_int32 == 30`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_slice_empty_start", expr: `proto3_list.slice(0, 0) == []`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_slice_empty_mid", expr: `proto3_list.slice(1, 1) == []`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_slice_empty_end", expr: `proto3_list.slice(3, 3) == []`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_reverse", expr: `proto3_list.reverse()[0].single_int32 == 30 && proto3_list.reverse()[2].single_int32 == 10`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_reverse_double", expr: `proto3_list.reverse().reverse() == proto3_list`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_slice_and_reverse", expr: `proto3_list.slice(0, 2).reverse()[0].single_int32 == 20`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "proto3_slice_invalid_order", expr: `proto3_list.slice(2, 1)`, vars: map[string]any{"proto3_list": proto3Val}, err: "start index must be less than or equal to end index"},
		{name: "proto3_slice_out_of_bounds", expr: `proto3_list.slice(0, 4)`, vars: map[string]any{"proto3_list": proto3Val}, err: "list is length 3"},
		{name: "proto3_slice_negative", expr: `proto3_list.slice(-1, 2)`, vars: map[string]any{"proto3_list": proto3Val}, err: "negative indexes not supported"},

		// Native struct pointers (TestNestedType)
		{name: "native_nested_ptrs_slice_all", expr: `native_nested_list.slice(0, 3).size() == 3`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "native_nested_ptrs_slice_sub", expr: `native_nested_list.slice(1, 3).size() == 2 && native_nested_list.slice(1, 3)[0].NestedCustomName == 'second' && native_nested_list.slice(1, 3)[1].NestedCustomName == 'third'`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "native_nested_ptrs_slice_empty", expr: `native_nested_list.slice(1, 1) == []`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "native_nested_ptrs_reverse", expr: `native_nested_list.reverse()[0].NestedCustomName == 'third' && native_nested_list.reverse()[2].NestedCustomName == 'first'`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "native_nested_ptrs_reverse_double", expr: `native_nested_list.reverse().reverse() == native_nested_list`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "native_nested_ptrs_slice_and_reverse", expr: `native_nested_list.slice(0, 2).reverse()[0].NestedCustomName == 'second'`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "native_nested_ptrs_slice_invalid_order", expr: `native_nested_list.slice(2, 1)`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}, err: "start index must be less than or equal to end index"},
		{name: "native_nested_ptrs_slice_out_of_bounds", expr: `native_nested_list.slice(0, 4)`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}, err: "list is length 3"},
		{name: "native_nested_ptrs_slice_negative", expr: `native_nested_list.slice(-1, 2)`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}, err: "negative indexes not supported"},

		// Native struct values (TestNestedType)
		{name: "native_nested_values_slice_all", expr: `native_nested_list.slice(0, 3).size() == 3`, vars: map[string]any{"native_nested_list": nativeNestedValues}},
		{name: "native_nested_values_slice_sub", expr: `native_nested_list.slice(1, 3).size() == 2 && native_nested_list.slice(1, 3)[0].NestedCustomName == 'second' && native_nested_list.slice(1, 3)[1].NestedCustomName == 'third'`, vars: map[string]any{"native_nested_list": nativeNestedValues}},
		{name: "native_nested_values_slice_empty", expr: `native_nested_list.slice(1, 1) == []`, vars: map[string]any{"native_nested_list": nativeNestedValues}},
		{name: "native_nested_values_reverse", expr: `native_nested_list.reverse()[0].NestedCustomName == 'third' && native_nested_list.reverse()[2].NestedCustomName == 'first'`, vars: map[string]any{"native_nested_list": nativeNestedValues}},
		{name: "native_nested_values_reverse_double", expr: `native_nested_list.reverse().reverse() == native_nested_list`, vars: map[string]any{"native_nested_list": nativeNestedValues}},
		{name: "native_nested_values_slice_and_reverse", expr: `native_nested_list.slice(0, 2).reverse()[0].NestedCustomName == 'second'`, vars: map[string]any{"native_nested_list": nativeNestedValues}},

		// Native struct pointers (TestAllTypes)
		{name: "native_all_ptrs_slice_all", expr: `native_all_list.slice(0, 3).size() == 3`, vars: map[string]any{"native_all_list": nativeAllPtrs}},
		{name: "native_all_ptrs_slice_sub", expr: `native_all_list.slice(1, 3).size() == 2 && native_all_list.slice(1, 3)[0].Int32Val == 20 && native_all_list.slice(1, 3)[1].Int32Val == 30`, vars: map[string]any{"native_all_list": nativeAllPtrs}},
		{name: "native_all_ptrs_slice_empty", expr: `native_all_list.slice(0, 0) == []`, vars: map[string]any{"native_all_list": nativeAllPtrs}},
		{name: "native_all_ptrs_reverse", expr: `native_all_list.reverse()[0].Int32Val == 30 && native_all_list.reverse()[2].Int32Val == 10`, vars: map[string]any{"native_all_list": nativeAllPtrs}},
		{name: "native_all_ptrs_reverse_double", expr: `native_all_list.reverse().reverse() == native_all_list`, vars: map[string]any{"native_all_list": nativeAllPtrs}},
		{name: "native_all_ptrs_slice_and_reverse", expr: `native_all_list.slice(0, 2).reverse()[0].Int32Val == 20`, vars: map[string]any{"native_all_list": nativeAllPtrs}},

		// Native struct values (TestAllTypes)
		{name: "native_all_values_slice_all", expr: `native_all_list.slice(0, 3).size() == 3`, vars: map[string]any{"native_all_list": nativeAllValues}},
		{name: "native_all_values_slice_sub", expr: `native_all_list.slice(1, 3).size() == 2 && native_all_list.slice(1, 3)[0].Int32Val == 20 && native_all_list.slice(1, 3)[1].Int32Val == 30`, vars: map[string]any{"native_all_list": nativeAllValues}},
		{name: "native_all_values_slice_empty", expr: `native_all_list.slice(2, 2) == []`, vars: map[string]any{"native_all_list": nativeAllValues}},
		{name: "native_all_values_reverse", expr: `native_all_list.reverse()[0].Int32Val == 30 && native_all_list.reverse()[2].Int32Val == 10`, vars: map[string]any{"native_all_list": nativeAllValues}},
		{name: "native_all_values_reverse_double", expr: `native_all_list.reverse().reverse() == native_all_list`, vars: map[string]any{"native_all_list": nativeAllValues}},
		{name: "native_all_values_slice_and_reverse", expr: `native_all_list.slice(0, 2).reverse()[0].Int32Val == 20`, vars: map[string]any{"native_all_list": nativeAllValues}},

		// Concrete primitive types: string, int, uint, double, bool, bytes, dyn
		{name: "string_list_slice", expr: `string_list.slice(1, 3) == ['second', 'third']`, vars: map[string]any{"string_list": stringVal}},
		{name: "string_list_reverse", expr: `string_list.reverse() == ['third', 'second', 'first']`, vars: map[string]any{"string_list": stringVal}},
		{name: "string_list_reverse_double", expr: `string_list.reverse().reverse() == string_list`, vars: map[string]any{"string_list": stringVal}},

		{name: "int_list_slice", expr: `int_list.slice(0, 2) == [10, 20]`, vars: map[string]any{"int_list": intVal}},
		{name: "int_list_reverse", expr: `int_list.reverse() == [30, 20, 10]`, vars: map[string]any{"int_list": intVal}},
		{name: "int_list_reverse_double", expr: `int_list.reverse().reverse() == int_list`, vars: map[string]any{"int_list": intVal}},

		{name: "uint_list_slice", expr: `uint_list.slice(1, 3) == [20u, 30u]`, vars: map[string]any{"uint_list": uintVal}},
		{name: "uint_list_reverse", expr: `uint_list.reverse() == [30u, 20u, 10u]`, vars: map[string]any{"uint_list": uintVal}},
		{name: "uint_list_reverse_double", expr: `uint_list.reverse().reverse() == uint_list`, vars: map[string]any{"uint_list": uintVal}},

		{name: "double_list_slice", expr: `double_list.slice(0, 2) == [1.5, 2.5]`, vars: map[string]any{"double_list": doubleVal}},
		{name: "double_list_reverse", expr: `double_list.reverse() == [3.5, 2.5, 1.5]`, vars: map[string]any{"double_list": doubleVal}},

		{name: "bool_list_slice", expr: `bool_list.slice(0, 2) == [true, false]`, vars: map[string]any{"bool_list": boolVal}},
		{name: "bool_list_reverse", expr: `bool_list.reverse() == [true, false, true]`, vars: map[string]any{"bool_list": boolVal}},

		{name: "bytes_list_slice", expr: `bytes_list.slice(0, 2) == [b'first', b'second']`, vars: map[string]any{"bytes_list": bytesVal}},
		{name: "bytes_list_reverse", expr: `bytes_list.reverse() == [b'third', b'second', b'first']`, vars: map[string]any{"bytes_list": bytesVal}},

		{name: "dyn_list_slice", expr: `dyn_list.slice(1, 3) == [20, true]`, vars: map[string]any{"dyn_list": dynVal}},
		{name: "dyn_list_reverse", expr: `dyn_list.reverse() == [true, 20, 'first']`, vars: map[string]any{"dyn_list": dynVal}},

		// Optional values within lists (variables)
		{name: "optional_int_slice_all", expr: `optional_int_list.slice(0, 3).size() == 3`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_slice_sub", expr: `optional_int_list.slice(0, 2) == [optional.of(10), optional.of(20)]`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_slice_none", expr: `optional_int_list.slice(1, 3) == [optional.of(20), optional.none()]`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_slice_empty", expr: `optional_int_list.slice(1, 1) == []`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_reverse", expr: `optional_int_list.reverse() == [optional.none(), optional.of(20), optional.of(10)]`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_reverse_double", expr: `optional_int_list.reverse().reverse() == optional_int_list`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_slice_and_reverse", expr: `optional_int_list.slice(0, 2).reverse() == [optional.of(20), optional.of(10)]`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_int_reverse_value_access", expr: `optional_int_list.slice(0, 2).reverse()[0].value() == 20 && !optional_int_list.reverse()[0].hasValue()`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "optional_string_reverse", expr: `optional_string_list.reverse() == [optional.of('third'), optional.none(), optional.of('first')]`, vars: map[string]any{"optional_string_list": optionalStringVal}},
		{name: "optional_int_slice_invalid_order", expr: `optional_int_list.slice(2, 1)`, vars: map[string]any{"optional_int_list": optionalIntVal}, err: "start index must be less than or equal to end index"},
		{name: "optional_int_slice_out_of_bounds", expr: `optional_int_list.slice(0, 4)`, vars: map[string]any{"optional_int_list": optionalIntVal}, err: "list is length 3"},

		// ConcatList cases generated by list + list
		{name: "concat_int_slice_all", expr: `(int_list + int_list).slice(0, 6).size() == 6`, vars: map[string]any{"int_list": intVal}},
		{name: "concat_int_slice_sub", expr: `(int_list + int_list).slice(1, 5) == [20, 30, 10, 20]`, vars: map[string]any{"int_list": intVal}},
		{name: "concat_int_slice_empty", expr: `(int_list + int_list).slice(2, 2) == []`, vars: map[string]any{"int_list": intVal}},
		{name: "concat_int_reverse", expr: `(int_list + int_list).reverse() == [30, 20, 10, 30, 20, 10]`, vars: map[string]any{"int_list": intVal}},
		{name: "concat_int_reverse_double", expr: `(int_list + int_list).reverse().reverse() == int_list + int_list`, vars: map[string]any{"int_list": intVal}},
		{name: "concat_int_slice_and_reverse", expr: `(int_list + int_list).slice(1, 5).reverse() == [20, 10, 30, 20]`, vars: map[string]any{"int_list": intVal}},

		{name: "concat_string_slice", expr: `(string_list + string_list).slice(2, 4) == ['third', 'first']`, vars: map[string]any{"string_list": stringVal}},
		{name: "concat_string_reverse", expr: `(string_list + string_list).reverse() == ['third', 'second', 'first', 'third', 'second', 'first']`, vars: map[string]any{"string_list": stringVal}},

		{name: "concat_proto2_slice", expr: `(proto2_list + proto2_list).slice(1, 5).size() == 4 && (proto2_list + proto2_list).slice(1, 5)[0].single_int32 == 20`, vars: map[string]any{"proto2_list": proto2Val}},
		{name: "concat_proto2_reverse", expr: `(proto2_list + proto2_list).reverse().size() == 6 && (proto2_list + proto2_list).reverse()[0].single_int32 == 30`, vars: map[string]any{"proto2_list": proto2Val}},

		{name: "concat_proto3_slice", expr: `(proto3_list + proto3_list).slice(1, 5).size() == 4 && (proto3_list + proto3_list).slice(1, 5)[0].single_int32 == 20`, vars: map[string]any{"proto3_list": proto3Val}},
		{name: "concat_proto3_reverse", expr: `(proto3_list + proto3_list).reverse().size() == 6 && (proto3_list + proto3_list).reverse()[0].single_int32 == 30`, vars: map[string]any{"proto3_list": proto3Val}},

		{name: "concat_native_nested_slice", expr: `(native_nested_list + native_nested_list).slice(1, 4).size() == 3 && (native_nested_list + native_nested_list).slice(1, 4)[0].NestedCustomName == 'second'`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},
		{name: "concat_native_nested_reverse", expr: `(native_nested_list + native_nested_list).reverse()[0].NestedCustomName == 'third'`, vars: map[string]any{"native_nested_list": nativeNestedPtrs}},

		{name: "concat_optional_int_slice", expr: `(optional_int_list + optional_int_list).slice(1, 4) == [optional.of(20), optional.none(), optional.of(10)]`, vars: map[string]any{"optional_int_list": optionalIntVal}},
		{name: "concat_optional_int_reverse", expr: `(optional_int_list + optional_int_list).reverse() == [optional.none(), optional.of(20), optional.of(10), optional.none(), optional.of(20), optional.of(10)]`, vars: map[string]any{"optional_int_list": optionalIntVal}},

		{name: "nested_concat_int_slice", expr: `((int_list + int_list) + int_list).slice(2, 7) == [30, 10, 20, 30, 10]`, vars: map[string]any{"int_list": intVal}},
		{name: "nested_concat_int_reverse", expr: `((int_list + int_list) + int_list).reverse().size() == 9`, vars: map[string]any{"int_list": intVal}},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.vars)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("got %v, wanted error %s for expr %s", out.Value(), tc.err, tc.expr)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Errorf("got error %v, wanted %s", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			} else if out.Value() != true {
				t.Errorf("got %v, wanted true for expr %s", out.Value(), tc.expr)
			}
		})
	}
}

func TestListsRuntimeErrors(t *testing.T) {
	env, err := cel.NewEnv(Lists(ListsVersion(1)))
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	listsTests := []struct {
		expr string
		err  string
	}{
		{
			expr: "dyn({}).flatten()",
			err:  "no such overload",
		},
		{
			expr: "dyn({}).flatten(0)",
			err:  "no such overload",
		},
		{
			expr: "[].flatten(-1)",
			err:  "level must be non-negative",
		},
		{
			expr: "[].flatten(dyn('1'))",
			err:  "no such overload",
		},
	}
	for i, tst := range listsTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			_, _, err = prg.Eval(cel.NoVars())
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.err)
			}
		})
	}
}

func TestListsVersion(t *testing.T) {
	versionCases := []struct {
		version            uint32
		supportedFunctions map[string]string
	}{
		{
			version: 0,
			supportedFunctions: map[string]string{
				"slice": "[1, 2, 3, 4, 5].slice(2, 4) == [3, 4]",
			},
		},
		{
			version: 1,
			supportedFunctions: map[string]string{
				"flatten": "[[1, 2], [3, 4]].flatten() == [1, 2, 3, 4]",
			},
		},
		{
			version: 2,
			supportedFunctions: map[string]string{
				"distinct": "[1, 2, 2, 1].distinct() == [1, 2]",
				"range":    "lists.range(5) == [0, 1, 2, 3, 4]",
				"reverse":  "[1, 2, 3].reverse() == [3, 2, 1]",
				"sort":     "[2, 1, 3].sort() == [1, 2, 3]",
				"sortBy":   "[{'field': 'lo'}, {'field': 'hi'}].sortBy(m, m.field) == [{'field': 'hi'}, {'field': 'lo'}]",
			},
		},
		{
			// Versions 3 and 4 only introduce cost support for existing functions, but they are
			// declared here to assert that later function additions are not visible to them.
			version:            3,
			supportedFunctions: map[string]string{},
		},
		{
			version:            4,
			supportedFunctions: map[string]string{},
		},
		{
			version: 5,
			supportedFunctions: map[string]string{
				"hasOnly":    "[1, 2].hasOnly([1, 2, 3])",
				"hasAny":     "[1, 2].hasAny([2, 3])",
				"hasAll":     "[1, 2, 3].hasAll([1, 2])",
				"hasExactly": "[1, 2].hasExactly([2, 1, 1])",
			},
		},
	}
	for _, lib := range versionCases {
		env, err := cel.NewEnv(Lists(ListsVersion(lib.version)))
		if err != nil {
			t.Fatalf("cel.NewEnv(Lists(ListsVersion(%d))) failed: %v", lib.version, err)
		}
		t.Run(fmt.Sprintf("version=%d", lib.version), func(t *testing.T) {
			for _, tc := range versionCases {
				for name, expr := range tc.supportedFunctions {
					supported := lib.version >= tc.version
					t.Run(fmt.Sprintf("%s-supported=%t", name, supported), func(t *testing.T) {
						ast, iss := env.Compile(expr)
						if supported {
							if iss.Err() != nil {
								t.Errorf("unexpected error: %v", iss.Err())
							}
						} else {
							if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "undeclared reference") {
								t.Errorf("got error %v, wanted error %s for expr: %s, version: %d", iss.Err(), "undeclared reference", expr, tc.version)
							}
							return
						}
						prg, err := env.Program(ast)
						if err != nil {
							t.Fatalf("env.Program() failed: %v", err)
						}
						out, _, err := prg.Eval(cel.NoVars())
						if err != nil {
							t.Fatalf("prg.Eval() failed: %v", err)
						}
						if out != types.True {
							t.Errorf("prg.Eval() got %v, wanted true", out)
						}
					})
				}
			}
		})
	}
}

func TestListsCosts(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost cost.CostEstimate
		// estimatedCostV0 is the estimate under cost.0, set only where the revision moved it.
		estimatedCostV0 *cost.CostEstimate
		actualCost      uint64
		version         int
	}{
		{
			// (1 array alloc + internal alloc) * 10
			// + size(list)
			// + 2 calls
			name:          "list_range",
			expr:          `lists.range(4) == [0, 1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(26),
			actualCost:    26,
		},
		{
			name:          "list_range_computed",
			expr:          `lists.range(4 / 2) == [0, 1]`,
			estimatedCost: cost.FixedCostEstimate(math.MaxUint64),
			actualCost:    25,
		},
		{
			name:          "list_range_var",
			expr:          `lists.range(x) == [0, 1, 2, 3, 4]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.IntType)},
			in:            map[string]any{"x": 5},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: cost.FixedCostEstimate(math.MaxUint64),
			actualCost:    28,
		},
		{
			// (3 array allocs + internal alloc) * 10 + size(list) + 2 calls
			name:          "list_flatten_depth_one",
			expr:          `[[1, 2], 3].flatten(1) == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(44),
			actualCost:    44,
			version:       3,
		},
		{
			name:          "list_flatten_depth_one_v4",
			expr:          `[[1, 2], 3].flatten(1) == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			// (3 array allocs + internal alloc) * 10 + size(list) * 2 + 2 calls
			name:          "list_flatten_depth_two",
			expr:          `[[1, 2], 3].flatten(2) == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(46),
			actualCost:    46,
			version:       3,
		},
		{
			name:          "list_flatten_depth_two_v4",
			expr:          `[[1, 2], 3].flatten(2) == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			// (3 array allocs + internal alloc) * 10 + size(list) * 3 + 2 calls
			name:          "list_flatten_depth_three",
			expr:          `[[1, 2], 3].flatten(3) == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(48),
			actualCost:    48,
			version:       3,
		},
		{
			name:          "list_flatten_depth_three_v4",
			expr:          `[[1, 2], 3].flatten(3) == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			name:          "list_flatten",
			expr:          `[[1], 2, 3].flatten() == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(45),
			actualCost:    45,
			version:       3,
		},
		{
			name:          "list_flatten_v4",
			expr:          `[[1], 2, 3].flatten() == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			name:            "list_flatten_var",
			expr:            `x.flatten() == [1, 2, 3]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:              map[string]any{"x": []any{[]any{1}, 2, 3}},
			hints:           map[string]uint64{"x": 3},
			estimatedCost:   cost.RangedCostEstimate(22, 26),
			estimatedCostV0: costV0(23, 26),
			actualCost:      26,
			version:         3,
		},
		{
			name:            "list_flatten_var_v4_no_item_hint",
			expr:            `x.flatten() == [1, 2, 3]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:              map[string]any{"x": []any{[]any{1}, 2, 3}},
			hints:           map[string]uint64{"x": 3},
			estimatedCost:   cost.RangedCostEstimate(22, math.MaxUint64),
			estimatedCostV0: costV0(23, math.MaxUint64),
			actualCost:      26,
		},
		{
			name:            "list_flatten_var_v4_with_item_hint",
			expr:            `x.flatten() == [1, 2, 3]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:              map[string]any{"x": []any{[]any{1}, 2, 3}},
			hints:           map[string]uint64{"x": 3, "x.@items": 5},
			estimatedCost:   cost.RangedCostEstimate(22, 38),
			estimatedCostV0: costV0(23, 38),
			actualCost:      26,
		},
		{
			name:          "list_flatten_depth_var",
			expr:          `[[1, 2], 3].flatten(x) == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.IntType)},
			in:            map[string]any{"x": 5},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: cost.FixedCostEstimate(math.MaxUint64),
			actualCost:    53,
			version:       3,
		},
		{
			name:          "list_flatten_depth_var_v4",
			expr:          `[[1, 2], 3].flatten(x) == [1, 2, 3]`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.IntType)},
			in:            map[string]any{"x": 5},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: cost.FixedCostEstimate(46),
			actualCost:    46,
		},
		{
			// (2 array allocs + 1 internal) * 10
			// + size(list) * size(list) * 2
			// + 2 calls
			name:          "list_distinct_worst_case",
			expr:          `[1, 2, 3].distinct() == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(50),
			actualCost:    50,
		},
		{
			// (2 array allocs + 1 internal) * 10
			// + size(list) * size(list) * 2
			// + 2 calls
			name:          "list_distinct_best_case",
			expr:          `[1, 1, 1].distinct() == [1]`,
			estimatedCost: cost.FixedCostEstimate(50),
			actualCost:    50,
		},
		{
			// (1 array alloc + 1 internal) * 20
			// + [0, size(x) * size(x)] * 2 --> [0, 18]
			// + 2 calls
			// + 1 ident lookup
			name:            "list_distinct_var",
			expr:            `x.distinct() == ['hello']`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"hello", "hello"}},
			hints:           map[string]uint64{"x": 3, "x.@items": 5},
			estimatedCost:   cost.RangedCostEstimate(22, 41),
			estimatedCostV0: costV0(23, 41),
			actualCost:      31,
		},
		{
			// allocs: (2 + one internal) * 10
			// max_slice cost: 5
			// lookups: 2
			// calls: 2
			name: "list_slice_var_range",
			expr: `[1, 2, 3, 4, 5].slice(x, y) == [2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.IntType),
				cel.Variable("y", cel.IntType),
			},
			in:            map[string]any{"x": 1, "y": 3},
			estimatedCost: cost.FixedCostEstimate(39),
			actualCost:    36,
		},
		{
			// allocs: (1 + one internal) * 10
			// max_slice cost: 2
			// lookups: 1
			// calls: 2
			name: "list_slice_var_list",
			expr: `z.slice(1, 3) == [2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.IntType),
				cel.Variable("y", cel.IntType),
				cel.Variable("z", cel.ListType(cel.IntType)),
			},
			in:            map[string]any{"x": 1, "y": 3, "z": []int{1, 2, 3, 4, 5, 6, 7}},
			hints:         map[string]uint64{"z": 10},
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			// allocs: (1 + one internal) * 10
			// max_slice cost: 10
			// lookups: 3
			// calls: 2
			name: "list_slice_var_list_var_range",
			expr: `z.slice(x, y) == [2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.IntType),
				cel.Variable("y", cel.IntType),
				cel.Variable("z", cel.ListType(cel.IntType)),
			},
			in:            map[string]any{"x": 1, "y": 3, "z": []int{1, 2, 3, 4, 5, 6, 7}},
			hints:         map[string]uint64{"z": 10},
			estimatedCost: cost.FixedCostEstimate(35),
			actualCost:    27,
		},
		{
			name:          "list_slice",
			expr:          `[1, 2, 3].slice(1, 3) == [2, 3]`,
			estimatedCost: cost.FixedCostEstimate(34),
			actualCost:    34,
		},
		{
			// allocs: (2 + one internal) * 10
			// reverse cost: 3
			// calls: 2
			name:          "list_reverse",
			expr:          `[3, 2, 1].reverse() == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(35),
			actualCost:    35,
		},
		{
			// allocs: (1 + one internal) * 10
			// reverse cost: 5
			// lookups: 1
			// calls: 2
			name: "list_var_reverse",
			expr: `x.reverse() == [1, 2, 3]`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
			},
			in:              map[string]any{"x": []int{3, 2, 1}},
			hints:           map[string]uint64{"x": 5},
			estimatedCost:   cost.RangedCostEstimate(22, 28),
			estimatedCostV0: costV0(23, 28),
			actualCost:      26,
		},
		{
			// (2 allocs + 1 internal) * 10
			// + size(list) * size(list) * 2
			// + 2 calls
			name:          "list_sort",
			expr:          `[2, 3, 1].sort() == [1, 2, 3]`,
			estimatedCost: cost.FixedCostEstimate(50),
			actualCost:    50,
		},
		{
			// (1 allocs + 1 internal) * 10
			// + [0, size(x) * size(x)] * 2 --> [0, 50]
			// + 2 calls
			// + 1 ident lookup
			name:            "list_sort_var",
			expr:            `x.sort() == [1, 2, 3]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
			in:              map[string]any{"x": []int{3, 2, 1}},
			hints:           map[string]uint64{"x": 5},
			estimatedCost:   cost.RangedCostEstimate(22, 73),
			estimatedCostV0: costV0(23, 73),
			actualCost:      41,
		},
		{
			name:            "list_sort_var_string",
			expr:            `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:           map[string]uint64{"x": 10, "x.@items": 1},
			estimatedCost:   cost.RangedCostEstimate(22, 223),
			estimatedCostV0: costV0(23, 223),
			actualCost:      98,
		},
		{
			name:          "list_sort_var_int_empty",
			expr:          `x.sort() == []`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
			in:            map[string]any{"x": []int{}},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: cost.RangedCostEstimate(22, 222),
			actualCost:    22,
		},
		{
			name:          "list_sortBy",
			expr:          `[{'x':4}, {'x':3}].sortBy(m, m['x']) == [{'x':3}, {'x':4}]`,
			estimatedCost: cost.FixedCostEstimate(211),
			actualCost:    211,
		},
		{
			name:            "list_sortBy_var",
			expr:            `x.sortBy(m, m['x']) == [{'x':3}, {'x':4}]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.DynType))},
			in:              map[string]any{"x": []any{map[string]any{"x": 4}, map[string]any{"x": 3}}},
			hints:           map[string]uint64{"x": 5},
			estimatedCost:   cost.RangedCostEstimate(105, 226),
			estimatedCostV0: costV0(106, 226),
			actualCost:      142,
		},
		{
			name:            "list_sortBy_var_string",
			expr:            `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:              map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:           map[string]uint64{"x": 3, "x.@items": 1},
			estimatedCost:   cost.RangedCostEstimate(135, 196),
			estimatedCostV0: costV0(136, 196),
			actualCost:      196,
		},
		{
			name: "list_sort_concat_cost",
			expr: `(x.sort() + y).size() == 20`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.ListType(cel.IntType)),
			},
			in: map[string]any{
				"x": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				"y": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			hints: map[string]uint64{
				"x": 10,
				"y": 10,
			},
			estimatedCost: cost.RangedCostEstimate(16, 216),
			actualCost:    216,
		},
		{
			name: "list_distinct_concat_cost_with_item_hint",
			expr: `(x.distinct() + y).size() == 20`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.ListType(cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				"y": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
			hints: map[string]uint64{
				"x":        10,
				"x.@items": 1,
				"y":        10,
			},
			estimatedCost: cost.RangedCostEstimate(16, 216),
			actualCost:    226,
		},
		{
			name: "list_distinct_concat_cost",
			expr: `(x.distinct() + y).size() == 20`,
			vars: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.ListType(cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				"y": []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
			hints: map[string]uint64{
				"x": 10,
				"y": 10,
			},
			estimatedCost: cost.RangedCostEstimate(16, math.MaxUint64),
			actualCost:    226,
		},
		{
			name:            "list_distinct_var_v3",
			expr:            `x.distinct() == ['hello']`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"hello", "hello"}},
			hints:           map[string]uint64{"x": 3},
			estimatedCost:   cost.RangedCostEstimate(22, 42),
			estimatedCostV0: costV0(23, 42),
			actualCost:      31,
			version:         3,
		},
		{
			name:            "list_distinct_var_v4_no_item_hint",
			expr:            `x.distinct() == ['hello']`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"hello", "hello"}},
			hints:           map[string]uint64{"x": 3},
			estimatedCost:   cost.RangedCostEstimate(22, math.MaxUint64),
			estimatedCostV0: costV0(23, math.MaxUint64),
			actualCost:      31,
		},
		{
			name:            "list_distinct_var_v4_with_item_hint",
			expr:            `x.distinct() == ['hello']`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"hello", "hello"}},
			hints:           map[string]uint64{"x": 3, "x.@items": 5},
			estimatedCost:   cost.RangedCostEstimate(22, 41),
			estimatedCostV0: costV0(23, 41),
			actualCost:      31,
		},
		{
			name:            "list_sort_var_string_v3",
			expr:            `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:           map[string]uint64{"x": 10},
			estimatedCost:   cost.RangedCostEstimate(22, 233),
			estimatedCostV0: costV0(23, 233),
			actualCost:      98,
			version:         3,
		},
		{
			name:            "list_sort_var_string_v4_no_item_hint",
			expr:            `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:           map[string]uint64{"x": 10},
			estimatedCost:   cost.RangedCostEstimate(22, math.MaxUint64),
			estimatedCostV0: costV0(23, math.MaxUint64),
			actualCost:      98,
		},
		{
			name:            "list_sort_var_string_v4_with_item_hint",
			expr:            `x.sort() == ["a", "a", "b", "b", "c", "c"]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:              map[string]any{"x": []string{"b", "a", "b", "a", "c", "c"}},
			hints:           map[string]uint64{"x": 10, "x.@items": 1},
			estimatedCost:   cost.RangedCostEstimate(22, 223),
			estimatedCostV0: costV0(23, 223),
			actualCost:      98,
		},
		{
			name:            "list_sortBy_var_string_v3",
			expr:            `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:              map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:           map[string]uint64{"x": 3},
			estimatedCost:   cost.RangedCostEstimate(135, 197),
			estimatedCostV0: costV0(136, 197),
			actualCost:      196,
			version:         3,
		},
		{
			name:            "list_sortBy_var_string_v4_no_item_hint",
			expr:            `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:              map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:           map[string]uint64{"x": 3},
			estimatedCost:   cost.RangedCostEstimate(135, math.MaxUint64),
			estimatedCostV0: costV0(136, math.MaxUint64),
			actualCost:      196,
		},
		{
			name:            "list_sortBy_var_string_v4_with_item_hint",
			expr:            `x.sortBy(m, m['x']) == [{'x': 'a'}, {'x': 'b'}, {'x': 'c'}]`,
			vars:            []cel.EnvOption{cel.Variable("x", cel.ListType(cel.MapType(cel.StringType, cel.StringType)))},
			in:              map[string]any{"x": []any{map[string]any{"x": "b"}, map[string]any{"x": "c"}, map[string]any{"x": "a"}}},
			hints:           map[string]uint64{"x": 3, "x.@items": 1},
			estimatedCost:   cost.RangedCostEstimate(135, 196),
			estimatedCostV0: costV0(136, 196),
			actualCost:      196,
		},
		{
			name:          "list_hasOnly",
			expr:          `[1, 2].hasOnly([1, 2, 3])`,
			estimatedCost: cost.FixedCostEstimate(27),
			actualCost:    27,
		},
		{
			name:          "list_hasAny",
			expr:          `[1, 2].hasAny([2, 3])`,
			estimatedCost: cost.FixedCostEstimate(25),
			actualCost:    25,
		},
		{
			name:          "list_hasAll",
			expr:          `[1, 2, 3].hasAll([1, 2])`,
			estimatedCost: cost.FixedCostEstimate(27),
			actualCost:    27,
		},
		{
			name:          "list_hasExactly",
			expr:          `[1, 2].hasExactly([2, 1])`,
			estimatedCost: cost.FixedCostEstimate(29),
			actualCost:    29,
		},
		{
			name:          "list_hasAll_var",
			expr:          `x.hasAll(['a', 'b'])`,
			vars:          []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
			in:            map[string]any{"x": []string{"a", "b", "c"}},
			hints:         map[string]uint64{"x": 10},
			estimatedCost: cost.RangedCostEstimate(12, 32),
			actualCost:    18,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env := testListsEnv(t, tc.version, tc.vars...)
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}

			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost, tc.estimatedCostV0)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost)
			}
		})
	}
}

func testListsEnv(t *testing.T, version int, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	var listsOpt cel.EnvOption
	if version > 0 {
		listsOpt = Lists(ListsVersion(uint32(version)))
	} else {
		listsOpt = Lists()
	}
	baseOpts := []cel.EnvOption{
		listsOpt,
		cel.Container("google.expr.proto2.test"),
		cel.Types(
			&proto2pb.ExampleType{},
			&proto2pb.ExternalMessageType{},
			&proto2pb.TestAllTypes{},
			&proto3pb.TestAllTypes{},
		),
		NativeTypes(reflect.TypeFor[TestNestedType](), reflect.TypeFor[TestAllTypes]()),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Lists()) failed: %v", err)
	}
	return env
}

func TestGenRangeMaxSize(t *testing.T) {
	adapt := types.DefaultTypeAdapter
	// Negative size should fail.
	_, err := genRange(adapt, -1, defaultMaxRangeSize)
	if err == nil {
		t.Error("genRange(-1) should fail")
	}

	// Small size should work.
	val, err := genRange(adapt, 10, defaultMaxRangeSize)
	if err != nil {
		t.Fatalf("genRange(10) failed: %v", err)
	}
	if val == nil {
		t.Fatal("genRange(10) returned nil")
	}

	// Over the limit should fail, not allocate.
	_, err = genRange(adapt, defaultMaxRangeSize+1, defaultMaxRangeSize)
	if err == nil {
		t.Error("genRange(defaultMaxRangeSize+1) should fail")
	}

	// Zero limit disables the check.
	val, err = genRange(adapt, 100, 0)
	if err != nil {
		t.Fatalf("genRange(100, 0) with disabled limit failed: %v", err)
	}
	if val == nil {
		t.Fatal("genRange(100, 0) returned nil")
	}
}
