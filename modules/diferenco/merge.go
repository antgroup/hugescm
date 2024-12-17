/*
Copyright (c) 2024 epic labs
Package diff3 implements a three-way merge algorithm
Original version in Javascript by Bryan Housel @bhousel: https://github.com/bhousel/node-diff3,
which in turn is based on project Synchrotron, created by Tony Garnock-Jones. For more detail please visit:
http://homepages.kcbbs.gen.nz/tonyg/projects/synchrotron.html
https://github.com/tonyg/synchrotron

Ported to go by Javier Peletier @jpeletier

SOURCE: https://github.com/epiclabs-io/diff3

SPDX-License-Identifier: MIT
*/
package diferenco

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// https://blog.jcoglan.com/2017/05/08/merging-with-diff3/

// Alice               Original            Bob
//
// 1. celery           1. celery           1. celery
// 2. salmon           2. garlic           2. salmon
// 3. tomatoes         3. onions           3. garlic
// 4. garlic           4. salmon           4. onions
// 5. onions           5. tomatoes         5. tomatoes
// 6. wine             6. wine             6. wine

// Alice               Original            Bob
//
// 1. celery           1. celery           1. celery         A
// -----------------------------------------------------------
// 					2. garlic           2. salmon         B
// 2. salmon           3. onions           3. garlic
// 					4. salmon           4. onions
// -----------------------------------------------------------
// 3. tomatoes         5. tomatoes         5. tomatoes       C
// -----------------------------------------------------------
// 4. garlic                                                 D
// 5. onions
// -----------------------------------------------------------
// 6. wine             6. wine             6. wine           E

// celery
// <<<<<<< Alice
// salmon
// =======
// salmon
// garlic
// onions
// >>>>>>> Bob
// tomatoes
// garlic
// onions
// wine

const (
	// Sep1 signifies the start of a conflict.
	Sep1 = "<<<<<<<"
	// Sep2 signifies the middle of a conflict.
	Sep2 = "======="
	// Sep3 signifies the end of a conflict.
	Sep3 = ">>>>>>>"
	// SepO origin content
	SepO = "|||||||"
)

type hunk [5]int
type hunkList []*hunk

func (h hunkList) Len() int           { return len(h) }
func (h hunkList) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h hunkList) Less(i, j int) bool { return h[i][0] < h[j][0] }

// Given three files, A, O, and B, where both A and B are
// independently derived from O, returns a fairly complicated
// internal representation of merge decisions it's taken. The
// interested reader may wish to consult
//
// Sanjeev Khanna, Keshav Kunal, and Benjamin C. Pierce.
// 'A Formal Investigation of ' In Arvind and Prasad,
// editors, Foundations of Software Technology and Theoretical
// Computer Science (FSTTCS), December 2007.
//
// (http://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf)
func diff3MergeIndices[E comparable](ctx context.Context, o, a, b []E, algo Algorithm) ([][]int, error) {
	m1, err := diffInternal(ctx, o, a, algo)
	if err != nil {
		return nil, err
	}
	m2, err := diffInternal(ctx, o, b, algo)
	if err != nil {
		return nil, err
	}
	var hunks []*hunk
	addHunk := func(h Change, side int) {
		hunks = append(hunks, &hunk{h.P1, side, h.Del, h.P2, h.Ins})
	}
	for i := 0; i < len(m1); i++ {
		addHunk(m1[i], 0)
	}
	for i := 0; i < len(m2); i++ {
		addHunk(m2[i], 2)
	}
	sort.Sort(hunkList(hunks))

	var result [][]int
	var commonOffset = 0
	copyCommon := func(targetOffset int) {
		if targetOffset > commonOffset {
			result = append(result, []int{1, commonOffset, targetOffset - commonOffset})
			commonOffset = targetOffset
		}
	}

	for hunkIndex := 0; hunkIndex < len(hunks); hunkIndex++ {
		firstHunkIndex := hunkIndex
		hunk := hunks[hunkIndex]
		regionLhs := hunk[0]
		regionRhs := regionLhs + hunk[2]
		for hunkIndex < len(hunks)-1 {
			maybeOverlapping := hunks[hunkIndex+1]
			maybeLhs := maybeOverlapping[0]
			if maybeLhs > regionRhs {
				break
			}
			regionRhs = max(regionRhs, maybeLhs+maybeOverlapping[2])
			hunkIndex++
		}

		copyCommon(regionLhs)
		if firstHunkIndex == hunkIndex {
			// The 'overlap' was only one hunk long, meaning that
			// there's no conflict here. Either a and o were the
			// same, or b and o were the same.
			if hunk[4] > 0 {
				result = append(result, []int{hunk[1], hunk[3], hunk[4]})
			}
		} else {
			// A proper conflict. Determine the extents of the
			// regions involved from a, o and b. Effectively merge
			// all the hunks on the left into one giant hunk, and
			// do the same for the right; then, correct for skew
			// in the regions of o that each side changed, and
			// report appropriate spans for the three sides.
			regions := [][]int{{len(a), -1, len(o), -1}, nil, {len(b), -1, len(o), -1}}
			for i := firstHunkIndex; i <= hunkIndex; i++ {
				hunk = hunks[i]
				side := hunk[1]
				r := regions[side]
				oLhs := hunk[0]
				oRhs := oLhs + hunk[2]
				abLhs := hunk[3]
				abRhs := abLhs + hunk[4]
				r[0] = min(abLhs, r[0])
				r[1] = max(abRhs, r[1])
				r[2] = min(oLhs, r[2])
				r[3] = max(oRhs, r[3])
			}
			aLhs := regions[0][0] + (regionLhs - regions[0][2])
			aRhs := regions[0][1] + (regionRhs - regions[0][3])
			bLhs := regions[2][0] + (regionLhs - regions[2][2])
			bRhs := regions[2][1] + (regionRhs - regions[2][3])
			result = append(result, []int{-1,
				aLhs, aRhs - aLhs,
				regionLhs, regionRhs - regionLhs,
				bLhs, bRhs - bLhs})
		}
		commonOffset = regionRhs
	}

	copyCommon(len(o))
	return result, nil
}

