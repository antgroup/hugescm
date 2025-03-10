// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"io"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// EncodeFast: Store the object and update the Hash value of the object
func (o *ODB) EncodeFast(ctx context.Context, e object.Encoder) (oid plumbing.Hash, err error) {
	if oid, err = o.odb.WriteEncoded(e); err != nil {
		return
	}
	switch v := e.(type) {
	case *object.Commit:
		v.Hash = oid
	case *object.Tree:
		v.Hash = oid
	case *object.Fragments:
		v.Hash = oid
	case *object.Tag:
		v.Hash = oid
	}
	return
}

// EncodeEx: Store the object and update the Hash value of the object
func (o *ODB) Encode(ctx context.Context, e object.Encoder) (oid plumbing.Hash, err error) {
	if oid, err = o.odb.WriteEncoded(e); err != nil {
		return
	}
	switch v := e.(type) {
	case *object.Commit:
		v.Hash = oid
		if err = o.mdb.EncodeCommit(ctx, v); err != nil {
			return
		}
	case *object.Tree:
		v.Hash = oid
		if err = o.mdb.EncodeTree(ctx, v); err != nil {
			return
		}
	case *object.Fragments:
		v.Hash = oid
		if err = o.mdb.Encode(ctx, oid, v); err != nil {
			return
		}
	case *object.Tag:
		v.Hash = oid
		if err = o.mdb.Encode(ctx, oid, v); err != nil {
			return
		}
	}
	_ = o.cdb.Store(ctx, o.rid, e)
	return
}

func (o *ODB) HashFast(ctx context.Context, r io.Reader, size int64) (oid plumbing.Hash, err error) {
	return o.odb.HashTo(ctx, r, size)
}

// HashTo: Encode the read stream into a blob and upload it
func (o *ODB) HashTo(ctx context.Context, r io.Reader, size int64) (oid plumbing.Hash, err error) {
	if oid, err = o.odb.HashTo(ctx, r, size); err != nil {
		return
	}
	resourcePath := ossJoin(o.rid, oid)
	if _, err = o.bucket.Stat(ctx, resourcePath); err == nil {
		return oid, nil
	}
	if !os.IsNotExist(err) {
		return plumbing.ZeroHash, err
	}
	var sr backend.SizeReader
	if sr, err = o.odb.SizeReader(oid, false); err != nil {
		return
	}
	defer sr.Close()
	if err = o.bucket.LinearUpload(ctx, resourcePath, sr, size, OSS_ZETA_BLOB_MIME); err != nil {
		return plumbing.ZeroHash, err
	}
	return oid, nil
}
