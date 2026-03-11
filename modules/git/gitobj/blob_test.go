package gitobj

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBlobReturnsCorrectObjectType(t *testing.T) {
	if BlobObjectType != new(Blob).Type() {
		t.Errorf("Expected %v, got %v", BlobObjectType, new(Blob).Type())
	}
}

func TestBlobFromString(t *testing.T) {
	given := []byte("example")
	glen := len(given)

	b := NewBlobFromBytes(given)

	if uint64(glen) != uint64(b.Size) {
		t.Errorf("Expected %v, got %v", glen, b.Size)
	}

	contents, err := io.ReadAll(b.Contents)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if !bytes.Equal(given, contents) {
		t.Errorf("Expected %v, got %v", given, contents)
	}
}

func TestBlobEncoding(t *testing.T) {
	const contents = "Hello, world!\n"

	b := &Blob{
		Size:     int64(len(contents)),
		Contents: strings.NewReader(contents),
	}

	var buf bytes.Buffer
	if _, err := b.Encode(&buf); err != nil {
		t.Fatal(err.Error())
	}
	if contents != (&buf).String() {
		t.Errorf("Expected %v, got %v", contents, (&buf).String())
	}
}

func TestBlobDecoding(t *testing.T) {
	const contents = "Hello, world!\n"
	from := strings.NewReader(contents)

	b := new(Blob)
	n, err := b.Decode(sha1.New(), from, int64(len(contents)))

	if n != 0 {
		t.Errorf("Expected %v, got %v", 0, n)
	}
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if uint64(len(contents)) != uint64(b.Size) {
		t.Errorf("Expected %v, got %v", len(contents), b.Size)
	}

	got, err := io.ReadAll(b.Contents)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte(contents), got) {
		t.Errorf("Expected %v, got %v", []byte(contents), got)
	}
}

func TestBlobCallCloseFn(t *testing.T) {
	var calls uint32

	expected := errors.New("some close error")

	b := &Blob{
		closeFn: func() error {
			atomic.AddUint32(&calls, 1)
			return expected
		},
	}

	got := b.Close()

	if expected != got {
		t.Errorf("Expected %v, got %v", expected, got)
	}
	if uint32(1) != calls {
		t.Errorf("Expected %v, got %v", 1, calls)
	}
}

func TestBlobCanCloseWithoutCloseFn(t *testing.T) {
	b := &Blob{
		closeFn: nil,
	}

	if b.Close() != nil {
		t.Errorf("Expected nil, got %v", b.Close())
	}
}

func TestBlobEqualReturnsTrueWithUnchangedContents(t *testing.T) {
	c := strings.NewReader("Hello, world!")

	b1 := &Blob{Size: int64(c.Len()), Contents: c}
	b2 := &Blob{Size: int64(c.Len()), Contents: c}

	if !b1.Equal(b2) {
		t.Errorf("Expected true")
	}
}

func TestBlobEqualReturnsFalseWithChangedContents(t *testing.T) {
	c1 := strings.NewReader("Hello, world!")
	c2 := strings.NewReader("Goodbye, world!")

	b1 := &Blob{Size: int64(c1.Len()), Contents: c1}
	b2 := &Blob{Size: int64(c2.Len()), Contents: c2}

	if b1.Equal(b2) {
		t.Errorf("Expected false")
	}
}

func TestBlobEqualReturnsTrueWhenOneBlobIsNil(t *testing.T) {
	b1 := &Blob{Size: 1, Contents: bytes.NewReader([]byte{0xa})}
	b2 := (*Blob)(nil)

	if b1.Equal(b2) {
		t.Errorf("Expected false")
	}
	if b2.Equal(b1) {
		t.Errorf("Expected false")
	}
}

func TestBlobEqualReturnsTrueWhenBothBlobsAreNil(t *testing.T) {
	b1 := (*Blob)(nil)
	b2 := (*Blob)(nil)

	if !b1.Equal(b2) {
		t.Errorf("Expected true")
	}
}
