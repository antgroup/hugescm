// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type Storage interface {
	// Open returns a handle on an existing object keyed by the given object
	// ID.  It returns an error if that file does not already exist.
	Open(oid plumbing.Hash) (f io.ReadCloser, err error)
	//
	Exists(name plumbing.Hash) error
	//
	Search(prefix plumbing.Hash) (plumbing.Hash, error)
	// Close closes the filesystem, after which no more operations are
	// allowed.
	Close() error
}

type WritableStorage interface {
	Storage
	HashTo(ctx context.Context, r io.Reader, size int64) (oid plumbing.Hash, err error)
	Unpack(oid plumbing.Hash, r io.Reader) (err error)
	WriteEncoded(e object.Encoder) (oid plumbing.Hash, err error)
	LooseObjects() ([]plumbing.Hash, error)
	PruneObject(ctx context.Context, oid plumbing.Hash) error
	PruneObjects(ctx context.Context, largeSize int64) ([]plumbing.Hash, int64, error)
}

// Storage implements an interface for reading, but not writing, objects in an
// object database.
type multiStorage struct {
	impls []Storage
}

func MultiStorage(args ...Storage) Storage {
	return &multiStorage{impls: args}
}

// Open returns a handle on an existing object keyed by the given object
// ID.  It returns an error if that file does not already exist.
func (m *multiStorage) Open(oid plumbing.Hash) (f io.ReadCloser, err error) {
	for _, s := range m.impls {
		f, err := s.Open(oid)
		if err != nil {
			if plumbing.IsNoSuchObject(err) {
				continue
			}
			return nil, err
		}
		return f, nil
	}
	return nil, plumbing.NoSuchObject(oid)
}

func (m *multiStorage) Exists(oid plumbing.Hash) error {
	for _, s := range m.impls {
		if err := s.Exists(oid); err != nil {
			if plumbing.IsNoSuchObject(err) {
				continue
			}
			return err
		}
		return nil
	}
	return plumbing.NoSuchObject(oid)
}

func (m *multiStorage) Search(prefix plumbing.Hash) (plumbing.Hash, error) {
	for _, s := range m.impls {
		oid, err := s.Search(prefix)
		if err != nil {
			if plumbing.IsNoSuchObject(err) {
				continue
			}
			return oid, err
		}
		return oid, nil
	}
	return plumbing.ZeroHash, plumbing.NoSuchObject(prefix)
}

// Close closes the filesystem, after which no more operations are
// allowed.
func (m *multiStorage) Close() error {
	for _, s := range m.impls {
		if err := s.Close(); err != nil {
			return err
		}
	}
	return nil
}
