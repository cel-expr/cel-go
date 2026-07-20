package interpreter

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// testErrorFunction is a simple helper to verifies the behaviour of function that
// generate error implementations.
func testErrorFunction(t *testing.T, name string, candidate func() error, expected string) {

	actualError := candidate()
	if actualError == nil {
		t.Fatalf("%s returned nil, a non-nil error implementation is expected", name)
	}
	actual := actualError.Error()
	if expected != actual {
		t.Errorf("%s has unexpected message (got: %s, want: %s)", name, actual, expected)
	}
}

// TestUncheckedAstError verifies the implemented behaviour of UncheckedAstError. The expectation
// is for the function to return a non-nil error implementation that contains the message defined
// by the constant errrorUncheckedAst.
func TestUncheckedAstError(t *testing.T) {

	testErrorFunction(t, "UncheckAstError", UncheckedAstError, errorUncheckedAst)
}

// TestUnknownCallNodeError verifies the implemented behaviour of UnknownCallNodeError.
// The expectation is for the function to return a non-nil error implementation whose
// message is configured according to the errorUnknownCallNode template.
func TestUnknownCallNodeError(t *testing.T) {
	testErrorFunction(
		t,
		"UnknownCallNodeError",
		func() error {
			return UnknownCallNodeError(45, &evalBinary{})
		},
		fmt.Sprintf(errorUnknownCallNode, 45, &evalBinary{}),
	)
}

// TestOverloadMismatchError verifies the implemented behaviour of OverloadMismatchError.
// The expectation is for the function to return an error implementation with a message
// formatted according to the errorOverloadMismatch template.
func TestOverloadMismatchError(t *testing.T) {

	testErrorFunction(
		t,
		"OverloadMismtachError",
		func() error { return OverloadMismatchError("ovlId", "Operator", "op1", "op2") },
		fmt.Sprintf(errorOverloadMismatch, "ovlId", "Operator", "op1", "op2"),
	)
}

// TestOverloadSignatureError verifies the implemented behaviour of the OverloadSignatureError.
// The expectation is for the function to return an error implementation with a message formatted
// according to the errorOverloadSignature template.
func TestOverloadSignatureError(t *testing.T) {

	testErrorFunction(
		t,
		"OverloadSignatureError",
		func() error {
			return OverloadSignatureError("f1", "ovlId", "func(ref.Val) ref.Val", "func(...ref.Val) ref.Val")
		},
		fmt.Sprintf(errorOverloadSignature, "f1", "ovlId", "func(ref.Val) ref.Val", "func(...ref.Val) ref.Val"),
	)
}

