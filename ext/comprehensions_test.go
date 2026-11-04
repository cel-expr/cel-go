// Copyright 2024 Google LLC
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
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestTwoVarComprehensions(t *testing.T) {
	compreTests := []struct {
		expr string
	}{
		// list.all()
		{expr: "[1, 2, 3, 4].all(i, v, i < 5 && v > 0)"},
		{expr: "[1, 2, 3, 4].all(i, v, i < v)"},
		{expr: "[1, 2, 3, 4].all(i, v, i > v) == false"},
		{expr: `
		cel.bind(listA, [1, 2, 3, 4],
		cel.bind(listB, [1, 2, 3, 4, 5],
		   listA.all(i, v, listB[?i].hasValue() && listB[i] == v)
		))
		`},
		{expr: `
		cel.bind(listA, [1, 2, 3, 4, 5, 6],
		cel.bind(listB, [1, 2, 3, 4, 5],
		   listA.all(i, v, listB[?i].hasValue() && listB[i] == v)
		)) == false
		`},
		// list.exists()
		{expr: `
		cel.bind(l, ['hello', 'world', 'hello!', 'worlds'],
		  l.exists(i, v,
		    v.startsWith('hello') && l[?(i+1)].optMap(next, next.endsWith('world')).orValue(false)
		  )
		)
		`},
		// list.existsOne()
		{expr: `
		cel.bind(l, ['hello', 'world', 'hello!', 'worlds'],
		  l.existsOne(i, v,
		    v.startsWith('hello') && l[?(i+1)].optMap(next, next.endsWith('world')).orValue(false)
		  )
		)
		`},
		{expr: `
		cel.bind(l, ['hello', 'goodbye', 'hello!', 'goodbye'],
		  l.exists_one(i, v,
		    v.startsWith('hello') && l[?(i+1)].optMap(next, next == "goodbye").orValue(false)
		  )
		) == false
		`},
		// list.transformList()
		{expr: `
		['Hello', 'world'].transformList(i, v, "[%d]%s".format([i, v.lowerAscii()])) == ["[0]hello", "[1]world"]
		`},
		{expr: `
		['hello', 'world'].transformList(i, v, v.startsWith('greeting'), "[%d]%s".format([i, v])) == []
		`},
		{expr: `
		[1, 2, 3].transformList(indexVar, valueVar, (indexVar * valueVar) + valueVar) == [1, 4, 9]
		`},
		{expr: `
		[1, 2, 3].transformList(indexVar, valueVar, indexVar % 2 == 0, (indexVar * valueVar) + valueVar) == [1, 9]
		`},
		// list.transformMap()
		{expr: `
		['Hello', 'world'].transformMap(i, v, [v.lowerAscii()]) == {0: ['hello'], 1: ['world']}
		`},
		{expr: `
		// round-tripping example
		['world', 'Hello'].transformMap(i, v, [v.lowerAscii()])
		  .transformList(k, v, v) // extract the list back form the map
		  .flatten()
		  .sort() == ['hello', 'world']
		`},
		{expr: `
		[1, 2, 3].transformMap(indexVar, valueVar,
	      (indexVar * valueVar) + valueVar) == {0: 1, 1: 4, 2: 9}
        `},
		{expr: `
		[1, 2, 3].transformMap(indexVar, valueVar, indexVar % 2 == 0,
	  	  (indexVar * valueVar) + valueVar) == {0: 1, 2: 9}
		`},
		// list.transformMapEntry()
		{expr: `
		"key1:value1 key2:value2 key3:value3".split(" ")
		.transformMapEntry(i, v,
		  cel.bind(entry, v.split(":"),
		    entry.size() == 2 ? {entry[0]: entry[1]} : {}
		  )
		) == {'key1': 'value1', 'key2': 'value2', 'key3': 'value3'}
		`},
		{expr: `
		"key1:value1:extra key2:value2 key3".split(" ")
		.transformMapEntry(i, v,
		  cel.bind(entry, v.split(":"), {?entry[0]: entry[?1]})
		) == {'key1': 'value1', 'key2': 'value2'}
		`},
		// map.all()
		{expr: `
		{'hello': 'world', 'hello!': 'world'}.all(k, v, k.startsWith('hello') && v == 'world')
		`},
		{expr: `
		{'hello': 'world', 'hello!': 'worlds'}.all(k, v, k.startsWith('hello') && v.endsWith('world')) == false
		`},
		// map.exists()
		{expr: `
		{'hello': 'world', 'hello!': 'worlds'}.exists(k, v, k.startsWith('hello') && v.endsWith('world'))
		`},
		// map.existsOne()
		{expr: `
		{'hello': 'world', 'hello!': 'worlds'}.existsOne(k, v, k.startsWith('hello') && v.endsWith('world'))
		`},
		// map.exists_one()
		{expr: `
		{'hello': 'world', 'hello!': 'worlds'}.exists_one(k, v, k.startsWith('hello') && v.endsWith('world'))
		`},
		{expr: `
		{'hello': 'world', 'hello!': 'wow, world'}.exists_one(k, v, k.startsWith('hello') && v.endsWith('world')) == false
		`},
		// map.transformList()
		{expr: `
		{'Hello': 'world'}.transformList(k, v, "%s=%s".format([k.lowerAscii(), v])) == ["hello=world"]
		`},
		{expr: `
		dyn({'Hello': 'world'}).transformList(k, v, "%s=%s".format([k.lowerAscii(), v])) == ["hello=world"]
		`},
		{expr: `
		{'hello': 'world'}.transformList(k, v, k.startsWith('greeting'), "%s=%s".format([k, v])) == []
		`},
		{expr: `
		{'greeting': 'hello', 'farewell': 'goodbye'}
		  .transformList(k, _, k).sort() == ['farewell', 'greeting']
		`},
		{expr: `
		{'greeting': 'hello', 'farewell': 'goodbye'}
		  .transformList(_, v, v).sort() == ['goodbye', 'hello']
		`},
		// map.transformMap()
		{expr: `
		{'hello': 'world', 'goodbye': 'cruel world'}.transformMap(k, v, "%s, %s!".format([k, v]))
		   == {'hello': 'hello, world!', 'goodbye': 'goodbye, cruel world!'}
		`},
		{expr: `
		dyn({'hello': 'world', 'goodbye': 'cruel world'}).transformMap(k, v, "%s, %s!".format([k, v]))
		   == {'hello': 'hello, world!', 'goodbye': 'goodbye, cruel world!'}
		`},
		{expr: `
		{'hello': 'world', 'goodbye': 'cruel world'}.transformMap(k, v, v.startsWith('world'), "%s, %s!".format([k, v]))
		   == {'hello': 'hello, world!'}
		`},
		// map.transformMapEntry()
		{expr: `
		{'hello': 'world', 'greetings': 'tacocat'}.transformMapEntry(k, v, {k.reverse(): v.reverse()})
		   == {'olleh': 'dlrow', 'sgniteerg': 'tacocat'}
		`},
		{expr: `
		{'hello': 'world', 'greetings': 'tacocat'}.transformMapEntry(k, v, v.reverse() == v, {k.reverse(): v.reverse()})
		   == {'sgniteerg': 'tacocat'}
		`},
		{expr: `
		{'hello': 'world', 'greetings': 'tacocat'}.transformMapEntry(k, v, {}) == {}
		`},
	}

	env := testCompreEnv(t)
	for i, tst := range compreTests {
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
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out.Value() != true {
					t.Errorf("prg.Eval() got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestTwoVarComprehensionsCost(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		vars          []cel.EnvOption
		in            map[string]any
		hints         map[string]uint64
		estimatedCost cost.CostEstimate
		// estimatedCostV0 is the estimate under cost.ModelVersion0, set only where the revision moved it.
		estimatedCostV0 *cost.CostEstimate
		actualCost      uint64
		// actualCostV0 is the tracked cost under cost.ModelVersion0, set only where the revision moved it.
		actualCostV0 *uint64
	}{
		{
			name:          "all list literal",
			expr:          `[1, 2, 3, 4].all(i, v, i < 5 && v > 0)`,
			estimatedCost: cost.RangedCostEstimate(23, 39),
			actualCost:    39,
		},
		{
			name:          "all map literal - true",
			expr:          `{1: 1, 2: 2, 3: 3}.all(i, v, i < 5 && v > 0)`,
			estimatedCost: cost.RangedCostEstimate(40, 52),
			actualCost:    52,
		},
		{
			name:          "all map literal - false",
			expr:          `!{0: 0}.all(i, v, i < 5 && v > 0)`,
			estimatedCost: cost.RangedCostEstimate(35, 39),
			actualCost:    39,
		},
		{
			name: "all map(int,int) variable",
			expr: `m.all(i, v, i < 5 && v > 0)`,
			vars: []cel.EnvOption{cel.Variable("m", cel.MapType(cel.IntType, cel.IntType))},
			hints: map[string]uint64{
				"m": 3,
			},
			in: map[string]any{
				"m": map[int]int{1: 1, 2: 2},
			},
			estimatedCost: cost.RangedCostEstimate(2, 23),
			actualCost:    16,
		},
		{
			name: "all map(string,string) variable",
			expr: `m.all(k, v, k < v)`,
			vars: []cel.EnvOption{cel.Variable("m", cel.MapType(cel.StringType, cel.StringType))},
			hints: map[string]uint64{
				"m":         3,
				"m.@keys":   16,
				"m.@values": 128,
			},
			in: map[string]any{
				"m": map[string]string{"he": "hello", "go": "goodbye"},
			},
			estimatedCost: cost.RangedCostEstimate(2, 23),
			actualCost:    14,
		},
		{
			name:            "transformList empty",
			expr:            `[].transformList(i, v, v) == []`,
			estimatedCost:   cost.FixedCostEstimate(32),
			estimatedCostV0: costV0(31, 31),
			actualCost:      32,
			actualCostV0:    actualV0(31),
		},
		{
			name:          "transformList single element",
			expr:          `[1].transformList(i, v, i) == [0]`,
			estimatedCost: cost.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			name:            "transformList with filter",
			expr:            `[3, 2, 1].transformList(i, v, v > i, v) == [3, 2]`,
			estimatedCost:   cost.RangedCostEstimate(41, 80),
			estimatedCostV0: costV0(44, 80),
			actualCost:      67,
		},
		{
			name:            "transformMap empty list",
			expr:            `[].transformMap(k, v, v + 1) == {}`,
			estimatedCost:   cost.FixedCostEstimate(72),
			estimatedCostV0: costV0(71, 71),
			actualCost:      72,
			actualCostV0:    actualV0(71),
		},
		{
			name:            "transformMap empty map",
			expr:            `{}.transformMap(k, v, v + 1) == {}`,
			estimatedCost:   cost.FixedCostEstimate(92),
			estimatedCostV0: costV0(91, 91),
			actualCost:      92,
			actualCostV0:    actualV0(91),
		},
		{
			name:          "transformMap literal scalar map",
			expr:          `{1: 2}.transformMap(k, v, v + 1) == {1: 3}`,
			estimatedCost: cost.FixedCostEstimate(97),
			actualCost:    97,
		},
		{
			name: "transformMap local bind",
			expr: `cel.bind(m, {"hello": "hello"},
			                m.transformMap(k, v, v + "world")) == {"hello": "helloworld"}`,
			estimatedCost:   cost.FixedCostEstimate(98),
			estimatedCostV0: costV0(108, 108),
			actualCost:      98,
			actualCostV0:    actualV0(108),
		},
		{
			name:            "transformMap filter map",
			expr:            `{1: 2, 3: 4, 5: 6}.transformMap(k, v, k % 3 == 0, v + 1) == {3: 5}`,
			estimatedCost:   cost.RangedCostEstimate(101, 116),
			estimatedCostV0: costV0(104, 116),
			actualCost:      106,
		},
		{
			name: "transformMap variable input",
			expr: `m.transformMap(k, v, k.startsWith('legacy') && v.size() == 1, v + [2]) == {'legacy-solo': [1, 2]}`,
			vars: []cel.EnvOption{
				cel.Variable("m", cel.MapType(cel.StringType, cel.ListType(cel.IntType))),
			},
			in: map[string]any{
				"m": map[string][]int{
					"legacy-solo": {1},
					"legacy-pair": {3, 2},
				},
			},
			hints: map[string]uint64{
				"m":                5,
				"m.@keys":          16,
				"m.@values":        10,
				"m.@values.@items": 2,
			},
			estimatedCost: cost.RangedCostEstimate(73, 173),
			actualCost:    98,
		},
		{
			name:          "transformMapEntry literal input",
			expr:          `{1: 2}.transformMapEntry(k, v, {v: k}) == {2: 1}`,
			estimatedCost: cost.FixedCostEstimate(126),
			actualCost:    126,
		},
		{
			name: "transformMapEntry variable input",
			expr: `m.transformMapEntry(k, v, {v: k}) == m.transformMapEntry(k, v, {v: k})`,
			vars: []cel.EnvOption{
				cel.Variable("m", cel.MapType(cel.StringType, cel.IntType)),
			},
			in: map[string]any{
				"m": map[string]int{
					"legacy-solo": 1,
					"legacy-pair": 2,
				},
			},
			hints: map[string]uint64{
				"m":         5,
				"m.@keys":   16,
				"m.@values": 10,
			},
			estimatedCost: cost.RangedCostEstimate(65, 405),
			actualCost:    201,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env := testCompreEnv(t, tc.vars...)
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
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost, ledgerV0(tc.actualCostV0)...)
			}
		})
	}
}

