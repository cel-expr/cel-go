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

package common

import (
	"math"
	"slices"
	"testing"
)

func mustParse(t *testing.T, specs ...string) BitSet {
	t.Helper()
	d := DefaultDictionary()
	var out BitSet
	for _, spec := range specs {
		b, err := d.Parse(spec)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", spec, err)
		}
		out = out.Union(b)
	}
	return out
}

func targets(t *testing.T, specs ...string) []BitSet {
	t.Helper()
	out := make([]BitSet, len(specs))
	for i, s := range specs {
		out[i] = mustParse(t, s)
	}
	return out
}

// sampleSignals constructs a SignalSet with signals at varying confidences.
func sampleSignals(t *testing.T) *SignalSet {
	t.Helper()
	return NewSignalBuilder().
		Add(mustParse(t, "safety:defamation"), 1.0).
		Add(mustParse(t, "threat:prompt-injection"), 0.88).
		Add(mustParse(t, "privacy:credit-card"), 0.40).
		Add(mustParse(t, "privacy:credit-card"), 0.65).
		Add(mustParse(t, "privacy:ssn"), 0.30).
		Build()
}

func TestSignalMatchConfidence(t *testing.T) {
	d := DefaultDictionary()
	ss := sampleSignals(t)
	tests := []struct {
		name    string
		kind    MatchKind
		targets []string
		want    float64
		tags    []string
	}{
		{"any/none", MatchAny, []string{"safety:violence"}, 0, nil},
		{"any/max across categories", MatchAny, []string{"safety", "threat"}, 1.0,
			[]string{"threat:prompt-injection", "safety:defamation"}},
		{"any/max in category", MatchAny, []string{"privacy"}, 0.65, []string{"privacy:credit-card", "privacy:ssn"}},
		{"any/single tag", MatchAny, []string{"privacy:ssn"}, 0.30, []string{"privacy:ssn"}},
		{"all/two tags", MatchAll, []string{"safety:defamation", "threat:prompt-injection"},
			0.88, []string{"threat:prompt-injection", "safety:defamation"}},
		{"all/category target", MatchAll, []string{"privacy", "threat"}, 0.65,
			[]string{"threat:prompt-injection", "privacy:credit-card", "privacy:ssn"}},
		{"all/missing target", MatchAll, []string{"threat", "safety:violence"}, 0, nil},
		{"all/empty targets", MatchAll, nil, 0, nil},
		{"only/extra bits", MatchOnly, []string{"privacy", "threat"}, 0, nil},
		{"only/covered", MatchOnly, []string{"privacy", "threat", "safety"}, 1.0,
			[]string{"threat:prompt-injection", "safety:defamation", "privacy:credit-card", "privacy:ssn"}},
		{"exactly/extra bits", MatchExactly, []string{"privacy", "threat"}, 0, nil},
		{"exactly/missing target", MatchExactly, []string{"privacy", "threat", "safety:defamation", "safety:violence"}, 0, nil},
		{"exactly/equal", MatchExactly, []string{"privacy", "threat", "safety:defamation"}, 0.65,
			[]string{"threat:prompt-injection", "safety:defamation", "privacy:credit-card", "privacy:ssn"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tgts := targets(t, tc.targets...)
			var m *SignalMatch
			switch tc.kind {
			case MatchAny:
				m = ss.HasAny(tgts...)
			case MatchAll:
				m = ss.HasAll(tgts...)
			case MatchOnly:
				m = ss.HasOnly(tgts...)
			case MatchExactly:
				m = ss.HasExactly(tgts...)
			}
			if m.Kind() != tc.kind || m.Signals() != ss {
				t.Errorf("Kind()/Signals() = %v/%p, want %v/%p", m.Kind(), m.Signals(), tc.kind, ss)
			}
			if got := m.Confidence(); got != tc.want {
				t.Errorf("Confidence() = %v, want %v", got, tc.want)
			}
			if m.IsMatch() != (tc.want > 0) {
				t.Errorf("IsMatch() = %v, want %v", m.IsMatch(), tc.want > 0)
			}
			if got := d.Names(m.Matched()); !slices.Equal(got, tc.tags) {
				t.Errorf("d.Names(m.Matched()) = %v, want %v", got, tc.tags)
			}
			if want := mustParse(t, tc.targets...); !m.Targets().Equal(want) {
				t.Errorf("Targets() = %v, want %v", m.Targets(), want)
			}
		})
	}
}

