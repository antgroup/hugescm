// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (d *database) SearchKey(ctx context.Context, fingerprint string) (*Key, error) {
	var k Key
	if err := d.QueryRowContext(ctx, "select id, uid, content, title, type, fingerprint, created_at, updated_at from ssh_keys where fingerprint =?",
		fingerprint).Scan(&k.ID, &k.UID, &k.Content, &k.Title, &k.Type, &k.Fingerprint, &k.CreatedAt, &k.UpdatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

func (d *database) FindKey(ctx context.Context, id int64) (*Key, error) {
	var k Key
	if err := d.QueryRowContext(ctx, "select id, uid, content, title, type, fingerprint, created_at, updated_at from ssh_keys where id =?",
		id).Scan(&k.ID, &k.UID, &k.Content, &k.Title, &k.Type, &k.Fingerprint, &k.CreatedAt, &k.UpdatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

func (d *database) AddKey(ctx context.Context, k *Key) (*Key, error) {
	now := time.Now()
	_, err := d.ExecContext(ctx, "insert into ssh_keys(uid, content, title, type, fingerprint, created_at, updated_at) values(?,?,?,?,?,?,?)",
		k.UID, k.Content, k.Title, k.Type, k.Fingerprint, now, now)
	if IsDupEntry(err) {
		return nil, &ErrExist{message: "key already exists"}
	}
	if err != nil {
		return nil, err
	}
	return d.SearchKey(ctx, k.Fingerprint)
}

const (
	sqlListKeysByUser = `SELECT id, uid, title, type, fingerprint, created_at, updated_at
	FROM   ssh_keys
	WHERE  uid = ?
	ORDER BY id`
)

func (d *database) ListKeysByUser(ctx context.Context, uid int64) ([]*Key, error) {
	rows, err := d.QueryContext(ctx, sqlListKeysByUser, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	keys := make([]*Key, 0)
	for rows.Next() {
		k := &Key{}
		if err := rows.Scan(&k.ID, &k.UID, &k.Title, &k.Type, &k.Fingerprint, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (d *database) DeleteKey(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM ssh_keys WHERE id = ?", id)
	return err
}

func (d *database) IsDeployKeyEnabled(ctx context.Context, rid int64, kid int64) (bool, error) {
	var id int64
	if err := d.QueryRowContext(ctx, "select id from deploy_keys_repositories where rid=? and kid=?", rid, kid).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
