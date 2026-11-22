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

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	chkdecls "cel.dev/cel-go/checker/decls"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestConvertAST(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name  string
		goAST *ast.AST
		pbAST *exprpb.CheckedExpr
	}{
		{
			name: "simple ast",
			goAST: ast.NewCheckedAST(ast.NewAST(nil, nil),
				map[int64]*types.Type{
					1: types.BoolType,
					2: types.DynType,
				},
				map[int64]*ast.ReferenceInfo{
					1: ast.NewFunctionReference(overloads.LogicalNot),
					2: ast.NewIdentReference("TRUE", types.True),
				},
			),
			pbAST: &exprpb.CheckedExpr{
				Expr:       &exprpb.Expr{},
				SourceInfo: &exprpb.SourceInfo{},
				TypeMap: map[int64]*exprpb.Type{
					1: chkdecls.Bool,
					2: chkdecls.Dyn,
				},
				ReferenceMap: map[int64]*exprpb.Reference{
					1: {OverloadId: []string{overloads.LogicalNot}},
					2: {
						Name: "TRUE",
						Value: &exprpb.Constant{
							ConstantKind: &exprpb.Constant_BoolValue{BoolValue: true},
						},
					},
				},
			},
		},
		{
			name: "comprehension ast",
			goAST: ast.NewAST(
				fac.NewComprehensionTwoVar(1,
					fac.NewIdent(2, "data"),
					"i",
					"v",
					"__result__",
					fac.NewList(3, []ast.Expr{}, []int32{}),
					fac.NewLiteral(4, types.True),
					fac.NewCall(8, operators.Add,
						fac.NewAccuIdent(9),
						fac.NewCall(5, operators.Add,
							fac.NewIdent(6, "i"),
							fac.NewIdent(7, "v"),
						)),
					fac.NewAccuIdent(10),
				), nil),
			pbAST: &exprpb.CheckedExpr{
				Expr: &exprpb.Expr{
					Id: 1,
					ExprKind: &exprpb.Expr_ComprehensionExpr{
						ComprehensionExpr: &exprpb.Expr_Comprehension{
							IterRange: &exprpb.Expr{
								Id: 2,
								ExprKind: &exprpb.Expr_IdentExpr{
									IdentExpr: &exprpb.Expr_Ident{
										Name: "data",
									},
								},
							},
							IterVar:  "i",
							IterVar2: "v",
							AccuVar:  "__result__",
							AccuInit: &exprpb.Expr{
								Id: 3,
								ExprKind: &exprpb.Expr_ListExpr{
									ListExpr: &exprpb.Expr_CreateList{},
								},
							},
							LoopCondition: &exprpb.Expr{
								Id: 4,
								ExprKind: &exprpb.Expr_ConstExpr{
									ConstExpr: &exprpb.Constant{
										ConstantKind: &exprpb.Constant_BoolValue{
											BoolValue: true,
										},
									},
								},
							},
							LoopStep: &exprpb.Expr{
								Id: 8,
								ExprKind: &exprpb.Expr_CallExpr{
									CallExpr: &exprpb.Expr_Call{
										Function: operators.Add,
										Args: []*exprpb.Expr{
											{
												Id: 9,
												ExprKind: &exprpb.Expr_IdentExpr{
													IdentExpr: &exprpb.Expr_Ident{
														Name: "__result__",
													},
												},
											},
											{
												Id: 5,
												ExprKind: &exprpb.Expr_CallExpr{
													CallExpr: &exprpb.Expr_Call{
														Function: operators.Add,
														Args: []*exprpb.Expr{
															{
																Id: 6,
																ExprKind: &exprpb.Expr_IdentExpr{
																	IdentExpr: &exprpb.Expr_Ident{
																		Name: "i",
																	},
																},
															},
															{
																Id: 7,
																ExprKind: &exprpb.Expr_IdentExpr{
																	IdentExpr: &exprpb.Expr_Ident{
																		Name: "v",
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							Result: &exprpb.Expr{
								Id: 10,
								ExprKind: &exprpb.Expr_IdentExpr{
									IdentExpr: &exprpb.Expr_Ident{
										Name: "__result__",
									},
								},
							},
						},
					},
				},
				SourceInfo:   &exprpb.SourceInfo{},
				TypeMap:      map[int64]*exprpb.Type{},
				ReferenceMap: map[int64]*exprpb.Reference{},
			},
		},
		{
			name: "json names ast",
			goAST: ast.NewCheckedAST(
				ast.NewAST(
					fac.NewCall(2, overloads.LogicalNot, fac.NewIdent(1, "value")),
					sourceWithExtension(
						ast.NewSourceInfo(common.NewTextSource("!value")),
						ast.NewExtension("json_name", ast.NewExtensionVersion(1, 1), ast.ComponentRuntime),
					),
				),
				map[int64]*types.Type{
					1: types.BoolType,
					2: types.BoolType,
				},
				map[int64]*ast.ReferenceInfo{
					1: ast.NewFunctionReference(overloads.LogicalNot),
					2: ast.NewIdentReference("value", nil),
				},
			),
			pbAST: &exprpb.CheckedExpr{
				Expr: &exprpb.Expr{
					Id: 2,
					ExprKind: &exprpb.Expr_CallExpr{
						CallExpr: &exprpb.Expr_Call{
							Function: "!_",
							Args: []*exprpb.Expr{
								{
									Id: 1,
									ExprKind: &exprpb.Expr_IdentExpr{
										IdentExpr: &exprpb.Expr_Ident{
											Name: "value",
										},
									},
								},
							},
						},
					},
				},
				SourceInfo: &exprpb.SourceInfo{
					Location: "<input>",
					Extensions: []*exprpb.SourceInfo_Extension{
						{
							Id: "json_name",
							Version: &exprpb.SourceInfo_Extension_Version{
								Major: 1,
								Minor: 1,
							},
							AffectedComponents: []exprpb.SourceInfo_Extension_Component{
								exprpb.SourceInfo_Extension_COMPONENT_RUNTIME,
							},
						},
					},
				},
				TypeMap: map[int64]*exprpb.Type{
					1: chkdecls.Bool,
					2: chkdecls.Bool,
				},
				ReferenceMap: map[int64]*exprpb.Reference{
					1: {OverloadId: []string{overloads.LogicalNot}},
					2: {Name: "value"},
				},
			},
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			goAST := tc.goAST
			pbAST := tc.pbAST
			checkedAST, err := ast.ToAST(pbAST)
			if err != nil {
				t.Fatalf("ProtoToAST() failed: %v", err)
			}
			if !reflect.DeepEqual(checkedAST.ReferenceMap(), goAST.ReferenceMap()) ||
				!reflect.DeepEqual(checkedAST.TypeMap(), goAST.TypeMap()) {
				t.Errorf("conversion to AST did not produce identical results: got %v, wanted %v", checkedAST, goAST)
			}
			if !reflect.DeepEqual(checkedAST.SourceInfo().Extensions(), goAST.SourceInfo().Extensions()) {
				t.Errorf("conversion to AST did not preserve SourceInfo extensions. got %v, wanted %v",
					checkedAST.SourceInfo().Extensions(), goAST.SourceInfo().Extensions())
			}
			if len(checkedAST.ReferenceMap()) > 2 {
				if !checkedAST.ReferenceMap()[1].Equals(goAST.ReferenceMap()[1]) ||
					!checkedAST.ReferenceMap()[2].Equals(goAST.ReferenceMap()[2]) {
					t.Error("converted reference info values not equal")
				}
			}
			checkedExpr, err := ast.ToProto(checkedAST)
			if err != nil {
				t.Fatalf("ASTToProto() failed: %v", err)
			}
			if !proto.Equal(checkedExpr, pbAST) {
				t.Errorf("conversion to protobuf did not produce identical results: got %v, wanted %v", checkedExpr, pbAST)
			}
		})
	}
}

func TestConvertProtoToEntryExpr(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		goAST ast.EntryExpr
		pbAST *exprpb.Expr_CreateStruct_Entry
	}{
		{
			goAST: fac.NewMapEntry(1,
				fac.NewIdent(2, "var_key"),
				fac.NewLiteral(3, types.String("hello")),
				true),
			pbAST: &exprpb.Expr_CreateStruct_Entry{
				Id: 1,
				KeyKind: &exprpb.Expr_CreateStruct_Entry_MapKey{
					MapKey: &exprpb.Expr{
						Id: 2,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{
								Name: "var_key",
							},
						},
					},
				},
				Value: &exprpb.Expr{
					Id: 3,
					ExprKind: &exprpb.Expr_ConstExpr{
						ConstExpr: &exprpb.Constant{
							ConstantKind: &exprpb.Constant_StringValue{
								StringValue: "hello",
							},
						},
					},
				},
				OptionalEntry: true,
			},
		},
		{
			goAST: fac.NewStructField(1,
				"field_name",
				fac.NewLiteral(2, types.String("hello")),
				false),
			pbAST: &exprpb.Expr_CreateStruct_Entry{
				Id: 1,
				KeyKind: &exprpb.Expr_CreateStruct_Entry_FieldKey{
					FieldKey: "field_name",
				},
				Value: &exprpb.Expr{
					Id: 2,
					ExprKind: &exprpb.Expr_ConstExpr{
						ConstExpr: &exprpb.Constant{
							ConstantKind: &exprpb.Constant_StringValue{
								StringValue: "hello",
							},
						},
					},
				},
				OptionalEntry: false,
			},
		},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			goAST := tc.goAST
			pbAST := tc.pbAST
			gotGoAST, err := ast.ProtoToEntryExpr(pbAST)
			if err != nil {
				t.Fatalf("ProtoToEntryExpr() failed: %v", err)
			}
			if !reflect.DeepEqual(goAST, gotGoAST) {
				t.Errorf("conversion to go AST did not produce identical results: got %v, wanted %v", gotGoAST, goAST)
			}
			gotProtoAST, err := ast.EntryExprToProto(gotGoAST)
			if err != nil {
				t.Fatalf("EntryExprToProto() failed: %v", err)
			}
			if !proto.Equal(gotProtoAST, pbAST) {
				t.Errorf("conversion to protobuf did not produce identical results: got %v, wanted %v", gotProtoAST, pbAST)
			}
		})
	}
}

func TestConvertExpr(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		expr       string
		wantExpr   ast.Expr
		macroCalls map[int64]ast.Expr
	}{
		{
			expr:     `true`,
			wantExpr: fac.NewLiteral(1, types.True),
		},
		{
			expr:     `a`,
			wantExpr: fac.NewIdent(1, "a"),
		},
		{
			expr:     `a.b`,
			wantExpr: fac.NewSelect(2, fac.NewIdent(1, "a"), "b"),
		},
		{
			expr:     `has(a.b)`,
			wantExpr: fac.NewPresenceTest(4, fac.NewIdent(2, "a"), "b"),
			macroCalls: map[int64]ast.Expr{
				4: fac.NewCall(0, "has", fac.NewSelect(3, fac.NewIdent(2, "a"), "b")),
			},
		},
		{
			expr:     `!a`,
			wantExpr: fac.NewCall(1, "!_", fac.NewIdent(2, "a")),
		},
		{
			expr:     `a.size()`,
			wantExpr: fac.NewMemberCall(2, "size", fac.NewIdent(1, "a")),
		},
		{
			expr:     `[a]`,
			wantExpr: fac.NewList(1, []ast.Expr{fac.NewIdent(2, "a")}, []int32{}),
		},
		{
			expr:     `[?a]`,
			wantExpr: fac.NewList(1, []ast.Expr{fac.NewIdent(2, "a")}, []int32{0}),
		},
		{
			expr: `{'string': 42}`,
			wantExpr: fac.NewMap(1, []ast.EntryExpr{
				fac.NewMapEntry(2,
					fac.NewLiteral(3, types.String("string")),
					fac.NewLiteral(4, types.Int(42)),
					false),
			}),
		},
		{
			expr: `{?'string': a.?b}`,
			wantExpr: fac.NewMap(1, []ast.EntryExpr{
				fac.NewMapEntry(2,
					fac.NewLiteral(3, types.String("string")),
					fac.NewCall(6, "_?._", fac.NewIdent(4, "a"), fac.NewLiteral(5, types.String("b"))),
					true),
			}),
		},
		{
			expr: `custom.StructType{uint_field: 42u}`,
			wantExpr: fac.NewStruct(1,
				"custom.StructType",
				[]ast.EntryExpr{
					fac.NewStructField(2,
						"uint_field",
						fac.NewLiteral(3, types.Uint(42)),
						false),
				}),
		},
		{
			expr: `[].exists(i, i)`,
			wantExpr: fac.NewComprehension(12,
				fac.NewList(1, []ast.Expr{}, []int32{}),
				"i",
				fac.AccuIdentName(),
				fac.NewLiteral(5, types.False),
				fac.NewCall(8, "@not_strictly_false", fac.NewCall(7, "!_", fac.NewAccuIdent(6))),
				fac.NewCall(10, "_||_", fac.NewAccuIdent(9), fac.NewIdent(4, "i")),
				fac.NewAccuIdent(11),
			),
			macroCalls: map[int64]ast.Expr{
				12: fac.NewMemberCall(0, "exists",
					fac.NewList(1, []ast.Expr{}, []int32{}),
					fac.NewIdent(3, "i"),
					fac.NewIdent(4, "i"),
				),
			},
		},
	}
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
		parser.EnableOptionalSyntax(true),
		parser.PopulateMacroCalls(true))
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			parsed, errs := p.Parse(common.NewTextSource(tc.expr))
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("Parse() failed: %s", errs.ToDisplayString())
			}
			gotPBExpr, err := ast.ExprToProto(parsed.Expr())
			if err != nil {
				t.Fatalf("ast.ExprToProto() failed: %v", err)
			}
			wantPBExpr, err := ast.ExprToProto(tc.wantExpr)
			if err != nil {
				t.Fatalf("ast.ExprToProto() failed: %v", err)
			}
			if !proto.Equal(gotPBExpr, wantPBExpr) {
				t.Errorf("got %v\n, wanted %v",
					prototext.Format(gotPBExpr), prototext.Format(wantPBExpr))
			}
			gotExpr := parsed.Expr()
			if !reflect.DeepEqual(gotExpr, tc.wantExpr) {
				t.Errorf("got %v, wanted %v", gotExpr, tc.wantExpr)
			}
			gotExprRoundtrip, err := ast.ProtoToExpr(gotPBExpr)
			if err != nil {
				t.Fatalf("ast.ProtoToExpr() failed: %v", err)
			}
			if !reflect.DeepEqual(parsed.Expr(), gotExprRoundtrip) {
				t.Errorf("ast.ProtoToExpr() got %v, wanted %v", gotExprRoundtrip, parsed.Expr())
			}
			info := parsed.SourceInfo()
			for id, wantCall := range tc.macroCalls {
				call, found := info.GetMacroCall(id)
				if !found {
					t.Fatalf("GetMacroCall(%d) returned not found", id)
				}
				if !reflect.DeepEqual(call, wantCall) {
					t.Errorf("macro call got %v, wanted %v", call, wantCall)
				}
			}
		})
	}
}

