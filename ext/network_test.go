// Copyright 2025 Google LLC
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

package ext

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/types"
)

func TestNetwork_Success(t *testing.T) {
	// These test cases are ported from kubernetes/staging/src/k8s.io/apiserver/pkg/cel/library
	// to ensure 1-to-1 parity with the Kubernetes implementation.
	tests := []struct {
		name string
		expr string
		out  any
	}{
		// CIDR Accessors
		{
			name: "cidr ip extraction",
			expr: "cidr('192.168.0.0/24').ip() == ip('192.168.0.0')",
			out:  true,
		},
		{
			name: "cidr ip extraction (host bits set)",
			// K8s behavior: cidr('1.2.3.4/24').ip() returns 1.2.3.4, not 1.2.3.0
			expr: "cidr('192.168.1.5/24').ip() == ip('192.168.1.5')",
			out:  true,
		},
		{
			name: "cidr masked",
			// masked() zeroes out the host bits
			expr: "cidr('192.168.1.5/24').masked() == cidr('192.168.1.0/24')",
			out:  true,
		},
		{
			name: "cidr masked identity",
			expr: "cidr('192.168.1.0/24').masked() == cidr('192.168.1.0/24')",
			out:  true,
		},
		{
			name: "cidr prefixLength",
			expr: "cidr('192.168.0.0/24').prefixLength()",
			out:  int64(24),
		},
		{
			name: "cidr to string IPv4",
			expr: "string(cidr('10.0.0.0/8'))",
			out:  "10.0.0.0/8",
		},
		{
			name: "cidr to string IPv6",
			expr: "string(cidr('::1/128'))",
			out:  "::1/128",
		},

		// Containment (CIDR in CIDR)
		{
			name: "containsCIDR different family",
			expr: "cidr('10.0.0.0/8').containsCIDR(cidr('::1/128'))",
			out:  false,
		},
		{
			name: "containsCIDR disjoint",
			expr: "cidr('10.0.0.0/8').containsCIDR(cidr('11.0.0.0/8'))",
			out:  false,
		},
		{
			name: "containsCIDR exact match",
			expr: "cidr('10.0.0.0/8').containsCIDR(cidr('10.0.0.0/8'))",
			out:  true,
		},
		{
			name: "containsCIDR larger prefix (false)",
			// /8 does not contain /4
			expr: "cidr('10.0.0.0/8').containsCIDR(cidr('0.0.0.0/4'))",
			out:  false,
		},
		{
			name: "containsCIDR string overload",
			expr: "cidr('10.0.0.0/8').containsCIDR('10.1.0.0/16')",
			out:  true,
		},
		{
			name: "containsCIDR subnet",
			expr: "cidr('10.0.0.0/8').containsCIDR(cidr('10.1.0.0/16'))",
			out:  true,
		},

		// Containment (IP in CIDR)
		{
			name: "containsIP edge case (broadcast)",
			expr: "cidr('10.0.0.0/8').containsIP(ip('10.255.255.255'))",
			out:  true,
		},
		{
			name: "containsIP edge case (network address)",
			expr: "cidr('10.0.0.0/8').containsIP(ip('10.0.0.0'))",
			out:  true,
		},
		{
			name: "containsIP false",
			expr: "cidr('10.0.0.0/8').containsIP(ip('11.0.0.0'))",
			out:  false,
		},
		{
			name: "containsIP simple",
			expr: "cidr('10.0.0.0/8').containsIP(ip('10.1.2.3'))",
			out:  true,
		},
		{
			name: "containsIP string overload",
			expr: "cidr('10.0.0.0/8').containsIP('10.1.2.3')",
			out:  true,
		},

		// IP Constructors & Properties
		{
			name: "family IPv4",
			expr: "ip('127.0.0.1').family()",
			out:  int64(4),
		},
		{
			name: "family IPv6",
			expr: "ip('::1').family()",
			out:  int64(6),
		},
		{
			name: "ip equality IPv4",
			expr: "ip('127.0.0.1') == ip('127.0.0.1')",
			out:  true,
		},
		{
			name: "ip equality IPv6 mixed case inputs",
			// Logic check: The value is equal even if string rep was different
			expr: "ip('2001:db8::1') == ip('2001:DB8::1')",
			out:  true,
		},
		{
			name: "ip inequality",
			expr: "ip('127.0.0.1') == ip('1.2.3.4')",
			out:  false,
		},
		{
			name: "ip to string IPv4",
			expr: "string(ip('1.2.3.4'))",
			out:  "1.2.3.4",
		},
		{
			name: "ip to string IPv6",
			expr: "string(ip('2001:db8::1'))",
			out:  "2001:db8::1",
		},

		// IP Canonicalization
		{
			name: "isCanonical IPv4 simple",
			expr: "ip.isCanonical('127.0.0.1')",
			out:  true,
		},
		{
			name: "isCanonical IPv6 expanded (invalid)",
			expr: "ip.isCanonical('2001:db8:0:0:0:0:0:1')",
			out:  false,
		},
		{
			name: "isCanonical IPv6 standard",
			expr: "ip.isCanonical('2001:db8::1')",
			out:  true,
		},
		{
			name: "isCanonical IPv6 uppercase (invalid)",
			expr: "ip.isCanonical('2001:DB8::1')",
			out:  false,
		},

		// IP Types & Predicates
		{
			name: "isGlobalUnicast 8.8.8.8",
			expr: "ip('8.8.8.8').isGlobalUnicast()",
			out:  true,
		},
		{
			name: "isLinkLocalMulticast",
			expr: "ip('ff02::1').isLinkLocalMulticast()",
			out:  true,
		},
		{
			name: "isLoopback IPv4",
			expr: "ip('127.0.0.1').isLoopback()",
			out:  true,
		},
		{
			name: "isLoopback IPv6",
			expr: "ip('::1').isLoopback()",
			out:  true,
		},
		{
			name: "isUnspecified IPv4",
			expr: "ip('0.0.0.0').isUnspecified()",
			out:  true,
		},
		{
			name: "isUnspecified IPv6",
			expr: "ip('::').isUnspecified()",
			out:  true,
		},

		// Global Predicates (IP & CIDR)
		{
			name: "isCIDR invalid mask",
			expr: "isCIDR('10.0.0.0/999')",
			out:  false,
		},
		{
			name: "isCIDR loose (host bits)",
			expr: "isCIDR('10.0.0.1/8')",
			out:  true,
		},
		{
			name: "isCIDR valid",
			expr: "isCIDR('10.0.0.0/8')",
			out:  true,
		},
		{
			name: "isIP invalid",
			expr: "isIP('not.an.ip')",
			out:  false,
		},
		{
			name: "isIP valid IPv4",
			expr: "isIP('1.2.3.4')",
			out:  true,
		},
		{
			name: "isIP valid IPv6",
			expr: "isIP('2001:db8::1')",
			out:  true,
		},
		{
			name: "isIP with port (invalid)",
			expr: "isIP('127.0.0.1:80')",
			out:  false,
		},
		{
			name: "isMask true (no host bits)",
			expr: "cidr('10.0.0.0/8').isMask()",
			out:  true,
		},
		{
			name: "isMask false (host bits)",
			expr: "cidr('10.0.0.1/8').isMask()",
			out:  false,
		},
		{
			name: "isMask IPv6 true (no host bits)",
			expr: "cidr('2001:db8::/32').isMask()",
			out:  true,
		},
		{
			name: "isMask IPv6 false (host bits)",
			expr: "cidr('2001:db8::1/32').isMask()",
			out:  false,
		},
		// IP success cases from K8s ip_test.go
		{
			name: "ipv4 isUnspecified false",
			expr: "ip('127.0.0.1').isUnspecified()",
			out:  false,
		},
		{
			name: "ipv4 isLoopback false",
			expr: "ip('1.2.3.4').isLoopback()",
			out:  false,
		},
		{
			name: "ipv4 isLinkLocalMulticast true",
			expr: "ip('224.0.0.1').isLinkLocalMulticast()",
			out:  true,
		},
		{
			name: "ipv4 isLinkLocalMulticast false",
			expr: "ip('224.0.1.1').isLinkLocalMulticast()",
			out:  false,
		},
		{
			name: "ipv4 isLinkLocalUnicast true",
			expr: "ip('169.254.169.254').isLinkLocalUnicast()",
			out:  true,
		},
		{
			name: "ipv4 isLinkLocalUnicast false",
			expr: "ip('192.168.0.1').isLinkLocalUnicast()",
			out:  false,
		},
		{
			name: "ipv4 isGlobalUnicast false",
			expr: "ip('255.255.255.255').isGlobalUnicast()",
			out:  false,
		},
		{
			name: "ipv6 isUnspecified false",
			expr: "ip('::1').isUnspecified()",
			out:  false,
		},
		{
			name: "ipv6 isLoopback false",
			expr: "ip('2001:db8::abcd').isLoopback()",
			out:  false,
		},
		{
			name: "ipv6 isLinkLocalMulticast false",
			expr: "ip('fd00::1').isLinkLocalMulticast()",
			out:  false,
		},
		{
			name: "ipv6 isLinkLocalUnicast true",
			expr: "ip('fe80::1').isLinkLocalUnicast()",
			out:  true,
		},
		{
			name: "ipv6 isLinkLocalUnicast false",
			expr: "ip('fd80::1').isLinkLocalUnicast()",
			out:  false,
		},
		{
			name: "ipv6 isGlobalUnicast true",
			expr: "ip('2001:db8::abcd').isGlobalUnicast()",
			out:  true,
		},
		{
			name: "ipv6 isGlobalUnicast false",
			expr: "ip('ff00::1').isGlobalUnicast()",
			out:  false,
		},
		{
			name: "type of IP is net.IP",
			expr: "type(ip('192.168.0.1')) == net.IP",
			out:  true,
		},
		// CIDR success cases from K8s cidr_test.go
		{
			name: "contains IP ipv6 (IP)",
			expr: "cidr('2001:db8::/32').containsIP(ip('2001:db8::1'))",
			out:  true,
		},
		{
			name: "does not contain IP ipv6 (IP)",
			expr: "cidr('2001:db8::/32').containsIP(ip('2001:dc8::1'))",
			out:  false,
		},
		{
			name: "contains IP ipv6 (string)",
			expr: "cidr('2001:db8::/32').containsIP('2001:db8::1')",
			out:  true,
		},
		{
			name: "does not contain IP ipv6 (string)",
			expr: "cidr('2001:db8::/32').containsIP('2001:dc8::1')",
			out:  false,
		},
		{
			name: "contains CIDR ipv6 (CIDR)",
			expr: "cidr('2001:db8::/32').containsCIDR(cidr('2001:db8::/33'))",
			out:  true,
		},
		{
			name: "does not contain CIDR ipv6 (CIDR)",
			expr: "cidr('2001:db8::/32').containsCIDR(cidr('2001:db8::/31'))",
			out:  false,
		},
		{
			name: "contains CIDR ipv6 (string)",
			expr: "cidr('2001:db8::/32').containsCIDR('2001:db8::/33')",
			out:  true,
		},
		{
			name: "does not contain CIDR ipv6 (string)",
			expr: "cidr('2001:db8::/32').containsCIDR('2001:db8::/31')",
			out:  false,
		},
		{
			name: "returns IP ipv6",
			expr: "cidr('2001:db8::/32').ip() == ip('2001:db8::')",
			out:  true,
		},
		{
			name: "masks masked ipv6",
			expr: "cidr('2001:db8::/32').masked() == cidr('2001:db8::/32')",
			out:  true,
		},
		{
			name: "masks unmasked ipv6",
			expr: "cidr('2001:db8:1::/32').masked() == cidr('2001:db8::/32')",
			out:  true,
		},
		{
			name: "returns prefix length ipv6",
			expr: "cidr('2001:db8::/32').prefixLength()",
			out:  int64(32),
		},
		{
			name: "type of CIDR is net.CIDR",
			expr: "type(cidr('192.168.0.0/24')) == net.CIDR",
			out:  true,
		},
	}

	// Initialize the environment with the Network extension
	env, err := cel.NewEnv(Network())
	if err != nil {
		t.Fatalf("cel.NewEnv(Network()) failed: %v", err)
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			ast, iss := env.Compile(tst.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tst.expr, iss.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", tst.expr, err)
			}

			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("Eval(%q) failed: %v", tst.expr, err)
			}

			// Convert the CEL result to a native Go value for comparison
			got, err := out.ConvertToNative(reflect.TypeOf(tst.out))
			if err != nil {
				t.Fatalf("ConvertToNative failed for expr %q: %v", tst.expr, err)
			}

			if !reflect.DeepEqual(got, tst.out) {
				t.Errorf("Expr %q result got %v, wanted %v", tst.expr, got, tst.out)
			}
		})
	}
}

