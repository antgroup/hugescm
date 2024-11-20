package gitobj

import (
	"bytes"
	"encoding/hex"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMemoryBackend(t *testing.T) {
	backend, err := NewMemoryBackend(nil)
	assert.NoError(t, err)

	ro, rw := backend.Storage()
	assert.Equal(t, ro, rw)
	assert.NotNil(t, ro.(*memoryStorer))
}

func TestNewMemoryBackendWithReadOnlyData(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oid, err := hex.DecodeString(sha)

	assert.Nil(t, err)

	m := map[string]io.ReadWriter{
		sha: bytes.NewBuffer([]byte{0x1}),
	}

	backend, err := NewMemoryBackend(m)
	assert.NoError(t, err)

	ro, _ := backend.Storage()
	reader, err := ro.Open(oid)
	assert.NoError(t, err)

	contents, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x1}, contents)
}

func TestNewMemoryBackendWithWritableData(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oid, err := hex.DecodeString(sha)

	assert.Nil(t, err)

	backend, err := NewMemoryBackend(make(map[string]io.ReadWriter))
	assert.NoError(t, err)

	buf := bytes.NewBuffer([]byte{0x1})

	ro, rw := backend.Storage()
	_, _ = rw.Store(oid, buf)

	reader, err := ro.Open(oid)
	assert.NoError(t, err)

	contents, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x1}, contents)
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
