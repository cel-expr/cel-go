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

package cel

import (
	"testing"

	celenv "github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	proto2pb "github.com/google/cel-go/test/proto2pb"
	proto3pb "github.com/google/cel-go/test/proto3pb"
)

const (
	enumP2Container  = "google.expr.proto2.test"
	enumP3Container  = "google.expr.proto3.test"
	enumP2Global = "google.expr.proto2.test.GlobalEnum"
	enumP3Global = "google.expr.proto3.test.GlobalEnum"
	enumP2Nested = "google.expr.proto2.test.TestAllTypes.NestedEnum"
	enumP3Nested = "google.expr.proto3.test.TestAllTypes.NestedEnum"
	strongEnumsFeature  = "cel.feature.strong_enums"
)

func strongEnumEnv(t *testing.T, container string, opts ...EnvOption) *Env {
	t.Helper()
	conf := celenv.NewConfig("strong enum test config").AddFeatures(
		&celenv.Feature{Name: strongEnumsFeature, Enabled: true},
	)
	baseOpts := []EnvOption{
		Container(container),
		Types(&proto2pb.TestAllTypes{}),
		Types(&proto3pb.TestAllTypes{}),
		FromConfig(conf),
	}
	env, err := NewEnv(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	return env
}

func legacyEnumEnv(t *testing.T, container string, enumFeature bool) *Env {
	t.Helper()
	opts := []EnvOption{
		Container(container),
		Types(&proto2pb.TestAllTypes{}),
		Types(&proto3pb.TestAllTypes{}),
	}
	if enumFeature {
		conf := celenv.NewConfig("legacy enum test config").AddFeatures(
			&celenv.Feature{Name: strongEnumsFeature, Enabled: false},
		)
		opts = append(opts, FromConfig(conf))
	}
	env, err := NewEnv(opts...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	return env
}

func compileEvalEnum(t *testing.T, env *Env, expr string, vars map[string]any) (ref.Val, error) {
	t.Helper()
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program(%q) failed: %v", expr, err)
	}
	if vars == nil {
		vars = map[string]any{}
	}
	out, _, err := prg.Eval(vars)
	return out, err
}

func parseEvalEnum(t *testing.T, env *Env, expr string, vars map[string]any) (ref.Val, error) {
	t.Helper()
	ast, iss := env.Parse(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Parse(%q) failed: %v", expr, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program(%q) failed: %v", expr, err)
	}
	if vars == nil {
		vars = map[string]any{}
	}
	out, _, err := prg.Eval(vars)
	return out, err
}

// wantEnumValue asserts that expr evaluates to a value of the given enum type
// carrying the given number. The type assertion guarantees the check cannot be
// satisfied by the legacy integer representation.
func wantEnumValue(t *testing.T, env *Env, expr, typeName string, number int64) {
	t.Helper()
	out, err := compileEvalEnum(t, env, expr, nil)
	if err != nil {
		t.Fatalf("eval(%q) errored: %v", expr, err)
	}
	if out.Type().TypeName() != typeName {
		t.Errorf("eval(%q) got type %q, want %q", expr, out.Type().TypeName(), typeName)
	}
	num, err := compileEvalEnum(t, env, "int("+expr+")", nil)
	if err != nil {
		t.Fatalf("eval(int(%q)) errored: %v", expr, err)
	}
	iv, ok := num.(types.Int)
	if !ok {
		t.Fatalf("int(%q) got %T, want types.Int", expr, num)
	}
	if int64(iv) != number {
		t.Errorf("int(%q) got %d, want %d", expr, int64(iv), number)
	}
}

func wantEnumEvalError(t *testing.T, env *Env, expr string) {
	t.Helper()
	out, err := compileEvalEnum(t, env, expr, nil)
	if err == nil {
		t.Errorf("eval(%q) got %v, want evaluation error", expr, out)
	}
}

// Enum constant literals produce enum-typed values.

func TestStrongEnumsLiteralGlobalProto2(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "GlobalEnum.GAZ", enumP2Global, 2)
}

func TestStrongEnumsLiteralGlobalProto3(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "GlobalEnum.GAZ", enumP3Global, 2)
}

func TestStrongEnumsLiteralNestedProto2(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "TestAllTypes.NestedEnum.BAR", enumP2Nested, 1)
}

func TestStrongEnumsLiteralNestedProto3(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "TestAllTypes.NestedEnum.BAR", enumP3Nested, 1)
}

