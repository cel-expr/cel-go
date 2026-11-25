// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package env_conformance_test

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/env"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/ext"
	valuepb "cel.dev/expr"
	testpb "cel.dev/expr/conformance/test"
	envtestpb "github.com/cel-expr/cel-conformance/test/env"
)

// stringListFlag is a comma-separated flag.Getter that appends values across
// repeated flag invocations.
type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(v string) error {
	for part := range strings.SplitSeq(v, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			*f = append(*f, trimmed)
		}
	}
	return nil
}

func (f *stringListFlag) Get() any {
	return ([]string)(*f)
}

var (
	tests     stringListFlag
	skipTests stringListFlag
)

func init() {
	flag.Var(&tests, "tests", "Comma-separated list of test suite textproto runfiles paths.")
	flag.Var(&skipTests, "skip_tests", "Comma-separated list of tests (or suite/section prefixes) to skip.")
}

func shouldSkipTest(name string, skips []string) bool {
	for _, skip := range skips {
		skip = strings.TrimSuffix(skip, "/")
		if skip == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(name, skip); ok && (rest == "" || strings.HasPrefix(rest, "/")) {
			return true
		}
	}
	return false
}

func loadSuite(runfilePath string) (*envtestpb.TestSuite, error) {
	absPath, err := runfiles.Rlocation(runfilePath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve runfiles path %q: %w", runfilePath, err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test suite file %q: %w", absPath, err)
	}
	suite := &envtestpb.TestSuite{}
	if err := prototext.Unmarshal(data, suite); err != nil {
		return nil, fmt.Errorf("failed to parse test suite textproto %q: %w", absPath, err)
	}
	return suite, nil
}

func decodeSingleYAML(input string) (any, error) {
	dec := yaml.NewDecoder(strings.NewReader(input))
	var doc any
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	return doc, nil
}

func diffYAML(wantYAML, gotYAML string) (string, error) {
	want, err := decodeSingleYAML(wantYAML)
	if err != nil {
		return "", fmt.Errorf("failed to parse want YAML: %w", err)
	}
	got, err := decodeSingleYAML(gotYAML)
	if err != nil {
		return "", fmt.Errorf("failed to parse got YAML: %w", err)
	}
	return cmp.Diff(want, got), nil
}

