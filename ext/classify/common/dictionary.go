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

// Package common provides the signal dictionary, bitset, and signal set primitives for content
// classification policies. Signals are organized into a two-level category:tag taxonomy, where
// each category and each tag within a category is assigned a bit position.
package common

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

// maxSlots is the maximum number of categories in a Dictionary, and of tags per category.
const maxSlots = 64

// Entry declares a single category:tag pair within a Dictionary.
type Entry struct {
	Category    string
	Tag         string
	Description string
}

// defaultEntries is the canonical policy taxonomy from the design doc (section 3.1).
// Order is significant: it determines bit positions.
var defaultEntries = []Entry{
	{"threat", "prompt-injection", "System prompt overrides, delimiter attacks, goal hijacking"},
	{"threat", "jailbreak", "Adversarial personas, roleplay bypasses, obfuscation"},
	{"threat", "typosquatting", "Lookalike or confusable domains targeting trusted hosts"},
	{"threat", "malicious-uri", "Phishing hosts, malicious domains, dangerous URI schemes"},
	{"threat", "exploit", "Shell injection, command execution payloads"},
	{"threat", "cyberattack", "Malware synthesis, vulnerability weaponization"},
	{"threat", "cbrn", "Chemical, biological, radiological, or nuclear weapons"},
	{"threat", "frontmatter-injection", "Suspicious frontmatter directives or delimiter spoofing"},
	{"safety", "hate-speech", "Hatred or disparagement targeting protected attributes"},
	{"safety", "harassment", "Bullying, intimidation, threats, or abusive insults"},
	{"safety", "sexual", "Sexually explicit content"},
	{"safety", "child-safety", "Child sexual exploitation and abuse"},
	{"safety", "violence", "Threats, incitement, or instructions for physical harm"},
	{"safety", "self-harm", "Suicide or self-injury encouragement or instructions"},
	{"safety", "defamation", "False statements damaging reputations"},
	{"privacy", "credit-card", "Payment card numbers"},
	{"privacy", "ssn", "Social security and national identity numbers"},
	{"privacy", "phone-number", "Telephone numbers"},
	{"privacy", "email", "Email addresses"},
	{"privacy", "secret-key", "Cryptographic keys, API tokens, passwords"},
	{"privacy", "pii", "General personal data"},
}

var defaultDictionary = func() *Dictionary {
	d, err := NewDictionary(defaultEntries...)
	if err != nil {
		panic(err)
	}
	return d
}()

// DefaultEntries returns a copy of the canonical taxonomy entries, suitable for extension:
//
//	d, err := NewDictionary(append(DefaultEntries(), Entry{Category: "org", Tag: "internal"})...)
func DefaultEntries() []Entry {
	return slices.Clone(defaultEntries)
}

// DefaultDictionary returns the Dictionary for the canonical taxonomy.
func DefaultDictionary() *Dictionary {
	return defaultDictionary
}

type category struct {
	name   string
	tags   []Entry
	tagIdx map[string]int
}

// mask returns the tag mask covering every registered tag in the category.
func (c *category) mask() uint64 {
	if len(c.tags) == maxSlots {
		return ^uint64(0)
	}
	return uint64(1)<<len(c.tags) - 1
}

// Dictionary assigns bit positions to categories and tags. It is immutable once constructed
// and safe for concurrent use.
type Dictionary struct {
	cats   []*category
	catIdx map[string]int
}

