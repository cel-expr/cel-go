// Copyright 2019 Google LLC
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

package interpreter

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	anypb "google.golang.org/protobuf/types/known/anypb"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func TestAttributesAbsoluteAttr(t *testing.T) {
	reg := newTestRegistry(t)
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	vars, _ := NewActivation(map[string]any{
		"acme.a": map[string]any{
			"b": map[uint]any{
				4: map[bool]string{
					false: "success",
				},
			},
		},
	})

	// acme.a.b[4][false]
	attr := attrs.AbsoluteAttribute(1, "acme.a")
	qualB := makeQualifier(t, attrs, nil, 2, "b")
	qual4 := makeQualifier(t, attrs, nil, 3, uint64(4))
	qualFalse := makeQualifier(t, attrs, nil, 4, false)
	attr.AddQualifier(qualB)
	attr.AddQualifier(qual4)
	attr.AddQualifier(qualFalse)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.String("success") {
		t.Errorf("Got %v (%T), wanted success", out, out)
	}
}

func TestAttributesAbsoluteAttrType(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)

	// int
	attr := attrs.AbsoluteAttribute(1, "int")
	out, err := attr.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	if out != types.IntType {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributesAbsoluteAttrError(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	vars, err := NewActivation(map[string]any{
		"err": types.NewErr("invalid variable computation"),
	})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}

	// acme.a.b[4][false]
	attr := attrs.AbsoluteAttribute(1, "err")
	qualMsg := makeQualifier(t, attrs, nil, 2, "message")
	attr.AddQualifier(qualMsg)
	out, err := attr.Resolve(vars)
	if err == nil {
		t.Errorf("attr.Resolve('err') got %v, wanted error", out)
	}
}

