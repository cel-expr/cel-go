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

package interpreter

import (
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/containers"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/parser"
)

func TestTraceRecords(t *testing.T) {
	for _, state := range []EvalState{nil, NewEvalState()} {
		if _, ok := TraceRecords(state); ok {
			t.Errorf("TraceRecords(%T) reported records for an untraced state", state)
		}
	}
	s := NewTracedEvalState().(*evalState)
	outer := s.enterIteration(0, 10, types.Int(0))
	inner := s.enterIteration(outer, 20, types.String("a"))
	s.recordValue(1, inner, types.Int(1))
	// Back-to-back observations of an expression within an iteration collapse into one record.
	s.recordValue(1, inner, types.Int(2))
	s.recordValue(1, outer, types.Int(3))
	s.SetValue(2, types.True)
	want := []TraceRecord{
		{ID: 10, Key: types.Int(0)},
		{ID: 20, Parent: outer, Key: types.String("a")},
		{ID: 1, Parent: inner, Value: types.Int(2)},
		{ID: 1, Parent: outer, Value: types.Int(3)},
		{ID: 2, Value: types.True},
	}
	if got, ok := TraceRecords(s); !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("TraceRecords() got %v, %v, wanted %v", got, ok, want)
	}
	if !want[0].IsIteration() || want[2].IsIteration() {
		t.Error("IsIteration() did not distinguish iteration records from value records")
	}
}

func TestTracedEvalStateValues(t *testing.T) {
	// The EvalState methods of a traced state report the last value of each expression, including
	// values recorded after an earlier read.
	s := NewTracedEvalState().(*evalState)
	s.recordValue(1, 0, types.Int(1))
	s.recordValue(2, 0, types.False)
	if v, found := s.Value(1); !found || v != types.Int(1) {
		t.Errorf("Value(1) got %v, %v, wanted 1", v, found)
	}
	it := s.enterIteration(0, 10, types.Int(0))
	s.recordValue(1, it, types.Int(4))
	s.SetValue(2, nil)
	if v, found := s.Value(1); !found || v != types.Int(4) {
		t.Errorf("Value(1) got %v, %v, wanted the last value 4", v, found)
	}
	if _, found := s.Value(2); found {
		t.Error("Value(2) found a cleared value")
	}
	if ids := s.IDs(); !reflect.DeepEqual(ids, []int64{1}) {
		t.Errorf("IDs() got %v, wanted [1]", ids)
	}
	if s.indexed != 0 || len(s.values) != 0 {
		t.Error("reading the state before it was indexed wrote to it")
	}
	// Records made after the state was indexed take precedence over the indexed values.
	s.index()
	s.recordValue(3, 0, types.String("x"))
	s.SetValue(1, types.Int(5))
	if v, found := s.Value(1); !found || v != types.Int(5) {
		t.Errorf("Value(1) got %v, %v, wanted the value recorded after indexing 5", v, found)
	}
	if v, found := s.Value(3); !found || v != types.String("x") {
		t.Errorf("Value(3) got %v, %v, wanted x", v, found)
	}
	if ids := s.IDs(); !reflect.DeepEqual(sortedIDs(ids), []int64{1, 3}) {
		t.Errorf("IDs() got %v, wanted [1 3]", ids)
	}
	if indexed, n := s.indexed, len(s.values); indexed != len(s.records)-2 || n != 2 {
		t.Errorf("reading the state wrote to it: indexed %d of %d records, %d values", indexed, len(s.records), n)
	}
	s.Reset()
	if records, _ := TraceRecords(s); len(records) != 0 || len(s.IDs()) != 0 {
		t.Error("Reset() did not clear the state")
	}
}

func TestEvalStateClearValue(t *testing.T) {
	s := NewEvalState()
	s.SetValue(1, types.True)
	s.SetValue(2, types.False)
	s.SetValue(1, nil)
	if _, found := s.Value(1); found {
		t.Error("Value(1) found a cleared value")
	}
	if ids := s.IDs(); !reflect.DeepEqual(ids, []int64{2}) {
		t.Errorf("IDs() got %v, wanted [2]", ids)
	}
}

func TestTracedEvalStateResume(t *testing.T) {
	// A resumed evaluation, e.g. a later ConcurrentEval pass, replaces the records of the earlier
	// pass rather than accumulating duplicate iterations.
	parsed := parseTraced(t, `[1, 2].all(x, x < 2)`)
	prg, state := planTraced(t, parsed.Expr(), parsed.SourceInfo(), IterationTracing())
	obs, ok := prg.(*ObservableInterpretable)
	if !ok {
		t.Fatalf("planned program got %T, wanted *ObservableInterpretable", prg)
	}
	frame := mustNewExecutionFrame(t, EmptyActivation())
	defer frame.Close()
	obs.ObserveExec(frame, func(any) {})
	first, _ := TraceRecords(state)
	n := len(first)
	obs.ObserveExec(frame, func(any) {})
	if second, _ := TraceRecords(state); len(second) != n {
		t.Errorf("TraceRecords() after two passes got %d records, wanted %d", len(second), n)
	}
}

