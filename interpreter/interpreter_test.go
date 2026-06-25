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

package interpreter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"

	proto2pb "github.com/google/cel-go/test/proto2pb"
	proto3pb "github.com/google/cel-go/test/proto3pb"
)

type testCase struct {
	name      string
	expr      string
	container string
	abbrevs   []string
	typeOpts  []types.RegistryOption
	vars      []*decls.VariableDecl
	funcs     []*decls.FunctionDecl
	attrs     AttributeFactory
	unchecked bool
	extraOpts []PlannerOption

	in      any
	out     any
	err     string
	progErr string
}

func testData(t testing.TB) []testCase {
	return []testCase{
		{
			name: "double_ne_nan",
			expr: `0.0/0.0 == 0.0/0.0`,
			out:  types.False,
		},
		{
			name: "and_false_1st",
			expr: `false && true`,
			out:  types.False,
		},
		{
			name: "and_false_2nd",
			expr: `true && false`,
			out:  types.False,
		},
		{
			name: "and_error_1st_false",
			expr: `1/0 != 0 && false`,
			out:  types.False,
		},
		{
			name: "and_error_2nd_false",
			expr: `false && 1/0 != 0`,
			out:  types.False,
		},
		{
			name: "and_error_1st_error",
			expr: `1/0 != 0 && true`,
			err:  "division by zero",
		},
		{
			name: "and_error_2nd_error",
			expr: `true && 1/0 != 0`,
			err:  "division by zero",
		},
		{
			name:      "call_no_args",
			expr:      `zero()`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "zero",
					decls.Overload("zero", []*types.Type{}, types.IntType),
					decls.SingletonFunctionBinding(func(args ...ref.Val) ref.Val {
						return types.IntZero
					}),
				)},
			out: types.IntZero,
		},
		{
			name:      "call_one_arg",
			expr:      `neg(1)`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "neg",
					decls.Overload("neg_int", []*types.Type{types.IntType}, types.IntType,
						decls.OverloadOperandTrait(traits.NegatorType),
						decls.UnaryBinding(func(arg ref.Val) ref.Val {
							return arg.(traits.Negater).Negate()
						}),
					),
				),
			},
			out: types.IntNegOne,
		},
		{
			name:      "call_two_arg",
			expr:      `b'abc'.concat(b'def')`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "concat",
					decls.MemberOverload("bytes_concat_bytes", []*types.Type{types.BytesType, types.BytesType}, types.BytesType,
						decls.OverloadOperandTrait(traits.AdderType),
						decls.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
							return lhs.(traits.Adder).Add(rhs)
						}))),
			},
			out: []byte{'a', 'b', 'c', 'd', 'e', 'f'},
		},
		{
			name:      "call_four_args",
			expr:      `addall(a, b, c, d) == 10`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "addall",
					decls.Overload("addall_four",
						[]*types.Type{types.IntType, types.IntType, types.IntType, types.IntType},
						types.IntType),
					decls.DisableTypeGuards(true),
					decls.SingletonFunctionBinding(func(args ...ref.Val) ref.Val {
						val := types.Int(0)
						for _, arg := range args {
							val += arg.(types.Int)
						}
						return val
					}, traits.AdderType)),
			},
			in: map[string]any{
				"a": 1, "b": 2, "c": 3, "d": 4,
			},
		},
		{
			name: `call_ns_func`,
			expr: `base64.encode('hello')`,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "base64.encode",
					decls.Overload("base64_encode_string", []*types.Type{types.StringType}, types.StringType),
					decls.SingletonUnaryBinding(base64Encode)),
			},
			out: "aGVsbG8=",
		},
		{
			name:      `call_ns_func_unchecked`,
			expr:      `base64.encode('hello')`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "base64.encode",
					decls.Overload("base64_encode_string", []*types.Type{types.StringType}, types.StringType),
					decls.SingletonUnaryBinding(base64Encode)),
			},
			out: "aGVsbG8=",
		},
		{
			name:      `call_ns_func_in_pkg`,
			container: `base64`,
			expr:      `encode('hello')`,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "base64.encode",
					decls.Overload("base64_encode_string", []*types.Type{types.StringType}, types.StringType),
					decls.SingletonUnaryBinding(base64Encode)),
			},
			out: "aGVsbG8=",
		},
		{
			name:      `call_ns_func_unchecked_in_pkg`,
			expr:      `encode('hello')`,
			container: `base64`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "base64.encode",
					decls.Overload("base64_encode_string", []*types.Type{types.StringType}, types.StringType),
					decls.SingletonUnaryBinding(base64Encode)),
			},
			out: "aGVsbG8=",
		},
		{
			name: "complex",
			expr: `
			!(headers.ip in ["10.0.1.4", "10.0.1.5"]) &&
				((headers.path.startsWith("v1") && headers.token in ["v1", "v2", "admin"]) ||
				(headers.path.startsWith("v2") && headers.token in ["v2", "admin"]) ||
				(headers.path.startsWith("/admin") && headers.token == "admin" && headers.ip in ["10.0.1.2", "10.0.1.2", "10.0.1.2"]))
			`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("headers", types.NewMapType(types.StringType, types.StringType)),
			},
			in: map[string]any{
				"headers": map[string]any{
					"ip":    "10.0.1.2",
					"path":  "/admin/edit",
					"token": "admin",
				},
			},
		},
		{
			name: "complex_qual_vars",
			expr: `
			!(headers.ip in ["10.0.1.4", "10.0.1.5"]) &&
				((headers.path.startsWith("v1") && headers.token in ["v1", "v2", "admin"]) ||
				(headers.path.startsWith("v2") && headers.token in ["v2", "admin"]) ||
				(headers.path.startsWith("/admin") && headers.token == "admin" && headers.ip in ["10.0.1.2", "10.0.1.2", "10.0.1.2"]))
			`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("headers.ip", types.StringType),
				decls.NewVariable("headers.path", types.StringType),
				decls.NewVariable("headers.token", types.StringType),
			},
			in: map[string]any{
				"headers.ip":    "10.0.1.2",
				"headers.path":  "/admin/edit",
				"headers.token": "admin",
			},
		},
		{
			name: "cond",
			expr: `a ? b < 1.2 : c == ['hello']`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.BoolType),
				decls.NewVariable("b", types.DoubleType),
				decls.NewVariable("c", types.NewListType(types.StringType)),
			},
			in: map[string]any{
				"a": true,
				"b": 2.0,
				"c": []string{"hello"},
			},
			out: types.False,
		},
		{
			name: "cond_attr_out_of_bounds_error",
			expr: `m[(x ? 0 : 1)] >= 0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewListType(types.IntType)),
				decls.NewVariable("x", types.BoolType),
			},
			in: map[string]any{
				"m": []int{-1},
				"x": false,
			},
			err: "index out of bounds: 1",
		},
		{
			name: "cond_attr_qualify_bad_type_error",
			expr: `m[(x ? a : b)] >= 0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewListType(types.DynType)),
				decls.NewVariable("a", types.DynType),
				decls.NewVariable("b", types.DynType),
				decls.NewVariable("x", types.BoolType),
			},
			in: map[string]any{
				"m": []int{1},
				"x": false,
				"a": time.Millisecond,
				"b": time.Millisecond,
			},
			err: "invalid qualifier type",
		},
		{
			name: "cond_attr_qualify_bad_field_error",
			expr: `m[(x ? a : b).c] >= 0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewListType(types.DynType)),
				decls.NewVariable("a", types.DynType),
				decls.NewVariable("b", types.DynType),
				decls.NewVariable("x", types.BoolType),
			},
			in: map[string]any{
				"m": []int{1},
				"x": false,
				"a": int32(1),
				"b": int32(2),
			},
			err: "no such key: c",
		},
		{
			name: "in_empty_list",
			expr: `6 in []`,
			out:  types.False,
		},
		{
			name: "in_constant_list",
			expr: `6 in [2, 12, 6]`,
		},
		{
			name: "bytes_in_constant_list",
			expr: "b'hello' in [b'world', b'universe', b'hello']",
		},
		{
			name: "list_in_constant_list",
			expr: `[6] in [2, 12, [6]]`,
		},
		{
			name: "in_constant_list_cross_type_uint_int",
			expr: `dyn(12u) in [2, 12, 6]`,
		},
		{
			name: "in_constant_list_cross_type_double_int",
			expr: `dyn(6.0) in [2, 12, 6]`,
		},
		{
			name: "in_constant_list_cross_type_int_double",
			expr: `dyn(6) in [2.1, 12.0, 6.0]`,
		},
		{
			name: "not_in_constant_list_cross_type_int_double",
			expr: `dyn(2) in [2.1, 12.0, 6.0]`,
			out:  types.False,
		},
		{
			name: "in_constant_list_cross_type_int_uint",
			expr: `dyn(6) in [2u, 12u, 6u]`,
		},
		{
			name: "in_constant_list_cross_type_negative_int_uint",
			expr: `dyn(-6) in [2u, 12u, 6u]`,
			out:  types.False,
		},
		{
			name: "in_constant_list_cross_type_negative_double_uint",
			expr: `dyn(-6.1) in [2u, 12u, 6u]`,
			out:  types.False,
		},
		{
			name: "in_var_list_int",
			expr: `6 in [2, 12, x]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
			},
			in: map[string]any{"x": 6},
		},
		{
			name: "in_var_list_uint",
			expr: `6 in [2, 12, x]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
			},
			in: map[string]any{"x": uint64(6)},
		},
		{
			name: "in_var_list_double",
			expr: `6 in [2, 12, x]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
			},
			in: map[string]any{"x": 6.0},
		},
		{
			name: "in_var_list_double_double",
			expr: `dyn(6.0) in [2, 12, x]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
			},
			in: map[string]any{"x": 6},
		},
		{
			name: "in_constant_map",
			expr: `'other-key' in {'key': null, 'other-key': 42}`,
			out:  types.True,
		},
		{
			name: "in_constant_map_cross_type_string_number",
			expr: `'other-key' in {1: null, 2u: 42}`,
			out:  types.False,
		},
		{
			name: "in_constant_map_cross_type_double_int",
			expr: `2.0 in {1: null, 2u: 42}`,
		},
		{
			name: "not_in_constant_map_cross_type_double_int",
			expr: `2.1 in {1: null, 2u: 42}`,
			out:  types.False,
		},
		{
			name: "in_constant_heterogeneous_map",
			expr: `'hello' in {1: 'one', false: true, 'hello': 'world'}`,
			out:  types.True,
		},
		{
			name: "not_in_constant_heterogeneous_map",
			expr: `!('hello' in {1: 'one', false: true})`,
			out:  types.True,
		},
		{
			name: "not_in_constant_heterogeneous_map_with_same_key_type",
			expr: `!('hello' in {1: 'one', 'world': true})`,
			out:  types.True,
		},
		{
			name: "in_var_key_map",
			expr: `'other-key' in {x: null, y: 42}`,
			out:  types.True,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.StringType),
				decls.NewVariable("y", types.IntType),
			},
			in: map[string]any{
				"x": "other-key",
				"y": 2,
			},
		},
		{
			name: "in_var_value_map",
			expr: `'other-key' in {1: x, 2u: y}`,
			out:  types.False,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.StringType),
				decls.NewVariable("y", types.IntType),
			},
			in: map[string]any{
				"x": "other-value",
				"y": 2,
			},
		},
		{
			name: "index",
			expr: `m['key'][1] == 42u && m['null'] == null && m[string(0)] == 10`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.StringType, types.DynType)),
			},
			in: map[string]any{
				"m": map[string]any{
					"key":  []uint{21, 42},
					"null": nil,
					"0":    10,
				},
			},
		},
		{
			name: "index_cross_type_float_uint",
			expr: `{1: 'hello'}[x] == 'hello' && {2: 'world'}[y] == 'world'`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
				decls.NewVariable("y", types.DynType),
			},
			in: map[string]any{
				"x": float32(1.0),
				"y": uint(2),
			},
		},
		{
			name: "no_index_cross_type_float_uint",
			expr: `{1: 'hello'}[x] == 'hello' && ['world'][y] == 'world'`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
				decls.NewVariable("y", types.DynType),
			},
			in: map[string]any{
				"x": float32(2.0),
				"y": uint(3),
			},
			err: "no such key: 2",
		},
		{
			name: "index_cross_type_double",
			expr: `{1: 'hello', 2: 'world'}[x] == 'hello'`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
			},
			in: map[string]any{
				"x": 1.0,
			},
		},
		{
			name: "index_cross_type_double_const",
			expr: `{1: 'hello', 2: 'world'}[dyn(2.0)] == 'world'`,
		},
		{
			name: "index_cross_type_uint",
			expr: `{1: 'hello', 2: 'world'}[dyn(2u)] == 'world'`,
		},
		{
			name: "index_cross_type_bad_qualifier",
			expr: `{1: 'hello', 2: 'world'}[x] == 'world'`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.DynType),
			},
			in: map[string]any{
				"x": time.Millisecond,
			},
			err: "invalid qualifier type",
		},
		{
			name: "index_list_int_double_type_index",
			expr: `[7, 8, 9][dyn(0.0)] == 7`,
		},
		{
			name: "index_list_int_uint_type_index",
			expr: `[7, 8, 9][dyn(0u)] == 7`,
		},
		{
			name: "index_list_int_bad_double_type_index",
			expr: `[7, 8, 9][dyn(0.1)] == 7`,
			err:  `unsupported index value`,
		},
		{
			name: "index_relative",
			expr: `([[[1]], [[2]], [[3]]][0][0] + [2, 3, {'four': {'five': 'six'}}])[3].four.five == 'six'`,
		},
		{
			name: "list_eq_false_with_error",
			expr: `['string', 1] == [2, 3]`,
			out:  types.False,
		},
		{
			name: "list_eq_error",
			expr: `['string', true] == [2, 3]`,
			out:  types.False,
		},
		{
			name: "literal_bool_false",
			expr: `false`,
			out:  types.False,
		},
		{
			name: "literal_bool_true",
			expr: `true`,
		},
		{
			name: "literal_null",
			expr: `null`,
			out:  types.NullValue,
		},
		{
			name: "literal_list",
			expr: `[1, 2, 3]`,
			out:  []int64{1, 2, 3},
		},
		{
			name: "literal_map",
			expr: `{'hi': 21, 'world': 42u}`,
			out: map[string]any{
				"hi":    21,
				"world": uint(42),
			},
		},
		{
			name: "literal_equiv_string_bytes",
			expr: `string(bytes("\303\277")) == '''\303\277'''`,
		},
		{
			name: "literal_not_equiv_string_bytes",
			expr: `string(b"\303\277") != '''\303\277'''`,
		},
		{
			name: "literal_equiv_bytes_string",
			expr: `string(b"\303\277") == 'ÿ'`,
		},
		{
			name: "literal_bytes_string",
			expr: `string(b'aaa"bbb')`,
			out:  `aaa"bbb`,
		},
		{
			name: "literal_bytes_string2",
			expr: `string(b"""Kim\t""")`,
			out:  `Kim	`,
		},
		{
			name:      "literal_pb3_msg",
			container: "google.api.expr",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&exprpb.Expr{})},
			expr: `v1alpha1.Expr{
				id: 1,
				const_expr: v1alpha1.Constant{
					string_value: "oneof_test"
				}
			}`,
			out: &exprpb.Expr{Id: 1,
				ExprKind: &exprpb.Expr_ConstExpr{
					ConstExpr: &exprpb.Constant{
						ConstantKind: &exprpb.Constant_StringValue{
							StringValue: "oneof_test"}}}},
		},
		{
			name:      "literal_pb_enum",
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			expr: `TestAllTypes{
				repeated_nested_enum: [
					0,
					TestAllTypes.NestedEnum.BAZ,
					TestAllTypes.NestedEnum.BAR],
				repeated_int32: [
					TestAllTypes.NestedEnum.FOO,
					TestAllTypes.NestedEnum.BAZ]}`,
			out: &proto3pb.TestAllTypes{
				RepeatedNestedEnum: []proto3pb.TestAllTypes_NestedEnum{
					proto3pb.TestAllTypes_FOO,
					proto3pb.TestAllTypes_BAZ,
					proto3pb.TestAllTypes_BAR,
				},
				RepeatedInt32: []int32{0, 2},
			},
		},
		{
			name:      "literal_pb_wrapper_assign",
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			expr: `TestAllTypes{
				single_int64_wrapper: 10,
				single_int32_wrapper: TestAllTypes{}.single_int32_wrapper,
			}`,
			out: &proto3pb.TestAllTypes{
				SingleInt64Wrapper: wrapperspb.Int64(10),
			},
		},
		{
			name:      "literal_pb_wrapper_assign_roundtrip",
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			expr: `TestAllTypes{
				single_int32_wrapper: TestAllTypes{}.single_int32_wrapper,
			}.single_int32_wrapper == null`,
			out: true,
		},
		{
			name:      "literal_pb_list_assign_null_wrapper",
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			expr: `TestAllTypes{
				repeated_int32: [123, 456, TestAllTypes{}.single_int32_wrapper],
			}`,
			err: "field type conversion error",
		},
		{
			name:      "literal_pb_map_assign_null_entry_value",
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			expr: `TestAllTypes{
				map_string_string: {
					'hello': 'world',
					'goodbye': TestAllTypes{}.single_string_wrapper,
				},
			}`,
			err: "field type conversion error",
		},
		{
			name:      "unset_wrapper_access",
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			expr:      `TestAllTypes{}.single_string_wrapper`,
			out:       types.NullValue,
		},
		{
			name: "timestamp_eq_timestamp",
			expr: `timestamp(0) == timestamp(0)`,
		},
		{
			name: "timestamp_ne_timestamp",
			expr: `timestamp(1) != timestamp(2)`,
		},
		{
			name: "timestamp_lt_timestamp",
			expr: `timestamp(0) < timestamp(1)`,
		},
		{
			name: "timestamp_le_timestamp",
			expr: `timestamp(2) <= timestamp(2)`,
		},
		{
			name: "timestamp_gt_timestamp",
			expr: `timestamp(1) > timestamp(0)`,
		},
		{
			name: "timestamp_ge_timestamp",
			expr: `timestamp(2) >= timestamp(2)`,
		},
		{
			name: "timestamp_methods",
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.TimestampType),
			},
			in: map[string]any{
				"x": time.Unix(7506, 1000000).Local(),
			},
			expr: `
			x.getFullYear() == 1970
			&& x.getMonth() == 0
			&& x.getDayOfYear() == 0
			&& x.getDayOfMonth() == 0
			&& x.getDate() == 1
			&& x.getDayOfWeek() == 4
			&& x.getHours() == 2
			&& x.getMinutes() == 5
			&& x.getSeconds() == 6
			&& x.getMilliseconds() == 1
			&& x.getFullYear('-07:30') == 1969
			&& x.getDayOfYear('-07:30') == 364
			&& x.getMonth('-07:30') == 11
			&& x.getDayOfMonth('-07:30') == 30
			&& x.getDate('-07:30') == 31
			&& x.getDayOfWeek('-07:30') == 3
			&& x.getHours('-07:30') == 18
			&& x.getMinutes('-07:30') == 35
			&& x.getSeconds('-07:30') == 6
			&& x.getMilliseconds('-07:30') == 1
			&& x.getFullYear('23:15') == 1970
			&& x.getDayOfYear('23:15') == 1
			&& x.getMonth('23:15') == 0
			&& x.getDayOfMonth('23:15') == 1
			&& x.getDate('23:15') == 2
			&& x.getDayOfWeek('23:15') == 5
			&& x.getHours('23:15') == 1
			&& x.getMinutes('23:15') == 20
			&& x.getSeconds('23:15') == 6
			&& x.getMilliseconds('23:15') == 1`,
		},
		{
			name: "string_to_timestamp",
			expr: `timestamp('1986-04-26T01:23:40Z')`,
			out:  &tpb.Timestamp{Seconds: 514862620},
		},
		{
			name: "macro_all_non_strict",
			expr: `![0, 2, 4].all(x, 4/x != 2 && 4/(4-x) != 2)`,
		},
		{
			name: "macro_all_non_strict_var",
			expr: `code == "111" && ["a", "b"].all(x, x in tags)
				|| code == "222" && ["a", "b"].all(x, x in tags)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("code", types.StringType),
				decls.NewVariable("tags", types.NewListType(types.StringType)),
			},
			in: map[string]any{
				"code": "222",
				"tags": []string{"a", "b"},
			},
		},
		{
			name: "macro_exists_lit",
			expr: `[1, 2, 3, 4, 5u, 1.0].exists(e, type(e) == uint)`,
		},
		{
			name: "macro_exists_nonstrict",
			expr: `[0, 2, 4].exists(x, 4/x == 2 && 4/(4-x) == 2)`,
		},
		{
			name: "macro_exists_var",
			expr: `elems.exists(e, type(e) == uint)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("elems", types.NewListType(types.DynType)),
			},
			in: map[string]any{
				"elems": []any{0, 1, 2, 3, 4, uint(5), 6},
			},
		},
		{
			name: "macro_exists_one",
			expr: `[1, 2, 3].exists_one(x, (x % 2) == 0)`,
		},
		{
			name: "macro_filter",
			expr: `[-10, -9, -8, -7, -6, -5, -4, -3, -2, -1, 0, 1, 2, 3].filter(x, x > 0)`,
			out:  []int64{1, 2, 3},
		},
		{
			name: "macro_has_map_key",
			expr: `has({'a':1}.a) && !has({}.a)`,
		},
		{
			name:      "macro_has_pb2_field_undefined",
			container: "google.expr.proto2.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto2pb.TestAllTypes{})},
			unchecked: true,
			expr:      `has(TestAllTypes{}.invalid_field)`,
			err:       "no such field 'invalid_field'",
		},
		{
			name:      "macro_has_pb2_field",
			container: "google.expr.proto2.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto2pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("pb2", types.NewObjectType("google.expr.proto2.test.TestAllTypes")),
			},
			in: map[string]any{
				"pb2": &proto2pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto2pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			expr: `has(TestAllTypes{standalone_enum: TestAllTypes.NestedEnum.BAR}.standalone_enum)
			&& has(TestAllTypes{standalone_enum: TestAllTypes.NestedEnum.FOO}.standalone_enum)
			&& !has(TestAllTypes{single_nested_enum: TestAllTypes.NestedEnum.FOO}.single_nested_message)
			&& has(TestAllTypes{single_nested_enum: TestAllTypes.NestedEnum.FOO}.single_nested_enum)
			&& !has(TestAllTypes{}.standalone_enum)
			&& !has(pb2.single_int64)
			&& has(pb2.repeated_bool)
			&& !has(pb2.repeated_int32)
			&& has(pb2.map_int64_nested_type)
			&& !has(pb2.map_string_string)`,
		},
		{
			name:      "macro_has_pb2_field_json",
			container: "google.expr.proto2.test",
			typeOpts:  []types.RegistryOption{types.JSONFieldNames(true), types.ProtoTypeDefs(&proto2pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("pb2", types.NewObjectType("google.expr.proto2.test.TestAllTypes")),
			},
			in: map[string]any{
				"pb2": &proto2pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto2pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			expr: `has(TestAllTypes{standaloneEnum: TestAllTypes.NestedEnum.BAR}.standaloneEnum)
			&& has(TestAllTypes{standaloneEnum: TestAllTypes.NestedEnum.FOO}.standaloneEnum)
			&& !has(TestAllTypes{singleNestedEnum: TestAllTypes.NestedEnum.FOO}.singleNestedMessage)
			&& has(TestAllTypes{singleNestedEnum: TestAllTypes.NestedEnum.FOO}.singleNestedEnum)
			&& !has(TestAllTypes{}.singleNestedMessage)
			&& has(TestAllTypes{singleNestedMessage: TestAllTypes.NestedMessage{}}.singleNestedMessage)
			&& !has(TestAllTypes{}.standaloneEnum)
			&& !has(pb2.singleInt64)
			&& has(pb2.repeatedBool)
			&& !has(pb2.repeatedInt32)
			&& has(pb2.mapInt64NestedType)
			&& !has(pb2.mapStringString)`,
		},
		{
			name:     "macro_has_pb3_field",
			typeOpts: []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("pb3", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
			},
			container: "google.expr.proto3.test",
			in: map[string]any{
				"pb3": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			expr: `has(TestAllTypes{standalone_enum: TestAllTypes.NestedEnum.BAR}.standalone_enum)
			&& !has(TestAllTypes{standalone_enum: TestAllTypes.NestedEnum.FOO}.standalone_enum)
			&& !has(TestAllTypes{single_nested_enum: TestAllTypes.NestedEnum.FOO}.single_nested_message)
			&& has(TestAllTypes{single_nested_enum: TestAllTypes.NestedEnum.FOO}.single_nested_enum)
			&& !has(TestAllTypes{}.single_nested_message)
			&& has(TestAllTypes{single_nested_message: TestAllTypes.NestedMessage{}}.single_nested_message)
			&& !has(TestAllTypes{}.standalone_enum)
			&& !has(pb3.single_int64)
			&& has(pb3.repeated_bool)
			&& !has(pb3.repeated_int32)
			&& has(pb3.map_int64_nested_type)
			&& !has(pb3.map_string_string)`,
		},
		{
			name:     "macro_has_pb3_field_json",
			typeOpts: []types.RegistryOption{types.JSONFieldNames(true), types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("pb3", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
			},
			container: "google.expr.proto3.test",
			in: map[string]any{
				"pb3": &proto3pb.TestAllTypes{
					RepeatedBool: []bool{false},
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						1: {},
					},
					MapStringString: map[string]string{},
				},
			},
			expr: `has(TestAllTypes{standaloneEnum: TestAllTypes.NestedEnum.BAR}.standaloneEnum)
			&& !has(TestAllTypes{standaloneEnum: TestAllTypes.NestedEnum.FOO}.standaloneEnum)
			&& !has(TestAllTypes{singleNestedEnum: TestAllTypes.NestedEnum.FOO}.singleNestedMessage)
			&& has(TestAllTypes{singleNestedEnum: TestAllTypes.NestedEnum.FOO}.singleNestedEnum)
			&& !has(TestAllTypes{}.singleNestedMessage)
			&& has(TestAllTypes{singleNestedMessage: TestAllTypes.NestedMessage{}}.singleNestedMessage)
			&& !has(TestAllTypes{}.standaloneEnum)
			&& !has(pb3.singleInt64)
			&& has(pb3.repeatedBool)
			&& !has(pb3.repeatedInt32)
			&& has(pb3.mapInt64NestedType)
			&& !has(pb3.mapStringString)`,
		},
		{
			name: "macro_map",
			expr: `[1, 2, 3].map(x, x * 2) == [2, 4, 6]`,
		},
		{
			name: "matches_global",
			expr: `matches(input, 'k.*')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			in: map[string]any{
				"input": "kathmandu",
			},
		},
		{
			name: "matches_member",
			expr: `input.matches('k.*')
				&& !'foo'.matches('k.*')
				&& !'bar'.matches('k.*')
				&& 'kilimanjaro'.matches('.*ro')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			in: map[string]any{
				"input": "kathmandu",
			},
		},
		{
			name: "matches_error",
			expr: `input.matches(')k.*')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			in: map[string]any{
				"input": "kathmandu",
			},
			extraOpts: []PlannerOption{CompileRegexConstants(MatchesRegexOptimization)},
			// unoptimized program should report a regex compile error at runtime
			err: "unexpected ): `)k.*`",
			// optimized program should report a regex compile at program creation time
			progErr: "unexpected ): `)k.*`",
		},
		{
			name:     "nested_proto_field",
			expr:     `pb3.single_nested_message.bb`,
			typeOpts: []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("pb3",
					types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
			},
			in: map[string]any{
				"pb3": &proto3pb.TestAllTypes{
					NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{
						SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{
							Bb: 1234,
						},
					},
				},
			},
			out: types.Int(1234),
		},
		{
			name:     "nested_proto_field_with_index",
			expr:     `pb3.map_int64_nested_type[0].child.payload.single_int32 == 1`,
			typeOpts: []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("pb3",
					types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
			},
			in: map[string]any{
				"pb3": &proto3pb.TestAllTypes{
					MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
						0: {
							Child: &proto3pb.NestedTestAllTypes{
								Payload: &proto3pb.TestAllTypes{
									SingleInt32: 1,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "or_true_1st",
			expr: `ai == 20 || ar["foo"] == "bar"`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("ai", types.IntType),
				decls.NewVariable("ar", types.NewMapType(types.StringType, types.StringType)),
			},
			in: map[string]any{
				"ai": 20,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: "or_true_2nd",
			expr: `ai == 20 || ar["foo"] == "bar"`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("ai", types.IntType),
				decls.NewVariable("ar", types.NewMapType(types.StringType, types.StringType)),
			},
			in: map[string]any{
				"ai": 2,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: "or_false",
			expr: `ai == 20 || ar["foo"] == "bar"`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("ai", types.IntType),
				decls.NewVariable("ar", types.NewMapType(types.StringType, types.StringType)),
			},
			in: map[string]any{
				"ai": 2,
				"ar": map[string]string{
					"foo": "baz",
				},
			},
			out: types.False,
		},
		{
			name: "or_error_1st_error",
			expr: `1/0 != 0 || false`,
			err:  "division by zero",
		},
		{
			name: "or_error_2nd_error",
			expr: `false || 1/0 != 0`,
			err:  "division by zero",
		},
		{
			name: "or_error_1st_true",
			expr: `1/0 != 0 || true`,
			out:  types.True,
		},
		{
			name: "or_error_2nd_true",
			expr: `true || 1/0 != 0`,
			out:  types.True,
		},
		{
			name:      "pkg_qualified_id",
			expr:      `b.c.d != 10`,
			container: "a.b",
			vars: []*decls.VariableDecl{
				decls.NewVariable("a.b.c.d", types.IntType),
			},
			in: map[string]any{
				"a.b.c.d": 9,
			},
		},
		{
			name:      "pkg_qualified_id_unchecked",
			expr:      `c.d != 10`,
			unchecked: true,
			container: "a.b",
			in: map[string]any{
				"a.c.d": 9,
			},
		},
		{
			name:      "pkg_qualified_index_unchecked",
			expr:      `b.c['d'] == 10`,
			unchecked: true,
			container: "a.b",
			in: map[string]any{
				"a.b.c": map[string]int{
					"d": 10,
				},
			},
		},
		{
			name: "type_dyn_equals_string",
			expr: `type(dyn('')) == string`,
		},
		{
			name: "type_override",
			expr: `type == 'string'`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("type", types.StringType),
			},
			in: map[string]any{
				"type": "string",
			},
		},
		{
			name: "select_key",
			expr: `m.strMap['val'] == 'string'
				&& m.floatMap['val'] == 1.5
				&& m.doubleMap['val'] == -2.0
				&& m.intMap['val'] == -3
				&& m.int32Map['val'] == 4
				&& m.int64Map['val'] == -5
				&& m.uintMap['val'] == 6u
				&& m.uint32Map['val'] == 7u
				&& m.uint64Map['val'] == 8u
				&& m.boolMap['val'] == true
				&& m.boolMap['val'] != false`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.StringType, types.DynType)),
			},
			in: map[string]any{
				"m": map[string]any{
					"strMap":    map[string]string{"val": "string"},
					"floatMap":  map[string]float32{"val": 1.5},
					"doubleMap": map[string]float64{"val": -2.0},
					"intMap":    map[string]int{"val": -3},
					"int32Map":  map[string]int32{"val": 4},
					"int64Map":  map[string]int64{"val": -5},
					"uintMap":   map[string]uint{"val": 6},
					"uint32Map": map[string]uint32{"val": 7},
					"uint64Map": map[string]uint64{"val": 8},
					"boolMap":   map[string]bool{"val": true},
				},
			},
		},
		{
			name: "select_bool_key",
			expr: `m.boolStr[true] == 'string'
				&& m.boolFloat32[true] == 1.5
				&& m.boolFloat64[false] == -2.1
				&& m.boolInt[false] == -3
				&& m.boolInt32[false] == 0
				&& m.boolInt64[true] == 4
				&& m.boolUint[true] == 5u
				&& m.boolUint32[true] == 6u
				&& m.boolUint64[false] == 7u
				&& m.boolBool[true]
				&& m.boolIface[false] == true`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.StringType, types.DynType)),
			},
			in: map[string]any{
				"m": map[string]any{
					"boolStr":     map[bool]string{true: "string"},
					"boolFloat32": map[bool]float32{true: 1.5},
					"boolFloat64": map[bool]float64{false: -2.1},
					"boolInt":     map[bool]int{false: -3},
					"boolInt32":   map[bool]int32{false: 0},
					"boolInt64":   map[bool]int64{true: 4},
					"boolUint":    map[bool]uint{true: 5},
					"boolUint32":  map[bool]uint32{true: 6},
					"boolUint64":  map[bool]uint64{false: 7},
					"boolBool":    map[bool]bool{true: true},
					"boolIface":   map[bool]any{false: true},
				},
			},
		},
		{
			name: "select_uint_key",
			expr: `m.uintIface[1u] == 'string'
				&& m.uint32Iface[2u] == 1.5
				&& m.uint64Iface[3u] == -2.1
				&& m.uint64String[4u] == 'three'`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.StringType, types.DynType)),
			},
			in: map[string]any{
				"m": map[string]any{
					"uintIface":    map[uint]any{1: "string"},
					"uint32Iface":  map[uint32]any{2: 1.5},
					"uint64Iface":  map[uint64]any{3: -2.1},
					"uint64String": map[uint64]string{4: "three"},
				},
			},
		},
		{
			name: "select_index",
			expr: `m.strList[0] == 'string'
				&& m.floatList[0] == 1.5
				&& m.doubleList[0] == -2.0
				&& m.intList[0] == -3
				&& m.int32List[0] == 4
				&& m.int64List[0] == -5
				&& m.uintList[0] == 6u
				&& m.uint32List[0] == 7u
				&& m.uint64List[0] == 8u
				&& m.boolList[0] == true
				&& m.boolList[1] != true
				&& m.ifaceList[0] == {}`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.StringType, types.DynType)),
			},
			in: map[string]any{
				"m": map[string]any{
					"strList":    []string{"string"},
					"floatList":  []float32{1.5},
					"doubleList": []float64{-2.0},
					"intList":    []int{-3},
					"int32List":  []int32{4},
					"int64List":  []int64{-5},
					"uintList":   []uint{6},
					"uint32List": []uint32{7},
					"uint64List": []uint64{8},
					"boolList":   []bool{true, false},
					"ifaceList":  []any{map[string]string{}},
				},
			},
		},
		{
			name: "select_field",
			expr: `a.b.c
				&& pb3.repeated_nested_enum[0] == test.TestAllTypes.NestedEnum.BAR
				&& json.list[0] == 'world'`,
			container: "google.expr.proto3",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a.b", types.NewMapType(types.StringType, types.BoolType)),
				decls.NewVariable("pb3", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				decls.NewVariable("json", types.NewMapType(types.StringType, types.DynType)),
			},
			in: map[string]any{
				"a.b": map[string]bool{
					"c": true,
				},
				"pb3": &proto3pb.TestAllTypes{
					RepeatedNestedEnum: []proto3pb.TestAllTypes_NestedEnum{proto3pb.TestAllTypes_BAR},
				},
				"json": &structpb.Value{
					Kind: &structpb.Value_StructValue{
						StructValue: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"list": {Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											structpb.NewStringValue("world"),
										},
									},
								}},
							},
						},
					},
				},
			},
		},
		// pb2 primitive fields may have default values set.
		{
			name: "select_pb2_primitive_fields",
			expr: `!has(a.single_int32)
			&& a.single_int32 == -32
			&& a.single_int64 == -64
			&& a.single_uint32 == 32u
			&& a.single_uint64 == 64u
			&& a.single_float == 3.0
			&& a.single_double == 6.4
			&& a.single_bool
			&& "empty" == a.single_string`,
			typeOpts: []types.RegistryOption{types.ProtoTypeDefs(&proto2pb.TestAllTypes{})},
			in: map[string]any{
				"a": &proto2pb.TestAllTypes{},
			},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewObjectType("google.expr.proto2.test.TestAllTypes")),
			},
		},
		// Wrapper type nil or value test.
		{
			name: "select_pb3_wrapper_fields",
			expr: `!has(a.single_int32_wrapper) && a.single_int32_wrapper == null
				&& has(a.single_int64_wrapper) && a.single_int64_wrapper == 0
				&& has(a.single_string_wrapper) && a.single_string_wrapper == "hello"
				&& a.single_int64_wrapper == Int32Value{value: 0}`,
			typeOpts: []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			abbrevs:  []string{"google.protobuf.Int32Value"},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
			},
			in: map[string]any{
				"a": &proto3pb.TestAllTypes{
					SingleInt64Wrapper:  &wrapperspb.Int64Value{},
					SingleStringWrapper: wrapperspb.String("hello"),
				},
			},
		},
		{
			name:      "select_pb3_compare",
			expr:      `a.single_uint64 > 3u`,
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
			},
			in: map[string]any{
				"a": &proto3pb.TestAllTypes{
					SingleUint64: 10,
				},
			},
			out: types.True,
		},
		{
			name:      "select_custom_pb3_compare",
			expr:      `a.bb > 100`,
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes_NestedMessage{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a",
					types.NewObjectType("google.expr.proto3.test.TestAllTypes.NestedMessage")),
			},
			attrs: &custAttrFactory{
				AttributeFactory: NewAttributeFactory(
					testContainer("google.expr.proto3.test"),
					newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes_NestedMessage{})),
					newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes_NestedMessage{})),
				),
			},
			in: map[string]any{
				"a": &proto3pb.TestAllTypes_NestedMessage{
					Bb: 101,
				},
			},
			out: types.True,
		},
		{
			name:      "select_custom_pb3_optional_field",
			expr:      `a.?bb`,
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes_NestedMessage{})},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a",
					types.NewObjectType("google.expr.proto3.test.TestAllTypes.NestedMessage")),
			},
			attrs: &custAttrFactory{
				AttributeFactory: NewAttributeFactory(
					testContainer("google.expr.proto3.test"),
					newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes_NestedMessage{})),
					newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes_NestedMessage{})),
				),
			},
			in: map[string]any{
				"a": &proto3pb.TestAllTypes_NestedMessage{
					Bb: 101,
				},
			},
			out: types.OptionalOf(types.Int(101)),
		},
		{
			name: "select_relative",
			expr: `json('{"hi":"world"}').hi == 'world'`,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "json",
					decls.Overload("json_string", []*types.Type{types.StringType}, types.DynType,
						decls.UnaryBinding(func(val ref.Val) ref.Val {
							str, ok := val.(types.String)
							if !ok {
								return types.MaybeNoSuchOverloadErr(val)
							}
							m := make(map[string]any)
							err := json.Unmarshal([]byte(str), &m)
							if err != nil {
								return types.NewErr("invalid json: %v", err)
							}
							return types.DefaultTypeAdapter.NativeToValue(m)
						}),
					),
				),
			},
		},
		{
			name: "select_subsumed_field",
			expr: `a.b.c`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a.b.c", types.IntType),
				decls.NewVariable("a.b", types.NewMapType(types.StringType, types.StringType)),
			},
			in: map[string]any{
				"a.b.c": 10,
				"a.b": map[string]string{
					"c": "ten",
				},
			},
			out: types.Int(10),
		},
		{
			name:      "select_empty_repeated_nested",
			expr:      `TestAllTypes{}.repeated_nested_message.size() == 0`,
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			container: "google.expr.proto3.test",
			out:       types.True,
		},
		{
			name:      "call_with_error_unary",
			expr:      `try(0/0)`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "try",
					decls.Overload("try_dyn", []*types.Type{types.DynType}, types.DynType,
						decls.OverloadIsNonStrict(),
						decls.UnaryBinding(func(arg ref.Val) ref.Val {
							if types.IsError(arg) {
								return types.String(fmt.Sprintf("error: %s", arg))
							}
							return arg
						}),
					),
				),
			},
			out: types.String("error: division by zero"),
		},
		{
			name:      "call_with_error_binary",
			expr:      `try(0/0, 0)`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "try",
					decls.Overload("try_dyn",
						[]*types.Type{types.DynType, types.DynType},
						types.NewListType(types.DynType),
						decls.OverloadIsNonStrict(),
						decls.BinaryBinding(func(arg0, arg1 ref.Val) ref.Val {
							if types.IsError(arg0) {
								return types.String(fmt.Sprintf("error: %s", arg0))
							}
							return types.NewDynamicList(types.DefaultTypeAdapter, []ref.Val{arg0, arg1})
						}),
					),
				),
			},
			out: types.String("error: division by zero"),
		},
		{
			name:      "call_with_error_function",
			expr:      `try(0/0, 0, 0)`,
			unchecked: true,
			funcs: []*decls.FunctionDecl{
				funcDecl(t, "try",
					decls.Overload("try_dyn",
						[]*types.Type{types.DynType, types.DynType, types.DynType},
						types.NewListType(types.DynType),
						decls.OverloadIsNonStrict(),
						decls.FunctionBinding(func(args ...ref.Val) ref.Val {
							if types.IsError(args[0]) {
								return types.String(fmt.Sprintf("error: %s", args[0]))
							}
							return types.NewDynamicList(types.DefaultTypeAdapter, args)
						}),
					),
				),
			},
			out: types.String("error: division by zero"),
		},
		{
			name: "literal_map_optional_field",
			expr: `{?'hi': {}.?missing,
			        ?'world': {'present': 42u}.?present}`,
			out: map[string]any{
				"world": uint(42),
			},
		},
		{
			name:      "literal_map_optional_field_bad_init",
			expr:      `{?'hi': 'world'}`,
			unchecked: true,
			err:       `cannot initialize optional entry 'hi' from non-optional`,
		},
		{
			name:      "literal_pb_optional_field",
			expr:      `TestAllTypes{?single_int32: {'value': 1}.?value, ?single_string: {}.?missing}`,
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			out: &proto3pb.TestAllTypes{
				SingleInt32: 1,
			},
		},
		{
			name:      "literal_pb_optional_field_bad_init",
			expr:      `TestAllTypes{?single_int32: 1}`,
			container: "google.expr.proto3.test",
			typeOpts:  []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
			unchecked: true,
			err:       `cannot initialize optional entry 'single_int32' from non-optional`,
		},
		{
			name: "literal_list_optional_element",
			expr: `[?{}.?missing, ?{'present': 42u}.?present]`,
			out:  []uint64{42},
		},
		{
			name:      "literal_list_optional_bad_element",
			expr:      `[?123]`,
			unchecked: true,
			err:       `cannot initialize optional list element from non-optional value 123`,
		},
		{
			name: "bad_argument_in_optimized_list",
			expr: `1/0 in [1, 2, 3]`,
			err:  `division by zero`,
		},
		{
			name:      "list_index_error",
			expr:      `mylistundef[0]`,
			unchecked: true,
			err:       `no such attribute(s): mylistundef`,
		},
		{
			name:      "pkg_list_index_error",
			container: "goog",
			expr:      `pkg.mylistundef[0]`,
			unchecked: true,
			err:       `no such attribute(s): goog.pkg.mylistundef, pkg.mylistundef`,
		},
		{
			name: "unknown_attribute",
			expr: `a[0]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a",
					types.NewMapType(types.IntType, types.BoolType)),
			},
			attrs: NewPartialAttributeFactory(testContainer(""), types.DefaultTypeAdapter, types.NewEmptyRegistry()),
			in: newTestPartialActivation(t, map[string]any{
				"a": map[int64]any{
					1: true,
				},
			}, NewAttributePattern("a").QualInt(0)),
			out: types.NewUnknown(2, types.QualifyAttribute[int64](types.NewAttributeTrail("a"), 0)),
		},
		{
			name: "macro_has_map_key_unknown_propagates",
			expr: `has(a.b)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.BoolType)),
			},
			attrs: NewPartialAttributeFactory(testContainer(""), types.DefaultTypeAdapter, types.NewEmptyRegistry()),
			in:    newTestPartialActivation(t, map[string]any{}, NewAttributePattern("a")),
			out:   types.NewUnknown(4, types.NewAttributeTrail("a")),
		},
		{
			name: "unknown_attribute_mixed_qualifier",
			expr: `a[dyn(0u)]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a",
					types.NewMapType(types.IntType, types.BoolType)),
			},
			attrs: NewPartialAttributeFactory(testContainer(""), types.DefaultTypeAdapter, types.NewEmptyRegistry()),
			in: newTestPartialActivation(t, map[string]any{
				"a": map[int64]any{
					1: true,
				},
			}, NewAttributePattern("a").QualInt(0)),
			out: types.NewUnknown(2, types.QualifyAttribute[uint64](types.NewAttributeTrail("a"), 0)),
		},
		{
			name: "invalid_presence_test_on_int_literal",
			expr: `has(dyn(1).invalid)`,
			err:  "no such key: invalid",
			attrs: NewAttributeFactory(testContainer(""), types.DefaultTypeAdapter, types.NewEmptyRegistry(),
				EnableErrorOnBadPresenceTest(true)),
		},
		{
			name: "invalid_presence_test_on_list_literal",
			expr: `has(dyn([]).invalid)`,
			err:  "unsupported index type 'string' in list",
			attrs: NewAttributeFactory(testContainer(""), types.DefaultTypeAdapter, types.NewEmptyRegistry(),
				EnableErrorOnBadPresenceTest(true)),
		},

		{
			name: "optional_select_on_undefined",
			expr: `{}.?invalid`,
			out:  types.OptionalNone,
		},
		{
			name: "optional_select_on_null_literal",
			expr: `{"invalid": dyn(null)}.?invalid.?nested`,
			out:  types.OptionalNone,
		},
		{
			name:      "local_shadow_identifier_in_select",
			expr:      `[{'z': 0}].exists(y, y.z == 0)`,
			container: "cel.example",
			vars: []*decls.VariableDecl{
				decls.NewVariable("cel.example.y", types.IntType),
			},
			in: map[string]any{
				"cel.example.y": map[string]int{"z": 1},
			},
			out: types.True,
		},
		{
			name:      "local_shadow_identifier_in_select_global_disambiguation",
			expr:      `[{'z': 0}].exists(y, y.z == 0 && .y.z == 1)`,
			container: "y",
			vars: []*decls.VariableDecl{
				decls.NewVariable("y.z", types.IntType),
			},
			in: map[string]any{
				"y.z": 1,
			},
			out: types.True,
		},
		{
			name: "local_shadow_identifier_with_global_disambiguation",
			expr: `[0].exists(x, x == 0 && .x == 1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
			},
			in: map[string]any{
				"x": 1,
			},
			out: types.True,
		},
		{
			name: "local_double_shadow_identifier_with_global_disambiguation",
			expr: `[0].exists(x, [x+1].exists(x, x == .x))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
			},
			in: map[string]any{
				"x": 1,
			},
			out: types.True,
		},
		{
			name:      "unchecked_local_shadow_identifier_in_select",
			expr:      `[{'z': 0}].exists(y, y.z == 0)`,
			unchecked: true,
			container: "cel.example",
			vars: []*decls.VariableDecl{
				decls.NewVariable("cel.example.y", types.IntType),
			},
			in: map[string]any{
				"cel.example.y": map[string]int{"z": 1},
			},
			out: types.True,
		},
		{
			name:      "unchecked_local_shadow_identifier_in_select_global_disambiguation",
			expr:      `[{'z': 0}].exists(y, y.z == 0 && .y.z == 1)`,
			container: "y",
			unchecked: true,
			vars: []*decls.VariableDecl{
				decls.NewVariable("y.z", types.IntType),
			},
			in: map[string]any{
				"y.z": 1,
			},
			out: types.True,
		},
		{
			name:      "unchecked_local_shadow_identifier_with_global_disambiguation",
			expr:      `[0].exists(x, x == 0 && .x == 1)`,
			unchecked: true,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
			},
			in: map[string]any{
				"x": 1,
			},
			out: types.True,
		},
		{
			name:      "unchecked_local_double_shadow_identifier_with_global_disambiguation",
			expr:      `[0].exists(x, [x+1].exists(x, x == .x))`,
			unchecked: true,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
			},
			in: map[string]any{
				"x": 1,
			},
			out: types.True,
		},
	}
}

