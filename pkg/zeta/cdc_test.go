// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
)

// randomBytes returns a deterministic pseudo-random buffer. Using a fixed seed
// keeps the chunker output stable across runs while still exercising enough
// entropy for the Gear hash to produce realistic boundaries.
func randomBytes(seed int64, n int) []byte {
	rng := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	_, _ = rng.Read(b)
	return b
}

func TestChunkerSplitCoversAllBytes(t *testing.T) {
	// 40MB of pseudo-random data ensures the Gear hash sees enough entropy to
	// actually trigger boundary detection multiple times.
	data := randomBytes(1, 40<<20)

	chunker := NewChunker(4 << 20) // 4MB target
	spans, err := chunker.Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	var total int64
	for _, s := range spans {
		total += s.Size
	}
	if total != int64(len(data)) {
		t.Errorf("spans total %d != data size %d", total, len(data))
	}

	// PR-2 invariant: every span (including the tail, which is merged into
	// its predecessor when too small) is within [minSize, maxSize].
	for i, s := range spans {
		if s.Size < 1<<20 {
			t.Errorf("span %d size %d below minSize 1MB", i, s.Size)
		}
		if s.Size > 32<<20 {
			t.Errorf("span %d size %d above maxSize 32MB", i, s.Size)
		}
	}

	t.Logf("File size: %d bytes, spans: %d, avg=%d", len(data), len(spans), total/int64(len(spans)))
	for i, s := range spans {
		if i < 5 || i >= len(spans)-2 {
			t.Logf("  span %d: offset=%d size=%d", i, s.Offset, s.Size)
		}
	}
}

// TestChunkerWalkMatchesSplit guards that Walk and Split agree on boundaries
// for the same input, and that Walk hands out exactly span.Size bytes per
// chunk. This is a refactor safety net for the PR-1 rename (no behavior
// changes vs. the previous CDCChunker implementation).
func TestChunkerWalkMatchesSplit(t *testing.T) {
	data := make([]byte, 8<<20)
	for i := range data {
		data[i] = byte((i * 31) % 256)
	}

	chunker := NewChunker(2 << 20)

	expected, err := chunker.Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	var got []Span
	err = chunker.Walk(bytes.NewReader(data), func(span Span, r io.Reader) error {
		buf, readErr := io.ReadAll(r)
		if readErr != nil {
			return readErr
		}
		if int64(len(buf)) != span.Size {
			t.Errorf("walk span at offset %d: read %d bytes, want %d", span.Offset, len(buf), span.Size)
		}
		got = append(got, span)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("Walk produced %d spans, Split produced %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("span %d mismatch: walk=%+v split=%+v", i, got[i], expected[i])
		}
	}
}

// TestChunkerDeterministic guards that the chunker is a pure function of its
// input: the same bytes always produce the same boundaries.
func TestChunkerDeterministic(t *testing.T) {
	data := randomBytes(7, 16<<20)
	chunker := NewChunker(2 << 20)

	first, err := chunker.Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("first Split: %v", err)
	}
	second, err := chunker.Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("second Split: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("non-deterministic span count: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("span %d differs across runs: %+v vs %+v", i, first[i], second[i])
		}
	}
}

// TestChunkerSingleByteInsertionShiftsLocally is the core CDC property:
// inserting one byte should only invalidate the boundaries near the insertion
// point. The vast majority of chunks should still match between the two
// versions of the file.
func TestChunkerSingleByteInsertionShiftsLocally(t *testing.T) {
	original := randomBytes(42, 32<<20)
	// Insert one byte roughly in the middle.
	mid := len(original) / 2
	modified := make([]byte, 0, len(original)+1)
	modified = append(modified, original[:mid]...)
	modified = append(modified, 0xAB)
	modified = append(modified, original[mid:]...)

	chunker := NewChunker(2 << 20)
	a, err := chunker.Split(bytes.NewReader(original))
	if err != nil {
		t.Fatalf("Split original: %v", err)
	}
	b, err := chunker.Split(bytes.NewReader(modified))
	if err != nil {
		t.Fatalf("Split modified: %v", err)
	}

	// Compare by size+content hash, not by offset (offsets shift after the
	// insertion). We compare span "content fingerprints" via length+first/last
	// byte for speed; collisions are extremely unlikely with random data.
	type sig struct {
		size int64
		head byte
		tail byte
	}
	sigOf := func(buf []byte, span Span) sig {
		return sig{size: span.Size, head: buf[span.Offset], tail: buf[span.Offset+span.Size-1]}
	}

	aSet := make(map[sig]int)
	for _, s := range a {
		aSet[sigOf(original, s)]++
	}

	common := 0
	for _, s := range b {
		k := sigOf(modified, s)
		if aSet[k] > 0 {
			aSet[k]--
			common++
		}
	}

	// We expect at least (len(a)-3) chunks to survive: at worst the chunk
	// containing the insertion plus the two adjacent chunks are affected.
	minCommon := len(a) - 3
	if common < minCommon {
		t.Errorf("CDC failed locality property: %d/%d chunks survived a 1-byte insertion (want >= %d)", common, len(a), minCommon)
	}
	t.Logf("original spans=%d modified spans=%d common=%d", len(a), len(b), common)
}

