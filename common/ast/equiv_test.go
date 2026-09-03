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

package ast_test

import (
	"math"
	"testing"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func TestEquivAST(t *testing.T) {
	tests := []struct {
		name  string
		expr1 string
		expr2 string
		equiv bool
	}{
		{
			name:  "identical simple expression",
			expr1: `1 + 2`,
			expr2: `1 + 2`,
			equiv: true,
		},
		{
			name:  "different literals",
			expr1: `1 + 2`,
			expr2: `1 + 3`,
			equiv: false,
		},
		{
			name:  "different operator",
			expr1: `1 + 2`,
			expr2: `1 - 2`,
			equiv: false,
		},
		{
			name:  "member vs global call",
			expr1: `'hello'.size()`,
			expr2: `size('hello')`,
			equiv: false,
		},
		{
			name:  "identical member call",
			expr1: `'hello'.size()`,
			expr2: `'hello'.size()`,
			equiv: true,
		},
		{
			name:  "identical list",
			expr1: `[1, 2, 3]`,
			expr2: `[1, 2, 3]`,
			equiv: true,
		},
		{
			name:  "different list elements",
			expr1: `[1, 2, 3]`,
			expr2: `[1, 2, 4]`,
			equiv: false,
		},
		{
			name:  "different list size",
			expr1: `[1, 2]`,
			expr2: `[1, 2, 3]`,
			equiv: false,
		},
		{
			name:  "optional list elements",
			expr1: `[?msg.?single_int32, 1]`,
			expr2: `[?msg.?single_int32, 1]`,
			equiv: true,
		},
		{
			name:  "different optional indices in list",
			expr1: `[?msg.?single_int32, 1]`,
			expr2: `[1, ?msg.?single_int32]`,
			equiv: false,
		},
		{
			name:  "identical map",
			expr1: `{'a': 1, 'b': 2}`,
			expr2: `{'a': 1, 'b': 2}`,
			equiv: true,
		},
		{
			name:  "different map values",
			expr1: `{'a': 1}`,
			expr2: `{'a': 2}`,
			equiv: false,
		},
		{
			name:  "different map keys",
			expr1: `{'a': 1}`,
			expr2: `{'b': 1}`,
			equiv: false,
		},
		{
			name:  "identical optional map entry",
			expr1: `{?'a': msg.?single_int32}`,
			expr2: `{?'a': msg.?single_int32}`,
			equiv: true,
		},
		{
			name:  "optional vs non-optional map entry",
			expr1: `{?'a': msg.?single_int32}`,
			expr2: `{'a': msg.single_int32}`,
			equiv: false,
		},
		{
			name:  "identical select",
			expr1: `msg.single_int32`,
			expr2: `msg.single_int32`,
			equiv: true,
		},
		{
			name:  "different select field",
			expr1: `msg.single_int32`,
			expr2: `msg.single_int64`,
			equiv: false,
		},
		{
			name:  "presence test select",
			expr1: `has(msg.single_int32)`,
			expr2: `has(msg.single_int32)`,
			equiv: true,
		},
		{
			name:  "presence test vs regular select",
			expr1: `has(msg.single_int32)`,
			expr2: `msg.single_int32`,
			equiv: false,
		},
		{
			name:  "identical struct",
			expr1: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			expr2: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			equiv: true,
		},
		{
			name:  "different struct field value",
			expr1: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			expr2: `google.expr.proto3.test.TestAllTypes{single_int32: 2}`,
			equiv: false,
		},
		{
			name:  "different struct type name",
			expr1: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			expr2: `google.expr.proto3.test.NestedTestAllTypes{}`,
			equiv: false,
		},
		{
			name:  "identical comprehension",
			expr1: `[1, 2, 3].exists(i, i % 2 == 1)`,
			expr2: `[1, 2, 3].exists(i, i % 2 == 1)`,
			equiv: true,
		},
		{
			name:  "different comprehension predicate",
			expr1: `[1, 2, 3].exists(i, i % 2 == 1)`,
			expr2: `[1, 2, 3].exists(i, i % 2 == 0)`,
			equiv: false,
		},
		{
			name:  "different comprehension range",
			expr1: `[1, 2, 3].exists(i, i > 0)`,
			expr2: `[1, 2].exists(i, i > 0)`,
			equiv: false,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast1 := mustTypeCheck(t, tc.expr1)
			ast2 := mustTypeCheck(t, tc.expr2)
			if got := ast.EquivAST(ast1, ast2); got != tc.equiv {
				t.Errorf("ast.EquivAST(%q, %q) = %v, want %v", tc.expr1, tc.expr2, got, tc.equiv)
			}
			if got := ast1.Equiv(ast2); got != tc.equiv {
				t.Errorf("ast1.Equiv(ast2) = %v, want %v", got, tc.equiv)
			}
		})
	}
}