func BenchmarkInterpreter(b *testing.B) {
	for _, tst := range testData(b) {
		if tst.err != "" || tst.progErr != "" {
			continue
		}
		prg, frame, err := program(b, &tst, Optimize(), CompileRegexConstants(MatchesRegexOptimization))
		if err != nil {
			b.Fatal(err)
		}
		// Benchmark the eval.
		b.Run(tst.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				prg.Exec(frame)
			}
		})
	}
}

func BenchmarkInterpreterParallel(b *testing.B) {
	for _, tst := range testData(b) {
		prg, vars, err := program(b, &tst, Optimize(), CompileRegexConstants(MatchesRegexOptimization))
		if tst.err != "" || tst.progErr != "" {
			continue
		}
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.Run(tst.name,
			func(b *testing.B) {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						prg.Eval(vars)
					}
				})
			})
	}
}

func TestInterpreter(t *testing.T) {
	for _, tst := range testData(t) {
		tc := tst
		prg, vars, err := program(t, &tc)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var want ref.Val = types.True
			if tc.out != nil {
				want = tc.out.(ref.Val)
			}
			got := prg.Eval(vars)
			_, expectUnk := want.(*types.Unknown)
			if expectUnk {
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Got %v, wanted %v", got, want)
				}
			} else if tc.err != "" {
				if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), tc.err) {
					t.Fatalf("Got %v (%T), wanted error: %s", got, got, tc.err)
				}
			} else if got.Equal(want) != types.True {
				t.Fatalf("Got %v, wanted %v", got, want)
			}

			state := NewEvalState()
			opts := map[string][]PlannerOption{
				"optimize": {Optimize()},
				"exhaustive": {ExhaustiveEval(),
					EvalStateObserver(EvalStateFactory(func() EvalState { return state }))},
				"track": {EvalStateObserver(EvalStateFactory(func() EvalState { return state }))},
			}
			for mode, opt := range opts {
				opts := opt
				if tc.extraOpts != nil {
					opts = append(opts, tc.extraOpts...)
				}
				prg, vars, err = program(t, &tc, opts...)
				if tc.progErr != "" {
					if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), tc.progErr) {
						t.Errorf("Got %v (%T), wanted error: %s", got, got, tc.progErr)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Run(mode, func(t *testing.T) {
					got := prg.Eval(vars)
					_, expectUnk := want.(*types.Unknown)
					if expectUnk {
						if !reflect.DeepEqual(got, want) {
							t.Errorf("Got %v, wanted %v", got, want)
						}
					} else if tc.err != "" {
						if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), tc.err) {
							t.Errorf("Got %v (%T), wanted error: %s", got, got, tc.err)
						}
						type nodeIDer interface {
							NodeID() int64
						}
						nodeErr, ok := got.(nodeIDer)
						if !ok || nodeErr.NodeID() == 0 {
							t.Errorf("Did not get AST node ID from error: %#v", got)
						}
					} else if got.Equal(want) != types.True {
						t.Errorf("Got %v, wanted %v", got, want)
					}
					state.Reset()
				})
			}
		})
	}
}

