// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	sqlFindUser = `SELECT    username,
          name,
          admin,
          email,
		  type,
          password,
          signature_token,
          locked_at,
          created_at,
          updated_at
FROM      users
WHERE     id = ?`
	sqlSearchUserByName = `SELECT    id,
          username,
          name,
          admin,
          email,
		  type,
          password,
          signature_token,
          locked_at,
          created_at,
          updated_at
FROM      users
WHERE     username = ?`
	sqlSearchUserByEmail = `SELECT    u.id,
          u.username,
          u.name,
          u.admin,
          u.email,
		  u.type,
          u.password,
          u.signature_token,
          u.locked_at,
          u.created_at,
          u.updated_at
FROM      users AS u
INNER     JOIN emails AS e
WHERE     e.email = ?
AND       e.confirmed_at IS NOT NULL
AND       u.id = e.uid`
)

var (
	zeroLockedAt = sql.NullTime{}
)

func (d *database) FindUser(ctx context.Context, uid int64) (*User, error) {
	u := &User{
		ID: uid,
	}
	var lockedAt sql.NullTime
	if err := d.QueryRowContext(ctx, sqlFindUser, uid).Scan(
		&u.UserName, &u.Name, &u.Administrator, &u.Email, &u.Type, &u.Password, &u.SignatureToken, &lockedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	u.LockedAt = lockedAt.Time
	return u, nil
}

func (d *database) SearchUser(ctx context.Context, emailOrName string) (*User, error) {
	var lockedAt sql.NullTime
	if strings.Contains(emailOrName, "@") {
		var u User
		if err := d.QueryRowContext(ctx, sqlSearchUserByEmail, emailOrName).Scan(
			&u.ID, &u.UserName, &u.Name, &u.Administrator, &u.Email, &u.Type, &u.Password, &u.SignatureToken, &lockedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.LockedAt = lockedAt.Time
		return &u, nil
	}
	var u User
	if err := d.QueryRowContext(ctx, sqlSearchUserByName, emailOrName).Scan(
		&u.ID, &u.UserName, &u.Name, &u.Administrator, &u.Email, &u.Type, &u.Password, &u.SignatureToken, &lockedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	u.LockedAt = lockedAt.Time
	return &u, nil
}

func (d *database) NewUser(ctx context.Context, u *User) (*User, error) {
	now := time.Now()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %w", err)
	}
	result, err := tx.ExecContext(ctx, "insert into users(username,name,admin,email,type,password,signature_token,created_at,updated_at) values(?,?,?,?,?,?,?,?,?)",
		u.UserName, u.Name, u.Administrator, u.Email, u.Type, u.Password, u.SignatureToken, now, now)
	if IsDupEntry(err) {
		_ = tx.Rollback()
		return nil, &ErrExist{message: "user already exists"}
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	uid, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	_, err = tx.ExecContext(ctx, "insert into namespaces(path, name, owner_id, type, description, created_at, updated_at) values(?,?,?,?,?,?,?)",
		u.UserName, u.UserName, uid, 0, "", now, now)
	if IsDupEntry(err) {
		_ = tx.Rollback()
		return nil, &ErrExist{message: "namespace already exists"}
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.FindUser(ctx, uid)
}

const (
	sqlListUsers = `SELECT    id,
	          username,
	          name,
	          admin,
	          email,
	          type,
	          locked_at,
	          created_at,
	          updated_at
	FROM      users
	ORDER BY  id
	LIMIT     ? OFFSET ?`

	sqlCountUsers = `SELECT COUNT(*) FROM users`
)

func (d *database) ListUsers(ctx context.Context, page, perPage int) ([]*User, int64, error) {
	var total int64
	if err := d.QueryRowContext(ctx, sqlCountUsers).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := d.QueryContext(ctx, sqlListUsers, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close() //nolint:errcheck
	users := make([]*User, 0, perPage)
	for rows.Next() {
		u := &User{}
		var lockedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.UserName, &u.Name, &u.Administrator, &u.Email, &u.Type, &lockedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		u.LockedAt = lockedAt.Time
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

const (
	sqlUpdateUser = `UPDATE users
	SET    name = ?,
	       email = ?,
	       password = ?,
	       updated_at = ?
	WHERE  id = ?`
)

func (d *database) UpdateUser(ctx context.Context, u *User) (*User, error) {
	now := time.Now()
	_, err := d.ExecContext(ctx, sqlUpdateUser, u.Name, u.Email, u.Password, now, u.ID)
	if err != nil {
		return nil, err
	}
	return d.FindUser(ctx, u.ID)
}

const (
	sqlLockUser = `UPDATE users
	SET    locked_at = ?,
	       updated_at = ?
	WHERE  id = ?`
)

func (d *database) LockUser(ctx context.Context, uid int64) (*User, error) {
	now := time.Now()
	_, err := d.ExecContext(ctx, sqlLockUser, now, now, uid)
	if err != nil {
		return nil, err
	}
	return d.FindUser(ctx, uid)
}

func (d *database) UnlockUser(ctx context.Context, uid int64) (*User, error) {
	now := time.Now()
	_, err := d.ExecContext(ctx, sqlLockUser, zeroLockedAt, now, uid)
	if err != nil {
		return nil, err
	}
	return d.FindUser(ctx, uid)
}

const (
	// sqlCountReposByOwner counts all non-deleted repositories across every
	// namespace (personal + group) owned by the user. If the count > 0 the
	// user cannot be hard-deleted because the repos' author/committer history
	// and namespace ownership still reference them.
	sqlCountReposByOwner = `SELECT COUNT(*)
