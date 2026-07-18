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

package cel

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"

	proto3pb "github.com/google/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestConstantFoldingOptimizer(t *testing.T) {
	tests := []struct {
		expr        string
		folded      string
		knownValues map[string]any
	}{
		{
			expr:   `[1, 1 + 2, 1 + (2 + 3)]`,
			folded: `[1, 3, 6]`,
		},
		{
			expr:   `6 in [1, 1 + 2, 1 + (2 + 3)]`,
			folded: `true`,
		},
		{
			expr:   `5 in [1, 1 + 2, 1 + (2 + 3)]`,
			folded: `false`,
		},
		{
			expr:   `x in [1, 1 + 2, 1 + (2 + 3)]`,
			folded: `x in [1, 3, 6]`,
		},
		{
			expr:   `1 in [1, x + 2, 1 + (2 + 3)]`,
			folded: `true`,
		},
		{
			expr:   `1 in [x, x + 2, 1 + (2 + 3)]`,
			folded: `1 in [x, x + 2, 6]`,
		},
		{
			expr:   `x in []`,
			folded: `false`,
		},
		{
			expr:   `{'hello': 'world'}.hello == x`,
			folded: `"world" == x`,
		},
		{
			expr:   `{'hello': 'world'}.?hello.orValue('default') == x`,
			folded: `"world" == x`,
		},
		{
			expr:   `{'hello': 'world'}['hello'] == x`,
			folded: `"world" == x`,
		},
		{
			expr:   `optional.of("hello")`,
			folded: `optional.of("hello")`,
		},
		{
			expr:   `optional.ofNonZeroValue("")`,
			folded: `optional.none()`,
		},
		{
			expr:   `{?'hello': optional.of('world')}['hello'] == x`,
			folded: `"world" == x`,
		},
		{
			expr:   `duration(string(7 * 24) + 'h')`,
			folded: `duration("604800s")`,
		},
		{
			expr:   `timestamp("1970-01-01T00:00:00Z")`,
			folded: `timestamp("1970-01-01T00:00:00Z")`,
		},
		{
			expr:   `[1, 1 + 1, 1 + 2, 2 + 3].exists(i, i < 10)`,
			folded: `true`,
		},
		{
			expr:   `[1, 1 + 1, 1 + 2, 2 + 3].exists(i, i < 1 % 2)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j))`,
			folded: `[[1, 2, 3], [2, 4, 6], [3, 6, 9]]`,
		},
		{
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j).filter(k, k % 2 == 0))`,
			folded: `[[2], [2, 4, 6], [6]]`,
		},
		{
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j).filter(k, k % 2 == x))`,
			folded: `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j).filter(k, k % 2 == x))`,
		},
		{
			expr:   `[(x - 1 > 3) ? 1 : 2].all(x, x < .x)`,
			folded: `[(x - 1 > 3) ? 1 : 2].all(x, x < .x)`,
		},
		{
			expr:   `[(x - 1 > 3) ? (x - 1) : 5].exists(x, x - 1 > 3)`,
			folded: `[(x - 1 > 3) ? (x - 1) : 5].exists(x, x - 1 > 3)`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].filter(m, has(m.a))`,
			folded: `[{"a": 1}]`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].filter(m, has({'a': true}.a))`,
			folded: `[{}, {"a": 1}, {"b": 2}]`,
		},
		{
			expr:   `type(1)`,
			folded: `int`,
		},
		{
			expr:   `[google.expr.proto3.test.TestAllTypes{single_int32: 2 + 3}].map(i, i)[0]`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: 5}`,
		},
		{
			expr:   `[?optional.ofNonZeroValue(0)]`,
			folded: `[]`,
		},
		{
			expr:   `[1, ?optional.ofNonZeroValue(0)]`,
			folded: `[1]`,
		},
		{
			expr:   `[optional.none(), ?x]`,
			folded: `[optional.none(), ?x]`,
		},
		{
			expr:   `[?optional.none(), ?x]`,
			folded: `[?x]`,
		},
		{
			expr:   `[?optional.of(1), ?x]`,
			folded: `[1, ?x]`,
		},
		{
			expr:   `[1, x, ?optional.ofNonZeroValue(0), ?x.?y]`,
			folded: `[1, x, ?x.?y]`,
		},
		{
			expr:   `[1, x, ?optional.ofNonZeroValue(3), ?x.?y]`,
			folded: `[1, x, 3, ?x.?y]`,
		},
		{
			expr:   `[1, x, ?optional.ofNonZeroValue(3), ?x.?y].size() > 3`,
			folded: `[1, x, 3, ?x.?y].size() > 3`,
		},
		{
			expr:   `{?'a': optional.of('hello'), ?x : optional.of(1), ?'b': optional.none()}`,
			folded: `{"a": "hello", ?x: optional.of(1)}`,
		},
		{
			expr:   `true ? x + 1 : x + 2`,
			folded: `x + 1`,
		},
		{
			expr:   `false ? x + 1 : x + 2`,
			folded: `x + 2`,
		},
		{
			expr:   `false ? x + 'world' : 'hello' + 'world'`,
			folded: `"helloworld"`,
		},
		{
			expr:   `x == true`,
			folded: `x == true`,
		},
		{
			expr:   `true == x`,
			folded: `true == x`,
		},
		{
			expr:   `x != false`,
			folded: `x != false`,
		},
		{
			expr:   `false != x`,
			folded: `false != x`,
		},
		{
			expr:   `x ? 1 + 2 : 3 + 4`,
			folded: `x ? 3 : 7`,
		},
		{
			expr:   `true && x`,
			folded: `x`,
		},
		{
			expr:   `x && true`,
			folded: `x`,
		},
		{
			expr:   `false && x`,
			folded: `false`,
		},
		{
			expr:   `x && false`,
			folded: `false`,
		},
		{
			expr:   `true || x`,
			folded: `true`,
		},
		{
			expr:   `x || true`,
			folded: `true`,
		},
		{
			expr:   `false || x`,
			folded: `x`,
		},
		{
			expr:   `x || false`,
			folded: `x`,
		},
		{
			expr:   `true && x && true && x`,
			folded: `x && x`,
		},
		{
			expr:   `false || x || false || x`,
			folded: `x || x`,
		},
		{
			expr:   `true && true`,
			folded: `true`,
		},
		{
			expr:   `true && false`,
			folded: `false`,
		},
		{
			expr:   `true || false`,
			folded: `true`,
		},
		{
			expr:   `false || false`,
			folded: `false`,
		},
		{
			expr:   `true && false || true`,
			folded: `true`,
		},
		{
			expr:   `false && true || false`,
			folded: `false`,
		},
		{
			expr:   `null`,
			folded: `null`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.ofNonZeroValue(1)}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.ofNonZeroValue(0)}`,
			folded: `google.expr.proto3.test.TestAllTypes{}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{single_int32: x, repeated_int32: [1, 2, 3]}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: x, repeated_int32: [1, 2, 3]}`,
		},
		{
			expr:   `x + dyn([1, 2] + [3, 4])`,
			folded: `x + [1, 2, 3, 4]`,
		},
		{
			expr:   `dyn([1, 2]) + [3.0, 4.0]`,
			folded: `[1, 2, 3.0, 4.0]`,
		},
		{
			expr:   `{'a': dyn([1, 2]), 'b': x}`,
			folded: `{"a": [1, 2], "b": x}`,
		},
		{
			expr:   `1 + x + 2 == 2 + x + 1`,
			folded: `1 + x + 2 == 2 + x + 1`,
		},
		{
			// The order of operations makes it such that the appearance of x in the first means that
			// none of the values provided into the addition call will be folded with the current
			// implementation. Ideally, the result would be 3 + x == x + 3 (which could be trivially true
			// and more easily observed as a result of common subexpression eliminiation)
			expr:   `1 + 2 + x ==  x + 2 + 1`,
			folded: `3 + x == x + 2 + 1`,
		},
		{
			expr:        `google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR`,
			folded:      `1`,
			knownValues: map[string]any{},
		},
		{
			expr:   `google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR`,
			folded: `google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR`,
		},
		{
			expr:        `c == google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAZ ? "BAZ" : "Unknown"`,
			folded:      `"BAZ"`,
			knownValues: map[string]any{},
		},
		{
			expr: `[
						google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR,
						c,
						google.expr.proto3.test.ImportedGlobalEnum.IMPORT_FOO
					].exists(e, e == google.expr.proto3.test.ImportedGlobalEnum.IMPORT_FOO)
						? "has Foo" : "no Foo"`,
			folded:      `"has Foo"`,
			knownValues: map[string]any{},
		},
		{
			expr:   `l.exists(e, e == "foo") ? "has Foo" : "no Foo"`,
			folded: `"has Foo"`,
			knownValues: map[string]any{
				"l": []string{"foo", "bar", "baz"},
			},
		},
		{
			expr:   `"foo" in l`,
			folded: `true`,
			knownValues: map[string]any{
				"l": []string{"foo", "bar", "baz"},
			},
		},
		{
			expr:   `o.repeated_int32`,
			folded: `[1, 2, 3]`,
			knownValues: map[string]any{
				"o": &proto3pb.TestAllTypes{RepeatedInt32: []int32{1, 2, 3}},
			},
		},
		{
			expr:   `false || x || false || y`,
			folded: `x || y`,
		},
		{
			expr:   `true ? (false ? x + 1 : x + 2) : x`,
			folded: `x + 2`,
		},
		{
			expr:   `false ? x : (true ? x + 1 : x + 2)`,
			folded: `x + 1`,
		},
		{
			expr:   `1 in []`,
			folded: `false`,
		},
		{
			expr:   `x in [1, 2, x]`,
			folded: `true`,
		},
		{
			expr:   `[1, 2].filter(x, x in [1, 2])`,
			folded: `[1, 2]`,
		},
		{
			expr:   `5 in [1, x, y, 5]`,
			folded: `true`,
		},
		{
			expr:   `!(5 in [1, x, y, 5])`,
			folded: `false`,
		},
		{
			expr:   `[1, ?optional.of(3)]`,
			folded: `[1, 3]`,
		},
		{
			expr:   `[1, optional.of(3)]`,
			folded: `[1, optional.of(3)]`,
		},
		{
			expr:   `[?optional.of(1 + 2 + 3)]`,
			folded: `[6]`,
		},
		{
			expr:   `[?optional.of(3)]`,
			folded: `[3]`,
		},
		{
			expr:   `[?optional.of(x)]`,
			folded: `[?optional.of(x)]`,
		},
		{
			expr:   `[?optional.ofNonZeroValue(3)]`,
			folded: `[3]`,
		},
		{
			expr:   `[optional.of(1 + 2 + 3)]`,
			folded: `[optional.of(6)]`,
		},
		{
			expr:   `[optional.of(x)]`,
			folded: `[optional.of(x)]`,
		},
		{
			expr:   `[optional.ofNonZeroValue(1 + 2 + 3)]`,
			folded: `[optional.of(6)]`,
		},
		{
			expr:   `[optional.ofNonZeroValue(3)]`,
			folded: `[optional.of(3)]`,
		},
		{
			expr:   `{?1: optional.none()}`,
			folded: `{}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{single_int64: 1 + 2 + 3 + x}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int64: 6 + x}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{single_nested_message: google.expr.proto3.test.TestAllTypes.NestedMessage{bb: 42}}.single_nested_message.bb`,
			folded: `42`,
		},
		{
			expr:   `{"a": 1}["a"]`,
			folded: `1`,
		},
		{
			expr:   `{"a": {"b": 2}}["a"]["b"]`,
			folded: `2`,
		},
		{
			expr:   `{"hello": "world"}.?hello`,
			folded: `optional.of("world")`,
		},
		{
			expr:   `[1] + [2] + [3]`,
			folded: `[1, 2, 3]`,
		},
		{
			expr:   `[?optional.none(), 2]`,
			folded: `[2]`,
		},
		{
			expr:   `[1] + [?optional.of(2)] + [3]`,
			folded: `[1, 2, 3]`,
		},
		{
			expr:   `[1] + [x]`,
			folded: `[1] + [x]`,
		},

		{
			expr:   `duration("1h") - duration("60m")`,
			folded: `duration("0s")`,
		},
		{
			expr:   `timestamp("1970-01-01T00:15:00Z") - timestamp("1970-01-01T00:00:10Z")`,
			folded: `duration("890s")`,
		},
		{
			expr:   `timestamp("2000-01-01T00:02:03.2123Z") + duration("25h2m32s42ms53us29ns")`,
			folded: `timestamp("2000-01-02T01:04:35.254353029Z")`,
		},
		{
			expr:   `[1 + 1, 1 + 2].exists(i, i < 10)`,
			folded: `true`,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType),
		Variable("y", DynType),
		// work around different package convention in piper vs github.
		// google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAZ
		Constant("c", IntType, types.Int(2)),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	e, err = e.Extend(Variable("l", ListType(StringType)))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	e, err = e.Extend(Variable("o", ObjectType("google.expr.proto3.test.TestAllTypes")))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			var foldingOpts []ConstantFoldingOption
			if tc.knownValues != nil {
				knownValues, err := NewActivation(tc.knownValues)
				if err != nil {
					t.Fatalf("NewActivation() failed: %v", err)
				}
				foldingOpts = append(foldingOpts, FoldKnownValues(knownValues))
			}
			folder, err := NewConstantFoldingOptimizer(foldingOpts...)
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}

