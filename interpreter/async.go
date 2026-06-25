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
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"sync/atomic"

	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Async extension function support.
//
// CEL supports `types.Unknown` as a first-class value, and concurrent (async) function execution
// in CEL invokes a stub function which checks for the presence of an existing result which matches
// the function call and call arguments, or which records the 'unexecuted' function and call arguments
// for concurrent execution in a later phase if the result is `types.Unknown` and indicates the
// expression ids of the functions necessary to advance the execution.
//
// This call pattern is repeated iteratively until there are either no more functions to call or no
// progress is made toward resolving the unknowns.

// AsyncObserver provides callbacks for monitoring the lifecycle of asynchronous function calls.
//
// Implementations must be safe for concurrent use: OnCallStarted is invoked from the evaluator
// goroutine when a call is launched, while OnCallFinished is invoked from the call's own goroutine
// when it completes. The two callbacks therefore run on different goroutines, and OnCallFinished
// callbacks for distinct calls may run concurrently with each other.
type AsyncObserver interface {
	// OnCallStarted is called when an asynchronous function is first launched.
	OnCallStarted(callID int64, function, overload string, args []ref.Val)
	// OnCallFinished is called when an asynchronous function completes.
	OnCallFinished(callID int64, function, overload string, res ref.Val)
}

// AsyncCall describes a pending or completed asynchronous function call.
type AsyncCall interface {
	// CallID returns the unique identifier for this async call invocation.
	CallID() int64
	// Function returns the name of the function being called.
	Function() string
	// Overload returns the specific overload ID being invoked.
	Overload() string
}

// evalAsyncFunc is the planned Interpretable for an asynchronous function call.
type evalAsyncFunc struct {
	id       int64
	function string
	overload string
	args     []InterpretableV2
	impl     functions.AsyncOp
}

// ID implements the Interpretable interface method.
func (fn *evalAsyncFunc) ID() int64 {
	return fn.id
}

// Function returns the name of the function being invoked.
func (fn *evalAsyncFunc) Function() string {
	return fn.function
}

// OverloadID returns the overload id of the function being invoked.
func (fn *evalAsyncFunc) OverloadID() string {
	return fn.overload
}

// Args returns the argument Interpretables for the function call.
func (fn *evalAsyncFunc) Args() []InterpretableV2 {
	return fn.args
}

// Eval implements the Interpretable interface method.
func (fn *evalAsyncFunc) Eval(vars Activation) ref.Val {
	return fn.Exec(AsFrame(vars))
}

// Exec implements the InterpretableV2 interface method.
func (fn *evalAsyncFunc) Exec(frame *ExecutionFrame) ref.Val {
	argVals := make([]ref.Val, len(fn.args))
	// Early return if any argument to the function is unknown or error.
	for i, arg := range fn.args {
		argVals[i] = arg.Exec(frame)
		// TODO: early return only on errors, aggregate unknowns and validate
		// whether any argument is unknown before proceeding with the call.
		if types.IsUnknownOrError(argVals[i]) {
			return argVals[i]
		}
	}
	result := frame.ComputeResult(fn.ID(), fn.Function(), fn.OverloadID(), fn.impl, argVals)
	return types.LabelErrNode(fn.id, result)
}

// asyncCallStateTracker manages async call states across re-evaluations of a single program.
type asyncCallStateTracker struct {
	mu sync.RWMutex
	// calls buckets call states by a composite hash of (node id, overload, string/int/double/uint/bool args).
	// A single AST node id may host many concurrently-live calls when it is evaluated inside a
	// comprehension (once per element with different arguments), so each bucket may hold more
	// than one state. The exact match within a bucket is resolved via asyncCallState.equals,
	// which applies CEL's full equality semantics to the arguments.
	calls        map[uint64][]*asyncCallState
	callsByID    map[int64]*asyncCallState
	nextCallID   atomic.Int64
	pendingCalls atomic.Int32
}