func TestShouldSkipTest(t *testing.T) {
	skips := []string{
		"type_signatures/primitives/bool",
		"type_signatures/timestamp",
		"other_suite/section/",
		"",
	}
	cases := []struct {
		name string
		want bool
	}{
		{name: "type_signatures/primitives/bool", want: true},
		{name: "type_signatures/primitives/bool_extra", want: false},
		{name: "type_signatures/timestamp/timestamp_alias", want: true},
		{name: "type_signatures/timestamps/foo", want: false},
		{name: "other_suite/section/case1", want: true},
		{name: "type_signatures/primitives/int", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipTest(tc.name, skips); got != tc.want {
				t.Errorf("shouldSkipTest(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestEnvConformance(t *testing.T) {
	if len(tests) == 0 {
		t.Fatal("no --tests specified; run via the env_conformance_test Bazel target")
	}

	for _, suitePath := range tests {
		suite, err := loadSuite(suitePath)
		if err != nil {
			t.Fatalf("loadSuite(%q) failed: %v", suitePath, err)
		}

		suiteName := suite.GetName()
		var testCount int
		for _, section := range suite.GetSections() {
			sectionName := section.GetName()
			for _, tc := range section.GetTests() {
				testCount++
				fullTestName := fmt.Sprintf("%s/%s/%s", suiteName, sectionName, tc.GetName())
				t.Run(fullTestName, func(t *testing.T) {
					runEnvTestCase(t, fullTestName, tc)
				})
			}
		}
		if testCount == 0 {
			t.Fatalf("test suite %q contained no test cases", suitePath)
		}
	}
}

func runEnvTestCase(t *testing.T, fullTestName string, tc *envtestpb.TestCase) {
	if shouldSkipTest(fullTestName, skipTests) {
		t.Skip("skipped via --skip_tests")
	}

	var (
		cfg    *env.Config
		celEnv *cel.Env
		cfgErr error
		// errSrc names the call that produced cfgErr for use in failure messages.
		errSrc string
	)
	switch cfgInput := tc.GetEnvConfig().(type) {
	case *envtestpb.TestCase_EnvYaml:
		errSrc = "env.ConfigFromYAML()"
		cfg, cfgErr = env.ConfigFromYAML([]byte(cfgInput.EnvYaml))
		if cfgErr == nil {
			errSrc = "Config.Validate()"
			cfgErr = cfg.Validate()
		}
		if cfgErr == nil {
			errSrc = "cel.NewCustomEnv(cel.FromConfig())"
			celEnv, cfgErr = cel.NewCustomEnv(cel.FromConfig(cfg, ext.ExtensionOptionFactory))
		}
	case *envtestpb.TestCase_Env:
		t.Fatal("proto Environment input is not yet supported")
	default:
		t.Fatal("no env_config specified")
	}

	output := tc.GetOutput()
	hasExportedYAML := output.GetExportedEnvYaml() != ""
	hasExpectedErrors := len(output.GetErrors()) > 0
	exprTests := tc.GetExprTests()
	hasExprTests := len(exprTests) > 0

	if !hasExportedYAML && !hasExpectedErrors && !hasExprTests {
		t.Fatal("no expected output or expr_tests specified")
	}
	if hasExportedYAML && hasExpectedErrors {
		t.Fatal("test case specifies both exported_env_yaml and errors")
	}

	if hasExpectedErrors {
		if hasExprTests {
			t.Fatal("test case specifies both errors and expr_tests")
		}
		if cfgErr == nil {
			t.Fatalf("%s succeeded, want error matching %q", errSrc, output.GetErrors())
		}
		for _, wantErr := range output.GetErrors() {
			if wantErr == "" {
				t.Fatal("empty expected error string in test case")
			}
			if !strings.Contains(cfgErr.Error(), wantErr) {
				t.Errorf("%s error = %v, want substring %q", errSrc, cfgErr, wantErr)
			}
		}
		return
	}

	if cfgErr != nil {
		t.Fatalf("%s failed: %v", errSrc, cfgErr)
	}

	if hasExportedYAML {
		gotYAML, err := env.ConfigToYAML(cfg)
		if err != nil {
			t.Fatalf("env.ConfigToYAML() failed: %v", err)
		}
		diff, err := diffYAML(output.GetExportedEnvYaml(), string(gotYAML))
		if err != nil {
			t.Fatalf("diffYAML() failed: %v", err)
		}
		if diff != "" {
			t.Errorf("exported env YAML mismatch (-want +got):\n%s\ngot YAML:\n%s\nwant YAML:\n%s",
				diff, string(gotYAML), output.GetExportedEnvYaml())
		}
	}

	for _, exprTest := range exprTests {
		exprTestName := fmt.Sprintf("%s/%s", fullTestName, exprTest.GetName())
		t.Run(exprTest.GetName(), func(t *testing.T) {
			if shouldSkipTest(exprTestName, skipTests) {
				t.Skip("skipped via --skip_tests")
			}
			runExprTestCase(t, celEnv, exprTest)
		})
	}
}

func evalExpr(celEnv *cel.Env, expr string) (ref.Val, error) {
	ast, iss := celEnv.Compile(expr)
	if err := iss.Err(); err != nil {
		return nil, fmt.Errorf("celEnv.Compile(%q) failed: %w", expr, err)
	}
	prg, err := celEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("celEnv.Program(%q) failed: %w", expr, err)
	}
	val, _, err := prg.Eval(cel.NoVars())
	if err != nil {
		return nil, fmt.Errorf("prg.Eval(%q) failed: %w", expr, err)
	}
	return val, nil
}

func buildExprInput(celEnv *cel.Env, tc *testpb.TestCase) (any, error) {
	if ic := tc.GetInputContext(); ic != nil {
		if len(tc.GetInput()) > 0 {
			return nil, errors.New("test case specifies both input and input_context")
		}
		var ctxVal ref.Val
		switch k := ic.GetInputContextKind().(type) {
		case *testpb.InputContext_ContextExpr:
			var err error
			ctxVal, err = evalExpr(celEnv, k.ContextExpr)
			if err != nil {
				return nil, fmt.Errorf("input_context expr failed: %w", err)
			}
		case *testpb.InputContext_ContextMessage:
			ctxVal = celEnv.CELTypeAdapter().NativeToValue(k.ContextMessage)
		default:
			return nil, fmt.Errorf("unsupported input_context kind: %T", ic.GetInputContextKind())
		}
		ctxMsg, err := ctxVal.ConvertToNative(reflect.TypeFor[proto.Message]())
		if err != nil {
			return nil, fmt.Errorf("input_context is not a proto.Message: %w", err)
		}
		pbMsg, ok := ctxMsg.(proto.Message)
		if !ok {
			return nil, fmt.Errorf("input_context converted to %T, want proto.Message", ctxMsg)
		}
		return cel.ContextProtoVars(pbMsg)
	}

	vars := make(map[string]any, len(tc.GetInput()))
	for name, in := range tc.GetInput() {
		switch k := in.GetKind().(type) {
		case *testpb.InputValue_Value:
			val, err := cel.ProtoAsValue(celEnv.CELTypeAdapter(), k.Value)
			if err != nil {
				return nil, fmt.Errorf("cel.ProtoAsValue(%q) failed: %w", name, err)
			}
			vars[name] = val
		case *testpb.InputValue_Expr:
			val, err := evalExpr(celEnv, k.Expr)
			if err != nil {
				return nil, fmt.Errorf("input %q expr failed: %w", name, err)
			}
			vars[name] = val
		default:
			return nil, fmt.Errorf("unsupported input value kind for %q: %T", name, in.GetKind())
		}
	}
	return vars, nil
}

func diffValue(want, got *valuepb.Value) string {
	return cmp.Diff(
		want,
		got,
		protocmp.Transform(),
		protocmp.SortRepeatedFields(&valuepb.MapValue{}, "entries"),
	)
}

func runExprTestCase(t *testing.T, celEnv *cel.Env, tc *testpb.TestCase) {
	if tc.GetEnv() != nil {
		t.Fatal("per-expr_test proto Environment override is not yet supported")
	}

	if tc.GetDisableCheck() && tc.GetDeducedType() != nil {
		t.Fatal("expr_test specifies both disable_check and deduced_type")
	}

	var (
		ast *cel.Ast
		iss *cel.Issues
	)
	if tc.GetDisableCheck() {
		ast, iss = celEnv.Parse(tc.GetExpr())
		if err := iss.Err(); err != nil {
			t.Fatalf("parsing %q failed: %v", tc.GetExpr(), err)
		}
	} else {
		ast, iss = celEnv.Compile(tc.GetExpr())
		if err := iss.Err(); err != nil {
			t.Fatalf("compiling %q failed: %v", tc.GetExpr(), err)
		}
	}

	if wantType := tc.GetDeducedType(); wantType != nil {
		gotType, err := types.TypeToProto(ast.OutputType())
		if err != nil {
			t.Fatalf("types.TypeToProto() failed: %v", err)
		}
		if diff := cmp.Diff(wantType, gotType, protocmp.Transform()); diff != "" {
			t.Errorf("deduced type mismatch (-want +got):\n%s", diff)
		}
	}

	output := tc.GetOutput()
	if output == nil {
		if tc.GetDeducedType() == nil {
			t.Fatal("expr_test specifies neither output nor deduced_type")
		}
		return
	}

	var prgOpts []cel.ProgramOption
	if output.GetUnknown() != nil {
		prgOpts = append(prgOpts, cel.EvalOptions(cel.OptPartialEval))
	}
	prg, err := celEnv.Program(ast, prgOpts...)
	if err != nil {
		t.Fatalf("celEnv.Program(%q) failed: %v", tc.GetExpr(), err)
	}
	vars, err := buildExprInput(celEnv, tc)
	if err != nil {
		t.Fatalf("buildExprInput() failed: %v", err)
	}
	if output.GetUnknown() != nil {
		vars, err = celEnv.PartialVars(vars)
		if err != nil {
			t.Fatalf("celEnv.PartialVars() failed: %v", err)
		}
	}
	got, _, evalErr := prg.Eval(vars)

	switch want := output.GetResultKind().(type) {
	case *testpb.TestOutput_ResultValue:
		if evalErr != nil {
			t.Fatalf("prg.Eval(%q) failed: %v", tc.GetExpr(), evalErr)
		}
		gotProto, err := cel.ValueAsProto(got)
		if err != nil {
			t.Fatalf("cel.ValueAsProto(%v) failed: %v", got, err)
		}
		if diff := diffValue(want.ResultValue, gotProto); diff != "" {
			t.Errorf("prg.Eval(%q) result mismatch (-want +got):\n%s", tc.GetExpr(), diff)
		}
	case *testpb.TestOutput_ResultExpr:
		if evalErr != nil {
			t.Fatalf("prg.Eval(%q) failed: %v", tc.GetExpr(), evalErr)
		}
		wantVal, err := evalExpr(celEnv, want.ResultExpr)
		if err != nil {
			t.Fatalf("evaluating result_expr %q failed: %v", want.ResultExpr, err)
		}
		if got.Equal(wantVal) != types.True {
			t.Errorf("prg.Eval(%q) = %v, want %v", tc.GetExpr(), got, wantVal)
		}
	case *testpb.TestOutput_EvalError:
		if evalErr == nil && types.IsError(got) {
			evalErr = got.(*types.Err)
		}
		if evalErr == nil {
			t.Fatalf("prg.Eval(%q) = %v, want eval_error %v", tc.GetExpr(), got, want.EvalError)
		}
		wantErrs := want.EvalError.GetErrors()
		if len(wantErrs) == 0 {
			return
		}
		matched := false
		for _, e := range wantErrs {
			if e.GetMessage() == "" || strings.Contains(evalErr.Error(), e.GetMessage()) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("prg.Eval(%q) error = %v, want one of %v", tc.GetExpr(), evalErr, wantErrs)
		}
	case *testpb.TestOutput_Unknown:
		if evalErr != nil {
			t.Fatalf("prg.Eval(%q) failed: %v, want unknown %v", tc.GetExpr(), evalErr, want.Unknown)
		}
		if !types.IsUnknown(got) {
			t.Fatalf("prg.Eval(%q) = %v, want unknown %v", tc.GetExpr(), got, want.Unknown)
		}
		gotIDs := slices.Clone(got.(*types.Unknown).IDs())
		wantIDs := slices.Clone(want.Unknown.GetExprs())
		slices.Sort(gotIDs)
		slices.Sort(wantIDs)
		if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
			t.Errorf("prg.Eval(%q) unknown IDs mismatch (-want +got):\n%s", tc.GetExpr(), diff)
		}
	default:
		t.Fatalf("unsupported expr_test output kind: %T", output.GetResultKind())
	}
}
