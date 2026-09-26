// Copyright 2022 Google LLC
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

package types_test

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/pb"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/ext"
	"cel.dev/cel-go/test"

	structpb "google.golang.org/protobuf/types/known/structpb"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestNativeTypes(t *testing.T) {
	var nativeTests = []struct {
		expr    string
		out     any
		in      any
		envOpts []any
	}{
		{
			expr: `types_test.TestAllTypes{
				NestedVal: types_test.TestNestedType{NestedMapVal: {1: false}},
				BoolVal: true,
				BytesVal: b'hello',
				DurationVal: duration('5s'),
				DoubleVal: 1.5,
				FloatVal: 2.5,
				Int32Val: 10,
				Int64Val: 20,
				StringVal: 'hello world',
				TimestampVal: timestamp('2011-08-06T01:23:45Z'),
				Uint32Val: 100u,
				Uint64Val: 200u,
				ListVal: [
					types_test.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
						custom_name: 'name',
					},
				],
				ArrayVal: [
					types_test.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
						custom_name: 'name',
					},
				],
				MapVal: {'map-key': types_test.TestAllTypes{BoolVal: true}},
				CustomSliceVal: [types_test.TestNestedSliceType{Value: 'none'}],
				CustomMapVal: {'even': types_test.TestMapVal{Value: 'more'}},
				custom_name: 'name',
			}`,
			out: &TestAllTypes{
				NestedVal:    &TestNestedType{NestedMapVal: map[int64]bool{1: false}},
				BoolVal:      true,
				BytesVal:     []byte("hello"),
				DurationVal:  time.Second * 5,
				DoubleVal:    1.5,
				FloatVal:     2.5,
				Int32Val:     10,
				Int64Val:     20,
				StringVal:    "hello world",
				TimestampVal: mustParseTime(t, "2011-08-06T01:23:45Z"),
				Uint32Val:    uint32(100),
				Uint64Val:    uint64(200),
				ListVal: []*TestNestedType{
					{
						NestedListVal:    []string{"goodbye", "cruel", "world"},
						NestedMapVal:     map[int64]bool{42: true},
						NestedCustomName: "name",
					},
				},
				ArrayVal: [1]*TestNestedType{{
					NestedListVal:    []string{"goodbye", "cruel", "world"},
					NestedMapVal:     map[int64]bool{42: true},
					NestedCustomName: "name",
				}},
				MapVal:         map[string]TestAllTypes{"map-key": {BoolVal: true}},
				CustomSliceVal: []TestNestedSliceType{{Value: "none"}},
				CustomMapVal:   map[string]TestMapVal{"even": {Value: "more"}},
				CustomName:     "name",
			},
			envOpts: []any{types.ParseStructTags(true)},
		},

		{
			expr: `types_test.TestAllTypes{
				nestedVal: types_test.TestNestedType{NestedMapVal: {1: false}},
				boolVal: true,
				BytesVal: b'hello',
				DurationVal: duration('5s'),
				DoubleVal: 1.5,
				FloatVal: 2.5,
				Int32Val: 10,
				Int64Val: 20,
				StringVal: 'hello world',
				TimestampVal: timestamp('2011-08-06T01:23:45Z'),
				Uint32Val: 100u,
				Uint64Val: 200u,
				ListVal: [
					types_test.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
						custom_name: 'name',
					},
				],
				ArrayVal: [
					types_test.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
						custom_name: 'name',
					},
				],
				MapVal: {'map-key': types_test.TestAllTypes{boolVal: true}},
				CustomSliceVal: [types_test.TestNestedSliceType{Value: 'none'}],
				CustomMapVal: {'even': types_test.TestMapVal{Value: 'more'}},
				CustomName: 'name',
			}`,
			out: &TestAllTypes{
				NestedVal:    &TestNestedType{NestedMapVal: map[int64]bool{1: false}},
				BoolVal:      true,
				BytesVal:     []byte("hello"),
				DurationVal:  time.Second * 5,
				DoubleVal:    1.5,
				FloatVal:     2.5,
				Int32Val:     10,
				Int64Val:     20,
				StringVal:    "hello world",
				TimestampVal: mustParseTime(t, "2011-08-06T01:23:45Z"),
				Uint32Val:    uint32(100),
				Uint64Val:    uint64(200),
				ListVal: []*TestNestedType{
					{
						NestedListVal:    []string{"goodbye", "cruel", "world"},
						NestedMapVal:     map[int64]bool{42: true},
						NestedCustomName: "name",
					},
				},
				ArrayVal: [1]*TestNestedType{{
					NestedListVal:    []string{"goodbye", "cruel", "world"},
					NestedMapVal:     map[int64]bool{42: true},
					NestedCustomName: "name",
				}},
				MapVal:         map[string]TestAllTypes{"map-key": {BoolVal: true}},
				CustomSliceVal: []TestNestedSliceType{{Value: "none"}},
				CustomMapVal:   map[string]TestMapVal{"even": {Value: "more"}},
				CustomName:     "name",
			},
			envOpts: []any{types.ParseStructTag("json")},
		},
		{
			expr: `types_test.TestAllTypes{
				NestedVal: types_test.TestNestedType{NestedMapVal: {1: false}},
				BoolVal: true,
				BytesVal: b'hello',
				DurationVal: duration('5s'),
				DoubleVal: 1.5,
				FloatVal: 2.5,
				Int32Val: 10,
				Int64Val: 20,
				StringVal: 'hello world',
				TimestampVal: timestamp('2011-08-06T01:23:45Z'),
				Uint32Val: 100u,
				Uint64Val: 200u,
				ListVal: [
					types_test.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
						NestedCustomName: 'name',
					},
				],
				ArrayVal: [
					types_test.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
						NestedCustomName: 'name',
					},
				],
				MapVal: {'map-key': types_test.TestAllTypes{BoolVal: true}},
				CustomSliceVal: [types_test.TestNestedSliceType{Value: 'none'}],
				CustomMapVal: {'even': types_test.TestMapVal{Value: 'more'}},
				CustomName: 'name',
			}`,
			out: &TestAllTypes{
				NestedVal:    &TestNestedType{NestedMapVal: map[int64]bool{1: false}},
				BoolVal:      true,
				BytesVal:     []byte("hello"),
				DurationVal:  time.Second * 5,
				DoubleVal:    1.5,
				FloatVal:     2.5,
				Int32Val:     10,
				Int64Val:     20,
				StringVal:    "hello world",
				TimestampVal: mustParseTime(t, "2011-08-06T01:23:45Z"),
				Uint32Val:    uint32(100),
				Uint64Val:    uint64(200),
				ListVal: []*TestNestedType{
					{
						NestedListVal:    []string{"goodbye", "cruel", "world"},
						NestedMapVal:     map[int64]bool{42: true},
						NestedCustomName: "name",
					},
				},
				ArrayVal: [1]*TestNestedType{{
					NestedListVal:    []string{"goodbye", "cruel", "world"},
					NestedMapVal:     map[int64]bool{42: true},
					NestedCustomName: "name",
				}},
				MapVal:         map[string]TestAllTypes{"map-key": {BoolVal: true}},
				CustomSliceVal: []TestNestedSliceType{{Value: "none"}},
				CustomMapVal:   map[string]TestMapVal{"even": {Value: "more"}},
				CustomName:     "name",
			},
		},
		{
			expr: `types_test.TestAllTypes{
					PbVal: test.TestAllTypes{single_int32: 123}
				}.PbVal`,
			out: &proto3pb.TestAllTypes{SingleInt32: 123},
		},
		{
			expr: `types_test.TestAllTypes{PbVal: test.TestAllTypes{}} ==
			types_test.TestAllTypes{PbVal: test.TestAllTypes{single_bool: false}}`,
		},
		{expr: `types_test.TestNestedType{} == TestNestedType{}`},
		{expr: `types_test.TestAllTypes{}.BoolVal != true`},
		{expr: `!has(types_test.TestAllTypes{}.BoolVal) && !has(types_test.TestAllTypes{}.NestedVal)`},
		{expr: `type(types_test.TestAllTypes) == type`},
		{expr: `type(types_test.TestAllTypes{}) == types_test.TestAllTypes`},
		{expr: `type(types_test.TestAllTypes{}) == types_test.TestAllTypes`},
		{expr: `types_test.TestAllTypes != test.TestAllTypes`},
		{expr: `types_test.TestAllTypes{BoolVal: true} != dyn(test.TestAllTypes{single_bool: true})`},
		{expr: `types_test.TestAllTypes{}.NestedVal == types_test.TestNestedType{}`},
		{expr: `types_test.TestNestedType{} == types_test.TestAllTypes{}.NestedStructVal`},
		{expr: `types_test.TestAllTypes{}.NestedStructVal == types_test.TestNestedType{}`},
		{expr: `types_test.TestAllTypes{}.ListVal.size() == 0`},
		{expr: `types_test.TestAllTypes{}.MapVal.size() == 0`},
		{expr: `types_test.TestAllTypes{}.TimestampVal == timestamp(0)`},
		{expr: `test.TestAllTypes{}.single_timestamp == timestamp(0)`},
		{expr: `[TestAllTypes{BoolVal: true}, TestAllTypes{BoolVal: false}].exists(t, t.BoolVal == true)`},
		{expr: `[TestAllTypes{CustomName: 'Alice'}, TestAllTypes{CustomName: 'Bob'}].exists(t, t.CustomName == 'Alice')`},
		{expr: `[TestAllTypes{custom_name: 'Alice'}, TestAllTypes{custom_name: 'Bob'}].exists(t, t.custom_name == 'Alice')`, envOpts: []any{types.ParseStructTags(true)}},
		{expr: `TestAllTypes{BytesArrayVal: b'1234'}.BytesArrayVal != b'123'`},
		{expr: `TestAllTypes{BytesArrayVal: b'1234'}.BytesArrayVal == b'1234'`},
		{
			expr: `tests.all(t, t.Int32Val > 17)`,
			in: map[string]any{
				"tests": []*TestAllTypes{{Int32Val: 18}, {Int32Val: 19}, {Int32Val: 20}},
			},
		},
	}
	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			env := testNativeEnv(t, tc.envOpts...)
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
					t.Fatal(err)
				}
				in := tc.in
				if in == nil {
					in = cel.NoVars()
				}
				out, _, err := prg.Eval(in)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.out
				if want == nil {
					want = true
				}
				wantPB, isPB := want.(proto.Message)
				if isPB && !pb.Equal(wantPB, out.Value().(proto.Message)) {
					t.Errorf("got %v, wanted %v for expr: %s", out.Value(), want, tc.expr)
				}
				if !isPB && !reflect.DeepEqual(out.Value(), want) {
					t.Errorf("got %v, wanted %v for expr: %s", out.Value(), want, tc.expr)
				}
			}
		})
	}
}

func TestNativeFindStructFieldNames(t *testing.T) {
	env := testNativeEnv(t, types.ParseStructTags(true))
	provider := env.CELTypeProvider()
	tests := []struct {
		typeName string
		fields   []string
	}{
		{
			typeName: "types_test.TestNestedType",
			fields:   []string{"NestedListVal", "NestedMapVal", "custom_name"},
		},
		{
			typeName: "google.expr.proto3.test.TestAllTypes.NestedMessage",
			fields:   []string{"bb"},
		},
		{
			typeName: "invalid.TypeName",
			fields:   []string{},
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%s", tc.typeName), func(t *testing.T) {
			fields, _ := provider.FindStructFieldNames(tc.typeName)
			sort.Strings(fields)
			sort.Strings(tc.fields)
			if !reflect.DeepEqual(fields, tc.fields) {
				t.Errorf("got %v, wanted %v", fields, tc.fields)
			}
		})
	}
}

func TestNativeTypesStaticErrors(t *testing.T) {
	var nativeTests = []struct {
		expr string
		err  string
	}{
		{
			expr: `TestAllTypos{}`,
			err: `ERROR: <input>:1:13: undeclared reference to 'TestAllTypos' (in container 'types_test')
			 | TestAllTypos{}
			 | ............^`,
		},
		{
			expr: `types_test.TestAllTypes{bool_val: false}`,
			err: `ERROR: <input>:1:33: undefined field 'bool_val'
			| types_test.TestAllTypes{bool_val: false}
			| ................................^`,
		},
		{
			expr: `types_test.TestAllTypes{UnsupportedVal: null}`,
			err: `ERROR: <input>:1:39: undefined field 'UnsupportedVal'
			| types_test.TestAllTypes{UnsupportedVal: null}
			| ......................................^`,
		},
		{
			expr: `types_test.TestAllTypes{UnsupportedListVal: null}`,
			err: `ERROR: <input>:1:43: undefined field 'UnsupportedListVal'
			| types_test.TestAllTypes{UnsupportedListVal: null}
			| ..........................................^`,
		},
		{
			expr: `types_test.TestAllTypes{UnsupportedMapVal: null}`,
			err: `ERROR: <input>:1:42: undefined field 'UnsupportedMapVal'
			| types_test.TestAllTypes{UnsupportedMapVal: null}
			| .........................................^`,
		},
	}
	env := testNativeEnv(t)
	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if iss.Err() == nil {
				t.Fatalf("env.Compile(%v) succeeded, wanted error", tc.expr)
			}
			if !test.Compare(iss.Err().Error(), tc.err) {
				t.Errorf("env.Compile(%v) got %v, wanted error %s", tc.expr, iss.Err(), tc.err)
			}
		})
	}
}

func TestNativeTypesJsonSerialization(t *testing.T) {
	tests := []struct {
		expr                 string
		out                  string
		additionalEnvOptions []any
	}{
		{
			expr: `[b'string']`,
			out:  `["c3RyaW5n"]`,
		},
		{
			expr: `TestAllTypes{
				BoolVal: true,
				DurationVal: duration('5s'),
				DoubleVal: 1.5,
				FloatVal: 2.0,
				Int32Val: 23,
				Int64Val: 64,
				MapVal: {
					'map-key': types_test.TestAllTypes{
						BoolVal: true
					}
				},
				NestedVal: TestNestedType{
					NestedListVal: ["first", "second"],
				},
				StringVal: "string",
				CustomName: "name",
			}`,
			out: `{
				"CustomName":  "name",
				"DoubleVal":  1.5,
				"DurationVal":  "5s",
				"FloatVal":  2,
				"Int32Val":  23,
				"Int64Val":  64,
				"MapVal": {
	              "map-key": {
    	            "boolVal": true
        	      }
            	},
				"StringVal":  "string",
				"boolVal":  true,
				"nestedVal": {
					"NestedListVal": [
					  "first",
					  "second"
					]
				}
			  }`,
		},
		{
			expr: `TestAllTypes{
				BoolVal: true,
				DurationVal: duration('5s'),
				DoubleVal: 1.5,
				FloatVal: 2.0,
				Int32Val: 23,
				Int64Val: 64,
				MapVal: {
					'map-key': types_test.TestAllTypes{
						BoolVal: true
					}
				},
				NestedVal: TestNestedType{
					NestedListVal: ["first", "second"],
				},
				StringVal: "string",
                custom_name: "name",
			}`,
			out: `{
				"DoubleVal":  1.5,
				"DurationVal":  "5s",
				"FloatVal":  2,
				"Int32Val":  23,
				"Int64Val":  64,
				"MapVal": {
	              "map-key": {
    	            "boolVal": true
        	      }
            	},
				"StringVal":  "string",
				"boolVal":  true,
				"custom_name": "name",
				"nestedVal": {
					"NestedListVal": [
					  "first",
					  "second"
					]
				}
			  }`,
			additionalEnvOptions: []any{types.ParseStructTags(true)},
		},
		{
			expr: `TestSpecialJSONTags{
				ignored: "sensitive",
				hyphen_name: "hyphen-val",
				quoted_hyphen: "quoted-val",
				renamed: "renamed-val",
				empty_int: 0,
				empty_str: "",
				pop_int: 42,
				keep_zero: 0,
				keep_empty: "",
				keep_false: false,
				cel_field: "divergent-val",
			}`,
			out: `{
				"-": "quoted-val",
				"custom_json_name": "renamed-val",
				"json_field": "divergent-val",
				"keep_empty": "",
				"keep_false": false,
				"keep_zero": 0,
				"pop_int": 42
			}`,
			additionalEnvOptions: []any{types.ParseStructTags(true)},
		},
		{
			expr: `TestEmbeddedTypes{
				name: "alice",
				Skipped: "secret",
				NestedListVal: ["a", "b"],
				custom_name: "nested",
			}`,
			out: `{
				"embedded": {
					"NestedListVal": [
						"a",
						"b"
					],
					"custom_name": "nested"
				},
				"name": "alice"
			}`,
			additionalEnvOptions: []any{types.ParseStructTags(true)},
		},
		{
			expr: `TestEmbeddedTypes{
				name: "bob",
				Skipped: "secret",
			}`,
			out: `{
				"name": "bob"
			}`,
			additionalEnvOptions: []any{types.ParseStructTags(true)},
		},
		{
			expr: `TestEmbeddedPointerTypes{
				NestedListVal: ["x"],
				custom_name: "ptr_nested",
			}`,
			out: `{
				"embedded": {
					"NestedListVal": [
						"x"
					],
					"custom_name": "ptr_nested"
				}
			}`,
			additionalEnvOptions: []any{types.ParseStructTags(true)},
		},
		{
			expr:                 `TestEmbeddedPointerTypes{}`,
			out:                  `{}`,
			additionalEnvOptions: []any{types.ParseStructTags(true)},
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			env := testNativeEnv(t, tst.additionalEnvOptions...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			conv, err := out.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
			if err != nil {
				t.Fatalf("out.ConvertToNative(Value) failed: %v", err)
			}
			json := protojson.Format(conv.(proto.Message))
			if !test.Compare(json, tc.out) {
				t.Errorf("expr %v converted to %v, wanted %v", tc.expr, json, tc.out)
			}
		})
	}
}

