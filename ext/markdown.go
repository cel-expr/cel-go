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
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"go.yaml.in/yaml/v3"
)

// MaxMarkdownFrontmatterBytes bounds the size of the YAML frontmatter block recognized by
// markdown.parse.
const MaxMarkdownFrontmatterBytes = 64 << 10

const maxMDSize = maxParseSize // 10MB maximum allowed Markdown input size

// Markdown section kinds. Sections partition MarkdownDocument.Body: every byte belongs to
// exactly one section.
const (
	MarkdownSectionText       = "text"
	MarkdownSectionHeading    = "heading"
	MarkdownSectionBlockquote = "blockquote"
	MarkdownSectionCodeBlock  = "code_block"
)

// Markdown returns a cel.EnvOption which declares the `markdown.Content` type and the
// `markdown.parse` functions for parsing Markdown text or `bytes` into structured
// content with YAML frontmatter, body text, sections, code blocks, links, and images.
//
// This library requires `cel.OptionalTypes()`.
//
// # Markdown.Parse
//
//	markdown.parse(<string>) -> <optional_type(markdown.Content)>
//	markdown.parse(<bytes>) -> <optional_type(markdown.Content)>
//
// Parses a Markdown string or `bytes` value into an optional `markdown.Content`.
// If the input begins with a `---` YAML frontmatter block closed by another `---` line within
// 64 KiB, the frontmatter is decoded into `frontmatter` (`map(string, dyn)`) and stripped from
// `body`. If the frontmatter block is malformed YAML, not a mapping, or if a `bytes` input is not
// valid UTF-8, `markdown.parse` returns `optional.none()`.
//
// Examples:
//
//	markdown.parse('# Hello\n[CEL](https://cel.dev)').value().links[0].url == 'https://cel.dev'
//	markdown.parse('---\nname: hr\n---\n# HR').value().frontmatter.name == 'hr'
//	markdown.parse(b'# Hello').value().body == '# Hello'
func Markdown(options ...MarkdownOption) cel.EnvOption {
	l := &markdownLib{}
	for _, o := range options {
		l = o(l)
	}
	return cel.Lib(l)
}

// MarkdownOption configures the Markdown extension library.
type MarkdownOption func(*markdownLib) *markdownLib

// MarkdownVersion sets the library version for the Markdown extension.
func MarkdownVersion(version uint32) MarkdownOption {
	return func(lib *markdownLib) *markdownLib {
		lib.version = version
		return lib
	}
}

var (
	// MarkdownContentType is the CEL type of parsed Markdown content.
	MarkdownContentType = cel.ObjectType("markdown.Content")
	// MarkdownDocumentType is an alias for MarkdownContentType.
	MarkdownDocumentType = MarkdownContentType
	// MarkdownSpanType is the CEL type of a half-open character and byte interval in a Markdown body.
	MarkdownSpanType = cel.ObjectType("markdown.Span")
	// MarkdownLinkType is the CEL type of an extracted Markdown or HTML link.
	MarkdownLinkType = cel.ObjectType("markdown.Link")
	// MarkdownImageType is the CEL type of an extracted Markdown or HTML image.
	MarkdownImageType = cel.ObjectType("markdown.Image")
	// MarkdownCodeBlockType is the CEL type of an extracted fenced or indented code block.
	MarkdownCodeBlockType = cel.ObjectType("markdown.CodeBlock")
	// MarkdownSectionType is the CEL type of a structural section of a Markdown body.
	MarkdownSectionType = cel.ObjectType("markdown.Section")
)

// MarkdownSpan is a half-open interval [Start, Stop) within MarkdownContent.Body.
// Start and Stop are 0-indexed Unicode code point (rune) offsets; StartByte and StopByte are
// UTF-8 byte offsets; Line and Column are 1-indexed rune coordinates of Start.
type MarkdownSpan struct {
	Start     int64 `json:"start" cel:"start"`
	Stop      int64 `json:"stop" cel:"stop"`
	StartByte int64 `json:"start_byte" cel:"start_byte"`
	StopByte  int64 `json:"stop_byte" cel:"stop_byte"`
	Line      int64 `json:"line,omitempty" cel:"line"`
	Column    int64 `json:"column,omitempty" cel:"column"`
}

