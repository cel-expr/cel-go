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

package parser

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/debug"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/test"
)

var testCases = []testInfo{
	{
		I: `"A"`,
		P: `"A"^#1:*expr.Constant_StringValue#`,
	},
	{
		I: `true`,
		P: `true^#1:*expr.Constant_BoolValue#`,
	},
	{
		I: `false`,
		P: `false^#1:*expr.Constant_BoolValue#`,
	},
	{
		I: `0`,
		P: `0^#1:*expr.Constant_Int64Value#`,
	},
	{
		I: `42`,
		P: `42^#1:*expr.Constant_Int64Value#`,
	},
	{
		I: `0xF`,
		P: `15^#1:*expr.Constant_Int64Value#`,
	},
	{
		I: `0u`,
		P: `0u^#1:*expr.Constant_Uint64Value#`,
	},
	{
		I: `23u`,
		P: `23u^#1:*expr.Constant_Uint64Value#`,
	},
	{
		I: `24u`,
		P: `24u^#1:*expr.Constant_Uint64Value#`,
	},
	{
		I: `0xFu`,
		P: `15u^#1:*expr.Constant_Uint64Value#`,
	},
	{
		I: `-1`,
		P: `-1^#1:*expr.Constant_Int64Value#`,
	},
	{
		I: `4--4`,
		P: `_-_(
			4^#1:*expr.Constant_Int64Value#,
			-4^#3:*expr.Constant_Int64Value#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `4--4.1`,
		P: `_-_(
			4^#1:*expr.Constant_Int64Value#,
			-4.1^#3:*expr.Constant_DoubleValue#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `b"abc"`,
		P: `b"abc"^#1:*expr.Constant_BytesValue#`,
	},
	{
		I: `23.39`,
		P: `23.39^#1:*expr.Constant_DoubleValue#`,
	},
	{
		I: `!a`,
		P: `!_(
			a^#2:*expr.Expr_IdentExpr#
		)^#1:*expr.Expr_CallExpr#`,
	},
	{
		I: `null`,
		P: `null^#1:*expr.Constant_NullValue#`,
	},
	{
		I: `a`,
		P: `a^#1:*expr.Expr_IdentExpr#`,
	},
	{
		I: `a?b:c`,
		P: `_?_:_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#,
			c^#4:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a || b`,
		P: `_||_(
    		  a^#1:*expr.Expr_IdentExpr#,
    		  b^#2:*expr.Expr_IdentExpr#
			)^#3:*expr.Expr_CallExpr#`,
	},
	{
		I: `a || b || c || d || e || f `,
		P: ` _||_(
			_||_(
			  _||_(
				a^#1:*expr.Expr_IdentExpr#,
				b^#2:*expr.Expr_IdentExpr#
			  )^#3:*expr.Expr_CallExpr#,
			  c^#4:*expr.Expr_IdentExpr#
			)^#5:*expr.Expr_CallExpr#,
			_||_(
			  _||_(
				d^#6:*expr.Expr_IdentExpr#,
				e^#8:*expr.Expr_IdentExpr#
			  )^#9:*expr.Expr_CallExpr#,
			  f^#10:*expr.Expr_IdentExpr#
			)^#11:*expr.Expr_CallExpr#
		  )^#7:*expr.Expr_CallExpr#`,
	},
	{
		I: `a && b`,
		P: `_&&_(
    		  a^#1:*expr.Expr_IdentExpr#,
    		  b^#2:*expr.Expr_IdentExpr#
			)^#3:*expr.Expr_CallExpr#`,
	},
	{
		I: `a && b && c && d && e && f && g`,
		P: `_&&_(
			_&&_(
			  _&&_(
				a^#1:*expr.Expr_IdentExpr#,
				b^#2:*expr.Expr_IdentExpr#
			  )^#3:*expr.Expr_CallExpr#,
			  _&&_(
				c^#4:*expr.Expr_IdentExpr#,
				d^#6:*expr.Expr_IdentExpr#
			  )^#7:*expr.Expr_CallExpr#
			)^#5:*expr.Expr_CallExpr#,
			_&&_(
			  _&&_(
				e^#8:*expr.Expr_IdentExpr#,
				f^#10:*expr.Expr_IdentExpr#
			  )^#11:*expr.Expr_CallExpr#,
			  g^#12:*expr.Expr_IdentExpr#
			)^#13:*expr.Expr_CallExpr#
		  )^#9:*expr.Expr_CallExpr#`,
	},
	{
		I: `a && b && c && d || e && f && g && h`,
		P: `_||_(
			_&&_(
			  _&&_(
				a^#1:*expr.Expr_IdentExpr#,
				b^#2:*expr.Expr_IdentExpr#
			  )^#3:*expr.Expr_CallExpr#,
			  _&&_(
				c^#4:*expr.Expr_IdentExpr#,
				d^#6:*expr.Expr_IdentExpr#
			  )^#7:*expr.Expr_CallExpr#
			)^#5:*expr.Expr_CallExpr#,
			_&&_(
			  _&&_(
				e^#8:*expr.Expr_IdentExpr#,
				f^#9:*expr.Expr_IdentExpr#
			  )^#10:*expr.Expr_CallExpr#,
			  _&&_(
				g^#11:*expr.Expr_IdentExpr#,
				h^#13:*expr.Expr_IdentExpr#
			  )^#14:*expr.Expr_CallExpr#
			)^#12:*expr.Expr_CallExpr#
		  )^#15:*expr.Expr_CallExpr#`,
	},
	{
		I: `a + b`,
		P: `_+_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a - b`,
		P: `_-_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a * b`,
		P: `_*_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a / b`,
		P: `_/_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a % b`,
		P: `_%_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a in b`,
		P: `@in(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a == b`,
		P: `_==_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a != b`,
		P: ` _!=_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a > b`,
		P: `_>_(
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a >= b`,
		P: `_>=_(
    		  a^#1:*expr.Expr_IdentExpr#,
    		  b^#3:*expr.Expr_IdentExpr#
			)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a < b`,
		P: `_<_(
    		  a^#1:*expr.Expr_IdentExpr#,
    		  b^#3:*expr.Expr_IdentExpr#
			)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a <= b`,
		P: `_<=_(
    		  a^#1:*expr.Expr_IdentExpr#,
    		  b^#3:*expr.Expr_IdentExpr#
			)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a.b`,
		P: `a^#1:*expr.Expr_IdentExpr#.b^#2:*expr.Expr_SelectExpr#`,
	},
	{
		I: `a.b.c`,
		P: `a^#1:*expr.Expr_IdentExpr#.b^#2:*expr.Expr_SelectExpr#.c^#3:*expr.Expr_SelectExpr#`,
	},
	{
		I: `a[b]`,
		P: `_[_](
			a^#1:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `foo{ }`,
		P: `foo{}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `foo{ a:b }`,
		P: `foo{
			a:b^#3:*expr.Expr_IdentExpr#^#2:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `foo{ a:b, c:d }`,
		P: `foo{
			a:b^#3:*expr.Expr_IdentExpr#^#2:*expr.Expr_CreateStruct_Entry#,
			c:d^#5:*expr.Expr_IdentExpr#^#4:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `{}`,
		P: `{}^#1:*expr.Expr_StructExpr#`,
	},

	{
		I: `{a:b, c:d}`,
		P: `{
			a^#3:*expr.Expr_IdentExpr#:b^#4:*expr.Expr_IdentExpr#^#2:*expr.Expr_CreateStruct_Entry#,
			c^#6:*expr.Expr_IdentExpr#:d^#7:*expr.Expr_IdentExpr#^#5:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
		PrattP: `{
			a^#2:*expr.Expr_IdentExpr#:b^#4:*expr.Expr_IdentExpr#^#3:*expr.Expr_CreateStruct_Entry#,
			c^#5:*expr.Expr_IdentExpr#:d^#7:*expr.Expr_IdentExpr#^#6:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `[]`,
		P: `[]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I: `[a]`,
		P: `[
			a^#2:*expr.Expr_IdentExpr#
		]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I: `[a, b, c]`,
		P: `[
			a^#2:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#,
			c^#4:*expr.Expr_IdentExpr#
		]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I: `(a)`,
		P: `a^#1:*expr.Expr_IdentExpr#`,
	},
	{
		I: `((a))`,
		P: `a^#1:*expr.Expr_IdentExpr#`,
	},
	{
		I: `a()`,
		P: `a()^#1:*expr.Expr_CallExpr#`,
	},

	{
		I: `a(b)`,
		P: `a(
			b^#2:*expr.Expr_IdentExpr#
		)^#1:*expr.Expr_CallExpr#`,
	},

	{
		I: `a(b, c)`,
		P: `a(
			b^#2:*expr.Expr_IdentExpr#,
			c^#3:*expr.Expr_IdentExpr#
		)^#1:*expr.Expr_CallExpr#`,
	},
	{
		I: `a.b()`,
		P: `a^#1:*expr.Expr_IdentExpr#.b()^#2:*expr.Expr_CallExpr#`,
	},

	{
		I: `a.b(c)`,
		P: `a^#1:*expr.Expr_IdentExpr#.b(
			c^#3:*expr.Expr_IdentExpr#
		)^#2:*expr.Expr_CallExpr#`,
		L: `a^#1[1,0]#.b(
    		  c^#3[1,4]#
    		)^#2[1,3]#`,
	},

	// Parse error tests
	{
		I: `0xFFFFFFFFFFFFFFFFF`,
		E: `ERROR: <input>:1:1: invalid int literal
		| 0xFFFFFFFFFFFFFFFFF
		| ^`,
	},
	{
		I: `0xFFFFFFFFFFFFFFFFFu`,
		E: `ERROR: <input>:1:1: invalid uint literal
		| 0xFFFFFFFFFFFFFFFFFu
		| ^`,
	},
	{
		I: `1.99e90000009`,
		E: `ERROR: <input>:1:1: invalid double literal
		| 1.99e90000009
		| ^`,
	},
	{
		I: `*@a | b`,
		E: `ERROR: <input>:1:1: Syntax error: extraneous input '*' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| *@a | b
		| ^
		ERROR: <input>:1:2: Syntax error: token recognition error at: '@'
		| *@a | b
		| .^
		ERROR: <input>:1:5: Syntax error: token recognition error at: '| '
		| *@a | b
		| ....^
		ERROR: <input>:1:7: Syntax error: extraneous input 'b' expecting <EOF>
		| *@a | b
		| ......^`,
		PrattE: `ERROR: <input>:1:1: unexpected token
		| *@a | b
		| ^
		ERROR: <input>:1:2: unexpected character
		| *@a | b
		| .^
		ERROR: <input>:1:5: unexpected single '|', expected '||'
		| *@a | b
		| ....^`,
	},
	{
		I: `a | b`,
		E: `ERROR: <input>:1:3: Syntax error: token recognition error at: '| '
		| a | b
		| ..^
		ERROR: <input>:1:5: Syntax error: extraneous input 'b' expecting <EOF>
		| a | b
		| ....^`,
		PrattE: `ERROR: <input>:1:3: unexpected single '|', expected '||'
		| a | b
		| ..^`,
	},

	// Macro tests
	{
		I: `has(m.f)`,
		P: `m^#2:*expr.Expr_IdentExpr#.f~test-only~^#4:*expr.Expr_SelectExpr#`,
		L: `m^#2[1,4]#.f~test-only~^#4[1,3]#`,
		M: `has(
			m^#2:*expr.Expr_IdentExpr#.f^#3:*expr.Expr_SelectExpr#
		  )^#4:has#`,
	},
	{
		I: `has(m)`,
		E: `ERROR: <input>:1:5: invalid argument to has() macro
             | has(m)
             | ....^`,
	},

	{
		I: `m.exists(v, f)`,
		P: `__comprehension__(
			// Variable
			v,
			// Target
			m^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			false^#5:*expr.Constant_BoolValue#,
			// LoopCondition
			@not_strictly_false(
                !_(
                  @result^#6:*expr.Expr_IdentExpr#
                )^#7:*expr.Expr_CallExpr#
			)^#8:*expr.Expr_CallExpr#,
			// LoopStep
			_||_(
                @result^#9:*expr.Expr_IdentExpr#,
                f^#4:*expr.Expr_IdentExpr#
			)^#10:*expr.Expr_CallExpr#,
			// Result
			@result^#11:*expr.Expr_IdentExpr#)^#12:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.exists(
			v^#3:*expr.Expr_IdentExpr#,
			f^#4:*expr.Expr_IdentExpr#
		  	)^#12:exists#`,
	},
	{
		I: `m.all(v, f)`,
		P: `__comprehension__(
			// Variable
			v,
			// Target
			m^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			true^#5:*expr.Constant_BoolValue#,
			// LoopCondition
			@not_strictly_false(
                @result^#6:*expr.Expr_IdentExpr#
            )^#7:*expr.Expr_CallExpr#,
			// LoopStep
			_&&_(
                @result^#8:*expr.Expr_IdentExpr#,
                f^#4:*expr.Expr_IdentExpr#
            )^#9:*expr.Expr_CallExpr#,
			// Result
			@result^#10:*expr.Expr_IdentExpr#)^#11:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.all(
			v^#3:*expr.Expr_IdentExpr#,
			f^#4:*expr.Expr_IdentExpr#
		  	)^#11:all#`,
	},
	{
		I: `m.existsOne(v, f)`,
		P: `__comprehension__(
			// Variable
			v,
			// Target
			m^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			0^#5:*expr.Constant_Int64Value#,
			// LoopCondition
			true^#6:*expr.Constant_BoolValue#,
			// LoopStep
			_?_:_(
				f^#4:*expr.Expr_IdentExpr#,
				_+_(
					  @result^#7:*expr.Expr_IdentExpr#,
				  1^#8:*expr.Constant_Int64Value#
				)^#9:*expr.Expr_CallExpr#,
				@result^#10:*expr.Expr_IdentExpr#
			)^#11:*expr.Expr_CallExpr#,
			// Result
			_==_(
				@result^#12:*expr.Expr_IdentExpr#,
				1^#13:*expr.Constant_Int64Value#
			)^#14:*expr.Expr_CallExpr#)^#15:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.existsOne(
			v^#3:*expr.Expr_IdentExpr#,
			f^#4:*expr.Expr_IdentExpr#
		  	)^#15:existsOne#`,
	},
	{
		I: `[].existsOne(__result__, __result__)`,
		E: `ERROR: <input>:1:14: iteration variable overwrites accumulator variable
             | [].existsOne(__result__, __result__)
             | .............^`,
	},
	{
		I: `m.map(v, f)`,
		P: `__comprehension__(
			// Variable
			v,
			// Target
			m^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			[]^#5:*expr.Expr_ListExpr#,
			// LoopCondition
			true^#6:*expr.Constant_BoolValue#,
			// LoopStep
			_+_(
				@result^#7:*expr.Expr_IdentExpr#,
				[
					f^#4:*expr.Expr_IdentExpr#
				]^#8:*expr.Expr_ListExpr#
			)^#9:*expr.Expr_CallExpr#,
			// Result
			@result^#10:*expr.Expr_IdentExpr#)^#11:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.map(
			v^#3:*expr.Expr_IdentExpr#,
			f^#4:*expr.Expr_IdentExpr#
		  	)^#11:map#`,
	},
	{
		I: `m.map(__result__, __result__)`,
		E: `ERROR: <input>:1:7: iteration variable overwrites accumulator variable
             | m.map(__result__, __result__)
             | ......^`,
	},
	{
		I: `m.map(v, p, f)`,
		P: `__comprehension__(
			// Variable
			v,
			// Target
			m^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			[]^#6:*expr.Expr_ListExpr#,
			// LoopCondition
			true^#7:*expr.Constant_BoolValue#,
			// LoopStep
			_?_:_(
				p^#4:*expr.Expr_IdentExpr#,
				_+_(
					@result^#8:*expr.Expr_IdentExpr#,
					[
						f^#5:*expr.Expr_IdentExpr#
					]^#9:*expr.Expr_ListExpr#
				)^#10:*expr.Expr_CallExpr#,
				@result^#11:*expr.Expr_IdentExpr#
			)^#12:*expr.Expr_CallExpr#,
			// Result
			@result^#13:*expr.Expr_IdentExpr#)^#14:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.map(
			v^#3:*expr.Expr_IdentExpr#,
			p^#4:*expr.Expr_IdentExpr#,
			f^#5:*expr.Expr_IdentExpr#
		  	)^#14:map#`,
	},

	{
		I: `m.filter(v, p)`,
		P: `__comprehension__(
			// Variable
			v,
			// Target
			m^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			[]^#5:*expr.Expr_ListExpr#,
			// LoopCondition
			true^#6:*expr.Constant_BoolValue#,
			// LoopStep
			_?_:_(
				p^#4:*expr.Expr_IdentExpr#,
				_+_(
					@result^#7:*expr.Expr_IdentExpr#,
					[
						v^#3:*expr.Expr_IdentExpr#
					]^#8:*expr.Expr_ListExpr#
				)^#9:*expr.Expr_CallExpr#,
				@result^#10:*expr.Expr_IdentExpr#
			)^#11:*expr.Expr_CallExpr#,
			// Result
			@result^#12:*expr.Expr_IdentExpr#)^#13:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.filter(
			v^#3:*expr.Expr_IdentExpr#,
			p^#4:*expr.Expr_IdentExpr#
		  	)^#13:filter#`,
	},
	{
		I: `m.filter(__result__, false)`,
		E: `ERROR: <input>:1:10: iteration variable overwrites accumulator variable
             | m.filter(__result__, false)
             | .........^`,
	},
	{
		I: `m.filter(a.b, false)`,
		E: `ERROR: <input>:1:11: argument is not an identifier
             | m.filter(a.b, false)
             | ..........^`,
	},

	// Tests from C++ parser
	{
		I: "x * 2",
		P: `_*_(
			x^#1:*expr.Expr_IdentExpr#,
			2^#3:*expr.Constant_Int64Value#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: "x * 2u",
		P: `_*_(
			x^#1:*expr.Expr_IdentExpr#,
			2u^#3:*expr.Constant_Uint64Value#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: "x * 2.0",
		P: `_*_(
			x^#1:*expr.Expr_IdentExpr#,
			2^#3:*expr.Constant_DoubleValue#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `"\u2764"`,
		P: "\"\u2764\"^#1:*expr.Constant_StringValue#",
	},
	{
		I: "\"\u2764\"",
		P: "\"\u2764\"^#1:*expr.Constant_StringValue#",
	},
	{
		I: `! false`,
		P: `!_(
			false^#2:*expr.Constant_BoolValue#
		)^#1:*expr.Expr_CallExpr#`,
	},
	{
		I: `-a`,
		P: `-_(
			a^#2:*expr.Expr_IdentExpr#
		)^#1:*expr.Expr_CallExpr#`,
	},
	{
		I: `a.b(5)`,
		P: `a^#1:*expr.Expr_IdentExpr#.b(
			5^#3:*expr.Constant_Int64Value#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `a[3]`,
		P: `_[_](
			a^#1:*expr.Expr_IdentExpr#,
			3^#3:*expr.Constant_Int64Value#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `SomeMessage{foo: 5, bar: "xyz"}`,
		P: `SomeMessage{
			foo:5^#3:*expr.Constant_Int64Value#^#2:*expr.Expr_CreateStruct_Entry#,
			bar:"xyz"^#5:*expr.Constant_StringValue#^#4:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `[3, 4, 5]`,
		P: `[
			3^#2:*expr.Constant_Int64Value#,
			4^#3:*expr.Constant_Int64Value#,
			5^#4:*expr.Constant_Int64Value#
		]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I: `[3, 4, 5,]`,
		P: `[
			3^#2:*expr.Constant_Int64Value#,
			4^#3:*expr.Constant_Int64Value#,
			5^#4:*expr.Constant_Int64Value#
		]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I: `{foo: 5, bar: "xyz"}`,
		P: `{
			foo^#3:*expr.Expr_IdentExpr#:5^#4:*expr.Constant_Int64Value#^#2:*expr.Expr_CreateStruct_Entry#,
			bar^#6:*expr.Expr_IdentExpr#:"xyz"^#7:*expr.Constant_StringValue#^#5:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
		PrattP: `{
			foo^#2:*expr.Expr_IdentExpr#:5^#4:*expr.Constant_Int64Value#^#3:*expr.Expr_CreateStruct_Entry#,
			bar^#5:*expr.Expr_IdentExpr#:"xyz"^#7:*expr.Constant_StringValue#^#6:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `{foo: 5, bar: "xyz", }`,
		P: `{
			foo^#3:*expr.Expr_IdentExpr#:5^#4:*expr.Constant_Int64Value#^#2:*expr.Expr_CreateStruct_Entry#,
			bar^#6:*expr.Expr_IdentExpr#:"xyz"^#7:*expr.Constant_StringValue#^#5:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
		PrattP: `{
			foo^#2:*expr.Expr_IdentExpr#:5^#4:*expr.Constant_Int64Value#^#3:*expr.Expr_CreateStruct_Entry#,
			bar^#5:*expr.Expr_IdentExpr#:"xyz"^#7:*expr.Constant_StringValue#^#6:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `a > 5 && a < 10`,
		P: `_&&_(
			_>_(
			  a^#1:*expr.Expr_IdentExpr#,
			  5^#3:*expr.Constant_Int64Value#
			)^#2:*expr.Expr_CallExpr#,
			_<_(
			  a^#4:*expr.Expr_IdentExpr#,
			  10^#6:*expr.Constant_Int64Value#
			)^#5:*expr.Expr_CallExpr#
		)^#7:*expr.Expr_CallExpr#`,
	},
	{
		I: `a < 5 || a > 10`,
		P: `_||_(
			_<_(
			  a^#1:*expr.Expr_IdentExpr#,
			  5^#3:*expr.Constant_Int64Value#
			)^#2:*expr.Expr_CallExpr#,
			_>_(
			  a^#4:*expr.Expr_IdentExpr#,
			  10^#6:*expr.Constant_Int64Value#
			)^#5:*expr.Expr_CallExpr#
		)^#7:*expr.Expr_CallExpr#`,
	},
	{
		I: `{`,
		E: `ERROR: <input>:1:2: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '}', '(', '.', ',', '-', '!', '?', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		 | {
		 | .^`,
		PrattE: `ERROR: <input>:1:2: expected '}'
		 | {
		 | .^`,
	},

	// Tests from Java parser
	{
		I: `[] + [1,2,3,] + [4]`,
		P: `_+_(
			_+_(
				[]^#1:*expr.Expr_ListExpr#,
				[
					1^#4:*expr.Constant_Int64Value#,
					2^#5:*expr.Constant_Int64Value#,
					3^#6:*expr.Constant_Int64Value#
				]^#3:*expr.Expr_ListExpr#
			)^#2:*expr.Expr_CallExpr#,
			[
				4^#9:*expr.Constant_Int64Value#
			]^#8:*expr.Expr_ListExpr#
		)^#7:*expr.Expr_CallExpr#`,
	},
	{
		I: `{1:2u, 2:3u}`,
		P: `{
			1^#3:*expr.Constant_Int64Value#:2u^#4:*expr.Constant_Uint64Value#^#2:*expr.Expr_CreateStruct_Entry#,
			2^#6:*expr.Constant_Int64Value#:3u^#7:*expr.Constant_Uint64Value#^#5:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
		PrattP: `{
			1^#2:*expr.Constant_Int64Value#:2u^#4:*expr.Constant_Uint64Value#^#3:*expr.Expr_CreateStruct_Entry#,
			2^#5:*expr.Constant_Int64Value#:3u^#7:*expr.Constant_Uint64Value#^#6:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `TestAllTypes{single_int32: 1, single_int64: 2}`,
		P: `TestAllTypes{
			single_int32:1^#3:*expr.Constant_Int64Value#^#2:*expr.Expr_CreateStruct_Entry#,
			single_int64:2^#5:*expr.Constant_Int64Value#^#4:*expr.Expr_CreateStruct_Entry#
		}^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `TestAllTypes(){}`,
		E: `ERROR: <input>:1:15: Syntax error: mismatched input '{' expecting <EOF>
		| TestAllTypes(){}
		| ..............^`,
	},
	{
		I: `TestAllTypes{}()`,
		E: `ERROR: <input>:1:15: Syntax error: mismatched input '(' expecting <EOF>
		| TestAllTypes{}()
		| ..............^`,
	},
	{
		I: `size(x) == x.size()`,
		P: `_==_(
			size(
				x^#2:*expr.Expr_IdentExpr#
			)^#1:*expr.Expr_CallExpr#,
			x^#4:*expr.Expr_IdentExpr#.size()^#5:*expr.Expr_CallExpr#
		)^#3:*expr.Expr_CallExpr#`,
	},
	{
		I: `1 + $`,
		E: `ERROR: <input>:1:5: Syntax error: token recognition error at: '$'
		| 1 + $
		| ....^
		ERROR: <input>:1:6: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| 1 + $
		| .....^`,
		PrattE: `ERROR: <input>:1:5: unexpected character
		| 1 + $
		| ....^`,
	},
	{
		I: `1 + 2
3 +`,
		E: `ERROR: <input>:2:1: Syntax error: mismatched input '3' expecting <EOF>
		| 3 +
		| ^`,
	},
	{
		I: `"\""`,
		P: `"\""^#1:*expr.Constant_StringValue#`,
	},
	{
		I: `[1,3,4][0]`,
		P: `_[_](
			[
				1^#2:*expr.Constant_Int64Value#,
				3^#3:*expr.Constant_Int64Value#,
				4^#4:*expr.Constant_Int64Value#
			]^#1:*expr.Expr_ListExpr#,
			0^#6:*expr.Constant_Int64Value#
		)^#5:*expr.Expr_CallExpr#`,
	},
	{
		I: `1.all(2, 3)`,
		E: `ERROR: <input>:1:7: argument must be a simple name
		| 1.all(2, 3)
		| ......^`,
	},
	{
		I: `x["a"].single_int32 == 23`,
		P: `_==_(
			_[_](
				x^#1:*expr.Expr_IdentExpr#,
				"a"^#3:*expr.Constant_StringValue#
			)^#2:*expr.Expr_CallExpr#.single_int32^#4:*expr.Expr_SelectExpr#,
			23^#6:*expr.Constant_Int64Value#
		)^#5:*expr.Expr_CallExpr#`,
	},
	{
		I: `x.single_nested_message != null`,
		P: `_!=_(
			x^#1:*expr.Expr_IdentExpr#.single_nested_message^#2:*expr.Expr_SelectExpr#,
			null^#4:*expr.Constant_NullValue#
		)^#3:*expr.Expr_CallExpr#`,
	},
	{
		I: `false && !true || false ? 2 : 3`,
		P: `_?_:_(
			_||_(
				_&&_(
					false^#1:*expr.Constant_BoolValue#,
					!_(
						true^#3:*expr.Constant_BoolValue#
					)^#2:*expr.Expr_CallExpr#
				)^#4:*expr.Expr_CallExpr#,
				false^#5:*expr.Constant_BoolValue#
			)^#6:*expr.Expr_CallExpr#,
			2^#8:*expr.Constant_Int64Value#,
			3^#9:*expr.Constant_Int64Value#
		)^#7:*expr.Expr_CallExpr#`,
	},
	{
		I: `b"abc" + B"def"`,
		P: `_+_(
			b"abc"^#1:*expr.Constant_BytesValue#,
			b"def"^#3:*expr.Constant_BytesValue#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `1 + 2 * 3 - 1 / 2 == 6 % 1`,
		P: `_==_(
			_-_(
				_+_(
					1^#1:*expr.Constant_Int64Value#,
					_*_(
						2^#3:*expr.Constant_Int64Value#,
						3^#5:*expr.Constant_Int64Value#
					)^#4:*expr.Expr_CallExpr#
				)^#2:*expr.Expr_CallExpr#,
				_/_(
					1^#7:*expr.Constant_Int64Value#,
					2^#9:*expr.Constant_Int64Value#
				)^#8:*expr.Expr_CallExpr#
			)^#6:*expr.Expr_CallExpr#,
			_%_(
				6^#11:*expr.Constant_Int64Value#,
				1^#13:*expr.Constant_Int64Value#
			)^#12:*expr.Expr_CallExpr#
		)^#10:*expr.Expr_CallExpr#`,
	},
	{
		I: `1 + +`,
		E: `ERROR: <input>:1:5: Syntax error: mismatched input '+' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| 1 + +
		| ....^
		ERROR: <input>:1:6: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| 1 + +
		| .....^`,
		PrattE: `ERROR: <input>:1:5: unexpected token
		| 1 + +
		| ....^`,
	},
	{
		I: `"abc" + "def"`,
		P: `_+_(
			"abc"^#1:*expr.Constant_StringValue#,
			"def"^#3:*expr.Constant_StringValue#
		)^#2:*expr.Expr_CallExpr#`,
	},

	{
		I: `{"a": 1}."a"`,
		E: `ERROR: <input>:1:10: Syntax error: no viable alternative at input '."a"'
		| {"a": 1}."a"
		| .........^`,
		PrattE: `ERROR: <input>:1:10: expected identifier after '.'
		| {"a": 1}."a"
		| .........^`,
	},

	{
		I: `"\xC3\XBF"`,
		P: `"Ã¿"^#1:*expr.Constant_StringValue#`,
	},

	{
		I: `"\303\277"`,
		P: `"Ã¿"^#1:*expr.Constant_StringValue#`,
	},

	{
		I: `"hi\u263A \u263Athere"`,
		P: `"hi☺ ☺there"^#1:*expr.Constant_StringValue#`,
	},

	{
		I: `"\U000003A8\?"`,
		P: `"Ψ?"^#1:*expr.Constant_StringValue#`,
	},

	{
		I: `"\a\b\f\n\r\t\v'\"\\\? Legal escapes"`,
		P: `"\a\b\f\n\r\t\v'\"\\? Legal escapes"^#1:*expr.Constant_StringValue#`,
	},

	{
		I: `"\xFh"`,
		E: `ERROR: <input>:1:1: Syntax error: token recognition error at: '"\xFh'
		| "\xFh"
		| ^
		ERROR: <input>:1:6: Syntax error: token recognition error at: '"'
		| "\xFh"
		| .....^
		ERROR: <input>:1:7: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| "\xFh"
		| ......^`,
		PrattE: `ERROR: <input>:1:1: unable to unescape string
		| "\xFh"
		| ^`,
	},

	{
		I: `"\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>"`,
		E: `ERROR: <input>:1:1: Syntax error: token recognition error at: '"\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>'
		| "\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>"
		| ^
		ERROR: <input>:1:42: Syntax error: token recognition error at: '"'
		| "\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>"
		| .........................................^
		ERROR: <input>:1:43: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| "\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>"
		| ..........................................^`,
		PrattE: `ERROR: <input>:1:1: unable to unescape string
		| "\a\b\f\n\r\t\v\'\"\\\? Illegal escape \>"
		| ^`,
	},

	{
		I: `"😁" in ["😁", "😑", "😦"]`,
		P: `@in(
			"😁"^#1:*expr.Constant_StringValue#,
			[
				"😁"^#4:*expr.Constant_StringValue#,
				"😑"^#5:*expr.Constant_StringValue#,
				"😦"^#6:*expr.Constant_StringValue#
			]^#3:*expr.Expr_ListExpr#
		)^#2:*expr.Expr_CallExpr#`,
	},
	{
		I: `      '😁' in ['😁', '😑', '😦']
			&& in.😁`,
		E: `ERROR: <input>:2:7: Syntax error: extraneous input 'in' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		|    && in.😁
		| ......^
	    ERROR: <input>:2:10: Syntax error: token recognition error at: '😁'
		|    && in.😁
		| .........＾
		ERROR: <input>:2:11: Syntax error: no viable alternative at input '.'
		|    && in.😁
		| .........．^`,
		PrattE: `ERROR: <input>:2:7: unexpected token
		|    && in.😁
		| ......^
		ERROR: <input>:2:10: unexpected character
		|    && in.😁
		| .........＾`,
	},
	{
		I: "as",
		E: `ERROR: <input>:1:1: reserved identifier: as
		| as
		| ^`,
	},
	{
		I: "break",
		E: `ERROR: <input>:1:1: reserved identifier: break
		| break
		| ^`,
	},
	{
		I: "const",
		E: `ERROR: <input>:1:1: reserved identifier: const
		| const
		| ^`,
	},
	{
		I: "continue",
		E: `ERROR: <input>:1:1: reserved identifier: continue
		| continue
		| ^`,
	},
	{
		I: "else",
		E: `ERROR: <input>:1:1: reserved identifier: else
		| else
		| ^`,
	},
	{
		I: "for",
		E: `ERROR: <input>:1:1: reserved identifier: for
		| for
		| ^`,
	},
	{
		I: "function",
		E: `ERROR: <input>:1:1: reserved identifier: function
		| function
		| ^`,
	},
	{
		I: "if",
		E: `ERROR: <input>:1:1: reserved identifier: if
		| if
		| ^`,
	},
	{
		I: "import",
		E: `ERROR: <input>:1:1: reserved identifier: import
		| import
		| ^`,
	},
	{
		I: "in",
		E: `ERROR: <input>:1:1: Syntax error: mismatched input 'in' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| in
		| ^
        ERROR: <input>:1:3: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| in
		| ..^`,
		PrattE: `ERROR: <input>:1:1: unexpected token
		| in
		| ^`,
	},
	{
		I: "let",
		E: `ERROR: <input>:1:1: reserved identifier: let
		| let
		| ^`,
	},
	{
		I: "loop",
		E: `ERROR: <input>:1:1: reserved identifier: loop
		| loop
		| ^`,
	},
	{
		I: "package",
		E: `ERROR: <input>:1:1: reserved identifier: package
		| package
		| ^`,
	},
	{
		I: "namespace",
		E: `ERROR: <input>:1:1: reserved identifier: namespace
		| namespace
		| ^`,
	},
	{
		I: "return",
		E: `ERROR: <input>:1:1: reserved identifier: return
		| return
		| ^`,
	},
	{
		I: "var",
		E: `ERROR: <input>:1:1: reserved identifier: var
		| var
		| ^`,
	},
	{
		I: "void",
		E: `ERROR: <input>:1:1: reserved identifier: void
		| void
		| ^`,
	},
	{
		I: "while",
		E: `ERROR: <input>:1:1: reserved identifier: while
		| while
		| ^`,
	},
	{
		I: "[1, 2, 3].map(var, var * var)",
		E: `ERROR: <input>:1:15: reserved identifier: var
		| [1, 2, 3].map(var, var * var)
		| ..............^
		ERROR: <input>:1:15: argument is not an identifier
		| [1, 2, 3].map(var, var * var)
		| ..............^
		ERROR: <input>:1:20: reserved identifier: var
		| [1, 2, 3].map(var, var * var)
		| ...................^
		ERROR: <input>:1:26: reserved identifier: var
		| [1, 2, 3].map(var, var * var)
		| .........................^`,
		PrattE: `ERROR: <input>:1:15: reserved identifier: var
		| [1, 2, 3].map(var, var * var)
		| ..............^
		ERROR: <input>:1:20: reserved identifier: var
		| [1, 2, 3].map(var, var * var)
		| ...................^
		ERROR: <input>:1:26: reserved identifier: var
		| [1, 2, 3].map(var, var * var)
		| .........................^`,
	},
	{
		I: "func{{a}}",
		E: `ERROR: <input>:1:6: Syntax error: extraneous input '{' expecting {'}', ',', '?', IDENTIFIER, ESC_IDENTIFIER}
		| func{{a}}
		| .....^
	    ERROR: <input>:1:8: Syntax error: mismatched input '}' expecting ':'
		| func{{a}}
		| .......^
	    ERROR: <input>:1:9: Syntax error: extraneous input '}' expecting <EOF>
		| func{{a}}
		| ........^`,
		PrattE: `ERROR: <input>:1:6: expected struct field name
		| func{{a}}
		| .....^
		ERROR: <input>:1:9: Syntax error: mismatched input '}' expecting <EOF>
		| func{{a}}
		| ........^`,
	},
	{
		I: "msg{:a}",
		E: `ERROR: <input>:1:5: Syntax error: extraneous input ':' expecting {'}', ',', '?', IDENTIFIER, ESC_IDENTIFIER}
		| msg{:a}
		| ....^
	    ERROR: <input>:1:7: Syntax error: mismatched input '}' expecting ':'
		| msg{:a}
		| ......^`,
		PrattE: `ERROR: <input>:1:5: expected struct field name
		| msg{:a}
		| ....^`,
	},
	{
		I: "{a}",
		E: `ERROR: <input>:1:3: Syntax error: mismatched input '}' expecting ':'
		| {a}
		| ..^`,
		PrattE: `ERROR: <input>:1:3: expected ':' in map entry
		| {a}
		| ..^`,
	},
	{
		I: "{:a}",
		E: `ERROR: <input>:1:2: Syntax error: extraneous input ':' expecting {'[', '{', '}', '(', '.', ',', '-', '!', '?', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| {:a}
		| .^
	    ERROR: <input>:1:4: Syntax error: mismatched input '}' expecting ':'
		| {:a}
		| ...^`,
		PrattE: `ERROR: <input>:1:2: unexpected token
		| {:a}
		| .^
		ERROR: <input>:1:3: expected ':' in map entry
		| {:a}
		| ..^`,
	},
	{
		I: "ind[a{b}]",
		E: `ERROR: <input>:1:8: Syntax error: mismatched input '}' expecting ':'
		| ind[a{b}]
		| .......^`,
		PrattE: `ERROR: <input>:1:8: expected ':' in struct field
		| ind[a{b}]
		| .......^`,
	},
	{
		I: `--`,
		E: `ERROR: <input>:1:3: Syntax error: no viable alternative at input '-'
		| --
		| ..^
	    ERROR: <input>:1:3: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| --
		| ..^`,
		PrattE: `ERROR: <input>:1:3: Syntax error: mismatched input '<EOF>' expecting expression
		| --
		| ..^`,
	},
	{
		I: `?`,
		E: `ERROR: <input>:1:1: Syntax error: mismatched input '?' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| ?
		| ^
	    ERROR: <input>:1:2: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| ?
		| .^`,
		PrattE: `ERROR: <input>:1:1: unexpected token
		| ?
		| ^`,
	},
	{
		I: `a ? b ((?))`,
		E: `ERROR: <input>:1:9: Syntax error: mismatched input '?' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| a ? b ((?))
		| ........^
	    ERROR: <input>:1:10: Syntax error: mismatched input ')' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
		| a ? b ((?))
		| .........^
	    ERROR: <input>:1:12: Syntax error: error recovery attempt limit exceeded: 4
		| a ? b ((?))
		| ...........^`,
		PrattE: `ERROR: <input>:1:9: unexpected token
		| a ? b ((?))
		| ........^
		ERROR: <input>:1:12: expected ':' in conditional expression
		| a ? b ((?))
		| ...........^`,
	},
	{
		I: `[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[
			[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[['too many']]]]]]]]]]]]]]]]]]]]]]]]]]]]
			]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]`,
		E: "ERROR: <input>:-1:0: expression recursion limit exceeded: 32",
		PrattE: `ERROR: <input>:-1:0: expression recursion limit exceeded: 32
ERROR: <input>:1:34: expected ']'
 | [[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[
 | .................................^`,
	},
	{
		I: `-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
		--3-[-1--1--1--1---1-1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1-À1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
		--1--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1
		--1--0--1--1--1--3-[-1--1--1--1---1--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1--1
		--1---1--1--1--0--1--1--1--1--0--3--1--1--0--1`,
		E: `ERROR: <input>:-1:0: expression recursion limit exceeded: 32
        ERROR: <input>:3:33: Syntax error: extraneous input '/' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
        |   --3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
        | ................................^
        ERROR: <input>:8:33: Syntax error: extraneous input '/' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
        |   --3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
        | ................................^
        ERROR: <input>:11:17: Syntax error: token recognition error at: 'À'
        |   --1--1---1--1-À1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
        | ................＾
        ERROR: <input>:14:23: Syntax error: extraneous input '/' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
        |   --1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
        | ......................^`,
		PrattE: `ERROR: <input>:3:33: unexpected token
        |   --3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
        | ................................^
        ERROR: <input>:3:34: expected ']'
        |   --3-[-1--1--1--1---1--1--1--0-/1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1
        | .................................^
        ERROR: <input>:11:17: unexpected character
        |   --1--1---1--1-À1--0--1--1--1--1--0--2--1--1--0--1--1--1--1--0--1--1--1--3-[-1--1
        | ................＾
        ERROR: <input>:34:49: expected ']'
        |   --1---1--1--1--0--1--1--1--1--0--3--1--1--0--1
        | ................................................^
        ERROR: <input>:34:49: expected ']'
        |   --1---1--1--1--0--1--1--1--1--0--3--1--1--0--1
        | ................................................^`,
	}, {
		I: `ó ¢
		ó 0 
		0"""\""\"""\""\"""\""\"""\""\"""\"\"""\""\"""\""\"""\""\"""\"!\"""\""\"""\""\"`,
		E: `ERROR: <input>:-1:0: error recovery token lookahead limit exceeded: 4
		ERROR: <input>:1:1: Syntax error: token recognition error at: 'ó'
	    | ó ¢
		| ＾
		ERROR: <input>:1:2: Syntax error: token recognition error at: ' '
		| ó ¢
		| ．＾
		ERROR: <input>:1:3: Syntax error: token recognition error at: '¢'
		| ó ¢
		| ．．＾
		ERROR: <input>:2:3: Syntax error: token recognition error at: 'ó'
		|   ó 0 
		| ..＾
		ERROR: <input>:2:4: Syntax error: token recognition error at: ' '
		|   ó 0 
		| ..．＾
		ERROR: <input>:2:6: Syntax error: token recognition error at: ' '
		|   ó 0 
		| ..．．.＾
		ERROR: <input>:3:3: Syntax error: token recognition error at: ''
		|   0"""\""\"""\""\"""\""\"""\""\"""\"\"""\""\"""\""\"""\""\"""\"!\"""\""\"""\""\"
		| ..^
		ERROR: <input>:3:4: Syntax error: mismatched input '0' expecting <EOF>
		|   0"""\""\"""\""\"""\""\"""\""\"""\"\"""\""\"""\""\"""\""\"""\"!\"""\""\"""\""\"
		| ...^
		ERROR: <input>:3:11: Syntax error: token recognition error at: '\'
		|   0"""\""\"""\""\"""\""\"""\""\"""\"\"""\""\"""\""\"""\""\"""\"!\"""\""\"""\""\"
		| ..........^`,
		PrattE: `ERROR: <input>:1:1: unexpected character
		| ó ¢
		| ＾
		ERROR: <input>:1:2: unexpected character
		| ó ¢
		| ．＾
		ERROR: <input>:1:3: unexpected character
		| ó ¢
		| ．．＾
		ERROR: <input>:2:3: unexpected character
		|   ó 0 
		| ..＾
		ERROR: <input>:2:4: unexpected character
		|   ó 0 
		| ..．＾`,
	},
	// Macro Calls Tests
	{
		I: `x.filter(y, y.filter(z, z > 0))`,
		P: `__comprehension__(
			// Variable
			y,
			// Target
			x^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			[]^#19:*expr.Expr_ListExpr#,
			// LoopCondition
			true^#20:*expr.Constant_BoolValue#,
			// LoopStep
			_?_:_(
			  __comprehension__(
				// Variable
				z,
				// Target
				y^#4:*expr.Expr_IdentExpr#,
				// Accumulator
				@result,
				// Init
				[]^#10:*expr.Expr_ListExpr#,
				// LoopCondition
				true^#11:*expr.Constant_BoolValue#,
				// LoopStep
				_?_:_(
				  _>_(
					z^#7:*expr.Expr_IdentExpr#,
					0^#9:*expr.Constant_Int64Value#
				  )^#8:*expr.Expr_CallExpr#,
				  _+_(
					@result^#12:*expr.Expr_IdentExpr#,
					[
					  z^#6:*expr.Expr_IdentExpr#
					]^#13:*expr.Expr_ListExpr#
				  )^#14:*expr.Expr_CallExpr#,
				  @result^#15:*expr.Expr_IdentExpr#
				)^#16:*expr.Expr_CallExpr#,
				// Result
				@result^#17:*expr.Expr_IdentExpr#)^#18:*expr.Expr_ComprehensionExpr#,
			  _+_(
				@result^#21:*expr.Expr_IdentExpr#,
				[
				  y^#3:*expr.Expr_IdentExpr#
				]^#22:*expr.Expr_ListExpr#
			  )^#23:*expr.Expr_CallExpr#,
			  @result^#24:*expr.Expr_IdentExpr#
			)^#25:*expr.Expr_CallExpr#,
			// Result
			@result^#26:*expr.Expr_IdentExpr#)^#27:*expr.Expr_ComprehensionExpr#`,
		M: `x^#1:*expr.Expr_IdentExpr#.filter(
			y^#3:*expr.Expr_IdentExpr#,
			^#18:filter#
		  )^#27:filter#,
		  y^#4:*expr.Expr_IdentExpr#.filter(
			z^#6:*expr.Expr_IdentExpr#,
			_>_(
			  z^#7:*expr.Expr_IdentExpr#,
			  0^#9:*expr.Constant_Int64Value#
			)^#8:*expr.Expr_CallExpr#
		  )^#18:filter#`,
	},
	{
		I: `has(a.b).filter(c, c)`,
		P: `__comprehension__(
			// Variable
			c,
			// Target
			a^#2:*expr.Expr_IdentExpr#.b~test-only~^#4:*expr.Expr_SelectExpr#,
			// Accumulator
			@result,
			// Init
			[]^#8:*expr.Expr_ListExpr#,
			// LoopCondition
			true^#9:*expr.Constant_BoolValue#,
			// LoopStep
			_?_:_(
			  c^#7:*expr.Expr_IdentExpr#,
			  _+_(
				@result^#10:*expr.Expr_IdentExpr#,
				[
				  c^#6:*expr.Expr_IdentExpr#
				]^#11:*expr.Expr_ListExpr#
			  )^#12:*expr.Expr_CallExpr#,
			  @result^#13:*expr.Expr_IdentExpr#
			)^#14:*expr.Expr_CallExpr#,
			// Result
			@result^#15:*expr.Expr_IdentExpr#)^#16:*expr.Expr_ComprehensionExpr#`,
		M: `^#4:has#.filter(
			c^#6:*expr.Expr_IdentExpr#,
			c^#7:*expr.Expr_IdentExpr#
			)^#16:filter#,
			has(
				a^#2:*expr.Expr_IdentExpr#.b^#3:*expr.Expr_SelectExpr#
			)^#4:has#`,
	},
	{
		I: `x.filter(y, y.exists(z, has(z.a)) && y.exists(z, has(z.b)))`,
		P: `__comprehension__(
			// Variable
			y,
			// Target
			x^#1:*expr.Expr_IdentExpr#,
			// Accumulator
			@result,
			// Init
			[]^#35:*expr.Expr_ListExpr#,
			// LoopCondition
			true^#36:*expr.Constant_BoolValue#,
			// LoopStep
			_?_:_(
			  _&&_(
				__comprehension__(
				  // Variable
				  z,
				  // Target
				  y^#4:*expr.Expr_IdentExpr#,
				  // Accumulator
				  @result,
				  // Init
				  false^#11:*expr.Constant_BoolValue#,
				  // LoopCondition
				  @not_strictly_false(
					!_(
					  @result^#12:*expr.Expr_IdentExpr#
					)^#13:*expr.Expr_CallExpr#
				  )^#14:*expr.Expr_CallExpr#,
				  // LoopStep
				  _||_(
					@result^#15:*expr.Expr_IdentExpr#,
					z^#8:*expr.Expr_IdentExpr#.a~test-only~^#10:*expr.Expr_SelectExpr#
				  )^#16:*expr.Expr_CallExpr#,
				  // Result
				  @result^#17:*expr.Expr_IdentExpr#)^#18:*expr.Expr_ComprehensionExpr#,
				__comprehension__(
				  // Variable
				  z,
				  // Target
				  y^#19:*expr.Expr_IdentExpr#,
				  // Accumulator
				  @result,
				  // Init
				  false^#26:*expr.Constant_BoolValue#,
				  // LoopCondition
				  @not_strictly_false(
					!_(
					  @result^#27:*expr.Expr_IdentExpr#
					)^#28:*expr.Expr_CallExpr#
				  )^#29:*expr.Expr_CallExpr#,
				  // LoopStep
				  _||_(
					@result^#30:*expr.Expr_IdentExpr#,
					z^#23:*expr.Expr_IdentExpr#.b~test-only~^#25:*expr.Expr_SelectExpr#
				  )^#31:*expr.Expr_CallExpr#,
				  // Result
				  @result^#32:*expr.Expr_IdentExpr#)^#33:*expr.Expr_ComprehensionExpr#
			  )^#34:*expr.Expr_CallExpr#,
			  _+_(
				@result^#37:*expr.Expr_IdentExpr#,
				[
				  y^#3:*expr.Expr_IdentExpr#
				]^#38:*expr.Expr_ListExpr#
			  )^#39:*expr.Expr_CallExpr#,
			  @result^#40:*expr.Expr_IdentExpr#
			)^#41:*expr.Expr_CallExpr#,
			// Result
			@result^#42:*expr.Expr_IdentExpr#)^#43:*expr.Expr_ComprehensionExpr#`,
		M: `x^#1:*expr.Expr_IdentExpr#.filter(
			y^#3:*expr.Expr_IdentExpr#,
			_&&_(
			  ^#18:exists#,
			  ^#33:exists#
			)^#34:*expr.Expr_CallExpr#
			)^#43:filter#,
			y^#19:*expr.Expr_IdentExpr#.exists(
				z^#21:*expr.Expr_IdentExpr#,
				^#25:has#
			)^#33:exists#,
			has(
				z^#23:*expr.Expr_IdentExpr#.b^#24:*expr.Expr_SelectExpr#
			)^#25:has#,
			y^#4:*expr.Expr_IdentExpr#.exists(
				z^#6:*expr.Expr_IdentExpr#,
				^#10:has#
			)^#18:exists#,
			has(
				z^#8:*expr.Expr_IdentExpr#.a^#9:*expr.Expr_SelectExpr#
			)^#10:has#`,
	},
	{
		I: `(has(a.b) || has(c.d)).string()`,
		P: `_||_(
			  a^#2:*expr.Expr_IdentExpr#.b~test-only~^#4:*expr.Expr_SelectExpr#,
			  c^#6:*expr.Expr_IdentExpr#.d~test-only~^#8:*expr.Expr_SelectExpr#
		    )^#9:*expr.Expr_CallExpr#.string()^#10:*expr.Expr_CallExpr#`,
		M: `has(
			  c^#6:*expr.Expr_IdentExpr#.d^#7:*expr.Expr_SelectExpr#
			)^#8:has#,
			has(
			  a^#2:*expr.Expr_IdentExpr#.b^#3:*expr.Expr_SelectExpr#
			)^#4:has#`,
	},
	{
		I: `has(a.b).asList().exists(c, c)`,
		P: `__comprehension__(
			// Variable
			c,
			// Target
			a^#2:*expr.Expr_IdentExpr#.b~test-only~^#4:*expr.Expr_SelectExpr#.asList()^#5:*expr.Expr_CallExpr#,
			// Accumulator
			@result,
			// Init
			false^#9:*expr.Constant_BoolValue#,
			// LoopCondition
			@not_strictly_false(
			  !_(
				@result^#10:*expr.Expr_IdentExpr#
			  )^#11:*expr.Expr_CallExpr#
			)^#12:*expr.Expr_CallExpr#,
			// LoopStep
			_||_(
			  @result^#13:*expr.Expr_IdentExpr#,
			  c^#8:*expr.Expr_IdentExpr#
			)^#14:*expr.Expr_CallExpr#,
			// Result
			@result^#15:*expr.Expr_IdentExpr#)^#16:*expr.Expr_ComprehensionExpr#`,
		M: `^#4:has#.asList()^#5:*expr.Expr_CallExpr#.exists(
			c^#7:*expr.Expr_IdentExpr#,
			c^#8:*expr.Expr_IdentExpr#
		  )^#16:exists#,
		  has(
			a^#2:*expr.Expr_IdentExpr#.b^#3:*expr.Expr_SelectExpr#
		  )^#4:has#`,
	},
	{
		I: `[has(a.b), has(c.d)].exists(e, e)`,
		P: `__comprehension__(
			// Variable
			e,
			// Target
			[
			  a^#3:*expr.Expr_IdentExpr#.b~test-only~^#5:*expr.Expr_SelectExpr#,
			  c^#7:*expr.Expr_IdentExpr#.d~test-only~^#9:*expr.Expr_SelectExpr#
			]^#1:*expr.Expr_ListExpr#,
			// Accumulator
			@result,
			// Init
			false^#13:*expr.Constant_BoolValue#,
			// LoopCondition
			@not_strictly_false(
			  !_(
				@result^#14:*expr.Expr_IdentExpr#
			  )^#15:*expr.Expr_CallExpr#
			)^#16:*expr.Expr_CallExpr#,
			// LoopStep
			_||_(
			  @result^#17:*expr.Expr_IdentExpr#,
			  e^#12:*expr.Expr_IdentExpr#
			)^#18:*expr.Expr_CallExpr#,
			// Result
			@result^#19:*expr.Expr_IdentExpr#)^#20:*expr.Expr_ComprehensionExpr#`,
		M: `[
			^#5:has#,
			^#9:has#
		  ]^#1:*expr.Expr_ListExpr#.exists(
			e^#11:*expr.Expr_IdentExpr#,
			e^#12:*expr.Expr_IdentExpr#
		  )^#20:exists#,
		  has(
			c^#7:*expr.Expr_IdentExpr#.d^#8:*expr.Expr_SelectExpr#
		  )^#9:has#,
		  has(
			a^#3:*expr.Expr_IdentExpr#.b^#4:*expr.Expr_SelectExpr#
		  )^#5:has#`,
	},
	{
		I: `y!=y!=y!=y!=y!=y!=y!=y!=y!=-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y
		!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y
		!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y
		!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y
		!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y
		!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y!=-y!=-y-y!=-y`,
		E:      `ERROR: <input>:-1:0: max recursion depth exceeded`,
		PrattE: "-",
	},
	{
		// More than 32 nested list creation statements
		I:      `[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[['not fine']]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]`,
		E:      `ERROR: <input>:-1:0: expression recursion limit exceeded: 32`,
		PrattE: "-",
	},
	{
		// More than 32 arithmetic operations.
		I: `1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10
		+ 11 + 12 + 13 + 14 + 15 + 16 + 17 + 18 + 19 + 20
		+ 21 + 22 + 23 + 24 + 25 + 26 + 27 + 28 + 29 + 30
		+ 31 + 32 + 33 + 34`,
		E:      `ERROR: <input>:-1:0: max recursion depth exceeded`,
		PrattE: "-",
	},
	{
		// More than 32 field selections
		I:      `a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z.A.B.C.D.E.F.G.H`,
		E:      `ERROR: <input>:-1:0: max recursion depth exceeded`,
		PrattE: "-",
	},
	{
		// More than 32 index operations
		I: `a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20]
		     [21][22][23][24][25][26][27][28][29][30][31][32][33]`,
		E:      `ERROR: <input>:-1:0: max recursion depth exceeded`,
		PrattE: "-",
	},
	{
		// More than 32 relation operators
		I: `a < 1 < 2 < 3 < 4 < 5 < 6 < 7 < 8 < 9 < 10 < 11
		      < 12 < 13 < 14 < 15 < 16 < 17 < 18 < 19 < 20 < 21
			  < 22 < 23 < 24 < 25 < 26 < 27 < 28 < 29 < 30 < 31
			  < 32 < 33`,
		E:      `ERROR: <input>:-1:0: max recursion depth exceeded`,
		PrattE: "-",
	},
	{
		// More than 32 index / relation operators. Note, the recursion count is the
		// maximum recursion level on the left or right side index expression (20) plus
		// the number of relation operators (13)
		I: `a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20] !=
		a[1][2][3][4][5][6][7][8][9][10][11][12][13][14][15][16][17][18][19][20]`,
		E:      `ERROR: <input>:-1:0: max recursion depth exceeded`,
		PrattE: "-",
	},
	{
		I: `self.true == 1`,
		E: `ERROR: <input>:1:6: Syntax error: mismatched input 'true' expecting IDENTIFIER
		| self.true == 1
		| .....^`,
		PrattE: `ERROR: <input>:1:6: expected identifier after '.'
		| self.true == 1
		| .....^`,
	},
	{
		I: `a.?b && a[?b]`,
		E: `ERROR: <input>:1:2: unsupported syntax '.?'
        | a.?b && a[?b]
        | .^
        ERROR: <input>:1:10: unsupported syntax '[?'
        | a.?b && a[?b]
		    | .........^`,
		PrattE: `ERROR: <input>:1:2: unsupported syntax '.?'
        | a.?b && a[?b]
        | .^
        ERROR: <input>:1:10: unsupported syntax '?'
        | a.?b && a[?b]
		    | .........^`,
	},
	{
		I:    `a.?b[?0] && a[?c]`,
		Opts: []Option{EnableOptionalSyntax(true)},
		P: `_&&_(
			_[?_](
			  _?._(
				a^#1:*expr.Expr_IdentExpr#,
				"b"^#2:*expr.Constant_StringValue#
			  )^#3:*expr.Expr_CallExpr#,
			  0^#5:*expr.Constant_Int64Value#
			)^#4:*expr.Expr_CallExpr#,
			_[?_](
			  a^#6:*expr.Expr_IdentExpr#,
			  c^#8:*expr.Expr_IdentExpr#
			)^#7:*expr.Expr_CallExpr#
		  )^#9:*expr.Expr_CallExpr#`,
		PrattP: `_&&_(
			_[?_](
			  _?._(
				a^#1:*expr.Expr_IdentExpr#,
				"b"^#3:*expr.Constant_StringValue#
			  )^#2:*expr.Expr_CallExpr#,
			  0^#5:*expr.Constant_Int64Value#
			)^#4:*expr.Expr_CallExpr#,
			_[?_](
			  a^#6:*expr.Expr_IdentExpr#,
			  c^#8:*expr.Expr_IdentExpr#
			)^#7:*expr.Expr_CallExpr#
		  )^#9:*expr.Expr_CallExpr#`,
	},
	{
		I:    `{?'key': value}`,
		Opts: []Option{EnableOptionalSyntax(true)},
		P: `{
			?"key"^#3:*expr.Constant_StringValue#:value^#4:*expr.Expr_IdentExpr#^#2:*expr.Expr_CreateStruct_Entry#
		  }^#1:*expr.Expr_StructExpr#`,
		PrattP: `{
			?"key"^#2:*expr.Constant_StringValue#:value^#4:*expr.Expr_IdentExpr#^#3:*expr.Expr_CreateStruct_Entry#
		  }^#1:*expr.Expr_StructExpr#`,
	},
	{
		I:    `[?a, ?b]`,
		Opts: []Option{EnableOptionalSyntax(true)},
		P: `[
			a^#2:*expr.Expr_IdentExpr#,
			b^#3:*expr.Expr_IdentExpr#
		  ]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I:    `[?a[?b]]`,
		Opts: []Option{EnableOptionalSyntax(true)},
		P: `[
			_[?_](
			  a^#2:*expr.Expr_IdentExpr#,
			  b^#4:*expr.Expr_IdentExpr#
			)^#3:*expr.Expr_CallExpr#
		  ]^#1:*expr.Expr_ListExpr#`,
	},
	{
		I: `[?a, ?b]`,
		E: `
	    ERROR: <input>:1:2: unsupported syntax '?'
		 | [?a, ?b]
		 | .^
	    ERROR: <input>:1:6: unsupported syntax '?'
		 | [?a, ?b]
		 | .....^`,
	},
	{
		I:    `Msg{?field: value}`,
		Opts: []Option{EnableOptionalSyntax(true)},
		P: `Msg{
			?field:value^#3:*expr.Expr_IdentExpr#^#2:*expr.Expr_CreateStruct_Entry#
		  }^#1:*expr.Expr_StructExpr#`,
	},
	{
		I: `Msg{?field: value} && {?'key': value}`,
		E: `
		ERROR: <input>:1:5: unsupported syntax '?'
	 	 | Msg{?field: value} && {?'key': value}
		 | ....^
	    ERROR: <input>:1:24: unsupported syntax '?'
		 | Msg{?field: value} && {?'key': value}
		 | .......................^`,
	},
	{
		I:    "a.`b-c`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		P:    `a^#1:*expr.Expr_IdentExpr#.b-c^#2:*expr.Expr_SelectExpr#`,
	},
	{I: "a.`b c`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		P:    `a^#1:*expr.Expr_IdentExpr#.b c^#2:*expr.Expr_SelectExpr#`,
	},
	{
		I:    "a.`b.c`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		P:    `a^#1:*expr.Expr_IdentExpr#.b.c^#2:*expr.Expr_SelectExpr#`,
	},
	{
		I:    "a.`in`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		P:    `a^#1:*expr.Expr_IdentExpr#.in^#2:*expr.Expr_SelectExpr#`,
	},
	{
		I:    "a.`/foo`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		P:    `a^#1:*expr.Expr_IdentExpr#./foo^#2:*expr.Expr_SelectExpr#`,
	},
	{
		I:    "Message{`in`: true}",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		P: `Message{
			in:true^#3:*expr.Constant_BoolValue#^#2:*expr.Expr_CreateStruct_Entry#
		  }^#1:*expr.Expr_StructExpr#`,
	},
	{
		I:    "`b-c`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		E: "ERROR: <input>:1:1: Syntax error: mismatched input '`b-c`' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}\n" +
			"| `b-c`\n" +
			"| ^",
		PrattE: "ERROR: <input>:1:1: unexpected quoted identifier\n" +
			"| `b-c`\n" +
			"| ^",
	},
	{
		I:    "`b-c`()",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		E: "ERROR: <input>:1:1: Syntax error: extraneous input '`b-c`' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}\n" +
			"| `b-c`()\n" +
			"| ^\n" +
			"ERROR: <input>:1:7: Syntax error: mismatched input ')' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}\n" +
			"| `b-c`()\n" +
			"| ......^",
		PrattE: "ERROR: <input>:1:1: unexpected quoted identifier\n" +
			"| `b-c`()\n" +
			"| ^",
	},
	{
		I:    "a.`$b`",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		E: "ERROR: <input>:1:3: Syntax error: token recognition error at: '`$'\n" +
			"| a.`$b`\n" +
			"| ..^\n" +
			"ERROR: <input>:1:6: Syntax error: token recognition error at: '`'\n" +
			"| a.`$b`\n" +
			"| .....^",
		PrattE: "ERROR: <input>:1:3: unexpected quoted identifier\n" +
			"| a.`$b`\n" +
			"| ..^",
	},
	{
		I:    "a.`b.c`()",
		Opts: []Option{EnableIdentEscapeSyntax(true)},
		E: "ERROR: <input>:1:8: Syntax error: mismatched input '(' expecting <EOF>\n" +
			"| a.`b.c`()\n" +
			"| .......^\n",
		PrattE: "ERROR: <input>:1:3: unexpected quoted identifier\n" +
			"| a.`b.c`()\n" +
			"| ..^",
	},
	{
		I:    "a.`b-c`",
		Opts: []Option{EnableIdentEscapeSyntax(false)},
		E: "ERROR: <input>:1:3: unsupported syntax: '`'\n" +
			"| a.`b-c`\n" +
			"| ..^",
	},
	{
		I:    "a.`b.c`",
		Opts: []Option{EnableIdentEscapeSyntax(false)},
		E: "ERROR: <input>:1:3: unsupported syntax: '`'\n" +
			"| a.`b.c`\n" +
			"| ..^\n",
	},
	{
		I:    "a.`in`",
		Opts: []Option{EnableIdentEscapeSyntax(false)},
		E: "ERROR: <input>:1:3: unsupported syntax: '`'\n" +
			"| a.`in`\n" +
			"| ..^",
	},
	{
		I:    "a.`/foo`",
		Opts: []Option{EnableIdentEscapeSyntax(false)},
		E: "ERROR: <input>:1:3: unsupported syntax: '`'\n" +
			"| a.`/foo`\n" +
			"| ..^",
	},
	{
		I:    "Message{`in`: true}",
		Opts: []Option{EnableIdentEscapeSyntax(false)},
		E: "ERROR: <input>:1:9: unsupported syntax: '`'\n" +
			"| Message{`in`: true}\n" +
			"| ........^",
	},
	{
		I: `noop_macro(123)`,
		Opts: []Option{
			Macros(NewGlobalVarArgMacro("noop_macro",
				func(eh ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
					return nil, nil
				})),
		},
		P: `noop_macro(
			123^#2:*expr.Constant_Int64Value#
		  )^#1:*expr.Expr_CallExpr#`,
	},
	{
		I: `x{?.`,
		Opts: []Option{
			ErrorRecoveryLookaheadTokenLimit(10),
			ErrorRecoveryLimit(10),
		},
		E: `
		ERROR: <input>:1:3: unsupported syntax '?'
		 | x{?.
		 | ..^
	    ERROR: <input>:1:4: Syntax error: mismatched input '.' expecting {IDENTIFIER, ESC_IDENTIFIER}
		 | x{?.
		 | ...^`,
		PrattE: `ERROR: <input>:1:3: unsupported syntax '?'
		 | x{?.
		 | ..^
		ERROR: <input>:1:4: expected struct field name
		 | x{?.
		 | ...^
		ERROR: <input>:1:5: expected '}'
		 | x{?.
		 | ....^`,
	},
	{
		I: `x{.`,
		E: `
		ERROR: <input>:1:3: Syntax error: mismatched input '.' expecting {'}', ',', '?', IDENTIFIER, ESC_IDENTIFIER}
		 | x{.
		 | ..^`,
		PrattE: `ERROR: <input>:1:3: expected struct field name
		 | x{.
		 | ..^
		ERROR: <input>:1:4: expected '}'
		 | x{.
		 | ...^`,
	},
	{
		I:    `'3# < 10" '& tru ^^`,
		Opts: []Option{ErrorReportingLimit(2)},
		E: `
		ERROR: <input>:1:12: Syntax error: token recognition error at: '& '
		 | '3# < 10" '& tru ^^
		 | ...........^
		ERROR: <input>:1:18: Syntax error: token recognition error at: '^'
		 | '3# < 10" '& tru ^^
		 | .................^
		ERROR: <input>:1:19: Syntax error: More than 2 syntax errors
		 | '3# < 10" '& tru ^^
		 | ..................^
		`,
		PrattE: `ERROR: <input>:1:12: unexpected single '&', expected '&&'
		 | '3# < 10" '& tru ^^
		 | ...........^
		ERROR: <input>:1:18: unexpected character
		 | '3# < 10" '& tru ^^
		 | .................^`,
	},
	{
		I: `'\udead' == '\ufffd'`,
		E: `
		ERROR: <input>:1:1: invalid unicode code point
         | '\udead' == '\ufffd'
         | ^`,
	},
	// Macro tests for old accumulator name
	{
		I: `m.exists(v, f)`,
		Opts: []Option{
			EnableHiddenAccumulatorName(false),
		},
		P: `__comprehension__(
				// Variable
				v,
				// Target
				m^#1:*expr.Expr_IdentExpr#,
				// Accumulator
				__result__,
				// Init
				false^#5:*expr.Constant_BoolValue#,
				// LoopCondition
				@not_strictly_false(
					!_(
					  __result__^#6:*expr.Expr_IdentExpr#
					)^#7:*expr.Expr_CallExpr#
				)^#8:*expr.Expr_CallExpr#,
				// LoopStep
				_||_(
					__result__^#9:*expr.Expr_IdentExpr#,
					f^#4:*expr.Expr_IdentExpr#
				)^#10:*expr.Expr_CallExpr#,
				// Result
				__result__^#11:*expr.Expr_IdentExpr#)^#12:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.exists(
				v^#3:*expr.Expr_IdentExpr#,
				f^#4:*expr.Expr_IdentExpr#
				  )^#12:exists#`,
	},
	{
		I: `m.all(v, f)`,
		Opts: []Option{
			EnableHiddenAccumulatorName(false),
		},
		P: `__comprehension__(
				// Variable
				v,
				// Target
				m^#1:*expr.Expr_IdentExpr#,
				// Accumulator
				__result__,
				// Init
				true^#5:*expr.Constant_BoolValue#,
				// LoopCondition
				@not_strictly_false(
					__result__^#6:*expr.Expr_IdentExpr#
				)^#7:*expr.Expr_CallExpr#,
				// LoopStep
				_&&_(
					__result__^#8:*expr.Expr_IdentExpr#,
					f^#4:*expr.Expr_IdentExpr#
				)^#9:*expr.Expr_CallExpr#,
				// Result
				__result__^#10:*expr.Expr_IdentExpr#)^#11:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.all(
				v^#3:*expr.Expr_IdentExpr#,
				f^#4:*expr.Expr_IdentExpr#
				  )^#11:all#`,
	},
	{
		I: `m.existsOne(v, f)`,
		Opts: []Option{
			EnableHiddenAccumulatorName(false),
		},
		P: `__comprehension__(
				// Variable
				v,
				// Target
				m^#1:*expr.Expr_IdentExpr#,
				// Accumulator
				__result__,
				// Init
				0^#5:*expr.Constant_Int64Value#,
				// LoopCondition
				true^#6:*expr.Constant_BoolValue#,
				// LoopStep
				_?_:_(
					f^#4:*expr.Expr_IdentExpr#,
					_+_(
						  __result__^#7:*expr.Expr_IdentExpr#,
					  1^#8:*expr.Constant_Int64Value#
					)^#9:*expr.Expr_CallExpr#,
					__result__^#10:*expr.Expr_IdentExpr#
				)^#11:*expr.Expr_CallExpr#,
				// Result
				_==_(
					__result__^#12:*expr.Expr_IdentExpr#,
					1^#13:*expr.Constant_Int64Value#
				)^#14:*expr.Expr_CallExpr#)^#15:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.existsOne(
				v^#3:*expr.Expr_IdentExpr#,
				f^#4:*expr.Expr_IdentExpr#
				  )^#15:existsOne#`,
	},
	{
		I: `m.map(v, f)`,
		Opts: []Option{
			EnableHiddenAccumulatorName(false),
		},
		P: `__comprehension__(
				// Variable
				v,
				// Target
				m^#1:*expr.Expr_IdentExpr#,
				// Accumulator
				__result__,
				// Init
				[]^#5:*expr.Expr_ListExpr#,
				// LoopCondition
				true^#6:*expr.Constant_BoolValue#,
				// LoopStep
				_+_(
					__result__^#7:*expr.Expr_IdentExpr#,
					[
						f^#4:*expr.Expr_IdentExpr#
					]^#8:*expr.Expr_ListExpr#
				)^#9:*expr.Expr_CallExpr#,
				// Result
				__result__^#10:*expr.Expr_IdentExpr#)^#11:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.map(
				v^#3:*expr.Expr_IdentExpr#,
				f^#4:*expr.Expr_IdentExpr#
				  )^#11:map#`,
	},
	{
		I: `m.map(v, p, f)`,
		Opts: []Option{
			EnableHiddenAccumulatorName(false),
		},
		P: `__comprehension__(
				// Variable
				v,
				// Target
				m^#1:*expr.Expr_IdentExpr#,
				// Accumulator
				__result__,
				// Init
				[]^#6:*expr.Expr_ListExpr#,
				// LoopCondition
				true^#7:*expr.Constant_BoolValue#,
				// LoopStep
				_?_:_(
					p^#4:*expr.Expr_IdentExpr#,
					_+_(
						__result__^#8:*expr.Expr_IdentExpr#,
						[
							f^#5:*expr.Expr_IdentExpr#
						]^#9:*expr.Expr_ListExpr#
					)^#10:*expr.Expr_CallExpr#,
					__result__^#11:*expr.Expr_IdentExpr#
				)^#12:*expr.Expr_CallExpr#,
				// Result
				__result__^#13:*expr.Expr_IdentExpr#)^#14:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.map(
				v^#3:*expr.Expr_IdentExpr#,
				p^#4:*expr.Expr_IdentExpr#,
				f^#5:*expr.Expr_IdentExpr#
				  )^#14:map#`,
	},

	{
		I: `m.filter(v, p)`,
		Opts: []Option{
			EnableHiddenAccumulatorName(false),
		},
		P: `__comprehension__(
				// Variable
				v,
				// Target
				m^#1:*expr.Expr_IdentExpr#,
				// Accumulator
				__result__,
				// Init
				[]^#5:*expr.Expr_ListExpr#,
				// LoopCondition
				true^#6:*expr.Constant_BoolValue#,
				// LoopStep
				_?_:_(
					p^#4:*expr.Expr_IdentExpr#,
					_+_(
						__result__^#7:*expr.Expr_IdentExpr#,
						[
							v^#3:*expr.Expr_IdentExpr#
						]^#8:*expr.Expr_ListExpr#
					)^#9:*expr.Expr_CallExpr#,
					__result__^#10:*expr.Expr_IdentExpr#
				)^#11:*expr.Expr_CallExpr#,
				// Result
				__result__^#12:*expr.Expr_IdentExpr#)^#13:*expr.Expr_ComprehensionExpr#`,
		M: `m^#1:*expr.Expr_IdentExpr#.filter(
				v^#3:*expr.Expr_IdentExpr#,
				p^#4:*expr.Expr_IdentExpr#
				  )^#13:filter#`,
	},
}

type testInfo struct {
	// I contains the input expression to be parsed.
	I string

	// P contains the type/id adorned debug output of the expression tree.
	P string

	// PrattP contains the expected output for the Pratt parser when it differs from P.
	PrattP string

	// E contains the expected error output for a failed parse, or "" if the parse is expected to be successful.
	E string

	// PrattE contains the expected error output for the Pratt parser when it differs from E.
	PrattE string

	// L contains the expected source adorned debug output of the expression tree.
	L string

	// PrattL contains the expected source adorned debug output for the Pratt parser when it differs from L.
	PrattL string

	// M contains the expected adorned debug output of the macro calls map
	M string

	// PrattM contains the expected adorned debug output of the macro calls map for the Pratt parser when it differs from M.
	PrattM string

	// Opts contains the list of options to be configured with the parser before parsing the expression.
	Opts []Option
}

type metadata interface {
	GetLocation(exprID int64) (common.Location, bool)
}

type kindAndIDAdorner struct {
	sourceInfo *ast.SourceInfo
}

func (k *kindAndIDAdorner) GetMetadata(elem any) string {
	switch e := elem.(type) {
	case ast.Expr:
		if macroCall, found := k.sourceInfo.GetMacroCall(e.ID()); found {
			return fmt.Sprintf("^#%d:%s#", e.ID(), macroCall.AsCall().FunctionName())
		}
		var valType string
		switch e.Kind() {
		case ast.CallKind:
			valType = "*expr.Expr_CallExpr"
		case ast.ComprehensionKind:
			valType = "*expr.Expr_ComprehensionExpr"
		case ast.IdentKind:
			valType = "*expr.Expr_IdentExpr"
		case ast.LiteralKind:
			lit := e.AsLiteral()
			switch lit.(type) {
			case types.Bool:
				valType = "*expr.Constant_BoolValue"
			case types.Bytes:
				valType = "*expr.Constant_BytesValue"
			case types.Double:
				valType = "*expr.Constant_DoubleValue"
			case types.Int:
				valType = "*expr.Constant_Int64Value"
			case types.Null:
				valType = "*expr.Constant_NullValue"
			case types.String:
				valType = "*expr.Constant_StringValue"
			case types.Uint:
				valType = "*expr.Constant_Uint64Value"
			default:
				valType = reflect.TypeOf(lit).String()
			}
		case ast.ListKind:
			valType = "*expr.Expr_ListExpr"
		case ast.MapKind, ast.StructKind:
			valType = "*expr.Expr_StructExpr"
		case ast.SelectKind:
			valType = "*expr.Expr_SelectExpr"
		}
		return fmt.Sprintf("^#%d:%s#", e.ID(), valType)
	case ast.EntryExpr:
		return fmt.Sprintf("^#%d:%s#", e.ID(), "*expr.Expr_CreateStruct_Entry")
	}
	return ""
}

type locationAdorner struct {
	sourceInfo *ast.SourceInfo
}

var _ metadata = &locationAdorner{}

func (l *locationAdorner) GetLocation(exprID int64) (common.Location, bool) {
	loc := l.sourceInfo.GetStartLocation(exprID)
	return loc, loc != common.NoLocation
}

func (l *locationAdorner) GetMetadata(elem any) string {
	var elemID int64
	switch elem := elem.(type) {
	case ast.Expr:
		elemID = elem.ID()
	case ast.EntryExpr:
		elemID = elem.ID()
	}
	location, _ := l.GetLocation(elemID)
	return fmt.Sprintf("^#%d[%d,%d]#", elemID, location.Line(), location.Column())
}

func convertMacroCallsToString(source *ast.SourceInfo) string {
	macroCalls := source.MacroCalls()
	keys := make([]int64, len(macroCalls))
	adornedStrings := make([]string, len(macroCalls))
	i := 0
	for k := range macroCalls {
		keys[i] = k
		i++
	}
	fac := ast.NewExprFactory()
	// Sort the keys in descending order to create a stable ordering for tests and improve readability.
	sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })
	i = 0
	for _, key := range keys {
		call := macroCalls[int64(key)].AsCall()
		var callWithID ast.Expr
		if call.IsMemberFunction() {
			callWithID = fac.NewMemberCall(int64(key), call.FunctionName(), call.Target(), call.Args()...)
		} else {
			callWithID = fac.NewCall(int64(key), call.FunctionName(), call.Args()...)
		}
		adornedStrings[i] = debug.ToAdornedDebugString(
			callWithID,
			&kindAndIDAdorner{sourceInfo: source})
		i++
	}
	return strings.Join(adornedStrings, ",\n")
}

func TestParse(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		name := fmt.Sprintf("enablePrattParser=%t", pratt)
		t.Run(name, func(t *testing.T) {
			defaultParser := newTestParser(t, EnablePrattParser(pratt))
			for i, tst := range testCases {
				name := fmt.Sprintf("%d %s", i, tst.I)
				// Local variable required as the closure will reference the value for the last
				// 'tst' value rather than the local 'tc' instance declared within the loop.
				tc := tst
				t.Run(name, func(t *testing.T) {
					// Runs the tests in parallel to ensure that there are no data races
					// due to shared mutable state across tests.
					t.Parallel()
					p := defaultParser
					if len(tc.Opts) > 0 {
						p = newTestParser(t, append([]Option{EnablePrattParser(pratt)}, tc.Opts...)...)
					}
					src := common.NewTextSource(tc.I)
					parsed, errors := p.Parse(src)
					wantP := tc.P
					wantE := tc.E
					wantL := tc.L
					wantM := tc.M
					if pratt {
						if tc.PrattP != "" {
							wantP = tc.PrattP
						}
						if tc.PrattE == "-" {
							wantE = ""
						} else if tc.PrattE != "" {
							wantE = tc.PrattE
						}
						if tc.PrattL != "" {
							wantL = tc.PrattL
						}
						if tc.PrattM != "" {
							wantM = tc.PrattM
						}
					}
					if len(errors.GetErrors()) > 0 {
						actualErr := errors.ToDisplayString()
						if wantE == "" {
							t.Fatalf("Unexpected errors: %v", actualErr)
						} else if !test.Compare(actualErr, wantE) {
							t.Fatal(test.DiffMessage("Error mismatch", actualErr, wantE))
						}
						return
					} else if wantE != "" {
						t.Fatalf("Expected error not thrown: '%s'", wantE)
					}
					failureDisplayMethod := fmt.Sprintf("Parse(\"%s\")", tc.I)
					if wantP != "" {
						actualWithKind := debug.ToAdornedDebugString(parsed.Expr(), &kindAndIDAdorner{})
						if !test.Compare(actualWithKind, wantP) {
							t.Fatal(test.DiffMessage(fmt.Sprintf("Structure - %s", failureDisplayMethod), actualWithKind, wantP))
						}
					}

					if !pratt && wantL != "" {
						actualWithLocation := debug.ToAdornedDebugString(parsed.Expr(), &locationAdorner{parsed.SourceInfo()})
						if !test.Compare(actualWithLocation, wantL) {
							t.Fatal(test.DiffMessage(fmt.Sprintf("Location - %s", failureDisplayMethod), actualWithLocation, wantL))
						}
					}

					if wantM != "" {
						actualAdornedMacroCalls := convertMacroCallsToString(parsed.SourceInfo())
						if !test.Compare(actualAdornedMacroCalls, wantM) {
							t.Fatal(test.DiffMessage(fmt.Sprintf("Macro Calls - %s", failureDisplayMethod), actualAdornedMacroCalls, wantM))
						}
					}

					// Verify there are no unused IDs in the source info.
					astIDs := parsed.IDs()
					unusedIDs := []int64{}
					for id := range parsed.SourceInfo().OffsetRanges() {
						if !astIDs[id] {
							unusedIDs = append(unusedIDs, id)
						}
					}
					if len(unusedIDs) > 0 {
						t.Errorf("SourceInfo has offset range for IDs %v, but no such nodes exists in AST: %s",
							unusedIDs, debug.ToDebugStringWithIDs(parsed.Expr()))
					}

					// Verify that source info offset ranges are shifted when the source is prepended with whitespace.
					padding := strings.Repeat("         \n", 10)
					padSrc := &RelativeSource{
						Source:   common.NewTextSource(padding + src.Content()),
						localSrc: src,
						absLoc:   common.NewLocation(11, 0),
					}
					padded, padErrs := p.Parse(padSrc)
					if len(padErrs.GetErrors()) > 0 {
						t.Fatalf("Unexpected errors with padded source: %v", padErrs.ToDisplayString())
					}
					for id, origRange := range parsed.SourceInfo().OffsetRanges() {
						padRange, found := padded.SourceInfo().GetOffsetRange(id)
						if !found {
							t.Errorf("ID %d not found in padded source info", id)
							continue
						}
						want := ast.OffsetRange{Start: origRange.Start + 100, Stop: origRange.Stop + 100}
						if padRange != want {
							t.Errorf("ID %d offset range mismatch: got %v, want %v", id, padRange, want)
						}
					}
				})
			}
		})
	}
}

func TestExpressionSizeCodePointLimit(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			p, err := NewParser(Macros(AllMacros...), ExpressionSizeCodePointLimit(2), EnablePrattParser(pratt))
			if err != nil {
				t.Fatal(err)
			}
			src := common.NewTextSource("foo")
			_, errs := p.Parse(src)
			if got, want := len(errs.GetErrors()), 1; got != want {
				t.Fatalf("got %d errors, want %d errors: %s", got, want, errs.ToDisplayString())
			}
			if got, want := errs.GetErrors()[0].Message, "expression code point size exceeds limit: size: 3, limit 2"; got != want {
				t.Fatalf("got %q, want %q: %s", got, want, errs.GetErrors()[0].ToDisplayString(src))
			}
		})
	}
}

func TestMaxExpressionNodeCount(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			p, err := NewParser(Macros(AllMacros...), MaxExpressionNodeCount(10), EnablePrattParser(pratt))
			if err != nil {
				t.Fatal(err)
			}
			src := common.NewTextSource("a.exists(x, x.exists(y, y == 1))")
			_, errs := p.Parse(src)
			if len(errs.GetErrors()) == 0 {
				t.Fatalf("expected errors, got none: %s", errs.ToDisplayString())
			}
			if !strings.Contains(errs.GetErrors()[0].Message, "expression count exceeds limit of 10 while expanding macro 'exists'") {
				t.Fatalf("got %q, want substring matching limit error: %s", errs.GetErrors()[0].Message, errs.GetErrors()[0].ToDisplayString(src))
			}
		})
	}
}

