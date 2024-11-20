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

// https://git-scm.com/docs/gitformat-pack

const (
	IndexVersionCurrent = 'Z'
	// indexMagicWidth is the width of the magic header of packfiles version
	// 1 and newer.
	indexMagicWidth = 4
	// indexVersionWidth is the width of the version following the magic
	// header.
	indexVersionWidth = 4
	// indexV2Width is the total width of the header in V2.
	indexWidth = indexMagicWidth + indexVersionWidth

	// indexFanoutEntries is the number of entries in the fanout table.
	indexFanoutEntries = 256
	// indexFanoutEntryWidth is the width of each entry in the fanout table.
	indexFanoutEntryWidth = 4
	// indexFanoutWidth is the width of the entire fanout table.
	indexFanoutWidth = indexFanoutEntries * indexFanoutEntryWidth

	// indexOffsetStart is the location of the first object outside of the
	// header.
	indexOffsetStart = indexWidth + indexFanoutWidth

	// indexObjectCRCWidth is the width of the CRC accompanying each object.
	indexObjectCRCWidth = 4
	// indexObjectSmallOffsetWidth is the width of the small offset encoded
	// into each object.
	indexObjectSmallOffsetWidth = 4
	// indexObjectLargeOffsetWidth is the width of the optional large offset
	// encoded into the small offset.
	indexObjectLargeOffsetWidth = 8
)

var (
	indexMagic = [4]byte{0xff, 0x74, 0x4f, 0x63}
)

/*
 * Minimum size:
 * - 8 bytes of header
 * - 256 index entries 4 bytes each
 * - 32-byte BLAKE3 entry * nr
 * - 4-byte crc entry * nr
 * - 4-byte offset entry * nr
 * - 32-byte BLAKE3 of the packfile
 * - 32-byte BLAKE3 file checksum
 * And after the 4-byte offset table might be a
 * variable sized table containing 8-byte entries
 * for offsets larger than 2^31.
 */

// IndexEntry specifies data encoded into an entry in the pack index.
type IndexEntry struct {
	Pos int64
	// PackOffset is the number of bytes before the associated object in a
	// packfile.
	PackOffset uint64
}

type IndexVersion interface {
	// Name returns the name of the object located at the given offset "at",
	// in the Index file "idx".
	//
	// It returns an error if the object at that location could not be
	// parsed.
	Name(idx *Index, at int64) (plumbing.Hash, error)

	// Entry parses and returns the full *IndexEntry located at the offset
	// "at" in the Index file "idx".
	//
	// If there was an error parsing the IndexEntry at that location, it
	// will be returned.
	Entry(idx *Index, at int64) (*IndexEntry, error)
	// PackedObjects
	PackedObjects(idx *Index, recv RecvFunc) error

	// Width returns the number of bytes occupied by the header of a
	// particular index version.
	Width() int64
}

// Index stores information about the location of objects in a corresponding
// packfile.
type Index struct {
	// version is the encoding version used by this index.
	//
	// Currently, versions 1 and 2 are supported.
	version IndexVersion
	// fanout is the L1 fanout table stored in this index. For a given index
	// "i" into the array, the value stored at that index specifies the
	// number of objects in the packfile/index that are lexicographically
	// less than or equal to that index.
	//
	// See: https://github.com/git/git/blob/v2.13.0/Documentation/technical/pack-format.txt#L41-L45
	fanout []uint32

	// r is the underlying set of encoded data comprising this index file.
	r io.ReaderAt
}

// Count returns the number of objects in the packfile.
func (i *Index) Count() int {
	return int(i.fanout[255])
}

// Close closes the packfile index if the underlying data stream is closeable.
// If so, it returns any error involved in closing.
func (i *Index) Close() error {
	if close, ok := i.r.(io.Closer); ok {
		return close.Close()
	}
	return nil
}

var (
	// errNotFound is an error returned by Index.Entry() (see: below) when
	// an object cannot be found in the index.
	errNotFound = fmt.Errorf("zeta: object not found in index")
	// ErrShortFanout is an error representing situations where the entire
	// fanout table could not be read, and is thus too short.
	ErrShortFanout = fmt.Errorf("zeta: too short fanout table")
)

// IsNotFound returns whether a given error represents a missing object in the
// index.
func IsNotFound(err error) bool {
	return err == errNotFound
}

// Entry returns an entry containing the offset of a given BLAKE3 "name".
//
// Entry operates in O(log(n))-time in the worst case, where "n" is the number
// of objects that begin with the first byte of "name".
//
// If the entry cannot be found, (nil, ErrNotFound) will be returned. If there
// was an error searching for or parsing an entry, it will be returned as (nil,
// err).
//
// Otherwise, (entry, nil) will be returned.
func (i *Index) Entry(name plumbing.Hash) (*IndexEntry, error) {
	var last *bounds
	bounds := i.bounds(name)

	for bounds.Left() < bounds.Right() {
		if last.Equal(bounds) {
			// If the bounds are unchanged, that means either that
			// the object does not exist in the packfile, or the
			// fanout table is corrupt.
			//
			// Either way, we won't be able to find the object.
			// Return immediately to prevent infinite looping.
			return nil, errNotFound
		}
		last = bounds

		// Find the midpoint between the upper and lower bounds.
		mid := bounds.Left() + ((bounds.Right() - bounds.Left()) / 2)

		got, err := i.version.Name(i, mid)
		if err != nil {
			return nil, err
		}

		if cmp := bytes.Compare(name[:], got[:]); cmp == 0 {
			// If "cmp" is zero, that means the object at that index
			// "at" had a SHA equal to the one given by name, and we
			// are done.
			return i.version.Entry(i, mid)
		} else if cmp < 0 {
			// If the comparison is less than 0, we searched past
			// the desired object, so limit the upper bound of the
			// search to the midpoint.
			bounds = bounds.WithRight(mid)
		} else if cmp > 0 {
			// Likewise, if the comparison is greater than 0, we
			// searched below the desired object. Modify the bounds
			// accordingly.
			bounds = bounds.WithLeft(mid)
		}

	}

	return nil, errNotFound
}

