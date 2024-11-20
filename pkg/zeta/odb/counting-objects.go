// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type Entry struct {
	Hash plumbing.Hash
	Size int64
}

func newEntry(h plumbing.Hash, size int64) *Entry {
	return &Entry{Hash: h, Size: size}
}

type Entries []*Entry

const (
	defaultMaxEntries = 320000 // 32w objects
)

type Fetcher func(ctx context.Context, entries Entries) error

type entriesGroup struct {
	entries Entries
	seen    map[plumbing.Hash]bool
}

func (g *entriesGroup) clean() {
	g.entries = g.entries[:0]
}

func (d *ODB) countingFragments(ctx context.Context, oid plumbing.Hash, g *entriesGroup, maxEntries int, fetcher Fetcher) error {
	f, err := d.Fragments(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range f.Entries {
		if g.seen[e.Hash] {
			continue
		}
		if d.Exists(e.Hash, false) {
			g.seen[e.Hash] = true
			continue
		}
		if len(g.entries) >= maxEntries {
			if err := fetcher(ctx, g.entries); err != nil {
				return err
			}
			g.clean()
		}
		g.entries = append(g.entries, newEntry(e.Hash, int64(e.Size)))
		g.seen[e.Hash] = true
	}
	return nil
}

func (o *ODB) countingTreeObjects(ctx context.Context, oid plumbing.Hash, g *entriesGroup, maxEntries int, fetcher Fetcher) error {
	t, err := o.Tree(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		typ := e.Type()
		if typ == object.TreeObject {
			if err := o.countingTreeObjects(ctx, e.Hash, g, maxEntries, fetcher); err != nil {
				return err
			}
			continue
		}
		if typ == object.FragmentsObject {
			if err := o.countingFragments(ctx, e.Hash, g, maxEntries, fetcher); err != nil {
				return err
			}
			continue
		}
		if len(e.Payload) != 0 {
			continue
		}
		if g.seen[e.Hash] {
			continue
		}
		if len(g.entries) >= defaultMaxEntries {
			if err := fetcher(ctx, g.entries); err != nil {
				return err
			}
			g.clean()
		}
		if o.Exists(e.Hash, false) {
			g.seen[e.Hash] = true
			continue
		}
		g.entries = append(g.entries, newEntry(e.Hash, e.Size))
		g.seen[e.Hash] = true
	}
	return nil
}

func (o *ODB) sparseCountingTreeObjects(ctx context.Context, oid plumbing.Hash, m noder.Matcher, g *entriesGroup, maxEntries int, fetcher Fetcher) error {
	if m == nil || m.Len() == 0 {
		return o.countingTreeObjects(ctx, oid, g, maxEntries, fetcher)
	}
	t, err := o.Tree(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		typ := e.Type()
		if typ == object.FragmentsObject {
			if err := o.countingFragments(ctx, e.Hash, g, maxEntries, fetcher); err != nil {
				return err
			}
			continue
		}
		if typ == object.TreeObject {
			if sub, ok := m.Match(e.Name); ok {
				if err := o.sparseCountingTreeObjects(ctx, e.Hash, sub, g, maxEntries, fetcher); err != nil {
					return err
				}
			}
			continue
		}
		if len(e.Payload) != 0 {
			continue
		}
		if g.seen[e.Hash] {
			continue
		}
		if len(g.entries) >= defaultMaxEntries {
			if err := fetcher(ctx, g.entries); err != nil {
				return err
			}
			g.clean()
		}
		if o.Exists(e.Hash, false) {
			g.seen[e.Hash] = true
			continue
		}
		g.entries = append(g.entries, newEntry(e.Hash, e.Size))
		g.seen[e.Hash] = true
	}
	return nil
}

func (o *ODB) sparseCountingObjects(ctx context.Context, target plumbing.Hash, sparseDirs []string, maxEntries int, fetcher Fetcher) error {
	g := &entriesGroup{
		entries: make(Entries, 0, 1000),
		seen:    make(map[plumbing.Hash]bool),
	}
	cc, err := o.ParseRevExhaustive(ctx, target)
	if err != nil {
		return err
	}
	root, err := o.Tree(ctx, cc.Tree)
	if err != nil {
		return err
	}
	m := noder.NewSparseTreeMatcher(sparseDirs)
	if err := o.sparseCountingTreeObjects(ctx, root.Hash, m, g, maxEntries, fetcher); err != nil {
		return err
	}
	if len(g.entries) != 0 {
		return fetcher(ctx, g.entries)
	}
	return nil
}

// CountingSliceObjects: counting all objects for current commit
func (o *ODB) CountingSliceObjects(ctx context.Context, target plumbing.Hash, sparseDirs []string, maxEntries int, fetcher Fetcher) error {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	if len(sparseDirs) != 0 {
		return o.sparseCountingObjects(ctx, target, sparseDirs, maxEntries, fetcher)
	}
	c, err := o.ParseRevExhaustive(ctx, target)
	if err != nil {
		return err
	}

	g := &entriesGroup{
		entries: make(Entries, 0, 1000),
		seen:    make(map[plumbing.Hash]bool),
	}
	if err := o.countingTreeObjects(ctx, c.Tree, g, maxEntries, fetcher); err != nil {
		return err
	}
	if len(g.entries) != 0 {
		return fetcher(ctx, g.entries)
	}
	return nil
}

// CountingObjects: counting objects for current commit and parents...
// deepenFrom is zero --> counting all objects
func (o *ODB) CountingObjects(ctx context.Context, commit, deepenFrom plumbing.Hash, maxEntries int, fetcher Fetcher) error {
	g := &entriesGroup{
		entries: make(Entries, 0, 1000),
		seen:    make(map[plumbing.Hash]bool),
	}
	c, err := o.ParseRevExhaustive(ctx, commit)
	if err != nil {
		return err
	}
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	iter := object.NewCommitIterBSF(c, nil, nil)
	defer iter.Close()
	for {
		cc, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if cc.Hash == deepenFrom {
			break
		}
		if err := o.countingTreeObjects(ctx, cc.Tree, g, maxEntries, fetcher); err != nil {
			return err
		}
	}
	if len(g.entries) != 0 {
		return fetcher(ctx, g.entries)
	}
	return nil
}
