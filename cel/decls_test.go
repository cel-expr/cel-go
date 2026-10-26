// Copyright 2022 Google LLC
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
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	chkdecls "cel.dev/cel-go/checker/decls"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/stdlib"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

var dispatchTests = []struct {
	expr string
	out  any
}{
	{
		expr: "max(-1)",
		out:  types.IntNegOne,
	},
	{
		expr: "max(-1, 0)",
		out:  types.IntZero,
	},
	{
		expr: "max(-1, 0, 1)",
		out:  types.IntOne,
	},
	{
		expr: "max(dyn(1.2))",
		out:  fmt.Errorf("no such overload: max(double)"),
	},

	{
		expr: "max(1, dyn(1.2))",
		out:  fmt.Errorf("no such overload: max(int, double)"),
	},
	{
		expr: "max(1, 2, dyn(1.2))",
		out:  fmt.Errorf("no such overload: max(int, int, double)"),
	},
	{
		expr: "max(err, 1)",
		out:  fmt.Errorf("error argument"),
	},
	{
		expr: "max(err, unk)",
		out:  fmt.Errorf("error argument"),
	},
	{
		expr: "max(unk, unk)",
		out:  types.NewUnknown(42, nil),
	},
	{
		expr: "max(unk, unk, unk)",
		out:  types.NewUnknown(42, nil),
	},
}

func TestFunctionMerge(t *testing.T) {
	size := Function("size",
		Overload("size_map", []*Type{MapType(TypeParamType("K"), TypeParamType("V"))}, IntType),
		Overload("size_list", []*Type{ListType(TypeParamType("V"))}, IntType),
		Overload("size_string", []*Type{StringType}, IntType),
		Overload("size_bytes", []*Type{BytesType}, IntType),
		MemberOverload("map_size", []*Type{MapType(TypeParamType("K"), TypeParamType("V"))}, IntType),
		MemberOverload("list_size", []*Type{ListType(TypeParamType("V"))}, IntType),
		MemberOverload("string_size", []*Type{StringType}, IntType),
		MemberOverload("bytes_size", []*Type{BytesType}, IntType),
		SingletonUnaryBinding(func(arg ref.Val) ref.Val {
			return arg.(traits.Sizer).Size()
		}, traits.SizerType),
	)
	// Note, the size() implementation is inherited from the singleton implementation for the size function.
	// It is possible to redefine the singleton, but the singleton approach is incompatible with specialized
	// overloads as the singleton is compatible with the parse-only approach and compiled approach; but
	// a mix of singleton and specialized overloads might result in a singleton which does not encompass
	// dynamic dispatch to all possible overloads.
	sizeExt := Function("size",
		Overload("size_vector", []*Type{OpaqueType("vector", TypeParamType("V"))}, IntType),
		MemberOverload("vector_size", []*Type{OpaqueType("vector", TypeParamType("V"))}, IntType))

	vectorExt := Function("vector",
		Overload("vector_list", []*Type{ListType(TypeParamType("V"))}, OpaqueType("vector", TypeParamType("V")),
			UnaryBinding(func(list ref.Val) ref.Val {
				return list
			})))

	eq := Function(operators.Equals,
		Overload(operators.Equals, []*Type{TypeParamType("T"), TypeParamType("T")}, BoolType,
			BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				return lhs.Equal(rhs)
			})))

	e, err := NewCustomEnv(size, sizeExt, vectorExt, eq)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Compile(`[[0].size() == 1,
		{'a': true, 'b': false}.size() == 2,
		'hello'.size() == 5,
		b'hello'.size() == 5,
		vector([1.2, 2.3, 3.4]).size() == 3]`)
	if iss.Err() != nil {
		t.Fatalf("Compile() errored: %v", iss.Err())
	}
	if !reflect.DeepEqual(ast.OutputType(), ListType(BoolType)) {
		t.Errorf("Compile() produced a %v, wanted bool", ast.OutputType())
	}
	if ast.OutputType().String() != "list(bool)" {
		t.Errorf("ast.OutputType().String() produced %s, wanted list(bool)", ast.OutputType())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("e.Program(ast) failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Errorf("prg.Eval() errored: %v", err)
	}
	want := types.DefaultTypeAdapter.NativeToValue([]bool{true, true, true, true, true})
	if out.Equal(want) != types.True {
		t.Errorf("prg.Eval() got %v, wanted %v", out, want)
	}

	sizeBad := Function("size",
		Overload("size_vector", []*Type{OpaqueType("vector", TypeParamType("V"))}, IntType),
		MemberOverload("vector_size", []*Type{OpaqueType("vector", TypeParamType("V"))}, IntType),
		SingletonBinaryBinding(func(lhs, rhs ref.Val) ref.Val {
			return nil
		}))
	_, err = NewCustomEnv(size, sizeBad)
	if err == nil || !strings.Contains(err.Error(), "already has a singleton binding") {
		t.Errorf("NewCustomEnv(size, size) did not produce the expected error: %v", err)
	}
	e, err = NewCustomEnv(size,
		Function("size",
			Overload("size_int", []*Type{IntType}, IntType,
				UnaryBinding(func(arg ref.Val) ref.Val { return types.Int(2) }),
			),
		),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv(size, <custom>) failed: %v", err)
	}
	_, err = e.Program(ast)
	if err == nil || !strings.Contains(err.Error(), "incompatible with specialized overloads") {
		t.Errorf("NewCustomEnv(size, size_specialization) did not produce the expected error: %v", err)
	}
}

