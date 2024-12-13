package diferenco

import "slices"

// https://github.com/Wilfred/difftastic/wiki/Line-Based-Diffs
// https://neil.fraser.name/writing/diff/
// https://prettydiff.com/2/guide/unrelated_diff.xhtml
// https://blog.robertelder.org/diff-algorithm/
// https://news.ycombinator.com/item?id=33417466

// Operation defines the operation of a diff item.
type Operation int8

const (
	// Delete item represents a delete hunk.
	Delete Operation = -1
	// Insert item represents an insert hunk.
	Insert Operation = 1
	// Equal item represents an equal hunk.
	Equal Operation = 0
)

// commonPrefixLength returns the length of the common prefix of two T slices.
func commonPrefixLength[E comparable](a, b []E) int {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

// commonSuffixLength returns the length of the common suffix of two rune slices.
func commonSuffixLength[E comparable](a, b []E) int {
	i1, i2 := len(a), len(b)
	n := min(i1, i2)
	i := 0
	for i < n && a[i1-1-i] == b[i2-1-i] {
		i++
	}
	return i
}

func slicesHasSuffix[E comparable](a, suffix []E) bool {
	return len(a) >= len(suffix) && slices.Equal(a[len(a)-len(suffix):], suffix)
}

func slicesHasPrefix[E comparable](a, prefix []E) bool {
	return len(a) >= len(prefix) && slices.Equal(a[:len(prefix)], prefix)
}

// slicesIndex is the equivalent of strings.Index for rune slices.
func slicesIndex[E comparable](s1, s2 []E) int {
	last := len(s1) - len(s2)
	for i := 0; i <= last; i++ {
		if slices.Equal(s1[i:i+len(s2)], s2) {
			return i
		}
	}
	return -1
}

// slicesIndexOf returns the index of pattern in target, starting at target[i].
func slicesIndexOf[E comparable](target, pattern []E, i int) int {
	if i > len(target)-1 {
		return -1
	}
	if i <= 0 {
		return slicesIndex(target, pattern)
	}
	ind := slicesIndex(target[i:], pattern)
	if ind == -1 {
		return -1
	}
	return ind + i
}

type Change struct {
	P1  int // before: position in before
	P2  int // after: position in after
	Del int // number of elements that deleted from a
	Ins int // number of elements that inserted into b
}

type Dfio[E comparable] struct {
	T Operation
	E []E
}

// StringDiff represents one diff operation
type StringDiff struct {
	Type Operation
	Text string
}