func TestInterpreter_ProtoAttributeOpt(t *testing.T) {
	inst, _, err := program(t, &testCase{
		name:     "nested_proto_field_with_index",
		expr:     `pb3.map_int64_nested_type[0].child.payload.single_int32`,
		typeOpts: []types.RegistryOption{types.ProtoTypeDefs(&proto3pb.TestAllTypes{})},
		vars: []*decls.VariableDecl{
			decls.NewVariable("pb3",
				types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
		},
		in: map[string]any{
			"pb3": &proto3pb.TestAllTypes{
				MapInt64NestedType: map[int64]*proto3pb.NestedTestAllTypes{
					0: {
						Child: &proto3pb.NestedTestAllTypes{
							Payload: &proto3pb.TestAllTypes{
								SingleInt32: 1,
							},
						},
					},
				},
			},
		},
	}, Optimize())
	if err != nil {
		t.Fatal(err)
	}
	attr, ok := inst.(InterpretableAttribute)
	if !ok {
		t.Fatalf("got %v, expected attribute value", inst)
	}
	absAttr, ok := attr.Attr().(NamespacedAttribute)
	if !ok {
		t.Fatalf("got %v, expected global attribute", attr.Attr())
	}
	quals := absAttr.Qualifiers()
	if len(quals) != 5 {
		t.Errorf("got %d qualifiers, wanted 5", len(quals))
	}
	if !isFieldQual(quals[0], "map_int64_nested_type") ||
		!isConstQual(quals[1], types.IntZero) ||
		!isFieldQual(quals[2], "child") ||
		!isFieldQual(quals[3], "payload") ||
		!isFieldQual(quals[4], "single_int32") {
		t.Error("unoptimized qualifier types present in optimized attribute")
	}
}

func TestInterpreter_LogicalAndMissingType(t *testing.T) {
	parsed := testMustParse(t, `a && TestProto{c: true}.c`)
	reg := newTestRegistry(t)
	cont := containers.DefaultContainer
	attrs := NewAttributeFactory(cont, reg, reg)
	intr := newStandardInterpreter(t, cont, reg, reg, attrs)
	i, err := intr.NewInterpretable(parsed)
	if err == nil {
		t.Errorf("Got '%v', wanted error", i)
	}
}

func TestInterpreter_ExhaustiveConditionalExpr(t *testing.T) {
	parsed := testMustParse(t, `a ? b < 1.0 : c == ['hello']`)
	state := NewEvalState()
	cont := containers.DefaultContainer
	reg := newTestRegistry(t, types.ProtoTypeDefs(&exprpb.ParsedExpr{}))
	attrs := NewAttributeFactory(cont, reg, reg)
	intr := newStandardInterpreter(t, cont, reg, reg, attrs)
	interpretable, _ := intr.NewInterpretable(parsed, ExhaustiveEval(),
		EvalStateObserver(EvalStateFactory(func() EvalState { return state })))
	vars, _ := NewActivation(map[string]any{
		"a": types.True,
		"b": types.Double(0.999),
		"c": types.NewStringList(reg, []string{"hello"})})
	result := interpretable.Eval(vars)
	// Operator "_==_" is at Expr 7, should be evaluated in exhaustive mode
	// even though "a" is true
	ev, _ := state.Value(7)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if ev != types.True {
		t.Errorf("Else expression expected to be true, got: %v", ev)
	}
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_InterruptableEval(t *testing.T) {
	items := make([]int64, 5000)
	for i := int64(0); i < 5000; i++ {
		items[i] = i
	}
	tc := testCase{
		expr: `items.map(i, i).map(i, i).size() != 0`,
		vars: []*decls.VariableDecl{
			decls.NewVariable("items", types.NewListType(types.IntType)),
		},
		in: map[string]any{
			"items": items,
		},
		out: true,
	}
	prg, frame, err := program(t, &tc, InterruptableEval())
	if err != nil {
		t.Fatalf("program(%s) failed: %v", tc.expr, err)
	}

	ctx := context.TODO()
	evalCtx, cancel := context.WithTimeout(ctx, 10*time.Microsecond)
	defer cancel()

	frame.SetContext(evalCtx, 100)
	out := prg.Exec(frame)
	frame.Close()
	if !types.IsError(out) || out.(*types.Err).String() != "operation interrupted" {
		t.Errorf("Got %v, wanted operation interrupted error", out)
	}
}

func TestInterpreter_ExhaustiveLogicalOrEquals(t *testing.T) {
	// a || b == "b"
	// Operator "==" is at Expr 4, should be evaluated though "a" is true
	parsed := testMustParse(t, `a || b == "b"`)
	state := NewEvalState()
	reg := newTestRegistry(t, types.ProtoTypeDefs(&exprpb.Expr{}))
	cont := testContainer("test")
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := newStandardInterpreter(t, cont, reg, reg, attrs)
	i, _ := interp.NewInterpretable(parsed, ExhaustiveEval(),
		EvalStateObserver(EvalStateFactory(func() EvalState { return state })))
	vars, _ := NewActivation(map[string]any{
		"a": true,
		"b": "b",
	})
	result := i.Eval(vars)
	rhv, _ := state.Value(3)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if rhv != types.True {
		t.Errorf("Right hand side expression expected to be true, got: %v", rhv)
	}
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_SetProto2PrimitiveFields(t *testing.T) {
	// Test the use of proto2 primitives within object construction.
	src := common.NewTextSource(
		`input == TestAllTypes{
			single_int32: 1,
			single_int64: 2,
			single_uint32: 3u,
			single_uint64: 4u,
			single_float: -3.3,
			single_double: -2.2,
			single_string: "hello world",
			single_bool: true
		}`)
	parsed := testMustParse(t, src)
	cont := testContainer("google.expr.proto2.test")
	reg := newTestRegistry(t, types.ProtoTypeDefs(&proto2pb.TestAllTypes{}))
	env := newTestEnv(t, cont, reg)
	env.AddIdents(
		decls.NewVariable("input",
			types.NewObjectType("google.expr.proto2.test.TestAllTypes")))
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Error(errors.ToDisplayString())
	}

	attrs := NewAttributeFactory(cont, reg, reg)
	i := newStandardInterpreter(t, cont, reg, reg, attrs)
	eval, _ := i.NewInterpretable(checked)
	one := int32(1)
	two := int64(2)
	three := uint32(3)
	four := uint64(4)
	five := float32(-3.3)
	six := float64(-2.2)
	str := "hello world"
	truth := true
	input := &proto2pb.TestAllTypes{
		SingleInt32:  &one,
		SingleInt64:  &two,
		SingleUint32: &three,
		SingleUint64: &four,
		SingleFloat:  &five,
		SingleDouble: &six,
		SingleString: &str,
		SingleBool:   &truth,
	}
	vars, _ := NewActivation(map[string]any{
		"input": reg.NativeToValue(input),
	})
	result := eval.Eval(vars)
	got, ok := result.Value().(bool)
	if !ok {
		t.Fatalf("Got '%v', wanted 'true'.", result)
	}
	expected := true
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Could not build object properly. Got '%v', wanted '%v'",
			result.Value(),
			expected)
	}
}