// TestValidateOverloads verifies the implemented behaviour of ValidateOverloads.
// the expectation is for the function to resolve all function overload definitions
// that are exposed by the given activation and compare them with the reference
// dispatcher maintaining the set of functions statically bound during expression
// parsing. For any function overload that is redefined in the activation the
// expectation is for the two function signatures to match and the associated
// overload attributes to be the same.
func TestValidateOverloads(t *testing.T) {

	f2_string_string := unary("f2_string_string", 0, true, func(arg ref.Val) ref.Val {
		return types.String("f2_string_string")
	})
	f2_string_string_string := binary("f2_string_string_string", 0, true, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return types.String("f2_string_string_string")
	})
	f2_varargs_string := function("f2_varargs_string", 0, true, func(args ...ref.Val) ref.Val {
		return types.String("f2_varargs_string")
	})

	f1_string_string := unary("f1_string_string", 0, true, func(arg ref.Val) ref.Val {
		return types.String("f1_string_string")
	})
	f1_string_string_string := binary("f1_string_string_string", 0, true, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return types.String("f1_string")
	})
	f1_varargs_string := function("f1_varargs_string", 0, true, func(args ...ref.Val) ref.Val {
		return types.String("f1_varargs_string")
	})

	// matching overloads

	f1_string_string_overload := unary("f1_string_string", 0, true, func(arg ref.Val) ref.Val {
		return types.String("f1_string_string_overload")
	})
	f1_string_string_string_overload := binary("f1_string_string_string", 0, true, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return types.String("f1_string_overload")
	})
	f1_varargs_string_overload := function("f1_varargs_string", 0, true, func(args ...ref.Val) ref.Val {
		return types.String("f1_varargs_string_overload")
	})

	f3_string_string := unary("f3_string_string", 0, true, func(arg ref.Val) ref.Val {
		return types.String("f3_string_string")
	})

	// mismatched overloads

	f1_string_string_binary := binary("f1_string_string", 0, true, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return types.String("f1_string_string_binary")
	})

	f1_string_string_varargs := function("f1_string_string", 0, true, func(args ...ref.Val) ref.Val {
		return types.String("f1_string_string_binary")
	})

	f1_string_string_string_unary := unary("f1_string_string_string", 0, true, func(arg ref.Val) ref.Val {
		return types.String("f1_string_string_string_unary")
	})
	f1_string_string_string_varargs := function("f1_string_string_string", 0, true, func(args ...ref.Val) ref.Val {
		return types.String("f1_string_string_string_varargs")
	})

	f1_varargs_string_unary := unary("f1_varargs_string", 0, true, func(arg ref.Val) ref.Val {
		return types.String("f1_varargs_string_unary")
	})
	f1_varargs_string_binary := binary("f1_varargs_string", 0, true, func(lhs ref.Val, rhs ref.Val) ref.Val {
		return types.String("f1_varargs_string_unary")
	})

	f1_string_operand_trait := unary("f1_string_string", 2, true, func(arg ref.Val) ref.Val {
		return types.String("f1_string_operand_trait")
	})

	f1_string_non_strict := unary("f1_string_string", 0, false, func(arg ref.Val) ref.Val {
		return types.String("f1_string_non_strict")
	})

	// referenceDispatcher generates a Dispatcher implementation that contains
	// a baseline configuration for all the function overloads that are defined.
	referenceDispatcher := func() Dispatcher {

		return &defaultDispatcher{
			parent: nil,
			overloads: overloadMap{
				"f1_string_string":        f1_string_string,
				"f1_string_string_string": f1_string_string_string,
				"f1_varargs_string":       f1_varargs_string,
				"f2_string_string":        f2_string_string,
				"f2_string_string_string": f2_string_string_string,
				"f2_varargs_string":       f2_varargs_string,
			},
		}
	}

	// candidateActivation generates a function that produces a repeatable configuration
	// for the Activation implementation based on lateBindActivation. When supplied with
	// a non-empty overload identifier, it also maps the given overload to such identifier.
	//
	// NOTE: these parameters are used to produce alterations over a baseline and produce
	//       different test cases, to ensure that all branches are explored.
	candidateActivation := func(overloadId string, overload *functions.Overload) func() Activation {

		return func() Activation {

			dispatcher := &defaultDispatcher{
				parent: nil,
				overloads: overloadMap{
					"f1_string_string":        f1_string_string_overload,
					"f1_string_string_string": f1_string_string_string_overload,
					"f1_varargs_string":       f1_varargs_string_overload,
					"f3_string_string":        f3_string_string,
				},
			}
			if len(overloadId) > 0 {
				dispatcher.overloads[overloadId] = overload
			}
			return &lateBindActivation{
				vars:       &mapActivation{},
				dispatcher: dispatcher,
			}
		}
	}

	testCases := []struct {
		name      string
		reference Dispatcher
		candidate func() Activation
		err       error
	}{
		{
			name:      "OK_Nil_Activation",
			reference: referenceDispatcher(),
			candidate: func() Activation { return nil },
			err:       nil,
		},
		{
			name:      "OK_Unknown_Activation_Type",
			reference: referenceDispatcher(),
			candidate: func() Activation { return &dummyActivation{} },
			err:       nil,
		},
		{
			name:      "OK_Matching_Overloads",
			reference: referenceDispatcher(),
			candidate: candidateActivation("", nil),
			err:       nil,
		},

		// binary function is unmatched in signature
		{
			name:      "ERROR_Unary_Mismatch_Binary",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string", f1_string_string_binary),
			err:       fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_string_string", binarySignature, unarySignature),
		}, {
			name:      "ERROR_Unary_Mismatch_VarArgs",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string", f1_string_string_varargs),
			err:       fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_string_string", functionSignature, unarySignature),
		}, {
			name:      "ERROR_Unary_Mismatch_Nil",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string", &functions.Overload{
				Operator:     "f1_string_string",
				OperandTrait: 0,
				NonStrict:    true,
			}),
			err: fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_string_string", "<nil>", unarySignature),
		},

		// binary function is unmatched in signature
		{
			name:      "ERROR_Binary_Mismatch_Unary",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string_string", f1_string_string_string_unary),
			err:       fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_string_string_string", unarySignature, binarySignature),
		}, {
			name:      "ERROR_Binary_Mismatch_VarArgs",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string_string", f1_string_string_string_varargs),
			err:       fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_string_string_string", functionSignature, binarySignature),
		}, {
			name:      "ERROR_Binary_Mismatch_Nil",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string_string", &functions.Overload{
				Operator:     "f1_string_string_string",
				OperandTrait: 0,
				NonStrict:    true,
			}),
			err: fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_string_string_string", "<nil>", binarySignature),
		},

		// varargs function is unmatched in signature
		{
			name:      "ERROR_VarArgs_Mismatch_Unary",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_varargs_string", f1_varargs_string_unary),
			err:       fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_varargs_string", unarySignature, functionSignature),
		}, {
			name:      "ERROR_VarArgs_Mismatch_Binary",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_varargs_string", f1_varargs_string_binary),
			err:       fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_varargs_string", binarySignature, functionSignature),
		}, {
			name:      "ERROR_VarArgs_Mismatch_Nil",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_varargs_string", &functions.Overload{
				Operator:     "f1_varargs_string",
				OperandTrait: 0,
				NonStrict:    true,
			}),
			err: fmt.Errorf(errorOverloadSignature, "<unknown>", "f1_varargs_string", "<nil>", functionSignature),
		},

		// unmatched attributes
		{
			name:      "ERROR_Mismatch_OperandTrait",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string", f1_string_operand_trait),
			err:       fmt.Errorf(errorOverloadMismatch, "f1_string_string", "OperandTrait", 2, 0),
		}, {
			name:      "ERROR_Mismatch_NonStrict",
			reference: referenceDispatcher(),
			candidate: candidateActivation("f1_string_string", f1_string_non_strict),
			err:       fmt.Errorf(errorOverloadMismatch, "f1_string_string", "NonStrict", false, true),
		},
		// NOTE: in this scenario a misconfiguration of the key of the original
		//       dispacher that is mapped to an overload with a different operator
		//       causes a signature error, when the same key has a match in the
		//       Activation implementation passed to ValidateOverloads
		{
			name: "ERROR_Misconfigured_Dispatcher",
			reference: &defaultDispatcher{
				overloads: overloadMap{
					"f7_string_string": f1_string_string,
				},
			},
			candidate: candidateActivation("f7_string_string", &functions.Overload{
				Operator:     "f7_string_string",
				OperandTrait: 0,
				NonStrict:    true,
				Unary: func(_ ref.Val) ref.Val {
					return types.String("")
				},
			}),
			err: fmt.Errorf(errorOverloadMismatch, "f7_string_string", "Operator", "f7_string_string", "f1_string_string"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			activation := testCase.candidate()
			err := ValidateOverloads(testCase.reference, activation)

			// if we expect an error
			if testCase.err != nil {
				if err == nil {
					// it should not be nil
					t.Errorf("outcome mismatch (got: <nil>,  want: %s)", testCase.err.Error())
				} else if testCase.err.Error() != err.Error() {

					// it should match
					t.Errorf("outcome mismatch (got: %s want: %s)", err.Error(), testCase.err.Error())
				}
			} else if err != nil {

				t.Errorf("outcome mismatch (got: %s, want: <nil>)", err.Error())
			}
		})
	}
}