func TestTracedEvalStateConcurrentReads(t *testing.T) {
	// Once evaluation completes, the records are indexed and the state may be read concurrently.
	// Run with -race to detect writes during reads.
	parsed := parseTraced(t, `[1, 2, 3].all(x, x < 3)`)
	prg, state := planTraced(t, parsed.Expr(), parsed.SourceInfo(), IterationTracing())
	prg.Eval(EmptyActivation())
	s := state.(*evalState)
	if s.indexed != len(s.records) {
		t.Fatalf("evaluation indexed %d of %d records", s.indexed, len(s.records))
	}
	rootID := parsed.Expr().ID()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, found := state.Value(rootID); !found || v != types.False {
				t.Errorf("Value(root) got %v, %v, wanted false", v, found)
			}
			if len(state.IDs()) == 0 {
				t.Error("IDs() got no ids")
			}
		}()
	}
	wg.Wait()
}

func sortedIDs(ids []int64) []int64 {
	slices.Sort(ids)
	return ids
}

// traceStepsFor returns the iteration path of each recorded value of the expression, rendered as
// `comprehension[key]` steps, in evaluation order.
func traceStepsFor(state EvalState, id int64, names map[int64]string) []string {
	records, _ := TraceRecords(state)
	var out []string
	for _, r := range records {
		if r.IsIteration() || r.ID != id {
			continue
		}
		desc := ""
		for t := r.Parent; t != 0; t = records[t-1].Parent {
			desc = fmt.Sprintf("%s[%v]", names[records[t-1].ID], records[t-1].Key) + desc
		}
		out = append(out, fmt.Sprintf("%s=%v", desc, r.Value))
	}
	return out
}

// planTraced plans the expression with a TracedEvalState observer and returns the state.
func planTraced(t *testing.T, expr ast.Expr, info *ast.SourceInfo, opts ...PlannerOption) (Interpretable, EvalState) {
	t.Helper()
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	interp := newStandardInterpreter(t, cont, reg, reg, NewAttributeFactory(cont, reg, reg))
	state := NewTracedEvalState()
	opts = append(opts, EvalStateObserver(EvalStateFactory(func() EvalState { return state })))
	prg, err := interp.NewInterpretable(ast.NewAST(expr, info), opts...)
	if err != nil {
		t.Fatalf("NewInterpretable() failed: %v", err)
	}
	return prg, state
}

func parseTraced(t *testing.T, src string) *ast.AST {
	t.Helper()
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	parsed, errs := p.Parse(common.NewTextSource(src))
	if len(errs.GetErrors()) != 0 {
		t.Fatalf("Parse(%q) failed: %v", src, errs.ToDisplayString())
	}
	return parsed
}

// comprehensionNames names each comprehension in the AST by its iteration variable.
func comprehensionNames(e ast.Expr) map[int64]string {
	names := map[int64]string{}
	ast.PostOrderVisit(e, ast.NewExprVisitor(func(x ast.Expr) {
		if x.Kind() == ast.ComprehensionKind {
			names[x.ID()] = x.AsComprehension().IterVar()
		}
	}))
	return names
}

// findCall returns the id of the first call to the function in a post-order traversal.
func findCall(e ast.Expr, fn string) int64 {
	var id int64
	ast.PostOrderVisit(e, ast.NewExprVisitor(func(x ast.Expr) {
		if id == 0 && x.Kind() == ast.CallKind && x.AsCall().FunctionName() == fn {
			id = x.ID()
		}
	}))
	return id
}

