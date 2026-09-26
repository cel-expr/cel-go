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
	"testing"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// unversionedContext stands in for a third-party EstimateContext, which cannot implement the
// unexported versionedEstimateContext interface.
type unversionedContext struct {
	EstimateContext
}

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

func TestContextModelVersion(t *testing.T) {
	tests := []struct {
		name string
		ctx  EstimateContext
		want uint32
	}{
		{
			// A context with no revision of its own is treated as running at the latest, which is
			// the right default for a context built without any knowledge of pinning.
			name: "unversioned_context_defaults_to_latest",
			ctx:  unversionedContext{},
			want: defaultModelVersion,
		},
		{
			// newEstimateContext must forward its coster's pin. It is the context handed to sizing
			// strategies, so a silent default here would ignore the caller's pin.
			name: "estimator_context_reports_coster_pin",
			ctx:  (&coster{modelVersion: 0}).newEstimateContext(),
			want: 0,
		},
		{
			name: "estimator_context_reports_latest_pin",
			ctx:  (&coster{modelVersion: 1}).newEstimateContext(),
			want: 1,
		},
		{
			// estimatorEvalContext bakes the revision in at build time, since it has no coster to
			// consult.
			name: "eval_context_reports_baked_in_version",
			ctx:  &estimatorEvalContext{version: 0},
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contextModelVersion(tc.ctx); got != tc.want {
				t.Errorf("contextModelVersion() got %v, wanted %v", got, tc.want)
			}
		})
	}
}

func TestModelVersionOfAndVersionedEstimator(t *testing.T) {
	if got := ModelVersionOf(nil); got != defaultModelVersion {
		t.Errorf("ModelVersionOf(nil) = %v, want %v", got, defaultModelVersion)
	}
	if got := ModelVersionOf(stubEstimator{}); got != defaultModelVersion {
		t.Errorf("ModelVersionOf(stubEstimator{}) = %v, want %v", got, defaultModelVersion)
	}

	cNil := &coster{modelVersion: 0}
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
	cDel := &coster{
		estimator:    stubEstimator{sizeEst: &sz, callEst: call},
		modelVersion: 1,
	}
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

	// Without a configured sizing strategy, getSizingStrategy defaults to defaultSizing,
	// and when a custom noop strategy returns false for a variable, sizeOrUnknown returns UnknownSizeEstimate.
	c := &coster{
		checkedAST:     checked,
		computedSizes:  make(map[int64]SizeEstimate),
		exprPaths:      make(map[int64][]string),
		sizingStrategy: noopSizingStrategy{},
	}
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
	t.Run("nil option and nil strategy are ignored", func(t *testing.T) {
		opts := newModelOptions(nil, WithSizingStrategy(nil), ModelVersion(0))
		if opts.strategy == nil {
			t.Error("strategy got nil, wanted the default sizing strategy to be retained")
		}
		if opts.version != 0 {
			t.Errorf("version got %v, wanted 0", opts.version)
		}
	})
}