func TestInterpreter_MissingIdentInSelect(t *testing.T) {
	src := common.NewTextSource(`a.b.c`)
	parsed := testMustParse(t, src)
	cont := testContainer("test")
	reg := newTestRegistry(t)
	env := newTestEnv(t, cont, reg)
	env.AddIdents(decls.NewVariable("a.b", types.DynType))
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Fatal(errors.ToDisplayString())
	}

	attrs := NewPartialAttributeFactory(cont, reg, reg)
	interp := newStandardInterpreter(t, cont, reg, reg, attrs)
	i, _ := interp.NewInterpretable(checked)
	vars, _ := NewPartialActivation(
		map[string]any{
			"a.b": map[string]any{
				"d": "hello",
			},
		},
		NewAttributePattern("a.b").QualString("c"))
	result := i.Eval(vars)
	if !types.IsUnknown(result) {
		t.Errorf("Got %v, wanted unknown", result)
	}

	result = i.Eval(EmptyActivation())
	if !types.IsError(result) {
		t.Errorf("Got %v, wanted error", result)
	}
}

func TestInterpreter_TypeConversionOpt(t *testing.T) {
	tests := []struct {
		in  string
		out ref.Val
		err bool
	}{
		{in: `bool('tru')`, err: true},
		{in: `bool("true")`, out: types.True},
		{in: `bytes("hello")`, out: types.Bytes("hello")},
		{in: `double("_123")`, err: true},
		{in: `double("123.0")`, out: types.Double(123.0)},
		{in: `duration('12hh3')`, err: true},
		{in: `duration('12s')`, out: types.Duration{Duration: time.Duration(12) * time.Second}},
		{in: `dyn(1u)`, out: types.Uint(1)},
		{in: `int('11l')`, err: true},
		{in: `int('11')`, out: types.Int(11)},
		{in: `string('11')`, out: types.String("11")},
		{in: `timestamp('123')`, err: true},
		{in: `timestamp(123)`, out: types.Timestamp{Time: time.Unix(123, 0).UTC()}},
		{in: `type(null)`, out: types.NullType},
		{in: `type(timestamp(int('123')))`, out: types.TimestampType},
		{in: `uint(-1)`, err: true},
		{in: `uint(1)`, out: types.Uint(1)},
	}
	for _, tc := range tests {
		src := common.NewTextSource(tc.in)
		parsed := testMustParse(t, src)
		cont := containers.DefaultContainer
		reg := newTestRegistry(t)
		env := newTestEnv(t, cont, reg)
		checked, errors := checker.Check(parsed, src, env)
		if len(errors.GetErrors()) != 0 {
			t.Fatal(errors.ToDisplayString())
		}
		attrs := NewAttributeFactory(cont, reg, reg)
		interp := newStandardInterpreter(t, cont, reg, reg, attrs)
		// Show that program planning will now produce an error.
		i, err := interp.NewInterpretable(checked, Optimize())
		if tc.err && err == nil {
			t.Fatalf("got %v, expected error", i)
		}
		if tc.out != nil {
			if err != nil {
				t.Fatal(err)
			}
			ic, isConst := i.(InterpretableConst)
			if !isConst {
				t.Fatalf("got %v, expected constant", ic)
			}
			if tc.out.Equal(ic.Value()) != types.True {
				t.Errorf("got %v, wanted %v", ic.Value(), tc.out)
			}
		}
		// Show how the error returned during program planning is the same as the runtime
		// error which would be produced normally.
		if tc.err {
			i2, err2 := interp.NewInterpretable(checked)
			if err2 != nil {
				t.Fatalf("got error, wanted interpretable: %v", i2)
			}
			errVal := i2.Eval(EmptyActivation())
			errValStr := errVal.(*types.Err).Error()
			if errValStr != err.Error() {
				t.Errorf("got error %s, wanted error %s", errValStr, err.Error())
			}
		}
	}
}

