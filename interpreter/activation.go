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
	"errors"
	"fmt"

	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types/ref"
)

// Activation used to resolve identifiers by name and references by id.
//
// An Activation is the primary mechanism by which a caller supplies input into a CEL program.
type Activation interface {
	// ResolveName returns a value from the activation by qualified name, or false if the name
	// could not be found.
	ResolveName(name string) (any, bool)

	// Parent returns the parent of the current activation, may be nil.
	// If non-nil, the parent will be searched during resolve calls.
	Parent() Activation
}

// EmptyActivation returns a variable-free activation.
func EmptyActivation() Activation {
	return emptyActivation{}
}

// emptyActivation is a variable-free activation.
type emptyActivation struct{}

func (emptyActivation) ResolveName(string) (any, bool) { return nil, false }
func (emptyActivation) Parent() Activation             { return nil }

// NewActivation returns an activation based on a map-based binding where the map keys are
// expected to be qualified names used with ResolveName calls.
//
// The input `bindings` may either be of type `Activation` or `map[string]any`.
//
// Lazy bindings may be supplied within the map-based input in either of the following forms:
// - func() any
// - func() ref.Val
//
// The output of the lazy binding will overwrite the variable reference in the internal map.
//
// Values which are not represented as ref.Val types on input may be adapted to a ref.Val using
// the types.Adapter configured in the environment.
func NewActivation(bindings any) (Activation, error) {
	if bindings == nil {
		return nil, errors.New("bindings must be non-nil")
	}
	a, isActivation := bindings.(Activation)
	if isActivation {
		return a, nil
	}
	m, isMap := bindings.(map[string]any)
	if !isMap {
		return nil, fmt.Errorf(
			"activation input must be an activation or map[string]interface: got %T",
			bindings)
	}
	return &mapActivation{bindings: m}, nil
}

// mapActivation which implements Activation and maps of named values.
//
// Named bindings may lazily supply values by providing a function which accepts no arguments and
// produces an interface value.
type mapActivation struct {
	bindings map[string]any
}

// Parent implements the Activation interface method.
func (a *mapActivation) Parent() Activation {
	return nil
}

// ResolveName implements the Activation interface method.
func (a *mapActivation) ResolveName(name string) (any, bool) {
	obj, found := a.bindings[name]
	if !found {
		return nil, false
	}
	fn, isLazy := obj.(func() ref.Val)
	if isLazy {
		obj = fn()
		a.bindings[name] = obj
	}
	fnRaw, isLazy := obj.(func() any)
	if isLazy {
		obj = fnRaw()
		a.bindings[name] = obj
	}
	return obj, found
}

// hierarchicalActivation which implements Activation and contains a parent and
// child activation.
type hierarchicalActivation struct {
	parent        Activation
	child         Activation
	poolAllocated bool
}

// Parent implements the Activation interface method.
func (a *hierarchicalActivation) Parent() Activation {
	return a.parent
}

// ResolveName implements the Activation interface method.
func (a *hierarchicalActivation) ResolveName(name string) (any, bool) {
	if object, found := a.child.ResolveName(name); found {
		return object, found
	}
	return a.parent.ResolveName(name)
}

// Unwrap returns the parent activation, stripping the local child scope.
// This allows global disambiguation to skip past locally introduced variables.
func (a *hierarchicalActivation) Unwrap() Activation {
	return a.parent
}

// AsPartialActivation checks the child first via direct type assertion (to
// avoid recursion through the folder → frame → hierarchicalActivation cycle),
// then walks the parent hierarchy via the free function.
func (a *hierarchicalActivation) AsPartialActivation() (PartialActivation, bool) {
	if pv, ok := a.child.(partialActivationConverter); ok {
		if p, ok := pv.AsPartialActivation(); ok {
			return p, true
		}
	}
	return AsPartialActivation(a.parent)
}

// NewHierarchicalActivation takes two activations and produces a new one which prioritizes
// resolution in the child first and parent(s) second.
func NewHierarchicalActivation(parent Activation, child Activation) Activation {
	return &hierarchicalActivation{parent: parent, child: child, poolAllocated: false}
}

// NewPartialActivation returns an Activation which contains a list of AttributePattern values
// representing field and index operations that should result in a 'types.Unknown' result.
//
// The `bindings` value may be any value type supported by the interpreter.NewActivation call,
// but is typically either an existing Activation or map[string]any.
func NewPartialActivation(bindings any,
	unknowns ...*AttributePattern) (PartialActivation, error) {
	a, err := NewActivation(bindings)
	if err != nil {
		return nil, err
	}
	return &partActivation{Activation: a, unknowns: unknowns}, nil
}

// PartialActivation extends the Activation interface with a set of UnknownAttributePatterns.
type PartialActivation interface {
	Activation

	// UnknownAttributePaths returns a set of AttributePattern values which match Attribute
	// expressions for data accesses whose values are not yet known.
	UnknownAttributePatterns() []*AttributePattern
}

// partialActivationConverter indicates whether an Activation implementation supports conversion to a PartialActivation
type partialActivationConverter interface {
	// AsPartialActivation converts the current activation to a PartialActivation
	AsPartialActivation() (PartialActivation, bool)
}

// partActivation is the default implementations of the PartialActivation interface.
type partActivation struct {
	Activation
	unknowns []*AttributePattern
}

