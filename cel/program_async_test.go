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

package cel_test

import (
	"context"
	"errors"
	"math/rand"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/cel/async"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/google/cel-go/interpreter"
	"github.com/google/cel-go/test"
)

func TestConcurrentEval(t *testing.T) {
	cases := []struct {
		name         string
		expr         string
		vars         any
		opts         []any
		maxConc      int
		want         any
		wantLaunches int32
		trackCost    bool
		trackState   bool
		wantErr      string
		wantCost     uint64
		leakCheck    bool
	}{
		{
			name: "sync_eval",
			expr: `x + 1`,
			vars: map[string]any{"x": 10},
			opts: []any{cel.Variable("x", cel.IntType)},
			want: 11,
		},
		{
			name: "single_async",
			expr: `async_func(42) + 1`,
			opts: []any{
				cel.Function("async_func",
					cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
						cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
							time.Sleep(1 * time.Millisecond)
							return args[0]
						}),
					),
				),
			},
			want: 43,
		},
		{
			name: "completion_buffer_size",
			expr: `async_func(42) + 1`,
			opts: []any{
				cel.Function("async_func",
					cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
						cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
							time.Sleep(1 * time.Millisecond)
							return args[0]
						}),
					),
				),
				cel.AsyncCompletionBufferSize(16),
			},
			want: 43,
		},
		{
			name:    "outside_parallel_conc_1",
			expr:    `async_inc(10) + async_inc(20)`,
			maxConc: 1,
			want:    32,
		},
		{
			name:      "outside_parallel_conc_unlimited",
			expr:      `async_inc(10) + async_inc(20)`,
			maxConc:   -1,
			trackCost: true,
			want:      32,
		},
		{
			name:      "outside_chained_conc_1",
			expr:      `async_inc(async_inc(10))`,
			maxConc:   1,
			trackCost: true,
			want:      12,
		},
		{
			name:    "outside_chained_conc_default",
			expr:    `async_inc(async_inc(10))`,
			maxConc: 0,
			want:    12,
		},
		{
			name:         "comprehension_single",
			expr:         `[1, 2, 3].map(i, dbl(i))`,
			want:         []int64{2, 4, 6},
			wantLaunches: 3,
		},
		{
			name:      "comprehension_single_conc_2",
			expr:      `[1, 2, 3].map(i, async_inc(i))`,
			maxConc:   2,
			trackCost: true,
			want:      []int64{2, 3, 4},
		},
		{
			name:    "comprehension_single_conc_unlimited",
			expr:    `[1, 2, 3].map(i, async_inc(i))`,
			maxConc: -1,
			want:    []int64{2, 3, 4},
		},
		{
			name:      "comprehension_chained_conc_1",
			expr:      `[1, 2, 3].map(i, async_inc(async_inc(i)))`,
			maxConc:   1,
			trackCost: true,
			want:      []int64{3, 4, 5},
		},
		{
			name:    "comprehension_chained_conc_default",
			expr:    `[1, 2, 3].map(i, async_inc(async_inc(i)))`,
			maxConc: 0,
			want:    []int64{3, 4, 5},
		},
		{
			name:      "nested_comprehension_chained_conc_2",
			expr:      `[1, 2].map(i, [10, 20].map(j, async_inc(async_inc(i + j))))`,
			maxConc:   2,
			trackCost: true,
			want:      [][]int64{{13, 23}, {14, 24}},
		},
		{
			name:    "nested_comprehension_chained_conc_unlimited",
			expr:    `[1, 2].map(i, [10, 20].map(j, async_inc(async_inc(i + j))))`,
			maxConc: -1,
			want:    [][]int64{{13, 23}, {14, 24}},
		},
		{
			name: "fake_rpc",
			expr: `rpc("a") + rpc("b") + rpc("c")`,
			opts: []any{
				cel.Function("rpc",
					cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
						cel.AsyncBinding(test.FakeRPC(time.Second)),
					),
				),
			},
			want: "a success!b success!c success!",
		},
		{
			name:      "drain_all",
			expr:      `delayed_rpc("a", 1) + delayed_rpc("b", 2) + delayed_rpc("c", 10)`,
			opts:      []any{cel.ConcurrentDrainStrategy(async.DrainAll())},
			trackCost: true,
			wantCost:  10,
			want:      "abc",
		},
		{
			name:      "drain_ready_batched",
			expr:      `delayed_rpc("a", 1) + delayed_rpc("b", 2) + delayed_rpc("c", 10)`,
			opts:      []any{cel.ConcurrentDrainStrategy(async.DrainReady(15 * time.Millisecond))},
			trackCost: true,
			wantCost:  10,
			want:      "abc",
		},
		{
			name:      "drain_ready_partial_debounce",
			expr:      `delayed_rpc("a", 1) + delayed_rpc("b", 2) + delayed_rpc("c", 10)`,
			opts:      []any{cel.ConcurrentDrainStrategy(async.DrainReady(2 * time.Millisecond))},
			trackCost: true,
			wantCost:  15,
			want:      "abc",
		},
		{
			name:      "drain_none",
			expr:      `delayed_rpc("a", 1) + delayed_rpc("b", 2) + delayed_rpc("c", 10)`,
			opts:      []any{cel.ConcurrentDrainStrategy(async.DrainNone())},
			trackCost: true,
			wantCost:  20,
			want:      "abc",
		},
		{
			name: "exhaustive_eval",
			expr: `async_inc(10) > 0`,
			opts: []any{
				cel.EvalOptions(cel.OptExhaustiveEval),
			},
			trackState: true,
			want:       true,
		},
		{
			name:       "async_error",
			expr:       `async_fail()`,
			opts:       []any{cel.EvalOptions(cel.OptTrackState)},
			trackState: true,
			wantErr:    "async failure",
		},
		{
			name:         "short_circuit_and_false",
			expr:         `false && (async_inc(10) == 11)`,
			want:         false,
			wantLaunches: 0,
		},
		{
			name:         "short_circuit_or_true",
			expr:         `true || (async_inc(10) == 11)`,
			want:         true,
			wantLaunches: 0,
		},
		{
			name:         "short_circuit_ternary_false",
			expr:         `false ? async_inc(10) : 42`,
			want:         42,
			wantLaunches: 0,
		},
		{
			name:         "short_circuit_ternary_true",
			expr:         `true ? async_inc(10) : 42`,
			want:         11,
			wantLaunches: 1,
		},
		{
			name:         "eval_and_true",
			expr:         `true && (async_inc(10) == 11)`,
			want:         true,
			wantLaunches: 1,
		},
		{
			name:         "eval_or_false",
			expr:         `false || (async_inc(10) == 11)`,
			want:         true,
			wantLaunches: 1,
		},
		{
			name: "eval_left_async_or_async",
			expr: `(async_inc(10) == 11) || (async_inc(20) == 21)`,
			want: true,
		},
		{
			name: "eval_left_async_and_async",
			expr: `(async_inc(10) == 0) && (async_inc(20) == 21)`,
			want: false,
		},
		{
			name:         "eval_or_var_expr_short_circuit_pass1",
			expr:         `(async_inc(10) == 11) || (11 - x == 10)`,
			vars:         map[string]any{"x": 1},
			opts:         []any{cel.Variable("x", cel.IntType)},
			want:         true,
			wantLaunches: 0,
		},
		{
			name:         "eval_or_var_expr_await_async",
			expr:         `(async_inc(10) == 11) || (11 - x == 10)`,
			vars:         map[string]any{"x": 0},
			opts:         []any{cel.Variable("x", cel.IntType)},
			want:         true,
			wantLaunches: 1,
		},
		{
			name:         "compile_time_fold_or_true",
			expr:         `(async_inc(10) == 11) || true`,
			want:         true,
			wantLaunches: 0,
		},
		{
			name:         "compile_time_fold_and_false",
			expr:         `(async_inc(10) == 11) && false`,
			want:         false,
			wantLaunches: 0,
		},
		{
			name:    "comprehension_async_error",
			expr:    `[1, 2, 3].map(i, i == 2 ? async_fail() : async_inc(i))`,
			wantErr: "async failure",
		},
		{
			name:         "completion_buffer_size_zero",
			expr:         `async_inc(10) + 1`,
			opts:         []any{cel.AsyncCompletionBufferSize(0)},
			want:         12,
			wantLaunches: 1,
		},
		{
			name:         "completion_buffer_size_negative",
			expr:         `async_inc(10) + 1`,
			opts:         []any{cel.AsyncCompletionBufferSize(-1)},
			want:         12,
			wantLaunches: 1,
		},
		// Tests the scenario where there are more async invocations than completion buffer.
		{
			name:    "more requests than completion buffer w/ debounce",
			expr:    `lists.range(1000).exists(i, async_inc(i) == 1000)`,
			maxConc: 5,
			opts: []any{
				ext.Lists(),
				cel.AsyncCompletionBufferSize(10),
				cel.ConcurrentDrainStrategy(async.DrainReady(10 * time.Microsecond)),
			},
			want:      true,
			leakCheck: true,
		},
		// Tests the scenario where there are more async invocations than completion buffer.
		{
			name:    "more requests than completion buffer w/ drain all",
			expr:    `lists.range(1000).exists(i, async_inc(i) == 1000)`,
			maxConc: 5,
			opts: []any{
				ext.Lists(),
				cel.AsyncCompletionBufferSize(5),
				cel.ConcurrentDrainStrategy(async.DrainAll()),
			},
			want:      true,
			leakCheck: true,
		},
		{
			name: "more requests than completion buffer w/ drain none",
			expr: `lists.range(300).exists(i, async_inc(i) == 300)`,
			opts: []any{
				ext.Lists(),
				cel.AsyncCompletionBufferSize(1),
				cel.AsyncMaxConcurrency(2),
				cel.ConcurrentDrainStrategy(async.DrainNone()),
			},
			want:      true,
			leakCheck: true,
		},
		// Tests the scenario where there are more async invocations than completion buffer across two different comprehensions.
		{
			name: "chained comprehensions with drain none",
			expr: `lists.range(300).map(i, async_inc(i) * 2).exists(j, j == 600)`,
			opts: []any{
				ext.Lists(),
				cel.AsyncCompletionBufferSize(4),
				cel.AsyncMaxConcurrency(2),
				cel.ConcurrentDrainStrategy(async.DrainNone()),
			},
			want:      true,
			leakCheck: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var launches, live, maxLive atomic.Int32

			asyncInc := cel.Function("async_inc",
				cel.Overload("async_inc_int", []*cel.Type{cel.IntType}, cel.IntType,
					cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
						launches.Add(1)
						cur := live.Add(1)
						for {
							old := maxLive.Load()
							if cur <= old || maxLive.CompareAndSwap(old, cur) {
								break
							}
						}
						// Add a random delay between 1-500 microseconds to simulate network latency.
						time.Sleep(time.Duration(rand.Intn(500)+1) * time.Microsecond)
						live.Add(-1)
						v := int64(args[0].(types.Int))
						return types.Int(v + 1)
					}),
				),
			)

			dblFunc := cel.Function("dbl",
				cel.Overload("dbl_int", []*cel.Type{cel.IntType}, cel.IntType,
					cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
						launches.Add(1)
						time.Sleep(1 * time.Millisecond)
						return args[0].(types.Int) * 2
					})),
			)

			rpcFunc := cel.Function("rpc",
				cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
					cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
						time.Sleep(1 * time.Millisecond)
						return args[0]
					}),
				),
			)

			delayedRpcFunc := cel.Function("delayed_rpc",
				cel.Overload("delayed_rpc_string_int", []*cel.Type{cel.StringType, cel.IntType}, cel.StringType,
					cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
						msg := string(args[0].(types.String))
						delayMs := time.Duration(int64(args[1].(types.Int))) * time.Millisecond
						time.Sleep(delayMs)
						return types.String(msg)
					}),
				),
			)

			asyncFailFunc := cel.Function("async_fail",
				cel.Overload("async_fail_void", []*cel.Type{}, cel.IntType,
					cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
						return types.NewErr("async failure")
					}),
				),
			)

			testOpts := append([]any{asyncInc, dblFunc, rpcFunc, delayedRpcFunc, asyncFailFunc}, tc.opts...)
			if tc.maxConc != 0 {
				testOpts = append(testOpts, cel.AsyncMaxConcurrency(tc.maxConc))
			}
			if tc.trackCost {
				testOpts = append(testOpts, cel.EvalOptions(cel.OptTrackCost))
			}

			vars := tc.vars
			if vars == nil {
				vars = cel.NoVars()
			}

			prg := mustProgram(t, tc.expr, testOpts...)

			// Count the active goroutines
			initialCount := runtime.NumGoroutine()
			res := awaitEval(t, prg, context.Background(), vars)

			if tc.wantErr != "" {
				if res.Err == nil || !strings.Contains(res.Err.Error(), tc.wantErr) {
					t.Fatalf("ConcurrentEval(%q) error = %v, want error containing %q", tc.expr, res.Err, tc.wantErr)
				}
			} else {
				if res.Err != nil {
					t.Fatalf("ConcurrentEval(%q) error: %v", tc.expr, res.Err)
				}
				wantVal := types.DefaultTypeAdapter.NativeToValue(tc.want)
				if res.Val.Equal(wantVal) != types.True {
					t.Errorf("ConcurrentEval(%q) = %v, want %v", tc.expr, res.Val, wantVal)
				}
			}

			if tc.maxConc > 0 {
				if got := maxLive.Load(); got > int32(tc.maxConc) {
					t.Errorf("max observed concurrency = %d, want <= %d", got, tc.maxConc)
				}
			}
			if tc.wantLaunches > 0 {
				if got := launches.Load(); got != tc.wantLaunches {
					t.Errorf("async launches = %d, want %d", got, tc.wantLaunches)
				}
			}
			if tc.trackCost {
				if res.EvalDetails == nil || res.EvalDetails.ActualCost() == nil {
					t.Errorf("res.EvalDetails.ActualCost() is nil, want non-nil when cost tracking is enabled")
				} else if cost := *res.EvalDetails.ActualCost(); cost == 0 {
					t.Errorf("ActualCost() = 0, want > 0")
				}
			}
			if tc.wantCost > 0 {
				if res.EvalDetails == nil || res.EvalDetails.ActualCost() == nil {
					t.Errorf("res.EvalDetails.ActualCost() is nil, want %d", tc.wantCost)
				} else if got := *res.EvalDetails.ActualCost(); got != tc.wantCost {
					t.Errorf("ActualCost() = %d, want %d", got, tc.wantCost)
				}
			}
			if tc.trackState {
				if res.EvalDetails == nil || res.EvalDetails.State() == nil {
					t.Errorf("res.EvalDetails.State() is nil, want non-nil")
				}
			}

			if tc.leakCheck {
				// Give the runtime a brief moment to clean up if there were async launches.
				time.Sleep(1 * time.Second)

				// Capture the final count
				finalCount := runtime.NumGoroutine()

				// Assert that no new goroutines were left behind
				if finalCount > initialCount {
					t.Errorf("Goroutine leak detected! Initial: %d, Final: %d", initialCount, finalCount)
				}
			}
		})
	}
}

