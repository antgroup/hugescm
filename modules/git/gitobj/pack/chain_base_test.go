package pack

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func TestChainBaseDecompressesData(t *testing.T) {
	const contents = "Hello, world!\n"

	compressed, err := compress(contents)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	var buf bytes.Buffer

	_, err = buf.Write([]byte{0x0, 0x0, 0x0, 0x0})
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	_, err = buf.Write(compressed)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	_, err = buf.Write([]byte{0x0, 0x0, 0x0, 0x0})
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	base := &ChainBase{
		offset: 4,
		size:   int64(len(contents)),

		r: bytes.NewReader(buf.Bytes()),
	}

	unpacked, err := base.Unpack()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if contents != string(unpacked) {
		t.Errorf("Expected %v, got %v", contents, string(unpacked))
	}
}

func TestChainBaseTypeReturnsType(t *testing.T) {
	b := &ChainBase{
		typ: TypeCommit,
	}

	if TypeCommit != b.Type() {
		t.Errorf("Expected %v, got %v", TypeCommit, b.Type())
	}
}

func compress(base string) ([]byte, error) {
	var buf bytes.Buffer

	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write([]byte(base)); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