func TestNativeTypesRuntimeErrors(t *testing.T) {
	var nativeTests = []struct {
		expr string
		err  string
	}{
		{
			expr: `TestAllTypos{}`,
			err:  `unknown type: TestAllTypos`,
		},
		{
			expr: `types_test.TestAllTypes{bool_val: false}`,
			err:  `no such field: bool_val`,
		},
		{
			expr: `types_test.TestAllTypes{UnsupportedVal: null}`,
			err:  `no such field: UnsupportedVal`,
		},
		{
			expr: `types_test.TestAllTypes{UnsupportedListVal: null}`,
			err:  `no such field: UnsupportedListVal`,
		},
		{
			expr: `types_test.TestAllTypes{UnsupportedMapVal: null}`,
			err:  `no such field: UnsupportedMapVal`,
		},
		{
			expr: `types_test.TestAllTypes{privateVal: null}`,
			err:  `no such field: privateVal`,
		},
		{
			expr: `types_test.TestAllTypes{}.UnsupportedMapVal`,
			err:  `no such field: UnsupportedMapVal`,
		},
		{
			expr: `types_test.TestAllTypes{}.privateVal`,
			err:  `no such field: privateVal`,
		},
		{
			expr: `types_test.TestAllTypes{BoolVal: 'false'}`,
			err:  `unsupported native conversion from string to 'bool'`,
		},
		{
			expr: `has(types_test.TestAllTypes{}.BadFieldName)`,
			err:  `no such field: BadFieldName`,
		},
		{
			expr: `types_test.TestAllTypes{}[42]`,
			err:  `no such overload`,
		},
		{
			expr: `types_test.TestAllTypes{Int32Val: 9223372036854775807}`,
			err:  `integer overflow`,
		},
		{
			expr: `types_test.TestAllTypes{Uint32Val: 9223372036854775807u}`,
			err:  `unsigned integer overflow`,
		},
	}
	env := testNativeEnv(t)
	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			ast, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatal(err)
				}
				return
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				var got any = err
				if err == nil {
					got = out
				}
				t.Fatalf("prg.Eval() got %v, wanted error %v", got, tc.err)
			}
		})
	}
}

func TestNativeTypesErrors(t *testing.T) {
	envTests := []struct {
		nativeType any
		err        string
	}{
		{
			nativeType: reflect.TypeOf(1),
			err:        "unsupported reflect.Type",
		},
		{
			nativeType: reflect.ValueOf(1),
			err:        "unsupported reflect.Type",
		},
		{
			nativeType: 1,
			err:        "must be reflect.Type",
		},
	}
	for i, tst := range envTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, err := cel.NewEnv(ext.NativeTypes(tc.nativeType))
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("cel.NewEnv(NativeTypes(%v)) got error %v, wanted %v", tc.nativeType, err, tc.err)
			}
		})
	}
}

func TestNativeTypesConvertToNative(t *testing.T) {
	env := testNativeEnv(t, ext.NativeTypes(reflect.TypeOf(TestNestedType{})))
	adapter := env.CELTypeAdapter()
	conversions := []struct {
		in     any
		inType *cel.Type
		out    any
		err    string
	}{
		{
			in:     &TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("types_test.TestAllTypes"),
			out:    &TestAllTypes{BoolVal: true},
		},
		{
			in:     TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("types_test.TestAllTypes"),
			out:    &TestAllTypes{BoolVal: true},
		},
		{
			in:     &TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("types_test.TestAllTypes"),
			out:    TestAllTypes{BoolVal: true},
		},
		{
			in:     nil,
			inType: cel.NullType,
			out:    types.NullValue,
		},
		{
			in:     &TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("types_test.TestAllTypes"),
			out:    &proto3pb.TestAllTypes{},
			err:    "type conversion error",
		},
		{
			in:     [3]int32{1, 2, 3},
			inType: cel.ListType(cel.IntType),
			out:    []int32{1, 2, 3},
		},
		{
			in:     &[3]byte{1, 2, 3},
			inType: cel.BytesType,
			out:    []byte{1, 2, 3},
		},
		{
			in:     [3]byte{1, 2, 3},
			inType: cel.BytesType,
			out:    []byte{1, 2, 3},
		},
	}
	for _, c := range conversions {
		inVal := adapter.NativeToValue(c.in)
		if types.IsError(inVal) {
			t.Fatalf("adapter.NativeToValue(%v) failed: %v", c.in, inVal)
		}
		if inVal.Type().TypeName() != c.inType.TypeName() {
			t.Fatalf("adapter.NativeToValue() got type %v, wanted type %v", inVal.Type(), c.inType)
		}
		out, err := inVal.ConvertToNative(reflect.TypeOf(c.out))
		if err != nil {
			if c.err != "" {
				if !strings.Contains(err.Error(), c.err) {
					t.Fatalf("%v.ConvertToNative(%T) got %v, wanted error %v", c.in, c.out, err, c.err)
				}
				return
			}
			t.Fatalf("%v.ConvertToNative(%T) failed: %v", c.in, c.out, err)
		}
		if !reflect.DeepEqual(out, c.out) {
			t.Errorf("%v.ConvertToNative(%T) got %v, wanted %v", c.in, c.out, out, c.out)
		}
	}
}

func TestConvertToTypeErrors(t *testing.T) {
	env := testNativeEnv(t, ext.NativeTypes(reflect.TypeOf(TestNestedType{})))
	adapter := env.CELTypeAdapter()
	conversions := []struct {
		in  any
		out any
		err string
	}{
		{
			in:  &TestAllTypes{BoolVal: true},
			out: &TestAllTypes{BoolVal: true},
		},
		{
			in:  TestAllTypes{BoolVal: true},
			out: &TestAllTypes{BoolVal: true},
		},
		{
			in:  &TestAllTypes{BoolVal: true},
			out: TestAllTypes{BoolVal: true},
		},
		{
			in:  &TestAllTypes{BoolVal: true},
			out: &proto3pb.TestAllTypes{},
			err: "type conversion error",
		},
	}
	for _, c := range conversions {
		inVal := adapter.NativeToValue(c.in)
		outVal := adapter.NativeToValue(c.out)
		if types.IsError(inVal) {
			t.Fatalf("adapter.NativeToValue(%v) failed: %v", c.in, inVal)
		}
		if types.IsError(outVal) {
			t.Fatalf("adapter.NativeToValue(%v) failed: %v", c.out, outVal)
		}
		conv := inVal.ConvertToType(outVal.Type())
		if c.err != "" {
			if !types.IsError(conv) {
				t.Fatalf("%v.ConvertToType(%v) got %v, wanted error %v", c.in, outVal.Type(), conv, c.err)
			}
			convErr := conv.(*types.Err)
			if !strings.Contains(convErr.Error(), c.err) {
				t.Fatalf("%v.ConvertToType(%v) got %v, wanted error %v", c.in, outVal.Type(), conv, c.err)
			}
			return
		}
		if conv != inVal {
			t.Errorf("%v.ConvertToType(%v) got %v, wanted %v", c.in, outVal.Type(), conv, c.err)
		}
		conv = inVal.ConvertToType(types.TypeType)
		if conv.Type() != types.TypeType || conv.(ref.Type) != inVal.Type() {
			t.Errorf("%v.ConvertToType(Type) got %v, wanted %v", inVal, conv, inVal.Type())
		}
	}
}

func TestNativeTypesWithOptional(t *testing.T) {
	var nativeTests = []struct {
		expr string
	}{
		{expr: `!optional.ofNonZeroValue(types_test.TestAllTypes{}).hasValue()`},
		{expr: `!types_test.TestAllTypes{}.?BoolVal.orValue(false)`},
		{expr: `!types_test.TestAllTypes{}.?BoolVal.hasValue()`},
		{expr: `!types_test.TestAllTypes{BoolVal: false}.?BoolVal.hasValue()`},
		{expr: `types_test.TestAllTypes{BoolVal: true}.?BoolVal.hasValue()`},
		{expr: `types_test.TestAllTypes{}.NestedVal.?NestedMapVal.orValue({}).size() == 0`},
	}
	env := testNativeEnv(t, cel.OptionalTypes())
	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
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
					t.Fatal(err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(out.Value(), true) {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestNativeTypesWithCELTypedFields(t *testing.T) {
	var nativeTests = []struct {
		expr string
	}{
		{
			expr: `types_test.TestRefValFieldType{optional_name: optional.of('my name')}.optional_name.orValue('') == 'my name'`,
		},
		{
			expr: `types_test.TestRefValFieldType{IntVal: 2}.IntVal >= 1`,
		},
		{
			expr: `types_test.TestRefValFieldType{time: timestamp('2001-01-01T00:00:00Z')}.time > timestamp('1970-01-01T00:00:00Z')`,
		},
	}
	env := testNativeEnv(t, cel.OptionalTypes(), types.ParseStructTag("cel"))
	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
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
					t.Fatal(err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(out.Value(), true) {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestNativeTypeConvertToType(t *testing.T) {
	var nativeTests = []struct {
		tag string
	}{
		{tag: "cel"},
		{tag: "json"},
	}

	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			handler := func(f reflect.StructField) string {
				tag, found := f.Tag.Lookup(tc.tag)
				if found {
					splits := strings.Split(tag, ",")
					if len(splits) > 0 {
						return splits[0]
					}
				}
				return f.Name
			}
			nt, err := types.NewNativeType(reflect.TypeFor[*TestAllTypes](), types.ParseStructField(handler))
			if err != nil {
				t.Fatalf("NewNativeType() failed: %v", err)
			}
			if nt.ConvertToType(types.TypeType) != types.TypeType {
				t.Error("ConvertToType(Type) failed")
			}
			if !types.IsError(nt.ConvertToType(types.StringType)) {
				t.Errorf("ConvertToType(String) got %v, wanted error", nt.ConvertToType(types.StringType))
			}
		})
	}
}

func TestNativeTypeConvertToNative(t *testing.T) {
	nt, err := types.NewNativeType(reflect.TypeFor[*TestAllTypes]())
	if err != nil {
		t.Fatalf("NewNativeType() failed: %v", err)
	}
	out, err := nt.ConvertToNative(reflect.TypeOf(1))
	if err == nil {
		t.Errorf("nt.ConvertToNative(1) produced %v, wanted error", out)
	}
}

func TestNativeTypeHasTrait(t *testing.T) {
	nt, err := types.NewNativeType(reflect.TypeFor[*TestAllTypes]())
	if err != nil {
		t.Fatalf("NewNativeType() failed: %v", err)
	}
	if !nt.HasTrait(traits.IndexerType) || !nt.HasTrait(traits.FieldTesterType) {
		t.Error("nt.HasTrait() failed indicate support for presence test and field access.")
	}
}

func TestNativeTypeValue(t *testing.T) {
	nt, err := types.NewNativeType(reflect.TypeFor[*TestAllTypes]())
	if err != nil {
		t.Fatalf("NewNativeType() failed: %v", err)
	}
	if nt.Value() != nt.String() {
		t.Errorf("nt.Value() got %v, wanted %v", nt.Value(), nt.String())
	}
}

func TestNativeStructWithMultipleSameFieldNames(t *testing.T) {
	tagHandler := func(f reflect.StructField) string {
		tag, found := f.Tag.Lookup("cel")
		if found {
			splits := strings.Split(tag, ",")
			if len(splits) > 0 {
				return splits[0]
			}
		}
		return f.Name
	}
	_, err := types.NewNativeType(
		reflect.TypeFor[TestStructWithMultipleSameNames](),
		types.ParseStructField(tagHandler),
	)
	if err == nil {
		t.Fatal("NewNativeType() did not fail as expected")
	}
	if !strings.Contains(err.Error(), "field name already exists") {
		t.Fatalf("NewNativeType() expected duplicated field name error, but got: %v", err)
	}
}

func TestNativeStructEmbedded(t *testing.T) {
	var nativeTests = []struct {
		expr string
		in   any
		out  any
	}{
		{
			expr: `test.embedded.custom_name == "name"`,
			in: map[string]any{
				"test": &TestEmbeddedTypes{
					TestNestedType: TestNestedType{NestedCustomName: "name"},
					Skipped:        "should-be-hidden",
				},
			},
			out: true,
		},
		{
			expr: `dyn(test.embedded)["-"] == "error"`,
			in: map[string]any{
				"test": &TestEmbeddedTypes{
					TestNestedType: TestNestedType{NestedCustomName: "name"},
					Skipped:        "should-be-hidden",
				},
			},
			out: errors.New("no such field: -"),
		},
		{
			expr: `test.embedded == types_test.TestNestedType{custom_name: "name"}`,
			in: map[string]any{
				"test": &TestEmbeddedTypes{
					TestNestedType: TestNestedType{NestedCustomName: "name"},
					Skipped:        "should-be-hidden",
				},
			},
			out: true,
		},
		{
			expr: `test.Name == "name"`,
			in: map[string]any{
				"test": &TestEmbeddedTypes{
					Custom: Custom{Name: "name"},
				},
			},
			out: true,
		},
	}

	envOpts := []cel.EnvOption{
		ext.NativeTypes(
			reflect.TypeFor[*TestEmbeddedTypes](),
			reflect.TypeFor[*TestNestedType](),
			types.ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("types_test.TestEmbeddedTypes")),
	}

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("cel.NewEnv(NativeTypes()) failed: %v", err)
	}

	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
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
					t.Fatal(err)
				}
				out, _, err := prg.Eval(tc.in)
				if err != nil {
					if !errors.Is(err, tc.out.(error)) {
						t.Fatalf("got %v, wanted %v for expr: %s", err, tc.out, tc.expr)
					}
					continue
				}
				if !reflect.DeepEqual(out.Value(), tc.out) {
					t.Errorf("got %v, wanted %v for expr: %s", out.Value(), tc.out, tc.expr)
				}
			}
		})
	}
}

func TestNativeStructEmbeddedPointer(t *testing.T) {
	nativeTests := []struct {
		expr string
		in   map[string]any
		out  any
	}{
		{
			expr: `!has(test.custom_name) && test.custom_name == ""`,
			in: map[string]any{
				"test": &TestEmbeddedPointerTypes{
					TestNestedType: nil,
				},
			},
			out: true,
		},
		{
			expr: `has(test.custom_name) && test.custom_name == "name"`,
			in: map[string]any{
				"test": &TestEmbeddedPointerTypes{
					TestNestedType: &TestNestedType{NestedCustomName: "name"},
				},
			},
			out: true,
		},
		{
			expr: `types_test.TestEmbeddedPointerTypes{custom_name: "name"}.custom_name == "name"`,
			in:   nil,
			out:  true,
		},
	}

	envOpts := []cel.EnvOption{
		ext.NativeTypes(
			reflect.TypeFor[*TestEmbeddedPointerTypes](),
			reflect.TypeFor[*TestNestedType](),
			types.ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("types_test.TestEmbeddedPointerTypes")),
	}

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("cel.NewEnv(NativeTypes()) failed: %v", err)
	}

	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			for _, ast := range []*cel.Ast{pAst, cAst} {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatal(err)
				}
				out, _, err := prg.Eval(tc.in)
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if !reflect.DeepEqual(out.Value(), tc.out) {
					t.Errorf("got %v, wanted %v for expr: %s", out.Value(), tc.out, tc.expr)
				}
			}
		})
	}
}

func TestNativeStructHiddenField(t *testing.T) {
	envOpts := []cel.EnvOption{
		ext.NativeTypes(
			reflect.TypeFor[*TestEmbeddedTypes](),
			types.ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("types_test.TestEmbeddedTypes")),
	}

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("cel.NewEnv(NativeTypes()) failed: %v", err)
	}

	// 1. Static reference compilation failure case
	// Attempting to compile `test.Password` should fail static analysis because the field is skipped/hidden.
	_, iss := env.Compile("test.Password")
	if iss.Err() == nil {
		t.Error("env.Compile('test.Password') succeeded, expected a compilation/check error")
	}

	// 2. Dynamic reference runtime evaluation failure case
	// Using dyn(test).Password should compile successfully (since dyn disables static type checks),
	// but it must fail at runtime during evaluation because the field is not exposed.
	ast, iss := env.Compile("dyn(test).Password")
	if iss.Err() != nil {
		t.Fatalf("env.Compile('dyn(test).Password') failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	in := map[string]any{
		"test": &TestEmbeddedTypes{
			Skipped: "sensitive_password",
		},
	}
	out, _, err := prg.Eval(in)
	if err == nil {
		t.Errorf("prg.Eval() succeeded and returned %v, expected runtime error accessing hidden field", out)
	}
}

type TestNestedStruct struct {
	ListVal []*TestNestedType
}

func TestNativeNestedStruct(t *testing.T) {
	var nativeTests = []struct {
		expr string
		in   any
	}{
		{
			expr: `test.ListVal.exists(x, x.custom_name == "name")`,
			in: map[string]any{
				"test": &TestNestedStruct{ListVal: []*TestNestedType{{NestedCustomName: "name"}}},
			},
		},
	}

	envOpts := []cel.EnvOption{
		ext.NativeTypes(
			reflect.ValueOf(&TestNestedStruct{}),
			types.ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("types_test.TestNestedStruct")),
	}

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("cel.NewEnv(NativeTypes()) failed: %v", err)
	}

	for i, tst := range nativeTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
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
					t.Fatal(err)
				}
				out, _, err := prg.Eval(tc.in)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(out.Value(), true) {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestNativeTypesVersion(t *testing.T) {
	_, err := cel.NewEnv(ext.NativeTypes(ext.NativeTypesVersion(0)))
	if err != nil {
		t.Fatalf("NewEnv(NativeTypes(NativeTypesVersion(0))) failed: %v", err)
	}
}

func TestTypeResolutionRace(t *testing.T) {
	customType := reflect.TypeFor[*Custom]()
	env, err := cel.NewEnv(
		cel.Container("types_test"),
		ext.NativeTypes(
			types.ParseStructTag("cel"),
			customType,
		),
	)
	if err != nil {
		t.Fatal("NewEnv:", err)
	}

	tests := []struct {
		name string
		expr string
	}{
		{name: "custom1", expr: `Custom{ name: "name1" }`},
		{name: "custom2", expr: `Custom{ name: "name2" }`},
		{name: "custom3", expr: `Custom{ name: "name3" }`},
		{name: "custom4", expr: `Custom{ name: "name4" }`},
		{name: "custom5", expr: `Custom{ name: "name5" }`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ast, iss := env.Compile(test.expr)
			if err := iss.Err(); err != nil {
				t.Fatal("Compile:", err)
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %s", err)
			}
			prg.Eval(cel.NoVars())
		})
	}
}

