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

package cel_test

import (
	"math"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/ext"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestK8sSetsCost(t *testing.T) {
	cases := []struct {
		name                string
		expr                string
		expectEstimatedCost cost.CostEstimate
		expectRuntimeCost   uint64
	}{
		{
			name:                "sets",
			expr:                `sets.contains([], [])`,
			expectEstimatedCost: cost.FixedCostEstimate(21),
			expectRuntimeCost:   21,
		},
		{
			expr:                `sets.contains([1], [])`,
			expectEstimatedCost: cost.FixedCostEstimate(21),
			expectRuntimeCost:   21,
		},
		{
			expr:                `sets.contains([1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(22),
			expectRuntimeCost:   22,
		},
		{
			expr:                `sets.contains([1], [1, 1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.contains([1, 1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.contains([2, 1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.contains([1, 2, 3, 4], [2, 3])`,
			expectEstimatedCost: cost.FixedCostEstimate(29),
			expectRuntimeCost:   29,
		},
		{
			expr:                `sets.contains([1], [1.0, 1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.contains([1, 2], [2u, 2.0])`,
			expectEstimatedCost: cost.FixedCostEstimate(25),
			expectRuntimeCost:   25,
		},
		{
			expr:                `sets.contains([1, 2u], [2, 2.0])`,
			expectEstimatedCost: cost.FixedCostEstimate(25),
			expectRuntimeCost:   25,
		},
		{
			expr:                `sets.contains([1, 2.0, 3u], [1.0, 2u, 3])`,
			expectEstimatedCost: cost.FixedCostEstimate(30),
			expectRuntimeCost:   30,
		},
		{
			expr: `sets.contains([[1], [2, 3]], [[2, 3.0]])`,
			// 10 for each list creation, top-level list sizes are 2, 1
			expectEstimatedCost: cost.FixedCostEstimate(53),
			expectRuntimeCost:   53,
		},
		{
			expr:                `!sets.contains([1], [2])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `!sets.contains([1], [1, 2])`,
			expectEstimatedCost: cost.FixedCostEstimate(24),
			expectRuntimeCost:   24,
		},
		{
			expr:                `!sets.contains([1], ["1", 1])`,
			expectEstimatedCost: cost.FixedCostEstimate(24),
			expectRuntimeCost:   24,
		},
		{
			expr:                `!sets.contains([1], [1.1, 1u])`,
			expectEstimatedCost: cost.FixedCostEstimate(24),
			expectRuntimeCost:   24,
		},

		// set equivalence (note the cost factor is higher as it's basically two contains checks)
		{
			expr:                `sets.equivalent([], [])`,
			expectEstimatedCost: cost.FixedCostEstimate(21),
			expectRuntimeCost:   21,
		},
		{
			expr:                `sets.equivalent([1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.equivalent([1], [1, 1])`,
			expectEstimatedCost: cost.FixedCostEstimate(25),
			expectRuntimeCost:   25,
		},
		{
			expr:                `sets.equivalent([1, 1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(25),
			expectRuntimeCost:   25,
		},
		{
			expr:                `sets.equivalent([1], [1u, 1.0])`,
			expectEstimatedCost: cost.FixedCostEstimate(25),
			expectRuntimeCost:   25,
		},
		{
			expr:                `sets.equivalent([1], [1u, 1.0])`,
			expectEstimatedCost: cost.FixedCostEstimate(25),
			expectRuntimeCost:   25,
		},
		{
			expr:                `sets.equivalent([1, 2, 3], [3u, 2.0, 1])`,
			expectEstimatedCost: cost.FixedCostEstimate(39),
			expectRuntimeCost:   39,
		},
		{
			expr:                `sets.equivalent([[1.0], [2, 3]], [[1], [2, 3.0]])`,
			expectEstimatedCost: cost.FixedCostEstimate(69),
			expectRuntimeCost:   69,
		},
		{
			expr:                `!sets.equivalent([2, 1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(26),
			expectRuntimeCost:   26,
		},
		{
			expr:                `!sets.equivalent([1], [1, 2])`,
			expectEstimatedCost: cost.FixedCostEstimate(26),
			expectRuntimeCost:   26,
		},
		{
			expr:                `!sets.equivalent([1, 2], [2u, 2, 2.0])`,
			expectEstimatedCost: cost.FixedCostEstimate(34),
			expectRuntimeCost:   34,
		},
		{
			expr:                `!sets.equivalent([1, 2], [1u, 2, 2.3])`,
			expectEstimatedCost: cost.FixedCostEstimate(34),
			expectRuntimeCost:   34,
		},
		{
			expr:                `sets.intersects([1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(22),
			expectRuntimeCost:   22,
		},
		{
			expr:                `sets.intersects([1], [1, 1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.intersects([1, 1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.intersects([2, 1], [1])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.intersects([1], [1, 2])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.intersects([1], [1.0, 2])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `sets.intersects([1, 2], [2u, 2, 2.0])`,
			expectEstimatedCost: cost.FixedCostEstimate(27),
			expectRuntimeCost:   27,
		},
		{
			expr:                `sets.intersects([1, 2], [1u, 2, 2.3])`,
			expectEstimatedCost: cost.FixedCostEstimate(27),
			expectRuntimeCost:   27,
		},
		{
			expr:                `sets.intersects([[1], [2, 3]], [[1, 2], [2, 3.0]])`,
			expectEstimatedCost: cost.FixedCostEstimate(65),
			expectRuntimeCost:   65,
		},
		{
			expr:                `!sets.intersects([], [])`,
			expectEstimatedCost: cost.FixedCostEstimate(22),
			expectRuntimeCost:   22,
		},
		{
			expr:                `!sets.intersects([1], [])`,
			expectEstimatedCost: cost.FixedCostEstimate(22),
			expectRuntimeCost:   22,
		},
		{
			expr:                `!sets.intersects([1], [2])`,
			expectEstimatedCost: cost.FixedCostEstimate(23),
			expectRuntimeCost:   23,
		},
		{
			expr:                `!sets.intersects([1], ["1", 2])`,
			expectEstimatedCost: cost.FixedCostEstimate(24),
			expectRuntimeCost:   24,
		},
		{
			expr:                `!sets.intersects([1], [1.1, 2u])`,
			expectEstimatedCost: cost.FixedCostEstimate(24),
			expectRuntimeCost:   24,
		},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			testK8sCost(t, tc.expr, tc.expectEstimatedCost, tc.expectRuntimeCost)
		})
	}
}

func TestK8sTwoVariableComprehensionCost(t *testing.T) {
	cases := []struct {
		name                string
		expr                string
		expectEstimatedCost cost.CostEstimate
		// expectEstimatedCostV0 is the estimate under cost.ModelVersion0, set only where the
		// revision moved it.
		expectEstimatedCostV0 *cost.CostEstimate
		expectRuntimeCost     uint64
	}{
		{
			name:                "map all",
			expr:                `{'a': 1, 'b': 2}.all(k, v, v > 0)`,
			expectEstimatedCost: cost.RangedCostEstimate(37, 41),
			expectRuntimeCost:   41,
		},
		{
			name:                "map exists",
			expr:                `{'a': 1, 'b': 2}.exists(k, v, v > 0)`,
			expectEstimatedCost: cost.RangedCostEstimate(39, 43),
			expectRuntimeCost:   40,
		},
		{
			name:                  "map existsOne",
			expr:                  `{'a': 1, 'b': 2}.existsOne(k, v, v > 0)`,
			expectEstimatedCost:   cost.RangedCostEstimate(36, 40),
			expectEstimatedCostV0: costV0(38, 40),
			expectRuntimeCost:     40,
		},
		{
			name:                "map transformMap",
			expr:                `{'a': 1, 'b': 2}.transformMap(k, v, v + 1)`,
			expectEstimatedCost: cost.FixedCostEstimate(71),
			expectRuntimeCost:   71,
		},
		{
			name:                  "map transformMap with filter",
			expr:                  `{'a': 1, 'b': 2}.transformMap(k, v, v < 5, v + 1)`,
			expectEstimatedCost:   cost.RangedCostEstimate(65, 75),
			expectEstimatedCostV0: costV0(67, 75),
			expectRuntimeCost:     75,
		},
		{
			name:                "map transformMapEntry",
			expr:                `{'a': 1, 'b': 2}.transformMapEntry(k, v, {k: v + 1})`,
			expectEstimatedCost: cost.FixedCostEstimate(131),
			expectRuntimeCost:   131,
		},
		{
			name:                  "map transformMapEntry with filter",
			expr:                  `{'a': 1, 'b': 2}.transformMapEntry(k, v, v < 5, {k: v + 1})`,
			expectEstimatedCost:   cost.RangedCostEstimate(65, 135),
			expectEstimatedCostV0: costV0(67, 135),
			expectRuntimeCost:     135,
		},

		{
			name:                "list all",
			expr:                `[1, 2].all(i, v, v > 0)`,
			expectEstimatedCost: cost.RangedCostEstimate(17, 21),
			expectRuntimeCost:   21,
		},
		{
			name:                "list exists",
			expr:                `[1, 2].exists(i, v, v > 0)`,
			expectEstimatedCost: cost.RangedCostEstimate(19, 23),
			expectRuntimeCost:   20,
		},
		{
			name:                  "list existsOne",
			expr:                  `[1, 2].existsOne(i, v, v > 0)`,
			expectEstimatedCost:   cost.RangedCostEstimate(16, 20),
			expectEstimatedCostV0: costV0(18, 20),
			expectRuntimeCost:     20,
		},
		{
			name:                "list transformList",
			expr:                `[1, 2].transformList(i, v, v + 1)`,
			expectEstimatedCost: cost.FixedCostEstimate(49),
			expectRuntimeCost:   49,
		},
		{
			name:                  "list transformList with filter",
			expr:                  `[1, 2].transformList(i, v, v < 5, v + 1)`,
			expectEstimatedCost:   cost.RangedCostEstimate(25, 53),
			expectEstimatedCostV0: costV0(27, 53),
			expectRuntimeCost:     53,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectEstimatedCostV0 != nil {
				testK8sCost(t, tc.expr, tc.expectEstimatedCost, tc.expectRuntimeCost, *tc.expectEstimatedCostV0)
				return
			}
			testK8sCost(t, tc.expr, tc.expectEstimatedCost, tc.expectRuntimeCost)
		})
	}
}

func TestK8sStringQuoteCost(t *testing.T) {
	cases := []struct {
		name                string
		expr                string
		expectEstimatedCost cost.CostEstimate
		// expectEstimatedCostV0 is the estimate under cost.ModelVersion0, set only where the
		// revision moved it.
		expectEstimatedCostV0 *cost.CostEstimate
		expectRuntimeCost     uint64
	}{
		{
			name:                "quote",
			expr:                "strings.quote('ABCDEFGHIJ abcdefghij')",
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
		// Both operands have identical size bounds, so the exact interval min of
		// the comparison traversal lands on the runtime cost of 9.
		{
			name:                "quoteEquals",
			expr:                "strings.quote('ABCDEFGHIJ abcdefghij') == strings.quote('ABCDEFGHIJ abcdefghij')",
			expectEstimatedCost: cost.RangedCostEstimate(9, 11),
			// Version 0 keeps the v0.32.0 comparison lower bound of 1 for non-empty operands.
			expectEstimatedCostV0: new(cost.RangedCostEstimate(7, 11)),
			expectRuntimeCost:     9,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectEstimatedCostV0 != nil {
				testK8sCost(t, tc.expr, tc.expectEstimatedCost, tc.expectRuntimeCost, *tc.expectEstimatedCostV0)
				return
			}
			testK8sCost(t, tc.expr, tc.expectEstimatedCost, tc.expectRuntimeCost)
		})
	}
}

func TestK8sIPCost(t *testing.T) {
	ipv4 := "ip('192.168.0.1')"
	ipv4BaseEstimatedCost := cost.FixedCostEstimate(2)
	ipv4BaseRuntimeCost := uint64(2)

	ipv6 := "ip('2001:db8:3333:4444:5555:6666:7777:8888')"
	ipv6BaseEstimatedCost := cost.FixedCostEstimate(4)
	ipv6BaseRuntimeCost := uint64(4)

	testCases := []struct {
		ops                 []string
		expectEstimatedCost func(cost.CostEstimate) cost.CostEstimate
		// expectEstimatedCostV0 is the estimate under cost.ModelVersion0, set only where the
		// revision moved it.
		expectEstimatedCostV0 func(cost.CostEstimate) cost.CostEstimate
		expectRuntimeCost     func(uint64) uint64
	}{
		{
			// For just parsing the IP, the cost is expected to be the base.
			ops:                 []string{""},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate { return c },
			expectRuntimeCost:   func(c uint64) uint64 { return c },
		},
		{
			ops: []string{".family()", ".isUnspecified()", ".isLoopback()", ".isLinkLocalMulticast()", ".isLinkLocalUnicast()", ".isGlobalUnicast()"},
			// For most other operations, the cost is expected to be the base + 1.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+1, c.Max+1)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 1 },
		},
		{
			ops: []string{" == ip('192.168.0.1')"},
			// In CEL-go, equality for opaque types is estimated as [1, 2].
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return c.Add(ipv4BaseEstimatedCost).Add(cost.RangedCostEstimate(1, 2))
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + ipv4BaseRuntimeCost + 1 },
		},
	}

	for _, tc := range testCases {
		for _, op := range tc.ops {
			t.Run(ipv4+op, func(t *testing.T) {
				if tc.expectEstimatedCostV0 != nil {
					testK8sCost(t, ipv4+op, tc.expectEstimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost), tc.expectEstimatedCostV0(ipv4BaseEstimatedCost))
					return
				}
				testK8sCost(t, ipv4+op, tc.expectEstimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost))
			})

			t.Run(ipv6+op, func(t *testing.T) {
				if tc.expectEstimatedCostV0 != nil {
					testK8sCost(t, ipv6+op, tc.expectEstimatedCost(ipv6BaseEstimatedCost), tc.expectRuntimeCost(ipv6BaseRuntimeCost), tc.expectEstimatedCostV0(ipv6BaseEstimatedCost))
					return
				}
				testK8sCost(t, ipv6+op, tc.expectEstimatedCost(ipv6BaseEstimatedCost), tc.expectRuntimeCost(ipv6BaseRuntimeCost))
			})
		}
	}
}

func TestK8sIPIsCanonicalCost(t *testing.T) {
	testCases := []struct {
		op                  string
		expectEstimatedCost cost.CostEstimate
		expectRuntimeCost   uint64
	}{
		{
			op:                  "ip.isCanonical('192.168.0.1')",
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
		{
			op:                  "ip.isCanonical('2001:db8:3333:4444:5555:6666:7777:8888')",
			expectEstimatedCost: cost.FixedCostEstimate(8),
			expectRuntimeCost:   8,
		},
		{
			op:                  "ip.isCanonical('2001:db8::abcd')",
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.op, func(t *testing.T) {
			testK8sCost(t, tc.op, tc.expectEstimatedCost, tc.expectRuntimeCost)
		})
	}
}

func TestK8sCIDRCost(t *testing.T) {
	ipv4 := "cidr('192.168.0.0/16')"
	ipv4BaseEstimatedCost := cost.FixedCostEstimate(2)
	ipv4BaseRuntimeCost := uint64(2)

	ipv6 := "cidr('2001:db8::/32')"
	ipv6BaseEstimatedCost := cost.FixedCostEstimate(2)
	ipv6BaseRuntimeCost := uint64(2)

	type testCase struct {
		ops                 []string
		expectEstimatedCost func(cost.CostEstimate) cost.CostEstimate
		// expectEstimatedCostV0 is the estimate under cost.ModelVersion0, set only where the
		// revision moved it.
		expectEstimatedCostV0 func(cost.CostEstimate) cost.CostEstimate
		expectRuntimeCost     func(uint64) uint64
	}

	cases := []testCase{
		{
			// For just parsing the IP, the cost is expected to be the base.
			ops:                 []string{""},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate { return c },
			expectRuntimeCost:   func(c uint64) uint64 { return c },
		},
		{
			ops: []string{".ip()", ".prefixLength()", ".masked()"},
			// For most other operations, the cost is expected to be the base + 1.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+1, c.Max+1)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 1 },
		},
		{
			ops: []string{" == cidr('2001:db8::/32')"},
			// In CEL-go, equality for opaque types is estimated as [1, 2].
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return c.Add(ipv6BaseEstimatedCost).Add(cost.RangedCostEstimate(1, 2))
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + ipv6BaseRuntimeCost + 1 },
		},
	}

	ipv4Cases := append(cases, []testCase{
		{
			ops: []string{".containsCIDR(cidr('192.0.0.0/30'))"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR(cidr('192.168.0.0/16'))"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('192.0.0.0/30')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('192.168.0.0/16')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP(ip('192.0.0.1'))"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+2, c.Max+5)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 2 },
		},
		{
			ops: []string{".containsIP(ip('192.169.0.1'))"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+6)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
		{
			ops: []string{".containsIP(ip('192.169.169.250'))"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+6)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
		{
			ops: []string{".containsIP('192.0.0.1')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+1, c.Max+5)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+2, c.Max+5)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 2 },
		},
		{
			ops: []string{".containsIP('192.169.0.1')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+2, c.Max+6)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+6)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
	}...)

	ipv6Cases := append(cases, []testCase{
		{
			ops: []string{".containsCIDR(cidr('2001:db8::/126'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR(cidr('2001:db8::/32'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('2001:db8::/126')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('2001:db8::/32')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+9)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP(ip('2001:db8:3333:4444:5555:6666:7777:8888'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+8)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP(ip('2001:db8::1'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+6)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
		{
			ops: []string{".containsIP('2001:db8:3333:4444:5555:6666:7777:8888')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+4, c.Max+8)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+8)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP('2001:db8::1')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+2, c.Max+6)
			},
			expectEstimatedCostV0: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+6)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
	}...)

	for _, tc := range ipv4Cases {
		for _, op := range tc.ops {
			t.Run(ipv4+op, func(t *testing.T) {
				if tc.expectEstimatedCostV0 != nil {
					testK8sCost(t, ipv4+op, tc.expectEstimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost), tc.expectEstimatedCostV0(ipv4BaseEstimatedCost))
					return
				}
				testK8sCost(t, ipv4+op, tc.expectEstimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost))
			})
		}
	}

	for _, tc := range ipv6Cases {
		for _, op := range tc.ops {
			t.Run(ipv6+op, func(t *testing.T) {
				if tc.expectEstimatedCostV0 != nil {
					testK8sCost(t, ipv6+op, tc.expectEstimatedCost(ipv6BaseEstimatedCost), tc.expectRuntimeCost(ipv6BaseRuntimeCost), tc.expectEstimatedCostV0(ipv6BaseEstimatedCost))
					return
				}
				testK8sCost(t, ipv6+op, tc.expectEstimatedCost(ipv6BaseEstimatedCost), tc.expectRuntimeCost(ipv6BaseRuntimeCost))
			})
		}
	}
}

// TestK8sNonConformance_CostEstimators demonstrates the non-k8s conformance differences
// where Kubernetes used a custom CostEstimator (in k8s.io/apiserver/pkg/cel/library/cost.go)
// rather than CEL-go's built-in cost estimators.
func TestK8sNonConformance_CostEstimators(t *testing.T) {
	cases := []struct {
		name             string
		expr             string
		k8sEstimatedCost cost.CostEstimate
		celEstimatedCost cost.CostEstimate
		// celEstimatedCostV0 is CEL-go's estimate under cost.ModelVersion0, set only where the
		// revision moved it.
		celEstimatedCostV0 *cost.CostEstimate
		k8sRuntimeCost     uint64
		celRuntimeCost     uint64
		reason             string
	}{
		// 1. IP and CIDR Equality (_==_)
		// Kubernetes hardcoded _==_ for IP/CIDR types to a fixed unit cost of 1 (Min: 1, Max: 1).
		// CEL-go estimates _==_ for opaque types as [1, 2].
		{
			name:             "ip_equality",
			expr:             "ip('192.168.0.1') == ip('192.168.0.1')",
			k8sEstimatedCost: cost.FixedCostEstimate(5),
			celEstimatedCost: cost.RangedCostEstimate(5, 6),
			k8sRuntimeCost:   5,
			celRuntimeCost:   5,
			reason:           "K8s custom estimator hardcoded _==_ on IP/CIDR to fixed cost 1, whereas CEL-go estimates _==_ on opaque types as [1, 2]",
		},
		{
			name:             "ipv6_equality",
			expr:             "ip('2001:db8:3333:4444:5555:6666:7777:8888') == ip('192.168.0.1')",
			k8sEstimatedCost: cost.FixedCostEstimate(7),
			celEstimatedCost: cost.RangedCostEstimate(7, 8),
			k8sRuntimeCost:   7,
			celRuntimeCost:   7,
			reason:           "K8s custom estimator hardcoded _==_ on IP/CIDR to fixed cost 1, whereas CEL-go estimates _==_ on opaque types as [1, 2]",
		},
		{
			name:             "cidr_equality",
			expr:             "cidr('192.168.0.0/16') == cidr('2001:db8::/32')",
			k8sEstimatedCost: cost.FixedCostEstimate(5),
			celEstimatedCost: cost.RangedCostEstimate(5, 6),
			k8sRuntimeCost:   5,
			celRuntimeCost:   5,
			reason:           "K8s custom estimator hardcoded _==_ on IP/CIDR to fixed cost 1, whereas CEL-go estimates _==_ on opaque types as [1, 2]",
		},

		// 2. String Library Transformations
		// Kubernetes custom CostEstimator calculated cost purely based on ceil(len * 0.1) traversal without
		// call overhead/result size estimation. CEL-go ext.Strings uses CallCostEstimate + string scan + allocation sizing.
		{
			name:             "string_lowerAscii",
			expr:             "'ABCDEFGHIJ abcdefghij'.lowerAscii()",
			k8sEstimatedCost: cost.FixedCostEstimate(3),
			celEstimatedCost: cost.FixedCostEstimate(1),
			k8sRuntimeCost:   3,
			celRuntimeCost:   1,
			reason:           "K8s custom estimator used ceil(len * 0.1) = 3 for lowerAscii on string literal, whereas CEL-go uses standard literal cost",
		},
		{
			name:             "string_upperAscii",
			expr:             "'ABCDEFGHIJ abcdefghij'.upperAscii()",
			k8sEstimatedCost: cost.FixedCostEstimate(3),
			celEstimatedCost: cost.FixedCostEstimate(1),
			k8sRuntimeCost:   3,
			celRuntimeCost:   1,
			reason:           "K8s custom estimator used ceil(len * 0.1) = 3 for upperAscii on string literal, whereas CEL-go uses standard literal cost",
		},
		{
			name:             "string_replace",
			expr:             "'abc 123 def 123'.replace('123', '456')",
			k8sEstimatedCost: cost.FixedCostEstimate(3),
			celEstimatedCost: cost.FixedCostEstimate(1),
			k8sRuntimeCost:   3,
			celRuntimeCost:   1,
			reason:           "K8s custom estimator used ceil(len * 2 * 0.1) = 3 for replace on string literal, whereas CEL-go uses standard literal cost",
		},
		{
			name:             "string_split",
			expr:             "'abc 123 def 123'.split(' ')",
			k8sEstimatedCost: cost.FixedCostEstimate(3),
			celEstimatedCost: cost.FixedCostEstimate(1),
			k8sRuntimeCost:   3,
			celRuntimeCost:   1,
			reason:           "K8s custom estimator used ceil(len * 2 * 0.1) = 3 for split on string literal, whereas CEL-go uses standard literal cost",
		},
		{
			name:             "string_substring",
			expr:             "'abc 123 def 123'.substring(5)",
			k8sEstimatedCost: cost.FixedCostEstimate(2),
			celEstimatedCost: cost.FixedCostEstimate(1),
			k8sRuntimeCost:   2,
			celRuntimeCost:   1,
			reason:           "K8s custom estimator used ceil(len * 0.1) = 2 for substring on string literal, whereas CEL-go uses standard literal cost",
		},
		{
			name:             "string_trim",
			expr:             "'  abc 123 def 123  '.trim()",
			k8sEstimatedCost: cost.FixedCostEstimate(2),
			celEstimatedCost: cost.FixedCostEstimate(1),
			k8sRuntimeCost:   2,
			celRuntimeCost:   1,
			reason:           "K8s custom estimator used ceil(len * 0.1) = 2 for trim on string literal, whereas CEL-go uses standard literal cost",
		},
		{
			name:             "string_join",
			expr:             "['aa', 'bb', 'cc', 'd', 'e', 'f', 'g', 'h', 'i', 'j'].join(' ')",
			k8sEstimatedCost: cost.RangedCostEstimate(11, 23),
			celEstimatedCost: cost.FixedCostEstimate(11),
			k8sRuntimeCost:   15,
			celRuntimeCost:   11,
			reason:           "K8s custom estimator used ceil(resultLen * 2 * 0.1) with string hints for join, whereas CEL-go computes cost directly from list size and separator",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify that CEL-go produces the celEstimatedCost and celRuntimeCost as triaged.
			if tc.celEstimatedCostV0 != nil {
				testK8sCost(t, tc.expr, tc.celEstimatedCost, tc.celRuntimeCost, *tc.celEstimatedCostV0)
			} else {
				testK8sCost(t, tc.expr, tc.celEstimatedCost, tc.celRuntimeCost)
			}

			// Document that K8s expected values differ from CEL-go's actual values.
			if tc.k8sEstimatedCost != tc.celEstimatedCost {
				t.Logf("[%s] Non-conformance in estimated cost: K8s expected %v, CEL-go produces %v. Reason: %s",
					tc.name, tc.k8sEstimatedCost, tc.celEstimatedCost, tc.reason)
			}
			if tc.k8sRuntimeCost != tc.celRuntimeCost {
				t.Logf("[%s] Non-conformance in runtime cost: K8s expected %d, CEL-go produces %d. Reason: %s",
					tc.name, tc.k8sRuntimeCost, tc.celRuntimeCost, tc.reason)
			}
		})
	}
}

// testK8sCost asserts the estimated cost at the latest cost model revision and at ModelVersion0,
// alongside the runtime cost.
//
// Kubernetes pins its published cost budgets to a released revision, so both numbers matter here:
// the latest one is what a new deployment sees, and the ModelVersion0 one is what an existing
// pinned deployment continues to see. expectEstimatedCostV0 is supplied only for the cases a
// revision actually moved; omitting it asserts the two revisions agree.
func testK8sCost(t *testing.T, expr string, expectEstimatedCost cost.CostEstimate, expectRuntimeCost uint64, expectEstimatedCostV0 ...cost.CostEstimate) {
	t.Helper()
	est := &k8sTestCostEstimator{}
	env, err := cel.NewEnv(
		ext.Strings(ext.StringsVersion(2)),
		ext.Lists(ext.ListsVersion(1)),
		ext.Sets(),
		ext.TwoVarComprehensions(),
		ext.Network(),
		cel.OptionalTypes(),
		cel.CostEstimatorOptions(cost.PresenceTestHasCost(false)),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	compiled, issues := env.Compile(expr)
	if issues.Err() != nil {
		t.Fatalf("env.Compile(%q) failed: %v", expr, issues.Err())
	}
	estCost, err := env.EstimateCost(compiled, est)
	if err != nil {
		t.Fatalf("env.EstimateCost() failed: %v", err)
	}
	if estCost.Min != expectEstimatedCost.Min || estCost.Max != expectEstimatedCost.Max {
		t.Errorf("Expected estimated cost of %d..%d but got %d..%d", expectEstimatedCost.Min, expectEstimatedCost.Max, estCost.Min, estCost.Max)
	}
	wantLegacy := expectEstimatedCost
	switch len(expectEstimatedCostV0) {
	case 0:
	case 1:
		wantLegacy = expectEstimatedCostV0[0]
	default:
		t.Fatalf("testK8sCost() accepts at most one ModelVersion0 expectation, got %d", len(expectEstimatedCostV0))
	}
	legacyCost, err := env.EstimateCost(compiled, est, cost.ModelVersion(0))
	if err != nil {
		t.Fatalf("env.EstimateCost(version 0) failed: %v", err)
	}
	if legacyCost.Min != wantLegacy.Min || legacyCost.Max != wantLegacy.Max {
		t.Errorf("Expected version 0 estimated cost of %d..%d but got %d..%d", wantLegacy.Min, wantLegacy.Max, legacyCost.Min, legacyCost.Max)
	}
	prog, err := env.Program(compiled, cel.CostTracking(k8sTestRuntimeCostEstimator{}))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	_, details, err := prog.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("prog.Eval() failed: %v", err)
	}
	actualCost := details.ActualCost()
	if actualCost == nil {
		t.Fatalf("details.ActualCost() is nil")
	}
	if *actualCost != expectRuntimeCost {
		t.Errorf("Expected runtime cost of %d but got %d", expectRuntimeCost, *actualCost)
	}
}

// costV0 records the estimate a case produces under cost.ModelVersion0, for the
// expectEstimatedCostV0 field. It exists because the constructors return values, and a table field
// of pointer type needs something addressable.
func costV0(lo, hi uint64) *cost.CostEstimate {
	est := cost.RangedCostEstimate(lo, hi)
	return &est
}

type k8sTestCostEstimator struct{}

func (t *k8sTestCostEstimator) EstimateSize(element cost.AstNode) *cost.SizeEstimate {
	expr, err := cel.TypeToExprType(element.Type())
	if err != nil {
		return nil
	}
	switch expr.GetPrimitive() {
	case exprpb.Type_STRING:
		est := cost.RangedSizeEstimate(0, 12)
		return &est
	case exprpb.Type_BYTES:
		est := cost.RangedSizeEstimate(0, 12)
		return &est
	}
	return nil
}

func (t *k8sTestCostEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	return nil
}

type k8sTestRuntimeCostEstimator struct{}

func (k8sTestRuntimeCostEstimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	return nil
}

//go:fix inline
func estimatePtr(est cost.CostEstimate) *cost.CostEstimate {
	return new(est)
}

// schemaPathEstimator answers size hints by field path, the way the Kubernetes CRD estimator
// answers them from an OpenAPI schema.
//
// A path rooted at a declared variable resolves through the schema. A bare path, one that starts
// at `@items` or `@values` because the indexed operand had no path of its own, resolves against
// the root of the object the expression is attached to, whose size is 7 here. That is how the
// Kubernetes estimator walks path[1:] from its root, and estimates pinned at ModelVersion(0) rely
// on it.
//
// With noBareHints set it has no answer for bare paths, like estimators that only know the paths
// of declared variables.
type schemaPathEstimator struct {
	noBareHints bool
}

var schemaPathSizes = map[string]uint64{
	"s":                 64,
	"ls":                16,
	"ls.@items":         64,
	"lls":               4,
	"lls.@items":        4,
	"lls.@items.@items": 32,
	"ms":                16,
	"ms.@keys":          64,
	"ms.@values":        64,
}

func (e schemaPathEstimator) EstimateSize(node cost.AstNode) *cost.SizeEstimate {
	path := node.Path()
	if len(path) == 0 {
		return nil
	}
	if strings.HasPrefix(path[0], "@") {
		if len(path) == 1 && !e.noBareHints {
			size := cost.RangedSizeEstimate(0, 7)
			return &size
		}
		return nil
	}
	if max, ok := schemaPathSizes[strings.Join(path, ".")]; ok {
		size := cost.RangedSizeEstimate(0, max)
		return &size
	}
	return nil
}

func (e schemaPathEstimator) EstimateCallCost(_, _ string, _ *cost.AstNode, _ []cost.AstNode) *cost.CallEstimate {
	return nil
}

// TestCostModelVersion0IndexMatchesV032 checks that indexing a list or map at ModelVersion(0)
// produces the estimate cel-go v0.32.0 produced, for operands that have no path of their own and
// for comprehension and bind variables. Each want value is the Max cel-go v0.32.0 estimated for
// the expression with the same environment and estimator.
//
// Before the fix every case but the controls estimated at math.MaxUint64 at version 0, so an
// expression that fit a budget pinned to v0.32.0 was rejected after upgrading.
func TestCostModelVersion0IndexMatchesV032(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Variable("s", cel.StringType),
		cel.Variable("ls", cel.ListType(cel.StringType)),
		cel.Variable("lls", cel.ListType(cel.ListType(cel.StringType))),
		cel.Variable("ms", cel.MapType(cel.StringType, cel.StringType)),
		cel.OptionalTypes(cel.OptionalTypesVersion(2)),
		ext.Strings(ext.StringsVersion(2)),
		ext.Lists(ext.ListsVersion(3)),
		ext.TwoVarComprehensions(),
		ext.Bindings(),
		cel.CostModelVersion(0),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		name string
		expr string
		want uint64
	}{
		// Operand without a path: v0.32.0 asked the Estimator about the bare path.
		{name: "split result", expr: `s.split('/')[1].matches('^[a-z]+$')`, want: 5},
		{name: "optional value list", expr: `optional.of(ls).value()[0].matches('y')`, want: 5},
		{name: "optional value map", expr: `optional.of(ms).value()['a'].matches('y')`, want: 5},
		{name: "optional ofNonZeroValue", expr: `optional.ofNonZeroValue(ls).value()[0].matches('y')`, want: 5},
		{name: "optional orValue list", expr: `optional.of(ls).orValue([])[0].matches('y')`, want: 15},
		{name: "optional orValue map", expr: `optional.of(ms).orValue({})['a'].matches('y')`, want: 35},
		{name: "sortBy result", expr: `ls.sortBy(x, x)[0].matches('y')`, want: 783},
		{name: "sortBy computed key", expr: `ls.sortBy(x, x.size())[0].matches('y')`, want: 773},

		// Comprehension or bind variable: the path is right, but an unbounded element size
		// from the overload model used to win over the path hint.
		{name: "index iter var in all", expr: `lls.all(l, l[0].matches('y'))`, want: 38},
		{name: "index iter var in exists", expr: `lls.exists(l, l[0].matches('y'))`, want: 42},
		{name: "index two-var iter value", expr: `lls.all(i, l, l[0].matches('y'))`, want: 38},
		{name: "index bind var", expr: `cel.bind(l, lls[0], l[0].matches('y'))`, want: 18},
		{name: "list of indexed bind var", expr: `cel.bind(l, lls[0], [l[0]])[0].matches('y')`, want: 29},
		{name: "list of indexed iter var", expr: `lls.exists(l, [l[0]][0].matches('y'))`, want: 86},
		{name: "map of indexed iter var", expr: `lls.map(l, l[0])[0].matches('y')`, want: 73},
		{name: "filtered map of indexed iter var", expr: `lls.map(l, l.size() > 0, l[0])[0].matches('y')`, want: 85},
		{name: "map of derived iter value", expr: `lls.map(l, l[0] + '')[0].matches('y')`, want: 89},
		{name: "transformList of indexed iter var", expr: `lls.transformList(i, l, l[0])[0].matches('y')`, want: 73},

		// Element sizes that v0.32.0 did not derive from the shape of the expression: a call's
		// model, a conditional or concatenation over field values, or dyn() over one, now may
		// report an element size, but before version 1 the path hint takes precedence.
		{name: "sortBy of literal", expr: `[s].sortBy(x, x)[0].matches('y')`, want: 63},
		{name: "sort result", expr: `ls.sort()[0].matches('y')`, want: 552},
		// distinct keeps the flat lists v3 factor of v0.30.0 and earlier; v0.32.0 estimated 552.
		{name: "distinct result", expr: `ls.distinct()[0].matches('y')`, want: 526},
		{name: "reverse result", expr: `ls.reverse()[0].matches('y')`, want: 30},
		{name: "flatten of field list", expr: `[ls].flatten()[0].matches('y')`, want: 25},
		{name: "slice of split", expr: `s.split('/').slice(0, 1)[0].matches('y')`, want: 16},
		{name: "optional value of literal", expr: `optional.of([s]).value()[0].matches('y')`, want: 15},
		{name: "transformMap result", expr: `ms.transformMap(k, v, v)['a'].matches('y')`, want: 98},
		{name: "conditional over field lists", expr: `(ls.size() > 0 ? ls : ls)[0].matches('y')`, want: 6},
		{name: "conditional over field maps", expr: `(ls.size() > 0 ? ms : ms)['a'].matches('y')`, want: 6},
		{name: "conditional over literal and field", expr: `(ls.size() > 0 ? [s] : ls)[0].matches('y')`, want: 16},
		{name: "concat of field and literal", expr: `(ls + [s])[0].matches('y')`, want: 15},
		{name: "dyn of field list", expr: `dyn(ls)[0].matches('y')`, want: 4},
		{name: "bind result of literal", expr: `cel.bind(v, [s], v)[0].matches('y')`, want: 24},
		{name: "bind result of field", expr: `cel.bind(v, ls, v)[0].matches('y')`, want: 14},

		// Shapes v0.32.0 did track: the element size comes from the expression itself.
		{name: "conditional over literals", expr: `(ls.size() > 0 ? [s] : [s])[0].matches('y')`, want: 22},
		{name: "concat of tracked lists", expr: `(ls.map(x, x) + [s])[0].matches('y')`, want: 240},
		{name: "concat of literals", expr: `([s] + [s])[1].matches('y')`, want: 31},
		{name: "dyn of literal", expr: `dyn([s])[0].matches('y')`, want: 20},
		{name: "index bind var of map result", expr: `cel.bind(v, ls.map(x, x), v[0].matches('y'))`, want: 239},
		{name: "filter result", expr: `ls.filter(x, x != '')[0].matches('y')`, want: 244},
		{name: "transformList result", expr: `ls.transformList(i, v, v)[0].matches('y')`, want: 228},

		// Controls the regression did not touch.
		{name: "map of iter var", expr: `ls.map(x, x)[0].matches('y')`, want: 228},
		{name: "list literal", expr: `[s][0].matches('y')`, want: 19},
		{name: "index iter var size", expr: `lls.all(l, l[0].size() < 5)`, want: 30},
		{name: "index by iter key", expr: `ms.all(k, ms[k].matches('y'))`, want: 210},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			est, err := env.EstimateCost(ast, schemaPathEstimator{})
			if err != nil {
				t.Fatalf("env.EstimateCost(%q) failed: %v", tc.expr, err)
			}
			if est.Max != tc.want {
				t.Errorf("env.EstimateCost(%q).Max = %d, want %d (cel-go v0.32.0)", tc.expr, est.Max, tc.want)
			}
		})
	}
}

// splitElemEstimator is schemaPathEstimator plus a cost model for split() that, like the one
// Kubernetes added for kubernetes/kubernetes#144135, reports the size of the result's elements.
// withElem controls whether the element size is reported; the cost is the same either way.
type splitElemEstimator struct {
	schemaPathEstimator
	withElem bool
}

func (e splitElemEstimator) EstimateCallCost(function, _ string, target *cost.AstNode, _ []cost.AstNode) *cost.CallEstimate {
	if function != "split" || target == nil {
		return nil
	}
	receiver := e.EstimateSize(*target)
	if receiver == nil {
		return nil
	}
	result := cost.RangedSizeEstimate(0, receiver.Max)
	if e.withElem {
		elem := cost.RangedSizeEstimate(0, receiver.Max)
		result.Elem = &elem
	}
	return cost.NewCallEstimate(cost.FixedCostEstimate(1), &result)
}

// TestCostModelVersion0IgnoresCallModelElemSizes checks that at ModelVersion(0) a call model which
// reports an element size for its result changes neither the estimate of indexing that result nor
// that of iterating over it, since cel-go v0.32.0 asked the Estimator for the element's path
// instead. It holds whether or not the Estimator answers bare paths.
func TestCostModelVersion0IgnoresCallModelElemSizes(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Variable("s", cel.StringType),
		cel.Variable("ls", cel.ListType(cel.StringType)),
		ext.Strings(ext.StringsVersion(2)),
		ext.Lists(ext.ListsVersion(3)),
		cel.CostModelVersion(0),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	for _, expr := range []string{
		`s.split('/')[1].matches('^[a-z]+$')`,
		`s.split('/', 2)[1].matches('y')`,
		`s.split('/').slice(0, 1)[0].matches('y')`,
		`s.split('/').sortBy(p, p)[0].matches('y')`,
		`s.split(',').map(kv, kv.split('='))[0][1].matches('y')`,
		`s.split('/').all(p, p.matches('y'))`,
		`s.split(',').exists(kv, kv.split('=')[1].matches('y'))`,
	} {
		t.Run(expr, func(t *testing.T) {
			ast, iss := env.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
			}
			for _, noBareHints := range []bool{false, true} {
				paths := schemaPathEstimator{noBareHints: noBareHints}
				without, err := env.EstimateCost(ast, splitElemEstimator{schemaPathEstimator: paths})
				if err != nil {
					t.Fatalf("env.EstimateCost(%q) without element size failed: %v", expr, err)
				}
				with, err := env.EstimateCost(ast, splitElemEstimator{schemaPathEstimator: paths, withElem: true})
				if err != nil {
					t.Fatalf("env.EstimateCost(%q) with element size failed: %v", expr, err)
				}
				if with != without {
					t.Errorf("env.EstimateCost(%q) (noBareHints=%v) = %+v with the model's element size, %+v without, want equal",
						expr, noBareHints, with, without)
				}
			}
		})
	}
}

// TestCostModelVersion0WithoutBareHintsMatchesV032 checks ModelVersion(0) against cel-go v0.32.0
// for an Estimator that has no answer for bare paths. v0.32.0 then had no size for an element of
// a call's result, or for a variable iterating over one, so the estimate was unbounded; v0.33
// took the size from the call's model instead, which underestimated some of these. Each bounded
// want value is the Max cel-go v0.32.0 estimated; unbounded marks a v0.32.0 estimate of at least
// math.MaxUint64/4.
func TestCostModelVersion0WithoutBareHintsMatchesV032(t *testing.T) {
	const unbounded = math.MaxUint64
	env, err := cel.NewEnv(
		cel.Variable("s", cel.StringType),
		cel.Variable("ls", cel.ListType(cel.StringType)),
		cel.Variable("lls", cel.ListType(cel.ListType(cel.StringType))),
		cel.Variable("ms", cel.MapType(cel.StringType, cel.StringType)),
		cel.OptionalTypes(cel.OptionalTypesVersion(2)),
		ext.Strings(ext.StringsVersion(2)),
		ext.Lists(ext.ListsVersion(3)),
		ext.TwoVarComprehensions(),
		cel.CostModelVersion(0),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	tests := []struct {
		expr string
		want uint64
	}{
		// Iterating over or indexing a call's result.
		{expr: `ls.sort().all(x, x.matches('y'))`, want: unbounded},
		{expr: `[ls].flatten().all(x, x.matches('y'))`, want: unbounded},
		{expr: `ms.transformMap(k, v, v).all(k, k.matches('y'))`, want: unbounded},
		{expr: `ms.transformMap(k, v, v).all(k, v, v.matches('y'))`, want: unbounded},
		{expr: `lls.flatten().exists(i, i.contains('z'))`, want: unbounded},
		{expr: `ls.distinct().exists(i, i.contains('z'))`, want: unbounded},
		{expr: `ls.slice(0, 2).exists(i, i.contains('z'))`, want: unbounded},
		{expr: `lls.flatten()[0].contains('z')`, want: unbounded},
		{expr: `optional.of(ls).value()[0].matches('y')`, want: unbounded},

		// Shapes whose sizes v0.32.0 knew without a bare path.
		{expr: `lls.all(l, l[0].matches('y'))`, want: 38},
		{expr: `ls.map(x, x).all(y, y.matches('y'))`, want: 397},
		{expr: `[s, s].all(x, x.matches('y'))`, want: 35},
		{expr: `(ls.size() > 0 ? [s] : [s]).all(x, x.matches('y'))`, want: 26},
		{expr: `ls.all(x, x.matches('y'))`, want: 178},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			est, err := env.EstimateCost(ast, schemaPathEstimator{noBareHints: true})
			if err != nil {
				t.Fatalf("env.EstimateCost(%q) failed: %v", tc.expr, err)
			}
			switch {
			case tc.want == unbounded && est.Max < math.MaxUint64/4:
				t.Errorf("env.EstimateCost(%q).Max = %d, want unbounded (cel-go v0.32.0)", tc.expr, est.Max)
			case tc.want != unbounded && est.Max != tc.want:
				t.Errorf("env.EstimateCost(%q).Max = %d, want %d (cel-go v0.32.0)", tc.expr, est.Max, tc.want)
			}
		})
	}
}
