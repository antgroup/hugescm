// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

const (
	rootNode = ""
)

// treeBuilder converts a given index.Index file into multiple zeta objects
// reading the blobs from the given filesystem and creating the trees from the
// index structure. The created objects are pushed to a given Storer.
type treeBuilder struct {
	w     *Worktree
	trees map[string]*object.Tree
	// readonly entries
	readOnlyEntries map[string]*object.TreeEntry
}

func (h *treeBuilder) resolveReadOnlyEntries(ctx context.Context, t *object.Tree, parent string, m noder.Matcher) error {
	noDuplicateEntries := make(map[string]bool)
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		if e.Type() == object.TreeObject {
			var subMatcher noder.Matcher
			if m != nil && m.Len() != 0 {
				var ok bool
				if subMatcher, ok = m.Match(e.Name); !ok {
					h.readOnlyEntries[name] = e
					continue
				}
			}
			tree, err := h.w.odb.Tree(ctx, e.Hash)
			if err != nil {
				return err
			}
			if err = h.resolveReadOnlyEntries(ctx, tree, name, subMatcher); err != nil {
				return err
			}
		}
		cname := canonicalName(e.Name)
		if noDuplicateEntries[cname] {
			h.readOnlyEntries[name] = e
			continue
		}
		noDuplicateEntries[cname] = true
		continue
	}
	return nil
}

func (h *treeBuilder) writeIndexEntry(e *index.Entry) error {
	parts := strings.Split(e.Name, "/")

	var fullpath string
	for _, part := range parts {
		parent := fullpath
		fullpath = path.Join(fullpath, part)

		h.recurseNewTree(e, parent, fullpath)
	}

	return nil
}

func (h *treeBuilder) recurseNewTree(e *index.Entry, parent, fullpath string) {
	if _, ok := h.trees[fullpath]; ok {
		return
	}

	te := object.TreeEntry{Name: path.Base(fullpath)}

	if fullpath == e.Name {
		te.Mode = e.Mode
		te.Hash = e.Hash
		te.Size = int64(e.Size)
	} else {
		te.Mode = filemode.Dir
		h.trees[fullpath] = &object.Tree{}
	}

	h.trees[parent].Entries = append(h.trees[parent].Entries, &te)
}

// unchecked tree entry
// sigma/appops/trafficsaas
// 1 check sigma exists
// 2 check sigma/appops exists
// 3 restore sigma/appops/trafficsaas
func (h *treeBuilder) restoreReadOnlyTreeEntry(e *object.TreeEntry, name string) error {
	parts := strings.Split(name, "/")
	var current string
	for _, part := range parts[:len(parts)-1] {
		parent := current
		current = path.Join(parent, part)
		if _, ok := h.trees[current]; ok {
			continue
		}
		h.trees[current] = &object.Tree{}
	}
	if t, ok := h.trees[current]; ok {
		t.Append(e)
	}
	return nil
}

func (h *treeBuilder) copyTreeToStorageRecursive(parent string, t *object.Tree) (plumbing.Hash, error) {
	for i, e := range t.Entries {
		if e.Mode != filemode.Dir && !e.Hash.IsZero() {
			continue
		}

		name := path.Join(parent, e.Name)

		subTree, ok := h.trees[name]
		if !ok {
			if _, ok = h.readOnlyEntries[name]; !ok {
				return plumbing.ZeroHash, fmt.Errorf("unreachable tree object %s: %s", e.Hash, name)
			}
			continue
		}

		var err error
		if e.Hash, err = h.copyTreeToStorageRecursive(name, subTree); err != nil {
			return plumbing.ZeroHash, err
		}

		t.Entries[i] = e
	}
	sort.Sort(object.SubtreeOrder(t.Entries))
	if oid := object.Hash(t); h.w.odb.Exists(oid, true) {
		return oid, nil
	}
	return h.w.odb.WriteEncoded(t)
}

func (h *treeBuilder) makeSparseTrees(ctx context.Context, idx *index.Index, readOnlyTree plumbing.Hash, allowEmptyCommits bool, sparseDirs []string) (plumbing.Hash, error) {
	if len(idx.Entries) == 0 && !allowEmptyCommits {
		return plumbing.ZeroHash, ErrNoChanges
	}
	if len(sparseDirs) == 0 || readOnlyTree.IsZero() {
		return h.makeTrees(idx)
	}
	readOnlyRoot, err := h.w.odb.Tree(ctx, readOnlyTree)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	// resolve sparse trees
	if err = h.resolveReadOnlyEntries(ctx, readOnlyRoot, "", noder.NewSparseTreeMatcher(sparseDirs)); err != nil {
		return plumbing.ZeroHash, err
	}

	for _, e := range idx.Entries {
		if err := h.writeIndexEntry(e); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	for name, e := range h.readOnlyEntries {
		if err := h.restoreReadOnlyTreeEntry(e, name); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	return h.copyTreeToStorageRecursive(rootNode, h.trees[rootNode])
}

// makeTrees builds the tree objects and push its to the storer, the hash
// of the root tree is returned.
func (h *treeBuilder) makeTrees(idx *index.Index) (plumbing.Hash, error) {
	for _, e := range idx.Entries {
		if err := h.writeIndexEntry(e); err != nil {
			return plumbing.ZeroHash, err
		}
	}

	return h.copyTreeToStorageRecursive(rootNode, h.trees[rootNode])
}

func (w *Worktree) writeIndexAsTree(ctx context.Context, readOnlyTree plumbing.Hash, allowEmptyCommits bool) (plumbing.Hash, error) {
	idx, err := w.odb.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	b := &treeBuilder{
		w:               w,
		trees:           map[string]*object.Tree{rootNode: {}},
		readOnlyEntries: make(map[string]*object.TreeEntry),
	}
	return b.makeSparseTrees(ctx, idx, readOnlyTree, allowEmptyCommits, w.Core.SparseDirs)
}
