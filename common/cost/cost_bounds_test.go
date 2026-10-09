// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cost_test

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/ext"
)

// Cost bound checks for the libraries that ship custom estimators and trackers.
//
// The invariant is that for any expression and any input, the cost charged at runtime falls
// within the range reported by cost estimation:
//
//	estimate.Min <= actualCost <= estimate.Max
//
// Kubernetes validates a CEL rule against the estimated Max when the rule is written and enforces
// the tracked cost when it runs, so an estimate a tracker can exceed lets an admitted policy fail
// at runtime. An estimate whose Min is above the tracked cost is unsound in the other direction,
// and makes the "this expression is always too expensive" check reject affordable expressions.
//
// Estimators and trackers are written as separate code paths per function, so the two drift apart
// easily and silently. These tests exist to make that drift a build failure.
//
// Libraries with a version option are checked at every version that ships a distinct cost path,
// because a pinned environment (Kubernetes pins ListsVersion(3) and StringsVersion(2)) must hold
// the invariant too.

// costBoundsCase is a single expression to evaluate and check.
type costBoundsCase struct {
	// name identifies the case in failure output and in a costBoundsAllowlist.
	name string

	// expr is the CEL expression to compile, estimate, and evaluate.
	expr string

	// hints maps a dotted expression path to an exact size, e.g. {"l": 3, "l.@items": 10} for a
	// three element list of ten character strings.
	//
	// Sizes are exact rather than ranged so that a violation is unambiguously a defect in the
	// cost model, rather than an artifact of a loose input bound.
	hints map[string]uint64

	// vars supplies the runtime values, whose sizes must agree with hints.
	vars map[string]any
}

// costBoundsVerdict describes how a tracked cost compares against its estimate.
type costBoundsVerdict int

const (
	costWithinEstimate costBoundsVerdict = iota
	costBelowMin
	costAboveMax
)

// String implements the fmt.Stringer interface.
func (v costBoundsVerdict) String() string {
	switch v {
	case costBelowMin:
		return "below Min"
	case costAboveMax:
		return "above Max"
	default:
		return "within estimate"
	}
}

// costBoundsResult is the outcome of checking a single case.
type costBoundsResult struct {
	name     string
	estimate cost.CostEstimate
	actual   uint64
	verdict  costBoundsVerdict
}

// violated returns true if the tracked cost fell outside the estimated range.
func (r costBoundsResult) violated() bool {
	return r.verdict != costWithinEstimate
}

// String implements the fmt.Stringer interface.
func (r costBoundsResult) String() string {
	return fmt.Sprintf("%s: estimate [%d, %d], charged %d (%v)",
		r.name, r.estimate.Min, r.estimate.Max, r.actual, r.verdict)
}

// exactSizeEstimator reports the configured size for a hinted path, and no estimate otherwise.
type exactSizeEstimator struct {
	hints map[string]uint64
}

// EstimateSize implements the cost.Estimator interface.
func (e exactSizeEstimator) EstimateSize(element cost.AstNode) *cost.SizeEstimate {
	if sz, found := e.hints[strings.Join(element.Path(), ".")]; found {
		est := cost.FixedSizeEstimate(sz)
		return &est
	}
	return nil
}

// EstimateCallCost implements the cost.Estimator interface, deferring to the library's own
// estimators, which are what these tests exist to check.
func (exactSizeEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	return nil
}

// runCostBoundsCase compiles, estimates, and evaluates a single case.
//
// An error is returned only when the case itself is malformed. Cost violations are reported
// through the result.
func runCostBoundsCase(env *cel.Env, c costBoundsCase) (costBoundsResult, error) {
	res := costBoundsResult{name: c.name}
	ast, iss := env.Compile(c.expr)
	if iss.Err() != nil {
		return res, fmt.Errorf("env.Compile(%q) failed: %w", c.expr, iss.Err())
	}
	est, err := env.EstimateCost(ast, exactSizeEstimator{hints: c.hints})
	if err != nil {
		return res, fmt.Errorf("env.EstimateCost(%q) failed: %w", c.expr, err)
	}
	res.estimate = est
	prg, err := env.Program(ast, cel.CostTracking(nil))
	if err != nil {
		return res, fmt.Errorf("env.Program(%q) failed: %w", c.expr, err)
	}
	var in any = cel.NoVars()
	if c.vars != nil {
		in = c.vars
	}
	_, det, err := prg.Eval(in)
	if err != nil {
		return res, fmt.Errorf("prg.Eval(%q) failed: %w", c.expr, err)
	}
	if det == nil || det.ActualCost() == nil {
		return res, fmt.Errorf("prg.Eval(%q) reported no cost", c.expr)
	}
	res.actual = *det.ActualCost()
	switch {
	case res.actual < est.Min:
		res.verdict = costBelowMin
	case res.actual > est.Max:
		res.verdict = costAboveMax
	}
	return res, nil
}

// costBoundsAllowlist records cases that are known to violate the invariant, keyed by case name,
// with the defect that causes it as the value.
type costBoundsAllowlist map[string]string

