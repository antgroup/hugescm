package errors

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoSuchObjectTypeErrFormatting(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oid, err := hex.DecodeString(sha)
	assert.NoError(t, err)

	err = NoSuchObject(oid)

	assert.Equal(t, "git/object: no such object: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", err.Error())
	assert.Equal(t, IsNoSuchObject(err), true)
}

func TestIsNoSuchObjectNilHandling(t *testing.T) {
	assert.Equal(t, IsNoSuchObject((*noSuchObject)(nil)), false)
	assert.Equal(t, IsNoSuchObject(nil), false)
}
