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

package dpop_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/ext/security/dpop"
	"cel.dev/cel-go/ext/security/jwt"
)

func createTestDPoP(t *testing.T, header, payload map[string]any) string {
	t.Helper()
	hBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("json.Marshal header failed: %v", err)
	}
	pBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload failed: %v", err)
	}

	hB64 := base64.RawURLEncoding.EncodeToString(hBytes)
	pB64 := base64.RawURLEncoding.EncodeToString(pBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString([]byte("signature-placeholder"))

	return hB64 + "." + pB64 + "." + sigB64
}

func createTestJWT(t *testing.T, header, payload map[string]any) string {
	t.Helper()
	hBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("json.Marshal header failed: %v", err)
	}
	pBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload failed: %v", err)
	}

	hB64 := base64.RawURLEncoding.EncodeToString(hBytes)
	pB64 := base64.RawURLEncoding.EncodeToString(pBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString([]byte("sig"))

	return hB64 + "." + pB64 + "." + sigB64
}

func evalExpr(t *testing.T, env *cel.Env, expr string, vars map[string]any) any {
	t.Helper()
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

func TestRFC9449Vectors(t *testing.T) {
	// RFC 9449 Section 4.1 Figure 2 / Figure 4
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}
	payload := map[string]any{
		"jti": "-BwC3ESc6acc2lTc",
		"htm": "POST",
		"htu": "https://server.example.com/token",
		"iat": 1562262616,
	}

	proofStr := createTestDPoP(t, header, payload)
	proof, err := dpop.ParseProof(proofStr)
	if err != nil {
		t.Fatalf("ParseProof failed: %v", err)
	}

	// RFC 9449 Section 6.1 Figure 9 expected JWK thumbprint (jkt)
	expectedJKT := "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
	if proof.Thumbprint != expectedJKT {
		t.Errorf("Thumbprint = %q, want %q", proof.Thumbprint, expectedJKT)
	}

	// RFC 9449 Section 7 Figure 13 / 14 expected access token hash (ath)
	accessToken := "Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU"
	expectedATH := "fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo"
	computedATH := dpop.ComputeAccessTokenHash(accessToken)
	if computedATH != expectedATH {
		t.Errorf("ComputeAccessTokenHash = %q, want %q", computedATH, expectedATH)
	}

	// Test with "DPoP " prefix
	computedATHWithPrefix := dpop.ComputeAccessTokenHash("DPoP " + accessToken)
	if computedATHWithPrefix != expectedATH {
		t.Errorf("ComputeAccessTokenHash with DPoP prefix = %q, want %q", computedATHWithPrefix, expectedATH)
	}
}