func TestEquivRenumberedIDs(t *testing.T) {
	ast1 := mustTypeCheck(t, `[1, 2, 3].exists(i, i % 2 == 1)`)
	ast2 := ast.Copy(ast1)

	// Renumber ast2 with an offset
	idGen := testIDGen(500)
	ast2.Expr().RenumberIDs(idGen)
	newTypeMap := make(map[int64]*types.Type)
	for id, t := range ast1.TypeMap() {
		newTypeMap[idGen(id)] = t
	}
	newRefMap := make(map[int64]*ast.ReferenceInfo)
	for id, r := range ast1.ReferenceMap() {
		newRefMap[idGen(id)] = r
	}
	ast2 = ast.NewCheckedAST(ast.NewAST(ast2.Expr(), ast.CopySourceInfo(ast1.SourceInfo())), newTypeMap, newRefMap)

	if !ast.EquivAST(ast1, ast2) {
		t.Errorf("ast.EquivAST(ast1, renumberedAst2) = false, want true")
	}
	if !ast1.Equiv(ast2) {
		t.Errorf("ast1.Equiv(renumberedAst2) = false, want true")
	}
}

func TestEquivExpr(t *testing.T) {
	fac := ast.NewExprFactory()
	e1 := fac.NewCall(1, "_+_", fac.NewLiteral(2, types.Int(1)), fac.NewIdent(3, "x"))
	e2 := fac.NewCall(10, "_+_", fac.NewLiteral(20, types.Int(1)), fac.NewIdent(30, "x"))
	e3 := fac.NewCall(10, "_+_", fac.NewLiteral(20, types.Int(2)), fac.NewIdent(30, "x"))

	if !ast.EquivExpr(e1, e2) {
		t.Errorf("ast.EquivExpr(e1, e2) = false, want true")
	}
	if ast.EquivExpr(e1, e3) {
		t.Errorf("ast.EquivExpr(e1, e3) = true, want false")
	}
}

func TestEquivWithOptions(t *testing.T) {
	fac := ast.NewExprFactory()
	e1 := fac.NewIdent(1, "x")
	e2 := fac.NewIdent(10, "x")

	types1 := map[int64]*types.Type{1: types.IntType}
	types2 := map[int64]*types.Type{10: types.IntType}
	typesDiff := map[int64]*types.Type{10: types.StringType}

	refs1 := map[int64]*ast.ReferenceInfo{1: ast.NewIdentReference("x", nil)}
	refs2 := map[int64]*ast.ReferenceInfo{10: ast.NewIdentReference("x", nil)}
	refsDiff := map[int64]*ast.ReferenceInfo{10: ast.NewIdentReference("y", nil)}

	// Matching types and references
	if !ast.EquivExpr(e1, e2, ast.EquivTypes(types1, types2), ast.EquivReferences(refs1, refs2)) {
		t.Errorf("ast.EquivExpr with matching types/refs = false, want true")
	}

	// Mismatched types
	if ast.EquivExpr(e1, e2, ast.EquivTypes(types1, typesDiff)) {
		t.Errorf("ast.EquivExpr with mismatched types = true, want false")
	}

	// Missing type on one side
	if ast.EquivExpr(e1, e2, ast.EquivTypes(types1, map[int64]*types.Type{})) {
		t.Errorf("ast.EquivExpr with missing type on one side = true, want false")
	}

	// Mismatched references
	if ast.EquivExpr(e1, e2, ast.EquivReferences(refs1, refsDiff)) {
		t.Errorf("ast.EquivExpr with mismatched refs = true, want false")
	}

	// Overload IDs in references
	fnRef1 := map[int64]*ast.ReferenceInfo{1: ast.NewFunctionReference(overloads.AddInt64, overloads.AddUint64)}
	fnRef2 := map[int64]*ast.ReferenceInfo{10: ast.NewFunctionReference(overloads.AddUint64, overloads.AddInt64)}
	fnRefDiff := map[int64]*ast.ReferenceInfo{10: ast.NewFunctionReference(overloads.AddInt64)}

	if !ast.EquivExpr(e1, e2, ast.EquivReferences(fnRef1, fnRef2)) {
		t.Errorf("ast.EquivExpr with reordered overloads = false, want true")
	}
	if ast.EquivExpr(e1, e2, ast.EquivReferences(fnRef1, fnRefDiff)) {
		t.Errorf("ast.EquivExpr with different overloads = true, want false")
	}
}

