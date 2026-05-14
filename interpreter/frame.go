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
	"sync"
	"sync/atomic"
)

// evalContext contains the stateful information needed for a single evaluation.
//
// This state is shared across all frames within a single evaluation, including
// child frames created for comprehension blocks.
type evalContext struct {
	// interrupt exposes a callback channel for cancellation.
	interrupt <-chan struct{}

	// interruptCheckCount is the number of times the interrupt channel has been checked.
	interruptCheckCount atomic.Uint64

	// interruptCheckFrequency is the frequency at which the interrupt channel is checked.
	interruptCheckFrequency uint

	// interrupted indicates whether the evaluation has been interrupted.
	interrupted atomic.Bool

	// state provides the context for tracking the evaluation state.
	state EvalState

	// costs provides the context for tracking the evaluation costs.
	costs *CostTracker

	// ctx is the context for async call implementations to use.
	ctx context.Context

	// cancel cancels the context when the evaluation is finished.
	cancel context.CancelFunc
}

// ExecutionFrame provides the context for a single evaluation of an expression.
//
// The execution frame must not be stored in any fashion as its lifecycle is completely
// controlled by the CEL evaluation process.
type ExecutionFrame struct {
	// Activation provides the context for resolving variables by name.
	Activation

	// parent provides the context for parent scopes (used for comprehension iterators).
	parent *ExecutionFrame

	// ctx provides the shared evaluation state across frames.
	ctx *evalContext
}

// NewExecutionFrame creates a new execution frame from the pool.
func NewExecutionFrame(vars Activation) *ExecutionFrame {
	f := frameStack.Get().(*ExecutionFrame)
	f.Activation = vars
	return f
}

// SetContext sets the context for the execution frame.
func (f *ExecutionFrame) SetContext(ctx context.Context, interruptCheckFrequency uint) {
	if f.ctx == nil {
		f.ctx = evalContextPool.Get().(*evalContext)
	}
	f.ctx.ctx, f.ctx.cancel = context.WithCancel(ctx)
	f.ctx.interrupt = ctx.Done()
	f.ctx.interruptCheckFrequency = interruptCheckFrequency
	f.ctx.interruptCheckCount.Store(0)
	f.ctx.interrupted.Store(false)
}

// Close releases the resources held by the execution frame and returns it to the pool.
func (f *ExecutionFrame) Close() {
	if f.ctx != nil {
		if f.ctx.cancel != nil {
			f.ctx.cancel()
			f.ctx.cancel = nil
		}
		f.ctx.ctx = nil
		f.ctx.interrupt = nil
		f.ctx.state = nil
		f.ctx.costs = nil
		f.ctx.interrupted.Store(false)
		f.ctx.interruptCheckCount.Store(0)
		f.ctx.interruptCheckFrequency = 0
		evalContextPool.Put(f.ctx)
		f.ctx = nil
	}
	f.parent = nil
	activationStack.release(f.Activation)
	f.Activation = nil
	frameStack.Put(f)
}

// push pushes the given activation onto the activation stack and returns the new frame.
//
// This operation is internal to the interpreter and is used to handle comprehension
// scoping. The child frame inherits the shared evalContext from the parent.
func (f *ExecutionFrame) push(activation Activation) *ExecutionFrame {
	child := frameStack.Get().(*ExecutionFrame)
	child.parent = f
	child.ctx = f.ctx
	child.Activation = activationStack.create(f.Activation, activation)
	return child
}

// pop returns the parent frame, releasing the current frame back to the pool.
func (f *ExecutionFrame) pop() *ExecutionFrame {
	parent := f.parent
	activationStack.release(f.Activation)
	f.Activation = nil
	f.parent = nil
	f.ctx = nil
	frameStack.Put(f)
	return parent
}

// ResolveName implements the Activation interface by proxying to the internal activation.
func (f *ExecutionFrame) ResolveName(name string) (any, bool) {
	return f.Activation.ResolveName(name)
}

// Parent implements the Activation interface by proxying to the internal activation.
func (f *ExecutionFrame) Parent() Activation {
	return f.Activation.Parent()
}

// AsPartialActivation implements the PartialActivation interface by proxying to the internal activation.
func (f *ExecutionFrame) AsPartialActivation() (PartialActivation, bool) {
	return AsPartialActivation(f.Activation)
}

// Unwrap returns the internal activation.
func (f *ExecutionFrame) Unwrap() Activation {
	return f.Activation
}

// CheckInterrupt returns whether the evaluation has been interrupted.
func (f *ExecutionFrame) CheckInterrupt() bool {
	if f.ctx == nil {
		return false
	}
	if f.ctx.interrupted.Load() {
		return true
	}
	count := f.ctx.interruptCheckCount.Add(1)
	if f.ctx.interruptCheckFrequency > 0 && count%uint64(f.ctx.interruptCheckFrequency) == 0 {
		select {
		case <-f.ctx.interrupt:
			f.ctx.interrupted.Store(true)
			return true
		default:
			return false
		}
	}
	return false
}

// frameStack provides a synchronized pool of ExecutionFrames.
var frameStack = &sync.Pool{
	New: func() any {
		return &ExecutionFrame{}
	},
}

// evalContextPool provides a synchronized pool of evalContexts.
var evalContextPool = &sync.Pool{
	New: func() any {
		return &evalContext{}
	},
}

type activationStackPool struct {
	sync.Pool
}

func (pool *activationStackPool) create(parent, child Activation) Activation {
	h := pool.Get().(*hierarchicalActivation)
	h.child = child
	h.parent = parent
	return h
}

func (pool *activationStackPool) release(activation Activation) {
	h, ok := activation.(*hierarchicalActivation)
	if !ok {
		return
	}
	h.parent = nil
	h.child = nil
	pool.Pool.Put(h)
}

func newActivationPool() *activationStackPool {
	return &activationStackPool{
		Pool: sync.Pool{
			New: func() any {
				return &hierarchicalActivation{}
			},
		},
	}
}

var (
	activationStack = newActivationPool()
)
