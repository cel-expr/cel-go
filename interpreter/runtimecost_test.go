// Copyright 2022 Google LLC
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
	"testing"

	"cel.dev/cel-go/checker"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/parser"
)

func TestCostTrackerForwarding(t *testing.T) {
	tracker, err := NewCostTracker(nil,
		CostTrackerLimit(100),
		PresenceTestHasCost(true),
		OverloadCostTracker(overloads.ContainsString, func(args []ref.Val, result ref.Val) *uint64 {
			c := uint64(5)
			return &c
		}),
	)
	if err != nil {
		t.Fatalf("NewCostTracker() failed: %v", err)
	}

	clone, err := tracker.Clone()
	if err != nil {
		t.Fatalf("tracker.Clone() failed: %v", err)
	}

	clone.CreateList(1, nil)
	if clone.ActualCost() != 10 {
		t.Errorf("clone.ActualCost() = %d, wanted 10", clone.ActualCost())
	}
	if !clone.PresenceTestHasCost() {
		t.Errorf("clone.PresenceTestHasCost() = false, wanted true")
	}
}

func TestCostObserverIntegration(t *testing.T) {
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	src := common.NewTextSource("a + b")
	parsed, errs := p.Parse(src)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Parse() failed: %v", errs.ToDisplayString())
	}

	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	env := newTestEnv(t, cont, reg)
	err = env.AddIdents(
		decls.NewVariable("a", types.IntType),
		decls.NewVariable("b", types.IntType),
	)
	if err != nil {
		t.Fatalf("AddIdents() failed: %v", err)
	}

	checked, errs := checker.Check(parsed, src, env)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Check() failed: %v", errs.ToDisplayString())
	}

	tracker, err := NewCostTracker(nil)
	if err != nil {
		t.Fatalf("NewCostTracker() failed: %v", err)
	}

	interp := newStandardInterpreter(t, cont, reg, reg, attrs)
	prg, err := interp.NewInterpretable(checked,
		CostObserver(CostTrackerFactory(func() (*CostTracker, error) {
			return tracker, nil
		})))
	if err != nil {
		t.Fatalf("NewInterpretable() failed: %v", err)
	}

	act, err := NewActivation(map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}
	frame := AsFrame(act)
	prg.Exec(frame)

	if tracker.ActualCost() != 3 {
		t.Errorf("tracker.ActualCost() = %d, wanted 3", tracker.ActualCost())
	}
}

func TestCostLimitExceededPanic(t *testing.T) {
	tracker, err := NewCostTracker(nil, CostTrackerLimit(5))
	if err != nil {
		t.Fatalf("NewCostTracker() failed: %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on cost limit exceeded")
		}
		if cancelledErr, ok := r.(EvalCancelledError); !ok || cancelledErr.Cause != CostLimitExceeded {
			t.Errorf("got panic %v, wanted EvalCancelledError with CostLimitExceeded", r)
		}
	}()

	tracker.CreateList(1, nil) // base cost = 10 > 5 -> triggers limitExceededHandler -> panics EvalCancelledError
}
