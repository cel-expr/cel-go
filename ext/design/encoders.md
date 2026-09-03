# CEL Encoders Extension Library Design

## 1. Overview

The `encoders` extension library provides standard encoding, decoding, and parsing functions for Common Expression Language (CEL). It allows expressions to safely interact with serialized data representations like Base64 and JSON without compromising execution safety, determinism, or resource boundaries.

The library defines functions under two primary namespaces:
- `base64`: For standard Base64 binary-to-text encoding and decoding.
- `json`: For serializing CEL values to JSON and parsing JSON payloads into dynamic or strongly-typed CEL representations.

---

## 2. Design Decisions & Precedents

### 2.1. Terminology: `encode` vs. `decode` vs. `parse`

Across the CEL standard library and extension ecosystem, function naming follows distinct semantic conventions:

| Operation | Canonical Signature Pattern | Precedent | Semantic Meaning | Error Handling |
| :--- | :--- | :--- | :--- | :--- |
| **`encode`** | `<Type>.encode(val) -> string` | `base64.encode`, `json.encode` | Converts in-memory structured data / bytes / CEL values into a serialized text format. | Produces valid serialized string or returns a CEL error on serialization failure. |
| **`decode`** | `<Type>.decode(string) -> bytes` | `base64.decode` | Inverts a fixed-shape binary-to-text encoding where target shape is well-known (typically `bytes`). | Returns CEL evaluation error on malformed inputs. |
| **`parse`** | `<Type>.parse(string[, type(T)]) -> optional_type(T)` | `json.parse`, `jwt.parse` | Deserializes text containing structured schemas or dynamic payloads into structured CEL values. | Returns `optional.none()` on malformed inputs or schema mismatches, avoiding hard evaluation errors on untrusted input. |

#### Why `json.parse` returns `optional_type(T)` instead of throwing errors:
In CEL evaluation pipelines (e.g., authorization, admission control, policy enforcement), input JSON payloads are frequently untrusted. Raising runtime evaluation errors would short-circuit entire policy evaluations and abort rule checking. Returning `optional_type(T)` empowers authors to use CEL's `optional` macros (`opt.hasValue()`, `opt.orValue(default)`, `opt.optMap(...)`) to handle absent or malformed data gracefully.

---

## 3. Function Signatures & Behavior

### 3.1. Base64 Functions

```text
base64.encode(bytes) -> string
base64.decode(string) -> bytes
```

- **`base64.encode`**: Converts `bytes` into standard Base64 encoded string with standard padding (`=`).
- **`base64.decode`**: Decodes Base64 string into `bytes`. Supports both standard padding and unpadded / raw Base64 strings.

### 3.2. JSON Functions

```text
json.encode(dyn) -> string
json.parse(string) -> optional_type(dyn)
json.parse(string, type(T)) -> optional_type(T)
```

#### `json.encode(val)`
- Converts any CEL value (primitives, lists, maps, protobuf messages, native structs) into a compact JSON string.
- Uses a single-pass `protojson.Marshal` on `types.JSONValueType` (`structpb.Value`), eliminating double-marshaling overhead.
- **Key Ordering**: Maps and structs in Go iterate non-deterministically; thus JSON object key ordering is non-deterministic. Equality checks against JSON outputs in tests and policies should parse back to objects or verify membership across valid permutations.

#### `json.parse(string)`
- Parses any valid JSON payload into a dynamic CEL representation (`optional_type(dyn)`):
  - JSON objects $\rightarrow$ CEL `map(string, dyn)`
  - JSON arrays $\rightarrow$ CEL `list(dyn)`
  - JSON numbers $\rightarrow$ CEL `int` (if integer) or `double` (if float/exponent), using `json.Number` for precision preservation.
  - JSON booleans $\rightarrow$ CEL `bool`
  - JSON strings $\rightarrow$ CEL `string`
  - JSON `null` $\rightarrow$ CEL `null_type`
- Returns `optional.none()` if input is not valid JSON or has unexpected trailing data.

