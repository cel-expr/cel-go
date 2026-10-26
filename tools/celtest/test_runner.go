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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/debug"
	"cel.dev/cel-go/common/env"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/interpreter"
	"cel.dev/cel-go/test"
	"cel.dev/cel-go/tools/compiler"
	"github.com/google/go-cmp/cmp"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/testing/protocmp"

	"go.yaml.in/yaml/v3"

	celpb "cel.dev/expr"
	conformancepb "cel.dev/expr/conformance/test"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	dynamicpb "google.golang.org/protobuf/types/dynamicpb"
)

var (
	celExpression         string
	testSuitePath         string
	fileDescriptorSetPath string
	configPath            string
	baseConfigPath        string
	enableCoverage        bool
	enableDebug           bool
)

func init() {
	flag.StringVar(&testSuitePath, "test_suite_path", "", "path to a test suite")
	flag.StringVar(&fileDescriptorSetPath, "file_descriptor_set", "", "path to a file descriptor set")
	flag.StringVar(&configPath, "config_path", "", "path to a config file")
	flag.StringVar(&baseConfigPath, "base_config_path", "", "path to a base config file")
	flag.StringVar(&celExpression, "cel_expr", "", "CEL expression to test")
	flag.BoolVar(&enableCoverage, "enable_coverage", false, "Enable coverage calculation and reporting.")
	flag.BoolVar(&enableDebug, "celtest_debug", false, "Enables verbose logging of test case execution.")
}

func updateRunfilesPathForFlags(testResourcesDir string) error {
	if testResourcesDir == "" {
		return nil
	}
	paths := make([]*string, 0, 5)
	if compiler.InferFileFormat(testSuitePath) != compiler.Unspecified {
		paths = append(paths, &testSuitePath)
	}
	if compiler.InferFileFormat(fileDescriptorSetPath) != compiler.Unspecified {
		paths = append(paths, &fileDescriptorSetPath)
	}
	if compiler.InferFileFormat(configPath) != compiler.Unspecified {
		paths = append(paths, &configPath)
	}
	if compiler.InferFileFormat(baseConfigPath) != compiler.Unspecified {
		paths = append(paths, &baseConfigPath)
	}
	if compiler.InferFileFormat(celExpression) != compiler.Unspecified {
		paths = append(paths, &celExpression)
	}
	return UpdateTestResourcesPaths(testResourcesDir, paths)
}

// UpdateTestResourcesPaths updates the list of paths with their absolute paths as per their location
// in the testResourcesDir directory. This will allow the executable targets to locate and access the
// data dependencies needed to trigger the tests.
// For example: In case of Bazel, this method can be used to update the file paths with the corresponding
// location in the runfiles directory tree:
//
//	UpdateTestResourcesPaths(os.Getenv("RUNFILES_DIR"), <file_paths_list>)
func UpdateTestResourcesPaths(testResourcesDir string, paths []*string) error {
	if testResourcesDir == "" {
		return nil
	}
	err := filepath.Walk(testResourcesDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(testResourcesDir, p)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if strings.Contains(relPath, *path) {
				*path = p
			}
		}
		return nil
	})
	return err
}

// TestRunnerOption is used to configure the following attributes of the Test Runner:
// - set the Compiler
// - add Input Expressions
// - set the test suite file path
// - set the test suite parser based on the file format: YAML or Textproto
type TestRunnerOption func(*TestRunner) (*TestRunner, error)

// TriggerTests triggers tests for a CEL policy, expression or checked expression
// with the provided set of options. The options can be used to:
// - configure the Compiler used for parsing and compiling the expression
// - configure the Test Runner used for parsing and executing the tests
func TriggerTests(t *testing.T, testRunnerOpts ...TestRunnerOption) {
	tr, err := NewTestRunner(testRunnerOpts...)
	if err != nil {
		t.Fatalf("error creating test runner: %v", err)
	}
	programs, err := tr.Programs(t, tr.testProgramOptions...)
	if err != nil {
		t.Fatalf("error creating programs: %v", err)
	}
	if len(programs) == 0 {
		t.Fatalf("no programs created for the provided expressions")
	}
	tests, err := tr.Tests(t)
	if err != nil {
		t.Fatalf("error creating tests: %v", err)
	}
	if len(tests) == 0 {
		t.Fatalf("no tests found")
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := tr.ExecuteTest(t, programs, test)
			if err != nil {
				t.Fatalf("error executing test: %v", err)
			}
		})
	}
	if tr.EnableCoverage {
		reportCoverage(t, programs)
	}
}

