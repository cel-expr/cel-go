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

// TestParserRecursionDepth evaluates stack memory consumption and crash boundaries
// when parsing deeply nested and complex CEL expressions.
//
// This test determines the dependency of parser stack usage on expression complexity
// across four structural categories:
//   - Parens: Nested parentheses grouping, e.g. `(((...42...)))`
//   - Unary:  Chained unary operators, e.g. `! ! ! ... true`
//   - Binary: Nested binary operations, e.g. `1 + (1 + (...42...))`
//   - List:   Nested list literals, e.g. `[[[...42...]]]`
//
// Mechanics:
// Go goroutines dynamically grow their stack frames. In order to benchmark and observe stack
// consumption boundaries, each probe executes in an isolated helper subprocess that configures
// the maximum allowable goroutine stack via `debug.SetMaxStack(maxStackSize)` (defaulting to 64 KiB).
// If a parser exceeds the stack limit during parsing, the Go runtime triggers a fatal error
// terminating the worker, which is safely captured by the parent test process.
//
// The test executes in two parser modes (ANTLR and Pratt) and computes the maximum successful
// recursion depth for each category using binary search, generating detailed comparative statistics.
//
// To run manually:
//
//	go test -v ./parser -run TestParserRecursionDepth
//	go test -v ./parser -run TestParserRecursionDepth --stack_size=65536 --max_depth=1000
//
// Or with Bazel:
//
//	bazel test //parser:go_default_test --test_output=all --test_filter=TestParserRecursionDepth
//	bazel test //parser:go_default_test --test_output=all --test_filter=TestParserRecursionDepth --test_arg=--stack_size=65536
//
// Flags:
//   - --stack_size: Max stack limit per goroutine in bytes (default: 65536).
//   - --max_depth:  Max recursion depth search upper bound (default: 1000).
//
// Internal worker IPC (passed automatically to helper subprocesses via environment):
//   - CEL_TEST_PARSER_RECURSION_WORKER: Set to "1" to indicate execution within a subprocess worker.
//   - CEL_TEST_PARSER_IMPL: Parser implementation to test ("Antlr" or "Pratt").
//   - CEL_TEST_PARSER_CATEGORY: Recursion structure category ("Parens", "Unary", "Binary", "List").
//   - CEL_TEST_PARSER_DEPTH: Target recursion depth for the probe attempt.
//   - CEL_TEST_PARSER_STACK_SIZE: Goroutine stack size limit passed to worker subprocess.
package parser

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"cel.dev/cel-go/common"
)

var (
	flagStackSize = flag.Int("stack_size", 64*1024, "Max goroutine stack limit in bytes")
	flagMaxDepth  = flag.Int("max_depth", 1000, "Max recursion depth search upper bound")
)

const (
	// workerEnvVar is a discriminator flag set to "1" in probe subprocesses to execute
	// TestParserRecursionHelperProcess and bypass the main TestParserRecursionDepth test runner.
	workerEnvVar = "CEL_TEST_PARSER_RECURSION_WORKER"

	// implEnvVar passes the parser implementation to test in the worker ("Antlr" or "Pratt").
	implEnvVar = "CEL_TEST_PARSER_IMPL"

	// categoryEnvVar passes the recursion structure category under test ("Parens", "Unary", "Binary", or "List").
	categoryEnvVar = "CEL_TEST_PARSER_CATEGORY"

	// depthEnvVar passes the integer recursion depth to test in the helper subprocess probe.
	depthEnvVar = "CEL_TEST_PARSER_DEPTH"

	// stackSizeEnvVar passes the goroutine stack size limit in bytes (debug.SetMaxStack) to worker subprocesses.
	stackSizeEnvVar = "CEL_TEST_PARSER_STACK_SIZE"
)

var recursionCategories = map[string]string{
	"Parens": "(((...42...)))",
	"Unary":  "! ! ! ... true",
	"Binary": "1 + (1 + (...42...))",
	"List":   "[[[...42...]]]",
}

var recursionCategoryOrder = []string{"Parens", "Unary", "Binary", "List"}

func generateRecursionExpression(category string, depth int) (string, error) {
	switch category {
	case "Parens":
		return strings.Repeat("(", depth) + "42" + strings.Repeat(")", depth), nil
	case "Unary":
		return strings.Repeat("! ", depth) + "true", nil
	case "Binary":
		return strings.Repeat("1 + (", depth) + "42" + strings.Repeat(")", depth), nil
	case "List":
		return strings.Repeat("[", depth) + "42" + strings.Repeat("]", depth), nil
	default:
		return "", fmt.Errorf("unknown recursion category: %s", category)
	}
}