// MarkdownLink is a hyperlink extracted from MarkdownContent.Body.
type MarkdownLink struct {
	URL  string       `json:"url" cel:"url"`
	Text string       `json:"text" cel:"text"`
	Span MarkdownSpan `json:"span" cel:"span"`
}

// MarkdownImage is an image extracted from MarkdownContent.Body.
type MarkdownImage struct {
	Src   string       `json:"src" cel:"src"`
	Alt   string       `json:"alt" cel:"alt"`
	Title string       `json:"title,omitempty" cel:"title"`
	Span  MarkdownSpan `json:"span" cel:"span"`
}

// MarkdownCodeBlock is a fenced or indented code block extracted from MarkdownContent.Body.
type MarkdownCodeBlock struct {
	ID       string       `json:"id" cel:"id"`
	Language string       `json:"language" cel:"language"`
	Info     string       `json:"info,omitempty" cel:"info"`
	Span     MarkdownSpan `json:"span" cel:"span"`
	CodeSpan MarkdownSpan `json:"code_span" cel:"code_span"`
	Indented bool         `json:"indented,omitempty" cel:"indented"`
}

// MarkdownSection is a contiguous structural region of MarkdownContent.Body.
type MarkdownSection struct {
	ID    string       `json:"id" cel:"id"`
	Kind  string       `json:"kind" cel:"kind"`
	Title string       `json:"title,omitempty" cel:"title"`
	Span  MarkdownSpan `json:"span" cel:"span"`
}

// MarkdownContent represents parsed Markdown content.
type MarkdownContent struct {
	Frontmatter map[string]any      `json:"frontmatter,omitempty" cel:"frontmatter"`
	Body        string              `json:"body" cel:"body"`
	Links       []MarkdownLink      `json:"links,omitempty" cel:"links"`
	Images      []MarkdownImage     `json:"images,omitempty" cel:"images"`
	CodeBlocks  []MarkdownCodeBlock `json:"code_blocks,omitempty" cel:"code_blocks"`
	Sections    []MarkdownSection   `json:"sections,omitempty" cel:"sections"`
}

// MarkdownDocument is an alias for MarkdownContent.
type MarkdownDocument = MarkdownContent

const (
	markdownParseStringOverload = "markdown_parse_string"
	markdownParseBytesOverload  = "markdown_parse_bytes"
)

type markdownLib struct {
	version uint32
}

// LibraryName implements the cel.SingletonLibrary interface method.
func (*markdownLib) LibraryName() string {
	return "cel.lib.ext.markdown"
}

// CompileOptions implements the cel.Library interface method.
func (*markdownLib) CompileOptions() []cel.EnvOption {
	var adapt types.Adapter = types.DefaultTypeAdapter
	return []cel.EnvOption{
		func(env *cel.Env) (*cel.Env, error) {
			if !env.HasLibrary("cel.lib.optional") {
				return nil, fmt.Errorf("markdown library requires the optional library")
			}
			p, a, _ := types.ComposeTypes(env.CELTypeProvider(), env.CELTypeAdapter(),
				types.ParseStructTags(true),
				types.NativeTypeFor[MarkdownContent](types.NativeTypeAlias("markdown.Content"), types.ParseStructTags(true)),
				types.NativeTypeFor[MarkdownSpan](types.NativeTypeAlias("markdown.Span"), types.ParseStructTags(true)),
				types.NativeTypeFor[MarkdownLink](types.NativeTypeAlias("markdown.Link"), types.ParseStructTags(true)),
				types.NativeTypeFor[MarkdownImage](types.NativeTypeAlias("markdown.Image"), types.ParseStructTags(true)),
				types.NativeTypeFor[MarkdownCodeBlock](types.NativeTypeAlias("markdown.CodeBlock"), types.ParseStructTags(true)),
				types.NativeTypeFor[MarkdownSection](types.NativeTypeAlias("markdown.Section"), types.ParseStructTags(true)),
			)
			adapt = a
			env, _ = cel.CustomTypeAdapter(a)(env)
			return cel.CustomTypeProvider(p)(env)
		},
		cel.Function("markdown.parse",
			cel.Overload(markdownParseStringOverload,
				[]*cel.Type{cel.StringType},
				cel.OptionalType(MarkdownContentType),
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					raw := string(val.(types.String))
					if len(raw) > maxMDSize {
						return types.NewErr("markdown parse error: input size exceeds maximum allowed limit of %d bytes", maxMDSize)
					}
					doc, err := ParseMarkdown(raw)
					if err != nil {
						return types.OptionalNone
					}
					return types.OptionalOf(adapt.NativeToValue(doc))
				})),
			cel.Overload(markdownParseBytesOverload,
				[]*cel.Type{cel.BytesType},
				cel.OptionalType(MarkdownContentType),
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					b := []byte(val.(types.Bytes))
					if len(b) > maxMDSize {
						return types.NewErr("markdown parse error: input size exceeds maximum allowed limit of %d bytes", maxMDSize)
					}
					strVal := val.ConvertToType(types.StringType)
					if types.IsError(strVal) {
						return types.OptionalNone
					}
					doc, err := ParseMarkdown(string(strVal.(types.String)))
					if err != nil {
						return types.OptionalNone
					}
					return types.OptionalOf(adapt.NativeToValue(doc))
				})),
		),
		cel.CostModel(
			unboundedCostModel(markdownParseStringOverload),
			unboundedCostModel(markdownParseBytesOverload),
		),
	}
}

