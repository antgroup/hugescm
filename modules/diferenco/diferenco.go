package diferenco

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	return Unspecified, fmt.Errorf("unsupported algorithm '%s' %w", s, ErrUnsupportedAlgorithm)
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

type Change struct {
	P1  int // before: position in before
	P2  int // after: position in after
	Del int // number of elements that deleted from a
	Ins int // number of elements that inserted into b
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

func diffInternal[E comparable](ctx context.Context, L1, L2 []E, algo Algorithm) ([]Change, error) {
	switch algo {
	case Unspecified:
		if len(L1) < 5000 && len(L2) < 5000 {
			return HistogramDiff(ctx, L1, L2)
		}
		return OnpDiff(ctx, L1, L2)
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

func DiffRunes(ctx context.Context, a, b string, algo Algorithm) ([]StringDiff, error) {
	runesA := []rune(a)
	runesB := []rune(b)
	changes, err := diffInternal(ctx, runesA, runesB, algo)
	if err != nil {
		return nil, err
	}
	diffs := make([]StringDiff, 0, 10)
	i := 0
	for _, c := range changes {
		if i < c.P1 {
			diffs = append(diffs, StringDiff{Type: Equal, Text: string(runesA[i:c.P1])})
		}
		if c.Del != 0 {
			diffs = append(diffs, StringDiff{Type: Delete, Text: string(runesA[c.P1 : c.P1+c.Del])})
		}
		if c.Ins != 0 {
			diffs = append(diffs, StringDiff{Type: Insert, Text: string(runesB[c.P2 : c.P2+c.Ins])})
		}
		i = c.P1 + c.Del
	}
	if i < len(runesA) {
		diffs = append(diffs, StringDiff{Type: Equal, Text: string(runesA[i:])})
	}
	return diffs, nil
}

func DiffWords(ctx context.Context, a, b string, algo Algorithm, splitFunc func(string) []string) ([]StringDiff, error) {
	if splitFunc == nil {
		splitFunc = SplitWords
	}
	wordsA := splitFunc(a)
	wordsB := splitFunc(b)
	changes, err := diffInternal(ctx, wordsA, wordsB, algo)
	if err != nil {
		return nil, err
	}
	diffs := make([]StringDiff, 0, 10)
	i := 0
	for _, c := range changes {
		if i < c.P1 {
			diffs = append(diffs, StringDiff{Type: Equal, Text: strings.Join(wordsA[i:c.P1], "")})
		}
		if c.Del != 0 {
			diffs = append(diffs, StringDiff{Type: Delete, Text: strings.Join(wordsA[c.P1:c.P1+c.Del], "")})
		}
		if c.Ins != 0 {
			diffs = append(diffs, StringDiff{Type: Insert, Text: strings.Join(wordsB[c.P2:c.P2+c.Ins], "")})
		}
		i = c.P1 + c.Del
	}
	if i < len(wordsA) {
		diffs = append(diffs, StringDiff{Type: Equal, Text: strings.Join(wordsA[i:], "")})
	}
	return diffs, nil
}