func TestStrongEnumsLiteralZero(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "GlobalEnum.GOO", enumP2Global, 0)
}

// type() reports the enum's own type.

func TestStrongEnumsTypeOfGlobal(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	out, err := compileEvalEnum(t, env, "type(GlobalEnum.GOO)", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	tv, ok := out.(*types.Type)
	if !ok {
		t.Fatalf("type() got %T, want *types.Type", out)
	}
	if tv.TypeName() != enumP3Global {
		t.Errorf("type() got %q, want %q", tv.TypeName(), enumP3Global)
	}
}

func TestStrongEnumsTypeOfNested(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	out, err := compileEvalEnum(t, env, "type(TestAllTypes.NestedEnum.BAZ)", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	tv, ok := out.(*types.Type)
	if !ok {
		t.Fatalf("type() got %T, want *types.Type", out)
	}
	if tv.TypeName() != enumP2Nested {
		t.Errorf("type() got %q, want %q", tv.TypeName(), enumP2Nested)
	}
}

func TestStrongEnumsTypeOfFieldDefault(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	out, err := compileEvalEnum(t, env, "type(TestAllTypes{}.standalone_enum)", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	tv, ok := out.(*types.Type)
	if !ok {
		t.Fatalf("type() got %T, want *types.Type", out)
	}
	if tv.TypeName() != enumP3Nested {
		t.Errorf("type() got %q, want %q", tv.TypeName(), enumP3Nested)
	}
}

// Equality is by enum type and number. Each test also asserts the operands
// are enum-typed so the equality outcome is not trivially the legacy one.

func TestStrongEnumsEqualitySameValue(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "GlobalEnum.GAR", enumP2Global, 1)
	out, err := compileEvalEnum(t, env, "GlobalEnum.GAR == GlobalEnum.GAR", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if b, ok := out.(types.Bool); !ok || !bool(b) {
		t.Errorf("same-value equality got %v, want true", out)
	}
}

func TestStrongEnumsEqualityDifferentValue(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "GlobalEnum.GAZ", enumP3Global, 2)
	out, err := compileEvalEnum(t, env, "GlobalEnum.GAR == GlobalEnum.GAZ", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if b, ok := out.(types.Bool); !ok || bool(b) {
		t.Errorf("different-value equality got %v, want false", out)
	}
}

func TestStrongEnumsEnumNotEqualInt(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	out, err := parseEvalEnum(t, env, "GlobalEnum.GAR == 1", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if b, ok := out.(types.Bool); !ok || bool(b) {
		t.Errorf("enum == int got %v, want false", out)
	}
}

func TestStrongEnumsEnumNotEqualOtherEnumType(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	out, err := parseEvalEnum(t, env, "GlobalEnum.GOO == TestAllTypes.NestedEnum.FOO", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if b, ok := out.(types.Bool); !ok || bool(b) {
		t.Errorf("cross-enum equality got %v, want false", out)
	}
}

// Field selection yields enum-typed values, including defaults and
// numbers without a declared name.

func TestStrongEnumsSelectDefault(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "TestAllTypes{}.standalone_enum", enumP2Nested, 0)
}

func TestStrongEnumsSelectSetValue(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container, Variable("x", ObjectType("google.expr.proto3.test.TestAllTypes")))
	msg := &proto3pb.TestAllTypes{StandaloneEnum: proto3pb.TestAllTypes_BAZ}
	out, err := compileEvalEnum(t, env, "x.standalone_enum", map[string]any{"x": msg})
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if out.Type().TypeName() != enumP3Nested {
		t.Errorf("select got type %q, want %q", out.Type().TypeName(), enumP3Nested)
	}
}

func TestStrongEnumsSelectSetValueProto2(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container, Variable("x", ObjectType("google.expr.proto2.test.TestAllTypes")))
	enumVal := proto2pb.TestAllTypes_BAZ
	msg := &proto2pb.TestAllTypes{StandaloneEnum: &enumVal}
	out, err := compileEvalEnum(t, env, "x.standalone_enum", map[string]any{"x": msg})
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if out.Type().TypeName() != enumP2Nested {
		t.Errorf("select got type %q, want %q", out.Type().TypeName(), enumP2Nested)
	}
}

func TestStrongEnumsSelectUnnamedBig(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container, Variable("x", ObjectType("google.expr.proto3.test.TestAllTypes")))
	msg := &proto3pb.TestAllTypes{StandaloneEnum: proto3pb.TestAllTypes_NestedEnum(108)}
	out, err := compileEvalEnum(t, env, "x.standalone_enum", map[string]any{"x": msg})
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if out.Type().TypeName() != enumP3Nested {
		t.Errorf("select got type %q, want %q", out.Type().TypeName(), enumP3Nested)
	}
	num, err := compileEvalEnum(t, env, "int(x.standalone_enum)", map[string]any{"x": msg})
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if iv, ok := num.(types.Int); !ok || int64(iv) != 108 {
		t.Errorf("unnamed select got %v, want 108", num)
	}
}

func TestStrongEnumsSelectUnnamedNeg(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container, Variable("x", ObjectType("google.expr.proto3.test.TestAllTypes")))
	msg := &proto3pb.TestAllTypes{StandaloneEnum: proto3pb.TestAllTypes_NestedEnum(-3)}
	out, err := compileEvalEnum(t, env, "x.standalone_enum", map[string]any{"x": msg})
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if out.Type().TypeName() != enumP3Nested {
		t.Errorf("select got type %q, want %q", out.Type().TypeName(), enumP3Nested)
	}
	num, err := compileEvalEnum(t, env, "int(x.standalone_enum)", map[string]any{"x": msg})
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if iv, ok := num.(types.Int); !ok || int64(iv) != -3 {
		t.Errorf("unnamed select got %v, want -3", num)
	}
}

