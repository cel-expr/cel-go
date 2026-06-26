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

package interpreter

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/parser"
)

// asyncReturning returns an AsyncOp that immediately produces the given value, while counting the
// number of times the implementation is invoked.
func asyncReturning(val ref.Val, calls *atomic.Int32) functions.AsyncOp {
	return func(ctx context.Context, args ...ref.Val) <-chan ref.Val {
		if calls != nil {
			calls.Add(1)
		}
		ch := make(chan ref.Val, 1)
		ch <- val
		close(ch)
		return ch
	}
}

// newTestFrame creates an ExecutionFrame with an evaluation context attached.
func newTestFrame(t *testing.T, ctx context.Context) (*ExecutionFrame, func()) {
	t.Helper()
	frame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	if err := frame.SetContext(ctx, 0); err != nil {
		t.Fatalf("SetContext() failed: %v", err)
	}
	return frame, frame.Close
}

// awaitResult re-evaluates ComputeResult until it returns a non-Unknown value or the deadline fires.
func awaitResult(t *testing.T, frame *ExecutionFrame, completions <-chan int64, id int64, fn, overload string, impl functions.AsyncOp, args ...ref.Val) ref.Val {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		res := frame.ComputeResult(id, fn, overload, impl, args)
		if !types.IsUnknown(res) {
			return res
		}
		select {
		case <-completions:
		case <-deadline:
			t.Fatal("timed out waiting for async result")
		}
	}
}

func TestComputeResultWithoutContext(t *testing.T) {
	frame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	defer frame.Close()
	// No SetContext call, so async tracking is uninitialized.
	res := frame.ComputeResult(1, "fn", "fn_overload", asyncReturning(types.Int(1), nil), nil)
	if !types.IsError(res) {
		t.Errorf("ComputeResult() got %v, wanted error when tracking uninitialized", res)
	}
	if frame.ActiveAsyncCalls() != 0 {
		t.Errorf("ActiveAsyncCalls() = %d, wanted 0", frame.ActiveAsyncCalls())
	}
	if call := frame.AsyncCall(1); call != nil {
		t.Errorf("AsyncCall() = %v, wanted nil", call)
	}
}

func TestComputeResultResolves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	completions := make(chan int64, 1)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	var calls atomic.Int32
	impl := asyncReturning(types.Int(42), &calls)
	res := awaitResult(t, frame, completions, 1, "fn", "fn_int", impl, types.Int(1))
	if res.Equal(types.Int(42)) != types.True {
		t.Errorf("async result = %v, wanted 42", res)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("impl invoked %d times, wanted exactly 1", got)
	}
}

func TestTrackerDedupAndCallIDs(t *testing.T) {
	// Exercise the tracker directly to keep bookkeeping assertions isolated from the
	// process-global frame/tracker pools used by ExecutionFrame.
	tracker := newAsyncCallStateTracker()
	completions := make(chan int64, 4)
	gate := newAsyncGate(0, completions)
	impl := asyncReturning(types.Int(1), nil)

	// Same node id, same args, invoked repeatedly: must dedup to a single registered call.
	a1 := tracker.getOrCreate(1, "fn", "fn_int", []ref.Val{types.Int(1)}, impl, gate)
	a2 := tracker.getOrCreate(1, "fn", "fn_int", []ref.Val{types.Int(1)}, impl, gate)
	if a1 != a2 {
		t.Error("getOrCreate returned distinct states for identical (id, args)")
	}
	if a1.CallID() != a2.CallID() {
		t.Errorf("dedup callIDs differ: %d vs %d", a1.CallID(), a2.CallID())
	}
	if got := tracker.getByID(a1.CallID()); got != a1 {
		t.Errorf("getByID(%d) did not return the registered call", a1.CallID())
	}

	// Same node id, different args: a new call must be registered (re-evaluation with new inputs).
	a3 := tracker.getOrCreate(1, "fn", "fn_int", []ref.Val{types.Int(2)}, impl, gate)
	if a3.CallID() == a1.CallID() {
		t.Error("arg change reused the prior callID, wanted a fresh call")
	}
	if got := tracker.getByID(a3.CallID()); got != a3 {
		t.Errorf("getByID(%d) did not return the second registered call", a3.CallID())
	}
	// Registration alone does not mark a call as in-flight; pending counts launched calls only.
	if got := gate.ActiveCalls(); got != 0 {
		t.Errorf("ActiveCalls() after registration only = %d, wanted 0", got)
	}
}

