// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ext

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/env"
)

func TestExtensionOptionFactoryInvalidExtension(t *testing.T) {
	invalidExtension := "invalid extension"
	_, validExtension := ExtensionOptionFactory(invalidExtension)
	if validExtension {
		t.Fatalf("ExtensionOptionFactory(%s) returned valid extension for invalid input", invalidExtension)
	}
	var nilExt *env.Extension
	if _, valid := ExtensionOptionFactory(nilExt); valid {
		t.Fatalf("ExtensionOptionFactory(nil) returned valid extension")
	}
}

func TestExtensionOptionFactoryInvalidExtensionName(t *testing.T) {
	e := &env.Extension{Name: "invalid extension name"}
	_, validExtension := ExtensionOptionFactory(e)
	if validExtension {
		t.Fatalf("ExtensionOptionFactory(%s) returned valid extension for invalid extension name", e.Name)
	}
}

func TestExtensionOptionFactoryInvalidExtensionVersion(t *testing.T) {
	e := &env.Extension{Name: "bindings", Version: "invalid version"}
	opt, validExtension := ExtensionOptionFactory(e)
	if !validExtension {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
	_, err := cel.NewCustomEnv(opt)
	if err == nil || err.Error() != fmt.Sprintf("invalid extension version: %s - %s", e.Name, e.Version) {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension version", e.Name)
	}
}

func TestExtensionOptionFactoryValidBindingsExtension(t *testing.T) {
	e := &env.Extension{Name: "cel.lib.ext.cel.bindings", Version: "latest"}
	opt, validExtension := ExtensionOptionFactory(e)
	if !validExtension {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
	en, err := cel.NewCustomEnv(opt)
	if err != nil {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
	cfg, err := en.ToConfig("test config")
	if err != nil {
		t.Fatalf("ToConfig(%s) returned error: %v", e.Name, err)
	}
	if len(cfg.Extensions) != 1 || cfg.Extensions[0].Name != "cel.lib.ext.cel.bindings" || cfg.Extensions[0].Version != "latest" {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
}

func TestExtensionOptionFactoryValidBindingsExtensionAlias(t *testing.T) {
	e := &env.Extension{Name: "bindings", Version: "latest"}
	opt, validExtension := ExtensionOptionFactory(e)
	if !validExtension {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
	en, err := cel.NewCustomEnv(opt)
	if err != nil {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
	cfg, err := en.ToConfig("test config")
	if err != nil {
		t.Fatalf("ToConfig(%s) returned error: %v", e.Name, err)
	}
	if len(cfg.Extensions) != 1 || cfg.Extensions[0].Name != "cel.lib.ext.cel.bindings" || cfg.Extensions[0].Version != "latest" {
		t.Fatalf("ExtensionOptionFactory(%s) returned invalid extension", e.Name)
	}
}

func TestExtensionOptionFactoryStringsConfig(t *testing.T) {
	t.Run("locale", func(t *testing.T) {
		e := env.NewExtension("strings", 3).SetConfig(map[string]any{"locale": "de_DE"})
		opt, ok := ExtensionOptionFactory(e)
		if !ok {
			t.Fatalf("ExtensionOptionFactory(%s) returned false", e.Name)
		}
		en, err := cel.NewEnv(opt)
		if err != nil {
			t.Fatalf("cel.NewEnv() failed: %v", err)
		}
		ast, iss := en.Compile(`"%.2f".format([1.5])`)
		if iss.Err() != nil {
			t.Fatalf("Compile() failed: %v", iss.Err())
		}
		prg, err := en.Program(ast)
		if err != nil {
			t.Fatalf("Program() failed: %v", err)
		}
		out, _, err := prg.Eval(cel.NoVars())
		if err != nil {
			t.Fatalf("Eval() failed: %v", err)
		}
		if out.Value() != "1,50" {
			t.Fatalf("Eval() got %v, wanted '1,50'", out.Value())
		}
	})

	for _, tc := range []struct {
		name string
		val  any
	}{
		{name: "int", val: 2},
		{name: "json_float64", val: float64(2)},
	} {
		t.Run("max_precision_"+tc.name, func(t *testing.T) {
			e := env.NewExtension("strings", 5).SetConfig(map[string]any{"max_precision": tc.val})
			opt, ok := ExtensionOptionFactory(e)
			if !ok {
				t.Fatalf("ExtensionOptionFactory(%s) returned false", e.Name)
			}
			en, err := cel.NewEnv(opt)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			if _, iss := en.Compile(`"%.2f".format([1.5])`); iss.Err() != nil {
				t.Fatalf("Compile() within max_precision failed: %v", iss.Err())
			}
			if _, iss := en.Compile(`"%.5f".format([1.5])`); iss.Err() == nil {
				t.Fatalf("Compile() exceeding max_precision unexpectedly succeeded")
			}
		})
	}
}

func TestExtensionOptionFactoryListsConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		val  any
	}{
		{name: "int", val: 3},
		{name: "int64", val: int64(3)},
		{name: "uint", val: uint(3)},
		{name: "uint64", val: uint64(3)},
		{name: "json_float64", val: float64(3)},
	} {
		t.Run("max_range_size_"+tc.name, func(t *testing.T) {
			e := env.NewExtension("lists", 2).SetConfig(map[string]any{"max_range_size": tc.val})
			opt, ok := ExtensionOptionFactory(e)
			if !ok {
				t.Fatalf("ExtensionOptionFactory(%s) returned false", e.Name)
			}
			en, err := cel.NewEnv(opt)
			if err != nil {
				t.Fatalf("cel.NewEnv() failed: %v", err)
			}
			ast, iss := en.Compile(`lists.range(3)`)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			prg, err := en.Program(ast)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			if _, _, err := prg.Eval(cel.NoVars()); err != nil {
				t.Fatalf("Eval() within max_range_size failed: %v", err)
			}

			astOver, iss := en.Compile(`lists.range(4)`)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			prgOver, err := en.Program(astOver)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			if _, _, err := prgOver.Eval(cel.NoVars()); err == nil {
				t.Fatalf("Eval() exceeding max_range_size unexpectedly succeeded")
			}
		})
	}
}

func TestExtensionOptionFactoryConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		ext     *env.Extension
		wantErr string
	}{
		{
			name:    "unknown key on bindings",
			ext:     env.NewExtension("bindings", 0).SetConfig(map[string]any{"foo": "bar"}),
			wantErr: `unknown key "foo"`,
		},
		{
			name:    "unknown key on strings",
			ext:     env.NewExtension("strings", 3).SetConfig(map[string]any{"bad": 1}),
			wantErr: `unknown key "bad"`,
		},
		{
			name:    "unknown key on lists",
			ext:     env.NewExtension("lists", 2).SetConfig(map[string]any{"bad": 1}),
			wantErr: `unknown key "bad"`,
		},
		{
			name:    "invalid locale type",
			ext:     env.NewExtension("strings", 3).SetConfig(map[string]any{"locale": 123}),
			wantErr: `"locale" must be a string`,
		},
		{
			name:    "invalid max_precision fractional",
			ext:     env.NewExtension("strings", 5).SetConfig(map[string]any{"max_precision": 1.5}),
			wantErr: `not a whole number`,
		},
		{
			name:    "negative max_precision",
			ext:     env.NewExtension("strings", 5).SetConfig(map[string]any{"max_precision": -1}),
			wantErr: `must be non-negative`,
		},
		{
			name:    "negative max_range_size",
			ext:     env.NewExtension("lists", 2).SetConfig(map[string]any{"max_range_size": -1}),
			wantErr: `must be non-negative`,
		},
		{
			name:    "invalid max_range_size type",
			ext:     env.NewExtension("lists", 2).SetConfig(map[string]any{"max_range_size": "3"}),
			wantErr: `unsupported "max_range_size" type`,
		},
		{
			name:    "max_range_size uint64 overflow",
			ext:     env.NewExtension("lists", 2).SetConfig(map[string]any{"max_range_size": uint64(math.MaxUint64)}),
			wantErr: `out of int64 range`,
		},
		{
			name:    "max_range_size float64 overflow",
			ext:     env.NewExtension("lists", 2).SetConfig(map[string]any{"max_range_size": float64(math.MaxInt64)}),
			wantErr: `out of int64 range`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opt, ok := ExtensionOptionFactory(tc.ext)
			if !ok {
				t.Fatalf("ExtensionOptionFactory(%s) returned false", tc.ext.Name)
			}
			_, err := cel.NewCustomEnv(opt)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("cel.NewCustomEnv() got %v, wanted error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestExtensionOptionFactorySubset(t *testing.T) {
	t.Run("strings_exclude_function", func(t *testing.T) {
		subset := env.NewLibrarySubset().AddExcludedFunctions(&env.Function{Name: "charAt"})
		conf := env.NewConfig("strings-subset").
			AddExtensions(env.NewExtension("strings", 3).SetSubset(subset))
		en, err := cel.NewEnv(cel.FromConfig(conf, ExtensionOptionFactory))
		if err != nil {
			t.Fatalf("cel.NewEnv(FromConfig()) failed: %v", err)
		}
		if _, iss := en.Compile(`"hello".replace("l", "x")`); iss.Err() != nil {
			t.Fatalf("Compile(replace) failed: %v", iss.Err())
		}
		if _, iss := en.Compile(`"hello".charAt(1)`); iss.Err() == nil {
			t.Fatalf("Compile(charAt) succeeded, wanted error for excluded function")
		}
		gotCfg, err := en.ToConfig("strings-subset")
		if err != nil {
			t.Fatalf("en.ToConfig() failed: %v", err)
		}
		if len(gotCfg.Extensions) != 1 || !reflect.DeepEqual(gotCfg.Extensions[0].Subset, subset) {
			t.Fatalf("ToConfig().Extensions = %v, wanted subset %v", gotCfg.Extensions, subset)
		}
	})

	t.Run("math_exclude_macro", func(t *testing.T) {
		subset := env.NewLibrarySubset().AddExcludedMacros("least")
		conf := env.NewConfig("math-subset").
			AddExtensions(env.NewExtension("math", math.MaxUint32).SetSubset(subset))
		en, err := cel.NewEnv(cel.FromConfig(conf, ExtensionOptionFactory))
		if err != nil {
			t.Fatalf("cel.NewEnv(FromConfig()) failed: %v", err)
		}
		if _, iss := en.Compile(`math.greatest(1, 2)`); iss.Err() != nil {
			t.Fatalf("Compile(math.greatest) failed: %v", iss.Err())
		}
		if _, iss := en.Compile(`math.least(1, 2)`); iss.Err() == nil {
			t.Fatalf("Compile(math.least) succeeded, wanted error for excluded macro")
		}
	})

	t.Run("lists_exclude_function", func(t *testing.T) {
		subset := env.NewLibrarySubset().AddExcludedFunctions(&env.Function{Name: "lists.range"})
		conf := env.NewConfig("lists-subset").
			AddExtensions(env.NewExtension("lists", 2).SetSubset(subset))
		en, err := cel.NewEnv(cel.FromConfig(conf, ExtensionOptionFactory))
		if err != nil {
			t.Fatalf("cel.NewEnv(FromConfig()) failed: %v", err)
		}
		if _, iss := en.Compile(`[[1], [2]].flatten()`); iss.Err() != nil {
			t.Fatalf("Compile(flatten) failed: %v", iss.Err())
		}
		if _, iss := en.Compile(`lists.range(3)`); iss.Err() == nil {
			t.Fatalf("Compile(lists.range) succeeded, wanted error for excluded function")
		}
	})

	t.Run("disabled_extension", func(t *testing.T) {
		conf := env.NewConfig("strings-disabled").
			AddExtensions(env.NewExtension("strings", 3).SetSubset(env.NewLibrarySubset().SetDisabled(true)))
		en, err := cel.NewEnv(cel.FromConfig(conf, ExtensionOptionFactory))
		if err != nil {
			t.Fatalf("cel.NewEnv(FromConfig()) failed: %v", err)
		}
		if en.HasLibrary("cel.lib.ext.strings") {
			t.Fatalf("en.HasLibrary('cel.lib.ext.strings') = true, wanted false")
		}
		if _, iss := en.Compile(`"hello".charAt(1)`); iss.Err() == nil {
			t.Fatalf("Compile(charAt) succeeded on disabled extension")
		}
	})

	t.Run("invalid_subset", func(t *testing.T) {
		subset := env.NewLibrarySubset().AddIncludedMacros("least").AddExcludedMacros("greatest")
		e := env.NewExtension("math", math.MaxUint32).SetSubset(subset)
		opt, ok := ExtensionOptionFactory(e)
		if !ok {
			t.Fatalf("ExtensionOptionFactory() returned false")
		}
		if _, err := cel.NewCustomEnv(opt); err == nil {
			t.Fatalf("cel.NewCustomEnv() succeeded, wanted invalid subset error")
		}
	})
}
