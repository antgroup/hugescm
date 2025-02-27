// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/zeta/backend/pack"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

const (
	BLANK_BLOB = "af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262"
)

var (
	BLANK_BLOB_HASH = plumbing.NewHash(BLANK_BLOB)
)

var (
	ErrUncacheableObject = errors.New("uncacheable object")
)

func (d *Database) store(a any) error {
	if !d.enableLRU {
		return nil
	}
	switch v := a.(type) {
	case *object.Commit:
		// don't save backend
		_ = d.metaLRU.Set(v.Hash.String(), object.NewSnapshotCommit(v, nil), 1)
	case *object.Tree:
		// don't save backend
		_ = d.metaLRU.Set(v.Hash.String(), object.NewSnapshotTree(v, nil), 1)
	case *object.Fragments:
		_ = d.metaLRU.Set(v.Hash.String(), v, 1)
	case *object.Tag:
		_ = d.metaLRU.Set(v.Hash.String(), v, 1)
	default:
		return ErrUncacheableObject
	}
	return nil
}

func (d *Database) fromCache(oid plumbing.Hash) (any, error) {
	a, ok := d.metaLRU.Get(oid.String())
	if !ok {
		return nil, os.ErrNotExist
	}
	switch v := a.(type) {
	case *object.Commit:
		return object.NewSnapshotCommit(v, d), nil
	case *object.Tree:
		return object.NewSnapshotTree(v, d), nil
	case *object.Fragment:
		return v, nil
	case *object.Tag:
		return v, nil
	default:

	}
	return nil, ErrUncacheableObject
}

func (d *Database) Exists(oid plumbing.Hash, metadata bool) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if metadata {
		return d.metaRO.Exists(oid)
	}
	return d.ro.Exists(oid)
}

// Object: find object and set backend
// decode and set backend
func (d *Database) Object(_ context.Context, oid plumbing.Hash) (any, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.enableLRU {
		if a, err := d.fromCache(oid); err == nil {
			return a, nil
		}
	}
	rc, err := d.metaRO.Open(oid)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	a, err := object.Decode(rc, oid, d.backend)
	if err == nil {
		_ = d.store(a)
	}
	return a, err
}

func (d *Database) Commit(ctx context.Context, oid plumbing.Hash) (*object.Commit, error) {
	a, err := d.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	if c, ok := a.(*object.Commit); ok {
		return c, nil
	}
	return nil, NewErrMismatchedObjectType(oid, "commit")
}

func (d *Database) ParseRevEx(ctx context.Context, oid plumbing.Hash) (*object.Commit, []plumbing.Hash, error) {
	objects := make([]plumbing.Hash, 0, 2)
	for i := 0; i < 10; i++ {
		a, err := d.Object(ctx, oid)
		if err != nil {
			return nil, nil, err
		}
		if c, ok := a.(*object.Commit); ok {
			return c, objects, nil
		}
		t, ok := a.(*object.Tag)
		if !ok {
			return nil, nil, NewErrMismatchedObjectType(oid, "tag")
		}
		objects = append(objects, oid)
		if t.ObjectType != object.CommitObject && t.ObjectType != object.TagObject {
			return nil, nil, NewErrMismatchedObjectType(oid, "commit")
		}
		oid = t.Object
	}
	return nil, nil, NewErrMismatchedObjectType(oid, "commit")
}

func (d *Database) Tree(ctx context.Context, oid plumbing.Hash) (*object.Tree, error) {
	a, err := d.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	if t, ok := a.(*object.Tree); ok {
		return t, nil
	}
	return nil, NewErrMismatchedObjectType(oid, "tree")
}

func (d *Database) Fragments(ctx context.Context, oid plumbing.Hash) (*object.Fragments, error) {
	a, err := d.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	if f, ok := a.(*object.Fragments); ok {
		return f, nil
	}
	return nil, NewErrMismatchedObjectType(oid, "fragments")
}

func (d *Database) Tag(ctx context.Context, oid plumbing.Hash) (*object.Tag, error) {
	a, err := d.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	if t, ok := a.(*object.Tag); ok {
		return t, nil
	}
	return nil, NewErrMismatchedObjectType(oid, "tag")
}

