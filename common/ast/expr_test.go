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

package ast_test

import (
	"fmt"
	"reflect"
	"testing"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
)

func TestSetKindCase(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []ast.Expr{
		fac.NewUnspecifiedExpr(1),
		fac.NewCall(1, "_==_", fac.NewLiteral(2, types.True), fac.NewLiteral(3, types.False)),
		fac.NewMemberCall(1, overloads.Size, fac.NewLiteral(2, types.String("hello"))),
		fac.NewComprehension(12,
			fac.NewList(1, []ast.Expr{}, []int32{}),
			"i",
			"__result__",
			fac.NewLiteral(5, types.False),
			fac.NewCall(8, "@not_strictly_false", fac.NewCall(7, "!_", fac.NewAccuIdent(6))),
			fac.NewCall(10, "_||_", fac.NewAccuIdent(9), fac.NewIdent(4, "i")),
			fac.NewAccuIdent(11),
		),
		fac.NewIdent(1, "a"),
		fac.NewLiteral(1, types.Bytes("hello")),
		fac.NewList(1, []ast.Expr{fac.NewIdent(2, "a"), fac.NewIdent(3, "b")}, []int32{}),
		fac.NewMap(1, []ast.EntryExpr{
			fac.NewMapEntry(2,
				fac.NewLiteral(3, types.String("string")),
				fac.NewCall(6, "_?._", fac.NewIdent(4, "a"), fac.NewLiteral(5, types.String("b"))),
				true),
		}),
		fac.NewSelect(2, fac.NewIdent(1, "a"), "b"),
		fac.NewStruct(1,
			"custom.StructType",
			[]ast.EntryExpr{
				fac.NewStructField(2,
					"uint_field",
					fac.NewLiteral(3, types.Uint(42)),
					false),
			}),
	}
	expr := nilTestExpr(t)
	for i, tst := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			expr.SetKindCase(tst)
			switch expr.Kind() {
			case ast.CallKind:
				if !reflect.DeepEqual(expr.AsCall(), tst.AsCall()) {
					t.Errorf("got %v, wanted %v", expr.AsCall(), tst.AsCall())
				}
			case ast.ComprehensionKind:
				if !reflect.DeepEqual(expr.AsComprehension(), tst.AsComprehension()) {
					t.Errorf("got %v, wanted %v", expr.AsComprehension(), tst.AsComprehension())
				}
			case ast.IdentKind:
				if !reflect.DeepEqual(expr.AsIdent(), tst.AsIdent()) {
					t.Errorf("got %v, wanted %v", expr.AsIdent(), tst.AsIdent())
				}
			case ast.LiteralKind:
				if !reflect.DeepEqual(expr.AsLiteral(), tst.AsLiteral()) {
					t.Errorf("got %v, wanted %v", expr.AsLiteral(), tst.AsLiteral())
				}
			case ast.ListKind:
				if !reflect.DeepEqual(expr.AsList(), tst.AsList()) {
					t.Errorf("got %v, wanted %v", expr.AsList(), tst.AsList())
				}
			case ast.UnspecifiedExprKind:
				if !reflect.DeepEqual(expr, tst) {
					t.Errorf("got %v, wanted %v", expr, tst)
				}
			case ast.MapKind:
				if !reflect.DeepEqual(expr.AsMap(), tst.AsMap()) {
					t.Errorf("got %v, wanted %v", expr, tst)
				}
			case ast.SelectKind:
				if !reflect.DeepEqual(expr.AsSelect(), tst.AsSelect()) {
					t.Errorf("got %v, wanted %v", expr, tst)
				}
			case ast.StructKind:
				if !reflect.DeepEqual(expr.AsStruct(), tst.AsStruct()) {
					t.Errorf("got %v, wanted %v", expr, tst)
				}
			default:
				t.Errorf("unable to determine kind case: %v", tst)
			}
		})
	}
}

