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
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/checker"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/interpreter"
	"cel.dev/cel-go/test"
)

var bindingTests = []struct {
	name          string
	expr          string
	vars          []cel.EnvOption
	in            map[string]any
	hints         map[string]uint64
	estimatedCost checker.CostEstimate
	actualCost    uint64
}{
	{
		name: "single bind",
		expr: `cel.bind(a, 'hell' + 'o' + '!', "%s, %s, %s".format([a, a, a])) ==
	                       'hello!, hello!, hello' + '!'`,
		estimatedCost: checker.CostEstimate{Min: 30, Max: 32},
		actualCost:    32,
	},
	{
		name: "multiple binds",
		expr: `cel.bind(a, 'hello!',
		       cel.bind(b, 'goodbye',
				a + ' and, ' + b)) == 'hello! and, goodbye'`,
		estimatedCost: checker.CostEstimate{Min: 27, Max: 28},
		actualCost:    28,
	},
	{
		name: "shadow binds",
		expr: `cel.bind(a,
		       cel.bind(a, 'world', a + '!'),
		   	    'hello ' + a) == 'hello ' + 'world' + '!'`,
		estimatedCost: checker.CostEstimate{Min: 30, Max: 31},
		actualCost:    31,
	},
	{
		name: "nested bind with int list",
		expr: `cel.bind(a, x,
			   cel.bind(b, a[0],
			   cel.bind(c, a[1], b + c))) == 10`,
		vars: []cel.EnvOption{cel.Variable("x", cel.ListType(cel.IntType))},
		in: map[string]any{
			"x": []int64{3, 7},
		},
		hints: map[string]uint64{
			"x": 3,
		},
		estimatedCost: checker.CostEstimate{Min: 39, Max: 39},
		actualCost:    39,
	},
	{
		name: "nested bind with string list",
		expr: `cel.bind(a, x,
			   cel.bind(b, a[0],
			   cel.bind(c, a[1], b + c))) == "threeseven"`,
		vars: []cel.EnvOption{cel.Variable("x", cel.ListType(cel.StringType))},
		in: map[string]any{
			"x": []string{"three", "seven"},
		},
		hints: map[string]uint64{
			"x":        3,
			"x.@items": 10,
		},
		estimatedCost: checker.CostEstimate{Min: 38, Max: 40},
		actualCost:    39,
	},
	{
		name: "shadowed binding",
		expr: `cel.bind(x, 0, x == 0)`,
		vars: []cel.EnvOption{cel.Variable("x", cel.StringType)},
		in: map[string]any{
			"cel.example.x": "1",
		},
		estimatedCost: checker.FixedCostEstimate(12),
		actualCost:    12,
	},
	{
		name: "container shadowed binding",
		expr: `cel.bind(x, 0, x == 0)`,
		vars: []cel.EnvOption{
			cel.Container("cel.example"),
			cel.Variable("cel.example.x", cel.StringType),
		},
		in: map[string]any{
			"cel.example.x": "1",
		},
		estimatedCost: checker.FixedCostEstimate(12),
		actualCost:    12,
	},
	{
		name: "shadowing namespace resolution selector",
		expr: `cel.bind(x, {'y': 0}, x.y == 0)`,
		vars: []cel.EnvOption{
			cel.Container("cel.example"),
			cel.Variable("cel.example.x.y", cel.IntType),
		},
		in: map[string]any{
			"cel.example.x.y": 1,
		},
		estimatedCost: checker.FixedCostEstimate(43),
		actualCost:    43,
	},
	{
		name: "shadowing namespace resolution selector with local",
		expr: `cel.bind(x, {'y': 0}, .x.y == x.y)`,
		vars: []cel.EnvOption{
			cel.Variable("x.y", cel.IntType),
		},
		in: map[string]any{
			"x.y": 0,
		},
		estimatedCost: checker.FixedCostEstimate(44),
		actualCost:    44,
	},
	{
		name: "namespace disambiguation",
		expr: `cel.bind(y, 0, .y != y)`,
		vars: []cel.EnvOption{
			cel.Variable("y", cel.IntType),
		},
		in: map[string]any{
			"y": 1,
		},
		estimatedCost: checker.FixedCostEstimate(13),
		actualCost:    13,
	},
	{
		name:          "nesting shadowing",
		expr:          `cel.bind(y, 0, cel.bind(y, 1, y != 0))`,
		estimatedCost: checker.FixedCostEstimate(22),
		actualCost:    22,
	},
}