#### `json.parse(string, type(T))`
- Strongly-typed JSON parsing into target type `T`:
  - **Primitives**: `int`, `uint`, `double`, `bool`, `string`, `bytes` (decodes base64 string), `null_type`, `timestamp` (RFC3339 string), `duration` (e.g. `"5s"`).
  - **Collections**: `list(T)` and `map(string, T)`.
  - **Protobuf Types**: `google.protobuf.Struct`, `google.protobuf.Value`, `google.protobuf.ListValue`, and arbitrary registered `proto.Message` types.
  - **Go Native Structs**: Instantiated via `types.Provider.NewValue(typeName, ...)` and Go reflection, respecting struct tags:
    - `json:"<custom_name>"`: maps JSON field `<custom_name>` to the struct field.
    - `json:"-"`: ignores and omits the field.
    - `json:",omitempty"`: omits zero-value fields during serialization.
- Returns `optional.none()` if the JSON value does not match the expected target schema or type.

---

## 4. Security & Resource Bounds

### 4.1. Payload Size Limit
To prevent memory exhaustion attacks (e.g., billion-laughs-style JSON bombs, deeply nested objects, or multi-gigabyte payloads), `json.parse` enforces a strict 10MB maximum input size:
```go
const maxJSONSize = 10 * 1024 * 1024 // 10MB
```
Inputs exceeding this limit immediately return a CEL runtime error.

### 4.2. Cost Modeling
JSON parsing and serialization complexity cannot be statically bounded without inspecting runtime payloads and target schema depths. Therefore:
- **Cost Estimation**: Overload cost estimators return `checker.UnknownCostEstimate()` (`CostEstimate{Min: 0, Max: math.MaxUint64}`).
- **Cost Tracking**: Runtime cost trackers return `math.MaxUint64`.

---

### 5.1. Instantiation via `Provider.NewValue`

`json.parse` utilizes `types.Provider.NewValue(typeName, map[string]ref.Val{})` to instantiate target types:
1. **Protobuf Messages**: `NewValue` returns a proto value whose underlying `proto.Message` is cloned and populated using `protojson.Unmarshal`.
2. **Native Go Objects**: `NewValue` returns a native struct value whose `reflect.Type` is used to instantiate a pointer (`reflect.New(rt)`) and populated via standard `json.Unmarshal`.
3. **No Private Registry Exposure**: Avoids leaking internal registry structures or requiring custom reflection getters on `Registry`.

### 5.2. Separation of Concerns & Format Extensibility (YAML, XML)

To maintain a clean separation of concerns:
- **`common/types/native.go` & `ext/native.go`**: Contain all Go struct reflection, struct tag inspection (`json:"..."`, `cel:"..."`, `omitempty`, `json:"-"`), anonymous embedded struct promotion, and zero-value omission rules. `nativeObj.ConvertToNative(types.JSONValueType)` / `nativeObj.ConvertToNative(types.JSONStructType)` transforms native objects into standard structured representations.
- **`ext/encoders.go`**: Acts as a format-level encoder/decoder. It performs payload size verification, invokes `val.ConvertToNative(types.JSONValueType)`, and calls the format-specific marshal/unmarshal engine (`protojson`).
- **Future Format Support (YAML, XML)**: Adding support for formats such as YAML or XML will follow the exact same architecture: format encoders in `ext/` remain lightweight wrappers over `ConvertToNative` / `NewValue`, while native type reflection and tag mappings reside centrally in `common/types/native.go` and `ext/native.go`.

---

## 6. Test Matrix & Edge Cases

### 6.1. Positive Test Cases

