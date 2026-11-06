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

package cel_test

import (
	"testing"

	"cel.dev/cel-go/cel"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestInliningOptimizerNoopShadow(t *testing.T) {
	type varExpr struct {
		name  string
		alias string
		t     *cel.Type
		expr  string
	}
	tests := []struct {
		name    string
		expr    string
		vars    []varExpr
		inlined string
	}{
		{
			name: "shadow at parent",
			expr: `[0].exists(shadowed_ident, shadowed_ident == 0)`,
			vars: []varExpr{
				{
					name: "shadowed_ident",
					t:    cel.IntType,
					expr: "1",
				},
			},
			inlined: `[0].exists(shadowed_ident, shadowed_ident == 0)`,
		},
		{
			name: "shadow in ancestor",
			expr: `[[1]].all(shadowed_ident, shadowed_ident.all(shadowed, shadowed + 1 == 2))`,
			vars: []varExpr{
				{
					name: "shadowed_ident",
					t:    cel.IntType,
					expr: "42",
				},
			},
			inlined: `[[1]].all(shadowed_ident, shadowed_ident.all(shadowed, shadowed + 1 == 2))`,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			opts := []cel.EnvOption{
				cel.OptionalTypes(),
				cel.EnableMacroCallTracking(),
			}
			varDecls := make([]cel.EnvOption, len(tc.vars))
			for i, v := range tc.vars {
				varDecls[i] = cel.Variable(v.name, v.t)
			}
			e, err := cel.NewEnv(append(varDecls, opts...)...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			inlinedVars := []*cel.InlineVariable{}
			for _, v := range tc.vars {
				if v.expr == "" {
					continue
				}
				checked, iss := e.Compile(v.expr)
				if iss.Err() != nil {
					t.Fatalf("Compile(%q) failed: %v", v.expr, iss.Err())
				}
				if v.alias == "" {
					inlinedVars = append(inlinedVars, cel.NewInlineVariable(v.name, checked))
				} else {
					inlinedVars = append(inlinedVars, cel.NewInlineVariableWithAlias(v.name, v.alias, checked))
				}
			}
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			opt, err := cel.NewStaticOptimizer(cel.NewInliningOptimizer(inlinedVars...))
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			cel.AssertNormalizedIDs(t, optimized)
			inlined, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if inlined != tc.inlined {
				t.Errorf("inlined got %q, wanted %q", inlined, tc.inlined)
			}
		})
	}
}

func TestInliningOptimizerPresenceTests(t *testing.T) {
	type varExpr struct {
		name  string
		alias string
		t     *cel.Type
		expr  string
	}
	tests := []struct {
		name    string
		expr    string
		vars    []varExpr
		inlined string
	}{
		{
			name: "presence with bool literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.BoolType,
					expr: "true",
				},
			},
			inlined: `true != false`,
		},
		{
			name: "presence with bytes literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.BytesType,
					expr: "b'abc'",
				},
			},
			inlined: `b"\141\142\143".size() != 0`,
		},
		{
			name: "presence with double literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.DoubleType,
					expr: "42.0",
				},
			},
			inlined: `42.0 != 0.0`,
		},
		{
			name: "presence with duration literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.DurationType,
					expr: "duration('1s')",
				},
			},
			inlined: `duration("1s") != duration("0s")`,
		},
		{
			name: "presence with int literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.IntType,
					expr: "1",
				},
			},
			inlined: `1 != 0`,
		},
		{
			name: "presence with list literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.ListType(cel.StringType),
					expr: "['foo', 'bar']",
				},
			},
			inlined: `["foo", "bar"].size() != 0`,
		},
		{
			name: "presence with map literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.MapType(cel.StringType, cel.StringType),
					expr: "{'foo': 'bar'}",
				},
			},
			inlined: `{"foo": "bar"}.size() != 0`,
		},
		{
			name: "presence with string literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.StringType,
					expr: "'foo'",
				},
			},
			inlined: `"foo".size() != 0`,
		},
		{
			name: "presence with struct literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
					expr: "TestAllTypes{single_int64: 1}",
				},
			},
			inlined: `google.expr.proto3.test.TestAllTypes{single_int64: 1} != google.expr.proto3.test.TestAllTypes{}`,
		},
		{
			name: "presence with timestamp literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.TimestampType,
					expr: "timestamp(123)",
				},
			},
			inlined: `timestamp(123) != timestamp(0)`,
		},
		{
			name: "presence with uint literal",
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varExpr{
				{
					name: "msg.single_any.processing_purpose",
					t:    cel.UintType,
					expr: "1u",
				},
			},
			inlined: `1u != 0u`,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			opts := []cel.EnvOption{
				cel.Container("google.expr.proto3.test"),
				cel.Types(&proto3pb.TestAllTypes{}),
				cel.Variable("msg", cel.ObjectType("google.expr.proto3.test.TestAllTypes")),
				cel.OptionalTypes(),
				cel.EnableMacroCallTracking(),
			}
			varDecls := make([]cel.EnvOption, len(tc.vars))
			for i, v := range tc.vars {
				varDecls[i] = cel.Variable(v.name, v.t)
			}
			e, err := cel.NewEnv(append(varDecls, opts...)...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			inlinedVars := []*cel.InlineVariable{}
			for _, v := range tc.vars {
				if v.expr == "" {
					continue
				}
				checked, iss := e.Compile(v.expr)
				if iss.Err() != nil {
					t.Fatalf("Compile(%q) failed: %v", v.expr, iss.Err())
				}
				if v.alias == "" {
					inlinedVars = append(inlinedVars, cel.NewInlineVariable(v.name, checked))
				} else {
					inlinedVars = append(inlinedVars, cel.NewInlineVariableWithAlias(v.name, v.alias, checked))
				}
			}
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			opt, err := cel.NewStaticOptimizer(cel.NewInliningOptimizer(inlinedVars...))
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			cel.AssertNormalizedIDs(t, optimized)
			inlined, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if inlined != tc.inlined {
				t.Errorf("inlined got %q, wanted %q", inlined, tc.inlined)
			}
		})
	}
}

