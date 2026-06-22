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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/interpreter"
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
		estimatedCost checker.CostEstimate
		actualCost    uint64
	}{
		{
			name:          "all list literal",
			expr:          `[1, 2, 3, 4].all(i, v, i < 5 && v > 0)`,
			estimatedCost: checker.CostEstimate{Min: 23, Max: 39},
			actualCost:    39,
		},
		{
			name:          "all map literal - true",
			expr:          `{1: 1, 2: 2, 3: 3}.all(i, v, i < 5 && v > 0)`,
			estimatedCost: checker.CostEstimate{Min: 40, Max: 52},
			actualCost:    52,
		},
		{
			name:          "all map literal - false",
			expr:          `!{0: 0}.all(i, v, i < 5 && v > 0)`,
			estimatedCost: checker.CostEstimate{Min: 35, Max: 39},
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
			estimatedCost: checker.CostEstimate{Min: 2, Max: 23},
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
			estimatedCost: checker.CostEstimate{Min: 2, Max: 23},
			actualCost:    14,
		},
		{
			name:          "transformList empty",
			expr:          `[].transformList(i, v, v) == []`,
			estimatedCost: checker.FixedCostEstimate(31),
			actualCost:    31,
		},
		{
			name:          "transformList single element",
			expr:          `[1].transformList(i, v, i) == [0]`,
			estimatedCost: checker.FixedCostEstimate(45),
			actualCost:    45,
		},
		{
			name:          "transformList with filter",
			expr:          `[3, 2, 1].transformList(i, v, v > i, v) == [3, 2]`,
			estimatedCost: checker.CostEstimate{Min: 44, Max: 80},
			actualCost:    67,
		},
		{
			name:          "transformMap empty list",
			expr:          `[].transformMap(k, v, v + 1) == {}`,
			estimatedCost: checker.FixedCostEstimate(71),
			actualCost:    71,
		},
		{
			name:          "transformMap empty map",
			expr:          `{}.transformMap(k, v, v + 1) == {}`,
			estimatedCost: checker.FixedCostEstimate(91),
			actualCost:    91,
		},
		{
			name:          "transformMap literal scalar map",
			expr:          `{1: 2}.transformMap(k, v, v + 1) == {1: 3}`,
			estimatedCost: checker.FixedCostEstimate(97),
			actualCost:    97,
		},
		{
			name: "transformMap local bind",
			expr: `cel.bind(m, {"hello": "hello"},
			                m.transformMap(k, v, v + "world")) == {"hello": "helloworld"}`,
			estimatedCost: checker.FixedCostEstimate(108),
			actualCost:    108,
		},
		{
			name:          "transformMap filter map",
			expr:          `{1: 2, 3: 4, 5: 6}.transformMap(k, v, k % 3 == 0, v + 1) == {3: 5}`,
			estimatedCost: checker.CostEstimate{Min: 104, Max: 116},
			actualCost:    106,
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
			estimatedCost: checker.CostEstimate{Min: 73, Max: 173},
			actualCost:    98,
		},
		{
			name:          "transformMapEntry literal input",
			expr:          `{1: 2}.transformMapEntry(k, v, {v: k}) == {2: 1}`,
			estimatedCost: checker.FixedCostEstimate(126),
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
			estimatedCost: checker.CostEstimate{Min: 65, Max: 405},
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

			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost)
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
		unks     []*interpreter.AttributePattern
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("y")},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("y")},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("y")},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("x")},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("x")},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("x").QualInt(1)},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("x").QualInt(1)},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("y").QualInt(1)},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("x").QualInt(0)},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("y").QualInt(1)},
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
			unks:     []*interpreter.AttributePattern{cel.AttributePattern("y").QualInt(1)},
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
