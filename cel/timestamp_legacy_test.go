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

package cel

import (
	"strings"
	"testing"
	"time"

	"cel.dev/cel-go/common/types"
)

func TestStrictTimestampParsingConversion(t *testing.T) {
	tests := []struct {
		in string
		// want is the expected instant (RFC 3339) when strict parsing is disabled, or empty if the
		// input is rejected in both modes.
		want string
		// strictOK is true if strict (default) parsing also accepts the input.
		strictOK bool
	}{
		{in: "2024-01-02T03:04:05Z", want: "2024-01-02T03:04:05Z", strictOK: true},
		{in: "2024-01-02T03:04:05.123456789+05:30", want: "2024-01-01T21:34:05.123456789Z", strictOK: true},
		{in: "2024-01-02T03:04:05,5Z", want: "2024-01-02T03:04:05.5Z"},
		{in: "2024-01-02T3:04:05Z", want: "2024-01-02T03:04:05Z"},
		{in: "2024-01-02T03:04:05+24:00", want: "2024-01-01T03:04:05Z"},
		// Out-of-range offset minutes carry into the hour under legacy parsing.
		{in: "2024-01-02T03:04:05+05:60", want: "2024-01-01T21:04:05Z"},
		// Rejected by time.Parse, and therefore by both modes.
		{in: "2024-01-02 03:04:05Z"},
		{in: "2024-01-02T03:04:05"},
		{in: "2024-02-30T03:04:05Z"},
	}
	strictEnv := testEnv(t, Variable("s", StringType))
	explicitStrictEnv := testEnv(t, Variable("s", StringType), StrictTimestampParsing(true))
	legacyEnv := testEnv(t, Variable("s", StringType), StrictTimestampParsing(false))
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			vars := map[string]any{"s": tc.in}
			for name, env := range map[string]*Env{"default": strictEnv, "explicit-strict": explicitStrictEnv} {
				_, err := interpret(t, env, "timestamp(s)", vars)
				if tc.strictOK && err != nil {
					t.Errorf("%s: timestamp(%q) failed: %v", name, tc.in, err)
				}
				if !tc.strictOK && err == nil {
					t.Errorf("%s: timestamp(%q) succeeded, wanted error", name, tc.in)
				}
			}
			out, err := interpret(t, legacyEnv, "timestamp(s)", vars)
			if tc.want == "" {
				if err == nil {
					t.Errorf("legacy: timestamp(%q) got %v, wanted error", tc.in, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("legacy: timestamp(%q) failed: %v", tc.in, err)
			}
			want, _ := time.Parse(time.RFC3339Nano, tc.want)
			if got := out.(types.Timestamp).Time; !got.Equal(want) {
				t.Errorf("legacy: timestamp(%q) got %v, wanted %v", tc.in, got.UTC().Format(time.RFC3339Nano), tc.want)
			}
		})
	}
}

func TestStrictTimestampParsingOverflow(t *testing.T) {
	env := testEnv(t, StrictTimestampParsing(false))
	_, err := interpret(t, env, "timestamp('0000-01-01T00:00:00Z')", NoVars())
	if err == nil || !strings.Contains(err.Error(), "timestamp overflow") {
		t.Errorf("got %v, wanted timestamp overflow error", err)
	}
}

func TestStrictTimestampParsingTimezoneOffsets(t *testing.T) {
	strictEnv := testEnv(t, Variable("x", TimestampType))
	legacyEnv := testEnv(t, Variable("x", TimestampType), StrictTimestampParsing(false))
	// 2024-01-02T03:04:05Z
	vars := map[string]any{"x": time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}
	tests := []struct {
		expr       string
		want       int64
		strictFail bool
	}{
		{expr: "x.getHours('+05:30')", want: 8},
		{expr: "x.getHours('America/New_York')", want: 22},
		{expr: "x.getHours('+24:00')", want: 3, strictFail: true},
		{expr: "x.getHours('+00:99')", want: 4, strictFail: true},
		{expr: "x.getMinutes('+05:60')", want: 4, strictFail: true},
		{expr: "x.getFullYear('-24:00')", want: 2024, strictFail: true},
		{expr: "x.getMonth('+24:00')", want: 0, strictFail: true},
		{expr: "x.getDayOfYear('+24:00')", want: 2, strictFail: true},
		{expr: "x.getDayOfMonth('+24:00')", want: 2, strictFail: true},
		{expr: "x.getDate('+24:00')", want: 3, strictFail: true},
		{expr: "x.getDayOfWeek('+24:00')", want: 3, strictFail: true},
		{expr: "x.getSeconds('+24:00')", want: 5, strictFail: true},
		{expr: "x.getMilliseconds('+24:00')", want: 0, strictFail: true},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := interpret(t, strictEnv, tc.expr, vars)
			if tc.strictFail && err == nil {
				t.Errorf("strict: %s succeeded, wanted error", tc.expr)
			}
			if !tc.strictFail && err != nil {
				t.Errorf("strict: %s failed: %v", tc.expr, err)
			}
			out, err := interpret(t, legacyEnv, tc.expr, vars)
			if err != nil {
				t.Fatalf("legacy: %s failed: %v", tc.expr, err)
			}
			if out.Equal(types.Int(tc.want)) != types.True {
				t.Errorf("legacy: %s got %v, wanted %d", tc.expr, out, tc.want)
			}
		})
	}
}

func TestStrictTimestampParsingLiteralValidation(t *testing.T) {
	expr := "timestamp('2024-01-02T3:04:05Z') < timestamp('2025-01-01T00:00:00Z')"

	strictEnv := testEnv(t, ASTValidators(ValidateTimestampLiterals()))
	if _, iss := strictEnv.Compile(expr); iss.Err() == nil {
		t.Errorf("strict: Compile(%q) succeeded, wanted literal validation error", expr)
	}

	legacyEnv := testEnv(t, ASTValidators(ValidateTimestampLiterals()), StrictTimestampParsing(false))
	out, err := interpret(t, legacyEnv, expr, NoVars())
	if err != nil {
		t.Fatalf("legacy: %q failed: %v", expr, err)
	}
	if out != types.True {
		t.Errorf("legacy: %q got %v, wanted true", expr, out)
	}
}

func TestStrictTimestampParsingExtend(t *testing.T) {
	// The relaxed overloads are inherited by extended environments.
	base := testEnv(t, StrictTimestampParsing(false))
	env, err := base.Extend(Variable("s", StringType))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	if _, err := interpret(t, env, "timestamp(s)", map[string]any{"s": "2024-01-02T3:04:05Z"}); err != nil {
		t.Errorf("extended legacy env: timestamp() failed: %v", err)
	}
}
