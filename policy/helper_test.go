// Copyright 2024 Google LLC
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
	"log"
	"os"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/test"

	"go.yaml.in/yaml/v3"

)

var (
	policyTests = []struct {
		name      string
		envOpts   []cel.EnvOption
		parseOpts []ParserOption
		expr      string
	}{
		{
			name: "k8s",
			parseOpts: []ParserOption{func(p *Parser) (*Parser, error) {
				p.TagVisitor = K8sTestTagHandler()
				return p, nil
			}},
			expr: `
	cel.@block([
	  resource.labels.?environment.orValue("prod"),
	  resource.labels.?break_glass.orValue("false") == "true"],
	  !(@index1 || resource.containers.all(c, c.startsWith(@index0 + ".")))
	    ? optional.of("only %s containers are allowed in namespace %s".format([@index0, resource.namespace]))
	    : optional.none())`,
		},
		{
			name: "unnest",
			expr: `
	cel.@block([values.filter(x, x > 2)],
	((@index0.size() == 0) ? false : @index0.all(x, x % 2 == 0))
	? optional.of("some divisible by 2")
	: (values.map(x, x * 3).exists(x, x % 4 == 0)
	   ? optional.of("at least one divisible by 4")
	   : (values.map(x, x * x * x).exists(x, x % 6 == 0)
	     ? optional.of("at least one power of 6")
		 : optional.none())))
			`,
		},
		{
			name: "restricted_destinations",
			expr: `
    cel.@block([
	  locationCode(origin.ip) == spec.origin,
	  has(request.auth.claims.nationality),
	  @index1 && request.auth.claims.nationality == spec.origin,
	  locationCode(destination.ip) in spec.restricted_destinations,
	  resource.labels.location in spec.restricted_destinations,
	  @index3 || @index4],
	  (@index2 && @index5) ? true : ((!@index1 && @index0 && @index5) ? true : false))`,
			envOpts: []cel.EnvOption{
				cel.Function("locationCode",
					cel.Overload("locationCode_string", []*cel.Type{cel.StringType}, cel.StringType,
						cel.UnaryBinding(func(ip ref.Val) ref.Val {
							switch ip.(types.String) {
							case types.String("10.0.0.1"):
								return types.String("us")
							case types.String("10.0.0.2"):
								return types.String("de")
							default:
								return types.String("ir")
							}
						}))),
			},
		},
		{
			name: "limits",
			expr: `
    cel.@block([
	  "hello",
	  "goodbye",
	  "me",
	  "%s, %s",
	  @index3.format([@index1, @index2])],
	  (now.getHours() >= 20)
	  ? ((now.getHours() < 21)
	    ? optional.of(@index4 + "!")
		: ((now.getHours() < 22)
		  ? optional.of(@index4 + "!!")
		  : ((now.getHours() < 24)
		    ? optional.of(@index4 + "!!!")
			: optional.none())))
	  : optional.of(@index3.format([@index0, @index2])))`,
		},
		{
			name: "nested_rules_unconditional_chaining",
			expr: `
	cel.@block([3],
	((x > @index0) ? optional.of("a") : ((x == @index0) ? optional.of("b") : optional.none()))
	  .orValue("c"))`,
		},
		{
			name: "nested_rules_unconditional_chaining_optional",
			expr: `
	cel.@block([3],
	((x > @index0) ? optional.of("a") : ((x == @index0) ? optional.of("b") : optional.none()))
	  .or((x == 1) ? optional.of("c") : optional.none()))`,
		},
		{
			name: "nested_rules_unwrap_rewrap",
			expr: `
	(x == 1)
	  ? optional.of(((y == 1) ? optional.of("a") : optional.none()).orValue("b"))
	  : optional.none()`,
		},
	}

	composerUnnestTests = []struct {
		name         string
		expr         string
		composed     string
		composerOpts []ComposerOption
		outputType   *cel.Type
	}{
		{
			name:         "unnest",
			composerOpts: []ComposerOption{ExpressionUnnestHeight(2)},
			composed: `
	cel.@block([
	  values.filter(x, x > 2),
	  @index0.size() == 0,
	  @index1 ? false : @index0.all(x, x % 2 == 0),
	  values.map(x, x * x * x).exists(x, x % 6 == 0)
	    ? optional.of("at least one power of 6")
		: optional.none(),
	  values.map(x, x * 3).exists(x, x % 4 == 0)
	    ? optional.of("at least one divisible by 4")
		: @index3],
	  @index2 ? optional.of("some divisible by 2") : @index4)
			`,
			outputType: cel.OptionalType(cel.StringType),
		},

		{
			name:         "limits",
			composerOpts: []ComposerOption{ExpressionUnnestHeight(3)},
			composed: `
	cel.@block([
		"hello",
		"goodbye",
		"me",
		"%s, %s",
		@index3.format([@index1, @index2]),
		(now.getHours() < 24) ? optional.of(@index4 + "!!!") : optional.none(),
		optional.of(@index3.format([@index0, @index2]))],
		(now.getHours() >= 20)
		? ((now.getHours() < 21) ? optional.of(@index4 + "!") :
		  ((now.getHours() < 22) ? optional.of(@index4 + "!!") : @index5))
		: @index6)`,
			outputType: cel.OptionalType(cel.StringType),
		},
		{
			name:         "limits",
			composerOpts: []ComposerOption{ExpressionUnnestHeight(4)},
			composed: `
	cel.@block([
		"hello",
		"goodbye",
		"me",
		"%s, %s",
		@index3.format([@index1, @index2]),
		(now.getHours() < 22) ? optional.of(@index4 + "!!") :
		((now.getHours() < 24) ? optional.of(@index4 + "!!!") : optional.none())],
		(now.getHours() >= 20)
		? ((now.getHours() < 21) ? optional.of(@index4 + "!") : @index5)
		: optional.of(@index3.format([@index0, @index2])))
		`,
			outputType: cel.OptionalType(cel.StringType),
		},
		{
			name:         "limits",
			composerOpts: []ComposerOption{ExpressionUnnestHeight(5)},
			composed: `
	cel.@block([
		"hello",
		"goodbye",
		"me",
		"%s, %s",
		@index3.format([@index1, @index2]),
		(now.getHours() < 21) ? optional.of(@index4 + "!") :
		((now.getHours() < 22) ? optional.of(@index4 + "!!") :
		((now.getHours() < 24) ? optional.of(@index4 + "!!!") : optional.none()))],
		(now.getHours() >= 20) ? @index5 : optional.of(@index3.format([@index0, @index2])))`,
			outputType: cel.OptionalType(cel.StringType),
		},
	}

	policyErrorTests = []struct {
		name         string
		err          string
		compilerOpts []CompilerOption
	}{
		{
			name: "errors",
			err: `ERROR: testdata/errors/policy.yaml:19:1: error configuring import: invalid qualified name: punc.Import!, wanted name of the form 'qualified.name'
 |       punc.Import!
 | ^
ERROR: testdata/errors/policy.yaml:20:12: error configuring import: invalid qualified name: bad import, wanted name of the form 'qualified.name'
 |   - name: "bad import"
 | ...........^
ERROR: testdata/errors/policy.yaml:24:19: undeclared reference to 'spec' (in container '')
 |       expression: spec.labels
 | ..................^
ERROR: testdata/errors/policy.yaml:25:7: invalid variable declaration: overlapping identifier for name 'variables.want'
 |     - name: want
 | ......^
ERROR: testdata/errors/policy.yaml:28:50: Syntax error: mismatched input 'resource' expecting ')'
 |       expression: variables.want.filter(l, !(lin resource.labels))
 | .................................................^
ERROR: testdata/errors/policy.yaml:28:66: Syntax error: extraneous input ')' expecting <EOF>
 |       expression: variables.want.filter(l, !(lin resource.labels))
 | .................................................................^
ERROR: testdata/errors/policy.yaml:30:27: Syntax error: mismatched input '2' expecting {'}', ','}
 |       expression: "{1:305 2:569}"
 | ..........................^
ERROR: testdata/errors/policy.yaml:38:75: Syntax error: extraneous input ']' expecting ')'
 |         "missing one or more required labels: %s".format(variables.missing])
 | ..........................................................................^
ERROR: testdata/errors/policy.yaml:41:67: undeclared reference to 'format' (in container '')
 |         "invalid values provided on one or more labels: %s".format([variables.invalid])
 | ..................................................................^
ERROR: testdata/errors/policy.yaml:45:16: incompatible output types: block has output type string, but previous outputs have type bool
 |       output: "'false'"
 | ...............^`,
		},
		{
			name: "limits",
			err: `ERROR: testdata/limits/policy.yaml:22:14: variable exceeds nested expression limit
 |     - name: "person"
 | .............^`,
			compilerOpts: []CompilerOption{MaxNestedExpressions(2)},
		},
		{
			name: "limits",
			err: `ERROR: testdata/limits/policy.yaml:30:9: rule exceeds nested expression limit
 |         id: "farewells"
 | ........^`,
			compilerOpts: []CompilerOption{MaxNestedExpressions(5)},
		},
		{
			name: "errors_unreachable",
			err: `ERROR: testdata/errors_unreachable/policy.yaml:28:9: rule creates unreachable outputs
 |         match:
 | ........^
ERROR: testdata/errors_unreachable/policy.yaml:36:13: match creates unreachable outputs
 |           - output: |
 | ............^
ERROR: testdata/errors_unreachable/policy.yaml:38:13: Condition is always false
 |           - condition: "false"
 | ............^`,
		},
		{
			name: "nested_incompatible_outputs",
			err: `ERROR: testdata/nested_incompatible_outputs/policy.yaml:22:9: incompatible output types: block has output type string, but previous outputs have type bool
 |         match:
 | ........^`,
		},
		{
			name: "aggregate_errors",
			err: `ERROR: testdata/aggregate_errors/policy.yaml:21:13: match creates unreachable outputs
 |           - condition: "true"
 | ............^
ERROR: testdata/aggregate_errors/policy.yaml:24:22: incompatible output types: block has output type int, but previous outputs have type optional_type(string)
 |             output: "403"
 | .....................^`,
		},
		{
			name: "aggregate_list_errors",
			err: `ERROR: testdata/aggregate_list_errors/policy.yaml:21:13: match creates unreachable outputs
 |           - condition: "true"
 | ............^
ERROR: testdata/aggregate_list_errors/policy.yaml:24:22: incompatible output types: block has output type int, but previous outputs have type list(string)
 |             output: "403"
 | .....................^`,
		},
		{
			name: "aggregate_nested_mixed_semantics",
			err: `ERROR: testdata/aggregate_nested_mixed_semantics/policy.yaml:23:15: nested aggregate rules are not allowed
 |               aggregate:
 | ..............^`,
		},
	}
)

func readPolicy(t testing.TB, fileName string) *Source {
	t.Helper()
	policyBytes, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) failed: %v", fileName, err)
	}
	return ByteSource(policyBytes, fileName)
}

func readPolicyConfig(t testing.TB, fileName string) *env.Config {
	t.Helper()
	testCaseBytes, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) failed: %v", fileName, err)
	}
	config := &env.Config{}
	err = yaml.Unmarshal(testCaseBytes, config)
	if err != nil {
		log.Fatalf("yaml.Unmarshal(%s) error: %v", fileName, err)
	}
	return config
}

func readTestSuite(t testing.TB, fileName string) *test.Suite {
	t.Helper()
	testCaseBytes, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) failed: %v", fileName, err)
	}
	suite := &test.Suite{}
	err = yaml.Unmarshal(testCaseBytes, suite)
	if err != nil {
		log.Fatalf("yaml.Unmarshal(%s) error: %v", fileName, err)
	}
	return suite
}