func TestParserOptionErrors(t *testing.T) {
	if _, err := NewParser(Macros(AllMacros...), MaxRecursionDepth(-2)); err == nil {
		t.Fatalf("got %q, want %q", err, "max recursion depth must be greater than or equal to -1: -2")
	}
	if _, err := NewParser(ErrorRecoveryLimit(-2)); err == nil {
		t.Fatalf("got %q, want %q", err, "error recovery limit must be greater than or equal to -1: -2")
	}
	if _, err := NewParser(ErrorRecoveryLookaheadTokenLimit(0)); err == nil {
		t.Fatalf("got %q, want %q", err, "error recovery lookahead token limit must be at least 1: 0")
	}
	if _, err := NewParser(ErrorReportingLimit(0)); err == nil {
		t.Fatalf("got %q, want %q", err, "error reporting limit must be greater than 0: -2")
	}
	if _, err := NewParser(ExpressionSizeCodePointLimit(-2)); err == nil {
		t.Fatalf("got %q, want %q", err, "expression size code point limit must be greater than or equal to -1: -2")
	}
	if _, err := NewParser(MaxExpressionNodeCount(-2)); err == nil {
		t.Fatalf("got %q, want %q", err, "max expression node count must be greater than or equal to -1: -2")
	}
}

func TestSourceInfoPositions(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			t.Run("ASCII", func(t *testing.T) {
				src := common.NewTextSource("a + b")
				p, err := NewParser(EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				parsed, errs := p.Parse(src)
				if len(errs.GetErrors()) > 0 {
					t.Fatalf("Parse() failed: %s", errs.ToDisplayString())
				}
				sourceInfo := parsed.SourceInfo()
				root := parsed.Expr()
				if sourceInfo.GetStartLocation(root.ID()).Column() != 2 {
					t.Errorf("expected root column 2, got %d", sourceInfo.GetStartLocation(root.ID()).Column())
				}
				args := root.AsCall().Args()
				if len(args) != 2 {
					t.Fatalf("expected 2 args, got %d", len(args))
				}
				if sourceInfo.GetStartLocation(args[0].ID()).Column() != 0 {
					t.Errorf("expected arg[0] column 0, got %d", sourceInfo.GetStartLocation(args[0].ID()).Column())
				}
				if sourceInfo.GetStartLocation(args[1].ID()).Column() != 4 {
					t.Errorf("expected arg[1] column 4, got %d", sourceInfo.GetStartLocation(args[1].ID()).Column())
				}
			})

			t.Run("MixedUnicodeMultiByteAndMultiLine", func(t *testing.T) {
				// Mix of 1-byte ASCII (a, b, +), 2-byte Unicode ("ñ"), 3-byte Unicode ("❤"), and 4-byte Unicode ("🚀")
				expr := "a + \"ñ\" +\n\"🚀\" + \"❤\" + b"
				src := common.NewTextSource(expr)
				p, err := NewParser(EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				parsed, errs := p.Parse(src)
				if len(errs.GetErrors()) > 0 {
					t.Fatalf("Parse() failed: %s", errs.ToDisplayString())
				}
				sinfo := parsed.SourceInfo()

				// AST hierarchy:
				// expr4: [expr3] + [b]               (line 2, col 10)
				// expr3: [expr2] + ["❤"]             (line 2, col 4)
				// expr2: [expr1] + ["🚀"]            (line 1, col 8)
				// expr1: [a] + ["ñ"]                 (line 1, col 2)
				expr4 := parsed.Expr()
				call4 := expr4.AsCall()
				bExpr := call4.Args()[1]

				expr3 := call4.Args()[0]
				call3 := expr3.AsCall()
				heartExpr := call3.Args()[1]

				expr2 := call3.Args()[0]
				call2 := expr2.AsCall()
				rocketExpr := call2.Args()[1]

				expr1 := call2.Args()[0]
				call1 := expr1.AsCall()
				aExpr := call1.Args()[0]
				enyeExpr := call1.Args()[1]

				assertLoc := func(name string, id int64, wantLine, wantCol int32) {
					t.Helper()
					loc := sinfo.GetStartLocation(id)
					if int32(loc.Line()) != wantLine || int32(loc.Column()) != wantCol {
						t.Errorf("%s location mismatch: got (%d, %d), want (%d, %d)",
							name, loc.Line(), loc.Column(), wantLine, wantCol)
					}
				}

				assertLoc("call1 (+)", expr1.ID(), 1, 2)
				assertLoc("a", aExpr.ID(), 1, 0)
				assertLoc("\"ñ\" (2-byte)", enyeExpr.ID(), 1, 4)
				assertLoc("call2 (+)", expr2.ID(), 1, 8)
				assertLoc("\"🚀\" (4-byte)", rocketExpr.ID(), 2, 0)
				assertLoc("call3 (+)", expr3.ID(), 2, 4)
				assertLoc("\"❤\" (3-byte)", heartExpr.ID(), 2, 6)
				assertLoc("call4 (+)", expr4.ID(), 2, 10)
				assertLoc("b", bExpr.ID(), 2, 12)
			})
		})
	}
}

