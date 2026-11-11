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

package ext

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/checker"
	"cel.dev/cel-go/common/env"
	"cel.dev/cel-go/common/types"
)

func TestMarkdownParse(t *testing.T) {
	celEnv, err := cel.NewEnv(
		cel.OptionalTypes(),
		Markdown(MarkdownVersion(1)),
		cel.Variable("docBytes", cel.BytesType),
		cel.Variable("badUTF8", cel.BytesType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}

	rawMD := "---\r\n" +
		"name: hr-helper\r\n" +
		"metadata:\r\n" +
		"  owner: sec\r\n" +
		"---\r\n" +
		"# Guide\n" +
		"See [CEL](https://cel.dev/docs) and ![Diagram](https://cel.dev/arch.png \"Arch\").\n" +
		"Also [![Badge](https://cel.dev/b.svg)](https://cel.dev/target.png) and [Logo](https://cel.dev/logo.png).\n" +
		"HTML: <img src=\"https://cel.dev/h.jpg\" alt='H' title=T> <a href=\"https://cel.dev/a.gif\" alt=\"A\">link</a> <a href=\"https://cel.dev/noattr\">plain</a>.\n" +
		"Auto: <https://cel.dev/auto.webp> <https://cel.dev/autolink> and bare https://cel.dev/bare.png?v=1#top, https://cel.dev/bare.\n" +
		"Data image: ![Inline](data:image/png;base64,AAAA).\n" +
		"Inline code: `[not a link](https://ignored.example.com)`.\n" +
		"\n" +
		"[ref-img]: https://cel.dev/ref.tiff\n" +
		"[ref-doc]: https://cel.dev/ref\n" +
		"\n" +
		"> A blockquote\n" +
		"> second line\n" +
		"Paragraph after blockquote.\n" +
		"\n" +
		"- list item\n" +
		"    continued in list\n" +
		"\n" +
		"Paragraph ending list.\n" +
		"\n" +
		"    indented_code_1()\n" +
		"\tindented_code_2()\n" +
		"\n" +
		"After indented code.\n" +
		"\n" +
		"```bad`info\n" +
		"not a fence\n" +
		"```python\n" +
		"print('hi')\n" +
		"```\n"

	vars := map[string]any{
		"docBytes": []byte(rawMD),
		"badUTF8":  []byte{0xff, 0xfe},
	}

	tests := []struct {
		expr string
		want any
	}{
		// markdown.parse(string)
		{
			expr: "markdown.parse('# Hello\\n[CEL](https://cel.dev)').hasValue()",
			want: true,
		},
		{
			expr: "type(markdown.parse('# Hello').value()) == markdown.Content",
			want: true,
		},
		{
			expr: "markdown.parse('# Hello\\n[CEL](https://cel.dev)').value().links[0].url",
			want: "https://cel.dev",
		},
		{
			expr: "markdown.parse('# Hello\\n[CEL](https://cel.dev)').value().links[0].text",
			want: "CEL",
		},
		{
			expr: "markdown.parse('# Hello\\n[CEL](https://cel.dev)').value().sections[0].title",
			want: "Hello",
		},
		// markdown.parse(bytes)
		{
			expr: "markdown.parse(docBytes).value().frontmatter.name",
			want: "hr-helper",
		},
		{
			expr: "markdown.parse(docBytes).value().frontmatter.metadata.owner",
			want: "sec",
		},
		{
			expr: "markdown.parse(docBytes).value().links.exists(l, l.url == 'https://cel.dev/docs' && l.text == 'CEL')",
			want: true,
		},
		{
			expr: "markdown.parse(docBytes).value().images.exists(i, i.src == 'https://cel.dev/arch.png' && i.alt == 'Diagram' && i.title == 'Arch')",
			want: true,
		},
		{
			expr: "markdown.parse(docBytes).value().images.exists(i, i.src == 'https://cel.dev/h.jpg' && i.alt == 'H' && i.title == 'T')",
			want: true,
		},
		{
			expr: "markdown.parse(docBytes).value().images.exists(i, i.src == 'data:image/png;base64,AAAA')",
			want: true,
		},
		{
			expr: "markdown.parse(docBytes).value().code_blocks.size() == 2 && markdown.parse(docBytes).value().code_blocks[0].indented && markdown.parse(docBytes).value().code_blocks[1].language == 'python'",
			want: true,
		},
		{
			expr: "markdown.parse(docBytes).value().sections.exists(s, s.kind == 'blockquote')",
			want: true,
		},
		// Null / empty frontmatter
		{
			expr: "markdown.parse('---\\nnull\\n---\\n# Body').value().frontmatter.size() == 0",
			want: true,
		},
		{
			expr: "markdown.parse('---\\n---\\n# Body').value().frontmatter.size() == 0",
			want: true,
		},
		// Unclosed frontmatter treated as body
		{
			expr: "markdown.parse('---\\nname: unclosed\\n# Body').value().body.startsWith('---')",
			want: true,
		},
		// Unterminated code fence and trailing indented code block
		{
			expr: "markdown.parse('```sh\\necho 1').value().code_blocks[0].language == 'sh'",
			want: true,
		},
		{
			expr: "markdown.parse('    trailing_indented()').value().code_blocks[0].indented",
			want: true,
		},
		// Long document exercising textIndex checkpoints (> 256 bytes)
		{
			expr: "markdown.parse('" + strings.Repeat("a", 600) + "\\n[end](https://cel.dev)').value().links[0].span.line == 2",
			want: true,
		},
		// Malformed frontmatter yields optional.none()
		{
			expr: "markdown.parse('---\\n- not_a_map\\n---\\nBody').hasValue()",
			want: false,
		},
		{
			expr: "markdown.parse('---\\n: : invalid yaml\\n---\\nBody').hasValue()",
			want: false,
		},
		{
			expr: "markdown.parse(b'---\\n- not_a_map\\n---\\nBody').hasValue()",
			want: false,
		},
		// Invalid UTF-8 bytes yields optional.none()
		{
			expr: "markdown.parse(badUTF8).hasValue()",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			ast, iss := celEnv.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", tc.expr, iss.Err())
			}
			prg, err := celEnv.Program(ast)
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", tc.expr, err)
			}
			out, _, err := prg.Eval(vars)
			if err != nil {
				t.Fatalf("Eval(%q) failed: %v", tc.expr, err)
			}
			if types.IsError(out) {
				t.Fatalf("Eval(%q) = %v, want %v", tc.expr, out, tc.want)
			}
			got, err := out.ConvertToNative(reflect.TypeOf(tc.want))
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Eval(%q) = %v (%v), want %v", tc.expr, out, err, tc.want)
			}
		})
	}
}