func TestNetwork_RuntimeErrors(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		errContains string
	}{
		{
			name:        "containsIP string overload invalid",
			expr:        "cidr('10.0.0.0/8').containsIP('not-an-ip')",
			errContains: "parse error",
		},
		{
			name:        "containsCIDR string overload invalid",
			expr:        "cidr('10.0.0.0/8').containsCIDR('not-a-cidr')",
			errContains: "parse error",
		},
		{
			name:        "ip.isCanonical invalid ipv4",
			expr:        "ip.isCanonical('127.0.0.1.0')",
			errContains: "parse error",
		},
		{
			name:        "ip.isCanonical invalid ipv6",
			expr:        "ip.isCanonical('2001:db8:::68')",
			errContains: "parse error",
		},
	}

	env, err := cel.NewEnv(Network())
	if err != nil {
		t.Fatalf("cel.NewEnv(Network()) failed: %v", err)
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			ast, iss := env.Compile(tst.expr)
			if iss.Err() != nil {
				// Note: We only check runtime errors here. Compile errors are unexpected
				// because these functions accept strings, so type-check passes.
				t.Fatalf("Compile(%q) failed unexpectedly: %v", tst.expr, iss.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", tst.expr, err)
			}

			_, _, err = prg.Eval(cel.NoVars())
			if err == nil {
				t.Errorf("Expected runtime error for %q, got nil", tst.expr)
				return
			}

			// CEL errors are sometimes wrapped, so we check substring
			if !types.IsError(types.NewErr("%s", err.Error())) {
				// Just a sanity check that it is indeed a CEL-compatible error structure
				// Not strictly necessary but good practice
			}

			// Standard substring check
			gotErr := err.Error()
			// We just check if the message contains the specific error text we return in network.go
			found := false
			// Note: The actual error might be wrapped in "evaluation error: ..."
			if len(tst.errContains) > 0 {
				// Simple string contains check
				for i := 0; i < len(gotErr)-len(tst.errContains)+1; i++ {
					if gotErr[i:i+len(tst.errContains)] == tst.errContains {
						found = true
						break
					}
				}
			}

			if !found {
				t.Errorf("Expected error containing %q, got %q", tst.errContains, gotErr)
			}
		})
	}
}