// TestChunkerAverageSizeNearTarget checks that PR-2's mask direction fix
// brings the average chunk size into a plausible band around the configured
// target. Without the fix, average sizes were strongly biased toward minSize.
func TestChunkerAverageSizeNearTarget(t *testing.T) {
	const target = 4 << 20
	data := randomBytes(99, 128<<20) // 128MB

	chunker := NewChunker(target)
	spans, err := chunker.Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(spans) == 0 {
		t.Fatal("no spans produced")
	}

	avg := int64(len(data)) / int64(len(spans))
	// Wide acceptance band: [target/2, target*3]. The fix is asymmetric and
	// the precise value depends on the Gear table; this test is a sanity
	// floor, not a tight bound.
	low := int64(target / 2)
	high := int64(target * 3)
	if avg < low || avg > high {
		t.Errorf("average chunk size %d outside expected band [%d, %d]", avg, low, high)
	}
	t.Logf("128MB random data, target=%dMB, spans=%d, avg=%d (%.2fx target)", target>>20, len(spans), avg, float64(avg)/float64(target))
}

// TestCalculateChunkSpansAreContiguous pins down the invariants that
// writeFixedFragments relies on: spans are contiguous, cover the entire input,
// no span exceeds 2x partSize, and exact-multiple inputs split into uniform
// partSize chunks. If any of these drifts the on-disk Fragments layout — and
// therefore historical fragment hashes — would silently change.
func TestCalculateChunkSpansAreContiguous(t *testing.T) {
	cases := []struct {
		name     string
		size     int64
		partSize int64
	}{
		{"exact multiple", 16 << 20, 4 << 20},
		{"with small tail", 16<<20 + 1<<20, 4 << 20},
		{"with big tail", 16<<20 + 7<<20, 4 << 20},
		{"smaller than part", 3 << 20, 4 << 20},
		{"one part exactly", 4 << 20, 4 << 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := calculateChunk(tc.size, tc.partSize)
			if len(spans) == 0 {
				t.Fatalf("no spans produced")
			}

			var offset, total int64
			for i, s := range spans {
				if s.Offset != offset {
					t.Fatalf("span %d: offset=%d, want %d (gap or overlap)", i, s.Offset, offset)
				}
				if s.Size <= 0 {
					t.Fatalf("span %d: non-positive size %d", i, s.Size)
				}
				if s.Size > tc.partSize*2 {
					t.Fatalf("span %d: size %d exceeds 2x partSize %d", i, s.Size, tc.partSize)
				}
				offset += s.Size
				total += s.Size
			}
			if total != tc.size {
				t.Fatalf("spans cover %d bytes, want %d", total, tc.size)
			}

			// For exact multiples, every chunk must equal partSize. This is the
			// stricter invariant that historical fragment hashes depend on.
			if tc.size%tc.partSize == 0 {
				want := int(tc.size / tc.partSize)
				if len(spans) != want {
					t.Fatalf("exact multiple: got %d spans, want %d", len(spans), want)
				}
				for i, s := range spans {
					if s.Size != tc.partSize {
						t.Fatalf("exact multiple: span %d size %d != partSize %d", i, s.Size, tc.partSize)
					}
				}
			}
		})
	}
}
