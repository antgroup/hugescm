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

func (r *Repository) WriteEncoded(e object.Encoder) (oid plumbing.Hash, err error) {
	return r.odb.WriteEncoded(e)
}
