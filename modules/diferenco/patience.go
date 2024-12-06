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

import "slices"

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
func patienceLCS[E comparable](a, b []E) [][2]int {
	// Initialize the LCS table.
	lcs := make([][]int, len(a)+1)
	for i := 0; i <= len(a); i++ {
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

	// Reverse the backtracked LCS.
	slices.Reverse(s)
	// for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
	// 	s[i], s[j] = s[j], s[i]
	// }
	return s
}

func PatienceDiff[E comparable](a, b []E) []Dfio[E] {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	if len(a) == 0 {
		return []Dfio[E]{{E: b, T: Insert}}
	}
	if len(b) == 0 {
		return []Dfio[E]{{E: a, T: Delete}}
	}
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	if i > 0 {
		return append([]Dfio[E]{{E: a[:i], T: Equal}}, PatienceDiff(a[i:], b[i:])...)
	}
	// Find equal elements at the tail of slices a and b.
	j := 0
	for j < len(a) && j < len(b) && a[len(a)-1-j] == b[len(b)-1-j] {
		j++
	}
	if j > 0 {
		return append(
			PatienceDiff(a[:len(a)-j], b[:len(b)-j]),
			Dfio[E]{E: a[len(a)-j:], T: Equal},
		)
	}
	// Find the longest common subsequence of unique elements in a and b.
	ua, idxa := uniqueElements(a)
	ub, idxb := uniqueElements(b)
	lcs := patienceLCS(ua, ub)

	// If the LCS is empty, the diff is all deletions and insertions.
	if len(lcs) == 0 {
		return append([]Dfio[E]{{E: a, T: Delete}}, []Dfio[E]{{E: b, T: Insert}}...)
	}

	// Lookup the original indices of slices a and b.
	for i, x := range lcs {
		lcs[i][0] = idxa[x[0]]
		lcs[i][1] = idxb[x[1]]
	}

	diffs := []Dfio[E]{}
	ga, gb := 0, 0
	for _, ip := range lcs {
		// Diff the gaps between the lcs elements.
		diffs = append(diffs, PatienceDiff(a[ga:ip[0]], b[gb:ip[1]])...)
		// Append the LCS elements to the diff.
		diffs = append(diffs, Dfio[E]{T: Equal, E: []E{a[ip[0]]}})
		ga = ip[0] + 1
		gb = ip[1] + 1
	}
	// Diff the remaining elements of a and b after the final LCS element.
	diffs = append(diffs, PatienceDiff(a[ga:], b[gb:])...)

	return diffs
}
