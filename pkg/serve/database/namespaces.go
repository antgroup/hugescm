// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"time"
)

func (d *database) FindNamespaceByID(ctx context.Context, namespaceID int64) (*Namespace, error) {
	var n Namespace
	if err := d.QueryRowContext(ctx, "select id, path, name, owner_id, type, description, created_at, updated_at from namespaces where id = ?", namespaceID).
		Scan(&n.ID, &n.Path, &n.Name, &n.Owner, &n.Type, &n.Description, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, err
	}
	return &n, nil
}

func (d *database) FindNamespaceByPath(ctx context.Context, namespacePath string) (*Namespace, error) {
	var n Namespace
	if err := d.QueryRowContext(ctx, "select id, path, name, owner_id, type, description, created_at, updated_at from namespaces where name = ?", namespacePath).
		Scan(&n.ID, &n.Path, &n.Name, &n.Owner, &n.Type, &n.Description, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, err
	}
	return &n, nil
}

const (
	sqlCountNamespaces = `SELECT COUNT(*) FROM namespaces`

	sqlListNamespaces = `SELECT id, path, name, owner_id, type, description, created_at, updated_at
	FROM   namespaces
	ORDER BY id
	LIMIT  ? OFFSET ?`

	sqlCountNamespacesByType = `SELECT COUNT(*) FROM namespaces WHERE type = ?`

	sqlListNamespacesByType = `SELECT id, path, name, owner_id, type, description, created_at, updated_at
	FROM   namespaces
	WHERE  type = ?
	ORDER BY id
	LIMIT  ? OFFSET ?`

	sqlCountNamespacesByOwner = `SELECT COUNT(*) FROM namespaces WHERE owner_id = ?`

	sqlListNamespacesByOwner = `SELECT id, path, name, owner_id, type, description, created_at, updated_at
	FROM   namespaces
	WHERE  owner_id = ?
	ORDER BY id
	LIMIT  ? OFFSET ?`

	sqlCountNamespacesByTypeAndOwner = `SELECT COUNT(*) FROM namespaces WHERE type = ? AND owner_id = ?`

	sqlListNamespacesByTypeAndOwner = `SELECT id, path, name, owner_id, type, description, created_at, updated_at
	FROM   namespaces
	WHERE  type = ? AND owner_id = ?
	ORDER BY id
	LIMIT  ? OFFSET ?`
)

func (d *database) ListNamespaces(ctx context.Context, nsType *int, ownerID *int64, page, perPage int) ([]*Namespace, int64, error) {
	offset := (page - 1) * perPage

	switch {
	case nsType != nil && ownerID != nil:
		var total int64
		if err := d.QueryRowContext(ctx, sqlCountNamespacesByTypeAndOwner, *nsType, *ownerID).Scan(&total); err != nil {
			return nil, 0, err
		}
		rows, err := d.QueryContext(ctx, sqlListNamespacesByTypeAndOwner, *nsType, *ownerID, perPage, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close() //nolint:errcheck
		return scanNamespaces(rows, total)

	case nsType != nil:
		var total int64
		if err := d.QueryRowContext(ctx, sqlCountNamespacesByType, *nsType).Scan(&total); err != nil {
			return nil, 0, err
		}
		rows, err := d.QueryContext(ctx, sqlListNamespacesByType, *nsType, perPage, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close() //nolint:errcheck
		return scanNamespaces(rows, total)

	case ownerID != nil:
		var total int64
		if err := d.QueryRowContext(ctx, sqlCountNamespacesByOwner, *ownerID).Scan(&total); err != nil {
			return nil, 0, err
		}
		rows, err := d.QueryContext(ctx, sqlListNamespacesByOwner, *ownerID, perPage, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close() //nolint:errcheck
		return scanNamespaces(rows, total)

	default:
		var total int64
		if err := d.QueryRowContext(ctx, sqlCountNamespaces).Scan(&total); err != nil {
			return nil, 0, err
		}
		rows, err := d.QueryContext(ctx, sqlListNamespaces, perPage, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close() //nolint:errcheck
		return scanNamespaces(rows, total)
	}
}

func scanNamespaces(rows *sql.Rows, total int64) ([]*Namespace, int64, error) {
	nss := make([]*Namespace, 0)
	for rows.Next() {
		ns := &Namespace{}
		if err := rows.Scan(&ns.ID, &ns.Path, &ns.Name, &ns.Owner, &ns.Type, &ns.Description, &ns.CreatedAt, &ns.UpdatedAt); err != nil {
			return nil, 0, err
		}
		nss = append(nss, ns)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return nss, total, nil
}

func (d *database) NewGroupNamespace(ctx context.Context, ns *Namespace) (*Namespace, error) {
	now := time.Now()
	_, err := d.ExecContext(ctx, "insert into namespaces(path, name, owner_id, type, description, created_at, updated_at) values(?,?,?,?,?,?,?)",
		ns.Path, ns.Name, ns.Owner, 1, ns.Description, now, now)
	if IsDupEntry(err) {
		return nil, &ErrExist{message: "namespace already exists"}
	}
	if err != nil {
		return nil, err
	}
	return d.FindNamespaceByPath(ctx, ns.Path)
}
