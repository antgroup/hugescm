package diferenco

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Helper functions for benchmark test data
func generateText(lines int, prefix string) string {
	var builder strings.Builder
	builder.Grow(lines * 20) // Pre-allocate approximate size
	for i := range lines {
		builder.WriteString(prefix)
		builder.WriteString(strconv.Itoa(i))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func generateModifiedText(lines int, prefix string, changes int) string {
	var builder strings.Builder
	builder.Grow(lines * 25) // Pre-allocate approximate size
	for i := range lines {
		// Modify some lines
		if i%10 == 0 && changes > 0 {
			builder.WriteString(prefix)
			builder.WriteString("_modified_")
			builder.WriteString(strconv.Itoa(i))
			builder.WriteByte('\n')
			changes--
		} else {
			builder.WriteString(prefix)
			builder.WriteString(strconv.Itoa(i))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

// generateConflictText generates texts where A and B modify the same lines (creates conflicts)
func generateConflictText(lines int, prefix string, conflictRate int) (o, a, b string) {
	var oBuilder, aBuilder, bBuilder strings.Builder
	oBuilder.Grow(lines * 20)
	aBuilder.Grow(lines * 25)
	bBuilder.Grow(lines * 25)

	for i := range lines {
		// Original line
		oBuilder.WriteString(prefix)
		oBuilder.WriteString(strconv.Itoa(i))
		oBuilder.WriteByte('\n')

		// A and B modify the same lines with conflictRate probability
		if i%conflictRate == 0 {
			// A's modification
			aBuilder.WriteString(prefix)
			aBuilder.WriteString("_A_modified_")
			aBuilder.WriteString(strconv.Itoa(i))
			aBuilder.WriteByte('\n')

			// B's modification (different from A - creates conflict)
			bBuilder.WriteString(prefix)
			bBuilder.WriteString("_B_modified_")
			bBuilder.WriteString(strconv.Itoa(i))
			bBuilder.WriteByte('\n')
		} else if i%10 == 0 {
			// Only A modifies
			aBuilder.WriteString(prefix)
			aBuilder.WriteString("_A_only_")
			aBuilder.WriteString(strconv.Itoa(i))
			aBuilder.WriteByte('\n')
			bBuilder.WriteString(prefix)
			bBuilder.WriteString(strconv.Itoa(i))
			bBuilder.WriteByte('\n')
		} else if i%7 == 0 {
			// Only B modifies
			aBuilder.WriteString(prefix)
			aBuilder.WriteString(strconv.Itoa(i))
			aBuilder.WriteByte('\n')
			bBuilder.WriteString(prefix)
			bBuilder.WriteString("_B_only_")
			bBuilder.WriteString(strconv.Itoa(i))
			bBuilder.WriteByte('\n')
		} else {
			// No change
			aBuilder.WriteString(prefix)
			aBuilder.WriteString(strconv.Itoa(i))
			aBuilder.WriteByte('\n')
			bBuilder.WriteString(prefix)
			bBuilder.WriteString(strconv.Itoa(i))
			bBuilder.WriteByte('\n')
		}
	}

	return oBuilder.String(), aBuilder.String(), bBuilder.String()
}

// BenchmarkMergeParallel compares MergeParallel performance with Merge
func BenchmarkMergeParallel(b *testing.B) {
	ctx := context.Background()

	// Test cases with different sizes
	benchmarks := []struct {
		name  string
		size  int
		textO string
		textA string
		textB string
	}{
		{
			name:  "small",
			size:  100,
			textO: generateText(100, "line"),
			textA: generateModifiedText(100, "line", 10),
			textB: generateModifiedText(100, "line", 15),
		},
		{
			name:  "medium",
			size:  1000,
			textO: generateText(1000, "line"),
			textA: generateModifiedText(1000, "line", 100),
			textB: generateModifiedText(1000, "line", 150),
		},
		{
			name:  "large",
			size:  10000,
			textO: generateText(10000, "line"),
			textA: generateModifiedText(10000, "line", 1000),
			textB: generateModifiedText(10000, "line", 1500),
		},
	}

	for _, bm := range benchmarks {
		b.Run("MergeParallel_"+bm.name, func(b *testing.B) {
			opts := &MergeOptions{
				TextO: bm.textO,
				TextA: bm.textA,
				TextB: bm.textB,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = MergeParallel(ctx, opts)
			}
		})

		b.Run("Merge_"+bm.name, func(b *testing.B) {
			opts := &MergeOptions{
				TextO: bm.textO,
				TextA: bm.textA,
				TextB: bm.textB,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = Merge(ctx, opts)
			}
		})
	}
}

// BenchmarkMergeParallelAlgorithms compares different algorithms
func BenchmarkMergeParallelAlgorithms(b *testing.B) {
	ctx := context.Background()
	algorithms := []Algorithm{
		Histogram,
		Myers,
		ONP,
		Patience,
		Minimal,
	}

	textO := generateText(1000, "line")
	textA := generateModifiedText(1000, "line", 100)
	textB := generateModifiedText(1000, "line", 150)

	for _, algo := range algorithms {
		b.Run("MergeParallel_"+algo.String(), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: STYLE_DEFAULT,
				A:     algo,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = MergeParallel(ctx, opts)
			}
		})

		b.Run("Merge_"+algo.String(), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: STYLE_DEFAULT,
				A:     algo,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = Merge(ctx, opts)
			}
		})
	}
}

// BenchmarkMergeParallelConflictScenarios tests different conflict scenarios
func BenchmarkMergeParallelConflictScenarios(b *testing.B) {
	ctx := context.Background()

	scenarios := []struct {
		name         string
		lines        int
		conflictRate int
		description  string
	}{
		{"no_conflicts", 1000, 1000, "no conflicts - only independent changes"},
		{"few_conflicts", 1000, 100, "few conflicts - ~1% conflicting lines"},
		{"moderate_conflicts", 1000, 50, "moderate conflicts - ~2% conflicting lines"},
		{"many_conflicts", 1000, 20, "many conflicts - ~5% conflicting lines"},
	}

	for _, scenario := range scenarios {
		textO, textA, textB := generateConflictText(scenario.lines, "line", scenario.conflictRate)

		b.Run(fmt.Sprintf("MergeParallel_%s", scenario.name), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = MergeParallel(ctx, opts)
			}
		})

		b.Run(fmt.Sprintf("Merge_%s", scenario.name), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = Merge(ctx, opts)
			}
		})
	}
}

