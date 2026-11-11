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
	"maps"
	"math/bits"
	"slices"
	"strings"
)

// SignalScore pairs a signal or uncategorized signal name (such as "safety:violence"
// or "llama-guard:unsafe") with its confidence in (0.0, 1.0].
type SignalScore struct {
	Name       string  `json:"name" cel:"name"`
	Confidence float64 `json:"confidence" cel:"confidence"`
}

// scoreKey packs a (category index, tag index) pair into a single map key in 0..4095.
func scoreKey(ci, ti int) uint16 {
	return uint16(ci<<6 | ti)
}

// SignalSet is an immutable set of probabilistic signal bits and uncategorized signal scores.
// Every active signal bit and uncategorized signal carries a max-merged confidence in (0.0, 1.0].
//
// SignalSet is decoupled from Dictionary: set queries (HasAny, HasAll, HasOnly, HasExactly) and
// Confidence operate directly on BitSet masks, while Dictionary resolves string names to BitSet
// masks and formats BitSet masks back to category and 'category:tag' names.
type SignalSet struct {
	bits          BitSet
	scores        map[uint16]float64
	uncategorized map[string]float64
}

// NewSignalSet returns an empty SignalSet.
func NewSignalSet() *SignalSet {
	return &SignalSet{}
}

// Bits returns every active signal bit.
func (s *SignalSet) Bits() BitSet {
	if s == nil {
		return BitSet{}
	}
	return s.bits
}

// IsEmpty reports whether the set has no active signal bits.
func (s *SignalSet) IsEmpty() bool {
	return s == nil || s.bits.IsEmpty()
}

// Confidence returns the max-merged confidence over the active bits in target, or 0.0 when
// absent.
func (s *SignalSet) Confidence(target BitSet) float64 {
	if s == nil {
		return 0
	}
	return s.anyConfidence(target)
}

// UncategorizedConfidence returns the max-merged confidence of an uncategorized signal name,
// such as "llama-guard:unsafe", or 0.0 when absent. It never reports Dictionary signals, so a
// vendor signal cannot be mistaken for a canonical one.
func (s *SignalSet) UncategorizedConfidence(name string) float64 {
	if s == nil {
		return 0
	}
	return s.uncategorized[name]
}

// BitConfidence returns the confidence for a single (categoryIndex, tagIndex) bit.
func (s *SignalSet) BitConfidence(categoryIndex, tagIndex int) float64 {
	if s == nil {
		return 0
	}
	return s.scores[scoreKey(categoryIndex, tagIndex)]
}

// Uncategorized returns the uncategorized signals and their max-merged confidences, sorted by
// name.
func (s *SignalSet) Uncategorized() []SignalScore {
	return s.UncategorizedAbove(0)
}

// UncategorizedAbove returns the uncategorized signals whose confidence is >= threshold,
// sorted by name.
func (s *SignalSet) UncategorizedAbove(threshold float64) []SignalScore {
	if s == nil || len(s.uncategorized) == 0 {
		return nil
	}
	var out []SignalScore
	for _, name := range slices.Sorted(maps.Keys(s.uncategorized)) {
		if conf := s.uncategorized[name]; conf > 0 && conf >= threshold {
			out = append(out, SignalScore{Name: name, Confidence: conf})
		}
	}
	return out
}

// With returns the union of s and o: signal bits are OR'ed and confidences are max-merged.
func (s *SignalSet) With(o *SignalSet) *SignalSet {
	if s == nil && o == nil {
		return NewSignalSet()
	}
	if s == nil {
		return o.With(NewSignalSet())
	}
	b := &SignalBuilder{
		bits:          s.bits,
		scores:        maps.Clone(s.scores),
		uncategorized: maps.Clone(s.uncategorized),
	}
	if o != nil {
		for k, sc := range o.scores {
			b.addBit(int(k>>6), int(k&63), sc)
		}
		for name, sc := range o.uncategorized {
			b.AddUncategorized(name, sc)
		}
	}
	return b.Build()
}

// Without returns a copy of s with every bit in targets cleared.
func (s *SignalSet) Without(targets BitSet) *SignalSet {
	if s == nil {
		return NewSignalSet()
	}
	out := &SignalSet{
		bits:          s.bits.Difference(targets),
		uncategorized: maps.Clone(s.uncategorized),
	}
	for k, sc := range s.scores {
		if targets.tagMask(int(k>>6))&(1<<(k&63)) == 0 {
			if out.scores == nil {
				out.scores = make(map[uint16]float64)
			}
			out.scores[k] = sc
		}
	}
	return out
}

// SignalBuilder accumulates signals and their confidences into a SignalSet. It is not safe for
// concurrent use.
type SignalBuilder struct {
	bits          BitSet
	scores        map[uint16]float64
	uncategorized map[string]float64
}

// NewSignalBuilder returns an empty SignalBuilder.
func NewSignalBuilder() *SignalBuilder {
	return &SignalBuilder{}
}

// Add records a signal with the given confidence for every bit in bs. Confidences above 1.0
// are clamped; non-positive and NaN confidences are ignored.
func (b *SignalBuilder) Add(bs BitSet, confidence float64) *SignalBuilder {
	bs.Each(func(ci, ti int) bool {
		b.addBit(ci, ti, confidence)
		return true
	})
	return b
}