func TestBindings(t *testing.T) {
	for _, tst := range bindingTests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			var asts []*cel.Ast
			opts := append([]cel.EnvOption{Bindings(BindingsVersion(0)), Strings()}, tc.vars...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
			}
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%v) failed: %v", tc.expr, iss.Err())
			}
			asts = append(asts, pAst)
			cAst, iss := env.Check(pAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%v) failed: %v", tc.expr, iss.Err())
			}
			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost)
			}
		})
	}
}

// nestedBindExpr produces an exponentially amplified value through nested cel.bind calls:
//
//	cel.bind(a0, [0,1,2,3,4,5,6,7,8,9],
//	  cel.bind(a1, [a0,a0,a0,a0,a0,a0,a0,a0,a0,a0],
//	    ... cel.bind(aN, [aN-1 x10], aN == aN)))
//
// Each level is physically a 10-element list of references to the level below, so the
// logical element count grows by 10x per level while the physical allocation stays small.
func nestedBindExpr(levels int) string {
	expr := fmt.Sprintf("a%d == a%d", levels, levels)
	for i := levels; i >= 1; i-- {
		refs := strings.TrimSuffix(strings.Repeat(fmt.Sprintf("a%d,", i-1), 10), ",")
		expr = fmt.Sprintf("cel.bind(a%d, [%s], %s)", i, refs, expr)
	}
	return fmt.Sprintf("cel.bind(a0, [0,1,2,3,4,5,6,7,8,9], %s)", expr)
}

func TestBindingsMemoryPeakAmplification(t *testing.T) {
	// The tracked peak reflects the full logical element count of the largest bound value:
	// size(a0) = 11, and size(aN) = 1 + 10*size(aN-1). Each bound level is a list of
	// references to a single shared instance whose aggregate size is memoized on first
	// computation, so sizing each level costs ~11 traversals while remaining exact — the
	// calculator's traversal and depth budgets never bind on the shared structure.
	tests := []struct {
		levels   int
		wantPeak uint32
	}{
		{levels: 2, wantPeak: 1111},
		{levels: 4, wantPeak: 111111},
	}
	env, err := cel.NewEnv(Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("levels_%d", tc.levels), func(t *testing.T) {
			expr := nestedBindExpr(tc.levels)
			ast, iss := env.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", expr, iss.Err())
			}
			prg, err := env.Program(ast, cel.MemoryTracking())
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, details, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out != types.True {
				t.Errorf("prg.Eval() got %v, wanted true", out)
			}
			peak := details.PeakMemory()
			if peak == nil {
				t.Fatal("EvalDetails.PeakMemory() got nil, wanted non-nil peak")
			}
			if *peak != tc.wantPeak {
				t.Errorf("EvalDetails.PeakMemory() got %d, wanted %d", *peak, tc.wantPeak)
			}
			if details.MemoryTracker().CalculationLimitExceeded() {
				t.Error("MemoryTracker.CalculationLimitExceeded() got true, wanted false for exact memoized sizing")
			}
		})
	}
}

