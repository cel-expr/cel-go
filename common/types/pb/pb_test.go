// Copyright 2020 Google LLC
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
	"reflect"
	"slices"
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	proto2pb "cel.dev/cel-go/test/proto2pb"
	proto3pb "cel.dev/cel-go/test/proto3pb"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	dynamicpb "google.golang.org/protobuf/types/dynamicpb"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestDbJSONFieldNames(t *testing.T) {
	pbdb := NewDb(JSONFieldNames(true))
	if pbdb.JSONFieldNames() != true {
		t.Errorf("pbdb.JSONFieldNames() got %v, wanted true", pbdb.JSONFieldNames())
	}
	fd, err := pbdb.RegisterMessage(&proto2pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("pbdb.RegisterMessage() failed: %v", err)
	}
	td, found := fd.GetTypeDescription("google.expr.proto2.test.TestAllTypes")
	if !found {
		t.Fatal("fd.GetTypeDescription() not found")
	}
	var fieldNames []string
	for fieldName, f := range td.FieldMap() {
		fieldNames = append(fieldNames, fieldName)
		if f.jsonFieldName != true {
			t.Error("f.jsonFieldName did not propagate")
		}
	}
	// Note that 'group' type names don't have camelCase representations.
	wantFields := []string{"singleInt32", "repeatedInt64", "nestedgroup"}
	for _, want := range wantFields {
		found := slices.Contains(fieldNames, want)
		if !found {
			t.Errorf("%v field name not found", want)
		}
	}
	copied := pbdb.Copy()
	if copied.JSONFieldNames() != true {
		t.Errorf("copied.JSONFieldNames() got %v, wanted true", copied.JSONFieldNames())
	}
	td, found = copied.DescribeType("google.expr.proto2.test.TestAllTypes")
	if !found {
		t.Fatal("copied.DescribeType() not found")
	}
	fieldNames = []string{}
	for fieldName, f := range td.FieldMap() {
		fieldNames = append(fieldNames, fieldName)
		if f.jsonFieldName != true {
			t.Error("f.jsonFieldName did not propagate")
		}
	}
	for _, want := range wantFields {
		found := slices.Contains(fieldNames, want)
		if !found {
			t.Errorf("%v field name not found", want)
		}
	}
}

func TestDbCopy(t *testing.T) {
	clone := DefaultDb.Copy()
	if !reflect.DeepEqual(clone, DefaultDb) {
		t.Error("db.Copy() did not result in equivalent objects.")
	}
	_, err := clone.RegisterMessage(&proto3pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("db.RegisterMessage() failed: %v", err)
	}
	if reflect.DeepEqual(clone, DefaultDb) {
		t.Error("db.RegisterMessage() altered the default db as well")
	}

	// modify the clone after deriving another copy.
	clone2 := clone.Copy()
	if !reflect.DeepEqual(clone, clone2) {
		t.Error("db.Copy() did not result in equivalent objects.")
	}
	_, err = clone.RegisterMessage(&proto2pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("Db.RegisterMessage() failed: %v", err)
	}
	if reflect.DeepEqual(clone, clone2) {
		t.Error("clone modification altered derived clone2 also")
	}

	// modify the clone2 with some extensions and assert equivalence to clone3
	_, err = clone2.RegisterMessage(&proto2pb.TestAllTypes{})
	if err != nil {
		t.Fatalf("Db.RegisterMessage() failed: %v", err)
	}
	_, err = clone2.RegisterMessage(&proto2pb.ExternalMessageType{})
	if err != nil {
		t.Fatalf("Db.RegisterMessage() failed: %v", err)
	}
	clone3 := clone2.Copy()
	if !reflect.DeepEqual(clone2, clone3) {
		t.Error("db.Copy() did not result in equivalent objects.")
	}
}

func BenchmarkDbCopy(b *testing.B) {
	for b.Loop() {
		NewDb().Copy()
	}
}

func TestProtoReflectRoundTrip(t *testing.T) {
	msg := &proto3pb.TestAllTypes{SingleBool: true}
	fdMap := CollectFileDescriptorSet(msg)
	files := []*descpb.FileDescriptorProto{}
	for _, fd := range fdMap {
		files = append(files, protodesc.ToFileDescriptorProto(fd))
	}
	// Round-tripping from a protoreflect.FileDescriptor to a FileDescriptorSet and back
	// will result in completely independent copies of the protoreflect.FileDescriptor
	// whose values are incompatible with each other.
	//
	// This test showcases what happens when the protoregistry.GlobalFiles values are
	// used when a given protoreflect.FileDescriptor is linked into the binary.
	fds := &descpb.FileDescriptorSet{File: files}
	pbReg, err := protodesc.NewFiles(fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	pbdb := NewDb()
	pbReg.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		_, err := pbdb.RegisterDescriptor(fd)
		if err != nil {
			t.Fatalf("pbdb.RegisterDecriptor(%v) failed: %v", fd, err)
		}
		return true
	})
	msgType, found := pbdb.DescribeType("google.expr.proto3.test.TestAllTypes")
	if !found {
		t.Fatal("pbdb.DescribeType(google.expr.proto3.test.TestAllTypes) failed")
	}
	boolField, found := msgType.FieldByName("single_bool")
	if !found {
		t.Fatal("msgType.FieldByName(single_bool) failed")
	}
	val, err := boolField.GetFrom(msg)
	if err != nil {
		t.Errorf("boolField.GetFrom(msg) failed: %v", err)
	}
	if val != true {
		t.Errorf("got TestAllTypes.single_bool %v, wanted true", val)
	}
}

func TestMerge(t *testing.T) {
	timestampPB := &tpb.Timestamp{}
	fileDesc := protodesc.ToFileDescriptorProto(timestampPB.ProtoReflect().Descriptor().ParentFile())
	fds := &descpb.FileDescriptorSet{}
	fds.File = append(fds.File, fileDesc)
	pbFiles, err := protodesc.NewFiles(fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles(%v) failed: %v", fds, err)
	}
	tsDesc, err := pbFiles.FindDescriptorByName(protoreflect.FullName("google.protobuf.Timestamp"))
	if err != nil {
		t.Fatalf("pbFiles.FindDescriptorByName() failed for Timestamp: %v", err)
	}
	tsMsgDesc := tsDesc.(protoreflect.MessageDescriptor)
	tsType := dynamicpb.NewMessageType(tsMsgDesc)
	dynTimestampPB := tsType.New()
	dynTimestampPB.Set(tsMsgDesc.Fields().ByName(protoreflect.Name("seconds")), protoreflect.ValueOf(int64(123)))
	err = Merge(timestampPB, dynTimestampPB.Interface())
	if err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
}

func TestMergeError(t *testing.T) {
	timestampPB := &tpb.Timestamp{}
	durationPB := &durationpb.Duration{}
	err := Merge(timestampPB, durationPB)
	if err == nil || err.Error() != "pb.Merge() arguments must be the same type. got: google.protobuf.Timestamp, google.protobuf.Duration" {
		t.Fatalf("got err: %v, wanted bad argument error", err)
	}
}
