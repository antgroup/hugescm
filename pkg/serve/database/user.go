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
	sqlSearhUserByEmail = `SELECT    u.id,
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
	zeroLockedAt = time.Unix(0, 0).UTC()
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
		if err := d.QueryRowContext(ctx, sqlSearhUserByEmail, emailOrName).Scan(
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
		return nil, fmt.Errorf("new tx error: %v", err)
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
