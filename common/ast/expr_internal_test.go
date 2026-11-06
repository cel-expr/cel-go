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

package ast

import (
	"testing"

	"cel.dev/cel-go/common/types"
)

// TestSetKindCaseRebuildsNavigableEntries ensures that SetKindCase does not retain the
// navigable entry implementations when the source expression is a NavigableExpr.
func TestSetKindCaseRebuildsNavigableEntries(t *testing.T) {
	fac := NewExprFactory()
	tests := []struct {
		name string
		src  Expr
	}{
		{
			name: "map",
			src: fac.NewMap(1, []EntryExpr{
				fac.NewMapEntry(2, fac.NewLiteral(3, types.String("a")), fac.NewIdent(4, "x"), false),
				fac.NewMapEntry(5, fac.NewLiteral(6, types.String("b")), fac.NewIdent(7, "y"), true),
			}),
		},
		{
			name: "struct",
			src: fac.NewStruct(1, "custom.StructType", []EntryExpr{
				fac.NewStructField(2, "a", fac.NewIdent(3, "x"), false),
				fac.NewStructField(4, "b", fac.NewIdent(5, "y"), true),
			}),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			nav := NavigateAST(NewAST(tc.src, nil))
			var navEntries []EntryExpr
			if nav.Kind() == MapKind {
				navEntries = nav.AsMap().Entries()
			} else {
				navEntries = nav.AsStruct().Fields()
			}
			// Sanity check the precondition: navigable entries use navigable kind cases.
			for _, e := range navEntries {
				switch e.(*entryExpr).entryExprKindCase.(type) {
				case navigableEntryImpl, navigableFieldImpl:
				default:
					t.Fatalf("navigable entry has kind case %T, wanted navigable impl", e.(*entryExpr).entryExprKindCase)
				}
			}

			target := fac.NewUnspecifiedExpr(100)
			target.SetKindCase(nav)

			var entries []EntryExpr
			if target.Kind() == MapKind {
				entries = target.AsMap().Entries()
			} else {
				entries = target.AsStruct().Fields()
			}
			if len(entries) != len(navEntries) {
				t.Fatalf("SetKindCase() got %d entries, wanted %d", len(entries), len(navEntries))
			}
			for i, e := range entries {
				want := navEntries[i]
				if e.ID() != want.ID() {
					t.Errorf("entry[%d].ID() got %d, wanted %d", i, e.ID(), want.ID())
				}
				// Note: navigable children are freshly wrapped on each call to Entries() / Fields(),
				// so compare the key and value expressions by id rather than by identity.
				switch kc := e.(*entryExpr).entryExprKindCase.(type) {
				case *baseMapEntry:
					wantMe := want.AsMapEntry()
					if kc.Key().ID() != wantMe.Key().ID() || kc.Value().ID() != wantMe.Value().ID() || kc.IsOptional() != wantMe.IsOptional() {
						t.Errorf("entry[%d] got %v, wanted %v", i, kc, wantMe)
					}
				case *baseStructField:
					wantSf := want.AsStructField()
					if kc.Name() != wantSf.Name() || kc.Value().ID() != wantSf.Value().ID() || kc.IsOptional() != wantSf.IsOptional() {
						t.Errorf("entry[%d] got %v, wanted %v", i, kc, wantSf)
					}
				default:
					t.Errorf("entry[%d] has kind case %T, wanted base implementation", i, kc)
				}
			}
		})
	}
}
