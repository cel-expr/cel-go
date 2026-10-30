// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cost

import (
	"strings"
	"testing"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

type noopSizingStrategy struct{}

func (noopSizingStrategy) EstimateSize(_ EstimateContext, _ AstNode) (SizeEstimate, bool) {
	return SizeEstimate{}, false
}

func (noopSizingStrategy) TrackSize(_ TrackContext, _ ref.Val) (uint64, bool) {
	return 0, false
}

type stubEstimator struct {
	sizeEst *SizeEstimate
	callEst *CallEstimate
}

func (s stubEstimator) EstimateSize(_ AstNode) *SizeEstimate {
	return s.sizeEst
}

func (s stubEstimator) EstimateCallCost(_, _ string, _ *AstNode, _ []AstNode) *CallEstimate {
	return s.callEst
}

func TestModelVersionOfAndVersionedEstimator(t *testing.T) {
	if got := ModelVersionOf(nil); got != defaultModelVersion {
		t.Errorf("ModelVersionOf(nil) = %v, want %v", got, defaultModelVersion)
	}
	if got := ModelVersionOf(stubEstimator{}); got != defaultModelVersion {
		t.Errorf("ModelVersionOf(stubEstimator{}) = %v, want %v", got, defaultModelVersion)
	}

	cNil := mustNewCoster(t, nil, nil, ModelVersion(0))
	estNil := cNil.getEstimator()
	if got := ModelVersionOf(estNil); got != 0 {
		t.Errorf("ModelVersionOf(cNil.getEstimator()) = %v, want 0", got)
	}
	if got := estNil.EstimateSize(nil); got != nil {
		t.Errorf("estNil.EstimateSize(nil) = %v, want nil", got)
	}
	if got := estNil.EstimateCallCost("f", "f_id", nil, nil); got != nil {
		t.Errorf("estNil.EstimateCallCost() = %v, want nil", got)
	}

	sz := FixedSizeEstimate(7)
	call := NewCallEstimate(FixedCostEstimate(3), &sz)
	cDel := mustNewCoster(t, nil, stubEstimator{sizeEst: &sz, callEst: call}, ModelVersion(1))
	estDel := cDel.getEstimator()
	if got := ModelVersionOf(estDel); got != 1 {
		t.Errorf("ModelVersionOf(cDel.getEstimator()) = %v, want 1", got)
	}
	if got := estDel.EstimateSize(nil); got == nil || *got != sz {
		t.Errorf("estDel.EstimateSize(nil) = %v, want %v", got, sz)
	}
	if got := estDel.EstimateCallCost("f", "f_id", nil, nil); got == nil || *got != *call {
		t.Errorf("estDel.EstimateCallCost() = %v, want %v", got, call)
	}
}

func TestCosterHelpersAndEvalContextSize(t *testing.T) {
	fac := ast.NewExprFactory()
	identExpr := fac.NewIdent(1, "x")
	checked := ast.NewCheckedAST(ast.NewAST(identExpr, nil), map[int64]*types.Type{1: types.StringType}, nil)

	// When a custom noop strategy returns false for a variable, sizeOrUnknown returns UnknownSizeEstimate.
	c := mustNewCoster(t, checked, nil, WithSizingStrategy(noopSizingStrategy{}))
	if got := c.sizeOrUnknown(identExpr); got != UnknownSizeEstimate() {
		t.Errorf("sizeOrUnknown(unhinted) = %v, want %v", got, UnknownSizeEstimate())
	}

	// Once a size is recorded on coster, estimatorEvalContext.Size resolves it via e.coster.computeSize.
	expected := FixedSizeEstimate(42)
	c.setSize(identExpr, &expected)
	if got := c.sizeOrUnknown(identExpr); got != expected {
		t.Errorf("sizeOrUnknown(cached) = %v, want %v", got, expected)
	}
	ctx := c.newEstimateContext()
	node := astNode{expr: identExpr, t: types.StringType}
	if got := ctx.Size(node); got != expected {
		t.Errorf("ctx.Size(node) = %v, want %v", got, expected)
	}
}

func TestNewModelOptions(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		opts := newModelOptions()
		if opts.version != defaultModelVersion {
			t.Errorf("version got %v, wanted %v", opts.version, defaultModelVersion)
		}
		if opts.strategy == nil {
			t.Error("strategy got nil, wanted the default sizing strategy")
		}
	})
	t.Run("nil strategy is ignored", func(t *testing.T) {
		opts := newModelOptions(WithSizingStrategy(nil), ModelVersion(0))
		if opts.strategy == nil {
			t.Error("strategy got nil, wanted the default sizing strategy to be retained")
		}
		if opts.version != 0 {
			t.Errorf("version got %v, wanted 0", opts.version)
		}
	})
}

