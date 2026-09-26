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
		expectRuntimeCost   uint64
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
			name:                "map existsOne",
			expr:                `{'a': 1, 'b': 2}.existsOne(k, v, v > 0)`,
			expectEstimatedCost: cost.RangedCostEstimate(38, 40),
			expectRuntimeCost:   40,
		},
		{
			name:                "map transformMap",
			expr:                `{'a': 1, 'b': 2}.transformMap(k, v, v + 1)`,
			expectEstimatedCost: cost.FixedCostEstimate(71),
			expectRuntimeCost:   71,
		},
		{
			name:                "map transformMap with filter",
			expr:                `{'a': 1, 'b': 2}.transformMap(k, v, v < 5, v + 1)`,
			expectEstimatedCost: cost.RangedCostEstimate(67, 75),
			expectRuntimeCost:   75,
		},
		{
			name:                "map transformMapEntry",
			expr:                `{'a': 1, 'b': 2}.transformMapEntry(k, v, {k: v + 1})`,
			expectEstimatedCost: cost.FixedCostEstimate(131),
			expectRuntimeCost:   131,
		},
		{
			name:                "map transformMapEntry with filter",
			expr:                `{'a': 1, 'b': 2}.transformMapEntry(k, v, v < 5, {k: v + 1})`,
			expectEstimatedCost: cost.RangedCostEstimate(67, 135),
			expectRuntimeCost:   135,
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
			name:                "list existsOne",
			expr:                `[1, 2].existsOne(i, v, v > 0)`,
			expectEstimatedCost: cost.RangedCostEstimate(18, 20),
			expectRuntimeCost:   20,
		},
		{
			name:                "list transformList",
			expr:                `[1, 2].transformList(i, v, v + 1)`,
			expectEstimatedCost: cost.FixedCostEstimate(49),
			expectRuntimeCost:   49,
		},
		{
			name:                "list transformList with filter",
			expr:                `[1, 2].transformList(i, v, v < 5, v + 1)`,
			expectEstimatedCost: cost.RangedCostEstimate(27, 53),
			expectRuntimeCost:   53,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
		// the comparison traversal now lands on the runtime cost of 9. Under
		// ModelVersion0 the lower bound was a hardcoded 7, below the 9 actually
		// charged, which is the unsoundness the revision corrects.
		{
			name:                  "quoteEquals",
			expr:                  "strings.quote('ABCDEFGHIJ abcdefghij') == strings.quote('ABCDEFGHIJ abcdefghij')",
			expectEstimatedCost:   cost.RangedCostEstimate(9, 11),
			expectEstimatedCostV0: costV0(7, 11),
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
		expectRuntimeCost   func(uint64) uint64
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
				testK8sCost(t, ipv4+op, tc.expectEstimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost))
			})

			t.Run(ipv6+op, func(t *testing.T) {
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
		expectRuntimeCost   func(uint64) uint64
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
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR(cidr('192.168.0.0/16'))"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('192.0.0.0/30')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('192.168.0.0/16')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
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
				return cost.RangedCostEstimate(c.Min+2, c.Max+5)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 2 },
		},
		{
			ops: []string{".containsIP('192.169.0.1')"},
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
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
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR(cidr('2001:db8::/32'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('2001:db8::/126')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+5, c.Max+9)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('2001:db8::/32')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
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
				return cost.RangedCostEstimate(c.Min+5, c.Max+8)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP('2001:db8::1')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEstimatedCost: func(c cost.CostEstimate) cost.CostEstimate {
				return cost.RangedCostEstimate(c.Min+3, c.Max+6)
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
	}...)

	for _, tc := range ipv4Cases {
		for _, op := range tc.ops {
			t.Run(ipv4+op, func(t *testing.T) {
				testK8sCost(t, ipv4+op, tc.expectEstimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost))
			})
		}
	}

	for _, tc := range ipv6Cases {
		for _, op := range tc.ops {
			t.Run(ipv6+op, func(t *testing.T) {
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
		k8sRuntimeCost   uint64
		celRuntimeCost   uint64
		reason           string
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
			testK8sCost(t, tc.expr, tc.celEstimatedCost, tc.celRuntimeCost)

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
	legacyCost, err := env.EstimateCost(compiled, est, cost.EstimateModelVersion(0))
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