func TestNativeToValueDelegatesUnregisteredStructs(t *testing.T) {
	custom := &recordingAdapter{base: types.DefaultTypeAdapter}
	env, err := cel.NewEnv(
		cel.CustomTypeAdapter(custom),
		ext.NativeTypes(reflect.TypeOf(registeredNativeStruct{})),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	adapter := env.CELTypeAdapter()

	// An unregistered struct must reach the composed base adapter.
	got := adapter.NativeToValue(unregisteredNativeStruct{Name: "x"})
	if !custom.saw {
		t.Error("base adapter was not consulted for an unregistered struct")
	}
	if got.Equal(types.String("from-base-adapter")) != types.True {
		t.Errorf("NativeToValue(unregisteredNativeStruct) = %v, want the base adapter's value", got)
	}

	// A registered native type must still be wrapped as a native object.
	custom.saw = false
	gotReg := adapter.NativeToValue(registeredNativeStruct{Name: "y"})
	if custom.saw {
		t.Error("base adapter was consulted for a registered native type")
	}
	if tn := gotReg.Type().TypeName(); !strings.Contains(tn, "registeredNativeStruct") {
		t.Errorf("NativeToValue(registeredNativeStruct).Type() = %q, want a native object type", tn)
	}
}

func TestNativeObjectCalculateSize(t *testing.T) {
	env, err := cel.NewEnv(
		ext.NativeTypes(
			reflect.TypeOf(TestAllTypes{}),
			reflect.TypeOf(TestNestedType{}),
			reflect.TypeOf(TestEmbeddedPointerTypes{}),
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	adapter := env.CELTypeAdapter()

	tests := []struct {
		name string
		val  any
		want uint32
	}{
		{
			name: "empty_struct",
			val:  &TestNestedType{},
			want: 1, // 1 (container)
		},
		{
			name: "nil_embedded_pointer",
			val:  &TestEmbeddedPointerTypes{},
			want: 1, // 1 (container); promoted fields through the nil embedded pointer count as unset
		},
		{
			name: "struct_with_scalar_and_list",
			val: &TestNestedType{
				NestedListVal: []string{"a", "b", "c"},
			},
			want: 5, // 1 (root struct) + ["a", "b", "c"] (1 list container + 3 elements = 4) = 5
		},
		{
			name: "struct_with_nested_map",
			val: &TestNestedType{
				NestedMapVal: map[int64]bool{1: true, 2: false},
			},
			want: 6, // 1 (root struct) + map (1 container + (1+1) + (1+1) = 5) = 6
		},
		{
			name: "nested_struct",
			val: &TestAllTypes{
				StringVal: "hello",
				NestedVal: &TestNestedType{
					NestedListVal: []string{"a", "b"},
				},
			},
			// 1 (root struct) + "hello"(1 unit) + NestedVal(1 container + ["a", "b"](1+2=3) = 4) = 6
			want: 6,
		},
		{
			name: "bytes_and_time",
			val: &TestAllTypes{
				BytesVal:     []byte("test"),
				DurationVal:  time.Second,
				TimestampVal: time.Unix(100, 0),
			},
			// 1 (root struct) + "test"(1 unit) + duration(1) + timestamp(1) = 4
			want: 4,
		},
		{
			name: "slice_of_structs",
			val: &TestAllTypes{
				ListVal: []*TestNestedType{
					{NestedListVal: []string{"x"}},
					{NestedListVal: []string{"y", "z"}},
				},
			},
			want: 9,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val := adapter.NativeToValue(tc.val)
			sizer, ok := val.(types.AggregateSizeVisitor)
			if !ok {
				t.Fatalf("expected types.AggregateSizeVisitor implementation for %T", val)
			}
			if got := sizer.AggregateSize(types.NewSizeCalculator()); got != tc.want {
				t.Errorf("got aggregate size %d, want %d", got, tc.want)
			}
		})
	}
}

func BenchmarkNativeTypesEval(b *testing.B) {
	benchmarks := []struct {
		name    string
		expr    string
		in      any
		envOpts []any
	}{
		{
			name: "FieldAccess",
			expr: "t.Int32Val + t.Int64Val",
			in: map[string]any{
				"t": &TestAllTypes{Int32Val: 10, Int64Val: 20},
			},
		},
		{
			name: "NestedFieldAccess",
			expr: "t.NestedVal.NestedCustomName == 'name'",
			in: map[string]any{
				"t": &TestAllTypes{
					NestedVal: &TestNestedType{NestedCustomName: "name"},
				},
			},
		},
		{
			name: "StructCreation",
			expr: `types_test.TestAllTypes{
				BoolVal: true,
				Int32Val: 10,
				Int64Val: 20,
				StringVal: 'hello world',
			}`,
		},
		{
			name: "FieldPresence",
			expr: "has(t.BoolVal) && has(t.NestedVal)",
			in: map[string]any{
				"t": &TestAllTypes{
					BoolVal:   true,
					NestedVal: &TestNestedType{},
				},
			},
		},
		{
			name:    "StructTagFieldAccess",
			expr:    "t.custom_name == 'name'",
			envOpts: []any{types.ParseStructTags(true)},
			in: map[string]any{
				"t": &TestAllTypes{CustomName: "name"},
			},
		},
		{
			name: "ListExists",
			expr: "tests.exists(t, t.Int32Val > 15)",
			in: map[string]any{
				"tests": []*TestAllTypes{
					{Int32Val: 10},
					{Int32Val: 20},
				},
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			envOpts := append([]any{
				cel.Variable("t", cel.ObjectType("types_test.TestAllTypes")),
			}, bm.envOpts...)
			env := testNativeEnv(b, envOpts...)
			ast, iss := env.Compile(bm.expr)
			if iss.Err() != nil {
				b.Fatalf("env.Compile(%q) failed: %v", bm.expr, iss.Err())
			}
			prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
			if err != nil {
				b.Fatalf("env.Program() failed: %v", err)
			}
			input := bm.in
			if input == nil {
				input = cel.NoVars()
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				prg.Eval(input)
			}
		})
	}
}

func BenchmarkNativeToValue(b *testing.B) {
	env := testNativeEnv(b)
	adapter := env.CELTypeAdapter()

	nested := &TestNestedType{
		NestedListVal:    []string{"a", "b", "c"},
		NestedMapVal:     map[int64]bool{1: true},
		NestedCustomName: "test",
	}
	allTypes := &TestAllTypes{
		BoolVal:   true,
		Int32Val:  10,
		Int64Val:  20,
		StringVal: "hello world",
		NestedVal: nested,
		ListVal:   []*TestNestedType{nested},
	}
	allTypesSlice := []*TestAllTypes{allTypes, allTypes}

	benchmarks := []struct {
		name string
		val  any
	}{
		{name: "TestNestedType", val: nested},
		{name: "TestAllTypes", val: allTypes},
		{name: "SliceTestAllTypes", val: allTypesSlice},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				adapter.NativeToValue(bm.val)
			}
		})
	}
}

func BenchmarkConvertToNative(b *testing.B) {
	env := testNativeEnv(b)
	adapter := env.CELTypeAdapter()

	allTypes := &TestAllTypes{
		BoolVal:   true,
		Int32Val:  10,
		Int64Val:  20,
		StringVal: "hello world",
	}
	celVal := adapter.NativeToValue(allTypes)
	targetType := reflect.TypeOf(&TestAllTypes{})

	allTypesSlice := []*TestAllTypes{allTypes, allTypes}
	celSliceVal := adapter.NativeToValue(allTypesSlice)
	sliceTargetType := reflect.TypeOf([]*TestAllTypes{})

	b.Run("TestAllTypes", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := celVal.ConvertToNative(targetType)
			if err != nil {
				b.Fatalf("ConvertToNative failed: %v", err)
			}
		}
	})

	b.Run("SliceTestAllTypes", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := celSliceVal.ConvertToNative(sliceTargetType)
			if err != nil {
				b.Fatalf("ConvertToNative failed: %v", err)
			}
		}
	})
}

// testEnv initializes the test environment common to all tests.
func testNativeEnv(t testing.TB, opts ...any) *cel.Env {
	t.Helper()
	envOpts := []cel.EnvOption{
		cel.Container("types_test"),
		cel.Abbrevs("google.expr.proto3.test"),
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.Variable("tests", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
	}
	nativeOpts := []any{
		reflect.ValueOf(&TestAllTypes{}),
		reflect.ValueOf(&TestRefValFieldType{}),
		reflect.ValueOf(&TestEmbeddedTypes{}),
		reflect.ValueOf(&TestEmbeddedPointerTypes{}),
		reflect.ValueOf(&TestSpecialJSONTags{}),
	}
	for _, o := range opts {
		switch opt := o.(type) {
		case types.NativeTypeOption:
			nativeOpts = append(nativeOpts, opt)
		case cel.EnvOption:
			envOpts = append(envOpts, opt)
		default:
			t.Fatalf("invalid option type: %s", reflect.TypeOf(o).Name())
		}
	}

	envOpts = append(envOpts,
		ext.NativeTypes(
			nativeOpts...,
		),
	)
	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("cel.NewEnv(NativeTypes()) failed: %v", err)
	}
	return env
}

func mustParseTime(t *testing.T, timestamp string) time.Time {
	t.Helper()
	out, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		t.Fatalf("time.Parse(%q) failed: %v", timestamp, err)
	}
	return out
}

type Custom struct {
	Name string `cel:"name"`
}

type TestStructWithMultipleSameNames struct {
	Name       string
	CustomName string `cel:"Name"`
}

type TestNestedType struct {
	NestedListVal    []string
	NestedMapVal     map[int64]bool
	NestedCustomName string `cel:"custom_name" json:"custom_name,omitempty"`
}

type TestAllTypes struct {
	NestedVal       *TestNestedType `json:"nestedVal,omitempty"`
	NestedStructVal TestNestedType  `json:"nestedStructVal,omitempty"`
	BoolVal         bool            `json:"boolVal"`
	BytesVal        []byte
	DurationVal     time.Duration
	DoubleVal       float64
	FloatVal        float32
	Int32Val        int32
	Int64Val        int64
	StringVal       string
	TimestampVal    time.Time
	Uint32Val       uint32
	Uint64Val       uint64
	ListVal         []*TestNestedType
	ArrayVal        [1]*TestNestedType
	BytesArrayVal   [4]byte
	MapVal          map[string]TestAllTypes
	PbVal           *proto3pb.TestAllTypes
	CustomSliceVal  []TestNestedSliceType
	CustomMapVal    map[string]TestMapVal
	CustomName      string `cel:"custom_name"`

	// channel types are not supported
	UnsupportedVal     chan string
	UnsupportedListVal []chan string
	UnsupportedMapVal  map[int]chan string

	// unexported types can be found but not set or accessed
	privateVal map[string]string
}

type TestNestedSliceType struct {
	Value string
}

type TestMapVal struct {
	Value string
}

type TestEmbeddedTypes struct {
	Custom
	TestNestedType `json:"embedded,omitempty"`
	Skipped        string `json:"-"`
}

type TestEmbeddedPointerTypes struct {
	*TestNestedType `json:"embedded,omitempty"`
}

type TestSpecialJSONTags struct {
	Ignored              string `json:"-" cel:"ignored"`
	HyphenName           string `json:"-," cel:"hyphen_name"`
	QuotedHyphen         string `json:"'-'" cel:"quoted_hyphen"`
	Renamed              string `json:"custom_json_name" cel:"renamed"`
	OmitEmptyInt         int    `json:"empty_int,omitempty" cel:"empty_int"`
	OmitEmptyStr         string `json:"empty_str,omitempty" cel:"empty_str"`
	PopulatedInt         int    `json:"pop_int,omitempty" cel:"pop_int"`
	NonOmitEmptyZero     int    `json:"keep_zero" cel:"keep_zero"`
	NonOmitEmptyEmptyStr string `json:"keep_empty" cel:"keep_empty"`
	NonOmitEmptyFalse    bool   `json:"keep_false" cel:"keep_false"`
	DivergentField       string `cel:"cel_field" json:"json_field"`
}

type TestRefValFieldType struct {
	OptionalName *types.Optional `cel:"optional_name"`
	IntVal       types.Int
	CELTime      types.Timestamp `cel:"time"`
}

// registeredNativeStruct is registered with NativeTypes in the delegation test.
type registeredNativeStruct struct {
	Name string
}

// unregisteredNativeStruct is not registered, so NativeToValue should hand it to
// the composed base adapter rather than wrapping it as a native object.
type unregisteredNativeStruct struct {
	Name string
}

// recordingAdapter converts unregisteredNativeStruct into a sentinel string and
// records that it was asked to, so the test can confirm nativeTypeProvider
// delegated the value. Everything else falls through to the base adapter.
type recordingAdapter struct {
	base types.Adapter
	saw  bool
}

func (a *recordingAdapter) NativeToValue(value any) ref.Val {
	if _, ok := value.(unregisteredNativeStruct); ok {
		a.saw = true
		return types.String("from-base-adapter")
	}
	return a.base.NativeToValue(value)
}

func TestNativeTypeAlias(t *testing.T) {
	type CustomStruct struct {
		Name string
	}

	desc := types.NativeTypeFor[CustomStruct](types.NativeTypeAlias("custom.MyStruct"))
	if desc.ReflectType() != reflect.TypeFor[CustomStruct]() {
		t.Fatalf("ReflectType() got %v, wanted %v", desc.ReflectType(), reflect.TypeFor[CustomStruct]())
	}

	nt, err := types.NewNativeType(desc.ReflectType(), desc.Options()...)
	if err != nil {
		t.Fatalf("NewNativeType() failed: %v", err)
	}

	if nt.TypeName() != "custom.MyStruct" {
		t.Errorf("nt.TypeName() got %s, wanted custom.MyStruct", nt.TypeName())
	}
	if nt.String() != "custom.MyStruct" {
		t.Errorf("nt.String() got %s, wanted custom.MyStruct", nt.String())
	}
}

type iterableOnlyWrapper struct {
	ref.Val
	iterable traits.Iterable
}

func (w iterableOnlyWrapper) Iterator() traits.Iterator {
	return w.iterable.Iterator()
}

func BenchmarkListComprehensions(b *testing.B) {
	env := testNativeEnv(b,
		cel.Variable("ptrList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("valList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("strList", cel.ListType(cel.StringType)),
		cel.Variable("intList", cel.ListType(cel.IntType)),
	)
	adapter := env.CELTypeAdapter()

	ptrSlice := make([]*TestAllTypes, 10)
	valSlice := make([]TestAllTypes, 10)
	strSlice := make([]string, 10)
	intSlice := make([]int64, 10)
	for i := 0; i < 10; i++ {
		v := int32(i)
		ptrSlice[i] = &TestAllTypes{Int32Val: v}
		valSlice[i] = TestAllTypes{Int32Val: v}
		strSlice[i] = fmt.Sprintf("item-%d", i)
		intSlice[i] = int64(i * 100)
	}

	ptrLister := types.NewList(adapter, ptrSlice)
	valLister := types.NewList(adapter, valSlice)
	strLister := types.NewList(adapter, strSlice)
	intLister := types.NewList(adapter, intSlice)

	cases := []struct {
		name string
		expr string
		in   map[string]any
	}{
		{
			name: "PtrStruct/Foldable",
			expr: "ptrList.exists(x, x.Int32Val == 9)",
			in:   map[string]any{"ptrList": ptrLister},
		},
		{
			name: "PtrStruct/Iterable",
			expr: "ptrList.exists(x, x.Int32Val == 9)",
			in:   map[string]any{"ptrList": iterableOnlyWrapper{Val: ptrLister, iterable: ptrLister}},
		},
		{
			name: "ValStruct/Foldable",
			expr: "valList.exists(x, x.Int32Val == 9)",
			in:   map[string]any{"valList": valLister},
		},
		{
			name: "ValStruct/Iterable",
			expr: "valList.exists(x, x.Int32Val == 9)",
			in:   map[string]any{"valList": iterableOnlyWrapper{Val: valLister, iterable: valLister}},
		},
		{
			name: "String/Foldable",
			expr: "strList.exists(x, x == 'item-9')",
			in:   map[string]any{"strList": strLister},
		},
		{
			name: "String/Iterable",
			expr: "strList.exists(x, x == 'item-9')",
			in:   map[string]any{"strList": iterableOnlyWrapper{Val: strLister, iterable: strLister}},
		},
		{
			name: "Int64/Foldable",
			expr: "intList.exists(x, x == 900)",
			in:   map[string]any{"intList": intLister},
		},
		{
			name: "Int64/Iterable",
			expr: "intList.exists(x, x == 900)",
			in:   map[string]any{"intList": iterableOnlyWrapper{Val: intLister, iterable: intLister}},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				b.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
			if err != nil {
				b.Fatalf("Program() failed: %v", err)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, _, err := prg.Eval(tc.in)
				if err != nil || out != types.True {
					b.Fatalf("Eval got (%v, %v), want true", out, err)
				}
			}
		})
	}
}

type directIfaceStruct struct {
	Ptr *int32
}

