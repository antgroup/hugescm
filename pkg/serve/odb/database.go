// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type MetadataDB struct {
	*sql.DB
	rid int64
}

func NewMetadataDB(db *sql.DB, rid int64) *MetadataDB {
	return &MetadataDB{DB: db, rid: rid}
}

func (d *MetadataDB) DecodeCommit(ctx context.Context, oid plumbing.Hash, b object.Backend) (*object.Commit, error) {
	var bindata string
	err := d.QueryRowContext(ctx, "select bindata from commits where rid = ? and hash = ?", d.rid, oid.String()).Scan(&bindata)
	if err == sql.ErrNoRows {
		return nil, plumbing.NoSuchObject(oid)
	}
	if err != nil {
		return nil, err
	}
	return object.Base64DecodeAs[object.Commit](bindata, oid, b)
}

// EncodeCommit: encode commit to DB
func (d *MetadataDB) EncodeCommit(ctx context.Context, cc *object.Commit) error {
	bindata, err := object.Base64Encode(cc)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, "insert into commits(rid, hash, author, committer, bindata) values(?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE rid = rid",
		d.rid, cc.Hash.String(), cc.Author.Email, cc.Committer.Email, bindata)
	return err
}

// BatchEncodeCommit: batch encode commit to DB
func (d *MetadataDB) BatchEncodeCommit(ctx context.Context, commits []*object.Commit) error {
	batchFn := func(cs []*object.Commit) error {
		if len(cs) == 0 {
			return nil
		}
		var args []any
		for _, c := range cs {
			bindata, err := object.Base64Encode(c)
			if err != nil {
				return err
			}
			args = append(args, d.rid, c.Hash.String(), c.Author.Email, c.Committer.Email, bindata)
		}
		sb := strings.Builder{}
		sb.WriteString("insert into commits(rid, hash, author, committer, bindata) values(?, ?, ?, ?, ?)")
		sb.WriteString(strings.Repeat(", (?, ?, ?, ?, ?)", len(cs)-1))
		sb.WriteString(" ON DUPLICATE KEY UPDATE rid = rid")
		_, err := d.ExecContext(ctx, sb.String(), args...)
		return err
	}
	for len(commits) > 0 {
		g := min(len(commits), 10)
		if err := batchFn(commits[0:g]); err != nil {
			return err
		}
		commits = commits[g:]
	}
	return nil
}

func (d *MetadataDB) DecodeTree(ctx context.Context, oid plumbing.Hash, b object.Backend) (*object.Tree, error) {
	var bindata string
	err := d.QueryRowContext(ctx, "select bindata from trees where rid = ? and hash = ?", d.rid, oid.String()).Scan(&bindata)
	if err == sql.ErrNoRows {
		return nil, plumbing.NoSuchObject(oid)
	}
	if err != nil {
		return nil, err
	}
	return object.Base64DecodeAs[object.Tree](bindata, oid, b)
}

func (d *MetadataDB) EncodeTree(ctx context.Context, t *object.Tree) error {
	bindata, err := object.Base64Encode(t)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, "insert into trees(rid, hash, bindata) values(?, ?, ?) ON DUPLICATE KEY UPDATE rid = rid",
		d.rid, t.Hash.String(), bindata)
	return err
}

func (d *MetadataDB) BatchEncodeTree(ctx context.Context, trees []*object.Tree) error {
	batchFn := func(ts []*object.Tree) error {
		if len(ts) == 0 {
			return nil
		}
		var args []any
		for _, tree := range ts {
			bindata, err := object.Base64Encode(tree)
			if err != nil {
				return err
			}
			args = append(args, d.rid, tree.Hash.String(), bindata)
		}

		sb := strings.Builder{}
		sb.WriteString("insert into trees(rid, hash, bindata) values(?, ?, ?)")
		sb.WriteString(strings.Repeat(", (?, ?, ?)", len(ts)-1))
		sb.WriteString(" ON DUPLICATE KEY UPDATE rid = rid")
		_, err := d.ExecContext(ctx, sb.String(), args...)
		return err
	}
	for len(trees) > 0 {
		g := min(len(trees), 10)
		if err := batchFn(trees[:g]); err != nil {
			return err
		}
		trees = trees[g:]
	}
	return nil
}

func (d *MetadataDB) Object(ctx context.Context, oid plumbing.Hash, b object.Backend) (any, error) {
	var bindata string
	if err := d.QueryRowContext(ctx, "select bindata from objects where rid = ? and hash = ?", d.rid, oid.String()).Scan(&bindata); err != nil {
		if err == sql.ErrNoRows {
			return nil, plumbing.NoSuchObject(oid)
		}
		return nil, err
	}
	return object.Base64Decode(bindata, oid, b)
}

func (d *MetadataDB) Fragments(ctx context.Context, oid plumbing.Hash, b object.Backend) (*object.Fragments, error) {
	a, err := d.Object(ctx, oid, b)
	if err != nil {
		return nil, err
	}
	if f, ok := a.(*object.Fragments); ok {
		return f, nil
	}
	return nil, backend.NewErrMismatchedObjectType(oid, "fragments")
}

func (d *MetadataDB) Tag(ctx context.Context, oid plumbing.Hash, b object.Backend) (*object.Tag, error) {
	a, err := d.Object(ctx, oid, b)
	if err != nil {
		return nil, err
	}
	if t, ok := a.(*object.Tag); ok {
		return t, nil
	}
	return nil, backend.NewErrMismatchedObjectType(oid, "tag")
}

