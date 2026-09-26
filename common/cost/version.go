// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cost

import "math"

const defaultModelVersion uint32 = math.MaxUint32

// ModelOption configures how an OverloadModel is compiled into an estimator or a tracker.
type ModelOption func(*modelOptions)

// modelOptions is the resolved configuration for compiling an OverloadModel.
type modelOptions struct {
	strategy SizingStrategy
	version  uint32
}

// newModelOptions resolves the supplied options over the defaults.
func newModelOptions(opts ...ModelOption) *modelOptions {
	resolved := &modelOptions{
		strategy: DefaultSizingStrategy(),
		version:  defaultModelVersion,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(resolved)
		}
	}
	if resolved.strategy == nil {
		resolved.strategy = DefaultSizingStrategy()
	}
	return resolved
}

// WithSizingStrategy sets the SizingStrategy used to resolve sizes.
//
// A nil strategy is ignored, leaving the default in place.
func WithSizingStrategy(strategy SizingStrategy) ModelOption {
	return func(o *modelOptions) {
		if strategy != nil {
			o.strategy = strategy
		}
	}
}

// ModelVersion pins the revision of the cost model's estimation rules.
//
// If this option is not set, the latest cost model version is used. Revisions exist so that
// corrections to the model can land without silently moving numbers that existing deployments
// budget against. A caller holding estimates recorded under an earlier release can pin to the
// revision those estimates were produced under, review the delta at leisure, and adopt the
// correction deliberately.
//
// Revisions affect estimation only. The cost charged at runtime is not versioned, because a
// revision that moved the charge would change what a given expression is billed rather than what it
// is predicted to be billed.
//
// Version 0 (v0.32.0):
//   - Min() reports a lower bound of 1 for any interval whose upper bound is non-zero, rather than
//     the true minimum of its operands.
//
// Version 1:
//   - Min() is computed as the exact minimum of its operands' intervals. The lower bound moves in
//     both directions relative to version 0: it rises where both operands are large and equally
//     sized, and falls where an operand can legitimately be empty.
//   - A value inside an optional keeps its size. optional.of, optional.ofNonZeroValue and value()
//     report the size they wrap or unwrap, the two `or` forms report the union of the receiver and
//     the alternative, and the optional index forms record the same `@items` and `@values` field
//     path their non-optional counterparts do.
//   - Extension estimators in ext/lists (distinct, sort, sortBy, slice, range, flatten),
//     ext/strings (join, split, substring, replace), and ext/regex (extract, extractAll, replace)
//     align their Min/Max cost and result-size bounds with their runtime cost trackers.
func ModelVersion(version uint32) ModelOption {
	return func(o *modelOptions) {
		o.version = version
	}
}

// versionedEstimateContext is implemented by EstimateContext values that carry a model revision.
//
// It is deliberately unexported: EstimateContext is an interface third parties may implement, and
// adding a method to it would break them. Contexts that do not implement this are treated as
// running at the latest revision, which is the correct default for a context built without any
// knowledge of pinning.
type versionedEstimateContext interface {
	modelVersion() uint32
}

// contextModelVersion returns the revision a context was created under.
func contextModelVersion(ctx EstimateContext) uint32 {
	if v, ok := ctx.(versionedEstimateContext); ok {
		return v.modelVersion()
	}
	return defaultModelVersion
}
