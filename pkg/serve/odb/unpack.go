// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/antgroup/hugescm/modules/binary"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/backend/pack"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type OStats struct {
	M int
	B int
}

const (
	supportedVersion uint32 = 1
)

var (
	PUSH_STREAM_MAGIC = [4]byte{'Z', 'P', '\x00', '\x01'}
)

type Objects struct {
	Commits     []plumbing.Hash // commits
	Trees       []plumbing.Hash // trees
	MetaObjects []plumbing.Hash // fragments and tags
	Objects     []plumbing.Hash // blobs
	Larges      []plumbing.Hash
}

type Validator func(ctx context.Context, quarantineDir string, o *Objects) error

// Unpack:
//
//	FIXME: CRC64 verification has been temporarily stopped and may need to be restored later.
func (o *ODB) Unpack(ctx context.Context, r io.Reader, ss *OStats, validator Validator) (*Objects, error) {
	now := time.Now()
	incoming := filepath.Join(o.odb.Root(), "incoming")
	if err := os.MkdirAll(incoming, 0755); err != nil {
		return nil, zeta.NewErrStatusCode(http.StatusInternalServerError, "create quarantine dir error: %v", err)
	}
	quarantineDir, err := os.MkdirTemp(incoming, "quarantine-")
	if err != nil {
		return nil, zeta.NewErrStatusCode(http.StatusInternalServerError, "create quarantine dir error: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(quarantineDir)
	}()
	blobDir := filepath.Join(quarantineDir, "blob")
	metadataDir := filepath.Join(quarantineDir, "metadata")

	//r := crc.NewCrc64Reader(reader)
	var magic [4]byte
	var version uint32
	var reserved [16]byte

	if _, err = io.ReadFull(r, magic[:]); err != nil {
		return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "read magic error: %v", err)
	}
	if !bytes.Equal(magic[:], PUSH_STREAM_MAGIC[:]) {
		return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "Bad magic ['\\%x','\\%x','\\%x','\\%x']", magic[0], magic[1], magic[2], magic[3])
	}
	if version, err = binary.ReadUint32(r); err != nil {
		return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "read version error: %v", err)
	}
	if version != supportedVersion {
		return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "unsupported version '%d'", version)
	}
	if _, err := io.ReadFull(r, reserved[:]); err != nil {
		return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "read reserved error: %v", err)
	}
	recvObjects := &Objects{
		Commits:     make([]plumbing.Hash, 0, 10),
		Trees:       make([]plumbing.Hash, 0, 100),
		MetaObjects: make([]plumbing.Hash, 0, 10),
		Objects:     make([]plumbing.Hash, 0, 100),
	}
	u, err := NewUnpackers(ss, metadataDir, blobDir)
	if err != nil {
		return nil, zeta.NewErrStatusCode(http.StatusInternalServerError, "new unpacker error: %v", err)
	}
	var unpackerClosed bool
	defer func() {
		if !unpackerClosed {
			_ = u.Close()
		}
	}()
	for {
		objectSize, err := binary.ReadUint64(r)
		if err != nil {
			return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "read object length error: %v", err)
		}
		size := int64(objectSize)
		if size == 0 {
			break
		}
		metadata := false
		if size < 0 {
			metadata = true
			size = -size
		}
		if size < 64 {
			return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "bad chunk size: %d", size)
		}
		var hashBytes [plumbing.HASH_HEX_SIZE]byte
		if _, err = io.ReadFull(r, hashBytes[:]); err != nil {
			return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "read object hash error: %v", err)
		}
		oid := plumbing.NewHash(string(hashBytes[:]))
		currentSize := size - plumbing.HASH_HEX_SIZE
		reader := io.LimitReader(r, currentSize) // object reader
		if !metadata {
			if _, err := u.WriteTo(oid, reader, uint32(currentSize)); err != nil {
				return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "decode blob error: %v", err)
			}
			if size > LargeSize {
				recvObjects.Larges = append(recvObjects.Larges, oid)
			}
			recvObjects.Objects = append(recvObjects.Objects, oid)
			continue
		}
		t, err := u.mu.Unpack(oid, reader, uint32(currentSize), true)
		if err != nil {
			return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "decode metadata error: %v", err)
		}
		switch t {
		case object.CommitObject:
			recvObjects.Commits = append(recvObjects.Commits, oid)
		case object.TreeObject:
			recvObjects.Trees = append(recvObjects.Trees, oid)
		default:
			recvObjects.MetaObjects = append(recvObjects.MetaObjects, oid)
		}
	}
	// if err := r.Verify(); err != nil {
	// 	return nil, zeta.NewErrStatusCode(http.StatusBadRequest, "verify crc64 error: %v", err)
	// }

	if err := u.Close(); err != nil {
		unpackerClosed = true
		return nil, err
	}
	logrus.Infof("[RID-%d] objects unpacking consumption: %v", o.rid, time.Since(now))
	if err := validator(ctx, quarantineDir, recvObjects); err != nil {
		return nil, err
	}

	if err := o.moveFromQuarantineDir(quarantineDir); err != nil {
		return nil, err
	}
	return recvObjects, nil
}

