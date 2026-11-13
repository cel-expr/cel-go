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
	"math"
	"strconv"
	"strings"
	"time"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	celstrings "cel.dev/cel-go/internal/strings"
	"go.yaml.in/yaml/v3"
)

// TypeDefFromYAML parses a TypeDef from a YAML mapping.
//
// An empty or null YAML document returns an empty TypeDef with UnspecifiedKind.
func TypeDefFromYAML(data []byte) (*TypeDef, error) {
	root, err := loadYAML(data)
	if err != nil {
		return nil, err
	}
	if isYAMLNull(root) {
		return &TypeDef{}, nil
	}
	if !isYAMLMap(root) {
		return nil, yamlError(data, root, "invalid CEL type definition YAML")
	}
	return parseTypeDefNode(data, root)
}

// TypeDefSetFromYAML parses a TypeDefSet from YAML (either a sequence of type
// definitions or a mapping with a "types" sequence).
//
// An empty or null YAML document returns an empty TypeDefSet.
func TypeDefSetFromYAML(data []byte) (*TypeDefSet, error) {
	root, err := loadYAML(data)
	if err != nil {
		return nil, err
	}
	if isYAMLNull(root) {
		return &TypeDefSet{}, nil
	}
	if isYAMLSeq(root) {
		return parseTypeDefSetNode(data, root)
	}
	if isYAMLMap(root) {
		_, typesNode, ok := lookupMapValue(root, "types")
		if !ok {
			return nil, yamlError(data, root, "invalid CEL type definitions YAML")
		}
		return parseTypeDefSetNode(data, typesNode)
	}
	return nil, yamlError(data, root, "invalid CEL type definitions YAML")
}

func parseTypeDefSetNode(yamlBytes []byte, seqNode *yaml.Node) (*TypeDefSet, error) {
	result := &TypeDefSet{}
	if isYAMLNull(seqNode) {
		return result, nil
	}
	if !isYAMLSeq(seqNode) {
		return nil, yamlError(yamlBytes, seqNode, "node 'types' is not a sequence")
	}
	for _, rawItem := range seqNode.Content {
		item := resolveAlias(rawItem)
		td, err := parseTypeDefNode(yamlBytes, item)
		if err != nil {
			return nil, err
		}
		if err := result.AddTypeDef(td); err != nil {
			return nil, wrapYAMLError(yamlBytes, item, err)
		}
	}
	return result, nil
}

func parseTypeDefNode(yamlBytes []byte, node *yaml.Node) (*TypeDef, error) {
	if !isYAMLMap(node) {
		return nil, yamlError(yamlBytes, node, "type definition is not a map")
	}
	_, structNode, hasStruct := lookupMapValue(node, "struct")
	enumKeyNode, enumNode, hasEnum := lookupMapValue(node, "enum")
	if hasStruct && hasEnum {
		return nil, yamlError(yamlBytes, enumKeyNode, "node 'struct' and 'enum' are mutually exclusive")
	}
	if hasStruct {
		s, err := parseStructTypeDef(yamlBytes, structNode)
		if err != nil {
			return nil, err
		}
		return FromStruct(s), nil
	}
	if hasEnum {
		e, err := parseEnumTypeDef(yamlBytes, enumNode)
		if err != nil {
			return nil, err
		}
		return FromEnum(e), nil
	}
	return nil, yamlError(yamlBytes, node, "type definition must specify 'struct' or 'enum'")
}

