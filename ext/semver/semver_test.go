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

package semver_test

import (
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	semverlib "cel.dev/cel-go/ext/semver"
	blangsemver "github.com/blang/semver/v4"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestSemver_Success(t *testing.T) {
	// These test cases are ported from kubernetes/staging/src/k8s.io/apiserver/pkg/cel/library
	// to ensure 1-to-1 parity with the Kubernetes implementation.
	tests := []struct {
		name string
		expr string
		out  any
	}{
		{
			name: "parse",
			expr: `semver("1.2.3")`,
			out:  blangsemver.MustParse("1.2.3"),
		},
		{
			name: "parse_to_custom_type",
			expr: `semver("1.2.3")`,
			out:  semverlib.Semver{Version: blangsemver.MustParse("1.2.3")},
		},
		{
			name: "isSemver",
			expr: `isSemver("1.2.3-beta.1+build.1")`,
			out:  true,
		},
		{
			name: "isSemver_empty_false",
			expr: `isSemver("")`,
			out:  false,
		},
		{
			name: "isSemver_v_prefix_false",
			expr: `isSemver("v1.0.0")`,
			out:  false,
		},
		{
			name: "isSemver_v_leading_whitespace_false",
			expr: `isSemver(" 1.0.0")`,
			out:  false,
		},
		{
			name: "isSemver_v_contains_whitespace_false",
			expr: `isSemver("1. 0.0")`,
			out:  false,
		},
		{
			name: "isSemver_v_trailing_whitespace_false",
			expr: `isSemver("1.0.0 ")`,
			out:  false,
		},
		{
			name: "isSemver_leading_zeros_false",
			expr: `isSemver("01.01.01")`,
			out:  false,
		},
		{
			name: "isSemver_major_only_false",
			expr: `isSemver("1")`,
			out:  false,
		},
		{
			name: "isSemver_major_minor_only_false",
			expr: `isSemver("1.1")`,
			out:  false,
		},
		{
			name: "isSemver_empty_normalize_false",
			expr: `isSemver("", true)`,
			out:  false,
		},
		{
			name: "isSemver_v_leading_whitespace_normalize_false",
			expr: `isSemver(" 1.0.0", true)`,
			out:  false,
		},
		{
			name: "isSemver_v_contains_whitespace_normalize_false",
			expr: `isSemver("1. 0.0", true)`,
			out:  false,
		},
		{
			name: "isSemver_v_trailing_whitespace_normalize_false",
			expr: `isSemver("1.0.0 ", true)`,
			out:  false,
		},
		{
			name: "isSemver_v_prefix_normalize_true",
			expr: `isSemver("v1.0.0", true)`,
			out:  true,
		},
		{
			name: "isSemver_leading_zeros_normalize_true",
			expr: `isSemver("01.01.01", true)`,
			out:  true,
		},
		{
			name: "isSemver_all_zeros_normalize_true",
			expr: `isSemver("00.00.00", true)`,
			out:  true,
		},
		{
			name: "isSemver_major_only_normalize_true",
			expr: `isSemver("1", true)`,
			out:  true,
		},
		{
			name: "isSemver_major_minor_only_normalize_true",
			expr: `isSemver("1.1", true)`,
			out:  true,
		},
		{
			name: "isSemver_normalize_false_arg",
			expr: `isSemver("v1.0.0", false)`,
			out:  false,
		},
		{
			name: "isSemver_short_prerelease_normalize_false",
			expr: `isSemver("1.2-alpha", true)`,
			out:  false,
		},
		{
			name: "equality_normalize",
			expr: `semver("v01.01", true) == semver("1.1.0")`,
			out:  true,
		},
		{
			name: "equality_normalize_zeros",
			expr: `semver("v00.00.00-alpha", true) == semver("0.0.0-alpha")`,
			out:  true,
		},
		{
			name: "equality_normalize_major_only",
			expr: `semver("v1", true) == semver("1.0.0")`,
			out:  true,
		},
		{
			name: "equality_normalize_false_arg",
			expr: `semver("1.2.3", false) == semver("1.2.3")`,
			out:  true,
		},
		{
			name: "equality_reflexivity",
			expr: `semver("1.2.3") == semver("1.2.3")`,
			out:  true,
		},
		{
			name: "inequality",
			expr: `semver("1.2.3") == semver("1.0.0")`,
			out:  false,
		},
		{
			name: "semver_less",
			expr: `semver("1.0.0").isLessThan(semver("1.2.3"))`,
			out:  true,
		},
		{
			name: "semver_less_false",
			expr: `semver("1.0.0").isLessThan(semver("1.0.0"))`,
			out:  false,
		},
		{
			name: "semver_greater",
			expr: `semver("1.2.3").isGreaterThan(semver("1.0.0"))`,
			out:  true,
		},
		{
			name: "semver_greater_false",
			expr: `semver("1.0.0").isGreaterThan(semver("1.0.0"))`,
			out:  false,
		},
		{
			name: "compare_equal",
			expr: `semver("1.2.3").compareTo(semver("1.2.3"))`,
			out:  int64(0),
		},
		{
			name: "compare_less",
			expr: `semver("1.0.0").compareTo(semver("1.2.3"))`,
			out:  int64(-1),
		},
		{
			name: "compare_greater",
			expr: `semver("1.2.3").compareTo(semver("1.0.0"))`,
			out:  int64(1),
		},
		{
			name: "major",
			expr: `semver("1.2.3").major()`,
			out:  int64(1),
		},
		{
			name: "minor",
			expr: `semver("1.2.3").minor()`,
			out:  int64(2),
		},
		{
			name: "patch",
			expr: `semver("1.2.3").patch()`,
			out:  int64(3),
		},
		{
			name: "string_conversion",
			expr: `string(semver("1.2.3-beta.1+build.1"))`,
			out:  "1.2.3-beta.1+build.1",
		},
		{
			name: "string_conversion_normalized",
			expr: `string(semver("v01.02", true))`,
			out:  "1.2.0",
		},
		{
			name: "type_is_semver_type",
			expr: `type(semver("1.2.3")) == semver_type`,
			out:  true,
		},
	}

	env, err := cel.NewEnv(semverlib.Library())
	if err != nil {
		t.Fatalf("cel.NewEnv(semverlib.Library()) failed: %v", err)
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

func TestSemver_CompileErrors(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		version     *uint32
		errContains string
	}{
		{
			name:        "semver constructor invalid literal",
			expr:        `semver("v1.0")`,
			errContains: "invalid semver argument: No Major.Minor.Patch elements found",
		},
		{
			name:        "semver constructor invalid literal with leading zero",
			expr:        `semver("01.2.3")`,
			errContains: "invalid semver argument",
		},
		{
			name:        "semver constructor normalize false invalid literal",
			expr:        `semver("v1.0", false)`,
			errContains: "invalid semver argument: No Major.Minor.Patch elements found",
		},
		{
			name:        "semver constructor normalize true invalid short prerelease",
			expr:        `semver("v1.2-alpha", true)`,
			errContains: "invalid semver argument: short version cannot contain PreRelease/Build meta data",
		},
		{
			name:        "semver constructor valid literal",
			expr:        `semver("1.2.3")`,
			errContains: "",
		},
		{
			name:        "semver constructor normalize true valid literal",
			expr:        `semver("v1.0", true)`,
			errContains: "",
		},
		{
			name:        "isSemver no matching overload for list",
			expr:        `isSemver([1, 2, 3])`,
			errContains: "found no matching overload for 'isSemver'",
		},
		{
			name:        "isSemver no matching overload for Semver",
			expr:        `isSemver(semver("1.2.3"))`,
			errContains: "found no matching overload for 'isSemver'",
		},
		{
			name:        "semver normalize not available in version 0",
			expr:        `semver("v1.0", true)`,
			version:     uint32Ptr(0),
			errContains: "found no matching overload for 'semver'",
		},
		{
			name:        "isSemver normalize not available in version 0",
			expr:        `isSemver("v1.0", true)`,
			version:     uint32Ptr(0),
			errContains: "found no matching overload for 'isSemver'",
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			var opts []semverlib.Option
			if tst.version != nil {
				opts = append(opts, semverlib.Version(*tst.version))
			}
			env, err := cel.NewEnv(semverlib.Library(opts...))
			if err != nil {
				t.Fatalf("cel.NewEnv(semverlib.Library()) failed: %v", err)
			}
			_, iss := env.Compile(tst.expr)
			if tst.errContains != "" {
				if iss.Err() == nil {
					t.Errorf("Expected compile error for %q, got nil", tst.expr)
					return
				}
				if !strings.Contains(iss.Err().Error(), tst.errContains) {
					t.Errorf("Expected compile error containing %q, got %q", tst.errContains, iss.Err().Error())
				}
			} else if iss.Err() != nil {
				t.Errorf("Compile(%q) failed unexpectedly: %v", tst.expr, iss.Err())
			}
		})
	}
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func TestSemver_RuntimeErrors(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		vars        map[string]any
		errContains string
	}{
		{
			name:        "parseInvalidVersion",
			expr:        `semver(v)`,
			vars:        map[string]any{"v": "v1.0"},
			errContains: "No Major.Minor.Patch elements found",
		},
		{
			name:        "parseNormalizeFalseInvalidVersion",
			expr:        `semver(v, false)`,
			vars:        map[string]any{"v": "v1.0"},
			errContains: "No Major.Minor.Patch elements found",
		},
		{
			name:        "parseNormalizeShortPrerelease",
			expr:        `semver(v, true)`,
			vars:        map[string]any{"v": "v1.0-alpha"},
			errContains: "short version cannot contain PreRelease/Build meta data",
		},
		{
			name:        "parseNormalizeDynamicFlagInvalid",
			expr:        `semver("v1.0", norm)`,
			vars:        map[string]any{"norm": false},
			errContains: "No Major.Minor.Patch elements found",
		},
	}

	env, err := cel.NewEnv(
		semverlib.Library(),
		cel.Variable("v", cel.StringType),
		cel.Variable("norm", cel.BoolType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			ast, iss := env.Compile(tst.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed unexpectedly: %v", tst.expr, iss.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", tst.expr, err)
			}

			_, _, err = prg.Eval(tst.vars)
			if err == nil {
				t.Fatalf("Expected runtime error for %q, got nil", tst.expr)
			}
			if !strings.Contains(err.Error(), tst.errContains) {
				t.Errorf("Expected error containing %q, got %q", tst.errContains, err.Error())
			}
		})
	}
}