func TestCall(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewCall(1, "size", fac.NewLiteral(2, types.String("hello")))
	if expr.Kind() != ast.CallKind {
		t.Fatalf("NewCall() produced non-call expression: %v", expr.Kind())
	}
	call := expr.AsCall()
	if call.FunctionName() != "size" {
		t.Errorf("Function() got %s, wanted 'size''", call.FunctionName())
	}
	if len(call.Args()) != 1 {
		t.Errorf("Args() got %v, wanted one", call.Args())
	}
	if call.IsMemberFunction() {
		t.Error("IsMemberFunction() got true, wanted false")
	}
	if call.Target().ID() != 0 {
		t.Errorf("empty Target() got %d, wanted 0", call.Target().ID())
	}
	expr.RenumberIDs(testIDGen(100))
	if expr.ID() != 101 {
		t.Errorf("expr.ID() got %d, wanted 101 after RenumberIDs", expr.ID())
	}
	if expr.AsCall().Args()[0].ID() != 102 {
		t.Errorf("Args()[0].ID() got %d, wanted 102 after RenumberIDs", expr.AsCall().Args()[0].ID())
	}
}

func TestMemberCall(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewMemberCall(1, "size", fac.NewLiteral(2, types.String("hello")))
	if expr.Kind() != ast.CallKind {
		t.Fatalf("NewCall() produced non-call expression: %v", expr.Kind())
	}
	call := expr.AsCall()
	if call.FunctionName() != "size" {
		t.Errorf("Function() got %s, wanted 'size''", call.FunctionName())
	}
	if len(call.Args()) != 0 {
		t.Errorf("Args() got %v, wanted zero", call.Args())
	}
	if !call.IsMemberFunction() {
		t.Error("IsMemberFunction() got false, wanted true")
	}
	if call.Target().ID() != 2 {
		t.Errorf("Target() got %d, wanted 2", call.Target().ID())
	}
	expr.RenumberIDs(testIDGen(100))
	if expr.ID() != 101 {
		t.Errorf("expr.ID() got %d, wanted 101 after RenumberIDs", expr.ID())
	}
	if expr.AsCall().Target().ID() != 102 {
		t.Errorf("Target().ID() got %d, wanted 102 after RenumberIDs", expr.AsCall().Target().ID())
	}
}

func TestCallNil(t *testing.T) {
	expr := nilTestExpr(t)
	call := expr.AsCall()
	if call.FunctionName() != "" {
		t.Errorf("FunctionName() got %s, wanted empty value", call.FunctionName())
	}
	if len(call.Args()) != 0 {
		t.Errorf("Args() got %v, wanted zero", call.Args())
	}
	if call.IsMemberFunction() {
		t.Error("IsMemberFunction() got true, wanted false")
	}
	if call.Target().ID() != 0 {
		t.Errorf("empty Target() got %d, wanted 0", call.Target().ID())
	}
	expr.RenumberIDs(testIDGen(100))
	if expr.ID() != 101 {
		t.Errorf("RenumberIDs() got %d, wanted 101", expr.ID())
	}
}

func TestComprehension(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewComprehension(1,
		fac.NewList(2, []ast.Expr{
			fac.NewLiteral(3, types.Int(1)),
			fac.NewLiteral(4, types.Int(2)),
		}, []int32{}),
		"i",
		"__result__",
		fac.NewLiteral(5, types.False),
		fac.NewLiteral(6, types.True),
		fac.NewCall(7, "_||_",
			fac.NewAccuIdent(8),
			fac.NewCall(9, "<", fac.NewIdent(10, "i"), fac.NewLiteral(11, types.Int(3))),
		),
		fac.NewAccuIdent(12),
	)
	comp := expr.AsComprehension()
	if comp.IterRange().Kind() != ast.ListKind {
		t.Errorf("IterRange() got %v, wanted list", comp.IterRange().Kind())
	}
	if comp.AccuInit().Kind() != ast.LiteralKind {
		t.Errorf("AccuInit() got %v, wanted literal", comp.AccuInit().Kind())
	}
	if comp.LoopCondition().Kind() != ast.LiteralKind {
		t.Errorf("LoopCondition() got %v, wanted literal", comp.LoopCondition().Kind())
	}
	if comp.LoopStep().Kind() != ast.CallKind {
		t.Errorf("LoopStep() got %v, wanted call", comp.LoopStep().Kind())
	}
	if comp.Result().Kind() != ast.IdentKind {
		t.Errorf("Result() got %v, wanted identifier", comp.Result().Kind())
	}
	expr.RenumberIDs(testIDGen(100))
	if expr.ID() != 101 {
		t.Errorf("Renumbering an unspecified expression mutated the value: %v", expr)
	}
	comp = expr.AsComprehension()
	if comp.IterRange().ID() != 102 {
		t.Errorf("IterRange() got %v, wanted list", comp.IterRange().ID())
	}
	if comp.AccuInit().ID() != 105 {
		t.Errorf("AccuInit() got %v, wanted literal", comp.AccuInit().ID())
	}
	if comp.LoopCondition().ID() != 106 {
		t.Errorf("LoopCondition() got %v, wanted literal", comp.LoopCondition().ID())
	}
	if comp.LoopStep().ID() != 107 {
		t.Errorf("LoopStep() got %v, wanted call", comp.LoopStep().ID())
	}
	if comp.Result().ID() != 112 {
		t.Errorf("Result() got %v, wanted identifier", comp.Result().ID())
	}
}

