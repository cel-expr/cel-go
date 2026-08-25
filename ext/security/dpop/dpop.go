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

// Package dpop implements CEL extension functions and Go helper utilities for OAuth 2.0
// Demonstrating Proof of Possession (DPoP) per RFC 9449.
package dpop

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/ext/security/jwt"
)

const (
	// dpopProofType is the CEL type name for dpop.Proof.
	dpopProofType = "dpop.Proof"
	maxProofSize  = 10 * 1024 * 1024 // 10MB maximum allowed proof size

	// Required header type for DPoP proof JWTs per RFC 9449 Section 4.2.
	dpopHeaderType = "dpop+jwt"
)

func defaultNowFunc() time.Time {
	return time.Now().UTC()
}

// NewDPoPLib creates a new DPoP library with the given options.
func NewDPoPLib(opts ...Option) *dpopLib {
	l := &dpopLib{
		version: ^uint32(0),
		now:     defaultNowFunc,
	}
	for _, o := range opts {
		l = o(l)
	}
	return l
}

// Library returns a cel.EnvOption to configure extended functions for DPoP proof parsing,
// request verification, and key confirmation inspection.
func Library(options ...Option) cel.EnvOption {
	return cel.Lib(NewDPoPLib(options...))
}

// Option declares a functional operator for configuring DPoP extension library behavior.
type Option func(*dpopLib) *dpopLib

// Version sets the library version for DPoP extensions.
func Version(version uint32) Option {
	return func(l *dpopLib) *dpopLib {
		l.version = version
		return l
	}
}

// ValidateTimes enables automatic time validation (iat) during proof parsing with an optional maximum age and clock leeway.
func ValidateTimes(maxAge time.Duration, leeway ...time.Duration) Option {
	return func(l *dpopLib) *dpopLib {
		l.validateTimes = true
		l.maxAge = maxAge
		if len(leeway) > 0 {
			l.clockLeeway = leeway[0]
		}
		return l
	}
}

// MaxAge sets the maximum acceptable age for DPoP proof iat creation timestamp.
func MaxAge(maxAge time.Duration) Option {
	return func(l *dpopLib) *dpopLib {
		l.maxAge = maxAge
		return l
	}
}

// Clock sets a custom clock function for time validation (defaults to time.Now).
func Clock(nowFunc func() time.Time) Option {
	return func(l *dpopLib) *dpopLib {
		l.now = nowFunc
		return l
	}
}

// ClockLeeway sets the tolerance window when checking proof time claims (iat).
func ClockLeeway(leeway time.Duration) Option {
	return func(l *dpopLib) *dpopLib {
		l.clockLeeway = leeway
		return l
	}
}

// AllowedAlgorithms restricts acceptable JWS alg values for DPoP proof JWTs.
func AllowedAlgorithms(algs ...string) Option {
	return func(l *dpopLib) *dpopLib {
		l.allowedAlgorithms = append(l.allowedAlgorithms, algs...)
		return l
	}
}

type dpopLib struct {
	version           uint32
	validateTimes     bool
	maxAge            time.Duration
	clockLeeway       time.Duration
	now               func() time.Time
	allowedAlgorithms []string
}

// LibraryName returns the CEL library identifier string.
func (*dpopLib) LibraryName() string {
	return "cel.lib.ext.security.dpop"
}

