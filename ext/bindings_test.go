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
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/test"
	proto3pb "cel.dev/cel-go/test/proto3pb"
)

var bindingTests = []struct {
	name          string
	expr          string
	vars          []cel.EnvOption
	in            map[string]any
	hints         map[string]uint64
	estimatedCost cost.CostEstimate
	// estimatedCostV0 is the estimate under cost.ModelVersion0, set only where the revision moved it.
	estimatedCostV0 *cost.CostEstimate
	actualCost      uint64
	actualCostV0    *uint64
}{
	{
		name: "single bind",
		expr: `cel.bind(a, 'hell' + 'o' + '!', "%s, %s, %s".format([a, a, a])) ==
	                       'hello!, hello!, hello' + '!'`,
		estimatedCost:   cost.RangedCostEstimate(20, 22),
		estimatedCostV0: costV0(30, 32),
		actualCost:      22,
		actualCostV0:    actualV0(32),
	},
	{
		name: "multiple binds",
		expr: `cel.bind(a, 'hello!',
		       cel.bind(b, 'goodbye',
				a + ' and, ' + b)) == 'hello! and, goodbye'`,
		estimatedCost:   cost.FixedCostEstimate(8),
		estimatedCostV0: costV0(27, 28),
		actualCost:      8,
		actualCostV0:    actualV0(28),
	},
	{
		name: "shadow binds",
		expr: `cel.bind(a,
		       cel.bind(a, 'world', a + '!'),
		   	    'hello ' + a) == 'hello ' + 'world' + '!'`,
		estimatedCost:   cost.FixedCostEstimate(11),
		estimatedCostV0: costV0(30, 31),
		actualCost:      11,
		actualCostV0:    actualV0(31),
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
		estimatedCost:   cost.FixedCostEstimate(9),
		estimatedCostV0: costV0(39, 39),
		actualCost:      9,
		actualCostV0:    actualV0(39),
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
		estimatedCost:   cost.RangedCostEstimate(9, 10),
		estimatedCostV0: costV0(38, 40),
		actualCost:      9,
		actualCostV0:    actualV0(39),
	},
	{
		name: "shadowed binding",
		expr: `cel.bind(x, 0, x == 0)`,
		vars: []cel.EnvOption{cel.Variable("x", cel.StringType)},
		in: map[string]any{
			"cel.example.x": "1",
		},
		estimatedCost:   cost.FixedCostEstimate(2),
		estimatedCostV0: costV0(12, 12),
		actualCost:      2,
		actualCostV0:    actualV0(12),
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
		estimatedCost:   cost.FixedCostEstimate(2),
		estimatedCostV0: costV0(12, 12),
		actualCost:      2,
		actualCostV0:    actualV0(12),
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
		estimatedCost:   cost.FixedCostEstimate(33),
		estimatedCostV0: costV0(43, 43),
		actualCost:      33,
		actualCostV0:    actualV0(43),
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
		estimatedCost:   cost.FixedCostEstimate(34),
		estimatedCostV0: costV0(44, 44),
		actualCost:      34,
		actualCostV0:    actualV0(44),
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
		estimatedCost:   cost.FixedCostEstimate(3),
		estimatedCostV0: costV0(13, 13),
		actualCost:      3,
		actualCostV0:    actualV0(13),
	},
	{
		name:            "nesting shadowing",
		expr:            `cel.bind(y, 0, cel.bind(y, 1, y != 0))`,
		estimatedCost:   cost.FixedCostEstimate(2),
		estimatedCostV0: costV0(22, 22),
		actualCost:      2,
		actualCostV0:    actualV0(22),
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
			testCheckCost(t, env, cAst, tc.hints, tc.estimatedCost, tc.estimatedCostV0)
			asts = append(asts, cAst)
			for _, ast := range asts {
				testEvalWithCost(t, env, ast, tc.in, tc.actualCost, ledgerV0(tc.actualCostV0)...)
			}
		})
	}
}

