package gitobj

import (
	"bytes"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/git/gitobj/errors"
	"github.com/stretchr/testify/assert"
)

func TestMemoryStorerIncludesGivenEntries(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hex, err := hex.DecodeString(sha)

	assert.Nil(t, err)

	ms := newMemoryStorer(map[string]io.ReadWriter{
		sha: bytes.NewBuffer([]byte{0x1}),
	})

	buf, err := ms.Open(hex)
	assert.Nil(t, err)

	contents, err := io.ReadAll(buf)
	assert.Nil(t, err)
	assert.Equal(t, []byte{0x1}, contents)
}

func TestMemoryStorerAcceptsNilEntries(t *testing.T) {
	ms := newMemoryStorer(nil)

	assert.NotNil(t, ms)
	assert.Equal(t, 0, len(ms.fs))
	assert.NoError(t, ms.Close())
}

func TestMemoryStorerDoesntOpenMissingEntries(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	hex, err := hex.DecodeString(sha)
	assert.Nil(t, err)

	ms := newMemoryStorer(nil)

	f, err := ms.Open(hex)
	assert.Equal(t, errors.NoSuchObject(hex), err)
	assert.Nil(t, f)
}

func TestMemoryStorerStoresNewEntries(t *testing.T) {
	hex, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	assert.Nil(t, err)

	ms := newMemoryStorer(nil)

	assert.Equal(t, 0, len(ms.fs))

	_, err = ms.Store(hex, strings.NewReader("hello"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ms.fs))

	got, err := ms.Open(hex)
	assert.Nil(t, err)

	contents, err := io.ReadAll(got)
	assert.Nil(t, err)
	assert.Equal(t, "hello", string(contents))
}

func TestMemoryStorerStoresExistingEntries(t *testing.T) {
	hex, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	assert.Nil(t, err)

	ms := newMemoryStorer(nil)

	assert.Equal(t, 0, len(ms.fs))

	_, err = ms.Store(hex, new(bytes.Buffer))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ms.fs))

	n, err := ms.Store(hex, new(bytes.Buffer))
	assert.Nil(t, err)
	assert.EqualValues(t, 0, n)
}
