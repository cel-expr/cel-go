// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package celtest provides functions for testing CEL policies and expressions.
package celtest

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/policy"
	"cel.dev/cel-go/test"
	"cel.dev/cel-go/tools/compiler"

	"go.yaml.in/yaml/v3"

	conformancepb "cel.dev/expr/conformance/test"
)

type testCase struct {
	name                  string
	celExpression         string
	testSuitePath         string
	fileDescriptorSetPath string
	configPath            string
	opts                  []any
}

func setupTests() []*testCase {
	testCases := []*testCase{
		{
			name:          "policy test with custom policy parser",
			celExpression: "../../policy/testdata/k8s/policy.yaml",
			testSuitePath: "../../policy/testdata/k8s/tests.yaml",
			configPath:    "../../policy/testdata/k8s/config.yaml",
			opts:          []any{k8sParserOpts()},
		},
		{
			name:          "policy test with function binding",
			celExpression: "../../policy/testdata/restricted_destinations/policy.yaml",
			testSuitePath: "../../policy/testdata/restricted_destinations/tests.yaml",
			configPath:    "../../policy/testdata/restricted_destinations/config.yaml",
			opts:          []any{locationCodeEnvOption()},
		},
		{
			name:          "policy test with custom policy metadata",
			celExpression: "testdata/custom_policy.celpolicy",
			testSuitePath: "testdata/custom_policy_tests.yaml",
			opts:          []any{customPolicyParserOption(), compiler.PolicyMetadataEnvOption(ParsePolicyVariables)},
		},
		{
			name:          "raw expression file test",
			celExpression: "testdata/raw_expr.cel",
			testSuitePath: "testdata/raw_expr_tests.yaml",
			configPath:    "testdata/config.yaml",
			opts:          []any{fnEnvOption()},
		},
		{
			name:          "raw expression test",
			celExpression: "a || i + fn(j) == 42",
			testSuitePath: "testdata/raw_expr_tests.yaml",
			configPath:    "testdata/config.yaml",
			opts:          []any{fnEnvOption()},
		},
	}
	return testCases
}

