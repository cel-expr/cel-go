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
	"maps"
	"reflect"
	"testing"

	"cel.dev/cel-go/checker"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

func TestASTCopy(t *testing.T) {
	tests := []string{
		`'a' == 'b'`,
		`'a'.size()`,
		`size('a')`,
		`has({'a': 1}.a)`,
		`{'a': 1}`,
		`{'a': 1}['a']`,
		`[1, 2, 3].exists(i, i % 2 == 1)`,
		`google.expr.proto3.test.TestAllTypes{}`,
		`google.expr.proto3.test.TestAllTypes{repeated_int32: [1, 2]}`,
	}

	for _, tst := range tests {
		checked := mustTypeCheck(t, tst)
		copyChecked := ast.Copy(checked)
		if !reflect.DeepEqual(copyChecked.Expr(), checked.Expr()) {
			t.Errorf("Copy() got expr %v, wanted %v", copyChecked.Expr(), checked.Expr())
		}
		if !reflect.DeepEqual(copyChecked.SourceInfo(), checked.SourceInfo()) {
			t.Errorf("Copy() got source info %v, wanted %v", copyChecked.SourceInfo(), checked.SourceInfo())
		}
		copyParsed := ast.Copy(ast.NewAST(checked.Expr(), checked.SourceInfo()))
		if !reflect.DeepEqual(copyParsed.Expr(), checked.Expr()) {
			t.Errorf("Copy() got expr %v, wanted %v", copyParsed.Expr(), checked.Expr())
		}
		if !reflect.DeepEqual(copyParsed.SourceInfo(), checked.SourceInfo()) {
			t.Errorf("Copy() got source info %v, wanted %v", copyParsed.SourceInfo(), checked.SourceInfo())
		}
		checkedPB, err := ast.ToProto(checked)
		if err != nil {
			t.Errorf("ast.ToProto() failed: %v", err)
		}
		copyCheckedPB, err := ast.ToProto(copyChecked)
		if err != nil {
			t.Errorf("ast.ToProto() failed: %v", err)
		}
		if !proto.Equal(checkedPB, copyCheckedPB) {
			t.Errorf("Copy() produced different proto results, got %v, wanted %v",
				prototext.Format(checkedPB), prototext.Format(copyCheckedPB))
		}
		checkedRoundtrip, err := ast.ToAST(checkedPB)
		if err != nil {
			t.Errorf("ast.ToAST() failed: %v", err)
		}
		same := reflect.DeepEqual(checked.Expr(), checkedRoundtrip.Expr()) &&
			reflect.DeepEqual(checked.ReferenceMap(), checkedRoundtrip.ReferenceMap()) &&
			reflect.DeepEqual(checked.TypeMap(), checkedRoundtrip.TypeMap()) &&
			reflect.DeepEqual(checked.SourceInfo().MacroCalls(), checkedRoundtrip.SourceInfo().MacroCalls())
		if !same {
			t.Errorf("Roundtrip got %v, wanted %v", checkedRoundtrip, checked)
		}
	}
}

