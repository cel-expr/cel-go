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

package cost

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// The tests in this file check the algebraic laws and soundness property stated in the
// "Specification" section of docs/cost-model.md, the revisions listed in its "Model Revisions"
// section (TestVersion1Revisions), and the model validation rules (TestValidateRejections).

// algebraRanges are the argument intervals the algebraic laws are checked over. They include
// empty, singleton, and wide intervals, but not unknown ones, which are covered separately since
// saturation is not expected to preserve every law.
var algebraRanges = []SizeEstimate{
	FixedSizeEstimate(0),
	FixedSizeEstimate(1),
	FixedSizeEstimate(7),
	RangedSizeEstimate(0, 3),
	RangedSizeEstimate(2, 9),
	RangedSizeEstimate(10, 1000),
}

// algebraValues returns representative concrete values within an interval.
func algebraValues(r SizeEstimate) []uint64 {
	vals := []uint64{r.Min, r.Max}
	if r.Max-r.Min > 1 {
		vals = append(vals, r.Min+(r.Max-r.Min)/2)
	}
	return vals
}

// estimateWith estimates an expression as a compiled model evaluates it.
func estimateWith(e QuantityExpr, args ...SizeEstimate) SizeEstimate {
	return normalize(e).estimate(&testEvalContext{args: args})
}

// trackWith tracks an expression as a compiled model evaluates it.
func trackWith(e QuantityExpr, args ...uint64) uint64 {
	return normalize(e).track(&testTrackContext{args: args})
}

func boundsOf(s SizeEstimate) string {
	return fmt.Sprintf("[%d, %d]", s.Min, s.Max)
}

// forEachTriple invokes fn for every combination of three argument intervals and every
// combination of representative values within them.
func forEachTriple(fn func(ranges []SizeEstimate, vals []uint64)) {
	for _, ra := range algebraRanges {
		for _, rb := range algebraRanges {
			for _, rc := range algebraRanges {
				ranges := []SizeEstimate{ra, rb, rc}
				for _, va := range algebraValues(ra) {
					for _, vb := range algebraValues(rb) {
						for _, vc := range algebraValues(rc) {
							fn(ranges, []uint64{va, vb, vc})
						}
					}
				}
			}
		}
	}
}

