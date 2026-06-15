// Copyright 2021 Google LLC
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
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	"google.golang.org/protobuf/proto"

	proto3pb "github.com/google/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestAstNil(t *testing.T) {
	var ast *Ast
	if ast.IsChecked() {
		t.Error("ast.IsChecked() returned true for nil ast")
	}
	if ast.Expr() != nil {
		t.Errorf("ast.Expr() got %v, wanted nil", ast.Expr())
	}
	if ast.SourceInfo() != nil {
		t.Errorf("ast.SourceInfo() got %v, wanted nil", ast.SourceInfo())
	}
	if ast.OutputType() != types.ErrorType {
		t.Errorf("ast.OutputType() got %v, wanted error type", ast.OutputType())
	}
	if ast.Source() != nil {
		t.Errorf("ast.Source() got %v, wanted nil", ast.Source())
	}
}

func TestIssuesNil(t *testing.T) {
	var iss *Issues
	iss = iss.Append(iss)
	if iss.Err() != nil {
		t.Errorf("iss.Err() got %v, wanted nil given nil issue set", iss.Err())
	}
	if len(iss.Errors()) != 0 {
		t.Errorf("iss.Errors() got %v, wanted empty value", iss.Errors())
	}
	if iss.String() != "" {
		t.Errorf("iss.String() returned %v, wanted empty value", iss.String())
	}
}

func TestIssuesEmpty(t *testing.T) {
	iss := NewIssues(common.NewErrors(nil))
	if iss.Err() != nil {
		t.Errorf("iss.Err() got %v, wanted nil given nil issue set", iss.Err())
	}
	if len(iss.Errors()) != 0 {
		t.Errorf("iss.Errors() got %v, wanted empty value", iss.Errors())
	}
	if iss.String() != "" {
		t.Errorf("iss.String() returned %v, wanted empty value", iss.String())
	}
	var iss2 *Issues
	iss3 := iss.Append(iss2)
	iss4 := iss3.Append(nil)
	if !reflect.DeepEqual(iss4, iss) {
		t.Error("Append() with a nil value resulted in the creation of a new issue set")
	}
}

func TestErrorAsIssues(t *testing.T) {
	iss := ErrorAsIssues(errors.New("wrapped-error"))
	if iss.Err() == nil {
		t.Fatal("iss.Err() got nil, wanted 'wrapped-error'")
	}
	if !strings.Contains(iss.Err().Error(), "wrapped-error") {
		t.Errorf("iss.Err() got %v, wanted 'wrapped-error'", iss.Err())
	}
}

func TestIssuesAppendSelf(t *testing.T) {
	e, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	_, iss := e.Compile("a")
	if len(iss.Errors()) != 1 {
		t.Errorf("iss.Errors() got %v, wanted 1 error", iss.Errors())
	}
	iss = iss.Append(iss)
	if len(iss.Errors()) != 1 {
		t.Errorf("iss.Errors() got %v, wanted 1 error", iss.Errors())
	}
}

func TestIssues(t *testing.T) {
	e, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	_, iss := e.Compile("-")
	_, iss2 := e.Compile("b")
	iss = iss.Append(iss2)
	if len(iss.Errors()) != 3 {
		t.Errorf("iss.Errors() got %v, wanted 3 errors", iss.Errors())
	}

	wantIss := `ERROR: <input>:1:1: undeclared reference to 'b' (in container '')
 | -
 | ^
ERROR: <input>:1:2: Syntax error: no viable alternative at input '-'
 | -
 | .^
ERROR: <input>:1:2: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
 | -
 | .^`
	if iss.String() != wantIss {
		t.Errorf("iss.String() returned %v, wanted %v", iss.String(), wantIss)
	}
}

func TestFormatCELTypeEquivalence(t *testing.T) {
	values := []*Type{
		AnyType,
		MapType(StringType, DynType),
		types.NewTypeTypeWithParam(ListType(IntType)),
		TypeType,
		NullableType(DoubleType),
	}
	for _, v := range values {
		v := v
		t.Run(v.String(), func(t *testing.T) {
			celStr := FormatCELType(v)
			et, err := TypeToExprType(v)
			if err != nil {
				t.Fatalf("TypeToExprType(%v) failed: %v", v, err)
			}
			exprStr := FormatType(et)
			if celStr != exprStr {
				t.Errorf("FormatCELType(%v) got %s, wanted %s", v, celStr, exprStr)
			}
		})
	}
}

func TestEnvCheckExtendRace(t *testing.T) {
	t.Parallel()
	for i := 0; i < 500; i++ {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		env, err := NewCustomEnv(StdLib())
		if err != nil {
			t.Fatalf("NewCustomEnv() failed: %v", err)
		}
		t.Run(fmt.Sprintf("Compile[%d]", i), func(t *testing.T) {
			go func() {
				defer wg.Done()
				_, _ = env.Compile(`1 + 1 * 20 < 400`)
			}()
		})
		t.Run(fmt.Sprintf("Extend[%d]", i), func(t *testing.T) {
			go func() {
				defer wg.Done()
				_, _ = env.Extend(Variable("bar", BoolType))
			}()
		})
		wg.Wait()
	}
}

