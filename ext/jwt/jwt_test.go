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

package jwt_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext/jwt"
)

func createTestHMACJWT(header, payload map[string]any, secret string) string {
	hBytes, _ := json.Marshal(header)
	pBytes, _ := json.Marshal(payload)

	hB64 := base64.RawURLEncoding.EncodeToString(hBytes)
	pB64 := base64.RawURLEncoding.EncodeToString(pBytes)

	signedData := hB64 + "." + pB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedData))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signedData + "." + sigB64
}

func createTestRSAJWT(header, payload map[string]any, privKey *rsa.PrivateKey) (string, string) {
	hBytes, _ := json.Marshal(header)
	pBytes, _ := json.Marshal(payload)

	hB64 := base64.RawURLEncoding.EncodeToString(hBytes)
	pB64 := base64.RawURLEncoding.EncodeToString(pBytes)

	signedData := hB64 + "." + pB64
	hashed := sha256.Sum256([]byte(signedData))
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hashed[:])
	if err != nil {
		panic(err)
	}
	sigB64 := base64.RawURLEncoding.EncodeToString(sigBytes)

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		panic(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return signedData + "." + sigB64, string(pubPEM)
}

func TestJWTVerifyAndPresentedBy(t *testing.T) {
	secret := "my-secret-key-123"
	payload := map[string]any{
		"iss":         "https://auth.example.com",
		"sub":         "user_987",
		"aud":         "my-api-audience",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"jti":         "token-id-001",
		"tenant_id":   "tenant-42",
		"user_role":   "admin",
		"is_verified": true,
	}
	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}

	tokStr := createTestHMACJWT(header, payload, secret)

	env, err := cel.NewEnv(
		jwt.Library(),
		cel.Variable("token", cel.StringType),
		cel.Variable("key", cel.StringType),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	tests := []struct {
		name string
		expr string
		vars map[string]any
		want any
	}{
		{
			name: "presentedBy_match",
			expr: `jwt.verifyWithKey(token, key).presentedBy('my-api-audience', 'https://auth.example.com')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: true,
		},
		{
			name: "optional_presentedBy_direct_match",
			expr: `jwt.verifyWithKey(token, key).presentedBy('my-api-audience', 'https://auth.example.com')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: true,
		},
		{
			name: "optional_presentedBy_direct_none",
			expr: `jwt.verifyWithKey(token, 'wrong-key').presentedBy('my-api-audience', 'https://auth.example.com')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: false,
		},
		{
			name: "presentedBy_mismatch_aud",
			expr: `jwt.verifyWithKey(token, key).presentedBy('wrong-audience', 'https://auth.example.com')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: false,
		},
		{
			name: "presentedBy_mismatch_iss",
			expr: `jwt.verifyWithKey(token, key).presentedBy('my-api-audience', 'https://wrong.com')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: false,
		},
		{
			name: "authorization_header_bearer",
			expr: `jwt.verifyWithKey(headers['authorization'], key).presentedBy('my-api-audience', 'https://auth.example.com')`,
			vars: map[string]any{
				"token": tokStr,
				"key":   secret,
				"headers": map[string]string{
					"authorization": "Bearer " + tokStr,
				},
			},
			want: true,
		},
		{
			name: "claim_present",
			expr: `jwt.verifyWithKey(token, key).value().claim('tenant_id').orValue('default')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: "tenant-42",
		},
		{
			name: "claim_missing",
			expr: `jwt.verifyWithKey(token, key).value().claim('non_existent').orValue('default')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: "default",
		},
		{
			name: "claim_hasValue",
			expr: `jwt.verifyWithKey(token, key).value().claim('user_role').hasValue()`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: true,
		},
		{
			name: "claim_boolean_to_string",
			expr: `jwt.verifyWithKey(token, key).value().claim('is_verified').orValue('false')`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: "true",
		},
		{
			name: "standard_claim_iss",
			expr: `jwt.verifyWithKey(token, key).value().issuer`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: "https://auth.example.com",
		},
		{
			name: "standard_claim_sub",
			expr: `jwt.verifyWithKey(token, key).value().subject`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: "user_987",
		},
		{
			name: "standard_claim_aud",
			expr: `jwt.verifyWithKey(token, key).value().aud`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: []string{"my-api-audience"},
		},
		{
			name: "verifyWithKey_hasValue",
			expr: `jwt.verifyWithKey(token, key).hasValue()`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: true,
		},
		{
			name: "verifyWithKey_invalid_hasValue",
			expr: `jwt.verifyWithKey(token, 'wrong-key').hasValue()`,
			vars: map[string]any{"token": tokStr, "key": secret, "headers": map[string]string{}},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ast, issues := env.Compile(tc.expr)
			if issues.Err() != nil {
				t.Fatalf("env.Compile(%q) failed: %v", tc.expr, issues.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program() failed: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			res := <-prg.ConcurrentEval(ctx, tc.vars)
			if res.Err != nil {
				t.Fatalf("prg.ConcurrentEval() failed for %q: %v", tc.expr, res.Err)
			}

			if got := res.Val.Value(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
			}
		})
	}
}

func TestJWTRSASignature(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	payload := map[string]any{
		"iss": "https://rsa-issuer.org",
		"aud": "rsa-audience",
		"sub": "subject_rsa",
	}
	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}

	tokStr, pubPEM := createTestRSAJWT(header, payload, privKey)

	env, err := cel.NewEnv(
		jwt.Library(),
		cel.Variable("token", cel.StringType),
		cel.Variable("pubkey", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	ast, issues := env.Compile(`jwt.verifyWithKey(token, pubkey).value().presentedBy('rsa-audience', 'https://rsa-issuer.org')`)
	if issues.Err() != nil {
		t.Fatalf("env.Compile failed: %v", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := <-prg.ConcurrentEval(ctx, map[string]any{
		"token":  tokStr,
		"pubkey": pubPEM,
	})
	if res.Err != nil {
		t.Fatalf("ConcurrentEval failed: %v", res.Err)
	}

	if res.Val.Value() != true {
		t.Errorf("RSA JWT verification got %v, want true", res.Val.Value())
	}
}

func TestJWTSignatureVerificationFailure(t *testing.T) {
	secret := "valid-secret"
	wrongSecret := "wrong-secret"

	payload := map[string]any{
		"iss": "https://auth.example.com",
		"aud": "my-api",
	}
	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}

	tokStr := createTestHMACJWT(header, payload, secret)

	env, err := cel.NewEnv(
		jwt.Library(),
		cel.Variable("token", cel.StringType),
		cel.Variable("key", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	ast, issues := env.Compile(`jwt.verifyWithKey(token, key).hasValue()`)
	if issues.Err() != nil {
		t.Fatalf("env.Compile failed: %v", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := <-prg.ConcurrentEval(ctx, map[string]any{
		"token": tokStr,
		"key":   wrongSecret,
	})
	if res.Err != nil {
		t.Fatalf("ConcurrentEval failed: %v", res.Err)
	}
	if res.Val.Value() != false {
		t.Errorf("Expected hasValue() == false for wrong secret, got %v", res.Val.Value())
	}
}

func TestJWTKeyFetchingAndCaching(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	payload := map[string]any{
		"iss": "https://fetched-issuer.example.com",
		"aud": "fetched-api-audience",
		"sub": "user_12345",
	}
	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "key-id-99",
	}

	tokStr, pubPEM := createTestRSAJWT(header, payload, privKey)

	var fetchCount int64
	mockFetcher := func(ctx context.Context, issuer, keyID string) (string, error) {
		atomic.AddInt64(&fetchCount, 1)
		if issuer != "https://fetched-issuer.example.com" {
			t.Errorf("mockFetcher got issuer %q, want 'https://fetched-issuer.example.com'", issuer)
		}
		if keyID != "key-id-99" {
			t.Errorf("mockFetcher got keyID %q, want 'key-id-99'", keyID)
		}
		return pubPEM, nil
	}

	env, err := cel.NewEnv(
		jwt.Library(
			jwt.CustomKeyFetcher(mockFetcher),
			jwt.KeyCacheTTL(5*time.Minute),
		),
		cel.Variable("token", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	ast, issues := env.Compile(`jwt.verify(token).value().presentedBy('fetched-api-audience', 'https://fetched-issuer.example.com')`)
	if issues.Err() != nil {
		t.Fatalf("env.Compile failed: %v", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("env.Program failed: %v", err)
	}

	ctx := context.Background()

	// First call -> key fetcher should be invoked (fetchCount = 1)
	res1 := <-prg.ConcurrentEval(ctx, map[string]any{"token": tokStr})
	if res1.Err != nil {
		t.Fatalf("First ConcurrentEval failed: %v", res1.Err)
	}
	if res1.Val.Value() != true {
		t.Errorf("First ConcurrentEval got %v, want true", res1.Val.Value())
	}
	if atomic.LoadInt64(&fetchCount) != 1 {
		t.Errorf("After 1st eval, fetchCount = %d, want 1", atomic.LoadInt64(&fetchCount))
	}

	// Second call -> should hit cache, key fetcher NOT invoked again (fetchCount remains 1)
	res2 := <-prg.ConcurrentEval(ctx, map[string]any{"token": tokStr})
	if res2.Err != nil {
		t.Fatalf("Second ConcurrentEval failed: %v", res2.Err)
	}
	if res2.Val.Value() != true {
		t.Errorf("Second ConcurrentEval got %v, want true", res2.Val.Value())
	}
	if atomic.LoadInt64(&fetchCount) != 1 {
		t.Errorf("After 2nd eval, fetchCount = %d, want 1 (cache hit)", atomic.LoadInt64(&fetchCount))
	}
}

func exportPublicKeyPEM(pubKey any) string {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		panic(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})
	return string(pubPEM)
}

func createTestToken(alg string, header, payload map[string]any, signingKey any) (string, string) {
	header["alg"] = alg
	hBytes, _ := json.Marshal(header)
	pBytes, _ := json.Marshal(payload)

	hB64 := base64.RawURLEncoding.EncodeToString(hBytes)
	pB64 := base64.RawURLEncoding.EncodeToString(pBytes)
	signedData := hB64 + "." + pB64

	switch strings.ToUpper(alg) {
	case "NONE":
		return signedData + ".", ""
	case "HS256":
		secret := signingKey.(string)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signedData))
		sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		return signedData + "." + sigB64, secret
	case "HS384":
		secret := signingKey.(string)
		mac := hmac.New(sha512.New384, []byte(secret))
		mac.Write([]byte(signedData))
		sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		return signedData + "." + sigB64, secret
	case "HS512":
		secret := signingKey.(string)
		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write([]byte(signedData))
		sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		return signedData + "." + sigB64, secret
	case "RS256":
		priv := signingKey.(*rsa.PrivateKey)
		hashed := sha256.Sum256([]byte(signedData))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "RS384":
		priv := signingKey.(*rsa.PrivateKey)
		hashed := sha512.Sum384([]byte(signedData))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA384, hashed[:])
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "RS512":
		priv := signingKey.(*rsa.PrivateKey)
		hashed := sha512.Sum512([]byte(signedData))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA512, hashed[:])
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "PS256":
		priv := signingKey.(*rsa.PrivateKey)
		hashed := sha256.Sum256([]byte(signedData))
		sig, _ := rsa.SignPSS(rand.Reader, priv, crypto.SHA256, hashed[:], nil)
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "PS384":
		priv := signingKey.(*rsa.PrivateKey)
		hashed := sha512.Sum384([]byte(signedData))
		sig, _ := rsa.SignPSS(rand.Reader, priv, crypto.SHA384, hashed[:], nil)
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "PS512":
		priv := signingKey.(*rsa.PrivateKey)
		hashed := sha512.Sum512([]byte(signedData))
		sig, _ := rsa.SignPSS(rand.Reader, priv, crypto.SHA512, hashed[:], nil)
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "ES256":
		priv := signingKey.(*ecdsa.PrivateKey)
		hashed := sha256.Sum256([]byte(signedData))
		sig, _ := ecdsa.SignASN1(rand.Reader, priv, hashed[:])
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "ES384":
		priv := signingKey.(*ecdsa.PrivateKey)
		hashed := sha512.Sum384([]byte(signedData))
		sig, _ := ecdsa.SignASN1(rand.Reader, priv, hashed[:])
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "ES512":
		priv := signingKey.(*ecdsa.PrivateKey)
		hashed := sha512.Sum512([]byte(signedData))
		sig, _ := ecdsa.SignASN1(rand.Reader, priv, hashed[:])
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(&priv.PublicKey)
	case "EDDSA":
		priv := signingKey.(ed25519.PrivateKey)
		pub := priv.Public().(ed25519.PublicKey)
		sig := ed25519.Sign(priv, []byte(signedData))
		return signedData + "." + base64.RawURLEncoding.EncodeToString(sig), exportPublicKeyPEM(pub)
	default:
		panic("unsupported test alg: " + alg)
	}
}

func TestJWTAllAlgorithms(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}
	es256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey P256 failed: %v", err)
	}
	es384Key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey P384 failed: %v", err)
	}
	es512Key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey P521 failed: %v", err)
	}
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey failed: %v", err)
	}
	hmacSecret := "my-test-hmac-secret-key-32bytes!"

	algs := []struct {
		name       string
		signingKey any
	}{
		{"NONE", nil},
		{"HS256", hmacSecret},
		{"HS384", hmacSecret},
		{"HS512", hmacSecret},
		{"RS256", rsaKey},
		{"RS384", rsaKey},
		{"RS512", rsaKey},
		{"PS256", rsaKey},
		{"PS384", rsaKey},
		{"PS512", rsaKey},
		{"ES256", es256Key},
		{"ES384", es384Key},
		{"ES512", es512Key},
		{"EdDSA", edKey},
	}

	for _, tc := range algs {
		t.Run(tc.name, func(t *testing.T) {
			header := map[string]any{"typ": "JWT"}
			payload := map[string]any{
				"iss": "https://test-issuer.org",
				"sub": "user-123",
				"aud": "test-audience",
			}
			tokStr, keyStr := createTestToken(tc.name, header, payload, tc.signingKey)

			env, err := cel.NewEnv(
				jwt.Library(),
				cel.Variable("token", cel.StringType),
				cel.Variable("key", cel.StringType),
			)
			if err != nil {
				t.Fatalf("cel.NewEnv failed: %v", err)
			}

			ast, issues := env.Compile(`jwt.verifyWithKey(token, key).hasValue()`)
			if issues.Err() != nil {
				t.Fatalf("env.Compile failed for %s: %v", tc.name, issues.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("env.Program failed for %s: %v", tc.name, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			res := <-prg.ConcurrentEval(ctx, map[string]any{
				"token": tokStr,
				"key":   keyStr,
			})
			if res.Err != nil {
				t.Fatalf("ConcurrentEval failed for %s: %v", tc.name, res.Err)
			}
			want := tc.name != "NONE"
			if res.Val.Value() != want {
				t.Errorf("Verification result for %s got %v, want %v", tc.name, res.Val.Value(), want)
			}
		})
	}
}

type customStringer struct {
	val string
}

func (s customStringer) String() string {
	return s.val
}

func TestJWKSKeyFetcherCoverage(t *testing.T) {
	ctx := context.Background()

	// Empty issuer test
	_, err := jwt.DefaultJWKSKeyFetcher(ctx, "", "kid-1")
	if err == nil || !strings.Contains(err.Error(), "issuer is empty") {
		t.Errorf("Expected empty issuer error, got: %v", err)
	}

	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid/.well-known/jwks.json":
			nB64 := base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
			eBytes := big.NewInt(int64(rsaKey.E)).Bytes()
			eB64 := base64.RawURLEncoding.EncodeToString(eBytes)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{
					{
						"kty": "RSA",
						"kid": "key-rsa-1",
						"n":   nB64,
						"e":   eB64,
					},
					{
						"kty": "RSA",
						"kid": "key-x5c-1",
						"x5c": []any{"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA"},
					},
				},
			})
		case "/404/.well-known/jwks.json":
			http.NotFound(w, r)
		case "/badjson/.well-known/jwks.json":
			w.Write([]byte("{invalid json"))
		case "/empty/.well-known/jwks.json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// 1. Valid RSA fetch
	pemKey, err := jwt.DefaultJWKSKeyFetcher(ctx, server.URL+"/valid", "key-rsa-1")
	if err != nil || pemKey == "" {
		t.Fatalf("DefaultJWKSKeyFetcher failed for valid RSA: %v", err)
	}

	// 2. Valid x5c fetch
	pemCert, err := jwt.DefaultJWKSKeyFetcher(ctx, server.URL+"/valid", "key-x5c-1")
	if err != nil || !strings.Contains(pemCert, "BEGIN CERTIFICATE") {
		t.Fatalf("DefaultJWKSKeyFetcher failed for x5c: %v", err)
	}

	// 3. Key ID not found
	_, err = jwt.DefaultJWKSKeyFetcher(ctx, server.URL+"/valid", "non-existent-kid")
	if err == nil || !strings.Contains(err.Error(), "not found in JWKS") {
		t.Errorf("Expected key ID not found error, got %v", err)
	}

	// 4. HTTP 404
	_, err = jwt.DefaultJWKSKeyFetcher(ctx, server.URL+"/404", "key-1")
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Errorf("Expected 404 error, got %v", err)
	}

	// 5. Bad JSON
	_, err = jwt.DefaultJWKSKeyFetcher(ctx, server.URL+"/badjson", "key-1")
	if err == nil || !strings.Contains(err.Error(), "failed to parse JWKS response") {
		t.Errorf("Expected bad JSON error, got %v", err)
	}

	// 6. Empty keys array
	_, err = jwt.DefaultJWKSKeyFetcher(ctx, server.URL+"/empty", "key-1")
	if err == nil || !strings.Contains(err.Error(), "no keys found in JWKS") {
		t.Errorf("Expected no keys error, got %v", err)
	}

	// 7. Request error (invalid server URL)
	_, err = jwt.DefaultJWKSKeyFetcher(ctx, "http://invalid.domain.that.does.not.exist.local", "key-1")
	if err == nil {
		t.Errorf("Expected HTTP request error, got nil")
	}

	// 8. Scheme inference (domain without http:// or https:// prefix defaults to https://)
	_, err = jwt.DefaultJWKSKeyFetcher(ctx, "accounts.example.invalid", "key-1")
	if err == nil || !strings.Contains(err.Error(), "accounts.example.invalid") {
		t.Errorf("Expected request error for scheme-less issuer, got %v", err)
	}
}

func TestJwkToPEMCoverage(t *testing.T) {
	// 1. Missing n or e
	_, err := jwt.JwkToPEM(map[string]any{"kty": "RSA", "n": "abc"})
	if err == nil || !strings.Contains(err.Error(), "missing RSA n or e") {
		t.Errorf("Expected missing n or e error, got %v", err)
	}

	// 2. Bad n base64
	_, err = jwt.JwkToPEM(map[string]any{"kty": "RSA", "n": "!!!bad-base64!!!", "e": "AQAB"})
	if err == nil || !strings.Contains(err.Error(), "failed to decode RSA n") {
		t.Errorf("Expected bad n base64 error, got %v", err)
	}

	// 3. Bad e base64
	_, err = jwt.JwkToPEM(map[string]any{"kty": "RSA", "n": "AQAB", "e": "!!!bad-base64!!!"})
	if err == nil || !strings.Contains(err.Error(), "failed to decode RSA e") {
		t.Errorf("Expected bad e base64 error, got %v", err)
	}

	// 4. Unsupported kty
	_, err = jwt.JwkToPEM(map[string]any{"kty": "UNKNOWN_KTY"})
	if err == nil || !strings.Contains(err.Error(), "unsupported JWK key type") {
		t.Errorf("Expected unsupported kty error, got %v", err)
	}
}

func TestCachedKeyFetcherCoverage(t *testing.T) {
	ctx := context.Background()

	// Option exercise Version
	opt := jwt.Version(1)
	if opt == nil {
		t.Fatal("jwt.Version returned nil")
	}

	// 1. Nil fetcher
	var nilFetcherKeyFetcher jwt.KeyFetcher = nil
	env, err := cel.NewEnv(
		jwt.Library(jwt.CustomKeyFetcher(nilFetcherKeyFetcher)),
		cel.Variable("token", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	payload := map[string]any{"iss": "https://foo.com", "sub": "123"}
	header := map[string]any{"alg": "none"}
	tokStr, _ := createTestToken("NONE", header, payload, nil)

	ast, _ := env.Compile(`jwt.verify(token).hasValue()`)
	prg, _ := env.Program(ast)
	res := <-prg.ConcurrentEval(ctx, map[string]any{"token": tokStr})
	if res.Err != nil {
		t.Fatalf("ConcurrentEval failed: %v", res.Err)
	}
	if res.Val.Value() != false {
		t.Errorf("Expected hasValue() == false for nil key fetcher, got %v", res.Val.Value())
	}

	// 2. Fetcher returns error
	errFetcher := func(ctx context.Context, iss, kid string) (string, error) {
		return "", fmt.Errorf("custom fetch failure")
	}
	envErr, _ := cel.NewEnv(
		jwt.Library(jwt.CustomKeyFetcher(errFetcher)),
		cel.Variable("token", cel.StringType),
	)
	astErr, _ := envErr.Compile(`jwt.verify(token).hasValue()`)
	prgErr, _ := envErr.Program(astErr)
	resErr := <-prgErr.ConcurrentEval(ctx, map[string]any{"token": tokStr})
	if resErr.Err != nil {
		t.Fatalf("ConcurrentEval failed: %v", resErr.Err)
	}
	if resErr.Val.Value() != false {
		t.Errorf("Expected hasValue() == false for custom fetch failure, got %v", resErr.Val.Value())
	}
}

func TestVerifyTokenWithIssuerKeyCoverage(t *testing.T) {
	ctx := context.Background()

	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubPEM := exportPublicKeyPEM(&rsaKey.PublicKey)

	mockFetcher := func(ctx context.Context, iss, kid string) (string, error) {
		return pubPEM, nil
	}

	env, err := cel.NewEnv(
		jwt.Library(jwt.CustomKeyFetcher(mockFetcher)),
		cel.Variable("token", cel.StringType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	// 1. Missing iss claim in payload
	noIssTok, _ := createTestToken("RS256", map[string]any{"typ": "JWT"}, map[string]any{"sub": "123"}, rsaKey)
	ast1, _ := env.Compile(`jwt.verify(token).hasValue()`)
	prg1, _ := env.Program(ast1)
	res1 := <-prg1.ConcurrentEval(ctx, map[string]any{"token": noIssTok})
	if res1.Err != nil || res1.Val.Value() != false {
		t.Errorf("Expected hasValue() == false for missing iss claim, got val=%v err=%v", res1.Val, res1.Err)
	}

	// 2. Valid iss claim & signature
	validTok, _ := createTestToken("RS256", map[string]any{"typ": "JWT"}, map[string]any{"iss": "https://auth.com", "sub": "123"}, rsaKey)
	res2 := <-prg1.ConcurrentEval(ctx, map[string]any{"token": validTok})
	if res2.Err != nil || res2.Val.Value() != true {
		t.Fatalf("Expected successful verification, got val=%v err=%v", res2.Val, res2.Err)
	}

	// 3. Signature failure with fetched key
	badSigTok := validTok + "extra-garbage-sig"
	res3 := <-prg1.ConcurrentEval(ctx, map[string]any{"token": badSigTok})
	if res3.Err != nil || res3.Val.Value() != false {
		t.Errorf("Expected hasValue() == false for tampered token, got val=%v err=%v", res3.Val, res3.Err)
	}
}

func TestDecodeBase64SegmentCoverage(t *testing.T) {
	// Standard base64
	data, err := jwt.DecodeBase64Segment("aGVsbG8=")
	if err != nil || string(data) != "hello" {
		t.Errorf("Standard base64 failed: %v, %s", err, string(data))
	}

	// Raw base64
	data, err = jwt.DecodeBase64Segment("aGVsbG8")
	if err != nil || string(data) != "hello" {
		t.Errorf("Raw base64 failed: %v, %s", err, string(data))
	}

	// URL base64
	encodedURL := base64.URLEncoding.EncodeToString([]byte("hello~world?"))
	data, err = jwt.DecodeBase64Segment(encodedURL)
	if err != nil || string(data) != "hello~world?" {
		t.Errorf("URL base64 failed: %v", err)
	}

	// Invalid base64
	_, err = jwt.DecodeBase64Segment("!!!invalid!!!")
	if err == nil {
		t.Errorf("Expected base64 decode error for invalid input, got nil")
	}
}

func TestParseUnixTimeCoverage(t *testing.T) {
	// int64
	t1 := jwt.ParseUnixTime(int64(1600000000))
	if t1.Unix() != 1600000000 {
		t.Errorf("ParseUnixTime(int64) = %v, want 1600000000", t1.Unix())
	}

	// float64
	t2 := jwt.ParseUnixTime(float64(1600000000.5))
	if t2.Unix() != 1600000000 || t2.Nanosecond() == 0 {
		t.Errorf("ParseUnixTime(float64) failed: %v", t2)
	}

	// json.Number float
	numFloat := json.Number("1600000000.5")
	t3 := jwt.ParseUnixTime(numFloat)
	if t3.Unix() != 1600000000 {
		t.Errorf("ParseUnixTime(json.Number float) failed: %v", t3)
	}

	// json.Number invalid
	numInvalid := json.Number("invalid")
	t4 := jwt.ParseUnixTime(numInvalid)
	if !t4.IsZero() {
		t.Errorf("ParseUnixTime(json.Number invalid) = %v, want zero time", t4)
	}

	// Non-numeric type
	t5 := jwt.ParseUnixTime("string-value")
	if !t5.IsZero() {
		t.Errorf("ParseUnixTime(string) = %v, want zero time", t5)
	}
}

func TestParsePublicKeyCoverage(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, edKey, _ := ed25519.GenerateKey(rand.Reader)

	// 1. Raw bytes string (no PEM block)
	k1, err := jwt.ParsePublicKey("plain-secret-string")
	if err != nil || string(k1.([]byte)) != "plain-secret-string" {
		t.Errorf("ParsePublicKey(plain string) failed: %v", err)
	}

	// 2. PKCS1 RSA Public Key
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey),
	}))
	k2, err := jwt.ParsePublicKey(pkcs1PEM)
	if err != nil || reflect.TypeOf(k2) != reflect.TypeOf(&rsa.PublicKey{}) {
		t.Errorf("ParsePublicKey(PKCS1 RSA) failed: %v", err)
	}

	// 3. EC Private Key
	ecPrivBytes, _ := x509.MarshalECPrivateKey(ecKey)
	ecPrivPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: ecPrivBytes,
	}))
	k3, err := jwt.ParsePublicKey(ecPrivPEM)
	if err != nil || reflect.TypeOf(k3) != reflect.TypeOf(&ecdsa.PublicKey{}) {
		t.Errorf("ParsePublicKey(EC Private Key) failed: %v", err)
	}

	// 4. PKCS8 Private Key (RSA)
	pkcs8RSABytes, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
	pkcs8RSAPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8RSABytes,
	}))
	k4, err := jwt.ParsePublicKey(pkcs8RSAPEM)
	if err != nil || reflect.TypeOf(k4) != reflect.TypeOf(&rsa.PublicKey{}) {
		t.Errorf("ParsePublicKey(PKCS8 RSA) failed: %v", err)
	}

	// 5. PKCS8 Private Key (ECDSA)
	pkcs8ECBytes, _ := x509.MarshalPKCS8PrivateKey(ecKey)
	pkcs8ECPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8ECBytes,
	}))
	k5, err := jwt.ParsePublicKey(pkcs8ECPEM)
	if err != nil || reflect.TypeOf(k5) != reflect.TypeOf(&ecdsa.PublicKey{}) {
		t.Errorf("ParsePublicKey(PKCS8 ECDSA) failed: %v", err)
	}

	// 6. PKCS8 Private Key (Ed25519)
	pkcs8EdBytes, _ := x509.MarshalPKCS8PrivateKey(edKey)
	pkcs8EdPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8EdBytes,
	}))
	k6, err := jwt.ParsePublicKey(pkcs8EdPEM)
	if err != nil || reflect.TypeOf(k6) != reflect.TypeOf(ed25519.PublicKey{}) {
		t.Errorf("ParsePublicKey(PKCS8 Ed25519) failed: %v", err)
	}

	// 7. X509 Certificate
	certPEM := createTestCertificatePEM(rsaKey)
	k7, err := jwt.ParsePublicKey(certPEM)
	if err != nil || reflect.TypeOf(k7) != reflect.TypeOf(&rsa.PublicKey{}) {
		t.Errorf("ParsePublicKey(Certificate) failed: %v", err)
	}

	// 8. Unsupported / Malformed PEM
	malformedPEM := "-----BEGIN UNKNOWN-----\nQUJD\n-----END UNKNOWN-----\n"
	_, err = jwt.ParsePublicKey(malformedPEM)
	if err == nil || !strings.Contains(err.Error(), "unsupported or malformed public key PEM") {
		t.Errorf("Expected malformed PEM error, got %v", err)
	}
}

func createTestCertificatePEM(rsaKey *rsa.PrivateKey) string {
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}))
}

func TestVerifySignatureErrorBranches(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	// 1. NONE signature disabled error
	err := jwt.VerifySignature("NONE", []byte("data"), []byte("sig"), nil)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Expected disabled error for NONE, got %v", err)
	}

	// 2. HS256 with non-[]byte key
	err = jwt.VerifySignature("HS256", []byte("data"), []byte("sig"), &rsaKey.PublicKey)
	if err == nil || !strings.Contains(err.Error(), "invalid key for HMAC algorithm") {
		t.Errorf("Expected invalid key for HMAC error, got %v", err)
	}

	// 2b. HS256 with PEM public key block (HMAC algorithm confusion defense)
	pemKey := exportPublicKeyPEM(&rsaKey.PublicKey)
	err = jwt.VerifySignature("HS256", []byte("data"), []byte("sig"), []byte(pemKey))
	if err == nil || !strings.Contains(err.Error(), "PEM block cannot be used as an HMAC secret key") {
		t.Errorf("Expected HMAC PEM confusion error, got %v", err)
	}

	// 2c. RS256 with weak key (< 2048 bits)
	weakRsaKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	err = jwt.VerifySignature("RS256", []byte("data"), []byte("sig"), &weakRsaKey.PublicKey)
	if err == nil || !strings.Contains(err.Error(), "insecure RSA key size") {
		t.Errorf("Expected insecure RSA key size error, got %v", err)
	}

	// 3. RS256 with non-rsa key
	err = jwt.VerifySignature("RS256", []byte("data"), []byte("sig"), []byte("secret"))
	if err == nil || !strings.Contains(err.Error(), "invalid key for RSA algorithm") {
		t.Errorf("Expected invalid key for RSA error, got %v", err)
	}

	// 4. PS256 with non-rsa key
	err = jwt.VerifySignature("PS256", []byte("data"), []byte("sig"), &ecKey.PublicKey)
	if err == nil || !strings.Contains(err.Error(), "invalid key for RSA-PSS algorithm") {
		t.Errorf("Expected invalid key for RSA-PSS error, got %v", err)
	}

	// 5. ES256 with non-ecdsa key
	err = jwt.VerifySignature("ES256", []byte("data"), []byte("sig"), &rsaKey.PublicKey)
	if err == nil || !strings.Contains(err.Error(), "invalid key for ECDSA algorithm") {
		t.Errorf("Expected invalid key for ECDSA error, got %v", err)
	}

	// 6. EDDSA with invalid key
	err = jwt.VerifySignature("EDDSA", []byte("data"), []byte("sig"), []byte("short"))
	if err == nil || !strings.Contains(err.Error(), "invalid key for EdDSA algorithm") {
		t.Errorf("Expected invalid key for EdDSA error, got %v", err)
	}

	// 7. Unsupported algorithm
	err = jwt.VerifySignature("UNKNOWN_ALG", []byte("data"), []byte("sig"), []byte("key"))
	if err == nil || !strings.Contains(err.Error(), "unsupported algorithm: UNKNOWN_ALG") {
		t.Errorf("Expected unsupported algorithm error, got %v", err)
	}
}

func TestTokenClaimAndPresentedByCoverage(t *testing.T) {
	tok := &jwt.Token{
		Issuer:   "https://iss.com",
		Audience: []string{"aud1", "aud2"},
		Payload: map[string]any{
			"str":      "hello",
			"stringer": customStringer{val: "stringer-val"},
			"bool":     true,
			"num":      123.45,
			"nil_val":  nil,
			"bad_json": make(chan int),
		},
	}

	// PresentedBy
	if tok.PresentedBy("aud1", "https://wrong.com") != false {
		t.Errorf("PresentedBy issuer mismatch got true, want false")
	}
	if tok.PresentedBy("wrong_aud", "https://iss.com") != false {
		t.Errorf("PresentedBy audience mismatch got true, want false")
	}

	// Claim
	if tok.Claim("missing").Value() != nil {
		t.Errorf("Claim('missing') got %v, want nil", tok.Claim("missing").Value())
	}
	if tok.Claim("nil_val").Value() != nil {
		t.Errorf("Claim('nil_val') got %v, want nil", tok.Claim("nil_val").Value())
	}
	if tok.Claim("str").Value() != "hello" {
		t.Errorf("Claim('str') = %v, want 'hello'", tok.Claim("str").Value())
	}
	if tok.Claim("stringer").Value() != "stringer-val" {
		t.Errorf("Claim('stringer') = %v, want 'stringer-val'", tok.Claim("stringer").Value())
	}
	if tok.Claim("bool").Value() != "true" {
		t.Errorf("Claim('bool') = %v, want 'true'", tok.Claim("bool").Value())
	}
	if tok.Claim("bad_json").Value() == "" {
		t.Errorf("Claim('bad_json') expected non-empty fallback string")
	}
}

func TestParseUnverifiedTokenErrorBranches(t *testing.T) {
	// 1. Parts count < 2
	_, err := jwt.ParseUnverifiedToken("only-one-part")
	if err == nil || !strings.Contains(err.Error(), "invalid token format") {
		t.Errorf("Expected invalid token format error, got %v", err)
	}

	// 2. Parts count > 3
	_, err = jwt.ParseUnverifiedToken("a.b.c.d")
	if err == nil || !strings.Contains(err.Error(), "invalid token format") {
		t.Errorf("Expected invalid token format error, got %v", err)
	}

	// 3. Bad base64 header
	_, err = jwt.ParseUnverifiedToken("!!!bad-base64!!!.eyJzdWIiOiIxMjMifQ")
	if err == nil || !strings.Contains(err.Error(), "failed to decode header") {
		t.Errorf("Expected header decode error, got %v", err)
	}

	// 4. Bad JSON header
	badJSONB64 := base64.RawURLEncoding.EncodeToString([]byte("{invalid-json"))
	validPayloadB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123"}`))
	_, err = jwt.ParseUnverifiedToken(badJSONB64 + "." + validPayloadB64)
	if err == nil || !strings.Contains(err.Error(), "failed to parse header JSON") {
		t.Errorf("Expected header parse error, got %v", err)
	}

	// 5. Bad base64 payload
	validHeaderB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	_, err = jwt.ParseUnverifiedToken(validHeaderB64 + ".!!!bad-base64!!!")
	if err == nil || !strings.Contains(err.Error(), "failed to decode payload") {
		t.Errorf("Expected payload decode error, got %v", err)
	}

	// 6. Bad JSON payload
	_, err = jwt.ParseUnverifiedToken(validHeaderB64 + "." + badJSONB64)
	if err == nil || !strings.Contains(err.Error(), "failed to parse payload JSON") {
		t.Errorf("Expected payload parse error, got %v", err)
	}
}

