// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
)

const (
	sqlListTags = `SELECT name, hash, subject, description, uid, created_at, updated_at
	FROM   tags
	WHERE  rid = ?
	ORDER BY name`
)

func (d *database) ListTags(ctx context.Context, rid int64) ([]*Tag, error) {
	rows, err := d.QueryContext(ctx, sqlListTags, rid)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	tags := make([]*Tag, 0)
	for rows.Next() {
		t := &Tag{RID: rid}
		if err := rows.Scan(&t.Name, &t.Hash, &t.Subject, &t.Description, &t.UID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt = t.CreatedAt.Local()
		t.UpdatedAt = t.UpdatedAt.Local()
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, &ErrRevisionNotFound{Revision: prefix}
	}
	return tags[0], nil
}
