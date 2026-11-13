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
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// resolveAlias dereferences yaml.AliasNode pointers so YAML aliases work
// uniformly across mappings, sequences, and scalars. Unlike yaml-cpp, which
// resolves aliases transparently inside YAML::Node operations, yaml.v3 keeps
// explicit AliasNode wrappers in its AST.
func resolveAlias(node *yaml.Node) *yaml.Node {
	for i := 0; node != nil && node.Kind == yaml.AliasNode && i < 64; i++ {
		node = node.Alias
	}
	return node
}

func formatYAMLLineContext(yamlBytes []byte, line, col int) (string, bool) {
	if line <= 0 || col <= 0 {
		return "", false
	}
	lines := bytes.Split(yamlBytes, []byte("\n"))
	if line > len(lines) {
		return "", false
	}
	lineText := string(bytes.TrimSuffix(lines[line-1], []byte("\r")))
	if col-1 > len(lineText) {
		return "", false
	}
	return fmt.Sprintf("\n|%s\n|%s^", lineText, strings.Repeat(" ", col-1)), true
}

func yamlError(yamlBytes []byte, node *yaml.Node, format string, args ...any) error {
	return wrapYAMLError(yamlBytes, node, fmt.Errorf("%w: "+format, append([]any{ErrBadTypeDef}, args...)...))
}

func wrapYAMLError(yamlBytes []byte, node *yaml.Node, err error) error {
	if node == nil || node.Line <= 0 || node.Column <= 0 {
		return err
	}
	if ctx, ok := formatYAMLLineContext(yamlBytes, node.Line, node.Column); ok {
		return fmt.Errorf("%d:%d: %w%s", node.Line, node.Column, err, ctx)
	}
	return fmt.Errorf("%d:%d: %w", node.Line, node.Column, err)
}

func loadYAML(yamlBytes []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadTypeDef, err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, nil
	}
	return resolveAlias(doc.Content[0]), nil
}

func isYAMLNull(node *yaml.Node) bool {
	return node == nil || (node.Kind == yaml.ScalarNode && node.Tag == "!!null")
}

func isYAMLScalar(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode
}

// isYAMLStringLike matches yaml-cpp's Node::IsScalar() semantics for string,
// numeric, and boolean scalars: null nodes are excluded, while non-null scalars
// (e.g. unquoted numbers or booleans) can be read as text via Node.Value.
func isYAMLStringLike(node *yaml.Node) bool {
	return isYAMLScalar(node) && node.Tag != "!!null"
}

func isYAMLMap(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.MappingNode && len(node.Content)%2 == 0
}

func isYAMLSeq(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.SequenceNode
}

func isYAMLBinary(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode && node.Tag == "!!binary"
}

func getYAMLBinary(yamlBytes []byte, node *yaml.Node) ([]byte, error) {
	if !isYAMLBinary(node) {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(node.Value)
	if err != nil {
		return nil, yamlError(yamlBytes, node, "node '%s' is not a valid Base64 encoded binary", node.Value)
	}
	return decoded, nil
}

func getYAMLBool(yamlBytes []byte, key string, node *yaml.Node) (bool, error) {
	if isYAMLStringLike(node) {
		switch strings.ToLower(node.Value) {
		case "y", "yes", "true", "on":
			return true, nil
		case "n", "no", "false", "off":
			return false, nil
		}
	}
	return false, yamlError(yamlBytes, node, "node '%s' is not a boolean", key)
}

func getYAMLInt32(yamlBytes []byte, key string, node *yaml.Node) (int32, error) {
	if !isYAMLStringLike(node) {
		return 0, yamlError(yamlBytes, node, "node '%s' is not an integer", key)
	}
	val, err := strconv.ParseInt(node.Value, 10, 32)
	if err != nil {
		return 0, yamlError(yamlBytes, node, "node '%s' is not an integer", key)
	}
	return int32(val), nil
}

func lookupMapValue(mapNode *yaml.Node, key string) (*yaml.Node, *yaml.Node, bool) {
	if !isYAMLMap(mapNode) {
		return nil, nil, false
	}
	for i := 0; i < len(mapNode.Content); i += 2 {
		k := resolveAlias(mapNode.Content[i])
		v := resolveAlias(mapNode.Content[i+1])
		if k != nil && k.Kind == yaml.ScalarNode && k.Value == key {
			return k, v, true
		}
	}
	return nil, nil, false
}

func getRequiredYAMLString(yamlBytes []byte, parent *yaml.Node, key, notStringMsg, emptyMsg string) (*yaml.Node, string, error) {
	_, valNode, ok := lookupMapValue(parent, key)
	if !ok || !isYAMLStringLike(valNode) {
		errNode := parent
		if ok {
			errNode = valNode
		}
		return nil, "", yamlError(yamlBytes, errNode, "%s", notStringMsg)
	}
	if valNode.Value == "" {
		return nil, "", yamlError(yamlBytes, valNode, "%s", emptyMsg)
	}
	return valNode, valNode.Value, nil
}

func getOptionalYAMLString(yamlBytes []byte, parent *yaml.Node, key, notStringMsg string) (string, error) {
	_, valNode, ok := lookupMapValue(parent, key)
	if !ok {
		return "", nil
	}
	if !isYAMLStringLike(valNode) {
		return "", yamlError(yamlBytes, valNode, "%s", notStringMsg)
	}
	return valNode.Value, nil
}

func newYAMLQuotedStringNode(val string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Style: yaml.DoubleQuotedStyle,
		Value: val,
	}
}

func newYAMLPlainNode(tag, val string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   tag,
		Value: val,
	}
}

func appendMapPair(m *yaml.Node, key string, val *yaml.Node) {
	m.Content = append(m.Content, newYAMLPlainNode("!!str", key), val)
}