func TestAttributesRelativeAttr(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]any{
		"a": map[int]any{
			-1: []int32{2, 42},
		},
		"b": 1,
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: 1
	// }
	//
	// The expression being evaluated is: <map-literal>.a[-1][b] -> 42
	op := NewConstValue(1, reg.NativeToValue(data))
	attr := attrs.RelativeAttribute(1, op)
	qualA := makeQualifier(t, attrs, nil, 2, "a")
	qualNeg1 := makeQualifier(t, attrs, nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(attrs.AbsoluteAttribute(4, "b"))
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributesRelativeAttrOneOf(t *testing.T) {
	reg := newTestRegistry(t)
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	data := map[string]any{
		"a": map[int]any{
			-1: []int32{2, 42},
		},
		"acme.b": 1,
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: 1
	// }
	//
	// The expression being evaluated is: <map-literal>.a[-1][b] -> 42
	//
	// However, since the test is validating what happens with maybe attributes
	// the attribute resolution must also consider the following variations:
	// - <map-literal>.a[-1][acme.ns.b]
	// - <map-literal>.a[-1][acme.b]
	//
	// The correct behavior should yield the value of the last alternative.
	op := NewConstValue(1, reg.NativeToValue(data))
	attr := attrs.RelativeAttribute(1, op)
	qualA := makeQualifier(t, attrs, nil, 2, "a")
	qualNeg1 := makeQualifier(t, attrs, nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(attrs.MaybeAttribute(4, "b"))
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributesRelativeAttrConditional(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]any{
		"a": map[int]any{
			-1: []int32{2, 42},
		},
		"b": []int{0, 1},
		"c": []any{1, 0},
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: [0, 1],
	//   c: {1, 0},
	// }
	//
	// The expression being evaluated is:
	// <map-literal>.a[-1][(false ? b : c)[0]] -> 42
	//
	// Effectively the same as saying <map-literal>.a[-1][c[0]]
	cond := NewConstValue(2, types.False)
	condAttr := attrs.ConditionalAttribute(4, cond,
		attrs.AbsoluteAttribute(5, "b"),
		attrs.AbsoluteAttribute(6, "c"))
	qual0 := makeQualifier(t, attrs, nil, 7, 0)
	condAttr.AddQualifier(qual0)

	obj := NewConstValue(1, reg.NativeToValue(data))
	attr := attrs.RelativeAttribute(1, obj)
	qualA := makeQualifier(t, attrs, nil, 2, "a")
	qualNeg1 := makeQualifier(t, attrs, nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(condAttr)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributesRelativeAttrRelativeQualifier(t *testing.T) {
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	data := map[string]any{
		"a": map[int]any{
			-1: map[string]any{
				"first":  uint(1),
				"second": uint(2),
				"third":  uint(3),
			},
		},
		"b": uint32(2),
	}
	vars, _ := NewActivation(data)

	// The environment declares the following variables:
	// {
	//   a: {
	//     -1: {
	//       "first": 1u,
	//       "second": 2u,
	//       "third": 3u,
	//     }
	//   },
	//   b: 2u,
	// }
	//
	// The map of input variables is also re-used as a map-literal <obj> in the expression.
	//
	// The relative object under test is the following map literal.
	// <mp> {
	//   1u: "first",
	//   2u: "second",
	//   3u: "third",
	// }
	//
	// The expression under test is:
	//   <obj>.a[-1][<mp>[b]]
	//
	// This is equivalent to:
	//   <obj>.a[-1]["second"] -> 2u
	obj := NewConstValue(1, reg.NativeToValue(data))
	mp := NewConstValue(1, reg.NativeToValue(map[uint32]any{
		1: "first",
		2: "second",
		3: "third",
	}))
	relAttr := attrs.RelativeAttribute(4, mp)
	qualB := makeQualifier(t, attrs, nil, 5, attrs.AbsoluteAttribute(5, "b"))
	relAttr.AddQualifier(qualB)
	attr := attrs.RelativeAttribute(1, obj)
	qualA := makeQualifier(t, attrs, nil, 2, "a")
	qualNeg1 := makeQualifier(t, attrs, nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(relAttr)

	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Uint(2) {
		t.Errorf("Got %v (%T), wanted 2", out, out)
	}
}

func TestAttributesOneofAttr(t *testing.T) {
	reg := newTestRegistry(t)
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	data := map[string]any{
		"a": map[string]any{
			"b": []int32{2, 42},
		},
		"acme.a.b":    1,
		"acme.ns.a.b": "found",
	}
	vars, _ := NewActivation(data)

	// a.b -> should resolve to acme.ns.a.b per namespace resolution rules.
	attr := attrs.MaybeAttribute(1, "a")
	qualB := makeQualifier(t, attrs, nil, 2, "b")
	attr.AddQualifier(qualB)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "found" {
		t.Errorf("Got %v, wanted 'found'", out)
	}
}

func TestAttributesConditionalAttrTrueBranch(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]any{
		"a": map[int]any{
			-1: []int32{2, 42},
		},
		"b": map[string]any{
			"c": map[int32]any{
				-1: []uint{2, 42},
			},
		},
	}
	vars, _ := NewActivation(data)

	// (true ? a : b.c)[-1][1]
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	qualC := makeQualifier(t, attrs, nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := attrs.ConditionalAttribute(1, NewConstValue(0, types.True), tv, fv)
	qualNeg1 := makeQualifier(t, attrs, nil, 5, int64(-1))
	qual1 := makeQualifier(t, attrs, nil, 6, int64(1))
	cond.AddQualifier(qualNeg1)
	cond.AddQualifier(qual1)
	out, err := cond.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != int32(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributesConditionalAttrFalseBranch(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]any{
		"a": map[int]any{
			-1: []int32{2, 42},
		},
		"b": map[string]any{
			"c": map[int32]any{
				-1: []uint{2, 42},
			},
		},
	}
	vars, _ := NewActivation(data)

	// (false ? a : b.c)[-1][1]
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	qualC := makeQualifier(t, attrs, nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := attrs.ConditionalAttribute(1, NewConstValue(0, types.False), tv, fv)
	qualNeg1 := makeQualifier(t, attrs, nil, 5, int64(-1))
	qual1 := makeQualifier(t, attrs, nil, 6, int64(1))
	cond.AddQualifier(qualNeg1)
	cond.AddQualifier(qual1)
	out, err := cond.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != uint(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributesNarrowMapKeyQualifier(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	vars, err := NewActivation(map[string]any{
		"i32": map[int32]any{0: "zero"},
		"u32": map[uint32]any{0: "zero"},
	})
	if err != nil {
		t.Fatalf("NewActivation() failed: %v", err)
	}
	// An index outside the key type's range must not be truncated into a
	// matching key. int32(1<<32) and uint32(1<<32) both wrap to 0.
	tests := []struct {
		varName string
		qual    any
		out     any
		err     error
	}{
		{varName: "i32", qual: int64(1) << 32, err: errors.New("no such key: 4294967296")},
		{varName: "u32", qual: uint64(1) << 32, err: errors.New("no such key: 4294967296")},
		{varName: "i32", qual: int64(0), out: "zero"},
		{varName: "u32", qual: uint64(0), out: "zero"},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			attr := attrs.AbsoluteAttribute(1, tc.varName)
			attr.AddQualifier(makeQualifier(t, attrs, nil, 2, tc.qual))
			out, err := attr.Resolve(vars)
			if err != nil {
				if tc.err == nil {
					t.Fatalf("attr.Resolve() failed: %v", err)
				}
				if tc.err.Error() != err.Error() {
					t.Fatalf("attr.Resolve() errored with %v, wanted error %v", err, tc.err)
				}
				return
			}
			if tc.err != nil {
				t.Fatalf("attr.Resolve() got %v, wanted error %v", out, tc.err)
			}
			if out != tc.out {
				t.Errorf("attr.Resolve() got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestAttributesOptional(t *testing.T) {
	reg := newTestRegistry(t, types.ProtoTypeDefs(&proto3pb.TestAllTypes{}))
	cont, err := containers.NewContainer(containers.Name("ns"))
	if err != nil {
		t.Fatalf("")
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	tests := []struct {
		varName  string
		quals    []any
		optQuals []any
		vars     map[string]any
		out      any
		err      error
	}{
		{
			// a.?b[0][false]
			varName:  "a",
			optQuals: []any{"b", int32(0), false},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{
						0: map[bool]string{
							false: "success",
						},
					},
				},
			},
			out: types.OptionalOf(reg.NativeToValue("success")),
		},
		{
			// a.?b[0][false]
			varName:  "a",
			optQuals: []any{"b", uint32(0), false},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{
						0: map[bool]string{
							false: "success",
						},
					},
				},
			},
			out: types.OptionalOf(reg.NativeToValue("success")),
		},
		{
			// a.?b[0][false]
			varName:  "a",
			optQuals: []any{"b", float32(0), false},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{
						0: map[bool]string{
							false: "success",
						},
					},
				},
			},
			out: types.OptionalOf(reg.NativeToValue("success")),
		},
		{
			// a.?b[1] with no value
			varName:  "a",
			optQuals: []any{"b", uint(1)},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[uint]any{},
				},
			},
			out: types.OptionalNone,
		},
		{
			// a.b[1] with no value where b is a map[uint]
			varName: "a",
			quals:   []any{"b", uint(1)},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[uint]any{},
				},
			},
			err: errors.New("no such key: 1"),
		},
		{
			// a.b[?1] with no value where 'b' is a []int
			varName:  "a",
			quals:    []any{"b"},
			optQuals: []any{1},
			vars: map[string]any{
				"a": map[string]any{
					"b": []int{},
				},
			},
			out: types.OptionalNone,
		},
		{
			// a.b[1] with no value where 'b' is a map[int]any
			varName: "a",
			quals:   []any{"b", 1},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{},
				},
			},
			err: errors.New("no such key: 1"),
		},
		{
			// a.b[?1] with no value where 'b' is a []int
			varName:  "a",
			quals:    []any{"b", 1, false},
			optQuals: []any{},
			vars: map[string]any{
				"a": map[string]any{
					"b": []int{},
				},
			},
			err: errors.New("index out of bounds: 1"),
		},
		{
			// a.?b[0][true] with no value
			varName:  "a",
			optQuals: []any{"b", 0, false},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{
						0: map[bool]any{},
					},
				},
			},
			out: types.OptionalNone,
		},
		{
			// a.b[0][?true] with no value
			varName:  "a",
			quals:    []any{"b", 0},
			optQuals: []any{true},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{
						0: map[bool]any{},
					},
				},
			},
			out: types.OptionalNone,
		},
		{
			// a.b[0][true] with no value
			varName: "a",
			quals:   []any{"b", 0, true},
			vars: map[string]any{
				"a": map[string]any{
					"b": map[int]any{
						0: map[bool]any{},
					},
				},
			},
			err: errors.New("no such key: true"),
		},
		{
			// a.b[0][false] where 'a' is optional
			varName: "a",
			quals:   []any{"b", int32(0), false},
			vars: map[string]any{
				"a": types.OptionalOf(reg.NativeToValue(map[string]any{
					"b": map[int]any{
						0: map[bool]string{
							false: "success",
						},
					},
				})),
			},
			out: types.OptionalOf(reg.NativeToValue("success")),
		},
		{
			// a.b[0][false] where 'a' is optional none.
			varName: "a",
			quals:   []any{"b", int32(0), false},
			vars: map[string]any{
				"a": types.OptionalNone,
			},
			out: types.OptionalNone,
		},
		{
			// a.?c[1][true]
			varName:  "a",
			optQuals: []any{"c", int32(1), true},
			vars: map[string]any{
				"a": map[string]any{},
			},
			out: types.OptionalNone,
		},
		{
			// a[?b] where 'b' is dynamically computed.
			varName:  "a",
			optQuals: []any{attrs.AbsoluteAttribute(0, "b")},
			vars: map[string]any{
				"a": map[string]any{
					"hello": "world",
				},
				"b": "hello",
			},
			out: types.OptionalOf(reg.NativeToValue("world")),
		},
		{
			// a[?(false ? : b : c.d.e)]
			varName: "a",
			optQuals: []any{
				attrs.ConditionalAttribute(0,
					NewConstValue(100, types.False),
					attrs.AbsoluteAttribute(101, "b"),
					attrs.MaybeAttribute(102, "c.d.e")),
			},
			vars: map[string]any{
				"a": map[string]any{
					"hello":   "world",
					"goodbye": "universe",
				},
				"b":     "hello",
				"c.d.e": "goodbye",
			},
			out: types.OptionalOf(reg.NativeToValue("universe")),
		},
		{
			// a[?c.d.e]
			varName: "a",
			optQuals: []any{
				attrs.MaybeAttribute(102, "c.d.e"),
			},
			vars: map[string]any{
				"a": map[string]any{
					"hello":   "world",
					"goodbye": "universe",
				},
				"b":     "hello",
				"c.d.e": "goodbye",
			},
			out: types.OptionalOf(reg.NativeToValue("universe")),
		},
		{
			// a[c.d.e] where the c.d.e errors
			varName: "a",
			quals: []any{
				addQualifier(t, attrs.MaybeAttribute(102, "c.d"), makeQualifier(t, attrs, nil, 103, "e")),
			},
			vars: map[string]any{
				"a": map[string]any{
					"goodbye": "universe",
				},
				"c.d": map[string]any{},
			},
			err: errors.New("no such key: e"),
		},
		{
			// a[?c.d.e] where the c.d.e errors
			varName: "a",
			optQuals: []any{
				addQualifier(t, attrs.MaybeAttribute(102, "c.d"), makeQualifier(t, attrs, nil, 103, "e")),
			},
			vars: map[string]any{
				"a": map[string]any{
					"goodbye": "universe",
				},
				"c.d": map[string]any{},
			},
			err: errors.New("no such key: e"),
		},
		{
			// a.?single_int32 with a value.
			varName:  "a",
			optQuals: []any{"single_int32"},
			vars: map[string]any{
				"a": &proto3pb.TestAllTypes{SingleInt32: 1},
			},
			out: types.OptionalOf(reg.NativeToValue(1)),
		},
		{
			// a.?single_int32 where the field is not set.
			varName:  "a",
			optQuals: []any{"single_int32"},
			vars: map[string]any{
				"a": &proto3pb.TestAllTypes{},
			},
			out: types.OptionalNone,
		},
		{
			// a.?single_int32 where the field is set (uses more optimal selection logic)
			varName: "a",
			optQuals: []any{
				makeOptQualifier(t,
					attrs,
					types.NewObjectType("google.expr.proto3.test.TestAllTypes"),
					103,
					"single_int32",
				),
			},
			vars: map[string]any{
				"a": &proto3pb.TestAllTypes{SingleInt32: 1},
			},
			out: types.OptionalOf(reg.NativeToValue(1)),
		},
		{
			// a.c[1][true]
			varName: "a",
			quals:   []any{"c", int32(1), true},
			vars: map[string]any{
				"a": map[string]any{},
			},
			err: errors.New("no such key: c"),
		},
		{
			// a, no bindings
			varName: "a",
			quals:   []any{},
			vars:    map[string]any{},
			err:     errors.New("no such attribute(s): a"),
		},
	}
	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			i := int64(1)
			attr := attrs.AbsoluteAttribute(i, tc.varName)
			for _, q := range tc.quals {
				i++
				attr.AddQualifier(makeQualifier(t, attrs, nil, i, q))
			}
			for _, oq := range tc.optQuals {
				i++
				attr.AddQualifier(makeOptQualifier(t, attrs, nil, i, oq))
			}
			vars, err := NewActivation(tc.vars)
			if err != nil {
				t.Fatalf("NewActivation() failed: %v", err)
			}
			out, err := attr.Resolve(vars)
			if err != nil {
				if tc.err != nil {
					if tc.err.Error() == err.Error() {
						return
					}
					t.Fatalf("attr.Resolve() errored with %v, wanted error %v", err, tc.err)
				}
				t.Fatalf("attr.Resolve() failed: %v", err)
			}
			if !reflect.DeepEqual(out, tc.out) {
				t.Errorf("attr.Resolve() got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestAttributesConditionalAttrErrorUnknown(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)

	// err ? a : b
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	cond := attrs.ConditionalAttribute(1, NewConstValue(0, types.NewErr("test error")), tv, fv)
	out, err := cond.Resolve(EmptyActivation())
	if err == nil {
		t.Errorf("Got %v, wanted error", out)
	}

	// unk ? a : b
	condUnk := attrs.ConditionalAttribute(1, NewConstValue(0, types.NewUnknown(1, nil)), tv, fv)
	out, err = condUnk.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	if !types.IsUnknown(out.(ref.Val)) {
		t.Errorf("Got %v, wanted unknown", out)
	}
}

func BenchmarkResolverFieldQualifier(b *testing.B) {
	msg := &proto3pb.TestAllTypes{
		NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{
			SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{
				Bb: 123,
			},
		},
	}
	reg := newTestRegistry(b, types.ProtoTypeDefs(msg))
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	vars, _ := NewActivation(map[string]any{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	opType, found := reg.FindType("google.expr.proto3.test.TestAllTypes")
	if !found {
		b.Fatal("FindType() could not find TestAllTypes")
	}
	fieldType, found := reg.FindType("google.expr.proto3.test.TestAllTypes.NestedMessage")
	if !found {
		b.Fatal("FindType() could not find NestedMessage")
	}
	attr.AddQualifier(makeQualifier(b, attrs, testExprTypeToType(b, opType), 2, "single_nested_message"))
	attr.AddQualifier(makeQualifier(b, attrs, testExprTypeToType(b, fieldType), 3, "bb"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := attr.Resolve(vars)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestResolverCustomQualifier(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := &custAttrFactory{
		AttributeFactory: NewAttributeFactory(containers.DefaultContainer, reg, reg),
	}
	msg := &proto3pb.TestAllTypes_NestedMessage{
		Bb: 123,
	}
	vars, _ := NewActivation(map[string]any{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	fieldType := types.NewObjectType("google.expr.proto3.test.TestAllTypes.NestedMessage")
	qualBB := makeQualifier(t, attrs, fieldType, 2, "bb")
	attr.AddQualifier(qualBB)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Error(err)
	}
	if out != int32(123) {
		t.Errorf("Got %v, wanted 123", out)
	}
}

func TestAttributesMissingMsg(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	anyPB, _ := anypb.New(&proto3pb.TestAllTypes{})
	vars, _ := NewActivation(map[string]any{
		"missing_msg": anyPB,
	})

	// missing_msg.field
	attr := attrs.AbsoluteAttribute(1, "missing_msg")
	field := makeQualifier(t, attrs, nil, 2, "field")
	attr.AddQualifier(field)
	out, err := attr.Resolve(vars)
	if err == nil {
		t.Fatalf("got %v, wanted error", out)
	}
	if err.Error() != "unknown type: 'google.expr.proto3.test.TestAllTypes'" {
		t.Fatalf("got %v, wanted unknown type: 'google.expr.proto3.test.TestAllTypes'", err)
	}
}

func TestAttributeMissingMsgUnknownField(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
	anyPB, _ := anypb.New(&proto3pb.TestAllTypes{})
	vars, _ := NewPartialActivation(map[string]any{
		"missing_msg": anyPB,
	}, NewAttributePattern("missing_msg").QualString("field"))

	// missing_msg.field
	attr := attrs.AbsoluteAttribute(1, "missing_msg")
	field := makeQualifier(t, attrs, nil, 2, "field")
	attr.AddQualifier(field)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	_, isUnk := out.(*types.Unknown)
	if !isUnk {
		t.Errorf("got %v, wanted unknown value", out)
	}
}

func TestAttributeStateTracking(t *testing.T) {
	var tests = []struct {
		expr  string
		vars  []*decls.VariableDecl
		in    any
		out   ref.Val
		attrs []*AttributePattern
		state map[int64]any
	}{
		{
			expr: `[{"field": true}][0].field`,
			vars: []*decls.VariableDecl{},
			in:   map[string]any{},
			out:  types.True,
			state: map[int64]any{
				// [{"field": true}]
				1: []ref.Val{types.DefaultTypeAdapter.NativeToValue(map[ref.Val]ref.Val{types.String("field"): types.True})},
				// [{"field": true}][0]
				6: map[ref.Val]ref.Val{types.String("field"): types.True},
				// [{"field": true}][0].field
				8: true,
			},
		},
		{
			expr: `a[1]['two']`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(
					types.IntType,
					types.NewMapType(types.StringType, types.BoolType))),
			},
			in: map[string]any{
				"a": map[int64]any{
					1: map[string]bool{
						"two": true,
					},
				},
			},
			out: types.True,
			state: map[int64]any{
				// a[1]
				2: map[string]bool{"two": true},
				// a[1]["two"]
				4: true,
			},
		},
		{
			expr: `a[1][2][3]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(
					types.IntType,
					types.NewMapType(types.DynType, types.DynType))),
			},
			in: map[string]any{
				"a": map[int64]any{
					1: map[int64]any{
						1: 0,
						2: []string{"index", "middex", "outdex", "dex"},
					},
				},
			},
			out: types.String("dex"),
			state: map[int64]any{
				// a[1]
				2: map[int64]any{
					1: 0,
					2: []string{"index", "middex", "outdex", "dex"},
				},
				// a[1][2]
				4: []string{"index", "middex", "outdex", "dex"},
				// a[1][2][3]
				6: "dex",
			},
		},
		{
			expr: `a[1][2][a[1][1]]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(
					types.IntType,
					types.NewMapType(types.DynType, types.DynType))),
			},
			in: map[string]any{
				"a": map[int64]any{
					1: map[int64]any{
						1: 0,
						2: []string{"index", "middex", "outdex", "dex"},
					},
				},
			},
			out: types.String("index"),
			state: map[int64]any{
				// a[1]
				2: map[int64]any{
					1: 0,
					2: []string{"index", "middex", "outdex", "dex"},
				},
				// a[1][2]
				4: []string{"index", "middex", "outdex", "dex"},
				// a[1][2][a[1][1]]
				6: "index",
				// dynamic index into a[1][2]
				// a[1]
				8: map[int64]any{
					1: 0,
					2: []string{"index", "middex", "outdex", "dex"},
				},
				// a[1][1]
				10: int64(0),
			},
		},
		{
			expr: `true ? a : b`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.StringType),
				decls.NewVariable("b", types.StringType),
			},
			in: map[string]any{
				"a": "hello",
				"b": "world",
			},
			out: types.String("hello"),
			state: map[int64]any{
				// 'hello'
				2: types.String("hello"),
			},
		},
		{
			expr: `(a.size() != 0 ? a : b)[0]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewListType(types.StringType)),
				decls.NewVariable("b", types.NewListType(types.StringType)),
			},
			in: map[string]any{
				"a": []string{"hello", "world"},
				"b": []string{"world", "hello"},
			},
			out: types.String("hello"),
			state: map[int64]any{
				// ["hello", "world"]
				1: types.DefaultTypeAdapter.NativeToValue([]string{"hello", "world"}),
				// ["hello", "world"].size() // 2
				2: types.Int(2),
				// ["hello", "world"].size() != 0
				3: types.True,
				// constant 0
				4: types.IntZero,
				// 'hello'
				8: types.String("hello"),
			},
		},
		{
			expr: `a.?b.c`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.NewMapType(types.StringType, types.StringType))),
			},
			in: map[string]any{
				"a": map[string]any{"b": map[string]any{"c": "world"}},
			},
			out: types.OptionalOf(types.String("world")),
			state: map[int64]any{
				// {c: world}
				3: types.DefaultTypeAdapter.NativeToValue(map[string]string{"c": "world"}),
				// 'world'
				4: types.OptionalOf(types.String("world")),
			},
		},
		{
			expr: `a.?b.c`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.NewMapType(types.StringType, types.StringType))),
			},
			in: map[string]any{
				"a": map[string]any{"b": map[string]string{"random": "value"}},
			},
			out: types.OptionalNone,
			state: map[int64]any{
				// {random: value}
				3: types.DefaultTypeAdapter.NativeToValue(map[string]string{"random": "value"}),
				// optional.none()
				4: types.OptionalNone,
			},
		},
		{
			expr: `a.b.c`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.NewMapType(types.StringType, types.StringType))),
			},
			in: map[string]any{
				"a": map[string]any{"b": map[string]any{"c": "world"}},
			},
			out: types.String("world"),
			state: map[int64]any{
				// {c: world}
				2: types.DefaultTypeAdapter.NativeToValue(map[string]string{"c": "world"}),
				// 'world'
				3: types.String("world"),
			},
		},
		{
			expr: `m[has(a.b)]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.StringType)),
				decls.NewVariable("m", types.NewMapType(types.BoolType, types.StringType)),
			},
			in: map[string]any{
				"a": map[string]string{"b": ""},
				"m": map[bool]string{true: "world"},
			},
			out: types.String("world"),
		},
		{
			expr: `m[?has(a.b)]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.StringType)),
				decls.NewVariable("m", types.NewMapType(types.BoolType, types.StringType)),
			},
			in: map[string]any{
				"a": map[string]string{"b": ""},
				"m": map[bool]string{true: "world"},
			},
			out: types.OptionalOf(types.String("world")),
		},
		{
			expr: `m[?has(a.b.c)]`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("a", types.NewMapType(types.StringType, types.DynType)),
				decls.NewVariable("m", types.NewMapType(types.BoolType, types.StringType)),
			},
			in: partialActivation(
				map[string]any{
					"a": map[string]any{},
					"m": map[bool]string{true: "world"},
				},
				NewAttributePattern("a").QualString("b"),
			),
			out: types.NewUnknown(5, types.QualifyAttribute[string](types.NewAttributeTrail("a"), "b")),
		},
		{
			expr: `['a', b.val, 'c'].filter(i, i + 'b' != 'ab')`,
			vars: []*decls.VariableDecl{
				decls.NewVariable("b", types.NewMapType(types.StringType, types.DynType)),
			},
			in: partialActivation(
				map[string]any{
					"b": map[string]any{
						"val": 1,
					},
				},
			),
			// Error node 9 corresponds to the `i + 'b'` expression
			out: types.LabelErrNode(9, types.NoSuchOverloadErr()),
		},
	}
	for _, test := range tests {
		tc := test
		t.Run(tc.expr, func(t *testing.T) {
			src := common.NewTextSource(tc.expr)
			p, err := parser.NewParser(
				parser.EnableOptionalSyntax(true),
				parser.Macros(parser.AllMacros...),
			)
			if err != nil {
				t.Fatalf("parser.NewParser() failed: %v", err)
			}
			parsed, errors := p.Parse(src)
			if len(errors.GetErrors()) != 0 {
				t.Fatal(errors.ToDisplayString())
			}
			cont := containers.DefaultContainer
			reg := newTestRegistry(t)
			env, err := checker.NewEnv(cont, reg)
			if err != nil {
				t.Fatalf("checker.NewEnv() failed: %v", err)
			}
			env.AddFunctions(stdlib.Functions()...)
			env.AddFunctions(optionalDecls(t)...)
			if tc.vars != nil {
				env.AddIdents(tc.vars...)
			}
			checked, errors := checker.Check(parsed, src, env)
			if len(errors.GetErrors()) != 0 {
				t.Fatal(errors.ToDisplayString())
			}
			in, err := NewActivation(tc.in)
			if err != nil {
				t.Fatalf("NewActivation(%v) failed: %v", tc.in, err)
			}
			var attrs AttributeFactory
			_, isPartial := in.(PartialActivation)
			if isPartial {
				attrs = NewPartialAttributeFactory(cont, reg, reg)
			} else {
				attrs = NewAttributeFactory(cont, reg, reg)
			}
			interp := newStandardInterpreter(t, cont, reg, reg, attrs)
			// Show that program planning will now produce an error.
			type stateHolder struct {
				st EvalState
			}
			holder := stateHolder{}
			i, err := interp.NewInterpretable(checked, Optimize(),
				EvalStateObserver(EvalStateFactory(func() EvalState {
					holder.st = NewEvalState()
					return holder.st
				})))
			if err != nil {
				t.Fatal(err)
			}
			out := i.Eval(in)
			if types.IsUnknown(tc.out) && types.IsUnknown(out) {
				if !reflect.DeepEqual(tc.out, out) {
					t.Errorf("got %v, wanted %v", out, tc.out)
				}
			} else if types.IsError(tc.out) && types.IsError(out) {
				if tc.out.(*types.Err).Error() != out.(*types.Err).Error() {
					t.Errorf("got %v, wanted %v", out, tc.out)
				}
			} else if tc.out.Equal(out) != types.True {
				t.Errorf("got %v, wanted %v", out, tc.out)
			}
			if err, isErr := out.(*types.Err); isErr && err.NodeID() != 0 {
				wantErr := tc.out.(*types.Err)
				if err.NodeID() != wantErr.NodeID() {
					t.Errorf("err.NodeID() got %d, wanted %d", err.NodeID(), wantErr.NodeID())
				}
			}
			for id, val := range tc.state {
				stVal, found := holder.st.Value(id)
				if !found {
					for _, id := range holder.st.IDs() {
						v, _ := holder.st.Value(id)
						t.Error(id, v)
					}
					t.Errorf("state not found for %d=%v", id, val)
					continue
				}
				wantStVal := types.DefaultTypeAdapter.NativeToValue(val)
				if wantStVal.Equal(stVal) != types.True {
					t.Errorf("got %v, wanted %v for id: %d", stVal.Value(), val, id)
				}
			}
		})
	}
}

func BenchmarkResolverCustomQualifier(b *testing.B) {
	reg := newTestRegistry(b)
	attrs := &custAttrFactory{
		AttributeFactory: NewAttributeFactory(containers.DefaultContainer, reg, reg),
	}
	msg := &proto3pb.TestAllTypes_NestedMessage{
		Bb: 123,
	}
	vars, _ := NewActivation(map[string]any{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	fieldType := types.NewObjectType("google.expr.proto3.test.TestAllTypes.NestedMessage")
	qualBB := makeQualifier(b, attrs, fieldType, 2, "bb")
	attr.AddQualifier(qualBB)
	for i := 0; i < b.N; i++ {
		attr.Resolve(vars)
	}
}

type custAttrFactory struct {
	AttributeFactory
}

func (r *custAttrFactory) NewQualifier(objType *types.Type, qualID int64, val any, opt bool) (Qualifier, error) {
	if objType.Kind() == types.StructKind && objType.TypeName() == "google.expr.proto3.test.TestAllTypes.NestedMessage" {
		switch v := val.(type) {
		case string:
			return &nestedMsgQualifier{id: qualID, field: v, opt: opt}, nil
		case types.String:
			return &nestedMsgQualifier{id: qualID, field: string(v), opt: opt}, nil
		}
	}
	return r.AttributeFactory.NewQualifier(objType, qualID, val, opt)
}

type nestedMsgQualifier struct {
	id    int64
	field string
	opt   bool
}

func (q *nestedMsgQualifier) ID() int64 {
	return q.id
}

func (q *nestedMsgQualifier) IsOptional() bool {
	return q.opt
}

func (q *nestedMsgQualifier) Qualify(vars Activation, obj any) (any, error) {
	pb := obj.(*proto3pb.TestAllTypes_NestedMessage)
	return pb.GetBb(), nil
}

func (q *nestedMsgQualifier) QualifyIfPresent(vars Activation, obj any, presenceOnly bool) (any, bool, error) {
	pb := obj.(*proto3pb.TestAllTypes_NestedMessage)
	if pb.GetBb() == 0 {
		return nil, false, nil
	}
	return pb.GetBb(), true, nil
}

func addQualifier(t testing.TB, attr Attribute, qual Qualifier) Attribute {
	t.Helper()
	_, err := attr.AddQualifier(qual)
	if err != nil {
		t.Fatalf("attr.AddQualifier(%v) failed: %v", qual, err)
	}
	return attr
}

func makeQualifier(t testing.TB, attrs AttributeFactory, fieldType *types.Type, qualID int64, val any) Qualifier {
	t.Helper()
	qual, err := attrs.NewQualifier(fieldType, qualID, val, false)
	if err != nil {
		t.Fatalf("attrs.NewQualifier() failed: %v", err)
	}
	return qual
}

func makeOptQualifier(t testing.TB, attrs AttributeFactory, fieldType *types.Type, qualID int64, val any) Qualifier {
	t.Helper()
	qual, err := attrs.NewQualifier(fieldType, qualID, val, true)
	if err != nil {
		t.Fatalf("attrs.NewQualifier() failed: %v", err)
	}
	return qual
}

func findField(t testing.TB, reg ref.TypeRegistry, typeName, field string) *ref.FieldType {
	t.Helper()
	ft, found := reg.FindFieldType(typeName, field)
	if !found {
		t.Fatalf("reg.FindFieldType(%v, %v) failed", typeName, field)
	}
	return ft
}

func testExprTypeToType(t testing.TB, fieldType *exprpb.Type) *types.Type {
	t.Helper()
	ft, err := types.ExprTypeToType(fieldType)
	if err != nil {
		t.Fatalf("types.ExprTypeToType() failed: %v", err)
	}
	return ft
}

func TestConditionalAttributeQualify(t *testing.T) {
	reg, _ := types.NewRegistry()
	cont := containers.DefaultContainer
	fac := NewAttributeFactory(cont, reg, reg)

	truthy := fac.AbsoluteAttribute(1, "a")
	falsy := fac.AbsoluteAttribute(2, "b")
	cond := &conditionalAttribute{
		id:      3,
		expr:    NewConstValue(4, types.True),
		truthy:  truthy,
		falsy:   falsy,
		adapter: reg,
		fac:     fac,
	}

	activation, _ := NewActivation(map[string]any{"a": "key", "b": "other"})
	obj := map[string]any{"key": 100}

	// Test Qualify
	res, err := cond.Qualify(activation, obj)
	if err != nil {
		t.Fatalf("Qualify() failed: %v", err)
	}
	if res != 100 {
		t.Errorf("Qualify() returned %v, wanted 100", res)
	}

	// Test QualifyIfPresent
	res, found, err := cond.QualifyIfPresent(activation, obj, false)
	if err != nil {
		t.Fatalf("QualifyIfPresent() failed: %v", err)
	}
	if !found || res != 100 {
		t.Errorf("QualifyIfPresent() returned (%v, %v), wanted (100, true)", res, found)
	}
}

func TestQualifyIfPresent(t *testing.T) {
	reg := newTestRegistry(t)
	cont := containers.DefaultContainer
	fac := NewAttributeFactory(cont, reg, reg)
	activation, _ := NewActivation(map[string]any{
		"a": "b",
		"c": int64(1),
	})

	tests := []struct {
		name string
		qual Qualifier
		obj  any
		out  any
	}{
		{
			name: "absolute_attribute",
			qual: fac.AbsoluteAttribute(1, "a"),
			obj:  map[string]any{"b": 100},
			out:  100,
		},
		{
			name: "maybe_attribute",
			qual: fac.MaybeAttribute(1, "a"),
			obj:  map[string]any{"b": 100},
			out:  100,
		},
		{
			name: "relative_attribute",
			qual: fac.RelativeAttribute(2, NewConstValue(1, types.String("b"))),
			obj:  map[string]any{"b": 200},
			out:  200,
		},
		{
			name: "string_qualifier",
			qual: makeOptQualifier(t, fac, nil, 1, "b"),
			obj:  map[string]any{"b": 300},
			out:  300,
		},
		{
			name: "int_qualifier",
			qual: makeOptQualifier(t, fac, nil, 1, int64(1)),
			obj:  map[int64]any{int64(1): "value"},
			out:  "value",
		},
		{
			name: "uint_qualifier",
			qual: makeOptQualifier(t, fac, nil, 1, uint64(1)),
			obj:  map[uint64]any{uint64(1): "uvalue"},
			out:  "uvalue",
		},
		{
			name: "bool_qualifier",
			qual: makeOptQualifier(t, fac, nil, 1, true),
			obj:  map[bool]any{true: "bvalue"},
			out:  "bvalue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, found, err := tc.qual.QualifyIfPresent(activation, tc.obj, false)
			if err != nil {
				t.Fatalf("QualifyIfPresent() failed: %v", err)
			}
			if !found || !reflect.DeepEqual(res, tc.out) {
				t.Errorf("QualifyIfPresent() returned (%v, %v), wanted (%v, true)", res, found, tc.out)
			}
		})
	}
}
