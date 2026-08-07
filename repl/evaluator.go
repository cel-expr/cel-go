// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package repl defines a set of utilities for working with command line processing of CEL.
package repl

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/env"
	envlib "github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.yaml.in/yaml/v3"

	test2pb "cel.dev/expr/conformance/proto2"
	test3pb "cel.dev/expr/conformance/proto3"
	attrpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	descpb "google.golang.org/protobuf/types/descriptorpb"
)

var (
	extensionMap = map[string]cel.EnvOption{
		"optional":               cel.OptionalTypes(),
		"bindings":               ext.Bindings(),
		"strings":                ext.Strings(),
		"protos":                 ext.Protos(),
		"math":                   ext.Math(),
		"encoders":               ext.Encoders(),
		"sets":                   ext.Sets(),
		"lists":                  ext.Lists(),
		"maps":                   ext.Maps(),
		"two_var_comprehensions": ext.TwoVarComprehensions(),
	}
)

// letVariable let variable representation
type letVariable struct {
	identifier string
	src        string
	typeHint   *env.TypeDesc

	// memoized results from building the expression AST and program
	resultType *env.TypeDesc
	env        *cel.Env
	ast        *cel.Ast
	prog       cel.Program
}

type letFunctionParam struct {
	identifier string
	typeHint   *env.TypeDesc
}

// letFunction coordinates let function data (type definition and CEL function implementation).
type letFunction struct {
	identifier string
	src        string
	resultType *env.TypeDesc
	params     []letFunctionParam
	receiver   *env.TypeDesc // if not nil indicates an instance function

	// memoized results from building the expression AST and program
	env   *cel.Env // the context env for repl evaluation
	fnEnv *cel.Env // the env for implementing the extension fn
	prog  cel.Program
	impl  functions.FunctionOp
}

func runtimeTypeAgrees(env *cel.Env, val ref.Val, declType *envlib.TypeDesc) bool {
	to, err := declType.AsCELType(env.CELTypeProvider())

	if err != nil {
		return val.Type().TypeName() == declType.TypeName
	}
	return to.IsAssignableRuntimeType(val)
}

func typeAssignable(env *cel.Env, rtType ref.Type, declType *envlib.TypeDesc) bool {
	// This is not totally correct (we don't bind parameter types correctly), but
	// good enough for the repl.
	from, ok := rtType.(*types.Type)
	to, err := declType.AsCELType(env.CELTypeProvider())

	if !ok || err != nil {
		return rtType.TypeName() == declType.TypeName
	}

	return to.IsAssignableType(from)
}

func checkArgsMatch(env *cel.Env, params []letFunctionParam, args []ref.Val) error {
	if len(params) != len(args) {
		return fmt.Errorf("got %d args, expected %d", len(args), len(params))
	}
	for i, arg := range args {
		if !runtimeTypeAgrees(env, arg, params[i].typeHint) {
			return fmt.Errorf("got %s, expected %s for argument %d", arg.Type().TypeName(), params[i].typeHint.SpecifierFormat(), i)
		}
	}
	return nil
}

func makeParamVar(env *cel.Env, id string, t *envlib.TypeDesc) (*decls.VariableDecl, error) {
	rt, err := t.AsCELType(env.CELTypeProvider())
	if err != nil {
		return nil, err
	}
	return decls.NewVariable(id, rt), nil
}

func (l *letFunction) updateImpl(env *cel.Env) error {
	// declaration but no binding.
	if l.src == "" {
		l.impl = nil
		return nil
	}
	var paramVars []*decls.VariableDecl

	if l.receiver != nil {
		decl, err := makeParamVar(env, "this", l.receiver)

		if err != nil {
			return err
		}
		paramVars = append(paramVars, decl)
	}

	for _, p := range l.params {
		decl, err := makeParamVar(env, p.identifier, p.typeHint)
		if err != nil {
			return err
		}
		paramVars = append(paramVars, decl)
	}

	var err error
	l.fnEnv, err = env.Extend(cel.VariableDecls(paramVars...))
	if err != nil {
		return err
	}

	ast, iss := l.fnEnv.Compile(l.src)

	if iss != nil {
		return iss.Err()
	}

	if !typeAssignable(env, ast.OutputType(), l.resultType) {
		return fmt.Errorf("got result type %s for function %s", UnparseType(ast.OutputType()), l)
	}

	l.prog, err = l.fnEnv.Program(ast)

	if err != nil {
		return err
	}

	l.impl = func(args ...ref.Val) ref.Val {
		var err error
		var instance ref.Val
		if l.receiver != nil {
			instance = args[0]
			args = args[1:]
		}
		err = checkArgsMatch(env, l.params, args)
		if err != nil {
			return types.NewErr("error evaluating %s: %v", l, err)
		}

		activation := make(map[string]any)
		for i, param := range l.params {
			activation[param.identifier] = args[i]
		}

		if instance != nil {
			if !runtimeTypeAgrees(env, instance, l.receiver) {
				return types.NewErr("error evaluating %s: got receiver type: %s wanted %s", l, instance.Type().TypeName(), l.receiver.SpecifierFormat())
			}
			activation["this"] = instance
		}

		val, _, err := l.prog.Eval(activation)

		if err != nil {
			return types.NewErr("error evaluating %s: %v", l, err)
		}

		return val
	}
	return nil
}