func mustNewCoster(t *testing.T, checked *ast.AST, estimator Estimator, opts ...Option) *coster {
	t.Helper()
	c, err := newCoster(checked, estimator, opts...)
	if err != nil {
		t.Fatalf("newCoster() failed: %v", err)
	}
	return c
}

// TestBuildersOwnTheirInputs guards the immutability of built models: a caller reusing the slice
// it passed to a variadic builder must not change what was built.
func TestBuildersOwnTheirInputs(t *testing.T) {
	models := []OverloadModel{Overload("f", EvalCost(Const(1)))}
	opt := Model(models...)
	models[0] = Overload("f", EvalCost(Const(5)))
	est := newModelOptions(opt).modelSets[0].estimators(defaultModelRules())["f"](nil, nil, nil)
	if est.CostEstimate != FixedCostEstimate(1) {
		t.Errorf("Model() estimate = %v after the caller's slice changed, want %v", est.CostEstimate, FixedCostEstimate(1))
	}

	terms := []QuantityExpr{Const(2), Const(3)}
	sum := Sum(terms...)
	terms[0] = Const(9)
	if got := sum.estimate(nil); got != FixedSizeEstimate(5) {
		t.Errorf("Sum() estimate = %v after the caller's slice changed, want %v", got, FixedSizeEstimate(5))
	}
}

// TestNilOverloadFunctionsRejected guards that a missing estimator or tracker is reported when the
// estimator or tracker is configured, rather than panicking when the overload is first priced.
func TestNilOverloadFunctionsRejected(t *testing.T) {
	if _, err := newCoster(nil, nil, OverloadCostEstimate("f", nil)); err == nil || !strings.Contains(err.Error(), `OverloadCostEstimate("f")`) {
		t.Errorf("newCoster(OverloadCostEstimate(\"f\", nil)) errored: %v, wanted an error naming the overload", err)
	}
	if _, err := NewTracker(nil, OverloadTracker("f", nil)); err == nil || !strings.Contains(err.Error(), `OverloadTracker("f")`) {
		t.Errorf("NewTracker(OverloadTracker(\"f\", nil)) errored: %v, wanted an error naming the overload", err)
	}
}

