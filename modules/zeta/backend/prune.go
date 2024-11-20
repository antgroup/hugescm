// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func (d *Database) PruneObject(ctx context.Context, oid plumbing.Hash, metadata bool) error {
	if metadata {
		return d.metaRW.PruneObject(ctx, oid)
	}
	return d.rw.PruneObject(ctx, oid)
}

func (d *Database) PruneObjects(ctx context.Context, largeSize int64) ([]plumbing.Hash, int64, error) {
	return d.rw.PruneObjects(ctx, largeSize)
}