func TestHashCall(t *testing.T) {
	type hashInput struct {
		id       int64
		overload string
		args     []ref.Val
	}
	tests := []struct {
		name     string
		input    hashInput
		wantHash uint64
	}{
		{
			name:     "node id 1 with string arg a",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.String("a")}},
			wantHash: 13175600815575489707,
		},
		{
			name:     "node id 1 with string arg b",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.String("b")}},
			wantHash: 13172731090226426672,
		},
		{
			name:     "node id 1 with bool arg true",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Bool(true)}},
			wantHash: 5368363472817480916,
		},
		{
			name:     "node id 1 with bool arg false",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Bool(false)}},
			wantHash: 5369320047933835261,
		},
		{
			name:     "node id 1 with nil args",
			input:    hashInput{1, overloads.ContainsString, nil},
			wantHash: 16571860343215109013,
		},
		{
			name:     "node id 2 with nil args",
			input:    hashInput{2, overloads.ContainsString, nil},
			wantHash: 7332562693386296450,
		},
		{
			name:     "node id 1 with overload matches_string",
			input:    hashInput{1, overloads.MatchesString, nil},
			wantHash: 6112375249444952169,
		},
		{
			name:     "node id 1 with overload starts_with_string",
			input:    hashInput{1, overloads.StartsWithString, nil},
			wantHash: 17791913581187873402,
		},
		{
			name:     "node id 1 with int arg 1",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Int(1)}},
			wantHash: 6250879603390619476,
		},
		{
			name:     "node id 1 with uint arg 1",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Uint(1)}},
			wantHash: 6250879603390619476,
		},
		{
			name:     "node id 1 with double arg 1.0",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Double(1.0)}},
			wantHash: 6250879603390619476,
		},
		{
			name:     "node id 1 with int arg 2",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Int(2)}},
			wantHash: 2269972419901218387,
		},
		{
			name:     "node id 1 with double arg 1.5",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Double(1.5)}},
			wantHash: 1181031487041842476,
		},
		{
			name:     "node id 1 with double arg 9.9",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Double(9.9)}},
			wantHash: 6763353195175059609,
		},
		{
			name:     "separation a, bc",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.String("a"), types.String("bc")}},
			wantHash: 16286284200968323077,
		},
		{
			name:     "separation ab, c",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.String("ab"), types.String("c")}},
			wantHash: 14974838604657976619,
		},
		{
			name:     "node id 1 with double arg NaN",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Double(math.NaN())}},
			wantHash: 4356663283625852078,
		},
		{
			name:     "node id 1 with string arg NaN",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.String("NaN")}},
			wantHash: 17422508148545277865,
		},
		{
			name:     "node id 1 with double arg 0.0",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Double(0.0)}},
			wantHash: 2331193227347896467,
		},
		{
			name:     "node id 1 with double arg -0.0",
			input:    hashInput{1, overloads.ContainsString, []ref.Val{types.Double(math.Copysign(0.0, -1.0))}},
			wantHash: 2331193227347896467,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := hashCall(tc.input.id, tc.input.overload, tc.input.args)
			if got != tc.wantHash {
				t.Errorf("hashCall() = %d, wanted %d", got, tc.wantHash)
			}
		})
	}
}

