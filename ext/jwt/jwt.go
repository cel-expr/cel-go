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

package jwt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"math"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

const (
	// jwtTokenType is the CEL type name for jwt.Token.
	jwtTokenType = "jwt.Token"
)

// KeyFetcher defines a function for retrieving verification keys for a given issuer and key ID.
type KeyFetcher func(ctx context.Context, issuer string, keyID string) (string, error)

// Library returns a cel.EnvOption to configure extended functions for JWT verification.
//
// # Supported Signature Algorithms
//
// Validated against specifications and major OAuth/OIDC identity providers (Auth0, Google, Apple, jwt.io):
//   - HMAC: HS256, HS384, HS512
//   - RSA PKCS#1 v1.5: RS256, RS384, RS512
//   - RSA-PSS: PS256, PS384, PS512
//   - ECDSA: ES256 (P-256), ES384 (P-384), ES512 (P-521)
//   - EdDSA: EdDSA (Ed25519)
//
// # Key Fetching & JWKS Support
//
// By default, `jwt.verify` automatically fetches verification keys via standard OIDC JWKS endpoints
// (`<issuer>/.well-known/jwks.json`). Keys are cached in-memory for 15 minutes by default (configurable
// via `KeyCacheTTL`). Key formats supported include RSA, EC (Elliptic Curve), OKP (Ed25519), and x5c certificates.
// Non-signature keys (e.g. `use: "enc"`) are automatically filtered out.
//
// # Requirements & Limitations
//
//   - Tokens processed by `jwt.verify` must contain an `iss` (issuer) claim to look up public keys.
//   - Cryptographic verification checks signature integrity; claims validation (e.g., `aud`, `iss`)
//     can be performed using `.presentedBy(aud, iss)`.
//   - Optional `Bearer ` token prefixes are automatically trimmed.
//
// # Functions
//
// ## jwt.verify
//
// Verifies and parses a JWT token using a configured key fetcher.
//
//	jwt.verify(<string>) -> jwt.Token
//
// ## jwt.verifyWithKey
//
// Verifies and parses a JWT token with an explicit verification key string.
//
//	jwt.verifyWithKey(<string>, <string>) -> jwt.Token
//
// ## jwt.Token.presentedBy
//
// Determines whether the token (or optional token) was issued for the expected audience (`aud`) and issuer (`iss`).
// If invoked on an `optional_type<jwt.Token>` that is `optional.none()`, it returns `false`.
//
//	<jwt.Token>.presentedBy(<string aud>, <string iss>) -> bool
//	<optional_type<jwt.Token>>.presentedBy(<string aud>, <string iss>) -> bool
//
// ## jwt.Token.claim
//
// Queries a claim value by key name, returning an optional string value.
//
//	<jwt.Token>.claim(<string>) -> optional_type<string>
func Library(options ...Option) cel.EnvOption {
	l := &jwtLib{
		version:    math.MaxUint32,
		keyFetcher: defaultJWKSKeyFetcher,
		cacheTTL:   15 * time.Minute,
	}
	for _, o := range options {
		l = o(l)
	}
	l.cachedFetcher = newCachedKeyFetcher(l.keyFetcher, l.cacheTTL)
	return cel.Lib(l)
}

// Option declares a functional operator for configuring JWT extension library behavior.
type Option func(*jwtLib) *jwtLib

// Version sets the library version for JWT extensions.
func Version(version uint32) Option {
	return func(l *jwtLib) *jwtLib {
		l.version = version
		return l
	}
}

// CustomKeyFetcher sets a custom KeyFetcher for retrieving verification public keys by issuer.
func CustomKeyFetcher(fetcher KeyFetcher) Option {
	return func(l *jwtLib) *jwtLib {
		l.keyFetcher = fetcher
		return l
	}
}

// KeyCacheTTL sets the cache duration for fetched verification keys.
func KeyCacheTTL(ttl time.Duration) Option {
	return func(l *jwtLib) *jwtLib {
		l.cacheTTL = ttl
		return l
	}
}

