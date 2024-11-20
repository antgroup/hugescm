// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package pack

import (
	"io"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// Storage implements the storage.Storage interface.
type Storage struct {
	packs Set
}

// NewStorage returns a new storage object based on a pack set.
func NewStorage(root string) (*Storage, error) {
	packs, err := NewSets(root)
	if err != nil {
		return nil, err
	}
	return &Storage{packs: packs}, nil
}

// Open implements the storage.Storage.Open interface.
func (f *Storage) Open(oid plumbing.Hash) (r io.ReadCloser, err error) {
	return f.packs.Object(oid)
}

// check object exists
func (f *Storage) Exists(name plumbing.Hash) error {
	return f.packs.Exists(name)
}

func (f *Storage) Search(prefix plumbing.Hash) (oid plumbing.Hash, err error) {
	return f.packs.Search(prefix)
}

// Open implements the storage.Storage.Open interface.
func (f *Storage) Close() error {
	return f.packs.Close()
}

type Scanner struct {
	set   Set
	packs Packs
}

func NewScanner(root string) (*Scanner, error) {
	set, packs, err := NewPacks(root)
	if err != nil {
		return nil, err
	}
	return &Scanner{set: set, packs: packs}, nil
}

// Open implements the storage.Storage.Open interface.
func (s *Scanner) Open(oid plumbing.Hash) (r io.ReadCloser, err error) {
	return s.set.Object(oid)
}

func (s *Scanner) PackedObjects(recv RecvFunc) error {
	return s.packs.PackedObjects(recv)
}

// check object exists
func (s *Scanner) Exists(name plumbing.Hash) error {
	for _, p := range s.packs {
		if err := p.Exists(name); err != nil {
			if plumbing.IsNoSuchObject(err) {
				continue
			}
			return err
		}
		return nil
	}
	return plumbing.NoSuchObject(name)
}

func (s *Scanner) Search(prefix plumbing.Hash) (plumbing.Hash, error) {
	for _, p := range s.packs {
		oid, err := p.Search(prefix)
		if err != nil {
			if plumbing.IsNoSuchObject(err) {
				continue
			}
			return plumbing.ZeroHash, err
		}
		return oid, nil
	}
	return plumbing.ZeroHash, plumbing.NoSuchObject(prefix)
}

func (s *Scanner) Names() []string {
	names := make([]string, 0, len(s.packs))
	for _, p := range s.packs {
		if fd, ok := p.r.(*os.File); ok {
			names = append(names, fd.Name())
		}
	}
	return names
}

// Open implements the storage.Storage.Open interface.
func (s *Scanner) Close() error {
	return s.set.Close()
}