func locationCodeEnvOption() cel.EnvOption {
	return cel.Function("locationCode",
		cel.Overload("locationCode_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(locationCode)))
}

func locationCode(ip ref.Val) ref.Val {
	switch ip.(types.String) {
	case "10.0.0.1":
		return types.String("us")
	case "10.0.0.2":
		return types.String("de")
	default:
		return types.String("ir")
	}
}

func k8sParserOpts() policy.ParserOption {
	return func(p *policy.Parser) (*policy.Parser, error) {
		p.TagVisitor = policy.K8sTestTagHandler()
		return p, nil
	}
}

// TestTriggerTestsWithRunnerOptions tests the TriggerTestsFromCompiler function for a custom policy
// by providing test runner and compiler options without setting the flag variables.
func TestTriggerTestsWithRunnerOptions(t *testing.T) {
	t.Run("test trigger tests custom policy", func(t *testing.T) {
		envOpt := compiler.EnvironmentFile("../../policy/testdata/k8s/config.yaml")
		testSuite := TestSuite("../../policy/testdata/k8s/tests.yaml")
		testCELPolicy := TestExpression("../../policy/testdata/k8s/policy.yaml")
		c, err := compiler.NewCompiler(envOpt, k8sParserOpts())
		if err != nil {
			t.Fatalf("compiler.NewCompiler() failed: %v", err)
		}
		compilerOpt := CustomTestCompiler(c)
		opts := []TestRunnerOption{compilerOpt, testSuite, testCELPolicy}
		TriggerTests(t, opts...)
	})
}

func customPolicyParserOption() policy.ParserOption {
	return func(p *policy.Parser) (*policy.Parser, error) {
		p.TagVisitor = customTagHandler{TagVisitor: policy.DefaultTagVisitor()}
		return p, nil
	}
}

func ParsePolicyVariables(metadata map[string]any) cel.EnvOption {
	var variables []*decls.VariableDecl
	for n, t := range metadata {
		variables = append(variables, decls.NewVariable(n, parseCustomPolicyVariableType(t.(string))))
	}
	return cel.VariableDecls(variables...)
}

func parseCustomPolicyVariableType(t string) *types.Type {
	switch t {
	case "int":
		return types.IntType
	case "string":
		return types.StringType
	default:
		return types.UnknownType
	}
}

type variableType struct {
	VariableName string `yaml:"variable_name"`
	VariableType string `yaml:"variable_type"`
}

type customTagHandler struct {
	policy.TagVisitor
}

func (customTagHandler) PolicyTag(ctx policy.ParserContext, id int64, tagName string, node *yaml.Node, p *policy.Policy) {
	switch tagName {
	case "variable_types":
		var varList []*variableType
		if err := node.Decode(&varList); err != nil {
			ctx.ReportErrorAtID(id, "invalid yaml variable_types node: %v, error: %v", node, err)
			return
		}
		for _, v := range varList {
			p.SetMetadata(v.VariableName, v.VariableType)
		}
	default:
		ctx.ReportErrorAtID(id, "unsupported policy tag: %s", tagName)
	}
}

func fnEnvOption() cel.EnvOption {
	return cel.Function("fn",
		cel.Overload("fn_int", []*cel.Type{cel.IntType}, cel.IntType,
			cel.UnaryBinding(func(in ref.Val) ref.Val {
				i := in.(types.Int)
				return i / types.Int(2)
			})))
}

// TestTriggerTests tests different scenarios of the TriggerTestsFromCompiler function.
func TestTriggerTests(t *testing.T) {
	for _, tc := range setupTests() {
		t.Run(tc.name, func(t *testing.T) {
			var testOpts []TestRunnerOption
			compileOpts := make([]any, 0, len(tc.opts)+2)
			for _, opt := range tc.opts {
				compileOpts = append(compileOpts, opt)
			}
			if tc.fileDescriptorSetPath != "" {
				compileOpts = append(compileOpts, compiler.TypeDescriptorSetFile(tc.fileDescriptorSetPath))
			}
			if tc.configPath != "" {
				compileOpts = append(compileOpts, compiler.EnvironmentFile(tc.configPath))
			}
			testOpts = append(testOpts,
				TestCompiler(compileOpts...),
				FileDescriptorSet(tc.fileDescriptorSetPath),
				TestSuite(tc.testSuitePath),
				TestExpression(tc.celExpression),
				PartialEvalProgramOption(),
			)
			TriggerTests(t, testOpts...)
		})
	}
}

// TestCustomTestSuiteParser triggers the test runner where the tests are provided by a custom
// test suite parser configured using TestSuiteParserOption.
func TestCustomTestSuiteParser(t *testing.T) {
	t.Run("test custom test suite parser", func(t *testing.T) {
		celExpr := "a || i + fn(j) == 42"
		compilerOpts := []any{compiler.EnvironmentFile("testdata/config.yaml"), fnEnvOption()}
		testRunnerOpts := []TestRunnerOption{
			TestCompiler(compilerOpts...),
			TestExpression(celExpr),
			TestSuiteParserOption(&tsparser{}),
		}
		TriggerTests(t, testRunnerOpts...)
	})
}

type tsparser struct {
	TestSuiteParser
}

// ParseTextproto implements the ParseTextproto method of the TestSuiteParser interface.
func (p *tsparser) ParseTextproto(_ string) (*conformancepb.TestSuite, error) {
	return nil, nil
}

// ParseYAML implements the ParseYAML method of the TestSuiteParser interface.
func (p *tsparser) ParseYAML(_ string) (*test.Suite, error) {
	testCase := &test.Case{
		Name: "sample test case",
		Input: map[string]*test.InputValue{
			"i": {Value: 21},
			"j": {Value: 42},
			"a": {Value: false},
		},
		Output: &test.Output{Value: true},
	}
	testSection := &test.Section{
		Name:  "sample test section",
		Tests: []*test.Case{testCase},
	}
	suite := &test.Suite{
		Description: "sample test suite",
		Sections:    []*test.Section{testSection},
	}
	return suite, nil
}
