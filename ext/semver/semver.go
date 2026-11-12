// Copyright 2026 Google LLC
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

// Package semver implements CEL extension functions for Semantic Versioning (SemVer 2.0.0)
// parsing, validation, component extraction, and comparison.
package semver

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	blangsemver "github.com/blang/semver/v4"
)

// Library returns a cel.EnvOption to configure extended functions for
// semantic version (SemVer 2.0.0) parsing, validation, component extraction,
// and comparison.
//
// Note: This library defines global functions `semver` and `isSemver`. If you
// are currently using variables with those names, these functions will likely
// work as intended, however there is a chance for collision.
//
// The library closely mirrors the behavior of the Kubernetes CEL semver
// library, treating semantic versions as an opaque type (`semver_type`).
//
// This library includes a TypeAdapter that allows `semver.Version` Go values
// (from `github.com/blang/semver/v4`) to be passed directly into the CEL
// environment.
//
// # Semver Parsing and Validation
//
// The `semver` function converts a string to a semantic version (`Semver`) or
// results in an error if the string is not a valid semantic version. Refer to
// semver.org documentation for information on accepted patterns.
//
// An optional `normalize` boolean argument (introduced in version 1) can be
// passed to enable normalization. Normalization removes any leading `"v"`
// prefix, adds `0` minor and patch numbers to versions with only major or
// major.minor components specified, and removes any leading `0`s from numeric
// components.
//
// The `isSemver` function checks whether a string is a valid semantic version
// (optionally with normalization in version 1) without throwing an error.
//
//	semver(string) -> Semver
//	semver(string, bool) -> Semver // Introduced at version 1
//	isSemver(string) -> bool
//	isSemver(string, bool) -> bool // Introduced at version 1
//
// Examples:
//
//	semver('1.0.0')
//	semver('0.1.0-alpha.1')
//	semver('v1.0', true) == semver('1.0.0')
//	semver('01.01.01', true) == semver('1.1.1')
//	isSemver('1.2.3-beta.1+build.1') == true
//	isSemver('v1.0') == false
//	isSemver('v1.0', true) == true
//
// # Component Accessors
//
//	<Semver>.major() -> int
//	<Semver>.minor() -> int
//	<Semver>.patch() -> int
//
// Examples:
//
//	semver('1.2.3').major() == 1
//	semver('1.2.3').minor() == 2
//	semver('1.2.3').patch() == 3
//
// # Comparison and Conversion
//
//   - `isGreaterThan`: Returns true if and only if the receiver is greater than the operand.
//   - `isLessThan`: Returns true if and only if the receiver is less than the operand.
//   - `compareTo`: Compares receiver to operand and returns 0 if they are equal, 1 if the receiver is greater, or -1 if the receiver is less than the operand.
//   - `string`: Formats a `Semver` value as its canonical semantic version string.
//
// Signatures:
//
//	<Semver>.isGreaterThan(Semver) -> bool
//	<Semver>.isLessThan(Semver) -> bool
//	<Semver>.compareTo(Semver) -> int
//	string(Semver) -> string
//
// Examples:
//
//	semver('1.2.3').compareTo(semver('1.2.3')) == 0
//	semver('1.2.3').compareTo(semver('2.0.0')) == -1
//	semver('1.2.3').compareTo(semver('0.1.2')) == 1
//	semver('1.0.0').isLessThan(semver('1.2.3')) == true
//	semver('1.2.3').isGreaterThan(semver('1.0.0')) == true
//	string(semver('v1.2', true)) == '1.2.0'
func Library(options ...Option) cel.EnvOption {
	lib := &semverLib{version: math.MaxUint32}
	for _, o := range options {
		lib = o(lib)
	}
	return func(e *cel.Env) (*cel.Env, error) {
		e, err := cel.Lib(lib)(e)
		if err != nil {
			return nil, err
		}
		adapter := &semverAdapter{Adapter: e.CELTypeAdapter()}
		return cel.CustomTypeAdapter(adapter)(e)
	}
}

// SemverLib is an alias for Library.
func SemverLib(options ...Option) cel.EnvOption {
	return Library(options...)
}