// TestRunnerOptionsFromFlags returns a TestRunnerOption which configures the following attributes
// of the test runner using the parsed flags and the optionally provided test runner and test compiler options:
//   - Test compiler - The `file_descriptor_set`, `base_config_path` and `config_path` flags are used
//     to set up the test compiler. The optionally provided test compiler options are also used to
//     augment the test compiler.
//   - Test suite parser - The `test_suite_path` flag is used to set up the test suite parser.
//   - File descriptor set path - The value of the `file_descriptor_set` flag is set as the
//     File Descriptor Set Path of the test runner.
//   - Test expression - The `cel_expr` flag is used to populate the test expressions which need to be
//     evaluated by the test runner.
//   - Enable coverage - The `enable_coverage` flag is used to enable coverage calculation and reporting.
func TestRunnerOptionsFromFlags(testResourcesDir string, testRunnerOpts []TestRunnerOption, testCompilerOpts ...any) TestRunnerOption {
	if !flag.Parsed() {
		flag.Parse()
	}
	if err := updateRunfilesPathForFlags(testResourcesDir); err != nil {
		return nil
	}
	return func(tr *TestRunner) (*TestRunner, error) {
		opts := []TestRunnerOption{
			testRunnerCompilerFromFlags(testCompilerOpts...),
			TestSuite(testSuitePath),
			FileDescriptorSet(fileDescriptorSetPath),
			TestExpression(celExpression),
		}
		if enableCoverage {
			opts = append(opts, EnableCoverage())
		}
		opts = append(opts, testRunnerOpts...)
		var err error
		for _, opt := range opts {
			tr, err = opt(tr)
			if err != nil {
				return nil, err
			}
		}
		return tr, nil
	}
}

func testRunnerCompilerFromFlags(testCompilerOpts ...any) TestRunnerOption {
	var opts []any
	if fileDescriptorSetPath != "" {
		opts = append(opts, compiler.TypeDescriptorSetFile(fileDescriptorSetPath))
	}
	if baseConfigPath != "" {
		opts = append(opts, compiler.EnvironmentFile(baseConfigPath))
	}
	if configPath != "" {
		opts = append(opts, compiler.EnvironmentFile(configPath))
	}
	if enableCoverage {
		opts = append(opts, cel.EnableMacroCallTracking())
	}
	opts = append(opts, testCompilerOpts...)
	return TestCompiler(opts...)
}

// ActivationFactory is a function which creates a CEL activation from the provided variables.
type ActivationFactory func(vars any) (cel.Activation, error)

// TestInputActivationFactory updates the test runner with the provided activation factory.
// The activation factory is used to create a CEL activation from the provided variables.
func TestInputActivationFactory(f ActivationFactory) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.activationFactory = f
		return tr, nil
	}
}

// TestSuiteParser is an interface for parsing a test suite:
// - ParseTextproto: Returns a cel.spec.expr.conformance.test.TestSuite message.
// - ParseYAML: Returns a test.Suite object.
// In case the test suite is serialized in a Textproto/YAML file, the path of the file is passed as
// an argument to the parse method.
type TestSuiteParser interface {
	ParseTextproto(string) (*conformancepb.TestSuite, error)
	ParseYAML(string) (*test.Suite, error)
}

type tsParser struct {
	TestSuiteParser
}

// ParseTextproto parses a test suite file in Textproto format.
func (p *tsParser) ParseTextproto(path string) (*conformancepb.TestSuite, error) {
	if path == "" {
		return nil, nil
	}
	if fileFormat := compiler.InferFileFormat(path); fileFormat != compiler.TextProto {
		return nil, fmt.Errorf("invalid file extension wanted: .textproto: found %v", fileFormat)
	}
	testSuite := &conformancepb.TestSuite{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile(%q) failed: %v", path, err)
	}
	err = prototext.Unmarshal(data, testSuite)
	return testSuite, err
}

// ParseYAML parses a test suite file in YAML format.
func (p *tsParser) ParseYAML(path string) (*test.Suite, error) {
	if path == "" {
		return nil, nil
	}
	if fileFormat := compiler.InferFileFormat(path); fileFormat != compiler.TextYAML {
		return nil, fmt.Errorf("invalid file extension wanted: .yaml: found %v", fileFormat)
	}
	testSuiteBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile(%q) failed: %v", path, err)
	}
	testSuite := &test.Suite{}
	err = yaml.Unmarshal(testSuiteBytes, testSuite)
	return testSuite, err
}

// TestSuite provides a reference to the test suite file (either .yaml or .textproto)
func TestSuite(path string) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.TestSuiteFilePath = path
		return tr, nil
	}
}

// DefaultTestSuiteParser returns a TestRunnerOption which configures the test runner with
// the default test suite parser.
// Deprecated: prefer TestSuite(path)
func DefaultTestSuiteParser(path string) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		if path == "" {
			return tr, nil
		}
		tr.TestSuiteFilePath = path
		tr.testSuiteParser = &tsParser{}
		return tr, nil
	}
}

// TestSuiteParserOption returns a TestRunnerOption which configures the test runner with
// a custom test suite parser.
func TestSuiteParserOption(p TestSuiteParser) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.testSuiteParser = p
		return tr, nil
	}
}

type typeAdorner struct {
	ast *cel.Ast
}

// Implements type.Adorner
func (a *typeAdorner) GetMetadata(ctx any) string {
	e, ok := ctx.(ast.Expr)
	if !ok {
		return ""
	}
	t := a.ast.NativeRep().GetType(e.ID())
	if t != nil {
		return " : " + env.SerializeTypeDesc(t).SpecifierFormat()
	}
	return " : <no type inferred>"
}

// DebugAST returns a string representation of the AST adorned with type information.
func DebugAST(ast *cel.Ast) string {
	if ast == nil {
		return "<nil>"
	}
	adorner := &typeAdorner{ast: ast}
	return debug.ToAdornedDebugString(ast.NativeRep().Expr(), adorner)
}

