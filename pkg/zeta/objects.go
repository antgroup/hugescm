// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// calculateChunk computes fixed-size fragment spans for a file of the given
// size, using partSize as the nominal chunk size.
func calculateChunk(size, partSize int64) []Span {
	N := int(size / partSize)
	spans := make([]Span, 0, N)
	if N == 0 {
		return []Span{{Offset: 0, Size: size}}
	}
	var offset int64
	for i := 0; i < N-1; i++ {
		spans = append(spans, Span{Offset: offset, Size: partSize})
		offset += partSize
	}
	if size-offset > partSize {
		if float64(size-offset)/float64(partSize) > 1.5 {
			spans = append(spans, Span{Offset: offset, Size: partSize})
			offset += partSize
		} else {
			curSize := partSize / 2
			spans = append(spans, Span{Offset: offset, Size: curSize})
			offset += curSize
		}
	}
	spans = append(spans, Span{Offset: offset, Size: size - offset})
	return spans
}

// HashTo computes the (possibly fragmented) hash of an input stream and
// stores any resulting Fragments object in the object database.
//
// Files below Fragment.Threshold() are stored as a single blob. Larger files
// are split using either fixed-size or content-defined chunking depending on
// Fragment.EnableCDC.
func (r *Repository) HashTo(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
	if size < r.Fragment.Threshold() {
		oid, err = r.odb.HashTo(ctx, io.LimitReader(reader, size), size)
		return
	}
	if r.Fragment.EnableCDC.True() {
		return r.writeCDCFragments(ctx, reader, size)
	}
	return r.writeFixedFragments(ctx, reader, size)
}

// writeFixedFragments splits the stream at boundaries computed by
// calculateChunk and writes a Fragments object referencing each chunk.
func (r *Repository) writeFixedFragments(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
	spans := calculateChunk(size, r.Fragment.Size())

	h := plumbing.NewHasher()
	tr := io.TeeReader(reader, h)

	ff := &object.Fragments{
		Size:    uint64(size),
		Entries: make([]*object.Fragment, 0, len(spans)),
	}
	for i, span := range spans {
		chunkHash, hashErr := r.odb.HashTo(ctx, io.LimitReader(tr, span.Size), span.Size)
		if hashErr != nil {
			return plumbing.ZeroHash, false, hashErr
		}
		ff.Entries = append(ff.Entries, &object.Fragment{
			Index: uint32(i),
			Hash:  chunkHash,
			Size:  uint64(span.Size),
		})
	}

	ff.Origin = h.Sum()
	oid, err = r.odb.WriteEncoded(ff)
	fragments = true
	return
}

// writeCDCFragments splits the stream using FastCDC content-defined chunking
// and writes a Fragments object referencing each chunk.
func (r *Repository) writeCDCFragments(ctx context.Context, reader io.Reader, size int64) (oid plumbing.Hash, fragments bool, err error) {
	h := plumbing.NewHasher()
	tr := io.TeeReader(reader, h)

	ff := &object.Fragments{
		Size:    uint64(size),
		Entries: make([]*object.Fragment, 0),
	}

	index := uint32(0)
	chunker := NewChunker(r.Fragment.Size())
	walkErr := chunker.Walk(tr, func(span Span, data io.Reader) error {
		chunkHash, hashErr := r.odb.HashTo(ctx, data, span.Size)
		if hashErr != nil {
			return hashErr
		}
		ff.Entries = append(ff.Entries, &object.Fragment{
			Index: index,
			Hash:  chunkHash,
			Size:  uint64(span.Size),
		})
		index++
		return nil
	})
	if walkErr != nil {
		return plumbing.ZeroHash, false, walkErr
	}

	ff.Origin = h.Sum()
	oid, err = r.odb.WriteEncoded(ff)
	fragments = true
	return
}

func (r *Repository) WriteEncoded(e object.Encoder) (oid plumbing.Hash, err error) {
	return r.odb.WriteEncoded(e)
}