func TestNetwork_TypeConversions(t *testing.T) {
	addr, _ := netip.ParseAddr("1.2.3.4")
	prefix, _ := netip.ParsePrefix("10.0.0.0/8")

	ipVal := IP{Addr: addr}
	cidrVal := CIDR{Prefix: prefix}

	// IP Conversions
	t.Run("IP ConvertToNative netip.Addr", func(t *testing.T) {
		got, err := ipVal.ConvertToNative(reflect.TypeOf(netip.Addr{}))
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if got != addr {
			t.Errorf("got %v, want %v", got, addr)
		}
	})

	t.Run("IP ConvertToNative string", func(t *testing.T) {
		got, err := ipVal.ConvertToNative(reflect.TypeOf(""))
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if got != "1.2.3.4" {
			t.Errorf("got %v, want %v", got, "1.2.3.4")
		}
	})

	t.Run("IP ConvertToNative unsupported", func(t *testing.T) {
		_, err := ipVal.ConvertToNative(reflect.TypeOf(0))
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("IP ConvertToType StringType", func(t *testing.T) {
		got := ipVal.ConvertToType(types.StringType)
		if got.Type() != types.StringType {
			t.Errorf("got type %v, want %v", got.Type(), types.StringType)
		}
		if got.Value() != "1.2.3.4" {
			t.Errorf("got value %v, want %v", got.Value(), "1.2.3.4")
		}
	})

	t.Run("IP ConvertToType IPType", func(t *testing.T) {
		got := ipVal.ConvertToType(IPType)
		if got != ipVal {
			t.Errorf("got %v, want %v", got, ipVal)
		}
	})

	t.Run("IP ConvertToType TypeType", func(t *testing.T) {
		got := ipVal.ConvertToType(types.TypeType)
		if got != IPType {
			t.Errorf("got %v, want %v", got, IPType)
		}
	})

	// CIDR Conversions
	t.Run("CIDR ConvertToNative netip.Prefix", func(t *testing.T) {
		got, err := cidrVal.ConvertToNative(reflect.TypeOf(netip.Prefix{}))
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if got != prefix {
			t.Errorf("got %v, want %v", got, prefix)
		}
	})

	t.Run("CIDR ConvertToNative string", func(t *testing.T) {
		got, err := cidrVal.ConvertToNative(reflect.TypeOf(""))
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if got != "10.0.0.0/8" {
			t.Errorf("got %v, want %v", got, "10.0.0.0/8")
		}
	})

	t.Run("CIDR ConvertToNative unsupported", func(t *testing.T) {
		_, err := cidrVal.ConvertToNative(reflect.TypeOf(0))
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CIDR ConvertToType StringType", func(t *testing.T) {
		got := cidrVal.ConvertToType(types.StringType)
		if got.Type() != types.StringType {
			t.Errorf("got type %v, want %v", got.Type(), types.StringType)
		}
		if got.Value() != "10.0.0.0/8" {
			t.Errorf("got value %v, want %v", got.Value(), "10.0.0.0/8")
		}
	})

	t.Run("CIDR ConvertToType CIDRType", func(t *testing.T) {
		got := cidrVal.ConvertToType(CIDRType)
		if got != cidrVal {
			t.Errorf("got %v, want %v", got, cidrVal)
		}
	})

	t.Run("CIDR ConvertToType TypeType", func(t *testing.T) {
		got := cidrVal.ConvertToType(types.TypeType)
		if got != CIDRType {
			t.Errorf("got %v, want %v", got, CIDRType)
		}
	})
}

func TestNetwork_CompileErrors(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		errContains string
	}{
		{
			name:        "ip constructor invalid literal",
			expr:        "ip('999.999.999.999')",
			errContains: "invalid ip argument",
		},
		{
			name:        "cidr constructor invalid literal",
			expr:        "cidr('1.2.3.4')",
			errContains: "invalid cidr argument",
		},
		{
			name:        "cidr constructor invalid mask literal",
			expr:        "cidr('10.0.0.0/999')",
			errContains: "invalid cidr argument",
		},
		{
			name:        "ip constructor valid literal",
			expr:        "ip('127.0.0.1')",
			errContains: "",
		},
		{
			name:        "cidr constructor valid literal",
			expr:        "cidr('10.0.0.0/8')",
			errContains: "",
		},
		{
			name:        "passing cidr into isIP returns compile error",
			expr:        "isIP(cidr('192.168.0.0/24'))",
			errContains: "found no matching overload for 'isIP'",
		},
		{
			name:        "cidr parse invalid ipv4",
			expr:        "cidr('192.168.0.0/')",
			errContains: "invalid cidr argument",
		},
		{
			name:        "cidr parse invalid ipv6",
			expr:        "cidr('2001:db8::/')",
			errContains: "invalid cidr argument",
		},
	}

	env, err := cel.NewEnv(Network())
	if err != nil {
		t.Fatalf("cel.NewEnv(Network()) failed: %v", err)
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			_, iss := env.Compile(tst.expr)
			if tst.errContains != "" {
				if iss.Err() == nil {
					t.Errorf("Expected compile error for %q, got nil", tst.expr)
					return
				}
				gotErr := iss.Err().Error()
				// Simple string contains check
				found := false
				for i := 0; i < len(gotErr)-len(tst.errContains)+1; i++ {
					if gotErr[i:i+len(tst.errContains)] == tst.errContains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected compile error containing %q, got %q", tst.errContains, gotErr)
				}
			} else {
				if iss.Err() != nil {
					t.Errorf("Compile(%q) failed unexpectedly: %v", tst.expr, iss.Err())
				}
			}
		})
	}
}

