package deflect

// We only support Git pack index file version 2 (SHA1/SHA256)
// Reference: https://forcemz.net/git/2017/11/22/GitNativeHookDepthOptimization/

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

var (
	// ErrUnsupportedVersion is returned when the pack index file version is not supported
	ErrUnsupportedVersion = errors.New("idxfile: Unsupported version")
	// ErrMalformedIdxFile is returned when the pack index file is corrupted or invalid
	ErrMalformedIdxFile = errors.New("idxfile: Malformed IDX file")
)

const (
	// fanout is the number of fanout table entries (256 for SHA1/SHA256)
	fanout = 256
	// VersionSupported is the only pack index version supported (v2)
	// Version 3 supports SHA1/SHA256 hybrid object storage but we only support v2
	VersionSupported uint32 = 2
	// isO64Mask is used to identify 64-bit offsets in the offset table
	isO64Mask = uint64(1) << 31
	// offsetMask extracts the actual offset value from a 32-bit offset entry
	offsetMask = int(0x7fffffff)
)

var (
	// idxHeader is the magic header for Git pack index files: "\xfftOc"
	idxHeader = []byte{255, 't', 'O', 'c'}
)

// validateHeader reads and validates the pack index file header
func validateHeader(r io.Reader) error {
	var h = make([]byte, 4)
	if _, err := io.ReadFull(r, h); err != nil {
		return err
	}

	if !bytes.Equal(h, idxHeader) {
		return ErrMalformedIdxFile
	}

	return nil
}

// hashFromIndex extracts object hash from pack index file at the given index
// Parameters:
//   - rs: ReadSeeker for the pack index file
//   - i: object index position
//
// Returns the hexadecimal encoded hash string
func (a *Auditor) hashFromIndex(rs io.ReadSeeker, i int64) (string, error) {
	bin := make([]byte, a.rawsz)
	// Pack index file format v2 offset calculation:
	// - 4 bytes: magic header
	// - 4 bytes: version (2)
	// - 4 bytes: fanout count (256)
	// - 255*4 bytes: fanout table (256 entries, 4 bytes each)
	const ob int64 = 4 + 4 + 4 + 255*4
	if _, err := rs.Seek(ob+i*a.rawsz, io.SeekStart); err != nil {
		return "", err
	}
	if _, err := io.ReadFull(rs, bin[0:a.rawsz]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bin[0:a.rawsz]), nil
}

// analyzePack analyzes a single pack file to find large objects
// Opens the corresponding .idx file and determines whether to use
// 32-bit or 64-bit offset processing based on file size
func (a *Auditor) analyzePack(p *pack) error {
	idx := strings.TrimSuffix(p.path, ".pack") + ".idx"
	fd, err := os.Open(idx)
	if err != nil {
		return err
	}
	defer fd.Close() // nolint
	fi, err := fd.Stat()
	if err != nil {
		return err
	}
	if err = validateHeader(fd); err != nil {
		return err
	}
	var v, nr uint32
	if err := binary.Read(fd, binary.BigEndian, &v); err != nil {
		return err
	}
	if v != VersionSupported {
		return ErrUnsupportedVersion
	}
	if _, err := fd.Seek(255*4, io.SeekCurrent); err != nil {
		return err
	}
	/// number of entries in pack file
	if err := binary.Read(fd, binary.BigEndian, &nr); err != nil {
		return err
	}
	a.counts += nr
	/*
	 * Minimum pack index file size calculation:
	 *  - 8 bytes of header (4 magic + 4 version)
	 *  - 256 fanout entries, 4 bytes each
	 *  - object ID entry * nr
	 *  - 4-byte crc entry * nr
	 *  - 4-byte offset entry * nr
	 *  - packfile hash
	 *  - file checksum
	 * And after the 4-byte offset table there might be a
	 * variable sized table containing 8-byte entries
	 * for offsets larger than 2^31.
	 */
	// hash + offset + crc32 + magic + version + fanout
	minSize := (a.rawsz+4+4)*int64(nr) + 4 + 4 + 4*fanout + a.rawsz + a.rawsz
	if minSize < fi.Size() {
		return a.analyzePack64(fd, nr, p.size)
	}
	return a.analyzePack32(fd, nr, p.size)
}

// analyzePack32 processes pack files with 32-bit offsets (< 2GB)
// Uses sorting algorithm to estimate object sizes by comparing consecutive offsets
func (a *Auditor) analyzePack32(rs io.ReadSeeker, nr uint32, packsz int64) error {
	seekTo := int64(nr)*int64(a.rawsz+4) + 4 + 4 + fanout*4
	if _, err := rs.Seek(seekTo, io.SeekStart); err != nil {
		return err
	}
	br := bufio.NewReader(rs)
	objs := make(object32s, nr)
	for i := range nr {
		objs[i].index = i
		var offset uint32
		if err := binary.Read(br, binary.BigEndian, &offset); err != nil {
			return err
		}
		objs[i].offset = offset
	}
	sort.Sort(objs)
	pre := packsz - a.rawsz
	for _, o := range objs {
		sz := pre - int64(o.offset)
		pre = int64(o.offset)
		if sz > hugeSizeLimit {
			a.hugeSum += sz
		}
		if sz < a.Limit {
			continue
		}
		hs, err := a.hashFromIndex(rs, int64(o.index))
		if err != nil {
			return err
		}
		if err := a.onOversized(hs, sz); err != nil {
			return err
		}
	}
	return nil
}

// analyzePack64 processes pack files with 64-bit offsets (>= 2GB)
// Handles both 32-bit and 64-bit offset entries, using the 64-bit offset table
// when the MSB (most significant bit) is set in the 32-bit offset field
func (a *Auditor) analyzePack64(rs io.ReadSeeker, nr uint32, packsz int64) error {
	seekTo := int64(nr)*(a.rawsz+4) + 4 + 4 + fanout*4
	if _, err := rs.Seek(seekTo, io.SeekStart); err != nil {
		return err
	}
	bindata := make([]byte, nr*4)
	if _, err := io.ReadFull(rs, bindata); err != nil {
		return err
	}
	objs := make(object64s, nr)
	for i := range nr {
		objs[i].index = i
		objs[i].offset = int64(binary.BigEndian.Uint32(bindata[i*4:]))
		// Check if this is a large offset (MSB set)
		if objs[i].offset&int64(isO64Mask) != 0 {
			off := objs[i].offset & int64(offsetMask)
			if _, err := rs.Seek(seekTo+int64(nr)*4+off*8, io.SeekStart); err != nil {
				return err
			}
			if err := binary.Read(rs, binary.BigEndian, &objs[i].offset); err != nil {
				return err
			}
		}
	}

	sort.Sort(objs)
	pre := packsz - a.rawsz
	for _, o := range objs {
		sz := pre - int64(o.offset)
		pre = int64(o.offset)
		if sz > hugeSizeLimit {
			a.hugeSum += sz
		}
		if sz < a.Limit {
			continue
		}
		hs, err := a.hashFromIndex(rs, int64(o.index))
		if err != nil {
			return err
		}
		if err := a.onOversized(hs, sz); err != nil {
			return err
		}
	}
	return nil
}
