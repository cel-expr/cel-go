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
	"bytes"
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	celstrings "cel.dev/cel-go/internal/strings"
	"go.yaml.in/yaml/v3"
)

// ToYAMLOption configures how TypeDef and TypeDefSet values are serialized to YAML.
type ToYAMLOption func(*toYAMLOptions)

type toYAMLOptions struct {
	useTypeSignatures bool
}

func defaultToYAMLOptions() toYAMLOptions {
	return toYAMLOptions{
		useTypeSignatures: true,
	}
}

// UseTypeSignatures configures whether to emit concise type signatures (e.g.,
// `type: "list<string>"`) instead of structured type definitions
// (`type_name: "list"`, `params: ...`) in the output YAML. Enabled by default.
func UseTypeSignatures(enabled bool) ToYAMLOption {
	return func(o *toYAMLOptions) {
		o.useTypeSignatures = enabled
	}
}

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

// TypeDefToYAML serializes a TypeDef as a YAML mapping.
func TypeDefToYAML(t *TypeDef, opts ...ToYAMLOption) ([]byte, error) {
	if t == nil || t.Kind() == UnspecifiedKind {
		return nil, fmt.Errorf("%w: cannot serialize unspecified TypeDef", ErrBadTypeDef)
	}
	cfg := defaultToYAMLOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	node, err := emitTypeDefNode(t, cfg)
	if err != nil {
		return nil, err
	}
	return encodeYAMLNode(node)
}

// TypeDefSetToYAML serializes a TypeDefSet as a YAML sequence sorted
// deterministically by type name. Returns nil if s is nil or empty.
func TypeDefSetToYAML(s *TypeDefSet, opts ...ToYAMLOption) ([]byte, error) {
	typeDefs := s.Types()
	if len(typeDefs) == 0 {
		return nil, nil
	}
	cfg := defaultToYAMLOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	slices.SortFunc(typeDefs, func(a, b *TypeDef) int {
		return cmp.Compare(a.Name(), b.Name())
	})
	seq := &yaml.Node{Kind: yaml.SequenceNode, Content: make([]*yaml.Node, 0, len(typeDefs))}
	for _, td := range typeDefs {
		node, err := emitTypeDefNode(td, cfg)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, node)
	}
	return encodeYAMLNode(seq)
}

func encodeYAMLNode(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// formatBytesLiteral formats b as a CEL-style hex-escaped bytes literal, e.g. `b"\x00\xff"`.
func formatBytesLiteral(b []byte) string {
	var sb strings.Builder
	sb.Grow(3 + len(b)*4)
	sb.WriteString(`b"`)
	for _, c := range b {
		fmt.Fprintf(&sb, `\x%02x`, c)
	}
	sb.WriteByte('"')
	return sb.String()
}

// formatDuration formats d using absl::FormatDuration rules so emitted YAML is
// compatible with C++: ASCII "us" is used instead of Go's "µs" (which
// absl::ParseDuration rejects), zero components are omitted, and 0 formats as "0".
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	var sb strings.Builder
	var u uint64
	if d < 0 {
		sb.WriteByte('-')
		u = uint64(-d)
	} else {
		u = uint64(d)
	}
	if u < uint64(time.Second) {
		switch {
		case u < uint64(time.Microsecond):
			fmt.Fprintf(&sb, "%dns", u)
		case u < uint64(time.Millisecond):
			appendScaledDuration(&sb, u, uint64(time.Microsecond), 3, "us")
		default:
			appendScaledDuration(&sb, u, uint64(time.Millisecond), 6, "ms")
		}
		return sb.String()
	}
	if h := u / uint64(time.Hour); h > 0 {
		fmt.Fprintf(&sb, "%dh", h)
		u %= uint64(time.Hour)
	}
	if m := u / uint64(time.Minute); m > 0 {
		fmt.Fprintf(&sb, "%dm", m)
		u %= uint64(time.Minute)
	}
	if u > 0 {
		appendScaledDuration(&sb, u, uint64(time.Second), 9, "s")
	}
	return sb.String()
}

