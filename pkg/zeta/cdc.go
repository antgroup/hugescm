// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"io"
	"math/bits"
)

// Span is a half-open [Offset, Offset+Size) byte range in a file.
//
// It is the common currency for fragment boundaries across the fixed-size and
// content-defined chunking paths.
type Span struct {
	Offset int64
	Size   int64
}

// Chunker implements the FastCDC content-defined chunking algorithm.
//
// Reference: Xia et al., "FastCDC: A Fast and Efficient Content-Defined
// Chunking Approach for Data Deduplication", USENIX ATC 2016.
// https://www.usenix.org/node/196197
//
// Three-phase normalized chunking:
//
//	minSize <= size < normalSize                : maskShort  (more bits → harder to cut, keeps small chunks growing)
//	normalSize <= size < normalSize+normalSpan  : maskNormal (target probability)
//	size >= normalSize+normalSpan && size<maxSize: maskLong  (fewer bits → easier to cut, closes off large chunks)
//	size >= maxSize                              : force cut
//
// Note: previous revisions of this code had the maskShort / maskLong
// probabilities swapped relative to the paper, which biased average chunk
// size well below the configured target. PR-2 fixes the direction.
type Chunker struct {
	targetSize int64
	minSize    int64
	maxSize    int64
	normalSize int64 // boundary between short and normal phase
	normalSpan int64 // length of the normal phase

	maskShort  uint64 // applied while size ∈ [minSize, normalSize)
	maskNormal uint64 // applied while size ∈ [normalSize, normalSize+normalSpan)
	maskLong   uint64 // applied while size ∈ [normalSize+normalSpan, maxSize)
}

// NewChunker creates a FastCDC chunker tuned around the given target size.
//
// Default normalization:
//
//	minSize = max(target/4, 64KiB)
//	maxSize = min(target*8, 64MiB)
//	normal  = target
func NewChunker(targetSize int64) *Chunker {
	minSize := max(targetSize/4, 64<<10)
	maxSize := min(targetSize*8, 64<<20)

	// maskBits = log2(target) - 1, clamped to [10, 24] to avoid degenerate
	// values when the target size is unusual. The +1 / -2 offsets below give
	// the normalization a chance to dominate without ever shifting beyond 63.
	maskBits := min(max(bits.Len64(uint64(targetSize))-1, 10), 24)

	return &Chunker{
		targetSize: targetSize,
		minSize:    minSize,
		maxSize:    maxSize,
		normalSize: targetSize,
		normalSpan: targetSize,
		// Short phase: more bits → harder to cut → keeps short chunks growing
		// toward normalSize.
		maskShort: uint64(1)<<(maskBits+1) - 1,
		// Normal phase: target cut probability.
		maskNormal: uint64(1)<<maskBits - 1,
		// Long phase: fewer bits → easier to cut → closes off oversized
		// chunks before hitting maxSize.
		maskLong: uint64(1)<<(maskBits-2) - 1,
	}
}

