package diffmatchpatch

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	// Sep1 signifies the start of a conflict.
	Sep1 = "<<<<<<<"
	// Sep2 signifies the middle of a conflict.
	Sep2 = "======="
	// Sep3 signifies the end of a conflict.
	Sep3 = ">>>>>>>"
)

type MergeRunes struct {
	Lines          LineMap
	StrIndexArrayO []rune
	StrIndexArrayA []rune
	StrIndexArrayB []rune
}

func (dmp *DiffMatchPatch) MergeLinesToRunes(textO, textA, textB string) *MergeRunes {
	lines := make(LineMap)
	lineHash := make(map[string]rune)
	strIndexArrayO := dmp.diffLinesToRunesMunge(textO, lines, lineHash)
	strIndexArrayA := dmp.diffLinesToRunesMunge(textA, lines, lineHash)
	strIndexArrayB := dmp.diffLinesToRunesMunge(textB, lines, lineHash)
	return &MergeRunes{Lines: lines, StrIndexArrayO: strIndexArrayO, StrIndexArrayA: strIndexArrayA, StrIndexArrayB: strIndexArrayB}
}

// Merge: Run a three-way text merge
//
//		Link: https://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf
//
//	 input:
//	 A = [1, 4, 5, 2, 3, 6]
//	 O = [1, 2, 3, 4, 5, 6]
//	 B = [1, 2, 4, 5, 3, 6]
//
//	 maximum matches:
//
//	 .---.---.---.---.---.---.---.---.---.
//	 | A | 1 | 4 | 5 | 2 | 3 |   |   | 6 |
//	 :---+---+---+---+---+---+---+---+---:
//	 | O | 1 |   |   | 2 | 3 | 4 | 5 | 6 |
//	 '---'---'---'---'---'---'---'---'---'
//
//	 .---.---.---.---.---.---.---.---.
//	 | A | 1 | 2 | 3 | 4 | 5 |   | 6 |
//	 :---+---+---+---+---+---+---+---:
//	 | O | 1 | 2 |   | 4 | 5 | 3 | 6 |
//	 '---'---'---'---'---'---'---'---'
//
//	 diff3 parse:
//	 .---.---.-----.---.-------.----.
//	 | A | 1 | 4,5 | 2 |     3 |  6 |
//	 :---+---+-----+---+-------+----:
//	 | O | 1 |     | 2 | 3,4,5 | ,5 |
//	 :---+---+-----+---+-------+----:
//	 | B | 1 |     | 2 | 4,5,3 |  6 |
//	 '---'---'-----'---'-------'----'
//
//	 calculated output:
//	 .----.---.------.---.-------.---.
//	 | A' | 1 |  4,5 | 2 |     3 | 6 |
//	 :----+---+------+---+-------+---:
//	 | O' | 1 |  4,5 | 2 | 3,4,5 | 6 |
//	 :----+---+------+---+-------+---:
//	 | B' | 1 |  4,5 | 2 | 4,5,3 | 6 |
//	 '----'---'------'---'-------'---'
//
//	 printed output:
//	 1
//	 4
//	 5
//	 2
//	 <<<<<<< A
//	 3
//	 ||||||| O
//	 3
//	 4
//	 5
//	 =======
//	 4
//	 5
//	 3
//	 >>>>>>> B
//	 6
func (dmp *DiffMatchPatch) Merge(textO, textA, textB string, labelA, labelB string) (string, bool, error) {
	m := dmp.MergeLinesToRunes(textO, textA, textB)
	diffsA := dmp.DiffMainRunes(m.StrIndexArrayO, m.StrIndexArrayA, false)
	diffsB := dmp.DiffMainRunes(m.StrIndexArrayO, m.StrIndexArrayB, false)
	p1 := diffsToPair(diffsA)
	p2 := diffsToPair(diffsB)
	chunks := diffsPairToChunks(p1, p2)
	var b strings.Builder
	b.Grow(max(len(textA), len(textB))) // grow: reduce
	mergeLine := func(i rune) {
		if line, ok := m.Lines[i]; ok {
			_, _ = b.WriteString(line)
		}
	}
	var conflict bool
	for _, chunk := range chunks {
		// stable chunk, add lines to new file
		if !chunk.conflict {
			for _, i := range chunk.o {
				mergeLine(i)
			}
			continue
		}
		// unstable chunk, add version A and version B to file
		conflict = true
		fmt.Fprintf(&b, "%s %s\n", Sep1, labelA)
		for _, i := range chunk.a {
			mergeLine(i)
		}
		fmt.Fprintf(&b, "%s\n", Sep2)
		for _, i := range chunk.b {
			mergeLine(i)
		}
		fmt.Fprintf(&b, "%s %s\n", Sep3, labelB)
	}
	return b.String(), conflict, nil
}

