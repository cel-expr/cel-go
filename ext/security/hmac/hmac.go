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

// Package hmac implements CEL extension functions for Hash-based Message Authentication Code (HMAC) verification and computation.
package hmac

import (
	"crypto"
	"crypto/hmac"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// Library returns a cel.EnvOption to configure extended functions for HMAC signature verification and computation.
func Library(options ...Option) cel.EnvOption {
	l := &hmacLib{
		version:          ^uint32(0),
		customAlgorithms: make(map[string]crypto.Hash),
	}
	for _, o := range options {
		l = o(l)
	}
	if len(l.customAlgorithms) == 0 {
		l = CommonAlgorithms()(l)
	}
	return cel.Lib(l)
}

// Option declares a functional operator for configuring HMAC extension library behavior.
type Option func(*hmacLib) *hmacLib

// Version sets the library version for HMAC extensions.
func Version(version uint32) Option {
	return func(l *hmacLib) *hmacLib {
		l.version = version
		return l
	}
}

// MaxPrefixLength sets the maximum signature prefix length to parse during verification.
// Defaults to 20.
func MaxPrefixLength(limit int) Option {
	return func(l *hmacLib) *hmacLib {
		l.maxPrefixLength = limit
		return l
	}
}

// Algorithm registers a crypto.Hash algorithm with optional aliases
// (e.g. Algorithm(crypto.SHA256, "HS256")),
// exposing constant declarations (e.g., hmac.SHA256, hmac.HS256) in CEL and enabling it for HMAC operations.
func Algorithm(h crypto.Hash, aliases ...string) Option {
	return func(l *hmacLib) *hmacLib {
		if l.customAlgorithms == nil {
			l.customAlgorithms = make(map[string]crypto.Hash)
		}
		name := h.String()
		normName := normalizeAlgName(name)
		l.customAlgorithms[normName] = h
		l.customAlgorithms[name] = h
		for _, alias := range aliases {
			l.customAlgorithms[normalizeAlgName(alias)] = h
			l.customAlgorithms[alias] = h
		}

		if normName != "" {
			l.addConstant("hmac."+normName, normName)
		}
		for _, alias := range aliases {
			constAlias := normalizeAlgName(alias)
			if constAlias != "" {
				l.addConstant("hmac."+constAlias, normName)
			}
		}

		return l
	}
}

// CommonAlgorithms registers the most common HMAC hash algorithms (SHA256, SHA384, SHA512, SHA224, SHA512/256, SHA512/224)
// along with their JOSE/JWT aliases (HS256, HS384, HS512, HS224, HS512/256, HS512/224) using Algorithm options by proxy.
func CommonAlgorithms() Option {
	return func(l *hmacLib) *hmacLib {
		opts := []Option{
			Algorithm(crypto.SHA256, "HS256"),
			Algorithm(crypto.SHA384, "HS384"),
			Algorithm(crypto.SHA512, "HS512"),
			Algorithm(crypto.SHA224, "HS224"),
			Algorithm(crypto.SHA512_256, "HS512_256"),
			Algorithm(crypto.SHA512_224, "HS512_224"),
		}
		for _, opt := range opts {
			l = opt(l)
		}
		return l
	}
}

type celConstant struct {
	name string
	val  string
}

type hmacLib struct {
	version          uint32
	maxPrefixLength  int
	customAlgorithms map[string]crypto.Hash
	constants        []celConstant
}

func (l *hmacLib) addConstant(name, val string) {
	for _, c := range l.constants {
		if c.name == name {
			return
		}
	}
	l.constants = append(l.constants, celConstant{name: name, val: val})
}

// LibraryName returns the CEL library identifier string.
func (*hmacLib) LibraryName() string {
	return "cel.lib.ext.security.hmac"
}

// CompileOptions returns environment options for declaring CEL functions and constants.
func (l *hmacLib) CompileOptions() []cel.EnvOption {
	var opts []cel.EnvOption

	for _, c := range l.constants {
		opts = append(opts, cel.Constant(c.name, cel.StringType, types.String(c.val)))
	}

	opts = append(opts,
		cel.Function("hmac.verify",
			cel.Overload("hmac_verify_bytes_bytes_bytes_string",
				[]*cel.Type{cel.BytesType, cel.BytesType, cel.BytesType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					msg := args[0].(types.Bytes)
					sig := args[1].(types.Bytes)
					secret := args[2].(types.Bytes)
					alg := args[3].(types.String)
					return types.Bool(l.verifyBytes(msg, sig, secret, string(alg)))
				}),
			),
			cel.Overload("hmac_verify_string_string_string_string",
				[]*cel.Type{cel.StringType, cel.StringType, cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					msg := args[0].(types.String)
					sig := args[1].(types.String)
					secret := args[2].(types.String)
					alg := args[3].(types.String)
					return types.Bool(l.verifyString(string(msg), string(sig), string(secret), string(alg)))
				}),
			),
		),

		cel.Function("hmac.compute",
			cel.Overload("hmac_compute_bytes_bytes_string",
				[]*cel.Type{cel.BytesType, cel.BytesType, cel.StringType},
				cel.BytesType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					msg := args[0].(types.Bytes)
					secret := args[1].(types.Bytes)
					alg := args[2].(types.String)
					mac, err := l.compute(msg, secret, string(alg))
					if err != nil {
						return types.ValOrErr(args[0], "%v", err)
					}
					return types.Bytes(mac)
				}),
			),
			cel.Overload("hmac_compute_string_string_string",
				[]*cel.Type{cel.StringType, cel.StringType, cel.StringType},
				cel.BytesType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					msg := args[0].(types.String)
					secret := args[1].(types.String)
					alg := args[2].(types.String)
					mac, err := l.compute([]byte(string(msg)), []byte(string(secret)), string(alg))
					if err != nil {
						return types.ValOrErr(args[0], "%v", err)
					}
					return types.Bytes(mac)
				}),
			),
		),

		cel.Function("hmac.digest",
			cel.Overload("hmac_digest_bytes_string",
				[]*cel.Type{cel.BytesType, cel.StringType},
				cel.BytesType,
				cel.BinaryBinding(func(msgVal, algVal ref.Val) ref.Val {
					msg := msgVal.(types.Bytes)
					alg := algVal.(types.String)
					sum, err := l.digest(msg, string(alg))
					if err != nil {
						return types.ValOrErr(msgVal, "%v", err)
					}
					return types.Bytes(sum)
				}),
			),
			cel.Overload("hmac_digest_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BytesType,
				cel.BinaryBinding(func(msgVal, algVal ref.Val) ref.Val {
					msg := msgVal.(types.String)
					alg := algVal.(types.String)
					sum, err := l.digest([]byte(string(msg)), string(alg))
					if err != nil {
						return types.ValOrErr(msgVal, "%v", err)
					}
					return types.Bytes(sum)
				}),
			),
		),

		cel.Function("hmac.equal",
			cel.Overload("hmac_equal_bytes_bytes",
				[]*cel.Type{cel.BytesType, cel.BytesType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x := lhs.(types.Bytes)
					y := rhs.(types.Bytes)
					return types.Bool(hmac.Equal(x, y))
				}),
			),
		),
	)

	// The digest and equal functions are unbounded in cost: the work is driven by
	// input sizes the checker cannot bound, so they are reported as unknown at
	// check time and as the maximum cost at runtime.
	estimators := make([]cost.CostOption, 0, len(unboundedCostOverloads))
	for _, overloadID := range unboundedCostOverloads {
		estimators = append(estimators, cost.OverloadCostEstimate(overloadID, estimateUnboundedCost))
	}
	opts = append(opts, cel.CostEstimatorOptions(estimators...))

	return opts
}