func TestContextEvalRejectsAsync(t *testing.T) {
	prg := mustProgram(t, `rpc("a")`,
		cel.Function("rpc",
			cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val { return args[0] }))),
	)
	_, _, err := prg.ContextEval(context.Background(), cel.NoVars())
	if err == nil || !strings.Contains(err.Error(), "ConcurrentEval") {
		t.Errorf("ContextEval() on async expr = %v, want error mentioning ConcurrentEval", err)
	}
}

func TestEvalRejectsAsync(t *testing.T) {
	prg := mustProgram(t, `rpc("a")`,
		cel.Function("rpc",
			cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val { return args[0] }))),
	)
	_, _, err := prg.Eval(cel.NoVars())
	if err == nil || !strings.Contains(err.Error(), "ConcurrentEval") {
		t.Errorf("Eval() on async expr = %v, want error mentioning ConcurrentEval", err)
	}
}

func TestContextEvalAllowsPartialUnknown(t *testing.T) {
	// A variable unknown from partial evaluation must NOT be mistaken for an async call.
	prg := mustProgram(t, `x + 1`,
		cel.Variable("x", cel.IntType),
		cel.EvalOptions(cel.OptPartialEval),
	)
	pvars, err := cel.PartialVars(map[string]any{}, cel.AttributePattern("x"))
	if err != nil {
		t.Fatalf("PartialVars() failed: %v", err)
	}
	out, _, err := prg.ContextEval(context.Background(), pvars)
	if err != nil {
		t.Fatalf("ContextEval() with partial unknown returned error: %v", err)
	}
	if !types.IsUnknown(out) {
		t.Errorf("ContextEval() = %v, want Unknown", out)
	}
}

