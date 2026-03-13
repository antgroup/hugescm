package diferenco

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSuffixArrayDiff(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	sink := &Sink{
		Index: make(map[string]int),
	}
	a := sink.SplitLines(textA)
	b := sink.SplitLines(textB)
	changes, _ := SuffixArrayDiff(t.Context(), a, b)
	i := 0
	for _, c := range changes {
		for ; i < c.P1; i++ {
			fmt.Fprintf(os.Stderr, "  %s", sink.Lines[a[i]])
		}
		for j := c.P1; j < c.P1+c.Del; j++ {
			fmt.Fprintf(os.Stderr, "- %s", sink.Lines[a[j]])
		}
		for j := c.P2; j < c.P2+c.Ins; j++ {
			fmt.Fprintf(os.Stderr, "+ %s", sink.Lines[b[j]])
		}
		i += c.Del
	}
	for ; i < len(a); i++ {
		fmt.Fprintf(os.Stderr, "  %s", sink.Lines[a[i]])
	}
	fmt.Fprintf(os.Stderr, "\n\nEND\n\n")
}

func TestSuffixArrayDiffBasic(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []Change
	}{
		{
			name:     "empty both",
			a:        []string{},
			b:        []string{},
			expected: []Change{},
		},
		{
			name: "empty a",
			a:    []string{},
			b:    []string{"a", "b", "c"},
			expected: []Change{
				{P1: 0, P2: 0, Ins: 3},
			},
		},
		{
			name: "empty b",
			a:    []string{"a", "b", "c"},
			b:    []string{},
			expected: []Change{
				{P1: 0, P2: 0, Del: 3},
			},
		},
		{
			name:     "identical",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: []Change{},
		},
		{
			name: "single_insertion",
			a:    []string{"a", "c"},
			b:    []string{"a", "b", "c"},
			expected: []Change{
				{P1: 1, P2: 1, Ins: 1},
			},
		},
		{
			name: "single_deletion",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "c"},
			expected: []Change{
				{P1: 1, P2: 1, Del: 1},
			},
		},
		{
			name: "replace_middle",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "x", "c"},
			expected: []Change{
				{P1: 1, P2: 1, Del: 1, Ins: 1},
			},
		},
		{
			name: "completely_different",
			a:    []string{"a", "b", "c"},
			b:    []string{"x", "y", "z"},
			expected: []Change{
				{P1: 0, P2: 0, Del: 3, Ins: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			changes, err := SuffixArrayDiff(ctx, tt.a, tt.b)
			if err != nil {
				t.Fatalf("SuffixArrayDiff() error = %v", err)
			}

			// Verify the changes reconstruct the correct result
			result := reconstructFromChanges(tt.a, changes, tt.b)
			if !equalSlices(result, tt.b) {
				t.Errorf("SuffixArrayDiff() reconstructed = %v, want %v", result, tt.b)
			}
		})
	}
}

func TestSuffixArrayDiffRune(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
	}{
		{
			name: "simple",
			a:    "Hello World",
			b:    "Hello There",
		},
		{
			name: "insertion",
			a:    "abc",
			b:    "abXc",
		},
		{
			name: "deletion",
			a:    "abXc",
			b:    "abc",
		},
		{
			name: "complex",
			a:    "The quick brown fox jumps over the lazy dog",
			b:    "The quick brown dog jumps over the lazy fox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runesA := []rune(tt.a)
			runesB := []rune(tt.b)
			changes, err := SuffixArrayDiff(ctx, runesA, runesB)
			if err != nil {
				t.Fatalf("SuffixArrayDiff() error = %v", err)
			}

			// Verify changes are valid
			for i, c := range changes {
				if c.P1 < 0 || c.P1 > len(runesA) {
					t.Errorf("Change[%d].P1 = %d out of range [0, %d]", i, c.P1, len(runesA))
				}
				if c.P2 < 0 || c.P2 > len(runesB) {
					t.Errorf("Change[%d].P2 = %d out of range [0, %d]", i, c.P2, len(runesB))
				}
				if c.Del < 0 || c.P1+c.Del > len(runesA) {
					t.Errorf("Change[%d].Del = %d invalid with P1=%d, lenA=%d", i, c.Del, c.P1, len(runesA))
				}
				if c.Ins < 0 || c.P2+c.Ins > len(runesB) {
					t.Errorf("Change[%d].Ins = %d invalid with P2=%d, lenB=%d", i, c.Ins, c.P2, len(runesB))
				}
			}

			// Verify reconstruction
			result := reconstructFromChanges(runesA, changes, runesB)
			if !equalSlices(result, runesB) {
				t.Errorf("SuffixArrayDiff() reconstructed = %v, want %v", string(result), string(runesB))
			}
		})
	}
}