// Message construction accepts enum-typed values.

func TestStrongEnumsConstructWithName(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "TestAllTypes{standalone_enum: TestAllTypes.NestedEnum.BAZ}.standalone_enum", enumP2Nested, 2)
}

func TestStrongEnumsConstructWithIntConversion(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "TestAllTypes{standalone_enum: TestAllTypes.NestedEnum(1)}.standalone_enum", enumP3Nested, 1)
	out, err := compileEvalEnum(t, env, "TestAllTypes{standalone_enum: TestAllTypes.NestedEnum(1)}.standalone_enum == TestAllTypes.NestedEnum.BAR", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if b, ok := out.(types.Bool); !ok || !bool(b) {
		t.Errorf("constructed field equality got %v, want true", out)
	}
}

func TestStrongEnumsConstructWithUnnamedInt(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "TestAllTypes{standalone_enum: TestAllTypes.NestedEnum(99)}.standalone_enum", enumP3Nested, 99)
}

func TestStrongEnumsConstructWithNegativeInt(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "TestAllTypes{standalone_enum: TestAllTypes.NestedEnum(-1)}.standalone_enum", enumP3Nested, -1)
}

// Explicit conversions.

func TestStrongEnumsIntOfNamedConstant(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "GlobalEnum.GAZ", enumP2Global, 2)
}

func TestStrongEnumsEnumOfIntInRange(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "TestAllTypes.NestedEnum(2)", enumP2Nested, 2)
}

func TestStrongEnumsEnumOfIntUnnamedBig(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "TestAllTypes.NestedEnum(20000)", enumP2Nested, 20000)
}

func TestStrongEnumsEnumOfIntUnnamedNeg(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "GlobalEnum(-33)", enumP3Global, -33)
}

func TestStrongEnumsEnumOfIntTooBig(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumEvalError(t, env, "TestAllTypes.NestedEnum(5000000000)")
}

func TestStrongEnumsEnumOfIntTooNeg(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumEvalError(t, env, "TestAllTypes.NestedEnum(-7000000000)")
}

func TestStrongEnumsEnumOfIntBoundary(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "GlobalEnum(2147483647)", enumP3Global, 2147483647)
	wantEnumValue(t, env, "GlobalEnum(-2147483648)", enumP3Global, -2147483648)
	wantEnumEvalError(t, env, "GlobalEnum(2147483648)")
	wantEnumEvalError(t, env, "GlobalEnum(-2147483649)")
}

func TestStrongEnumsEnumOfString(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, env, "TestAllTypes.NestedEnum('BAZ')", enumP3Nested, 2)
}

func TestStrongEnumsEnumOfStringProto2(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, env, "GlobalEnum('GAR')", enumP2Global, 1)
}

func TestStrongEnumsEnumOfStringUnknown(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	wantEnumEvalError(t, env, "TestAllTypes.NestedEnum('BLETCH')")
}

