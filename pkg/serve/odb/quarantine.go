// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type QuarantineDB struct {
	o *ODB
	q *backend.Database
}

func NewQuarantineDB(o *ODB, quarantineDir string) (*QuarantineDB, error) {
	q, err := backend.NewDatabase(quarantineDir, backend.WithCompressionALGO(o.odb.CompressionALGO()), backend.WithAbstractBackend(o))
	if err != nil {
		return nil, err
	}
	return &QuarantineDB{o: o, q: q}, nil
}

func (q *QuarantineDB) Close() error {
	if q.q != nil {
		return q.q.Close()
	}
	return nil
}

func (q *QuarantineDB) parseRev(ctx context.Context, oid plumbing.Hash) (a any, isolated bool, err error) {
	if a, err = q.q.Object(ctx, oid); !plumbing.IsNoSuchObject(err) {
		isolated = true
		return
	}
	a, err = q.o.Objects(ctx, oid)
	return
}

func (q *QuarantineDB) ParseRev(ctx context.Context, oid plumbing.Hash) (cc *object.Commit, isolated bool, err error) {
	var a any
	for range 10 {
		if a, isolated, err = q.parseRev(ctx, oid); err != nil {
			return
		}
		switch v := a.(type) {
		case *object.Commit:
			return v, isolated, nil
		case *object.Tag:
			if v.ObjectType != object.TagObject && v.ObjectType != object.CommitObject {
				err = backend.NewErrMismatchedObjectType(oid, "commit")
				return
			}
			oid = v.Object
		default:
			err = backend.NewErrMismatchedObjectType(oid, "commit")
			return
		}
	}
	err = backend.NewErrMismatchedObjectType(oid, "commit")
	return
}

func (q *QuarantineDB) Commit(ctx context.Context, oid plumbing.Hash) (cc *object.Commit, err error) {
	if cc, err = q.q.Commit(ctx, oid); !plumbing.IsNoSuchObject(err) {
		return
	}
	return q.o.Commit(ctx, oid)
}

func (q *QuarantineDB) Tree(ctx context.Context, oid plumbing.Hash) (t *object.Tree, err error) {
	if t, err = q.q.Tree(ctx, oid); !plumbing.IsNoSuchObject(err) {
		return
	}
	return q.o.Tree(ctx, oid)
}

func (q *QuarantineDB) Fragments(ctx context.Context, oid plumbing.Hash) (ff *object.Fragments, err error) {
	if ff, err = q.q.Fragments(ctx, oid); !plumbing.IsNoSuchObject(err) {
		return
	}
	return q.o.Fragments(ctx, oid)
}

func (q *QuarantineDB) Tag(ctx context.Context, oid plumbing.Hash) (tag *object.Tag, err error) {
	if tag, err = q.q.Tag(ctx, oid); !plumbing.IsNoSuchObject(err) {
		return
	}
	return q.o.Tag(ctx, oid)
}

func (q *QuarantineDB) Exists(ctx context.Context, oid plumbing.Hash, meta bool) error {
	if err := q.q.Exists(oid, meta); !plumbing.IsNoSuchObject(err) {
		return err
	}
	if meta {
		return q.o.odb.Exists(oid, meta)
	}
	if err := q.o.odb.Exists(oid, meta); !plumbing.IsNoSuchObject(err) {
		return err
	}
	return q.o.ossExists(ctx, oid)
}
