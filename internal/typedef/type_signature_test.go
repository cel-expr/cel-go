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
	"errors"
	"strings"
	"testing"

	"cel.dev/cel-go/common/types"
	"go.yaml.in/yaml/v3"
)

func TestTypeSignatureRoundTrip(t *testing.T) {
	tests := []struct {
		sig       string
		want      *types.Type
		canonical string
	}{
		{sig: "null", want: types.NullType, canonical: "null"},
		{sig: "int", want: types.IntType, canonical: "int"},
		{sig: "dyn", want: types.DynType, canonical: "dyn"},
		{sig: "google.protobuf.Value", want: types.DynType, canonical: "dyn"},
		{sig: "google.protobuf.Timestamp", want: types.TimestampType, canonical: "timestamp"},
		{sig: "int_wrapper", want: types.NewNullableType(types.IntType), canonical: "int_wrapper"},
		{sig: "google.protobuf.Int32Value", want: types.NewNullableType(types.IntType), canonical: "int_wrapper"},
		{sig: "google.protobuf.ListValue", want: types.NewListType(types.DynType), canonical: "list<dyn>"},
		{sig: "google.protobuf.Struct", want: types.NewMapType(types.StringType, types.DynType), canonical: "map<string,dyn>"},
		{sig: "list", want: types.NewListType(types.DynType), canonical: "list<dyn>"},
		{sig: "list<string>", want: types.NewListType(types.StringType), canonical: "list<string>"},
		{sig: "map", want: types.NewMapType(types.DynType, types.DynType), canonical: "map<dyn,dyn>"},
		{sig: " map < string ,\n\tint > ", want: types.NewMapType(types.StringType, types.IntType), canonical: "map<string,int>"},
		{sig: "type<int>", want: types.NewTypeTypeWithParam(types.IntType), canonical: "type<int>"},
		{sig: "~T", want: types.NewTypeParamType("T"), canonical: "~T"},
		{sig: "list<~T>", want: types.NewListType(types.NewTypeParamType("T")), canonical: "list<~T>"},
		{
			sig:       "function<int,string,bool>",
			want:      types.NewOpaqueType(functionTypeName, types.IntType, types.StringType, types.BoolType),
			canonical: "function<int,string,bool>",
		},
		{sig: "function", want: types.NewOpaqueType(functionTypeName, types.DynType), canonical: "function<dyn>"},
		{sig: "com.example.Foo", want: types.NewOpaqueType("com.example.Foo"), canonical: "com.example.Foo"},
		{sig: "vector<int>", want: types.NewOpaqueType("vector", types.IntType), canonical: "vector<int>"},
		{sig: `list<a\<b\>c>`, want: types.NewListType(types.NewOpaqueType("a<b>c")), canonical: `list<a\<b\>c>`},
		{sig: `~T\.U`, want: types.NewTypeParamType("T.U"), canonical: `~T\.U`},
		{sig: `a\ b`, want: types.NewOpaqueType("a b"), canonical: "a b"},
	}
	for _, tc := range tests {
		t.Run(tc.sig, func(t *testing.T) {
			got, err := parseTypeSignature(tc.sig)
			if err != nil {
				t.Fatalf("parseTypeSignature(%q) failed: %v", tc.sig, err)
			}
			if !got.IsExactType(tc.want) {
				t.Errorf("parseTypeSignature(%q) = %v, want %v", tc.sig, got, tc.want)
			}
			sig, err := formatTypeSignature(got)
			if err != nil {
				t.Fatalf("formatTypeSignature(%v) failed: %v", got, err)
			}
			if sig != tc.canonical {
				t.Errorf("formatTypeSignature(%v) = %q, want %q", got, sig, tc.canonical)
			}
		})
	}
}

func TestParseTypeSignatureErrors(t *testing.T) {
	tests := []struct {
		sig     string
		wantSub string
	}{
		{sig: "", wantSub: "empty type signature"},
		{sig: "<int>", wantSub: "invalid identifier"},
		{sig: "a..b", wantSub: "invalid identifier"},
		{sig: "~", wantSub: "invalid type parameter name"},
		{sig: "~a<b", wantSub: "mismatched brackets"},
		{sig: `~a\<b<c>`, wantSub: "invalid type parameter name"},
		{sig: "~a,b", wantSub: "invalid type parameter name"},
		{sig: "list<int", wantSub: "mismatched brackets"},
		{sig: "list<int>>", wantSub: "mismatched brackets"},
		{sig: "list<int>extra", wantSub: "missing closing >"},
		{sig: "list<int,string>", wantSub: "list expects at most 1 parameter"},
		{sig: "map<string>", wantSub: "map expects 0 or 2 parameters"},
		{sig: "type<int,string>", wantSub: "type expects at most 1 parameter"},
		{sig: "int<string>", wantSub: "int cannot have type parameters"},
		{sig: "list<>", wantSub: "empty type signature"},
		{
			sig:     strings.Repeat("list<", maxTypeSpecDepth+2) + "int" + strings.Repeat(">", maxTypeSpecDepth+2),
			wantSub: "type specification exceeds maximum depth",
		},
	}
	for _, tc := range tests {
		t.Run(tc.sig, func(t *testing.T) {
			_, err := parseTypeSignature(tc.sig)
			if !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("parseTypeSignature(%q) = %v, want ErrBadTypeDef containing %q", tc.sig, err, tc.wantSub)
			}
		})
	}
}

