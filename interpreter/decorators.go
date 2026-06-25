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
	"reflect"

	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// InterpretableDecorator is a functional interface for decorating or replacing
// Interpretable expression nodes at construction time.
type InterpretableDecorator func(Interpretable) (Interpretable, error)

// InterpretableDecoratorV2 is a functional interface for decorating or replacing
// InterpretableV2 expression nodes at construction time.
type InterpretableDecoratorV2 func(InterpretableV2) (InterpretableV2, error)

// decObserveEval records evaluation state into an EvalState object.
func decObserveEval(observer EvalObserver) InterpretableDecoratorV2 {
	return func(i InterpretableV2) (InterpretableV2, error) {
		switch inst := i.(type) {
		case *evalWatch, *evalWatchAttr, *evalWatchConst, *evalWatchConstructor:
			// these instruction are already watching, return straight-away.
			return i, nil
		case InterpretableAttribute:
			return &evalWatchAttr{
				InterpretableAttribute: inst,
				observer:               observer,
			}, nil
		case InterpretableConst:
			return &evalWatchConst{
				InterpretableConst: inst,
				observer:           observer,
			}, nil
		case InterpretableConstructor:
			return &evalWatchConstructor{
				constructor: inst,
				observer:    observer,
			}, nil
		default:
			return &evalWatch{
				InterpretableV2: i,
				observer:        observer,
			}, nil
		}
	}
}

// decInterruptFolds creates an intepretable decorator which marks comprehensions as interruptable
// where the interrupt state is communicated via a hidden variable on the Activation.
func decInterruptFolds() InterpretableDecoratorV2 {
	return func(i InterpretableV2) (InterpretableV2, error) {
		fold, ok := i.(*evalFold)
		if !ok {
			return i, nil
		}
		fold.interruptable = true
		return fold, nil
	}
}

// decDisableShortcircuits ensures that all branches of an expression will be evaluated, no short-circuiting.
func decDisableShortcircuits() InterpretableDecoratorV2 {
	return func(i InterpretableV2) (InterpretableV2, error) {
		switch expr := i.(type) {
		case *evalOr:
			return &evalExhaustiveOr{
				id:    expr.id,
				terms: expr.terms,
			}, nil
		case *evalAnd:
			return &evalExhaustiveAnd{
				id:    expr.id,
				terms: expr.terms,
			}, nil
		case *evalFold:
			expr.exhaustive = true
			return expr, nil
		case InterpretableAttribute:
			cond, isCond := expr.Attr().(*conditionalAttribute)
			if isCond {
				return &evalExhaustiveConditional{
					id:      cond.id,
					attr:    cond,
					adapter: expr.Adapter(),
				}, nil
			}
		}
		return i, nil
	}
}

// decOptimize optimizes the program plan by looking for common evaluation patterns and
// conditionally precomputing the result.
// - build list and map values with constant elements.
// - convert 'in' operations to set membership tests if possible.
func decOptimize() InterpretableDecoratorV2 {
	return func(i InterpretableV2) (InterpretableV2, error) {
		switch inst := i.(type) {
		case *evalList:
			return maybeBuildListLiteral(i, inst)
		case *evalMap:
			return maybeBuildMapLiteral(i, inst)
		case InterpretableCall:
			if inst.OverloadID() == overloads.InList {
				return maybeOptimizeSetMembership(i, inst)
			}
			if overloads.IsTypeConversionFunction(inst.Function()) {
				return maybeOptimizeConstUnary(i, inst)
			}
		}
		return i, nil
	}
}

// decRegexOptimizer compiles regex pattern string constants.
func decRegexOptimizer(regexOptimizations ...*RegexOptimization) InterpretableDecoratorV2 {
	functionMatchMap := make(map[string]*RegexOptimization)
	overloadMatchMap := make(map[string]*RegexOptimization)
	for _, m := range regexOptimizations {
		functionMatchMap[m.Function] = m
		if m.OverloadID != "" {
			overloadMatchMap[m.OverloadID] = m
		}
	}

	return func(i InterpretableV2) (InterpretableV2, error) {
		call, ok := i.(InterpretableCall)
		if !ok {
			return i, nil
		}

		var matcher *RegexOptimization
		var found bool
		if call.OverloadID() != "" {
			matcher, found = overloadMatchMap[call.OverloadID()]
		}
		if !found {
			matcher, found = functionMatchMap[call.Function()]
		}
		if !found || matcher.RegexIndex >= len(call.Args()) {
			return i, nil
		}
		args := call.Args()
		regexArg := args[matcher.RegexIndex]
		regexStr, isConst := regexArg.(InterpretableConst)
		if !isConst {
			return i, nil
		}
		pattern, ok := regexStr.Value().(types.String)
		if !ok {
			return i, nil
		}
		return matcher.Factory(call, string(pattern))
	}
}

func maybeOptimizeConstUnary(i InterpretableV2, call InterpretableCall) (InterpretableV2, error) {
	args := call.Args()
	if len(args) != 1 {
		return i, nil
	}
	_, isConst := args[0].(InterpretableConst)
	if !isConst {
		return i, nil
	}
	val := call.Eval(EmptyActivation())
	if types.IsError(val) {
		return nil, val.(*types.Err)
	}
	return NewConstValue(call.ID(), val), nil
}

func maybeBuildListLiteral(i InterpretableV2, l *evalList) (InterpretableV2, error) {
	for _, elem := range l.elems {
		_, isConst := elem.(InterpretableConst)
		if !isConst {
			return i, nil
		}
	}
	return NewConstValue(l.ID(), l.Eval(EmptyActivation())), nil
}