// ProgramOptions returns program options for HMAC extensions.
func (l *hmacLib) ProgramOptions() []cel.ProgramOption {
	trackers := make([]cost.TrackerOption, 0, len(unboundedCostOverloads))
	for _, overloadID := range unboundedCostOverloads {
		trackers = append(trackers, cost.OverloadTracker(overloadID, trackUnboundedCost))
	}
	return []cel.ProgramOption{cel.CostTrackerOptions(trackers...)}
}

// unboundedCostOverloads are the overload identifiers reported as having an
// unbounded estimated and actual cost.
var unboundedCostOverloads = []string{
	"hmac_digest_bytes_string",
	"hmac_digest_string_string",
	"hmac_equal_bytes_bytes",
}

// estimateUnboundedCost reports an unknown cost estimate, whose maximum is
// math.MaxUint64, for the overload under estimation.
func estimateUnboundedCost(estimator cost.Estimator, target *cost.AstNode, args []cost.AstNode) *cost.CallEstimate {
	return &cost.CallEstimate{CostEstimate: cost.UnknownCostEstimate()}
}

// trackUnboundedCost reports the maximum actual cost for the overload invoked.
func trackUnboundedCost(args []ref.Val, result ref.Val) *uint64 {
	maxCost := uint64(math.MaxUint64)
	return &maxCost
}

func (l *hmacLib) compute(msg, secret []byte, alg string) ([]byte, error) {
	hType, err := l.resolveHash(alg)
	if err != nil {
		return nil, err
	}
	return computeHMAC(msg, secret, hType)
}