type jwtLib struct {
	version       uint32
	keyFetcher    KeyFetcher
	cacheTTL      time.Duration
	cachedFetcher *cachedKeyFetcher
}

func (*jwtLib) LibraryName() string {
	return "cel.lib.ext.jwt"
}

func (l *jwtLib) CompileOptions() []cel.EnvOption {
	celTokenType := cel.ObjectType(jwtTokenType)
	tokenType, err := types.NewNativeType(reflect.TypeFor[Token](), types.ParseStructTag("cel"))
	if err != nil {
		panic(fmt.Errorf("failed to create token type: %w", err))
	}
	var adapt func() types.Adapter
	return []cel.EnvOption{
		cel.OptionalTypes(),
		cel.Types(tokenType),
		func(e *cel.Env) (*cel.Env, error) {
			adapt = func() types.Adapter { return e.CELTypeAdapter() }
			return e, nil
		},
		cel.Function("jwt.verify",
			cel.Overload("jwt_verify_string",
				[]*cel.Type{cel.StringType},
				cel.OptionalType(celTokenType),
				cel.AsyncBinding(func(ctx context.Context, args ...ref.Val) ref.Val {
					tokStr := args[0].(types.String)
					token, err := l.verifyTokenWithIssuerKey(ctx, string(tokStr))
					if err != nil {
						return types.OptionalNone
					}
					return types.OptionalOf(adapt().NativeToValue(token))
				}),
			),
		),
		cel.Function("jwt.verifyWithKey",
			cel.Overload("jwt_verify_with_key_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.OptionalType(celTokenType),
				cel.BinaryBinding(func(tok, key ref.Val) ref.Val {
					tokStr := tok.(types.String)
					keyStr := key.(types.String)
					token, err := VerifyToken(string(tokStr), string(keyStr))
					if err != nil {
						return types.OptionalNone
					}
					return types.OptionalOf(adapt().NativeToValue(token))
				}),
			),
		),
		cel.Function("claim",
			cel.MemberOverload("jwt_token_claim_string",
				[]*cel.Type{celTokenType, cel.StringType},
				cel.OptionalType(cel.StringType),
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					tok := lhs.Value().(*Token)
					claimName := rhs.(types.String)
					return tok.Claim(string(claimName))
				}),
			),
		),
		cel.Function("presentedBy",
			cel.MemberOverload("jwt_token_presented_by_string_string",
				[]*cel.Type{celTokenType, cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					tok, ok := args[0].Value().(*Token)
					if !ok {
						return types.ValOrErr(args[0], "expected jwt.Token")
					}
					audVal := args[1].(types.String)
					issVal := args[2].(types.String)
					return types.Bool(tok.PresentedBy(string(audVal), string(issVal)))
				}),
			),
			cel.MemberOverload("jwt_optional_token_presented_by_string_string",
				[]*cel.Type{cel.OptionalType(celTokenType), cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					optTok, ok := args[0].(*types.Optional)
					if !ok || !optTok.HasValue() {
						return types.False
					}
					tok, ok := optTok.GetValue().Value().(*Token)
					if !ok {
						return types.False
					}
					audVal := args[1].(types.String)
					issVal := args[2].(types.String)
					return types.Bool(tok.PresentedBy(string(audVal), string(issVal)))
				}),
			),
		),
	}
}

