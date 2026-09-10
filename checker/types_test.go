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

package checker

import (
	"testing"

	"cel.dev/cel-go/common/types"
)

func TestIsAssignable_TypeType(t *testing.T) {
	tests := []struct {
		name       string
		from       *types.Type
		to         *types.Type
		setup      func(m *mapping)
		wantAssign bool
		wantSubs   map[string]*types.Type
	}{
		{
			name:       "unparameterized_to_unparameterized",
			from:       types.TypeType,
			to:         types.TypeType,
			wantAssign: true,
		},
		{
			name:       "parameterized_to_unparameterized",
			from:       types.NewTypeTypeWithParam(types.IntType),
			to:         types.TypeType,
			wantAssign: true,
		},
		{
			name:       "unparameterized_to_parameterized",
			from:       types.TypeType,
			to:         types.NewTypeTypeWithParam(types.IntType),
			wantAssign: false,
		},
		{
			name:       "concreteTypes_legacyCoassignability",
			from:       types.NewTypeTypeWithParam(types.IntType),
			to:         types.NewTypeTypeWithParam(types.StringType),
			wantAssign: true,
		},
		{
			name:       "concreteTypes_legacyCoassignability_reverse",
			from:       types.NewTypeTypeWithParam(types.StringType),
			to:         types.NewTypeTypeWithParam(types.IntType),
			wantAssign: true,
		},
		{
			name:       "mapContainerErasure",
			from:       types.NewTypeTypeWithParam(types.NewMapType(types.IntType, types.UintType)),
			to:         types.NewTypeTypeWithParam(types.NewMapType(types.DynType, types.DynType)),
			wantAssign: true,
		},
		{
			name:       "listContainerErasure",
			from:       types.NewTypeTypeWithParam(types.NewListType(types.IntType)),
			to:         types.NewTypeTypeWithParam(types.NewListType(types.DynType)),
			wantAssign: true,
		},
		{
			name:       "typeParamTarget_bindsConcreteType",
			from:       types.NewTypeTypeWithParam(types.IntType),
			to:         types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"T": types.IntType,
			},
		},
		{
			name:       "typeParamSource_bindsConcreteType",
			from:       types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			to:         types.NewTypeTypeWithParam(types.IntType),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"T": types.IntType,
			},
		},
		{
			name:       "nestedTypeParam_unifies",
			from:       types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			to:         types.NewTypeTypeWithParam(types.NewTypeTypeWithParam(types.NewTypeParamType("R"))),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"T": types.NewTypeTypeWithParam(types.NewTypeParamType("R")),
			},
		},
		{
			name:       "deeplyNestedTypeParam_bindsConcreteType",
			from:       types.NewTypeTypeWithParam(types.NewTypeTypeWithParam(types.IntType)),
			to:         types.NewTypeTypeWithParam(types.NewTypeTypeWithParam(types.NewTypeParamType("T"))),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"T": types.IntType,
			},
		},
		{
			name:       "compositeListTypeParam_bindsConcreteType",
			from:       types.NewTypeTypeWithParam(types.NewListType(types.IntType)),
			to:         types.NewTypeTypeWithParam(types.NewListType(types.NewTypeParamType("T"))),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"T": types.IntType,
			},
		},
		{
			name:       "compositeMapTypeParam_bindsConcreteTypes",
			from:       types.NewTypeTypeWithParam(types.NewMapType(types.StringType, types.IntType)),
			to:         types.NewTypeTypeWithParam(types.NewMapType(types.NewTypeParamType("K"), types.NewTypeParamType("V"))),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"K": types.StringType,
				"V": types.IntType,
			},
		},
		{
			name:       "optionalTypeParam_unifies",
			from:       types.NewTypeTypeWithParam(types.NewOptionalType(types.IntType)),
			to:         types.NewTypeTypeWithParam(types.NewOptionalType(types.NewTypeParamType("T"))),
			wantAssign: true,
			wantSubs: map[string]*types.Type{
				"T": types.IntType,
			},
		},
		{
			name:       "incompatibleTypeParams_returnsNull",
			from:       types.NewTypeTypeWithParam(types.NewListType(types.NewTypeParamType("T"))),
			to:         types.NewTypeTypeWithParam(types.IntType),
			wantAssign: false,
		},
		{
			name: "conflictingBoundTypeParam_returnsNull",
			from: types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			to:   types.NewTypeTypeWithParam(types.IntType),
			setup: func(m *mapping) {
				m.add(types.NewTypeParamType("T"), types.StringType)
			},
			wantAssign: false,
		},
		{
			name:       "occursCheck_failsOnSelfReference",
			from:       types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			to:         types.NewTypeTypeWithParam(types.NewTypeTypeWithParam(types.NewTypeParamType("T"))),
			wantAssign: false,
		},
		{
			name:       "occursCheck_mapTypeParam_to_typeParam",
			from:       types.NewTypeTypeWithParam(types.NewMapType(types.StringType, types.NewTypeParamType("T"))),
			to:         types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			wantAssign: false,
		},
		{
			name:       "occursCheck_typeParam_to_mapTypeParam",
			from:       types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			to:         types.NewTypeTypeWithParam(types.NewMapType(types.StringType, types.NewTypeParamType("T"))),
			wantAssign: false,
		},
		{
			name: "occursCheck_failsOnTransitiveCycle",
			from: types.NewTypeTypeWithParam(types.NewTypeParamType("R")),
			to:   types.NewTypeTypeWithParam(types.NewTypeTypeWithParam(types.NewTypeParamType("T"))),
			setup: func(m *mapping) {
				m.add(types.NewTypeParamType("T"), types.NewTypeTypeWithParam(types.NewTypeParamType("R")))
			},
			wantAssign: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newMapping()
			if tc.setup != nil {
				tc.setup(m)
			}
			result := isAssignable(m, tc.from, tc.to)
			if tc.wantAssign {
				if result == nil {
					t.Fatalf("isAssignable(%v, %v) = nil, want non-nil", tc.from, tc.to)
				}
				for paramName, wantType := range tc.wantSubs {
					gotType, found := result.find(types.NewTypeParamType(paramName))
					if !found {
						t.Errorf("type param %q not found in substitutions", paramName)
					} else if !gotType.IsExactType(wantType) {
						t.Errorf("type param %q = %v, want %v", paramName, gotType, wantType)
					}
				}
			} else if result != nil {
				t.Fatalf("isAssignable(%v, %v) = %v, want nil", tc.from, tc.to, result)
			}
		})
	}
}

