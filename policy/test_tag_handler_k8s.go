// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"go.yaml.in/yaml/v3"
)

// K8sTestTagHandler returns a TagVisitor which handles custom policy tags used in K8s policies. This is
// a helper function to be used in tests.
func K8sTestTagHandler() TagVisitor {
	return k8sAdmissionTagHandler{TagVisitor: DefaultTagVisitor()}
}

type k8sAdmissionTagHandler struct {
	TagVisitor
}

func (k8sAdmissionTagHandler) PolicyTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, policy *Policy) {
	switch tagName {
	case "kind":
		policy.SetMetadata("kind", ctx.NewString(node).Value)
	case "metadata":
		m := k8sMetadata{}
		if err := node.Decode(&m); err != nil {
			ctx.ReportErrorAtID(id, "invalid yaml metadata node: %v, error: %v", node, err)
			return
		}
	case "spec":
		spec := ctx.ParseRule(ctx, policy, node)
		policy.SetRule(spec)
	default:
		ctx.ReportErrorAtID(id, "unsupported policy tag: %s", tagName)
	}
}

func (k8sAdmissionTagHandler) RuleTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, policy *Policy, r *Rule) {
	switch tagName {
	case "failurePolicy":
		policy.SetMetadata(tagName, ctx.NewString(node).Value)
	case "matchConstraints":
		m := k8sMatchConstraints{}
		if err := node.Decode(&m); err != nil {
			ctx.ReportErrorAtID(id, "invalid yaml matchConstraints node: %v, error: %v", node, err)
			return
		}
	case "validations":
		id := ctx.CollectMetadata(node)
		if node.LongTag() != "tag:yaml.org,2002:seq" {
			ctx.ReportErrorAtID(id, "invalid 'validations' type, expected list got: %s", node.LongTag())
			return
		}
		for _, val := range node.Content {
			r.AddMatch(ctx.ParseMatch(ctx, policy, val))
		}
	default:
		ctx.ReportErrorAtID(id, "unsupported rule tag: %s", tagName)
	}
}

func (k8sAdmissionTagHandler) MatchTag(ctx ParserContext, id int64, tagName string, node *yaml.Node, policy *Policy, m *Match) {
	if m.Output().Value == "" {
		m.SetOutput(ValueString{Value: "'invalid admission request'"})
	}
	switch tagName {
	case "expression":
		// The K8s expression to validate must return false in order to generate a violation message.
		condition := ctx.NewString(node)
		condition.Value = "!(" + condition.Value + ")"
		m.SetCondition(condition)
	case "messageExpression":
		m.SetOutput(ctx.NewString(node))
	}
}

type k8sMetadata struct {
	Name string `yaml:"name"`
}

type k8sMatchConstraints struct {
	ResourceRules []k8sResourceRule `yaml:"resourceRules"`
}

type k8sResourceRule struct {
	APIGroups   []string `yaml:"apiGroups"`
	APIVersions []string `yaml:"apiVersions"`
	Operations  []string `yaml:"operations"`
}