func TestComprehensionNil(t *testing.T) {
	expr := nilTestExpr(t)
	comp := expr.AsComprehension()
	if comp.IterRange().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("IterRange() got %v, wanted unspecified", comp.IterRange().Kind())
	}
	if comp.AccuInit().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("AccuInit() got %v, wanted unspecified", comp.AccuInit().Kind())
	}
	if comp.LoopCondition().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("LoopCondition() got %v, wanted unspecified", comp.LoopCondition().Kind())
	}
	if comp.LoopStep().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("LoopStep() got %v, wanted unspecified", comp.LoopStep().Kind())
	}
	if comp.Result().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("Result() got %v, wanted unspecified", comp.Result().Kind())
	}
	expr.RenumberIDs(testIDGen(100))
	if expr.ID() != 101 {
		t.Errorf("RenumberIDs() got %d, wanted 101", expr.ID())
	}
}

func TestIsBind(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "nil expr",
			expr: nil,
			want: false,
		},
		{
			name: "non-comprehension expr",
			expr: fac.NewLiteral(1, types.True),
			want: false,
		},
		{
			name: "valid bind comprehension",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "a"),
				fac.NewIdent(6, "a"),
			),
			want: true,
		},
		{
			name: "non-empty list iterRange",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{fac.NewLiteral(7, types.Int(1))}, []int32{}),
				"#unused",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "a"),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
		{
			name: "non-list iterRange",
			expr: fac.NewComprehension(1,
				fac.NewIdent(2, "items"),
				"#unused",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "a"),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
		{
			name: "empty accuVar",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, ""),
				fac.NewLiteral(6, types.Int(1)),
			),
			want: false,
		},
		{
			name: "accuVar equals unusedIterVar",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"#unused",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "#unused"),
				fac.NewIdent(6, "#unused"),
			),
			want: false,
		},
		{
			name: "user iterVar",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"x",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "a"),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
		{
			name: "two-var comprehension",
			expr: fac.NewComprehensionTwoVar(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"v",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "a"),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
		{
			name: "non-false loopCondition",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.True),
				fac.NewIdent(5, "a"),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
		{
			name: "non-ident loopStep",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewLiteral(5, types.Int(1)),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
		{
			name: "mismatched loopStep ident",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{}, []int32{}),
				"#unused",
				"a",
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.False),
				fac.NewIdent(5, "b"),
				fac.NewIdent(6, "a"),
			),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ast.IsBind(tc.expr); got != tc.want {
				t.Errorf("ast.IsBind() got %v, wanted %v", got, tc.want)
			}
		})
	}
}

func TestIdent(t *testing.T) {
	fac := ast.NewExprFactory()
	id := fac.NewIdent(1, "a")
	if id.AsIdent() != "a" {
		t.Errorf("id.AsIdent() got %s, wanted 'a'", id.AsIdent())
	}
	id.RenumberIDs(testIDGen(50))
	if id.ID() != 51 {
		t.Errorf("id.ID() got %d, wanted 51", id.ID())
	}
}

func TestIdentNil(t *testing.T) {
	expr := nilTestExpr(t)
	if expr.AsIdent() != "" {
		t.Errorf("AsIdent() got %s, wanted ''", expr.AsIdent())
	}
}

