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

package ext

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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/pb"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/test"

	structpb "google.golang.org/protobuf/types/known/structpb"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func TestNativeTypes(t *testing.T) {
	var nativeTests = []struct {
		expr    string
		out     any
		in      any
		envOpts []any
	}{
		{
			expr: `ext.TestAllTypes{
				NestedVal: ext.TestNestedType{NestedMapVal: {1: false}},
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
					ext.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
            custom_name: 'name',
					},
				],
				ArrayVal: [
					ext.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
            custom_name: 'name',
					},
				],
				MapVal: {'map-key': ext.TestAllTypes{BoolVal: true}},
				CustomSliceVal: [ext.TestNestedSliceType{Value: 'none'}],
				CustomMapVal: {'even': ext.TestMapVal{Value: 'more'}},
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
			envOpts: []any{ParseStructTags(true)},
		},

		{
			expr: `ext.TestAllTypes{
				nestedVal: ext.TestNestedType{NestedMapVal: {1: false}},
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
					ext.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
            custom_name: 'name',
					},
				],
				ArrayVal: [
					ext.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
            custom_name: 'name',
					},
				],
				MapVal: {'map-key': ext.TestAllTypes{boolVal: true}},
				CustomSliceVal: [ext.TestNestedSliceType{Value: 'none'}],
				CustomMapVal: {'even': ext.TestMapVal{Value: 'more'}},
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
			envOpts: []any{ParseStructTag("json")},
		},
		{
			expr: `ext.TestAllTypes{
				NestedVal: ext.TestNestedType{NestedMapVal: {1: false}},
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
					ext.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
            NestedCustomName: 'name',
					},
				],
				ArrayVal: [
					ext.TestNestedType{
						NestedListVal:['goodbye', 'cruel', 'world'],
						NestedMapVal: {42: true},
            NestedCustomName: 'name',
					},
				],
				MapVal: {'map-key': ext.TestAllTypes{BoolVal: true}},
				CustomSliceVal: [ext.TestNestedSliceType{Value: 'none'}],
				CustomMapVal: {'even': ext.TestMapVal{Value: 'more'}},
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
			expr: `ext.TestAllTypes{
					PbVal: test.TestAllTypes{single_int32: 123}
				}.PbVal`,
			out: &proto3pb.TestAllTypes{SingleInt32: 123},
		},
		{
			expr: `ext.TestAllTypes{PbVal: test.TestAllTypes{}} ==
			ext.TestAllTypes{PbVal: test.TestAllTypes{single_bool: false}}`,
		},
		{expr: `ext.TestNestedType{} == TestNestedType{}`},
		{expr: `ext.TestAllTypes{}.BoolVal != true`},
		{expr: `!has(ext.TestAllTypes{}.BoolVal) && !has(ext.TestAllTypes{}.NestedVal)`},
		{expr: `type(ext.TestAllTypes) == type`},
		{expr: `type(ext.TestAllTypes{}) == ext.TestAllTypes`},
		{expr: `type(ext.TestAllTypes{}) == ext.TestAllTypes`},
		{expr: `ext.TestAllTypes != test.TestAllTypes`},
		{expr: `ext.TestAllTypes{BoolVal: true} != dyn(test.TestAllTypes{single_bool: true})`},
		{expr: `ext.TestAllTypes{}.NestedVal == ext.TestNestedType{}`},
		{expr: `ext.TestNestedType{} == ext.TestAllTypes{}.NestedStructVal`},
		{expr: `ext.TestAllTypes{}.NestedStructVal == ext.TestNestedType{}`},
		{expr: `ext.TestAllTypes{}.ListVal.size() == 0`},
		{expr: `ext.TestAllTypes{}.MapVal.size() == 0`},
		{expr: `ext.TestAllTypes{}.TimestampVal == timestamp(0)`},
		{expr: `test.TestAllTypes{}.single_timestamp == timestamp(0)`},
		{expr: `[TestAllTypes{BoolVal: true}, TestAllTypes{BoolVal: false}].exists(t, t.BoolVal == true)`},
		{expr: `[TestAllTypes{CustomName: 'Alice'}, TestAllTypes{CustomName: 'Bob'}].exists(t, t.CustomName == 'Alice')`},
		{expr: `[TestAllTypes{custom_name: 'Alice'}, TestAllTypes{custom_name: 'Bob'}].exists(t, t.custom_name == 'Alice')`, envOpts: []any{ParseStructTags(true)}},
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
	env := testNativeEnv(t, ParseStructTags(true))
	provider := env.CELTypeProvider()
	tests := []struct {
		typeName string
		fields   []string
	}{
		{
			typeName: "ext.TestNestedType",
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
			err: `ERROR: <input>:1:13: undeclared reference to 'TestAllTypos' (in container 'ext')
			 | TestAllTypos{}
			 | ............^`,
		},
		{
			expr: `ext.TestAllTypes{bool_val: false}`,
			err: `ERROR: <input>:1:26: undefined field 'bool_val'
			| ext.TestAllTypes{bool_val: false}
			| .........................^`,
		},
		{
			expr: `ext.TestAllTypes{UnsupportedVal: null}`,
			err: `ERROR: <input>:1:32: undefined field 'UnsupportedVal'
			| ext.TestAllTypes{UnsupportedVal: null}
			| ...............................^`,
		},
		{
			expr: `ext.TestAllTypes{UnsupportedListVal: null}`,
			err: `ERROR: <input>:1:36: undefined field 'UnsupportedListVal'
			| ext.TestAllTypes{UnsupportedListVal: null}
			| ...................................^`,
		},
		{
			expr: `ext.TestAllTypes{UnsupportedMapVal: null}`,
			err: `ERROR: <input>:1:35: undefined field 'UnsupportedMapVal'
			| ext.TestAllTypes{UnsupportedMapVal: null}
			| ..................................^`,
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
					'map-key': ext.TestAllTypes{
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
				"BoolVal":  true,
				"CustomName":  "name",
				"DoubleVal":  1.5,
				"DurationVal":  "5s",
				"FloatVal":  2,
				"Int32Val":  23,
				"Int64Val":  64,
				"MapVal": {
	              "map-key": {
    	            "BoolVal": true
        	      }
            	},
				"NestedVal": {
					"NestedListVal": [
					  "first",
					  "second"
					]
				},
				"StringVal":  "string"
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
					'map-key': ext.TestAllTypes{
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
				"BoolVal":  true,
				"DoubleVal":  1.5,
				"DurationVal":  "5s",
				"FloatVal":  2,
				"Int32Val":  23,
				"Int64Val":  64,
				"MapVal": {
	              "map-key": {
    	            "BoolVal": true
        	      }
            	},
				"NestedVal": {
					"NestedListVal": [
					  "first",
					  "second"
					]
				},
				"StringVal":  "string",
                "custom_name": "name"
			  }`,
			additionalEnvOptions: []any{ParseStructTags(true)},
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
			expr: `ext.TestAllTypes{bool_val: false}`,
			err:  `no such field: bool_val`,
		},
		{
			expr: `ext.TestAllTypes{UnsupportedVal: null}`,
			err:  `no such field: UnsupportedVal`,
		},
		{
			expr: `ext.TestAllTypes{UnsupportedListVal: null}`,
			err:  `no such field: UnsupportedListVal`,
		},
		{
			expr: `ext.TestAllTypes{UnsupportedMapVal: null}`,
			err:  `no such field: UnsupportedMapVal`,
		},
		{
			expr: `ext.TestAllTypes{privateVal: null}`,
			err:  `no such field: privateVal`,
		},
		{
			expr: `ext.TestAllTypes{}.UnsupportedMapVal`,
			err:  `no such field: UnsupportedMapVal`,
		},
		{
			expr: `ext.TestAllTypes{}.privateVal`,
			err:  `no such field: privateVal`,
		},
		{
			expr: `ext.TestAllTypes{BoolVal: 'false'}`,
			err:  `unsupported native conversion from string to 'bool'`,
		},
		{
			expr: `has(ext.TestAllTypes{}.BadFieldName)`,
			err:  `no such field: BadFieldName`,
		},
		{
			expr: `ext.TestAllTypes{}[42]`,
			err:  `no such overload`,
		},
		{
			expr: `ext.TestAllTypes{Int32Val: 9223372036854775807}`,
			err:  `integer overflow`,
		},
		{
			expr: `ext.TestAllTypes{Uint32Val: 9223372036854775807u}`,
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
			err:        "must be reflect.Type or reflect.Value",
		},
	}
	for i, tst := range envTests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, err := cel.NewEnv(NativeTypes(tc.nativeType))
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("cel.NewEnv(NativeTypes(%v)) got error %v, wanted %v", tc.nativeType, err, tc.err)
			}
		})
	}
}

