// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

type LsTreeOptions struct {
	OnlyTrees bool
	Recurse   bool
	Tree      bool
	NewLine   byte
	Long      bool
	NameOnly  bool
	Abbrev    int
	Revision  string
	Paths     []string
	JSON      bool
}

func sizePadding(e *object.TreeEntry, padding int) string {
	switch e.Type() {
	case object.TreeObject:
		return strings.Repeat(" ", padding-1) + "-"
	case object.FragmentsObject:
		return strings.Repeat(" ", padding-1-5) + "L"
	default:
	}
	ss := strconv.FormatInt(e.Size, 10)
	if len(ss) >= padding {
		return ss
	}
	return strings.Repeat(" ", padding-len(ss)) + ss
}

func spacePadding(e *object.TreeEntry, padding int) string {
	if e.Type() == object.FragmentsObject {
		return strings.Repeat(" ", padding-5)
	}
	return strings.Repeat(" ", padding)
}

func (opts *LsTreeOptions) ShortName(oid plumbing.Hash) string {
	s := oid.String()
	if opts.Abbrev > 0 && opts.Abbrev < 64 {
		return s[0:opts.Abbrev]
	}
	return s
}

func (opts *LsTreeOptions) ShowTree(w io.Writer, t *object.Tree) {
	if opts.NameOnly {
		if opts.JSON {
			names := make([]string, 0, len(t.Entries))
			for _, e := range t.Entries {
				names = append(names, e.Name)
			}
			_ = json.NewEncoder(w).Encode(names)
			return
		}
		for _, e := range t.Entries {
			fmt.Fprintf(w, "%s%c", e.Name, opts.NewLine)
		}
		return
	}
	if opts.JSON {
		_ = json.NewEncoder(w).Encode(t.Entries)
		return
	}
	if opts.Long {
		padding := t.SizePadding()
		for _, e := range t.Entries {
			fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode.Origin(), e.Type(), opts.ShortName(e.Hash), sizePadding(e, padding), e.Name)
		}
		return
	}
	padding := t.SpacePadding()
	for _, e := range t.Entries {
		fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode.Origin(), e.Type(), opts.ShortName(e.Hash), spacePadding(e, padding), e.Name)
	}

}

func (opts *LsTreeOptions) Rev() string {
	if len(opts.Revision) == 0 {
		return string(plumbing.HEAD)
	}
	return opts.Revision
}

func (r *Repository) resolveTree0(ctx context.Context, branchOrTag string) (t *object.Tree, err error) {
	var oid plumbing.Hash
	if oid, err = r.Revision(ctx, branchOrTag); err != nil {
		return nil, err
	}
	r.DbgPrint("resolve object '%s'", oid)
	o, err := r.odb.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	switch a := o.(type) {
	case *object.Tree:
		return a, nil
	case *object.Commit:
		return r.odb.Tree(ctx, a.Tree)
	}
	return nil, errors.New("not a tree object")
}

func (r *Repository) resolveTree(ctx context.Context, revisionPair string) (*object.Tree, error) {
	k, v, ok := strings.Cut(revisionPair, ":")
	if !ok {
		return r.resolveTree0(ctx, k)
	}
	if len(k) == 0 {
		k = string(plumbing.HEAD)
	}
	oid, err := r.Revision(ctx, k)
	if err != nil {
		return nil, err
	}
	return r.readTree(ctx, oid, v)
}

func (r *Repository) LsTree(ctx context.Context, opts *LsTreeOptions) error {
	rev := opts.Rev()
	t, err := r.resolveTree(ctx, rev)
	if err != nil {
		return err
	}
	m := NewMatcher(opts.Paths)
	if opts.Recurse {
		return r.LsTreeRecurse(ctx, opts, t, m)
	}
	if len(opts.Paths) == 0 {
		opts.ShowTree(os.Stdout, t)
		return nil
	}
	g := make(map[string]*object.TreeEntry)
	for _, p := range opts.Paths {
		if strings.HasSuffix(p, "/") {
			treeName := p[:len(p)-1]
			if tree, err := t.Tree(ctx, treeName); err == nil {
				for _, e := range tree.Entries {
					g[path.Join(treeName, e.Name)] = e
				}
			}
			continue
		}
		if e, err := t.FindEntry(ctx, p); err == nil {
			g[p] = e
		}
	}
	entries := make([]*object.TreeEntry, 0, len(g))
	for k, e := range g {
		entries = append(entries, &object.TreeEntry{Name: k, Size: e.Size, Hash: e.Hash, Mode: e.Mode})
	}
	sort.Sort(object.SubtreeOrder(entries))
	opts.ShowTree(os.Stdout, &object.Tree{Entries: entries})
	return nil
}