// TestQuantityAlgebraLaws checks that pairs of algebraically equivalent expressions produce the
// same estimate bounds and the same tracked cost.
func TestQuantityAlgebraLaws(t *testing.T) {
	a, b, c := Arg(0), Arg(1), Arg(2)
	laws := []struct {
		name     string
		lhs, rhs QuantityExpr
	}{
		// Commutativity.
		{name: "Sum commutes", lhs: Sum(a, b), rhs: Sum(b, a)},
		{name: "Mul commutes", lhs: Mul(a, b), rhs: Mul(b, a)},
		{name: "Min commutes", lhs: Min(a, b), rhs: Min(b, a)},
		{name: "Max commutes", lhs: Max(a, b), rhs: Max(b, a)},
		{name: "Union commutes", lhs: Union(a, b), rhs: Union(b, a)},
		{name: "Sum of three permutes", lhs: Sum(a, b, c), rhs: Sum(c, a, b)},
		{name: "Mul of three permutes", lhs: Mul(a, b, c), rhs: Mul(c, a, b)},
		// Associativity.
		{name: "Sum associates", lhs: Sum(Sum(a, b), c), rhs: Sum(a, Sum(b, c))},
		{name: "Sum flattens", lhs: Sum(Sum(a, b), c), rhs: Sum(a, b, c)},
		{name: "Mul associates", lhs: Mul(Mul(a, b), c), rhs: Mul(a, Mul(b, c))},
		{name: "Mul flattens", lhs: Mul(Mul(a, b), c), rhs: Mul(a, b, c)},
		{name: "Min associates", lhs: Min(Min(a, b), c), rhs: Min(a, Min(b, c))},
		{name: "Max associates", lhs: Max(Max(a, b), c), rhs: Max(a, Max(b, c))},
		{name: "Union associates", lhs: Union(Union(a, b), c), rhs: Union(a, Union(b, c))},
		// Identities and annihilators.
		{name: "Sum identity", lhs: Sum(a, Const(0)), rhs: a},
		{name: "empty Sum", lhs: Sum(), rhs: Const(0)},
		{name: "Mul identity", lhs: Mul(a, Const(1)), rhs: a},
		{name: "empty Mul", lhs: Mul(), rhs: Const(1)},
		{name: "Mul annihilator", lhs: Mul(a, Const(0)), rhs: Const(0)},
		{name: "Scale identity", lhs: Scale(a, 1), rhs: a},
		{name: "Sub identity", lhs: Sub(a, Const(0)), rhs: a},
		// Distributivity of multiplication over addition.
		{name: "Mul distributes over Sum", lhs: Mul(a, Sum(b, c)), rhs: Sum(Mul(a, b), Mul(a, c))},
		{name: "Square expands", lhs: Square(Sum(a, b)), rhs: Mul(Sum(a, b), Sum(a, b))},
		// Truncated subtraction.
		{name: "Sub of Sum", lhs: Sub(a, Sum(b, c)), rhs: Sub(Sub(a, b), c)},
		// Lattice laws for Min and Max.
		{name: "Min idempotent", lhs: Min(a, a), rhs: a},
		{name: "Max idempotent", lhs: Max(a, a), rhs: a},
		{name: "Min absorbs Max", lhs: Min(a, Max(a, b)), rhs: a},
		{name: "Max absorbs Min", lhs: Max(a, Min(a, b)), rhs: a},
		{name: "Sum distributes over Max", lhs: Sum(a, Max(b, c)), rhs: Max(Sum(a, b), Sum(a, c))},
		{name: "Sum distributes over Min", lhs: Sum(a, Min(b, c)), rhs: Min(Sum(a, b), Sum(a, c))},
		{name: "Mul distributes over Max", lhs: Mul(a, Max(b, c)), rhs: Max(Mul(a, b), Mul(a, c))},
		// Scaling laws: factors are exact rationals, and normalization rounds each
		// product and each uniformly scaled sum once.
		{name: "Scale distributes over Sum", lhs: Scale(Sum(a, b), 0.1), rhs: Sum(Scale(a, 0.1), Scale(b, 0.1))},
		{name: "Scale factors out of Mul", lhs: Scale(Mul(a, b), 0.1), rhs: Mul(Scale(a, 0.1), b)},
		{name: "Scale composes", lhs: Scale(Scale(a, 0.25), 0.1), rhs: Scale(a, 0.025)},
		{name: "Scale composes to identity", lhs: Scale(Scale(a, 0.1), 10), rhs: a},
		{name: "products of scales round once", lhs: Mul(Scale(a, 0.1), Scale(b, 0.1)), rhs: Scale(Mul(a, b), 0.01)},
		// Integer scale factors are exact.
		{name: "integer Scale is Mul", lhs: Scale(a, 3), rhs: Mul(Const(3), a)},
		{name: "integer Scale distributes", lhs: Scale(Sum(a, b), 2), rhs: Sum(Scale(a, 2), Scale(b, 2))},
	}
	for _, law := range laws {
		law := law
		t.Run(law.name, func(t *testing.T) {
			forEachTriple(func(ranges []SizeEstimate, vals []uint64) {
				le, re := estimateWith(law.lhs, ranges...), estimateWith(law.rhs, ranges...)
				if le.Min != re.Min || le.Max != re.Max {
					t.Fatalf("estimate over %v: got %s, want %s", ranges, boundsOf(le), boundsOf(re))
				}
				lt, rt := trackWith(law.lhs, vals...), trackWith(law.rhs, vals...)
				if lt != rt {
					t.Fatalf("track over %v: got %d, want %d", vals, lt, rt)
				}
			})
		})
	}
}