func TestPopulateMacroCalls(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			t.Run("DisabledByDefault", func(t *testing.T) {
				p, err := NewParser(Macros(AllMacros...), PopulateMacroCalls(false), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				parsed, errs := p.Parse(common.NewTextSource("has(a.b)"))
				if len(errs.GetErrors()) > 0 {
					t.Fatalf("unexpected error: %s", errs.ToDisplayString())
				}
				if len(parsed.SourceInfo().MacroCalls()) != 0 {
					t.Errorf("expected 0 macro calls, got %d", len(parsed.SourceInfo().MacroCalls()))
				}
			})

			t.Run("GlobalMacroCallRecorded", func(t *testing.T) {
				p, err := NewParser(Macros(AllMacros...), PopulateMacroCalls(true), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				parsed, errs := p.Parse(common.NewTextSource("has(a.b)"))
				if len(errs.GetErrors()) > 0 {
					t.Fatalf("unexpected error: %s", errs.ToDisplayString())
				}
				macroCalls := parsed.SourceInfo().MacroCalls()
				if len(macroCalls) != 1 {
					t.Fatalf("expected 1 macro call, got %d", len(macroCalls))
				}
			})

			t.Run("ReceiverMacroCallRecorded", func(t *testing.T) {
				p, err := NewParser(Macros(AllMacros...), PopulateMacroCalls(true), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				parsed, errs := p.Parse(common.NewTextSource("[1, 2].exists(x, x > 0)"))
				if len(errs.GetErrors()) > 0 {
					t.Fatalf("unexpected error: %s", errs.ToDisplayString())
				}
				macroCalls := parsed.SourceInfo().MacroCalls()
				if len(macroCalls) != 1 {
					t.Fatalf("expected 1 macro call, got %d", len(macroCalls))
				}
			})

			t.Run("NestedMacroCallsRecorded", func(t *testing.T) {
				p, err := NewParser(Macros(AllMacros...), PopulateMacroCalls(true), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				parsed, errs := p.Parse(common.NewTextSource("[1, 2].all(x, has(x.b))"))
				if len(errs.GetErrors()) > 0 {
					t.Fatalf("unexpected error: %s", errs.ToDisplayString())
				}
				macroCalls := parsed.SourceInfo().MacroCalls()
				if len(macroCalls) != 2 {
					t.Fatalf("expected 2 macro calls, got %d", len(macroCalls))
				}
			})
		})
	}
}

func TestErrorRecoveryLimits(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			t.Run("LimitZero", func(t *testing.T) {
				p, err := NewParser(ErrorRecoveryLimit(0), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				_, errs := p.Parse(common.NewTextSource("......"))
				if len(errs.GetErrors()) == 0 {
					t.Errorf("expected error recovery limit error, got none")
				}
			})

			t.Run("LimitOne", func(t *testing.T) {
				p, err := NewParser(ErrorRecoveryLimit(1), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				_, errs := p.Parse(common.NewTextSource("......"))
				if len(errs.GetErrors()) == 0 {
					t.Errorf("expected error recovery limit error, got none")
				}
			})
		})
	}
}

