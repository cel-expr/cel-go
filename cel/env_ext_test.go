// Copyright 2024 Google LLC
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

package cel_test

import (
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/ext"
)

func TestCatalogSuggestionsExtensionLibraries(t *testing.T) {
	tests := []struct {
		name             string
		opts             []cel.EnvOption
		expr             string
		wantErr          string
		wantNoSuggestion bool
	}{
		// 1. Strings extension library typos
		{
			name: "strings - charAt typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.chraAt(0)",
			wantErr: "undeclared reference to 'chraAt' (did you mean 'charAt'?)",
		},
		{
			name: "strings - indexOf typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.indeOf('h')",
			wantErr: "undeclared reference to 'indeOf' (did you mean 'indexOf'?)",
		},
		{
			name: "strings - lastIndexOf typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.lastIndeOf('h')",
			wantErr: "undeclared reference to 'lastIndeOf' (did you mean 'lastIndexOf'?)",
		},
		{
			name: "strings - lowerAscii typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.lowrAscii()",
			wantErr: "undeclared reference to 'lowrAscii' (did you mean 'lowerAscii'?)",
		},
		{
			name: "strings - upperAscii typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.upprAscii()",
			wantErr: "undeclared reference to 'upprAscii' (did you mean 'upperAscii'?)",
		},
		{
			name: "strings - replace typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.replce('a', 'b')",
			wantErr: "undeclared reference to 'replce' (did you mean 'replace'?)",
		},
		{
			name: "strings - split typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.splti(',')",
			wantErr: "undeclared reference to 'splti' (did you mean 'split'?)",
		},
		{
			name: "strings - substring typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.subtrng(0, 2)",
			wantErr: "undeclared reference to 'subtrng' (did you mean 'substring'?)",
		},
		{
			name: "strings - trim typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "'hello'.trimm()",
			wantErr: "undeclared reference to 'trimm' (did you mean 'trim'?)",
		},
		{
			name: "strings - join typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "['a', 'b'].joinn('-')",
			wantErr: "undeclared reference to 'joinn' (did you mean 'join'?)",
		},
		{
			name: "strings - strings.quote typo",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:    "strings.qoute('abc')",
			wantErr: "undeclared reference to 'strings.qoute' (did you mean 'strings.quote'?)",
		},

		// 2. Lists extension library typos
		{
			name: "lists - flatten typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "[[1, 2], [3]].flaten()",
			wantErr: "undeclared reference to 'flaten' (did you mean 'flatten'?)",
		},
		{
			name: "lists - distinct typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "[1, 2, 2].distnct()",
			wantErr: "undeclared reference to 'distnct' (did you mean 'distinct'?)",
		},
		{
			name: "lists - reverse typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "[1, 2, 3].revrse()",
			wantErr: "undeclared reference to 'revrse' (did you mean 'reverse'?)",
		},
		{
			name: "lists - sort typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "[3, 2, 1].srot()",
			wantErr: "undeclared reference to 'srot' (did you mean 'sort'?)",
		},
		{
			name: "lists - sortBy typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "[3, 2, 1].srotBy(x, x)",
			wantErr: "undeclared reference to 'srotBy' (did you mean 'sortBy'?)",
		},
		{
			name: "lists - slice typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "[1, 2, 3].slce(0, 1)",
			wantErr: "undeclared reference to 'slce' (did you mean 'slice'?)",
		},
		{
			name: "lists - lists.range typo",
			opts: []cel.EnvOption{
				ext.Lists(),
			},
			expr:    "lists.rnge(0, 10)",
			wantErr: "undeclared reference to 'lists.rnge' (did you mean 'lists.range'?)",
		},

		// 3. Sets extension library typos
		{
			name: "sets - sets.intersects typo",
			opts: []cel.EnvOption{
				ext.Sets(),
			},
			expr:    "sets.interscets([1], [1])",
			wantErr: "undeclared reference to 'sets.interscets' (did you mean 'sets.intersects'?)",
		},
		{
			name: "sets - sets.contains typo",
			opts: []cel.EnvOption{
				ext.Sets(),
			},
			expr:    "sets.contians([1], [1])",
			wantErr: "undeclared reference to 'sets.contians' (did you mean 'sets.contains'?)",
		},
		{
			name: "sets - sets.equivalent typo",
			opts: []cel.EnvOption{
				ext.Sets(),
			},
			expr:    "sets.equivlent([1], [1])",
			wantErr: "undeclared reference to 'sets.equivlent' (did you mean 'sets.equivalent'?)",
		},

		// 4. Encoders extension library typos
		{
			name: "encoders - base64.encode typo",
			opts: []cel.EnvOption{
				ext.Encoders(),
			},
			expr:    "base64.encde(b'hello')",
			wantErr: "undeclared reference to 'base64.encde' (did you mean 'base64.encode'?)",
		},
		{
			name: "encoders - base64.decode typo",
			opts: []cel.EnvOption{
				ext.Encoders(),
			},
			expr:    "base64.decde('aGVsbG8=')",
			wantErr: "undeclared reference to 'base64.decde' (did you mean 'base64.decode'?)",
		},

		// 5. Math extension library typos
		{
			name: "math - math.sqrt typo",
			opts: []cel.EnvOption{
				ext.Math(),
			},
			expr:    "math.sqr(81)",
			wantErr: "undeclared reference to 'math.sqr' (did you mean 'math.sqrt'?)",
		},
		{
			name: "math - math.ceil typo",
			opts: []cel.EnvOption{
				ext.Math(),
			},
			expr:    "math.ceill(1.5)",
			wantErr: "undeclared reference to 'math.ceill' (did you mean 'math.ceil'?)",
		},
		{
			name: "sets - sets.intersects prefix match",
			opts: []cel.EnvOption{
				ext.Sets(),
			},
			expr:    "sets.inter([1], [1])",
			wantErr: "undeclared reference to 'sets.inter' (did you mean 'sets.intersects'?)",
		},

		// 6. Combined libraries
		{
			name: "combined - optional + strings + lists (flatten typo)",
			opts: []cel.EnvOption{
				cel.OptionalTypes(),
				ext.Strings(),
				ext.Lists(),
			},
			expr:    "[[1, 2], [3]].flaten()",
			wantErr: "undeclared reference to 'flaten' (did you mean 'flatten'?)",
		},
		{
			name: "combined - optional + strings + lists (lowerAscii typo)",
			opts: []cel.EnvOption{
				cel.OptionalTypes(),
				ext.Strings(),
				ext.Lists(),
			},
			expr:    "'hello'.lower()",
			wantErr: "undeclared reference to 'lower' (did you mean 'lowerAscii'?)",
		},
		{
			name: "combined - optional + strings + lists (optional.of typo)",
			opts: []cel.EnvOption{
				cel.OptionalTypes(),
				ext.Strings(),
				ext.Lists(),
			},
			expr:    "optional.off('hello')",
			wantErr: "undeclared reference to 'optional.off' (did you mean 'optional.of'?)",
		},
		{
			name: "no suggestion - iteration budget exceeded",
			opts: []cel.EnvOption{
				ext.Strings(),
			},
			expr:             "long_unrecognized_custom_function_call(1)",
			wantErr:          "undeclared reference to 'long_unrecognized_custom_function_call'",
			wantNoSuggestion: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, err := cel.NewEnv(tc.opts...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			pAst, iss := e.Parse(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.expr, iss.Err())
			}
			_, iss = e.Check(pAst)
			if iss.Err() == nil {
				t.Fatalf("Check(%q) succeeded, wanted error containing %q", tc.expr, tc.wantErr)
			}
			if !strings.Contains(iss.Err().Error(), tc.wantErr) {
				t.Errorf("Check(%q) error = %q, want %q", tc.expr, iss.Err().Error(), tc.wantErr)
			}
			if tc.wantNoSuggestion {
				if strings.Contains(iss.Err().Error(), "did you mean") || strings.Contains(iss.Err().Error(), "enable with") {
					t.Errorf("Check(%q) error unexpectedly contained suggestion: %q", tc.expr, iss.Err().Error())
				}
			}
		})
	}
}
