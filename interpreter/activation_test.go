// Copyright 2018 Google LLC
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
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

func TestActivation(t *testing.T) {
	act, err := NewActivation(map[string]any{"a": types.True})
	if err != nil {
		t.Fatalf("Got err: %v, wanted activation", err)
	}
	_, err = NewActivation(act)
	if err != nil {
		t.Fatalf("Got err: %v, wanted activation", err)
	}
	act3, err := NewActivation("")
	if err == nil {
		t.Fatalf("Got %v, wanted err", act3)
	}
}

func TestActivation_Resolve(t *testing.T) {
	activation, _ := NewActivation(map[string]any{"a": types.True})
	if val, found := activation.ResolveName("a"); !found || val != types.True {
		t.Error("Activation failed to resolve 'a'")
	}
}

func TestActivation_ResolveLazy(t *testing.T) {
	var v ref.Val
	now := func() ref.Val {
		if v == nil {
			v = types.DefaultTypeAdapter.NativeToValue(time.Now().Unix())
		}
		return v
	}
	a, _ := NewActivation(map[string]any{
		"now": now,
	})
	first, _ := a.ResolveName("now")
	second, _ := a.ResolveName("now")
	if first != second {
		t.Errorf("Got different second, "+
			"expected same as first: 1:%v 2:%v", first, second)
	}
}

func TestActivation_ResolveLazyAny(t *testing.T) {
	var v any
	now := func() any {
		if v == nil {
			v = time.Now().Unix()
		}
		return v
	}
	a, _ := NewActivation(map[string]any{
		"now": now,
	})
	first, _ := a.ResolveName("now")
	second, _ := a.ResolveName("now")
	if first != second {
		t.Errorf("Got different second, "+
			"expected same as first: 1:%v 2:%v", first, second)
	}
}

func TestHierarchicalActivation(t *testing.T) {
	// compose a parent with more properties than the child
	parent, _ := NewActivation(map[string]any{
		"a": types.String("world"),
		"b": types.Int(-42),
	})
	// compose the child such that it shadows the parent
	child, _ := NewActivation(map[string]any{
		"a": types.True,
		"c": types.String("universe"),
	})
	combined := NewHierarchicalActivation(parent, child)

	// Resolve the shadowed child value.
	if val, found := combined.ResolveName("a"); !found || val != types.True {
		t.Error("Activation failed to resolve shadow value of 'a'")
	}
	// Resolve the parent only value.
	if val, found := combined.ResolveName("b"); !found || val.(types.Int) != -42 {
		t.Error("Activation failed to resolve parent value of 'b'")
	}
	// Resolve the child only value.
	if val, found := combined.ResolveName("c"); !found || val.(types.String) != "universe" {
		t.Error("Activation failed to resolve child value of 'c'")
	}
}

func TestAsPartialActivation(t *testing.T) {
	// compose a parent with more properties than the child
	parent, _ := NewPartialActivation(map[string]any{
		"a": types.String("world"),
		"b": types.Int(-42),
	}, NewAttributePattern("c"))
	// compose the child such that it shadows the parent
	child, _ := NewActivation(map[string]any{
		"d": types.String("universe"),
	})
	combined := NewHierarchicalActivation(parent, child)

	// Resolve the shadowed child value.
	if part, found := AsPartialActivation(combined); found {
		if part != parent {
			t.Errorf("AsPartialActivation() got %v, wanted %v", part, parent)
		}
	} else {
		t.Error("AsPartialActivation() failed, did not find parent partial activation")
	}
}

