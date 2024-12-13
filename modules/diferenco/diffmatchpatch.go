// Copyright (c) 2012-2016 The go-diff authors. All rights reserved.
// https://github.com/sergi/go-diff
// See the included LICENSE file for license details.
//
// go-diff is a Go implementation of Google's Diff, Match, and Patch library
// Original library is Copyright (c) 2006 Google Inc.
// http://code.google.com/p/google-diff-match-patch/
package diferenco

import (
	"context"
)

func diffSlices[E comparable](ctx context.Context, s1, s2 []E) ([]Dfio[E], error) {
	// Trim off common prefix (speedup).
	commonlength := commonPrefixLength(s1, s2)
	commonprefix := s1[:commonlength]
	s1 = s1[commonlength:]
	s2 = s2[commonlength:]

	// Trim off common suffix (speedup).
	commonlength = commonSuffixLength(s1, s2)
	commonsuffix := s1[len(s1)-commonlength:]
	s1 = s1[:len(s1)-commonlength]
	s2 = s2[:len(s2)-commonlength]
	// Compute the diff on the middle block.
	diffs, err := diffCompute(ctx, s1, s2)
	if err != nil {
		return nil, err
	}

	// Restore the prefix and suffix.
	if len(commonprefix) != 0 {
		diffs = append([]Dfio[E]{{E: commonprefix, T: Equal}}, diffs...)
	}
	if len(commonsuffix) != 0 {
		diffs = append(diffs, Dfio[E]{E: commonsuffix, T: Equal})
	}
	return diffCleanupMerge(diffs), nil
}

func diffHalfMatchI[E comparable](l, s []E, i int) [][]E {
	var bestCommon []E
	var bestCommonLen int
	var bestLongtextA []E
	var bestLongtextB []E
	var bestShorttextA []E
	var bestShorttextB []E

	// Start with a 1/4 length substring at position i as a seed.
	seed := l[i : i+len(l)/4]

	for j := slicesIndexOf(s, seed, 0); j != -1; j = slicesIndexOf(s, seed, j+1) {
		prefixLength := commonPrefixLength(l[i:], s[j:])
		suffixLength := commonSuffixLength(l[:i], s[:j])

		if bestCommonLen < suffixLength+prefixLength {
			bestCommon = s[j-suffixLength : j+prefixLength]
			bestCommonLen = len(bestCommon)
			bestLongtextA = l[:i-suffixLength]
			bestLongtextB = l[i+prefixLength:]
			bestShorttextA = s[:j-suffixLength]
			bestShorttextB = s[j+prefixLength:]
		}
	}

	if bestCommonLen*2 < len(l) {
		return nil
	}

	return [][]E{
		bestLongtextA,
		bestLongtextB,
		bestShorttextA,
		bestShorttextB,
		bestCommon,
	}
}

func diffHalfMatch[E comparable](ctx context.Context, s1, s2 []E) [][]E {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	var longtext, shorttext []E
	if len(s1) > len(s2) {
		longtext = s1
		shorttext = s2
	} else {
		longtext = s2
		shorttext = s1
	}

	if len(longtext) < 4 || len(shorttext)*2 < len(longtext) {
		return nil // Pointless.
	}

	// First check if the second quarter is the seed for a half-match.
	hm1 := diffHalfMatchI(longtext, shorttext, int(float64(len(longtext)+3)/4))

	// Check again based on the third quarter.
	hm2 := diffHalfMatchI(longtext, shorttext, int(float64(len(longtext)+1)/2))

	var hm [][]E
	switch {
	case hm1 == nil && hm2 == nil:
		return nil
	case hm2 == nil:
		hm = hm1
	case hm1 == nil:
		hm = hm2
	default:
		// Both matched.  Select the longest.
		if len(hm1[4]) > len(hm2[4]) {
			hm = hm1
		} else {
			hm = hm2
		}
	}

	// A half-match was found, sort out the return data.
	if len(s1) > len(s2) {
		return hm
	}

	return [][]E{hm[2], hm[3], hm[0], hm[1], hm[4]}
}

func diffBisectSplit[E comparable](ctx context.Context, s1, s2 []E, x, y int) ([]Dfio[E], error) {
	s1a := s1[:x]
	s2a := s2[:y]
	s1b := s1[x:]
	s2b := s2[y:]

	// Compute both diffs serially.
	diffs, err := diffSlices(ctx, s1a, s2a)
	if err != nil {
		return nil, err
	}
	diffsb, err := diffSlices(ctx, s1b, s2b)
	if err != nil {
		return nil, err
	}

	return append(diffs, diffsb...), nil
}