// ProgramOptions implements the cel.Library interface method.
func (*markdownLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// ParseMarkdown parses raw Markdown text into a MarkdownDocument. It returns an error if raw
// exceeds maxMDSize, is not valid UTF-8, or if a leading `---` YAML frontmatter block is
// malformed or not a YAML mapping.
func ParseMarkdown(raw string) (*MarkdownDocument, error) {
	if len(raw) > maxMDSize {
		return nil, fmt.Errorf("markdown parse error: input size exceeds maximum allowed limit of %d bytes", maxMDSize)
	}
	if !utf8.ValidString(raw) {
		return nil, errors.New("markdown parse error: input is not valid UTF-8")
	}
	front, body := splitMarkdownFrontmatter(raw)
	fm := map[string]any{}
	if front != "" {
		parsedFM, err := parseMarkdownFrontmatterMap(front)
		if err != nil {
			return nil, fmt.Errorf("markdown parse error: invalid frontmatter: %w", err)
		}
		fm = parsedFM
	}
	ix := newMdTextIndex(body)
	x := &mdExtractor{
		body:    body,
		ix:      ix,
		code:    make([]bool, len(body)),
		claimed: make([]bool, len(body)),
	}
	x.scanBlocks()
	x.extractInline()
	return &MarkdownDocument{
		Frontmatter: fm,
		Body:        body,
		Links:       x.links,
		Images:      x.images,
		CodeBlocks:  x.codeBlocks,
		Sections:    x.sections,
	}, nil
}

func splitMarkdownFrontmatter(raw string) (front, body string) {
	rest, found := strings.CutPrefix(raw, "---\n")
	if !found {
		if rest, found = strings.CutPrefix(raw, "---\r\n"); !found {
			return "", raw
		}
	}
	for off := 0; off <= len(rest) && off <= MaxMarkdownFrontmatterBytes; {
		line, next := rest[off:], len(rest)
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line, next = line[:i], off+i+1
		}
		if strings.TrimSuffix(line, "\r") == "---" {
			return rest[:off], rest[next:]
		}
		if next == len(rest) {
			break
		}
		off = next
	}
	return "", raw
}

func parseMarkdownFrontmatterMap(front string) (map[string]any, error) {
	dec := yaml.NewDecoder(strings.NewReader(front))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return map[string]any{}, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("frontmatter must be a YAML mapping")
	}
	var obj map[string]any
	_ = root.Decode(&obj)
	return cleanYAMLObj(obj).(map[string]any), nil
}

const mdIndexStride = 256

type mdTextIndex struct {
	text       string
	lineBytes  []int
	lineRunes  []int
	cpBytes    []int
	cpRunes    []int
	totalRunes int
}