func TestEquivIgnoreNames(t *testing.T) {
	fac := ast.NewExprFactory()

	// Identifiers with different names
	identA := fac.NewIdent(1, "a")
	identB := fac.NewIdent(2, "b")
	if ast.EquivExpr(identA, identB) {
		t.Errorf("ast.EquivExpr(identA, identB) = true, want false")
	}
	if !ast.EquivExpr(identA, identB, ast.EquivIgnoreNames()) {
		t.Errorf("ast.EquivExpr(identA, identB, EquivIgnoreNames()) = false, want true")
	}
	if !ast.EquivExpr(identA, identB, ast.EquivIgnoreNames(true)) {
		t.Errorf("ast.EquivExpr(identA, identB, EquivIgnoreNames(true)) = false, want true")
	}
	if ast.EquivExpr(identA, identB, ast.EquivIgnoreNames(false)) {
		t.Errorf("ast.EquivExpr(identA, identB, EquivIgnoreNames(false)) = true, want false")
	}

	// Select expressions with different field names
	selA := fac.NewSelect(1, identA, "fieldA")
	selB := fac.NewSelect(2, identA, "fieldB")
	if ast.EquivExpr(selA, selB) {
		t.Errorf("ast.EquivExpr(selA, selB) = true, want false")
	}
	if !ast.EquivExpr(selA, selB, ast.EquivIgnoreNames()) {
		t.Errorf("ast.EquivExpr(selA, selB, EquivIgnoreNames()) = false, want true")
	}

	// Struct fields with different names
	structA := fac.NewStruct(1, "MyStruct", []ast.EntryExpr{
		fac.NewStructField(2, "fieldA", fac.NewLiteral(3, types.Int(1)), false),
	})
	structB := fac.NewStruct(10, "MyStruct", []ast.EntryExpr{
		fac.NewStructField(20, "fieldB", fac.NewLiteral(30, types.Int(1)), false),
	})
	if ast.EquivExpr(structA, structB) {
		t.Errorf("ast.EquivExpr(structA, structB) = true, want false")
	}
	if !ast.EquivExpr(structA, structB, ast.EquivIgnoreNames()) {
		t.Errorf("ast.EquivExpr(structA, structB, EquivIgnoreNames()) = false, want true")
	}

	// Comprehension variables with different names
	compA := fac.NewComprehension(1,
		fac.NewList(2, []ast.Expr{}, []int32{}),
		"i", "__result__",
		fac.NewLiteral(3, types.False),
		fac.NewLiteral(4, types.True),
		fac.NewIdent(5, "i"),
		fac.NewIdent(6, "__result__"),
	)
	compB := fac.NewComprehension(10,
		fac.NewList(20, []ast.Expr{}, []int32{}),
		"x", "@result",
		fac.NewLiteral(30, types.False),
		fac.NewLiteral(40, types.True),
		fac.NewIdent(50, "x"),
		fac.NewIdent(60, "@result"),
	)
	if ast.EquivExpr(compA, compB) {
		t.Errorf("ast.EquivExpr(compA, compB) = true, want false")
	}
	if !ast.EquivExpr(compA, compB, ast.EquivIgnoreNames()) {
		t.Errorf("ast.EquivExpr(compA, compB, EquivIgnoreNames()) = false, want true")
	}

	// References with different names
	refsA := map[int64]*ast.ReferenceInfo{1: ast.NewIdentReference("a", nil)}
	refsB := map[int64]*ast.ReferenceInfo{2: ast.NewIdentReference("b", nil)}
	if !ast.EquivExpr(identA, identB, ast.EquivReferences(refsA, refsB), ast.EquivIgnoreNames()) {
		t.Errorf("ast.EquivExpr with different ref names + EquivIgnoreNames() = false, want true")
	}
}

