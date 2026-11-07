// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package typedef

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"

	celpb "cel.dev/expr"
)

// constantFromProto converts a cel.expr.Constant message to a CEL scalar
// constant, or returns nil if the constant kind is unset.
//
// Unlike ast.ProtoConstantAsVal, the deprecated duration_value and
// timestamp_value fields are supported since type definitions may use them
// for default values and aspect properties.
func constantFromProto(pb *celpb.Constant) (ref.Val, error) {
	switch kind := pb.GetConstantKind().(type) {
	case nil:
		return nil, nil
	case *celpb.Constant_NullValue:
		return types.NullValue, nil
	case *celpb.Constant_BoolValue:
		return types.Bool(kind.BoolValue), nil
	case *celpb.Constant_Int64Value:
		return types.Int(kind.Int64Value), nil
	case *celpb.Constant_Uint64Value:
		return types.Uint(kind.Uint64Value), nil
	case *celpb.Constant_DoubleValue:
		return types.Double(kind.DoubleValue), nil
	case *celpb.Constant_StringValue:
		return types.String(kind.StringValue), nil
	case *celpb.Constant_BytesValue:
		return types.Bytes(kind.BytesValue), nil
	case *celpb.Constant_DurationValue:
		d := kind.DurationValue
		if err := d.CheckValid(); err != nil {
			return nil, fmt.Errorf("%w: invalid duration constant: %v", ErrBadTypeDef, err)
		}
		// durationpb.Duration spans +/-10,000 years, whereas time.Duration
		// overflows past ~292 years and AsDuration silently saturates.
		dur := d.AsDuration()
		if !proto.Equal(durationpb.New(dur), d) {
			return nil, fmt.Errorf("%w: duration constant overflows time.Duration: %v", ErrBadTypeDef, d)
		}
		return types.Duration{Duration: dur}, nil
	case *celpb.Constant_TimestampValue:
		ts := kind.TimestampValue
		if err := ts.CheckValid(); err != nil {
			return nil, fmt.Errorf("%w: invalid timestamp constant: %v", ErrBadTypeDef, err)
		}
		return types.Timestamp{Time: ts.AsTime()}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported constant kind: %T", ErrBadTypeDef, kind)
	}
}

// constantToProto converts a CEL scalar constant to a cel.expr.Constant
// message. A nil value converts to an empty (unset) message.
func constantToProto(val ref.Val) (*celpb.Constant, error) {
	switch v := val.(type) {
	case nil:
		return &celpb.Constant{}, nil
	case types.Null:
		return &celpb.Constant{ConstantKind: &celpb.Constant_NullValue{NullValue: structpb.NullValue_NULL_VALUE}}, nil
	case types.Bool:
		return &celpb.Constant{ConstantKind: &celpb.Constant_BoolValue{BoolValue: bool(v)}}, nil
	case types.Int:
		return &celpb.Constant{ConstantKind: &celpb.Constant_Int64Value{Int64Value: int64(v)}}, nil
	case types.Uint:
		return &celpb.Constant{ConstantKind: &celpb.Constant_Uint64Value{Uint64Value: uint64(v)}}, nil
	case types.Double:
		return &celpb.Constant{ConstantKind: &celpb.Constant_DoubleValue{DoubleValue: float64(v)}}, nil
	case types.String:
		return &celpb.Constant{ConstantKind: &celpb.Constant_StringValue{StringValue: string(v)}}, nil
	case types.Bytes:
		return &celpb.Constant{ConstantKind: &celpb.Constant_BytesValue{BytesValue: []byte(v)}}, nil
	case types.Duration:
		return &celpb.Constant{ConstantKind: &celpb.Constant_DurationValue{DurationValue: durationpb.New(v.Duration)}}, nil
	case types.Timestamp:
		ts := timestamppb.New(v.Time)
		if err := ts.CheckValid(); err != nil {
			return nil, fmt.Errorf("%w: invalid timestamp constant: %v", ErrBadTypeDef, err)
		}
		return &celpb.Constant{ConstantKind: &celpb.Constant_TimestampValue{TimestampValue: ts}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported constant value %T", ErrBadTypeDef, val)
	}
}