func newMdTextIndex(text string) *mdTextIndex {
	ix := &mdTextIndex{text: text, lineBytes: []int{0}, lineRunes: []int{0}, cpBytes: []int{0}, cpRunes: []int{0}}
	runes := 0
	for i, r := range text {
		for len(ix.cpBytes)*mdIndexStride <= i {
			ix.cpBytes = append(ix.cpBytes, i)
			ix.cpRunes = append(ix.cpRunes, runes)
		}
		runes++
		if r == '\n' {
			ix.lineBytes = append(ix.lineBytes, i+1)
			ix.lineRunes = append(ix.lineRunes, runes)
		}
	}
	ix.totalRunes = runes
	return ix
}

func (ix *mdTextIndex) lineOfByte(b int) int {
	return sort.Search(len(ix.lineBytes), func(i int) bool { return ix.lineBytes[i] > b }) - 1
}

func (ix *mdTextIndex) runeOf(b int) (runeOffset, line, col int) {
	k := min(b/mdIndexStride, len(ix.cpBytes)-1)
	runeOffset = ix.cpRunes[k] + utf8.RuneCountInString(ix.text[ix.cpBytes[k]:b])
	l := ix.lineOfByte(b)
	return runeOffset, l + 1, runeOffset - ix.lineRunes[l] + 1
}

func (ix *mdTextIndex) spanFromBytes(start, stop int) MarkdownSpan {
	rs, line, col := ix.runeOf(start)
	re, _, _ := ix.runeOf(stop)
	return MarkdownSpan{
		Start:     int64(rs),
		Stop:      int64(re),
		StartByte: int64(start),
		StopByte:  int64(stop),
		Line:      int64(line),
		Column:    int64(col),
	}
}

var (
	mdListItemRE    = regexp.MustCompile(`^(?:[-*+]|\d{1,9}[.)])(?:[ \t]|$)`)
	mdHeadingRE     = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t#]*$`)
	mdImagePat      = `!\[([^\[\]\n]*)\]\(\s*<?([^\s)>]*)>?(?:\s+["']([^"'\n]*)["'])?\s*\)`
	mdLinkedImageRE = regexp.MustCompile(`\[\s*(` + mdImagePat + `)\s*\]\(\s*<?([^\s)>]*)>?(?:\s+["'][^"'\n]*["'])?\s*\)`)
	mdLinkRE        = regexp.MustCompile(`(!?)\[([^\[\]\n]*)\]\(\s*<?([^\s)>]*)>?(?:\s+["']([^"'\n]*)["'])?\s*\)`)
	mdAutoRE        = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9+.-]*:[^\s<>]*)>`)
	mdRefDefRE      = regexp.MustCompile(`(?m)^ {0,3}\[([^\]\n]+)\]:[ \t]*<?([^\s>]*)>?.*$`)
	mdHTMLImgRE     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	mdHTMLARE       = regexp.MustCompile(`(?i)<a\b[^>]*>`)
	mdSrcRE         = regexp.MustCompile(`(?i)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	mdHrefRE        = regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	mdAltRE         = regexp.MustCompile(`(?i)\balt\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	mdTitleRE       = regexp.MustCompile(`(?i)\btitle\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	mdBareURLRE     = regexp.MustCompile("(?i)\\b(?:https?://|www\\.)[^\\s<>()\\[\\]\"'`]+")
	mdInlineCodeRE  = regexp.MustCompile("`[^`\n]+`")
)

type mdExtractor struct {
	body       string
	ix         *mdTextIndex
	codeCount  int
	code       []bool
	claimed    []bool
	links      []MarkdownLink
	images     []MarkdownImage
	codeBlocks []MarkdownCodeBlock
	sections   []MarkdownSection
}

