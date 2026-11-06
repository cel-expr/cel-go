// Copyright 2018 Google LLC
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
	"maps"

	"cel.dev/cel-go/common/types/ref"
)

// EvalState tracks the values associated with expression ids during execution.
type EvalState interface {
	// IDs returns the list of ids with recorded values.
	IDs() []int64

	// Value returns the observed value of the given expression id if found, and a nil false
	// result if not.
	Value(int64) (ref.Val, bool)

	// SetValue sets the observed value of the expression id.
	SetValue(int64, ref.Val)

	// Reset clears the previously recorded expression values.
	Reset()
}

// evalState records the last value observed for each expression.
//
// A state created by NewTracedEvalState also records every observation in evaluation order, so that
// recording a value during evaluation is an append rather than a map write. The records are folded
// into the last values once, when evaluation completes, so that reading the state never writes to
// it.
type evalState struct {
	values map[int64]ref.Val

	traced  bool
	records []TraceRecord
	// indexed is the number of records reflected in values.
	indexed int
}

// NewEvalState returns an EvalState instanced used to observe the intermediate
// evaluations of an expression.
func NewEvalState() EvalState {
	return &evalState{values: make(map[int64]ref.Val)}
}

// index folds the records made since the last call into the last value of each expression.
func (s *evalState) index() {
	foldLastValues(s.values, s.records[s.indexed:])
	s.indexed = len(s.records)
}

// foldLastValues sets the value of each expression in the records into values, in order.
func foldLastValues(values map[int64]ref.Val, records []TraceRecord) {
	for _, r := range records {
		if !r.IsIteration() {
			values[r.ID] = r.Value
		}
	}
}

// IDs implements the EvalState interface method.
func (s *evalState) IDs() []int64 {
	values := s.values
	if s.indexed < len(s.records) {
		// Evaluation was interrupted before the records were indexed, e.g. by cancellation.
		values = maps.Clone(s.values)
		foldLastValues(values, s.records[s.indexed:])
	}
	var ids []int64
	for k, v := range values {
		if v != nil {
			ids = append(ids, k)
		}
	}
	return ids
}

// Value is an implementation of the EvalState interface method.
func (s *evalState) Value(exprID int64) (ref.Val, bool) {
	// Records which have not been indexed, e.g. because evaluation was interrupted, are newer than
	// the indexed values.
	for i := len(s.records) - 1; i >= s.indexed; i-- {
		if r := s.records[i]; r.ID == exprID && !r.IsIteration() {
			return r.Value, r.Value != nil
		}
	}
	val := s.values[exprID]
	return val, val != nil
}

// SetValue is an implementation of the EvalState interface method.
//
// A nil value clears the value of the expression. A traced state records the value outside of any
// iteration.
func (s *evalState) SetValue(exprID int64, val ref.Val) {
	if s.traced {
		s.recordValue(exprID, 0, val)
		return
	}
	if val == nil {
		delete(s.values, exprID)
	} else {
		s.values[exprID] = val
	}
}

// Reset implements the EvalState interface method.
func (s *evalState) Reset() {
	s.values = map[int64]ref.Val{}
	s.records = s.records[:0]
	s.indexed = 0
}