func TestConstantFoldingCallsWithSideEffects(t *testing.T) {
	tests := []struct {
		expr   string
		folded string
	}{
		{
			expr:   `noSideEffect(3)`,
			folded: `3`,
		},
		{
			expr:   `withSideEffect(3)`,
			folded: `withSideEffect(3)`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b) && withSideEffect(i.b) == 1)`,
			folded: `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b) && withSideEffect(i.b) == 1)`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b) && noSideEffect(i.b) == 2)`,
			folded: `true`,
		},
		{
			expr:   `noImpl(3)`,
			folded: "noImpl(3)",
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Function("noSideEffect",
			Overload("noSideEffect_int_int",
				[]*Type{IntType},
				IntType, FunctionBinding(func(args ...ref.Val) ref.Val {
					return args[0]
				}))),
		Function("withSideEffect",
			Overload("withSideEffect_int_int",
				[]*Type{IntType},
				IntType, LateFunctionBinding())),
		Function("noImpl",
			Overload("noImpl_int_int",
				[]*Type{IntType},
				IntType)),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}

func TestConstantFoldingOptimizerMacroElimination(t *testing.T) {
	tests := []struct {
		expr       string
		folded     string
		macroCount int
	}{
		{
			expr:   `has({}.key)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].filter(i, i < 1)`,
			folded: `[]`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b))`,
			folded: `true`,
		},
		{
			expr:       `has(x.b) && [{}, {"a": 1}, {"b": 2}].exists(i, has(i.b))`,
			folded:     `has(x.b)`,
			macroCount: 1,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("folding got %q, wanted %q", folded, tc.folded)
			}
			if len(optimized.SourceInfo().GetMacroCalls()) != tc.macroCount {
				t.Errorf("folding got %d macros, wanted %d macros", len(optimized.SourceInfo().GetMacroCalls()), tc.macroCount)
			}
		})
	}
}

func TestConstantFoldingOptimizerWithLimit(t *testing.T) {
	tests := []struct {
		expr   string
		limit  int
		folded string
	}{
		{
			expr:   `[1, 1 + 2, 1 + (2 + 3)]`,
			limit:  1,
			folded: `[1, 3, 1 + 5]`,
		},
		{
			expr:   `5 in [1, 1 + 2, 1 + (2 + 3)]`,
			limit:  2,
			folded: `5 in [1, 3, 6]`,
		},
		{
			// though more complex, the final tryFold() at the end of the optimization pass
			// results in this computed output.
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j))`,
			limit:  1,
			folded: `[[1, 2, 3], [2, 4, 6], [3, 6, 9]]`,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer(MaxConstantFoldIterations(tc.limit))
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}