// TestRunner provides a structure to hold the different components required to execute tests for
// a list of Input Expressions. The TestRunner can be configured with the following options:
// - Compiler: The compiler used for parsing and compiling the input expressions.
// - Input Expressions: The list of input expressions to be tested.
// - Test Suite File Path: The path to the test suite file.
// - File Descriptor Set Path: The path to the file descriptor set file.
// - test Suite Parser: A parser for a test suite file serialized in Textproto/YAML format.
// - test Program Options: A list of options to be used when creating the CEL programs.
// - EnableCoverage: A boolean to enable coverage calculation.
//
// The TestRunner provides the following methods:
// - Programs: Creates a list of CEL programs from the input expressions.
// - Tests: Creates a list of tests from the test suite file.
// - ExecuteTest: Executes a single
type TestRunner struct {
	compiler.Compiler
	Expressions       []compiler.InputExpression
	TestSuiteFilePath string
	FileDescriptorSet *descpb.FileDescriptorSet
	EnableCoverage    bool

	activationFactory  ActivationFactory
	testSuiteParser    TestSuiteParser
	testProgramOptions []cel.ProgramOption
}

// Test represents a single test case to be executed. It encompasses the following:
// - name: The name of the test case.
// - input: The input to be used for evaluating the CEL expression.
// - resultMatcher: A function that takes in the result of evaluating the CEL expression and
// returns a TestResult.
type Test struct {
	name          string
	input         cel.PartialActivation
	resultMatcher func(ref.Val, error) TestResult
}

// NewTest creates a new Test with the provided name, input and result matcher.
func NewTest(name string, input cel.PartialActivation, resultMatcher func(ref.Val, error) TestResult) *Test {
	return &Test{
		name:          name,
		input:         input,
		resultMatcher: resultMatcher,
	}
}

// Name returns the name of the test case.
func (t *Test) Name() string {
	return t.name
}

// ResultMatcher returns the result matcher function of the test case.
func (t *Test) ResultMatcher() func(ref.Val, error) TestResult {
	return t.resultMatcher
}

// TestResult represents the result of a test case execution. It contains the validation result
// along with the expected result and any errors encountered during the execution.
// - Success: Whether the result matcher condition validating the test case was satisfied.
// - Wanted: The expected result of the test case.
// - Error: Any error encountered during the execution.
type TestResult struct {
	Success bool
	Wanted  string
	Error   error
}

// NewTestRunner creates a Test Runner with the provided options.
// The options can be used to:
// - configure the Compiler used for parsing and compiling the input expressions
// - configure the Test Runner used for parsing and executing the tests
func NewTestRunner(opts ...TestRunnerOption) (*TestRunner, error) {
	tr := &TestRunner{
		activationFactory: cel.NewActivation,
		testSuiteParser:   &tsParser{},
	}
	var err error
	for _, opt := range opts {
		tr, err = opt(tr)
		if err != nil {
			return nil, err
		}
	}
	return tr, nil
}

// TestExpression returns a TestRunnerOption which configures a policy file, expression file, or raw expression
// for testing
func TestExpression(value string) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		if value != "" {
			tr.Expressions = append(tr.Expressions,
				&compiler.CompiledExpression{Path: value},
				&compiler.FileExpression{Path: value},
				&compiler.RawExpression{Value: value},
			)
		}
		return tr, nil
	}
}

// TestCompiler returns a TestRunnerOption which configures the test runner with
// a compiler created using the set of compiler options.
func TestCompiler(compileOpts ...any) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		c, err := compiler.NewCompiler(compileOpts...)
		if err != nil {
			return nil, err
		}
		tr.Compiler = c
		return tr, nil
	}
}

// CustomTestCompiler returns a TestRunnerOption which configures the test runner with
// a custom compiler.
func CustomTestCompiler(c compiler.Compiler) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.Compiler = c
		return tr, nil
	}
}

// FileDescriptorSet creates a Test Runner Option which adds a file descriptor set to the test
// runner. The file descriptor set is used to register proto messages in the global proto registry.
func FileDescriptorSet(path string) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		if path != "" {
			fds, err := fileDescriptorSet(path)
			if err != nil {
				return nil, err
			}
			tr.FileDescriptorSet = fds
		}
		return tr, nil
	}
}

// AddFileDescriptorSetProto creates a Test Runner Option which adds a file descriptor set proto to
// the test runner. The file descriptor set is used to register proto messages in the global proto
// registry.
func AddFileDescriptorSetProto(fds *descpb.FileDescriptorSet) TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.FileDescriptorSet = fds
		return tr, nil
	}
}

func registerMessages(fds *descpb.FileDescriptorSet) error {
	for _, file := range fds.GetFile() {
		reflectFD, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
		if err != nil {
			return fmt.Errorf("protodesc.NewFile(%q) failed: %v", file.GetName(), err)
		}
		if _, err := protoregistry.GlobalFiles.FindFileByPath(reflectFD.Path()); err == nil {
			continue
		}
		err = protoregistry.GlobalFiles.RegisterFile(reflectFD)
		if err != nil {
			return fmt.Errorf("protoregistry.GlobalFiles.RegisterFile() failed: %v", err)
		}
		for i := 0; i < reflectFD.Messages().Len(); i++ {
			msg := reflectFD.Messages().Get(i)
			msgType := dynamicpb.NewMessageType(msg)
			err = protoregistry.GlobalTypes.RegisterMessage(msgType)
			if err != nil && !strings.Contains(err.Error(), "already registered") {
				return fmt.Errorf("protoregistry.GlobalTypes.RegisterMessage(%q) failed: %v", msgType, err)
			}
		}
	}
	return nil
}