func TestList(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewList(20, []ast.Expr{fac.NewLiteral(21, types.OptionalOf(types.True))}, []int32{0})
	list := expr.AsList()
	if list.Size() != 1 {
		t.Errorf("list.Size() got %s, wanted 'a'", expr.AsIdent())
	}
	if list.Elements()[0].Kind() != ast.LiteralKind {
		t.Errorf("list.Elements()[0] got %v, wanted true", list.Elements()[0])
	}
	if list.OptionalIndices()[0] != 0 {
		t.Errorf("list.OptionalIndices()[0] got %d, wanted 0", list.OptionalIndices()[0])
	}
	expr.RenumberIDs(testIDGen(50))
	if expr.ID() != 51 {
		t.Errorf("expr.ID() got %d, wanted 51", expr.ID())
	}
	if list.Elements()[0].ID() != 52 {
		t.Errorf("list.Elements()[0].ID() got %d, wanted 52", list.Elements()[0].ID())
	}
}

func TestListNil(t *testing.T) {
	expr := nilTestExpr(t)
	list := expr.AsList()
	if list.Size() != 0 {
		t.Errorf("nil list.Size() got %d, wanted 0", list.Size())
	}
	if len(list.Elements()) != 0 {
		t.Errorf("nil list.Elements() got %d, wanted 0", list.Elements())
	}
	if len(list.OptionalIndices()) != 0 {
		t.Errorf("nil list.OptionalIndices() got %d, wanted 0", list.OptionalIndices())
	}
}

func TestLiteralNil(t *testing.T) {
	expr := nilTestExpr(t)
	if expr.AsLiteral() != nil {
		t.Errorf("AsLiteral() got %v, wanted nil", expr.AsLiteral())
	}
}

func TestMap(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewMap(1,
		[]ast.EntryExpr{
			fac.NewMapEntry(2, fac.NewIdent(3, "a"), fac.NewLiteral(4, types.True), true),
			fac.NewMapEntry(5, fac.NewIdent(6, "b"), fac.NewLiteral(7, types.False), false),
		})
	m := expr.AsMap()
	if m.Size() != 2 {
		t.Errorf("map.Size() got %d, wanted 2", m.Size())
	}
	expr.RenumberIDs(testIDGen(50))
	if expr.ID() != 51 {
		t.Errorf("expr.ID() got %d, wanted 51", expr.ID())
	}
	if m.Entries()[0].ID() != 52 {
		t.Errorf("m.Entries()[0].ID() got %d, wanted 52", m.Entries()[0].ID())
	}
	entry := m.Entries()[0].AsMapEntry()
	if key := entry.Key(); key.ID() != 53 {
		t.Errorf("key.ID() got %d, wanted 53", key.ID())
	}
	if val := entry.Value(); val.ID() != 54 {
		t.Errorf("val.ID() got %d, wanted 54", val.ID())
	}
	if !entry.IsOptional() {
		t.Error("entry.IsOptional() got false, wanted true")
	}
}

func TestMapNil(t *testing.T) {
	expr := nilTestExpr(t)
	m := expr.AsMap()
	if m.Size() != 0 {
		t.Errorf("nil map.Size() got %d, wanted 0", m.Size())
	}
	if len(m.Entries()) != 0 {
		t.Errorf("nil map.Entries() got %d, wanted 0", m.Entries())
	}
}

func TestMapEntryNil(t *testing.T) {
	fac := ast.NewExprFactory()
	field := fac.NewStructField(1,
		"hello", fac.NewLiteral(3, types.String("world")), false)

	// intentionally convert the struct field to the wrong type
	entry := field.AsMapEntry()
	if entry.Key().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("entry.Key() got %s, wanted ''", entry.Key())
	}
	if entry.Value().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("entry.Value() got %v, wanted unspecified", entry.Value())
	}
	if entry.IsOptional() {
		t.Errorf("entry.IsOptional() got %v, wanted ''", entry.IsOptional())
	}
}

func TestSelect(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewSelect(2, fac.NewIdent(1, "operand"), "field")
	sel := expr.AsSelect()
	if sel.FieldName() != "field" {
		t.Errorf("sel.FieldName() got %s, wanted 'field'", sel.FieldName())
	}
	if sel.Operand().AsIdent() != "operand" {
		t.Errorf("sel.Operand().AsIdent() got %s, wanted 'operand'", sel.Operand().AsIdent())
	}
	expr.RenumberIDs(testIDGen(20))
	if sel.Operand().ID() != 22 {
		t.Errorf("Operand().ID() got %d, wanted 22", sel.Operand().ID())
	}
}

