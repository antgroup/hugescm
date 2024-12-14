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
	"fmt"
	"slices"
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
)

type candidate struct {
	file1index int
	file2index int
	chain      *candidate
}

// Text diff algorithm following Hunt and McIlroy 1976.
// J. W. Hunt and M. D. McIlroy, An algorithm for differential file
// comparison, Bell Telephone Laboratories CSTR #41 (1976)
// http://www.cs.dartmouth.edu/~doug/
func d3Lcs[E comparable](file1, file2 []E) *candidate {
	var equivalenceClasses map[E][]int
	var file2indices []int

	var candidates []*candidate
	var line E
	var c *candidate
	var i, j, jX, r, s int

	equivalenceClasses = make(map[E][]int)
	for j = 0; j < len(file2); j++ {
		line = file2[j]
		equivalenceClasses[line] = append(equivalenceClasses[line], j)
	}

	candidates = append(candidates, &candidate{file1index: -1, file2index: -1, chain: nil})

	for i = 0; i < len(file1); i++ {
		line = file1[i]
		file2indices = equivalenceClasses[line] // || []

		r = 0
		c = candidates[0]

		for jX = 0; jX < len(file2indices); jX++ {
			j = file2indices[jX]

			for s = r; s < len(candidates); s++ {
				if (candidates[s].file2index < j) && ((s == len(candidates)-1) || (candidates[s+1].file2index > j)) {
					break
				}
			}

			if s < len(candidates) {
				newCandidate := &candidate{file1index: i, file2index: j, chain: candidates[s]}
				if r == len(candidates) {
					candidates = append(candidates, c)
				} else {
					candidates[r] = c
				}
				r = s + 1
				c = newCandidate
				if r == len(candidates) {
					break // no point in examining further (j)s
				}
			}
		}

		if r == len(candidates) {
			candidates = append(candidates, c)
		} else {
			if r > len(candidates) {
				panic("out of range")
			} else {
				candidates[r] = c
			}
		}
	}

	// At this point, we know the LCS: it's in the reverse of the
	// linked-list through .chain of candidates[candidates.length - 1].

	return candidates[len(candidates)-1]
}

type diffIndicesResult struct {
	file1 []int
	file2 []int
}

// We apply the LCS to give a simple representation of the
// offsets and lengths of mismatched chunks in the input
// files. This is used by diff3MergeIndices below.
func diffIndices[E comparable](file1, file2 []E) []*diffIndicesResult {
	var result []*diffIndicesResult
	tail1 := len(file1)
	tail2 := len(file2)

	for candidate := d3Lcs(file1, file2); candidate != nil; candidate = candidate.chain {
		mismatchLength1 := tail1 - candidate.file1index - 1
		mismatchLength2 := tail2 - candidate.file2index - 1
		tail1 = candidate.file1index
		tail2 = candidate.file2index

		if mismatchLength1 != 0 || mismatchLength2 != 0 {
			result = append(result, &diffIndicesResult{
				file1: []int{tail1 + 1, mismatchLength1},
				file2: []int{tail2 + 1, mismatchLength2},
			})
		}
	}

	slices.Reverse(result)
	return result
}

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
func diff3MergeIndices[E comparable](a, o, b []E) [][]int {
	m1 := diffIndices(o, a)
	m2 := diffIndices(o, b)

	var hunks []*hunk
	addHunk := func(h *diffIndicesResult, side int) {
		hunks = append(hunks, &hunk{h.file1[0], side, h.file1[1], h.file2[0], h.file2[1]})
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
	return result
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
func Diff3Merge[E comparable](a, o, b []E, excludeFalseConflicts bool) []*Diff3MergeResult[E] {
	var result []*Diff3MergeResult[E]
	files := [][]E{a, o, b}
	indices := diff3MergeIndices(a, o, b)

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
	return result
}

// Merge implements the diff3 algorithm to merge two texts into a common base.
func Merge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error) {
	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	default:
	}
	if len(labelA) != 0 {
		labelA = " " + labelA
	}
	if len(labelB) != 0 {
		labelB = " " + labelB
	}
	sink := NewSink(NEWLINE_RAW)
	slicesO := sink.SplitLines(o)
	slicesA := sink.SplitLines(a)
	slicesB := sink.SplitLines(b)
	regions := Diff3Merge(slicesA, slicesO, slicesB, true)
	out := &strings.Builder{}
	out.Grow(max(len(o), len(a), len(b)))
	var conflicts = false
	for _, r := range regions {
		if r.ok != nil {
			sink.WriteLine(out, r.ok...)
			continue
		}
		if r.conflict != nil {
			conflicts = true
			fmt.Fprintf(out, "%s%s\n", Sep1, labelA)
			sink.WriteLine(out, r.conflict.a...)
			fmt.Fprintf(out, "%s\n", Sep2)
			sink.WriteLine(out, r.conflict.b...)
			fmt.Fprintf(out, "%s%s\n", Sep3, labelB)
		}
	}
	return out.String(), conflicts, nil
}
