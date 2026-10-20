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

package cost_test

import (
	"fmt"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/ext"
)

// optionalCostEnv returns an environment with optional types, the regex library, and a few
// variables, at a cost model version and sizing strategy.
func optionalCostEnv(t *testing.T, version uint32, strategy cost.SizingStrategy) *cel.Env {
	t.Helper()
	env, err := cel.NewEnv(
		cel.OptionalTypes(), ext.Regex(),
		cel.Variable("s", cel.StringType),
		cel.Variable("o", cel.OptionalType(cel.StringType)),
		cel.Variable("l", cel.ListType(cel.StringType)),
		cel.CostModelVersion(version),
		cel.CostSizingStrategy(strategy),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	return env
}

// optionalCostHints bounds the variables, so that the estimates below are finite.
type optionalCostHints struct{}

func (optionalCostHints) EstimateSize(node cost.AstNode) *cost.SizeEstimate {
	switch strings.Join(node.Path(), ".") {
	case "s", "l.@items":
		sz := cost.RangedSizeEstimate(0, 20)
		return &sz
	case "l":
		sz := cost.RangedSizeEstimate(0, 5)
		return &sz
	case "o":
		sz := cost.RangedSizeEstimate(1, 21)
		return &sz
	}
	return nil
}

func (optionalCostHints) EstimateCallCost(string, string, *cost.AstNode, []cost.AstNode) *cost.CallEstimate {
	return nil
}

// optionalCost estimates and evaluates expr, and reports the estimate and the tracked cost.
func optionalCost(t *testing.T, env *cel.Env, expr string) (cost.CostEstimate, uint64) {
	t.Helper()
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
	}
	est, err := env.EstimateCost(ast, optionalCostHints{})
	if err != nil {
		t.Fatalf("env.EstimateCost(%q) failed: %v", expr, err)
	}
	prg, err := env.Program(ast, cel.CostTracking(nil))
	if err != nil {
		t.Fatalf("env.Program(%q) failed: %v", expr, err)
	}
	out, det, err := prg.Eval(map[string]any{
		"s": "abcdefghij",
		"o": types.OptionalOf(types.String("abcdefghij")),
		"l": []string{"abc", "defghij"},
	})
	if err != nil {
		t.Fatalf("prg.Eval(%q) failed: %v", expr, err)
	}
	if out.Value() != true {
		t.Fatalf("prg.Eval(%q) = %v, want true", expr, out)
	}
	return est, *det.ActualCost()
}