func maybeBuildMapLiteral(i InterpretableV2, mp *evalMap) (InterpretableV2, error) {
	for idx, key := range mp.keys {
		_, isConst := key.(InterpretableConst)
		if !isConst {
			return i, nil
		}
		_, isConst = mp.vals[idx].(InterpretableConst)
		if !isConst {
			return i, nil
		}
	}
	return NewConstValue(mp.ID(), mp.Eval(EmptyActivation())), nil
}

// maybeOptimizeSetMembership may convert an 'in' operation against a list to map key membership
// test if the following conditions are true:
// - the list is a constant with homogeneous element types.
// - the elements are all of primitive type.
func maybeOptimizeSetMembership(i InterpretableV2, inlist InterpretableCall) (InterpretableV2, error) {
	args := inlist.Args()
	lhs := args[0]
	rhs := args[1]
	l, isConst := rhs.(InterpretableConst)
	if !isConst {
		return i, nil
	}
	// When the incoming binary call is flagged with as the InList overload, the value will
	// always be convertible to a `traits.Lister` type.
	list := l.Value().(traits.Lister)
	if list.Size() == types.IntZero {
		return NewConstValue(inlist.ID(), types.False), nil
	}
	it := list.Iterator()
	valueSet := make(map[ref.Val]ref.Val)
	for it.HasNext() == types.True {
		elem := it.Next()
		if !types.IsPrimitiveType(elem) || elem.Type() == types.BytesType {
			// Note, non-primitive type are not yet supported, and []byte isn't hashable.
			return i, nil
		}
		valueSet[elem] = types.True
		switch ev := elem.(type) {
		case types.Double:
			iv := ev.ConvertToType(types.IntType)
			// Ensure that only lossless conversions are added to the set
			if !types.IsError(iv) && iv.Equal(ev) == types.True {
				valueSet[iv] = types.True
			}
			// Ensure that only lossless conversions are added to the set
			uv := ev.ConvertToType(types.UintType)
			if !types.IsError(uv) && uv.Equal(ev) == types.True {
				valueSet[uv] = types.True
			}
		case types.Int:
			dv := ev.ConvertToType(types.DoubleType)
			if !types.IsError(dv) {
				valueSet[dv] = types.True
			}
			uv := ev.ConvertToType(types.UintType)
			if !types.IsError(uv) {
				valueSet[uv] = types.True
			}
		case types.Uint:
			dv := ev.ConvertToType(types.DoubleType)
			if !types.IsError(dv) {
				valueSet[dv] = types.True
			}
			iv := ev.ConvertToType(types.IntType)
			if !types.IsError(iv) {
				valueSet[iv] = types.True
			}
		}
	}
	return &evalSetMembership{
		inst:     inlist,
		arg:      lhs,
		valueSet: valueSet,
	}, nil
}

// decLateBinding creates an InterpretableDecorator that is configured
// with the given options and transforms the Interpretable created by
// the planner with wrappers around function call nodes to defer the
// selection of the overload at evaluation time.
func decLateBinding(options ...LateBindCallOption) InterpretableDecorator {

	// initialise the configuration with the known types
	// of injectors.
	config := defaultInjectors(&lateBindConfig{
		injectors: map[reflect.Type]OverloadInjector{},
	})
	// add any other options to the configuration
	for _, option := range options {
		config = option(config)
	}
	// make sure that the cache is clean
	config.cache = map[int64]Interpretable{}

	// return the decorator.
	return func(interpretable Interpretable) (Interpretable, error) {

		return lateBind(config, interpretable)
	}
}

// lateBind implements the late binding decoration behaviour. The function
// uses a configuration to maintain a map of injectors that can be used to
// replicate and reconfigure InterpretableCall nodes with a runtime version
// of the matching overload identifier.
func lateBind(config *lateBindConfig, i Interpretable) (Interpretable, error) {

	if i == nil {
		return nil, nil
	}

	// have we already seen the interpretable, this is more of a safety
	// guard than anything else, which may happen because evalWatchXXX
	// structs wrap other Intepretable implementation, which may have been
	// already processed based on the order of decorators.
	id := i.ID()
	if _, seen := config.cache[id]; seen {
		return i, nil
	}

	// we need to make sure that we process nodes that wrap other
	// nodes that have the same identifiers. Therefore, we add the
	// node only when we complete this scope, otherwise the recursion
	// won't do anything on a node wrapping another.
	defer func() {
		// store the interpretable in the cache.
		config.cache[id] = i
	}()

	switch interpretable := i.(type) {

	case InterpretableCall:

		switch evalCall := interpretable.(type) {

		// we don't want to override the standard equality and
		// and non equality behaviour.
		case *evalEq:
		case *evalNe:
		// we don't want to double down on our own late binding
		// in case we have multiple late bind calls options in
		// planner.
		case *evalLateBind:
			return i, nil

		// all the other implementations of InterpretableCall are
		// not supported. We could rely on a default behaviour,
		// which relies only on InterpretableCall, but we wont be
		// executing possibly additional logic that is implemented
		// in the Eval method.
		default:

			evalType := reflect.TypeOf(evalCall)

			injector, found := config.injectors[evalType]
			if !found {
				return nil, UnknownCallNodeError(id, evalCall)
			}

			return &evalLateBind{
				target:         evalCall,
				injectOverload: injector,
				flags:          config.flags,
			}, nil
		}

	case *evalWatch:

		mapped, err := lateBind(config, interpretable.Interpretable)
		if err != nil {
			return nil, err
		}
		interpretable.Interpretable = mapped

		return interpretable, nil
	}

	// all the other cases aren't relevant.
	return i, nil
}
