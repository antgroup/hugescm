package pack

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeIndexV2(t *testing.T) {
	buf := make([]byte, 0, indexV2Width+indexFanoutWidth)
	buf = append(buf, 0xff, 0x74, 0x4f, 0x63)
	buf = append(buf, 0x0, 0x0, 0x0, 0x2)
	for range indexFanoutEntries {
		x := make([]byte, 4)

		binary.BigEndian.PutUint32(x, uint32(3))

		buf = append(buf, x...)
	}

	idx, err := DecodeIndex(bytes.NewReader(buf), sha1.New())

	assert.NoError(t, err)
	assert.EqualValues(t, 3, idx.Count())
}

func TestDecodeIndexV2InvalidFanout(t *testing.T) {
	buf := make([]byte, 0, indexV2Width+indexFanoutWidth-indexFanoutEntryWidth)
	buf = append(buf, 0xff, 0x74, 0x4f, 0x63)
	buf = append(buf, 0x0, 0x0, 0x0, 0x2)
	buf = append(buf, make([]byte, indexFanoutWidth-1)...)

	idx, err := DecodeIndex(bytes.NewReader(buf), sha1.New())

	assert.Equal(t, ErrShortFanout, err)
	assert.Nil(t, idx)
}

func TestDecodeIndexV1(t *testing.T) {
	idx, err := DecodeIndex(bytes.NewReader(make([]byte, indexFanoutWidth)), sha1.New())

	assert.NoError(t, err)
	assert.EqualValues(t, 0, idx.Count())
}

func TestDecodeIndexV1InvalidFanout(t *testing.T) {
	idx, err := DecodeIndex(bytes.NewReader(make([]byte, indexFanoutWidth-1)), sha1.New())

	assert.Equal(t, ErrShortFanout, err)
	assert.Nil(t, idx)
}

func TestDecodeIndexUnsupportedVersion(t *testing.T) {
	buf := make([]byte, 0, 4+4)
	buf = append(buf, 0xff, 0x74, 0x4f, 0x63)
	buf = append(buf, 0x0, 0x0, 0x0, 0x3)

	idx, err := DecodeIndex(bytes.NewReader(buf), sha1.New())

	assert.EqualError(t, err, "git/object/pack:: unsupported version: 3")
	assert.Nil(t, idx)
}

func TestDecodeIndexEmptyContents(t *testing.T) {
	idx, err := DecodeIndex(bytes.NewReader(make([]byte, 0)), sha1.New())

	assert.Equal(t, io.EOF, err)
	assert.Nil(t, idx)
}
