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

	"cel.dev/cel-go/cel"
)

// Example_cel_TypeConversions showcases type casting functions (int, uint, double, string, bytes, dyn)
// as documented in https://celbyexample.com/type-conversions/
func Example_cel_TypeConversions() {
	exprs := []string{
		`int("123")`,
		`int(3.14)`,
		`uint(42)`,
		`double("3.14")`,
		`double(100)`,
		`string(42)`,
		`string(true)`,
		`bytes("hello")`,
		`dyn("anything")`,
	}

	for _, expr := range exprs {
		prg, err := cel.Compile(expr)
		if err != nil {
			log.Fatalf("cel.Compile() error for %q: %v", expr, err)
		}
		out, _, err := prg.Eval(cel.NoVars())
		if err != nil {
			log.Fatalf("prg.Eval() error for %q: %v", expr, err)
		}
		fmt.Printf("%s -> %v\n", expr, out)
	}

	// Output:
	// int("123") -> 123
	// int(3.14) -> 3
	// uint(42) -> 42
	// double("3.14") -> 3.14
	// double(100) -> 100
	// string(42) -> 42
	// string(true) -> true
	// bytes("hello") -> [104 101 108 108 111]
	// dyn("anything") -> anything
}

// Example_cel_TypeIntrospection showcases type identification using type(x)
// as documented in https://celbyexample.com/type-introspection/
func Example_cel_TypeIntrospection() {
	exprs := []string{
		`type("hello") == string`,
		`type(42) == int`,
		`type(3.14) == double`,
		`type(true) == bool`,
		`type([1, 2, 3])`,
		`type({"a": 1})`,
	}

	for _, expr := range exprs {
		prg, err := cel.Compile(expr)
		if err != nil {
			log.Fatalf("cel.Compile() error for %q: %v", expr, err)
		}
		out, _, err := prg.Eval(cel.NoVars())
		if err != nil {
			log.Fatalf("prg.Eval() error for %q: %v", expr, err)
		}
		fmt.Printf("%s -> %v\n", expr, out)
	}

	// Output:
	// type("hello") == string -> true
	// type(42) == int -> true
	// type(3.14) == double -> true
	// type(true) == bool -> true
	// type([1, 2, 3]) -> list(dyn)
	// type({"a": 1}) -> map(dyn, dyn)
}