func fileDescriptorSet(path string) (*descpb.FileDescriptorSet, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file descriptor set file %q: %v", fileDescriptorSetPath, err)
	}
	fds := &descpb.FileDescriptorSet{}
	if err := proto.Unmarshal(bytes, fds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file descriptor set file %q: %v", fileDescriptorSetPath, err)
	}
	return fds, nil
}

// PartialEvalProgramOption returns a TestRunnerOption which enables partial evaluation for the CEL
// program by setting the OptPartialEval eval option.
//
// Note: The test setup uses env.PartialVars() for creating PartialActivation.
func PartialEvalProgramOption() TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.testProgramOptions = append(tr.testProgramOptions, cel.EvalOptions(cel.OptPartialEval))
		return tr, nil
	}
}

// EnableCoverage returns a TestRunnerOption which enables coverage calculation for the test run.
func EnableCoverage() TestRunnerOption {
	return func(tr *TestRunner) (*TestRunner, error) {
		tr.EnableCoverage = true
		tr.testProgramOptions = append(tr.testProgramOptions, cel.EvalOptions(cel.OptTrackState))
		return tr, nil
	}
}

// Program represents the result of creating CEL programs for the configured expressions in the
// test runner. It encompasses the following:
// - CELProgram - the evaluable CEL program
// - PolicyMetadata - the metadata map obtained while creating the CEL AST from the expression
// - Ast - the CEL AST created from the expression
// - CoverageStats - the coverage report map obtained from calculating the coverage if enabled.
type Program struct {
	cel.Program
	PolicyMetadata map[string]any
	Ast            *cel.Ast
	CoverageStats  map[int64]set
}

// set is a generic set implementation using a map where the keys are the elements of the set
// and the values are empty structs. This approach is memory-efficient because an empty struct
// occupies zero bytes. This will be used to store the values observed for each expression during
// coverage calculation.
type set map[ref.Val]struct{}

// Programs creates a list of CEL programs from the input expressions configured in the test runner
// using the provided program options.
func (tr *TestRunner) Programs(t *testing.T, opts ...cel.ProgramOption) ([]Program, error) {
	t.Helper()
	if tr.Compiler == nil {
		return nil, fmt.Errorf("compiler is not set")
	}
	e, err := tr.CreateEnv()
	if err != nil {
		return nil, err
	}
	programs := make([]Program, 0, len(tr.Expressions))
	for _, expr := range tr.Expressions {
		ast, policyMetadata, err := expr.CreateAST(tr.Compiler)
		if err != nil {
			if strings.Contains(err.Error(), "invalid file extension") ||
				strings.Contains(err.Error(), "invalid raw expression") {
				continue
			}
			return nil, err
		}
		if enableDebug {
			t.Logf("Loaded AST:\n%s", DebugAST(ast))
		}
		prg, err := e.Program(ast, opts...)
		if err != nil {
			return nil, err
		}
		programs = append(programs, Program{
			Program:        prg,
			PolicyMetadata: policyMetadata,
			Ast:            ast,
		})
	}
	return programs, nil
}

// Tests creates a list of tests from the test suite file and test suite parser configured in the
// test runner.
//
// Note: The test setup uses env.PartialVars() for creating PartialActivation.
func (tr *TestRunner) Tests(t *testing.T) ([]*Test, error) {
	if tr.Compiler == nil {
		return nil, fmt.Errorf("compiler is not set")
	}
	if tr.testSuiteParser == nil {
		return nil, fmt.Errorf("test suite parser is not set")
	}
	if testSuite, err := tr.testSuiteParser.ParseYAML(tr.TestSuiteFilePath); err != nil &&
		!strings.Contains(err.Error(), "invalid file extension") {
		return nil, fmt.Errorf("tr.testSuiteParser.ParseYAML(%q) failed: %v", tr.TestSuiteFilePath, err)
	} else if testSuite != nil {
		return tr.createTestsFromYAML(t, testSuite)
	}
	err := registerMessages(tr.FileDescriptorSet)
	if err != nil {
		return nil, fmt.Errorf("registerMessages(%v) failed: %v", tr.FileDescriptorSet, err)
	}
	if testSuite, err := tr.testSuiteParser.ParseTextproto(tr.TestSuiteFilePath); err != nil &&
		!strings.Contains(err.Error(), "invalid file extension") {
		return nil, fmt.Errorf("tr.testSuiteParser.ParseTextproto(%q) failed: %v", tr.TestSuiteFilePath, err)
	} else if testSuite != nil {
		return tr.createTestsFromTextproto(t, testSuite)
	}
	return nil, nil
}

func (tr *TestRunner) createTestsFromTextproto(t *testing.T, testSuite *conformancepb.TestSuite) ([]*Test, error) {
	var tests []*Test
	for _, section := range testSuite.GetSections() {
		sectionName := section.GetName()
		for _, testCase := range section.GetTests() {
			testName := fmt.Sprintf("%s/%s", sectionName, testCase.GetName())
			testInput, err := tr.createTestInputFromPB(t, testCase)
			if err != nil {
				return nil, err
			}
			testResultMatcher, err := tr.createResultMatcherFromPB(t, testCase)
			if err != nil {
				return nil, err
			}
			tests = append(tests, NewTest(testName, testInput, testResultMatcher))
		}
	}
	return tests, nil
}

