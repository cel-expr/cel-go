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

package cel

import (
	"testing"

	"cel.dev/cel-go/common/ast"
)

// AssertNormalizedIDs is exported for use by tests in the cel_test package.
var AssertNormalizedIDs = assertNormalizedIDs

// assertNormalizedIDs verifies that the expression ids within an optimized AST are exactly the
// contiguous sequence 1..N, where N is the number of expression and entry nodes in the AST.
//
// The StaticOptimizer renumbers every node with a fresh, stable id generator after each
// optimization pass, so any gap or repeat indicates that some portion of the expression graph
// escaped renumbering.
func assertNormalizedIDs(t testing.TB, a *Ast) {
	t.Helper()
	collector := newIDCollector()
	ast.PostOrderVisit(a.NativeRep().Expr(), collector)
	ids := collector.IDs()
	for i, id := range ids {
		if id != int64(i+1) {
			t.Errorf("optimized AST ids %v, wanted contiguous ids 1..%d with no gaps or repeats", ids, len(ids))
			return
		}
	}
}
