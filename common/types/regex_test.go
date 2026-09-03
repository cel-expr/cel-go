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
	"testing"
)

func TestRegexProgramSize(t *testing.T) {
	tests := []struct {
		pattern  string
		minSize  int
		hasError bool
	}{
		{pattern: "a", minSize: 1},
		{pattern: "el*", minSize: 3},
		{pattern: "(a|b)*[0-9]+", minSize: 5},
		{pattern: "a{3}", minSize: 3},
		{pattern: "(a|b){2,4}", minSize: 4},
		{pattern: "[0-9a-f]{8}-[0-9a-f]{4}", minSize: 12},
		{pattern: "(", hasError: true},
	}

	for _, tc := range tests {
		sz, err := RegexProgramSize(tc.pattern)
		if tc.hasError {
			if err == nil {
				t.Errorf("RegexProgramSize(%q) expected error, got nil", tc.pattern)
			}
			continue
		}
		if err != nil {
			t.Errorf("RegexProgramSize(%q) unexpected error: %v", tc.pattern, err)
			continue
		}
		if sz < tc.minSize {
			t.Errorf("RegexProgramSize(%q) = %d, expected >= %d", tc.pattern, sz, tc.minSize)
		}
	}
}

func TestCompileRegexWithLimit(t *testing.T) {
	tests := []struct {
		pattern  string
		limit    int
		hasError bool
	}{
		{pattern: "el*", limit: 10},
		{pattern: "el*", limit: 0},
		{pattern: "el*", limit: -1},
		{pattern: "(a|b)*[0-9]+", limit: 5, hasError: true},
		{pattern: "a{3}", limit: 10},
		{pattern: "[0-9a-f]{8}", limit: 5, hasError: true},
		{pattern: "(", limit: 10, hasError: true},
	}

	for _, tc := range tests {
		_, err := CompileRegexWithLimit(tc.pattern, tc.limit)
		if tc.hasError {
			if err == nil {
				t.Errorf("CompileRegexWithLimit(%q, %d) expected error, got nil", tc.pattern, tc.limit)
			}
		} else {
			if err != nil {
				t.Errorf("CompileRegexWithLimit(%q, %d) unexpected error: %v", tc.pattern, tc.limit, err)
			}
		}
	}
}
