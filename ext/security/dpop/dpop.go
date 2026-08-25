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

// Package dpop implements CEL extension functions for OAuth 2.0 Demonstrating Proof of Possession (DPoP)
// proof parsing, JWK thumbprint confirmation, access token hash validation, and request matching per RFC 9449.
package dpop

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"slices"
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

// Library returns a cel.EnvOption to configure extended functions for DPoP proof parsing,
// request verification, and key confirmation inspection.
func Library(options ...Option) cel.EnvOption {
	l := &dpopLib{
		version: ^uint32(0),
		now:     defaultNowFunc,
	}
	for _, o := range options {
		l = o(l)
	}
	return cel.Lib(l)
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
		cel.Function("dpop.ath",
			cel.FunctionDocs(
				"Computes the RFC 9449 access token hash (ath): base64url(sha256(access_token)).",
				"Automatically strips leading 'DPoP ' or 'Bearer ' prefixes if present.",
			),
			cel.Overload("dpop_ath_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.OverloadExamples(
					"dpop.ath(accessTokenStr)",
					"dpop.ath('DPoP Kz~8mXK1...')",
				),
				cel.UnaryBinding(func(arg ref.Val) ref.Val {
					tokenStr := string(arg.(types.String))
					return types.String(ComputeAccessTokenHash(tokenStr))
				}),
			),
		),
		cel.Function("dpop.thumbprint",
			cel.FunctionDocs(
				"Computes the RFC 7638 SHA-256 JWK Thumbprint (base64url encoded) from a JWK map.",
			),
			cel.Overload("dpop_thumbprint_map",
				[]*cel.Type{cel.MapType(cel.StringType, cel.DynType)},
				cel.StringType,
				cel.OverloadExamples(
					"dpop.thumbprint(proof.jwk)",
				),
				cel.UnaryBinding(func(arg ref.Val) ref.Val {
					jwkVal := arg.Value()
					jwkMap, ok := jwkVal.(map[string]any)
					if !ok {
						if genericMap, ok := jwkVal.(map[ref.Val]ref.Val); ok {
							jwkMap = make(map[string]any, len(genericMap))
							for k, v := range genericMap {
								jwkMap[fmt.Sprint(k.Value())] = v.Value()
							}
						} else {
							return types.NewErr("expected map[string]dyn for JWK, got %T", jwkVal)
						}
					}
					tp, err := ComputeJWKThumbprint(jwkMap)
					if err != nil {
						return types.NewErr("failed to compute JWK thumbprint: %w", err)
					}
					return types.String(tp)
				}),
			),
		),
		cel.Function("claim",
			cel.FunctionDocs(
				"Queries a custom claim value by key name from the DPoP proof payload, returning an optional dynamic value.",
			),
			cel.MemberOverload("dpop_proof_claim_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.OptionalType(cel.DynType),
				cel.OverloadExamples(
					"proof.claim('nonce')",
				),
				cel.BinaryBinding(func(targetVal, claimNameVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					claimName := claimNameVal.(types.String)
					return target.Claim(adapt(), string(claimName))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_claim_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.OptionalType(cel.DynType),
				cel.OverloadExamples(
					"dpop.parse(proofStr).claim('nonce')",
				),
				cel.BinaryBinding(func(targetVal, claimNameVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.OptionalNone
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					claimName := claimNameVal.(types.String)
					return target.Claim(adapt(), string(claimName))
				}),
			),
		),
		cel.Function("matchesRequest",
			cel.FunctionDocs(
				"Validates that the DPoP proof's htm and htu claims match the HTTP request method and target URI.",
				"Performs RFC 3986 syntax and scheme normalization on the target URI, ignoring query and fragment parts.",
			),
			cel.MemberOverload("dpop_proof_matches_request_string_string",
				[]*cel.Type{celProofType, cel.StringType, cel.StringType},
				cel.BoolType,
				cel.OverloadExamples(
					"proof.matchesRequest('POST', 'https://server.example.com/token')",
				),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					target := args[0].Value().(*Proof)
					method := string(args[1].(types.String))
					targetURI := string(args[2].(types.String))
					return types.Bool(target.MatchesRequest(method, targetURI))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_request_string_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType, cel.StringType},
				cel.BoolType,
				cel.OverloadExamples(
					"dpop.parse(proofStr).matchesRequest('POST', 'https://server.example.com/token')",
				),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					optTarget := args[0].(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					method := string(args[1].(types.String))
					targetURI := string(args[2].(types.String))
					return types.Bool(target.MatchesRequest(method, targetURI))
				}),
			),
		),
		cel.Function("matchesMethod",
			cel.FunctionDocs(
				"Validates that the DPoP proof's htm claim matches the given HTTP method.",
			),
			cel.MemberOverload("dpop_proof_matches_method_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, methodVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					method := string(methodVal.(types.String))
					return types.Bool(target.MatchesMethod(method))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_method_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, methodVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					method := string(methodVal.(types.String))
					return types.Bool(target.MatchesMethod(method))
				}),
			),
		),
		cel.Function("matchesURI",
			cel.FunctionDocs(
				"Validates that the DPoP proof's htu claim matches the given HTTP target URI (ignoring query/fragment, with normalization).",
			),
			cel.MemberOverload("dpop_proof_matches_uri_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, uriVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					targetURI := string(uriVal.(types.String))
					return types.Bool(target.MatchesURI(targetURI))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_uri_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, uriVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					targetURI := string(uriVal.(types.String))
					return types.Bool(target.MatchesURI(targetURI))
				}),
			),
		),
		cel.Function("matchesHtu",
			cel.FunctionDocs(
				"Alias for matchesURI: validates that the DPoP proof's htu claim matches the given HTTP target URI.",
			),
			cel.MemberOverload("dpop_proof_matches_htu_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, uriVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					targetURI := string(uriVal.(types.String))
					return types.Bool(target.MatchesURI(targetURI))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_htu_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, uriVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					targetURI := string(uriVal.(types.String))
					return types.Bool(target.MatchesURI(targetURI))
				}),
			),
		),
		cel.Function("matchesAccessToken",
			cel.FunctionDocs(
				"Validates that the DPoP proof's ath claim matches the base64url SHA-256 hash of the presented access token.",
			),
			cel.MemberOverload("dpop_proof_matches_access_token_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, tokenVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					tokenStr := string(tokenVal.(types.String))
					return types.Bool(target.MatchesAccessToken(tokenStr))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_access_token_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, tokenVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					tokenStr := string(tokenVal.(types.String))
					return types.Bool(target.MatchesAccessToken(tokenStr))
				}),
			),
		),
		cel.Function("matchesNonce",
			cel.FunctionDocs(
				"Validates that the DPoP proof's nonce claim matches the expected server-provided nonce.",
			),
			cel.MemberOverload("dpop_proof_matches_nonce_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, nonceVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					nonceStr := string(nonceVal.(types.String))
					return types.Bool(target.MatchesNonce(nonceStr))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_nonce_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, nonceVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					nonceStr := string(nonceVal.(types.String))
					return types.Bool(target.MatchesNonce(nonceStr))
				}),
			),
		),
		cel.Function("matchesConfirmation",
			cel.FunctionDocs(
				"Validates that the DPoP proof's public key thumbprint matches the jkt confirmation method from an access token or introspection map.",
			),
			cel.MemberOverload("dpop_proof_matches_confirmation_string",
				[]*cel.Type{celProofType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, jktVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					jktStr := string(jktVal.(types.String))
					return types.Bool(target.MatchesConfirmationString(jktStr))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_confirmation_string",
				[]*cel.Type{cel.OptionalType(celProofType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, jktVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					jktStr := string(jktVal.(types.String))
					return types.Bool(target.MatchesConfirmationString(jktStr))
				}),
			),
			cel.MemberOverload("dpop_proof_matches_confirmation_map",
				[]*cel.Type{celProofType, cel.MapType(cel.StringType, cel.DynType)},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, cnfVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					cnfMap, ok := cnfVal.Value().(map[string]any)
					if !ok {
						if genericMap, ok := cnfVal.Value().(map[ref.Val]ref.Val); ok {
							cnfMap = make(map[string]any, len(genericMap))
							for k, v := range genericMap {
								cnfMap[fmt.Sprint(k.Value())] = v.Value()
							}
						} else {
							return types.False
						}
					}
					return types.Bool(target.MatchesConfirmationMap(cnfMap))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_confirmation_map",
				[]*cel.Type{cel.OptionalType(celProofType), cel.MapType(cel.StringType, cel.DynType)},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, cnfVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					cnfMap, ok := cnfVal.Value().(map[string]any)
					if !ok {
						if genericMap, ok := cnfVal.Value().(map[ref.Val]ref.Val); ok {
							cnfMap = make(map[string]any, len(genericMap))
							for k, v := range genericMap {
								cnfMap[fmt.Sprint(k.Value())] = v.Value()
							}
						} else {
							return types.False
						}
					}
					return types.Bool(target.MatchesConfirmationMap(cnfMap))
				}),
			),
		),
		cel.Function("matchesToken",
			cel.FunctionDocs(
				"Validates that the DPoP proof's public key matches the cnf.jkt confirmation claim inside a parsed jwt.Token.",
			),
			cel.MemberOverload("dpop_proof_matches_token_jwt_token",
				[]*cel.Type{celProofType, celJWTTokenType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, tokenVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					tok, ok := tokenVal.Value().(*jwt.Token)
					if !ok {
						return types.ValOrErr(tokenVal, "expected jwt.Token")
					}
					return types.Bool(target.MatchesToken(tok))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_token_jwt_token",
				[]*cel.Type{cel.OptionalType(celProofType), celJWTTokenType},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, tokenVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					tok, ok := tokenVal.Value().(*jwt.Token)
					if !ok {
						return types.ValOrErr(tokenVal, "expected jwt.Token")
					}
					return types.Bool(target.MatchesToken(tok))
				}),
			),
			cel.MemberOverload("dpop_proof_opt_matches_token_opt_jwt_token",
				[]*cel.Type{cel.OptionalType(celProofType), cel.OptionalType(celJWTTokenType)},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, tokenVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Proof)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected dpop.Proof")
					}
					optTok := tokenVal.(*types.Optional)
					if !optTok.HasValue() {
						return types.False
					}
					tok, ok := optTok.GetValue().Value().(*jwt.Token)
					if !ok {
						return types.ValOrErr(optTok.GetValue(), "expected jwt.Token")
					}
					return types.Bool(target.MatchesToken(tok))
				}),
			),
			cel.MemberOverload("dpop_proof_matches_token_opt_jwt_token",
				[]*cel.Type{celProofType, cel.OptionalType(celJWTTokenType)},
				cel.BoolType,
				cel.BinaryBinding(func(targetVal, tokenVal ref.Val) ref.Val {
					target := targetVal.Value().(*Proof)
					optTok := tokenVal.(*types.Optional)
					if !optTok.HasValue() {
						return types.False
					}
					tok, ok := optTok.GetValue().Value().(*jwt.Token)
					if !ok {
						return types.ValOrErr(optTok.GetValue(), "expected jwt.Token")
					}
					return types.Bool(target.MatchesToken(tok))
				}),
			),
		),
	}
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
	JWK       map[string]any `json:"jwk" cel:"jwk"`

	// Derived public key thumbprint (RFC 7638 SHA-256 JWK Thumbprint)
	Thumbprint string `json:"jkt" cel:"thumbprint"`

	// Payload claims (RFC 9449)
	ID              string    `json:"jti" cel:"id"`
	Method          string    `json:"htm" cel:"method"`
	URI             string    `json:"htu" cel:"uri"`
	IssuedAt        time.Time `json:"iat" cel:"iat"`
	AccessTokenHash string    `json:"ath,omitempty" cel:"ath"`
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

