package gitobj

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitReturnsCorrectObjectType(t *testing.T) {
	assert.Equal(t, CommitObjectType, new(Commit).Type())
}

func TestCommitEncoding(t *testing.T) {
	author := &Signature{Name: "John Doe", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "Jane Doe", Email: "jane@example.com", When: time.Now()}

	sig := "-----BEGIN PGP SIGNATURE-----\n<signature>\n-----END PGP SIGNATURE-----"

	c := &Commit{
		Author:    author.String(),
		Committer: committer.String(),
		ParentIDs: [][]byte{
			[]byte("aaaaaaaaaaaaaaaaaaaa"), []byte("bbbbbbbbbbbbbbbbbbbb"),
		},
		TreeID: []byte("cccccccccccccccccccc"),
		ExtraHeaders: []*ExtraHeader{
			{"foo", "bar"},
			{"gpgsig", sig},
		},
		Message: "initial commit",
	}

	buf := new(bytes.Buffer)

	_, err := c.Encode(buf)
	assert.Nil(t, err)

	assertLine(t, buf, "tree 6363636363636363636363636363636363636363")
	assertLine(t, buf, "parent 6161616161616161616161616161616161616161")
	assertLine(t, buf, "parent 6262626262626262626262626262626262626262")
	assertLine(t, buf, "author %s", author.String())
	assertLine(t, buf, "committer %s", committer.String())
	assertLine(t, buf, "foo bar")
	assertLine(t, buf, "gpgsig -----BEGIN PGP SIGNATURE-----")
	assertLine(t, buf, " <signature>")
	assertLine(t, buf, " -----END PGP SIGNATURE-----")
	assertLine(t, buf, "")
	assertLine(t, buf, "initial commit")

	assert.Equal(t, 0, buf.Len())
}

func TestCommitDecoding(t *testing.T) {
	author := &Signature{Name: "John Doe", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "Jane Doe", Email: "jane@example.com", When: time.Now()}

	p1 := []byte("aaaaaaaaaaaaaaaaaaaa")
	p2 := []byte("bbbbbbbbbbbbbbbbbbbb")
	treeId := []byte("cccccccccccccccccccc")

	from := new(bytes.Buffer)
	fmt.Fprintf(from, "author %s\n", author)
	fmt.Fprintf(from, "committer %s\n", committer)
	fmt.Fprintf(from, "parent %s\n", hex.EncodeToString(p1))
	fmt.Fprintf(from, "parent %s\n", hex.EncodeToString(p2))
	fmt.Fprintf(from, "foo bar\n")
	fmt.Fprintf(from, "tree %s\n", hex.EncodeToString(treeId))
	fmt.Fprintf(from, "\ninitial commit")

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	assert.Nil(t, err)
	assert.Equal(t, flen, n)

	assert.Equal(t, author.String(), commit.Author)
	assert.Equal(t, committer.String(), commit.Committer)
	assert.Equal(t, [][]byte{p1, p2}, commit.ParentIDs)
	assert.Equal(t, 1, len(commit.ExtraHeaders))
	assert.Equal(t, "foo", commit.ExtraHeaders[0].K)
	assert.Equal(t, "bar", commit.ExtraHeaders[0].V)
	assert.Equal(t, "initial commit", commit.Message)
}

func TestCommitDecodingWithEmptyName(t *testing.T) {
	author := &Signature{Name: "", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "", Email: "jane@example.com", When: time.Now()}

	treeId := []byte("cccccccccccccccccccc")

	from := new(bytes.Buffer)

	fmt.Fprintf(from, "author %s\n", author)
	fmt.Fprintf(from, "committer %s\n", committer)
	fmt.Fprintf(from, "tree %s\n", hex.EncodeToString(treeId))
	fmt.Fprintf(from, "\ninitial commit")

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	assert.Nil(t, err)
	assert.Equal(t, flen, n)

	assert.Equal(t, author.String(), commit.Author)
	assert.Equal(t, committer.String(), commit.Committer)
	assert.Equal(t, "initial commit", commit.Message)
}

func TestCommitDecodingWithLargeCommitMessage(t *testing.T) {
	message := "This message text is, with newline, exactly 64 characters long. "
	// This message will be exactly 10 MiB in size when part of the commit.
	longMessage := strings.Repeat(message, (10*1024*1024/64)-1)
	longMessage += strings.TrimSpace(message)

	author := &Signature{Name: "", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "", Email: "jane@example.com", When: time.Now()}

	treeId := []byte("cccccccccccccccccccc")

	from := new(bytes.Buffer)

	fmt.Fprintf(from, "author %s\n", author)
	fmt.Fprintf(from, "committer %s\n", committer)
	fmt.Fprintf(from, "tree %s\n", hex.EncodeToString(treeId))
	fmt.Fprintf(from, "\n%s", longMessage)

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	assert.Nil(t, err)
	assert.Equal(t, flen, n)

	assert.Equal(t, author.String(), commit.Author)
	assert.Equal(t, committer.String(), commit.Committer)
	assert.Equal(t, longMessage, commit.Message)
}