// CompileOptions returns environment options for declaring CEL functions and types.
func (l *dpopLib) CompileOptions() []cel.EnvOption {
	celProofType := cel.ObjectType(dpopProofType)
	proofType, err := types.NewNativeType(reflect.TypeFor[Proof](), types.ParseStructTag("cel"))
	if err != nil {
		panic(fmt.Errorf("failed to create dpop proof type: %w", err))
	}
	var adapt func() types.Adapter = func() types.Adapter {
		return types.DefaultTypeAdapter
	}
	celJWTTokenType := cel.ObjectType("jwt.Token")

	proofOverloads := func(baseID string, argTypes []*cel.Type, resType *cel.Type, fallback ref.Val, fn func(*Proof, []ref.Val) ref.Val) []cel.FunctionOpt {
		return proofMemberOverloadPair(celProofType, baseID, argTypes, resType, fallback, fn)
	}

	return []cel.EnvOption{
		cel.OptionalTypes(),
		cel.Types(proofType),
		func(e *cel.Env) (*cel.Env, error) {
			adapt = func() types.Adapter { return e.CELTypeAdapter() }
			return e, nil
		},
		cel.Function("dpop.parse",
			cel.FunctionDocs(
				"Parses a DPoP proof JWT string into a structured Proof representation per RFC 9449.",
				"Automatically strips leading 'DPoP ' prefixes if present.",
			),
			cel.Overload("dpop_parse_string",
				[]*cel.Type{cel.StringType},
				cel.OptionalType(celProofType),
				cel.OverloadExamples(
					"dpop.parse(dpopHeaderStr)",
					"dpop.parse('DPoP eyJ0eXAiOiJkcG9wK2p3dCIs...')",
				),
				cel.UnaryBinding(func(arg ref.Val) ref.Val {
					proofStr := arg.(types.String)
					p, err := ParseProof(string(proofStr))
					if err != nil {
						return types.NewErr("parse dpop proof failed: %w", err)
					}
					if len(l.allowedAlgorithms) > 0 && !slices.Contains(l.allowedAlgorithms, p.Algorithm) {
						return types.OptionalNone
					}
					if l.validateTimes && !l.isProofTimeValid(p) {
						return types.OptionalNone
					}
					return types.OptionalOf(adapt().NativeToValue(p))
				}),
			),
		),
		makeFunction("claim",
			"Queries a custom claim value by key name from the DPoP proof payload, returning an optional dynamic value.",
			proofOverloads("dpop_proof_claim_string", []*cel.Type{cel.StringType}, cel.OptionalType(cel.DynType), types.OptionalNone, func(p *Proof, args []ref.Val) ref.Val {
				return p.Claim(adapt(), string(args[0].(types.String)))
			})...,
		),
		makeFunction("matchesRequest",
			"Validates that the DPoP proof's htm and htu claims match the HTTP request method and target URI.\nPerforms RFC 3986 syntax and scheme normalization on the target URI, ignoring query and fragment parts.",
			proofOverloads("dpop_proof_matches_request_string_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType, types.False, func(p *Proof, args []ref.Val) ref.Val {
				return types.Bool(p.MatchesRequest(string(args[0].(types.String)), string(args[1].(types.String))))
			})...,
		),
		makeFunction("matchesNonce",
			"Validates that the DPoP proof's nonce claim matches the expected server-provided nonce.",
			proofOverloads("dpop_proof_matches_nonce_string", []*cel.Type{cel.StringType}, cel.BoolType, types.False, func(p *Proof, args []ref.Val) ref.Val {
				return types.Bool(p.MatchesNonce(string(args[0].(types.String))))
			})...,
		),
		makeFunction("matchesToken",
			"Validates that the DPoP proof matches the access token, simultaneously checking both the accessTokenHash (ath) and cnf.jkt key confirmation.\nAccepts either a token string (e.g. from the Authorization header) or a parsed jwt.Token.",
			concatFunctionOpts(
				proofOverloads("dpop_proof_matches_token_string", []*cel.Type{cel.StringType}, cel.BoolType, types.False, func(p *Proof, args []ref.Val) ref.Val {
					return evalMatchesToken(p, args[0])
				}),
				proofOverloads("dpop_proof_matches_token_jwt_token", []*cel.Type{celJWTTokenType}, cel.BoolType, types.False, func(p *Proof, args []ref.Val) ref.Val {
					return evalMatchesToken(p, args[0])
				}),
				proofOverloads("dpop_proof_matches_token_opt_jwt_token", []*cel.Type{cel.OptionalType(celJWTTokenType)}, cel.BoolType, types.False, func(p *Proof, args []ref.Val) ref.Val {
					return evalMatchesToken(p, args[0])
				}),
			)...,
		),
	}
}

func makeFunction(name string, doc string, opts ...cel.FunctionOpt) cel.EnvOption {
	allOpts := make([]cel.FunctionOpt, 0, len(opts)+1)
	allOpts = append(allOpts, cel.FunctionDocs(doc))
	allOpts = append(allOpts, opts...)
	return cel.Function(name, allOpts...)
}

func withProofReceiver(fallback ref.Val, fn func(*Proof, []ref.Val) ref.Val) func(...ref.Val) ref.Val {
	return func(args ...ref.Val) ref.Val {
		switch target := args[0].(type) {
		case *types.Optional:
			if !target.HasValue() {
				return fallback
			}
			p, ok := target.GetValue().Value().(*Proof)
			if !ok {
				return types.ValOrErr(target.GetValue(), "expected dpop.Proof")
			}
			return fn(p, args[1:])
		default:
			p, ok := target.Value().(*Proof)
			if !ok {
				return types.ValOrErr(target, "expected dpop.Proof")
			}
			return fn(p, args[1:])
		}
	}
}

