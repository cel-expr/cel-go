// Copyright 2026 Google LLC
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

package hmac_test

import (
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"math"
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/ext"
	hmaclib "cel.dev/cel-go/ext/security/hmac"
)

func evalExpr(t *testing.T, env *cel.Env, expr string, vars map[string]any) any {
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		t.Fatalf("Compile(%q) failed: %v", expr, issues.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program(%q) failed: %v", expr, err)
	}
	val, _, err := prg.Eval(vars)
	if err != nil {
		t.Fatalf("Eval(%q) failed: %v", expr, err)
	}
	return val.Value()
}

func TestHMACUniformSignaturesAndConstants(t *testing.T) {
	secretStr := "my-shared-secret-key"
	secretBytes := []byte(secretStr)
	msgStr := `{"action":"push","ref":"refs/heads/main"}`
	msgBytes := []byte(msgStr)

	// Compute expected SHA256 MAC
	h256 := hmac.New(sha256.New, secretBytes)
	h256.Write(msgBytes)
	mac256Bytes := h256.Sum(nil)
	mac256Hex := hex.EncodeToString(mac256Bytes)
	mac256B64 := base64.StdEncoding.EncodeToString(mac256Bytes)
	mac256B64URL := base64.RawURLEncoding.EncodeToString(mac256Bytes)

	// Compute expected SHA512 MAC
	h512 := hmac.New(sha512.New, secretBytes)
	h512.Write(msgBytes)
	mac512Bytes := h512.Sum(nil)
	mac512Hex := hex.EncodeToString(mac512Bytes)

	env, err := cel.NewEnv(
		hmaclib.Library(),
		cel.Variable("msgStr", cel.StringType),
		cel.Variable("msgBytes", cel.BytesType),
		cel.Variable("secretStr", cel.StringType),
		cel.Variable("secretBytes", cel.BytesType),
		cel.Variable("sigHex", cel.StringType),
		cel.Variable("sigB64", cel.StringType),
		cel.Variable("sigB64URL", cel.StringType),
		cel.Variable("sigBytes", cel.BytesType),
		cel.Variable("sigGitHub", cel.StringType),
		cel.Variable("sigStripe", cel.StringType),
		cel.Variable("sig512Hex", cel.StringType),
		cel.Variable("sig512Bytes", cel.BytesType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"msgStr":      msgStr,
		"msgBytes":    msgBytes,
		"secretStr":   secretStr,
		"secretBytes": secretBytes,
		"sigHex":      mac256Hex,
		"sigB64":      mac256B64,
		"sigB64URL":   mac256B64URL,
		"sigBytes":    mac256Bytes,
		"sigGitHub":   "sha256=" + mac256Hex,
		"sigStripe":   "v1=" + mac256Hex,
		"sig512Hex":   mac512Hex,
		"sig512Bytes": mac512Bytes,
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		// Uniform all-bytes verify
		{
			name: "verify_all_bytes_sha256",
			expr: `hmac.verify(msgBytes, sigBytes, secretBytes, hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_all_bytes_sha512",
			expr: `hmac.verify(msgBytes, sig512Bytes, secretBytes, hmac.SHA512)`,
			want: true,
		},
		{
			name: "verify_all_bytes_mismatch_sig",
			expr: `hmac.verify(msgBytes, sig512Bytes, secretBytes, hmac.SHA256)`,
			want: false,
		},

		// Uniform all-strings verify
		{
			name: "verify_all_strings_hex",
			expr: `hmac.verify(msgStr, sigHex, secretStr, hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_all_strings_b64",
			expr: `hmac.verify(msgStr, sigB64, secretStr, hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_all_strings_b64url",
			expr: `hmac.verify(msgStr, sigB64URL, secretStr, hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_all_strings_github_prefixed",
			expr: `hmac.verify(msgStr, sigGitHub, secretStr, hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_all_strings_stripe_prefixed",
			expr: `hmac.verify(msgStr, sigStripe, secretStr, hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_all_strings_sha512",
			expr: `hmac.verify(msgStr, sig512Hex, secretStr, hmac.SHA512)`,
			want: true,
		},
		{
			name: "verify_all_strings_string_literal_alg",
			expr: `hmac.verify(msgStr, sigHex, secretStr, 'SHA256')`,
			want: true,
		},

		// Uniform compute (returning bytes)
		{
			name: "compute_bytes_bytes_sha256",
			expr: `hmac.compute(msgBytes, secretBytes, hmac.SHA256) == sigBytes`,
			want: true,
		},
		{
			name: "compute_string_string_sha256",
			expr: `hmac.compute(msgStr, secretStr, hmac.SHA256) == sigBytes`,
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, vars)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
			}
		})
	}
}

func TestAlgorithmConstants(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	tests := []struct {
		expr string
		want string
	}{
		{`hmac.SHA256`, "SHA256"},
		{`hmac.SHA384`, "SHA384"},
		{`hmac.SHA512`, "SHA512"},
		{`hmac.SHA224`, "SHA224"},
		{`hmac.SHA512_256`, "SHA512_256"},
		{`hmac.SHA512_224`, "SHA512_224"},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, nil)
			if got != tc.want {
				t.Errorf("Eval(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestHMACCompositionWithEncodersAndStrings(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(),
		ext.Encoders(),
		ext.Strings(),
		cel.Variable("msg", cel.StringType),
		cel.Variable("secret", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"msg":    "hello world",
		"secret": "key",
	}

	resBytes := evalExpr(t, env, `hmac.compute(msg, secret, hmac.SHA256)`, vars).([]byte)
	expectedHex := hex.EncodeToString(resBytes)
	expectedB64 := base64.StdEncoding.EncodeToString(resBytes)

	tests := []struct {
		name string
		expr string
		want any
	}{
		{
			name: "format_hex",
			expr: `"%x".format([hmac.compute(msg, secret, hmac.SHA256)])`,
			want: expectedHex,
		},
		{
			name: "base64_encode",
			expr: `base64.encode(hmac.compute(msg, secret, hmac.SHA256))`,
			want: expectedB64,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, vars)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
			}
		})
	}
}