func TestSliceListElemTypePtrAndDirectIface(t *testing.T) {
	v1 := int32(10)
	v2 := int32(20)
	env, err := cel.NewEnv(
		cel.Types(
			reflect.TypeFor[TestAllTypes](),
			reflect.TypeFor[directIfaceStruct](),
		),
		cel.Variable("valList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("directList", cel.ListType(cel.ObjectType("types_test.directIfaceStruct"))),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	adapter := env.CELTypeAdapter()

	valSlice := []TestAllTypes{
		{Int32Val: 10, StringVal: "first"},
		{Int32Val: 20, StringVal: "second"},
	}
	valLister := types.NewList(adapter, valSlice)

	// 1. Verify Get(0).Value() returns a copy and mutating it does not mutate the backing slice.
	elem0 := valLister.Get(types.Int(0)).Value().(TestAllTypes)
	elem0.Int32Val = 999
	if valSlice[0].Int32Val != 10 {
		t.Errorf("mutating Get(0).Value() modified backing slice: got %d, want 10", valSlice[0].Int32Val)
	}

	// 2. Verify Contains and Equal on value-struct sliceList.
	if valLister.Contains(adapter.NativeToValue(TestAllTypes{Int32Val: 20, StringVal: "second"})) != types.True {
		t.Errorf("valLister.Contains() got false, want true")
	}
	valListerCopy := types.NewList(adapter, []TestAllTypes{
		{Int32Val: 10, StringVal: "first"},
		{Int32Val: 20, StringVal: "second"},
	})
	if valLister.Equal(valListerCopy) != types.True {
		t.Errorf("valLister.Equal(valListerCopy) got false, want true")
	}

	// 3. Verify filter comprehension returning value structs and ConvertToNative back to []TestAllTypes.
	ast, iss := env.Compile("valList.filter(x, x.Int32Val > 15)")
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"valList": valLister})
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	filteredNative, err := out.ConvertToNative(reflect.TypeFor[[]TestAllTypes]())
	if err != nil {
		t.Fatalf("ConvertToNative([]TestAllTypes) failed: %v", err)
	}
	filteredSlice := filteredNative.([]TestAllTypes)
	if len(filteredSlice) != 1 || filteredSlice[0].Int32Val != 20 || filteredSlice[0].StringVal != "second" {
		t.Errorf("filteredSlice got %+v, want [{Int32Val:20 StringVal:second}]", filteredSlice)
	}
	// Mutating filteredSlice must not affect original valSlice.
	filteredSlice[0].Int32Val = 777
	if valSlice[1].Int32Val != 20 {
		t.Errorf("mutating filteredSlice modified original valSlice: got %d, want 20", valSlice[1].Int32Val)
	}

	// 4. Verify direct-interface struct (single pointer field, isDirectIface == true).
	directSlice := []directIfaceStruct{{Ptr: &v1}, {Ptr: &v2}}
	directLister := types.NewList(adapter, directSlice)
	astDirect, iss := env.Compile("directList.exists(d, d.Ptr == 20)")
	if iss.Err() != nil {
		t.Fatalf("Compile directList failed: %v", iss.Err())
	}
	prgDirect, err := env.Program(astDirect)
	if err != nil {
		t.Fatalf("Program directList failed: %v", err)
	}
	outDirect, _, err := prgDirect.Eval(map[string]any{"directList": directLister})
	if err != nil || outDirect != types.True {
		t.Errorf("directList.exists got (%v, %v), want true", outDirect, err)
	}

	// 5. Verify zero-sized struct slice []struct{}.
	emptyStructLister := types.NewList(adapter, []struct{}{{}, {}})
	if emptyStructLister.Size() != types.Int(2) {
		t.Errorf("emptyStructLister.Size() got %v, want 2", emptyStructLister.Size())
	}
}

type iterableOnlyMapper struct {
	traits.Mapper
}

func (m iterableOnlyMapper) FindStringKey(s string) (any, bool) {
	if nm, ok := m.Mapper.(interface{ FindStringKey(string) (any, bool) }); ok {
		return nm.FindStringKey(s)
	}
	return nil, false
}

func (m iterableOnlyMapper) FindInt64Key(ik int64) (any, bool) {
	if nm, ok := m.Mapper.(interface{ FindInt64Key(int64) (any, bool) }); ok {
		return nm.FindInt64Key(ik)
	}
	return nil, false
}

func (m iterableOnlyMapper) FindNative(key any) (any, bool) {
	if nm, ok := m.Mapper.(interface{ FindNative(any) (any, bool) }); ok {
		return nm.FindNative(key)
	}
	return nil, false
}

func TestNativeMapGeneric(t *testing.T) {
	env := testNativeEnv(
		t,
		cel.OptionalTypes(),
		ext.TwoVarComprehensions(),
		cel.Variable("valMap", cel.MapType(cel.StringType, cel.ObjectType("types_test.TestAllTypes"))),
	)
	adapter := env.CELTypeAdapter()

	valMap := map[string]TestAllTypes{
		"first":  {Int32Val: 10, StringVal: "alpha"},
		"second": {Int32Val: 20, StringVal: "beta"},
	}
	mapper := types.NewMap(adapter, valMap)

	// 1. Verify Get returns an independent copy (does not mutate backing map).
	got := mapper.Get(types.String("first"))
	if types.IsError(got) {
		t.Fatalf("mapper.Get('first') failed: %v", got)
	}
	gotStruct, ok := got.Value().(TestAllTypes)
	if !ok {
		t.Fatalf("mapper.Get('first').Value() got type %T, want TestAllTypes", got.Value())
	}
	gotStruct.Int32Val = 999
	if valMap["first"].Int32Val != 10 {
		t.Errorf("mutating Get('first').Value() modified backing map: got %d, want 10", valMap["first"].Int32Val)
	}

	// 2. Verify Contains and Equal.
	if mapper.Contains(types.String("second")) != types.True {
		t.Errorf("mapper.Contains('second') got false, want true")
	}
	mapperCopy := types.NewMap(adapter, map[string]TestAllTypes{
		"first":  {Int32Val: 10, StringVal: "alpha"},
		"second": {Int32Val: 20, StringVal: "beta"},
	})
	if mapper.Equal(mapperCopy) != types.True {
		t.Errorf("mapper.Equal(mapperCopy) got false, want true")
	}

	// 3. Verify 1-variable comprehension (FoldKeyOnly fast-path).
	ast1, iss := env.Compile("valMap.exists(k, k == 'second')")
	if iss.Err() != nil {
		t.Fatalf("Compile 1-var failed: %v", iss.Err())
	}
	prg1, err := env.Program(ast1)
	if err != nil {
		t.Fatalf("Program 1-var failed: %v", err)
	}
	out1, _, err := prg1.Eval(map[string]any{"valMap": mapper})
	if err != nil || out1 != types.True {
		t.Errorf("1-var exists got (%v, %v), want true", out1, err)
	}

	// 4. Verify 2-variable comprehension (zero-copy valTypePtr fast-path).
	ast2, iss := env.Compile("valMap.exists(k, v, k == 'second' && v.Int32Val == 20)")
	if iss.Err() != nil {
		t.Fatalf("Compile 2-var failed: %v", iss.Err())
	}
	prg2, err := env.Program(ast2)
	if err != nil {
		t.Fatalf("Program 2-var failed: %v", err)
	}
	out2, _, err := prg2.Eval(map[string]any{"valMap": mapper})
	if err != nil || out2 != types.True {
		t.Errorf("2-var exists got (%v, %v), want true", out2, err)
	}

	// 5. Verify transformMap returning value structs and ConvertToNative back to map[string]TestAllTypes
	// ensuring no stack aliasing across loop iterations.
	astTrans, iss := env.Compile("valMap.transformMap(k, v, v.Int32Val > 15, v)")
	if iss.Err() != nil {
		t.Fatalf("Compile transformMap failed: %v", iss.Err())
	}
	prgTrans, err := env.Program(astTrans)
	if err != nil {
		t.Fatalf("Program transformMap failed: %v", err)
	}
	outTrans, _, err := prgTrans.Eval(map[string]any{"valMap": mapper})
	if err != nil {
		t.Fatalf("Eval transformMap failed: %v", err)
	}
	transNative, err := outTrans.ConvertToNative(reflect.TypeFor[map[string]TestAllTypes]())
	if err != nil {
		t.Fatalf("ConvertToNative(map[string]TestAllTypes) failed: %v", err)
	}
	transMap := transNative.(map[string]TestAllTypes)
	if len(transMap) != 1 || transMap["second"].Int32Val != 20 || transMap["second"].StringVal != "beta" {
		t.Errorf("transMap got %+v, want map[second:{Int32Val:20 StringVal:beta}]", transMap)
	}
}

func BenchmarkMapComprehensions(b *testing.B) {
	env := testNativeEnv(
		b,
		cel.OptionalTypes(),
		ext.TwoVarComprehensions(),
		cel.Variable("ptrMap", cel.MapType(cel.StringType, cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("valMap", cel.MapType(cel.StringType, cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("strMap", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("intMap", cel.MapType(cel.IntType, cel.BoolType)),
	)
	adapter := env.CELTypeAdapter()

	ptrMap := make(map[string]*TestAllTypes, 10)
	valMap := make(map[string]TestAllTypes, 10)
	strMap := make(map[string]string, 10)
	intMap := make(map[int64]bool, 10)
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("k%d", i)
		v := fmt.Sprintf("v%d", i)
		ptrMap[k] = &TestAllTypes{Int32Val: int32(i), StringVal: v}
		valMap[k] = TestAllTypes{Int32Val: int32(i), StringVal: v}
		strMap[k] = v
		intMap[int64(i)] = (i == 9)
	}

	ptrMapper := types.NewMap(adapter, ptrMap)
	valMapper := types.NewMap(adapter, valMap)
	strMapper := types.NewMap(adapter, strMap)
	intMapper := types.NewMap(adapter, intMap)

	ptrLegacy := types.NewDynamicMap(adapter, ptrMap)
	valLegacy := types.NewDynamicMap(adapter, valMap)
	strLegacy := types.NewDynamicMap(adapter, strMap)
	intLegacy := types.NewDynamicMap(adapter, intMap)

	compileBench := func(expr string) cel.Program {
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			b.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
		if err != nil {
			b.Fatalf("Program(%q) failed: %v", expr, err)
		}
		return prg
	}

	prgPtr1Var := compileBench("ptrMap.exists(k, ptrMap[k].Int32Val == 9)")
	prgPtr2Var := compileBench("ptrMap.exists(k, v, v.Int32Val == 9)")
	prgVal1Var := compileBench("valMap.exists(k, valMap[k].Int32Val == 9)")
	prgVal2Var := compileBench("valMap.exists(k, v, v.Int32Val == 9)")
	prgStr1Var := compileBench("strMap.exists(k, k == 'k9')")
	prgStr2Var := compileBench("strMap.exists(k, v, v == 'v9')")
	prgInt1Var := compileBench("intMap.exists(k, intMap[k])")
	prgInt2Var := compileBench("intMap.exists(k, v, v)")

	cases := []struct {
		name string
		prg  cel.Program
		vars map[string]any
	}{
		{"PtrStruct_1Var_LegacyReflect", prgPtr1Var, map[string]any{"ptrMap": ptrLegacy}},
		{"PtrStruct_1Var_Iterable", prgPtr1Var, map[string]any{"ptrMap": iterableOnlyMapper{ptrMapper}}},
		{"PtrStruct_1Var_Foldable", prgPtr1Var, map[string]any{"ptrMap": ptrMapper}},
		{"PtrStruct_2Var_LegacyReflect", prgPtr2Var, map[string]any{"ptrMap": ptrLegacy}},
		{"PtrStruct_2Var_Foldable", prgPtr2Var, map[string]any{"ptrMap": ptrMapper}},
		{"ValStruct_1Var_LegacyReflect", prgVal1Var, map[string]any{"valMap": valLegacy}},
		{"ValStruct_1Var_Iterable", prgVal1Var, map[string]any{"valMap": iterableOnlyMapper{valMapper}}},
		{"ValStruct_1Var_Foldable", prgVal1Var, map[string]any{"valMap": valMapper}},
		{"ValStruct_2Var_LegacyReflect", prgVal2Var, map[string]any{"valMap": valLegacy}},
		{"ValStruct_2Var_Foldable", prgVal2Var, map[string]any{"valMap": valMapper}},
		{"StringMap_1Var_LegacyReflect", prgStr1Var, map[string]any{"strMap": strLegacy}},
		{"StringMap_1Var_Iterable", prgStr1Var, map[string]any{"strMap": iterableOnlyMapper{strMapper}}},
		{"StringMap_1Var_Foldable", prgStr1Var, map[string]any{"strMap": strMapper}},
		{"StringMap_2Var_LegacyReflect", prgStr2Var, map[string]any{"strMap": strLegacy}},
		{"StringMap_2Var_Foldable", prgStr2Var, map[string]any{"strMap": strMapper}},
		{"Int64Bool_1Var_LegacyReflect", prgInt1Var, map[string]any{"intMap": intLegacy}},
		{"Int64Bool_1Var_Iterable", prgInt1Var, map[string]any{"intMap": iterableOnlyMapper{intMapper}}},
		{"Int64Bool_1Var_Foldable", prgInt1Var, map[string]any{"intMap": intMapper}},
		{"Int64Bool_2Var_LegacyReflect", prgInt2Var, map[string]any{"intMap": intLegacy}},
		{"Int64Bool_2Var_Foldable", prgInt2Var, map[string]any{"intMap": intMapper}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, _, err := tc.prg.Eval(tc.vars)
				if err != nil || out != types.True {
					b.Fatalf("Eval got (%v, %v), want true", out, err)
				}
			}
		})
	}
}

func BenchmarkMapLookup(b *testing.B) {
	env := testNativeEnv(
		b,
		cel.Variable("ptrMap", cel.MapType(cel.StringType, cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("valMap", cel.MapType(cel.StringType, cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("strMap", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("intMap", cel.MapType(cel.IntType, cel.BoolType)),
	)
	adapter := env.CELTypeAdapter()

	ptrMap := map[string]*TestAllTypes{"k9": {Int32Val: 9}}
	valMap := map[string]TestAllTypes{"k9": {Int32Val: 9}}
	strMap := map[string]string{"k9": "v9"}
	intMap := map[int64]bool{9: true}

	ptrMapper := types.NewMap(adapter, ptrMap)
	valMapper := types.NewMap(adapter, valMap)
	strMapper := types.NewMap(adapter, strMap)
	intMapper := types.NewMap(adapter, intMap)

	ptrLegacy := types.NewDynamicMap(adapter, ptrMap)
	valLegacy := types.NewDynamicMap(adapter, valMap)
	strLegacy := types.NewDynamicMap(adapter, strMap)
	intLegacy := types.NewDynamicMap(adapter, intMap)

	compileBench := func(expr string) cel.Program {
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			b.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
		if err != nil {
			b.Fatalf("Program(%q) failed: %v", expr, err)
		}
		return prg
	}

	prgPtr := compileBench("ptrMap['k9'].Int32Val == 9")
	prgVal := compileBench("valMap['k9'].Int32Val == 9")
	prgStr := compileBench("strMap['k9'] == 'v9'")
	prgInt := compileBench("intMap[9] == true")

	cases := []struct {
		name string
		prg  cel.Program
		vars map[string]any
	}{
		{"PtrStruct_LegacyReflect", prgPtr, map[string]any{"ptrMap": ptrLegacy}},
		{"PtrStruct_NewMap", prgPtr, map[string]any{"ptrMap": ptrMapper}},
		{"ValStruct_LegacyReflect", prgVal, map[string]any{"valMap": valLegacy}},
		{"ValStruct_NewMap", prgVal, map[string]any{"valMap": valMapper}},
		{"StringMap_LegacyReflect", prgStr, map[string]any{"strMap": strLegacy}},
		{"StringMap_NewMap", prgStr, map[string]any{"strMap": strMapper}},
		{"Int64Bool_LegacyReflect", prgInt, map[string]any{"intMap": intLegacy}},
		{"Int64Bool_NewMap", prgInt, map[string]any{"intMap": intMapper}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, _, err := tc.prg.Eval(tc.vars)
				if err != nil || out != types.True {
					b.Fatalf("Eval got (%v, %v), want true", out, err)
				}
			}
		})
	}
}

func TestNewDynamicListAdaptations(t *testing.T) {
	env := testNativeEnv(
		t,
		cel.Variable("ptrList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("valList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("entity", cel.ObjectType("types_test.TestAllTypes")),
	)
	adapter := env.CELTypeAdapter()

	ptrSlice := []*TestAllTypes{{Int32Val: 1}, {Int32Val: 2}, {Int32Val: 9}}
	valSlice := []TestAllTypes{{Int32Val: 1}, {Int32Val: 2}, {Int32Val: 9}}
	entity := &TestAllTypes{
		ListVal: []*TestNestedType{
			{NestedCustomName: "a"},
			{NestedCustomName: "b"},
			{NestedCustomName: "target"},
		},
		CustomSliceVal: []TestNestedSliceType{
			{Value: "x"},
			{Value: "y"},
			{Value: "target"},
		},
	}

	vars := map[string]any{
		"ptrList": types.NewDynamicList(adapter, ptrSlice),
		"valList": types.NewDynamicList(adapter, valSlice),
		"entity":  entity,
	}

	exprs := []string{
		"ptrList.exists(x, x.Int32Val == 9)",
		"ptrList[2].Int32Val == 9",
		"valList.exists(x, x.Int32Val == 9)",
		"valList[2].Int32Val == 9",
		"entity.ListVal.exists(x, x.NestedCustomName == 'target')",
		"entity.ListVal[2].NestedCustomName == 'target'",
		"entity.CustomSliceVal.exists(x, x.Value == 'target')",
		"entity.CustomSliceVal[2].Value == 'target'",
	}

	for _, expr := range exprs {
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", expr, err)
		}
		out, _, err := prg.Eval(vars)
		if err != nil || out != types.True {
			t.Fatalf("Eval(%q) got (%v, %v), want true", expr, out, err)
		}
	}
}

func BenchmarkNewDynamicList(b *testing.B) {
	env := testNativeEnv(
		b,
		cel.Variable("ptrList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("valList", cel.ListType(cel.ObjectType("types_test.TestAllTypes"))),
		cel.Variable("strList", cel.ListType(cel.StringType)),
		cel.Variable("entity", cel.ObjectType("types_test.TestAllTypes")),
	)
	adapter := env.CELTypeAdapter()

	var ptrArr [10]*TestAllTypes
	var valArr [10]TestAllTypes
	var strArr [10]string
	ptrSlice := make([]*TestAllTypes, 10)
	valSlice := make([]TestAllTypes, 10)
	strSlice := make([]string, 10)
	nestedPtrs := make([]*TestNestedType, 10)
	nestedVals := make([]TestNestedSliceType, 10)

	for i := 0; i < 10; i++ {
		s := fmt.Sprintf("item-%d", i)
		tVal := TestAllTypes{Int32Val: int32(i), StringVal: s}
		valArr[i] = tVal
		valSlice[i] = tVal
		ptrArr[i] = &valSlice[i]
		ptrSlice[i] = &valSlice[i]
		strArr[i] = s
		strSlice[i] = s
		nestedPtrs[i] = &TestNestedType{NestedCustomName: s}
		nestedVals[i] = TestNestedSliceType{Value: s}
	}

	entity := &TestAllTypes{
		ListVal:        nestedPtrs,
		CustomSliceVal: nestedVals,
	}

	// Compare legacy reflection-backed baseList (NewLegacyDynamicList) against
	// the adapted NewDynamicList (backed by NewList / sliceList) on identical slices.
	ptrSliceList := types.NewDynamicList(adapter, ptrSlice)
	valSliceList := types.NewDynamicList(adapter, valSlice)
	strSliceList := types.NewDynamicList(adapter, strSlice)

	compileBench := func(expr string) cel.Program {
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			b.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
		if err != nil {
			b.Fatalf("Program(%q) failed: %v", expr, err)
		}
		return prg
	}

	prgPtrFold := compileBench("ptrList.exists(x, x.Int32Val == 9)")
	prgPtrIter := compileBench("ptrList.all(x, x.Int32Val >= 0 && x.Int32Val < 10)")
	prgPtrIdx := compileBench("ptrList[9].Int32Val == 9")

	prgValFold := compileBench("valList.exists(x, x.Int32Val == 9)")
	prgValIter := compileBench("valList.all(x, x.Int32Val >= 0 && x.Int32Val < 10)")
	prgValIdx := compileBench("valList[9].Int32Val == 9")

	prgStrFold := compileBench("strList.exists(x, x == 'item-9')")

	prgFieldPtrFold := compileBench("entity.ListVal.exists(x, x.NestedCustomName == 'item-9')")
	prgFieldPtrIdx := compileBench("entity.ListVal[9].NestedCustomName == 'item-9'")
	prgFieldValFold := compileBench("entity.CustomSliceVal.exists(x, x.Value == 'item-9')")
	prgFieldValIdx := compileBench("entity.CustomSliceVal[9].Value == 'item-9'")

	cases := []struct {
		name string
		prg  cel.Program
		vars map[string]any
	}{
		{"PtrStruct_Fold_NewDynamicList", prgPtrFold, map[string]any{"ptrList": ptrSliceList}},
		{"PtrStruct_Iter_NewDynamicList", prgPtrIter, map[string]any{"ptrList": ptrSliceList}},
		{"PtrStruct_Index_NewDynamicList", prgPtrIdx, map[string]any{"ptrList": ptrSliceList}},
		{"ValStruct_Fold_NewDynamicList", prgValFold, map[string]any{"valList": valSliceList}},
		{"ValStruct_Iter_NewDynamicList", prgValIter, map[string]any{"valList": valSliceList}},
		{"ValStruct_Index_NewDynamicList", prgValIdx, map[string]any{"valList": valSliceList}},
		{"String_Fold_NewDynamicList", prgStrFold, map[string]any{"strList": strSliceList}},
		{"StructField_PtrSlice_Fold", prgFieldPtrFold, map[string]any{"entity": entity}},
		{"StructField_PtrSlice_Index", prgFieldPtrIdx, map[string]any{"entity": entity}},
		{"StructField_ValSlice_Fold", prgFieldValFold, map[string]any{"entity": entity}},
		{"StructField_ValSlice_Index", prgFieldValIdx, map[string]any{"entity": entity}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, _, err := tc.prg.Eval(tc.vars)
				if err != nil || out != types.True {
					b.Fatalf("Eval got (%v, %v), want true", out, err)
				}
			}
		})
	}
	b.Run("ConstructAndFold_PtrSlice_NewDynamicList", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			l := types.NewDynamicList(adapter, ptrSlice)
			out, _, err := prgPtrFold.Eval(map[string]any{"ptrList": l})
			if err != nil || out != types.True {
				b.Fatalf("Eval got (%v, %v), want true", out, err)
			}
		}
	})
	b.Run("ConstructAndFold_ValSlice_NewDynamicList", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			l := types.NewDynamicList(adapter, valSlice)
			out, _, err := prgValFold.Eval(map[string]any{"valList": l})
			if err != nil || out != types.True {
				b.Fatalf("Eval got (%v, %v), want true", out, err)
			}
		}
	})
}