// verifyCostBounds runs every case and fails t for any violation that is not allowlisted.
//
// It also fails when an allowlisted case no longer violates the invariant, and when an
// allowlisted case is not among the cases run at all. Fixing a defect therefore forces its entry
// to be removed, and a renamed or deleted case cannot leave a stale entry behind. Without both
// checks the allowlist silently outlives the bugs it documents.
func verifyCostBounds(t *testing.T, env *cel.Env, cases []costBoundsCase, allowed costBoundsAllowlist) {
	t.Helper()
	unseen := make(map[string]string, len(allowed))
	for name, defect := range allowed {
		unseen[name] = defect
	}
	for _, c := range cases {
		res, err := runCostBoundsCase(env, c)
		if err != nil {
			t.Errorf("runCostBoundsCase() failed: %v", err)
			continue
		}
		_, isAllowed := allowed[res.name]
		delete(unseen, res.name)
		switch {
		case res.violated() && !isAllowed:
			t.Errorf("cost outside estimate: %v", res)
		case !res.violated() && isAllowed:
			t.Errorf("case %q no longer violates its estimate (%v) - remove it from the allowlist", res.name, res)
		}
	}
	names := make([]string, 0, len(unseen))
	for name := range unseen {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Errorf("allowlisted case %q (%s) was never run - remove it from the allowlist", name, unseen[name])
	}
}

// costBoundsVars are the runtime values for the checked expressions. Sizes are uniform so that
// they can be described exactly by costBoundsHints: a ragged input would make the estimator's Min
// legitimately exceed the cost of the cheapest actual element.
var costBoundsVars = map[string]any{
	"s":  "0123456789",
	"s2": "abc",
	"l":  []string{"aaaaa", "bbbbb", "ccccc"},
	"l1": []string{"c", "b", "a", "a", "b"},
	"li": []int64{1, 2, 3, 4},
	"ll": [][]string{{"aa", "bb"}, {"cc", "dd"}},
	"m":  map[string]string{"key1": "aaaaa", "key2": "bbbbb"},
	"d":  int64(1),
}

// costBoundsHints describes costBoundsVars exactly.
var costBoundsHints = map[string]uint64{
	"s":                10,
	"s2":               3,
	"l":                3,
	"l.@items":         5,
	"l1":               5,
	"l1.@items":        1,
	"li":               4,
	"li.@items":        1,
	"ll":               2,
	"ll.@items":        2,
	"ll.@items.@items": 2,
	"m":                2,
	"m.@keys":          4,
	"m.@values":        5,
}