FROM   repositories r
INNER  JOIN namespaces n ON r.namespace_id = n.id
WHERE  n.owner_id = ?
AND    r.deleted_at = 0`

	// sqlLockUserOnDelete disables the account and wipes credentials. Unlike
	// the plain LockUser (which only sets locked_at for a temporary suspension),
	// this also clears password and signature_token so the user can never
	// authenticate again — the row is kept solely to preserve referential
	// integrity for the repositories they own.
	sqlLockUserOnDelete = `UPDATE users
SET    password = '',
       signature_token = '',
       locked_at = ?,
       updated_at = ?
WHERE  id = ?`

	// sqlDeleteUserEmails removes all email-map rows owned by the target user.
	sqlDeleteUserEmails = `DELETE FROM emails WHERE uid = ?`

	// sqlDeleteUser removes the user row itself. Only reached when the user
	// owns zero repositories, so there are no dangling references.
	sqlDeleteUser = `DELETE FROM users WHERE id = ?`
)

// DeleteUser deletes a user, or locks the account if the user still owns
// repositories. When the user has repos the account is permanently disabled
// (credentials wiped, locked_at set) so the row — and the repo ownership /
// commit history it anchors — is preserved; locked is returned as true.
// When the user has no repos the row and its email-map entries are
// hard-deleted in a single transaction; locked is returned as false.
func (d *database) DeleteUser(ctx context.Context, uid int64) (locked bool, err error) {
	var count int64
	if err := d.QueryRowContext(ctx, sqlCountReposByOwner, uid).Scan(&count); err != nil {
		return false, fmt.Errorf("count repos of user %d: %w", uid, err)
	}
	now := time.Now()
	if count > 0 {
		if _, err := d.ExecContext(ctx, sqlLockUserOnDelete, now, now, uid); err != nil {
			return false, fmt.Errorf("lock user %d on delete: %w", uid, err)
		}
		return true, nil
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // commit wins on success
	if _, err := tx.ExecContext(ctx, sqlDeleteUserEmails, uid); err != nil {
		return false, fmt.Errorf("delete emails of user %d: %w", uid, err)
	}
	if _, err := tx.ExecContext(ctx, sqlDeleteUser, uid); err != nil {
		return false, fmt.Errorf("delete user %d: %w", uid, err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return false, nil
}

const (
	// sqlSetUserAdministrator is a dedicated statement that grants or revokes
	// administrator status. It deliberately touches only the admin column —
	// not folded into sqlUpdateUser — so privilege changes can be applied and
	// audited independently of profile (name/email/password) edits.
	sqlSetUserAdministrator = `UPDATE users
SET    admin = ?,
       updated_at = ?
WHERE  id = ?`
)

func (d *database) SetUserAdministrator(ctx context.Context, uid int64, admin bool) error {
	now := time.Now()
	_, err := d.ExecContext(ctx, sqlSetUserAdministrator, admin, now, uid)
	return err
}