func TestEnvPartialVarsError(t *testing.T) {
	env := testEnv(t)
	_, err := env.PartialVars(10)
	if err == nil {
		t.Error("env.PartialVars(10) succeeded, wanted error")
	}
}

func TestTypeProviderInterop(t *testing.T) {
	reg, err := types.NewProtoRegistry(types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("types.NewProtoRegistry() failed: %v", err)
	}
	tests := []struct {
		name     string
		provider any
	}{
		{
			name:     "custom provider",
			provider: &customCELProvider{provider: reg},
		},
		{
			name:     "custom legacy provider",
			provider: &customLegacyProvider{provider: reg},
		},
		{
			name:     "provider",
			provider: reg,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			env, err := NewEnv(CustomTypeProvider(tc.provider))
			if err != nil {
				t.Fatalf("NewEnv(CustomTypeProvider()) failed: %v", err)
			}
			// Found type
			pbType, found := env.TypeProvider().FindType("google.expr.proto3.test.TestAllTypes")
			if !found {
				t.Fatal("FindType(google.expr.proto3.test.TestAllTypes) failed")
			}
			celType, found := env.CELTypeProvider().FindStructType("google.expr.proto3.test.TestAllTypes")
			if !found {
				t.Fatal("FindStructType(google.expr.proto3.test.TestAllTypes) failed")
			}
			pbConvType, err := types.ExprTypeToType(pbType)
			if err != nil {
				t.Fatalf("types.ExprTypeToType(%v) failed: %v", pbType, err)
			}
			if !celType.IsExactType(pbConvType) {
				t.Errorf("got converted type %v, wanted %v", pbConvType, celType)
			}
			// Found field
			pbFieldType, found := env.TypeProvider().FindFieldType("google.expr.proto3.test.TestAllTypes", "single_int32")
			if !found {
				t.Fatal("FindFieldType(google.expr.proto3.test.TestAllTypes, single_int32) not found")
			}
			celFieldType, found := env.CELTypeProvider().FindStructFieldType("google.expr.proto3.test.TestAllTypes", "single_int32")
			if !found {
				t.Fatal("FindStructFieldType(google.expr.proto3.test.TestAllTypes, single_int32) not found")
			}
			pbConvFieldType, err := types.ExprTypeToType(pbFieldType.Type)
			if err != nil {
				t.Fatalf("types.ExprTypeToType(%v) failed: %v", pbFieldType.Type, err)
			}
			if !celFieldType.Type.IsExactType(pbConvFieldType) {
				t.Errorf("got converted field type %v, wanted %v", pbConvFieldType, celFieldType)
			}
			// Not found type
			_, found = env.TypeProvider().FindType("test.BadTypeName")
			if found {
				t.Fatal("FindType(test.BadTypeName) succeeded")
			}
			_, found = env.CELTypeProvider().FindStructType("test.BadTypeName")
			if found {
				t.Fatal("FindStructType(test.BadTypeName) succeeded")
			}
			// Not found field
			_, found = env.TypeProvider().FindFieldType("google.expr.proto3.test.TestAllTypes", "undefined_field")
			if found {
				t.Fatal("FindFieldType(google.expr.proto3.test.TestAllTypes, undefined_field) was found")
			}
			_, found = env.CELTypeProvider().FindStructFieldType("google.expr.proto3.test.TestAllTypes", "undefined_field")
			if found {
				t.Fatal("FindStructFieldType(google.expr.proto3.test.TestAllTypes, undefined_field) not found")
			}
		})
	}
}

func TestLibraries(t *testing.T) {
	e, err := NewEnv(OptionalTypes())
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, expected := range []string{"cel.lib.std", "cel.lib.optional"} {
		if !e.HasLibrary(expected) {
			t.Errorf("Expected HasLibrary() to return true for '%s'", expected)
		}
		libMap := map[string]struct{}{}
		libraries := e.Libraries()
		for _, lib := range libraries {
			libMap[lib] = struct{}{}
		}
		if len(libraries) != 2 {
			t.Errorf("Expected HasLibrary() to contain exactly 2 libraries but got: %v", libraries)
		}

		if _, ok := libMap[expected]; !ok {
			t.Errorf("Expected Libraries() to include '%s'", expected)
		}
	}
}

func TestFunctions(t *testing.T) {
	e, err := NewEnv(OptionalTypes())
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, expected := range []string{"optional.of", "or"} {
		if !e.HasFunction(expected) {
			t.Errorf("Expected HasFunction() to return true for '%s'", expected)
		}

		if _, ok := e.Functions()[expected]; !ok {
			t.Errorf("Expected Functions() to include '%s'", expected)
		}
	}
}

