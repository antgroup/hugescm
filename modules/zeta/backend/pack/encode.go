// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package pack

import (
	"bufio"
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"

	"github.com/antgroup/hugescm/modules/binary"
	"github.com/antgroup/hugescm/modules/plumbing"
)

const (
	PackVersion   uint32 = 'Z'
	NoEntries     uint32 = 0
	entriesOffset        = 4 + 4             // MAGIC(4)+VERSION(4)
	objectOffset         = entriesOffset + 4 // ENTRIES(4)
)

var (
	packMagic = [4]byte{'P', 'A', 'C', 'K'}
)

type Entry struct {
	Hash         plumbing.Hash
	CRC32        uint32
	Offset       uint64
	Modification uint64
}

type objects []*Entry

// EntriesSort sorts a slice of write index in increasing order.
func EntriesSort(o objects) {
	sort.Sort(o)
}

func (o objects) Len() int           { return len(o) }
func (o objects) Less(i, j int) bool { return bytes.Compare(o[i].Hash[:], o[j].Hash[:]) < 0 }
func (o objects) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }

type Encoder struct {
	fd      *os.File
	hasher  plumbing.Hasher
	bw      *bufio.Writer
	w       io.Writer
	version uint32
	entries uint32
	offset  uint64
	objects objects
	sum     plumbing.Hash
}

func NewEncoder(fd *os.File, entries uint32) (*Encoder, error) {
	e := &Encoder{fd: fd, bw: bufio.NewWriter(fd), version: PackVersion, entries: entries}
	if entries != 0 {
		e.hasher = plumbing.NewHasher()
		e.w = io.MultiWriter(e.bw, e.hasher)
		e.objects = make([]*Entry, 0, int(entries))
	} else {
		e.w = e.bw
		e.objects = make([]*Entry, 0, 400)
	}
	if _, err := e.w.Write(packMagic[:]); err != nil {
		return nil, err
	}
	if err := binary.WriteUint32(e.w, e.version); err != nil {
		return nil, err
	}
	if err := binary.WriteUint32(e.w, e.entries); err != nil {
		return nil, err
	}
	e.offset = objectOffset
	return e, nil
}

func (e *Encoder) WriteTrailer() error {
	if e.hasher.Hash != nil {
		e.sum = e.hasher.Sum()
		if _, err := e.bw.Write(e.sum[:]); err != nil {
			return err
		}
		return e.bw.Flush()
	}
	// Flush all data, we shou
	if err := e.bw.Flush(); err != nil {
		return err
	}
	// The data in the buffer should be flushed to the file immediately,
	// then the number of entries should be corrected, the file BLAKE3 hash should be calculated, and written to the end of the packet.
	if _, err := e.fd.WriteAt(binary.Swap32(uint32(len(e.objects))), entriesOffset); err != nil {
		return err
	}
	if _, err := e.fd.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hasher := plumbing.NewHasher()
	if _, err := io.Copy(hasher, e.fd); err != nil {
		return err
	}
	// When we have read all the data, the offset of the file has reached the end.
	e.sum = hasher.Sum()
	_, err := e.fd.Write(e.sum[:])
	return err
}

func (e *Encoder) Write(oid plumbing.Hash, size uint32, r io.Reader, modification int64) (err error) {
	if err = binary.WriteUint32(e.w, size); err != nil {
		return
	}
	var written int64
	cr := crc32.New(crc32.IEEETable)
	if written, err = io.Copy(e.w, io.TeeReader(r, cr)); err != nil {
		return
	}
	if written != int64(size) {
		return fmt.Errorf("written %d not equal object %s size %d: %w", written, oid, size, io.ErrShortWrite)
	}
	e.objects = append(e.objects, &Entry{Hash: oid, CRC32: cr.Sum32(), Offset: e.offset, Modification: uint64(modification)})
	e.offset += uint64(size) + 4
	return
}

func (e *Encoder) Name() string {
	return e.sum.String()
}

const (
	offset64PosMask = uint64(1) << 31
)