// Conflict describes a merge conflict
type Conflict[E comparable] struct {
	a      []E
	aIndex int
	o      []E
	oIndex int
	b      []E
	bIndex int
}

// Diff3MergeResult describes a merge result
type Diff3MergeResult[E comparable] struct {
	ok       []E
	conflict *Conflict[E]
}

// Diff3Merge applies the output of diff3MergeIndices to actually
// construct the merged file; the returned result alternates
// between 'ok' and 'conflict' blocks.
func Diff3Merge[E comparable](ctx context.Context, o, a, b []E, algo Algorithm, excludeFalseConflicts bool) ([]*Diff3MergeResult[E], error) {
	var result []*Diff3MergeResult[E]
	files := [][]E{a, o, b}
	indices, err := diff3MergeIndices(ctx, o, a, b, algo)
	if err != nil {
		return nil, err
	}

	var okLines []E
	flushOk := func() {
		if len(okLines) != 0 {
			result = append(result, &Diff3MergeResult[E]{ok: okLines})
		}
		okLines = nil
	}

	pushOk := func(xs []E) {
		for j := 0; j < len(xs); j++ {
			okLines = append(okLines, xs[j])
		}
	}

	isTrueConflict := func(rec []int) bool {
		if rec[2] != rec[6] {
			return true
		}
		var aoff = rec[1]
		var boff = rec[5]
		for j := 0; j < rec[2]; j++ {
			if a[j+aoff] != b[j+boff] {
				return true
			}
		}
		return false
	}

	for i := 0; i < len(indices); i++ {
		var x = indices[i]
		var side = x[0]
		if side == -1 {
			if excludeFalseConflicts && !isTrueConflict(x) {
				pushOk(files[0][x[1] : x[1]+x[2]])
			} else {
				flushOk()
				result = append(result, &Diff3MergeResult[E]{
					conflict: &Conflict[E]{
						a:      a[x[1] : x[1]+x[2]],
						aIndex: x[1],
						o:      o[x[3] : x[3]+x[4]],
						oIndex: x[3],
						b:      b[x[5] : x[5]+x[6]],
						bIndex: x[5],
					},
				})
			}
		} else {
			pushOk(files[side][x[1] : x[1]+x[2]])
		}
	}

	flushOk()
	return result, nil
}

const (
	// Only show the zealously minified conflicting lines of the local changes and the incoming (other) changes,
	// hiding the base version entirely.
	//
	// ```text
	// line1-changed-by-both
	// <<<<<<< local
	// line2-to-be-changed-in-incoming
	// =======
	// line2-changed
	// >>>>>>> incoming
	// ```
	STYLE_DEFAULT = iota
	// Show non-minimized hunks of local changes, the base, and the incoming (other) changes.
	//
	// This mode does not hide any information.
	//
	// ```text
	// <<<<<<< local
	// line1-changed-by-both
	// line2-to-be-changed-in-incoming
	// ||||||| 9a8d80c
	// line1-to-be-changed-by-both
	// line2-to-be-changed-in-incoming
	// =======
	// line1-changed-by-both
	// line2-changed
	// >>>>>>> incoming
	// ```
	STYLE_DIFF3
	// Like diff3, but will show *minimized* hunks of local change and the incoming (other) changes,
	// as well as non-minimized hunks of the base.
	//
	// ```text
	// line1-changed-by-both
	// <<<<<<< local
	// line2-to-be-changed-in-incoming
	// ||||||| 9a8d80c
	// line1-to-be-changed-by-both
	// line2-to-be-changed-in-incoming
	// =======
	// line2-changed
	// >>>>>>> incoming
	// ```
	STYLE_ZEALOUS_DIFF3
)