// BenchmarkMergeParallelConflictStyles compares different conflict styles
func BenchmarkMergeParallelConflictStyles(b *testing.B) {
	ctx := context.Background()

	textO, textA, textB := generateConflictText(1000, "line", 30)
	styles := []struct {
		name  string
		style int
	}{
		{"STYLE_DEFAULT", STYLE_DEFAULT},
		{"STYLE_DIFF3", STYLE_DIFF3},
		{"STYLE_ZEALOUS_DIFF3", STYLE_ZEALOUS_DIFF3},
	}

	for _, s := range styles {
		b.Run(fmt.Sprintf("MergeParallel_%s", s.name), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: s.style,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = MergeParallel(ctx, opts)
			}
		})

		b.Run(fmt.Sprintf("Merge_%s", s.name), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: s.style,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = Merge(ctx, opts)
			}
		})
	}
}

// BenchmarkHasConflictComparison compares HasConflict vs HasConflictParallel
func BenchmarkHasConflictComparison(b *testing.B) {
	ctx := context.Background()

	scenarios := []struct {
		name         string
		lines        int
		conflictRate int
	}{
		{"small_no_conflict", 100, 1000},
		{"small_with_conflict", 100, 20},
		{"medium_no_conflict", 1000, 1000},
		{"medium_with_conflict", 1000, 20},
		{"large_no_conflict", 10000, 1000},
		{"large_with_conflict", 10000, 20},
	}

	for _, scenario := range scenarios {
		textO, textA, textB := generateConflictText(scenario.lines, "line", scenario.conflictRate)

		b.Run(fmt.Sprintf("HasConflictParallel_%s", scenario.name), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, _ = HasConflictParallel(ctx, textO, textA, textB)
			}
		})

		b.Run(fmt.Sprintf("HasConflict_%s", scenario.name), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, _ = HasConflict(ctx, textO, textA, textB)
			}
		})
	}
}

// BenchmarkMergeParallelMemory traces memory allocations
func BenchmarkMergeParallelMemory(b *testing.B) {
	ctx := context.Background()

	textO := generateText(1000, "line")
	textA := generateModifiedText(1000, "line", 100)
	textB := generateModifiedText(1000, "line", 150)

	b.Run("MergeParallel_memory", func(b *testing.B) {
		opts := &MergeOptions{
			TextO: textO,
			TextA: textA,
			TextB: textB,
			Style: STYLE_DEFAULT,
			A:     Histogram,
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _, _ = MergeParallel(ctx, opts)
		}
	})

	b.Run("Merge_memory", func(b *testing.B) {
		opts := &MergeOptions{
			TextO: textO,
			TextA: textA,
			TextB: textB,
			Style: STYLE_DEFAULT,
			A:     Histogram,
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _, _ = Merge(ctx, opts)
		}
	})
}

// BenchmarkMergeParallelComponents benchmarks individual components of the merge
func BenchmarkMergeParallelComponents(b *testing.B) {
	ctx := context.Background()
	sink := NewSink(NEWLINE_LF)

	textO := generateText(1000, "line")
	textA := generateModifiedText(1000, "line", 100)
	textB := generateModifiedText(1000, "line", 150)

	oIdx, _ := sink.parseLines(nil, textO)
	aIdx, _ := sink.parseLines(nil, textA)
	bIdx, _ := sink.parseLines(nil, textB)

	b.Run("parseLines", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			sink := NewSink(NEWLINE_LF)
			_, _ = sink.parseLines(nil, textO)
			_, _ = sink.parseLines(nil, textA)
			_, _ = sink.parseLines(nil, textB)
		}
	})

	b.Run("DiffSlices_OA", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_, _ = DiffSlices(ctx, oIdx, aIdx, Histogram)
		}
	})

	b.Run("DiffSlices_OB", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_, _ = DiffSlices(ctx, oIdx, bIdx, Histogram)
		}
	})

	changesA, _ := DiffSlices(ctx, oIdx, aIdx, Histogram)
	changesB, _ := DiffSlices(ctx, oIdx, bIdx, Histogram)

	b.Run("findMergeRegions", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_ = findMergeRegions(changesA, changesB)
		}
	})
}