func TestSemver_TypeConversions(t *testing.T) {
	ver := blangsemver.MustParse("1.2.3-alpha.1+build.1")
	semverVal := semverlib.Semver{Version: ver}

	t.Run("Semver ConvertToNative blangsemver.Version", func(t *testing.T) {
		got, err := semverVal.ConvertToNative(reflect.TypeFor[blangsemver.Version]())
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if !reflect.DeepEqual(got, ver) {
			t.Errorf("got %v, want %v", got, ver)
		}
	})

	t.Run("Semver ConvertToNative Semver", func(t *testing.T) {
		got, err := semverVal.ConvertToNative(reflect.TypeFor[semverlib.Semver]())
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if !reflect.DeepEqual(got, semverVal) {
			t.Errorf("got %v, want %v", got, semverVal)
		}
	})

	t.Run("Semver ConvertToNative string", func(t *testing.T) {
		got, err := semverVal.ConvertToNative(reflect.TypeFor[string]())
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if got != "1.2.3-alpha.1+build.1" {
			t.Errorf("got %v, want %v", got, "1.2.3-alpha.1+build.1")
		}
	})

	t.Run("Semver ConvertToNative any", func(t *testing.T) {
		got, err := semverVal.ConvertToNative(reflect.TypeFor[any]())
		if err != nil {
			t.Fatalf("ConvertToNative failed: %v", err)
		}
		if !reflect.DeepEqual(got, ver) {
			t.Errorf("got %v, want %v", got, ver)
		}
	})

	t.Run("Semver ConvertToNative unsupported", func(t *testing.T) {
		_, err := semverVal.ConvertToNative(reflect.TypeFor[int]())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("Semver ConvertToType StringType", func(t *testing.T) {
		got := semverVal.ConvertToType(types.StringType)
		if got.Type() != types.StringType {
			t.Errorf("got type %v, want %v", got.Type(), types.StringType)
		}
		if got.Value() != "1.2.3-alpha.1+build.1" {
			t.Errorf("got value %v, want %v", got.Value(), "1.2.3-alpha.1+build.1")
		}
	})

	t.Run("Semver ConvertToType SemverType", func(t *testing.T) {
		got := semverVal.ConvertToType(semverlib.SemverType)
		if !reflect.DeepEqual(got, semverVal) {
			t.Errorf("got %v, want %v", got, semverVal)
		}
	})

	t.Run("Semver ConvertToType TypeType", func(t *testing.T) {
		got := semverVal.ConvertToType(types.TypeType)
		if got != semverlib.SemverType {
			t.Errorf("got %v, want %v", got, semverlib.SemverType)
		}
	})

	t.Run("Semver ConvertToType unsupported", func(t *testing.T) {
		got := semverVal.ConvertToType(types.IntType)
		if !types.IsError(got) {
			t.Errorf("expected error val, got %v", got)
		}
	})

	t.Run("Semver Equal non-Semver", func(t *testing.T) {
		if semverVal.Equal(types.String("1.2.3-alpha.1+build.1")) != types.False {
			t.Errorf("expected False when comparing Semver with String")
		}
	})
}

func TestSemver_Adapter(t *testing.T) {
	env, err := cel.NewEnv(
		semverlib.Library(),
		cel.Variable("v", semverlib.SemverType),
		cel.Variable("vp", semverlib.SemverType),
		cel.Variable("vlist", cel.ListType(semverlib.SemverType)),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	v := blangsemver.MustParse("1.2.3")
	vp := blangsemver.MustParse("2.0.0")
	vlist := []blangsemver.Version{v, vp}

	ast, iss := env.Compile(`v.isLessThan(vp) && vlist[0] == semver("1.2.3") && vlist[1].major() == 2`)
	if iss.Err() != nil {
		t.Fatalf("Compile failed: %v", iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{
		"v":     v,
		"vp":    &vp,
		"vlist": vlist,
	})
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out != types.True {
		t.Errorf("got %v, want true", out)
	}
}

type noopCostEstimator struct{}

func (noopCostEstimator) EstimateSize(cost.AstNode) *cost.SizeEstimate {
	return nil
}

func (noopCostEstimator) EstimateCallCost(string, string, *cost.AstNode, []cost.AstNode) *cost.CallEstimate {
	return nil
}

func (noopCostEstimator) CallCost(string, string, []ref.Val, ref.Val) *uint64 {
	return nil
}

func TestSemverCost(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		estimatedCost cost.CostEstimate
		runtimeCost   uint64
	}{
		{
			name:          "semver",
			expr:          `semver("1.0.0")`,
			estimatedCost: cost.FixedCostEstimate(1),
			runtimeCost:   1,
		},
		{
			name:          "semver normalize",
			expr:          `semver("v1.0", true)`,
			estimatedCost: cost.FixedCostEstimate(1),
			runtimeCost:   1,
		},
		{
			name:          "semver long input",
			expr:          `semver("1234.56789012345.67890123456789")`,
			estimatedCost: cost.FixedCostEstimate(4),
			runtimeCost:   4,
		},
		{
			name:          "isSemver",
			expr:          `isSemver("1.0.0")`,
			estimatedCost: cost.FixedCostEstimate(1),
			runtimeCost:   1,
		},
		{
			name:          "isSemver normalize",
			expr:          `isSemver("v1.0", true)`,
			estimatedCost: cost.FixedCostEstimate(1),
			runtimeCost:   1,
		},
		{
			name:          "isSemver long input",
			expr:          `isSemver("1234.56789012345.67890123456789")`,
			estimatedCost: cost.FixedCostEstimate(4),
			runtimeCost:   4,
		},
		{
			name:          "major",
			expr:          `semver("1.2.3").major()`,
			estimatedCost: cost.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "minor",
			expr:          `semver("1.2.3").minor()`,
			estimatedCost: cost.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "patch",
			expr:          `semver("1.2.3").patch()`,
			estimatedCost: cost.FixedCostEstimate(2),
			runtimeCost:   2,
		},
		{
			name:          "isLessThan",
			expr:          `semver("1.0.0").isLessThan(semver("1.1.0"))`,
			estimatedCost: cost.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "isGreaterThan",
			expr:          `semver("1.1.0").isGreaterThan(semver("1.0.0"))`,
			estimatedCost: cost.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "compareTo",
			expr:          `semver("1.0.0").compareTo(semver("1.2.3"))`,
			estimatedCost: cost.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "equality",
			expr:          `semver("1.0.0") == semver("1.0.0")`,
			estimatedCost: cost.FixedCostEstimate(3),
			runtimeCost:   3,
		},
		{
			name:          "string",
			expr:          `string(semver("1.2.3"))`,
			estimatedCost: cost.FixedCostEstimate(2),
			runtimeCost:   2,
		},
	}

	env, err := cel.NewEnv(semverlib.Library())
	if err != nil {
		t.Fatalf("cel.NewEnv(semverlib.Library()) failed: %v", err)
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			parsedAst, iss := env.Parse(tst.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Parse(%q) failed: %v", tst.expr, iss.Err())
			}
			checkedAst, iss := env.Check(parsedAst)
			if iss.Err() != nil {
				t.Fatalf("env.Check(%q) failed: %v", tst.expr, iss.Err())
			}

			actualEst, err := env.EstimateCost(checkedAst, &noopCostEstimator{})
			if err != nil {
				t.Fatalf("env.EstimateCost(%q) failed: %v", tst.expr, err)
			}
			if actualEst.Min != tst.estimatedCost.Min || actualEst.Max != tst.estimatedCost.Max {
				t.Errorf("expected estimated cost %v, got %v for expr %q", tst.estimatedCost, actualEst, tst.expr)
			}

			legacyEst, err := env.EstimateCost(checkedAst, &noopCostEstimator{}, cost.ModelVersion(0))
			if err != nil {
				t.Fatalf("env.EstimateCost(%q, version 0) failed: %v", tst.expr, err)
			}
			if legacyEst.Min != tst.estimatedCost.Min || legacyEst.Max != tst.estimatedCost.Max {
				t.Errorf("expected version 0 estimated cost %v, got %v for expr %q", tst.estimatedCost, legacyEst, tst.expr)
			}

			program, err := env.Program(checkedAst, cel.CostTracking(&noopCostEstimator{}))
			if err != nil {
				t.Fatalf("env.Program(%q) failed: %v", tst.expr, err)
			}
			_, evalDetails, err := program.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("program.Eval(%q) failed: %v", tst.expr, err)
			}
			if evalDetails == nil || evalDetails.ActualCost() == nil {
				t.Fatalf("evalDetails or actualCost is nil for %q", tst.expr)
			}
			if *evalDetails.ActualCost() != tst.runtimeCost {
				t.Errorf("expected runtime cost %d, got %d for expr %q", tst.runtimeCost, *evalDetails.ActualCost(), tst.expr)
			}
		})
	}
}

