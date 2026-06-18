// Copyright 2019 Google LLC
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/google/cel-go/checker"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter"
	"github.com/google/cel-go/parser"
	"github.com/google/cel-go/test"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	dynamicpb "google.golang.org/protobuf/types/dynamicpb"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"

	proto2pb "github.com/google/cel-go/test/proto2pb"
	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func Test_ExampleWithBuiltins(t *testing.T) {
	// Variables used within this expression environment.
	env, err := NewEnv(
		Variable("i", StringType),
		Variable("you", StringType),
	)
	if err != nil {
		t.Fatalf("environment creation error: %s\n", err)
	}

	// Compile the expression.
	ast, iss := env.Compile(`"Hello " + you + "! I'm " + i + "."`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}

	// Create the program, and evaluate it against some input.
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}

	// If the Eval() call were provided with cel.EvalOptions(OptTrackState) the details response
	// (2nd return) would be non-nil.
	out, _, err := prg.Eval(
		map[string]any{
			"i":   "CEL",
			"you": "world",
		},
	)
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}

	// Hello world! I'm CEL.
	if out.Equal(types.String("Hello world! I'm CEL.")) != types.True {
		t.Errorf(`got '%v', wanted "Hello world! I'm CEL."`, out.Value())
	}
}

func TestEval(t *testing.T) {
	env, err := NewEnv(
		Variable("input", ListType(IntType)),
		CostEstimatorOptions(
			checker.OverloadCostEstimate(overloads.TimestampToYear, estimateTimestampToYear),
		),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	tests := []struct {
		expr string
		in   any
	}{
		{
			expr: `input.size() != 0`,
			in:   map[string]any{"input": []int{1, 2, 3}},
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%s) failed: %v", tc.expr, iss.Err())
			}
			ctx := context.Background()
			prgOpts := []ProgramOption{
				CostTracking(testRuntimeCostEstimator{}),
				CostTrackerOptions(
					interpreter.OverloadCostTracker(overloads.TimestampToYear, trackTimestampToYear),
				),
				EvalOptions(OptOptimize, OptTrackCost),
				InterruptCheckFrequency(100),
			}
			prg, err := env.Program(ast, prgOpts...)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			for k := 0; k < 100; k++ {
				t.Run(fmt.Sprintf("[%d]", k), func(t *testing.T) {
					t.Parallel()
					prg.Eval(tc.in)
					evalCtx, cancel := context.WithTimeout(ctx, time.Minute)
					defer cancel()
					_, _, err := prg.ContextEval(evalCtx, tc.in)
					if err != nil {
						t.Fatalf("prg.ContextEval() failed: %v", err)
					}
				})
			}
		})
	}
}

