package diferenco

import (
	"context"
	"math/rand"
	"slices"
	"testing"
)

// patienceLCSLegacy is the original O(n²) implementation for benchmark comparison
func patienceLCSLegacy[E comparable](a, b []E) [][2]int {
	// Initialize the LCS table.
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}

	// Populate the LCS table.
	for i := 1; i < len(lcs); i++ {
		for j := 1; j < len(lcs[i]); j++ {
			if a[i-1] == b[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				lcs[i][j] = max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}

	// Backtrack to find the LCS.
	i, j := len(a), len(b)
	s := make([][2]int, 0, lcs[i][j])
	for i > 0 && j > 0 {
		switch {
		case a[i-1] == b[j-1]:
			s = append(s, [2]int{i - 1, j - 1})
			i--
			j--
		case lcs[i-1][j] > lcs[i][j-1]:
			i--
		default:
			j--
		}
	}

	slices.Reverse(s)
	return s
}

func generateUniqueLinesPatience(n int) []string {
	seen := make(map[string]bool, n)
	lines := make([]string, 0, n)
	for len(lines) < n {
		s := randStringPatience(20)
		if !seen[s] {
			seen[s] = true
			lines = append(lines, s)
		}
	}
	return lines
}

func randStringPatience(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func BenchmarkPatienceLCSLegacy_Small(b *testing.B) {
	a := generateUniqueLinesPatience(50)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	b.ResetTimer()
	for b.Loop() {
		_ = patienceLCSLegacy(ua, ub)
	}
}

func BenchmarkPatienceLCS_Small(b *testing.B) {
	a := generateUniqueLinesPatience(50)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	b.ResetTimer()
	for b.Loop() {
		_ = patienceLCS(ua, ub)
	}
}

func BenchmarkPatienceLCSLegacy_Medium(b *testing.B) {
	a := generateUniqueLinesPatience(200)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	b.ResetTimer()
	for b.Loop() {
		_ = patienceLCSLegacy(ua, ub)
	}
}

func BenchmarkPatienceLCS_Medium(b *testing.B) {
	a := generateUniqueLinesPatience(200)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	b.ResetTimer()
	for b.Loop() {
		_ = patienceLCS(ua, ub)
	}
}

func BenchmarkPatienceLCSLegacy_Large(b *testing.B) {
	a := generateUniqueLinesPatience(500)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	b.ResetTimer()
	for b.Loop() {
		_ = patienceLCSLegacy(ua, ub)
	}
}

func BenchmarkPatienceLCS_Large(b *testing.B) {
	a := generateUniqueLinesPatience(500)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	b.ResetTimer()
	for b.Loop() {
		_ = patienceLCS(ua, ub)
	}
}

// Test LCS correctness - verify O(n log n) produces same results as O(n²)
func TestPatienceLCSCorrectness(t *testing.T) {
	a := generateUniqueLinesPatience(100)
	c := make([]string, len(a))
	copy(c, a)
	rand.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })

	ua, _ := uniqueElements(a)
	ub, _ := uniqueElements(c)

	result1 := patienceLCSLegacy(ua, ub)
	result2 := patienceLCS(ua, ub)

	// Both should find same length LCS
	if len(result1) != len(result2) {
		t.Errorf("LCS length mismatch: legacy=%d, optimized=%d", len(result1), len(result2))
	}

	// Verify result is valid
	for _, p := range result2 {
		if ua[p[0]] != ub[p[1]] {
			t.Errorf("Invalid match: a[%d]=%v, b[%d]=%v", p[0], ua[p[0]], p[1], ub[p[1]])
		}
	}
}