| Scenario | Input Expression | Expected Output |
| :--- | :--- | :--- |
| Dynamic Primitive String | `json.parse('"hello"')` | `optional.of('hello')` |
| Dynamic Negative Int | `json.parse('-42')` | `optional.of(-42)` |
| Dynamic Float & Scientific | `json.parse('1.25e2')` | `optional.of(125.0)` |
| Dynamic Boolean & Null | `json.parse('true')`, `json.parse('null')` | `optional.of(true)`, `optional.of(null)` |
| Dynamic Nested Structure | `json.parse('{"items": [1, true, "a"]}')` | `optional.of({'items': [1, true, 'a']})` |
| Typed Unsigned Int | `json.parse('18446744073709551615', uint)` | `optional.of(18446744073709551615u)` |
| Typed Base64 Bytes | `json.parse('"aGVsbG8="', bytes)` | `optional.of(b'hello')` |
| Typed RFC3339 Timestamp | `json.parse('"2023-01-01T00:00:00Z"', type(timestamp('...')))` | `optional.of(timestamp('2023-01-01T00:00:00Z'))` |
| Typed Duration | `json.parse('"5s"', type(duration('5s')))` | `optional.of(duration('5s'))` |
| Typed Generic List | `json.parse('[1, 2, 3]', type([1]))` | `optional.of([1, 2, 3])` |
| Typed Generic Map | `json.parse('{"a": 1}', type({'': 1}))` | `optional.of({'a': 1})` |
| Native Struct with Tags | `json.parse('{"user_name":"Alice","age":25}', ext.testNativeUser)` | `optional.of(testNativeUser{Username: 'Alice', Age: 25})` |
| Roundtrip Identity | `json.parse(json.encode({'a': 1, 'b': 2}))` | `optional.of({'a': 1, 'b': 2})` |

### 6.2. Edge Cases (Formatting & Syntax)

| Scenario | Input Expression | Expected Output | Notes |
| :--- | :--- | :--- | :--- |
| Leading / Trailing Whitespace | `json.parse('  \t\n {"a": 1} \r\n ')` | `optional.of({'a': 1})` | Strips outer whitespace properly. |
| Unicode Escape Sequences | `json.parse('"\\u003chello\\u003e"')` | `optional.of('<hello>')` | Decodes hex escapes. |
| Multibyte Emojis / UTF-8 | `json.parse('"🚀"')` | `optional.of('🚀')` | Valid UTF-8 preserved. |
| Double Coercion from Integer | `json.parse('42', double)` | `optional.of(42.0)` | Integer JSON literal valid for double. |
| Empty Collections | `json.parse('{}')`, `json.parse('[]')` | `optional.of({})`, `optional.of([])` | Empty structures preserved. |
| Raw vs Padded Base64 | `base64.decode('aGVsbG8=')`, `base64.decode('aGVsbG8')` | `b'hello'` | Supports both padding modes. |
| Non-deterministic Key Order | `json.encode({'a': 1, 'b': 2})` | `'{"a":1, "b":2}'` or `'{"b":2, "a":1}'` | Validated using `in [...]` or roundtrip parse. |

### 6.3. Negative Test Cases

| Scenario | Input Expression | Expected Output | Notes |
| :--- | :--- | :--- | :--- |
| Malformed JSON: Unclosed Brackets | `json.parse('{')`, `json.parse('[1, 2')` | `optional.none()` | Syntax error returns none. |
| Malformed JSON: Unquoted Key | `json.parse('{key: "value"}')` | `optional.none()` | Invalid JSON syntax. |
| Malformed JSON: Trailing Comma | `json.parse('[1, 2,]')`, `json.parse('{"a": 1,}')` | `optional.none()` | Trailing comma is invalid JSON. |
| Multiple Root Values | `json.parse('{"a": 1} {"b": 2}')` | `optional.none()` | Unexpected trailing token. |
| Trailing Junk Tokens | `json.parse('123 abc')` | `optional.none()` | Trailing garbage returns none. |
| Empty String Input | `json.parse('')` | `optional.none()` | EOF returns none. |
| Type Mismatch: String to Int | `json.parse('"123"', int)` | `optional.none()` | String is not a number. |
| Type Mismatch: Negative to Uint | `json.parse('-1', uint)` | `optional.none()` | Negative not allowable for uint. |
| Type Mismatch: Float to Int | `json.parse('1.5', int)` | `optional.none()` | Fractional number invalid for int. |
| Type Mismatch: Number to Bool | `json.parse('1', bool)` | `optional.none()` | Number is not boolean. |
| Type Mismatch: Object to List | `json.parse('{"a": 1}', type([1]))` | `optional.none()` | Map cannot parse into list. |
| Type Mismatch: Null to Non-Null | `json.parse('null', int)`, `json.parse('null', string)` | `optional.none()` | Null only valid for null_type / dyn. |
| Invalid Base64 in Bytes | `json.parse('"not valid base64 %%%"', bytes)` | `optional.none()` | Corrupt base64 string. |
| Invalid RFC3339 in Timestamp | `json.parse('"invalid-time"', type(timestamp(...)))` | `optional.none()` | Invalid timestamp format. |
| Invalid Duration Unit | `json.parse('"10x"', type(duration('5s')))` | `optional.none()` | Invalid duration format. |
| Size Limit Exceeded (> 10MB) | `json.parse(largeJsonString)` | CEL Runtime Error | String size exceeds 10MB limit. |