func parseStructTypeDef(yamlBytes []byte, node *yaml.Node) (*StructTypeDef, error) {
	if !isYAMLMap(node) {
		return nil, yamlError(yamlBytes, node, "struct type definition is not a map")
	}
	nameNode, name, err := getRequiredYAMLString(
		yamlBytes, node, "name",
		"type definition 'name' is not a string",
		"type definition 'name' cannot be empty",
	)
	if err != nil {
		return nil, err
	}
	doc, err := getOptionalYAMLString(yamlBytes, node, "doc", "type definition 'doc' is not a string")
	if err != nil {
		return nil, err
	}
	if constantsKeyNode, _, ok := lookupMapValue(node, "constants"); ok {
		return nil, yamlError(yamlBytes, constantsKeyNode, "struct type definition cannot have 'constants'")
	}

	aspects, err := parseAspects(yamlBytes, node)
	if err != nil {
		return nil, err
	}

	var opts []StructOpt
	if doc != "" {
		opts = append(opts, StructDoc(doc))
	}
	if len(aspects) > 0 {
		opts = append(opts, StructAspects(aspects))
	}
	structDef, err := NewStruct(name, opts...)
	if err != nil {
		return nil, wrapYAMLError(yamlBytes, nameNode, err)
	}

	_, fieldsNode, hasFields := lookupMapValue(node, "fields")
	if !hasFields {
		return structDef, nil
	}
	if !isYAMLSeq(fieldsNode) {
		return nil, yamlError(yamlBytes, fieldsNode, "node 'fields' is not a sequence")
	}

	for _, rawFieldNode := range fieldsNode.Content {
		fieldNode := resolveAlias(rawFieldNode)
		if !isYAMLMap(fieldNode) {
			return nil, yamlError(yamlBytes, fieldNode, "field is not a map")
		}
		fieldNameNode, fieldName, err := getRequiredYAMLString(
			yamlBytes, fieldNode, "name",
			"field name is not a string",
			"field name cannot be empty",
		)
		if err != nil {
			return nil, err
		}

		_, typeNode, hasType := lookupMapValue(fieldNode, "type")
		if !hasType {
			return nil, yamlError(yamlBytes, fieldNode, "field 'type' is not specified")
		}
		var fieldType *types.Type
		switch {
		case isYAMLStringLike(typeNode):
			fieldType, err = parseTypeSignature(typeNode.Value)
			if err != nil {
				return nil, wrapYAMLError(yamlBytes, typeNode, err)
			}
		case isYAMLMap(typeNode):
			remaining := maxStructuredTypeNodes
			fieldType, err = parseStructuredType(yamlBytes, typeNode, 0, &remaining)
			if err != nil {
				return nil, err
			}
		default:
			return nil, yamlError(yamlBytes, typeNode, "field 'type' is neither a string nor a map")
		}

		fieldDoc, err := getOptionalYAMLString(yamlBytes, fieldNode, "doc", "field 'doc' is not a string")
		if err != nil {
			return nil, err
		}

		var defaultVal ref.Val
		if _, defaultNode, ok := lookupMapValue(fieldNode, "default"); ok {
			constKind := constantKindForType(fieldType)
			if constKind == types.UnspecifiedKind {
				typeName, err := formatTypeSignature(fieldType)
				if err != nil {
					typeName = fieldType.String()
				}
				return nil, yamlError(yamlBytes, defaultNode, "constant type '%s' is not supported", typeName)
			}
			if isYAMLNull(defaultNode) {
				// In yaml.v3, null values are ScalarNodes (with Tag "!!null"), whereas
				// yaml-cpp treats null as non-scalar (IsScalar() == false). Reject null
				// defaults on non-null types with the non-scalar error to match C++.
				if constKind != types.NullTypeKind {
					return nil, yamlError(yamlBytes, defaultNode, "field 'default' is not a scalar")
				}
				defaultVal = types.NullValue
			} else {
				if !isYAMLScalar(defaultNode) {
					return nil, yamlError(yamlBytes, defaultNode, "field 'default' is not a scalar")
				}
				// Decode !!binary before switching on constKind so base64 scalars work
				// as defaults for any scalar field type (e.g. string), matching C++.
				valStr := defaultNode.Value
				isBinary := isYAMLBinary(defaultNode)
				if isBinary {
					b, err := getYAMLBinary(yamlBytes, defaultNode)
					if err != nil {
						return nil, err
					}
					valStr = string(b)
				}
				defaultVal, err = parseConstantValue(yamlBytes, defaultNode, constKind, valStr, isBinary)
				if err != nil {
					return nil, err
				}
			}
		}

		fieldAspects, err := parseAspects(yamlBytes, fieldNode)
		if err != nil {
			return nil, err
		}

		var fieldOpts []FieldOpt
		if fieldDoc != "" {
			fieldOpts = append(fieldOpts, FieldDoc(fieldDoc))
		}
		if defaultVal != nil {
			fieldOpts = append(fieldOpts, FieldDefaultValue(defaultVal))
		}
		if len(fieldAspects) > 0 {
			fieldOpts = append(fieldOpts, FieldAspects(fieldAspects))
		}
		field, err := NewField(fieldName, fieldType, fieldOpts...)
		if err != nil {
			return nil, wrapYAMLError(yamlBytes, fieldNameNode, err)
		}
		if err := structDef.AddField(field); err != nil {
			return nil, wrapYAMLError(yamlBytes, fieldNameNode, err)
		}
	}

	return structDef, nil
}