// diffBisect finds the 'middle snake' of a diff, splits the problem in two and returns the recursively constructed diff.
// See Myers's 1986 paper: An O(ND) Difference Algorithm and Its Variations.
func diffBisect[E comparable](ctx context.Context, s1, s2 []E) ([]Dfio[E], error) {
	// Cache the text lengths to prevent multiple calls.
	s1Len, s2Len := len(s1), len(s2)

	maxD := (s1Len + s2Len + 1) / 2
	vOffset := maxD
	vLength := 2 * maxD

	v1 := make([]int, vLength)
	v2 := make([]int, vLength)
	for i := range v1 {
		v1[i] = -1
		v2[i] = -1
	}
	v1[vOffset+1] = 0
	v2[vOffset+1] = 0

	delta := s1Len - s2Len
	// If the total number of characters is odd, then the front path will collide with the reverse path.
	front := (delta%2 != 0)
	// Offsets for start and end of k loop. Prevents mapping of space beyond the grid.
	k1start := 0
	k1end := 0
	k2start := 0
	k2end := 0
	for d := 0; d < maxD; d++ {
		// Bail out if deadline is reached.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Walk the front path one step.
		for k1 := -d + k1start; k1 <= d-k1end; k1 += 2 {
			k1Offset := vOffset + k1
			var x1 int

			if k1 == -d || (k1 != d && v1[k1Offset-1] < v1[k1Offset+1]) {
				x1 = v1[k1Offset+1]
			} else {
				x1 = v1[k1Offset-1] + 1
			}

			y1 := x1 - k1
			for x1 < s1Len && y1 < s2Len {
				if s1[x1] != s2[y1] {
					break
				}
				x1++
				y1++
			}
			v1[k1Offset] = x1
			if x1 > s1Len {
				// Ran off the right of the graph.
				k1end += 2
			} else if y1 > s2Len {
				// Ran off the bottom of the graph.
				k1start += 2
			} else if front {
				k2Offset := vOffset + delta - k1
				if k2Offset >= 0 && k2Offset < vLength && v2[k2Offset] != -1 {
					// Mirror x2 onto top-left coordinate system.
					x2 := s1Len - v2[k2Offset]
					if x1 >= x2 {
						// Overlap detected.
						return diffBisectSplit(ctx, s1, s2, x1, y1)
					}
				}
			}
		}
		// Walk the reverse path one step.
		for k2 := -d + k2start; k2 <= d-k2end; k2 += 2 {
			k2Offset := vOffset + k2
			var x2 int
			if k2 == -d || (k2 != d && v2[k2Offset-1] < v2[k2Offset+1]) {
				x2 = v2[k2Offset+1]
			} else {
				x2 = v2[k2Offset-1] + 1
			}
			var y2 = x2 - k2
			for x2 < s1Len && y2 < s2Len {
				if s1[s1Len-x2-1] != s2[s2Len-y2-1] {
					break
				}
				x2++
				y2++
			}
			v2[k2Offset] = x2
			if x2 > s1Len {
				// Ran off the left of the graph.
				k2end += 2
			} else if y2 > s2Len {
				// Ran off the top of the graph.
				k2start += 2
			} else if !front {
				k1Offset := vOffset + delta - k2
				if k1Offset >= 0 && k1Offset < vLength && v1[k1Offset] != -1 {
					x1 := v1[k1Offset]
					y1 := vOffset + x1 - k1Offset
					// Mirror x2 onto top-left coordinate system.
					x2 = s1Len - x2
					if x1 >= x2 {
						// Overlap detected.
						return diffBisectSplit(ctx, s1, s2, x1, y1)
					}
				}
			}
		}
	}
	// Diff took too long and hit the deadline or number of diffs equals number of characters, no commonality at all.
	return []Dfio[E]{
		{T: Delete, E: s1},
		{T: Insert, E: s2},
	}, nil
}

