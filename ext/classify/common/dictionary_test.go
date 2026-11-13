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
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestDefaultDictionary(t *testing.T) {
	d := DefaultDictionary()
	wantCats := []string{"threat", "safety", "privacy"}
	if got := d.Categories(); !slices.Equal(got, wantCats) {
		t.Errorf("Categories() got %v, wanted %v", got, wantCats)
	}
	wantCounts := map[string]int{"threat": 8, "safety": 7, "privacy": 6}
	for cat, want := range wantCounts {
		if got := len(d.Tags(cat)); got != want {
			t.Errorf("len(Tags(%q)) got %d, wanted %d", cat, got, want)
		}
	}
	if got := d.Tags("unknown"); got != nil {
		t.Errorf("Tags(unknown) got %v, wanted nil", got)
	}
	// Every canonical entry must round-trip through Parse and Names.
	for _, e := range DefaultEntries() {
		spec := e.Category + ":" + e.Tag
		b, err := d.Parse(spec)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", spec, err)
		}
		if got := d.Names(b); !slices.Equal(got, []string{spec}) {
			t.Errorf("Names(Parse(%q)) got %v", spec, got)
		}
	}
}

func TestDefaultEntriesIsCopy(t *testing.T) {
	entries := DefaultEntries()
	entries[0].Tag = "mutated"
	if got := DefaultEntries()[0].Tag; got != "prompt-injection" {
		t.Errorf("DefaultEntries() was mutated through a returned copy: %q", got)
	}
}

func TestParse(t *testing.T) {
	d := DefaultDictionary()
	tests := []struct {
		spec string
		want []string
	}{
		{spec: "privacy:ssn", want: []string{"privacy:ssn"}},
		{spec: "privacy", want: d.qualified("privacy")},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			b, err := d.Parse(tc.spec)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.spec, err)
			}
			if got := d.Names(b); !slices.Equal(got, tc.want) {
				t.Errorf("Names() got %v, wanted %v", got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	d := DefaultDictionary()
	tests := []struct {
		spec    string
		wantErr string
	}{
		{spec: "", wantErr: "unknown category"},
		{spec: "unknown", wantErr: "unknown category"},
		{spec: ":ssn", wantErr: "unknown category"},
		{spec: "privacy:", wantErr: "unknown tag"},
		{spec: "privacy:unknown", wantErr: "unknown tag"},
		{spec: "privacy:ssn:extra", wantErr: "unknown tag"},
		{spec: "Privacy", wantErr: "unknown category"},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			_, err := d.Parse(tc.spec)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Parse(%q) got error %v, wanted %q", tc.spec, err, tc.wantErr)
			}
		})
	}
}

func TestNewDictionaryErrors(t *testing.T) {
	tooManyCats := make([]Entry, maxSlots+1)
	for i := range tooManyCats {
		tooManyCats[i] = Entry{Category: fmt.Sprintf("c%d", i), Tag: "t"}
	}
	tooManyTags := make([]Entry, maxSlots+1)
	for i := range tooManyTags {
		tooManyTags[i] = Entry{Category: "c", Tag: fmt.Sprintf("t%d", i)}
	}
	tests := []struct {
		name    string
		entries []Entry
		wantErr string
	}{
		{name: "empty category", entries: []Entry{{Category: "", Tag: "t"}}, wantErr: "empty name"},
		{name: "empty tag", entries: []Entry{{Category: "c", Tag: ""}}, wantErr: "empty name"},
		{name: "uppercase", entries: []Entry{{Category: "Privacy", Tag: "t"}}, wantErr: "invalid character"},
		{name: "delimiter in tag", entries: []Entry{{Category: "c", Tag: "a:b"}}, wantErr: "invalid character"},
		{name: "markup in tag", entries: []Entry{{Category: "c", Tag: "<b>"}}, wantErr: "invalid character"},
		{name: "duplicate", entries: []Entry{{Category: "c", Tag: "t"}, {Category: "c", Tag: "t"}}, wantErr: "duplicate entry c:t"},
		{name: "too many categories", entries: tooManyCats, wantErr: "too many categories"},
		{name: "too many tags", entries: tooManyTags, wantErr: "too many tags"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDictionary(tc.entries...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("NewDictionary() got error %v, wanted %q", err, tc.wantErr)
			}
		})
	}
}

func TestNewDictionaryCapacity(t *testing.T) {
	var entries []Entry
	for c := 0; c < maxSlots; c++ {
		for tg := 0; tg < maxSlots; tg++ {
			entries = append(entries, Entry{Category: fmt.Sprintf("c%d", c), Tag: fmt.Sprintf("t%d", tg)})
		}
	}
	d, err := NewDictionary(entries...)
	if err != nil {
		t.Fatalf("NewDictionary() with 64x64 entries failed: %v", err)
	}
	var all BitSet
	for _, cat := range d.Categories() {
		b, err := d.Category(cat)
		if err != nil {
			t.Fatalf("Category(%q) failed: %v", cat, err)
		}
		all = all.Union(b)
	}
	if got := len(d.Names(all)); got != maxSlots*maxSlots {
		t.Errorf("len(Names(all)) got %d, wanted %d", got, maxSlots*maxSlots)
	}
	last, err := d.Tag("c63", "t63")
	if err != nil {
		t.Fatalf("Tag(c63, t63) failed: %v", err)
	}
	if !all.Contains(last) {
		t.Error("full set does not contain the highest (category, tag) bit")
	}
}

func TestExtendedDictionary(t *testing.T) {
	d, err := NewDictionary(append(DefaultEntries(), Entry{Category: "org", Tag: "internal"})...)
	if err != nil {
		t.Fatalf("NewDictionary() failed: %v", err)
	}
	b, err := d.Parse("org:internal")
	if err != nil {
		t.Fatalf("Parse(org:internal) failed: %v", err)
	}
	if got := d.Names(b); !slices.Equal(got, []string{"org:internal"}) {
		t.Errorf("Names() got %v", got)
	}
	if _, err := DefaultDictionary().Parse("org:internal"); err == nil {
		t.Error("extending a dictionary leaked into DefaultDictionary()")
	}
}

// qualified returns the 'category:tag' names for every tag in a category.
func (d *Dictionary) qualified(categoryName string) []string {
	var out []string
	for _, tag := range d.Tags(categoryName) {
		out = append(out, categoryName+":"+tag)
	}
	return out
}