// TestNewLateBindingActivation verifies the implementation of NewLateBindingActivation. The
// expectation is for the constructor function to produce a LateBindActivation implementation
// (i.e. lateBindActivation) that is configured with the given parent activation and with a
// dispatcher declaring containing the specified function overloads. The function should return
// a nil implementation and an error in case of duplicate overload function definitions or a nil
// activation.
func TestNewLateBindingActivation(t *testing.T) {

	// expectActivation generates an expectation function that verifies that
	// the outcome of NewLateBindingActivation has not generated any error and
	// contains the given activation as well as the specified function overloads.
	expectActivation := func(expected Activation, overloads ...*functions.Overload) func(t *testing.T, actual LateBindActivation, err error) {

		return func(t *testing.T, actual LateBindActivation, err error) {

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if actual == nil {
				t.Errorf("expected non nil activation")
			} else {

				lateBind, ok := actual.(*lateBindActivation)
				if !ok {
					t.Errorf("unexpected activation type (got: %T, want: %T)", actual, &lateBindActivation{})
				} else {
					if lateBind.vars != expected {
						t.Errorf("unexpected wrapped activation (got: %v, want: %v)", lateBind.vars, expected)
					}
					if lateBind.dispatcher == nil {
						t.Errorf("expected non-nil dispatcher")
					}

					actualIds := lateBind.dispatcher.OverloadIds()
					if len(actualIds) != len(overloads) {
						t.Errorf("number of overloads do not match (got: %d, want: %d)", len(actualIds), len(overloads))
					} else {

						for _, expOvl := range overloads {

							actOvl, found := lateBind.dispatcher.FindOverload(expOvl.Operator)
							if !found {
								t.Errorf("expected overload (id: %s)", expOvl.Operator)
							} else {

								if expOvl != actOvl {
									t.Errorf("overload (id: %s) mismatch", expOvl.Operator)
								}
							}
						}
					}
				}
			}
		}
	}

	// expectError generates an expectation function that checks that the
	// outcome of NewLateBindActivation produces a nil activation and an
	// error that contains the specified message.
	expectError := func(msg string) func(t *testing.T, actual LateBindActivation, err error) {

		return func(t *testing.T, actual LateBindActivation, err error) {

			if actual != nil {
				t.Errorf("expected nil activation")
			}
			if err == nil {
				t.Errorf("expected non-nil error")
			} else {
				if !strings.Contains(err.Error(), msg) {
					t.Errorf("error message (value: %s) does not contain '%s'", err.Error(), msg)
				}
			}
		}
	}

	f1_string_string := unary("f1_string_string", 0, false, func(value ref.Val) ref.Val {
		return types.String("f1_string_string")
	})

	f1_string_string_string := binary("f1_string_string_string", 0, false, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return types.String("f1_string_string_string")
	})

	f1_varargs_string := function("f1_varargs_string", 0, false, func(args ...ref.Val) ref.Val {
		return types.String("f1_varargs_string")
	})

	f2_string := function("f2_string", 0, false, func(args ...ref.Val) ref.Val {
		return types.String("f2_string")
	})

	f2_string_string := &functions.Overload{
		Operator:  "f2_string_string",
		NonStrict: true,
		Unary: func(arg ref.Val) ref.Val {
			return types.String("f2_string_string")
		},
	}

	actHierarchical := &hierarchicalActivation{
		parent: &emptyActivation{},
		child: &mapActivation{
			bindings: map[string]any{},
		},
	}

	actEmpty := &emptyActivation{}

	testCases := []struct {
		name       string
		activation Activation
		overloads  []*functions.Overload
		expect     func(t *testing.T, activation LateBindActivation, err error)
	}{
		{
			name:       "OK_No_Overloads",
			activation: actEmpty,
			overloads:  nil,
			expect:     expectActivation(actEmpty),
		},
		{
			name:       "OK_Happy_Path",
			activation: actHierarchical,
			overloads: []*functions.Overload{
				f1_string_string,
				f1_string_string_string,
				f1_varargs_string,
			},
			expect: expectActivation(actHierarchical, f1_string_string, f1_string_string_string, f1_varargs_string),
		},
		{
			name:       "ERROR_Activation_Nil",
			activation: nil,
			overloads: []*functions.Overload{
				{
					Operator: "f2",
					Function: func(values ...ref.Val) ref.Val {
						return types.String("f2")
					},
					NonStrict: false,
				},
			},
			expect: expectError("cannot create a late bind activation with a nil activation"),
		},
		{
			name:       "ERROR_Duplicate_Overloads",
			activation: &mapActivation{},
			overloads: []*functions.Overload{
				f2_string_string,
				f2_string,
				f2_string_string,
			},
			expect: expectError(fmt.Sprintf("overload already exists '%s'", "f2_string_string")),
		},
	}

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {

			actual, err := NewLateBindActivation(testCase.activation, testCase.overloads...)
			testCase.expect(t, actual, err)
		})
	}
}