func (l *letFunction) update(env *cel.Env) error {
	var err error

	err = l.updateImpl(env)
	if err != nil {
		return err
	}

	paramTypes := make([]*types.Type, len(l.params))
	for i, p := range l.params {
		ty, err := p.typeHint.AsCELType(env.CELTypeProvider())
		if err != nil {
			return err
		}
		paramTypes[i] = ty
	}

	resultTy, err := l.resultType.AsCELType(env.CELTypeProvider())
	if err != nil {
		return err
	}

	var opt cel.EnvOption
	if l.receiver != nil {
		ty, err := l.receiver.AsCELType(env.CELTypeProvider())
		if err != nil {
			return err
		}
		paramTypes = append([]*types.Type{ty}, paramTypes...)
		opt = cel.Function(
			l.identifier,
			cel.MemberOverload(
				l.identifier,
				paramTypes,
				resultTy,
				l.generateFunctionBinding()...,
			))
	} else {
		opt = cel.Function(
			l.identifier,
			cel.Overload(
				l.identifier,
				paramTypes,
				resultTy,
				l.generateFunctionBinding()...,
			))
	}

	l.env, err = env.Extend(opt)
	if err != nil {
		return err
	}

	return nil
}

func (l letVariable) String() string {
	return fmt.Sprintf("%s = %s", l.identifier, l.src)
}

func formatParams(params []letFunctionParam) string {
	fmtParams := make([]string, len(params))

	for i, p := range params {
		fmtParams[i] = fmt.Sprintf("%s: %s", p.identifier, p.typeHint.SpecifierFormat())
	}

	return fmt.Sprintf("(%s)", strings.Join(fmtParams, ", "))
}

func (l letFunction) String() string {
	receiverFmt := ""
	if l.receiver != nil {
		receiverFmt = fmt.Sprintf("%s.", l.receiver.SpecifierFormat())
	}

	return fmt.Sprintf("%s%s%s : %s -> %s", receiverFmt, l.identifier, formatParams(l.params), l.resultType.SpecifierFormat(), l.src)
}

func (l *letFunction) generateFunctionBinding() []cel.OverloadOpt {
	if l.impl == nil {
		return nil
	}
	argLen := len(l.params)
	if l.receiver != nil {
		argLen++
	}
	switch argLen {
	case 1:
		return []cel.OverloadOpt{cel.UnaryBinding(func(v ref.Val) ref.Val { return l.impl(v) })}
	case 2:
		return []cel.OverloadOpt{cel.BinaryBinding(func(lhs ref.Val, rhs ref.Val) ref.Val { return l.impl(lhs, rhs) })}
	default:
		return []cel.OverloadOpt{cel.FunctionBinding(l.impl)}
	}
}

// Reset plan if we need to recompile based on a dependency change.
func (l *letVariable) clearPlan() {
	l.resultType = nil
	l.env = nil
	l.ast = nil
	l.prog = nil
}

// Optioner interface represents an option set on the base CEL environment used by
// the evaluator.
type Optioner interface {
	// Option returns the cel.EnvOption that should be applied to the
	// environment.
	Option() cel.EnvOption
}

type contextOption struct {
	opt Optioner
	// True if the option is captured by the environment yaml format.
	yaml bool
}

type genericOption struct {
	envOpt cel.EnvOption
}

func (o *genericOption) String() string {
	return "// unsupported %configure option. see:\n" +
		"// %status --yaml"
}

func (o *genericOption) Option() cel.EnvOption {
	return o.envOpt
}

// EvaluationContext context for the repl.
// Handles maintaining state for multiple let expressions.
type EvaluationContext struct {
	letVars           []letVariable
	letFns            []letFunction
	options           []contextOption
	enablePartialEval bool
}

func (ctx *EvaluationContext) indexLetVar(name string) int {
	for idx, el := range ctx.letVars {
		if el.identifier == name {
			return idx
		}
	}
	return -1
}

func (ctx *EvaluationContext) applyContextOptions(env *cel.Env) *cel.Env {
	if len(ctx.options) == 0 {
		return env
	}
	opts := make([]cel.EnvOption, 0, len(ctx.options))
	for _, o := range ctx.options {
		opts = append(opts, o.opt.Option())
	}
	env, _ = env.Extend(opts...)
	return env
}

func (ctx *EvaluationContext) getEffectiveEnv(env *cel.Env) *cel.Env {
	if len(ctx.letVars) > 0 {
		env = ctx.letVars[len(ctx.letVars)-1].env
	} else if len(ctx.letFns) > 0 {
		env = ctx.letFns[len(ctx.letFns)-1].env
	}

	return ctx.applyContextOptions(env)
}

func (ctx *EvaluationContext) indexLetFn(name string) int {
	for idx, el := range ctx.letFns {
		if el.identifier == name {
			return idx
		}
	}
	return -1
}

func (ctx *EvaluationContext) copy() *EvaluationContext {
	var cpy EvaluationContext
	cpy.options = make([]contextOption, len(ctx.options))
	copy(cpy.options, ctx.options)
	cpy.letVars = make([]letVariable, len(ctx.letVars))
	copy(cpy.letVars, ctx.letVars)
	cpy.letFns = make([]letFunction, len(ctx.letFns))
	copy(cpy.letFns, ctx.letFns)
	cpy.enablePartialEval = ctx.enablePartialEval
	return &cpy
}