func TestBindingsMemoryLimitAmplification(t *testing.T) {
	// With eight amplification levels the logical size is ~10^9 elements while the physical
	// structure is ~90 small lists sharing backing storage. The memory limit must trip while
	// the bound values are being constructed, long before the a8 == a8 comparison would
	// attempt to traverse the logical structure.
	//
	// Because each level's aggregate size is memoized on the shared list instances, sizing
	// stays exact (size(aN) = 1 + 10*size(aN-1)) at ~11 traversals per level, and the
	// calculator's traversal and depth budgets never bind under either configuration below.
	// The limit trips at the a5 binding, whose exact logical size of 1,111,111 elements
	// exceeds the 1M limit. Since sizing walks shared references without materializing the
	// logical value, the Go-native allocations stay flat (~20KB) regardless of the
	// calculator budgets — the flat allocation bound below asserts exactly that.
	env, err := cel.NewEnv(Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	expr := nestedBindExpr(8)
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}

	tests := []struct {
		name string
		opts []cel.ProgramOption
	}{
		{
			name: "default_calculator_limits",
			opts: []cel.ProgramOption{cel.MemoryLimit(1_000_000)},
		},
		{
			name: "custom_calculator_limits_100k_traversals_10_deep",
			opts: []cel.ProgramOption{
				cel.MemoryTracking(
					types.MemoryTrackerSizeCalculator(types.NewSizeCalculator(
						types.SizeCalculatorMaxTraversal(100_000),
						types.SizeCalculatorMaxDepth(10)))),
				cel.MemoryLimit(1_000_000),
			},
		},
	}

	// Subtests share process-global MemStats and must not run in parallel.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := env.Program(ast, tc.opts...)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			_, _, err = prg.Eval(cel.NoVars())
			runtime.ReadMemStats(&after)

			if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
				t.Fatalf("prg.Eval() got error %v, wanted error containing 'memory limit exceeded'", err)
			}
			allocated := after.TotalAlloc - before.TotalAlloc
			t.Logf("evaluation allocated %d bytes", allocated)
			// Measured allocation is ~20KB in either configuration; 1MiB provides ample
			// headroom for incidental runtime allocations while still proving the sizing
			// pass allocates nothing proportional to the traversal budget or the logical
			// value size.
			const maxAllocBytes = 1 << 20 // 1MiB
			if allocated > maxAllocBytes {
				t.Errorf("evaluation allocated %d bytes, wanted less than %d", allocated, maxAllocBytes)
			}
		})
	}
}

// doublingStringBindExpr doubles the input string s through nested cel.bind calls:
//
//	cel.bind(a0, s, cel.bind(a1, a0 + a0, ... cel.bind(aN, aN-1 + aN-1, aN == aN)))
//
// Unlike list concatenation, string concatenation materializes a new backing string at each
// level, so the physical allocation doubles alongside the logical size.
func doublingStringBindExpr(levels int) string {
	expr := fmt.Sprintf("a%d == a%d", levels, levels)
	for i := levels; i >= 1; i-- {
		expr = fmt.Sprintf("cel.bind(a%d, a%d + a%d, %s)", i, i-1, i-1, expr)
	}
	return fmt.Sprintf("cel.bind(a0, s, %s)", expr)
}

