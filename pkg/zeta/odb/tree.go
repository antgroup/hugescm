// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type TreeEntry struct {
	Path string
	*object.TreeEntry
}

func (e *TreeEntry) Equal(other *TreeEntry) bool {
	if (e == nil) != (other == nil) {
		return false
	}
	return e.Path == other.Path && e.TreeEntry.Equal(other.TreeEntry)
}

type lsTreeEntries struct {
	entries []*TreeEntry
}

func (d *ODB) lsTreeRecurse(ctx context.Context, oid plumbing.Hash, parent string, g *lsTreeEntries) error {
	tree, err := d.Tree(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range tree.Entries {
		absName := filepath.Join(parent, e.Name)
		if e.Type() != object.TreeObject {
			g.entries = append(g.entries, &TreeEntry{Path: absName, TreeEntry: e})
			continue
		}
		if err := d.lsTreeRecurse(ctx, e.Hash, absName, g); err != nil {
			return err
		}
	}
	return nil
}

// LsTreeRecurse: list all tree entries: merge required
func (d *ODB) LsTreeRecurse(ctx context.Context, root *object.Tree) ([]*TreeEntry, error) {
	g := &lsTreeEntries{
		entries: make([]*TreeEntry, 0, 100),
	}
	for _, e := range root.Entries {
		if e.Type() != object.TreeObject {
			g.entries = append(g.entries, &TreeEntry{Path: e.Name, TreeEntry: e})
			continue
		}
		if err := d.lsTreeRecurse(ctx, e.Hash, e.Name, g); err != nil {
			return nil, err
		}
	}
	return g.entries, nil
}

// treeMaker converts a given index.Index file into multiple zeta objects
// reading the blobs from the given filesystem and creating the trees from the
// index structure. The created objects are pushed to a given Storer.
type treeMaker struct {
	*ODB
	trees map[string]*object.Tree
}

func (h *treeMaker) recurseMakeTree(e *TreeEntry, parent, fullpath string) {
	if _, ok := h.trees[fullpath]; ok {
		return
	}

	te := object.TreeEntry{Name: path.Base(fullpath)}

	if fullpath == e.Path {
		te.Mode = e.Mode
		te.Hash = e.Hash
		te.Size = int64(e.Size)
	} else {
		te.Mode = filemode.Dir
		h.trees[fullpath] = &object.Tree{}
	}

	h.trees[parent].Entries = append(h.trees[parent].Entries, &te)
}

func (h *treeMaker) makeRecursiveTrees(e *TreeEntry) error {
	parts := strings.Split(e.Path, "/")

	var fullpath string
	for _, part := range parts {
		parent := fullpath
		fullpath = path.Join(fullpath, part)

		h.recurseMakeTree(e, parent, fullpath)
	}

	return nil
}

func (h *treeMaker) copyTreeToStorageRecursive(parent string, t *object.Tree) (plumbing.Hash, error) {
	for i, e := range t.Entries {
		if e.Mode != filemode.Dir && !e.Hash.IsZero() {
			continue
		}

		name := path.Join(parent, e.Name)

		subTree, ok := h.trees[name]
		if !ok {
			return plumbing.ZeroHash, fmt.Errorf("unreachable tree object %s: %s", e.Hash, name)
		}

		var err error
		if e.Hash, err = h.copyTreeToStorageRecursive(name, subTree); err != nil {
			return plumbing.ZeroHash, err
		}

		t.Entries[i] = e
	}
	sort.Sort(object.SubtreeOrder(t.Entries))
	if oid := object.Hash(t); h.Exists(oid, true) {
		return oid, nil
	}
	return h.WriteEncoded(t)
}

func (h *treeMaker) makeTrees(entries []*TreeEntry) (plumbing.Hash, error) {
	const rootNode = ""
	h.trees = map[string]*object.Tree{rootNode: {}}

	for _, e := range entries {
		if err := h.makeRecursiveTrees(e); err != nil {
			return plumbing.ZeroHash, err
		}
	}

	return h.copyTreeToStorageRecursive(rootNode, h.trees[rootNode])
}

func (d *ODB) EmptyTree() *object.Tree {
	return object.NewEmptyTree(d)
}