func TestVerifyTokenErrorBranches(t *testing.T) {
	// 1. Missing signature segment when key is provided
	token2Parts := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123"}`))
	_, err := jwt.VerifyToken(token2Parts, "secret")
	if err == nil || !strings.Contains(err.Error(), "missing token signature") {
		t.Errorf("Expected missing token signature error, got %v", err)
	}

	// 2. Bad base64 signature
	_, err = jwt.VerifyToken(token2Parts+".!!!bad-sig-base64!!!", "secret")
	if err == nil || !strings.Contains(err.Error(), "failed to decode signature") {
		t.Errorf("Expected decode signature error, got %v", err)
	}

	// 3. Bad key PEM
	tokenRS2Parts := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123"}`))
	validSigB64 := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	_, err = jwt.VerifyToken(tokenRS2Parts+"."+validSigB64, "-----BEGIN FOO-----\n\n-----END FOO-----\n")
	if err == nil || !strings.Contains(err.Error(), "failed to parse key") {
		t.Errorf("Expected parse key error, got %v", err)
	}

	// 4. Algorithm 'none' error in VerifyToken
	tokenNoneWithSig := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123"}`)) + "." + validSigB64
	_, err = jwt.VerifyToken(tokenNoneWithSig, "")
	if err == nil || !strings.Contains(err.Error(), "unsigned token with algorithm 'none'") {
		t.Errorf("Expected unsigned token alg none error, got %v", err)
	}
}

