// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func (d *ODB) DecodeTo(ctx context.Context, w io.Writer, oid plumbing.Hash, n int64) error {
	if oid == backend.BLANK_BLOB_HASH {
		return nil // empty blob, skip
	}
	if n <= 0 {
		n = math.MaxInt64
	}
	b, err := d.Blob(ctx, oid)
	if err != nil {
		return err
	}
	defer b.Close() // nolint
	if _, err = io.CopyN(w, b.Contents, min(n, b.Size)); err != nil {
		return err
	}
	return nil
}

func (d *ODB) DecodeFragments(ctx context.Context, w io.Writer, fe *object.TreeEntry) error {
	fragments, err := d.Fragments(ctx, fe.Hash)
	if err != nil {
		return err
	}
	hasher := plumbing.NewHasher()
	w = io.MultiWriter(w, hasher)
	for _, e := range fragments.Entries {
		if err := d.DecodeTo(ctx, w, e.Hash, -1); err != nil {
			return err
		}
	}
	got := hasher.Sum()
	if got != fragments.Origin {
		return fmt.Errorf("decode fragments error: hash mistake, want: %s got: %s", fragments.Origin, got)
	}
	return nil
}
