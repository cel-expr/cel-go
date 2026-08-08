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

package ext

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter"
)

// Maps returns a cel.EnvOption to configure namespaced map functions.
//
// CEL has no operator for combining two maps: the `+` operator concatenates
// strings, bytes, and lists, but is not defined for maps. This library provides
// map combination as a named function.
//
// # Maps.Merge
//
// Returns a new map containing the entries of both arguments. When a key is
// present in both, the value from the second argument wins. Neither input is
// modified.
//
// The merge is shallow: a value that is itself a map is replaced rather than
// merged recursively.
//
//	maps.merge(map(K, V), map(K, V)) -> map(K, V)
//
// Examples:
//
//	maps.merge({}, {}) // {}
//	maps.merge({'a': 1}, {'b': 2}) // {'a': 1, 'b': 2}
//	maps.merge({'a': 1}, {'a': 2}) // {'a': 2}
//	maps.merge({'a': {'x': 1}}, {'a': {'y': 2}}) // {'a': {'y': 2}}, values are replaced, not merged
func Maps(options ...MapsOption) cel.EnvOption {
	l := &mapsLib{}
	for _, o := range options {
		l = o(l)
	}
	return cel.Lib(l)
}

// MapsOption declares a functional operator for configuring map extensions.
type MapsOption func(*mapsLib) *mapsLib

// MapsVersion sets the library version for map extensions.
func MapsVersion(version uint32) MapsOption {
	return func(lib *mapsLib) *mapsLib {
		lib.version = version
		return lib
	}
}

type mapsLib struct {
	version uint32
}

// LibraryName implements the SingletonLibrary interface method.
func (mapsLib) LibraryName() string {
	return "cel.lib.ext.maps"
}

// CompileOptions implements the Library interface method.
func (mapsLib) CompileOptions() []cel.EnvOption {
	mapType := cel.MapType(cel.TypeParamType("K"), cel.TypeParamType("V"))
	return []cel.EnvOption{
		cel.Function("maps.merge",
			cel.Overload("map_maps_merge_map", []*cel.Type{mapType, mapType}, mapType,
				cel.BinaryBinding(mapsMerge))),
		cel.CostEstimatorOptions(
			checker.OverloadCostEstimate("map_maps_merge_map", estimateMapsMergeCost),
		),
	}
}

// ProgramOptions implements the Library interface method.
func (mapsLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.CostTrackerOptions(
			interpreter.OverloadCostTracker("map_maps_merge_map", trackMapsMergeCost),
		),
	}
}

// estimateMapsMergeCost charges for visiting every entry of both inputs and for
// allocating the result map.
func estimateMapsMergeCost(estimator checker.CostEstimator, _ *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	if len(args) != 2 {
		return nil
	}
	lhsSize := estimateSize(estimator, args[0])
	rhsSize := estimateSize(estimator, args[1])
	entries := lhsSize.Add(rhsSize)
	cost := entries.MultiplyByCostFactor(1).Add(mapAllocCost).Add(callCostEstimate)
	// The result holds at least as many entries as the larger input, when every
	// key collides, and at most the sum of both, when none do.
	resultSize := rangedSizeEstimate(max(lhsSize.Min, rhsSize.Min), entries.Max)
	return callEstimate(cost, &resultSize)
}

// trackMapsMergeCost mirrors estimateMapsMergeCost against the actual inputs.
func trackMapsMergeCost(args []ref.Val, _ ref.Val) *uint64 {
	entries := safeAdd(actualSize(args[0]), actualSize(args[1]))
	cost := safeAdd(callCost, uint64(common.MapCreateBaseCost), entries)
	return &cost
}

// mapsMerge returns a new map holding the entries of both inputs, with the
// values of the second input taking precedence on conflicting keys.
func mapsMerge(lhs, rhs ref.Val) ref.Val {
	first, ok := lhs.(traits.Mapper)
	if !ok {
		return types.MaybeNoSuchOverloadErr(lhs)
	}
	second, ok := rhs.(traits.Mapper)
	if !ok {
		return types.MaybeNoSuchOverloadErr(rhs)
	}
	merged := make(map[ref.Val]ref.Val, actualSize(first)+actualSize(second))
	if err := copyEntries(first, merged); err != nil {
		return err
	}
	if err := copyEntries(second, merged); err != nil {
		return err
	}
	return types.NewRefValMap(types.DefaultTypeAdapter, merged)
}

// copyEntries writes every entry of m into dst, overwriting entries whose keys
// are already present. It returns a non-nil ref.Val only when the map yields an
// error or unknown value.
func copyEntries(m traits.Mapper, dst map[ref.Val]ref.Val) ref.Val {
	it := m.Iterator()
	for it.HasNext() == types.True {
		key := it.Next()
		if types.IsUnknownOrError(key) {
			return key
		}
		val, _ := m.Find(key)
		if types.IsUnknownOrError(val) {
			return val
		}
		dst[key] = val
	}
	return nil
}