func TestCommitDecodingWithMessageKeywordPrefix(t *testing.T) {
	author := &Signature{Name: "John Doe", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "Jane Doe", Email: "jane@example.com", When: time.Now()}

	treeId := []byte("aaaaaaaaaaaaaaaaaaaa")
	treeIdAscii := hex.EncodeToString(treeId)

	from := new(bytes.Buffer)
	fmt.Fprintf(from, "author %s\n", author)
	fmt.Fprintf(from, "committer %s\n", committer)
	fmt.Fprintf(from, "tree %s\n", hex.EncodeToString(treeId))
	fmt.Fprintf(from, "\nfirst line\n\nsecond line")

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	assert.NoError(t, err)
	assert.Equal(t, flen, n)

	assert.Equal(t, author.String(), commit.Author)
	assert.Equal(t, committer.String(), commit.Committer)
	assert.Equal(t, treeIdAscii, hex.EncodeToString(commit.TreeID))
	assert.Equal(t, "first line\n\nsecond line", commit.Message)
}

func TestCommitDecodingWithWhitespace(t *testing.T) {
	author := &Signature{Name: "John Doe", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "Jane Doe", Email: "jane@example.com", When: time.Now()}

	treeId := []byte("aaaaaaaaaaaaaaaaaaaa")
	treeIdAscii := hex.EncodeToString(treeId)

	from := new(bytes.Buffer)
	fmt.Fprintf(from, "author %s\n", author)
	fmt.Fprintf(from, "committer %s\n", committer)
	fmt.Fprintf(from, "tree %s\n", hex.EncodeToString(treeId))
	fmt.Fprintf(from, "\ntree <- initial commit")

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	assert.NoError(t, err)
	assert.Equal(t, flen, n)

	assert.Equal(t, author.String(), commit.Author)
	assert.Equal(t, committer.String(), commit.Committer)
	assert.Equal(t, treeIdAscii, hex.EncodeToString(commit.TreeID))
	assert.Equal(t, "tree <- initial commit", commit.Message)
}

func TestCommitDecodingMultilineHeader(t *testing.T) {
	author := &Signature{Name: "", Email: "john@example.com", When: time.Now()}
	committer := &Signature{Name: "", Email: "jane@example.com", When: time.Now()}

	treeId := []byte("cccccccccccccccccccc")

	from := new(bytes.Buffer)

	fmt.Fprintf(from, "author %s\n", author)
	fmt.Fprintf(from, "committer %s\n", committer)
	fmt.Fprintf(from, "tree %s\n", hex.EncodeToString(treeId))
	fmt.Fprintf(from, "gpgsig -----BEGIN PGP SIGNATURE-----\n")
	fmt.Fprintf(from, " <signature>\n")
	fmt.Fprintf(from, " -----END PGP SIGNATURE-----\n")
	fmt.Fprintf(from, "\ninitial commit")

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	require.Nil(t, err)
	require.Equal(t, flen, n)
	require.Len(t, commit.ExtraHeaders, 1)

	hdr := commit.ExtraHeaders[0]

	assert.Equal(t, "gpgsig", hdr.K)
	assert.EqualValues(t, []string{
		"-----BEGIN PGP SIGNATURE-----",
		"<signature>",
		"-----END PGP SIGNATURE-----"},
		strings.Split(hdr.V, "\n"))
}

func TestCommitDecodingBadMessageWithLineStartingWithTree(t *testing.T) {
	from := new(bytes.Buffer)

	// The tricky part here that we're testing is the "tree support" in the
	// `mergetag` header, which we should not try to parse as a tree header.
	// Note also that this entry contains trailing whitespace which must not
	// be trimmed.
	fmt.Fprintf(from, `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
parent b343c8beec664ef6f0e9964d3001c7c7966331ae
parent 
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
mergetag object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
 type commit
 tag random
 tagger J. Roe <jroe@example.ca> 1337889148 -0600
 
 Random changes
 
 This text contains some
 tree support code.
 -----BEGIN PGP SIGNATURE-----
 Version: GnuPG v1.4.11 (GNU/Linux)
 
 Not a real signature
 -----END PGP SIGNATURE-----

Merge tag 'random' of git://git.example.ca/git/
`)

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	require.Nil(t, err)
	require.Equal(t, flen, n)
	require.Equal(t, commit.ExtraHeaders, []*ExtraHeader{
		{
			K: "mergetag",
			V: `object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
type commit
tag random
tagger J. Roe <jroe@example.ca> 1337889148 -0600

Random changes

This text contains some
tree support code.
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.11 (GNU/Linux)

Not a real signature
-----END PGP SIGNATURE-----`},
	},
	)
	require.Equal(t, commit.Message, "Merge tag 'random' of git://git.example.ca/git/\n")
}

