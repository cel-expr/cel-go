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

package checker

import (
	"math"
	"strings"
	"testing"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func TestCost(t *testing.T) {
	allTypes := types.NewObjectType("google.expr.proto3.test.TestAllTypes")
	allList := types.NewListType(allTypes)
	intList := types.NewListType(types.IntType)
	nestedList := types.NewListType(allList)

	allMap := types.NewMapType(types.StringType, allTypes)
	nestedMap := types.NewMapType(types.StringType, allMap)

	zeroCost := CostEstimate{}
	oneCost := FixedCostEstimate(1)
	cases := []struct {
		name    string
		expr    string
		vars    []*decls.VariableDecl
		hints   map[string]uint64
		options []CostOption
		wanted  CostEstimate
	}{
		{
			name:   "const",
			expr:   `"Hello World!"`,
			wanted: zeroCost,
		},
		{
			name:   "identity",
			expr:   `input`,
			vars:   []*decls.VariableDecl{decls.NewVariable("input", intList)},
			wanted: CostEstimate{Min: 1, Max: 1},
		},
		{
			name: "select: map",
			expr: `input['key']`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.NewMapType(types.StringType, types.StringType))},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:   "select: field",
			expr:   `input.single_int32`,
			vars:   []*decls.VariableDecl{decls.NewVariable("input", allTypes)},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:    "select: field test only no has() cost",
			expr:    `has(input.single_int32)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			wanted:  CostEstimate{Min: 1, Max: 1},
			options: []CostOption{PresenceTestHasCost(false)},
		},
		{
			name:   "select: field test only",
			expr:   `has(input.single_int32)`,
			vars:   []*decls.VariableDecl{decls.NewVariable("input", types.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:    "select: non-proto field test has() cost",
			expr:    `has(input.testAttr.nestedAttr)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			wanted:  CostEstimate{Min: 3, Max: 3},
			options: []CostOption{PresenceTestHasCost(true)},
		},
		{
			name:    "select: non-proto field test no has() cost",
			expr:    `has(input.testAttr.nestedAttr)`,
			vars:    []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			wanted:  CostEstimate{Min: 2, Max: 2},
			options: []CostOption{PresenceTestHasCost(false)},
		},
		{
			name:   "select: non-proto field test",
			expr:   `has(input.testAttr.nestedAttr)`,
			vars:   []*decls.VariableDecl{decls.NewVariable("input", nestedMap)},
			wanted: CostEstimate{Min: 3, Max: 3},
		},
		{
			name:   "estimated function call",
			expr:   `input.getFullYear()`,
			vars:   []*decls.VariableDecl{decls.NewVariable("input", types.TimestampType)},
			wanted: CostEstimate{Min: 8, Max: 8},
		},
		{
			name:   "create list",
			expr:   `[1, 2, 3]`,
			wanted: CostEstimate{Min: 10, Max: 10},
		},
		{
			name:   "create struct",
			expr:   `google.expr.proto3.test.TestAllTypes{single_int32: 1, single_float: 3.14, single_string: 'str'}`,
			wanted: CostEstimate{Min: 40, Max: 40},
		},
		{
			name:   "create map",
			expr:   `{"a": 1, "b": 2, "c": 3}`,
			wanted: CostEstimate{Min: 30, Max: 30},
		},
		{
			name:   "all comprehension",
			vars:   []*decls.VariableDecl{decls.NewVariable("input", allList)},
			hints:  map[string]uint64{"input": 100},
			expr:   `input.all(x, true)`,
			wanted: CostEstimate{Min: 2, Max: 302},
		},
		{
			name:   "nested all comprehension",
			vars:   []*decls.VariableDecl{decls.NewVariable("input", nestedList)},
			hints:  map[string]uint64{"input": 50, "input.@items": 10},
			expr:   `input.all(x, x.all(y, true))`,
			wanted: CostEstimate{Min: 2, Max: 1752},
		},
		{
			name:   "all comprehension on literal",
			expr:   `[1, 2, 3].all(x, true)`,
			wanted: CostEstimate{Min: 20, Max: 20},
		},
		{
			name:   "variable cost function",
			vars:   []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			hints:  map[string]uint64{"input": 500},
			expr:   `input.matches('[0-9]')`,
			wanted: CostEstimate{Min: 3, Max: 103},
		},
		{
			name:   "variable cost function with constant",
			expr:   `'123'.matches('[0-9]')`,
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:   "or",
			expr:   `true || false`,
			wanted: zeroCost,
		},
		{
			name: "or accumulated branch cost",
			expr: `a || b || c || d`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.BoolType),
				decls.NewVariable("b", types.BoolType),
				decls.NewVariable("c", types.BoolType),
				decls.NewVariable("d", types.BoolType),
			},
			wanted: CostEstimate{Min: 1, Max: 4},
		},
		{
			name:   "and",
			expr:   `true && false`,
			wanted: zeroCost,
		},
		{
			name: "and accumulated branch cost",
			expr: `a && b && c && d`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.BoolType),
				decls.NewVariable("b", types.BoolType),
				decls.NewVariable("c", types.BoolType),
				decls.NewVariable("d", types.BoolType),
			},
			wanted: CostEstimate{Min: 1, Max: 4},
		},
		{
			name:   "lt",
			expr:   `1 < 2`,
			wanted: oneCost,
		},
		{
			name:   "lte",
			expr:   `1 <= 2`,
			wanted: oneCost,
		},
		{
			name:   "eq",
			expr:   `1 == 2`,
			wanted: oneCost,
		},
		{
			name:   "gt",
			expr:   `2 > 1`,
			wanted: oneCost,
		},
		{
			name:   "gte",
			expr:   `2 >= 1`,
			wanted: oneCost,
		},
		{
			name:   "in",
			expr:   `2 in [1, 2, 3]`,
			wanted: CostEstimate{Min: 13, Max: 13},
		},
		{
			name:   "plus",
			expr:   `1 + 1`,
			wanted: oneCost,
		},
		{
			name:   "minus",
			expr:   `1 - 1`,
			wanted: oneCost,
		},
		{
			name:   "/",
			expr:   `1 / 1`,
			wanted: oneCost,
		},
		{
			name:   "/",
			expr:   `1 * 1`,
			wanted: oneCost,
		},
		{
			name:   "%",
			expr:   `1 % 1`,
			wanted: oneCost,
		},
		{
			name:   "ternary",
			expr:   `true ? 1 : 2`,
			wanted: zeroCost,
		},
		{
			name:   "string size",
			expr:   `size("123")`,
			wanted: oneCost,
		},
		{
			name:   "bytes size",
			expr:   `size(b"123")`,
			wanted: oneCost,
		},
		{
			name:   "bytes to string conversion",
			vars:   []*decls.VariableDecl{decls.NewVariable("input", types.BytesType)},
			hints:  map[string]uint64{"input": 500},
			expr:   `string(input)`,
			wanted: CostEstimate{Min: 1, Max: 51},
		},
		{
			name:  "bytes to string conversion equality",
			vars:  []*decls.VariableDecl{decls.NewVariable("input", types.BytesType)},
			hints: map[string]uint64{"input": 500},
			// equality check ensures that the resultSize calculation is included in cost
			expr:   `string(input) == string(input)`,
			wanted: CostEstimate{Min: 3, Max: 152},
		},
		{
			name:   "string to bytes conversion",
			vars:   []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			hints:  map[string]uint64{"input": 500},
			expr:   `bytes(input)`,
			wanted: CostEstimate{Min: 1, Max: 51},
		},
		{
			name:  "string to bytes conversion equality",
			vars:  []*decls.VariableDecl{decls.NewVariable("input", types.StringType)},
			hints: map[string]uint64{"input": 500},
			// equality check ensures that the resultSize calculation is included in cost
			expr:   `bytes(input) == bytes(input)`,
			wanted: CostEstimate{Min: 3, Max: 302},
		},
		{
			name:   "int to string conversion",
			expr:   `string(1)`,
			wanted: CostEstimate{Min: 1, Max: 1},
		},
		{
			name: "contains",
			expr: `input.contains(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			hints:  map[string]uint64{"input": 500, "arg1": 500},
			wanted: CostEstimate{Min: 2, Max: 2502},
		},
		{
			name: "matches",
			expr: `input.matches('\\d+a\\d+b')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			hints:  map[string]uint64{"input": 500},
			wanted: CostEstimate{Min: 3, Max: 103},
		},
		{
			name: "matches global",
			expr: `matches(input, '\\d+a\\d+b')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			hints:  map[string]uint64{"input": 500},
			wanted: CostEstimate{Min: 3, Max: 103},
		},
		{
			name: "startsWith",
			expr: `input.startsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			hints:  map[string]uint64{"arg1": 500},
			wanted: CostEstimate{Min: 2, Max: 52},
		},
		{
			name: "endsWith",
			expr: `input.endsWith(arg1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
				decls.NewVariable("arg1", types.StringType),
			},
			hints:  map[string]uint64{"arg1": 500},
			wanted: CostEstimate{Min: 2, Max: 52},
		},
		{
			name: "size receiver",
			expr: `input.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name: "size",
			expr: `size(input)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", types.StringType),
			},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name: "ternary eval",
			expr: `(x > 2 ? input1 : input2).all(y, true)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("input1", allList),
				decls.NewVariable("input2", allList),
			},
			hints:  map[string]uint64{"input1": 1, "input2": 1},
			wanted: CostEstimate{Min: 4, Max: 7},
		},
		{
			name: "comprehension over map",
			expr: `input.all(k, input[k].single_int32 > 3)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", allMap),
			},
			hints:  map[string]uint64{"input": 10},
			wanted: CostEstimate{Min: 2, Max: 82},
		},
		{
			name: "comprehension over nested map of maps",
			expr: `input.all(k, input[k].all(x, true))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:  map[string]uint64{"input": 5, "input.@values": 10},
			wanted: CostEstimate{Min: 2, Max: 187},
		},
		{
			name: "string size of map keys",
			expr: `input.all(k, k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:  map[string]uint64{"input": 5, "input.@keys": 10},
			wanted: CostEstimate{Min: 2, Max: 32},
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:  map[string]uint64{"input": 2, "input.@values": 2, "input.@keys": 5},
			wanted: CostEstimate{Min: 2, Max: 34},
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("input", nestedMap),
			},
			hints:  map[string]uint64{"input": 2, "input.@values": 2, "input.@keys": 5},
			wanted: CostEstimate{Min: 2, Max: 34},
		},
		{
			name: "list concat",
			expr: `(list1 + list2).all(x, true)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			hints:  map[string]uint64{"list1": 10, "list2": 10},
			wanted: CostEstimate{Min: 4, Max: 64},
		},
		{
			name: "str concat",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			hints:  map[string]uint64{"str1": 10, "str2": 10},
			wanted: CostEstimate{Min: 2, Max: 6},
		},
		{
			name: "str concat custom cost estimate",
			expr: `"abcdefg".contains(str1 + str2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			hints: map[string]uint64{"str1": 10, "str2": 10},
			options: []CostOption{
				OverloadCostEstimate(overloads.ContainsString,
					func(estimator CostEstimator, target *AstNode, args []AstNode) *CallEstimate {
						if target != nil && len(args) == 1 {
							strSize := estimateSize(estimator, *target).MultiplyByCostFactor(0.2)
							subSize := estimateSize(estimator, args[0]).MultiplyByCostFactor(0.2)
							return &CallEstimate{CostEstimate: strSize.Multiply(subSize)}
						}
						return nil
					}),
			},
			wanted: CostEstimate{Min: 2, Max: 12},
		},
		{
			name: "list size comparison",
			expr: `list1.size() == list2.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			wanted: CostEstimate{Min: 5, Max: 5},
		},
		{
			name: "list size from ternary",
			expr: `x > y ? list1.size() : list2.size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("y", types.IntType),
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			wanted: CostEstimate{Min: 5, Max: 5},
		},
		{
			name: "list size from concat",
			expr: `([x, y] + list1 + list2).size()`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("x", types.IntType),
				decls.NewVariable("y", types.IntType),
				decls.NewVariable("list1", types.NewListType(types.IntType)),
				decls.NewVariable("list2", types.NewListType(types.IntType)),
			},
			hints: map[string]uint64{
				"list1": 10,
				"list2": 20,
			},
			wanted: CostEstimate{Min: 17, Max: 17},
		},
		{
			name: "list cost tracking through comprehension",
			expr: `[list1, list2].exists(l, l.exists(v, v.startsWith('hi')))`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("list1", types.NewListType(types.StringType)),
				decls.NewVariable("list2", types.NewListType(types.StringType)),
			},
			hints: map[string]uint64{
				"list1":        10,
				"list1.@items": 64,
				"list2":        20,
				"list2.@items": 128,
			},
			wanted: CostEstimate{Min: 21, Max: 265},
		},
		{
			name: "str endsWith equality",
			expr: `str1.endsWith("abcdefghijklmnopqrstuvwxyz") == str2.endsWith("abcdefghijklmnopqrstuvwxyz")`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("str1", types.StringType),
				decls.NewVariable("str2", types.StringType),
			},
			wanted: CostEstimate{Min: 9, Max: 9},
		},
		{
			name:   "nested subexpression operators",
			expr:   `((5 != 6) == (1 == 2)) == ((3 <= 4) == (9 != 9))`,
			wanted: CostEstimate{Min: 7, Max: 7},
		},
		{
			name: "str size estimate",
			expr: `string(timestamp1) == string(timestamp2)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("timestamp1", types.TimestampType),
				decls.NewVariable("timestamp2", types.TimestampType),
			},
			wanted: CostEstimate{Min: 5, Max: 1844674407370955268},
		},
		{
			name: "timestamp equality check",
			expr: `timestamp1 == timestamp2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("timestamp1", types.TimestampType),
				decls.NewVariable("timestamp2", types.TimestampType),
			},
			wanted: CostEstimate{Min: 3, Max: 3},
		},
		{
			name: "duration inequality check",
			expr: `duration1 != duration2`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("duration1", types.DurationType),
				decls.NewVariable("duration2", types.DurationType),
			},
			wanted: CostEstimate{Min: 3, Max: 3},
		},
		{
			name:   ".filter list literal",
			expr:   `[1,2,3,4,5].filter(x, x % 2 == 0)`,
			wanted: CostEstimate{Min: 41, Max: 101},
		},
		{
			name:   ".map list literal",
			expr:   `[1,2,3,4,5].map(x, x)`,
			wanted: CostEstimate{Min: 86, Max: 86},
		},
		{
			name:   ".map.filter list literal",
			expr:   `[1,2,3,4,5].map(x, x).filter(x, x % 2 == 0)`,
			wanted: CostEstimate{Min: 117, Max: 177},
		},
		{
			name:   ".map.exists list literal",
			expr:   `[1,2,3,4,5].map(x, x).exists(x, x == 5) == true`,
			wanted: CostEstimate{Min: 108, Max: 118},
		},
		{
			name:   ".map.map list literal",
			expr:   `[1,2,3,4,5].map(x, x).map(x, x)`,
			wanted: CostEstimate{Min: 162, Max: 162},
		},
		{
			name:   ".map list literal selection",
			expr:   `[1,2,3,4,5].map(x, x)[4]`,
			wanted: CostEstimate{Min: 87, Max: 87},
		},
		{
			name:   "nested array selection",
			expr:   `[[1,2],[1,2],[1,2],[1,2],[1,2]][4]`,
			wanted: CostEstimate{Min: 61, Max: 61},
		},
		{
			name:   "nested map selection",
			expr:   `{'a': [1,2], 'b': [1,2], 'c': [1,2], 'd': [1,2], 'e': [1,2]}.b`,
			wanted: CostEstimate{Min: 81, Max: 81},
		},
		{
			name:   "comprehension on nested list",
			expr:   `[[1, 1], [2, 2], [3, 3], [4, 4], [5, 5]].all(y, y.all(y, y == 1))`,
			wanted: CostEstimate{Min: 76, Max: 136},
		},
		{
			name:   "comprehension on transformed nested list",
			expr:   `[1,2,3,4,5].map(x, [x, x]).all(y, y.all(y, y == 1))`,
			wanted: CostEstimate{Min: 157, Max: 217},
		},
		{
			name:   "comprehension on nested literal list",
			expr:   `["a", "ab", "abc", "abcd", "abcde"].map(x, [x, x]).all(y, y.all(y, y.startsWith('a')))`,
			wanted: CostEstimate{Min: 157, Max: 217},
		},
		{
			name: "comprehension on nested variable list",
			expr: `input.map(x, [x, x]).all(y, y.all(y, y.startsWith('a')))`,
			vars: []*decls.VariableDecl{decls.NewVariable("input", types.NewListType(types.StringType))},
			hints: map[string]uint64{
				"input":        5,
				"input.@items": 10,
			},
			wanted: CostEstimate{Min: 13, Max: 208},
		},
		{
			name:   "comprehension chaining with concat",
			expr:   `[1,2,3,4,5].map(x, x).map(x, x) + [1]`,
			wanted: CostEstimate{Min: 173, Max: 173},
		},
		{
			name:   "nested comprehension",
			expr:   `[1,2,3].all(i, i in [1,2,3].map(j, j + j))`,
			wanted: CostEstimate{Min: 20, Max: 230},
		},
		{
			name:   "nested dyn comprehension",
			expr:   `dyn([1,2,3]).all(i, i in dyn([1,2,3]).map(j, j + j))`,
			wanted: CostEstimate{Min: 21, Max: 234},
		},
		{
			name:   "literal map access",
			expr:   `{'hello': 'hi'}['hello'] != {'hello': 'bye'}['hello']`,
			wanted: CostEstimate{Min: 63, Max: 63},
		},
		{
			name:   "literal list access",
			expr:   `['hello', 'hi'][0] != ['hello', 'bye'][1]`,
			wanted: CostEstimate{Min: 23, Max: 23},
		},
		{
			name:   "type call",
			expr:   `type(1)`,
			wanted: CostEstimate{Min: 1, Max: 1},
		},
		{
			name: "type call variable",
			expr: `type(self.val1)`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.IntType)),
			},
			wanted: CostEstimate{Min: 3, Max: 3},
		},
		{
			name: "type call variable equality",
			expr: `type(self.val1) == int`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.IntType)),
			},
			wanted: CostEstimate{Min: 5, Max: 1844674407370955268},
		},
		{
			name:   "type literal equality cost",
			expr:   `type(1) == int`,
			wanted: CostEstimate{Min: 3, Max: 1844674407370955266},
		},
		{
			name:   "type variable equality cost",
			expr:   `type(1) == int`,
			wanted: CostEstimate{Min: 3, Max: 1844674407370955266},
		},
		{
			name: "namespace variable equality",
			expr: `self.val1 == 1.0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self.val1", types.DoubleType),
			},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name: "simple map variable equality",
			expr: `self.val1 == 1.0`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.DoubleType)),
			},
			wanted: CostEstimate{Min: 3, Max: 3},
		},
		{
			name: "date-time math",
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.TimestampType)),
			},
			expr:   `self.val1 == timestamp('2011-08-18T00:00:00.000+01:00') + duration('19h3m37s10ms')`,
			wanted: FixedCostEstimate(6),
		},
		{
			name: "date-time math self-conversion",
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.TimestampType)),
			},
			expr:   `timestamp(self.val1) == timestamp('2011-08-18T00:00:00.000+01:00') + duration('19h3m37s10ms')`,
			wanted: FixedCostEstimate(7),
		},
		{
			name: "boolean vars equal",
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.BoolType)),
			},
			expr:   `self.val1 != self.val2`,
			wanted: FixedCostEstimate(5),
		},
		{
			name: "boolean var equals literal",
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.BoolType)),
			},
			expr:   `self.val1 != true`,
			wanted: FixedCostEstimate(3),
		},
		{
			name: "double var equals literal",
			vars: []*decls.VariableDecl{
				decls.NewVariable("self", types.NewMapType(types.StringType, types.DoubleType)),
			},
			expr:   `self.val1 == 1.0`,
			wanted: FixedCostEstimate(3),
		},
		{
			name: "bytes list max",
			expr: "[bytes('012345678901'), bytes('012345678901'), bytes('012345678901'), bytes('012345678901'), bytes('012345678901')].max()",
			options: []CostOption{
				OverloadCostEstimate("list_bytes_max",
					func(estimator CostEstimator, target *AstNode, args []AstNode) *CallEstimate {
						if target != nil {
							// Charge 1 cost for comparing each element in the list
							elCost := CostEstimate{Min: 1, Max: 1}
							// If the list contains strings or bytes, add the cost of traversing all the strings/bytes as a way
							// of estimating the additional comparison cost.
							if elNode := listElementNode(*target); elNode != nil {
								k := elNode.Type().Kind()
								if k == types.StringKind || k == types.BytesKind {
									sz := sizeEstimate(estimator, elNode)
									elCost = elCost.Add(sz.MultiplyByCostFactor(common.StringTraversalCostFactor))
								}
								return &CallEstimate{CostEstimate: sizeEstimate(estimator, *target).MultiplyByCost(elCost)}
							}
						}
						return nil
					}),
			},
			wanted: CostEstimate{Min: 25, Max: 35},
		},
	}

	for _, tst := range cases {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			if tc.hints == nil {
				tc.hints = map[string]uint64{}
			}
			p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
			if err != nil {
				t.Fatalf("parser.NewParser() failed: %v", err)
			}
			src := common.NewStringSource(tc.expr, "<input>")
			pe, errs := p.Parse(src)
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("parser.Parse(%v) failed: %v", tc.expr, errs.ToDisplayString())
			}
			reg, err := types.NewProtoRegistry(types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
			if err != nil {
				t.Fatalf("types.NewProtoRegistry(...) failed: %v", err)
			}

			e, err := NewEnv(containers.DefaultContainer, reg)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			err = e.AddFunctions(stdlib.Functions()...)
			if err != nil {
				t.Fatalf("environment creation error: %v", err)
			}
			maxFunc, _ := decls.NewFunction("max",
				decls.MemberOverload("list_bytes_max",
					[]*types.Type{types.NewListType(types.BytesType)},
					types.BytesType))
			err = e.AddFunctions(maxFunc)
			if err != nil {
				t.Fatalf("environment creation error: %v", err)
			}
			err = e.AddIdents(tc.vars...)
			if err != nil {
				t.Fatalf("environment creation error: %s\n", err)
			}
			checked, errs := Check(pe, src, e)
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("Check(%s) failed: %v", tc.expr, errs.ToDisplayString())
			}
			est, err := Cost(checked, testCostEstimator{hints: tc.hints}, tc.options...)
			if err != nil {
				t.Fatalf("Cost() failed: %v", err)
			}
			if est.Min != tc.wanted.Min || est.Max != tc.wanted.Max {
				t.Fatalf("Got cost interval [%v, %v], wanted [%v, %v]",
					est.Min, est.Max, tc.wanted.Min, tc.wanted.Max)
			}
		})
	}
}

