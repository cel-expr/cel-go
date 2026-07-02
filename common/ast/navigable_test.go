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
	"reflect"
	"testing"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func TestNavigateAST(t *testing.T) {
	tests := []struct {
		expr            string
		descendantCount int
		callCount       int
		maxDepth        int
		maxID           int64
	}{
		{
			expr:            `'a' == 'b'`,
			descendantCount: 3,
			callCount:       1,
			maxDepth:        1,
			maxID:           4,
		},
		{
			expr:            `'a'.size()`,
			descendantCount: 2,
			callCount:       1,
			maxDepth:        1,
			maxID:           3,
		},
		{
			expr:            `[1, 2, 3]`,
			descendantCount: 4,
			callCount:       0,
			maxDepth:        1,
			maxID:           5,
		},
		{
			expr:            `[1, 2, 3][0]`,
			descendantCount: 6,
			callCount:       1,
			maxDepth:        2,
			maxID:           7,
		},
		{
			expr:            `{1u: 'hello'}`,
			descendantCount: 3,
			callCount:       0,
			maxDepth:        1,
			maxID:           5,
		},
		{
			expr:            `{'hello': 'world'}.hello`,
			descendantCount: 4,
			callCount:       0,
			maxDepth:        2,
			maxID:           6,
		},
		{
			expr:            `type(1) == int`,
			descendantCount: 4,
			callCount:       2,
			maxDepth:        2,
			maxID:           5,
		},
		{
			expr:            `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			descendantCount: 2,
			callCount:       0,
			maxDepth:        1,
			maxID:           4,
		},
		{
			expr:            `[true].exists(i, i)`,
			descendantCount: 11, // 2 for iter range, 1 for accu init, 4 for loop condition, 3 for loop step, 1 for result
			callCount:       3,  // @not_strictly_false(!result), accu_init || i
			maxDepth:        3,
			maxID:           14,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked := mustTypeCheck(t, tc.expr)
			nav := ast.NavigateAST(checked)
			descendants := ast.MatchDescendants(nav, ast.AllMatcher())
			if len(descendants) != tc.descendantCount {
				t.Errorf("ast.MatchDescendants(%v) got %d descendants, wanted %d", checked.Expr(), len(descendants), tc.descendantCount)
			}
			maxDepth := 0
			for _, d := range descendants {
				if d.Depth() > maxDepth {
					maxDepth = d.Depth()
				}
			}
			if maxDepth != tc.maxDepth {
				t.Errorf("got max NavigableExpr.Depth() of %d, wanted %d", maxDepth, tc.maxDepth)
			}
			maxID := ast.MaxID(checked)
			if maxID != tc.maxID {
				t.Errorf("got max id %d, wanted %d", maxID, tc.maxID)
			}
			calls := ast.MatchSubset(descendants, ast.KindMatcher(ast.CallKind))
			if len(calls) != tc.callCount {
				t.Errorf("ast.MatchSubset(%v) got %d calls, wanted %d", checked.Expr(), len(calls), tc.callCount)
			}
		})
	}
}

func TestExprVisitor(t *testing.T) {
	tests := []struct {
		expr         string
		preOrderIDs  []int64
		postOrderIDs []int64
	}{
		{
			// [2] ==, [1] 'a', [3] 'b'
			expr:         `'a' == 'b'`,
			preOrderIDs:  []int64{2, 1, 3},
			postOrderIDs: []int64{1, 3, 2},
		},
		{
			// [2] size(), [1] 'a'
			expr:         `'a'.size()`,
			preOrderIDs:  []int64{2, 1},
			postOrderIDs: []int64{1, 2},
		},
		{
			// [3] ==, [1] type(), [2] 1, [4] int
			expr:         `type(1) == int`,
			preOrderIDs:  []int64{3, 1, 2, 4},
			postOrderIDs: []int64{2, 1, 4, 3},
		},
		{
			// [5] .hello, [1] {}, [3] 'hello', [4] 'world'
			expr:         `{'hello': 'world'}.hello`,
			preOrderIDs:  []int64{5, 1, 3, 4},
			postOrderIDs: []int64{3, 4, 1, 5},
		},
		{
			// [1] TestAllTypes, [3] 1
			expr:         `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			preOrderIDs:  []int64{1, 3},
			postOrderIDs: []int64{3, 1},
		},
		{
			// [13] comprehension
			// range:    [1] [], [2] true
			// accuInit: [6] result=false
			// loopCond: [9] @not_strictly_false, [8] !, [7] result
			// loopStep: [11] ||, [10] result [5] i
			// result:   [12] result
			expr:         `[true].exists(i, i)`,
			preOrderIDs:  []int64{13, 1, 2, 6, 9, 8, 7, 11, 10, 5, 12},
			postOrderIDs: []int64{2, 1, 6, 7, 8, 9, 10, 5, 11, 12, 13},
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked := mustTypeCheck(t, tc.expr)
			root := ast.NavigateAST(checked)
			// Verify pre order visit behavior
			preOrderExprIDs := []int64{}
			navVisitor := ast.NewExprVisitor(func(e ast.Expr) {
				nav := e.(ast.NavigableExpr)
				preOrderExprIDs = append(preOrderExprIDs, nav.ID())
			})
			ast.PreOrderVisit(root, navVisitor)
			if !reflect.DeepEqual(tc.preOrderIDs, preOrderExprIDs) {
				t.Errorf("PreOrderVisit() got %v expressions, wanted %v", tc.preOrderIDs, preOrderExprIDs)
			}

			// Demonstrate preOrder visit behavior with Children()
			preOrderExprIDs = []int64{}
			visited := []ast.NavigableExpr{root}
			for len(visited) > 0 {
				e := visited[0]
				preOrderExprIDs = append(preOrderExprIDs, e.ID())
				visited = append(e.Children()[:], visited[1:]...)
			}
			if !reflect.DeepEqual(tc.preOrderIDs, preOrderExprIDs) {
				t.Errorf("PreOrderVisit() got %v expressions, wanted %v", tc.preOrderIDs, preOrderExprIDs)
			}

			// Verify post order visit behavior.
			postOrderExprIDs := []int64{}
			navVisitor = ast.NewExprVisitor(func(e ast.Expr) {
				nav := e.(ast.NavigableExpr)
				postOrderExprIDs = append(postOrderExprIDs, nav.ID())
			})
			ast.PostOrderVisit(root, navVisitor)
			if !reflect.DeepEqual(tc.postOrderIDs, postOrderExprIDs) {
				t.Errorf("PostOrderVisit() got %v expressions, wanted %v", tc.postOrderIDs, postOrderExprIDs)
			}
		})
	}
}