func diffCompute[E comparable](ctx context.Context, s1, s2 []E) ([]Dfio[E], error) {
	diffs := []Dfio[E]{}
	if len(s1) == 0 {
		// Just add some text (speedup).
		return append(diffs, Dfio[E]{T: Insert, E: s2}), nil
	}
	if len(s2) == 0 {
		// Just delete some text (speedup).
		return append(diffs, Dfio[E]{T: Delete, E: s1}), nil
	}

	var longSlices, shortSlices []E
	if len(s1) > len(s2) {
		longSlices = s1
		shortSlices = s2
	} else {
		longSlices = s2
		shortSlices = s1
	}

	if i := slicesIndex(longSlices, shortSlices); i != -1 {
		op := Insert
		// Swap insertions for deletions if diff is reversed.
		if len(s1) > len(s2) {
			op = Delete
		}
		// Shorter text is inside the longer text (speedup).
		return []Dfio[E]{
			{T: op, E: longSlices[:i]},
			{T: Equal, E: shortSlices},
			{T: op, E: longSlices[i+len(shortSlices):]},
		}, nil
	}
	if len(shortSlices) == 1 {
		// Single character string.
		// After the previous speedup, the character can't be an equality.
		return []Dfio[E]{
			{T: Delete, E: s1},
			{T: Insert, E: s2},
		}, nil

	}
	// Check to see if the problem can be split in two.
	if hm := diffHalfMatch(ctx, s1, s2); hm != nil {
		// A half-match was found, sort out the return data.
		s1A := hm[0]
		s1B := hm[1]
		s2A := hm[2]
		s2B := hm[3]
		midCommon := hm[4]
		// Send both pairs off for separate processing.
		diffsA, err := diffSlices(ctx, s1A, s2A)
		if err != nil {
			return nil, err
		}
		diffsB, err := diffSlices(ctx, s1B, s2B)
		if err != nil {
			return nil, err
		}
		// Merge the results.
		diffs := diffsA
		diffs = append(diffs, Dfio[E]{T: Equal, E: midCommon})
		diffs = append(diffs, diffsB...)
		return diffs, nil
	}
	return diffBisect(ctx, s1, s2)
}

// splice removes amount elements from slice at index index, replacing them with elements.
func splice[E comparable](slice []Dfio[E], index int, amount int, elements ...Dfio[E]) []Dfio[E] {
	if len(elements) == amount {
		// Easy case: overwrite the relevant items.
		copy(slice[index:], elements)
		return slice
	}
	if len(elements) < amount {
		// Fewer new items than old.
		// Copy in the new items.
		copy(slice[index:], elements)
		// Shift the remaining items left.
		copy(slice[index+len(elements):], slice[index+amount:])
		// Calculate the new end of the slice.
		end := len(slice) - amount + len(elements)
		// Zero stranded elements at end so that they can be garbage collected.
		tail := slice[end:]
		for i := range tail {
			tail[i] = Dfio[E]{}
		}
		return slice[:end]
	}
	// More new items than old.
	// Make room in slice for new elements.
	// There's probably an even more efficient way to do this,
	// but this is simple and clear.
	need := len(slice) - amount + len(elements)
	for len(slice) < need {
		slice = append(slice, Dfio[E]{})
	}
	// Shift slice elements right to make room for new elements.
	copy(slice[index+len(elements):], slice[index+amount:])
	// Copy in new elements.
	copy(slice[index:], elements)
	return slice
}

