// Copyright (c) 2016-present GitLab Inc.
// SPDX-License-Identifier: MIT
package reftable

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/git"
)

var (
	// magic is the magic value of a reftable header.
	magic = [...]byte{'R', 'E', 'F', 'T'}
	// hashIDSHA1 denotes this reftable uses SHA1.
	hashIDSHA1 = [...]byte{'s', 'h', 'a', '1'}
	// hashIDSHA256 denotes this reftable uses SHA256.
	hashIDSHA256 = [...]byte{'s', '2', '5', '6'}
)

// version represents a reftable version.
type version uint8

// HeaderSize returns the size of the header for this reftable version.
func (v version) HeaderSize() int {
	switch v {
	case 1:
		// The Size is documented at https://git-scm.com/docs/reftable#_header_version_1
		return 24
	case 2:
		// The size  is documented at https://git-scm.com/docs/reftable#_header_version_2
		return 28
	default:
		panic(fmt.Errorf("unsupported version: %d", v))
	}
}

// FooterSize returns the size of the footer for this reftable version.
func (v version) FooterSize() int {
	// The footer sizes are documented at https://git-scm.com/docs/reftable#_footer.
	switch v {
	case 1:
		return 68
	case 2:
		return 72
	default:
		panic(fmt.Errorf("unsupported version: %d", v))
	}
}

// headerV1 is the exact byte layout of a header in reftable version 1.
type headerV1 struct {
	Magic          [4]byte
	Version        version
	BlockSize      [3]byte
	MinUpdateIndex uint64
	MaxUpdateIndex uint64
}

// header is exact byte layout of a header in reftable version 2.
type header struct {
	headerV1
	// HashID is only present if version is 2
	HashID [4]byte
}

// parseHeader parses the header of a reftable. reader should be at the beginning
// of the header.
func parseHeader(reader io.Reader, hdr *header) error {
	if err := binary.Read(reader, binary.BigEndian, &hdr.headerV1); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	if hdr.Magic != magic {
		return fmt.Errorf("unexpected magic bytes: %q", hdr.Magic)
	}

	if hdr.Version != 1 && hdr.Version != 2 {
		return fmt.Errorf("unsupported version: %d", hdr.Version)
	}

	if hdr.Version == 2 {
		if err := binary.Read(reader, binary.BigEndian, &hdr.HashID); err != nil {
			return fmt.Errorf("read hash id: %w", err)
		}

		if hdr.HashID != hashIDSHA1 && hdr.HashID != hashIDSHA256 {
			return fmt.Errorf("unsupported hash id: %q", hdr.HashID)
		}
	}

	return nil
}

// footerEnd is the exact byte layout of the unique fields in the footer after the duplicated header.
type footerEnd struct {
	RefIndexOffset     uint64
	ObjectOffsetAndLen uint64
	ObjectIndexOffset  uint64
	LogOffset          uint64
	LogIndexPosition   uint64
	CRC32              uint32
}

// footer is the exact byte layout of a footer in a reftable.
type footer struct {
	header
	footerEnd
}

// parseFooter parses the footer of a reftable. reader should be at the beginning
// of the footer.
func parseFooter(reader io.Reader, f *footer) error {
	footerBytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read all: %w", err)
	}

	footerReader := bytes.NewReader(footerBytes)
	if err := parseHeader(footerReader, &f.header); err != nil {
		return fmt.Errorf("parse header: %w", err)
	}

	if err := binary.Read(footerReader, binary.BigEndian, &f.footerEnd); err != nil {
		return fmt.Errorf("parse remainder: %w", err)
	}

	if crc32.ChecksumIEEE(footerBytes[:len(footerBytes)-binary.Size(f.CRC32)]) != f.CRC32 {
		return errors.New("checksum mismatch")
	}

	return nil
}

type block struct {
	BlockStart    uint
	FullBlockSize uint
	HeaderOffset  uint
	RestartCount  uint16
	RestartStart  uint
}

// Table represents .ref table file.
type Table struct {
	blockSize    uint
	footerOffset uint
	src          *os.File
	absolutePath string
	footer       footer
}

// MinUpdateIndex is the minimum update index in the table.
func (t *Table) MinUpdateIndex() uint64 {
	return t.footer.MinUpdateIndex
}