func TestNativeTypesConvertToNative(t *testing.T) {
	env := testNativeEnv(t, NativeTypes(reflect.TypeOf(TestNestedType{})))
	adapter := env.CELTypeAdapter()
	conversions := []struct {
		in     any
		inType *cel.Type
		out    any
		err    string
	}{
		{
			in:     &TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("ext.TestAllTypes"),
			out:    &TestAllTypes{BoolVal: true},
		},
		{
			in:     TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("ext.TestAllTypes"),
			out:    &TestAllTypes{BoolVal: true},
		},
		{
			in:     &TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("ext.TestAllTypes"),
			out:    TestAllTypes{BoolVal: true},
		},
		{
			in:     nil,
			inType: cel.NullType,
			out:    types.NullValue,
		},
		{
			in:     &TestAllTypes{BoolVal: true},
			inType: cel.ObjectType("ext.TestAllTypes"),
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
	env := testNativeEnv(t, NativeTypes(reflect.TypeOf(TestNestedType{})))
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
		{expr: `!optional.ofNonZeroValue(ext.TestAllTypes{}).hasValue()`},
		{expr: `!ext.TestAllTypes{}.?BoolVal.orValue(false)`},
		{expr: `!ext.TestAllTypes{}.?BoolVal.hasValue()`},
		{expr: `!ext.TestAllTypes{BoolVal: false}.?BoolVal.hasValue()`},
		{expr: `ext.TestAllTypes{BoolVal: true}.?BoolVal.hasValue()`},
		{expr: `ext.TestAllTypes{}.NestedVal.?NestedMapVal.orValue({}).size() == 0`},
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
			expr: `ext.TestRefValFieldType{optional_name: optional.of('my name')}.optional_name.orValue('') == 'my name'`,
		},
		{
			expr: `ext.TestRefValFieldType{IntVal: 2}.IntVal >= 1`,
		},
		{
			expr: `ext.TestRefValFieldType{time: timestamp('2001-01-01T00:00:00Z')}.time > timestamp('1970-01-01T00:00:00Z')`,
		},
	}
	env := testNativeEnv(t, cel.OptionalTypes(), ParseStructTag("cel"))
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
			handler := fieldNameByTag(tc.tag)
			nt, err := newNativeType(handler, reflect.TypeOf(&TestAllTypes{}))
			if err != nil {
				t.Fatalf("newNativeType() failed: %v", err)
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
	nt, err := newNativeType(fieldNameByTag("cel"), reflect.TypeOf(&TestAllTypes{}))
	if err != nil {
		t.Fatalf("newNativeType() failed: %v", err)
	}
	out, err := nt.ConvertToNative(reflect.TypeOf(1))
	if err == nil {
		t.Errorf("nt.ConvertToNative(1) produced %v, wanted error", out)
	}
}

func TestNativeTypeHasTrait(t *testing.T) {
	nt, err := newNativeType(fieldNameByTag("cel"), reflect.TypeOf(&TestAllTypes{}))
	if err != nil {
		t.Fatalf("newNativeType() failed: %v", err)
	}
	if !nt.HasTrait(traits.IndexerType) || !nt.HasTrait(traits.FieldTesterType) {
		t.Error("nt.HasTrait() failed indicate support for presence test and field access.")
	}
}

func TestNativeTypeValue(t *testing.T) {
	nt, err := newNativeType(fieldNameByTag("cel"), reflect.TypeOf(&TestAllTypes{}))
	if err != nil {
		t.Fatalf("newNativeType() failed: %v", err)
	}
	if nt.Value() != nt.String() {
		t.Errorf("nt.Value() got %v, wanted %v", nt.Value(), nt.String())
	}
}

func TestNativeStructWithMultipleSameFieldNames(t *testing.T) {
	_, err := newNativeType(fieldNameByTag("cel"), reflect.TypeOf(TestStructWithMultipleSameNames{}))
	if err == nil {
		t.Fatal("newNativeType() did not fail as expected")
	}
	if !errors.Is(err, errDuplicatedFieldName) {
		t.Fatalf("newNativeType() exepected duplicated field name error, but got: %v", err)
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
			expr: `test.embedded == ext.TestNestedType{custom_name: "name"}`,
			in: map[string]any{
				"test": &TestEmbeddedTypes{
					TestNestedType: TestNestedType{NestedCustomName: "name"},
					Skipped:        "should-be-hidden",
				},
			},
			out: true,
		},
	}

	envOpts := []cel.EnvOption{
		NativeTypes(
			reflect.TypeFor[*TestEmbeddedTypes](),
			reflect.TypeFor[*TestNestedType](),
			ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("ext.TestEmbeddedTypes")),
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

func TestNativeStructHiddenField(t *testing.T) {
	envOpts := []cel.EnvOption{
		NativeTypes(
			reflect.TypeFor[*TestEmbeddedTypes](),
			ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("ext.TestEmbeddedTypes")),
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
		NativeTypes(
			reflect.ValueOf(&TestNestedStruct{}),
			ParseStructTag("json"),
		),
		cel.Variable("test", cel.ObjectType("ext.TestNestedStruct")),
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
	_, err := cel.NewEnv(NativeTypes(NativeTypesVersion(0)))
	if err != nil {
		t.Fatalf("NewEnv(NativeTypes(NativeTypesVersion(0))) failed: %v", err)
	}
}

func TestTypeResolutionRace(t *testing.T) {
	customType := reflect.TypeFor[*Custom]()
	env, err := cel.NewEnv(
		cel.Container("ext"),
		NativeTypes(
			ParseStructTag("cel"),
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

// testEnv initializes the test environment common to all tests.
func testNativeEnv(t *testing.T, opts ...any) *cel.Env {
	t.Helper()
	envOpts := []cel.EnvOption{
		cel.Container("ext"),
		cel.Abbrevs("google.expr.proto3.test"),
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.Variable("tests", cel.ListType(cel.ObjectType("ext.TestAllTypes"))),
	}
	nativeOpts := []any{
		reflect.ValueOf(&TestAllTypes{}),
		reflect.ValueOf(&TestRefValFieldType{}),
	}
	for _, o := range opts {
		switch opt := o.(type) {
		case NativeTypesOption:
			nativeOpts = append(nativeOpts, opt)
		case cel.EnvOption:
			envOpts = append(envOpts, opt)
		default:
			t.Fatalf("invalid option type: %s", reflect.TypeOf(o).Name())
		}
	}

	envOpts = append(envOpts,
		NativeTypes(
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
	Name        string
	custom_name string `cel:"Name"`
}

type TestNestedType struct {
	NestedListVal    []string
	NestedMapVal     map[int64]bool
	NestedCustomName string `cel:"custom_name" json:"custom_name"`
}

type TestAllTypes struct {
	NestedVal       *TestNestedType `json:"nestedVal,omitempty"`
	NestedStructVal TestNestedType  `json:"nestedStructVal"`
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
	TestNestedType `json:"embedded,omitempty"`
	Skipped        string `json:"-"`
}

type TestRefValFieldType struct {
	OptionalName *types.Optional `cel:"optional_name"`
	IntVal       types.Int
	CELTime      types.Timestamp `cel:"time"`
}