func TestInterpreter_PlanOptionalElements(t *testing.T) {
	fac := ast.NewExprFactory()
	// [?a] manipulated so the optional index is negative.
	badOptionalA := fac.NewList(1, []ast.Expr{fac.NewIdent(2, "a")}, []int32{-1})
	// [?b] manipulated so the optional index is out of range.
	badOptionalB := fac.NewList(1, []ast.Expr{fac.NewIdent(2, "b")}, []int32{24})
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := newStandardInterpreter(t, cont, reg, reg, attrs)
	_, err := interp.NewInterpretable(ast.NewAST(badOptionalA, nil), Optimize())
	if err == nil {
		t.Fatal("interp.NewInterpretable() should have failed with negative optional index: -1")
	}
	_, err = interp.NewInterpretable(ast.NewAST(badOptionalB, nil), Optimize())
	if err == nil {
		t.Fatal("interp.NewInterpretable() should have failed with out of range optional index: 24")
	}
}

func TestInterpreter_PlanListComprehensionTwoVar(t *testing.T) {
	fac := ast.NewExprFactory()
	listTwoArgTuples := fac.NewComprehensionTwoVar(1,
		fac.NewList(2, []ast.Expr{
			fac.NewLiteral(3, types.Int(2)),
			fac.NewLiteral(4, types.Int(3)),
		}, []int32{}),
		"i",
		"v",
		fac.AccuIdentName(),
		fac.NewList(5, []ast.Expr{}, []int32{}),
		fac.NewLiteral(6, types.True),
		fac.NewCall(7, operators.Add, fac.NewAccuIdent(8),
			fac.NewList(9, []ast.Expr{fac.NewIdent(10, "i"), fac.NewIdent(11, "v")}, []int32{})),
		fac.NewAccuIdent(12),
	)
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := newStandardInterpreter(t, cont, reg, reg, attrs)
	expr, err := interp.NewInterpretable(ast.NewAST(listTwoArgTuples, nil), Optimize())
	if err != nil {
		t.Fatalf("interp.NewInterpretable() failed for two-variable comprehension: %v", err)
	}
	result := expr.Eval(EmptyActivation())
	if types.IsError(result) {
		t.Fatalf("expr.Eval() yielded error: %v", result)
	}
	want := []int64{0, 2, 1, 3}
	out, err := result.ConvertToNative(reflect.TypeOf(want))
	if err != nil {
		t.Fatalf("result.ConvertToNative() failed: %v", err)
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got %v, wanted %v", out, want)
	}
}

func TestInterpreter_PlanMapComprehensionTwoVar(t *testing.T) {
	fac := ast.NewExprFactory()
	listTwoArgTuples := fac.NewComprehensionTwoVar(1,
		fac.NewMap(2, []ast.EntryExpr{
			fac.NewMapEntry(3, fac.NewLiteral(4, types.Int(0)), fac.NewLiteral(5, types.String("first")), false),
			fac.NewMapEntry(6, fac.NewLiteral(7, types.Int(1)), fac.NewLiteral(8, types.String("second")), false),
		}),
		"k",
		"v",
		fac.AccuIdentName(),
		fac.NewMap(9, []ast.EntryExpr{}),
		fac.NewLiteral(10, types.True),
		fac.NewCall(11, "cel.@mapInsert",
			fac.NewAccuIdent(12),
			fac.NewCall(13, operators.Add, fac.NewIdent(14, "k"), fac.NewLiteral(15, types.IntOne)),
			fac.NewIdent(16, "v"),
		),
		fac.NewAccuIdent(17),
	)
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	interp := newStandardInterpreter(t, cont, reg, reg, attrs,
		funcDecl(t, "cel.@mapInsert",
			decls.Overload("cel.@mapInsert",
				[]*types.Type{
					types.NewMapType(types.IntType, types.StringType),
					types.IntType,
					types.StringType,
				}, types.NewMapType(types.IntType, types.StringType)),
			decls.SingletonFunctionBinding(func(args ...ref.Val) ref.Val {
				m := args[0].(traits.Mapper)
				k := args[1]
				v := args[2]
				return types.InsertMapKeyValue(m, k, v)
			}),
		))
	expr, err := interp.NewInterpretable(ast.NewAST(listTwoArgTuples, nil), Optimize())
	if err != nil {
		t.Fatalf("interp.NewInterpretable() failed for two-variable comprehension: %v", err)
	}
	result := expr.Eval(EmptyActivation())
	if types.IsError(result) {
		t.Fatalf("expr.Eval() yielded error: %v", result)
	}
	want := map[int64]string{1: "first", 2: "second"}
	out, err := result.ConvertToNative(reflect.TypeOf(want))
	if err != nil {
		t.Fatalf("result.ConvertToNative() failed: %v", err)
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got %v, wanted %v", out, want)
	}
}