func newAsyncCallStateTracker() *asyncCallStateTracker {
	return &asyncCallStateTracker{
		calls:     make(map[uint64][]*asyncCallState),
		callsByID: make(map[int64]*asyncCallState),
	}
}

var (
	hashZeroMarker      = []byte{0}
	hashStringMarker    = []byte{'s'}
	hashBoolTrueMarker  = []byte{'b', 1}
	hashBoolFalseMarker = []byte{'b', 0}
	hashNumberMarker    = []byte{'n'}
	hashDefaultMarker   = []byte{'x'}
)

// hashCall computes the composite bucket key for an async call.
//
// Only string, int, double, uint, and bool argument values contribute to the hash. More complex types
// rely on a richer notion of equivalence (e.g. unordered maps, proto equality, custom types)
// that a byte-level hash cannot capture safely, so they are intentionally excluded from the key
// and are instead disambiguated within the bucket by asyncCallState.equals.
func hashCall(id int64, overload string, args []ref.Val) uint64 {
	h := fnv.New64a()
	var idBuf [8]byte
	binary.LittleEndian.PutUint64(idBuf[:], uint64(id))
	h.Write(idBuf[:])
	h.Write([]byte(overload))
	h.Write(hashZeroMarker)
	for _, arg := range args {
		switch v := arg.(type) {
		case types.String:
			h.Write(hashStringMarker)
			h.Write([]byte(string(v)))
		case types.Bool:
			if bool(v) {
				h.Write(hashBoolTrueMarker)
			} else {
				h.Write(hashBoolFalseMarker)
			}
		case types.Int:
			h.Write(hashNumberMarker)
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(float64(v)))
			h.Write(buf[:])
		case types.Uint:
			h.Write(hashNumberMarker)
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(float64(v)))
			h.Write(buf[:])
		case types.Double:
			h.Write(hashNumberMarker)
			if math.IsNaN(float64(v)) {
				h.Write([]byte("NaN"))
				continue
			}
			// Normalize -0.0 to 0.0. Go will treat -0.0 as 0.0 at compile time,
			// but the function math.Copysign(0.0, -1.0) can be used to test the -0.0 case.
			if v == types.Double(0.0) && math.Signbit(float64(v)) {
				v = types.Double(0.0)
			}
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(float64(v)))
			h.Write(buf[:])
		default:
			// Value intentionally omitted; bucket membership falls back to equals.
			h.Write(hashDefaultMarker)
		}
		// Separator to avoid cross-argument collisions, e.g. ("a", "bc") vs ("ab", "c").
		h.Write(hashZeroMarker)
	}
	return h.Sum64()
}

// findInBucket returns the call state in the bucket matching the same node id and call identity,
// or nil if no match is present.
func findInBucket(bucket []*asyncCallState, id int64, want *asyncCallState) *asyncCallState {
	for _, acs := range bucket {
		if acs.id == id && acs.equals(want) {
			return acs
		}
	}
	return nil
}

// getOrCreate returns the existing call state for the (node id, args) tuple, or registers and
// returns a new one. A newly registered call is assigned a unique callID and counted as pending.
func (t *asyncCallStateTracker) getOrCreate(id int64, function, overload string, argVals []ref.Val, impl functions.AsyncOp, completions chan<- int64) *asyncCallState {
	key := hashCall(id, overload, argVals)
	potentialState := newAsyncCallState(id, function, overload, argVals, impl)

	t.mu.RLock()
	acs := findInBucket(t.calls[key], id, potentialState)
	t.mu.RUnlock()
	if acs != nil {
		return acs
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	// Check again in case it was created while waiting for the lock.
	if acs := findInBucket(t.calls[key], id, potentialState); acs != nil {
		return acs
	}

	// Assign a new unique call ID for this async call.
	callID := t.nextCallID.Add(1)
	potentialState.callID = callID
	potentialState.completions = completions
	t.calls[key] = append(t.calls[key], potentialState)
	t.callsByID[callID] = potentialState
	// Note: pendingCalls is incremented when the call is actually launched (see launch), not at
	// registration, so that "pending" reflects in-flight calls and excludes calls deferred by the
	// launch limiter. Counting deferred calls as pending would deadlock DrainAll.
	return potentialState
}

func (t *asyncCallStateTracker) getByID(callID int64) *asyncCallState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.callsByID[callID]
}