func TestBindingsMemoryLimitStringConcat(t *testing.T) {
	// A 1MiB input string doubled per bind level. The doublings rapidly exceed the 1M unit
	// memory limit, so evaluation must terminate while the bound values are still being
	// constructed; unchecked, the doublings would allocate ~510MiB cumulative through a8.
	env, err := cel.NewEnv(Bindings(), cel.Variable("s", cel.StringType))
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	in := map[string]any{"s": strings.Repeat("a", 1<<20)}
	compile := func(levels int) *cel.Ast {
		ast, iss := env.Compile(doublingStringBindExpr(levels))
		if iss.Err() != nil {
			t.Fatalf("env.Compile() failed: %v", iss.Err())
		}
		return ast
	}
	measure := func(t *testing.T, prg cel.Program) (time.Duration, uint64, error) {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		start := time.Now()
		_, _, err := prg.Eval(in)
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		return elapsed, after.TotalAlloc - before.TotalAlloc, err
	}

	// Subtests share process-global MemStats and must not run in parallel.
	t.Run("limit_trips_mid_amplification", func(t *testing.T) {
		prg, err := env.Program(compile(8), cel.MemoryLimit(1_000_000))
		if err != nil {
			t.Fatalf("env.Program() failed: %v", err)
		}
		elapsed, allocated, err := measure(t, prg)
		if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
			t.Fatalf("prg.Eval() got error %v, wanted error containing 'memory limit exceeded'", err)
		}
		t.Logf("limit tripped in %v after allocating %d bytes", elapsed, allocated)
		// The enforcement point follows the materialization of the offending value, so a
		// few doublings of intermediates is the expected floor; the bound proves the
		// remaining exponential trajectory (~510MiB unchecked) was halted.
		const maxAllocBytes = 64 << 20 // 64MiB
		if allocated > maxAllocBytes {
			t.Errorf("evaluation allocated %d bytes, wanted less than %d", allocated, maxAllocBytes)
		}
		if elapsed > 5*time.Second {
			t.Errorf("evaluation took %v, wanted under 5s", elapsed)
		}
	})

	t.Run("unchecked_go_layer_baseline", func(t *testing.T) {
		// The same workload the tripped case performed (doubling through 16MiB) without
		// memory tracking: quantifies the Go-layer time and allocation the limit raced
		// against, and the tracking overhead paid by the tripped case.
		prg, err := env.Program(compile(4))
		if err != nil {
			t.Fatalf("env.Program() failed: %v", err)
		}
		elapsed, allocated, err := measure(t, prg)
		if err != nil {
			t.Fatalf("prg.Eval() failed: %v", err)
		}
		t.Logf("unchecked baseline completed in %v allocating %d bytes", elapsed, allocated)
		const maxAllocBytes = 64 << 20 // 64MiB
		if allocated > maxAllocBytes {
			t.Errorf("evaluation allocated %d bytes, wanted less than %d", allocated, maxAllocBytes)
		}
	})
}

func BenchmarkBindingsMemoryAmplification(b *testing.B) {
	env, err := cel.NewEnv(Bindings())
	if err != nil {
		b.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	compile := func(levels int) *cel.Ast {
		ast, iss := env.Compile(nestedBindExpr(levels))
		if iss.Err() != nil {
			b.Fatalf("env.Compile() failed: %v", iss.Err())
		}
		return ast
	}
	ast2 := compile(2)
	ast8 := compile(8)
	customCalc := cel.MemoryTracking(
		types.MemoryTrackerSizeCalculator(types.NewSizeCalculator(
			types.SizeCalculatorMaxTraversal(100_000),
			types.SizeCalculatorMaxDepth(10))))

	benchmarks := []struct {
		name    string
		ast     *cel.Ast
		opts    []cel.ProgramOption
		wantErr bool
	}{
		// Successful evaluation of the 2-level expression, without and with tracking, to
		// isolate the per-eval overhead of watermark observation and sizing.
		{name: "levels_2_no_tracking", ast: ast2},
		{name: "levels_2_tracking", ast: ast2, opts: []cel.ProgramOption{cel.MemoryTracking()}},
		// Enforcement trip on the 8-level expression: sizing runs until the calculator's
		// traversal budget aborts, the saturated watermark trips the limit, and evaluation
		// unwinds through the cancellation panic.
		{
			name:    "levels_8_limit_default_calculator",
			ast:     ast8,
			opts:    []cel.ProgramOption{cel.MemoryLimit(1_000_000)},
			wantErr: true,
		},
		{
			name:    "levels_8_limit_custom_calculator_100k_10",
			ast:     ast8,
			opts:    []cel.ProgramOption{customCalc, cel.MemoryLimit(1_000_000)},
			wantErr: true,
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			prg, err := env.Program(bm.ast, bm.opts...)
			if err != nil {
				b.Fatalf("env.Program() failed: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := prg.Eval(cel.NoVars())
				if bm.wantErr != (err != nil) {
					b.Fatalf("prg.Eval() got err=%v, wantErr=%v", err, bm.wantErr)
				}
			}
		})
	}
}

func TestBindingsNonMatch(t *testing.T) {
	env, err := cel.NewEnv(Bindings(), Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
	}
	nonMatchExpr := `ceel.bind(a, 1, a)`
	ast, iss := env.Parse(nonMatchExpr)
	if iss.Err() != nil {
		t.Fatalf("env.Parse(%v) failed: %v", nonMatchExpr, iss.Err())
	}
	if len(ast.SourceInfo().GetMacroCalls()) != 0 {
		t.Fatalf("env.Parse(%v) performed a macro replacement when none was expected: %v",
			nonMatchExpr, ast.SourceInfo().GetMacroCalls())
	}
}

func TestBindingsInvalidIdent(t *testing.T) {
	env, err := cel.NewEnv(Bindings(), Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings(), Strings()) failed: %v", err)
	}
	invalidIdentExpr := `cel.bind(a.b, 1, a.b)`
	wantErr := "ERROR: <input>:1:11: cel.bind() variable names must be simple identifiers"
	_, iss := env.Parse(invalidIdentExpr)
	if !strings.Contains(iss.Err().Error(), wantErr) {
		t.Fatalf("env.Parse(%v) failed: %v", invalidIdentExpr, iss.Err())
	}
}

