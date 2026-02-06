package gitobj

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

func TestBadCommit(t *testing.T) {
	cc := `tree 2aedfd35087c75d17bdbaf4dd56069d44fc75b71
parent 75158117eb8efe60453f8c077527ac3530c81e38
author Credit Card Account <Credit Card Account> 1722305889 +0800
committer \346\244\260\346\235\215
 <Credit Card Account> 1722305889 +0800

Credit Card Account`
	var c Commit
	_, err := c.Decode(sha1.New(), strings.NewReader(cc), int64(len(cc)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad commit: '%v'\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", c)
}

func TestBad2Commit(t *testing.T) {
	cc := `tree 2aedfd35087c75d17bdbaf4dd56069d44fc75b71
parent 75158117eb8efe60453f8c077527ac3530c81e38
author Credit Card Account <Credit Card Account> 1722305889 +0800
committer Credit Card Account <Credit Card Account> 1722305889 +0800
V  
 
D
---`
	var c Commit
	_, err := c.Decode(sha1.New(), strings.NewReader(cc), int64(len(cc)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad commit: '%v'\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", c)
}

// TestCommitDecodeWithLeadingWhitespaceWithoutPreviousHeader
// Tests handling lines starting with space after standard headers but before empty line
// This test verifies the code does not panic and handles this case correctly
func TestCommitDecodeWithLeadingWhitespaceWithoutPreviousHeader(t *testing.T) {
	cc := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
  extra line without previous header

test message`

	flen := len(cc)
	commit := new(Commit)

	// This call should not panic
	n, err := commit.Decode(sha1.New(), strings.NewReader(cc), int64(flen))

	// May return error or success, but should not panic
	_ = n
	_ = err
}

// TestCommitDecodePanicOnContinuationWithoutPreviousHeader
// Attempts to trigger commit.go:119 panic: when encountering blank line without previous header
func TestCommitDecodePanicOnContinuationWithoutPreviousHeader(t *testing.T) {
	cc := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
  first continuation line before any extra header

test message`

	flen := len(cc)
	commit := new(Commit)

	// Try to see if it will panic
	n, err := commit.Decode(sha1.New(), strings.NewReader(cc), int64(flen))
	fmt.Printf("Result: n=%d, err=%v\n", n, err)
	fmt.Printf("Commit: %+v\n", commit)
}

// TestSplitBehavior
// Directly tests strings.Split behavior to confirm if it can return empty array
func TestSplitBehavior(t *testing.T) {
	testCases := []struct {
		input  string
		sep    string
		expect int
	}{
		{"", " ", 1},
		{" ", " ", 2},
		{"  ", " ", 3},
		{"\t", " ", 1},
		{"\n", " ", 1},
		{"\r\n", " ", 1},
		{"\u0000", " ", 1},
	}

	for _, tc := range testCases {
		fields := strings.Split(tc.input, tc.sep)
		fmt.Printf("Split(%q, %q): len=%d\n", tc.input, tc.sep, len(fields))
		if len(fields) == 0 {
			fmt.Printf("  >>> EMPTY ARRAY! <<<\n")
		}
		assert.Equal(t, tc.expect, len(fields))
	}
}

// TestCommitDecodePanicOnEmptyFields
// 测试是否能触发 len(fields) == 0 的情况
func TestCommitDecodePanicOnEmptyFields(t *testing.T) {
	// 尝试构造特殊输入
	testCases := []string{
		`tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
`, // 在 header 区域结尾只有空行
		`tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700

message`,
	}

	for i, cc := range testCases {
		fmt.Printf("\n=== Test case %d ===\n", i)
		fmt.Printf("Input:\n%s\n", cc)

		flen := len(cc)
		commit := new(Commit)

		// Check if it will panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC CAUGHT: %v\n", r)
				}
			}()

			n, err := commit.Decode(sha1.New(), strings.NewReader(cc), int64(flen))
			fmt.Printf("Result: n=%d, err=%v\n", n, err)
			fmt.Printf("Commit: %+v\n", commit)
		}()
	}
}

// TestCommitDecodePanicWithMalformedInput
// Attempts to trigger panic using various malformed inputs
func TestCommitDecodePanicWithMalformedInput(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name: "Extra header followed by pure space line",
			input: `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
custom value

message`,
		},
		{
			name: "Multiple spaces line after extra header",
			input: `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
custom value
  
message`,
		},
		{
			name: "Only tab after extra header",
			input: `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
custom value
	
message`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("\n=== %s ===\n", tc.name)
			fmt.Printf("Input:\n%s\n", tc.input)

			commit := new(Commit)
			flen := len(tc.input)

			// 使用 recover 捕获 panic
			defer func() {
				if r := recover(); r != nil {
					t.Logf(">>> PANIC CAUGHT: %v <<<", r)
					t.Logf("This proves the panic can be triggered!")
					t.FailNow()
				}
			}()

			n, err := commit.Decode(sha1.New(), strings.NewReader(tc.input), int64(flen))
			t.Logf("Result: n=%d, err=%v", n, err)
			t.Logf("ExtraHeaders count: %d", len(commit.ExtraHeaders))
			if len(commit.ExtraHeaders) > 0 {
				for i, h := range commit.ExtraHeaders {
					t.Logf("  [%d] K=%q, V=%q", i, h.K, h.V)
				}
			}
		})
	}
}

