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

package typedef_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/internal/typedef"
	"cel.dev/cel-go/internal/typedef/typedefpb"
	celpb "cel.dev/expr"
	"go.yaml.in/yaml/v3"
)

func unindent(s string) string {
	lines := strings.Split(s, "\n")
	indent := -1
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		pos := len(line) - len(trimmed)
		if indent == -1 {
			indent = pos
		}
		if pos >= indent {
			out = append(out, line[indent:])
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func TestCelTypesFromYAML(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/cel_types.yaml")
	if err != nil {
		t.Fatalf("os.ReadFile(testdata/cel_types.yaml) failed: %v", err)
	}
	baselineBytes, err := os.ReadFile("testdata/cel_types_from_yaml.baseline")
	if err != nil {
		t.Fatalf("os.ReadFile(testdata/cel_types_from_yaml.baseline) failed: %v", err)
	}

	const marker = "TYPEDEF PROTO:\n"
	_, protoText, ok := strings.Cut(string(baselineBytes), marker)
	if !ok {
		t.Fatalf("baseline missing %q marker", marker)
	}
	wantProto := &typedefpb.TypeDefSet{}
	if err := prototext.Unmarshal([]byte(protoText), wantProto); err != nil {
		t.Fatalf("prototext.Unmarshal(baseline) failed: %v", err)
	}

	gotSet, err := typedef.TypeDefSetFromYAML(yamlBytes)
	if err != nil {
		t.Fatalf("TypeDefSetFromYAML() failed: %v", err)
	}
	gotProto, err := typedef.TypeDefSetToProto(gotSet)
	if err != nil {
		t.Fatalf("TypeDefSetToProto() failed: %v", err)
	}

	if !proto.Equal(gotProto, wantProto) {
		t.Fatalf("TypeDefSetFromYAML() proto mismatch:\ngot:\n%s\nwant:\n%s",
			prototext.Format(gotProto), prototext.Format(wantProto))
	}
}

func yamlSliceByLines(yamlBytes []byte, startLine, endLine int) []byte {
	lines := bytes.Split(yamlBytes, []byte("\n"))
	startIdx := startLine - 1
	endIdx := len(lines)
	if endLine > 0 && endLine-1 < endIdx {
		endIdx = endLine - 1
	}
	return bytes.Join(lines[startIdx:endIdx], []byte("\n"))
}

func lookupMapValueNode(m *yaml.Node, key string) (*yaml.Node, bool) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], true
		}
	}
	return nil, false
}