// MatchesURI checks whether the DPoP proof's htu claim matches the given target URI per RFC 9449 / RFC 3986.
// Normalizes scheme/host to lowercase, removes default ports (:80, :443), cleans path segments, and ignores query/fragment.
func (p *Proof) MatchesURI(targetURI string) bool {
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
	return p.MatchesMethod(method) && p.MatchesURI(targetURI)
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

// MatchesToken validates that the DPoP proof's JWK thumbprint matches the cnf.jkt confirmation claim inside a jwt.Token.
func (p *Proof) MatchesToken(token *jwt.Token) bool {
	if token == nil || token.Payload == nil {
		return false
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
	typ, ok := header["typ"].(string)
	if !ok || !strings.EqualFold(strings.TrimSpace(typ), dpopHeaderType) {
		return nil, fmt.Errorf("invalid or missing 'typ' header: expected %q, got %q", dpopHeaderType, typ)
	}

	alg, ok := header["alg"].(string)
	if !ok || alg == "" {
		return nil, fmt.Errorf("missing required header: 'alg'")
	}
	if strings.EqualFold(alg, "none") {
		return nil, fmt.Errorf("insecure algorithm 'none' is not allowed for DPoP proof")
	}
	if strings.HasPrefix(strings.ToUpper(alg), "HS") {
		return nil, fmt.Errorf("symmetric MAC algorithm %q is not allowed for DPoP proof", alg)
	}

	jwkRaw, ok := header["jwk"]
	if !ok || jwkRaw == nil {
		return nil, fmt.Errorf("missing required header: 'jwk'")
	}
	jwkMap, ok := jwkRaw.(map[string]any)
	if !ok || len(jwkMap) == 0 {
		return nil, fmt.Errorf("invalid header 'jwk': expected non-empty JSON object")
	}

	// Validate JWK does not contain private key components (RFC 9449 Section 4.2)
	if err := validateJWKPublicKeyOnly(jwkMap); err != nil {
		return nil, err
	}

	thumbprint, err := ComputeJWKThumbprint(jwkMap)
	if err != nil {
		return nil, fmt.Errorf("failed to compute JWK thumbprint: %w", err)
	}

	jti, ok := payload["jti"].(string)
	if !ok || strings.TrimSpace(jti) == "" {
		return nil, fmt.Errorf("missing required claim: 'jti'")
	}

	htm, ok := payload["htm"].(string)
	if !ok || strings.TrimSpace(htm) == "" {
		return nil, fmt.Errorf("missing required claim: 'htm'")
	}

	htu, ok := payload["htu"].(string)
	if !ok || strings.TrimSpace(htu) == "" {
		return nil, fmt.Errorf("missing required claim: 'htu'")
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
	proofStr = trimDPoPPrefix(proofStr)
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
	token = trimTokenPrefix(token)
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// ComputeJWKThumbprint computes the RFC 7638 SHA-256 JWK Thumbprint (base64url encoded without padding).
func ComputeJWKThumbprint(jwk map[string]any) (string, error) {
	kty, ok := jwk["kty"].(string)
	if !ok || kty == "" {
		return "", fmt.Errorf("missing or invalid 'kty' in JWK")
	}

	var canonicalJSON string
	switch kty {
	case "RSA":
		e, ok := jwk["e"].(string)
		if !ok || e == "" {
			return "", fmt.Errorf("missing or invalid 'e' in RSA JWK")
		}
		n, ok := jwk["n"].(string)
		if !ok || n == "" {
			return "", fmt.Errorf("missing or invalid 'n' in RSA JWK")
		}
		canonicalJSON = fmt.Sprintf(`{"e":%q,"kty":"RSA","n":%q}`, e, n)

	case "EC":
		crv, ok := jwk["crv"].(string)
		if !ok || crv == "" {
			return "", fmt.Errorf("missing or invalid 'crv' in EC JWK")
		}
		x, ok := jwk["x"].(string)
		if !ok || x == "" {
			return "", fmt.Errorf("missing or invalid 'x' in EC JWK")
		}
		y, ok := jwk["y"].(string)
		if !ok || y == "" {
			return "", fmt.Errorf("missing or invalid 'y' in EC JWK")
		}
		canonicalJSON = fmt.Sprintf(`{"crv":%q,"kty":"EC","x":%q,"y":%q}`, crv, x, y)

	case "OKP":
		crv, ok := jwk["crv"].(string)
		if !ok || crv == "" {
			return "", fmt.Errorf("missing or invalid 'crv' in OKP JWK")
		}
		x, ok := jwk["x"].(string)
		if !ok || x == "" {
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

func optString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func trimDPoPPrefix(proofStr string) string {
	proofStr = strings.TrimSpace(proofStr)
	if strings.HasPrefix(strings.ToLower(proofStr), "dpop ") {
		return strings.TrimSpace(proofStr[5:])
	}
	return proofStr
}

func trimTokenPrefix(tokenStr string) string {
	tokenStr = strings.TrimSpace(tokenStr)
	lower := strings.ToLower(tokenStr)
	if strings.HasPrefix(lower, "dpop ") {
		return strings.TrimSpace(tokenStr[5:])
	}
	if strings.HasPrefix(lower, "bearer ") {
		return strings.TrimSpace(tokenStr[7:])
	}
	return tokenStr
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