// UnknownAttributePatterns implements the PartialActivation interface method.
func (a *partActivation) UnknownAttributePatterns() []*AttributePattern {
	return a.unknowns
}

// AsPartialActivation returns the partActivation as a PartialActivation interface.
func (a *partActivation) AsPartialActivation() (PartialActivation, bool) {
	return a, true
}

// AsPartialActivation walks the activation hierarchy and returns the first PartialActivation, if found.
func AsPartialActivation(vars Activation) (PartialActivation, bool) {
	// Only internal activation instances may implement this interface
	if pv, ok := vars.(partialActivationConverter); ok {
		return pv.AsPartialActivation()
	}
	// Since Activations may be hierarchical, test whether a parent converts to a PartialActivation
	if vars.Parent() != nil {
		return AsPartialActivation(vars.Parent())
	}
	return nil, false
}

// NewLateBindActivation creates an activation that wraps the given activation and
// exposes the given function overloads to the evaluation. If the list of overloads
// has duplicates or the given activation is nil, it will return an error.
func NewLateBindActivation(activation Activation, overloads ...*functions.Overload) (LateBindActivation, error) {

	dispatcher := NewDispatcher()
	err := dispatcher.Add(overloads...)
	if err != nil {
		return nil, err
	}

	if activation == nil {
		return nil, errors.New("cannot create a late bind activation with a nil activation")
	}

	return &lateBindActivation{
		vars:       activation,
		dispatcher: dispatcher,
	}, nil
}

// LateBindActivation provides an interface that defines
// the contract for exposing function overloads during
// the evaluation.
//
// This interface enables the integration of external
// implementations of the late bind behaviour, without
// limiting the design to a given concrete type.
type LateBindActivation interface {
	Activation
	// ResolveOverload resolves the function overload that is
	// mapped to overloadId. Implementations of this function
	// are expected to recursively navigate the activation tree
	// by respecting the parent-child relationships to find the
	// first overload definition that is mapped to overloadId.
	ResolveOverload(overloadId string) *functions.Overload
	// ResolveOverloads returns a Dispatcher implementation that maintains all
	// the overload functions that are defined starting from the instance of the
	// concrete type implementing this method. The list is guaranteed to be
	// unique (i.e. with no duplicates). Should duplicates be found, only the
	// first occurrence of the overload is added to the list, thus ensuring
	// that the correct behaviour is being implemented.
	ResolveOverloads() Dispatcher
}

// lateBindActivation is an Activation implementation
// that carries a dispatcher which can be used to
// supply overrides for function overloads during
// evaluation.
type lateBindActivation struct {
	vars       Activation
	dispatcher Dispatcher
}

// ResolveName implemments Activation.ResolveName(string). The
// method defers the name resolution to the activation instance
// that is wrapped.
func (activation *lateBindActivation) ResolveName(name string) (any, bool) {
	return activation.vars.ResolveName(name)
}

// Parent implements Activation.Parent() and returns the
// activation that is wrapped by this struct.
func (activation *lateBindActivation) Parent() Activation {
	return activation.vars
}

// ResolveOverload resolves function overload that is mapped by
// the given overloadId. The implementation first checks if the
// dispatcher configured with the current activation defines an
// overload for overloadId, and if found it returns such overload.
// If the dispatcher does not define such overloads the function
// recursively checks the activation to find any LateBindActivation
// that might declare such overload.
func (activation *lateBindActivation) ResolveOverload(overloadId string) *functions.Overload {

	if activation.dispatcher != nil {
		ovl, found := activation.dispatcher.FindOverload(overloadId)
		if found {
			return ovl
		}
	}

	return resolveOverload(overloadId, activation.vars)
}

// ResolveOverloads returns a Dispatcher implementation that aggregates
// all function overloads definition that are accessible from the current
// activation reference. The preference is given to the overloads of the
// defined dispatcher, and then the hierarchy of activations originating
// from the configured parent activation. If there are any duplicates
func (activation *lateBindActivation) ResolveOverloads() Dispatcher {

	dispatcher := NewDispatcher()
	for _, ovlId := range activation.dispatcher.OverloadIds() {
		ovl, _ := activation.dispatcher.FindOverload(ovlId)
		dispatcher.Add(ovl)
	}

	resolveAllOverloads(dispatcher, activation.vars)

	return dispatcher
}

// resolveOverload travels the hierarchy of activations originating from the given
// Activation implementation to find the overload associatd to overloadId. Since the
// Activation APIs allow for different types of activations and compositions we need
// to ensure that if there is any valid overload that is mapped to overloadId we can
// find it.
func resolveOverload(overloadId string, activation Activation) *functions.Overload {

	if activation == nil {
		return nil
	}

	switch act := activation.(type) {
	case *mapActivation:
		return nil
	case *emptyActivation:
		return nil
	case *partActivation:
		return resolveOverload(overloadId, act.Activation)
	case *hierarchicalActivation:
		ovl := resolveOverload(overloadId, act.child)
		if ovl == nil {
			return resolveOverload(overloadId, act.parent)
		}
		return ovl
	case LateBindActivation:

		return act.ResolveOverload(overloadId)
	default:
		// this is to cater for all other implementations
		// that we don't known about but that rightfully
		// implement the Activation interface.
		return resolveOverload(overloadId, act.Parent())
	}

}