func TestConcurrentEvalAllowsPartialUnknown(t *testing.T) {
	prg := mustProgram(t, `async_func(42) + x`,
		cel.Variable("x", cel.IntType),
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(5 * time.Millisecond)
					return args[0]
				}),
			),
		),
		cel.EvalOptions(cel.OptPartialEval),
	)
	pvars, err := cel.PartialVars(map[string]any{}, cel.AttributePattern("x"))
	if err != nil {
		t.Fatalf("PartialVars() failed: %v", err)
	}
	res := awaitEval(t, prg, context.Background(), pvars)
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() with partial unknown returned error: %v", res.Err)
	}
	if !types.IsUnknown(res.Val) {
		t.Errorf("ConcurrentEval() = %v, want Unknown", res.Val)
	}
}

func TestConcurrentEvalAsyncObserver(t *testing.T) {
	obs := &countingObserver{}
	prg := mustProgram(t, `async_func(10) + async_func(20)`,
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(5 * time.Millisecond)
					return args[0]
				}),
			),
		),
		cel.AsyncCallObserver(obs),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	if res.Val.Equal(types.Int(30)) != types.True {
		t.Errorf("ConcurrentEval() = %v, want 30", res.Val)
	}
	if got := obs.started.Load(); got != 2 {
		t.Errorf("OnCallStarted count = %d, want 2", got)
	}
	if got := obs.finished.Load(); got != 2 {
		t.Errorf("OnCallFinished count = %d, want 2", got)
	}
}