// TestQuantityAlgebraSoundness checks that the cost tracked for concrete values always lies within
// the estimate computed for intervals containing those values.
func TestQuantityAlgebraSoundness(t *testing.T) {
	a, b, c := Arg(0), Arg(1), Arg(2)
	exprs := map[string]QuantityExpr{
		"Sum":        Sum(a, b, c),
		"Sub":        Sub(a, b),
		"Mul":        Mul(a, b, c),
		"Square":     Square(a),
		"Scale":      Scale(Sum(a, b), StringTraversalCostFactor),
		"Min":        Min(a, b),
		"Max":        Max(a, b),
		"Union":      Union(a, b, c),
		"AtMost":     AtMost(a),
		"AtLeastOne": AtLeastOneQuantity(Sub(a, b)),
		"Ranged":     Ranged(Min(a, b), Max(a, b)),
		"matches":    Mul(Scale(Sum(a, Const(1)), StringTraversalCostFactor), Scale(b, RegexStringLengthCostFactor)),
		"nested":     Sub(Mul(Sum(a, Const(2)), Max(b, c)), Min(a, c)),
	}
	for name, expr := range exprs {
		expr := expr
		t.Run(name, func(t *testing.T) {
			forEachTriple(func(ranges []SizeEstimate, vals []uint64) {
				est := estimateWith(expr, ranges...)
				got := trackWith(expr, vals...)
				if got > est.Max {
					t.Fatalf("track(%v) = %d exceeds estimate(%v) = %s", vals, got, ranges, boundsOf(est))
				}
				// Ranged and AtMost deliberately charge their upper bound at runtime, so only the
				// upper bound is checked for them.
				if name != "Ranged" && name != "AtMost" && got < est.Min {
					t.Fatalf("track(%v) = %d is below estimate(%v) = %s", vals, got, ranges, boundsOf(est))
				}
			})
		})
	}
}

// TestQuantityAlgebraMetadataCommutes checks that the key and element sizes attached to a combined
// estimate do not depend on the order of the operands.
func TestQuantityAlgebraMetadataCommutes(t *testing.T) {
	a, b := Arg(0), Arg(1)
	la := MapSizeEstimate(FixedSizeEstimate(2), FixedSizeEstimate(3), FixedSizeEstimate(5))
	lb := MapSizeEstimate(FixedSizeEstimate(4), RangedSizeEstimate(1, 8), FixedSizeEstimate(10))
	pairs := map[string][2]QuantityExpr{
		"Sum":    {Sum(a, b), Sum(b, a)},
		"Mul":    {Mul(a, b), Mul(b, a)},
		"Min":    {Min(a, b), Min(b, a)},
		"Max":    {Max(a, b), Max(b, a)},
		"Union":  {Union(a, b), Union(b, a)},
		"Ranged": {Ranged(a, b), Ranged(b, a)},
	}
	for name, pair := range pairs {
		lhs, rhs := estimateWith(pair[0], la, lb), estimateWith(pair[1], la, lb)
		if lhs.Key == nil || rhs.Key == nil || lhs.Elem == nil || rhs.Elem == nil {
			t.Errorf("%s: metadata dropped: %+v vs %+v", name, lhs, rhs)
			continue
		}
		if boundsOf(*lhs.Key) != boundsOf(*rhs.Key) || boundsOf(*lhs.Elem) != boundsOf(*rhs.Elem) {
			t.Errorf("%s: metadata depends on operand order: key %s vs %s, elem %s vs %s", name,
				boundsOf(*lhs.Key), boundsOf(*rhs.Key), boundsOf(*lhs.Elem), boundsOf(*rhs.Elem))
		}
	}
}

