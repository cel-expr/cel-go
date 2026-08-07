// Copyright 2025 Google LLC
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

package env

import (
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name string
		want *Config
	}{
		{
			name: "context_env.yaml",
			want: NewConfig("context-env").
				SetContainer("google.expr").
				AddImports(NewImport("google.expr.proto3.test.TestAllTypes")).
				SetStdLib(NewLibrarySubset().
					AddIncludedMacros("has").
					AddIncludedFunctions([]*Function{
						{Name: operators.Equals},
						{Name: operators.NotEquals},
						{Name: operators.LogicalNot},
						{Name: operators.Less},
						{Name: operators.LessEquals},
						{Name: operators.Greater},
						{Name: operators.GreaterEquals},
					}...)).
				AddExtensions(NewExtension("optional", math.MaxUint32), NewExtension("strings", 1)).
				SetContextVariable(NewContextVariable("google.expr.proto3.test.TestAllTypes")).
				AddFunctions(
					NewFunctionWithDoc("coalesce",
						"Converts a potentially null wrapper-type to a default value.",
						NewOverload("coalesce_wrapped_int",
							[]*TypeDesc{NewTypeDesc("google.protobuf.Int64Value"), NewTypeDesc("int")},
							NewTypeDesc("int"),
							`coalesce(null, 1) // 1`,
							`coalesce(2, 1) // 2`),
						NewOverload("coalesce_wrapped_double",
							[]*TypeDesc{NewTypeDesc("google.protobuf.DoubleValue"), NewTypeDesc("double")},
							NewTypeDesc("double"),
							`coalesce(null, 1.3) // 1.3`),
						NewOverload("coalesce_wrapped_uint",
							[]*TypeDesc{NewTypeDesc("google.protobuf.UInt64Value"), NewTypeDesc("uint")},
							NewTypeDesc("uint"),
							`coalesce(null, 14u) // 14u`),
					),
				),
		},
		{
			name: "extended_env.yaml",
			want: NewConfig("extended-env").
				SetContainer("google.expr").
				AddExtensions(
					NewExtension("optional", 2),
					NewExtension("math", math.MaxUint32),
				).AddVariables(
				NewVariableWithDoc("msg",
					NewTypeDesc("google.expr.proto3.test.TestAllTypes"),
					`msg represents all possible type permutation which CEL understands from a proto perspective`),
				NewVariableWithDoc("opt_msg",
					NewTypeDesc("optional_type", NewTypeDesc("google.expr.proto3.test.TestAllTypes")),
					`opt_msg represents all possible type permutation which CEL understands from a proto perspective`),
			).AddFunctions(
				NewFunctionWithDoc("isEmpty",
					common.MultilineDescription(
						`determines whether a list is empty,`,
						`or a string has no characters`),
					NewMemberOverload("wrapper_string_isEmpty",
						NewTypeDesc("google.protobuf.StringValue"), nil,
						NewTypeDesc("bool"),
						`''.isEmpty() // true`),
					NewMemberOverload("list_isEmpty",
						NewTypeDesc("list", NewTypeParam("T")), nil,
						NewTypeDesc("bool"),
						`[].isEmpty() // true`,
						`[1].isEmpty() // false`)),
				NewFunctionWithDoc("isEmptyAlt",
					common.MultilineDescription(
						`determines whether a list is empty,`,
						`or a string has no characters`),
					NewMemberOverload("wrapper_string_isEmpty",
						NewTypeDesc("string_wrapper"), nil,
						NewTypeDesc("bool"),
						`''.isEmptyAlt() // true`),
					NewMemberOverload("list_isEmpty",
						NewTypeDesc("list", NewTypeParam("T")), nil,
						NewTypeDesc("bool"),
						`[].isEmptyAlt() // true`,
						`[1].isEmptyAlt() // false`)),
				NewFunctionWithDoc(
					"getOrDefault",
					common.MultilineDescription(
						"Returns the value of a key in a map or the provided",
						"default value."),
					NewMemberOverload("map_getOrDefault",
						NewTypeDesc("map", NewTypeParam("K"), NewTypeParam("V")),
						[]*TypeDesc{
							NewTypeParam("K"),
							NewTypeParam("V"),
						},
						NewTypeParam("V"),
					)),
			).AddFeatures(
				NewFeature("cel.feature.macro_call_tracking", true),
			).AddLimits(
				NewLimit("cel.limit.parse_recursion_depth", 7),
			).AddValidators(
				NewValidator("cel.validator.duration"),
				NewValidator("cel.validator.matches"),
				NewValidator("cel.validator.timestamp"),
				NewValidator("cel.validator.comprehension_nesting_limit").
					SetConfig(map[string]any{"limit": 2}),
			),
		},
		{
			name: "json_env.json",
			want: NewConfig("extended-env").
				SetContainer("google.expr").
				AddExtensions(
					NewExtension("optional", 2),
					NewExtension("math", math.MaxUint32),
				).AddVariables(
				NewVariableWithDoc("msg",
					NewTypeDesc("google.expr.proto3.test.TestAllTypes"),
					`msg represents all possible type permutation which CEL understands from a proto perspective`),
				NewVariableWithDoc("opt_msg",
					NewTypeDesc("optional_type", NewTypeDesc("google.expr.proto3.test.TestAllTypes")),
					`opt_msg represents all possible type permutation which CEL understands from a proto perspective`),
			).AddFunctions(
				NewFunctionWithDoc("isEmpty",
					common.MultilineDescription(
						`determines whether a list is empty,`,
						`or a string has no characters`),
					NewMemberOverload("wrapper_string_isEmpty",
						NewTypeDesc("wrapper_string"), nil,
						NewTypeDesc("bool"),
						`''.isEmpty() // true`),
					NewMemberOverload("list_isEmpty",
						NewTypeDesc("list", NewTypeParam("T")), nil,
						NewTypeDesc("bool"),
						`[].isEmpty() // true`,
						`[1].isEmpty() // false`)),
				NewFunctionWithDoc(
					"getOrDefault",
					common.MultilineDescription(
						"Returns the value of a key in a map or the provided",
						"default value."),
					NewMemberOverload("map_getOrDefault",
						NewTypeDesc("map", NewTypeParam("K"), NewTypeParam("V")),
						[]*TypeDesc{
							NewTypeParam("K"),
							NewTypeParam("V"),
						},
						NewTypeParam("V"),
					)),
			).AddFeatures(
				NewFeature("cel.feature.macro_call_tracking", true),
			).AddLimits(
				NewLimit("cel.limit.parse_recursion_depth", 7),
			).AddValidators(
				NewValidator("cel.validator.duration"),
				NewValidator("cel.validator.matches"),
				NewValidator("cel.validator.timestamp"),
				NewValidator("cel.validator.comprehension_nesting_limit").
					SetConfig(map[string]any{"limit": 2}),
			),
		},
		{
			name: "subset_env.yaml",
			want: NewConfig("subset-env").
				SetStdLib(NewLibrarySubset().
					AddExcludedMacros("map", "filter").
					AddExcludedFunctions(
						[]*Function{
							{Name: operators.Add, Overloads: []*Overload{
								{ID: overloads.AddBytes},
								{ID: overloads.AddList},
								{ID: overloads.AddString},
							}},
							{Name: overloads.Matches},
							{Name: overloads.TypeConvertTimestamp, Overloads: []*Overload{
								{ID: overloads.StringToTimestamp},
							}},
							{Name: overloads.TypeConvertDuration, Overloads: []*Overload{
								{ID: overloads.StringToDuration},
							}},
						}...,
					)).AddVariables(
				NewVariable("x", NewTypeDesc("int")),
				NewVariable("y", NewTypeDesc("double")),
				NewVariable("z", NewTypeDesc("uint")),
			),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			fileName := fmt.Sprintf("testdata/%s", tc.name)
			data, err := os.ReadFile(fileName)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) failed: %v", fileName, err)
			}
			got := unmarshalYAML(t, data)
			if err := got.Validate(); err != nil {
				t.Errorf("Validate() got %v, wanted nil error", err)
			}
			if got.Container != tc.want.Container {
				t.Errorf("Container got %s, wanted %s", got.Container, tc.want.Container)
			}
			if !reflect.DeepEqual(got.Imports, tc.want.Imports) {
				t.Errorf("Imports got %v, wanted %v", got.Imports, tc.want.Imports)
			}
			if !reflect.DeepEqual(got.StdLib, tc.want.StdLib) {
				t.Errorf("StdLib got %v, wanted %v", got.StdLib, tc.want.StdLib)
			}
			if !reflect.DeepEqual(got.ContextVariable, tc.want.ContextVariable) {
				t.Errorf("ContextVariable got %v, wanted %v", got.ContextVariable, tc.want.ContextVariable)
			}
			if len(got.Variables) != len(tc.want.Variables) {
				t.Errorf("Variables count got %d, wanted %d", len(got.Variables), len(tc.want.Variables))
			} else {
				for i, v := range got.Variables {
					wv := tc.want.Variables[i]
					if !reflect.DeepEqual(v, wv) {
						t.Errorf("Variables[%d] not equal, got %v, wanted %v", i, v.Description, wv.Description)
					}
				}
			}
			if len(got.Functions) != len(tc.want.Functions) {
				t.Errorf("Functions count got %d, wanted %d", len(got.Functions), len(tc.want.Functions))
			} else {
				for i, f := range got.Functions {
					wf := tc.want.Functions[i]
					if f.Name != wf.Name {
						t.Errorf("Functions[%d] not equal, got %v, wanted %v", i, f.Name, wf.Name)
					}
					if f.Description != wf.Description {
						t.Errorf("Functions[%d] description not equal, got %v, wanted %v", i, f.Description, wf.Description)
					}
					if len(f.Overloads) != len(wf.Overloads) {
						t.Errorf("Function %s got overload count: %d, wanted %d", f.Name, len(f.Overloads), len(wf.Overloads))
					}
					for j, o := range f.Overloads {
						wo := wf.Overloads[j]
						if !reflect.DeepEqual(o, wo) {
							t.Error(o.Examples)
							t.Errorf("Overload[%d] got %v, wanted %v", j, o, wo)
						}
					}
				}
			}
			if len(got.Features) != len(tc.want.Features) {
				t.Errorf("Features count got %d, wanted %d", len(got.Features), len(tc.want.Features))
			} else {
				for i, f := range got.Features {
					wf := tc.want.Features[i]
					if f.Name != wf.Name {
						t.Errorf("Features[%d] got name %s, wanted %s", i, f.Name, wf.Name)
					}
					if f.Enabled != wf.Enabled {
						t.Errorf("Features[%d] got enabled %t, wanted %t", i, f.Enabled, wf.Enabled)
					}
				}
			}
			if len(got.Limits) != len(tc.want.Limits) {
				t.Errorf("Limits count got %d, wanted %d", len(got.Limits), len(tc.want.Limits))
			} else {
				for i, l := range got.Limits {
					wl := tc.want.Limits[i]
					if l.Name != wl.Name {
						t.Errorf("Limits[%d] got name %s, wanted %s", i, l.Name, wl.Name)
					}
					if l.Value != wl.Value {
						t.Errorf("Limits[%d] got enabled %d, wanted %d", i, l.Value, wl.Value)
					}
				}
			}
			if len(got.Validators) != len(tc.want.Validators) {
				t.Errorf("Validators count got %d, wanted %d", len(got.Validators), len(tc.want.Validators))
			} else {
				for i, f := range got.Validators {
					wf := tc.want.Validators[i]
					if f.Name != wf.Name {
						t.Errorf("Validators[%d] got name %s, wanted %s", i, f.Name, wf.Name)
					}
					if !reflect.DeepEqual(f.Config, wf.Config) {
						t.Errorf("Validators[%d] got enabled %v, wanted %v", i, f.Config, wf.Config)
					}
				}
			}
		})
	}
}

func TestConfigValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		in   *Config
		want error
	}{
		{
			name: "nil config valid",
		},
		{
			name: "invalid import",
			in:   NewConfig("invalid import").AddImports(NewImport("")),
			want: errors.New("invalid import"),
		},
		{
			name: "invalid subset",
			in:   NewConfig("invalid subset").SetStdLib(NewLibrarySubset().AddExcludedMacros("has").AddIncludedMacros("exists")),
			want: errors.New("invalid subset"),
		},
		{
			name: "invalid extension",
			in:   NewConfig("invalid extension").AddExtensions(NewExtension("", 0)),
			want: errors.New("invalid extension"),
		},
		{
			name: "invalid context variable",
			in:   NewConfig("invalid context variable").SetContextVariable(NewContextVariable("")),
			want: errors.New("invalid context variable"),
		},
		{
			name: "invalid variable",
			in:   NewConfig("invalid variable").AddVariables(NewVariable("", nil)),
			want: errors.New("invalid variable"),
		},
		{
			name: "invalid variable",
			in:   NewConfig("invalid variable").AddVariables(NewVariable("foo", NewTypeParam("X"))),
			want: errors.New("variables cannot be type parameters"),
		},
		{
			name: "colliding context variable",
			in: NewConfig("colliding context variable").
				SetContextVariable(NewContextVariable("msg.type.Name")).
				AddVariables(NewVariable("local", NewTypeDesc("string"))),
			want: errors.New("invalid config"),
		},
		{
			name: "invalid function",
			in:   NewConfig("invalid function").AddFunctions(NewFunction("", nil)),
			want: errors.New("invalid function"),
		},
		{
			name: "invalid feature",
			in:   NewConfig("invalid feature").AddFeatures(NewFeature("", false)),
			want: errors.New("invalid feature"),
		},
		{
			name: "invalid validator",
			in:   NewConfig("invalid validator").AddValidators(NewValidator("")),
			want: errors.New("invalid validator"),
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.Validate()
			if err == nil && tc.want == nil {
				return
			}
			if err == nil && tc.want != nil {
				t.Fatalf("config.Validate() got valid, wanted error %v", tc.want)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("config.Validate() got error %v, wanted nil error", err)
			}
			if !strings.Contains(err.Error(), tc.want.Error()) {
				t.Errorf("config.Validate() got error %v, wanted %v", err, tc.want)
			}
		})
	}
}