func prefixCompare(want, got plumbing.Hash) int {
	sl := want.Shorten()
	return bytes.Compare(want[:sl], got[:sl])
}

func (i *Index) Search(name plumbing.Hash) (oid plumbing.Hash, error error) {
	var last *bounds
	bounds := i.bounds(name)

	for bounds.Left() < bounds.Right() {
		if last.Equal(bounds) {
			// If the bounds are unchanged, that means either that
			// the object does not exist in the packfile, or the
			// fanout table is corrupt.
			//
			// Either way, we won't be able to find the object.
			// Return immediately to prevent infinite looping.
			return oid, errNotFound
		}
		last = bounds

		// Find the midpoint between the upper and lower bounds.
		mid := bounds.Left() + ((bounds.Right() - bounds.Left()) / 2)

		got, err := i.version.Name(i, mid)
		if err != nil {
			return oid, err
		}

		if cmp := prefixCompare(name, got); cmp == 0 {
			// If "cmp" is zero, that means the object at that index
			// "at" had a SHA equal to the one given by name, and we
			// are done.
			return got, nil
		} else if cmp < 0 {
			// If the comparison is less than 0, we searched past
			// the desired object, so limit the upper bound of the
			// search to the midpoint.
			bounds = bounds.WithRight(mid)
		} else if cmp > 0 {
			// Likewise, if the comparison is greater than 0, we
			// searched below the desired object. Modify the bounds
			// accordingly.
			bounds = bounds.WithLeft(mid)
		}

	}

	return oid, errNotFound
}

// readAt is a convenience method that allow reading into the underlying data
// source from other callers within this package.
func (i *Index) readAt(p []byte, at int64) (n int, err error) {
	return i.r.ReadAt(p, at)
}

// bounds returns the initial bounds for a given name using the fanout table to
// limit search results.
func (i *Index) bounds(name plumbing.Hash) *bounds {
	var left, right int64

	if name[0] == 0 {
		// If the lower bound is 0, there are no objects before it,
		// start at the beginning of the index file.
		left = 0
	} else {
		// Otherwise, make the lower bound the slot before the given
		// object.
		left = int64(i.fanout[name[0]-1])
	}

	if name[0] == 255 {
		// As above, if the upper bound is the max byte value, make the
		// upper bound the last object in the list.
		right = int64(i.Count())
	} else {
		// Otherwise, make the upper bound the first object which is not
		// within the given slot.
		right = int64(i.fanout[name[0]+1])
	}

	return newBounds(left, right)
}

func (i *Index) PackedObjects(recv RecvFunc) error {
	return i.version.PackedObjects(i, recv)
}

// DecodeIndex decodes an index whose underlying data is supplied by "r".
//
// DecodeIndex reads only the header and fanout table, and does not eagerly
// parse index entries.
//
// If there was an error parsing, it will be returned immediately.
func DecodeIndex(r io.ReaderAt) (*Index, error) {
	version, err := decodeIndexHeader(r)
	if err != nil {
		return nil, err
	}

	fanout, err := decodeIndexFanout(r, version.Width())
	if err != nil {
		return nil, err
	}

	return &Index{
		version: version,
		fanout:  fanout,

		r: r,
	}, nil
}

// decodeIndexHeader determines which version the index given by "r" is.
func decodeIndexHeader(r io.ReaderAt) (IndexVersion, error) {
	hdr := make([]byte, 4)
	if _, err := r.ReadAt(hdr, 0); err != nil {
		return nil, err
	}

	if !bytes.Equal(hdr, indexMagic[:]) {
		return nil, errBadIndexHeader
	}
	versionByte := make([]byte, 4)
	if _, err := r.ReadAt(versionByte, 4); err != nil {
		return nil, err
	}
	version := binary.BigEndian.Uint32(versionByte)
	switch version {
	case IndexVersionCurrent:
		return &IndexZ{}, nil
	}
	return nil, &UnsupportedVersionErr{uint32(version)}
}

// decodeIndexFanout decodes the fanout table given by "r" and beginning at the
// given offset.
func decodeIndexFanout(r io.ReaderAt, offset int64) ([]uint32, error) {
	b := make([]byte, 256*4)
	if _, err := r.ReadAt(b, offset); err != nil {
		if err == io.EOF {
			return nil, ErrShortFanout
		}
		return nil, err
	}

	fanout := make([]uint32, 256)
	for i := range fanout {
		fanout[i] = binary.BigEndian.Uint32(b[(i * 4):])
	}

	return fanout, nil
}