func (x *mdExtractor) scanBlocks() {
	counts := map[string]int{}
	kind, title, start := MarkdownSectionText, "", 0
	fence, codeStart := "", 0
	indented, indentEnd := false, 0
	canIndent, inList := true, false
	var cb MarkdownCodeBlock
	flush := func(end int, nextKind string) {
		if end > start {
			counts[kind]++
			id := fmt.Sprintf("%s_%d", kind, counts[kind])
			x.sections = append(x.sections, MarkdownSection{
				ID:    id,
				Kind:  kind,
				Title: title,
				Span:  x.ix.spanFromBytes(start, end),
			})
		}
		kind, start = nextKind, end
	}
	openCode := func(off, payload int, c MarkdownCodeBlock) {
		flush(off, MarkdownSectionCodeBlock)
		x.codeCount++
		c.ID = fmt.Sprintf("%s_%d", MarkdownSectionCodeBlock, x.codeCount)
		cb, codeStart = c, payload
	}
	closeCode := func(codeEnd, blockEnd int) {
		cb.Span, cb.CodeSpan = x.ix.spanFromBytes(start, blockEnd), x.ix.spanFromBytes(codeStart, codeEnd)
		x.codeBlocks = append(x.codeBlocks, cb)
		mdMark(x.code, start, blockEnd)
		fence, indented = "", false
		flush(blockEnd, MarkdownSectionText)
	}
	for off := 0; off < len(x.body); {
		end := len(x.body)
		if i := strings.IndexByte(x.body[off:], '\n'); i >= 0 {
			end = off + i + 1
		}
		line := strings.TrimRight(x.body[off:end], "\r\n")
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		blank, width := strings.TrimSpace(line) == "", mdIndentWidth(line)
		if indented {
			if blank || width >= 4 {
				if !blank {
					indentEnd = end
				}
				off = end
				continue
			}
			closeCode(indentEnd, indentEnd)
		}
		lineBreaks := blank
		switch {
		case fence != "":
			if indent <= 3 && strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
				closeCode(off, end)
				lineBreaks = true
			}
		case !blank && width >= 4 && canIndent && !inList:
			openCode(off, off, MarkdownCodeBlock{Indented: true})
			indented, indentEnd = true, end
		case indent <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			marker := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, trimmed[:1]))]
			info := strings.TrimSpace(trimmed[len(marker):])
			if marker[0] == '`' && strings.Contains(info, "`") {
				break
			}
			lang, _, _ := strings.Cut(info, " ")
			openCode(off, end, MarkdownCodeBlock{Language: lang, Info: info})
			fence = marker
		case mdHeadingRE.MatchString(line):
			flush(off, MarkdownSectionHeading)
			title = strings.TrimSpace(mdHeadingRE.FindStringSubmatch(line)[2])
			lineBreaks, inList = true, false
		case indent <= 3 && strings.HasPrefix(trimmed, ">"):
			if kind != MarkdownSectionBlockquote {
				flush(off, MarkdownSectionBlockquote)
			}
		case kind == MarkdownSectionBlockquote:
			flush(off, MarkdownSectionText)
		}
		if fence == "" && !indented && !blank && width <= 3 {
			if mdListItemRE.MatchString(strings.TrimLeft(line, " \t")) {
				inList = true
			} else if canIndent {
				inList = false
			}
		}
		canIndent = lineBreaks
		off = end
	}
	switch {
	case indented:
		closeCode(indentEnd, indentEnd)
	case fence != "":
		closeCode(len(x.body), len(x.body))
	}
	flush(len(x.body), MarkdownSectionText)
}