// costBoundsEnv builds an environment declaring the shared variables plus the given libraries.
func costBoundsEnv(t *testing.T, opts ...cel.EnvOption) *cel.Env {
	t.Helper()
	decls := []cel.EnvOption{
		cel.Variable("s", cel.StringType),
		cel.Variable("s2", cel.StringType),
		cel.Variable("l", cel.ListType(cel.StringType)),
		cel.Variable("l1", cel.ListType(cel.StringType)),
		cel.Variable("li", cel.ListType(cel.IntType)),
		cel.Variable("ll", cel.ListType(cel.ListType(cel.StringType))),
		cel.Variable("m", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("d", cel.IntType),
	}
	env, err := cel.NewEnv(append(decls, opts...)...)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	return env
}

// costBoundsCases builds the cases for a library, labelled with the version under test so that
// failures identify the environment as well as the expression.
func costBoundsCases(label string, exprs []string) []costBoundsCase {
	cases := make([]costBoundsCase, 0, len(exprs))
	for _, expr := range exprs {
		cases = append(cases, costBoundsCase{
			name:  fmt.Sprintf("%s: %s", label, expr),
			expr:  expr,
			hints: costBoundsHints,
			vars:  costBoundsVars,
		})
	}
	return cases
}

// latestListsVersion is the highest lists version with a distinct cost path, used to label the
// unpinned environment.
const latestListsVersion = 5

// listsExprs returns the list extension calls to check. Functions are gated by the version that
// introduced them: slice at v0, flatten at v1, sort/sortBy at v2, range/reverse/distinct at v3,
// and the sets-backed predicates at v5.
func listsExprs(version uint32) []string {
	exprs := []string{
		// slice
		`l.slice(1, 3)`,
		`l.slice(0, l.size())`,
		`li.slice(1, 2)`,
		`l.slice(0, 2).exists(i, i.contains('z'))`,
		// flatten
		`ll.flatten()`,
		`ll.flatten(1)`,
		`ll.flatten(d)`,
		`ll.flatten().exists(i, i.contains('z'))`,
		`[['a'], ['b', 'c']].flatten()`,
	}
	if version >= 2 {
		exprs = append(exprs,
			`l.sort()`,
			`li.sort()`,
			`l1.sort()`,
			`['c', 'a', 'b'].sort()`,
			`l.sortBy(i, i)`,
			`l1.sortBy(i, i)`,
			`l.sortBy(i, i.size())`,
		)
	}
	if version >= 3 {
		exprs = append(exprs,
			`lists.range(4)`,
			`lists.range(2 + 2)`,
			`lists.range(d)`,
			`l.reverse()`,
			`li.reverse()`,
			`l.reverse().exists(i, i.contains('z'))`,
			`l.distinct()`,
			`l1.distinct()`,
			`li.distinct()`,
			`['a', 'bb', 'ccc'].distinct()`,
			`l.distinct().exists(i, i.contains('z'))`,
			`l1.distinct().exists(i, i.contains('z'))`,
		)
	}
	if version >= 5 {
		exprs = append(exprs,
			`l.hasOnly(l)`,
			`l.hasAny(l)`,
			`l.hasAll(l)`,
			`li.hasAll([1, 2])`,
		)
	}
	return exprs
}

// stringsExprs returns the string extension calls to check. Custom cost functions start at
// StringsVersion(5); earlier versions charge a fixed call cost on both sides.
func stringsExprs() []string {
	return []string{
		`l.join(',')`,
		`l.join()`,
		`['x', 'y'].join('--')`,
		`[s, s2].join(',')`,
		`s.split(',')`,
		`s.split('0')`,
		`''.split(',')`,
		`','.split(',')`,
		`s.split(',').exists(i, i.contains('z'))`,
		`s.substring(2)`,
		`s.substring(2, 5)`,
		`s.substring(s.size())`,
		`s.replace('0', 'zz')`,
		`s.replace('0', '')`,
		`'a'.replace('a', '')`,
		`s.replace('0', 'z', 1)`,
		`s.indexOf('3')`,
		`s.lastIndexOf('3')`,
		`s.charAt(1)`,
		`s.lowerAscii()`,
		`s.upperAscii()`,
		`s.trim()`,
		`s.reverse()`,
		`strings.quote(s)`,
	}
}

// regexExprs returns the regex extension calls to check.
func regexExprs() []string {
	return []string{
		`regex.extract(s, '[0-9]')`,
		`regex.extract(s, 'zzz')`,
		`regex.extract('', '')`,
		`regex.extract(s, '([0-9])')`,
		`regex.extractAll(s, '[0-9]')`,
		`regex.extractAll(s, 'zzz')`,
		`regex.extractAll('', '')`,
		`regex.extractAll(s, '[0-9]').exists(i, i.contains('z'))`,
		`regex.replace(s, '[0-9]', 'x')`,
		`regex.replace(s, '', 'Z')`,
		`regex.replace(s, '0', 'yy')`,
		`regex.replace(s, '0', 'yy', 1)`,
		`regex.replace('hello world hello', 'hello', 'hi')`,
	}
}

// setsExprs returns the sets extension calls to check.
func setsExprs() []string {
	return []string{
		`sets.contains(l, ['aaaaa'])`,
		`sets.contains(l, l)`,
		`sets.contains(li, [1])`,
		`sets.intersects(l, l)`,
		`sets.intersects(li, [9])`,
		`sets.equivalent(l, l)`,
		`sets.equivalent(li, li)`,
	}
}

// optionalExprs returns optional `or` / `orValue` chains to check.
//
// These short-circuit: the receiver is always evaluated, but every alternative after it is skipped
// once a present value is found. The runtime charges nothing for the dispatch itself, so the
// estimate must not add one either. Cases are chosen so that the winning link varies across the
// chain -- first, middle, last and none -- since each position exercises a different subset of the
// operands at runtime while the estimate has to cover all of them.
func optionalExprs() []string {
	return append(optionalChainExprs(), optionalConsumerExprs()...)
}

// optionalChainExprs are chains which stop at the optional itself.
func optionalChainExprs() []string {
	return []string{
		`optional.of(s).orValue(s2)`,
		`optional.none().orValue(s2)`,
		`optional.of(s).or(optional.of(s2)).orValue(s2)`,
		`optional.none().or(optional.of(s2)).orValue(s2)`,
		`optional.none().or(optional.none()).orValue(s2)`,
		`optional.of(s).or(optional.of(s2)).or(optional.of(s)).orValue(s2)`,
		`optional.none().or(optional.none()).or(optional.of(s)).orValue(s2)`,
		// Receivers whose own cost is size-dependent, so the chain's bounds are a range rather
		// than a constant.
		`regex.extract(s, '[0-9]').orValue(s2)`,
		`regex.extract(s, 'zzz').orValue(s2)`,
		`regex.extract(s, 'zzz').or(regex.extract(s, '[0-9]')).orValue(s2)`,
		`regex.extract(s, '[0-9]').or(regex.extract(s, 'zzz')).orValue(s2)`,
		`l[?0].orValue('zzz')`,
		`l[?9].orValue('zzz')`,
		`l[?9].or(l[?0]).orValue('zzz')`,
		`m[?'key1'].orValue('zzz')`,
		`m[?'nope'].orValue('zzz')`,
	}
}

// optionalConsumerExprs feed the value an optional yields into another call, which is where a size
// that does not survive the optional shows up. The chains above hold either way.
func optionalConsumerExprs() []string {
	return []string{
		`optional.of(s).orValue(s2).contains('x')`,
		`optional.ofNonZeroValue(s).orValue(s2).contains('x')`,
		`optional.of(s).value().contains('x')`,
		`optional.of(s).or(optional.of(s2)).orValue(s2).contains('x')`,
		`optional.of(s).orValue(s2).lowerAscii()`,
		`(optional.of(s).orValue(s2) + s2).size()`,
		`optional.of(l).orValue(l)[0].contains('x')`,
		`optional.of(l).orValue(l).exists(i, i.contains('z'))`,
		`l[?0].orValue('zzz').contains('x')`,
		`l[?9].orValue('zzz').contains('x')`,
		`m[?'key1'].orValue('zzz').contains('x')`,
		`regex.extract(s, '[0-9]').orValue(s2).contains('x')`,
	}
}

// TestOptionalResultSizeRevision pins what ModelVersion1 changed on the optional path.
//
// TestOptionalCostBounds cannot guard this on its own: dropping the result sizes only widens Max,
// and a Max which is too large never violates the bounds invariant. What it does is make every
// limit check below an optional useless, so the property recorded here is that the estimate stays
// bounded — and that a caller pinned to ModelVersion0 still sees the saturated estimate it
// recorded.
func TestOptionalResultSizeRevision(t *testing.T) {
	// Every expression below costs a few dozen units at most, so a Max above this is a saturated
	// bound rather than a merely loose one.
	const bounded = 1000
	libs := []cel.EnvOption{cel.OptionalTypes(), ext.Regex(), ext.Strings()}
	latest := costBoundsEnv(t, libs...)
	pinned := costBoundsEnv(t, append([]cel.EnvOption{cel.CostModelVersion(0)}, libs...)...)

	for _, expr := range optionalConsumerExprs() {
		t.Run(expr, func(t *testing.T) {
			if got := estimateCostBound(t, latest, expr); got.Max > bounded {
				t.Errorf("env.EstimateCost() = [%d, %d], wanted a Max below %d: the size did not survive the optional",
					got.Min, got.Max, bounded)
			}
			if got := estimateCostBound(t, pinned, expr); got.Max <= bounded {
				t.Errorf("at ModelVersion0 env.EstimateCost() = [%d, %d], wanted the saturated estimate the revision replaced",
					got.Min, got.Max)
			}
		})
	}
}

// estimateCostBound compiles and estimates an expression against the shared hints.
func estimateCostBound(t *testing.T, env *cel.Env, expr string) cost.CostEstimate {
	t.Helper()
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
	}
	est, err := env.EstimateCost(ast, exactSizeEstimator{hints: costBoundsHints})
	if err != nil {
		t.Fatalf("env.EstimateCost(%q) failed: %v", expr, err)
	}
	return est
}

