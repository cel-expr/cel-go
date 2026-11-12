# Semantic Versioning (SemVer) Extensions

The SemVer extension library provides functions for semantic version (SemVer 2.0.0)
parsing, validation, component extraction, and comparison within CEL expressions.

It mirrors the behavior of the Kubernetes CEL semver library, treating semantic
versions as an opaque type (`semver_type`).

The library is not enabled by default. Configure it with the `Library` option:

```go
import "cel.dev/cel-go/ext/semver"

env, err := cel.NewEnv(semver.Library())
```

Note: This library defines global functions `semver` and `isSemver`. If you are
currently using variables with those names, these functions will likely work as
intended; however, there is some chance for collision.

## Semver Parsing and Validation

The `semver` function converts a string to a semantic version (`Semver`) or results
in an error if the string is not a valid semantic version. Refer to semver.org
documentation for information on accepted patterns.

An optional `normalize` boolean argument (introduced in version 1) can be passed
to enable normalization. Normalization removes any leading `"v"` prefix, adds `0`
minor and patch numbers to versions with only major or major.minor components specified,
and removes any leading `0`s from numeric components.

The `isSemver` function checks whether a string is a valid semantic version
(optionally with normalization in version 1) without throwing an error.

    semver(string) -> Semver
    semver(string, bool) -> Semver // Introduced at version 1
    isSemver(string) -> bool
    isSemver(string, bool) -> bool // Introduced at version 1

Examples:

    semver('1.0.0')
    semver('0.1.0-alpha.1')
    semver('v1.0', true) == semver('1.0.0')
    semver('01.01.01', true) == semver('1.1.1')
    isSemver('1.2.3-beta.1+build.1') == true
    isSemver('v1.0') == false
    isSemver('v1.0', true) == true

## Component Accessors

    <Semver>.major() -> int
    <Semver>.minor() -> int
    <Semver>.patch() -> int

Examples:

    semver('1.2.3').major() == 1
    semver('1.2.3').minor() == 2
    semver('1.2.3').patch() == 3

## Comparison and Conversion

- `isGreaterThan`: Returns true if and only if the receiver is greater than the operand.
- `isLessThan`: Returns true if and only if the receiver is less than the operand.
- `compareTo`: Compares receiver to operand and returns 0 if they are equal, 1 if the receiver is greater, or -1 if the receiver is less than the operand.
- `string`: Formats a `Semver` value as its canonical semantic version string.

Signatures:

    <Semver>.isGreaterThan(Semver) -> bool
    <Semver>.isLessThan(Semver) -> bool
    <Semver>.compareTo(Semver) -> int
    string(Semver) -> string

Examples:

    semver('1.2.3').compareTo(semver('1.2.3')) == 0
    semver('1.2.3').compareTo(semver('2.0.0')) == -1
    semver('1.2.3').compareTo(semver('0.1.2')) == 1
    semver('1.0.0').isLessThan(semver('1.2.3')) == true
    semver('1.2.3').isGreaterThan(semver('1.0.0')) == true
    string(semver('v1.2', true)) == '1.2.0'