func TestTwoVarComprehensionsStaticErrors(t *testing.T) {
	tests := []struct {
		expr string
		err  string
	}{
		{
			expr: "[].all(i, i, i < i)",
			err:  "duplicate variable name: i",
		},
		{
			expr: "[].all(__result__, i, __result__ < i)",
			err:  "iteration variable overwrites accumulator variable",
		},
		{
			expr: "[].all(j, __result__, __result__ < j)",
			err:  "iteration variable overwrites accumulator variable",
		},
		{
			expr: "[].all(i.j, k, i.j < k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "[].all(j, i.k, j < i.k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "1.all(j, k, j < k)",
			err:  "cannot be range",
		},
		{
			expr: "[].exists(i.j, k, i.j < k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "[].exists(j, i.k, j < i.k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "''.exists(j, k, j < k)",
			err:  "cannot be range",
		},
		{
			expr: "[].exists_one(i.j, k, i.j < k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "[].existsOne(j, i.k, j < i.k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "[].exists_one(i.j, k, i.j < k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "''.existsOne(j, k, j < k)",
			err:  "cannot be range",
		},
		{
			expr: "[].transformList(i.j, k, i.j + k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "[].transformList(j, i.k, j + i.k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "{}.transformMap(i.j, k, i.j + k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "{}.transformMap(j, i.k, j + i.k)",
			err:  "argument must be a simple name",
		},
		{
			expr: "{}.transformMapEntry(j, i.k, {j: i.k})",
			err:  "argument must be a simple name",
		},
		{
			expr: "{}.transformMapEntry(i.j, k, {k: i.j})",
			err:  "argument must be a simple name",
		},
		{
			expr: "{}.transformMapEntry(j, k, 'bad filter', {k: j})",
			err:  "no matching overload",
		},
		{
			expr: "[1, 2].transformList(i, v, v % 2 == 0 ? [v] : v)",
			err:  "no matching overload",
		},
		{
			expr: `{'hello': 'world', 'greetings': 'tacocat'}.transformMapEntry(k, v, []) == {}`,
			err:  "no matching overload"},
	}
	env := testCompreEnv(t)
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if iss.Err() == nil || !strings.Contains(iss.Err().Error(), tc.err) {
				t.Errorf("env.Compile(%q) got %v, wanted error %v", tc.expr, iss.Err(), tc.err)
			}
		})
	}
}

func TestTwoVarComprehensionsRuntimeErrors(t *testing.T) {
	tests := []struct {
		expr string
		err  string
	}{
		{
			expr: "[1, 1].transformMapEntry(i, v, {v: i})",
			err:  "insert failed: key 1 already exists",
		},
		{
			expr: `[0, 0u].transformMapEntry(i, v, {v: i})`,
			err:  "insert failed: key 0 already exists",
		},
	}
	env := testCompreEnv(t)
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed with error %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program(ast) failed: %v", err)
			}
			in := cel.NoVars()
			_, _, err = prg.Eval(in)
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.err)
			}
		})
	}
}