func TestFunctionMergeDuplicate(t *testing.T) {
	maxFunc := Function("max",
		Overload("max_int", []*Type{IntType}, IntType),
		Overload("max_int", []*Type{IntType}, IntType),
	)
	_, err := NewCustomEnv(maxFunc, maxFunc)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
}

func TestFunctionMergeDeclarationAndDefinition(t *testing.T) {
	idFuncDecl := Function("id", Overload("id", []*Type{TypeParamType("T")}, TypeParamType("T"), OverloadIsNonStrict()))
	idFuncDef := Function("id",
		Overload("id", []*Type{TypeParamType("T")}, TypeParamType("T"), OverloadIsNonStrict(),
			UnaryBinding(func(arg ref.Val) ref.Val {
				return arg
			}),
		),
	)
	env, err := NewCustomEnv(Variable("x", AnyType), idFuncDecl, idFuncDef)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := env.Compile("id(x)")
	if iss.Err() != nil {
		t.Fatalf("env.Compile(id(x)) failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"x": true})
	if err != nil {
		t.Fatalf("prg.Eval(x: true) errored: %v", err)
	}
	if out != types.True {
		t.Errorf("prg.Eval() yielded %v, wanted true", out)
	}
}

func TestFunctionMergeCollision(t *testing.T) {
	maxFunc := Function("max",
		Overload("max_int", []*Type{IntType}, IntType),
		Overload("max_int2", []*Type{IntType}, IntType),
	)
	_, err := NewCustomEnv(maxFunc, maxFunc)
	if err == nil {
		t.Fatal("NewCustomEnv() succeeded, wanted collision error")
	}
}

func TestFunctionNoOverloads(t *testing.T) {
	_, err := NewCustomEnv(
		Function("right", SingletonBinaryBinding(func(arg1, arg2 ref.Val) ref.Val {
			return arg2
		})))
	if err == nil || !strings.Contains(err.Error(), "must have at least one overload") {
		t.Errorf("got %v, wanted 'must have at least one overload'", err)
	}
}

func TestSingletonUnaryBinding(t *testing.T) {
	// Test the case where the singleton unary impl is merged with the earlier declaration.
	e, err := NewCustomEnv(
		Variable("x", AnyType),
		Function("id", Overload("id_any", []*Type{AnyType}, AnyType)),
		Function("id", Overload("id_any", []*Type{AnyType}, AnyType), SingletonUnaryBinding(func(arg ref.Val) ref.Val { return arg })),
	)
	if err != nil {
		t.Errorf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Parse("id(x)")
	if iss.Err() != nil {
		t.Fatalf("Parse('id(x)') failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"x": "hello"})
	if err != nil {
		t.Errorf("prg.Eval(x=hello) failed: %v", err)
	}
	if out.Equal(types.String("hello")) != types.True {
		t.Errorf("Eval got %v, wanted 'hello'", out)
	}
}

func TestSingletonUnaryBindingParameterized(t *testing.T) {
	// Test the case where the singleton unary impl is merged with the earlier declaration.
	e, err := NewCustomEnv(
		Variable("x", AnyType),
		Function("isSorted", MemberOverload("list_int_is_sorted", []*Type{ListType(IntType)}, BoolType)),
		Function("isSorted", MemberOverload("list_uint_is_sorted", []*Type{ListType(UintType)}, BoolType),
			SingletonUnaryBinding(func(arg ref.Val) ref.Val { return types.True })),
	)
	if err != nil {
		t.Errorf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Parse("x.isSorted()")
	if iss.Err() != nil {
		t.Fatalf("Parse('x.isSorted()') failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"x": []int{1, 2, 3}})
	if err != nil {
		t.Errorf("prg.Eval(x=[1,2,3]) failed: %v", err)
	}
	if out != types.True {
		t.Errorf("Eval got %v, wanted true", out)
	}
}

func TestSingletonBinaryBinding(t *testing.T) {
	_, err := NewCustomEnv(
		Function("right",
			Overload("right_int_int", []*Type{IntType, IntType}, IntType),
			Overload("right_double_double", []*Type{DoubleType, DoubleType}, DoubleType),
			Overload("right_string_string", []*Type{StringType, StringType}, StringType),
			SingletonBinaryBinding(func(arg1, arg2 ref.Val) ref.Val {
				return arg2
			}, traits.ComparerType),
		),
	)
	if err != nil {
		t.Errorf("NewCustomEnv() failed: %v", err)
	}
}

func TestSingletonFunctionBinding(t *testing.T) {
	env, err := NewCustomEnv(
		Variable("unk", DynType),
		Variable("err", DynType),
		Function("dyn",
			Overload("dyn", []*Type{DynType}, DynType),
			SingletonUnaryBinding(func(arg ref.Val) ref.Val {
				return arg
			})),
		Function("max",
			Overload("max_int", []*Type{IntType}, IntType),
			Overload("max_int_int", []*Type{IntType, IntType}, IntType),
			Overload("max_int_int_int", []*Type{IntType, IntType, IntType}, IntType),
			SingletonFunctionBinding(func(args ...ref.Val) ref.Val {
				max := types.Int(math.MinInt64)
				for _, arg := range args {
					i, ok := arg.(types.Int)
					if !ok {
						// With a singleton implementation, the error handling must be explicitly
						// performed as a binding detail of the singleton function.
						// With custom overload implementations, a function guard is automatically
						// added to the function to validate that the runtime types are compatible
						// to provide some basic invocation protections.
						return decls.MaybeNoSuchOverload("max", args...)
					}
					if i > max {
						max = i
					}
				}
				return max
			}),
		),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}

	for _, tc := range dispatchTests {
		t.Run(fmt.Sprintf("Parse(%s)", tc.expr), func(t *testing.T) {
			testParse(t, env, tc.expr, tc.out)
		})
	}
	for _, tc := range dispatchTests {
		t.Run(fmt.Sprintf("Compile(%s)", tc.expr), func(t *testing.T) {
			testCompile(t, env, tc.expr, tc.out)
		})
	}
}

func TestUnaryBinding(t *testing.T) {
	_, err := NewCustomEnv(
		Function("dyn",
			Overload("dyn", []*Type{}, DynType,
				// wrong arg count
				UnaryBinding(func(arg ref.Val) ref.Val { return arg }),
			),
		),
	)
	if err == nil || !strings.Contains(err.Error(), "function bound to non-unary overload") {
		t.Errorf("bad binding did not produce expected error: %v", err)
	}

	e, err := NewCustomEnv(
		Function("size",
			Overload("size_non_strict", []*Type{ListType(DynType)}, IntType,
				OverloadIsNonStrict(),
				OverloadOperandTrait(traits.SizerType),
				UnaryBinding(func(arg ref.Val) ref.Val {
					if types.IsUnknownOrError(arg) {
						return arg
					}
					return arg.(traits.Sizer).Size()
				}),
			),
		),
		Variable("x", ListType(DynType)),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Compile("size(x)")
	if iss.Err() != nil {
		t.Fatalf("Compile(size(x)) failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"x": types.NewUnknown(1, nil)})
	if err != nil {
		t.Fatalf("prg.Eval(x=unk) failed: %v", err)
	}
	if !types.NewUnknown(1, nil).Contains(out.(*types.Unknown)) {
		t.Errorf("prg.Eval(x=unk) returned %v, wanted unknown{1}", out)
	}
}

func TestBinaryBinding(t *testing.T) {
	e, err := NewCustomEnv(
		Function("max",
			Overload("max_int_int", []*Type{IntType, IntType}, IntType,
				OverloadIsNonStrict(),
				BinaryBinding(func(arg1, arg2 ref.Val) ref.Val {
					if types.IsUnknownOrError(arg1) {
						return arg2
					}
					if types.IsUnknownOrError(arg2) {
						return arg1
					}
					i1 := arg1.(types.Int)
					i2 := arg2.(types.Int)
					if i1 > i2 {
						return i1
					}
					return i2
				}),
			),
		),
		Variable("x", IntType),
		Variable("y", IntType),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Parse("max(x, y)")
	if iss.Err() != nil {
		t.Fatalf("Parse(max(x, y))) failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"x": types.NewUnknown(1, nil), "y": 1})
	if err != nil {
		t.Fatalf("prg.Eval(x=unk) failed: %v", err)
	}
	if !reflect.DeepEqual(out, types.IntOne) {
		t.Errorf("prg.Eval(x=unk, y=1) returned %v, wanted 1", out)
	}
	out, _, err = prg.Eval(map[string]any{"x": 2, "y": types.NewUnknown(2, nil)})
	if err != nil {
		t.Fatalf("prg.Eval(x=2, y=unk) failed: %v", err)
	}
	if !reflect.DeepEqual(out, types.Int(2)) {
		t.Errorf("prg.Eval(x=2, y=unk) returned %v, wanted 2", out)
	}
	out, _, err = prg.Eval(map[string]any{"x": 2, "y": 1})
	if err != nil {
		t.Fatalf("prg.Eval(x=2, y=1) failed: %v", err)
	}
	if !reflect.DeepEqual(out, types.Int(2)) {
		t.Errorf("prg.Eval(x=2, y=1) returned %v, wanted 2", out)
	}

	_, err = NewCustomEnv(
		Function("right",
			Overload("right_int_int", []*Type{IntType, IntType, IntType}, IntType,
				// wrong arg count
				BinaryBinding(func(arg1, arg2 ref.Val) ref.Val { return arg2 }),
			),
		),
	)
	if err == nil || !strings.Contains(err.Error(), "function bound to non-binary overload") {
		t.Errorf("bad binding did not produce expected error: %v", err)
	}
}

func TestFunctionBinding(t *testing.T) {
	e, err := NewCustomEnv(
		Variable("unk", DynType),
		Variable("err", DynType),
		Function("dyn",
			Overload("dyn", []*Type{DynType}, DynType),
			SingletonUnaryBinding(func(arg ref.Val) ref.Val {
				return arg
			})),
		Function("max",
			Overload("max_int", []*Type{IntType}, IntType,
				UnaryBinding(func(arg ref.Val) ref.Val { return arg })),
			Overload("max_int_int", []*Type{IntType, IntType}, IntType,
				BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					if lhs.(types.Int).Compare(rhs) == types.IntNegOne {
						return rhs
					}
					return lhs
				})),
			Overload("max_int_int_int", []*Type{IntType, IntType, IntType}, IntType,
				FunctionBinding(func(args ...ref.Val) ref.Val {
					max := types.Int(math.MinInt64)
					for _, arg := range args {
						i := arg.(types.Int)
						if i > max {
							max = i
						}
					}
					return max
				})),
		),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}

	for _, tc := range dispatchTests {
		t.Run(fmt.Sprintf("Parse(%s)", tc.expr), func(t *testing.T) {
			testParse(t, e, tc.expr, tc.out)
		})
	}
	for _, tc := range dispatchTests {
		t.Run(fmt.Sprintf("Compile(%s)", tc.expr), func(t *testing.T) {
			testCompile(t, e, tc.expr, tc.out)
		})
	}
}

func TestFunctionDisableDeclaration(t *testing.T) {
	e, err := NewCustomEnv(
		Function("disabled",
			DisableDeclaration(true),
			Overload("disabled_any", []*Type{BoolType}, BoolType),
			SingletonFunctionImpl(func(args ...ref.Val) ref.Val {
				return types.True
			}),
		),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Parse("disabled(true)")
	if iss.Err() != nil {
		t.Errorf("Parse(disabled(true)) failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program(ast) failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Errorf("disabled runtime binding missing: %v", err)
	} else if out != types.True {
		t.Errorf("disabled runtime binding failed: %v", out)
	}
	_, iss = e.Compile("disabled(true)")
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "undeclared reference to 'disabled'") {
		t.Errorf("Compile(disabled(true)) got an unexpected error: %v", iss.Err())
	}
}

func TestFunctionDisableDeclarationMerge(t *testing.T) {
	e, err := NewCustomEnv(
		Function("disabled",
			Overload("disabled_any", []*Type{BoolType}, BoolType),
		),
		// Ensure the previously enabled declaration is disabled
		Function("disabled",
			DisableDeclaration(true),
			Overload("disabled_any", []*Type{BoolType}, BoolType,
				FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.True
				})),
		),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Parse("disabled(true)")
	if iss.Err() != nil {
		t.Errorf("Parse(disabled(true)) failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program(ast) failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Errorf("disabled runtime binding missing: %v", err)
	} else if out != types.True {
		t.Errorf("disabled runtime binding failed: %v", out)
	}
	_, iss = e.Compile("disabled(true)")
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "undeclared reference to 'disabled'") {
		t.Errorf("Compile(disabled(true)) got an unexpected error: %v", iss.Err())
	}
}

func TestFunctionDisableDeclarationMergeReenable(t *testing.T) {
	e, err := NewCustomEnv(
		Function("enabled",
			DisableDeclaration(true),
			Overload("enabled_any", []*Type{BoolType}, BoolType),
		),
		// Ensure the previously disabled declaration is enabled
		Function("enabled",
			DisableDeclaration(false),
			Overload("enabled_any", []*Type{BoolType}, BoolType,
				FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.True
				})),
		),
	)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Parse("enabled(true)")
	if iss.Err() != nil {
		t.Errorf("Parse(enabled(true)) failed: %v", iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		t.Fatalf("Program(ast) failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Errorf("enabled runtime binding missing: %v", err)
	} else if out != types.True {
		t.Errorf("enabled runtime binding failed: %v", out)
	}
	_, iss = e.Compile("enabled(true)")
	if iss.Err() != nil {
		t.Errorf("Compile(enabled(true)) got an unexpected error: %v", iss.Err())
	}
}

func TestExprDeclToDeclaration(t *testing.T) {
	paramT := chkdecls.NewTypeParamType("T")
	eq, err := ExprDeclToDeclaration(
		chkdecls.NewFunction(operators.Equals,
			chkdecls.NewParameterizedOverload(overloads.Equals,
				[]*exprpb.Type{paramT, paramT}, chkdecls.Bool, []string{"T"})),
	)
	if err != nil {
		t.Fatalf("ExprDeclToDeclaration(equals) failed: %v", err)
	}
	size, err := ExprDeclToDeclaration(
		chkdecls.NewFunction(overloads.Size,
			chkdecls.NewOverload(overloads.SizeString,
				[]*exprpb.Type{chkdecls.String}, chkdecls.Int),
			chkdecls.NewInstanceOverload(overloads.SizeStringInst,
				[]*exprpb.Type{chkdecls.String}, chkdecls.Int)),
	)
	if err != nil {
		t.Fatalf("ExprDeclToDeclaration(size) failed: %v", err)
	}
	x, err := ExprDeclToDeclaration(chkdecls.NewVar("x", chkdecls.String))
	if err != nil {
		t.Fatalf("ExprDeclToDeclaration(x) failed: %v", err)
	}
	constant, err := ExprDeclToDeclaration(
		chkdecls.NewConst("constant", chkdecls.Bool, &exprpb.Constant{
			ConstantKind: &exprpb.Constant_BoolValue{BoolValue: true},
		}))
	if err != nil {
		t.Fatalf("ExprDeclToDeclaration(constant) failed: %v", err)
	}
	e, err := NewCustomEnv(size, eq, x, constant)
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	ast, iss := e.Compile("(size(x) == x.size()) == constant")
	if iss.Err() != nil {
		t.Fatalf("Compile((size(x) == x.size()) == constant) failed: %v", iss.Err())
	}
	prg, err := e.Program(ast, Functions(&functions.Overload{
		Operator: overloads.SizeString,
		Unary: func(arg ref.Val) ref.Val {
			str, ok := arg.(types.String)
			if !ok {
				return types.MaybeNoSuchOverloadErr(arg)
			}
			return types.Int(len([]rune(string(str))))
		},
	}, &functions.Overload{
		Operator: overloads.SizeStringInst,
		Unary: func(arg ref.Val) ref.Val {
			str, ok := arg.(types.String)
			if !ok {
				return types.MaybeNoSuchOverloadErr(arg)
			}
			return types.Int(len([]rune(string(str))))
		},
	}))
	if err != nil {
		t.Fatalf("Program(ast) failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"x": "hello"})
	if err != nil {
		t.Fatalf("prg.Eval(x=hello) failed: %v", err)
	}
	if out != types.True {
		t.Errorf("prg.Eval((size(x) == x.size()) == constant) got %v, wanted true", out)
	}
}

func TestExprDeclToDeclarationInvalid(t *testing.T) {
	tests := []struct {
		in  *exprpb.Decl
		out string
	}{
		{
			in:  &exprpb.Decl{},
			out: "unsupported decl",
		},
		{
			in: &exprpb.Decl{
				Name: "bad_var",
				DeclKind: &exprpb.Decl_Ident{
					Ident: &exprpb.Decl_IdentDecl{
						Type: chkdecls.NewListType(&exprpb.Type{}),
					},
				},
			},
			out: "unsupported type",
		},
		{
			in: &exprpb.Decl{
				Name: "bad_var",
				DeclKind: &exprpb.Decl_Ident{
					Ident: &exprpb.Decl_IdentDecl{
						Type:  chkdecls.Bool,
						Value: &exprpb.Constant{},
					},
				},
			},
			out: "unsupported constant",
		},
		{
			in: &exprpb.Decl{
				Name: "bad_func_return",
				DeclKind: &exprpb.Decl_Function{
					Function: &exprpb.Decl_FunctionDecl{
						Overloads: []*exprpb.Decl_FunctionDecl_Overload{
							{
								OverloadId: "bad_overload",
								ResultType: &exprpb.Type{},
							},
						},
					},
				},
			},
			out: "unsupported type",
		},
		{
			in: &exprpb.Decl{
				Name: "bad_func_arg",
				DeclKind: &exprpb.Decl_Function{
					Function: &exprpb.Decl_FunctionDecl{
						Overloads: []*exprpb.Decl_FunctionDecl_Overload{
							{
								OverloadId: "bad_overload",
								Params:     []*exprpb.Type{{}},
								ResultType: chkdecls.Dyn,
							},
						},
					},
				},
			},
			out: "unsupported type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.in.String(), func(t *testing.T) {
			_, err := ExprDeclToDeclaration(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.out) {
				t.Fatalf("ExprDeclToDeclarations(%v) got %v, wanted error %v", tc.in, err, tc.out)
			}
		})
	}
}

func TestFunctionDeclExcludeOverloads(t *testing.T) {
	funcs := []*decls.FunctionDecl{}
	for _, fn := range stdlib.Functions() {
		if fn.Name() == operators.Add {
			fn = fn.Subset(ExcludeOverloads(overloads.AddList, overloads.AddBytes, overloads.AddString))
		}
		funcs = append(funcs, fn)
	}
	env, err := NewCustomEnv(FunctionDecls(funcs...))
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}

	successTests := []struct {
		name string
		expr string
		want ref.Val
	}{
		{
			name: "ints",
			expr: "1 + 1",
			want: types.Int(2),
		},
		{
			name: "doubles",
			expr: "1.5 + 1.5",
			want: types.Double(3.0),
		},
		{
			name: "uints",
			expr: "1u + 2u",
			want: types.Uint(3),
		},
		{
			name: "timestamp plus duration",
			expr: "timestamp('2001-01-01T00:00:00Z') + duration('1h') == timestamp('2001-01-01T01:00:00Z')",
			want: types.True,
		},
		{
			name: "durations",
			expr: "duration('1h') + duration('1m') == duration('1h1m')",
			want: types.True,
		},
	}
	for _, tst := range successTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() errored: %v", err)
			}
			if out.Equal(tc.want) != types.True {
				t.Errorf("Eval() got %v, wanted %v", out, tc.want)
			}
		})
	}
	failureTests := []struct {
		name string
		expr string
	}{
		{
			name: "strings",
			expr: "'a' + 'b'",
		},
		{
			name: "bytes",
			expr: "b'123' + b'456'",
		},
		{
			name: "lists",
			expr: "[1] + [2, 3]",
		},
	}
	for _, tst := range failureTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if iss.Err() == nil {
				t.Error("env.Compile() got ast, wanted error")
			}
		})
	}
}

func TestFunctionDeclIncludeOverloads(t *testing.T) {
	funcs := []*decls.FunctionDecl{}
	for _, fn := range stdlib.Functions() {
		if fn.Name() == operators.Add {
			fn = fn.Subset(IncludeOverloads(overloads.AddInt64, overloads.AddDouble))
		}
		funcs = append(funcs, fn)
	}
	env, err := NewCustomEnv(FunctionDecls(funcs...))
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}

	successTests := []struct {
		name string
		expr string
		want ref.Val
	}{
		{
			name: "ints",
			expr: "1 + 1",
			want: types.Int(2),
		},
		{
			name: "doubles",
			expr: "1.5 + 1.5",
			want: types.Double(3.0),
		},
	}
	for _, tst := range successTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() errored: %v", err)
			}
			if out.Equal(tc.want) != types.True {
				t.Errorf("Eval() got %v, wanted %v", out, tc.want)
			}
		})
	}
	failureTests := []struct {
		name string
		expr string
	}{
		{
			name: "strings",
			expr: "'a' + 'b'",
		},
		{
			name: "bytes",
			expr: "b'123' + b'456'",
		},
		{
			name: "lists",
			expr: "[1] + [2, 3]",
		},
		{
			name: "uints",
			expr: "1u + 2u",
		},
		{
			name: "timestamp plus duration",
			expr: "timestamp('2001-01-01T00:00:00Z') + duration('1h') == timestamp('2001-01-01T01:00:00Z')",
		},
		{
			name: "durations",
			expr: "duration('1h') + duration('1m') == duration('1h1m')",
		},
	}
	for _, tst := range failureTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if iss.Err() == nil {
				t.Error("env.Compile() got ast, wanted error")
			}
		})
	}
}

