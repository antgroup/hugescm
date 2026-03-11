package pack

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
	"testing"
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

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if idx.Count() != 3 {
		t.Errorf("Expected %v, got %v", 3, idx.Count())
	}
}

func TestDecodeIndexV2InvalidFanout(t *testing.T) {
	buf := make([]byte, 0, indexV2Width+indexFanoutWidth-indexFanoutEntryWidth)
	buf = append(buf, 0xff, 0x74, 0x4f, 0x63)
	buf = append(buf, 0x0, 0x0, 0x0, 0x2)
	buf = append(buf, make([]byte, indexFanoutWidth-1)...)

	idx, err := DecodeIndex(bytes.NewReader(buf), sha1.New())

	if ErrShortFanout != err {
		t.Errorf("Expected %v, got %v", ErrShortFanout, err)
	}
	if idx != nil {
		t.Errorf("Expected nil, got %v", idx)
	}
}

func TestDecodeIndexV1(t *testing.T) {
	idx, err := DecodeIndex(bytes.NewReader(make([]byte, indexFanoutWidth)), sha1.New())

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if idx.Count() != 0 {
		t.Errorf("Expected %v, got %v", 0, idx.Count())
	}
}

func TestDecodeIndexV1InvalidFanout(t *testing.T) {
	idx, err := DecodeIndex(bytes.NewReader(make([]byte, indexFanoutWidth-1)), sha1.New())

	if ErrShortFanout != err {
		t.Errorf("Expected %v, got %v", ErrShortFanout, err)
	}
	if idx != nil {
		t.Errorf("Expected nil, got %v", idx)
	}
}

func TestDecodeIndexUnsupportedVersion(t *testing.T) {
	buf := make([]byte, 0, 4+4)
	buf = append(buf, 0xff, 0x74, 0x4f, 0x63)
	buf = append(buf, 0x0, 0x0, 0x0, 0x3)

	idx, err := DecodeIndex(bytes.NewReader(buf), sha1.New())

	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != "git/object/pack:: unsupported version: 3" {
		t.Errorf("Expected error message %v, got %v", "git/object/pack:: unsupported version: 3", err.Error())
	}
	if idx != nil {
		t.Errorf("Expected nil, got %v", idx)
	}
}

func TestDecodeIndexEmptyContents(t *testing.T) {
	idx, err := DecodeIndex(bytes.NewReader(make([]byte, 0)), sha1.New())

	if !errors.Is(err, io.EOF) {
		t.Errorf("Expected %v, got %v", io.EOF, err)
	}
	if idx != nil {
		t.Errorf("Expected nil, got %v", idx)
	}
}