func (tr *TestRunner) createTestInputFromPB(t *testing.T, testCase *conformancepb.TestCase) (cel.PartialActivation, error) {
	t.Helper()
	input := map[string]any{}
	e, err := tr.CreateEnv()
	if err != nil {
		return nil, err
	}
	var activation cel.Activation
	if testCase.GetInputContext() != nil {
		if len(testCase.GetInput()) != 0 {
			return nil, fmt.Errorf("only one of input and input_context can be provided at a time")
		}
		switch testInput := testCase.GetInputContext().GetInputContextKind().(type) {
		case *conformancepb.InputContext_ContextExpr:
			refVal, err := tr.eval(testInput.ContextExpr)
			if err != nil {
				return nil, fmt.Errorf("eval(%q) failed: %w", testInput.ContextExpr, err)
			}
			ctx, err := refVal.ConvertToNative(
				reflect.TypeFor[proto.Message]())
			if err != nil {
				return nil, fmt.Errorf("context variable is not a valid proto: %w", err)
			}
			activation, err = cel.ContextProtoVars(ctx.(proto.Message))
			if err != nil {
				return nil, fmt.Errorf("cel.ContextProtoVars() failed: %w", err)
			}
		case *conformancepb.InputContext_ContextMessage:
			refVal := e.CELTypeAdapter().NativeToValue(testInput.ContextMessage)
			ctx, err := refVal.ConvertToNative(reflect.TypeFor[proto.Message]())
			if err != nil {
				return nil, fmt.Errorf("context variable is not a valid proto: %w", err)
			}
			activation, err = cel.ContextProtoVars(ctx.(proto.Message))
			if err != nil {
				return nil, fmt.Errorf("cel.ContextProtoVars() failed: %w", err)
			}
		}
		return e.PartialVars(activation)
	}
	for k, v := range testCase.GetInput() {
		switch v.GetKind().(type) {
		case *conformancepb.InputValue_Value:
			input[k], err = cel.ProtoAsValue(e.CELTypeAdapter(), v.GetValue())
			if err != nil {
				return nil, fmt.Errorf("cel.ProtoAsValue(%q) failed: %w", v, err)
			}
		case *conformancepb.InputValue_Expr:
			input[k], err = tr.eval(v.GetExpr())
			if err != nil {
				return nil, fmt.Errorf("eval(%q) failed: %w", v.GetExpr(), err)
			}
		}
	}
	activation, err = tr.activationFactory(input)
	if err != nil {
		return nil, fmt.Errorf("activationFactory(%q) failed: %w", input, err)
	}
	return e.PartialVars(activation)
}

func (tr *TestRunner) createResultMatcherFromPB(t *testing.T, testCase *conformancepb.TestCase) (func(ref.Val, error) TestResult, error) {
	t.Helper()
	if testCase.GetOutput() == nil {
		return nil, fmt.Errorf("expected output is nil")
	}
	successResult := TestResult{Success: true}
	e, err := tr.CreateEnv()
	if err != nil {
		return nil, err
	}
	switch testOutput := testCase.GetOutput().GetResultKind().(type) {
	case *conformancepb.TestOutput_ResultValue:
		return func(val ref.Val, err error) TestResult {
			want := e.CELTypeAdapter().NativeToValue(testOutput.ResultValue)
			if err != nil {
				return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: err}
			}
			if optVal, ok := val.(*types.Optional); ok && optVal.HasValue() {
				val = optVal.GetValue()
			}
			outputVal, err := refValueToExprValue(val)
			if err != nil {
				return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: fmt.Errorf("refValueToExprValue(%q) failed: %v", val, err)}
			}
			testResultVal, err := canonicalValueToV1Alpha1(testOutput.ResultValue)
			if err != nil {
				return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: fmt.Errorf("canonicalValueToV1Alpha1(%q) failed: %v", testOutput.ResultValue, err)}
			}
			testVal := &exprpb.ExprValue{
				Kind: &exprpb.ExprValue_Value{Value: testResultVal}}

			if diff := cmp.Diff(testVal, outputVal, protocmp.Transform(),
				protocmp.SortRepeatedFields(&exprpb.MapValue{}, "entries")); diff != "" {
				return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: fmt.Errorf("mismatched test output with diff (-want +got):\n%s", diff)}
			}
			return successResult
		}, nil
	case *conformancepb.TestOutput_ResultExpr:
		return func(got ref.Val, err error) TestResult {
			if err != nil {
				return TestResult{Success: false, Error: err}
			}
			want, err := tr.eval(testOutput.ResultExpr)
			if err != nil {
				return TestResult{Success: false, Error: fmt.Errorf("eval(%q) failed: %v", testOutput.ResultExpr, err)}
			}
			if gotOpt, ok := got.(*types.Optional); ok {
				if gotOpt.Equal(types.OptionalNone) == types.True {
					if want.Equal(types.OptionalNone) == types.True {
						return successResult
					}
					return TestResult{Success: false, Wanted: fmt.Sprintf("optional value %v", want), Error: fmt.Errorf("policy eval got %v", got)}
				}
				if want.Equal(gotOpt.GetValue()) == types.True {
					if enableDebug {
						t.Log("result implicitly unwrapped optional\n")
					}
					return successResult
				}
			}
			if got.Equal(want) == types.True {
				return successResult
			}
			return TestResult{Success: false, Wanted: fmt.Sprintf("optional value %v", want), Error: fmt.Errorf("policy eval got %v", got)}
		}, nil
	case *conformancepb.TestOutput_EvalError:
		return func(val ref.Val, err error) TestResult {
			failureResult := TestResult{Success: false, Wanted: fmt.Sprintf("error %v", testOutput.EvalError)}
			if err == nil {
				return failureResult
			}
			// Compare the evaluated error with the expected error message only.
			for _, want := range testOutput.EvalError.GetErrors() {
				if strings.Contains(err.Error(), want.GetMessage()) {
					return successResult
				}
			}
			return failureResult
		}, nil
	case *conformancepb.TestOutput_Unknown:
		// Validate that all expected unknown expression ids are returned by the evaluation result.
		return func(out ref.Val, err error) TestResult {
			expectedUnknownIDs := testOutput.Unknown.GetExprs()
			if err == nil && types.IsUnknown(out) {
				actualUnknownIDs := out.Value().(*types.Unknown).IDs()
				return compareUnknownIDs(expectedUnknownIDs, actualUnknownIDs)
			}
			return TestResult{Success: false, Wanted: fmt.Sprintf("unknown value %v", expectedUnknownIDs), Error: err}
		}, nil
	}
	return nil, nil
}