func TestListsCostBounds(t *testing.T) {
	for _, version := range []uint32{3, 4, 5} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			label := fmt.Sprintf("lists v%d", version)
			env := costBoundsEnv(t, ext.Lists(ext.ListsVersion(version)), ext.Strings())
			verifyCostBounds(t, env, costBoundsCases(label, listsExprs(version)), nil)
		})
		t.Run(fmt.Sprintf("v%d_model_v0", version), func(t *testing.T) {
			label := fmt.Sprintf("lists v%d", version)
			env := costBoundsEnv(t, cel.CostModelVersion(0), ext.Lists(ext.ListsVersion(version)), ext.Strings())
			verifyCostBounds(t, env, costBoundsCases(label, listsExprs(version)), listsCostAllowlistV0(label, version))
		})
	}
	t.Run("latest", func(t *testing.T) {
		const label = "lists latest"
		env := costBoundsEnv(t, ext.Lists(), ext.Strings())
		verifyCostBounds(t, env, costBoundsCases(label, listsExprs(latestListsVersion)), nil)
	})
	t.Run("latest_model_v0", func(t *testing.T) {
		const label = "lists latest"
		env := costBoundsEnv(t, cel.CostModelVersion(0), ext.Lists(), ext.Strings())
		verifyCostBounds(t, env, costBoundsCases(label, listsExprs(latestListsVersion)), listsCostAllowlistV0(label, latestListsVersion))
	})
}

func TestStringsCostBounds(t *testing.T) {
	const label = "strings latest"
	env := costBoundsEnv(t, ext.Strings(), ext.Lists())
	verifyCostBounds(t, env, costBoundsCases(label, stringsExprs()), nil)
	t.Run("model_v0", func(t *testing.T) {
		envV0 := costBoundsEnv(t, cel.CostModelVersion(0), ext.Strings(), ext.Lists())
		verifyCostBounds(t, envV0, costBoundsCases(label, stringsExprs()), stringsCostAllowlistV0(label))
	})
}

func TestRegexCostBounds(t *testing.T) {
	const label = "regex latest"
	env := costBoundsEnv(t, cel.OptionalTypes(), ext.Regex(), ext.Strings())
	verifyCostBounds(t, env, costBoundsCases(label, regexExprs()), nil)
	t.Run("model_v0", func(t *testing.T) {
		envV0 := costBoundsEnv(t, cel.CostModelVersion(0), cel.OptionalTypes(), ext.Regex(), ext.Strings())
		verifyCostBounds(t, envV0, costBoundsCases(label, regexExprs()), regexCostAllowlistV0(label))
	})
}