// AddUncategorized records a signal whose name is outside the Dictionary (for example,
// "llama-guard:unsafe" or "model-armor:rai.dangerous") without setting canonical category bits.
// The name must be a 'category:tag' pair whose parts follow the Dictionary's naming rules.
// Confidences above 1.0 are clamped; other names, and non-positive and NaN confidences, are
// ignored.
func (b *SignalBuilder) AddUncategorized(name string, confidence float64) *SignalBuilder {
	category, tag, _ := strings.Cut(name, ":")
	if validateName(category) != nil || validateName(tag) != nil || !(confidence > 0) {
		return b
	}
	confidence = min(confidence, 1.0)
	if b.uncategorized == nil {
		b.uncategorized = make(map[string]float64)
	}
	if confidence > b.uncategorized[name] {
		b.uncategorized[name] = confidence
	}
	return b
}

func (b *SignalBuilder) addBit(ci, ti int, confidence float64) {
	if !(confidence > 0) {
		return
	}
	confidence = min(confidence, 1.0)
	if b.scores == nil {
		b.scores = make(map[uint16]float64)
	}
	k := scoreKey(ci, ti)
	if confidence > b.scores[k] {
		if _, found := b.scores[k]; !found {
			b.bits = b.bits.Union(BitSet{categories: 1 << ci, tags: []uint64{1 << ti}})
		}
		b.scores[k] = confidence
	}
}

// Build returns an immutable SignalSet snapshot. The builder may continue to be used.
func (b *SignalBuilder) Build() *SignalSet {
	return &SignalSet{
		bits:          b.bits,
		scores:        maps.Clone(b.scores),
		uncategorized: maps.Clone(b.uncategorized),
	}
}

// MatchKind identifies the set query that produced a SignalMatch.
type MatchKind int

const (
	// MatchAny is satisfied when any target bit is present (max confidence).
	MatchAny MatchKind = iota
	// MatchAll is satisfied when every target is present (min of per-target confidence).
	MatchAll
	// MatchOnly is satisfied when only target bits are present, and at least one is.
	MatchOnly
	// MatchExactly is satisfied when every target is present and no other bits are.
	MatchExactly
)

// SignalMatch is the result of a set query over a SignalSet.
type SignalMatch struct {
	signals    *SignalSet
	kind       MatchKind
	targets    BitSet
	matched    BitSet
	confidence float64
}

// HasAny matches when at least one bit in any target is present.
func (s *SignalSet) HasAny(targets ...BitSet) *SignalMatch { return s.query(MatchAny, targets) }

// HasAll matches when every target has at least one bit present. Each target is satisfied by
// any of its bits, so a whole-category target is satisfied by any tag in that category.
func (s *SignalSet) HasAll(targets ...BitSet) *SignalMatch { return s.query(MatchAll, targets) }

// HasOnly matches when no bits outside the targets are present and at least one target bit is.
func (s *SignalSet) HasOnly(targets ...BitSet) *SignalMatch { return s.query(MatchOnly, targets) }

// HasExactly matches when every target is present and no bits outside the targets are.
func (s *SignalSet) HasExactly(targets ...BitSet) *SignalMatch { return s.query(MatchExactly, targets) }

// query evaluates a set query. An empty target list never matches.
func (s *SignalSet) query(kind MatchKind, targets []BitSet) *SignalMatch {
	var mask BitSet
	for _, m := range targets {
		mask = mask.Union(m)
	}
	m := &SignalMatch{signals: s, kind: kind, targets: mask}
	if s == nil || len(targets) == 0 {
		return m
	}
	if (kind == MatchOnly || kind == MatchExactly) && !s.bits.Difference(mask).IsEmpty() {
		return m
	}
	switch kind {
	case MatchAny, MatchOnly:
		m.confidence = s.anyConfidence(mask)
	case MatchAll, MatchExactly:
		m.confidence = 1.0
		for _, target := range targets {
			if m.confidence = min(m.confidence, s.anyConfidence(target)); m.confidence == 0 {
				break
			}
		}
	}
	if m.confidence > 0 {
		m.matched = s.bits.Intersect(mask)
	}
	return m
}

// anyConfidence returns the max confidence over the active bits in mask.
func (s *SignalSet) anyConfidence(mask BitSet) float64 {
	if !s.bits.Intersects(mask) {
		return 0
	}
	best := 0.0
	for rem := s.bits.categories & mask.categories; rem != 0; rem &= rem - 1 {
		ci := bits.TrailingZeros64(rem)
		for tags := s.bits.tagMask(ci) & mask.tagMask(ci); tags != 0; tags &= tags - 1 {
			best = max(best, s.scores[scoreKey(ci, bits.TrailingZeros64(tags))])
		}
	}
	return best
}

// Kind returns the query kind.
func (m *SignalMatch) Kind() MatchKind { return m.kind }

// Signals returns the SignalSet that was queried.
func (m *SignalMatch) Signals() *SignalSet { return m.signals }

// Targets returns the union of the queried target bits.
func (m *SignalMatch) Targets() BitSet { return m.targets }

// Matched returns the active target bits when the query matches, and an empty set otherwise.
func (m *SignalMatch) Matched() BitSet { return m.matched }

// Confidence returns the aggregate match confidence in [0.0, 1.0].
func (m *SignalMatch) Confidence() float64 { return m.confidence }

// IsMatch reports whether Confidence() > 0.
func (m *SignalMatch) IsMatch() bool { return m.confidence > 0 }

// MatchedAbove returns the subset of Matched() bits whose confidence is >= threshold.
func (m *SignalMatch) MatchedAbove(threshold float64) BitSet {
	var out BitSet
	m.matched.Each(func(ci, ti int) bool {
		if m.signals.BitConfidence(ci, ti) >= threshold {
			out = out.Union(BitSet{categories: 1 << ci, tags: []uint64{1 << ti}})
		}
		return true
	})
	return out
}
