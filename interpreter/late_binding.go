package interpreter

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

const (
	errorUncheckedAst      = "cannot decorate an un-checked AST for late binding, unchecked ASTs are the result of env.Parse(...), while late binding requires ASTs produced by env.Compile(...) or env.Check(...)"
	errorOverloadMismatch  = "function overload (id: %s) has different attributes (name: %s, got: %v, want: %v)"
	errorOverloadSignature = "function overload (name: %s, id: %s) is not matched (got: %s, want: %s)"
	errorOverloadNotFound  = "unexpected: overload (id: %s) not found"
	errorUnknownCallNode   = "cannot apply late binding decoration to node (id: %d, type: %T): unsupported type"
	errorOverloadInjection = "runtime dispatch error (cause: %v)"

	unarySignature    = "unary{ func(ref.Val) ref.Val }"
	binarySignature   = "binary{ func(ref.Val, ref.Val) ref.Val }"
	functionSignature = "varargs{ func(...ref.Val) ref.Val }"
)

// UncheckedAstError returns an error implementation that notifies the
// caller that the AST is unchecked and therefore it is not possible to
// apply the late binding decorator to the resulting Inteprepretable.
func UncheckedAstError() error {
	return errors.New(errorUncheckedAst)
}

// UnknownCallNodeError returns an error implementation that notifies the
// caller that the late binding decorator has encountered an IntepreptableCall
// implementation that is not known and therefore it cannot applu late binding.
func UnknownCallNodeError(id int64, callNode InterpretableCall) error {

	return fmt.Errorf(errorUnknownCallNode, id, callNode)
}

// OverloadMismatchError returns an error implementation that contains information
// about a mismatch between the runtime supplied overload and the statically linked
// overload for the given overload identifier.
func OverloadMismatchError(overloadId string, attribute string, got any, want any) error {

	return fmt.Errorf(errorOverloadMismatch, overloadId, attribute, got, want)
}

// OverloadSignatureError returns an error implementation that contains information about
// a signature mismatch between the function overload statically configured in the environment
// and the corresponding function overload supplied at runtime during evaluation.
func OverloadSignatureError(function string, overloadId string, got string, want string) error {
	return fmt.Errorf(errorOverloadSignature, function, overloadId, got, want)
}

