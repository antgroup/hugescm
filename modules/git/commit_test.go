package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommitDecodeWithMultipleParents tests decoding with multiple parents
func TestCommitDecodeWithMultipleParents(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
parent a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
parent b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3
parent c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700

test message`
	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Equal(t, 3, len(commit.Parents))
}

// TestCommitDecodeWithSpecialCharacters tests decoding with special characters
func TestCommitDecodeWithSpecialCharacters(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author 张三 <zhangsan@example.com> 1337892984 +0800
committer 张三 <zhangsan@example.com> 1337892984 +0800
custom value with spaces & special!@#$%^&*()_+-=[]{}|;':",./<>?

test message with 中文 and 日本語`

	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Contains(t, commit.Author.String(), "张三")
	assert.Equal(t, 1, len(commit.ExtraHeaders))
	assert.Equal(t, "custom", commit.ExtraHeaders[0].K)
	assert.Equal(t, "value with spaces & special!@#$%^&*()_+-=[]{}|;':\",./<>?", commit.ExtraHeaders[0].V)
	assert.Contains(t, commit.Message, "中文")
	assert.Contains(t, commit.Message, "日本語")
}

// TestCommitDecodeWithExtraHeaderBeforeStandard tests extra header before standard headers
func TestCommitDecodeWithExtraHeaderBeforeStandard(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
custom extra header before standard
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700

test message`
	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Equal(t, 1, len(commit.ExtraHeaders))
	assert.Equal(t, "custom", commit.ExtraHeaders[0].K)
	assert.Equal(t, "extra header before standard", commit.ExtraHeaders[0].V)
}

// TestCommitDecodeWithComplexHeaders tests complex multi-line headers
func TestCommitDecodeWithComplexHeaders(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
parent b343c8beec664ef6f0e9964d3001c7c7966331ae
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
mergetag object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
 type commit
 tag random
 tagger J. Roe <jroe@example.ca> 1337889148 -0600

Random changes`

	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)

	// Verify ExtraHeaders
	require.Equal(t, 1, len(commit.ExtraHeaders))
	require.Equal(t, "mergetag", commit.ExtraHeaders[0].K)
	require.Contains(t, commit.ExtraHeaders[0].V, "object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd")
	require.Contains(t, commit.ExtraHeaders[0].V, "type commit")
	require.Contains(t, commit.ExtraHeaders[0].V, "tag random")
	require.Contains(t, commit.ExtraHeaders[0].V, "tagger J. Roe <jroe@example.ca> 1337889148 -0600")
}
