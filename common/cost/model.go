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

package cost

import (
	"slices"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// TypeContext provides type information for AST nodes.
type TypeContext interface {
	// TargetType returns the deduced type of the receiver/target object, if present.
	TargetType() (*types.Type, bool)

	// ArgType returns the deduced type of the argument at the given 0-based index.
	ArgType(index int) (*types.Type, bool)
}

// EstimateContext provides size and argument evaluation during cost estimation.
type EstimateContext interface {
	TypeContext

	// Arg returns the size estimate for the argument at the given 0-based index.
	Arg(index int) (SizeEstimate, bool)

	// Target returns the size estimate for the receiver/target object, if present.
	Target() (SizeEstimate, bool)

	// Result returns the size estimate of the evaluated result, if known.
	Result() (SizeEstimate, bool)

	// Estimator returns the user-provided Estimator, if configured.
	Estimator() Estimator

	// Size returns the estimated size based on the SizingStrategy configured on the Estimator.
	Size(node AstNode) SizeEstimate

	// ArgValue returns the literal/constant uint64 value of the argument at index, or defaultVal.
	ArgValue(index int, defaultVal uint64) uint64

	// TargetValue returns the literal/constant uint64 value of the receiver/target, or defaultVal.
	TargetValue(defaultVal uint64) uint64
}

// TrackContext provides value size and argument evaluation during runtime cost tracking.
type TrackContext interface {
	TypeContext

	// Arg returns the size of the argument at the given 0-based index.
	Arg(index int) uint64

	// Target returns the size of the receiver/target object, if present.
	Target() uint64

	// Result returns the size of the evaluated result, if present.
	Result() uint64

	// Estimator returns the runtime ActualCostEstimator, if configured.
	Estimator() ActualCostEstimator

	// Size returns the actual runtime size of a value.
	Size(value ref.Val) uint64

	// ArgValue returns the uint64 value of the argument at index, or defaultVal.
	ArgValue(index int, defaultVal uint64) uint64

	// TargetValue returns the uint64 value of the receiver/target, or defaultVal.
	TargetValue(defaultVal uint64) uint64
}

// QuantityExpr represents a computable size or cost equation.
type QuantityExpr interface {
	estimate(ctx EstimateContext) SizeEstimate
	track(ctx TrackContext) uint64
}

// ModelVersionOf reports the cost model version the given Estimator is configured with, or
// math.MaxUint32 if the estimator does not carry version metadata.
func ModelVersionOf(estimator Estimator) uint32 {
	if v, ok := estimator.(interface{ modelVersion() uint32 }); ok {
		return v.modelVersion()
	}
	return defaultModelVersion
}

// targetInspector is an internal interface for expressions that inspect receiver/target presence.
type targetInspector interface {
	hasTarget() bool
}

// hasTarget returns true if the expression or any of its subexpressions references the target object.
func hasTarget(expr QuantityExpr) bool {
	if expr == nil {
		return false
	}
	if r, ok := expr.(targetInspector); ok {
		return r.hasTarget()
	}
	return false
}

// constExpr represents a constant integer quantity.
type constExpr struct {
	val uint64
}

func (c constExpr) estimate(_ EstimateContext) SizeEstimate {
	return FixedSizeEstimate(c.val)
}

func (c constExpr) track(_ TrackContext) uint64 {
	return c.val
}

func (constExpr) hasTarget() bool { return false }

// Const creates a constant quantity expression.
func Const(val uint64) QuantityExpr {
	return constExpr{val: val}
}

// argExpr represents the size of a function argument.
type argExpr struct {
	index int
}

func (a argExpr) estimate(ctx EstimateContext) SizeEstimate {
	if sz, ok := ctx.Arg(a.index); ok {
		return sz
	}
	return UnknownSizeEstimate()
}

func (a argExpr) track(ctx TrackContext) uint64 {
	return ctx.Arg(a.index)
}

func (argExpr) hasTarget() bool { return false }

// Arg creates an expression referencing the size of the argument at the given index.
func Arg(index int) QuantityExpr {
	return argExpr{index: index}
}

// ArgElem creates an expression referencing the element size of the argument at the given index.
func ArgElem(index int) QuantityExpr {
	return ElemOf(Arg(index))
}

// ArgKey creates an expression referencing the key size of the argument at the given index.
func ArgKey(index int) QuantityExpr {
	return KeyOf(Arg(index))
}

// intArgExpr represents the integer value of an argument.
type intArgExpr struct {
	index      int
	defaultVal uint64
}

func (a intArgExpr) estimate(ctx EstimateContext) SizeEstimate {
	return FixedSizeEstimate(ctx.ArgValue(a.index, a.defaultVal))
}

func (a intArgExpr) track(ctx TrackContext) uint64 {
	return ctx.ArgValue(a.index, a.defaultVal)
}

func (intArgExpr) hasTarget() bool { return false }

// IntArg creates an expression referencing the integer value of the argument at the given index,
// falling back to defaultVal if the argument is not a non-negative integer.
func IntArg(index int, defaultVal uint64) QuantityExpr {
	return intArgExpr{index: index, defaultVal: defaultVal}
}

// targetExpr represents the size of the receiver/target object.
type targetExpr struct{}

func (targetExpr) estimate(ctx EstimateContext) SizeEstimate {
	if sz, ok := ctx.Target(); ok {
		return sz
	}
	return UnknownSizeEstimate()
}

func (targetExpr) track(ctx TrackContext) uint64 {
	return ctx.Target()
}

func (targetExpr) hasTarget() bool { return true }

// Target creates an expression referencing the size of the receiver / target object.
func Target() QuantityExpr {
	return targetExpr{}
}

// TargetElem creates an expression referencing the element size of the receiver / target object.
func TargetElem() QuantityExpr {
	return ElemOf(Target())
}

// elemExpr represents the element size of another quantity expression.
type elemExpr struct {
	expr QuantityExpr
}

func (e elemExpr) estimate(ctx EstimateContext) SizeEstimate {
	sz := e.expr.estimate(ctx)
	if sz.Elem != nil {
		return *sz.Elem
	}
	return UnknownSizeEstimate()
}

func (e elemExpr) track(ctx TrackContext) uint64 {
	return 0
}

func (e elemExpr) hasTarget() bool {
	return hasTarget(e.expr)
}

// ElemOf creates an expression referencing the element size of another expression.
func ElemOf(expr QuantityExpr) QuantityExpr {
	return elemExpr{expr: expr}
}

// TargetKey creates an expression referencing the key size of the receiver / target object.
func TargetKey() QuantityExpr {
	return KeyOf(Target())
}

// intTargetExpr represents the integer value of the receiver/target object.
type intTargetExpr struct {
	defaultVal uint64
}

func (t intTargetExpr) estimate(ctx EstimateContext) SizeEstimate {
	return FixedSizeEstimate(ctx.TargetValue(t.defaultVal))
}

func (t intTargetExpr) track(ctx TrackContext) uint64 {
	return ctx.TargetValue(t.defaultVal)
}

func (intTargetExpr) hasTarget() bool { return true }

// IntTarget creates an expression referencing the integer value of the receiver/target object,
// falling back to defaultVal if the target is not a non-negative integer.
func IntTarget(defaultVal uint64) QuantityExpr {
	return intTargetExpr{defaultVal: defaultVal}
}

// keyExpr represents the key size of another quantity expression.
type keyExpr struct {
	expr QuantityExpr
}

func (k keyExpr) estimate(ctx EstimateContext) SizeEstimate {
	sz := k.expr.estimate(ctx)
	if sz.Key != nil {
		return *sz.Key
	}
	// An absent key size is unknown rather than single-byte: map keys may be strings or bytes of
	// any length. Strategies which know the key type is fixed-width populate Key themselves.
	return UnknownSizeEstimate()
}

func (k keyExpr) track(ctx TrackContext) uint64 {
	return 1
}

func (k keyExpr) hasTarget() bool {
	return hasTarget(k.expr)
}

// KeyOf creates an expression referencing the key size of another expression.
func KeyOf(expr QuantityExpr) QuantityExpr {
	return keyExpr{expr: expr}
}

// resultExpr represents the size of the evaluated result.
type resultExpr struct{}

func (resultExpr) estimate(ctx EstimateContext) SizeEstimate {
	if sz, ok := ctx.Result(); ok {
		return sz
	}
	return UnknownSizeEstimate()
}

func (resultExpr) track(ctx TrackContext) uint64 {
	return ctx.Result()
}

func (resultExpr) hasTarget() bool { return false }

// Result creates an expression referencing the size of the result.
func Result() QuantityExpr {
	return resultExpr{}
}

// addExpr represents the sum of multiple quantity expressions.
type addExpr struct {
	terms []QuantityExpr
}

func (a addExpr) estimate(ctx EstimateContext) SizeEstimate {
	if len(a.terms) == 0 {
		return FixedSizeEstimate(0)
	}
	sum := a.terms[0].estimate(ctx)
	for _, term := range a.terms[1:] {
		sum = sum.Add(term.estimate(ctx))
	}
	return sum
}

func (a addExpr) track(ctx TrackContext) uint64 {
	sum := uint64(0)
	for _, term := range a.terms {
		sum = SafeAdd(sum, term.track(ctx))
	}
	return sum
}

func (a addExpr) hasTarget() bool {
	return slices.ContainsFunc(a.terms, hasTarget)
}

// Sum creates an expression representing the sum of the given terms.
func Sum(terms ...QuantityExpr) QuantityExpr {
	return addExpr{terms: terms}
}

// subExpr represents the subtraction of two quantity expressions.
type subExpr struct {
	lhs, rhs QuantityExpr
}

func (s subExpr) estimate(ctx EstimateContext) SizeEstimate {
	lhsVal := s.lhs.estimate(ctx)
	rhsVal := s.rhs.estimate(ctx)
	return lhsVal.Subtract(rhsVal)
}

func (s subExpr) track(ctx TrackContext) uint64 {
	return SafeSubtract(s.lhs.track(ctx), s.rhs.track(ctx))
}

func (s subExpr) hasTarget() bool {
	return hasTarget(s.lhs) || hasTarget(s.rhs)
}

// Sub creates an expression representing lhs - rhs, saturated at zero.
func Sub(lhs, rhs QuantityExpr) QuantityExpr {
	return subExpr{lhs: lhs, rhs: rhs}
}

// mulExpr represents the product of multiple quantity expressions.
type mulExpr struct {
	terms []QuantityExpr
}

func (m mulExpr) estimate(ctx EstimateContext) SizeEstimate {
	if len(m.terms) == 0 {
		return FixedSizeEstimate(1)
	}
	prod := m.terms[0].estimate(ctx)
	for _, term := range m.terms[1:] {
		prod = prod.Multiply(term.estimate(ctx))
	}
	return prod
}

func (m mulExpr) track(ctx TrackContext) uint64 {
	if len(m.terms) == 0 {
		return 1
	}
	prod := m.terms[0].track(ctx)
	for _, term := range m.terms[1:] {
		prod = SafeMultiply(prod, term.track(ctx))
	}
	return prod
}

func (m mulExpr) hasTarget() bool {
	return slices.ContainsFunc(m.terms, hasTarget)
}

// Mul creates an expression representing the product of the given terms.
func Mul(terms ...QuantityExpr) QuantityExpr {
	return mulExpr{terms: terms}
}

// scaleExpr represents an expression scaled by a factor function.
type scaleExpr struct {
	expr    QuantityExpr
	scaleBy ScaleFn
}

func (s scaleExpr) estimate(ctx EstimateContext) SizeEstimate {
	val := s.expr.estimate(ctx)
	fac := s.scaleBy(ctx)
	res := RangedSizeEstimate(
		SafeMultiplyByFactor(val.Min, fac),
		SafeMultiplyByFactor(val.Max, fac),
	)
	res.Key = val.Key
	res.Elem = val.Elem
	return res
}

func (s scaleExpr) track(ctx TrackContext) uint64 {
	return SafeMultiplyByFactor(s.expr.track(ctx), s.scaleBy(ctx))
}

func (s scaleExpr) hasTarget() bool {
	return hasTarget(s.expr)
}

// ScaleFn represents a function that returns a scaling factor based on type context.
type ScaleFn func(ctx TypeContext) float64

// Scale creates an expression that scales another expression by a constant factor.
func Scale(expr QuantityExpr, factor float64) QuantityExpr {
	return ScaleBy(expr, func(ctx TypeContext) float64 { return factor })
}

// ScaleBy creates an expression that scales another expression by a given factor.
func ScaleBy(expr QuantityExpr, scaleBy ScaleFn) QuantityExpr {
	return scaleExpr{expr: expr, scaleBy: scaleBy}
}

// ArgTypeScale returns a scale factor based on the type of the i-th argument.
func ArgTypeScale(index int, scaler func(targetType *types.Type) float64) ScaleFn {
	return func(ctx TypeContext) float64 {
		t, ok := ctx.ArgType(index)
		if !ok || t == nil {
			return scaler(nil)
		}
		return scaler(t)
	}
}

// containerElemType extracts the element type from a parameterized container (List, Map value, or Opaque).
func containerElemType(t *types.Type) *types.Type {
	if t == nil || len(t.Parameters()) == 0 {
		return nil
	}
	switch t.Kind() {
	case types.ListKind, types.OpaqueKind:
		return t.Parameters()[0]
	case types.MapKind:
		if len(t.Parameters()) >= 2 {
			return t.Parameters()[1]
		}
	}
	return nil
}

// ArgElemTypeScale returns a scale factor based on the element type of the i-th argument.
func ArgElemTypeScale(index int, scaler func(elemType *types.Type) float64) ScaleFn {
	return func(ctx TypeContext) float64 {
		t, ok := ctx.ArgType(index)
		if !ok {
			return scaler(nil)
		}
		return scaler(containerElemType(t))
	}
}

// TargetTypeScale returns a scale factor based on the receiver / target type.
func TargetTypeScale(scaler func(targetType *types.Type) float64) ScaleFn {
	return func(ctx TypeContext) float64 {
		t, ok := ctx.TargetType()
		if !ok || t == nil {
			return scaler(nil)
		}
		return scaler(t)
	}
}

// TargetElemTypeScale returns a scale factor based on the element type of the receiver / target.
func TargetElemTypeScale(scaler func(*types.Type) float64) ScaleFn {
	return func(ctx TypeContext) float64 {
		t, ok := ctx.TargetType()
		if !ok {
			return scaler(nil)
		}
		return scaler(containerElemType(t))
	}
}

// Square creates an expression representing the square of another expression.
func Square(expr QuantityExpr) QuantityExpr {
	return Mul(expr, expr)
}

// minExpr represents the minimum of two quantity expressions.
type minExpr struct {
	lhs, rhs QuantityExpr
}

func (m minExpr) estimate(ctx EstimateContext) SizeEstimate {
	lhsVal := m.lhs.estimate(ctx)
	rhsVal := m.rhs.estimate(ctx)
	smallestMax := min(lhsVal.Max, rhsVal.Max)
	if contextModelVersion(ctx) < 1 {
		// 0 reported a floor of 1 for any non-empty interval, ignoring the operands'
		// own lower bounds. Retained verbatim so pinned callers see their recorded estimates.
		minVal := uint64(0)
		if smallestMax > 0 {
			minVal = 1
		}
		return RangedSizeEstimate(minVal, smallestMax)
	}
	return RangedSizeEstimate(min(lhsVal.Min, rhsVal.Min), smallestMax)
}

func (m minExpr) track(ctx TrackContext) uint64 {
	return min(m.lhs.track(ctx), m.rhs.track(ctx))
}

func (m minExpr) hasTarget() bool {
	return hasTarget(m.lhs) || hasTarget(m.rhs)
}

// Min creates an expression representing the minimum of two quantities.
func Min(lhs, rhs QuantityExpr) QuantityExpr {
	return minExpr{lhs: lhs, rhs: rhs}
}

// maxExpr represents the maximum of two quantity expressions.
type maxExpr struct {
	lhs, rhs QuantityExpr
}

func (m maxExpr) estimate(ctx EstimateContext) SizeEstimate {
	lhsVal := m.lhs.estimate(ctx)
	rhsVal := m.rhs.estimate(ctx)
	return SizeEstimate{
		Min: max(lhsVal.Min, rhsVal.Min),
		Max: max(lhsVal.Max, rhsVal.Max),
	}
}

func (m maxExpr) track(ctx TrackContext) uint64 {
	return max(m.lhs.track(ctx), m.rhs.track(ctx))
}

func (m maxExpr) hasTarget() bool {
	return hasTarget(m.lhs) || hasTarget(m.rhs)
}

// Max creates an expression representing the maximum of two quantities.
func Max(lhs, rhs QuantityExpr) QuantityExpr {
	return maxExpr{lhs: lhs, rhs: rhs}
}

// unionExpr represents the union interval of multiple quantity expressions.
type unionExpr struct {
	terms []QuantityExpr
}

func (u unionExpr) estimate(ctx EstimateContext) SizeEstimate {
	if len(u.terms) == 0 {
		return FixedSizeEstimate(0)
	}
	res := u.terms[0].estimate(ctx)
	for _, term := range u.terms[1:] {
		res = res.Union(term.estimate(ctx))
	}
	return res
}

func (u unionExpr) track(ctx TrackContext) uint64 {
	res := uint64(0)
	for _, term := range u.terms {
		res = max(res, term.track(ctx))
	}
	return res
}

func (u unionExpr) hasTarget() bool {
	return slices.ContainsFunc(u.terms, hasTarget)
}

// Union creates an expression representing the union interval of multiple expressions.
func Union(terms ...QuantityExpr) QuantityExpr {
	return unionExpr{terms: terms}
}

// rangedExpr represents a quantity bounded by minExpr on the bottom and maxExpr on top.
type rangedExpr struct {
	minExpr QuantityExpr
	maxExpr QuantityExpr
}

func (r rangedExpr) estimate(ctx EstimateContext) SizeEstimate {
	minVal := r.minExpr.estimate(ctx).Min
	maxVal := r.maxExpr.estimate(ctx).Max
	return SizeEstimate{
		Min: minVal,
		Max: maxVal,
	}
}

func (r rangedExpr) track(ctx TrackContext) uint64 {
	return r.maxExpr.track(ctx)
}

func (r rangedExpr) hasTarget() bool {
	return hasTarget(r.minExpr) || hasTarget(r.maxExpr)
}

// Ranged creates an expression where the lower bound is taken from minExpr and the upper bound from maxExpr.
func Ranged(minExpr, maxExpr QuantityExpr) QuantityExpr {
	return rangedExpr{minExpr: minExpr, maxExpr: maxExpr}
}

// AtMost creates an expression representing a quantity bounded between 0 and the upper bound of maxExpr.
func AtMost(maxExpr QuantityExpr) QuantityExpr {
	return Ranged(Const(0), maxExpr)
}

// atLeastOneExpr represents a quantity guaranteed to be at least 1.
type atLeastOneExpr struct {
	expr QuantityExpr
}

func (a atLeastOneExpr) estimate(ctx EstimateContext) SizeEstimate {
	return AtLeastOneSize(a.expr.estimate(ctx))
}

func (a atLeastOneExpr) track(ctx TrackContext) uint64 {
	val := a.expr.track(ctx)
	if val == 0 {
		return 1
	}
	return val
}

func (a atLeastOneExpr) hasTarget() bool {
	return hasTarget(a.expr)
}

// AtLeastOneQuantity creates an expression ensuring the quantity is at least 1.
func AtLeastOneQuantity(expr QuantityExpr) QuantityExpr {
	return atLeastOneExpr{expr: expr}
}

// StringScan creates an expression representing the cost of scanning a string.
func StringScan(expr QuantityExpr) QuantityExpr {
	return Scale(expr, StringTraversalCostFactor)
}

// ListAlloc creates an expression representing list allocation cost with base cost and scaled element count.
func ListAlloc(elemCount QuantityExpr, costFactor float64) QuantityExpr {
	return Sum(Const(ListCreateBaseCost), Scale(elemCount, costFactor))
}

// Traversal creates an expression representing the traversal and optional allocation cost over an expression.
func Traversal(expr QuantityExpr, costFactor float64, allocCost uint64) QuantityExpr {
	if allocCost == 0 {
		return Scale(expr, costFactor)
	}
	return Sum(Const(allocCost), Scale(expr, costFactor))
}

// listExpr represents a list size estimate composed of length and element size expressions.
type listExpr struct {
	lenExpr  QuantityExpr
	elemExpr QuantityExpr
}

func (l listExpr) estimate(ctx EstimateContext) SizeEstimate {
	lenSize := l.lenExpr.estimate(ctx)
	elemSize := l.elemExpr.estimate(ctx)
	lenSize.Elem = &elemSize
	return lenSize
}

func (l listExpr) track(ctx TrackContext) uint64 {
	return l.lenExpr.track(ctx)
}

func (l listExpr) hasTarget() bool {
	return hasTarget(l.lenExpr) || hasTarget(l.elemExpr)
}

// List creates an expression representing a list size with the given length and element size expressions.
func List(lenExpr, elemExpr QuantityExpr) QuantityExpr {
	return listExpr{lenExpr: lenExpr, elemExpr: elemExpr}
}

// mapExpr represents a map size estimate composed of size, key size, and value size expressions.
type mapExpr struct {
	sizeExpr QuantityExpr
	keyExpr  QuantityExpr
	valExpr  QuantityExpr
}

func (m mapExpr) estimate(ctx EstimateContext) SizeEstimate {
	mapSize := m.sizeExpr.estimate(ctx)
	keySize := m.keyExpr.estimate(ctx)
	valSize := m.valExpr.estimate(ctx)
	mapSize.Key = &keySize
	mapSize.Elem = &valSize
	return mapSize
}

func (m mapExpr) track(ctx TrackContext) uint64 {
	return m.sizeExpr.track(ctx)
}

func (m mapExpr) hasTarget() bool {
	return hasTarget(m.sizeExpr) || hasTarget(m.keyExpr) || hasTarget(m.valExpr)
}

// Map creates an expression representing a map size with the given map size, key size, and value size expressions.
func Map(sizeExpr, keyExpr, valExpr QuantityExpr) QuantityExpr {
	return mapExpr{sizeExpr: sizeExpr, keyExpr: keyExpr, valExpr: valExpr}
}

// OverloadModel pairs an overload ID with its abstract cost and result size expressions.
type OverloadModel struct {
	ID       string
	Cost     QuantityExpr
	Size     QuantityExpr
	IsMember bool
}

// OverloadModelOption functional options for configuring an OverloadModel.
type OverloadModelOption func(*OverloadModel)

// EvalCost sets the evaluation cost expression for the overload model.
func EvalCost(cost QuantityExpr) OverloadModelOption {
	return func(m *OverloadModel) {
		m.Cost = cost
	}
}

// ResultSize sets the result size expression for the overload model.
func ResultSize(size QuantityExpr) OverloadModelOption {
	return func(m *OverloadModel) {
		m.Size = size
	}
}

// Overload creates a global function OverloadModel with functional options.
func Overload(id string, opts ...OverloadModelOption) OverloadModel {
	m := OverloadModel{ID: id, IsMember: false}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// MemberOverload creates a member function OverloadModel with functional options.
func MemberOverload(id string, opts ...OverloadModelOption) OverloadModel {
	m := OverloadModel{ID: id, IsMember: true}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func (m OverloadModel) hasTarget() bool {
	return m.IsMember || hasTarget(m.Cost) || hasTarget(m.Size)
}

// FunctionEstimator returns a FunctionEstimator implementing the cost model.
func (m OverloadModel) FunctionEstimator() FunctionEstimator {
	return m.FunctionEstimatorWithOptions()
}

// FunctionEstimatorWithOptions returns a FunctionEstimator implementing the cost model, configured
// by the supplied options. Unset options take their defaults: the default SizingStrategy and the
// latest model revision.
func (m OverloadModel) FunctionEstimatorWithOptions(opts ...ModelOption) FunctionEstimator {
	resolved := newModelOptions(opts...)
	hasTarget := m.hasTarget()
	return func(estimator Estimator, target *AstNode, args []AstNode) *CallEstimate {
		if hasTarget && target == nil {
			return nil
		}
		ctx := &estimatorEvalContext{
			estimator: estimator,
			strategy:  resolved.strategy,
			version:   resolved.version,
			target:    target,
			args:      args,
			hasTarget: hasTarget,
		}
		costEst := m.Cost.estimate(ctx)
		var resSize *SizeEstimate
		if m.Size != nil {
			sz := m.Size.estimate(ctx)
			resSize = &sz
		}
		return NewCallEstimate(CostEstimate{Min: costEst.Min, Max: costEst.Max}, resSize)
	}
}

// FunctionTracker returns a FunctionTracker implementing the cost model.
func (m OverloadModel) FunctionTracker() FunctionTracker {
	return m.FunctionTrackerWithOptions()
}

// FunctionTrackerWithOptions returns a FunctionTracker implementing the cost model, configured by
// the supplied options.
//
// ModelVersion is accepted for symmetry with FunctionEstimatorWithOptions but has no effect:
// revisions correct estimation only, never the cost charged at runtime.
func (m OverloadModel) FunctionTrackerWithOptions(opts ...ModelOption) FunctionTracker {
	resolved := newModelOptions(opts...)
	isMember := m.hasTarget()
	return func(args []ref.Val, result ref.Val) *uint64 {
		ctx := &trackerEvalContext{
			strategy: resolved.strategy,
			args:     args,
			result:   result,
			isMember: isMember,
		}
		costVal := m.Cost.track(ctx)
		return &costVal
	}
}

// estimatorEvalContext provides evaluation context for cost estimation.
type estimatorEvalContext struct {
	coster    *coster
	estimator Estimator
	strategy  SizingStrategy
	version   uint32
	target    *AstNode
	args      []AstNode
	hasTarget bool
}

// modelVersion implements versionedEstimateContext.
func (e *estimatorEvalContext) modelVersion() uint32 {
	return e.version
}

// Arg returns the size estimate for the argument at index.
func (e *estimatorEvalContext) Arg(index int) (SizeEstimate, bool) {
	if index < len(e.args) {
		return e.Size(e.args[index]), true
	}
	return UnknownSizeEstimate(), false
}

// Target returns the size estimate for the receiver/target object.
func (e *estimatorEvalContext) Target() (SizeEstimate, bool) {
	if e.target != nil {
		return e.Size(*e.target), true
	}
	return UnknownSizeEstimate(), false
}

// Result returns the size estimate of the result.
func (e *estimatorEvalContext) Result() (SizeEstimate, bool) {
	return UnknownSizeEstimate(), false
}

// Estimator returns the underlying Estimator.
func (e *estimatorEvalContext) Estimator() Estimator {
	return e.estimator
}

// Size returns the estimated size of an AST node.
func (e *estimatorEvalContext) Size(node AstNode) SizeEstimate {
	if node == nil {
		return UnknownSizeEstimate()
	}
	if sz := node.ComputedSize(); sz != nil {
		return *sz
	}
	if e.coster != nil && node.Expr() != nil {
		if sz := e.coster.computeSize(node.Expr()); sz != nil {
			return *sz
		}
	}
	if e.strategy != nil {
		if sz, ok := e.strategy.EstimateSize(e, node); ok {
			return sz
		}
	}
	if e.estimator != nil {
		if sz := e.estimator.EstimateSize(node); sz != nil {
			return *sz
		}
	}
	return UnknownSizeEstimate()
}

// TargetType returns the type of the receiver/target object.
func (e *estimatorEvalContext) TargetType() (*types.Type, bool) {
	if e.target != nil && (*e.target) != nil {
		return (*e.target).Type(), true
	}
	return nil, false
}

// ArgType returns the type of the argument at index.
func (e *estimatorEvalContext) ArgType(index int) (*types.Type, bool) {
	if index < len(e.args) && e.args[index] != nil {
		return e.args[index].Type(), true
	}
	return nil, false
}

// ArgValue returns the uint64 value of the argument at index, or defaultVal if not a literal int/uint.
func (e *estimatorEvalContext) ArgValue(index int, defaultVal uint64) uint64 {
	if index < len(e.args) && e.args[index] != nil {
		return NodeAsUintValue(e.args[index], defaultVal)
	}
	return defaultVal
}

// TargetValue returns the uint64 value of the receiver/target, or defaultVal if not a literal int/uint.
func (e *estimatorEvalContext) TargetValue(defaultVal uint64) uint64 {
	if e.target != nil && (*e.target) != nil {
		return NodeAsUintValue(*e.target, defaultVal)
	}
	return defaultVal
}

// trackerEvalContext provides evaluation context for runtime cost tracking.
type trackerEvalContext struct {
	estimator ActualCostEstimator
	strategy  SizingStrategy
	args      []ref.Val
	result    ref.Val
	isMember  bool
}

// valueAsUint returns the non-negative uint64 value of ref.Val (int or uint), or defaultVal.
func valueAsUint(val ref.Val, defaultVal uint64) uint64 {
	if val == nil {
		return defaultVal
	}
	switch v := val.(type) {
	case types.Int:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case types.Uint:
		return uint64(v)
	default:
		return defaultVal
	}
}

// ArgValue returns the uint64 value of the argument at index, or defaultVal.
func (t *trackerEvalContext) ArgValue(index int, defaultVal uint64) uint64 {
	idx := index
	if t.isMember {
		idx = index + 1
	}
	if idx < len(t.args) {
		return valueAsUint(t.args[idx], defaultVal)
	}
	return defaultVal
}

// TargetValue returns the uint64 value of the receiver/target object, or defaultVal.
func (t *trackerEvalContext) TargetValue(defaultVal uint64) uint64 {
	if t.isMember && len(t.args) > 0 {
		return valueAsUint(t.args[0], defaultVal)
	}
	return defaultVal
}

// TargetType returns the type of the receiver/target object.
func (t *trackerEvalContext) TargetType() (*types.Type, bool) {
	if t.isMember && len(t.args) > 0 && t.args[0] != nil {
		if tp, ok := t.args[0].Type().(*types.Type); ok {
			return tp, true
		}
	}
	return nil, false
}

// ArgType returns the type of the argument at index.
func (t *trackerEvalContext) ArgType(index int) (*types.Type, bool) {
	idx := index
	if t.isMember {
		idx = index + 1
	}
	if idx < len(t.args) && t.args[idx] != nil {
		if tp, ok := t.args[idx].Type().(*types.Type); ok {
			return tp, true
		}
	}
	return nil, false
}

// Estimator returns the runtime ActualCostEstimator.
func (t *trackerEvalContext) Estimator() ActualCostEstimator {
	return t.estimator
}

// Size returns the actual runtime size of a value.
func (t *trackerEvalContext) Size(value ref.Val) uint64 {
	if value == nil {
		return 0
	}
	if t.strategy != nil {
		if sz, ok := t.strategy.TrackSize(t, value); ok {
			return sz
		}
	}
	return ActualSize(value)
}

// Arg returns the runtime size of the argument at index.
func (t *trackerEvalContext) Arg(index int) uint64 {
	idx := index
	if t.isMember {
		idx = index + 1
	}
	if idx < len(t.args) {
		return t.Size(t.args[idx])
	}
	return 0
}

// Target returns the runtime size of the receiver/target object.
func (t *trackerEvalContext) Target() uint64 {
	if t.isMember && len(t.args) > 0 {
		return t.Size(t.args[0])
	}
	return 0
}

// Result returns the runtime size of the result.
func (t *trackerEvalContext) Result() uint64 {
	if t.result != nil {
		return t.Size(t.result)
	}
	return 0
}