func appendScaledDuration(sb *strings.Builder, u, unit uint64, fracDigits int, suffix string) {
	whole := u / unit
	rem := u % unit
	fmt.Fprintf(sb, "%d", whole)
	if rem > 0 {
		frac := fmt.Sprintf("%0*d", fracDigits, rem)
		frac = strings.TrimRight(frac, "0")
		sb.WriteByte('.')
		sb.WriteString(frac)
	}
	sb.WriteString(suffix)
}

func emitTypeDefNode(td *TypeDef, cfg toYAMLOptions) (*yaml.Node, error) {
	outer := &yaml.Node{Kind: yaml.MappingNode}
	if td.IsStruct() {
		s := td.StructType()
		m := &yaml.Node{Kind: yaml.MappingNode}
		if s.Name() != "" {
			appendMapPair(m, "name", newYAMLQuotedStringNode(s.Name()))
		}
		if s.Doc() != "" {
			appendMapPair(m, "doc", newYAMLQuotedStringNode(s.Doc()))
		}
		aspectsNode, err := emitAspectsNode(s.Aspects())
		if err != nil {
			return nil, err
		}
		if aspectsNode != nil {
			appendMapPair(m, "aspects", aspectsNode)
		}
		if fields := s.Fields(); len(fields) > 0 {
			seq := &yaml.Node{Kind: yaml.SequenceNode, Content: make([]*yaml.Node, 0, len(fields))}
			for _, f := range fields {
				fm, err := emitFieldNode(f, cfg.useTypeSignatures)
				if err != nil {
					return nil, err
				}
				seq.Content = append(seq.Content, fm)
			}
			appendMapPair(m, "fields", seq)
		}
		appendMapPair(outer, "struct", m)
		return outer, nil
	}

	e := td.EnumType()
	m := &yaml.Node{Kind: yaml.MappingNode}
	if e.Name() != "" {
		appendMapPair(m, "name", newYAMLQuotedStringNode(e.Name()))
	}
	if e.Doc() != "" {
		appendMapPair(m, "doc", newYAMLQuotedStringNode(e.Doc()))
	}
	aspectsNode, err := emitAspectsNode(e.Aspects())
	if err != nil {
		return nil, err
	}
	if aspectsNode != nil {
		appendMapPair(m, "aspects", aspectsNode)
	}
	if constants := e.Constants(); len(constants) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Content: make([]*yaml.Node, 0, len(constants))}
		for _, c := range constants {
			cm, err := emitEnumConstantNode(c)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, cm)
		}
		appendMapPair(m, "constants", seq)
	}
	appendMapPair(outer, "enum", m)
	return outer, nil
}

func emitFieldNode(f *Field, useTypeSignatures bool) (*yaml.Node, error) {
	fm := &yaml.Node{Kind: yaml.MappingNode}
	appendMapPair(fm, "name", newYAMLQuotedStringNode(f.Name()))
	typeNode, err := emitTypeNode(f.Type(), useTypeSignatures)
	if err != nil {
		return nil, err
	}
	appendMapPair(fm, "type", typeNode)
	if f.Doc() != "" {
		appendMapPair(fm, "doc", newYAMLQuotedStringNode(f.Doc()))
	}
	if f.HasDefaultValue() {
		defNode, err := emitConstantNode(f.DefaultValue(), false)
		if err != nil {
			return nil, err
		}
		if defNode != nil {
			appendMapPair(fm, "default", defNode)
		}
	}
	fAspects, err := emitAspectsNode(f.Aspects())
	if err != nil {
		return nil, err
	}
	if fAspects != nil {
		appendMapPair(fm, "aspects", fAspects)
	}
	return fm, nil
}

