// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

const (
	largeRawSize = 20 << 20 // 20M
)

type walker struct {
	*ODB
	shallow plumbing.Hash
	theirs  plumbing.Hash
	seen    map[plumbing.Hash]bool // have objects
	deltaM  map[plumbing.Hash]bool
	deltaB  map[plumbing.Hash]bool
	delta   *PushObjects
}

func newWalker(odb *ODB, shallow, theirs plumbing.Hash) *walker {
	return &walker{
		ODB:     odb,
		shallow: shallow,
		theirs:  theirs,
		seen:    make(map[plumbing.Hash]bool),
		deltaM:  make(map[plumbing.Hash]bool),
		deltaB:  make(map[plumbing.Hash]bool),
		delta:   &PushObjects{},
	}
}

// countingTree
func (w *walker) countingTree(ctx context.Context, oid plumbing.Hash) error {
	t, err := w.Tree(ctx, oid)
	if err != nil {
		if plumbing.IsNoSuchObject(err) {
			return nil
		}
		return err
	}
	for _, e := range t.Entries {
		w.seen[e.Hash] = true
		switch e.Type() {
		case object.TreeObject:
			if err := w.countingTree(ctx, e.Hash); err != nil {
				return err
			}
		case object.FragmentsObject:
			f, err := w.Fragments(ctx, e.Hash)
			if err != nil {
				if plumbing.IsNoSuchObject(err) {
					return nil
				}
				return err
			}
			for _, b := range f.Entries {
				w.seen[b.Hash] = true
			}
		}
	}
	return nil
}

func (w *walker) init(ctx context.Context) error {
	if !w.theirs.IsZero() {
		cc, err := w.Commit(ctx, w.theirs)
		switch {
		case err == nil:
			w.seen[cc.Tree] = true
			if err := w.countingTree(ctx, cc.Tree); err != nil {
				return err
			}
		case !plumbing.IsNoSuchObject(err):
			return err
		default:
			// nothing
		}
	}
	if w.shallow.IsZero() {
		return nil
	}
	cc, err := w.Commit(ctx, w.shallow)
	if err != nil {
		return err
	}
	w.seen[cc.Tree] = true
	return w.countingTree(ctx, cc.Tree)
}

func (w *walker) deltaFragments(ctx context.Context, oid plumbing.Hash) error {
	ff, err := w.Fragments(ctx, oid)
	if err != nil {
		return err
	}
	for _, e := range ff.Entries {
		if w.seen[e.Hash] {
			continue
		}
		if w.deltaB[e.Hash] {
			continue
		}
		w.deltaB[e.Hash] = true
		size, err := w.Size(e.Hash, false)
		if err != nil {
			return err
		}
		w.delta.LargeObjects = append(w.delta.LargeObjects, &HaveObject{Hash: e.Hash, Size: size})
	}
	w.deltaM[oid] = true
	w.delta.Metadata = append(w.delta.Metadata, oid)
	return nil
}

func (w *walker) deltaTree(ctx context.Context, oid plumbing.Hash) error {
	// have oid
	if w.seen[oid] {
		return nil
	}
	// tree counted
	if w.deltaM[oid] {
		return nil
	}
	t, err := w.Tree(ctx, oid)
	if err != nil {
		return err
	}
	w.deltaM[oid] = true
	w.delta.Metadata = append(w.delta.Metadata, oid)
	for _, e := range t.Entries {
		typ := e.Type()
		if typ == object.TreeObject {
			if err := w.deltaTree(ctx, e.Hash); err != nil {
				return err
			}
			continue
		}
		if typ == object.FragmentsObject {
			// have fragments
			if w.seen[e.Hash] {
				continue
			}
			// fragments counted
			if w.deltaM[e.Hash] {
				continue
			}
			if err := w.deltaFragments(ctx, e.Hash); err != nil {
				return err
			}
			continue
		}
		// have blob
		if w.seen[e.Hash] {
			continue
		}
		// blob counted
		if w.deltaB[e.Hash] {
			continue
		}
		if e.Size > largeRawSize {
			size, err := w.Size(e.Hash, false)
			if err != nil {
				return err
			}
			if size > largeRawSize {
				w.delta.LargeObjects = append(w.delta.LargeObjects, &HaveObject{Hash: e.Hash, Size: size})
				continue
			}
		}
		w.deltaB[e.Hash] = true
		w.delta.Objects = append(w.delta.Objects, e.Hash)
	}
	return nil
}

func (w *walker) get() *PushObjects {
	return w.delta
}

func (w *walker) next(ctx context.Context, current plumbing.Hash) error {
	if current == w.shallow || current == w.theirs || w.seen[current] {
		return nil
	}
	cc, objects, err := w.ParseRevEx(ctx, current)
	if err != nil {
		if plumbing.IsNoSuchObject(err) {
			return nil
		}
	}
	w.delta.Metadata = append(w.delta.Metadata, current)
	w.delta.Metadata = append(w.delta.Metadata, objects...)
	w.seen[current] = true
	if err := w.deltaTree(ctx, cc.Tree); err != nil {
		return err
	}
	for _, p := range cc.Parents {
		if err := w.next(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (o *ODB) Delta(ctx context.Context, newRev, shallow, head plumbing.Hash) (*PushObjects, error) {
	w := newWalker(o, shallow, head)
	if err := w.init(ctx); err != nil {
		return nil, err
	}
	if err := w.next(ctx, newRev); err != nil {
		return nil, err
	}
	return w.get(), nil
}

func (o *ODB) ParseRevExhaustive(ctx context.Context, oid plumbing.Hash) (*object.Commit, error) {
	a, err := o.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	if cc, ok := a.(*object.Commit); ok {
		return cc, nil
	}
	current, ok := a.(*object.Tag)
	if !ok {
		return nil, backend.NewErrMismatchedObjectType(oid, "commit")
	}
	for i := 0; i < 10; i++ {
		if current.ObjectType == object.TreeObject {
			return nil, backend.NewErrMismatchedObjectType(oid, "commit")
		}
		if current.ObjectType == object.CommitObject {
			cc, err := o.Commit(ctx, current.Object)
			if err != nil {
				return nil, err
			}
			return cc, nil
		}
		if current.ObjectType != object.TagObject {
			return nil, backend.NewErrMismatchedObjectType(oid, "commit")
		}
		tag, err := o.Tag(ctx, current.Object)
		if err != nil {
			return nil, err
		}
		current = tag
	}
	return nil, plumbing.NoSuchObject(oid)
}