func compareUnknownIDs(expectedUnknownIDs, actualUnknownIDs []int64) TestResult {
	sortOption := cmp.Transformer("Sort", func(in []int64) []int64 {
		out := append([]int64{}, in...)
		slices.Sort(out)
		return out
	})
	if diff := cmp.Diff(expectedUnknownIDs, actualUnknownIDs, sortOption); diff != "" {
		return TestResult{
			Success: false,
			Wanted:  fmt.Sprintf("unknown value %v", expectedUnknownIDs),
			Error:   fmt.Errorf("mismatched test output with diff (-got +want):\n%s", diff)}
	}
	return TestResult{Success: true}
}

func refValueToExprValue(refVal ref.Val) (*exprpb.ExprValue, error) {
	if types.IsUnknown(refVal) {
		return &exprpb.ExprValue{
			Kind: &exprpb.ExprValue_Unknown{
				Unknown: &exprpb.UnknownSet{
					Exprs: refVal.Value().([]int64),
				},
			}}, nil
	}
	v, err := cel.RefValueToValue(refVal)
	if err != nil {
		return nil, err
	}
	return &exprpb.ExprValue{
		Kind: &exprpb.ExprValue_Value{Value: v}}, nil
}

func canonicalValueToV1Alpha1(val *celpb.Value) (*exprpb.Value, error) {
	var v1val exprpb.Value
	b, err := prototext.Marshal(val)
	if err != nil {
		return nil, err
	}
	if err := prototext.Unmarshal(b, &v1val); err != nil {
		return nil, err
	}
	return &v1val, nil
}

func (tr *TestRunner) eval(expr string) (ref.Val, error) {
	e, err := tr.CreateEnv()
	if err != nil {
		return nil, err
	}
	e, err = e.Extend(cel.OptionalTypes())
	if err != nil {
		return nil, fmt.Errorf("e.Extend() failed: %v", err)
	}
	ast, iss := e.Compile(expr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("e.Compile(%q) failed: %v", expr, iss.Err())
	}
	prg, err := e.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("e.Program(%q) failed: %v", expr, err)
	}
	out, _, err := prg.Eval(cel.NoVars())
	if err != nil {
		return nil, fmt.Errorf("prg.Eval(%q) failed: %v", expr, err)
	}
	return out, nil
}

func (tr *TestRunner) createTestsFromYAML(t *testing.T, testSuite *test.Suite) ([]*Test, error) {
	var tests []*Test
	for _, section := range testSuite.Sections {
		for _, testCase := range section.Tests {
			testName := fmt.Sprintf("%s/%s", section.Name, testCase.Name)
			testInput, err := tr.createTestInput(t, testCase)
			if err != nil {
				return nil, err
			}
			testResultMatcher, err := tr.createResultMatcher(t, testCase.Output)
			if err != nil {
				return nil, err
			}
			tests = append(tests, NewTest(testName, testInput, testResultMatcher))
		}
	}
	return tests, nil
}

func (tr *TestRunner) createTestInput(t *testing.T, testCase *test.Case) (cel.PartialActivation, error) {
	t.Helper()
	e, err := tr.CreateEnv()
	if err != nil {
		return nil, err
	}
	var activation cel.Activation
	if testCase.InputContext != nil && testCase.InputContext.ContextExpr != "" {
		if len(testCase.Input) != 0 {
			return nil, fmt.Errorf("only one of input and input_context can be provided at a time")
		}
		contextExpr := testCase.InputContext.ContextExpr
		out, err := tr.eval(contextExpr)
		if err != nil {
			return nil, fmt.Errorf("eval(%q) failed: %w", contextExpr, err)
		}
		ctx, err := out.ConvertToNative(reflect.TypeFor[proto.Message]())
		if err != nil {
			return nil, fmt.Errorf("context variable is not a valid proto: %w", err)
		}
		activation, err = cel.ContextProtoVars(ctx.(proto.Message))
		if err != nil {
			return nil, fmt.Errorf("cel.ContextProtoVars() failed: %w", err)
		}
		return e.PartialVars(activation)
	}
	input := map[string]any{}
	for k, v := range testCase.Input {
		if v.Expr != "" {
			val, err := tr.eval(v.Expr)
			if err != nil {
				return nil, fmt.Errorf("eval(%q) failed: %w", v.Expr, err)
			}
			input[k] = val
			continue
		}
		input[k] = v.Value
	}
	activation, err = tr.activationFactory(input)
	if err != nil {
		return nil, fmt.Errorf("activationFactory(%q) failed: %w", input, err)
	}
	return e.PartialVars(activation)
}

