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
	"math/bits"
	"slices"
)

// BitSet is an immutable set of (category, tag) bits. Category c is present when bit c of
// categories is set; its tag mask is stored at tags[rank], where rank is the number of
// present categories below c. Categories with an empty tag mask are never stored, so equal
// sets have identical representations.
type BitSet struct {
	categories uint64
	tags       []uint64
}

// IsEmpty reports whether no bits are set.
func (b BitSet) IsEmpty() bool {
	return b.categories == 0
}

// tagMask returns the tag mask for category index ci, or zero if the category is absent.
func (b BitSet) tagMask(ci int) uint64 {
	bit := uint64(1) << ci
	if b.categories&bit == 0 {
		return 0
	}
	return b.tags[bits.OnesCount64(b.categories&(bit-1))]
}

// Union returns the bits set in b or o.
func (b BitSet) Union(o BitSet) BitSet {
	return combine(b, o, b.categories|o.categories, func(x, y uint64) uint64 { return x | y })
}

// Intersect returns the bits set in both b and o.
func (b BitSet) Intersect(o BitSet) BitSet {
	return combine(b, o, b.categories&o.categories, func(x, y uint64) uint64 { return x & y })
}

// Difference returns the bits set in b but not in o.
func (b BitSet) Difference(o BitSet) BitSet {
	return combine(b, o, b.categories, func(x, y uint64) uint64 { return x &^ y })
}

// Intersects reports whether b and o share any bit, without allocating.
func (b BitSet) Intersects(o BitSet) bool {
	for rem := b.categories & o.categories; rem != 0; rem &= rem - 1 {
		ci := bits.TrailingZeros64(rem)
		if b.tagMask(ci)&o.tagMask(ci) != 0 {
			return true
		}
	}
	return false
}

// Contains reports whether every bit in o is also set in b.
func (b BitSet) Contains(o BitSet) bool {
	return o.Difference(b).IsEmpty()
}

// Equal reports whether b and o contain the same bits.
func (b BitSet) Equal(o BitSet) bool {
	return b.categories == o.categories && slices.Equal(b.tags, o.tags)
}

// Each calls fn for every set (category index, tag index) pair in ascending order until fn
// returns false.
func (b BitSet) Each(fn func(categoryIndex, tagIndex int) bool) {
	rank := 0
	for cats := b.categories; cats != 0; cats &= cats - 1 {
		ci := bits.TrailingZeros64(cats)
		for mask := b.tags[rank]; mask != 0; mask &= mask - 1 {
			if !fn(ci, bits.TrailingZeros64(mask)) {
				return
			}
		}
		rank++
	}
}

// combine applies op to the tag masks of each category in cats, dropping empty results.
func combine(a, b BitSet, cats uint64, op func(x, y uint64) uint64) BitSet {
	out := BitSet{tags: make([]uint64, 0, bits.OnesCount64(cats))}
	for rem := cats; rem != 0; rem &= rem - 1 {
		ci := bits.TrailingZeros64(rem)
		if mask := op(a.tagMask(ci), b.tagMask(ci)); mask != 0 {
			out.categories |= 1 << ci
			out.tags = append(out.tags, mask)
		}
	}
	return out
}
