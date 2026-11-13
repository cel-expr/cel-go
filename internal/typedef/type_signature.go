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
	"strings"

	"cel.dev/cel-go/common/types"
	"go.yaml.in/yaml/v3"
)

const (
	maxTypeSpecDepth       = 32
	maxStructuredTypeNodes = 10000
)

func parseBuiltinOrWrapper(name string) (*types.Type, bool) {
	switch name {
	case "null":
		return types.NullType, true
	case "bool":
		return types.BoolType, true
	case "int":
		return types.IntType, true
	case "uint":
		return types.UintType, true
	case "double":
		return types.DoubleType, true
	case "string":
		return types.StringType, true
	case "bytes":
		return types.BytesType, true
	case "any", "google.protobuf.Any":
		return types.AnyType, true
	case "timestamp", "google.protobuf.Timestamp":
		return types.TimestampType, true
	case "duration", "google.protobuf.Duration":
		return types.DurationType, true
	case "dyn", "google.protobuf.Value":
		return types.DynType, true
	case "bool_wrapper", "google.protobuf.BoolValue":
		return types.NewNullableType(types.BoolType), true
	case "int_wrapper", "google.protobuf.Int64Value", "google.protobuf.Int32Value":
		return types.NewNullableType(types.IntType), true
	case "uint_wrapper", "google.protobuf.UInt64Value", "google.protobuf.UInt32Value":
		return types.NewNullableType(types.UintType), true
	case "double_wrapper", "google.protobuf.DoubleValue", "google.protobuf.FloatValue":
		return types.NewNullableType(types.DoubleType), true
	case "string_wrapper", "google.protobuf.StringValue":
		return types.NewNullableType(types.StringType), true
	case "bytes_wrapper", "google.protobuf.BytesValue":
		return types.NewNullableType(types.BytesType), true
	case "google.protobuf.ListValue":
		return types.NewListType(types.DynType), true
	case "google.protobuf.Struct":
		return types.NewMapType(types.StringType, types.DynType), true
	default:
		return nil, false
	}
}

func stripUnescapedWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			b.WriteByte(c)
			escaped = true
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func unescapeSignature(s string) string {
	idx := strings.IndexByte(s, '\\')
	if idx < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:idx])
	escaped := false
	for i := idx; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
		} else if c == '\\' {
			escaped = true
		} else {
			b.WriteByte(c)
		}
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func escapeSignature(s string, escapeDot bool) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '(', ')', '<', '>', '"', ',', '~':
			b.WriteByte('\\')
		case '.':
			if escapeDot {
				b.WriteByte('\\')
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func findTopLevelChar(s string, target byte) (int, error) {
	nesting := 0
	escaped := false
	foundIdx := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == target && nesting == 0 && foundIdx == -1 {
			foundIdx = i
		}
		if c == '<' {
			nesting++
		} else if c == '>' {
			nesting--
			if nesting < 0 {
				return -1, fmt.Errorf("%w: invalid type signature: mismatched brackets", ErrBadTypeDef)
			}
		}
	}
	if nesting != 0 {
		return -1, fmt.Errorf("%w: invalid type signature: mismatched brackets", ErrBadTypeDef)
	}
	return foundIdx, nil
}

func splitTopLevelComma(s string) ([]string, error) {
	var result []string
	nesting := 0
	escaped := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == ',' && nesting == 0 {
			result = append(result, s[start:i])
			start = i + 1
		}
		if c == '<' {
			nesting++
		} else if c == '>' {
			nesting--
			if nesting < 0 {
				return nil, fmt.Errorf("%w: invalid type signature: mismatched brackets", ErrBadTypeDef)
			}
		}
	}
	if nesting != 0 {
		return nil, fmt.Errorf("%w: invalid type signature: mismatched brackets", ErrBadTypeDef)
	}
	result = append(result, s[start:])
	return result, nil
}

func parseTypeSignature(sig string) (*types.Type, error) {
	return parseTypeSignatureDepth(stripUnescapedWhitespace(sig), 0)
}

