package git

import (
	"bufio"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
)

// We define these here instead of using the system ones because not all
// operating systems use the traditional values.  For example, zOS uses
// different values.
const (
	sIFMT      = int32(0170000)
	sIFREG     = int32(0100000)
	sIFDIR     = int32(0040000)
	sIFLNK     = int32(0120000)
	sIFGITLINK = int32(0160000)
)

// Tree encapsulates a Git tree object.
type Tree struct {
	// Hash of the tree object.
	Hash string
	// Entries is the list of entries held by this tree.
	Entries []*TreeEntry
}

// TreeEntry encapsulates information about a single tree entry in a tree
// listing.
type TreeEntry struct {
	// Name is the entry name relative to the tree in which this entry is
	// contained.
	Name string
	// Hash is the object ID (Hex) for this tree entry.
	Hash string
	// Filemode is the filemode of this tree entry on disk.
	Filemode int32
}

// Decode implements Object.Decode and decodes the uncompressed tree being
// read. It returns the number of uncompressed bytes being consumed off of the
// stream, which should be strictly equal to the size given.
//
// If any error was encountered along the way, that will be returned, along with
// the number of bytes read up to that point.
func (t *Tree) Decode(hash string, from io.Reader) (n int, err error) {
	t.Hash = hash
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
			Filemode: int32(mode),
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
