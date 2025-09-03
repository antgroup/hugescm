package git

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// We define these here instead of using the system ones because not all
// operating systems use the traditional values.  For example, zOS uses
// different values.
const (
	sIFMT      = FileMode(0170000)
	sIFREG     = FileMode(0100000)
	sIFDIR     = FileMode(0040000)
	sIFLNK     = FileMode(0120000)
	sIFGITLINK = FileMode(0160000)
)

// Tree encapsulates a Git tree object.
type Tree struct {
	// Hash of the tree object.
	Hash string `json:"hash"`
	// Entries is the list of entries held by this tree.
	Entries []*TreeEntry `json:"entries"`
	size    int64
}

// TreeEntry encapsulates information about a single tree entry in a tree
// listing.
type TreeEntry struct {
	// Name is the entry name relative to the tree in which this entry is
	// contained.
	Name string `json:"name"`
	// Hash is the object ID (Hex) for this tree entry.
	Hash string `json:"hash"`
	// Filemode is the filemode of this tree entry on disk.
	Filemode FileMode `json:"mode"`
}

func (t *Tree) Size() int64 {
	return t.size
}

// Decode implements Object.Decode and decodes the uncompressed tree being
// read. It returns the number of uncompressed bytes being consumed off of the
// stream, which should be strictly equal to the size given.
//
// If any error was encountered along the way, that will be returned, along with
// the number of bytes read up to that point.
func (t *Tree) Decode(hash string, from io.Reader, size int64) (n int, err error) {
	t.Hash = hash
	t.size = size
	buf := bufio.NewReader(from)
	hashSize := len(t.Hash) / 2
	var entries []*TreeEntry
	for {
		modes, err := buf.ReadString(' ')
		if err != nil {
			if err == io.EOF {
				break
			}
			return n, err
		}
		n += len(modes)
		modes = strings.TrimSuffix(modes, " ")

		mode, _ := strconv.ParseInt(modes, 8, 32)

		fname, err := buf.ReadString('\x00')
		if err != nil {
			return n, err
		}
		n += len(fname)
		fname = strings.TrimSuffix(fname, "\x00")

		var sha [GIT_SHA256_RAWSZ]byte
		if _, err = io.ReadFull(buf, sha[:hashSize]); err != nil {
			return n, err
		}
		n += hashSize

		entries = append(entries, &TreeEntry{
			Name:     fname,
			Hash:     hex.EncodeToString(sha[:hashSize]),
			Filemode: FileMode(mode),
		})
	}

	t.Entries = entries

	return n, nil
}

// Type is the type of entry (either blob: BlobObjectType, or a sub-tree:
// TreeObjectType).
func (e *TreeEntry) Type() string {
	switch e.Filemode & sIFMT {
	case sIFREG:
		return "blob"
	case sIFDIR:
		return "tree"
	case sIFLNK:
		return "blob"
	case sIFGITLINK:
		return "commit"
	default:
		return "unknown"
	}
}

// IsLink returns true if the given TreeEntry is a blob which represents a
// symbolic link (i.e., with a filemode of 0120000.
func (e *TreeEntry) IsLink() bool {
	return e.Filemode&sIFMT == sIFLNK
}

// Pretty tree
func (t *Tree) Pretty(w io.Writer) error {
	for _, e := range t.Entries {
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, e.Type(), e.Hash, e.Name); err != nil {
			return err
		}
	}
	return nil
}