// MaxUpdateIndex is the maximum update index in the table.
func (t *Table) MaxUpdateIndex() uint64 {
	return t.footer.MaxUpdateIndex
}

// shaFormat maps reftable sha format to Gitaly's hash object.
func (t *Table) shaFormat() git.HashFormat {
	if t.footer.Version == 2 && t.footer.HashID == hashIDSHA256 {
		return git.HashSHA256
	}
	return git.HashSHA1
}

// parseUInt24 parses a big endian encoded uint24 into a uint.
func parseUint24(data [3]byte) uint {
	return uint(data[2]) | uint(data[1])<<8 | uint(data[0])<<16
}

// getBlockRange provides the abs block range if the block is smaller
// than the table.
func (t *Table) getBlockRange(offset, size uint) (uint, uint) {
	if offset >= t.footerOffset {
		return 0, 0
	}

	if offset+size > t.footerOffset {
		size = t.footerOffset - offset
	}

	return offset, offset + size
}

// extractBlockLen extracts the block length from a given location.
func (t *Table) extractBlockLen(src []byte, blockStart uint) uint {
	return uint(big.NewInt(0).SetBytes(src[blockStart+1 : blockStart+4]).Uint64())
}

// getVarInt parses a variable int and increases the index.
func (t *Table) getVarInt(src []byte, start uint, blockEnd uint) (uint, uint, error) {
	var val uint

	val = uint(src[start]) & 0x7f

	for (uint(src[start]) & 0x80) > 0 {
		start++
		if start > blockEnd {
			return 0, 0, fmt.Errorf("exceeded block length")
		}

		val = ((val + 1) << 7) | (uint(src[start]) & 0x7f)
	}

	return start + 1, val, nil
}

// getRefsFromBlock provides the ref udpates from a reference block.
func (t *Table) getRefsFromBlock(src []byte, b *block) ([]git.Reference, error) {
	var references []git.Reference

	prefix := ""

	// Skip the block_type and block_len
	idx := b.BlockStart + 4

	for idx < b.RestartStart {
		var prefixLength, suffixLength, updateIndexDelta uint
		var err error

		idx, prefixLength, err = t.getVarInt(src, idx, b.RestartStart)
		if err != nil {
			return nil, fmt.Errorf("getting prefix length: %w", err)
		}

		idx, suffixLength, err = t.getVarInt(src, idx, b.RestartStart)
		if err != nil {
			return nil, fmt.Errorf("getting suffix length: %w", err)
		}

		extra := (suffixLength & 0x7)
		suffixLength >>= 3

		refname := prefix[:prefixLength] + string(src[idx:idx+suffixLength])
		idx = idx + suffixLength

		idx, updateIndexDelta, err = t.getVarInt(src, idx, b.FullBlockSize)
		if err != nil {
			return nil, fmt.Errorf("getting update index delta: %w", err)
		}
		// we don't use this for now
		_ = updateIndexDelta
		reference := git.Reference{
			Name: git.ReferenceName(refname),
		}

		switch extra {
		case 0:

			// Deletion, no value
			reference.Target = t.shaFormat().ZeroOID()
		case 1:
			// Regular reference
			hashSize := t.shaFormat().RawSize()
			reference.Target = hex.EncodeToString(src[idx : idx+uint(hashSize)])

			idx += uint(hashSize)
		case 2:
			// Peeled Tag
			hashSize := t.shaFormat().RawSize()
			reference.Target = hex.EncodeToString(src[idx : idx+uint(hashSize)])

			idx += uint(hashSize)

			// For now we don't need the peeledOID, but we still need
			// to skip the index.
			// peeledOID := ObjectID(bytesToHex(t.src[idx : idx+uint(hashSize)]))
			idx += uint(hashSize)
		case 3:
			// Symref
			var size uint
			idx, size, err = t.getVarInt(src, idx, b.FullBlockSize)
			if err != nil {
				return nil, fmt.Errorf("getting symref size: %w", err)
			}

			reference.Target = git.ReferenceName(src[idx : idx+size]).String()
			reference.IsSymbolic = true
			idx = idx + size
		}

		prefix = refname

		references = append(references, reference)
	}

	return references, nil
}

