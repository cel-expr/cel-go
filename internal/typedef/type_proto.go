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

// This file only references celpb.Type (checked.proto). Keep celpb.Constant
// (syntax.proto) conversions in separate files: the two messages share the
// cel.dev/expr Go package in OSS, but map to distinct Go packages in other
// build environments.
//
// typeFromProto and typeToProto intentionally do not delegate to
// types.ProtoAsType and types.TypeToProto: type definitions must preserve
// unset types (types.UnspecifiedKind), encode cel-go's opaque "function" type
// as a cel.expr.Type function type, bound the nesting depth of untrusted
// input, and report errors as ErrBadTypeDef.

import (
	"fmt"

	"cel.dev/cel-go/common/types"

	celpb "cel.dev/expr"
)

const (
	// functionTypeName is the reserved opaque type name cel-go uses to model
	// cel.expr.Type function types (see checker.NewFunctionType).
	functionTypeName = "function"

	// maxTypeDepth bounds the nesting depth of type conversions so that
	// untrusted, deeply nested types cannot exhaust the stack.
	maxTypeDepth = 250
)

// typeFromProto converts a cel.expr.Type message to a CEL-native type.
//
// An unset type kind (including a nil message) converts to a zero-value
// *types.Type (types.UnspecifiedKind). Function types convert to the opaque
// "function" type used by the checker, whose first parameter is the result
// type followed by the argument types.
//
// Unlike cel-cpp, unspecified primitive, wrapper, and well-known types are
// rejected since types.Type has no representation for them. Message types
// naming protobuf well-known types (e.g. google.protobuf.Int64Value or
// google.protobuf.Struct) are canonicalized to their CEL equivalents by
// types.NewObjectType, so they re-encode as wrapper, well_known, map, list,
// null, or dyn types rather than message_type.
func typeFromProto(pb *celpb.Type) (*types.Type, error) {
	return typeFromProtoDepth(pb, 0)
}

func typeFromProtoDepth(pb *celpb.Type, depth int) (*types.Type, error) {
	if depth > maxTypeDepth {
		return nil, fmt.Errorf("%w: type nesting exceeds max depth of %d", ErrBadTypeDef, maxTypeDepth)
	}
	depth++
	switch kind := pb.GetTypeKind().(type) {
	case nil:
		return &types.Type{}, nil
	case *celpb.Type_Dyn:
		return types.DynType, nil
	case *celpb.Type_Null:
		return types.NullType, nil
	case *celpb.Type_Primitive:
		return primitiveFromProto(kind.Primitive)
	case *celpb.Type_Wrapper:
		t, err := primitiveFromProto(kind.Wrapper)
		if err != nil {
			return nil, err
		}
		return types.NewNullableType(t), nil
	case *celpb.Type_WellKnown:
		switch kind.WellKnown {
		case celpb.Type_ANY:
			return types.AnyType, nil
		case celpb.Type_TIMESTAMP:
			return types.TimestampType, nil
		case celpb.Type_DURATION:
			return types.DurationType, nil
		default:
			return nil, fmt.Errorf("%w: unsupported WellKnownType: %v", ErrBadTypeDef, kind.WellKnown)
		}
	case *celpb.Type_ListType_:
		elem, err := typeFromProtoDepth(kind.ListType.GetElemType(), depth)
		if err != nil {
			return nil, err
		}
		return types.NewListType(elem), nil
	case *celpb.Type_MapType_:
		key, err := typeFromProtoDepth(kind.MapType.GetKeyType(), depth)
		if err != nil {
			return nil, err
		}
		val, err := typeFromProtoDepth(kind.MapType.GetValueType(), depth)
		if err != nil {
			return nil, err
		}
		return types.NewMapType(key, val), nil
	case *celpb.Type_Function:
		argTypes := kind.Function.GetArgTypes()
		params := make([]*types.Type, 0, len(argTypes)+1)
		result, err := typeFromProtoDepth(kind.Function.GetResultType(), depth)
		if err != nil {
			return nil, err
		}
		params = append(params, result)
		for _, argType := range argTypes {
			arg, err := typeFromProtoDepth(argType, depth)
			if err != nil {
				return nil, err
			}
			params = append(params, arg)
		}
		return types.NewOpaqueType(functionTypeName, params...), nil
	case *celpb.Type_MessageType:
		return types.NewObjectType(kind.MessageType), nil
	case *celpb.Type_TypeParam:
		return types.NewTypeParamType(kind.TypeParam), nil
	case *celpb.Type_Type:
		if kind.Type.GetTypeKind() == nil {
			return types.TypeType, nil
		}
		param, err := typeFromProtoDepth(kind.Type, depth)
		if err != nil {
			return nil, err
		}
		return types.NewTypeTypeWithParam(param), nil
	case *celpb.Type_Error:
		return types.ErrorType, nil
	case *celpb.Type_AbstractType_:
		name := kind.AbstractType.GetName()
		if name == functionTypeName {
			// Reject the reserved name so it cannot be confused with a function type.
			return nil, fmt.Errorf("%w: abstract type name %q is reserved", ErrBadTypeDef, name)
		}
		paramTypes := kind.AbstractType.GetParameterTypes()
		params := make([]*types.Type, 0, len(paramTypes))
		for _, paramType := range paramTypes {
			param, err := typeFromProtoDepth(paramType, depth)
			if err != nil {
				return nil, err
			}
			params = append(params, param)
		}
		return types.NewOpaqueType(name, params...), nil
	default:
		return nil, fmt.Errorf("%w: unsupported type kind: %T", ErrBadTypeDef, kind)
	}
}