func TestConfigAddVariableDecls(t *testing.T) {
	tests := []struct {
		name string
		in   *decls.VariableDecl
		out  *Variable
	}{
		{
			name: "nil var decl",
		},
		{
			name: "simple var decl",
			in:   decls.NewVariable("var", types.StringType),
			out:  NewVariable("var", NewTypeDesc("string")),
		},
		{
			name: "parameterized var decl",
			in:   decls.NewVariable("var", types.NewListType(types.NewTypeParamType("T"))),
			out:  NewVariable("var", NewTypeDesc("list", NewTypeParam("T"))),
		},
		{
			name: "opaque var decl",
			in:   decls.NewVariable("var", types.NewOpaqueType("bitvector")),
			out:  NewVariable("var", NewTypeDesc("bitvector")),
		},
		{
			name: "proto var decl",
			in:   decls.NewVariable("var", types.NewObjectType("google.type.Expr")),
			out:  NewVariable("var", NewTypeDesc("google.type.Expr")),
		},
		{
			name: "proto var decl with doc",
			in:   decls.NewVariableWithDoc("var", types.NewObjectType("google.type.Expr"), "API-friendly CEL expression type"),
			out:  NewVariableWithDoc("var", NewTypeDesc("google.type.Expr"), "API-friendly CEL expression type"),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			conf := NewConfig(tc.name).AddVariableDecls(tc.in)
			if len(conf.Variables) != 1 {
				t.Fatalf("AddVariableDecls() did not add declaration to conf: %v", conf)
			}
			if !reflect.DeepEqual(conf.Variables[0], tc.out) {
				t.Errorf("AddVariableDecls() added %v, wanted %v", conf.Variables, tc.out)
			}
		})
	}
}

func TestConfigAddVariableDeclsEmpty(t *testing.T) {
	if len(NewConfig("").AddVariables().Variables) != 0 {
		t.Error("AddVariables() with no args failed")
	}
}

func TestConfigAddFunctionDecls(t *testing.T) {
	tests := []struct {
		name string
		in   *decls.FunctionDecl
		out  *Function
	}{
		{
			name: "nil function decl",
		},
		{
			name: "global function decl",
			in: mustNewFunction(t, "size",
				decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType),
			),
			out: NewFunction("size",
				NewOverload("size_string", []*TypeDesc{NewTypeDesc("string")}, NewTypeDesc("int")),
			),
		},
		{
			name: "global function decl - nullable arg",
			in: mustNewFunction(t, "size",
				decls.Overload("size_wrapper_string", []*types.Type{types.NewNullableType(types.StringType)}, types.IntType),
			),
			out: NewFunction("size",
				NewOverload("size_wrapper_string", []*TypeDesc{NewTypeDesc("google.protobuf.StringValue")}, NewTypeDesc("int")),
			),
		},
		{
			name: "member function decl - nullable arg",
			in: mustNewFunction(t, "size",
				decls.MemberOverload("list_size", []*types.Type{types.NewListType(types.NewTypeParamType("T"))}, types.IntType),
				decls.MemberOverload("string_size", []*types.Type{types.StringType}, types.IntType),
			),
			out: NewFunction("size",
				NewMemberOverload("list_size", NewTypeDesc("list", NewTypeParam("T")), []*TypeDesc{}, NewTypeDesc("int")),
				NewMemberOverload("string_size", NewTypeDesc("string"), []*TypeDesc{}, NewTypeDesc("int")),
			),
		},
		{
			name: "global function decl - with doc",
			in: mustNewFunction(t, "size",
				decls.FunctionDocs("return the number of unicode code points", "in a string"),
				decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType,
					decls.OverloadExamples(`'hello'.size() // 5`)),
			),
			out: NewFunctionWithDoc("size",
				"return the number of unicode code points\nin a string",
				NewOverload("size_string", []*TypeDesc{NewTypeDesc("string")}, NewTypeDesc("int"),
					`'hello'.size() // 5`),
			),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			conf := NewConfig(tc.name).AddFunctionDecls(tc.in)
			if len(conf.Functions) != 1 {
				t.Fatalf("AddFunctionDecls() did not add declaration to conf: %v", conf)
			}
			if !reflect.DeepEqual(conf.Functions[0], tc.out) {
				t.Errorf("AddFunctionDecls() added %v, wanted %v", conf.Functions, tc.out)
			}
		})
	}
}

