package diferenco

import (
	"context"
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

// BenchmarkNewMerge compares NewMerge performance with Merge
func BenchmarkNewMerge(b *testing.B) {
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
		b.Run("NewMerge_"+bm.name, func(b *testing.B) {
			opts := &MergeOptions{
				TextO: bm.textO,
				TextA: bm.textA,
				TextB: bm.textB,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = NewMerge(ctx, opts)
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

// BenchmarkNewMergeAlgorithms compares different algorithms
func BenchmarkNewMergeAlgorithms(b *testing.B) {
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
		b.Run("NewMerge_"+algo.String(), func(b *testing.B) {
			opts := &MergeOptions{
				TextO: textO,
				TextA: textA,
				TextB: textB,
				Style: STYLE_DEFAULT,
				A:     algo,
			}
			b.ResetTimer()
			for range b.N {
				_, _, _ = NewMerge(ctx, opts)
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