func TestEquivCheckedVsUnchecked(t *testing.T) {
	checked := mustTypeCheck(t, `1 + 2`)
	parsed := ast.NewAST(checked.Expr(), nil)

	// Checked vs Unchecked should fail by default because types/refs are checked
	if ast.EquivAST(checked, parsed) {
		t.Errorf("ast.EquivAST(checked, parsed) = true, want false")
	}

	// Disabling types and references explicitly should pass
	if !ast.EquivAST(checked, parsed, ast.EquivTypes(nil, nil), ast.EquivReferences(nil, nil)) {
		t.Errorf("ast.EquivAST(checked, parsed, ignore types/refs) = false, want true")
	}
}

func TestEquivLiterals(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name  string
		l1    ref.Val
		l2    ref.Val
		equiv bool
	}{
		{name: "bool equal", l1: types.True, l2: types.True, equiv: true},
		{name: "bool not equal", l1: types.True, l2: types.False, equiv: false},
		{name: "int equal", l1: types.Int(42), l2: types.Int(42), equiv: true},
		{name: "int not equal", l1: types.Int(42), l2: types.Int(43), equiv: false},
		{name: "uint equal", l1: types.Uint(42), l2: types.Uint(42), equiv: true},
		{name: "int vs uint not equal", l1: types.Int(42), l2: types.Uint(42), equiv: false},
		{name: "int vs double not equal", l1: types.Int(42), l2: types.Double(42.0), equiv: false},
		{name: "string equal", l1: types.String("hello"), l2: types.String("hello"), equiv: true},
		{name: "string not equal", l1: types.String("hello"), l2: types.String("world"), equiv: false},
		{name: "bytes equal", l1: types.Bytes("bytes"), l2: types.Bytes("bytes"), equiv: true},
		{name: "bytes not equal", l1: types.Bytes("bytes"), l2: types.Bytes("other"), equiv: false},
		{name: "null equal", l1: types.NullValue, l2: types.NullValue, equiv: true},
		{name: "NaN double equal", l1: types.Double(math.NaN()), l2: types.Double(math.NaN()), equiv: true},
		{name: "nil vs val", l1: nil, l2: types.True, equiv: false},
		{name: "nil vs nil", l1: nil, l2: nil, equiv: true},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			e1 := fac.NewLiteral(1, tc.l1)
			e2 := fac.NewLiteral(2, tc.l2)
			if got := ast.EquivExpr(e1, e2); got != tc.equiv {
				t.Errorf("Equiv literals (%v, %v) = %v, want %v", tc.l1, tc.l2, got, tc.equiv)
			}
		})
	}
}

func TestEquivNilSafety(t *testing.T) {
	fac := ast.NewExprFactory()
	astValid := ast.NewAST(fac.NewIdent(1, "x"), nil)
	exprValid := fac.NewIdent(1, "x")

	var nilAST *ast.AST = nil

	if !ast.EquivAST(nil, nil) {
		t.Errorf("ast.EquivAST(nil, nil) = false, want true")
	}
	if !nilAST.Equiv(nil) {
		t.Errorf("nilAST.Equiv(nil) = false, want true")
	}
	if ast.EquivAST(nil, astValid) {
		t.Errorf("ast.EquivAST(nil, astValid) = true, want false")
	}
	if ast.EquivAST(astValid, nil) {
		t.Errorf("ast.EquivAST(astValid, nil) = true, want false")
	}
	if astValid.Equiv(nil) {
		t.Errorf("astValid.Equiv(nil) = true, want false")
	}

	if !ast.EquivExpr(nil, nil) {
		t.Errorf("ast.EquivExpr(nil, nil) = false, want true")
	}
	if ast.EquivExpr(nil, exprValid) {
		t.Errorf("ast.EquivExpr(nil, exprValid) = true, want false")
	}
	if ast.EquivExpr(exprValid, nil) {
		t.Errorf("ast.EquivExpr(exprValid, nil) = true, want false")
	}

	// Unspecified exprs
	unspec1 := fac.NewUnspecifiedExpr(1)
	unspec2 := fac.NewUnspecifiedExpr(2)
	nilExpr := ast.NewAST(nil, nil).Expr()
	if !ast.EquivExpr(unspec1, unspec2) {
		t.Errorf("ast.EquivExpr(unspec1, unspec2) = false, want true")
	}
	if !ast.EquivExpr(unspec1, nilExpr) {
		t.Errorf("ast.EquivExpr(unspec1, nilExpr) = false, want true")
	}
	if !ast.EquivExpr(nilExpr, unspec1) {
		t.Errorf("ast.EquivExpr(nilExpr, unspec1) = false, want true")
	}
	if ast.EquivExpr(nilExpr, nil) {
		t.Errorf("ast.EquivExpr(nilExpr, nil) = true, want false")
	}
	if ast.EquivExpr(unspec1, nil) {
		t.Errorf("ast.EquivExpr(unspec1, nil) = true, want false")
	}
	if ast.EquivExpr(nil, unspec1) {
		t.Errorf("ast.EquivExpr(nil, unspec1) = true, want false")
	}
	if ast.EquivExpr(exprValid, unspec1) {
		t.Errorf("ast.EquivExpr(exprValid, unspec1) = true, want false")
	}
}

