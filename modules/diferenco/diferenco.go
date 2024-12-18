package diferenco

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

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

type Algorithm int

const (
	Unspecified Algorithm = iota
	Histogram
	ONP
	Myers
	Minimal
	Patience
)

var (
	ErrUnsupportedAlgorithm = errors.New("unsupport algorithm")
)

var (
	algorithmValueMap = map[string]Algorithm{
		"histogram": Histogram,
		"onp":       ONP,
		"myers":     Myers,
		"patience":  Patience,
		"minimal":   Minimal,
	}
	algorithmNameMap = map[Algorithm]string{
		Unspecified: "unspecified",
		Histogram:   "histogram",
		ONP:         "onp",
		Myers:       "myers",
		Minimal:     "minimal",
		Patience:    "patience",
	}
)

func (a Algorithm) String() string {
	n, ok := algorithmNameMap[a]
	if ok {
		return n
	}
	return "unspecified"
}

func AlgorithmFromName(s string) (Algorithm, error) {
	if a, ok := algorithmValueMap[strings.ToLower(s)]; ok {
		return a, nil
	}
	return Unspecified, fmt.Errorf("unsupport algoritm '%s' %w", s, ErrUnsupportedAlgorithm)
}

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

type FileStat struct {
	Addition, Deletion, Hunks int
}

type Options struct {
	From, To *File
	S1, S2   string
	R1, R2   io.Reader
	A        Algorithm // algorithm
}

func diffInternal[E comparable](ctx context.Context, L1, L2 []E, a Algorithm) ([]Change, error) {
	if a == Unspecified {
		switch {
		case len(L1) < 5000 && len(L2) < 5000:
			a = Histogram
		default:
			a = ONP
		}
	}
	switch a {
	case Histogram:
		return HistogramDiff(ctx, L1, L2)
	case ONP:
		return OnpDiff(ctx, L1, L2)
	case Myers:
		return MyersDiff(ctx, L1, L2)
	case Minimal:
		return MinimalDiff(ctx, L1, L2)
	case Patience:
		return PatienceDiff(ctx, L1, L2)
	default:
		return nil, ErrUnsupportedAlgorithm
	}
}

func Stat(ctx context.Context, opts *Options) (*FileStat, error) {
	sink := &Sink{
		Index: make(map[string]int),
	}
	a, err := sink.parseLines(opts.R1, opts.S1)
	if err != nil {
		return nil, err
	}
	b, err := sink.parseLines(opts.R2, opts.S2)
	if err != nil {
		return nil, err
	}
	changes, err := diffInternal(ctx, a, b, opts.A)
	if err != nil {
		return nil, err
	}
	stats := &FileStat{
		Hunks: len(changes),
	}
	for _, ch := range changes {
		stats.Addition += ch.Ins
		stats.Deletion += ch.Del
	}
	return stats, nil
}