func TestEnvToConfig(t *testing.T) {
	tests := []struct {
		name string
		opts []EnvOption
		want *env.Config
	}{
		{
			name: "std env",
			want: env.NewConfig("std env"),
		},
		{
			name: "std env - container",
			opts: []EnvOption{
				Container("example.container"),
			},
			want: env.NewConfig("std env - container").SetContainer("example.container"),
		},
		{
			name: "std env - aliases",
			opts: []EnvOption{
				Abbrevs("example.type.name"),
			},
			want: env.NewConfig("std env - aliases").AddImports(env.NewImport("example.type.name")),
		},
		{
			name: "std env disabled",
			opts: []EnvOption{
				func(*Env) (*Env, error) {
					return NewCustomEnv()
				},
			},
			want: env.NewConfig("std env disabled").SetStdLib(
				env.NewLibrarySubset().SetDisabled(true)),
		},
		{
			name: "std env - with variable",
			opts: []EnvOption{
				Variable("var", IntType),
			},
			want: env.NewConfig("std env - with variable").AddVariables(env.NewVariable("var", env.NewTypeDesc("int"))),
		},
		{
			name: "std env - with function",
			opts: []EnvOption{Function("hello", Overload("hello_string", []*Type{StringType}, StringType))},
			want: env.NewConfig("std env - with function").AddFunctions(
				env.NewFunction("hello",
					env.NewOverload("hello_string",
						[]*env.TypeDesc{env.NewTypeDesc("string")}, env.NewTypeDesc("string")),
				)),
		},
		{
			name: "optional lib",
			opts: []EnvOption{
				OptionalTypes(),
			},
			want: env.NewConfig("optional lib").AddExtensions(env.NewExtension("optional", math.MaxUint32)),
		},
		{
			name: "optional lib - versioned",
			opts: []EnvOption{
				OptionalTypes(OptionalTypesVersion(1)),
			},
			want: env.NewConfig("optional lib - versioned").AddExtensions(env.NewExtension("optional", 1)),
		},
		{
			name: "optional lib - alt last()",
			opts: []EnvOption{
				OptionalTypes(),
				Function("last",
					FunctionDocs(`return the last value in a list, or last character in a string`),
					MemberOverload("string_last", []*Type{StringType}, StringType)),
			},
			want: env.NewConfig("optional lib - alt last()").
				AddExtensions(env.NewExtension("optional", math.MaxUint32)).
				AddFunctions(env.NewFunctionWithDoc("last",
					`return the last value in a list, or last character in a string`,
					env.NewMemberOverload("string_last", env.NewTypeDesc("string"), []*env.TypeDesc{}, env.NewTypeDesc("string")),
				)),
		},
		{
			name: "json field names",
			opts: []EnvOption{
				JSONFieldNames(true),
			},
			want: env.NewConfig("json field names").
				AddFeatures(env.NewFeature("cel.feature.json_field_names", true)),
		},
		{
			name: "context proto - with extra variable",
			opts: []EnvOption{
				DeclareContextProto((&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor()),
				Variable("extra", StringType),
			},
			want: env.NewConfig("context proto - with extra variable").
				SetContextVariable(env.NewContextVariable("google.expr.proto3.test.TestAllTypes")).
				AddVariables(env.NewVariable("extra", env.NewTypeDesc("string"))),
		},
		{
			name: "context proto",
			opts: []EnvOption{
				DeclareContextProto((&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor()),
			},
			want: env.NewConfig("context proto").SetContextVariable(env.NewContextVariable("google.expr.proto3.test.TestAllTypes")),
		},
		{
			name: "feature flags",
			opts: []EnvOption{
				DefaultUTCTimeZone(false),
				EnableMacroCallTracking(),
			},
			want: env.NewConfig("feature flags").AddFeatures(
				env.NewFeature("cel.feature.macro_call_tracking", true),
			),
		},
		{
			name: "validators",
			opts: []EnvOption{
				ExtendedValidations(),
				ASTValidators(ValidateComprehensionNestingLimit(1)),
			},
			want: env.NewConfig("validators").AddValidators(
				env.NewValidator("cel.validator.comprehension_nesting_limit").SetConfig(map[string]any{"limit": 1}),
				env.NewValidator("cel.validator.duration"),
				env.NewValidator("cel.validator.homogeneous_literals"),
				env.NewValidator("cel.validator.matches"),
				env.NewValidator("cel.validator.timestamp"),
			),
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			e, err := NewEnv(tc.opts...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			gotConfig, err := e.ToConfig(tc.name)
			if err != nil {
				t.Fatalf("ToConfig() failed: %v", err)
			}
			if !reflect.DeepEqual(gotConfig, tc.want) {
				t.Errorf("e.Config() got %v, wanted %v", gotConfig, tc.want)
			}
		})
	}
}

func TestEnvFromConfig(t *testing.T) {
	type exprCase struct {
		name string
		in   any
		expr string
		iss  error
		out  ref.Val
	}
	tests := []struct {
		name         string
		beforeOpts   []EnvOption
		afterOpts    []EnvOption
		conf         *env.Config
		confHandlers []ConfigOptionFactory
		exprs        []exprCase
	}{
		{
			name: "std env",
			conf: env.NewConfig("std env"),
			exprs: []exprCase{
				{
					name: "literal",
					expr: "'hello world'",
					out:  types.String("hello world"),
				},
				{
					name: "size",
					expr: "'hello world'.size()",
					out:  types.Int(11),
				},
			},
		},
		{
			name:       "std env - imports",
			beforeOpts: []EnvOption{Types(&proto3pb.TestAllTypes{})},
			conf: env.NewConfig("std env - imports").
				AddImports(env.NewImport("google.expr.proto3.test.TestAllTypes")),
			exprs: []exprCase{
				{
					name: "literal",
					expr: "TestAllTypes{single_int64: 15}.single_int64",
					out:  types.Int(15),
				},
			},
		},
		{
			name:       "std env - context proto",
			beforeOpts: []EnvOption{Types(&proto3pb.TestAllTypes{})},
			conf: env.NewConfig("std env - context proto").
				SetContainer("google.expr.proto3.test").
				SetContextVariable(env.NewContextVariable("google.expr.proto3.test.TestAllTypes")),
			exprs: []exprCase{
				{
					name: "field select literal",
					in:   mustContextProto(t, &proto3pb.TestAllTypes{SingleInt64: 10}),
					expr: "TestAllTypes{single_int64: single_int64}.single_int64",
					out:  types.Int(10),
				},
			},
		},
		{
			name:       "std env - context proto w/ json field names",
			beforeOpts: []EnvOption{Types(&proto3pb.TestAllTypes{})},
			conf: env.NewConfig("std env - context proto w/ json field names").
				SetContainer("google.expr.proto3.test").
				SetContextVariable(env.NewContextVariable("google.expr.proto3.test.TestAllTypes")).
				AddFeatures(env.NewFeature("cel.feature.json_field_names", true)),
			exprs: []exprCase{
				{
					name: "field select literal",
					in:   mustContextProto(t, &proto3pb.TestAllTypes{SingleInt64: 10}, types.JSONFieldNames(true)),
					expr: "TestAllTypes{singleInt64: singleInt64}.singleInt64",
					out:  types.Int(10),
				},
			},
		},
		{
			name:       "std env - wrapper type aliases",
			beforeOpts: []EnvOption{Types(&proto3pb.TestAllTypes{})},
			conf: env.NewConfig("std env - wrapper_type_aliases").
				AddVariables(env.NewVariable("single_int64_wrapper", env.NewTypeDesc("int_wrapper")),
					env.NewVariable("single_double_wrapper", env.NewTypeDesc("double_wrapper"))),

			exprs: []exprCase{
				{
					name: "eq_int_wrapper",
					in:   map[string]any{"single_int64_wrapper": 42},
					expr: "single_int64_wrapper != null && single_int64_wrapper == 42",
					out:  types.True,
				},
				{
					name: "eq_double_wrapper",
					in:   map[string]any{"single_double_wrapper": 42.1},
					expr: "single_double_wrapper != null && single_double_wrapper == 42.1",
					out:  types.True,
				},
			},
		},
		{
			name:       "custom env - variables",
			beforeOpts: []EnvOption{Types(&proto3pb.TestAllTypes{})},
			conf: env.NewConfig("custom env - variables").
				SetStdLib(env.NewLibrarySubset().SetDisabled(true)).
				SetContainer("google.expr.proto3.test").
				AddVariables(env.NewVariable("single_int64", env.NewTypeDesc("int"))),
			exprs: []exprCase{
				{
					name: "field select literal",
					in:   map[string]any{"single_int64": 42},
					expr: "TestAllTypes{single_int64: single_int64}.single_int64",
					out:  types.Int(42),
				},
				{
					name: "invalid operator",
					in:   map[string]any{"single_int64": 42},
					expr: "TestAllTypes{single_int64: single_int64}.single_int64 + 1",
					iss:  errors.New("undeclared reference"),
				},
			},
		},
		{
			name: "custom env - functions",
			afterOpts: []EnvOption{
				Function("plus",
					MemberOverload("int_plus_int", []*Type{IntType, IntType}, IntType,
						BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
							l := lhs.(types.Int)
							r := rhs.(types.Int)
							return l + r
						}),
					),
				)},
			conf: env.NewConfig("custom env - functions").
				SetStdLib(env.NewLibrarySubset().SetDisabled(true)).
				AddVariables(env.NewVariable("x", env.NewTypeDesc("int"))).
				AddFunctions(env.NewFunction("plus",
					env.NewMemberOverload("int_plus_int",
						env.NewTypeDesc("int"),
						[]*env.TypeDesc{env.NewTypeDesc("int")},
						env.NewTypeDesc("int"),
					),
				)),
			exprs: []exprCase{
				{
					name: "plus",
					in:   map[string]any{"x": 42},
					expr: "x.plus(2)",
					out:  types.Int(44),
				},
				{
					name: "plus invalid type",
					in:   map[string]any{"x": 42},
					expr: "x.plus(2.0)",
					iss:  errors.New("no matching overload"),
				},
			},
		},
		{
			name: "custom env - variable to constant",
			afterOpts: []EnvOption{
				Constant("foo.bar", types.IntType, types.Int(42)),
			},
			conf: env.NewConfig("custom env - functions").
				AddVariables(env.NewVariable("foo.bar", env.NewTypeDesc("int"))),
			exprs: []exprCase{
				{
					name: "valid",
					expr: "foo.bar == 42",
					out:  types.True,
				},
			},
		},
		{
			name: "custom env - variable to compatible constants",
			beforeOpts: []EnvOption{
				Constant("foo.bar", types.IntType, types.Int(42)),
			},
			afterOpts: []EnvOption{
				Constant("foo.bar", types.IntType, types.Int(42)),
			},
			conf: env.NewConfig("custom env - functions").
				AddVariables(env.NewVariable("foo.bar", env.NewTypeDesc("int"))),
			exprs: []exprCase{
				{
					name: "valid",
					expr: "foo.bar == 42",
					out:  types.True,
				},
			},
		},
		{
			name: "pure custom env",
			beforeOpts: []EnvOption{func(*Env) (*Env, error) {
				return NewCustomEnv()
			}},
			conf: env.NewConfig("pure custom env").SetStdLib(
				env.NewLibrarySubset().AddIncludedFunctions([]*env.Function{{Name: "_==_"}}...),
			),
			exprs: []exprCase{
				{
					name: "equals",
					expr: "'hello world' == 'hello'",
					out:  types.False,
				},
				{
					name: "not equals - invalid",
					expr: "'hello world' != 'hello'",
					iss:  errors.New("undeclared reference"),
				},
			},
		},
		{
			name: "std env - allow subset",
			conf: env.NewConfig("std env - allow subset").SetStdLib(
				env.NewLibrarySubset().AddIncludedFunctions([]*env.Function{{Name: "_==_"}}...),
			),
			exprs: []exprCase{
				{
					name: "equals",
					expr: "'hello world' == 'hello'",
					out:  types.False,
				},
				{
					name: "not equals - invalid",
					expr: "'hello world' != 'hello'",
					iss:  errors.New("undeclared reference"),
				},
			},
		},
		{
			name: "std env - deny subset",
			conf: env.NewConfig("std env - deny subset").SetStdLib(
				env.NewLibrarySubset().AddExcludedFunctions([]*env.Function{{Name: "size"}}...),
			),
			exprs: []exprCase{
				{
					name: "size - invalid",
					expr: "'hello world'.size()",
					iss:  errors.New("undeclared reference"),
				},
				{
					name: "equals",
					expr: "'hello world' == 'hello'",
					out:  types.False,
				},
			},
		},
		{
			name: "extensions",
			conf: env.NewConfig("extensions").
				AddVariables(
					env.NewVariable("m",
						env.NewTypeDesc("map", env.NewTypeDesc("string"), env.NewTypeDesc("string")))).
				AddExtensions(env.NewExtension("optional", math.MaxUint32)),
			exprs: []exprCase{
				{
					name: "optional none",
					expr: "optional.none()",
					out:  types.OptionalNone,
				},
				{
					name: "optional key",
					expr: "m.?key.hasValue()",
					in:   map[string]any{"m": map[string]string{"key": "value"}},
					out:  types.True,
				},
			},
		},
		{
			name: "extensions - config factory",
			conf: env.NewConfig("extensions").
				AddExtensions(env.NewExtension("plus", math.MaxUint32)),
			confHandlers: []ConfigOptionFactory{
				func(a any) (EnvOption, bool) {
					ext, ok := a.(*env.Extension)
					if !ok || ext.Name != "plus" {
						return nil, false
					}
					return Function("plus", Overload("plus_int_int", []*Type{IntType, IntType}, IntType,
						decls.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
							l := lhs.(types.Int)
							r := rhs.(types.Int)
							return l + r
						}))), true
				},
			},
			exprs: []exprCase{
				{
					name: "plus",
					expr: "plus(1, 2)",
					out:  types.Int(3),
				},
			},
		},
		{
			name: "features",
			conf: env.NewConfig("features").
				AddVariables(
					env.NewVariable("m",
						env.NewTypeDesc("map", env.NewTypeDesc("string"), env.NewTypeDesc("string")))).
				AddFeatures(
					env.NewFeature("cel.feature.backtick_escape_syntax", true),
					env.NewFeature("cel.feature.unknown_feature_name", true)),
			exprs: []exprCase{
				{
					name: "optional key",
					expr: "m.`key-name` == 'value'",
					in:   map[string]any{"m": map[string]string{"key-name": "value"}},
					out:  types.True,
				},
			},
		},
		{
			name: "limits_recursion",
			conf: env.NewConfig("limits").
				AddLimits(env.NewLimit("cel.limit.parse_recursion_depth", 3)),
			exprs: []exprCase{
				{
					name: "under limit",
					expr: "1 + 2",
					out:  types.Int(3),
				},
				{
					name: "over limit",
					expr: "1 + 2 + 3 + 4 + 5",
					iss:  errors.New("max recursion depth exceeded"),
				},
			},
		},
		{
			name: "limits_codepoints",
			conf: env.NewConfig("limits").
				AddLimits(env.NewLimit("cel.limit.expression_code_points", 10)),
			exprs: []exprCase{
				{
					name: "under limit",
					expr: "'12345'",
					out:  types.String("12345"),
				},
				{
					name: "over limit",
					expr: "'1234567890'",
					iss:  errors.New("code point size exceeds limit: size: 12, limit 10"),
				},
			},
		},
		{
			name: "limits_error_recovery",
			conf: env.NewConfig("limits").
				AddLimits(env.NewLimit("cel.limit.parse_error_recovery", 4)),
			exprs: []exprCase{
				{
					name: "over limit",
					expr: "a ? b ((?))",
					iss:  errors.New("Syntax error: error recovery attempt limit exceeded: 4"),
				},
			},
		},
		{
			name: "validators",
			conf: env.NewConfig("validators").
				AddVariables(
					env.NewVariable("m",
						env.NewTypeDesc("map", env.NewTypeDesc("string"), env.NewTypeDesc("string"))),
				).
				AddValidators(
					env.NewValidator(durationValidatorName),
					env.NewValidator(timestampValidatorName),
					env.NewValidator(regexValidatorName),
					env.NewValidator(homogeneousValidatorName),
					env.NewValidator(nestingLimitValidatorName).SetConfig(map[string]any{"limit": 0}),
				),
			exprs: []exprCase{
				{
					name: "bad duration",
					expr: "duration('1')",
					iss:  errors.New("invalid duration"),
				},
				{
					name: "bad timestamp",
					expr: "timestamp('1')",
					iss:  errors.New("invalid timestamp"),
				},
				{
					name: "bad regex",
					expr: "'hello'.matches('?^()')",
					iss:  errors.New("invalid matches"),
				},
				{
					name: "mixed type list",
					expr: "[1, 2.0]",
					iss:  errors.New("expected type 'int'"),
				},
				{
					name: "disabled comprehension",
					expr: "[1, 2].exists(x, x % 2 == 0)",
					iss:  errors.New("comprehension exceeds nesting limit"),
				},
			},
		},
		{
			name: "validators - config factory",
			conf: env.NewConfig("validators").
				AddValidators(
					env.NewValidator("cel.validators.return_type").SetConfig(map[string]any{"type_name": "string"}),
				),
			confHandlers: []ConfigOptionFactory{
				func(a any) (EnvOption, bool) {
					val, ok := a.(*env.Validator)
					if !ok || val.Name != "cel.validators.return_type" {
						return nil, false
					}
					typeName, found := val.ConfigValue("type_name")
					if !found {
						return func(*Env) (*Env, error) {
							return nil, fmt.Errorf("invalid validator: %s  missing config parameter 'type_name'", val.Name)
						}, true
					}
					return func(e *Env) (*Env, error) {
						t, err := env.NewTypeDesc(typeName.(string)).AsCELType(e.CELTypeProvider())
						if err != nil {
							return nil, err
						}
						return ASTValidators(returnTypeValidator{returnType: t})(e)
					}, true
				},
			},
			exprs: []exprCase{
				{
					name: "string - ok",
					expr: "'hello'",
					out:  types.String("hello"),
				},
				{
					name: "int - error",
					expr: "1",
					iss:  errors.New("unsupported return type: int, want string"),
				},
			},
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.beforeOpts
			opts = append(opts, FromConfig(tc.conf, tc.confHandlers...))
			opts = append(opts, tc.afterOpts...)
			var e *Env
			var err error
			if tc.conf.StdLib != nil {
				e, err = NewCustomEnv(opts...)
			} else {
				e, err = NewEnv(opts...)
			}
			if err != nil {
				t.Fatalf("NewEnv(FromConfig()) failed: %v", err)
			}
			for _, ex := range tc.exprs {
				t.Run(ex.name, func(t *testing.T) {
					ast, iss := e.Compile(ex.expr)
					if iss.Err() != nil {
						if ex.iss == nil || !strings.Contains(iss.Err().Error(), ex.iss.Error()) {
							t.Errorf("e.Compile() failed with %v, wanted %v", iss.Err(), ex.iss)
						}
						return
					}
					if ex.iss != nil {
						t.Fatalf("e.Compile() succeeded, wanted error %v", ex.iss)
					}
					prg, err := e.Program(ast)
					if err != nil {
						t.Fatalf("e.Program() failed: %v", err)
					}
					var in any = map[string]any{}
					if ex.in != nil {
						in = ex.in
					}
					out, _, err := prg.Eval(in)
					if err != nil {
						t.Fatalf("prg.Eval() failed: %v", err)
					}
					if out.Equal(ex.out) != types.True {
						t.Errorf("prg.Eval() got %v, wanted %v", out, ex.out)
					}
				})
			}
		})
	}
}

func TestEnvFromConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		conf *env.Config
		want error
	}{
		{
			name: "bad container",
			conf: env.NewConfig("bad container").SetContainer(".hello.world"),
			want: errors.New("container name must not contain"),
		},
		{
			name: "colliding imports",
			conf: env.NewConfig("colliding imports").AddImports(env.NewImport("pkg.ImportName"), env.NewImport("pkg2.ImportName")),
			want: errors.New("abbreviation collides"),
		},
		{
			name: "invalid subset",
			conf: env.NewConfig("invalid subset").SetStdLib(env.NewLibrarySubset().SetDisableMacros(true)),
			want: errors.New("invalid subset"),
		},
		{
			name: "invalid import",
			conf: env.NewConfig("invalid import").AddImports(env.NewImport("")),
			want: errors.New("invalid import"),
		},
		{
			name: "invalid context proto",
			conf: env.NewConfig("invalid context proto").SetContextVariable(env.NewContextVariable("invalid")),
			want: errors.New("invalid context proto type"),
		},
		{
			name: "undefined variable type",
			conf: env.NewConfig("undefined variable type").AddVariables(env.NewVariable("undef", env.NewTypeDesc("undefined"))),
			want: errors.New("invalid variable"),
		},
		{
			name: "undefined function type",
			conf: env.NewConfig("undefined function type").AddFunctions(env.NewFunction("invalid", env.NewOverload("invalid", []*env.TypeDesc{}, env.NewTypeDesc("undefined")))),
			want: errors.New("invalid function"),
		},
		{
			name: "unrecognized extension",
			conf: env.NewConfig("unrecognized extension").
				AddExtensions(env.NewExtension("unrecognized", math.MaxUint32)),
			want: errors.New("unrecognized extension"),
		},
		{
			name: "invalid validator config",
			conf: env.NewConfig("invalid validator config").
				AddValidators(env.NewValidator("cel.validator.comprehension_nesting_limit")),
			want: errors.New("invalid validator"),
		},
		{
			name: "invalid validator config type - unsupported type",
			conf: env.NewConfig("invalid validator config").
				AddValidators(env.NewValidator("cel.validator.comprehension_nesting_limit").SetConfig(map[string]any{"limit": "2"})),
			want: errors.New("invalid validator"),
		},
		{
			name: "invalid validator config type - fractional",
			conf: env.NewConfig("invalid validator config").
				AddValidators(env.NewValidator("cel.validator.comprehension_nesting_limit").SetConfig(map[string]any{"limit": 2.5})),
			want: errors.New("invalid validator: cel.validator.comprehension_nesting_limit, limit value is not a whole number: 2.5"),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEnv(FromConfig(tc.conf))
			if err == nil || !strings.Contains(err.Error(), tc.want.Error()) {
				t.Fatalf("NewEnv(FromConfig()) got %v, wanted error containing %v", err, tc.want)
			}
		})
	}
}