// TestLateBindActivation_Parent verifies the implementation of lateBindActivation.Parent(). The
// expectation is for the function to return the Activation implementation configured with the
// vars field of the activation.
func TestLateBindActivation_Parent(t *testing.T) {

	actHierarchical := &hierarchicalActivation{
		parent: &mapActivation{
			bindings: map[string]any{
				"a": 5,
				"b": 10,
			},
		},
		child: &mapActivation{
			bindings: map[string]any{
				"a": 4,
				"c": 22,
			},
		},
	}

	testCases := []struct {
		name       string
		activation func() *lateBindActivation
		expect     func(t *testing.T, actual Activation)
	}{
		// NOTE: this test is implemented for completeness but unless
		//       we have access to the private type there is no way to
		//       produce a nil parent.
		{
			name: "OK_Nil_Parent",
			activation: func() *lateBindActivation {
				return &lateBindActivation{
					vars: nil,
					dispatcher: &defaultDispatcher{
						parent:    nil,
						overloads: overloadMap{},
					},
				}
			},
			expect: func(t *testing.T, actual Activation) {

				if actual != nil {
					t.Error("expected nil parent.")
				}
			},
		},
		{
			name: "OK_Non_Nil_Parent",
			activation: func() *lateBindActivation {
				return &lateBindActivation{
					vars: actHierarchical,
					dispatcher: &defaultDispatcher{
						parent:    nil,
						overloads: overloadMap{},
					},
				}
			},
			expect: func(t *testing.T, actual Activation) {
				if actual != actHierarchical {
					t.Errorf("unexpected parent (got: %v, want: %v)", actual, actHierarchical)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			candidate := testCase.activation()
			actual := candidate.Parent()
			testCase.expect(t, actual)
		})
	}
}

// TestLateBindActivation_ResolveName verifies the implemented behaviour of
// lateBindActivation.ResolveName(string). The expectation is for the function
// to defer all the name resolution to the configured activation.
func TestLateBindActivation_ResolveName(t *testing.T) {

	activation := func() Activation {
		return &hierarchicalActivation{
			parent: &mapActivation{
				bindings: map[string]any{
					"a": 5,
					"b": 10,
				},
			},
			child: &mapActivation{
				bindings: map[string]any{
					"a": 4,
					"c": 22,
				},
			},
		}
	}

	testCases := []struct {
		name     string
		vars     Activation
		varName  string
		found    bool
		expected any
	}{
		{
			name:     "TRUE_Single_Name_Occurrence",
			vars:     activation(),
			varName:  "c",
			found:    true,
			expected: 22,
		},
		{
			name:     "TRUE_Multiple_Name_Occurrences",
			vars:     activation(),
			varName:  "a",
			found:    true,
			expected: 4,
		},
		{
			name:     "FALSE_Missing_Name",
			vars:     activation(),
			varName:  "d",
			found:    false,
			expected: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := &lateBindActivation{
				vars:       testCase.vars,
				dispatcher: NewDispatcher(),
			}

			actual, found := candidate.ResolveName(testCase.varName)
			if testCase.found != found {
				t.Errorf("found mistmatch for var (name: '%s', got: %v, want: %v", testCase.varName, found, testCase.found)
			}

			if testCase.expected != actual {
				t.Errorf("value mismatch for var (name: '%s', got: %v, want: %v)", testCase.varName, actual, testCase.expected)
			}
		})
	}
}