// patienceComputeLegacy uses the legacy O(n²) LCS implementation
func patienceComputeLegacy[E comparable](ctx context.Context, L1 []E, P1 int, L2 []E, P2 int) ([]Change, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if len(L1) == 0 && len(L2) == 0 {
		return []Change{}, nil
	}
	if len(L1) == 0 {
		return []Change{{P1: P1, P2: P2, Ins: len(L2)}}, nil
	}
	if len(L2) == 0 {
		return []Change{{P1: P1, P2: P2, Del: len(L1)}}, nil
	}

	i := 0
	for i < len(L1) && i < len(L2) && L1[i] == L2[i] {
		i++
	}
	if i > 0 {
		return patienceComputeLegacy(ctx, L1[i:], P1+i, L2[i:], P2+i)
	}
	j := 0
	for j < len(L1) && j < len(L2) && L1[len(L1)-1-j] == L2[len(L2)-1-j] {
		j++
	}
	if j > 0 {
		return patienceComputeLegacy(ctx, L1[:len(L1)-j], P1, L2[:len(L2)-j], P2)
	}

	ua, idxa := uniqueElements(L1)
	ub, idxb := uniqueElements(L2)
	lcs := patienceLCSLegacy(ua, ub) // Use legacy LCS

	if len(lcs) == 0 {
		return []Change{{P1: P1, P2: P2, Del: len(L1), Ins: len(L2)}}, nil
	}

	for i, x := range lcs {
		lcs[i][0] = idxa[x[0]]
		lcs[i][1] = idxb[x[1]]
	}
	changes := make([]Change, 0, 10)
	ga, gb := 0, 0
	for _, ip := range lcs {
		sub, err := patienceComputeLegacy(ctx, L1[ga:ip[0]], P1+ga, L2[gb:ip[1]], P2+gb)
		if err != nil {
			return nil, err
		}
		changes = append(changes, sub...)
		ga = ip[0] + 1
		gb = ip[1] + 1
	}
	sub, err := patienceComputeLegacy(ctx, L1[ga:], P1+ga, L2[gb:], P2+gb)
	if err != nil {
		return nil, err
	}
	changes = append(changes, sub...)
	return changes, nil
}

// DiffSlicesLegacy uses the O(n²) LCS implementation for benchmark comparison
func DiffSlicesLegacy[E comparable](ctx context.Context, L1, L2 []E) ([]Change, error) {
	prefix := commonPrefixLength(L1, L2)
	L1 = L1[prefix:]
	L2 = L2[prefix:]
	suffix := commonSuffixLength(L1, L2)
	L1 = L1[:len(L1)-suffix]
	L2 = L2[:len(L2)-suffix]
	return patienceComputeLegacy(ctx, L1, prefix, L2, prefix)
}

// Benchmark full diff algorithm
func BenchmarkDiffSlicesLegacy(b *testing.B) {
	ctx := context.Background()
	a := generateUniqueLinesPatience(200)
	c := make([]string, len(a))
	copy(c, a)
	for range 20 {
		idx := rand.Intn(len(c))
		c[idx] = randStringPatience(20)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = DiffSlicesLegacy(ctx, a, c)
	}
}

func BenchmarkPatienceDiff(b *testing.B) {
	ctx := context.Background()
	a := generateUniqueLinesPatience(200)
	c := make([]string, len(a))
	copy(c, a)
	for range 20 {
		idx := rand.Intn(len(c))
		c[idx] = randStringPatience(20)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = DiffSlices(ctx, a, c, Patience)
	}
}

// Test diff equivalence - both implementations should produce same results
func TestPatienceDiffEquivalence(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		a, b []string
	}{
		{
			name: "simple",
			a:    []string{"a", "b", "c", "d", "e"},
			b:    []string{"a", "c", "d", "f", "e"},
		},
		{
			name: "insert",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "x", "c"},
		},
		{
			name: "delete",
			a:    []string{"a", "b", "c", "d"},
			b:    []string{"a", "c", "d"},
		},
		{
			name: "replace",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "x", "c"},
		},
		{
			name: "reorder",
			a:    []string{"a", "b", "c", "d", "e"},
			b:    []string{"e", "d", "c", "b", "a"},
		},
		{
			name: "random_100",
			a:    generateUniqueLinesPatience(100),
			b: func() []string {
				s := generateUniqueLinesPatience(100)
				rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
				return s
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes1, err := DiffSlicesLegacy(ctx, tt.a, tt.b)
			if err != nil {
				t.Fatalf("DiffSlicesLegacy error: %v", err)
			}

			changes2, err := DiffSlices(ctx, tt.a, tt.b, Patience)
			if err != nil {
				t.Fatalf("PatienceDiff error: %v", err)
			}

			// Compare total deletions and insertions
			var del1, ins1, del2, ins2 int
			for _, c := range changes1 {
				del1 += c.Del
				ins1 += c.Ins
			}
			for _, c := range changes2 {
				del2 += c.Del
				ins2 += c.Ins
			}

			if del1 != del2 || ins1 != ins2 {
				t.Errorf("Diff mismatch: legacy (del=%d, ins=%d), optimized (del=%d, ins=%d)",
					del1, ins1, del2, ins2)
			}

			t.Logf("Both implementations: %d changes, %d del, %d ins", len(changes1), del1, ins1)
		})
	}
}
