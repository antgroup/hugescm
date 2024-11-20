// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found")
	ErrReferenceNotExist  = errors.New("reference not exist")
)

type SizeReader interface {
	io.Reader
	io.Closer
	Offset() int64
	Size() int64
	LastError() error
}

type MetadataOptions struct {
	Sparses    []string
	DeepenFrom plumbing.Hash
	Have       plumbing.Hash
	Deepen     int
	Depth      int
}

type SessionReader interface {
	io.Reader
	io.Closer
	LastError() error
}

type Transport interface {
	// FetchReference: discover reference and remote repo info and caps
	FetchReference(ctx context.Context, refname plumbing.ReferenceName) (*Reference, error)
	// FetchMetadata: support base metadata and sparses metadata.
	//  target: commit or tag
	FetchMetadata(ctx context.Context, target plumbing.Hash, opts *MetadataOptions) (SessionReader, error)
	// BatchObjects: batch download objects AKA blobs
	BatchObjects(ctx context.Context, oids []plumbing.Hash) (SessionReader, error)
	// GetObject: get large object, support Range feature
	GetObject(ctx context.Context, oid plumbing.Hash, fromByte int64) (SizeReader, error)
	// Shared: get large objects shared links
	Shared(ctx context.Context, wantObjects []*WantObject) ([]*Representation, error)
	// Push: push metadata and blobs to remote and update reference
	Push(ctx context.Context, r io.Reader, cmd *Command) (rc SessionReader, err error)
	// BatchCheck: check large objects exists in remote
	BatchCheck(ctx context.Context, refname plumbing.ReferenceName, haveObjects []*HaveObject) ([]*HaveObject, error)
	// PutObject: upload large object to remote
	PutObject(ctx context.Context, refname plumbing.ReferenceName, oid plumbing.Hash, r io.Reader, size int64) error
}