func TestNetworkCost(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		estimatedCost checker.CostEstimate
		runtimeCost   uint64
	}{
		{
			name:          "ip parse",
			expr:          "ip('192.168.0.1')",
			estimatedCost: checker.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "isIP parse",
			expr:          "isIP('192.168.0.1')",
			estimatedCost: checker.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "cidr parse",
			expr:          "cidr('192.168.0.0/16')",
			estimatedCost: checker.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "isCIDR parse",
			expr:          "isCIDR('192.168.0.0/16')",
			estimatedCost: checker.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "ip.isCanonical",
			expr:          "ip.isCanonical('192.168.0.1')",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "cidr containsIP ip",
			expr:          "cidr('192.168.0.0/16').containsIP(ip('192.169.0.1'))",
			estimatedCost: checker.CostEstimate{Min: 5, Max: 8},
			runtimeCost:   5,
		},
		{
			name:          "cidr containsIP string",
			expr:          "cidr('192.168.0.0/16').containsIP('192.0.0.1')",
			estimatedCost: checker.CostEstimate{Min: 4, Max: 7},
			runtimeCost:   4,
		},
		{
			name:          "cidr containsCIDR cidr",
			expr:          "cidr('192.168.0.0/16').containsCIDR(cidr('192.0.0.0/30'))",
			estimatedCost: checker.CostEstimate{Min: 7, Max: 11},
			runtimeCost:   7,
		},
		{
			name:          "cidr containsCIDR string",
			expr:          "cidr('192.168.0.0/16').containsCIDR('192.0.0.0/30')",
			estimatedCost: checker.CostEstimate{Min: 7, Max: 11},
			runtimeCost:   7,
		},
		{
			name:          "ip family",
			expr:          "ip('192.168.0.1').family()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "ip unspecified",
			expr:          "ip('192.168.0.1').isUnspecified()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "ip isLoopback",
			expr:          "ip('192.168.0.1').isLoopback()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "ip isLinkLocalMulticast",
			expr:          "ip('192.168.0.1').isLinkLocalMulticast()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "ip isLinkLocalUnicast",
			expr:          "ip('192.168.0.1').isLinkLocalUnicast()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "ip isGlobalUnicast",
			expr:          "ip('192.168.0.1').isGlobalUnicast()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "ipv6 family",
			expr:          "ip('2001:db8:3333:4444:5555:6666:7777:8888').family()",
			estimatedCost: checker.FixedCostEstimate(5),
			runtimeCost:   5,
		},
		{
			name:          "ipv6 unspecified",
			expr:          "ip('2001:db8:3333:4444:5555:6666:7777:8888').isUnspecified()",
			estimatedCost: checker.FixedCostEstimate(5),
			runtimeCost:   5,
		},
		{
			name:          "ipv6 isLoopback",
			expr:          "ip('2001:db8:3333:4444:5555:6666:7777:8888').isLoopback()",
			estimatedCost: checker.FixedCostEstimate(5),
			runtimeCost:   5,
		},
		{
			name:          "ipv6 isLinkLocalMulticast",
			expr:          "ip('2001:db8:3333:4444:5555:6666:7777:8888').isLinkLocalMulticast()",
			estimatedCost: checker.FixedCostEstimate(5),
			runtimeCost:   5,
		},
		{
			name:          "ipv6 isLinkLocalUnicast",
			expr:          "ip('2001:db8:3333:4444:5555:6666:7777:8888').isLinkLocalUnicast()",
			estimatedCost: checker.FixedCostEstimate(5),
			runtimeCost:   5,
		},
		{
			name:          "ipv6 isGlobalUnicast",
			expr:          "ip('2001:db8:3333:4444:5555:6666:7777:8888').isGlobalUnicast()",
			estimatedCost: checker.FixedCostEstimate(5),
			runtimeCost:   5,
		},
		{
			name:          "cidr ip extraction",
			expr:          "cidr('2001:db8::/32').ip()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "cidr prefixLength",
			expr:          "cidr('2001:db8::/32').prefixLength()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "cidr masked",
			expr:          "cidr('2001:db8::/32').masked()",
			estimatedCost: checker.FixedCostEstimate(3),
			runtimeCost:   3,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			testCost(t, tst.expr, tst.estimatedCost, tst.runtimeCost)
		})
	}
}