type TestNativePointerFields struct {
	BoolPtr      *bool
	DurationPtr  *time.Duration
	IntPtr       *int
	Int32Ptr     *int32
	Int64Ptr     *int64
	UintPtr      *uint
	Uint32Ptr    *uint32
	Uint64Ptr    *uint64
	Float32Ptr   *float32
	Float64Ptr   *float64
	StringPtr    *string
	TimestampPtr *time.Time
	StructPtr    *TestNestedType
}

type TestNativeSliceAndMapFields struct {
	ByteSlice        []byte
	StringSlice      []string
	IntSlice         []int
	Int32Slice       []int32
	Int64Slice       []int64
	UintSlice        []uint
	Uint32Slice      []uint32
	Uint64Slice      []uint64
	Float32Slice     []float32
	Float64Slice     []float64
	BoolSlice        []bool
	TimeSlice        []time.Time
	DurationSlice    []time.Duration
	DynamicPtrSlice  []*TestNestedType
	DynamicValSlice  []TestNestedSliceType
	StringStringMap  map[string]string
	Int64BoolMap     map[int64]bool
	StringInt64Map   map[string]int64
	StringIntMap     map[string]int
	StringBoolMap    map[string]bool
	StringFloat64Map map[string]float64
}

type TestNativePrimitivesAndFallbacks struct {
	IntVal      int
	Int8Val     int8
	Int16Val    int16
	Int32Val    int32
	Int64Val    int64
	UintVal     uint
	Uint8Val    uint8
	Uint16Val   uint16
	Uint32Val   uint32
	Uint64Val   uint64
	Float32Val  float32
	Float64Val  float64
	BoolVal     bool
	StringVal   string
	DurationVal time.Duration
	TimeVal     time.Time
	NestedVal   TestNestedType
	Int8Ptr     *int8
	Int16Ptr    *int16
	Uint8Ptr    *uint8
	Uint16Ptr   *uint16
}

func TestNativePointerFieldsCoverage(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Types(reflect.TypeFor[TestNativePointerFields]()),
		cel.Variable("ptrObj", cel.ObjectType("types_test.TestNativePointerFields")),
		cel.Variable("valObj", cel.ObjectType("types_test.TestNativePointerFields")),
		cel.Variable("nilFieldsObj", cel.ObjectType("types_test.TestNativePointerFields")),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	bVal := true
	dVal := 10 * time.Second
	iVal := 42
	i32Val := int32(4232)
	i64Val := int64(4264)
	uVal := uint(100)
	u32Val := uint32(10032)
	u64Val := uint64(10064)
	f32Val := float32(3.14)
	f64Val := float64(6.28)
	sVal := "hello pointer"
	tVal := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	nestedVal := &TestNestedType{NestedCustomName: "nested-ptr"}

	populated := TestNativePointerFields{
		BoolPtr:      &bVal,
		DurationPtr:  &dVal,
		IntPtr:       &iVal,
		Int32Ptr:     &i32Val,
		Int64Ptr:     &i64Val,
		UintPtr:      &uVal,
		Uint32Ptr:    &u32Val,
		Uint64Ptr:    &u64Val,
		Float32Ptr:   &f32Val,
		Float64Ptr:   &f64Val,
		StringPtr:    &sVal,
		TimestampPtr: &tVal,
		StructPtr:    nestedVal,
	}
	empty := TestNativePointerFields{}

	vars := map[string]any{
		"ptrObj":       &populated,
		"valObj":       populated,
		"nilFieldsObj": &empty,
	}

	tests := []struct {
		expr string
		want any
	}{
		// Pointer receiver populated
		{"ptrObj.BoolPtr", true},
		{"ptrObj.DurationPtr", 10 * time.Second},
		{"ptrObj.IntPtr", int64(42)},
		{"ptrObj.Int32Ptr", int64(4232)},
		{"ptrObj.Int64Ptr", int64(4264)},
		{"ptrObj.UintPtr", uint64(100)},
		{"ptrObj.Uint32Ptr", uint64(10032)},
		{"ptrObj.Uint64Ptr", uint64(10064)},
		{"ptrObj.Float32Ptr > 3.13 && ptrObj.Float32Ptr < 3.15", true},
		{"ptrObj.Float64Ptr", 6.28},
		{"ptrObj.StringPtr", "hello pointer"},
		{"ptrObj.TimestampPtr == timestamp('2026-09-25T12:00:00Z')", true},
		{"ptrObj.StructPtr.NestedCustomName", "nested-ptr"},
		{"has(ptrObj.BoolPtr)", true},
		{"has(ptrObj.DurationPtr)", true},
		{"has(ptrObj.IntPtr)", true},
		{"has(ptrObj.Int32Ptr)", true},
		{"has(ptrObj.Int64Ptr)", true},
		{"has(ptrObj.UintPtr)", true},
		{"has(ptrObj.Uint32Ptr)", true},
		{"has(ptrObj.Uint64Ptr)", true},
		{"has(ptrObj.Float32Ptr)", true},
		{"has(ptrObj.Float64Ptr)", true},
		{"has(ptrObj.StringPtr)", true},
		{"has(ptrObj.TimestampPtr)", true},
		{"has(ptrObj.StructPtr)", true},

		// Value receiver populated
		{"valObj.BoolPtr", true},
		{"valObj.DurationPtr", 10 * time.Second},
		{"valObj.IntPtr", int64(42)},
		{"valObj.Int32Ptr", int64(4232)},
		{"valObj.Int64Ptr", int64(4264)},
		{"valObj.UintPtr", uint64(100)},
		{"valObj.Uint32Ptr", uint64(10032)},
		{"valObj.Uint64Ptr", uint64(10064)},
		{"valObj.Float32Ptr > 3.13 && valObj.Float32Ptr < 3.15", true},
		{"valObj.Float64Ptr", 6.28},
		{"valObj.StringPtr", "hello pointer"},
		{"valObj.TimestampPtr == timestamp('2026-09-25T12:00:00Z')", true},
		{"has(valObj.BoolPtr)", true},

		// Nil pointers
		{"nilFieldsObj.BoolPtr", false},
		{"nilFieldsObj.DurationPtr", time.Duration(0)},
		{"nilFieldsObj.IntPtr", int64(0)},
		{"nilFieldsObj.Int32Ptr", int64(0)},
		{"nilFieldsObj.Int64Ptr", int64(0)},
		{"nilFieldsObj.UintPtr", uint64(0)},
		{"nilFieldsObj.Uint32Ptr", uint64(0)},
		{"nilFieldsObj.Uint64Ptr", uint64(0)},
		{"nilFieldsObj.Float32Ptr", 0.0},
		{"nilFieldsObj.Float64Ptr", 0.0},
		{"nilFieldsObj.StringPtr", ""},
		{"nilFieldsObj.TimestampPtr == timestamp('1970-01-01T00:00:00Z')", true},
		{"has(nilFieldsObj.BoolPtr)", false},
		{"has(nilFieldsObj.DurationPtr)", false},
		{"has(nilFieldsObj.IntPtr)", false},
		{"has(nilFieldsObj.Int32Ptr)", false},
		{"has(nilFieldsObj.Int64Ptr)", false},
		{"has(nilFieldsObj.UintPtr)", false},
		{"has(nilFieldsObj.Uint32Ptr)", false},
		{"has(nilFieldsObj.Uint64Ptr)", false},
		{"has(nilFieldsObj.Float32Ptr)", false},
		{"has(nilFieldsObj.Float64Ptr)", false},
		{"has(nilFieldsObj.StringPtr)", false},
		{"has(nilFieldsObj.TimestampPtr)", false},
		{"has(nilFieldsObj.StructPtr)", false},
	}

	for _, tc := range tests {
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", tc.expr, err)
		}
		out, _, err := prg.Eval(vars)
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		adapter := env.CELTypeAdapter()
		wantVal := adapter.NativeToValue(tc.want)
		if out.Equal(wantVal) != types.True {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, out, out, wantVal, wantVal)
		}
	}

	nt, err := types.NewNativeType(reflect.TypeFor[TestNativePointerFields]())
	if err != nil {
		t.Fatalf("NewNativeType failed: %v", err)
	}
	ptrFieldNames := []string{
		"BoolPtr", "DurationPtr", "IntPtr", "Int32Ptr", "Int64Ptr",
		"UintPtr", "Uint32Ptr", "Uint64Ptr", "Float32Ptr", "Float64Ptr",
		"StringPtr", "TimestampPtr", "StructPtr",
	}
	for _, name := range ptrFieldNames {
		f, ok := nt.FindFieldType(name)
		if !ok {
			t.Fatalf("FindFieldType(%q) failed", name)
		}
		val, err := f.GetFrom("not-a-struct")
		if err != nil || val != nil {
			t.Errorf("GetFrom('not-a-struct') for %q got (%v, %v), want (nil, nil)", name, val, err)
		}
	}
}

func TestNativeSliceAndMapFieldsCoverage(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Types(
			reflect.TypeFor[TestNativeSliceAndMapFields](),
			reflect.TypeFor[TestNestedType](),
			reflect.TypeFor[TestNestedSliceType](),
		),
		cel.Variable("ptrObj", cel.ObjectType("types_test.TestNativeSliceAndMapFields")),
		cel.Variable("valObj", cel.ObjectType("types_test.TestNativeSliceAndMapFields")),
		cel.Variable("emptyObj", cel.ObjectType("types_test.TestNativeSliceAndMapFields")),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	populated := TestNativeSliceAndMapFields{
		ByteSlice:        []byte("bytes"),
		StringSlice:      []string{"a", "b"},
		IntSlice:         []int{1, 2, 3},
		Int32Slice:       []int32{10, 20},
		Int64Slice:       []int64{100, 200},
		UintSlice:        []uint{1, 2},
		Uint32Slice:      []uint32{10, 20},
		Uint64Slice:      []uint64{100, 200},
		Float32Slice:     []float32{1.5, 2.5},
		Float64Slice:     []float64{10.5, 20.5},
		BoolSlice:        []bool{true, false},
		TimeSlice:        []time.Time{now},
		DurationSlice:    []time.Duration{time.Second, 2 * time.Second},
		DynamicPtrSlice:  []*TestNestedType{{NestedCustomName: "dyn-ptr"}},
		DynamicValSlice:  []TestNestedSliceType{{Value: "dyn-val"}},
		StringStringMap:  map[string]string{"k": "v"},
		Int64BoolMap:     map[int64]bool{7: true},
		StringInt64Map:   map[string]int64{"si64": 64},
		StringIntMap:     map[string]int{"si": 32},
		StringBoolMap:    map[string]bool{"sb": true},
		StringFloat64Map: map[string]float64{"sf": 3.14},
	}
	empty := TestNativeSliceAndMapFields{}

	vars := map[string]any{
		"ptrObj":   &populated,
		"valObj":   populated,
		"emptyObj": &empty,
	}

	tests := []struct {
		expr string
		want any
	}{
		{"ptrObj.ByteSlice", []byte("bytes")},
		{"ptrObj.StringSlice[0]", "a"},
		{"ptrObj.IntSlice.size()", int64(3)},
		{"ptrObj.Int32Slice[1]", int64(20)},
		{"ptrObj.Int64Slice[0]", int64(100)},
		{"ptrObj.UintSlice[0]", uint64(1)},
		{"ptrObj.Uint32Slice[1]", uint64(20)},
		{"ptrObj.Uint64Slice[0]", uint64(100)},
		{"ptrObj.Float32Slice[0]", 1.5},
		{"ptrObj.Float64Slice[1]", 20.5},
		{"ptrObj.BoolSlice[0]", true},
		{"ptrObj.TimeSlice[0] == timestamp('2026-09-25T00:00:00Z')", true},
		{"ptrObj.DurationSlice[0]", time.Second},
		{"ptrObj.DynamicPtrSlice[0].NestedCustomName", "dyn-ptr"},
		{"ptrObj.DynamicValSlice[0].Value", "dyn-val"},
		{"ptrObj.StringStringMap['k']", "v"},
		{"ptrObj.Int64BoolMap[7]", true},
		{"ptrObj.StringInt64Map['si64']", int64(64)},
		{"ptrObj.StringIntMap['si']", int64(32)},
		{"ptrObj.StringBoolMap['sb']", true},
		{"ptrObj.StringFloat64Map['sf']", 3.14},
		{"has(ptrObj.StringSlice)", true},
		{"has(ptrObj.StringStringMap)", true},

		// Value receiver
		{"valObj.StringSlice[1]", "b"},
		{"valObj.DynamicPtrSlice[0].NestedCustomName", "dyn-ptr"},
		{"valObj.DynamicValSlice[0].Value", "dyn-val"},
		{"valObj.StringIntMap['si']", int64(32)},
		{"valObj.StringFloat64Map['sf']", 3.14},

		// Empty / Nil
		{"has(emptyObj.StringSlice)", false},
		{"has(emptyObj.StringStringMap)", false},
		{"emptyObj.DynamicPtrSlice.size()", int64(0)},
		{"emptyObj.DynamicValSlice.size()", int64(0)},
	}

	for _, tc := range tests {
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", tc.expr, err)
		}
		out, _, err := prg.Eval(vars)
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		adapter := env.CELTypeAdapter()
		wantVal := adapter.NativeToValue(tc.want)
		if out.Equal(wantVal) != types.True {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, out, out, wantVal, wantVal)
		}
	}
}