type dummyActivation struct {
	Activation
}

func (da *dummyActivation) ResolveName(name string) (any, bool) {
	if da.Activation != nil {
		return da.Activation.ResolveName(name)
	}
	return nil, false
}
func (da *dummyActivation) Parent() Activation { return da.Activation }

// evalLateBindTestCase is a convenience structure used to test
// the different methods of evalLateBind.
type evalLateBindTestCase struct {
	name   string
	target InterpretableCall

	// the attributes that follow are only required for the
	// execution of tests for the Eval method.
	//
	injector   OverloadInjector
	activation Activation
	// expect is used to implement post execution assertions
	// for the Eval method. The signature could be more precise
	// but in this way we can re-use pre-built expectation
	// functions.
	expect func(*testing.T, Interpretable, ref.Val)
}

// TestEvalLateBind_ID verifies the implemented behaviour of evalLatebind.ID().
// The expectation is for the function to return the unique identifier of the
// IntepretableCall wrapped by  the structure.
func TestEvalLateBind_ID(t *testing.T) {

	testCases := testAllEvalTypes()

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {

			candidate := &evalLateBind{
				target: testCase.target,
			}
			actual := candidate.ID()
			expected := candidate.target.ID()

			if actual != expected {
				t.Errorf("ID() returned an unexpected value (got: %d, want: %d)", actual, expected)
			}
		})
	}
}

