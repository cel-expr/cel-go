# Versioned Cost Estimation & Efficient Aggregate Size Tracking Design

## Executive Summary

This design document specifies a system for versioning cost estimation and runtime tracking in CEL-Go ([`checker/cost.go`](file:///Users/tswadell/go/src/github.com/google/cel-go/checker/cost.go) and [`interpreter/runtimecost.go`](file:///Users/tswadell/go/src/github.com/google/cel-go/interpreter/runtimecost.go)). It addresses equality operator (`==`, `!=`) cost estimation for lists, maps, and structured objects with:

1. **Size Tracking**: Introduction of `traits.AggregateSizer` for `ref.Val` container objects, backed by zero-allocation struct layout packing (`uint32`).
2. **Size Estimation**: A recursive static size estimation utility that handles inline AST literals and multiplicative path-hint chains.
3. **Equality Cost V1**: Recursive equality estimators and short-circuiting trackers bound to `overloads.Equals` and `overloads.NotEquals` when configured via `cel.CostVersion(1)`.

---

## 1. Size Tracking

### 1.1 Runtime Size Interface (`traits.AggregateSizer`)

A new interface is introduced in `common/types/traits/aggregate_sizer.go`:

```go
package traits

// AggregateSizer interface for ref.Val implementations capable of returning
// their total recursive element count.
type AggregateSizer interface {
	// AggregateSize returns the total count of nested atomic elements (capped at math.MaxUint32).
	AggregateSize() uint32
}
```

#### Memoizing Implementations Across Ref.Val Types
- **Lists (`baseList` in `common/types/list.go`)**: Computes aggregate element count lazily on first access and memoizes the value in an `aggSize` field. Pre-computes `aggSize` during constructor initialization for list literals (`NewRefValList`).
- **Maps (`baseMap` in `common/types/map.go`)**: Sums aggregate key and value sizes across all entries lazily and memoizes the result.
- **Objects / Structs (`protoObj` in `common/types/object.go`)**: Computes set field count plus recursive field aggregate sizes.
- **Optionals (`optionalVal` in `common/types/optional.go`)**: Returns `0` if empty (`OptionalNone`), or `1 + value.AggregateSize()` if engaged (`OptionalSome`).

---

### 1.2 Runtime Efficiency: Packed Struct Layout

To prevent `aggSize` from increasing struct memory allocation or pushing instances into higher Go runtime `mcache` size classes, `size` is converted from `int` (8 bytes) to `uint32` (4 bytes), and `aggSize uint32` (4 bytes) is packed into the same 8-byte word slot.

#### `baseList` Struct Packing (48 Bytes — 0 Allocation Increase)
```go
// Before: 48 bytes (fits in 48-byte size class)
// After: EXACTLY 48 bytes (0 allocation increase!)
type baseList struct {
	Adapter               // 16 bytes
	value   any           // 16 bytes
	size    uint32        // 4 bytes \ Packed into single 8-byte word!
	aggSize uint32        // 4 bytes / Zero extra memory allocated!
	get     func(int) any // 8 bytes
}
```

#### `baseMap` Struct Packing (56 Bytes — 0 Allocation Increase)
```go
// Before: 56 bytes (fits in 64-byte size class)
// After: EXACTLY 56 bytes (0 allocation increase!)
type baseMap struct {
	Adapter             // 16 bytes
	mapAccessor         // 16 bytes
	value       any     // 16 bytes
	size        uint32  // 4 bytes \ Packed into single 8-byte word!
	aggSize     uint32  // 4 bytes / Zero extra memory allocated!
}
```

---

## 2. Size Estimation

A recursive static size estimation utility, `computeAggregateSize`, is added to `checker/cost.go`. It handles both inline AST literal inspection and path-hint chain composition.

```go
// computeAggregateSize computes the recursive element count range of an AstNode.
func (c *coster) computeAggregateSize(node AstNode) SizeEstimate {
	if node == nil {
		return SizeEstimate{Min: 0, Max: 0}
	}

	// 1. Inline AST Literals & Containers
	if expr := node.Expr(); expr != nil {
		switch expr.Kind() {
		case ast.LiteralKind:
			return FixedSizeEstimate(1)

		case ast.ListKind:
			var total SizeEstimate
			for _, elem := range expr.AsList().Elements() {
				total = total.Add(c.computeAggregateSize(c.newAstNode(elem)))
			}
			if total.Max > 0 {
				return total
			}

		case ast.MapKind:
			var total SizeEstimate
			for _, ent := range expr.AsMap().Entries() {
				entry := ent.AsMapEntry()
				total = total.Add(c.computeAggregateSize(c.newAstNode(entry.Key())))
				total = total.Add(c.computeAggregateSize(c.newAstNode(entry.Value())))
			}
			if total.Max > 0 {
				return total
			}
		}
	}

	// 2. Multiplicative Path-Hint Chain Composition
	if path := node.Path(); len(path) > 0 {
		return c.computePathChainSize(node, path)
	}

	if sz := node.ComputedSize(); sz != nil {
		return *sz
	}
	return UnknownSizeEstimate()
}

// computePathChainSize evaluates multiplicative hint chains (x -> x.@items -> x.@items.@items)
func (c *coster) computePathChainSize(node AstNode, basePath []string) SizeEstimate {
	topSize := c.estimator.EstimateSize(node)
	if topSize == nil {
		return UnknownSizeEstimate()
	}

	maxProd := topSize.Max
	minBound := uint64(1)
	if topSize.Min == 0 {
		minBound = 0
	}

	subpath := "@items"
	if node.Type() != nil && node.Type().Kind() == types.MapKind {
		subpath = "@values"
	}

	currentPath := append([]string{}, basePath...)
	for depth := 0; depth < maxCostRecursionDepth; depth++ {
		currentPath = append(currentPath, subpath)
		childNode := &astNode{path: currentPath}
		childSize := c.estimator.EstimateSize(childNode)
		if childSize == nil {
			break
		}
		maxProd = multiplyUint64NoOverflow(maxProd, childSize.Max)
	}

	return SizeEstimate{Min: minBound, Max: maxProd}
}
```

### Verified Target Behavior:
1. **Inline Literal `[1, [3, 4], [[7, 8], [9, 10]]]`**:
   - `1` $\rightarrow 1$
   - `[3, 4]` $\rightarrow 2$
   - `[[7, 8], [9, 10]]` $\rightarrow 4$
   - **Aggregate Total = 7**.
2. **Variable `x` with hints `{'x': 3, 'x.@items': 3, 'x.@items.@items': 5}`**:
   - Level 1: $3$
   - Level 2: $3 \times 3 = 9$
   - Level 3: $9 \times 5 = 45$
   - **Result Range = `{Min: 1, Max: 45}`**.

---

## 3. Equality Cost V1

### 3.1 Static & Runtime Equality Utilities

#### Static AST Equality Cost (`checker/cost.go`)
```go
func estimateEqualityCost(c *coster, lhs, rhs AstNode, depth int) CostEstimate {
	if depth > maxCostRecursionDepth {
		return UnknownCostEstimate()
	}

	lhsType, rhsType := lhs.Type(), rhs.Type()

	// Primitive Equality
	if isScalar(lhsType) && isScalar(rhsType) {
		return FixedCostEstimate(1)
	}

	// Container Equality (Lists & Maps)
	lhsAgg := c.computeAggregateSize(lhs)
	rhsAgg := c.computeAggregateSize(rhs)
	maxElements := minUint64(lhsAgg.Max, rhsAgg.Max)

	// Bounded max cost based on aggregate size bounds
	return CostEstimate{Min: 1, Max: addUint64NoOverflow(1, maxElements)}
}
```

#### Runtime Equality Cost with Short-Circuiting (`interpreter/runtimecost.go`)
```go
func trackEqualityCost(val1, val2 ref.Val, depth int) uint64 {
	if depth > maxCostRecursionDepth || val1 == nil || val2 == nil {
		return 1
	}

	// Fast O(1) aggregate size lookup via traits.AggregateSizer
	var agg1, agg2 uint32 = 1, 1
	if a1, ok := val1.(traits.AggregateSizer); ok {
		agg1 = a1.AggregateSize()
	}
	if a2, ok := val2.(traits.AggregateSizer); ok {
		agg2 = a2.AggregateSize()
	}

	// List short-circuiting: unequal lengths cost 1 base unit
	if l1, ok1 := val1.(traits.Lister); ok1 {
		if l2, ok2 := val2.(traits.Lister); ok2 {
			if l1.Size().(types.Int) != l2.Size().(types.Int) {
				return 1
			}
			var accum uint64 = 1
			sz := l1.Size().(types.Int)
			for i := types.Int(0); i < sz; i++ {
				e1, e2 := l1.Get(i), l2.Get(i)
				accum = safeAdd(accum, trackEqualityCost(e1, e2, depth+1))
				if e1.Equal(e2) != types.True {
					break // Short-circuit on first unequal element pair
				}
			}
			return accum
		}
	}

	return safeAdd(1, uint64(minUint32(agg1, agg2)))
}
```

---

### 3.2 Overloads & `CostVersion(1)` Configuration

#### `cel/options.go` Functional Option
```go
package cel

// CostVersion configures the cost model version.
// Version 0: Legacy O(1) container equality costing.
// Version 1: Specialized recursive equality costing based on AggregateSize and path hints.
func CostVersion(v uint32) EnvOption {
	return func(e *Env) error {
		e.costVersion = v
		return nil
	}
}
```

#### FunctionEstimator & FunctionTracker Overload Bindings

- **Static Handler (`checker/cost.go`)**:
  ```go
  func EqualsEstimatorV1(estimator CostEstimator, target *AstNode, args []AstNode) *CallEstimate {
  	if len(args) != 2 {
  		return nil
  	}
  	est := estimateEqualityCost(nil, args[0], args[1], 0)
  	return &CallEstimate{CostEstimate: est}
  }
  ```

- **Runtime Handler (`interpreter/runtimecost.go`)**:
  ```go
  func EqualsTrackerV1(args []ref.Val, result ref.Val) *uint64 {
  	if len(args) != 2 {
  		return nil
  	}
  	cost := trackEqualityCost(args[0], args[1], 0)
  	return &cost
  }
  ```

When `cel.NewEnv(cel.CostVersion(1))` is invoked, `EqualsEstimatorV1` and `EqualsTrackerV1` are registered for `overloads.Equals` (`"equals"`) and `overloads.NotEquals` (`"not_equals"`), enabling Version 1 cost estimation without breaking legacy Version 0 configurations.