func lowerFirstByte(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func TestCelTypeErrors(t *testing.T) {
	contents, err := os.ReadFile("testdata/cel_type_error_tests.yaml")
	if err != nil {
		t.Fatalf("os.ReadFile(testdata/cel_type_error_tests.yaml) failed: %v", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(contents, &doc); err != nil {
		t.Fatalf("yaml.Unmarshal() failed: %v", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.SequenceNode {
		t.Fatalf("expected top-level sequence in cel_type_error_tests.yaml")
	}
	seq := doc.Content[0]

	seenNames := make(map[string]bool)
	for i, entry := range seq.Content {
		testNode, ok := lookupMapValueNode(entry, "test")
		if !ok || testNode.Kind != yaml.MappingNode {
			t.Fatalf("entry %d missing 'test' map", i)
		}
		nameNode, ok := lookupMapValueNode(testNode, "name")
		if !ok || nameNode.Value == "" {
			t.Fatalf("entry %d missing 'name'", i)
		}
		testName := nameNode.Value
		if seenNames[testName] {
			t.Fatalf("duplicate test name %q", testName)
		}
		seenNames[testName] = true

		var expectedError string
		if goErrorNode, hasGoErr := lookupMapValueNode(testNode, "go_error"); hasGoErr {
			expectedError = strings.TrimRight(goErrorNode.Value, " \t\r\n")
		} else {
			errorNode, ok := lookupMapValueNode(testNode, "error")
			if !ok {
				t.Fatalf("test %q missing 'error'", testName)
			}
			// Go error strings start lowercase by convention while sharing the
			// C++ testdata file, so lowercase only the leading character.
			expectedError = lowerFirstByte(strings.TrimRight(errorNode.Value, " \t\r\n"))
		}
		if expectedError == "" {
			t.Fatalf("test %q has empty expected error", testName)
		}

		nextLine := 0
		if i+1 < len(seq.Content) {
			nextLine = seq.Content[i+1].Line
		}
		slice := yamlSliceByLines(contents, testNode.Line, nextLine)

		t.Run(testName, func(t *testing.T) {
			_, err := typedef.TypeDefSetFromYAML(slice)
			if err == nil {
				t.Fatalf("TypeDefSetFromYAML() succeeded, want error containing %q", expectedError)
			}
			if !errors.Is(err, typedef.ErrBadTypeDef) && !errors.Is(err, typedef.ErrAlreadyExists) {
				t.Errorf("error %v does not wrap ErrBadTypeDef or ErrAlreadyExists", err)
			}
			if !strings.Contains(err.Error(), expectedError) {
				t.Errorf("TypeDefSetFromYAML() error = %q, want substring %q", err.Error(), expectedError)
			}
		})
	}
	if len(seenNames) == 0 {
		t.Fatal("expected at least one error test case in cel_type_error_tests.yaml")
	}
}

// TestCelTypesToYAML verifies TypeDefToYAML and TypeDefSetToYAML against
// testdata/cel_types_to_yaml.baseline (ported from cel-cpp with expected Go
// formatting differences: omitted unspec_* fields, inline empty `proto: {}`,
// shortest-representation doubles, `.inf`/`-.inf`, and trailing newlines).
func TestCelTypesToYAML(t *testing.T) {
	txtpb, err := os.ReadFile("testdata/cel_types.txtproto")
	if err != nil {
		t.Fatalf("os.ReadFile(testdata/cel_types.txtproto) failed: %v", err)
	}
	protoSet := &typedefpb.TypeDefSet{}
	if err := prototext.Unmarshal(txtpb, protoSet); err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	typeDefSet, err := typedef.TypeDefSetFromProto(protoSet)
	if err != nil {
		t.Fatalf("TypeDefSetFromProto() failed: %v", err)
	}

	baselineBytes, err := os.ReadFile("testdata/cel_types_to_yaml.baseline")
	if err != nil {
		t.Fatalf("os.ReadFile(testdata/cel_types_to_yaml.baseline) failed: %v", err)
	}
	const separator = "--------------------------------------------------------------------\n"
	var out bytes.Buffer
	out.WriteString("PROTO SOURCE: cel_types.txtproto\n")
	for _, td := range typeDefSet.Types() {
		out.WriteString(separator)
		concise, err := typedef.TypeDefToYAML(td, typedef.UseTypeSignatures(true))
		if err != nil {
			t.Fatalf("TypeDefToYAML(%q, useSig=true) failed: %v", td.Name(), err)
		}
		out.Write(concise)

		verbose, err := typedef.TypeDefToYAML(td, typedef.UseTypeSignatures(false))
		if err != nil {
			t.Fatalf("TypeDefToYAML(%q, useSig=false) failed: %v", td.Name(), err)
		}
		if !bytes.Equal(verbose, concise) {
			out.WriteString("# use_type_signatures = false\n")
			out.Write(verbose)
		}
	}
	gotBaseline := strings.TrimSpace(out.String())
	wantBaseline := strings.TrimSpace(string(baselineBytes))
	if diff := cmp.Diff(wantBaseline, gotBaseline); diff != "" {
		t.Fatalf("TypeDefToYAML() baseline mismatch (-want +got):\n%s", diff)
	}

	wantProto, err := typedef.TypeDefSetToProto(typeDefSet)
	if err != nil {
		t.Fatalf("TypeDefSetToProto(original) failed: %v", err)
	}
	// Normalize the 4 fields whose proto type representation is not preserved
	// by YAML: unset map key -> dyn{}, error{} -> dyn{}, type{} -> type{dyn{}},
	// message_type -> abstract_type.
	for _, td := range wantProto.GetTypes() {
		if td.GetStructType().GetName() != "AllSupportedFieldTypes" {
			continue
		}
		for _, f := range td.GetStructType().GetFields() {
			switch f.GetName() {
			case "map_val_only_f":
				f.Type = &celpb.Type{TypeKind: &celpb.Type_MapType_{MapType: &celpb.Type_MapType{
					KeyType:   &celpb.Type{TypeKind: &celpb.Type_Dyn{}},
					ValueType: &celpb.Type{TypeKind: &celpb.Type_Primitive{Primitive: celpb.Type_INT64}},
				}}}
			case "error_f":
				f.Type = &celpb.Type{TypeKind: &celpb.Type_Dyn{}}
			case "type_unspec_f":
				f.Type = &celpb.Type{TypeKind: &celpb.Type_Type{Type: &celpb.Type{TypeKind: &celpb.Type_Dyn{}}}}
			case "shipping_addr":
				f.Type = &celpb.Type{TypeKind: &celpb.Type_AbstractType_{AbstractType: &celpb.Type_AbstractType{Name: "com.example.Address"}}}
			}
		}
	}

	for _, useSig := range []bool{true, false} {
		yamlOut, err := typedef.TypeDefSetToYAML(typeDefSet, typedef.UseTypeSignatures(useSig))
		if err != nil {
			t.Fatalf("TypeDefSetToYAML(useSig=%v) failed: %v", useSig, err)
		}
		roundTripped, err := typedef.TypeDefSetFromYAML(yamlOut)
		if err != nil {
			t.Fatalf("TypeDefSetFromYAML(useSig=%v) failed: %v\nYAML:\n%s", useSig, err, string(yamlOut))
		}
		gotProto, err := typedef.TypeDefSetToProto(roundTripped)
		if err != nil {
			t.Fatalf("TypeDefSetToProto(useSig=%v) failed: %v", useSig, err)
		}
		// TypeDefSetToYAML sorts by type name; compare each TypeDef by name.
		gotByName := make(map[string]*typedefpb.TypeDef, len(gotProto.GetTypes()))
		for _, td := range gotProto.GetTypes() {
			name := td.GetStructType().GetName()
			if td.GetEnumType() != nil {
				name = td.GetEnumType().GetName()
			}
			gotByName[name] = td
		}
		if len(gotByName) != len(wantProto.GetTypes()) {
			t.Fatalf("round-trip type count (useSig=%v) = %d, want %d", useSig, len(gotByName), len(wantProto.GetTypes()))
		}
		for _, wantTD := range wantProto.GetTypes() {
			name := wantTD.GetStructType().GetName()
			if wantTD.GetEnumType() != nil {
				name = wantTD.GetEnumType().GetName()
			}
			gotTD, ok := gotByName[name]
			if !ok {
				t.Fatalf("round-trip missing type %q (useSig=%v)", name, useSig)
			}
			if !proto.Equal(gotTD, wantTD) {
				t.Fatalf("round-trip proto mismatch for %q (useSig=%v):\ngot:\n%s\nwant:\n%s",
					name, useSig, prototext.Format(gotTD), prototext.Format(wantTD))
			}
		}
	}
}

func TestTypeDefFromYAMLInvalidYAML(t *testing.T) {
	empty, err := typedef.TypeDefFromYAML(nil)
	if err != nil {
		t.Fatalf("TypeDefFromYAML(nil) failed: %v", err)
	}
	if empty.Kind() != typedef.UnspecifiedKind {
		t.Errorf("TypeDefFromYAML(nil).Kind() = %v, want UnspecifiedKind", empty.Kind())
	}

	if _, err := typedef.TypeDefFromYAML([]byte("[1]")); !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), "invalid CEL type definition YAML") {
		t.Errorf("TypeDefFromYAML([1]) = %v, want ErrBadTypeDef with invalid CEL type definition YAML", err)
	}
	if _, err := typedef.TypeDefFromYAML([]byte("{ invalid: [ }")); !errors.Is(err, typedef.ErrBadTypeDef) {
		t.Errorf("TypeDefFromYAML(malformed) = %v, want ErrBadTypeDef", err)
	}
}

func TestTypeDefSetFromYAML(t *testing.T) {
	emptySet, err := typedef.TypeDefSetFromYAML(nil)
	if err != nil {
		t.Fatalf("TypeDefSetFromYAML(nil) failed: %v", err)
	}
	if len(emptySet.Types()) != 0 {
		t.Errorf("len(emptySet.Types()) = %d, want 0", len(emptySet.Types()))
	}

	mapYAML := `
types:
  - struct:
      name: "com.example.B"
  - enum:
      name: "com.example.A"
`
	set, err := typedef.TypeDefSetFromYAML([]byte(mapYAML))
	if err != nil {
		t.Fatalf("TypeDefSetFromYAML(mapYAML) failed: %v", err)
	}
	if len(set.Types()) != 2 {
		t.Fatalf("len(set.Types()) = %d, want 2", len(set.Types()))
	}
	if _, ok := set.FindTypeDef("com.example.A"); !ok {
		t.Errorf("FindTypeDef(com.example.A) = false, want true")
	}
	if _, ok := set.FindTypeDef("com.example.B"); !ok {
		t.Errorf("FindTypeDef(com.example.B) = false, want true")
	}

	dupYAML := `
- struct:
    name: "com.example.Dup"
- struct:
    name: "com.example.Dup"
`
	_, err = typedef.TypeDefSetFromYAML([]byte(dupYAML))
	if !errors.Is(err, typedef.ErrAlreadyExists) || !strings.Contains(err.Error(), `type "com.example.Dup"`) {
		t.Errorf("TypeDefSetFromYAML(dupYAML) = %v, want ErrAlreadyExists for com.example.Dup", err)
	}

	for _, bad := range []string{"123", "struct:\n  name: \"OnlyOne\"", "{ invalid: [ }"} {
		if _, err := typedef.TypeDefSetFromYAML([]byte(bad)); !errors.Is(err, typedef.ErrBadTypeDef) {
			t.Errorf("TypeDefSetFromYAML(%q) = %v, want ErrBadTypeDef", bad, err)
		}
	}
}

func TestTypeDefSetToYAML(t *testing.T) {
	emptyOut, err := typedef.TypeDefSetToYAML(&typedef.TypeDefSet{})
	if err != nil {
		t.Fatalf("TypeDefSetToYAML(empty) failed: %v", err)
	}
	if len(emptyOut) != 0 {
		t.Errorf("TypeDefSetToYAML(empty) = %q, want empty", string(emptyOut))
	}
	nilOut, err := typedef.TypeDefSetToYAML(nil)
	if err != nil {
		t.Fatalf("TypeDefSetToYAML(nil) failed: %v", err)
	}
	if len(nilOut) != 0 {
		t.Errorf("TypeDefSetToYAML(nil) = %q, want empty", string(nilOut))
	}

	s, _ := typedef.NewStruct("com.example.B")
	e, _ := typedef.NewEnum("com.example.A")
	set, err := typedef.NewTypeDefSet(s.AsTypeDef(), e.AsTypeDef())
	if err != nil {
		t.Fatalf("NewTypeDefSet() failed: %v", err)
	}

	out, err := typedef.TypeDefSetToYAML(set)
	if err != nil {
		t.Fatalf("TypeDefSetToYAML() failed: %v", err)
	}
	want := unindent(`
		- enum:
		    name: "com.example.A"
		- struct:
		    name: "com.example.B"
	`) + "\n"
	if string(out) != want {
		t.Errorf("TypeDefSetToYAML() got:\n%s\nwant:\n%s", string(out), want)
	}
}

func TestNaNDoubleToYAML(t *testing.T) {
	protoDef := &typedefpb.TypeDef{}
	if err := prototext.Unmarshal([]byte(`
		struct_type {
			name: "NanFloats"
			fields {
				name: "nan_val"
				type { primitive: DOUBLE }
				default_value { double_value: nan }
			}
		}
	`), protoDef); err != nil {
		t.Fatalf("prototext.Unmarshal() failed: %v", err)
	}
	td, err := typedef.TypeDefFromProto(protoDef)
	if err != nil {
		t.Fatalf("TypeDefFromProto() failed: %v", err)
	}

	out, err := typedef.TypeDefToYAML(td)
	if err != nil {
		t.Fatalf("TypeDefToYAML() failed: %v", err)
	}
	want := unindent(`
		struct:
		  name: "NanFloats"
		  fields:
		    - name: "nan_val"
		      type: "double"
		      default: .nan
	`) + "\n"
	if string(out) != want {
		t.Errorf("TypeDefToYAML(NaN) got:\n%s\nwant:\n%s", string(out), want)
	}
}

func TestAspectDurationAndTimestampWrittenAsStrings(t *testing.T) {
	aspect, err := typedef.NewAspect(map[string]ref.Val{
		"d": types.Duration{Duration: 10 * time.Second},
		"t": types.Timestamp{Time: time.Unix(0, 0).UTC()},
	})
	if err != nil {
		t.Fatalf("NewAspect() failed: %v", err)
	}
	s, err := typedef.NewStruct("com.example.AspectTime", typedef.StructAspect("proto", aspect))
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}

	out, err := typedef.TypeDefToYAML(s.AsTypeDef())
	if err != nil {
		t.Fatalf("TypeDefToYAML() failed: %v", err)
	}
	want := unindent(`
		struct:
		  name: "com.example.AspectTime"
		  aspects:
		    proto:
		      d: "10s"
		      t: "1970-01-01T00:00:00Z"
	`) + "\n"
	if string(out) != want {
		t.Errorf("TypeDefToYAML() got:\n%s\nwant:\n%s", string(out), want)
	}

	parsed, err := typedef.TypeDefFromYAML([]byte(unindent(`
		struct:
		  name: A
		  aspects:
		    proto:
		      d: "10s"
	`)))
	if err != nil {
		t.Fatalf("TypeDefFromYAML() failed: %v", err)
	}
	protoAspect, ok := parsed.Aspect("proto")
	if !ok {
		t.Fatalf("Aspect(proto) not found")
	}
	dVal, ok := protoAspect.Property("d")
	if !ok || dVal != types.String("10s") {
		t.Errorf("Property(d) = (%v, %v), want (\"10s\", true)", dVal, ok)
	}
}

func TestStructuredTypeSpecMaxDepthExceeded(t *testing.T) {
	deepType := types.IntType
	for i := 0; i < 35; i++ {
		deepType = types.NewListType(deepType)
	}
	field, err := typedef.NewField("deep_field", deepType)
	if err != nil {
		t.Fatalf("NewField() failed: %v", err)
	}
	s, err := typedef.NewStruct("com.example.DeepStruct", typedef.StructFields(field))
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}

	_, err = typedef.TypeDefToYAML(s.AsTypeDef(), typedef.UseTypeSignatures(false))
	if !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), "type specification exceeds maximum depth") {
		t.Errorf("TypeDefToYAML(deep) = %v, want ErrBadTypeDef with max depth", err)
	}
}