func TestTrackerComprehensionReuse(t *testing.T) {
	// Regression: a single AST node id evaluated repeatedly with distinct argument values (as
	// happens for an async call inside a comprehension) must register one call per distinct
	// argument set, and re-lookups must return the existing state rather than relaunching. A
	// node-id-keyed map collapses these into a single slot, causing every re-evaluation pass to
	// relaunch every iteration and never converge.
	tracker := newAsyncCallStateTracker()
	completions := make(chan int64, 8)
	gate := newAsyncGate(0, completions)
	impl := asyncReturning(types.Int(0), nil)
	const id = int64(1)

	args := [][]ref.Val{
		{types.Int(1)},
		{types.Int(2)},
		{types.Int(3)},
	}
	states := make([]*asyncCallState, len(args))
	for i, a := range args {
		states[i] = tracker.getOrCreate(id, "fn", "fn_int", a, impl, gate)
	}

	// Each distinct argument set is a distinct, uniquely-identified call.
	seen := map[int64]bool{}
	for _, s := range states {
		if seen[s.CallID()] {
			t.Errorf("duplicate callID %d across distinct args", s.CallID())
		}
		seen[s.CallID()] = true
	}

	// Re-evaluation: every prior (id, args) tuple must resolve to its existing state and must
	// not register a new call.
	for i, a := range args {
		if got := tracker.getOrCreate(id, "fn", "fn_int", a, impl, gate); got != states[i] {
			t.Errorf("re-lookup of args %v returned a new state, wanted the existing one", a)
		}
	}

	// Identical string arguments at the same node dedup to a single call.
	s1 := tracker.getOrCreate(2, "fn", "fn_str", []ref.Val{types.String("k")}, impl, gate)
	s2 := tracker.getOrCreate(2, "fn", "fn_str", []ref.Val{types.String("k")}, impl, gate)
	if s1 != s2 {
		t.Error("identical string args did not dedup to a single call")
	}
}

func TestTrackerRegistrationLookup(t *testing.T) {
	tracker := newAsyncCallStateTracker()
	completions := make(chan int64, 2)
	gate := newAsyncGate(0, completions)
	impl := asyncReturning(types.Int(1), nil)

	a := tracker.getOrCreate(1, "fn", "a", []ref.Val{types.Int(1)}, impl, gate)
	b := tracker.getOrCreate(2, "fn", "b", []ref.Val{types.Int(2)}, impl, gate)

	// Registered calls must be retrievable by callID.
	for _, want := range []*asyncCallState{a, b} {
		if got := tracker.getByID(want.CallID()); got != want {
			t.Errorf("getByID(%d) returned a different call record", want.CallID())
		}
	}
	// Unknown callIDs resolve to nil rather than panicking.
	if got := tracker.getByID(99999); got != nil {
		t.Errorf("getByID(unknown) = %v, wanted nil", got)
	}
}

func TestAsyncObserverLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	obs := &recordingObserver{}
	if err := frame.SetAsyncObserver(obs); err != nil {
		t.Fatalf("SetAsyncObserver() failed: %v", err)
	}
	completions := make(chan int64, 1)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	awaitResult(t, frame, completions, 1, "fn", "fn_int", asyncReturning(types.Int(7), nil), types.Int(1))

	if got := obs.started.Load(); got != 1 {
		t.Errorf("OnCallStarted called %d times, wanted 1", got)
	}
	if got := obs.finished.Load(); got != 1 {
		t.Errorf("OnCallFinished called %d times, wanted 1", got)
	}
}

// asyncControllable returns a channel-based AsyncOp whose calls block until release is closed,
// tracking the number of concurrently live calls and the high-water mark.
func asyncControllable(release <-chan struct{}, live, maxLive *atomic.Int32) functions.AsyncOp {
	return func(ctx context.Context, args ...ref.Val) <-chan ref.Val {
		ch := make(chan ref.Val, 1)
		go func() {
			cur := live.Add(1)
			for {
				old := maxLive.Load()
				if cur <= old || maxLive.CompareAndSwap(old, cur) {
					break
				}
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			live.Add(-1)
			ch <- args[0]
			close(ch)
		}()
		return ch
	}
}

func TestLaunchAdmissionAndBounding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	const limit = 2
	const total = 5
	if err := frame.SetAsyncMaxConcurrency(limit); err != nil {
		t.Fatalf("SetAsyncMaxConcurrency() failed: %v", err)
	}
	completions := make(chan int64, total*2)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	release := make(chan struct{})
	var live, maxLive atomic.Int32
	impl := asyncControllable(release, &live, &maxLive)

	// A single evaluation pass: attempt to launch all calls (distinct node ids).
	pass := func() {
		for i := range total {
			frame.ComputeResult(int64(i+1), "fn", "fn_int", impl, []ref.Val{types.Int(int64(i))})
		}
	}

	// First pass admits only `limit` launches; the rest are deferred. pending is updated
	// synchronously as calls are admitted, so it must equal the limit immediately.
	pass()
	if got := frame.ActiveAsyncCalls(); got != limit {
		t.Fatalf("ActiveAsyncCalls() after first pass = %d, wanted %d", got, limit)
	}

	// Drive completions and re-evaluate until every call has resolved, simulating ConcurrentEval.
	close(release)
	done := map[int64]bool{}
	deadline := time.After(5 * time.Second)
	for len(done) < total {
		pass() // re-evaluate: launch any newly-admissible calls
		select {
		case id := <-completions:
			done[id] = true
		case <-deadline:
			t.Fatalf("only %d/%d calls completed; maxLive=%d", len(done), total, maxLive.Load())
		}
	}

	if got := maxLive.Load(); got > limit {
		t.Errorf("max concurrent launches = %d, wanted <= %d", got, limit)
	}
	if got := frame.ActiveAsyncCalls(); got != 0 {
		t.Errorf("ActiveAsyncCalls() after all completions = %d, wanted 0", got)
	}
}

