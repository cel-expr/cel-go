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

// Package jwt implements CEL extension functions for JSON Web Token (JWT) parsing, claims inspection, and validation.
package jwt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

const (
	// jwtTokenType is the CEL type name for jwt.Token.
	jwtTokenType = "jwt.Token"
	maxTokenSize = 10 * 1024 * 1024 // 10MB maximum allowed token size
)

func defaultNowFunc() time.Time {
	return time.Now().UTC()
}

// NewJWTLib constructs an internal jwtLib instance for direct use.
func NewJWTLib() *jwtLib {
	return &jwtLib{
		version: ^uint32(0),
		now:     defaultNowFunc,
	}
}

// Library returns a cel.EnvOption to configure extended functions for JWT data handling and claims inspection.
func Library(options ...Option) cel.EnvOption {
	l := NewJWTLib()
	for _, o := range options {
		l = o(l)
	}
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

// ValidateTimes enables automatic time validation (iat, nbf, exp) during token parsing with an optional clock leeway.
func ValidateTimes(leeway ...time.Duration) Option {
	return func(l *jwtLib) *jwtLib {
		l.validateTimes = true
		if len(leeway) > 0 {
			l.clockLeeway = leeway[0]
		}
		return l
	}
}

// Clock sets a custom clock function for time validation (defaults to time.Now).
func Clock(nowFunc func() time.Time) Option {
	return func(l *jwtLib) *jwtLib {
		l.now = nowFunc
		return l
	}
}

// ClockLeeway sets the tolerance window when checking token time claims (iat, nbf, exp).
func ClockLeeway(leeway time.Duration) Option {
	return func(l *jwtLib) *jwtLib {
		l.clockLeeway = leeway
		return l
	}
}

type jwtLib struct {
	version       uint32
	validateTimes bool
	clockLeeway   time.Duration
	now           func() time.Time
}

// LibraryName returns the CEL library identifier string.
func (*jwtLib) LibraryName() string {
	return "cel.lib.ext.security.jwt"
}

// CompileOptions returns environment options for declaring CEL functions and types.
func (l *jwtLib) CompileOptions() []cel.EnvOption {
	celTokenType := cel.ObjectType(jwtTokenType)
	tokenType, err := types.NewNativeType(reflect.TypeFor[Token](), types.ParseStructTag("cel"))
	if err != nil {
		panic(fmt.Errorf("failed to create token type: %w", err))
	}
	var adapt func() types.Adapter = func() types.Adapter {
		return types.DefaultTypeAdapter
	}
	return []cel.EnvOption{
		cel.OptionalTypes(),
		cel.Types(tokenType),
		func(e *cel.Env) (*cel.Env, error) {
			adapt = func() types.Adapter { return e.CELTypeAdapter() }
			return e, nil
		},
		cel.Function("jwt.parse",
			cel.FunctionDocs(
				"Parses a JWT token string into a structured Token representation.",
				"Automatically strips leading 'Bearer ' prefixes if present.",
			),
			cel.Overload("jwt_parse_string",
				[]*cel.Type{cel.StringType},
				cel.OptionalType(celTokenType),
				cel.OverloadExamples(
					"jwt.parse(tokenStr)",
					"jwt.parse('Bearer eyJhbGciOi...')",
				),
				cel.UnaryBinding(func(arg ref.Val) ref.Val {
					tokenStr := arg.(types.String)
					tok, err := ParseToken(string(tokenStr))
					if err != nil {
						return types.NewErr("parse token failed: %w", err)
					}
					if l.validateTimes && !l.isTokenTimeValid(tok) {
						return types.OptionalNone
					}
					return types.OptionalOf(adapt().NativeToValue(tok))
				}),
			),
		),
		cel.Function("claim",
			cel.FunctionDocs(
				"Queries a custom claim value by key name from the JWT token payload, returning an optional dynamic value.",
			),
			cel.MemberOverload("jwt_token_claim_string",
				[]*cel.Type{celTokenType, cel.StringType},
				cel.OptionalType(cel.DynType),
				cel.OverloadExamples(
					"token.claim('tenant_id')",
					"token.claim('roles').orValue([])",
				),
				cel.BinaryBinding(func(targetVal, claimNameVal ref.Val) ref.Val {
					target := targetVal.Value().(*Token)
					claimName := claimNameVal.(types.String)
					return target.Claim(adapt(), string(claimName))
				}),
			),
			cel.MemberOverload("jwt_token_opt_claim_string",
				[]*cel.Type{cel.OptionalType(celTokenType), cel.StringType},
				cel.OptionalType(cel.DynType),
				cel.OverloadExamples(
					"jwt.parse(tokenStr).claim('tenant_id')",
					"jwt.parse(tokenStr).claim('tier').orValue('standard')",
				),
				cel.BinaryBinding(func(targetVal, claimNameVal ref.Val) ref.Val {
					optTarget := targetVal.(*types.Optional)
					if !optTarget.HasValue() {
						return types.OptionalNone
					}
					target, ok := optTarget.GetValue().Value().(*Token)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected jwt.Token")
					}
					claimName := claimNameVal.(types.String)
					return target.Claim(adapt(), string(claimName))
				}),
			),
		),
		cel.Function("presentedBy",
			cel.FunctionDocs(
				"Determines whether the token was presented by the expected authorized party (`azp`) or audience (`aud`) for the given issuer (`iss`).",
				"When `azp` is present in the token, it takes precedence over `aud`.",
			),
			cel.MemberOverload("jwt_token_presented_by_string_string",
				[]*cel.Type{celTokenType, cel.StringType, cel.StringType},
				cel.BoolType,
				cel.OverloadExamples(
					"token.presentedBy('https://accounts.google.com', 'my-client-app')",
				),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					target := args[0].Value().(*Token)
					issuer := args[1].(types.String)
					presenter := args[2].(types.String)
					return types.Bool(target.PresentedBy(string(issuer), string(presenter)))
				}),
			),
			cel.MemberOverload("jwt_token_opt_presented_by_string_string",
				[]*cel.Type{cel.OptionalType(celTokenType), cel.StringType, cel.StringType},
				cel.BoolType,
				cel.OverloadExamples(
					"jwt.parse(tokenStr).presentedBy('https://accounts.google.com', 'my-client-app')",
				),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					optTarget := args[0].(*types.Optional)
					if !optTarget.HasValue() {
						return types.False
					}
					target, ok := optTarget.GetValue().Value().(*Token)
					if !ok {
						return types.ValOrErr(optTarget.GetValue(), "expected jwt.Token")
					}
					issuer := args[1].(types.String)
					presenter := args[2].(types.String)
					return types.Bool(target.PresentedBy(string(issuer), string(presenter)))
				}),
			),
		),
	}
}

