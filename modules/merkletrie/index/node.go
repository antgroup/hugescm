package index

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

// The Node represents a index.Entry or a directory inferred from the path
// of all entries. It implements the interface noder.Noder of merkletrie
// package.
//
// This implementation implements a "standard" hash method being able to be
// compared with any other noder.Noder implementation inside of go-git
type Node struct {
	path      string
	entry     *index.Entry
	children  []noder.Noder
	isDir     bool
	skip      bool
	fragments plumbing.Hash
}

type FragmentsGetter func(ctx context.Context, e *index.Entry) *index.Entry

// NewRootNode returns the root node of a computed tree from a index.Index,
func NewRootNode(ctx context.Context, idx *index.Index, fn FragmentsGetter) noder.Noder {
	const rootNode = ""

	m := map[string]*Node{rootNode: {isDir: true}}

	for _, e := range idx.Entries {
		parts := strings.Split(e.Name, "/")

		var fullpath string
		for _, part := range parts {
			parent := fullpath
			fullpath = path.Join(fullpath, part)

			if _, ok := m[fullpath]; ok {
				continue
			}
			n := &Node{path: fullpath}
			if fullpath == e.Name {
				if e.Mode&filemode.Fragments != 0 {
					n.fragments = e.Hash
					n.entry = fn(ctx, e)
				} else {
					n.entry = e
				}
				n.skip = e.SkipWorktree
			} else {
				n.isDir = true
			}

			m[n.path] = n
			m[parent].children = append(m[parent].children, n)
		}
	}

	return m[rootNode]
}

func (n *Node) String() string {
	return n.path
}

func (n *Node) Skip() bool {
	return n.skip
}

// Hash the hash of a filesystem is a 36-byte slice, is the result of
// concatenating the computed plumbing.Hash of the file as a Blob and its
// plumbing.FileMode; that way the difftree algorithm will detect changes in the
// contents of files and also in their mode.
//
// If the node is computed and not based on a index.Entry the hash is equals
// to a 36-bytes slices of zero values.
func (n *Node) Hash() []byte {
	if n.entry == nil {
		return make([]byte, plumbing.HASH_DIGEST_SIZE+4)
	}

	return append(n.entry.Hash[:], n.entry.Mode.Bytes()...)
}

// HashRaw: Get the original Hash of Entry. If it is fragments, get the hash of fragments, otherwise get the hash of blob
func (n *Node) HashRaw() plumbing.Hash {
	if n.entry == nil {
		return plumbing.ZeroHash
	}
	if !n.fragments.IsZero() {
		return n.fragments
	}
	return n.entry.Hash
}

func (n *Node) Mode() filemode.FileMode {
	if n.entry == nil {
		return filemode.Empty
	}
	return n.entry.Mode // origin mode. not fragments mode
}

func (n *Node) TrueMode() filemode.FileMode {
	if n.entry == nil {
		return filemode.Empty
	}
	if !n.fragments.IsZero() {
		return n.entry.Mode | filemode.Fragments
	}
	return n.entry.Mode
}

func (n *Node) ModifiedAt() time.Time {
	return n.entry.ModifiedAt
}

func (n *Node) IsFragments() bool {
	return !n.fragments.IsZero()
}

func (n *Node) Size() int64 {
	if n.entry == nil {
		return 0
	}
	return int64(n.entry.Size)
}

func (n *Node) Name() string {
	return path.Base(n.path)
}

func (n *Node) IsDir() bool {
	return n.isDir
}

func (n *Node) Children(ctx context.Context) ([]noder.Noder, error) {
	return n.children, nil
}

func (n *Node) NumChildren(ctx context.Context) (int, error) {
	return len(n.children), nil
}
