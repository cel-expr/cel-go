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
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
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

	env, err := cel.NewEnv(
		dpop.Library(),
		jwt.Library(),
		cel.Variable("proofStr", cel.StringType),
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
			name: "proof_ath",
			expr: `dpop.parse(proofStr).value().ath == 'fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo'`,
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
			name: "dpop_ath_func",
			expr: `dpop.ath(tokenStr) == 'fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo'`,
			want: true,
		},
		{
			name: "dpop_ath_from_auth_header",
			expr: `dpop.ath(authHeader) == 'fUHyO2r2Z3DZ53EsNrWBb0xWXoaNy59IiKCAqksmQEo'`,
			want: true,
		},
		{
			name: "dpop_thumbprint_func",
			expr: `dpop.thumbprint(dpop.parse(proofStr).value().jwk) == '0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I'`,
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
			name: "matchesMethod_exact",
			expr: `dpop.parse(proofStr).matchesMethod('GET')`,
			want: true,
		},
		{
			name: "matchesMethod_case_insensitive",
			expr: `dpop.parse(proofStr).matchesMethod('get')`,
			want: true,
		},
		{
			name: "matchesMethod_mismatch",
			expr: `dpop.parse(proofStr).matchesMethod('POST')`,
			want: false,
		},
		{
			name: "matchesURI_exact",
			expr: `dpop.parse(proofStr).matchesURI('https://resource.example.org/protectedresource')`,
			want: true,
		},
		{
			name: "matchesHtu_alias",
			expr: `dpop.parse(proofStr).matchesHtu('https://resource.example.org/protectedresource')`,
			want: true,
		},
		{
			name: "matchesURI_with_default_port_and_case",
			expr: `dpop.parse(proofStr).matchesURI('HTTPS://RESOURCE.EXAMPLE.ORG:443/protectedresource')`,
			want: true,
		},
		{
			name: "matchesURI_ignores_query_and_fragment",
			expr: `dpop.parse(proofStr).matchesURI('https://resource.example.org/protectedresource?query=1#frag')`,
			want: true,
		},
		{
			name: "matchesURI_path_cleaning",
			expr: `dpop.parse(proofStr).matchesURI('https://resource.example.org/foo/../protectedresource')`,
			want: true,
		},
		{
			name: "matchesURI_mismatch",
			expr: `dpop.parse(proofStr).matchesURI('https://other.example.org/protectedresource')`,
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
			name: "matchesRequest_method_mismatch",
			expr: `dpop.parse(proofStr).matchesRequest('POST', 'https://resource.example.org/protectedresource')`,
			want: false,
		},
		{
			name: "matchesAccessToken_raw",
			expr: `dpop.parse(proofStr).matchesAccessToken(tokenStr)`,
			want: true,
		},
		{
			name: "matchesAccessToken_header",
			expr: `dpop.parse(proofStr).matchesAccessToken(authHeader)`,
			want: true,
		},
		{
			name: "matchesAccessToken_mismatch",
			expr: `dpop.parse(proofStr).matchesAccessToken('wrong-token')`,
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
			name: "matchesConfirmation_string",
			expr: `dpop.parse(proofStr).matchesConfirmation('0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I')`,
			want: true,
		},
		{
			name: "matchesConfirmation_map",
			expr: `dpop.parse(proofStr).matchesConfirmation({'jkt': '0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I'})`,
			want: true,
		},
		{
			name: "matchesToken_with_parsed_jwt",
			expr: `dpop.parse(proofStr).matchesToken(jwt.parse(jwtTokenStr))`,
			want: true,
		},
		{
			name: "matchesToken_with_parsed_jwt_value",
			expr: `dpop.parse(proofStr).matchesToken(jwt.parse(jwtTokenStr).value())`,
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
	// RFC 7638 Section 3.1 RSA thumbprint test vector: NzbLsHIexยอด...
	// NzbLsHIexUVQuOfNReIsTyXOvjX676Yu5_BpYZShqKE
	expectedRSA := "NzbLsHIexUVQuOfNReIsTyXOvjX676Yu5_BpYZShqKE"
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
