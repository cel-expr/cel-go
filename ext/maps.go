// Copyright 2025 Google LLC
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
	"math"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// Maps returns a cel.EnvOption to configure extended functions for map manipulation.
//
// # Merge
//
// Merges two maps. Keys from the second map overwrite already available keys in the first map.
// Keys must be of type string, value types must be identical in the maps merged.
//
//	map(string, T).merge(map(string, T)) -> map(string, T)
//
// Examples:
//
//  {}.merge({}) == {}
//  {}.merge({'a': 1}) == {'a': 1}`},
//  {}.merge({'a': 2.1}) == {'a': 2.1}`},
//  {}.merge({'a': 'foo'}) == {'a': 'foo'}`},
//  {'a': 1}.merge({}) == {'a': 1}`},
//  {'a': 1}.merge({'b': 2}) == {'a': 1, 'b': 2}`},
//  {'a': 1}.merge({'a': 2, 'b': 2}) == {'a': 2, 'b': 2}`},

func Maps(options ...MapsOption) cel.EnvOption {
	l := &mapsLib{version: math.MaxUint32}
	for _, o := range options {
		l = o(l)
	}
	return cel.Lib(l)
}

type mapsLib struct {
	version uint32
}

type MapsOption func(*mapsLib) *mapsLib

// MapsVersion configures the version of the maps library.
//
// The version limits which functions are available. Only functions introduced
// below or equal to the given version included in the library. If this option
// is not set, all functions are available.
//
// See the library documentation to determine which version a function was introduced.
// If the documentation does not state which version a function was introduced, it can
// be assumed to be introduced at version 0, when the library was first created.
func MapsVersion(version uint32) MapsOption {
	return func(lib *mapsLib) *mapsLib {
		lib.version = version
		return lib
	}
}

// LibraryName implements the cel.SingletonLibrary interface method.
func (mapsLib) LibraryName() string {
	return "cel.lib.ext.maps"
}

// CompileOptions implements the cel.Library interface method.
func (lib mapsLib) CompileOptions() []cel.EnvOption {
	mapType := cel.MapType(cel.TypeParamType("K"), cel.TypeParamType("V"))
	opts := []cel.EnvOption{
		cel.Function("merge",
			cel.MemberOverload("map_merge",
				[]*cel.Type{mapType, mapType},
				mapType,
				cel.BinaryBinding(mergeVals),
			),
		),
	}
	return opts
}

// ProgramOptions implements the cel.Library interface method.
func (lib mapsLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func mergeVals(lhs, rhs ref.Val) ref.Val {
	left, lok := lhs.(traits.Mapper)
	right, rok := rhs.(traits.Mapper)
	if !lok || !rok {
		return types.ValOrErr(lhs, "no such overload: %v.merge(%v)", lhs.Type(), rhs.Type())
	}
	return merge(left, right)
}

// merge returns a new map containing entries from both maps.
// Keys in 'other' overwrite keys in 'self'.
func merge(self, other traits.Mapper) traits.Mapper {
	result := mapperTraitToMutableMapper(other)
	for i := self.Iterator(); i.HasNext().(types.Bool); {
		k := i.Next()
		if !result.Contains(k).(types.Bool) {
			result.Insert(k, self.Get(k))
		}
	}
	return result.ToImmutableMap()
}

// mapperTraitToMutableMapper copies a traits.Mapper into a MutableMap.
func mapperTraitToMutableMapper(m traits.Mapper) traits.MutableMapper {
	vals := make(map[ref.Val]ref.Val, m.Size().(types.Int))
	for it := m.Iterator(); it.HasNext().(types.Bool); {
		k := it.Next()
		vals[k] = m.Get(k)
	}
	return types.NewMutableMap(types.DefaultTypeAdapter, vals)
}