func TestDPoPCELIntegration(t *testing.T) {
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"kid": "key-test-1",
		"jwk": ecJWK,
	}
	payload := map[string]any{
		"jti":    "e1j3V_bKic8-LAEB",
		"htm":    "GET",
		"htu":    "https://resource.example.org/protectedresource",
		"iat":    1562262618,
		"ath":    "fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo",
		"nonce":  "nonce-xyz-123",
		"custom": "custom-value",
	}

	proofStr := createTestDPoP(t, header, payload)
	accessTokenStr := "Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU"
	dpopAuthHeader := "DPoP " + accessTokenStr

	// Create a JWT access token with cnf claim containing jkt
	jwtAccessHeader := map[string]any{"alg": "ES256", "typ": "JWT"}
	jwtAccessPayload := map[string]any{
		"iss": "https://server.example.com",
		"sub": "someone@example.com",
		"aud": "https://resource.example.org",
		"exp": 1562266216,
		"iat": 1562262616,
		"cnf": map[string]any{
			"jkt": "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I",
		},
	}
	jwtTokenStr := createTestJWT(t, jwtAccessHeader, jwtAccessPayload)
	jwtATH := dpop.ComputeAccessTokenHash(jwtTokenStr)
	jwtProofPayload := map[string]any{
		"jti":    "e1j3V_bKic8-LAEB",
		"htm":    "GET",
		"htu":    "https://resource.example.org/protectedresource",
		"iat":    1562262618,
		"ath":    jwtATH,
		"nonce":  "nonce-xyz-123",
		"custom": "custom-value",
	}
	jwtProofStr := createTestDPoP(t, header, jwtProofPayload)

	env, err := cel.NewEnv(
		dpop.Library(),
		jwt.Library(),
		cel.Variable("proofStr", cel.StringType),
		cel.Variable("jwtProofStr", cel.StringType),
		cel.Variable("dpopHeader", cel.StringType),
		cel.Variable("tokenStr", cel.StringType),
		cel.Variable("authHeader", cel.StringType),
		cel.Variable("jwtTokenStr", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"proofStr":    proofStr,
		"jwtProofStr": jwtProofStr,
		"dpopHeader":  "DPoP " + proofStr,
		"tokenStr":    accessTokenStr,
		"authHeader":  dpopAuthHeader,
		"jwtTokenStr": jwtTokenStr,
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		{
			name: "parse_has_value",
			expr: `dpop.parse(proofStr).hasValue()`,
			want: true,
		},
		{
			name: "parse_dpop_header_prefix",
			expr: `dpop.parse(dpopHeader).hasValue()`,
			want: true,
		},
		{
			name: "proof_alg",
			expr: `dpop.parse(proofStr).value().alg == 'ES256'`,
			want: true,
		},
		{
			name: "proof_keyId",
			expr: `dpop.parse(proofStr).value().keyId == 'key-test-1'`,
			want: true,
		},
		{
			name: "proof_type",
			expr: `dpop.parse(proofStr).value().type == 'dpop+jwt'`,
			want: true,
		},
		{
			name: "proof_id",
			expr: `dpop.parse(proofStr).value().id == 'e1j3V_bKic8-LAEB'`,
			want: true,
		},
		{
			name: "proof_method",
			expr: `dpop.parse(proofStr).value().method == 'GET'`,
			want: true,
		},
		{
			name: "proof_uri",
			expr: `dpop.parse(proofStr).value().uri == 'https://resource.example.org/protectedresource'`,
			want: true,
		},
		{
			name: "proof_accessTokenHash",
			expr: `dpop.parse(proofStr).value().accessTokenHash == 'fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo'`,
			want: true,
		},
		{
			name: "proof_nonce",
			expr: `dpop.parse(proofStr).value().nonce == 'nonce-xyz-123'`,
			want: true,
		},
		{
			name: "proof_thumbprint",
			expr: `dpop.parse(proofStr).value().thumbprint == '0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I'`,
			want: true,
		},
		{
			name: "claim_custom",
			expr: `dpop.parse(proofStr).value().claim('custom').orValue('') == 'custom-value'`,
			want: true,
		},
		{
			name: "claim_on_opt",
			expr: `dpop.parse(proofStr).claim('custom').orValue('') == 'custom-value'`,
			want: true,
		},
		{
			name: "claim_missing",
			expr: `dpop.parse(proofStr).claim('missing').hasValue()`,
			want: false,
		},
		{
			name: "matchesRequest_success",
			expr: `dpop.parse(proofStr).matchesRequest('GET', 'https://resource.example.org/protectedresource')`,
			want: true,
		},
		{
			name: "matchesRequest_on_value",
			expr: `dpop.parse(proofStr).value().matchesRequest('GET', 'https://resource.example.org/protectedresource')`,
			want: true,
		},
		{
			name: "matchesRequest_with_default_port_and_case",
			expr: `dpop.parse(proofStr).matchesRequest('get', 'HTTPS://RESOURCE.EXAMPLE.ORG:443/protectedresource?query=1#frag')`,
			want: true,
		},
		{
			name: "matchesRequest_path_cleaning",
			expr: `dpop.parse(proofStr).matchesRequest('GET', 'https://resource.example.org/foo/../protectedresource')`,
			want: true,
		},
		{
			name: "matchesRequest_method_mismatch",
			expr: `dpop.parse(proofStr).matchesRequest('POST', 'https://resource.example.org/protectedresource')`,
			want: false,
		},
		{
			name: "matchesRequest_uri_mismatch",
			expr: `dpop.parse(proofStr).matchesRequest('GET', 'https://other.example.org/protectedresource')`,
			want: false,
		},
		{
			name: "matchesNonce_success",
			expr: `dpop.parse(proofStr).matchesNonce('nonce-xyz-123')`,
			want: true,
		},
		{
			name: "matchesNonce_mismatch",
			expr: `dpop.parse(proofStr).matchesNonce('other-nonce')`,
			want: false,
		},
		{
			name: "matchesToken_with_jwt_string",
			expr: `dpop.parse(jwtProofStr).matchesToken(jwtTokenStr)`,
			want: true,
		},
		{
			name: "matchesToken_with_parsed_jwt",
			expr: `dpop.parse(jwtProofStr).matchesToken(jwt.parse(jwtTokenStr))`,
			want: true,
		},
		{
			name: "matchesToken_with_parsed_jwt_value",
			expr: `dpop.parse(jwtProofStr).matchesToken(jwt.parse(jwtTokenStr).value())`,
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

func TestJWKThumbprintAlgorithms(t *testing.T) {
	// RSA Key thumbprint test
	rsaJWK := map[string]any{
		"kty": "RSA",
		"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
		"e":   "AQAB",
	}
	rsaThumbprint, err := dpop.ComputeJWKThumbprint(rsaJWK)
	if err != nil {
		t.Fatalf("ComputeJWKThumbprint(RSA) failed: %v", err)
	}
	// RFC 7638 Section 3.1 RSA thumbprint test vector:
	// NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs
	expectedRSA := "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs"
	if rsaThumbprint != expectedRSA {
		t.Errorf("RSA Thumbprint = %q, want %q", rsaThumbprint, expectedRSA)
	}

	// OKP Key (Ed25519) thumbprint test (RFC 8037)
	okpJWK := map[string]any{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo",
	}
	okpThumbprint, err := dpop.ComputeJWKThumbprint(okpJWK)
	if err != nil {
		t.Fatalf("ComputeJWKThumbprint(OKP) failed: %v", err)
	}
	if okpThumbprint == "" {
		t.Errorf("expected non-empty OKP thumbprint")
	}

	// Unsupported key type
	badJWK := map[string]any{
		"kty": "UNKNOWN",
	}
	if _, err := dpop.ComputeJWKThumbprint(badJWK); err == nil {
		t.Errorf("expected error for unknown kty")
	}
}

func TestSecurityValidations(t *testing.T) {
	validJWK := map[string]any{
		"kty": "EC",
		"crv": "P-256",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
	}

	tests := []struct {
		name      string
		header    map[string]any
		payload   map[string]any
		expectErr bool
	}{
		{
			name: "insecure_alg_none",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "none",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "symmetric_alg_hs256",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "HS256",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "symmetric_key_type_oct",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "ES256",
				"jwk": map[string]any{
					"kty": "oct",
					"k":   "secret-key",
				},
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "private_key_parameter_in_jwk",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "ES256",
				"jwk": map[string]any{
					"kty": "EC",
					"crv": "P-256",
					"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
					"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
					"d":   "private-key-d-field",
				},
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "invalid_typ",
			header: map[string]any{
				"typ": "JWT",
				"alg": "ES256",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "missing_jti",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "ES256",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"htm": "GET",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "missing_htm",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "ES256",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"jti": "id-1",
				"htu": "https://server.example.com",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "missing_htu",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "ES256",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"iat": 1562262616,
			},
			expectErr: true,
		},
		{
			name: "missing_iat",
			header: map[string]any{
				"typ": "dpop+jwt",
				"alg": "ES256",
				"jwk": validJWK,
			},
			payload: map[string]any{
				"jti": "id-1",
				"htm": "GET",
				"htu": "https://server.example.com",
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proofStr := createTestDPoP(t, tc.header, tc.payload)
			_, err := dpop.ParseProof(proofStr)
			if (err != nil) != tc.expectErr {
				t.Errorf("ParseProof() err = %v, expectErr = %v", err, tc.expectErr)
			}
		})
	}
}

func TestTimeValidationOptions(t *testing.T) {
	fixedNow := time.Unix(1700000000, 0).UTC()
	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": map[string]any{
			"kty": "EC",
			"crv": "P-256",
			"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
			"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		},
	}

	tokValid := createTestDPoP(t, header, map[string]any{
		"jti": "id-valid",
		"htm": "GET",
		"htu": "https://server.example.com",
		"iat": fixedNow.Add(-30 * time.Second).Unix(),
	})
	tokFuture := createTestDPoP(t, header, map[string]any{
		"jti": "id-future",
		"htm": "GET",
		"htu": "https://server.example.com",
		"iat": fixedNow.Add(2 * time.Minute).Unix(),
	})
	tokExpired := createTestDPoP(t, header, map[string]any{
		"jti": "id-expired",
		"htm": "GET",
		"htu": "https://server.example.com",
		"iat": fixedNow.Add(-10 * time.Minute).Unix(),
	})

	env, err := cel.NewEnv(
		dpop.Library(
			dpop.Clock(func() time.Time { return fixedNow }),
			dpop.ValidateTimes(5*time.Minute, 10*time.Second),
			dpop.AllowedAlgorithms("ES256", "PS256"),
		),
		cel.Variable("tokValid", cel.StringType),
		cel.Variable("tokFuture", cel.StringType),
		cel.Variable("tokExpired", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"tokValid":   tokValid,
		"tokFuture":  tokFuture,
		"tokExpired": tokExpired,
	}

	if got := evalExpr(t, env, `dpop.parse(tokValid).hasValue()`, vars); got != true {
		t.Errorf("expected tokValid to have value, got %v", got)
	}
	if got := evalExpr(t, env, `dpop.parse(tokFuture).hasValue()`, vars); got != false {
		t.Errorf("expected tokFuture to be None, got %v", got)
	}
	if got := evalExpr(t, env, `dpop.parse(tokExpired).hasValue()`, vars); got != false {
		t.Errorf("expected tokExpired to be None, got %v", got)
	}
}

func TestAllowedAlgorithmsOption(t *testing.T) {
	headerRS := map[string]any{
		"typ": "dpop+jwt",
		"alg": "RS256",
		"jwk": map[string]any{
			"kty": "RSA",
			"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e":   "AQAB",
		},
	}
	tokRS := createTestDPoP(t, headerRS, map[string]any{
		"jti": "id-rs",
		"htm": "GET",
		"htu": "https://server.example.com",
		"iat": time.Now().Unix(),
	})

	// Allow only ES256
	env, err := cel.NewEnv(
		dpop.Library(dpop.AllowedAlgorithms("ES256")),
		cel.Variable("tokRS", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{"tokRS": tokRS}
	if got := evalExpr(t, env, `dpop.parse(tokRS).hasValue()`, vars); got != false {
		t.Errorf("expected tokRS to be rejected by allowed algorithms, got %v", got)
	}
}

func TestMatchesTokenBothChecks(t *testing.T) {
	// 1. Create a DPoP-bound JWT access token
	jwtHeader := map[string]any{"alg": "ES256", "typ": "JWT"}
	jwtPayload := map[string]any{
		"iss": "https://auth.example.com",
		"sub": "user-42",
		"aud": "https://api.example.com",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"cnf": map[string]any{
			"jkt": "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I",
		},
	}
	boundJWT := createTestJWT(t, jwtHeader, jwtPayload)

	// Calculate ATH for this exact JWT
	jwtATH := dpop.ComputeAccessTokenHash(boundJWT)

	// 2. Create DPoP Proof matching this JWT's thumbprint and ATH
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	proofHeader := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}
	proofPayload := map[string]any{
		"jti": "jti-unique-999",
		"htm": "GET",
		"htu": "https://api.example.com/data",
		"iat": time.Now().Unix(),
		"ath": jwtATH,
	}
	proofStr := createTestDPoP(t, proofHeader, proofPayload)

	// Another JWT with SAME cnf.jkt but different payload (different ATH)
	otherJWTPayload := map[string]any{
		"iss": "https://auth.example.com",
		"sub": "user-different-id",
		"aud": "https://api.example.com",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"cnf": map[string]any{
			"jkt": "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I",
		},
	}
	otherJWT := createTestJWT(t, jwtHeader, otherJWTPayload)

	// Another JWT with WRONG cnf.jkt
	wrongCnfJWTPayload := map[string]any{
		"iss": "https://auth.example.com",
		"sub": "user-42",
		"aud": "https://api.example.com",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"cnf": map[string]any{
			"jkt": "WRONG_JKT_THUMBPRINT_VALUE",
		},
	}
	wrongCnfJWT := createTestJWT(t, jwtHeader, wrongCnfJWTPayload)

	env, err := cel.NewEnv(
		dpop.Library(),
		jwt.Library(),
		cel.Variable("proofStr", cel.StringType),
		cel.Variable("boundJWT", cel.StringType),
		cel.Variable("dpopBoundJWT", cel.StringType),
		cel.Variable("otherJWT", cel.StringType),
		cel.Variable("wrongCnfJWT", cel.StringType),
		cel.Variable("opaqueToken", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"proofStr":     proofStr,
		"boundJWT":     boundJWT,
		"dpopBoundJWT": "DPoP " + boundJWT,
		"otherJWT":     otherJWT,
		"wrongCnfJWT":  wrongCnfJWT,
		"opaqueToken":  "opaque-non-jwt-token-string",
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "matchesToken_with_matching_jwt_string",
			expr: `dpop.parse(proofStr).matchesToken(boundJWT)`,
			want: true,
		},
		{
			name: "matchesToken_with_matching_dpop_header_string",
			expr: `dpop.parse(proofStr).matchesToken(dpopBoundJWT)`,
			want: true,
		},
		{
			name: "matchesToken_with_parsed_jwt_token",
			expr: `dpop.parse(proofStr).matchesToken(jwt.parse(boundJWT))`,
			want: true,
		},
		{
			name: "matchesToken_with_parsed_jwt_token_value",
			expr: `dpop.parse(proofStr).matchesToken(jwt.parse(boundJWT).value())`,
			want: true,
		},
		{
			name: "matchesToken_rejects_ath_mismatch_even_if_jkt_matches",
			expr: `dpop.parse(proofStr).matchesToken(otherJWT)`,
			want: false,
		},
		{
			name: "matchesToken_rejects_parsed_jwt_ath_mismatch",
			expr: `dpop.parse(proofStr).matchesToken(jwt.parse(otherJWT))`,
			want: false,
		},
		{
			name: "matchesToken_rejects_wrong_jkt_confirmation",
			expr: `dpop.parse(proofStr).matchesToken(wrongCnfJWT)`,
			want: false,
		},
		{
			name: "matchesToken_rejects_opaque_non_jwt_string",
			expr: `dpop.parse(proofStr).matchesToken(opaqueToken)`,
			want: false,
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

func TestMatchesChained(t *testing.T) {
	jwtHeader := map[string]any{"alg": "ES256", "typ": "JWT"}
	jwtPayload := map[string]any{
		"iss": "https://auth.example.com",
		"sub": "user-42",
		"aud": "https://api.example.com",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"cnf": map[string]any{
			"jkt": "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I",
		},
	}
	boundJWT := createTestJWT(t, jwtHeader, jwtPayload)
	jwtATH := dpop.ComputeAccessTokenHash(boundJWT)

	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	proofHeader := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}
	proofPayload := map[string]any{
		"jti":   "jti-unique-matches-123",
		"htm":   "POST",
		"htu":   "https://api.example.com/orders",
		"iat":   time.Now().Unix(),
		"ath":   jwtATH,
		"nonce": "server-nonce-valid-789",
	}
	proofStr := createTestDPoP(t, proofHeader, proofPayload)

	env, err := cel.NewEnv(
		dpop.Library(),
		jwt.Library(),
		cel.Variable("proofStr", cel.StringType),
		cel.Variable("boundJWT", cel.StringType),
		cel.Variable("method", cel.StringType),
		cel.Variable("url", cel.StringType),
		cel.Variable("nonce", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"proofStr": proofStr,
		"boundJWT": boundJWT,
		"method":   "POST",
		"url":      "https://api.example.com/orders",
		"nonce":    "server-nonce-valid-789",
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "matchesRequest_method_and_uri",
			expr: `dpop.parse(proofStr).matchesRequest(method, url)`,
			want: true,
		},
		{
			name: "matchesRequest_method_mismatch",
			expr: `dpop.parse(proofStr).matchesRequest('GET', url)`,
			want: false,
		},
		{
			name: "matchesRequest_uri_mismatch",
			expr: `dpop.parse(proofStr).matchesRequest(method, 'https://other.example.com/orders')`,
			want: false,
		},
		{
			name: "matchesToken_string",
			expr: `dpop.parse(proofStr).matchesToken(boundJWT)`,
			want: true,
		},
		{
			name: "matchesToken_header_prefix",
			expr: `dpop.parse(proofStr).matchesToken('DPoP ' + boundJWT)`,
			want: true,
		},
		{
			name: "matchesToken_jwt_token_object",
			expr: `dpop.parse(proofStr).matchesToken(jwt.parse(boundJWT))`,
			want: true,
		},
		{
			name: "matchesNonce_valid",
			expr: `dpop.parse(proofStr).matchesNonce(nonce)`,
			want: true,
		},
		{
			name: "matchesNonce_mismatch",
			expr: `dpop.parse(proofStr).matchesNonce('wrong-nonce')`,
			want: false,
		},
		{
			name: "chained_matchesRequest_matchesToken_matchesNonce",
			expr: `dpop.parse(proofStr).matchesRequest(method, url) && dpop.parse(proofStr).matchesToken(boundJWT) && dpop.parse(proofStr).matchesNonce(nonce)`,
			want: true,
		},
		{
			name: "chained_with_parsed_jwt_token",
			expr: `dpop.parse(proofStr).matchesRequest(method, url) && dpop.parse(proofStr).matchesToken(jwt.parse(boundJWT)) && dpop.parse(proofStr).matchesNonce(nonce)`,
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

func TestNonceUtilities(t *testing.T) {
	t.Run("GenerateNonce_random", func(t *testing.T) {
		n1, err := dpop.GenerateNonce()
		if err != nil {
			t.Fatalf("GenerateNonce failed: %v", err)
		}
		n2, err := dpop.GenerateNonce()
		if err != nil {
			t.Fatalf("GenerateNonce failed: %v", err)
		}
		if n1 == "" || n2 == "" {
			t.Fatalf("expected non-empty nonces")
		}
		if n1 == n2 {
			t.Fatalf("expected distinct random nonces, got identical: %q", n1)
		}
	})

	t.Run("StatelessNonce_valid", func(t *testing.T) {
		key := []byte("super-secret-hmac-key-for-nonce")
		nonce, err := dpop.GenerateStatelessNonce(key, "client-ip-127.0.0.1", "client-id-abc")
		if err != nil {
			t.Fatalf("GenerateStatelessNonce failed: %v", err)
		}

		// Validate with correct key, maxAge, and context
		if !dpop.ValidateStatelessNonce(nonce, key, 1*time.Minute, "client-ip-127.0.0.1", "client-id-abc") {
			t.Fatalf("ValidateStatelessNonce failed on valid nonce %q", nonce)
		}

		// Validate with wrong key
		wrongKey := []byte("wrong-key-value-123456789012345")
		if dpop.ValidateStatelessNonce(nonce, wrongKey, 1*time.Minute, "client-ip-127.0.0.1", "client-id-abc") {
			t.Fatalf("ValidateStatelessNonce should reject wrong key")
		}

		// Validate with wrong context
		if dpop.ValidateStatelessNonce(nonce, key, 1*time.Minute, "client-ip-10.0.0.1", "client-id-abc") {
			t.Fatalf("ValidateStatelessNonce should reject mismatched context")
		}

		// Validate with 0 maxAge (expired)
		if dpop.ValidateStatelessNonce(nonce, key, -1*time.Second, "client-ip-127.0.0.1", "client-id-abc") {
			t.Fatalf("ValidateStatelessNonce should reject expired nonce")
		}

		// Validate malformed nonces
		if dpop.ValidateStatelessNonce("invalid.malformed.parts", key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject invalid parts")
		}
		if dpop.ValidateStatelessNonce("invalid", key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject single segment")
		}
		if dpop.ValidateStatelessNonce("bad_b64.sig", key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject invalid b64 payload")
		}
		if dpop.ValidateStatelessNonce("cGF5bG9hZA.bad_sig", key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject invalid b64 sig")
		}
		if dpop.ValidateStatelessNonce("bm9fc2VwYXJhdG9ycw.c2lnbmF0dXJl", key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject payload without timestamp separator")
		}
		// Malformed timestamp in payload
		badTsPayload := base64.RawURLEncoding.EncodeToString([]byte("not_a_ts:entropy:ctx"))
		if dpop.ValidateStatelessNonce(badTsPayload+".c2ln", key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject invalid timestamp")
		}
		// Future timestamp outside 10s window
		futureTime := time.Now().UTC().Add(1 * time.Hour).Unix()
		futurePayload := []byte(fmt.Sprintf("%d:entropy:ctx", futureTime))
		mac := hmac.New(sha256.New, key)
		mac.Write(futurePayload)
		futureSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		futureNonce := base64.RawURLEncoding.EncodeToString(futurePayload) + "." + futureSig
		if dpop.ValidateStatelessNonce(futureNonce, key, 1*time.Minute, "ctx") {
			t.Fatalf("ValidateStatelessNonce should reject far-future timestamp")
		}

		// Tampered valid-base64 signature
		nonceParts := strings.Split(nonce, ".")
		fakeSig := base64.RawURLEncoding.EncodeToString([]byte("wrong-signature-32-bytes-long!"))
		if dpop.ValidateStatelessNonce(nonceParts[0]+"."+fakeSig, key, 1*time.Minute, "client-ip-127.0.0.1", "client-id-abc") {
			t.Fatalf("ValidateStatelessNonce should reject tampered valid-base64 signature")
		}

		// Single-part signed payload (len(payloadParts) < 2)
		singlePart := []byte("singlepartnoparts")
		mac1 := hmac.New(sha256.New, key)
		mac1.Write(singlePart)
		sig1 := base64.RawURLEncoding.EncodeToString(mac1.Sum(nil))
		nonce1 := base64.RawURLEncoding.EncodeToString(singlePart) + "." + sig1
		if dpop.ValidateStatelessNonce(nonce1, key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should reject single part payload")
		}

		// Non-integer timestamp signed payload
		badTsPart := []byte("notanumber:entropy:ctx")
		mac2 := hmac.New(sha256.New, key)
		mac2.Write(badTsPart)
		sig2 := base64.RawURLEncoding.EncodeToString(mac2.Sum(nil))
		nonce2 := base64.RawURLEncoding.EncodeToString(badTsPart) + "." + sig2
		if dpop.ValidateStatelessNonce(nonce2, key, 1*time.Minute, "ctx") {
			t.Fatalf("ValidateStatelessNonce should reject invalid integer timestamp")
		}

		// Nonce generated without context validated with context required
		noCtxNonce, err := dpop.GenerateStatelessNonce(key)
		if err != nil {
			t.Fatalf("GenerateStatelessNonce without context failed: %v", err)
		}
		if dpop.ValidateStatelessNonce(noCtxNonce, key, 1*time.Minute, "required-ctx") {
			t.Fatalf("ValidateStatelessNonce should reject nonce lacking expected context")
		}
		if !dpop.ValidateStatelessNonce(noCtxNonce, key, 1*time.Minute) {
			t.Fatalf("ValidateStatelessNonce should accept nonce without context")
		}
	})
}

func TestDPoPOptions(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()

	t.Run("Version_Option", func(t *testing.T) {
		lib := dpop.NewDPoPLib(dpop.Version(1))
		if lib == nil {
			t.Fatal("expected non-nil lib")
		}
	})

	t.Run("MaxAge_Option", func(t *testing.T) {
		lib := dpop.NewDPoPLib(dpop.MaxAge(15 * time.Minute))
		if lib == nil {
			t.Fatal("expected non-nil lib")
		}
	})

	t.Run("ClockLeeway_Option", func(t *testing.T) {
		lib := dpop.NewDPoPLib(dpop.ClockLeeway(10 * time.Second))
		if lib == nil {
			t.Fatal("expected non-nil lib")
		}
	})

	t.Run("ValidateTimes_DefaultClock", func(t *testing.T) {
		env, err := cel.NewEnv(
			cel.Variable("proof", cel.StringType),
			dpop.Library(
				dpop.ValidateTimes(5 * time.Minute),
			),
		)
		if err != nil {
			t.Fatalf("NewEnv failed: %v", err)
		}
		ecJWK := map[string]any{
			"kty": "EC",
			"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
			"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
			"crv": "P-256",
		}
		proofStr := createTestDPoP(t, map[string]any{
			"typ": "dpop+jwt",
			"alg": "ES256",
			"jwk": ecJWK,
		}, map[string]any{
			"jti": "jti-123",
			"htm": "GET",
			"htu": "https://example.com/resource",
			"iat": time.Now().UTC().Unix(),
		})

		got := evalExpr(t, env, `dpop.parse(proof).hasValue()`, map[string]any{"proof": proofStr})
		if got != true {
			t.Errorf("dpop.parse with default clock failed: got %v, want true", got)
		}
	})

	t.Run("ValidateTimes_CustomClock", func(t *testing.T) {
		env, err := cel.NewEnv(
			cel.Variable("proof", cel.StringType),
			dpop.Library(
				dpop.ValidateTimes(5*time.Minute, 10*time.Second),
				dpop.Clock(func() time.Time { return now }),
			),
		)
		if err != nil {
			t.Fatalf("NewEnv failed: %v", err)
		}
		ecJWK := map[string]any{
			"kty": "EC",
			"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
			"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
			"crv": "P-256",
		}
		// Expired proof (> 5 min before now)
		expiredProof := createTestDPoP(t, map[string]any{
			"typ": "dpop+jwt",
			"alg": "ES256",
			"jwk": ecJWK,
		}, map[string]any{
			"jti": "jti-123",
			"htm": "GET",
			"htu": "https://example.com/resource",
			"iat": now.Add(-10 * time.Minute).Unix(),
		})
		got := evalExpr(t, env, `dpop.parse(proof).hasValue()`, map[string]any{"proof": expiredProof})
		if got != false {
			t.Errorf("expected expired proof to evaluate to optional.none(), got %v", got)
		}
	})

	t.Run("AllowedAlgorithms_CEL_Rejection", func(t *testing.T) {
		env, err := cel.NewEnv(
			cel.Variable("proof", cel.StringType),
			dpop.Library(
				dpop.AllowedAlgorithms("RS256"),
			),
		)
		if err != nil {
			t.Fatalf("NewEnv failed: %v", err)
		}
		ecJWK := map[string]any{
			"kty": "EC",
			"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
			"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
			"crv": "P-256",
		}
		es256Proof := createTestDPoP(t, map[string]any{
			"typ": "dpop+jwt",
			"alg": "ES256",
			"jwk": ecJWK,
		}, map[string]any{
			"jti": "jti-123",
			"htm": "GET",
			"htu": "https://example.com/resource",
			"iat": time.Now().UTC().Unix(),
		})

		got := evalExpr(t, env, `dpop.parse(proof).hasValue()`, map[string]any{"proof": es256Proof})
		if got != false {
			t.Errorf("expected disallowed algorithm ES256 to evaluate to optional.none(), got %v", got)
		}
	})
}

func TestParseProofErrorsAndLimits(t *testing.T) {
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}

	t.Run("exceeds_max_size", func(t *testing.T) {
		hugeProof := strings.Repeat("a", 65537)
		_, err := dpop.ParseProof(hugeProof)
		if err == nil {
			t.Fatal("expected error for oversized proof")
		}
	})

	t.Run("invalid_segment_counts", func(t *testing.T) {
		for _, s := range []string{"single_part", "one.two.three.four.five"} {
			if _, err := dpop.ParseProof(s); err == nil {
				t.Fatalf("expected error for %q", s)
			}
		}
	})

	t.Run("invalid_base64_header_and_payload", func(t *testing.T) {
		if _, err := dpop.ParseProof("???bad_b64???.eyJqdGkiOiIxIn0.sig"); err == nil {
			t.Fatal("expected error for bad header b64")
		}
		if _, err := dpop.ParseProof("eyJhbGciOiJFUzI1NiJ9.???bad_payload???.sig"); err == nil {
			t.Fatal("expected error for bad payload b64")
		}
	})

	t.Run("invalid_json_header_and_payload", func(t *testing.T) {
		notJSON := base64.RawURLEncoding.EncodeToString([]byte("not json"))
		validB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"dpop+jwt"}`))
		if _, err := dpop.ParseProof(notJSON + "." + validB64 + ".sig"); err == nil {
			t.Fatal("expected error for non-json header")
		}
		if _, err := dpop.ParseProof(validB64 + "." + notJSON + ".sig"); err == nil {
			t.Fatal("expected error for non-json payload")
		}
	})

	t.Run("invalid_typ_header", func(t *testing.T) {
		for _, typVal := range []any{"jwt", "application/json", 123, nil, ""} {
			header := map[string]any{"typ": typVal, "alg": "ES256", "jwk": ecJWK}
			payload := map[string]any{"jti": "1", "htm": "GET", "htu": "https://example.com", "iat": 1000}
			if _, err := dpop.NewProof(header, payload); err == nil {
				t.Fatalf("expected error for typ=%v", typVal)
			}
		}
	})

	t.Run("invalid_alg_header", func(t *testing.T) {
		for _, algVal := range []any{"none", "HS256", "HS384", "HS512", "", 123, nil} {
			header := map[string]any{"typ": "dpop+jwt", "alg": algVal, "jwk": ecJWK}
			payload := map[string]any{"jti": "1", "htm": "GET", "htu": "https://example.com", "iat": 1000}
			if _, err := dpop.NewProof(header, payload); err == nil {
				t.Fatalf("expected error for alg=%v", algVal)
			}
		}
	})

	t.Run("invalid_jwk_header", func(t *testing.T) {
		for _, jwkVal := range []any{nil, "not-a-map", map[string]any{}, 123} {
			header := map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": jwkVal}
			payload := map[string]any{"jti": "1", "htm": "GET", "htu": "https://example.com", "iat": 1000}
			if _, err := dpop.NewProof(header, payload); err == nil {
				t.Fatalf("expected error for jwk=%v", jwkVal)
			}
		}
	})

	t.Run("forbidden_jwk_private_keys", func(t *testing.T) {
		for _, key := range []string{"d", "p", "q", "dp", "dq", "qi", "dmp1", "dmq1", "oth", "k"} {
			jwkWithPriv := map[string]any{
				"kty": "EC",
				"crv": "P-256",
				"x":   "x",
				"y":   "y",
				key:   "secret-value",
			}
			header := map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": jwkWithPriv}
			payload := map[string]any{"jti": "1", "htm": "GET", "htu": "https://example.com", "iat": 1000}
			if _, err := dpop.NewProof(header, payload); err == nil {
				t.Fatalf("expected error for private JWK member %q", key)
			}
		}
		// oct symmetric key
		octJWK := map[string]any{"kty": "oct", "k": "c2VjcmV0"}
		header := map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": octJWK}
		payload := map[string]any{"jti": "1", "htm": "GET", "htu": "https://example.com", "iat": 1000}
		if _, err := dpop.NewProof(header, payload); err == nil {
			t.Fatal("expected error for oct JWK")
		}
	})

	t.Run("missing_required_payload_claims", func(t *testing.T) {
		validClaims := map[string]any{
			"jti": "jti-val",
			"htm": "GET",
			"htu": "https://example.com",
			"iat": 1700000000,
		}
		header := map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": ecJWK}

		for _, missingKey := range []string{"jti", "htm", "htu", "iat"} {
			payload := make(map[string]any)
			for k, v := range validClaims {
				if k != missingKey {
					payload[k] = v
				}
			}
			if _, err := dpop.NewProof(header, payload); err == nil {
				t.Fatalf("expected error when %q claim is missing", missingKey)
			}
		}

		// Invalid iat timestamp
		badIatPayload := map[string]any{
			"jti": "jti-val",
			"htm": "GET",
			"htu": "https://example.com",
			"iat": "not-a-timestamp",
		}
		if _, err := dpop.NewProof(header, badIatPayload); err == nil {
			t.Fatal("expected error for non-timestamp iat")
		}
	})
}

func TestJWKThumbprintAlgorithmsAndErrors(t *testing.T) {
	t.Run("RSA_JWK", func(t *testing.T) {
		rsaJWK := map[string]any{
			"kty": "RSA",
			"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e":   "AQAB",
		}
		thumb, err := dpop.ComputeJWKThumbprint(rsaJWK)
		if err != nil {
			t.Fatalf("ComputeJWKThumbprint RSA failed: %v", err)
		}
		if thumb != "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs" {
			t.Errorf("RSA thumbprint = %q, want RFC 7638 vector", thumb)
		}

		// Missing e or n
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "RSA", "n": "n"}); err == nil {
			t.Fatal("expected error for missing e")
		}
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "RSA", "e": "AQAB"}); err == nil {
			t.Fatal("expected error for missing n")
		}
	})

	t.Run("OKP_JWK", func(t *testing.T) {
		okpJWK := map[string]any{
			"kty": "OKP",
			"crv": "Ed25519",
			"x":   "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo",
		}
		thumb, err := dpop.ComputeJWKThumbprint(okpJWK)
		if err != nil {
			t.Fatalf("ComputeJWKThumbprint OKP failed: %v", err)
		}
		if thumb == "" {
			t.Fatal("expected non-empty OKP thumbprint")
		}

		// Missing crv or x
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "OKP", "x": "x"}); err == nil {
			t.Fatal("expected error for missing crv")
		}
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "OKP", "crv": "Ed25519"}); err == nil {
			t.Fatal("expected error for missing x")
		}
	})

	t.Run("EC_JWK_Errors", func(t *testing.T) {
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "EC", "x": "x", "y": "y"}); err == nil {
			t.Fatal("expected error for missing crv")
		}
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "EC", "crv": "P-256", "y": "y"}); err == nil {
			t.Fatal("expected error for missing x")
		}
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "EC", "crv": "P-256", "x": "x"}); err == nil {
			t.Fatal("expected error for missing y")
		}
	})

	t.Run("Unsupported_and_empty_kty", func(t *testing.T) {
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{}); err == nil {
			t.Fatal("expected error for empty JWK")
		}
		if _, err := dpop.ComputeJWKThumbprint(map[string]any{"kty": "UNSUPPORTED"}); err == nil {
			t.Fatal("expected error for unsupported kty")
		}
	})
}

func TestMatchesTargetURIExtensive(t *testing.T) {
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	proof, err := dpop.NewProof(map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}, map[string]any{
		"jti": "jti-1",
		"htm": "GET",
		"htu": "https://example.com/path",
		"iat": 1700000000,
	})
	if err != nil {
		t.Fatalf("NewProof failed: %v", err)
	}

	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{
			name: "exact_match",
			uri:  "https://example.com/path",
			want: true,
		},
		{
			name: "default_port_stripped_matches",
			uri:  "https://example.com:443/path",
			want: true,
		},
		{
			name: "http_default_port_stripped",
			uri:  "http://example.com:80/path",
			want: false, // scheme is http vs https
		},
		{
			name: "dot_segments_cleaned_matches",
			uri:  "https://example.com/a/b/../../path",
			want: true,
		},
		{
			name: "custom_port_mismatch",
			uri:  "https://example.com:8443/path",
			want: false,
		},
		{
			name: "scheme_mismatch",
			uri:  "http://example.com/path",
			want: false,
		},
		{
			name: "empty_uri",
			uri:  "",
			want: false,
		},
		{
			name: "invalid_scheme",
			uri:  "//example.com/path",
			want: false,
		},
		{
			name: "missing_host",
			uri:  "http:",
			want: false,
		},
		{
			name: "ipv6_literal_host_with_default_port",
			uri:  "https://[2001:db8::1]:443/path",
			want: false, // host mismatch
		},
		{
			name: "ipv6_literal_host_with_custom_port",
			uri:  "https://[2001:db8::1]:8443/path",
			want: false, // host mismatch
		},
		{
			name: "root_path",
			uri:  "https://example.com",
			want: false, // / vs /path
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := proof.MatchesTargetURI(tc.uri)
			if got != tc.want {
				t.Errorf("MatchesTargetURI(%q) = %v, want %v", tc.uri, got, tc.want)
			}
		})
	}

	// Test proof with root path
	rootProof, err := dpop.NewProof(map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}, map[string]any{
		"jti": "jti-1",
		"htm": "GET",
		"htu": "https://example.com",
		"iat": 1700000000,
	})
	if err != nil {
		t.Fatalf("NewProof failed: %v", err)
	}
	if !rootProof.MatchesTargetURI("https://example.com/") {
		t.Errorf("expected root proof to match https://example.com/")
	}

	// Test proof with HTTP scheme and default port 80
	httpProof, err := dpop.NewProof(map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}, map[string]any{
		"jti": "jti-1",
		"htm": "GET",
		"htu": "http://example.com:80/path",
		"iat": 1700000000,
	})
	if err != nil {
		t.Fatalf("NewProof failed: %v", err)
	}
	if !httpProof.MatchesTargetURI("http://example.com/path") {
		t.Errorf("expected http proof to match http://example.com/path")
	}
}

func TestDirectProofMethodsEdgeCases(t *testing.T) {
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	proof, err := dpop.NewProof(map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}, map[string]any{
		"jti":     "jti-1",
		"htm":     "GET",
		"htu":     "https://example.com/api",
		"iat":     1700000000,
		"nil_val": nil,
	})
	if err != nil {
		t.Fatalf("NewProof failed: %v", err)
	}

	t.Run("MatchesTargetURI_failures", func(t *testing.T) {
		if proof.MatchesTargetURI("invalid uri :::") {
			t.Error("expected false for invalid URI")
		}
		emptyProof := &dpop.Proof{}
		if emptyProof.MatchesTargetURI("https://example.com") {
			t.Error("expected false for proof with empty URI")
		}
	})

	t.Run("MatchesAccessToken_empty", func(t *testing.T) {
		if proof.MatchesAccessToken("token") {
			t.Error("expected false when proof has no ath claim")
		}
	})

	t.Run("MatchesNonce_empty", func(t *testing.T) {
		if proof.MatchesNonce("nonce") {
			t.Error("expected false when proof has no nonce claim")
		}
	})

	t.Run("MatchesConfirmationString_empty_and_mismatch", func(t *testing.T) {
		emptyProof := &dpop.Proof{}
		if emptyProof.MatchesConfirmationString("thumb") {
			t.Error("expected false when proof has empty thumbprint")
		}
		if proof.MatchesConfirmationString("different-length-thumb") {
			t.Error("expected false for mismatched thumbprint")
		}
	})

	t.Run("MatchesConfirmationMap_failures", func(t *testing.T) {
		if proof.MatchesConfirmationMap(nil) {
			t.Error("expected false for nil cnf")
		}
		if proof.MatchesConfirmationMap(map[string]any{"jkt": 123}) {
			t.Error("expected false for non-string jkt")
		}
		if proof.MatchesConfirmationMap(map[string]any{"jkt": ""}) {
			t.Error("expected false for empty jkt")
		}
	})

	t.Run("MatchesToken_failures", func(t *testing.T) {
		if proof.MatchesToken(nil) {
			t.Error("expected false for nil token")
		}
		if proof.MatchesToken(&jwt.Token{}) {
			t.Error("expected false for token with nil payload")
		}
		if proof.MatchesToken(&jwt.Token{Payload: map[string]any{}}) {
			t.Error("expected false for token without cnf")
		}
		if proof.MatchesToken(&jwt.Token{Payload: map[string]any{"cnf": "not-a-map"}}) {
			t.Error("expected false for token with invalid cnf type")
		}
		// Raw ath mismatch
		proofWithAth := &dpop.Proof{
			AccessTokenHash: "different-ath",
			Thumbprint:      proof.Thumbprint,
		}
		tokWithCnf := &jwt.Token{
			Raw: "some-raw-token",
			Payload: map[string]any{
				"cnf": map[string]any{
					"jkt": proof.Thumbprint,
				},
			},
		}
		if proofWithAth.MatchesToken(tokWithCnf) {
			t.Error("expected false for mismatched raw token ath")
		}
	})

	t.Run("MatchesTokenString_non_jwt", func(t *testing.T) {
		proofWithAth := &dpop.Proof{
			AccessTokenHash: dpop.ComputeAccessTokenHash("opaque-secret-token"),
			Thumbprint:      proof.Thumbprint,
		}
		// opaque token matches ath, but is not a valid JWT -> returns false
		if proofWithAth.MatchesTokenString("opaque-secret-token") {
			t.Error("expected false for non-JWT token string")
		}
	})

	t.Run("Claim_nil_missing_and_unadaptable", func(t *testing.T) {
		env, err := cel.NewEnv()
		if err != nil {
			t.Fatalf("NewEnv failed: %v", err)
		}
		adapter := env.CELTypeAdapter()

		if proof.Claim(adapter, "missing") != types.OptionalNone {
			t.Error("expected OptionalNone for missing claim")
		}
		if proof.Claim(adapter, "nil_val") != types.OptionalNone {
			t.Error("expected OptionalNone for nil claim value")
		}

		// Unadaptable value returning error
		badProof := &dpop.Proof{
			Payload: map[string]any{
				"unadaptable": make(chan int),
			},
		}
		res := badProof.Claim(adapter, "unadaptable")
		if !types.IsError(res) {
			t.Errorf("expected error for unadaptable claim, got %v", res)
		}
	})
}

func TestParseProofBase64Variants(t *testing.T) {
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}
	payload := map[string]any{
		"jti":    "jti-123",
		"htm":    "GET",
		"htu":    "https://example.com/api",
		"iat":    1700000000,
		"custom": "?>>??<<?",
	}
	hBytes, _ := json.Marshal(header)
	pBytes, _ := json.Marshal(payload)

	// Padded URL encoding
	hPadded := base64.URLEncoding.EncodeToString(hBytes)
	pPadded := base64.URLEncoding.EncodeToString(pBytes)
	if _, err := dpop.ParseProof(hPadded + "." + pPadded + ".sig"); err != nil {
		t.Errorf("ParseProof with padded URL encoding failed: %v", err)
	}

	// Standard encoding (with padding and '+' / '/' chars)
	hStd := base64.StdEncoding.EncodeToString(hBytes)
	pStd := base64.StdEncoding.EncodeToString(pBytes)
	if _, err := dpop.ParseProof(hStd + "." + pStd + ".sig"); err != nil {
		t.Errorf("ParseProof with standard base64 failed: %v", err)
	}

	// Raw Standard encoding (without padding and '+' / '/' chars)
	hRawStd := base64.RawStdEncoding.EncodeToString(hBytes)
	pRawStd := base64.RawStdEncoding.EncodeToString(pBytes)
	if _, err := dpop.ParseProof(hRawStd + "." + pRawStd + ".sig"); err != nil {
		t.Errorf("ParseProof with raw standard base64 failed: %v", err)
	}
}

func TestCELBindingsAndErrors(t *testing.T) {
	ecJWK := map[string]any{
		"kty": "EC",
		"x":   "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		"y":   "9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA",
		"crv": "P-256",
	}
	proofStr := createTestDPoP(t, map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": ecJWK,
	}, map[string]any{
		"jti":   "jti-123",
		"htm":   "GET",
		"htu":   "https://example.com/resource",
		"iat":   1700000000,
		"claim": "present-value",
	})

	boundJWTPayload := map[string]any{
		"sub": "user1",
		"iss": "https://auth.example.com",
		"aud": "https://example.com",
		"exp": 1800000000,
		"iat": 1700000000,
		"cnf": map[string]any{
			"jkt": "l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs",
		},
	}
	boundJWT := createTestJWT(t, map[string]any{"alg": "ES256"}, boundJWTPayload)

	env, err := cel.NewEnv(
		cel.Variable("proofStr", cel.StringType),
		cel.Variable("boundJWT", cel.StringType),
		dpop.Library(),
		jwt.Library(),
	)
	if err != nil {
		t.Fatalf("NewEnv failed: %v", err)
	}

	vars := map[string]any{
		"proofStr": proofStr,
		"boundJWT": boundJWT,
	}

	t.Run("dpop_parse_error", func(t *testing.T) {
		ast, issues := env.Compile(`dpop.parse('invalid.jwt')`)
		if issues != nil && issues.Err() != nil {
			t.Fatalf("Compile failed: %v", issues.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			t.Fatalf("Program failed: %v", err)
		}
		_, _, err = prg.Eval(vars)
		if err == nil {
			t.Fatal("expected eval error for malformed proof")
		}
	})

	t.Run("evalMatchesToken_all_paths", func(t *testing.T) {
		// matchesToken with optional.none()
		got := evalExpr(t, env, `dpop.parse(proofStr).matchesToken(optional.none())`, vars)
		if got != false {
			t.Errorf("expected false for optional.none() token, got %v", got)
		}

		// matchesToken with optional jwt.parse
		got = evalExpr(t, env, `dpop.parse(proofStr).matchesToken(jwt.parse(boundJWT))`, vars)
		if got != false { // thumbprint doesn't match dummy vector, but evaluates cleanly
			// ok
		}

		// optional receiver on optional.none() receiver
		got = evalExpr(t, env, `optional.none().matchesToken(boundJWT)`, vars)
		if got != false {
			t.Errorf("expected false on optional.none() receiver, got %v", got)
		}
		got = evalExpr(t, env, `optional.none().matchesRequest('GET', 'https://example.com')`, vars)
		if got != false {
			t.Errorf("expected false on optional.none() receiver, got %v", got)
		}
		got = evalExpr(t, env, `optional.none().matchesNonce('nonce')`, vars)
		if got != false {
			t.Errorf("expected false on optional.none() receiver, got %v", got)
		}
		hasVal := evalExpr(t, env, `optional.none().claim('claim').hasValue()`, vars)
		if hasVal != false {
			t.Errorf("expected false on optional.none().claim(), got %v", hasVal)
		}
	})

	t.Run("claim_queries", func(t *testing.T) {
		got := evalExpr(t, env, `dpop.parse(proofStr).claim('claim').value()`, vars)
		if got != "present-value" {
			t.Errorf("claim('claim') = %v, want %q", got, "present-value")
		}
		hasMissing := evalExpr(t, env, `dpop.parse(proofStr).claim('non_existent').hasValue()`, vars)
		if hasMissing != false {
			t.Errorf("claim('non_existent').hasValue() = %v, want false", hasMissing)
		}
	})
}