// /home/zeta/repositories/a.zeta/tmp/quarantine-XXXX/metadata/8d/8d0607257a2ee5a4c85d287c70900c14c2380f55cd49179f2db6228edee7db25
// /home/zeta/repositories/a.zeta/metadata/8d/8d0607257a2ee5a4c85d287c70900c14c2380f55cd49179f2db6228edee7db25
func (o *ODB) moveFromQuarantineDir(quarantineDir string) error {
	return filepath.WalkDir(quarantineDir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(quarantineDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(o.odb.Root(), rel)
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.Rename(path, target)
	})
}

type Unpacker interface {
	Unpack(oid plumbing.Hash, r io.Reader, size uint32, metadata bool) (object.ObjectType, error)
	Close() error
}

type looseUnpacker struct {
	root string
}

var (
	_ Unpacker = &looseUnpacker{}
)

func (u *looseUnpacker) Unpack(oid plumbing.Hash, r io.Reader, size uint32, metadata bool) (t object.ObjectType, err error) {
	saveTo := backend.Join(u.root, oid)
	if err = os.MkdirAll(filepath.Dir(saveTo), 0755); err != nil {
		return
	}
	var fd *os.File

	if fd, err = os.Create(saveTo); err != nil {
		return
	}
	defer fd.Close() // nolint
	var got plumbing.Hash
	tr := io.TeeReader(r, fd)
	if metadata {
		got, t, err = object.HashObject(tr)
	} else {
		got, err = object.HashFrom(tr)
	}
	if err != nil {
		return
	}
	if got != oid {
		return t, fmt.Errorf("unexpected metadata oid got '%s' want '%s'", got, oid)
	}
	return t, nil
}

func (u *looseUnpacker) Close() error {
	return nil
}

type packedUnpacker struct {
	*pack.Writer
	modification int64
}

// root: /home/zeta/repositories/001/10001.zeta/incoming/quarantine-1111/metadata
func NewPackedUnpacker(root string) (*packedUnpacker, error) {
	w, err := pack.NewWriter(filepath.Join(root, "pack"), 0)
	if err != nil {
		return nil, err
	}
	return &packedUnpacker{
		Writer:       w,
		modification: time.Now().Unix(),
	}, nil
}

func (u *packedUnpacker) Unpack(oid plumbing.Hash, r io.Reader, size uint32, metadata bool) (object.ObjectType, error) {
	pr, pw := io.Pipe()
	var got plumbing.Hash
	var t object.ObjectType
	var g errgroup.Group
	g.Go(func() error {
		var err error
		if metadata {
			if got, t, err = object.HashObject(pr); err != nil {
				_ = pr.CloseWithError(err)
				return err
			}
			_ = pr.Close()
			return nil
		}
		if got, err = object.HashFrom(pr); err != nil {
			_ = pr.CloseWithError(err)
			return err
		}
		_ = pr.Close()
		return nil
	})
	g.Go(func() error {
		if err := u.Write(oid, size, io.TeeReader(r, pw), u.modification); err != nil {
			_ = pw.CloseWithError(err)
			return err
		}
		_ = pw.Close()
		return nil
	})
	if err := g.Wait(); err != nil {
		return t, err
	}
	if got != oid {
		return t, fmt.Errorf("unexpected blob oid got '%s' want '%s'", got, oid)
	}
	return t, nil
}

func (u *packedUnpacker) Close() error {
	if err := u.WriteTrailer(); err != nil {
		return err
	}
	return nil
}

type Unpackers struct {
	mu Unpacker
	bu Unpacker
	bo Unpacker
}

func (u *Unpackers) Close() error {
	_ = u.mu.Close()
	_ = u.bu.Close()
	_ = u.bo.Close()
	return nil
}

func NewUnpackers(s *OStats, metadataRoot, blobRoot string) (*Unpackers, error) {
	var err error
	var m, b Unpacker
	if s.M > 2000 {
		if m, err = NewPackedUnpacker(metadataRoot); err != nil {
			return nil, err
		}
	} else {
		m = &looseUnpacker{root: metadataRoot}
	}
	if s.B > 2000 {
		if b, err = NewPackedUnpacker(blobRoot); err != nil {
			_ = m.Close()
			return nil, err
		}
	} else {
		b = &looseUnpacker{root: blobRoot}
	}
	return &Unpackers{mu: m, bu: b, bo: &looseUnpacker{root: blobRoot}}, nil
}

const (
	LargeSize = 5 << 20 // 5M
)

func (u *Unpackers) WriteTo(oid plumbing.Hash, r io.Reader, size uint32) (object.ObjectType, error) {
	if size > LargeSize {
		// LOOSE objects
		return u.bo.Unpack(oid, r, size, false)
	}
	// packed objects
	return u.bu.Unpack(oid, r, size, false)
}

func (o *ODB) NewUnpacker(entries uint32, metadata bool) (*backend.Unpacker, error) {
	return o.odb.NewUnpacker(entries, metadata)
}