// TestVersion1Revisions checks the unified QuantityExpr properties and the BeforeVersion(1, ...)
// priors on StandardOverloadModels.
func TestVersion1Revisions(t *testing.T) {
	a, b := Arg(0), Arg(1)
	fixed := FixedSizeEstimate
	unknown := UnknownSizeEstimate()
	cases := []struct {
		name string
		got  func() string
		want string
	}{
		{
			name: "Scale distributes over Sum",
			got:  func() string { return fmt.Sprint(trackWith(Sum(Scale(a, 0.1), Scale(b, 0.1)), 1, 1)) },
			want: "1",
		},
		{
			name: "Scale factors out of Mul",
			got:  func() string { return fmt.Sprint(trackWith(Mul(Scale(a, 0.1), b), 1, 10)) },
			want: "1",
		},
		{
			name: "Scale composes",
			got:  func() string { return fmt.Sprint(trackWith(Scale(Scale(a, 0.1), 10), 5)) },
			want: "5",
		},
		{
			name: "Scale is exact above 2^53",
			got:  func() string { return fmt.Sprint(trackWith(Scale(a, 1.0), 1<<53+1)) },
			want: "9007199254740993",
		},
		{
			name: "decimal factors apply exactly",
			got:  func() string { return fmt.Sprint(trackWith(Mul(Scale(a, 0.1), Scale(b, 0.1)), 10, 10)) },
			want: "1",
		},
		{
			name: "fractional Scale preserves unknown",
			got:  func() string { return boundsOf(estimateWith(Scale(a, 0.1), unknown)) },
			want: boundsOf(unknown),
		},
		{
			name: "tracked Scale preserves unbounded",
			got:  func() string { return fmt.Sprint(trackWith(Scale(a, 0.5), math.MaxUint64)) },
			want: fmt.Sprint(uint64(math.MaxUint64)),
		},
		{
			name: "Ranged never inverts",
			got:  func() string { return boundsOf(estimateWith(Ranged(Arg(0), Arg(1)), fixed(5), fixed(3))) },
			want: "[3, 3]",
		},
		{
			name: "unknown element size survives merge",
			got: func() string {
				return boundsOf(estimateWith(ElemOf(Sum(a, b)), fixed(10), ListSizeEstimate(fixed(1), fixed(1))))
			},
			want: boundsOf(unknown),
		},
		{
			name: "constants contribute no metadata",
			got: func() string {
				return boundsOf(*estimateWith(Sum(a, Const(1)), ListSizeEstimate(fixed(2), fixed(3))).Elem)
			},
			want: "[3, 3]",
		},
		{
			name: "difference inherits only the minuend's elements",
			got: func() string {
				la, lb := ListSizeEstimate(fixed(4), fixed(10)), ListSizeEstimate(fixed(2), fixed(3))
				return boundsOf(*estimateWith(Sub(a, b), la, lb).Elem)
			},
			want: "[10, 10]",
		},
		{
			name: "Result is available to the cost estimate",
			got: func() string {
				m := Overload("f", EvalCost(Result()), ResultSize(Arg(0)))
				arg := NewAstNode(nil, nil, types.StringType, &SizeEstimate{Min: 5, Max: 5})
				est := m.FunctionEstimator()(nil, nil, []AstNode{arg})
				return boundsOf(SizeEstimate{Min: est.Min, Max: est.Max})
			},
			want: "[5, 5]",
		},
		{
			name: "IntArg of a non-literal is unbounded",
			got: func() string {
				ctx := &testEvalContext{receiver: &SizeEstimate{Min: 10, Max: 10}}
				return boundsOf(Mul(Target(), IntArg(0, 1)).estimate(ctx))
			},
			want: boundsOf(unknown),
		},
		{
			name: "IntArg of a non-literal at version 0 uses default",
			got: func() string {
				m := Overload("f", EvalCost(IntArg(0, 42)))
				est := m.FunctionEstimatorWithOptions(ModelVersion(0))(nil, nil, nil)
				return boundsOf(SizeEstimate{Min: est.Min, Max: est.Max})
			},
			want: "[42, 42]",
		},
		{
			name: "IntTarget of a non-literal at version 0 uses default",
			got: func() string {
				m := MemberOverload("f", EvalCost(IntTarget(42)))
				tgt := NewAstNode(nil, nil, types.IntType, nil)
				est := m.FunctionEstimatorWithOptions(ModelVersion(0))(nil, &tgt, nil)
				return boundsOf(SizeEstimate{Min: est.Min, Max: est.Max})
			},
			want: "[42, 42]",
		},
		{
			name: "IntArg of a literal is its value",
			got:  func() string { return boundsOf(IntArg(0, 1).estimate(&testEvalContext{argValues: []uint64{7}})) },
			want: "[7, 7]",
		},
		{
			name: "List reports the aggregate size of its list",
			got: func() string {
				m := Overload("l", EvalCost(Const(1)), ResultSize(List(Const(3), Const(5))))
				est := m.FunctionEstimatorWithOptions(WithSizingStrategy(AggregateSizingStrategy()))(nil, nil, nil)
				return boundsOf(*est.ResultSize)
			},
			want: "[16, 16]",
		},
		{
			name: "standard contains at version 0 rounds factors before multiplying",
			got: func() string {
				return fmt.Sprint(*StandardOverloadTrackersWithOptions(ModelVersion(0))[overloads.ContainsString](
					[]ref.Val{types.String("abcdefghijk"), types.String("abcdefghijk")}, nil))
			},
			want: "4",
		},
		{
			name: "standard contains at version 1 rounds product once",
			got: func() string {
				return fmt.Sprint(*StandardOverloadTrackersWithOptions(ModelVersion(1))[overloads.ContainsString](
					[]ref.Val{types.String("abcdefghijk"), types.String("abcdefghijk")}, nil))
			},
			want: "2",
		},
		{
			name: "standard optional.of result size omitted at version 0",
			got: func() string {
				arg := NewAstNode(nil, nil, types.StringType, &SizeEstimate{Min: 5, Max: 5})
				est := StandardOverloadEstimatorsWithOptions(ModelVersion(0))[overloads.OptionalOf](nil, nil, []AstNode{arg})
				return fmt.Sprint(est.ResultSize)
			},
			want: "<nil>",
		},
		{
			name: "standard optional.of result size present at version 1",
			got: func() string {
				arg := NewAstNode(nil, nil, types.StringType, &SizeEstimate{Min: 5, Max: 5})
				est := StandardOverloadEstimatorsWithOptions(ModelVersion(1))[overloads.OptionalOf](nil, nil, []AstNode{arg})
				return boundsOf(*est.ResultSize)
			},
			want: "[6, 6]",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got(); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestValidateRejections checks the model validation rules, which apply at every version.
func TestValidateRejections(t *testing.T) {
	tests := []struct {
		name  string
		model OverloadModel
		err   string
	}{
		{name: "zero factor", model: Overload("z", EvalCost(Scale(Arg(0), 0))), err: "finite and positive"},
		{name: "negative factor", model: Overload("n", EvalCost(Scale(Arg(0), -1))), err: "finite and positive"},
		{name: "NaN factor", model: Overload("nan", EvalCost(Scale(Arg(0), math.NaN()))), err: "finite and positive"},
		{name: "infinite factor", model: Overload("inf", EvalCost(Scale(Arg(0), math.Inf(1)))), err: "finite and positive"},
		{name: "inverted Ranged", model: Overload("r", EvalCost(Ranged(Const(5), Const(3)))), err: "exceeds upper bound"},
		{name: "self-referential size", model: Overload("s", EvalCost(Const(1)), ResultSize(Sum(Result(), Const(1)))), err: "references Result()"},
		{name: "global receiver", model: Overload("g", EvalCost(Sum(Target(), Arg(0)))), err: "use MemberOverload"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.model.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("Validate() = %v, want an error containing %q", err, tc.err)
			}
		})
	}
	ok := []OverloadModel{
		MemberOverload("m", EvalCost(Sum(Target(), Arg(0)))),
		Overload("c", EvalCost(StringScan(Result())), ResultSize(Arg(0))),
		Overload("rg", EvalCost(Ranged(Const(3), Const(5)))),
	}
	for _, m := range ok {
		if err := m.Validate(); err != nil {
			t.Errorf("Validate(%s) = %v, want nil", m.ID, err)
		}
	}
}
