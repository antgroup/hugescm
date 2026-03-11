package gitobj

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

func TestObjectReaderReadsHeaders(t *testing.T) {
	var compressed bytes.Buffer

	zw := zlib.NewWriter(&compressed)
	_, _ = zw.Write([]byte("blob 1\x00"))
	_ = zw.Close()

	or, err := NewObjectReader(&compressed)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	typ, size, err := or.Header()

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if size != 1 {
		t.Errorf("Expected %v, got %v", 1, size)
	}
	if BlobObjectType != typ {
		t.Errorf("Expected %v, got %v", BlobObjectType, typ)
	}
}

func TestObjectReaderConsumesHeaderBeforeReads(t *testing.T) {
	var compressed bytes.Buffer

	zw := zlib.NewWriter(&compressed)
	_, _ = zw.Write([]byte("blob 1\x00asdf"))
	_ = zw.Close()

	or, err := NewObjectReader(&compressed)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	var buf [4]byte
	n, err := or.Read(buf[:])

	if n != 4 {
		t.Errorf("Expected %v, got %v", 4, n)
	}
	if !bytes.Equal([]byte{'a', 's', 'd', 'f'}, buf[:]) {
		t.Errorf("Expected %v, got %v", []byte{'a', 's', 'd', 'f'}, buf[:])
	}
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}

type ReadCloserFn struct {
	io.Reader
	closeFn func() error
}

func (r *ReadCloserFn) Close() error {
	return r.closeFn()
}

func TestObjectReaderCallsClose(t *testing.T) {
	var calls uint32
	expected := errors.New("expected")

	or, err := NewObjectReadCloser(&ReadCloserFn{
		Reader: bytes.NewBuffer([]byte{0x78, 0x01}),
		closeFn: func() error {
			atomic.AddUint32(&calls, 1)
			return expected
		},
	})
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	got := or.Close()

	if expected != got {
		t.Errorf("Expected %v, got %v", expected, got)
	}
	if atomic.LoadUint32(&calls) != 1 {
		t.Errorf("Expected %v, got %v", 1, atomic.LoadUint32(&calls))
	}

}
