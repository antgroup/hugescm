package gitobj

import (
	"bytes"
	"encoding/hex"
	"io"
	"reflect"
	"testing"
)

func TestNewMemoryBackend(t *testing.T) {
	backend, err := NewMemoryBackend(nil)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	ro, rw := backend.Storage()
	if ro != rw {
		t.Errorf("Expected %v, got %v", ro, rw)
	}
	if ro.(*memoryStorer) == nil {
		t.Errorf("Expected non-nil")
	}
}

func TestNewMemoryBackendWithReadOnlyData(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oid, err := hex.DecodeString(sha)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	m := map[string]io.ReadWriter{
		sha: bytes.NewBuffer([]byte{0x1}),
	}

	backend, err := NewMemoryBackend(m)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	ro, _ := backend.Storage()
	reader, err := ro.Open(oid)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte{0x1}, contents) {
		t.Errorf("Expected %v, got %v", []byte{0x1}, contents)
	}
}

func TestNewMemoryBackendWithWritableData(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oid, err := hex.DecodeString(sha)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	backend, err := NewMemoryBackend(make(map[string]io.ReadWriter))
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	buf := bytes.NewBuffer([]byte{0x1})

	ro, rw := backend.Storage()
	_, _ = rw.Store(oid, buf)

	reader, err := ro.Open(oid)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte{0x1}, contents) {
		t.Errorf("Expected %v, got %v", []byte{0x1}, contents)
	}
}

func TestSplitAlternatesString(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"abc", []string{"abc"}},
		{"abc:def", []string{"abc", "def"}},
		{`"abc":def`, []string{"abc", "def"}},
		{`"i\alike\bcomplicated\tstrings":def`, []string{"i\alike\bcomplicated\tstrings", "def"}},
		{`abc:"i\nlike\vcomplicated\fstrings\r":def`, []string{"abc", "i\nlike\vcomplicated\fstrings\r", "def"}},
		{`abc:"uni\xc2\xa9ode":def`, []string{"abc", "uni©ode", "def"}},
		{`abc:"uni\302\251ode\10\0":def`, []string{"abc", "uni©ode\x08\x00", "def"}},
		{`abc:"cookie\\monster\"":def`, []string{"abc", "cookie\\monster\"", "def"}},
	}

	for _, test := range testCases {
		actual := splitAlternateString(test.input, ":")
		if !reflect.DeepEqual(actual, test.expected) {
			t.Errorf("unexpected output for %q: got %v, expected %v", test.input, actual, test.expected)
		}
	}
}