// parseRefBlock parses a block and if it is a ref block, provides
// all the reference updates.
func (t *Table) parseRefBlock(src []byte, headerOffset, blockStart, blockEnd uint) ([]git.Reference, error) {
	currentBS := t.extractBlockLen(src, blockStart+headerOffset)

	fullBlockSize := t.blockSize
	if fullBlockSize == 0 {
		fullBlockSize = currentBS
	} else if currentBS < fullBlockSize && currentBS < (blockEnd-blockStart) && src[blockStart+currentBS] != 0 {
		fullBlockSize = currentBS
	}

	b := &block{
		BlockStart:    blockStart + headerOffset,
		FullBlockSize: fullBlockSize,
	}

	if err := binary.Read(bytes.NewBuffer(src[blockStart+currentBS-2:]), binary.BigEndian, &b.RestartCount); err != nil {
		return nil, fmt.Errorf("reading restart count: %w", err)
	}

	b.RestartStart = blockStart + currentBS - 2 - 3*uint(b.RestartCount)

	return t.getRefsFromBlock(src, b)
}

// GetReferences returns all references from the table.
func (t *Table) GetReferences() ([]git.Reference, error) {
	headerOffset := uint(t.footer.Version.HeaderSize())
	offset := uint(0)
	var allRefs []git.Reference

	if _, err := t.src.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek start: %w", err)
	}

	src, err := io.ReadAll(t.src)
	if err != nil {
		return nil, fmt.Errorf("read all: %w", err)
	}

	for offset < t.footerOffset {
		blockStart, blockEnd := t.getBlockRange(offset, t.blockSize)
		if blockStart == 0 && blockEnd == 0 {
			break
		}

		// If we run out of ref blocks, we can stop the iteration.
		if src[blockStart+headerOffset] != 'r' {
			return allRefs, nil
		}

		references, err := t.parseRefBlock(src, headerOffset, blockStart, blockEnd)
		if err != nil {
			return nil, fmt.Errorf("parsing block: %w", err)
		}

		if len(references) == 0 {
			break
		}

		allRefs = append(allRefs, references...)

		offset = blockEnd
	}

	return allRefs, nil
}