func TestFormatTypeSignatureErrors(t *testing.T) {
	deep := types.IntType
	for i := 0; i < maxTypeSpecDepth+2; i++ {
		deep = types.NewListType(deep)
	}
	tests := []struct {
		name    string
		typ     *types.Type
		wantSub string
	}{
		{name: "nil", typ: nil, wantSub: "unspecified type in signature"},
		{name: "unspecified", typ: &types.Type{}, wantSub: "unsupported type in signature"},
		{name: "type_without_param", typ: types.TypeType, wantSub: "unspecified type parameter in signature"},
		{name: "function_without_result", typ: types.NewOpaqueType(functionTypeName), wantSub: "function type is missing a result type"},
		{name: "too_deep", typ: deep, wantSub: "type specification exceeds maximum depth"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := formatTypeSignature(tc.typ)
			if !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("formatTypeSignature(%v) = %v, want ErrBadTypeDef containing %q", tc.typ, err, tc.wantSub)
			}
		})
	}
}

func parseStructuredTypeYAML(t *testing.T, data string, budget int) (*types.Type, error) {
	t.Helper()
	root, err := loadYAML([]byte(data))
	if err != nil {
		t.Fatalf("loadYAML(%q) failed: %v", data, err)
	}
	return parseStructuredType([]byte(data), root, 0, &budget)
}

