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

package async_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"cel.dev/cel-go/cel/async"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

type retryableTestErr struct{}

func (retryableTestErr) Error() string     { return "retry me" }
func (retryableTestErr) IsRetryable() bool { return true }

type nonRetryableTestErr struct{}

func (nonRetryableTestErr) Error() string     { return "do not retry me" }
func (nonRetryableTestErr) IsRetryable() bool { return false }

// buildZeroArgAsync builds a zero-arity async overload from an option and returns its AsyncOp.
func buildZeroArgAsync(t *testing.T, opt decls.OverloadOpt) functions.AsyncOp {
	t.Helper()
	fnDecl, err := decls.NewFunction("fn", decls.Overload("fn_zero", []*types.Type{}, types.IntType, opt))
	if err != nil {
		t.Fatalf("NewFunction() failed: %v", err)
	}
	bindings, err := fnDecl.Bindings()
	if err != nil {
		t.Fatalf("Bindings() failed: %v", err)
	}
	for _, b := range bindings {
		if b.Async != nil {
			return b.Async
		}
	}
	t.Fatal("no async binding produced")
	return nil
}

func TestRetryMultipleAttemptsTimerReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32
		op := buildZeroArgAsync(t, async.RetryBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
			a := attempts.Add(1)
			if a < 3 {
				return types.WrapErr(retryableTestErr{})
			}
			return types.Int(100)
		}, async.RetryAttempts(4), async.RetryBackoff(5*time.Millisecond)))

		start := time.Now()
		res := <-op(t.Context())
		if res.Equal(types.Int(100)) != types.True {
			t.Fatalf("result = %v, want 100", res)
		}
		if got := attempts.Load(); got != 3 {
			t.Errorf("attempts = %d, want 3", got)
		}
		// Two retries, each preceded by one backoff from the reused timer.
		if elapsed := time.Since(start); elapsed != 10*time.Millisecond {
			t.Errorf("elapsed = %v, want exactly two 5ms backoffs", elapsed)
		}
	})
}

func TestRetryMaxAttemptsExhausted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32
		op := buildZeroArgAsync(t, async.RetryBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
			a := attempts.Add(1)
			return types.WrapErr(fmt.Errorf("retry attempt %d failed: %w", a, retryableTestErr{}))
		}, async.RetryAttempts(3), async.RetryBackoff(5*time.Millisecond)))

		res := <-op(t.Context())
		if !types.IsError(res) {
			t.Fatalf("result = %v, want error", res)
		}
		msg := res.(*types.Err).Error()
		if !strings.Contains(msg, "retry attempt 3 failed") {
			t.Errorf("result error = %q, want last error 'retry attempt 3 failed'", msg)
		}
		if got := attempts.Load(); got != 3 {
			t.Errorf("attempts = %d, want 3", got)
		}
	})
}

func TestRetryBindingCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32
		op := buildZeroArgAsync(t, async.RetryBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
			attempts.Add(1)
			return types.WrapErr(retryableTestErr{})
		}, async.RetryAttempts(5), async.RetryBackoff(500*time.Millisecond)))

		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			time.Sleep(40 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		res := <-op(ctx)
		elapsed := time.Since(start)

		if !types.IsError(res) || !strings.Contains(res.(*types.Err).Error(), "cancelled") {
			t.Errorf("result = %v, want a cancellation error", res)
		}
		// Cancellation must interrupt the 500ms backoff wait at the moment of cancellation rather
		// than running it to completion.
		if elapsed != 40*time.Millisecond {
			t.Errorf("retry returned after %v, want exactly 40ms when cancellation interrupts the backoff", elapsed)
		}
	})
}

func TestRetryNonRetryableError(t *testing.T) {
	var attempts atomic.Int32
	op := buildZeroArgAsync(t, async.RetryBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
		attempts.Add(1)
		return types.WrapErr(nonRetryableTestErr{})
	}, async.RetryAttempts(5), async.RetryBackoff(10*time.Millisecond)))

	res := <-op(context.Background())
	if !types.IsError(res) || !strings.Contains(res.(*types.Err).Error(), "do not retry me") {
		t.Fatalf("result = %v, want 'do not retry me'", res)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (non-retryable error should not be retried)", got)
	}
}

