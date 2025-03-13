package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func (d *database) doRemoveBranch(ctx context.Context, rid int64, branchName string) (*Branch, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var branchID int64
	var oldRev string
	var createdAt, updateAt time.Time
	if err := tx.QueryRowContext(ctx, "select id, hash, created_at, updated_at from branches where rid = ? and name = ?",
		rid, branchName).Scan(&branchID, &oldRev, &createdAt, &updateAt); err != nil {
		if err == sql.ErrNoRows {
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
	return &Branch{Name: branchName, ID: branchID, RID: rid, Hash: oldRev, CreatedAt: createdAt.Local(), UpdatedAt: updateAt.Local()}, nil
}

func (d *database) doCreateBranch(ctx context.Context, rid int64, branchName string, newRev string) (*Branch, error) {
	now := time.Now()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	result, err := tx.ExecContext(ctx, "insert into branches(name, rid, hash, created_at, updated_at) values(?,?,?,?,?)", branchName, rid, newRev, now, now)
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
	return &Branch{ID: branchID, Name: branchName, RID: rid, Hash: newRev, CreatedAt: now, UpdatedAt: now}, nil
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
		if err == sql.ErrNoRows {
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
		cmd.NewRev, cmd.Subject, cmd.Description, cmd.RID, tagName, cmd.OldRev)
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

func (d *database) doCreateOrdinaryRef(ctx context.Context, rid int64, refname plumbing.ReferenceName, newRev string) (*Reference, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	now := time.Now()
	result, err := d.ExecContext(ctx, "insert into refs(name, rid, hash, created_at, updated_at) values(?,?,?,?,?)", refname, rid, newRev, now, now)
	if IsDupEntry(err) {
		_ = tx.Rollback()
		return nil, &ErrAlreadyLocked{Reference: string(refname)}
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
		return nil, &ErrAlreadyLocked{Reference: string(refname)}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Reference{Name: refname, RID: rid, Hash: newRev, CreatedAt: now, UpdatedAt: now}, nil
}

func (d *database) doRemoveOrdinaryRef(ctx context.Context, rid int64, refname plumbing.ReferenceName) (*Reference, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var ref Reference
	if err := tx.QueryRowContext(ctx, "select hash, created_at, updated_at from refs where rid = ? and name = ?",
		rid, refname).Scan(&ref.Hash, &ref.CreatedAt, &ref.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, &ErrRevisionNotFound{Revision: string(refname)}
		}
		return nil, err
	}
	result, err := tx.ExecContext(ctx, "delete from refs where rid = ? and name = ?", rid, refname)
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
		return nil, &ErrAlreadyLocked{Reference: string(refname)}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ref, nil
}

func (d *database) doOrdinaryRefUpdate(ctx context.Context, cmd *Command) (*Reference, error) {
	if cmd.OldRev == plumbing.ZERO_OID {
		return d.doCreateOrdinaryRef(ctx, cmd.RID, cmd.ReferenceName, cmd.NewRev)
	}
	if cmd.NewRev == plumbing.ZERO_OID {
		return d.doRemoveOrdinaryRef(ctx, cmd.RID, cmd.ReferenceName)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("new tx error: %v", err)
	}
	var oldRev string
	if err := tx.QueryRowContext(ctx, "select hash from refs where rid = ? and name = ?", cmd.RID, cmd.ReferenceName).Scan(&oldRev); err != nil {
		if err == sql.ErrNoRows {
			return nil, &ErrRevisionNotFound{Revision: string(cmd.ReferenceName)}
		}
		return nil, err
	}
	if cmd.OldRev != oldRev {
		return nil, &ErrAlreadyLocked{Reference: string(cmd.ReferenceName)}
	}
	result, err := tx.ExecContext(ctx, "update refs set hash = ? where rid = ? and name = ? and hash = ?", cmd.NewRev, cmd.RID, cmd.ReferenceName, cmd.OldRev)
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
		return nil, &ErrAlreadyLocked{Reference: string(cmd.ReferenceName)}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.FindOrdinaryReference(ctx, cmd.RID, cmd.ReferenceName)
}

func (d *database) DoReferenceUpdate(ctx context.Context, cmd *Command) (*Reference, error) {
	switch {
	case cmd.ReferenceName.IsBranch():
		b, err := d.DoBranchUpdate(ctx, cmd)
		if err != nil {
			return nil, err
		}
		return &Reference{
			ID:        b.ID,
			Name:      cmd.ReferenceName,
			RID:       b.RID,
			Hash:      b.Hash,
			CreatedAt: b.CreatedAt,
			UpdatedAt: b.UpdatedAt}, nil
	case cmd.ReferenceName.IsTag():
		t, err := d.doTagUpdate(ctx, cmd)
		if err != nil {
			return nil, err
		}
		return &Reference{
			Name:      cmd.ReferenceName,
			RID:       t.RID,
			Hash:      t.Hash,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		}, nil
	case cmd.ReferenceName.HasReferencePrefix():
		return d.doOrdinaryRefUpdate(ctx, cmd)
	}
	return nil, ErrReferenceNotAllowed
}