func parseEnumTypeDef(yamlBytes []byte, node *yaml.Node) (*EnumTypeDef, error) {
	if !isYAMLMap(node) {
		return nil, yamlError(yamlBytes, node, "enum type definition is not a map")
	}
	nameNode, name, err := getRequiredYAMLString(
		yamlBytes, node, "name",
		"type definition 'name' is not a string",
		"type definition 'name' cannot be empty",
	)
	if err != nil {
		return nil, err
	}
	doc, err := getOptionalYAMLString(yamlBytes, node, "doc", "type definition 'doc' is not a string")
	if err != nil {
		return nil, err
	}
	if fieldsKeyNode, _, ok := lookupMapValue(node, "fields"); ok {
		return nil, yamlError(yamlBytes, fieldsKeyNode, "enum type definition cannot have 'fields'")
	}

	aspects, err := parseAspects(yamlBytes, node)
	if err != nil {
		return nil, err
	}

	var opts []EnumOpt
	if doc != "" {
		opts = append(opts, EnumDoc(doc))
	}
	if len(aspects) > 0 {
		opts = append(opts, EnumAspects(aspects))
	}
	enumDef, err := NewEnum(name, opts...)
	if err != nil {
		return nil, wrapYAMLError(yamlBytes, nameNode, err)
	}

	_, constantsNode, hasConstants := lookupMapValue(node, "constants")
	if !hasConstants {
		return enumDef, nil
	}
	if !isYAMLSeq(constantsNode) {
		return nil, yamlError(yamlBytes, constantsNode, "node 'constants' is not a sequence")
	}

	for i, rawConstantNode := range constantsNode.Content {
		constantNode := resolveAlias(rawConstantNode)
		if !isYAMLMap(constantNode) {
			return nil, yamlError(yamlBytes, constantNode, "enum constant is not a map")
		}
		constNameNode, constName, err := getRequiredYAMLString(
			yamlBytes, constantNode, "name",
			"enum constant name is not a string",
			"enum constant name cannot be empty",
		)
		if err != nil {
			return nil, err
		}

		id := int32(i)
		if _, idNode, ok := lookupMapValue(constantNode, "id"); ok {
			id, err = getYAMLInt32(yamlBytes, "id", idNode)
			if err != nil {
				return nil, err
			}
		}

		constDoc, err := getOptionalYAMLString(yamlBytes, constantNode, "doc", "enum constant 'doc' is not a string")
		if err != nil {
			return nil, err
		}

		constAspects, err := parseAspects(yamlBytes, constantNode)
		if err != nil {
			return nil, err
		}

		var constOpts []EnumConstantOpt
		if constDoc != "" {
			constOpts = append(constOpts, EnumConstantDoc(constDoc))
		}
		if len(constAspects) > 0 {
			constOpts = append(constOpts, EnumConstantAspects(constAspects))
		}
		ec, err := NewEnumConstant(constName, id, constOpts...)
		if err != nil {
			return nil, wrapYAMLError(yamlBytes, constNameNode, err)
		}
		if err := enumDef.AddConstant(ec); err != nil {
			return nil, wrapYAMLError(yamlBytes, constNameNode, err)
		}
	}

	return enumDef, nil
}

