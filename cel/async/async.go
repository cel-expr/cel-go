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

// Package async provides helpers for configuring and executing asynchronous CEL functions,
// including drain strategies, retry, timeout, concurrency limiting, and caching wrappers.
package async

import (
	"time"

	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/interpreter"
)

// Call describes a pending or completed asynchronous function call.
// This interface exposes a safe, read-only view of the internal interpreter state.
type Call = interpreter.AsyncCall

// Observer provides callbacks for monitoring the lifecycle of asynchronous function calls.
//
// Implementations must be safe for concurrent use: the start and finish callbacks run on different
// goroutines, and finish callbacks for distinct calls may run concurrently. See
// interpreter.AsyncObserver for details.
type Observer = interpreter.AsyncObserver

// BlockingOp is a blocking asynchronous function operation.
type BlockingOp = functions.BlockingAsyncOp

// DrainAction dictates what ConcurrentEval should do after inspecting completions.
type DrainAction struct {
	// Reevaluate indicates that the AST should be re-evaluated immediately.
	// If true, WaitDuration is ignored.
	Reevaluate bool
	// WaitDuration indicates how long the evaluator should wait for additional
	// completions before deciding to re-evaluate. A duration of 0 means wait
	// indefinitely (block on the next completion).
	WaitDuration time.Duration
}

// DrainStrategy controls when ConcurrentEval re-evaluates after async completions.
//
// The evaluator consults the strategy each time a completion is received.
type DrainStrategy interface {
	// NextAction evaluates the current state of asynchronous evaluation and
	// determines the next step.
	//
	// - completed: The set of completions accumulated in the current batch.
	// - active: The number of async calls currently launched but unresolved.
	NextAction(completed []Call, active int) DrainAction
}

// DrainNone returns a strategy that re-evaluates after every single completion.
// This is the default strategy.
func DrainNone() DrainStrategy {
	return drainNone{}
}

type drainNone struct{}

func (drainNone) NextAction(completed []Call, active int) DrainAction {
	return DrainAction{Reevaluate: active == 0 || len(completed) > 0}
}

// DrainReady returns a strategy that waits for a short duration after the first
// completion to batch any other functions that complete at roughly the same time.
func DrainReady(debounce time.Duration) DrainStrategy {
	return drainReady{debounce: debounce}
}

type drainReady struct {
	debounce time.Duration
}

func (d drainReady) NextAction(completed []Call, active int) DrainAction {
	if active == 0 {
		return DrainAction{Reevaluate: true} // Nothing left to wait for
	}
	if len(completed) == 0 {
		return DrainAction{Reevaluate: false, WaitDuration: 0} // Wait indefinitely for first
	}
	return DrainAction{Reevaluate: false, WaitDuration: d.debounce} // Wait for debounce period
}

// DrainAll returns a strategy that waits for all currently pending calls to
// complete before re-evaluating.
//
// Note: This strategy is optimal for independent async calls, but will over-wait
// if some calls depend on the results of others.
func DrainAll() DrainStrategy {
	return drainAll{}
}

type drainAll struct{}

func (drainAll) NextAction(completed []Call, active int) DrainAction {
	return DrainAction{Reevaluate: active == 0}
}
