package deflect

// we only support v2(SHA1/SHA256) idxfile format
// https://forcemz.net/git/2017/11/22/GitNativeHookDepthOptimization/

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
	// ErrUnsupportedVersion is returned by Decode when the idx file version
	// is not supported.
	ErrUnsupportedVersion = errors.New("idxfile: Unsupported version")
	// ErrMalformedIdxFile is returned by Decode when the idx file is corrupted.
	ErrMalformedIdxFile = errors.New("idxfile: Malformed IDX file")
)

const (
	fanout = 256
	// VersionSupported is the only idx version supported.
	// version 3 --> sha1/sha256 object hybrid storage
	VersionSupported uint32 = 2
	isO64Mask               = uint64(1) << 31
	offsetMask              = int(0x7fffffff)
)

var (
	idxHeader = []byte{255, 't', 'O', 'c'}
)

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

func (f *Filter) hashFromIndex(rs io.ReadSeeker, i int64) (string, error) {
	bin := make([]byte, f.rawsz)
	const ob int64 = 4 + 4 + 4 + 255*4
	if _, err := rs.Seek(ob+i*f.rawsz, io.SeekStart); err != nil {
		return "", err
	}
	if _, err := io.ReadFull(rs, bin[0:f.rawsz]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bin[0:f.rawsz]), nil
}

func (f *Filter) FilterPack(p *pack) error {
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
	/// number of entries
	if err := binary.Read(fd, binary.BigEndian, &nr); err != nil {
		return err
	}
	f.counts += nr
	/*
	 * Minimum size:
	 *  - 8 bytes of header
	 *  - 256 index entries 4 bytes each
	 *  - object ID entry * nr
	 *  - 4-byte crc entry * nr
	 *  - 4-byte offset entry * nr
	 *  - hash of the packfile
	 *  - file checksum
	 * And after the 4-byte offset table might be a
	 * variable sized table containing 8-byte entries
	 * for offsets larger than 2^31.
	 */
	// hash + offset + crc32 + magic+ version+ fan-out
	minSize := (f.rawsz+4+4)*int64(nr) + 4 + 4 + 4*fanout + f.rawsz + f.rawsz
	if minSize < fi.Size() {
		return f.filterPack64(fd, nr, p.size)
	}
	return f.filterPack32(fd, nr, p.size)
}

func (f *Filter) filterPack32(rs io.ReadSeeker, nr uint32, packsz int64) error {
	seekTo := int64(nr)*int64(f.rawsz+4) + 4 + 4 + fanout*4
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
	pre := packsz - f.rawsz
	for _, o := range objs {
		sz := pre - int64(o.offset)
		pre = int64(o.offset)
		if sz > hugeSizeLimit {
			f.hugeSum += sz
		}
		if sz < f.Limit {
			continue
		}
		hs, err := f.hashFromIndex(rs, int64(o.index))
		if err != nil {
			return err
		}
		if err := f.reject(hs, sz); err != nil {
			return err
		}
	}
	return nil
}

func (f *Filter) filterPack64(rs io.ReadSeeker, nr uint32, packsz int64) error {
	seekTo := int64(nr)*(f.rawsz+4) + 4 + 4 + fanout*4
	if _, err := rs.Seek(seekTo, io.SeekStart); err != nil {
		return err
	}
	bindata := make([]byte, nr*4)
	if _, err := io.ReadFull(rs, bindata[:]); err != nil {
		return err
	}
	objs := make(object64s, nr)
	for i := range nr {
		objs[i].index = i
		objs[i].offset = int64(binary.BigEndian.Uint32(bindata[i*4:]))
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
	pre := packsz - f.rawsz
	for _, o := range objs {
		sz := pre - int64(o.offset)
		pre = int64(o.offset)
		if sz > hugeSizeLimit {
			f.hugeSum += sz
		}
		if sz < f.Limit {
			continue
		}
		hs, err := f.hashFromIndex(rs, int64(o.index))
		if err != nil {
			return err
		}
		if err := f.reject(hs, sz); err != nil {
			return err
		}
	}
	return nil
}
