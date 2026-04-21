// Package fuzzy provides fuzzy string matching.
//
// A query matches a string when every character in the query appears in the
// string in order, but the characters need not be contiguous. Matching is
// case-insensitive by default.
//
// Adapted from github.com/koki-develop/go-fzf.
package fuzzy

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Match describes a single successful fuzzy match.
type Match struct {
	// Str is the original (unmodified) string that was matched.
	Str string
	// Index is the position of this string in the input slice.
	Index int
	// MatchedIndexes holds the rune indexes (0-based) of each query character
	// that was matched inside Str.
	MatchedIndexes []int
}

// MatchedByteRanges converts the rune-based MatchedIndexes into byte ranges
// within Str. Each returned ByteRange covers exactly one matched rune.
func (m Match) MatchedByteRanges() []ByteRange {
	if len(m.MatchedIndexes) == 0 {
		return nil
	}

	// Build a rune-index → byte-offset map for only the rune indexes we need.
	// We walk the string once, keeping a running rune counter.
	needed := make(map[int]struct{}, len(m.MatchedIndexes))
	for _, ri := range m.MatchedIndexes {
		needed[ri] = struct{}{}
	}

	type runePos struct {
		byteOffset int
		byteLen    int
	}
	found := make(map[int]runePos, len(needed))

	runeIdx := 0
	byteIdx := 0
	for byteIdx < len(m.Str) && len(found) < len(needed) {
		_, size := utf8.DecodeRuneInString(m.Str[byteIdx:])
		if _, ok := needed[runeIdx]; ok {
			found[runeIdx] = runePos{byteOffset: byteIdx, byteLen: size}
		}
		byteIdx += size
		runeIdx++
	}

	ranges := make([]ByteRange, 0, len(m.MatchedIndexes))
	for _, ri := range m.MatchedIndexes {
		rp := found[ri]
		ranges = append(ranges, ByteRange{Start: rp.byteOffset, End: rp.byteOffset + rp.byteLen})
	}
	return ranges
}

// ByteRange represents a half-open byte range [Start, End).
type ByteRange struct {
	Start int
	End   int
}

// Matches is a sortable slice of Match values.
// The default sort order ranks matches with fewer matched indexes first (shorter
// queries matched sooner), breaking ties by matched-index position
// (left-biased), then by original index.
type Matches []Match

func (m Matches) Len() int      { return len(m) }
func (m Matches) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m Matches) Less(i, j int) bool {
	mi, mj := m[i].MatchedIndexes, m[j].MatchedIndexes
	li, lj := len(mi), len(mj)
	if li != lj {
		return li < lj
	}
	for k := range li {
		if mi[k] != mj[k] {
			return mi[k] < mj[k]
		}
	}
	return m[i].Index < m[j].Index
}

// Option configures a fuzzy search.
type Option func(*option)

type option struct {
	caseSensitive bool
}

// WithCaseSensitive enables or disables case-sensitive matching.
// The default is case-insensitive.
func WithCaseSensitive(v bool) Option {
	return func(o *option) {
		o.caseSensitive = v
	}
}

// Find performs a fuzzy search of query against each string in items,
// returning only the matches, sorted by quality.
func Find(items []string, query string, opts ...Option) Matches {
	var o option
	for _, fn := range opts {
		fn(&o)
	}

	if !o.caseSensitive {
		query = strings.ToLower(query)
	}

	var result Matches
	for i, s := range items {
		if m, ok := match(s, query, o); ok {
			m.Index = i
			result = append(result, m)
		}
	}

	sort.Sort(result)
	return result
}

// match checks whether query fuzzy-matches str and returns the Match if so.
// It uses a two-pass approach to find the tightest (shortest-span) match:
//  1. Forward pass: greedily match left-to-right to confirm all query chars exist in order.
//  2. Backward pass: from the end of the string, match query chars in reverse to find the
//     rightmost possible match.
//  3. Forward pass over that window to tighten and record exact matched indexes.
func match(str, query string, o option) (Match, bool) {
	normalizedStr := str
	if !o.caseSensitive {
		normalizedStr = strings.ToLower(str)
	}

	runes := []rune(normalizedStr)
	queryRunes := []rune(query)
	n := len(runes)
	qn := len(queryRunes)

	if qn == 0 {
		return Match{Str: str, MatchedIndexes: []int{}}, true
	}

	// Forward pass: confirm a match exists.
	qi := 0
	for i := 0; i < n && qi < qn; i++ {
		if runes[i] == queryRunes[qi] {
			qi++
		}
	}
	if qi < qn {
		return Match{}, false
	}

	// Backward pass: from the end of the string, match query chars in reverse.
	// This finds the rightmost end and the latest possible start.
	qi = qn - 1
	endIdx := -1
	startIdx := 0
	for i := n - 1; i >= 0 && qi >= 0; i-- {
		if runes[i] == queryRunes[qi] {
			if qi == qn-1 {
				endIdx = i
			}
			if qi == 0 {
				startIdx = i
			}
			qi--
		}
	}

	// Forward pass from startIdx to endIdx to tighten and collect matched indexes.
	matchedIndexes := make([]int, 0, qn)
	qi = 0
	for i := startIdx; i <= endIdx && qi < qn; i++ {
		if runes[i] == queryRunes[qi] {
			matchedIndexes = append(matchedIndexes, i)
			qi++
		}
	}

	return Match{Str: str, MatchedIndexes: matchedIndexes}, true
}
