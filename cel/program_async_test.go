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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/cel/async"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
	"github.com/google/cel-go/test"
)

func TestConcurrentEvalSync(t *testing.T) {
	prg := mustProgram(t, `x + 1`, cel.Variable("x", cel.IntType))
	res := awaitEval(t, prg, context.Background(), map[string]any{"x": 10})
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	if res.Val.Equal(types.Int(11)) != types.True {
		t.Errorf("ConcurrentEval() = %v, want 11", res.Val)
	}
}

func TestConcurrentEvalSingleAsync(t *testing.T) {
	prg := mustProgram(t, `async_func(42) + 1`,
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(10 * time.Millisecond)
					return args[0]
				}),
			),
		),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	if res.Val.Equal(types.Int(43)) != types.True {
		t.Errorf("ConcurrentEval() = %v, want 43", res.Val)
	}
}

// rpcEnv builds an env with an async string rpc() function that delays before echoing its input.
func rpcEnv(t *testing.T, opt cel.ProgramOption, binding async.BlockingOp) (cel.Program, error) {
	t.Helper()
	var opts []any
	opts = append(opts, cel.Function("rpc",
		cel.Overload("rpc_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.AsyncBinding(binding)),
	))
	if opt != nil {
		opts = append(opts, opt)
	}
	return mustProgram(t, `rpc("a") + rpc("b") + rpc("c")`, opts...), nil
}

func TestConcurrentEvalFanOutDrainStrategies(t *testing.T) {
	binding := func(ctx context.Context, args ...ref.Val) ref.Val {
		time.Sleep(10 * time.Millisecond)
		return args[0]
	}
	strategies := map[string]cel.ProgramOption{
		"default":     nil,
		"drain_all":   cel.ConcurrentDrainStrategy(async.DrainAll()),
		"drain_ready": cel.ConcurrentDrainStrategy(async.DrainReady(5 * time.Millisecond)),
		"drain_none":  cel.ConcurrentDrainStrategy(async.DrainNone()),
	}
	for name, opt := range strategies {
		t.Run(name, func(t *testing.T) {
			prg, err := rpcEnv(t, opt, binding)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			res := awaitEval(t, prg, context.Background(), cel.NoVars())
			if res.Err != nil {
				t.Fatalf("ConcurrentEval() error: %v", res.Err)
			}
			if res.Val.Equal(types.String("abc")) != types.True {
				t.Errorf("ConcurrentEval() = %v, want 'abc'", res.Val)
			}
		})
	}
}

func TestConcurrentEvalErrorPropagation(t *testing.T) {
	prg, err := rpcEnv(t, nil, func(ctx context.Context, args ...ref.Val) ref.Val {
		return types.NewErr("rpc failed")
	})
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err == nil {
		t.Fatalf("ConcurrentEval() expected error, got val %v", res.Val)
	}
}

func TestConcurrentEvalCancel(t *testing.T) {
	prg, err := rpcEnv(t, nil, func(ctx context.Context, args ...ref.Val) ref.Val {
		<-ctx.Done()
		return types.NewErr("cancelled")
	})
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	resCh := prg.ConcurrentEval(ctx, cel.NoVars())
	cancel()
	select {
	case res := <-resCh:
		if res.Err == nil {
			t.Errorf("ConcurrentEval() expected cancellation error, got val %v", res.Val)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ConcurrentEval() timed out after cancel")
	}
}

func TestConcurrentEvalNilContext(t *testing.T) {
	prg := mustProgram(t, `1 + 1`)
	res := <-prg.ConcurrentEval(nil, cel.NoVars())
	if res.Err == nil || !strings.Contains(res.Err.Error(), "context can not be nil") {
		t.Errorf("ConcurrentEval(nil) error: %v", res.Err)
	}
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

func TestConcurrentEvalObserver(t *testing.T) {
	obs := &countingObserver{}
	prg, err := rpcEnv(t, cel.AsyncCallObserver(obs), func(ctx context.Context, args ...ref.Val) ref.Val {
		time.Sleep(5 * time.Millisecond)
		return args[0]
	})
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	// Three distinct rpc calls in the expression.
	if got := obs.started.Load(); got != 3 {
		t.Errorf("OnCallStarted count = %d, want 3", got)
	}
	if got := obs.finished.Load(); got != 3 {
		t.Errorf("OnCallFinished count = %d, want 3", got)
	}
}

func TestConcurrentEvalEvalObserverTrackStateAndCost(t *testing.T) {
	callObs := &countingObserver{}
	prg := mustProgram(t, `async_func(42) + 1`,
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(5 * time.Millisecond)
					return args[0]
				}),
			),
		),
		cel.EvalOptions(cel.OptTrackState, cel.OptTrackCost),
		cel.AsyncCallObserver(callObs),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	if res.Val.Equal(types.Int(43)) != types.True {
		t.Errorf("ConcurrentEval() = %v, want 43", res.Val)
	}
	if got := callObs.started.Load(); got != 1 {
		t.Errorf("OnCallStarted count = %d, want 1", got)
	}
	if got := callObs.finished.Load(); got != 1 {
		t.Errorf("OnCallFinished count = %d, want 1", got)
	}
	if res.EvalDetails == nil {
		t.Fatal("res.EvalDetails is nil, want non-nil EvalDetails when evaluation observer is configured")
	}
	state := res.EvalDetails.State()
	if state == nil || len(state.IDs()) == 0 {
		t.Errorf("res.EvalDetails.State() = %v, want non-empty EvalState", state)
	}
	cost := res.EvalDetails.ActualCost()
	if cost == nil {
		t.Errorf("res.EvalDetails.ActualCost() = nil, want non-nil")
	}
}

