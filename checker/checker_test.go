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

package checker

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/debug"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"
	"github.com/google/cel-go/test"

	proto2pb "github.com/google/cel-go/test/proto2pb"
	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func testCases(t testing.TB) []testInfo {
	t.Helper()
	return []testInfo{
		// Const types
		{
			in:      `"A"`,
			out:     `"A"~string`,
			outType: types.StringType,
		},
		{
			in:      `12`,
			out:     `12~int`,
			outType: types.IntType,
		},
		{
			in:      `12u`,
			out:     `12u~uint`,
			outType: types.UintType,
		},
		{
			in:      `true`,
			out:     `true~bool`,
			outType: types.BoolType,
		},
		{
			in:      `false`,
			out:     `false~bool`,
			outType: types.BoolType,
		},
		{
			in:      `12.23`,
			out:     `12.23~double`,
			outType: types.DoubleType,
		},
		{
			in:      `null`,
			out:     `null~null`,
			outType: types.NullType,
		},
		{
			in:      `b"ABC"`,
			out:     `b"ABC"~bytes`,
			outType: types.BytesType,
		},
		// Ident types
		{
			in:      `is`,
			out:     `is~string^is`,
			outType: types.StringType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `ii`,
			out:     `ii~int^ii`,
			outType: types.IntType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `iu`,
			out:     `iu~uint^iu`,
			outType: types.UintType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `iz`,
			out:     `iz~bool^iz`,
			outType: types.BoolType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `id`,
			out:     `id~double^id`,
			outType: types.DoubleType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `ix`,
			out:     `ix~null^ix`,
			outType: types.NullType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `ib`,
			out:     `ib~bytes^ib`,
			outType: types.BytesType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `id`,
			out:     `id~double^id`,
			outType: types.DoubleType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `[]`,
			out:     `[]~list(dyn)`,
			outType: types.NewListType(types.DynType),
		},
		{
			in:      `[1]`,
			out:     `[1~int]~list(int)`,
			outType: types.NewListType(types.IntType),
		},
		{
			in:      `[1, "A"]`,
			out:     `[1~int, "A"~string]~list(dyn)`,
			outType: types.NewListType(types.DynType),
		},
		{
			in:      `foo`,
			out:     `foo~!error!`,
			outType: types.ErrorType,
			err: `
ERROR: <input>:1:1: undeclared reference to 'foo' (in container '')
| foo
| ^`,
		},
		// Call resolution
		{
			in:      `fg_s()`,
			out:     `fg_s()~string^fg_s_0`,
			outType: types.StringType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `is.fi_s_s()`,
			out:     `is~string^is.fi_s_s()~string^fi_s_s_0`,
			outType: types.StringType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `1 + 2`,
			out:     `_+_(1~int, 2~int)~int^add_int64`,
			outType: types.IntType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `1 + ii`,
			out:     `_+_(1~int, ii~int^ii)~int^add_int64`,
			outType: types.IntType,
			env:     testEnvs(t)["default"],
		},
		{
			in:      `[1] + [2]`,
			out:     `_+_([1~int]~list(int), [2~int]~list(int))~list(int)^add_list`,
			outType: types.NewListType(types.IntType),
			env:     testEnvs(t)["default"],
		},
		{
			in:      `[] + [1,2,3,] + [4]`,
			outType: types.NewListType(types.IntType),
			out: `
	_+_(
		_+_(
			[]~list(int),
			[1~int, 2~int, 3~int]~list(int))~list(int)^add_list,
			[4~int]~list(int))
	~list(int)^add_list
	`,
		},
		{
			in: `[1, 2u] + []`,
			out: `_+_(
			[
				1~int,
				2u~uint
			]~list(dyn),
			[]~list(dyn)
		)~list(dyn)^add_list`,
			outType: types.NewListType(types.DynType),
		},
		{
			in:      `{1:2u, 2:3u}`,
			outType: types.NewMapType(types.IntType, types.UintType),
			out:     `{1~int : 2u~uint, 2~int : 3u~uint}~map(int, uint)`,
		},
		{
			in:      `{"a":1, "b":2}.a`,
			outType: types.IntType,
			out:     `{"a"~string : 1~int, "b"~string : 2~int}~map(string, int).a~int`,
		},
		{
			in:      `{1:2u, 2u:3}`,
			outType: types.NewMapType(types.DynType, types.DynType),
			out:     `{1~int : 2u~uint, 2u~uint : 3~int}~map(dyn, dyn)`,
		},
		{
			in:        `TestAllTypes{single_int32: 1, single_int64: 2}`,
			container: "google.expr.proto3.test",
			out: `
		google.expr.proto3.test.TestAllTypes{
			single_int32 : 1~int,
			single_int64 : 2~int
		}~google.expr.proto3.test.TestAllTypes^google.expr.proto3.test.TestAllTypes`,
			outType: types.NewObjectType("google.expr.proto3.test.TestAllTypes"),
		},
		{
			in:        `TestAllTypes{single_int32: 1u}`,
			container: "google.expr.proto3.test",
			err: `
	ERROR: <input>:1:26: expected type of field 'single_int32' is 'int' but provided type is 'uint'
	  | TestAllTypes{single_int32: 1u}
	  | .........................^`,
		},
		{
			in:        `TestAllTypes{single_int32: 1, undefined: 2}`,
			container: "google.expr.proto3.test",
			err: `
	ERROR: <input>:1:40: undefined field 'undefined'
	  | TestAllTypes{single_int32: 1, undefined: 2}
	  | .......................................^`,
		},
		{
			in: `size(x) == x.size()`,
			out: `
_==_(size(x~list(int)^x)~int^size_list, x~list(int)^x.size()~int^list_size)
  ~bool^equals`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewListType(types.IntType)),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `int(1u) + int(uint("1"))`,
			out: `
_+_(int(1u~uint)~int^uint64_to_int64,
      int(uint("1"~string)~uint^string_to_uint64)~int^uint64_to_int64)
  ~int^add_int64`,
			outType: types.IntType,
		},
		{
			in: `false && !true || false ? 2 : 3`,
			out: `
_?_:_(_||_(_&&_(false~bool, !_(true~bool)~bool^logical_not)~bool^logical_and,
            false~bool)
        ~bool^logical_or,
      2~int,
      3~int)
  ~int^conditional
`,
			outType: types.IntType,
		},
		{
			in:      `b"abc" + b"def"`,
			out:     `_+_(b"abc"~bytes, b"def"~bytes)~bytes^add_bytes`,
			outType: types.BytesType,
		},
		{
			in: `1.0 + 2.0 * 3.0 - 1.0 / 2.20202 != 66.6`,
			out: `
_!=_(_-_(_+_(1~double, _*_(2~double, 3~double)~double^multiply_double)
           ~double^add_double,
           _/_(1~double, 2.20202~double)~double^divide_double)
       ~double^subtract_double,
      66.6~double)
  ~bool^not_equals`,
			outType: types.BoolType,
		},
		{
			in: `null == null && null != null`,
			out: `
		_&&_(
			_==_(
				null~null,
				null~null
			)~bool^equals,
			_!=_(
				null~null,
				null~null
			)~bool^not_equals
		)~bool^logical_and`,
			outType: types.BoolType,
		},
		{
			in: `1 == 1 && 2 != 1`,
			out: `
		_&&_(
			_==_(
				1~int,
				1~int
			)~bool^equals,
			_!=_(
				2~int,
				1~int
			)~bool^not_equals
		)~bool^logical_and`,
			outType: types.BoolType,
		},
		{
			in:      `1 + 2 * 3 - 1 / 2 == 6 % 1`,
			out:     ` _==_(_-_(_+_(1~int, _*_(2~int, 3~int)~int^multiply_int64)~int^add_int64, _/_(1~int, 2~int)~int^divide_int64)~int^subtract_int64, _%_(6~int, 1~int)~int^modulo_int64)~bool^equals`,
			outType: types.BoolType,
		},
		{
			in:      `"abc" + "def"`,
			out:     `_+_("abc"~string, "def"~string)~string^add_string`,
			outType: types.StringType,
		},
		{
			in: `1u + 2u * 3u - 1u / 2u == 6u % 1u`,
			out: `_==_(_-_(_+_(1u~uint, _*_(2u~uint, 3u~uint)~uint^multiply_uint64)
	         ~uint^add_uint64,
	         _/_(1u~uint, 2u~uint)~uint^divide_uint64)
	     ~uint^subtract_uint64,
	    _%_(6u~uint, 1u~uint)~uint^modulo_uint64)
	~bool^equals`,
			outType: types.BoolType,
		},
		{
			in: `x.single_int32 != null`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.Proto2Message")),
				},
			},
			err: `
	ERROR: <input>:1:2: unexpected failed resolution of 'google.expr.proto3.test.Proto2Message'
	  | x.single_int32 != null
	  | .^
	`,
		},
		{
			in: `x.single_value + 1 / x.single_struct.y == 23`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `_==_(
			_+_(
			  x~google.expr.proto3.test.TestAllTypes^x.single_value~dyn,
			  _/_(
				1~int,
				x~google.expr.proto3.test.TestAllTypes^x.single_struct~map(string, dyn).y~dyn
			  )~int^divide_int64
			)~int^add_int64,
			23~int
		  )~bool^equals`,
			outType: types.BoolType,
		},
		{
			in: `x.single_value[23] + x.single_struct['y']`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `_+_(
			_[_](
			  x~google.expr.proto3.test.TestAllTypes^x.single_value~dyn,
			  23~int
			)~dyn^index_list|index_map,
			_[_](
			  x~google.expr.proto3.test.TestAllTypes^x.single_struct~map(string, dyn),
			  "y"~string
			)~dyn^index_map
		  )~dyn^add_bytes|add_double|add_duration_duration|add_duration_timestamp|add_int64|add_list|add_string|add_timestamp_duration|add_uint64
		  `,
			outType: types.DynType,
		},
		{
			in:        `TestAllTypes.NestedEnum.BAR != 99`,
			container: "google.expr.proto3.test",
			out: `_!=_(google.expr.proto3.test.TestAllTypes.NestedEnum.BAR
	     ~int^google.expr.proto3.test.TestAllTypes.NestedEnum.BAR,
	    99~int)
	~bool^not_equals`,
			outType: types.BoolType,
		},
		{
			in:      `size([] + [1])`,
			out:     `size(_+_([]~list(int), [1~int]~list(int))~list(int)^add_list)~int^size_list`,
			outType: types.IntType,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
		},
		{
			in: `x["claims"]["groups"][0].name == "dummy"
		&& x.claims["exp"] == y[1].time
		&& x.claims.structured == {'key': z}
		&& z == 1.0`,
			out: `_&&_(
			_&&_(
				_==_(
					_[_](
						_[_](
							_[_](
								x~map(string, dyn)^x,
								"claims"~string
							)~dyn^index_map,
							"groups"~string
						)~list(dyn)^index_map,
						0~int
					)~dyn^index_list.name~dyn,
					"dummy"~string
				)~bool^equals,
				_==_(
					_[_](
						x~map(string, dyn)^x.claims~dyn,
						"exp"~string
					)~dyn^index_map,
					_[_](
						y~list(dyn)^y,
						1~int
					)~dyn^index_list.time~dyn
				)~bool^equals
			)~bool^logical_and,
			_&&_(
				_==_(
					x~map(string, dyn)^x.claims~dyn.structured~dyn,
					{
						"key"~string:z~dyn^z
					}~map(string, dyn)
				)~bool^equals,
				_==_(
					z~dyn^z,
					1~double
				)~bool^equals
			)~bool^logical_and
		)~bool^logical_and`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.protobuf.Struct")),
					decls.NewVariable("y", types.NewObjectType("google.protobuf.ListValue")),
					decls.NewVariable("z", types.NewObjectType("google.protobuf.Value")),
				},
			},
			outType: types.BoolType,
		},
		{
			in:  `x + y`,
			out: ``,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewListType(types.NewObjectType("google.expr.proto3.test.TestAllTypes"))),
					decls.NewVariable("y", types.NewListType(types.IntType)),
				},
			},
			err: `
ERROR: <input>:1:3: found no matching overload for '_+_' applied to '(list(google.expr.proto3.test.TestAllTypes), list(int))'
  | x + y
  | ..^
		`,
		},
		{
			in: `x[1u]`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewListType(types.NewObjectType("google.expr.proto3.test.TestAllTypes"))),
				},
			},
			err: `
ERROR: <input>:1:2: found no matching overload for '_[_]' applied to '(list(google.expr.proto3.test.TestAllTypes), uint)'
  | x[1u]
  | .^
`,
		},
		{
			in: `(x + x)[1].single_int32 == size(x)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewListType(types.NewObjectType("google.expr.proto3.test.TestAllTypes"))),
				},
			},
			out: `
_==_(_[_](_+_(x~list(google.expr.proto3.test.TestAllTypes)^x,
                x~list(google.expr.proto3.test.TestAllTypes)^x)
            ~list(google.expr.proto3.test.TestAllTypes)^add_list,
           1~int)
       ~google.expr.proto3.test.TestAllTypes^index_list
       .
       single_int32
       ~int,
      size(x~list(google.expr.proto3.test.TestAllTypes)^x)~int^size_list)
  ~bool^equals
	`,
			outType: types.BoolType,
		},
		{
			in: `x.repeated_int64[x.single_int32] == 23`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
_==_(_[_](x~google.expr.proto3.test.TestAllTypes^x.repeated_int64~list(int),
           x~google.expr.proto3.test.TestAllTypes^x.single_int32~int)
       ~int^index_list,
      23~int)
  ~bool^equals`,
			outType: types.BoolType,
		},
		{
			in: `size(x.map_int64_nested_type) == 0`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
_==_(size(x~google.expr.proto3.test.TestAllTypes^x.map_int64_nested_type
            ~map(int, google.expr.proto3.test.NestedTestAllTypes))
       ~int^size_map,
      0~int)
  ~bool^equals
		`,
			outType: types.BoolType,
		},
		{
			in: `x.all(y, y == true)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.BoolType),
				},
			},
			out: `
		__comprehension__(
		// Variable
		y,
		// Target
		x~bool^x,
		// Accumulator
		@result,
		// Init
		true~bool,
		// LoopCondition
		@not_strictly_false(
			@result~bool^@result
		)~bool^not_strictly_false,
		// LoopStep
		_&&_(
			@result~bool^@result,
			_==_(
			y~!error!^y,
			true~bool
			)~bool^equals
		)~bool^logical_and,
		// Result
		@result~bool^@result)~bool
		`,
			err: `ERROR: <input>:1:1: expression of type 'bool' cannot be range of a comprehension (must be list, map, or dynamic)
		| x.all(y, y == true)
		| ^`,
		},
		{
			in: `x.repeated_int64.map(x, double(x))`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
		__comprehension__(
    		  // Variable
    		  x,
    		  // Target
    		  x~google.expr.proto3.test.TestAllTypes^x.repeated_int64~list(int),
    		  // Accumulator
    		  @result,
    		  // Init
    		  []~list(double),
    		  // LoopCondition
    		  true~bool,
    		  // LoopStep
    		  _+_(
    		    @result~list(double)^@result,
    		    [
    		      double(
    		        x~int^x
    		      )~double^int64_to_double
    		    ]~list(double)
    		  )~list(double)^add_list,
    		  // Result
    		  @result~list(double)^@result)~list(double)
		`,
			outType: types.NewListType(types.DoubleType),
		},
		{
			in: `x.repeated_int64.map(x, x > 0, double(x))`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
	__comprehension__(
    		  // Variable
    		  x,
    		  // Target
    		  x~google.expr.proto3.test.TestAllTypes^x.repeated_int64~list(int),
    		  // Accumulator
    		  @result,
    		  // Init
    		  []~list(double),
    		  // LoopCondition
    		  true~bool,
    		  // LoopStep
    		  _?_:_(
    		    _>_(
    		      x~int^x,
    		      0~int
    		    )~bool^greater_int64,
    		    _+_(
    		      @result~list(double)^@result,
    		      [
    		        double(
    		          x~int^x
    		        )~double^int64_to_double
    		      ]~list(double)
    		    )~list(double)^add_list,
    		    @result~list(double)^@result
    		  )~list(double)^conditional,
    		  // Result
    		  @result~list(double)^@result)~list(double)
		`,
			outType: types.NewListType(types.DoubleType),
		},
		{
			in: `x[2].single_int32 == 23`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x",
						types.NewMapType(types.StringType,
							types.NewObjectType("google.expr.proto3.test.TestAllTypes"))),
				},
			},
			err: `
ERROR: <input>:1:2: found no matching overload for '_[_]' applied to '(map(string, google.expr.proto3.test.TestAllTypes), int)'
  | x[2].single_int32 == 23
  | .^
		`,
		},
		{
			in: `x["a"].single_int32 == 23`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x",
						types.NewMapType(types.StringType,
							types.NewObjectType("google.expr.proto3.test.TestAllTypes"))),
				},
			},
			out: `
		_==_(_[_](x~map(string, google.expr.proto3.test.TestAllTypes)^x, "a"~string)
		~google.expr.proto3.test.TestAllTypes^index_map
		.
		single_int32
		~int,
		23~int)
		~bool^equals`,
			outType: types.BoolType,
		},
		{
			in: `x.single_nested_message.bb == 43 && has(x.single_nested_message)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},

			// Our implementation code is expanding the macro
			out: `_&&_(
    		  _==_(
    		    x~google.expr.proto3.test.TestAllTypes^x.single_nested_message~google.expr.proto3.test.TestAllTypes.NestedMessage.bb~int,
    		    43~int
    		  )~bool^equals,
    		  x~google.expr.proto3.test.TestAllTypes^x.single_nested_message~test-only~~bool
    		)~bool^logical_and`,
			outType: types.BoolType,
		},
		{
			in: `x.single_nested_message.undefined == x.undefined && has(x.single_int32) && has(x.repeated_int32)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			err: `
ERROR: <input>:1:24: undefined field 'undefined'
| x.single_nested_message.undefined == x.undefined && has(x.single_int32) && has(x.repeated_int32)
| .......................^
ERROR: <input>:1:39: undefined field 'undefined'
| x.single_nested_message.undefined == x.undefined && has(x.single_int32) && has(x.repeated_int32)
| ......................................^`,
		},
		{
			in: `x.single_nested_message != null`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
		_!=_(x~google.expr.proto3.test.TestAllTypes^x.single_nested_message
		~google.expr.proto3.test.TestAllTypes.NestedMessage,
		null~null)
		~bool^not_equals
		`,
			outType: types.BoolType,
		},
		{
			in: `x.single_int64 != null`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			err: `
ERROR: <input>:1:16: found no matching overload for '_!=_' applied to '(int, null)'
 | x.single_int64 != null
 | ...............^
		`,
		},
		{
			in: `x.single_int64_wrapper == null`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
		_==_(x~google.expr.proto3.test.TestAllTypes^x.single_int64_wrapper
		~wrapper(int),
		null~null)
		~bool^equals
		`,
			outType: types.BoolType,
		},
		{
			in: `x.single_bool_wrapper
		&& x.single_bytes_wrapper == b'hi'
		&& x.single_double_wrapper != 2.0
		&& x.single_float_wrapper == 1.0
		&& x.single_int32_wrapper != 2
		&& x.single_int64_wrapper == 1
		&& x.single_string_wrapper == 'hi'
		&& x.single_uint32_wrapper == 1u
		&& x.single_uint64_wrapper != 42u`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
		_&&_(
			_&&_(
				_&&_(
				_&&_(
					x~google.expr.proto3.test.TestAllTypes^x.single_bool_wrapper~wrapper(bool),
					_==_(
					x~google.expr.proto3.test.TestAllTypes^x.single_bytes_wrapper~wrapper(bytes),
					b"hi"~bytes
					)~bool^equals
				)~bool^logical_and,
				_!=_(
					x~google.expr.proto3.test.TestAllTypes^x.single_double_wrapper~wrapper(double),
					2~double
				)~bool^not_equals
				)~bool^logical_and,
				_&&_(
				_==_(
					x~google.expr.proto3.test.TestAllTypes^x.single_float_wrapper~wrapper(double),
					1~double
				)~bool^equals,
				_!=_(
					x~google.expr.proto3.test.TestAllTypes^x.single_int32_wrapper~wrapper(int),
					2~int
				)~bool^not_equals
				)~bool^logical_and
			)~bool^logical_and,
			_&&_(
				_&&_(
				_==_(
					x~google.expr.proto3.test.TestAllTypes^x.single_int64_wrapper~wrapper(int),
					1~int
				)~bool^equals,
				_==_(
					x~google.expr.proto3.test.TestAllTypes^x.single_string_wrapper~wrapper(string),
					"hi"~string
				)~bool^equals
				)~bool^logical_and,
				_&&_(
				_==_(
					x~google.expr.proto3.test.TestAllTypes^x.single_uint32_wrapper~wrapper(uint),
					1u~uint
				)~bool^equals,
				_!=_(
					x~google.expr.proto3.test.TestAllTypes^x.single_uint64_wrapper~wrapper(uint),
					42u~uint
				)~bool^not_equals
				)~bool^logical_and
			)~bool^logical_and
		)~bool^logical_and`,
			outType: types.BoolType,
		},
		{
			in: `x.single_timestamp == google.protobuf.Timestamp{seconds: 20} &&
		     x.single_duration < google.protobuf.Duration{seconds: 10}`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `x.single_bool_wrapper == google.protobuf.BoolValue{value: true}
			&& x.single_bytes_wrapper == google.protobuf.BytesValue{value: b'hi'}
			&& x.single_double_wrapper != google.protobuf.DoubleValue{value: 2.0}
			&& x.single_float_wrapper == google.protobuf.FloatValue{value: 1.0}
			&& x.single_int32_wrapper != google.protobuf.Int32Value{value: -2}
			&& x.single_int64_wrapper == google.protobuf.Int64Value{value: 1}
			&& x.single_string_wrapper == google.protobuf.StringValue{value: 'hi'}
			&& x.single_string_wrapper == google.protobuf.Value{string_value: 'hi'}
			&& x.single_uint32_wrapper == google.protobuf.UInt32Value{value: 1u}
			&& x.single_uint64_wrapper != google.protobuf.UInt64Value{value: 42u}`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `x.repeated_int64.exists(y, y > 10) && y < 5`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			err: `ERROR: <input>:1:39: undeclared reference to 'y' (in container '')
		| x.repeated_int64.exists(y, y > 10) && y < 5
		| ......................................^`,
		},
		{
			in: `x.repeated_int64.all(e, e > 0) && x.repeated_int64.exists(e, e < 0) && x.repeated_int64.exists_one(e, e == 0)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `_&&_(
			_&&_(
			  __comprehension__(
				// Variable
				e,
				// Target
				x~google.expr.proto3.test.TestAllTypes^x.repeated_int64~list(int),
				// Accumulator
				@result,
				// Init
				true~bool,
				// LoopCondition
				@not_strictly_false(
				  @result~bool^@result
				)~bool^not_strictly_false,
				// LoopStep
				_&&_(
				  @result~bool^@result,
				  _>_(
					e~int^e,
					0~int
				  )~bool^greater_int64
				)~bool^logical_and,
				// Result
				@result~bool^@result)~bool,
			  __comprehension__(
				// Variable
				e,
				// Target
				x~google.expr.proto3.test.TestAllTypes^x.repeated_int64~list(int),
				// Accumulator
				@result,
				// Init
				false~bool,
				// LoopCondition
				@not_strictly_false(
				  !_(
					@result~bool^@result
				  )~bool^logical_not
				)~bool^not_strictly_false,
				// LoopStep
				_||_(
				  @result~bool^@result,
				  _<_(
					e~int^e,
					0~int
				  )~bool^less_int64
				)~bool^logical_or,
				// Result
				@result~bool^@result)~bool
			)~bool^logical_and,
			__comprehension__(
			  // Variable
			  e,
			  // Target
			  x~google.expr.proto3.test.TestAllTypes^x.repeated_int64~list(int),
			  // Accumulator
			  @result,
			  // Init
			  0~int,
			  // LoopCondition
			  true~bool,
			  // LoopStep
			  _?_:_(
				_==_(
				  e~int^e,
				  0~int
				)~bool^equals,
				_+_(
				  @result~int^@result,
				  1~int
				)~int^add_int64,
				@result~int^@result
			  )~int^conditional,
			  // Result
			  _==_(
				@result~int^@result,
				1~int
			  )~bool^equals)~bool
		  )~bool^logical_and`,
			outType: types.BoolType,
		},

		{
			in: `x.all(e, 0)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			err: `
ERROR: <input>:1:1: expression of type 'google.expr.proto3.test.TestAllTypes' cannot be range of a comprehension (must be list, map, or dynamic)
 | x.all(e, 0)
 | ^
ERROR: <input>:1:10: expected type 'bool' but found 'int'
 | x.all(e, 0)
 | .........^
		`,
		},
		{
			in: `lists.filter(x, x > 1.5)`,
			out: `__comprehension__(
			// Variable
			x,
			// Target
			lists~dyn^lists,
			// Accumulator
			@result,
			// Init
			[]~list(dyn),
			// LoopCondition
			true~bool,
			// LoopStep
			_?_:_(
			  _>_(
				x~dyn^x,
				1.5~double
			  )~bool^greater_double|greater_int64_double|greater_uint64_double,
			  _+_(
				@result~list(dyn)^@result,
				[
				  x~dyn^x
				]~list(dyn)
			  )~list(dyn)^add_list,
			  @result~list(dyn)^@result
			)~list(dyn)^conditional,
			// Result
			@result~list(dyn)^@result)~list(dyn)`,
			outType: types.NewListType(types.DynType),
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("lists", types.DynType),
				},
			},
		},

		{
			in: `.google.expr.proto3.test.TestAllTypes`,
			out: `google.expr.proto3.test.TestAllTypes
	~type(google.expr.proto3.test.TestAllTypes)
	^google.expr.proto3.test.TestAllTypes`,
			outType: types.NewTypeTypeWithParam(
				types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
		},

		{
			in:        `test.TestAllTypes`,
			container: "google.expr.proto3",
			out: `
	google.expr.proto3.test.TestAllTypes
	~type(google.expr.proto3.test.TestAllTypes)
	^google.expr.proto3.test.TestAllTypes
		`,
			outType: types.NewTypeTypeWithParam(
				types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
		},

		{
			in: `1 + x`,
			err: `
ERROR: <input>:1:5: undeclared reference to 'x' (in container '')
 | 1 + x
 | ....^`,
		},

		{
			in: `x == google.protobuf.Any{
				type_url:'types.googleapis.com/google.expr.proto3.test.TestAllTypes'
			} && x.single_nested_message.bb == 43
			|| x == google.expr.proto3.test.TestAllTypes{}
			|| y < x
			|| x >= x`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.AnyType),
					decls.NewVariable("y", types.NewNullableType(types.IntType)),
				},
			},
			out: `
		_||_(
			_||_(
				_&&_(
					_==_(
						x~any^x,
						google.protobuf.Any{
							type_url:"types.googleapis.com/google.expr.proto3.test.TestAllTypes"~string
						}~any^google.protobuf.Any
					)~bool^equals,
					_==_(
						x~any^x.single_nested_message~dyn.bb~dyn,
						43~int
					)~bool^equals
				)~bool^logical_and,
				_==_(
					x~any^x,
					google.expr.proto3.test.TestAllTypes{}~google.expr.proto3.test.TestAllTypes^google.expr.proto3.test.TestAllTypes
				)~bool^equals
			)~bool^logical_or,
			_||_(
				_<_(
					y~wrapper(int)^y,
					x~any^x
				)~bool^less_int64|less_int64_double|less_int64_uint64,
				_>=_(
					x~any^x,
					x~any^x
				)~bool^greater_equals_bool|greater_equals_bytes|greater_equals_double|greater_equals_double_int64|greater_equals_double_uint64|greater_equals_duration|greater_equals_int64|greater_equals_int64_double|greater_equals_int64_uint64|greater_equals_string|greater_equals_timestamp|greater_equals_uint64|greater_equals_uint64_double|greater_equals_uint64_int64
			)~bool^logical_or
		)~bool^logical_or
		`,
			outType: types.BoolType,
		},

		{
			in: `x == google.protobuf.Any{
				type_url:'types.googleapis.com/google.expr.proto3.test.TestAllTypes'
			} && x.single_nested_message.bb == 43
			|| x == google.expr.proto3.test.TestAllTypes{}
			|| y < x
			|| x >= x`,
			env: testEnv{
				variadicASTs: true,
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.AnyType),
					decls.NewVariable("y", types.NewNullableType(types.IntType)),
				},
			},
			out: `
		_||_(
			_&&_(
			  _==_(
				x~any^x,
				google.protobuf.Any{
				  type_url:"types.googleapis.com/google.expr.proto3.test.TestAllTypes"~string
				}~any^google.protobuf.Any
			  )~bool^equals,
			  _==_(
				x~any^x.single_nested_message~dyn.bb~dyn,
				43~int
			  )~bool^equals
			)~bool^logical_and,
			_==_(
			  x~any^x,
			  google.expr.proto3.test.TestAllTypes{}~google.expr.proto3.test.TestAllTypes^google.expr.proto3.test.TestAllTypes
			)~bool^equals,
			_<_(
			  y~wrapper(int)^y,
			  x~any^x
			)~bool^less_int64|less_int64_double|less_int64_uint64,
			_>=_(
			  x~any^x,
			  x~any^x
			)~bool^greater_equals_bool|greater_equals_bytes|greater_equals_double|greater_equals_double_int64|greater_equals_double_uint64|greater_equals_duration|greater_equals_int64|greater_equals_int64_double|greater_equals_int64_uint64|greater_equals_string|greater_equals_timestamp|greater_equals_uint64|greater_equals_uint64_double|greater_equals_uint64_int64
		  )~bool^logical_or
		`,
			outType: types.BoolType,
		},

		{
			in:        `x`,
			container: "container",
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("container.x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out:     `container.x~google.expr.proto3.test.TestAllTypes^container.x`,
			outType: types.NewObjectType("google.expr.proto3.test.TestAllTypes"),
		},

		{
			in: `list == type([1]) && map == type({1:2u})`,
			out: `
_&&_(_==_(list~type(list(dyn))^list,
           type([1~int]~list(int))~type(list(int))^type)
       ~bool^equals,
      _==_(map~type(map(dyn, dyn))^map,
            type({1~int : 2u~uint}~map(int, uint))~type(map(int, uint))^type)
        ~bool^equals)
  ~bool^logical_and
	`,
			outType: types.BoolType,
		},

		{
			in: `myfun(1, true, 3u) + 1.myfun(false, 3u).myfun(true, 42u)`,
			env: testEnv{
				functions: []*decls.FunctionDecl{
					testFunction(t, "myfun",
						decls.MemberOverload("myfun_instance",
							[]*types.Type{types.IntType, types.BoolType, types.UintType}, types.IntType),
						decls.Overload("myfun_static",
							[]*types.Type{types.IntType, types.BoolType, types.UintType}, types.IntType)),
				},
			},
			out: `_+_(
    		  myfun(
    		    1~int,
    		    true~bool,
    		    3u~uint
    		  )~int^myfun_static,
    		  1~int.myfun(
    		    false~bool,
    		    3u~uint
    		  )~int^myfun_instance.myfun(
    		    true~bool,
    		    42u~uint
    		  )~int^myfun_instance
    		)~int^add_int64`,
			outType: types.IntType,
		},

		{
			in: `size(x) > 4`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
				functions: []*decls.FunctionDecl{
					testFunction(t, "size",
						decls.Overload("size_message",
							[]*types.Type{types.NewObjectType("google.expr.proto3.test.TestAllTypes")},
							types.IntType)),
				},
			},
			outType: types.BoolType,
		},

		{
			in: `x.single_int64_wrapper + 1 != 23`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
		_!=_(_+_(x~google.expr.proto3.test.TestAllTypes^x.single_int64_wrapper
		~wrapper(int),
		1~int)
		~int^add_int64,
		23~int)
		~bool^not_equals
		`,
			outType: types.BoolType,
		},

		{
			in: `x.single_int64_wrapper + y != 23`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
					decls.NewVariable("y", types.NewObjectType("google.protobuf.Int32Value")),
				},
			},
			out: `
		_!=_(
			_+_(
			  x~google.expr.proto3.test.TestAllTypes^x.single_int64_wrapper~wrapper(int),
			  y~wrapper(int)^y
			)~int^add_int64,
			23~int
		  )~bool^not_equals
		`,
			outType: types.BoolType,
		},

		{
			in: `1 in [1, 2, 3]`,
			out: `@in(
    		  1~int,
    		  [
    		    1~int,
    		    2~int,
    		    3~int
    		  ]~list(int)
    		)~bool^in_list`,
			outType: types.BoolType,
		},

		{
			in: `1 in dyn([1, 2, 3])`,
			out: `@in(
			1~int,
			dyn(
			  [
				1~int,
				2~int,
				3~int
			  ]~list(int)
			)~dyn^to_dyn
		  )~bool^in_list|in_map`,
			outType: types.BoolType,
		},

		{
			in: `type(null) == null_type`,
			out: `_==_(
    		  type(
    		    null~null
    		  )~type(null)^type,
    		  null_type~type(null)^null_type
    		)~bool^equals`,
			outType: types.BoolType,
		},

		{
			in: `type(type) == type`,
			out: `_==_(
		  type(
		    type~type(type)^type
		  )~type(type(type))^type,
		  type~type(type)^type
		)~bool^equals`,
			outType: types.BoolType,
		},
		{
			in: `([[[1]], [[2]], [[3]]][0][0] + [2, 3, {'four': {'five': 'six'}}])[3]`,
			out: `_[_](
			_+_(
				_[_](
					_[_](
						[
							[
								[
									1~int
								]~list(int)
							]~list(list(int)),
							[
								[
									2~int
								]~list(int)
							]~list(list(int)),
							[
								[
									3~int
								]~list(int)
							]~list(list(int))
						]~list(list(list(int))),
						0~int
					)~list(list(int))^index_list,
					0~int
				)~list(int)^index_list,
				[
					2~int,
					3~int,
					{
						"four"~string:{
							"five"~string:"six"~string
						}~map(string, string)
					}~map(string, map(string, string))
				]~list(dyn)
			)~list(dyn)^add_list,
			3~int
		)~dyn^index_list`,
			outType: types.DynType,
		},
		{
			in: `[1] + [dyn('string')]`,
			out: `_+_(
			[
				1~int
			]~list(int),
			[
				dyn(
					"string"~string
				)~dyn^to_dyn
			]~list(dyn)
		)~list(dyn)^add_list`,
			outType: types.NewListType(types.DynType),
		},
		{
			in: `[dyn('string')] + [1]`,
			out: `_+_(
			[
				dyn(
					"string"~string
				)~dyn^to_dyn
			]~list(dyn),
			[
				1~int
			]~list(int)
		)~list(dyn)^add_list`,
			outType: types.NewListType(types.DynType),
		},
		{
			in: `[].map(x, [].map(y, x in y && y in x))`,
			err: `
		ERROR: <input>:1:33: found no matching overload for '@in' applied to '(list(dyn), dyn)'
		| [].map(x, [].map(y, x in y && y in x))
		| ................................^`,
		},
		{
			in: `args.user["myextension"].customAttributes.filter(x, x.name == "hobbies")`,
			out: `__comprehension__(
			// Variable
			x,
			// Target
			_[_](
			args~map(string, dyn)^args.user~dyn,
			"myextension"~string
			)~dyn^index_map.customAttributes~dyn,
			// Accumulator
			@result,
			// Init
			[]~list(dyn),
			// LoopCondition
			true~bool,
			// LoopStep
			_?_:_(
			_==_(
				x~dyn^x.name~dyn,
				"hobbies"~string
			)~bool^equals,
			_+_(
				@result~list(dyn)^@result,
				[
				x~dyn^x
				]~list(dyn)
			)~list(dyn)^add_list,
			@result~list(dyn)^@result
			)~list(dyn)^conditional,
			// Result
			@result~list(dyn)^@result)~list(dyn)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("args", types.NewMapType(types.StringType, types.DynType)),
				},
			},
			outType: types.NewListType(types.DynType),
		},
		{
			in: `a.b + 1 == a[0]`,
			out: `_==_(
			_+_(
			  a~dyn^a.b~dyn,
			  1~int
			)~int^add_int64,
			_[_](
			  a~dyn^a,
			  0~int
			)~dyn^index_list|index_map
		  )~bool^equals`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewTypeParamType("T")),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `!has(pb2.single_int64)
		&& !has(pb2.repeated_int32)
		&& !has(pb2.map_string_string)
		&& !has(pb3.single_int64)
		&& !has(pb3.repeated_int32)
		&& !has(pb3.map_string_string)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("pb2", types.NewObjectType("google.expr.proto2.test.TestAllTypes")),
					decls.NewVariable("pb3", types.NewObjectType("google.expr.proto3.test.TestAllTypes")),
				},
			},
			out: `
		_&&_(
			_&&_(
			  _&&_(
				!_(
				  pb2~google.expr.proto2.test.TestAllTypes^pb2.single_int64~test-only~~bool
				)~bool^logical_not,
				!_(
				  pb2~google.expr.proto2.test.TestAllTypes^pb2.repeated_int32~test-only~~bool
				)~bool^logical_not
			  )~bool^logical_and,
			  !_(
				pb2~google.expr.proto2.test.TestAllTypes^pb2.map_string_string~test-only~~bool
			  )~bool^logical_not
			)~bool^logical_and,
			_&&_(
			  _&&_(
				!_(
				  pb3~google.expr.proto3.test.TestAllTypes^pb3.single_int64~test-only~~bool
				)~bool^logical_not,
				!_(
				  pb3~google.expr.proto3.test.TestAllTypes^pb3.repeated_int32~test-only~~bool
				)~bool^logical_not
			  )~bool^logical_and,
			  !_(
				pb3~google.expr.proto3.test.TestAllTypes^pb3.map_string_string~test-only~~bool
			  )~bool^logical_not
			)~bool^logical_and
		  )~bool^logical_and`,
			outType: types.BoolType,
		},
		{
			in:        `TestAllTypes{}.repeated_nested_message`,
			container: "google.expr.proto2.test",
			out: `
		google.expr.proto2.test.TestAllTypes{}~google.expr.proto2.test.TestAllTypes^
		google.expr.proto2.test.TestAllTypes.repeated_nested_message
		~list(google.expr.proto2.test.TestAllTypes.NestedMessage)`,
			outType: types.NewListType(
				types.NewObjectType(
					"google.expr.proto2.test.TestAllTypes.NestedMessage",
				),
			),
		},
		{
			in:        `TestAllTypes{}.repeated_nested_message`,
			container: "google.expr.proto3.test",
			out: `
		google.expr.proto3.test.TestAllTypes{}~google.expr.proto3.test.TestAllTypes^
		google.expr.proto3.test.TestAllTypes.repeated_nested_message
		~list(google.expr.proto3.test.TestAllTypes.NestedMessage)`,
			outType: types.NewListType(
				types.NewObjectType(
					"google.expr.proto3.test.TestAllTypes.NestedMessage",
				),
			),
		},
		{
			in: `base64.encode('hello')`,
			env: testEnv{
				functions: []*decls.FunctionDecl{
					testFunction(t, "base64.encode",
						decls.Overload(
							"base64_encode_string",
							[]*types.Type{types.StringType},
							types.StringType)),
				},
			},
			out: `
		base64.encode(
			"hello"~string
		)~string^base64_encode_string`,
			outType: types.StringType,
		},
		{
			in:        `encode('hello')`,
			container: `base64`,
			env: testEnv{
				functions: []*decls.FunctionDecl{
					testFunction(t, "base64.encode",
						decls.Overload(
							"base64_encode_string",
							[]*types.Type{types.StringType},
							types.StringType)),
				},
			},
			out: `
		base64.encode(
			"hello"~string
		)~string^base64_encode_string`,
			outType: types.StringType,
		},
		{
			in:      `{}`,
			out:     `{}~map(dyn, dyn)`,
			outType: types.NewMapType(types.DynType, types.DynType),
		},
		{
			in: `set([1, 2, 3])`,
			out: `
		set(
		  [
		    1~int,
		    2~int,
		    3~int
		  ]~list(int)
		)~set(int)^set_list`,
			env: testEnv{
				functions: []*decls.FunctionDecl{
					testFunction(t, "set",
						decls.Overload("set_list",
							[]*types.Type{types.NewListType(types.NewTypeParamType("T"))},
							types.NewOpaqueType("set", types.NewTypeParamType("T")))),
				},
			},
			outType: types.NewOpaqueType("set", types.IntType),
		},
		{
			in: `set([1, 2]) == set([2, 1])`,
			out: `
		_==_(
		  set([1~int, 2~int]~list(int))~set(int)^set_list,
		  set([2~int, 1~int]~list(int))~set(int)^set_list
		)~bool^equals`,
			env: testEnv{
				functions: []*decls.FunctionDecl{
					testFunction(t, "set",
						decls.Overload(
							"set_list", []*types.Type{types.NewListType(types.NewTypeParamType("T"))},
							types.NewOpaqueType("set", types.NewTypeParamType("T")))),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `set([1, 2]) == x`,
			out: `
		_==_(
		  set([1~int, 2~int]~list(int))~set(int)^set_list,
		  x~set(int)^x
		)~bool^equals`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewOpaqueType("set", types.NewTypeParamType("T"))),
				},
				functions: []*decls.FunctionDecl{
					testFunction(t, "set",
						decls.Overload(
							"set_list", []*types.Type{types.NewListType(types.NewTypeParamType("T"))},
							types.NewOpaqueType("set", types.NewTypeParamType("T")))),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `int{}`,
			err: `
		ERROR: <input>:1:4: 'int' is not a message type
		 | int{}
		 | ...^
		`,
		},
		{
			in: `Msg{}`,
			err: `
		ERROR: <input>:1:4: undeclared reference to 'Msg' (in container '')
		 | Msg{}
		 | ...^
		`,
		},
		{
			in: `fun()`,
			err: `
		ERROR: <input>:1:4: undeclared reference to 'fun' (in container '')
		 | fun()
		 | ...^
		`,
		},
		{
			in: `'string'.fun()`,
			err: `
		ERROR: <input>:1:13: undeclared reference to 'fun' (in container '')
		 | 'string'.fun()
		 | ............^
		`,
		},
		{
			in: `[].length`,
			err: `
		ERROR: <input>:1:3: type 'list(_var0)' does not support field selection
		 | [].length
		 | ..^
		`,
		},
		{
			in:   `1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1`,
			opts: []Option{CrossTypeNumericComparisons(false)},
			err: `
		ERROR: <input>:1:3: found no matching overload for '_<=_' applied to '(int, double)'
		 | 1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1
		 | ..^
		ERROR: <input>:1:16: found no matching overload for '_<=_' applied to '(uint, double)'
		 | 1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1
		 | ...............^
		ERROR: <input>:1:30: found no matching overload for '_<=_' applied to '(double, int)'
		 | 1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1
		 | .............................^
		ERROR: <input>:1:42: found no matching overload for '_<=_' applied to '(double, uint)'
		 | 1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1
		 | .........................................^
		ERROR: <input>:1:53: found no matching overload for '_<=_' applied to '(int, uint)'
		 | 1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1
		 | ....................................................^
		ERROR: <input>:1:65: found no matching overload for '_<=_' applied to '(uint, int)'
		 | 1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1
		 | ................................................................^
		`,
		},
		{
			in:      `1 <= 1.0 && 1u <= 1.0 && 1.0 <= 1 && 1.0 <= 1u && 1 <= 1u && 1u <= 1`,
			opts:    []Option{CrossTypeNumericComparisons(true)},
			outType: types.BoolType,
			out: `
		_&&_(
			_&&_(
			  _&&_(
				_<=_(
				  1~int,
				  1~double
				)~bool^less_equals_int64_double,
				_<=_(
				  1u~uint,
				  1~double
				)~bool^less_equals_uint64_double
			  )~bool^logical_and,
			  _<=_(
				1~double,
				1~int
			  )~bool^less_equals_double_int64
			)~bool^logical_and,
			_&&_(
			  _&&_(
				_<=_(
				  1~double,
				  1u~uint
				)~bool^less_equals_double_uint64,
				_<=_(
				  1~int,
				  1u~uint
				)~bool^less_equals_int64_uint64
			  )~bool^logical_and,
			  _<=_(
				1u~uint,
				1~int
			  )~bool^less_equals_uint64_int64
			)~bool^logical_and
		  )~bool^logical_and`,
		},
		{
			in:      `1 < 1.0 && 1u < 1.0 && 1.0 < 1 && 1.0 < 1u && 1 < 1u && 1u < 1`,
			opts:    []Option{CrossTypeNumericComparisons(true)},
			outType: types.BoolType,
			out: `
		_&&_(
			_&&_(
			  _&&_(
				_<_(
				  1~int,
				  1~double
				)~bool^less_int64_double,
				_<_(
				  1u~uint,
				  1~double
				)~bool^less_uint64_double
			  )~bool^logical_and,
			  _<_(
				1~double,
				1~int
			  )~bool^less_double_int64
			)~bool^logical_and,
			_&&_(
			  _&&_(
				_<_(
				  1~double,
				  1u~uint
				)~bool^less_double_uint64,
				_<_(
				  1~int,
				  1u~uint
				)~bool^less_int64_uint64
			  )~bool^logical_and,
			  _<_(
				1u~uint,
				1~int
			  )~bool^less_uint64_int64
			)~bool^logical_and
		  )~bool^logical_and`,
		},
		{
			in:      `1 > 1.0 && 1u > 1.0 && 1.0 > 1 && 1.0 > 1u && 1 > 1u && 1u > 1`,
			opts:    []Option{CrossTypeNumericComparisons(true)},
			outType: types.BoolType,
			out: `
		_&&_(
			_&&_(
			  _&&_(
				_>_(
				  1~int,
				  1~double
				)~bool^greater_int64_double,
				_>_(
				  1u~uint,
				  1~double
				)~bool^greater_uint64_double
			  )~bool^logical_and,
			  _>_(
				1~double,
				1~int
			  )~bool^greater_double_int64
			)~bool^logical_and,
			_&&_(
			  _&&_(
				_>_(
				  1~double,
				  1u~uint
				)~bool^greater_double_uint64,
				_>_(
				  1~int,
				  1u~uint
				)~bool^greater_int64_uint64
			  )~bool^logical_and,
			  _>_(
				1u~uint,
				1~int
			  )~bool^greater_uint64_int64
			)~bool^logical_and
		  )~bool^logical_and`,
		},
		{
			in:      `1 >= 1.0 && 1u >= 1.0 && 1.0 >= 1 && 1.0 >= 1u && 1 >= 1u && 1u >= 1`,
			opts:    []Option{CrossTypeNumericComparisons(true)},
			outType: types.BoolType,
			out: `
		_&&_(
			_&&_(
			  _&&_(
				_>=_(
				  1~int,
				  1~double
				)~bool^greater_equals_int64_double,
				_>=_(
				  1u~uint,
				  1~double
				)~bool^greater_equals_uint64_double
			  )~bool^logical_and,
			  _>=_(
				1~double,
				1~int
			  )~bool^greater_equals_double_int64
			)~bool^logical_and,
			_&&_(
			  _&&_(
				_>=_(
				  1~double,
				  1u~uint
				)~bool^greater_equals_double_uint64,
				_>=_(
				  1~int,
				  1u~uint
				)~bool^greater_equals_int64_uint64
			  )~bool^logical_and,
			  _>=_(
				1u~uint,
				1~int
			  )~bool^greater_equals_uint64_int64
			)~bool^logical_and
		  )~bool^logical_and`,
		},
		{
			in:      `1 >= 1.0 && 1u >= 1.0 && 1.0 >= 1 && 1.0 >= 1u && 1 >= 1u && 1u >= 1`,
			opts:    []Option{CrossTypeNumericComparisons(true)},
			env:     testEnv{variadicASTs: true},
			outType: types.BoolType,
			out: `
		_&&_(
			_>=_(
			  1~int,
			  1~double
			)~bool^greater_equals_int64_double,
			_>=_(
			  1u~uint,
			  1~double
			)~bool^greater_equals_uint64_double,
			_>=_(
			  1~double,
			  1~int
			)~bool^greater_equals_double_int64,
			_>=_(
			  1~double,
			  1u~uint
			)~bool^greater_equals_double_uint64,
			_>=_(
			  1~int,
			  1u~uint
			)~bool^greater_equals_int64_uint64,
			_>=_(
			  1u~uint,
			  1~int
			)~bool^greater_equals_uint64_int64
		  )~bool^logical_and`,
		},
		{
			in:      `[1].map(x, [x, x]).map(x, [x, x])`,
			outType: types.NewListType(types.NewListType(types.NewListType(types.IntType))),
			out: `__comprehension__(
			// Variable
			x,
			// Target
			__comprehension__(
			  // Variable
			  x,
			  // Target
			  [
				1~int
			  ]~list(int),
			  // Accumulator
			  @result,
			  // Init
			  []~list(list(int)),
			  // LoopCondition
			  true~bool,
			  // LoopStep
			  _+_(
				@result~list(list(int))^@result,
				[
				  [
					x~int^x,
					x~int^x
				  ]~list(int)
				]~list(list(int))
			  )~list(list(int))^add_list,
			  // Result
			  @result~list(list(int))^@result)~list(list(int)),
			// Accumulator
			@result,
			// Init
			[]~list(list(list(int))),
			// LoopCondition
			true~bool,
			// LoopStep
			_+_(
			  @result~list(list(list(int)))^@result,
			  [
				[
				  x~list(int)^x,
				  x~list(int)^x
				]~list(list(int))
			  ]~list(list(list(int)))
			)~list(list(list(int)))^add_list,
			// Result
			@result~list(list(list(int)))^@result)~list(list(list(int)))
		  `,
		},
		{
			in:      `values.filter(i, i.content != "").map(i, i.content)`,
			outType: types.NewListType(types.StringType),
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("values", types.NewListType(types.NewMapType(types.StringType, types.StringType))),
				},
			},
			out: `__comprehension__(
			// Variable
			i,
			// Target
			__comprehension__(
			  // Variable
			  i,
			  // Target
			  values~list(map(string, string))^values,
			  // Accumulator
			  @result,
			  // Init
			  []~list(map(string, string)),
			  // LoopCondition
			  true~bool,
			  // LoopStep
			  _?_:_(
				_!=_(
				  i~map(string, string)^i.content~string,
				  ""~string
				)~bool^not_equals,
				_+_(
				  @result~list(map(string, string))^@result,
				  [
					i~map(string, string)^i
				  ]~list(map(string, string))
				)~list(map(string, string))^add_list,
				@result~list(map(string, string))^@result
			  )~list(map(string, string))^conditional,
			  // Result
			  @result~list(map(string, string))^@result)~list(map(string, string)),
			// Accumulator
			@result,
			// Init
			[]~list(string),
			// LoopCondition
			true~bool,
			// LoopStep
			_+_(
			  @result~list(string)^@result,
			  [
				i~map(string, string)^i.content~string
			  ]~list(string)
			)~list(string)^add_list,
			// Result
			@result~list(string)^@result)~list(string)`,
		},
		{
			in:      `[{}.map(c,c,c)]+[{}.map(c,c,c)]`,
			outType: types.NewListType(types.NewListType(types.BoolType)),
			out: `_+_(
			[
			  __comprehension__(
				// Variable
				c,
				// Target
				{}~map(bool, dyn),
				// Accumulator
				@result,
				// Init
				[]~list(bool),
				// LoopCondition
				true~bool,
				// LoopStep
				_?_:_(
				  c~bool^c,
				  _+_(
					@result~list(bool)^@result,
					[
					  c~bool^c
					]~list(bool)
				  )~list(bool)^add_list,
				  @result~list(bool)^@result
				)~list(bool)^conditional,
				// Result
				@result~list(bool)^@result)~list(bool)
			]~list(list(bool)),
			[
			  __comprehension__(
				// Variable
				c,
				// Target
				{}~map(bool, dyn),
				// Accumulator
				@result,
				// Init
				[]~list(bool),
				// LoopCondition
				true~bool,
				// LoopStep
				_?_:_(
				  c~bool^c,
				  _+_(
					@result~list(bool)^@result,
					[
					  c~bool^c
					]~list(bool)
				  )~list(bool)^add_list,
				  @result~list(bool)^@result
				)~list(bool)^conditional,
				// Result
				@result~list(bool)^@result)~list(bool)
			]~list(list(bool))
		  )~list(list(bool))^add_list`,
		},
		{
			in: "type(testAllTypes.nestedgroup.nested_id) == int",
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("testAllTypes", types.NewObjectType("google.expr.proto2.test.TestAllTypes")),
				},
			},
			outType: types.BoolType,
			out: `_==_(
			type(
			  testAllTypes~google.expr.proto2.test.TestAllTypes^testAllTypes.nestedgroup~google.expr.proto2.test.TestAllTypes.NestedGroup.nested_id~int
			)~type(int)^type,
			int~type(int)^int
		  )~bool^equals`,
		},
		{
			in: `a.?b`,
			env: testEnv{
				optionalSyntax: true,
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewMapType(types.StringType, types.StringType)),
				},
			},
			outType: types.NewOptionalType(types.StringType),
			out: `_?._(
			a~map(string, string)^a,
			"b"
		  )~optional_type(string)^select_optional_field`,
		},
		{
			in: `type(a.?b) == optional_type`,
			env: testEnv{
				optionalSyntax: true,
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewMapType(types.StringType, types.StringType)),
				},
			},
			outType: types.BoolType,
			out: `_==_(
				type(
				  _?._(
					a~map(string, string)^a,
					"b"
				  )~optional_type(string)^select_optional_field
				)~type(optional_type(string))^type,
				optional_type~type(optional_type(dyn))^optional_type
			  )~bool^equals`,
		},
		{
			in: `a.b`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewOptionalType(types.NewMapType(types.StringType, types.StringType))),
				},
			},
			outType: types.NewOptionalType(types.StringType),
			out:     `a~optional_type(map(string, string))^a.b~optional_type(string)`,
		},
		{
			in: `a.dynamic`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewOptionalType(types.DynType)),
				},
			},
			outType: types.NewOptionalType(types.DynType),
			out:     `a~optional_type(dyn)^a.dynamic~optional_type(dyn)`,
		},
		{
			in: `has(a.dynamic)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewOptionalType(types.DynType)),
				},
			},
			outType: types.BoolType,
			out:     `a~optional_type(dyn)^a.dynamic~test-only~~bool`,
		},
		{
			in: `has(a.?b.c)`,
			env: testEnv{
				optionalSyntax: true,
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewOptionalType(types.NewMapType(types.StringType, types.DynType))),
				},
			},
			outType: types.BoolType,
			out: `_?._(
			a~optional_type(map(string, dyn))^a,
			"b"
		  )~optional_type(dyn)^select_optional_field.c~test-only~~bool`,
		},
		{
			in:      `{?'key': {'a': 'b'}.?value}`,
			env:     testEnv{optionalSyntax: true},
			outType: types.NewMapType(types.StringType, types.StringType),
			out: `{
			?"key"~string:_?._(
			  {
				"a"~string:"b"~string
			  }~map(string, string),
			  "value"
			)~optional_type(string)^select_optional_field
		  }~map(string, string)`,
		},
		{
			in:      `{?'key': {'a': 'b'}.?value}.key`,
			env:     testEnv{optionalSyntax: true},
			outType: types.StringType,
			out: `{
			?"key"~string:_?._(
			  {
				"a"~string:"b"~string
			  }~map(string, string),
			  "value"
			)~optional_type(string)^select_optional_field
		  }~map(string, string).key~string`,
		},
		{
			in: `{?'nested': a.b}`,
			env: testEnv{
				optionalSyntax: true,
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewOptionalType(types.NewMapType(types.StringType, types.StringType))),
				},
			},
			outType: types.NewMapType(types.StringType, types.StringType),
			out: `{
			?"nested"~string:a~optional_type(map(string, string))^a.b~optional_type(string)
		  }~map(string, string)`,
		},
		{
			in:  `{?'key': 'hi'}`,
			env: testEnv{optionalSyntax: true},
			err: `ERROR: <input>:1:10: expected type 'optional_type(string)' but found 'string'
		| {?'key': 'hi'}
		| .........^`,
		},
		{
			in: `[?a, ?b, 'world']`,
			env: testEnv{
				optionalSyntax: true,
				idents: []*decls.VariableDecl{
					decls.NewVariable("a", types.NewOptionalType(types.StringType)),
					decls.NewVariable("b", types.NewOptionalType(types.StringType)),
				},
			},
			outType: types.NewListType(types.StringType),
			out: `[
			a~optional_type(string)^a,
			b~optional_type(string)^b,
			"world"~string
		  ]~list(string)`,
		},
		{
			in:  `[?'value']`,
			env: testEnv{optionalSyntax: true},
			err: `ERROR: <input>:1:3: expected type 'optional_type(string)' but found 'string'
		| [?'value']
		| ..^`,
		},
		{
			in:        `TestAllTypes{?single_int32: {}.?i}`,
			container: "google.expr.proto2.test",
			env:       testEnv{optionalSyntax: true},
			out: `google.expr.proto2.test.TestAllTypes{
			?single_int32:_?._(
			  {}~map(dyn, int),
			  "i"
			)~optional_type(int)^select_optional_field
		  }~google.expr.proto2.test.TestAllTypes^google.expr.proto2.test.TestAllTypes`,
			outType: types.NewObjectType(
				"google.expr.proto2.test.TestAllTypes",
			),
		},
		{
			in:        `TestAllTypes{?single_int32: 1}`,
			container: "google.expr.proto2.test",
			env:       testEnv{optionalSyntax: true},
			err: `ERROR: <input>:1:29: expected type 'optional_type(int)' but found 'int'
		| TestAllTypes{?single_int32: 1}
		| ............................^`,
		},
		{
			in: `undef`,
			err: `ERROR: <input>:1:1: undeclared reference to 'undef' (in container '')
			| undef
			| ^`,
		},
		{
			in: `undef()`,
			err: `ERROR: <input>:1:6: undeclared reference to 'undef' (in container '')
			| undef()
			| .....^`,
		},
		{
			in: `null_int == null || null == null_int || null_msg == null || null == null_msg`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("null_int", types.NewNullableType(types.IntType)),
					decls.NewVariable("null_msg", types.NewObjectType("google.expr.proto2.test.TestAllTypes")),
				},
			},
			outType: types.BoolType,
		},
		{
			in: `NotAMessage{}`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("NotAMessage", types.NewNullableType(types.IntType)),
				},
			},
			err: `ERROR: <input>:1:12: undeclared reference to 'NotAMessage' (in container '')
             | NotAMessage{}
             | ...........^`,
		},
		{
			in: `{}.map(c,[c,type(c)])`,
			out: `__comprehension__(
				// Variable
				c,
				// Target
				{}~map(dyn, dyn),
				// Accumulator
				@result,
				// Init
				[]~list(list(dyn)),
				// LoopCondition
				true~bool,
				// LoopStep
				_+_(
				  @result~list(list(dyn))^@result,
				  [
					[
					  c~dyn^c,
					  type(
						c~dyn^c
					  )~type(dyn)^type
					]~list(dyn)
				  ]~list(list(dyn))
				)~list(list(dyn))^add_list,
				// Result
				@result~list(list(dyn))^@result)~list(list(dyn))`,
			outType: types.NewListType(types.NewListType(types.DynType)),
		},
		{
			in: `[{'z': 0}].exists(y, y.z == 0)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("cel.example.y", types.NewMapType(types.StringType, types.IntType)),
				},
			},
			out: `__comprehension__(
              // Variable
              y,
              // Target
              [
                {
                  "z"~string:0~int
                }~map(string, int)
              ]~list(map(string, int)),
              // Accumulator
              @result,
              // Init
              false~bool,
              // LoopCondition
              @not_strictly_false(
                !_(
                  @result~bool^@result
                )~bool^logical_not
              )~bool^not_strictly_false,
              // LoopStep
              _||_(
                @result~bool^@result,
                _==_(
                  y~map(string, int)^y.z~int,
                  0~int
                )~bool^equals
              )~bool^logical_or,
              // Result
              @result~bool^@result)~bool`,
			outType: types.BoolType,
		},
		{
			in: `[{'y': 0}].exists(x, x.y == 0)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.NewMapType(types.StringType, types.IntType)),
				},
			},
			out: `__comprehension__(
              // Variable
              x,
              // Target
              [
                {
                  "y"~string:0~int
                }~map(string, int)
              ]~list(map(string, int)),
              // Accumulator
              @result,
              // Init
              false~bool,
              // LoopCondition
              @not_strictly_false(
                !_(
                  @result~bool^@result
                )~bool^logical_not
              )~bool^not_strictly_false,
              // LoopStep
              _||_(
                @result~bool^@result,
                _==_(
                  x~map(string, int)^x.y~int,
                  0~int
                )~bool^equals
              )~bool^logical_or,
              // Result
              @result~bool^@result)~bool`,
			outType: types.BoolType,
		},
		{
			in: `[0].exists(x, x != .x)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("x", types.IntType),
				},
			},
			out: `__comprehension__(
              // Variable
              x,
              // Target
              [
                0~int
              ]~list(int),
              // Accumulator
              @result,
              // Init
              false~bool,
              // LoopCondition
              @not_strictly_false(
                !_(
                  @result~bool^@result
                )~bool^logical_not
              )~bool^not_strictly_false,
              // LoopStep
              _||_(
                @result~bool^@result,
                _!=_(
                  x~int^x,
                  .x~int^.x
                )~bool^not_equals
              )~bool^logical_or,
              // Result
              @result~bool^@result)~bool`,
			outType: types.BoolType,
		},
		{
			in: `[{'z': 0}].exists(y, .y.z == y.z)`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("y.z", types.IntType),
				},
			},
			out: `__comprehension__(
              // Variable
              y,
              // Target
              [
                {
                  "z"~string:0~int
                }~map(string, int)
              ]~list(map(string, int)),
              // Accumulator
              @result,
              // Init
              false~bool,
              // LoopCondition
              @not_strictly_false(
                !_(
                  @result~bool^@result
                )~bool^logical_not
              )~bool^not_strictly_false,
              // LoopStep
              _||_(
                @result~bool^@result,
                _==_(
                  .y.z~int^.y.z,
                  y~map(string, int)^y.z~int
                )~bool^equals
              )~bool^logical_or,
              // Result
              @result~bool^@result)~bool`,
			outType: types.BoolType,
		},
		{
			in:        `TestAllTypes{?singleInt32: {}.?i}`,
			container: "google.expr.proto2.test",
			env:       testEnv{optionalSyntax: true, jsonFieldNames: true},
			out: `google.expr.proto2.test.TestAllTypes{
			?singleInt32:_?._(
			  {}~map(dyn, int),
			  "i"
			)~optional_type(int)^select_optional_field
		  }~google.expr.proto2.test.TestAllTypes^google.expr.proto2.test.TestAllTypes`,
			outType: types.NewObjectType(
				"google.expr.proto2.test.TestAllTypes",
			),
		},
		{
			in:        `TestAllTypes{?singleInt32: {'i': 20}.?i}.singleInt32`,
			container: "google.expr.proto2.test",
			env:       testEnv{optionalSyntax: true, jsonFieldNames: true},
			out: `google.expr.proto2.test.TestAllTypes{
              ?singleInt32:_?._(
                {
                  "i"~string:20~int
                }~map(string, int),
                "i"
              )~optional_type(int)^select_optional_field
            }~google.expr.proto2.test.TestAllTypes^google.expr.proto2.test.TestAllTypes.singleInt32~int`,
			outType: types.IntType,
		},
		{
			in:        `TestAllTypes{singleInt32: 1, single_bool: true}.singleInt32`,
			container: "google.expr.proto2.test",
			env:       testEnv{optionalSyntax: true, jsonFieldNames: true},
			err: `ERROR: <input>:1:41: undefined field 'single_bool'
             | TestAllTypes{singleInt32: 1, single_bool: true}.singleInt32
             | ........................................^`,
		},
	}
}