func TestTypeDefToYAMLNilAndEmptyErrors(t *testing.T) {
	if _, err := typedef.TypeDefToYAML(nil); !errors.Is(err, typedef.ErrBadTypeDef) {
		t.Errorf("TypeDefToYAML(nil) = %v, want ErrBadTypeDef", err)
	}
	if _, err := typedef.TypeDefToYAML(&typedef.TypeDef{}); !errors.Is(err, typedef.ErrBadTypeDef) {
		t.Errorf("TypeDefToYAML(unspecified) = %v, want ErrBadTypeDef", err)
	}
}

func TestAspectDoubleSpecialValuesRoundTrip(t *testing.T) {
	aspect, err := typedef.NewAspect(map[string]ref.Val{
		"nan_prop":     types.Double(math.NaN()),
		"pos_inf_prop": types.Double(math.Inf(1)),
		"neg_inf_prop": types.Double(math.Inf(-1)),
	})
	if err != nil {
		t.Fatalf("NewAspect() failed: %v", err)
	}
	s, err := typedef.NewStruct("com.example.AspectFloats", typedef.StructAspect("meta", aspect))
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}

	out, err := typedef.TypeDefToYAML(s.AsTypeDef())
	if err != nil {
		t.Fatalf("TypeDefToYAML() failed: %v", err)
	}
	parsed, err := typedef.TypeDefFromYAML(out)
	if err != nil {
		t.Fatalf("TypeDefFromYAML() failed: %v\nYAML:\n%s", err, string(out))
	}
	meta, ok := parsed.Aspect("meta")
	if !ok {
		t.Fatalf("Aspect(meta) not found")
	}
	nanVal, ok := meta.Property("nan_prop")
	if !ok {
		t.Fatalf("Property(nan_prop) not found")
	}
	if d, isDouble := nanVal.(types.Double); !isDouble || !math.IsNaN(float64(d)) {
		t.Errorf("nan_prop = %T(%v), want types.Double(NaN)", nanVal, nanVal)
	}
	posInf, _ := meta.Property("pos_inf_prop")
	if posInf != types.Double(math.Inf(1)) {
		t.Errorf("pos_inf_prop = %T(%v), want types.Double(+Inf)", posInf, posInf)
	}
	negInf, _ := meta.Property("neg_inf_prop")
	if negInf != types.Double(math.Inf(-1)) {
		t.Errorf("neg_inf_prop = %T(%v), want types.Double(-Inf)", negInf, negInf)
	}
}

