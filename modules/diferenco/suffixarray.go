// Package diferenco provides diff algorithms.
// Suffix-Array Diff implementation inspired by diff-match-patch.
package diferenco

import (
	"cmp"
	"context"
	"slices"
)

// match represents a common substring match between two sequences.
type match struct {
	start1 int // start position in sequence 1
	start2 int // start position in sequence 2
	length int // length of the match
}

// buildSuffixArray constructs a suffix array for the given data.
func buildSuffixArray[E cmp.Ordered](data []E) []int {
	n := len(data)
	if n <= 1 {
		if n == 1 {
			return []int{0}
		}
		return nil
	}

	sa := make([]int, n)
	for i := range n {
		sa[i] = i
	}

	slices.SortFunc(sa, func(i, j int) int {
		return compareSuffixes(data, i, j)
	})

	return sa
}

// compareSuffixes compares two suffixes starting at positions i and j.
func compareSuffixes[E cmp.Ordered](data []E, i, j int) int {
	n := len(data)
	for i < n && j < n {
		if c := cmp.Compare(data[i], data[j]); c != 0 {
			return c
		}
		i++
		j++
	}
	return cmp.Compare(n-i, n-j)
}

// findLongestCommonSubstring finds the longest common substring between two sequences
// using suffix array on the first sequence.
func findLongestCommonSubstring[E cmp.Ordered](data1, data2 []E, sa []int) match {
	if len(data1) == 0 || len(data2) == 0 || len(sa) == 0 {
		return match{}
	}

	var bestMatch match

	// For each starting position in data2, binary search in suffix array
	for start2 := range len(data2) {
		matchLen := binarySearchMatch(data1, data2, sa, start2)
		if matchLen > bestMatch.length {
			bestMatch.start2 = start2
			bestMatch.length = matchLen
		}
	}

	// Find the start position in data1 for the best match
	if bestMatch.length > 0 {
		bestMatch.start1 = findSuffixPosition(data1, data2, sa, bestMatch.start2, bestMatch.length)
	}

	return bestMatch
}

// binarySearchMatch finds the longest match for data2[start2:] in data1 using suffix array.
func binarySearchMatch[E cmp.Ordered](data1, data2 []E, sa []int, start2 int) int {
	if len(data2) == 0 || start2 >= len(data2) {
		return 0
	}

	n := len(data1)
	target := data2[start2:]

	// Binary search for lower bound
	pos, _ := slices.BinarySearchFunc(sa, target[0], func(suffixIdx int, firstElem E) int {
		if suffixIdx >= n {
			return -1
		}
		return cmp.Compare(data1[suffixIdx], firstElem)
	})

	bestLen := 0

	// Check nearby suffixes for matches
	end := min(pos+10, n) // Limit search range for efficiency
	for i := pos; i < end; i++ {
		suffixStart := sa[i]
		if suffixStart >= n || data1[suffixStart] != target[0] {
			break
		}

		// Count matching elements
		maxLen := min(n-suffixStart, len(target))
		matchLen := 0
		for matchLen < maxLen && data1[suffixStart+matchLen] == target[matchLen] {
			matchLen++
		}

		bestLen = max(bestLen, matchLen)
	}

	return bestLen
}

// findSuffixPosition finds the starting position in data1 for a match.
func findSuffixPosition[E cmp.Ordered](data1, data2 []E, sa []int, start2, length int) int {
	if length == 0 {
		return 0
	}

	target := data2[start2 : start2+length]

	// Binary search for the suffix
	pos, found := slices.BinarySearchFunc(sa, target, func(suffixIdx int, t []E) int {
		if suffixIdx >= len(data1) {
			return 1
		}
		return slices.Compare(data1[suffixIdx:], t)
	})

	if found {
		return sa[pos]
	}
	if pos < len(sa) {
		return sa[pos]
	}
	return 0
}