// Option declares a functional operator for configuring the Semver library behavior.
type Option func(*semverLib) *semverLib

// Version configures the version of the Semver library.
//
// The version limits which functions and overloads are available. Only
// functions introduced below or equal to the given version are included in the
// library. If this option is not set, all functions are available.
func Version(version uint32) Option {
	return func(lib *semverLib) *semverLib {
		lib.version = version
		return lib
	}
}

// SemverVersion is an alias for Version.
func SemverVersion(version uint32) Option {
	return Version(version)
}

// SemverOption is an alias for Option.
type SemverOption = Option

const (
	compareToFunc     = "compareTo"
	isGreaterThanFunc = "isGreaterThan"
	isLessThanFunc    = "isLessThan"
	isSemverFunc      = "isSemver"
	majorFunc         = "major"
	minorFunc         = "minor"
	patchFunc         = "patch"
	semverFunc        = "semver"
	semverToString    = overloads.TypeConvertString
)

var (
	// SemverType represents a semantic version in CEL.
	SemverType = types.NewOpaqueType("semver_type")
)

type semverLib struct {
	version uint32
}

// LibraryName implements the SingletonLibrary interface method.
func (*semverLib) LibraryName() string {
	return "cel.lib.ext.semver"
}

// LibraryVersion implements the LibraryVersioner interface method.
func (s *semverLib) LibraryVersion() uint32 {
	return s.version
}

// CompileOptions implements the cel.Library interface method.
func (s *semverLib) CompileOptions() []cel.EnvOption {
	semverOverloads := []cel.FunctionOpt{
		cel.Overload("string_to_semver", []*cel.Type{cel.StringType}, SemverType,
			cel.UnaryBinding(stringToSemver)),
	}
	isSemverOverloads := []cel.FunctionOpt{
		cel.Overload("is_semver_string", []*cel.Type{cel.StringType}, cel.BoolType,
			cel.UnaryBinding(isSemver)),
	}
	if s.version >= 1 {
		semverOverloads = append(semverOverloads,
			cel.Overload("string_bool_to_semver", []*cel.Type{cel.StringType, cel.BoolType}, SemverType,
				cel.BinaryBinding(stringToSemverNormalize)),
		)
		isSemverOverloads = append(isSemverOverloads,
			cel.Overload("is_semver_string_bool", []*cel.Type{cel.StringType, cel.BoolType}, cel.BoolType,
				cel.BinaryBinding(isSemverNormalize)),
		)
	}

	return []cel.EnvOption{
		cel.Types(SemverType),
		cel.Function(compareToFunc,
			cel.MemberOverload("semver_compare_to", []*cel.Type{SemverType, SemverType}, cel.IntType,
				cel.BinaryBinding(semverCompareTo)),
		),
		cel.Function(isGreaterThanFunc,
			cel.MemberOverload("semver_is_greater_than", []*cel.Type{SemverType, SemverType}, cel.BoolType,
				cel.BinaryBinding(semverIsGreaterThan)),
		),
		cel.Function(isLessThanFunc,
			cel.MemberOverload("semver_is_less_than", []*cel.Type{SemverType, SemverType}, cel.BoolType,
				cel.BinaryBinding(semverIsLessThan)),
		),
		cel.Function(isSemverFunc, isSemverOverloads...),
		cel.Function(majorFunc,
			cel.MemberOverload("semver_major", []*cel.Type{SemverType}, cel.IntType,
				cel.UnaryBinding(semverMajor)),
		),
		cel.Function(minorFunc,
			cel.MemberOverload("semver_minor", []*cel.Type{SemverType}, cel.IntType,
				cel.UnaryBinding(semverMinor)),
		),
		cel.Function(patchFunc,
			cel.MemberOverload("semver_patch", []*cel.Type{SemverType}, cel.IntType,
				cel.UnaryBinding(semverPatch)),
		),
		cel.Function(semverFunc, semverOverloads...),
		cel.Function(semverToString,
			cel.Overload("semver_to_string", []*cel.Type{SemverType}, cel.StringType,
				cel.UnaryBinding(semverValToString)),
		),
		cel.ASTValidators(
			semverFormatValidator{},
		),
		cel.CostModel(
			semverParseCostModel("string_to_semver"),
			semverParseCostModel("string_bool_to_semver"),
			semverParseBoolCostModel("is_semver_string"),
			semverParseBoolCostModel("is_semver_string_bool"),
			semverToStringCostModel("semver_to_string"),
			semverNominalCostModel("semver_compare_to"),
			semverNominalCostModel("semver_is_greater_than"),
			semverNominalCostModel("semver_is_less_than"),
			semverNominalCostModel("semver_major"),
			semverNominalCostModel("semver_minor"),
			semverNominalCostModel("semver_patch"),
		),
	}
}