func TestSetsCostBounds(t *testing.T) {
	env := costBoundsEnv(t, ext.Sets(), ext.Lists())
	verifyCostBounds(t, env, costBoundsCases("sets latest", setsExprs()), nil)
}

func TestOptionalCostBounds(t *testing.T) {
	env := costBoundsEnv(t, cel.OptionalTypes(), ext.Regex(), ext.Strings())
	verifyCostBounds(t, env, costBoundsCases("optional", optionalExprs()), nil)
}

// Historical cost bound violations preserved only under ModelVersion0.
// Under ModelVersion1 (the default), all allowlists are nil.
const (
	// L1: distinct/sort/sortBy charge a flat 2.1*n^2 while the v4+ estimate is
	// 2*n^2*ceil(len*0.1). For elements of 10 characters or fewer the tracker charges more than
	// the estimate allows; the v3 estimate rounds the same factor up where the tracker truncates.
	defectL1 = "L1: distinct/sort/sortBy estimator and tracker use different cost factors"

	// L3: a non-literal argument is treated as an exact length of math.MaxUint, which puts the
	// estimated Min at math.MaxUint64 for an operation that charges a handful of units.
	defectL3 = "L3: lists.range estimates Min as math.MaxUint64 for a non-literal argument"

	// L4: a variable depth is treated as an exact depth, so the estimate is a fixed value that
	// the actual single-level flatten falls below.
	defectL4 = "L4: flatten estimates a fixed cost for a variable depth"

	// L7: the v3 flatten result size is the size of the *input* list rather than the flattened
	// length, so a comprehension over the result iterates more elements than were estimated.
	// Note that correcting it raises an estimated Max at the version Kubernetes pins.
	defectL7 = "L7: lists v3 flatten result size is the input length, not the flattened length"

	// S1: the result is sized as one character per element plus separators, ignoring element
	// length entirely.
	defectS1 = "S1: join sizes the result as one character per element"

	// S2: splitting on a separator yields up to n+1 pieces, not n.
	defectS2 = "S2: split estimates at most n pieces for an n character input"

	// S3: a variable bound is treated as an exact length.
	defectS3 = "S3: substring estimates a fixed length for variable bounds"

	// S4: with no match the result is the target, which can be shorter than the replacement.
	defectS4 = "S4: replace computes the result Min from the replacement size"

	// R1: the estimate omits the call cost and rounds each factor up before multiplying.
	defectR1 = "R1: regex.extract estimate omits the call cost and over-rounds"

	// R2: a pattern that matches the empty string matches at all n+1 positions.
	defectR2 = "R2: regex.extractAll does not account for n+1 empty matches"

	// R3: the result Min is set to the target Max, and the Max misses empty matches.
	defectR3 = "R3: regex.replace result bounds are wrong in both directions"
)

// allowFor builds allowlist entries for a label, so that the keys line up with the case names.
func allowFor(label string) func(costBoundsAllowlist, string, ...string) {
	return func(allow costBoundsAllowlist, defect string, exprs ...string) {
		for _, expr := range exprs {
			allow[fmt.Sprintf("%s: %s", label, expr)] = defect
		}
	}
}

// listsCostAllowlistV0 returns the legacy violations for a lists library version under ModelVersion0.
func listsCostAllowlistV0(label string, version uint32) costBoundsAllowlist {
	allow := costBoundsAllowlist{}
	add := allowFor(label)
	add(allow, defectL3, `lists.range(2 + 2)`, `lists.range(d)`)
	add(allow, defectL4, `ll.flatten(d)`)
	if version == 3 {
		// The legacy estimator rounds the shared 2.1 factor up where the tracker truncates, so
		// every self-comparison is estimated one unit above what it charges.
		add(allow, defectL1,
			`l.sort()`,
			`l1.sort()`,
			`['c', 'a', 'b'].sort()`,
			`l.sortBy(i, i)`,
			`l1.sortBy(i, i)`,
			`l.distinct()`,
			`l1.distinct()`,
			`['a', 'bb', 'ccc'].distinct()`,
			`l.distinct().exists(i, i.contains('z'))`,
			`l1.distinct().exists(i, i.contains('z'))`,
		)
		add(allow, defectL7, `ll.flatten().exists(i, i.contains('z'))`)
		return allow
	}
	// From v4 the estimate scales by the element equality cost, which rounds to 1 for elements of
	// 10 characters or fewer, while the tracker still charges a flat 2.1 per pair.
	add(allow, defectL1, `l1.distinct()`, `l1.sort()`, `l1.sortBy(i, i)`)
	return allow
}