---

## 7. Performance Benchmarks & Roundtrip Analysis

### 7.1. Conversion Benchmark Metrics (Apple Silicon M1 Pro)

| Benchmark Scenario | Execution Time (`ns/op`) | Memory Allocated (`B/op`) | Allocs (`allocs/op`) |
| :--- | :--- | :--- | :--- |
| `BenchmarkBase64/EncodeShort` | 65.5 ns | 32 B | 2 |
| `BenchmarkBase64/DecodeShort` | 73.9 ns | 40 B | 2 |
| `BenchmarkJSONEncode/ScalarInt` | 46.6 ns | 16 B | 1 |
| `BenchmarkJSONEncode/ScalarString` | 139.4 ns | 32 B | 2 |
| `BenchmarkJSONEncode/ListSmall` | 316.8 ns | 272 B | 10 |
| `BenchmarkJSONEncode/MapSmall` | 1,102 ns | 841 B | 24 |
| `BenchmarkJSONEncode/MapNested` | 1,563 ns | 1,682 B | 37 |
| `BenchmarkJSONEncode/NativeStruct` | 665.3 ns | 568 B | 9 |
| `BenchmarkJSONEncode/ProtoMessage` | 1,317 ns | 1,441 B | 26 |
| `BenchmarkJSONParse/DynamicScalar` | 88.1 ns | 32 B | 2 |
| `BenchmarkJSONParse/DynamicString` | 73.4 ns | 48 B | 3 |
| `BenchmarkJSONParse/DynamicMap` | 1,335 ns | 1,156 B | 21 |
| `BenchmarkJSONParse/DynamicNested` | 1,846 ns | 1,629 B | 31 |
| `BenchmarkJSONParse/TypedInt` | 77.9 ns | 16 B | 1 |
| `BenchmarkJSONParse/TypedString` | 95.8 ns | 48 B | 3 |
| `BenchmarkJSONParse/TypedNativeStruct` | 705.1 ns | 619 B | 8 |
| `BenchmarkJSONParse/TypedProto` | 1,092 ns | 928 B | 18 |
| `BenchmarkJSONParse/TypedMap` | 1,240 ns | 1,397 B | 21 |
| `BenchmarkJSONParse/TypedList` | 1,393 ns | 1,089 B | 28 |

### 7.2. Edge-Case Roundtrip Behavior

1. **64-bit Integers & Safe Float Range**:
   - Integers in $[-2^{53}+1, 2^{53}-1]$ encode as JSON numbers (`42`, `9007199254740991`) and roundtrip directly via `json.parse(json.encode(n)) == optional.of(n)`.
   - Integers outside this range encode as JSON decimal strings (`"9223372036854775807"`) to prevent IEEE 754 precision loss in downstream JSON consumers. When parsed dynamically, they return a CEL string which can be coerced with `int(...)`. When parsed with explicit schema `json.parse(str, int)`, numbers in the JSON number representation are strictly required to reject raw string mismatches like `json.parse('"123"', int) == optional.none()`.
2. **Binary Data**:
   - `bytes` (`b'...'`) serializes to standard Base64 string and roundtrips faithfully using `json.parse(json.encode(b), bytes) == optional.of(b)`.
3. **Escapes & Unicode**:
   - Quotes (`"hello\"world"`), control characters (`\n`, `\t`, `\r`), and UTF-8 symbols (`🚀`, `\u003c\u003e`) maintain identity across roundtrips.