func TestAbbrevsCompiled(t *testing.T) {
	// Test whether abbreviations successfully resolve at type-check time (compile time).
	env := testEnv(t,
		Abbrevs("qualified.identifier.name"),
		Variable("qualified.identifier.name.first", StringType),
	)
	prg := compile(t, env, `"hello "+ name.first`) // abbreviation resolved here.
	out, _, err := prg.Eval(
		map[string]any{
			"qualified.identifier.name.first": "Jim",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value() != "hello Jim" {
		t.Errorf("got %v, wanted 'hello Jim'", out)
	}
}

func TestAbbrevsParsed(t *testing.T) {
	// Test whether abbreviations are resolved properly at evaluation time.
	env := testEnv(t,
		Abbrevs("qualified.identifier.name"),
	)
	ast, iss := env.Parse(`"hello " + name.first`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := env.Program(ast) // abbreviation resolved here.
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := prg.Eval(
		map[string]any{
			"qualified.identifier.name": map[string]string{
				"first": "Jim",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value() != "hello Jim" {
		t.Errorf("got %v, wanted 'hello Jim'", out)
	}
}

func TestAbbrevsDisambiguation(t *testing.T) {
	env := testEnv(t,
		Abbrevs("external.Expr"),
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),

		Variable("test", BoolType),
		Variable("external.Expr", StringType),
	)
	// This expression will return either a string or a protobuf Expr value depending on the value
	// of the 'test' argument. The fully qualified type name is used indicate that the protobuf
	// typed 'Expr' should be used rather than the abbreviatation for 'external.Expr'.
	out, err := interpret(t, env, `test ? dyn(Expr) : google.api.expr.v1alpha1.Expr{id: 1}`,
		map[string]any{
			"test":          true,
			"external.Expr": "string expr",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value() != "string expr" {
		t.Errorf("got %v, wanted 'string expr'", out)
	}
	out, err = interpret(t, env, `test ? dyn(Expr) : google.api.expr.v1alpha1.Expr{id: 1}`,
		map[string]any{
			"test":          false,
			"external.Expr": "wrong expr",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := &exprpb.Expr{Id: 1}
	got, err := out.ConvertToNative(reflect.TypeOf(want))
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(got.(*exprpb.Expr), want) {
		t.Errorf("got %v, wanted '%v'", out, want)
	}
}

func TestConvertToNativeJSONStructure(t *testing.T) {
	env, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, issues := env.Compile(`{
		"parts": [{"kind": "text"}]
	}`)
	if issues != nil && issues.Err() != nil {
		t.Fatal(issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		t.Fatal(err)
	}

	result, _, err := prg.Eval(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}

	native, err := result.ConvertToNative(types.JSONValueType)
	if err != nil {
		t.Fatal(err)
	}

	jsonBytes, err := json.Marshal(native)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	want := `{"parts":[{"kind":"text"}]}`
	if string(jsonBytes) != want {
		t.Errorf("json.Marshal() failed, got : %s, wanted ", jsonBytes)
	}
}

func TestCustomEnvError(t *testing.T) {
	env, err := NewCustomEnv(StdLib(), StdLib())
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}
	_, iss := compileOrError(t, env, "a.b.c == true")
	if iss == nil {
		t.Error("got successful compile, expected error for duplicate function declarations.")
	}
}

func TestCustomEnv(t *testing.T) {
	env, err := NewCustomEnv(Variable("a.b.c", BoolType))
	if err != nil {
		t.Fatalf("NewCustomEnv(a.b.c:bool) failed: %v", err)
	}
	t.Run("err", func(t *testing.T) {
		_, iss := compileOrError(t, env, "a.b.c == true")
		if iss == nil {
			t.Error("got successful compile, expected error for missing operator '_==_'")
		}
	})

	t.Run("ok", func(t *testing.T) {
		out, err := interpret(t, env, "a.b.c", map[string]any{"a.b.c": true})
		if err != nil {
			t.Fatal(err)
		}
		if out != types.True {
			t.Errorf("got '%v', wanted 'true'", out.Value())
		}
	})
}

func TestCrossTypeNumericComparisons(t *testing.T) {
	tests := []struct {
		name string
		expr string
		iss  string
		opt  EnvOption
		out  ref.Val
	}{
		// Statically typed expressions need to opt in to cross-type numeric comparisons
		{
			name: "double_less_than_int_err",
			expr: `1.0 < 2`,
			opt:  CrossTypeNumericComparisons(false),
			iss: `
			ERROR: <input>:1:5: found no matching overload for '_<_' applied to '(double, int)'
             | 1.0 < 2
             | ....^`,
		},
		{
			name: "double_less_than_int_success",
			expr: `1.0 < 2`,
			opt:  CrossTypeNumericComparisons(true),
			out:  types.True,
		},
		// Dynamic data already benefits from cross-type numeric comparisons
		{
			name: "dyn_less_than_int_success",
			expr: `dyn(1.0) < 2`,
			opt:  CrossTypeNumericComparisons(false),
			out:  types.True,
		},
		{
			name: "dyn_less_than_int_success",
			expr: `dyn(1.0) < 2`,
			opt:  CrossTypeNumericComparisons(true),
			out:  types.True,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			e := testEnv(t, tc.opt)
			ast, iss := e.Compile(tc.expr)
			if tc.iss != "" {
				if iss.Err() == nil {
					t.Fatalf("e.Compile(%v) returned ast, expected error: %v", tc.expr, tc.iss)
				}
				if !test.Compare(iss.Err().Error(), tc.iss) {
					t.Fatalf("e.Compile(%v) returned %v, expected error: %v", tc.expr, iss.Err(), tc.iss)
				}
				return
			}
			if iss.Err() != nil {
				t.Fatalf("e.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			prg, err := e.Program(ast)
			if err != nil {
				t.Fatalf("e.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() errored: %v", err)
			}
			if out != tc.out {
				t.Errorf("program eval got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestExtendStdlibFunction(t *testing.T) {
	env := testEnv(t,
		Function(overloads.Contains,
			MemberOverload("bytes_contains_bytes", []*Type{BytesType, BytesType}, BoolType,
				BinaryBinding(func(bstr, bsub ref.Val) ref.Val {
					return types.Bool(bytes.Contains([]byte(bstr.(types.Bytes)), []byte(bsub.(types.Bytes))))
				}))),
	)
	prg := compile(t, env, `b'string'.contains(b'tri') && 'string'.contains('tri')`)
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Fatalf("contains check errored: %v", err)
	}
	if out != types.True {
		t.Errorf("contains check got %v, wanted true", out)
	}
}

func TestSubsetStdLib(t *testing.T) {
	env, err := NewCustomEnv(
		StdLib(StdLibSubset(
			&env.LibrarySubset{
				IncludeMacros: []string{"has"},
				IncludeFunctions: []*env.Function{
					{Name: operators.Equals},
					{Name: operators.NotEquals},
					{Name: operators.LogicalAnd},
					{Name: operators.LogicalOr},
					{Name: operators.LogicalNot},
					{Name: overloads.Size, Overloads: []*env.Overload{{ID: "list_size"}}},
				},
			},
		)))
	if err != nil {
		t.Fatalf("StdLib() subsetting failed: %v", err)
	}
	tests := []struct {
		name     string
		expr     string
		compiles bool
		want     ref.Val
	}{
		{
			name:     "has macro",
			expr:     "!has({}.a)",
			compiles: true,
			want:     types.True,
		},
		{
			name:     "not equals",
			expr:     "has({}.a) != true",
			compiles: true,
			want:     types.True,
		},
		{
			name:     "logical operators",
			expr:     "has({}.a) != true && has({'b': 1}.b) == true",
			compiles: true,
			want:     types.True,
		},
		{
			name:     "list size - allowed",
			expr:     "[1, 2, 3].size()",
			compiles: true,
			want:     types.Int(3),
		},
		{
			name:     "excluded macro",
			expr:     "[1, 2, 3].exists(i, i != 0)",
			compiles: false,
		},
		{
			name:     "string size - not allowed",
			expr:     "'hello'.size()",
			compiles: false,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if tc.compiles && iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			if !tc.compiles && iss.Err() != nil {
				return
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %s", err)
			}
			if out.Equal(tc.want) != types.True {
				t.Errorf("prg.Eval() got %v, wanted %v", out, tc.want)
			}
		})
	}
}

func TestSubsetStdLibError(t *testing.T) {
	_, err := NewCustomEnv(
		StdLib(StdLibSubset(
			env.NewLibrarySubset().AddIncludedMacros("has").AddExcludedMacros("exists")),
		))
	if err == nil || !strings.Contains(err.Error(), "invalid subset") {
		t.Errorf("StdLib() subsetting got %v, wanted error 'invalid subset'", err)
	}
}

func TestSubsetStdLibMerge(t *testing.T) {
	_, err := NewCustomEnv(
		Function("size", MemberOverload("string_size", []*Type{StringType}, IntType)),
		StdLib(StdLibSubset(
			env.NewLibrarySubset().AddIncludedFunctions([]*env.Function{
				{Name: overloads.Size, Overloads: []*env.Overload{{ID: "string_size"}}},
			}...),
		)))
	if err != nil {
		t.Errorf("StdLib() subsetting failed to merge: %v", err)
	}
}

func TestSubsetStdLibMergeError(t *testing.T) {
	_, err := NewCustomEnv(
		Function("size", MemberOverload("string_size", []*Type{StringType}, UintType)),
		StdLib(StdLibSubset(
			env.NewLibrarySubset().AddIncludedFunctions([]*env.Function{
				{Name: overloads.Size, Overloads: []*env.Overload{{ID: "string_size"}}},
			}...),
		)))
	if err == nil || !strings.Contains(err.Error(), "merge failed") {
		t.Errorf("StdLib() subsetting got %v, wanted error 'merge failed'", err)
	}
}

func TestCustomTypes(t *testing.T) {
	reg := types.NewEmptyRegistry()
	env := testEnv(t,
		CustomTypeAdapter(reg),
		CustomTypeProvider(reg),
		Container("google.api.expr.v1alpha1"),
		Types(
			&exprpb.Expr{},
			types.BoolType,
			types.IntType,
			types.StringType),
		Variable("expr", ObjectType("google.api.expr.v1alpha1.Expr")),
	)
	ast, iss := env.Compile(`
		expr == Expr{id: 2,
			call_expr: Expr.Call{
				function: "_==_",
				args: [
					Expr{id: 1, ident_expr: Expr.Ident{ name: "a" }},
					Expr{id: 3, ident_expr: Expr.Ident{ name: "b" }}]
			}}`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	if ast.OutputType() != BoolType {
		t.Fatalf("got %v, wanted type bool", ast.OutputType())
	}
	prg, _ := env.Program(ast)
	vars := map[string]any{"expr": &exprpb.Expr{
		Id: 2,
		ExprKind: &exprpb.Expr_CallExpr{
			CallExpr: &exprpb.Expr_Call{
				Function: "_==_",
				Args: []*exprpb.Expr{
					{
						Id: 1,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "a"},
						},
					},
					{
						Id: 3,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "b"},
						},
					},
				},
			},
		},
	}}
	out, _, _ := prg.Eval(vars)
	if out != types.True {
		t.Errorf("got '%v', wanted 'true'", out.Value())
	}
}

func TestTypeIsolation(t *testing.T) {
	b, err := os.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatal("can't read fds file: ", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatal("can't unmarshal descriptor data: ", err)
	}

	env := testEnv(t,
		TypeDescs(&fds),
		Variable("myteam", ObjectType("cel.testdata.Team")),
	)
	src := "myteam.members[0].name == 'Cyclops'"
	compile(t, env, src)

	// Ensure that isolated types don't leak through.
	e2 := testEnv(t, Variable("myteam", ObjectType("cel.testdata.Team")))
	_, iss := compileOrError(t, e2, src)
	if iss == nil {
		t.Errorf("wanted compile failure for unknown message.")
	}
}

func TestDynamicProto(t *testing.T) {
	b, err := os.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatalf("os.ReadFile() failed: %v", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatalf("proto.Unmarshal() failed: %v", err)
	}
	files := (&fds).GetFile()
	fileCopy := make([]any, len(files))
	for i := 0; i < len(files); i++ {
		fileCopy[i] = files[i]
	}
	pbFiles, err := protodesc.NewFiles(&fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	e := testEnv(t,
		Container("cel"),
		// The following is identical to registering the FileDescriptorSet;
		// however, it tests a different code path which aggregates individual
		// FileDescriptorProto values together.
		TypeDescs(fileCopy...),
		// Additionally, demonstrate that double registration of files doesn't
		// cause any problems.
		TypeDescs(pbFiles),
	)
	src := `testdata.Team{name: 'X-Men', members: [
		testdata.Mutant{name: 'Jean Grey', level: 20},
		testdata.Mutant{name: 'Cyclops', level: 7},
		testdata.Mutant{name: 'Storm', level: 7},
		testdata.Mutant{name: 'Wolverine', level: 11}
	]}`
	ast, iss := e.Compile(src)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %v", src, iss.Err())
	}
	prg, err := e.Program(ast, EvalOptions(OptOptimize))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	obj, ok := out.(traits.Indexer)
	if !ok {
		t.Fatalf("unable to convert output to object: %v", out)
	}
	if obj.Get(types.String("name")).Equal(types.String("X-Men")) == types.False {
		t.Fatalf("got field 'name' %v, wanted X-Men", obj.Get(types.String("name")))
	}
}

func TestDynamicProtoFileDescriptors(t *testing.T) {
	b, err := os.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatalf("os.ReadFile() failed: %v", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatalf("proto.Unmarshal() failed: %v", err)
	}
	files := (&fds).GetFile()
	fileCopy := make([]any, len(files))
	for i := 0; i < len(files); i++ {
		fileCopy[i] = files[i]
	}
	pbFiles, err := protodesc.NewFiles(&fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	desc, err := pbFiles.FindDescriptorByName("cel.testdata.Mutant")
	if err != nil {
		t.Fatalf("pbFiles.FindDescriptorByName() could not find Mutant: %v", err)
	}
	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("desc not convertible to MessageDescriptor: %T", desc)
	}
	wolverine := dynamicpb.NewMessage(msgDesc)
	wolverine.ProtoReflect().Set(msgDesc.Fields().ByName("name"), protoreflect.ValueOfString("Wolverine"))
	env := testEnv(t,
		// The following is identical to registering the FileDescriptorSet;
		// however, it tests a different code path which aggregates individual
		// FileDescriptorProto values together.
		TypeDescs(fileCopy...),
		Variable("mutant", ObjectType("cel.testdata.Mutant")),
	)
	src := `has(mutant.name) && mutant.name == 'Wolverine'`
	ast, iss := env.Compile(src)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %v", src, iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{
		"mutant": wolverine,
	})
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	obj, ok := out.(types.Bool)
	if !ok {
		t.Fatalf("unable to convert output to object: %v", out)
	}
	if obj != types.True {
		t.Errorf("got %v, wanted true", out)
	}
}

func TestGlobalVars(t *testing.T) {
	env := testEnv(t,
		Variable("attrs", MapType(StringType, DynType)),
		Variable("default", DynType),
		Function("get",
			MemberOverload("get_map", []*Type{MapType(StringType, DynType), StringType, DynType}, DynType,
				FunctionBinding(func(args ...ref.Val) ref.Val {
					attrs, ok := args[0].(traits.Mapper)
					if !ok {
						return types.NewErr(
							"invalid operand of type '%v' to obj.get(key, def)",
							args[0].Type())
					}
					key := args[1]
					defVal := args[2]
					if attrs.Contains(key) == types.True {
						return attrs.Get(key)
					}
					return defVal
				}),
			),
		),
	)
	ast, iss := env.Compile(`attrs.get("first", attrs.get("second", default))`)
	if iss.Err() != nil {
		t.Fatalf("e.Parse() failed: %v", iss.Err())
	}

	// Global variables can be configured as a ProgramOption and optionally overridden on Eval.
	// Add a previous globals map to confirm the order of shadowing and a final empty global
	// map to show that globals are not clobbered.
	prg, err := env.Program(ast,
		Globals(map[string]any{
			"default": "shadow me",
		}),
		Globals(map[string]any{
			"default": "third",
		}),
		Globals(map[string]any{}),
	)
	if err != nil {
		t.Fatalf("e.Program() failed: %v", err)
	}

	t.Run("bad_attrs", func(t *testing.T) {
		out, _, err := prg.Eval(map[string]any{
			"attrs": []string{"one", "two"},
		})
		if err == nil {
			t.Errorf("prg.Eval() of incorrect arg type invoked function, wanted error, got %v", out)
		}
	})

	t.Run("global_default", func(t *testing.T) {
		vars := map[string]any{
			"attrs": map[string]any{},
		}
		out, _, err := prg.Eval(vars)
		if err != nil {
			t.Fatalf("prg.Eval() failed: %v", err)
		}
		if out.Equal(types.String("third")) != types.True {
			t.Errorf("got '%v', expected 'third'.", out.Value())
		}
	})

	t.Run("attrs_alt", func(t *testing.T) {
		vars := map[string]any{
			"attrs": map[string]any{"second": "yep"}}
		out, _, err := prg.Eval(vars)
		if err != nil {
			t.Fatalf("prg.Eval(vars) failed: %v", err)
		}
		if out.Equal(types.String("yep")) != types.True {
			t.Errorf("got '%v', expected 'yep'.", out.Value())
		}
	})

	t.Run("local_default", func(t *testing.T) {
		vars := map[string]any{
			"attrs":   map[string]any{},
			"default": "fourth"}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("fourth")) != types.True {
			t.Errorf("got '%v', expected 'fourth'.", out.Value())
		}
	})
}

func TestMacroSubset(t *testing.T) {
	// Only enable the 'has' macro rather than all parser macros.
	env := testEnv(t,
		ClearMacros(), Macros(HasMacro),
		Variable("name", MapType(StringType, StringType)),
	)
	out, err := interpret(t, env, `has(name.first)`,
		map[string]any{
			"name": map[string]string{
				"first": "Jim",
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	if out != types.True {
		t.Errorf("got %v, wanted true", out)
	}
	out, err = interpret(t, env, `[1, 2].all(i, i > 0)`, NoVars())
	if err == nil {
		t.Errorf("got %v, wanted err", out)
	}
}

func TestCustomMacro(t *testing.T) {
	joinMacro := NewReceiverMacro("join", 1,
		func(meh MacroExprHelper, iterRange *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
			delim := args[0]
			iterIdent := meh.Ident("__iter__")
			accuIdent := meh.AccuIdent()
			accuInit := meh.LiteralString("")
			condition := meh.LiteralBool(true)
			step := meh.GlobalCall(
				// __result__.size() > 0 ? __result__  + delim + __iter__ : __iter__
				operators.Conditional,
				meh.GlobalCall(operators.Greater, meh.ReceiverCall("size", accuIdent), meh.LiteralInt(0)),
				meh.GlobalCall(operators.Add, meh.GlobalCall(operators.Add, accuIdent, delim), iterIdent),
				iterIdent)
			return meh.Fold(
				"__iter__",
				iterRange,
				accuIdent.GetIdentExpr().GetName(),
				accuInit,
				condition,
				step,
				accuIdent), nil
		})
	env := testEnv(t, Macros(joinMacro))
	ast, iss := env.Compile(`['hello', 'cel', 'friend'].join(',')`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Fatal(err)
	}
	if out.Equal(types.String("hello,cel,friend")) != types.True {
		t.Errorf("got %v, wanted 'hello,cel,friend'", out)
	}
}

func TestMacroInterop(t *testing.T) {
	existsOneMacro := NewReceiverMacro("exists_one", 2,
		func(meh MacroExprHelper, iterRange *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
			return ExistsOneMacroExpander(meh, iterRange, args)
		})
	transformMacro := NewReceiverMacro("transform", 2,
		func(meh MacroExprHelper, iterRange *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
			return MapMacroExpander(meh, iterRange, args)
		})
	filterMacro := NewReceiverMacro("filter", 2,
		func(meh MacroExprHelper, iterRange *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
			return FilterMacroExpander(meh, iterRange, args)
		})
	pairMacro := NewGlobalMacro("pair", 2,
		func(meh MacroExprHelper, iterRange *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
			return meh.NewMap(meh.NewMapEntry(args[0], args[1], false)), nil
		})
	getMacro := NewReceiverMacro("get", 2,
		func(meh MacroExprHelper, target *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
			return meh.GlobalCall(
				operators.Conditional,
				meh.PresenceTest(meh.Copy(target), args[0].GetIdentExpr().GetName()),
				meh.Select(meh.Copy(target), args[0].GetIdentExpr().GetName()),
				meh.Copy(args[1]),
			), nil
		})
	env := testEnv(t, Macros(existsOneMacro, transformMacro, filterMacro, pairMacro, getMacro))
	tests := []struct {
		expr string
		out  ref.Val
	}{
		{
			expr: `['tr', 's', 'fri'].filter(i, i.size() > 1).transform(i, i + 'end').exists_one(i, i == 'friend')`,
			out:  types.True,
		},
		{
			expr: `pair('a', 'b')`,
			out:  types.DefaultTypeAdapter.NativeToValue(map[string]string{"a": "b"}),
		},
		{
			expr: `{}.get(a, 'default')`,
			out:  types.String("default"),
		},
		{
			expr: `{'a': 'b'}.get(a, 'default')`,
			out:  types.String("b"),
		},
	}

	for _, tst := range tests {
		ast, iss := env.Compile(tst.expr)
		if iss.Err() != nil {
			t.Fatal(iss.Err())
		}
		prg, err := env.Program(ast, EvalOptions(OptExhaustiveEval))
		if err != nil {
			t.Fatalf("program creation error: %s\n", err)
		}
		out, _, err := prg.Eval(NoVars())
		if err != nil {
			t.Fatal(err)
		}
		if out.Equal(tst.out) != types.True {
			t.Errorf("got %v, wanted %v", out, tst.out)
		}
	}
}

func TestMacroModern(t *testing.T) {
	existsOneMacro := ReceiverMacro("exists_one", 2, parser.MakeExistsOne)
	transformMacro := ReceiverMacro("transform", 2, parser.MakeMap)
	filterMacro := ReceiverMacro("filter", 2, parser.MakeFilter)
	pairMacro := GlobalMacro("pair", 2,
		func(mef MacroExprFactory, iterRange celast.Expr, args []celast.Expr) (celast.Expr, *Error) {
			return mef.NewMap(mef.NewMapEntry(args[0], args[1], false)), nil
		})
	getMacro := ReceiverMacro("get", 2,
		func(mef MacroExprFactory, target celast.Expr, args []celast.Expr) (celast.Expr, *Error) {
			return mef.NewCall(
				operators.Conditional,
				mef.NewPresenceTest(mef.Copy(target), args[0].AsIdent()),
				mef.NewSelect(mef.Copy(target), args[0].AsIdent()),
				mef.Copy(args[1]),
			), nil
		})
	env := testEnv(t, Macros(existsOneMacro, transformMacro, filterMacro, pairMacro, getMacro))
	tests := []struct {
		expr string
		out  ref.Val
	}{
		{
			expr: `['tr', 's', 'fri'].filter(i, i.size() > 1).transform(i, i + 'end').exists_one(i, i == 'friend')`,
			out:  types.True,
		},
		{
			expr: `pair('a', 'b')`,
			out:  types.DefaultTypeAdapter.NativeToValue(map[string]string{"a": "b"}),
		},
		{
			expr: `{}.get(a, 'default')`,
			out:  types.String("default"),
		},
		{
			expr: `{'a': 'b'}.get(a, 'default')`,
			out:  types.String("b"),
		},
	}

	for _, tst := range tests {
		ast, iss := env.Compile(tst.expr)
		if iss.Err() != nil {
			t.Fatal(iss.Err())
		}
		prg, err := env.Program(ast, EvalOptions(OptExhaustiveEval))
		if err != nil {
			t.Fatalf("program creation error: %s\n", err)
		}
		out, _, err := prg.Eval(NoVars())
		if err != nil {
			t.Fatal(err)
		}
		if out.Equal(tst.out) != types.True {
			t.Errorf("got %v, wanted %v", out, tst.out)
		}
	}
}

func TestCustomExistsMacro(t *testing.T) {
	env := testEnv(t,
		Variable("attr", MapType(StringType, BoolType)),
		Macros(
			NewGlobalVarArgMacro("kleeneOr",
				func(meh MacroExprHelper, unused *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
					inputs := meh.NewList(args...)
					eqOne, err := ExistsMacroExpander(meh, inputs, []*exprpb.Expr{
						meh.Ident("__iter__"),
						meh.GlobalCall(operators.Equals, meh.Ident("__iter__"), meh.LiteralInt(1)),
					})
					if err != nil {
						return nil, err
					}
					eqZero, err := ExistsMacroExpander(meh, meh.Copy(inputs), []*exprpb.Expr{
						meh.Ident("__iter__"),
						meh.GlobalCall(operators.Equals, meh.Ident("__iter__"), meh.LiteralInt(0)),
					})
					if err != nil {
						return nil, err
					}
					return meh.GlobalCall(
						operators.Conditional,
						eqOne,
						meh.LiteralInt(1),
						meh.GlobalCall(
							operators.Conditional,
							eqZero,
							meh.LiteralInt(0),
							meh.LiteralInt(-1),
						),
					), nil
				},
			),
			NewGlobalMacro("kleeneEq", 2,
				func(meh MacroExprHelper, unused *exprpb.Expr, args []*exprpb.Expr) (*exprpb.Expr, *Error) {
					attr := args[0]
					value := args[1]
					hasAttr, err := HasMacroExpander(meh, nil, []*exprpb.Expr{meh.Copy(attr)})
					if err != nil {
						return nil, err
					}
					return meh.GlobalCall(
						operators.Conditional,
						meh.GlobalCall(operators.LogicalNot, hasAttr),
						meh.LiteralInt(0),
						meh.GlobalCall(
							operators.Conditional,
							meh.GlobalCall(operators.Equals, attr, value),
							meh.LiteralInt(1),
							meh.LiteralInt(-1),
						),
					), nil
				},
			),
		),
	)
	prg := compile(t, env, "kleeneOr(kleeneEq(attr.value, true), kleeneOr(0, 1, 1)) == 1")
	out, _, err := prg.Eval(map[string]any{"attr": map[string]bool{"value": false}})
	if err != nil {
		t.Errorf("prg.Eval() got %v, wanted non-error", err)
	}
	if out != types.True {
		t.Errorf("prg.Eval() got %v, wanted true", out)
	}
}

func TestAstIsChecked(t *testing.T) {
	env := testEnv(t)
	ast, iss := env.Compile("true")
	if iss.Err() != nil {
		t.Fatalf("e.Compile('true') failed: %v", iss.Err())
	}
	if !ast.IsChecked() {
		t.Error("got ast.IsChecked() 'false', wanted 'true'.")
	}
	ce, err := AstToCheckedExpr(ast)
	if err != nil {
		t.Fatalf("AstToCheckedExpr(%v) failed: %v", ast, err)
	}
	ast2 := CheckedExprToAst(ce)
	if !ast2.IsChecked() {
		t.Error("got ast2.IsChecked() 'false', wanted 'true'")
	}
	if !proto.Equal(ast.Expr(), ast2.Expr()) {
		t.Errorf("AST exprs did not roundtrip properly: ast1: %v, ast2: %v", ast, ast2)
	}
}

func TestExhaustiveEval(t *testing.T) {
	env := testEnv(t,
		Variable("k", StringType),
		Variable("v", BoolType),
	)
	ast, iss := env.Compile(`{k: true}[k] || v != false`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("env.Program() failed: %s\n", err)
	}
	out, details, err := prg.Eval(
		map[string]any{
			"k": "key",
			"v": true})
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}
	if out != types.True {
		t.Errorf("got '%v', expected 'true'", out.Value())
	}

	// Test to see whether 'v != false' was resolved to a value.
	// With short-circuiting it normally wouldn't be.
	s := details.State()
	lhsVal, found := s.Value(ast.Expr().GetCallExpr().GetArgs()[0].Id)
	if !found {
		t.Error("got not found, wanted evaluation of left hand side expression.")
		return
	}
	if lhsVal != types.True {
		t.Errorf("got '%v', expected 'true'", lhsVal)
	}
	rhsVal, found := s.Value(ast.Expr().GetCallExpr().GetArgs()[1].Id)
	if !found {
		t.Error("got not found, wanted evaluation of right hand side expression.")
		return
	}
	if rhsVal != types.True {
		t.Errorf("got '%v', expected 'true'", rhsVal)
	}
}

func TestContextEval(t *testing.T) {
	env := testEnv(t, Variable("items", ListType(IntType)))
	ast, iss := env.Compile("items.map(i, i * 2).filter(i, i >= 50).size()")
	if iss.Err() != nil {
		t.Fatalf("env.Compile(expr) failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize|OptTrackState), InterruptCheckFrequency(100))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	ctx := context.TODO()
	items := make([]int64, 2000)
	for i := int64(0); i < 2000; i++ {
		items[i] = i
	}
	out, _, err := prg.ContextEval(ctx, map[string]any{"items": items})
	if err != nil {
		t.Fatalf("prg.ContextEval() failed: %v", err)
	}
	if out != types.Int(1975) {
		t.Errorf("prg.ContextEval() got %v, wanted 1975", out)
	}

	evalCtx, cancel := context.WithTimeout(ctx, time.Microsecond)
	defer cancel()

	out, _, err = prg.ContextEval(evalCtx, map[string]any{"items": items})
	if err == nil {
		t.Errorf("Got result %v, wanted timeout error", out)
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Got %v, wanted context deadline exceeded", err)
	}
	if err != nil && !strings.Contains(err.Error(), "operation interrupted") {
		t.Errorf("Got %v, wanted error containing 'operation interrupted'", err)
	}
}

func TestContextEvalUnknowns(t *testing.T) {
	env, err := NewEnv(
		Variable("groups", ListType(IntType)),
		Variable("id", IntType),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	pvars, err := PartialVars(
		map[string]any{
			"groups": []int{1, 2, 3},
		},
		AttributePattern("id"),
	)
	if err != nil {
		t.Fatalf("PartialVars() failed: %v", err)
	}

	ast, iss := env.Compile(`groups.exists(t, t == id)`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	prg, err := env.Program(ast, EvalOptions(OptTrackState, OptPartialEval), InterruptCheckFrequency(100))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	out, _, err := prg.Eval(pvars)
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	ctxOut, _, err := prg.ContextEval(context.Background(), pvars)
	if err != nil {
		t.Fatalf("prg.ContextEval() failed: %v", err)
	}
	if !reflect.DeepEqual(out, ctxOut) {
		t.Errorf("got %v, wanted %v", out, ctxOut)
	}
}

func BenchmarkContextEval(b *testing.B) {
	env := testEnv(b,
		Variable("items", ListType(IntType)),
	)
	ast, iss := env.Compile("items.map(i, i * 2).filter(i, i >= 50).size()")
	if iss.Err() != nil {
		b.Fatalf("env.Compile(expr) failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize), InterruptCheckFrequency(200))
	if err != nil {
		b.Fatalf("env.Program() failed: %v", err)
	}

	ctx := context.TODO()
	items := make([]int64, 100)
	for i := int64(0); i < 100; i++ {
		items[i] = i
	}
	for i := 0; i < b.N; i++ {
		out, _, err := prg.ContextEval(ctx, map[string]any{"items": items})
		if err != nil {
			b.Fatalf("prg.ContextEval() failed: %v", err)
		}
		if out != types.Int(75) {
			b.Errorf("prg.ContextEval() got %v, wanted 75", out)
		}
	}
}

func TestEvalRecover(t *testing.T) {
	e := testEnv(t,
		Function("panic",
			Overload("global_panic", []*Type{}, BoolType,
				FunctionBinding(func(args ...ref.Val) ref.Val {
					panic("watch me recover")
				}),
			),
		),
	)
	// Test standard evaluation.
	pAst, iss := e.Parse("panic()")
	if iss.Err() != nil {
		t.Fatalf("e.Parse('panic()') failed: %v", iss.Err())
	}
	prgm, err := e.Program(pAst)
	if err != nil {
		t.Fatalf("e.Program(Ast) failed: %v", err)
	}
	_, _, err = prgm.Eval(map[string]any{})
	if err.Error() != "internal error: watch me recover" {
		t.Errorf("got '%v', wanted 'internal error: watch me recover'", err)
	}
	// Test the factory-based evaluation.
	prgm, _ = e.Program(pAst, EvalOptions(OptTrackState))
	_, _, err = prgm.Eval(map[string]any{})
	if err.Error() != "internal error: watch me recover" {
		t.Errorf("got '%v', wanted 'internal error: watch me recover'", err)
	}
}

func TestResidualAst(t *testing.T) {
	env := testEnv(t,
		Variable("x", IntType),
		Variable("y", IntType),
	)
	unkVars := env.UnknownVars()
	ast, iss := env.Parse(`x < 10 && (y == 0 || 'hello' != 'goodbye')`)
	if iss.Err() != nil {
		t.Fatalf("env.Parse() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, det, err := prg.Eval(unkVars)
	if !types.IsUnknown(out) {
		t.Fatalf("got %v, expected unknown", out)
	}
	if err != nil {
		t.Fatal(err)
	}
	residual, err := env.ResidualAst(ast, det)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := AstToString(residual)
	if err != nil {
		t.Fatal(err)
	}
	if expr != "x < 10" {
		t.Errorf("got expr: %s, wanted x < 10", expr)
	}
}

func TestResidualAstComplex(t *testing.T) {
	env := testEnv(t,
		Variable("resource.name", StringType),
		Variable("request.time", TimestampType),
		Variable("request.auth.claims", MapType(StringType, StringType)),
	)
	unkVars, _ := PartialVars(
		map[string]any{
			"resource.name": "bucket/my-bucket/objects/private",
			"request.auth.claims": map[string]string{
				"email_verified": "true",
			},
		},
		AttributePattern("request.auth.claims").QualString("email"),
	)
	ast, iss := env.Compile(
		`resource.name.startsWith("bucket/my-bucket") &&
		 bool(request.auth.claims.email_verified) == true &&
		 request.auth.claims.email == "wiley@acme.co"`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, det, err := prg.Eval(unkVars)
	if !types.IsUnknown(out) {
		t.Fatalf("got %v, expected unknown", out)
	}
	if err != nil {
		t.Fatal(err)
	}
	residual, err := env.ResidualAst(ast, det)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := AstToString(residual)
	if err != nil {
		t.Fatal(err)
	}
	if expr != `request.auth.claims.email == "wiley@acme.co"` {
		t.Errorf("got expr: %s, wanted request.auth.claims.email == \"wiley@acme.co\"", expr)
	}
}

func TestResidualAstMacros(t *testing.T) {
	tests := []struct {
		env      *Env
		in       map[string]any
		unks     []*interpreter.AttributePattern
		expr     string
		residual string
	}{
		{
			env: testEnv(t,
				Variable("x", ListType(IntType)),
				Variable("y", IntType),
				EnableMacroCallTracking()),
			in:       map[string]any{"y": 11},
			unks:     []*interpreter.AttributePattern{AttributePattern("x")},
			expr:     `x.exists(i, i < 10) && [11, 12, 13].all(i, i in [y, 12, 13])`,
			residual: `x.exists(i, i < 10)`,
		},
		{
			env: testEnv(t,
				Variable("bar", MapType(StringType, DynType)),
				Variable("foo", MapType(StringType, DynType)),
				EnableMacroCallTracking()),
			in: map[string]any{"foo": map[string]any{"a": "b"}},
			unks: []*interpreter.AttributePattern{
				AttributePattern("bar").QualString("baz").Wildcard(),
			},
			expr:     `foo.exists(t, t == bar.baz.x)`,
			residual: `{"a": "b"}.exists(t, t == bar.baz.x)`,
		},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			env := tc.env
			unkVars, err := PartialVars(tc.in, tc.unks...)
			if err != nil {
				t.Fatalf("PartialVars() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			prg, err := env.Program(ast, EvalOptions(OptTrackState, OptPartialEval))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, det, err := prg.Eval(unkVars)
			if !types.IsUnknown(out) {
				t.Fatalf("got %v, expected unknown", out)
			}
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			residual, err := env.ResidualAst(ast, det)
			if err != nil {
				t.Fatalf("env.ResidualAst() failed: %v", err)
			}
			expr, err := AstToString(residual)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if expr != tc.residual {
				t.Errorf("got expr: %s, wanted %s", expr, tc.residual)
			}
		})
	}
}

func TestResidualAstNil(t *testing.T) {
	env := testEnv(t)
	ast, err := env.ResidualAst(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported expr") {
		t.Errorf("env.ResidualAst() got (%v, %v) wanted unsupported expr error", ast, err)
	}
}

func BenchmarkEvalOptions(b *testing.B) {
	env := testEnv(b,
		Variable("ai", IntType),
		Variable("ar", MapType(StringType, StringType)),
	)
	ast, _ := env.Compile("ai == 20 || ar['foo'] == 'bar'")
	vars := map[string]any{
		"ai": 2,
		"ar": map[string]string{
			"foo": "bar",
		},
	}

	opts := map[string]EvalOption{
		"track-state":     OptTrackState,
		"exhaustive-eval": OptExhaustiveEval,
		"optimize":        OptOptimize,
	}
	for k, opt := range opts {
		b.Run(k, func(bb *testing.B) {
			prg, err := env.Program(ast, EvalOptions(opt))
			if err != nil {
				b.Fatalf("env.Program() failed: %v", err)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < bb.N; i++ {
				_, _, err := prg.Eval(vars)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestEnvExtension(t *testing.T) {
	env := testEnv(t,
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Variable("expr", ObjectType("google.api.expr.v1alpha1.Expr")),
		Variable("m", MapType(TypeParamType("K"), TypeParamType("V"))),
		OptionalTypes(),
	)
	e2, err := env.Extend(
		CustomTypeAdapter(types.DefaultTypeAdapter),
		Types(&proto3pb.TestAllTypes{}),
		OptionalTypes(),
		OptionalTypes(),
		OptionalTypes(),
	)
	if err != nil {
		t.Fatalf("env.Extend() failed: %v", err)
	}
	if env == e2 {
		t.Error("got object equality, wanted separate objects")
	}
	if env.TypeAdapter() == e2.TypeAdapter() {
		t.Error("got the same type adapter, wanted isolated instances.")
	}
	if env.TypeProvider() == e2.TypeProvider() {
		t.Error("got the same type provider, wanted isolated instances.")
	}
	e3, err := e2.Extend(OptionalTypes())
	if err != nil {
		t.Fatalf("env.Extend() failed: %v", err)
	}
	if e2.TypeAdapter() != e3.TypeAdapter() {
		t.Error("got different type adapters, wanted immutable adapter reference")
	}
	if e2.TypeProvider() == e3.TypeProvider() {
		t.Error("got the same type provider, wanted isolated instances.")
	}
}

func TestEnvExtensionIsolation(t *testing.T) {
	baseEnv := testEnv(t,
		Container("google.expr"),
		Variable("age", IntType),
		Variable("gender", StringType),
		Variable("country", StringType),
	)
	env1, err := baseEnv.Extend(
		Types(&proto2pb.TestAllTypes{}),
		Variable("name", StringType),
	)
	if err != nil {
		t.Fatal(err)
	}
	env2, err := baseEnv.Extend(
		Types(&proto3pb.TestAllTypes{}),
		Variable("group", StringType),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, issues := env2.Compile(`size(group) > 10
		&& !has(proto3.test.TestAllTypes{}.single_int32)`)
	if issues.Err() != nil {
		t.Fatal(issues.Err())
	}
	_, issues = env2.Compile(`size(name) > 10`)
	if issues.Err() == nil {
		t.Fatal("env2 contains 'name', but should not")
	}
	_, issues = env2.Compile(`!has(proto2.test.TestAllTypes{}.single_int32)`)
	if issues.Err() == nil {
		t.Fatal("env2 contains 'proto2.test.TestAllTypes', but should not")
	}

	_, issues = env1.Compile(`size(name) > 10
		&& !has(proto2.test.TestAllTypes{}.single_int32)`)
	if issues.Err() != nil {
		t.Fatal(issues.Err())
	}
	_, issues = env1.Compile("size(group) > 10")
	if issues.Err() == nil {
		t.Fatal("env1 contains 'group', but should not")
	}
	_, issues = env1.Compile(`!has(proto3.test.TestAllTypes{}.single_int32)`)
	if issues.Err() == nil {
		t.Fatal("env1 contains 'proto3.test.TestAllTypes', but should not")
	}
}

func TestVariadicLogicalOperators(t *testing.T) {
	env := testEnv(t, variadicLogicalOperatorASTs())
	ast, iss := env.Compile(
		`(false || false || false || false || true) &&
		 (true && true && true && true && false)`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program(ast) failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Errorf("Eval() got error %v, wanted false", err)
	}
	if out != types.False {
		t.Errorf("Eval() got %v, wanted false", out)
	}
}

func TestParseError(t *testing.T) {
	env := testEnv(t)
	_, iss := env.Parse("invalid & logical_and")
	if iss.Err() == nil {
		t.Fatal("e.Parse('invalid & logical_and') did not error")
	}
}

func TestParseWithMacroTracking(t *testing.T) {
	env := testEnv(t, EnableMacroCallTracking())
	ast, iss := env.Parse("has(a.b) && a.b.exists(c, c < 10)")
	if iss.Err() != nil {
		t.Fatalf("e.Parse() failed: %v", iss.Err())
	}
	pe, err := AstToParsedExpr(ast)
	if err != nil {
		t.Fatalf("AstToParsedExpr(%v) failed: %v", ast, err)
	}
	macroCalls := pe.GetSourceInfo().GetMacroCalls()
	if len(macroCalls) != 2 {
		t.Errorf("got %d macro calls, wanted 2", len(macroCalls))
	}
	callsFound := map[string]bool{"has": false, "exists": false}
	for _, expr := range macroCalls {
		f := expr.GetCallExpr().GetFunction()
		_, found := callsFound[f]
		if !found {
			t.Errorf("Unexpected macro call: %v", expr)
		}
		callsFound[f] = true
	}
	callsWanted := map[string]bool{"has": true, "exists": true}
	if !reflect.DeepEqual(callsFound, callsWanted) {
		t.Errorf("Tracked calls %v, but wanted %v", callsFound, callsWanted)
	}
}

func TestParseAndCheckConcurrently(t *testing.T) {
	env := testEnv(t,
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Variable("expr", ObjectType("google.api.expr.v1alpha1.Expr")),
	)
	parseAndCheck := func(expr string) {
		_, iss := env.Compile(expr)
		if iss.Err() != nil {
			t.Fatalf("e.Compile('%s') failed: %v", expr, iss.Err())
		}
	}

	const concurrency = 10
	wgDone := sync.WaitGroup{}
	wgDone.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(i int) {
			defer wgDone.Done()
			parseAndCheck(fmt.Sprintf("expr.id + %d", i))
		}(i)
	}
	wgDone.Wait()
}

func TestCustomInterpreterDecorator(t *testing.T) {
	var lastInstruction interpreter.Interpretable
	optimizeArith := func(i interpreter.Interpretable) (interpreter.Interpretable, error) {
		lastInstruction = i
		// Only optimize the instruction if it is a call.
		call, ok := i.(interpreter.InterpretableCall)
		if !ok {
			return i, nil
		}
		// Only optimize the math functions when they have constant arguments.
		switch call.Function() {
		case operators.Add,
			operators.Subtract,
			operators.Multiply,
			operators.Divide:
			// These are all binary operators so they should have to arguments
			args := call.Args()
			_, lhsIsConst := args[0].(interpreter.InterpretableConst)
			_, rhsIsConst := args[1].(interpreter.InterpretableConst)
			// When the values are constant then the call can be evaluated with
			// an empty activation and the value returns as a constant.
			if !lhsIsConst || !rhsIsConst {
				return i, nil
			}
			val := call.Eval(interpreter.EmptyActivation())
			if types.IsError(val) {
				return nil, val.(*types.Err)
			}
			return interpreter.NewConstValue(call.ID(), val), nil
		default:
			return i, nil
		}
	}

	env := testEnv(t, Variable("foo", IntType))
	ast, iss := env.Compile(`foo == -1 + 2 * 3 / 3`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	_, err := env.Program(ast,
		EvalOptions(OptPartialEval),
		CustomDecorator(optimizeArith))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	call, ok := lastInstruction.(interpreter.InterpretableCall)
	if !ok {
		t.Errorf("got %v, expected call", lastInstruction)
	}
	args := call.Args()
	lhs := args[0]
	lastAttr, ok := lhs.(interpreter.InterpretableAttribute)
	if !ok {
		t.Errorf("got %v, wanted attribute", lhs)
	}
	absAttr := lastAttr.Attr().(interpreter.NamespacedAttribute)
	varNames := absAttr.CandidateVariableNames()
	if len(varNames) != 1 || varNames[0] != "foo" {
		t.Errorf("got variables %v, wanted foo", varNames)
	}
	rhs := args[1]
	lastConst, ok := rhs.(interpreter.InterpretableConst)
	if !ok {
		t.Errorf("got %v, wanted constant", rhs)
	}
	// This is the last number produced by the optimization.
	if lastConst.Value().Equal(types.IntOne) == types.False {
		t.Errorf("got %v as the last observed constant, wanted 1", lastConst)
	}
}

func TestCustomInterpreterDecoratorV2(t *testing.T) {
	var lastInstruction interpreter.InterpretableV2
	optimizeArith := func(i interpreter.InterpretableV2) (interpreter.InterpretableV2, error) {
		lastInstruction = i
		// Only optimize the instruction if it is a call.
		call, ok := i.(interpreter.InterpretableCall)
		if !ok {
			return i, nil
		}
		// Only optimize the math functions when they have constant arguments.
		switch call.Function() {
		case operators.Add,
			operators.Subtract,
			operators.Multiply,
			operators.Divide:
			// These are all binary operators so they should have two arguments
			args := call.Args()
			_, lhsIsConst := args[0].(interpreter.InterpretableConst)
			_, rhsIsConst := args[1].(interpreter.InterpretableConst)
			// When the values are constant then the call can be evaluated with
			// an empty activation and the value returns as a constant.
			if !lhsIsConst || !rhsIsConst {
				return i, nil
			}
			val := call.Eval(interpreter.EmptyActivation())
			if types.IsError(val) {
				return nil, val.(*types.Err)
			}
			return interpreter.NewConstValue(call.ID(), val), nil
		default:
			return i, nil
		}
	}

	env := testEnv(t, Variable("foo", IntType))
	ast, iss := env.Compile(`foo == -1 + 2 * 3 / 3`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	_, err := env.Program(ast,
		EvalOptions(OptPartialEval),
		CustomDecoratorV2(optimizeArith))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	call, ok := lastInstruction.(interpreter.InterpretableCall)
	if !ok {
		t.Errorf("got %v, expected call", lastInstruction)
	}
	args := call.Args()
	lhs := args[0]
	lastAttr, ok := lhs.(interpreter.InterpretableAttribute)
	if !ok {
		t.Errorf("got %v, wanted attribute", lhs)
	}
	absAttr := lastAttr.Attr().(interpreter.NamespacedAttribute)
	varNames := absAttr.CandidateVariableNames()
	if len(varNames) != 1 || varNames[0] != "foo" {
		t.Errorf("got variables %v, wanted foo", varNames)
	}
	rhs := args[1]
	lastConst, ok := rhs.(interpreter.InterpretableConst)
	if !ok {
		t.Errorf("got %v, wanted constant", rhs)
	}
	// This is the last number produced by the optimization.
	if lastConst.Value().Equal(types.IntOne) == types.False {
		t.Errorf("got %v as the last observed constant, wanted 1", lastConst)
	}
}

// TestEstimateCostAndRuntimeCost sanity checks that the cost systems are usable from the program API.
func TestEstimateCostAndRuntimeCost(t *testing.T) {
	intList := ListType(IntType)
	zeroCost := checker.CostEstimate{}
	cases := []struct {
		name  string
		expr  string
		decls []EnvOption
		hints map[string]uint64
		want  checker.CostEstimate
		in    any
	}{
		{
			name: "const",
			expr: `"Hello World!"`,
			want: zeroCost,
			in:   map[string]any{},
		},
		{
			name:  "identity",
			expr:  `input`,
			decls: []EnvOption{Variable("input", intList)},
			want:  checker.CostEstimate{Min: 1, Max: 1},
			in:    map[string]any{"input": []int{1, 2}},
		},
		{
			name: "str concat",
			expr: `"abcdefg".contains(str1 + str2)`,
			decls: []EnvOption{
				Variable("str1", StringType),
				Variable("str2", StringType),
			},
			hints: map[string]uint64{"str1": 10, "str2": 10},
			want:  checker.CostEstimate{Min: 2, Max: 6},
			in:    map[string]any{"str1": "val1111111", "str2": "val2222222"},
		},
	}

	for _, tst := range cases {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.hints == nil {
				tc.hints = map[string]uint64{}
			}
			envOpts := []EnvOption{
				CostEstimatorOptions(
					checker.OverloadCostEstimate(overloads.TimestampToYear, estimateTimestampToYear),
				),
			}
			envOpts = append(envOpts, tc.decls...)
			env := testEnv(t, envOpts...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			est, err := env.EstimateCost(ast, testCostEstimator{hints: tc.hints})
			if err != nil {
				t.Fatalf("Env.EstimateCost(ast *Ast, estimator checker.CostEstimator) failed to estimate cost: %s\n", err)
			}
			if est.Min != tc.want.Min || est.Max != tc.want.Max {
				t.Fatalf("Env.EstimateCost(ast *Ast, estimator checker.CostEstimator) failed to return the right cost interval. Got [%v, %v], wanted [%v, %v]",
					est.Min, est.Max, tc.want.Min, tc.want.Max)
			}

			checkedAst, iss := env.Check(ast)
			if iss.Err() != nil {
				t.Fatalf(`Env.Check(ast *Ast) failed to check expression: %v`, iss.Err())
			}
			// Evaluate expression.
			program, err := env.Program(checkedAst,
				CostTracking(testRuntimeCostEstimator{}),
				CostTrackerOptions(
					interpreter.OverloadCostTracker(overloads.TimestampToYear, trackTimestampToYear),
				),
			)
			if err != nil {
				t.Fatalf(`Env.Program(ast *Ast, opts ...ProgramOption) failed to construct program: %v`, err)
			}
			_, details, err := program.Eval(tc.in)
			if err != nil {
				t.Fatalf(`Program.Eval(vars any) failed to evaluate expression: %v`, err)
			}
			actualCost := details.ActualCost()
			if actualCost == nil {
				t.Errorf(`EvalDetails.ActualCost() got nil for "%s" cost, wanted %d`, tc.expr, actualCost)
			}

			if est.Min > *actualCost || est.Max < *actualCost {
				t.Errorf("EvalDetails.ActualCost() failed to return a runtime cost %d is the range of estimate cost [%d, %d]", *actualCost,
					est.Min, est.Max)
			}
		})
	}
}

func TestCostLimit(t *testing.T) {
	cases := []struct {
		name      string
		expr      string
		decls     []EnvOption
		costLimit uint64
		in        any
		err       error
	}{
		{
			name: "greater",
			expr: `val1 > val2`,
			decls: []EnvOption{
				Variable("val1", IntType),
				Variable("val2", IntType),
			},
			in:        map[string]any{"val1": 1, "val2": 2},
			costLimit: 10,
		},
		{
			name: "greater - error",
			expr: `val1 > val2`,
			decls: []EnvOption{
				Variable("val1", IntType),
				Variable("val2", IntType),
			},
			in:        map[string]any{"val1": 1, "val2": 2},
			costLimit: 0,
			err:       errors.New("actual cost limit exceeded"),
		},
	}

	for _, tst := range cases {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			envOpts := []EnvOption{
				CostEstimatorOptions(
					checker.OverloadCostEstimate(overloads.TimestampToYear, estimateTimestampToYear),
				),
			}
			envOpts = append(envOpts, tc.decls...)
			env := testEnv(t, envOpts...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			est, err := env.EstimateCost(ast, testCostEstimator{hints: map[string]uint64{}})
			if err != nil {
				t.Fatalf("Env.EstimateCost(ast *Ast, estimator checker.CostEstimator) failed to estimate cost: %s\n", err)
			}

			checkedAst, iss := env.Check(ast)
			if iss.Err() != nil {
				t.Fatalf(`Env.Check(ast *Ast) failed to check expression: %v`, iss.Err())
			}
			// Evaluate expression.
			program, err := env.Program(checkedAst,
				CostTracking(testRuntimeCostEstimator{}),
				CostTrackerOptions(
					interpreter.OverloadCostTracker(overloads.TimestampToYear, trackTimestampToYear),
				),
				CostLimit(tc.costLimit),
			)
			if err != nil {
				t.Fatalf(`Env.Program(ast *Ast, opts ...ProgramOption) failed to construct program: %v`, err)
			}
			_, details, err := program.Eval(tc.in)
			if err != nil && tc.err == nil {
				t.Fatalf(`Program.Eval(vars any) failed to evaluate expression: %v`, err)
			}
			actualCost := details.ActualCost()
			if actualCost == nil {
				t.Errorf(`EvalDetails.ActualCost() got nil for "%s" cost, wanted %d`, tc.expr, actualCost)
			}
			if err == nil {
				if est.Min > *actualCost || est.Max < *actualCost {
					t.Errorf("EvalDetails.ActualCost() failed to return a runtime cost %d is the range of estimate cost [%d, %d]", *actualCost,
						est.Min, est.Max)
				}
			} else {
				if !strings.Contains(err.Error(), tc.err.Error()) {
					t.Fatalf("program.Eval() got error %v, wanted error containing %v", err, tc.err)
				}
			}
		})
	}
}

func TestPartialVars(t *testing.T) {
	env := testEnv(t,
		Variable("x", StringType),
		Variable("y", IntType),
	)
	ast, iss := env.Compile("x == string(y)")
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptPartialEval))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	tests := []struct {
		in         map[string]any
		unk        []*interpreter.AttributePattern
		out        ref.Val
		partialOut ref.Val
	}{
		{
			in: map[string]any{},
			unk: []*interpreter.AttributePattern{
				interpreter.NewAttributePattern("x"),
				interpreter.NewAttributePattern("y"),
			},
			out: types.NewUnknown(1, types.NewAttributeTrail("x")),
		},
		{
			in: map[string]any{"x": "10"},
			unk: []*interpreter.AttributePattern{
				interpreter.NewAttributePattern("y"),
			},
			out: types.NewUnknown(4, types.NewAttributeTrail("y")),
		},
		{
			in: map[string]any{"y": 10},
			unk: []*interpreter.AttributePattern{
				interpreter.NewAttributePattern("x"),
			},
			out: types.NewUnknown(1, types.NewAttributeTrail("x")),
		},
		{
			in:  map[string]any{"x": "10", "y": 10},
			unk: []*interpreter.AttributePattern{},
			out: types.True,
		},
		{
			in:  map[string]any{"x": "10", "y": 9},
			unk: []*interpreter.AttributePattern{},
			out: types.False,
		},
		{
			in:         map[string]any{"y": 10},
			unk:        []*interpreter.AttributePattern{},
			out:        types.NewErr("no such attribute: x"),
			partialOut: types.NewUnknown(1, types.NewAttributeTrail("x")),
		},
		{
			in:         map[string]any{"x": "10"},
			unk:        []*interpreter.AttributePattern{},
			out:        types.NewErr("no such attribute: y"),
			partialOut: types.NewUnknown(4, types.NewAttributeTrail("y")),
		},
		{
			in:         map[string]any{},
			unk:        []*interpreter.AttributePattern{},
			out:        types.NewErr("no such attribute: x"),
			partialOut: types.NewUnknown(1, types.NewAttributeTrail("x")),
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(t *testing.T) {
			// Manually configured unknown patterns
			vars, err := PartialVars(tc.in, tc.unk...)
			if err != nil {
				t.Fatalf("PartialVars() failed: %v", err)
			}
			out, _, err := prg.Eval(vars)
			if err != nil {
				if types.IsError(out) {
					if !out.(*types.Err).Is(err) {
						t.Errorf("Eval() got %v, wanted error %v", err, out)
					}
				}
			} else if types.IsUnknown(out) {
				if !reflect.DeepEqual(out, tc.out) {
					t.Errorf("Eval() got unknown %v, wanted %v", out, tc.out)
				}
			} else if out.Equal(tc.out) != types.True {
				t.Errorf("Eval() got %v, wanted %v", out, tc.out)
			}
			// Inferred unknown patterns
			vars2, err := env.PartialVars(tc.in)
			if err != nil {
				t.Fatalf("env.PartialVars() failed: %v", err)
			}
			out2, _, err := prg.Eval(vars2)
			if err != nil {
				t.Fatalf("prg.Eval() with inferred unknowns failed: %v", err)
			}
			want := tc.out
			if tc.partialOut != nil {
				want = tc.partialOut
			}
			if types.IsUnknown(out2) {
				if !reflect.DeepEqual(out2, want) {
					t.Errorf("Eval() got unknown %v, wanted %v", out2, want)
				}
			} else if out2.Equal(want) != types.True {
				t.Errorf("Eval() got %v, wanted %v", out2, want)
			}
		})
	}
}

func TestResidualAstAttributeQualifiers(t *testing.T) {
	env := testEnv(t,
		Variable("x", MapType(StringType, DynType)),
		Variable("y", ListType(IntType)),
		Variable("u", IntType),
	)
	ast, iss := env.Parse(`x.abc == u && x["abc"] == u && x[x.string] == u && y[0] == u && y[x.zero] == u && (true ? x : y).abc == u && (false ? y : x).abc == u`)
	if iss.Err() != nil {
		t.Fatalf("env.Parse() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	vars, _ := PartialVars(map[string]any{
		"x": map[string]any{
			"zero":   0,
			"abc":    123,
			"string": "abc",
		},
		"y": []int{123},
	}, AttributePattern("u"))
	out, det, err := prg.ContextEval(context.TODO(), vars)
	if !types.IsUnknown(out) {
		t.Fatalf("got %v, expected unknown", out)
	}
	if err != nil {
		t.Fatal(err)
	}
	residual, err := env.ResidualAst(ast, det)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := AstToString(residual)
	if err != nil {
		t.Fatal(err)
	}
	const want = "123 == u && 123 == u && 123 == u && 123 == u && 123 == u && 123 == u && 123 == u"
	if expr != want {
		t.Errorf("got expr: %s, wanted %s", expr, want)
	}
}

func TestPartialVarsEnv(t *testing.T) {
	env := testEnv(t,
		Variable("x", IntType),
		Variable("y", IntType),
	)

	// Use env to make sure internals are all initialized.
	ast, iss := env.Compile("x == y")

	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptPartialEval))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	act, err := env.PartialVars(map[string]any{"x": 1, "y": 1})

	if err != nil {
		t.Fatalf("env.PartialVars failed: %v", err)
	}
	val, _, err := prg.Eval(act)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if val != types.True {
		t.Fatalf("want: %v, got: %v", types.True, val)
	}
}

func TestPartialVarsExtendedEnv(t *testing.T) {
	env := testEnv(t,
		Variable("x", IntType),
		Variable("y", IntType),
	)

	env.Compile("x == y")
	// Now test that a sub environment is correctly copied.
	env2, err := env.Extend(Variable("z", IntType))
	if err != nil {
		t.Fatalf("env.Extend failed: %v", err)
	}

	ast, iss := env2.Compile("x == y && y == z")

	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env2.Program(ast, EvalOptions(OptPartialEval))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	act, err := env2.PartialVars(map[string]any{"z": 1, "y": 1})

	if err != nil {
		t.Fatalf("env.PartialVars failed: %v", err)
	}
	val, _, err := prg.Eval(act)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if !types.IsUnknown(val) {
		t.Fatalf("Wanted unknown, got %v", val)
	}

	if !reflect.DeepEqual(val, types.NewUnknown(1, types.NewAttributeTrail("x"))) {
		t.Fatalf("Wanted Unknown(x (1)), got: %v", val)
	}
}

func TestResidualAstModified(t *testing.T) {
	env := testEnv(t,
		Variable("x", MapType(StringType, IntType)),
		Variable("y", IntType),
	)
	ast, iss := env.Parse("x == y")
	if iss.Err() != nil {
		t.Fatalf("env.Parse() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	for _, x := range []int{123, 456} {
		vars, _ := PartialVars(map[string]any{
			"x": x,
		}, AttributePattern("y"))
		out, det, err := prg.Eval(vars)
		if !types.IsUnknown(out) {
			t.Fatalf("got %v, expected unknown", out)
		}
		if err != nil {
			t.Fatal(err)
		}
		residual, err := env.ResidualAst(ast, det)
		if err != nil {
			t.Fatal(err)
		}
		orig, err := AstToString(ast)
		if err != nil {
			t.Fatal(err)
		}
		if orig != "x == y" {
			t.Errorf("parsed ast: got expr: %s, wanted x == y", orig)
		}
		expr, err := AstToString(residual)
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("%d == y", x)
		if expr != want {
			t.Errorf("residual ast: got expr: %s, wanted %s", expr, want)
		}
	}
}

func TestContextProto(t *testing.T) {
	descriptor := new(proto3pb.TestAllTypes).ProtoReflect().Descriptor()
	option := DeclareContextProto(descriptor)
	env := testEnv(t, option)
	expression := `
	single_int64 == 1
	&& single_double == 1.0
	&& single_bool == true
	&& single_string == ''
	&& single_nested_message == google.expr.proto3.test.TestAllTypes.NestedMessage{}
	&& standalone_enum == google.expr.proto3.test.TestAllTypes.NestedEnum.FOO
	&& single_duration == duration('5s')
	&& single_timestamp == timestamp(63154820)
	&& single_any == null
	&& single_uint32_wrapper == null
	&& single_uint64_wrapper == 0u
	&& repeated_int32 == [1,2]
	&& map_string_string == {'': ''}
	&& map_int64_nested_type == {0 : google.expr.proto3.test.NestedTestAllTypes{}}`
	ast, iss := env.Compile(expression)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %s", expression, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	in := &proto3pb.TestAllTypes{
		SingleInt64:  1,
		SingleDouble: 1.0,
		SingleBool:   true,
		NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{
			SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{},
		},
		StandaloneEnum: proto3pb.TestAllTypes_FOO,
		SingleDuration: &durationpb.Duration{Seconds: 5},
		SingleTimestamp: &timestamppb.Timestamp{
			Seconds: 63154820,
		},
		SingleUint64Wrapper: wrapperspb.UInt64(0),
		RepeatedInt32:       []int32{1, 2},
		MapStringString:     map[string]string{"": ""},
		MapInt64NestedType:  map[int64]*proto3pb.NestedTestAllTypes{0: {}},
	}
	vars, err := ContextProtoVars(in)
	if err != nil {
		t.Fatalf("ContextProtoVars(%v) failed: %v", in, err)
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out.Equal(types.True) != types.True {
		t.Errorf("prg.Eval() got %v, wanted true", out)
	}
}

func TestContextProtoJSONFieldNames(t *testing.T) {
	descriptor := new(proto3pb.TestAllTypes).ProtoReflect().Descriptor()
	env := testEnv(t, JSONFieldNames(true), DeclareContextProto(descriptor))
	expression := `
	singleInt64 == 1
	&& singleDouble == 1.0
	&& singleBool == true
	&& singleString == ''
	&& singleNestedMessage == google.expr.proto3.test.TestAllTypes.NestedMessage{}
	&& standaloneEnum == google.expr.proto3.test.TestAllTypes.NestedEnum.FOO
	&& singleDuration == duration('5s')
	&& singleTimestamp == timestamp(63154820)
	&& singleAny == null
	&& singleUint32Wrapper == null
	&& singleUint64Wrapper == 0u
	&& repeatedInt32 == [1,2]
	&& mapStringString == {'': ''}
	&& mapInt64NestedType == {0 : google.expr.proto3.test.NestedTestAllTypes{}}`
	ast, iss := env.Compile(expression)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %s", expression, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	in := &proto3pb.TestAllTypes{
		SingleInt64:  1,
		SingleDouble: 1.0,
		SingleBool:   true,
		NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{
			SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{},
		},
		StandaloneEnum: proto3pb.TestAllTypes_FOO,
		SingleDuration: &durationpb.Duration{Seconds: 5},
		SingleTimestamp: &timestamppb.Timestamp{
			Seconds: 63154820,
		},
		SingleUint64Wrapper: wrapperspb.UInt64(0),
		RepeatedInt32:       []int32{1, 2},
		MapStringString:     map[string]string{"": ""},
		MapInt64NestedType:  map[int64]*proto3pb.NestedTestAllTypes{0: {}},
	}
	vars, err := ContextProtoVars(in, types.JSONFieldNames(true))
	if err != nil {
		t.Fatalf("ContextProtoVars(%v) failed: %v", in, err)
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out.Equal(types.True) != types.True {
		t.Errorf("prg.Eval() got %v, wanted true", out)
	}
}

func TestRegexOptimizer(t *testing.T) {
	var stringTests = []struct {
		expr          string
		optimizeRegex bool
		progErr       string
		err           string
		parseOnly     bool
	}{
		{expr: `"123 abc 456".matches('[0-9]*')`},
		{expr: `"123 abc 456".matches('[0-9]' + '*')`},
		{expr: `"123 abc 456".matches('[0-9]*')`, optimizeRegex: true},
		{expr: `"123 abc 456".matches('[0-9]' + '*')`, optimizeRegex: true},
		{
			// Verify that a regex compilation error for an optimized regex is
			// reported at program creation time.
			expr: `"123 abc 456".matches(')[0-9]*')`, optimizeRegex: true,
			progErr: "error parsing regexp: unexpected ): `)[0-9]*`",
		},
		{
			expr: `"123 abc 456".matches(')[0-9]*')`,
			err:  "error parsing regexp: unexpected ): `)[0-9]*`",
		},
	}

	env := testEnv(t)
	for i, tst := range stringTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(tt *testing.T) {
			var asts []*Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				tt.Fatal(iss.Err())
			}
			asts = append(asts, pAst)
			if !tc.parseOnly {
				cAst, iss := env.Check(pAst)
				if iss.Err() != nil {
					tt.Fatal(iss.Err())
				}
				asts = append(asts, cAst)
			}
			for _, ast := range asts {
				var opts []ProgramOption
				if tc.optimizeRegex {
					opts = append(opts, EvalOptions(OptOptimize))
				}
				prg, progErr := env.Program(ast, opts...)
				if tc.progErr != "" {
					if progErr == nil {
						tt.Fatalf("wanted error %s for expr: %s", tc.progErr, tc.expr)
					}
					if tc.progErr != progErr.Error() {
						tt.Errorf("got error %v, wanted error %s for expr: %s", progErr, tc.progErr, tc.expr)
					}
					continue
				} else if progErr != nil {
					tt.Fatal(progErr)
				}
				out, _, err := prg.Eval(NoVars())
				if tc.err != "" {
					if err == nil {
						tt.Fatalf("got value %v, wanted error %s for expr: %s",
							out.Value(), tc.err, tc.expr)
					}
					if tc.err != err.Error() {
						tt.Errorf("got error %v, wanted error %s for expr: %s", err, tc.err, tc.expr)
					}
				} else if err != nil {
					tt.Fatal(err)
				} else if out.Value() != true {
					tt.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}

func TestDefaultUTCTimeZoneDisabled(t *testing.T) {
	testEnvs := []struct {
		name string
		env  *Env
	}{
		{"default", testEnv(t, Variable("x", TimestampType))},
		{"enabled", testEnv(t, Variable("x", TimestampType), DefaultUTCTimeZone(true))},
		{"disabled", testEnv(t, Variable("x", TimestampType), DefaultUTCTimeZone(false))},
	}
	exprs := []struct {
		name   string
		value  string
		envOut map[string]ref.Val
	}{
		{
			name: "default-timezone",
			value: `
			x.getFullYear() == 1970
			&& x.getMonth() == 0
			&& x.getDayOfYear() == 0
			&& x.getDayOfMonth() == 0
			&& x.getDate() == 1
			&& x.getDayOfWeek() == 4
			&& x.getHours() == 2
			&& x.getMinutes() == 5
			&& x.getSeconds() == 6
			&& x.getMilliseconds() == 1`,
			envOut: map[string]ref.Val{
				"default":  types.True,
				"enabled":  types.True,
				"disabled": types.False,
			},
		},
		{
			name:  "default-local-year",
			value: `x.getFullYear()`,
			envOut: map[string]ref.Val{
				"default":  types.Int(1970),
				"enabled":  types.Int(1970),
				"disabled": types.Int(1969),
			},
		},
		{
			name:  "default-local-day-of-year",
			value: `x.getDayOfYear()`,
			envOut: map[string]ref.Val{
				"default":  types.Int(0),
				"enabled":  types.Int(0),
				"disabled": types.Int(364),
			},
		},
		{
			name:  "default-local-month",
			value: `x.getMonth()`,
			envOut: map[string]ref.Val{
				"default":  types.Int(0),
				"enabled":  types.Int(0),
				"disabled": types.Int(11),
			},
		},
		{
			name: "default-local-day-of-month",
			value: `
			x.getDayOfMonth() == 30
			&& x.getDate() == 31`,
			envOut: map[string]ref.Val{
				"default":  types.False,
				"enabled":  types.False,
				"disabled": types.True,
			},
		},
		{
			name:  "default-local-dates",
			value: `x.getDayOfWeek()`,
			envOut: map[string]ref.Val{
				"default":  types.Int(4),
				"enabled":  types.Int(4),
				"disabled": types.Int(3),
			},
		},
		{
			name: "default-local-times",
			value: `
			x.getHours() == 18
			&& x.getMinutes() == 5
			&& x.getSeconds() == 6
			&& x.getMilliseconds() == 1`,
			envOut: map[string]ref.Val{
				"default":  types.False,
				"enabled":  types.False,
				"disabled": types.True,
			},
		},
		{
			name: "explicit",
			value: `
			x.getFullYear('-07:30') == 1969
			&& x.getDayOfYear('-07:30') == 364
			&& x.getMonth('-07:30') == 11
			&& x.getDayOfMonth('-07:30') == 30
			&& x.getDate('-07:30') == 31
			&& x.getDayOfWeek('-07:30') == 3
			&& x.getHours('-07:30') == 18
			&& x.getMinutes('-07:30') == 35
			&& x.getSeconds('-07:30') == 6
			&& x.getMilliseconds('-07:30') == 1
			&& x.getFullYear('23:15') == 1970
			&& x.getDayOfYear('23:15') == 1
			&& x.getMonth('23:15') == 0
			&& x.getDayOfMonth('23:15') == 1
			&& x.getDate('23:15') == 2
			&& x.getDayOfWeek('23:15') == 5
			&& x.getHours('23:15') == 1
			&& x.getMinutes('23:15') == 20
			&& x.getSeconds('23:15') == 6
			&& x.getMilliseconds('23:15') == 1`,
			envOut: map[string]ref.Val{
				"default":  types.True,
				"enabled":  types.True,
				"disabled": types.True,
			},
		},
	}

	offset, _ := time.ParseDuration("-8h")
	vars := map[string]any{
		"x": time.Unix(7506, 1000000).In(time.FixedZone("", int(offset.Seconds()))),
	}
	for _, e := range testEnvs {
		te := e
		for _, expr := range exprs {
			ex := expr
			t.Run(fmt.Sprintf("%s/%s", te.name, ex.name), func(t *testing.T) {
				env := te.env
				expr := ex.value
				out, err := interpret(t, env, expr, vars)
				if err != nil {
					t.Fatal(err)
				}
				if out.Equal(ex.envOut[te.name]) != types.True {
					t.Errorf("interpret got %v, wanted %v", out, ex.envOut[te.name])
				}
			})
		}
	}
}

func TestDefaultUTCTimeZoneExtension(t *testing.T) {
	env := testEnv(t,
		Variable("x", TimestampType),
		Variable("y", DurationType),
	)
	env, err := env.Extend()
	if err != nil {
		t.Fatalf("env.Extend() failed: %v", err)
	}
	out, err := interpret(t, env, `
	    x.getFullYear() == 1970
		&& y.getHours() == 2
		&& y.getMinutes() == 120
		&& y.getSeconds() == 7235
		&& y.getMilliseconds() == 7235000`,
		map[string]any{
			"x": time.Unix(7506, 1000000).Local(),
			"y": time.Duration(7235) * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out != types.True {
		t.Errorf("Eval() got %v, wanted true", out.Value())
	}
}

func TestDefaultUTCTimeZoneError(t *testing.T) {
	env := testEnv(t, Variable("x", TimestampType))
	out, err := interpret(t, env, `
		x.getFullYear(':xx') == 1969
		|| x.getDayOfYear('xx:') == 364
		|| x.getMonth('Am/Ph') == 11
		|| x.getDayOfMonth('Am/Ph') == 30
		|| x.getDate('Am/Ph') == 31
		|| x.getDayOfWeek('Am/Ph') == 3
		|| x.getHours('Am/Ph') == 19
		|| x.getMinutes('Am/Ph') == 5
		|| x.getSeconds('Am/Ph') == 6
		|| x.getMilliseconds('Am/Ph') == 1
	`, map[string]any{
		"x": time.Unix(7506, 1000000).Local(),
	})
	if err == nil {
		t.Fatalf("prg.Eval() got %v wanted error", out)
	}
}

func TestParserRecursionLimit(t *testing.T) {
	testCases := []struct {
		expr        string
		errorSubstr string
		out         ref.Val
	}{
		{
			expr:        `0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10 + 11`,
			errorSubstr: "max recursion depth exceeded",
		},
		{
			expr: `0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10`,
			out:  types.Int(55),
		},
		{
			expr:        `0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10 == 45`,
			errorSubstr: "max recursion depth exceeded",
		},
		{
			// Operator precedence means that '==' is the root.
			expr: `0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 == 0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9`,
			out:  types.True,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expr, func(t *testing.T) {
			env := testEnv(t, ParserRecursionLimit(10))
			out, err := interpret(t, env,
				tc.expr, map[string]any{})

			if tc.errorSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorSubstr) {
					t.Fatalf("prg.Eval() wanted error containing '%s' got %v", tc.errorSubstr, err)
				}
			}

			if tc.out != nil {
				if tc.out != out {
					t.Errorf("prg.Eval() wanted %v got %v", tc.out, out)
				}
			}
		})

	}
}

func TestQuotedFields(t *testing.T) {
	testCases := []struct {
		expr        string
		errorSubstr string
		out         ref.Val
	}{
		{
			expr: "{'key-1': 64}.`key-1`",
			out:  types.Int(64),
		},
		{
			expr:        "{'key-1': 64}.`key-2`",
			errorSubstr: "no such key: key-2",
		},
		{
			expr: "has({'key-1': 64}.`key-1`)",
			out:  types.True,
		},
		{
			expr: "has({'key-1': 64}.`key-2`)",
			out:  types.False,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expr, func(t *testing.T) {
			env := testEnv(t, ParserRecursionLimit(10),
				EnableIdentifierEscapeSyntax())
			out, err := interpret(t, env,
				tc.expr, map[string]any{})

			if tc.errorSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorSubstr) {
					t.Fatalf("prg.Eval() wanted error containing '%s' got %v", tc.errorSubstr, err)
				}
			}

			if tc.out != nil {
				if tc.out != out {
					t.Errorf("prg.Eval() wanted %v got %v", tc.out, out)
				}
			}
		})

	}
}

func TestDynamicDispatch(t *testing.T) {
	env := testEnv(t,
		HomogeneousAggregateLiterals(),
		Function("first",
			MemberOverload("first_list_int", []*Type{ListType(IntType)}, IntType,
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.IntZero
					}
					return l.Get(types.IntZero)
				}),
			),
			MemberOverload("first_list_double", []*Type{ListType(DoubleType)}, DoubleType,
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.Double(0.0)
					}
					return l.Get(types.IntZero)
				}),
			),
			MemberOverload("first_list_string", []*Type{ListType(StringType)}, StringType,
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.String("")
					}
					return l.Get(types.IntZero)
				}),
			),
			MemberOverload("first_list_list_string", []*Type{ListType(ListType(StringType))}, ListType(StringType),
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.DefaultTypeAdapter.NativeToValue([]string{})
					}
					return l.Get(types.IntZero)
				}),
			),
		),
	)
	out, err := interpret(t, env, `
		dyn([]).first() == 0
		&& [1, 2].first() == 1
		&& [1.0, 2.0].first() == 1.0
		&& ["hello", "world"].first() == "hello"
		&& [["hello"], ["world", "!"]].first().first() == "hello"
		&& [[], ["empty"]].first().first() == ""
		&& dyn([1, 2]).first() == 1
		&& dyn([1.0, 2.0]).first() == 1.0
		&& dyn(["hello", "world"]).first() == "hello"
		&& dyn([["hello"], ["world", "!"]]).first().first() == "hello"
	`, map[string]any{},
	)
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out != types.True {
		t.Fatalf("prg.Eval() got %v wanted true", out)
	}
}

func TestOptionalValuesCompile(t *testing.T) {
	env := testEnv(t,
		OptionalTypes(),
		// Test variables.
		Variable("m", MapType(StringType, MapType(StringType, StringType))),
		Variable("optm", OptionalType(MapType(StringType, MapType(StringType, StringType)))),
		Variable("l", ListType(StringType)),
		Variable("optl", OptionalType(ListType(StringType))),
		Variable("x", OptionalType(IntType)),
		Variable("y", IntType),
	)
	tests := []struct {
		expr       string
		references map[int64]*celast.ReferenceInfo
	}{
		{
			expr: `x.or(optional.of(y)).orValue(42)`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "x"},
				2: {OverloadIDs: []string{"optional_or_optional"}},
				4: {OverloadIDs: []string{"optional_of"}},
				5: {Name: "y"},
				6: {OverloadIDs: []string{"optional_orValue_value"}},
			},
		},
		{
			expr: `m.?x.hasValue()`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "m"},
				3: {OverloadIDs: []string{"select_optional_field"}},
				4: {OverloadIDs: []string{"optional_hasValue"}},
			},
		},
		{
			expr: `has(m.?x.y)`,
			references: map[int64]*celast.ReferenceInfo{
				2: {Name: "m"},
				4: {OverloadIDs: []string{"select_optional_field"}},
			},
		},
		{
			// Optional index selection in map.
			expr: `m.k[?'dashed-index'].orValue('default value')`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "m"},
				3: {OverloadIDs: []string{"map_optindex_optional_value"}},
				5: {OverloadIDs: []string{"optional_orValue_value"}},
			},
		},
		{
			// Optional index selection in list.
			expr: `l[?y]`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "l"},
				2: {OverloadIDs: []string{"list_optindex_optional_int"}},
				3: {Name: "y"},
			},
		},
		{
			// Index selection against a value in an optional map.
			expr: `optm.c['index'].orValue('default value')`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "optm"},
				3: {OverloadIDs: []string{"optional_map_index_value"}},
				5: {OverloadIDs: []string{"optional_orValue_value"}},
			},
		},
		{
			// Index selection against a value in an optional map.
			expr: `optm.c[?'index']`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "optm"},
				3: {OverloadIDs: []string{"optional_map_optindex_optional_value"}},
			},
		},
		{
			// Index selection against a value in an optional list.
			expr: `optl[0]`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "optl"},
				2: {OverloadIDs: []string{"optional_list_index_int"}},
			},
		},
		{
			// Index selection against a value in an optional list.
			expr: `optl[?0]`,
			references: map[int64]*celast.ReferenceInfo{
				1: {Name: "optl"},
				2: {OverloadIDs: []string{"optional_list_optindex_optional_int"}},
			},
		},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("%v failed: %v", tc.expr, iss.Err())
			}
			for id, reference := range ast.NativeRep().ReferenceMap() {
				other, found := tc.references[id]
				if !found {
					t.Errorf("Compile(%v) expected reference %d: %v", tc.expr, id, reference)
				} else if !reference.Equals(other) {
					t.Errorf("Compile(%v) got reference %d: %v, wanted %v", tc.expr, id, reference, other)
				}
			}
		})
	}
}