func TestEnvVariableValidation(t *testing.T) {
	tcs := []struct {
		name    string
		opts    []EnvOption
		wantErr string
	}{
		{
			name: "compatible_duplicate",
			opts: []EnvOption{
				Variable("foo", IntType),
				Variable("foo", IntType),
			},
		},
		{
			name: "variable_and_constant",
			opts: []EnvOption{
				Variable("foo", IntType),
				Constant("foo", IntType, types.Int(50)),
			},
		},
		{
			name: "compatible_constant",
			opts: []EnvOption{
				Constant("foo", StringType, types.String("foo")),
				Constant("foo", StringType, types.String("foo")),
			},
		},
		{
			name: "incompatible_variable",
			opts: []EnvOption{
				Variable("foo", IntType),
				Variable("foo", DoubleType),
			},
			wantErr: "overlapping identifier for name",
		},
		{
			name: "incompatible_constant",
			opts: []EnvOption{
				Constant("foo", IntType, types.Int(42)),
				Constant("foo", IntType, types.Int(43)),
			},
			wantErr: "conflicting constant definitions",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(*testing.T) {
			env, err := NewEnv(tc.opts...)
			if err != nil {
				t.Fatalf("NewEnv() returned error: %v", err)
			}
			// Variables are lazily validated.
			_, iss := env.Compile("foo")
			err = iss.Err()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Compile() returned error %v, wanted %s", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("Compile() returned error %v wanted nil", err)
			}
		})
	}
}

