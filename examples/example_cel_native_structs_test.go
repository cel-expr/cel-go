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

package examples_test

import (
	"fmt"
	"log"
	"reflect"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/examples"
	"cel.dev/cel-go/ext"
)

// Example_cel_NativeTypes showcases evaluating Go native structs using ext.NativeTypes() with json struct tags
func Example_cel_NativeTypes() {
	u := examples.User{
		Name:  "Alice",
		Age:   30,
		Roles: []string{"admin", "editor"},
	}

	exprs := []string{
		`user.name == "Alice"`,
		`user.age >= 18`,
		`"admin" in user.roles`,
		`user.name + " (" + string(user.age) + ")"`,
	}

	for _, expr := range exprs {
		prg, err := cel.Compile(expr,
			cel.Variable("user", cel.ObjectType("examples.User")),
			ext.NativeTypes(reflect.TypeFor[examples.User](), ext.ParseStructTag("json")),
		)
		if err != nil {
			log.Fatalf("cel.Compile() error for %q: %v", expr, err)
		}
		out, _, err := prg.Eval(map[string]any{"user": u})
		if err != nil {
			log.Fatalf("prg.Eval() error for %q: %v", expr, err)
		}
		fmt.Printf("%s -> %v\n", expr, out)
	}

	// Output:
	// user.name == "Alice" -> true
	// user.age >= 18 -> true
	// "admin" in user.roles -> true
	// user.name + " (" + string(user.age) + ")" -> Alice (30)
}

// Example_cel_NativeTypes_structTags showcases mapping struct field names via cel tags
func Example_cel_NativeTypes_structTags() {
	acc := examples.Account{
		ID:        1001,
		OwnerName: "Bob",
	}

	prg, err := cel.Compile(`acc.owner == "Bob" && acc.id == 1001`,
		cel.Variable("acc", cel.ObjectType("examples.Account")),
		ext.NativeTypes(reflect.TypeFor[examples.Account](), ext.ParseStructTags(true)),
	)
	if err != nil {
		log.Fatalf("cel.Compile() error: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"acc": acc})
	if err != nil {
		log.Fatalf("prg.Eval() error: %v", err)
	}
	fmt.Println(out)

	// Output:
	// true
}