func TestLaunchUnlimitedWhenNoSemaphore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()
	// No SetAsyncMaxConcurrency -> nil semaphore -> all calls admitted in one pass.
	completions := make(chan int64, 8)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	release := make(chan struct{})
	defer close(release)
	var live, maxLive atomic.Int32
	impl := asyncControllable(release, &live, &maxLive)

	const total = 5
	for i := 0; i < total; i++ {
		frame.ComputeResult(int64(i+1), "fn", "fn_int", impl, []ref.Val{types.Int(int64(i))})
	}
	if got := frame.ActiveAsyncCalls(); got != total {
		t.Errorf("ActiveAsyncCalls() with no limit = %d, wanted %d", got, total)
	}
}

func TestAsyncCallStateCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	completions := make(chan int64, 1)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	var exited sync.WaitGroup
	exited.Add(1)
	blocking := func(ctx context.Context, args ...ref.Val) <-chan ref.Val {
		ch := make(chan ref.Val) // never written; goroutine must exit via ctx.Done
		go func() {
			defer exited.Done()
			<-ctx.Done()
		}()
		return ch
	}
	res := frame.ComputeResult(1, "fn", "fn_int", blocking, []ref.Val{types.Int(1)})
	if !types.IsUnknown(res) {
		t.Fatalf("ComputeResult() = %v, wanted Unknown while pending", res)
	}
	cancel()

	done := make(chan struct{})
	go func() { exited.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async impl goroutine did not exit on cancellation")
	}
	// No completion should have been delivered for the cancelled call.
	select {
	case callID := <-completions:
		t.Errorf("unexpected completion for cancelled call: %d", callID)
	default:
	}
}

func TestAsyncTrackerPoolReleaseClearsState(t *testing.T) {
	tracker := newAsyncCallStateTracker()
	completions := make(chan int64, 4)
	gate := newAsyncGate(0, completions)
	for i := int64(1); i <= 3; i++ {
		tracker.getOrCreate(i, "fn", "ov", []ref.Val{types.Int(i)}, asyncReturning(types.Int(i), nil), gate)
	}
	if tracker.getByID(2) == nil {
		t.Fatal("getByID(2) = nil before release, wanted a call record")
	}

	asyncCallStateTrackerPool.release(tracker)

	if got := len(tracker.calls); got != 0 {
		t.Errorf("calls map size after release = %d, wanted 0", got)
	}
	if got := len(tracker.callsByID); got != 0 {
		t.Errorf("callsByID map size after release = %d, wanted 0", got)
	}
	if got := tracker.nextCallID.Load(); got != 0 {
		t.Errorf("nextCallID after release = %d, wanted 0", got)
	}
	// A released tracker is safe to reuse: callIDs restart and bookkeeping is fresh.
	acs := tracker.getOrCreate(1, "fn", "ov", []ref.Val{types.Int(1)}, asyncReturning(types.Int(1), nil), gate)
	if acs.CallID() != 1 {
		t.Errorf("reused tracker assigned callID %d, wanted 1", acs.CallID())
	}
}