func BenchmarkNewCustomEnvLazy(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewCustomEnv(StdLib(), EagerlyValidateDeclarations(false))
		if err != nil {
			b.Fatalf("NewCustomEnv() failed: %v", err)
		}
	}
}

func BenchmarkNewCustomEnvEager(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewCustomEnv(StdLib(), EagerlyValidateDeclarations(true))
		if err != nil {
			b.Fatalf("NewCustomEnv() failed: %v", err)
		}
	}
}

func BenchmarkNewEnvLazy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewEnv()
		if err != nil {
			b.Fatalf("NewEnv() failed: %v", err)
		}
	}
}

func BenchmarkNewEnvEager(b *testing.B) {
	for i := 0; i < b.N; i++ {
		env, err := NewEnv()
		if err != nil {
			b.Fatalf("NewEnv() failed: %v", err)
		}
		_, iss := env.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func BenchmarkEnvExtendEager(b *testing.B) {
	env, err := NewEnv()
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		ext, err := env.Extend()
		if err != nil {
			b.Fatalf("env.Extend() failed: %v", err)
		}
		_, iss := ext.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func BenchmarkEnvExtendEagerTypes(b *testing.B) {
	env, err := NewEnv(EagerlyValidateDeclarations(true))
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		ext, err := env.Extend(CustomTypeProvider(types.NewEmptyRegistry()))
		if err != nil {
			b.Fatalf("env.Extend() failed: %v", err)
		}
		_, iss := ext.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func BenchmarkEnvExtendEagerDecls(b *testing.B) {
	env, err := NewEnv(EagerlyValidateDeclarations(true))
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		ext, err := env.Extend(
			Variable("test_var", StringType),
			Function(
				operators.In,
				Overload("string_in_string", []*Type{StringType, StringType}, BoolType),
			),
		)
		if err != nil {
			b.Fatalf("env.Extend() failed: %v", err)
		}
		_, iss := ext.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func mustContextProto(t *testing.T, pb proto.Message, opts ...types.RegistryOption) Activation {
	t.Helper()
	ctx, err := ContextProtoVars(pb, opts...)
	if err != nil {
		t.Fatalf("ContextProtoVars() failed: %v", err)
	}
	return ctx
}

type returnTypeValidator struct {
	returnType *Type
}

func (returnTypeValidator) Name() string {
	return "cel.validators.return_type"
}

func (v returnTypeValidator) Validate(_ *Env, c ValidatorConfig, a *ast.AST, iss *Issues) {
	if a.GetType(a.Expr().ID()) != v.returnType {
		iss.ReportErrorAtID(a.Expr().ID(),
			"unsupported return type: %s, want %s",
			a.GetType(a.Expr().ID()), v.returnType.TypeName())
	}
}

func (v returnTypeValidator) ToConfig() *env.Validator {
	return env.NewValidator(v.Name()).SetConfig(map[string]any{"type_name": v.returnType.TypeName()})
}

type customLegacyProvider struct {
	provider ref.TypeProvider
}

func (p *customLegacyProvider) EnumValue(enumName string) ref.Val {
	return p.provider.EnumValue(enumName)
}

func (p *customLegacyProvider) FindIdent(identName string) (ref.Val, bool) {
	return p.provider.FindIdent(identName)
}

func (p *customLegacyProvider) FindType(typeName string) (*exprpb.Type, bool) {
	return p.provider.FindType(typeName)
}

func (p *customLegacyProvider) FindFieldType(structType, fieldName string) (*ref.FieldType, bool) {
	return p.provider.FindFieldType(structType, fieldName)
}

func (p *customLegacyProvider) NewValue(structType string, fields map[string]ref.Val) ref.Val {
	return p.provider.NewValue(structType, fields)
}

type customCELProvider struct {
	provider types.Provider
}

func (p *customCELProvider) EnumValue(enumName string) ref.Val {
	return p.provider.EnumValue(enumName)
}

func (p *customCELProvider) FindIdent(identName string) (ref.Val, bool) {
	return p.provider.FindIdent(identName)
}

func (p *customCELProvider) FindStructType(typeName string) (*types.Type, bool) {
	return p.provider.FindStructType(typeName)
}

func (p *customCELProvider) FindStructFieldNames(typeName string) ([]string, bool) {
	return p.provider.FindStructFieldNames(typeName)
}

func (p *customCELProvider) FindStructFieldType(structType, fieldName string) (*types.FieldType, bool) {
	return p.provider.FindStructFieldType(structType, fieldName)
}

func (p *customCELProvider) NewValue(structType string, fields map[string]ref.Val) ref.Val {
	return p.provider.NewValue(structType, fields)
}

func TestCELTypeAdapter(t *testing.T) {
	env, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	adapter := env.CELTypeAdapter()
	if adapter == nil {
		t.Error("CELTypeAdapter() returned nil")
	}
}

type mockTypeProvider struct {
	ref.TypeProvider
}

func TestMaybeInteropProvider_Error(t *testing.T) {
	_, err := maybeInteropProvider(123)
	if err == nil {
		t.Error("maybeInteropProvider(int) should return error")
	}
}

func TestMaybeInteropProvider_LegacyTypeProvider(t *testing.T) {
	tp := &mockTypeProvider{}
	p, err := maybeInteropProvider(tp)
	if err != nil {
		t.Fatalf("maybeInteropProvider(mockTypeProvider) failed: %v", err)
	}
	if _, ok := p.(*interopCELTypeProvider); !ok {
		t.Errorf("expected *interopCELTypeProvider, got %T", p)
	}
}

func TestParserErrorRecoveryLimit(t *testing.T) {
	env, err := NewEnv(ParserErrorRecoveryLimit(10))
	if err != nil {
		t.Fatalf("NewEnv(ParserErrorRecoveryLimit(10)) failed: %v", err)
	}
	if env.limits[limitParseErrorRecovery] != 10 {
		t.Errorf("limitParseErrorRecovery = %d, want 10", env.limits[limitParseErrorRecovery])
	}
}

func TestEnableHiddenAccumulatorName(t *testing.T) {
	_, err := NewEnv(EnableHiddenAccumulatorName(true))
	if err != nil {
		t.Fatalf("NewEnv(EnableHiddenAccumulatorName(true)) failed: %v", err)
	}
}

func TestDeclareContextProto_Duplicate(t *testing.T) {
	desc := (&proto3pb.TestAllTypes{}).ProtoReflect().Descriptor()
	env, err := NewEnv(DeclareContextProto(desc))
	if err != nil {
		t.Fatalf("NewEnv(DeclareContextProto()) failed: %v", err)
	}
	_, err = DeclareContextProto(desc)(env)
	if err == nil {
		t.Error("DeclareContextProto() twice should fail")
	}
}