func TestInterpreter_LateBindCalls(t *testing.T) {

	f1 := func(t *testing.T) *decls.FunctionDecl {
		decl, err := decls.NewFunction(
			"f1",
			decls.Overload("f1_int", []*types.Type{}, types.IntType, decls.FunctionBinding(
				func(_ ...ref.Val) ref.Val {
					return types.Int(37)
				},
			)),
			decls.Overload("f1_string_int", []*types.Type{types.StringType}, types.IntType, decls.UnaryBinding(
				func(arg ref.Val) ref.Val {
					return arg.(types.String).Size()
				},
			)),
			decls.Overload("f1_int_int_int", []*types.Type{types.IntType, types.IntType}, types.IntType, decls.BinaryBinding(
				func(lhs ref.Val, rhs ref.Val) ref.Val {
					return lhs.(types.Int).Add(rhs)
				},
			)),
			decls.Overload("f1_bool_bool_bool_int", []*types.Type{types.BoolType, types.BoolType, types.BoolType}, types.IntType, decls.FunctionBinding(
				func(args ...ref.Val) ref.Val {
					count := 0
					for _, arg := range args {
						if arg == types.True {
							count++
						}
					}
					return types.Int(count)
				},
			)),
		)

		if err != nil {
			t.Fatalf("pre-condition failed: could not create function declaration for f1 (cause: %v)", err)
		}
		return decl
	}

	// overrides supplied at runtime with the activation.
	f1_int := function("f1_int", 0, false, func(_ ...ref.Val) ref.Val { return types.Int(51) })

	f1_string_int := unary("f1_string_int", 0, false, func(arg ref.Val) ref.Val {
		size := arg.(types.String).Size().(types.Int)
		return size.Multiply(types.Int(2))
	})
	f1_int_int_int := binary("f1_int_int_int", 0, false, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return lhs.(types.Int).Subtract(rhs)
	})
	f1_bool_bool_bool_int := function("f1_bool_bool_bool_int", 0, false, func(args ...ref.Val) ref.Val {
		count := 0
		for _, arg := range args {
			if arg == types.False {
				count++
			}
		}
		return types.Int(count)
	})

	// activation configures an activation that exposes the given variables
	// and the supplied runtime overrides for function overloads.
	activation := func(vars Activation, ovls ...*functions.Overload) Activation {

		d := &defaultDispatcher{
			overloads: overloadMap{},
		}
		for _, ovl := range ovls {
			d.overloads[ovl.Operator] = ovl
		}
		return &lateBindActivation{
			vars:       vars,
			dispatcher: d,
		}
	}

	// dummyDecorator substitutes the evalZeroArity with the
	// custom type dummyEval for the purpose of demonstrating
	// the handling of unknown implementations of IntepretableCall
	dummyDecorator := func() PlannerOption {

		return CustomDecorator(func(i Interpretable) (Interpretable, error) {
			switch expr := i.(type) {
			case *evalZeroArity:
				return &dummyEval{
					id:       expr.id,
					function: expr.function,
					overload: expr.overload,
					impl:     expr.impl,
				}, nil
			default:
				return i, nil
			}
		})
	}

	testCases := []testCase{
		// Test Group 01 - Single Function Call Expressions
		// ------------------------------------------------
		// This is to verify that the very simple case works
		// when we don't supply any function overload. In this
		// case the presence of the decorator should not alter
		// the execution.
		{
			name:      "T01.01__OK_ZeroArity_No_Overrides",
			expr:      "f1()",
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        EmptyActivation(),
			out:       types.Int(37),
		},
		{
			name:      "T01.02__OK_Unary_No_Overrides",
			expr:      `f1("hello")`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        EmptyActivation(),
			out:       types.Int(5),
		},
		{
			name:      "T01.03__OK_Binary_No_Overrides",
			expr:      `f1(3,4)`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        EmptyActivation(),
			out:       types.Int(7),
		},
		{
			name:      "T01.04__OK_VarArgs_No_Overrides",
			expr:      `f1(true, false, true)`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        EmptyActivation(),
			out:       types.Int(2),
		},
		// Test Group 02 - Single Function Call Expression Overrides
		// ---------------------------------------------------------
		// This case is to ensure that the decorator injects and
		// configures correctly the lateBindEval to replace the
		// call with the one supplied via the activation.
		{
			name:      "T02.01__OK_ZeroArity_With_Overrides",
			expr:      "f1()",
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int),
			out:       types.Int(51),
		},
		{
			name:      "T02.02__OK_Unary_With_Overrides",
			expr:      `f1("hello")`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_string_int),
			out:       types.Int(10),
		},
		{
			name:      "T02.03__OK_Binary_With_Overrides",
			expr:      `f1(3,4)`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int_int_int),
			out:       types.Int(-1),
		},
		{
			name:      "T02.04__OK_VarArgs_With_Overrides",
			expr:      `f1(true, false, true)`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_bool_bool_bool_int),
			out:       types.Int(1),
		},
		// TestGroup 03 - Expressions with Operators
		// -----------------------------------------
		// We expect the expressions of the operators
		// to be processed by the planner and decorated
		// accordingly.
		{
			name: "T03.01__OK_Equal_With_Overrides",
			// without overrides:
			// - f1() -> 37
			// - f1("hello") -> 5
			// result: 37 == 5 + 32 = true
			// with overrides: false (51 != 10 + 32)
			expr:      `f1() == f1("hello") + 32`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int, f1_string_int),
			out:       types.False,
		},
		{
			name: "T03.02__OK_Not_Equal_With_Overrides",
			// without overrides:
			// - f1(3,4) -> 7
			// - f1(true, true, true) -> 3
			// - f1() -> 37
			// result: 7 - 3 + 37 == 41 = false
			// with overrides: true (-1 - 0 + 51 != 41)
			expr:      `f1(3,4) - f1(true, true, true) + f1() != 41`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int, f1_int_int_int, f1_bool_bool_bool_int),
			out:       types.True,
		},
		{
			name: "T03.03__OK_And_Or_With_Overrides",
			// without overrides:
			// - f1(3,4) -> 7
			// - f1(true, true, true) -> 3
			// - f1("hello") -> 5
			// result: (7 > 0) && (3 == 3 || 5 == 10): true
			// with overrides: false (-1 < 0) && (0 == 3 || 10 == 10)
			expr:      `f1(3,4) > 0 && (f1(true, true, true) == 3 || f1("hello") == 10)`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_string_int, f1_int_int_int, f1_bool_bool_bool_int),
			out:       types.False,
		},
		{
			name: "T03.04__OK_Ternary_Operator",
			// without overrides:
			// - f1(3,4) -> 7
			// - f1() -> 51
			// - f1("hello") -> 5
			// result: (7 > 0 ? 51 : 5) = 51
			// with overrides: 10 (-1 > 0 ? 51 : 10)
			expr:      `f1(3,4) > 0 ? f1() : f1("hello")`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int, f1_string_int, f1_int_int_int),
			out:       types.Int(10),
		},
		{
			name: "T03.05__OK_List_Index_Operator",
			// without overrides:
			// - f1(true,true,true) -> 3
			// result: <array>[3] = 6
			// with overrides: 1 (<array>[0])
			expr:      `[1, 3, 5, 6][f1(true,true,true)]`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_bool_bool_bool_int),
			out:       types.Int(1),
		},
		{
			name: "T03.06__OK_Map_Index_Operator",
			// without overrides:
			// - f1() -> 37
			// result: { 31: 1, 51: 2 }[37] = 1
			// with overrides: 2 (<map>[51])
			expr:      `{ 37: 1, 51: 2 }[f1()]`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int),
			out:       types.Int(2),
		},
		// Test Group 04 - Object Construction
		// -----------------------------------
		// We expect the expressions to initialise
		// the object to be processed by the planner
		// and then decorated accordingly.
		{
			name: "T04.01__OK_Map_Construction_Values",
			// without overrides:
			// - f1() -> 37
			// - f1("hello") -> 5
			// - f1(2,3) -> 5
			// result: { "a": 37, "b": 5, "c": 5 }
			// with overrides: { "a": 51, "b": 10, "c": -1 }
			expr:      `{ "a": f1(), "b": f1("hello"), "c": f1(2,3) }`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int, f1_string_int, f1_int_int_int),
			out: types.NewDynamicMap(types.DefaultTypeAdapter, map[string]ref.Val{
				"a": types.Int(51), "b": types.Int(10), "c": types.Int(-1),
			}),
		},
		{
			name: "T04.02__OK_Map_Construction_Keys",
			// without overrides:
			// - f1() -> 37
			// - f1("hello") -> 5
			// - f1(2,3) -> 5
			// result: { 37: 10, 5: 3, 5: 6 }
			// with overrides: { 51: 10, 10: 3, -1: 6 }
			expr:      `{ f1(): 10, f1("hello"): 3, f1(2,3): 6 }`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int, f1_string_int, f1_int_int_int),
			out: types.NewDynamicMap(types.DefaultTypeAdapter, map[ref.Val]ref.Val{
				types.Int(51): types.Int(10),
				types.Int(10): types.Int(3),
				types.Int(-1): types.Int(6),
			}),
		},
		{
			name: "T04.03__OK_List_Construction",
			// without overrides:
			// - f1() -> 37
			// - f1("hi") -> 2
			// - f1(2,3) -> 5
			// result: [37, 2, 5]
			// with overrides: [51, 4, 3]
			expr:      `[ f1(), f1("hi"), f1(false, false, false) ]`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int, f1_string_int, f1_bool_bool_bool_int),
			out: types.NewDynamicList(types.DefaultTypeAdapter, []ref.Val{
				types.Int(51), types.Int(4), types.Int(3),
			}),
		},
		{
			name: "T04.04__OK_Object_Construction",
			// without overrides:
			// - f1() -> 37
			// result: { single_int64: 37 }
			// with overrides: { single_int64: 51 }
			expr:      `test.TestAllTypes{single_int64: f1()}`,
			container: "google.expr.proto3",
			types:     []proto.Message{&proto3pb.TestAllTypes{}},
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			in:        activation(EmptyActivation(), f1_int),
			out: &proto3pb.TestAllTypes{
				SingleInt64: 51,
			},
		},
		// Test Group 05 - Macros (Comprehensions)
		// ---------------------------------------
		// The expectation is that the planner processes
		// all the expressions and invokes the decorator
		// accordingly.
		{
			name: "T05.01__OK_Map_Filter",
			// without overrides:
			// - f1() -> 37
			// result: [39, 52]
			// with overrides: [52]
			expr:  `m.filter(k, k > f1())`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.IntType, types.StringType)),
			},
			in: activation(&mapActivation{
				bindings: map[string]any{
					"m": map[int]string{
						39: "hello",
						52: "hi",
					},
				}},
				f1_int,
			),
			out: types.NewDynamicList(types.DefaultTypeAdapter, []ref.Val{
				types.Int(52),
			}),
		},
		{
			name: "T05.02__OK_Map_All",
			// without overrides:
			// - f1() -> 37
			// result: [39 > 37, 52 > 37] = true
			// with overrides: [39 < 51, 52 > 51] = false
			expr:      `m.all(k, k > f1())`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.IntType, types.StringType)),
			},
			in: activation(&mapActivation{
				bindings: map[string]any{
					"m": map[int]string{
						39: "hello",
						52: "hi",
					},
				}},
				f1_int,
			),
			out: types.False,
		},
		{
			name: "T05.03__OK_Map_ExistOne",
			// without overrides:
			// - f1() -> 37
			// result: [39 > 37, 45 > 37, 52 > 37] = false
			// with overrides: [39 < 51, 45 < 51, 52 > 51] = true
			expr:      `m.exists_one(k, k > f1())`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.IntType, types.StringType)),
			},
			in: activation(&mapActivation{
				bindings: map[string]any{
					"m": map[int]string{
						39: "hello",
						45: "hey",
						52: "hi",
					},
				}},
				f1_int,
			),
			out: types.True,
		},
		{
			name: "T05.04__OK_Map_Exists",
			// without overrides:
			// - f1() -> 37
			// result: [39 > 37, 45 > 37, 52 > 37] = true
			// with overrides: [39 < 51, 45 < 51, 10 < 51] = false
			expr:      `m.exists(k, k > f1())`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.IntType, types.StringType)),
			},
			in: activation(&mapActivation{
				bindings: map[string]any{
					"m": map[int]string{
						39: "hello",
						45: "hey",
						10: "hi",
					},
				}},
				f1_int,
			),
			out: types.False,
		},
		{
			name: "T05.05__OK_Map_Map",
			// without overrides:
			// - f1(5,3) -> 8
			// - f1(true,true,false) -> 2
			// result: [1 < 8, 2 < 8, 7 < 8] = [1 * 2, 2 * 2, 7 * 2] = [2, 4, 14]
			// with overrides:
			// - f1(5,3) -> 2
			// - f1(true,true,false) -> 1
			// result: [1]
			//
			// NOTE: ensure the expected result is only one, so we don't get flakiness
			// due to variable key ordering of the map iterator.
			expr:      `m.map(k, k < f1(5,3), k * f1(true, true, false))`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			vars: []*decls.VariableDecl{
				decls.NewVariable("m", types.NewMapType(types.IntType, types.StringType)),
			},
			in: activation(&mapActivation{
				bindings: map[string]any{
					"m": map[int]string{
						1: "hello",
						2: "hey",
						7: "hi",
					},
				}},
				f1_int_int_int,
				f1_bool_bool_bool_int,
			),
			out: types.NewDynamicList(types.DefaultTypeAdapter, []int{1}),
		},
		{
			name: "T05.06__OK_List_Filter",
			// without overrides:
			// - f1(6,3) -> 9
			// result: [12, 20]
			// with overrides: [12, 4, 20]
			expr:  `l.filter(e, e > f1(6,3))`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("l", types.NewListType(types.IntType)),
			},
			unchecked: false,
			in: activation(&mapActivation{
				bindings: map[string]any{
					"l": []int{2, 12, 4, 20},
				}},
				f1_int_int_int,
			),
			out: types.NewDynamicList(types.DefaultTypeAdapter, []int{12, 4, 20}),
		},
		{
			name: "T05.07__OK_List_All",
			// without overrides:
			// - f1(6,3) -> 9
			// result: false
			// with overrides: true
			expr:  `l.all(e, e > f1(6,3))`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("l", types.NewListType(types.IntType)),
			},
			unchecked: false,
			in: activation(&mapActivation{
				bindings: map[string]any{
					"l": []int{5, 6, 4, 7},
				}},
				f1_int_int_int,
			),
			out: types.True,
		},
		{
			name: "T05.08__OK_List_Exists",
			// without overrides:
			// - f1() -> 37
			// result: true
			// with overrides: false
			expr:  `l.exists(e, e == f1())`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("l", types.NewListType(types.IntType)),
			},
			unchecked: false,
			in: activation(&mapActivation{
				bindings: map[string]any{
					"l": []int{37, 6, 4, 7},
				}},
				f1_int,
			),
			out: types.False,
		},
		{
			name: "T05.09__OK_List_Exists_One",
			// without overrides:
			// - f1("hello") -> 5
			// result: 4 < 5 (only one): true
			// with overrides: 6,4,7 < 10: false
			expr:  `l.exists_one(e, e < f1("hello"))`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("l", types.NewListType(types.IntType)),
			},
			unchecked: false,
			in: activation(&mapActivation{
				bindings: map[string]any{
					"l": []int{37, 6, 4, 7},
				}},
				f1_string_int,
			),
			out: types.False,
		},
		{
			name: "T05.10__OK_List_Map",
			// without overrides:
			// - f1("hi") -> 2
			// result: [5 * len("hello"), 5 * len("hey"), 5 * len("howdy")] = [25, 15, 25]
			// with overrides: [5 * 2 * len("hello"), 5 * 2 * len("howdy")] = [50, 50]
			expr:  `l.map(e, size(e) > f1("hi"),  5 * f1(e))`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("l", types.NewListType(types.StringType)),
			},
			unchecked: false,
			in: activation(&mapActivation{
				bindings: map[string]any{
					"l": []string{"hello", "hi", "hey", "howdy"},
				}},
				f1_string_int,
			),
			out: types.NewDynamicList(types.DefaultTypeAdapter, []int{50, 50}),
		},
		// Test Group 06 - Complex Nesting
		// -------------------------------
		// These are just sanity checks to ensure that when we
		// have complex expressions we don't have forgotten
		// something.
		{
			name: "T06.01__OK_Nested_Function_Calls",
			// without overrides:
			// - f1() -> 37
			// - f1(bool, bool, bool) -> nr of trues
			// - f1("hi") -> 2, f1("hello") -> 5
			// - f1(n,m) -> n + m
			// result: 37 + f1(true, true, false) = 39
			// with overrides:
			// - f1() -> 51
			// - f1(bool, bool, bool) -> nr of falses
			// - f1("hi") -> 4, f1("hello") -> 10
			// - f1(n,m) -> n - m
			// result: 51 + f1(false, false, true) = 53
			expr: `f1() + f1(
				l.all(
					e,
					e < f1(f1("hello"), f1("hi"))
				), 
				m.exists(
					k,
					k == f1()
				), 
				c > f1(a,b)
			)`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("l", types.NewListType(types.IntType)),
				decls.NewVariable("m", types.NewMapType(types.IntType, types.StringType)),
				decls.NewVariable("c", types.IntType),
				decls.NewVariable("a", types.IntType),
				decls.NewVariable("b", types.IntType),
			},
			unchecked: false,
			in: activation(&mapActivation{
				bindings: map[string]any{
					// true for l.all with no overrides, false with overrides
					"l": []int{3, 4, 5, 6},
					// true for m.exists with no overrides, false with overrides
					"m": map[int]string{
						37: "x",
						45: "y",
						81: "z",
					},
					// false for c > f1(a,b) with no overrides
					"c": 3,
					"a": 5,
					"b": 4,
				}},
				f1_int,
				f1_string_int,
				f1_int_int_int,
				f1_bool_bool_bool_int,
			),
			out: types.Int(53),
		},
		{
			name: "T06.02__OK_Nested_Runtime_Overrides",
			// without overrides:
			// - f1() -> 37
			// - f1(a) -> 5
			// - f1(b,c) -> 7
			// - f1(d,e,f) -> 2
			// result: 37 + 5 + 7 + 2 = 51
			// with overrides:
			// - f1() -> 51
			// - f1(a) -> 10
			// - f1(b,c) -> -1
			// - f1(d,e,f) -> 1
			// result: 51 + 10 - 1 + 1 = 61
			expr:  `f1() + f1(a) + f1(b,c) + f1(d,e,f)`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.StringType),
				decls.NewVariable("b", types.IntType),
				decls.NewVariable("c", types.IntType),
				decls.NewVariable("d", types.BoolType),
				decls.NewVariable("e", types.BoolType),
				decls.NewVariable("f", types.BoolType),
			},
			unchecked: false,
			in: &hierarchicalActivation{
				parent: activation(
					&mapActivation{
						bindings: map[string]any{
							"a": "howdy",
							"b": 3,
						},
					},
					f1_int,
				),
				child: &hierarchicalActivation{
					parent: activation(
						&mapActivation{
							bindings: map[string]any{
								"c": 4,
								"d": true,
							},
						},
						f1_string_int,
					),
					child: &lateBindActivation{
						dispatcher: &defaultDispatcher{
							parent: &defaultDispatcher{
								overloads: overloadMap{
									"f1_int_int_int": f1_int_int_int,
								},
							},
							overloads: overloadMap{
								"f1_bool_bool_bool_int": f1_bool_bool_bool_int,
							},
						},
						vars: &mapActivation{
							bindings: map[string]any{
								"e": false,
								"f": true,
							},
						},
					},
				},
			},
			out: types.Int(61),
		},

		// Test Group 07 - With Eval Observer and Others
		// ---------------------------------------------------
		// These test cases are important to ensure that when
		// interpretables are wrapped by evalWatchXXX the late
		// bind decorator, if added later, can still travel
		// through the wrapped Inteprepretable.
		{
			name: "T07.01__OK_With_EvalObserver",
			// without overrides:
			// - f1() -> 37
			// - f2(2,b) -> 2 + b
			// result: a + 37 - 2 + b = 28
			// with overrides:
			// - f1() -> 51
			// - f2(2,b) -> 2 - b
			// result: a + 51 - 2 - b = 66
			expr:  `a + f1() - f1(2,b)`,
			funcs: []*decls.FunctionDecl{f1(t)},
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.IntType),
				decls.NewVariable("b", types.IntType),
			},
			extraOpts: []PlannerOption{
				EvalStateObserver(),
				LateBindCalls(),
			},
			in: activation(
				&mapActivation{
					bindings: map[string]any{
						"a": 5,
						"b": 12,
					},
				},
				f1_int,
				f1_int_int_int,
			),
			out: types.Int(66),
		},
		{
			name:  "T07.02__OK_With_Optimize",
			expr:  `f1() in [ 23, 34, 51 ]`,
			funcs: []*decls.FunctionDecl{f1(t)},
			extraOpts: []PlannerOption{
				Optimize(),
				LateBindCalls(),
			},
			unchecked: false,
			in:        activation(&emptyActivation{}, f1_int),
			out:       types.True,
		},

		// Test Group 08 - LateBindCalls Variations
		// ------------------------------------------
		// These test cases are aimed at checking that when
		// we play around with LateBindCalls the outcome is
		// still predictable and expected:
		// - if we add two LateBindCalls options, only the
		//   first one will have effect.
		// - if we add a custom injector, this will be honored.
		{
			name: "T08.01__OK_With_Custom_Injector_And_Custom_Eval",
			// without overrides:
			// - f1() -> 37
			// with overrides:
			// - f1() -> 51
			expr:  `f1()`,
			funcs: []*decls.FunctionDecl{f1(t)},
			extraOpts: []PlannerOption{

				// injects custom type.
				dummyDecorator(),
				// lateBind will not process this node
				// and throw an error.
				LateBindCalls(Injector(
					&dummyEval{},
					func(i InterpretableCall, ovl *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {

						de := i.(*dummyEval)

						return &dummyEval{
							id:       de.id,
							function: de.function,
							overload: de.overload,
							// we should check that the function is not nil
							// but we only do this for the purpose of test.
							impl: ovl.Function,
						}, nil
					}),
				),
			},
			unchecked: false,
			in:        activation(&emptyActivation{}, f1_int),
			out:       types.Int(51),
		},
		{
			name: "T08.02__ERROR_With_Custom_Eval",
			// without overrides:
			// - f1() -> 37
			// with overrides:
			// - f1() -> 51 (error)
			// NOTE: since f1() is mutated from evalZeroArity to evalDummy
			// we will receive an error, because there is no custom injector
			// able to handle this type.
			expr:  `f1()`,
			funcs: []*decls.FunctionDecl{f1(t)},
			extraOpts: []PlannerOption{

				// injects custom type.
				dummyDecorator(),
				// lateBind will not process this node
				// and throw an error.
				LateBindCalls(),
			},
			in:      activation(&emptyActivation{}, f1_int),
			progErr: fmt.Sprintf(errorUnknownCallNode, 1, &dummyEval{}),
		},
		{
			name: "T08.03__OK_With_Multiple_LateBind_Calls",
			// without overrides:
			// - f1() -> 37
			// - f1("hi") -> 2
			// - f1(4,2) -> 6
			// - f1(true,true,true) -> 3
			// result: 37 + 2 + 6 + 3 = 48
			// with overrides:
			// - f1() -> 51
			// - f1("hi") -> 4
			// - f1(4,2) -> 2
			// - f1(true,true,true) -> 0
			// result: 51 + 4 + 2 + 0 = 57
			expr:      `f1() + f1("hi") + f1(4,2) + f1(true,true,true)`,
			funcs:     []*decls.FunctionDecl{f1(t)},
			unchecked: false,
			extraOpts: []PlannerOption{
				LateBindCalls(),
				LateBindCalls(), // this will not take effect.
			},
			in: activation(
				&emptyActivation{},
				f1_int,
				f1_string_int,
				f1_int_int_int,
				f1_bool_bool_bool_int,
			),
			out: types.Int(57),
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {

			// this is to control the use case scenarios where we
			// need to explicitly add and configure LateBindCalls
			if len(tc.extraOpts) == 0 {

				// if it is empty, we add the default behaviour
				// otherwise we expect the test case to explicitly
				// configure the decorator.
				tc.extraOpts = append(tc.extraOpts, LateBindCalls())
			}

			interpretable, activation, err := program(t, &tc, tc.extraOpts...)

			if err != nil {

				if len(tc.progErr) > 0 {

					if !strings.Contains(err.Error(), tc.progErr) {
						t.Fatalf("got %v, (%T), wanted program error with: %s", err.Error(), err, tc.progErr)
					}
					// if we have a program error, we cannot continue.
					return

				} else {

					t.Fatalf("pre-condition failed: could not create program (cause: %v)", err)
				}
			}
			got := interpretable.Eval(activation)
			if len(tc.err) > 0 {
				// we expect error
				if !types.IsError(got) || !strings.Contains(got.(*types.Err).String(), tc.err) {
					t.Fatalf("got %v (%T), wanted error: %s", got, got, tc.err)
				}
			} else {
				want := tc.out.(ref.Val)
				if got.Equal(want) != types.True {
					t.Fatalf("got %v, wanted: %v", got, want)
				}
			}
		})
	}
}

