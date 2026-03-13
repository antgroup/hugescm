// MIT License

// Copyright (c) 2022 Peter Evans

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package diferenco

import (
	"context"
	"slices"
)

// uniqueElements returns a slice of unique elements from a slice of
// strings, and a slice of the original indices of each element.
func uniqueElements[E comparable](a []E) ([]E, []int) {
	m := make(map[E]int)
	for _, e := range a {
		m[e]++
	}
	elements := []E{}
	indices := []int{}
	for i, e := range a {
		if m[e] == 1 {
			elements = append(elements, e)
			indices = append(indices, i)
		}
	}
	return elements, indices
}

// patienceLCS computes the longest common subsequence of two string
// slices and returns the index pairs of the patienceLCS.
// Uses O(n log n) LIS algorithm for better performance.
func patienceLCS[E comparable](a, b []E) [][2]int {
	// Build index map for unique elements in b
	pos := make(map[E]int, len(b))
	count := make(map[E]int, len(b))
	for _, e := range b {
		count[e]++
	}
	for i, e := range b {
		if count[e] == 1 {
			pos[e] = i
		}
	}

	// Build sequence of matching pairs (unique elements that appear in both)
	type pair struct {
		i int
		j int
	}
	pairs := make([]pair, 0, len(a))
	for i, e := range a {
		if j, ok := pos[e]; ok {
			pairs = append(pairs, pair{i, j})
		}
	}

	if len(pairs) == 0 {
		return nil
	}

	// LIS on j values using O(n log n) algorithm
	n := len(pairs)
	tails := make([]int, 0, n)
	prev := make([]int, n)
	for i := range prev {
		prev[i] = -1
	}

	for i, p := range pairs {
		j := p.j

		// Binary search for the position to insert
		lo, hi := 0, len(tails)
		for lo < hi {
			mid := (lo + hi) / 2
			if pairs[tails[mid]].j < j {
				lo = mid + 1
			} else {
				hi = mid
			}
		}

		if lo == len(tails) {
			tails = append(tails, i)
		} else {
			tails[lo] = i
		}

		if lo > 0 {
			prev[i] = tails[lo-1]
		}
	}

	// Reconstruct LIS
	res := make([][2]int, 0, len(tails))
	k := tails[len(tails)-1]
	for k >= 0 {
		p := pairs[k]
		res = append(res, [2]int{p.i, p.j})
		k = prev[k]
	}

	slices.Reverse(res)
	return res
}

func patienceCompute[E comparable](ctx context.Context, L1 []E, P1 int, L2 []E, P2 int) ([]Change, error) {
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
		return patienceCompute(ctx, L1[i:], P1+i, L2[i:], P2+i)
	}
	// Find equal elements at the tail of slices a and b.
	j := 0
	for j < len(L1) && j < len(L2) && L1[len(L1)-1-j] == L2[len(L2)-1-j] {
		j++
	}
	if j > 0 {
		return patienceCompute(ctx, L1[:len(L1)-j], P1, L2[:len(L2)-j], P2)
	}
	// Find the longest common subsequence of unique elements in a and b.
	ua, idxa := uniqueElements(L1)
	ub, idxb := uniqueElements(L2)
	lcs := patienceLCS(ua, ub)

	// If the LCS is empty, the diff is all deletions and insertions.
	if len(lcs) == 0 {
		return []Change{{P1: P1, P2: P2, Del: len(L1), Ins: len(L2)}}, nil
	}

	// Lookup the original indices of slices a and b.
	for i, x := range lcs {
		lcs[i][0] = idxa[x[0]]
		lcs[i][1] = idxb[x[1]]
	}
	changes := make([]Change, 0, 10)
	ga, gb := 0, 0
	for _, ip := range lcs {
		// Diff the gaps between the lcs elements.
		sub, err := patienceCompute(ctx, L1[ga:ip[0]], P1+ga, L2[gb:ip[1]], P2+gb)
		if err != nil {
			return nil, err
		}
		// Append the LCS elements to the diff.
		changes = append(changes, sub...)
		ga = ip[0] + 1
		gb = ip[1] + 1
	}
	// Diff the remaining elements of a and b after the final LCS element.
	sub, err := patienceCompute(ctx, L1[ga:], P1+ga, L2[gb:], P2+gb)
	if err != nil {
		return nil, err
	}
	changes = append(changes, sub...)
	return changes, nil
}

// PatienceDiff: Calculates the difference using the patience algorithm
func PatienceDiff[E comparable](ctx context.Context, L1 []E, L2 []E) ([]Change, error) {
	prefix := commonPrefixLength(L1, L2)
	L1 = L1[prefix:]
	L2 = L2[prefix:]
	suffix := commonSuffixLength(L1, L2)
	L1 = L1[:len(L1)-suffix]
	L2 = L2[:len(L2)-suffix]
	return patienceCompute(ctx, L1, prefix, L2, prefix)
}