func TestCompiledModelSetAndStdCachesPerModelRules(t *testing.T) {
	opt1 := Model(Overload("f1",
		EvalCost(Const(10)),
		BeforeVersion(1, EvalCost(Const(5))),
	))
	opt2 := Model(Overload("f2", EvalCost(Const(20))))
	cm1 := newModelOptions(opt1).modelSets[0]

	for _, strategy := range []SizingStrategy{DefaultSizingStrategy(), AggregateSizingStrategy()} {
		for _, v := range []uint32{0, 1, defaultModelVersion} {
			rules := modelRules{sizingStrategy: strategy, modelVersion: v}
			c := mustNewCoster(t, nil, nil, opt1, WithSizingStrategy(strategy), ModelVersion(v))
			if !c.overloadEstimators.shared {
				t.Errorf("rules %v: c.overloadEstimators.shared = false, want true", rules)
			}
			if cm1.estCache.rules != rules {
				t.Errorf("rules %v: cm1.estCache.rules = %v, want %v", rules, cm1.estCache.rules, rules)
			}
			cachedEst := cm1.estimators(rules)
			if len(cachedEst) != 1 {
				t.Fatalf("rules %v: cm1.estimators() = %v, want 1 entry", rules, cachedEst)
			}
			if c.standardOverloadEstimators == nil || len(c.standardOverloadEstimators) != len(StandardOverloadModels) {
				t.Errorf("rules %v: unexpected standardOverloadEstimators", rules)
			}

			tr, err := NewTracker(nil, opt1, WithSizingStrategy(strategy), ModelVersion(v))
			if err != nil {
				t.Fatalf("rules %v: NewTracker() failed: %v", rules, err)
			}
			if cm1.trkCache.rules != rules {
				t.Errorf("rules %v: cm1.trkCache.rules = %v, want %v", rules, cm1.trkCache.rules, rules)
			}
			cachedTrk := cm1.trackers(rules)
			if len(cachedTrk) != 1 {
				t.Fatalf("rules %v: cm1.trackers() = %v, want 1 entry", rules, cachedTrk)
			}
			if len(tr.overloadTrackers) != 1 {
				t.Errorf("rules %v: len(tr.overloadTrackers) = %d, want 1", rules, len(tr.overloadTrackers))
			}
			if tr.standardOverloadTrackers == nil || len(tr.standardOverloadTrackers) != len(StandardOverloadModels) {
				t.Errorf("rules %v: unexpected standardOverloadTrackers", rules)
			}

			// Merging a second Model option under the same rules must not mutate cm1's cache.
			cMerged := mustNewCoster(t, nil, nil, opt1, opt2, WithSizingStrategy(strategy), ModelVersion(v))
			if cMerged.overloadEstimators.shared {
				t.Errorf("rules %v: cMerged.overloadEstimators.shared = true, want false", rules)
			}
			if len(cMerged.overloadEstimators.fns) != 2 || len(cachedEst) != 1 {
				t.Errorf("rules %v: merged estimators = %d, cachedEst = %d, want 2 and 1", rules, len(cMerged.overloadEstimators.fns), len(cachedEst))
			}
			trMerged, err := NewTracker(nil, opt1, opt2, WithSizingStrategy(strategy), ModelVersion(v))
			if err != nil {
				t.Fatalf("rules %v: NewTracker(merged) failed: %v", rules, err)
			}
			if len(trMerged.overloadTrackers) != 2 || len(cachedTrk) != 1 {
				t.Errorf("rules %v: merged trackers = %d, cachedTrk = %d, want 2 and 1", rules, len(trMerged.overloadTrackers), len(cachedTrk))
			}
		}
	}

	// Non-comparable SizingStrategy implementations (e.g., structs containing a map) must not panic
	// when compared against previous rules.
	s1 := nonComparableSizingStrategy{sizes: map[string]uint64{"a": 1}}
	s2 := nonComparableSizingStrategy{sizes: map[string]uint64{"a": 2}}
	mustNewCoster(t, nil, nil, opt1, WithSizingStrategy(s1))
	mustNewCoster(t, nil, nil, opt1, WithSizingStrategy(s2))
	if _, err := NewTracker(nil, opt1, WithSizingStrategy(s1)); err != nil {
		t.Fatalf("NewTracker(s1) failed: %v", err)
	}
	if _, err := NewTracker(nil, opt1, WithSizingStrategy(s2)); err != nil {
		t.Fatalf("NewTracker(s2) failed: %v", err)
	}
}

type nonComparableSizingStrategy struct {
	sizes map[string]uint64
}

func (nonComparableSizingStrategy) EstimateSize(EstimateContext, AstNode) (SizeEstimate, bool) {
	return FixedSizeEstimate(1), true
}

func (nonComparableSizingStrategy) TrackSize(TrackContext, ref.Val) (uint64, bool) {
	return 1, true
}

