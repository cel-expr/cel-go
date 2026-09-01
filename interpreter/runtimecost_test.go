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

func TestListMapAccessCost(t *testing.T) {
	obj := map[string]any{
		"listMap": []any{
			map[string]any{"k": "a1", "v": "b1"},
			map[string]any{"k": "a2", "v": "b2"},
			map[string]any{"k": "a3", "v": "b3", "v2": "z"},
		},
	}
	expectCost := map[string]uint64{
		"has(self.listMap[0].v)":                           3,
		"self.listMap.all(m, m.k.startsWith('a'))":         21,
		"self.listMap.all(m, !has(m.v2) || m.v2 == 'z')":   21,
		"self.listMap.exists(m, m.k.endsWith('1'))":        13,
		"self.listMap.exists_one(m, m.k == 'a3')":          15,
		"!self.listMap.all(m, m.k.endsWith('1'))":          18,
		"!self.listMap.exists(m, m.v == 'x')":              25,
		"!self.listMap.exists_one(m, m.k.startsWith('a'))": 20,
		// In v0.28.1, filter and map did not incur a +10 base creation cost on empty accumulator init.
		// In HEAD, evaluating the empty list literal in accu_init incurs +10 per comprehension:
		// - size(filter) == 1 was 27 in v0.28.1, currently 37
		"size(self.listMap.filter(m, m.k == 'a1')) == 1":     37,
		"self.listMap.exists(m, m.k == 'a1' && m.v == 'b1')": 16,
		// - map.exists was 55 in v0.28.1, currently 65
		"self.listMap.map(m, m.v).exists(v, v == 'b1')": 65,

		// test comprehensions where the field used in predicates is unset on all but one of the elements:
		// - with has checks:

		"self.listMap.exists(m, has(m.v2) && m.v2 == 'z')":     21,
		"!self.listMap.all(m, has(m.v2) && m.v2 != 'z')":       10,
		"self.listMap.exists_one(m, has(m.v2) && m.v2 == 'z')": 12,
		// - filter.size == 1 was 24 in v0.28.1, currently 34
		"self.listMap.filter(m, has(m.v2) && m.v2 == 'z').size() == 1": 34,
		// - map(filter, transform).size == 1 was 25 in v0.28.1, currently 35
		"self.listMap.map(m, has(m.v2) && m.v2 == 'z', m.v2).size() == 1": 35,
		// - filter.map.size == 1 was 39 in v0.28.1 (two comprehensions), currently 59 (+20)
		"self.listMap.filter(m, has(m.v2) && m.v2 == 'z').map(m, m.v2).size() == 1": 59,
		// - without has checks:

		// all() and exists() macros ignore errors from predicates so long as the condition holds for at least one element
		"self.listMap.exists(m, m.v2 == 'z')": 24,
		"!self.listMap.all(m, m.v2 != 'z')":   22,
	}

	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	env := newTestEnv(t, cont, reg)
	err = env.AddIdents(decls.NewVariable("self", types.DynType))
	if err != nil {
		t.Fatalf("AddIdents() failed: %v", err)
	}

	for expr, expected := range expectCost {
		t.Run(expr, func(t *testing.T) {
			src := common.NewTextSource(expr)
			parsed, errs := p.Parse(src)
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("Parse(%q) failed: %v", expr, errs.ToDisplayString())
			}
			checked, errs := checker.Check(parsed, src, env)
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("Check(%q) failed: %v", expr, errs.ToDisplayString())
			}
			tracker, err := NewCostTracker(nil,
				PresenceTestHasCost(false),
			)
			if err != nil {
				t.Fatalf("NewCostTracker() failed: %v", err)
			}
			interp := newStandardInterpreter(t, cont, reg, reg, attrs)
			prg, err := interp.NewInterpretable(checked,
				CostObserver(CostTrackerFactory(func() (*CostTracker, error) {
					return tracker, nil
				})))
			if err != nil {
				t.Fatalf("NewInterpretable(%q) failed: %v", expr, err)
			}
			act, err := NewActivation(map[string]any{"self": obj})
			if err != nil {
				t.Fatalf("NewActivation() failed: %v", err)
			}
			frame := AsFrame(act)
			res := prg.Exec(frame)
			actual := tracker.ActualCost()
			t.Logf("expr: %s => res: %v, actual cost: %d, expected: %d", expr, res, actual, expected)
			if actual != expected {
				t.Errorf("expr %q actual cost = %d, wanted %d (res: %v)", expr, actual, expected, res)
			}
		})
	}
}