func TestOptionalValuesEval(t *testing.T) {
	env := testEnv(t,
		OptionalTypes(),
		// Container and test message types.
		Container("google.expr.proto2.test"),
		Types(&proto2pb.TestAllTypes{}),
		// Test variables.
		Variable("m", MapType(StringType, MapType(StringType, StringType))),
		Variable("l", ListType(StringType)),
		Variable("optm", OptionalType(MapType(StringType, MapType(StringType, StringType)))),
		Variable("optl", OptionalType(ListType(StringType))),
		Variable("x", OptionalType(IntType)),
		Variable("y", OptionalType(IntType)),
		Variable("z", IntType),
	)
	adapter := env.TypeAdapter()
	tests := []struct {
		expr string
		in   map[string]any
		out  any
	}{
		{
			expr: `has({'foo': optional.none()}.foo)`,
			out:  types.True,
		},
		{
			expr: `has({'foo': optional.none()}.foo.value)`,
			out:  types.False,
		},
		{
			expr: `has({?'foo': optional.none()}.foo)`,
			out:  types.False,
		},
		{
			expr: `has({?'foo': optional.none()}.foo.value)`,
			out:  "no such key: foo",
		},
		{
			expr: `{}.?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `{'null_field': dyn(null)}.?null_field`,
			out:  types.OptionalOf(types.NullValue),
		},
		{
			expr: `{'null_field': dyn(null)}.?null_field.?nested`,
			out:  types.OptionalNone,
		},
		{
			expr: `{'zero_field': dyn(0)}.?zero_field.?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `{0: dyn(0)}[?0].?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `{true: dyn(0)}[?false].?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `{true: dyn(0)}[?true].?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `x.or(y).orValue(z)`,
			in: map[string]any{
				"x": types.OptionalNone,
				"y": types.OptionalNone,
				"z": 42,
			},
			out: 42,
		},
		{
			expr: `x.optMap(y, y + 1)`,
			in: map[string]any{
				"x": types.OptionalNone,
			},
			out: types.OptionalNone,
		},
		{
			expr: `m.?key.optFlatMap(k, k.?subkey)`,
			in: map[string]any{
				"m": map[string]any{},
			},
			out: types.OptionalNone,
		},
		{
			expr: `m.?key.optFlatMap(k, k.?subkey)`,
			in: map[string]any{
				"m": map[string]any{
					"key": map[string]string{},
				},
			},
			out: types.OptionalNone,
		},
		{
			expr: `m.?key.optFlatMap(k, k.?subkey)`,
			in: map[string]any{
				"m": map[string]any{
					"key": map[string]string{
						"subkey": "subvalue",
					},
				},
			},
			out: types.OptionalOf(types.String("subvalue")),
		},
		{
			expr: `m.?key.optFlatMap(k, k.?subkey)`,
			in: map[string]any{
				"m": map[string]any{
					"key": map[string]string{
						"subkey": "",
					},
				},
			},
			out: types.OptionalOf(types.String("")),
		},
		{
			expr: `m.?key.optFlatMap(k, optional.ofNonZeroValue(k.subkey))`,
			in: map[string]any{
				"m": map[string]any{
					"key": map[string]string{
						"subkey": "",
					},
				},
			},
			out: types.OptionalNone,
		},
		{
			expr: `x.optMap(y, y + 1)`,
			in: map[string]any{
				"x": types.OptionalOf(types.Int(42)),
			},
			out: types.OptionalOf(types.Int(43)),
		},
		{
			expr: `optional.ofNonZeroValue(z).or(optional.of(10)).value() == 42`,
			in: map[string]any{
				"z": 42,
			},
			out: true,
		},
		{
			// Equivalent to m.?x.hasValue()
			expr: `(has(m.x) ? optional.of(m.x) : optional.none()).hasValue()`,
			in: map[string]any{
				"m": map[string]map[string]string{},
			},
			out: false,
		},
		{
			expr: `m.?x.hasValue()`,
			in: map[string]any{
				"m": map[string]any{},
			},
			out: false,
		},
		{
			expr: `has(m.?x.y)`,
			in: map[string]any{
				"m": map[string]any{},
			},
			out: false,
		},
		{
			expr: `has(m.?x.y)`,
			in: map[string]any{
				"m": map[string]any{
					"x": map[string]string{
						"y": "z",
					},
				},
			},
			out: true,
		},
		{
			expr: `type(optional.none()) == optional_type`,
			out:  true,
		},
		{
			// return the value of m.c['dashed-index'], no magic in the optional.of() call.
			expr: `optional.ofNonZeroValue('').or(optional.of(m.c['dashed-index'])).orValue('default value')`,
			in: map[string]any{
				"m": map[string]any{
					"c": map[string]string{
						"dashed-index": "goodbye",
					},
				},
			},
			out: "goodbye",
		},
		{
			// Optional index selection in map where the index is found.
			expr: `m.c[?'dashed-index'].orValue('default value')`,
			in: map[string]any{
				"m": map[string]any{
					"c": map[string]string{
						"dashed-index": "goodbye",
					},
				},
			},
			out: "goodbye",
		},
		{
			// Optional index selection in map where the index is absent.
			expr: `m.c[?'missing-index'].orValue('default value')`,
			in: map[string]any{
				"m": map[string]any{
					"c": map[string]string{},
				},
			},
			out: "default value",
		},
		{
			// Traditional index selection against an optional value in map where the index is found.
			expr: `optm.c.index.orValue('default value')`,
			in: map[string]any{
				"optm": types.OptionalOf(
					adapter.NativeToValue(
						map[string]any{
							"c": map[string]string{
								"index": "goodbye",
							},
						},
					),
				),
			},
			out: "goodbye",
		},
		{
			// Traditional index selection against an optional value in map where the index is absent.
			expr: `optm.c.missing.or(optl[0]).orValue('default value')`,
			in: map[string]any{
				"optm": types.OptionalOf(
					adapter.NativeToValue(
						map[string]any{
							"c": map[string]string{},
						},
					),
				),
				"optl": types.OptionalNone,
			},
			out: "default value",
		},
		{
			// Traditional index selection against an optional value in map where the index is absent.
			expr: `optm.c.missing.or(optl[0]).orValue('default value')`,
			in: map[string]any{
				"optm": types.OptionalOf(
					adapter.NativeToValue(
						map[string]any{
							"c": map[string]string{},
						},
					),
				),
				"optl": types.OptionalOf(
					adapter.NativeToValue([]string{"list-value"}),
				),
			},
			out: "list-value",
		},
		{
			// Traditional index selection against an optional value in map where the index is found.
			expr: `optm.c['index'].orValue('default value')`,
			in: map[string]any{
				"optm": types.OptionalOf(
					adapter.NativeToValue(
						map[string]any{
							"c": map[string]string{
								"index": "goodbye",
							},
						},
					),
				),
			},
			out: "goodbye",
		},
		{
			// Traditional index selection against an optional value in map where the index is absent.
			expr: `optm.c['missing'].orValue('default value')`,
			in: map[string]any{
				"optm": types.OptionalOf(
					adapter.NativeToValue(
						map[string]any{
							"c": map[string]string{},
						},
					),
				),
			},
			out: "default value",
		},
		{
			// Presence test using optional value where the field is absent.
			expr: `has(optm.c) && !has(optm.c.missing)`,
			in: map[string]any{
				"optm": types.OptionalOf(
					adapter.NativeToValue(
						map[string]any{
							"c": map[string]string{
								"entry": "hello world",
							},
						},
					),
				),
			},
			out: true,
		},
		{
			// ensure an error is propagated to the result.
			expr: `optional.ofNonZeroValue(m.a.z).orValue(m.c['dashed-index'])`,
			in: map[string]any{
				"m": map[string]any{
					"c": map[string]string{
						"dashed-index": "goodbye",
					},
				},
			},
			out: "no such key: a",
		},
		{
			expr: `m.?c.missing.or(m.?c['dashed-index']).orValue('').size()`,
			in: map[string]any{
				"m": map[string]any{
					"c": map[string]string{
						"dashed-index": "goodbye",
					},
				},
			},
			out: 7,
		},
		{
			expr: `{?'nested_map': optional.ofNonZeroValue({?'map': m.?c})}`,
			in: map[string]any{
				"m": map[string]any{
					"c": map[string]string{
						"dashed-index": "goodbye",
					},
				},
			},
			out: map[string]any{
				"nested_map": map[string]any{
					"map": map[string]string{
						"dashed-index": "goodbye",
					},
				},
			},
		},
		{
			expr: `{?'nested_map': optional.ofNonZeroValue({?'map': m.?c}), 'singleton': true}`,
			in: map[string]any{
				"m": map[string]any{},
			},
			out: map[string]any{
				"singleton": true,
			},
		},
		{
			expr: `[?m.?c, ?x, ?y]`,
			in: map[string]any{
				"m": map[string]any{},
				"x": types.OptionalOf(types.Int(42)),
				"y": types.OptionalNone,
			},
			out: []any{42},
		},
		{
			expr: `[?optional.ofNonZeroValue(m.?c.orValue({}))]`,
			in: map[string]any{
				"m": map[string]any{
					"c": []string{},
				},
			},
			out: []any{},
		},
		{
			expr: `optional.ofNonZeroValue({?'nested_map': optional.ofNonZeroValue({?'map': m.?c})})`,
			in: map[string]any{
				"m": map[string]any{},
			},
			out: types.OptionalNone,
		},
		{
			expr: `TestAllTypes{?single_double_wrapper: optional.ofNonZeroValue(0.0)}`,
			out:  &proto2pb.TestAllTypes{},
		},
		{
			expr: `optional.ofNonZeroValue(TestAllTypes{?single_double_wrapper: optional.ofNonZeroValue(0.0)})`,
			out:  types.OptionalNone,
		},
		{
			expr: `TestAllTypes{
				?map_string_string: m[?'nested']
			}`,
			in: map[string]any{
				"m": map[string]any{
					"nested": map[string]any{},
				},
			},
			out: &proto2pb.TestAllTypes{},
		},
		{
			expr: `TestAllTypes{
				?map_string_string: optional.ofNonZeroValue(m[?'nested'].orValue({}))
			}`,
			in: map[string]any{
				"m": map[string]any{
					"nested": map[string]any{},
				},
			},
			out: &proto2pb.TestAllTypes{},
		},
		{
			expr: `TestAllTypes{
				?map_string_string: m[?'nested']
			}`,
			in: map[string]any{
				"m": map[string]any{
					"nested": map[string]any{
						"hello": "world",
					},
				},
			},
			out: &proto2pb.TestAllTypes{
				MapStringString: map[string]string{"hello": "world"},
			},
		},
		{
			expr: `TestAllTypes{
				repeated_string: ['greetings', ?m.nested.?hello],
				?repeated_int32: optional.ofNonZeroValue([?x, ?y]),
			}`,
			in: map[string]any{
				"m": map[string]any{
					"nested": map[string]any{
						"hello": "world",
					},
				},
				"x": types.OptionalNone,
				"y": types.OptionalNone,
			},
			out: &proto2pb.TestAllTypes{
				RepeatedString: []string{"greetings", "world"},
			},
		},
		{expr: `[].first()`, out: types.OptionalNone},
		{expr: `['a','b','c'].first()`, out: types.OptionalOf(types.String("a"))},
		{expr: `[].last()`, out: types.OptionalNone},
		{expr: `[1, 2, 3].last()`, out: types.OptionalOf(types.Int(3))},
		{expr: `optional.unwrap([])`, out: []any{}},
		{expr: `optional.unwrap([optional.none(), optional.none()])`, out: []any{}},
		{expr: `optional.unwrap([optional.of(42), optional.none(), optional.of("a")])`, out: []any{types.Int(42), types.String("a")}},
		{expr: `optional.unwrap([optional.of(42), optional.of("a")])`, out: []any{types.Int(42), types.String("a")}},
		{expr: `[].unwrapOpt()`, out: []any{}},
		{expr: `[optional.none(), optional.none()].unwrapOpt()`, out: []any{}},
		{expr: `[optional.of(42), optional.none(), optional.of("a")].unwrapOpt()`, out: []any{types.Int(42), types.String("a")}},
		{expr: `[optional.of(42), optional.of("a")].unwrapOpt()`, out: []any{types.Int(42), types.String("a")}},
		{expr: `optional.of(optional.of(1)) != dyn(optional.of(1))`, out: types.True},
		{expr: `(true ? optional.of(optional.of(1)) : dyn(optional.of(2))) != dyn(optional.of(1))`, out: types.True},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%s/%d", tc.expr, i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("%v failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.in)
			if err != nil && err.Error() != tc.out {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.out)
			}
			want := adapter.NativeToValue(tc.out)
			if err == nil && out.Equal(want) != types.True {
				t.Errorf("prg.Eval() got %v, wanted %v", out, want)
			}
		})
	}
}

func unknownsEquivalent(t *testing.T, a *types.Unknown, b *types.Unknown) bool {
	t.Helper()
	return a.Contains(b) && b.Contains(a)
}

func TestOptionalValuesEvalUnknowns(t *testing.T) {
	env := testEnv(t,
		OptionalTypes(),
		// Container and test message types.
		Container("google.expr.proto2.test"),
		Types(&proto2pb.TestAllTypes{}),
		// Test variables.
		Variable("x", OptionalType(IntType)),
		Variable("y", OptionalType(IntType)),
		Variable("z", IntType),
	)

	tests := []struct {
		expr string
		in   map[string]any
		out  ref.Val
	}{
		{
			expr: `x.or(y).orValue(z)`,
			in: map[string]any{
				"y": types.OptionalNone,
				"z": 42,
			},
			out: types.NewUnknown(1, types.NewAttributeTrail("x")),
		},
		{
			expr: `x.or(y).orValue(z)`,
			in: map[string]any{
				"x": types.OptionalNone,
				"y": types.OptionalNone,
			},
			out: types.NewUnknown(5, types.NewAttributeTrail("z")),
		},
		{
			expr: `x.or(y).orValue(z)`,
			in: map[string]any{
				"x": types.OptionalOf(types.IntOne),
				"y": types.OptionalNone,
			},
			out: types.IntOne,
		},
		{
			expr: `x.or(y).orValue(z)`,
			in: map[string]any{
				"x": types.OptionalNone,
				"y": types.OptionalOf(types.IntOne),
			},
			out: types.IntOne,
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%s/%d", tc.expr, i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("%v failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast, EvalOptions(OptPartialEval))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			act, err := env.PartialVars(tc.in)
			if err != nil {
				t.Fatalf("env.PartialVars() returned error %v", err)
			}
			out, _, err := prg.Eval(act)
			if err != nil {
				t.Fatalf("prg.Eval() got error %v, wanted nil", err)
			}
			// unknowns don't define equality so special case.
			if wantUnk, ok := tc.out.(*types.Unknown); ok {
				if unk, ok := out.(*types.Unknown); !ok || !unknownsEquivalent(t, unk, wantUnk) {
					t.Errorf("prg.Eval() got %v, wanted %v", unk, tc.out)
				}
				return
			}
			if eq, ok := out.Equal(tc.out).Value().(bool); !ok || !eq {
				t.Errorf("prg.Eval() got %v, wanted %v", out, tc.out)
			}

		})
	}
}

func TestOptionalValuesEvalErrorCases(t *testing.T) {
	env := testEnv(t,
		OptionalTypes(),
		// Container and test message types.
		Container("google.expr.proto2.test"),
		Types(&proto2pb.TestAllTypes{}),
		// Test variables.
		Variable("x", OptionalType(IntType)),
		Variable("y", OptionalType(IntType)),
		Variable("z", IntType),
	)
	tests := []struct {
		expr    string
		in      map[string]any
		wantErr string
	}{
		{
			expr:    `dyn(1).or(optional.of(2))`,
			wantErr: "no such overload",
		},
		{
			expr:    `dyn(1).orValue(2)`,
			wantErr: "no such overload",
		},
		{
			expr:    `optional.of(1/0).or(optional.of(2))`,
			wantErr: "division by zero",
		},
		{
			expr:    `optional.of(1/0).orValue(2)`,
			wantErr: "division by zero",
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%s/%d", tc.expr, i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("%v failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Errorf("env.Program() failed: %v", err)
			}
			_, _, err = prg.Eval(tc.in)
			if err == nil {
				t.Fatalf("prg.Eval got nil error, wanted %v", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.wantErr)
			}
		})
	}
}

func TestEnableErrorOnBadPresenceTest(t *testing.T) {
	env := testEnv(t,
		OptionalTypes(),
		EnableErrorOnBadPresenceTest(true),
	)
	adapter := env.TypeAdapter()
	tests := []struct {
		expr string
		in   map[string]any
		out  any
	}{
		{
			expr: `{}.?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `{'null_field': dyn(null)}.?null_field`,
			out:  types.OptionalOf(types.NullValue),
		},
		{
			expr: `{'null_field': dyn(null)}.?null_field.?nested`,
			out:  "no such key: nested",
		},
		{
			expr: `{'zero_field': dyn(0)}.?zero_field.?invalid`,
			out:  "no such key: invalid",
		},
		{
			expr: `{0: dyn(0)}[?0].?invalid`,
			out:  "no such key: invalid",
		},
		{
			expr: `{true: dyn(0)}[?false].?invalid`,
			out:  types.OptionalNone,
		},
		{
			expr: `{true: dyn(0)}[?true].?invalid`,
			out:  "no such key: invalid",
		},
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("%v failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Errorf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.in)
			if err != nil && err.Error() != tc.out {
				t.Errorf("prg.Eval() got %v, wanted %v", err, tc.out)
			}
			want := adapter.NativeToValue(tc.out)
			if err == nil && out.Equal(want) != types.True {
				t.Errorf("prg.Eval() got %v, wanted %v", out, want)
			}
		})
	}

}