func parseTypeSignatureDepth(sig string, depth int) (*types.Type, error) {
	if depth > maxTypeSpecDepth {
		return nil, fmt.Errorf("%w: type specification exceeds maximum depth", ErrBadTypeDef)
	}
	if sig == "" {
		return nil, fmt.Errorf("%w: empty type signature", ErrBadTypeDef)
	}

	if sig[0] == '~' {
		paramName := sig[1:]
		if paramName == "" {
			return nil, fmt.Errorf("%w: invalid type signature: invalid type parameter name", ErrBadTypeDef)
		}
		lessIdx, err := findTopLevelChar(paramName, '<')
		if err != nil {
			return nil, err
		}
		commaIdx, err := findTopLevelChar(paramName, ',')
		if err != nil {
			return nil, err
		}
		if lessIdx != -1 || commaIdx != -1 {
			return nil, fmt.Errorf("%w: invalid type signature: invalid type parameter name", ErrBadTypeDef)
		}
		return types.NewTypeParamType(unescapeSignature(paramName)), nil
	}

	lessIdx, err := findTopLevelChar(sig, '<')
	if err != nil {
		return nil, err
	}

	var name string
	var params []*types.Type
	if lessIdx != -1 {
		if sig[len(sig)-1] != '>' {
			return nil, fmt.Errorf("%w: invalid type signature: missing closing >", ErrBadTypeDef)
		}
		name = unescapeSignature(sig[:lessIdx])
		paramsStr := sig[lessIdx+1 : len(sig)-1]
		paramList, err := splitTopLevelComma(paramsStr)
		if err != nil {
			return nil, err
		}
		params = make([]*types.Type, 0, len(paramList))
		for _, pStr := range paramList {
			p, err := parseTypeSignatureDepth(pStr, depth+1)
			if err != nil {
				return nil, err
			}
			params = append(params, p)
		}
	} else {
		name = unescapeSignature(sig)
	}

	if len(params) > 0 {
		if _, ok := parseBuiltinOrWrapper(name); ok {
			return nil, fmt.Errorf("%w: invalid type signature: %s cannot have type parameters", ErrBadTypeDef, name)
		}
	} else if builtin, ok := parseBuiltinOrWrapper(name); ok {
		return builtin, nil
	}

	if t, matched, arityErr := newParameterizedType(name, params); matched {
		if arityErr != "" {
			return nil, fmt.Errorf("%w: invalid type signature: %s %s", ErrBadTypeDef, name, arityErr)
		}
		return t, nil
	}

	if name == "" || strings.Contains(name, "..") {
		return nil, fmt.Errorf("%w: invalid type signature: invalid identifier", ErrBadTypeDef)
	}
	return types.NewOpaqueType(name, params...), nil
}

func newParameterizedType(name string, params []*types.Type) (*types.Type, bool, string) {
	paramOrDyn := func(idx int) *types.Type {
		if idx < len(params) {
			return params[idx]
		}
		return types.DynType
	}
	switch name {
	case "type":
		if len(params) > 1 {
			return nil, true, "expects at most 1 parameter"
		}
		return types.NewTypeTypeWithParam(paramOrDyn(0)), true, ""
	case "list":
		if len(params) > 1 {
			return nil, true, "expects at most 1 parameter"
		}
		return types.NewListType(paramOrDyn(0)), true, ""
	case "map":
		if len(params) != 0 && len(params) != 2 {
			return nil, true, "expects 0 or 2 parameters"
		}
		return types.NewMapType(paramOrDyn(0), paramOrDyn(1)), true, ""
	case "function":
		resultType := paramOrDyn(0)
		allParams := make([]*types.Type, 0, max(1, len(params)))
		allParams = append(allParams, resultType)
		if len(params) > 1 {
			allParams = append(allParams, params[1:]...)
		}
		return types.NewOpaqueType(functionTypeName, allParams...), true, ""
	default:
		return nil, false, ""
	}
}

func buildTypeFromStructured(yamlBytes []byte, node *yaml.Node, name string, isTypeParam bool, params []*types.Type) (*types.Type, error) {
	if isTypeParam {
		if len(params) > 0 {
			return nil, yamlError(yamlBytes, node, "type parameter cannot have type parameters")
		}
		return types.NewTypeParamType(name), nil
	}

	if t, matched, arityErr := newParameterizedType(name, params); matched {
		if arityErr != "" {
			return nil, yamlError(yamlBytes, node, "type '%s' %s", name, arityErr)
		}
		return t, nil
	}

	if !strings.HasPrefix(name, "~") && !strings.ContainsAny(name, "<>\\ \t\n\r") {
		if builtin, ok := parseBuiltinOrWrapper(name); ok {
			if len(params) > 0 {
				return nil, yamlError(yamlBytes, node, "type '%s' cannot have type parameters", name)
			}
			return builtin, nil
		}
	}

	return types.NewOpaqueType(name, params...), nil
}