func TestNewImport(t *testing.T) {
	imp := NewImport("qualified.type.name")
	if imp.Name != "qualified.type.name" {
		t.Errorf("NewImport() got name: %s, wanted %s", imp.Name, "qualified.type.name")
	}
}

func TestImportValidate(t *testing.T) {
	var imp *Import
	err := imp.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid import") {
		t.Errorf("imp.Validate() got %v, wanted error 'invalid import'", err)
	}

	imp = NewImport("")
	err = imp.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid import") {
		t.Errorf("imp.Validate() got %v, wanted error 'invalid import'", err)
	}
}

func TestNewContextVariable(t *testing.T) {
	ctx := NewContextVariable("qualified.type.name")
	if ctx.TypeName != "qualified.type.name" {
		t.Errorf("NewContextVariable() got name: %s, wanted %s", ctx.TypeName, "qualified.type.name")
	}
}

func TestContextVariableValidate(t *testing.T) {
	ctx := NewContextVariable("")
	err := ctx.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid context variable") {
		t.Errorf("ctx.Validate() got %v, wanted error 'invalid context variable'", err)
	}
}

func TestVariableGetType(t *testing.T) {
	tests := []struct {
		name string
		v    *Variable
		t    *TypeDesc
	}{
		{
			name: "nil-safety check",
			v:    nil,
			t:    nil,
		},
		{
			name: "nil type access",
			v:    &Variable{},
			t:    nil,
		},
		{
			name: "nested type desc",
			v:    &Variable{TypeDesc: &TypeDesc{}},
			t:    &TypeDesc{},
		},
		{
			name: "field type desc",
			v:    &Variable{Type: &TypeDesc{}},
			t:    &TypeDesc{},
		},
		{
			name: "nested type desc precedence",
			v: &Variable{
				TypeDesc: &TypeDesc{TypeName: "type.name.EmbeddedType"},
				Type:     &TypeDesc{TypeName: "type.name.FieldType"},
			},
			t: &TypeDesc{TypeName: "type.name.EmbeddedType"},
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.v.GetType(), tc.t) {
				t.Errorf("GetType() got %v, wanted %v", tc.v.GetType(), tc.t)
			}
		})
	}
}

func TestVariableAsCELVariable(t *testing.T) {
	tests := []struct {
		name string
		v    *Variable
		want any
	}{
		{
			name: "nil-safety check",
			v:    nil,
			want: errors.New("invalid variable: nil"),
		},
		{
			name: "no variable name",
			v:    &Variable{},
			want: errors.New("invalid variable"),
		},
		{
			name: "no type",
			v: &Variable{
				Name: "hello",
			},
			want: errors.New("invalid type: nil"),
		},
		{
			name: "bad type",
			v: &Variable{
				Name:     "hello",
				TypeDesc: &TypeDesc{},
			},
			want: errors.New("missing type name"),
		},
		{
			name: "undefined type",
			v: &Variable{
				Name:     "hello",
				TypeDesc: &TypeDesc{TypeName: "undefined"},
			},
			want: errors.New("undefined type name"),
		},
		{
			name: "simple type",
			v: &Variable{
				Name:     "t",
				TypeDesc: &TypeDesc{TypeName: "type"},
			},
			want: decls.NewVariable("t", types.TypeType),
		},
		{
			name: "type with param",
			v: &Variable{
				Name:     "t",
				TypeDesc: &TypeDesc{TypeName: "type", Params: []*TypeDesc{NewTypeParam("T")}},
			},
			want: decls.NewVariable("t", types.NewTypeTypeWithParam(types.NewTypeParamType("T"))),
		},
		{
			name: "type with too many params",
			v: &Variable{
				Name:     "t",
				TypeDesc: &TypeDesc{TypeName: "type", Params: []*TypeDesc{NewTypeParam("T"), NewTypeParam("U")}},
			},
			want: errors.New("expects 0 or 1 parameters"),
		},
		{
			name: "int type",
			v:    NewVariable("int_var", NewTypeDesc("int")),
			want: decls.NewVariable("int_var", types.IntType),
		},
		{
			name: "uint type",
			v: &Variable{
				Name:     "uint_var",
				TypeDesc: NewTypeDesc("uint"),
			},
			want: decls.NewVariable("uint_var", types.UintType),
		},
		{
			name: "dyn type",
			v: &Variable{
				Name:     "dyn_var",
				TypeDesc: NewTypeDesc("dyn"),
			},
			want: decls.NewVariable("dyn_var", types.DynType),
		},
		{
			name: "list type",
			v: &Variable{
				Name:     "list_var",
				TypeDesc: NewTypeDesc("list", NewTypeParam("T")),
			},
			want: decls.NewVariable("list_var", types.NewListType(types.NewTypeParamType("T"))),
		},
		{
			name: "map type",
			v: &Variable{
				Name: "map_var",
				TypeDesc: &TypeDesc{
					TypeName: "map",
					Params: []*TypeDesc{
						NewTypeDesc("string"),
						NewTypeDesc("optional_type", NewTypeParam("T")),
					},
				},
			},
			want: decls.NewVariable("map_var",
				types.NewMapType(types.StringType, types.NewOptionalType(types.NewTypeParamType("T")))),
		},
		{
			name: "set type",
			v: &Variable{
				Name:     "set_var",
				TypeDesc: NewTypeDesc("set", NewTypeDesc("string")),
			},
			want: decls.NewVariable("set_var", types.NewOpaqueType("set", types.StringType)),
		},
		{
			name: "string type - nested type precedence",
			v: &Variable{
				Name:     "hello",
				TypeDesc: NewTypeDesc("string"),
				Type:     NewTypeDesc("int"),
			},
			want: decls.NewVariable("hello", types.StringType),
		},
		{
			name: "wrapper type variable",
			v: &Variable{
				Name:     "msg",
				TypeDesc: NewTypeDesc("google.protobuf.StringValue"),
			},
			want: decls.NewVariable("msg", types.NewNullableType(types.StringType)),
		},
		{
			name: "wrapper type alias variable",
			v: &Variable{
				Name:     "msg",
				TypeDesc: NewTypeDesc("string_wrapper"),
			},
			want: decls.NewVariable("msg", types.NewNullableType(types.StringType)),
		},
	}

	tp, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}
	tp.RegisterType(types.NewOpaqueType("set", types.NewTypeParamType("T")))
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			gotVar, err := tc.v.AsCELVariable(tp)
			if err != nil {
				wantErr, ok := tc.want.(error)
				if !ok {
					t.Fatalf("AsCELVariable() got error %v, wanted %v", err, tc.want)
				}
				if !strings.Contains(err.Error(), wantErr.Error()) {
					t.Fatalf("AsCELVariable() got error %v, wanted error containing %v", err, wantErr)
				}
				return
			}
			if !gotVar.DeclarationIsEquivalent(tc.want.(*decls.VariableDecl)) {
				t.Errorf("AsCELVariable() got %v, wanted %v", gotVar, tc.want)
			}
		})
	}
}