func TestRecursionLimit(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			t.Run("DeeplyNestedBracketsLimitExceeded", func(t *testing.T) {
				p, err := NewParser(MaxRecursionDepth(5), EnablePrattParser(pratt))
				if err != nil {
					t.Fatalf("NewParser() failed: %v", err)
				}
				_, errs := p.Parse(common.NewTextSource("[[[[[[1]]]]]]"))
				if len(errs.GetErrors()) == 0 {
					t.Errorf("expected recursion limit error, got none")
				}
			})
		})
	}

	t.Run("PrattSequentialScopesDoNotAccumulateDepth", func(t *testing.T) {
		p, err := NewParser(MaxRecursionDepth(2), EnablePrattParser(true))
		if err != nil {
			t.Fatalf("NewParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("[1] + [2] + [3]"))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error on sequential scopes: %s", errs.ToDisplayString())
		}
	})

	t.Run("PrattIgnoreExtraParens", func(t *testing.T) {
		p, err := NewParser(MaxRecursionDepth(1), EnablePrattParser(true))
		if err != nil {
			t.Fatalf("NewParser() failed: %v", err)
		}
		_, errs := p.Parse(common.NewTextSource("((((1))))"))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error: %s", errs.ToDisplayString())
		}
	})

	t.Run("PrattDeeplyNestedParens1000", func(t *testing.T) {
		p, err := NewParser(MaxRecursionDepth(1), EnablePrattParser(true))
		if err != nil {
			t.Fatalf("NewParser() failed: %v", err)
		}
		expr1 := strings.Repeat("(", 1000) + "42" + strings.Repeat(")", 1000)
		_, errs := p.Parse(common.NewTextSource(expr1))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error on 1000 parens literal: %s", errs.ToDisplayString())
		}

		expr2 := strings.Repeat("(", 1000) + "1 + 2" + strings.Repeat(")", 1000)
		_, errs = p.Parse(common.NewTextSource(expr2))
		if len(errs.GetErrors()) > 0 {
			t.Errorf("unexpected error on 1000 parens binary: %s", errs.ToDisplayString())
		}
	})
}