func proofMemberOverloadPair(
	celProofType *cel.Type,
	baseID string,
	argTypes []*cel.Type,
	resultType *cel.Type,
	fallback ref.Val,
	fn func(*Proof, []ref.Val) ref.Val,
) []cel.FunctionOpt {
	binding := withProofReceiver(fallback, fn)
	return []cel.FunctionOpt{
		cel.MemberOverload(
			baseID,
			append([]*cel.Type{celProofType}, argTypes...),
			resultType,
			cel.FunctionBinding(binding),
		),
		cel.MemberOverload(
			baseID+"_opt",
			append([]*cel.Type{cel.OptionalType(celProofType)}, argTypes...),
			resultType,
			cel.FunctionBinding(binding),
		),
	}
}

func evalMatchesToken(p *Proof, tokenVal ref.Val) ref.Val {
	if opt, ok := tokenVal.(*types.Optional); ok {
		if !opt.HasValue() {
			return types.False
		}
		tokenVal = opt.GetValue()
	}
	switch tok := tokenVal.Value().(type) {
	case string:
		return types.Bool(p.MatchesTokenString(tok))
	case *jwt.Token:
		return types.Bool(p.MatchesToken(tok))
	default:
		return types.ValOrErr(tokenVal, "expected string or jwt.Token")
	}
}

func concatFunctionOpts(lists ...[]cel.FunctionOpt) []cel.FunctionOpt {
	var total int
	for _, l := range lists {
		total += len(l)
	}
	res := make([]cel.FunctionOpt, 0, total)
	for _, l := range lists {
		res = append(res, l...)
	}
	return res
}

// ProgramOptions returns program options for DPoP extensions.
func (l *dpopLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func (l *dpopLib) isProofTimeValid(p *Proof) bool {
	return !l.validateTimes || p.IsValidAt(l.now(), l.maxAge, l.clockLeeway)
}

// Proof represents a parsed OAuth 2.0 DPoP proof JWT (RFC 9449).
// A Proof instance and its associated Header and Payload maps MUST be treated as immutable once parsed or created.
type Proof struct {
	// Header fields
	Algorithm string         `json:"alg" cel:"alg"`
	KeyID     string         `json:"kid,omitempty" cel:"keyId"`
	Type      string         `json:"typ" cel:"type"`
	JWK       map[string]any `json:"jwk" cel:"-"`

	// Derived public key thumbprint (RFC 7638 SHA-256 JWK Thumbprint)
	Thumbprint string `json:"jkt" cel:"thumbprint"`

	// Payload claims (RFC 9449)
	ID              string    `json:"jti" cel:"id"`
	Method          string    `json:"htm" cel:"method"`
	URI             string    `json:"htu" cel:"uri"`
	IssuedAt        time.Time `json:"iat" cel:"iat"`
	AccessTokenHash string    `json:"ath,omitempty" cel:"accessTokenHash"`
	Nonce           string    `json:"nonce,omitempty" cel:"nonce"`

	// Raw JSON payload and header associated with the proof including custom claims.
	// Must be treated as read-only once initialized.
	Payload map[string]any `json:"-" cel:"-"`
	Header  map[string]any `json:"-" cel:"-"`
}

// IsValidAt checks whether the creation timestamp (iat) is valid relative to refTime, optional maxAge, and clock leeway tolerance.
func (p *Proof) IsValidAt(refTime time.Time, maxAge time.Duration, leeway time.Duration) bool {
	now := refTime.UTC()
	lateNow := now.Add(leeway)

	// Issued-at time is present and in the future.
	if !p.IssuedAt.IsZero() && p.IssuedAt.Compare(lateNow) > 0 {
		return false
	}
	// Check max age if configured
	if maxAge > 0 && !p.IssuedAt.IsZero() {
		earlyBound := now.Add(-maxAge - leeway)
		if p.IssuedAt.Compare(earlyBound) < 0 {
			return false
		}
	}

	return true
}

// MatchesMethod checks whether the DPoP proof's htm claim matches the given HTTP request method.
func (p *Proof) MatchesMethod(method string) bool {
	return strings.EqualFold(strings.TrimSpace(p.Method), strings.TrimSpace(method))
}

// MatchesTargetURI checks whether the DPoP proof's htu claim matches the given target URI per RFC 9449 / RFC 3986.
// Normalizes scheme/host to lowercase, removes default ports (:80, :443), cleans path segments, and ignores query/fragment.
func (p *Proof) MatchesTargetURI(targetURI string) bool {
	normProofURI, err := normalizeTargetURI(p.URI)
	if err != nil {
		return false
	}
	normTargetURI, err := normalizeTargetURI(targetURI)
	if err != nil {
		return false
	}
	return normProofURI == normTargetURI
}

// MatchesRequest checks whether both the method (htm) and target URI (htu) match the incoming HTTP request.
func (p *Proof) MatchesRequest(method, targetURI string) bool {
	return p.MatchesMethod(method) && p.MatchesTargetURI(targetURI)
}

// MatchesAccessToken validates that the DPoP proof's ath claim equals the SHA-256 base64url hash of the presented access token.
func (p *Proof) MatchesAccessToken(accessToken string) bool {
	if p.AccessTokenHash == "" {
		return false
	}
	expectedATH := ComputeAccessTokenHash(accessToken)
	return subtle.ConstantTimeCompare([]byte(p.AccessTokenHash), []byte(expectedATH)) == 1
}

// MatchesNonce validates that the DPoP proof's nonce claim matches the expected nonce.
func (p *Proof) MatchesNonce(expectedNonce string) bool {
	if p.Nonce == "" || expectedNonce == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(p.Nonce), []byte(expectedNonce)) == 1
}

