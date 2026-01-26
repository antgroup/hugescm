// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package pack

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// Packfile encapsulates the behavior of accessing an unpacked representation of
// all of the objects encoded in a single packfile.
type Packfile struct {
	// Version is the version of the packfile.
	Version uint32
	// Objects is the total number of objects in the packfile.
	Objects uint32
	// idx is the corresponding "pack-*.idx" file giving the positions of
	// objects in this packfile.
	idx *Index

	// r is an io.ReaderAt that allows read access to the packfile itself.
	r io.ReaderAt
}

// Close closes the packfile if the underlying data stream is closeable. If so,
// it returns any error involved in closing.
func (p *Packfile) Close() error {
	var iErr error
	if p.idx != nil {
		iErr = p.idx.Close()
	}

	if close, ok := p.r.(io.Closer); ok {
		return close.Close()
	}
	return iErr
}

func (p *Packfile) Exists(name plumbing.Hash) error {
	if _, err := p.idx.Entry(name); err != nil {
		if !IsNotFound(err) {
			// If the error was not an errNotFound, re-wrap it with
			// additional context.
			err = fmt.Errorf("zeta: could not load index: %s", err)
		}
		return err
	}
	return nil
}

func (p *Packfile) Search(name plumbing.Hash) (oid plumbing.Hash, err error) {
	return p.idx.Search(name)
}

func (p *Packfile) Object(name plumbing.Hash) (*SizeReader, error) {
	// First, try and determine the offset of the last entry in the
	// delta-base chain by loading it from the corresponding pack index.
	entry, err := p.idx.Entry(name)
	if err != nil {
		if !IsNotFound(err) {
			// If the error was not an errNotFound, re-wrap it with
			// additional context.
			err = fmt.Errorf("zeta: could not load index: %s", err)
		}
		return nil, err
	}
	return p.find(int64(entry.PackOffset))
}

func (p *Packfile) find(offset int64) (*SizeReader, error) {
	var sizeBytes [4]byte
	if _, err := p.r.ReadAt(sizeBytes[:], offset); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(sizeBytes[:])
	return NewSizeReader(p.r, offset+4, int64(size)), nil
}

// DecodePackfile opens the packfile given by the io.ReaderAt "r" for reading.
// It does not apply any delta-base chains, nor does it do reading otherwise
// beyond the header.
//
// If the header is malformed, or otherwise cannot be read, an error will be
// returned without a corresponding packfile.
func DecodePackfile(r io.ReaderAt) (*Packfile, error) {
	header := make([]byte, 12)
	if _, err := r.ReadAt(header, 0); err != nil {
		return nil, err
	}

	if !bytes.Equal(header[0:4], packMagic[:]) {
		return nil, errBadPackHeader
	}

	version := binary.BigEndian.Uint32(header[4:])
	objects := binary.BigEndian.Uint32(header[8:])

	return &Packfile{
		Version: version,
		Objects: objects,

		r: r,
	}, nil
}
