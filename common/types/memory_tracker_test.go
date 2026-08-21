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

package types

import (
	"math"
	"strings"
	"testing"

	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

func TestMemoryTrackerTrack(t *testing.T) {
	adapter := DefaultTypeAdapter

	t.Run("single_value_watermark", func(t *testing.T) {
		tracker := NewMemoryTracker()
		list := NewRefValList(adapter, []ref.Val{Int(1), Int(2)})
		if got := tracker.Track(list); got != 3 {
			t.Errorf("Track() got %d, want 3", got)
		}
		if got := tracker.Peak(); got != 3 {
			t.Errorf("Peak() got %d, want 3", got)
		}
	})

	t.Run("consecutive_watermarks", func(t *testing.T) {
		tracker := NewMemoryTracker()
		list := NewRefValList(adapter, []ref.Val{Int(1), Int(2)})
		if got := tracker.Track(list); got != 3 {
			t.Errorf("Track(list) got %d, want 3", got)
		}
		arg := Int(0)
		if got := tracker.Track(arg); got != 1 {
			t.Errorf("Track(arg) got %d, want 1", got)
		}
		if got := tracker.Peak(); got != 3 {
			t.Errorf("Peak() got %d, want 3", got)
		}
	})

	t.Run("peak_retains_max", func(t *testing.T) {
		tracker := NewMemoryTracker()
		tracker.Track(NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3)}))
		tracker.Track(Int(1))
		if got := tracker.Peak(); got != 4 {
			t.Errorf("Peak() got %d, want 4", got)
		}
	})

	t.Run("call_output_watermark", func(t *testing.T) {
		// Models the output of a + a, a string twice the size of its inputs.
		tracker := NewMemoryTracker()
		in := String(strings.Repeat("a", 50))
		tracker.Track(in)
		out := String(strings.Repeat("a", 100))
		tracker.Track(out)
		if got := tracker.Peak(); got != 10 {
			t.Errorf("Peak() got %d, want 10 (100-char output at 10 chars per unit)", got)
		}
	})

	t.Run("saturating_value", func(t *testing.T) {
		tracker := NewMemoryTracker()
		big := customSizerVal(math.MaxUint32)
		if got := tracker.Track(big); got != math.MaxUint32 {
			t.Errorf("Track() got %d, want MaxUint32", got)
		}
		if got := tracker.Peak(); got != math.MaxUint32 {
			t.Errorf("Peak() got %d, want MaxUint32", got)
		}
		if tracker.CalculationLimitExceeded() {
			t.Error("CalculationLimitExceeded() got true, want false for pure saturation")
		}
	})

	t.Run("calculation_limit_exceeded", func(t *testing.T) {
		tracker := NewMemoryTracker(
			MemoryTrackerSizeCalculator(NewSizeCalculator(SizeCalculatorMaxTraversal(2))))
		list := NewRefValList(adapter, []ref.Val{Int(1), Int(2), Int(3)})
		if got := tracker.Track(list); got != math.MaxUint32 {
			t.Errorf("Track() got %d, want MaxUint32", got)
		}
		if !tracker.CalculationLimitExceeded() {
			t.Error("CalculationLimitExceeded() got false, want true")
		}
	})
}

func TestMemoryTrackerSample(t *testing.T) {
	adapter := DefaultTypeAdapter

	t.Run("default_interval_samples_every_value", func(t *testing.T) {
		tracker := NewMemoryTracker()
		for i := 0; i < 3; i++ {
			if got := tracker.Sample(1, Int(i)); got != 1 {
				t.Errorf("Sample() got %d, want 1", got)
			}
		}
		if got := tracker.Peak(); got != 1 {
			t.Errorf("Peak() got %d, want 1", got)
		}
	})

	t.Run("interval_skips_intermediate_samples", func(t *testing.T) {
		tracker := NewMemoryTracker(MemoryTrackerSampleInterval(3))
		list := NewRefValList(adapter, []ref.Val{Int(1), Int(2)})
		if got := tracker.Sample(1, list); got != 3 {
			t.Errorf("Sample() #1 got %d, want 3 (computed on first observation)", got)
		}
		if got := tracker.Sample(1, list); got != 0 {
			t.Errorf("Sample() #2 got %d, want 0 (skipped)", got)
		}
		if got := tracker.Sample(1, list); got != 3 {
			t.Errorf("Sample() #3 got %d, want 3 (computed)", got)
		}
		if got := tracker.Peak(); got != 3 {
			t.Errorf("Peak() got %d, want 3", got)
		}
	})

	t.Run("per_id_tracking", func(t *testing.T) {
		tracker := NewMemoryTracker(MemoryTrackerSampleInterval(2))
		list := NewRefValList(adapter, []ref.Val{Int(1), Int(2)})
		// id 1: sample 1 (computed on first observation)
		if got := tracker.Sample(1, list); got != 3 {
			t.Errorf("Sample(1) #1 got %d, want 3", got)
		}
		// id 2: sample 1 (computed on first observation)
		if got := tracker.Sample(2, list); got != 3 {
			t.Errorf("Sample(2) #1 got %d, want 3", got)
		}
		// id 1: sample 2 (computed, multiple of 2)
		if got := tracker.Sample(1, list); got != 3 {
			t.Errorf("Sample(1) #2 got %d, want 3", got)
		}
		// id 2: sample 2 (computed, multiple of 2)
		if got := tracker.Sample(2, list); got != 3 {
			t.Errorf("Sample(2) #2 got %d, want 3", got)
		}
		// id 1: sample 3 (skipped)
		if got := tracker.Sample(1, list); got != 0 {
			t.Errorf("Sample(1) #3 got %d, want 0", got)
		}
	})

	t.Run("zero_interval_clamped_to_one", func(t *testing.T) {
		tracker := NewMemoryTracker(MemoryTrackerSampleInterval(0))
		if got := tracker.Sample(1, Int(1)); got != 1 {
			t.Errorf("Sample() got %d, want 1", got)
		}
	})
}

func TestMemoryTrackerLimit(t *testing.T) {
	t.Run("no_limit", func(t *testing.T) {
		tracker := NewMemoryTracker()
		tracker.Track(customSizerVal(math.MaxUint32))
		if tracker.ExceedsLimit() {
			t.Error("ExceedsLimit() got true, want false when no limit configured")
		}
	})

	t.Run("under_limit", func(t *testing.T) {
		tracker := NewMemoryTracker(MemoryTrackerLimit(10))
		tracker.Track(customSizerVal(10))
		if tracker.ExceedsLimit() {
			t.Error("ExceedsLimit() got true, want false at exactly the limit")
		}
	})

	t.Run("over_limit", func(t *testing.T) {
		tracker := NewMemoryTracker(MemoryTrackerLimit(10))
		tracker.Track(customSizerVal(11))
		if !tracker.ExceedsLimit() {
			t.Error("ExceedsLimit() got false, want true")
		}
	})
}

func TestMemoryTrackerVersion(t *testing.T) {
	if got := NewMemoryTracker().Version(); got != 0 {
		t.Errorf("Version() got %d, want 0", got)
	}
}

// Interface conformance check: the tracker's calculator remains usable as an AggregateSizer
// by visitor implementations.
var _ ref.Val = customSizerVal(0)
var _ traits.Sizer = customSizerVal(0)
