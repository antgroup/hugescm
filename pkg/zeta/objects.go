// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type chunk struct {
	offset int64 // chunk offset
	size   int64 // chunk size
}

func calculateChunk(size, partSize int64) []chunk {
	N := int(size / partSize)
	chunks := make([]chunk, 0, N)
	if N == 0 {
		return []chunk{{offset: 0, size: size}}
	}
	var offset int64
	for i := 0; i < N-1; i++ {
		chunks = append(chunks, chunk{offset: offset, size: partSize})
		offset += partSize
	}
	if size-offset > partSize {
		if float64(size-offset)/float64(partSize) > 1.5 {
			chunks = append(chunks, chunk{offset: offset, size: partSize})
			offset += partSize
		} else {
			curSize := partSize / 2
			chunks = append(chunks, chunk{offset: offset, size: curSize})
			offset += curSize
		}
	}
	chunks = append(chunks, chunk{offset: offset, size: size - offset})
	return chunks
}

func (r *Repository) HashTo(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
	if size < r.Fragment.Threshold() {
		oid, err = r.odb.HashTo(ctx, io.LimitReader(reader, size), size)
		return
	}

	// Use CDC (Content-Defined Chunking) for AI model files
	if r.Fragment.EnableCDC.True() {
		return r.hashToWithCDC(ctx, reader, size)
	}

	// Original fixed-size chunking logic
	h := plumbing.NewHasher()
	tr := io.TeeReader(reader, h)
	chunks := calculateChunk(size, r.Fragment.Size())
	ff := &object.Fragments{
		Size:    uint64(size),
		Entries: make([]*object.Fragment, len(chunks)),
	}
	for i, k := range chunks {
		var o plumbing.Hash
		if o, err = r.odb.HashTo(ctx, io.LimitReader(tr, k.size), k.size); err != nil {
			return
		}
		ff.Entries[i] = &object.Fragment{
			Index: uint32(i),
			Hash:  o,
			Size:  uint64(k.size),
		}
	}
	ff.Origin = h.Sum() // Sum raw file hash
	oid, err = r.odb.WriteEncoded(ff)
	fragments = true
	return
}

// hashToWithCDC uses CDC (Content-Defined Chunking) for large files
// Optimized: single-pass streaming with no temporary file I/O
func (r *Repository) hashToWithCDC(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
	// Streaming CDC implementation:
	// 1. Compute full file hash while chunking
	// 2. Use FastCDC for all formats (works well for both structured and unstructured data)
	// 3. Hash each chunk on-the-fly and build Fragments object
	// 4. Avoid materializing entire chunks in memory

	h := plumbing.NewHasher()
	teeReader := io.TeeReader(reader, h)

	cdcChunker := NewCDCChunker(r.Fragment.Size())

	ff := &object.Fragments{
		Size:    uint64(size),
		Entries: make([]*object.Fragment, 0),
	}

	chunkIndex := uint32(0)

	// Use streaming callback - avoid materializing entire chunks!
	err = cdcChunker.ChunkStreaming(teeReader, size, func(offset, chunkSize int64, chunkReader io.Reader) error {
		// Stream the chunk directly to hash computation
		// CRITICAL: chunkReader is a streaming reader, not a materialized byte slice
		chunkHash, hashErr := r.odb.HashTo(ctx, chunkReader, chunkSize)
		if hashErr != nil {
			return hashErr
		}

		ff.Entries = append(ff.Entries, &object.Fragment{
			Index: chunkIndex,
			Hash:  chunkHash,
			Size:  uint64(chunkSize),
		})
		chunkIndex++
		return nil
	})

	if err != nil {
		return plumbing.ZeroHash, false, err
	}

	ff.Origin = h.Sum()
	oid, err = r.odb.WriteEncoded(ff)
	fragments = true
	return
}

func (r *Repository) WriteEncoded(e object.Encoder) (oid plumbing.Hash, err error) {
	return r.odb.WriteEncoded(e)
}