func TestSelectNil(t *testing.T) {
	expr := nilTestExpr(t)
	sel := expr.AsSelect()
	if sel.FieldName() != "" {
		t.Errorf("sel.FieldName() got %s, wanted ''", sel.FieldName())
	}
	if sel.IsTestOnly() {
		t.Error("sel.IsTestOnly() got true, wanted false")
	}
	if sel.Operand().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("sel.Operand() got %v, wanted unspecified", sel.Operand())
	}
}

func TestStruct(t *testing.T) {
	fac := ast.NewExprFactory()
	expr := fac.NewStruct(1,
		"custom.StructType",
		[]ast.EntryExpr{
			fac.NewStructField(2, "a", fac.NewLiteral(3, types.True), true),
			fac.NewStructField(4, "b", fac.NewLiteral(5, types.False), false),
		})
	s := expr.AsStruct()
	if s.TypeName() != "custom.StructType" {
		t.Errorf("s.TypeName() got %q, wanted 'custom.StructType'", s.TypeName())
	}
	if len(s.Fields()) != 2 {
		t.Errorf("s.Fields() got %d, wanted 2", len(s.Fields()))
	}
	expr.RenumberIDs(testIDGen(50))
	if expr.ID() != 51 {
		t.Errorf("expr.ID() got %d, wanted 51", expr.ID())
	}
	if s.Fields()[0].ID() != 52 {
		t.Errorf("s.Fields()[0].ID() got %d, wanted 52", s.Fields()[0].ID())
	}
	field := s.Fields()[0].AsStructField()
	if val := field.Value(); val.ID() != 53 {
		t.Errorf("val.ID() got %d, wanted 53", val.ID())
	}
	if field.Name() != "a" {
		t.Errorf("field.Name() got %s, wanted 'a'", field.Name())
	}
	if !field.IsOptional() {
		t.Error("field.IsOptional() got false, wanted true")
	}
}

func TestStructNil(t *testing.T) {
	expr := nilTestExpr(t)
	s := expr.AsStruct()
	if s.TypeName() != "" {
		t.Errorf("nil struct.TypeName() got %s, wanted ''", s.TypeName())
	}
	if len(s.Fields()) != 0 {
		t.Errorf("nil struct.Fields() got %d, wanted 0", s.Fields())
	}
}

func TestStructFieldNil(t *testing.T) {
	fac := ast.NewExprFactory()
	entry := fac.NewMapEntry(1,
		fac.NewLiteral(2, types.String("hello")),
		fac.NewLiteral(3, types.String("world")),
		false)

	// intentionally convert the map entry to the wrong type
	field := entry.AsStructField()
	if field.Name() != "" {
		t.Errorf("field.FieldName() got %s, wanted ''", field.Name())
	}
	if field.Value().Kind() != ast.UnspecifiedExprKind {
		t.Errorf("field.Value() got %v, wanted unspecified", field.Value())
	}
	if field.IsOptional() {
		t.Errorf("field.IsOptional() got %v, wanted ''", field.IsOptional())
	}
}

func TestRenumberIDs(t *testing.T) {
	fac := ast.NewExprFactory()
	e := fac.NewUnspecifiedExpr(10)
	e.RenumberIDs(testIDGen(100))
	if e.ID() != 101 {
		t.Errorf("RenumberIDs() got %d, wanted 101", e.ID())
	}
	e = fac.NewLiteral(20, types.True)
	e.RenumberIDs(testIDGen(200))
	if e.ID() != 201 {
		t.Errorf("RenumberIDs() got %d, wanted 201", e.ID())
	}
}

func nilTestExpr(t testing.TB) ast.Expr {
	t.Helper()
	fac := ast.NewExprFactory()
	expr := fac.NewLiteral(1, types.NullValue)
	expr.SetKindCase(nil)
	if expr.Kind() != ast.UnspecifiedExprKind {
		t.Fatalf("SetKindCase(nil) did not produce an unspecified expr kind: %v", expr.Kind())
	}
	return expr
}

func testIDGen(seed int64) ast.IDGenerator {
	seen := map[int64]int64{}
	return func(originalID int64) int64 {
		if id, found := seen[originalID]; found {
			return id
		}
		seed++
		seen[originalID] = seed
		return seed
	}
}