func TestTypeDescString(t *testing.T) {
	tests := []struct {
		desc *TypeDesc
		want string
	}{
		{desc: NewTypeDesc("string"), want: "string"},
		{desc: NewTypeDesc("list", NewTypeParam("T")), want: "list(T)"},
		{desc: NewTypeDesc("type", NewTypeParam("T")), want: "type(T)"},
		{desc: NewTypeDesc("map", NewTypeDesc("string"), NewTypeParam("T")), want: "map(string,T)"},
	}
	for _, tc := range tests {
		if tc.desc.String() != tc.want {
			t.Errorf("String() got %s, wanted %s", tc.desc.String(), tc.want)
		}
	}
}

func TestFunctionAsCELFunction(t *testing.T) {
	tests := []struct {
		name string
		f    *Function
		want any
	}{
		{
			name: "nil function",
			f:    nil,
			want: errors.New("invalid function: nil"),
		},
		{
			name: "unnamed function",
			f:    &Function{},
			want: errors.New("invalid function"),
		},
		{
			name: "no overloads",
			f:    NewFunction("no_overloads"),
			want: errors.New("missing overloads"),
		},
		{
			name: "nil overload",
			f:    NewFunction("no_overloads", nil),
			want: errors.New("invalid overload: nil"),
		},
		{
			name: "missing overload id",
			f:    NewFunction("size", &Overload{}),
			want: errors.New("missing overload id"),
		},
		{
			name: "no return type",
			f: NewFunction("size",
				NewOverload("size_string", []*TypeDesc{NewTypeDesc("string")}, nil),
			),
			want: errors.New("return: invalid type"),
		},
		{
			name: "bad return type",
			f: NewFunction("size",
				NewOverload("size_string", []*TypeDesc{NewTypeDesc("string")}, NewTypeDesc("")),
			),
			want: errors.New("invalid type"),
		},
		{
			name: "bad arg type",
			f: NewFunction("size",
				NewOverload("size_string", []*TypeDesc{NewTypeDesc("")}, NewTypeDesc("")),
			),
			want: errors.New("invalid type"),
		},
		{
			name: "undefined arg type",
			f: NewFunction("size",
				NewOverload("size_undefined", []*TypeDesc{NewTypeDesc("undefined")}, NewTypeDesc("int")),
			),
			want: errors.New("undefined type"),
		},
		{
			name: "undefined return type",
			f: NewFunction("size",
				NewOverload("size_undefined", []*TypeDesc{NewTypeDesc("string")}, NewTypeDesc("undefined")),
			),
			want: errors.New("undefined type"),
		},
		{
			name: "undefined target type",
			f: NewFunction("size",
				NewMemberOverload("size_undefined", NewTypeDesc("undefined"), []*TypeDesc{NewTypeDesc("string")}, NewTypeDesc("int")),
			),
			want: errors.New("undefined type"),
		},
		{
			name: "bad target type",
			f: &Function{Name: "size",
				Overloads: []*Overload{
					{ID: "string_size",
						Target: &TypeDesc{},
						Args:   []*TypeDesc{},
						Return: &TypeDesc{TypeName: "int"},
					},
				},
			},
			want: errors.New("invalid type"),
		},
		{
			name: "global function",
			f: &Function{Name: "size",
				Overloads: []*Overload{
					{ID: "size_string",
						Args:   []*TypeDesc{{TypeName: "string"}},
						Return: &TypeDesc{TypeName: "int"}},
				},
			},
			want: mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
		},
		{
			name: "member function",
			f: &Function{Name: "size",
				Overloads: []*Overload{
					{ID: "string_size",
						Target: &TypeDesc{TypeName: "string"},
						Return: &TypeDesc{TypeName: "int"}},
				},
			},
			want: mustNewFunction(t, "size", decls.MemberOverload("string_size", []*types.Type{types.StringType}, types.IntType)),
		},
		{
			name: "member function with examples",
			f: &Function{Name: "size",
				Description: "return the number of unicode code points in a string",
				Overloads: []*Overload{{
					ID:     "string_size",
					Target: &TypeDesc{TypeName: "string"},
					Return: &TypeDesc{TypeName: "int"},
					Examples: []string{
						`'hello'.size() // 5`,
						`'hello world'.size() // 11`,
					},
				}},
			},
			want: mustNewFunction(t, "size",
				decls.FunctionDocs("return the number of unicode code points in a string"),
				decls.MemberOverload("string_size", []*types.Type{types.StringType}, types.IntType,
					decls.OverloadExamples(
						`'hello'.size() // 5`,
						`'hello world'.size() // 11`))),
		},
		{
			name: "function with type containing params",
			f: &Function{Name: "asType",
				Overloads: []*Overload{
					{ID: "bytes_asType_type(T)",
						Target: &TypeDesc{TypeName: "bytes"},
						Args:   []*TypeDesc{NewTypeDesc("type", NewTypeParam("T"))},
						Return: NewTypeParam("T")},
				},
			},
			want: mustNewFunction(t, "asType",
				decls.MemberOverload("bytes_asType_type(T)",
					[]*types.Type{types.BytesType, types.NewTypeTypeWithParam(types.NewTypeParamType("T"))},
					types.NewTypeParamType("T"))),
		},
	}
	tp, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}
	tp.RegisterType(types.NewOpaqueType("set", types.NewTypeParamType("T")))
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			gotFn, err := tc.f.AsCELFunction(tp)
			if err != nil {
				wantErr, ok := tc.want.(error)
				if !ok {
					t.Fatalf("AsCELFunction() got error %v, wanted %v", err, tc.want)
				}
				if !strings.Contains(err.Error(), wantErr.Error()) {
					t.Fatalf("AsCELFunction() got error %v, wanted error containing %v", err, wantErr)
				}
				return
			}
			assertFuncEquals(t, gotFn, tc.want.(*decls.FunctionDecl))
		})
	}
}

