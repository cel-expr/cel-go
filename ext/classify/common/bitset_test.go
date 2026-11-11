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
	"slices"
	"testing"
)

func TestBitSetOperations(t *testing.T) {
	d := DefaultDictionary()
	mustParse := func(specs ...string) BitSet {
		t.Helper()
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
	a := mustParse("privacy:ssn", "privacy:email", "threat:exploit")
	b := mustParse("privacy:ssn", "safety:harassment")

	tests := []struct {
		name string
		got  BitSet
		want []string
	}{
		{name: "union", got: a.Union(b), want: []string{"threat:exploit", "safety:harassment", "privacy:ssn", "privacy:email"}},
		{name: "intersect", got: a.Intersect(b), want: []string{"privacy:ssn"}},
		{name: "difference", got: a.Difference(b), want: []string{"threat:exploit", "privacy:email"}},
		{name: "difference reversed", got: b.Difference(a), want: []string{"safety:harassment"}},
		{name: "category minus tag", got: mustParse("privacy").Difference(mustParse("privacy:credit-card", "privacy:ssn", "privacy:phone-number", "privacy:email", "privacy:secret-key")), want: []string{"privacy:pii"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := d.Names(tc.got); !slices.Equal(got, tc.want) {
				t.Errorf("Names() got %v, wanted %v", got, tc.want)
			}
		})
	}
}

func TestBitSetPredicates(t *testing.T) {
	d := DefaultDictionary()
	privacy, _ := d.Parse("privacy")
	ssn, _ := d.Parse("privacy:ssn")
	email, _ := d.Parse("privacy:email")
	exploit, _ := d.Parse("threat:exploit")

	if !privacy.Intersects(ssn) || !ssn.Intersects(privacy) {
		t.Error("privacy and privacy:ssn should intersect")
	}
	if ssn.Intersects(email) {
		t.Error("privacy:ssn and privacy:email should not intersect")
	}
	if ssn.Intersects(exploit) {
		t.Error("privacy:ssn and threat:exploit should not intersect")
	}
	if !privacy.Contains(ssn) || ssn.Contains(privacy) {
		t.Error("privacy should contain privacy:ssn, but not the reverse")
	}
	if !privacy.Contains(BitSet{}) {
		t.Error("every set should contain the empty set")
	}
	// Disjoint tags in the same category intersect to empty, and the result is normalized so
	// that it compares equal to the zero value.
	empty := ssn.Intersect(email)
	if !empty.IsEmpty() || !empty.Equal(BitSet{}) {
		t.Errorf("ssn ∩ email got %+v, wanted the empty set", empty)
	}
	if !ssn.Union(email).Equal(email.Union(ssn)) {
		t.Error("Union should be commutative under Equal")
	}
	if ssn.Equal(email) {
		t.Error("distinct tags compared equal")
	}
}

func TestBitSetImmutability(t *testing.T) {
	d := DefaultDictionary()
	a, _ := d.Parse("privacy:ssn")
	b, _ := d.Parse("privacy:email")
	before := d.Names(a)
	_ = a.Union(b)
	_ = a.Intersect(b)
	_ = a.Difference(b)
	if got := d.Names(a); !slices.Equal(got, before) {
		t.Errorf("set operations mutated their receiver: got %v, wanted %v", got, before)
	}
}

func TestBitSetEachStopsEarly(t *testing.T) {
	all := mustParse(t, "threat", "safety")
	calls := 0
	all.Each(func(int, int) bool {
		calls++
		return calls < 3
	})
	if calls != 3 {
		t.Errorf("Each() made %d calls after fn returned false, wanted 3", calls)
	}
}
