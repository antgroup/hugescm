// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

type DB interface {
	Database() *sql.DB
	FindUser(ctx context.Context, uid int64) (*User, error)
	SearchUser(ctx context.Context, emailOrName string) (*User, error)
	SearchKey(ctx context.Context, fingerprint string) (*Key, error)
	NewUser(ctx context.Context, u *User) (*User, error)
	AddMember(ctx context.Context, m *Member) error
	FindKey(ctx context.Context, id int64) (*Key, error)
	AddKey(ctx context.Context, k *Key) (*Key, error)
	IsDeployKeyEnabled(ctx context.Context, rid int64, kid int64) (bool, error)
	FindNamespaceByID(ctx context.Context, namespaceID int64) (*Namespace, error)
	FindNamespaceByPath(ctx context.Context, namespacePath string) (*Namespace, error)
	FindRepositoryByID(ctx context.Context, rid int) (*Namespace, *Repository, error)
	FindRepositoryByPath(ctx context.Context, namespacePath, repoPath string) (*Namespace, *Repository, error)
	NewRepository(ctx context.Context, r *Repository) (*Repository, error)
	RepoAccessLevel(ctx context.Context, r *Repository, u *User) (AccessLevel, AccessLevel, error)
	FindBranchForPrefix(ctx context.Context, rid int64, prefix string) (*Branch, error)
	FindTagForPrefix(ctx context.Context, rid int64, prefix string) (*Tag, error)
	FindBranch(ctx context.Context, rid int64, branchName string) (*Branch, error)
	FindTag(ctx context.Context, rid int64, tagName string) (*Tag, error)
	DoBranchUpdate(ctx context.Context, cmd *Command) (*Branch, error)
	DoReferenceUpdate(ctx context.Context, cmd *Command) (*Reference, error)
	Close() error
}

type database struct {
	*sql.DB
}

func (d *database) Database() *sql.DB {
	return d.DB
}

func (d *database) Close() error {
	return d.DB.Close()
}

var (
	_ DB = &database{}
)

func NewDB(cfg *mysql.Config) (DB, error) {
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("new connector: %w", err)
	}

	db := sql.OpenDB(connector)
	db.SetMaxIdleConns(25)
	db.SetMaxOpenConns(50)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &database{DB: db}, nil
}