func TestIterationTracing(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		fn   string
		want []string
	}{
		{
			name: "list",
			expr: `[10, 20, 30].all(x, x < 25)`,
			fn:   operators.Less,
			want: []string{"x[0]=true", "x[1]=true", "x[2]=false"},
		},
		{
			name: "nested_lists",
			expr: `[[1, 2], [3]].all(l, l.all(x, x < 3))`,
			fn:   operators.Less,
			want: []string{"l[0]x[0]=true", "l[0]x[1]=true", "l[1]x[0]=false"},
		},
		{
			name: "map_keys",
			expr: `{'a': 1}.all(k, k != 'b')`,
			fn:   operators.NotEquals,
			want: []string{"k[a]=true"},
		},
		{
			name: "iterable_range",
			expr: `it.all(x, x < 2)`,
			vars: map[string]any{"it": &testIterable{elems: []ref.Val{types.Int(1), types.Int(2)}}},
			fn:   operators.Less,
			// Iterables without keys are keyed by ordinal.
			want: []string{"x[0]=true", "x[1]=false"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parseTraced(t, tc.expr)
			prg, state := planTraced(t, parsed.Expr(), parsed.SourceInfo(), IterationTracing())
			vars := tc.vars
			if vars == nil {
				vars = map[string]any{}
			}
			act, err := NewActivation(vars)
			if err != nil {
				t.Fatalf("NewActivation() failed: %v", err)
			}
			prg.Eval(act)
			got := traceStepsFor(state, findCall(parsed.Expr(), tc.fn), comprehensionNames(parsed.Expr()))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("traced observations got %v, wanted %v", got, tc.want)
			}
		})
	}
}

func TestIterationTracingLoopCondition(t *testing.T) {
	// An iteration which the loop condition stops before its step is evaluated is not recorded,
	// while values recorded on behalf of enclosing scopes during the condition are kept.
	fac := ast.NewExprFactory()
	tests := []struct {
		name string
		expr ast.Expr
		// iterations are the keys of the recorded iterations, in evaluation order.
		iterations []string
		// kept is the id of an expression whose value must be recorded outside of any iteration.
		kept int64
	}{
		{
			// exists() stops once its predicate holds.
			name:       "short_circuit",
			expr:       parseTraced(t, `[1, 2, 3].exists(x, x > 1)`).Expr(),
			iterations: []string{"0", "1"},
		},
		{
			// [1, 2] iterated with an accumulator initialized to `1 > 2` while the accumulator
			// holds: the lazily initialized accumulator is first evaluated by the loop condition
			// of the first iteration, which never begins.
			name: "lazy_accumulator",
			expr: fac.NewComprehension(1,
				fac.NewList(2, []ast.Expr{fac.NewLiteral(3, types.Int(1)), fac.NewLiteral(4, types.Int(2))}, nil),
				"x", "@result",
				fac.NewCall(5, operators.Greater, fac.NewLiteral(6, types.Int(1)), fac.NewLiteral(7, types.Int(2))),
				fac.NewAccuIdent(8),
				fac.NewAccuIdent(9),
				fac.NewAccuIdent(10)),
			kept: 5,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, state := planTraced(t, tc.expr, ast.NewSourceInfo(nil), IterationTracing())
			prg.Eval(EmptyActivation())
			records, _ := TraceRecords(state)
			var keys []string
			for _, r := range records {
				if r.IsIteration() {
					keys = append(keys, fmt.Sprintf("%v", r.Key))
				}
			}
			if !reflect.DeepEqual(keys, tc.iterations) {
				t.Errorf("iteration keys got %v, wanted %v", keys, tc.iterations)
			}
			if tc.kept != 0 && !slices.ContainsFunc(records, func(r TraceRecord) bool {
				return r.ID == tc.kept && r.Parent == 0
			}) {
				t.Errorf("records %v discarded the value of expression %d", records, tc.kept)
			}
		})
	}
}

func TestIterationTracingAccumulatorSnapshots(t *testing.T) {
	// Each iteration records the accumulator as it was at that iteration, rather than a reference to
	// an accumulator which is updated in place by later iterations.
	parsed := parseTraced(t, `[1, 2, 3].map(x, x * 10)`)
	prg, state := planTraced(t, parsed.Expr(), parsed.SourceInfo(), IterationTracing())
	if out := prg.Eval(EmptyActivation()); out.Equal(types.DefaultTypeAdapter.NativeToValue([]int64{10, 20, 30})) != types.True {
		t.Fatalf("Eval() got %v, wanted [10, 20, 30]", out)
	}
	got := traceStepsFor(state, findCall(parsed.Expr(), operators.Add), comprehensionNames(parsed.Expr()))
	want := []string{"x[0]=[10]", "x[1]=[10, 20]", "x[2]=[10, 20, 30]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("accumulator observations got %v, wanted %v", got, want)
	}
}