func TestTypeDescAsCELTypeErrors(t *testing.T) {
	tests := []struct {
		name string
		t    *TypeDesc
		want any
	}{
		{
			name: "nil-safety check",
			t:    nil,
			want: errors.New("invalid type: nil"),
		},
		{
			name: "no type name",
			t:    &TypeDesc{},
			want: errors.New("missing type name"),
		},
		{
			name: "invalid optional_type",
			t:    &TypeDesc{TypeName: "optional_type"},
			want: errors.New("expects 1 parameter"),
		},
		{
			name: "invalid optional param type",
			t:    &TypeDesc{TypeName: "optional_type", Params: []*TypeDesc{{}}},
			want: errors.New("invalid type"),
		},
		{
			name: "undefined optional param type",
			t:    &TypeDesc{TypeName: "optional_type", Params: []*TypeDesc{{TypeName: "undefined"}}},
			want: errors.New("undefined type"),
		},
		{
			name: "invalid param type",
			t:    &TypeDesc{TypeName: "T", IsTypeParam: true, Params: []*TypeDesc{{TypeName: "string"}}},
			want: errors.New("invalid type: param type"),
		},
		{
			name: "invalid list",
			t:    &TypeDesc{TypeName: "list"},
			want: errors.New("expects 1 parameter"),
		},
		{
			name: "invalid list param type",
			t:    &TypeDesc{TypeName: "list", Params: []*TypeDesc{{}}},
			want: errors.New("invalid type"),
		},
		{
			name: "undefined list param type",
			t:    &TypeDesc{TypeName: "list", Params: []*TypeDesc{{TypeName: "undefined"}}},
			want: errors.New("undefined type name"),
		},
		{
			name: "invalid map",
			t:    &TypeDesc{TypeName: "map"},
			want: errors.New("expects 2 parameters"),
		},
		{
			name: "invalid map key type",
			t:    &TypeDesc{TypeName: "map", Params: []*TypeDesc{{}, {}}},
			want: errors.New("invalid type"),
		},
		{
			name: "invalid map value type",
			t:    &TypeDesc{TypeName: "map", Params: []*TypeDesc{{TypeName: "string"}, {}}},
			want: errors.New("invalid type"),
		},
		{
			name: "undefined map key type",
			t:    &TypeDesc{TypeName: "map", Params: []*TypeDesc{{TypeName: "undefined"}, {TypeName: "undefined"}}},
			want: errors.New("undefined type name"),
		},
		{
			name: "undefined map value type",
			t:    &TypeDesc{TypeName: "map", Params: []*TypeDesc{{TypeName: "string"}, {TypeName: "undefined"}}},
			want: errors.New("undefined type name"),
		},
		{
			name: "invalid set",
			t:    &TypeDesc{TypeName: "set", Params: []*TypeDesc{{}}},
			want: errors.New("invalid type"),
		},
		{
			name: "undefined type identifier",
			t:    &TypeDesc{TypeName: "undefined"},
			want: errors.New("undefined type"),
		},
	}
	tp, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}
	tp.RegisterType(types.NewOpaqueType("set", types.NewTypeParamType("T")))
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			gotVar, err := tc.t.AsCELType(tp)
			if err != nil {
				wantErr, ok := tc.want.(error)
				if !ok {
					t.Fatalf("AsCELType() got error %v, wanted %v", err, tc.want)
				}
				if !strings.Contains(err.Error(), wantErr.Error()) {
					t.Fatalf("AsCELType() got error %v, wanted error containing %v", err, wantErr)
				}
				return
			}
			if !reflect.DeepEqual(gotVar, tc.want.(*decls.VariableDecl)) {
				t.Errorf("AsCELType() got %v, wanted %v", gotVar, tc.want)
			}
		})
	}
}