// TestOptionalCostSoundness checks that, under each strategy at cost model version 1, the tracked
// cost of an expression which creates, reads or compares optionals lies within its estimate.
//
// Version 0 is not checked: it reproduces cel-go v0.32.0, whose estimates were not always sound
// here. v0.32.0 estimated or() and orValue() at 1, as calls it had no model for, but charged them
// nothing, so its lower bound exceeds the cost of an expression whose other parts are exact.
func TestOptionalCostSoundness(t *testing.T) {
	long := strings.Repeat("a", 20)
	exprs := []string{
		`optional.none() == optional.none()`,
		fmt.Sprintf(`optional.of('%s') == optional.of('%s')`, long, long),
		`optional.of('') != optional.none()`,
		`optional.ofNonZeroValue('') == optional.none()`,
		`optional.ofNonZeroValue('abc') == optional.of('abc')`,
		`optional.of(optional.of('ab')) == optional.of(optional.of('ab'))`,
		`optional.of(optional.none()) != optional.none()`,
		`optional.of([1, 2, 3]).value().size() == 3`,
		`optional.of('abcdef').value() == 'abcdef'`,
		`optional.none().orValue('abc') == 'abc'`,
		`optional.of('abcdef').orValue('') == 'abcdef'`,
		`optional.none().or(optional.of('abc')) == optional.of('abc')`,
		`optional.of('abcdef').or(optional.none()) == optional.of('abcdef')`,
		`[1, 2, 3][?1] == optional.of(2)`,
		`[1, 2, 3][?5] == optional.none()`,
		`[1, 2, 3][?1].value() == 2`,
		`{'a': 'xyz'}[?'a'].orValue('') == 'xyz'`,
		`{'a': 'xyz'}[?'b'].orValue('default') == 'default'`,
		`[['ab', 'cd']][?0][?1] == optional.of('cd')`,
		`[['ab', 'cd']][?0][?1].value().size() == 2`,
		`l[?1].orValue('').size() == 7`,
		`l[?9].orValue('') == ''`,
		`o.orValue('') == s`,
		`o.value().size() == 10`,
		`o == optional.of(s)`,
		`regex.extract(s, 'c(.*)') == optional.of('defghij')`,
		`regex.extract(s, 'z') == optional.none()`,
		`regex.extract(s, 'c(.*)').orValue('').size() == 7`,
		`regex.extract(s, 'c(.*)').value().contains('j')`,
	}
	strategies := map[string]cost.SizingStrategy{
		"default":   cost.DefaultSizingStrategy(),
		"aggregate": cost.AggregateSizingStrategy(),
	}
	for name, strategy := range strategies {
		env := optionalCostEnv(t, 1, strategy)
		for _, expr := range exprs {
			t.Run(name+"/"+expr, func(t *testing.T) {
				est, actual := optionalCost(t, env, expr)
				if actual < est.Min || actual > est.Max {
					t.Errorf("actual cost %d outside estimate [%d, %d]", actual, est.Min, est.Max)
				}
			})
		}
	}
}

// TestOptionalCharge pins the charges which show that an optional is sized as 1 plus the value it
// holds from cost model version 1, and as 1 at version 0, as cel-go v0.32.0 charged it.
//
// A comparison charges 0.1 per unit of the smaller operand, rounded up, which makes the size of an
// optional of a 20 character string visible: 21 units cost 3, where the value alone, 20, would cost
// 2, and a size of 1 costs 1.
func TestOptionalCharge(t *testing.T) {
	long := strings.Repeat("a", 20)
	tests := []struct {
		expr            string
		wantV0, wantV1  uint64
		wantV1Aggregate uint64
	}{
		// Two optional.none() calls of 1, and a comparison of two optionals of size 1.
		{expr: `optional.none() == optional.none()`, wantV0: 3, wantV1: 3, wantV1Aggregate: 3},
		// Two optional.of() calls of 1, and a comparison of two optionals of size 21.
		{expr: fmt.Sprintf(`optional.of('%s') == optional.of('%s')`, long, long), wantV0: 3, wantV1: 5, wantV1Aggregate: 5},
		// regex.extract charges CallCost, its search, and its result. The search of the 10
		// character s for the 5 character 'c(.*)' is (10+1)*0.1 * (5+1)*0.25 = 1.65. The result
		// holds 'defghij', so it is 1 + 7 from version 1, and 1 at version 0: ceil(1 + 1.65 + 8) =
		// 11, or ceil(1 + 1.65 + 1) = 4. Reading s and calling hasValue() cost 1 each.
		{expr: `regex.extract(s, 'c(.*)').hasValue()`, wantV0: 6, wantV1: 13, wantV1Aggregate: 13},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			for _, c := range []struct {
				version  uint32
				strategy cost.SizingStrategy
				want     uint64
			}{
				{0, cost.DefaultSizingStrategy(), tc.wantV0},
				{1, cost.DefaultSizingStrategy(), tc.wantV1},
				{1, cost.AggregateSizingStrategy(), tc.wantV1Aggregate},
			} {
				if _, actual := optionalCost(t, optionalCostEnv(t, c.version, c.strategy), tc.expr); actual != c.want {
					t.Errorf("version %d, %T: cost %d, want %d", c.version, c.strategy, actual, c.want)
				}
			}
		})
	}
}