func TestCELOverloadsErrorHandling(t *testing.T) {
	env, err := cel.NewEnv(
		jwt.Library(),
		cel.Variable("token", cel.StringType),
		cel.Variable("key", cel.StringType),
		cel.Variable("num", cel.IntType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	ctx := context.Background()

	// 1. jwt.verify with non-string
	ast1, _ := env.Compile(`jwt.verify(num)`)
	prg1, _ := env.Program(ast1)
	res1 := <-prg1.ConcurrentEval(ctx, map[string]any{"num": 123})
	if res1.Err == nil {
		t.Errorf("Expected error for jwt.verify(int), got nil")
	}

	// 2. jwt.verifyWithKey with non-string token
	ast2, _ := env.Compile(`jwt.verifyWithKey(num, key)`)
	prg2, _ := env.Program(ast2)
	res2 := <-prg2.ConcurrentEval(ctx, map[string]any{"num": 123, "key": "secret"})
	if res2.Err == nil {
		t.Errorf("Expected error for jwt.verifyWithKey(int, string), got nil")
	}

	// 3. jwt.verifyWithKey with non-string key
	ast3, _ := env.Compile(`jwt.verifyWithKey(token, num)`)
	prg3, _ := env.Program(ast3)
	res3 := <-prg3.ConcurrentEval(ctx, map[string]any{"token": "tok", "num": 123})
	if res3.Err == nil {
		t.Errorf("Expected error for jwt.verifyWithKey(string, int), got nil")
	}

	// 4. tok.claim with non-string claim name
	ast4, _ := env.Compile(`jwt.verifyWithKey(token, key).value().claim(num)`)
	prg4, _ := env.Program(ast4)
	tokStr, _ := createTestToken("NONE", map[string]any{"alg": "none"}, map[string]any{"sub": "123"}, nil)
	res4 := <-prg4.ConcurrentEval(ctx, map[string]any{"token": tokStr, "key": "", "num": 123})
	if res4.Err == nil {
		t.Errorf("Expected error for claim(int), got nil")
	}

	// 5. tok.presentedBy with wrong target or wrong args
	ast5, _ := env.Compile(`jwt.verifyWithKey(token, key).value().presentedBy(num, 'iss')`)
	prg5, _ := env.Program(ast5)
	res5 := <-prg5.ConcurrentEval(ctx, map[string]any{"token": tokStr, "key": "", "num": 123})
	if res5.Err == nil {
		t.Errorf("Expected error for presentedBy(int, string), got nil")
	}

	// 6. tok.presentedBy with non-string second arg
	ast6, _ := env.Compile(`jwt.verifyWithKey(token, key).value().presentedBy('aud', num)`)
	prg6, _ := env.Program(ast6)
	res6 := <-prg6.ConcurrentEval(ctx, map[string]any{"token": tokStr, "key": "", "num": 123})
	if res6.Err == nil {
		t.Errorf("Expected error for presentedBy(string, int), got nil")
	}

	// 7. jwt.verify with malformed token
	ast7, _ := env.Compile(`jwt.verify(token).hasValue()`)
	prg7, _ := env.Program(ast7)
	res7 := <-prg7.ConcurrentEval(ctx, map[string]any{"token": "malformed.token.string!"})
	if res7.Err != nil || res7.Val.Value() != false {
		t.Errorf("Expected hasValue() == false for jwt.verify(malformed), got val=%v err=%v", res7.Val, res7.Err)
	}
}

func TestAdditionalCoverageGaps(t *testing.T) {
	// 1. Version option & KeyCacheTTL <= 0
	_ = jwt.Library(jwt.Version(1), jwt.KeyCacheTTL(0))
	_ = jwt.Library(jwt.KeyCacheTTL(-5 * time.Minute))

	// 2. Full ParseUnverifiedToken with kid, iss, sub, jti, exp, nbf, iat, and single string aud
	headerFull := map[string]any{
		"alg": "none",
		"kid": "key-id-123",
	}
	payloadFull := map[string]any{
		"iss": "https://full-issuer.com",
		"sub": "user-456",
		"aud": "single-aud-string",
		"jti": "jti-789",
		"exp": int64(1700000000),
		"nbf": int64(1600000000),
		"iat": int64(1500000000),
	}
	tokFullStr, _ := createTestToken("NONE", headerFull, payloadFull, nil)
	parsedFull, err := jwt.ParseUnverifiedToken(tokFullStr)
	if err != nil {
		t.Fatalf("ParseUnverifiedToken failed: %v", err)
	}
	if parsedFull.KeyID != "key-id-123" || parsedFull.Issuer != "https://full-issuer.com" ||
		parsedFull.Subject != "user-456" || parsedFull.ID != "jti-789" ||
		len(parsedFull.Audience) != 1 || parsedFull.Audience[0] != "single-aud-string" ||
		parsedFull.ExpiresAt.Unix() != 1700000000 || parsedFull.NotBefore.Unix() != 1600000000 ||
		parsedFull.IssuedAt.Unix() != 1500000000 {
		t.Errorf("ParseUnverifiedToken did not extract all fields correctly: %+v", parsedFull)
	}

	// 3. Audience array with mixed types
	payloadAudArray := map[string]any{
		"aud": []any{123, "valid-aud-1", true, "valid-aud-2"},
	}
	tokAud, _ := createTestToken("NONE", map[string]any{"alg": "none"}, payloadAudArray, nil)
	parsedTok, err := jwt.ParseUnverifiedToken(tokAud)
	if err != nil || len(parsedTok.Audience) != 2 || parsedTok.Audience[0] != "valid-aud-1" {
		t.Errorf("Expected 2 valid audience strings, got %v", parsedTok.Audience)
	}

	// 4. Claim with JSON objects, arrays, and json.RawMessage
	tokObj := &jwt.Token{
		Payload: map[string]any{
			"json_obj": map[string]any{"key": "val"},
			"json_arr": []any{1, 2, 3},
			"json_raw": json.RawMessage(`"quoted_string"`),
		},
	}
	if v := tokObj.Claim("json_obj").Value(); v == nil {
		t.Errorf("Claim('json_obj') got nil")
	}
	if v := tokObj.Claim("json_arr").Value(); v == nil {
		t.Errorf("Claim('json_arr') got nil")
	}
	if v := tokObj.Claim("json_raw").Value(); v != "quoted_string" {
		t.Errorf("Claim('json_raw') = %v, want 'quoted_string'", v)
	}
}

func TestDecodeBase64SegmentAllFormats(t *testing.T) {
	msg := []byte("hello+world/foo")

	// 1. RawURLEncoding
	b1 := base64.RawURLEncoding.EncodeToString(msg)
	d1, err := jwt.DecodeBase64Segment(b1)
	if err != nil || string(d1) != string(msg) {
		t.Errorf("RawURLEncoding failed: %v", err)
	}

	// 2. URLEncoding
	b2 := base64.URLEncoding.EncodeToString(msg)
	d2, err := jwt.DecodeBase64Segment(b2)
	if err != nil || string(d2) != string(msg) {
		t.Errorf("URLEncoding failed: %v", err)
	}

	// 3. RawStdEncoding
	b3 := base64.RawStdEncoding.EncodeToString(msg)
	d3, err := jwt.DecodeBase64Segment(b3)
	if err != nil || string(d3) != string(msg) {
		t.Errorf("RawStdEncoding failed: %v", err)
	}

	// 4. StdEncoding
	b4 := base64.StdEncoding.EncodeToString(msg)
	d4, err := jwt.DecodeBase64Segment(b4)
	if err != nil || string(d4) != string(msg) {
		t.Errorf("StdEncoding failed: %v", err)
	}
}

type badQuotedType struct{}

func (badQuotedType) MarshalJSON() ([]byte, error) {
	return []byte(`"invalid-\uZZZZ"`), nil
}

func TestClaimBadQuotedJSON(t *testing.T) {
	tok := &jwt.Token{
		Payload: map[string]any{
			"bad_quote": badQuotedType{},
		},
	}
	if v := tok.Claim("bad_quote").Value(); v == nil {
		t.Errorf("Claim('bad_quote') got nil")
	}
}

func TestSignatureMismatchErrors(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, edKey, _ := ed25519.GenerateKey(rand.Reader)

	// RSA signature mismatch
	err := jwt.VerifySignature("RS256", []byte("data"), []byte("invalid-sig"), &rsaKey.PublicKey)
	if err == nil {
		t.Errorf("Expected RS256 signature mismatch error, got nil")
	}

	// RSA-PSS signature mismatch
	err = jwt.VerifySignature("PS256", []byte("data"), []byte("invalid-sig"), &rsaKey.PublicKey)
	if err == nil {
		t.Errorf("Expected PS256 signature mismatch error, got nil")
	}

	// ECDSA signature mismatch
	err = jwt.VerifySignature("ES256", []byte("data"), []byte("invalid-sig"), &ecKey.PublicKey)
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("Expected ECDSA signature mismatch, got %v", err)
	}

	// EdDSA signature mismatch
	fakeSig := make([]byte, ed25519.SignatureSize)
	err = jwt.VerifySignature("EDDSA", []byte("data"), fakeSig, edKey.Public().(ed25519.PublicKey))
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("Expected EdDSA signature mismatch, got %v", err)
	}
}

