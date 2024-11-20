// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type Reference struct {
	ID              int64                  `json:"id"`
	Name            plumbing.ReferenceName `json:"name"`
	RID             int64                  `json:"rid"`
	Hash            string                 `json:"hash"`
	ProtectionLevel int                    `json:"protection_level"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

func (d *database) DoReferenceUpdate(ctx context.Context, cmd *Command) (*Reference, error) {
	switch {
	case cmd.ReferenceName.IsBranch():
		b, err := d.DoBranchUpdate(ctx, cmd)
		if err != nil {
			return nil, err
		}
		return &Reference{
			ID:              b.ID,
			Name:            cmd.ReferenceName,
			RID:             b.RID,
			Hash:            b.Hash,
			ProtectionLevel: b.ProtectionLevel,
			CreatedAt:       b.CreatedAt,
			UpdatedAt:       b.UpdatedAt}, nil
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
	}
	return nil, ErrReferenceNotAllowed
}