func TestBindingsImmutability(t *testing.T) {
	tests := []string{
		`cel.bind(x, [], [x + [1], x + [2]]) == [[1], [2]]`,
		`cel.bind(x, [], cel.bind(y, x + [1], cel.bind(z, x + [2], [x, y, z]))) == [[], [1], [2]]`,
		`cel.bind(x, [], [1, 2].map(i, x + [i])) == [[1], [2]]`,
		`cel.bind(x, [], [x + [1]].map(l, l + [2]) + [x]) == [[1, 2], []]`,
		`cel.bind(m, {}, [1, 2].transformList(i, v, m.size() + v)) == [1, 2]`,
		`cel.bind(x, [], [1, 2].transformMap(i, v, x + [v])) == {0: [1], 1: [2]}`,
		`optional.of([]).optMap(x, [x + [1], x + [2]]) == optional.of([[1], [2]])`,
		`optional.of([]).optFlatMap(x, optional.of([x + [1], x + [2]])) == optional.of([[1], [2]])`,
		`[[1], [2]].map(x, x.map(y, y + 1)) == [[2], [3]]`,
		`{'a': 1, 'b': 2}.transformMap(k, v, v + 1) == {'a': 2, 'b': 3}`,
	}
	env, err := cel.NewEnv(Bindings(), TwoVarComprehensions(), cel.OptionalTypes())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	progOpts := map[string][]cel.ProgramOption{
		"default":    {},
		"exhaustive": {cel.EvalOptions(cel.OptExhaustiveEval)},
		"optimized":  {cel.EvalOptions(cel.OptOptimize)},
	}
	for _, tst := range tests {
		expr := tst
		for name, opts := range progOpts {
			t.Run(name+"/"+expr, func(t *testing.T) {
				checkedAST, iss := env.Compile(expr)
				if iss.Err() != nil {
					t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
				}
				prg, err := env.Program(checkedAST, opts...)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				for range 3 {
					out, _, err := prg.Eval(cel.NoVars())
					if err != nil {
						t.Fatalf("prg.Eval() failed: %v", err)
					}
					if out != types.True {
						t.Errorf("prg.Eval(%q) got %v, wanted true", expr, out)
					}
				}
			})
		}
	}
}

func TestNestedComprehensionInitializers(t *testing.T) {
	// The comprehension initializer for xs is evaluated before the loop, but it is itself a
	// loop of another comprehension, so the variables of each comprehension must occupy distinct
	// slots or evaluating the initializer would clobber the variables of the enclosing loop.
	tests := []string{
		`cel.bind(xs, [1, 2, 3].map(i, i * 2), xs.filter(j, j < xs.size())) == [2]`,
		`cel.bind(a, [10, 20].map(z, z + 1), [1, 2, 3].filter(y, y == 1 || (y == 2 && a[0] > 0))) == [1, 2]`,
		`[1, 2, 3].transformList(i, x, cel.bind(y, [3, 4, 5].map(w, i > 0 ? x : 2), [6, 7, 8].map(z, z > 6 ? dyn(z) : dyn(y)))) == [[[2, 2, 2], 7, 8], [[2, 2, 2], 7, 8], [[3, 3, 3], 7, 8]]`,
		`[1, 2, 3].transformList(i, x, cel.bind(y, [3, 4, 5].map(w, i > 0 ? x : 2), [6, 7, 8].map(z, z < 8 ? [z] : y + [z + x]))) == [[[6], [7], [2, 2, 2, 9]], [[6], [7], [2, 2, 2, 10]], [[6], [7], [3, 3, 3, 11]]]`,
		`[1].transformList(i, x, cel.bind(y, [1].map(z, z), y)).size() < 0 || [1, 2, 3].transformList(i, x, cel.bind(y, [3, 4, 5].map(w, i > 0 ? x : 2), [6, 7, 8].map(z, z > 6 ? dyn(z) : dyn(y)))) == [[[2, 2, 2], 7, 8], [[2, 2, 2], 7, 8], [[3, 3, 3], 7, 8]]`,
		`cel.bind(y, [1, 2].map(a, a) + [3, 4].map(b, b * 10), [5, 6].map(c, c == 5 ? [c] : y + [c])) == [[5], [1, 2, 30, 40, 6]]`,
		`cel.bind(y, cel.bind(u, [1, 2].map(a, a + 1), [3, 4].map(b, b + u[0])), [5, 6].filter(c, c == 5 || y[1] == c)) == [5, 6]`,
	}
	env, err := cel.NewEnv(Bindings(), TwoVarComprehensions())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	progOpts := map[string][]cel.ProgramOption{
		"default":    {},
		"exhaustive": {cel.EvalOptions(cel.OptExhaustiveEval)},
		"optimized":  {cel.EvalOptions(cel.OptOptimize)},
		"tracked":    {cel.EvalOptions(cel.OptTrackState)},
	}
	for _, tst := range tests {
		expr := tst
		for name, opts := range progOpts {
			t.Run(name+"/"+expr, func(t *testing.T) {
				checkedAST, iss := env.Compile(expr)
				if iss.Err() != nil {
					t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
				}
				prg, err := env.Program(checkedAST, opts...)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out != types.True {
					t.Errorf("prg.Eval(%q) got %v, wanted true", expr, out)
				}
			})
		}
	}
}

