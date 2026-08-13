package conformance_test

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/google/go-cmp/cmp"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	valuepb "cel.dev/expr"
	test2pb "cel.dev/expr/conformance/proto2"
	test3pb "cel.dev/expr/conformance/proto3"
	testpb "cel.dev/expr/conformance/test"
)

type testsFlag []string

func (t *testsFlag) String() string {
	return strings.Join(([]string)(*t), ",")
}

func (t *testsFlag) Set(v string) error {
	*t = strings.Split(v, ",")
	for i, v := range *t {
		(*t)[i] = strings.TrimSpace(v)
	}
	return nil
}

func (t *testsFlag) Get() any {
	return ([]string)(*t)
}

type skipTestsFlag []string

func (t *skipTestsFlag) String() string {
	return strings.Join(([]string)(*t), ",")
}

func (t *skipTestsFlag) Set(v string) error {
	*t = strings.Split(v, ",")
	for i, v := range *t {
		(*t)[i] = strings.TrimSpace(v)
	}
	return nil
}

func (t *skipTestsFlag) Get() any {
	return ([]string)(*t)
}

var (
	dashboard bool
	tests     testsFlag
	skipTests skipTestsFlag

	envWithMacros *cel.Env
	envNoMacros   *cel.Env
)

func init() {
	flag.BoolVar(&dashboard, "dashboard", false, "Dashboard.")
	flag.Var(&tests, "tests", "Paths to run, separate by a comma.")
	flag.Var(&skipTests, "skip_tests", "Tests to skip, separate by a comma.")

	stdOpts := []cel.EnvOption{
		cel.StdLib(),
		cel.ClearMacros(),
		cel.OptionalTypes(),
		cel.EagerlyValidateDeclarations(true),
		cel.EnableErrorOnBadPresenceTest(true),
		cel.Types(&test2pb.TestAllTypes{}, &test2pb.Proto2ExtensionScopedMessage{}, &test3pb.TestAllTypes{}),
		ext.Bindings(),
		ext.Encoders(),
		ext.Lists(),
		ext.Math(),
		ext.Protos(),
		ext.Strings(),
		ext.TwoVarComprehensions(),
		cel.Lib(celBlockLib{}),
		cel.EnableIdentifierEscapeSyntax(),
	}

	var err error
	envNoMacros, err = cel.NewCustomEnv(stdOpts...)
	if err != nil {
		log.Fatalf("cel.NewCustomEnv() = %v", err)
	}
	envWithMacros, err = envNoMacros.Extend(cel.Macros(cel.StandardMacros...))
	if err != nil {
		log.Fatalf("cel.NewCustomEnv() = %v", err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	code := m.Run()
	if dashboard {
		code = 0
	}
	os.Exit(code)
}

func shouldSkipTest(s string) bool {
	for _, t := range skipTests {
		if strings.HasPrefix(s, t) {
			n := s[len(t):]
			if n == "" || strings.HasPrefix(n, "/") {
				return true
			}
		}
	}
	return false
}

func refValueToExprValue(res ref.Val) (*valuepb.ExprValue, error) {
	if types.IsUnknown(res) {
		return &valuepb.ExprValue{
			Kind: &valuepb.ExprValue_Unknown{
				Unknown: &valuepb.UnknownSet{
					Exprs: res.Value().([]int64),
				},
			}}, nil
	}
	v, err := cel.ValueAsProto(res)
	if err != nil {
		return nil, err
	}
	return &valuepb.ExprValue{
		Kind: &valuepb.ExprValue_Value{Value: v}}, nil
}

func exprValueToRefValue(adapter types.Adapter, ev *valuepb.ExprValue) (ref.Val, error) {
	switch ev.Kind.(type) {
	case *valuepb.ExprValue_Value:
		return cel.ProtoAsValue(adapter, ev.GetValue())
	case *valuepb.ExprValue_Error:
		// An error ExprValue is a repeated set of statuspb.Status
		// messages, with no convention for the status details.
		// To convert this to a types.Err, we need to convert
		// these Status messages to a single string, and be
		// able to decompose that string on output so we can
		// round-trip arbitrary ExprValue messages.
		// TODO(jimlarson) make a convention for this.
		return types.NewErr("XXX add details later"), nil
	case *valuepb.ExprValue_Unknown:
		var unk *types.Unknown
		for _, id := range ev.GetUnknown().GetExprs() {
			if unk == nil {
				unk = types.NewUnknown(id, nil)
			}
			unk = types.MergeUnknowns(types.NewUnknown(id, nil), unk)
		}
		return unk, nil
	}
	return nil, errors.New("unknown ExprValue kind")
}

func diffType(want proto.Message, t *cel.Type) (string, error) {
	got, err := types.TypeToProto(t)
	if err != nil {
		return "", err
	}
	return cmp.Diff(want, got, protocmp.Transform()), nil

}

func diffValue(want *valuepb.Value, got *valuepb.ExprValue) string {
	return cmp.Diff(
		&valuepb.ExprValue{Kind: &valuepb.ExprValue_Value{Value: want}},
		got,
		protocmp.Transform(),
		protocmp.SortRepeatedFields(&valuepb.MapValue{}, "entries"))
}

func conformanceTest(t *testing.T, name string, pb *testpb.SimpleTest) {
	if shouldSkipTest(name) {
		t.SkipNow()
		return
	}
	var env *cel.Env
	if pb.GetDisableMacros() {
		env = envNoMacros
	} else {
		env = envWithMacros
	}
	src := common.NewStringSource(pb.GetExpr(), pb.GetName())
	ast, iss := env.ParseSource(src)
	if err := iss.Err(); err != nil {
		t.Fatal(err)
	}
	var opts []cel.EnvOption
	if pb.GetContainer() != "" {
		opts = append(opts, cel.Container(pb.GetContainer()))
	}
	for _, d := range pb.GetTypeEnv() {
		opt, err := cel.ProtoAsDeclaration(d)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, opt)
	}
	var err error
	env, err = env.Extend(opts...)
	if err != nil {
		t.Fatal(err)
	}
	if !pb.GetDisableCheck() {
		ast, iss = env.Check(ast)
		if err := iss.Err(); err != nil {
			t.Fatal(err)
		}
	}
	if pb.GetCheckOnly() {
		m, ok := pb.GetResultMatcher().(*testpb.SimpleTest_TypedResult)
		if !ok {
			t.Fatalf("unexpected matcher kind for check only test: %T", pb.GetResultMatcher())
		}
		if diff, err := diffType(m.TypedResult.DeducedType, ast.OutputType()); err != nil || diff != "" {
			t.Errorf("env.Check() output type err: %v (-want +got):\n%s", err, diff)
		}
		return
	}
	program, err := env.Program(ast)
	if err != nil {
		t.Fatal(err)
	}
	act := make(map[string]any, len(pb.GetBindings()))
	for k, v := range pb.GetBindings() {
		act[k], err = exprValueToRefValue(env.CELTypeAdapter(), v)
		if err != nil {
			t.Fatal(err)
		}
	}
	ret, _, err := program.Eval(act)
	switch m := pb.GetResultMatcher().(type) {
	case *testpb.SimpleTest_Value:
		if err != nil {
			t.Fatalf("program.Eval(): got %v, want nil", err)
		}
		val, err := refValueToExprValue(ret)
		if err != nil {
			t.Fatal(err)
		}
		if diff := diffValue(m.Value, val); diff != "" {
			t.Errorf("program.Eval() diff (-want +got):\n%s", diff)
		}
	case *testpb.SimpleTest_TypedResult:
		if err != nil {
			t.Fatalf("program.Eval(): got %v, want nil", err)
		}
		val, err := refValueToExprValue(ret)
		if err != nil {
			t.Fatal(err)
		}
		if diff := diffValue(m.TypedResult.Result, val); diff != "" {
			t.Errorf("program.Eval() diff (-want +got):\n%s", diff)
		}
		if diff, err := diffType(m.TypedResult.DeducedType, ast.OutputType()); err != nil || diff != "" {
			t.Errorf("env.Check() output type err: %v (-want +got):\n%s", err, diff)
		}
	case *testpb.SimpleTest_EvalError:
		if err == nil && types.IsError(ret) {
			err = ret.(*types.Err).Unwrap()
		}
		if err == nil {
			t.Errorf("program.Eval(): got nil, want %v", m.EvalError)
		}
	default:
		t.Errorf("unexpected matcher kind: %T", pb.GetResultMatcher())
	}
}

func TestConformance(t *testing.T) {
	var files []*testpb.SimpleTestFile
	for _, path := range tests {
		path = strings.TrimPrefix(path, "external/")
		f, err := runfiles.RlocationFrom(path, "com_google_cel_spec")
		if err != nil {
			log.Fatalf("failed to find runfile %q: %v", path, err)
		}
		b, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("failed to read file %q: %v", f, err)
		}
		file := &testpb.SimpleTestFile{}
		err = prototext.Unmarshal(b, file)
		if err != nil {
			log.Fatalf("failed to parse file %q: %v", path, err)
		}
		files = append(files, file)
	}
	for _, file := range files {
		for _, section := range file.GetSection() {
			for _, test := range section.GetTest() {
				name := fmt.Sprintf("%s/%s/%s", file.GetName(), section.GetName(), test.GetName())
				if test.GetResultMatcher() == nil {
					test.ResultMatcher = &testpb.SimpleTest_Value{
						Value: &valuepb.Value{
							Kind: &valuepb.Value_BoolValue{
								BoolValue: true,
							},
						},
					}
				}
				t.Run(name, func(t *testing.T) {
					conformanceTest(t, name, test)
				})
			}
		}
	}
}