func TestIterationTracingIsOptIn(t *testing.T) {
	parsed := parseTraced(t, `[1, 2].all(x, x < 2)`)
	lessID := findCall(parsed.Expr(), operators.Less)

	// A traced state without the IterationTracing planner option records no iterations, since
	// comprehensions are only traced when requested at plan time.
	prg, state := planTraced(t, parsed.Expr(), parsed.SourceInfo())
	prg.Eval(EmptyActivation())
	records, _ := TraceRecords(state)
	for _, r := range records {
		if r.IsIteration() || r.Parent != 0 {
			t.Errorf("untraced plan recorded %+v, wanted values outside of iterations", r)
		}
	}

	// IterationTracing with a plain EvalState records the last value as before.
	cont := containers.DefaultContainer
	reg := newTestRegistry(t)
	interp := newStandardInterpreter(t, cont, reg, reg, NewAttributeFactory(cont, reg, reg))
	plain := NewEvalState()
	prg2, err := interp.NewInterpretable(parsed, IterationTracing(),
		EvalStateObserver(EvalStateFactory(func() EvalState { return plain })))
	if err != nil {
		t.Fatalf("NewInterpretable() failed: %v", err)
	}
	if out := prg2.Eval(EmptyActivation()); out != types.False {
		t.Errorf("Eval() got %v, wanted false", out)
	}
	if v, found := plain.Value(lessID); !found || v != types.False {
		t.Errorf("Value() got %v, %v, wanted the last iteration value", v, found)
	}
}

func TestIterationTracingBlockSlot(t *testing.T) {
	// cel.@block([[1, 2].map(y, y * 10)], [0, 1].all(i, @index0[i] > 5))
	//
	// The block slot is first referenced within an iteration of the all(), but the slot
	// initializer, including its own map() iterations, belongs to the block scope.
	fac := ast.NewExprFactory()
	slot := fac.NewComprehension(2,
		fac.NewList(3, []ast.Expr{fac.NewLiteral(4, types.Int(1)), fac.NewLiteral(5, types.Int(2))}, nil),
		"y", "@ac",
		fac.NewList(6, nil, nil),
		fac.NewLiteral(7, types.True),
		fac.NewCall(8, operators.Add,
			fac.NewIdent(9, "@ac"),
			fac.NewList(10, []ast.Expr{
				fac.NewCall(11, operators.Multiply, fac.NewIdent(12, "y"), fac.NewLiteral(13, types.Int(10))),
			}, nil)),
		fac.NewIdent(14, "@ac"))
	body := fac.NewComprehension(20,
		fac.NewList(21, []ast.Expr{fac.NewLiteral(22, types.Int(0)), fac.NewLiteral(23, types.Int(1))}, nil),
		"i", "@r",
		fac.NewLiteral(24, types.True),
		fac.NewCall(25, operators.NotStrictlyFalse, fac.NewIdent(26, "@r")),
		fac.NewCall(27, operators.LogicalAnd,
			fac.NewIdent(28, "@r"),
			fac.NewCall(29, operators.Greater,
				fac.NewCall(30, operators.Index, fac.NewIdent(31, "@index0"), fac.NewIdent(32, "i")),
				fac.NewLiteral(33, types.Int(5)))),
		fac.NewIdent(34, "@r"))
	block := fac.NewCall(1, "cel.@block", fac.NewList(40, []ast.Expr{slot}, nil), body)

	prg, state := planTraced(t, block, nil, IterationTracing())
	if out := prg.Eval(EmptyActivation()); out != types.True {
		t.Fatalf("Eval() got %v, wanted true", out)
	}
	names := comprehensionNames(block)
	if got, want := traceStepsFor(state, 11, names), []string{"y[0]=10", "y[1]=20"}; !reflect.DeepEqual(got, want) {
		t.Errorf("block slot observations got %v, wanted %v", got, want)
	}
	if got, want := traceStepsFor(state, 29, names), []string{"i[0]=true", "i[1]=true"}; !reflect.DeepEqual(got, want) {
		t.Errorf("block body observations got %v, wanted %v", got, want)
	}
}

// testIterable is an iterable which is neither a list nor a map, and so is folded through its
// iterator rather than as a traits.Foldable.
type testIterable struct {
	elems []ref.Val
}

var testIterableType = types.NewObjectType("test.Iterable", traits.IterableType)

func (it *testIterable) ConvertToNative(reflect.Type) (any, error) {
	return nil, fmt.Errorf("unsupported")
}
func (it *testIterable) ConvertToType(ref.Type) ref.Val { return types.NewErr("unsupported") }
func (it *testIterable) Equal(other ref.Val) ref.Val    { return types.Bool(it == other) }
func (it *testIterable) Type() ref.Type                 { return testIterableType }
func (it *testIterable) Value() any                     { return it.elems }
func (it *testIterable) Iterator() traits.Iterator {
	return &testIterator{elems: it.elems}
}

type testIterator struct {
	*testIterable
	elems []ref.Val
	next  int
}

func (it *testIterator) HasNext() ref.Val { return types.Bool(it.next < len(it.elems)) }
func (it *testIterator) Next() ref.Val {
	v := it.elems[it.next]
	it.next++
	return v
}