// Runtime behavior holds for parse-only programs as well.

func TestStrongEnumsParseOnlyConversion(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	out, err := parseEvalEnum(t, env, "TestAllTypes.NestedEnum(2)", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if out.Type().TypeName() != enumP3Nested {
		t.Errorf("parse-only conversion got type %q, want %q", out.Type().TypeName(), enumP3Nested)
	}
}

func TestStrongEnumsParseOnlyLiteral(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	out, err := parseEvalEnum(t, env, "GlobalEnum.GAZ", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if out.Type().TypeName() != enumP2Global {
		t.Errorf("parse-only literal got type %q, want %q", out.Type().TypeName(), enumP2Global)
	}
}

// The mode is opt-in: enabling it changes enum typing, while leaving it out
// or disabling it keeps the legacy integer behavior.

func TestStrongEnumsDefaultRemainsLegacy(t *testing.T) {
	strong := strongEnumEnv(t, enumP2Container)
	wantEnumValue(t, strong, "GlobalEnum.GAZ", enumP2Global, 2)

	legacy := legacyEnumEnv(t, enumP2Container, false)
	out, err := compileEvalEnum(t, legacy, "GlobalEnum.GAZ", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if iv, ok := out.(types.Int); !ok || int64(iv) != 2 {
		t.Errorf("legacy literal got %v (%T), want int 2", out, out)
	}
	out, err = compileEvalEnum(t, legacy, "GlobalEnum.GAZ == 2", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if b, ok := out.(types.Bool); !ok || !bool(b) {
		t.Errorf("legacy equality with int got %v, want true", out)
	}
}

func TestStrongEnumsDisabledRemainsLegacy(t *testing.T) {
	strong := strongEnumEnv(t, enumP3Container)
	wantEnumValue(t, strong, "TestAllTypes{}.standalone_enum", enumP3Nested, 0)

	disabled := legacyEnumEnv(t, enumP3Container, true)
	out, err := compileEvalEnum(t, disabled, "TestAllTypes{}.standalone_enum", nil)
	if err != nil {
		t.Fatalf("eval errored: %v", err)
	}
	if iv, ok := out.(types.Int); !ok || int64(iv) != 0 {
		t.Errorf("disabled select got %v (%T), want int 0", out, out)
	}
}

// Checked compilation agrees with the runtime types.

func TestStrongEnumsCheckerConstantType(t *testing.T) {
	env := strongEnumEnv(t, enumP2Container)
	ast, iss := env.Compile("GlobalEnum.GAZ")
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	if ast.OutputType().TypeName() != enumP2Global {
		t.Errorf("checked output type got %q, want %q", ast.OutputType().TypeName(), enumP2Global)
	}
}

func TestStrongEnumsCheckerFieldSelectType(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	ast, iss := env.Compile("TestAllTypes{}.standalone_enum")
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	if ast.OutputType().TypeName() != enumP3Nested {
		t.Errorf("checked output type got %q, want %q", ast.OutputType().TypeName(), enumP3Nested)
	}
}

func TestStrongEnumsCheckerConversionType(t *testing.T) {
	env := strongEnumEnv(t, enumP3Container)
	ast, iss := env.Compile("TestAllTypes.NestedEnum('BAZ')")
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	if ast.OutputType().TypeName() != enumP3Nested {
		t.Errorf("checked output type got %q, want %q", ast.OutputType().TypeName(), enumP3Nested)
	}
	ast, iss = env.Compile("int(GlobalEnum.GOO)")
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	if ast.OutputType() != IntType {
		t.Errorf("checked int(enum) type got %v, want int", ast.OutputType())
	}
}

func TestStrongEnumsLegacyCheckerUnchanged(t *testing.T) {
	strong := strongEnumEnv(t, enumP2Container)
	sast, siss := strong.Compile("GlobalEnum.GAZ")
	if siss.Err() != nil {
		t.Fatalf("Compile failed: %v", siss.Err())
	}
	if sast.OutputType().TypeName() != enumP2Global {
		t.Errorf("strong checked type got %q, want %q", sast.OutputType().TypeName(), enumP2Global)
	}

	legacy := legacyEnumEnv(t, enumP2Container, false)
	ast, iss := legacy.Compile("GlobalEnum.GAZ")
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	if ast.OutputType() != IntType {
		t.Errorf("legacy checked type got %v, want int", ast.OutputType())
	}
}