func primitiveFromProto(primitive celpb.Type_PrimitiveType) (*types.Type, error) {
	switch primitive {
	case celpb.Type_BOOL:
		return types.BoolType, nil
	case celpb.Type_INT64:
		return types.IntType, nil
	case celpb.Type_UINT64:
		return types.UintType, nil
	case celpb.Type_DOUBLE:
		return types.DoubleType, nil
	case celpb.Type_STRING:
		return types.StringType, nil
	case celpb.Type_BYTES:
		return types.BytesType, nil
	default:
		return nil, fmt.Errorf("%w: unsupported PrimitiveType: %v", ErrBadTypeDef, primitive)
	}
}

// typeToProto converts a CEL-native type to a cel.expr.Type message.
//
// A nil or zero-value *types.Type (types.UnspecifiedKind) converts to an empty
// message.
func typeToProto(t *types.Type) (*celpb.Type, error) {
	return typeToProtoDepth(t, 0)
}

func typeToProtoDepth(t *types.Type, depth int) (*celpb.Type, error) {
	if depth > maxTypeDepth {
		return nil, fmt.Errorf("%w: type nesting exceeds max depth of %d", ErrBadTypeDef, maxTypeDepth)
	}
	depth++
	params := t.Parameters()
	switch t.Kind() {
	case types.UnspecifiedKind:
		return &celpb.Type{}, nil
	case types.DynKind:
		return &celpb.Type{TypeKind: &celpb.Type_Dyn{}}, nil
	case types.NullTypeKind:
		return &celpb.Type{TypeKind: &celpb.Type_Null{}}, nil
	case types.BoolKind:
		return primitiveToProto(t, celpb.Type_BOOL), nil
	case types.IntKind:
		return primitiveToProto(t, celpb.Type_INT64), nil
	case types.UintKind:
		return primitiveToProto(t, celpb.Type_UINT64), nil
	case types.DoubleKind:
		return primitiveToProto(t, celpb.Type_DOUBLE), nil
	case types.StringKind:
		return primitiveToProto(t, celpb.Type_STRING), nil
	case types.BytesKind:
		return primitiveToProto(t, celpb.Type_BYTES), nil
	case types.AnyKind:
		return wellKnownToProto(celpb.Type_ANY), nil
	case types.TimestampKind:
		return wellKnownToProto(celpb.Type_TIMESTAMP), nil
	case types.DurationKind:
		return wellKnownToProto(celpb.Type_DURATION), nil
	case types.ListKind:
		if len(params) != 1 {
			return nil, fmt.Errorf("%w: list type has %d parameters, want 1", ErrBadTypeDef, len(params))
		}
		elem, err := typeToProtoDepth(params[0], depth)
		if err != nil {
			return nil, err
		}
		return &celpb.Type{TypeKind: &celpb.Type_ListType_{
			ListType: &celpb.Type_ListType{ElemType: elem},
		}}, nil
	case types.MapKind:
		if len(params) != 2 {
			return nil, fmt.Errorf("%w: map type has %d parameters, want 2", ErrBadTypeDef, len(params))
		}
		key, err := typeToProtoDepth(params[0], depth)
		if err != nil {
			return nil, err
		}
		val, err := typeToProtoDepth(params[1], depth)
		if err != nil {
			return nil, err
		}
		return &celpb.Type{TypeKind: &celpb.Type_MapType_{
			MapType: &celpb.Type_MapType{KeyType: key, ValueType: val},
		}}, nil
	case types.StructKind:
		return &celpb.Type{TypeKind: &celpb.Type_MessageType{MessageType: t.TypeName()}}, nil
	case types.TypeParamKind:
		return &celpb.Type{TypeKind: &celpb.Type_TypeParam{TypeParam: t.TypeName()}}, nil
	case types.TypeKind:
		if len(params) == 0 {
			return &celpb.Type{TypeKind: &celpb.Type_Type{Type: &celpb.Type{}}}, nil
		}
		if len(params) != 1 {
			return nil, fmt.Errorf("%w: type type has %d parameters, want at most 1", ErrBadTypeDef, len(params))
		}
		param, err := typeToProtoDepth(params[0], depth)
		if err != nil {
			return nil, err
		}
		return &celpb.Type{TypeKind: &celpb.Type_Type{Type: param}}, nil
	case types.ErrorKind:
		return &celpb.Type{TypeKind: &celpb.Type_Error{}}, nil
	case types.OpaqueKind:
		pbParams, err := typesToProto(params, depth)
		if err != nil {
			return nil, err
		}
		if t.TypeName() == functionTypeName {
			if len(pbParams) == 0 {
				return nil, fmt.Errorf("%w: function type is missing a result type", ErrBadTypeDef)
			}
			return &celpb.Type{TypeKind: &celpb.Type_Function{
				Function: &celpb.Type_FunctionType{ResultType: pbParams[0], ArgTypes: pbParams[1:]},
			}}, nil
		}
		return &celpb.Type{TypeKind: &celpb.Type_AbstractType_{
			AbstractType: &celpb.Type_AbstractType{Name: t.TypeName(), ParameterTypes: pbParams},
		}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported type kind for %v", ErrBadTypeDef, t)
	}
}

func typesToProto(ts []*types.Type, depth int) ([]*celpb.Type, error) {
	if len(ts) == 0 {
		return nil, nil
	}
	pbs := make([]*celpb.Type, 0, len(ts))
	for _, t := range ts {
		pb, err := typeToProtoDepth(t, depth)
		if err != nil {
			return nil, err
		}
		pbs = append(pbs, pb)
	}
	return pbs, nil
}

// primitiveToProto encodes t as a wrapper type when it is nullable (see
// types.NewNullableType), and as a primitive type otherwise.
func primitiveToProto(t *types.Type, primitive celpb.Type_PrimitiveType) *celpb.Type {
	if t.IsAssignableType(types.NullType) {
		return &celpb.Type{TypeKind: &celpb.Type_Wrapper{Wrapper: primitive}}
	}
	return &celpb.Type{TypeKind: &celpb.Type_Primitive{Primitive: primitive}}
}

func wellKnownToProto(wellKnown celpb.Type_WellKnownType) *celpb.Type {
	return &celpb.Type{TypeKind: &celpb.Type_WellKnown{WellKnown: wellKnown}}
}