func TestLibrarySubsetValidate(t *testing.T) {
	tests := []struct {
		name string
		lib  *LibrarySubset
		want error
	}{
		{
			name: "nil library",
			lib:  NewLibrarySubset(),
		},
		{
			name: "empty library",
			lib:  NewLibrarySubset(),
		},
		{
			name: "only excluded funcs",
			lib:  NewLibrarySubset().AddExcludedFunctions(NewFunction("size", nil)),
		},
		{
			name: "only included funcs",
			lib:  NewLibrarySubset().AddIncludedFunctions(NewFunction("size", nil)),
		},
		{
			name: "only excluded macros",
			lib:  NewLibrarySubset().AddExcludedMacros("has"),
		},
		{
			name: "only included macros",
			lib:  NewLibrarySubset().AddIncludedMacros("exists"),
		},
		{
			name: "both included and excluded funcs",
			lib: NewLibrarySubset().
				AddIncludedFunctions(NewFunction("size", nil)).
				AddExcludedFunctions(NewFunction("size", nil)),
			want: errors.New("invalid subset"),
		},
		{
			name: "both included and excluded macros",
			lib: NewLibrarySubset().
				AddIncludedMacros("has").
				AddExcludedMacros("exists"),
			want: errors.New("invalid subset"),
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			err := tc.lib.Validate()
			if err == nil && tc.want == nil {
				return
			}
			if err == nil && tc.want != nil {
				t.Fatalf("lib.Validate() got valid, wanted error %v", tc.want)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("lib.Validate() got error %v, wanted nil error", err)
			}
			if !strings.Contains(err.Error(), tc.want.Error()) {
				t.Errorf("lib.Validate() got error %v, wanted %v", err, tc.want)
			}
		})
	}
}

func TestSubsetFunction(t *testing.T) {
	tests := []struct {
		name     string
		lib      *LibrarySubset
		orig     *decls.FunctionDecl
		subset   *decls.FunctionDecl
		included bool
	}{
		{
			name:     "nil lib, included",
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			subset:   mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: true,
		},
		{
			name:     "empty, included",
			lib:      NewLibrarySubset(),
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			subset:   mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: true,
		},
		{
			name:     "empty, disabled",
			lib:      NewLibrarySubset().SetDisabled(true),
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: false,
		},
		{
			name: "lib, not included allow-list",
			lib: NewLibrarySubset().AddIncludedFunctions([]*Function{
				{Name: "int"},
			}...),
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: false,
		},
		{
			name: "lib, included whole function",
			lib: NewLibrarySubset().AddIncludedFunctions([]*Function{
				{Name: "size"},
			}...),
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			subset:   mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: true,
		},
		{
			name: "lib, included overload subset",
			lib: NewLibrarySubset().AddIncludedFunctions([]*Function{
				{Name: "size", Overloads: []*Overload{{ID: "size_string"}}},
			}...),
			orig: mustNewFunction(t, "size",
				decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType),
				decls.Overload("size_list", []*types.Type{types.NewListType(types.NewTypeParamType("T"))}, types.IntType),
			),
			subset:   mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: true,
		},
		{
			name: "lib, included deny-list",
			lib: NewLibrarySubset().AddExcludedFunctions([]*Function{
				{Name: "int"},
			}...),
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			subset:   mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: true,
		},
		{
			name: "lib, excluded whole function",
			lib: NewLibrarySubset().AddExcludedFunctions([]*Function{
				{Name: "size"},
			}...),
			orig:     mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: false,
		},
		{
			name: "lib, excluded partial function",
			lib: NewLibrarySubset().AddExcludedFunctions([]*Function{
				{Name: "size", Overloads: []*Overload{{ID: "size_list"}}},
			}...),
			orig: mustNewFunction(t, "size",
				decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType),
				decls.Overload("size_list", []*types.Type{types.NewListType(types.NewTypeParamType("T"))}, types.IntType),
			),
			subset:   mustNewFunction(t, "size", decls.Overload("size_string", []*types.Type{types.StringType}, types.IntType)),
			included: true,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			got, included := tc.lib.SubsetFunction(tc.orig)
			if included != tc.included {
				t.Fatalf("SubsetFunction() got included %t, wanted %t", included, tc.included)
			}
			if !tc.included {
				return
			}
			assertFuncEquals(t, got, tc.subset)
		})
	}
}

func TestSubsetMacro(t *testing.T) {
	tests := []struct {
		name      string
		lib       *LibrarySubset
		macroName string
		included  bool
	}{
		{
			name:      "nil lib, included",
			macroName: "has",
			included:  true,
		},
		{
			name:      "empty, included",
			lib:       NewLibrarySubset(),
			macroName: "has",
			included:  true,
		},
		{
			name:      "empty, disabled",
			lib:       NewLibrarySubset().SetDisabled(true),
			macroName: "has",
			included:  false,
		},
		{
			name:      "empty, included",
			lib:       NewLibrarySubset().SetDisableMacros(true),
			macroName: "has",
			included:  false,
		},
		{
			name:      "lib, not included allow-list",
			lib:       NewLibrarySubset().AddIncludedMacros("exists"),
			macroName: "has",
			included:  false,
		},
		{
			name:      "lib, included allow-list",
			lib:       NewLibrarySubset().AddIncludedMacros("exists"),
			macroName: "exists",
			included:  true,
		},
		{
			name:      "lib, not included deny-list",
			lib:       NewLibrarySubset().AddExcludedMacros("exists"),
			macroName: "exists",
			included:  false,
		},
		{
			name:      "lib, included deny-list",
			lib:       NewLibrarySubset().AddExcludedMacros("exists"),
			macroName: "has",
			included:  true,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			included := tc.lib.SubsetMacro(tc.macroName)
			if included != tc.included {
				t.Fatalf("SubsetMacro() got included %t, wanted %t", included, tc.included)
			}
		})
	}
}

func TestNewExtension(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		want    *Extension
	}{
		{
			name:    "strings",
			version: math.MaxUint32,
			want:    &Extension{Name: "strings", Version: "latest"},
		},
		{
			name:    "bindings",
			version: 1,
			want:    &Extension{Name: "bindings", Version: "1"},
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			got := NewExtension(tc.name, tc.version)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NewExtension() got %v, wanted %v", got, tc.want)
			}
		})
	}
}

