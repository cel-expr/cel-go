# CEL-Go Examples

This directory contains executable Go examples showcasing the CEL-Go APIs and language features. All examples correspond to topics on [CEL by Example](https://celbyexample.com) and are implemented as Go `Example...()` functions runnable via `go test` and viewable via `godoc`.

---

## 1. Quickstart & Compilation (`cel.Compile`)

Compile and evaluate expressions using `cel.Compile()`:

```go
package main

import (
	"fmt"
	"log"

	"cel.dev/cel-go/cel"
)

func main() {
	prg, err := cel.Compile(`"Hello world! I'm " + name + "."`,
		cel.Variable("name", cel.StringType),
	)
	if err != nil {
		log.Fatalln(err)
	}
	out, _, err := prg.Eval(map[string]any{
		"name": "CEL",
	})
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(out)
	// Output: Hello world! I'm CEL.
}
```

For custom options and extensions:

```go
prg, err := cel.Compile(`"%s! I'm %s.".format([greeting, name])`,
	ext.Strings(),
	cel.Variable("greeting", cel.StringType),
	cel.Variable("name", cel.StringType),
)
out, _, err := prg.Eval(map[string]any{
	"greeting": "Hello world",
	"name":     "CEL",
})
```

[Source code: example_cel_compile_test.go](example_cel_compile_test.go) | [Source code: example_cel_context_eval_test.go](example_cel_context_eval_test.go)

---

## 2. Strings & Numbers

Covers string functions (`endsWith`, `contains`, `startsWith`, `matches`, `size`), string concatenation (`+`), numeric comparisons, and `ext.Strings()` extensions:

```go
prg, _ := cel.Compile(`"hello world".replace("world", "CEL")`, ext.Strings())
out, _, _ := prg.Eval(cel.NoVars())
fmt.Println(out) // hello CEL
```

[Source code: example_cel_strings_and_numbers_test.go](example_cel_strings_and_numbers_test.go)

---

## 3. Collections (Lists & Maps)

Membership (`in`), size (`size()`), indexing (`[0]`), quantifier macros (`exists()`, `all()`), and filtering (`filter()`):

```go
prg, _ := cel.Compile(`tags.exists(t, t.startsWith("ex"))`, cel.Variable("tags", cel.DynType))
out, _, _ := prg.Eval(map[string]any{
	"tags": []string{"express", "gift-wrapped"},
})
fmt.Println(out) // true
```

[Source code: example_cel_collections_test.go](example_cel_collections_test.go)

---

## 4. Timestamps & Durations

Time component extraction (`getFullYear()`, `getHours()`), timezones, timestamp arithmetic, and epoch conversions:

```go
prg, _ := cel.Compile(`timestamp("2024-01-15T10:30:45Z") + duration("1h")`)
out, _, _ := prg.Eval(cel.NoVars())
fmt.Println(out) // 2024-01-15 11:30:45 +0000 UTC
```

[Source code: example_cel_time_test.go](example_cel_time_test.go)

---

## 5. Logic & Conditions

Logical operators (`!`, `&&`, `||`), short-circuiting, and conditional (ternary) operators:

```go
prg, _ := cel.Compile(`age >= 18 ? "adult" : "minor"`, cel.Variable("age", cel.IntType))
out, _, _ := prg.Eval(map[string]any{"age": 20})
fmt.Println(out) // adult
```

[Source code: example_cel_logic_and_conditions_test.go](example_cel_logic_and_conditions_test.go)

---

## 6. Transforming Data

Building maps, list transformation with `map()`, and filtering before transformation:

```go
prg, _ := cel.Compile(`nums.map(x, x % 2 == 0, x * 10)`, cel.Variable("nums", cel.ListType(cel.IntType)))
out, _, _ := prg.Eval(map[string]any{
	"nums": []int64{1, 2, 3, 4, 5},
})
fmt.Println(out) // [20, 40]
```

[Source code: example_cel_transforming_data_test.go](example_cel_transforming_data_test.go)

---

## 7. Type Conversions & Introspection

Type casting (`int()`, `uint()`, `double()`, `string()`, `bytes()`, `dyn()`) and type inspection (`type(x)`):

```go
prg, _ := cel.Compile(`type(42) == int`)
out, _, _ := prg.Eval(cel.NoVars())
fmt.Println(out) // true
```

[Source code: example_cel_type_conversions_test.go](example_cel_type_conversions_test.go)

---

## 8. Protocol Buffers & Well-Known Types

Protobuf message construction, enums, field presence `has()`, default values, wrappers (`StringValue`, `BoolValue`), and `google.protobuf.Struct`:

```go
prg, _ := cel.Compile(`has(user.single_string)`,
	cel.Types(&proto3pb.TestAllTypes{}),
	cel.Variable("user", cel.ObjectType("google.expr.proto3.test.TestAllTypes")),
)
out, _, _ := prg.Eval(map[string]any{
	"user": &proto3pb.TestAllTypes{SingleString: "Alice"},
})
fmt.Println(out) // true
```

[Source code: example_cel_protocol_buffers_test.go](example_cel_protocol_buffers_test.go)

---

## 9. Operators

Arithmetic, ordering, equality, and `null` handling:

```go
prg, _ := cel.Compile(`(2 + 3) * 4`)
out, _, _ := prg.Eval(cel.NoVars())
fmt.Println(out) // 20
```

[Source code: example_cel_operators_test.go](example_cel_operators_test.go)

---

## 10. Custom Functions

Extend CEL with global functions (`cel.Overload`) or member functions (`cel.MemberOverload`):

```go
prg, _ := cel.Compile(`shake_hands(i, you)`,
	cel.Variable("i", cel.StringType),
	cel.Variable("you", cel.StringType),
	cel.Function("shake_hands",
		cel.Overload("shake_hands_string_string",
			[]*cel.Type{cel.StringType, cel.StringType},
			cel.StringType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				return types.String(fmt.Sprintf("%s and %s are shaking hands.", lhs, rhs))
			}),
		),
	),
)
out, _, _ := prg.Eval(map[string]any{"i": "CEL", "you": "world"})
fmt.Println(out) // CEL and world are shaking hands.
```

[Source code: example_cel_custom_functions_test.go](example_cel_custom_functions_test.go)

---

## 11. Custom Macros

Define AST transformation macros using `parser.NewGlobalMacro` or `parser.NewReceiverMacro`:

```go
joinMacro := parser.NewGlobalMacro("join_str", 2, func(eh parser.ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
    space := eh.NewLiteral(types.String(" "))
    first := eh.NewCall(operators.Add, args[0], space)
    return eh.NewCall(operators.Add, first, args[1]), nil
})
prg, _ := cel.Compile(`join_str("Hello", "World")`, cel.Macros(joinMacro))
out, _, _ := prg.Eval(cel.NoVars())
fmt.Println(out) // Hello World
```

[Source code: example_cel_custom_macros_test.go](example_cel_custom_macros_test.go)

---

## 12. Execution Cost Analysis

Static cost estimation with `env.EstimateCost()` and dynamic limit enforcement with `cel.CostTracking()` & `cel.CostLimit()`:

```go
env, _ := cel.NewEnv(cel.Variable("items", cel.ListType(cel.IntType)))
ast, _ := env.Compile(`items.map(x, x * x).size()`)
cost, _ := env.EstimateCost(ast, costEstimator{hints: map[string]uint64{"items": 5, "items.@items": 10}})
prg, _ := env.Program(ast, cel.CostTracking(runtimeEstimator{}), cel.CostLimit(1000))
out, details, _ := prg.Eval(map[string]any{"items": []int64{1, 2, 3}})
fmt.Println(out, *details.ActualCost())
```

[Source code: example_cel_execution_cost_test.go](example_cel_execution_cost_test.go)

---

## 13. Advanced Topics: Error Handling & Name Resolution

Handling runtime errors and variable shadowing rules in macros:

[Source code: example_cel_advanced_test.go](example_cel_advanced_test.go)

---

## 14. Go Native Structs (`ext.NativeTypes`)

Export Go structs into CEL and map struct field names using `json`, `yaml`, or `cel` struct tags:

```go
type User struct {
	Name  string   `json:"name"`
	Age   int      `json:"age"`
	Roles []string `json:"roles"`
}

prg, _ := cel.Compile(`user.name == "Alice" && user.age >= 18`,
	cel.Variable("user", cel.ObjectType("examples.User")),
	ext.NativeTypes(reflect.TypeOf(User{}), ext.ParseStructTag("json")),
)
out, _, _ := prg.Eval(map[string]any{"user": User{Name: "Alice", Age: 30, Roles: []string{"admin"}}})
fmt.Println(out) // true
```

[Source code: example_cel_native_structs_test.go](example_cel_native_structs_test.go)

---

## 15. Evaluation Explainer (`cel.ExplainEval`)

Trace comprehension iterations with `cel.OptTraceIterations` (and optionally `cel.OptExhaustiveEval`) to explain violations, inspect evaluation steps, and step through evaluations with `Debugger`:

```go
prg, _ := env.Program(ast, cel.EvalOptions(cel.OptTraceIterations, cel.OptExhaustiveEval))
_, details, _ := prg.Eval(input)
exp, _ := cel.ExplainEval(ast, details)
fmt.Print(exp.String())
```

[Source code: example_cel_explain_test.go](example_cel_explain_test.go)

---

## Running the Examples

Run all examples locally with `go test`:

```bash
go test -v ./examples/...
```

For interactive online examples, visit [CEL by Example](https://celbyexample.com).