func TestOptionalMacroError(t *testing.T) {
	env := testEnv(t,
		OptionalTypes(),
		// Test variables.
		Variable("x", OptionalType(IntType)),
	)
	_, iss := env.Compile("x.optMap(y.z, y.z + 1)")
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "variable name must be a simple identifier") {
		t.Errorf("optMap() got an unexpected result: %v", iss.Err())
	}
	_, iss = env.Compile("x.optFlatMap(y.z, y.z + 1)")
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "variable name must be a simple identifier") {
		t.Errorf("optFlatMap() got an unexpected result: %v", iss.Err())
	}
	env = testEnv(t,
		OptionalTypes(OptionalTypesVersion(0)),
		// Test variables.
		Variable("x", OptionalType(IntType)),
	)
	_, iss = env.Compile("x.optFlatMap(y, y.z + 1)")
	if iss.Err() == nil || !strings.Contains(iss.Err().Error(), "undeclared reference to 'optFlatMap'") {
		t.Errorf("optFlatMap() got an unexpected result: %v", iss.Err())
	}
}

func TestParserExpressionSizeLimit(t *testing.T) {
	env := testEnv(t, ParserExpressionSizeLimit(10))
	_, iss := env.Parse("'greeting'")
	if iss.Err() != nil {
		t.Errorf("Parse('greeting') failed: %v", iss.Err())
	}
	_, iss = env.Parse("'greetings'")
	if !strings.Contains(iss.Err().Error(), "size exceeds limit") {
		t.Errorf("Parse('greetings') got unexpected error: %v", iss.Err())
	}
}

