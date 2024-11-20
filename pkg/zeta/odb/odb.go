// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
)

const (
	DefaultHashALGO        = "BLAKE3"
	DefaultCompressionALGO = "zstd"
)

type ODB struct {
	*backend.Database
	root string
}

func NewODB(root string, opts ...backend.Option) (*ODB, error) {
	db, err := backend.NewDatabase(root, opts...)
	if err != nil {
		return nil, err
	}
	return &ODB{
		Database: db,
		root:     root,
	}, nil
}

func (d *ODB) Exists(oid plumbing.Hash, metadata bool) bool {
	return d.Database.Exists(oid, metadata) == nil
}

func (d *ODB) Root() string {
	return d.root
}