// NewDictionary builds a Dictionary from entries. Categories are numbered in order of first
// appearance and tags in order of appearance within their category. Names must be non-empty
// and consist of lowercase ASCII letters, digits, '-', '_', or '.'.
func NewDictionary(entries ...Entry) (*Dictionary, error) {
	d := &Dictionary{catIdx: make(map[string]int)}
	for _, e := range entries {
		if err := validateName(e.Category); err != nil {
			return nil, fmt.Errorf("invalid category %q: %w", e.Category, err)
		}
		if err := validateName(e.Tag); err != nil {
			return nil, fmt.Errorf("invalid tag %q: %w", e.Tag, err)
		}
		ci, found := d.catIdx[e.Category]
		if !found {
			if len(d.cats) == maxSlots {
				return nil, fmt.Errorf("too many categories: limit is %d", maxSlots)
			}
			ci = len(d.cats)
			d.catIdx[e.Category] = ci
			d.cats = append(d.cats, &category{name: e.Category, tagIdx: make(map[string]int)})
		}
		c := d.cats[ci]
		if _, dup := c.tagIdx[e.Tag]; dup {
			return nil, fmt.Errorf("duplicate entry %s:%s", e.Category, e.Tag)
		}
		if len(c.tags) == maxSlots {
			return nil, fmt.Errorf("too many tags in category %q: limit is %d", e.Category, maxSlots)
		}
		c.tagIdx[e.Tag] = len(c.tags)
		c.tags = append(c.tags, e)
	}
	return d, nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("empty name")
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return fmt.Errorf("invalid character %q", r)
		}
	}
	return nil
}

// Categories returns the category names in bit order.
func (d *Dictionary) Categories() []string {
	out := make([]string, len(d.cats))
	for i, c := range d.cats {
		out[i] = c.name
	}
	return out
}

// Tags returns the tag names registered for a category in bit order, or nil if unknown.
func (d *Dictionary) Tags(categoryName string) []string {
	ci, found := d.catIdx[categoryName]
	if !found {
		return nil
	}
	out := make([]string, len(d.cats[ci].tags))
	for i, e := range d.cats[ci].tags {
		out[i] = e.Tag
	}
	return out
}

// Category returns the BitSet covering every tag registered in the named category.
func (d *Dictionary) Category(categoryName string) (BitSet, error) {
	ci, found := d.catIdx[categoryName]
	if !found {
		return BitSet{}, fmt.Errorf("unknown category %q", categoryName)
	}
	return BitSet{categories: 1 << ci, tags: []uint64{d.cats[ci].mask()}}, nil
}

// Tag returns the BitSet for a single category:tag pair.
func (d *Dictionary) Tag(categoryName, tagName string) (BitSet, error) {
	ci, found := d.catIdx[categoryName]
	if !found {
		return BitSet{}, fmt.Errorf("unknown category %q", categoryName)
	}
	ti, found := d.cats[ci].tagIdx[tagName]
	if !found {
		return BitSet{}, fmt.Errorf("unknown tag %q in category %q", tagName, categoryName)
	}
	return BitSet{categories: 1 << ci, tags: []uint64{1 << ti}}, nil
}

// Parse resolves a spec string: a plain name such as 'privacy' denotes the whole category,
// while a ':'-delimited name such as 'privacy:ssn' denotes a single tag.
func (d *Dictionary) Parse(spec string) (BitSet, error) {
	if categoryName, tagName, hasTag := strings.Cut(spec, ":"); hasTag {
		return d.Tag(categoryName, tagName)
	}
	return d.Category(spec)
}

// Names returns the 'category:tag' names of the registered tags set in b, in bit order.
func (d *Dictionary) Names(b BitSet) []string {
	var out []string
	b.Each(func(ci, ti int) bool {
		if ci < len(d.cats) && ti < len(d.cats[ci].tags) {
			out = append(out, d.cats[ci].name+":"+d.cats[ci].tags[ti].Tag)
		}
		return true
	})
	return out
}

// CategoryNames returns the category names of the registered categories set in b, in bit order.
func (d *Dictionary) CategoryNames(b BitSet) []string {
	var out []string
	for rem := b.categories; rem != 0; rem &= rem - 1 {
		if ci := bits.TrailingZeros64(rem); ci < len(d.cats) {
			out = append(out, d.cats[ci].name)
		}
	}
	return out
}

// Scores returns the active 'category:tag' names and confidences in s, in bit order.
func (d *Dictionary) Scores(s *SignalSet) []SignalScore {
	if d == nil || s == nil || s.IsEmpty() {
		return nil
	}
	var out []SignalScore
	s.Bits().Each(func(ci, ti int) bool {
		if ci < len(d.cats) && ti < len(d.cats[ci].tags) {
			out = append(out, SignalScore{
				Name:       d.cats[ci].name + ":" + d.cats[ci].tags[ti].Tag,
				Confidence: s.BitConfidence(ci, ti),
			})
		}
		return true
	})
	return out
}