// ProgramOptions returns program options for JWT extensions.
func (l *jwtLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func (l *jwtLib) isTokenTimeValid(tok *Token) bool {
	return !l.validateTimes || tok.IsValidAt(l.now(), l.clockLeeway)
}

// Token represents a parsed JWT token using Go native struct types.
// A Token instance and its associated Payload map MUST be treated as immutable once parsed or created.
type Token struct {
	// Standard claims
	Issuer          string    `json:"iss" cel:"issuer"`
	Subject         string    `json:"sub" cel:"subject"`
	Audience        []string  `json:"aud" cel:"aud"`
	AuthorizedParty string    `json:"azp,omitempty" cel:"azp"`
	ExpiresAt       time.Time `json:"exp" cel:"exp"`
	NotBefore       time.Time `json:"nbf" cel:"nbf"`
	IssuedAt        time.Time `json:"iat" cel:"iat"`
	ID              string    `json:"jti,omitempty" cel:"id"`

	// Header derived fields
	Algorithm string `json:"alg" cel:"alg"`
	KeyID     string `json:"kid" cel:"keyId"`

	// Raw JSON payload associated with the token including custom claims.
	// Must be treated as read-only once initialized.
	Payload map[string]any `json:"-" cel:"-"`

	// Raw holds the original token string (without scheme prefix) if parsed via ParseToken.
	Raw string `json:"-" cel:"-"`
}

// IsValidAt checks whether the token time claims (iat, nbf, exp) are valid at the given reference time with clock leeway tolerance.
func (t *Token) IsValidAt(refTime time.Time, leeway time.Duration) bool {
	now := refTime.UTC()
	lateNow := now.Add(leeway)
	earlyNow := now.Add(-leeway)

	// Issued-at time is present and in the future.
	if !t.IssuedAt.IsZero() && t.IssuedAt.Compare(lateNow) > 0 {
		return false
	}
	// Not-before time is present and in the future.
	if !t.NotBefore.IsZero() && t.NotBefore.Compare(lateNow) > 0 {
		return false
	}
	// Expires-at time is present and expiry happened in the past.
	if !t.ExpiresAt.IsZero() && t.ExpiresAt.Compare(earlyNow) <= 0 {
		return false
	}
	// Inverted validity window: nbf <= exp
	if !t.NotBefore.IsZero() && !t.ExpiresAt.IsZero() && t.NotBefore.Compare(t.ExpiresAt) > 0 {
		return false
	}
	// Inverted validity window: iat <= exp
	if !t.IssuedAt.IsZero() && !t.ExpiresAt.IsZero() && t.IssuedAt.Compare(t.ExpiresAt) > 0 {
		return false
	}

	return true
}

// PresentedBy determines whether the token from the given issuer was presented by the expected authorized party (`azp`) or audience (`aud`).
// If the token contains an `azp` claim, it is checked against the `presenter`. Otherwise, the `aud` claim is checked.
func (t *Token) PresentedBy(issuer, presenter string) bool {
	if t.Issuer != issuer {
		return false
	}
	if t.AuthorizedParty != "" {
		return t.AuthorizedParty == presenter
	}
	return slices.Contains(t.Audience, presenter)
}

// Claim queries a claim value by key name using the provided types.Adapter, returning an optional dyn value.
func (t *Token) Claim(adapter types.Adapter, claimName string) ref.Val {
	val, ok := t.Payload[claimName]
	if !ok || val == nil {
		return types.OptionalNone
	}
	refVal := adapter.NativeToValue(val)
	if types.IsError(refVal) {
		return refVal
	}
	return types.OptionalOf(refVal)
}

// NewToken generates a `jwt.Token` instance from the JSON-decoded header and payload of a JWT.
//
// Signature validation of the token must be performed before passing the token to CEL.
// It is recommended that `IsValidAt` and `PresentedBy` are checked after creation of the token
// to ensure the token matches core content assumptions.
func NewToken(header, payload map[string]any) (*Token, error) {
	alg, ok := header["alg"].(string)
	if !ok || alg == "" {
		return nil, fmt.Errorf("missing required header: 'alg'")
	}
	iss, ok := payload["iss"].(string)
	if !ok || iss == "" {
		return nil, fmt.Errorf("missing required claim: 'iss'")
	}
	sub, ok := payload["sub"].(string)
	if !ok || sub == "" {
		return nil, fmt.Errorf("missing required claim: 'sub'")
	}

	var audience []string
	switch a := payload["aud"].(type) {
	case string:
		if a != "" {
			audience = []string{a}
		}
	case []any:
		for _, item := range a {
			s, ok := item.(string)
			if !ok || s == "" {
				return nil, fmt.Errorf("invalid claim 'aud': expected non-empty string in audience list, got %T", item)
			}
			audience = append(audience, s)
		}
	default:
		if payload["aud"] != nil {
			return nil, fmt.Errorf("invalid claim 'aud': expected string or array of strings, got %T", payload["aud"])
		}
	}
	if len(audience) == 0 {
		return nil, fmt.Errorf("missing required claim: 'aud'")
	}

	exp, err := types.ParseTimestamp(payload["exp"])
	if err != nil || exp.IsZero() {
		return nil, fmt.Errorf("missing required claim: 'exp'")
	}
	iat, err := types.ParseTimestamp(payload["iat"])
	if err != nil || iat.IsZero() {
		return nil, fmt.Errorf("missing required claim: 'iat'")
	}
	var nbf time.Time
	if rawNbf, ok := payload["nbf"]; ok && rawNbf != nil {
		parsedNbf, err := types.ParseTimestamp(rawNbf)
		if err != nil {
			return nil, fmt.Errorf("invalid claim 'nbf': %w", err)
		}
		nbf = parsedNbf
	}

	return &Token{
		Algorithm:       alg,
		KeyID:           optString(header, "kid"),
		Issuer:          iss,
		Subject:         sub,
		Audience:        audience,
		AuthorizedParty: optString(payload, "azp"),
		ExpiresAt:       exp,
		IssuedAt:        iat,
		NotBefore:       nbf,
		ID:              optString(payload, "jti"),
		Payload:         payload,
	}, nil
}

// ParseToken parses a JWT token string into a structured Token.
// Verification of the token must be performed before passing the token to CEL.
func ParseToken(tokenStr string) (*Token, error) {
	tokenStr = trimBearerPrefix(tokenStr)
	if len(tokenStr) > maxTokenSize {
		return nil, fmt.Errorf("token size exceeds maximum allowed limit of %d bytes", maxTokenSize)
	}

	parts := strings.SplitN(tokenStr, ".", 4)
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
	tok, err := NewToken(header, payload)
	if err != nil {
		return nil, err
	}
	tok.Raw = tokenStr
	return tok, nil
}

func optString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func trimBearerPrefix(tokenStr string) string {
	tokenStr = strings.TrimSpace(tokenStr)
	lower := strings.ToLower(tokenStr)
	if strings.HasPrefix(lower, "bearer ") {
		return strings.TrimSpace(tokenStr[7:])
	}
	if strings.HasPrefix(lower, "dpop ") {
		return strings.TrimSpace(tokenStr[5:])
	}
	return tokenStr
}

func decodeBase64Segment(seg string) ([]byte, error) {
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
