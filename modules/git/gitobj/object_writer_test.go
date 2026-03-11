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
)

func TestObjectWriterWritesHeaders(t *testing.T) {
	var buf bytes.Buffer

	w := NewObjectWriter(&buf, sha1.New())

	n, err := w.WriteHeader(BlobObjectType, 1)
	if n != 7 {
		t.Errorf("Expected %v, got %v", 7, n)
	}
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if w.Close() != nil {
		t.Errorf("Expected nil, got %v", w.Close())
	}

	r, err := zlib.NewReader(&buf)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	all, err := io.ReadAll(r)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte("blob 1\x00"), all) {
		t.Errorf("Expected %v, got %v", []byte("blob 1\x00"), all)
	}

	if r.Close() != nil {
		t.Errorf("Expected nil, got %v", r.Close())
	}
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
		if n != 1 {
			t.Errorf("Expected %v, got %v", 1, n)
		}
		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}

		if w.Close() != nil {
			t.Errorf("Expected nil, got %v", w.Close())
		}

		r, err := zlib.NewReader(&buf)
		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}

		all, err := io.ReadAll(r)
		if err != nil {
			t.Errorf("Expected nil, got %v", err)
		}
		if !bytes.Equal([]byte("blob 1\x001"), all) {
			t.Errorf("Expected %v, got %v", []byte("blob 1\x001"), all)
		}

		if r.Close() != nil {
			t.Errorf("Expected nil, got %v", r.Close())
		}
		if test.sha != hex.EncodeToString(w.Sha()) {
			t.Errorf("Expected %v, got %v", test.sha, hex.EncodeToString(w.Sha()))
		}
	}
}

func TestObjectWriterKeepsTrackOfHash(t *testing.T) {
	w := NewObjectWriter(new(bytes.Buffer), sha1.New())
	n, err := w.WriteHeader(BlobObjectType, 1)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if n != 7 {
		t.Errorf("Expected %v, got %v", 7, n)
	}

	if hex.EncodeToString(w.Sha()) != "bb6ca78b66403a67c6281df142de5ef472186283" {
		t.Errorf("Expected %v, got %v", "bb6ca78b66403a67c6281df142de5ef472186283", hex.EncodeToString(w.Sha()))
	}

	w = NewObjectWriter(new(bytes.Buffer), sha256.New())
	n, err = w.WriteHeader(BlobObjectType, 1)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if n != 7 {
		t.Errorf("Expected %v, got %v", 7, n)
	}

	if hex.EncodeToString(w.Sha()) != "3a68c454a6eb75cc55bda147a53756f0f581497eb80b9b67156fb8a8d3931cd7" {
		t.Errorf("Expected %v, got %v", "3a68c454a6eb75cc55bda147a53756f0f581497eb80b9b67156fb8a8d3931cd7", hex.EncodeToString(w.Sha()))
	}
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

	if calls != 1 {
		t.Errorf("Expected %v, got %v", 1, calls)
	}
	if expected != got {
		t.Errorf("Expected %v, got %v", expected, got)
	}
}