func TestConcurrentEvalEvalObserverExhaustive(t *testing.T) {
	prg := mustProgram(t, `async_func(10) > 0`,
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					return args[0]
				}),
			),
		),
		cel.EvalOptions(cel.OptExhaustiveEval),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	if res.Val != types.True {
		t.Errorf("ConcurrentEval() = %v, want true", res.Val)
	}
	if res.EvalDetails == nil {
		t.Fatal("res.EvalDetails is nil, want non-nil EvalDetails")
	}
	if state := res.EvalDetails.State(); state == nil || len(state.IDs()) == 0 {
		t.Errorf("res.EvalDetails.State() = %v, want non-empty EvalState", state)
	}
}

func TestConcurrentEvalEvalObserverError(t *testing.T) {
	prg := mustProgram(t, `async_fail()`,
		cel.Function("async_fail",
			cel.Overload("async_fail_void", []*cel.Type{}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					return types.NewErr("async failure")
				}),
			),
		),
		cel.EvalOptions(cel.OptTrackState),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err == nil {
		t.Fatalf("ConcurrentEval() expected error, got nil")
	}
	if res.EvalDetails == nil {
		t.Fatal("res.EvalDetails is nil on error, want non-nil EvalDetails")
	}
	if state := res.EvalDetails.State(); state == nil {
		t.Errorf("res.EvalDetails.State() = nil, want non-nil EvalState")
	}
}

func TestConcurrentEvalAsyncCompletionBufferSize(t *testing.T) {
	prg := mustProgram(t, `async_func(42) + 1`,
		cel.Function("async_func",
			cel.Overload("async_func_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					time.Sleep(5 * time.Millisecond)
					return args[0]
				}),
			),
		),
		cel.AsyncCompletionBufferSize(16),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	if res.Val.Equal(types.Int(43)) != types.True {
		t.Errorf("ConcurrentEval() = %v, want 43", res.Val)
	}
}