func TestTwoVarComprehensionsVersion(t *testing.T) {
	_, err := cel.NewEnv(TwoVarComprehensions(TwoVarComprehensionsVersion(0)))
	if err != nil {
		t.Fatalf("TwoVarComprehensionVersion(0) failed: %v", err)
	}
}

func TestTwoVarComprehensionsUnparse(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		unparsed string
	}{
		{
			name:     "transform map entry",
			expr:     `[0, 0u].transformMapEntry(i, v, {v: i})`,
			unparsed: `[0, 0u].transformMapEntry(i, v, {v: i})`,
		},
		{
			name:     "transform map",
			expr:     `{'a': 'world', 'b': 'hello'}.transformMap(i, v, i == 'a' ? v.upperAscii() : v)`,
			unparsed: `{"a": "world", "b": "hello"}.transformMap(i, v, (i == "a") ? v.upperAscii() : v)`,
		},
		{
			name:     "transform list",
			expr:     `[1.0, 2.0, 2.0].transformList(i, v, i / 2.0 == 1.0)`,
			unparsed: `[1.0, 2.0, 2.0].transformList(i, v, i / 2.0 == 1.0)`,
		},
		{
			name:     "existsOne",
			expr:     `{'a': 'b', 'c': 'd'}.existsOne(k, v, k == 'b' || v == 'b')`,
			unparsed: `{"a": "b", "c": "d"}.existsOne(k, v, k == "b" || v == "b")`,
		},
		{
			name:     "exists",
			expr:     `{'a': 'b', 'c': 'd'}.exists(k, v, k == 'b' || v == 'b')`,
			unparsed: `{"a": "b", "c": "d"}.exists(k, v, k == "b" || v == "b")`,
		},
		{
			name:     "all",
			expr:     `[null, null, 'hello', string].all(i, v, i == 0 || type(v) != int)`,
			unparsed: `[null, null, "hello", string].all(i, v, i == 0 || type(v) != int)`,
		},
	}
	env := testCompreEnv(t)
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%q) failed: %v", tc.expr, iss.Err())
			}
			unparsed, err := cel.AstToString(ast)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if unparsed != tc.unparsed {
				t.Errorf("cel.AstToString() got %q, wanted %q", unparsed, tc.unparsed)
			}
		})
	}
}