func testCost(t *testing.T, expr string, estimatedCost checker.CostEstimate, runtimeCost uint64) {
	t.Helper()
	env, err := cel.NewEnv(Network())
	if err != nil {
		t.Fatalf("cel.NewEnv(Network()) failed: %v", err)
	}
	parsedAst, iss := env.Parse(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Parse(%q) failed: %v", expr, iss.Err())
	}
	checkedAst, iss := env.Check(parsedAst)
	if iss.Err() != nil {
		t.Fatalf("env.Check(%q) failed: %v", expr, iss.Err())
	}

	actualEst, err := env.EstimateCost(checkedAst, &noopCostEstimator{})
	if err != nil {
		t.Fatalf("env.EstimateCost(%q) failed: %v", expr, err)
	}
	if actualEst.Min != estimatedCost.Min || actualEst.Max != estimatedCost.Max {
		t.Errorf("expected estimated cost %v, got %v for expr %q", estimatedCost, actualEst, expr)
	}

	program, err := env.Program(checkedAst, cel.CostTracking(&noopCostEstimator{}))
	if err != nil {
		t.Fatalf("env.Program(%q) failed: %v", expr, err)
	}
	_, evalDetails, err := program.Eval(cel.NoVars())
	if err != nil {
		t.Fatalf("program.Eval(%q) failed: %v", expr, err)
	}
	if evalDetails == nil || evalDetails.ActualCost() == nil {
		t.Fatalf("evalDetails or actualCost is nil for %q", expr)
	}
	if *evalDetails.ActualCost() != runtimeCost {
		t.Errorf("expected runtime cost %d, got %d for expr %q", runtimeCost, *evalDetails.ActualCost(), expr)
	}
}