func TestConstantFoldingNormalizeIDs(t *testing.T) {
	tests := []struct {
		expr             string
		ids              []int64
		macros           map[int64]string
		normalizedIDs    []int64
		normalizedMacros map[int64]string
	}{
		{
			expr:          `[1, 2, 3]`,
			ids:           []int64{1, 2, 3, 4},
			normalizedIDs: []int64{1, 2, 3, 4},
		},
		{
			expr:          `google.expr.proto3.test.TestAllTypes{single_int32: 0}`,
			ids:           []int64{1, 2, 3},
			normalizedIDs: []int64{1, 2, 3},
		},
		{
			expr: `has({x: 'value'}.single_int32)`,
			ids:  []int64{2, 3, 4, 5, 7},
			macros: map[int64]string{7: `
			call_expr: {
				function: "has"
				args: {
				  id: 6
				  select_expr: {
					operand: {
					  id: 2
					  struct_expr: {
						entries: {
						  id: 3
						  map_key: {
							id: 4
							ident_expr: {
							  name: "x"
							}
						  }
						  value: {
							id: 5
							const_expr: {
							  string_value: "value"
							}
						  }
						}
					  }
					}
					field: "single_int32"
				  }
				}
			  }`},
			normalizedIDs: []int64{1, 2, 3, 4, 5},
			normalizedMacros: map[int64]string{1: `
			call_expr:  {
				function:  "has"
				args:  {
				  id:  6
				  select_expr:  {
					operand:  {
					  id:  2
					  struct_expr:  {
						entries:  {
						  id:  3
						  map_key:  {
							id:  4
							ident_expr:  {
							  name:  "x"
							}
						  }
						  value:  {
							id:  5
							const_expr:  {
							  string_value:  "value"
							}
						  }
						}
					  }
					}
					field:  "single_int32"
				  }
				}
			  }`,
			},
		},
		{
			expr: `has(google.expr.proto3.test.TestAllTypes{}.single_int32)`,
			ids:  []int64{2, 4},
			macros: map[int64]string{
				4: `call_expr:  {
					function:  "has"
					args:  {
					  id:  3
					  select_expr:  {
						operand:  {
						  id:  2
						  struct_expr:  {
							message_name:  "google.expr.proto3.test.TestAllTypes"
						  }
						}
						field:  "single_int32"
					  }
					}
				  }`,
			},
			normalizedIDs: []int64{1},
		},
		{
			expr: `[true].exists(i, i)`,
			ids:  []int64{1, 2, 5, 6, 7, 8, 9, 10, 11, 12, 13},
			macros: map[int64]string{
				13: `call_expr:  {
					target:  {
					  id:  1
					  list_expr:  {
						elements:  {
						  id:  2
						  const_expr:  {
							bool_value:  true
						  }
						}
					  }
					}
					function:  "exists"
					args:  {
					  id:  4
					  ident_expr:  {
						name:  "i"
					  }
					}
					args:  {
					  id:  5
					  ident_expr:  {
						name:  "i"
					  }
					}
				  }`,
			},
			normalizedIDs: []int64{1},
		},
		{
			expr: `[x].exists(i, i)`,
			ids:  []int64{1, 2, 5, 6, 7, 8, 9, 10, 11, 12, 13},
			macros: map[int64]string{
				13: `call_expr:  {
					target:  {
					  id:  1
					  list_expr:  {
						elements:  {
						  id:  2
						  ident_expr:  {
							name:  "x"
						  }
						}
					  }
					}
					function:  "exists"
					args:  {
					  id:  4
					  ident_expr:  {
						name:  "i"
					  }
					}
					args:  {
					  id:  5
					  ident_expr:  {
						name:  "i"
					  }
					}
				  }`,
			},
			normalizedIDs: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			normalizedMacros: map[int64]string{
				1: `call_expr: {
					target: {
					  id: 2
					  list_expr: {
						elements: {
						  id: 3
						  ident_expr: {
							name: "x"
						  }
						}
					  }
					}
					function: "exists"
					args: {
					  id: 12
					  ident_expr: {
						name: "i"
					  }
					}
					args: {
					  id: 10
					  ident_expr: {
						name: "i"
					  }
					}
				  }`,
			},
		},
	}
	e, err := NewEnv(
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			preOpt := newIDCollector()
			ast.PostOrderVisit(checked.NativeRep().Expr(), preOpt)
			if !reflect.DeepEqual(preOpt.IDs(), tc.ids) {
				t.Errorf("Compile() got ids %v, expected %v", preOpt.IDs(), tc.ids)
			}
			for id, call := range checked.NativeRep().SourceInfo().MacroCalls() {
				macroText, found := tc.macros[id]
				if !found {
					t.Fatalf("Compile() did not find macro %d", id)
				}
				pbCall, err := ast.ExprToProto(call)
				if err != nil {
					t.Fatalf("ast.ExprToProto() failed: %v", err)
				}
				pbMacro := &exprpb.Expr{}
				err = prototext.Unmarshal([]byte(macroText), pbMacro)
				if err != nil {
					t.Fatalf("prototext.Unmarshal() failed: %v", err)
				}
				if !proto.Equal(pbCall, pbMacro) {
					t.Errorf("Compile() for macro %d got %s, expected %s", id, prototext.Format(pbCall), macroText)
				}
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			postOpt := newIDCollector()
			ast.PostOrderVisit(optimized.NativeRep().Expr(), postOpt)
			if !reflect.DeepEqual(postOpt.IDs(), tc.normalizedIDs) {
				t.Errorf("Optimize() got ids %v, expected %v", postOpt.IDs(), tc.normalizedIDs)
			}
			for id, call := range optimized.NativeRep().SourceInfo().MacroCalls() {
				macroText, found := tc.normalizedMacros[id]
				if !found {
					t.Fatalf("Optimize() did not find macro %d", id)
				}
				pbCall, err := ast.ExprToProto(call)
				if err != nil {
					t.Fatalf("ast.ExprToProto() failed: %v", err)
				}
				pbMacro := &exprpb.Expr{}
				err = prototext.Unmarshal([]byte(macroText), pbMacro)
				if err != nil {
					t.Fatalf("prototext.Unmarshal() failed: %v", err)
				}
				if !proto.Equal(pbCall, pbMacro) {
					t.Errorf("Optimize() for macro %d got %s, expected %s", id, prototext.Format(pbCall), macroText)
				}
			}
		})
	}
}

