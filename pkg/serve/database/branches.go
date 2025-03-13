// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"errors"
)

func (d *database) FindBranch(ctx context.Context, rid int64, branchName string) (*Branch, error) {
	row := d.QueryRowContext(ctx, "select id, hash, protection_level, created_at, updated_at from branches where rid = ? and name = ?", rid, branchName)
	b := Branch{RID: rid, Name: branchName}
	err := row.Scan(&b.ID, &b.Hash, &b.ProtectionLevel, &b.CreatedAt, &b.UpdatedAt)
	if err == nil {
		b.CreatedAt = b.CreatedAt.Local()
		b.UpdatedAt = b.UpdatedAt.Local()
		return &b, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrRevisionNotFound{Revision: branchName}
	}
	return nil, err
}

func (d *database) FindBranchForPrefix(ctx context.Context, rid int64, prefix string) (*Branch, error) {
	rows, err := d.QueryContext(ctx, "select id, name, hash, protection_level, created_at, updated_at from branches  where rid = ? and (name = ? or name like ?)", rid, prefix, prefix+"/%")
	if err != nil {
		return nil, err
	}
	branches := make([]*Branch, 0, 10)
	for rows.Next() {
		b := &Branch{}
		if err := rows.Scan(&b.ID, &b.Name, &b.Hash, &b.ProtectionLevel, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.CreatedAt = b.CreatedAt.Local()
		b.UpdatedAt = b.UpdatedAt.Local()
		branches = append(branches, b)
	}
	if len(branches) == 0 {
		return nil, &ErrRevisionNotFound{Revision: prefix}
	}
	return branches[0], nil
}