func TestAsyncCallStateEquals(t *testing.T) {
	mk := func(fn, ov string, args ...ref.Val) *asyncCallState {
		return newAsyncCallState(1, fn, ov, args, nil)
	}
	base := mk("fn", "ov", types.Int(1), types.String("a"))
	tests := []struct {
		name  string
		other *asyncCallState
		want  bool
	}{
		{"identical", mk("fn", "ov", types.Int(1), types.String("a")), true},
		{"diff function", mk("other", "ov", types.Int(1), types.String("a")), false},
		{"diff overload", mk("fn", "other", types.Int(1), types.String("a")), false},
		{"diff arg value", mk("fn", "ov", types.Int(2), types.String("a")), false},
		{"diff arity", mk("fn", "ov", types.Int(1)), false},
		{"nil other", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.equals(tc.other); got != tc.want {
				t.Errorf("equals() = %v, wanted %v", got, tc.want)
			}
		})
	}
	t.Run("nil receiver", func(t *testing.T) {
		var nilState *asyncCallState
		if nilState.equals(base) {
			t.Error("nil.equals() returned true")
		}
	})
	t.Run("NaN equivalence", func(t *testing.T) {
		s1 := mk("fn", "ov", types.Double(math.NaN()))
		s2 := mk("fn", "ov", types.Double(math.NaN()))
		s3 := mk("fn", "ov", types.Double(1.23))

		tests := []struct {
			name  string
			state *asyncCallState
			other *asyncCallState
			want  bool
		}{
			{"NaN vs NaN", s1, s2, true},
			{"NaN vs non-NaN", s1, s3, false},
			{"non-NaN vs NaN", s3, s1, false},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				if got := tc.state.equals(tc.other); got != tc.want {
					t.Errorf("equals() = %v, wanted %v", got, tc.want)
				}
			})
		}
	})
}

func TestExecutionFrameChildSharesAsyncContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	completions := make(chan int64, 1)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	// Counts are taken relative to a baseline, since ExecutionFrame draws its tracker from a
	// process-global pool whose starting pending count is not guaranteed to be zero.
	base := frame.ActiveAsyncCalls()

	child := frame.Push(EmptyActivation())
	if got := child.ActiveAsyncCalls(); got != base {
		t.Errorf("child ActiveAsyncCalls() = %d, wanted %d (shared parent ctx)", got, base)
	}

	// A call launched from the child frame must be visible through the shared parent tracker.
	child.ComputeResult(1, "fn", "fn_int", asyncReturning(types.Int(5), nil), []ref.Val{types.Int(1)})
	if got := child.ActiveAsyncCalls(); got != base+1 {
		t.Errorf("child ActiveAsyncCalls() after launch = %d, wanted %d", got, base+1)
	}

	parent := child.Pop()
	if parent != frame {
		t.Fatalf("Pop() did not return the parent frame")
	}
	if got := frame.ActiveAsyncCalls(); got != base+1 {
		t.Errorf("parent ActiveAsyncCalls() after Pop = %d, wanted %d", got, base+1)
	}
	// The launched call is retrievable via the parent frame's AsyncCall accessor.
	select {
	case callID := <-completions:
		if call := frame.AsyncCall(callID); call == nil {
			t.Errorf("AsyncCall(%d) = nil, wanted a call record from the shared tracker", callID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for child call completion")
	}
}

type recordingObserver struct {
	started  atomic.Int32
	finished atomic.Int32
}

func (o *recordingObserver) OnCallStarted(callID int64, function, overload string, args []ref.Val) {
	o.started.Add(1)
}

func (o *recordingObserver) OnCallFinished(callID int64, function, overload string, res ref.Val) {
	o.finished.Add(1)
}

func TestEvalAsyncFuncGetters(t *testing.T) {
	fn := &evalAsyncFunc{
		id:       42,
		function: "async_fn",
		overload: "async_fn_overload",
		args: []InterpretableV2{
			&evalConst{val: types.Int(1)},
		},
		impl: asyncReturning(types.Int(100), nil),
	}

	if fn.ID() != 42 {
		t.Errorf("ID() = %d, wanted 42", fn.ID())
	}
	if fn.Function() != "async_fn" {
		t.Errorf("Function() = %s, wanted async_fn", fn.Function())
	}
	if fn.OverloadID() != "async_fn_overload" {
		t.Errorf("OverloadID() = %s, wanted async_fn_overload", fn.OverloadID())
	}
	if len(fn.Args()) != 1 {
		t.Errorf("Args() length = %d, wanted 1", len(fn.Args()))
	}
}

func TestEvalAsyncFuncLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	completions := make(chan int64, 1)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	impl := asyncReturning(types.Int(100), nil)
	fn := &evalAsyncFunc{
		id:       42,
		function: "async_fn",
		overload: "async_fn_overload",
		args: []InterpretableV2{
			&evalConst{val: types.Int(1)},
		},
		impl: impl,
	}

	// Test Exec first pass
	res := fn.Exec(frame)
	if !types.IsUnknown(res) {
		t.Errorf("Exec() first call got %v, wanted Unknown", res)
	}

	// Now await completion and re-evaluate
	select {
	case <-completions:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion")
	}

	res2 := fn.Exec(frame)
	if res2.Equal(types.Int(100)) != types.True {
		t.Errorf("Exec() second call got %v, wanted 100", res2)
	}

	// Test Eval method
	res3 := fn.Eval(frame)
	if res3.Equal(types.Int(100)) != types.True {
		t.Errorf("Eval() got %v, wanted 100", res3)
	}
}

