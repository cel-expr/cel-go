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

package bindings

import (
	"math"
	"testing"
	"time"

	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func TestBindingsDispatchAndHelpers(t *testing.T) {
	ts := types.Timestamp{Time: time.Date(2025, 3, 15, 10, 30, 45, 123000000, time.UTC)}
	dur := types.Duration{Duration: time.Hour + 2*time.Minute + 3*time.Second}

	// Timestamp / duration top-level dispatchers for unchecked ASTs.
	tsCases := []struct {
		op     string
		hasDur bool
		wantTS int64
		wantTZ int64
		wantDr int64
	}{
		{overloads.TimeGetFullYear, false, 2025, 2025, 0},
		{overloads.TimeGetMonth, false, 2, 2, 0},
		{overloads.TimeGetDayOfYear, false, 73, 73, 0},
		{overloads.TimeGetDayOfMonth, false, 14, 14, 0},
		{overloads.TimeGetDate, false, 15, 15, 0},
		{overloads.TimeGetDayOfWeek, false, 6, 6, 0},
		{overloads.TimeGetHours, true, 10, 8, 1},
		{overloads.TimeGetMinutes, true, 30, 30, 62},
		{overloads.TimeGetSeconds, true, 45, 45, 3723},
		{overloads.TimeGetMilliseconds, true, 123, 123, 3723000},
	}
	for _, tc := range tsCases {
		fn := Lookup(tc.op).Function
		if got := fn(ts); got != types.Int(tc.wantTS) {
			t.Errorf("%s(ts) = %v, want %d", tc.op, got, tc.wantTS)
		}
		if got := fn(ts, types.String("-02:00")); got != types.Int(tc.wantTZ) {
			t.Errorf("%s(ts, -02:00) = %v, want %d", tc.op, got, tc.wantTZ)
		}
		if tc.hasDur {
			if got := fn(dur); got != types.Int(tc.wantDr) {
				t.Errorf("%s(dur) = %v, want %d", tc.op, got, tc.wantDr)
			}
		} else if got := fn(dur); !types.IsError(got) {
			t.Errorf("%s(dur) = %v, want error", tc.op, got)
		}
	}
	// Singleton operators: LogicalNot, Negate, Relations error paths.
	if got := Lookup(operators.LogicalNot).Unary(types.Int(1)); !types.IsError(got) {
		t.Errorf("LogicalNot(1) = %v, want error", got)
	}
	if got := Lookup(operators.Negate).Unary(types.True); !types.IsError(got) {
		t.Errorf("Negate(true) = %v, want error", got)
	}
	for _, op := range []string{operators.Less, operators.LessEquals, operators.Greater, operators.GreaterEquals} {
		fn := Lookup(op).Binary
		if got := fn(types.Int(1), types.String("a")); !types.IsError(got) {
			t.Errorf("%s(1, 'a') = %v, want error", op, got)
		}
		if got := fn(types.Int(1), types.Int(1)); got != types.Bool(op == operators.LessEquals || op == operators.GreaterEquals) {
			t.Errorf("%s(1, 1) = %v", op, got)
		}
		if got := fn(types.Int(2), types.Int(1)); got != types.Bool(op == operators.Greater || op == operators.GreaterEquals) {
			t.Errorf("%s(2, 1) = %v", op, got)
		}
	}
	// Invalid timezone offsets in inTimeZone.
	for _, badTZ := range []string{"bad:00", "00:bad", "25:00", "00:99", "Invalid/Zone"} {
		if got := Lookup(overloads.TimeGetHours).Function(ts, types.String(badTZ)); !types.IsError(got) {
			t.Errorf("getHours(ts, %q) = %v, want error", badTZ, got)
		}
	}
	hoursFn := Lookup(overloads.TimeGetHours).Function
	if got := hoursFn(ts); got != types.Int(10) {
		t.Errorf("getHours(ts) = %v, want 10", got)
	}
	if got := hoursFn(ts, types.String("-02:00")); got != types.Int(8) {
		t.Errorf("getHours(ts, -02:00) = %v, want 8", got)
	}
	if got := hoursFn(dur); got != types.Int(1) {
		t.Errorf("getHours(dur) = %v, want 1", got)
	}
	if got := hoursFn(types.Int(1)); !types.IsError(got) {
		t.Errorf("getHours(1) = %v, want error", got)
	}
	if got := hoursFn(ts, types.Int(1)); !types.IsError(got) {
		t.Errorf("getHours(ts, 1) = %v, want error", got)
	}

	// Helpers: inAggregate, isNaN, noBinaryOverrides, noFunctionOverrides.
	if got := inAggregate(types.Int(1), types.Int(2)); !types.IsError(got) {
		t.Errorf("inAggregate(1, 2) = %v, want error", got)
	}
	if isNaN(types.Int(1)) || !isNaN(types.Double(math.NaN())) {
		t.Error("isNaN() mismatch")
	}
	if got := noBinaryOverrides(types.True, types.False); !types.IsError(got) {
		t.Errorf("noBinaryOverrides() = %v, want error", got)
	}
	if got := noFunctionOverrides(types.True); !types.IsError(got) {
		t.Errorf("noFunctionOverrides() = %v, want error", got)
	}

	// Relational NaN handling.
	for _, op := range []string{operators.Less, operators.LessEquals, operators.Greater, operators.GreaterEquals} {
		fn := Lookup(op).Binary
		if got := fn(types.Double(math.NaN()), types.Double(1)); got != types.False {
			t.Errorf("%s(NaN, 1) = %v, want false", op, got)
		}
		if got := fn(types.Int(1), types.Int(2)); !ref.Val(got).Equal(types.Bool(op == operators.Less || op == operators.LessEquals)).(types.Bool) {
			t.Errorf("%s(1, 2) = %v", op, got)
		}
	}
}
