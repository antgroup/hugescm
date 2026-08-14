// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

const sqlListDeployKeysByRepo = `SELECT k.id, k.uid, k.title, k.type, k.fingerprint, k.created_at, k.updated_at
FROM   ssh_keys k
INNER JOIN deploy_keys_repositories dkr ON k.id = dkr.kid
WHERE  dkr.rid = ?
ORDER BY k.id`

func (d *database) ListDeployKeysByRepo(ctx context.Context, rid int64) ([]*Key, error) {
	rows, err := d.QueryContext(ctx, sqlListDeployKeysByRepo, rid)
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

// AddDeployKey creates a deploy key (type=DeployKey, not bound to any user)
// and binds it to the given repository via deploy_keys_repositories.
// Both inserts are wrapped in a single transaction so a binding failure
// (e.g. duplicate fingerprint) does not leave an orphan ssh_keys row.
func (d *database) AddDeployKey(ctx context.Context, k *Key, rid int64) (*Key, error) {
	now := time.Now()
	if k.Type != DeployKey {
		k.Type = DeployKey
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %w", err)
	}
	result, err := tx.ExecContext(ctx, "insert into ssh_keys(uid, content, title, type, fingerprint, created_at, updated_at) values(?,?,?,?,?,?,?)",
		0, k.Content, k.Title, k.Type, k.Fingerprint, now, now)
	if IsDupEntry(err) {
		_ = tx.Rollback()
		return nil, &ErrExist{message: "key already exists"}
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	kid, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "insert into deploy_keys_repositories(kid, rid) values(?,?)", kid, rid); err != nil {
		_ = tx.Rollback()
		if IsDupEntry(err) {
			return nil, &ErrExist{message: "deploy key already exists"}
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.SearchKey(ctx, k.Fingerprint)
}

// RemoveDeployKey unbinds a deploy key from the repository and deletes
// the key record. A deploy key is repo-scoped, so removal is a two-step
// delete inside a single transaction: first the binding (verified by
// kid+rid), then the key itself (guarded by type=DeployKey).
//
// Security: the binding DELETE checks both kid AND rid, so a caller
// cannot remove a key that is not bound to their repo. The RowsAffected
// count must be 1 before deleting the ssh_keys row, and the ssh_keys
// DELETE includes a type=DeployKey guard as defense-in-depth to ensure
// a BasicKey (personal SSH key) can never be deleted through this path.
func (d *database) RemoveDeployKey(ctx context.Context, kid int64, rid int64) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("new tx error: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM deploy_keys_repositories WHERE kid=? AND rid=?", kid, rid)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n == 0 {
		_ = tx.Rollback()
		// No binding found for this kid+rid — the key does not belong
		// to this repo (or does not exist). Do not touch ssh_keys.
		return nil
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM ssh_keys WHERE id=? AND type=?", kid, DeployKey); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