func BenchmarkOptionalValues(b *testing.B) {
	env := testEnv(b,
		OptionalTypes(),
		Variable("x", OptionalType(IntType)),
		Variable("y", OptionalType(IntType)),
		Variable("z", IntType),
	)
	ast, iss := env.Compile("x.or(y).orValue(z)")
	if iss.Err() != nil {
		b.Fatalf("env.Compile(x.or(y).orValue(z)) failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize))
	if err != nil {
		b.Errorf("env.Program() failed: %v", err)
	}
	input := map[string]any{
		"x": types.OptionalNone,
		"y": types.OptionalNone,
		"z": 42,
	}
	for i := 0; i < b.N; i++ {
		prg.Eval(input)
	}
}

func BenchmarkDynamicDispatch(b *testing.B) {
	env := testEnv(b,
		HomogeneousAggregateLiterals(),
		Function("first",
			MemberOverload("first_list_int", []*Type{ListType(IntType)}, IntType,
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.IntZero
					}
					return l.Get(types.IntZero)
				}),
			),
			MemberOverload("first_list_double", []*Type{ListType(DoubleType)}, DoubleType,
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.Double(0.0)
					}
					return l.Get(types.IntZero)
				}),
			),
			MemberOverload("first_list_string", []*Type{ListType(StringType)}, StringType,
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.String("")
					}
					return l.Get(types.IntZero)
				}),
			),
			MemberOverload("first_list_list_string", []*Type{ListType(ListType(StringType))}, ListType(StringType),
				UnaryBinding(func(list ref.Val) ref.Val {
					l := list.(traits.Lister)
					if l.Size() == types.IntZero {
						return types.DefaultTypeAdapter.NativeToValue([]string{})
					}
					return l.Get(types.IntZero)
				}),
			),
		),
	)
	prg := compile(b, env, `
		[].first() == 0
		&& [1, 2].first() == 1
		&& [1.0, 2.0].first() == 1.0
		&& ["hello", "world"].first() == "hello"
		&& [["hello"], ["world", "!"]].first().first() == "hello"`)
	prgDyn := compile(b, env, `
		dyn([]).first() == 0
		&& dyn([1, 2]).first() == 1
		&& dyn([1.0, 2.0]).first() == 1.0
		&& dyn(["hello", "world"]).first() == "hello"
		&& dyn([["hello"], ["world", "!"]]).first().first() == "hello"`)
	b.ResetTimer()
	b.Run("DirectDispatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			prg.Eval(NoVars())
		}
	})
	b.ResetTimer()
	b.Run("DynamicDispatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			prgDyn.Eval(NoVars())
		}
	})
}