func (t *asyncCallStateTracker) pendingCount() int {
	return int(t.pendingCalls.Load())
}

func (t *asyncCallStateTracker) notifyCompletion(_ int64) {
	t.pendingCalls.Add(-1)
}

func newAsyncCallState(id int64, function, overload string, argVals []ref.Val, impl functions.AsyncOp) *asyncCallState {
	return &asyncCallState{
		id:       id,
		function: function,
		overload: overload,
		argVals:  argVals,
		impl:     impl,
	}
}

// asyncCallState tracks the result of a single async function call across multiple re-evaluations.
type asyncCallState struct {
	id       int64 // AST expression node ID where the call is defined.
	callID   int64 // Unique incremental tracking ID assigned to this call.
	function string
	overload string
	argVals  []ref.Val
	impl     functions.AsyncOp

	mu      sync.RWMutex
	started bool
	result  ref.Val

	completions chan<- int64
}

// CallID returns the unique identifier for this async call invocation.
func (acs *asyncCallState) CallID() int64 {
	return acs.callID
}

// Function returns the name of the function being called.
func (acs *asyncCallState) Function() string {
	return acs.function
}

// Overload returns the specific overload ID being invoked.
func (acs *asyncCallState) Overload() string {
	return acs.overload
}

// launch returns a call's cached result, or starts the call (subject to the launch limiter) and
// returns an Unknown referencing its callID while the result is pending.
//
// Admission control: when a concurrency semaphore is configured, a launch slot is reserved with a
// non-blocking send. If no slot is free the call is left unstarted and an Unknown is returned; the
// call is retried on a later re-evaluation pass once an in-flight call completes and frees a slot.
// The reservation is non-blocking on purpose — the evaluator runs on a single goroutine, and
// blocking it here while completing calls block on an undrained completion channel would deadlock.
// The slot is held by the launched goroutine and released when it exits, so the number of live
// async goroutines is bounded by the semaphore capacity.
func (t *asyncCallStateTracker) launch(ctx context.Context, acs *asyncCallState, observer AsyncObserver, semaphore chan struct{}) ref.Val {
	acs.mu.RLock()
	res := acs.result
	started := acs.started
	acs.mu.RUnlock()
	if res != nil {
		return res
	}
	if started {
		return types.NewUnknown(acs.callID, nil)
	}

	if semaphore != nil {
		// Non-blocking try-acquire of a concurrency slot. If the semaphore channel is full,
		// we immediately fallback to default to return an Unknown. This prevents blocking
		// the single-threaded evaluator, allowing it to continue with other evaluations.
		// The call will be retried in a future evaluation pass.
		select {
		case semaphore <- struct{}{}:
		default:
			// No launch slot available; defer to a later pass.
			return types.NewUnknown(acs.callID, nil)
		}
	}
	acs.mu.Lock()
	if acs.started || acs.result != nil {
		// Defensive: the evaluator is single-threaded so this should not happen, but if it does,
		// return the reserved slot rather than leak it.
		acs.mu.Unlock()
		if semaphore != nil {
			<-semaphore
		}
		return types.NewUnknown(acs.callID, nil)
	}
	acs.started = true
	acs.mu.Unlock()
	t.pendingCalls.Add(1)

	if observer != nil {
		observer.OnCallStarted(acs.callID, acs.function, acs.overload, acs.argVals)
	}
	go func() {
		if semaphore != nil {
			// Release the concurrency slot when this call finishes or is cancelled.
			defer func() { <-semaphore }()
		}
		if acs.completions != nil {
			defer func() {
				// Prioritize context cancellation to prevent racy completion signals.
				if ctx.Err() != nil {
					return
				}
				// Signal completion to the evaluator. Use a select with ctx.Done() to prevent
				// the goroutine from leaking if the evaluator has already timed out or aborted.
				select {
				case acs.completions <- acs.callID:
				case <-ctx.Done():
				}
			}()
		}

		ch := acs.impl(ctx, acs.argVals...)
		// Early terminate with a CEL error when an implementation returns an empty channel.
		if ch == nil {
			acs.mu.Lock()
			defer acs.mu.Unlock()
			acs.result = types.NewErrFromString(
				fmt.Sprintf("function %s returned an empty channel", acs.function))
			return
		}
		// Wait for the async computation to finish or for the context to be cancelled.
		select {
		case r := <-ch:
			acs.mu.Lock()
			acs.result = r
			acs.mu.Unlock()
			if observer != nil {
				observer.OnCallFinished(acs.callID, acs.function, acs.overload, r)
			}
		case <-ctx.Done():
			// Evaluation context cancelled before the async operation completed.
		}
	}()
	return types.NewUnknown(acs.callID, nil)
}