// stringsCostAllowlistV0 returns the legacy violations for the strings library under ModelVersion0.
func stringsCostAllowlistV0(label string) costBoundsAllowlist {
	allow := costBoundsAllowlist{}
	add := allowFor(label)
	add(allow, defectS1, `l.join(',')`, `l.join()`, `[s, s2].join(',')`)
	add(allow, defectS2, `''.split(',')`, `','.split(',')`)
	add(allow, defectS3, `s.substring(s.size())`)
	add(allow, defectS4, `'a'.replace('a', '')`)
	return allow
}

// regexCostAllowlistV0 returns the legacy violations for the regex library under ModelVersion0.
func regexCostAllowlistV0(label string) costBoundsAllowlist {
	allow := costBoundsAllowlist{}
	add := allowFor(label)
	add(allow, defectR1, `regex.extract('', '')`)
	add(allow, defectR2, `regex.extractAll('', '')`)
	add(allow, defectR3,
		`regex.replace(s, '', 'Z')`,
		`regex.replace(s, '[0-9]', 'x')`,
		`regex.replace('hello world hello', 'hello', 'hi')`,
	)
	return allow
}

// TestMapMacroTransfersKeyToListElem verifies that iterating a map into a list via .map, .filter,
// or .transformList transfers the map's key size into the resulting list's element size while
// clearing the map's key metadata from the list itself.
func TestMapMacroTransfersKeyToListElem(t *testing.T) {
	env := costBoundsEnv(t, ext.TwoVarComprehensions())
	exprs := []string{
		`m.map(k, k).exists(x, x.contains('y'))`,
		`m.map(k, k != '', k).exists(x, x.contains('y'))`,
		`m.filter(k, k != '').exists(x, x.contains('y'))`,
		`m.map(k, k)[0].contains('y')`,
		`m.transformList(k, v, k).exists(x, x.contains('y'))`,
		`m.transformList(k, v, v).exists(x, x.contains('a'))`,
	}
	verifyCostBounds(t, env, costBoundsCases("map-to-list", exprs), nil)

	bigKeys := exactSizeEstimator{hints: map[string]uint64{"m": 2, "m.@keys": 200, "m.@values": 1}}
	bigVals := exactSizeEstimator{hints: map[string]uint64{"m": 2, "m.@keys": 1, "m.@values": 200}}
	for _, expr := range []string{
		`m.map(k, k).exists(x, x.contains('y'))`,
		`m.map(k, k != '', k).exists(x, x.contains('y'))`,
		`m.filter(k, k != '').exists(x, x.contains('y'))`,
		`m.map(k, k)[0].contains('y')`,
		`m.transformList(k, v, k).exists(x, x.contains('y'))`,
	} {
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			t.Fatalf("env.Compile(%q) failed: %v", expr, iss.Err())
		}
		keyEst, err := env.EstimateCost(ast, bigKeys)
		if err != nil {
			t.Fatalf("env.EstimateCost(%q, bigKeys) failed: %v", expr, err)
		}
		valEst, err := env.EstimateCost(ast, bigVals)
		if err != nil {
			t.Fatalf("env.EstimateCost(%q, bigVals) failed: %v", expr, err)
		}
		if keyEst.Max <= valEst.Max {
			t.Errorf("%s: expected list element size to reflect map keys (bigKeys Max=%d > bigVals Max=%d)",
				expr, keyEst.Max, valEst.Max)
		}
	}
}