func (l *jwtLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func (l *jwtLib) verifyTokenWithIssuerKey(ctx context.Context, tokenStr string) (*Token, error) {
	unverified, err := ParseUnverifiedToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if unverified.Issuer == "" {
		return nil, errors.New("cannot verify token without public key: token contains no issuer ('iss') claim")
	}

	keyStr, err := l.cachedFetcher.GetKey(ctx, unverified.Issuer, unverified.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch verification key for issuer %q: %w", unverified.Issuer, err)
	}

	return VerifyToken(tokenStr, keyStr)
}

type keyCacheEntry struct {
	key       string
	fetchedAt time.Time
}

type cachedKeyFetcher struct {
	fetcher KeyFetcher
	cache   sync.Map
	ttl     time.Duration
}

func newCachedKeyFetcher(fetcher KeyFetcher, ttl time.Duration) *cachedKeyFetcher {
	return &cachedKeyFetcher{
		fetcher: fetcher,
		ttl:     ttl,
	}
}

func (c *cachedKeyFetcher) GetKey(ctx context.Context, issuer, keyID string) (string, error) {
	cacheKey := issuer + "::" + keyID
	if c.ttl > 0 {
		if val, ok := c.cache.Load(cacheKey); ok {
			entry := val.(keyCacheEntry)
			if time.Since(entry.fetchedAt) < c.ttl {
				return entry.key, nil
			}
		}
	}

	if c.fetcher == nil {
		return "", errors.New("no key fetcher configured for issuer key retrieval")
	}

	key, err := c.fetcher(ctx, issuer, keyID)
	if err != nil {
		return "", err
	}

	if c.ttl > 0 {
		c.cache.Store(cacheKey, keyCacheEntry{
			key:       key,
			fetchedAt: time.Now(),
		})
	}
	return key, nil
}

var defaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

func defaultJWKSKeyFetcher(ctx context.Context, issuer string, keyID string) (string, error) {
	if issuer == "" {
		return "", errors.New("cannot fetch JWKS key: issuer is empty")
	}

	lowerIss := strings.ToLower(issuer)
	if strings.HasPrefix(lowerIss, "http://") {
		if !strings.HasPrefix(lowerIss, "http://localhost") && !strings.HasPrefix(lowerIss, "http://127.0.0.1") && !strings.HasPrefix(lowerIss, "http://[::1]") {
			return "", errors.New("cannot fetch JWKS key over insecure HTTP: issuer URL must use HTTPS")
		}
	}

	url := issuer
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	url = strings.TrimSuffix(url, "/") + "/.well-known/jwks.json"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create JWKS HTTP request: %w", err)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("JWKS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("JWKS request returned status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return "", fmt.Errorf("failed to parse JWKS response: %w", err)
	}

	if len(jwks.Keys) == 0 {
		return "", errors.New("no keys found in JWKS")
	}

	for _, key := range jwks.Keys {
		if use, ok := key["use"].(string); ok && !strings.EqualFold(use, "sig") {
			continue
		}
		kid, _ := key["kid"].(string)
		if keyID == "" || kid == keyID {
			return jwkToPEM(key)
		}
	}

	return "", fmt.Errorf("key ID %q not found in JWKS for issuer %q", keyID, issuer)
}

func jwkToPEM(keyMap map[string]any) (string, error) {
	if x5c, ok := keyMap["x5c"].([]any); ok && len(x5c) > 0 {
		if certStr, ok := x5c[0].(string); ok {
			return "-----BEGIN CERTIFICATE-----\n" + certStr + "\n-----END CERTIFICATE-----\n", nil
		}
	}

	kty, _ := keyMap["kty"].(string)
	switch strings.ToUpper(kty) {
	case "RSA":
		nStr, _ := keyMap["n"].(string)
		eStr, _ := keyMap["e"].(string)
		if nStr == "" || eStr == "" {
			return "", errors.New("missing RSA n or e in JWK")
		}

		nBytes, err := decodeBase64Segment(nStr)
		if err != nil {
			return "", fmt.Errorf("failed to decode RSA n: %w", err)
		}
		eBytes, err := decodeBase64Segment(eStr)
		if err != nil {
			return "", fmt.Errorf("failed to decode RSA e: %w", err)
		}

		nInt := new(big.Int).SetBytes(nBytes)
		eInt := 0
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}

		rsaPubKey := &rsa.PublicKey{
			N: nInt,
			E: eInt,
		}

		der, err := x509.MarshalPKIXPublicKey(rsaPubKey)
		if err != nil {
			return "", err
		}

		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		})
		return string(pemBytes), nil

	case "EC":
		crv, _ := keyMap["crv"].(string)
		xStr, _ := keyMap["x"].(string)
		yStr, _ := keyMap["y"].(string)
		if crv == "" || xStr == "" || yStr == "" {
			return "", errors.New("missing EC crv, x, or y in JWK")
		}

		var curve elliptic.Curve
		switch strings.ToUpper(crv) {
		case "P-256", "P256":
			curve = elliptic.P256()
		case "P-384", "P384":
			curve = elliptic.P384()
		case "P-521", "P521":
			curve = elliptic.P521()
		default:
			return "", fmt.Errorf("unsupported EC curve: %s", crv)
		}

		xBytes, err := decodeBase64Segment(xStr)
		if err != nil {
			return "", fmt.Errorf("failed to decode EC x: %w", err)
		}
		yBytes, err := decodeBase64Segment(yStr)
		if err != nil {
			return "", fmt.Errorf("failed to decode EC y: %w", err)
		}

		ecPubKey := &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}

		der, err := x509.MarshalPKIXPublicKey(ecPubKey)
		if err != nil {
			return "", err
		}

		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		})
		return string(pemBytes), nil

	case "OKP":
		crv, _ := keyMap["crv"].(string)
		xStr, _ := keyMap["x"].(string)
		if !strings.EqualFold(crv, "Ed25519") || xStr == "" {
			return "", fmt.Errorf("unsupported OKP curve %q or missing x in JWK", crv)
		}

		xBytes, err := decodeBase64Segment(xStr)
		if err != nil {
			return "", fmt.Errorf("failed to decode OKP x: %w", err)
		}
		if len(xBytes) != ed25519.PublicKeySize {
			return "", fmt.Errorf("invalid Ed25519 public key length: %d", len(xBytes))
		}

		edPubKey := ed25519.PublicKey(xBytes)
		der, err := x509.MarshalPKIXPublicKey(edPubKey)
		if err != nil {
			return "", err
		}

		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		})
		return string(pemBytes), nil

	default:
		return "", fmt.Errorf("unsupported JWK key type: %s", kty)
	}
}