func (ctx *EvaluationContext) delLetVar(name string) {
	idx := ctx.indexLetVar(name)
	if idx < 0 {
		// no-op if deleting something that's not defined
		return
	}

	ctx.letVars = append(ctx.letVars[:idx], ctx.letVars[idx+1:]...)

	for i := idx; i < len(ctx.letVars); i++ {
		ctx.letVars[i].clearPlan()
	}
}

func (ctx *EvaluationContext) delLetFn(name string) {
	idx := ctx.indexLetFn(name)
	if idx < 0 {
		// no-op if deleting something that's not defined
		return
	}

	ctx.letFns = append(ctx.letFns[:idx], ctx.letFns[idx+1:]...)

	for i := range ctx.letVars {
		ctx.letVars[i].clearPlan()
	}
}

// Add or update an existing let then invalidate any computed plans.
func (ctx *EvaluationContext) addLetVar(name string, expr string, typeHint *envlib.TypeDesc) {
	idx := ctx.indexLetVar(name)
	newVar := letVariable{identifier: name, src: expr, typeHint: typeHint}
	if idx < 0 {
		ctx.letVars = append(ctx.letVars, newVar)
	} else {
		ctx.letVars[idx] = newVar
		for i := idx + 1; i < len(ctx.letVars); i++ {
			// invalidate dependant let exprs
			ctx.letVars[i].clearPlan()
		}
	}
}

// Try to normalize a defined function name as either a namespaced function or a receiver call.
func (ctx *EvaluationContext) resolveFn(env *cel.Env, name string) (string, *envlib.TypeDesc) {
	leadingDot := ""
	id := name
	if strings.HasPrefix(name, ".") {
		id = strings.TrimLeft(name, ".")
		leadingDot = "."
	}
	qualifiers := strings.Split(id, ".")
	if len(qualifiers) == 1 {
		return qualifiers[0], nil
	}

	namespace := strings.Join(qualifiers[:len(qualifiers)-1], ".")
	id = qualifiers[len(qualifiers)-1]

	maybeType, err := ParseType(leadingDot + namespace)
	if err != nil {
		return name, nil
	}
	env = ctx.applyContextOptions(env)
	t, err := maybeType.AsCELType(env.CELTypeProvider())
	if err != nil {
		return name, nil
	}

	switch t.Kind() {
	// unsupported type assume it's just namespaced
	case types.OpaqueKind:
	case types.ErrorKind:
	default:
		return id, maybeType
	}

	return name, nil
}

// Add or update an existing let then invalidate any computed plans.
func (ctx *EvaluationContext) addLetFn(env *cel.Env, name string, params []letFunctionParam, resultType *envlib.TypeDesc, expr string) {
	name, receiver := ctx.resolveFn(env, name)
	idx := ctx.indexLetFn(name)
	newFn := letFunction{identifier: name, params: params, receiver: receiver, resultType: resultType, src: expr}
	if idx < 0 {
		ctx.letFns = append(ctx.letFns, newFn)
	} else {
		ctx.letFns[idx] = newFn
	}

	for i := 0; i < len(ctx.letVars); i++ {
		// invalidate dependant let exprs
		ctx.letVars[i].clearPlan()
	}
}

func (ctx *EvaluationContext) addOption(opt Optioner, isYAML bool) {
	ctx.options = append(ctx.options,
		contextOption{opt: opt, yaml: isYAML})

	for i := 0; i < len(ctx.letVars); i++ {
		// invalidate dependant let exprs
		ctx.letVars[i].clearPlan()
	}
}

// programOptions generates the program options for planning.
// Assumes context has been planned.
func (ctx *EvaluationContext) programOptions() []cel.ProgramOption {
	result := []cel.ProgramOption{}
	if ctx.enablePartialEval {
		result = append(result, cel.EvalOptions(cel.OptPartialEval))
	}
	return result
}

// Evaluator provides basic environment for evaluating an expression with
// applied context.
type Evaluator struct {
	env *cel.Env
	ctx EvaluationContext
}

// NewEvaluator returns an inialized evaluator
func NewEvaluator() (*Evaluator, error) {
	env, err := cel.NewEnv(cel.EnableMacroCallTracking())
	if err != nil {
		return nil, err
	}

	return &Evaluator{env: env}, nil
}

// Attempt to update context in place after an update.
// This is done eagerly to help avoid introducing an invalid 'let' expression.
// The planned expressions are evaluated as needed when evaluating a (non-let) CEL expression.
// Return an error if any of the updates fail.
func updateContextPlans(ctx *EvaluationContext, env *cel.Env) error {
	for _, opt := range ctx.options {
		var err error
		env, err = env.Extend(opt.opt.Option())
		if err != nil {
			return err
		}
	}
	for i := range ctx.letFns {
		letFn := &ctx.letFns[i]
		err := letFn.update(env)
		if err != nil {
			return fmt.Errorf("error updating %s: %w", letFn, err)
		}
		env = letFn.env
	}
	for i := range ctx.letVars {
		el := &ctx.letVars[i]
		// Check if the let variable has a definition and needs to be re-planned
		if el.prog == nil && el.src != "" {
			ast, iss := env.Compile(el.src)
			if iss != nil {
				return fmt.Errorf("error updating %v\n%w", el, iss.Err())
			}

			if el.typeHint != nil && !typeAssignable(env, ast.OutputType(), el.typeHint) {
				return fmt.Errorf("error updating %v\ntype mismatch got %v expected %v",
					el,
					UnparseType(ast.OutputType()),
					el.typeHint.SpecifierFormat())
			}

			el.ast = ast
			el.resultType = envlib.SerializeTypeDesc(ast.OutputType())

			plan, err := env.Program(ast, ctx.programOptions()...)
			if err != nil {
				return err
			}
			el.prog = plan
		} else if el.src == "" {
			// Variable is declared but not defined, just update the type checking environment
			el.resultType = el.typeHint
		}
		if el.env == nil {
			t, err := el.resultType.AsCELType(env.CELTypeProvider())
			if err != nil {
				return err
			}
			env, err := env.Extend(cel.Variable(el.identifier, t))
			if err != nil {
				return err
			}
			el.env = env
		}
		env = el.env
	}
	return nil
}

