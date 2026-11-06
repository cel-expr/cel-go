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
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// TraceRecord is a single entry in the record stream of a traced evaluation.
//
// Records are numbered from one in evaluation order. A record with a Key begins an iteration of
// the comprehension ID, where Key is the list index or map key of the iteration. Any other record
// holds the Value of the expression ID. Parent is the number of the record which began the
// iteration in which the record was made, or zero outside of any iteration.
type TraceRecord struct {
	ID     int64
	Parent int
	Key    ref.Val
	Value  ref.Val
}

// IsIteration reports whether the record begins a comprehension iteration.
func (r TraceRecord) IsIteration() bool {
	return r.Key != nil
}

// NewTracedEvalState returns an EvalState which records a TraceRecord for every observed value and
// every comprehension iteration, for use with the EvalStateObserver and IterationTracing options.
//
// The records are retrieved with TraceRecords. When an evaluation is resumed, e.g. by
// ConcurrentEval, the records of the previous pass are discarded, so the records describe the
// final, complete pass. Once evaluation completes, the state may be read concurrently.
func NewTracedEvalState() EvalState {
	return &evalState{values: map[int64]ref.Val{}, traced: true}
}

// TraceRecords returns the records of a state created by NewTracedEvalState, in evaluation order.
//
// The records are owned by the state and must not be modified.
func TraceRecords(state EvalState) ([]TraceRecord, bool) {
	s, ok := state.(*evalState)
	if !ok || !s.traced {
		return nil, false
	}
	return s.records, true
}

// enterIteration records the start of an iteration and returns its record number.
func (s *evalState) enterIteration(parent int, comprehensionID int64, key ref.Val) int {
	s.records = append(s.records, TraceRecord{ID: comprehensionID, Parent: parent, Key: key})
	return len(s.records)
}

// discardIteration removes the record which began the iteration numbered iter, which must be the
// latest iteration, along with the records made within it. Records made within enclosing scopes,
// e.g. by a lazily initialized accumulator, are kept. They refer only to iterations which began
// earlier, so moving them does not invalidate any record number.
func (s *evalState) discardIteration(iter int) {
	kept := s.records[:iter-1]
	for _, r := range s.records[iter:] {
		if r.Parent < iter {
			kept = append(kept, r)
		}
	}
	s.records = kept
}

// recordValue records the value of an expression within an iteration. Repeated observations of an
// expression within the same iteration, such as the attribute and qualifier observations of a
// selection, are collapsed into one record holding the latest value.
func (s *evalState) recordValue(exprID int64, parent int, val ref.Val) {
	if n := len(s.records); n > 0 {
		if last := &s.records[n-1]; !last.IsIteration() && last.ID == exprID && last.Parent == parent {
			last.Value = val
			return
		}
	}
	s.records = append(s.records, TraceRecord{ID: exprID, Parent: parent, Value: val})
}

// IterationTracing annotates comprehensions so that each iteration is recorded, and lazily
// initialized comprehension accumulators and cel.@block slots so that their values are attributed
// to the iteration in which they were declared rather than the one in which they were first used.
//
// Traced comprehensions accumulate into immutable values so that the value recorded for each
// iteration is a snapshot rather than a reference to an accumulator which is later updated in place.
//
// Tracing takes effect when the EvalStateObserver produces a state created by NewTracedEvalState.
// Programs planned without this option incur no tracing overhead.
func IterationTracing() PlannerOption {
	return CustomDecoratorV2(func(i InterpretableV2) (InterpretableV2, error) {
		switch inst := i.(type) {
		case *evalFold:
			inst.traced = true
			inst.mutableAccu = false
		case *evalBlock:
			inst.traced = true
		}
		return i, nil
	})
}

// tracedLazySlot returns a per-evaluation copy of a lazy slot which resolves within the current
// iteration, or the planned lazy slot if tracing is not active.
func tracedLazySlot(frame *ExecutionFrame, l *lazySlot) *lazySlot {
	if frame.ctx == nil || frame.ctx.tracer == nil {
		return l
	}
	traced := *l
	traced.trace = frame.ctx.trace
	return &traced
}

// newTracingFolder returns a folder which records each iteration when tracing is active, or the
// slotFolder unmodified otherwise.
func newTracingFolder(sf *slotFolder) traits.Folder {
	ctx := sf.frame.ctx
	if ctx == nil || ctx.tracer == nil {
		return sf
	}
	return &tracingFolder{slotFolder: sf, parent: ctx.trace}
}

// tracingFolder records each fold iteration and attributes the observations made within it to the
// iteration.
type tracingFolder struct {
	*slotFolder
	parent  int
	ordinal int
}

// FoldEntry implements the traits.Folder interface method.
func (t *tracingFolder) FoldEntry(key, val any) bool {
	// Iterables which do not expose keys are keyed by the iteration ordinal.
	var k ref.Val = types.Int(t.ordinal)
	if key != nil {
		k = t.fold.adapter.NativeToValue(key)
	}
	t.ordinal++
	ctx := t.frame.ctx
	iter := ctx.tracer.enterIteration(t.parent, t.fold.id, k)
	ctx.trace = iter
	cont := t.slotFolder.FoldEntry(key, val)
	ctx.trace = t.parent
	if !cont && !t.interrupted {
		// The loop condition ended the fold before the step was evaluated, so the iteration did
		// not take place.
		ctx.tracer.discardIteration(iter)
	}
	return cont
}