// MatchesConfirmationString validates that the DPoP proof's JWK thumbprint matches the given jkt string.
func (p *Proof) MatchesConfirmationString(jkt string) bool {
	if p.Thumbprint == "" || jkt == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(p.Thumbprint), []byte(jkt)) == 1
}

// MatchesConfirmationMap validates that the DPoP proof's JWK thumbprint matches the jkt member in a cnf confirmation map.
func (p *Proof) MatchesConfirmationMap(cnf map[string]any) bool {
	if cnf == nil {
		return false
	}
	jkt, ok := cnf["jkt"].(string)
	if !ok || jkt == "" {
		return false
	}
	return p.MatchesConfirmationString(jkt)
}

// MatchesTokenString validates that the DPoP proof matches the given access token string:
// 1. Validates that the DPoP proof's accessTokenHash matches the base64url SHA-256 hash of the presented token string.
// 2. Parses the token string as a JWT and validates that the cnf.jkt confirmation claim matches the DPoP proof's thumbprint.
func (p *Proof) MatchesTokenString(tokenStr string) bool {
	if !p.MatchesAccessToken(tokenStr) {
		return false
	}
	tok, err := jwt.ParseToken(tokenStr)
	if err != nil {
		return false
	}
	return p.MatchesToken(tok)
}

// MatchesToken validates that the DPoP proof matches the given jwt.Token:
// 1. Validates that the token's cnf.jkt confirmation claim matches the DPoP proof's thumbprint.
// 2. If token.Raw is populated, also validates that the DPoP proof's accessTokenHash matches the hash of token.Raw.
func (p *Proof) MatchesToken(token *jwt.Token) bool {
	if token == nil || token.Payload == nil {
		return false
	}
	if token.Raw != "" && p.AccessTokenHash != "" {
		if !p.MatchesAccessToken(token.Raw) {
			return false
		}
	}
	cnf, ok := token.Payload["cnf"].(map[string]any)
	if !ok || cnf == nil {
		return false
	}
	return p.MatchesConfirmationMap(cnf)
}

// Claim queries a claim value by key name using the provided types.Adapter, returning an optional dyn value.
func (p *Proof) Claim(adapter types.Adapter, claimName string) ref.Val {
	val, ok := p.Payload[claimName]
	if !ok || val == nil {
		return types.OptionalNone
	}
	refVal := adapter.NativeToValue(val)
	if types.IsError(refVal) {
		return refVal
	}
	return types.OptionalOf(refVal)
}