func parseStructuredType(yamlBytes []byte, node *yaml.Node, depth int, remaining *int) (*types.Type, error) {
	if depth > maxTypeSpecDepth {
		return nil, yamlError(yamlBytes, node, "type specification exceeds maximum depth")
	}
	*remaining--
	if *remaining < 0 {
		return nil, yamlError(yamlBytes, node, "type specification exceeds maximum size")
	}
	_, typeNameNode, hasTypeName := lookupMapValue(node, "type_name")
	if !hasTypeName {
		return nil, yamlError(yamlBytes, node, "node 'type_name' is not specified")
	}
	if !isYAMLStringLike(typeNameNode) {
		return nil, yamlError(yamlBytes, typeNameNode, "node 'type_name' is not a string")
	}
	if typeNameNode.Value == "" {
		return nil, yamlError(yamlBytes, typeNameNode, "node 'type_name' cannot be empty")
	}

	isTypeParam := false
	if _, isTypeParamNode, ok := lookupMapValue(node, "is_type_param"); ok {
		var err error
		isTypeParam, err = getYAMLBool(yamlBytes, "is_type_param", isTypeParamNode)
		if err != nil {
			return nil, err
		}
	}

	var params []*types.Type
	if _, paramsNode, ok := lookupMapValue(node, "params"); ok {
		if !isYAMLSeq(paramsNode) {
			return nil, yamlError(yamlBytes, paramsNode, "node 'params' is not a sequence")
		}
		params = make([]*types.Type, 0, len(paramsNode.Content))
		for _, pNode := range paramsNode.Content {
			pNode = resolveAlias(pNode)
			if !isYAMLMap(pNode) {
				return nil, yamlError(yamlBytes, pNode, "type parameter is not a map")
			}
			p, err := parseStructuredType(yamlBytes, pNode, depth+1, remaining)
			if err != nil {
				return nil, err
			}
			params = append(params, p)
		}
	}

	return buildTypeFromStructured(yamlBytes, node, typeNameNode.Value, isTypeParam, params)
}

func formatTypeSignature(t *types.Type) (string, error) {
	var b strings.Builder
	if err := appendTypeSignature(&b, t, 0); err != nil {
		return "", err
	}
	return b.String(), nil
}

func appendTypeSignature(b *strings.Builder, t *types.Type, depth int) error {
	if depth > maxTypeSpecDepth {
		return fmt.Errorf("%w: type specification exceeds maximum depth", ErrBadTypeDef)
	}
	if t == nil {
		return fmt.Errorf("%w: unspecified type in signature", ErrBadTypeDef)
	}
	params := t.Parameters()
	switch t.Kind() {
	case types.NullTypeKind:
		b.WriteString("null")
	case types.DynKind:
		b.WriteString("dyn")
	case types.BoolKind:
		b.WriteString(primitiveOrWrapperName(t, "bool"))
	case types.IntKind:
		b.WriteString(primitiveOrWrapperName(t, "int"))
	case types.UintKind:
		b.WriteString(primitiveOrWrapperName(t, "uint"))
	case types.DoubleKind:
		b.WriteString(primitiveOrWrapperName(t, "double"))
	case types.StringKind:
		b.WriteString(primitiveOrWrapperName(t, "string"))
	case types.BytesKind:
		b.WriteString(primitiveOrWrapperName(t, "bytes"))
	case types.AnyKind:
		b.WriteString("any")
	case types.TimestampKind:
		b.WriteString("timestamp")
	case types.DurationKind:
		b.WriteString("duration")
	case types.ListKind:
		b.WriteString("list<")
		if len(params) > 0 && params[0] != nil && params[0].Kind() != types.UnspecifiedKind {
			if err := appendTypeSignature(b, params[0], depth+1); err != nil {
				return err
			}
		} else {
			b.WriteString("dyn")
		}
		b.WriteByte('>')
	case types.MapKind:
		b.WriteString("map<")
		if len(params) > 0 && params[0] != nil && params[0].Kind() != types.UnspecifiedKind {
			if err := appendTypeSignature(b, params[0], depth+1); err != nil {
				return err
			}
		} else {
			b.WriteString("dyn")
		}
		b.WriteByte(',')
		if len(params) > 1 && params[1] != nil && params[1].Kind() != types.UnspecifiedKind {
			if err := appendTypeSignature(b, params[1], depth+1); err != nil {
				return err
			}
		} else {
			b.WriteString("dyn")
		}
		b.WriteByte('>')
	case types.TypeKind:
		if len(params) == 0 || params[0] == nil || params[0].Kind() == types.UnspecifiedKind {
			return fmt.Errorf("%w: unspecified type parameter in signature", ErrBadTypeDef)
		}
		b.WriteString("type<")
		if err := appendTypeSignature(b, params[0], depth+1); err != nil {
			return err
		}
		b.WriteByte('>')
	case types.TypeParamKind:
		b.WriteByte('~')
		b.WriteString(escapeSignature(t.TypeName(), true))
	case types.StructKind:
		b.WriteString(escapeSignature(t.TypeName(), false))
	case types.OpaqueKind:
		if t.TypeName() == functionTypeName {
			if len(params) == 0 {
				return fmt.Errorf("%w: function type is missing a result type", ErrBadTypeDef)
			}
			b.WriteString("function<")
			if params[0] != nil && params[0].Kind() != types.UnspecifiedKind {
				if err := appendTypeSignature(b, params[0], depth+1); err != nil {
					return err
				}
			} else {
				b.WriteString("dyn")
			}
			for _, arg := range params[1:] {
				b.WriteByte(',')
				if err := appendTypeSignature(b, arg, depth+1); err != nil {
					return err
				}
			}
			b.WriteByte('>')
			return nil
		}
		b.WriteString(escapeSignature(t.TypeName(), false))
		if len(params) > 0 {
			b.WriteByte('<')
			for i, p := range params {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := appendTypeSignature(b, p, depth+1); err != nil {
					return err
				}
			}
			b.WriteByte('>')
		}
	default:
		return fmt.Errorf("%w: unsupported type in signature: %v", ErrBadTypeDef, t)
	}
	return nil
}