func TestExceedsDepth(t *testing.T) {
	// Depths below reflect NavigableExpr.Depth() of each expression's deepest node, matching the
	// maxDepth column of TestNavigateAST. ExceedsDepth reports true once a node reaches maxDepth,
	// so an expression of depth D is flagged for every maxDepth <= D.
	tests := []struct {
		name     string
		expr     string
		maxDepth int
		want     bool
	}{
		{name: "shallow below limit", expr: `'a' == 'b'`, maxDepth: 2, want: false},           // depth 1
		{name: "shallow at limit", expr: `'a' == 'b'`, maxDepth: 1, want: true},               // depth 1
		{name: "medium below limit", expr: `[1, 2, 3][0]`, maxDepth: 3, want: false},          // depth 2
		{name: "medium at limit", expr: `[1, 2, 3][0]`, maxDepth: 2, want: true},              // depth 2
		{name: "medium under limit", expr: `[1, 2, 3][0]`, maxDepth: 1, want: true},           // depth 2
		{name: "deep under default", expr: `[true].exists(i, i)`, maxDepth: 250, want: false}, // depth 3
		{name: "deep at limit", expr: `[true].exists(i, i)`, maxDepth: 3, want: true},         // depth 3
		{name: "deep one above limit", expr: `[true].exists(i, i)`, maxDepth: 4, want: false}, // depth 3
		{name: "disabled with zero", expr: `[true].exists(i, i)`, maxDepth: 0, want: false},
		{name: "disabled with negative", expr: `[true].exists(i, i)`, maxDepth: -1, want: false},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			checked := mustTypeCheck(t, tc.expr)
			if got := ast.ExceedsDepth(checked, tc.maxDepth); got != tc.want {
				t.Errorf("ExceedsDepth(%q, %d) = %v, wanted %v", tc.expr, tc.maxDepth, got, tc.want)
			}
		})
	}
}

func TestExceedsDepthNilSafety(t *testing.T) {
	if ast.ExceedsDepth(nil, 250) {
		t.Error("ExceedsDepth(nil, 250) = true, wanted false")
	}
	if ast.ExceedsDepth(ast.NewAST(nil, nil), 250) {
		t.Error("ExceedsDepth(emptyAST, 250) = true, wanted false")
	}
}

