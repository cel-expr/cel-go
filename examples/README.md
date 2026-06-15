# Examples

## Simple example using builtin operators

Evaluate expression `"Hello world! I'm " + name + "."` with `CEL` passed as
name.

```go
import "github.com/google/cel-go/cel"

    env, err := cel.NewEnv(cel.Variable("name", cel.StringType))
    // Check err for environment setup errors.
    if err != nil {
        log.Fatalln(err)
    }
    ast, iss := env.Compile(`"Hello world! I'm " + name + "."`)
    // Check iss for compilation errors.
    if iss.Err() != nil {
        log.Fatalln(iss.Err())
    }
    prg, err := env.Program(ast)
    out, _, err := prg.Eval(map[string]any{
        "name":   "CEL",
    })
    fmt.Println(out)
    // Output:Hello world! I'm CEL.
```

[Source code](simple_test.go)

## Custom function on string type

Evaluate expression `i.greet(you)` with:

```
    i       -> CEL
    you     -> world
    greet   -> "Hello %s! Nice to meet you, I'm %s."
```

First we need to declare two string variables and `greet` function.
`Function` must be used if we want to declare an extension to CEL. The
`MemberOverload` declares an overload id, a list of arguments where the
first element the `argTypes` slice is the target type of the member
function. The remaining argument types are the signature of the member
method.

```go
    env, _ := cel.NewEnv(
        cel.Variable("i", cel.StringType),
        cel.Variable("you", cel.StringType),
        cel.Function("greet",
            cel.MemberOverload("string_greet_string",
                []*cel.Type{cel.StringType, cel.StringType},
                cel.StringType,
                cel.BinaryBinding(func (lhs, rhs ref.Val) ref.Val {
                    return types.String(
                        fmt.Sprintf("Hello %s! Nice to meet you, I'm %s.\n", rhs, lhs))
                    },
                ),
            ),
        ),
    )
    prg, _ := env.Program(c)
    out, _, _ := prg.Eval(map[string]any{
        "i": "CEL",
        "you": "world",
    })
    fmt.Println(out)
    // Output:Hello world! Nice to meet you, I'm CEL.
```
[Source code](custom_instance_function_test.go)

## Define custom global function

Evaluate expression `shake_hands(i,you)` with:

```
    i           -> CEL
    you         -> world
    shake_hands -> "%s and %s are shaking hands."
```

In order to declare global function we need to use `Overload` instead
of `MemberOverload` in the `Function` option:

```go
    env, _ := cel.NewEnv(
        cel.Variable("i", cel.StringType),
        cel.Variable("you", cel.StringType),
        cel.Function("shake_hands",
            cel.Overload("shake_hands_string_string",
                []*cel.Type{cel.StringType, cel.StringType},
                cel.StringType,
                cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
                    return types.String(
                        fmt.Sprintf("%s and %s are shaking hands.\n", lhs, rhs))
                    },
                ),
            ),
        ),
    )
    prg, _ := env.Program(c)
    out, _, _ := prg.Eval(map[string]any{
        "i": "CEL",
        "you": "world",
    })
    fmt.Println(out)
    // Output:CEL and world are shaking hands.
```

[Source code](custom_global_function_test.go)

For more examples of how to use CEL, see
[cel_test.go](https://github.com/google/cel-go/tree/master/cel/cel_test.go).
