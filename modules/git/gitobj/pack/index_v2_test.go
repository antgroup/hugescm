package pack

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	V2IndexHeader = []byte{
		0xff, 0x74, 0x4f, 0x63,
		0x00, 0x00, 0x00, 0x02,
	}
	V2IndexFanout = make([]uint32, indexFanoutEntries)

	V2IndexCRCs = []byte{
		0x0, 0x0, 0x0, 0x0,
		0x1, 0x1, 0x1, 0x1,
		0x2, 0x2, 0x2, 0x2,
	}

	V2IndexOffsets = []byte{
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
		0x80, 0x00, 0x00, 0x01, // use the second large offset

		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // filler data
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, // large offset
	}
)

func TestIndexV2EntryExact(t *testing.T) {
	for _, algo := range []hash.Hash{sha1.New(), sha256.New()} {
		index := newV2Index(algo)
		v := &V2{hash: algo}
		e, err := v.Entry(index, 1)

		assert.NoError(t, err)
		assert.EqualValues(t, 2, e.PackOffset)
	}
}

func TestIndexV2EntryExtendedOffset(t *testing.T) {
	for _, algo := range []hash.Hash{sha1.New(), sha256.New()} {
		index := newV2Index(algo)
		v := &V2{hash: algo}
		e, err := v.Entry(index, 2)

		assert.NoError(t, err)
		assert.EqualValues(t, 3, e.PackOffset)
	}
}

func TestIndexVersionWidthV2(t *testing.T) {
	for _, algo := range []hash.Hash{sha1.New(), sha256.New()} {
		v := &V2{hash: algo}
		assert.EqualValues(t, 8, v.Width())
	}
}

func newV2Index(hash hash.Hash) *Index {
	V2IndexFanout[1] = 1
	V2IndexFanout[2] = 2
	V2IndexFanout[3] = 3

	for i := 3; i < len(V2IndexFanout); i++ {
		V2IndexFanout[i] = 3
	}

	fanout := make([]byte, indexFanoutWidth)
	for i, n := range V2IndexFanout {
		binary.BigEndian.PutUint32(fanout[i*indexFanoutEntryWidth:], n)
	}

	hashlen := hash.Size()
	names := make([]byte, hashlen*3)

	for i := range names {
		names[i] = byte((i / hashlen) + 1)
	}

	buf := make([]byte, 0, indexOffsetV2Start+3)
	buf = append(buf, V2IndexHeader...)
	buf = append(buf, fanout...)
	buf = append(buf, names...)
	buf = append(buf, V2IndexCRCs...)
	buf = append(buf, V2IndexOffsets...)

	return &Index{
		fanout:  V2IndexFanout,
		version: &V2{hash: hash},
		r:       bytes.NewReader(buf),
	}
}
