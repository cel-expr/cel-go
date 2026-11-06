// Copyright 2023 Google LLC
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
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/interpreter"

	proto3pb "cel.dev/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestConstantFoldingOptimizer(t *testing.T) {
	tests := []struct {
		expr        string
		folded      string
		knownValues map[string]any
		vars        map[string]any
	}{
		{
			expr:   `[1, 1 + 2, 1 + (2 + 3)]`,
			folded: `[1, 3, 6]`,
		},
		{
			expr:   `[1, 2] + [3, 4]`,
			folded: `[1, 2, 3, 4]`,
		},
		{
			expr:   `[1, ?optional.of(2)] + [3, 4]`,
			folded: `[1, 2, 3, 4]`,
		},
		{
			expr:   `[1, ?optional.none()] + [2]`,
			folded: `[1, 2]`,
		},
		{
			expr:   `[x, 1] + [2, y]`,
			folded: `[x, 1, 2, y]`,
		},
		{
			expr:   `[x, ?optional.of(1)] + [?optional.of(2), y]`,
			folded: `[x, 1, 2, y]`,
		},
		{
			expr:   `[1] + [x] + [2]`,
			folded: `[1, x, 2]`,
		},
		{
			expr:   `[1] + [?x] + [2]`,
			folded: `[1, ?x, 2]`,
			vars:   map[string]any{"x": types.OptionalOf(types.Int(99))},
		},
		{
			expr:   `[?x, 1] + [2, ?y]`,
			folded: `[?x, 1, 2, ?y]`,
			vars:   map[string]any{"x": types.OptionalOf(types.Int(10)), "y": types.OptionalOf(types.Int(20))},
		},
		{
			expr:   `6 in [1, 1 + 2, 1 + (2 + 3)]`,
			folded: `true`,
		},
		{
			expr:   `5 in [1, 1 + 2, 1 + (2 + 3)]`,
			folded: `false`,
		},
		{
			expr:   `x in [1, 1 + 2, 1 + (2 + 3)]`,
			folded: `x in [1, 3, 6]`,
			vars:   map[string]any{"x": int64(3)},
		},
		{
			expr:   `1 in [1, x + 2, 1 + (2 + 3)]`,
			folded: `true`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `1 in [x, x + 2, 1 + (2 + 3)]`,
			folded: `1 in [x, x + 2, 6]`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `x in []`,
			folded: `false`,
			vars:   map[string]any{"x": int64(1)},
		},
		{
			expr:   `optional.none() in [?optional.none()]`,
			folded: `false`,
		},
		{
			expr:   `1 in [?optional.of(1), 2]`,
			folded: `true`,
		},
		{
			expr:   `3 in [?optional.of(1), 2]`,
			folded: `false`,
		},
		{
			expr:   `x in [?optional.of(1), 2]`,
			folded: `x in [1, 2]`,
			vars:   map[string]any{"x": int64(3)},
		},
		{
			expr:   `3 in [?optional.of(1), x]`,
			folded: `3 in [1, x]`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `optional.of(1) in [optional.of(1)]`,
			folded: `true`,
		},
		{
			expr:   `optional.none() in [optional.of(1)]`,
			folded: `false`,
		},
		{
			expr:   `{'hello': 'world'}.hello == x`,
			folded: `"world" == x`,
			vars:   map[string]any{"x": "world"},
		},
		{
			expr:   `{'hello': 'world'}.?hello.orValue('default') == x`,
			folded: `"world" == x`,
			vars:   map[string]any{"x": "world"},
		},
		{
			expr:   `{'hello': 'world'}['hello'] == x`,
			folded: `"world" == x`,
			vars:   map[string]any{"x": "world"},
		},
		{
			expr:   `optional.of("hello")`,
			folded: `optional.of("hello")`,
		},
		{
			expr:   `optional.ofNonZeroValue("")`,
			folded: `optional.none()`,
		},
		{
			expr:   `{?'hello': optional.of('world')}['hello'] == x`,
			folded: `"world" == x`,
			vars:   map[string]any{"x": "world"},
		},
		{
			expr:   `duration(string(7 * 24) + 'h')`,
			folded: `duration("604800s")`,
		},
		{
			expr:   `timestamp("1970-01-01T00:00:00Z")`,
			folded: `timestamp("1970-01-01T00:00:00Z")`,
		},
		{
			expr:   `[1, 1 + 1, 1 + 2, 2 + 3].exists(i, i < 10)`,
			folded: `true`,
		},
		{
			expr:   `[1, 1 + 1, 1 + 2, 2 + 3].exists(i, i < 1 % 2)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j))`,
			folded: `[[1, 2, 3], [2, 4, 6], [3, 6, 9]]`,
		},
		{
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j).filter(k, k % 2 == 0))`,
			folded: `[[2], [2, 4, 6], [6]]`,
		},
		{
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j).filter(k, k % 2 == x))`,
			folded: `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j).filter(k, k % 2 == x))`,
			vars:   map[string]any{"x": int64(0)},
		},
		{
			expr:   `[(x - 1 > 3) ? 1 : 2].all(x, x < .x)`,
			folded: `[(x - 1 > 3) ? 1 : 2].all(x, x < .x)`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `[(x - 1 > 3) ? (x - 1) : 5].exists(x, x - 1 > 3)`,
			folded: `[(x - 1 > 3) ? (x - 1) : 5].exists(x, x - 1 > 3)`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].filter(m, has(m.a))`,
			folded: `[{"a": 1}]`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].filter(m, has({'a': true}.a))`,
			folded: `[{}, {"a": 1}, {"b": 2}]`,
		},
		{
			expr:   `[1, 2].filter(e, false)`,
			folded: `[]`,
		},
		{
			expr:   `[1, 2].filter(e, true)`,
			folded: `[1, 2]`,
		},
		{
			expr:   `[1, 2].exists(e, false)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2].exists(e, true)`,
			folded: `true`,
		},
		{
			expr:   `[1].all(e, true)`,
			folded: `true`,
		},
		{
			expr:   `[1].all(e, false)`,
			folded: `false`,
		},
		{
			expr:   `{1: 'a'}.filter(e, false)`,
			folded: `[]`,
		},
		{
			expr:   `{1: 'a'}.filter(e, true)`,
			folded: `[1]`,
		},
		{
			expr:   `x.filter(e, false)`,
			folded: `x.filter(e, false)`,
			vars:   map[string]any{"x": []int{1, 2}},
		},
		{
			expr:   `x.exists(e, false)`,
			folded: `x.exists(e, false)`,
			vars:   map[string]any{"x": []int{1, 2}},
		},
		{
			expr:   `x.all(e, true)`,
			folded: `x.all(e, true)`,
			vars:   map[string]any{"x": []int{1, 2}},
		},
		{
			expr:   `type(1)`,
			folded: `int`,
		},
		{
			expr:   `[google.expr.proto3.test.TestAllTypes{single_int32: 2 + 3}].map(i, i)[0]`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: 5}`,
		},
		{
			expr:   `[?optional.ofNonZeroValue(0)]`,
			folded: `[]`,
		},
		{
			expr:   `[1, ?optional.ofNonZeroValue(0)]`,
			folded: `[1]`,
		},
		{
			expr:   `[optional.none(), ?x]`,
			folded: `[optional.none(), ?x]`,
			vars:   map[string]any{"x": types.OptionalOf(types.Int(42))},
		},
		{
			expr:   `[?optional.none(), ?x]`,
			folded: `[?x]`,
			vars:   map[string]any{"x": types.OptionalOf(types.Int(42))},
		},
		{
			expr:   `[?optional.of(1), ?x]`,
			folded: `[1, ?x]`,
			vars:   map[string]any{"x": types.OptionalOf(types.Int(42))},
		},
		{
			expr:   `[1, x, ?optional.ofNonZeroValue(0), ?x.?y]`,
			folded: `[1, x, ?x.?y]`,
			vars:   map[string]any{"x": map[string]any{"y": "val"}},
		},
		{
			expr:   `[1, x, ?optional.ofNonZeroValue(3), ?x.?y]`,
			folded: `[1, x, 3, ?x.?y]`,
			vars:   map[string]any{"x": map[string]any{"y": "val"}},
		},
		{
			expr:   `[1, x, ?optional.ofNonZeroValue(3), ?x.?y].size() > 3`,
			folded: `[1, x, 3, ?x.?y].size() > 3`,
			vars:   map[string]any{"x": map[string]any{"y": "val"}},
		},
		{
			expr:   `{?'a': optional.of('hello'), ?x : optional.of(1), ?'b': optional.none()}`,
			folded: `{"a": "hello", ?x: optional.of(1)}`,
			vars:   map[string]any{"x": types.OptionalOf(types.String("k"))},
		},
		{
			expr:   `true ? x + 1 : x + 2`,
			folded: `x + 1`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `false ? x + 1 : x + 2`,
			folded: `x + 2`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `false ? x + 'world' : 'hello' + 'world'`,
			folded: `"helloworld"`,
			vars:   map[string]any{"x": "test"},
		},
		{
			expr:   `x == true`,
			folded: `x == true`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `true == x`,
			folded: `true == x`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `x != false`,
			folded: `x != false`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `false != x`,
			folded: `false != x`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `x ? 1 + 2 : 3 + 4`,
			folded: `x ? 3 : 7`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `true && x`,
			folded: `true && x`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `x && true`,
			folded: `x && true`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `false && x`,
			folded: `false`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `x && false`,
			folded: `false`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `true || x`,
			folded: `true`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `x || true`,
			folded: `true`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `false || x`,
			folded: `false || x`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `x || false`,
			folded: `x || false`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `true && b`,
			folded: `b`,
			vars:   map[string]any{"b": true},
		},
		{
			expr:   `b && true`,
			folded: `b`,
			vars:   map[string]any{"b": true},
		},
		{
			expr:   `false || b`,
			folded: `b`,
			vars:   map[string]any{"b": false},
		},
		{
			expr:   `b || false`,
			folded: `b`,
			vars:   map[string]any{"b": false},
		},
		{
			expr:   `false || x`,
			folded: `false || x`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `x || false`,
			folded: `x || false`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `true && x`,
			folded: `true && x`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `x && true`,
			folded: `x && true`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `true && x && true && x`,
			folded: `true && x && true && x`,
			vars:   map[string]any{"x": true},
		},
		{
			expr:   `false || x || false || x`,
			folded: `false || x || false || x`,
			vars:   map[string]any{"x": false},
		},
		{
			expr:   `true && b && true && b`,
			folded: `b && b`,
			vars:   map[string]any{"b": true},
		},
		{
			expr:   `false || b || false || b`,
			folded: `b || b`,
			vars:   map[string]any{"b": false},
		},
		{
			expr:   `true && true`,
			folded: `true`,
		},
		{
			expr:   `true && false`,
			folded: `false`,
		},
		{
			expr:   `true || false`,
			folded: `true`,
		},
		{
			expr:   `false || false`,
			folded: `false`,
		},
		{
			expr:   `true && false || true`,
			folded: `true`,
		},
		{
			expr:   `false && true || false`,
			folded: `false`,
		},
		{
			expr:   `null`,
			folded: `null`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.ofNonZeroValue(1)}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.ofNonZeroValue(0)}`,
			folded: `google.expr.proto3.test.TestAllTypes{}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{single_int32: x, repeated_int32: [1, 2, 3]}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: x, repeated_int32: [1, 2, 3]}`,
			vars:   map[string]any{"x": int32(5)},
		},
		{
			expr:   `x + dyn([1, 2] + [3, 4])`,
			folded: `x + [1, 2, 3, 4]`,
			vars:   map[string]any{"x": []int{0}},
		},
		{
			expr:   `dyn([1, 2]) + [3.0, 4.0]`,
			folded: `[1, 2, 3.0, 4.0]`,
		},
		{
			expr:   `{'a': dyn([1, 2]), 'b': x}`,
			folded: `{"a": [1, 2], "b": x}`,
			vars:   map[string]any{"x": 10},
		},
		{
			expr:   `1 + x + 2 == 2 + x + 1`,
			folded: `1 + x + 2 == 2 + x + 1`,
			vars:   map[string]any{"x": int64(5)},
		},
		{
			// The order of operations makes it such that the appearance of x in the first means that
			// none of the values provided into the addition call will be folded with the current
			// implementation. Ideally, the result would be 3 + x == x + 3 (which could be trivially true
			// and more easily observed as a result of common subexpression eliminiation)
			expr:   `1 + 2 + x ==  x + 2 + 1`,
			folded: `3 + x == x + 2 + 1`,
			vars:   map[string]any{"x": int64(5)},
		},
		{
			expr:        `google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR`,
			folded:      `1`,
			knownValues: map[string]any{},
		},
		{
			expr:   `google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR`,
			folded: `google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR`,
		},
		{
			expr:        `c == google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAZ ? "BAZ" : "Unknown"`,
			folded:      `"BAZ"`,
			knownValues: map[string]any{},
		},
		{
			expr: `[
						google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAR,
						c,
						google.expr.proto3.test.ImportedGlobalEnum.IMPORT_FOO
					].exists(e, e == google.expr.proto3.test.ImportedGlobalEnum.IMPORT_FOO)
						? "has Foo" : "no Foo"`,
			folded:      `"has Foo"`,
			knownValues: map[string]any{},
		},
		{
			expr:   `l.exists(e, e == "foo") ? "has Foo" : "no Foo"`,
			folded: `"has Foo"`,
			knownValues: map[string]any{
				"l": []string{"foo", "bar", "baz"},
			},
		},
		{
			expr:   `"foo" in l`,
			folded: `true`,
			knownValues: map[string]any{
				"l": []string{"foo", "bar", "baz"},
			},
		},
		{
			expr:   `o.repeated_int32`,
			folded: `[1, 2, 3]`,
			knownValues: map[string]any{
				"o": &proto3pb.TestAllTypes{RepeatedInt32: []int32{1, 2, 3}},
			},
		},
		{
			expr:   `false || x || false || y`,
			folded: `false || x || false || y`,
			vars:   map[string]any{"x": false, "y": true},
		},
		{
			expr:   `false || b || false || b`,
			folded: `b || b`,
			vars:   map[string]any{"b": false},
		},
		{
			expr:   `true ? (false ? x + 1 : x + 2) : x`,
			folded: `x + 2`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `false ? x : (true ? x + 1 : x + 2)`,
			folded: `x + 1`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `1 in []`,
			folded: `false`,
		},
		{
			expr:   `x in [1, 2, x]`,
			folded: `x in [1, 2, x]`,
			vars:   map[string]any{"x": int64(3)},
		},
		{
			expr:   `[1, 2].filter(x, x in [1, 2])`,
			folded: `[1, 2]`,
		},
		{
			expr:   `5 in [1, x, y, 5]`,
			folded: `true`,
			vars:   map[string]any{"x": int64(2), "y": int64(3)},
		},
		{
			expr:   `!(5 in [1, x, y, 5])`,
			folded: `false`,
			vars:   map[string]any{"x": int64(2), "y": int64(3)},
		},
		{
			expr:   `[1, ?optional.of(3)]`,
			folded: `[1, 3]`,
		},
		{
			expr:   `[1, optional.of(3)]`,
			folded: `[1, optional.of(3)]`,
		},
		{
			expr:   `[?optional.of(1 + 2 + 3)]`,
			folded: `[6]`,
		},
		{
			expr:   `[?optional.of(3)]`,
			folded: `[3]`,
		},
		{
			expr:   `[?optional.of(x)]`,
			folded: `[?optional.of(x)]`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `[?optional.ofNonZeroValue(3)]`,
			folded: `[3]`,
		},
		{
			expr:   `[optional.of(1 + 2 + 3)]`,
			folded: `[optional.of(6)]`,
		},
		{
			expr:   `[optional.of(x)]`,
			folded: `[optional.of(x)]`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `[optional.ofNonZeroValue(1 + 2 + 3)]`,
			folded: `[optional.of(6)]`,
		},
		{
			expr:   `[optional.ofNonZeroValue(3)]`,
			folded: `[optional.of(3)]`,
		},
		{
			expr:   `{?1: optional.none()}`,
			folded: `{}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{single_int64: 1 + 2 + 3 + x}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int64: 6 + x}`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{single_nested_message: google.expr.proto3.test.TestAllTypes.NestedMessage{bb: 42}}.single_nested_message.bb`,
			folded: `42`,
		},
		{
			expr:   `{"a": 1}["a"]`,
			folded: `1`,
		},
		{
			expr:   `{"a": {"b": 2}}["a"]["b"]`,
			folded: `2`,
		},
		{
			expr:   `{"hello": "world"}.?hello`,
			folded: `optional.of("world")`,
		},
		{
			expr:   `[1] + [2] + [3]`,
			folded: `[1, 2, 3]`,
		},
		{
			expr:   `[?optional.none(), 2]`,
			folded: `[2]`,
		},
		{
			expr:   `[1] + [?optional.of(2)] + [3]`,
			folded: `[1, 2, 3]`,
		},
		{
			expr:   `[1] + [x]`,
			folded: `[1, x]`,
			vars:   map[string]any{"x": int64(2)},
		},
		{
			expr:   `duration("1h") - duration("60m")`,
			folded: `duration("0s")`,
		},
		{
			expr:   `timestamp("1970-01-01T00:15:00Z") - timestamp("1970-01-01T00:00:10Z")`,
			folded: `duration("890s")`,
		},
		{
			expr:   `timestamp("2000-01-01T00:02:03.2123Z") + duration("25h2m32s42ms53us29ns")`,
			folded: `timestamp("2000-01-02T01:04:35.254353029Z")`,
		},
		{
			expr:   `[1 + 1, 1 + 2].exists(i, i < 10)`,
			folded: `true`,
		},
		{
			expr:   `[1, 2, 3].map(e, e * 2)`,
			folded: `[2, 4, 6]`,
		},
		{
			expr:   `[1, 2, 3].exists_one(e, e == 2)`,
			folded: `true`,
		},
		{
			expr:   `[1, 2, 3].exists_one(e, e > 1)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].exists_one(e, e > 5)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].all(e, e > 0)`,
			folded: `true`,
		},
		{
			expr:   `[1, 2, 3].all(e, e < 2)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].filter(e, e > 1)`,
			folded: `[2, 3]`,
		},
		{
			expr:   `{'a': 1, 'b': 2}.exists(k, k == 'b')`,
			folded: `true`,
		},
		{
			expr:   `{'a': 1, 'b': 2}.all(k, {'a': 1, 'b': 2}[k] > 0)`,
			folded: `true`,
		},
		{
			expr:   `{'a': 1}.filter(k, k == 'a')`,
			folded: `["a"]`,
		},
		{
			expr:   `{'a': 1}.map(k, {'a': 1}[k] * 2)`,
			folded: `[2]`,
		},
		{
			expr:   `optional.of(1).orValue(2)`,
			folded: `1`,
		},
		{
			expr:   `optional.none().orValue(2)`,
			folded: `2`,
		},
		{
			expr:   `optional.of(1).hasValue()`,
			folded: `true`,
		},
		{
			expr:   `optional.none().hasValue()`,
			folded: `false`,
		},
		{
			expr:   `optional.of(1).value()`,
			folded: `1`,
		},
		{
			expr:   `optional.of(optional.of(1)).optMap(x, x.orValue(0))`,
			folded: `optional.of(1)`,
		},
		{
			expr:   `optional.none().optMap(x, x + 1)`,
			folded: `optional.none()`,
		},
		{
			expr:   `optional.of(1).optFlatMap(x, optional.of(x + 1))`,
			folded: `optional.of(2)`,
		},
		{
			expr:   `optional.of(1).optFlatMap(x, optional.none())`,
			folded: `optional.none()`,
		},
		{
			expr:   `optional.of(1).or(optional.of(2))`,
			folded: `optional.of(1)`,
		},
		{
			expr:   `optional.none().or(optional.of(2))`,
			folded: `optional.of(2)`,
		},
		{
			expr:   `size([1, 2, 3])`,
			folded: `3`,
		},
		{
			expr:   `string(123)`,
			folded: `"123"`,
		},
		{
			expr:   `'hello'.contains('ell')`,
			folded: `true`,
		},
		{
			expr:   `'hello'.startsWith('he')`,
			folded: `true`,
		},
		{
			expr:   `'hello'.endsWith('lo')`,
			folded: `true`,
		},
		{
			expr:   `size('hello')`,
			folded: `5`,
		},
		{
			expr:   `'hello' + ' ' + 'world'`,
			folded: `"hello world"`,
		},
		{
			expr:   `1 + 2 * 3 - 4 / 2`,
			folded: `5`,
		},
		{
			expr:   `10 % 3`,
			folded: `1`,
		},
		{
			expr:   `-(5)`,
			folded: `-5`,
		},
		{
			expr:   `!true`,
			folded: `false`,
		},
		{
			expr:   `!false`,
			folded: `true`,
		},
		{
			expr:   `[10, 20, 30][1]`,
			folded: `20`,
		},
		{
			expr:   `{'a': 10, 'b': 20}['b']`,
			folded: `20`,
		},
		{
			expr:   `{'a': 10, 'b': 20}.a`,
			folded: `10`,
		},
		{
			expr:   `has({'a': 1}.a)`,
			folded: `true`,
		},
		{
			expr:   `has({'a': 1}.b)`,
			folded: `false`,
		},
		{
			expr:   `has(google.expr.proto3.test.TestAllTypes{single_int32: 1}.single_int32)`,
			folded: `true`,
		},
		{
			expr:   `has(google.expr.proto3.test.TestAllTypes{}.single_int32)`,
			folded: `false`,
		},
		{
			expr:   `timestamp("2023-01-01T00:00:00Z").getFullYear()`,
			folded: `2023`,
		},
		{
			expr:   `duration("2h").getHours()`,
			folded: `2`,
		},
		{
			expr:   `x in {}`,
			folded: `false`,
			vars:   map[string]any{"x": "a"},
		},
		{
			expr:   `1 in {}`,
			folded: `false`,
		},
		{
			expr:   `'a' in {'a': x, 'b': y}`,
			folded: `true`,
			vars:   map[string]any{"x": 1, "y": 2},
		},
		{
			expr:   `'a' in {?'a': optional.of(1), 'b': x}`,
			folded: `true`,
			vars:   map[string]any{"x": 2},
		},
		{
			expr:   `'a' in {?'a': optional.none(), 'b': x}`,
			folded: `"a" in {"b": x}`,
			vars:   map[string]any{"x": 2},
		},
		{
			expr:   `'a' in {?'a': optional.none()}`,
			folded: `false`,
		},
		{
			expr:   `'a' in {?'a': optional.of(1)}`,
			folded: `true`,
		},
		{
			expr:   `'b' in {?'a': optional.of(1)}`,
			folded: `false`,
		},
		{
			expr:   `'foo' in {'foo': 1, 'bar': 2}`,
			folded: `true`,
		},
		{
			expr:   `'baz' in {'foo': 1, 'bar': 2}`,
			folded: `false`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.of(1), single_int64: x}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: 1, single_int64: x}`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.none(), single_int64: x}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int64: x}`,
			vars:   map[string]any{"x": int64(10)},
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.of(1), ?single_int64: optional.none()}`,
			folded: `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.none(), ?single_int64: optional.none()}`,
			folded: `google.expr.proto3.test.TestAllTypes{}`,
		},
		{
			expr:   `has(google.expr.proto3.test.TestAllTypes{?single_int32: optional.of(1)}.single_int32)`,
			folded: `true`,
		},
		{
			expr:   `has(google.expr.proto3.test.TestAllTypes{?single_int32: optional.none()}.single_int32)`,
			folded: `false`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_int32: optional.of(42)}.single_int32`,
			folded: `42`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_nested_message: optional.of(google.expr.proto3.test.TestAllTypes.NestedMessage{bb: 42})}.single_nested_message.bb`,
			folded: `42`,
		},
		{
			expr:   `google.expr.proto3.test.TestAllTypes{?single_nested_message: optional.none()}`,
			folded: `google.expr.proto3.test.TestAllTypes{}`,
		},
		{
			expr:   `has({?'a': optional.of(1)}.a)`,
			folded: `true`,
		},
		{
			expr:   `has({?'a': optional.none()}.a)`,
			folded: `false`,
		},
		{
			expr:   `{?'a': optional.of(1)}.?a`,
			folded: `optional.of(1)`,
		},
		{
			expr:   `{?'a': optional.none()}.?a`,
			folded: `optional.none()`,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType),
		Variable("y", DynType),
		Variable("b", BoolType),
		// work around different package convention in piper vs github.
		// google.expr.proto3.test.ImportedGlobalEnum.IMPORT_BAZ
		Constant("c", IntType, types.Int(2)),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	e, err = e.Extend(Variable("l", ListType(StringType)))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	e, err = e.Extend(Variable("o", ObjectType("google.expr.proto3.test.TestAllTypes")))
	if err != nil {
		t.Fatalf("Extend() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			var foldingOpts []ConstantFoldingOption
			if tc.knownValues != nil {
				knownValues, err := NewActivation(tc.knownValues)
				if err != nil {
					t.Fatalf("NewActivation() failed: %v", err)
				}
				foldingOpts = append(foldingOpts, FoldKnownValues(knownValues))
			}
			folder, err := NewConstantFoldingOptimizer(foldingOpts...)
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}

			evalVars := map[string]any{
				"x": map[string]any{"y": "val", "b": 2},
				"y": "world",
				"b": true,
				"l": []string{"foo", "bar", "baz"},
				"o": &proto3pb.TestAllTypes{SingleInt64: 42, NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{Bb: 42}}, RepeatedInt32: []int32{1, 2, 3}},
			}
			if tc.knownValues != nil {
				maps.Copy(evalVars, tc.knownValues)
			}
			if tc.vars != nil {
				maps.Copy(evalVars, tc.vars)
			}
			prg, err := e.Program(checked)
			if err != nil {
				t.Fatalf("Program(checked) failed: %v", err)
			}
			fprg, err := e.Program(optimized)
			if err != nil {
				t.Fatalf("Program(optimized) failed: %v", err)
			}
			plainOut, _, plainErr := prg.Eval(evalVars)
			foldedOut, _, foldedErr := fprg.Eval(evalVars)
			if (plainErr != nil) != (foldedErr != nil) {
				t.Errorf("error status mismatch: plainErr=%v, foldedErr=%v", plainErr, foldedErr)
			} else if plainErr != nil {
				if plainErr.Error() != foldedErr.Error() {
					t.Errorf("error mismatch: plainErr=%v, foldedErr=%v", plainErr, foldedErr)
				}
			} else {
				if plainOut.Equal(foldedOut) != types.True {
					t.Errorf("evaluation mismatch: plain=%v (%T), folded=%v (%T)", plainOut, plainOut, foldedOut, foldedOut)
				}
			}
		})
	}
}