// dummyEval is a test struct used to demonstrate
// the behaviour of the OverloadInjector or its
// absence during late binding.
type dummyEval struct {
	id       int64
	function string
	overload string
	impl     func(...ref.Val) ref.Val
}

func (de *dummyEval) ID() int64                   { return de.id }
func (de *dummyEval) Eval(ctx Activation) ref.Val { return types.LabelErrNode(de.id, de.impl()) }
func (de *dummyEval) Function() string            { return de.function }
func (de *dummyEval) OverloadID() string          { return de.overload }
func (de *dummyEval) Args() []Interpretable       { return []Interpretable{} }

func testContainer(name string) *containers.Container {
	cont, _ := containers.NewContainer(containers.Name(name))
	return cont
}

func program(t testing.TB, tst *testCase, opts ...PlannerOption) (InterpretableV2, *ExecutionFrame, error) {
	// Configure the package.
	cont := containers.DefaultContainer
	if tst.container != "" {
		cont = testContainer(tst.container)
	}
	var err error
	if tst.abbrevs != nil {
		cont, err = containers.NewContainer(
			containers.Name(cont.Name()),
			containers.Abbrevs(tst.abbrevs...))
		if err != nil {
			return nil, nil, err
		}
	}
	var reg *types.Registry
	var env *checker.Env
	reg = newTestRegistry(t)
	if tst.typeOpts != nil {
		reg = newTestRegistry(t, tst.typeOpts...)
	}
	env = newTestEnv(t, cont, reg)
	attrs := NewAttributeFactory(cont, reg, reg)
	if tst.attrs != nil {
		attrs = tst.attrs
	}
	if tst.vars != nil {
		err = env.AddIdents(tst.vars...)
		if err != nil {
			return nil, nil, fmt.Errorf("env.Add(%v) failed: %v", tst.vars, err)
		}
	}
	// Configure the program input.
	vars := EmptyActivation()
	if tst.in != nil {
		vars, err = NewActivation(tst.in)
		if err != nil {
			t.Fatalf("NewActivation(%v) failed: %v", tst.in, err)
		}
	}
	// Adapt the test output, if needed.
	if tst.out != nil {
		tst.out = reg.NativeToValue(tst.out)
	}

	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	if tst.funcs != nil {
		err = env.AddFunctions(tst.funcs...)
		if err != nil {
			return nil, nil, fmt.Errorf("env.Add(%v) failed: %v", tst.funcs, err)
		}
		disp.Add(funcBindings(t, tst.funcs...)...)
	}
	interp := NewInterpreter(disp, cont, reg, reg, attrs)

	// Parse the expression.
	s := common.NewTextSource(tst.expr)
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
		parser.EnableOptionalSyntax(true),
		parser.EnableVariadicOperatorASTs(true),
	)
	if err != nil {
		return nil, nil, err
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		return nil, nil, errors.New(errs.ToDisplayString())
	}
	if tst.unchecked {
		// Build the program plan.
		prg, err := interp.NewInterpretable(parsed, opts...)
		if err != nil {
			return nil, nil, err
		}
		return prg, AsFrame(vars), nil
	}
	// Check the expression.
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		return nil, nil, errors.New(errs.ToDisplayString())
	}
	// Build the program plan.
	prg, err := interp.NewInterpretable(checked, opts...)
	if err != nil {
		return nil, nil, err
	}
	return prg, AsFrame(vars), nil
}