// Split scans the reader and returns chunk boundaries without retaining data.
//
// Use this when you only need the boundaries (e.g. for analysis or tests).
// For production chunking with hashing, prefer Walk.
//
// Tail handling: if the final chunk would be shorter than minSize, it is
// merged into the previous chunk to avoid pathologically small metadata.
func (c *Chunker) Split(reader io.Reader) ([]Span, error) {
	spans := make([]Span, 0)

	buf := make([]byte, 32<<10)
	hash := uint64(0)
	chunkStart := int64(0)
	bytesRead := int64(0)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for i := range n {
				hash = (hash << 1) + gearTable[buf[i]]
				bytesRead++

				size := bytesRead - chunkStart
				if size < c.minSize {
					continue
				}

				if c.shouldCut(size, hash) {
					spans = append(spans, Span{Offset: chunkStart, Size: size})
					chunkStart = bytesRead
					hash = 0
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if bytesRead > chunkStart {
		tailSize := bytesRead - chunkStart
		if tailSize < c.minSize && len(spans) > 0 && spans[len(spans)-1].Size+tailSize <= c.maxSize {
			spans[len(spans)-1].Size += tailSize
		} else {
			spans = append(spans, Span{Offset: chunkStart, Size: tailSize})
		}
	}
	return spans, nil
}

// Walk scans the reader and invokes onChunk for each chunk boundary,
// passing a reader over the chunk's data.
//
// IMPORTANT: this is not zero-buffer streaming. CDC needs to read bytes to
// compute the rolling hash before it can know where to cut, so the chunker
// buffers up to maxSize bytes between boundaries. This matches the standard
// CDC trade-off used by restic, borg, and similar tools. Memory usage is
// O(maxSize), independent of input size.
//
// CONTRACT: onChunk must fully consume the supplied reader before returning;
// the underlying buffer is reused for the next chunk.
//
// Tail handling: if the final chunk would be shorter than minSize, it is
// merged into the previous chunk by buffering across the previous emit. The
// implementation realizes this by deferring emit until either the buffer
// reaches a real boundary or the stream ends.
func (c *Chunker) Walk(reader io.Reader, onChunk func(span Span, data io.Reader) error) error {
	buf := make([]byte, 32<<10)
	hash := uint64(0)
	chunkStart := int64(0)
	bytesRead := int64(0)

	chunkBuf := make([]byte, 0, c.maxSize)

	// We hold back one emitted chunk so that a too-small tail can be merged
	// into it. Once a new chunk is finalized, the previously held one becomes
	// safe to emit.
	var (
		heldSpan Span
		heldData []byte
		hasHeld  bool
	)

	flushHeld := func() error {
		if !hasHeld {
			return nil
		}
		err := onChunk(heldSpan, bytes.NewReader(heldData))
		hasHeld = false
		heldData = nil
		return err
	}

	emit := func(span Span, data []byte) error {
		// Emit the previously held chunk first (if any) and then hold the new
		// one in case the next event is a short tail that we want to merge in.
		if err := flushHeld(); err != nil {
			return err
		}
		// Copy data because the rolling buffer will be reused.
		held := make([]byte, len(data))
		copy(held, data)
		heldSpan = span
		heldData = held
		hasHeld = true
		return nil
	}

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for i := range n {
				hash = (hash << 1) + gearTable[buf[i]]
				bytesRead++
				chunkBuf = append(chunkBuf, buf[i])

				size := bytesRead - chunkStart
				if size < c.minSize {
					continue
				}

				if c.shouldCut(size, hash) {
					if cbErr := emit(Span{Offset: chunkStart, Size: size}, chunkBuf); cbErr != nil {
						return cbErr
					}
					chunkStart = bytesRead
					hash = 0
					chunkBuf = chunkBuf[:0]
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if bytesRead > chunkStart {
		tailSize := bytesRead - chunkStart
		// Merge the tail into the held chunk if it would otherwise be too
		// small. Falls back to emitting separately if no held chunk exists or
		// the merge would exceed maxSize.
		if hasHeld && tailSize < c.minSize && heldSpan.Size+tailSize <= c.maxSize {
			heldSpan.Size += tailSize
			heldData = append(heldData, chunkBuf...)
			return flushHeld()
		}
		if err := flushHeld(); err != nil {
			return err
		}
		return onChunk(Span{Offset: chunkStart, Size: tailSize}, bytes.NewReader(chunkBuf))
	}
	return flushHeld()
}

// shouldCut applies the FastCDC three-phase normalization rule.
func (c *Chunker) shouldCut(size int64, hash uint64) bool {
	switch {
	case size < c.normalSize:
		return hash&c.maskShort == 0
	case size < c.normalSize+c.normalSpan:
		return hash&c.maskNormal == 0
	case size < c.maxSize:
		return hash&c.maskLong == 0
	default:
		return true
	}
}
