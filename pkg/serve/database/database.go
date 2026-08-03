// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
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
	FindOrdinaryReference(ctx context.Context, rid int64, refname plumbing.ReferenceName) (*Reference, error)
	DoBranchUpdate(ctx context.Context, cmd *Command) (*Branch, error)
	DoReferenceUpdate(ctx context.Context, cmd *Command) (*Reference, error)
	Close() error

	// Open API: User operations
	ListUsers(ctx context.Context, page, perPage int) ([]*User, int64, error)
	UpdateUser(ctx context.Context, u *User) (*User, error)
	LockUser(ctx context.Context, uid int64) (*User, error)
	UnlockUser(ctx context.Context, uid int64) (*User, error)
	DeleteUser(ctx context.Context, uid int64) (locked bool, err error)
	SetUserAdministrator(ctx context.Context, uid int64, admin bool) error

	// Open API: Key operations
	ListKeysByUser(ctx context.Context, uid int64) ([]*Key, error)
	DeleteKey(ctx context.Context, id int64) error

	// Open API: Repository operations
	ListRepositories(ctx context.Context, page, perPage int) ([]*Repository, int64, error)
	ListRepositoriesByNamespace(ctx context.Context, namespaceID int64, page, perPage int) ([]*Repository, int64, error)
	UpdateRepository(ctx context.Context, r *Repository) (*Repository, error)
	UpdateRepositoryDefaultBranch(ctx context.Context, rid int64, branch string) (*Repository, error)
	SearchRepositories(ctx context.Context, q string, page, perPage int) ([]*Repository, int64, error)

	// Open API: Branch and Tag operations
	ListBranches(ctx context.Context, rid int64) ([]*Branch, error)
	ListTags(ctx context.Context, rid int64) ([]*Tag, error)

	// Open API: Namespace operations
	ListNamespaces(ctx context.Context, nsType *int, ownerID *int64, page, perPage int) ([]*Namespace, int64, error)
	NewGroupNamespace(ctx context.Context, ns *Namespace) (*Namespace, error)
	DeleteNamespaceWithTransfer(ctx context.Context, srcID, dstID int64) (int64, error)

	// Open API: Member operations
	ListMembers(ctx context.Context, sourceID int64, sourceType MemberType) ([]*Member, error)
	UpdateMember(ctx context.Context, m *Member) error
	RemoveMember(ctx context.Context, id int64) error
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
