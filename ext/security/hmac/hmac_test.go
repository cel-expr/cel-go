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
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	hmaclib "github.com/google/cel-go/ext/security/hmac"
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

func TestHMACCustomAlgorithmOption(t *testing.T) {
	env, err := cel.NewEnv(
		hmaclib.Library(
			hmaclib.Algorithm(crypto.MD5, "MD5", "HASH-MD5"),
			hmaclib.Algorithm(crypto.SHA1, "SHA1"),
		),
		cel.Variable("msg", cel.StringType),
		cel.Variable("secret", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	msgStr := "hello custom alg"
	secretStr := "key"
	vars := map[string]any{
		"msg":    msgStr,
		"secret": secretStr,
	}

	hMD5 := hmac.New(md5.New, []byte(secretStr))
	hMD5.Write([]byte(msgStr))
	macMD5Bytes := hMD5.Sum(nil)
	macMD5Hex := hex.EncodeToString(macMD5Bytes)

	hSHA1 := hmac.New(sha1.New, []byte(secretStr))
	hSHA1.Write([]byte(msgStr))
	macSHA1Hex := hex.EncodeToString(hSHA1.Sum(nil))

	tests := []struct {
		name string
		expr string
		want any
	}{
		{
			name: "md5_constant",
			expr: `hmac.MD5`,
			want: "MD5",
		},
		{
			name: "sha1_constant",
			expr: `hmac.SHA1`,
			want: "SHA1",
		},
		{
			name: "compute_custom_md5",
			expr: `hmac.compute(msg, secret, hmac.MD5)`,
			want: macMD5Bytes,
		},
		{
			name: "compute_custom_md5_alias",
			expr: `hmac.compute(msg, secret, 'HASH-MD5')`,
			want: macMD5Bytes,
		},
		{
			name: "verify_custom_md5_prefixed",
			expr: `hmac.verify(msg, 'md5=` + macMD5Hex + `', secret, hmac.MD5)`,
			want: true,
		},
		{
			name: "verify_custom_sha1_prefixed",
			expr: `hmac.verify(msg, 'sha1=` + macSHA1Hex + `', secret, hmac.SHA1)`,
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