type diffsPair struct {
	unmodified []rune
	modified   []rune
}

// change diff type from diff trunk to aligned pair
// example:
// diffs: equal [1,2]; insert [3]; equal [4,5] delete [6], equal [7]
// pair: origin: [1,2,-1,4,5,6,7]; modified: [1,2,3,4,5,-1,7]
func diffsToPair(diffs []Diff) *diffsPair {
	p := &diffsPair{
		unmodified: make([]rune, 0, 100),
		modified:   make([]rune, 0, 100),
	}
	for _, d := range diffs {
		text := []rune(d.Text)
		switch d.Type {
		case DiffEqual:
			p.unmodified = append(p.unmodified, text...)
			p.modified = append(p.modified, text...)
		case DiffDelete:
			p.unmodified = append(p.unmodified, text...)
			for i := 0; i < len(text); i++ {
				p.modified = append(p.modified, -1)
			}
		case DiffInsert:
			p.modified = append(p.modified, text...)
			for i := 0; i < len(text); i++ {
				p.unmodified = append(p.unmodified, -1)
			}
		}
	}
	return p
}

type mergeChunk struct {
	o, a, b          []rune
	stable, conflict bool
}

func newMergeChunk() *mergeChunk {
	return &mergeChunk{
		o: make([]rune, 0, 32),
		a: make([]rune, 0, 32),
		b: make([]rune, 0, 32),
	}
}

