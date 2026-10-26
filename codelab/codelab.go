// Copyright 2020 Google LLC
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

// This file contains code that demonstrates common CEL features.
// This code is intended for use with the CEL Codelab: go/cel-codelab-go
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	rpcpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	exercise1()
	exercise2()
	exercise3()
	exercise4()
	exercise5()
	exercise6()
	exercise7()
	exercise8()
}

// exercise1 evaluates a simple literal expression: "Hello, World!"
//
// Compile, eval, profit!
func exercise1() {
	fmt.Println("=== Exercise 1: Hello World ===")

	fmt.Println()
}

// exercise2 shows how to declare and use variables in expressions.
//
// Given a `request` of type `google.rpc.context.AttributeContext.Request`
// determine whether a specific auth claim is set.
func exercise2() {
	fmt.Println("=== Exercise 2: Variables ===")

	fmt.Println()
}

// exercise3 demonstrates how CEL's commutative logical operators work.
//
// Construct an expression which checks whether the `request.auth.claims.group`
// value is equal to `admin` or the `request.auth.principal` is
// `user:me@acme.co` and the `request.time` is during work hours (9:00 - 17:00)
//
// Evaluate the expression once with a request containing no claims but which
// sets the appropriate principal and occurs at 12:00 hours. Then evaluate the
// request a second time at midnight. Observe the difference in output.
func exercise3() {
	fmt.Println("=== Exercise 3: Logical AND/OR ===")

	fmt.Println()
}

// exercise4 demonstrates how to extend CEL with custom functions.
//
// Declare a `contains` member function on map types that returns a boolean
// indicating whether the map contains the key-value pair.
func exercise4() {
	fmt.Println("=== Exercise 4: Customization ===")

	fmt.Println()
}

// exercise5 covers how to build complex objects as CEL literals.
//
// Given the input `now`, construct a JWT with an expiry of 5 minutes.
func exercise5() {
	fmt.Println("=== Exercise 5: Building JSON ===")

	fmt.Println()
}

// exercise6 describes how to build proto message types within CEL.
//
// Given an input `jwt` and time `now` construct a
// `google.rpc.context.AttributeContext.Request` with the `time` and `auth`
// fields populated according to the go/api-attributes specification.
func exercise6() {
	fmt.Println("=== Exercise 6: Building Protos ===")

	fmt.Println()
}

// exercise7 introduces macros for dealing with repeated fields and maps.
//
// Determine whether the `jwt.extra_claims` has at least one key that starts
// with the `group` prefix, and ensure that all group-like keys have list
// values containing only strings that end with '@acme.co`.
func exercise7() {
	fmt.Println("=== Exercise 7: Macros ===")

	fmt.Println()
}

// exercise8 covers some useful features of CEL-Go which can be used to
// improve performance and better understand evaluation behavior.
//
// Turn on the optimization, exhaustive eval, and state tracking
// `cel.ProgramOption` flags to see the impact on evaluation behavior.
//
// Also, turn on the homogeneous aggregate literals flag to disable
// heterogeneous list and map literals.
func exercise8() {
	fmt.Println("=== Exercise 8: Tuning ===")

	fmt.Println()
}

// Functions to assist with CEL execution.

// compile will parse and check an expression `expr` against a given
// environment `env` and determine whether the resulting type of the expression
// matches the `exprType` provided as input.
func compile(env *cel.Env, expr string, celType *cel.Type) *cel.Ast {
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	if !reflect.DeepEqual(ast.OutputType(), celType) {
		glog.Exitf(
			"Got %v, wanted %v result type", ast.OutputType(), celType)
	}
	fmt.Printf("%s\n\n", strings.ReplaceAll(expr, "\t", " "))
	return ast
}