type lsTreeEntries struct {
	entries      []*odb.TreeEntry
	sizeMax      int64
	hasFragments bool
}

type JsonTreeEntry struct {
	Name string            `json:"name"`
	Size int64             `json:"size"`
	Mode filemode.FileMode `json:"mode"`
	Hash plumbing.Hash     `json:"hash"`
}

func (g *lsTreeEntries) JsonTreeEntries() []*JsonTreeEntry {
	entries := make([]*JsonTreeEntry, 0, len(g.entries))
	for _, e := range g.entries {
		entries = append(entries, &JsonTreeEntry{
			Name: e.Path,
			Size: e.Size,
			Mode: e.Mode,
			Hash: e.Hash,
		})
	}
	return entries
}

func (g *lsTreeEntries) SizePadding() int {
	sizeMax := len(strconv.FormatInt(g.sizeMax, 10))
	if g.hasFragments {
		// blob/fragments 4/9 d5
		return max(5, sizeMax)
	}
	return sizeMax
}

func (r *Repository) lsTreeRecurse1(ctx context.Context, opts *LsTreeOptions, oid plumbing.Hash, parent string, g *lsTreeEntries) error {
	t, err := r.odb.Tree(ctx, oid)
	if plumbing.IsNoSuchObject(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		switch e.Type() {
		case object.TreeObject:
			if opts.Tree {
				g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
			}
			if err := r.lsTreeRecurse1(ctx, opts, e.Hash, name, g); err != nil {
				return err
			}
		case object.FragmentsObject:
			if !opts.OnlyTrees {
				g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
				g.sizeMax = max(g.sizeMax, e.Size)
				g.hasFragments = true
			}
		case object.BlobObject:
			if !opts.OnlyTrees {
				g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
				g.sizeMax = max(g.sizeMax, e.Size)
			}
		}
	}
	return nil
}

