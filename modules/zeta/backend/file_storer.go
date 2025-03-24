// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/mime"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta/backend/storage"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

const (
	mimePacketSize              = 4096
	DEFAULT_BLOB_VERSION uint16 = 1
)

var (
	BLOB_MAGIC = [4]byte{'Z', 'B', 0x00, 0x01}
)

type CompressMethod uint16

const (
	STORE   CompressMethod = 0
	ZSTD    CompressMethod = 1
	BROTLI  CompressMethod = 2
	DEFLATE CompressMethod = 3
	XZ      CompressMethod = 4
	BZ2     CompressMethod = 5
)

func fromCompressionALGO(compressionALGO string) CompressMethod {
	switch strings.ToLower(compressionALGO) {
	case "zlib", "deflate":
		return DEFLATE
	case "xz":
		return XZ
	case "bz2":
		return BZ2
	case "brotli":
		return BROTLI
	default: // zstd
	}
	return ZSTD
}

func isBinaryPayload(payload []byte) bool {
	result := mime.DetectAny(payload)
	for p := result; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return false
		}
	}
	return true
}

// fileStorer implements the storer interface by writing to the .git/objects
// directory on disc.
type fileStorer struct {
	// root is the top level /objects directory's path on disc.
	root string

	// temp directory, defaults to os.TempDir
	incoming       string
	selectedMethod CompressMethod
}

var (
	_ storage.Storage = &fileStorer{}
)

// NewFileStorer returns a new fileStorer instance with the given root.
func newFileStorer(root, incoming, compressionALGO string) *fileStorer {
	return &fileStorer{
		root:           root,
		incoming:       incoming,
		selectedMethod: fromCompressionALGO(compressionALGO),
	}
}

func Join(root string, oid plumbing.Hash) string {
	encoded := oid.String()
	return filepath.Join(root, encoded[:2], encoded[2:4], encoded)
}

// path returns an absolute path on disk to the object given by the OID "sha".
func (so *fileStorer) path(oid plumbing.Hash) string {
	encoded := oid.String()
	return filepath.Join(so.root, encoded[:2], encoded[2:4], encoded)
}

// Open implements the storer.Open function, and returns a io.ReadCloser
// for the given SHA. If the file does not exist, or if there was any other
// error in opening the file, an error will be returned.
//
// It is the caller's responsibility to close the given file "f" after its use
// is complete.
func (so *fileStorer) Open(oid plumbing.Hash) (f io.ReadCloser, err error) {
	f, err = so.open(so.path(oid), os.O_RDONLY)
	if os.IsNotExist(err) {
		return nil, plumbing.NoSuchObject(oid)
	}
	return f, err
}

func (so *fileStorer) Exists(oid plumbing.Hash) error {
	p := so.path(oid)
	if _, err := os.Stat(p); err != nil && os.IsNotExist(err) {
		return plumbing.NoSuchObject(oid)
	}
	return nil
}

// Root gives the absolute (fully-qualified) path to the file storer on disk.
func (so *fileStorer) Root() string {
	return so.root
}

// Close closes the file storer.
func (so *fileStorer) Close() error {
	return nil
}

// open opens a given file.
func (so *fileStorer) open(path string, flag int) (*os.File, error) {
	return os.OpenFile(path, flag, 0)
}

func (so *fileStorer) method(compressed bool) CompressMethod {
	if compressed {
		return STORE
	}
	return so.selectedMethod
}

type ExtendWriter interface {
	io.ReaderFrom
	io.Writer
}

func compress(r io.Reader, w ExtendWriter, method CompressMethod) (written int64, err error) {
	switch method {
	case STORE:
		return w.ReadFrom(r)
	case ZSTD:
		zw := streamio.GetZstdWriter(w)
		defer streamio.PutZstdWriter(zw)
		return zw.ReadFrom(r)
	case DEFLATE:
		zw := streamio.GetZlibWriter(w)
		defer streamio.PutZlibWriter(zw)
		return io.Copy(zw, r)
	default:
		return 0, fmt.Errorf("unsupported method: %d", method)
	}
}