func TestConcurrentEvalProgramThreadSafety(t *testing.T) {
	prg := mustProgram(t, `async_func(x) + 1`,
		cel.Variable("x", cel.IntType),
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(5 * time.Millisecond)
					return args[0]
				}),
			),
		),
	)

	const numGoroutines = 10
	errCh := make(chan error, numGoroutines)
	for i := range numGoroutines {
		go func(val int64) {
			res := awaitEval(t, prg, context.Background(), map[string]any{"x": val})
			if res.Err != nil {
				errCh <- res.Err
				return
			}
			if res.Val.Equal(types.Int(val+1)) != types.True {
				errCh <- errors.New("unexpected eval result")
				return
			}
			errCh <- nil
		}(int64(i * 10))
	}

	for range numGoroutines {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent thread safety evaluation failed: %v", err)
		}
	}
}

func TestConcurrentEvalPreCanceledContext(t *testing.T) {
	prg := mustProgram(t, `async_func(42)`,
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					return args[0]
				}),
			),
		),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := <-prg.ConcurrentEval(ctx, cel.NoVars())
	if res.Err == nil || !errors.Is(res.Err, context.Canceled) {
		t.Errorf("ConcurrentEval() on pre-canceled context = %v, want context.Canceled", res.Err)
	}
}

