// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
)

type RevObjects struct {
	Target  *object.Commit
	Objects map[string]object.Encoder
}

func (r *repository) fallbackParseRev(ctx context.Context, rev string) (a any, err error) {
	if b, err := r.mdb.FindBranch(ctx, r.rid, rev); err == nil {
		return r.odb.ParseRev(ctx, plumbing.NewHash(b.Hash))
	}
	if t, err := r.mdb.FindTag(ctx, r.rid, rev); err == nil {
		return r.odb.ParseRev(ctx, plumbing.NewHash(t.Hash))
	}
	return nil, plumbing.NewErrRevNotFound("not a valid object name %s", rev)
}

func (r *repository) resolveExhaustiveRev(ctx context.Context, rev string) (plumbing.Hash, error) {
	if rev == protocol.HEAD {
		b, err := r.mdb.FindBranch(ctx, r.rid, r.defaultBranch)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		return plumbing.NewHash(b.Hash), nil
	}
	if branchName, ok := strings.CutPrefix(rev, "refs/heads/"); ok {
		b, err := r.mdb.FindBranch(ctx, r.rid, branchName)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		return plumbing.NewHash(b.Hash), nil
	}
	if tagName, ok := strings.CutPrefix(rev, "refs/tags/"); ok {
		b, err := r.mdb.FindTag(ctx, r.rid, tagName)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		return plumbing.NewHash(b.Hash), nil
	}
	if plumbing.ValidateHashHex(rev) {
		return plumbing.NewHash(rev), nil
	}
	if b, err := r.mdb.FindBranch(ctx, r.rid, rev); err == nil {
		return plumbing.NewHash(b.Hash), nil
	}
	if t, err := r.mdb.FindTag(ctx, r.rid, rev); err == nil {
		return plumbing.NewHash(t.Hash), nil
	}
	return plumbing.ZeroHash, plumbing.NewErrRevNotFound("not a valid object name '%s'", rev)
}

func (r *repository) ParseRev(ctx context.Context, rev string) (*RevObjects, error) {
	oid, err := r.resolveExhaustiveRev(ctx, rev)
	if err != nil {
		return nil, err
	}
	a, err := r.odb.ParseRev(ctx, oid)
	if err != nil {
		if !plumbing.IsNoSuchObject(err) || !plumbing.ValidateHashHex(rev) {
			return nil, err
		}
		if a, err = r.fallbackParseRev(ctx, rev); err != nil {
			return nil, err
		}
	}
	if cc, ok := a.(*object.Commit); ok {
		return &RevObjects{Target: cc, Objects: make(map[string]object.Encoder)}, nil
	}
	current, ok := a.(*object.Tag)
	if !ok {
		return nil, plumbing.NewErrRevNotFound("not a valid object name '%s'", rev)
	}
	ro := &RevObjects{Objects: make(map[string]object.Encoder)}
	for {
		ro.Objects[current.Hash.String()] = current
		if current.ObjectType == object.BlobObject {
			break
		}
		if current.ObjectType == object.CommitObject {
			cc, err := r.odb.Commit(ctx, current.Object)
			if err != nil {
				return nil, err
			}
			ro.Target = cc
			return ro, nil
		}
		if current.ObjectType != object.TagObject {
			return nil, plumbing.NoSuchObject(current.Object)
		}
		tag, err := r.odb.Tag(ctx, current.Object)
		if err != nil {
			return nil, err
		}
		current = tag
	}
	return ro, nil
}