func TestSourceInfoToProto(t *testing.T) {
	expr := `[{}, {'field': true}].exists(i, has(i.field))`
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
		parser.EnableOptionalSyntax(true),
		parser.PopulateMacroCalls(true))
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	parsed, errs := p.Parse(common.NewTextSource(expr))
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Parse() failed: %s", errs.ToDisplayString())
	}
	pbInfo, err := ast.SourceInfoToProto(parsed.SourceInfo())
	if err != nil {
		t.Fatalf("SourceInfoToProto() failed: %v", err)
	}
	wantInfo := `
		location: "<input>"
        line_offsets: 46
        positions: {
          key: 1
          value: 0
        }
        positions: {
          key: 2
          value: 1
        }
        positions: {
          key: 3
          value: 5
        }
        positions: {
          key: 4
          value: 13
        }
        positions: {
          key: 5
          value: 6
        }
        positions: {
          key: 6
          value: 15
        }
        positions: {
          key: 8
          value: 29
        }
        positions: {
          key: 10
          value: 36
        }
        positions: {
          key: 11
          value: 37
        }
        positions: {
          key: 12
          value: 35
        }
        positions: {
          key: 13
          value: 28
        }
        positions: {
          key: 14
          value: 28
        }
        positions: {
          key: 15
          value: 28
        }
        positions: {
          key: 16
          value: 28
        }
        positions: {
          key: 17
          value: 28
        }
        positions: {
          key: 18
          value: 28
        }
        positions: {
          key: 19
          value: 28
        }
        positions: {
          key: 20
          value: 28
        }
        macro_calls: {
          key: 12
          value: {
            call_expr: {
              function: "has"
              args: {
                id: 11
                select_expr: {
                  operand: {
                    id: 10
                    ident_expr: {
                      name: "i"
                    }
                  }
                  field: "field"
                }
              }
            }
          }
        }
        macro_calls: {
          key: 20
          value: {
            call_expr: {
              target: {
                id: 1
                list_expr: {
                  elements: {
                    id: 2
                    struct_expr: {}
                  }
                  elements: {
                    id: 3
                    struct_expr: {
                      entries: {
                        id: 4
                        map_key: {
                          id: 5
                          const_expr: {
                            string_value: "field"
                          }
                        }
                        value: {
                          id: 6
                          const_expr: {
                            bool_value: true
                          }
                        }
                      }
                    }
                  }
                }
              }
              function: "exists"
              args: {
                id: 8
                ident_expr: {
                  name: "i"
                }
              }
              args: {
                id: 12
              }
            }
          }
        }`
	wantPBInfo := &exprpb.SourceInfo{}
	prototext.Unmarshal([]byte(wantInfo), wantPBInfo)
	if !proto.Equal(pbInfo, wantPBInfo) {
		t.Errorf("SourceInfoToProto() got %v, wanted %v",
			prototext.Format(pbInfo), prototext.Format(wantPBInfo))
	}
}