// AddLetVar adds a let variable to the evaluation context.
// The expression is planned but evaluated lazily.
func (e *Evaluator) AddLetVar(name string, expr string, typeHint *envlib.TypeDesc) error {
	// copy the current context and attempt to update dependant expressions.
	// if successful, swap the current context with the updated copy.
	ctx := e.ctx.copy()
	ctx.addLetVar(name, expr, typeHint)
	err := updateContextPlans(ctx, e.env)
	if err != nil {
		return err
	}
	e.ctx = *ctx
	return nil
}

// AddLetFn adds a let function to the evaluation context.
func (e *Evaluator) AddLetFn(name string, params []letFunctionParam, resultType *envlib.TypeDesc, expr string) error {
	// copy the current context and attempt to update dependant expressions.
	// if successful, swap the current context with the updated copy.
	cpy := e.ctx.copy()
	cpy.addLetFn(e.env, name, params, resultType, expr)
	err := updateContextPlans(cpy, e.env)
	if err != nil {
		return err
	}
	e.ctx = *cpy
	return nil
}

// AddDeclVar declares a variable in the environment but doesn't register an expr with it.
// This allows planning to succeed, but with no value for the variable at runtime.
func (e *Evaluator) AddDeclVar(name string, typeHint *envlib.TypeDesc) error {
	ctx := e.ctx.copy()
	ctx.addLetVar(name, "", typeHint)
	err := updateContextPlans(ctx, e.env)
	if err != nil {
		return err
	}
	e.ctx = *ctx
	return nil
}

// AddDeclFn declares a function in the environment but doesn't register an expr with it.
// This allows planning to succeed, but with no value for the function at runtime.
func (e *Evaluator) AddDeclFn(name string, params []letFunctionParam, typeHint *envlib.TypeDesc) error {
	ctx := e.ctx.copy()
	ctx.addLetFn(e.env, name, params, typeHint, "")
	err := updateContextPlans(ctx, e.env)
	if err != nil {
		return err
	}
	e.ctx = *ctx
	return nil
}

// AddOption adds an option to the basic environment.
// Options are applied before evaluating any of the let statements.
// Returns an error if setting the option prevents planning any of the defined let expressions.
func (e *Evaluator) AddOption(opt Optioner) error {
	cpy := e.ctx.copy()
	cpy.addOption(opt, false)
	err := updateContextPlans(cpy, e.env)
	if err != nil {
		return err
	}
	e.ctx = *cpy
	return nil
}

// AddSerializableOption adds an option to the basic environment that is flagged as YAML compatible.
func (e *Evaluator) AddSerializableOption(opt Optioner) error {
	cpy := e.ctx.copy()
	cpy.addOption(opt, true)
	err := updateContextPlans(cpy, e.env)
	if err != nil {
		return err
	}
	e.ctx = *cpy
	return nil
}

// EnablePartialEval enables the option to allow partial evaluations.
func (e *Evaluator) EnablePartialEval() error {
	cpy := e.ctx.copy()
	cpy.enablePartialEval = true
	err := updateContextPlans(cpy, e.env)
	if err != nil {
		return err
	}
	e.ctx = *cpy
	return nil
}

// DelLetVar removes a variable from the evaluation context.
// If deleting the variable breaks a later expression, this function will return an error without modifying the context.
func (e *Evaluator) DelLetVar(name string) error {
	ctx := e.ctx.copy()
	ctx.delLetVar(name)
	err := updateContextPlans(ctx, e.env)
	if err != nil {
		return err
	}
	e.ctx = *ctx
	return nil
}

// DelLetFn removes a function from the evaluation context.
// If deleting the function breaks a later expression, this function will return an error without modifying the context.
func (e *Evaluator) DelLetFn(name string) error {
	ctx := e.ctx.copy()
	ctx.delLetFn(name)
	err := updateContextPlans(ctx, e.env)
	if err != nil {
		return err
	}
	e.ctx = *ctx
	return nil
}

type statusFormat int

const (
	statusFormatREPL statusFormat = iota
	statusFormatYAML
	statusFomatUnspecfied
)

func (e *Evaluator) handleStatus(args []string) (string, error) {
	format := statusFormatREPL
	for _, a := range args {
		switch a {
		case "--yaml":
			format = statusFormatYAML
		case "--repl":
			format = statusFormatREPL
		default:
			return "", fmt.Errorf("status: unsupported option: '%s'", a)
		}
	}

	switch format {
	case statusFormatREPL:
		return e.REPLConfig(), nil
	case statusFormatYAML:
		return e.YAMLConfig()
	}

	return "", errors.New("status: unsupported status format requested")
}