func parseAspects(yamlBytes []byte, parentNode *yaml.Node) (map[string]*Aspect, error) {
	_, aspectsNode, ok := lookupMapValue(parentNode, "aspects")
	if !ok {
		return nil, nil
	}
	if !isYAMLMap(aspectsNode) {
		return nil, yamlError(yamlBytes, aspectsNode, "node 'aspects' is not a map")
	}
	result := make(map[string]*Aspect, len(aspectsNode.Content)/2)
	for i := 0; i < len(aspectsNode.Content); i += 2 {
		kNode := resolveAlias(aspectsNode.Content[i])
		vNode := resolveAlias(aspectsNode.Content[i+1])
		if !isYAMLStringLike(kNode) {
			return nil, yamlError(yamlBytes, kNode, "target aspect name is not a string")
		}
		if kNode.Value == "" {
			return nil, yamlError(yamlBytes, kNode, "target aspect name cannot be empty")
		}
		if _, dup := result[kNode.Value]; dup {
			return nil, wrapYAMLError(yamlBytes, kNode, fmt.Errorf("%w: aspect %q", ErrAlreadyExists, kNode.Value))
		}
		if isYAMLNull(vNode) {
			result[kNode.Value] = &Aspect{}
			continue
		}
		if !isYAMLMap(vNode) {
			return nil, yamlError(yamlBytes, vNode, "aspect '%s' is not a map", kNode.Value)
		}
		props := make(map[string]ref.Val, len(vNode.Content)/2)
		for j := 0; j < len(vNode.Content); j += 2 {
			propKeyNode := resolveAlias(vNode.Content[j])
			propValNode := resolveAlias(vNode.Content[j+1])
			if !isYAMLStringLike(propKeyNode) {
				return nil, yamlError(yamlBytes, propKeyNode, "aspect property name is not a string")
			}
			if propKeyNode.Value == "" {
				return nil, yamlError(yamlBytes, propKeyNode, "aspect property name cannot be empty")
			}
			if _, dup := props[propKeyNode.Value]; dup {
				return nil, wrapYAMLError(yamlBytes, propKeyNode, fmt.Errorf("%w: aspect property %q", ErrAlreadyExists, propKeyNode.Value))
			}
			propVal, err := parseUntypedConstantValue(yamlBytes, propValNode)
			if err != nil {
				return nil, err
			}
			props[propKeyNode.Value] = propVal
		}
		aspect, err := NewAspect(props)
		if err != nil {
			return nil, wrapYAMLError(yamlBytes, vNode, err)
		}
		result[kNode.Value] = aspect
	}
	return result, nil
}

func constantKindForType(t *types.Type) types.Kind {
	if t == nil {
		return types.UnspecifiedKind
	}
	switch t.Kind() {
	case types.NullTypeKind:
		return types.NullTypeKind
	case types.BoolKind, types.IntKind, types.UintKind, types.DoubleKind, types.StringKind, types.BytesKind:
		if t.IsAssignableType(types.NullType) {
			// Wrapper types do not support default constant values.
			return types.UnspecifiedKind
		}
		return t.Kind()
	case types.DurationKind, types.TimestampKind:
		return t.Kind()
	case types.StructKind:
		return types.StringKind
	case types.OpaqueKind:
		if t.TypeName() != functionTypeName && len(t.Parameters()) == 0 {
			return types.StringKind
		}
		return types.UnspecifiedKind
	default:
		return types.UnspecifiedKind
	}
}

func parseUintConstant(val string) (uint64, error) {
	// absl::SimpleAtoi accepts an optional leading '+' on unsigned integers,
	// whereas strconv.ParseUint rejects it.
	return strconv.ParseUint(strings.TrimPrefix(val, "+"), 10, 64)
}

