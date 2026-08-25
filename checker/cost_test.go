// Copyright 2022 Google LLC
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

package checker

import (
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/stdlib"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/parser"
)

func TestCheckerCostForwarding(t *testing.T) {
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	src := common.NewStringSource("a + b", "<input>")
	pe, errs := p.Parse(src)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("parser.Parse() failed: %v", errs.ToDisplayString())
	}
	reg, err := types.NewRegistry()
	if err != nil {
		t.Fatalf("types.NewRegistry() failed: %v", err)
	}
	e, err := NewEnv(containers.DefaultContainer, reg)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	err = e.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Fatalf("AddFunctions() failed: %v", err)
	}
	err = e.AddIdents(
		decls.NewVariable("a", types.IntType),
		decls.NewVariable("b", types.IntType),
	)
	if err != nil {
		t.Fatalf("AddIdents() failed: %v", err)
	}
	checked, errs := Check(pe, src, e)
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Check() failed: %v", errs.ToDisplayString())
	}

	est, err := Cost(checked, dummyCostEstimator{}, PresenceTestHasCost(true))
	if err != nil {
		t.Fatalf("Cost() failed: %v", err)
	}
	if est.Min != 3 || est.Max != 3 {
		t.Errorf("Cost() = [%d, %d], wanted [3, 3]", est.Min, est.Max)
	}

	fixedCost := FixedCostEstimate(5)
	if fixedCost.Min != 5 || fixedCost.Max != 5 {
		t.Errorf("FixedCostEstimate(5) = %v", fixedCost)
	}

	unknownCost := UnknownCostEstimate()
	if unknownCost.Min != 0 {
		t.Errorf("UnknownCostEstimate() = %v", unknownCost)
	}

	fixedSize := FixedSizeEstimate(10)
	if fixedSize.Min != 10 || fixedSize.Max != 10 {
		t.Errorf("FixedSizeEstimate(10) = %v", fixedSize)
	}

	unknownSize := UnknownSizeEstimate()
	if unknownSize.Min != 0 {
		t.Errorf("UnknownSizeEstimate() = %v", unknownSize)
	}

	node := NewAstNode(nil, []string{"foo"}, types.IntType, nil)
	if len(node.Path()) != 1 || node.Path()[0] != "foo" {
		t.Errorf("NewAstNode().Path() = %v", node.Path())
	}

	opt := OverloadCostEstimate("op", func(estimator cost.Estimator, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
		return nil
	})
	if opt == nil {
		t.Errorf("OverloadCostEstimate() = nil")
	}
}

type dummyCostEstimator struct{}

func (d dummyCostEstimator) EstimateSize(element AstNode) *SizeEstimate {
	return nil
}

func (d dummyCostEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	return nil
}