func TestAstProgramNilValue(t *testing.T) {
	var ast *Ast = nil
	env := testEnv(t)
	prg, err := env.Program(ast)
	if err == nil || !strings.Contains(err.Error(), "unsupported expr") {
		t.Errorf("env.Program() got (%v,%v) wanted unsupported expr error", prg, err)
	}
}

func TestJSONFieldNames(t *testing.T) {
	tests := []struct {
		name           string
		expr           string
		jsonFieldNames bool
	}{
		{
			name: "proto simple field",
			expr: `msg.single_int32 == 1`,
		},
		{
			name: "proto map field",
			expr: `msg.map_string_string['key'] == 'value'`,
		},
		{
			name:           "json simple field",
			expr:           `msg.singleInt32 == 1`,
			jsonFieldNames: true,
		},
		{
			name:           "json repeated field",
			expr:           `msg.mapStringString['key'] == 'value'`,
			jsonFieldNames: true,
		},
		{
			name:           "message with json field",
			expr:           `TestAllTypes{singleInt32: 1} != msg`,
			jsonFieldNames: true,
		},
		{
			name:           "message with json field and proto fallback",
			expr:           `dyn(TestAllTypes{singleInt32: 2}).single_int32 == 2`,
			jsonFieldNames: true,
		},
		{
			name:           "json with proto fallback",
			expr:           `dyn(msg).single_int32 == dyn(msg).singleInt32`,
			jsonFieldNames: true,
		},
		{
			name:           "proto with extensions",
			expr:           `google.expr.proto2.test.ExampleType{fooBar: 'value'}.fooBar == 'value'`,
			jsonFieldNames: true,
		},
		{
			name: "json opt fields",
			expr: "jsonOptMsg.int32_snake_case_json_name == 1 && " +
				"jsonOptMsg.int64CamelCaseJsonName == 2 && " +
				"jsonOptMsg.uint32DefaultJsonName == 3u && " +
				"jsonOptMsg.`uint64-custom-json-name` == 4u && " +
				"jsonOptMsg.single_string == 'shadows' && " +
				"jsonOptMsg.singleString == 'shadowed'",
			jsonFieldNames: true,
		},
		{
			name: "json opt fields fallback",
			expr: "dyn(jsonOptMsg).int32_snake_case_json_name == 1 && " +
				"dyn(jsonOptMsg).`uint64-custom-json-name` == 4u && " +
				"dyn(jsonOptMsg).single_string == 'shadows' && " +
				"dyn(jsonOptMsg).string_json_name_shadows == 'shadows' && " +
				"dyn(jsonOptMsg).singleString == 'shadowed'",
			jsonFieldNames: true,
		},
	}
	msg := &proto3pb.TestAllTypes{
		SingleInt32: 1,
		MapStringString: map[string]string{
			"key": "value",
		},
	}
	jsonOptMsg := &proto3pb.TestJsonNames{
		Int32SnakeCaseJsonName: 1,
		Int64CamelCaseJsonName: 2,
		Uint32DefaultJsonName:  3,
		Uint64CustomJsonName:   4,
		StringJsonNameShadows:  "shadows",
		SingleString:           "shadowed",
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env, err := NewEnv(
				EnableIdentifierEscapeSyntax(),
				JSONFieldNames(tc.jsonFieldNames),
				Types(msg, &proto2pb.ExternalMessageType{}, jsonOptMsg),
				Container(string(msg.ProtoReflect().Descriptor().ParentFile().Package())),
				Variable("msg", ObjectType(string(msg.ProtoReflect().Descriptor().FullName()))),
				Variable("jsonOptMsg", ObjectType(string(jsonOptMsg.ProtoReflect().Descriptor().FullName()))),
			)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(map[string]any{"msg": msg, "jsonOptMsg": jsonOptMsg})
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out != types.True {
				t.Errorf("prg.Eval() got %v, wanted 'true'", out)
			}

			if tc.jsonFieldNames {
				noJSONEnv, err := env.Extend(JSONFieldNames(false))
				if err != nil {
					t.Fatalf("env.Extend() failed: %v", err)
				}
				_, err = noJSONEnv.Program(ast)
				if err == nil {
					t.Fatal("env with json disabled allowed program with json extension to be planned")
				}
			} else {
				jsonEnv, err := env.Extend(JSONFieldNames(true))
				if err != nil {
					t.Fatalf("env.Extend() failed: %v", err)
				}
				prg, err = jsonEnv.Program(ast)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(map[string]any{"msg": msg})
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out != types.True {
					t.Errorf("prg.Eval() got %v, wanted 'true'", out)
				}
			}
		})
	}
}

