// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func (d *database) FindTag(ctx context.Context, rid int64, tagName string) (*Tag, error) {
	row := d.QueryRowContext(ctx, "select hash, subject, description, uid, created_at, updated_at from tags where rid = ? and name = ?", rid, tagName)
	t := &Tag{Name: tagName, RID: rid}
	err := row.Scan(&t.Hash, &t.Subject, &t.Description, &t.UID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		return nil, &ErrRevisionNotFound{Revision: tagName}
	}
	t.CreatedAt = t.CreatedAt.Local()
	t.UpdatedAt = t.UpdatedAt.Local()
	return t, nil
}

func (d *database) FindTagForPrefix(ctx context.Context, rid int64, prefix string) (*Tag, error) {
	rows, err := d.QueryContext(ctx, "select hash, name, subject, description, uid, created_at, updated_at  from tags  where rid = ? and (name = ? or name like ?)", rid, prefix, prefix+"/%")
	if err != nil {
		return nil, err
	}
	tags := make([]*Tag, 0, 10)
	for rows.Next() {
		t := &Tag{}
		if err := rows.Scan(&t.Hash, &t.Name, &t.Subject, &t.Description, &t.UID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt = t.CreatedAt.Local()
		t.UpdatedAt = t.UpdatedAt.Local()
		tags = append(tags, t)
	}
	if len(tags) == 0 {
		return nil, &ErrRevisionNotFound{Revision: prefix}
	}
	return tags[0], nil
}

func (d *database) doCreateTag(ctx context.Context, rid, uid int64, tagName string, newRev string, subject, description string) (*Tag, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	now := time.Now()
	result, err := d.ExecContext(ctx, "insert into tags(name, rid, uid, hash, subject, description, created_at, updated_at) values(?,?,?,?,?,?,?,?)",
		tagName, rid, uid, newRev, subject, description, now, now)
	if IsDupEntry(err) {
		_ = tx.Rollback()
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewTagReferenceName(tagName))}
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	a, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if a == 0 {
		_ = tx.Rollback()
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewTagReferenceName(tagName))}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Tag{Name: tagName, RID: rid, Hash: newRev, Subject: subject, Description: description, CreatedAt: now, UpdatedAt: now}, nil
}

func (d *database) doRemoveTag(ctx context.Context, rid int64, tagName string) (*Tag, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var t Tag
	if err := tx.QueryRowContext(ctx, "select hash, subject, description, uid, created_at, updated_at from tags where rid = ? and name = ?", rid, tagName).Scan(
		&t.Hash, &t.Subject, &t.Description, &t.UID, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, &ErrRevisionNotFound{Revision: string(plumbing.NewTagReferenceName(tagName))}
		}
		return nil, err
	}
	result, err := tx.ExecContext(ctx, "delete from tags where rid = ? and name = ?", rid, tagName)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	a, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if a == 0 {
		_ = tx.Rollback()
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewTagReferenceName(tagName))}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &t, nil
}

func (d *database) doTagUpdate(ctx context.Context, cmd *Command) (*Tag, error) {
	tagName := cmd.ReferenceName.TagName()
	if cmd.OldRev == plumbing.ZERO_OID {
		return d.doCreateTag(ctx, cmd.RID, cmd.UID, tagName, cmd.NewRev, cmd.Subject, cmd.Description)
	}
	if cmd.NewRev == plumbing.ZERO_OID {
		return d.doRemoveTag(ctx, cmd.RID, tagName)
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var oldRev string
	if err := tx.QueryRowContext(ctx, "select hash from tags where rid = ? and name = ?", cmd.RID, tagName).Scan(&oldRev); err != nil {
		if err == sql.ErrNoRows {
			return nil, &ErrRevisionNotFound{Revision: tagName}
		}
		return nil, err
	}
	if cmd.OldRev != oldRev {
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewTagReferenceName(tagName))}
	}
	result, err := tx.ExecContext(ctx, "update tags set hash = ?, subject=?, description=? where rid = ? and name = ? and hash = ?",
		cmd.NewRev, cmd.RID, tagName, cmd.OldRev, cmd.Subject, cmd.Description)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	a, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if a == 0 {
		_ = tx.Rollback()
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewTagReferenceName(tagName))}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.FindTag(ctx, cmd.RID, tagName)
}
