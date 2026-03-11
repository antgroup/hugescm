package gitobj

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"sort"
	"strconv"
	"testing"
)

func TestTreeReturnsCorrectObjectType(t *testing.T) {
	if TreeObjectType != new(Tree).Type() {
		t.Errorf("Expected %v, got %v", TreeObjectType, new(Tree).Type())
	}
}

func TestTreeEncoding(t *testing.T) {
	tree := &Tree{
		Entries: []*TreeEntry{
			{
				Name:     "a.dat",
				Oid:      []byte("aaaaaaaaaaaaaaaaaaaa"),
				Filemode: 0100644,
			},
			{
				Name:     "subdir",
				Oid:      []byte("bbbbbbbbbbbbbbbbbbbb"),
				Filemode: 040000,
			},
			{
				Name:     "submodule",
				Oid:      []byte("cccccccccccccccccccc"),
				Filemode: 0160000,
			},
		},
	}

	buf := new(bytes.Buffer)

	n, err := tree.Encode(buf)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if n == 0 {
		t.Errorf("Expected not equal")
	}

	assertTreeEntry(t, buf, "a.dat", []byte("aaaaaaaaaaaaaaaaaaaa"), 0100644)
	assertTreeEntry(t, buf, "subdir", []byte("bbbbbbbbbbbbbbbbbbbb"), 040000)
	assertTreeEntry(t, buf, "submodule", []byte("cccccccccccccccccccc"), 0160000)

	if buf.Len() != 0 {
		t.Errorf("Expected %v, got %v", 0, buf.Len())
	}
}

func TestTreeDecoding(t *testing.T) {
	from := new(bytes.Buffer)
	fmt.Fprintf(from, "%s %s\x00%s",
		strconv.FormatInt(int64(0100644), 8),
		"a.dat", []byte("aaaaaaaaaaaaaaaaaaaa"))
	fmt.Fprintf(from, "%s %s\x00%s",
		strconv.FormatInt(int64(040000), 8),
		"subdir", []byte("bbbbbbbbbbbbbbbbbbbb"))
	fmt.Fprintf(from, "%s %s\x00%s",
		strconv.FormatInt(int64(0120000), 8),
		"symlink", []byte("cccccccccccccccccccc"))
	fmt.Fprintf(from, "%s %s\x00%s",
		strconv.FormatInt(int64(0160000), 8),
		"submodule", []byte("dddddddddddddddddddd"))

	flen := from.Len()

	tree := new(Tree)
	n, err := tree.Decode(sha1.New(), from, int64(flen))

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if flen != n {
		t.Errorf("Expected %v, got %v", flen, n)
	}

	if len(tree.Entries) != 4 {
		t.Fatalf("Expected %v, got %v", 4, len(tree.Entries))
	}
	// Check a.dat
	if tree.Entries[0].Name != "a.dat" {
		t.Errorf("Expected 'a.dat', got %v", tree.Entries[0].Name)
	}
	if !bytes.Equal([]byte("aaaaaaaaaaaaaaaaaaaa"), tree.Entries[0].Oid) {
		t.Errorf("Expected aaaaaaaaaaaaaaaaaaaa, got %v", tree.Entries[0].Oid)
	}
	if tree.Entries[0].Filemode != 0100644 {
		t.Errorf("Expected 0100644, got %v", tree.Entries[0].Filemode)
	}
	// Check subdir
	if tree.Entries[1].Name != "subdir" {
		t.Errorf("Expected 'subdir', got %v", tree.Entries[1].Name)
	}
	if !bytes.Equal([]byte("bbbbbbbbbbbbbbbbbbbb"), tree.Entries[1].Oid) {
		t.Errorf("Expected bbbbbbbbbbbbbbbbbbbb, got %v", tree.Entries[1].Oid)
	}
	if tree.Entries[1].Filemode != 040000 {
		t.Errorf("Expected 040000, got %v", tree.Entries[1].Filemode)
	}
	// Check symlink
	if tree.Entries[2].Name != "symlink" {
		t.Errorf("Expected 'symlink', got %v", tree.Entries[2].Name)
	}
	if !bytes.Equal([]byte("cccccccccccccccccccc"), tree.Entries[2].Oid) {
		t.Errorf("Expected cccccccccccccccccccc, got %v", tree.Entries[2].Oid)
	}
	if tree.Entries[2].Filemode != 0120000 {
		t.Errorf("Expected 0120000, got %v", tree.Entries[2].Filemode)
	}
	// Check submodule
	if tree.Entries[3].Name != "submodule" {
		t.Errorf("Expected 'submodule', got %v", tree.Entries[3].Name)
	}
	if !bytes.Equal([]byte("dddddddddddddddddddd"), tree.Entries[3].Oid) {
		t.Errorf("Expected dddddddddddddddddddd, got %v", tree.Entries[3].Oid)
	}
	if tree.Entries[3].Filemode != 0160000 {
		t.Errorf("Expected 0160000, got %v", tree.Entries[3].Filemode)
	}
}