// TestConstantFoldingInMapIdent checks which identifier needles may be matched against a map
// key by name.
func TestConstantFoldingInMapIdent(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		expr   string
		folded string
		vars   map[string]any
	}{
		// Scalar types which cannot hold a NaN.
		{expr: `b in {b: 1}`, folded: `true`},
		{expr: `by in {by: 1}`, folded: `true`},
		{expr: `du in {du: 1}`, folded: `true`},
		{expr: `i in {1: 1, 2: 2, i: 3}`, folded: `true`},
		{expr: `s in {s: 1}`, folded: `true`},
		{expr: `ts in {ts: 1}`, folded: `true`},
		{expr: `ty in {ty: 1}`, folded: `true`},
		{expr: `u in {u: 1}`, folded: `true`},
		// Double and dyn may be a NaN directly.
		{expr: `d in {d: 1}`, folded: `d in {d: 1}`, vars: map[string]any{"d": nan}},
		{expr: `x in {1: 1, 2: 2, x: 3}`, folded: `x in {1: 1, 2: 2, x: 3}`, vars: map[string]any{"x": types.Double(nan)}},
		// Literal needles
		{expr: `'a' in {'a': 1, 'b': 2}`, folded: `true`},
		{expr: `1 in {1: 1, 2: 2}`, folded: `true`},
		{expr: `1.0 in {d: 1, 1.0: 2}`, folded: `true`},
	}
	env, err := NewEnv(
		OptionalTypes(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("b", BoolType),
		Variable("by", BytesType),
		Variable("du", DurationType),
		Variable("i", IntType),
		Variable("s", StringType),
		Variable("ts", TimestampType),
		Variable("ty", TypeType),
		Variable("u", UintType),
		Variable("d", DoubleType),
		Variable("x", DynType),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(env, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
			if tc.vars == nil {
				return
			}
			prg, err := env.Program(optimized)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.vars)
			if err != nil {
				t.Fatalf("Eval() failed: %v", err)
			}
			if out != types.False {
				t.Errorf("got %v, wanted false since NaN is not equal to itself", out)
			}
		})
	}
}

