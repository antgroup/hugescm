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

const (
	sqlCountRepos = `SELECT COUNT(*) FROM repositories WHERE deleted_at = 0`

	sqlListRepos = `SELECT id, name, path, namespace_id, description, visible_level, default_branch, hash_algo, compression_algo, created_at, updated_at
	FROM   repositories
	WHERE  deleted_at = 0
	ORDER BY id
	LIMIT  ? OFFSET ?`

	sqlCountReposByNamespace = `SELECT COUNT(*) FROM repositories WHERE namespace_id = ? AND deleted_at = 0`

	sqlListReposByNamespace = `SELECT id, name, path, namespace_id, description, visible_level, default_branch, hash_algo, compression_algo, created_at, updated_at
	FROM   repositories
	WHERE  namespace_id = ? AND deleted_at = 0
	ORDER BY id
	LIMIT  ? OFFSET ?`
)

func (d *database) ListRepositories(ctx context.Context, page, perPage int) ([]*Repository, int64, error) {
	var total int64
	if err := d.QueryRowContext(ctx, sqlCountRepos).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := d.QueryContext(ctx, sqlListRepos, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close() //nolint:errcheck
	repos := make([]*Repository, 0, perPage)
	for rows.Next() {
		r := &Repository{}
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.NamespaceID, &r.Description, &r.VisibleLevel, &r.DefaultBranch, &r.HashAlgo, &r.CompressionAlgo, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		repos = append(repos, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return repos, total, nil
}

func (d *database) ListRepositoriesByNamespace(ctx context.Context, namespaceID int64, page, perPage int) ([]*Repository, int64, error) {
	var total int64
	if err := d.QueryRowContext(ctx, sqlCountReposByNamespace, namespaceID).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := d.QueryContext(ctx, sqlListReposByNamespace, namespaceID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close() //nolint:errcheck
	repos := make([]*Repository, 0, perPage)
	for rows.Next() {
		r := &Repository{}
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.NamespaceID, &r.Description, &r.VisibleLevel, &r.DefaultBranch, &r.HashAlgo, &r.CompressionAlgo, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		repos = append(repos, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return repos, total, nil
}

const (
	// sqlSearchRepos filters non-deleted repos by a LIKE substring on
	// name/path/description. The caller wraps q with '%'.
	sqlCountSearchRepos = `SELECT COUNT(*) FROM repositories WHERE deleted_at = 0 AND (name LIKE ? OR path LIKE ? OR description LIKE ?)`

	sqlSearchRepos = `SELECT id, name, path, namespace_id, description, visible_level, default_branch, hash_algo, compression_algo, created_at, updated_at
	FROM   repositories
	WHERE  deleted_at = 0 AND (name LIKE ? OR path LIKE ? OR description LIKE ?)
	ORDER BY id
	LIMIT  ? OFFSET ?`
)

func (d *database) SearchRepositories(ctx context.Context, q string, page, perPage int) ([]*Repository, int64, error) {
	like := "%" + q + "%"
	var total int64
	if err := d.QueryRowContext(ctx, sqlCountSearchRepos, like, like, like).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := d.QueryContext(ctx, sqlSearchRepos, like, like, like, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close() //nolint:errcheck
	repos := make([]*Repository, 0, perPage)
	for rows.Next() {
		r := &Repository{}
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.NamespaceID, &r.Description, &r.VisibleLevel, &r.DefaultBranch, &r.HashAlgo, &r.CompressionAlgo, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		repos = append(repos, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return repos, total, nil
}

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

const (
	// sqlUpdateRepository updates only the mutable repo metadata exposed via
	// the web settings page — description and visible_level. It deliberately
	// does not touch default_branch / hash_algo / compression_algo.
	sqlUpdateRepository = `UPDATE repositories
SET    description = ?,
       visible_level = ?,
       updated_at = ?
WHERE  id = ?`
)

func (d *database) UpdateRepository(ctx context.Context, r *Repository) (*Repository, error) {
	now := time.Now()
	if _, err := d.ExecContext(ctx, sqlUpdateRepository, r.Description, r.VisibleLevel, now, r.ID); err != nil {
		return nil, err
	}
	_, repo, err := d.FindRepositoryByID(ctx, int(r.ID))
	if err != nil {
		return nil, err
	}
	return repo, nil
}

const (
	// sqlUpdateRepoDefaultBranch is a dedicated statement that changes only the
	// repository's default-branch pointer. It must land a branch that actually
	// exists (validated by the caller) so checkout keeps resolving.
	sqlUpdateRepoDefaultBranch = `UPDATE repositories
SET    default_branch = ?,
       updated_at = ?
WHERE  id = ?`
)

func (d *database) UpdateRepositoryDefaultBranch(ctx context.Context, rid int64, branch string) (*Repository, error) {
	now := time.Now()
	if _, err := d.ExecContext(ctx, sqlUpdateRepoDefaultBranch, branch, now, rid); err != nil {
		return nil, err
	}
	_, repo, err := d.FindRepositoryByID(ctx, int(rid))
	if err != nil {
		return nil, err
	}
	return repo, nil
}
