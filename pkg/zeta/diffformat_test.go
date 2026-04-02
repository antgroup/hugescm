package zeta

import (
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco"
)

var benchmarkFormattedPatch string

func TestShouldWordDiffThreshold(t *testing.T) {
	shortDel := diferenco.Line{Kind: diferenco.Delete, Content: "short-old\n"}
	shortIns := diferenco.Line{Kind: diferenco.Insert, Content: "short-new\n"}
	if !shouldWordDiff(shortDel, shortIns) {
		t.Fatalf("short lines should enable word-diff")
	}

	tooLong := strings.Repeat("a", maxWordDiffLineBytes+1) + "\n"
	if shouldWordDiff(diferenco.Line{Kind: diferenco.Delete, Content: tooLong}, shortIns) {
		t.Fatalf("line exceeding maxWordDiffLineBytes should disable word-diff")
	}

	half := strings.Repeat("x", maxWordDiffTotal/2+100) + "\n"
	if shouldWordDiff(
		diferenco.Line{Kind: diferenco.Delete, Content: half},
		diferenco.Line{Kind: diferenco.Insert, Content: half},
	) {
		t.Fatalf("combined line size exceeding maxWordDiffTotal should disable word-diff")
	}
}

func TestPickWordDiffAlgorithm(t *testing.T) {
	if got := pickWordDiffAlgorithm("a", "b"); got != diferenco.Myers {
		t.Fatalf("small input algo=%v, want %v", got, diferenco.Myers)
	}
	long := strings.Repeat("x", 4096)
	if got := pickWordDiffAlgorithm(long, "y"); got != diferenco.Histogram {
		t.Fatalf("large input algo=%v, want %v", got, diferenco.Histogram)
	}
}

func BenchmarkFormatPatchWordDiff(b *testing.B) {
	formatter := newDiffFormatter(true)
	patch := &diferenco.Patch{
		From: &diferenco.File{Name: "a.txt", Hash: strings.Repeat("1", 64), Mode: 0o100644},
		To:   &diferenco.File{Name: "a.txt", Hash: strings.Repeat("2", 64), Mode: 0o100644},
		Hunks: []*diferenco.Hunk{
			{
				FromLine: 1,
				ToLine:   1,
				Lines: []diferenco.Line{
					{Kind: diferenco.Equal, Content: "context line 1\n"},
					{Kind: diferenco.Delete, Content: "The quick brown fox jumps over the lazy dog\n"},
					{Kind: diferenco.Insert, Content: "The quick brown cat jumps over the lazy dog\n"},
					{Kind: diferenco.Equal, Content: "context line 2\n"},
				},
			},
		},
	}
	b.ReportAllocs()

	for b.Loop() {
		benchmarkFormattedPatch = formatter.formatPatch(patch)
	}
}