func newIDCollector() *idCollector {
	return &idCollector{
		ids: int64Slice{},
	}
}

type idCollector struct {
	ids int64Slice
}

func (c *idCollector) VisitExpr(e ast.Expr) {
	if e.ID() == 0 {
		return
	}
	c.ids = append(c.ids, e.ID())
}

// VisitEntryExpr updates the max identifier if the incoming entry id is greater than previously observed.
func (c *idCollector) VisitEntryExpr(e ast.EntryExpr) {
	if e.ID() == 0 {
		return
	}
	c.ids = append(c.ids, e.ID())
}

func (c *idCollector) IDs() []int64 {
	sort.Sort(c.ids)
	return c.ids
}

// int64Slice is an implementation of the sort.Interface
type int64Slice []int64

// Len returns the number of elements in the slice.
func (x int64Slice) Len() int { return len(x) }

// Less indicates whether the value at index i is less than the value at index j.
func (x int64Slice) Less(i, j int) bool { return x[i] < x[j] }

// Swap swaps the values at indices i and j in place.
func (x int64Slice) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

// Sort is a convenience method: x.Sort() calls Sort(x).
func (x int64Slice) Sort() { sort.Sort(x) }

func TestConstantFoldingOption_FoldKnownValuesNilInput(t *testing.T) {
	opt, err := FoldKnownValues(nil)(&constantFoldingOptimizer{})
	if err != nil || opt.knownValues == nil {
		t.Errorf("FoldKnownValues(nil) failed: %v", err)
	}
}