// https://codewords.recurse.com/issues/three/unpacking-git-packfiles
func (e *Encoder) WriteIndex(fd *os.File) error {
	sort.Sort(e.objects)
	var fanout [256]uint32
	for _, o := range e.objects {
		fanout[uint8(o.Hash[0])]++
	}

	hasher := plumbing.NewHasher()
	bufWriter := bufio.NewWriter(fd)
	w := io.MultiWriter(bufWriter, hasher)
	if err := binary.Write(w, indexMagic[:]); err != nil {
		return err
	}
	if err := binary.WriteUint32(w, IndexVersionCurrent); err != nil {
		return err
	}
	var fanoutStore uint32
	for i := 0; i < 256; i++ {
		fanoutStore += fanout[i]
		if err := binary.WriteUint32(w, fanoutStore); err != nil {
			return err
		}
	}
	for _, o := range e.objects {
		if err := binary.Write(w, o.Hash[:]); err != nil {
			return err
		}
	}
	for _, o := range e.objects {
		if err := binary.WriteUint32(w, o.CRC32); err != nil {
			return err
		}
	}
	offset64Set := make([]uint64, 0, 20)
	var offset64Pos uint64
	for _, o := range e.objects {
		offset := o.Offset
		if offset > math.MaxInt32 {
			offset64Set = append(offset64Set, offset)
			offset = uint64(offset64Pos | offset64PosMask)
			offset64Pos++
		}
		if err := binary.WriteUint32(w, uint32(offset)); err != nil {
			return err
		}
	}
	for _, o := range offset64Set {
		if err := binary.WriteUint64(w, o); err != nil {
			return err
		}
	}
	if err := binary.Write(w, e.sum[:]); err != nil {
		return err
	}
	sum := hasher.Sum()
	if err := binary.Write(bufWriter, sum[:]); err != nil {
		return err
	}
	return bufWriter.Flush()
}

var (
	mtimeMagic = [4]byte{'M', 'T', 'E', 'M'}
)

func (e *Encoder) WriteModification(fd *os.File) error {
	hasher := plumbing.NewHasher()
	bufWriter := bufio.NewWriter(fd)
	w := io.MultiWriter(bufWriter, hasher)
	if err := binary.Write(w, mtimeMagic[:]); err != nil {
		return err
	}
	if err := binary.WriteUint32(w, PackVersion); err != nil {
		return err
	}
	for _, o := range e.objects {
		if err := binary.WriteUint64(w, o.Modification); err != nil {
			return err
		}
	}
	sum := hasher.Sum()
	if err := binary.Write(bufWriter, sum[:]); err != nil {
		return err
	}
	return bufWriter.Flush()
}

type Writer struct {
	e       *Encoder
	fd      *os.File
	packDir string
	closed  uint32
}

func NewWriter(packDir string, entries uint32) (*Writer, error) {
	fd, err := os.CreateTemp(packDir, "pack-")
	if err != nil {
		return nil, err
	}
	e, err := NewEncoder(fd, entries)
	if err != nil {
		return nil, err
	}
	return &Writer{e: e, fd: fd, packDir: packDir}, nil
}

func (w *Writer) Close() error {
	if w.fd != nil && atomic.CompareAndSwapUint32(&w.closed, 0, 1) {
		_ = w.fd.Chmod(0444) // Set pack to read-only
		return w.fd.Close()
	}
	return nil
}

func (w *Writer) Write(oid plumbing.Hash, size uint32, r io.Reader, modification int64) (err error) {
	return w.e.Write(oid, size, r, modification)
}

func (w *Writer) WriteTrailer() error {
	if err := w.e.WriteTrailer(); err != nil {
		return err
	}
	name := w.e.Name()
	packName := w.fd.Name()
	_ = w.Close()
	packNewName := filepath.Join(w.packDir, fmt.Sprintf("pack-%s.pack", name))
	if err := os.Rename(packName, packNewName); err != nil {
		return err
	}
	ifd, err := os.Create(filepath.Join(w.packDir, fmt.Sprintf("pack-%s.idx", name)))
	if err != nil {
		return err
	}
	defer ifd.Close()
	if err := w.e.WriteIndex(ifd); err != nil {
		return err
	}
	_ = ifd.Chmod(0444) // Set idx to read-only
	mfd, err := os.Create(filepath.Join(w.packDir, fmt.Sprintf("pack-%s.mtimes", name)))
	if err != nil {
		return err
	}
	defer mfd.Close()
	err = w.e.WriteModification(mfd)
	_ = mfd.Chmod(0444) // Set mtimes to read-only
	return err
}