// hashToInternal: write reader to disk
// 4 byte magic
// 2 byte version
// 2 byte method
// 8 byte uncompressed length
// N bytes raw or compressed data
func (so *fileStorer) hashToInternal(fd *os.File, r io.Reader, size int64, compressed bool) error {
	var err error
	// 4 byte magic
	if _, err := fd.Write(BLOB_MAGIC[:]); err != nil {
		return err
	}
	// 2 byte version
	if err := binary.Write(fd, binary.BigEndian, DEFAULT_BLOB_VERSION); err != nil {
		return err
	}
	// 2 byte method
	method := so.method(compressed)
	if err := binary.Write(fd, binary.BigEndian, method); err != nil {
		return err
	}
	// 8 byte uncompressed length
	if err = binary.Write(fd, binary.BigEndian, size); err != nil {
		return err
	}
	bytes, err := compress(r, fd, method)
	if err != nil {
		return err
	}
	if size >= 0 {
		if size != bytes {
			return fmt.Errorf("blob size not match expected, actual size %d, expected size %d", bytes, size)
		}
		return nil
	}
	if err := fd.Sync(); err != nil {
		return err
	}
	if _, err := fd.Seek(8, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(fd, binary.BigEndian, bytes); err != nil {
		return err
	}
	return nil
}

func mkdir(paths ...string) error {
	for _, path := range paths {
		// os.MkdirAll check dir exists
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func finalizeObject(oldpath string, newpath string) (err error) {
	if err = strengthen.FinalizeObject(oldpath, newpath); err == nil {
		_ = os.Chmod(newpath, 0444)
	}
	return
}

// HashTo encode input reader to blob
// BLOB format
//
//	4 byte magic
//	2 byte version
//	2 byte method
//	8 byte uncompressed length
//	N bytes raw or compressed data
func (so *fileStorer) HashTo(ctx context.Context, r io.Reader, size int64) (oid plumbing.Hash, err error) {
	var payload []byte
	if payload, err = streamio.ReadMax(r, mimePacketSize); err != nil && err != io.EOF {
		return oid, fmt.Errorf("ReadFull error: %v", err)
	}
	compressed := isBinaryPayload(payload)
	var contents io.Reader = bytes.NewReader(payload)
	if err != io.EOF {
		contents = io.MultiReader(contents, r)
	}
	hasher := plumbing.NewHasher()
	if err = mkdir(so.incoming); err != nil {
		return
	}
	var fd *os.File
	if fd, err = os.CreateTemp(so.incoming, "blob"); err != nil {
		return oid, err
	}
	incomingPath := fd.Name()
	if err = so.hashToInternal(fd, io.TeeReader(contents, hasher), size, compressed); err != nil {
		_ = fd.Close()
		_ = os.Remove(incomingPath)
		return
	}
	_ = fd.Sync() // flush
	_ = fd.Close()
	oid = hasher.Sum()
	objectPath := so.path(oid)
	if err = os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		_ = os.Remove(incomingPath)
		return
	}
	if err = finalizeObject(incomingPath, objectPath); err != nil {
		_ = os.Remove(incomingPath)
		return
	}
	return
}

func (so *fileStorer) WriteEncoded(e object.Encoder) (oid plumbing.Hash, err error) {
	var fd *os.File
	if err = mkdir(so.incoming); err != nil {
		return
	}
	if fd, err = os.CreateTemp(so.incoming, "metadata"); err != nil {
		return oid, err
	}
	incomingPath := fd.Name()
	hasher := plumbing.NewHasher()
	if err = e.Encode(io.MultiWriter(hasher, fd)); err != nil {
		_ = fd.Close()
		_ = os.Remove(incomingPath)
		return
	}
	_ = fd.Sync() // flush
	_ = fd.Close()
	oid = hasher.Sum()
	metaObjectPath := so.path(oid)
	if err = os.MkdirAll(filepath.Dir(metaObjectPath), 0755); err != nil {
		_ = os.Remove(incomingPath)
		return
	}
	if err = finalizeObject(incomingPath, metaObjectPath); err != nil {
		_ = os.Remove(incomingPath)
		return
	}
	return
}

var (
	ignoreDir = map[string]bool{
		"pack": true,
	}
)

func (so *fileStorer) Search(prefix plumbing.Hash) (oid plumbing.Hash, err error) {
	prefixStr := prefix.Prefix()
	searchRoot := filepath.Join(so.root, prefixStr[0:2], prefixStr[2:4])
	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, prefixStr) {
			return nil
		}
		if !plumbing.ValidateHashHex(name) {
			return nil
		}
		oid = plumbing.NewHash(name)
		return filepath.SkipAll
	})
	if oid.IsZero() {
		return oid, plumbing.NoSuchObject(prefix)
	}
	return
}

type LooseObject struct {
	Hash         plumbing.Hash
	Size         int64
	Modification int64
}

type LooseObjects []*LooseObject

func (so *fileStorer) looseObjects(sizeMax int64) (LooseObjects, error) {
	objects := make([]*LooseObject, 0, 100)
	err := filepath.WalkDir(so.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !plumbing.ValidateHashHex(name) {
			return nil
		}
		si, err := d.Info()
		if err != nil {
			return err
		}
		// skip large files
		if si.Size() > sizeMax {
			return nil
		}
		objects = append(objects, &LooseObject{Hash: plumbing.NewHash(name), Size: si.Size(), Modification: si.ModTime().Unix()})
		return nil
	})
	return objects, err
}

