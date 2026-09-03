// Copyright 2020 Google LLC
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
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"

	proto3pb "cel.dev/cel-go/test/proto3pb"
)

func TestStringsWithExtensionV2(t *testing.T) {
	env, err := cel.NewEnv(Strings())
	if err != nil {
		t.Fatalf("cel.NewEnv(Strings()) failed: %v", err)
	}
	_, err = env.Extend(Strings())
	if err != nil {
		t.Fatalf("env.Extend(Strings()) failed: %v", err)
	}
}

func TestStringFormatV2(t *testing.T) {
	tests := []struct {
		name                  string
		format                string
		dynArgs               map[string]any
		formatArgs            string
		err                   string
		expectedOutput        string
		expectedRuntimeCost   uint64
		expectedEstimatedCost cost.CostEstimate
		skipCompileCheck      bool
	}{
		{
			name:                  "no-op",
			format:                "no substitution",
			expectedOutput:        "no substitution",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},

		{
			name:                  "mid-string substitution",
			format:                "str is %s and some more",
			formatArgs:            `"filler"`,
			expectedOutput:        "str is filler and some more",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "percent escaping",
			format:                "%% and also %%",
			expectedOutput:        "% and also %",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "substution inside escaped percent signs",
			format:                "%%%s%%",
			formatArgs:            `"text"`,
			expectedOutput:        "%text%",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "substitution with one escaped percent sign on the right",
			format:                "%s%%",
			formatArgs:            `"percent on the right"`,
			expectedOutput:        "percent on the right%",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "substitution with one escaped percent sign on the left",
			format:                "%%%s",
			formatArgs:            `"percent on the left"`,
			expectedOutput:        "%percent on the left",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "multiple substitutions",
			format:                "%d %d %d, %s %s %s, %d %d %d, %s %s %s",
			formatArgs:            `1, 2, 3, "A", "B", "C", 4, 5, 6, "D", "E", "F"`,
			expectedOutput:        "1 2 3, A B C, 4 5 6, D E F",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:                  "percent sign escape sequence support",
			format:                "\u0025\u0025escaped \u0025s\u0025\u0025",
			formatArgs:            `"percent"`,
			expectedOutput:        "%escaped percent%",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.FixedCostEstimate(12),
		},
		{
			name:                  "fixed point formatting clause",
			format:                "%.3f",
			formatArgs:            "1.2345",
			expectedOutput:        "1.234",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.FixedCostEstimate(11),
		},
		{
			name:                  "fixed point formatting clause with int",
			format:                "%.3f + %.2f",
			formatArgs:            "1, 1u",
			expectedOutput:        "1.000 + 1.00",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.FixedCostEstimate(12),
		},
		{
			name:                  "binary formatting clause",
			format:                "this is 5 in binary: %b",
			formatArgs:            "5",
			expectedOutput:        "this is 5 in binary: 101",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "negative binary formatting clause",
			format:                "this is -5 in binary: %b",
			formatArgs:            "-5",
			expectedOutput:        "this is -5 in binary: -101",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "uint support for binary formatting",
			format:                "unsigned 64 in binary: %b",
			formatArgs:            "uint(64)",
			expectedOutput:        "unsigned 64 in binary: 1000000",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:                  "bool support for binary formatting",
			format:                "bit set from bool: %b",
			formatArgs:            "true",
			expectedOutput:        "bit set from bool: 1",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "octal formatting clause",
			format:                "%o",
			formatArgs:            "11",
			expectedOutput:        "13",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "negative octal formatting clause",
			format:                "%o",
			formatArgs:            "-11",
			expectedOutput:        "-13",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "uint support for octal formatting clause",
			format:                "this is an unsigned octal: %o",
			formatArgs:            "uint(65535)",
			expectedOutput:        "this is an unsigned octal: 177777",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:                  "lowercase hexadecimal formatting clause",
			format:                "%x is 30 in hexadecimal",
			formatArgs:            "30",
			expectedOutput:        "1e is 30 in hexadecimal",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "uppercase hexadecimal formatting clause",
			format:                "%X is 20 in hexadecimal",
			formatArgs:            "30",
			expectedOutput:        "1E is 20 in hexadecimal",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "negative hexadecimal formatting clause",
			format:                "%x is -30 in hexadecimal",
			formatArgs:            "-30",
			expectedOutput:        "-1e is -30 in hexadecimal",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:                  "unsigned support for hexadecimal formatting clause",
			format:                "%X is 6000 in hexadecimal",
			formatArgs:            "uint(6000)",
			expectedOutput:        "1770 is 6000 in hexadecimal",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:                  "string support with hexadecimal formatting clause",
			format:                "%x",
			formatArgs:            `"Hello world!"`,
			expectedOutput:        "48656c6c6f20776f726c6421",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "string support with uppercase hexadecimal formatting clause",
			format:                "%X",
			formatArgs:            `"Hello world!"`,
			expectedOutput:        "48656C6C6F20776F726C6421",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "byte support with hexadecimal formatting clause",
			format:                "%x",
			formatArgs:            `b"byte string"`,
			expectedOutput:        "6279746520737472696e67",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "byte support with hexadecimal formatting clause leading zero",
			format:                "%x",
			formatArgs:            `b"\x00\x00byte string\x00"`,
			expectedOutput:        "00006279746520737472696e6700",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "byte support with uppercase hexadecimal formatting clause",
			format:                "%X",
			formatArgs:            `b"byte string"`,
			expectedOutput:        "6279746520737472696E67",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "scientific notation formatting clause",
			format:                "%.6e",
			formatArgs:            "1052.032911275",
			expectedOutput:        "1.052033e+03",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "scientific notation formatting clause with uint",
			format:                "%.4e",
			formatArgs:            "1052u",
			expectedOutput:        "1.0520e+03",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "scientific notation formatting clause with int",
			format:                "%.5e",
			formatArgs:            "1052",
			expectedOutput:        "1.05200e+03",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "default precision for fixed-point clause",
			format:                "%f",
			formatArgs:            "2.71828",
			expectedOutput:        "2.718280",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "default precision for scientific notation",
			format:                "%e",
			formatArgs:            "2.71828",
			expectedOutput:        "2.718280e+00",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "default precision for string",
			format:                "%s",
			formatArgs:            "2.71",
			expectedOutput:        "2.71",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "default list precision for string",
			format:                "%s",
			formatArgs:            "[2.71]",
			expectedOutput:        "[2.71]",
			expectedRuntimeCost:   21,
			expectedEstimatedCost: cost.CostEstimate{Min: 21, Max: 21},
		},
		{
			name:                  "default format for string",
			format:                "%s",
			formatArgs:            "0.000000002",
			expectedOutput:        "0.000000002",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "default list scientific notation for string",
			format:                "%s",
			formatArgs:            "[0.000000002]",
			expectedOutput:        "[0.000000002]",
			expectedRuntimeCost:   21,
			expectedEstimatedCost: cost.CostEstimate{Min: 21, Max: 21},
		},
		{
			name:                  "NaN support for fixed-point",
			format:                "%f",
			formatArgs:            `double("NaN")`,
			expectedOutput:        "NaN",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "positive infinity support for fixed-point",
			format:                "%f",
			formatArgs:            `double("Infinity")`,
			expectedOutput:        "Infinity",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "negative infinity support for fixed-point",
			format:                "%f",
			formatArgs:            `double("-Infinity")`,
			expectedOutput:        "-Infinity",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:           "NaN support for string",
			format:         "%s",
			formatArgs:     `double("NaN")`,
			expectedOutput: "NaN",
		},
		{
			name:           "positive infinity support for string",
			format:         "%s",
			formatArgs:     `double("Infinity")`,
			expectedOutput: "Infinity",
		},
		{
			name:           "negative infinity support for string",
			format:         "%s",
			formatArgs:     `double("-Infinity")`,
			expectedOutput: "-Infinity",
		},
		{
			name:           "infinity list support for string",
			format:         "%s",
			formatArgs:     `[double("NaN"),double("+Infinity"), double("-Infinity")]`,
			expectedOutput: `[NaN, Infinity, -Infinity]`,
		},
		{
			name:                  "uint support for decimal clause",
			format:                "%d",
			formatArgs:            "uint(64)",
			expectedOutput:        "64",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "null support for string",
			format:                "null: %s",
			formatArgs:            "null",
			expectedOutput:        "null: null",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "int support for string",
			format:                "%s",
			formatArgs:            `999999999999`,
			expectedOutput:        "999999999999",
			expectedRuntimeCost:   11,
			expectedEstimatedCost: cost.CostEstimate{Min: 11, Max: 11},
		},
		{
			name:                  "bytes support for string",
			format:                "some bytes: %s",
			formatArgs:            `b"xyz"`,
			expectedOutput:        "some bytes: xyz",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "type() support for string",
			format:                "type is %s",
			formatArgs:            `type("test string")`,
			expectedOutput:        "type is string",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "timestamp support for string",
			format:                "%s",
			formatArgs:            `timestamp("2023-02-03T23:31:20+00:00")`,
			expectedOutput:        "2023-02-03T23:31:20Z",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "duration support for string",
			format:                "%s",
			formatArgs:            `duration("1h45m47s")`,
			expectedOutput:        "6347s",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "small duration support for string",
			format:                "%s",
			formatArgs:            `duration("2ns")`,
			expectedOutput:        "0.000000002s",
			expectedRuntimeCost:   12,
			expectedEstimatedCost: cost.CostEstimate{Min: 12, Max: 12},
		},
		{
			name:                  "list support for string",
			format:                "%s",
			formatArgs:            `["abc", 3.14, null, [9, 8, 7, 6], timestamp("2023-02-03T23:31:20Z")]`,
			expectedOutput:        `[abc, 3.14, null, [9, 8, 7, 6], 2023-02-03T23:31:20Z]`,
			expectedRuntimeCost:   32,
			expectedEstimatedCost: cost.CostEstimate{Min: 32, Max: 32},
		},
		{
			name:                  "map support for string",
			format:                "%s",
			formatArgs:            `{"key1": b"xyz", "key5": null, "key2": duration("2h"), "key4": true, "key3": 2.71828}`,
			expectedOutput:        `{key1: xyz, key2: 7200s, key3: 2.71828, key4: true, key5: null}`,
			expectedRuntimeCost:   42,
			expectedEstimatedCost: cost.CostEstimate{Min: 42, Max: 42},
		},
		{
			name:                  "map support (all key types)",
			format:                "map with multiple key types: %s",
			formatArgs:            `{1: "value1", uint(2): "value2", true: double("NaN")}`,
			expectedOutput:        `map with multiple key types: {1: value1, 2: value2, true: NaN}`,
			expectedRuntimeCost:   46,
			expectedEstimatedCost: cost.CostEstimate{Min: 46, Max: 46},
		},
		{
			name:                  "boolean support for %s",
			format:                "true bool: %s, false bool: %s",
			formatArgs:            `true, false`,
			expectedOutput:        "true bool: true, false bool: false",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for string formatting clause",
			format:     "dynamic string: %s",
			formatArgs: `dynStr`,
			dynArgs: map[string]any{
				"dynStr": "a string",
			},
			expectedOutput:        "dynamic string: a string",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for numbers with string formatting clause",
			format:     "dynIntStr: %s dynDoubleStr: %s",
			formatArgs: `dynIntStr, dynDoubleStr`,
			dynArgs: map[string]any{
				"dynIntStr":    32,
				"dynDoubleStr": 56.8,
			},
			expectedOutput:        "dynIntStr: 32 dynDoubleStr: 56.8",
			expectedRuntimeCost:   15,
			expectedEstimatedCost: cost.CostEstimate{Min: 15, Max: 15},
		},
		{
			name:       "dyntype support for integer formatting clause",
			format:     "dynamic int: %d",
			formatArgs: `dynInt`,
			dynArgs: map[string]any{
				"dynInt": 128,
			},
			expectedOutput:        "dynamic int: 128",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for integer formatting clause (unsigned)",
			format:     "dynamic unsigned int: %d",
			formatArgs: `dynUnsignedInt`,
			dynArgs: map[string]any{
				"dynUnsignedInt": uint64(256),
			},
			expectedOutput:        "dynamic unsigned int: 256",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:       "dyntype support for hex formatting clause",
			format:     "dynamic hex int: %x",
			formatArgs: `dynHexInt`,
			dynArgs: map[string]any{
				"dynHexInt": 22,
			},
			expectedOutput:        "dynamic hex int: 16",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for hex formatting clause (uppercase)",
			format:     "dynamic hex int: %X (uppercase)",
			formatArgs: `dynHexInt`,
			dynArgs: map[string]any{
				"dynHexInt": 26,
			},
			expectedOutput:        "dynamic hex int: 1A (uppercase)",
			expectedRuntimeCost:   15,
			expectedEstimatedCost: cost.CostEstimate{Min: 15, Max: 15},
		},
		{
			name:       "dyntype support for unsigned hex formatting clause",
			format:     "dynamic hex int: %x (unsigned)",
			formatArgs: `dynUnsignedHexInt`,
			dynArgs: map[string]any{
				"dynUnsignedHexInt": uint(500),
			},
			expectedOutput:        "dynamic hex int: 1f4 (unsigned)",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:       "dyntype support for fixed-point formatting clause",
			format:     "dynamic double: %.3f",
			formatArgs: `dynDouble`,
			dynArgs: map[string]any{
				"dynDouble": 4.5,
			},
			expectedOutput:        "dynamic double: 4.500",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for scientific notation",
			format:     "(dyntype) e: %e",
			formatArgs: "dynE",
			dynArgs: map[string]any{
				"dynE": 2.71828,
			},
			expectedOutput:        "(dyntype) e: 2.718280e+00",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype NaN/infinity support for decimal",
			format:     "NaN: %d, infinity: %d, -infinity: %d",
			formatArgs: `dynNaN, dynInf, dynNegInf`,
			dynArgs: map[string]any{
				"dynNaN":    math.NaN(),
				"dynInf":    math.Inf(1),
				"dynNegInf": math.Inf(-1),
			},
			expectedOutput:        "NaN: NaN, infinity: Infinity, -infinity: -Infinity",
			expectedRuntimeCost:   17,
			expectedEstimatedCost: cost.FixedCostEstimate(17),
		},
		{
			name:       "dyntype NaN/infinity support for fixed-point",
			format:     "NaN: %f, infinity: %f",
			formatArgs: `dynNaN, dynInf`,
			dynArgs: map[string]any{
				"dynNaN": math.NaN(),
				"dynInf": math.Inf(1),
			},
			expectedOutput:        "NaN: NaN, infinity: Infinity",
			expectedRuntimeCost:   15,
			expectedEstimatedCost: cost.CostEstimate{Min: 15, Max: 15},
		},
		{
			name:       "dyntype NaN/infinity support for scientific",
			format:     "NaN: %e, infinity: %e, -infinity: %e",
			formatArgs: `dynNaN, dynInf, dynNegInf`,
			dynArgs: map[string]any{
				"dynNaN":    math.NaN(),
				"dynInf":    math.Inf(1),
				"dynNegInf": math.Inf(-1),
			},
			expectedOutput:        "NaN: NaN, infinity: Infinity, -infinity: -Infinity",
			expectedRuntimeCost:   17,
			expectedEstimatedCost: cost.FixedCostEstimate(17),
		},
		{
			name:       "dyntype support for timestamp",
			format:     "dyntype timestamp: %s",
			formatArgs: `dynTime`,
			dynArgs: map[string]any{
				"dynTime": time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
			},
			expectedOutput:        "dyntype timestamp: 2009-11-10T23:00:00Z",
			expectedRuntimeCost:   14,
			expectedEstimatedCost: cost.CostEstimate{Min: 14, Max: 14},
		},
		{
			name:       "dyntype support for duration",
			format:     "dyntype duration: %s",
			formatArgs: `dynDuration`,
			dynArgs: map[string]any{
				"dynDuration": mustParseDuration("2h25m47s"),
			},
			expectedOutput:        "dyntype duration: 8747s",
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for lists",
			format:     "dyntype list: %s",
			formatArgs: `dynList`,
			dynArgs: map[string]any{
				"dynList": []any{6, 4.2, "a string"},
			},
			expectedOutput:        `dyntype list: [6, 4.2, a string]`,
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "dyntype support for maps",
			format:     "dyntype map: %s",
			formatArgs: `dynMap`,
			dynArgs: map[string]any{
				"dynMap": map[any]any{
					"strKey": "x",
					true:     42,
					int64(6): mustParseDuration("7m2s"),
				},
			},
			expectedOutput:        `dyntype map: {6: 422s, strKey: x, true: 42}`,
			expectedRuntimeCost:   13,
			expectedEstimatedCost: cost.CostEstimate{Min: 13, Max: 13},
		},
		{
			name:       "message field support",
			format:     "message field msg.single_int32: %d, msg.single_double: %.1f",
			formatArgs: `msg.single_int32, msg.single_double`,
			dynArgs: map[string]any{
				"msg": &proto3pb.TestAllTypes{
					SingleInt32:  2,
					SingleDouble: 1.0,
				},
			},
			expectedOutput: `message field msg.single_int32: 2, msg.single_double: 1.0`,
		},
		{
			name:             "unrecognized formatting clause",
			format:           "%a",
			formatArgs:       "1",
			skipCompileCheck: true,
			err:              "could not parse formatting clause: unrecognized formatting clause \"a\"",
		},
		{
			name:             "out of bounds arg index",
			format:           "%d %d %d",
			formatArgs:       "0, 1",
			skipCompileCheck: true,
			err:              "index 2 out of range",
		},
		{
			name:             "string substitution is not allowed with binary clause",
			format:           "string is %b",
			formatArgs:       `"abc"`,
			skipCompileCheck: true,
			err:              "error during formatting: only ints, uints, and bools can be formatted as binary, was given string",
		},
		{
			name:             "duration substitution not allowed with decimal clause",
			format:           "%d",
			formatArgs:       `duration("30m2s")`,
			skipCompileCheck: true,
			err:              "error during formatting: decimal clause can only be used on ints, uints, and doubles, was given google.protobuf.Duration",
		},
		{
			name:             "string substitution not allowed with octal clause",
			format:           "octal: %o",
			formatArgs:       `"a string"`,
			skipCompileCheck: true,
			err:              "error during formatting: octal clause can only be used on ints and uints, was given string",
		},
		{
			name:             "double substitution not allowed with hex clause",
			format:           "double is %x",
			formatArgs:       "0.5",
			skipCompileCheck: true,
			err:              "error during formatting: only ints, uints, bytes, and strings can be formatted as hex, was given double",
		},
		{
			name:             "uppercase not allowed for scientific clause",
			format:           "double is %E",
			formatArgs:       "0.5",
			skipCompileCheck: true,
			err:              `could not parse formatting clause: unrecognized formatting clause "E"`,
		},
		{
			name:             "object not allowed",
			format:           "object is %s",
			formatArgs:       `ext.TestAllTypes{PbVal: test.TestAllTypes{}}`,
			skipCompileCheck: true,
			err:              "error during formatting: string clause can only be used on strings, bools, bytes, ints, doubles, maps, lists, types, durations, and timestamps, was given ext.TestAllTypes",
		},
		{
			name:             "object inside list",
			format:           "%s",
			formatArgs:       "[1, 2, ext.TestAllTypes{PbVal: test.TestAllTypes{}}]",
			skipCompileCheck: true,
			err:              "error during formatting: string clause can only be used on strings, bools, bytes, ints, doubles, maps, lists, types, durations, and timestamps, was given ext.TestAllTypes",
		},
		{
			name:             "object inside map",
			format:           "%s",
			formatArgs:       `{1: "a", 2: ext.TestAllTypes{}}`,
			skipCompileCheck: true,
			err:              "error during formatting: string clause can only be used on strings, bools, bytes, ints, doubles, maps, lists, types, durations, and timestamps, was given ext.TestAllTypes",
		},
		{
			name:             "null not allowed for %d",
			format:           "null: %d",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: decimal clause can only be used on ints, uints, and doubles, was given null_type",
		},
		{
			name:             "null not allowed for %e",
			format:           "null: %e",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: scientific clause can only be used on ints, uints, and doubles, was given null_type",
		},
		{
			name:             "null not allowed for %f",
			format:           "null: %f",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: fixed-point clause can only be used on ints, uints, and doubles, was given null_type",
		},
		{
			name:             "null not allowed for %x",
			format:           "null: %x",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: only ints, uints, bytes, and strings can be formatted as hex, was given null_type",
		},
		{
			name:             "null not allowed for %X",
			format:           "null: %X",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: only ints, uints, bytes, and strings can be formatted as hex, was given null_type",
		},
		{
			name:             "null not allowed for %b",
			format:           "null: %b",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: only ints, uints, and bools can be formatted as binary, was given null_type",
		},
		{
			name:             "null not allowed for %o",
			format:           "null: %o",
			formatArgs:       "null",
			skipCompileCheck: true,
			err:              "error during formatting: octal clause can only be used on ints and uints, was given null_type",
		},
		{
			name:       "compile-time cardinality check (too few for string)",
			format:     "%s %s",
			formatArgs: `"abc"`,
			err:        "index 1 out of range",
		},
		{
			name:       "compile-time cardinality check (too many for string)",
			format:     "%s %s",
			formatArgs: `"abc", "def", "ghi"`,
			err:        "too many arguments supplied to string.format (expected 2, got 3)",
		},
		{
			name:       "compile-time syntax check (unexpected end of string)",
			format:     "filler %",
			formatArgs: "",
			err:        "unexpected end of string",
		},
		{
			name:   "compile-time syntax check (unrecognized formatting clause)",
			format: "%j",
			// pass args here, otherwise the cardinality check will fail first
			formatArgs: "123",
			err:        `could not parse formatting clause: unrecognized formatting clause "j"`,
		},
		{
			name:       "compile-time %s check",
			format:     "object is %s",
			formatArgs: `ext.TestAllTypes{PbVal: test.TestAllTypes{}}`,
			err:        "error during formatting: string clause can only be used on strings, bools, bytes, ints, doubles, maps, lists, types, durations, and timestamps",
		},
		{
			name:       "compile-time check for objects inside list literal",
			format:     "list is %s",
			formatArgs: `[1, 2, ext.TestAllTypes{PbVal: test.TestAllTypes{}}]`,
			err:        "error during formatting: string clause can only be used on strings, bools, bytes, ints, doubles, maps, lists, types, durations, and timestamps",
		},
		{
			name:       "compile-time %d check",
			format:     "int is %d",
			formatArgs: "null",
			err:        "error during formatting: decimal clause can only be used on ints, uints, and doubles, was given null_type",
		},
		{
			name:       "compile-time %f check",
			format:     "double is %f",
			formatArgs: "true",
			err:        "error during formatting: fixed-point clause can only be used on ints, uints, and doubles, was given bool",
		},
		{
			name:       "compile-time precision syntax check",
			format:     "double is %.34",
			formatArgs: "5.0",
			err:        "could not parse formatting clause: error while parsing precision: could not find end of precision specifier",
		},
		{
			name:       "compile-time %e check",
			format:     "double is %e",
			formatArgs: "true",
			err:        "error during formatting: scientific clause can only be used on ints, uints, and doubles, was given bool",
		},
		{
			name:       "compile-time %b check",
			format:     "string is %b",
			formatArgs: `"a string"`,
			err:        "error during formatting: only ints, uints, and bools can be formatted as binary, was given string",
		},
		{
			name:       "compile-time %x check",
			format:     "%x is a double",
			formatArgs: "2.5",
			err:        "error during formatting: only ints, uints, bytes, and strings can be formatted as hex, was given double",
		},
		{
			name:       "compile-time %X check",
			format:     "%X is a double",
			formatArgs: "2.5",
			err:        "error during formatting: only ints, uints, bytes, and strings can be formatted as hex, was given double",
		},
		{
			name:       "compile-time %o check",
			format:     "an octal: %o",
			formatArgs: "3.14",
			err:        "octal clause can only be used on ints and uints, was given double",
		},
		{
			name:             "precision exceeds maximum",
			format:           "%.9999999f",
			formatArgs:       "3.14",
			skipCompileCheck: true,
			err:              "precision 9999999 exceeds maximum allowed precision 100",
		},
	}
	evalExpr := func(env *cel.Env, expr string, evalArgs any, expectedRuntimeCost uint64, expectedEstimatedCost cost.CostEstimate, t *testing.T) (ref.Val, error) {
		t.Logf("evaluating expr: %s", expr)
		parsedAst, issues := env.Parse(expr)
		if issues.Err() != nil {
			t.Fatalf("env.Parse(%v) failed: %v", expr, issues.Err())
		}
		checkedAst, issues := env.Check(parsedAst)
		if issues.Err() != nil {
			return nil, issues.Err()
		}
		evalOpts := make([]cel.ProgramOption, 0)
		costTracker := &noopCostEstimator{}
		if expectedRuntimeCost != 0 {
			evalOpts = append(evalOpts, cel.CostTracking(costTracker))
		}
		program, err := env.Program(checkedAst, evalOpts...)
		if err != nil {
			return nil, err
		}

		actualEstimatedCost, err := env.EstimateCost(checkedAst, costTracker)
		if err != nil {
			t.Fatal(err)
		}
		if expectedEstimatedCost.Min != 0 && expectedEstimatedCost.Max != 0 {
			if actualEstimatedCost.Min != expectedEstimatedCost.Min && actualEstimatedCost.Max != expectedEstimatedCost.Max {
				t.Fatalf("expected estimated cost range to be %v, was %v", expectedEstimatedCost, actualEstimatedCost)
			}
		}

		var out ref.Val
		var details *cel.EvalDetails
		if evalArgs != nil {
			out, details, err = program.Eval(evalArgs)
		} else {
			out, details, err = program.Eval(cel.NoVars())
		}

		if expectedRuntimeCost != 0 {
			if details == nil {
				t.Fatal("no EvalDetails available when runtime cost was expected")
			}
			if *details.ActualCost() != expectedRuntimeCost {
				t.Fatalf("expected runtime cost to be %d, was %d", expectedRuntimeCost, *details.ActualCost())
			}
			if expectedEstimatedCost.Min != 0 && expectedEstimatedCost.Max != 0 {
				if *details.ActualCost() < expectedEstimatedCost.Min || *details.ActualCost() > expectedEstimatedCost.Max {
					t.Fatalf("runtime cost %d outside of expected estimated cost range %v", *details.ActualCost(), expectedEstimatedCost)
				}
			}
		}
		return out, err
	}
	buildVariables := func(vars map[string]any) []cel.EnvOption {
		opts := make([]cel.EnvOption, len(vars))
		i := 0
		for name, value := range vars {
			t := cel.DynType
			switch v := value.(type) {
			case proto.Message:
				t = cel.ObjectType(string(v.ProtoReflect().Descriptor().FullName()))
			case types.Bool:
				t = cel.BoolType
			case types.Bytes:
				t = cel.BytesType
			case types.Double:
				t = cel.DoubleType
			case types.Duration:
				t = cel.DurationType
			case types.Int:
				t = cel.IntType
			case types.Null:
				t = cel.NullType
			case types.String:
				t = cel.StringType
			case types.Timestamp:
				t = cel.TimestampType
			case types.Uint:
				t = cel.UintType
			}
			opts[i] = cel.Variable(name, t)
			i++
		}
		return opts
	}
	buildOpts := func(skipCompileCheck bool, variables []cel.EnvOption) []cel.EnvOption {
		opts := []cel.EnvOption{
			Strings(StringsValidateFormatCalls(!skipCompileCheck)),
			cel.Container("ext"),
			cel.Abbrevs("google.expr.proto3.test"),
			cel.Types(&proto3pb.TestAllTypes{}),
			NativeTypes(
				reflect.TypeOf(&TestNestedType{}),
				reflect.ValueOf(&TestAllTypes{}),
			),
		}
		opts = append(opts, cel.ASTValidators(cel.ValidateHomogeneousAggregateLiterals()))
		opts = append(opts, variables...)
		return opts
	}
	runCase := func(format, formatArgs string, dynArgs map[string]any, skipCompileCheck bool, expectedRuntimeCost uint64, expectedEstimatedCost cost.CostEstimate, t *testing.T) (ref.Val, error) {
		env, err := cel.NewEnv(buildOpts(skipCompileCheck, buildVariables(dynArgs))...)
		if err != nil {
			t.Fatalf("cel.NewEnv() failed: %v", err)
		}
		expr := fmt.Sprintf("%q.format([%s])", format, formatArgs)
		if len(dynArgs) == 0 {
			return evalExpr(env, expr, cel.NoVars(), expectedRuntimeCost, expectedEstimatedCost, t)
		}
		return evalExpr(env, expr, dynArgs, expectedRuntimeCost, expectedEstimatedCost, t)
	}
	checkCase := func(output ref.Val, expectedOutput string, err error, expectedErr string, t *testing.T) {
		if err != nil {
			if expectedErr != "" {
				if !strings.Contains(err.Error(), expectedErr) {
					t.Fatalf("expected %q as error message, got %q", expectedErr, err.Error())
				}
			} else {
				t.Fatalf("unexpected error: %s", err)
			}
		} else {
			if output.Type() != types.StringType {
				t.Fatalf("expected test expr to eval to string (got %s instead)", output.Type().TypeName())
			} else {
				outputStr := output.Value().(string)
				if outputStr != expectedOutput {
					t.Errorf("expected %q as output, got %q", expectedOutput, outputStr)
				}
			}
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runCase(tt.format, tt.formatArgs, tt.dynArgs, tt.skipCompileCheck, tt.expectedRuntimeCost, tt.expectedEstimatedCost, t)
			checkCase(out, tt.expectedOutput, err, tt.err, t)
		})
	}
}

func TestStringFormatHeterogeneousLiteralsV2(t *testing.T) {
	tests := []struct {
		expr string
		out  string
	}{
		{
			expr: `"list: %s".format([[[1, 2, [3.0, 4]]]])`,
			out:  `list: [[1, 2, [3, 4]]]`,
		},
		{
			expr: `"list size: %d".format([[[1, 2, [3.0, 4]]].size()])`,
			out:  `list size: 1`,
		},
		{
			expr: `"list element: %s".format([[[1, 2, [3.0, 4]]][0]])`,
			out:  `list element: [1, 2, [3, 4]]`,
		},
	}
	env, err := cel.NewEnv(Strings(), cel.ASTValidators(cel.ValidateHomogeneousAggregateLiterals()))
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			ast, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("Eval() failed: %v", err)
			}
			if out.Value() != tc.out {
				t.Errorf("Eval() got %v, wanted %v", out, tc.out)
			}
		})
	}
}