// TestConstantFoldingInListIdent checks which identifier needles may be matched against a list
// element by name.
//
// The rewrite is only sound when the identifier's static type guarantees that its runtime value
// is equal to itself. A NaN double is not, so any type which can carry a NaN must be left alone:
// the vars case of each such entry evaluates the optimized program with a NaN binding and expects
// the same result the unoptimized expression produces.
func TestConstantFoldingInListIdent(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		expr   string
		folded string
		// vars, when set, is evaluated against the optimized AST and must yield false.
		vars map[string]any
	}{
		// Scalar types which cannot hold a NaN.
		{expr: `b in [b]`, folded: `true`},
		{expr: `by in [by]`, folded: `true`},
		{expr: `du in [du]`, folded: `true`},
		{expr: `i in [1, 2, i]`, folded: `true`},
		{expr: `n in [n]`, folded: `true`},
		{expr: `s in [s]`, folded: `true`},
		{expr: `ts in [ts]`, folded: `true`},
		{expr: `ty in [ty]`, folded: `true`},
		{expr: `u in [u]`, folded: `true`},
		// Aggregate types compare element-wise, so they are self-equal exactly when their type
		// parameters are.
		{expr: `li in [li]`, folded: `true`},
		{expr: `lli in [lli]`, folded: `true`},
		{expr: `msi in [msi]`, folded: `true`},
		{expr: `ld in [ld]`, folded: `ld in [ld]`, vars: map[string]any{"ld": []float64{nan}}},
		{expr: `lx in [lx]`, folded: `lx in [lx]`, vars: map[string]any{"lx": []any{nan}}},
		{expr: `msd in [msd]`, folded: `msd in [msd]`, vars: map[string]any{"msd": map[string]float64{"a": nan}}},
		// Doubles and dyn may be a NaN directly.
		{expr: `d in [d]`, folded: `d in [d]`, vars: map[string]any{"d": nan}},
		{expr: `x in [1, 2, x]`, folded: `x in [1, 2, x]`, vars: map[string]any{"x": types.Double(nan)}},
		// Abstract and struct types are left alone as their contents are not inspected.
		{expr: `oi in [oi]`, folded: `oi in [oi]`},
		{expr: `o in [o]`, folded: `o in [o]`},
		// Literal needles use CEL equality rather than a name match, so they are unaffected.
		{expr: `1 in [1, 2]`, folded: `true`},
		{expr: `1.0 in [d, 1.0]`, folded: `true`},
	}
	env, err := NewEnv(
		OptionalTypes(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("b", BoolType),
		Variable("by", BytesType),
		Variable("du", DurationType),
		Variable("i", IntType),
		Variable("n", NullType),
		Variable("s", StringType),
		Variable("ts", TimestampType),
		Variable("ty", TypeType),
		Variable("u", UintType),
		Variable("li", ListType(IntType)),
		Variable("lli", ListType(ListType(IntType))),
		Variable("msi", MapType(StringType, IntType)),
		Variable("ld", ListType(DoubleType)),
		Variable("lx", ListType(DynType)),
		Variable("msd", MapType(StringType, DoubleType)),
		Variable("d", DoubleType),
		Variable("x", DynType),
		Variable("oi", OptionalType(IntType)),
		Variable("o", ObjectType("google.expr.proto3.test.TestAllTypes")),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(env, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
			if tc.vars == nil {
				return
			}
			prg, err := env.Program(optimized)
			if err != nil {
				t.Fatalf("Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.vars)
			if err != nil {
				t.Fatalf("Eval() failed: %v", err)
			}
			if out != types.False {
				t.Errorf("got %v, wanted false since NaN is not equal to itself", out)
			}
		})
	}
}

func TestConstantFoldingCallsWithSideEffects(t *testing.T) {
	tests := []struct {
		expr   string
		folded string
	}{
		{
			expr:   `noSideEffect(3)`,
			folded: `3`,
		},
		{
			expr:   `withSideEffect(3)`,
			folded: `withSideEffect(3)`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b) && withSideEffect(i.b) == 1)`,
			folded: `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b) && withSideEffect(i.b) == 1)`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b) && noSideEffect(i.b) == 2)`,
			folded: `true`,
		},
		{
			expr:   `noImpl(3)`,
			folded: "noImpl(3)",
		},
		{
			expr:   `asyncFunc(3)`,
			folded: `asyncFunc(3)`,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Function("noSideEffect",
			Overload("noSideEffect_int_int",
				[]*Type{IntType},
				IntType, FunctionBinding(func(args ...ref.Val) ref.Val {
					return args[0]
				}))),
		Function("withSideEffect",
			Overload("withSideEffect_int_int",
				[]*Type{IntType},
				IntType, LateFunctionBinding())),
		Function("noImpl",
			Overload("noImpl_int_int",
				[]*Type{IntType},
				IntType)),
		Function("asyncFunc",
			Overload("asyncFunc_int_int",
				[]*Type{IntType},
				IntType, AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					return args[0]
				}))),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}

