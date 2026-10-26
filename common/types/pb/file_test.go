// Copyright 2022 Google LLC
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

package pb

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"cel.dev/cel-go/checker/decls"

	proto2pb "cel.dev/cel-go/test/proto2pb"
	proto3pb "cel.dev/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	descpb "google.golang.org/protobuf/types/descriptorpb"
)

func TestFileDescriptionGetExtensions(t *testing.T) {
	pbdb := NewDb()
	registerFileDescriptor(t, pbdb, &proto2pb.TestAllTypes{})
	registerFileDescriptor(t, pbdb, &proto2pb.ExternalMessageType{})
	ex, found := pbdb.DescribeType("google.expr.proto2.test.ExampleType")
	if !found {
		t.Fatal("ExampleType not found")
	}
	tests := []struct {
		field     string
		fieldType *exprpb.Type
	}{
		{
			field:     "google.expr.proto2.test.nested_example",
			fieldType: decls.NewObjectType("google.expr.proto2.test.ExampleType"),
		},
		{
			field:     "google.expr.proto2.test.int32_ext",
			fieldType: decls.Int,
		},
		{
			field:     "google.expr.proto2.test.ExtendedExampleType.extended_examples",
			fieldType: decls.NewListType(decls.String),
		},
		{
			field:     "google.expr.proto2.test.ExtendedExampleType.enum_ext",
			fieldType: decls.Int,
		},
		{
			field:     "google.expr.proto2.test.ExternalMessageType.int64_ext",
			fieldType: decls.Int,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.field, func(t *testing.T) {
			field, found := ex.FieldByName(tc.field)
			if !found {
				t.Fatalf("%s extension not found", tc.field)
			}
			if !proto.Equal(field.CheckedType(), tc.fieldType) {
				t.Errorf("Got %v, wanted %v", field.CheckedType(), tc.fieldType)
			}
		})
	}
}

func TestFileDescriptionJSONFieldNames(t *testing.T) {
	pbdb := NewDb(JSONFieldNames(true))
	msg := &proto2pb.TestAllTypes{}
	fd, err := pbdb.RegisterMessage(msg)
	if err != nil {
		t.Fatalf("pbdb.RegisterMessage() failed: %v", err)
	}
	fileDesc := msg.ProtoReflect().Descriptor().ParentFile()
	if fd.FileDescriptor() != fileDesc {
		t.Errorf("got %v, wanted %v file descriptor", fd.FileDescriptor(), fileDesc)
	}
}

func TestFileDescriptionGetTypes(t *testing.T) {
	pbdb := NewDb()
	fd, err := pbdb.RegisterMessage(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Error(err)
	}
	expected := []string{
		"google.expr.proto3.test.TestAllTypes",
		"google.expr.proto3.test.TestAllTypes.NestedMessage",
		"google.expr.proto3.test.TestAllTypes.MapStringStringEntry",
		"google.expr.proto3.test.TestAllTypes.MapInt64NestedTypeEntry",
		"google.expr.proto3.test.TestJsonNames",
		"google.expr.proto3.test.NestedTestAllTypes"}
	if len(fd.GetTypeNames()) != len(expected) {
		t.Errorf("got '%v', wanted '%v'", fd.GetTypeNames(), expected)
	}
	for _, tn := range fd.GetTypeNames() {
		var found = false
		if slices.Contains(expected, tn) {
			found = true
		}
		if !found {
			t.Error("Unexpected type name", tn)
		}
	}
	for _, typeName := range fd.GetTypeNames() {
		td, found := fd.GetTypeDescription(typeName)
		if !found {
			t.Fatalf("fd.GetTypeDescription(%v) returned not found", typeName)
		}
		if td.Name() != typeName {
			t.Error("Indexed type name not equal to descriptor type name", td, typeName)
		}
	}
}

func TestFileDescriptionGetEnumNames(t *testing.T) {
	pbdb := NewDb()
	fd, err := pbdb.RegisterMessage(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]int32{
		"google.expr.proto3.test.TestAllTypes.NestedEnum.FOO": 0,
		"google.expr.proto3.test.TestAllTypes.NestedEnum.BAR": 1,
		"google.expr.proto3.test.TestAllTypes.NestedEnum.BAZ": 2,
		"google.expr.proto3.test.GlobalEnum.GOO":              0,
		"google.expr.proto3.test.GlobalEnum.GAR":              1,
		"google.expr.proto3.test.GlobalEnum.GAZ":              2}
	if len(expected) != len(fd.GetEnumNames()) {
		t.Error("Count of enum names does not match expected'",
			fd.GetEnumNames(), expected)
	}
	for _, enumName := range fd.GetEnumNames() {
		if enumVal, found := expected[enumName]; found {
			ed, found := fd.GetEnumDescription(enumName)
			if !found {
				t.Fatalf("fd.GetEnumDescription(%v) returned not found", enumName)
			}
			if ed.Value() != enumVal {
				t.Errorf("Enum did not have expected value. %s got '%v', wanted '%v'",
					enumName, ed.Value(), enumVal)
			}
		} else {
			t.Errorf("Enum value not found for: %s", enumName)
		}
	}
}

func TestFileDescriptionGetImportedEnumNames(t *testing.T) {
	pbdb := NewDb()
	fdMap := CollectFileDescriptorSet(&proto3pb.TestAllTypes{})
	fileSet := make([]*descpb.FileDescriptorProto, 0, len(fdMap))
	for _, fd := range fdMap {
		fileSet = append(fileSet, protodesc.ToFileDescriptorProto(fd))
	}
	fds := &descpb.FileDescriptorSet{File: fileSet}
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		t.Logf("registering file: %v", fd.Path())
		_, err = pbdb.RegisterDescriptor(fd)
		if err != nil {
			t.Fatalf("pbdb.RegisterDescriptor(%v) failed: %v", fd, err)
			return false
		}
		return true
	})
	imported := map[string]int32{
		"google.expr.proto3.test.ImportedGlobalEnum.IMPORT_FOO": 0,
		"google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR": 1,
		"google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAZ": 2,
	}
	for enumName, value := range imported {
		ed, found := pbdb.DescribeEnum(enumName)
		if !found {
			t.Fatalf("pbdb.DescribeEnum(%q) returned not found", enumName)
		}
		if ed.Value() != value {
			t.Errorf("Got %v, wanted %v for enum %s", ed, value, enumName)
		}
	}
}

func registerFileDescriptor(t *testing.T, pbdb *Db, pbMsg proto.Message) {
	fileDesc := pbMsg.ProtoReflect().Descriptor().ParentFile()
	_, err := pbdb.RegisterDescriptor(fileDesc)
	if err != nil {
		t.Fatalf("pbdb.RegisterDescriptor(%v) failed: %v", fileDesc, err)
	}
}