func TestJSONFieldNamesInvalidProvider(t *testing.T) {
	type wrapperRegistry struct {
		*types.Registry
	}
	reg, err := types.NewProtoRegistry(types.JSONFieldNames(true))
	if err != nil {
		t.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	wrapped := wrapperRegistry{Registry: reg}
	_, err = NewEnv(CustomTypeProvider(wrapped), CustomTypeAdapter(reg), JSONFieldNames(true))
	if err == nil {
		t.Error("NewEnv() created a CEL environment successfully despite incompatible configs")
	}
}

// TODO: ideally testCostEstimator and testRuntimeCostEstimator would be shared in a test fixtures package
type testCostEstimator struct {
	hints map[string]uint64
}

func (tc testCostEstimator) EstimateSize(element checker.AstNode) *checker.SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &checker.SizeEstimate{Min: 0, Max: l}
	}
	return nil
}

func (tc testCostEstimator) EstimateCallCost(function, overloadID string, target *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	return nil
}

func estimateTimestampToYear(estimator checker.CostEstimator, target *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	return &checker.CallEstimate{CostEstimate: checker.CostEstimate{Min: 7, Max: 7}}
}

type testRuntimeCostEstimator struct{}

var timeToYearCost uint64 = 7

func (e testRuntimeCostEstimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	argsSize := make([]uint64, len(args))
	for i, arg := range args {
		reflectV := reflect.ValueOf(arg.Value())
		switch reflectV.Kind() {
		// Note that the CEL bytes type is implemented with Go byte slices, therefore also supported by the following
		// code.
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			argsSize[i] = uint64(reflectV.Len())
		default:
			argsSize[i] = 1
		}
	}
	return nil
}