func TestExceedsDepthBoundedTraversal(t *testing.T) {
	// Build a synthetic AST nested far deeper than the parser's recursion limit would ever produce.
	// This is exactly the shape ExceedsDepth guards against, since such an AST can only enter through
	// proto loading rather than the parser. Stacking depth unary '!' calls on a literal places the
	// deepest node (the literal) at NavigableExpr.Depth() == depth.
	const depth = 300
	fac := ast.NewExprFactory()
	expr := fac.NewLiteral(1, types.Bool(true))
	for i := 0; i < depth; i++ {
		expr = fac.NewCall(int64(i+2), operators.LogicalNot, expr)
	}
	deep := ast.NewAST(expr, ast.NewSourceInfo(nil))

	tests := []struct {
		name     string
		maxDepth int
		want     bool
	}{
		{name: "below the deepest node", maxDepth: 250, want: true},
		// The traversal is bounded to maxDepth+1 levels, so it must still descend far enough to
		// observe a node sitting exactly at maxDepth.
		{name: "equal to the deepest node", maxDepth: depth, want: true},
		{name: "one past the deepest node", maxDepth: depth + 1, want: false},
		{name: "check disabled", maxDepth: 0, want: false},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			if got := ast.ExceedsDepth(deep, tc.maxDepth); got != tc.want {
				t.Errorf("ExceedsDepth(deepAST(%d), %d) = %v, wanted %v", depth, tc.maxDepth, got, tc.want)
			}
		})
	}
}

func TestNavigableASTNilSafety(t *testing.T) {
	tests := []struct {
		name string
		e    ast.NavigableExpr
	}{
		{
			name: "nil expr",
			e:    ast.NavigateAST(ast.NewAST(nil, nil)),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			e := tc.e
			if e.ID() != 0 {
				t.Errorf("ID() got %d, wanted 0", e.ID())
			}
			if e.Kind() != ast.UnspecifiedExprKind {
				t.Errorf("Kind() got %v, wanted unspecified kind", e.Kind())
			}
			if e.Type() != types.DynType {
				t.Errorf("Type() got %v, wanted types.DynType", e.Type())
			}
			if p, found := e.Parent(); found {
				t.Errorf("Parent() got %v, wanted not found", p)
			}
			if len(e.Children()) != 0 {
				t.Errorf("Children() got %v, wanted none", e.Children())
			}
			if e.AsLiteral() != nil {
				t.Errorf("AsLiteral() got %v, wanted nil", e.AsLiteral())
			}
			if e.AsCall() == nil {
				t.Errorf("AsCall() got nil, wanted non-nil for safe traversal")
			}
			if e.AsComprehension() == nil {
				t.Errorf("AsComprehension() got nil, wanted non-nil for safe traversal")
			}
		})
	}
}

func TestNavigableExpr(t *testing.T) {
	checkedAST := mustTypeCheck(t, `'a' == 'b'`)
	navAST := ast.NavigateAST(checkedAST)
	literals := ast.MatchDescendants(navAST, func(expr ast.NavigableExpr) bool {
		return expr.Kind() == ast.LiteralKind &&
			expr.AsLiteral().Equal(types.String("a")) == types.True
	})
	if len(literals) != 1 {
		t.Fatalf("MatchDescendents('a') got %d results, wanted 1", len(literals))
	}
	litA := literals[0]
	if litA.Depth() != 1 {
		t.Fatalf("litA.Depth() got %d, wanted 1", litA.Depth())
	}
	parent, found := litA.Parent()
	if !found {
		t.Fatal("litA.Parent() returned nil")
	}
	if parent.Kind() != ast.CallKind && parent.AsCall().FunctionName() != "==" {
		t.Fatalf("litA.Parent() got %v, watned '==' function call", parent)
	}
	litAPrime := ast.NavigateExpr(checkedAST, litA)
	if litAPrime.Depth() != litA.Depth() {
		t.Errorf("litAPrime.Depth() != litA.Depth(), got %d, wanted %d", litAPrime.Depth(), litA.Depth())
	}
}