func TestTreeDecodingShaBoundary(t *testing.T) {
	var from bytes.Buffer

	fmt.Fprintf(&from, "%s %s\x00%s",
		strconv.FormatInt(int64(0100644), 8),
		"a.dat", []byte("aaaaaaaaaaaaaaaaaaaa"))

	flen := from.Len()

	tree := new(Tree)
	n, err := tree.Decode(sha1.New(), bufio.NewReaderSize(&from, flen-2), int64(flen))

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if flen != n {
		t.Errorf("Expected %v, got %v", flen, n)
	}

	if len(tree.Entries) != 1 {
		t.Fatalf("Expected len %v, got %v", 1, len(tree.Entries))
	}
	entry := tree.Entries[0]
	if entry.Name != "a.dat" {
		t.Errorf("Expected Name %v, got %v", "a.dat", entry.Name)
	}
	if !bytes.Equal(entry.Oid, []byte("aaaaaaaaaaaaaaaaaaaa")) {
		t.Errorf("Expected Oid %v, got %v", []byte("aaaaaaaaaaaaaaaaaaaa"), entry.Oid)
	}
	if entry.Filemode != 0100644 {
		t.Errorf("Expected Filemode %v, got %v", 0100644, entry.Filemode)
	}
}

func TestTreeMergeReplaceElements(t *testing.T) {
	e1 := &TreeEntry{Name: "a", Filemode: 0100644, Oid: []byte{0x1}}
	e2 := &TreeEntry{Name: "b", Filemode: 0100644, Oid: []byte{0x2}}
	e3 := &TreeEntry{Name: "c", Filemode: 0100755, Oid: []byte{0x3}}

	e4 := &TreeEntry{Name: "b", Filemode: 0100644, Oid: []byte{0x4}}
	e5 := &TreeEntry{Name: "c", Filemode: 0100644, Oid: []byte{0x5}}

	t1 := &Tree{Entries: []*TreeEntry{e1, e2, e3}}

	t2 := t1.Merge(e4, e5)

	if len(t1.Entries) != 3 {
		t.Fatalf("Expected len %v, got %v", 3, len(t1.Entries))
	}
	if !bytes.Equal(t1.Entries[0].Oid, []byte{0x1}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t1.Entries[1].Oid, []byte{0x2}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t1.Entries[2].Oid, []byte{0x3}) {
		t.Errorf("Expected true")
	}

	if len(t2.Entries) != 3 {
		t.Fatalf("Expected len %v, got %v", 3, len(t2.Entries))
	}
	if !bytes.Equal(t2.Entries[0].Oid, []byte{0x1}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t2.Entries[1].Oid, []byte{0x4}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t2.Entries[2].Oid, []byte{0x5}) {
		t.Errorf("Expected true")
	}
}