func TestReferenceInfoToProtoError(t *testing.T) {
	out, err := ast.ReferenceInfoToProto(
		ast.NewIdentReference("SECOND", types.Duration{Seconds: 1}))
	if err == nil {
		t.Errorf("ReferenceInfoToProto() got %v, wanted error", out)
	}
}

func TestProtoToReferenceInfoError(t *testing.T) {
	out, err := ast.ProtoToReferenceInfo(&exprpb.Reference{Value: &exprpb.Constant{}})
	if err == nil {
		t.Errorf("ProtoToReferenceInfo() got %v, wanted error", out)
	}
}

func TestConvertVal(t *testing.T) {
	tests := []ref.Val{
		types.True,
		types.Bytes("bytes"),
		types.Double(3.2),
		types.Int(-1),
		types.NullValue,
		types.String("string"),
		types.Uint(27),
	}
	for _, tst := range tests {
		c, err := ast.ValToConstant(tst)
		if err != nil {
			t.Errorf("ValToConstant(%v) failed: %v", tst, err)
		}
		v, err := ast.ConstantToVal(c)
		if err != nil {
			t.Errorf("ValToConstant(%v) failed: %v", c, err)
		}
		if tst.Equal(v) != types.True {
			t.Errorf("roundtrip from %v to %v and back did not produce equal results, got %v, wanted %v", tst, c, v, tst)
		}
	}
}

func TestValToConstantError(t *testing.T) {
	out, err := ast.ValToConstant(types.Duration{Nanos: 10})
	if err == nil {
		t.Errorf("ValToConstant() got %v, wanted error", out)
	}
}

func TestConstantToValError(t *testing.T) {
	out, err := ast.ConstantToVal(&exprpb.Constant{})
	if err == nil {
		t.Errorf("ConstantToVal() got %v, wanted error", out)
	}
}

func sourceWithExtension(info *ast.SourceInfo, ext ast.Extension) *ast.SourceInfo {
	info.AddExtension(ext)
	return info
}
