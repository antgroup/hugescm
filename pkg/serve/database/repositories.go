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
	sqlRepoFromID = `select
  r.id
, r.name
, r.path
, r.description
, r.visible_level
, r.default_branch
, r.hash_algo
, r.compression_algo
, r.created_at
, r.updated_at
, n.id
, n.path
, n.name
, n.description
, n.owner_id
, n.type
, n.created_at
, n.updated_at
from
repositories as r inner join namespaces as n on r.namespace_id = n.id
where
r.id = ?`
)

func (d *database) FindRepositoryByID(ctx context.Context, rid int) (*Namespace, *Repository, error) {
	var n Namespace
	var r Repository
	// query repo table to find repo
	if err := d.QueryRowContext(ctx, sqlRepoFromID, rid).Scan(
		&r.ID, &r.Name, &r.Path, &r.Description, &r.VisibleLevel, &r.DefaultBranch, &r.HashAlgo, &r.CompressionAlgo, &r.CreatedAt, &r.UpdatedAt, // repositories
		&n.ID, &n.Path, &n.Name, &n.Description, &n.Owner, &n.Type, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, nil, err
	}
	r.NamespaceID = n.ID
	return &n, &r, nil
}

const (
	sqlRepoFromPath = `select
	r.id
  , r.name
  , r.path
  , r.description
  , r.visible_level
  , r.default_branch
  , r.hash_algo
  , r.compression_algo
  , r.created_at
  , r.updated_at
  , n.id
  , n.path
  , n.name
  , n.description
  , n.owner_id
  , n.type
  , n.created_at
  , n.updated_at
  from
  repositories as r inner join namespaces as n on r.namespace_id = n.id
where
  n.path = ?
  and r.path = ?`
)

func (d *database) FindRepositoryByPath(ctx context.Context, namespacePath, repoPath string) (*Namespace, *Repository, error) {
	var n Namespace
	var r Repository
	// query repo table to find repo
	if err := d.QueryRowContext(ctx, sqlRepoFromPath, namespacePath, repoPath).Scan(
		&r.ID, &r.Name, &r.Path, &r.Description, &r.VisibleLevel, &r.DefaultBranch, &r.HashAlgo, &r.CompressionAlgo, &r.CreatedAt, &r.UpdatedAt, // repositories
		&n.ID, &n.Path, &n.Name, &n.Description, &n.Owner, &n.Type, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, nil, err
	}
	r.NamespaceID = n.ID
	return &n, &r, nil
}

const (
	sqlNewRepository = `INSERT    INTO repositories (
          name,
          path,
          description,
          visible_level,
          default_branch,
          hash_algo,
          compression_algo,
          namespace_id,
          created_at,
          updated_at
          )
VALUES    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

func (d *database) NewRepository(ctx context.Context, r *Repository) (*Repository, error) {
	var err error
	if err = r.Validate(); err != nil {
		return nil, err
	}
	now := time.Now()
	result, err := d.ExecContext(ctx, sqlNewRepository, r.Name, r.Path, r.Description, r.VisibleLevel, r.DefaultBranch, r.HashAlgo, r.CompressionAlgo, r.NamespaceID, now, now)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ErrExist{message: "repository already exists"}
		}
		return nil, err
	}
	rid, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Repository{
		ID:              rid,
		Name:            r.Name,
		Path:            r.Path,
		Description:     r.Description,
		VisibleLevel:    r.VisibleLevel,
		DefaultBranch:   r.DefaultBranch,
		HashAlgo:        r.HashAlgo,
		CompressionAlgo: r.CompressionAlgo,
		UpdatedAt:       now,
		CreatedAt:       now,
	}, nil
}