func TestListUnification(t *testing.T) {
	tests := []struct {
		in      string
		env     testEnv
		outType *types.Type
	}{
		{
			in: `[1, null_int, null]`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("null_int", types.NewNullableType(types.IntType)),
				},
			},
			outType: types.NewListType(types.NewNullableType(types.IntType)),
		},
		{
			in: `[null, 1, null_int]`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("null_int", types.NewNullableType(types.IntType)),
				},
			},
			outType: types.NewListType(types.NewNullableType(types.IntType)),
		},
		{
			in: `[null_int, null, 1]`,
			env: testEnv{
				idents: []*decls.VariableDecl{
					decls.NewVariable("null_int", types.NewNullableType(types.IntType)),
				},
			},
			outType: types.NewListType(types.NewNullableType(types.IntType)),
		},
		{
			in:      `[1, null]`,
			outType: types.NewListType(types.NewNullableType(types.IntType)),
		},
		{
			in:      `[null, 1]`,
			outType: types.NewListType(types.NewNullableType(types.IntType)),
		},
		{
			in:      `['foo', null]`,
			outType: types.NewListType(types.NewNullableType(types.StringType)),
		},
		{
			in:      `[null, 'foo']`,
			outType: types.NewListType(types.NewNullableType(types.StringType)),
		},
	}

	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			src := common.NewTextSource(tc.in)
			pAst, errors := p.Parse(src)
			if len(errors.GetErrors()) > 0 {
				t.Fatalf("Parse(%s) failed: %v", tc.in, errors.ToDisplayString())
			}
			reg, err := types.NewProtoRegistry()
			if err != nil {
				t.Fatalf("types.NewProtoRegistry() failed: %v", err)
			}
			cont, err := containers.NewContainer()
			if err != nil {
				t.Fatalf("containers.NewContainer() failed: %v", err)
			}
			env, err := NewEnv(cont, reg)
			if err != nil {
				t.Fatalf("NewEnv failed: %v", err)
			}
			if len(tc.env.idents) > 0 {
				for _, id := range tc.env.idents {
					env.AddIdents(id)
				}
			}
			cAst, errs := Check(pAst, src, env)
			if len(errs.GetErrors()) > 0 {
				t.Fatalf("Check(%s) failed: %v", tc.in, errs.ToDisplayString())
			}
			gotType := cAst.GetType(cAst.Expr().ID())
			if !gotType.IsExactType(tc.outType) {
				t.Errorf("Check(%s) type = %v, want %v", tc.in, gotType, tc.outType)
			}
		})
	}
}