func TestCommitDecodingMessageWithLineStartingWithTree(t *testing.T) {
	from := new(bytes.Buffer)

	// The tricky part here that we're testing is the "tree support" in the
	// `mergetag` header, which we should not try to parse as a tree header.
	// Note also that this entry contains trailing whitespace which must not
	// be trimmed.
	fmt.Fprintf(from, `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
parent b343c8beec664ef6f0e9964d3001c7c7966331ae
parent 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
mergetag object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
 type commit
 tag random
 tagger J. Roe <jroe@example.ca> 1337889148 -0600
 
 Random changes
 
 This text contains some
 tree support code.
 -----BEGIN PGP SIGNATURE-----
 Version: GnuPG v1.4.11 (GNU/Linux)
 
 Not a real signature
 -----END PGP SIGNATURE-----

Merge tag 'random' of git://git.example.ca/git/
`)

	flen := from.Len()

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), from, int64(flen))

	require.Nil(t, err)
	require.Equal(t, flen, n)
	require.Equal(t, commit.ExtraHeaders, []*ExtraHeader{
		{
			K: "mergetag",
			V: `object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
type commit
tag random
tagger J. Roe <jroe@example.ca> 1337889148 -0600

Random changes

This text contains some
tree support code.
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.11 (GNU/Linux)

Not a real signature
-----END PGP SIGNATURE-----`},
	},
	)
	require.Equal(t, commit.Message, "Merge tag 'random' of git://git.example.ca/git/\n")
}

func assertLine(t *testing.T, buf *bytes.Buffer, wanted string, args ...any) {
	got, err := buf.ReadString('\n')
	if err == io.EOF {
		err = nil
	}

	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf(wanted, args...), strings.TrimSuffix(got, "\n"))
}

func TestCommitEqualReturnsTrueWithIdenticalCommits(t *testing.T) {
	c1 := &Commit{
		Author:    "Jane Doe <jane@example.com> 1503956287 -0400",
		Committer: "Jane Doe <jane@example.com> 1503956287 -0400",
		ParentIDs: [][]byte{make([]byte, 20)},
		TreeID:    make([]byte, 20),
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Joe Smith"},
		},
		Message: "initial commit",
	}
	c2 := &Commit{
		Author:    "Jane Doe <jane@example.com> 1503956287 -0400",
		Committer: "Jane Doe <jane@example.com> 1503956287 -0400",
		ParentIDs: [][]byte{make([]byte, 20)},
		TreeID:    make([]byte, 20),
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Joe Smith"},
		},
		Message: "initial commit",
	}

	assert.True(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentParentCounts(t *testing.T) {
	c1 := &Commit{
		ParentIDs: [][]byte{make([]byte, 20), make([]byte, 20)},
	}
	c2 := &Commit{
		ParentIDs: [][]byte{make([]byte, 20)},
	}

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentParentsIds(t *testing.T) {
	c1 := &Commit{
		ParentIDs: [][]byte{make([]byte, 20)},
	}
	c2 := &Commit{
		ParentIDs: [][]byte{make([]byte, 20)},
	}

	c1.ParentIDs[0][1] = 0x1

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentHeaderCounts(t *testing.T) {
	c1 := &Commit{
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Joe Smith"},
			{K: "GPG-Signature", V: "..."},
		},
	}
	c2 := &Commit{
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Joe Smith"},
		},
	}

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentHeaders(t *testing.T) {
	c1 := &Commit{
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Joe Smith"},
		},
	}
	c2 := &Commit{
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Jane Smith"},
		},
	}

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentAuthors(t *testing.T) {
	c1 := &Commit{
		Author: "Jane Doe <jane@example.com> 1503956287 -0400",
	}
	c2 := &Commit{
		Author: "John Doe <john@example.com> 1503956287 -0400",
	}

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentCommitters(t *testing.T) {
	c1 := &Commit{
		Committer: "Jane Doe <jane@example.com> 1503956287 -0400",
	}
	c2 := &Commit{
		Committer: "John Doe <john@example.com> 1503956287 -0400",
	}

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentMessages(t *testing.T) {
	c1 := &Commit{
		Message: "initial commit",
	}
	c2 := &Commit{
		Message: "not the initial commit",
	}

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWithDifferentTreeIDs(t *testing.T) {
	c1 := &Commit{
		TreeID: make([]byte, 20),
	}
	c2 := &Commit{
		TreeID: make([]byte, 20),
	}

	c1.TreeID[0] = 0x1

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsFalseWhenOneCommitIsNil(t *testing.T) {
	c1 := &Commit{
		Author:    "Jane Doe <jane@example.com> 1503956287 -0400",
		Committer: "Jane Doe <jane@example.com> 1503956287 -0400",
		ParentIDs: [][]byte{make([]byte, 20)},
		TreeID:    make([]byte, 20),
		ExtraHeaders: []*ExtraHeader{
			{K: "Signed-off-by", V: "Joe Smith"},
		},
		Message: "initial commit",
	}
	c2 := (*Commit)(nil)

	assert.False(t, c1.Equal(c2))
}

func TestCommitEqualReturnsTrueWhenBothCommitsAreNil(t *testing.T) {
	c1 := (*Commit)(nil)
	c2 := (*Commit)(nil)

	assert.True(t, c1.Equal(c2))
}