func TestSignalMatchCategoriesAndThreshold(t *testing.T) {
	d := DefaultDictionary()
	m := sampleSignals(t).HasAny(targets(t, "privacy", "safety")...)
	if got, want := d.CategoryNames(m.Matched()), []string{"safety", "privacy"}; !slices.Equal(got, want) {
		t.Errorf("CategoryNames(Matched()) = %v, want %v", got, want)
	}
	if got, want := d.Names(m.MatchedAbove(0.5)), []string{"safety:defamation", "privacy:credit-card"}; !slices.Equal(got, want) {
		t.Errorf("MatchedAbove(0.5) = %v, want %v", got, want)
	}
	if !m.MatchedAbove(1.01).IsEmpty() {
		t.Errorf("MatchedAbove(1.01) = %v, want empty", d.Names(m.MatchedAbove(1.01)))
	}
	none := sampleSignals(t).HasAny(mustParse(t, "safety:violence"))
	if d.CategoryNames(none.Matched()) != nil || !none.Matched().IsEmpty() {
		t.Errorf("non-match Categories/Matched() = %v/%v, want empty", d.CategoryNames(none.Matched()), none.Matched())
	}
}

func TestSignalBuilderAdd(t *testing.T) {
	d := DefaultDictionary()
	ss := NewSignalBuilder().
		Add(mustParse(t, "safety:violence"), 0).
		Add(mustParse(t, "safety:sexual"), -1).
		Add(mustParse(t, "safety:harassment"), math.NaN()).
		Add(mustParse(t, "safety:hate-speech"), 1.5).
		Add(mustParse(t, "safety:hate-speech"), 0.2).
		Build()
	if got, want := d.Names(ss.Bits()), []string{"safety:hate-speech"}; !slices.Equal(got, want) {
		t.Fatalf("Bits() = %v, want %v", got, want)
	}
	if got := ss.HasAny(mustParse(t, "safety")).Confidence(); got != 1.0 {
		t.Errorf("clamped confidence = %v, want 1.0", got)
	}
	if !NewSignalSet().IsEmpty() || ss.IsEmpty() {
		t.Error("IsEmpty() mismatch")
	}
}

func TestSignalBuilderSnapshot(t *testing.T) {
	b := NewSignalBuilder().Add(mustParse(t, "threat:exploit"), 0.5)
	first := b.Build()
	b.Add(mustParse(t, "threat:exploit"), 0.9).Add(mustParse(t, "safety:self-harm"), 1.0)
	if got := first.HasAny(mustParse(t, "threat")).Confidence(); got != 0.5 {
		t.Errorf("snapshot confidence = %v, want 0.5", got)
	}
	if first.Bits().Intersects(mustParse(t, "safety")) {
		t.Error("snapshot observed later signal")
	}
}

func TestSignalSetConfidenceLookup(t *testing.T) {
	d := DefaultDictionary()
	ss := sampleSignals(t)
	if ss.Confidence(mustParse(t, "safety:defamation")) != 1.0 {
		t.Errorf("Confidence(safety:defamation) = %v, want 1.0", ss.Confidence(mustParse(t, "safety:defamation")))
	}
	if got := ss.Confidence(mustParse(t, "privacy")); got != 0.65 {
		t.Errorf("Confidence(privacy) = %v, want 0.65", got)
	}
	if got := ss.Confidence(BitSet{}); got != 0 {
		t.Errorf("Confidence(empty) = %v, want 0", got)
	}
	var nilSet *SignalSet
	var nilDict *Dictionary
	if nilSet.Confidence(mustParse(t, "privacy")) != 0 ||
		nilSet.UncategorizedConfidence("vendor:tag") != 0 ||
		nilSet.BitConfidence(0, 0) != 0 ||
		!nilSet.Bits().IsEmpty() ||
		!nilSet.IsEmpty() ||
		d.Scores(nilSet) != nil ||
		nilDict.Scores(ss) != nil {
		t.Error("nil SignalSet or Dictionary returned non-zero values")
	}
}