func (e *Evaluator) handleConfig(args []string) (string, error) {
	format := statusFormatREPL
	var fromFile bool
	var configRef string
	for _, a := range args {
		switch a {
		case "--yaml":
			format = statusFormatYAML
		case "--repl":
			format = statusFormatREPL
		case "--file":
			fromFile = true
		default:
			configRef = a
		}
	}

	if configRef == "" {
		return "", errors.New("config: no config provided")
	}
	src := []byte(configRef)
	if fromFile {
		tmp, err := os.ReadFile(configRef)
		if err != nil {
			return "", fmt.Errorf("configure: %w", err)
		}
		src = tmp
	}

	switch format {
	case statusFormatYAML:
		return e.parseYAMLConfig(src)
	}

	return "", fmt.Errorf("configure: not yet implemented %v %v %v ", format, fromFile, configRef)
}

// REPLConfig returns a stringified view of the current evaluator state in terms of REPL Commands.
func (e *Evaluator) REPLConfig() string {
	var status strings.Builder

	if len(e.ctx.options) > 0 {
		status.WriteString("// Options\n")
		for _, opt := range e.ctx.options {
			status.WriteString(fmt.Sprintf("%s\n", opt.opt))
		}
		status.WriteString("\n")
	}

	if len(e.ctx.letFns) > 0 {
		status.WriteString("// Functions\n")
		for _, fn := range e.ctx.letFns {
			cmd := "let"
			if fn.src == "" {
				cmd = "declare"
			}
			status.WriteString(fmt.Sprintf("%%%s %s\n", cmd, fn))
		}
		status.WriteString("\n")
	}

	if len(e.ctx.letVars) > 0 {
		status.WriteString("// Variables\n")
		for _, lVar := range e.ctx.letVars {
			cmd := "let"
			if lVar.src == "" {
				cmd = "declare"
			}
			status.WriteString(fmt.Sprintf("%%%s %s\n", cmd, lVar))
		}
		status.WriteString("\n")
	}
	return status.String()
}

func pruneDuplicateDecls(conf *env.Config) {
	// Duplicate variables are allowed when they have an equivalent type. The way the REPL handles
	// bindings ends up with a declaration in the config and a repeat when the "%let" operation is
	// applied. Filter here to keep the config clear.
	filtered := make([]*env.Variable, 0, len(conf.Variables))
	seen := make(map[string]bool)
	for _, v := range conf.Variables {
		if seen[v.Name] {
			continue
		}
		seen[v.Name] = true
		filtered = append(filtered, v)
	}
	conf.Variables = filtered
}

// YAMLConfig returns a yaml-based CEL environment representing the current evaluator state.
func (e *Evaluator) YAMLConfig() (string, error) {
	var status strings.Builder
	status.WriteString("# CEL environment YAML\n")
	conf, err := e.getEffectiveEnv().ToConfig("repl-session")
	if err != nil {
		return "", fmt.Errorf("status: REPL env broken: %w", err)
	}
	pruneDuplicateDecls(conf)
	data, err := yaml.Marshal(conf)
	if err != nil {
		return "", fmt.Errorf("status: unserializable YAML environment: %w", err)
	}
	status.Write(data)
	status.WriteString("\n\n# REPL Bindings: \n")

	for _, opt := range e.ctx.options {
		if !opt.yaml {
			status.WriteString(fmt.Sprintf("# %s\n", opt.opt))
		}
	}

	for _, lFn := range e.ctx.letFns {
		if lFn.src == "" {
			// Declaration handled in the standard YAML
			continue
		}
		lines := strings.Split(lFn.String(), "\n")
		for i, l := range lines {
			if i == 0 {
				status.WriteString("# %let ")
			} else {
				status.WriteString("# ")
			}
			status.WriteString(l)
			if i < len(lines)-1 {
				status.WriteString("\\\n")
			}
		}
		status.WriteString("\n#\n")
	}

	for _, lVar := range e.ctx.letVars {
		if lVar.src == "" {
			continue
		}
		lines := strings.Split(lVar.String(), "\n")
		for i, l := range lines {
			if i == 0 {
				status.WriteString("# %let ")
			} else {
				status.WriteString("# ")
			}
			status.WriteString(l)
			if i < len(lines)-1 {
				status.WriteString("\\\n")
			}
		}
		status.WriteString("\n#\n")
	}

	return status.String(), nil
}

var trailerRegexp = regexp.MustCompile("# REPL Bindings:")