// ValidateOverloads ensures that if the activation contains an overload function
// its signature matches the one associated to the same overload identifier in
// the dispatcher otherwise throws an error. If the activation defines more function
// overloads, those won't be considered in the validation.
func ValidateOverloads(original Dispatcher, activation Activation) error {

	// we create a
	aggregate := NewDispatcher()
	resolveAllOverloads(aggregate, activation)

	overloads := original.OverloadIds()
	for _, overloadId := range overloads {

		refOvl, found := original.FindOverload(overloadId)
		if !found {
			return fmt.Errorf(errorOverloadNotFound, overloadId)
		}

		ovl, found := aggregate.FindOverload(overloadId)
		if found {
			// we need to make sure that the overloads are
			// matching.

			result := matchSignature(overloadId, refOvl, ovl)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

// resolveAllOverloads aggregates all function overloads defined in the
// activation into a single dispatcher so that they can be easily checked
// at once when we validate the overloads.
func resolveAllOverloads(aggregate Dispatcher, activation Activation) {

	if activation == nil {
		return
	}
	switch act := activation.(type) {
	case *mapActivation:
		return
	case *emptyActivation:
		return
	case *partActivation:
		resolveAllOverloads(aggregate, act.Activation)
	case *hierarchicalActivation:
		resolveAllOverloads(aggregate, act.child)
		resolveAllOverloads(aggregate, act.parent)
	case LateBindActivation:

		// the implementation of Overloads() is expected to be
		// recursive, therefore we don't need to look any further.
		dispatcher := act.ResolveOverloads()
		for _, ovlId := range dispatcher.OverloadIds() {
			ovl, found := dispatcher.FindOverload(ovlId)
			if found {
				// note we don't need to check an error because if there
				// is an error the overload is already defined. This may
				// happen because we nest multiple activation with late
				// binding capabilities and one may shadow another as it
				// happens for variable names. Since the activations are
				// visited in the correct order this is expected behaviour.
				aggregate.Add(ovl)
			}
		}
	default:
		// this is to cater for all other implementations
		// that we don't known about but that rightfully
		// implement the Activation interface.
		resolveAllOverloads(aggregate, act.Parent())
	}

}

// matchSignature compares the two overload definitions and returns an error
// if the overload function does not have a matching signature with the
// reference overload. The only check we can implement is over the number of
// parameters and the attributes of the overload.
//
// The impmlementation verifies the following:
//
// - if refOvl.Unary is not nil, the expectation is that ovl.Unary is not nil.
// - if refOvl.Binry is not nil, the expectation is that ovl.Binary is not nil.
// - if refOvl.Function not nil, the expectation is that ovl.Fnuction is not nil.
// - refOvl.NotStrict and ovl.NonStrict must be the same.
// - refOvl.OperandTrait and ovl.OperandTrait must be the same.
// - refOvl.Operator and ovl.Operator must be the same.
func matchSignature(overloadId string, refOvl *functions.Overload, ovl *functions.Overload) error {

	got := "<nil>"
	function := "<unknown>"

	if refOvl.Unary != nil {

		if ovl.Unary == nil {

			if ovl.Binary != nil {
				got = binarySignature

			} else if ovl.Function != nil {
				got = functionSignature
			}
			return OverloadSignatureError(function, overloadId, got, unarySignature)
		}
	} else if refOvl.Binary != nil {

		if ovl.Binary == nil {

			if ovl.Unary != nil {
				got = unarySignature
			} else if ovl.Function != nil {
				got = functionSignature
			}

			return OverloadSignatureError(function, overloadId, got, binarySignature)
		}

	} else if refOvl.Function != nil {

		if ovl.Function == nil {

			if ovl.Unary != nil {
				got = unarySignature
			} else if ovl.Binary != nil {
				got = binarySignature
			}

			return OverloadSignatureError(function, overloadId, got, functionSignature)

		}
	}

	if refOvl.NonStrict != ovl.NonStrict {

		return OverloadMismatchError(overloadId, "NonStrict", ovl.NonStrict, refOvl.NonStrict)
	}
	if refOvl.OperandTrait != ovl.OperandTrait {

		return OverloadMismatchError(overloadId, "OperandTrait", ovl.OperandTrait, refOvl.OperandTrait)
	}
	if refOvl.Operator != ovl.Operator {

		// unless we test directly matchSignature, this branch can only be reached
		// with misconfiguration that are unlikely to occur (see test cases for
		// ValidateOverloads -> ERROR_Misconfigured_Dispatcher).
		return OverloadMismatchError(overloadId, "Operator", ovl.Operator, refOvl.Operator)
	}

	return nil
}

// LateBindFlags is a bitmask that is reserved for future uses to pass parameters
// to the late binding algorithm both during the program planning phase and the
// runtime dispatch behaviour.
type LateBindFlags int

const (
	LateBindFlagsNone LateBindFlags = iota
)

// OverloadInjector defines the signature of the function that is used to create a replica
// of the given InterpretableCall configured with the new function Overload passed as the
// second argument. The contract defined by this signature requires implementations to
// create a new instance of the InterpretableCall, which is identical to the one passed as
// argument, except for the overload used for the function. If the injection is not possible
// the function will return an error.
type OverloadInjector func(InterpretableCall, *functions.Overload, LateBindFlags) (InterpretableCall, error)

// evalLateBind implements the decorator pattern for function call nodes that implement
// InterpretableCall. This type is shallow wrapper around an InterpretableCall implementation
// that during evaluation looks up the overload identifier exposed by the interpretable and
// resolves it from the activation if present. It then used the configured OverloadInjector
type evalLateBind struct {
	target         InterpretableCall
	injectOverload OverloadInjector
	flags          LateBindFlags
}

// ID implements Interpretable.ID() and returns the node identifier
// of the wrapped InterpretableCall.
func (elb *evalLateBind) ID() int64 {
	return elb.target.ID()
}

// Eval implements Interpretable.Eval(Activation) and executes the late binding
// behaviour, by looking up the overload identifier associated to the wrapped
// IntepretableCall implementation in the given Activation. If a non-nil overload
// is found, it then uses the configured OverloadInjector to create a fresh copy
// of the original InterpretableCall and reconfigures it with the new overload.
// If there is no override in the activation for the overload associated to the
// InterpretableCall implementation, the original function statically linked during
// the planner execution will be executed.
func (elb *evalLateBind) Eval(ctx Activation) ref.Val {

	ovlId := elb.target.OverloadID()
	ovl := resolveOverload(ovlId, ctx)

	var err error
	var subject Interpretable = elb.target
	if ovl != nil {

		// this creates a new instance of the original
		// node, to ensure that the original remains
		// unchanged.
		subject, err = elb.injectOverload(elb.target, ovl, elb.flags)
		if err != nil {
			return types.NewErrWithNodeID(elb.target.ID(), errorOverloadInjection, err)
		}
	}

	return subject.Eval(ctx)

}

// Function implements InterpretableCall.Function() and returns the
// resolved function name configured with the wrapped InterpretableCall.
func (elb *evalLateBind) Function() string {
	return elb.target.Function()
}

// OverloadID implements InterpretableCall.OverloadID() and returns the
// resolved overload identifier configured with the wrapped InterpretableCall.
func (elb *evalLateBind) OverloadID() string {
	return elb.target.OverloadID()
}

// Args implements InterpretableCall.Args() and returns the resolved
// array of arguments configured with the wrapped InterpretableCall.
func (elb *evalLateBind) Args() []Interpretable {
	return elb.target.Args()
}

// LateBindCallOption defines the signature of an option function
// that can be used to configure the behaviour of the late binding
// algorithm.
type LateBindCallOption func(c *lateBindConfig) *lateBindConfig

// Injector returns an option function that can be used to extend the
// behaviour of the late binding algorithm to include handing for a
// specific type of InterpretableCall, which is not part of the core
// code base and therefore unknown to the algorithm.
func Injector(t InterpretableCall, injector OverloadInjector) LateBindCallOption {

	return func(c *lateBindConfig) *lateBindConfig {

		theType := reflect.TypeOf(t)
		c.injectors[theType] = injector

		return c
	}
}

// lateBindConfig defines the configuration settings for the late
// binding behaviour as well as some runtime state that is used
// by the algorithm (i.e. cache of nodes processed)
type lateBindConfig struct {
	cache     map[int64]Interpretable
	injectors map[reflect.Type]OverloadInjector
	flags     LateBindFlags
}

// defaultInjectors is implements a LateBindCallOption that is
// used to populate the injectors map with handlers for all
// known types of function call nodes.
func defaultInjectors(c *lateBindConfig) *lateBindConfig {

	c.injectors[reflect.TypeOf(&evalZeroArity{})] = injectZeroArity
	c.injectors[reflect.TypeOf(&evalUnary{})] = injectUnary
	c.injectors[reflect.TypeOf(&evalBinary{})] = injectBinary
	c.injectors[reflect.TypeOf(&evalVarArgs{})] = injectVarArgs

	return c
}

// injectZeroArity implements an OverloadInjector for the evalZeroArity implementation
// of InterpretableCall. This implementation expects a varargs function to be defined
// by the overload in order to be substituted to the function implementation that is
// statically linked to the node during the planning phase.
func injectZeroArity(target InterpretableCall, overload *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {

	zeroArity := target.(*evalZeroArity)

	if overload.Function == nil {

		return nil, OverloadSignatureError(zeroArity.function, zeroArity.overload, "<nil>", functionSignature)
	}

	return &evalZeroArity{
		id:       zeroArity.id,
		function: zeroArity.function,
		overload: zeroArity.overload,
		impl:     overload.Function,
	}, nil
}

// injectUnary implements an OverloadInjector for the evalUnary implementation of
// InterpretableCall. This implementation expects a unary function to be defined
// by the overload in order to be substituted to the function implementation that
// is statically linked to the node during the planning phase.
func injectUnary(target InterpretableCall, overload *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {

	unary := target.(*evalUnary)

	if overload.Unary == nil {

		return nil, OverloadSignatureError(unary.function, unary.overload, "<nil>", unarySignature)
	}

	return &evalUnary{
		id:        unary.id,
		function:  unary.function,
		overload:  unary.overload,
		arg:       unary.arg,
		trait:     unary.trait,
		nonStrict: unary.nonStrict,

		impl: overload.Unary,
	}, nil
}

// injectBinary implements an OverloadInjector for the evalBinary implementation of
// InterpretableCall. This implementation expects a binary function to be defined by
// the overload in order to be substituted to the function implementation that is
// statically linked to the node during the planning phase.
func injectBinary(target InterpretableCall, overload *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {
	binary := target.(*evalBinary)

	if overload.Binary == nil {

		return nil, OverloadSignatureError(binary.function, binary.overload, "<nil>", binarySignature)
	}

	return &evalBinary{
		id:        binary.id,
		function:  binary.function,
		overload:  binary.overload,
		lhs:       binary.lhs,
		rhs:       binary.rhs,
		trait:     binary.trait,
		nonStrict: binary.nonStrict,

		impl: overload.Binary,
	}, nil
}

// injectVarArgs implements an OverloadInjector for the evalVarArgs implementation of
// InterpretableCall. This implementation expects a varargs function to be defined by
// the overload in order to be substituted to the function implementation that is
// statically linked to the node during the planning phase.
func injectVarArgs(target InterpretableCall, overload *functions.Overload, _ LateBindFlags) (InterpretableCall, error) {

	varArgs := target.(*evalVarArgs)

	if overload.Function == nil {

		return nil, OverloadSignatureError(varArgs.function, varArgs.overload, "<nil>", functionSignature)
	}

	return &evalVarArgs{
		id:        varArgs.id,
		function:  varArgs.function,
		overload:  varArgs.overload,
		args:      varArgs.args,
		trait:     varArgs.trait,
		nonStrict: varArgs.nonStrict,

		impl: overload.Function,
	}, nil
}
