// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
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