func (e *Evaluator) parseYAMLConfig(conf []byte) (string, error) {
	yamlSrc := conf
	var replSrc []byte

	pos := trailerRegexp.FindIndex(conf)
	if len(pos) == 2 {
		yamlSrc = conf[0:pos[0]]
		replSrc = conf[pos[1]:]
	}

	var c env.Config
	if err := yaml.Unmarshal(yamlSrc, &c); err != nil {
		return "", fmt.Errorf("configure (yaml): %w", err)
	}

	s := string(replSrc)

	lines := strings.Split(s, "\n")

	// A little unfortunate, but we need to apply option commands manually
	// before the yaml environment. This is mainly because the yaml doesn't
	// describe how to lookup extension types (e.g. load_descriptors).
	updated, _ := NewEvaluator()

	var cmds []Cmder
	var sb strings.Builder
	for _, line := range lines {
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		norm := strings.TrimPrefix(line, "# ")
		if strings.HasSuffix(norm, "\\") {
			norm = strings.TrimSuffix(norm, "\\")
			sb.WriteString(norm)
			sb.WriteString("\n")
			continue
		}
		sb.WriteString(norm)
		cmd := sb.String()
		sb.Reset()
		pCmd, err := Parse(cmd)
		if err != nil {
			return "", fmt.Errorf("configure: invalid REPL command: %w", err)
		}
		switch pCmd.(type) {
		case *letFnCmd, *letVarCmd:
			cmds = append(cmds, pCmd)
			continue
		}
		_, exit, err := updated.Process(pCmd)

		if exit {
			return "", errors.New("configure: config triggered an exit")
		}
		if err != nil {
			return "", fmt.Errorf("configure: failed to process option %w", err)
		}
	}

	if len(updated.ctx.letFns) > 0 || len(updated.ctx.letVars) > 0 {
		return "", errors.New("configure: unsupported REPL config. Option defines variables")
	}

	err := updated.AddSerializableOption(&genericOption{envOpt: cel.FromConfig(&c, ext.ExtensionOptionFactory)})

	if err != nil {
		return "", fmt.Errorf("configure: failed to apply yaml: %w", err)
	}

	// Apply bindings now.
	for _, cmd := range cmds {
		_, exit, err := updated.Process(cmd)

		if exit {
			return "", errors.New("configure: config triggered an exit")
		}
		if err != nil {
			return "", fmt.Errorf("configure: failed to apply let: %w", err)
		}
	}

	*e, *updated = *updated, *e
	return "<loaded config>", nil
}

func (e *Evaluator) getEffectiveEnv() *cel.Env {
	env := e.ctx.getEffectiveEnv(e.env)
	return env
}

// applyContext evaluates the let expressions in the context to build an activation for the given expression.
// returns the environment for compiling and planning the top level CEL expression and an activation with the
// values of the let expressions.
func (e *Evaluator) applyContext() (*cel.Env, cel.Activation, error) {
	var vars = make(map[string]any)

	for _, el := range e.ctx.letVars {
		if el.prog == nil {
			// Declared but not defined variable so nothing to evaluate
			continue
		}

		val, _, err := el.prog.Eval(vars)
		if val != nil {
			vars[el.identifier] = val
		} else if err != nil {
			return nil, nil, err
		}
	}

	act, err := cel.NewActivation(vars)
	if err != nil {
		return nil, nil, err
	}

	return e.getEffectiveEnv(), act, nil
}

// typeOption implements optioner for loading a set of types defined by a protobuf file descriptor set.
type typeOption struct {
	path  string
	fds   *descpb.FileDescriptorSet
	isPkg bool
}

func (o *typeOption) String() string {
	flags := ""
	if o.isPkg {
		flags = "--pkg"
	}
	return fmt.Sprintf("%%load_descriptors %s '%s'", flags, o.path)
}

func (o *typeOption) Option() cel.EnvOption {
	return cel.TypeDescs(o.fds)
}

type backtickOpt struct {
	enabled bool
}

func (o *backtickOpt) Option() cel.EnvOption {
	if o.enabled {
		return cel.EnableIdentifierEscapeSyntax()
	}
	return func(e *cel.Env) (*cel.Env, error) { return e, nil }
}

func (o *backtickOpt) String() string {
	return "%option --enable_escaped_fields"
}

type jsonOpt struct {
	enabled bool
}

func (o *jsonOpt) Option() cel.EnvOption {

	return cel.JSONFieldNames(o.enabled)
}

func (o *jsonOpt) String() string {
	return "%option --enable_json_field_names"
}

type containerOption struct {
	container string
}

func (o *containerOption) String() string {
	return fmt.Sprintf("%%option --container '%s'", o.container)
}

func (o *containerOption) Option() cel.EnvOption {
	return cel.Container(o.container)
}

// extensionOption implements optional for loading a specific extension into the environment (String, Math, Proto, Encoder)
type extensionOption struct {
	extensionType string
	option        cel.EnvOption
}

func (o *extensionOption) String() string {
	return fmt.Sprintf("%%option --extension '%s'", o.extensionType)
}

func (o extensionOption) Option() cel.EnvOption {
	return o.option
}

func newExtensionOption(extType string) (*extensionOption, error) {
	extType = strings.ToLower(extType)
	if extOption, found := extensionMap[extType]; found {
		return &extensionOption{extensionType: extType, option: extOption}, nil
	} else {
		keys := make([]string, 0)
		for k := range extensionMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		extKeyName := make([]string, 0, len(keys))
		for _, k := range keys {
			extKeyName = append(extKeyName, "'"+k+"'")
		}
		joinedOptions := "['all', " + strings.Join(extKeyName, ", ") + "]"
		return nil, fmt.Errorf("unknown option: %s. Available options are: %s", extType, joinedOptions)
	}
}