func TestSyncEvalRejectsAsyncBeforeEvaluating(t *testing.T) {
	// The async guard must fire at the entry point, before any evaluation: the async function
	// must never be invoked (no goroutines launched, no work done) for Eval or ContextEval.
	var called atomic.Int32
	prg := mustProgram(t, `rpc("a")`,
		cel.Function("rpc",
			cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					called.Add(1)
					return args[0]
				}))),
	)

	if _, _, err := prg.Eval(cel.NoVars()); err == nil || !strings.Contains(err.Error(), "ConcurrentEval") {
		t.Errorf("Eval() = %v, want ConcurrentEval error", err)
	}
	if _, _, err := prg.ContextEval(context.Background(), cel.NoVars()); err == nil || !strings.Contains(err.Error(), "ConcurrentEval") {
		t.Errorf("ContextEval() = %v, want ConcurrentEval error", err)
	}
	if got := called.Load(); got != 0 {
		t.Errorf("async function invoked %d times; the guard must reject before evaluating", got)
	}
}

func TestSyncEvalRejectedInAsyncEnv(t *testing.T) {
	// Env-level rejection: an environment that declares any async function rejects the synchronous
	// entry points even for an expression that does not call the async function. Callers needing
	// synchronous evaluation should build a separate, non-async environment.
	prg := mustProgram(t, `x + 1`, // pure, synchronous, does not use rpc
		cel.Variable("x", cel.IntType),
		cel.Function("rpc",
			cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val { return args[0] }))),
	)
	if _, _, err := prg.Eval(map[string]any{"x": 1}); err == nil || !strings.Contains(err.Error(), "ConcurrentEval") {
		t.Errorf("Eval() in async env = %v, want ConcurrentEval error", err)
	}

	// A separate non-async env evaluates the same expression synchronously.
	syncPrg := mustProgram(t, `x + 1`, cel.Variable("x", cel.IntType))
	out, _, err := syncPrg.Eval(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("Eval() in non-async env returned error: %v", err)
	}
	if out.Equal(types.Int(2)) != types.True {
		t.Errorf("Eval() = %v, want 2", out)
	}
}

