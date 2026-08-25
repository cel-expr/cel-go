// Copyright 2026 Google LLC
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

package examples

import (
	"fmt"
	"log"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/test/proto3pb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Example_cel_ProtocolBuffers showcases evaluating protobuf messages, enum values,
// field presence (has), and default values as documented in https://celbyexample.com/protocol-buffers/
func Example_cel_ProtocolBuffers() {
	userMsg := &proto3pb.TestAllTypes{
		SingleString: "Alice",
		SingleInt64:  42,
		NestedType:   &proto3pb.TestAllTypes_SingleNestedEnum{SingleNestedEnum: proto3pb.TestAllTypes_BAR},
	}

	exprs := []string{
		`user.single_string`,
		`user.single_int64 == 42`,
		`user.single_nested_enum == google.expr.proto3.test.TestAllTypes.NestedEnum.BAR`,
		`has(user.single_string)`,
		`has(user.single_bytes)`,
		`user.single_bool`,
	}

	opts := []cel.EnvOption{
		cel.Types(userMsg),
		cel.Variable("user", cel.ObjectType("google.expr.proto3.test.TestAllTypes")),
	}

	for _, expr := range exprs {
		prg, err := cel.Compile(expr, opts...)
		if err != nil {
			log.Fatalf("cel.Compile() error for %q: %v", expr, err)
		}
		out, _, err := prg.Eval(map[string]any{"user": userMsg})
		if err != nil {
			log.Fatalf("prg.Eval() error for %q: %v", expr, err)
		}
		fmt.Printf("%s -> %v\n", expr, out)
	}

	// Output:
	// user.single_string -> Alice
	// user.single_int64 == 42 -> true
	// user.single_nested_enum == google.expr.proto3.test.TestAllTypes.NestedEnum.BAR -> true
	// has(user.single_string) -> true
	// has(user.single_bytes) -> false
	// user.single_bool -> false
}

// Example_cel_WellKnownTypes showcases wrapper types (StringValue, BoolValue) and google.protobuf.Struct
// as documented in https://celbyexample.com/well-known-types/
func Example_cel_WellKnownTypes() {
	structVal, _ := structpb.NewStruct(map[string]any{
		"key":   "value",
		"count": 10,
	})

	vars := map[string]any{
		"wrapped_str": wrapperspb.String("hello"),
		"json_obj":    structVal,
	}

	exprs := []string{
		`wrapped_str == "hello"`,
		`json_obj.key == "value"`,
		`json_obj.count == 10`,
	}

	opts := []cel.EnvOption{
		cel.Types(structVal),
		cel.Variable("wrapped_str", cel.ObjectType("google.protobuf.StringValue")),
		cel.Variable("json_obj", cel.ObjectType("google.protobuf.Struct")),
	}

	for _, expr := range exprs {
		prg, err := cel.Compile(expr, opts...)
		if err != nil {
			log.Fatalf("cel.Compile() error for %q: %v", expr, err)
		}
		out, _, err := prg.Eval(vars)
		if err != nil {
			log.Fatalf("prg.Eval() error for %q: %v", expr, err)
		}
		fmt.Printf("%s -> %v\n", expr, out)
	}

	// Output:
	// wrapped_str == "hello" -> true
	// json_obj.key == "value" -> true
	// json_obj.count == 10 -> true
}