type celBlockLib struct{}

func (celBlockLib) LibraryName() string {
	return "cel.lib.ext.cel.block.conformance"
}

func (celBlockLib) CompileOptions() []cel.EnvOption {
	// Simulate indexed arguments which would normally have strong types associated
	// with the values as part of a static optimization pass
	maxIndices := 30
	indexOpts := make([]cel.EnvOption, maxIndices)
	for i := 0; i < maxIndices; i++ {
		indexOpts[i] = cel.Variable(fmt.Sprintf("@index%d", i), cel.DynType)
	}
	return append([]cel.EnvOption{
		cel.Macros(
			// cel.block([args], expr)
			cel.ReceiverMacro("block", 2, celBlock),
			// cel.index(int)
			cel.ReceiverMacro("index", 1, celIndex),
			// cel.iterVar(int, int)
			cel.ReceiverMacro("iterVar", 2, celCompreVar("cel.iterVar", "@it")),
			// cel.accuVar(int, int)
			cel.ReceiverMacro("accuVar", 2, celCompreVar("cel.accuVar", "@ac")),
		),
	}, indexOpts...)
}

func (celBlockLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func celBlock(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
	if !isCELNamespace(target) {
		return nil, nil
	}
	bindings := args[0]
	if bindings.Kind() != ast.ListKind {
		return bindings, mef.NewError(bindings.ID(), "cel.block requires the first arg to be a list literal")
	}
	return mef.NewCall("cel.@block", args...), nil
}

func celIndex(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
	if !isCELNamespace(target) {
		return nil, nil
	}
	index := args[0]
	if !isNonNegativeInt(index) {
		return index, mef.NewError(index.ID(), "cel.index requires a single non-negative int constant arg")
	}
	indexVal := index.AsLiteral().(types.Int)
	return mef.NewIdent(fmt.Sprintf("@index%d", indexVal)), nil
}

func celCompreVar(funcName, varPrefix string) cel.MacroFactory {
	return func(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
		if !isCELNamespace(target) {
			return nil, nil
		}
		depth := args[0]
		if !isNonNegativeInt(depth) {
			return depth, mef.NewError(depth.ID(), fmt.Sprintf("%s requires two non-negative int constant args", funcName))
		}
		unique := args[1]
		if !isNonNegativeInt(unique) {
			return unique, mef.NewError(unique.ID(), fmt.Sprintf("%s requires two non-negative int constant args", funcName))
		}
		depthVal := depth.AsLiteral().(types.Int)
		uniqueVal := unique.AsLiteral().(types.Int)
		return mef.NewIdent(fmt.Sprintf("%s:%d:%d", varPrefix, depthVal, uniqueVal)), nil
	}
}

func isCELNamespace(target ast.Expr) bool {
	return target.Kind() == ast.IdentKind && target.AsIdent() == "cel"
}

func isNonNegativeInt(expr ast.Expr) bool {
	if expr.Kind() != ast.LiteralKind {
		return false
	}
	val := expr.AsLiteral()
	return val.Type() == cel.IntType && val.(types.Int) >= 0
}