func TestASTJsonNames(t *testing.T) {
	tests := []string{
		`google.expr.proto3.test.TestAllTypes{}`,
		`google.expr.proto3.test.TestAllTypes{repeatedInt32: [1, 2]}`,
		`google.expr.proto3.test.TestAllTypes{singleInt32: 2}.singleInt32 == 2`,
	}

	for _, tst := range tests {
		checked := mustTypeCheck(t, tst, checker.JSONFieldNames(true), types.JSONFieldNames(true))
		copyChecked := ast.Copy(checked)
		if !reflect.DeepEqual(copyChecked.Expr(), checked.Expr()) {
			t.Errorf("Copy() got expr %v, wanted %v", copyChecked.Expr(), checked.Expr())
		}
		if !reflect.DeepEqual(copyChecked.SourceInfo(), checked.SourceInfo()) {
			t.Errorf("Copy() got source info %v, wanted %v", copyChecked.SourceInfo(), checked.SourceInfo())
		}
		copyParsed := ast.Copy(ast.NewAST(checked.Expr(), checked.SourceInfo()))
		if !reflect.DeepEqual(copyParsed.Expr(), checked.Expr()) {
			t.Errorf("Copy() got expr %v, wanted %v", copyParsed.Expr(), checked.Expr())
		}
		if !reflect.DeepEqual(copyParsed.SourceInfo(), checked.SourceInfo()) {
			t.Errorf("Copy() got source info %v, wanted %v", copyParsed.SourceInfo(), checked.SourceInfo())
		}
		checkedPB, err := ast.ToProto(checked)
		if err != nil {
			t.Errorf("ast.ToProto() failed: %v", err)
		}
		copyCheckedPB, err := ast.ToProto(copyChecked)
		if err != nil {
			t.Errorf("ast.ToProto() failed: %v", err)
		}
		if !proto.Equal(checkedPB, copyCheckedPB) {
			t.Errorf("Copy() produced different proto results, got %v, wanted %v",
				prototext.Format(checkedPB), prototext.Format(copyCheckedPB))
		}
		checkedRoundtrip, err := ast.ToAST(checkedPB)
		if err != nil {
			t.Errorf("ast.ToAST() failed: %v", err)
		}
		same := reflect.DeepEqual(checked.Expr(), checkedRoundtrip.Expr()) &&
			reflect.DeepEqual(checked.ReferenceMap(), checkedRoundtrip.ReferenceMap()) &&
			reflect.DeepEqual(checked.TypeMap(), checkedRoundtrip.TypeMap()) &&
			reflect.DeepEqual(checked.SourceInfo().MacroCalls(), checkedRoundtrip.SourceInfo().MacroCalls())
		if !same {
			t.Errorf("Roundtrip got %v, wanted %v", checkedRoundtrip, checked)
		}
	}
}

func TestASTNilSafety(t *testing.T) {
	ex, err := ast.ProtoToExpr(nil)
	if err != nil {
		t.Fatalf("ast.ProtoToExpr() failed: %v", err)
	}
	info, err := ast.ProtoToSourceInfo(nil)
	if err != nil {
		t.Fatalf("ast.ProtoToSourceInfo() failed: %v", err)
	}
	tests := []*ast.AST{
		nil,
		ast.NewAST(nil, nil),
		ast.NewCheckedAST(nil, nil, nil),
		ast.NewCheckedAST(ast.NewAST(nil, nil), nil, nil),
		ast.NewAST(ex, info),
		ast.NewCheckedAST(ast.NewAST(ex, info), map[int64]*types.Type{}, map[int64]*ast.ReferenceInfo{}),
	}
	for _, tst := range tests {
		a := tst
		asts := []*ast.AST{a, ast.Copy(a)}
		for _, testAST := range asts {
			if testAST.Expr().ID() != 0 {
				t.Errorf("Expr().ID() got %v, wanted 0", testAST.Expr().ID())
			}
			if testAST.SourceInfo().SyntaxVersion() != "" {
				t.Errorf("SourceInfo().SyntaxVersion() got %s, wanted empty string", testAST.SourceInfo().SyntaxVersion())
			}
			if testAST.IsChecked() {
				t.Error("IsChecked() returned true, wanted false")
			}
			if testAST.GetType(testAST.Expr().ID()) != types.DynType {
				t.Errorf("GetType() got %v, wanted dyn", testAST.GetType(testAST.Expr().ID()))
			}
			if len(testAST.GetOverloadIDs(testAST.Expr().ID())) != 0 {
				t.Errorf("GetOverloadIDs() got %v, wanted empty set", testAST.GetOverloadIDs(testAST.Expr().ID()))
			}
		}
	}
}