// ProgramOptions implements the cel.Library interface method.
func (*semverLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// semverAdapter adapts semver.Version values while preserving existing adapters.
type semverAdapter struct {
	types.Adapter
}

func (a *semverAdapter) NativeToValue(value any) ref.Val {
	switch v := value.(type) {
	case blangsemver.Version:
		return Semver{Version: v}
	case *blangsemver.Version:
		if v != nil {
			return Semver{Version: *v}
		}
	case []blangsemver.Version:
		list := make([]Semver, len(v))
		for i, ver := range v {
			list[i] = Semver{Version: ver}
		}
		return a.Adapter.NativeToValue(list)
	}
	return a.Adapter.NativeToValue(value)
}

// --- Implementation Logic ---

func isSemver(arg ref.Val) ref.Val {
	return isSemverNormalize(arg, types.False)
}

func isSemverNormalize(arg, normalizeArg ref.Val) ref.Val {
	str, ok := arg.Value().(string)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	normalize, ok := normalizeArg.Value().(bool)
	if !ok {
		return types.MaybeNoSuchOverloadErr(normalizeArg)
	}
	var err error
	if normalize {
		_, err = normalizeAndParse(str)
	} else {
		_, err = blangsemver.Parse(str)
	}
	return types.Bool(err == nil)
}

func stringToSemver(arg ref.Val) ref.Val {
	return stringToSemverNormalize(arg, types.False)
}

func stringToSemverNormalize(arg, normalizeArg ref.Val) ref.Val {
	str, ok := arg.Value().(string)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	normalize, ok := normalizeArg.Value().(bool)
	if !ok {
		return types.MaybeNoSuchOverloadErr(normalizeArg)
	}
	var (
		v   blangsemver.Version
		err error
	)
	if normalize {
		v, err = normalizeAndParse(str)
	} else {
		v, err = blangsemver.Parse(str)
	}
	if err != nil {
		return types.WrapErr(err)
	}
	return Semver{Version: v}
}

func semverValToString(arg ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.String())
}

func semverMajor(arg ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.Int(v.Major)
}

func semverMinor(arg ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.Int(v.Minor)
}

func semverPatch(arg ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.Int(v.Patch)
}

func semverIsGreaterThan(arg, other ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	v2, ok := other.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}
	return types.Bool(v.Compare(v2) == 1)
}

func semverIsLessThan(arg, other ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	v2, ok := other.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}
	return types.Bool(v.Compare(v2) == -1)
}

func semverCompareTo(arg, other ref.Val) ref.Val {
	v, ok := arg.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	v2, ok := other.Value().(blangsemver.Version)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}
	return types.Int(v.Compare(v2))
}

// normalizeAndParse removes any "v" prefix, adds a 0 minor and patch numbers to versions with
// only major or major.minor components specified, and removes any leading 0s.
// normalizeAndParse is based on semver.ParseTolerant but does not trim extra whitespace and is
// guaranteed to not change behavior in the future.
func normalizeAndParse(s string) (blangsemver.Version, error) {
	s = strings.TrimPrefix(s, "v")

	// Split into major.minor.(patch+pr+meta)
	parts := strings.SplitN(s, ".", 3)
	// Remove leading zeros.
	for i, p := range parts {
		if len(p) > 1 {
			p = strings.TrimLeft(p, "0")
			if len(p) == 0 || !strings.ContainsAny(p[0:1], "0123456789") {
				p = "0" + p
			}
			parts[i] = p
		}
	}

	// Fill up shortened versions.
	if len(parts) < 3 {
		if strings.ContainsAny(parts[len(parts)-1], "+-") {
			return blangsemver.Version{}, errors.New("short version cannot contain PreRelease/Build meta data")
		}
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
	}
	s = strings.Join(parts, ".")

	return blangsemver.Parse(s)
}