func TestNewConstantFoldingOptimizer_OptionErrorPropagation(t *testing.T) {
	errOpt := func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error) {
		return nil, errors.New("option error")
	}
	if _, err := NewConstantFoldingOptimizer(errOpt); err == nil {
		t.Error("NewConstantFoldingOptimizer(errOpt) wanted error, got nil")
	}
}

func TestConstantFoldingOptimizer_EvaluateExpr(t *testing.T) {
	partAct, err := interpreter.NewPartialActivation(
		interpreter.EmptyActivation(),
		interpreter.NewAttributePattern("x"))
	if err != nil {
		t.Fatalf("NewPartialActivation() failed: %v", err)
	}
	unknownValAct, err := NewActivation(map[string]any{
		"x": types.NewUnknown(1, types.NewAttributeTrail("x")),
	})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}

	tests := []struct {
		name     string
		expr     string
		vars     []EnvOption
		act      Activation
		wantFold string
	}{
		{
			name:     "missing attribute",
			expr:     "x + 1",
			vars:     []EnvOption{Variable("x", IntType)},
			act:      partAct,
			wantFold: "x + 1",
		},
		{
			name:     "unknown value",
			expr:     "x + 1",
			vars:     []EnvOption{Variable("x", IntType)},
			act:      unknownValAct,
			wantFold: "x + 1",
		},
		{
			name:     "evaluation error division by zero",
			expr:     "1 / 0",
			wantFold: "1 / 0",
		},
		{
			name:     "evaluation error index out of bounds",
			expr:     "[1, 2][5]",
			wantFold: "[1, 2][5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnv(tt.vars...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			var foldOpts []ConstantFoldingOption
			if tt.act != nil {
				foldOpts = append(foldOpts, FoldKnownValues(tt.act))
			}
			folder, err := NewConstantFoldingOptimizer(foldOpts...)
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			ast, iss := env.Compile(tt.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			optimized, iss := opt.Optimize(env, ast)
			if iss.Err() != nil {
				t.Fatalf("Optimize() failed: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tt.wantFold {
				t.Errorf("got %q, wanted %q", folded, tt.wantFold)
			}
		})
	}
}

