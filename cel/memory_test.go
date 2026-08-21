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

package cel

import (
	"context"
	"math"
	"strings"
	"testing"

	"cel.dev/cel-go/common/types"
)

func TestMemoryTracking(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		decls    []EnvOption
		memOpts  []types.MemoryTrackerOption
		in       any
		wantPeak uint32
	}{
		{
			name:  "attribute_resolution",
			expr:  `a`,
			decls: []EnvOption{Variable("a", ListType(IntType))},
			in:    map[string]any{"a": []int64{1, 2, 3}},
			// 1 (list container) + 3 elements, then the call-free program peaks at the attribute.
			wantPeak: 4,
		},
		{
			name:  "call_output",
			expr:  `a + a`,
			decls: []EnvOption{Variable("a", ListType(IntType))},
			in:    map[string]any{"a": []int64{1, 2}},
			// The peak is the call output: a lazy concat list backed by both inputs, sizing
			// as their sum (3 + 3 = 6), which exceeds either input observed on its own.
			wantPeak: 6,
		},
		{
			name:  "attribute_field_selection",
			expr:  `m.vals`,
			decls: []EnvOption{Variable("m", MapType(StringType, ListType(IntType)))},
			in:    map[string]any{"m": map[string][]int64{"vals": {1, 2, 3}}},
			// The resolved attribute value is the inner list: 1 (container) + 3 elements.
			wantPeak: 4,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t, tc.decls...)
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			program, err := env.Program(ast, MemoryTracking(tc.memOpts...))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			_, details, err := program.Eval(tc.in)
			if err != nil {
				t.Fatalf("program.Eval() failed: %v", err)
			}
			peak := details.PeakMemory()
			if peak == nil {
				t.Fatalf("EvalDetails.PeakMemory() got nil, wanted %d", tc.wantPeak)
			}
			if *peak != tc.wantPeak {
				t.Errorf("EvalDetails.PeakMemory() got %d, wanted %d", *peak, tc.wantPeak)
			}
			if details.MemoryTracker() == nil {
				t.Error("EvalDetails.MemoryTracker() got nil, wanted tracker")
			}
		})
	}
}

func TestMemoryTrackingComprehension(t *testing.T) {
	env := testEnv(t, Variable("a", ListType(IntType)))
	ast, iss := env.Compile(`a.map(x, x * 2)`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	program, err := env.Program(ast, MemoryTracking())
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	in := map[string]any{"a": []int64{1, 2, 3, 4, 5}}
	_, details, err := program.Eval(in)
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	peak := details.PeakMemory()
	if peak == nil {
		t.Fatal("EvalDetails.PeakMemory() got nil, wanted non-nil peak")
	}
	// The comprehension result is 1 (container) + 5 elements; the peak must be at least as
	// large since the final accumulation observes the input alongside the built list.
	if *peak < 6 {
		t.Errorf("EvalDetails.PeakMemory() got %d, wanted at least 6", *peak)
	}
}

func TestMemoryTrackingConcurrentEval(t *testing.T) {
	env := testEnv(t, Variable("a", ListType(IntType)))
	ast, iss := env.Compile(`a + a`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	program, err := env.Program(ast, MemoryTracking())
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res := <-program.ConcurrentEval(ctx, map[string]any{"a": []int64{1, 2}})
	if res.Err != nil {
		t.Fatalf("program.ConcurrentEval() failed: %v", res.Err)
	}
	peak := res.EvalDetails.PeakMemory()
	if peak == nil {
		t.Fatal("EvalDetails.PeakMemory() got nil, wanted non-nil peak")
	}
	if *peak != 6 {
		t.Errorf("EvalDetails.PeakMemory() got %d, wanted 6", *peak)
	}
}

func TestMemoryTrackingDisabled(t *testing.T) {
	env := testEnv(t, Variable("a", ListType(IntType)))
	ast, iss := env.Compile(`a + a`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	program, err := env.Program(ast, EvalOptions(OptTrackState))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, details, err := program.Eval(map[string]any{"a": []int64{1, 2}})
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	if peak := details.PeakMemory(); peak != nil {
		t.Errorf("EvalDetails.PeakMemory() got %d, wanted nil when tracking disabled", *peak)
	}
	if tracker := details.MemoryTracker(); tracker != nil {
		t.Errorf("EvalDetails.MemoryTracker() got %v, wanted nil when tracking disabled", tracker)
	}
}

func TestMemoryLimit(t *testing.T) {
	tests := []struct {
		name     string
		memLimit uint32
		wantErr  string
	}{
		{
			name:     "under_limit",
			memLimit: 1000,
		},
		{
			name:     "over_limit",
			memLimit: 5,
			wantErr:  "memory limit exceeded",
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t, Variable("a", ListType(IntType)))
			ast, iss := env.Compile(`a + a`)
			if iss.Err() != nil {
				t.Fatalf("env.Compile() failed: %v", iss.Err())
			}
			program, err := env.Program(ast, MemoryLimit(tc.memLimit))
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			_, _, err = program.Eval(map[string]any{"a": []int64{1, 2, 3}})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("program.Eval() failed: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("program.Eval() got error %v, wanted error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestMemoryTrackingCalculationLimitExceeded(t *testing.T) {
	env := testEnv(t, Variable("a", ListType(StringType)))
	ast, iss := env.Compile(`a`)
	if iss.Err() != nil {
		t.Fatalf("env.Compile() failed: %v", iss.Err())
	}
	program, err := env.Program(ast,
		MemoryTracking(
			types.MemoryTrackerSizeCalculator(
				types.NewSizeCalculator(types.SizeCalculatorMaxTraversal(2)))))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, details, err := program.Eval(map[string]any{"a": []string{"a", "b", "c", "d", "e"}})
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	tracker := details.MemoryTracker()
	if tracker == nil {
		t.Fatal("EvalDetails.MemoryTracker() got nil, wanted tracker")
	}
	if !tracker.CalculationLimitExceeded() {
		t.Error("MemoryTracker.CalculationLimitExceeded() got false, wanted true")
	}
	if peak := details.PeakMemory(); peak == nil || *peak != math.MaxUint32 {
		t.Errorf("EvalDetails.PeakMemory() got %v, wanted MaxUint32", peak)
	}
}