func diffsPairToChunks(a, b *diffsPair) []*mergeChunk {
	// indexA and indexB represent the current positions already traversed in A and B, respectively
	// nextA and nextB represent the next position that a.o and b.o not empty(-1)
	indexA := -1
	indexB := -1
	nextA := -1
	nextB := -1
	lenA := len(a.unmodified)
	lenB := len(b.unmodified)
	chunks := make([]*mergeChunk, 0)
	chunk := newMergeChunk()
	for indexA < lenA && indexB < lenB {
		// update na
		for i := indexA + 1; i < lenA; i++ {
			if a.unmodified[i] != -1 {
				nextA = i
				break
			}
		}
		// update nb
		for i := indexB + 1; i < lenB; i++ {
			if b.unmodified[i] != -1 {
				nextB = i
				break
			}
		}

		// that means the last common part has been traversed
		if nextA == indexA {
			// nothing left in pairs
			if lenA == indexA+1 && lenB == indexB+1 {
				// add chunk left
				if len(chunk.o) != 0 {
					chunks = append(chunks, chunk)
				}
				break
			}
			// left stuff must be unstable
			if chunk.stable && len(chunk.o) != 0 {
				chunks = append(chunks, chunk)
				chunk = newMergeChunk()
				chunk.stable = false
			}
			// unstable chunk, only append
			chunk.a = append(chunk.a, a.modified[indexA+1:lenA]...)
			chunk.b = append(chunk.b, b.modified[indexB+1:lenB]...)
			chunks = append(chunks, chunk)
			break
		}

		// chunk is empty, init chunk
		if len(chunk.o) == 0 {
			// next index is adjacent, only delete possible, no insert.
			// so only judge if o, a, b is equal
			if nextA-indexA == 1 && nextB-indexB == 1 {
				chunk.o = append(chunk.o, a.unmodified[nextA])
				if a.modified[nextA] != -1 {
					chunk.a = append(chunk.a, a.unmodified[nextA])
				}
				if b.modified[nextB] != -1 {
					chunk.b = append(chunk.b, b.unmodified[nextB])
				}
				// determine whether this chunk is a stable chunk or a unstable chunk
				chunk.stable = a.modified[nextA] != -1 && b.modified[nextB] != -1
				indexA = nextA
				indexB = nextB
				continue
			}
			// na or nb not adjacent, so it should be a unstable chunk
			chunk.stable = false
			// first add insert part(index between ia-na & ib-nb) to unstable chunk
			chunk.a = append(chunk.a, a.modified[indexA+1:nextA]...)
			chunk.b = append(chunk.b, b.modified[indexB+1:nextB]...)
			// if next origin is unstable, add it to chunk and finish
			if a.modified[nextA] == -1 || b.modified[nextB] == -1 {
				chunk.o = append(chunk.o, a.unmodified[nextA])
				if a.modified[nextA] != -1 {
					chunk.a = append(chunk.a, a.unmodified[nextA])
				}
				if b.modified[nextB] != -1 {
					chunk.b = append(chunk.b, b.unmodified[nextB])
				}
				indexA = nextA
				indexB = nextB
				continue
			}
			// next origin is stable, that means o = a = b
			// curernt unstable chunk should be closed, and a new chunk should be created
			chunks = append(chunks, chunk)
			chunk = newMergeChunk()
			chunk.o = append(chunk.o, a.unmodified[nextA])
			chunk.a = append(chunk.a, a.unmodified[nextA])
			chunk.b = append(chunk.b, b.unmodified[nextB])
			chunk.stable = true
			indexA = nextA
			indexB = nextB
			continue
		}

		// chunk is not empty, determine increase chunk or close & create chunk
		// next index is adjacent, only delete possible, no insert.
		// so only judge if o, a, b is equal
		if nextA-indexA == 1 && nextB-indexB == 1 {
			// o = a = b, stable index
			if a.modified[nextA] != -1 && b.modified[nextB] != -1 {
				// unstable chunk, close and create a new stable chunk
				if !chunk.stable {
					chunks = append(chunks, chunk)
					chunk = newMergeChunk()
					chunk.stable = true
				}
				// stable chunk, only append
				chunk.o = append(chunk.o, a.unmodified[nextA])
				chunk.a = append(chunk.a, a.unmodified[nextA])
				chunk.b = append(chunk.b, b.unmodified[nextB])
				indexA = nextA
				indexB = nextB
				continue
			}
			// unstable index
			// stable chunk, close and create a new unstable chunk
			if chunk.stable {
				chunks = append(chunks, chunk)
				chunk = newMergeChunk()
				chunk.stable = false
			}
			// unstable chunk, only append
			chunk.o = append(chunk.o, a.unmodified[nextA])
			if a.modified[nextA] != -1 {
				chunk.a = append(chunk.a, a.unmodified[nextA])
			}
			if b.modified[nextB] != -1 {
				chunk.b = append(chunk.b, b.unmodified[nextB])
			}
			indexA = nextA
			indexB = nextB
			continue
		}

		// na or nb not adjacent, so it should be a unstable chunk
		// stable chunk, close and create a new unstable chunk
		if chunk.stable {
			chunks = append(chunks, chunk)
			chunk = newMergeChunk()
			chunk.stable = false
		}
		// first add insert part(index between ia-na & ib-nb) to unstable chunk
		chunk.a = append(chunk.a, a.modified[indexA+1:nextA]...)
		chunk.b = append(chunk.b, b.modified[indexB+1:nextB]...)
		// if next origin is unstable, add it to chunk and finish
		if a.modified[nextA] == -1 || b.modified[nextB] == -1 {
			chunk.o = append(chunk.o, a.unmodified[nextA])
			if a.modified[nextA] != -1 {
				chunk.a = append(chunk.a, a.unmodified[nextA])
			}
			if b.modified[nextB] != -1 {
				chunk.b = append(chunk.b, b.unmodified[nextB])
			}
			indexA = nextA
			indexB = nextB
			continue
		}
		// next origin is stable, that means o = a = b
		// curernt unstable chunk should be closed, and a new chunk should be created
		chunks = append(chunks, chunk)
		chunk = newMergeChunk()
		chunk.o = append(chunk.o, a.unmodified[nextA])
		chunk.a = append(chunk.a, a.unmodified[nextA])
		chunk.b = append(chunk.b, b.unmodified[nextB])
		chunk.stable = true
		indexA = nextA
		indexB = nextB
		continue
	}

	for _, chunk := range chunks {
		chunk.propagate()
	}
	return chunks
}

// examine what has changed in each chunk
// decide what changes can be propagated
// if A & B both different from O and A and B not equal themself, there will be conflict
func (c *mergeChunk) propagate() {
	// stable trunk doesn't need propagate
	if c.stable {
		return
	}
	switch {
	case slices.Equal(c.a, c.b):
		c.o = c.a
	case slices.Equal(c.o, c.a):
		c.o = c.b
		c.a = c.b
	case slices.Equal(c.o, c.b):
		c.o = c.a
		c.b = c.a
	default:
		c.conflict = true
	}
}

// Merge: Built-in text merging implementation.
// TODO: ignore CRLF --> LF ???
func Merge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error) {
	e := New()
	e.DiffTimeout = time.Hour
	return e.Merge(o, a, b, labelA, labelB)
}