type testVariadicLogicOptimizer struct{}

func (t *testVariadicLogicOptimizer) Optimize(ctx *OptimizerContext, a *ast.AST) *ast.AST {
	call := ctx.NewCall(operators.LogicalAnd, ctx.NewIdent("x"), ctx.NewLiteral(types.True), ctx.NewIdent("y"))
	return ctx.NewAST(call)
}

func TestConstantFoldingOptimizer_VariadicShortcircuitLogic(t *testing.T) {
	env, err := NewEnv(Variable("x", BoolType), Variable("y", BoolType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	folder, err := NewConstantFoldingOptimizer()
	if err != nil {
		t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
	}
	opt, err := NewStaticOptimizer(&testVariadicLogicOptimizer{}, folder)
	if err != nil {
		t.Fatalf("NewStaticOptimizer() failed: %v", err)
	}
	parsed, iss := env.Parse("x && y")
	if iss.Err() != nil {
		t.Fatalf("Parse() failed: %v", iss.Err())
	}
	optimized, iss := opt.Optimize(env, parsed)
	if iss.Err() != nil {
		t.Fatalf("Optimize() failed: %v", iss.Err())
	}
	folded, err := AstToString(optimized)
	if err != nil {
		t.Fatalf("AstToString() failed: %v", err)
	}
	if folded != "x && y" {
		t.Errorf("got %q, wanted %q", folded, "x && y")
	}
}