func TestEquivComprehensionTwoVar(t *testing.T) {
	fac := ast.NewExprFactory()
	comp1 := fac.NewComprehensionTwoVar(1,
		fac.NewList(2, []ast.Expr{}, []int32{}),
		"i", "v", "__result__",
		fac.NewLiteral(3, types.Int(0)),
		fac.NewLiteral(4, types.True),
		fac.NewCall(5, "_+_", fac.NewIdent(6, "i"), fac.NewIdent(7, "v")),
		fac.NewIdent(8, "__result__"),
	)
	comp2 := fac.NewComprehensionTwoVar(10,
		fac.NewList(20, []ast.Expr{}, []int32{}),
		"i", "v", "__result__",
		fac.NewLiteral(30, types.Int(0)),
		fac.NewLiteral(40, types.True),
		fac.NewCall(50, "_+_", fac.NewIdent(60, "i"), fac.NewIdent(70, "v")),
		fac.NewIdent(80, "__result__"),
	)
	compDiffVar2 := fac.NewComprehensionTwoVar(10,
		fac.NewList(20, []ast.Expr{}, []int32{}),
		"i", "k", "__result__",
		fac.NewLiteral(30, types.Int(0)),
		fac.NewLiteral(40, types.True),
		fac.NewCall(50, "_+_", fac.NewIdent(60, "i"), fac.NewIdent(70, "v")),
		fac.NewIdent(80, "__result__"),
	)
	compOneVar := fac.NewComprehension(10,
		fac.NewList(20, []ast.Expr{}, []int32{}),
		"i", "__result__",
		fac.NewLiteral(30, types.Int(0)),
		fac.NewLiteral(40, types.True),
		fac.NewCall(50, "_+_", fac.NewIdent(60, "i"), fac.NewIdent(70, "v")),
		fac.NewIdent(80, "__result__"),
	)

	if !ast.EquivExpr(comp1, comp2) {
		t.Errorf("ast.EquivExpr(comp1, comp2) = false, want true")
	}
	if ast.EquivExpr(comp1, compDiffVar2) {
		t.Errorf("ast.EquivExpr(comp1, compDiffVar2) = true, want false")
	}
	if ast.EquivExpr(comp1, compOneVar) {
		t.Errorf("ast.EquivExpr(comp1, compOneVar) = true, want false")
	}
}

func TestEquivMapAndStructExpr(t *testing.T) {
	fac := ast.NewExprFactory()
	map1 := fac.NewMap(1, []ast.EntryExpr{
		fac.NewMapEntry(2, fac.NewIdent(3, "k"), fac.NewLiteral(4, types.Int(1)), false),
	})
	map2 := fac.NewMap(10, []ast.EntryExpr{
		fac.NewMapEntry(20, fac.NewIdent(30, "k"), fac.NewLiteral(40, types.Int(1)), false),
	})
	mapOpt := fac.NewMap(10, []ast.EntryExpr{
		fac.NewMapEntry(20, fac.NewIdent(30, "k"), fac.NewLiteral(40, types.Int(1)), true),
	})

	struct1 := fac.NewStruct(1, "MyStruct", []ast.EntryExpr{
		fac.NewStructField(2, "field", fac.NewLiteral(3, types.Int(1)), false),
	})
	struct2 := fac.NewStruct(10, "MyStruct", []ast.EntryExpr{
		fac.NewStructField(20, "field", fac.NewLiteral(30, types.Int(1)), false),
	})
	structOpt := fac.NewStruct(10, "MyStruct", []ast.EntryExpr{
		fac.NewStructField(20, "field", fac.NewLiteral(30, types.Int(1)), true),
	})
	structDiffField := fac.NewStruct(10, "MyStruct", []ast.EntryExpr{
		fac.NewStructField(20, "other", fac.NewLiteral(30, types.Int(1)), false),
	})

	if !ast.EquivExpr(map1, map2) {
		t.Errorf("ast.EquivExpr(map1, map2) = false, want true")
	}
	if ast.EquivExpr(map1, mapOpt) {
		t.Errorf("ast.EquivExpr(map1, mapOpt) = true, want false")
	}
	if !ast.EquivExpr(struct1, struct2) {
		t.Errorf("ast.EquivExpr(struct1, struct2) = false, want true")
	}
	if ast.EquivExpr(struct1, structOpt) {
		t.Errorf("ast.EquivExpr(struct1, structOpt) = true, want false")
	}
	if ast.EquivExpr(struct1, structDiffField) {
		t.Errorf("ast.EquivExpr(struct1, structDiffField) = true, want false")
	}
	if ast.EquivExpr(map1, struct1) {
		t.Errorf("ast.EquivExpr(map1, struct1) = true, want false")
	}
}