func TestTwoVarComprehensionsResidualAST(t *testing.T) {
	tests := []struct {
		name     string
		in       map[string]any
		varOpts  []cel.EnvOption
		unks     []*cel.AttributePatternType
		expr     string
		residual string
	}{
		{
			name: "transform map entry residual compare",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.DynType)),
				cel.Variable("y", cel.IntType),
			},
			in: map[string]any{
				"x": []any{0, uint(1)},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("y")},
			expr:     `x.transformMapEntry(i, v, {v: i}).size() < y`,
			residual: `2 < y`,
		},
		{
			name: "transform map entry residual transform",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.DynType)),
				cel.Variable("y", cel.IntType),
			},
			in: map[string]any{
				"x": []any{0, uint(1)},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("y")},
			expr:     `x.transformMapEntry(i, v, i < y, {v: i})`,
			residual: `[0, 1u].transformMapEntry(i, v, i < y, {v: i})`,
		},
		{
			name: "nested exists unknown inner range",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.DynType)),
			},
			in: map[string]any{
				"x": []any{1, 2, 3},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("y")},
			expr:     `x.exists(val, y.exists(key, _, key == val))`,
			residual: `[1, 2, 3].exists(val, y.exists(key, _, key == val))`,
		},
		{
			name: "nested exists unknown inner range",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.DynType)),
			},
			in: map[string]any{
				"y": map[int]string{1: "hi", 2: "hello", 3: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("x")},
			expr:     `x.exists(val, y.exists(key, _, key == val))`,
			residual: `x.exists(val, y.exists(key, _, key == val))`,
		},
		{
			name: "nested exists unknown outer range with extra predicate",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.DynType)),
			},
			in: map[string]any{
				"y": map[int]string{1: "hi", 2: "hello", 3: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("x")},
			expr:     `x.exists(val, y.exists(key, _, key == val)) && y.all(key, val, val.startsWith('h'))`,
			residual: `x.exists(val, y.exists(key, _, key == val))`,
		},
		{
			name: "nested exists partial unknown outer range",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.DynType)),
			},
			in: map[string]any{
				"x": []int{42, 0, 43},
				"y": map[int]string{1: "hi", 2: "hello", 3: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("x").QualInt(1)},
			expr:     `x.exists(val, y.exists(key, _, key == val)) || x[0] == 0 || x[1] == 1 || x[2] == 2`,
			residual: `x.exists(val, y.exists(key, _, key == val)) || x[1] == 1`,
		},
		{
			name: "nested exists partial unknown outer range with optionals",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.IntType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.DynType)),
			},
			in: map[string]any{
				"x": []int{42, 0, 43},
				"y": map[int]string{1: "hi", 2: "hello", 3: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("x").QualInt(1)},
			expr:     `x.exists(val, y.exists(key, _, key == val)) || (x[?0].hasValue() && x[?1].hasValue())`,
			residual: `x.exists(val, y.exists(key, _, key == val)) || x[?1].hasValue()`,
		},
		{
			name: "inner value partial unknown two-var",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"howdy", "hello", "hi"},
				"y": map[int]string{0: "hi", 1: "hello", 2: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("y").QualInt(1)},
			expr:     `x.exists(key, val, y[?key] == optional.of(val))`,
			residual: `["howdy", "hello", "hi"].exists(key, val, y[?key] == optional.of(val))`,
		},
		{
			name: "inner value partial unknown one-var",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"howdy"},
				"y": map[int]string{0: "hello"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("x").QualInt(0)},
			expr:     `y.exists(key, y[?key] == x[?key])`,
			residual: `{0: "hello"}.exists(key, y[?key] == x[?key])`,
		},
		{
			name: "simple bind",
			varOpts: []cel.EnvOption{
				cel.Variable("y", cel.MapType(cel.IntType, cel.StringType)),
			},
			in: map[string]any{
				"y": map[int]string{0: "hi", 1: "hello", 2: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("y").QualInt(1)},
			expr:     `cel.bind(z, y[0], z + y[1])`,
			residual: `cel.bind(z, "hi", "hi" + y[1])`,
		},
		{
			name: "bind with comprehension",
			varOpts: []cel.EnvOption{
				cel.Variable("x", cel.ListType(cel.StringType)),
				cel.Variable("y", cel.MapType(cel.IntType, cel.StringType)),
			},
			in: map[string]any{
				"x": []string{"hi", "hello", "howdy"},
				"y": map[int]string{0: "hi", 1: "hello", 2: "howdy"},
			},
			unks:     []*cel.AttributePatternType{cel.AttributePattern("y").QualInt(1)},
			expr:     `cel.bind(z, y[0], x.all(i, val, val == z || optional.of(val) == y[?i]))`,
			residual: `cel.bind(z, "hi", ["hi", "hello", "howdy"].all(i, val, val == z || optional.of(val) == y[?i]))`,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env := testCompreEnv(t, tc.varOpts...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast,
				cel.EvalOptions(cel.OptTrackState, cel.OptPartialEval))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			unkVars, err := cel.PartialVars(tc.in, tc.unks...)
			if err != nil {
				t.Fatalf("PartialVars() failed: %v", err)
			}
			out, det, err := prg.Eval(unkVars)
			if !types.IsUnknown(out) {
				t.Fatalf("got %v, expected unknown", out)
			}
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			residual, err := env.ResidualAst(ast, det)
			if err != nil {
				t.Fatalf("env.ResidualAst() failed: %v", err)
			}
			expr, err := cel.AstToString(residual)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if expr != tc.residual {
				t.Errorf("got expr: %s, wanted %s", expr, tc.residual)
			}
		})
	}
}