func (r *Repository) lsTreeRecurse0(ctx context.Context, opts *LsTreeOptions, t *object.Tree, m *Matcher, parent string, g *lsTreeEntries) error {
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		if m.Match(name) {
			switch e.Type() {
			case object.TreeObject:
				if opts.Tree {
					g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
				}
				if err := r.lsTreeRecurse1(ctx, opts, e.Hash, name, g); err != nil {
					return err
				}
			case object.FragmentsObject:
				if !opts.OnlyTrees {
					g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
					g.sizeMax = max(g.sizeMax, e.Size)
					g.hasFragments = true
				}
			case object.BlobObject:
				if !opts.OnlyTrees {
					g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
					g.sizeMax = max(g.sizeMax, e.Size)
				}
			}
			continue
		}
		var tree *object.Tree
		tree, err := r.odb.Tree(ctx, e.Hash)
		if plumbing.IsNoSuchObject(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := r.lsTreeRecurse0(ctx, opts, tree, m, name, g); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) LsTreeRecurse(ctx context.Context, opts *LsTreeOptions, t *object.Tree, m *Matcher) error {
	g := &lsTreeEntries{
		entries: make([]*odb.TreeEntry, 0, 100),
	}
	if err := r.lsTreeRecurse0(ctx, opts, t, m, "", g); err != nil {
		return err
	}
	if opts.NameOnly {
		if opts.JSON {
			names := make([]string, 0, len(g.entries))
			for _, e := range g.entries {
				names = append(names, e.Name)
			}
			return json.NewEncoder(os.Stdout).Encode(names)
		}
		for _, e := range g.entries {
			fmt.Fprintf(os.Stdout, "%s%c", e.Path, opts.NewLine)
		}
		return nil
	}
	if opts.JSON {
		jsonEntries := g.JsonTreeEntries()
		return json.NewEncoder(os.Stdout).Encode(jsonEntries)
	}

	if opts.Long {
		padding := g.SizePadding()
		for _, e := range g.entries {
			fmt.Fprintf(os.Stdout, "%s %s %s %s %s\n", e.Mode.Origin(), e.Type(), opts.ShortName(e.Hash), sizePadding(e.TreeEntry, padding), e.Path)
		}
		return nil
	}
	padding := 0
	if g.hasFragments {
		padding = 5
	}
	for _, e := range g.entries {
		fmt.Fprintf(os.Stdout, "%s %s %s %s %s\n", e.Mode.Origin(), e.Type(), opts.ShortName(e.Hash), spacePadding(e.TreeEntry, padding), e.Path)
	}
	return nil
}

func (r *Repository) lsTreeRecurseFilter1(ctx context.Context, oid plumbing.Hash, parent string, g *lsTreeEntries) error {
	t, err := r.odb.Tree(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		if e.Type() != object.TreeObject {
			g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
			continue
		}
		if err := r.lsTreeRecurseFilter1(ctx, e.Hash, name, g); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) lsSparseTreeRecurseFilter1(ctx context.Context, oid plumbing.Hash, m noder.Matcher, parent string, g *lsTreeEntries) error {
	if m.Len() == 0 {
		return r.lsTreeRecurseFilter1(ctx, oid, parent, g)
	}
	t, err := r.odb.Tree(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		if e.Type() != object.TreeObject {
			g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
			continue
		}
		subMatcher, ok := m.Match(e.Name)
		if !ok {
			continue
		}
		if err := r.lsSparseTreeRecurseFilter1(ctx, e.Hash, subMatcher, name, g); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) lsTreeRecurseFilter0(ctx context.Context, t *object.Tree, m *Matcher, parent string, g *lsTreeEntries) error {
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		if m.Match(name) {
			if e.Type() != object.TreeObject {
				g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
				continue
			}
			if err := r.lsTreeRecurseFilter1(ctx, e.Hash, name, g); err != nil {
				return err
			}
			continue
		}
		if e.Type() != object.TreeObject {
			continue
		}
		var tree *object.Tree
		tree, err := r.odb.Tree(ctx, e.Hash)
		if err != nil {
			return err
		}
		if err := r.lsTreeRecurseFilter0(ctx, tree, m, name, g); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) lsTreeSparseRecurseFilter0(ctx context.Context, t *object.Tree, m *Matcher, sparseMatcher noder.Matcher, parent string, g *lsTreeEntries) error {
	if sparseMatcher.Len() == 0 {
		return r.lsTreeRecurseFilter0(ctx, t, m, parent, g)
	}
	for _, e := range t.Entries {
		name := path.Join(parent, e.Name)
		if e.Type() != object.TreeObject {
			if m.Match(name) {
				g.entries = append(g.entries, &odb.TreeEntry{Path: name, TreeEntry: e})
			}
			continue
		}
		subMatcher, ok := sparseMatcher.Match(e.Name)
		if !ok {
			continue
		}
		if m.Match(name) {
			if err := r.lsSparseTreeRecurseFilter1(ctx, e.Hash, subMatcher, parent, g); err != nil {
				return err
			}
			continue
		}
		tree, err := r.odb.Tree(ctx, e.Hash)
		if err != nil {
			return err
		}
		if err := r.lsTreeSparseRecurseFilter0(ctx, tree, m, subMatcher, name, g); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) lsTreeRecurseFilter(ctx context.Context, t *object.Tree, m *Matcher) ([]*odb.TreeEntry, error) {
	g := &lsTreeEntries{
		entries: make([]*odb.TreeEntry, 0, 100),
	}
	var err error
	if len(r.Core.SparseDirs) != 0 {
		if err = r.lsTreeSparseRecurseFilter0(ctx, t, m, noder.NewSparseTreeMatcher(r.Core.SparseDirs), "", g); err != nil {
			return nil, err
		}
		return g.entries, nil
	}
	if err = r.lsTreeRecurseFilter0(ctx, t, m, "", g); err != nil {
		return nil, err
	}
	return g.entries, nil
}