func (so *fileStorer) LooseObjects() ([]plumbing.Hash, error) {
	oids := make([]plumbing.Hash, 0, 100)
	err := filepath.WalkDir(so.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !plumbing.ValidateHashHex(name) {
			return nil
		}
		oids = append(oids, plumbing.NewHash(name))
		return nil
	})
	return oids, err
}

func (so *fileStorer) Unpack(oid plumbing.Hash, r io.Reader) (err error) {
	if err = mkdir(so.incoming); err != nil {
		return
	}
	var fd *os.File
	if fd, err = os.CreateTemp(so.incoming, "object"); err != nil {
		return
	}
	incomingPath := fd.Name()
	if _, err = fd.ReadFrom(r); err != nil {
		_ = fd.Close()
		_ = os.Remove(incomingPath)
		return
	}
	_ = fd.Close()
	objectPath := so.path(oid)
	if err = os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		_ = os.Remove(incomingPath)
		return
	}
	if err = finalizeObject(incomingPath, objectPath); err != nil {
		_ = os.Remove(incomingPath)
		return
	}
	return
}

// func removeEmptyDirs(ctx context.Context, target string) (int, error) {
// 	if err := ctx.Err(); err != nil {
// 		return 0, err
// 	}

// 	entries, err := os.ReadDir(target)
// 	switch {
// 	case os.IsNotExist(err):
// 		return 0, nil // race condition: someone else deleted it first
// 	case err != nil:
// 		return 0, err
// 	}

// 	prunedDirsTotal := 0
// 	for _, e := range entries {
// 		if !e.IsDir() {
// 			continue
// 		}

// 		prunedDirs, err := removeEmptyDirs(ctx, filepath.Join(target, e.Name()))
// 		if err != nil {
// 			return prunedDirsTotal, err
// 		}
// 		prunedDirsTotal += prunedDirs
// 	}

// 	// recheck entries now that we have potentially removed some dirs
// 	entries, err = os.ReadDir(target)
// 	if err != nil && !os.IsNotExist(err) {
// 		return prunedDirsTotal, err
// 	}
// 	if len(entries) > 0 {
// 		return prunedDirsTotal, nil
// 	}

// 	switch err := os.Remove(target); {
// 	case os.IsNotExist(err):
// 		return prunedDirsTotal, nil // race condition: someone else deleted it first
// 	case err != nil:
// 		return prunedDirsTotal, err
// 	}

// 	return prunedDirsTotal + 1, nil
// }

func removeDirIfEmpty(ctx context.Context, target string) (total int, deleted bool, err error) {
	entries, err := os.ReadDir(target)
	switch {
	case os.IsNotExist(err):
		return 0, true, nil // race condition: someone else deleted it first
	case err != nil:
		return 0, false, err
	}
	var removedEntries int
	for _, e := range entries {
		if !e.IsDir() {
			return
		}
		name := filepath.Join(target, e.Name())
		var sd int
		var ok bool
		if sd, ok, err = removeDirIfEmpty(ctx, name); err != nil {
			return
		}
		if ok {
			removedEntries++
		}
		total += sd
	}
	if removedEntries != len(entries) {
		return total, false, nil
	}
	switch err = os.Remove(target); {
	case os.IsExist(err):
		return total, false, nil
	case err != nil:
		return total, false, err
	}
	return total + 1, true, nil
}

func (so *fileStorer) Prune(ctx context.Context) (int, error) {
	total, _, err := removeDirIfEmpty(ctx, so.root)
	return total, err
}

var (
	ErrCanceled = context.Canceled
)

func (so *fileStorer) PruneObject(ctx context.Context, oid plumbing.Hash) error {
	if err := ctx.Err(); err != nil {
		return ErrCanceled
	}
	p := so.path(oid)
	if err := os.Remove(p); err != nil {
		return err
	}
	return nil
}

func (so *fileStorer) PruneObjects(ctx context.Context, largeSize int64) ([]plumbing.Hash, int64, error) {
	oids := make([]plumbing.Hash, 0, 100)
	var totalSize int64
	err := filepath.WalkDir(so.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !plumbing.ValidateHashHex(name) {
			return nil
		}
		si, err := d.Info()
		if err != nil {
			return err
		}
		size := si.Size()
		if size < largeSize {
			return nil

		}
		if err = os.Remove(filepath.Join(path, name)); err == nil {
			oids = append(oids, plumbing.NewHash(name))
			totalSize += size
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	return oids, totalSize, err
}