// TestEvalLateBind_Function verifies the implemented behaviour of
// evalLatebind.Function(). The expectation is for the function to
// return the name of the function that has been configured with the
// IntepretableCall wrapped by  the structure.
func TestEvalLateBind_Function(t *testing.T) {
	testCases := testAllEvalTypes()

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {

			candidate := &evalLateBind{
				target: testCase.target,
			}
			actual := candidate.Function()
			expected := candidate.target.Function()

			if actual != expected {
				t.Errorf("Function() returned an unexpected value (got: %s, want: %ss)", actual, expected)
			}
		})
	}
}

// TestEvalLateBind_OverloadID verifies the implemented behaviour of
// evalLatebind.OverloadID(). The expectation is for the function to
// return the unique identifier of the overload configured with the
// IntepretableCall wrapped by the structure.
func TestEvalLateBind_OverloadID(t *testing.T) {
	testCases := testAllEvalTypes()

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {

			candidate := &evalLateBind{
				target: testCase.target,
			}
			actual := candidate.OverloadID()
			expected := candidate.target.OverloadID()

			if actual != expected {
				t.Errorf("OverloadID() returned an unexpected value (got: %s, want: %s)", actual, expected)
			}
		})
	}
}

// TestEvalLateBind_Args verifies the implemented behaviour of
// evalLatebind.Args(). The expectation is for the function to
// return the slice of Interpretable implementation that are
// the arguments resolved for the function call configured with
// IntepretableCall wrapped by the structure.
func TestEvalLateBind_Args(t *testing.T) {
	testCases := testAllEvalTypes()

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {

			candidate := &evalLateBind{
				target: testCase.target,
			}
			actual := candidate.Args()
			expected := candidate.target.Args()

			if len(actual) != len(expected) {
				t.Errorf("Args() returned an array of different size (got: %d, want: %d)", len(actual), len(expected))
			}
			for index, e := range expected {
				a := actual[index]
				if a != e {
					t.Errorf("Args() returned an unexpected value for index '%d' (got: %v, want: %v)", index, actual, expected)
				}
			}
		})
	}
}