// setOption sets a number of options on the environment. returns an error if
// any of them fail.
func (e *Evaluator) setOption(args []string) error {
	var issues []string
	for idx := 0; idx < len(args); {
		arg := args[idx]
		idx++
		switch arg {
		case "--container":
			err := e.loadContainerOption(idx, args)
			idx++
			if err != nil {
				issues = append(issues, fmt.Sprintf("container: %v", err))
			}
		case "--extension":
			err := e.loadExtensionOption(idx, args)
			idx++
			if err != nil {
				issues = append(issues, fmt.Sprintf("extension: %v", err))
			}
		case "--enable_escaped_fields":
			err := e.AddSerializableOption(&backtickOpt{enabled: true})
			if err != nil {
				issues = append(issues, fmt.Sprintf("enable_escaped_fields: %v", err))
			}
		case "--enable_json_field_names":
			err := e.AddSerializableOption(&jsonOpt{enabled: true})
			if err != nil {
				issues = append(issues, fmt.Sprintf("enable_json_field_names: %v", err))
			}
		case "--enable_partial_eval":
			err := e.EnablePartialEval()
			if err != nil {
				issues = append(issues, fmt.Sprintf("enable_partial_eval: %v", err))
			}
		default:
			issues = append(issues, fmt.Sprintf("unsupported option '%s'", arg))
		}
	}
	if len(issues) > 0 {
		return errors.New(strings.Join(issues, "\n"))
	}
	return nil
}

func checkOptionArgs(idx int, args []string) error {
	if idx >= len(args) {
		return fmt.Errorf("not enough arguments")
	}
	return nil
}

func (e *Evaluator) loadContainerOption(idx int, args []string) error {
	err := checkOptionArgs(idx, args)
	if err != nil {
		return err
	}

	container := args[idx]
	idx++
	err = e.AddSerializableOption(&containerOption{container: container})
	if err != nil {
		return err
	}
	return nil
}