func TestNullDefaultOnNonNullFields(t *testing.T) {
	for _, fieldType := range []string{"string", "int", "bytes", "com.example.Status"} {
		for _, defVal := range []string{"~", "null", ""} {
			y := fmt.Sprintf("struct:\n  name: S\n  fields:\n    - name: f\n      type: %s\n      default: %s\n", fieldType, defVal)
			_, err := typedef.TypeDefFromYAML([]byte(y))
			if !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), "field 'default' is not a scalar") {
				t.Errorf("type=%s default=%q got err=%v, want field 'default' is not a scalar", fieldType, defVal, err)
			}
		}
	}
}

func TestTypeSignatureValidationAndEscaping(t *testing.T) {
	badSigs := []struct {
		sig     string
		wantSub string
	}{
		{sig: "<int>", wantSub: "invalid identifier"},
		{sig: "a..b", wantSub: "invalid identifier"},
		{sig: "~", wantSub: "invalid type parameter name"},
		{sig: "~a<b", wantSub: "mismatched brackets"},
		{sig: "~a\\<b<c>", wantSub: "invalid type parameter name"},
		{sig: "~a,b", wantSub: "invalid type parameter name"},
		{sig: "list<int", wantSub: "mismatched brackets"},
		{sig: "list<int>>", wantSub: "mismatched brackets"},
		{sig: "list<int>extra", wantSub: "missing closing >"},
		{sig: "map<string>", wantSub: "map expects 0 or 2 parameters"},
		{sig: "type<int,string>", wantSub: "type expects at most 1 parameter"},
		{sig: "int<string>", wantSub: "int cannot have type parameters"},
	}
	for _, tc := range badSigs {
		t.Run(tc.sig, func(t *testing.T) {
			y := fmt.Sprintf("struct:\n  name: S\n  fields:\n    - name: f\n      type: %q\n", tc.sig)
			_, err := typedef.TypeDefFromYAML([]byte(y))
			if !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("sig %q got err=%v, want ErrBadTypeDef containing %q", tc.sig, err, tc.wantSub)
			}
		})
	}

	// Escaped characters in type names and type params round-trip cleanly.
	escapedYAML := unindent(`
		struct:
		  name: S
		  fields:
		    - name: f
		      type: "list<a\\<b\\>c>"
		    - name: p
		      type: "~T\\.U"
	`)
	td, err := typedef.TypeDefFromYAML([]byte(escapedYAML))
	if err != nil {
		t.Fatalf("TypeDefFromYAML(escaped) failed: %v", err)
	}
	fields := td.StructType().Fields()
	if got := fields[0].Type().Parameters()[0].TypeName(); got != "a<b>c" {
		t.Errorf("field f elem TypeName() = %q, want %q", got, "a<b>c")
	}
	if got := fields[1].Type().TypeName(); got != "T.U" {
		t.Errorf("field p TypeName() = %q, want %q", got, "T.U")
	}
}