func TestNavigableCallExprMember(t *testing.T) {
	checked := mustTypeCheck(t, `'hello'.size()`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.CallKind {
		t.Errorf("Kind() got %v, wanted CallKind", expr.Kind())
	}
	call := expr.AsCall()
	if call.FunctionName() != "size" {
		t.Errorf("FunctionName() got %s, wanted size", call.FunctionName())
	}
	if call.Target() == nil {
		t.Fatalf("Target() got nil, wanted non-nil")
	}
	if len(call.Args()) != 0 {
		t.Errorf("Args() got %v, wanted 0", call.Args())
	}
	target := call.Target()
	if target.Kind() != ast.LiteralKind {
		t.Errorf("Kind() got %v, wanted literal", target.Kind())
	}
	if target.AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("AsLiteral() got %v, wanted 'hello'", target.AsLiteral())
	}
	if p, found := target.(ast.NavigableExpr).Parent(); !found || p != expr {
		t.Errorf("Parent() got %v, wanted %v", p, expr)
	}
	sizeFn := ast.MatchDescendants(expr, ast.FunctionMatcher("size"))
	if len(sizeFn) != 1 {
		t.Errorf("ast.MatchDescendants() size function returned %v, wanted 1", sizeFn)
	}
	constantValues := ast.MatchDescendants(expr, ast.ConstantValueMatcher())
	if len(constantValues) != 1 {
		t.Fatalf("ast.MatchDescendants() constant values returned %v, wanted 1 value", constantValues)
	}
	if constantValues[0].AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("constantValues[0] got %v, wanted 'hello'", constantValues[0])
	}
}

func TestNavigableCallExprGlobal(t *testing.T) {
	checked := mustTypeCheck(t, `size('hello')`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.CallKind {
		t.Errorf("Kind() got %v, wanted CallKind", expr.Kind())
	}
	call := expr.AsCall()
	if call.FunctionName() != "size" {
		t.Errorf("FunctionName() got %s, wanted size", call.FunctionName())
	}
	if call.IsMemberFunction() {
		t.Fatal("IsMemberFunction() returned true, wanted false")
	}
	if len(call.Args()) != 1 {
		t.Errorf("Args() got %v, wanted 1", call.Args())
	}
	arg := call.Args()[0]
	if arg.Kind() != ast.LiteralKind {
		t.Errorf("Kind() got %v, wanted literal", arg.Kind())
	}
	if arg.AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("AsLiteral() got %v, wanted 'hello'", arg.AsLiteral())
	}
	if p, found := arg.(ast.NavigableExpr).Parent(); !found || p != expr {
		t.Errorf("Parent() got %v, wanted %v", p, expr)
	}
	sizeFn := ast.MatchDescendants(expr, ast.FunctionMatcher("size"))
	if len(sizeFn) != 1 {
		t.Errorf("ast.MatchDescendants() size function returned %v, wanted 1", sizeFn)
	}
	constantValues := ast.MatchDescendants(expr, ast.ConstantValueMatcher())
	if len(constantValues) != 1 {
		t.Fatalf("ast.MatchDescendants() constant values returned %v, wanted 1 value", constantValues)
	}
	if constantValues[0].AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("constantValues[0] got %v, wanted 'hello'", constantValues[0])
	}
}