func TestNativePrimitivesAndFallbacksCoverage(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Types(reflect.TypeFor[TestNativePrimitivesAndFallbacks]()),
		cel.Variable("ptrObj", cel.ObjectType("types_test.TestNativePrimitivesAndFallbacks")),
		cel.Variable("valObj", cel.ObjectType("types_test.TestNativePrimitivesAndFallbacks")),
		cel.Variable("zeroObj", cel.ObjectType("types_test.TestNativePrimitivesAndFallbacks")),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	i8 := int8(8)
	i16 := int16(16)
	u8 := uint8(88)
	u16 := uint16(166)

	populated := TestNativePrimitivesAndFallbacks{
		IntVal:      10,
		Int8Val:     8,
		Int16Val:    16,
		Int32Val:    32,
		Int64Val:    64,
		UintVal:     20,
		Uint8Val:    88,
		Uint16Val:   166,
		Uint32Val:   320,
		Uint64Val:   640,
		Float32Val:  1.25,
		Float64Val:  2.5,
		BoolVal:     true,
		StringVal:   "primitive",
		DurationVal: 3 * time.Second,
		TimeVal:     time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
		NestedVal:   TestNestedType{NestedCustomName: "embedded-nested"},
		Int8Ptr:     &i8,
		Int16Ptr:    &i16,
		Uint8Ptr:    &u8,
		Uint16Ptr:   &u16,
	}
	zero := TestNativePrimitivesAndFallbacks{}

	vars := map[string]any{
		"ptrObj":  &populated,
		"valObj":  populated,
		"zeroObj": &zero,
	}

	tests := []struct {
		expr string
		want any
	}{
		{"ptrObj.IntVal", int64(10)},
		{"ptrObj.Int8Val", int64(8)},
		{"ptrObj.Int16Val", int64(16)},
		{"ptrObj.Int32Val", int64(32)},
		{"ptrObj.Int64Val", int64(64)},
		{"ptrObj.UintVal", uint64(20)},
		{"ptrObj.Uint8Val", uint64(88)},
		{"ptrObj.Uint16Val", uint64(166)},
		{"ptrObj.Uint32Val", uint64(320)},
		{"ptrObj.Uint64Val", uint64(640)},
		{"ptrObj.Float32Val", 1.25},
		{"ptrObj.Float64Val", 2.5},
		{"ptrObj.BoolVal", true},
		{"ptrObj.StringVal", "primitive"},
		{"ptrObj.DurationVal", 3 * time.Second},
		{"ptrObj.TimeVal == timestamp('2026-09-25T00:00:00Z')", true},
		{"ptrObj.NestedVal.NestedCustomName", "embedded-nested"},
		{"ptrObj.Int8Ptr", int64(8)},
		{"ptrObj.Int16Ptr", int64(16)},
		{"ptrObj.Uint8Ptr", uint64(88)},
		{"ptrObj.Uint16Ptr", uint64(166)},

		// Presence checks
		{"has(ptrObj.IntVal)", true},
		{"has(ptrObj.Int32Val)", true},
		{"has(ptrObj.Int64Val)", true},
		{"has(ptrObj.UintVal)", true},
		{"has(ptrObj.Uint32Val)", true},
		{"has(ptrObj.Uint64Val)", true},
		{"has(ptrObj.Float32Val)", true},
		{"has(ptrObj.Float64Val)", true},
		{"has(ptrObj.BoolVal)", true},
		{"has(ptrObj.StringVal)", true},
		{"has(ptrObj.DurationVal)", true},
		{"has(ptrObj.TimeVal)", true},
		{"has(ptrObj.NestedVal)", true},
		{"has(ptrObj.Int8Ptr)", true},
		{"has(ptrObj.Int16Ptr)", true},
		{"has(ptrObj.Uint8Ptr)", true},
		{"has(ptrObj.Uint16Ptr)", true},

		// Value receiver
		{"valObj.Int8Val", int64(8)},
		{"valObj.Uint16Val", uint64(166)},
		{"valObj.Float32Val", 1.25},
		{"valObj.DurationVal", 3 * time.Second},
		{"valObj.NestedVal.NestedCustomName", "embedded-nested"},
		{"has(valObj.TimeVal)", true},

		// Zero values
		{"zeroObj.IntVal", int64(0)},
		{"zeroObj.Int8Ptr", int64(0)},
		{"zeroObj.Uint8Ptr", uint64(0)},
		{"zeroObj.TimeVal == timestamp('1970-01-01T00:00:00Z')", true},
		{"has(zeroObj.IntVal)", false},
		{"has(zeroObj.Int32Val)", false},
		{"has(zeroObj.Int64Val)", false},
		{"has(zeroObj.UintVal)", false},
		{"has(zeroObj.Uint32Val)", false},
		{"has(zeroObj.Uint64Val)", false},
		{"has(zeroObj.Float32Val)", false},
		{"has(zeroObj.Float64Val)", false},
		{"has(zeroObj.BoolVal)", false},
		{"has(zeroObj.StringVal)", false},
		{"has(zeroObj.DurationVal)", false},
		{"has(zeroObj.TimeVal)", false},
		{"has(zeroObj.NestedVal)", false},
		{"has(zeroObj.Int8Ptr)", false},
		{"has(zeroObj.Uint8Ptr)", false},
	}

	for _, tc := range tests {
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", tc.expr, err)
		}
		out, _, err := prg.Eval(vars)
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		adapter := env.CELTypeAdapter()
		wantVal := adapter.NativeToValue(tc.want)
		if out.Equal(wantVal) != types.True {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, out, out, wantVal, wantVal)
		}
	}
}

func TestNativeDirectFieldGetterTesterFallbacks(t *testing.T) {
	nt, err := types.NewNativeType(reflect.TypeFor[TestNativePrimitivesAndFallbacks]())
	if err != nil {
		t.Fatalf("NewNativeType failed: %v", err)
	}

	fields := []string{
		"IntVal", "Int32Val", "Int64Val", "UintVal", "Uint32Val", "Uint64Val",
		"Float32Val", "Float64Val", "BoolVal", "StringVal", "DurationVal", "TimeVal", "NestedVal",
	}

	obj := TestNativePrimitivesAndFallbacks{
		IntVal:      42,
		Int32Val:    42,
		Int64Val:    42,
		UintVal:     42,
		Uint32Val:   42,
		Uint64Val:   42,
		Float32Val:  42.0,
		Float64Val:  42.0,
		BoolVal:     true,
		StringVal:   "test",
		DurationVal: time.Second,
		TimeVal:     time.Unix(100, 0),
		NestedVal:   TestNestedType{NestedCustomName: "custom"},
	}

	for _, name := range fields {
		field, ok := nt.FindFieldType(name)
		if !ok {
			t.Fatalf("FindFieldType(%q) failed", name)
		}

		// 1. Direct struct ptr
		val, err := field.GetFrom(&obj)
		if err != nil || val == nil {
			t.Errorf("field.GetFrom(&obj) for %q got (%v, %v)", name, val, err)
		}
		if !field.IsSet(&obj) {
			t.Errorf("field.IsSet(&obj) for %q got false", name)
		}

		// 2. Direct struct value
		val, err = field.GetFrom(obj)
		if err != nil || val == nil {
			t.Errorf("field.GetFrom(obj) for %q got (%v, %v)", name, val, err)
		}
		if !field.IsSet(obj) {
			t.Errorf("field.IsSet(obj) for %q got false", name)
		}

		// 3. Invalid target
		val, err = field.GetFrom("invalid-type")
		if err != nil || val != nil {
			t.Errorf("field.GetFrom('invalid-type') for %q got (%v, %v), want (nil, nil)", name, val, err)
		}
		if field.IsSet("invalid-type") {
			t.Errorf("field.IsSet('invalid-type') for %q got true, want false", name)
		}
	}
}

type specialCollectionsStruct struct {
	StringSlice      []string
	IntSlice         []int
	Int32Slice       []int32
	Int64Slice       []int64
	UintSlice        []uint
	Uint32Slice      []uint32
	Uint64Slice      []uint64
	Float32Slice     []float32
	Float64Slice     []float64
	BoolSlice        []bool
	TimeSlice        []time.Time
	DurationSlice    []time.Duration
	StringStringMap  map[string]string
	Int64BoolMap     map[int64]bool
	StringInt64Map   map[string]int64
	StringIntMap     map[string]int
	StringBoolMap    map[string]bool
	StringFloat64Map map[string]float64
}

type otherStruct struct {
	OtherField string
}

func TestNativeSpecialCollectionsAndFallbacks(t *testing.T) {
	nt, err := types.NewNativeType(reflect.TypeFor[specialCollectionsStruct]())
	if err != nil {
		t.Fatalf("NewNativeType failed: %v", err)
	}

	obj := specialCollectionsStruct{
		StringSlice:      []string{"s1"},
		IntSlice:         []int{1},
		Int32Slice:       []int32{32},
		Int64Slice:       []int64{64},
		UintSlice:        []uint{2},
		Uint32Slice:      []uint32{320},
		Uint64Slice:      []uint64{640},
		Float32Slice:     []float32{1.5},
		Float64Slice:     []float64{2.5},
		BoolSlice:        []bool{true},
		TimeSlice:        []time.Time{time.Unix(100, 0)},
		DurationSlice:    []time.Duration{time.Second},
		StringStringMap:  map[string]string{"k": "v"},
		Int64BoolMap:     map[int64]bool{1: true},
		StringInt64Map:   map[string]int64{"k": 64},
		StringIntMap:     map[string]int{"k": 32},
		StringBoolMap:    map[string]bool{"k": true},
		StringFloat64Map: map[string]float64{"k": 1.5},
	}

	fieldNames := []string{
		"StringSlice", "IntSlice", "Int32Slice", "Int64Slice", "UintSlice", "Uint32Slice", "Uint64Slice",
		"Float32Slice", "Float64Slice", "BoolSlice", "TimeSlice", "DurationSlice",
		"StringStringMap", "Int64BoolMap", "StringInt64Map", "StringIntMap", "StringBoolMap", "StringFloat64Map",
	}

	type FallbackCollectionsStruct struct {
		StringSlice      []string
		IntSlice         []int
		Int32Slice       []int32
		Int64Slice       []int64
		UintSlice        []uint
		Uint32Slice      []uint32
		Uint64Slice      []uint64
		Float32Slice     []float32
		Float64Slice     []float64
		BoolSlice        []bool
		TimeSlice        []time.Time
		DurationSlice    []time.Duration
		StringStringMap  map[string]string
		Int64BoolMap     map[int64]bool
		StringInt64Map   map[string]int64
		StringIntMap     map[string]int
		StringBoolMap    map[string]bool
		StringFloat64Map map[string]float64
	}
	fbCollections := FallbackCollectionsStruct{
		StringSlice:      []string{"s1"},
		IntSlice:         []int{1},
		Int32Slice:       []int32{32},
		Int64Slice:       []int64{64},
		UintSlice:        []uint{2},
		Uint32Slice:      []uint32{320},
		Uint64Slice:      []uint64{640},
		Float32Slice:     []float32{1.5},
		Float64Slice:     []float64{2.5},
		BoolSlice:        []bool{true},
		TimeSlice:        []time.Time{time.Unix(100, 0)},
		DurationSlice:    []time.Duration{time.Second},
		StringStringMap:  map[string]string{"k": "v"},
		Int64BoolMap:     map[int64]bool{1: true},
		StringInt64Map:   map[string]int64{"k": 64},
		StringIntMap:     map[string]int{"k": 32},
		StringBoolMap:    map[string]bool{"k": true},
		StringFloat64Map: map[string]float64{"k": 1.5},
	}

	for _, name := range fieldNames {
		f, ok := nt.FindFieldType(name)
		if !ok {
			t.Fatalf("FindFieldType(%q) failed", name)
		}

		// Direct pointer
		val, err := f.GetFrom(&obj)
		if err != nil || val == nil {
			t.Errorf("GetFrom(&obj) for %q got (%v, %v)", name, val, err)
		}
		if !f.IsSet(&obj) {
			t.Errorf("IsSet(&obj) for %q got false", name)
		}

		// Direct value (triggers struct unwrap path)
		val, err = f.GetFrom(obj)
		if err != nil || val == nil {
			t.Errorf("GetFrom(obj) for %q got (%v, %v)", name, val, err)
		}
		if !f.IsSet(obj) {
			t.Errorf("IsSet(obj) for %q got false", name)
		}

		// Fallback value (untyped struct with matching field index)
		val, err = f.GetFrom(fbCollections)
		if err != nil || val == nil {
			t.Errorf("GetFrom(fbCollections) for %q got (%v, %v)", name, val, err)
		}
		if !f.IsSet(fbCollections) {
			t.Errorf("IsSet(fbCollections) for %q got false", name)
		}

		// Non-matching target (triggers safeGetFieldByIndex fallback when invalid)
		val, err = f.GetFrom("not-a-struct")
		if err != nil || val != nil {
			t.Errorf("GetFrom('not-a-struct') for %q got (%v, %v), want (nil, nil)", name, val, err)
		}
		if f.IsSet("not-a-struct") {
			t.Errorf("IsSet('not-a-struct') for %q got true, want false", name)
		}
	}
}

type StructWithUnsupportedFields struct {
	Ch chan int
}

type StructWithNilEmbeddedPtr struct {
	*TestNestedType
	Value string
}