func testParse(t testing.TB, env *Env, expr string, want any) {
	t.Helper()
	ast, iss := env.Parse(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Parse(%s) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"err": types.NewErr("error argument"), "unk": types.NewUnknown(42, nil)})
	switch want := want.(type) {
	case *types.Unknown:
		if !want.Contains(out.(*types.Unknown)) {
			t.Errorf("prg.Eval() got %v, wanted %v", out, want)
		}
	case ref.Val:
		if want.Equal(out) != types.True {
			t.Errorf("prg.Eval() got %v, wanted %v", out, want)
		}
	case error:
		if err == nil || want.Error() != err.Error() {
			t.Errorf("prg.Eval() got error '%v', wanted '%v'", err, want)
		}
	}
}

func testCompile(t testing.TB, env *Env, expr string, want any) {
	t.Helper()
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"err": types.NewErr("error argument"), "unk": types.NewUnknown(42, nil)})
	switch want := want.(type) {
	case *types.Unknown:
		if !want.Contains(out.(*types.Unknown)) {
			t.Errorf("prg.Eval() got %v, wanted %v", out, want)
		}
	case ref.Val:
		if want.Equal(out) != types.True {
			t.Errorf("prg.Eval() got %v, wanted %v", out, want)
		}
	case error:
		if err == nil || want.Error() != err.Error() {
			t.Errorf("prg.Eval() got error '%v', wanted '%v'", err, want)
		}
	}
}
