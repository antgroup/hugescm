// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func (o *ODB) ParseRev(ctx context.Context, oid plumbing.Hash) (a any, err error) {
	if a, err = o.cdb.Object(ctx, o.rid, oid); err == nil {
		if cc, ok := a.(*object.Commit); ok {
			return object.NewSnapshotCommit(cc, o), nil
		}
		if t, ok := a.(*object.Tag); ok {
			return t.Copy(), nil
		}
		return nil, plumbing.NewErrRevNotFound("not a valid object name %s", oid)
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	if a, err = o.odb.Object(ctx, oid); err == nil {
		_ = o.cdb.Store(ctx, o.rid, a)
		if cc, ok := a.(*object.Commit); ok {
			return cc, nil
		}
		if t, ok := a.(*object.Tag); ok {
			return t, nil
		}
		return nil, plumbing.NewErrRevNotFound("not a valid object name %s", oid)
	}
	if cc, err := o.mdb.DecodeCommit(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(cc)
		_ = o.cdb.Store(ctx, o.rid, cc)
		return cc, nil
	}
	var tag *object.Tag
	if tag, err = o.mdb.Tag(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(tag)
		_ = o.cdb.Store(ctx, o.rid, tag)
		return tag, nil
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (o *ODB) ParseRevExhaustive(ctx context.Context, oid plumbing.Hash) (*object.Commit, error) {
	a, err := o.ParseRev(ctx, oid)
	if err != nil {
		return nil, err
	}
	if cc, ok := a.(*object.Commit); ok {
		return cc, nil
	}
	current, ok := a.(*object.Tag)
	if !ok {
		return nil, backend.NewErrMismatchedObjectType(oid, "commit")
	}
	for i := 0; i < 10; i++ {
		if current.ObjectType == object.BlobObject {
			return nil, backend.NewErrMismatchedObjectType(oid, "commit")
		}
		if current.ObjectType == object.CommitObject {
			cc, err := o.Commit(ctx, current.Object)
			if err != nil {
				return nil, err
			}
			return cc, nil
		}
		if current.ObjectType != object.TagObject {
			return nil, plumbing.NoSuchObject(current.Object)
		}
		tag, err := o.Tag(ctx, current.Object)
		if err != nil {
			return nil, err
		}
		current = tag
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (o *ODB) Objects(ctx context.Context, oid plumbing.Hash) (a any, err error) {
	if a, err = o.cdb.Object(ctx, o.rid, oid); err == nil {
		switch v := a.(type) {
		case *object.Commit:
			return object.NewSnapshotCommit(v, o), nil
		case *object.Tree:
			return object.NewSnapshotTree(v, o), nil
		case *object.Fragments:
			return v, nil
		case *object.Tag:
			return v, nil
		default:
			return nil, ErrUncacheableObject
		}
	}
	if !plumbing.IsNoSuchObject(err) {
		return
	}
	if a, err = o.odb.Object(ctx, oid); err == nil {
		_ = o.cdb.Store(ctx, o.rid, a)
		return a, nil
	}
	if cc, err := o.mdb.DecodeCommit(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(cc)
		_ = o.cdb.Store(ctx, o.rid, cc)
		return cc, nil
	}
	if t, err := o.mdb.DecodeTree(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(t)
		_ = o.cdb.Store(ctx, o.rid, t)
		return t, nil
	}
	if a, err = o.mdb.Object(ctx, oid, o); err == nil {
		switch v := a.(type) {
		case *object.Fragments:
			_, _ = o.odb.WriteEncoded(v)
			_ = o.cdb.Store(ctx, o.rid, v)
		case *object.Tag:
			_, _ = o.odb.WriteEncoded(v)
			_ = o.cdb.Store(ctx, o.rid, v)
		default:
			return nil, ErrUncacheableObject
		}
		return a, nil
	}
	return
}

func (o *ODB) Commit(ctx context.Context, oid plumbing.Hash) (cc *object.Commit, err error) {
	if cc, err = o.cdb.Commit(ctx, o.rid, oid); err == nil {
		return object.NewSnapshotCommit(cc, o), nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return
	}
	if cc, err = o.odb.Commit(ctx, oid); err == nil {
		_ = o.cdb.Store(ctx, o.rid, cc)
		return cc, nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	if cc, err = o.mdb.DecodeCommit(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(cc)
		_ = o.cdb.Store(ctx, o.rid, cc)
		return cc, nil
	}
	return nil, err
}

func (o *ODB) Tree(ctx context.Context, oid plumbing.Hash) (t *object.Tree, err error) {
	if t, err = o.cdb.Tree(ctx, o.rid, oid); err == nil {
		return object.NewSnapshotTree(t, o), nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return
	}
	if t, err = o.odb.Tree(ctx, oid); err == nil {
		_ = o.cdb.Store(ctx, o.rid, t)
		return t, nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	if t, err = o.mdb.DecodeTree(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(t)
		_ = o.cdb.Store(ctx, o.rid, t)
		return t, nil
	}
	return nil, err
}

func (o *ODB) Fragments(ctx context.Context, oid plumbing.Hash) (*object.Fragments, error) {
	if ff, err := o.cdb.Fragments(ctx, o.rid, oid); !plumbing.IsNoSuchObject(err) {
		return ff, err
	}
	ff, err := o.odb.Fragments(ctx, oid)
	if err == nil {
		_ = o.cdb.Store(ctx, o.rid, ff)
		return ff, nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	if ff, err = o.mdb.Fragments(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(ff)
		_ = o.cdb.Store(ctx, o.rid, ff)
		return ff, nil
	}
	return nil, err
}

func (o *ODB) Tag(ctx context.Context, oid plumbing.Hash) (*object.Tag, error) {
	if t, err := o.cdb.Tag(ctx, o.rid, oid); !plumbing.IsNoSuchObject(err) {
		return t, err
	}
	t, err := o.odb.Tag(ctx, oid)
	if err == nil {
		return t, nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	var tag *object.Tag
	if tag, err = o.mdb.Tag(ctx, oid, o); err == nil {
		_, _ = o.odb.WriteEncoded(tag)
		_ = o.cdb.Store(ctx, o.rid, tag)
		return tag, nil
	}
	return nil, err
}

const (
	cachedThreshold   = 512 * 1024
	noPooledThreshold = 64 * 1024
)

type bufferedReader struct {
	io.Reader
	br   *bytes.Buffer
	size int64
}

func (r *bufferedReader) Size() int64 {
	return r.size
}

func (r *bufferedReader) Close() error {
	if r.br != nil {
		streamio.PutBytesBuffer(r.br)
	}
	return nil
}

func readSizeReader(sr backend.SizeReader) ([]byte, error) {
	b := make([]byte, 0, sr.Size())
	for {
		n, err := sr.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return b, err
		}

		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
	}
}

// newBufferedReader: For smaller files, we cache them to disk. Please note that during the caching process, our buffer application strategy is different.
// If it is a smaller file, we use pooled bytes.Buffer to avoid frequent allocation. Memory and GC, for slightly larger files, we directly apply for memory,
// which can avoid the program occupying a high amount of memory.
func (o *ODB) newBufferedReader(ctx context.Context, oid plumbing.Hash, sr backend.SizeReader) (backend.SizeReader, error) {
	defer sr.Close()
	if sr.Size() > noPooledThreshold {
		readBytes, err := readSizeReader(sr)
		if err != nil {
			return nil, err
		}
		_ = o.odb.WriteTo(ctx, oid, bytes.NewReader(readBytes))
		return &bufferedReader{Reader: bytes.NewReader(readBytes), size: sr.Size()}, nil
	}
	//
	br := streamio.GetBytesBuffer()
	br.Grow(int(sr.Size())) // Avoid allocate
	if _, err := br.ReadFrom(sr); err != nil {
		streamio.PutBytesBuffer(br)
		return nil, err
	}
	readBytes := br.Bytes()
	_ = o.odb.WriteTo(ctx, oid, bytes.NewReader(readBytes))
	return &bufferedReader{Reader: bytes.NewReader(readBytes), br: br, size: sr.Size()}, nil
}

func (o *ODB) Open(ctx context.Context, oid plumbing.Hash, start int64) (sr backend.SizeReader, err error) {
	if sr, err = o.odb.SizeReader(oid, false); err == nil {
		if start != 0 {
			if _, err := io.CopyN(io.Discard, sr, start); err != nil {
				_ = sr.Close()
				return nil, err
			}
		}
		return sr, nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	if sr, err = o.bucket.Open(ctx, ossJoin(o.rid, oid), start, -1); err != nil {
		return
	}
	if sr.Size() < cachedThreshold && start == 0 {
		return o.newBufferedReader(ctx, oid, sr)
	}
	return sr, nil
}

func (o *ODB) Blob(ctx context.Context, oid plumbing.Hash) (b *object.Blob, err error) {
	if oid == backend.BLANK_BLOB_HASH {
		return &object.Blob{Contents: strings.NewReader("")}, nil
	}
	sr, err := o.Open(ctx, oid, 0)
	if err != nil {
		return nil, err
	}
	return object.NewBlob(sr)
}

func (o *ODB) IsBinaryFast(ctx context.Context, oid plumbing.Hash) (bool, error) {
	if oid == backend.BLANK_BLOB_HASH {
		return false, nil
	}
	sr, err := o.Open(ctx, oid, 0)
	if err != nil {
		return false, err
	}
	defer sr.Close()
	var hdr [16]byte
	if _, err := io.ReadFull(sr, hdr[:]); err != nil {
		return false, err
	}
	method := object.CompressMethod(binary.BigEndian.Uint16(hdr[6:8]))
	return method != object.STORE, nil
}