func TestSignalSetScoresAndUncategorized(t *testing.T) {
	d := DefaultDictionary()
	ss := NewSignalBuilder().
		Add(mustParse(t, "safety:defamation"), 1.0).
		Add(mustParse(t, "privacy:credit-card"), 0.65).
		Add(mustParse(t, "privacy:ssn"), 0.30).
		AddUncategorized("llama-guard:unsafe", 0.80).
		AddUncategorized("llama-guard:unsafe", 0.92).
		AddUncategorized("model-armor:rai.dangerous", 0.55).
		AddUncategorized("", 0.9).
		AddUncategorized("orphan", 0.9).
		AddUncategorized("privacy", 0.9).
		AddUncategorized("Vendor:Toxicity", 0.9).
		AddUncategorized("vendor:a:b", 0.9).
		AddUncategorized("ignored:zero", 0).
		AddUncategorized("ignored:nan", math.NaN()).
		Build()

	if got := ss.UncategorizedConfidence("llama-guard:unsafe"); got != 0.92 {
		t.Errorf("UncategorizedConfidence(llama-guard:unsafe) = %v, want 0.92", got)
	}
	if got := ss.UncategorizedConfidence("privacy:credit-card"); got != 0 {
		t.Errorf("UncategorizedConfidence(privacy:credit-card) = %v, want 0", got)
	}
	wantUncat := []SignalScore{
		{Name: "llama-guard:unsafe", Confidence: 0.92},
		{Name: "model-armor:rai.dangerous", Confidence: 0.55},
	}
	if got := ss.Uncategorized(); !slices.Equal(got, wantUncat) {
		t.Errorf("Uncategorized() = %v, want %v", got, wantUncat)
	}
	if got := ss.UncategorizedAbove(0.70); !slices.Equal(got, wantUncat[:1]) {
		t.Errorf("UncategorizedAbove(0.70) = %v, want %v", got, wantUncat[:1])
	}
	wantScores := []SignalScore{
		{Name: "safety:defamation", Confidence: 1.0},
		{Name: "privacy:credit-card", Confidence: 0.65},
		{Name: "privacy:ssn", Confidence: 0.30},
	}
	if got := d.Scores(ss); !slices.Equal(got, wantScores) {
		t.Errorf("d.Scores(ss) = %v, want %v", got, wantScores)
	}
	if got := NewSignalSet().Uncategorized(); got != nil {
		t.Errorf("empty Uncategorized() = %v, want nil", got)
	}
}

func TestSignalSetWith(t *testing.T) {
	a := sampleSignals(t)
	b := NewSignalBuilder().
		Add(mustParse(t, "privacy:ssn"), 1.0).
		Add(mustParse(t, "threat:prompt-injection"), 0.95).
		Add(mustParse(t, "privacy:credit-card"), 0.10).
		AddUncategorized("llama-guard:unsafe", 0.75).
		Build()
	got := a.With(b)
	checks := map[string]float64{
		"threat:prompt-injection": 0.95,
		"privacy:credit-card":     0.65,
		"privacy:ssn":             1.0,
		"safety:defamation":       1.0,
	}
	for spec, want := range checks {
		if c := got.Confidence(mustParse(t, spec)); c != want {
			t.Errorf("With() %s confidence = %v, want %v", spec, c, want)
		}
	}
	if c := got.UncategorizedConfidence("llama-guard:unsafe"); c != 0.75 {
		t.Errorf("With() llama-guard:unsafe confidence = %v, want 0.75", c)
	}
	// The originals are unchanged.
	if c := a.HasAny(mustParse(t, "threat")).Confidence(); c != 0.88 {
		t.Errorf("original mutated: confidence = %v, want 0.88", c)
	}
	var nilSet *SignalSet
	if !nilSet.With(nil).IsEmpty() {
		t.Error("nil.With(nil) should be empty")
	}
	if nilSet.With(a).Confidence(mustParse(t, "threat")) != 0.88 {
		t.Error("nil.With(a) mismatch")
	}
}

func TestSignalSetWithout(t *testing.T) {
	d := DefaultDictionary()
	ss := sampleSignals(t)
	got := ss.Without(mustParse(t, "privacy:credit-card", "safety"))
	if want := []string{"threat:prompt-injection", "privacy:ssn"}; !slices.Equal(d.Names(got.Bits()), want) {
		t.Errorf("Without() Bits() = %v, want %v", d.Names(got.Bits()), want)
	}
	if c := got.HasAny(mustParse(t, "privacy")).Confidence(); c != 0.30 {
		t.Errorf("Without() privacy confidence = %v, want 0.30", c)
	}
	if c := ss.HasAny(mustParse(t, "privacy")).Confidence(); c != 0.65 {
		t.Errorf("original mutated: confidence = %v, want 0.65", c)
	}
	if !ss.Without(ss.Bits()).IsEmpty() {
		t.Error("Without(all) is not empty")
	}
	var nilSet *SignalSet
	if !nilSet.Without(ss.Bits()).IsEmpty() {
		t.Error("nil.Without(all) should be empty")
	}
}
