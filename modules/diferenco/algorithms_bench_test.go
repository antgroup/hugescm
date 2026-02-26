package diferenco

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Benchmark helpers to generate test data
// 生成测试数据的基准测试辅助函数

func generateSequence(size int, changeRate float64) []string {
	seq := make([]string, size)
	for i := range size {
		if rand.Float64() < changeRate {
			seq[i] = fmt.Sprintf("item_%d_variant", i)
		} else {
			seq[i] = fmt.Sprintf("item_%d", i)
		}
	}
	return seq
}

func generateModifiedSequence(base []string, changeRate float64) []string {
	modified := make([]string, len(base))
	copy(modified, base)

	for i := range modified {
		if rand.Float64() < changeRate {
			modified[i] = fmt.Sprintf("modified_%d", i)
		}
	}
	return modified
}

// BenchmarkMyersAlgorithm benchmarks the Myers algorithm
// Myers 算法基准测试
func BenchmarkMyersAlgorithm(b *testing.B) {
	ctx := context.Background()
	algos := []struct {
		name   string
		algo   Algorithm
		size   int
		change float64
	}{
		{"small_10pct_change", Myers, 100, 0.1},
		{"small_50pct_change", Myers, 100, 0.5},
		{"medium_10pct_change", Myers, 1000, 0.1},
		{"medium_50pct_change", Myers, 1000, 0.5},
		{"large_10pct_change", Myers, 5000, 0.1},
		{"large_50pct_change", Myers, 5000, 0.5},
	}

	for _, tt := range algos {
		b.Run(tt.name, func(b *testing.B) {
			before := generateSequence(tt.size, 0)
			after := generateModifiedSequence(before, tt.change)

			b.ResetTimer()
			for range b.N {
				_, err := diffInternal(ctx, before, after, tt.algo)
				if err != nil {
					b.Fatalf("diffInternal() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkHistogramAlgorithm benchmarks the Histogram algorithm
// Histogram 算法基准测试
func BenchmarkHistogramAlgorithm(b *testing.B) {
	ctx := context.Background()
	algos := []struct {
		name   string
		algo   Algorithm
		size   int
		change float64
	}{
		{"small_10pct_change", Histogram, 100, 0.1},
		{"small_50pct_change", Histogram, 100, 0.5},
		{"medium_10pct_change", Histogram, 1000, 0.1},
		{"medium_50pct_change", Histogram, 1000, 0.5},
		{"large_10pct_change", Histogram, 5000, 0.1},
		{"large_50pct_change", Histogram, 5000, 0.5},
	}

	for _, tt := range algos {
		b.Run(tt.name, func(b *testing.B) {
			before := generateSequence(tt.size, 0)
			after := generateModifiedSequence(before, tt.change)

			b.ResetTimer()
			for range b.N {
				_, err := diffInternal(ctx, before, after, tt.algo)
				if err != nil {
					b.Fatalf("diffInternal() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkONPAlgorithm benchmarks the ONP algorithm
// ONP 算法基准测试
func BenchmarkONPAlgorithm(b *testing.B) {
	ctx := context.Background()
	algos := []struct {
		name   string
		algo   Algorithm
		size   int
		change float64
	}{
		{"small_10pct_change", ONP, 100, 0.1},
		{"small_50pct_change", ONP, 100, 0.5},
		{"medium_10pct_change", ONP, 1000, 0.1},
		{"medium_50pct_change", ONP, 1000, 0.5},
		{"large_10pct_change", ONP, 5000, 0.1},
		{"large_50pct_change", ONP, 5000, 0.5},
	}

	for _, tt := range algos {
		b.Run(tt.name, func(b *testing.B) {
			before := generateSequence(tt.size, 0)
			after := generateModifiedSequence(before, tt.change)

			b.ResetTimer()
			for range b.N {
				_, err := diffInternal(ctx, before, after, tt.algo)
				if err != nil {
					b.Fatalf("diffInternal() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkPatienceAlgorithm benchmarks the Patience algorithm
// Patience 算法基准测试
func BenchmarkPatienceAlgorithm(b *testing.B) {
	ctx := context.Background()
	algos := []struct {
		name   string
		algo   Algorithm
		size   int
		change float64
	}{
		{"small_10pct_change", Patience, 100, 0.1},
		{"small_50pct_change", Patience, 100, 0.5},
		{"medium_10pct_change", Patience, 1000, 0.1},
		{"medium_50pct_change", Patience, 1000, 0.5},
		{"large_10pct_change", Patience, 5000, 0.1},
		{"large_50pct_change", Patience, 5000, 0.5},
	}

	for _, tt := range algos {
		b.Run(tt.name, func(b *testing.B) {
			before := generateSequence(tt.size, 0)
			after := generateModifiedSequence(before, tt.change)

			b.ResetTimer()
			for range b.N {
				_, err := diffInternal(ctx, before, after, tt.algo)
				if err != nil {
					b.Fatalf("diffInternal() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkMinimalAlgorithm benchmarks the Minimal algorithm
// Minimal 算法基准测试
func BenchmarkMinimalAlgorithm(b *testing.B) {
	ctx := context.Background()
	algos := []struct {
		name   string
		algo   Algorithm
		size   int
		change float64
	}{
		{"small_10pct_change", Minimal, 100, 0.1},
		{"small_50pct_change", Minimal, 100, 0.5},
		{"medium_10pct_change", Minimal, 1000, 0.1},
		{"medium_50pct_change", Minimal, 1000, 0.5},
		{"large_10pct_change", Minimal, 5000, 0.1},
		{"large_50pct_change", Minimal, 5000, 0.5},
	}

	for _, tt := range algos {
		b.Run(tt.name, func(b *testing.B) {
			before := generateSequence(tt.size, 0)
			after := generateModifiedSequence(before, tt.change)

			b.ResetTimer()
			for range b.N {
				_, err := diffInternal(ctx, before, after, tt.algo)
				if err != nil {
					b.Fatalf("diffInternal() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkAlgorithmComparison compares all algorithms with the same input
// 算法对比基准测试
func BenchmarkAlgorithmComparison(b *testing.B) {
	ctx := context.Background()
	sizes := []int{100, 1000, 5000}
	changeRates := []float64{0.1, 0.5}

	for _, size := range sizes {
		for _, changeRate := range changeRates {
			before := generateSequence(size, 0)
			after := generateModifiedSequence(before, changeRate)

			name := fmt.Sprintf("size_%d_change_%.0f", size, changeRate*100)

			b.Run(name+"_myers", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					_, _ = diffInternal(ctx, before, after, Myers)
				}
			})

			b.Run(name+"_histogram", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					_, _ = diffInternal(ctx, before, after, Histogram)
				}
			})

			b.Run(name+"_onp", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					_, _ = diffInternal(ctx, before, after, ONP)
				}
			})

			b.Run(name+"_patience", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					_, _ = diffInternal(ctx, before, after, Patience)
				}
			})
		}
	}
}

// BenchmarkSpecialCases benchmarks special edge cases
// 特殊情况基准测试
func BenchmarkSpecialCases(b *testing.B) {
	ctx := context.Background()

	// Benchmark identical inputs
	b.Run("identical", func(b *testing.B) {
		input := generateSequence(1000, 0)
		b.ResetTimer()
		for range b.N {
			_, _ = diffInternal(ctx, input, input, Myers)
		}
	})

	// Benchmark completely different inputs
	b.Run("completely_different", func(b *testing.B) {
		before := generateSequence(1000, 0)
		after := generateSequence(1000, 1)
		b.ResetTimer()
		for range b.N {
			_, _ = diffInternal(ctx, before, after, Myers)
		}
	})

	// Benchmark single insertion
	b.Run("single_insertion", func(b *testing.B) {
		before := generateSequence(1000, 0)
		after := make([]string, len(before)+1)
		copy(after[:500], before[:500])
		after[500] = "inserted_line"
		copy(after[501:], before[500:])
		b.ResetTimer()
		for range b.N {
			_, _ = diffInternal(ctx, before, after, Myers)
		}
	})

	// Benchmark single deletion
	b.Run("single_deletion", func(b *testing.B) {
		before := generateSequence(1000, 0)
		after := make([]string, len(before)-1)
		copy(after[:500], before[:500])
		copy(after[500:], before[501:])
		b.ResetTimer()
		for range b.N {
			_, _ = diffInternal(ctx, before, after, Myers)
		}
	})
}

// BenchmarkDiffRunes benchmarks rune-level diff
// 字符级 diff 基准测试
func BenchmarkDiffRunes(b *testing.B) {
	ctx := context.Background()
	tests := []struct {
		name string
		algo Algorithm
		a    string
		b    string
	}{
		{"small_myers", Myers, "Hello World", "Hello There"},
		{"small_histogram", Histogram, "Hello World", "Hello There"},
		{"medium_myers", Myers, strings.Repeat("Hello World ", 100), strings.Repeat("Hello There ", 100)},
		{"medium_histogram", Histogram, strings.Repeat("Hello World ", 100), strings.Repeat("Hello There ", 100)},
		{"large_myers", Myers, strings.Repeat("Hello World ", 1000), strings.Repeat("Hello There ", 1000)},
		{"large_histogram", Histogram, strings.Repeat("Hello World ", 1000), strings.Repeat("Hello There ", 1000)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, err := DiffRunes(ctx, tt.a, tt.b, tt.algo)
				if err != nil {
					b.Fatalf("DiffRunes() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkDiffWords benchmarks word-level diff
// 单词级 diff 基准测试
func BenchmarkDiffWords(b *testing.B) {
	ctx := context.Background()
	tests := []struct {
		name string
		algo Algorithm
		a    string
		b    string
	}{
		{"small_myers", Myers, "The quick brown fox", "The quick brown dog"},
		{"small_histogram", Histogram, "The quick brown fox", "The quick brown dog"},
		{"medium_myers", Myers, strings.Repeat("The quick brown fox jumps ", 50), strings.Repeat("The quick brown dog jumps ", 50)},
		{"medium_histogram", Histogram, strings.Repeat("The quick brown fox jumps ", 50), strings.Repeat("The quick brown dog jumps ", 50)},
		{"large_myers", Myers, strings.Repeat("The quick brown fox jumps ", 500), strings.Repeat("The quick brown dog jumps ", 500)},
		{"large_histogram", Histogram, strings.Repeat("The quick brown fox jumps ", 500), strings.Repeat("The quick brown dog jumps ", 500)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, err := DiffWords(ctx, tt.a, tt.b, tt.algo, nil)
				if err != nil {
					b.Fatalf("DiffWords() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkHelperFunctions benchmarks helper functions
// 辅助函数基准测试
func BenchmarkHelperFunctions(b *testing.B) {
	// Benchmark commonPrefixLength
	b.Run("commonPrefixLength", func(b *testing.B) {
		a := generateSequence(1000, 0)
		b_ := generateSequence(1000, 0.1)

		b.ResetTimer()
		for range b.N {
			_ = commonPrefixLength(a, b_)
		}
	})

	// Benchmark commonSuffixLength
	b.Run("commonSuffixLength", func(b *testing.B) {
		a := generateSequence(1000, 0)
		b_ := generateSequence(1000, 0.1)

		b.ResetTimer()
		for range b.N {
			_ = commonSuffixLength(a, b_)
		}
	})
}

// BenchmarkWithRealWorldData simulates real-world diff scenarios
// 真实场景模拟基准测试
func BenchmarkWithRealWorldData(b *testing.B) {
	ctx := context.Background()

	// Simulate code file with function changes
	codeBefore := `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
    greet("Alice")
    greet("Bob")
    process(100)
}

func greet(name string) {
    fmt.Printf("Hello, %s!\n", name)
}

func process(n int) {
    for range n {
        fmt.Println("processed")
    }
}
`

	codeAfter := `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
    greet("Alice")
    greet("Charlie")
    process(1000)
    cleanup()
}

func greet(name string) {
    fmt.Printf("Greetings, %s!\n", name)
}

func process(n int) {
    for i := range n {
        fmt.Printf("Processing: %d\n", i)
    }
}

func cleanup() {
    fmt.Println("Cleaning up...")
}
`

	b.Run("code_diff_myers", func(b *testing.B) {
		beforeLines := splitLines(codeBefore)
		afterLines := splitLines(codeAfter)

		b.ResetTimer()
		for range b.N {
			_, _ = diffInternal(ctx, beforeLines, afterLines, Myers)
		}
	})

	b.Run("code_diff_histogram", func(b *testing.B) {
		beforeLines := splitLines(codeBefore)
		afterLines := splitLines(codeAfter)

		b.ResetTimer()
		for range b.N {
			_, _ = diffInternal(ctx, beforeLines, afterLines, Histogram)
		}
	})

	// Simulate text document changes
	textBefore := strings.Repeat("This is a sample document with some content. ", 100)
	textAfter := strings.Replace(textBefore, "sample", "detailed", 10)
	textAfter = strings.Replace(textAfter, "content", "information", 15)

	b.Run("text_diff_runes", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_, _ = DiffRunes(ctx, textBefore, textAfter, Histogram)
		}
	})

	b.Run("text_diff_words", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_, _ = DiffWords(ctx, textBefore, textAfter, Histogram, nil)
		}
	})
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
// 内存分配模式基准测试
func BenchmarkMemoryAllocation(b *testing.B) {
	ctx := context.Background()

	algos := []Algorithm{Myers, Histogram, ONP, Patience}

	for _, algo := range algos {
		b.Run(algo.String(), func(b *testing.B) {
			before := generateSequence(1000, 0)
			after := generateModifiedSequence(before, 0.3)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_, err := diffInternal(ctx, before, after, algo)
				if err != nil {
					b.Fatalf("diffInternal() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkParallel benchmarks parallel execution
// 并行执行基准测试
func BenchmarkParallel(b *testing.B) {
	ctx := context.Background()
	before := generateSequence(1000, 0)
	after := generateModifiedSequence(before, 0.3)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := diffInternal(ctx, before, after, Myers)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Helper function to split text into lines
// 将文本分割成行的辅助函数
func splitLines(text string) []string {
	lines := make([]string, 0)
	start := 0
	for i, r := range text {
		if r == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

// In Go 1.20+, the random generator is automatically seeded