func BenchmarkParse(b *testing.B) {
	p, err := NewParser(
		Macros(AllMacros...),
		MaxRecursionDepth(32),
		ErrorRecoveryLimit(4),
		ErrorRecoveryLookaheadTokenLimit(4),
		PopulateMacroCalls(true),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, testCase := range testCases {
			p.Parse(common.NewTextSource(testCase.I))
		}
	}
}

func BenchmarkParseParallel(b *testing.B) {
	p, err := NewParser(
		Macros(AllMacros...),
		MaxRecursionDepth(32),
		ErrorRecoveryLimit(4),
		ErrorRecoveryLookaheadTokenLimit(4),
		PopulateMacroCalls(true),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, testCase := range testCases {
				p.Parse(common.NewTextSource(testCase.I))
			}
		}
	})
}

type benchTestInfo struct {
	// I contains the input expression to be parsed.
	I string

	// E indicates whether an error is expected.
	E bool
}

type benchCategory struct {
	name  string
	cases []benchTestInfo
}

var benchCategories = []benchCategory{
	// Simple: common, representative CEL expressions covering basic syntax, operators, calls, and literals
	{
		name: "Simple",
		cases: []benchTestInfo{
			{
				I: "x * 2 + y / 3",
			},
			{
				I: `foo.bar.baz(1, 2, "abc")`,
			},
			{
				I: `a > 5 && b < 10 || c == "xyz"`,
			},
			{
				I: "x ? y : z",
			},
			{
				I: `{"foo": 1, "bar": [2, 3]}`,
			},
			{
				I: "a[b]",
			},
			{
				I: "a.b.c",
			},
			{
				I: "a.`b-c`",
			},
			{
				I: "\"\\a\\b\\f\\n\\r\\t\\v'\\\"\\\\ Legal escapes \\u2764\"",
			},
		},
	},

	// Complex: expressions with deep chaining, nesting, precedence, and complex structures
	{
		name: "Complex",
		cases: []benchTestInfo{
			{
				I: "a" + strings.Repeat(" + a", 49),
			},
			{
				I: "a" + strings.Repeat(" || a", 49),
			},
			{
				I: "a" + strings.Repeat(".f", 49),
			},
			{
				I: strings.Repeat("(", 20) + "a" + strings.Repeat(")", 20),
			},
			{
				I: `SomeMessage{foo: 5, bar: "xyz"}`,
			},
			{
				I: "1 + 2 * 3 - 1 / 2 == 6 % 1",
			},
			{
				I: "[] + [1, 2, 3] + [4]",
			},
		},
	},

	// Macros: standard and receiver comprehension macros, optional syntax traversal
	{
		name: "Macros",
		cases: []benchTestInfo{
			{
				I: "has(m.f)",
			},
			{
				I: "[1, 2, 3].all(x, x > 0)",
			},
			{
				I: "m.map(v, v * 2)",
			},
			{
				I: "m.filter(v, v > 0)",
			},
			{
				I: "m.exists_one(v, v == 1)",
			},
			{
				I: "x.filter(y, y.exists(z, has(z.a)))",
			},
			{
				I: "a.?b[?0] && a[?c]",
			},
			{
				I: "m.optMap(v, v + 1)",
			},
		},
	},

	// Errors: representative syntax errors, invalid tokens, keywords, and unclosed delimiters
	{
		name: "Errors",
		cases: []benchTestInfo{
			{
				I: "x * 2 + y /",
				E: true,
			},
			{
				I: `foo.bar.baz(1, 2, "abc"`,
				E: true,
			},
			{
				I: "a > 5 && && b < 10",
				E: true,
			},
			{
				I: `{"foo": 1, "bar": [2, 3`,
				E: true,
			},
			{
				I: "1 + $",
				E: true,
			},
			{
				I: "break",
				E: true,
			},
			{
				I: `"\xFh"`,
				E: true,
			},
			{
				I: "a" + strings.Repeat(" + a", 49) + " +",
				E: true,
			},
			{
				I: strings.Repeat("(", 20) + "a",
				E: true,
			},
			{
				I: "f(*" + strings.Repeat(", *", 9) + ")",
				E: true,
			},
		},
	},
}