func (d *MetadataDB) Encode(ctx context.Context, oid plumbing.Hash, e object.Encoder) error {
	bindata, err := object.Base64Encode(e)
	if err != nil {
		return err
	}
	if _, err := d.ExecContext(ctx, "insert into objects(rid, hash, bindata) values(?, ?, ?) ON DUPLICATE KEY UPDATE rid = rid", d.rid, oid.String(), bindata); err != nil {
		return fmt.Errorf("encode object %s error: %w", oid, err)
	}
	return nil
}

func (d *MetadataDB) BatchEncodeFragments(ctx context.Context, fss []*object.Fragments) error {
	batchFn := func(fs []*object.Fragments) error {
		if len(fs) == 0 {
			return nil
		}
		var args []any
		for _, f := range fs {
			bindata, err := object.Base64Encode(f)
			if err != nil {
				return err
			}
			args = append(args, d.rid, f.Hash.String(), bindata)
		}

		sb := strings.Builder{}
		sb.WriteString("insert into objects(rid, hash, bindata) values(?, ?, ?)")
		sb.WriteString(strings.Repeat(", (?, ?, ?)", len(fs)-1))
		sb.WriteString(" ON DUPLICATE KEY UPDATE rid = rid")
		_, err := d.ExecContext(ctx, sb.String(), args...)
		return err
	}
	for len(fss) > 0 {
		g := min(len(fss), 10)
		if err := batchFn(fss[:g]); err != nil {
			return err
		}
		fss = fss[g:]
	}
	return nil
}

func (d *MetadataDB) BatchEncodeTags(ctx context.Context, tags []*object.Tag) error {
	batchFn := func(ts []*object.Tag) error {
		if len(ts) == 0 {
			return nil
		}
		var args []any
		for _, f := range ts {
			bindata, err := object.Base64Encode(f)
			if err != nil {
				return err
			}
			args = append(args, d.rid, f.Hash.String(), bindata)
		}

		sb := strings.Builder{}
		sb.WriteString("insert into objects(rid, hash, bindata) values(?, ?, ?)")
		sb.WriteString(strings.Repeat(", (?, ?, ?)", len(ts)-1))
		sb.WriteString(" ON DUPLICATE KEY UPDATE rid = rid")
		_, err := d.ExecContext(ctx, sb.String(), args...)
		return err
	}
	for len(tags) > 0 {
		g := min(len(tags), 10)
		if err := batchFn(tags[:g]); err != nil {
			return err
		}
		tags = tags[g:]
	}
	return nil
}

func (o *ODB) batchCommits(ctx context.Context, oids []plumbing.Hash) error {
	commits := make([]*object.Commit, 0, len(oids))
	for _, oid := range oids {
		cc, err := o.odb.Commit(ctx, oid)
		if err != nil {
			return err
		}
		commits = append(commits, cc)
	}
	if err := o.mdb.BatchEncodeCommit(ctx, commits); err != nil {
		// Batch encode error
		return err
	}
	// cache commits
	for _, cc := range commits {
		_ = o.cdb.Store(ctx, o.rid, cc)
	}
	return nil
}

func (o *ODB) BatchCommits(ctx context.Context, oids []plumbing.Hash) error {
	for len(oids) > 0 {
		batchSize := min(len(oids), 1000)
		if err := o.batchCommits(ctx, oids[:batchSize]); err != nil {
			return err
		}
		oids = oids[batchSize:]
	}
	return nil
}

func (o *ODB) batchTrees(ctx context.Context, oids []plumbing.Hash) error {
	trees := make([]*object.Tree, 0, len(oids))
	for _, oid := range oids {
		cc, err := o.odb.Tree(ctx, oid)
		if err != nil {
			return err
		}
		trees = append(trees, cc)
	}
	if err := o.mdb.BatchEncodeTree(ctx, trees); err != nil {
		// Batch encode error
		return err
	}
	// cache commits
	for _, cc := range trees {
		_ = o.cdb.Store(ctx, o.rid, cc)
	}
	return nil
}

func (o *ODB) BatchTrees(ctx context.Context, oids []plumbing.Hash) error {
	for len(oids) > 0 {
		batchSize := min(len(oids), 1000)
		if err := o.batchTrees(ctx, oids[:batchSize]); err != nil {
			return err
		}
		oids = oids[batchSize:]
	}
	return nil
}

func (o *ODB) batchObjects(ctx context.Context, oids []plumbing.Hash) error {
	fragments := make([]*object.Fragments, 0, 100)
	tags := make([]*object.Tag, 0, 100)
	for _, oid := range oids {
		a, err := o.odb.Object(ctx, oid)
		if err != nil {
			return err
		}
		switch v := a.(type) {
		case *object.Fragments:
			fragments = append(fragments, v)
		case *object.Tag:
			tags = append(tags, v)
		default:
			return fmt.Errorf("object '%s' bad object type: %v", oid, reflect.TypeOf(a))
		}
	}
	if len(fragments) != 0 {
		if err := o.mdb.BatchEncodeFragments(ctx, fragments); err != nil {
			return err
		}
		for _, ff := range fragments {
			_ = o.cdb.Store(ctx, o.rid, ff)
		}
	}
	if len(tags) != 0 {
		if err := o.mdb.BatchEncodeTags(ctx, tags); err != nil {
			return err
		}
		for _, t := range tags {
			_ = o.cdb.Store(ctx, o.rid, t)
		}
	}
	return nil
}

func (o *ODB) BatchObjects(ctx context.Context, oids []plumbing.Hash) error {
	for len(oids) > 0 {
		batchSize := min(len(oids), 1000)
		if err := o.batchObjects(ctx, oids[:batchSize]); err != nil {
			return err
		}
		oids = oids[batchSize:]
	}
	return nil
}