func TestSourceInfo(t *testing.T) {
	src := common.NewStringSource("a\n? b\n: c", "custom description")
	info := ast.NewSourceInfo(src)
	if info.Description() != "custom description" {
		t.Errorf("Description() got %s, wanted 'custom description'", info.Description())
	}
	if len(info.LineOffsets()) != 3 {
		t.Errorf("LineOffsets() got %v, wanted 3 offsets", info.LineOffsets())
	}
	info.SetOffsetRange(1, ast.OffsetRange{Start: 0, Stop: 1}) // a
	info.SetOffsetRange(2, ast.OffsetRange{Start: 4, Stop: 5}) // b
	info.SetOffsetRange(3, ast.OffsetRange{Start: 8, Stop: 9}) // c
	if !reflect.DeepEqual(info.GetStartLocation(1), common.NewLocation(1, 0)) {
		t.Errorf("info.GetStartLocation(1) got %v, wanted line 1, col 0", info.GetStartLocation(1))
	}
	if !reflect.DeepEqual(info.GetStopLocation(1), common.NewLocation(1, 1)) {
		t.Errorf("info.GetStopLocation(1) got %v, wanted line 1, col 1", info.GetStopLocation(1))
	}
	if !reflect.DeepEqual(info.GetStartLocation(2), common.NewLocation(2, 2)) {
		t.Errorf("info.GetStartLocation(2) got %v, wanted line 2, col 2", info.GetStartLocation(2))
	}
	if !reflect.DeepEqual(info.GetStopLocation(2), common.NewLocation(2, 3)) {
		t.Errorf("info.GetStopLocation(2) got %v, wanted line 2, col 3", info.GetStopLocation(2))
	}
	if !reflect.DeepEqual(info.GetStartLocation(3), common.NewLocation(3, 2)) {
		t.Errorf("info.GetStartLocation(3) got %v, wanted line 2, col 2", info.GetStartLocation(3))
	}
	if !reflect.DeepEqual(info.GetStopLocation(3), common.NewLocation(3, 3)) {
		t.Errorf("info.GetStopLocation(3) got %v, wanted line 2, col 3", info.GetStopLocation(3))
	}
	if info.ComputeOffset(3, 2) != 8 {
		t.Errorf("info.ComputeOffset(3, 2) got %d, wanted 8", info.ComputeOffset(3, 2))
	}
}

func TestSourceInfoNilSafety(t *testing.T) {
	info, err := ast.ProtoToSourceInfo(nil)
	if err != nil {
		t.Fatalf("ast.ProtoToSourceInfo() failed: %v", err)
	}
	tests := []*ast.SourceInfo{
		nil,
		info,
		ast.NewSourceInfo(nil),
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			testInfo := tc
			if testInfo.SyntaxVersion() != "" {
				t.Errorf("SyntaxVersion() got %s, wanted empty string", testInfo.SyntaxVersion())
			}
			if testInfo.Description() != "" {
				t.Errorf("Description() got %s, wanted empty string", testInfo.Description())
			}
			if len(testInfo.LineOffsets()) != 0 {
				t.Errorf("LineOffsets() got %v, wanted empty list", testInfo.LineOffsets())
			}
			if len(testInfo.MacroCalls()) != 0 {
				t.Errorf("MacroCalls() got %v, wanted empty map", testInfo.MacroCalls())
			}
			if len(testInfo.Extensions()) != 0 {
				t.Errorf("Extensions() got %v, wanted empty list", testInfo.Extensions())
			}
			if call, found := testInfo.GetMacroCall(0); found {
				t.Errorf("GetMacroCall(0) got %v, wanted not found", call)
			}
			if r, found := testInfo.GetOffsetRange(0); found {
				t.Errorf("GetOffsetRange(0) got %v, wanted not found", r)
			}
			if loc := testInfo.GetStartLocation(0); loc != common.NoLocation {
				t.Errorf("GetStartLocation(0) got %v, wanted no location", loc)
			}
			if loc := testInfo.GetStopLocation(0); loc != common.NoLocation {
				t.Errorf("GetStopLocation(0) got %v, wanted no location", loc)
			}
			if off := testInfo.ComputeOffset(1, 0); off != 0 {
				t.Errorf("ComputeOffset(1, 0) got %d, wanted 0", off)
			}
			if off := testInfo.ComputeOffset(-2, 0); off != -1 {
				t.Errorf("ComputeOffset(-2, 0) got %d, wanted -1", off)
			}
			if off := testInfo.ComputeOffset(2, 0); off != -1 {
				t.Errorf("ComputeOffset(2, 0) got %d, wanted -1", off)
			}
		})
	}
}