// suffixArrayComputeOrdered performs the recursive diff computation using suffix array.
func suffixArrayComputeOrdered[E cmp.Ordered](ctx context.Context, L1 []E, P1 int, L2 []E, P2 int) ([]Change, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Base cases
	switch {
	case len(L1) == 0 && len(L2) == 0:
		return []Change{}, nil
	case len(L1) == 0:
		return []Change{{P1: P1, P2: P2, Ins: len(L2)}}, nil
	case len(L2) == 0:
		return []Change{{P1: P1, P2: P2, Del: len(L1)}}, nil
	}

	// Check for common prefix
	prefixLen := 0
	for prefixLen < len(L1) && prefixLen < len(L2) && L1[prefixLen] == L2[prefixLen] {
		prefixLen++
	}
	if prefixLen > 0 {
		return suffixArrayComputeOrdered(ctx, L1[prefixLen:], P1+prefixLen, L2[prefixLen:], P2+prefixLen)
	}

	// Check for common suffix
	suffixLen := 0
	for suffixLen < len(L1) && suffixLen < len(L2) && L1[len(L1)-1-suffixLen] == L2[len(L2)-1-suffixLen] {
		suffixLen++
	}
	if suffixLen > 0 {
		return suffixArrayComputeOrdered(ctx, L1[:len(L1)-suffixLen], P1, L2[:len(L2)-suffixLen], P2)
	}

	// Build suffix array for L1
	sa := buildSuffixArray(L1)

	// Find longest common substring
	lcs := findLongestCommonSubstring(L1, L2, sa)

	// If no common substring found, return all as changes
	if lcs.length == 0 {
		return []Change{{P1: P1, P2: P2, Del: len(L1), Ins: len(L2)}}, nil
	}

	// Recursively process left and right parts
	// Process left part (before the match)
	leftChanges, err := suffixArrayComputeOrdered(ctx, L1[:lcs.start1], P1, L2[:lcs.start2], P2)
	if err != nil {
		return nil, err
	}

	// Process right part (after the match)
	rightStart1 := lcs.start1 + lcs.length
	rightStart2 := lcs.start2 + lcs.length
	rightChanges, err := suffixArrayComputeOrdered(ctx, L1[rightStart1:], P1+rightStart1, L2[rightStart2:], P2+rightStart2)
	if err != nil {
		return nil, err
	}

	return append(leftChanges, rightChanges...), nil
}

// SuffixArrayDiff calculates the difference using suffix array algorithm.
// This algorithm is efficient for finding longest common substrings and works well
// for both text and binary data.
//
// Time complexity: O((n+m) log n) where n and m are the lengths of the input sequences.
// Space complexity: O(n) for the suffix array.
func suffixArray[E comparable](ctx context.Context, L1, L2 []E) ([]Change, error) {
	// Handle empty inputs
	if len(L1) == 0 && len(L2) == 0 {
		return []Change{}, nil
	}

	// Remove common prefix
	prefix := commonPrefixLength(L1, L2)
	L1 = L1[prefix:]
	L2 = L2[prefix:]

	// Remove common suffix
	suffix := commonSuffixLength(L1, L2)
	L1 = L1[:len(L1)-suffix]
	L2 = L2[:len(L2)-suffix]

	// If either slice is empty after removing prefix/suffix
	if len(L1) == 0 && len(L2) == 0 {
		return []Change{}, nil
	}

	// Try ordered types using type assertion helper
	if changes, err, ok := trySuffixArrayDiff(ctx, L1, L2, prefix); ok {
		return changes, err
	}

	// Fallback to ONP algorithm for unsupported types
	return onp(ctx, L1, L2)
}

// trySuffixArrayDiff attempts to run suffix array diff for ordered types.
// Returns (changes, err, true) if the type is supported, or (nil, nil, false) if not.
func trySuffixArrayDiff[E comparable](ctx context.Context, L1, L2 []E, prefix int) ([]Change, error, bool) {
	switch any(L1).(type) {
	case []string:
		changes, err := suffixArrayComputeOrdered(ctx, any(L1).([]string), prefix, any(L2).([]string), prefix)
		return changes, err, true
	case []int:
		changes, err := suffixArrayComputeOrdered(ctx, any(L1).([]int), prefix, any(L2).([]int), prefix)
		return changes, err, true
	case []int64:
		changes, err := suffixArrayComputeOrdered(ctx, any(L1).([]int64), prefix, any(L2).([]int64), prefix)
		return changes, err, true
	case []rune:
		changes, err := suffixArrayComputeOrdered(ctx, any(L1).([]rune), prefix, any(L2).([]rune), prefix)
		return changes, err, true
	case []byte:
		changes, err := suffixArrayComputeOrdered(ctx, any(L1).([]byte), prefix, any(L2).([]byte), prefix)
		return changes, err, true
	default:
		return nil, nil, false
	}
}
