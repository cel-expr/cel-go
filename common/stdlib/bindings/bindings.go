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

// Package bindings provides the runtime function overloads for the CEL standard library without
// depending on type-checker declarations.
package bindings

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

var (
	initOnce     sync.Once
	stdOverloads []*functions.Overload
	byOperator   map[string]*functions.Overload
	utcTZ        = types.String("UTC")
)

func initBindings() {
	stdOverloads = []*functions.Overload{
		// Logical operators. Special-cased within the interpreter.
		{Operator: operators.Conditional, Function: noFunctionOverrides},
		{Operator: operators.LogicalAnd, Binary: noBinaryOverrides},
		{Operator: operators.LogicalOr, Binary: noBinaryOverrides},
		{
			Operator: operators.LogicalNot,
			Unary: func(val ref.Val) ref.Val {
				b, ok := val.(types.Bool)
				if !ok {
					return types.MaybeNoSuchOverloadErr(val)
				}
				return b.Negate()
			},
		},

		// Comprehension short-circuiting related function.
		{Operator: overloads.NotStrictlyFalse, Unary: notStrictlyFalse, NonStrict: true},
		{Operator: operators.OldNotStrictlyFalse, Unary: notStrictlyFalse, NonStrict: true},

		// Equality / inequality. Special-cased in the interpreter.
		{Operator: operators.Equals, Binary: noBinaryOverrides},
		{Operator: operators.NotEquals, Binary: noBinaryOverrides},

		// Mathematical operators.
		{
			Operator:     operators.Add,
			OperandTrait: traits.AdderType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return lhs.(traits.Adder).Add(rhs)
			},
		},
		{
			Operator:     operators.Divide,
			OperandTrait: traits.DividerType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return lhs.(traits.Divider).Divide(rhs)
			},
		},
		{
			Operator:     operators.Modulo,
			OperandTrait: traits.ModderType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return lhs.(traits.Modder).Modulo(rhs)
			},
		},
		{
			Operator:     operators.Multiply,
			OperandTrait: traits.MultiplierType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return lhs.(traits.Multiplier).Multiply(rhs)
			},
		},
		{
			Operator:     operators.Negate,
			OperandTrait: traits.NegatorType,
			Unary: func(val ref.Val) ref.Val {
				if types.IsBool(val) {
					return types.MaybeNoSuchOverloadErr(val)
				}
				return val.(traits.Negater).Negate()
			},
		},
		{
			Operator:     operators.Subtract,
			OperandTrait: traits.SubtractorType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return lhs.(traits.Subtractor).Subtract(rhs)
			},
		},

		// Relations operators.
		{
			Operator:     operators.Less,
			OperandTrait: traits.ComparerType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				if isNaN(lhs) || isNaN(rhs) {
					return types.False
				}
				cmp := lhs.(traits.Comparer).Compare(rhs)
				if cmp == types.IntNegOne {
					return types.True
				}
				if cmp == types.IntOne || cmp == types.IntZero {
					return types.False
				}
				return cmp
			},
		},
		{
			Operator:     operators.LessEquals,
			OperandTrait: traits.ComparerType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				if isNaN(lhs) || isNaN(rhs) {
					return types.False
				}
				cmp := lhs.(traits.Comparer).Compare(rhs)
				if cmp == types.IntNegOne || cmp == types.IntZero {
					return types.True
				}
				if cmp == types.IntOne {
					return types.False
				}
				return cmp
			},
		},
		{
			Operator:     operators.Greater,
			OperandTrait: traits.ComparerType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				if isNaN(lhs) || isNaN(rhs) {
					return types.False
				}
				cmp := lhs.(traits.Comparer).Compare(rhs)
				if cmp == types.IntOne {
					return types.True
				}
				if cmp == types.IntNegOne || cmp == types.IntZero {
					return types.False
				}
				return cmp
			},
		},
		{
			Operator:     operators.GreaterEquals,
			OperandTrait: traits.ComparerType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				if isNaN(lhs) || isNaN(rhs) {
					return types.False
				}
				cmp := lhs.(traits.Comparer).Compare(rhs)
				if cmp == types.IntOne || cmp == types.IntZero {
					return types.True
				}
				if cmp == types.IntNegOne {
					return types.False
				}
				return cmp
			},
		},
		{
			Operator:     operators.Index,
			OperandTrait: traits.IndexerType,
			Binary: func(lhs, rhs ref.Val) ref.Val {
				return lhs.(traits.Indexer).Get(rhs)
			},
		},

		// Collections operators.
		{Operator: operators.In, Binary: inAggregate},
		{Operator: operators.OldIn, Binary: inAggregate},
		{Operator: overloads.DeprecatedIn, Binary: inAggregate},
		{
			Operator:     overloads.Size,
			OperandTrait: traits.SizerType,
			Unary: func(val ref.Val) ref.Val {
				return val.(traits.Sizer).Size()
			},
		},

		// Type conversions.
		{Operator: overloads.TypeConvertType, Unary: convertToType(types.TypeType)},
		{Operator: overloads.BoolToBool, Unary: identity},
		{Operator: overloads.StringToBool, Unary: convertToType(types.BoolType)},
		{Operator: overloads.BytesToBytes, Unary: identity},
		{Operator: overloads.StringToBytes, Unary: convertToType(types.BytesType)},
		{Operator: overloads.DoubleToDouble, Unary: identity},
		{Operator: overloads.IntToDouble, Unary: convertToType(types.DoubleType)},
		{Operator: overloads.StringToDouble, Unary: convertToType(types.DoubleType)},
		{Operator: overloads.UintToDouble, Unary: convertToType(types.DoubleType)},
		{Operator: overloads.DurationToDuration, Unary: identity},
		{Operator: overloads.StringToDuration, Unary: convertToType(types.DurationType)},
		{Operator: overloads.TypeConvertDyn, Unary: identity},
		{Operator: overloads.IntToInt, Unary: identity},
		{Operator: overloads.DoubleToInt, Unary: convertToType(types.IntType)},
		{Operator: overloads.DurationToInt, Unary: convertToType(types.IntType)},
		{Operator: overloads.StringToInt, Unary: convertToType(types.IntType)},
		{Operator: overloads.TimestampToInt, Unary: convertToType(types.IntType)},
		{Operator: overloads.UintToInt, Unary: convertToType(types.IntType)},
		{Operator: overloads.StringToString, Unary: identity},
		{Operator: overloads.BoolToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.BytesToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.DoubleToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.DurationToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.IntToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.TimestampToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.UintToString, Unary: convertToType(types.StringType)},
		{Operator: overloads.TimestampToTimestamp, Unary: identity},
		{Operator: overloads.IntToTimestamp, Unary: convertToType(types.TimestampType)},
		{Operator: overloads.StringToTimestamp, Unary: convertToType(types.TimestampType)},
		{Operator: overloads.UintToUint, Unary: identity},
		{Operator: overloads.DoubleToUint, Unary: convertToType(types.UintType)},
		{Operator: overloads.IntToUint, Unary: convertToType(types.UintType)},
		{Operator: overloads.StringToUint, Unary: convertToType(types.UintType)},

		// String functions.
		{Operator: overloads.ContainsString, Binary: types.StringContains},
		{Operator: overloads.EndsWithString, Binary: types.StringEndsWith},
		{Operator: overloads.StartsWithString, Binary: types.StringStartsWith},
		{
			Operator:     overloads.Matches,
			OperandTrait: traits.MatcherType,
			Binary: func(str, pat ref.Val) ref.Val {
				return str.(traits.Matcher).Match(pat)
			},
		},

		// Timestamp / duration functions.
		{
			Operator: overloads.TimestampToYear,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetFullYear(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToYearWithTz, Binary: timestampGetFullYear},
		{
			Operator: overloads.TimestampToMonth,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetMonth(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToMonthWithTz, Binary: timestampGetMonth},
		{
			Operator: overloads.TimestampToDayOfYear,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetDayOfYear(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToDayOfYearWithTz, Binary: timestampGetDayOfYear},
		{
			Operator: overloads.TimestampToDayOfMonthZeroBased,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetDayOfMonthZeroBased(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToDayOfMonthZeroBasedWithTz, Binary: timestampGetDayOfMonthZeroBased},
		{
			Operator: overloads.TimestampToDayOfMonthOneBased,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetDayOfMonthOneBased(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToDayOfMonthOneBasedWithTz, Binary: timestampGetDayOfMonthOneBased},
		{
			Operator: overloads.TimestampToDayOfWeek,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetDayOfWeek(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToDayOfWeekWithTz, Binary: timestampGetDayOfWeek},
		{
			Operator: overloads.TimestampToHours,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetHours(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToHoursWithTz, Binary: timestampGetHours},
		{Operator: overloads.DurationToHours, Unary: types.DurationGetHours},
		{
			Operator: overloads.TimestampToMinutes,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetMinutes(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToMinutesWithTz, Binary: timestampGetMinutes},
		{Operator: overloads.DurationToMinutes, Unary: types.DurationGetMinutes},
		{
			Operator: overloads.TimestampToSeconds,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetSeconds(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToSecondsWithTz, Binary: timestampGetSeconds},
		{Operator: overloads.DurationToSeconds, Unary: types.DurationGetSeconds},
		{
			Operator: overloads.TimestampToMilliseconds,
			Unary:    func(ts ref.Val) ref.Val { return timestampGetMilliseconds(ts, utcTZ) },
		},
		{Operator: overloads.TimestampToMillisecondsWithTz, Binary: timestampGetMilliseconds},
		{Operator: overloads.DurationToMilliseconds, Unary: types.DurationGetMilliseconds},

		// Top-level function dispatchers for unchecked ASTs.
		{Operator: operators.NotStrictlyFalse, Unary: notStrictlyFalse, NonStrict: true},
		{Operator: overloads.TypeConvertBool, Unary: convertToType(types.BoolType)},
		{Operator: overloads.TypeConvertBytes, Unary: convertToType(types.BytesType)},
		{Operator: overloads.TypeConvertDouble, Unary: convertToType(types.DoubleType)},
		{Operator: overloads.TypeConvertDuration, Unary: convertToType(types.DurationType)},
		{Operator: overloads.TypeConvertInt, Unary: convertToType(types.IntType)},
		{Operator: overloads.TypeConvertString, Unary: convertToType(types.StringType)},
		{Operator: overloads.TypeConvertTimestamp, Unary: convertToType(types.TimestampType)},
		{Operator: overloads.TypeConvertUint, Unary: convertToType(types.UintType)},
		{Operator: overloads.Contains, Binary: types.StringContains},
		{Operator: overloads.EndsWith, Binary: types.StringEndsWith},
		{Operator: overloads.StartsWith, Binary: types.StringStartsWith},
		{Operator: overloads.TimeGetFullYear, Function: timestampAccessorDispatch(overloads.TimeGetFullYear, timestampGetFullYear, nil)},
		{Operator: overloads.TimeGetMonth, Function: timestampAccessorDispatch(overloads.TimeGetMonth, timestampGetMonth, nil)},
		{Operator: overloads.TimeGetDayOfYear, Function: timestampAccessorDispatch(overloads.TimeGetDayOfYear, timestampGetDayOfYear, nil)},
		{Operator: overloads.TimeGetDayOfMonth, Function: timestampAccessorDispatch(overloads.TimeGetDayOfMonth, timestampGetDayOfMonthZeroBased, nil)},
		{Operator: overloads.TimeGetDate, Function: timestampAccessorDispatch(overloads.TimeGetDate, timestampGetDayOfMonthOneBased, nil)},
		{Operator: overloads.TimeGetDayOfWeek, Function: timestampAccessorDispatch(overloads.TimeGetDayOfWeek, timestampGetDayOfWeek, nil)},
		{Operator: overloads.TimeGetHours, Function: timestampAccessorDispatch(overloads.TimeGetHours, timestampGetHours, types.DurationGetHours)},
		{Operator: overloads.TimeGetMinutes, Function: timestampAccessorDispatch(overloads.TimeGetMinutes, timestampGetMinutes, types.DurationGetMinutes)},
		{Operator: overloads.TimeGetSeconds, Function: timestampAccessorDispatch(overloads.TimeGetSeconds, timestampGetSeconds, types.DurationGetSeconds)},
		{Operator: overloads.TimeGetMilliseconds, Function: timestampAccessorDispatch(overloads.TimeGetMilliseconds, timestampGetMilliseconds, types.DurationGetMilliseconds)},
	}

	byOperator = make(map[string]*functions.Overload, len(stdOverloads))
	for _, o := range stdOverloads {
		byOperator[o.Operator] = o
	}
}

// Overloads returns the runtime function overloads for the CEL standard library.
func Overloads() []*functions.Overload {
	initOnce.Do(initBindings)
	return slices.Clone(stdOverloads)
}

// Lookup returns the standard library overload for the given operator or overload id, or nil if
// not found.
func Lookup(operator string) *functions.Overload {
	initOnce.Do(initBindings)
	return byOperator[operator]
}

func notStrictlyFalse(value ref.Val) ref.Val {
	if types.IsBool(value) {
		return value
	}
	return types.True
}

func inAggregate(lhs ref.Val, rhs ref.Val) ref.Val {
	if rhs.Type().HasTrait(traits.ContainerType) {
		return rhs.(traits.Container).Contains(lhs)
	}
	return types.MaybeNoSuchOverloadErr(rhs)
}

func noBinaryOverrides(rhs, lhs ref.Val) ref.Val {
	return types.NoSuchOverloadErr()
}

func isNaN(val ref.Val) bool {
	d, ok := val.(types.Double)
	if !ok {
		return false
	}
	return math.IsNaN(float64(d))
}

func noFunctionOverrides(args ...ref.Val) ref.Val {
	return types.NoSuchOverloadErr()
}

func identity(val ref.Val) ref.Val {
	return val
}

func convertToType(t ref.Type) functions.UnaryOp {
	return func(val ref.Val) ref.Val {
		return val.ConvertToType(t)
	}
}

func timestampGetFullYear(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Year())
}

func timestampGetMonth(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	// CEL spec indicates that the month should be 0-based, but the Time value
	// for Month() is 1-based.
	return types.Int(t.Month() - 1)
}

func timestampGetDayOfYear(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.YearDay() - 1)
}

func timestampGetDayOfMonthZeroBased(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Day() - 1)
}

func timestampGetDayOfMonthOneBased(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Day())
}

func timestampGetDayOfWeek(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Weekday())
}

func timestampGetHours(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Hour())
}

func timestampGetMinutes(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Minute())
}

