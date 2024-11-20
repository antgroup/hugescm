// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type Backend interface {
	Commit(ctx context.Context, oid plumbing.Hash) (*Commit, error)
	Tree(ctx context.Context, oid plumbing.Hash) (*Tree, error)
	Fragments(ctx context.Context, oid plumbing.Hash) (*Fragments, error)
	Tag(ctx context.Context, oid plumbing.Hash) (*Tag, error)
	Blob(ctx context.Context, oid plumbing.Hash) (*Blob, error)
}