func TestConcurrentEvalDrainReady(t *testing.T) {
	cases := []struct {
		name       string
		expr       string
		debounce   time.Duration
		want       ref.Val
		wantPasses int
		minElapsed time.Duration
	}{
		{
			name:       "timer_reset",
			expr:       `delayed_rpc("a", 10) + delayed_rpc("b", 25) + delayed_rpc("c", 200)`,
			debounce:   250 * time.Millisecond,
			want:       types.String("abc"),
			wantPasses: 2,
			minElapsed: 200 * time.Millisecond,
		},
		{
			name:       "timer_reset",
			expr:       `delayed_rpc("a", 10) + delayed_rpc("b", 25) + delayed_rpc("c", 200)`,
			debounce:   60 * time.Millisecond,
			want:       types.String("abc"),
			wantPasses: 3,
			minElapsed: 200 * time.Millisecond,
		},
		{
			name:       "timer_already_fired",
			expr:       `delayed_rpc("a", 5) + delayed_rpc("b", 30) + delayed_rpc("c", 200)`,
			debounce:   1 * time.Millisecond,
			want:       types.String("abc"),
			wantPasses: 4,
			minElapsed: 200 * time.Millisecond,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var evalPasses atomic.Int32
			opts := []any{
				cel.Function("delayed_rpc",
					cel.Overload("delayed_rpc_string_int", []*cel.Type{cel.StringType, cel.IntType}, cel.StringType,
						cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
							msg := string(args[0].(types.String))
							delayMs := time.Duration(int64(args[1].(types.Int))) * time.Millisecond
							time.Sleep(delayMs)
							return types.String(msg)
						}),
					),
				),
				cel.ConcurrentDrainStrategy(async.DrainReady(tc.debounce)),
				trackEvalPasses(&evalPasses),
			}

			start := time.Now()
			prg := mustProgram(t, tc.expr, opts...)
			res := awaitEval(t, prg, context.Background(), cel.NoVars())
			elapsed := time.Since(start)

			if res.Err != nil {
				t.Fatalf("ConcurrentEval() error: %v", res.Err)
			}
			if res.Val.Equal(tc.want) != types.True {
				t.Errorf("ConcurrentEval() = %v, want %v", res.Val, tc.want)
			}
			if tc.wantPasses > 0 {
				if got := evalPasses.Load(); got != int32(tc.wantPasses) {
					t.Errorf("evaluation loop pass count = %d, want %d", got, tc.wantPasses)
				}
			}
			if elapsed < tc.minElapsed {
				t.Errorf("evaluation completed in %v, want >= %v", elapsed, tc.minElapsed)
			}
		})
	}
}

