// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/oss"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type DB interface {
	Commit(ctx context.Context, oid plumbing.Hash) (cc *object.Commit, err error)
	Tree(ctx context.Context, oid plumbing.Hash) (t *object.Tree, err error)
	Fragments(ctx context.Context, oid plumbing.Hash) (*object.Fragments, error)
	Tag(ctx context.Context, oid plumbing.Hash) (*object.Tag, error)
	Objects(ctx context.Context, oid plumbing.Hash) (a any, err error)
	Open(ctx context.Context, oid plumbing.Hash, start int64) (sr backend.SizeReader, err error)
	Blob(ctx context.Context, oid plumbing.Hash) (b *object.Blob, err error)
	Push(ctx context.Context, oid plumbing.Hash) error // Push object to OSS
	WriteDirect(ctx context.Context, oid plumbing.Hash, r io.Reader, size int64) (int64, error)
	Stat(ctx context.Context, oid plumbing.Hash) (*oss.Stat, error)
	Sharing(ctx context.Context, oid plumbing.Hash, expiresAt int64) (*Representation, error)
}

type ODB struct {
	odb    *backend.Database
	cdb    CacheDB
	mdb    *MetadataDB
	bucket oss.Bucket
	rid    int64
}

var (
	_ DB = &ODB{}
)

func NewODB(rid int64, root string, compressionALGO string, cdb CacheDB, mdb *MetadataDB, bucket oss.Bucket) (*ODB, error) {
	o := &ODB{
		cdb:    cdb,
		mdb:    mdb,
		bucket: bucket,
		rid:    rid,
	}
	odb, err := backend.NewDatabase(root, backend.WithCompressionALGO(compressionALGO), backend.WithAbstractBackend(o))
	if err != nil {
		return nil, err
	}
	o.odb = odb
	return o, nil
}

// Reload: reload odb
func (o *ODB) Reload() error {
	root := o.odb.Root()
	compressionALGO := o.odb.CompressionALGO()
	if err := o.odb.Close(); err != nil {
		o.odb = nil
		return err
	}
	odb, err := backend.NewDatabase(root, backend.WithCompressionALGO(compressionALGO), backend.WithAbstractBackend(o))
	if err != nil {
		return err
	}
	o.odb = odb
	return nil
}

func (o *ODB) Close() error {
	if o.odb != nil {
		return o.odb.Close()
	}
	return nil
}

// Copy copy reader to writer
func Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := streamio.GetByteSlice()
	defer streamio.PutByteSlice(buf)
	return io.CopyBuffer(dst, src, *buf)
}