func BenchmarkBindings(b *testing.B) {
	for i, tst := range bindingTests {
		tc := tst
		opts := append([]cel.EnvOption{Bindings(), Strings()}, tc.vars...)
		env, err := cel.NewEnv(opts...)
		if err != nil {
			b.Fatalf("cel.NewEnv() failed: %v", err)
		}
		ast, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			b.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
		if err != nil {
			b.Fatalf("env.Program(ast, Optimize) failed: %v", err)
		}
		// Benchmark the eval.
		b.Run(fmt.Sprintf("[%d]", i), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			var input any = cel.NoVars()
			if tc.in != nil {
				input = tc.in
			}
			for i := 0; i < b.N; i++ {
				prg.Eval(input)
			}
		})
	}
}

func TestBlockEval(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		expr ast.Expr
		opts []cel.EnvOption
		in   map[string]any
		out  ref.Val
	}{
		{
			name: "chained block",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "x"),
					fac.NewIdent(4, "@index0"),
					fac.NewIdent(5, "@index1"),
				}, []int32{}),
				fac.NewCall(9, operators.Add,
					fac.NewCall(6, operators.Add,
						fac.NewIdent(7, "@index2"),
						fac.NewIdent(8, "@index1")),
					fac.NewIdent(10, "@index0"),
				),
			),
			opts: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in:  map[string]any{"x": "hello"},
			out: types.String("hellohellohello"),
		},
		{
			name: "empty block",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{}, []int32{}),
				fac.NewCall(3, operators.LogicalNot, fac.NewLiteral(4, types.False)),
			),
			in:  map[string]any{},
			out: types.True,
		},
		{
			name: "mixed block constant values",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewLiteral(3, types.String("hello")),
					fac.NewLiteral(4, types.Int(5)),
				}, []int32{}),
				fac.NewCall(5, operators.Equals,
					fac.NewCall(6, "size",
						fac.NewIdent(7, "@index0")),
					fac.NewIdent(8, "@index1"),
				),
			),
			opts: []cel.EnvOption{
				cel.ExtendedValidations(),
			},
			in:  map[string]any{},
			out: types.True,
		},
		{
			name: "mixed block dynamic values",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "x"),
					fac.NewLiteral(4, types.Int(5)),
				}, []int32{}),
				fac.NewCall(5, operators.Equals,
					fac.NewCall(6, "size",
						fac.NewIdent(7, "@index0")),
					fac.NewIdent(8, "@index1"),
				),
			),
			opts: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
				cel.ExtendedValidations(),
			},
			in:  map[string]any{"x": "goodbye"},
			out: types.False,
		},
		{
			name: "mixed block constant values dyn var",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewLiteral(3, types.String("hello")),
				}, []int32{}),
				fac.NewCall(4, operators.Equals,
					fac.NewCall(5, "size",
						fac.NewIdent(6, "@index0")),
					fac.NewIdent(7, "y"),
				),
			),
			opts: []cel.EnvOption{
				cel.Variable("y", cel.IntType),
				cel.ExtendedValidations(),
			},
			in: map[string]any{
				"y": 5,
			},
			out: types.True,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			blockAST := ast.NewAST(tc.expr, nil)
			opts := append([]cel.EnvOption{Bindings()}, tc.opts...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
			}
			prg, err := env.PlanProgram(blockAST, cel.EvalOptions(cel.OptOptimize))
			if err != nil {
				t.Fatalf("PlanProgram() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.in)
			if err != nil {
				t.Fatalf("prg.Eval() failed: %v", err)
			}
			if out.Equal(tc.out) != types.True {
				t.Errorf("got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestBlockEval_BadPlan(t *testing.T) {
	fac := ast.NewExprFactory()
	blockExpr := fac.NewCall(
		1, "cel.@block",
		fac.NewList(2, []ast.Expr{
			fac.NewIdent(3, "x"),
			fac.NewIdent(4, "@index0"),
		}, []int32{}),
		fac.NewCall(6, operators.Add,
			fac.NewIdent(7, "@index1"),
			fac.NewIdent(8, "@index0")),
		fac.NewIdent(9, "x"),
	)
	blockAST := ast.NewAST(blockExpr, nil)
	env, err := cel.NewEnv(
		Bindings(BindingsVersion(1)),
		cel.Variable("x", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	_, err = env.PlanProgram(blockAST)
	if err == nil {
		t.Fatal("PlanProgram() succeeded, expected error")
	}
}

func TestBlockEval_BadBlock(t *testing.T) {
	fac := ast.NewExprFactory()
	blockExpr := fac.NewCall(
		1, "cel.@block",
		fac.NewCall(2, operators.Add,
			fac.NewIdent(3, "@index1"),
			fac.NewIdent(4, "@index0")),
		fac.NewIdent(5, "x"),
	)
	blockAST := ast.NewAST(blockExpr, nil)
	env, err := cel.NewEnv(
		Bindings(BindingsVersion(1)),
		cel.Variable("x", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	_, err = env.PlanProgram(blockAST)
	if err == nil {
		t.Fatal("PlanProgram() succeeded, expected error")
	}
}

func TestBlockEval_RuntimeErrors(t *testing.T) {
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		expr ast.Expr
	}{
		{
			name: "bad index",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "x"),
					fac.NewIdent(4, "@indexNext"),
				}, []int32{}),
				fac.NewCall(6, operators.Add,
					fac.NewIdent(7, "@indexNext"),
					fac.NewIdent(8, "@index0")),
			),
		},
		{
			name: "infinite recursion",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "@index0"),
					fac.NewIdent(4, "@index0"),
				}, []int32{}),
				fac.NewIdent(10, "@index0"),
			),
		},
		{
			name: "negative index",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "@index-1"),
					fac.NewIdent(4, "@index0"),
				}, []int32{}),
				fac.NewIdent(10, "@index0"),
			),
		},
		{
			name: "out of range index",
			expr: fac.NewCall(
				1, "cel.@block",
				fac.NewList(2, []ast.Expr{
					fac.NewIdent(3, "@index100"),
					fac.NewIdent(4, "@index0"),
				}, []int32{}),
				fac.NewIdent(10, "@index0"),
			),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			blockAST := ast.NewAST(tc.expr, nil)
			env, err := cel.NewEnv(
				Bindings(BindingsVersion(1)),
				cel.Variable("x", cel.StringType),
			)
			if err != nil {
				t.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
			}
			prg, err := env.PlanProgram(blockAST)
			if err != nil {
				t.Fatalf("PlanProgram() failed: %v", err)
			}
			_, _, err = prg.Eval(map[string]any{"x": "hello"})
			if !strings.Contains(err.Error(), "no such attribute") {
				t.Fatalf("prg.Eval() got %v, expected no such attribute error", err)
			}
		})
	}
}