func TestExtensionGetVersion(t *testing.T) {
	tests := []struct {
		name string
		ext  *Extension
		want any
	}{
		{
			name: "nil extension",
			want: errors.New("invalid extension: nil"),
		},
		{
			name: "missing name",
			ext:  &Extension{},
			want: errors.New("missing name"),
		},
		{
			name: "unset version",
			ext:  &Extension{Name: "test"},
			want: uint32(0),
		},
		{
			name: "numeric version",
			ext:  &Extension{Name: "test", Version: "1"},
			want: uint32(1),
		},
		{
			name: "latest version",
			ext:  &Extension{Name: "test", Version: "latest"},
			want: uint32(math.MaxUint32),
		},
		{
			name: "bad version",
			ext:  &Extension{Name: "test", Version: "1.0"},
			want: errors.New("invalid syntax"),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ver, err := tc.ext.VersionNumber()
			if err != nil {
				wantErr, ok := tc.want.(error)
				if !ok {
					t.Fatalf("GetVersion() got error %v, wanted %v", err, tc.want)
				}
				if !strings.Contains(err.Error(), wantErr.Error()) {
					t.Fatalf("GetVersion() got error %v, wanted error containing %v", err, wantErr)
				}
				return
			}
			if tc.want.(uint32) != ver {
				t.Fatalf("GetVersion() got %d, wanted %v", ver, tc.want)
			}
		})
	}
}

func TestValidatorValidate(t *testing.T) {
	tests := []struct {
		name string
		v    *Validator
		want error
	}{
		{
			name: "nil validator",
			v:    nil,
			want: errors.New("invalid validator: nil"),
		},
		{
			name: "empty validator",
			v:    NewValidator(""),
			want: errors.New("missing name"),
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			err := tc.v.Validate()
			if err == nil && tc.want == nil {
				return
			}
			if err == nil && tc.want != nil {
				t.Fatalf("v.Validate() got valid, wanted error %v", tc.want)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("v.Validate() got error %v, wanted nil error", err)
			}
			if !strings.Contains(err.Error(), tc.want.Error()) {
				t.Errorf("v.Validate() got error %v, wanted %v", err, tc.want)
			}
		})
	}
}

func TestValidatorConfigValue(t *testing.T) {
	var v *Validator
	if _, found := v.ConfigValue("limit"); found {
		t.Error("v.ConfigValue() got value from nil validator")
	}
	v = NewValidator("validator").SetConfig(map[string]any{"limit": 2})
	if _, found := v.ConfigValue("absent"); found {
		t.Error("v.ConfigValue() found absent key")
	}
	if val, found := v.ConfigValue("limit"); !found || val != 2 {
		t.Errorf("v.ConfigValue() got %v, %t -- wanted 2, true", val, found)
	}
}

func TestFeatureValidate(t *testing.T) {
	tests := []struct {
		name string
		f    *Feature
		want error
	}{
		{
			name: "nil feature",
			f:    nil,
			want: errors.New("invalid feature: nil"),
		},
		{
			name: "empty feature",
			f:    NewFeature("", true),
			want: errors.New("missing name"),
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			err := tc.f.Validate()
			if err == nil && tc.want == nil {
				return
			}
			if err == nil && tc.want != nil {
				t.Fatalf("f.Validate() got valid, wanted error %v", tc.want)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("f.Validate() got error %v, wanted nil error", err)
			}
			if !strings.Contains(err.Error(), tc.want.Error()) {
				t.Errorf("f.Validate() got error %v, wanted %v", err, tc.want)
			}
		})
	}
}

func mustNewFunction(t *testing.T, name string, opts ...decls.FunctionOpt) *decls.FunctionDecl {
	t.Helper()
	fn, err := decls.NewFunction(name, opts...)
	if err != nil {
		t.Fatalf("decls.NewFunction() failed: %v", err)
	}
	return fn
}

func assertFuncEquals(t *testing.T, got, want *decls.FunctionDecl) {
	t.Helper()
	if got.Name() != want.Name() {
		t.Fatalf("got function name %s, wanted %s", got.Name(), want.Name())
	}
	if len(got.OverloadDecls()) != len(want.OverloadDecls()) {
		t.Fatalf("got overload count %d, wanted %d", len(got.OverloadDecls()), len(want.OverloadDecls()))
	}
	for i, gotOverload := range got.OverloadDecls() {
		wantOverload := want.OverloadDecls()[i]
		if gotOverload.ID() != wantOverload.ID() {
			t.Errorf("got overload id: %s, wanted: %s", gotOverload.ID(), wantOverload.ID())
		}
		if gotOverload.IsMemberFunction() != wantOverload.IsMemberFunction() {
			t.Errorf("got is member function %t, wanted %t", gotOverload.IsMemberFunction(), wantOverload.IsMemberFunction())
		}
		if len(gotOverload.ArgTypes()) != len(wantOverload.ArgTypes()) {
			t.Fatalf("got arg count %d, wanted %d", len(gotOverload.ArgTypes()), len(wantOverload.ArgTypes()))
		}
		for i, p := range gotOverload.ArgTypes() {
			wp := wantOverload.ArgTypes()[i]
			if !p.IsExactType(wp) {
				t.Errorf("got arg[%d] type %v, wanted %v", i, p, wp)
			}
		}
		if len(gotOverload.TypeParams()) != len(wantOverload.TypeParams()) {
			t.Fatalf("got type param count %d, wanted %d", len(gotOverload.TypeParams()), len(wantOverload.TypeParams()))
		}
		for i, p := range gotOverload.TypeParams() {
			wp := wantOverload.TypeParams()[i]
			if p != wp {
				t.Errorf("got type param[%d] %s, wanted %s", i, p, wp)
			}
		}
		if !gotOverload.ResultType().IsExactType(wantOverload.ResultType()) {
			t.Errorf("got result type %v, wanted %v", gotOverload.ResultType(), wantOverload.ResultType())
		}
	}
}

func unmarshalYAML(t *testing.T, data []byte) *Config {
	t.Helper()
	config, err := ConfigFromYAML(data)
	if err != nil {
		t.Fatalf("ConfigFromYAML(%q) failed: %v", string(data), err)
	}
	return config
}

func marshalYAML(t *testing.T, config *Config) []byte {
	t.Helper()
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("yaml.Marshal(%q) failed: %v", string(data), err)
	}
	return data
}