func TestRetryStandardError(t *testing.T) {
	var attempts atomic.Int32
	op := buildZeroArgAsync(t, async.RetryBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
		attempts.Add(1)
		return types.NewErr("generic error")
	}, async.RetryAttempts(5), async.RetryBackoff(10*time.Millisecond)))

	res := <-op(context.Background())
	if !types.IsError(res) || !strings.Contains(res.(*types.Err).Error(), "generic error") {
		t.Fatalf("result = %v, want 'generic error'", res)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (standard error should not be retried)", got)
	}
}

type mockAsyncCall struct {
	id       int64
	function string
	overload string
}

func (m mockAsyncCall) CallID() int64    { return m.id }
func (m mockAsyncCall) Function() string { return m.function }
func (m mockAsyncCall) Overload() string { return m.overload }

func TestAsyncCallMethods(t *testing.T) {
	m := mockAsyncCall{id: 1, function: "f", overload: "o"}
	if m.CallID() != 1 {
		t.Errorf("got %d, want 1", m.CallID())
	}
	if m.Function() != "f" {
		t.Errorf("got %s, want f", m.Function())
	}
	if m.Overload() != "o" {
		t.Errorf("got %s, want o", m.Overload())
	}
}

func TestDrainNone(t *testing.T) {
	s := async.DrainNone()
	// No completions, calls still pending -> no re-evaluation
	if s.NextAction(nil, 1).Reevaluate {
		t.Error("DrainNone re-evaluated with nil batch")
	}
	// One completion -> re-evaluate
	if !s.NextAction([]async.Call{mockAsyncCall{}}, 1).Reevaluate {
		t.Error("DrainNone did not re-evaluate with 1 completion")
	}
	// No pending -> re-evaluate regardless of batch
	if !s.NextAction(nil, 0).Reevaluate {
		t.Error("DrainNone did not re-evaluate when nothing pending")
	}
}

func TestDrainAll(t *testing.T) {
	s := async.DrainAll()
	// Pending calls remain -> no re-evaluation
	if s.NextAction([]async.Call{mockAsyncCall{}}, 1).Reevaluate {
		t.Error("DrainAll re-evaluated while calls are pending")
	}
	// No pending calls -> re-evaluate
	if !s.NextAction([]async.Call{mockAsyncCall{}}, 0).Reevaluate {
		t.Error("DrainAll did not re-evaluate when no calls pending")
	}
}

func TestDrainReady(t *testing.T) {
	debounce := 10 * time.Millisecond
	s := async.DrainReady(debounce)

	// No pending calls -> re-evaluate immediately
	action := s.NextAction([]async.Call{mockAsyncCall{}}, 0)
	if !action.Reevaluate {
		t.Error("DrainReady did not re-evaluate when no calls pending")
	}

	// No completions -> wait indefinitely
	action = s.NextAction(nil, 1)
	if action.Reevaluate || action.WaitDuration != 0 {
		t.Errorf("DrainReady NextAction(nil, 1) = %v, want {false, 0}", action)
	}

	// Some completions, still pending -> wait for the debounce window
	action = s.NextAction([]async.Call{mockAsyncCall{}}, 1)
	if action.Reevaluate || action.WaitDuration != debounce {
		t.Errorf("DrainReady NextAction(batch, 1) = %v, want {false, %v}", action, debounce)
	}
}

func TestTimeoutBindingEnforcesAgainstContextIgnoringOp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// The wrapped op sleeps well past the timeout without consulting its context. TimeoutBinding
		// must still return at the deadline rather than waiting for the op to finish.
		op := buildZeroArgAsync(t, async.TimeoutBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
			time.Sleep(2 * time.Second) // ignores ctx entirely
			return types.Int(1)
		}, 50*time.Millisecond))

		start := time.Now()
		res := <-op(t.Context())
		elapsed := time.Since(start)

		if !types.IsError(res) || !strings.Contains(res.(*types.Err).Error(), "timed out") {
			t.Errorf("result = %v, want a timeout error", res)
		}
		if elapsed != 50*time.Millisecond {
			t.Errorf("TimeoutBinding returned after %v, want exactly the 50ms timeout", elapsed)
		}

		// The abandoned op keeps running until its own sleep ends. Advance the fake clock past it
		// so the goroutine finishes; its result lands in Timeout's buffered channel and is
		// discarded, so it does not leak. synctest.Test fails if any goroutine is left blocked.
		time.Sleep(2 * time.Second)
	})
}