// TestLateBindActivation_ResolveOverload verifies the implemented behaviour of
// lateBindActivation.ResolveOverload(string). The expectation is for the function
// to resolve the overload that is mapped to the given overload identifier if this
// is present. The resolution rules are as follows:
//
//   - the overload is first searched in the dispatcher associated to the instance.
//   - if a non nil function overload is found, it is returned.
//   - if a nil overload is found, the search continues by inspecting the activation
//     bound to the instance.
//   - if the activation bound to the instance is an empty activation the search is
//     complete and nil is returned.
//   - if the activation bound to the instance is a mapActivation the search is complete
//     and nil is returned.
//   - if the activation bound to the instance is a hierarchical activation, first the
//     child is searched to determine whether there is a LateBindActivation implementation
//     in the tree that originates from the parent.
//   - if the child search returns a nil overload, the parent is searched to determine
//     whether there is a LateBindActivation implementation in the tree that originates
//     from the child.
//   - if a LateBindActivation implementation is found, the ResolveOverload(string) name
//     is invoked to repeat the search detailed in the previous step.
//
// If the activation tree is exhausted and no overload is found matching the given
// identifier, nil is returned.
func TestLateBindActivation_ResolveOverload(t *testing.T) {

	nestedActivation, overloads := prepareNestedActivation()

	testCases := []struct {
		name       string
		candidate  func() *lateBindActivation
		overloadId string
		expected   *functions.Overload
	}{
		{
			name: "TRUE_Simple_Case",
			candidate: func() *lateBindActivation {

				return &lateBindActivation{
					vars: &emptyActivation{},
					dispatcher: &defaultDispatcher{
						parent: nil,
						overloads: overloadMap{
							"f1_string":        overloads["f1_string"],
							"f1_string_string": overloads["f1_string_string"],
						},
					},
				}
			},
			overloadId: "f1_string",
			expected:   overloads["f1_string"],
		}, {
			name: "FALSE_Simple_Case",
			candidate: func() *lateBindActivation {

				return &lateBindActivation{
					vars: &emptyActivation{},
					dispatcher: &defaultDispatcher{
						parent: nil,
						overloads: overloadMap{
							"f1_string":        overloads["f1_string"],
							"f1_string_string": overloads["f1_string_string"],
						},
					},
				}
			},
			overloadId: "f1_string_string_string",
			expected:   nil,
		}, {
			name:       "FALSE_Simple_Case_With_Nil",
			overloadId: "f1_string",
			candidate: func() *lateBindActivation {

				return &lateBindActivation{
					vars: &hierarchicalActivation{
						parent: nil,
						child:  &emptyActivation{},
					},
				}
			},
			expected: nil,
		}, {
			name:       "FALSE_Simple_Case_With_Partial_Activation",
			overloadId: "f1_string",
			candidate: func() *lateBindActivation {

				return &lateBindActivation{
					vars: &hierarchicalActivation{
						parent: &partActivation{
							Activation: &emptyActivation{},
						},
						child: &emptyActivation{},
					},
				}
			},
			expected: nil,
		}, {
			name:       "TRUE_Complex_Case_With_Nesting_Top_Level",
			candidate:  nestedActivation,
			overloadId: "f1_string",
			expected:   overloads["f1_string"],
		}, {
			name:       "TRUE_Complex_Case_With_Nesting_Top_Level_Parent",
			candidate:  nestedActivation,
			overloadId: "f2_string",
			expected:   overloads["f2_string_parent"],
		}, {
			name:       "TRUE_Complex_Case_With_Nesting_Top_Level_Shadows_Vars_Parent",
			candidate:  nestedActivation,
			overloadId: "f3_string",
			expected:   overloads["f3_string"],
		}, {
			name:       "TRUE_Complex_Case_With_Nesting_Top_Level_Shadows_Vars_Child",
			candidate:  nestedActivation,
			overloadId: "f4_string",
			expected:   overloads["f4_string"],
		}, {
			name:       "TRUE_Complex_Case_With_Nesting_Vars_Child_Shadows_Parent",
			candidate:  nestedActivation,
			overloadId: "f5_string",
			expected:   overloads["f5_string_nested_child"],
		}, {
			name:       "TRUE_Complex_Case_With_Nexting_Vars_Child_Only_Find",
			candidate:  nestedActivation,
			overloadId: "f6_string",
			expected:   overloads["f6_string_nested_child"],
		}, {
			name:       "TRUE_Complex_Case_With_Nesting_Vars_Parent_Only_Find",
			candidate:  nestedActivation,
			overloadId: "f7_string",
			expected:   overloads["f7_string_nested_parent"],
		}, {
			name:       "FALSE_Complex_Case_With_Nesting_Missing",
			candidate:  nestedActivation,
			overloadId: "f8_string",
			expected:   nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			activation := testCase.candidate()
			actual := activation.ResolveOverload(testCase.overloadId)
			if actual != testCase.expected {
				t.Errorf("mismatch function for overload (id: %s, nil: %v)", testCase.overloadId, actual == nil)
			}
		})
	}

}