// eval will evaluate a given program `prg` against a set of variables `vars`
// and return the output, eval details (optional), or error that results from
// evaluation.
func eval(prg cel.Program,
	vars any) (out ref.Val, det *cel.EvalDetails, err error) {
	varMap, isMap := vars.(map[string]any)
	fmt.Println("------ input ------")
	if !isMap {
		fmt.Printf("(%T)\n", vars)
	} else {
		for k, v := range varMap {
			switch val := v.(type) {
			case proto.Message:
				bytes, err := prototext.Marshal(val)
				if err != nil {
					glog.Exitf("failed to marshal proto to text: %v", val)
				}
				fmt.Printf("%s = %s", k, string(bytes))
			case map[string]any:
				b, _ := json.MarshalIndent(v, "", "  ")
				fmt.Printf("%s = %v\n", k, string(b))
			case uint64:
				fmt.Printf("%s = %vu\n", k, v)
			default:
				fmt.Printf("%s = %v\n", k, v)
			}
		}
	}
	fmt.Println()
	out, det, err = prg.Eval(vars)
	report(out, det, err)
	fmt.Println()
	return
}

// report prints out the result of evaluation in human-friendly terms.
func report(result ref.Val, details *cel.EvalDetails, err error) {
	fmt.Println("------ result ------")
	if err != nil {
		fmt.Printf("error: %s\n", err)
	} else {
		fmt.Printf("value: %v (%T)\n", result, result)
	}
	if details != nil {
		fmt.Printf("\n------ eval states ------\n")
		state := details.State()
		stateIDs := state.IDs()
		ids := make([]int, len(stateIDs), len(stateIDs))
		for i, id := range stateIDs {
			ids[i] = int(id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			v, found := state.Value(int64(id))
			if !found {
				continue
			}
			fmt.Printf("%d: %v (%T)\n", id, v, v)
		}
	}
}

// mapContainsKeyValue implements the custom function:
//
//	`map.contains(key, value) bool`.
func mapContainsKeyValue(args ...ref.Val) ref.Val {
	// The declaration of the function ensures that only arguments which match
	// the mapContainsKey signature will be provided to the function.
	m := args[0].(traits.Mapper)

	// CEL has many interfaces for dealing with different type abstractions.
	// The traits.Mapper interface unifies field presence testing on proto
	// messages and maps.
	key := args[1]
	v, found := m.Find(key)

	// If not found and the value was non-nil, the value is an error per the
	// `Find` contract. Propagate it accordingly.
	if !found {
		if v != nil {
			return types.ValOrErr(v, "unsupported key type")
		}
		// Return CEL False if the key was not found.
		return types.False
	}
	// Otherwise whether the value at the key equals the value provided.
	return v.Equal(args[2])
}

// Functions for constructing CEL inputs.

// auth constructs a `google.rpc.context.AttributeContext.Auth` message.
func auth(user string, claims map[string]string) *rpcpb.AttributeContext_Auth {
	claimFields := make(map[string]*structpb.Value)
	for k, v := range claims {
		claimFields[k] = structpb.NewStringValue(v)
	}
	return &rpcpb.AttributeContext_Auth{
		Principal: user,
		Claims:    &structpb.Struct{Fields: claimFields},
	}
}

// request constructs a `google.rpc.context.AttributeContext.Request` message.
func request(auth *rpcpb.AttributeContext_Auth, t time.Time) map[string]any {
	req := &rpcpb.AttributeContext_Request{
		Auth: auth,
		Time: &tpb.Timestamp{Seconds: t.Unix()},
	}
	return map[string]any{"request": req}
}

// valueToJSON converts the CEL type to a protobuf JSON representation and
// marshals the result to a string.
func valueToJSON(val ref.Val) string {
	v, err := val.ConvertToNative(reflect.TypeFor[*structpb.Value]())
	if err != nil {
		glog.Exit(err)
	}
	marshaller := protojson.MarshalOptions{Indent: "    "}
	bytes, err := marshaller.Marshal(v.(proto.Message))
	if err != nil {
		glog.Exit(err)
	}
	return string(bytes)
}

var (
	emptyClaims = make(map[string]string)
)