func TestEvalAsyncFuncEarlyReturn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, closeFrame := newTestFrame(t, ctx)
	defer closeFrame()

	impl := asyncReturning(types.Int(100), nil)

	// Test Exec returning early when an argument is error
	fnErrArg := &evalAsyncFunc{
		id:       43,
		function: "async_fn",
		overload: "async_fn_overload",
		args: []InterpretableV2{
			&evalConst{val: types.NewErr("argument error")},
		},
		impl: impl,
	}
	resErr := fnErrArg.Exec(frame)
	if !types.IsError(resErr) {
		t.Errorf("Exec with error arg got %v, wanted error", resErr)
	}

	// Test Exec returning early when an argument is unknown
	fnUnknownArg := &evalAsyncFunc{
		id:       44,
		function: "async_fn",
		overload: "async_fn_overload",
		args: []InterpretableV2{
			&evalConst{val: types.NewUnknown(999, nil)},
		},
		impl: impl,
	}
	resUnknown := fnUnknownArg.Exec(frame)
	if !types.IsUnknown(resUnknown) {
		t.Errorf("Exec with unknown arg got %v, wanted Unknown", resUnknown)
	}
}

func TestTrackerPoolShrink(t *testing.T) {
	pool := newAsyncCallTrackerPool()
	tracker := pool.create()
	completions := make(chan int64, 1)
	gate := newAsyncGate(0, completions)
	impl := asyncReturning(types.Int(1), nil)

	// Register more than trackerShrinkThreshold calls to trigger map reallocation
	for i := int64(1); i <= trackerShrinkThreshold+10; i++ {
		tracker.getOrCreate(i, "fn", "fn_int", []ref.Val{types.Int(i)}, impl, gate)
	}

	// Release the tracker; since size exceeds threshold, maps should be reallocated
	pool.release(tracker)

	// Retrieve again from pool to make sure it's clean and empty
	tracker2 := pool.create()
	if len(tracker2.calls) != 0 || len(tracker2.callsByID) != 0 {
		t.Errorf("released tracker was not cleared: calls size = %d, callsByID size = %d", len(tracker2.calls), len(tracker2.callsByID))
	}

	// Now register a small number of calls, release, and check that delete path works
	tracker2.getOrCreate(1, "fn", "fn_int", []ref.Val{types.Int(1)}, impl, gate)
	pool.release(tracker2)

	tracker3 := pool.create()
	if len(tracker3.calls) != 0 || len(tracker3.callsByID) != 0 {
		t.Errorf("released tracker (delete path) was not cleared: calls size = %d, callsByID size = %d", len(tracker3.calls), len(tracker3.callsByID))
	}
	pool.release(tracker3)
}