func TestParseStructuredType(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want *types.Type
	}{
		{name: "primitive", yaml: "type_name: int", want: types.IntType},
		{name: "wrapper", yaml: "type_name: google.protobuf.Int64Value", want: types.NewNullableType(types.IntType)},
		{name: "list_default", yaml: "type_name: list", want: types.NewListType(types.DynType)},
		{
			name: "list",
			yaml: "type_name: list\nparams:\n  - type_name: string\n",
			want: types.NewListType(types.StringType),
		},
		{
			name: "map",
			yaml: "type_name: map\nparams:\n  - type_name: string\n  - type_name: T\n    is_type_param: true\n",
			want: types.NewMapType(types.StringType, types.NewTypeParamType("T")),
		},
		{name: "type_param", yaml: "type_name: T\nis_type_param: yes", want: types.NewTypeParamType("T")},
		{name: "opaque", yaml: "type_name: com.example.Foo", want: types.NewOpaqueType("com.example.Foo")},
		{
			// Structured type names are taken literally and never parsed as signatures.
			name: "literal_name",
			yaml: `type_name: "list<int>"`,
			want: types.NewOpaqueType("list<int>"),
		},
		{
			name: "alias",
			yaml: "type_name: function\nparams:\n  - &p {type_name: int}\n  - *p\n",
			want: types.NewOpaqueType(functionTypeName, types.IntType, types.IntType),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseStructuredTypeYAML(t, tc.yaml, maxStructuredTypeNodes)
			if err != nil {
				t.Fatalf("parseStructuredType() failed: %v", err)
			}
			if !got.IsExactType(tc.want) {
				t.Errorf("parseStructuredType() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseStructuredTypeErrors(t *testing.T) {
	var deep strings.Builder
	for i := 0; i < maxTypeSpecDepth+2; i++ {
		indent := strings.Repeat("  ", i)
		deep.WriteString(indent + "type_name: list\n" + indent + "params:\n" + indent + "- \n")
	}
	deep.WriteString(strings.Repeat("  ", maxTypeSpecDepth+2) + "type_name: int\n")

	tests := []struct {
		name    string
		yaml    string
		budget  int
		wantSub string
	}{
		{name: "missing_type_name", yaml: "params: []", wantSub: "node 'type_name' is not specified"},
		{name: "type_name_not_string", yaml: "type_name: [a]", wantSub: "node 'type_name' is not a string"},
		{name: "empty_type_name", yaml: `type_name: ""`, wantSub: "node 'type_name' cannot be empty"},
		{name: "bad_is_type_param", yaml: "type_name: T\nis_type_param: maybe", wantSub: "node 'is_type_param' is not a boolean"},
		{name: "params_not_seq", yaml: "type_name: list\nparams: int", wantSub: "node 'params' is not a sequence"},
		{name: "param_not_map", yaml: "type_name: list\nparams: [int]", wantSub: "type parameter is not a map"},
		{
			name:    "type_param_with_params",
			yaml:    "type_name: T\nis_type_param: true\nparams: [{type_name: int}]",
			wantSub: "type parameter cannot have type parameters",
		},
		{name: "builtin_with_params", yaml: "type_name: int\nparams: [{type_name: int}]", wantSub: "type 'int' cannot have type parameters"},
		{name: "map_arity", yaml: "type_name: map\nparams: [{type_name: int}]", wantSub: "type 'map' expects 0 or 2 parameters"},
		{name: "too_deep", yaml: deep.String(), wantSub: "type specification exceeds maximum depth"},
		{name: "too_large", yaml: "type_name: list\nparams: [{type_name: int}]", budget: 1, wantSub: "type specification exceeds maximum size"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			budget := tc.budget
			if budget == 0 {
				budget = maxStructuredTypeNodes
			}
			_, err := parseStructuredTypeYAML(t, tc.yaml, budget)
			if !errors.Is(err, ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("parseStructuredType(%q) = %v, want ErrBadTypeDef containing %q", tc.yaml, err, tc.wantSub)
			}
		})
	}
}

func TestEmitTypeNode(t *testing.T) {
	tests := []struct {
		name          string
		typ           *types.Type
		useSignatures bool
		want          string
	}{
		{name: "signature", typ: types.NewListType(types.IntType), useSignatures: true, want: "\"list<int>\"\n"},
		{
			name: "structured",
			typ:  types.NewMapType(types.StringType, types.NewTypeParamType("T")),
			want: "type_name: \"map\"\nparams:\n  - type_name: \"string\"\n  - type_name: \"T\"\n    is_type_param: true\n",
		},
		{
			// type without a parameter has no signature form, so emission falls back
			// to the structured representation even when signatures are requested.
			name:          "signature_fallback",
			typ:           types.TypeType,
			useSignatures: true,
			want:          "type_name: \"type\"\n",
		},
		{name: "nil", typ: nil, want: "type_name: \"dyn\"\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, err := emitTypeNode(tc.typ, tc.useSignatures)
			if err != nil {
				t.Fatalf("emitTypeNode() failed: %v", err)
			}
			if got := encodeTestNode(t, node); got != tc.want {
				t.Errorf("emitTypeNode() got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestStructuredTypeRoundTrip(t *testing.T) {
	for _, typ := range []*types.Type{
		types.NullType,
		types.BytesType,
		types.DurationType,
		types.NewNullableType(types.DoubleType),
		types.NewListType(types.NewListType(types.StringType)),
		types.NewMapType(types.IntType, types.NewNullableType(types.BoolType)),
		types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
		types.NewOpaqueType(functionTypeName, types.IntType, types.StringType),
		types.NewOpaqueType("com.example.Vec", types.UintType),
		types.NewObjectType("com.example.Msg"),
	} {
		t.Run(typ.String(), func(t *testing.T) {
			node, err := emitStructuredType(typ, 0)
			if err != nil {
				t.Fatalf("emitStructuredType(%v) failed: %v", typ, err)
			}
			got, err := parseStructuredTypeYAML(t, encodeTestNode(t, node), maxStructuredTypeNodes)
			if err != nil {
				t.Fatalf("parseStructuredType() failed: %v", err)
			}
			want := typ
			if typ.Kind() == types.StructKind {
				// Struct types are decoded as opaque named types.
				want = types.NewOpaqueType(typ.TypeName())
			}
			if !got.IsExactType(want) {
				t.Errorf("structured round-trip = %v, want %v", got, want)
			}
		})
	}
}

func TestYAMLErrorContext(t *testing.T) {
	data := []byte("a: 1\nb:\n  c: x\n")
	root, err := loadYAML(data)
	if err != nil {
		t.Fatalf("loadYAML() failed: %v", err)
	}
	_, b, _ := lookupMapValue(root, "b")
	_, c, _ := lookupMapValue(b, "c")
	err = yamlError(data, c, "bad %s", "value")
	if !errors.Is(err, ErrBadTypeDef) {
		t.Errorf("yamlError() = %v, want ErrBadTypeDef", err)
	}
	want := "3:6: invalid typedef: bad value\n|  c: x\n|     ^"
	if err.Error() != want {
		t.Errorf("yamlError() = %q, want %q", err.Error(), want)
	}
	if err := yamlError(data, nil, "no position"); err.Error() != "invalid typedef: no position" {
		t.Errorf("yamlError(nil node) = %q, want no position context", err.Error())
	}
}

func encodeTestNode(t *testing.T, node *yaml.Node) string {
	t.Helper()
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		t.Fatalf("yaml.Encode() failed: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("yaml.Encoder.Close() failed: %v", err)
	}
	return b.String()
}
