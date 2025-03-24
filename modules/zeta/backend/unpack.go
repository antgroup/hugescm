// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/zeta/backend/pack"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type Unpacker struct {
	*pack.Writer
	root           string
	quarantineDir  string
	selectedMethod CompressMethod
}

func (u *Unpacker) method(compressed bool) CompressMethod {
	if compressed {
		return STORE
	}
	return u.selectedMethod
}

func (u *Unpacker) HashTo(r io.Reader, size int64, modification int64) (oid plumbing.Hash, err error) {
	payload, err := streamio.ReadMax(r, mimePacketSize)
	if err != nil && err != io.EOF {
		return oid, fmt.Errorf("ReadFull error: %v", err)
	}
	compressed := isBinaryPayload(payload)
	var contents io.Reader = bytes.NewReader(payload)
	if err != io.EOF {
		contents = io.MultiReader(contents, r)
	}
	hasher := plumbing.NewHasher()
	buffer := streamio.GetBytesBuffer()
	defer streamio.PutBytesBuffer(buffer)
	// 4 byte magic
	if _, err = buffer.Write(BLOB_MAGIC[:]); err != nil {
		return
	}
	// 2 byte version
	if err = binary.Write(buffer, binary.BigEndian, DEFAULT_BLOB_VERSION); err != nil {
		return
	}
	// 2 byte method
	method := u.method(compressed)
	if err = binary.Write(buffer, binary.BigEndian, method); err != nil {
		return
	}
	// 8 byte uncompressed length
	if err = binary.Write(buffer, binary.BigEndian, size); err != nil {
		return
	}
	var written int64
	if written, err = compress(io.TeeReader(contents, hasher), buffer, method); err != nil {
		return
	}
	if size != written {
		return oid, fmt.Errorf("blob size not match expected, actual size %d, expected size %d", written, size)
	}
	oid = hasher.Sum()
	encBytes := buffer.Bytes()
	if err = u.Write(oid, uint32(len(encBytes)), bytes.NewReader(encBytes), modification); err != nil {
		return
	}
	return
}

func (u *Unpacker) WriteEncoded(e object.Encoder, squeeze bool, modification int64) (plumbing.Hash, error) {
	buffer := streamio.GetBytesBuffer()
	defer streamio.PutBytesBuffer(buffer)
	hasher := plumbing.NewHasher()
	if squeeze {
		zw := streamio.GetZstdWriter(buffer)
		if err := e.Encode(io.MultiWriter(zw, hasher)); err != nil {
			streamio.PutZstdWriter(zw)
			return plumbing.ZeroHash, err
		}
		streamio.PutZstdWriter(zw) // MUST CLOSE ZSTD WRITER
	} else {
		if err := e.Encode(io.MultiWriter(buffer, hasher)); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	oid := hasher.Sum()
	data := buffer.Bytes()
	if err := u.Write(oid, uint32(len(data)), bytes.NewReader(data), modification); err != nil {
		return oid, err
	}
	return oid, nil
}

func (u *Unpacker) Close() error {
	if u.Writer == nil {
		return nil
	}
	err := u.Writer.Close()
	if len(u.quarantineDir) != 0 {
		_ = os.RemoveAll(u.quarantineDir)
	}
	return err
}

func (d *Database) NewUnpackerEx(entries uint32, metadata bool, method CompressMethod) (*Unpacker, error) {
	var root, incoming string
	switch {
	case metadata:
		root = filepath.Join(d.root, "metadata")
		incoming = filepath.Join(d.root, "incoming")
	case len(d.sharingRoot) != 0:
		root = filepath.Join(d.sharingRoot, "blob")
		incoming = filepath.Join(d.sharingRoot, "incoming")
	default:
		root = filepath.Join(d.root, "blob")
		incoming = filepath.Join(d.root, "incoming")
	}
	quarantineDir, err := os.MkdirTemp(incoming, "quarantine-")
	if err != nil {
		return nil, err
	}
	w, err := pack.NewWriter(quarantineDir, entries)
	if err != nil {
		_ = os.RemoveAll(quarantineDir)
		return nil, err
	}
	return &Unpacker{Writer: w, root: root, quarantineDir: quarantineDir, selectedMethod: method}, nil
}

func (d *Database) NewUnpacker(entries uint32, metadata bool) (*Unpacker, error) {
	return d.NewUnpackerEx(entries, metadata, fromCompressionALGO(d.compressionALGO))
}

func (u *Unpacker) Preserve() error {
	if err := u.WriteTrailer(); err != nil {
		return err
	}
	return preservePack(u.root, u.quarantineDir)
}