func TestInliningOptimizer(t *testing.T) {
	type varExpr struct {
		name  string
		alias string
		t     *cel.Type
		expr  string
	}
	tests := []struct {
		expr    string
		vars    []varExpr
		inlined string
		folded  string
	}{
		{
			expr: `a || b`,
			vars: []varExpr{
				{
					name: "a",
					t:    cel.BoolType,
				},
				{
					name:  "b",
					alias: "bravo",
					t:     cel.BoolType,
					expr:  `'hello'.contains('lo')`,
				},
			},
			inlined: `a || "hello".contains("lo")`,
			folded:  `true`,
		},
		{
			expr: `a + [a]`,
			vars: []varExpr{
				{
					name:  "a",
					alias: "alpha",
					t:     cel.DynType,
					expr:  `dyn([1, 2])`,
				},
			},
			inlined: `cel.bind(alpha, dyn([1, 2]), alpha + [alpha])`,
			folded:  `[1, 2, [1, 2]]`,
		},
		{
			expr: `a && (a || b)`,
			vars: []varExpr{
				{
					name:  "a",
					alias: "alpha",
					t:     cel.BoolType,
					expr:  `'hello'.contains('lo')`,
				},
				{
					name: "b",
					t:    cel.BoolType,
				},
			},
			inlined: `cel.bind(alpha, "hello".contains("lo"), alpha && (alpha || b))`,
			folded:  `true`,
		},
		{
			expr: `a && b && a`,
			vars: []varExpr{
				{
					name:  "a",
					alias: "alpha",
					t:     cel.BoolType,
					expr:  `'hello'.contains('lo')`,
				},
				{
					name: "b",
					t:    cel.BoolType,
				},
			},
			inlined: `cel.bind(alpha, "hello".contains("lo"), alpha && b && alpha)`,
			folded:  `cel.bind(alpha, true, alpha && b && alpha)`,
		},
		{
			expr: `(c || d) || (a && (a || b))`,
			vars: []varExpr{
				{
					name:  "a",
					alias: "alpha",
					t:     cel.BoolType,
					expr:  `'hello'.contains('lo')`,
				},
				{
					name: "b",
					t:    cel.BoolType,
				},
				{
					name: "c",
					t:    cel.BoolType,
				},
				{
					name: "d",
					t:    cel.BoolType,
					expr: "!false",
				},
			},
			inlined: `c || !false || cel.bind(alpha, "hello".contains("lo"), alpha && (alpha || b))`,
			folded:  `true`,
		},
		{
			expr: `a && (a || b)`,
			vars: []varExpr{
				{
					name: "a",
					t:    cel.BoolType,
				},
				{
					name:  "b",
					alias: "bravo",
					t:     cel.BoolType,
					expr:  `'hello'.contains('lo')`,
				},
			},
			inlined: `a && (a || "hello".contains("lo"))`,
			folded:  `a`,
		},
		{
			expr: `a && b`,
			vars: []varExpr{
				{
					name:  "a",
					alias: "alpha",
					t:     cel.BoolType,
					expr:  `!'hello'.contains('lo')`,
				},
				{
					name:  "b",
					alias: "bravo",
					t:     cel.BoolType,
				},
			},
			inlined: `!"hello".contains("lo") && b`,
			folded:  `false`,
		},
		{
			expr: `operation.system.consumers + operation.destination_consumers`,
			vars: []varExpr{
				{
					name: "operation.system",
					t:    cel.DynType,
				},
				{
					name: "operation.destination_consumers",
					t:    cel.ListType(cel.IntType),
					expr: `productsToConsumers(operation.destination_products)`,
				},
				{
					name: "operation.destination_products",
					t:    cel.ListType(cel.IntType),
					expr: `operation.system.products`,
				},
			},
			inlined: `operation.system.consumers + productsToConsumers(operation.system.products)`,
			folded:  `operation.system.consumers + productsToConsumers(operation.system.products)`,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			opts := []cel.EnvOption{cel.OptionalTypes(),
				cel.EnableMacroCallTracking(),
				cel.Function("productsToConsumers",
					cel.Overload("productsToConsumers_list",
						[]*cel.Type{cel.ListType(cel.IntType)},
						cel.ListType(cel.IntType)))}

			varDecls := make([]cel.EnvOption, len(tc.vars))
			for i, v := range tc.vars {
				varDecls[i] = cel.Variable(v.name, v.t)
			}
			e, err := cel.NewEnv(append(varDecls, opts...)...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			inlinedVars := []*cel.InlineVariable{}
			for _, v := range tc.vars {
				if v.expr == "" {
					continue
				}
				checked, iss := e.Compile(v.expr)
				if iss.Err() != nil {
					t.Fatalf("Compile(%q) failed: %v", v.expr, iss.Err())
				}
				if v.alias == "" {
					inlinedVars = append(inlinedVars, cel.NewInlineVariable(v.name, checked))
				} else {
					inlinedVars = append(inlinedVars, cel.NewInlineVariableWithAlias(v.name, v.alias, checked))
				}
			}
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}

			opt, err := cel.NewStaticOptimizer(cel.NewInliningOptimizer(inlinedVars...))
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			cel.AssertNormalizedIDs(t, optimized)
			inlined, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if inlined != tc.inlined {
				t.Errorf("inlined got %q, wanted %q", inlined, tc.inlined)
			}
			folder, err := cel.NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err = cel.NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss = opt.Optimize(e, optimized)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			cel.AssertNormalizedIDs(t, optimized)
			folded, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("folded got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}

func TestInliningOptimizerMultiStage(t *testing.T) {
	type varDecl struct {
		name string
		t    *cel.Type
	}
	type inlineVarExpr struct {
		name  string
		alias string
		t     *cel.Type
		expr  string
	}
	tests := []struct {
		expr       string
		vars       []varDecl
		inlineVars []inlineVarExpr
		inlined    string
		folded     string
	}{
		{
			expr: `has(a.b)`,
			vars: []varDecl{
				{
					name: "a",
					t:    cel.MapType(cel.StringType, cel.StringType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "a.b",
					alias: "alpha",
					t:     cel.StringType,
					expr:  `a.b_long`,
				},
			},
			inlined: `has(a.b_long)`,
			folded:  `has(a.b_long)`,
		},
		{
			expr: `has(a.b) ? a.b : 'default'`,
			vars: []varDecl{
				{
					name: "a",
					t:    cel.MapType(cel.StringType, cel.StringType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "a.b",
					alias: "alpha",
					t:     cel.StringType,
					expr:  `'hello'`,
				},
			},
			inlined: `cel.bind(alpha, "hello", (alpha.size() != 0) ? alpha : "default")`,
			folded:  `"hello"`,
		},
		{
			expr: `has(a.b) ? a.b : ['default']`,
			vars: []varDecl{
				{
					name: "a",
					t:    cel.MapType(cel.StringType, cel.ListType(cel.StringType)),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "a.b",
					alias: "alpha",
					t:     cel.StringType,
					expr:  `['hello']`,
				},
			},
			inlined: `cel.bind(alpha, ["hello"], (alpha.size() != 0) ? alpha : ["default"])`,
			folded:  `["hello"]`,
		},
		{
			expr: `0 in msg.map_int64_nested_type`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "nested_map",
					t:    cel.MapType(cel.IntType, cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes")),
				},
			},
			inlineVars: []inlineVarExpr{

				{
					name: "msg.map_int64_nested_type",
					t:    cel.MapType(cel.IntType, cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes")),
					expr: `nested_map`,
				},
			},
			inlined: `0 in nested_map`,
			folded:  `0 in nested_map`,
		},
		{
			expr: `has(msg.single_any)`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "unpacked_wrapper",
					t:    cel.NullableType(cel.StringType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name: "msg.single_any",
					t:    cel.NullableType(cel.StringType),
					expr: `unpacked_wrapper`,
				},
			},
			inlined: `unpacked_wrapper != null`,
			folded:  `unpacked_wrapper != null`,
		},
		{
			expr: `has(msg.single_any) ? msg.single_any : '10'`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "unpacked_wrapper",
					t:    cel.NullableType(cel.StringType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any",
					t:     cel.NullableType(cel.StringType),
					alias: "wrapped",
					expr:  `unpacked_wrapper`,
				},
			},
			inlined: `cel.bind(wrapped, unpacked_wrapper, (wrapped != null) ? wrapped : "10")`,
			folded:  `cel.bind(wrapped, unpacked_wrapper, (wrapped != null) ? wrapped : "10")`,
		},
		{
			expr: `has(msg.child.payload.single_int32_wrapper)`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
				},
				{
					name: "unpacked_child",
					t:    cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.child.payload",
					t:     cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
					alias: "payload",
					expr:  `unpacked_child.payload`,
				},
			},
			inlined: `has(unpacked_child.payload.single_int32_wrapper)`,
			folded:  `has(unpacked_child.payload.single_int32_wrapper)`,
		},
		{
			expr: `has(msg.child.payload.single_int32_wrapper)`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
				},
				{
					name: "unpacked_payload",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.child.payload.single_int32_wrapper",
					t:     cel.NullableType(cel.IntType),
					alias: "payload",
					expr:  `unpacked_payload.single_int32_wrapper`,
				},
			},
			inlined: `has(unpacked_payload.single_int32_wrapper)`,
			folded:  `has(unpacked_payload.single_int32_wrapper)`,
		},
		{
			expr: `has(msg.child.payload.single_int32_wrapper) ? msg.child.payload.single_int32_wrapper : 1`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
				},
				{
					name: "unpacked_payload",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.child.payload.single_int32_wrapper",
					t:     cel.NullableType(cel.IntType),
					alias: "nullable_int",
					expr:  `unpacked_payload.single_int32_wrapper`,
				},
			},
			inlined: `cel.bind(nullable_int, unpacked_payload.single_int32_wrapper, (nullable_int != null) ? nullable_int : 1)`,
			folded:  `cel.bind(nullable_int, unpacked_payload.single_int32_wrapper, (nullable_int != null) ? nullable_int : 1)`,
		},
		{
			expr: `has(msg.single_value) ? msg.single_value : null`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_value",
					t:     cel.NullableType(cel.DoubleType),
					alias: "nullable_float",
					expr:  `dyn(1.5)`,
				},
			},
			inlined: `cel.bind(nullable_float, dyn(1.5), (nullable_float != null) ? nullable_float : null)`,
			folded:  `1.5`,
		},
		{
			expr: `has(msg.single_any) ? msg.single_any : 42`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any",
					t:     cel.IntType,
					alias: "unpacked_nested",
					expr:  `proto3.test.NestedTestAllTypes{}.payload.single_int32`,
				},
			},
			inlined: `has(google.expr.proto3.test.NestedTestAllTypes{}.payload.single_int32) ? google.expr.proto3.test.NestedTestAllTypes{}.payload.single_int32 : 42`,
			folded:  `42`,
		},
		{
			expr: `has(msg.single_any.processing_purpose)`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "unpacked_purpose",
					t:    cel.ListType(cel.IntType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any.processing_purpose",
					t:     cel.ListType(cel.IntType),
					alias: "unpacked_purpose",
					expr:  `[1, 2, 3].map(i, i * 2)`,
				},
			},
			inlined: `[1, 2, 3].map(i, i * 2).size() != 0`,
			folded:  `true`,
		},
		{
			expr: `has(msg.single_any.processing_purpose) ? msg.single_any.processing_purpose[0] : 42`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "unpacked_purpose",
					t:    cel.ListType(cel.IntType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any.processing_purpose",
					t:     cel.ListType(cel.IntType),
					alias: "unpacked_purpose",
					expr:  `[1, 2, 3].map(i, i * 2)`,
				},
			},
			inlined: `cel.bind(unpacked_purpose, [1, 2, 3].map(i, i * 2), (unpacked_purpose.size() != 0) ? (unpacked_purpose[0]) : 42)`,
			folded:  `2`,
		},
		{
			expr: `has(msg.single_any.processing_purpose) ? msg.single_any.processing_purpose.map(i, i * 2)[0] : 42`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "unpacked_purpose",
					t:    cel.ListType(cel.IntType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any.processing_purpose",
					t:     cel.ListType(cel.IntType),
					alias: "unpacked_purpose",
					expr:  `[1, 2, 3].map(i, i * 2)`,
				},
			},
			inlined: `cel.bind(unpacked_purpose, [1, 2, 3].map(i, i * 2), (unpacked_purpose.size() != 0) ? (unpacked_purpose.map(i, i * 2)[0]) : 42)`,
			folded:  `4`,
		},
		{
			expr: `msg.single_any.processing_purpose.filter(j,
							j < msg.single_any.processing_purpose.size()) == [2]`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "unpacked_purpose",
					t:    cel.ListType(cel.IntType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any.processing_purpose",
					t:     cel.ListType(cel.IntType),
					alias: "unpacked_purpose",
					expr:  `[1, 2, 3].map(i, i * 2)`,
				},
			},
			inlined: `cel.bind(unpacked_purpose, [1, 2, 3].map(i, i * 2), unpacked_purpose.filter(j, j < unpacked_purpose.size())) == [2]`,
			folded:  `true`,
		},
		{
			expr: `has(msg.single_any.listA) && msg.single_any.listB.size() > 0 &&
				   msg.single_any.listB.all(b, b == msg.single_any.listA[0]) &&
				   msg.single_any.listA.all(a, a == msg.single_any.listB[0])`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "listA",
					t:    cel.ListType(cel.IntType),
				},
				{
					name: "listB",
					t:    cel.ListType(cel.IntType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any.listA",
					t:     cel.ListType(cel.IntType),
					alias: "listA",
					expr:  `[1, 1]`,
				},
				{
					name:  "msg.single_any.listB",
					t:     cel.ListType(cel.IntType),
					alias: "listB",
					expr:  `[1, 1, 1]`,
				},
			},
			inlined: "cel.bind(listA, [1, 1], cel.bind(listB, [1, 1, 1], listA.size() != 0 && listB.size() > 0 &&\nlistB.all(b, b == listA[0]) && listA.all(a, a == listB[0])))",
			folded:  `true`,
		},
		{
			expr: `((msg.single_any.listB.all(b, b == msg.single_any.listA[0]) &&
				   msg.single_any.listA.all(a, a == msg.single_any.listB[0])) ||
				   msg.single_any.listA.size() == 0) ||
				   false`,
			vars: []varDecl{
				{
					name: "msg",
					t:    cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				},
				{
					name: "listA",
					t:    cel.ListType(cel.IntType),
				},
				{
					name: "listB",
					t:    cel.ListType(cel.IntType),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "msg.single_any.listA",
					t:     cel.ListType(cel.IntType),
					alias: "listA",
					expr:  `[1, 1]`,
				},
				{
					name:  "msg.single_any.listB",
					t:     cel.ListType(cel.IntType),
					alias: "listB",
					expr:  `[1, 1, 1]`,
				},
			},
			inlined: "cel.bind(listA, [1, 1], cel.bind(listB, [1, 1, 1], listB.all(b, b == listA[0]) &&\nlistA.all(a, a == listB[0])) || listA.size() == 0) || false",
			folded:  `true`,
		},
		{
			expr: `has(m.child) && has(m.child.payload)`,
			vars: []varDecl{
				{
					name: "m",
					t:    cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
				},
				{
					name: "m_view",
					t:    cel.MapType(cel.StringType, cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes")),
				},
			},
			inlineVars: []inlineVarExpr{
				{
					name:  "m.child",
					t:     cel.ObjectType("google.expr.proto3.test.NestedTestAllTypes"),
					alias: "child",
					expr:  "m_view.nested.child",
				},
			},
			inlined: "has(m_view.nested.child) && has(m_view.nested.child.payload)",
			folded:  "has(m_view.nested.child) && has(m_view.nested.child.payload)",
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			opts := []cel.EnvOption{
				cel.Container("google.expr"),
				cel.Types(&proto3pb.TestAllTypes{}),
				cel.OptionalTypes(),
				cel.EnableMacroCallTracking()}

			varDecls := make([]cel.EnvOption, len(tc.vars))
			for i, v := range tc.vars {
				varDecls[i] = cel.Variable(v.name, v.t)
			}
			e, err := cel.NewEnv(append(varDecls, opts...)...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			inlinedVars := []*cel.InlineVariable{}
			for _, v := range tc.inlineVars {
				if v.expr == "" {
					continue
				}
				checked, iss := e.Compile(v.expr)
				if iss.Err() != nil {
					t.Fatalf("Compile(%q) failed: %v", v.expr, iss.Err())
				}
				if v.alias == "" {
					inlinedVars = append(inlinedVars, cel.NewInlineVariable(v.name, checked))
				} else {
					inlinedVars = append(inlinedVars, cel.NewInlineVariableWithAlias(v.name, v.alias, checked))
				}
			}
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}

			opt, err := cel.NewStaticOptimizer(cel.NewInliningOptimizer(inlinedVars...))
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			cel.AssertNormalizedIDs(t, optimized)
			inlined, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if inlined != tc.inlined {
				t.Errorf("inlined got %q, wanted %q", inlined, tc.inlined)
			}
			folder, err := cel.NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err = cel.NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss = opt.Optimize(e, optimized)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			cel.AssertNormalizedIDs(t, optimized)
			folded, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("folded got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}