func TestScalarTagFlexibility(t *testing.T) {
	structYAML := unindent(`
		struct:
		  name: 123
		  doc: true
		  fields:
		    - name: f
		      type: 456
		    - name: p
		      type:
		        type_name: T
		        is_type_param: yes
	`)
	td, err := typedef.TypeDefFromYAML([]byte(structYAML))
	if err != nil {
		t.Fatalf("TypeDefFromYAML(struct scalar tags) failed: %v", err)
	}
	if td.Name() != "123" || td.Doc() != "true" {
		t.Errorf("struct name=%q doc=%q, want 123 / true", td.Name(), td.Doc())
	}
	if got := td.StructType().Fields()[0].Type().TypeName(); got != "456" {
		t.Errorf("field f type = %q, want 456", got)
	}
	if got := td.StructType().Fields()[1].Type(); got.Kind() != types.TypeParamKind || got.TypeName() != "T" {
		t.Errorf("field p type = %v, want TypeParamKind(T)", got)
	}

	enumYAML := unindent(`
		enum:
		  name: BoolEnum
		  constants:
		    - name: TRUE
		      id: "5"
		    - name: FALSE
		      id: -2
	`)
	etd, err := typedef.TypeDefFromYAML([]byte(enumYAML))
	if err != nil {
		t.Fatalf("TypeDefFromYAML(enum scalar tags) failed: %v", err)
	}
	consts := etd.EnumType().Constants()
	if consts[0].Name() != "TRUE" || consts[0].ID() != 5 {
		t.Errorf("const[0] = (%q, %d), want (TRUE, 5)", consts[0].Name(), consts[0].ID())
	}
	if consts[1].Name() != "FALSE" || consts[1].ID() != -2 {
		t.Errorf("const[1] = (%q, %d), want (FALSE, -2)", consts[1].Name(), consts[1].ID())
	}
}

