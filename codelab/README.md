### Welcome to the CEL Codelab!
---

Find the codelab instructions [here](https://codelabs.developers.google.com/codelabs/cel-go/#0). It requires some knowledge of GoLang and Protobuf.

If you get stuck, check out the [solutions](https://cel.dev/cel-go/blob/master/codelab/solution/codelab.go).

If you find a bug or want to make an improvement, PRs and issues are welcome. Please follow the [contributing guidelines](https://cel.dev/cel-go/blob/master/CONTRIBUTING.md).

### Timestamp inputs

A variable declared with `cel.TimestampType` accepts both Go's `time.Time` and
`*timestamppb.Timestamp` from `google.golang.org/protobuf/types/known/timestamppb`.
CEL's type adapter converts either representation to the CEL `timestamp` type.
For evaluation bindings, use the representation your application already has:

```go
now := time.Now()
nativeInput := map[string]any{"now": now}
protoInput := map[string]any{"now": timestamppb.New(now)}
```

These bindings represent the same instant, including its nanoseconds. In contrast,
`&timestamppb.Timestamp{Seconds: now.Unix()}` omits the fractional second; use
`timestamppb.New(now)` when you want to preserve it.

When constructing a Go protobuf message such as `AttributeContext_Request`, its
`Time` field still requires a protobuf timestamp. This is a constraint of the Go
message field, not a requirement for CEL evaluation bindings.