func TestConcurrentEvalMaxConcurrency(t *testing.T) {
	cases := []struct {
		name         string
		maxConc      int
		wantMaxBound int // max expected observed concurrency, or 0 if unconstrained
	}{
		{
			name:         "bounded",
			maxConc:      1,
			wantMaxBound: 1,
		},
		{
			name:         "default_zero",
			maxConc:      0,
			wantMaxBound: 0,
		},
		{
			name:         "unlimited_negative",
			maxConc:      -1,
			wantMaxBound: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var concurrent atomic.Int32
			var maxConcurrent atomic.Int32
			binding := func(ctx context.Context, args ...ref.Val) ref.Val {
				cur := concurrent.Add(1)
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				concurrent.Add(-1)
				return args[0]
			}
			prg, err := rpcEnv(t, cel.AsyncMaxConcurrency(tc.maxConc), binding)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			res := awaitEval(t, prg, context.Background(), cel.NoVars())
			if res.Err != nil {
				t.Fatalf("ConcurrentEval() error: %v", res.Err)
			}
			if res.Val.Equal(types.String("abc")) != types.True {
				t.Errorf("ConcurrentEval() = %v, want 'abc'", res.Val)
			}
			if tc.wantMaxBound > 0 {
				if got := maxConcurrent.Load(); got > int32(tc.wantMaxBound) {
					t.Errorf("max observed concurrency = %d, want <= %d", got, tc.wantMaxBound)
				}
			}
		})
	}
}

func TestConcurrentEvalFakeRPC(t *testing.T) {
	prg, err := rpcEnv(t, nil, test.FakeRPC(time.Second))
	if err != nil {
		t.Fatalf("Program() failed: %v", err)
	}
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	want := types.String("a success!b success!c success!")
	if res.Val.Equal(want) != types.True {
		t.Errorf("ConcurrentEval() = %v, want %v", res.Val, want)
	}
}

func TestConcurrentEvalAsyncInComprehension(t *testing.T) {
	// Regression: an async call inside a comprehension is evaluated once per element with a
	// different loop-variable binding but the same AST node id. The evaluator must track each
	// element's call independently; otherwise re-evaluation relaunches every element forever and
	// never converges.
	var launches atomic.Int32
	prg := mustProgram(t, `[1, 2, 3].map(i, dbl(i))`,
		cel.Function("dbl",
			cel.Overload("dbl_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					launches.Add(1)
					time.Sleep(5 * time.Millisecond)
					return args[0].(types.Int) * 2
				}))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case res := <-prg.ConcurrentEval(ctx, cel.NoVars()):
		if res.Err != nil {
			t.Fatalf("ConcurrentEval() error: %v (launches=%d)", res.Err, launches.Load())
		}
		want := types.DefaultTypeAdapter.NativeToValue([]int64{2, 4, 6})
		if res.Val.Equal(want) != types.True {
			t.Errorf("ConcurrentEval() = %v, want [2, 4, 6]", res.Val)
		}
		if got := launches.Load(); got != 3 {
			t.Errorf("async launches = %d, want exactly 3 (one per element, no relaunch)", got)
		}
	case <-time.After(6 * time.Second):
		t.Fatalf("ConcurrentEval() did not converge; launches=%d", launches.Load())
	}
}

func TestConcurrentEvalBoundsLaunchConcurrency(t *testing.T) {
	// A wide async fan-out inside a comprehension must not launch more concurrent calls than the
	// configured limit, and must still converge to the correct result.
	var live, maxLive atomic.Int32
	const limit = 2
	prg := mustProgram(t, `[1, 2, 3, 4, 5, 6, 7, 8].map(i, rpc(i))`,
		cel.Function("rpc",
			cel.Overload("rpc_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					cur := live.Add(1)
					for {
						old := maxLive.Load()
						if cur <= old || maxLive.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					live.Add(-1)
					return args[0]
				}))),
		cel.AsyncMaxConcurrency(limit),
		cel.ConcurrentDrainStrategy(async.DrainAll()),
	)
	res := awaitEval(t, prg, context.Background(), cel.NoVars())
	if res.Err != nil {
		t.Fatalf("ConcurrentEval() error: %v", res.Err)
	}
	want := types.DefaultTypeAdapter.NativeToValue([]int64{1, 2, 3, 4, 5, 6, 7, 8})
	if res.Val.Equal(want) != types.True {
		t.Errorf("ConcurrentEval() = %v, want [1..8]", res.Val)
	}
	if got := maxLive.Load(); got > limit {
		t.Errorf("max concurrent async launches = %d, want <= %d", got, limit)
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
