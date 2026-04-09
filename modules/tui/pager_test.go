package tui

import (
	"strings"
	"testing"
)

var benchmarkVisibleContent string
var benchmarkRenderedLines int

func TestBuildLineStarts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []int
	}{
		{name: "empty", content: "", want: []int{0}},
		{name: "single line", content: "abc", want: []int{0}},
		{name: "multiple lines", content: "a\nbb\nccc", want: []int{0, 2, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLineStarts(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("len(buildLineStarts(%q))=%d, want %d", tt.content, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("buildLineStarts(%q)[%d]=%d, want %d", tt.content, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCountRenderedLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		width   int
		want    int
	}{
		{name: "empty", content: "", width: 80, want: 0},
		{name: "plain no wrap", content: "abc", width: 80, want: 1},
		{name: "plain wrap", content: "abcdef", width: 3, want: 2},
		{name: "ansi no wrap", content: "\x1b[31mabcdef\x1b[0m", width: 6, want: 1},
		{name: "ansi wrap", content: "\x1b[31mabcdef\x1b[0m", width: 3, want: 2},
		{name: "cjk wrap", content: "你好", width: 3, want: 2},
		{name: "trailing newline", content: "a\n", width: 80, want: 1},
		{name: "mixed lines", content: "ab\ncdef", width: 3, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countRenderedLinesLimit(tt.content, tt.width, 10000)
			if got != tt.want {
				t.Fatalf("countRenderedLinesLimit(%q, %d)=%d, want %d", tt.content, tt.width, got, tt.want)
			}
		})
	}
}

func TestGetVisibleContent(t *testing.T) {
	m := newPagerModel("line1\nline2\nline3\nline4\n", 1, false)
	m.height = 2

	m.scrollPos = 0
	if got, want := m.getVisibleContent(), "line1\nline2"; got != want {
		t.Fatalf("scroll 0 got %q, want %q", got, want)
	}

	m.scrollPos = 1
	if got, want := m.getVisibleContent(), "line2\nline3"; got != want {
		t.Fatalf("scroll 1 got %q, want %q", got, want)
	}

	m.scrollPos = 2
	if got, want := m.getVisibleContent(), "line3\nline4"; got != want {
		t.Fatalf("scroll 2 got %q, want %q", got, want)
	}
}

func BenchmarkGetVisibleContent(b *testing.B) {
	var sb strings.Builder
	sb.Grow(20000 * len("line content 1234567890\n"))
	for range 20000 {
		sb.WriteString("line content 1234567890\n")
	}
	content := sb.String()
	m := newPagerModel(content, 1, false)
	m.height = 40
	m.scrollPos = 1000
	b.ReportAllocs()

	for b.Loop() {
		benchmarkVisibleContent = m.getVisibleContent()
	}
}

func BenchmarkCountRenderedLines(b *testing.B) {
	var sb strings.Builder
	for range 5000 {
		sb.WriteString("\x1b[31mline with ansi 你好 and long long long content\x1b[0m\n")
	}
	content := sb.String()
	b.ReportAllocs()

	for b.Loop() {
		benchmarkRenderedLines = countRenderedLinesLimit(content, 80, 10000)
	}
}
