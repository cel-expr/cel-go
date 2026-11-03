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
	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// Maps returns a cel.EnvOption to configure namespaced map functions.
//
// CEL has no operator for combining two maps: the `+` operator concatenates
// strings, bytes, and lists, but is not defined for maps. This library provides
// map combination as a named function.
//
// # PutAll
//
// Returns a new map containing the entries of both maps. When a key is
// present in both, the value from the argument wins. Neither input is
// modified.
//
// The copy is shallow: a value that is itself a map is replaced rather than
// combined recursively.
//
//	<map(K, V)>.putAll(<map(K, V)>) -> <map(K, V)>
//
// Examples:
//
//	{}.putAll({}) // {}
//	{'a': 1}.putAll({'b': 2}) // {'a': 1, 'b': 2}
//	{'a': 1}.putAll({'a': 2}) // {'a': 2}
//	{'a': {'x': 1}}.putAll({'a': {'y': 2}}) // {'a': {'y': 2}}, values are replaced, not combined
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
		cel.Function("putAll",
			cel.MemberOverload("map_put_all_map", []*cel.Type{mapType, mapType}, mapType,
				cel.BinaryBinding(mapsPutAll))),
		cel.CostModel(mapPutAllCostModel("map_put_all_map")),
	}
}

// ProgramOptions implements the Library interface method.
func (mapsLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

// mapPutAllCostModel models m.putAll(other): it visits every entry of both maps and allocates the
// result. The result holds at least as many entries as the larger input, when every key collides,
// and at most the sum of both, when none do. Each of its keys and values comes from one of the
// inputs.
func mapPutAllCostModel(id string) cost.OverloadModel {
	targetLen := cost.Length(cost.Target())
	argLen := cost.Length(cost.Arg(0))
	return cost.MemberOverload(id,
		cost.EvalCost(cost.Sum(cost.Const(cost.CallCost+cost.MapCreateBaseCost), targetLen, argLen)),
		cost.ResultSize(cost.Map(
			cost.Ranged(cost.Max(targetLen, argLen), cost.Sum(targetLen, argLen)),
			cost.Union(cost.TargetKey(), cost.ArgKey(0)),
			cost.Union(cost.TargetElem(), cost.ArgElem(0)),
		)),
	)
}

// mapsPutAll returns a new map holding the entries of both inputs, with the
// values of the second input taking precedence on conflicting keys.
func mapsPutAll(lhs, rhs ref.Val) ref.Val {
	first, ok := lhs.(traits.Mapper)
	if !ok {
		return types.MaybeNoSuchOverloadErr(lhs)
	}
	second, ok := rhs.(traits.Mapper)
	if !ok {
		return types.MaybeNoSuchOverloadErr(rhs)
	}
	entries := make(map[ref.Val]ref.Val, cost.ActualSize(first)+cost.ActualSize(second))
	if err := copyEntries(first, entries); err != nil {
		return err
	}
	if err := copyEntries(second, entries); err != nil {
		return err
	}
	return types.NewRefValMap(types.DefaultTypeAdapter, entries)
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