var (
	styles = map[string]int{
		"merge":  STYLE_DEFAULT,
		"diff3":  STYLE_DIFF3,
		"zdiff3": STYLE_ZEALOUS_DIFF3,
	}
)

func ParseConflictStyle(s string) int {
	if s, ok := styles[strings.ToLower(s)]; ok {
		return s
	}
	return STYLE_DEFAULT
}

type MergeOptions struct {
	TextO, TextA, TextB    string
	LabelO, LabelA, LabelB string
	A                      Algorithm
	Style                  int // Conflict Style
}

func (opts *MergeOptions) ValidateOptions() error {
	if opts == nil {
		return errors.New("invalid merge options")
	}
	if opts.A == Unspecified {
		opts.A = Histogram
	}
	if len(opts.LabelO) != 0 {
		opts.LabelO = " " + opts.LabelO
	}
	if len(opts.LabelA) != 0 {
		opts.LabelA = " " + opts.LabelA
	}
	if len(opts.LabelB) != 0 {
		opts.LabelB = " " + opts.LabelB
	}
	return nil
}

func (s *Sink) writeConflict(out io.Writer, opts *MergeOptions, conflict *Conflict[int]) {
	if opts.Style == STYLE_DIFF3 {
		fmt.Fprintf(out, "%s%s\n", Sep1, opts.LabelA)
		s.WriteLine(out, conflict.a...)
		fmt.Fprintf(out, "%s%s\n", SepO, opts.LabelO)
		s.WriteLine(out, conflict.o...)
		fmt.Fprintf(out, "%s\n", Sep2)
		s.WriteLine(out, conflict.b...)
		fmt.Fprintf(out, "%s%s\n", Sep3, opts.LabelB)
		return
	}
	a, b := conflict.a, conflict.b
	prefix := commonPrefixLength(a, b)
	s.WriteLine(out, a[:prefix]...)
	a = a[prefix:]
	b = b[prefix:]
	suffix := commonSuffixLength(a, b)
	fmt.Fprintf(out, "%s%s\n", Sep1, opts.LabelA)
	s.WriteLine(out, a[:len(a)-suffix]...)

	if opts.Style == STYLE_ZEALOUS_DIFF3 {
		// Zealous Diff3
		fmt.Fprintf(out, "%s%s\n", SepO, opts.LabelO)
		s.WriteLine(out, conflict.o...)
	}

	fmt.Fprintf(out, "%s\n", Sep2)
	s.WriteLine(out, b[:len(b)-suffix]...)
	fmt.Fprintf(out, "%s%s\n", Sep3, opts.LabelB)
	if suffix != 0 {
		s.WriteLine(out, b[suffix:]...)
	}
}

// Merge implements the diff3 algorithm to merge two texts into a common base.
//
//	Support multiple diff algorithms and multiple conflict styles
func Merge(ctx context.Context, opts *MergeOptions) (string, bool, error) {
	if err := opts.ValidateOptions(); err != nil {
		return "", false, err
	}
	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	default:
	}
	s := NewSink(NEWLINE_RAW)
	slicesO := s.SplitLines(opts.TextO)
	slicesA := s.SplitLines(opts.TextA)
	slicesB := s.SplitLines(opts.TextB)
	regions, err := Diff3Merge(ctx, slicesO, slicesA, slicesB, opts.A, true)
	if err != nil {
		return "", false, err
	}
	out := &strings.Builder{}
	out.Grow(max(len(opts.TextO), len(opts.TextA), len(opts.TextB)))
	var conflicts = false
	for _, r := range regions {
		if r.ok != nil {
			s.WriteLine(out, r.ok...)
			continue
		}
		if r.conflict != nil {
			conflicts = true
			s.writeConflict(out, opts, r.conflict)
		}
	}
	return out.String(), conflicts, nil
}

// DefaultMerge implements the diff3 algorithm to merge two texts into a common base.
func DefaultMerge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error) {
	return Merge(ctx, &MergeOptions{TextO: o, TextA: a, TextB: b, LabelO: labelO, LabelA: labelA, LabelB: labelB, A: Histogram})
}