func TestNativeTypeCoverageBoost(t *testing.T) {
	// 1. NewNativeType with option error
	optErr := errors.New("opt error")
	_, err := types.NewNativeType(reflect.TypeFor[TestAllTypes](), func(*types.NativeTypeOptions) error {
		return optErr
	})
	if err == nil || !errors.Is(err, optErr) {
		t.Errorf("NewNativeType with error option got %v, want %v", err, optErr)
	}

	// 2. ParseStructTags(false)
	optTags := types.ParseStructTags(false)
	ntTags, err := types.NewNativeType(reflect.TypeFor[TestAllTypes](), optTags)
	if err != nil {
		t.Fatalf("NewNativeType with ParseStructTags(false) failed: %v", err)
	}
	if ntTags == nil {
		t.Errorf("expected non-nil NativeType")
	}

	// 3. Adapt nil and nil pointer and direct interface struct value
	nt, err := types.NewNativeType(reflect.TypeFor[TestAllTypes]())
	if err != nil {
		t.Fatalf("NewNativeType failed: %v", err)
	}
	if nt.Adapt(types.DefaultTypeAdapter, nil) != types.NullValue {
		t.Errorf("Adapt(nil) want NullValue")
	}
	if nt.Adapt(types.DefaultTypeAdapter, (*TestAllTypes)(nil)) != types.NullValue {
		t.Errorf("Adapt((*TestAllTypes)(nil)) want NullValue")
	}
	ntDirect, err := types.NewNativeType(reflect.TypeFor[directIfaceStruct]())
	if err != nil {
		t.Fatalf("NewNativeType for directIfaceStruct failed: %v", err)
	}
	adaptedDirect := ntDirect.Adapt(types.DefaultTypeAdapter, directIfaceStruct{})
	if adaptedDirect == nil {
		t.Errorf("Adapt(directIfaceStruct{}) want non-nil")
	}

	// 4. FindFieldType on unsupported field
	ntChan, err := types.NewNativeType(reflect.TypeFor[StructWithUnsupportedFields]())
	if err != nil {
		t.Fatalf("NewNativeType for StructWithUnsupportedFields failed: %v", err)
	}
	if _, found := ntChan.FindFieldType("Ch"); found {
		t.Errorf("FindFieldType('Ch') want not found")
	}

	// 5. NewValue error paths
	errVal := nt.NewValue(types.DefaultTypeAdapter, map[string]ref.Val{"invalidField": types.Int(1)})
	if !types.IsError(errVal) {
		t.Errorf("NewValue with invalidField got %v, want error", errVal)
	}
	errVal = nt.NewValue(types.DefaultTypeAdapter, map[string]ref.Val{"Int32Val": types.String("not-an-int")})
	if !types.IsError(errVal) {
		t.Errorf("NewValue with invalid field type got %v, want error", errVal)
	}

	// 6. safeSetFieldByIndex with nil embedded pointer struct
	reg, err := types.NewRegistry(reflect.TypeFor[StructWithNilEmbeddedPtr]())
	if err != nil {
		t.Fatalf("NewRegistry for StructWithNilEmbeddedPtr failed: %v", err)
	}
	ntEmb, err := types.NewNativeType(reflect.TypeFor[StructWithNilEmbeddedPtr]())
	if err != nil {
		t.Fatalf("NewNativeType for StructWithNilEmbeddedPtr failed: %v", err)
	}
	embVal := ntEmb.NewValue(reg, map[string]ref.Val{
		"NestedCustomName": types.String("embedded-name"),
		"Value":            types.String("val"),
	})
	if types.IsError(embVal) {
		t.Errorf("NewValue on StructWithNilEmbeddedPtr got %v", embVal)
	}

	// 7. ConvertToNative on nativeObj for jsonStruct and jsonValue
	adaptedAll := nt.Adapt(types.DefaultTypeAdapter, &TestAllTypes{
		StringVal:    "hello json",
		Int32Val:     42,
		BoolVal:      true,
		DurationVal:  time.Minute,
		TimestampVal: time.Unix(12345, 0),
	})
	nativeJSONVal, err := adaptedAll.ConvertToNative(reflect.TypeFor[*structpb.Value]())
	if err != nil {
		t.Errorf("ConvertToNative(structpb.Value) failed: %v", err)
	}
	if nativeJSONVal == nil {
		t.Errorf("ConvertToNative(structpb.Value) want non-nil")
	}
	nativeJSONStruct, err := adaptedAll.ConvertToNative(reflect.TypeFor[*structpb.Struct]())
	if err != nil {
		t.Errorf("ConvertToNative(structpb.Struct) failed: %v", err)
	}
	if nativeJSONStruct == nil {
		t.Errorf("ConvertToNative(structpb.Struct) want non-nil")
	}

	// 8. AggregateSize cache
	calc := types.NewSizeCalculator()
	res1 := calc.ApproximateAggregateSize(adaptedAll)
	res2 := calc.ApproximateAggregateSize(adaptedAll)
	if res1.Size != res2.Size || res1.Size == 0 {
		t.Errorf("AggregateSize got %d, %d", res1.Size, res2.Size)
	}

	// 9. structPtrFrom & unwrapStruct with *nativeObj directly on field getters & testers
	for _, name := range []string{"StringVal", "Int32Val", "BoolVal", "DurationVal", "TimestampVal"} {
		f, ok := nt.FindFieldType(name)
		if !ok {
			t.Fatalf("FindFieldType(%q) failed", name)
		}
		got, err := f.GetFrom(adaptedAll)
		if err != nil || got == nil {
			t.Errorf("GetFrom(adaptedAll) for %q got (%v, %v)", name, got, err)
		}
		if !f.IsSet(adaptedAll) {
			t.Errorf("IsSet(adaptedAll) for %q got false", name)
		}
	}

	// 10. safeSetFieldByIndex and safeGetFieldByIndex boundary cases
	type NestedPointerStruct struct {
		Ptr *TestNestedType
	}
	ntNested, err := types.NewNativeType(reflect.TypeFor[NestedPointerStruct]())
	if err != nil {
		t.Fatalf("NewNativeType for NestedPointerStruct failed: %v", err)
	}
	fNested, ok := ntNested.FindFieldType("Ptr")
	if !ok {
		t.Fatalf("FindFieldType('Ptr') failed")
	}
	_, _ = fNested.GetFrom(&NestedPointerStruct{Ptr: nil})
	_ = fNested.IsSet(&NestedPointerStruct{Ptr: nil})

	// Test nil struct pointer for len(index) == 1
	var nilNested *NestedPointerStruct
	_, _ = fNested.GetFrom(nilNested)
	_ = fNested.IsSet(nilNested)
	_, _ = fNested.GetFrom("not a struct")
	_ = fNested.IsSet("not a struct")

	fAllBool, _ := nt.FindFieldType("BoolVal")
	if fAllBool != nil {
		var nilAll *TestAllTypes
		_, _ = fAllBool.GetFrom(nilAll)
		_ = fAllBool.IsSet(nilAll)
		_, _ = fAllBool.GetFrom("not a struct")
		_ = fAllBool.IsSet("not a struct")
	}

	// Test embedded struct with non-nil and nil intermediate pointer for len(index) > 1
	type EmbeddedIntermediate struct {
		*TestNestedType
	}
	type OuterEmbedded struct {
		EmbeddedIntermediate
	}
	ntEmbMulti, err := types.NewNativeType(reflect.TypeFor[OuterEmbedded]())
	if err == nil {
		fEmbMulti, ok := ntEmbMulti.FindFieldType("NestedCustomName")
		if ok {
			// Non-nil intermediate pointer
			_, _ = fEmbMulti.GetFrom(OuterEmbedded{EmbeddedIntermediate: EmbeddedIntermediate{TestNestedType: &TestNestedType{NestedCustomName: "test"}}})
			_ = fEmbMulti.IsSet(OuterEmbedded{EmbeddedIntermediate: EmbeddedIntermediate{TestNestedType: &TestNestedType{NestedCustomName: "test"}}})
			// Nil intermediate pointer
			_, _ = fEmbMulti.GetFrom(OuterEmbedded{EmbeddedIntermediate: EmbeddedIntermediate{TestNestedType: nil}})
			_ = fEmbMulti.IsSet(OuterEmbedded{EmbeddedIntermediate: EmbeddedIntermediate{TestNestedType: nil}})
			var nilOuter *OuterEmbedded
			_, _ = fEmbMulti.GetFrom(nilOuter)
			_ = fEmbMulti.IsSet(nilOuter)
			_, _ = fEmbMulti.GetFrom("not a struct")
			_ = fEmbMulti.IsSet("not a struct")
		}
	}
	// NewValue error tests
	_ = nt.NewValue(types.DefaultTypeAdapter, map[string]ref.Val{
		"NoSuchField": types.String("val"),
	})
	_ = nt.NewValue(types.DefaultTypeAdapter, map[string]ref.Val{
		"BoolVal": types.String("cannot convert to bool"),
	})

	// 11. Fallback unwrap for all field types via an untyped nativeObj wrapper (structPtr == nil)
	fBool, _ := nt.FindFieldType("BoolVal")
	fInt32, _ := nt.FindFieldType("Int32Val")
	fString, _ := nt.FindFieldType("StringVal")
	fBytes, _ := nt.FindFieldType("BytesVal")
	fDuration, _ := nt.FindFieldType("DurationVal")
	fList, _ := nt.FindFieldType("ListVal")
	fCustomSlice, _ := nt.FindFieldType("CustomSliceVal")
	fNestedStruct, _ := nt.FindFieldType("NestedStructVal")

	valAllFalse := TestAllTypes{
		BoolVal:         false,
		Int32Val:        42,
		StringVal:       "hello",
		BytesVal:        []byte("bytes"),
		DurationVal:     time.Second,
		ListVal:         []*TestNestedType{{NestedCustomName: "dyn"}},
		CustomSliceVal:  []TestNestedSliceType{{Value: "val"}},
		NestedStructVal: TestNestedType{NestedCustomName: "nested"},
	}
	valAllTrue := TestAllTypes{
		BoolVal: true,
	}

	for _, f := range []*types.FieldType{fBool, fInt32, fString, fBytes, fDuration, fList, fCustomSlice, fNestedStruct} {
		if f != nil {
			_, _ = f.GetFrom(valAllFalse)
			_ = f.IsSet(valAllFalse)
			_, _ = f.GetFrom(valAllTrue)
			_ = f.IsSet(valAllTrue)
		}
	}

	// 12. ConvertToNative on nativeObj for struct by value vs pointer
	convertedVal, err := adaptedAll.ConvertToNative(reflect.TypeFor[TestAllTypes]())
	if err != nil {
		t.Errorf("ConvertToNative(TestAllTypes) failed: %v", err)
	}
	if _, ok := convertedVal.(TestAllTypes); !ok {
		t.Errorf("ConvertToNative(TestAllTypes) got %T", convertedVal)
	}
	convertedPtr, err := adaptedAll.ConvertToNative(reflect.TypeFor[*TestAllTypes]())
	if err != nil {
		t.Errorf("ConvertToNative(*TestAllTypes) failed: %v", err)
	}
	if _, ok := convertedPtr.(*TestAllTypes); !ok {
		t.Errorf("ConvertToNative(*TestAllTypes) got %T", convertedPtr)
	}
	_, err = adaptedAll.ConvertToNative(reflect.TypeFor[int]())
	if err == nil {
		t.Errorf("expected error converting to int")
	}


	// 14. getFieldValue direct calls
	dummyAdapter := types.DefaultTypeAdapter
	_ = dummyAdapter.NativeToValue([]byte("hello"))
	_ = dummyAdapter.NativeToValue([]string{"a", "b"})
	_ = dummyAdapter.NativeToValue([]ref.Val{types.Int(1)})
	_ = dummyAdapter.NativeToValue(map[string]string{"a": "b"})
	_ = dummyAdapter.NativeToValue(map[string]any{"a": 1})
	_ = dummyAdapter.NativeToValue(map[ref.Val]ref.Val{types.String("k"): types.String("v")})
	_ = dummyAdapter.NativeToValue((*bool)(nil))
	_ = dummyAdapter.NativeToValue((*time.Duration)(nil))
	_ = dummyAdapter.NativeToValue((*int)(nil))
	_ = dummyAdapter.NativeToValue((*int8)(nil))
	_ = dummyAdapter.NativeToValue((*int16)(nil))
	_ = dummyAdapter.NativeToValue((*int32)(nil))
	_ = dummyAdapter.NativeToValue((*int64)(nil))
	_ = dummyAdapter.NativeToValue((*uint)(nil))
	_ = dummyAdapter.NativeToValue((*uint8)(nil))
	_ = dummyAdapter.NativeToValue((*uint16)(nil))
	_ = dummyAdapter.NativeToValue((*uint32)(nil))
	_ = dummyAdapter.NativeToValue((*uint64)(nil))
	_ = dummyAdapter.NativeToValue((*float32)(nil))
	_ = dummyAdapter.NativeToValue((*float64)(nil))
	_ = dummyAdapter.NativeToValue((*string)(nil))
	_ = dummyAdapter.NativeToValue((*time.Time)(nil))
	_ = dummyAdapter.NativeToValue((*TestNestedType)(nil))
	_ = dummyAdapter.NativeToValue(time.Time{})
	_ = dummyAdapter.NativeToValue(time.Unix(100, 0))
	_ = dummyAdapter.NativeToValue(&time.Time{})
	nowTime := time.Unix(200, 0)
	_ = dummyAdapter.NativeToValue(&nowTime)

	// 15. convertToCelType and FindFieldType branch coverage
	type StructWithBadTypes struct {
		AnySlice       []any
		AnyArray       [2]any
		BadKeyMap      map[any]string
		BadValMap      map[string]any
		AnyField       any
		RefValSt       types.Int
		RefValPtr      *types.Err
		ProtoPtr       *proto3pb.TestAllTypes
		BytesArray     [4]byte
		PtrPtrInt      **int
		TimestampField time.Time
		TimestampPtr   *time.Time
		DurationField  time.Duration
		DurationPtr    *time.Duration
	}
	ntBad, err := types.NewNativeType(reflect.TypeFor[StructWithBadTypes]())
	if err != nil {
		t.Fatalf("NewNativeType for StructWithBadTypes failed: %v", err)
	}
	for _, name := range []string{
		"AnySlice", "AnyArray", "BadKeyMap", "BadValMap", "AnyField", "RefValSt", "RefValPtr", "ProtoPtr",
		"BytesArray", "PtrPtrInt", "TimestampField", "TimestampPtr", "DurationField", "DurationPtr",
	} {
		_, _ = ntBad.FindFieldType(name)
	}

	// 15b. JSON conversion error coverage
	type StructWithUnconvertibleJSON struct {
		ErrField *types.Err `json:"err"`
	}
	regUnconv, err := types.NewRegistry(reflect.TypeFor[StructWithUnconvertibleJSON]())
	if err == nil {
		ntUnconv, err := types.NewNativeType(reflect.TypeFor[StructWithUnconvertibleJSON]())
		if err == nil {
			adaptedUnconv := ntUnconv.Adapt(regUnconv, &StructWithUnconvertibleJSON{
				ErrField: types.NewErr("test err").(*types.Err),
			})
			_, _ = adaptedUnconv.ConvertToNative(reflect.TypeFor[*structpb.Struct]())
			_, _ = adaptedUnconv.ConvertToNative(reflect.TypeFor[*structpb.Value]())
		}
	}

	// 16. Registry NewValue and ConvertToNative API boundary coverage
	reg, err = types.NewRegistry(reflect.TypeFor[TestAllTypes]())
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}
	res := reg.NewValue("types_test.TestAllTypes", map[string]ref.Val{"NonExistentField": types.Int(1)})
	if !types.IsError(res) {
		t.Errorf("expected error for non-existent field in NewValue")
	}
	res = reg.NewValue("types_test.TestAllTypes", map[string]ref.Val{"IntVal": types.String("not-int")})
	if !types.IsError(res) {
		t.Errorf("expected error for incompatible field value in NewValue")
	}
	createdVal := reg.NewValue("types_test.TestAllTypes", map[string]ref.Val{
		"StringVal": types.String("created-string"),
		"Int64Val":  types.Int(99),
	})
	if types.IsError(createdVal) {
		t.Fatalf("NewValue failed: %v", createdVal)
	}
	vStruct, err := createdVal.ConvertToNative(reflect.TypeFor[TestAllTypes]())
	if err != nil || vStruct.(TestAllTypes).StringVal != "created-string" {
		t.Errorf("ConvertToNative(TestAllTypes) = %v, %v", vStruct, err)
	}
	vPtr, err := createdVal.ConvertToNative(reflect.TypeFor[*TestAllTypes]())
	if err != nil || vPtr.(*TestAllTypes).StringVal != "created-string" {
		t.Errorf("ConvertToNative(*TestAllTypes) = %v, %v", vPtr, err)
	}

	// 17. FindFieldType on interface field (maps to DynType)
	type StructWithInterfaceField struct {
		AnyField any
	}
	ntIface, err := types.NewNativeType(
		reflect.TypeFor[StructWithInterfaceField](),
		types.NativeTypeAdapter(types.DefaultTypeAdapter),
	)
	if err != nil {
		t.Fatalf("NewNativeType for StructWithInterfaceField failed: %v", err)
	}
	fIface, ok := ntIface.FindFieldType("AnyField")
	if !ok {
		t.Fatalf("expected true for AnyField in FindFieldType")
	}
	if fIface.Type != types.DynType {
		t.Errorf("expected DynType for AnyField, got %v", fIface.Type)
	}
	if val, err := fIface.GetFrom(&StructWithInterfaceField{AnyField: "dyn-val"}); err != nil || val != types.String("dyn-val") {
		t.Errorf("GetFrom on AnyField got (%v, %v), want 'dyn-val'", val, err)
	}
	if !fIface.IsSet(&StructWithInterfaceField{AnyField: "dyn-val"}) {
		t.Errorf("IsSet on non-nil AnyField got false, want true")
	}
	if fIface.IsSet(&StructWithInterfaceField{AnyField: nil}) {
		t.Errorf("IsSet on nil AnyField got true, want false")
	}
	if ntIface.NativeToValue(123) != types.Int(123) {
		t.Errorf("ntIface.NativeToValue(123) failed")
	}

	// Test NativeType.Clone
	clonedNT, err := ntIface.Clone()
	if err != nil || clonedNT == nil {
		t.Fatalf("ntIface.Clone() failed: %v", err)
	}
	aliasedNT, err := ntIface.Clone(types.NativeTypeAlias("custom.Alias"))
	if err != nil || aliasedNT.TypeName() != "custom.Alias" {
		t.Fatalf("ntIface.Clone(NativeTypeAlias) failed: %v", err)
	}
	taggedNT, err := ntIface.Clone(types.ParseStructTags(true))
	if err != nil || taggedNT == nil {
		t.Fatalf("ntIface.Clone(ParseStructTags) failed: %v", err)
	}
	var nilNT *types.NativeType
	nilCloned, err := nilNT.Clone()
	if err != nil || nilCloned != nil {
		t.Fatalf("nilNT.Clone() failed: %v", err)
	}

	// 18. FieldType.GetFrom and IsSet fallback paths across all supported types
	type FallbackMatchingFields struct {
		NestedVal       *TestNestedType
		NestedStructVal TestNestedType
		BoolVal         bool
		BytesVal        []byte
		DurationVal     time.Duration
		DoubleVal       float64
		FloatVal        float32
		Int32Val        int32
		Int64Val        int64
		StringVal       string
		TimestampVal    time.Time
		Uint32Val       uint32
		Uint64Val       uint64
		ListVal         []*TestNestedType
		ArrayVal        [1]*TestNestedType
		BytesArrayVal   [4]byte
		MapVal          map[string]TestAllTypes
	}

	fbPopulated := FallbackMatchingFields{
		BoolVal:      true,
		BytesVal:     []byte("bytes"),
		DurationVal:  time.Second,
		DoubleVal:    2.5,
		FloatVal:     1.5,
		Int32Val:     42,
		Int64Val:     84,
		StringVal:    "hello",
		TimestampVal: time.Unix(100, 0),
		Uint32Val:    10,
		Uint64Val:    20,
		ListVal:      []*TestNestedType{{NestedCustomName: "l"}},
		MapVal:       map[string]TestAllTypes{"k": {BoolVal: true}},
	}
	fbZero := FallbackMatchingFields{}

	for _, name := range nt.FieldNames() {
		f, ok := nt.FindFieldType(name)
		if !ok {
			continue
		}
		_, _ = f.GetFrom(fbPopulated)
		_ = f.IsSet(fbPopulated)
		_, _ = f.GetFrom(&fbPopulated)
		_ = f.IsSet(&fbPopulated)
		_, _ = f.GetFrom(fbZero)
		_ = f.IsSet(fbZero)
		_, _ = f.GetFrom(struct{}{})
		_ = f.IsSet(struct{}{})
		// Passing adaptedVal to exercise unwrapStruct(*nativeObj)
		adaptedFb := nt.Adapt(types.DefaultTypeAdapter, &TestAllTypes{StringVal: "s"})
		_, _ = f.GetFrom(adaptedFb)
		_ = f.IsSet(adaptedFb)
	}

	ntSpecial, err := types.NewNativeType(reflect.TypeFor[specialCollectionsStruct]())
	if err == nil {
		specialSt := specialCollectionsStruct{
			StringSlice:      []string{"a"},
			IntSlice:         []int{1},
			Int32Slice:       []int32{2},
			Int64Slice:       []int64{3},
			UintSlice:        []uint{4},
			Uint32Slice:      []uint32{5},
			Uint64Slice:      []uint64{6},
			Float32Slice:     []float32{1.5},
			Float64Slice:     []float64{2.5},
			BoolSlice:        []bool{true},
			TimeSlice:        []time.Time{time.Unix(100, 0)},
			DurationSlice:    []time.Duration{time.Second},
			StringStringMap:  map[string]string{"k": "v"},
			Int64BoolMap:     map[int64]bool{1: true},
			StringInt64Map:   map[string]int64{"k": 64},
			StringIntMap:     map[string]int{"k": 32},
			StringBoolMap:    map[string]bool{"k": true},
			StringFloat64Map: map[string]float64{"k": 1.5},
		}
		for _, name := range ntSpecial.FieldNames() {
			f, ok := ntSpecial.FindFieldType(name)
			if !ok {
				continue
			}
			_, _ = f.GetFrom(specialSt)
			_ = f.IsSet(specialSt)
			_, _ = f.GetFrom(struct{}{})
			_ = f.IsSet(struct{}{})
		}
	}
}