func (tr *TestRunner) createResultMatcher(t *testing.T, testOutput *test.Output) (func(ref.Val, error) TestResult, error) {
	t.Helper()
	e, err := tr.CreateEnv()
	if err != nil {
		return nil, err
	}
	successResult := TestResult{Success: true}
	if testOutput.Value != nil {
		want := e.CELTypeAdapter().NativeToValue(testOutput.Value)
		return func(out ref.Val, err error) TestResult {
			if err != nil {
				return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: err}
			}
			if out.Equal(want) == types.True {
				return successResult
			}
			if optOut, ok := out.(*types.Optional); ok {
				if optOut.HasValue() && optOut.GetValue().Equal(want) == types.True {
					return successResult
				}
			}
			return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: fmt.Errorf("policy eval got %v", out)}

		}, nil
	}
	if testOutput.Expr != "" {
		want, err := tr.eval(testOutput.Expr)
		if err != nil {
			return nil, fmt.Errorf("eval(%q) failed: %w", testOutput.Expr, err)
		}
		return func(out ref.Val, err error) TestResult {
			if err != nil {
				return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: err}
			}
			if out.Equal(want) == types.True {
				return successResult
			}
			if optOut, ok := out.(*types.Optional); ok {
				if optOut.HasValue() && optOut.GetValue().Equal(want) == types.True {
					return successResult
				}
			}
			return TestResult{Success: false, Wanted: fmt.Sprintf("simple value %v", want), Error: fmt.Errorf("policy eval got %v", out)}

		}, nil
	}
	if testOutput.ErrorSet != nil {
		return func(out ref.Val, err error) TestResult {
			failureResult := TestResult{Success: false, Wanted: fmt.Sprintf("error %v", testOutput.ErrorSet)}
			if err == nil {
				return failureResult
			}
			for _, want := range testOutput.ErrorSet {
				if strings.Contains(err.Error(), want) {
					return successResult
				}
			}
			return failureResult
		}, nil
	}
	if testOutput.UnknownSet != nil {
		return func(out ref.Val, err error) TestResult {
			if err == nil && types.IsUnknown(out) {
				unknownIDs := out.Value().(*types.Unknown).IDs()
				return compareUnknownIDs(testOutput.UnknownSet, unknownIDs)
			}
			return TestResult{Success: false, Wanted: fmt.Sprintf("unknown value %v", testOutput.UnknownSet), Error: err}
		}, nil
	}
	return nil, nil
}

// ExecuteTest executes the test case against the provided list of programs and returns an error if
// the test fails.
// During the test execution, the intermediate values encountered during the evaluation of each
// program are collected and stored in the CoverageStats map of the TestRunner.
func (tr *TestRunner) ExecuteTest(t *testing.T, programs []Program, test *Test) error {
	t.Helper()
	if tr.Compiler == nil {
		return fmt.Errorf("compiler is not set")
	}
	for i, pr := range programs {
		if pr.Program == nil {
			return fmt.Errorf("CEL program not set")
		}
		out, details, err := pr.Eval(test.input)
		if enableDebug {
			t.Logf("Eval result: %v (err: %v)", out, err)
		}
		if testResult := test.resultMatcher(out, err); !testResult.Success {
			return fmt.Errorf("test: %s \n wanted: %v \n failed: %v", test.name, testResult.Wanted, testResult.Error)
		}
		if tr.EnableCoverage {
			collectCoverageStats(details, &pr)
			programs[i] = pr
		}
	}
	return nil
}

// collectCoverageStats collects the coverage stats from the EvalDetails and stores them in the
// CoverageStats map of the Program.
func collectCoverageStats(details *cel.EvalDetails, p *Program) {
	if details == nil || details.State() == nil {
		return
	}
	if p.CoverageStats == nil {
		p.CoverageStats = make(map[int64]set)
	}
	state := details.State()
	for _, id := range state.IDs() {
		value, found := state.Value(id)
		if !found {
			continue
		}
		if _, ok := p.CoverageStats[id]; !ok {
			p.CoverageStats[id] = make(set)
		}
		p.CoverageStats[id][value] = struct{}{}
	}
	// Propagate visitedness after collecting direct observations
	propagateVisitedness(p.Ast, p.CoverageStats, state)
}

// propagateVisitedness implements two rules for extending the coverage report:
// - If a node is visited, all its ancestors are visited.
// - If a node is visited, and it has only one child, then the child is visited.
func propagateVisitedness(root *cel.Ast, coverageStats map[int64]set, state interpreter.EvalState) {
	if root == nil || coverageStats == nil {
		return
	}
	// Propagate visitedness upwards for ancestors.
	ast.PostOrderVisit(root.NativeRep().Expr(), parentVisitor{coverageStats: coverageStats, evalState: state})
	// Propagate visitedness downwards for single-child nodes.
	ast.PostOrderVisit(root.NativeRep().Expr(), childrenVisitor{coverageStats: coverageStats, evalState: state})
}