func TestIPCost(t *testing.T) {
	ipv4 := "ip('192.168.0.1')"
	ipv4BaseEstimatedCost := checker.FixedCostEstimate(2)
	ipv4BaseRuntimeCost := uint64(2)

	ipv6 := "ip('2001:db8:3333:4444:5555:6666:7777:8888')"
	ipv6BaseEstimatedCost := checker.FixedCostEstimate(4)
	ipv6BaseRuntimeCost := uint64(4)

	testCases := []struct {
		ops                []string
		expectEsimatedCost func(checker.CostEstimate) checker.CostEstimate
		expectRuntimeCost  func(uint64) uint64
	}{
		{
			// For just parsing the IP, the cost is expected to be the base.
			ops:                []string{""},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate { return c },
			expectRuntimeCost:  func(c uint64) uint64 { return c },
		},
		{
			ops: []string{".family()", ".isUnspecified()", ".isLoopback()", ".isLinkLocalMulticast()", ".isLinkLocalUnicast()", ".isGlobalUnicast()"},
			// For most other operations, the cost is expected to be the base + 1.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 1, Max: c.Max + 1}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 1 },
		},
		{
			ops: []string{" == ip('192.168.0.1')"},
			// For most other operations, the cost is expected to be the base + 1.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return c.Add(ipv4BaseEstimatedCost).Add(checker.CostEstimate{Min: 1, Max: 2})
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + ipv4BaseRuntimeCost + 1 },
		},
	}

	for _, tc := range testCases {
		for _, op := range tc.ops {
			t.Run(ipv4+op, func(t *testing.T) {
				testCost(t, ipv4+op, tc.expectEsimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost))
			})

			t.Run(ipv6+op, func(t *testing.T) {
				testCost(t, ipv6+op, tc.expectEsimatedCost(ipv6BaseEstimatedCost), tc.expectRuntimeCost(ipv6BaseRuntimeCost))
			})
		}
	}
}

