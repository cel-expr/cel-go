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
	"testing"
	"time"

	"github.com/google/cel-go/cel/async"
)

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