// childrenVisitor is a visitor that populates the coverageStats for the child nodes of a given expression.
type childrenVisitor struct {
	coverageStats map[int64]set
	evalState     interpreter.EvalState
}

// VisitExpr sets the coverageStats for the child of the given expression when the expression has just one child.
func (cv childrenVisitor) VisitExpr(e ast.Expr) {
	if cv.coverageStats[e.ID()] == nil {
		return
	}
	switch e.Kind() {
	case ast.SelectKind:
		selExprOperand := e.AsSelect().Operand()
		if _, ok := cv.coverageStats[selExprOperand.ID()]; !ok {
			cv.coverageStats[selExprOperand.ID()] = make(set)
		}
	case ast.CallKind:
		if c := e.AsCall(); c.FunctionName() == operators.Negate || c.FunctionName() == operators.LogicalNot {
			if _, ok := cv.coverageStats[c.Args()[0].ID()]; !ok {
				cv.coverageStats[c.Args()[0].ID()] = make(set)
			}
		}
	}
}

// VisitEntryExpr does not affect the coverageStats for the child nodes of the given entry expression.
func (cv childrenVisitor) VisitEntryExpr(e ast.EntryExpr) {}

// parentVisitor is a visitor that populates the coverageStats of a given expression based on the
// coverageStats of its direct children.
type parentVisitor struct {
	coverageStats map[int64]set
	evalState     interpreter.EvalState
}

// VisitExpr populates the coverageStats for the current expression based on the coverageStats of its direct children.
func (pv parentVisitor) VisitExpr(e ast.Expr) {
	switch e.Kind() {
	case ast.SelectKind:
		if pv.coverageStats[e.ID()] != nil {
			return
		}
		if _, ok := pv.coverageStats[e.AsSelect().Operand().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		}
	case ast.CallKind:
		c := e.AsCall()
		if c.FunctionName() == "cel.@block" {
			pv.coverageStats[e.ID()] = pv.coverageStats[c.Args()[1].ID()]
			return
		}
		if c.FunctionName() == operators.Conditional {
			truthyID := c.Args()[1].ID()
			if truthyVal, ok := pv.evalState.Value(truthyID); ok {
				if pv.coverageStats[e.ID()] == nil {
					pv.coverageStats[e.ID()] = make(set)
				}
				pv.coverageStats[e.ID()][truthyVal] = struct{}{}
			}
			falsyID := c.Args()[2].ID()
			if falsyVal, ok := pv.evalState.Value(falsyID); ok {
				if pv.coverageStats[e.ID()] == nil {
					pv.coverageStats[e.ID()] = make(set)
				}
				pv.coverageStats[e.ID()][falsyVal] = struct{}{}
			}
			return
		}
		if pv.coverageStats[e.ID()] != nil {
			return
		}
		for _, arg := range c.Args() {
			if _, ok := pv.coverageStats[arg.ID()]; ok {
				pv.coverageStats[e.ID()] = make(set)
				break
			}
		}
		if c.IsMemberFunction() && pv.coverageStats[c.Target().ID()] == nil && pv.coverageStats[e.ID()] != nil {
			pv.coverageStats[c.Target().ID()] = make(set)
		}
	case ast.ListKind:
		if pv.coverageStats[e.ID()] != nil {
			return
		}
		l := e.AsList()
		for _, element := range l.Elements() {
			if _, ok := pv.coverageStats[element.ID()]; ok {
				pv.coverageStats[e.ID()] = make(set)
				break
			}
		}
	case ast.MapKind:
		if pv.coverageStats[e.ID()] != nil {
			return
		}
		m := e.AsMap()
		for _, entry := range m.Entries() {
			if _, ok := pv.coverageStats[entry.ID()]; ok {
				pv.coverageStats[e.ID()] = make(set)
				break
			}
		}
	case ast.StructKind:
		if pv.coverageStats[e.ID()] != nil {
			return
		}
		s := e.AsStruct()
		for _, field := range s.Fields() {
			if _, ok := pv.coverageStats[field.ID()]; ok {
				pv.coverageStats[e.ID()] = make(set)
				break
			}
		}
	case ast.ComprehensionKind:
		if pv.coverageStats[e.ID()] != nil {
			return
		}
		comp := e.AsComprehension()
		if _, ok := pv.coverageStats[comp.IterRange().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		} else if _, ok = pv.coverageStats[comp.AccuInit().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		} else if _, ok = pv.coverageStats[comp.LoopCondition().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		} else if _, ok = pv.coverageStats[comp.LoopStep().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		} else if _, ok = pv.coverageStats[comp.Result().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		}
	}
}

// VisitEntryExpr populates the coverageStats for the current entry expression based on the coverageStats
// of its direct children.
func (pv parentVisitor) VisitEntryExpr(e ast.EntryExpr) {
	if pv.coverageStats[e.ID()] != nil {
		return
	}
	switch e.Kind() {
	case ast.MapEntryKind:
		me := e.AsMapEntry()
		if _, ok := pv.coverageStats[me.Key().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		}
		if _, ok := pv.coverageStats[me.Value().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		}
	case ast.StructFieldKind:
		sf := e.AsStructField()
		if _, ok := pv.coverageStats[sf.Value().ID()]; ok {
			pv.coverageStats[e.ID()] = make(set)
		}
	}
}
