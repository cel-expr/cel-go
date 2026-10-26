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

package ext

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/test"

	proto2pb "cel.dev/cel-go/test/proto2pb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

func TestProtos(t *testing.T) {
	var protosTests = []struct {
		expr string
	}{
		// Positive test cases.
		{expr: `proto.getExt(ExampleType{}, google.expr.proto2.test.int32_ext) == 0`},
		{expr: `!proto.hasExt(ExampleType{}, google.expr.proto2.test.int32_wrapper_ext)`},
		{expr: `proto.getExt(ExampleType{}, google.expr.proto2.test.int32_wrapper_ext) == null`},
		{expr: `!proto.hasExt(ExampleType{}, google.expr.proto2.test.nested_example)`},
		{expr: `proto.getExt(ExampleType{}, google.expr.proto2.test.nested_example) == ExampleType{}`},
		{expr: `proto.getExt(ExampleType{}, google.expr.proto2.test.ExtendedExampleType.extended_examples) == []`},
		{expr: `proto.getExt(ExampleType{}, google.expr.proto2.test.ExtendedExampleType.enum_ext) == GlobalEnum.GOO`},
		{expr: "ExampleType{`in`: 64}.`in` == 64"},
		// TODO(issue/1095): legal parse, but can't assign extension fields without updates to type checker and runtime
		// ExampleType{`google.expr.proto2.test.int32_ext`: 42}.`google.expr.proto2.test.int32_ext` == 42
		{expr: `proto.getExt(msg, google.expr.proto2.test.int32_ext) == 42`},
		{expr: "msg.`google.expr.proto2.test.int32_ext` == 42"},
		{expr: `proto.getExt(msg, google.expr.proto2.test.int32_wrapper_ext) == 21`},
		{expr: "msg.`google.expr.proto2.test.int32_wrapper_ext` == 21"},
		{expr: `proto.hasExt(msg, google.expr.proto2.test.nested_example)`},
		{expr: "has(msg.`google.expr.proto2.test.nested_example`)"},
		{expr: `proto.getExt(msg, google.expr.proto2.test.nested_example) == ExampleType{name: 'nested'}`},
		{expr: "msg.`google.expr.proto2.test.nested_example` == ExampleType{name: 'nested'}"},
		{expr: `proto.getExt(msg, google.expr.proto2.test.ExtendedExampleType.extended_examples) == ['example1', 'example2']`},
		{expr: "msg.`google.expr.proto2.test.ExtendedExampleType.extended_examples` == ['example1', 'example2']"},
		{expr: `proto.getExt(msg, google.expr.proto2.test.ExtendedExampleType.enum_ext) == GlobalEnum.GAZ`},
		{expr: "msg.`google.expr.proto2.test.ExtendedExampleType.enum_ext` == GlobalEnum.GAZ"},
	}

	env := testProtosEnv(t)
	for i, tst := range protosTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatal(err)
				}
				out, _, err := prg.Eval(map[string]any{"msg": msgWithExtensions()})
				if err != nil {
					t.Fatal(err)
				}
				if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestProtosNonMatch(t *testing.T) {
	var protosTests = []struct {
		expr string
	}{
		// Even though 'getExt' is the macro, the call is left unexpanded is an identifier
		// that's not named 'proto'
		{
			expr: `msg.getExt("google.expr.proto2.test.int32_ext", 0) == 42`,
		},
		// Even though 'getExt' is the macro, the call is left unexpanded as the operand is a call
		{
			expr: `dyn(msg).getExt("google.expr.proto2.test.int32_ext", 0) == 42`,
		},
		// Test for hasExt for completeness
		{
			expr: `msg.hasExt("google.expr.proto2.test.int32_ext", 0)`,
		},
	}
	env := testProtosEnv(t,
		cel.Function("getExt",
			cel.MemberOverload("msg_getExt_field_default",
				[]*cel.Type{cel.DynType, cel.StringType, cel.DynType},
				cel.DynType),
			cel.SingletonFunctionBinding(func(args ...ref.Val) ref.Val {
				msg := args[0]
				field := args[1]
				indexer := msg.(traits.Indexer)
				return indexer.Get(field)
			})),
		cel.Function("hasExt",
			cel.MemberOverload("msg_hasExt_field_any",
				[]*cel.Type{cel.DynType, cel.StringType, cel.DynType},
				cel.BoolType),
			cel.SingletonFunctionBinding(func(args ...ref.Val) ref.Val {
				return types.True
			})))
	for i, tst := range protosTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, cAst)

			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					t.Fatal(err)
				}
				out, _, err := prg.Eval(map[string]any{"msg": msgWithExtensions()})
				if err != nil {
					t.Fatal(err)
				}
				if out.Value() != true {
					t.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestProtosParseErrors(t *testing.T) {
	var protosTests = []struct {
		expr string
		err  string
	}{
		{
			expr: `proto.getExt(ExtendedExampleType{}, enum_ext)`,
			err: `ERROR: <input>:1:37: invalid extension field
		| proto.getExt(ExtendedExampleType{}, enum_ext)
		| ....................................^`,
		},
		{
			expr: `proto.hasExt(ExtendedExampleType{}, call().enum_ext)`,
			err: `ERROR: <input>:1:43: invalid extension field
		| proto.hasExt(ExtendedExampleType{}, call().enum_ext)
		| ..........................................^`,
		},
		{
			expr: `proto.getExt(ExtendedExampleType{}, has(google.expr.proto2.test.int32_ext))`,
			err: `ERROR: <input>:1:40: invalid extension field
		| proto.getExt(ExtendedExampleType{}, has(google.expr.proto2.test.int32_ext))
		| .......................................^`,
		},
		{
			expr: `ExampleType{}.in`,
			err: `ERROR: <input>:1:15: Syntax error: no viable alternative at input '.in'
			| ExampleType{}.in
			| ..............^
		   ERROR: <input>:1:17: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
			| ExampleType{}.in
			| ................^`,
		},
	}
	env := testProtosEnv(t)
	for i, tst := range protosTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			ast, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				if !test.Compare(iss.Err().Error(), tc.err) {
					t.Errorf("got parse error %v, wanted error %s for expr: %s", iss.Err(), tc.err, tc.expr)
				}
			} else {
				t.Fatalf("got %v, wanted parse error", ast)
			}
		})
	}
}