func TestDynamicBlockEval(t *testing.T) {
	db := &dynamicBlock{
		expr: interpreter.NewConstValue(1, types.IntOne),
		slotActivationPool: &sync.Pool{
			New: func() any {
				return &dynamicSlotActivation{}
			},
		},
	}
	res := db.Eval(cel.NoVars())
	if res.Equal(types.IntOne) != types.True {
		t.Errorf("db.Eval() = %v, wanted 1", res)
	}
}

func TestConstantBlockEval(t *testing.T) {
	cb := &constantBlock{
		expr: interpreter.NewConstValue(2, types.IntOne),
	}
	res := cb.Eval(cel.NoVars())
	if res.Equal(types.IntOne) != types.True {
		t.Errorf("cb.Eval() = %v, wanted 1", res)
	}
}

func BenchmarkBlockEval(b *testing.B) {
	fac := ast.NewExprFactory()
	expr := fac.NewCall(
		1, "cel.@block",
		fac.NewList(2, []ast.Expr{
			fac.NewIdent(3, "x"),
			fac.NewIdent(4, "@index0"),
			fac.NewIdent(5, "@index1"),
		}, []int32{}),
		fac.NewCall(9, operators.Add,
			fac.NewCall(6, operators.Add,
				fac.NewIdent(7, "@index2"),
				fac.NewIdent(8, "@index1")),
			fac.NewIdent(10, "@index0"),
		),
	)
	blockAST := ast.NewAST(expr, nil)
	env, err := cel.NewEnv(
		Bindings(),
		cel.Variable("x", cel.StringType),
	)
	if err != nil {
		b.Fatalf("cel.NewEnv(Bindings()) failed: %v", err)
	}
	prg, err := env.PlanProgram(blockAST, cel.EvalOptions(cel.OptOptimize))
	if err != nil {
		b.Fatalf("PlanProgram() failed: %v", err)
	}
	input := map[string]any{"x": "hello"}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prg.Eval(input)
	}
}