func TestMergeInsertElementsInSubtreeOrder(t *testing.T) {
	e1 := &TreeEntry{Name: "a-b", Filemode: 0100644, Oid: []byte{0x1}}
	e2 := &TreeEntry{Name: "a", Filemode: 040000, Oid: []byte{0x2}}
	e3 := &TreeEntry{Name: "a=", Filemode: 0100644, Oid: []byte{0x3}}
	e4 := &TreeEntry{Name: "a-", Filemode: 0100644, Oid: []byte{0x4}}

	t1 := &Tree{Entries: []*TreeEntry{e1, e2, e3}}
	t2 := t1.Merge(e4)

	if len(t1.Entries) != 3 {
		t.Fatalf("Expected len %v, got %v", 3, len(t1.Entries))
	}
	if !bytes.Equal(t1.Entries[0].Oid, []byte{0x1}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t1.Entries[1].Oid, []byte{0x2}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t1.Entries[2].Oid, []byte{0x3}) {
		t.Errorf("Expected true")
	}

	if len(t2.Entries) != 4 {
		t.Fatalf("Expected len %v, got %v", 4, len(t2.Entries))
	}
	if !bytes.Equal(t2.Entries[0].Oid, []byte{0x4}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t2.Entries[1].Oid, []byte{0x1}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t2.Entries[2].Oid, []byte{0x2}) {
		t.Errorf("Expected true")
	}
	if !bytes.Equal(t2.Entries[3].Oid, []byte{0x3}) {
		t.Errorf("Expected true")
	}
}

type TreeEntryTypeTestCase struct {
	Filemode int32
	Expected ObjectType
	IsLink   bool
}

func (c *TreeEntryTypeTestCase) AssertType(t *testing.T) {
	e := &TreeEntry{Filemode: c.Filemode}

	got := e.Type()

	if c.Expected != got {
		t.Errorf("git/object: expected type: %s, got: %s", c.Expected, got)
	}
}

func (c *TreeEntryTypeTestCase) AssertIsLink(t *testing.T) {
	e := &TreeEntry{Filemode: c.Filemode}

	isLink := e.IsLink()

	if c.IsLink != isLink {
		t.Errorf("git/object: expected link: %v, got: %v, for type %s", c.IsLink, isLink, c.Expected)
	}
}

func TestTreeEntryTypeResolution(t *testing.T) {
	for desc, c := range map[string]*TreeEntryTypeTestCase{
		"blob":    {0100644, BlobObjectType, false},
		"subtree": {040000, TreeObjectType, false},
		"symlink": {0120000, BlobObjectType, true},
		"commit":  {0160000, CommitObjectType, false},
	} {
		t.Run(desc, c.AssertType)
		t.Run(desc, c.AssertIsLink)
	}
}

func TestSubtreeOrder(t *testing.T) {
	// The below list (e1, e2, ..., e5) is entered in subtree order: that
	// is, lexicographically byte-ordered as if blobs end in a '\0', and
	// sub-trees end in a '/'.
	//
	// See:
	//   http://public-inbox.org/git/7vac6jfzem.fsf@assigned-by-dhcp.cox.net
	e1 := &TreeEntry{Filemode: 0100644, Name: "a-"}
	e2 := &TreeEntry{Filemode: 0100644, Name: "a-b"}
	e3 := &TreeEntry{Filemode: 040000, Name: "a"}
	e4 := &TreeEntry{Filemode: 0100644, Name: "a="}
	e5 := &TreeEntry{Filemode: 0100644, Name: "a=b"}

	// Create a set of entries in the wrong order:
	entries := []*TreeEntry{e3, e4, e1, e5, e2}

	sort.Sort(SubtreeOrder(entries))

	// Assert that they are in the correct order after sorting in sub-tree
	// order:
	if len(entries) != 5 {
		t.Fatalf("Expected len %v, got %v", 5, len(entries))
	}
	if entries[0].Name != "a-" {
		t.Errorf("Expected %v, got %v", "a-", entries[0].Name)
	}
	if entries[1].Name != "a-b" {
		t.Errorf("Expected %v, got %v", "a-b", entries[1].Name)
	}
	if entries[2].Name != "a" {
		t.Errorf("Expected %v, got %v", "a", entries[2].Name)
	}
	if entries[3].Name != "a=" {
		t.Errorf("Expected %v, got %v", "a=", entries[3].Name)
	}
	if entries[4].Name != "a=b" {
		t.Errorf("Expected %v, got %v", "a=b", entries[4].Name)
	}
}

func TestSubtreeOrderReturnsEmptyForOutOfBounds(t *testing.T) {
	o := SubtreeOrder([]*TreeEntry{{Name: "a"}})

	result := o.Name(len(o) + 1)
	if result != "" {
		t.Errorf("Expected %v, got %v", "", result)
	}
}

