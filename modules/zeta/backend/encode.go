// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func (d *Database) WriteEncoded(e object.Encoder) (oid plumbing.Hash, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.metaRW.WriteEncoded(e)
}

// HashTo:
//
//	size == -1: unknown file size, need to detect file size.
//	size == 0: empty file, returns the specified BLOB.
//	size > 0: the file size is known.
func (d *Database) HashTo(ctx context.Context, r io.Reader, size int64) (oid plumbing.Hash, err error) {
	if size == 0 {
		return BLANK_BLOB_HASH, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.rw.HashTo(ctx, r, size)
}

func (d *Database) WriteTo(ctx context.Context, oid plumbing.Hash, r io.Reader) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.rw.Unpack(oid, r)
}

func (d *Database) JoinPart(oid plumbing.Hash) string {
	name := oid.String() + ".part"
	if len(d.sharingRoot) != 0 {
		return filepath.Join(d.sharingRoot, "incoming", name)
	}
	return filepath.Join(d.root, "incoming", name)
}

func (d *Database) ValidatePart(saveTo string, oid plumbing.Hash) error {
	fd, err := os.Open(saveTo)
	if err != nil {
		return err
	}
	return d.ValidateFD(fd, oid)
}

func (d *Database) newPartName(oid plumbing.Hash) string {
	encoded := oid.String()
	partName := encoded + ".part"
	if len(d.sharingRoot) != 0 {
		return filepath.Join(d.sharingRoot, "incoming", partName)
	}
	return filepath.Join(d.root, "incoming", partName)
}

func (d *Database) encodedPath(oid plumbing.Hash) string {
	encoded := oid.String()
	if len(d.sharingRoot) != 0 {
		return filepath.Join(d.sharingRoot, "blob", encoded[:2], encoded[2:4], encoded)
	}
	return filepath.Join(d.root, "blob", encoded[:2], encoded[2:4], encoded)
}

// NewFD: new file fd
func (d *Database) NewFD(oid plumbing.Hash) (*os.File, error) {
	return os.OpenFile(d.newPartName(oid), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
}

func (d *Database) NewTruncateFD(oid plumbing.Hash) (*os.File, error) {
	return os.OpenFile(d.newPartName(oid), os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
}

func (d *Database) validateFD(fd *os.File, oid plumbing.Hash) error {
	defer fd.Close() // nolint
	if _, err := fd.Seek(0, io.SeekStart); err != nil {
		return err
	}
	b, err := object.NewBlob(io.NopCloser(fd))
	if err != nil {
		return err
	}
	h := plumbing.NewHasher()
	if _, err := io.Copy(h, b.Contents); err != nil {
		return err
	}
	if s := h.Sum(); s != oid {
		return fmt.Errorf("bad blob oid: want '%s' got '%s'", oid, s)
	}
	_ = fd.Chmod(0444) // Set blob to read-only
	return nil
}

func (d *Database) ValidateFD(fd *os.File, oid plumbing.Hash) error {
	saveTo := d.encodedPath(oid)
	name := fd.Name()
	if err := d.validateFD(fd, oid); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(saveTo), 0755); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := finalizeObject(name, saveTo); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