func TestHMACAllAlgorithmsAndPrefixes(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(hmaclib.Version(1)),
		cel.Variable("msgStr", cel.StringType),
		cel.Variable("secretStr", cel.StringType),
		cel.Variable("sig", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	msg := "test-message"
	secret := "secret-key"
	vars := map[string]any{
		"msgStr":    msg,
		"secretStr": secret,
	}

	algTests := []struct {
		alg string
	}{
		{"SHA256"},
		{"SHA384"},
		{"SHA512"},
		{"SHA224"},
		{"SHA512/256"},
		{"SHA512/224"},
		{"HS256"},
		{"HS384"},
		{"HS512"},
		{"HS224"},
		{"HS512/256"},
		{"HS512/224"},
	}

	for _, tc := range algTests {
		t.Run("compute_"+tc.alg, func(t *testing.T) {
			mac := evalExpr(t, env, `hmac.compute(msgStr, secretStr, '`+tc.alg+`')`, vars)
			if len(mac.([]byte)) == 0 {
				t.Errorf("empty mac for alg %q", tc.alg)
			}
		})
	}

	prefixTests := []struct {
		prefix string
		alg    string
	}{
		{"sha256=", "SHA256"},
		{"sha-256=", "SHA256"},
		{"hs256=", "SHA256"},
		{"sha384=", "SHA384"},
		{"sha-384=", "SHA384"},
		{"hs384=", "SHA384"},
		{"sha512=", "SHA512"},
		{"sha-512=", "SHA512"},
		{"hs512=", "SHA512"},
		{"v0=", "SHA256"},
		{"v1=", "SHA256"},
	}

	for _, tc := range prefixTests {
		t.Run("prefix_"+tc.prefix, func(t *testing.T) {
			macBytes := evalExpr(t, env, `hmac.compute(msgStr, secretStr, '`+tc.alg+`')`, vars).([]byte)
			sigStr := tc.prefix + hex.EncodeToString(macBytes)
			got := evalExpr(t, env, `hmac.verify(msgStr, '`+sigStr+`', secretStr, '`+tc.alg+`')`, vars)
			if got != true {
				t.Errorf("verify failed for prefix %q: got %v", tc.prefix, got)
			}
		})
	}

	rawSig := string(evalExpr(t, env, `hmac.compute(msgStr, secretStr, hmac.SHA256)`, vars).([]byte))
	rawVars := map[string]any{"msgStr": msg, "secretStr": secret, "sig": rawSig}

	invalidTests := []struct {
		name string
		expr string
		vars map[string]any
		want any
	}{
		{
			name: "verify_raw_string",
			expr: `hmac.verify(msgStr, sig, secretStr, hmac.SHA256)`,
			vars: rawVars,
			want: true,
		},
		{
			name: "verify_unknown_alg_string",
			expr: `hmac.verify(msgStr, 'sig', secretStr, 'UNKNOWN_ALG')`,
			vars: vars,
			want: false,
		},
		{
			name: "verify_unknown_alg_bytes",
			expr: `hmac.verify(bytes(msgStr), bytes('sig'), bytes(secretStr), 'UNKNOWN_ALG')`,
			vars: vars,
			want: false,
		},
	}

	for _, tc := range invalidTests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, tc.vars)
			if got != tc.want {
				t.Errorf("Eval(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestHMACSpecificAlgorithmOptions(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(hmaclib.Algorithm(crypto.SHA256)),
		cel.Variable("msgStr", cel.StringType),
		cel.Variable("secretStr", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"msgStr":    "msg",
		"secretStr": "key",
	}

	tests := []struct {
		name string
		expr string
		mode string // "compile_error" or "eval_error" or "success"
	}{
		{
			name: "sha256_success",
			expr: `hmac.compute(msgStr, secretStr, hmac.SHA256)`,
			mode: "success",
		},
		{
			name: "unregistered_constant_sha512",
			expr: `hmac.compute(msgStr, secretStr, hmac.SHA512)`,
			mode: "compile_error",
		},
		{
			name: "unregistered_literal_sha512",
			expr: `hmac.compute(msgStr, secretStr, 'SHA512')`,
			mode: "eval_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, issues := env.Compile(tc.expr)
			if tc.mode == "compile_error" {
				if issues == nil || issues.Err() == nil {
					t.Errorf("expected compile error for %q, got nil", tc.expr)
				}
				return
			}
			if issues != nil && issues.Err() != nil {
				t.Fatalf("Compile(%q) failed unexpectedly: %v", tc.expr, issues.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", tc.expr, err)
			}
			_, _, evalErr := prg.Eval(vars)
			if tc.mode == "eval_error" {
				if evalErr == nil {
					t.Errorf("expected eval error for %q, got nil", tc.expr)
				}
			} else if evalErr != nil {
				t.Errorf("unexpected eval error for %q: %v", tc.expr, evalErr)
			}
		})
	}
}

func TestHMACCommonAlgorithmsOption(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(hmaclib.CommonAlgorithms()),
		cel.Variable("msgStr", cel.StringType),
		cel.Variable("secretStr", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"msgStr":    "msg",
		"secretStr": "key",
	}

	tests := []struct {
		name string
		expr string
	}{
		{
			name: "sha256",
			expr: `hmac.compute(msgStr, secretStr, hmac.SHA256)`,
		},
		{
			name: "sha512",
			expr: `hmac.compute(msgStr, secretStr, hmac.SHA512)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, issues := env.Compile(tc.expr)
			if issues != nil && issues.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tc.expr, issues.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", tc.expr, err)
			}
			if _, _, err := prg.Eval(vars); err != nil {
				t.Errorf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestHMACMaxPrefixLengthOption(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(
			hmaclib.CommonAlgorithms(),
			hmaclib.MaxPrefixLength(40),
			hmaclib.Algorithm(crypto.SHA256, "very-long-prefix-custom-algorithm"),
		),
		cel.Variable("msg", cel.StringType),
		cel.Variable("secret", cel.StringType),
		cel.Variable("sigStr", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	msg := "test"
	secret := "key"
	vars := map[string]any{
		"msg":    msg,
		"secret": secret,
	}
	mac := evalExpr(t, env, `hmac.compute(msg, secret, hmac.SHA256)`, vars).([]byte)
	sigStr := "very-long-prefix-custom-algorithm=" + hex.EncodeToString(mac)
	vars["sigStr"] = sigStr

	tests := []struct {
		name string
		expr string
		want any
	}{
		{
			name: "verify_long_prefix",
			expr: `hmac.verify(msg, sigStr, secret, hmac.SHA256)`,
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, vars)
			if got != tc.want {
				t.Errorf("Eval(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestHMACDigest(t *testing.T) {
	msgStr := `{"action":"push","ref":"refs/heads/main"}`
	msgBytes := []byte(msgStr)

	unicodeStr := "h\u00e9llo \u4e16\u754c"

	sha256Sum := sha256.Sum256(msgBytes)
	sha512Sum := sha512.Sum512(msgBytes)
	emptySum := sha256.Sum256(nil)
	unicodeSum := sha256.Sum256([]byte(unicodeStr))

	env, err := cel.NewEnv(
		hmaclib.Library(),
		ext.Encoders(),
		ext.Strings(),
		cel.Variable("msgStr", cel.StringType),
		cel.Variable("msgBytes", cel.BytesType),
		cel.Variable("msgUnicode", cel.StringType),
		cel.Variable("secretBytes", cel.BytesType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"msgStr":      msgStr,
		"msgBytes":    msgBytes,
		"msgUnicode":  unicodeStr,
		"secretBytes": []byte("my-shared-secret-key"),
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		{
			name: "digest_sha256",
			expr: `hmac.digest(msgBytes, hmac.SHA256)`,
			want: sha256Sum[:],
		},
		{
			name: "digest_sha512",
			expr: `hmac.digest(msgBytes, hmac.SHA512)`,
			want: sha512Sum[:],
		},
		{
			name: "digest_alias_alg",
			expr: `hmac.digest(msgBytes, 'HS256')`,
			want: sha256Sum[:],
		},
		{
			name: "digest_literal_alg",
			expr: `hmac.digest(msgBytes, 'SHA-256')`,
			want: sha256Sum[:],
		},
		{
			name: "digest_empty_message",
			expr: `hmac.digest(b'', hmac.SHA256)`,
			want: emptySum[:],
		},
		{
			name: "digest_is_unkeyed",
			expr: `hmac.digest(msgBytes, hmac.SHA256) != hmac.compute(msgBytes, secretBytes, hmac.SHA256)`,
			want: true,
		},
		{
			name: "digest_string_sha256",
			expr: `hmac.digest(msgStr, hmac.SHA256)`,
			want: sha256Sum[:],
		},
		{
			name: "digest_string_sha512",
			expr: `hmac.digest(msgStr, hmac.SHA512)`,
			want: sha512Sum[:],
		},
		{
			name: "digest_string_alias_alg",
			expr: `hmac.digest(msgStr, 'HS256')`,
			want: sha256Sum[:],
		},
		{
			name: "digest_string_empty_message",
			expr: `hmac.digest('', hmac.SHA256)`,
			want: emptySum[:],
		},
		{
			// The string overload hashes the UTF-8 encoding of the message,
			// so it must agree with the bytes overload for every input.
			name: "digest_string_matches_bytes",
			expr: `hmac.digest(msgStr, hmac.SHA256) == hmac.digest(msgBytes, hmac.SHA256)`,
			want: true,
		},
		{
			name: "digest_string_multibyte_utf8",
			expr: `hmac.digest(msgUnicode, hmac.SHA256)`,
			want: unicodeSum[:],
		},
		{
			name: "digest_string_multibyte_matches_bytes",
			expr: `hmac.digest(msgUnicode, hmac.SHA256) == hmac.digest(bytes(msgUnicode), hmac.SHA256)`,
			want: true,
		},
		{
			name: "digest_composes_with_encoders",
			expr: `base64.encode(hmac.digest(msgBytes, hmac.SHA256))`,
			want: base64.StdEncoding.EncodeToString(sha256Sum[:]),
		},
		{
			name: "digest_composes_with_format",
			expr: `"%x".format([hmac.digest(msgBytes, hmac.SHA256)])`,
			want: hex.EncodeToString(sha256Sum[:]),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, vars)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("evalExpr(%q) got %v, wanted %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestHMACDigestUnsupportedAlgorithm(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(hmaclib.Algorithm(crypto.SHA256)),
		cel.Variable("msgBytes", cel.BytesType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}
	for _, expr := range []string{
		`hmac.digest(msgBytes, 'SHA512')`,
		`hmac.digest('msg', 'SHA512')`,
	} {
		t.Run(expr, func(t *testing.T) {
			ast, issues := env.Compile(expr)
			if issues != nil && issues.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", expr, issues.Err())
			}
			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			_, _, err = prg.Eval(map[string]any{"msgBytes": []byte("msg")})
			if err == nil {
				t.Fatalf("Eval(%q) got nil error, wanted unsupported HMAC hash algorithm error", expr)
			}
			if !strings.Contains(err.Error(), "unsupported HMAC hash algorithm") {
				t.Errorf("Eval(%q) got error %v, wanted unsupported HMAC hash algorithm error", expr, err)
			}
		})
	}
}

func TestHMACEqual(t *testing.T) {
	secretBytes := []byte("my-shared-secret-key")
	msgBytes := []byte(`{"action":"push","ref":"refs/heads/main"}`)

	h256 := hmac.New(sha256.New, secretBytes)
	h256.Write(msgBytes)
	mac256Bytes := h256.Sum(nil)

	env, err := cel.NewEnv(
		hmaclib.Library(),
		ext.Encoders(),
		cel.Variable("msgBytes", cel.BytesType),
		cel.Variable("secretBytes", cel.BytesType),
		cel.Variable("sigBytes", cel.BytesType),
		cel.Variable("sigHex", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"msgBytes":    msgBytes,
		"secretBytes": secretBytes,
		"sigBytes":    mac256Bytes,
		"sigHex":      hex.EncodeToString(mac256Bytes),
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		{
			name: "equal_identical_literals",
			expr: `hmac.equal(b'abc', b'abc')`,
			want: true,
		},
		{
			name: "equal_differing_literals",
			expr: `hmac.equal(b'abc', b'abd')`,
			want: false,
		},
		{
			name: "equal_differing_lengths",
			expr: `hmac.equal(b'abc', b'abcd')`,
			want: false,
		},
		{
			name: "equal_both_empty",
			expr: `hmac.equal(b'', b'')`,
			want: true,
		},
		{
			name: "equal_empty_and_non_empty",
			expr: `hmac.equal(b'', b'a')`,
			want: false,
		},
		{
			name: "equal_computed_mac",
			expr: `hmac.equal(hmac.compute(msgBytes, secretBytes, hmac.SHA256), sigBytes)`,
			want: true,
		},
		{
			name: "equal_computed_mac_wrong_algorithm",
			expr: `hmac.equal(hmac.compute(msgBytes, secretBytes, hmac.SHA512), sigBytes)`,
			want: false,
		},
		{
			name: "equal_computed_mac_wrong_secret",
			expr: `hmac.equal(hmac.compute(msgBytes, b'wrong-secret', hmac.SHA256), sigBytes)`,
			want: false,
		},
		{
			name: "equal_decoded_hex_signature",
			expr: `hmac.equal(hmac.compute(msgBytes, secretBytes, hmac.SHA256), base64.decode(base64.encode(sigBytes)))`,
			want: true,
		},
		{
			name: "equal_agrees_with_verify",
			expr: `hmac.equal(hmac.compute(msgBytes, secretBytes, hmac.SHA256), sigBytes) == hmac.verify(msgBytes, sigBytes, secretBytes, hmac.SHA256)`,
			want: true,
		},
		{
			name: "equal_digests",
			expr: `hmac.equal(hmac.digest(msgBytes, hmac.SHA256), hmac.digest(msgBytes, hmac.SHA256))`,
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, vars)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("evalExpr(%q) got %v, wanted %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestHMACEqualStringsUnsupported(t *testing.T) {
	env, err := cel.NewEnv(hmaclib.Library())
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}
	if _, issues := env.Compile(`hmac.equal('abc', 'abc')`); issues == nil || issues.Err() == nil {
		t.Error("Compile(`hmac.equal('abc', 'abc')`) got nil error, wanted no matching overload")
	}
}

// TestHMACREADMEExamples evaluates the examples documented in README.md so that
// the documented values cannot drift from the implementation.
func TestHMACREADMEExamples(t *testing.T) {
	const sigHex = "5d98b45c90a207fa998ce639fea6f02ecc8cc3f36fef81d694fb856b4d0a28ca"

	env, err := cel.NewEnv(
		hmaclib.Library(),
		ext.Encoders(),
		ext.Strings(),
		cel.Variable("signature", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}
	vars := map[string]any{"signature": sigHex}

	tests := []struct {
		name string
		expr string
		want any
	}{
		// Overloads agree on equivalent inputs.
		{
			name: "string_and_bytes_digests_agree",
			expr: `hmac.digest('payload', hmac.SHA256) == hmac.digest(b'payload', hmac.SHA256)`,
			want: true,
		},

		// Hmac.Compute
		{
			name: "compute_hex",
			expr: `"%x".format([hmac.compute('payload', 'key', hmac.SHA256)])`,
			want: sigHex,
		},
		{
			name: "compute_base64",
			expr: `base64.encode(hmac.compute(b'payload', b'key', hmac.SHA256))`,
			want: "XZi0XJCiB/qZjOY5/qbwLsyMw/Nv74HWlPuFa00KKMo=",
		},

		// Hmac.Digest
		{
			name: "digest_hex",
			expr: `"%x".format([hmac.digest('payload', hmac.SHA256)])`,
			want: "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5",
		},
		{
			name: "digest_base64",
			expr: `base64.encode(hmac.digest(b'payload', hmac.SHA256))`,
			want: "I59Z7VXnN8dxR89VrQwbAwttfudIp0JpUvm4UtWpNeU=",
		},

		// Hmac.Equal
		{
			name: "equal_same",
			expr: `hmac.equal(b'abc', b'abc')`,
			want: true,
		},
		{
			name: "equal_differs",
			expr: `hmac.equal(b'abc', b'abd')`,
			want: false,
		},
		{
			name: "equal_shorter",
			expr: `hmac.equal(b'abc', b'ab')`,
			want: false,
		},

		// Hmac.Verify signature encodings.
		{
			name: "verify_hex",
			expr: `hmac.verify('payload', '` + sigHex + `', 'key', hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_base64",
			expr: `hmac.verify('payload', 'XZi0XJCiB/qZjOY5/qbwLsyMw/Nv74HWlPuFa00KKMo=', 'key', hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_algorithm_prefix",
			expr: `hmac.verify('payload', 'sha256=` + sigHex + `', 'key', hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_version_prefix",
			expr: `hmac.verify('payload', 'v1=` + sigHex + `', 'key', hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_surrounding_whitespace",
			expr: `hmac.verify('payload', '  ` + sigHex + `  ', 'key', hmac.SHA256)`,
			want: true,
		},
		{
			// A recognized algorithm prefix wins over the algorithm argument.
			name: "verify_prefix_overrides_algorithm_argument",
			expr: `hmac.verify('payload', 'sha512=' + "%x".format([hmac.compute('payload', 'key', hmac.SHA512)]), 'key', hmac.SHA256)`,
			want: true,
		},
		{
			name: "verify_unparsable_signature",
			expr: `hmac.verify('payload', 'not-a-signature', 'key', hmac.SHA256)`,
			want: false,
		},
		{
			// verify reports false rather than erroring on an unregistered algorithm.
			name: "verify_unregistered_algorithm",
			expr: `hmac.verify('payload', signature, 'key', 'MD5')`,
			want: false,
		},

		// Algorithm name matching and constant values.
		{
			name: "constant_alias_value",
			expr: `hmac.HS256 == 'SHA256'`,
			want: true,
		},
		{
			name: "constant_alias_value_sha512_256",
			expr: `hmac.HS512_256 == 'SHA512_256'`,
			want: true,
		},
		{
			name: "algorithm_name_case_and_separator_insensitive",
			expr: `hmac.digest('payload', 'sha-256') == hmac.digest('payload', hmac.SHA256)`,
			want: true,
		},
		{
			name: "algorithm_name_slash_separator",
			expr: `hmac.digest('payload', 'SHA-512/256') == hmac.digest('payload', hmac.SHA512_256)`,
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalExpr(t, env, tc.expr, vars)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("evalExpr(%q) got %v, wanted %v", tc.expr, got, tc.want)
			}
		})
	}
}

// TestHMACREADMEConfiguration exercises the Configuration section of README.md.
func TestHMACREADMEConfiguration(t *testing.T) {
	t.Run("Algorithm_replaces_defaults", func(t *testing.T) {
		env, err := cel.NewEnv(hmaclib.Library(
			hmaclib.Algorithm(crypto.SHA256),
			hmaclib.Algorithm(crypto.SHA1, "legacy-sha1"),
		))
		if err != nil {
			t.Fatalf("cel.NewEnv failed: %v", err)
		}
		for expr, want := range map[string]string{
			`hmac.SHA256`:      "SHA256",
			`hmac.SHA1`:        "SHA1",
			`hmac.LEGACY_SHA1`: "SHA1",
		} {
			if got := evalExpr(t, env, expr, nil); got != want {
				t.Errorf("evalExpr(%q) got %v, wanted %v", expr, got, want)
			}
		}
		if _, issues := env.Compile(`hmac.SHA512`); issues == nil || issues.Err() == nil {
			t.Error("Compile(`hmac.SHA512`) got nil error, wanted undeclared reference")
		}
	})

	t.Run("CommonAlgorithms_extends_defaults", func(t *testing.T) {
		env, err := cel.NewEnv(hmaclib.Library(
			hmaclib.CommonAlgorithms(),
			hmaclib.Algorithm(crypto.SHA1, "SHA1"),
		))
		if err != nil {
			t.Fatalf("cel.NewEnv failed: %v", err)
		}
		for _, expr := range []string{`hmac.SHA256`, `hmac.HS512`, `hmac.SHA1`} {
			if _, issues := env.Compile(expr); issues != nil && issues.Err() != nil {
				t.Errorf("Compile(%q) failed: %v", expr, issues.Err())
			}
		}
	})
}

// TestHMACUnboundedCosts verifies that hmac.digest and hmac.equal report an unbounded
// estimated cost and an unbounded actual cost, including when combined with cost accrued
// elsewhere in the expression, and that they trip a cost limit.
func TestHMACUnboundedCosts(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(),
		cel.Variable("msgStr", cel.StringType),
		cel.Variable("msgBytes", cel.BytesType),
		cel.Variable("sigBytes", cel.BytesType),
		cel.Variable("secretBytes", cel.BytesType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}
	vars := map[string]any{
		"msgStr":      "payload",
		"msgBytes":    []byte("payload"),
		"sigBytes":    []byte("signature"),
		"secretBytes": []byte("key"),
	}

	unbounded := []string{
		`hmac.digest(msgBytes, hmac.SHA256)`,
		`hmac.digest(msgStr, hmac.SHA256)`,
		`hmac.equal(msgBytes, sigBytes)`,
		`hmac.equal(hmac.digest(msgBytes, hmac.SHA256), sigBytes)`,
		`size(msgBytes) > 0 && hmac.equal(hmac.digest(msgStr, hmac.SHA256), sigBytes)`,
	}
	for _, expr := range unbounded {
		t.Run(expr, func(t *testing.T) {
			ast, issues := env.Compile(expr)
			if issues != nil && issues.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", expr, issues.Err())
			}
			est, err := env.EstimateCost(ast, unboundedCostTestEstimator{})
			if err != nil {
				t.Fatalf("EstimateCost(%q) failed: %v", expr, err)
			}
			if est.Max != math.MaxUint64 {
				t.Errorf("EstimateCost(%q) got max %d, wanted %d", expr, est.Max, uint64(math.MaxUint64))
			}

			prg, err := env.Program(ast, cel.CostTracking(nil))
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			_, det, err := prg.Eval(vars)
			if err != nil {
				t.Fatalf("Eval(%q) failed: %v", expr, err)
			}
			if det.ActualCost() == nil {
				t.Fatalf("Eval(%q) got nil actual cost, wanted a value", expr)
			}
			if *det.ActualCost() != math.MaxUint64 {
				t.Errorf("Eval(%q) got actual cost %d, wanted %d", expr, *det.ActualCost(), uint64(math.MaxUint64))
			}

			limited, err := env.Program(ast, cel.CostLimit(1000))
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			if _, _, err := limited.Eval(vars); err == nil {
				t.Errorf("Eval(%q) under a cost limit got nil error, wanted cost limit exceeded", expr)
			}
		})
	}

	// compute and verify are unchanged and remain bounded.
	bounded := []string{
		`hmac.compute(msgBytes, secretBytes, hmac.SHA256)`,
		`hmac.verify(msgBytes, sigBytes, secretBytes, hmac.SHA256)`,
	}
	for _, expr := range bounded {
		t.Run(expr, func(t *testing.T) {
			ast, issues := env.Compile(expr)
			if issues != nil && issues.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", expr, issues.Err())
			}
			est, err := env.EstimateCost(ast, unboundedCostTestEstimator{})
			if err != nil {
				t.Fatalf("EstimateCost(%q) failed: %v", expr, err)
			}
			if est.Max == math.MaxUint64 {
				t.Errorf("EstimateCost(%q) got an unbounded max, wanted a bounded estimate", expr)
			}
			prg, err := env.Program(ast, cel.CostTracking(nil))
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			if _, det, err := prg.Eval(vars); err != nil {
				t.Fatalf("Eval(%q) failed: %v", expr, err)
			} else if *det.ActualCost() == math.MaxUint64 {
				t.Errorf("Eval(%q) got an unbounded actual cost, wanted a bounded cost", expr)
			}
		})
	}
}

type unboundedCostTestEstimator struct{}

func (unboundedCostTestEstimator) EstimateSize(cost.AstNode) *cost.SizeEstimate {
	return nil
}

func (unboundedCostTestEstimator) EstimateCallCost(function, overloadID string, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	return nil
}