func (x *mdExtractor) extractInline() {
	body := x.body
	for _, m := range mdInlineCodeRE.FindAllStringIndex(body, -1) {
		mdMark(x.code, m[0], m[1])
	}
	for _, m := range mdLinkedImageRE.FindAllStringSubmatchIndex(body, -1) {
		if x.claim(m, false) {
			alt := body[m[4]:m[5]]
			url := body[m[10]:m[11]]
			src := body[m[6]:m[7]]
			x.addLink(m, url, alt)
			x.addImage(m[2:], src, alt, mdGroup(body, m, 4))
			if mdIsImageURL(url) && url != src {
				x.addImage(m, url, alt, "")
			}
		}
	}
	for _, m := range mdLinkRE.FindAllStringSubmatchIndex(body, -1) {
		if x.claim(m, false) {
			url, text, title := body[m[6]:m[7]], body[m[4]:m[5]], mdGroup(body, m, 4)
			if m[3] > m[2] {
				x.addImage(m, url, text, title)
			} else {
				x.addLink(m, url, text)
				if mdIsImageURL(url) {
					x.addImage(m, url, text, title)
				}
			}
		}
	}
	for _, m := range mdHTMLImgRE.FindAllStringIndex(body, -1) {
		if tag := body[m[0]:m[1]]; x.claim(m, false) {
			x.addImage(m, mdAttr(mdSrcRE, tag), mdAttr(mdAltRE, tag), mdAttr(mdTitleRE, tag))
		}
	}
	for _, m := range mdHTMLARE.FindAllStringIndex(body, -1) {
		if tag := body[m[0]:m[1]]; x.claim(m, false) {
			href := mdAttr(mdHrefRE, tag)
			x.addLink(m, href, "")
			if mdIsImageURL(href) {
				x.addImage(m, href, mdAttr(mdAltRE, tag), mdAttr(mdTitleRE, tag))
			}
		}
	}
	for _, m := range mdRefDefRE.FindAllStringSubmatchIndex(body, -1) {
		if x.claim(m, false) {
			url, text := body[m[4]:m[5]], body[m[2]:m[3]]
			x.addLink(m, url, text)
			if mdIsImageURL(url) {
				x.addImage(m, url, text, "")
			}
		}
	}
	for _, m := range mdAutoRE.FindAllStringSubmatchIndex(body, -1) {
		if x.claim(m, false) {
			url := body[m[2]:m[3]]
			x.addLink(m, url, "")
			if mdIsImageURL(url) {
				x.addImage(m, url, "", "")
			}
		}
	}
	for _, m := range mdBareURLRE.FindAllStringIndex(body, -1) {
		m[1] = m[0] + len(strings.TrimRight(body[m[0]:m[1]], ".,;:!?*_~"))
		if x.claim(m, true) {
			url := body[m[0]:m[1]]
			x.addLink(m, url, "")
			if mdIsImageURL(url) {
				x.addImage(m, url, "", "")
			}
		}
	}
	slices.SortFunc(x.links, func(a, b MarkdownLink) int { return cmp.Compare(a.Span.StartByte, b.Span.StartByte) })
	slices.SortFunc(x.images, func(a, b MarkdownImage) int { return cmp.Compare(a.Span.StartByte, b.Span.StartByte) })
}

func (x *mdExtractor) claim(m []int, inCode bool) bool {
	if slices.Contains(x.claimed[m[0]:m[1]], true) || (!inCode && x.code[m[0]]) {
		return false
	}
	mdMark(x.claimed, m[0], m[1])
	return true
}

func (x *mdExtractor) addLink(m []int, url, text string) {
	x.links = append(x.links, MarkdownLink{URL: url, Text: text, Span: x.ix.spanFromBytes(m[0], m[1])})
}

func (x *mdExtractor) addImage(m []int, src, alt, title string) {
	x.images = append(x.images, MarkdownImage{Src: src, Alt: alt, Title: title, Span: x.ix.spanFromBytes(m[0], m[1])})
}

func mdMark(mask []bool, start, stop int) {
	for i := start; i < stop; i++ {
		mask[i] = true
	}
}

func mdIndentWidth(line string) int {
	w := 0
	for _, r := range line {
		switch r {
		case ' ':
			w++
		case '\t':
			w += 4 - w%4
		default:
			return w
		}
	}
	return w
}

func mdIsImageURL(u string) bool {
	lower := strings.ToLower(strings.TrimSpace(u))
	if strings.HasPrefix(lower, "data:image/") {
		return true
	}
	if i := strings.IndexAny(lower, "?#"); i >= 0 {
		lower = lower[:i]
	}
	dot := strings.LastIndexByte(lower, '.')
	if dot <= 0 {
		return false
	}
	switch lower[dot:] {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp", ".avif", ".tiff", ".tif", ".apng", ".jfif":
		return true
	}
	return false
}

func mdGroup(s string, m []int, n int) string {
	if i := 2 * n; i+1 < len(m) && m[i] >= 0 {
		return s[m[i]:m[i+1]]
	}
	return ""
}

func mdAttr(re *regexp.Regexp, tag string) string {
	if m := re.FindStringSubmatch(tag); m != nil {
		for _, v := range m[1:] {
			if v != "" {
				return v
			}
		}
	}
	return ""
}
