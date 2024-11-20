// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package pack

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
)

const (
	HashDigestSize = plumbing.HASH_DIGEST_SIZE
)

// IndexZ implements IndexVersion for packfiles.
type IndexZ struct {
}

// Name implements IndexVersion.Name by returning the 32 byte BLAKE3 object name
// for the given entry at offset "at" in the v2 index file "idx".
func (v *IndexZ) Name(idx *Index, at int64) (oid plumbing.Hash, err error) {
	if _, err = idx.readAt(oid[:], hashOffset(at)); err != nil {
		return
	}
	return
}

// Entry implements IndexVersion.Entry for v2 packfiles by parsing and returning
// the IndexEntry specified at the offset "at" in the given index file.
func (v *IndexZ) Entry(idx *Index, at int64) (*IndexEntry, error) {
	var offs [4]byte

	if _, err := idx.readAt(offs[:], smallOffsetOffset(at, int64(idx.Count()))); err != nil {
		return nil, err
	}

	loc := uint64(binary.BigEndian.Uint32(offs[:]))
	if loc&0x80000000 > 0 {
		// If the most significant bit (MSB) of the offset is set, then
		// the offset encodes the indexed location for an 8-byte offset.
		//
		// Mask away (offs&0x7fffffff) the MSB to use as an index to
		// find the offset of the 8-byte pack offset.
		lo := largeOffsetOffset(int64(loc&0x7fffffff), int64(idx.Count()))

		var offs [8]byte
		if _, err := idx.readAt(offs[:], lo); err != nil {
			return nil, err
		}

		loc = binary.BigEndian.Uint64(offs[:])
	}
	return &IndexEntry{PackOffset: loc, Pos: at}, nil
}

// Width implements IndexVersion.Width() by returning the number of bytes that
// v2 packfile index header occupy.
func (v *IndexZ) Width() int64 {
	return indexWidth
}

type RecvFunc func(oid plumbing.Hash, modification int64) error

func openMtimesFD(idx *Index) (*os.File, error) {
	fd, ok := idx.r.(*os.File)
	if !ok {
		return nil, errors.New("bad index")
	}
	return os.Open(strings.TrimSuffix(fd.Name(), ".idx") + ".mtimes")
}

func (v *IndexZ) PackedObjects(idx *Index, recv RecvFunc) error {
	total := idx.Count()
	br := bufio.NewReader(NewSizeReader(idx.r, indexOffsetStart, int64(total*HashDigestSize)))
	mfd, err := openMtimesFD(idx)
	if err != nil {
		for i := 0; i < total; i++ {
			var oid plumbing.Hash
			if _, err := io.ReadFull(br, oid[:]); err != nil {
				return err
			}
			if err := recv(oid, 0); err != nil {
				return err
			}
		}
		return nil
	}
	defer mfd.Close()
	if _, err := mfd.Seek(8, io.SeekStart); err != nil {
		return err
	}
	mbr := bufio.NewReader(mfd)
	var mtimeBytes [8]byte
	for i := 0; i < total; i++ {
		var oid plumbing.Hash
		if _, err := io.ReadFull(br, oid[:]); err != nil {
			return err
		}
		if _, err := io.ReadFull(mbr, mtimeBytes[:]); err != nil {
			return err
		}
		if err := recv(oid, int64(binary.BigEndian.Uint64(mtimeBytes[:]))); err != nil {
			return err
		}
	}
	return nil
}

// hashOffset returns the offset of a SHA1 given at "at" in the V2 index file.
func hashOffset(at int64) int64 {
	// Skip the packfile index header and the L1 fanout table.
	return indexOffsetStart +
		// Skip until the desired name in the sorted names table.
		(HashDigestSize * at)
}

// smallOffsetOffset returns the offset of an object's small (4-byte) offset
// given by "at".
func smallOffsetOffset(at, total int64) int64 {
	// Skip the packfile index header and the L1 fanout table.
	return indexOffsetStart +
		// Skip the name table.
		(HashDigestSize * total) +
		// Skip the CRC table.
		(indexObjectCRCWidth * total) +
		// Skip until the desired index in the small offsets table.
		(indexObjectSmallOffsetWidth * at)
}

// largeOffsetOffset returns the offset of an object's large (4-byte) offset,
// given by the index "at".
func largeOffsetOffset(at, total int64) int64 {
	// Skip the packfile index header and the L1 fanout table.
	return indexOffsetStart +
		// Skip the name table.
		(HashDigestSize * total) +
		// Skip the CRC table.
		(indexObjectCRCWidth * total) +
		// Skip the small offsets table.
		(indexObjectSmallOffsetWidth * total) +
		// Seek to the large offset within the large offset(s) table.
		(indexObjectLargeOffsetWidth * at)
}
