// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
)

// A treenoder is a helper type that wraps git trees into merkletrie
// noders.
//
// As a merkletrie noder doesn't understand the concept of modes (e.g.
// file permissions), the treenoder includes the mode of the git tree in
// the hash, so changes in the modes will be detected as modifications
// to the file contents by the merkletrie difftree algorithm.  This is
// consistent with how the "git diff-tree" command works.
type TreeNoder struct {
	parent    *Tree  // the root node is its own parent
	name      string // empty string for the root node
	mode      filemode.FileMode
	hash      plumbing.Hash
	size      int64
	fragments plumbing.Hash
	children  []noder.Noder // memoized

	m                 noder.Matcher
	conflictDetection bool
}

// NewTreeRootNode returns the root node of a Tree
func NewTreeRootNode(t *Tree, m noder.Matcher, conflictDetection bool) noder.Noder {
	if t == nil {
		return &TreeNoder{}
	}

	return &TreeNoder{
		parent:            t,
		name:              "",
		mode:              filemode.Dir,
		hash:              t.Hash,
		m:                 m,
		conflictDetection: conflictDetection,
	}
}

func (t *TreeNoder) Skip() bool {
	return false
}

func (t *TreeNoder) isRoot() bool {
	return t.name == ""
}

func (t *TreeNoder) String() string {
	return "treeNoder <" + t.name + ">"
}

func (t *TreeNoder) Mode() filemode.FileMode {
	return t.mode
}

func (t *TreeNoder) TrueMode() filemode.FileMode {
	if !t.fragments.IsZero() {
		return t.mode | filemode.Fragments
	}
	return t.mode
}

func (t *TreeNoder) HashRaw() plumbing.Hash {
	if !t.fragments.IsZero() {
		return t.fragments
	}
	return t.hash
}

func (t *TreeNoder) IsFragments() bool {
	return !t.fragments.IsZero()
}

func (t *TreeNoder) Hash() []byte {
	if t.mode == filemode.Deprecated {
		return append(t.hash[:], filemode.Regular.Bytes()...)
	}
	return append(t.hash[:], t.mode.Bytes()...)
}

func (t *TreeNoder) Name() string {
	return t.name
}

func (t *TreeNoder) Size() int64 {
	return t.size
}

func (t *TreeNoder) IsDir() bool {
	return t.mode == filemode.Dir
}

// Children will return the children of a treenoder as treenoders,
// building them from the children of the wrapped git tree.
func (t *TreeNoder) Children(ctx context.Context) ([]noder.Noder, error) {
	if t.mode != filemode.Dir {
		return noder.NoChildren, nil
	}

	// children are memoized for efficiency
	if t.children != nil {
		return t.children, nil
	}

	// the parent of the returned children will be ourself as a tree if
	// we are a not the root treenoder.  The root is special as it
	// is is own parent.
	parent := t.parent
	if !t.isRoot() {
		var err error
		if parent, err = t.parent.Tree(ctx, t.name); err != nil {
			return nil, err
		}
	}
	var err error
	t.children, err = transformChildren(ctx, parent, t.m, t.conflictDetection)
	return t.children, err
}

var (
	caseInsensitive = func() bool {
		return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	}()
)

func canonicalName(name string) string {
	if caseInsensitive {
		return strings.ToLower(name)
	}
	return name
}

// Returns the children of a tree as treenoders.
// Efficiency is key here.
func transformChildren(ctx context.Context, t *Tree, m noder.Matcher, conflictDetection bool) ([]noder.Noder, error) {
	var err error
	var e *TreeEntry

	// there will be more tree entries than children in the tree,
	// due to submodules and empty directories, but I think it is still
	// worth it to pre-allocate the whole array now, even if sometimes
	// is bigger than needed.
	noDuplicateEntries := make(map[string]bool)
	ret := make([]noder.Noder, 0, len(t.Entries))
	walker := NewTreeWalker(t, false, nil) // don't recurse
	// don't defer walker.Close() for efficiency reasons.
	for {
		_, e, err = walker.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			walker.Close()
			return nil, err
		}

		var n *TreeNoder
		switch typ := e.Type(); typ {
		case TreeObject:
			var ok bool
			var sub noder.Matcher
			if m != nil && m.Len() > 0 {
				if sub, ok = m.Match(e.Name); !ok {
					continue
				}
			}
			n = &TreeNoder{
				parent:            t,
				name:              e.Name,
				mode:              e.Mode,
				hash:              e.Hash,
				size:              e.Size,
				m:                 sub,
				conflictDetection: conflictDetection,
			}
		case FragmentsObject:
			n = &TreeNoder{
				parent:            t,
				name:              e.Name,
				mode:              e.Mode,
				hash:              e.Hash,
				size:              e.Size,
				conflictDetection: conflictDetection,
			}
			if ff, err := t.b.Fragments(ctx, e.Hash); err == nil {
				n.mode = e.OriginMode()
				n.hash = ff.Origin
				n.fragments = e.Hash
				n.size = int64(ff.Size)
			}
		case BlobObject:
			n = &TreeNoder{
				parent:            t,
				name:              e.Name,
				mode:              e.Mode,
				hash:              e.Hash,
				size:              e.Size,
				conflictDetection: conflictDetection,
			}
		default:
			return nil, fmt.Errorf("unexpected object type '%d'", typ)
		}
		if conflictDetection {
			cname := canonicalName(e.Name)
			if noDuplicateEntries[cname] {
				continue
			}
			noDuplicateEntries[cname] = true
		}
		ret = append(ret, n)

	}
	walker.Close()

	return ret, nil
}

// len(t.tree.Entries) != the number of elements walked by treewalker
// for some reason because of empty directories, submodules, etc, so we
// have to walk here.
func (t *TreeNoder) NumChildren(ctx context.Context) (int, error) {
	children, err := t.Children(ctx)
	if err != nil {
		return 0, err
	}

	return len(children), nil
}