// Token represents a parsed and verified JWT token using Go native struct types.
type Token struct {
	RawHeader  string         `json:"-" cel:"-"`
	RawPayload string         `json:"-" cel:"-"`
	Header     map[string]any `json:"-" cel:"-"`
	Payload    map[string]any `json:"-" cel:"-"`

	Issuer    string    `json:"iss" cel:"issuer"`
	Subject   string    `json:"sub" cel:"subject"`
	Audience  []string  `json:"aud" cel:"aud"`
	ExpiresAt time.Time `json:"exp" cel:"exp"`
	NotBefore time.Time `json:"nbf" cel:"nbf"`
	IssuedAt  time.Time `json:"iat" cel:"iat"`
	ID        string    `json:"jti" cel:"id"`
	KeyID     string    `json:"kid" cel:"key_id"`
}

// PresentedBy determines whether the token was issued for the expected audience (`aud`) and issuer (`iss`).
func (t *Token) PresentedBy(aud, iss string) bool {
	if t.Issuer != iss {
		return false
	}
	for _, a := range t.Audience {
		if a == aud {
			return true
		}
	}
	return false
}

// Claim queries a claim value by key name, returning an optional string value.
func (t *Token) Claim(claimName string) ref.Val {
	val, ok := t.Payload[claimName]
	if !ok || val == nil {
		return types.OptionalNone
	}
	switch v := val.(type) {
	case string:
		return types.OptionalOf(types.String(v))
	case fmt.Stringer:
		return types.OptionalOf(types.String(v.String()))
	default:
		b, err := json.Marshal(v)
		if err == nil && len(b) > 0 && b[0] == '"' {
			var s string
			if json.Unmarshal(b, &s) == nil {
				return types.OptionalOf(types.String(s))
			}
			return types.OptionalOf(types.String(string(b)))
		} else if err == nil {
			return types.OptionalOf(types.String(string(b)))
		} else {
			return types.OptionalOf(types.String(fmt.Sprintf("%v", v)))
		}
	}
}