// TestEvalLateBind_Eval verifies the implemented behaviour of evalLateBind.Eval(Activation).
// The expectation is for the method to execute runtime dispatching of the function overload
// configured with the wrapped InterpretableCall by looking up the Activation passed to the
// function. If a matching overload is found, the implementation should replicate the wrapped
// intepretable and reconfigure it with the new overload implementation prior to executing
// the evaluation. If there is any error in the injection of the overload the function will
// return such error.
func TestEvalLateBind_Eval(t *testing.T) {

	// evalZeroCall returns an evalZeroArity reference that
	// is configured with a function that returns 0.
	evalZeroCall := func() *evalZeroArity {
		return &evalZeroArity{
			id:       50,
			function: "f1",
			overload: "f1_int",
			impl: func(_ ...ref.Val) ref.Val {
				return types.Int(0)
			},
		}
	}

	// evalUnaryCall returns an evalUnary reference that
	// is configured with a function that computes to true
	// when applied to the argument supplied.
	evalUnaryCall := func() *evalUnary {
		return &evalUnary{
			id:       51,
			function: "f2",
			overload: "f2_string_bool",
			impl: func(arg ref.Val) ref.Val {

				text := arg.(types.String)
				return text.Equal(types.String("hello"))
			},
			arg:       NewConstValue(52, types.String("hello")),
			nonStrict: false,
			trait:     0,
		}
	}

	// evalBinaryCall returns an evalBinary reference that
	// is configured with a function that computes to 15
	// when applied to the arguments supplied (13, 2).
	evalBinaryCall := func() *evalBinary {
		return &evalBinary{
			id:       53,
			function: "f3",
			overload: "f3_int_int_int",
			impl: func(lhs ref.Val, rhs ref.Val) ref.Val {

				l := lhs.(types.Int)
				r := rhs.(types.Int)
				return l + r
			},
			lhs:       NewConstValue(54, types.Int(13)),
			rhs:       NewConstValue(55, types.Int(2)),
			nonStrict: false,
			trait:     0,
		}
	}

	// evalVarArgsCall returns an evalVarArgs reference that
	// is configured with a function that computes the string
	// `this is fun` when applied to the arguments supplied
	// (`this`, `is`, `fun`).
	evalVarArgsCall := func() *evalVarArgs {

		return &evalVarArgs{

			id:       55,
			function: "f4",
			overload: "f4_string_string_string_string",
			impl: func(args ...ref.Val) ref.Val {

				parts := make([]string, len(args))
				for i, arg := range args {
					parts[i] = arg.(types.String).Value().(string)
				}

				return types.String(strings.Join(parts, " "))
			},
			args: []Interpretable{
				NewConstValue(56, types.String("this")),
				NewConstValue(56, types.String("is")),
				NewConstValue(56, types.String("fun")),
			},
			nonStrict: false,
			trait:     0,
		}
	}

	// activation creates a lateBindActivation and configures it with the
	// given activation and overload functions.
	activation := func(vars Activation, ovls ...*functions.Overload) Activation {
		d := &defaultDispatcher{
			parent:    nil,
			overloads: overloadMap{},
		}
		for _, ovl := range ovls {
			d.overloads[ovl.Operator] = ovl
		}

		return &lateBindActivation{vars, d}
	}

	// expectOriginal generates an expectation function that checks that the wrapped
	// target has remained unchanged. It does so by invoking the Eval method on an
	// empty activation and compares the result with the supplied original value.
	expectOriginal := func(original ref.Val) func(t *testing.T, target Interpretable, _ ref.Val) {

		return func(t *testing.T, target Interpretable, _ ref.Val) {

			actual := target.Eval(&emptyActivation{})
			if actual != original {
				t.Errorf("target.Eval(Activation) returned unexpected value (got: %v, want: %v)", actual, original)
			}
		}
	}

	testCases := []evalLateBindTestCase{

		// Test Case Group 0 - No Overrides
		// ---------------------------------------------
		// The expectation is that the evaluation returns
		// the same result as the result computed with the
		// function statically configured with the eval
		// struct.

		// Test Case 01 - evalZeroArity
		// ------------------------------------------------
		// result:     0
		// execution: f1_int (pre-configured)
		{
			name:       "OK_evalZeroArity_No_Overrides",
			target:     evalZeroCall(),
			injector:   injectZeroArity,
			activation: &emptyActivation{},
			expect:     expectValue(types.Int(0)),
		},

		// Test Case 02 - evalUnary
		// ------------------------------------------------
		// result:    true
		// execution: f2_string_bool (pre-configured)
		{
			name:     "OK_evalUnaryy_No_Overrides",
			target:   evalUnaryCall(),
			injector: injectUnary,
			activation: activation(
				&emptyActivation{},
				function("f2_bool", 0, false, func(_ ...ref.Val) ref.Val { return types.False }),
			),
			expect: expectValue(types.True),
		},

		// Test Case 03 - evalBinary
		// ------------------------------------------------
		// result:    15
		// execution: f3_int_int_int (pre-configured)
		{
			name:     "OK_evalBinary_No_Overrides",
			target:   evalBinaryCall(),
			injector: injectBinary,
			activation: activation(
				&emptyActivation{},
				function("f7_int_int_int", 0, false, func(_ ...ref.Val) ref.Val { return types.Int(12) }),
			),
			expect: expectValue(types.Int(15)),
		},

		// Test Case 04 - evalVarArgs
		// ------------------------------------------------
		// result:    this is fun
		// execution: f4_string_string_string_string (pre-configured)
		{
			name:     "OK_evalVarArgs_No_Overrides",
			target:   evalVarArgsCall(),
			injector: injectVarArgs,
			activation: &mapActivation{
				bindings: map[string]any{
					"a": 10,
					"f": true,
				},
			},
			expect: expectValue(types.String("this is fun")),
		},

		// Test Case Group 1 - Overrides (Hppy Path)
		// -----------------------------------------
		// The expectation is that the result of the
		// evaluation returns the same result of the
		// function that is supplied at runtime with
		// Activation implementation, but the original
		// struct remains unchanged.

		// Test Case 11 - evalZeroArity
		// ------------------------------------------------
		// result:    2
		// execution: f1_int (runtime override)
		{
			name:     "OK_evalZeroArity_With_Overrides",
			target:   evalZeroCall(),
			injector: injectZeroArity,
			activation: activation(
				&emptyActivation{},
				function("f1_int", 0, false, func(_ ...ref.Val) ref.Val { return types.Int(2) }),
			),
			expect: chain(
				// this is the result from the override
				expectValue(types.Int(2)),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.Int(0)),
			),
		},

		// Test Case 12 - evalUnary
		// ------------------------------------------------
		// result:    false
		// execution: f2_string_bool (runtime override)
		{
			name:     "OK_evalUnary_With_Overrides",
			target:   evalUnaryCall(),
			injector: injectUnary,
			activation: activation(
				&emptyActivation{},
				unary("f2_string_bool", 0, false, func(arg ref.Val) ref.Val {
					text := arg.Value().(string)
					return types.Bool(len(text) == 10)
				}),
			),
			expect: chain(
				// this is the result from the override
				expectValue(types.False),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.True),
			),
		},

		// Test Case 13 - evalBinary
		// ------------------------------------------------
		// result:    2
		// execution: f3_int_int_int (runtime override)
		{
			name:     "OK_evalBinary_With_Overrides",
			target:   evalBinaryCall(),
			injector: injectBinary,
			activation: activation(
				&emptyActivation{},
				binary("f3_int_int_int", 0, false, func(lhs ref.Val, rhs ref.Val) ref.Val { return types.Int(2) }),
			),
			expect: chain(
				// this is the result from the override
				expectValue(types.Int(2)),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.Int(15)),
			),
		},

		// Test Case 14 - evalVarArgs
		// ------------------------------------------------
		// result:    fun this is
		// execution: f4_string_string_string_string (runtime override)
		{
			name:     "OK_evalVarArgs_With_Overrides",
			target:   evalVarArgsCall(),
			injector: injectVarArgs,
			activation: activation(
				&emptyActivation{},
				function("f4_string_string_string_string", 0, false, func(args ...ref.Val) ref.Val {

					max := len(args)
					parts := make([]string, max)
					for i := max; i > 0; i-- {
						parts[max-i] = args[i-1].Value().(string)
					}
					return types.String(strings.Join(parts, " "))
				}),
			),
			expect: chain(
				// this is the result from the override
				expectValue(types.String("fun is this")),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.String("this is fun")),
			),
		},

		// Test Case Group 2 - Overrides With Errors
		// -----------------------------------------
		// The expectation is that an error is returned
		// while trying to inject the overload override
		// resolved at runtime because of signature
		// mismatch.

		// Test Case 21 - evalZeroArity
		// ------------------------------------------------
		// result:    <error, overload mismatch>
		// execution: f1_int (none)
		{
			name:     "ERROR_evalZeroArity_With_Overrides",
			target:   evalZeroCall(),
			injector: injectZeroArity,
			activation: activation(
				&emptyActivation{},
				unary("f1_int", 0, false, func(arg ref.Val) ref.Val { return types.Int(2) }),
			),
			expect: chain(
				// this is the result from the override
				expectValue(
					types.NewErrWithNodeID(
						51,
						errorOverloadInjection,
						OverloadSignatureError("f1", "f1_int", "<nil>", functionSignature),
					),
				),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.Int(0)),
			),
		},

		// Test Case 22 - evalUnary
		// ------------------------------------------------
		// result:    <error, overload mismatch>
		// execution: f2_string_bool (none)
		{
			name:     "ERROR_evalUnary_With_Overrides",
			target:   evalUnaryCall(),
			injector: injectUnary,
			activation: activation(
				&emptyActivation{},
				function("f2_string_bool", 0, false, func(args ...ref.Val) ref.Val {
					return types.False
				}),
			),
			expect: chain(
				// this is the result from the override
				expectValue(
					types.NewErrWithNodeID(
						51,
						errorOverloadInjection,
						OverloadSignatureError("f2", "f2_string_bool", "<nil>", unarySignature),
					),
				),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.True),
			),
		},

		// Test Case 23 - evalBinary
		// ------------------------------------------------
		// result:    <error, overload mismatch>
		// execution: f3_int_int_int (none)
		{
			name:     "ERROR_evalBinary_With_Overrides",
			target:   evalBinaryCall(),
			injector: injectBinary,
			activation: activation(
				&emptyActivation{},
				unary("f3_int_int_int", 0, false, func(arg ref.Val) ref.Val { return types.Int(2) }),
			),
			expect: chain(
				// this is the result from the override
				expectValue(
					types.NewErrWithNodeID(
						53,
						errorOverloadInjection,
						OverloadSignatureError("f3", "f3_int_int_int", "<nil>", binarySignature),
					),
				),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.Int(15)),
			),
		},

		// Test Case 24 - evalVarArgs
		// ------------------------------------------------
		// result:    <error, overload mismatch>
		// execution: f4_string_string_string_string (none)
		{
			name:     "ERROR_evalVarArgs_With_Overrides",
			target:   evalVarArgsCall(),
			injector: injectVarArgs,
			activation: activation(
				&emptyActivation{},
				binary("f4_string_string_string_string", 0, false, func(lhs ref.Val, rhs ref.Val) ref.Val { return types.String("") }),
			),
			expect: chain(
				// this is the result from the override
				expectValue(
					types.NewErrWithNodeID(
						55,
						errorOverloadInjection,
						OverloadSignatureError("f4", "f4_string_string_string_string", "<nil>", functionSignature),
					),
				),
				// this is the original result that we
				// should stil retain in the node.
				expectOriginal(types.String("this is fun")),
			),
		},
	}

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {

			candidate := &evalLateBind{
				target:         testCase.target,
				injectOverload: testCase.injector,
			}

			actual := candidate.Eval(testCase.activation)
			testCase.expect(t, candidate.target, actual)
		})
	}
}