func BenchmarkListUnification(b *testing.B) {
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		b.Fatalf("parser.NewParser() failed: %v", err)
	}
	src := common.NewTextSource(`[1, null_int, null]`)
	pAst, errors := p.Parse(src)
	if len(errors.GetErrors()) > 0 {
		b.Fatalf("Parse failed: %v", errors.ToDisplayString())
	}
	reg, err := types.NewProtoRegistry()
	if err != nil {
		b.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	cont, err := containers.NewContainer()
	if err != nil {
		b.Fatalf("containers.NewContainer() failed: %v", err)
	}
	env, err := NewEnv(cont, reg)
	if err != nil {
		b.Fatalf("NewEnv failed: %v", err)
	}
	env.AddIdents(decls.NewVariable("null_int", types.NewNullableType(types.IntType)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Check(pAst, src, env)
	}
}

func BenchmarkConcreteList(b *testing.B) {
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		b.Fatalf("parser.NewParser() failed: %v", err)
	}
	src := common.NewTextSource(`[1, 2, 3]`)
	pAst, errors := p.Parse(src)
	if len(errors.GetErrors()) > 0 {
		b.Fatalf("Parse failed: %v", errors.ToDisplayString())
	}
	reg, err := types.NewProtoRegistry()
	if err != nil {
		b.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	cont, err := containers.NewContainer()
	if err != nil {
		b.Fatalf("containers.NewContainer() failed: %v", err)
	}
	env, err := NewEnv(cont, reg)
	if err != nil {
		b.Fatalf("NewEnv failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Check(pAst, src, env)
	}
}

func testEnvs(t testing.TB) map[string]testEnv {
	return map[string]testEnv{
		"default": {
			functions: []*decls.FunctionDecl{
				testFunction(t, "fg_s",
					decls.Overload("fg_s_0", []*types.Type{}, types.StringType)),
				testFunction(t, "fi_s_s",
					decls.MemberOverload("fi_s_s_0",
						[]*types.Type{types.StringType}, types.StringType)),
			},
			idents: []*decls.VariableDecl{
				decls.NewVariable("is", types.StringType),
				decls.NewVariable("ii", types.IntType),
				decls.NewVariable("iu", types.UintType),
				decls.NewVariable("iz", types.BoolType),
				decls.NewVariable("ib", types.BytesType),
				decls.NewVariable("id", types.DoubleType),
				decls.NewVariable("ix", types.NullType),
			},
		},
	}
}

type testInfo struct {
	// in contains the expression to be parsed.
	in string

	// out contains the output.
	out string

	// outType is the expected type of the expression
	outType *types.Type

	// container is the container name to use for test.
	container string

	// env is the environment to use for testing.
	env testEnv

	// err is the expected error for negative test cases.
	err string

	// disableStdEnv indicates whether the standard functions should be disabled.
	disableStdEnv bool

	// opts is the set of checker Option flags to use when type-checking.
	opts []Option
}

type testEnv struct {
	idents         []*decls.VariableDecl
	functions      []*decls.FunctionDecl
	variadicASTs   bool
	optionalSyntax bool
	jsonFieldNames bool
}

func TestCheck(t *testing.T) {
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
	)
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}

	for i, tst := range testCases(t) {
		name := fmt.Sprintf("%d %s", i, tst.in)
		tc := tst
		t.Run(name, func(t *testing.T) {
			tcParser := p
			if tc.env.optionalSyntax || tc.env.variadicASTs {
				tcParser, err = parser.NewParser(
					parser.Macros(parser.AllMacros...),
					parser.EnableOptionalSyntax(tc.env.optionalSyntax),
					parser.EnableVariadicOperatorASTs(tc.env.variadicASTs),
				)
				if err != nil {
					t.Fatalf("parser.NewParser() failed: %v", err)
				}
			}
			// Runs the tests in parallel to ensure that there are no data races
			// due to shared mutable state across tests.
			t.Parallel()

			src := common.NewTextSource(tc.in)
			pAst, errors := tcParser.Parse(src)
			if len(errors.GetErrors()) > 0 {
				t.Fatalf("Unexpected parse errors: %v", errors.ToDisplayString())
			}

			reg, err := types.NewProtoRegistry(
				types.JSONFieldNames(tc.env.jsonFieldNames),
				types.ProtoTypeDefs(&proto2pb.TestAllTypes{}, &proto3pb.TestAllTypes{}),
			)
			if err != nil {
				t.Fatalf("types.NewProtoRegistry() failed: %v", err)
			}
			if tc.env.optionalSyntax {
				if err := reg.RegisterType(types.OptionalType); err != nil {
					t.Fatalf("reg.RegisterType(optional_type) failed: %v", err)
				}
			}
			cont, err := containers.NewContainer(containers.Name(tc.container))
			if err != nil {
				t.Fatalf("containers.NewContainer() failed: %v", err)
			}
			opts := []Option{CrossTypeNumericComparisons(true)}
			if len(tc.opts) != 0 {
				opts = tc.opts
			}
			if tc.env.jsonFieldNames {
				opts = append(opts, JSONFieldNames(true))
			}
			env, err := NewEnv(cont, reg, opts...)
			if err != nil {
				t.Fatalf("NewEnv(cont, reg) failed: %v", err)
			}
			if !tc.disableStdEnv {
				env.AddFunctions(stdlib.Functions()...)
			}
			if tc.env.idents != nil {
				for _, ident := range tc.env.idents {
					env.AddIdents(ident)
				}
			}
			if tc.env.functions != nil {
				for _, fn := range tc.env.functions {
					env.AddFunctions(fn)
				}
			}

			cAst, errors := Check(pAst, src, env)
			if len(errors.GetErrors()) > 0 {
				errorString := errors.ToDisplayString()
				if tc.err != "" {
					if !test.Compare(errorString, tc.err) {
						t.Error(test.DiffMessage("Error mismatch", errorString, tc.err))
					}
				} else {
					t.Errorf("Unexpected type-check errors: %v", errorString)
				}
			} else if tc.err != "" {
				t.Errorf("Expected error not thrown: %s", tc.err)
			}

			astIDs := cAst.IDs()
			unusedIDs := []int64{}
			for id := range cAst.SourceInfo().OffsetRanges() {
				if !astIDs[id] {
					unusedIDs = append(unusedIDs, id)
				}
			}
			if len(unusedIDs) > 0 {
				t.Errorf("SourceInfo has offset range for IDs %v, but no such nodes exists in AST: %s",
					unusedIDs, debug.ToDebugStringWithIDs(cAst.Expr()))
			}

			actual := cAst.GetType(pAst.Expr().ID())
			if tc.err == "" {
				if actual == nil || !actual.IsEquivalentType(tc.outType) {
					t.Error(test.DiffMessage("Type Error", actual, tc.outType))
				}
			}

			if tc.out != "" {
				actualStr := Print(cAst.Expr(), cAst)
				if !test.Compare(actualStr, tc.out) {
					t.Error(test.DiffMessage("Structure error", actualStr, tc.out))
				}
			}
		})
	}
}