func TestDefaultJWKSKeyFetcherInvalidRequest(t *testing.T) {
	ctx := context.Background()
	_, err := jwt.DefaultJWKSKeyFetcher(ctx, "https://\x7f", "key-1")
	if err == nil || !strings.Contains(err.Error(), "failed to create JWKS HTTP request") {
		t.Errorf("Expected request creation error, got %v", err)
	}

	_, err = jwt.DefaultJWKSKeyFetcher(ctx, "http://insecure.example.com", "key-1")
	if err == nil || !strings.Contains(err.Error(), "insecure HTTP") {
		t.Errorf("Expected insecure HTTP error, got %v", err)
	}
}

func TestFinalCoveragePushes(t *testing.T) {
	// 1. Lowercase bearer prefix in VerifyToken
	secret := "secret-123"
	header := map[string]any{"alg": "HS256"}
	payload := map[string]any{"sub": "user1"}
	tokStr, _ := createTestToken("HS256", header, payload, secret)

	_, err := jwt.VerifyToken("bearer "+tokStr, secret)
	if err != nil {
		t.Errorf("VerifyToken with lowercase 'bearer ' prefix failed: %v", err)
	}

	// 2. Non-string non-array aud in ParseUnverifiedToken
	payloadIntAud := map[string]any{"aud": 12345}
	tokIntAud, _ := createTestToken("NONE", map[string]any{"alg": "none"}, payloadIntAud, nil)
	tokParsed, err := jwt.ParseUnverifiedToken(tokIntAud)
	if err != nil || len(tokParsed.Audience) != 0 {
		t.Errorf("Expected empty audience for int aud, got %v", tokParsed.Audience)
	}

	// 3. Signature mismatch for HS384, HS512, RS384, RS512, PS384, PS512, ES384, ES512
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	es384Key, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	es512Key, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)

	_ = jwt.VerifySignature("HS384", []byte("data"), []byte("bad-sig"), []byte(secret))
	_ = jwt.VerifySignature("HS512", []byte("data"), []byte("bad-sig"), []byte(secret))
	_ = jwt.VerifySignature("RS384", []byte("data"), []byte("bad-sig"), &rsaKey.PublicKey)
	_ = jwt.VerifySignature("RS512", []byte("data"), []byte("bad-sig"), &rsaKey.PublicKey)
	_ = jwt.VerifySignature("PS384", []byte("data"), []byte("bad-sig"), &rsaKey.PublicKey)
	_ = jwt.VerifySignature("PS512", []byte("data"), []byte("bad-sig"), &rsaKey.PublicKey)
	_ = jwt.VerifySignature("ES384", []byte("data"), []byte("bad-sig"), &es384Key.PublicKey)
	_ = jwt.VerifySignature("ES512", []byte("data"), []byte("bad-sig"), &es512Key.PublicKey)
}