func (l *hmacLib) digest(msg []byte, alg string) ([]byte, error) {
	hType, err := l.resolveHash(alg)
	if err != nil {
		return nil, err
	}
	return computeDigest(msg, hType)
}

func (l *hmacLib) verifyBytes(msg, sig, secret []byte, alg string) bool {
	hType, err := l.resolveHash(alg)
	if err != nil {
		return false
	}
	expectedMAC, err := computeHMAC(msg, secret, hType)
	if err != nil {
		return false
	}
	return hmac.Equal(expectedMAC, sig)
}

func (l *hmacLib) verifyString(msgStr, sigStr, secretStr, alg string) bool {
	sigStr = strings.TrimSpace(sigStr)
	detectedAlg, cleanSig := l.parseSignaturePrefix(sigStr)
	effectiveAlg := alg
	if detectedAlg != "" {
		effectiveAlg = detectedAlg
	}

	hType, err := l.resolveHash(effectiveAlg)
	if err != nil {
		return false
	}

	expectedMAC, err := computeHMAC([]byte(msgStr), []byte(secretStr), hType)
	if err != nil {
		return false
	}

	// Try hex decoding
	if hexBytes, err := hex.DecodeString(cleanSig); err == nil && len(hexBytes) == len(expectedMAC) {
		if hmac.Equal(expectedMAC, hexBytes) {
			return true
		}
	}

	// Try base64 standard decoding
	if b64Bytes, err := decodeBase64StdSegment(cleanSig); err == nil && len(b64Bytes) == len(expectedMAC) {
		if hmac.Equal(expectedMAC, b64Bytes) {
			return true
		}
	}

	// Try base64 URL decoding
	if b64URLBytes, err := decodeBase64URLSegment(cleanSig); err == nil && len(b64URLBytes) == len(expectedMAC) {
		if hmac.Equal(expectedMAC, b64URLBytes) {
			return true
		}
	}

	// Fallback raw string comparison
	return hmac.Equal(expectedMAC, []byte(cleanSig))
}

func (l *hmacLib) parseSignaturePrefix(sig string) (string, string) {
	sig = strings.TrimSpace(sig)
	limit := l.maxPrefixLength
	if limit <= 0 {
		limit = 20
	}
	if idx := strings.Index(sig, "="); idx > 0 && idx < limit {
		prefix := strings.TrimSpace(sig[:idx])
		rest := strings.TrimSpace(sig[idx+1:])

		normPrefix := normalizeAlgName(prefix)
		if _, ok := l.customAlgorithms[normPrefix]; ok {
			for name := range l.customAlgorithms {
				if normalizeAlgName(name) == normPrefix {
					return name, rest
				}
			}
		}
		if _, ok := l.customAlgorithms[prefix]; ok {
			return prefix, rest
		}

		if strings.EqualFold(prefix, "v1") || strings.EqualFold(prefix, "v0") {
			return "", rest
		}
	}
	return "", sig
}

func normalizeAlgName(alg string) string {
	s := strings.TrimSpace(alg)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ToUpper(s)
	if after, ok := strings.CutPrefix(s, "SHA_"); ok {
		s = "SHA" + after
	}
	return s
}

func (l *hmacLib) resolveHash(alg string) (crypto.Hash, error) {
	norm := normalizeAlgName(alg)
	for name, h := range l.customAlgorithms {
		if strings.EqualFold(alg, name) || norm == normalizeAlgName(name) {
			return h, nil
		}
	}

	return 0, fmt.Errorf("unsupported HMAC hash algorithm: %q", alg)
}

func computeHMAC(msg, secret []byte, hType crypto.Hash) ([]byte, error) {
	if !hType.Available() {
		return nil, fmt.Errorf("hash algorithm %v is not available", hType)
	}
	mac := hmac.New(hType.New, secret)
	mac.Write(msg)
	return mac.Sum(nil), nil
}

func computeDigest(msg []byte, hType crypto.Hash) ([]byte, error) {
	if !hType.Available() {
		return nil, fmt.Errorf("hash algorithm %v is not available", hType)
	}
	h := hType.New()
	h.Write(msg)
	return h.Sum(nil), nil
}

// decodeBase64URLSegment decodes a URL-safe base64 string with or without padding.
func decodeBase64URLSegment(seg string) ([]byte, error) {
	seg = strings.TrimSpace(seg)
	if data, err := base64.RawURLEncoding.DecodeString(seg); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(seg)
}

// decodeBase64StdSegment decodes a standard base64 string with or without padding.
func decodeBase64StdSegment(seg string) ([]byte, error) {
	seg = strings.TrimSpace(seg)
	if data, err := base64.RawStdEncoding.DecodeString(seg); err == nil {
		return data, nil
	}
	return base64.StdEncoding.DecodeString(seg)
}