func trackTimestampToYear(args []ref.Val, result ref.Val) *uint64 {
	return &timeToYearCost
}

func testEnv(t testing.TB, opts ...EnvOption) *Env {
	t.Helper()
	e, err := NewEnv(opts...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	return e
}

func compile(t testing.TB, env *Env, expr string) Program {
	t.Helper()
	prg, err := compileOrError(t, env, expr)
	if err != nil {
		t.Fatal(err)
	}
	return prg
}

func compileOrError(t testing.TB, env *Env, expr string) (Program, error) {
	t.Helper()
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("env.Compile(%s) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize))
	if err != nil {
		return nil, fmt.Errorf("env.Program() failed: %v", err)
	}
	return prg, nil
}

func interpret(t testing.TB, env *Env, expr string, vars any) (ref.Val, error) {
	t.Helper()
	prg, err := compileOrError(t, env, expr)
	if err != nil {
		return nil, err
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("prg.Eval(%v) failed: %v", vars, err)
	}
	return out, nil
}

func TestExpressionSizeLimitEarlyEnforcement(t *testing.T) {
	env, err := NewEnv(ParserExpressionSizeLimit(1000))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	tests := []struct {
		name string
		mode string
	}{
		{name: "compile_rejects_oversized", mode: "compile"},
		{name: "parse_rejects_oversized", mode: "parse"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := strings.Repeat("a", 10_000_000)

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			switch tc.mode {
			case "compile":
				_, iss := env.Compile(payload)
				if iss == nil || iss.Err() == nil {
					t.Fatal("expected size limit error, got nil")
				}
				if !strings.Contains(iss.Err().Error(), "expression code point size exceeds limit") {
					t.Fatalf("unexpected error: %v", iss.Err())
				}
			case "parse":
				_, iss := env.Parse(payload)
				if iss == nil || iss.Err() == nil {
					t.Fatal("expected size limit error, got nil")
				}
				if !strings.Contains(iss.Err().Error(), "expression code point size exceeds limit") {
					t.Fatalf("unexpected error: %v", iss.Err())
				}
			}

			runtime.ReadMemStats(&m2)
			allocDelta := (m2.TotalAlloc - m1.TotalAlloc) / (1024 * 1024)
			if allocDelta > 5 {
				t.Errorf("excessive memory allocation: %dMiB during %s (expected <5MiB with early enforcement)",
					allocDelta, tc.mode)
			}
			t.Logf("[%s] memory delta: %dMiB", tc.mode, allocDelta)
		})
	}
}

func TestProgramEvalInvalidInput(t *testing.T) {
	env, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("true")
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}

	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "int input",
			input:   123,
			wantErr: "invalid input, wanted Activation or map[string]any",
		},
		{
			name:    "nil input",
			input:   nil,
			wantErr: "invalid input, wanted Activation or map[string]any",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := prg.Eval(tc.input)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Eval(%v) err = %v, expected error containing %q", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestProgramContextEvalInvalidInput(t *testing.T) {
	env, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("true")
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}

	tests := []struct {
		name    string
		ctx     context.Context
		input   any
		wantErr string
	}{
		{
			name:    "nil context",
			ctx:     nil,
			input:   map[string]any{},
			wantErr: "context can not be nil",
		},
		{
			name:    "invalid input type",
			ctx:     context.Background(),
			input:   123,
			wantErr: "invalid input, wanted Activation or map[string]any",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := prg.ContextEval(tc.ctx, tc.input)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("ContextEval(%v, %v) err = %v, expected error containing %q", tc.ctx, tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestOptionalOperatorsLegacyEval(t *testing.T) {
	tests := []struct {
		name string
		expr interpreter.Interpretable
		want ref.Val
	}{
		{
			name: "optional or",
			expr: &evalOptionalOr{
				id:  1,
				lhs: interpreter.NewConstValue(2, types.OptionalOf(types.True)),
				rhs: interpreter.NewConstValue(3, types.False),
			},
			want: types.OptionalOf(types.True),
		},
		{
			name: "optional or value mismatch",
			expr: &evalOptionalOrValue{
				id:  4,
				lhs: interpreter.NewConstValue(5, types.True),
				rhs: interpreter.NewConstValue(6, types.False),
			},
			want: types.NoSuchOverloadErr(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.expr.Eval(NoVars())
			if got.Equal(tc.want) != types.True && !types.IsError(got) {
				t.Errorf("Eval() = %v, wanted %v", got, tc.want)
			}
			if types.IsError(got) && got.(*types.Err).Error() != tc.want.(*types.Err).Error() {
				t.Errorf("Eval() = %v, wanted %v", got, tc.want)
			}
		})
	}
}