func TestDecodeBase64SegmentDistinctBranches(t *testing.T) {
	// 1. RawURLEncoding (contains '_' or '-', no '=')
	_, err := jwt.DecodeBase64Segment("aGVsbG8_")
	if err != nil {
		t.Errorf("RawURLEncoding branch failed: %v", err)
	}

	// 2. URLEncoding (contains '_' or '-', with '=')
	_, err = jwt.DecodeBase64Segment("aGVsbG8_aGVsbG8=")
	if err != nil {
		t.Errorf("URLEncoding branch failed: %v", err)
	}

	// 3. RawStdEncoding (contains '+' or '/', no '=')
	_, err = jwt.DecodeBase64Segment("aGVsbG8+aGVsbG8")
	if err != nil {
		t.Errorf("RawStdEncoding branch failed: %v", err)
	}

	// 4. StdEncoding (contains '+' or '/', with '=')
	_, err = jwt.DecodeBase64Segment("aGVsbG8+aGVsbG8=")
	if err != nil {
		t.Errorf("StdEncoding branch failed: %v", err)
	}
}

func TestJWKS_ECAndOKPKeys(t *testing.T) {
	// Generate EC key (P-256)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate EC key: %v", err)
	}

	xBytes := ecKey.X.Bytes()
	yBytes := ecKey.Y.Bytes()
	xB64 := base64.RawURLEncoding.EncodeToString(xBytes)
	yB64 := base64.RawURLEncoding.EncodeToString(yBytes)

	// Create test server with EC JWK and an encryption key to test "use" filtering
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"use": "enc",
					"kid": "enc-key",
					"n":   "abc",
					"e":   "AQAB",
				},
				{
					"kty": "EC",
					"use": "sig",
					"crv": "P-256",
					"kid": "ec-key-1",
					"x":   xB64,
					"y":   yB64,
				},
			},
		})
	}))
	defer ts.Close()

	// Sign a token with EC key
	header := map[string]any{"alg": "ES256", "kid": "ec-key-1"}
	payload := map[string]any{"iss": ts.URL, "sub": "test-subject"}
	hBytes, _ := json.Marshal(header)
	pBytes, _ := json.Marshal(payload)
	hB64 := base64.RawURLEncoding.EncodeToString(hBytes)
	pB64 := base64.RawURLEncoding.EncodeToString(pBytes)
	signedData := hB64 + "." + pB64

	hashed := sha256.Sum256([]byte(signedData))
	sigASN1, err := ecdsa.SignASN1(rand.Reader, ecKey, hashed[:])
	if err != nil {
		t.Fatalf("Failed to ASN1 sign EC token: %v", err)
	}
	tokStr := signedData + "." + base64.RawURLEncoding.EncodeToString(sigASN1)

	env, err := cel.NewEnv(jwt.Library())
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	ast, issues := env.Compile(fmt.Sprintf("jwt.verify('%s').value().subject", tokStr))
	if issues.Err() != nil {
		t.Fatalf("Compile failed: %v", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Program failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	evalRes := <-prg.ConcurrentEval(ctx, cel.NoVars())
	if evalRes.Err != nil {
		t.Fatalf("ConcurrentEval failed: %v", evalRes.Err)
	}
	if evalRes.Val.Value() != "test-subject" {
		t.Errorf("Expected subject 'test-subject', got %v", evalRes.Val.Value())
	}
}

func TestConcurrentEnvCompileOptions(t *testing.T) {
	lib := jwt.Library()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cel.NewEnv(lib)
			if err != nil {
				t.Errorf("Concurrent cel.NewEnv failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestKeyCacheTTLDisabled(t *testing.T) {
	var fetchCount int32
	mockFetcher := func(ctx context.Context, issuer string, keyID string) (string, error) {
		atomic.AddInt32(&fetchCount, 1)
		return "secret-key", nil
	}

	env, err := cel.NewEnv(jwt.Library(
		jwt.CustomKeyFetcher(mockFetcher),
		jwt.KeyCacheTTL(0),
	))
	if err != nil {
		t.Fatalf("cel.NewEnv failed: %v", err)
	}

	tokStr := createTestHMACJWT(
		map[string]any{"alg": "HS256"},
		map[string]any{"iss": "https://custom.issuer.com"},
		"secret-key",
	)

	ast, _ := env.Compile(fmt.Sprintf("jwt.verify('%s').hasValue()", tokStr))
	prg, _ := env.Program(ast)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = <-prg.ConcurrentEval(ctx, cel.NoVars())
	_ = <-prg.ConcurrentEval(ctx, cel.NoVars())

	if got := atomic.LoadInt32(&fetchCount); got != 2 {
		t.Errorf("Expected 2 fetches with TTL=0 disabled cache, got %d", got)
	}
}

