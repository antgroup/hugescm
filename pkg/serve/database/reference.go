// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func (d *database) FindOrdinaryReference(ctx context.Context, rid int64, refname plumbing.ReferenceName) (*Reference, error) {
	row := d.QueryRowContext(ctx, "select id, hash, created_at, updated_at from refs where rid = ? and name = ?", rid, refname)
	ref := Reference{RID: rid, Name: refname}
	err := row.Scan(&ref.ID, &ref.Hash, &ref.CreatedAt, &ref.UpdatedAt)
	if err == nil {
		ref.CreatedAt = ref.CreatedAt.Local()
		ref.UpdatedAt = ref.UpdatedAt.Local()
		return &ref, nil
	}
	if err == sql.ErrNoRows {
		return nil, &ErrRevisionNotFound{Revision: string(refname)}
	}
	return nil, err
}
