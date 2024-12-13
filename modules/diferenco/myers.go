/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
// https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/myersDiffAlgorithm.ts

package diferenco

import "slices"

func MyersDiff[E comparable](seq1, seq2 []E) []Change {
	// These are common special cases.
	// The early return improves performance dramatically.
	if len(seq1) == 0 && len(seq2) == 0 {
		return []Change{}
	}
	if len(seq1) == 0 {
		return []Change{{Ins: len(seq2)}}
	}
	if len(seq2) == 0 {
		return []Change{{Del: len(seq1)}}
	}
	seqX := seq1
	seqY := seq2
	getXAfterSnake := func(x, y int) int {
		for x < len(seqX) && y < len(seqY) && seqX[x] == seqY[y] {
			y++
			x++
		}
		return x
	}
	d := 0
	// V[k]: X value of longest d-line that ends in diagonal k.
	// d-line: path from (0,0) to (x,y) that uses exactly d non-diagonals.
	// diagonal k: Set of points (x,y) with x-y = k.
	// k=1 -> (1,0),(2,1)
	V := NewFastIntArray()
	V.set(0, getXAfterSnake(0, 0))
	paths := &FastArrayNegativeIndices{
		positiveArr: make(map[int]*SnakePath),
		negativeArr: make(map[int]*SnakePath),
	}
	if V.get(0) == 0 {
		paths.set(0, nil)
	} else {
		paths.set(0, NewSnakePath(nil, 0, 0, V.get(0)))
	}
	k := 0
outer:
	for {
		d++
		// The paper has `for (k = -d; k <= d; k += 2)`, but we can ignore diagonals that cannot influence the result.
		lowerBound := -min(d, len(seqY)+(d%2))
		upperBound := min(d, len(seqX)+(d%2))
		for k = lowerBound; k <= upperBound; k += 2 {
			step := 0
			// We can use the X values of (d-1)-lines to compute X value of the longest d-lines.
			maxXofDLineTop, maxXofDLineLeft := -1, -1
			if k != upperBound {
				maxXofDLineTop = V.get(k + 1) // We take a vertical non-diagonal (add a symbol in seqX)
			}
			if k != lowerBound {
				maxXofDLineLeft = V.get(k-1) + 1 // We take a horizontal non-diagonal (+1 x) (delete a symbol in seqX)
			}
			step++
			x := min(max(maxXofDLineTop, maxXofDLineLeft), len(seqX))
			y := x - k
			step++
			if x > len(seqX) || y > len(seqY) {
				// This diagonal is irrelevant for the result.
				// TODO: Don't pay the cost for this in the next iteration.
				continue
			}
			newMaxX := getXAfterSnake(x, y)
			V.set(k, newMaxX)
			var lastPath *SnakePath
			if x == maxXofDLineTop {
				lastPath = paths.get(k + 1)
			} else {
				lastPath = paths.get(k - 1)
			}
			if newMaxX != x {
				paths.set(k, NewSnakePath(lastPath, x, y, newMaxX-x))
			} else {
				paths.set(k, lastPath)
			}
			if V.get(k) == len(seqX) && V.get(k)-k == len(seqY) {
				break outer
			}
		}
	}
	path := paths.get(k)
	lastAligningPosS1 := len(seqX)
	lastAligningPosS2 := len(seqY)
	changes := make([]Change, 0, 10)
	for {
		var endX, endY int
		if path != nil {
			endX = path.x + path.length
			endY = path.y + path.length
		}
		if endX != lastAligningPosS1 || endY != lastAligningPosS2 {
			changes = append(changes, Change{P1: endX, P2: endY, Del: lastAligningPosS1 - endX, Ins: lastAligningPosS2 - endY})
		}
		if path == nil {
			break
		}
		lastAligningPosS1 = path.x
		lastAligningPosS2 = path.y
		path = path.pre
	}
	slices.Reverse(changes)
	return changes
}

type SnakePath struct {
	pre          *SnakePath
	x, y, length int
}

func NewSnakePath(pre *SnakePath, x, y, length int) *SnakePath {
	return &SnakePath{
		pre:    pre,
		x:      x,
		y:      y,
		length: length,
	}
}

type FastIntArray struct {
	positiveArr []int
	negativeArr []int
}

func NewFastIntArray() *FastIntArray {
	return &FastIntArray{
		positiveArr: make([]int, 10),
		negativeArr: make([]int, 10),
	}
}

func (t *FastIntArray) get(i int) int {
	if i < 0 {
		i = -i - 1
		return t.negativeArr[i]
	}
	return t.positiveArr[i]
}

func (t *FastIntArray) set(i int, v int) {
	if i < 0 {
		i = -i - 1
		if i >= len(t.negativeArr) {
			newArr := make([]int, len(t.negativeArr)*2)
			copy(newArr, t.negativeArr)
			t.negativeArr = newArr
		}
		t.negativeArr[i] = v
		return
	}
	if i >= len(t.positiveArr) {
		newArr := make([]int, len(t.positiveArr)*2)
		copy(newArr, t.positiveArr)
		t.positiveArr = newArr
	}
	t.positiveArr[i] = v
}

// An array that supports fast negative indices.
type FastArrayNegativeIndices struct {
	positiveArr map[int]*SnakePath
	negativeArr map[int]*SnakePath
}

func (t *FastArrayNegativeIndices) get(i int) *SnakePath {
	if i < 0 {
		i = -i - 1
		return t.negativeArr[i]
	}
	return t.positiveArr[i]
}

func (t *FastArrayNegativeIndices) set(i int, v *SnakePath) {
	if i < 0 {
		i = -i - 1
		t.negativeArr[i] = v
		return
	}
	t.positiveArr[i] = v
}