// TestParserRecursionHelperProcess is a subprocess worker invoked by TestParserRecursionDepth.
// It sets the maximum goroutine stack size using debug.SetMaxStack and attempts to parse
// the generated expression at the specified depth. If a stack overflow occurs, Go runtime
// crashes the worker process with a fatal error.
func TestParserRecursionHelperProcess(t *testing.T) {
	if os.Getenv(workerEnvVar) != "1" {
		return
	}
	defer os.Exit(0)

	mode := os.Getenv(implEnvVar)
	category := os.Getenv(categoryEnvVar)
	depth, err := strconv.Atoi(os.Getenv(depthEnvVar))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid depth: %v\n", err)
		os.Exit(1)
	}
	stackSize, err := strconv.Atoi(os.Getenv(stackSizeEnvVar))
	if err != nil || stackSize <= 0 {
		stackSize = *flagStackSize
	}

	debug.SetMaxStack(stackSize)

	expr, err := generateRecursionExpression(category, depth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	opts := []Option{
		MaxRecursionDepth(-1),
		ExpressionSizeCodePointLimit(-1),
		MaxExpressionNodeCount(-1),
	}
	if strings.EqualFold(mode, "pratt") {
		opts = append(opts, EnablePrattParser(true))
	}

	p, err := NewParser(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create parser: %v\n", err)
		os.Exit(1)
	}

	src := common.NewTextSource(expr)
	_, errors := p.Parse(src)
	if errors != nil && len(errors.GetErrors()) > 0 {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", errors.ToDisplayString())
		os.Exit(1)
	}
	fmt.Printf("%s depth %d completed with status: OK\n", category, depth)
}

func probeParserDepth(mode, category string, depth, stackSize int) bool {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	cmd := exec.Command(exe, "-test.run=^TestParserRecursionHelperProcess$")
	cmd.Env = append(os.Environ(),
		workerEnvVar+"=1",
		implEnvVar+"="+mode,
		categoryEnvVar+"="+category,
		fmt.Sprintf("%s=%d", depthEnvVar, depth),
		fmt.Sprintf("%s=%d", stackSizeEnvVar, stackSize),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "completed with status: OK")
}

func findMaxRecursionDepth(mode, category string, maxLimit, stackSize int) int {
	low := 1
	high := maxLimit
	maxOK := 0

	for low <= high {
		mid := (low + high) / 2
		if probeParserDepth(mode, category, mid, stackSize) {
			maxOK = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return maxOK
}

func formatDepth(depth, maxLimit int) string {
	if depth >= maxLimit {
		return "unlimited"
	}
	if depth <= 0 {
		return "0"
	}
	return strconv.Itoa(depth)
}

func TestParserRecursionDepth(t *testing.T) {
	if os.Getenv(workerEnvVar) == "1" {
		return
	}

	stackSize := *flagStackSize
	maxLimit := *flagMaxDepth

	parsers := []string{"Antlr", "Pratt"}
	results := make(map[string]map[string]int)

	for _, parserName := range parsers {
		results[parserName] = make(map[string]int)
		overallMax := maxLimit

		t.Run(parserName, func(t *testing.T) {
			fmt.Printf("\n================ Running Tests: %s Parser (Stack: %d KB) ================\n", parserName, stackSize/1024)
			for _, cat := range recursionCategoryOrder {
				cat := cat
				example := recursionCategories[cat]
				t.Run(cat, func(t *testing.T) {
					fmt.Printf("Running test [%s / %s] (e.g. '%s')...\n", parserName, cat, example)
					maxOK := findMaxRecursionDepth(parserName, cat, maxLimit, stackSize)
					results[parserName][cat] = maxOK
					if maxOK < overallMax {
						overallMax = maxOK
					}
					fmt.Printf("  -> Max depth (OK): %s\n", formatDepth(maxOK, maxLimit))
					if maxOK <= 0 {
						t.Errorf("[%s / %s] expected maxOK > 0, got %d", parserName, cat, maxOK)
					}
				})
			}
			results[parserName]["Overall"] = overallMax
		})
	}

	// Print formatted summary and comparison statistics.
	fmt.Printf("\n================ Result Summary ================\n")
	for _, parserName := range parsers {
		fmt.Printf("\n[%s Parser]\n", parserName)
		for _, cat := range recursionCategoryOrder {
			example := recursionCategories[cat]
			maxOK := results[parserName][cat]
			if maxOK > 0 {
				fmt.Printf("[%s] (e.g. '%s'): Highest recursion depth with status OK: %s\n", cat, example, formatDepth(maxOK, maxLimit))
			} else {
				fmt.Printf("[%s] (e.g. '%s'): No successful completions recorded.\n", cat, example)
			}
		}
		fmt.Printf("Overall Highest Recursion Depth (Status OK): %s\n", formatDepth(results[parserName]["Overall"], maxLimit))
	}

	fmt.Printf("\n================ Comparison (Pratt vs ANTLR) ================\n")
	fmt.Printf("%-10s | %-12s | %-12s | %-18s\n", "Category", "ANTLR (OK)", "Pratt (OK)", "Pratt Advantage")
	fmt.Printf("-----------|--------------|--------------|--------------------\n")
	for _, cat := range recursionCategoryOrder {
		antlrMax := results["Antlr"][cat]
		prattMax := results["Pratt"][cat]
		var advantage string
		if antlrMax > 0 {
			if prattMax >= maxLimit && antlrMax < maxLimit {
				advantage = "unlimited"
			} else if antlrMax >= maxLimit && prattMax >= maxLimit {
				advantage = "1.0x"
			} else {
				advantage = fmt.Sprintf("%.1fx", float64(prattMax)/float64(antlrMax))
			}
		} else {
			advantage = "N/A"
		}
		fmt.Printf("%-10s | %-12s | %-12s | %-18s\n", cat, formatDepth(antlrMax, maxLimit), formatDepth(prattMax, maxLimit), advantage)
	}
}