func TestSuffixArrayDiffConsistency(t *testing.T) {
	// Test that SuffixArray produces consistent results with other algorithms
	tests := []struct {
		name string
		a    []string
		b    []string
	}{
		{
			name: "simple",
			a:    []string{"line1", "line2", "line3"},
			b:    []string{"line1", "modified", "line3"},
		},
		{
			name: "multiple_changes",
			a:    []string{"a", "b", "c", "d", "e"},
			b:    []string{"a", "x", "c", "y", "e"},
		},
		{
			name: "insert_delete",
			a:    []string{"a", "b", "c", "d", "e"},
			b:    []string{"a", "c", "d", "f", "e"},
		},
	}

	algorithms := []Algorithm{Histogram, ONP, Myers, Patience, SuffixArray}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results := make(map[Algorithm][]Change)

			for _, algo := range algorithms {
				changes, err := diffInternal(ctx, tt.a, tt.b, algo)
				if err != nil {
					t.Fatalf("Algorithm %s failed: %v", algo, err)
				}
				results[algo] = changes

				// Verify each algorithm produces valid results
				reconstructed := reconstructFromChanges(tt.a, changes, tt.b)
				if !equalSlices(reconstructed, tt.b) {
					t.Errorf("Algorithm %s: reconstructed = %v, want %v", algo, reconstructed, tt.b)
				}
			}

			// All algorithms should produce correct reconstruction
			// (The exact changes may differ, but the result should be the same)
		})
	}
}

func TestBuildSuffixArray(t *testing.T) {
	tests := []struct {
		name    string
		data    []string
		wantLen int
	}{
		{
			name:    "empty",
			data:    []string{},
			wantLen: 0,
		},
		{
			name:    "single",
			data:    []string{"a"},
			wantLen: 1,
		},
		{
			name:    "simple",
			data:    []string{"b", "a", "n", "a", "n", "a"},
			wantLen: 6,
		},
		{
			name:    "sorted",
			data:    []string{"a", "b", "c", "d", "e"},
			wantLen: 5,
		},
		{
			name:    "reverse",
			data:    []string{"e", "d", "c", "b", "a"},
			wantLen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := buildSuffixArray(tt.data)
			if len(sa) != tt.wantLen {
				t.Errorf("buildSuffixArray() length = %d, want %d", len(sa), tt.wantLen)
			}

			// Verify suffix array is valid (all indices present)
			if len(sa) > 0 {
				seen := make(map[int]bool)
				for _, idx := range sa {
					if idx < 0 || idx >= len(tt.data) {
						t.Errorf("Invalid index in suffix array: %d", idx)
					}
					if seen[idx] {
						t.Errorf("Duplicate index in suffix array: %d", idx)
					}
					seen[idx] = true
				}

				// Verify suffix array is sorted
				for i := 1; i < len(sa); i++ {
					cmp := compareSuffixes(tt.data, sa[i-1], sa[i])
					if cmp >= 0 {
						t.Errorf("Suffix array not sorted at position %d: sa[%d]=%d, sa[%d]=%d", i, i-1, sa[i-1], i, sa[i])
					}
				}
			}
		})
	}
}

func TestSuffixArrayDiffAlgorithm(t *testing.T) {
	// Test that the algorithm can be selected by name
	algo, err := AlgorithmFromName("suffixarray")
	if err != nil {
		t.Fatalf("AlgorithmFromName() error = %v", err)
	}
	if algo != SuffixArray {
		t.Errorf("AlgorithmFromName() = %v, want %v", algo, SuffixArray)
	}

	// Test string representation
	if SuffixArray.String() != "suffixarray" {
		t.Errorf("SuffixArray.String() = %q, want %q", SuffixArray.String(), "suffixarray")
	}
}

func TestSuffixArrayDiffContext(t *testing.T) {
	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := []string{"a", "b", "c"}
	b := []string{"a", "x", "c"}

	_, err := SuffixArrayDiff(ctx, a, b)
	if err == nil {
		t.Error("SuffixArrayDiff() should return error on cancelled context")
	}
}

func TestSuffixArrayDiffBinary(t *testing.T) {
	// Test with binary data (byte slices)
	tests := []struct {
		name string
		a    []byte
		b    []byte
	}{
		{
			name: "simple_binary",
			a:    []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			b:    []byte{0x00, 0x01, 0xFF, 0x03, 0x04},
		},
		{
			name: "insert_binary",
			a:    []byte{0x00, 0x01, 0x02},
			b:    []byte{0x00, 0x01, 0x0A, 0x02},
		},
		{
			name: "delete_binary",
			a:    []byte{0x00, 0x01, 0x0A, 0x02},
			b:    []byte{0x00, 0x01, 0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			changes, err := SuffixArrayDiff(ctx, tt.a, tt.b)
			if err != nil {
				t.Fatalf("SuffixArrayDiff() error = %v", err)
			}

			// Verify reconstruction
			result := reconstructFromChanges(tt.a, changes, tt.b)
			if !equalSlices(result, tt.b) {
				t.Errorf("SuffixArrayDiff() reconstructed = %v, want %v", result, tt.b)
			}
		})
	}
}

// Helper functions

func reconstructFromChanges[E comparable](a []E, changes []Change, b []E) []E {
	result := make([]E, 0, len(b))
	posA := 0
	posB := 0

	for _, c := range changes {
		// Add equal elements before the change
		for posA < c.P1 {
			result = append(result, a[posA])
			posA++
			posB++
		}

		// Skip deleted elements
		posA += c.Del

		// Add inserted elements
		for i := 0; i < c.Ins; i++ {
			result = append(result, b[posB])
			posB++
		}
	}

	// Add remaining equal elements
	for posA < len(a) {
		result = append(result, a[posA])
		posA++
	}

	return result
}

func equalSlices[E comparable](a, b []E) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