func emitEnumConstantNode(c *EnumConstant) (*yaml.Node, error) {
	cm := &yaml.Node{Kind: yaml.MappingNode}
	appendMapPair(cm, "name", newYAMLQuotedStringNode(c.Name()))
	appendMapPair(cm, "id", newYAMLPlainNode("!!int", strconv.FormatInt(int64(c.ID()), 10)))
	if c.Doc() != "" {
		appendMapPair(cm, "doc", newYAMLQuotedStringNode(c.Doc()))
	}
	cAspects, err := emitAspectsNode(c.Aspects())
	if err != nil {
		return nil, err
	}
	if cAspects != nil {
		appendMapPair(cm, "aspects", cAspects)
	}
	return cm, nil
}

func emitAspectsNode(aspects map[string]*Aspect) (*yaml.Node, error) {
	if len(aspects) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(aspects))
	for k := range aspects {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		aspect := aspects[k]
		am := &yaml.Node{Kind: yaml.MappingNode}
		props := aspect.Properties()
		if len(props) > 0 {
			propKeys := make([]string, 0, len(props))
			for pk, pv := range props {
				if pv != nil {
					propKeys = append(propKeys, pk)
				}
			}
			slices.Sort(propKeys)
			for _, pk := range propKeys {
				valNode, err := emitConstantNode(props[pk], true)
				if err != nil {
					return nil, err
				}
				if valNode != nil {
					appendMapPair(am, pk, valNode)
				}
			}
		}
		appendMapPair(m, k, am)
	}
	return m, nil
}

func emitConstantNode(val ref.Val, untyped bool) (*yaml.Node, error) {
	switch v := val.(type) {
	case nil:
		return nil, nil
	case types.Null:
		return newYAMLPlainNode("!!null", "~"), nil
	case types.Bool:
		if bool(v) {
			return newYAMLPlainNode("!!bool", "true"), nil
		}
		return newYAMLPlainNode("!!bool", "false"), nil
	case types.Int:
		return newYAMLPlainNode("!!int", strconv.FormatInt(int64(v), 10)), nil
	case types.Uint:
		s := strconv.FormatUint(uint64(v), 10)
		if untyped {
			return newYAMLPlainNode("!!str", s+"u"), nil
		}
		return newYAMLPlainNode("!!int", s), nil
	case types.Double:
		f := float64(v)
		switch {
		case math.IsNaN(f):
			return newYAMLPlainNode("!!float", ".nan"), nil
		case math.IsInf(f, 1):
			return newYAMLPlainNode("!!float", ".inf"), nil
		case math.IsInf(f, -1):
			return newYAMLPlainNode("!!float", "-.inf"), nil
		default:
			s := strconv.FormatFloat(f, 'g', -1, 64)
			if untyped && !strings.ContainsAny(s, ".eE") {
				s += ".0"
			}
			return newYAMLPlainNode("!!float", s), nil
		}
	case types.Bytes:
		return newYAMLPlainNode("!!str", formatBytesLiteral([]byte(v))), nil
	case types.String:
		return newYAMLQuotedStringNode(string(v)), nil
	case types.Duration:
		s := formatDuration(v.Duration)
		if untyped {
			return newYAMLQuotedStringNode(s), nil
		}
		// Leave the tag empty so yaml.v3 emits durations (including "0") as plain
		// scalars matching yaml-cpp instead of quoting "0" under !!str.
		return newYAMLPlainNode("", s), nil
	case types.Timestamp:
		s := v.Time.UTC().Format(time.RFC3339Nano)
		if untyped {
			return newYAMLQuotedStringNode(s), nil
		}
		// Leave the tag empty so yaml.v3 emits RFC3339 timestamps as plain scalars
		// rather than quoting them as !!str, matching yaml-cpp's output.
		return newYAMLPlainNode("", s), nil
	default:
		return nil, fmt.Errorf("%w: unsupported constant value %T", ErrBadTypeDef, val)
	}
}