func base64Encode(val ref.Val) ref.Val {
	str, ok := val.(types.String)
	if !ok {
		return types.MaybeNoSuchOverloadErr(val)
	}
	return types.String(base64.StdEncoding.EncodeToString([]byte(str)))
}

func isConstQual(q Qualifier, val ref.Val) bool {
	c, ok := q.(ConstantQualifier)
	if !ok {
		return false
	}
	return c.Value().Equal(val) == types.True
}

func isFieldQual(q Qualifier, fieldName string) bool {
	f, ok := q.(*fieldQualifier)
	if !ok {
		return false
	}
	return f.Name == fieldName
}

func testMustParse(t testing.TB, data any) *ast.AST {
	t.Helper()
	p, err := parser.NewParser()
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	var src common.Source
	switch d := data.(type) {
	case string:
		src = common.NewTextSource(d)
	case common.Source:
		src = d
	default:
		t.Fatalf("testMustParse() got invalid parse data: %v", data)
	}
	parsed, errors := p.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Fatalf("Parse(%q) failed: %v", src.Content(), errors.ToDisplayString())
	}
	return parsed
}

func newTestEnv(t testing.TB, cont *containers.Container, reg *types.Registry) *checker.Env {
	t.Helper()
	env, err := checker.NewEnv(cont, reg, checker.CrossTypeNumericComparisons(true))
	if err != nil {
		t.Fatalf("checker.NewEnv(%v, %v) failed: %v", cont, reg, err)
	}
	err = env.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Fatalf("env.Add(stdlib.Functions()...) failed: %v", err)
	}
	return env
}

func newTestRegistry(t testing.TB, opts ...types.RegistryOption) *types.Registry {
	t.Helper()
	reg, err := types.NewProtoRegistry(opts...)
	if err != nil {
		t.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	return reg
}

func newTestPartialActivation(t testing.TB, in any, unknowns ...*AttributePattern) any {
	t.Helper()
	vars, err := NewPartialActivation(in, unknowns...)
	if err != nil {
		t.Fatalf("NewPartialActivation(%v) failed: %v", in, err)
	}
	return vars
}

// newStandardInterpreter builds a Dispatcher and TypeProvider with support for all of the CEL
// builtins defined in the language definition.
func newStandardInterpreter(t *testing.T,
	container *containers.Container,
	provider types.Provider,
	adapter types.Adapter,
	resolver AttributeFactory,
	optFuncs ...*decls.FunctionDecl) Interpreter {
	t.Helper()
	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	for _, fn := range optFuncs {
		bindings, err := fn.Bindings()
		if err != nil {
			t.Fatalf("fn.Bindings() failed for function %v. error: %v", fn.Name(), err)
		}
		err = disp.Add(bindings...)
		if err != nil {
			t.Fatalf("dispatcher.Add() failed: %v", err)
		}
	}
	return NewInterpreter(disp, container, provider, adapter, resolver)
}

func addFunctionBindings(t testing.TB, dispatcher Dispatcher) {
	funcs := stdlib.Functions()
	for _, fn := range funcs {
		bindings, err := fn.Bindings()
		if err != nil {
			t.Fatalf("fn.Bindings() failed for function %v. error: %v", fn.Name(), err)
		}
		err = dispatcher.Add(bindings...)
		if err != nil {
			t.Fatalf("dispatcher.Add() failed: %v", err)
		}
	}
}

func funcDecl(t testing.TB, name string, opts ...decls.FunctionOpt) *decls.FunctionDecl {
	t.Helper()
	fn, err := decls.NewFunction(name, opts...)
	if err != nil {
		t.Fatalf("NewFunction(%v) failed: %v", name, err)
	}
	return fn
}

func funcBindings(t testing.TB, funcs ...*decls.FunctionDecl) []*functions.Overload {
	t.Helper()
	bindings := []*functions.Overload{}
	for _, fn := range funcs {
		overloads, err := fn.Bindings()
		if err != nil {
			t.Fatalf("fn.Bindings() failed: %v", err)
		}
		bindings = append(bindings, overloads...)
	}
	return bindings
}

type testActivationWrapper struct {
	Activation
	name string
}

func (tw *testActivationWrapper) Unwrap() Activation {
	return tw.Activation
}

func TestInterruptErrorIs(t *testing.T) {
	ie := InterruptError{}
	tests := []struct {
		name   string
		target error
		want   bool
	}{
		{
			name:   "same type",
			target: InterruptError{},
			want:   true,
		},
		{
			name:   "different error",
			target: fmt.Errorf("other error"),
			want:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ie.Is(tc.target); got != tc.want {
				t.Errorf("Is(%v) = %t, wanted %t", tc.target, got, tc.want)
			}
		})
	}
}

func TestFolderActivation(t *testing.T) {
	parentAct := EmptyActivation()
	frame := mustNewExecutionFrame(t, parentAct)
	defer frame.Close()
	fld := newFolder(&evalFold{}, frame)
	defer releaseFolder(fld)
	if fld.Parent() != frame {
		t.Errorf("fld.Parent() = %v, wanted %v", fld.Parent(), frame)
	}
	if fld.Unwrap() != frame {
		t.Errorf("fld.Unwrap() = %v, wanted %v", fld.Unwrap(), frame)
	}
}

func TestEvalWatchConstructor(t *testing.T) {
	watchConst := &evalWatchConstructor{
		constructor: &evalList{
			id:    1,
			elems: []InterpretableV2{NewConstValue(2, types.IntOne)},
		},
	}
	if len(watchConst.InitVals()) != 1 {
		t.Errorf("watchConst.InitVals() len = %d, wanted 1", len(watchConst.InitVals()))
	}
	if watchConst.Type() != types.ListType {
		t.Errorf("watchConst.Type() = %v, wanted %v", watchConst.Type(), types.ListType)
	}
}

func TestCustomDecorator(t *testing.T) {
	decV1 := func(i Interpretable) (Interpretable, error) {
		return i, nil
	}
	opt := CustomDecorator(decV1)
	p := &planner{}
	p2, err := opt(p)
	if err != nil {
		t.Fatalf("CustomDecorator failed: %v", err)
	}
	if len(p2.decorators) != 1 {
		t.Fatalf("CustomDecorator did not add decorator")
	}
	res, err := p2.decorators[0](NewConstValue(1, types.IntOne))
	if err != nil {
		t.Fatalf("wrapped decorator failed: %v", err)
	}
	if res.ID() != 1 {
		t.Errorf("wrapped decorator returned node with ID %d, wanted 1", res.ID())
	}
}

func TestCostTrackerActualCost(t *testing.T) {
	ct := &CostTracker{cost: 42}
	if ct.ActualCost() != 42 {
		t.Errorf("ct.ActualCost() = %d, wanted 42", ct.ActualCost())
	}
}

func TestV2Adapter(t *testing.T) {
	legacy := &testLegacyInterpretable{id: 42}
	adapted := adaptToV2(legacy)
	if adapted.ID() != 42 {
		t.Errorf("adapted.ID() = %d, wanted 42", adapted.ID())
	}
	frame := mustNewExecutionFrame(t, EmptyActivation())
	defer frame.Close()
	val := adapted.Exec(frame)
	if val.Equal(types.IntOne) != types.True {
		t.Errorf("adapted.Exec() = %v, wanted 1", val)
	}
}

func TestInterpretableArgs(t *testing.T) {
	tests := []struct {
		name string
		call InterpretableCall
		want int
	}{
		{
			name: "evalNe",
			call: &evalNe{lhs: NewConstValue(1, types.IntOne), rhs: NewConstValue(2, types.IntOne)},
			want: 2,
		},
		{
			name: "evalZeroArity",
			call: &evalZeroArity{},
			want: 0,
		},
		{
			name: "evalVarArgs",
			call: &evalVarArgs{args: []InterpretableV2{NewConstValue(1, types.IntOne)}},
			want: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(tc.call.Args()); got != tc.want {
				t.Errorf("Args() len = %d, wanted %d", got, tc.want)
			}
		})
	}
}

func TestNewCall(t *testing.T) {
	call := NewCall(10, "f", "f_overload", []InterpretableV2{NewConstValue(1, types.IntOne)}, func(args ...ref.Val) ref.Val { return types.True })
	if call.ID() != 10 {
		t.Errorf("NewCall.ID() = %d, wanted 10", call.ID())
	}
}

func TestExhaustiveOperatorsLegacyEval(t *testing.T) {
	reg, _ := types.NewRegistry()
	fac := NewAttributeFactory(containers.DefaultContainer, reg, reg)

	tests := []struct {
		name string
		expr Interpretable
	}{
		{
			name: "exhaustive or",
			expr: &evalExhaustiveOr{id: 1},
		},
		{
			name: "exhaustive and",
			expr: &evalExhaustiveAnd{id: 2},
		},
		{
			name: "exhaustive conditional",
			expr: &evalExhaustiveConditional{
				id: 3,
				attr: &conditionalAttribute{
					expr:   NewConstValue(4, types.True),
					truthy: fac.AbsoluteAttribute(5, "a"),
					falsy:  fac.AbsoluteAttribute(6, "b"),
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expr.Eval(EmptyActivation())
		})
	}
}

func TestFindFrame(t *testing.T) {
	frame := mustNewExecutionFrame(t, EmptyActivation())
	defer frame.Close()

	tests := []struct {
		name string
		act  Activation
	}{
		{
			name: "nested wrapper",
			act:  &testActivationWrapper{Activation: &testActivationWrapper{Activation: frame, name: "w1"}, name: "w2"},
		},
		{
			name: "parent hierarchy",
			act:  &parentActivationWrapper{parent: frame},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			found := findFrame(tc.act)
			if found != frame {
				t.Errorf("findFrame() = %v, wanted %v", found, frame)
			}
		})
	}
}

func TestObservableInterpretable(t *testing.T) {
	obsInt := &ObservableInterpretable{InterpretableV2: NewConstValue(12, types.True)}
	if obsInt.ID() != 12 {
		t.Errorf("obsInt.ID() = %d, wanted 12", obsInt.ID())
	}
	res := obsInt.Eval(EmptyActivation())
	if res.Equal(types.True) != types.True {
		t.Errorf("obsInt.Eval() = %v, wanted true", res)
	}
}

type testLegacyInterpretable struct {
	id int64
}

func (t *testLegacyInterpretable) ID() int64 {
	return t.id
}

func (t *testLegacyInterpretable) Eval(vars Activation) ref.Val {
	return types.IntOne
}

type parentActivationWrapper struct {
	parent Activation
}

func (paw *parentActivationWrapper) ResolveName(name string) (any, bool) {
	return paw.parent.ResolveName(name)
}

func (paw *parentActivationWrapper) Parent() Activation {
	return paw.parent
}
