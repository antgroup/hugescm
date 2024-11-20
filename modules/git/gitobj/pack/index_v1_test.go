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
	V1IndexFanout = make([]uint32, indexFanoutEntries)
)

func TestIndexV1SearchExact(t *testing.T) {
	for _, algo := range []hash.Hash{sha1.New(), sha256.New()} {
		index := newV1Index(algo)
		v := &V1{hash: algo}
		e, err := v.Entry(index, 1)

		assert.NoError(t, err)
		assert.EqualValues(t, 2, e.PackOffset)
	}
}

func TestIndexVersionWidthV1(t *testing.T) {
	for _, algo := range []hash.Hash{sha1.New(), sha256.New()} {
		v := &V1{hash: algo}
		assert.EqualValues(t, 0, v.Width())
	}
}

func newV1Index(hash hash.Hash) *Index {
	V1IndexFanout[1] = 1
	V1IndexFanout[2] = 2
	V1IndexFanout[3] = 3

	for i := 3; i < len(V1IndexFanout); i++ {
		V1IndexFanout[i] = 3
	}

	fanout := make([]byte, indexFanoutWidth)
	for i, n := range V1IndexFanout {
		binary.BigEndian.PutUint32(fanout[i*indexFanoutEntryWidth:], n)
	}

	hashlen := hash.Size()
	entrylen := hashlen + indexObjectCRCWidth
	entries := make([]byte, entrylen*3)

	for i := 0; i < 3; i++ {
		// For each entry, set the first three bytes to 0 and the
		// remainder to the same value.  That creates an initial 4-byte
		// CRC field with the value of i+1, followed by a series of data
		// bytes which all have that same value.
		for j := entrylen*i + 3; j < entrylen*(i+1); j++ {
			entries[j] = byte(i + 1)
		}
	}

	buf := make([]byte, 0, indexOffsetV1Start)

	buf = append(buf, fanout...)
	buf = append(buf, entries...)

	return &Index{
		fanout:  V1IndexFanout,
		version: &V1{hash: hash},
		r:       bytes.NewReader(buf),
	}

}