func TestCIDRCost(t *testing.T) {
	ipv4 := "cidr('192.168.0.0/16')"
	ipv4BaseEstimatedCost := checker.CostEstimate{Min: 2, Max: 2}
	ipv4BaseRuntimeCost := uint64(2)

	ipv6 := "cidr('2001:db8::/32')"
	ipv6BaseEstimatedCost := checker.CostEstimate{Min: 2, Max: 2}
	ipv6BaseRuntimeCost := uint64(2)

	type testCase struct {
		ops                []string
		expectEsimatedCost func(checker.CostEstimate) checker.CostEstimate
		expectRuntimeCost  func(uint64) uint64
	}

	cases := []testCase{
		{
			// For just parsing the IP, the cost is expected to be the base.
			ops:                []string{""},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate { return c },
			expectRuntimeCost:  func(c uint64) uint64 { return c },
		},
		{
			ops: []string{".ip()", ".prefixLength()", ".masked()"},
			// For most other operations, the cost is expected to be the base + 1.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 1, Max: c.Max + 1}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 1 },
		},
		{
			ops: []string{" == cidr('2001:db8::/32')"},
			// For most other operations, the cost is expected to be the base + 1.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return c.Add(ipv6BaseEstimatedCost).Add(checker.CostEstimate{Min: 1, Max: 2})
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + ipv6BaseRuntimeCost + 1 },
		},
	}

	//nolint:gocritic
	ipv4Cases := append(cases, []testCase{
		{
			ops: []string{".containsCIDR(cidr('192.0.0.0/30'))"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR(cidr('192.168.0.0/16'))"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('192.0.0.0/30')"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('192.168.0.0/16')"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP(ip('192.0.0.1'))"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 2, Max: c.Max + 5}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 2 },
		},
		{
			ops: []string{".containsIP(ip('192.169.0.1'))"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 3, Max: c.Max + 6}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
		{
			ops: []string{".containsIP(ip('192.169.169.250'))"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 3, Max: c.Max + 6}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
		{
			ops: []string{".containsIP('192.0.0.1')"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 2, Max: c.Max + 5}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 2 },
		},
		{
			ops: []string{".containsIP('192.169.0.1')"},
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 3, Max: c.Max + 6}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
	}...)

	//nolint:gocritic
	ipv6Cases := append(cases, []testCase{
		{
			ops: []string{".containsCIDR(cidr('2001:db8::/126'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR(cidr('2001:db8::/32'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('2001:db8::/126')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsCIDR('2001:db8::/32')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 9}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP(ip('2001:db8:3333:4444:5555:6666:7777:8888'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 8}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP(ip('2001:db8::1'))"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 3, Max: c.Max + 6}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
		{
			ops: []string{".containsIP('2001:db8:3333:4444:5555:6666:7777:8888')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 5, Max: c.Max + 8}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 5 },
		},
		{
			ops: []string{".containsIP('2001:db8::1')"},
			// For operations like checking if an IP is in a CIDR, the cost is expected to higher.
			expectEsimatedCost: func(c checker.CostEstimate) checker.CostEstimate {
				return checker.CostEstimate{Min: c.Min + 3, Max: c.Max + 6}
			},
			expectRuntimeCost: func(c uint64) uint64 { return c + 3 },
		},
	}...)

	for _, tc := range ipv4Cases {
		for _, op := range tc.ops {
			t.Run(ipv4+op, func(t *testing.T) {
				testCost(t, ipv4+op, tc.expectEsimatedCost(ipv4BaseEstimatedCost), tc.expectRuntimeCost(ipv4BaseRuntimeCost))
			})
		}
	}

	for _, tc := range ipv6Cases {
		for _, op := range tc.ops {
			t.Run(ipv6+op, func(t *testing.T) {
				testCost(t, ipv6+op, tc.expectEsimatedCost(ipv6BaseEstimatedCost), tc.expectRuntimeCost(ipv6BaseRuntimeCost))
			})
		}
	}
}