func TestConstantFoldingOptimizerMacroElimination(t *testing.T) {
	tests := []struct {
		expr       string
		folded     string
		macroCount int
	}{
		{
			expr:   `has({}.key)`,
			folded: `false`,
		},
		{
			expr:   `[1, 2, 3].filter(i, i < 1)`,
			folded: `[]`,
		},
		{
			expr:   `[{}, {"a": 1}, {"b": 2}].exists(i, has(i.b))`,
			folded: `true`,
		},
		{
			expr:       `has(x.b) && [{}, {"a": 1}, {"b": 2}].exists(i, has(i.b))`,
			folded:     `has(x.b)`,
			macroCount: 1,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("folding got %q, wanted %q", folded, tc.folded)
			}
			if len(optimized.SourceInfo().GetMacroCalls()) != tc.macroCount {
				t.Errorf("folding got %d macros, wanted %d macros", len(optimized.SourceInfo().GetMacroCalls()), tc.macroCount)
			}
		})
	}
}

func TestConstantFoldingOptimizerWithLimit(t *testing.T) {
	tests := []struct {
		expr   string
		limit  int
		folded string
	}{
		{
			expr:   `[1, 1 + 2, 1 + (2 + 3)]`,
			limit:  1,
			folded: `[1, 3, 1 + 5]`,
		},
		{
			expr:   `5 in [1, 1 + 2, 1 + (2 + 3)]`,
			limit:  2,
			folded: `5 in [1, 3, 6]`,
		},
		{
			// though more complex, the final tryFold() at the end of the optimization pass
			// results in this computed output.
			expr:   `[1, 2, 3].map(i, [1, 2, 3].map(j, i * j))`,
			limit:  1,
			folded: `[[1, 2, 3], [2, 4, 6], [3, 6, 9]]`,
		},
	}
	e, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			folder, err := NewConstantFoldingOptimizer(MaxConstantFoldIterations(tc.limit))
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tc.folded {
				t.Errorf("got %q, wanted %q", folded, tc.folded)
			}
		})
	}
}

func TestConstantFoldingNormalizeIDs(t *testing.T) {
	tests := []struct {
		expr             string
		ids              []int64
		macros           map[int64]string
		normalizedIDs    []int64
		normalizedMacros map[int64]string
	}{
		{
			expr:          `[1, 2, 3]`,
			ids:           []int64{1, 2, 3, 4},
			normalizedIDs: []int64{1, 2, 3, 4},
		},
		{
			expr:          `google.expr.proto3.test.TestAllTypes{single_int32: 0}`,
			ids:           []int64{1, 2, 3},
			normalizedIDs: []int64{1, 2, 3},
		},
		{
			expr: `has({x: 'value'}.single_int32)`,
			ids:  []int64{2, 3, 4, 5, 7},
			macros: map[int64]string{7: `
			call_expr: {
				function: "has"
				args: {
				  id: 6
				  select_expr: {
					operand: {
					  id: 2
					  struct_expr: {
						entries: {
						  id: 3
						  map_key: {
							id: 4
							ident_expr: {
							  name: "x"
							}
						  }
						  value: {
							id: 5
							const_expr: {
							  string_value: "value"
							}
						  }
						}
					  }
					}
					field: "single_int32"
				  }
				}
			  }`},
			normalizedIDs: []int64{1, 2, 3, 4, 5},
			normalizedMacros: map[int64]string{1: `
			call_expr:  {
				function:  "has"
				args:  {
				  id:  6
				  select_expr:  {
					operand:  {
					  id:  2
					  struct_expr:  {
						entries:  {
						  id:  3
						  map_key:  {
							id:  4
							ident_expr:  {
							  name:  "x"
							}
						  }
						  value:  {
							id:  5
							const_expr:  {
							  string_value:  "value"
							}
						  }
						}
					  }
					}
					field:  "single_int32"
				  }
				}
			  }`,
			},
		},
		{
			expr: `has(google.expr.proto3.test.TestAllTypes{}.single_int32)`,
			ids:  []int64{2, 4},
			macros: map[int64]string{
				4: `call_expr:  {
					function:  "has"
					args:  {
					  id:  3
					  select_expr:  {
						operand:  {
						  id:  2
						  struct_expr:  {
							message_name:  "google.expr.proto3.test.TestAllTypes"
						  }
						}
						field:  "single_int32"
					  }
					}
				  }`,
			},
			normalizedIDs: []int64{1},
		},
		{
			expr: `[true].exists(i, i)`,
			ids:  []int64{1, 2, 5, 6, 7, 8, 9, 10, 11, 12, 13},
			macros: map[int64]string{
				13: `call_expr:  {
					target:  {
					  id:  1
					  list_expr:  {
						elements:  {
						  id:  2
						  const_expr:  {
							bool_value:  true
						  }
						}
					  }
					}
					function:  "exists"
					args:  {
					  id:  4
					  ident_expr:  {
						name:  "i"
					  }
					}
					args:  {
					  id:  5
					  ident_expr:  {
						name:  "i"
					  }
					}
				  }`,
			},
			normalizedIDs: []int64{1},
		},
		{
			expr: `[x].exists(i, i)`,
			ids:  []int64{1, 2, 5, 6, 7, 8, 9, 10, 11, 12, 13},
			macros: map[int64]string{
				13: `call_expr:  {
					target:  {
					  id:  1
					  list_expr:  {
						elements:  {
						  id:  2
						  ident_expr:  {
							name:  "x"
						  }
						}
					  }
					}
					function:  "exists"
					args:  {
					  id:  4
					  ident_expr:  {
						name:  "i"
					  }
					}
					args:  {
					  id:  5
					  ident_expr:  {
						name:  "i"
					  }
					}
				  }`,
			},
			normalizedIDs: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			normalizedMacros: map[int64]string{
				1: `call_expr: {
					target: {
					  id: 2
					  list_expr: {
						elements: {
						  id: 3
						  ident_expr: {
							name: "x"
						  }
						}
					  }
					}
					function: "exists"
					args: {
					  id: 12
					  ident_expr: {
						name: "i"
					  }
					}
					args: {
					  id: 10
					  ident_expr: {
						name: "i"
					  }
					}
				  }`,
			},
		},
	}
	e, err := NewEnv(
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("x", DynType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			preOpt := newIDCollector()
			ast.PostOrderVisit(checked.NativeRep().Expr(), preOpt)
			if !reflect.DeepEqual(preOpt.IDs(), tc.ids) {
				t.Errorf("Compile() got ids %v, expected %v", preOpt.IDs(), tc.ids)
			}
			for id, call := range checked.NativeRep().SourceInfo().MacroCalls() {
				macroText, found := tc.macros[id]
				if !found {
					t.Fatalf("Compile() did not find macro %d", id)
				}
				pbCall, err := ast.ExprToProto(call)
				if err != nil {
					t.Fatalf("ast.ExprToProto() failed: %v", err)
				}
				pbMacro := &exprpb.Expr{}
				err = prototext.Unmarshal([]byte(macroText), pbMacro)
				if err != nil {
					t.Fatalf("prototext.Unmarshal() failed: %v", err)
				}
				if !proto.Equal(pbCall, pbMacro) {
					t.Errorf("Compile() for macro %d got %s, expected %s", id, prototext.Format(pbCall), macroText)
				}
			}
			folder, err := NewConstantFoldingOptimizer()
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(e, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() generated an invalid AST: %v", iss.Err())
			}
			postOpt := newIDCollector()
			ast.PostOrderVisit(optimized.NativeRep().Expr(), postOpt)
			if !reflect.DeepEqual(postOpt.IDs(), tc.normalizedIDs) {
				t.Errorf("Optimize() got ids %v, expected %v", postOpt.IDs(), tc.normalizedIDs)
			}
			for id, call := range optimized.NativeRep().SourceInfo().MacroCalls() {
				macroText, found := tc.normalizedMacros[id]
				if !found {
					t.Fatalf("Optimize() did not find macro %d", id)
				}
				pbCall, err := ast.ExprToProto(call)
				if err != nil {
					t.Fatalf("ast.ExprToProto() failed: %v", err)
				}
				pbMacro := &exprpb.Expr{}
				err = prototext.Unmarshal([]byte(macroText), pbMacro)
				if err != nil {
					t.Fatalf("prototext.Unmarshal() failed: %v", err)
				}
				if !proto.Equal(pbCall, pbMacro) {
					t.Errorf("Optimize() for macro %d got %s, expected %s", id, prototext.Format(pbCall), macroText)
				}
			}
		})
	}
}

func newIDCollector() *idCollector {
	return &idCollector{
		ids: int64Slice{},
	}
}

type idCollector struct {
	ids int64Slice
}

func (c *idCollector) VisitExpr(e ast.Expr) {
	if e.ID() == 0 {
		return
	}
	c.ids = append(c.ids, e.ID())
}

// VisitEntryExpr updates the max identifier if the incoming entry id is greater than previously observed.
func (c *idCollector) VisitEntryExpr(e ast.EntryExpr) {
	if e.ID() == 0 {
		return
	}
	c.ids = append(c.ids, e.ID())
}

func (c *idCollector) IDs() []int64 {
	sort.Sort(c.ids)
	return c.ids
}

// int64Slice is an implementation of the sort.Interface
type int64Slice []int64

// Len returns the number of elements in the slice.
func (x int64Slice) Len() int { return len(x) }

// Less indicates whether the value at index i is less than the value at index j.
func (x int64Slice) Less(i, j int) bool { return x[i] < x[j] }

// Swap swaps the values at indices i and j in place.
func (x int64Slice) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

// Sort is a convenience method: x.Sort() calls Sort(x).
func (x int64Slice) Sort() { sort.Sort(x) }

func TestConstantFoldingOption_FoldKnownValuesNilInput(t *testing.T) {
	opt, err := FoldKnownValues(nil)(&constantFoldingOptimizer{})
	if err != nil || opt.knownValues == nil {
		t.Errorf("FoldKnownValues(nil) failed: %v", err)
	}
}

func TestNewConstantFoldingOptimizer_OptionErrorPropagation(t *testing.T) {
	errOpt := func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error) {
		return nil, errors.New("option error")
	}
	if _, err := NewConstantFoldingOptimizer(errOpt); err == nil {
		t.Error("NewConstantFoldingOptimizer(errOpt) wanted error, got nil")
	}
}

func TestConstantFoldingOptimizer_EvaluateExpr(t *testing.T) {
	partAct, err := interpreter.NewPartialActivation(
		interpreter.EmptyActivation(),
		interpreter.NewAttributePattern("x"))
	if err != nil {
		t.Fatalf("NewPartialActivation() failed: %v", err)
	}
	unknownValAct, err := NewActivation(map[string]any{
		"x": types.NewUnknown(1, types.NewAttributeTrail("x")),
	})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}

	tests := []struct {
		name     string
		expr     string
		vars     []EnvOption
		act      Activation
		wantFold string
	}{
		{
			name:     "missing attribute",
			expr:     "x + 1",
			vars:     []EnvOption{Variable("x", IntType)},
			act:      partAct,
			wantFold: "x + 1",
		},
		{
			name:     "unknown value",
			expr:     "x + 1",
			vars:     []EnvOption{Variable("x", IntType)},
			act:      unknownValAct,
			wantFold: "x + 1",
		},
		{
			name:     "evaluation error division by zero",
			expr:     "1 / 0",
			wantFold: "1 / 0",
		},
		{
			name:     "evaluation error index out of bounds",
			expr:     "[1, 2][5]",
			wantFold: "[1, 2][5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnv(tt.vars...)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			var foldOpts []ConstantFoldingOption
			if tt.act != nil {
				foldOpts = append(foldOpts, FoldKnownValues(tt.act))
			}
			folder, err := NewConstantFoldingOptimizer(foldOpts...)
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			ast, iss := env.Compile(tt.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile() failed: %v", iss.Err())
			}
			optimized, iss := opt.Optimize(env, ast)
			if iss.Err() != nil {
				t.Fatalf("Optimize() failed: %v", iss.Err())
			}
			folded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if folded != tt.wantFold {
				t.Errorf("got %q, wanted %q", folded, tt.wantFold)
			}
		})
	}
}

