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
}

// QuantityExpr represents a computable size or cost equation.
type QuantityExpr interface {
	estimate(ctx EstimateContext) SizeEstimate
	track(ctx TrackContext) uint64
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

// keyExpr represents the key size of another quantity expression.
type keyExpr struct {
	expr QuantityExpr
}

func (k keyExpr) estimate(ctx EstimateContext) SizeEstimate {
	sz := k.expr.estimate(ctx)
	if sz.Key != nil {
		return *sz.Key
	}
	return FixedSizeEstimate(1)
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
	minVal := uint64(0)
	smallestMax := min(lhsVal.Max, rhsVal.Max)
	if smallestMax > 0 {
		minVal = 1
	}
	return RangedSizeEstimate(minVal, smallestMax)
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

// intersectExpr represents the intersection interval of multiple quantity expressions.
type intersectExpr struct {
	terms []QuantityExpr
}

func (in intersectExpr) estimate(ctx EstimateContext) SizeEstimate {
	if len(in.terms) == 0 {
		return FixedSizeEstimate(0)
	}
	res := in.terms[0].estimate(ctx)
	for _, term := range in.terms[1:] {
		other := term.estimate(ctx)
		minVal := max(res.Min, other.Min)
		maxVal := min(res.Max, other.Max)
		if minVal > maxVal {
			return FixedSizeEstimate(0)
		}
		res = SizeEstimate{Min: minVal, Max: maxVal}
	}
	return res
}

func (in intersectExpr) track(ctx TrackContext) uint64 {
	if len(in.terms) == 0 {
		return 0
	}
	res := in.terms[0].track(ctx)
	for _, term := range in.terms[1:] {
		res = min(res, term.track(ctx))
	}
	return res
}

func (in intersectExpr) hasTarget() bool {
	return slices.ContainsFunc(in.terms, hasTarget)
}

// Intersect creates an expression representing the intersection interval of multiple expressions.
func Intersect(terms ...QuantityExpr) QuantityExpr {
	return intersectExpr{terms: terms}
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
	return m.FunctionEstimatorWithOptions(nil)
}

// FunctionEstimatorWithOptions returns a FunctionEstimator implementing the cost model with an optional SizingStrategy.
func (m OverloadModel) FunctionEstimatorWithOptions(strategy SizingStrategy) FunctionEstimator {
	hasTarget := m.hasTarget()
	return func(estimator Estimator, target *AstNode, args []AstNode) *CallEstimate {
		if hasTarget && target == nil {
			return nil
		}
		ctx := &estimatorEvalContext{
			estimator: estimator,
			strategy:  strategy,
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
	return m.FunctionTrackerWithOptions(nil)
}

// FunctionTrackerWithOptions returns a FunctionTracker implementing the cost model with an optional SizingStrategy.
func (m OverloadModel) FunctionTrackerWithOptions(strategy SizingStrategy) FunctionTracker {
	isMember := m.hasTarget()
	return func(args []ref.Val, result ref.Val) *uint64 {
		ctx := &trackerEvalContext{
			strategy: strategy,
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
	estimator Estimator
	strategy  SizingStrategy
	target    *AstNode
	args      []AstNode
	hasTarget bool
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

// trackerEvalContext provides evaluation context for runtime cost tracking.
type trackerEvalContext struct {
	estimator ActualCostEstimator
	strategy  SizingStrategy
	args      []ref.Val
	result    ref.Val
	isMember  bool
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