func BenchmarkCheck(b *testing.B) {
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
	)
	if err != nil {
		b.Fatalf("parser.NewParser() failed: %v", err)
	}

	for i, tst := range testCases(b) {
		name := fmt.Sprintf("%d %s", i, tst.in)
		tc := tst
		b.Run(name, func(b *testing.B) {
			tcParser := p
			if tc.env.optionalSyntax || tc.env.variadicASTs {
				tcParser, err = parser.NewParser(
					parser.Macros(parser.AllMacros...),
					parser.EnableOptionalSyntax(tc.env.optionalSyntax),
					parser.EnableVariadicOperatorASTs(tc.env.variadicASTs),
				)
				if err != nil {
					b.Fatalf("parser.NewParser() failed: %v", err)
				}
			}
			src := common.NewTextSource(tc.in)
			pAst, errors := tcParser.Parse(src)
			if len(errors.GetErrors()) > 0 {
				b.Fatalf("Unexpected parse errors: %v", errors.ToDisplayString())
			}
			reg, err := types.NewProtoRegistry(
				types.JSONFieldNames(tc.env.jsonFieldNames),
				types.ProtoTypeDefs(&proto2pb.TestAllTypes{}, &proto3pb.TestAllTypes{}),
			)
			if err != nil {
				b.Fatalf("types.NewProtoRegistry() failed: %v", err)
			}
			if tc.env.optionalSyntax {
				if err := reg.RegisterType(types.OptionalType); err != nil {
					b.Fatalf("reg.RegisterType(optional_type) failed: %v", err)
				}
			}
			cont, err := containers.NewContainer(containers.Name(tc.container))
			if err != nil {
				b.Fatalf("containers.NewContainer() failed: %v", err)
			}
			opts := []Option{CrossTypeNumericComparisons(true)}
			if len(tc.opts) != 0 {
				opts = tc.opts
			}
			if tc.env.jsonFieldNames {
				opts = append(opts, JSONFieldNames(true))
			}
			env, err := NewEnv(cont, reg, opts...)
			if err != nil {
				b.Fatalf("NewEnv(cont, reg) failed: %v", err)
			}
			if !tc.disableStdEnv {
				env.AddFunctions(stdlib.Functions()...)
			}
			if tc.env.idents != nil {
				for _, ident := range tc.env.idents {
					env.AddIdents(ident)
				}
			}
			if tc.env.functions != nil {
				for _, fn := range tc.env.functions {
					env.AddFunctions(fn)
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cAst, errors := Check(pAst, src, env)
				if len(errors.GetErrors()) > 0 {
					errorString := errors.ToDisplayString()
					if tc.err != "" {
						if !test.Compare(errorString, tc.err) {
							b.Error(test.DiffMessage("Error mismatch", errorString, tc.err))
						}
					} else {
						b.Errorf("Unexpected type-check errors: %v", errorString)
					}
				} else if tc.err != "" {
					b.Errorf("Expected error not thrown: %s", tc.err)
				}

				actual := cAst.GetType(pAst.Expr().ID())
				if tc.err == "" {
					if actual == nil || !actual.IsEquivalentType(tc.outType) {
						b.Error(test.DiffMessage("Type Error", actual, tc.outType))
					}
				}

				if tc.out != "" {
					actualStr := Print(cAst.Expr(), cAst)
					if !test.Compare(actualStr, tc.out) {
						b.Error(test.DiffMessage("Structure error", actualStr, tc.out))
					}
				}
			}
		})
	}
}

