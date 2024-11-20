// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
)

type CompressMethod uint16

const (
	BLOB_CURRENT_VERSION  uint16         = 1
	BLOB_CACHE_SIZE_LIMIT                = 1024 * 1024
	STORE                 CompressMethod = 0
	ZSTD                  CompressMethod = 1
	BROTLI                CompressMethod = 2
	DEFLATE               CompressMethod = 3
	XZ                    CompressMethod = 4
	BZ2                   CompressMethod = 5
)

var (
	BLOB_MAGIC       = [4]byte{'Z', 'B', 0x00, 0x01}
	BLANK_BLOB_BYTES = [16]byte{'Z', 'B', 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
)

var (
	ErrMismatchedMagic   = errors.New("mismatched magic")
	ErrMismatchedVersion = errors.New("mismatched version")
)

type Blob struct {
	Contents io.Reader
	Size     int64
	closeFn  func() error
}

func (b *Blob) Close() error {
	if b.closeFn == nil {
		return nil
	}
	return b.closeFn()
}

func NewBlob(raw io.ReadCloser) (*Blob, error) {
	var hdr [16]byte
	if _, err := io.ReadFull(raw, hdr[:]); err != nil {
		return nil, err
	}
	if !bytes.Equal(BLOB_MAGIC[:], hdr[:4]) {
		return nil, ErrMismatchedMagic
	}
	if version := binary.BigEndian.Uint16(hdr[4:6]); version != BLOB_CURRENT_VERSION {
		return nil, ErrMismatchedVersion
	}
	method := CompressMethod(binary.BigEndian.Uint16(hdr[6:8]))
	uncompressedSize := int64(binary.BigEndian.Uint64(hdr[8:16]))
	switch method {
	case STORE:
		return &Blob{Contents: raw, Size: uncompressedSize, closeFn: func() error {
			return raw.Close()
		}}, nil
	case ZSTD:
		zr, err := streamio.GetZstdReader(raw)
		if err != nil {
			return nil, fmt.Errorf("unable new zstd decoder: %v", err)
		}
		return &Blob{Contents: zr, Size: uncompressedSize, closeFn: func() error {
			streamio.PutZstdReader(zr)
			return raw.Close()
		}}, nil
	case DEFLATE:
		zr, err := streamio.GetZlibReader(raw)
		if err != nil {
			return nil, fmt.Errorf("unable new zlib decoder: %v", err)
		}
		return &Blob{Contents: zr.Reader, Size: uncompressedSize, closeFn: func() error {
			streamio.PutZlibReader(zr)
			return raw.Close()
		}}, nil
	}
	return nil, fmt.Errorf("unsupported method: '%d'", method)
}

func HashFrom(r io.Reader) (plumbing.Hash, error) {
	br, err := NewBlob(io.NopCloser(r))
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer br.Close()
	hasher := plumbing.NewHasher()
	if _, err := io.Copy(hasher, br.Contents); err != nil {
		return plumbing.ZeroHash, err
	}
	return hasher.Sum(), nil
}
