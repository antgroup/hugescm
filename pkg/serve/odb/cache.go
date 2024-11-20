// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/dgraph-io/ristretto/v2"
)

func cacheKey(rid int64, oid plumbing.Hash) string {
	return fmt.Sprintf("%d/%s", rid, oid)
}

type CacheDB interface {
	Object(ctx context.Context, rid int64, oid plumbing.Hash) (any, error)
	Commit(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Commit, error)
	Tree(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Tree, error)
	Fragments(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Fragments, error)
	Tag(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Tag, error)
	Store(ctx context.Context, rid int64, a any) error
	Mark(rid int64, oid plumbing.Hash)
	Exist(rid int64, oid plumbing.Hash) bool
}

type cacheDB struct {
	*ristretto.Cache[string, any]
}

func NewCacheDB(numCounters int64, maxCost int64, bufferItems int64) (CacheDB, error) {
	c, err := ristretto.NewCache(&ristretto.Config[string, any]{
		NumCounters: numCounters,
		MaxCost:     maxCost << 30,
		BufferItems: bufferItems,
	})
	if err != nil {
		return nil, fmt.Errorf("unable initialize memory cache, error: %w", err)
	}
	return &cacheDB{Cache: c}, nil
}

func (d *cacheDB) Object(ctx context.Context, rid int64, oid plumbing.Hash) (any, error) {
	if o, ok := d.Get(cacheKey(rid, oid)); ok {
		return o, nil
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (d *cacheDB) Commit(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Commit, error) {
	if o, ok := d.Get(cacheKey(rid, oid)); ok {
		if c, ok := o.(*object.Commit); ok {
			return c, nil
		}
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (d *cacheDB) Tree(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Tree, error) {
	if o, ok := d.Get(cacheKey(rid, oid)); ok {
		if t, ok := o.(*object.Tree); ok {
			return t, nil
		}
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (d *cacheDB) Fragments(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Fragments, error) {
	if o, ok := d.Get(cacheKey(rid, oid)); ok {
		if f, ok := o.(*object.Fragments); ok {
			return f, nil
		}
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (d *cacheDB) Tag(ctx context.Context, rid int64, oid plumbing.Hash) (*object.Tag, error) {
	if o, ok := d.Get(cacheKey(rid, oid)); ok {
		if t, ok := o.(*object.Tag); ok {
			return t, nil
		}
	}
	return nil, plumbing.NoSuchObject(oid)
}

var (
	ErrUncacheableObject = errors.New("uncacheable object")
)

func (d *cacheDB) Store(ctx context.Context, rid int64, a any) error {
	switch v := a.(type) {
	case *object.Commit:
		// don't save backend
		_ = d.Set(cacheKey(rid, v.Hash), object.NewSnapshotCommit(v, nil), 1)
	case *object.Tree:
		// don't save backend
		d.SetWithTTL(cacheKey(rid, v.Hash), object.NewSnapshotTree(v, nil), 1, time.Hour*24)
	case *object.Fragments:
		d.SetWithTTL(cacheKey(rid, v.Hash), v, 1, time.Hour*4)
	case *object.Tag:
		_ = d.Set(cacheKey(rid, v.Hash), v, 1)
	default:
		return ErrUncacheableObject
	}
	return nil
}

func (d *cacheDB) Mark(rid int64, oid plumbing.Hash) {
	d.SetWithTTL(cacheKey(rid, oid), true, 1, time.Hour*24)
}

func (d *cacheDB) Exist(rid int64, oid plumbing.Hash) bool {
	_, ok := d.Get(cacheKey(rid, oid))
	return ok
}
