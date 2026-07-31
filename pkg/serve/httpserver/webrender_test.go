// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// TestNewWebRendererParsesTemplates guarantees embedded templates still
// parse after editing web/templates/blob.tmpl. The newWebRenderer
// constructor uses template.Must(...) and would crash the binary on init
// if any template were syntactically malformed.
func TestNewWebRendererParsesTemplates(t *testing.T) {
	r := newWebRenderer()
	if r == nil {
		t.Fatal("newWebRenderer returned nil")
	}
	if r.tmpl == nil {
		t.Fatal("webRenderer.tmpl is nil; templates did not parse")
	}
}

// TestBlobTemplateStashesRawSourceForCopy pins the contract that the
// blob page embeds the raw .Content into a hidden <pre id="blob-raw">
// from which copyBlobContent reads. Without this stash the clipboard text
// would be derived from the highlighted DOM: it would either lose the
// original `\t` characters (highlightCode expands them to 4 spaces for
// visual reasons — see TestHighlightCodeExpandsTabs) or gain extra blank
// lines between every pair of code lines (chroma's .cl spans each retain
// the trailing `\n` of their source line). Inspecting the template source
// keeps this test robust without mocking the full database page data.
func TestBlobTemplateStashesRawSourceForCopy(t *testing.T) {
	raw, err := fs.ReadFile(templateFileSystem(), "blob.tmpl")
	if err != nil {
		t.Fatalf("read blob.tmpl: %v", err)
	}
	tmpl := string(raw)
	for _, needle := range []string{
		`<pre id="blob-raw" style="display:none">`,
		`getElementById('blob-raw')`,
	} {
		if !strings.Contains(tmpl, needle) {
			t.Errorf("blob.tmpl is missing %q (copy would not round-trip the raw source)", needle)
		}
	}
	// The buggy separator (`join('\n')` doubling newlines between code
	// lines) must NOT appear anywhere in this template anymore.
	if strings.Contains(tmpl, "parts.join('\\n')") {
		t.Errorf("blob.tmpl still uses parts.join('\\n'); this inserts one extra \\n between every code line")
	}
}

// TestHighlightCodeExpandsTabs asserts that literal tab characters in the
// source are expanded to 4 spaces before chroma tokenises the input. The
// project ships its own .chroma CSS which does not include the
// `tab-size: 4` rule chroma would otherwise auto-emit; even when added,
// the `.ln` inline-block's width (~7.84 monospace cols at 0.85rem font +
// 4rem padding) is not a multiple of the 4-col tab-stop, so the leading
// tab of every indented line would squash to ~0 cols wide. Source-level
// replacement fixes this robustly. This does not affect the copy button,
// which reads the raw `.Content` from the hidden <pre id="blob-raw">
// stash (see TestBlobTemplateStashesRawSourceForCopy).
func TestHighlightCodeExpandsTabs(t *testing.T) {
	const src = "package main\n\nfunc main() {\n\tx := 1\n\tprintln(x)\n}\n"
	out := string(highlightCode(src, "main.go"))
	if strings.Contains(out, "\t") {
		t.Fatalf("highlighted HTML still contains a literal tab character; tabs should be expanded to 4 spaces before tokenisation:\n%s", out)
	}
}

// TestChromaCLSpansRebuildSourceViaConcat pins the behaviour of the legacy
// fallback path in copyBlobContent (used when #blob-raw is absent for any
// reason). Each <span class="cl"> emitted by chroma retains the trailing
// newline of its source line, so concatenating the per-line textContent
// (NOT joining with '\n') rebuilds the post-highlight input byte-for-byte.
// Joining with '\n' (the original, buggy implementation) inserts one extra
// blank line between every adjacent pair of code lines.
func TestChromaCLSpansRebuildSourceViaConcat(t *testing.T) {
	const src = "package main\n\nfunc main() {\n\tx := 1\n\tprintln(x)\n}\n"
	// Mirror highlightCode's preprocessing; only the tab-replaced bytes
	// reach chroma, so the reconstructed text reflects the post-tab-
	// expansion form (the raw tabs are preserved only inside the
	// #blob-raw stash — not in the chroma DOM).
	srcExpanded := strings.ReplaceAll(src, "\t", "    ")

	lexer := chroma.Coalesce(lexers.Get("go"))
	iterator, err := lexer.Tokenise(nil, srcExpanded)
	if err != nil {
		t.Fatalf("tokenise: %v", err)
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.TabWidth(4))
	var buf bytes.Buffer
	if err := formatter.Format(&buf, styles.Get("github"), iterator); err != nil {
		t.Fatalf("format: %v", err)
	}

	inner := extractCodeBlockForTest(buf.String())
	lines := splitChromaLines(inner)
	if len(lines) == 0 {
		t.Fatalf("splitChromaLines returned 0 lines")
	}

	var parts []string
	for _, lineHTML := range lines {
		parts = append(parts, stripTagsForTest(lineHTML))
	}

	got := strings.Join(parts, "")
	if got != srcExpanded {
		t.Fatalf("concatenated text does not match the expanded source:\n want (repr): %q\n got (repr): %q", srcExpanded, got)
	}

	buggyJoin := strings.Join(parts, "\n")
	if buggyJoin == srcExpanded {
		t.Fatalf("expected joining with '\\n' to differ, but it matched the source")
	}
	extraNewlines := strings.Count(buggyJoin, "\n") - strings.Count(srcExpanded, "\n")
	if extraNewLinesExpected := len(parts) - 1; extraNewlines != extraNewLinesExpected {
		t.Fatalf("expected %d extra newlines with '\\n' join, got %d", extraNewLinesExpected, extraNewlines)
	}
}

// TestChromaCLSpansRebuildSourceWithoutTrailingNewline covers the edge case
// where the source has no trailing newline. The last line emitted by chroma
// does not carry a trailing '\n', so concatenation recovers the original
// content verbatim — confirming the contract holds regardless of trailing
// newline presence.
func TestChromaCLSpansRebuildSourceWithoutTrailingNewline(t *testing.T) {
	const src = "line1\nline2"
	lexer := chroma.Coalesce(lexers.Get("go"))
	iterator, err := lexer.Tokenise(nil, src)
	if err != nil {
		t.Fatalf("tokenise: %v", err)
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.TabWidth(4))
	var buf bytes.Buffer
	if err := formatter.Format(&buf, styles.Get("github"), iterator); err != nil {
		t.Fatalf("format: %v", err)
	}
	inner := extractCodeBlockForTest(buf.String())
	lines := splitChromaLines(inner)
	var parts []string
	for _, lineHTML := range lines {
		parts = append(parts, stripTagsForTest(lineHTML))
	}
	if got := strings.Join(parts, ""); got != src {
		t.Fatalf("concatenated copy text does not match the source:\n want (repr): %q\n got (repr): %q", src, got)
	}
}

// extractCodeBlockForTest returns the content between <code> and the last
// </code> in the supplied chroma HTML. Mirrors the inline logic in
// wrapLineNumbers.
func extractCodeBlockForTest(s string) string {
	const (
		codeOpen  = "<code>"
		codeClose = "</code>"
	)
	start := strings.Index(s, codeOpen)
	end := strings.LastIndex(s, codeClose)
	if start < 0 || end < 0 || end <= start {
		return s
	}
	return s[start+len(codeOpen) : end]
}

// stripTagsForTest emulates the DOM textContent accessor: it discards all
// <...> tag fragments from the input and keeps everything else as-is. This
// matches what the browser exposes via a span's .textContent for the
// purposes of copyBlobContent.
func stripTagsForTest(html string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}
