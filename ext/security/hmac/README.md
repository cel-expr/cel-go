# HMAC Extensions

HMAC extensions are a set of functions and constants for computing and
verifying Hash-based Message Authentication Codes (HMAC) within CEL
expressions. They are commonly used to authenticate signed webhook payloads
and other messages carrying a shared-secret signature.

The library is not enabled by default. Configure it with the `Library` option:

    import hmaclib "cel.dev/cel-go/ext/security/hmac"

    env, err := cel.NewEnv(hmaclib.Library())

## HMAC

Note, all functions and constants use the 'hmac' namespace. If you are
currently using a variable named 'hmac', the functions will likely work as
intended; however, there is some chance for collision.

Every function takes the hash algorithm as a string. Only algorithms
registered with the library may be used; see [Algorithms](#algorithms) below.
Algorithm names are matched case-insensitively, and `-` and `/` are treated as
`_`, so `'SHA-512/256'`, `'sha512_256'`, and `hmac.SHA512_256` all name the
same algorithm.

Functions which accept `string` inputs operate on the UTF-8 encoding of the
string, so the `string` and `bytes` overloads agree for equivalent inputs:

    hmac.digest('payload', hmac.SHA256) == hmac.digest(b'payload', hmac.SHA256)

Note that a `string` secret is used as its literal UTF-8 bytes. If a secret is
distributed in an encoded form, decode it before use, e.g.
`hmac.compute(msg, base64.decode(secret), hmac.SHA256)`.

### Hmac.Compute

Computes the HMAC of a message under a secret key, and returns the raw
authentication code as bytes.

This function will return an error if the algorithm is not registered with the
library.

    hmac.compute(message: bytes, secret: bytes, algorithm: string) -> bytes
    hmac.compute(message: string, secret: string, algorithm: string) -> bytes

Examples:

    "%x".format([hmac.compute('payload', 'key', hmac.SHA256)])
      // return '5d98b45c90a207fa998ce639fea6f02ecc8cc3f36fef81d694fb856b4d0a28ca'

    base64.encode(hmac.compute(b'payload', b'key', hmac.SHA256))
      // return 'XZi0XJCiB/qZjOY5/qbwLsyMw/Nv74HWlPuFa00KKMo='

    hmac.compute('payload', 'key', 'MD5') // Runtime Error unsupported HMAC hash algorithm

### Hmac.Digest

Computes the unkeyed hash digest of a message and returns it as bytes. This is
the plain hash of the message, not an authentication code; use `hmac.compute`
when a secret is involved. It is useful for the payload digests referenced by
signing schemes, such as an AWS SigV4 content hash or a `Digest` header.

This function will return an error if the algorithm is not registered with the
library.

    hmac.digest(message: bytes, algorithm: string) -> bytes
    hmac.digest(message: string, algorithm: string) -> bytes

Examples:

    "%x".format([hmac.digest('payload', hmac.SHA256)])
      // return '239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5'

    base64.encode(hmac.digest(b'payload', hmac.SHA256))
      // return 'I59Z7VXnN8dxR89VrQwbAwttfudIp0JpUvm4UtWpNeU='

    hmac.digest('payload', 'MD5') // Runtime Error unsupported HMAC hash algorithm

### Hmac.Equal

Compares two byte sequences in constant time, returning true when they are
equal. Use this rather than `==` when comparing authentication codes, so that
the comparison does not leak information about the expected value through its
running time. Inputs of differing lengths compare as unequal.

    hmac.equal(lhs: bytes, rhs: bytes) -> bool

Examples:

    hmac.equal(b'abc', b'abc') // return true
    hmac.equal(b'abc', b'abd') // return false
    hmac.equal(b'abc', b'ab')  // return false

    hmac.equal(hmac.compute(msg, secret, hmac.SHA256), base64.decode(signature))

### Hmac.Verify

Computes the HMAC of a message under a secret key and reports whether it
matches the supplied signature, comparing in constant time.

    hmac.verify(message: bytes, signature: bytes, secret: bytes, algorithm: string) -> bool
    hmac.verify(message: string, signature: string, secret: string, algorithm: string) -> bool

The `bytes` overload compares the signature to the computed code directly.

The `string` overload additionally accommodates the signature encodings used by
common webhook schemes. Surrounding whitespace is ignored, and a leading
`<prefix>=` marker is stripped when the prefix names a registered algorithm
(e.g. GitHub's `sha256=...`) or is the version marker `v0=` or `v1=` (e.g.
Slack's and Stripe's). A stripped algorithm prefix takes precedence over the
`algorithm` argument. The remaining signature is accepted as hex, standard
base64, or URL-safe base64, with or without padding.

Unlike `hmac.compute` and `hmac.digest`, this function does not return an
error. An unregistered algorithm, an unparsable signature, and a genuine
mismatch all evaluate to false.

Examples:

    hmac.verify(b'payload', signature, b'key', hmac.SHA256)

    // hex, base64, and prefixed encodings of the same code all verify
    hmac.verify('payload', '5d98b45c90a207fa998ce639fea6f02ecc8cc3f36fef81d694fb856b4d0a28ca',
                'key', hmac.SHA256)                                   // return true
    hmac.verify('payload', 'XZi0XJCiB/qZjOY5/qbwLsyMw/Nv74HWlPuFa00KKMo=',
                'key', hmac.SHA256)                                   // return true
    hmac.verify('payload', 'sha256=5d98b45c90a207fa998ce639fea6f02ecc8cc3f36fef81d694fb856b4d0a28ca',
                'key', hmac.SHA256)                                   // return true

    hmac.verify('payload', 'not-a-signature', 'key', hmac.SHA256)     // return false
    hmac.verify('payload', signature, 'key', 'MD5')                   // return false

## Algorithms

By default the library registers the algorithms below, each under its own name,
its JOSE/JWT alias, and a string constant for each of those names. The constants
evaluate to the canonical algorithm name, so `hmac.HS256 == 'SHA256'`.

| Algorithm    | Constants                          |
| ------------ | ---------------------------------- |
| SHA-224      | `hmac.SHA224`, `hmac.HS224`        |
| SHA-256      | `hmac.SHA256`, `hmac.HS256`        |
| SHA-384      | `hmac.SHA384`, `hmac.HS384`        |
| SHA-512      | `hmac.SHA512`, `hmac.HS512`        |
| SHA-512/224  | `hmac.SHA512_224`, `hmac.HS512_224`|
| SHA-512/256  | `hmac.SHA512_256`, `hmac.HS512_256`|

Referring to an algorithm which is not registered is a compile error when the
constant does not exist, and a runtime error or a false result when an
unregistered name is passed as a string.

## Cost

`hmac.digest` and `hmac.equal` report an unbounded cost: their estimated cost has
a maximum of `math.MaxUint64`, and their tracked runtime cost is `math.MaxUint64`.
An expression using either function will therefore exceed any `cel.CostLimit`.

This is deliberate. The work these functions perform is driven by input sizes the
checker cannot bound, so no smaller estimate would be sound. Environments which
enforce a cost limit should treat these functions as opt-in, and expose them only
where the inputs are otherwise constrained.

`hmac.compute` and `hmac.verify` do not currently declare cost estimators and are
charged the default per-call cost.

## Configuration

The `Library` option accepts the following options.

### Algorithm

Registers a `crypto.Hash` with optional aliases, replacing the default set. The
hash must be linked into the binary, as with the `crypto/sha256` and similar
blank imports. Both the algorithm's own name and each alias become a string
constant in the `hmac` namespace.

    hmaclib.Library(
        hmaclib.Algorithm(crypto.SHA256),               // hmac.SHA256
        hmaclib.Algorithm(crypto.SHA1, "legacy-sha1"),  // hmac.SHA1, hmac.LEGACY_SHA1
    )

Registering a specific set of algorithms is a way to keep expressions from
selecting a hash which is no longer considered acceptable.

### CommonAlgorithms

Registers the default set of algorithms described in [Algorithms](#algorithms).
This is applied automatically when no `Algorithm` option is supplied, and may
be named explicitly to extend the defaults rather than replace them.

    hmaclib.Library(
        hmaclib.CommonAlgorithms(),
        hmaclib.Algorithm(crypto.SHA1, "SHA1"),
    )

### MaxPrefixLength

Sets the maximum length of the `<prefix>=` marker which `hmac.verify` will
parse from a string signature. Defaults to 20.

    hmaclib.Library(hmaclib.MaxPrefixLength(40))