func (d *Database) Blob(_ context.Context, oid plumbing.Hash) (blob *object.Blob, err error) {
	if oid == BLANK_BLOB_HASH {
		return &object.Blob{Contents: strings.NewReader("")}, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	var rc io.ReadCloser
	if rc, err = d.ro.Open(oid); err != nil {
		return nil, err
	}
	if blob, err = object.NewBlob(rc); err != nil {
		_ = rc.Close()
	}
	return
}

type SizeReader interface {
	io.Reader
	io.Closer
	Size() int64
}

type sizeReader struct {
	io.Reader
	closer io.Closer
	size   int64
}

func (sr *sizeReader) Close() error {
	if sr.closer == nil {
		return nil
	}
	return sr.closer.Close()
}

func (sr *sizeReader) Size() int64 {
	return sr.size
}

const (
	// ZSTD_MAGIC: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#frames
	ZSTD_MAGIC = 0xFD2FB528
)

func isZstdMagic(magic [4]byte) bool {
	return binary.LittleEndian.Uint32(magic[:]) == ZSTD_MAGIC
}

func (d *Database) metaSizeReader(oid plumbing.Hash) (SizeReader, error) {
	rc, err := d.metaRO.Open(oid)
	if err != nil {
		return nil, err
	}
	var magic [4]byte
	if _, err := io.ReadFull(rc, magic[:]); err != nil {
		return nil, err
	}
	// TODO: When the server supports compressed metadata, we don't need to decompress it.
	if isZstdMagic(magic) {
		defer rc.Close()
		b := &bytes.Buffer{}
		zr, err := streamio.GetZstdReader(rc)
		if err != nil {
			return nil, err
		}
		defer streamio.PutZstdReader(zr)
		if _, err := zr.WriteTo(b); err != nil {
			return nil, err
		}
		rawBytes := b.Bytes()
		return &sizeReader{Reader: bytes.NewReader(rawBytes), size: int64(len(rawBytes))}, nil
	}
	reader := io.MultiReader(bytes.NewReader(magic[:]), rc)
	switch v := rc.(type) {
	case *os.File:
		si, err := v.Stat()
		if err != nil {
			_ = v.Close()
			return nil, err
		}
		return &sizeReader{Reader: reader, closer: v, size: si.Size()}, nil
	case *pack.SizeReader:
		return &sizeReader{Reader: reader, closer: v, size: v.Size()}, nil
	default:
	}
	_ = rc.Close()
	return nil, errors.New("unable detect reader size")
}

func (d *Database) Size(oid plumbing.Hash, meta bool) (size int64, err error) {
	var sr SizeReader
	if sr, err = d.SizeReader(oid, meta); err != nil {
		return
	}
	size = sr.Size()
	_ = sr.Close()
	return
}

func (d *Database) SizeReader(oid plumbing.Hash, meta bool) (SizeReader, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if meta {
		return d.metaSizeReader(oid)
	}
	rc, err := d.ro.Open(oid)
	if err != nil {
		return nil, err
	}
	switch v := rc.(type) {
	case *os.File:
		si, err := v.Stat()
		if err != nil {
			_ = v.Close()
			return nil, err
		}
		return &sizeReader{Reader: v, closer: v, size: si.Size()}, nil
	case *pack.SizeReader:
		return &sizeReader{Reader: v, closer: v, size: v.Size()}, nil
	default:
	}
	_ = rc.Close()
	return nil, errors.New("unable detect reader size")
}

type readCloser struct {
	io.Reader
	closeFn func() error
}

func (r *readCloser) Close() error {
	if r.closeFn == nil {
		return nil
	}
	return r.closeFn()
}

func (d *Database) OpenReader(oid plumbing.Hash, meta bool) (io.ReadCloser, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if !meta {
		return d.ro.Open(oid)
	}
	rc, err := d.metaRO.Open(oid)
	if err != nil {
		return nil, err
	}
	var magic [4]byte
	if _, err := io.ReadFull(rc, magic[:]); err != nil {
		return nil, err
	}
	// TODO: When the server supports compressed metadata, we don't need to decompress it.
	if isZstdMagic(magic) {
		defer rc.Close()
		zr, err := streamio.GetZstdReader(rc)
		if err != nil {
			return nil, err
		}
		return &readCloser{Reader: zr, closeFn: func() error {
			streamio.PutZstdReader(zr)
			return rc.Close()
		}}, nil
	}
	return &readCloser{
		Reader: io.MultiReader(bytes.NewReader(magic[:]), rc),
		closeFn: func() error {
			return rc.Close()
		}}, nil
}

func (d *Database) Search(prefix string) (oid plumbing.Hash, err error) {
	h := plumbing.NewHash(prefix)
	if oid, err = d.metaRO.Search(h); err == nil {
		return
	}
	if !plumbing.IsNoSuchObject(err) {
		return
	}
	oid, err = d.ro.Search(h)
	return
}