// NewProof constructs a validated Proof from decoded JSON header and payload maps.
// Signature verification of the proof must be performed prior to passing the proof to CEL.
func NewProof(header, payload map[string]any) (*Proof, error) {
	typ, err := requireString(header, "typ", "header")
	if err != nil || !strings.EqualFold(typ, dpopHeaderType) {
		return nil, fmt.Errorf("invalid or missing 'typ' header: expected %q, got %q", dpopHeaderType, typ)
	}

	alg, err := requireString(header, "alg", "header")
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(alg, "none") {
		return nil, fmt.Errorf("insecure algorithm 'none' is not allowed for DPoP proof")
	}
	if strings.HasPrefix(strings.ToUpper(alg), "HS") {
		return nil, fmt.Errorf("symmetric MAC algorithm %q is not allowed for DPoP proof", alg)
	}

	jwkMap, err := requireMap(header, "jwk", "header")
	if err != nil {
		return nil, fmt.Errorf("missing required header: 'jwk'")
	}
	if err := validateJWKPublicKeyOnly(jwkMap); err != nil {
		return nil, err
	}

	thumbprint, err := ComputeJWKThumbprint(jwkMap)
	if err != nil {
		return nil, fmt.Errorf("failed to compute JWK thumbprint: %w", err)
	}

	jti, err := requireString(payload, "jti", "claim")
	if err != nil {
		return nil, err
	}

	htm, err := requireString(payload, "htm", "claim")
	if err != nil {
		return nil, err
	}

	htu, err := requireString(payload, "htu", "claim")
	if err != nil {
		return nil, err
	}

	iat, err := types.ParseTimestamp(payload["iat"])
	if err != nil || iat.IsZero() {
		return nil, fmt.Errorf("missing or invalid required claim: 'iat'")
	}

	return &Proof{
		Algorithm:       alg,
		KeyID:           optString(header, "kid"),
		Type:            typ,
		JWK:             jwkMap,
		Thumbprint:      thumbprint,
		ID:              jti,
		Method:          htm,
		URI:             htu,
		IssuedAt:        iat,
		AccessTokenHash: optString(payload, "ath"),
		Nonce:           optString(payload, "nonce"),
		Payload:         payload,
		Header:          header,
	}, nil
}

// ParseProof parses a DPoP proof JWT string into a structured Proof.
// Automatically strips leading 'DPoP ' prefixes if present.
func ParseProof(proofStr string) (*Proof, error) {
	proofStr = trimPrefixFold(proofStr, "dpop ")
	if len(proofStr) > maxProofSize {
		return nil, fmt.Errorf("dpop proof size exceeds maximum allowed limit of %d bytes", maxProofSize)
	}

	parts := strings.SplitN(proofStr, ".", 4)
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

	return NewProof(header, payload)
}

// ComputeAccessTokenHash computes the RFC 9449 access token hash (ath): base64url(sha256(access_token)).
// Automatically strips leading 'DPoP ' or 'Bearer ' prefixes if present.
func ComputeAccessTokenHash(token string) string {
	token = trimPrefixFold(token, "dpop ", "bearer ")
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// ComputeJWKThumbprint computes the RFC 7638 SHA-256 JWK Thumbprint (base64url encoded without padding).
func ComputeJWKThumbprint(jwk map[string]any) (string, error) {
	kty, err := requireString(jwk, "kty", "JWK parameter")
	if err != nil {
		return "", fmt.Errorf("missing or invalid 'kty' in JWK")
	}

	var canonicalJSON string
	switch kty {
	case "RSA":
		e, err := requireString(jwk, "e", "RSA JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'e' in RSA JWK")
		}
		n, err := requireString(jwk, "n", "RSA JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'n' in RSA JWK")
		}
		canonicalJSON = fmt.Sprintf(`{"e":%q,"kty":"RSA","n":%q}`, e, n)

	case "EC":
		crv, err := requireString(jwk, "crv", "EC JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'crv' in EC JWK")
		}
		x, err := requireString(jwk, "x", "EC JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'x' in EC JWK")
		}
		y, err := requireString(jwk, "y", "EC JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'y' in EC JWK")
		}
		canonicalJSON = fmt.Sprintf(`{"crv":%q,"kty":"EC","x":%q,"y":%q}`, crv, x, y)

	case "OKP":
		crv, err := requireString(jwk, "crv", "OKP JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'crv' in OKP JWK")
		}
		x, err := requireString(jwk, "x", "OKP JWK parameter")
		if err != nil {
			return "", fmt.Errorf("missing or invalid 'x' in OKP JWK")
		}
		canonicalJSON = fmt.Sprintf(`{"crv":%q,"kty":"OKP","x":%q}`, crv, x)

	default:
		return "", fmt.Errorf("unsupported key type %q for JWK thumbprint calculation", kty)
	}

	h := sha256.Sum256([]byte(canonicalJSON))
	return base64.RawURLEncoding.EncodeToString(h[:]), nil
}