func TestReferenceInfoEquals(t *testing.T) {
	tests := []struct {
		name  string
		a     *ast.ReferenceInfo
		b     *ast.ReferenceInfo
		equal bool
	}{
		{
			name:  "single overload equal",
			a:     ast.NewFunctionReference(overloads.AddBytes),
			b:     ast.NewFunctionReference(overloads.AddBytes),
			equal: true,
		},
		{
			name:  "single overload not equal",
			a:     ast.NewFunctionReference(overloads.AddBytes),
			b:     ast.NewFunctionReference(overloads.AddDouble),
			equal: false,
		},
		{
			name:  "single and multiple overload not equal",
			a:     ast.NewFunctionReference(overloads.AddBytes),
			b:     ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble),
			equal: false,
		},
		{
			name:  "multiple overloads equal",
			a:     ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble),
			b:     ast.NewFunctionReference(overloads.AddDouble, overloads.AddBytes),
			equal: true,
		},
		{
			name:  "identifier reference equal",
			a:     ast.NewIdentReference("BYTES", nil),
			b:     ast.NewIdentReference("BYTES", nil),
			equal: true,
		},
		{
			name:  "identifier reference not equal",
			a:     ast.NewIdentReference("BYTES", nil),
			b:     ast.NewIdentReference("TRUE", nil),
			equal: false,
		},
		{
			name:  "identifier and constant reference not equal",
			a:     ast.NewIdentReference("BYTES", nil),
			b:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			equal: false,
		},
		{
			name:  "constant references equal",
			a:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			b:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			equal: true,
		},
		{
			name:  "constant references not equal",
			a:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			b:     ast.NewIdentReference("BYTES", types.Bytes("bytes-other")),
			equal: false,
		},
		{
			name:  "constant and overload reference not equal",
			a:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			b:     ast.NewFunctionReference(overloads.AddDouble, overloads.AddBytes),
			equal: false,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			out := tc.a.Equals(tc.b)
			if out != tc.equal {
				t.Errorf("%v.Equals(%v) got %v, wanted %v", tc.a, tc.b, out, tc.equal)
			}
		})
	}
}

func TestReferenceInfoAddOverload(t *testing.T) {
	add := ast.NewFunctionReference(overloads.AddBytes)
	add.AddOverload(overloads.AddDouble)
	if !add.Equals(ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble)) {
		t.Error("AddOverload() did not produce equal references")
	}
	add.AddOverload(overloads.AddDouble)
	if !add.Equals(ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble)) {
		t.Error("repeated AddOverload() did not produce equal references")
	}
}

func TestNewSourceInfoRelative(t *testing.T) {
	sourceInfo := ast.NewSourceInfo(
		mockRelativeSource(t,
			"\n \n a || b ?\n cond1 :\n cond2",
			[]int32{1, 2, 13, 25},
			common.NewLocation(2, 1)))
	tests := []struct {
		loc    common.Location
		offset int32
	}{
		// All locations specify a line number starting at 1
		// The location of line 2, offset 1 is the same as the
		// relative offset at location 1, 0 (offset 2)
		{loc: common.NewLocation(1, 0), offset: 2},
		// Equivalent to line 3, column 4
		{loc: common.NewLocation(2, 3), offset: 6},
		// Equivalent to line 4, column 2
		{loc: common.NewLocation(3, 1), offset: 15},
	}
	for _, tst := range tests {
		gotOffset := sourceInfo.ComputeOffset(int32(tst.loc.Line()), int32(tst.loc.Column()))
		if gotOffset != tst.offset {
			t.Errorf("ComputeOffset() got %v, wanted %v", gotOffset, tst.offset)
		}
	}
}

func TestMaxID(t *testing.T) {
	checked := mustTypeCheck(t, `has({'a':'key'}.key)`)
	maxID := ast.MaxID(checked)
	checked.SourceInfo().SetMacroCall(
		maxID+2,
		ast.NewExprFactory().NewIdent(maxID+1, "dummy"))
	// Max ID always pads by 1 between invocations, and it's called twice here
	// due to the presence of the macro which was injected after the fact.
	if ast.MaxID(checked) != maxID+4 {
		t.Errorf("ast.MaxID() got %v, wanted %d", ast.MaxID(checked), maxID+4)
	}
}

func TestNodeCount(t *testing.T) {
	if ast.NodeCount(nil) != 0 {
		t.Errorf("NodeCount(nil) got %d, wanted 0", ast.NodeCount(nil))
	}
	checked := mustTypeCheck(t, `1 + 2`)
	if count := ast.NodeCount(checked); count != 3 {
		t.Errorf("NodeCount(1 + 2) got %d, wanted 3", count)
	}
}