func timestampGetSeconds(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Second())
}

func timestampGetMilliseconds(ts, tz ref.Val) ref.Val {
	t, err := inTimeZone(ts, tz)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.Int(t.Nanosecond() / 1000000)
}

func inTimeZone(ts, tz ref.Val) (time.Time, error) {
	t := ts.(types.Timestamp)
	val := string(tz.(types.String))
	before, after, ok := strings.Cut(val, ":")
	if !ok {
		loc, err := time.LoadLocation(val)
		if err != nil {
			return time.Time{}, err
		}
		return t.In(loc), nil
	}

	// If the input is not the name of a timezone (for example, 'US/Central'), it should be a numerical offset from UTC
	// in the format ^(+|-)([01]\d|2[0-3]):[0-5][0-9]$. The numerical input is parsed in terms of hours and minutes.
	hr, err := strconv.Atoi(string(before))
	if err != nil {
		return time.Time{}, err
	}
	min, err := strconv.Atoi(string(after))
	if err != nil {
		return time.Time{}, err
	}
	if hr < -23 || hr > 23 {
		return time.Time{}, fmt.Errorf("timezone offset hours out of range [-23, 23]: %s", val)
	}
	if min < 0 || min > 59 {
		return time.Time{}, fmt.Errorf("timezone offset minutes out of range [0, 59]: %s", val)
	}
	var offset int
	if string(val[0]) == "-" {
		offset = hr*60 - min
	} else {
		offset = hr*60 + min
	}
	secondsEastOfUTC := int((time.Duration(offset) * time.Minute).Seconds())
	timezone := time.FixedZone("", secondsEastOfUTC)
	return t.In(timezone), nil
}

func timestampAccessorDispatch(name string, tsFn functions.BinaryOp, durFn functions.UnaryOp) functions.FunctionOp {
	return func(args ...ref.Val) ref.Val {
		switch len(args) {
		case 1:
			if args[0].Type() == types.TimestampType {
				return tsFn(args[0], utcTZ)
			}
			if durFn != nil && args[0].Type() == types.DurationType {
				return durFn(args[0])
			}
		case 2:
			if args[0].Type() == types.TimestampType && args[1].Type() == types.StringType {
				return tsFn(args[0], args[1])
			}
		}
		return types.NoSuchOverloadErr()
	}
}