type testVariadicLogicOptimizer struct{}

func (t *testVariadicLogicOptimizer) Optimize(ctx *OptimizerContext, a *ast.AST) *ast.AST {
	call := ctx.NewCall(operators.LogicalAnd, ctx.NewIdent("x"), ctx.NewLiteral(types.True), ctx.NewIdent("y"))
	return ctx.NewAST(call)
}

func TestConstantFoldingOptimizer_VariadicShortcircuitLogic(t *testing.T) {
	env, err := NewEnv(Variable("x", BoolType), Variable("y", BoolType))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	folder, err := NewConstantFoldingOptimizer()
	if err != nil {
		t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
	}
	opt, err := NewStaticOptimizer(&testVariadicLogicOptimizer{}, folder)
	if err != nil {
		t.Fatalf("NewStaticOptimizer() failed: %v", err)
	}
	parsed, iss := env.Parse("x && y")
	if iss.Err() != nil {
		t.Fatalf("Parse() failed: %v", iss.Err())
	}
	optimized, iss := opt.Optimize(env, parsed)
	if iss.Err() != nil {
		t.Fatalf("Optimize() failed: %v", iss.Err())
	}
	folded, err := AstToString(optimized)
	if err != nil {
		t.Fatalf("AstToString() failed: %v", err)
	}
	if folded != "x && y" {
		t.Errorf("got %q, wanted %q", folded, "x && y")
	}
}

func TestNewConstantFoldingOptimizer_Options(t *testing.T) {
	errOption := func(opt *constantFoldingOptimizer) (*constantFoldingOptimizer, error) {
		return nil, errors.New("option error")
	}
	_, err := NewConstantFoldingOptimizer(errOption)
	if err == nil {
		t.Errorf("expected error, got nil")
	}

	folder, err := NewConstantFoldingOptimizer(FoldKnownValues(nil), MaxConstantFoldIterations(10))
	if err != nil {
		t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
	}
	if folder == nil {
		t.Errorf("expected non-nil folder")
	}
}

type unadaptableVal struct {
	valType *types.Type
}

func (u *unadaptableVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, errors.New("cannot convert")
}
func (u *unadaptableVal) ConvertToType(typeVal ref.Type) ref.Val {
	return types.NewErr("cannot convert")
}
func (u *unadaptableVal) Equal(other ref.Val) ref.Val { return types.False }
func (u *unadaptableVal) Type() ref.Type              { return u.valType }
func (u *unadaptableVal) Value() any                  { return u }

