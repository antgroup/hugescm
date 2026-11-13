package patchview

import (
	"regexp"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco"
)

// TestRenderContextLinesPresent is the regression test for context lines
// vanishing from the nav view: every Equal line of a hunk must be rendered
// with its (non-empty) content visible.
func TestRenderContextLinesPresent(t *testing.T) {
	r := NewPatchRenderer()
	r.SetSyntaxHighlight(false)
	r.SetPatch(&diferenco.Patch{
		To: &diferenco.File{Name: "a.go"},
		Hunks: []*diferenco.Hunk{{
			FromLine: 1,
			ToLine:   1,
			Section:  "",
			Lines: []diferenco.Line{
				{Kind: diferenco.Equal, Content: "l1"},
				{Kind: diferenco.Equal, Content: "l2"},
				{Kind: diferenco.Delete, Content: "l3 OLD"},
				{Kind: diferenco.Insert, Content: "l3 NEW"},
				{Kind: diferenco.Equal, Content: "l4"},
				{Kind: diferenco.Equal, Content: "l5"},
			},
		}},
	})
	r.SetSize(80, 10)

	out := r.Render()
	t.Logf("rendered frame:\n%s", out)

	for _, want := range []string{"l1", "l2", "l4", "l5"} {
		if !strings.Contains(out, want) {
			t.Errorf("context line %q is missing from the rendered frame", want)
		}
	}
}

// TestRenderContextLinesHighlighted repeats the probe with syntax
// highlighting enabled (the real `--nav` configuration on a TTY): the
// chroma round-trip must not swallow Equal-line content.
func TestRenderContextLinesHighlighted(t *testing.T) {
	r := NewPatchRenderer()
	r.SetSyntaxHighlight(true)
	r.SetPatch(&diferenco.Patch{
		To: &diferenco.File{Name: "a.go"},
		Hunks: []*diferenco.Hunk{{
			FromLine: 1,
			ToLine:   1,
			Lines: []diferenco.Line{
				{Kind: diferenco.Equal, Content: "package main"},
				{Kind: diferenco.Delete, Content: "func old() {}"},
				{Kind: diferenco.Insert, Content: "func new() {}"},
				{Kind: diferenco.Equal, Content: "var x = 1"},
			},
		}},
	})
	r.SetSize(80, 8)

	out := r.Render()
	t.Logf("highlighted frame:\n%s", out)

	for _, want := range []string{"package main", "var x = 1"} {
		stripped := reANSI.ReplaceAllString(out, "")
		if !strings.Contains(stripped, want) {
			t.Errorf("highlighted context line %q is missing from the rendered frame", want)
		}
	}
}

var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)