func TestLegacyAccumulatorImmutability(t *testing.T) {
	// The legacy accumulator name `__result__` may be referenced from user expressions, so the
	// comprehension must not use a mutable accumulator.
	tests := []string{
		`[1, 2].map(x, (__result__ + [x]).size()) == [1, 2]`,
		`[1, 2, 3].filter(x, (__result__ + [x]).size() > 1) == []`,
		`[1, 2].map(x, x > 0, (__result__ + [0]).size()) == [1, 2]`,
		`[1, 2].map(x, cel.bind(y, __result__, y + [9]).size()) == [1, 2]`,
		`[1, 2].transformList(i, v, (__result__ + [v]).size()) == [1, 2]`,
	}
	env, err := cel.NewEnv(cel.EnableHiddenAccumulatorName(false), Bindings(), TwoVarComprehensions())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	progOpts := map[string][]cel.ProgramOption{
		"default":    {},
		"exhaustive": {cel.EvalOptions(cel.OptExhaustiveEval)},
		"optimized":  {cel.EvalOptions(cel.OptOptimize)},
	}
	for _, tst := range tests {
		expr := tst
		for name, opts := range progOpts {
			t.Run(name+"/"+expr, func(t *testing.T) {
				checkedAST, iss := env.Compile(expr)
				if iss.Err() != nil {
					t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
				}
				prg, err := env.Program(checkedAST, opts...)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out != types.True {
					t.Errorf("prg.Eval(%q) got %v, wanted true", expr, out)
				}
			})
		}
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
			for b.Loop() {
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
			for b.Loop() {
				prg.Eval(input)
			}
		})
	}
}

// testBlockEnvOptions returns cel.EnvOptions that expose `cel.block([slots...], expr)` and
// `cel.index(n)` macros in tests so `cel.@block` and `@index<n>` ASTs can be written as
// human-readable CEL expressions.
func testBlockEnvOptions() []cel.EnvOption {
	const maxIndices = 10
	opts := make([]cel.EnvOption, 0, maxIndices+2)
	opts = append(opts,
		Bindings(),
		cel.Macros(
			cel.ReceiverMacro("block", 2, func(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
				if !macroTargetMatchesNamespace(celNamespace, target) {
					return nil, nil
				}
				return mef.NewCall("cel.@block", args...), nil
			}),
			cel.ReceiverMacro("index", 1, func(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
				if !macroTargetMatchesNamespace(celNamespace, target) {
					return nil, nil
				}
				idx := args[0].AsLiteral().(types.Int)
				return mef.NewIdent(fmt.Sprintf("@index%d", idx)), nil
			}),
		),
	)
	for i := range maxIndices {
		opts = append(opts, cel.Variable(fmt.Sprintf("@index%d", i), cel.DynType))
	}
	return opts
}