func TestAdaptLiteral(t *testing.T) {
	env, err := NewEnv(
		OptionalTypes(),
		Types(&proto3pb.TestAllTypes{}),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	optCtx := &OptimizerContext{
		Env: env,
		optimizerExprFactory: &optimizerExprFactory{
			idGenerator: newIDGenerator(0),
			fac:         ast.NewExprFactory(),
			sourceInfo:  ast.NewSourceInfo(nil),
		},
		Issues: NewIssues(nil),
	}

	adapter := types.DefaultTypeAdapter

	// Test scalars
	scalars := []ref.Val{
		types.True,
		types.Bytes("abc"),
		types.Double(3.14),
		types.Int(42),
		types.NullValue,
		types.String("hello"),
		types.Uint(100),
		types.DurationType, // TypeType
	}
	for _, val := range scalars {
		expr, err := adaptLiteral(optCtx, val)
		if err != nil {
			t.Errorf("adaptLiteral(%v) failed: %v", val, err)
		}
		if expr == nil {
			t.Errorf("adaptLiteral(%v) returned nil expr", val)
		}
	}

	// Test Optional
	optNone := types.OptionalNone
	if expr, err := adaptLiteral(optCtx, optNone); err != nil || expr == nil {
		t.Errorf("adaptLiteral(OptionalNone) failed: %v", err)
	}
	optVal := types.OptionalOf(types.String("world"))
	if expr, err := adaptLiteral(optCtx, optVal); err != nil || expr == nil {
		t.Errorf("adaptLiteral(OptionalOf) failed: %v", err)
	}

	// Test List
	listVal := adapter.NativeToValue([]string{"a", "b", "c"})
	if expr, err := adaptLiteral(optCtx, listVal); err != nil || expr == nil {
		t.Errorf("adaptLiteral(List) failed: %v", err)
	}

	// Test Map
	mapVal := adapter.NativeToValue(map[string]int64{"key": 1})
	if expr, err := adaptLiteral(optCtx, mapVal); err != nil || expr == nil {
		t.Errorf("adaptLiteral(Map) failed: %v", err)
	}

	// Test Struct (with field set)
	protoMsg := &proto3pb.TestAllTypes{SingleInt64: 123}
	structVal := env.TypeAdapter().NativeToValue(protoMsg)
	if expr, err := adaptLiteral(optCtx, structVal); err != nil || expr == nil {
		t.Errorf("adaptLiteral(Struct) failed: %v", err)
	}

	// Error branch: ListType on non-Lister
	badList := &unadaptableVal{valType: types.ListType}
	if _, err := adaptLiteral(optCtx, badList); err == nil {
		t.Errorf("expected error adapting bad list, got nil")
	}

	// Error branch: MapType on non-Mapper
	badMap := &unadaptableVal{valType: types.MapType}
	if _, err := adaptLiteral(optCtx, badMap); err == nil {
		t.Errorf("expected error adapting bad map, got nil")
	}

	// Error branch: StructType not found
	badStruct := &unadaptableVal{valType: types.NewObjectType("unknown.Type")}
	if _, err := adaptLiteral(optCtx, badStruct); err == nil {
		t.Errorf("expected error adapting unknown struct, got nil")
	}

	// Error branch: Optional containing unadaptable value
	badOpt := types.OptionalOf(badStruct)
	if _, err := adaptLiteral(optCtx, badOpt); err == nil {
		t.Errorf("expected error adapting bad optional, got nil")
	}

	// Error branch: List containing unadaptable element
	badElemList := types.NewRefValList(adapter, []ref.Val{badStruct})
	if _, err := adaptLiteral(optCtx, badElemList); err == nil {
		t.Errorf("expected error adapting list with bad element, got nil")
	}

	// Error branch: Map containing unadaptable value
	badValMap := types.NewRefValMap(adapter, map[ref.Val]ref.Val{types.String("key"): badStruct})
	if _, err := adaptLiteral(optCtx, badValMap); err == nil {
		t.Errorf("expected error adapting map with bad val, got nil")
	}

	// Error branch: Map containing unadaptable key
	badKeyMap := types.NewRefValMap(adapter, map[ref.Val]ref.Val{badStruct: types.String("val")})
	if _, err := adaptLiteral(optCtx, badKeyMap); err == nil {
		t.Errorf("expected error adapting map with bad key, got nil")
	}

	// Error branch: Non-*types.Type
	nonCelType := &unadaptableVal{valType: nil}
	if _, err := adaptLiteral(optCtx, nonCelType); err == nil {
		t.Errorf("expected error adapting non-CEL type, got nil")
	}
}

func TestUpdateMetadata(t *testing.T) {
	// updateMetadata with nil AST does not panic
	updateMetadata(nil, 1, 2)

	fac := ast.NewExprFactory()
	e1 := fac.NewIdent(1, "x")
	info := ast.NewSourceInfo(nil)
	a := ast.NewAST(e1, info)
	checked := ast.NewCheckedAST(a, map[int64]*types.Type{2: types.IntType}, map[int64]*ast.ReferenceInfo{
		2: ast.NewIdentReference("y", nil),
	})

	// When updatedID has reference and type info
	updateMetadata(checked, 1, 2)
	if ref, found := checked.ReferenceMap()[1]; !found || ref.Name != "y" {
		t.Errorf("expected ref 'y' on target ID 1, got %v", ref)
	}
	if typ, found := checked.TypeMap()[1]; !found || typ != types.IntType {
		t.Errorf("expected type IntType on target ID 1, got %v", typ)
	}

	// When updatedID is not in ReferenceMap
	updateMetadata(checked, 1, 3)
	if _, found := checked.ReferenceMap()[1]; found {
		t.Errorf("expected target ID 1 to be removed from ReferenceMap")
	}
}

func TestConstantFolding_ComprehensionUnroll(t *testing.T) {
	hugeBlob := strings.Repeat("x", 1024)
	attestations := []map[string]any{
		{
			"name":      "frontend",
			"huge_blob": hugeBlob,
			"predicate": map[string]any{
				"builder": map[string]any{
					"id": "builder-1",
				},
			},
		},
		{
			"name":      "backend",
			"huge_blob": hugeBlob,
			"predicate": map[string]any{
				"builder": map[string]any{
					"id": "builder-2",
				},
			},
		},
	}
	envRegions := []map[string]any{
		{"env": "prod", "region": "us"},
		{"env": "dev", "region": "eu"},
	}

	tests := []struct {
		name           string
		expr           string
		knownValues    map[string]any
		foldingOpts    []ConstantFoldingOption
		wantFolded     string
		forbidSubstr   string
		evalInputs     []map[string]any
		wantMacroCount int
		checkMacroCnt  bool
	}{
		// 1. Large Map / Attestation Projection (Core Use Case)
		{
			name:        "attestation_projection_exists",
			expr:        `a.exists(x, x.name == baz.name && x.predicate.builder.id == baz.expected_builder)`,
			knownValues: map[string]any{"a": attestations},
			wantFolded: "\"frontend\" == baz.name && \"builder-1\" == baz.expected_builder || \"backend\" == baz.name &&\n" +
				"\"builder-2\" == baz.expected_builder",
			forbidSubstr: "huge_blob",
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "frontend", "expected_builder": "builder-1"}},
				{"baz": map[string]any{"name": "backend", "expected_builder": "builder-2"}},
				{"baz": map[string]any{"name": "frontend", "expected_builder": "builder-2"}},
				{"baz": map[string]any{"name": "unknown", "expected_builder": "builder-1"}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:        "attestation_projection_all",
			expr:        `a.all(x, x.predicate.builder.id == baz.expected_builder)`,
			knownValues: map[string]any{"a": attestations},
			wantFolded:  `"builder-1" == baz.expected_builder && "builder-2" == baz.expected_builder`,
			forbidSubstr: "huge_blob",
			evalInputs: []map[string]any{
				{"baz": map[string]any{"expected_builder": "builder-1"}},
				{"baz": map[string]any{"expected_builder": "builder-2"}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		// 2. Partial Short-Circuiting Inside Unrolled Branches
		{
			name:        "partial_short_circuit_exists_branch_pruned",
			expr:        `a.exists(x, x.env == "prod" && x.region == baz.region)`,
			knownValues: map[string]any{"a": envRegions},
			wantFolded:  `"us" == baz.region`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"region": "us"}},
				{"baz": map[string]any{"region": "eu"}},
			},
		},
		{
			name:        "partial_short_circuit_exists_unconditional_true",
			expr:        `a.exists(x, x.env == "prod" || x.region == baz.region)`,
			knownValues: map[string]any{"a": envRegions},
			wantFolded:  `true`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"region": "us"}},
				{"baz": map[string]any{"region": "ap"}},
			},
		},
		{
			name:        "partial_short_circuit_all_unconditional_false",
			expr:        `a.all(x, x.env == "prod" && x.region == baz.region)`,
			knownValues: map[string]any{"a": envRegions},
			wantFolded:  `false`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"region": "us"}},
				{"baz": map[string]any{"region": "eu"}},
			},
		},
		// 3. Empty & Single-Element Lists
		{
			name:           "empty_list_literal_exists",
			expr:           `[].exists(x, x == baz.name)`,
			wantFolded:     `false`,
			evalInputs:     []map[string]any{{"baz": map[string]any{"name": "frontend"}}},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:           "empty_list_literal_all",
			expr:           `[].all(x, x == baz.name)`,
			wantFolded:     `true`,
			evalInputs:     []map[string]any{{"baz": map[string]any{"name": "frontend"}}},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:        "empty_known_list_exists",
			expr:        `a.exists(x, x.name == baz.name)`,
			knownValues: map[string]any{"a": []map[string]any{}},
			wantFolded:  `false`,
			evalInputs:  []map[string]any{{"baz": map[string]any{"name": "frontend"}}},
		},
		{
			name:        "empty_known_list_all",
			expr:        `a.all(x, x.name == baz.name)`,
			knownValues: map[string]any{"a": []map[string]any{}},
			wantFolded:  `true`,
			evalInputs:  []map[string]any{{"baz": map[string]any{"name": "frontend"}}},
		},
		{
			name:        "single_element_list_exists",
			expr:        `[foo].exists(x, x.name == baz.name)`,
			knownValues: map[string]any{"foo": attestations[0]},
			wantFolded:  `"frontend" == baz.name`,
			forbidSubstr: "huge_blob",
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "frontend"}},
				{"baz": map[string]any{"name": "backend"}},
			},
		},
		{
			name:        "single_element_list_all",
			expr:        `[foo].all(x, x.name == baz.name)`,
			knownValues: map[string]any{"foo": attestations[0]},
			wantFolded:  `"frontend" == baz.name`,
			forbidSubstr: "huge_blob",
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "frontend"}},
				{"baz": map[string]any{"name": "backend"}},
			},
		},
		// 4. Shadowed & Nested Comprehensions + Presence Tests
		{
			name:       "shadowed_same_iter_var_nested_all",
			expr:       `[1, 2].all(x, [3, 4].all(x, x > baz.min))`,
			wantFolded: `3 > baz.min && 4 > baz.min`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
				{"baz": map[string]any{"min": int64(3)}},
			},
		},
		{
			name:       "shadowed_same_iter_var_referenced_in_inner_range",
			expr:       `[1, 2].all(x, [x, 4].all(x, x > baz.min))`,
			wantFolded: `1 > baz.min && 4 > baz.min && 2 > baz.min && 4 > baz.min`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(0)}},
				{"baz": map[string]any{"min": int64(1)}},
			},
		},
		{
			name:       "outer_unroll_around_inner_dynamic_comprehension",
			expr:       `[1, 2].exists(x, dyn_list.exists(y, x == y))`,
			wantFolded: `dyn_list.exists(y, 1 == y) || dyn_list.exists(y, 2 == y)`,
			evalInputs: []map[string]any{
				{"dyn_list": []int64{2, 5}},
				{"dyn_list": []int64{3, 4}},
			},
			wantMacroCount: 2,
			checkMacroCnt:  true,
		},
		{
			name:       "inner_unroll_inside_outer_dynamic_comprehension",
			expr:       `dyn_list.exists(y, [1, 2].exists(x, x == y))`,
			wantFolded: `dyn_list.exists(y, 1 == y || 2 == y)`,
			evalInputs: []map[string]any{
				{"dyn_list": []int64{2, 5}},
				{"dyn_list": []int64{3, 4}},
			},
			wantMacroCount: 1,
			checkMacroCnt:  true,
		},
		{
			name:       "variable_capture_avoidance",
			expr:       `[y].all(x, dyn_list.all(y, x == y))`,
			wantFolded: `[y].all(x, dyn_list.all(y, x == y))`,
			evalInputs: []map[string]any{
				{"y": int64(10), "dyn_list": []int64{1, 2}},
				{"y": int64(1), "dyn_list": []int64{1, 1}},
			},
		},
		{
			name:       "presence_test_map_literals",
			expr:       `[{"a": 1, "name": "ok"}, {"b": 2, "name": "skip"}].exists(x, has(x.a) && x.name == baz.name)`,
			wantFolded: `"ok" == baz.name`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "ok"}},
				{"baz": map[string]any{"name": "skip"}},
			},
		},
		{
			name:       "presence_test_proto_structs",
			expr:       `[google.expr.proto3.test.TestAllTypes{single_int32: 42, single_string: "p1"}, google.expr.proto3.test.TestAllTypes{single_string: "p2"}].exists(x, has(x.single_int32) && x.single_string == baz.name)`,
			wantFolded: `"p1" == baz.name`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "p1"}},
				{"baz": map[string]any{"name": "p2"}},
			},
		},
		{
			name:       "resolved_optional_list_elements_unrolled",
			expr:       `[?optional.of(1), ?optional.none(), 2].exists(x, x == baz.min)`,
			wantFolded: `1 == baz.min || 2 == baz.min`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(1)}},
				{"baz": map[string]any{"min": int64(3)}},
			},
		},
		// 5. Non-Unrollable / Bypass Cases
		{
			name:       "dynamic_list_not_unrolled",
			expr:       `dyn_list.exists(x, x == baz.min)`,
			wantFolded: `dyn_list.exists(x, x == baz.min)`,
			evalInputs: []map[string]any{
				{"dyn_list": []int64{1, 2}, "baz": map[string]any{"min": int64(2)}},
			},
		},
		{
			name:       "unresolved_optional_list_element_not_unrolled",
			expr:       `[?opt_x, 1].exists(x, x == baz.min)`,
			wantFolded: `[?opt_x, 1].exists(x, x == baz.min)`,
			evalInputs: []map[string]any{
				{"opt_x": types.OptionalNone, "baz": map[string]any{"min": int64(1)}},
				{"opt_x": types.OptionalOf(types.Int(5)), "baz": map[string]any{"min": int64(5)}},
			},
		},
		{
			name:        "exceeds_max_comprehension_unroll_size",
			expr:        `[1, 2, 3].exists(x, x == baz.min)`,
			foldingOpts: []ConstantFoldingOption{MaxComprehensionUnrollSize(2)},
			wantFolded:  `[1, 2, 3].exists(x, x == baz.min)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
				{"baz": map[string]any{"min": int64(5)}},
			},
		},
		{
			name:        "disabled_comprehension_unroll_zero",
			expr:        `[1, 2].exists(x, x == baz.min)`,
			foldingOpts: []ConstantFoldingOption{MaxComprehensionUnrollSize(0)},
			wantFolded:  `[1, 2].exists(x, x == baz.min)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
			},
		},
		{
			name:       "filter_not_unrolled",
			expr:       `[1, 2, 3].filter(x, x == baz.min)`,
			wantFolded: `[1, 2, 3].filter(x, x == baz.min)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
			},
		},
		{
			name:       "map_not_unrolled",
			expr:       `[1, 2, 3].map(x, x + baz.min)`,
			wantFolded: `[1, 2, 3].map(x, x + baz.min)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
			},
		},
		{
			name:       "exists_one_not_unrolled",
			expr:       `[1, 2, 3].exists_one(x, x >= baz.min)`,
			wantFolded: `[1, 2, 3].exists_one(x, x >= baz.min)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(3)}},
				{"baz": map[string]any{"min": int64(2)}},
			},
		},
		// 6. Adversarial Edge Cases
		{
			name:       "presence_test_partially_dynamic_map",
			expr:       `[{"a": baz.name}, {"b": "other"}].exists(x, has(x.a))`,
			wantFolded: `has({"a": baz.name}.a)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "frontend"}},
				{"baz": map[string]any{"name": ""}},
			},
			wantMacroCount: 1,
			checkMacroCnt:  true,
		},
		{
			name:        "presence_test_known_values_list",
			expr:        `a.exists(x, has(x.predicate) && x.name == baz.name)`,
			knownValues: map[string]any{"a": attestations},
			wantFolded:  `"frontend" == baz.name || "backend" == baz.name`,
			forbidSubstr: "huge_blob",
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "frontend"}},
				{"baz": map[string]any{"name": "unknown"}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:        "multiple_iter_var_refs_in_single_predicate",
			expr:        `a.exists(x, x.env == "prod" && x.region == baz.region || x.env == "dev" && x.region == baz.fallback)`,
			knownValues: map[string]any{"a": envRegions},
			wantFolded:  `"us" == baz.region || "eu" == baz.fallback`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"region": "us", "fallback": "ap"}},
				{"baz": map[string]any{"region": "ap", "fallback": "eu"}},
				{"baz": map[string]any{"region": "ap", "fallback": "ap"}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:       "odd_sized_list_3_elements_exists",
			expr:       `[1, 2, 3].exists(x, x == baz.min)`,
			wantFolded: `1 == baz.min || 2 == baz.min || 3 == baz.min`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
				{"baz": map[string]any{"min": int64(4)}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:       "odd_sized_list_5_elements_all",
			expr:       `[1, 2, 3, 4, 5].all(x, x > baz.min)`,
			wantFolded: `1 > baz.min && 2 > baz.min && 3 > baz.min && 4 > baz.min && 5 > baz.min`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(0)}},
				{"baz": map[string]any{"min": int64(1)}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:       "nested_list_of_lists_multi_pass_unroll",
			expr:       `[[1, 2], [3, 4]].exists(x, x.exists(y, y == baz.min))`,
			wantFolded: `1 == baz.min || 2 == baz.min || 3 == baz.min || 4 == baz.min`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(3)}},
				{"baz": map[string]any{"min": int64(9)}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:       "outer_var_same_name_as_iter_var_in_range_not_unrolled",
			expr:       `[y, 4].all(y, y > baz.min)`,
			wantFolded: `[y, 4].all(y, y > baz.min)`,
			evalInputs: []map[string]any{
				{"y": int64(5), "baz": map[string]any{"min": int64(2)}},
				{"y": int64(1), "baz": map[string]any{"min": int64(2)}},
			},
			wantMacroCount: 1,
			checkMacroCnt:  true,
		},
		{
			name:       "outer_var_different_name_than_iter_var_in_range_unrolled",
			expr:       `[y, 4].all(x, x > baz.min)`,
			wantFolded: `y > baz.min && 4 > baz.min`,
			evalInputs: []map[string]any{
				{"y": int64(5), "baz": map[string]any{"min": int64(2)}},
				{"y": int64(1), "baz": map[string]any{"min": int64(2)}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:       "unused_iter_var_with_non_constant_elem_not_unrolled",
			expr:       `[y, 1].exists(x, baz.min == 1)`,
			wantFolded: `[y, 1].exists(x, baz.min == 1)`,
			evalInputs: []map[string]any{
				{"y": int64(5), "baz": map[string]any{"min": int64(1)}},
				{"y": int64(5), "baz": map[string]any{"min": int64(2)}},
			},
		},
		{
			name:       "optional_select_on_unrolled_maps",
			expr:       `[{"a": "ok"}, {"b": "skip"}].exists(x, x.?a.orValue("none") == baz.name)`,
			wantFolded: `"ok" == baz.name || "none" == baz.name`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"name": "ok"}},
				{"baz": map[string]any{"name": "none"}},
				{"baz": map[string]any{"name": "other"}},
			},
			wantMacroCount: 0,
			checkMacroCnt:  true,
		},
		{
			name:       "nested_inner_macro_with_presence_test",
			expr:       `[{"a": 1}, {"b": 2}].exists(x, dyn_list.exists(y, has(x.a) && y == 1))`,
			wantFolded: `dyn_list.exists(y, y == 1) || dyn_list.exists(y, false)`,
			evalInputs: []map[string]any{
				{"dyn_list": []int64{1, 2}},
				{"dyn_list": []int64{2, 3}},
			},
			wantMacroCount: 2,
			checkMacroCnt:  true,
		},
		{
			name:        "disabled_comprehension_unroll_negative",
			expr:        `[1, 2].exists(x, x == baz.min)`,
			foldingOpts: []ConstantFoldingOption{MaxComprehensionUnrollSize(-5)},
			wantFolded:  `[1, 2].exists(x, x == baz.min)`,
			evalInputs: []map[string]any{
				{"baz": map[string]any{"min": int64(2)}},
			},
		},
	}

	env, err := NewEnv(
		OptionalTypes(),
		EnableMacroCallTracking(),
		Types(&proto3pb.TestAllTypes{}),
		Variable("a", ListType(MapType(StringType, DynType))),
		Variable("foo", MapType(StringType, DynType)),
		Variable("baz", MapType(StringType, DynType)),
		Variable("dyn_list", ListType(IntType)),
		Variable("y", IntType),
		Variable("opt_x", OptionalType(IntType)),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checked, iss := env.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			var opts []ConstantFoldingOption
			if tc.knownValues != nil {
				act, err := NewActivation(tc.knownValues)
				if err != nil {
					t.Fatalf("NewActivation() failed: %v", err)
				}
				opts = append(opts, FoldKnownValues(act))
			}
			opts = append(opts, tc.foldingOpts...)
			folder, err := NewConstantFoldingOptimizer(opts...)
			if err != nil {
				t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				t.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			optimized, iss := opt.Optimize(env, checked)
			if iss.Err() != nil {
				t.Fatalf("Optimize() failed: %v", iss.Err())
			}

			gotFolded, err := AstToString(optimized)
			if err != nil {
				t.Fatalf("AstToString() failed: %v", err)
			}
			if gotFolded != tc.wantFolded {
				t.Errorf("AstToString() got %q, want %q", gotFolded, tc.wantFolded)
			}
			if tc.forbidSubstr != "" && strings.Contains(gotFolded, tc.forbidSubstr) {
				t.Errorf("AstToString() = %q unexpectedly contains %q", gotFolded, tc.forbidSubstr)
			}
			if tc.checkMacroCnt {
				if gotMacros := len(optimized.NativeRep().SourceInfo().MacroCalls()); gotMacros != tc.wantMacroCount {
					t.Errorf("len(MacroCalls()) = %d, want %d", gotMacros, tc.wantMacroCount)
				}
			}

			// 6. AST ID Normalization & CheckedExpr Round-Trip Verification
			checkedPB, err := AstToCheckedExpr(optimized)
			if err != nil {
				t.Fatalf("AstToCheckedExpr() failed: %v", err)
			}
			wireBytes, err := proto.Marshal(checkedPB)
			if err != nil {
				t.Fatalf("proto.Marshal() failed: %v", err)
			}
			roundTripPB := &exprpb.CheckedExpr{}
			if err := proto.Unmarshal(wireBytes, roundTripPB); err != nil {
				t.Fatalf("proto.Unmarshal() failed: %v", err)
			}
			roundTripAST := CheckedExprToAst(roundTripPB)

			origPrg, err := env.Program(checked)
			if err != nil {
				t.Fatalf("env.Program(checked) failed: %v", err)
			}
			optPrg, err := env.Program(optimized)
			if err != nil {
				t.Fatalf("env.Program(optimized) failed: %v", err)
			}
			rtPrg, err := env.Program(roundTripAST)
			if err != nil {
				t.Fatalf("env.Program(roundTripAST) failed: %v", err)
			}

			for _, in := range tc.evalInputs {
				evalVars := map[string]any{}
				if tc.knownValues != nil {
					maps.Copy(evalVars, tc.knownValues)
				}
				maps.Copy(evalVars, in)

				wantOut, _, wantErr := origPrg.Eval(evalVars)
				gotOut, _, gotErr := optPrg.Eval(in)
				rtOut, _, rtErr := rtPrg.Eval(in)

				if (gotErr != nil) != (wantErr != nil) {
					t.Errorf("optPrg.Eval(%v) error = %v, want %v", in, gotErr, wantErr)
					continue
				}
				if gotErr == nil && gotOut.Equal(wantOut) != types.True {
					t.Errorf("optPrg.Eval(%v) = %v, want %v", in, gotOut, wantOut)
				}
				if (rtErr != nil) != (wantErr != nil) {
					t.Errorf("rtPrg.Eval(%v) error = %v, want %v", in, rtErr, wantErr)
					continue
				}
				if rtErr == nil && rtOut.Equal(wantOut) != types.True {
					t.Errorf("rtPrg.Eval(%v) = %v, want %v", in, rtOut, wantOut)
				}
			}
		})
	}
}