func TestAsyncWithTraceAndExhaustiveEval(t *testing.T) {
	asyncFnDecl, err := decls.NewFunction("async_eq_true",
		decls.Overload("async_eq_true_int",
			[]*types.Type{types.IntType},
			types.BoolType,
			decls.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
				val := args[0].(types.Int)
				select {
				case <-time.After(50 * time.Millisecond):
					return types.Bool(val == 1)
				case <-ctx.Done():
					return types.NewErr("cancelled")
				}
			}),
		),
	)
	if err != nil {
		t.Fatalf("NewFunction failed: %v", err)
	}

	exprStr := "async_eq_true(1) && async_eq_true(2)"
	s := common.NewTextSource(exprStr)
	p, err := parser.NewParser(
		parser.Macros(parser.AllMacros...),
	)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	parsed, errs := p.Parse(s)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Parse errors: %s", errs.ToDisplayString())
	}

	reg := newTestRegistry(t)
	env := newTestEnv(t, containers.DefaultContainer, reg)
	err = env.AddFunctions(asyncFnDecl)
	if err != nil {
		t.Fatalf("AddFunctions failed: %v", err)
	}
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Check errors: %s", errs.ToDisplayString())
	}

	disp := NewDispatcher()
	addFunctionBindings(t, disp)
	bindings, err := asyncFnDecl.Bindings()
	if err != nil {
		t.Fatalf("Bindings() failed: %v", err)
	}
	err = disp.Add(bindings...)
	if err != nil {
		t.Fatalf("dispatcher.Add() failed: %v", err)
	}

	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	interp := NewInterpreter(disp, containers.DefaultContainer, reg, reg, attrs)

	state := NewEvalState()
	prg, err := interp.NewInterpretable(checked,
		ExhaustiveEval(),
		EvalStateObserver(EvalStateFactory(func() EvalState { return state })),
	)
	if err != nil {
		t.Fatalf("NewInterpretable failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame failed: %v", err)
	}
	defer frame.Close()
	if err := frame.SetContext(ctx, 0); err != nil {
		t.Fatalf("SetContext failed: %v", err)
	}

	completions := make(chan int64, 4)
	if err := frame.SetCompletions(completions); err != nil {
		t.Fatalf("SetCompletions() failed: %v", err)
	}

	oi, isObservable := prg.(*ObservableInterpretable)
	if !isObservable {
		t.Fatalf("expected program to be observable")
	}

	runPass := func() ref.Val {
		return oi.ObserveExec(frame, func(observed any) {
		})
	}

	var res ref.Val
	deadline := time.After(2 * time.Second)
loop:
	for {
		res = runPass()
		if !types.IsUnknown(res) && frame.ActiveAsyncCalls() == 0 {
			break loop
		}
		select {
		case <-completions:
		case <-deadline:
			t.Fatal("timed out waiting for async result")
		}
	}

	if res.Equal(types.False) != types.True {
		t.Errorf("expected false, got %v", res)
	}

	state.Reset()
	resLast := runPass()
	if resLast.Equal(types.False) != types.True {
		t.Errorf("expected final pass result to be false, got %v", resLast)
	}

	for _, id := range state.IDs() {
		val, found := state.Value(id)
		if !found {
			continue
		}
		if types.IsUnknown(val) {
			t.Errorf("found unknown value in final eval state for node %d: %v", id, val)
		}
	}
}

func TestAsyncSetupWithoutContextErrors(t *testing.T) {
	frame, err := NewExecutionFrame(EmptyActivation())
	if err != nil {
		t.Fatalf("NewExecutionFrame() failed: %v", err)
	}
	defer frame.Close()

	// No frame.SetContext called, so context is nil.
	completions := make(chan int64, 1)
	if err := frame.SetCompletions(completions); err == nil {
		t.Error("SetCompletions() succeeded without context, wanted error")
	}
	obs := &recordingObserver{}
	if err := frame.SetAsyncObserver(obs); err == nil {
		t.Error("SetAsyncObserver() succeeded without context, wanted error")
	}
	if err := frame.SetAsyncMaxConcurrency(2); err == nil {
		t.Error("SetAsyncMaxConcurrency() succeeded without context, wanted error")
	}
}
