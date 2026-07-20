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
	"testing"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/parser"
	"github.com/google/cel-go/test"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

type testInfo struct {
	in        any
	expr      string
	out       string
	iterRange string
}

var testCases = []testInfo{
	{
		expr: `{{'nested_key': true}.nested_key: true}`,
		out:  `{true: true}`,
	},
	{
		in: map[string]any{
			"msg": map[string]string{"foo": "bar"},
		},
		expr: `msg`,
		out:  `{"foo": "bar"}`,
	},
	{
		expr: `true && false`,
		out:  `false`,
	},
	{
		in:   unknownActivation("x"),
		expr: `(true || false) && x`,
		out:  `x`,
	},
	{
		in:   unknownActivation("x"),
		expr: `(false || false) && x`,
		out:  `false`,
	},
	{
		in:   unknownActivation("a"),
		expr: `a && [1, 1u, 1.0].exists(x, type(x) == uint)`,
		out:  `a`,
	},
	{
		in:   unknownActivation("this"),
		expr: `this in []`,
		out:  `false`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{"b": "exists"},
		}, NewAttributePattern("this")),
		expr: `has(this.a) || !has(this.b)`,
		out:  `has(this.a) || !has(this.b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{"b": "exists"},
		}, NewAttributePattern("this").QualString("a")),
		expr: `has(this.a) || !has(this.b)`,
		out:  `has(this.a)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{"b": "exists"},
		}, NewAttributePattern("this").QualString("a")),
		expr: `!has(this.b) || has(this.a)`,
		out:  `has(this.a)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{},
		}, NewAttributePattern("this")),
		expr: `(!(this.a in []) || has(this.a)) || !has(this.b)`,
		out:  `true`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{},
		}, NewAttributePattern("this")),
		expr: `has(this.a) || !has(this.b)`,
		out:  `has(this.a) || !has(this.b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{},
		}, NewAttributePattern("this")),
		expr: `(has(this.a) || !(this.a in [])) || !has(this.b)`,
		out:  `true`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{"a": "exists"},
		}, NewAttributePattern("this").QualString("b")),
		expr: `has(this.a) && !has(this.b)`,
		out:  `!has(this.b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{},
		}, NewAttributePattern("this")),
		expr: `(has(this.a) && this.a in []) || !has(this.b)`,
		out:  `!has(this.b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]string{},
		}, NewAttributePattern("this")),
		expr: `(this.a in [] && has(this.a)) || !has(this.b)`,
		out:  `!has(this.b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]any{"a": map[string]string{}},
		}, NewAttributePattern("this").QualString("a")),
		expr: `has(this.a.b)`,
		out:  `has(this.a.b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": map[string]any{"a": map[string]string{}},
		}, NewAttributePattern("this").QualString("a")),
		expr: `has(this["a"].b)`,
		out:  `has(this["a"].b)`,
	},
	{
		in: partialActivation(map[string]any{
			"this": &proto3pb.TestAllTypes{SingleInt32: 0, SingleInt64: 1},
		}, NewAttributePattern("this").QualString("single_int64")),
		expr: `has(this.single_int32) && !has(this.single_int64)`,
		out:  `false`,
	},
	{
		in:   unknownActivation("this"),
		expr: `this in {}`,
		out:  `false`,
	},
	{
		in:   partialActivation(map[string]any{"rules": []string{}}, "this"),
		expr: `this in rules`,
		out:  `false`,
	},
	{
		in:   partialActivation(map[string]any{"rules": map[string]any{"not_in": []string{}}}, "this"),
		expr: `this.size() > 0 ? this in rules.not_in : !(this in rules.not_in)`,
		out:  `(this.size() > 0) ? false : true`,
	},
	{
		in: partialActivation(map[string]any{"rules": map[string]any{"not_in": []string{}}}, "this"),
		expr: `this.size() > 0 ? this in rules.not_in :
				!(this in rules.not_in) ? true : false`,
		out: `(this.size() > 0) ? false : true`,
	},
	{
		expr: `{'hello': 'world'.size()}`,
		out:  `{"hello": 5}`,
	},
	{
		expr: `[b'bytes-string']`,
		out:  `[b"\142\171\164\145\163\055\163\164\162\151\156\147"]`,
	},
	{
		expr: `[b'bytes'] + [b'-' + b'string']`,
		out:  `[b"\142\171\164\145\163", b"\055\163\164\162\151\156\147"]`,
	},
	{
		expr: `1u + 3u`,
		out:  `4u`,
	},
	{
		expr: `2 < 3`,
		out:  `true`,
	},
	{
		expr: `!false`,
		out:  `true`,
	},
	{
		in:   unknownActivation("y"),
		expr: `!y`,
		out:  `!y`,
	},
	{
		in:   partialActivation(map[string]any{"y": 10}),
		expr: `optional.of(y)`,
		out:  `optional.of(10)`,
	},
	{
		in:   unknownActivation("a"),
		expr: `a.?b`,
		out:  `a.?b`,
	},
	{
		in:   partialActivation(map[string]any{"a": map[string]any{"b": 10}}),
		expr: `a.?b`,
		out:  `optional.of(10)`,
	},
	{
		in:   partialActivation(map[string]any{"a": map[string]any{"b": 10}}),
		expr: `a[?"b"]`,
		out:  `optional.of(10)`,
	},
	{
		in:   unknownActivation(),
		expr: `{'b': optional.of(10)}.?b`,
		out:  `optional.of(optional.of(10))`,
	},
	{
		in:   partialActivation(map[string]any{"a": map[string]any{}}),
		expr: `a.?b`,
		out:  `optional.none()`,
	},
	{
		in:   unknownActivation(),
		expr: `[10].last()`,
		out:  "optional.of(10)",
	},
	{
		in:   unknownActivation(),
		expr: `[].last()`,
		out:  "optional.none()",
	},
	{
		in:   unknownActivation("a"),
		expr: `a[?"b"]`,
		out:  `a[?"b"]`,
	},
	{
		in:   unknownActivation(),
		expr: `[1, 2, 3, ?optional.none()]`,
		out:  `[1, 2, 3]`,
	},
	{
		in:   unknownActivation(),
		expr: `[1, 2, 3, ?optional.of(10)]`,
		out:  `[1, 2, 3, 10]`,
	},
	{
		in:   unknownActivation(),
		expr: `{1: 2, ?3: optional.none()}`,
		out:  `{1: 2}`,
	},
	{
		in:   unknownActivation("a"),
		expr: `[?optional.none(), a, 2, 3]`,
		out:  `[a, 2, 3]`,
	},
	{
		in:   unknownActivation("a"),
		expr: `[?optional.of(10), ?a, 2, 3]`,
		out:  `[10, ?a, 2, 3]`,
	},
	{
		in:   unknownActivation("a"),
		expr: `[?optional.of(10), a, 2, 3]`,
		out:  `[10, a, 2, 3]`,
	},
	{
		in:   partialActivation(map[string]any{"a": "hi"}, "b"),
		expr: `{?a: b.?c}`,
		out:  `{?"hi": b.?c}`,
	},
	{
		in:   partialActivation(map[string]any{"a": "hi"}, "b"),
		expr: `"hi" in {?a: b.?c}`,
		out:  `"hi" in {?"hi": b.?c}`,
	},
	{
		in:   partialActivation(map[string]any{"a": "hi"}, "b"),
		expr: `"hi" in {?a: optional.of("world")}`,
		out:  `true`,
	},
	{
		in:   partialActivation(map[string]any{"a": "hi"}, "b"),
		expr: `{?a: optional.of("world")}[b]`,
		out:  `{"hi": "world"}[b]`,
	},
	{
		in:   unknownActivation("y"),
		expr: `duration('1h') + duration('2h') > y`,
		out:  `duration("10800s") > y`,
	},
	{
		in:   unknownActivation("x"),
		expr: `[x, timestamp(0)]`,
		out:  `[x, timestamp("1970-01-01T00:00:00Z")]`,
	},
	{
		expr: `[timestamp(0), timestamp(1)]`,
		out:  `[timestamp("1970-01-01T00:00:00Z"), timestamp("1970-01-01T00:00:01Z")]`,
	},
	{
		expr: `{"epoch": timestamp(0)}`,
		out:  `{"epoch": timestamp("1970-01-01T00:00:00Z")}`,
	},
	{
		in:   partialActivation(map[string]any{"x": false}, "y"),
		expr: `!y && !x`,
		out:  `!y`,
	},
	{
		expr: `!y && !(1/0 < 0)`,
		out:  `!y && !(1/0 < 0)`,
	},
	{
		in:   partialActivation(map[string]any{"y": false}),
		expr: `!y && !(1/0 < 0)`,
		out:  `!(1/0 < 0)`,
	},
	{
		in:   unknownActivation(),
		expr: `test == null`,
		out:  `test == null`,
	},
	{
		in:   unknownActivation(),
		expr: `test == null || true`,
		out:  `true`,
	},
	{
		in:   unknownActivation(),
		expr: `test == null && false`,
		out:  `false`,
	},
	{
		in:   unknownActivation("b", "c"),
		expr: `true ? b < 1.2 : c == ['hello']`,
		out:  `b < 1.2`,
	},
	{
		in:   unknownActivation("b", "c"),
		expr: `false ? b < 1.2 : c == ['hello']`,
		out:  `c == ["hello"]`,
	},
	{
		in:   unknownActivation(),
		expr: `[1+3, 2+2, 3+1, four]`,
		out:  `[4, 4, 4, four]`,
	},
	// Since `undef` is not actually declared, these prune calls early exit
	// as there is no logical operator available to absorb the error.
	{
		in:   unknownActivation(),
		expr: `undef == {'a': 1, 'field': 2}.field`,
		out:  `undef == {"a": 1, "field": 2}.field`,
	},
	{
		in:   unknownActivation(),
		expr: `undef in {'a': 1, 'field': [2, 3]}.field`,
		out:  `undef in {"a": 1, "field": [2, 3]}.field`,
	},
	{
		in:   unknownActivation(),
		expr: `undef == {'field': [1 + 2, 2 + 3]}`,
		out:  `undef == {"field": [1 + 2, 2 + 3]}`,
	},
	{
		in:   unknownActivation(),
		expr: `undef in {'a': 1, 'field': [undef, 3]}.field`,
		out:  `undef in {"a": 1, "field": [undef, 3]}.field`,
	},
	// These expressions are effectively identical to the ones above,
	// but use a declared variable 'def'
	{
		in:   unknownActivation("def"),
		expr: `def == {'a': 1, 'field': 2}.field`,
		out:  `def == 2`,
	},
	{
		in:   unknownActivation("def"),
		expr: `def in {'a': 1, 'field': [2, 3]}.field`,
		out:  `def in [2, 3]`,
	},
	{
		in:   unknownActivation("def"),
		expr: `def == {'field': [1 + 2, 2 + 3]}`,
		out:  `def == {"field": [3, 5]}`,
	},
	{
		in:   unknownActivation("def"),
		expr: `def in {'a': 1, 'field': [def, 3]}.field`,
		out:  `def in {"a": 1, "field": [def, 3]}.field`,
	},
	{
		in:   partialActivation(map[string]any{"foo": "bar"}, "r.attr"),
		expr: `foo == "bar" && r.attr.loc in ["GB", "US"]`,
		out:  `r.attr.loc in ["GB", "US"]`,
	},
	{
		in: partialActivation(map[string]any{
			"users": []map[string]string{
				{"name": "alice", "role": "EMPLOYEE"},
				{"name": "bob", "role": "MANAGER"},
				{"name": "eve", "role": "CUSTOMER"},
			}}, "r.attr"),
		expr: `users.filter(u, u.role=="MANAGER").map(u, u.name) == r.attr.authorized["managers"]`,
		out:  `["bob"] == r.attr.authorized["managers"]`,
	},
	{
		in: partialActivation(map[string]any{
			"users": []string{"alice", "bob"},
		}, NewAttributePattern("r").QualString("attr").Wildcard()),
		expr:      `users.filter(u, u.startsWith(r.attr.prefix))`,
		out:       `["alice", "bob"].filter(u, u.startsWith(r.attr.prefix))`,
		iterRange: `["alice", "bob"]`,
	},
	{
		in: partialActivation(map[string]any{
			"users": []string{"alice", "bob"},
		}, NewAttributePattern("r").QualString("attr").Wildcard()),
		expr:      `users.filter(u, r.attr.prefix.endsWith(u))`,
		out:       `["alice", "bob"].filter(u, r.attr.prefix.endsWith(u))`,
		iterRange: `["alice", "bob"]`,
	},
	{
		in:   unknownActivation("four"),
		expr: `[1+3, 2+2, 3+1, four]`,
		out:  `[4, 4, 4, four]`,
	},
	{
		in:        unknownActivation("four"),
		expr:      `[1+3, 2+2, 3+1, four].exists(x, x == four)`,
		out:       `[4, 4, 4, four].exists(x, x == four)`,
		iterRange: `[4, 4, 4, four]`,
	},
	{
		in:   unknownActivation("a", "c"),
		expr: `[has(a.b), has(c.d)].exists(x, x == true)`,
		out:  `[has(a.b), has(c.d)].exists(x, x == true)`,
	},
	{
		in: partialActivation(map[string]any{
			"a": map[string]any{},
		}, "c"),
		expr: `[has(a.b), has(c.d)].exists(x, x == true)`,
		out:  `[false, has(c.d)].exists(x, x == true)`,
	},
	{
		in: partialActivation(map[string]any{
			"a": map[string]any{},
		}, "c"),
		expr:      `[has(a.b), has(c.d)].exists(x, x == true)`,
		out:       `[false, has(c.d)].exists(x, x == true)`,
		iterRange: `[false, has(c.d)]`,
	},
	{
		in: partialActivation(map[string]any{
			"a": map[string]string{},
		}),
		expr: `[?a[?0], a.b]`,
		out:  `[a.b]`,
	},
	{
		in: partialActivation(map[string]any{
			"a": map[string]string{},
		}, "a"),
		expr: `[?a[?0], a.b].exists(x, x == true)`,
		out:  `[?a[?0], a.b].exists(x, x == true)`,
	},
	{
		in: partialActivation(map[string]any{
			"a": map[string]string{},
		}),
		expr: `[?a[?0], a.b].exists(x, x == true)`,
		out:  `[a.b].exists(x, x == true)`,
	},
	{
		in: partialActivation(map[string]any{
			"a": map[string]string{},
		}),
		expr: `[a[0], a.b].exists(x, x == true)`,
		out:  `[a[0], a.b].exists(x, x == true)`,
	},
}