func TestConcurrentEvalCancelDuringDebounce(t *testing.T) {
	// Tests context cancellation while awaiting a debounce timeout in the completion drain loop.
	prg := mustProgram(t, `delayed_rpc("first", 10) + delayed_rpc("second", 1000)`,
		cel.Function("delayed_rpc",
			cel.Overload("delayed_rpc_string_int", []*cel.Type{cel.StringType, cel.IntType}, cel.StringType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					msg := string(args[0].(types.String))
					delayMs := time.Duration(int64(args[1].(types.Int))) * time.Millisecond
					time.Sleep(delayMs)
					return types.String(msg)
				}),
			),
		),
		cel.ConcurrentDrainStrategy(async.DrainReady(10*time.Second)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	resCh := prg.ConcurrentEval(ctx, cel.NoVars())

	// Wait for the first call (10ms) to complete and enter the 10-second debounce wait.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case res := <-resCh:
		if res.Err == nil || !errors.Is(res.Err, context.Canceled) {
			t.Fatalf("ConcurrentEval() error = %v, want context.Canceled", res.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ConcurrentEval() timed out waiting for cancellation during debounce")
	}
}

func TestConcurrentEvalRecover(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Function("panic",
			cel.Overload("global_panic", []*cel.Type{}, cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					panic("watch me recover")
				}),
			),
		),
		cel.Function("cancel_panic",
			cel.Overload("global_cancel_panic", []*cel.Type{}, cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					panic(interpreter.EvalCancelledError{Message: "eval cancelled", Cause: interpreter.ContextCancelled})
				}),
			),
		),
		cel.Function("sleep_func",
			cel.Overload("global_sleep_func", []*cel.Type{}, cel.BoolType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(1 * time.Second)
					return types.True
				}),
			),
		),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		prgOpts []cel.ProgramOption
		getCtx  func() (context.Context, context.CancelFunc)
		wantErr any
	}{
		{
			name:    "panic",
			expr:    "panic()",
			wantErr: "internal error: watch me recover",
		},
		{
			name:    "panic_tracked_state",
			expr:    "panic()",
			prgOpts: []cel.ProgramOption{cel.EvalOptions(cel.OptTrackState)},
			wantErr: "internal error: watch me recover",
		},
		{
			name:    "eval_cancelled_error",
			expr:    "cancel_panic()",
			wantErr: &interpreter.EvalCancelledError{},
		},
		{
			name: "context_timeout",
			expr: "sleep_func()",
			getCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Millisecond)
			},
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast, tc.prgOpts...)
			if err != nil {
				t.Fatalf("env.Program(ast) failed: %v", err)
			}
			ctx := context.Background()
			if tc.getCtx != nil {
				var cancel context.CancelFunc
				ctx, cancel = tc.getCtx()
				defer cancel()
			}
			res := awaitEval(t, prg, ctx, cel.NoVars())
			if tc.wantErr != nil {
				if res.Err == nil {
					t.Fatalf("ConcurrentEval() error = nil, want %v", tc.wantErr)
				}
				switch want := tc.wantErr.(type) {
				case string:
					if res.Err.Error() != want && !strings.Contains(res.Err.Error(), want) {
						t.Errorf("ConcurrentEval() error = %v, want %q", res.Err, want)
					}
				default:
					if errVal, ok := tc.wantErr.(error); ok && errors.Is(res.Err, errVal) {
						break
					}
					if !errors.As(res.Err, tc.wantErr) {
						t.Errorf("ConcurrentEval() error = %v, want %v", res.Err, tc.wantErr)
					}
				}
			}
		})
	}
}

// awaitEval runs ConcurrentEval and returns the result or fails on timeout.
func awaitEval(t *testing.T, prg cel.Program, ctx context.Context, in any) cel.EvalResult {
	t.Helper()
	select {
	case res := <-prg.ConcurrentEval(ctx, in):
		return res
	case <-time.After(5 * time.Second):
		t.Fatal("ConcurrentEval() timed out")
		return cel.EvalResult{}
	}
}

// mustProgram compiles an expression and constructs a Program, separating EnvOptions and ProgramOptions.
func mustProgram(t *testing.T, expr string, opts ...any) cel.Program {
	t.Helper()
	var envOpts []cel.EnvOption
	var prgOpts []cel.ProgramOption
	var deferredPrgOpts []func(int64) cel.ProgramOption
	for _, opt := range opts {
		switch o := opt.(type) {
		case cel.EnvOption:
			envOpts = append(envOpts, o)
		case cel.ProgramOption:
			prgOpts = append(prgOpts, o)
		case func(int64) cel.ProgramOption:
			deferredPrgOpts = append(deferredPrgOpts, o)
		default:
			t.Fatalf("unsupported option type %T", opt)
		}
	}
	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
	}
	rootID := ast.NativeRep().Expr().ID()
	for _, deferred := range deferredPrgOpts {
		prgOpts = append(prgOpts, deferred(rootID))
	}
	prg, err := env.Program(ast, prgOpts...)
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	return prg
}

type countingObserver struct {
	started  atomic.Int32
	finished atomic.Int32
}

func (o *countingObserver) OnCallStarted(callID int64, function, overload string, args []ref.Val) {
	o.started.Add(1)
}
func (o *countingObserver) OnCallFinished(callID int64, function, overload string, res ref.Val) {
	o.finished.Add(1)
}

type passCountingInterpretable struct {
	interpreter.InterpretableV2
	count *atomic.Int32
}

func (p *passCountingInterpretable) Exec(frame *interpreter.ExecutionFrame) ref.Val {
	p.count.Add(1)
	return p.InterpretableV2.Exec(frame)
}

func trackEvalPasses(count *atomic.Int32) func(int64) cel.ProgramOption {
	return func(rootID int64) cel.ProgramOption {
		return cel.CustomDecoratorV2(func(in interpreter.InterpretableV2) (interpreter.InterpretableV2, error) {
			if in.ID() == rootID {
				return &passCountingInterpretable{InterpretableV2: in, count: count}, nil
			}
			return in, nil
		})
	}
}