// TestInjector verifies the implemented behaviour of Injector. The expectation is for the
// function to generate a LateBindCallOption that maps the supplied OverloadInjector to the
// key resolved by the type associated to the given IntepretableCall implementation.
func TestInjector(t *testing.T) {

	// control flag to ensure that the supplied method
	// is actually the one configured.
	isMatchingInjector := false

	// expectation is a function that produces an OverloadInjector that sets the above flag if
	// invoked. This is used as a control mechanism to ensure that we are actually invoking this
	// function. We are not really interested in the implementation of the injector, rather that
	// if we supply one that is the one used.
	expectation := func(match *bool) func(target InterpretableCall, overload *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {

		return func(target InterpretableCall, overload *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {

			*match = true

			return nil, nil
		}
	}

	expected := expectation(&isMatchingInjector)
	candidate := Injector(&evalUnary{}, expected)

	// check 1 - the LateBindCallOption should not be nil
	if candidate == nil {
		t.Fatal("Injector should return a non-nil LateBindCallOption")
	}

	// check 2 - the Injector when invoked should modify the
	//           injector map with a new entry (if not there
	//           already).
	config := &lateBindConfig{
		injectors: map[reflect.Type]OverloadInjector{},
	}
	modified := candidate(config)
	if len(modified.injectors) != 1 {
		t.Fatalf("Injector did not add the supplied injector, injector map is empty (got: %d, want: %d)", len(modified.injectors), 1)
	}

	// check 3 - the Injector should map the supplied injector
	//           to the key resolved by the supplied type.
	key := reflect.TypeOf(&evalUnary{})
	actual, found := modified.injectors[key]
	if !found {
		t.Fatalf("Injector did not add the supplied injector for key '%s'", key)
	}

	// check 4 - the Injector should configured he supplied injector
	//           and not a random method.
	target := &evalUnary{
		id:       30,
		arg:      NewConstValue(31, types.String("hello")),
		function: "f1",
		overload: "f1_string_int",
		impl: func(arg ref.Val) ref.Val {
			return arg.(types.String).Size()
		},
		trait:     0,
		nonStrict: false,
	}

	overload := &functions.Overload{
		Operator:     "f1_string_int",
		OperandTrait: 0,
		NonStrict:    false,
		Unary: func(arg ref.Val) ref.Val {
			return arg.(types.String).Size().(types.Int).Multiply(types.Int(2))
		},
	}

	actual(target, overload, LateBindFlagsNone)
	if !isMatchingInjector {
		t.Errorf("Injector did not configured the supplied OverloadInjector for key '%s'", key)
	}

}

// testAllEvalTypes produces an array of test cases for the
// purpose of testing evalLateBind methods across all the
// known wrapped types.
func testAllEvalTypes() []evalLateBindTestCase {

	return []evalLateBindTestCase{
		{
			name: "evalZeroArity",
			target: &evalZeroArity{
				id: 45,
				impl: func(_ ...ref.Val) ref.Val {
					return types.Int(0)
				},
				function: "f1",
				overload: "f1_int",
			},
		}, {
			name: "evalUnary",
			target: &evalUnary{
				id:  46,
				arg: NewConstValue(47, types.Int(2)),
				impl: func(_ ref.Val) ref.Val {
					return types.Int(0)
				},
				function:  "f1",
				overload:  "f1_int_int",
				nonStrict: true,
				trait:     0,
			},
		}, {
			name: "evalBinary",
			target: &evalBinary{
				id:  48,
				lhs: NewConstValue(49, types.Int(2)),
				rhs: NewConstValue(50, types.Int(5)),
				impl: func(_ ref.Val, _ ref.Val) ref.Val {
					return types.Int(3)
				},
				function:  "f1",
				overload:  "f1_int_int_int",
				nonStrict: true,
				trait:     0,
			},
		}, {
			name: "evalVarArgs",
			target: &evalVarArgs{
				id: 51,
				args: []Interpretable{
					NewConstValue(52, types.Int(2)),
					NewConstValue(53, types.Int(5)),
					NewConstValue(54, types.Int(5)),
				},
				impl: func(_ ...ref.Val) ref.Val {
					return types.Int(3)
				},
				function:  "f1",
				overload:  "f1_int_int_int_int",
				nonStrict: true,
				trait:     0,
			},
		},
	}
}

// unary is a convenience function to produce a reference to functions.Overload configured with
// the given parameters. This function sets the Overload.Unary to the given function.
func unary(operator string, operandTrait int, nonStrict bool, function functions.UnaryOp) *functions.Overload {

	return &functions.Overload{
		Operator:     operator,
		OperandTrait: operandTrait,
		NonStrict:    nonStrict,
		Unary:        function,
	}
}

// binary is a convenience function to produce a reference to functions.Overload configured with
// the given parameters. This function sets the Overload.Binary to the given function.
func binary(operator string, operandTrait int, nonStrict bool, function functions.BinaryOp) *functions.Overload {

	return &functions.Overload{
		Operator:     operator,
		OperandTrait: operandTrait,
		NonStrict:    nonStrict,
		Binary:       function,
	}
}

// function is a convenience function to produce a reference to functions.Overload configured with
// the given parameters. This function sets the Overload.Function to the given function.
func function(operator string, operandTrait int, nonStrict bool, function functions.FunctionOp) *functions.Overload {

	return &functions.Overload{
		Operator:     operator,
		OperandTrait: operandTrait,
		NonStrict:    nonStrict,
		Function:     function,
	}
}

// expectValue is a convenience function that produces an expectation function that checks
// that the value returned by the execution of Interpretable.Eval(Activation) matches the
// given expected value otherwise it fails the test.
func expectValue(expected ref.Val) func(t *testing.T, target Interpretable, actual ref.Val) {

	return func(t *testing.T, _ Interpretable, actual ref.Val) {

		t.Helper()

		if expected.Equal(actual) == types.False {
			t.Errorf("unexpected value (got: %v, want: %v)", actual, expected)
		}
	}
}

// expectType generates an expectation function that is used to validate that the name
// of Interpretable expression is the same of the name of the given type.
func expectType(expected Interpretable) func(t *testing.T, target Interpretable, actual ref.Val) {

	return func(t *testing.T, target Interpretable, _ ref.Val) {

		t.Helper()

		expectedType := reflect.TypeOf(expected).Name()
		actualType := reflect.TypeOf(target).Name()

		if expectedType != actualType {
			t.Errorf("unexpected type: (got: %s, want: %s)", actualType, expectedType)
		}
	}
}

// chain is a convenience function that can be used to run in sequence multiple expectation
// functions that are passed as argument. The function returns an expectation function that
// executes all the functions in the checks array, with the actual arguments passed to the
// function.
func chain(checks ...func(t *testing.T, target Interpretable, actual ref.Val)) func(t *testing.T, target Interpretable, actual ref.Val) {

	return func(t *testing.T, target Interpretable, actual ref.Val) {

		for _, check := range checks {
			check(t, target, actual)
		}
	}
}