func TestAddDuplicateDeclarations(t *testing.T) {
	reg, err := types.NewProtoRegistry(types.ProtoTypeDefs(&proto2pb.TestAllTypes{}, &proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	env, err := NewEnv(containers.DefaultContainer, reg, CrossTypeNumericComparisons(true))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	err = env.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Fatalf("env.Add() failed: %v", err)
	}
	err = env.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Errorf("env.Add() failed with duplicate declarations: %v", err)
	}
}

func TestAddEquivalentDeclarations(t *testing.T) {
	reg, err := types.NewProtoRegistry(types.ProtoTypeDefs(&proto2pb.TestAllTypes{}, &proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	env, err := NewEnv(containers.DefaultContainer, reg, CrossTypeNumericComparisons(true))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	optIndex := testFunction(t, "optional_index",
		decls.Overload("optional_map_key_value",
			[]*types.Type{
				types.NewMapType(types.NewTypeParamType("K"), types.NewTypeParamType("V")),
				types.NewTypeParamType("K")},
			types.NewOptionalType(types.NewTypeParamType("V"))))
	optIndexEquiv := testFunction(t, "optional_index",
		decls.Overload("optional_map_key_value",
			[]*types.Type{
				types.NewMapType(types.NewTypeParamType("K"), types.NewTypeParamType("V")),
				types.NewTypeParamType("K")},
			types.NewOptionalType(types.NewTypeParamType("V"))))
	err = env.AddFunctions(optIndex)
	if err != nil {
		t.Fatalf("env.Add(optIndex) failed: %v", err)
	}
	err = env.AddFunctions(optIndexEquiv)
	if err != nil {
		t.Errorf("env.Add(optIndexEquiv) failed: %v", err)
	}
}

func TestCheckErrorData(t *testing.T) {
	p, err := parser.NewParser(
		parser.EnableOptionalSyntax(true),
		parser.Macros(parser.AllMacros...),
	)
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	src := common.NewTextSource(`a || true`)
	ast, iss := p.Parse(src)
	if len(iss.GetErrors()) != 0 {
		t.Fatalf("Parse() failed: %v", iss.ToDisplayString())
	}

	reg := newTestRegistry(t)
	env, err := NewEnv(containers.DefaultContainer, reg)
	if err != nil {
		t.Fatalf("NewEnv(cont, reg) failed: %v", err)
	}
	env.AddFunctions(stdlib.Functions()...)
	_, iss = Check(ast, src, env)
	if len(iss.GetErrors()) != 1 {
		t.Fatalf("Check() of a bad expression did produce a single error: %v", iss.ToDisplayString())
	}
	celErr := iss.GetErrors()[0]
	if celErr.ExprID != 1 {
		t.Errorf("got exprID %v, wanted 1", celErr.ExprID)
	}
	if !strings.Contains(celErr.Message, "undeclared reference") {
		t.Errorf("got message %v, wanted undeclared reference", celErr.Message)
	}
}

func TestCheckInvalidOptSelectMember(t *testing.T) {
	fac := ast.NewExprFactory()
	target := fac.NewStruct(1, "Foo", nil)
	arg1 := fac.NewStruct(2, "Foo", nil)
	arg2 := fac.NewLiteral(3, types.String("field"))
	call := fac.NewMemberCall(4, "_?._", target, arg1, arg2)

	// This is not valid syntax, just for illustration purposes.
	src := common.NewTextSource("Foo{}._?._(Foo{}, 'field')")
	parsed := ast.NewAST(call, ast.NewSourceInfo(src))
	reg := newTestRegistry(t)
	env, err := NewEnv(containers.DefaultContainer, reg)
	if err != nil {
		t.Fatalf("NewEnv(cont, reg) failed: %v", err)
	}
	_, iss := Check(parsed, src, env)
	if !strings.Contains(iss.ToDisplayString(), "incorrect signature. member call") {
		t.Errorf("got %s, wanted 'incorrect signature. member call'", iss.ToDisplayString())
	}
}

func TestCheckInvalidOptSelectMissingArg(t *testing.T) {
	fac := ast.NewExprFactory()
	arg1 := fac.NewStruct(1, "Foo", nil)
	call := fac.NewCall(2, "_?._", arg1)

	// This is not valid syntax, just for illustration purposes.
	src := common.NewTextSource("_?._(Foo{})")
	parsed := ast.NewAST(call, ast.NewSourceInfo(src))
	reg := newTestRegistry(t)
	env, err := NewEnv(containers.DefaultContainer, reg)
	if err != nil {
		t.Fatalf("NewEnv(cont, reg) failed: %v", err)
	}
	_, iss := Check(parsed, src, env)
	if !strings.Contains(iss.ToDisplayString(), "incorrect signature. argument count: 1") {
		t.Errorf("got %s, wanted 'incorrect signature. argument count: 1'", iss.ToDisplayString())
	}
}

func TestCheckInvalidLiteral(t *testing.T) {
	fac := ast.NewExprFactory()
	durLiteral := fac.NewLiteral(1, types.Duration{Duration: time.Second})
	// This is not valid syntax, just for illustration purposes.
	src := common.NewTextSource(`1s`)
	parsed := ast.NewAST(durLiteral, ast.NewSourceInfo(src))
	reg := newTestRegistry(t)
	env, err := NewEnv(containers.DefaultContainer, reg)
	if err != nil {
		t.Fatalf("NewEnv(cont, reg) failed: %v", err)
	}
	_, iss := Check(parsed, src, env)
	if !strings.Contains(iss.ToDisplayString(), "unexpected literal type") {
		t.Errorf("got %s, wanted 'unexpected literal type'", iss.ToDisplayString())
	}
}

func testFunction(t testing.TB, name string, opts ...decls.FunctionOpt) *decls.FunctionDecl {
	t.Helper()
	fn, err := decls.NewFunction(name, opts...)
	if err != nil {
		t.Fatalf("decls.NewFunction(%s) failed: %v", name, err)
	}
	return fn
}
