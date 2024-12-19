// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/streamio"
)

type File struct {
	// Name is the path of the file. It might be relative to a tree,
	// depending of the function that generates it.
	Name string
	// path
	Path string
	// Mode is the file mode.
	Mode filemode.FileMode
	// Hash of the blob.
	Hash plumbing.Hash
	// Size of the (uncompressed) blob.
	Size int64
	b    Backend
}

func newFile(name string, p string, m filemode.FileMode, hash plumbing.Hash, size int64, b Backend) *File {
	return &File{Name: name, Path: p, Mode: m, Hash: hash, Size: size, b: b}
}

type readCloser struct {
	io.Reader
	io.Closer
}

func (f *File) IsFragments() bool {
	if f == nil {
		return false
	}
	return f.Mode.IsFragments()
}

func (f *File) asFile() *diferenco.File {
	if f == nil {
		return nil
	}
	return &diferenco.File{Name: f.Path, Hash: f.Hash.String(), Mode: uint32(f.Mode.Origin())}
}

// OriginReader return ReadCloser
func (f *File) OriginReader(ctx context.Context) (io.ReadCloser, int64, error) {
	if f.b == nil {
		return nil, 0, io.ErrUnexpectedEOF
	}
	br, err := f.b.Blob(ctx, f.Hash)
	if err != nil {
		return nil, 0, err
	}
	return &readCloser{Reader: br.Contents, Closer: br}, br.Size, nil
}

const (
	sniffLen = 8000
)

func (f *File) Reader(ctx context.Context) (io.ReadCloser, bool, error) {
	if f.b == nil {
		return nil, false, io.ErrUnexpectedEOF
	}
	br, err := f.b.Blob(ctx, f.Hash)
	if err != nil {
		return nil, false, err
	}
	sniffBytes, err := streamio.ReadMax(br.Contents, sniffLen)
	if err != nil {
		_ = br.Close()
		return nil, false, err
	}
	bin := bytes.IndexByte(sniffBytes, 0) != -1
	return &readCloser{Reader: io.MultiReader(bytes.NewReader(sniffBytes), br.Contents), Closer: br}, bin, nil
}

func (f *File) UnifiedText(ctx context.Context, codecvt bool) (content string, err error) {
	if f == nil {
		// NO CONTENT DELETE OR NEWFILE
		return "", nil
	}
	r, _, err := f.OriginReader(ctx)
	if err != nil {
		return "", err
	}
	defer r.Close()
	content, _, err = diferenco.ReadUnifiedText(r, f.Size, codecvt)
	return content, err
}

// FileIter provides an iterator for the files in a tree.
type FileIter struct {
	b Backend
	w *TreeWalker
}

// NewFileIter takes a Backend and a Tree and returns a
// *FileIter that iterates over all files contained in the tree, recursively.
func NewFileIter(b Backend, t *Tree) *FileIter {
	return &FileIter{b: b, w: NewTreeWalker(t, true, nil)}
}

// Next moves the iterator to the next file and returns a pointer to it. If
// there are no more files, it returns io.EOF.
func (iter *FileIter) Next(ctx context.Context) (*File, error) {
	for {
		name, entry, err := iter.w.Next(ctx)
		if err != nil {
			return nil, err
		}

		if entry.Mode == filemode.Dir || entry.Mode == filemode.Submodule || entry.IsFragments() {
			continue
		}

		return newFile(name, "", entry.Mode, entry.Hash, entry.Size, iter.b), nil
	}
}

// ForEach call the cb function for each file contained in this iter until
// an error happens or the end of the iter is reached. If plumbing.ErrStop is sent
// the iteration is stop but no error is returned. The iterator is closed.
func (iter *FileIter) ForEach(ctx context.Context, cb func(*File) error) error {
	defer iter.Close()

	for {
		f, err := iter.Next(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if err := cb(f); err != nil {
			if err == plumbing.ErrStop {
				return nil
			}

			return err
		}
	}
}

func (iter *FileIter) Close() {
	iter.w.Close()
}
