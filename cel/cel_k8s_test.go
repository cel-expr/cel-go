package cel

import (
	"runtime"
	"testing"

	"github.com/google/cel-go/common/types"
)

// wrappingProvider mimics the Kubernetes CRD validation pattern where each
// schema node extends a shared base environment with a custom type provider
// that wraps the parent provider, plus scoped variable declarations. No
// functions, macros, or libraries are added by the extension.
type wrappingProvider struct {
	types.Provider
}

func k8sShapeBaseEnv(tb testing.TB) *Env {
	tb.Helper()
	env, err := NewEnv(
		OptionalTypes(),
		CrossTypeNumericComparisons(true),
		EagerlyValidateDeclarations(true),
	)
	if err != nil {
		tb.Fatalf("NewEnv() failed: %v", err)
	}
	// Force the base checker and function bindings to materialize so that
	// per-iteration numbers measure only the marginal extension cost.
	ast, iss := env.Compile("1 == 1")
	if iss.Err() != nil {
		tb.Fatalf("base Compile() failed: %v", iss.Err())
	}
	if _, err := env.Program(ast); err != nil {
		tb.Fatalf("base Program() failed: %v", err)
	}
	return env
}

func extendK8sShape(tb testing.TB, base *Env) *Env {
	tb.Helper()
	env, err := base.Extend(
		Variable("self", DynType),
		Variable("oldSelf", DynType),
		CustomTypeProvider(&wrappingProvider{base.CELTypeProvider()}),
	)
	if err != nil {
		tb.Fatalf("Extend() failed: %v", err)
	}
	return env
}

// BenchmarkEnvExtendK8sShape measures the cost of extending a base environment
// with only variables and a wrapping type provider, then compiling one
// expression (which forces checker construction).
func BenchmarkEnvExtendK8sShape(b *testing.B) {
	base := k8sShapeBaseEnv(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := extendK8sShape(b, base)
		_, iss := env.Compile("self == oldSelf")
		if iss.Err() != nil {
			b.Fatalf("Compile() failed: %v", iss.Err())
		}
	}
}

// BenchmarkProgramK8sShape measures per-program construction cost on a single
// extended environment (dispatcher + binding materialization).
func BenchmarkProgramK8sShape(b *testing.B) {
	base := k8sShapeBaseEnv(b)
	env := extendK8sShape(b, base)
	ast, iss := env.Compile("self == oldSelf")
	if iss.Err() != nil {
		b.Fatalf("Compile() failed: %v", iss.Err())
	}
	// Force first-program materialization outside the loop for the marginal cost.
	if _, err := env.Program(ast); err != nil {
		b.Fatalf("Program() failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := env.Program(ast); err != nil {
			b.Fatalf("Program() failed: %v", err)
		}
	}
}

// BenchmarkProgramFirstPerEnvK8sShape measures the first-program cost per
// extended env, which includes binding materialization on that env.
func BenchmarkProgramFirstPerEnvK8sShape(b *testing.B) {
	base := k8sShapeBaseEnv(b)
	envs := make([]*Env, b.N)
	asts := make([]*Ast, b.N)
	for i := 0; i < b.N; i++ {
		envs[i] = extendK8sShape(b, base)
		ast, iss := envs[i].Compile("self == oldSelf")
		if iss.Err() != nil {
			b.Fatalf("Compile() failed: %v", iss.Err())
		}
		asts[i] = ast
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := envs[i].Program(asts[i]); err != nil {
			b.Fatalf("Program() failed: %v", err)
		}
	}
}

// TestExtendRetainedHeapReport reports the live-heap bytes retained per
// extended environment (each holding one compiled program), and per extra
// program on a single environment. Informational: values are logged, not
// asserted, and mirror the shape used by Kubernetes CRD validation where
// programs are retained for the lifetime of the CRD.
func TestExtendRetainedHeapReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heap report in short mode")
	}
	base := k8sShapeBaseEnv(t)

	liveHeap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapAlloc
	}

	const numEnvs = 500
	sinkPrograms := make([]Program, 0, numEnvs)
	before := liveHeap()
	for i := 0; i < numEnvs; i++ {
		env := extendK8sShape(t, base)
		ast, iss := env.Compile("self == oldSelf")
		if iss.Err() != nil {
			t.Fatalf("Compile() failed: %v", iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program() failed: %v", err)
		}
		sinkPrograms = append(sinkPrograms, prg)
	}
	after := liveHeap()
	t.Logf("retained per extended env (1 program each): %d bytes", (after-before)/numEnvs)

	const numProgs = 2000
	env := extendK8sShape(t, base)
	ast, iss := env.Compile("self == oldSelf")
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	moreProgs := make([]Program, 0, numProgs)
	before = liveHeap()
	for range numProgs {
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program() failed: %v", err)
		}
		moreProgs = append(moreProgs, prg)
	}
	after = liveHeap()
	t.Logf("retained per extra program (single env): %d bytes", (after-before)/numProgs)
	runtime.KeepAlive(sinkPrograms)
	runtime.KeepAlive(moreProgs)
}
