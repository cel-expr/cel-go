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