// diffCleanupMerge reorders and merges like edit sections. Merge equalities.
// Any edit section can move as long as it doesn't cross an equality.
func diffCleanupMerge[E comparable](diffs []Dfio[E]) []Dfio[E] {
	// Add a dummy entry at the end.
	diffs = append(diffs, Dfio[E]{T: Equal, E: []E{}})
	pointer := 0
	countDelete := 0
	countInsert := 0
	commonlength := 0
	textDelete := []E(nil)
	textInsert := []E(nil)

	for pointer < len(diffs) {
		switch diffs[pointer].T {
		case Insert:
			countInsert++
			textInsert = append(textInsert, diffs[pointer].E...)
			pointer++
		case Delete:
			countDelete++
			textDelete = append(textDelete, diffs[pointer].E...)
			pointer++
		case Equal:
			// Upon reaching an equality, check for prior redundancies.
			if countDelete+countInsert > 1 {
				if countDelete != 0 && countInsert != 0 {
					// Factor out any common prefixies.
					commonlength = commonPrefixLength(textInsert, textDelete)
					if commonlength != 0 {
						x := pointer - countDelete - countInsert
						if x > 0 && diffs[x-1].T == Equal {
							diffs[x-1].E = append(diffs[x-1].E, textInsert[:commonlength]...)
						} else {
							diffs = append([]Dfio[E]{{T: Equal, E: textInsert[:commonlength]}}, diffs...)
							pointer++
						}
						textInsert = textInsert[commonlength:]
						textDelete = textDelete[commonlength:]
					}
					// Factor out any common suffixies.
					commonlength = commonSuffixLength(textInsert, textDelete)
					if commonlength != 0 {
						insertIndex := len(textInsert) - commonlength
						deleteIndex := len(textDelete) - commonlength
						e := diffs[pointer].E
						diffs[pointer].E = textInsert[insertIndex:]
						diffs[pointer].E = append(diffs[pointer].E, e...)
						textInsert = textInsert[:insertIndex]
						textDelete = textDelete[:deleteIndex]
					}
				}
				// Delete the offending records and add the merged ones.
				if countDelete == 0 {
					diffs = splice(diffs, pointer-countInsert,
						countDelete+countInsert,
						Dfio[E]{T: Insert, E: textInsert})
				} else if countInsert == 0 {
					diffs = splice(diffs, pointer-countDelete,
						countDelete+countInsert,
						Dfio[E]{T: Delete, E: textDelete})
				} else {
					diffs = splice(diffs, pointer-countDelete-countInsert,
						countDelete+countInsert,
						Dfio[E]{T: Delete, E: textDelete},
						Dfio[E]{T: Insert, E: textInsert})
				}

				pointer = pointer - countDelete - countInsert + 1
				if countDelete != 0 {
					pointer++
				}
				if countInsert != 0 {
					pointer++
				}
			} else if pointer != 0 && diffs[pointer-1].T == Equal {
				// Merge this equality with the previous one.
				diffs[pointer-1].E = append(diffs[pointer-1].E, diffs[pointer].E...)
				diffs = append(diffs[:pointer], diffs[pointer+1:]...)
			} else {
				pointer++
			}
			countInsert = 0
			countDelete = 0
			textDelete = nil
			textInsert = nil
		}
	}

	if len(diffs[len(diffs)-1].E) == 0 {
		diffs = diffs[0 : len(diffs)-1] // Remove the dummy entry at the end.
	}

	// Second pass: look for single edits surrounded on both sides by equalities which can be shifted sideways to eliminate an equality. E.g: A<ins>BA</ins>C -> <ins>AB</ins>AC
	changes := false
	pointer = 1
	// Intentionally ignore the first and last element (don't need checking).
	for pointer < (len(diffs) - 1) {
		if diffs[pointer-1].T == Equal &&
			diffs[pointer+1].T == Equal {
			// This is a single edit surrounded by equalities.

			if slicesHasSuffix(diffs[pointer].E, diffs[pointer-1].E) {
				// Shift the edit over the previous equality.
				// diffs[pointer].Text = diffs[pointer-1].Text +
				// 	diffs[pointer].Text[:len(diffs[pointer].Text)-len(diffs[pointer-1].Text)]
				E := diffs[pointer].E
				diffs[pointer].E = diffs[pointer-1].E
				diffs[pointer].E = append(diffs[pointer].E, E[:len(E)-len(diffs[pointer-1].E)]...)
				// diffs[pointer+1].Text = diffs[pointer-1].Text + diffs[pointer+1].Text
				PE := diffs[pointer+1].E
				diffs[pointer+1].E = diffs[pointer-1].E
				diffs[pointer+1].E = append(diffs[pointer+1].E, PE...)

				diffs = splice(diffs, pointer-1, 1)
				changes = true
			} else if slicesHasPrefix(diffs[pointer].E, diffs[pointer+1].E) {
				// Shift the edit over the next equality.
				// diffs[pointer-1].Text += diffs[pointer+1].Text
				diffs[pointer-1].E = append(diffs[pointer-1].E, diffs[pointer+1].E...)
				// diffs[pointer].Text =
				// 	diffs[pointer].Text[len(diffs[pointer+1].Text):] + diffs[pointer+1].Text
				diffs[pointer].E = diffs[pointer].E[len(diffs[pointer+1].E):]
				diffs[pointer].E = append(diffs[pointer].E, diffs[pointer+1].E...)

				diffs = splice(diffs, pointer+1, 1)
				changes = true
			}
		}
		pointer++
	}

	// If shifts were made, the diff needs reordering and another shift sweep.
	if changes {
		diffs = diffCleanupMerge(diffs)
	}

	return diffs
}

func DiffSlices[E comparable](ctx context.Context, S1, S2 []E) ([]Dfio[E], error) {
	return diffSlices(ctx, S1, S2)
}