// testProtosEnv initializes the test environment common to all tests.
func testProtosEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	baseOpts := []cel.EnvOption{
		cel.Container("google.expr.proto2.test"),
		cel.Types(&proto2pb.ExampleType{}, &proto2pb.ExternalMessageType{}),
		cel.Variable("msg", cel.ObjectType("google.expr.proto2.test.ExampleType")),
		cel.EnableIdentifierEscapeSyntax(),
		Protos(),
	}
	env, err := cel.NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv(Protos()) failed: %v", err)
	}
	return env
}

func TestProtosWithExtension(t *testing.T) {
	env := testProtosEnv(t)
	_, err := env.Extend(Protos())
	if err != nil {
		t.Fatalf("env.Extend(Protos()) failed: %v", err)
	}
	_, iss := env.Compile("proto.getExt(ExampleType{}, google.expr.proto2.test.int32_ext) == 0")
	if iss.Err() != nil {
		t.Errorf("env.Compile() failed: %v", iss.Err())
	}
}

func TestProtosVersion(t *testing.T) {
	_, err := cel.NewEnv(Protos(ProtosVersion(0)))
	if err != nil {
		t.Fatalf("ProtosVersion(0) failed: %v", err)
	}
}

// msgWithExtensions generates a new example message with all possible extensions set.
func msgWithExtensions() *proto2pb.ExampleType {
	msg := &proto2pb.ExampleType{
		Name: new("example0"),
	}
	proto.SetExtension(msg, proto2pb.E_Int32Ext, int32(42))
	proto.SetExtension(msg, proto2pb.E_Int32WrapperExt, wrapperspb.Int32(21))
	proto.SetExtension(msg, proto2pb.E_NestedExample, &proto2pb.ExampleType{Name: new("nested")})
	proto.SetExtension(msg, proto2pb.E_ExtendedExampleType_EnumExt, proto2pb.GlobalEnum_GAZ)
	proto.SetExtension(msg, proto2pb.E_ExtendedExampleType_ExtendedExamples, []string{"example1", "example2"})
	return msg
}