func TestYAMLAliasSupportAndFanOutLimit(t *testing.T) {
	y := unindent(`
		x_defs:
		  - &fields_ref
		    - name: f1
		      type: &type_ref
		        type_name: int
		      default: &def_ref 42
		      aspects: &aspect_ref
		        meta: &aspect_val
		          k: 7
		    - name: f2
		      type: *type_ref
		types: &types_ref
		  - &td_ref
		    struct:
		      name: S
		      fields: *fields_ref
		      aspects: *aspect_ref
	`)
	set, err := typedef.TypeDefSetFromYAML([]byte(y))
	if err != nil {
		t.Fatalf("TypeDefSetFromYAML(aliases) failed: %v", err)
	}
	td, ok := set.FindTypeDef("S")
	if !ok {
		t.Fatal("FindTypeDef(S) = false")
	}
	if len(td.StructType().Fields()) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(td.StructType().Fields()))
	}
	if def := td.StructType().Fields()[0].DefaultValue(); def != types.Int(42) {
		t.Errorf("f1 default = %v, want 42", def)
	}
	if a, ok := td.Aspect("meta"); !ok {
		t.Error("struct aspect meta missing")
	} else if v, _ := a.Property("k"); v != types.Int(7) {
		t.Errorf("aspect property k = %v, want 7", v)
	}

	// Build a shallow (depth 18 <= maxTypeSpecDepth) DAG of 4-way aliases (4^18 nodes).
	var fanout strings.Builder
	fanout.WriteString("struct:\n  name: S\n  fields:\n    - name: f\n      type: &a0\n")
	for i := 0; i < 18; i++ {
		indent := strings.Repeat("    ", i) + "        "
		fmt.Fprintf(&fanout, "%stype_name: function\n%sparams:\n%s  - &a%d\n", indent, indent, indent, i+1)
	}
	leafIndent := strings.Repeat("    ", 18) + "        "
	fmt.Fprintf(&fanout, "%stype_name: int\n", leafIndent)
	for i := 17; i >= 0; i-- {
		indent := strings.Repeat("    ", i) + "        "
		for j := 0; j < 3; j++ {
			fmt.Fprintf(&fanout, "%s  - *a%d\n", indent, i+1)
		}
	}
	_, err = typedef.TypeDefFromYAML([]byte(fanout.String()))
	if !errors.Is(err, typedef.ErrBadTypeDef) || !strings.Contains(err.Error(), "maximum size") {
		t.Errorf("fan-out alias YAML got err=%v, want ErrBadTypeDef with maximum size", err)
	}
}