func TestMarkdownRequiresOptionalLibrary(t *testing.T) {
	if _, err := cel.NewEnv(Markdown()); err == nil || !strings.Contains(err.Error(), "requires the optional library") {
		t.Fatalf("cel.NewEnv(Markdown()) = %v, wanted optional library error", err)
	}
	if _, err := ParseMarkdown(string([]byte{0xff, 0xfe})); err == nil {
		t.Fatal("ParseMarkdown(invalid UTF-8) succeeded, wanted error")
	}
	oversized := strings.Repeat("a", maxMDSize+1)
	if _, err := ParseMarkdown(oversized); err == nil || !strings.Contains(err.Error(), "exceeds maximum allowed limit") {
		t.Fatalf("ParseMarkdown(oversized) = %v, wanted size limit error", err)
	}
	celEnv, err := cel.NewEnv(
		cel.OptionalTypes(),
		Markdown(),
		cel.Variable("bigStr", cel.StringType),
		cel.Variable("bigBytes", cel.BytesType),
	)
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	for _, expr := range []string{"markdown.parse(bigStr)", "markdown.parse(bigBytes)"} {
		ast, iss := celEnv.Compile(expr)
		if iss.Err() != nil {
			t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
		}
		prg, err := celEnv.Program(ast)
		if err != nil {
			t.Fatalf("Program(%q) failed: %v", expr, err)
		}
		_, _, err = prg.Eval(map[string]any{"bigStr": oversized, "bigBytes": []byte(oversized)})
		if err == nil || !strings.Contains(err.Error(), "exceeds maximum allowed limit") {
			t.Fatalf("Eval(%q) = %v, wanted size limit error", expr, err)
		}
	}
	if _, ok := ExtensionOptionFactory(&env.Extension{Name: "markdown", Version: "latest"}); !ok {
		t.Fatal("ExtensionOptionFactory(markdown) returned false, wanted true")
	}
}

func TestMarkdownCostUnbounded(t *testing.T) {
	celEnv, err := cel.NewEnv(cel.OptionalTypes(), Markdown())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	for _, expr := range []string{
		"markdown.parse('# Hello')",
		"markdown.parse(b'# Hello')",
	} {
		t.Run(expr, func(t *testing.T) {
			ast, iss := celEnv.Compile(expr)
			if iss.Err() != nil {
				t.Fatalf("Compile(%q) failed: %v", expr, iss.Err())
			}
			est, err := celEnv.EstimateCost(ast, testCostHintEstimator{})
			if err != nil {
				t.Fatalf("EstimateCost(%q) failed: %v", expr, err)
			}
			wantEst := checker.CostEstimate{Min: 1, Max: math.MaxUint64}
			if est != wantEst {
				t.Errorf("EstimateCost(%q) got %v, wanted %v", expr, est, wantEst)
			}
			prg, err := celEnv.Program(ast, cel.CostTracking(nil))
			if err != nil {
				t.Fatalf("Program(%q) failed: %v", expr, err)
			}
			_, det, err := prg.Eval(cel.NoVars())
			if err != nil {
				t.Fatalf("Eval(%q) failed: %v", expr, err)
			}
			if det.ActualCost() == nil || *det.ActualCost() != math.MaxUint64 {
				t.Errorf("ActualCost(%q) got %v, wanted %d", expr, det.ActualCost(), uint64(math.MaxUint64))
			}
		})
	}
}
