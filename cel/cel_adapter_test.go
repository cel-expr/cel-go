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

package cel

import (
	"testing"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

type issue1594Version struct {
	Major int64
}

type issue1594PtrVersion struct {
	Major *int64
}

type issue1594Adapter struct {
	types.Adapter
}

func (a issue1594Adapter) NativeToValue(v any) ref.Val {
	switch ver := v.(type) {
	case issue1594Version:
		return types.Int(ver.Major)
	case *issue1594Version:
		if ver == nil {
			return types.NullValue
		}
		return types.Int(ver.Major)
	case issue1594PtrVersion:
		if ver.Major == nil {
			return types.NullValue
		}
		return types.Int(*ver.Major)
	case []issue1594Version:
		return types.NewDynamicList(a, ver)
	case map[string]issue1594Version:
		return types.NewDynamicMap(a, ver)
	}
	return a.Adapter.NativeToValue(v)
}

func TestCustomAdapterInputs_Issue1594(t *testing.T) {
	env, err := NewEnv(
		Variable("versions", ListType(IntType)),
		Variable("versionMap", MapType(StringType, IntType)),
		Variable("intKeyMap", MapType(IntType, IntType)),
		Variable("customKeyMap", MapType(IntType, IntType)),
		Variable("dynVersions", DynType),
		Variable("dynMap", DynType),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	adapter := issue1594Adapter{Adapter: types.DefaultTypeAdapter}
	v1, v2 := int64(1), int64(2)

	listVariants := []struct {
		name string
		list traits.Lister
	}{
		{
			name: "NewDynamicList_Struct",
			list: types.NewDynamicList(adapter, []issue1594Version{{1}, {2}}),
		},
		{
			name: "NewDynamicList_DirectIfaceStruct",
			list: types.NewDynamicList(adapter, []issue1594PtrVersion{{&v1}, {&v2}}),
		},
		{
			name: "NewDynamicList_PtrStruct",
			list: types.NewDynamicList(adapter, []*issue1594Version{{1}, {2}}),
		},
		{
			name: "NewDynamicList_Any",
			list: types.NewDynamicList(adapter, []any{issue1594Version{1}, issue1594Version{2}}),
		},
		{
			name: "NewDynamicList_Array",
			list: types.NewDynamicList(adapter, [2]issue1594Version{{1}, {2}}),
		},
		{
			name: "NewList_Struct",
			list: types.NewList(adapter, []issue1594Version{{1}, {2}}),
		},
		{
			name: "NewList_DirectIfaceStruct",
			list: types.NewList(adapter, []issue1594PtrVersion{{&v1}, {&v2}}),
		},
		{
			name: "NewList_PtrStruct",
			list: types.NewList(adapter, []*issue1594Version{{1}, {2}}),
		},
		{
			name: "NewList_Any",
			list: types.NewList(adapter, []any{issue1594Version{1}, issue1594Version{2}}),
		},
		{
			name: "NewList_RefVal",
			list: types.NewList(adapter, []ref.Val{types.Int(1), types.Int(2)}),
		},
		{
			name: "NewList_ConcreteRefVal",
			list: types.NewList(adapter, []types.Int{types.Int(1), types.Int(2)}),
		},
		{
			name: "MaybeSliceList_Struct",
			list: func() traits.Lister {
				l, _ := types.MaybeSliceList(
					types.DefaultTypeAdapter,
					types.NewDynamicList(adapter, []issue1594Version{{0}, {1}, {2}, {3}}),
					1,
					3,
				)
				return l
			}(),
		},
		{
			name: "MaybeReverseList_Struct",
			list: func() traits.Lister {
				l, _ := types.MaybeReverseList(
					types.DefaultTypeAdapter,
					types.NewDynamicList(adapter, []issue1594Version{{2}, {1}}),
				)
				return l
			}(),
		},
		{
			name: "ConcatList_Struct",
			list: types.NewDynamicList(adapter, []issue1594Version{{1}}).Add(
				types.NewDynamicList(adapter, []issue1594Version{{2}}),
			).(traits.Lister),
		},
	}

	mapVariants := []struct {
		name string
		m    traits.Mapper
	}{
		{
			name: "NewDynamicMap_StringStruct",
			m:    types.NewDynamicMap(adapter, map[string]issue1594Version{"a": {1}, "b": {2}}),
		},
		{
			name: "NewDynamicMap_StringDirectIfaceStruct",
			m:    types.NewDynamicMap(adapter, map[string]issue1594PtrVersion{"a": {&v1}, "b": {&v2}}),
		},
		{
			name: "NewDynamicMap_StringPtrStruct",
			m:    types.NewDynamicMap(adapter, map[string]*issue1594Version{"a": {1}, "b": {2}}),
		},
		{
			name: "NewDynamicMap_StringAny",
			m:    types.NewDynamicMap(adapter, map[string]any{"a": issue1594Version{1}, "b": issue1594Version{2}}),
		},
		{
			name: "NewMap_StringStruct",
			m:    types.NewMap(adapter, map[string]issue1594Version{"a": {1}, "b": {2}}),
		},
		{
			name: "NewMap_StringDirectIfaceStruct",
			m:    types.NewMap(adapter, map[string]issue1594PtrVersion{"a": {&v1}, "b": {&v2}}),
		},
		{
			name: "NewMap_StringPtrStruct",
			m:    types.NewMap(adapter, map[string]*issue1594Version{"a": {1}, "b": {2}}),
		},
		{
			name: "NewMap_StringAny",
			m:    types.NewMap(adapter, map[string]any{"a": issue1594Version{1}, "b": issue1594Version{2}}),
		},
		{
			name: "NewMap_StringRefVal",
			m:    types.NewMap(adapter, map[string]ref.Val{"a": types.Int(1), "b": types.Int(2)}),
		},
		{
			name: "NewMap_RefValRefVal",
			m:    types.NewMap(adapter, map[ref.Val]ref.Val{types.String("a"): types.Int(1), types.String("b"): types.Int(2)}),
		},
	}

	intKeyMapVariants := []struct {
		name string
		m    traits.Mapper
	}{
		{
			name: "NewDynamicMap_Int64Struct",
			m:    types.NewDynamicMap(adapter, map[int64]issue1594Version{10: {1}, 20: {2}}),
		},
		{
			name: "NewMap_Int64Struct",
			m:    types.NewMap(adapter, map[int64]issue1594Version{10: {1}, 20: {2}}),
		},
		{
			name: "NewMap_IntStruct",
			m:    types.NewMap(adapter, map[int]issue1594Version{10: {1}, 20: {2}}),
		},
		{
			name: "NewMap_Int32Struct",
			m:    types.NewMap(adapter, map[int32]issue1594Version{10: {1}, 20: {2}}),
		},
		{
			name: "NewMap_Int64Any",
			m:    types.NewMap(adapter, map[int64]any{10: issue1594Version{1}, 20: issue1594Version{2}}),
		},
	}

	customKeyMapVariants := []struct {
		name string
		m    traits.Mapper
	}{
		{
			name: "NewDynamicMap_CustomKeyStructVal",
			m:    types.NewDynamicMap(adapter, map[issue1594Version]issue1594Version{{1}: {10}, {2}: {20}}),
		},
		{
			name: "NewMap_CustomKeyStructVal",
			m:    types.NewMap(adapter, map[issue1594Version]issue1594Version{{1}: {10}, {2}: {20}}),
		},
		{
			name: "NewMap_CustomKeyPrimitiveVal",
			m:    types.NewMap(adapter, map[issue1594Version]int64{{1}: 10, {2}: 20}),
		},
	}

	evalOptions := []struct {
		name string
		opts []ProgramOption
	}{
		{
			name: "Default",
			opts: nil,
		},
		{
			name: "OptOptimize",
			opts: []ProgramOption{EvalOptions(OptOptimize)},
		},
		{
			name: "OptTrackState",
			opts: []ProgramOption{EvalOptions(OptTrackState)},
		},
		{
			name: "OptExhaustiveEval",
			opts: []ProgramOption{EvalOptions(OptExhaustiveEval)},
		},
	}

	listExprs := []string{
		`versions.all(v, v > 0)`,
		`versions.exists(v, v == 2)`,
		`versions.exists_one(v, v == 2)`,
		`versions.filter(v, v > 1) == [2]`,
		`versions.map(v, v + 1) == [2, 3]`,
		`versions.map(v, v > 1, v * 10) == [20]`,
		`versions[0] == 1 && versions[1] == 2`,
		`2 in versions && !(99 in versions)`,
		`size(versions) == 2`,
		`versions == [1, 2]`,
		`dynVersions.all(v, v > 0)`,
		`dynVersions[0] == 1 && dynVersions[1] == 2`,
	}

	mapExprs := []string{
		`versionMap.all(k, versionMap[k] > 0)`,
		`versionMap.exists(k, versionMap[k] == 2)`,
		`versionMap.exists_one(k, versionMap[k] == 2)`,
		`versionMap.filter(k, versionMap[k] > 1) == ["b"]`,
		`versionMap.map(k, versionMap[k] + 10).exists(x, x == 12)`,
		`versionMap["a"] == 1 && versionMap["b"] == 2`,
		`versionMap.a == 1 && versionMap.b == 2`,
		`has(versionMap.a) && !has(versionMap.c)`,
		`"a" in versionMap && !("c" in versionMap)`,
		`size(versionMap) == 2`,
		`dynMap.all(k, dynMap[k] > 0)`,
		`dynMap["a"] == 1 && dynMap.b == 2`,
	}

	intKeyMapExprs := []string{
		`intKeyMap.all(k, k >= 10 && intKeyMap[k] > 0)`,
		`intKeyMap.exists(k, k == 20 && intKeyMap[k] == 2)`,
		`intKeyMap[10] == 1 && intKeyMap[20] == 2`,
		`10 in intKeyMap && !(99 in intKeyMap)`,
		`size(intKeyMap) == 2`,
	}

	customKeyMapExprs := []string{
		`customKeyMap.all(k, k > 0)`,
		`customKeyMap.exists(k, k == 2)`,
		`customKeyMap.exists_one(k, k == 2)`,
		`customKeyMap.map(k, k * 10).exists(v, v == 20)`,
		`size(customKeyMap) == 2`,
	}

	for _, eo := range evalOptions {
		t.Run("ListVariants/"+eo.name, func(t *testing.T) {
			for _, lv := range listVariants {
				t.Run(lv.name, func(t *testing.T) {
					vars := map[string]any{
						"versions":    lv.list,
						"dynVersions": lv.list,
					}
					for _, expr := range listExprs {
						ast, iss := env.Compile(expr)
						if iss.Err() != nil {
							t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
						}
						prg, err := env.Program(ast, eo.opts...)
						if err != nil {
							t.Fatalf("Program(%q) failed: %v", expr, err)
						}
						out, _, err := prg.Eval(vars)
						if err != nil {
							t.Fatalf("Eval(%q) failed: %v", expr, err)
						}
						if out != types.True {
							t.Errorf("Eval(%q) = %v, wanted true", expr, out)
						}
					}
				})
			}
		})

		t.Run("MapVariants/"+eo.name, func(t *testing.T) {
			for _, mv := range mapVariants {
				t.Run(mv.name, func(t *testing.T) {
					vars := map[string]any{
						"versionMap": mv.m,
						"dynMap":     mv.m,
					}
					for _, expr := range mapExprs {
						ast, iss := env.Compile(expr)
						if iss.Err() != nil {
							t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
						}
						prg, err := env.Program(ast, eo.opts...)
						if err != nil {
							t.Fatalf("Program(%q) failed: %v", expr, err)
						}
						out, _, err := prg.Eval(vars)
						if err != nil {
							t.Fatalf("Eval(%q) failed: %v", expr, err)
						}
						if out != types.True {
							t.Errorf("Eval(%q) = %v, wanted true", expr, out)
						}
					}
				})
			}
		})

		t.Run("IntKeyMapVariants/"+eo.name, func(t *testing.T) {
			for _, mv := range intKeyMapVariants {
				t.Run(mv.name, func(t *testing.T) {
					vars := map[string]any{
						"intKeyMap": mv.m,
					}
					for _, expr := range intKeyMapExprs {
						ast, iss := env.Compile(expr)
						if iss.Err() != nil {
							t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
						}
						prg, err := env.Program(ast, eo.opts...)
						if err != nil {
							t.Fatalf("Program(%q) failed: %v", expr, err)
						}
						out, _, err := prg.Eval(vars)
						if err != nil {
							t.Fatalf("Eval(%q) failed: %v", expr, err)
						}
						if out != types.True {
							t.Errorf("Eval(%q) = %v, wanted true", expr, out)
						}
					}
				})
			}
		})

		t.Run("CustomKeyMapVariants/"+eo.name, func(t *testing.T) {
			for _, mv := range customKeyMapVariants {
				t.Run(mv.name, func(t *testing.T) {
					vars := map[string]any{
						"customKeyMap": mv.m,
					}
					for _, expr := range customKeyMapExprs {
						ast, iss := env.Compile(expr)
						if iss.Err() != nil {
							t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
						}
						prg, err := env.Program(ast, eo.opts...)
						if err != nil {
							t.Fatalf("Program(%q) failed: %v", expr, err)
						}
						out, _, err := prg.Eval(vars)
						if err != nil {
							t.Fatalf("Eval(%q) failed: %v", expr, err)
						}
						if out != types.True {
							t.Errorf("Eval(%q) = %v, wanted true", expr, out)
						}
					}
				})
			}
		})
	}

	t.Run("EnvLevelCustomTypeAdapter", func(t *testing.T) {
		customEnv, err := NewEnv(
			CustomTypeAdapter(adapter),
			Variable("versions", ListType(IntType)),
			Variable("versionMap", MapType(StringType, IntType)),
			Variable("singleVer", IntType),
		)
		if err != nil {
			t.Fatalf("NewEnv() failed: %v", err)
		}
		vars := map[string]any{
			"versions":   []issue1594Version{{1}, {2}},
			"versionMap": map[string]issue1594Version{"a": {1}, "b": {2}},
			"singleVer":  issue1594Version{1},
		}
		for _, expr := range []string{
			`singleVer == 1`,
			`versions.all(v, v > 0)`,
			`versions[0] == 1 && versions[1] == 2`,
			`versionMap.all(k, versionMap[k] > 0)`,
			`versionMap["a"] == 1 && versionMap.b == 2`,
		} {
			ast, iss := customEnv.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
			}
			prg, err := customEnv.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			out, _, err := prg.Eval(vars)
			if err != nil {
				t.Fatalf("Eval(%q) failed: %v", expr, err)
			}
			if out != types.True {
				t.Errorf("Eval(%q) = %v, wanted true", expr, out)
			}
		}
	})

	t.Run("ComposedNativeAndCustomAdapter", func(t *testing.T) {
		type nativeChild struct {
			Label string
		}
		type nativeContainer struct {
			Name        string
			Children    []nativeChild
			PtrChildren []*nativeChild
		}
		baseReg, err := types.NewRegistry()
		if err != nil {
			t.Fatalf("NewRegistry() failed: %v", err)
		}
		prov, adapt, err := types.ComposeTypes(baseReg, adapter, types.NativeTypeFor[nativeContainer]())
		if err != nil {
			t.Fatalf("ComposeTypes() failed: %v", err)
		}
		compEnv, err := NewEnv(
			CustomTypeProvider(prov),
			CustomTypeAdapter(adapt),
			Variable("versions", ListType(IntType)),
			Variable("versionMap", MapType(StringType, IntType)),
			Variable("container", ObjectType("cel.nativeContainer")),
		)
		if err != nil {
			t.Fatalf("NewEnv() failed: %v", err)
		}
		vars := map[string]any{
			"versions":   types.NewDynamicList(adapt, []issue1594Version{{Major: 1}, {Major: 2}}),
			"versionMap": types.NewDynamicMap(adapt, map[string]issue1594Version{"a": {Major: 1}, "b": {Major: 2}}),
			"container": nativeContainer{
				Name:        "release",
				Children:    []nativeChild{{Label: "x"}, {Label: "y"}},
				PtrChildren: []*nativeChild{{Label: "x"}, {Label: "y"}},
			},
		}
		for _, expr := range []string{
			`versions.all(v, v > 0)`,
			`versions[1] == 2`,
			`versionMap.all(k, versionMap[k] > 0)`,
			`versionMap["a"] == 1`,
			`container.Name == "release"`,
			`container.Children.exists(c, c.Label == "y")`,
			`container.Children[0].Label == "x"`,
			`container.PtrChildren.exists(c, c.Label == "y")`,
			`container.PtrChildren[0].Label == "x"`,
		} {
			ast, iss := compEnv.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
			}
			prg, err := compEnv.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			out, _, err := prg.Eval(vars)
			if err != nil {
				t.Fatalf("Eval(%q) failed: %v", expr, err)
			}
			if out != types.True {
				t.Errorf("Eval(%q) = %v, wanted true", expr, out)
			}
		}
	})
}