func TestDuplicateAspectAndPropertyRejected(t *testing.T) {
	dupAspect := unindent(`
		struct:
		  name: S
		  aspects:
		    proto: { a: 1 }
		    proto: { b: 2 }
	`)
	if _, err := typedef.TypeDefFromYAML([]byte(dupAspect)); !errors.Is(err, typedef.ErrAlreadyExists) {
		t.Errorf("duplicate aspect err = %v, want ErrAlreadyExists", err)
	}

	dupProp := unindent(`
		struct:
		  name: S
		  aspects:
		    proto:
		      a: 1
		      a: 2
	`)
	if _, err := typedef.TypeDefFromYAML([]byte(dupProp)); !errors.Is(err, typedef.ErrAlreadyExists) {
		t.Errorf("duplicate aspect property err = %v, want ErrAlreadyExists", err)
	}
}

func TestUntypedAspectInference(t *testing.T) {
	y := unindent(`
		struct:
		  name: S
		  aspects:
		    meta:
		      tagged_int: !!int 5
		      tagged_str: !!str 5
		      quoted_str: "5"
		      mixed_bool: tRuE
		      plain_inf: inf
		      dot_inf: .inf
		      neg_inf: -.inf
		      plain_uint: 42u
		      plus_uint: +5u
		      bytes_val: b"abc"
		      bad_octal_bytes: b"\0"
		      bad_quote_bytes: b"a"b"
	`)
	td, err := typedef.TypeDefFromYAML([]byte(y))
	if err != nil {
		t.Fatalf("TypeDefFromYAML() failed: %v", err)
	}
	meta, _ := td.Aspect("meta")
	wants := map[string]ref.Val{
		"tagged_int":      types.Int(5),
		"tagged_str":      types.String("5"),
		"quoted_str":      types.String("5"),
		"mixed_bool":      types.True,
		"plain_inf":       types.Double(math.Inf(1)),
		"dot_inf":         types.Double(math.Inf(1)),
		"neg_inf":         types.Double(math.Inf(-1)),
		"plain_uint":      types.Uint(42),
		"plus_uint":       types.Uint(5),
		"bytes_val":       types.Bytes("abc"),
		"bad_octal_bytes": types.String(`b"\0"`),
		"bad_quote_bytes": types.String(`b"a"b"`),
	}
	for k, want := range wants {
		got, ok := meta.Property(k)
		if !ok {
			t.Errorf("missing property %q", k)
			continue
		}
		if got.Equal(want) != types.True {
			t.Errorf("property %q = %T(%v), want %T(%v)", k, got, got, want, want)
		}
	}
}