func TestPrune(t *testing.T) {
	p, err := parser.NewParser(
		parser.EnableOptionalSyntax(true),
		parser.PopulateMacroCalls(true),
		parser.Macros(parser.AllMacros...),
	)
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	for i, tst := range testCases {
		parsed, iss := p.Parse(common.NewStringSource(tst.expr, "<input>"))
		if len(iss.GetErrors()) > 0 {
			t.Fatalf("Parse(%q) failed: %v", tst.expr, iss.ToDisplayString())
		}
		state := NewEvalState()
		reg := newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
		attrs := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
		dispatcher := NewDispatcher()
		addFunctionBindings(t, dispatcher)
		dispatcher.Add(funcBindings(t, optionalDecls(t)...)...)
		interp := NewInterpreter(dispatcher, containers.DefaultContainer, reg, reg, attrs)
		interpretable, err := interp.NewInterpretable(parsed,
			ExhaustiveEval(), EvalStateObserver(EvalStateFactory(func() EvalState { return state })))
		if err != nil {
			t.Fatalf("NewUncheckedInterpretable() failed: %v", err)
		}
		interpretable.Eval(testActivation(t, tst.in))
		newExpr := PruneAst(parsed.Expr(), parsed.SourceInfo().MacroCalls(), state)
		if tst.iterRange != "" {
			if newExpr.Expr().Kind() != ast.ComprehensionKind {
				t.Fatalf("iter range check cannot operate on non comprehension output: %v", newExpr.Expr())
			}
			compre := newExpr.Expr().AsComprehension()
			gotIterRange, err := parser.Unparse(compre.IterRange(), newExpr.SourceInfo())
			if err != nil {
				t.Fatalf("parser.Unparse() failed: %v", err)
			}
			if gotIterRange != tst.iterRange {
				t.Errorf("iter range unparse got: %v, wanted %v", gotIterRange, tst.iterRange)
			}
		}
		actual, err := parser.Unparse(newExpr.Expr(), newExpr.SourceInfo())
		if err != nil {
			t.Fatalf("parser.Unparse() failed: %v", err)
		}
		if !test.Compare(actual, tst.out) {
			for _, id := range state.IDs() {
				v, _ := state.Value(id)
				t.Logf("state[%d] %v\n", id, v)
			}
			t.Errorf("prune[%d], diff: %s", i, test.DiffMessage("structure", actual, tst.out))
		}
	}
}

