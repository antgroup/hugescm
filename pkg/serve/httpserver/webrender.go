// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const (
	// webSessionCookieName is the HTTP cookie name storing the web JWT.
	webSessionCookieName = "zeta_web_session"
	// webSessionExpiry is how long a web session remains valid.
	webSessionExpiry = 24 * 3600 // 24 hours in seconds
	// webJwtIssuer is the issuer for web JWTs.
	webJwtIssuer = "zeta-web"
)

// webTemplateData is the data bag passed to every full-page template.
type webTemplateData struct {
	Title     string
	ServerVer string
	Username  string
	IsAdmin   bool
	Content   any // page-specific data
}

// webRenderer holds compiled templates and cached chroma state.
type webRenderer struct {
	tmpl      *template.Template
	formatter *chromahtml.Formatter
	style     *chroma.Style
}

// newWebRenderer parses all embedded templates and returns a webRenderer.
func newWebRenderer() *webRenderer {
	t := template.New("web")
	t = t.Funcs(template.FuncMap{
		"highlight":      highlightCode,
		"highlightDiff":  highlightDiff,
		"renderMarkdown": renderMarkdown,
		"isMarkdown":     isMarkdownFile,
		"truncate":       func(s string, n int) string { return truncate(s, n) },
		"firstLine":      func(s string) string { return firstLine(s) },
		"incPage":        func(p int) int { return p + 1 },
		"decPage":        func(p int) int { return p - 1 },
		"hasPage":        func(total int64, page, perPage int) bool { return int64(page*perPage) < total },
		"range1":         range1,
		"hasPrefix":      strings.HasPrefix,
		"shortHash":      func(s string) string { return shortHash(s) },
		"humanSize":      humanSize,
	})

	t = template.Must(t.ParseFS(templateFileSystem(), "*.tmpl", "partials/*.tmpl"))
	return &webRenderer{
		tmpl: t,
		formatter: chromahtml.New(
			chromahtml.WithClasses(true),
			chromahtml.TabWidth(4),
		),
		style: styles.Get("github"),
	}
}

// renderPage executes a layout-based full page.
func (r *webRenderer) renderPage(w http.ResponseWriter, serverName string, page string, data *webTemplateData) {
	data.ServerVer = serverName
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, page+".tmpl", data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// renderPartial executes a single partial (for HTMX responses).
func (r *webRenderer) renderPartial(w http.ResponseWriter, partial string, data *webTemplateData) {
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, "partials/"+partial+".tmpl", data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// highlightCode returns HTML span-highlighted source code with line numbers.
// Line numbers are rendered as a separate floated column so they don't merge
// with code content.
func highlightCode(source, filename string) template.HTML {
	if source == "" {
		return ""
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return template.HTML(escapeHTML(source))
	}
	// No chroma line numbers — we add our own to control spacing.
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.TabWidth(4),
	)
	var buf bytes.Buffer
	if err := formatter.Format(&buf, styles.Get("github"), iterator); err != nil {
		return template.HTML(escapeHTML(source))
	}
	// chroma outputs: <div class="chroma"><pre class="chroma"><code>...</code></pre></div>
	// Split the highlighted HTML into lines and wrap each line with a line number.
	highlighted := buf.String()
	return template.HTML(wrapLineNumbers(highlighted))
}

// wrapLineNumbers takes chroma's highlighted HTML (a single <pre><code> block)
// and restructures it so each source line is wrapped in a div with a line
// number span floated to the left. This gives full control over spacing
// between line numbers and code.
func wrapLineNumbers(chromaHTML string) string {
	// Extract the inner content between <code> and </code>
	codeOpen := "<code>"
	codeClose := "</code>"
	start := strings.Index(chromaHTML, codeOpen)
	end := strings.LastIndex(chromaHTML, codeClose)
	if start < 0 || end < 0 || end <= start {
		return chromaHTML
	}
	inner := chromaHTML[start+len(codeOpen) : end]
	// chroma wraps each line in <span class="line"><span class="cl">...</span></span>
	// Split on these boundaries to get individual lines.
	lines := splitChromaLines(inner)
	var b strings.Builder
	b.WriteString(`<div class="chroma"><pre class="chroma"><code>`)
	for i, line := range lines {
		lineNum := i + 1
		fmt.Fprintf(&b, `<span class="line"><span class="ln">%d</span><span class="cl">%s</span></span>`, lineNum, line)
	}
	b.WriteString(`</code></pre></div>`)
	return b.String()
}

// splitChromaLines extracts individual line content from chroma's
// <span class="line"><span class="cl">...</span></span> structure.
func splitChromaLines(inner string) []string {
	var lines []string
	// chroma uses <span class="line"><span class="cl">...</span></span> per line
	lineOpen := `<span class="line"><span class="cl">`
	lineClose := `</span></span>`
	remainder := inner
	for {
		idx := strings.Index(remainder, lineOpen)
		if idx < 0 {
			break
		}
		remainder = remainder[idx+len(lineOpen):]
		endIdx := strings.Index(remainder, lineClose)
		if endIdx < 0 {
			break
		}
		lines = append(lines, remainder[:endIdx])
		remainder = remainder[endIdx+len(lineClose):]
	}
	if len(lines) == 0 {
		// fallback: split by newline
		lines = strings.Split(inner, "\n")
	}
	return lines
}

// highlightDiff returns HTML-highlighted diff for a single hunk text.
func highlightDiff(diffText string) template.HTML {
	if diffText == "" {
		return ""
	}
	lines := strings.Split(diffText, "\n")
	var b strings.Builder
	for _, line := range lines {
		var cls string
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			cls = "diff-line-meta"
		case strings.HasPrefix(line, "+"):
			cls = "diff-line-add"
		case strings.HasPrefix(line, "-"):
			cls = "diff-line-del"
		case strings.HasPrefix(line, "@@"):
			cls = "diff-line-hunk"
		}
		fmt.Fprintf(&b, "<span class=\"%s\">%s</span>\n", cls, escapeHTML(line))
	}
	return template.HTML(b.String())
}

// isMarkdownFile returns true if filename has a markdown extension.
func isMarkdownFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown") ||
		strings.HasSuffix(lower, ".mdown") ||
		strings.HasSuffix(lower, ".mkd")
}

// renderMarkdown converts markdown source to safe HTML using goldmark with GFM extensions.
func renderMarkdown(source string) template.HTML {
	if source == "" {
		return ""
	}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, linkify, task lists)
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return template.HTML(escapeHTML(source))
	}
	return template.HTML(buf.String())
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

func range1(n int) []int {
	result := make([]int, n)
	for i := range result {
		result[i] = i + 1
	}
	return result
}

func shortHash(s string) string {
	if len(s) > 20 {
		return s[:20]
	}
	return s
}