// BenchmarkByCategory benchmarks parsing organized by workload categories.
func BenchmarkByCategory(b *testing.B) {
	for _, pratt := range []bool{false, true} {
		mode := "antlr"
		if pratt {
			mode = "pratt"
		}
		b.Run(mode, func(b *testing.B) {
			p := newBenchmarkCategoryParser(b, EnablePrattParser(pratt))
			for _, cat := range benchCategories {
				b.Run(cat.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						for _, tc := range cat.cases {
							src := common.NewTextSource(tc.I)
							_, errs := p.Parse(src)
							hasErr := len(errs.GetErrors()) > 0
							if hasErr != tc.E {
								b.Fatalf("p.Parse(%q) got error: %v, expected error: %v", tc.I, hasErr, tc.E)
							}
						}
					}
				})
			}
		})
	}
}

// BenchmarkParallelByCategory benchmarks parsing concurrently across goroutines by category.
func BenchmarkParallelByCategory(b *testing.B) {
	for _, pratt := range []bool{false, true} {
		mode := "antlr"
		if pratt {
			mode = "pratt"
		}
		b.Run(mode, func(b *testing.B) {
			p := newBenchmarkCategoryParser(b, EnablePrattParser(pratt))
			for _, cat := range benchCategories {
				b.Run(cat.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							for _, tc := range cat.cases {
								src := common.NewTextSource(tc.I)
								_, errs := p.Parse(src)
								hasErr := len(errs.GetErrors()) > 0
								if hasErr != tc.E {
									b.Fatalf("p.Parse(%q) got error: %v, expected error: %v", tc.I, hasErr, tc.E)
								}
							}
						}
					})
				})
			}
		})
	}
}