// --- Type Wrappers ---

// Semver provides a CEL representation of a [blangsemver.Version].
type Semver struct {
	blangsemver.Version
}

var _ ref.Val = Semver{}

// ConvertToNative implements the ref.Val interface method.
func (v Semver) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if reflect.TypeOf(v.Version).AssignableTo(typeDesc) {
		return v.Version, nil
	}
	if reflect.TypeOf("").AssignableTo(typeDesc) {
		return v.Version.String(), nil
	}
	if typeDesc == reflect.TypeFor[Semver]() {
		return v, nil
	}
	return nil, fmt.Errorf("type conversion error from 'Semver' to '%v'", typeDesc)
}

// ConvertToType implements the ref.Val interface method.
func (v Semver) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case SemverType:
		return v
	case types.TypeType:
		return SemverType
	case types.StringType:
		return types.String(v.Version.String())
	default:
		return types.NewErr("type conversion error from '%s' to '%s'", SemverType, typeVal)
	}
}

// Equal implements the ref.Val interface method.
func (v Semver) Equal(other ref.Val) ref.Val {
	otherSemver, ok := other.(Semver)
	if !ok {
		return types.False
	}
	return types.Bool(v.Version.EQ(otherSemver.Version))
}

// Type implements the ref.Val interface method.
func (v Semver) Type() ref.Type {
	return SemverType
}

// Value implements the ref.Val interface method.
func (v Semver) Value() any {
	return v.Version
}

// --- Static Validators ---

type semverFormatValidator struct{}

func (semverFormatValidator) Name() string {
	return "cel.validator.semver"
}

func (semverFormatValidator) Validate(_ *cel.Env, _ cel.ValidatorConfig, a *ast.AST, iss *cel.Issues) {
	root := ast.NavigateAST(a)
	funcCalls := ast.MatchDescendants(root, ast.FunctionMatcher(semverFunc))
	for _, call := range funcCalls {
		callArgs := call.AsCall().Args()
		if len(callArgs) == 0 || callArgs[0].Kind() != ast.LiteralKind {
			continue
		}
		str, ok := callArgs[0].AsLiteral().Value().(string)
		if !ok {
			continue
		}
		var err error
		switch len(callArgs) {
		case 1:
			_, err = blangsemver.Parse(str)
		case 2:
			if callArgs[1].Kind() != ast.LiteralKind {
				continue
			}
			norm, ok := callArgs[1].AsLiteral().Value().(bool)
			if !ok {
				continue
			}
			if norm {
				_, err = normalizeAndParse(str)
			} else {
				_, err = blangsemver.Parse(str)
			}
		}
		if err != nil {
			iss.ReportErrorAtID(callArgs[0].ID(), "invalid %s argument: %v", semverFunc, err)
		}
	}
}

// --- Cost Models ---

func semverParseCostModel(id string) cost.OverloadModel {
	return cost.Overload(id,
		cost.EvalCost(cost.StringScan(cost.Length(cost.Arg(0)))),
		cost.ResultSize(cost.Const(1)),
	)
}

func semverParseBoolCostModel(id string) cost.OverloadModel {
	return cost.Overload(id,
		cost.EvalCost(cost.StringScan(cost.Length(cost.Arg(0)))),
	)
}

func semverToStringCostModel(id string) cost.OverloadModel {
	return cost.Overload(id,
		cost.EvalCost(cost.Const(cost.CallCost)),
	)
}

func semverNominalCostModel(id string) cost.OverloadModel {
	return cost.MemberOverload(id,
		cost.EvalCost(cost.Const(cost.CallCost)),
	)
}