func (e *Evaluator) loadExtensionOption(idx int, args []string) error {
	err := checkOptionArgs(idx, args)
	if err != nil {
		return err
	}

	argExtType := args[idx]
	if argExtType == "all" {
		// Load all extension types as a convenience
		for val := range extensionMap {
			err := e.loadExtensionOptionType(val)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return e.loadExtensionOptionType(argExtType)
}

func (e *Evaluator) loadExtensionOptionType(extType string) error {
	extensionOption, err := newExtensionOption(extType)
	if err != nil {
		return err
	}

	err = e.AddSerializableOption(extensionOption)
	if err != nil {
		return err
	}

	return nil

}

func loadFileDescriptorSet(path string, textfmt bool) (*descpb.FileDescriptorSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fds descpb.FileDescriptorSet

	if textfmt {
		err = prototext.Unmarshal(data, &fds)
	} else {
		// binary pb
		err = proto.Unmarshal(data, &fds)
	}
	if err != nil {
		return nil, err
	}

	return &fds, nil
}

func deps(d protoreflect.FileDescriptor) []*descpb.FileDescriptorProto {
	var descriptorProtos []*descpb.FileDescriptorProto

	for i := 0; i < d.Imports().Len(); i++ {
		descriptorProtos = append(descriptorProtos,
			protodesc.ToFileDescriptorProto(d.Imports().Get(i)))
	}

	return descriptorProtos
}

func (e *Evaluator) loadDescriptorFromPackage(pkg string) error {
	switch pkg {
	case "cel-spec-test-types":
		fdp2 := (&test2pb.TestAllTypes{}).ProtoReflect().Type().Descriptor().ParentFile()
		fdp2ext := (&test2pb.Proto2ExtensionScopedMessage{}).ProtoReflect().Type().Descriptor().ParentFile()
		fdp3 := (&test3pb.TestAllTypes{}).ProtoReflect().Type().Descriptor().ParentFile()

		// We only depend on WKTs.
		descriptorProtos := deps(fdp2)

		descriptorProtos = append(descriptorProtos,
			protodesc.ToFileDescriptorProto(fdp2),
			protodesc.ToFileDescriptorProto(fdp2ext),
			protodesc.ToFileDescriptorProto(fdp3))

		fds := descpb.FileDescriptorSet{
			File: descriptorProtos,
		}

		return e.AddOption(&typeOption{pkg, &fds, true})
	case "google-rpc":
		fdp := (&attrpb.AttributeContext{}).ProtoReflect().Type().Descriptor().ParentFile()

		descriptorProtos := append(deps(fdp),
			protodesc.ToFileDescriptorProto(fdp))

		fds := descpb.FileDescriptorSet{
			File: descriptorProtos,
		}

		return e.AddOption(&typeOption{pkg, &fds, true})
	}

	return fmt.Errorf("unknown type package: '%s'", pkg)
}

func (e *Evaluator) loadDescriptorFromFile(p string, textfmt bool) error {
	fds, err := loadFileDescriptorSet(p, textfmt)
	if err != nil {
		return fmt.Errorf("error loading file: %v", err)
	}

	return e.AddOption(&typeOption{path: p, fds: fds})
}

func (e *Evaluator) loadDescriptors(args []string) error {
	if len(args) < 1 {
		return errors.New("expected args for load descriptors")
	}

	textfmt := true

	var paths []string
	var pkgs []string
	nextIsPkg := false
	for _, flag := range args {
		switch flag {
		case "--binarypb":
			{
				textfmt = false
			}
		case "--pkg":
			{
				nextIsPkg = true
			}
		default:
			{
				if nextIsPkg {
					pkgs = append(pkgs, flag)
					nextIsPkg = false
				} else {
					paths = append(paths, flag)
				}
			}
		}
	}

	for _, p := range paths {
		err := e.loadDescriptorFromFile(p, textfmt)
		if err != nil {
			return err
		}
	}

	for _, p := range pkgs {
		err := e.loadDescriptorFromPackage(p)
		if err != nil {
			return err
		}
	}

	return nil
}

// Process processes the command provided.
func (e *Evaluator) Process(cmd Cmder) (string, bool, error) {
	switch cmd := cmd.(type) {
	case *compileCmd:
		ast, err := e.Compile(cmd.expr)
		if err != nil {
			return "", false, fmt.Errorf("compile failed:\n%v", err)
		}
		cAST, err := cel.AstToCheckedExpr(ast)
		if err != nil {
			return "", false, fmt.Errorf("compile failed:\n%v", err)
		}
		return prototext.Format(cAST), false, nil
	case *parseCmd:
		ast, err := e.Parse(cmd.expr)
		if err != nil {
			return "", false, fmt.Errorf("parse failed:\n%v", err)
		}
		pAST, err := cel.AstToParsedExpr(ast)
		if err != nil {
			return "", false, fmt.Errorf("parse failed:\n%v", err)
		}
		return prototext.Format(pAST), false, nil
	case *evalCmd:
		var (
			val     ref.Val
			resultT *types.Type
			err     error
		)
		if cmd.parseOnly {
			val, resultT, err = e.EvaluateParseOnly(cmd.expr)
		} else {
			val, resultT, err = e.Evaluate(cmd.expr)
		}
		if err != nil {
			return "", false, fmt.Errorf("expr failed:\n%v", err)
		}
		if val != nil {
			unknown, ok := val.Value().(*types.Unknown)
			if ok {
				return fmt.Sprintf("Unknown %v", unknown), false, nil
			}
			if cmd.parseOnly {
				return fmt.Sprintf("%s", types.Format(val)), false, nil
			}
			t := envlib.SerializeTypeDesc(resultT).SpecifierFormat()
			return fmt.Sprintf("%s : %s", types.Format(val), t), false, nil
		}
	case *letVarCmd:
		var err error
		if cmd.src != "" {
			err = e.AddLetVar(cmd.identifier, cmd.src, cmd.typeHint)
		} else {
			// declare only
			err = e.AddDeclVar(cmd.identifier, cmd.typeHint)
		}
		if err != nil {
			return "", false, fmt.Errorf("adding variable failed:\n%v", err)
		}
	case *letFnCmd:
		err := errors.New("declare not yet implemented")
		if cmd.src != "" {
			err = e.AddLetFn(cmd.identifier, cmd.params, cmd.resultType, cmd.src)
		}
		if err != nil {
			return "", false, fmt.Errorf("adding function failed:\n%v", err)
		}
	case *delCmd:
		err := e.DelLetVar(cmd.identifier)
		if err != nil {
			return "", false, fmt.Errorf("deleting declaration failed:\n%v", err)
		}
		err = e.DelLetFn(cmd.identifier)
		if err != nil {
			return "", false, fmt.Errorf("deleting declaration failed:\n%v", err)
		}

	case *simpleCmd:
		switch cmd.Cmd() {
		case "exit":
			return "", true, nil
		case "null":
			return "", false, nil
		case "configure":
			out, err := e.handleConfig(cmd.args)
			return out, false, err
		case "status":
			out, err := e.handleStatus(cmd.args)
			return out, false, err
		case "load_descriptors":
			return "", false, e.loadDescriptors(cmd.args)
		case "option":
			return "", false, e.setOption(cmd.args)
		case "reset":
			e.ctx = EvaluationContext{}
			return "", false, nil
		default:
			return "", false, fmt.Errorf("unsupported command: %v", cmd.Cmd())
		}
	default:
		return "", false, fmt.Errorf("unsupported command: %v", cmd.Cmd())
	}
	return "", false, nil
}

// Evaluate sets up a CEL evaluation using the current REPL context.
func (e *Evaluator) Evaluate(expr string) (ref.Val, *types.Type, error) {
	env, act, err := e.applyContext()
	if err != nil {
		return nil, nil, err
	}

	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		return nil, nil, iss.Err()
	}

	p, err := env.Program(ast, e.ctx.programOptions()...)
	if err != nil {
		return nil, nil, err
	}

	act, _ = env.PartialVars(act)
	val, _, err := p.Eval(act)
	// expression can be well-formed and result in an error
	return val, ast.OutputType(), err
}

// EvaluateParseOnly evalutes the CEL expression using the current REPL context without type checking.
func (e *Evaluator) EvaluateParseOnly(expr string) (ref.Val, *types.Type, error) {
	env, act, err := e.applyContext()
	if err != nil {
		return nil, nil, err
	}

	ast, iss := env.Parse(expr)
	if iss.Err() != nil {
		return nil, nil, iss.Err()
	}

	p, err := env.Program(ast, e.ctx.programOptions()...)
	if err != nil {
		return nil, nil, err
	}

	act, _ = env.PartialVars(act)
	val, _, err := p.Eval(act)
	// expression can be well-formed and result in an error
	return val, ast.OutputType(), err
}

// Compile compiles the input expression using the current REPL context.
func (e *Evaluator) Compile(expr string) (*cel.Ast, error) {
	env, _, err := e.applyContext()
	if err != nil {
		return nil, err
	}
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		return nil, iss.Err()
	}
	return ast, nil
}

// Parse parses the input expression using the current REPL context.
func (e *Evaluator) Parse(expr string) (*cel.Ast, error) {
	env, _, err := e.applyContext()
	if err != nil {
		return nil, err
	}
	ast, iss := env.Parse(expr)
	if iss.Err() != nil {
		return nil, iss.Err()
	}
	return ast, nil
}