func unknownActivation(vars ...string) PartialActivation {
	pats := make([]*AttributePattern, len(vars))
	for i, v := range vars {
		pats[i] = NewAttributePattern(v)
	}
	a, _ := NewPartialActivation(map[string]any{}, pats...)
	return a
}

func partialActivation(in map[string]any, vars ...any) PartialActivation {
	pats := make([]*AttributePattern, len(vars))
	for i, v := range vars {
		if pat, ok := v.(*AttributePattern); ok {
			pats[i] = pat
			continue
		}
		if str, ok := v.(string); ok {
			pats[i] = NewAttributePattern(str)
			continue
		}
	}
	a, _ := NewPartialActivation(in, pats...)
	return a
}

func testActivation(t *testing.T, in any) Activation {
	t.Helper()
	if in == nil {
		return EmptyActivation()
	}
	a, err := NewActivation(in)
	if err != nil {
		t.Fatalf("NewActivation(%v) failed: %v", in, err)
	}
	return a
}

func optionalDecls(t *testing.T) []*decls.FunctionDecl {
	paramType := types.NewTypeParamType("T")
	optionalType := types.NewOptionalType(paramType)
	return []*decls.FunctionDecl{
		funcDecl(t, "optional.none",
			decls.Overload("optional_none", []*types.Type{}, optionalType,
				decls.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.OptionalNone
				}),
			),
		),
		funcDecl(t, "optional.of",
			decls.Overload("optional_of_value", []*types.Type{paramType}, optionalType,
				decls.UnaryBinding(func(val ref.Val) ref.Val {
					return types.OptionalOf(val)
				}),
			),
		),
		funcDecl(t, "_[?_]",
			decls.Overload("map_optindex_optional_value", []*types.Type{
				types.NewMapType(types.NewTypeParamType("K"), paramType),
				types.NewTypeParamType("K"),
			}, optionalType),
		),
		funcDecl(t, "last", decls.Overload("list_last", []*types.Type{paramType}, optionalType,
			decls.UnaryBinding(func(v ref.Val) ref.Val {
				list := v.(traits.Lister)
				sz := list.Size().Value().(int64)

				if sz == 0 {
					return types.OptionalNone
				}

				return types.OptionalOf(list.Get(types.Int(sz - 1)))
			}),
		),
		),
	}
}