func parseConstantValue(yamlBytes []byte, node *yaml.Node, kind types.Kind, val string, isBinary bool) (ref.Val, error) {
	switch kind {
	case types.NullTypeKind:
		if val != "" {
			return nil, yamlError(yamlBytes, node, "failed to parse null constant")
		}
		return types.NullValue, nil
	case types.BoolKind:
		if strings.EqualFold(val, "true") {
			return types.True, nil
		}
		if strings.EqualFold(val, "false") {
			return types.False, nil
		}
		return nil, yamlError(yamlBytes, node, "failed to parse bool constant")
	case types.IntKind:
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, yamlError(yamlBytes, node, "failed to parse int constant")
		}
		return types.Int(v), nil
	case types.UintKind:
		trimmed := val
		if strings.HasSuffix(trimmed, "u") || strings.HasSuffix(trimmed, "U") {
			trimmed = trimmed[:len(trimmed)-1]
		}
		v, err := parseUintConstant(trimmed)
		if err != nil {
			return nil, yamlError(yamlBytes, node, "failed to parse uint constant")
		}
		return types.Uint(v), nil
	case types.DoubleKind:
		v, ok := parseYAMLDouble(val)
		if !ok {
			return nil, yamlError(yamlBytes, node, "failed to parse double constant")
		}
		return types.Double(v), nil
	case types.BytesKind:
		if !isBinary {
			if b, err := celstrings.ParseBytesLiteral(val); err == nil {
				return types.Bytes(b), nil
			}
		}
		return types.Bytes(val), nil
	case types.StringKind:
		return types.String(val), nil
	case types.DurationKind:
		d, err := time.ParseDuration(val)
		if err != nil {
			return nil, yamlError(yamlBytes, node, "failed to parse duration constant")
		}
		return types.Duration{Duration: d}, nil
	case types.TimestampKind:
		// time.RFC3339Nano accepts numeric timezone offsets (e.g. +01:00) in
		// addition to UTC 'Z', whereas C++ absl::ParseTime requires a trailing 'Z'.
		ts, err := time.Parse(time.RFC3339Nano, val)
		if err != nil {
			return nil, yamlError(yamlBytes, node, "failed to parse timestamp constant: %v supported format: YYYY-MM-DDThh:mm:ssZ", err)
		}
		return types.Timestamp{Time: ts}, nil
	default:
		return nil, yamlError(yamlBytes, node, "constant type is not supported")
	}
}

func parseUntypedConstantValue(yamlBytes []byte, node *yaml.Node) (ref.Val, error) {
	if isYAMLNull(node) {
		return types.NullValue, nil
	}
	if !isYAMLScalar(node) {
		return nil, yamlError(yamlBytes, node, "aspect property value is not a scalar")
	}
	if isYAMLBinary(node) {
		b, err := getYAMLBinary(yamlBytes, node)
		if err != nil {
			return nil, err
		}
		return types.Bytes(b), nil
	}

	val := node.Value
	// Only quoted scalars or scalars explicitly tagged as strings ("!" or
	// explicit "!!str") bypass type inference, matching C++. Scalars with
	// other explicit tags (e.g. !!int) still go through the inference cascade.
	quoted := node.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0
	if quoted || node.Tag == "!" || (node.Tag == "!!str" && node.Style&yaml.TaggedStyle != 0) {
		return types.String(val), nil
	}

	if strings.EqualFold(val, "true") {
		return types.True, nil
	}
	if strings.EqualFold(val, "false") {
		return types.False, nil
	}
	if len(val) >= 2 && (val[0] == 'b' || val[0] == 'B') && (val[1] == '"' || val[1] == '\'') {
		if b, err := celstrings.ParseBytesLiteral(val); err == nil {
			return types.Bytes(b), nil
		}
	}
	if len(val) > 1 && (strings.HasSuffix(val, "u") || strings.HasSuffix(val, "U")) {
		if u, err := parseUintConstant(val[:len(val)-1]); err == nil {
			return types.Uint(u), nil
		}
	}
	if i, err := strconv.ParseInt(val, 10, 64); err == nil {
		return types.Int(i), nil
	}
	if f, ok := parseYAMLDouble(val); ok {
		return types.Double(f), nil
	}
	return types.String(val), nil
}

func parseYAMLDouble(val string) (float64, bool) {
	// strconv.ParseFloat accepts "nan" and signed "inf", but rejects YAML's
	// leading-dot forms (.nan, .inf) and signed NaNs (+nan, -nan).
	switch strings.ToLower(val) {
	case "-nan", "-.nan":
		return math.Copysign(math.NaN(), -1), true
	case ".nan", "+.nan", "+nan":
		return math.NaN(), true
	case ".inf", "+.inf":
		return math.Inf(1), true
	case "-.inf":
		return math.Inf(-1), true
	}
	v, err := strconv.ParseFloat(val, 64)
	return v, err == nil
}