func TestConstantFolding_ComprehensionUnrollBalancedDepth(t *testing.T) {
	const count = 100
	items := make([]map[string]any, count)
	for i := 0; i < count; i++ {
		items[i] = map[string]any{
			"id":        int64(i),
			"huge_blob": strings.Repeat("blob", 256),
		}
	}
	env, err := NewEnv(
		EnableMacroCallTracking(),
		Variable("a", ListType(MapType(StringType, DynType))),
		Variable("target_id", IntType),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	checked, iss := env.Compile(`a.exists(x, x.id == target_id)`)
	if iss.Err() != nil {
		t.Fatalf("Compile() failed: %v", iss.Err())
	}
	act, err := NewActivation(map[string]any{"a": items})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}
	folder, err := NewConstantFoldingOptimizer(FoldKnownValues(act))
	if err != nil {
		t.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
	}
	opt, err := NewStaticOptimizer(folder)
	if err != nil {
		t.Fatalf("NewStaticOptimizer() failed: %v", err)
	}
	optimized, iss := opt.Optimize(env, checked)
	if iss.Err() != nil {
		t.Fatalf("Optimize() failed: %v", iss.Err())
	}
	// A balanced binary tree of 100 comparisons has depth ceil(log2(100)) + 1 = 8,
	// well under a bound of 15 (whereas a linear chain would have depth > 100).
	if ast.ExceedsDepth(optimized.NativeRep(), 15) {
		t.Errorf("ast.ExceedsDepth(optimized, 15) = true, want balanced tree depth <= 15")
	}
	prg, err := env.Program(optimized)
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]any{"target_id": int64(99)})
	if err != nil || out != types.True {
		t.Errorf("prg.Eval(target_id=99) = %v, %v; want true, nil", out, err)
	}
	out, _, err = prg.Eval(map[string]any{"target_id": int64(101)})
	if err != nil || out != types.False {
		t.Errorf("prg.Eval(target_id=101) = %v, %v; want false, nil", out, err)
	}
}