func emitTypeNode(t *types.Type, useTypeSignatures bool) (*yaml.Node, error) {
	if useTypeSignatures {
		if sig, err := formatTypeSignature(t); err == nil {
			return newYAMLQuotedStringNode(sig), nil
		}
	}
	return emitStructuredType(t, 0)
}

func emitStructuredType(t *types.Type, depth int) (*yaml.Node, error) {
	if depth > maxTypeSpecDepth {
		return nil, fmt.Errorf("%w: type specification exceeds maximum depth", ErrBadTypeDef)
	}
	m := &yaml.Node{Kind: yaml.MappingNode}

	if t == nil {
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("dyn"))
		return m, nil
	}

	params := t.Parameters()
	switch t.Kind() {
	case types.NullTypeKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("null"))
	case types.BoolKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(primitiveOrWrapperName(t, "bool")))
	case types.IntKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(primitiveOrWrapperName(t, "int")))
	case types.UintKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(primitiveOrWrapperName(t, "uint")))
	case types.DoubleKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(primitiveOrWrapperName(t, "double")))
	case types.StringKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(primitiveOrWrapperName(t, "string")))
	case types.BytesKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(primitiveOrWrapperName(t, "bytes")))
	case types.AnyKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("any"))
	case types.TimestampKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("timestamp"))
	case types.DurationKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("duration"))
	case types.ListKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("list"))
		if len(params) > 0 && params[0] != nil && params[0].Kind() != types.UnspecifiedKind {
			elemNode, err := emitStructuredType(params[0], depth+1)
			if err != nil {
				return nil, err
			}
			appendMapPair(m, "params", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{elemNode}})
		}
	case types.MapKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("map"))
		if len(params) >= 2 && ((params[0] != nil && params[0].Kind() != types.UnspecifiedKind) || (params[1] != nil && params[1].Kind() != types.UnspecifiedKind)) {
			kNode, err := emitStructuredType(params[0], depth+1)
			if err != nil {
				return nil, err
			}
			vNode, err := emitStructuredType(params[1], depth+1)
			if err != nil {
				return nil, err
			}
			appendMapPair(m, "params", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{kNode, vNode}})
		}
	case types.TypeKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("type"))
		if len(params) > 0 && params[0] != nil && params[0].Kind() != types.UnspecifiedKind {
			pNode, err := emitStructuredType(params[0], depth+1)
			if err != nil {
				return nil, err
			}
			appendMapPair(m, "params", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{pNode}})
		}
	case types.TypeParamKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(t.TypeName()))
		appendMapPair(m, "is_type_param", newYAMLPlainNode("!!bool", "true"))
	case types.StructKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(t.TypeName()))
	case types.OpaqueKind:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode(t.TypeName()))
		if len(params) > 0 {
			seq := &yaml.Node{Kind: yaml.SequenceNode, Content: make([]*yaml.Node, 0, len(params))}
			for _, p := range params {
				pNode, err := emitStructuredType(p, depth+1)
				if err != nil {
					return nil, err
				}
				seq.Content = append(seq.Content, pNode)
			}
			appendMapPair(m, "params", seq)
		}
	default:
		appendMapPair(m, "type_name", newYAMLQuotedStringNode("dyn"))
	}
	return m, nil
}

func primitiveOrWrapperName(t *types.Type, base string) string {
	if t.IsAssignableType(types.NullType) {
		return base + "_wrapper"
	}
	return base
}