// optMapMacro expands `m.optMap(v, f)` into a conditional comprehension.
var optMapMacro = NewReceiverMacro("optMap", 2, optMapExpander)

func optMapExpander(meh ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	varIdent := args[0]
	varName := ""
	switch varIdent.Kind() {
	case ast.IdentKind:
		varName = varIdent.AsIdent()
	default:
		return nil, meh.NewError(varIdent.ID(), "optMap() variable name must be a simple identifier")
	}
	mapExpr := args[1]
	return meh.NewCall(
		operators.Conditional,
		meh.NewMemberCall("hasValue", target),
		meh.NewCall("optional.of",
			meh.NewComprehension(
				meh.NewList(),
				"#unused",
				varName,
				meh.NewMemberCall("value", meh.Copy(target)),
				meh.NewLiteral(types.False),
				meh.NewIdent(varName),
				mapExpr,
			),
		),
		meh.NewCall("optional.none"),
	), nil
}

func newBenchmarkCategoryParser(tb testing.TB, options ...Option) *Parser {
	tb.Helper()
	opts := append([]Option{
		Macros(append(AllMacros, optMapMacro)...),
		EnableOptionalSyntax(true),
		EnableIdentEscapeSyntax(true),
		MaxRecursionDepth(512),
	}, options...)
	p, err := NewParser(opts...)
	if err != nil {
		tb.Fatalf("NewParser() failed: %v", err)
	}
	return p
}