// ParseUnverifiedToken parses a JWT token string without signature verification.
func ParseUnverifiedToken(tokenStr string) (*Token, error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if strings.HasPrefix(strings.ToLower(tokenStr), "bearer ") {
		tokenStr = strings.TrimSpace(tokenStr[7:])
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid token format: expected 2 or 3 parts, got %d", len(parts))
	}

	headerBytes, err := decodeBase64Segment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse header JSON: %w", err)
	}

	payloadBytes, err := decodeBase64Segment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload JSON: %w", err)
	}

	tok := &Token{
		RawHeader:  parts[0],
		RawPayload: parts[1],
		Header:     header,
		Payload:    payload,
	}

	if v, ok := header["kid"].(string); ok {
		tok.KeyID = v
	}
	if v, ok := payload["iss"].(string); ok {
		tok.Issuer = v
	}
	if v, ok := payload["sub"].(string); ok {
		tok.Subject = v
	}
	if v, ok := payload["aud"].(string); ok {
		tok.Audience = []string{v}
	} else if arr, ok := payload["aud"].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok {
				tok.Audience = append(tok.Audience, s)
			}
		}
	}
	if v, ok := payload["exp"]; ok {
		tok.ExpiresAt = parseUnixTime(v)
	}
	if v, ok := payload["nbf"]; ok {
		tok.NotBefore = parseUnixTime(v)
	}
	if v, ok := payload["iat"]; ok {
		tok.IssuedAt = parseUnixTime(v)
	}
	if v, ok := payload["jti"].(string); ok {
		tok.ID = v
	}

	return tok, nil
}

// VerifyToken parses and verifies a JWT token string with the given public key or secret.
func VerifyToken(tokenStr, keyStr string) (*Token, error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if strings.HasPrefix(strings.ToLower(tokenStr), "bearer ") {
		tokenStr = strings.TrimSpace(tokenStr[7:])
	}

	tok, err := ParseUnverifiedToken(tokenStr)
	if err != nil {
		return nil, err
	}

	alg, _ := tok.Header["alg"].(string)
	if strings.EqualFold(alg, "none") {
		return nil, errors.New("unsigned token with algorithm 'none' cannot be verified; use ParseUnverifiedToken for unverified tokens")
	}

	if keyStr == "" {
		return nil, errors.New("missing verification key")
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) < 3 {
		return nil, errors.New("missing token signature")
	}
	sigBytes, err := decodeBase64Segment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	key, err := parsePublicKey(keyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key: %w", err)
	}

	signedData := []byte(parts[0] + "." + parts[1])
	if err := verifySignature(alg, signedData, sigBytes, key); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	return tok, nil
}

func decodeBase64Segment(seg string) ([]byte, error) {
	if data, err := base64.RawURLEncoding.DecodeString(seg); err == nil {
		return data, nil
	}
	if data, err := base64.URLEncoding.DecodeString(seg); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(seg); err == nil {
		return data, nil
	}
	return base64.StdEncoding.DecodeString(seg)
}

func parseUnixTime(v any) time.Time {
	switch val := v.(type) {
	case float64:
		sec := int64(val)
		nsec := int64((val - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).UTC()
	case int64:
		return time.Unix(val, 0).UTC()
	case int:
		return time.Unix(int64(val), 0).UTC()
	case json.Number:
		if f, err := val.Float64(); err == nil {
			sec := int64(f)
			nsec := int64((f - float64(sec)) * 1e9)
			return time.Unix(sec, nsec).UTC()
		}
	}
	return time.Time{}
}

func parsePublicKey(keyStr string) (any, error) {
	keyStr = strings.TrimSpace(keyStr)
	block, _ := pem.Decode([]byte(keyStr))
	if block == nil {
		return []byte(keyStr), nil
	}

	if pubKey, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		return pubKey, nil
	}
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		return cert.PublicKey, nil
	}
	if rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaKey, nil
	}
	if ecPrivKey, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return &ecPrivKey.PublicKey, nil
	}
	if privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch k := privKey.(type) {
		case *rsa.PrivateKey:
			return &k.PublicKey, nil
		case *ecdsa.PrivateKey:
			return &k.PublicKey, nil
		case ed25519.PrivateKey:
			return k.Public().(ed25519.PublicKey), nil
		}
	}
	return nil, errors.New("unsupported or malformed public key PEM")
}