func TestBlockEval(t *testing.T) {
	tests := []struct {
		name string
		expr string
		opts []cel.EnvOption
		in   map[string]any
		out  ref.Val
	}{
		{
			name: "chained block",
			expr: `cel.block([x, cel.index(0), cel.index(1)], cel.index(2) + cel.index(1) + cel.index(0))`,
			opts: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
			},
			in:  map[string]any{"x": "hello"},
			out: types.String("hellohellohello"),
		},
		{
			name: "empty block",
			expr: `cel.block([], !false)`,
			in:   map[string]any{},
			out:  types.True,
		},
		{
			name: "mixed block constant values",
			expr: `cel.block(['hello', 5], size(cel.index(0)) == cel.index(1))`,
			opts: []cel.EnvOption{
				cel.ExtendedValidations(),
			},
			in:  map[string]any{},
			out: types.True,
		},
		{
			name: "mixed block dynamic values",
			expr: `cel.block([x, 5], size(cel.index(0)) == cel.index(1))`,
			opts: []cel.EnvOption{
				cel.Variable("x", cel.StringType),
				cel.ExtendedValidations(),
			},
			in:  map[string]any{"x": "goodbye"},
			out: types.False,
		},
		{
			name: "mixed block constant values dyn var",
			expr: `cel.block(['hello'], size(cel.index(0)) == y)`,
			opts: []cel.EnvOption{
				cel.Variable("y", cel.IntType),
				cel.ExtendedValidations(),
			},
			in:  map[string]any{"y": 5},
			out: types.True,
		},
		{
			name: "block slot evaluated within nested comprehensions",
			expr: `[1, 2].map(x, cel.block([[10, 20].map(y, y + x)], [100].map(z, cel.index(0)[0] + x + z)[0]))`,
			in:   map[string]any{},
			out:  types.DefaultTypeAdapter.NativeToValue([]int64{112, 114}),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			opts := append(testBlockEnvOptions(), tc.opts...)
			env, err := cel.NewEnv(opts...)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			checked, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(checked, cel.EvalOptions(cel.OptOptimize))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
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

func TestBlockSlotZeroAllocs(t *testing.T) {
	env, err := cel.NewEnv(append(testBlockEnvOptions(), cel.Variable("x", cel.StringType))...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	checked, iss := env.Compile(`cel.block([x == 'hello', cel.index(0), 'hello'], cel.index(1) && x == cel.index(2))`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(checked, cel.EvalOptions(cel.OptOptimize))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	vars, err := cel.NewActivation(map[string]any{"x": types.String("hello")})
	if err != nil {
		t.Fatalf("cel.NewActivation() failed: %v", err)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		out, _, err := prg.Eval(vars)
		if err != nil || out != types.True {
			t.Fatalf("prg.Eval() = (%v, %v), wanted (true, nil)", out, err)
		}
	}); allocs != 0 {
		t.Errorf("prg.Eval() allocated %v allocs/run, wanted 0", allocs)
	}
}

func TestBlockSlotLazyAndCached(t *testing.T) {
	var calls int
	env, err := cel.NewEnv(
		append(testBlockEnvOptions(),
			cel.Function("nextVal",
				cel.Overload("next_val", nil, cel.IntType,
					cel.FunctionBinding(func(args ...ref.Val) ref.Val {
						calls++
						return types.Int(calls)
					}),
				),
			),
		)...,
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	checked, iss := env.Compile(`cel.block([nextVal(), 1 / 0], cel.index(0) + cel.index(0))`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(checked)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out != types.Int(2) {
		t.Errorf("prg.Eval() = %v, wanted 2", out)
	}
	if calls != 1 {
		t.Errorf("nextVal() called %d times, wanted 1", calls)
	}
}

func TestBlockSlotNestedShadowing(t *testing.T) {
	env, err := cel.NewEnv(testBlockEnvOptions()...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	checked, iss := env.Compile(`
		cel.block(['outer0', 'outer1'],
			cel.block([cel.index(0) + '_inner'], cel.index(0) + ':' + cel.index(1)) + ':' + cel.index(0))
	`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(checked)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	want := types.String("outer0_inner:outer1:outer0")
	if out != want {
		t.Errorf("prg.Eval() = %v, wanted %v", out, want)
	}
}

func TestBlockSlotObserversAndPartialEval(t *testing.T) {
	env, err := cel.NewEnv(append(testBlockEnvOptions(), cel.Variable("x", cel.StringType))...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	checked, iss := env.Compile(`cel.block([x + '!', 6], size(cel.index(0)) == cel.index(1))`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(
		checked,
		cel.CostTracking(nil),
		cel.MemoryLimit(1_000_000),
		cel.EvalOptions(cel.OptTrackState, cel.OptExhaustiveEval, cel.OptPartialEval),
	)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, det, err := prg.Eval(map[string]any{"x": "hello"})
	if err != nil {
		t.Fatalf("prg.Eval() failed: %v", err)
	}
	if out != types.True {
		t.Errorf("prg.Eval() = %v, wanted true", out)
	}
	if det == nil || det.ActualCost() == nil || *det.ActualCost() == 0 {
		t.Errorf("ActualCost() = %v, wanted non-zero cost", det)
	}

	pVars, err := cel.PartialVars(map[string]any{}, cel.AttributePattern("x"))
	if err != nil {
		t.Fatalf("cel.PartialVars() failed: %v", err)
	}
	unkOut, _, err := prg.Eval(pVars)
	if err != nil {
		t.Fatalf("prg.Eval(partial) failed: %v", err)
	}
	if !types.IsUnknown(unkOut) {
		t.Errorf("prg.Eval(partial) = %v (%T), wanted unknown", unkOut, unkOut)
	}
}

func TestBlockSlotComprehensions(t *testing.T) {
	env, err := cel.NewEnv(testBlockEnvOptions()...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		name string
		expr string
	}{
		{
			name: "chained_comprehension_slots_in_block",
			expr: `cel.block(
				[
					[1].map(x, x + 1),
					[1, 2].map(y, y < 2 ? dyn(y) : cel.index(0)),
					[1, 2, 3].map(z, z == 1 ? dyn(z) : (z == 2 ? cel.index(1) : cel.index(0)))
				],
				[1, 2].map(x, cel.index(2))
			) == [[1, [1, [2]], [2]], [1, [1, [2]], [2]]]`,
		},
		{
			name: "comprehension_vars_and_accumulators_live_after_lazy_block_slots",
			expr: `cel.block(
				[
					[1].map(x, x + 1),
					[1, 2].map(y, y < 2 ? y : cel.index(0)[0] + y),
					[1, 2, 3].map(z, z == 1 ? z : (z == 2 ? cel.index(1)[1] + z : cel.index(0)[0] + z))
				],
				[1, 2].filter(x, x == 1 || cel.index(2)[1] == x + 4).map(x, cel.index(2)[2] + x)
			) == [6, 7]`,
		},
	}
	progOpts := map[string][]cel.ProgramOption{
		"default":    {},
		"exhaustive": {cel.EvalOptions(cel.OptExhaustiveEval)},
		"optimized":  {cel.EvalOptions(cel.OptOptimize)},
		"tracked":    {cel.EvalOptions(cel.OptTrackState)},
	}
	for _, tc := range tests {
		checked, iss := env.Compile(tc.expr)
		if iss.Err() != nil {
			t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
		}
		for optName, opts := range progOpts {
			t.Run(tc.name+"/"+optName, func(t *testing.T) {
				prg, err := env.Program(checked, opts...)
				if err != nil {
					t.Fatalf("env.Program() failed: %v", err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if err != nil {
					t.Fatalf("prg.Eval() failed: %v", err)
				}
				if out != types.True {
					t.Errorf("prg.Eval() = %v, wanted true", out)
				}
			})
		}
	}
}

func TestBlockSlotOptionalValues(t *testing.T) {
	opts := append(
		testBlockEnvOptions(),
		cel.OptionalTypes(),
		cel.Variable("opt_map", cel.OptionalType(cel.MapType(cel.StringType, cel.MapType(cel.StringType, cel.StringType)))),
	)
	env, err := cel.NewEnv(opts...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	expr := `cel.block(
		[
			opt_map[?'k1'],
			cel.index(0).?a,
			'k2'
		],
		[cel.index(1), opt_map[?cel.index(2)].?a]
	)`
	checked, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(checked, cel.EvalOptions(cel.OptTrackState, cel.OptPartialEval))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	presentMap := types.OptionalOf(env.CELTypeAdapter().NativeToValue(map[string]map[string]string{
		"k1": {"a": "b"},
	}))
	out, _, err := prg.Eval(map[string]any{"opt_map": presentMap})
	if err != nil {
		t.Fatalf("prg.Eval(present) failed: %v", err)
	}
	wantPresent := env.CELTypeAdapter().NativeToValue([]any{types.OptionalOf(types.String("b")), types.OptionalNone})
	if out.Equal(wantPresent) != types.True {
		t.Errorf("prg.Eval(present) = %v, wanted %v", out, wantPresent)
	}

	outNone, _, err := prg.Eval(map[string]any{"opt_map": types.OptionalNone})
	if err != nil {
		t.Fatalf("prg.Eval(none) failed: %v", err)
	}
	wantNone := env.CELTypeAdapter().NativeToValue([]any{types.OptionalNone, types.OptionalNone})
	if outNone.Equal(wantNone) != types.True {
		t.Errorf("prg.Eval(none) = %v, wanted %v", outNone, wantNone)
	}
}

func BenchmarkBlockEval(b *testing.B) {
	env, err := cel.NewEnv(append(testBlockEnvOptions(), cel.Variable("x", cel.StringType))...)
	if err != nil {
		b.Fatalf("cel.NewEnv() failed: %v", err)
	}
	checked, iss := env.Compile(`cel.block([x, cel.index(0), cel.index(1)], cel.index(2) + cel.index(1) + cel.index(0))`)
	if iss.Err() != nil {
		b.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(checked, cel.EvalOptions(cel.OptOptimize))
	if err != nil {
		b.Fatalf("env.Program() failed: %v", err)
	}
	input := map[string]any{"x": "hello"}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
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

func TestSyncSelect_ProtoInBind(t *testing.T) {
	e, err := cel.NewEnv(
		Bindings(),
		cel.Types(&proto3pb.TestAllTypes{}),
		cel.Function("resp",
			cel.Overload("resp_proto", nil, cel.ObjectType("google.expr.proto3.test.TestAllTypes"),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.NewUnknown(1, types.NewAttributeTrail("resp"))
				}),
			),
		),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	astBind, iss := e.Compile(`cel.bind(r, resp(), r.single_int32 == 0)`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	prgBind, err := e.Program(astBind)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	valBind, _, errBind := prgBind.Eval(cel.NoVars())
	if errBind != nil {
		t.Fatalf("Eval() failed: %v", errBind)
	}
	if !types.IsUnknown(valBind) {
		t.Fatalf("Eval() got %v (%T), wanted unknown", valBind, valBind)
	}
}

// TestBindingsLateBoundFunctions verifies that late-bound function implementations remain
// reachable from inside the scopes introduced by cel.bind and cel.@block.
//
// Both constructs push an activation which wraps, rather than extends, the one supplied to Eval.
// The bindings are located once when the evaluation begins and carried on the frame, so these
// scopes do not need to forward the lookup themselves.
func TestBindingsLateBoundFunctions(t *testing.T) {
	env, err := cel.NewEnv(
		Bindings(),
		cel.Variable("x", cel.IntType),
		cel.Function("inc",
			cel.Overload("inc_int", []*cel.Type{cel.IntType}, cel.IntType, cel.LateFunctionBinding()),
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		name string
		expr string
		want ref.Val
	}{
		{
			name: "call inside a bind body",
			expr: "cel.bind(y, x + 1, inc(y))",
			want: types.Int(3),
		},
		{
			name: "call inside a bind initializer",
			expr: "cel.bind(y, inc(x), y + 1)",
			want: types.Int(3),
		},
		{
			name: "nested binds",
			expr: "cel.bind(y, inc(x), cel.bind(z, inc(y), inc(z)))",
			want: types.Int(4),
		},
		{
			name: "call inside a comprehension inside a bind",
			expr: "cel.bind(y, [1, 2], y.map(i, inc(i)))",
			want: types.DefaultTypeAdapter.NativeToValue([]int64{2, 3}),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			vars, err := cel.FunctionVars(map[string]any{"x": 1}, map[string]cel.LateBoundFunction{
				"inc": func(overloadID string, args ...ref.Val) ref.Val {
					return types.Int(args[0].(types.Int) + 1)
				},
			})
			if err != nil {
				t.Fatalf("cel.FunctionVars() failed: %v", err)
			}
			out, _, err := prg.Eval(vars)
			if err != nil {
				t.Fatalf("Eval() failed: %v", err)
			}
			if out.Equal(tc.want) != types.True {
				t.Errorf("Eval() got %v, wanted %v", out, tc.want)
			}
		})
	}
}
func TestBlockMemoryTrackingAcrossPasses(t *testing.T) {
	// async_list(5) only produces its result on a later ConcurrentEval pass, at which point the
	// expressions which depend on it are observed a second time. Values which are not within a
	// block scope must be tracked on every pass, regardless of the memory sampling interval. The
	// peak memory of 6 is the list container plus its 5 elements.
	tests := []struct {
		name string
		expr string
	}{
		{
			// An empty block introduces no scope, so the call within it is tracked.
			name: "empty block",
			expr: `cel.block([], size(async_list(5)) == 5)`,
		},
		{
			// The block itself is outside of its own scope, so its result is tracked.
			name: "block result",
			expr: `size(cel.block([async_list(5)], cel.index(0))) == 5`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := asyncListEnv(t)
			checked, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(checked,
				cel.MemoryTracking(types.MemoryTrackerSampleInterval(10)))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			checkConcurrentEvalPeak(t, prg, 6)
		})
	}
}

func TestBindingsMemoryTrackingAcrossPasses(t *testing.T) {
	// The cel.bind() initializer is evaluated once per comprehension, outside of the scope of the
	// bind, even when the bound variable is first referenced from within a nested loop. The
	// initializer is therefore tracked on every pass of a ConcurrentEval, regardless of the memory
	// sampling interval.
	env := asyncListEnv(t)
	checked, iss := env.Compile(`cel.bind(z, async_list(5), [1].map(i, z.size() + i)) == [6]`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	prg, err := env.Program(checked, cel.MemoryTracking(types.MemoryTrackerSampleInterval(10)))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	checkConcurrentEvalPeak(t, prg, 6)
}

// asyncListEnv returns an environment with cel.bind() and cel.block() support, as well as an
// async_list(n) function which asynchronously produces a list of n zeros.
func asyncListEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := cel.NewEnv(
		append(testBlockEnvOptions(),
			cel.Function("async_list",
				cel.Overload("async_list_int", []*cel.Type{cel.IntType}, cel.ListType(cel.IntType),
					cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
						return types.DefaultTypeAdapter.NativeToValue(make([]int64, int(args[0].(types.Int))))
					}),
				),
			),
		)...,
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	return env
}

// checkConcurrentEvalPeak evaluates the program, which must produce true, and checks its peak
// tracked memory.
func checkConcurrentEvalPeak(t *testing.T, prg cel.Program, wantPeak uint32) {
	t.Helper()
	var res cel.EvalResult
	select {
	case res = <-prg.ConcurrentEval(context.Background(), cel.NoVars()):
	case <-time.After(5 * time.Second):
		t.Fatal("ConcurrentEval() timed out")
	}
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() failed: %v", res.Err)
	}
	if res.Val != types.True {
		t.Fatalf("ConcurrentEval() got %v, wanted true", res.Val)
	}
	peak := res.EvalDetails.PeakMemory()
	if peak == nil {
		t.Fatal("EvalDetails.PeakMemory() got nil, wanted non-nil peak")
	}
	if *peak != wantPeak {
		t.Errorf("EvalDetails.PeakMemory() got %d, wanted %d", *peak, wantPeak)
	}
}
