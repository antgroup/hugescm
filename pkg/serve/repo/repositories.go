// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/antgroup/hugescm/modules/oss"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/odb"
)

type Repositories interface {
	Open(ctx context.Context, rid int64, compressionAlgo, defaultBranch string) (Repository, error)
	New(ctx context.Context, newRepo *database.Repository, u *database.User, empty bool) (*database.Repository, error)
}

var (
	_ Repositories = &repositories{}
)

type repositories struct {
	root   string
	cdb    odb.CacheDB
	mdb    database.DB
	bucket oss.Bucket
}

func NewRepositories(root string, ossConfig *serve.OSS, cacheConfig *serve.Cache, mdb database.DB) (Repositories, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}

	bucket, err := oss.NewBucket(&oss.NewBucketOptions{
		Endpoint:        ossConfig.Endpoint,
		SharedEndpoint:  ossConfig.SharedEndpoint,
		AccessKeyID:     ossConfig.AccessKeyID,
		AccessKeySecret: ossConfig.AccessKeySecret,
		Bucket:          ossConfig.Bucket,
	})
	if err != nil {
		return nil, err
	}
	cdb, err := odb.NewCacheDB(cacheConfig.NumCounters, cacheConfig.MaxCost, cacheConfig.BufferItems)
	if err != nil {
		return nil, err
	}
	return &repositories{root: root, cdb: cdb, mdb: mdb, bucket: bucket}, nil
}

func (r *repositories) zetaJoin(rid int64) string {
	return fmt.Sprintf("%s/%03d/%d.zeta", r.root, rid%1000, rid)
}

func (r *repositories) Open(ctx context.Context, rid int64, compressionAlgo, defaultBranch string) (Repository, error) {
	repoPath := r.zetaJoin(rid)
	o, err := odb.NewODB(rid, repoPath, compressionAlgo, r.cdb, odb.NewMetadataDB(r.mdb.Database(), rid), r.bucket)
	if err != nil {
		return nil, err
	}
	return &repository{odb: o, mdb: r.mdb, rid: rid, defaultBranch: defaultBranch}, nil
}

func (r *repositories) New(ctx context.Context, newRepo *database.Repository, u *database.User, empty bool) (*database.Repository, error) {
	repo, err := r.mdb.NewRepository(ctx, newRepo)
	if err != nil {
		return nil, err
	}
	if err := r.mdb.AddMember(ctx, &database.Member{
		UID:         u.ID,
		SourceID:    repo.ID,
		SourceType:  database.ProjectMember,
		AccessLevel: database.OwnerAccess,
	}); err != nil {
		return nil, err
	}
	if empty {
		return repo, nil
	}
	rr, err := r.Open(ctx, repo.ID, repo.CompressionAlgo, repo.DefaultBranch)
	if err != nil {
		return nil, err
	}
	defer rr.Close()
	if err := rr.Initialize(ctx, u, repo.DefaultBranch); err != nil {
		return nil, err
	}
	return repo, nil
}

type Repository interface {
	Initialize(ctx context.Context, u *database.User, initBranch string) error
	LsTag(ctx context.Context, tagName string) (string, string, error)
	ParseRev(ctx context.Context, rev string) (*RevObjects, error)
	DoPush(ctx context.Context, cmd *Command, reader io.Reader, w io.Writer) error
	ODB() odb.DB
	Close() error
}

type repository struct {
	mdb           database.DB
	odb           *odb.ODB
	rid           int64
	defaultBranch string
}

func (r *repository) Close() error {
	if r.odb != nil {
		return r.odb.Close()
	}
	return nil
}

func (r *repository) LsTag(ctx context.Context, tagName string) (string, string, error) {
	tag, err := r.mdb.FindTag(ctx, r.rid, tagName)
	if err != nil {
		return "", "", err
	}
	oid := plumbing.NewHash(tag.Hash)
	if to, err := r.odb.Tag(ctx, oid); err == nil {
		return tag.Hash, to.Object.String(), nil
	}
	return tag.Hash, "", nil
}

func (r *repository) ODB() odb.DB {
	return r.odb
}

//go:embed resources
var resourcesFs embed.FS

const (
	zetaIgnore    = "zetaignore"
	dotZetaIgnore = ".zetaignore"
)

func newEntryName(name string) string {
	switch name {
	case zetaIgnore:
		return dotZetaIgnore
	}
	return name
}

func (r *repository) Initialize(ctx context.Context, u *database.User, initBranch string) error {
	// generate trees and blobs
	dirs, err := fs.ReadDir(resourcesFs, "resources")
	if err != nil {
		return err
	}
	hashTo := func(path string) (plumbing.Hash, int64, error) {
		fd, err := resourcesFs.Open(path)
		if err != nil {
			return plumbing.ZeroHash, 0, err
		}
		defer fd.Close()
		si, err := fd.Stat()
		if err != nil {
			return plumbing.ZeroHash, 0, err
		}
		fileSize := si.Size()
		oid, err := r.odb.HashTo(ctx, fd, fileSize)
		return oid, si.Size(), err
	}
	tree := &object.Tree{}
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		si, err := d.Info()
		if err != nil {
			return err
		}
		name := d.Name()
		mode, err := filemode.NewFromOS(si.Mode())
		if err != nil {
			return err
		}
		if si.Size() == 0 {
			tree.Entries = append(tree.Entries, &object.TreeEntry{
				Name: newEntryName(name),
				Mode: mode,
				Hash: plumbing.ZeroHash,
			})
			continue
		}
		oid, fileSize, err := hashTo(path.Join("resources", name))
		if err != nil {
			return fmt.Errorf("hash object %s error: %w", name, err)
		}
		tree.Entries = append(tree.Entries, &object.TreeEntry{
			Name: newEntryName(name),
			Mode: mode,
			Size: fileSize,
			Hash: oid,
		})
	}
	treeOID, err := r.odb.Encode(ctx, tree)
	if err != nil {
		return err
	}
	// generate new commit
	signature := object.Signature{
		Name:  u.UserName,
		Email: u.Email,
		When:  time.Now(),
	}
	commit := &object.Commit{
		Message:   "initialize commit",
		Author:    signature,
		Committer: signature,
		Tree:      treeOID,
	}
	commitOID, err := r.odb.Encode(ctx, commit)
	if err != nil {
		return err
	}
	// create default branch mainline
	if _, err := r.mdb.DoBranchUpdate(ctx, &database.Command{
		ReferenceName: plumbing.NewBranchReferenceName(initBranch),
		OldRev:        plumbing.ZERO_OID,
		NewRev:        commitOID.String(),
		RID:           r.rid,
	}); err != nil {
		return err
	}
	return nil
}