func TestIsEqualOrLessSpecific_TypeType(t *testing.T) {
	tests := []struct {
		name string
		t1   *types.Type
		t2   *types.Type
		want bool
	}{
		{
			name: "dyn_less_specific_than_int",
			t1:   types.NewTypeTypeWithParam(types.DynType),
			t2:   types.NewTypeTypeWithParam(types.IntType),
			want: true,
		},
		{
			name: "int_not_less_specific_than_dyn",
			t1:   types.NewTypeTypeWithParam(types.IntType),
			t2:   types.NewTypeTypeWithParam(types.DynType),
			want: false,
		},
		{
			name: "same_type",
			t1:   types.NewTypeTypeWithParam(types.IntType),
			t2:   types.NewTypeTypeWithParam(types.IntType),
			want: true,
		},
		{
			name: "type_param_less_specific",
			t1:   types.NewTypeTypeWithParam(types.NewTypeParamType("T")),
			t2:   types.NewTypeTypeWithParam(types.IntType),
			want: true,
		},
		{
			name: "unparameterized_type_less_specific",
			t1:   types.TypeType,
			t2:   types.NewTypeTypeWithParam(types.IntType),
			want: true,
		},
		{
			name: "parameterized_not_less_specific_than_unparameterized",
			t1:   types.NewTypeTypeWithParam(types.IntType),
			t2:   types.TypeType,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isEqualOrLessSpecific(tc.t1, tc.t2)
			if got != tc.want {
				t.Errorf("isEqualOrLessSpecific(%v, %v) = %v, want %v", tc.t1, tc.t2, got, tc.want)
			}
		})
	}
}