func validateJWKPublicKeyOnly(jwk map[string]any) error {
	kty, _ := jwk["kty"].(string)
	if strings.EqualFold(kty, "oct") {
		return fmt.Errorf("symmetric key type 'oct' is forbidden in DPoP JWK header")
	}

	// Forbidden private key members across RSA, EC, OKP, and generic JWKs
	forbiddenPrivateFields := []string{"d", "p", "q", "dp", "dq", "qi", "dmp1", "dmq1", "oth", "k"}
	for _, field := range forbiddenPrivateFields {
		if _, exists := jwk[field]; exists {
			return fmt.Errorf("private key parameter %q MUST NOT be present in DPoP proof JWK header", field)
		}
	}
	return nil
}

func normalizeTargetURI(rawURI string) (string, error) {
	rawURI = strings.TrimSpace(rawURI)
	if rawURI == "" {
		return "", fmt.Errorf("empty URI")
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("invalid URI %q: %w", rawURI, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("URI must include scheme and host: %q", rawURI)
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()

	// Strip standard default ports per scheme
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	effectiveHost := host
	if strings.Contains(host, ":") { // IPv6 literal
		effectiveHost = "[" + strings.Trim(host, "[]") + "]"
	}
	if port != "" {
		effectiveHost = effectiveHost + ":" + port
	}

	cleanPath := path.Clean(u.Path)
	if cleanPath == "." || cleanPath == "" {
		cleanPath = "/"
	} else if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	return scheme + "://" + effectiveHost + cleanPath, nil
}

func requireString(m map[string]any, key, context string) (string, error) {
	v, ok := m[key].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("missing or invalid required %s: %q", context, key)
	}
	return strings.TrimSpace(v), nil
}

func requireMap(m map[string]any, key, context string) (map[string]any, error) {
	v, ok := m[key].(map[string]any)
	if !ok || len(v) == 0 {
		return nil, fmt.Errorf("missing or empty required %s object: %q", context, key)
	}
	return v, nil
}

func optString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func trimPrefixFold(s string, prefixes ...string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range prefixes {
		if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			return strings.TrimSpace(s[len(prefix):])
		}
	}
	return s
}

func decodeBase64Segment(seg string) ([]byte, error) {
	seg = strings.TrimSpace(seg)
	if b, err := base64.RawURLEncoding.DecodeString(seg); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(seg); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(seg); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(seg)
}

// GenerateNonce creates a cryptographically secure 256-bit base64url-encoded random nonce.
func GenerateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateStatelessNonce creates an HMAC-signed timestamped nonce valid across distributed servers.
// The nonce is formatted as: base64url(payload).base64url(hmac_signature)
// where payload contains the unix timestamp, random entropy, and optional context strings.
func GenerateStatelessNonce(secretKey []byte, context ...string) (string, error) {
	now := time.Now().UTC().Unix()
	entropy := make([]byte, 16)
	if _, err := rand.Read(entropy); err != nil {
		return "", fmt.Errorf("failed to generate nonce entropy: %w", err)
	}
	payload := fmt.Sprintf("%d:%s:%s", now, base64.RawURLEncoding.EncodeToString(entropy), strings.Join(context, ":"))

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig, nil
}

// ValidateStatelessNonce verifies that the stateless nonce has a valid HMAC signature, has not expired,
// and matches any optional context strings.
func ValidateStatelessNonce(nonce string, secretKey []byte, maxAge time.Duration, context ...string) bool {
	parts := strings.Split(nonce, ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedSigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secretKey)
	mac.Write(payloadBytes)
	if subtle.ConstantTimeCompare(mac.Sum(nil), expectedSigBytes) != 1 {
		return false
	}

	payloadParts := strings.Split(string(payloadBytes), ":")
	if len(payloadParts) < 2 {
		return false
	}
	ts, err := strconv.ParseInt(payloadParts[0], 10, 64)
	if err != nil {
		return false
	}
	nonceTime := time.Unix(ts, 0).UTC()
	now := time.Now().UTC()
	if now.Sub(nonceTime) > maxAge || nonceTime.Sub(now) > 10*time.Second {
		return false
	}

	if len(context) > 0 {
		expectedContext := strings.Join(context, ":")
		actualContext := strings.Join(payloadParts[2:], ":")
		if subtle.ConstantTimeCompare([]byte(actualContext), []byte(expectedContext)) != 1 {
			return false
		}
	}
	return true
}