// TestLateBindActivation_ResolverOverloads verifies the implemented behaviour of
// latebindActivation.ResolveOverloads(). The expectation is for the function to
// generate a dispatcher that aggregates all the function overloads definition by
// following the precedence rules implemented for ResolveOverload(string) when
// duplicates are encountered.
func TestLateBindActivation_ResolveOverloads(t *testing.T) {

	nestedActivation, overloads := prepareNestedActivation()

	testCases := []struct {
		name      string
		candidate func() *lateBindActivation
		expected  Dispatcher
	}{
		{
			name: "OK_Simple_Activation_Empty",
			candidate: func() *lateBindActivation {
				return &lateBindActivation{
					vars: &mapActivation{},
					dispatcher: &defaultDispatcher{
						parent:    nil,
						overloads: overloadMap{},
					},
				}
			},
			expected: &defaultDispatcher{
				parent:    nil,
				overloads: overloadMap{},
			},
		},
		{
			name: "OK_Simple_Activation_Not_Empty",
			candidate: func() *lateBindActivation {
				return &lateBindActivation{
					vars: &mapActivation{},
					dispatcher: &defaultDispatcher{
						parent: &defaultDispatcher{
							parent: nil,
							overloads: overloadMap{
								"f2_string": overloads["f2_string_parent"],
							},
						},
						overloads: overloadMap{
							"f1_string": overloads["f1_string"],
							"f3_string": overloads["f3_string"],
						},
					},
				}
			},
			expected: &defaultDispatcher{
				parent: nil,
				overloads: overloadMap{
					"f1_string": overloads["f1_string"],
					"f2_string": overloads["f2_string_parent"],
					"f3_string": overloads["f3_string"],
				},
			},
		},
		{
			name:      "OK_Nested_Activation",
			candidate: nestedActivation,
			expected: &defaultDispatcher{
				parent: nil,
				overloads: overloadMap{
					"f1_string":               overloads["f1_string"],
					"f1_string_string":        overloads["f1_string_string"],
					"f1_string_string_string": overloads["f1_string_string_string"],
					"f2_string":               overloads["f2_string_parent"],
					"f3_string":               overloads["f3_string"],
					"f4_string":               overloads["f4_string"],
					"f5_string":               overloads["f5_string_nested_child"],
					"f6_string":               overloads["f6_string_nested_child"],
					"f7_string":               overloads["f7_string_nested_parent"],
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			activation := testCase.candidate()
			actual := activation.ResolveOverloads()

			if actual == nil {
				t.Fatal("unexpected nil reference returned by ResolveOverloads")
			}

			expectedIds := testCase.expected.OverloadIds()
			actualIds := actual.OverloadIds()

			if len(expectedIds) != len(actualIds) {
				t.Errorf("number of overloads mismatch (got: %d, want: %d)", len(actualIds), len(expectedIds))
			}

			for _, ovlId := range expectedIds {

				expectedOverload, found := testCase.expected.FindOverload(ovlId)
				if !found {
					t.Fatalf("unexpected: overload (id: %s) declared but not found", ovlId)
				}
				actualOverload, found := actual.FindOverload(ovlId)
				if !found {
					t.Errorf("overload (id: %s) not found in result", ovlId)
				}
				if actualOverload == nil {
					t.Errorf("overload (id: %s) is found, but nil", ovlId)
				}
				if expectedOverload != actualOverload {
					t.Errorf("overload (id: %s) mismatch (got: %v, want: %v)", ovlId, actualOverload, expectedOverload)
				}
			}
		})
	}
}

// prepareNestedActivation generates a map of overloads and a function that produces a
// lateBindActivation reference which holds a tree of activations with implementations
// of LateBindActivation in the tree. The resulting activation is as structured as shown
// below:
//
// lateBindActivation:
//
//	├─ vars ---> hierarchicalActivation:
//	│  ├─ parent ---> partActivation:
//	│                 └─ Activation: emptyActivation
//	│  └─ child  ---> hierarchicalActivation:
//	│                 ├─ parent ---> lateBindActivation:
//	│                 │               ├─ vars: mapActivation,
//	│                 │               └─ dispatcher: defaultDispatcher
//	│                 │                               ├─ parent: nil
//	│                 │                               └─ overloads:
//	│                 │                                   ├─ "f3_string" --> f3_string_nested_parent
//	│                 │                                   ├─ "f5_string" --> f5_string_nested_parent
//	│                 │                                   └─ "f7_string" --> f7_string_nested_parent
//	│                 └─ child ---> lateBindActivation:
//	│                                ├─ vars: mapActivation,
//	│                                └─  dispatcher: defaultDispatcher
//	│                                                 ├─ parent: nil
//	│                                                 └─ overloads
//	│                                                     ├─ "f3_string" --> f3_string_nested_child
//	│                                                     ├─ "f4_string" --> f5_string_nested_child
//	│                                                     └─ "f6_string" --> f7_string_nested_child
//	└─ dispatcher: defaultDispatcher:
//	                ├─ parent: defaultDispatcher:
//	                │           ├─ parent: nil
//	                │           └─ overloads:
//	                │               ├─ "f1_string": f1_string_parent
//	                │               └─ "f2_string": f2_string_parent
//	                └─ overloads:
//	                    ├─ "f1_string": f1_string
//	                    ├─ "f1_string_string": f1_string_string
//	                    ├─ "f1_string_string_string": f1_string_stirng_string
//	                    ├─ "f3_string": f3_string
//	                    └─ "f4_string": f4_string
func prepareNestedActivation() (func() *lateBindActivation, map[string]*functions.Overload) {

	f1_string := function("f1_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f1_string") })
	f3_string := function("f3_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f3_string") })
	f4_string := function("f4_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f4_string") })

	// this function creates an upper case version of the original string passed as
	// argument (see: TestLateBindEvalUnaryEval).
	f1_string_string := unary("f1_string_string", 0, false, func(arg ref.Val) ref.Val {
		text, _ := arg.(types.String)
		return types.String(strings.ToUpper(string(text)))
	})

	// this function composes the two strings passed as arguments in inverse order and with
	// a space in the middle (see TestLateBindEvalBinaryEval).
	f1_string_string_string := binary("f1_string_string_string", 0, false, func(lhs ref.Val, rhs ref.Val) ref.Val {

		a, _ := lhs.(types.String)
		b, _ := rhs.(types.String)

		return b.Add(types.String(" ")).(types.String).Add(a)
	})
	f1_string_parent := function("f1_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f1_string_parent") })
	f2_string_parent := function("f2_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f2_string_parent") })

	f3_string_nested_parent := function("f3_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f3_string_nested_parent") })
	f5_string_nested_parent := function("f5_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f5_string_nested_parent") })
	f7_string_nested_parent := function("f7_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f7_string_nested_parent") })

	f4_string_nested_child := function("f4_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f4_string_nested_child") })
	f5_string_nested_child := function("f5_string", 0, false, func(args ...ref.Val) ref.Val { return types.String("f5_string_nested_child") })
	f6_string_nested_child := function("f6_string", 0, false, func(args ...ref.Val) ref.Val {

		var result traits.Adder = types.String("")

		for _, arg := range args {
			text, _ := arg.(types.String)
			result = result.Add(text).(traits.Adder)
		}
		return result.(ref.Val)
	})

	overloads := map[string]*functions.Overload{
		"f1_string":               f1_string,
		"f1_string_string":        f1_string_string,
		"f1_string_string_string": f1_string_string_string,
		"f1_string_parent":        f1_string_parent,
		"f2_string_parent":        f2_string_parent,
		"f3_string":               f3_string,
		"f3_string_nested_parent": f3_string_nested_parent,
		"f4_string":               f4_string,
		"f4_string_nested_child":  f4_string_nested_child,
		"f5_string_nested_child":  f5_string_nested_child,
		"f5_string_nested_parent": f5_string_nested_parent,
		"f6_string_nested_child":  f6_string_nested_child,
		"f7_string_nested_parent": f7_string_nested_parent,
	}

	nestedActivation := func() *lateBindActivation {

		return &lateBindActivation{
			vars: &hierarchicalActivation{
				parent: &partActivation{
					Activation: &emptyActivation{},
				},
				child: &hierarchicalActivation{
					parent: &lateBindActivation{
						vars: &mapActivation{},
						dispatcher: &defaultDispatcher{
							parent: nil,
							overloads: overloadMap{
								"f3_string": f3_string_nested_parent,
								"f5_string": f5_string_nested_parent,
								"f7_string": f7_string_nested_parent,
							},
						},
					},
					child: &lateBindActivation{
						vars: &mapActivation{},
						dispatcher: &defaultDispatcher{
							parent: nil,
							overloads: overloadMap{
								"f4_string": f4_string_nested_child,
								"f5_string": f5_string_nested_child,
								"f6_string": f6_string_nested_child,
							},
						},
					},
				},
			},
			dispatcher: &defaultDispatcher{
				parent: &defaultDispatcher{
					parent: nil,
					overloads: overloadMap{
						"f1_string": f1_string_parent,
						"f2_string": f2_string_parent,
					},
				},
				overloads: overloadMap{
					"f1_string":               f1_string,
					"f1_string_string":        f1_string_string,
					"f1_string_string_string": f1_string_string_string,
					"f3_string":               f3_string,
					"f4_string":               f4_string,
				},
			},
		}
	}

	return nestedActivation, overloads

}
