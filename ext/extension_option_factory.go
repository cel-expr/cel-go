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
	"slices"
	"sync"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/env"
)

// ExtensionOptionFactory converts an ExtensionConfig value to a CEL environment option.
func ExtensionOptionFactory(configElement any) (cel.EnvOption, bool) {
	ext, isExtension := configElement.(*env.Extension)
	if !isExtension || ext == nil {
		return nil, false
	}
	name := ext.Name
	alias, found := extAliases()[name]
	if found {
		name = alias
	}
	fac, found := extFactories()[name]
	if !found {
		return nil, false
	}
	// If the version is 'latest', set the version value to the max uint.
	ver, err := ext.VersionNumber()
	if err != nil {
		return func(*cel.Env) (*cel.Env, error) {
			return nil, fmt.Errorf("invalid extension version: %s - %s", ext.Name, ext.Version)
		}, true
	}
	opt, err := fac(ver, ext)
	if err != nil {
		return func(*cel.Env) (*cel.Env, error) {
			return nil, err
		}, true
	}
	return opt, true
}

// extensionFactory accepts a version and extension config and produces a CEL environment option.
type extensionFactory func(uint32, *env.Extension) (cel.EnvOption, error)

var (
	extFactoriesOnce sync.Once
	extFactoriesMap  map[string]extensionFactory
	extAliasesMap    map[string]string
)

func extFactories() map[string]extensionFactory {
	extFactoriesOnce.Do(initExtFactories)
	return extFactoriesMap
}

func extAliases() map[string]string {
	extFactoriesOnce.Do(initExtFactories)
	return extAliasesMap
}

func initExtFactories() {
	extFactoriesMap = map[string]extensionFactory{
		"cel.lib.ext.cel.bindings": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Bindings(BindingsVersion(version)), nil
		},
		"cel.lib.ext.encoders": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Encoders(EncodersVersion(version)), nil
		},
		"cel.lib.ext.lists": listsExtensionFactory,
		"cel.lib.ext.math": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Math(MathVersion(version)), nil
		},
		"cel.lib.ext.protos": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Protos(ProtosVersion(version)), nil
		},
		"cel.lib.ext.sets": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Sets(SetsVersion(version)), nil
		},
		"cel.lib.ext.strings": stringsExtensionFactory,
		"cel.lib.ext.comprev2": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return TwoVarComprehensions(TwoVarComprehensionsVersion(version)), nil
		},
		"cel.lib.ext.regex": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Regex(RegexVersion(version)), nil
		},
		"cel.lib.ext.markdown": func(version uint32, ext *env.Extension) (cel.EnvOption, error) {
			if err := validateAllowedConfigKeys(ext); err != nil {
				return nil, err
			}
			return Markdown(MarkdownVersion(version)), nil
		},
	}

	extAliasesMap = map[string]string{
		"bindings":               "cel.lib.ext.cel.bindings",
		"encoders":               "cel.lib.ext.encoders",
		"lists":                  "cel.lib.ext.lists",
		"math":                   "cel.lib.ext.math",
		"protos":                 "cel.lib.ext.protos",
		"sets":                   "cel.lib.ext.sets",
		"strings":                "cel.lib.ext.strings",
		"two-var-comprehensions": "cel.lib.ext.comprev2",
		"regex":                  "cel.lib.ext.regex",
		"markdown":               "cel.lib.ext.markdown",
	}
}

func listsExtensionFactory(version uint32, ext *env.Extension) (cel.EnvOption, error) {
	if err := validateAllowedConfigKeys(ext, "max_range_size"); err != nil {
		return nil, err
	}
	opts := []ListsOption{ListsVersion(version)}
	if raw, found := ext.ConfigValue("max_range_size"); found {
		size, err := extensionIntConfig(ext, "max_range_size", raw)
		if err != nil {
			return nil, err
		}
		if size < 0 {
			return nil, fmt.Errorf("invalid extension %q config: %q must be non-negative, got %d", ext.Name, "max_range_size", size)
		}
		opts = append(opts, ListsMaxRangeSize(size))
	}
	return Lists(opts...), nil
}

func stringsExtensionFactory(version uint32, ext *env.Extension) (cel.EnvOption, error) {
	if err := validateAllowedConfigKeys(ext, "locale", "max_precision"); err != nil {
		return nil, err
	}
	opts := []StringsOption{StringsVersion(version)}
	if raw, found := ext.ConfigValue("locale"); found {
		locale, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid extension %q config: %q must be a string, got %T", ext.Name, "locale", raw)
		}
		opts = append(opts, StringsLocale(locale))
	}
	if raw, found := ext.ConfigValue("max_precision"); found {
		limit, err := extensionIntConfig(ext, "max_precision", raw)
		if err != nil {
			return nil, err
		}
		if limit < 0 {
			return nil, fmt.Errorf("invalid extension %q config: %q must be non-negative, got %d", ext.Name, "max_precision", limit)
		}
		if limit > math.MaxInt {
			return nil, fmt.Errorf("invalid extension %q config: %q out of int range: %d", ext.Name, "max_precision", limit)
		}
		opts = append(opts, StringsMaxPrecision(int(limit)))
	}
	return Strings(opts...), nil
}

func validateAllowedConfigKeys(ext *env.Extension, allowed ...string) error {
	if len(ext.Config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(ext.Config))
	for k := range ext.Config {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(allowed, k) {
			return fmt.Errorf("invalid extension %q config: unknown key %q", ext.Name, k)
		}
	}
	return nil
}

func extensionIntConfig(ext *env.Extension, key string, raw any) (int64, error) {
	switch v := raw.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, fmt.Errorf("invalid extension %q config: %q out of int64 range: %d", ext.Name, key, v)
		}
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("invalid extension %q config: %q out of int64 range: %d", ext.Name, key, v)
		}
		return int64(v), nil
	case float64:
		// JSON and google.protobuf.Value decode numeric values as float64.
		if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
			return 0, fmt.Errorf("invalid extension %q config: %q value is not a whole number: %v", ext.Name, key, raw)
		}
		if v < math.MinInt64 || v >= math.MaxInt64 {
			return 0, fmt.Errorf("invalid extension %q config: %q out of int64 range: %v", ext.Name, key, raw)
		}
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid extension %q config: unsupported %q type %T", ext.Name, key, raw)
	}
}
