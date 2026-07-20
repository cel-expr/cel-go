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
	"sort"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/ext"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	proto3pb "github.com/google/cel-go/test/proto3pb"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestStaticOptimizerUpdateExpr(t *testing.T) {
	expr := `has(a.b)`
	inlined := `[x, y].filter(i, i.size() > 0)[0].z`

	e := optimizerEnv(t)
	exprAST, iss := e.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}

	inlinedAST, iss := e.Compile(inlined)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	opt, err := cel.NewStaticOptimizer(&testOptimizer{t: t, inlineExpr: inlinedAST.NativeRep()})
	if err != nil {
		t.Fatalf("NewStaticOptimizer() failed: %v", err)
	}
	optAST, iss := opt.Optimize(e, exprAST)
	if iss.Err() != nil {
		t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
	}
	optString, err := cel.AstToString(optAST)
	if err != nil {
		t.Fatalf("cel.AstToString() failed: %v", err)
	}
	sourceInfo := optAST.NativeRep().SourceInfo()
	sourceInfoPB, err := ast.SourceInfoToProto(sourceInfo)
	if err != nil {
		t.Fatalf("cel.AstToCheckedExpr() failed: %v", err)
	}
	sourceInfoPB.Positions = nil
	wantTextPB := `
		location:  "<input>"
        line_offsets:  9
        macro_calls:  {
          key:  1
          value:  {
            call_expr:  {
              function:  "has"
              args:  {
                id:  24
                select_expr:  {
                  operand:  {
                    id:  2
                    call_expr:  {
                      function:  "_[_]"
                      args:  {
                        id:  3
                      }
                      args:  {
                        id:  20
                        const_expr:  {
                          int64_value:  0
                        }
                      }
                    }
                  }
                  field:  "z"
                }
              }
            }
          }
        }
        macro_calls:  {
          key:  3
          value:  {
            call_expr:  {
              target:  {
                id:  4
                list_expr:  {
                  elements:  {
                    id:  5
                    ident_expr:  {
                      name:  "x"
                    }
                  }
                  elements:  {
                    id:  6
                    ident_expr:  {
                      name:  "y"
                    }
                  }
                }
              }
              function:  "filter"
              args:  {
                id:  17
                ident_expr:  {
                  name:  "i"
                }
              }
              args:  {
                id:  10
                call_expr:  {
                  function:  "_>_"
                  args:  {
                    id:  11
                    call_expr:  {
                      target:  {
                        id:  12
                        ident_expr:  {
                          name:  "i"
                        }
                      }
                      function:  "size"
                    }
                  }
                  args:  {
                    id:  13
                    const_expr:  {
                      int64_value:  0
                    }
                  }
                }
              }
            }
          }
        }
	`
	var wantSourceInfoPB exprpb.SourceInfo
	if err := prototext.Unmarshal([]byte(wantTextPB), &wantSourceInfoPB); err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	if !proto.Equal(&wantSourceInfoPB, sourceInfoPB) {
		t.Errorf("got source info: %s, wanted %s", prototext.Format(sourceInfoPB), wantTextPB)
	}
	expected := `has([x, y].filter(i, i.size() > 0)[0].z)`
	if expected != optString {
		t.Errorf("inlined got %q, wanted %q", optString, expected)
	}
}

func TestStaticOptimizerNewAST(t *testing.T) {
	tests := []string{
		`[3, 2, 1]`,
		`[1, 2, 3].all(i, i != 0)`,
		`cel.bind(m, {"a": 1, "b": 2}, m.filter(k, m[k] > 1))`,
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc, func(t *testing.T) {
			e := optimizerEnv(t)
			exprAST, iss := e.Compile(tc)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tc, iss.Err())
			}
			opt, err := cel.NewStaticOptimizer(&identityOptimizer{t: t})
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optAST, iss := opt.Optimize(e, exprAST)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			optString, err := cel.AstToString(optAST)
			if err != nil {
				t.Fatalf("cel.AstToString() failed: %v", err)
			}
			if tc != optString {
				t.Errorf("identity optimizer got %q, wanted %q", optString, tc)
			}
		})
	}
}

func TestOptimizeWithSource(t *testing.T) {
	initial := `has(a.b)`
	replacement := `x["a"]`
	e := optimizerEnv(t)
	initialAST, iss := e.Compile(initial)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", initial, iss.Err())
	}
	replacementAST, iss := e.Compile(replacement)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", replacement, iss.Err())
	}

	opt, err := cel.NewStaticOptimizer(
		&replaceOptimizer{t: t, targetAST: replacementAST.NativeRep()},
		cel.OptimizeWithSource(replacementAST.Source()),
	)
	if err != nil {
		t.Fatalf("NewStaticOptimizer() failed: %v", err)
	}
	optAST, iss := opt.Optimize(e, initialAST)
	if iss.Err() != nil {
		t.Fatalf("Optimize() returned an error: %v", iss.Err())
	}

	if optAST.Source().Content() != replacement {
		t.Errorf("got source content %q, wanted %q", optAST.Source().Content(), replacement)
	}
	sourceInfoPB, err := ast.SourceInfoToProto(optAST.NativeRep().SourceInfo())
	if err != nil {
		t.Fatalf("cel.AstToCheckedExpr() failed: %v", err)
	}
	wantTextPB := `
		location: "<input>"
		line_offsets: 7
        positions: {
          key: 1
          value: 1
        }
        positions: {
          key: 2
          value: 0
        }
        positions: {
          key: 3
          value: 2
        }
	`
	var wantSourceInfoPB exprpb.SourceInfo
	if err := prototext.Unmarshal([]byte(wantTextPB), &wantSourceInfoPB); err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	if !proto.Equal(&wantSourceInfoPB, sourceInfoPB) {
		t.Errorf("got source info: %s, wanted %s", prototext.Format(sourceInfoPB), wantTextPB)
	}
}