func TestConstantFolding_ComprehensionUnrollASTEdgeCases(t *testing.T) {
	env, err := NewEnv(
		EnableMacroCallTracking(),
		Variable("target_id", IntType),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	// 1. Two-variable comprehension (IterVar2 != "") must not be unrolled.
	t.Run("two_var_comprehension_not_unrolled", func(t *testing.T) {
		fac := ast.NewExprFactory()
		comp := fac.NewComprehensionTwoVar(
			1,
			fac.NewList(2, []ast.Expr{
				fac.NewLiteral(3, types.Int(10)),
				fac.NewLiteral(4, types.Int(20)),
			}, []int32{}),
			"i",
			"v",
			"@result",
			fac.NewLiteral(5, types.False),
			fac.NewCall(6, operators.NotStrictlyFalse,
				fac.NewCall(7, operators.LogicalNot, fac.NewIdent(8, "@result")),
			),
			fac.NewCall(9, operators.LogicalOr,
				fac.NewIdent(10, "@result"),
				fac.NewCall(11, operators.Equals,
					fac.NewIdent(12, "v"),
					fac.NewIdent(13, "target_id"),
				),
			),
			fac.NewIdent(14, "@result"),
		)
		checked := ast.NewCheckedAST(
			ast.NewAST(comp, ast.NewSourceInfo(nil)),
			map[int64]*types.Type{11: types.BoolType},
			map[int64]*ast.ReferenceInfo{},
		)
		folder := &constantFoldingOptimizer{
			maxFoldIterations: defaultMaxConstantFoldIterations,
			maxUnrollSize:     defaultMaxComprehensionUnrollSize,
		}
		ctx := &OptimizerContext{
			Env: env,
			optimizerExprFactory: &optimizerExprFactory{
				idGenerator: newIDGenerator(20),
				fac:         fac,
				sourceInfo:  checked.SourceInfo(),
			},
		}
		if folder.tryUnrollComprehension(ctx, checked, ast.NavigateAST(checked)) {
			t.Errorf("expected two-variable comprehension (IterVar2 != \"\") not to be unrolled")
		}
	})

	// 2. Predicate referencing AccuVar (@result) directly must not be unrolled.
	t.Run("predicate_referencing_accu_var_not_unrolled", func(t *testing.T) {
		fac := ast.NewExprFactory()
		comp := fac.NewComprehension(
			1,
			fac.NewList(2, []ast.Expr{
				fac.NewLiteral(3, types.Int(10)),
			}, []int32{}),
			"x",
			"@result",
			fac.NewLiteral(4, types.False),
			fac.NewCall(5, operators.NotStrictlyFalse,
				fac.NewCall(6, operators.LogicalNot, fac.NewIdent(7, "@result")),
			),
			fac.NewCall(8, operators.LogicalOr,
				fac.NewIdent(9, "@result"),
				fac.NewCall(10, operators.LogicalAnd,
					fac.NewIdent(11, "@result"),
					fac.NewCall(12, operators.Equals,
						fac.NewIdent(13, "x"),
						fac.NewIdent(14, "target_id"),
					),
				),
			),
			fac.NewIdent(15, "@result"),
		)
		checked := ast.NewCheckedAST(
			ast.NewAST(comp, ast.NewSourceInfo(nil)),
			map[int64]*types.Type{10: types.BoolType},
			map[int64]*ast.ReferenceInfo{},
		)
		folder := &constantFoldingOptimizer{
			maxFoldIterations: defaultMaxConstantFoldIterations,
			maxUnrollSize:     defaultMaxComprehensionUnrollSize,
		}
		ctx := &OptimizerContext{
			Env: env,
			optimizerExprFactory: &optimizerExprFactory{
				idGenerator: newIDGenerator(20),
				fac:         fac,
				sourceInfo:  checked.SourceInfo(),
			},
		}
		if folder.tryUnrollComprehension(ctx, checked, ast.NavigateAST(checked)) {
			t.Errorf("expected comprehension with free AccuVar reference in predicate not to be unrolled")
		}
	})

	// 3. Commuted LoopStep (predTemplate || @result instead of @result || predTemplate) unrolls cleanly.
	t.Run("commuted_loop_step_unrolled", func(t *testing.T) {
		fac := ast.NewExprFactory()
		comp := fac.NewComprehension(
			1,
			fac.NewList(2, []ast.Expr{
				fac.NewLiteral(3, types.Int(10)),
				fac.NewLiteral(4, types.Int(20)),
			}, []int32{}),
			"x",
			"@result",
			fac.NewLiteral(5, types.False),
			fac.NewCall(6, operators.NotStrictlyFalse,
				fac.NewCall(7, operators.LogicalNot, fac.NewIdent(8, "@result")),
			),
			fac.NewCall(9, operators.LogicalOr,
				fac.NewCall(10, operators.Equals,
					fac.NewIdent(11, "x"),
					fac.NewIdent(12, "target_id"),
				),
				fac.NewIdent(13, "@result"),
			),
			fac.NewIdent(14, "@result"),
		)
		checked := ast.NewCheckedAST(
			ast.NewAST(comp, ast.NewSourceInfo(nil)),
			map[int64]*types.Type{10: types.BoolType},
			map[int64]*ast.ReferenceInfo{},
		)
		folder := &constantFoldingOptimizer{
			maxFoldIterations: defaultMaxConstantFoldIterations,
			maxUnrollSize:     defaultMaxComprehensionUnrollSize,
		}
		ctx := &OptimizerContext{
			Env: env,
			optimizerExprFactory: &optimizerExprFactory{
				idGenerator: newIDGenerator(20),
				fac:         fac,
				sourceInfo:  checked.SourceInfo(),
			},
		}
		if !folder.tryUnrollComprehension(ctx, checked, ast.NavigateAST(checked)) {
			t.Fatalf("expected comprehension with commuted LoopStep to be unrolled")
		}
		got, err := AstToString(&Ast{impl: checked})
		if err != nil {
			t.Fatalf("AstToString() failed: %v", err)
		}
		if got != "10 == target_id || 20 == target_id" {
			t.Errorf("AstToString() = %q, want %q", got, "10 == target_id || 20 == target_id")
		}
	})

	// 4. Inner cel.bind shadowing AccuVar (x) vs substituting x in AccuInit (varInit).
	t.Run("cel_bind_shadowing_and_accu_init_substitution", func(t *testing.T) {
		fac := ast.NewExprFactory()
		info := ast.NewSourceInfo(nil)
		ctx := &OptimizerContext{
			Env: env,
			optimizerExprFactory: &optimizerExprFactory{
				idGenerator: newIDGenerator(100),
				fac:         fac,
				sourceInfo:  info,
			},
		}
		// Inner bind: cel.bind(x, x + 10, x == target_id)
		varInit := fac.NewCall(10, operators.Add, fac.NewIdent(11, "x"), fac.NewLiteral(12, types.Int(10)))
		body := fac.NewCall(13, operators.Equals, fac.NewIdent(14, "x"), fac.NewIdent(15, "target_id"))
		bindExpr, _ := ctx.NewBindMacro(20, "x", varInit, body)

		comp := fac.NewComprehension(
			1,
			fac.NewList(2, []ast.Expr{
				fac.NewLiteral(3, types.Int(1)),
				fac.NewLiteral(4, types.Int(2)),
			}, []int32{}),
			"x",
			"@result",
			fac.NewLiteral(5, types.False),
			fac.NewCall(6, operators.NotStrictlyFalse,
				fac.NewCall(7, operators.LogicalNot, fac.NewIdent(8, "@result")),
			),
			fac.NewCall(9, operators.LogicalOr,
				fac.NewIdent(16, "@result"),
				bindExpr,
			),
			fac.NewIdent(17, "@result"),
		)
		checked := ast.NewCheckedAST(
			ast.NewAST(comp, info),
			map[int64]*types.Type{20: types.BoolType, 13: types.BoolType},
			map[int64]*ast.ReferenceInfo{},
		)
		folder := &constantFoldingOptimizer{
			maxFoldIterations: defaultMaxConstantFoldIterations,
			maxUnrollSize:     defaultMaxComprehensionUnrollSize,
		}
		if !folder.tryUnrollComprehension(ctx, checked, ast.NavigateAST(checked)) {
			t.Fatalf("expected comprehension with inner cel.bind to be unrolled")
		}
		// Verify that in both unrolled branches, the inner bind's AccuInit had x substituted
		// (1 + 10 and 2 + 10), while the inner bind's Result kept Ident("x") shadowed.
		topCall := checked.Expr().AsCall()
		leftBind := topCall.Args()[0].AsComprehension()
		rightBind := topCall.Args()[1].AsComprehension()
		if leftBind.AccuInit().AsCall().Args()[0].AsLiteral() != types.Int(1) {
			t.Errorf("left bind AccuInit arg[0] = %v, want 1", leftBind.AccuInit().AsCall().Args()[0])
		}
		if rightBind.AccuInit().AsCall().Args()[0].AsLiteral() != types.Int(2) {
			t.Errorf("right bind AccuInit arg[0] = %v, want 2", rightBind.AccuInit().AsCall().Args()[0])
		}
		if leftBind.Result().AsCall().Args()[0].Kind() != ast.IdentKind || leftBind.Result().AsCall().Args()[0].AsIdent() != "x" {
			t.Errorf("left bind Result arg[0] = %v, want shadowed Ident(\"x\")", leftBind.Result().AsCall().Args()[0])
		}
	})
}

func makeBenchmarkAttestations(n int) []map[string]any {
	blob := strings.Repeat("attestation_payload_", 50) // ~1 KB payload per element
	items := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		envName := "dev"
		if i == n-1 {
			envName = "prod"
		}
		items[i] = map[string]any{
			"name":      fmt.Sprintf("service-%d", i),
			"env":       envName,
			"huge_blob": blob,
			"predicate": map[string]any{
				"builder": map[string]any{
					"id": fmt.Sprintf("https://builder.example.com/v%d", i),
				},
				"metadata": map[string]any{
					"invocation_id": fmt.Sprintf("inv-%d", i),
				},
			},
		}
	}
	return items
}

func BenchmarkConstantFolding_ComprehensionUnroll(b *testing.B) {
	sizes := []int{2, 10, 50, 100}
	env, err := NewEnv(
		EnableMacroCallTracking(),
		Variable("a", ListType(MapType(StringType, DynType))),
		Variable("baz", MapType(StringType, DynType)),
	)
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	const exprText = `a.exists(x, x.name == baz.name && x.predicate.builder.id == baz.expected_builder)`
	checked, iss := env.Compile(exprText)
	if iss.Err() != nil {
		b.Fatalf("Compile() failed: %v", iss.Err())
	}
	const pruneExprText = `a.exists(x, x.env == "prod" && x.name == baz.name && x.predicate.builder.id == baz.expected_builder)`
	checkedPrune, iss := env.Compile(pruneExprText)
	if iss.Err() != nil {
		b.Fatalf("Compile(pruneExprText) failed: %v", iss.Err())
	}

	for _, n := range sizes {
		items := makeBenchmarkAttestations(n)
		act, err := NewActivation(map[string]any{"a": items})
		if err != nil {
			b.Fatalf("NewActivation() failed: %v", err)
		}

		b.Run(fmt.Sprintf("CompileAndOptimize/N=%d", n), func(b *testing.B) {
			folder, err := NewConstantFoldingOptimizer(FoldKnownValues(act))
			if err != nil {
				b.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				b.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			var optimized *Ast
			for i := 0; i < b.N; i++ {
				var optIss *Issues
				optimized, optIss = opt.Optimize(env, checked)
				if optIss.Err() != nil {
					b.Fatalf("Optimize() failed: %v", optIss.Err())
				}
			}
			b.StopTimer()
			pb, err := AstToCheckedExpr(optimized)
			if err != nil {
				b.Fatalf("AstToCheckedExpr() failed: %v", err)
			}
			wire, err := proto.Marshal(pb)
			if err != nil {
				b.Fatalf("proto.Marshal() failed: %v", err)
			}
			b.ReportMetric(float64(len(wire)), "proto_bytes")
		})

		evalInput := map[string]any{
			"baz": map[string]any{
				"name":             fmt.Sprintf("service-%d", n-1),
				"expected_builder": fmt.Sprintf("https://builder.example.com/v%d", n-1),
			},
		}

		b.Run(fmt.Sprintf("RuntimeEval_Baseline_Ununrolled/N=%d", n), func(b *testing.B) {
			folder, err := NewConstantFoldingOptimizer(
				FoldKnownValues(act),
				MaxComprehensionUnrollSize(0),
			)
			if err != nil {
				b.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				b.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			baselineAST, optIss := opt.Optimize(env, checked)
			if optIss.Err() != nil {
				b.Fatalf("Optimize() failed: %v", optIss.Err())
			}
			pb, err := AstToCheckedExpr(baselineAST)
			if err != nil {
				b.Fatalf("AstToCheckedExpr() failed: %v", err)
			}
			wire, err := proto.Marshal(pb)
			if err != nil {
				b.Fatalf("proto.Marshal() failed: %v", err)
			}
			prg, err := env.Program(baselineAST)
			if err != nil {
				b.Fatalf("Program() failed: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, _, err := prg.Eval(evalInput)
				if err != nil || out != types.True {
					b.Fatalf("Eval() = %v, %v; want true, nil", out, err)
				}
			}
			b.ReportMetric(float64(len(wire)), "proto_bytes")
		})

		b.Run(fmt.Sprintf("RuntimeEval_Unrolled_ScalarFolded/N=%d", n), func(b *testing.B) {
			folder, err := NewConstantFoldingOptimizer(FoldKnownValues(act))
			if err != nil {
				b.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				b.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			unrolledAST, optIss := opt.Optimize(env, checked)
			if optIss.Err() != nil {
				b.Fatalf("Optimize() failed: %v", optIss.Err())
			}
			pb, err := AstToCheckedExpr(unrolledAST)
			if err != nil {
				b.Fatalf("AstToCheckedExpr() failed: %v", err)
			}
			wire, err := proto.Marshal(pb)
			if err != nil {
				b.Fatalf("proto.Marshal() failed: %v", err)
			}
			prg, err := env.Program(unrolledAST)
			if err != nil {
				b.Fatalf("Program() failed: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, _, err := prg.Eval(evalInput)
				if err != nil || out != types.True {
					b.Fatalf("Eval() = %v, %v; want true, nil", out, err)
				}
			}
			b.ReportMetric(float64(len(wire)), "proto_bytes")
		})

		b.Run(fmt.Sprintf("RuntimeEval_Unrolled_BranchPruned/N=%d", n), func(b *testing.B) {
			folder, err := NewConstantFoldingOptimizer(FoldKnownValues(act))
			if err != nil {
				b.Fatalf("NewConstantFoldingOptimizer() failed: %v", err)
			}
			opt, err := NewStaticOptimizer(folder)
			if err != nil {
				b.Fatalf("NewStaticOptimizer() failed: %v", err)
			}
			prunedAST, optIss := opt.Optimize(env, checkedPrune)
			if optIss.Err() != nil {
				b.Fatalf("Optimize() failed: %v", optIss.Err())
			}
			pb, err := AstToCheckedExpr(prunedAST)
			if err != nil {
				b.Fatalf("AstToCheckedExpr() failed: %v", err)
			}
			wire, err := proto.Marshal(pb)
			if err != nil {
				b.Fatalf("proto.Marshal() failed: %v", err)
			}
			prg, err := env.Program(prunedAST)
			if err != nil {
				b.Fatalf("Program() failed: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, _, err := prg.Eval(evalInput)
				if err != nil || out != types.True {
					b.Fatalf("Eval() = %v, %v; want true, nil", out, err)
				}
			}
			b.ReportMetric(float64(len(wire)), "proto_bytes")
		})
	}
}