func TestHeights(t *testing.T) {
	tests := []struct {
		expr   string
		height int
	}{
		{`'a' == 'b'`, 1},
		{`'a'.size()`, 1},
		{`[1, 2].size()`, 2},
		{`size('a')`, 1},
		{`has({'a': 1}.a)`, 2},
		{`{'a': 1}`, 1},
		{`{'a': 1}['a']`, 2},
		{`[1, 2, 3].exists(i, i % 2 == 1)`, 4},
		{`google.expr.proto3.test.TestAllTypes{}`, 1},
		{`google.expr.proto3.test.TestAllTypes{repeated_int32: [1, 2]}`, 2},
	}
	for _, tst := range tests {
		checked := mustTypeCheck(t, tst.expr)
		maxHeight := ast.Heights(checked)[checked.Expr().ID()]
		if maxHeight != tst.height {
			t.Errorf("ast.Heights(%q) got max height %d, wanted %d", tst.expr, maxHeight, tst.height)
		}
	}
}

func TestHasExtension(t *testing.T) {
	info := ast.NewSourceInfo(common.NewStringSource("true", "test-only"))
	info.AddExtension(ast.NewExtension("json_name", ast.NewExtensionVersion(1, 1), ast.ComponentRuntime))
	info.AddExtension(ast.NewExtension("cel_block", ast.NewExtensionVersion(1, 1), ast.ComponentRuntime))
	info.AddExtension(ast.NewExtension("cel_block", ast.NewExtensionVersion(1, 1), ast.ComponentRuntime))
	if len(info.Extensions()) != 2 {
		t.Errorf("len(info.Extensions()) got %d, wanted 2", len(info.Extensions()))
	}
	if !info.HasExtension("json_name", ast.NewExtensionVersion(1, 0)) {
		t.Error("info.HasExtension('json_name', 1.0) returned false for v1.1")
	}
	if !info.HasExtension("cel_block", ast.NewExtensionVersion(1, 1)) {
		t.Error("info.HasExtension('cel_block', 1.1) returned false for v1.1")
	}
	if info.HasExtension("json_name", ast.NewExtensionVersion(2, 1)) {
		t.Error("info.HasExtension() returned true for v2.1 when v1.1 configured")
	}
	if info.HasExtension("unrelated", ast.NewExtensionVersion(0, 0)) {
		t.Error("info.HasExtension() returned true for unrelated extensions not set on AST")
	}
}

func mockRelativeSource(t testing.TB, text string, lineOffsets []int32, baseLocation common.Location) common.Source {
	t.Helper()
	return &mockSource{
		Source:       common.NewTextSource(text),
		lineOffsets:  lineOffsets,
		baseLocation: baseLocation}
}

type mockSource struct {
	common.Source
	lineOffsets  []int32
	baseLocation common.Location
}

func (src *mockSource) LineOffsets() []int32 {
	return src.lineOffsets
}

func (src *mockSource) OffsetLocation(offset int32) (common.Location, bool) {
	if offset == 0 {
		return src.baseLocation, true
	}
	return src.Source.OffsetLocation(offset)
}

func TestSourceInfoRenumberIDs(t *testing.T) {
	info := ast.NewSourceInfo(nil)
	for old := int64(1); old <= 5; old++ {
		info.SetOffsetRange(old, ast.OffsetRange{Start: int32(old), Stop: int32(old) + 1})
	}
	original := make(map[int64]ast.OffsetRange)
	maps.Copy(original, info.OffsetRanges())

	// Verify the renumbering is stable.
	var next int64 = 101
	idMap := make(map[int64]int64)
	idGen := func(old int64) int64 {
		if _, found := idMap[old]; !found {
			idMap[old] = next
			next = next + 1
		}
		return idMap[old]
	}
	info.RenumberIDs(idGen)

	if len(info.OffsetRanges()) != 5 {
		t.Errorf("got %d offset ranges, wanted 5", len(info.OffsetRanges()))
	}

	for old := int64(1); old <= 5; old++ {
		want := original[old]
		new := old + 100
		got, found := info.GetOffsetRange(new)
		if !found {
			t.Errorf("offset range for ID %d not found", new)
		}
		if got != want {
			t.Errorf("offset range for ID %d incorrect; got %v, want %v", new, got, want)
		}
	}
}