func TestParseErrorData(t *testing.T) {
	for _, pratt := range []bool{false, true} {
		t.Run(fmt.Sprintf("enablePrattParser=%t", pratt), func(t *testing.T) {
			p := newTestParser(t, EnablePrattParser(pratt))
			src := common.NewTextSource(`a.?b`)
			_, iss := p.Parse(src)
			if len(iss.GetErrors()) != 1 {
				t.Fatalf("Check() of a bad expression did produce a single error: %v", iss.ToDisplayString())
			}
			celErr := iss.GetErrors()[0]
			if celErr.ExprID != 2 {
				t.Errorf("got exprID %v, wanted 2", celErr.ExprID)
			}
			if !strings.Contains(celErr.Message, "unsupported syntax") {
				t.Errorf("got message %v, wanted unsupported syntax", celErr.Message)
			}
		})
	}
}

func newTestParser(t *testing.T, options ...Option) *Parser {
	t.Helper()
	defaultOpts := []Option{
		Macros(AllMacros...),
		MaxRecursionDepth(32),
		ErrorRecoveryLimit(4),
		ErrorRecoveryLookaheadTokenLimit(4),
		PopulateMacroCalls(true),
	}
	opts := append([]Option{}, defaultOpts...)
	opts = append(opts, options...)
	p, err := NewParser(opts...)
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	return p
}

// RelativeSource represents an embedded source element within a larger source.
type RelativeSource struct {
	common.Source
	localSrc common.Source
	absLoc   common.Location
}

// Content returns the embedded source snippet.
func (rel *RelativeSource) Content() string {
	return rel.localSrc.Content()
}

// OffsetLocation returns the absolute location given the relative offset, if found.
func (rel *RelativeSource) OffsetLocation(offset int32) (common.Location, bool) {
	absOffset, found := rel.Source.LocationOffset(rel.absLoc)
	if !found {
		return common.NoLocation, false
	}
	return rel.Source.OffsetLocation(absOffset + offset)
}
