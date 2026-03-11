package gitobj

import (
	"bytes"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/git/gitobj/errors"
)

func TestMemoryStorerIncludesGivenEntries(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hex, err := hex.DecodeString(sha)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	ms := newMemoryStorer(map[string]io.ReadWriter{
		sha: bytes.NewBuffer([]byte{0x1}),
	})

	buf, err := ms.Open(hex)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	contents, err := io.ReadAll(buf)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte{0x1}, contents) {
		t.Errorf("Expected %v, got %v", []byte{0x1}, contents)
	}
}

func TestMemoryStorerAcceptsNilEntries(t *testing.T) {
	ms := newMemoryStorer(nil)

	if len(ms.fs) != 0 {
		t.Errorf("Expected 0, got %v", len(ms.fs))
	}
	if ms.Close() != nil {
		t.Errorf("Expected nil, got %v", ms.Close())
	}
}

func TestMemoryStorerDoesntOpenMissingEntries(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	hex, err := hex.DecodeString(sha)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	ms := newMemoryStorer(nil)

	f, err := ms.Open(hex)
	if !errors.IsNoSuchObject(err) {
		t.Errorf("Expected NoSuchObject error, got %v", err)
	}
	if f != nil {
		t.Errorf("Expected nil, got %v", f)
	}
}

func TestMemoryStorerStoresNewEntries(t *testing.T) {
	hex, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	ms := newMemoryStorer(nil)

	if len(ms.fs) != 0 {
		t.Errorf("Expected 0, got %v", len(ms.fs))
	}

	_, err = ms.Store(hex, strings.NewReader("hello"))
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if len(ms.fs) != 1 {
		t.Errorf("Expected 1, got %v", len(ms.fs))
	}

	got, err := ms.Open(hex)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	contents, err := io.ReadAll(got)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if string(contents) != "hello" {
		t.Errorf("Expected %v, got %v", "hello", string(contents))
	}
}

func TestMemoryStorerStoresExistingEntries(t *testing.T) {
	hex, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	ms := newMemoryStorer(nil)

	if len(ms.fs) != 0 {
		t.Errorf("Expected 0, got %v", len(ms.fs))
	}

	_, err = ms.Store(hex, new(bytes.Buffer))
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if len(ms.fs) != 1 {
		t.Errorf("Expected 1, got %v", len(ms.fs))
	}

	n, err := ms.Store(hex, new(bytes.Buffer))
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if int64(0) != n {
		t.Errorf("Expected %v, got %v", 0, n)
	}
}
