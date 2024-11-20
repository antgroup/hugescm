package gitobj

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectWriterWritesHeaders(t *testing.T) {
	var buf bytes.Buffer

	w := NewObjectWriter(&buf, sha1.New())

	n, err := w.WriteHeader(BlobObjectType, 1)
	assert.Equal(t, 7, n)
	assert.Nil(t, err)

	assert.Nil(t, w.Close())

	r, err := zlib.NewReader(&buf)
	assert.Nil(t, err)

	all, err := io.ReadAll(r)
	assert.Nil(t, err)
	assert.Equal(t, []byte("blob 1\x00"), all)

	assert.Nil(t, r.Close())
}

func TestObjectWriterWritesData(t *testing.T) {
	testCases := []struct {
		h   hash.Hash
		sha string
	}{
		{
			sha1.New(), "56a6051ca2b02b04ef92d5150c9ef600403cb1de",
		},
		{
			sha256.New(), "36456d9b87f21fc54ed5babf1222a9ab0fbbd0c4ad239a7933522d5e4447049c",
		},
	}

	for _, test := range testCases {
		var buf bytes.Buffer

		w := NewObjectWriter(&buf, test.h)
		_, _ = w.WriteHeader(BlobObjectType, 1)

		n, err := w.Write([]byte{0x31})
		assert.Equal(t, 1, n)
		assert.Nil(t, err)

		assert.Nil(t, w.Close())

		r, err := zlib.NewReader(&buf)
		assert.Nil(t, err)

		all, err := io.ReadAll(r)
		assert.Nil(t, err)
		assert.Equal(t, []byte("blob 1\x001"), all)

		assert.Nil(t, r.Close())
		assert.Equal(t, test.sha, hex.EncodeToString(w.Sha()))
	}
}

func TestObjectWriterKeepsTrackOfHash(t *testing.T) {
	w := NewObjectWriter(new(bytes.Buffer), sha1.New())
	n, err := w.WriteHeader(BlobObjectType, 1)

	assert.Nil(t, err)
	assert.Equal(t, 7, n)

	assert.Equal(t, "bb6ca78b66403a67c6281df142de5ef472186283", hex.EncodeToString(w.Sha()))

	w = NewObjectWriter(new(bytes.Buffer), sha256.New())
	n, err = w.WriteHeader(BlobObjectType, 1)

	assert.Nil(t, err)
	assert.Equal(t, 7, n)

	assert.Equal(t, "3a68c454a6eb75cc55bda147a53756f0f581497eb80b9b67156fb8a8d3931cd7", hex.EncodeToString(w.Sha()))
}

type WriteCloserFn struct {
	io.Writer
	closeFn func() error
}

func (r *WriteCloserFn) Close() error { return r.closeFn() }

func TestObjectWriterCallsClose(t *testing.T) {
	var calls uint32

	expected := errors.New("close error")

	w := NewObjectWriteCloser(&WriteCloserFn{
		Writer: new(bytes.Buffer),
		closeFn: func() error {
			atomic.AddUint32(&calls, 1)
			return expected
		},
	}, sha1.New())

	got := w.Close()

	assert.EqualValues(t, 1, calls)
	assert.Equal(t, expected, got)
}