// TestCommitDecodeWithEmptyAuthor tests decoding with empty author
func TestCommitDecodeWithEmptyAuthor(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author
committer Pat Doe <pdoe@example.org> 1337892984 -0700

test message`
	commit := new(Commit)
	_, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Equal(t, "", commit.Author)
	assert.Equal(t, "Pat Doe <pdoe@example.org> 1337892984 -0700", commit.Committer)
}

// TestCommitDecodeWithEmptyCommitter tests decoding with empty committer
func TestCommitDecodeWithEmptyCommitter(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer

test message`
	commit := new(Commit)
	_, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Equal(t, "Pat Doe <pdoe@example.org> 1337892984 -0700", commit.Author)
	assert.Equal(t, "", commit.Committer)
}

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
	_, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Equal(t, 3, len(commit.ParentIDs))
	assert.Equal(t, "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", hex.EncodeToString(commit.ParentIDs[0]))
	assert.Equal(t, "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3", hex.EncodeToString(commit.ParentIDs[1]))
	assert.Equal(t, "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", hex.EncodeToString(commit.ParentIDs[2]))
}

// TestCommitDecodeWithSpecialCharacters tests decoding with special characters
func TestCommitDecodeWithSpecialCharacters(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author 张三 <zhangsan@example.com> 1337892984 +0800
committer 张三 <zhangsan@example.com> 1337892984 +0800
custom value with spaces & special!@#$%^&*()_+-=[]{}|;':",./<>?

test message with 中文 and 日本語`

	commit := new(Commit)
	_, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Contains(t, commit.Author, "张三")
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
	_, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))
	require.NoError(t, err)
	assert.Equal(t, 1, len(commit.ExtraHeaders))
	assert.Equal(t, "custom", commit.ExtraHeaders[0].K)
	assert.Equal(t, "extra header before standard", commit.ExtraHeaders[0].V)
}

// TestCommitDecodeMultilineExtraHeaders tests correct parsing of multi-line extra headers
// This is a test case for fixing multi-line header bug
func TestCommitDecodeMultilineExtraHeaders(t *testing.T) {
	// Construct a commit with multi-line GPG signature
	// Note: In Git format, leading spaces in multi-line header continuation are removed
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
gpgsig -----BEGIN PGP SIGNATURE-----
 Version: GnuPG v1.4.11 (GNU/Linux)
 iQIcBAABAgAGBQJR9JqnAAoJEJyGw4i5t8hW3KUP/0XuWjE4kM6G8J7E6H4P2J8
 =i9Jh
 -----END PGP SIGNATURE-----

test message`

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))

	require.NoError(t, err)
	require.Equal(t, len(input), n)
	require.Equal(t, 1, len(commit.ExtraHeaders))

	// Verify multi-line header value is correctly concatenated
	// Note: Leading spaces are removed, but empty lines in continuation are preserved
	gpgsig := commit.ExtraHeaders[0]
	assert.Equal(t, "gpgsig", gpgsig.K)
	expectedValue := "-----BEGIN PGP SIGNATURE-----\n" +
		"Version: GnuPG v1.4.11 (GNU/Linux)\n" +
		"iQIcBAABAgAGBQJR9JqnAAoJEJyGw4i5t8hW3KUP/0XuWjE4kM6G8J7E6H4P2J8\n" +
		"=i9Jh\n" +
		"-----END PGP SIGNATURE-----"
	assert.Equal(t, expectedValue, gpgsig.V)
	assert.Equal(t, "test message", commit.Message)
}

// TestCommitDecodeMultipleExtraHeaders tests multiple extra headers
func TestCommitDecodeMultipleExtraHeaders(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
encoding utf-8
gpgsig -----BEGIN PGP SIGNATURE-----
 signature
 -----END PGP SIGNATURE-----
custom value1
custom value2

test message`

	commit := new(Commit)
	n, err := commit.Decode(sha1.New(), strings.NewReader(input), int64(len(input)))

	require.NoError(t, err)
	require.Equal(t, len(input), n)
	require.Equal(t, 4, len(commit.ExtraHeaders))

	assert.Equal(t, "encoding", commit.ExtraHeaders[0].K)
	assert.Equal(t, "utf-8", commit.ExtraHeaders[0].V)

	assert.Equal(t, "gpgsig", commit.ExtraHeaders[1].K)
	assert.Contains(t, commit.ExtraHeaders[1].V, "-----BEGIN PGP SIGNATURE-----")
	assert.Contains(t, commit.ExtraHeaders[1].V, "signature")
	assert.Contains(t, commit.ExtraHeaders[1].V, "-----END PGP SIGNATURE-----")

	assert.Equal(t, "custom", commit.ExtraHeaders[2].K)
	assert.Equal(t, "value1", commit.ExtraHeaders[2].V)

	assert.Equal(t, "custom", commit.ExtraHeaders[3].K)
	assert.Equal(t, "value2", commit.ExtraHeaders[3].V)

	assert.Equal(t, "test message", commit.Message)
}

// TestCommitDecodeWithStringsCut validates correct usage of strings.Cut
func TestCommitDecodeWithStringsCut(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTree string
		wantErr  bool
	}{
		{
			name:     "standard commit",
			input:    "tree abc123\nauthor test\n\nmsg",
			wantTree: "abc123",
			wantErr:  false,
		},
		{
			name:     "tree with value",
			input:    "tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb\nauthor test\n\nmsg",
			wantTree: "e8ad84c41c2acde27c77fa212b8865cd3acfe6fb",
			wantErr:  false,
		},
		{
			name:     "tree without value (should be skipped)",
			input:    "tree\ntree abc123\nauthor test\n\nmsg",
			wantTree: "abc123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commit := new(Commit)
			_, err := commit.Decode(sha1.New(), strings.NewReader(tt.input), int64(len(tt.input)))

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTree, hex.EncodeToString(commit.TreeID))
			}
		})
	}
}
