package filesystem

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/streamio"
)

var ignore = map[string]bool{
	".zeta": true,
}

// The Node represents a file or a directory in a billy.Filesystem. It
// implements the interface noder.Noder of merkletrie package.
//
// This implementation implements a "standard" hash method being able to be
// compared with any other noder.Noder implementation inside of go-git.
type Node struct {
	root string

	path       string
	hash       []byte
	children   []noder.Noder
	isDir      bool
	mode       os.FileMode
	size       int64
	modifiedAt time.Time

	m noder.Matcher
}

// NewRootNode returns the root node based on a given billy.Filesystem.
//
// In order to provide the submodule hash status, a map[string]plumbing.Hash
// should be provided where the key is the path of the submodule and the commit
// of the submodule HEAD
func NewRootNode(root string, m noder.Matcher) noder.Noder {
	return &Node{root: root, isDir: true, m: m}
}

func (n Node) fsPath(p string) string {
	return filepath.Join(n.root, p)
}

// Hash the hash of a filesystem is the result of concatenating the computed
// plumbing.Hash of the file as a Blob and its plumbing.FileMode; that way the
// difftree algorithm will detect changes in the contents of files and also in
// their mode.
//
// The hash of a directory is always a 36-bytes slice of zero values
func (n *Node) Hash() []byte {
	if len(n.hash) == 0 {
		n.calculateHash()
	}
	return n.hash
}

func (n *Node) Mode() filemode.FileMode {
	m, _ := filemode.NewFromOS(n.mode)
	return m
}

func (n *Node) HijackMode(mode filemode.FileMode) {
	n.mode, _ = mode.ToOSFileMode()
}

func (n *Node) ModifiedAt() time.Time {
	return n.modifiedAt
}

func (n *Node) Size() int64 {
	return n.size
}

func (n *Node) HashRaw() plumbing.Hash {
	hash := n.Hash()
	var oid plumbing.Hash
	copy(oid[:], hash)
	return oid
}

func (n *Node) Name() string {
	return path.Base(n.path)
}

func (n *Node) IsDir() bool {
	return n.isDir
}

func (n *Node) Skip() bool {
	return false
}

func (n *Node) Children(ctx context.Context) ([]noder.Noder, error) {
	if err := n.calculateChildren(); err != nil {
		return nil, err
	}

	return n.children, nil
}

func (n *Node) NumChildren(ctx context.Context) (int, error) {
	if err := n.calculateChildren(); err != nil {
		return -1, err
	}

	return len(n.children), nil
}

func (n *Node) calculateChildren() error {
	if !n.IsDir() {
		return nil
	}

	if len(n.children) != 0 {
		return nil
	}

	dirs, err := os.ReadDir(filepath.Join(n.root, n.path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, d := range dirs {
		if _, ok := ignore[d.Name()]; ok {
			continue
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}

		if fi.Mode()&os.ModeSocket != 0 {
			continue
		}

		c, err := n.newChildNode(fi)
		if err != nil {
			return err
		}
		if c != nil {
			n.children = append(n.children, c)
		}
	}

	return nil
}

func (n *Node) newChildNode(fi os.FileInfo) (*Node, error) {
	var m noder.Matcher
	var ok bool
	if fi.IsDir() && n.m != nil && n.m.Len() != 0 {
		if m, ok = n.m.Match(fi.Name()); !ok {
			return nil, nil
		}
	}
	node := &Node{
		root: n.root,

		path:       path.Join(n.path, fi.Name()),
		isDir:      fi.IsDir(),
		size:       fi.Size(),
		mode:       fi.Mode(),
		modifiedAt: fi.ModTime(),
		m:          m,
	}

	return node, nil
}

func (n *Node) calculateHash() {
	if n.isDir {
		n.hash = make([]byte, plumbing.HASH_DIGEST_SIZE+4)
		return
	}
	mode, err := filemode.NewFromOS(n.mode)
	if err != nil {
		n.hash = make([]byte, plumbing.HASH_DIGEST_SIZE+4)
		return
	}
	var hash plumbing.Hash
	if n.mode&os.ModeSymlink != 0 {
		hash = n.doCalculateHashForSymlink()
	} else {
		hash = n.doCalculateHashForRegular()
	}
	n.hash = append(hash[:], mode.Bytes()...)
}

func (n *Node) doCalculateHashForRegular() plumbing.Hash {
	f, err := os.Open(n.fsPath(n.path))
	if err != nil {
		return plumbing.ZeroHash
	}

	defer f.Close() // nolint

	h := plumbing.NewHasher()
	if _, err := streamio.Copy(h, f); err != nil {
		return plumbing.ZeroHash
	}
	return h.Sum()
}

func (n *Node) doCalculateHashForSymlink() plumbing.Hash {
	target, err := os.Readlink(n.fsPath(n.path))
	if err != nil {
		return plumbing.ZeroHash
	}

	h := plumbing.NewHasher()
	if _, err := h.Write([]byte(target)); err != nil {
		return plumbing.ZeroHash
	}

	return h.Sum()
}

func (n *Node) String() string {
	return n.path
}

func (n *Node) Type() string {
	return "fs"
}