// equals reports whether two call states refer to the same function, overload, and arguments.
func (acs *asyncCallState) equals(other *asyncCallState) bool {
	if acs == nil || other == nil {
		return false
	}
	if acs.function != other.function || acs.overload != other.overload {
		return false
	}
	if len(acs.argVals) != len(other.argVals) {
		return false
	}
	for i, v := range acs.argVals {
		otherV := other.argVals[i]
		if types.Equal(v, otherV) == types.True {
			continue
		}
		if n, ok := v.(types.Double); ok {
			// Treat NaN as equivalent for the sake of function dispatch equality.
			if otherN, ok := otherV.(types.Double); ok && math.IsNaN(float64(n)) && math.IsNaN(float64(otherN)) {
				continue
			}
		}
		return false
	}
	return true
}

// trackerShrinkThreshold is the entry count above which a released tracker's maps are reallocated
// rather than cleared in place, so the pool does not retain a large backing array indefinitely.
const trackerShrinkThreshold = 1024

// asyncCallStateTrackerPool provides a synchronized pool of asyncCallStateTrackers.
type asyncCallTrackerPool struct {
	sync.Pool
}

func (pool *asyncCallTrackerPool) create() *asyncCallStateTracker {
	return pool.Get().(*asyncCallStateTracker)
}

func (pool *asyncCallTrackerPool) release(tracker *asyncCallStateTracker) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	// Clearing with delete reuses the backing arrays, which is ideal for the common case but pins
	// a large allocation in the pool after a wide fan-out (e.g. an async call over a big list).
	// Past a threshold, reallocate so the high-water-mark memory is released to the GC instead of
	// being retained by the pooled tracker.
	if len(tracker.calls) > trackerShrinkThreshold || len(tracker.callsByID) > trackerShrinkThreshold {
		tracker.calls = make(map[uint64][]*asyncCallState)
		tracker.callsByID = make(map[int64]*asyncCallState)
	} else {
		for k := range tracker.calls {
			delete(tracker.calls, k)
		}
		for k := range tracker.callsByID {
			delete(tracker.callsByID, k)
		}
	}
	tracker.pendingCalls.Store(0)
	tracker.nextCallID.Store(0)
	tracker.mu.Unlock()
	pool.Pool.Put(tracker)
}

func newAsyncCallTrackerPool() *asyncCallTrackerPool {
	return &asyncCallTrackerPool{
		Pool: sync.Pool{
			New: func() any {
				return newAsyncCallStateTracker()
			},
		},
	}
}

var asyncCallStateTrackerPool = newAsyncCallTrackerPool()