func TestStaticOptimizerNilAST(t *testing.T) {
	env := optimizerEnv(t)
	opt, err := cel.NewStaticOptimizer(&identityOptimizer{t: t})
	if err != nil {
		t.Fatalf("NewStaticOptimizer() failed: %v", err)
	}
	optAST, iss := opt.Optimize(env, nil)
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "unexpected unspecified type") {
		t.Errorf("opt.Optimize(env, nil) got (%v, %v), wanted unexpected unspecified type", optAST, iss)
	}
}

type identityOptimizer struct {
	t *testing.T
}

func (opt *identityOptimizer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	opt.t.Helper()
	// The copy method should effectively update all of the old macro refs with new ones that are
	// identical, but renumbered.
	main := ctx.CopyASTAndMetadata(a)
	// The new AST call will create a parsed expression which will be type-checked by the static
	// optimizer. The input and output expressions should be identical, though may vary by number
	// though.
	return ctx.NewAST(main)
}

type testOptimizer struct {
	t          *testing.T
	inlineExpr *ast.AST
}

func (opt *testOptimizer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	opt.t.Helper()
	copy := ctx.CopyASTAndMetadata(opt.inlineExpr)
	origID := a.Expr().ID()
	presenceTest, hasMacro := ctx.NewHasMacro(origID, copy)
	macroKeys := getMacroKeys(ctx.MacroCalls())
	if len(macroKeys) != 2 {
		opt.t.Errorf("Got %v macro calls, wanted 2", macroKeys)
	}
	ctx.UpdateExpr(a.Expr(), presenceTest)
	ctx.SetMacroCall(origID, hasMacro)
	return ctx.NewAST(a.Expr())
}

type replaceOptimizer struct {
	t         *testing.T
	targetAST *ast.AST
}

func (opt *replaceOptimizer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	opt.t.Helper()
	copy := ctx.CopyASTAndMetadata(opt.targetAST)
	return ctx.NewAST(copy)
}

func getMacroKeys(macroCalls map[int64]ast.Expr) []int {
	keys := []int{}
	for k := range macroCalls {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	return keys
}

func optimizerEnv(t *testing.T) *cel.Env {
	t.Helper()
	opts := []cel.EnvOption{
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		ext.Bindings(),
		cel.Variable("a", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("x", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("y", cel.MapType(cel.StringType, cel.StringType)),
	}
	e, err := cel.NewEnv(opts...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	return e
}

func TestConstantFoldingOptimizerTwoVar(t *testing.T) {
	tests := []struct {
		expr        string
		folded      string
		knownValues map[string]any
	}{
		{
			expr:   `[1 + 1, 1 + 2].all(i, v, i < 10 && v < 10)`,
			folded: `true`,
		},
		{
			expr:   `[1 + 1, 1 + 2].all(i, v, i < 10 && v < x)`,
			folded: `[2, 3].all(i, v, i < 10 && v < x)`,
		},
		{
			expr:   `[1, 2, 3].transformList(i, v, i + v)`,
			folded: `[1, 3, 5]`,
		},
		{
			expr:   `{'a': 1, 'b': 2}.all(k, v, v > 0)`,
			folded: `true`,
		},
		{
			expr:   `v > 0 && [1 + 1, 1 + 2].all(i, v, i < 10 && v < 10)`,
			folded: `v > 0`,
		},
		{
			expr:   `[1 + 1, 1 + 2].all(i, v, i < 10 && v < 10) && v > 0`,
			folded: `v > 0`,
		},
		{
			expr:   `[1, 2, 3].transformList(i, v, i + v + k)`,
			folded: `[1, 2, 3].transformList(i, v, i + v + k)`,
		},
		{
			expr:   `l.transformList(i, v, i + v)`,
			folded: `l.transformList(i, v, i + v)`,
		},
		{
			expr:   `v > 0 && [1 + 1, 1 + 2].all(i, v, i < 10 && v < 10)`,
			folded: `true`,
			knownValues: map[string]any{
				"v": 5,
			},
		},
		{
			expr:   `[1 + 1, 1 + 2].all(i, v, i < 10 && v < .v)`,
			folded: `true`,
			knownValues: map[string]any{
				"v": 10,
			},
		},
	}
	e, err := cel.NewEnv(
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		ext.TwoVarComprehensions(),
		cel.Variable("x", cel.IntType),
		cel.Variable("v", cel.IntType),
		cel.Variable("i", cel.IntType),
		cel.Variable("k", cel.IntType),
		cel.Variable("l", cel.ListType(cel.IntType)),
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
			var foldingOpts []cel.ConstantFoldingOption
			if tc.knownValues != nil {
				knownValues, err := cel.NewActivation(tc.knownValues)
				if err != nil {
					t.Fatalf("NewActivation() failed: %v", err)
				}
				foldingOpts = append(foldingOpts, cel.FoldKnownValues(knownValues))
			}
			folder, err := cel.NewConstantFoldingOptimizer(foldingOpts...)
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := cel.NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := cel.AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}