type customExpr struct {
	ast.Expr
	kind ast.ExprKind
}

func (c *customExpr) Kind() ast.ExprKind {
	return c.kind
}

type customEntryExpr struct {
	ast.EntryExpr
	kind ast.EntryExprKind
}

func (c *customEntryExpr) Kind() ast.EntryExprKind {
	return c.kind
}

func TestEquivBranchCoverage(t *testing.T) {
	fac := ast.NewExprFactory()
	identX := fac.NewIdent(1, "x")
	identY := fac.NewIdent(2, "y")

	t.Run("select presence test mismatch", func(t *testing.T) {
		pres := fac.NewPresenceTest(1, identX, "field")
		sel := fac.NewSelect(2, identX, "field")
		if ast.EquivExpr(pres, sel) {
			t.Errorf("ast.EquivExpr(pres, sel) = true, want false")
		}
	})

	t.Run("call variations", func(t *testing.T) {
		// Different function name
		cDiff1 := fac.NewCall(1, "fn1", identX)
		cDiff2 := fac.NewCall(2, "fn2", identX)
		if ast.EquivExpr(cDiff1, cDiff2) {
			t.Errorf("ast.EquivExpr(cDiff1, cDiff2) with different names = true, want false")
		}

		c1 := fac.NewCall(1, "fn", identX)

		// Member vs global call with same function name
		m1 := fac.NewMemberCall(1, "fn", identX, fac.NewLiteral(2, types.Int(1)))
		g1 := fac.NewCall(3, "fn", identX, fac.NewLiteral(4, types.Int(1)))
		if ast.EquivExpr(m1, g1) {
			t.Errorf("ast.EquivExpr(m1, g1) member vs global = true, want false")
		}

		// Member call target mismatch
		m2 := fac.NewMemberCall(5, "fn", identY, fac.NewLiteral(6, types.Int(1)))
		if ast.EquivExpr(m1, m2) {
			t.Errorf("ast.EquivExpr(m1, m2) target mismatch = true, want false")
		}

		// Call argument count mismatch
		c3 := fac.NewCall(7, "fn", identX, identY)
		if ast.EquivExpr(c1, c3) {
			t.Errorf("ast.EquivExpr(c1, c3) arg count mismatch = true, want false")
		}

		// Call argument value mismatch
		c4 := fac.NewCall(8, "fn", identY)
		if ast.EquivExpr(c1, c4) {
			t.Errorf("ast.EquivExpr(c1, c4) arg mismatch = true, want false")
		}
	})

	t.Run("list variations", func(t *testing.T) {
		// Optional indices length mismatch
		l1 := fac.NewList(1, []ast.Expr{identX, identY}, []int32{0})
		l2 := fac.NewList(2, []ast.Expr{identX, identY}, []int32{0, 1})
		if ast.EquivExpr(l1, l2) {
			t.Errorf("ast.EquivExpr(l1, l2) optional indices length mismatch = true, want false")
		}

		// Element mismatch
		l3 := fac.NewList(3, []ast.Expr{identX}, nil)
		l4 := fac.NewList(4, []ast.Expr{identY}, nil)
		if ast.EquivExpr(l3, l4) {
			t.Errorf("ast.EquivExpr(l3, l4) element mismatch = true, want false")
		}
	})

	t.Run("map variations", func(t *testing.T) {
		// Map size mismatch
		m1 := fac.NewMap(1, []ast.EntryExpr{
			fac.NewMapEntry(2, identX, fac.NewLiteral(3, types.Int(1)), false),
		})
		m2 := fac.NewMap(4, []ast.EntryExpr{})
		if ast.EquivExpr(m1, m2) {
			t.Errorf("ast.EquivExpr(m1, m2) map size mismatch = true, want false")
		}

		// Nil entry in map
		mNil1 := fac.NewMap(5, []ast.EntryExpr{nil})
		mNil2 := fac.NewMap(6, []ast.EntryExpr{nil})
		if !ast.EquivExpr(mNil1, mNil2) {
			t.Errorf("ast.EquivExpr(mNil1, mNil2) = false, want true")
		}
		if ast.EquivExpr(mNil1, m1) {
			t.Errorf("ast.EquivExpr(mNil1, m1) = true, want false")
		}
		if ast.EquivExpr(m1, mNil1) {
			t.Errorf("ast.EquivExpr(m1, mNil1) = true, want false")
		}
	})

	t.Run("struct variations", func(t *testing.T) {
		// Struct type mismatch
		s1 := fac.NewStruct(1, "TypeA", []ast.EntryExpr{})
		s2 := fac.NewStruct(2, "TypeB", []ast.EntryExpr{})
		if ast.EquivExpr(s1, s2) {
			t.Errorf("ast.EquivExpr(s1, s2) type mismatch = true, want false")
		}

		// Field count mismatch
		s3 := fac.NewStruct(3, "TypeA", []ast.EntryExpr{
			fac.NewStructField(4, "f", identX, false),
		})
		if ast.EquivExpr(s1, s3) {
			t.Errorf("ast.EquivExpr(s1, s3) field count mismatch = true, want false")
		}

		// Struct field value mismatch
		s4 := fac.NewStruct(5, "TypeA", []ast.EntryExpr{
			fac.NewStructField(6, "f", identY, false),
		})
		if ast.EquivExpr(s3, s4) {
			t.Errorf("ast.EquivExpr(s3, s4) field value mismatch = true, want false")
		}

		// Entry kind mismatch (StructField vs MapEntry)
		sMixed := fac.NewStruct(7, "TypeA", []ast.EntryExpr{
			fac.NewMapEntry(8, identX, identY, false),
		})
		if ast.EquivExpr(s3, sMixed) {
			t.Errorf("ast.EquivExpr(s3, sMixed) entry kind mismatch = true, want false")
		}
	})

	t.Run("comprehension variable mismatches", func(t *testing.T) {
		comp1 := fac.NewComprehension(1, fac.NewList(2, nil, nil), "i", "acc",
			fac.NewLiteral(3, types.Int(0)), fac.NewLiteral(4, types.True),
			fac.NewIdent(5, "acc"), fac.NewIdent(6, "acc"))
		compDiffIter := fac.NewComprehension(10, fac.NewList(20, nil, nil), "j", "acc",
			fac.NewLiteral(30, types.Int(0)), fac.NewLiteral(40, types.True),
			fac.NewIdent(50, "acc"), fac.NewIdent(60, "acc"))
		compDiffAcc := fac.NewComprehension(10, fac.NewList(20, nil, nil), "i", "acc2",
			fac.NewLiteral(30, types.Int(0)), fac.NewLiteral(40, types.True),
			fac.NewIdent(50, "acc2"), fac.NewIdent(60, "acc2"))

		if ast.EquivExpr(comp1, compDiffIter) {
			t.Errorf("ast.EquivExpr iter var mismatch = true, want false")
		}
		if ast.EquivExpr(comp1, compDiffAcc) {
			t.Errorf("ast.EquivExpr accu var mismatch = true, want false")
		}
	})

	t.Run("type map branches", func(t *testing.T) {
		// Both types nil in map
		tNil1 := map[int64]*types.Type{1: nil}
		tNil2 := map[int64]*types.Type{2: nil}
		if !ast.EquivExpr(identX, identY, ast.EquivTypes(tNil1, tNil2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with both types nil in map = false, want true")
		}

		// One type nil, other non-nil in map
		tInt1 := map[int64]*types.Type{1: types.IntType}
		tInt2 := map[int64]*types.Type{2: types.IntType}
		if ast.EquivExpr(identX, identY, ast.EquivTypes(tNil1, tInt2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with one type nil = true, want false")
		}
		if ast.EquivExpr(identX, identY, ast.EquivTypes(tInt1, tNil2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with one type nil (inverted) = true, want false")
		}
	})

	t.Run("reference map branches", func(t *testing.T) {
		// Reference presence mismatch
		rValid1 := map[int64]*ast.ReferenceInfo{1: ast.NewIdentReference("x", nil)}
		rValid2 := map[int64]*ast.ReferenceInfo{2: ast.NewIdentReference("x", nil)}
		rEmpty := map[int64]*ast.ReferenceInfo{}
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rValid1, rEmpty)) {
			t.Errorf("ast.EquivExpr with ref presence mismatch = true, want false")
		}
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rEmpty, rValid2)) {
			t.Errorf("ast.EquivExpr with ref presence mismatch (inverted) = true, want false")
		}

		// Both nil ReferenceInfo in map
		rNil1 := map[int64]*ast.ReferenceInfo{1: nil}
		rNil2 := map[int64]*ast.ReferenceInfo{2: nil}
		if !ast.EquivExpr(identX, identY, ast.EquivReferences(rNil1, rNil2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with both nil ReferenceInfo = false, want true")
		}

		// One nil, other non-nil ReferenceInfo
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rNil1, rValid2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with one nil ReferenceInfo = true, want false")
		}
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rValid1, rNil2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with one nil ReferenceInfo (inverted) = true, want false")
		}

		// Constant value references
		rConst1 := map[int64]*ast.ReferenceInfo{1: {Value: types.Int(42)}}
		rConst2 := map[int64]*ast.ReferenceInfo{2: {Value: types.Int(42)}}
		rConst3 := map[int64]*ast.ReferenceInfo{2: {Value: types.Int(99)}}
		rIdent1 := map[int64]*ast.ReferenceInfo{1: ast.NewIdentReference("x", nil)}
		rIdent2 := map[int64]*ast.ReferenceInfo{2: ast.NewIdentReference("x", nil)}

		if !ast.EquivExpr(identX, identY, ast.EquivReferences(rConst1, rConst2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with matching const refs = false, want true")
		}
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rConst1, rConst3), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with different const refs = true, want false")
		}
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rConst1, rIdent2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with const vs ident ref = true, want false")
		}
		if ast.EquivExpr(identX, identY, ast.EquivReferences(rIdent1, rConst2), ast.EquivIgnoreNames()) {
			t.Errorf("ast.EquivExpr with ident vs const ref = true, want false")
		}
	})

	t.Run("entry expr type and reference checks", func(t *testing.T) {
		m1 := fac.NewMap(1, []ast.EntryExpr{
			fac.NewMapEntry(2, identX, fac.NewLiteral(3, types.Int(1)), false),
		})
		m2 := fac.NewMap(10, []ast.EntryExpr{
			fac.NewMapEntry(20, identX, fac.NewLiteral(30, types.Int(1)), false),
		})
		tMismatch1 := map[int64]*types.Type{2: types.IntType}
		tMismatch2 := map[int64]*types.Type{20: types.StringType}
		if ast.EquivExpr(m1, m2, ast.EquivTypes(tMismatch1, tMismatch2)) {
			t.Errorf("ast.EquivExpr with mismatched entry types = true, want false")
		}
	})

	t.Run("custom / unknown expr and entry expr kinds", func(t *testing.T) {
		customE1 := &customExpr{Expr: identX, kind: ast.ExprKind(999)}
		customE2 := &customExpr{Expr: identY, kind: ast.ExprKind(999)}
		if ast.EquivExpr(customE1, customE2) {
			t.Errorf("ast.EquivExpr with unknown ExprKind = true, want false")
		}

		customEntry1 := &customEntryExpr{EntryExpr: fac.NewMapEntry(1, identX, identY, false), kind: ast.EntryExprKind(999)}
		customEntry2 := &customEntryExpr{EntryExpr: fac.NewMapEntry(2, identX, identY, false), kind: ast.EntryExprKind(999)}
		m1 := fac.NewMap(10, []ast.EntryExpr{customEntry1})
		m2 := fac.NewMap(20, []ast.EntryExpr{customEntry2})
		if !ast.EquivExpr(m1, m2) {
			t.Errorf("ast.EquivExpr with unknown EntryExprKind default = false, want true")
		}
	})

	t.Run("literal NaN vs number comparison", func(t *testing.T) {
		eNaN := fac.NewLiteral(1, types.Double(math.NaN()))
		eNum := fac.NewLiteral(2, types.Double(1.0))
		if ast.EquivExpr(eNaN, eNum) {
			t.Errorf("ast.EquivExpr(NaN, 1.0) = true, want false")
		}
		if ast.EquivExpr(eNum, eNaN) {
			t.Errorf("ast.EquivExpr(1.0, NaN) = true, want false")
		}
	})
}