func TestOptionalSlotsAndInputAttributes(t *testing.T) {
	env := testCompreEnv(t,
		cel.Container("google.expr.proto3.test"),
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.Variable("opt_map", cel.OptionalType(cel.MapType(cel.StringType, cel.MapType(cel.StringType, cel.StringType)))),
		cel.Variable("opt_list", cel.OptionalType(cel.ListType(cel.MapType(cel.StringType, cel.IntType)))),
		cel.Variable("opt_msg", cel.OptionalType(cel.ObjectType("google.expr.proto3.test.TestAllTypes"))),
		cel.Variable("list_of_opt_maps", cel.ListType(cel.OptionalType(cel.MapType(cel.StringType, cel.StringType)))),
	)

	presentInputs := map[string]any{
		"opt_map": types.OptionalOf(env.CELTypeAdapter().NativeToValue(map[string]map[string]string{
			"k1": {"a": "b"},
			"k2": {"a": "c"},
		})),
		"opt_list": types.OptionalOf(env.CELTypeAdapter().NativeToValue([]map[string]int64{
			{"x": 10},
			{},
			{"x": 30},
		})),
		"opt_msg": types.OptionalOf(env.CELTypeAdapter().NativeToValue(&proto3pb.TestAllTypes{
			NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{
				SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{Bb: 99},
			},
			MapStringString: map[string]string{"k1": "v1"},
		})),
		"list_of_opt_maps": []any{
			types.OptionalOf(env.CELTypeAdapter().NativeToValue(map[string]string{"a": "b"})),
			types.OptionalNone,
			types.OptionalOf(env.CELTypeAdapter().NativeToValue(map[string]string{"a": "c"})),
		},
	}

	noneInputs := map[string]any{
		"opt_map":          types.OptionalNone,
		"opt_list":         types.OptionalNone,
		"opt_msg":          types.OptionalNone,
		"list_of_opt_maps": []any{types.OptionalNone},
	}

	tests := []struct {
		name string
		expr string
		in   map[string]any
	}{
		{
			name: "one_var_optional_elements_hasValue",
			expr: `[optional.of('hello'), optional.none()].all(i, i.hasValue()) == false`,
		},
		{
			name: "one_var_optional_map_qualified_hasValue_arg",
			expr: `[optional.of({'a': 'b'}), optional.of({'a': 'c'})].exists(v, v.a.hasValue('c'))`,
		},
		{
			name: "two_var_list_optional_map_qualified_hasValue_arg",
			expr: `[optional.of({'a': 'b'}), optional.none(), optional.of({'a': 'c'})].exists(i, v, i == 2 && v.a.hasValue('c'))`,
		},
		{
			name: "optional_slot_select_missing_key_and_none",
			expr: `[optional.of({'a': 'b'}), optional.none(), optional.of({'other': 'x'})].map(v, v.?a) == [optional.of('b'), optional.none(), optional.none()]`,
		},
		{
			name: "optional_slot_multi_step_select_and_index",
			expr: `[optional.of({'a': {'b': [10, 20]}}), optional.none(), optional.of({'a': {}})].map(v, v.a.?b[?1]) == [optional.of(20), optional.none(), optional.none()]`,
		},
		{
			name: "two_var_map_of_optionals_qualified",
			expr: `{'k1': optional.of({'a': 'b'}), 'k2': optional.none(), 'k3': optional.of({'a': 'c'})}.exists(k, v, k == 'k3' && v.a.hasValue('c'))`,
		},
		{
			name: "transformMap_over_optional_values_with_optional_entry_construction",
			expr: `{'k1': optional.of({'a': 10}), 'k2': optional.none(), 'k3': optional.of({})}.transformMap(k, v, {?'val': v.?a}) == {'k1': {'val': 10}, 'k2': {}, 'k3': {}}`,
		},
		{
			name: "map_optional_list_and_proto_field_construction_from_slot",
			expr: `[optional.of({'a': 1}), optional.none(), optional.of({})].map(v, [?v.?a]) == [[1], [], []] && [optional.of({'a': 7}), optional.none()].map(v, TestAllTypes{?single_int64: v.?a}) == [TestAllTypes{single_int64: 7}, TestAllTypes{}]`,
		},
		{
			name: "optional_proto_slot_nested_field_qualification",
			expr: `[optional.of(TestAllTypes{single_nested_message: TestAllTypes.NestedMessage{bb: 42}}), optional.none(), optional.of(TestAllTypes{})].map(m, m.?single_nested_message.bb) == [optional.of(42), optional.none(), optional.none()]`,
		},
		{
			name: "slot_used_as_optional_map_and_list_index_qualifier",
			expr: `['a', 'missing'].map(k, {'a': 1}[?k]) == [optional.of(1), optional.none()] && [0, 1].map(i, [10, 20][?(i + 1)]) == [optional.of(20), optional.none()]`,
		},
		{
			name: "input_list_of_optional_maps_qualified_in_two_var_exists",
			expr: `list_of_opt_maps.exists(i, v, i == 2 && v.a.hasValue('c'))`,
			in:   presentInputs,
		},
		{
			name: "input_list_of_optional_maps_all_none",
			expr: `!list_of_opt_maps.exists(i, v, v.a.hasValue('c'))`,
			in:   noneInputs,
		},
		{
			name: "input_optional_map_indexed_by_comprehension_slot_present",
			expr: `['k1', 'k2', 'k3'].map(k, opt_map[?k].a) == [optional.of('b'), optional.of('c'), optional.none()]`,
			in:   presentInputs,
		},
		{
			name: "input_optional_map_indexed_by_comprehension_slot_none",
			expr: `['k1', 'k2'].all(k, !opt_map[?k].a.hasValue())`,
			in:   noneInputs,
		},
		{
			name: "input_optional_list_indexed_by_slot_and_optMap_comprehension_present",
			expr: `[0, 1, 2, 3].map(i, opt_list[?i].?x) == [optional.of(10), optional.none(), optional.of(30), optional.none()] && opt_list.optMap(l, l.exists(m, m.?x.hasValue(30))).hasValue(true)`,
			in:   presentInputs,
		},
		{
			name: "input_optional_list_indexed_by_slot_and_optMap_comprehension_none",
			expr: `[0, 1].all(i, !opt_list[?i].?x.hasValue()) && !opt_list.optMap(l, l.exists(m, m.?x.hasValue(30))).hasValue()`,
			in:   noneInputs,
		},
		{
			name: "input_optional_proto_qualified_by_slot_and_nested_select_present",
			expr: `['k1', 'missing'].map(k, opt_msg.map_string_string[?k]) == [optional.of('v1'), optional.none()] && cel.bind(n, opt_msg.single_nested_message, n.bb.hasValue(99))`,
			in:   presentInputs,
		},
		{
			name: "input_optional_proto_qualified_by_slot_and_nested_select_none",
			expr: `['k1'].all(k, !opt_msg.map_string_string[?k].hasValue()) && cel.bind(n, opt_msg.single_nested_message, !n.bb.hasValue())`,
			in:   noneInputs,
		},
	}

	progModes := []struct {
		name string
		opts []cel.ProgramOption
	}{
		{name: "default"},
		{name: "optimized", opts: []cel.ProgramOption{cel.EvalOptions(cel.OptOptimize)}},
		{name: "tracked_cost", opts: []cel.ProgramOption{cel.CostTracking(nil), cel.EvalOptions(cel.OptTrackState)}},
		{name: "exhaustive_partial", opts: []cel.ProgramOption{cel.EvalOptions(cel.OptExhaustiveEval, cel.OptPartialEval)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%q) failed: %v", tc.expr, iss.Err())
			}
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%q) failed: %v", tc.expr, iss.Err())
			}
			var in any = cel.NoVars()
			if tc.in != nil {
				in = tc.in
			}
			for _, astVariant := range []struct {
				label string
				ast   *cel.Ast
			}{
				{label: "checked", ast: cAst},
				{label: "unchecked", ast: pAst},
			} {
				for _, mode := range progModes {
					prg, err := env.Program(astVariant.ast, mode.opts...)
					if err != nil {
						t.Fatalf("%s/%s: env.Program() failed: %v", astVariant.label, mode.name, err)
					}
					out, _, err := prg.Eval(in)
					if err != nil {
						t.Fatalf("%s/%s: prg.Eval() failed: %v", astVariant.label, mode.name, err)
					}
					if out != types.True {
						t.Errorf("%s/%s: prg.Eval() = %v, wanted true", astVariant.label, mode.name, out)
					}
				}
			}
		})
	}

	// Verify partial evaluation when an optional input attribute is unknown while local slots are bound.
	t.Run("unknown_optional_input_with_local_slot_qualifier", func(t *testing.T) {
		cAst, iss := env.Compile(`['k1', 'k2'].map(k, opt_map[?k].?a)`)
		if iss.Err() != nil {
			t.Fatalf("env.Compile() failed: %v", iss.Err())
		}
		prg, err := env.Program(cAst, cel.EvalOptions(cel.OptPartialEval, cel.OptTrackState))
		if err != nil {
			t.Fatalf("env.Program() failed: %v", err)
		}
		pVars, err := cel.PartialVars(map[string]any{}, cel.AttributePattern("opt_map"))
		if err != nil {
			t.Fatalf("cel.PartialVars() failed: %v", err)
		}
		out, _, err := prg.Eval(pVars)
		if err != nil {
			t.Fatalf("prg.Eval() failed: %v", err)
		}
		if !types.IsUnknown(out) {
			t.Errorf("prg.Eval(unknown opt_map) = %v (%T), wanted *types.Unknown", out, out)
		}
	})
}

func testCompreEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{
		TwoVarComprehensions(),
		Bindings(),
		Lists(),
		Strings(),
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		cel.EnableHiddenAccumulatorName(true),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(TwoVarComprehensions()) failed: %v", err)
	}
	return env
}
