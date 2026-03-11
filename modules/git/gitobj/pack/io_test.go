package pack

import (
	"bytes"
	"errors"
	"testing"
)

func TestOffsetReaderAtReadsAtOffset(t *testing.T) {
	bo := &OffsetReaderAt{
		r: bytes.NewReader([]byte{0x0, 0x1, 0x2, 0x3}),
		o: 1,
	}

	var x1 [1]byte
	n1, e1 := bo.Read(x1[:])

	if e1 != nil {
		t.Errorf("Expected nil, got %v", e1)
	}
	if n1 != 1 {
		t.Errorf("Expected %v, got %v", 1, n1)
	}

	if x1[0] != 0x1 {
		t.Errorf("Expected %v, got %v", 0x1, x1[0])
	}

	var x2 [1]byte
	n2, e2 := bo.Read(x2[:])

	if e2 != nil {
		t.Errorf("Expected nil, got %v", e2)
	}
	if n2 != 1 {
		t.Errorf("Expected %v, got %v", 1, n2)
	}
	if x2[0] != 0x2 {
		t.Errorf("Expected %v, got %v", 0x2, x2[0])
	}
}

func TestOffsetReaderPropogatesErrors(t *testing.T) {
	expected := errors.New("git/object/pack: testing")
	bo := &OffsetReaderAt{
		r: &ErrReaderAt{Err: expected},
		o: 1,
	}

	n, err := bo.Read(make([]byte, 1))

	if expected != err {
		t.Errorf("Expected %v, got %v", expected, err)
	}
	if n != 0 {
		t.Errorf("Expected %v, got %v", 0, n)
	}
}

type ErrReaderAt struct {
	Err error
}

func (e *ErrReaderAt) ReadAt(p []byte, at int64) (n int, err error) {
	return 0, e.Err
}
