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
)

// Example_cel_TimestampsAndDurations showcases timestamps, durations, arithmetic, and field access
// as documented in https://celbyexample.com/timestamps-and-durations/
func Example_cel_TimestampsAndDurations() {
	exprs := []string{
		`timestamp("2024-01-15T10:30:45Z")`,
		`duration("1h30m")`,
		`timestamp("2024-01-15T10:30:45Z") + duration("1h")`,
		`timestamp("2024-01-15T10:30:45Z") - timestamp("2024-01-15T09:30:45Z")`,
		`timestamp("2024-01-15T10:30:45Z").getFullYear()`,
		`timestamp("2024-01-15T10:30:45Z").getMonth()`,
		`timestamp("2024-01-15T10:30:45Z").getDayOfMonth()`,
		`duration("1h").getHours()`,
		`duration("90m").getMinutes()`,
		`timestamp("2024-01-15T10:30:45Z") > timestamp("2024-01-01T00:00:00Z")`,
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
	// timestamp("2024-01-15T10:30:45Z") -> 2024-01-15 10:30:45 +0000 UTC
	// duration("1h30m") -> 5400s
	// timestamp("2024-01-15T10:30:45Z") + duration("1h") -> 2024-01-15 11:30:45 +0000 UTC
	// timestamp("2024-01-15T10:30:45Z") - timestamp("2024-01-15T09:30:45Z") -> 3600s
	// timestamp("2024-01-15T10:30:45Z").getFullYear() -> 2024
	// timestamp("2024-01-15T10:30:45Z").getMonth() -> 0
	// timestamp("2024-01-15T10:30:45Z").getDayOfMonth() -> 14
	// duration("1h").getHours() -> 1
	// duration("90m").getMinutes() -> 90
	// timestamp("2024-01-15T10:30:45Z") > timestamp("2024-01-01T00:00:00Z") -> true
}

// Example_cel_Time showcases timezones, timestamp components, and epoch conversions
// as documented in https://celbyexample.com/time/
func Example_cel_Time() {
	exprs := []string{
		`timestamp("2024-01-15T10:30:45Z").getHours("America/New_York")`,
		`timestamp("2024-01-15T10:30:45Z").getDayOfWeek("UTC")`,
		`duration("3600s")`,
		`duration("-500ms")`,
		`timestamp(1705314645)`,
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
	// timestamp("2024-01-15T10:30:45Z").getHours("America/New_York") -> 5
	// timestamp("2024-01-15T10:30:45Z").getDayOfWeek("UTC") -> 1
	// duration("3600s") -> 3600s
	// duration("-500ms") -> -0.5s
	// timestamp(1705314645) -> 2024-01-15 10:30:45 +0000 UTC
}