type TestInnerAllFields struct {
	BoolVal            bool
	DurationVal        time.Duration
	TimestampVal       time.Time
	IntVal             int
	Int32Val           int32
	Int64Val           int64
	UintVal            uint
	Uint32Val          uint32
	Uint64Val          uint64
	Float32Val         float32
	Float64Val         float64
	StringVal          string
	BytesVal           []byte
	SliceVal           []string
	RefValList         []ref.Val
	MapVal             map[string]int
	StringStringMap    map[string]string
	StringInterfaceMap map[string]any
	RefValMap          map[ref.Val]ref.Val
	NestedVal          TestNestedType
	BoolPtr            *bool
	DurationPtr        *time.Duration
	TimestampPtr       *time.Time
	IntPtr             *int
	Int32Ptr           *int32
	Int64Ptr           *int64
	UintPtr            *uint
	Uint32Ptr          *uint32
	Uint64Ptr          *uint64
	Float32Ptr         *float32
	Float64Ptr         *float64
	StringPtr          *string
	NestedPtr          *TestNestedType
	Int8Ptr            *int8
	Int16Ptr           *int16
	Uint8Ptr           *uint8
	Uint16Ptr          *uint16
	NilTimePtr         *time.Time
	NilDurPtr          *time.Duration
	NilStructPtr       *TestNestedType
	ZeroTime           time.Time
	TaggedZero         int            `json:"num"`
	OmitEmpty          int            `json:"omit,omitempty"`
}

type TestOuterEmbeddedPointer struct {
	*TestInnerAllFields
	OuterField string
}

type Level2Struct struct {
	Leaf string
}

type Level1Struct struct {
	*Level2Struct
}

type TestDeepEmbeddedPointers struct {
	*Level1Struct
}

func TestNativeEmbeddedPointerExpressions(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Types(
			reflect.TypeFor[TestOuterEmbeddedPointer](),
			reflect.TypeFor[TestDeepEmbeddedPointers](),
			reflect.TypeFor[TestNestedType](),
			reflect.TypeFor[StructWithNilEmbeddedPtr](),
		),
		cel.Variable("popObj", cel.ObjectType("types_test.TestOuterEmbeddedPointer")),
		cel.Variable("zeroObj", cel.ObjectType("types_test.TestOuterEmbeddedPointer")),
		cel.Variable("nilEmbObj", cel.ObjectType("types_test.TestOuterEmbeddedPointer")),
		cel.Variable("deepObj", cel.ObjectType("types_test.TestDeepEmbeddedPointers")),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	bVal := true
	dVal := time.Minute
	tVal := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	iVal := 10
	i32Val := int32(20)
	i64Val := int64(30)
	uVal := uint(40)
	u32Val := uint32(50)
	u64Val := uint64(60)
	f32Val := float32(1.5)
	f64Val := 2.5
	sVal := "str-ptr"
	nestedVal := &TestNestedType{NestedCustomName: "sub-ptr"}
	i8Val := int8(8)
	i16Val := int16(16)
	u8Val := uint8(80)
	u16Val := uint16(160)

	popInner := &TestInnerAllFields{
		BoolVal:            true,
		DurationVal:        time.Second,
		TimestampVal:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IntVal:             100,
		Int32Val:           200,
		Int64Val:           300,
		UintVal:            400,
		Uint32Val:          500,
		Uint64Val:          600,
		Float32Val:         3.5,
		Float64Val:         4.5,
		StringVal:          "hello-emb",
		BytesVal:           []byte("bytes-emb"),
		SliceVal:           []string{"a", "b"},
		RefValList:         []ref.Val{types.Int(1)},
		MapVal:             map[string]int{"k": 1},
		StringStringMap:    map[string]string{"sk": "sv"},
		StringInterfaceMap: map[string]any{"ik": 2},
		RefValMap:          map[ref.Val]ref.Val{types.String("rk"): types.Int(3)},
		NestedVal:          TestNestedType{NestedCustomName: "sub-val"},
		BoolPtr:            &bVal,
		DurationPtr:        &dVal,
		TimestampPtr:       &tVal,
		IntPtr:             &iVal,
		Int32Ptr:           &i32Val,
		Int64Ptr:           &i64Val,
		UintPtr:            &uVal,
		Uint32Ptr:          &u32Val,
		Uint64Ptr:          &u64Val,
		Float32Ptr:         &f32Val,
		Float64Ptr:         &f64Val,
		StringPtr:          &sVal,
		NestedPtr:          nestedVal,
		Int8Ptr:            &i8Val,
		Int16Ptr:           &i16Val,
		Uint8Ptr:           &u8Val,
		Uint16Ptr:          &u16Val,
		ZeroTime:           time.Time{},
		TaggedZero:         0,
		OmitEmpty:          0,
	}

	popOuter := &TestOuterEmbeddedPointer{
		TestInnerAllFields: popInner,
		OuterField:         "outer-val",
	}
	zeroOuter := &TestOuterEmbeddedPointer{
		TestInnerAllFields: &TestInnerAllFields{},
		OuterField:         "",
	}
	nilEmbOuter := &TestOuterEmbeddedPointer{
		TestInnerAllFields: nil,
		OuterField:         "nil-emb",
	}
	deep := &TestDeepEmbeddedPointers{}

	vars := map[string]any{
		"popObj":    popOuter,
		"zeroObj":   zeroOuter,
		"nilEmbObj": nilEmbOuter,
		"deepObj":   deep,
	}

	exprTests := []struct {
		expr string
		want any
	}{
		// Populated embedded fields through pointer
		{expr: "popObj.OuterField", want: "outer-val"},
		{expr: "popObj.BoolVal", want: true},
		{expr: "popObj.DurationVal", want: time.Second},
		{expr: "popObj.TimestampVal", want: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{expr: "popObj.IntVal", want: int64(100)},
		{expr: "popObj.Int32Val", want: int64(200)},
		{expr: "popObj.Int64Val", want: int64(300)},
		{expr: "popObj.UintVal", want: uint64(400)},
		{expr: "popObj.Uint32Val", want: uint64(500)},
		{expr: "popObj.Uint64Val", want: uint64(600)},
		{expr: "popObj.Float32Val", want: float64(float32(3.5))},
		{expr: "popObj.Float64Val", want: 4.5},
		{expr: "popObj.StringVal", want: "hello-emb"},
		{expr: "popObj.BytesVal", want: []byte("bytes-emb")},
		{expr: "popObj.SliceVal", want: []string{"a", "b"}},
		{expr: "popObj.StringStringMap['sk']", want: "sv"},
		{expr: "popObj.MapVal['k']", want: int64(1)},
		{expr: "popObj.NestedVal.NestedCustomName", want: "sub-val"},
		{expr: "popObj.BoolPtr", want: true},
		{expr: "popObj.DurationPtr", want: time.Minute},
		{expr: "popObj.TimestampPtr", want: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)},
		{expr: "popObj.IntPtr", want: int64(10)},
		{expr: "popObj.Int32Ptr", want: int64(20)},
		{expr: "popObj.Int64Ptr", want: int64(30)},
		{expr: "popObj.UintPtr", want: uint64(40)},
		{expr: "popObj.Uint32Ptr", want: uint64(50)},
		{expr: "popObj.Uint64Ptr", want: uint64(60)},
		{expr: "popObj.Float32Ptr", want: float64(float32(1.5))},
		{expr: "popObj.Float64Ptr", want: 2.5},
		{expr: "popObj.StringPtr", want: "str-ptr"},
		{expr: "popObj.NestedPtr.NestedCustomName", want: "sub-ptr"},
		{expr: "popObj.Int8Ptr", want: int64(8)},
		{expr: "popObj.Int16Ptr", want: int64(16)},
		{expr: "popObj.Uint8Ptr", want: uint64(80)},
		{expr: "popObj.Uint16Ptr", want: uint64(160)},
		{expr: "popObj.ZeroTime", want: time.Unix(0, 0)},
		// Presence testing
		{expr: "has(popObj.BoolVal)", want: true},
		{expr: "has(popObj.IntVal)", want: true},
		{expr: "has(popObj.StringVal)", want: true},
		{expr: "has(popObj.BoolPtr)", want: true},
		{expr: "has(popObj.DurationPtr)", want: true},
		{expr: "has(popObj.IntPtr)", want: true},
		{expr: "has(popObj.UintPtr)", want: true},
		{expr: "has(popObj.Float32Ptr)", want: true},
		{expr: "has(popObj.Float64Ptr)", want: true},
		{expr: "has(popObj.StringPtr)", want: true},
		{expr: "has(popObj.TimestampPtr)", want: true},
		{expr: "has(popObj.NestedPtr)", want: true},
		{expr: "has(popObj.Int8Ptr)", want: true},
		{expr: "has(popObj.Uint8Ptr)", want: true},
		// Zero presence testing
		{expr: "has(zeroObj.BoolVal)", want: false},
		{expr: "has(zeroObj.IntVal)", want: false},
		{expr: "has(zeroObj.StringVal)", want: false},
		{expr: "has(zeroObj.BoolPtr)", want: false},
		{expr: "has(zeroObj.IntPtr)", want: false},
		{expr: "has(zeroObj.DurationPtr)", want: false},
		{expr: "has(zeroObj.TimestampPtr)", want: false},
		{expr: "has(zeroObj.Float32Ptr)", want: false},
		{expr: "has(zeroObj.Float64Ptr)", want: false},
		{expr: "has(zeroObj.StringPtr)", want: false},
		{expr: "has(zeroObj.NestedPtr)", want: false},
		{expr: "has(zeroObj.Int8Ptr)", want: false},
		{expr: "has(zeroObj.Uint8Ptr)", want: false},
		{expr: "has(zeroObj.NilTimePtr)", want: false},
		{expr: "has(zeroObj.NilDurPtr)", want: false},
		{expr: "has(zeroObj.NilStructPtr)", want: false},
		// Zero values accessed directly through embedded pointer
		{expr: "zeroObj.BoolVal", want: false},
		{expr: "zeroObj.BoolPtr", want: false},
		{expr: "zeroObj.DurationPtr", want: time.Duration(0)},
		{expr: "zeroObj.IntPtr", want: int64(0)},
		{expr: "zeroObj.UintPtr", want: uint64(0)},
		{expr: "zeroObj.Float32Ptr", want: float64(0)},
		{expr: "zeroObj.Float64Ptr", want: float64(0)},
		{expr: "zeroObj.StringPtr", want: ""},
		{expr: "zeroObj.TimestampPtr", want: time.Unix(0, 0)},
		{expr: "zeroObj.NilTimePtr", want: time.Unix(0, 0)},
		{expr: "zeroObj.NilDurPtr", want: time.Duration(0)},
		// Nil embedded struct pointer
		{expr: "has(nilEmbObj.BoolVal)", want: false},
		{expr: "has(nilEmbObj.IntVal)", want: false},
		{expr: "has(nilEmbObj.StringVal)", want: false},
		// Deep nil pointer index
		{expr: "has(deepObj.Leaf)", want: false},
	}

	for _, tc := range exprTests {
		ast, issues := env.Compile(tc.expr)
		if issues != nil && issues.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", tc.expr, issues.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", tc.expr, err)
		}
		out, _, err := prg.Eval(vars)
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		got := out.Value()
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
		}
	}

	// JSON conversions for embedded struct
	reg, err := types.NewRegistry(reflect.TypeFor[TestOuterEmbeddedPointer]())
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}
	nt, err := types.NewNativeType(reflect.TypeFor[TestOuterEmbeddedPointer]())
	if err != nil {
		t.Fatalf("NewNativeType failed: %v", err)
	}
	adapted := nt.Adapt(reg, popOuter)
	jsonStructVal, err := adapted.ConvertToNative(reflect.TypeFor[*structpb.Struct]())
	if err != nil {
		t.Fatalf("ConvertToNative(structpb.Struct) failed: %v", err)
	}
	stpb := jsonStructVal.(*structpb.Struct)
	if _, ok := stpb.Fields["num"]; !ok {
		t.Errorf("expected 'num' field in structpb.Struct")
	}
	if _, ok := stpb.Fields["omit"]; ok {
		t.Errorf("expected 'omit' field to be omitted in structpb.Struct")
	}

	// Direct Get on nativeObj for interface reflection fields
	if idx, ok := adapted.(traits.Indexer); ok {
		_ = idx.Get(types.String("RefValList"))
		_ = idx.Get(types.String("StringInterfaceMap"))
		_ = idx.Get(types.String("RefValMap"))
		_ = idx.Get(types.String("NonExistent"))
	}

	// Test NewValue initializing nil embedded struct pointer
	regEmb, err := types.NewRegistry(reflect.TypeFor[StructWithNilEmbeddedPtr]())
	if err == nil {
		createdEmb := regEmb.NewValue("types_test.StructWithNilEmbeddedPtr", map[string]ref.Val{
			"Value":            types.String("outer-val"),
			"NestedCustomName": types.String("inner-val"),
		})
		if types.IsError(createdEmb) {
			t.Errorf("NewValue on StructWithNilEmbeddedPtr failed: %v", createdEmb)
		}
	}
}

type NestedElem struct {
	Name string
}

type ContainerWithNestedCollections struct {
	Elems       []*NestedElem
	ElemMap     map[string]*NestedElem
	StringSlice []string
	StringMap   map[string]string
}

func TestNativeCustomAdapterSliceMapExpressions(t *testing.T) {
	env, err := cel.NewEnv(
		ext.NativeTypes(
			reflect.TypeFor[NestedElem](),
			reflect.TypeFor[ContainerWithNestedCollections](),
		),
		cel.Variable("container", cel.ObjectType("types_test.ContainerWithNestedCollections")),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	container := &ContainerWithNestedCollections{
		Elems: []*NestedElem{
			{Name: "first"},
			{Name: "second"},
		},
		ElemMap: map[string]*NestedElem{
			"k1": {Name: "v1"},
		},
		StringSlice: []string{"a", "b"},
		StringMap:   map[string]string{"k": "v"},
	}

	tests := []struct {
		expr string
		want any
	}{
		{expr: `container.Elems[0].Name`, want: "first"},
		{expr: `container.Elems[1].Name`, want: "second"},
		{expr: `container.Elems.size()`, want: int64(2)},
		{expr: `container.ElemMap["k1"].Name`, want: "v1"},
		{expr: `container.ElemMap.size()`, want: int64(1)},
		{expr: `container.StringSlice[0]`, want: "a"},
		{expr: `container.StringMap["k"]`, want: "v"},
	}

	for _, tc := range tests {
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", tc.expr, err)
		}
		out, _, err := prg.Eval(map[string]any{"container": container})
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		got := out.Value()
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
		}
	}
	type MapKeyStruct struct {
		KeyID string
	}
	type StructWithMapKey struct {
		MapField map[MapKeyStruct]string
	}
	reg, err := types.NewRegistry(reflect.TypeFor[StructWithMapKey]())
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}
	if _, found := reg.FindStructType("types_test.MapKeyStruct"); !found {
		t.Errorf("expected types_test.MapKeyStruct to be registered in NewRegistry")
	}
}

type StructWithDynamicFields struct {
	DynamicField   any
	NilField       any
	RefValField    ref.Val
	SliceOfAny     []any
	MapOfAny       map[string]any
	CustomIface    fmt.Stringer
}

type customStringer struct {
	Val string
}

func (c customStringer) String() string {
	return c.Val
}

func TestNativeInterfaceFieldsExpressions(t *testing.T) {
	env, err := cel.NewEnv(
		ext.NativeTypes(
			reflect.TypeFor[StructWithDynamicFields](),
			reflect.TypeFor[customStringer](),
		),
		cel.Variable("msg", cel.ObjectType("types_test.StructWithDynamicFields")),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	msg := &StructWithDynamicFields{
		DynamicField: "dynamic-string",
		NilField:     nil,
		RefValField:  types.Int(100),
		SliceOfAny:   []any{int64(1), "two", true},
		MapOfAny:     map[string]any{"num": int64(42), "str": "hello"},
		CustomIface:  customStringer{Val: "stringer-val"},
	}

	tests := []struct {
		expr string
		want any
	}{
		{expr: `msg.DynamicField == 'dynamic-string'`, want: true},
		{expr: `has(msg.DynamicField)`, want: true},
		{expr: `msg.NilField == null`, want: true},
		{expr: `has(msg.NilField)`, want: false},
		{expr: `msg.RefValField == 100`, want: true},
		{expr: `has(msg.RefValField)`, want: true},
		{expr: `msg.SliceOfAny[0] == 1 && msg.SliceOfAny[1] == 'two'`, want: true},
		{expr: `msg.SliceOfAny.size() == 3`, want: true},
		{expr: `msg.MapOfAny['num'] == 42 && msg.MapOfAny['str'] == 'hello'`, want: true},
		{expr: `'num' in msg.MapOfAny`, want: true},
		{expr: `msg.CustomIface != null`, want: true},
		{expr: `has(msg.CustomIface)`, want: true},
		{expr: `msg.CustomIface.Val == 'stringer-val'`, want: true},
	}

	for _, tc := range tests {
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", tc.expr, err)
		}
		out, _, err := prg.Eval(map[string]any{"msg": msg})
		if err != nil {
			t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
		}
		got := out.Value()
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
		}
	}
}





