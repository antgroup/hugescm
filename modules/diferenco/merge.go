package diferenco

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	// Sep1 signifies the start of a conflict.
	Sep1 = "<<<<<<<"
	// Sep2 signifies the middle of a conflict.
	Sep2 = "======="
	// Sep3 signifies the end of a conflict.
	Sep3 = ">>>>>>>"
)

// Merge implements the diff3 algorithm to merge two texts into a common base.
func Merge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error) {
	sink := NewSink(NEWLINE_RAW)
	slicesO := sink.ParseLines(o)
	slicesA := sink.ParseLines(a)
	slicesB := sink.ParseLines(b)
	// diffsA := PatienceDiff(slicesO, slicesA)
	// diffsB := PatienceDiff(slicesO, slicesB)
	diffsA, err := DiffSlices(context.Background(), slicesO, slicesA)
	if err != nil {
		return "", false, err
	}
	diffsB, err := DiffSlices(context.Background(), slicesO, slicesB)
	if err != nil {
		return "", false, err
	}
	p1 := diffsToPair(diffsA)
	p2 := diffsToPair(diffsB)
	chunks := diffsPairToChunks(p1, p2)
	out := &strings.Builder{}
	out.Grow(max(len(o), len(a), len(b))) // grow: reduce
	var conflict bool
	for _, chunk := range chunks {
		// stable chunk, add lines to new file
		if !chunk.conflict {
			sink.WriteLine(out, chunk.o...)
			continue
		}
		// unstable chunk, add version A and version B to file
		conflict = true
		fmt.Fprintf(out, "%s %s\n", Sep1, labelA)
		sink.WriteLine(out, chunk.a...)
		fmt.Fprintf(out, "%s\n", Sep2)
		sink.WriteLine(out, chunk.b...)
		fmt.Fprintf(out, "%s %s\n", Sep3, labelB)
	}
	return out.String(), conflict, nil
}

type diffsPair struct {
	unmodified []int
	modified   []int
}

// change diff type from diff trunk to aligned pair
// example:
// diffs: equal [1,2]; insert [3]; equal [4,5] delete [6], equal [7]
// pair: origin: [1,2,-1,4,5,6,7]; modified: [1,2,3,4,5,-1,7]
func diffsToPair(diffs []Dfio[int]) *diffsPair {
	p := &diffsPair{
		unmodified: make([]int, 0, 100),
		modified:   make([]int, 0, 100),
	}
	for _, d := range diffs {
		switch d.T {
		case Equal:
			p.unmodified = append(p.unmodified, d.E...)
			p.modified = append(p.modified, d.E...)
		case Delete:
			p.unmodified = append(p.unmodified, d.E...)
			for i := 0; i < len(d.E); i++ {
				p.modified = append(p.modified, -1)
			}
		case Insert:
			p.modified = append(p.modified, d.E...)
			for i := 0; i < len(d.E); i++ {
				p.unmodified = append(p.unmodified, -1)
			}
		}
	}
	return p
}

type mergeChunk struct {
	o, a, b          []int
	stable, conflict bool
}

func newMergeChunk() *mergeChunk {
	return &mergeChunk{
		o: make([]int, 0, 32),
		a: make([]int, 0, 32),
		b: make([]int, 0, 32),
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
