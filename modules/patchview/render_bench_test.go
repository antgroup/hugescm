package patchview

import (
	"fmt"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco"
)

// buildLargePatch builds a synthetic patch with hunks×linesPerHunk diff
// lines, in the same shape a unified diff produces (context/delete/insert mix).
func buildLargePatch(from, to string, hunks, linesPerHunk int) *diferenco.Patch {
	patch := &diferenco.Patch{
		From: &diferenco.File{Name: from},
		To:   &diferenco.File{Name: to},
	}
	for h := range hunks {
		lines := make([]diferenco.Line, 0, linesPerHunk)
		base := h * linesPerHunk
		for i := 0; i+2 < linesPerHunk; i += 3 {
			lines = append(lines,
				diferenco.Line{Kind: diferenco.Equal, Content: fmt.Sprintf("	context line %d with some padding to emulate real source width", base+i)},
				diferenco.Line{Kind: diferenco.Delete, Content: fmt.Sprintf("old code here %d with some padding to emulate real source width", base+i+1)},
				diferenco.Line{Kind: diferenco.Insert, Content: fmt.Sprintf("new code here %d with some padding to emulate real source width", base+i+2)},
			)
		}
		patch.Hunks = append(patch.Hunks, &diferenco.Hunk{
			FromLine: base + 1,
			ToLine:   base + 1,
			Lines:    lines,
		})
	}
	return patch
}

// BenchmarkRenderViewport measures the per-frame render cost of large
// patches at the top, middle and bottom of the viewport. Scroll offsets
// deep into the patch must stay as cheap as the top, so the worst case here
// is the regression gate for viewport virtualization.
func BenchmarkRenderViewport(b *testing.B) {
	for _, scale := range []struct {
		name  string
		hunks int
	}{{"2khunks", 2000}, {"20khunks", 20000}} {
		r := NewPatchRenderer()
		// Isolate the viewport/scroll cost from the chroma highlighter cache.
		r.SetSyntaxHighlight(false)
		r.SetPatch(buildLargePatch("a/big.rs", "b/big.rs", scale.hunks, 90))
		r.SetSize(200, 60)

		offsets := []int{
			0,
			r.TotalLines() / 2,
			r.TotalLines() - 60,
		}
		names := []string{"top", "middle", "bottom"}
		for i, off := range offsets {
			b.Run(scale.name+"/"+names[i], func(b *testing.B) {
				b.ReportAllocs()
				for j := 0; j < b.N; j++ {
					r.SetYOffset(off)
					_ = r.Render()
				}
			})
		}
	}
}

// BenchmarkRenderScrollStep simulates holding the j key: one-line-at-a-time
// scrolling through the middle of a large patch. This is the interaction
// latency a user feels while reviewing.
func BenchmarkRenderScrollStep(b *testing.B) {
	r := NewPatchRenderer()
	r.SetSyntaxHighlight(false)
	r.SetPatch(buildLargePatch("a/big.rs", "b/big.rs", 2000, 90))
	r.SetSize(200, 60)

	mid := r.TotalLines() / 2
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		r.SetYOffset(mid + (i % 60))
		_ = r.Render()
	}
}