// TestContainerKeyElemPropagationAndClearing verifies that container Key and Elem sizes (including
// nested Elem.Elem bounds) are neither accidentally swapped/dropped nor narrowed when merged with
// unhinted containers.
func TestContainerKeyElemPropagationAndClearing(t *testing.T) {
	longA := strings.Repeat("a", 50)
	longB := strings.Repeat("b", 50)
	longC := strings.Repeat("c", 50)
	longD := strings.Repeat("d", 50)

	env, err := cel.NewEnv(
		cel.OptionalTypes(),
		cel.EnableMacroCallTracking(),
		ext.Bindings(),
		ext.TwoVarComprehensions(),
		ext.Lists(),
		ext.Strings(),
		ext.Regex(),
		cel.Variable("ll", cel.ListType(cel.ListType(cel.StringType))),
		cel.Variable("l", cel.ListType(cel.StringType)),
		cel.Variable("m", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("unhinted_l", cel.ListType(cel.StringType)),
		cel.Variable("s", cel.StringType),
		cel.Variable("cond", cel.BoolType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	vars := map[string]any{
		"ll":         [][]string{{longA, longB}, {longC, longD}},
		"l":          []string{longA, longB, longC},
		"m":          map[string]string{"k1": longA, "k2": longB},
		"unhinted_l": []string{longA, longB},
		"s":          longA + "," + longB,
		"cond":       false,
	}
	rawHints := map[string]uint64{
		"ll":               2,
		"ll.@items":        2,
		"ll.@items.@items": 50,
		"l":                3,
		"l.@items":         50,
		"m":                2,
		"m.@keys":          2,
		"m.@values":        50,
		"unhinted_l":       2, // deliberately omit unhinted_l.@items
		"s":                101,
	}

	// Expressions whose Key and Elem sizes come from immediate (1-level) hints or constructed
	// containers and must propagate with a finite Max under BOTH DefaultSizingStrategy and
	// AggregateSizingStrategy:
	boundedExprs := []string{
		// Constructed nested list [l] carries l's immediate Elem size (50) into [l].Elem.Elem:
		`[l].flatten().exists(i, i.contains('z'))`,
		`[l].flatten()[0].contains('z')`,
		`[l][0][0].contains('z')`,
		`ll[0].exists(i, i.contains('z'))`,
		`ll.exists(inner, inner.exists(i, i.contains('z')))`,
		// Combination (+, ?:):
		`(l + l).exists(i, i.contains('z'))`,
		`(l + l)[0].contains('z')`,
		`([l] + [l]).flatten().exists(i, i.contains('z'))`,
		`(cond ? l : ['hello_world'])[0].contains('z')`,
		// Binding (cel.bind):
		`cel.bind(a, l, a.exists(i, i.contains('z')))`,
		`cel.bind(a, l, a)[0].contains('z')`,
		`cel.bind(a, m, a.all(k, k.contains('k') && a[k].contains('z')))`,
		`cel.bind(a, m, a)['k1'].contains('z')`,
		`cel.bind(a, ['hello_world'], a)[0].contains('z')`,
		`cel.bind(a, [l], a.flatten()[0].contains('z'))`,
		// Single-variable comprehensions (map, filter):
		`l.map(x, x)[0].contains('z')`,
		`l.filter(x, x != '')[0].contains('z')`,
		`m.map(k, k)[0].contains('k')`,
		`m.filter(k, k != '')[0].contains('k')`,
		// Two-variable comprehensions (transformList, transformMap, transformMapEntry):
		`l.transformList(i, v, v)[0].contains('z')`,
		`m.transformList(k, v, k)[0].contains('k')`,
		`m.transformList(k, v, v)[0].contains('z')`,
		`m.transformMap(k, v, v).all(k, k.contains('k'))`,
		`m.transformMap(k, v, v)['k1'].contains('z')`,
		`m.transformMapEntry(k, v, {k: v}).all(k, k.contains('k'))`,
		`m.transformMapEntry(k, v, {k: v})['k1'].contains('z')`,
		`[1, 2].transformMap(i, v, l[0])[0].contains('z')`,
		// sortBy with int sort key must preserve string target element size (50, not 1):
		`l.sortBy(i, i.size()).exists(i, i.contains('z'))`,
		`l.sortBy(i, i.size())[0].contains('z')`,
		// distinct and slice preserve target element size:
		`l.distinct().exists(i, i.contains('z'))`,
		`l.distinct()[0].contains('z')`,
		`l.slice(0, 2).exists(i, i.contains('z'))`,
		`l.slice(0, 2)[0].contains('z')`,
		// Computed container indexing preserves element/value size:
		`['hello_world', 'foo_bar_baz'][0].contains('z')`,
		`{'k': 'hello_world'}['k'].contains('z')`,
		// String split and regex extractAll bound element sizes by target string Max:
		`s.split(',').exists(i, i.contains('z'))`,
		`regex.extractAll(s, '[a-z]+').exists(i, i.contains('z'))`,
		// Nested (2-level) size hints on variable ll ("ll.@items.@items"): both strategies resolve
		// nested container hints (DefaultSizingStrategy descends through resolveChildSize when the
		// child type is itself a container), so the leaf element size is 50 and est.Max is finite.
		`ll[0][0].contains('z')`,
		`ll.flatten().exists(i, i.contains('z'))`,
		`ll.flatten()[0].contains('z')`,
		`(ll + ll)[0][0].contains('z')`,
	}

	for _, stratCase := range []struct {
		name     string
		strategy cost.SizingStrategy
		hints    cost.Estimator
	}{
		{name: "default", strategy: cost.DefaultSizingStrategy(), hints: exactSizeEstimator{hints: rawHints}},
		{name: "aggregate", strategy: cost.AggregateSizingStrategy(), hints: testCostEstimator{hints: rawHints}},
	} {
		t.Run(stratCase.name, func(t *testing.T) {
			stratEnv, err := env.Extend(cel.CostSizingStrategy(stratCase.strategy))
			if err != nil {
				t.Fatalf("env.Extend() failed: %v", err)
			}
			evalAndEstimate := func(t *testing.T, expr string) (cost.CostEstimate, uint64) {
				t.Helper()
				ast, iss := stratEnv.Compile(expr)
				if iss.Err() != nil {
					t.Fatalf("stratEnv.Compile(%q) failed: %v", expr, iss.Err())
				}
				est, err := stratEnv.EstimateCost(ast, stratCase.hints)
				if err != nil {
					t.Fatalf("stratEnv.EstimateCost(%q) failed: %v", expr, err)
				}
				prg, err := stratEnv.Program(ast, cel.CostTracking(nil))
				if err != nil {
					t.Fatalf("stratEnv.Program(%q) failed: %v", expr, err)
				}
				_, det, err := prg.Eval(vars)
				if err != nil {
					t.Fatalf("prg.Eval(%q) failed: %v", expr, err)
				}
				if det == nil || det.ActualCost() == nil {
					t.Fatalf("det.ActualCost() == nil for %q", expr)
				}
				return est, *det.ActualCost()
			}

			for _, expr := range boundedExprs {
				t.Run(expr, func(t *testing.T) {
					est, actual := evalAndEstimate(t, expr)
					if actual < est.Min || actual > est.Max {
						t.Errorf("%s: actual cost %d outside estimated bounds [%d, %d]",
							expr, actual, est.Min, est.Max)
					}
					if est.Max >= math.MaxUint32 {
						t.Errorf("%s: expected finite Max bound from propagated Key/Elem size, got %d",
							expr, est.Max)
					}
				})
			}

			// Verify cel.@block propagation by rewriting cel.bind ASTs into cel.@block([init], result):
			for _, bindExpr := range []string{
				`cel.bind(a, l, a[0].contains('z'))`,
				`cel.bind(a, l, a).exists(i, i.contains('z'))`,
				`cel.bind(a, m, a['k1'].contains('z'))`,
				`cel.bind(a, m, a).all(k, k.contains('k'))`,
			} {
				t.Run("block:"+bindExpr, func(t *testing.T) {
					parsed, iss := stratEnv.Compile(bindExpr)
					if iss.Err() != nil {
						t.Fatalf("Compile(%q) failed: %v", bindExpr, iss.Err())
					}
					so, err := cel.NewStaticOptimizer(&bindToBlockOptimizer{})
					if err != nil {
						t.Fatalf("NewStaticOptimizer() failed: %v", err)
					}
					optEnv, err := stratEnv.Extend(cel.Variable("@index0", cel.DynType))
					if err != nil {
						t.Fatalf("stratEnv.Extend() failed: %v", err)
					}
					blockAST, iss := so.Optimize(optEnv, parsed)
					if iss.Err() != nil {
						t.Fatalf("Optimize(%q) failed: %v", bindExpr, iss.Err())
					}
					est, err := optEnv.EstimateCost(blockAST, stratCase.hints)
					if err != nil {
						t.Fatalf("EstimateCost(cel.@block) failed: %v", err)
					}
					if est.Max >= math.MaxUint32 {
						t.Errorf("%s: expected finite Max bound from cel.@block Key/Elem propagation, got %d",
							bindExpr, est.Max)
					}
				})
			}

			// Union of a small literal list with an unhinted list must widen Elem to Unknown:
			for _, expr := range []string{
				`(['a'] + unhinted_l).exists(i, i.contains('z'))`,
				`(cond ? ['a'] : unhinted_l).exists(i, i.contains('z'))`,
				`(['a'] + unhinted_l)[0].contains('z')`,
				`(cond ? ['a'] : unhinted_l)[0].contains('z')`,
			} {
				t.Run(expr, func(t *testing.T) {
					est, actual := evalAndEstimate(t, expr)
					if actual < est.Min || actual > est.Max {
						t.Errorf("%s: actual cost %d outside estimated bounds [%d, %d]",
							expr, actual, est.Min, est.Max)
					}
					if est.Max < math.MaxUint32 {
						t.Errorf("%s: expected unbounded Max when merging with unhinted list, got %d",
							expr, est.Max)
					}
				})
			}
		})
	}
}

type bindToBlockOptimizer struct{}

func (b *bindToBlockOptimizer) Optimize(ctx *cel.OptimizerContext, a *ast.AST) *ast.AST {
	fac := ast.NewExprFactory()
	var rewrite func(e ast.Expr) ast.Expr
	rewrite = func(e ast.Expr) ast.Expr {
		switch e.Kind() {
		case ast.ComprehensionKind:
			comp := e.AsComprehension()
			if comp.IterRange().Kind() == ast.ListKind && comp.IterRange().AsList().Size() == 0 && comp.AccuVar() == "a" {
				return ctx.NewCall("cel.@block",
					ctx.NewList([]ast.Expr{rewrite(comp.AccuInit())}, nil),
					rewrite(comp.Result()),
				)
			}
			return fac.NewComprehension(
				e.ID(),
				rewrite(comp.IterRange()),
				comp.IterVar(),
				comp.AccuVar(),
				rewrite(comp.AccuInit()),
				rewrite(comp.LoopCondition()),
				rewrite(comp.LoopStep()),
				rewrite(comp.Result()),
			)
		case ast.IdentKind:
			if e.AsIdent() == "a" {
				return ctx.NewIdent("@index0")
			}
			return e
		case ast.CallKind:
			c := e.AsCall()
			args := make([]ast.Expr, len(c.Args()))
			for i, arg := range c.Args() {
				args[i] = rewrite(arg)
			}
			if c.IsMemberFunction() {
				return ctx.NewMemberCall(c.FunctionName(), rewrite(c.Target()), args...)
			}
			return ctx.NewCall(c.FunctionName(), args...)
		default:
			return e
		}
	}
	return ctx.NewAST(rewrite(a.Expr()))
}