// PatchUpdateIndexes patches in-place the update indexes stored in the table's
// header and footer, and syncs the file to the disk.
func (t *Table) PatchUpdateIndexes(min, max uint64) (returnedErr error) {
	// Table typically opens the file with read-only permissions. The update index
	// patching is an exception, and the only case when we should be modifying tables.
	// Typically the table files would not be modified, and the files in the storage
	// are read-only to prevent accidental modifications. Due to the default read-only
	// permissions, we'd fail to open most of the files in the storage for writes.
	//
	// Open a separate descriptor with write permissions to handle this special case
	// of patching update indexes.
	file, err := os.OpenFile(t.absolutePath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			returnedErr = errors.Join(err, fmt.Errorf("close: %w", err))
		}
	}()

	t.footer.MinUpdateIndex = min
	t.footer.MaxUpdateIndex = max

	// Construct a buffer that contains the full footer with patched values.
	//
	// The footer contains the header as well. First serialize the header.
	buffer := bytes.NewBuffer(make([]byte, 0, t.footer.Version.FooterSize()))
	if err := binary.Write(buffer, binary.BigEndian, t.footer.headerV1); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Only the version two header contains the HashID.
	if t.footer.Version == 2 {
		if err := binary.Write(buffer, binary.BigEndian, t.footer.HashID); err != nil {
			return fmt.Errorf("write hash ID: %w", err)
		}
	}

	// After the header, serialize the remaining footer values. This will also serialize
	// the old CRC32 into the footer, but we'll patch it below.
	if err := binary.Write(buffer, binary.BigEndian, t.footer.footerEnd); err != nil {
		return fmt.Errorf("write footer: %w", err)
	}

	// The footer ends with a CRC32 that covers everything in the footer except the checksum
	// itself. Compute the checksum and override the old value that was written above when
	// we serialized the footer with the patched update indexes.
	footerWithoutChecksum := buffer.Bytes()[:buffer.Len()-crc32.Size]
	t.footer.CRC32 = crc32.ChecksumIEEE(footerWithoutChecksum)

	footerBytes := binary.BigEndian.AppendUint32(footerWithoutChecksum, t.footer.CRC32)

	// Finally, write the updated header and footer into the file.
	if _, err := file.WriteAt(footerBytes[:t.footer.Version.HeaderSize()], 0); err != nil {
		return fmt.Errorf("patch header: %w", err)
	}

	if _, err := file.WriteAt(footerBytes, int64(t.footerOffset)); err != nil {
		return fmt.Errorf("patch footer: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	return nil
}

// Close closes the table's associated file.
func (t *Table) Close() error {
	return t.src.Close()
}

// ParseTable opens the table at the given path and parses it. Close must be called
// once the table is no longer used to close the associated file.
func ParseTable(absolutePath string) (_ *Table, returnedErr error) {
	src, err := os.Open(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	defer func() {
		if returnedErr != nil {
			if err := src.Close(); err != nil {
				returnedErr = errors.Join(returnedErr, fmt.Errorf("close: %w", err))
			}
		}
	}()

	t := &Table{src: src, absolutePath: absolutePath}

	var h header
	if err := parseHeader(src, &h); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	footerOffset, err := src.Seek(int64(-h.Version.FooterSize()), io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("seek footer: %w", err)
	}

	t.footerOffset = uint(footerOffset)

	if err := parseFooter(src, &t.footer); err != nil {
		return nil, fmt.Errorf("parse footer: %w", err)
	}

	if h != t.footer.header {
		return nil, fmt.Errorf("footer doesn't match header")
	}

	t.blockSize = parseUint24(t.footer.BlockSize)

	return t, nil
}

// ReadTablesList returns a list of tables in the "tables.list" for the
// reftable backend.
func ReadTablesList(repoPath string) ([]Name, error) {
	tablesListPath := filepath.Join(repoPath, "reftable", "tables.list")

	data, err := os.ReadFile(tablesListPath)
	if err != nil {
		return nil, fmt.Errorf("reading tables.list: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	names := make([]Name, len(lines))
	for i, line := range lines {
		if names[i], err = ParseName(line); err != nil {
			return nil, fmt.Errorf("parse name: %w", err)
		}
	}

	return names, nil
}

// Name contains the structured information in the name of a .ref file.
type Name struct {
	// MinUpdateIndex is the minimum update index contained in the file.
	MinUpdateIndex uint64
	// MinUpdateIndex is the maximum update index contained in the file.
	MaxUpdateIndex uint64
	// Suffix is the random suffix in the table's name.
	Suffix string
}

// String returns the string representation of the reftable name.
func (n Name) String() string {
	return fmt.Sprintf("0x%012x-0x%012x-%s.ref", n.MinUpdateIndex, n.MaxUpdateIndex, n.Suffix)
}

// nameRegex is a regex for matching reftable names
// e.g. 0x000000000001-0x00000000000a-b54f3b59.ref would result in the following submatches:
//   - 000000000001 (UpdateIndexMin)
//   - 00000000000a (UpdateIndexMax)
//   - b54f3b59     (Suffix)
//
// See the reftable documentation at https://www.git-scm.com/docs/reftable#_layout for more
// information.
var nameRegex = regexp.MustCompile("^0x([[:xdigit:]]{12,16})-0x([[:xdigit:]]{12,16})-([0-9a-zA-Z]{8}).ref$")

// ParseName parses the name of a reftable file.
func ParseName(reftableName string) (Name, error) {
	matches := nameRegex.FindStringSubmatch(reftableName)
	if len(matches) == 0 {
		return Name{}, fmt.Errorf("reftable name %q malformed", reftableName)
	}

	minIndex, err := strconv.ParseUint(matches[1], 16, 64)
	if err != nil {
		return Name{}, fmt.Errorf("parsing min index: %w", err)
	}

	maxIndex, err := strconv.ParseUint(matches[2], 16, 64)
	if err != nil {
		return Name{}, fmt.Errorf("parsing max index: %w", err)
	}

	return Name{
		MinUpdateIndex: minIndex,
		MaxUpdateIndex: maxIndex,
		Suffix:         matches[3],
	}, nil
}