func TestValidateBindNestingLimit(t *testing.T) {
	env, err := cel.NewEnv(
		Bindings(),
		cel.ASTValidators(cel.ValidateBindNestingLimit(2)),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		expr string
		iss  string
	}{
		{
			expr: `cel.bind(a, 1, a + 1)`,
		},
		{
			expr: `cel.bind(a, 1, cel.bind(b, 2, a + b))`,
		},
		{
			// two cel.binds, but in separate branches
			expr: `cel.bind(a, 1, a) + cel.bind(b, 2, b)`,
		},
		{
			// empty iteration range comprehension (e.g. cel.bind) does not count against comprehension limit,
			// but counts against cel.bind nesting limit.
			expr: `[1, 2, 3].exists(i, cel.bind(a, i, cel.bind(b, a, a + b) > 0))`,
		},
		{
			// three cel.binds, three levels deep
			expr: `cel.bind(a, 1, cel.bind(b, 2, cel.bind(c, 3, a + b + c)))`,
			iss: `
			ERROR: <input>:1:39: cel.bind exceeds nesting limit
             | cel.bind(a, 1, cel.bind(b, 2, cel.bind(c, 3, a + b + c)))
             | ......................................^`,
		},
		{
			// three cel.binds, three levels deep with non-comprehension ancestor (+)
			expr: `cel.bind(a, 1, cel.bind(b, 2, 1 + cel.bind(c, 3, a + b + c)))`,
			iss: `
			ERROR: <input>:1:43: cel.bind exceeds nesting limit
             | cel.bind(a, 1, cel.bind(b, 2, 1 + cel.bind(c, 3, a + b + c)))
             | ..........................................^`,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			_, iss := env.Compile(tc.expr)
			if tc.iss != "" {
				if iss.Err() == nil {
					t.Fatalf("env.Compile(%v) returned ast, expected error: %v", tc.expr, tc.iss)
				}
				if !test.Compare(iss.Err().Error(), tc.iss) {
					t.Fatalf("env.Compile(%v) returned %v, expected error: %v", tc.expr, iss.Err(), tc.iss)
				}
				return
			}
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
		})
	}
}