func TestTypedDefaultEdgeCases(t *testing.T) {
	y := unindent(`
		struct:
		  name: S
		  fields:
		    - name: s_bin
		      type: string
		      default: !!binary aGVsbG8=
		    - name: u_plus
		      type: uint
		      default: +5
		    - name: ts_offset
		      type: timestamp
		      default: 2026-10-05T01:00:00+01:00
		    - name: b_raw
		      type: bytes
		      default: 'rb"\x41"'
		    - name: b_triple
		      type: bytes
		      default: "b'''x''x'''"
		    - name: b_bad_fallback
		      type: bytes
		      default: 'b"a"b"'
	`)
	td, err := typedef.TypeDefFromYAML([]byte(y))
	if err != nil {
		t.Fatalf("TypeDefFromYAML() failed: %v", err)
	}
	wants := map[string]ref.Val{
		"s_bin":          types.String("hello"),
		"u_plus":         types.Uint(5),
		"ts_offset":      types.Timestamp{Time: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)},
		"b_raw":          types.Bytes(`\x41`),
		"b_triple":       types.Bytes(`x''x`),
		"b_bad_fallback": types.Bytes(`b"a"b"`),
	}
	for _, f := range td.StructType().Fields() {
		want, ok := wants[f.Name()]
		if !ok {
			continue
		}
		if f.DefaultValue().Equal(want) != types.True {
			t.Errorf("field %q default = %T(%v), want %T(%v)", f.Name(), f.DefaultValue(), f.DefaultValue(), want, want)
		}
	}
}

func TestDurationFormattingParity(t *testing.T) {
	durations := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "zero", d: 0, want: "0"},
		{name: "hms", d: 90 * time.Minute, want: "1h30m"},
		{name: "micro", d: 1500 * time.Nanosecond, want: "1.5us"},
		{name: "neg_milli", d: -2500 * time.Microsecond, want: "-2.5ms"},
		{name: "nano", d: 500 * time.Nanosecond, want: "500ns"},
		{name: "frac_sec", d: time.Hour + 500*time.Millisecond, want: "1h0.5s"},
	}
	for _, tc := range durations {
		t.Run(tc.name, func(t *testing.T) {
			f, err := typedef.NewField("d", types.DurationType, typedef.FieldDefaultValue(types.Duration{Duration: tc.d}))
			if err != nil {
				t.Fatalf("NewField() failed: %v", err)
			}
			s, err := typedef.NewStruct("S", typedef.StructFields(f))
			if err != nil {
				t.Fatalf("NewStruct() failed: %v", err)
			}
			out, err := typedef.TypeDefToYAML(s.AsTypeDef())
			if err != nil {
				t.Fatalf("TypeDefToYAML() failed: %v", err)
			}
			wantLine := "default: " + tc.want
			if !strings.Contains(string(out), wantLine) {
				t.Errorf("YAML output:\n%s\nwant substring %q", string(out), wantLine)
			}
			rt, err := typedef.TypeDefFromYAML(out)
			if err != nil {
				t.Fatalf("TypeDefFromYAML() failed: %v", err)
			}
			if got := rt.StructType().Fields()[0].DefaultValue(); got != (types.Duration{Duration: tc.d}) {
				t.Errorf("round-trip duration = %v, want %v", got, tc.d)
			}
		})
	}
}
