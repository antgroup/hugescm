// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	// group and global access level
	groupAccessSQL = `SELECT access_level FROM members
  WHERE
    uid = ? and rid = ? and source_type = 3`
)

func (d *database) GroupAccessLevel(ctx context.Context, namespaceID int64, u *User) (AccessLevel, error) {
	if u == nil {
		return NoneAccess, ErrUserNotGiven
	}
	if u.Administrator {
		return OwnerAccess, nil
	}
	var accessLevel AccessLevel
	if err := d.QueryRowContext(ctx, groupAccessSQL, u.ID, namespaceID).Scan(&accessLevel); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NoneAccess, nil
		}
		return NoneAccess, err
	}
	return accessLevel, nil
}

const (
	repoMemberSQL = `SELECT access_level FROM members
WHERE
    uid = ? and	rid = ? and source_type = 2`
)

// query access to group and access to repo
func (d *database) RepoAccessLevel(ctx context.Context, r *Repository, u *User) (AccessLevel, AccessLevel, error) {
	// admin have max access
	if u.Administrator {
		return OwnerAccess, OwnerAccess, nil
	}
	// find group access
	groupAccess, err := d.GroupAccessLevel(ctx, r.NamespaceID, u)
	if err != nil {
		return NoneAccess, NoneAccess, err
	}
	// repo access >= group access
	repoAccess := NoneAccess
	if err := d.QueryRowContext(ctx, repoMemberSQL, u.ID, r.ID).Scan(&repoAccess); err != nil && err != sql.ErrNoRows {
		return NoneAccess, NoneAccess, err
	}
	if repoAccess = max(groupAccess, repoAccess); repoAccess >= ReporterAccess {
		return groupAccess, repoAccess, nil
	}
	// Here we can also implement outsourced user-level permission management.
	if r.VisibleLevel == 20 {
		repoAccess = ReporterAccess
	}
	return groupAccess, repoAccess, nil
}

const (
	sqlNewMember = `INSERT    INTO members (rid, uid, access_level, source_type, expires_at, created_at, updated_at)
VALUES    (?, ?, ?, ?, ?, ?, ?)
ON        DUPLICATE KEY UPDATE expires_at = VALUES(expires_at)`
)

func (d *database) AddMember(ctx context.Context, m *Member) error {
	now := time.Now()
	_, err := d.ExecContext(ctx, sqlNewMember, &m.SourceID, &m.UID, &m.AccessLevel, &m.SourceType, &m.ExpiresAt, now, now)
	if err != nil {
		return err
	}
	return nil
}