type testCostEstimator struct {
	hints map[string]uint64
}

func (tc testCostEstimator) EstimateSize(element AstNode) *SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &SizeEstimate{Min: 0, Max: l}
	}
	if element.Type() == types.BytesType {
		return &SizeEstimate{Min: 0, Max: 12}
	}
	return nil
}

func (tc testCostEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	switch overloadID {
	case overloads.TimestampToYear:
		return &CallEstimate{CostEstimate: CostEstimate{Min: 7, Max: 7}}
	}
	return nil
}

func estimateSize(estimator CostEstimator, node AstNode) SizeEstimate {
	if l := node.ComputedSize(); l != nil {
		return *l
	}
	if l := estimator.EstimateSize(node); l != nil {
		return *l
	}
	return SizeEstimate{Min: 0, Max: math.MaxUint64}
}

func listElementNode(list AstNode) AstNode {
	if params := list.Type().Parameters(); len(params) > 0 {
		lt := params[0]
		nodePath := list.Path()
		if nodePath != nil {
			// Provide path if we have it so that a OpenAPIv3 maxLength validation can be looked up, if it exists
			// for this node.
			path := make([]string, len(nodePath)+1)
			copy(path, nodePath)
			path[len(nodePath)] = "@items"
			return &astNode{path: path, t: lt, expr: nil}
		} else {
			// Provide just the type if no path is available so that worst case size can be looked up based on type.
			return &astNode{t: lt, expr: nil}
		}
	}
	return nil
}

func sizeEstimate(estimator CostEstimator, t AstNode) SizeEstimate {
	if sz := t.ComputedSize(); sz != nil {
		return *sz
	}
	if sz := estimator.EstimateSize(t); sz != nil {
		return *sz
	}
	return SizeEstimate{Min: 0, Max: math.MaxUint64}
}