func TestNavigableListExpr(t *testing.T) {
	checked := mustTypeCheck(t, `[[1], [2]]`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.ListKind {
		t.Errorf("Kind() got %v, wanted ListKind", expr.Kind())
	}
	list := expr.AsList()
	if list.Size() != 2 {
		t.Errorf("Size() got %d, wanted 2", list.Size())
	}
	if len(list.OptionalIndices()) != 0 {
		t.Errorf("OptionalIndicies() returned %v, wanted none", list.OptionalIndices())
	}
	if len(list.Elements()) != 2 {
		t.Errorf("Elements() returned %v, wanted 2", list.Elements())
	}
	constantValues := ast.MatchDescendants(expr, ast.ConstantValueMatcher())
	if len(constantValues) != 5 {
		t.Errorf("ast.MatchDescendants() constant values returned %v, wanted 5", constantValues)
	}
	constantLists := ast.MatchSubset(constantValues, ast.KindMatcher(ast.ListKind))
	if len(constantLists) != 3 {
		t.Errorf("ast.MatchSubset() constant lists returned %v, wanted 3", constantLists)
	}
	literals := ast.MatchSubset(constantValues, ast.KindMatcher(ast.LiteralKind))
	if len(literals) != 2 {
		t.Errorf("ast.MatchSubset() literals returned %v, wanted 2", literals)
	}
}

func TestNavigableMapExpr(t *testing.T) {
	checked := mustTypeCheck(t, `{'hello': 1}`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.MapKind {
		t.Errorf("Kind() got %v, wanted MapKind", expr.Kind())
	}
	m := expr.AsMap()
	if m.Size() != 1 {
		t.Errorf("Size() got %d, wanted 1", m.Size())
	}
	if len(m.Entries()) != 1 {
		t.Errorf("Entries() returned %v, wanted 1", m.Entries())
	}
	e := m.Entries()[0]
	entry := e.AsMapEntry()
	if entry.IsOptional() {
		t.Error("IsOptional() returned true, wanted false")
	}
	if entry.Key().AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("Key() returned %v, wanted 'hello'", entry.Key().AsLiteral())
	}
	if entry.Value().AsLiteral().Equal(types.Int(1)) != types.True {
		t.Errorf("Value() returned %v, wanted 1", entry.Value().AsLiteral())
	}
	descendants := ast.MatchDescendants(expr, ast.AllMatcher())
	if len(descendants) != 3 {
		t.Errorf("ast.MatchDescendants() returned %v, wanted 3", descendants)
	}
	literals := ast.MatchSubset(descendants, ast.KindMatcher(ast.LiteralKind))
	if len(literals) != 2 {
		t.Errorf("ast.MatchSubset() literals returned %v, wanted 2", literals)
	}
}

func TestNavigableStructExpr(t *testing.T) {
	checked := mustTypeCheck(t, `google.expr.proto3.test.TestAllTypes{single_int32: 1}`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.StructKind {
		t.Errorf("Kind() got %v, wanted StructKind", expr.Kind())
	}
	s := expr.AsStruct()
	if s.TypeName() != "google.expr.proto3.test.TestAllTypes" {
		t.Errorf("TypeName() got %s, wanted TestAllTypes", s.TypeName())
	}
	if len(s.Fields()) != 1 {
		t.Errorf("Fields() returned %v, wanted 1", s.Fields())
	}
	f := s.Fields()[0]
	field := f.AsStructField()
	if field.IsOptional() {
		t.Error("IsOptional() returned true, wanted false")
	}
	if field.Name() != "single_int32" {
		t.Errorf("Name() returned %s, wanted 'single_int32'", field.Name())
	}
	if field.Value().AsLiteral().Equal(types.Int(1)) != types.True {
		t.Errorf("Value() returned %v, wanted 1", field.Value().AsLiteral())
	}
	descendants := ast.MatchDescendants(expr, ast.AllMatcher())
	if len(descendants) != 2 {
		t.Errorf("ast.MatchDescendants() returned %v, wanted 2", descendants)
	}
	literals := ast.MatchSubset(descendants, ast.KindMatcher(ast.LiteralKind))
	if len(literals) != 1 {
		t.Errorf("ast.MatchSubset() literals returned %v, wanted 1", literals)
	}
	if literals[0].AsLiteral().Equal(types.Int(1)) != types.True {
		t.Errorf("Value().AsLiteral() got %v, wanted 1", literals[0].AsLiteral())
	}
}

func TestNavigableComprehensionExpr(t *testing.T) {
	checked := mustTypeCheck(t, `[true].exists(i, i)`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.ComprehensionKind {
		t.Errorf("Kind() got %v, wanted ComprehensionKind", expr.Kind())
	}
	comp := expr.AsComprehension()
	iterRange := comp.IterRange()
	if len(ast.MatchSubset([]ast.NavigableExpr{iterRange.(ast.NavigableExpr)}, ast.ConstantValueMatcher())) != 1 {
		t.Errorf("IterRange() returned a non-constant list")
	}
	if comp.IterVar() != "i" {
		t.Errorf("IterVar() got %s, wanted 'i'", comp.IterVar())
	}
	if comp.HasIterVar2() {
		t.Error("HasIterVar2() returned true, wanted false")
	}
	if comp.IterVar2() != "" {
		t.Errorf("IterVar2() returned %s, wanted empty string", comp.IterVar2())
	}
	if comp.AccuVar() != "@result" {
		t.Errorf("AccuVar() got %s, wanted '@result'", comp.AccuVar())
	}
	if comp.AccuInit().AsLiteral() != types.False {
		t.Errorf("AccuInit() returned %v, wanted false", comp.AccuInit().AsLiteral())
	}
	if comp.Result().Kind() != ast.IdentKind {
		t.Errorf("Result() returned %v, wanted ident", comp.Result())
	}
	if comp.LoopCondition().Kind() != ast.CallKind {
		t.Errorf("LoopCondition() returned %v, wanted call", comp.LoopCondition())
	}
	if comp.LoopStep().Kind() != ast.CallKind {
		t.Errorf("LoopStep() returned %v, wanted call", comp.LoopStep())
	}
	if comp.Result().AsIdent() != "@result" {
		t.Errorf("AsIdent() returned %v, wanted @result", comp.Result().AsIdent())
	}
}

func TestNavigableSelectExpr(t *testing.T) {
	checked := mustTypeCheck(t, `msg.single_int32`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.SelectKind {
		t.Errorf("Kind() got %v, wanted SelectKind", expr.Kind())
	}
	sel := expr.AsSelect()
	if sel.FieldName() != "single_int32" {
		t.Errorf("FieldName() got %s, wanted single_int32", sel.FieldName())
	}
	if sel.Operand().Kind() != ast.IdentKind {
		t.Fatalf("Operand() kind got %v, wanted ident", sel.Operand().Kind())
	}
	if sel.Operand().AsIdent() != "msg" {
		t.Errorf("Operand() got %v, wanted ident 'msg'", sel.Operand())
	}
	if sel.IsTestOnly() {
		t.Error("IsTestOnly() got true, wanted false")
	}
}

func TestNavigableSelectExpr_TestOnly(t *testing.T) {
	checked := mustTypeCheck(t, `has(msg.single_int32)`)
	expr := ast.NavigateAST(checked)
	if expr.Kind() != ast.SelectKind {
		t.Errorf("Kind() got %v, wanted SelectKind", expr.Kind())
	}
	sel := expr.AsSelect()
	if sel.FieldName() != "single_int32" {
		t.Errorf("FieldName() got %s, wanted single_int32", sel.FieldName())
	}
	if sel.Operand().Kind() != ast.IdentKind {
		t.Fatalf("Operand() kind got %v, wanted ident", sel.Operand().Kind())
	}
	if sel.Operand().AsIdent() != "msg" {
		t.Errorf("Operand() got %v, wanted ident 'msg'", sel.Operand())
	}
	if !sel.IsTestOnly() {
		t.Error("IsTestOnly() got false, wanted true")
	}
}

func mustTypeCheck(t testing.TB, expr string, opts ...any) *ast.AST {
	t.Helper()
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
		parser.EnableOptionalSyntax(true),
		parser.PopulateMacroCalls(true))
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	exprSrc := common.NewTextSource(expr)
	parsed, iss := p.Parse(exprSrc)
	if len(iss.GetErrors()) != 0 {
		t.Fatalf("Parse(%s) failed: %s", expr, iss.ToDisplayString())
	}
	regOpts := []types.RegistryOption{}
	chkOpts := []checker.Option{}
	for _, opt := range opts {
		switch v := opt.(type) {
		case types.RegistryOption:
			regOpts = append(regOpts, v)
		case checker.Option:
			chkOpts = append(chkOpts, v)
		default:
			t.Fatalf("mustTypeCheck() failed with invalid option type: %T", v)
		}
	}
	regOpts = append(regOpts, types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	reg := newTestRegistry(t, regOpts...)
	env := newTestEnv(t, containers.DefaultContainer, reg, chkOpts...)
	checked, iss := checker.Check(parsed, exprSrc, env)
	if len(iss.GetErrors()) != 0 {
		t.Fatalf("Check(%s) failed: %s", expr, iss.ToDisplayString())
	}
	return checked
}

func newTestRegistry(t testing.TB, opts ...types.RegistryOption) *types.Registry {
	t.Helper()
	reg, err := types.NewProtoRegistry(opts...)
	if err != nil {
		t.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	return reg
}

func newTestEnv(t testing.TB, cont *containers.Container, reg *types.Registry, opts ...checker.Option) *checker.Env {
	t.Helper()
	chkOpts := []checker.Option{checker.CrossTypeNumericComparisons(true)}
	chkOpts = append(chkOpts, opts...)
	env, err := checker.NewEnv(cont, reg, chkOpts...)
	if err != nil {
		t.Fatalf("checker.NewEnv(%v, %v) failed: %v", cont, reg, err)
	}
	err = env.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Fatalf("env.Add(stdlib.Functions()...) failed: %v", err)
	}
	env.AddIdents(decls.NewVariable("msg", types.NewObjectType("google.expr.proto3.test.TestAllTypes")))
	return env
}