func TestSubtreeOrderReturnsEmptyForNilElements(t *testing.T) {
	o := SubtreeOrder([]*TreeEntry{nil})

	if o.Name(0) != "" {
		t.Errorf("Expected %v, got %v", "", o.Name(0))
	}
}

func TestTreeEqualReturnsTrueWithUnchangedContents(t *testing.T) {
	t1 := &Tree{Entries: []*TreeEntry{
		{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)},
	}}
	t2 := &Tree{Entries: []*TreeEntry{
		{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)},
	}}

	if !t1.Equal(t2) {
		t.Errorf("Expected true")
	}
}

func TestTreeEqualReturnsFalseWithChangedContents(t *testing.T) {
	t1 := &Tree{Entries: []*TreeEntry{
		{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)},
		{Name: "b.dat", Filemode: 0100644, Oid: make([]byte, 20)},
	}}
	t2 := &Tree{Entries: []*TreeEntry{
		{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)},
		{Name: "c.dat", Filemode: 0100644, Oid: make([]byte, 20)},
	}}

	if t1.Equal(t2) {
		t.Errorf("Expected false")
	}
}

func TestTreeEqualReturnsTrueWhenOneTreeIsNil(t *testing.T) {
	t1 := &Tree{Entries: []*TreeEntry{
		{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)},
	}}
	t2 := (*Tree)(nil)

	if t1.Equal(t2) {
		t.Errorf("Expected false")
	}
	if t2.Equal(t1) {
		t.Errorf("Expected false")
	}
}

func TestTreeEqualReturnsTrueWhenBothTreesAreNil(t *testing.T) {
	t1 := (*Tree)(nil)
	t2 := (*Tree)(nil)

	if !t1.Equal(t2) {
		t.Errorf("Expected true")
	}
}

func TestTreeEntryEqualReturnsTrueWhenEntriesAreTheSame(t *testing.T) {
	e1 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}
	e2 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}

	if !e1.Equal(e2) {
		t.Errorf("Expected true")
	}
}

func TestTreeEntryEqualReturnsFalseWhenDifferentNames(t *testing.T) {
	e1 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}
	e2 := &TreeEntry{Name: "b.dat", Filemode: 0100644, Oid: make([]byte, 20)}

	if e1.Equal(e2) {
		t.Errorf("Expected false")
	}
}

func TestTreeEntryEqualReturnsFalseWhenDifferentOids(t *testing.T) {
	e1 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}
	e2 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}

	e2.Oid[0] = 1

	if e1.Equal(e2) {
		t.Errorf("Expected false")
	}
}

func TestTreeEntryEqualReturnsFalseWhenDifferentFilemodes(t *testing.T) {
	e1 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}
	e2 := &TreeEntry{Name: "a.dat", Filemode: 0100755, Oid: make([]byte, 20)}

	if e1.Equal(e2) {
		t.Errorf("Expected false")
	}
}

func TestTreeEntryEqualReturnsFalseWhenOneEntryIsNil(t *testing.T) {
	e1 := &TreeEntry{Name: "a.dat", Filemode: 0100644, Oid: make([]byte, 20)}
	e2 := (*TreeEntry)(nil)

	if e1.Equal(e2) {
		t.Errorf("Expected false")
	}
}

func TestTreeEntryEqualReturnsTrueWhenBothEntriesAreNil(t *testing.T) {
	e1 := (*TreeEntry)(nil)
	e2 := (*TreeEntry)(nil)

	if !e1.Equal(e2) {
		t.Errorf("Expected true")
	}
}

func assertTreeEntry(t *testing.T, buf *bytes.Buffer,
	name string, oid []byte, mode int32) {

	fmode, err := buf.ReadBytes(' ')
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	expectedFmode := []byte(strconv.FormatInt(int64(mode), 8) + " ")
	if !bytes.Equal(expectedFmode, fmode) {
		t.Errorf("Expected %v, got %v", expectedFmode, fmode)
	}

	fname, err := buf.ReadBytes('\x00')
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte(name+"\x00"), fname) {
		t.Errorf("Expected %v, got %v", []byte(name+"\x00"), fname)
	}

	var sha [20]byte
	_, err = buf.Read(sha[:])
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal(oid, sha[:]) {
		t.Errorf("Expected %v, got %v", oid, sha[:])
	}
}