// TestK8sCostParity verifies 1:1 parity with the Kubernetes CEL semver cost tests.
func TestK8sCostParity(t *testing.T) {
	cases := []struct {
		name                string
		expr                string
		expectEstimatedCost cost.CostEstimate
		expectRuntimeCost   uint64
	}{
		{
			name:                "semver",
			expr:                `semver("1.0.0")`,
			expectEstimatedCost: cost.FixedCostEstimate(1),
			expectRuntimeCost:   1,
		},
		{
			name:                "semver long input",
			expr:                `semver("1234.56789012345.67890123456789")`,
			expectEstimatedCost: cost.FixedCostEstimate(4),
			expectRuntimeCost:   4,
		},
		{
			name:                "isSemver",
			expr:                `isSemver("1.0.0")`,
			expectEstimatedCost: cost.FixedCostEstimate(1),
			expectRuntimeCost:   1,
		},
		{
			name:                "isSemver long input",
			expr:                `isSemver("1234.56789012345.67890123456789")`,
			expectEstimatedCost: cost.FixedCostEstimate(4),
			expectRuntimeCost:   4,
		},
		{
			name:                "major",
			expr:                `semver("1.2.3").major()`,
			expectEstimatedCost: cost.FixedCostEstimate(2),
			expectRuntimeCost:   2,
		},
		{
			name:                "minor",
			expr:                `semver("1.2.3").minor()`,
			expectEstimatedCost: cost.FixedCostEstimate(2),
			expectRuntimeCost:   2,
		},
		{
			name:                "patch",
			expr:                `semver("1.2.3").patch()`,
			expectEstimatedCost: cost.FixedCostEstimate(2),
			expectRuntimeCost:   2,
		},
		{
			name:                "isLessThan",
			expr:                `semver("1.0.0").isLessThan(semver("1.1.0"))`,
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
		{
			name:                "isGreaterThan",
			expr:                `semver("1.1.0").isGreaterThan(semver("1.0.0"))`,
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
		{
			name:                "compareTo",
			expr:                `semver("1.0.0").compareTo(semver("1.2.3"))`,
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
		{
			name:                "equality",
			expr:                `semver("v01.01", true) == semver("1.1.0")`,
			expectEstimatedCost: cost.FixedCostEstimate(3),
			expectRuntimeCost:   3,
		},
	}

	est := &k8sTestCostEstimator{}
	env, err := cel.NewEnv(
		semverlib.Library(),
		cel.CostEstimatorOptions(cost.PresenceTestHasCost(false)),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiled, issues := env.Compile(tc.expr)
			if issues.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, issues.Err())
			}

			estCost, err := env.EstimateCost(compiled, est)
			if err != nil {
				t.Fatalf("env.EstimateCost() failed: %v", err)
			}
			if estCost.Min != tc.expectEstimatedCost.Min || estCost.Max != tc.expectEstimatedCost.Max {
				t.Errorf("Expected estimated cost %v, got %v for %q", tc.expectEstimatedCost, estCost, tc.expr)
			}

			legacyCost, err := env.EstimateCost(compiled, est, cost.ModelVersion(0))
			if err != nil {
				t.Fatalf("env.EstimateCost(version 0) failed: %v", err)
			}
			if legacyCost.Min != tc.expectEstimatedCost.Min || legacyCost.Max != tc.expectEstimatedCost.Max {
				t.Errorf("Expected version 0 cost %v, got %v for %q", tc.expectEstimatedCost, legacyCost, tc.expr)
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
			if *actualCost != tc.expectRuntimeCost {
				t.Errorf("Expected runtime cost %d, got %d for %q", tc.expectRuntimeCost, *actualCost, tc.expr)
			}
		})
	}
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
