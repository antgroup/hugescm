// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
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

func (d *database) doRemoveBranch(ctx context.Context, rid int64, branchName string) (*Branch, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var branchID int64
	var oldRev string
	var protectionLevel int
	var createdAt, updateAt time.Time
	if err := tx.QueryRowContext(ctx, "select id, hash, protection_level, created_at, updated_at from branches where rid = ? and name = ?",
		rid, branchName).Scan(&branchID, &oldRev, &protectionLevel, &createdAt, &updateAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ErrRevisionNotFound{Revision: string(plumbing.NewBranchReferenceName(branchName))}
		}
		return nil, err
	}
	result, err := tx.ExecContext(ctx, "delete from branches where rid = ? and name = ?", rid, branchName)
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
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewBranchReferenceName(branchName))}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Branch{Name: branchName, ID: branchID, RID: rid, Hash: oldRev, ProtectionLevel: protectionLevel, CreatedAt: createdAt.Local(), UpdatedAt: updateAt.Local()}, nil
}

func (d *database) doCreateBranch(ctx context.Context, rid int64, branchName string, newRev string) (*Branch, error) {
	now := time.Now()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	result, err := tx.ExecContext(ctx, "insert into branches(name, rid, hash, protection_level, created_at, updated_at) values(?,?,?,?,?,?)", branchName, rid, newRev, 0, now, now)
	if IsDupEntry(err) {
		_ = tx.Rollback()
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewBranchReferenceName(branchName))}
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
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewBranchReferenceName(branchName))}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	branchID, _ := result.LastInsertId()
	return &Branch{ID: branchID, Name: branchName, RID: rid, Hash: newRev, ProtectionLevel: 0, CreatedAt: now, UpdatedAt: now}, nil
}

func (d *database) DoBranchUpdate(ctx context.Context, cmd *Command) (*Branch, error) {
	branchName := cmd.ReferenceName.BranchName()
	if cmd.OldRev == plumbing.ZERO_OID {
		return d.doCreateBranch(ctx, cmd.RID, branchName, cmd.NewRev)
	}
	if cmd.NewRev == plumbing.ZERO_OID {
		return d.doRemoveBranch(ctx, cmd.RID, branchName)
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var oldRev string
	var protectionLevel int
	if err := tx.QueryRowContext(ctx, "select hash,protection_level from branches where rid = ? and name = ?", cmd.RID, branchName).Scan(&oldRev, &protectionLevel); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ErrRevisionNotFound{Revision: branchName}
		}
		return nil, err
	}
	if cmd.OldRev != oldRev {
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewBranchReferenceName(branchName))}
	}
	result, err := tx.ExecContext(ctx, "update branches set hash = ? where rid = ? and name = ? and hash = ?", cmd.NewRev, cmd.RID, branchName, cmd.OldRev)
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
		return nil, &ErrAlreadyLocked{Reference: string(plumbing.NewBranchReferenceName(branchName))}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.FindBranch(ctx, cmd.RID, branchName)
}