func cryptoHash(alg string) (crypto.Hash, error) {
	switch strings.ToUpper(alg) {
	case "RS256", "PS256", "ES256":
		return crypto.SHA256, nil
	case "RS384", "PS384", "ES384":
		return crypto.SHA384, nil
	case "RS512", "PS512", "ES512":
		return crypto.SHA512, nil
	default:
		return 0, fmt.Errorf("unsupported algorithm: %s", alg)
	}
}

func verifySignature(alg string, signedData []byte, sigBytes []byte, key any) error {
	switch strings.ToUpper(alg) {
	case "NONE":
		return errors.New("algorithm 'none' is disabled for signature verification")

	case "HS256", "HS384", "HS512":
		var macHash func() hash.Hash
		switch strings.ToUpper(alg) {
		case "HS256":
			macHash = sha256.New
		case "HS384":
			macHash = sha512.New384
		case "HS512":
			macHash = sha512.New
		}
		keyBytes, ok := key.([]byte)
		if !ok {
			return errors.New("invalid key for HMAC algorithm: expected raw secret bytes")
		}
		if strings.HasPrefix(strings.TrimSpace(string(keyBytes)), "-----BEGIN ") {
			return errors.New("public or private key PEM block cannot be used as an HMAC secret key")
		}
		h := hmac.New(macHash, keyBytes)
		h.Write(signedData)
		if !hmac.Equal(sigBytes, h.Sum(nil)) {
			return errors.New("HMAC signature mismatch")
		}
		return nil

	case "RS256", "RS384", "RS512":
		hashType, _ := cryptoHash(alg)
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("invalid key for RSA algorithm")
		}
		if rsaKey.N == nil || rsaKey.N.BitLen() < 2048 {
			return fmt.Errorf("insecure RSA key size (%d bits): minimum required is 2048 bits", rsaKey.N.BitLen())
		}
		hasher := hashType.New()
		hasher.Write(signedData)
		return rsa.VerifyPKCS1v15(rsaKey, hashType, hasher.Sum(nil), sigBytes)

	case "PS256", "PS384", "PS512":
		hashType, _ := cryptoHash(alg)
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("invalid key for RSA-PSS algorithm")
		}
		if rsaKey.N == nil || rsaKey.N.BitLen() < 2048 {
			return fmt.Errorf("insecure RSA key size (%d bits): minimum required is 2048 bits", rsaKey.N.BitLen())
		}
		hasher := hashType.New()
		hasher.Write(signedData)
		return rsa.VerifyPSS(rsaKey, hashType, hasher.Sum(nil), sigBytes, nil)

	case "ES256", "ES384", "ES512":
		hashType, _ := cryptoHash(alg)
		ecdsaKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("invalid key for ECDSA algorithm")
		}
		hasher := hashType.New()
		hasher.Write(signedData)
		if !ecdsa.VerifyASN1(ecdsaKey, hasher.Sum(nil), sigBytes) {
			return errors.New("ECDSA signature mismatch")
		}
		return nil

	case "EDDSA":
		edKey, ok := key.(ed25519.PublicKey)
		if !ok {
			if b, ok := key.([]byte); ok && len(b) == ed25519.PublicKeySize {
				edKey = ed25519.PublicKey(b)
			} else {
				return errors.New("invalid key for EdDSA algorithm")
			}
		}
		if !ed25519.Verify(edKey, signedData, sigBytes) {
			return errors.New("EdDSA signature mismatch")
		}
		return nil

	default:
		return fmt.Errorf("unsupported algorithm: %s", alg)
	}
}
