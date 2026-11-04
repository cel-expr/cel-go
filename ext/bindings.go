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

package ext

import (
	"math"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
)

// Bindings returns a cel.EnvOption to configure support for local variable
// bindings in expressions.
//
// # Cel.Bind
//
// Binds a simple identifier to an initialization expression which may be used
// in a subsequenct result expression. Bindings may also be nested within each
// other.
//
//	cel.bind(<varName>, <initExpr>, <resultExpr>)
//
// Examples:
//
//	cel.bind(a, 'hello',
//	cel.bind(b, 'world', a + b + b + a)) // "helloworldworldhello"
//
//	// Avoid a list allocation within the exists comprehension.
//	cel.bind(valid_values, [a, b, c],
//	[d, e, f].exists(elem, elem in valid_values))
//
// Local bindings are not guaranteed to be evaluated before use.
func Bindings(options ...BindingsOption) cel.EnvOption {
	b := &celBindings{version: math.MaxUint32}
	for _, o := range options {
		b = o(b)
	}
	return cel.Lib(b)
}

const (
	celNamespace  = "cel"
	bindMacro     = "bind"
	unusedIterVar = "#unused"
)

// BindingsOption declares a functional operator for configuring the Bindings library behavior.
type BindingsOption func(*celBindings) *celBindings

// BindingsVersion sets the version of the bindings library to an explicit version.
func BindingsVersion(version uint32) BindingsOption {
	return func(lib *celBindings) *celBindings {
		lib.version = version
		return lib
	}
}

type celBindings struct {
	version uint32
}

func (*celBindings) LibraryName() string {
	return "cel.lib.ext.cel.bindings"
}

func (lib *celBindings) CompileOptions() []cel.EnvOption {
	opts := []cel.EnvOption{
		cel.Macros(
			// cel.bind(var, <init>, <expr>)
			cel.ReceiverMacro(bindMacro, 3, celBind),
		),
	}
	if lib.version >= 1 {
		// The cel.@block signature takes a list of subexpressions and a typed expression which is
		// used as the output type.
		paramType := cel.TypeParamType("T")
		opts = append(opts,
			cel.Function("cel.@block",
				cel.Overload("cel_block_list",
					[]*cel.Type{cel.ListType(cel.DynType), paramType}, paramType)),
		)
		opts = append(opts, cel.ASTValidators(blockValidationExemption{}))
	}
	return opts
}

func (lib *celBindings) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

type blockValidationExemption struct{}

// Name returns the name of the validator.
func (blockValidationExemption) Name() string {
	return "cel.validator.cel_block"
}

// Configure implements the ASTValidatorConfigurer interface and augments the list of functions to skip
// during homogeneous aggregate literal type-checks.
func (blockValidationExemption) Configure(config cel.MutableValidatorConfig) error {
	functions := config.GetOrDefault(cel.HomogeneousAggregateLiteralExemptFunctions, []string{}).([]string)
	functions = append(functions, "cel.@block")
	return config.Set(cel.HomogeneousAggregateLiteralExemptFunctions, functions)
}

// Validate is a no-op as the intent is to simply disable strong type-checks for list literals during
// when they occur within cel.@block calls as the arg types have already been validated.
func (blockValidationExemption) Validate(env *cel.Env, _ cel.ValidatorConfig, a *ast.AST, iss *cel.Issues) {
}

func celBind(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
	if !macroTargetMatchesNamespace(celNamespace, target) {
		return nil, nil
	}
	varIdent := args[0]
	varName := ""
	switch varIdent.Kind() {
	case ast.IdentKind:
		varName = varIdent.AsIdent()
	default:
		return nil, mef.NewError(varIdent.ID(), "cel.bind() variable names must be simple identifiers")
	}
	varInit := args[1]
	resultExpr := args[2]
	return mef.NewComprehension(
		mef.NewList(),
		unusedIterVar,
		varName,
		varInit,
		mef.NewLiteral(types.False),
		mef.NewIdent(varName),
		resultExpr,
	), nil
}
