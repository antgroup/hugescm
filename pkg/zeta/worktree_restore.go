// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing/format/index"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

type RestoreOptions struct {
	Source   string
	Staged   bool
	Worktree bool
	Paths    []string
}

func (w *Worktree) lsRestoreEntriesFromTree(ctx context.Context, opts *RestoreOptions, root *object.Tree) ([]*odb.TreeEntry, error) {
	entries, err := w.lsTreeRecurseFilter(ctx, root, NewMatcher(opts.Paths))
	if err != nil {
		die("restore ls-tree %s error: %v", root.Hash, err)
		return nil, err
	}
	w.DbgPrint("matched entries: %d", len(entries))
	ci := newMissingFetcher()
	largeSize := w.largeSize()
	for _, e := range entries {
		switch e.Type() {
		case object.BlobObject:
			ci.store(w.odb, e.Hash, e.Size, largeSize)
		case object.FragmentsObject:
			fragmentEntry, err := w.odb.Fragments(ctx, e.Hash)
			if err != nil {
				return nil, fmt.Errorf("open fragments: %w", err)
			}
			for _, ee := range fragmentEntry.Entries {
				ci.store(w.odb, ee.Hash, int64(ee.Size), largeSize)
			}
		default:
		}
	}
	if err := w.fetchMissingObjects(ctx, ci, false); err != nil {
		return nil, err
	}
	return entries, nil
}

func (w *Worktree) lsRestoreEntries(ctx context.Context, opts *RestoreOptions) ([]*odb.TreeEntry, error) {
	switch {
	case len(opts.Source) != 0:
		root, err := w.parseTreeExhaustive(ctx, opts.Source, "")
		if err != nil {
			return nil, err
		}
		return w.lsRestoreEntriesFromTree(ctx, opts, root)
	case opts.Staged:
		root, err := w.parseTreeExhaustive(ctx, "HEAD", "")
		if err != nil {
			return nil, err
		}
		return w.lsRestoreEntriesFromTree(ctx, opts, root)
	default:
	}
	idx, err := w.odb.Index()
	if err != nil {
		return nil, err
	}
	m := NewMatcher(opts.Paths)
	largeSize := w.largeSize()
	entries := make([]*odb.TreeEntry, 0, 100)
	for _, e := range idx.Entries {
		if !m.Match(e.Name) {
			continue
		}
		entries = append(entries, &odb.TreeEntry{
			Path: e.Name,
			TreeEntry: &object.TreeEntry{
				Name: filepath.Base(e.Name),
				Size: int64(e.Size),
				Mode: e.Mode,
				Hash: e.Hash,
			}})
	}
	ci := newMissingFetcher()
	for _, e := range entries {
		switch e.Type() {
		case object.BlobObject:
			ci.store(w.odb, e.Hash, e.Size, largeSize)
		case object.FragmentsObject:
			fragmentEntry, err := w.odb.Fragments(ctx, e.Hash)
			if err != nil {
				return nil, fmt.Errorf("open fragments: %w", err)
			}
			for _, ee := range fragmentEntry.Entries {
				ci.store(w.odb, ee.Hash, int64(ee.Size), largeSize)
			}
		default:
		}
	}
	if err := w.fetchMissingObjects(ctx, ci, false); err != nil {
		return nil, err
	}
	return entries, nil
}

func (w *Worktree) restoreIndexMatch(ctx context.Context, entries []*odb.TreeEntry, m *Matcher) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	idx, err := w.odb.Index()

	if err != nil {
		return err
	}
	b := newUnlessIndexBuilder(idx, m)
	modifiedAt := time.Now()
	for _, e := range entries {
		b.Add(&index.Entry{
			Name:       e.Path,
			Hash:       e.Hash,
			Mode:       e.Mode,
			Size:       uint64(e.Size),
			ModifiedAt: modifiedAt,
		})
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) Restore(ctx context.Context, opts *RestoreOptions) error {
	entries, err := w.lsRestoreEntries(ctx, opts)
	if err != nil {
		die("zeta restore error: %v", err)
		return err
	}
	m := NewMatcher(opts.Paths)
	if opts.Staged {
		if err := w.restoreIndexMatch(ctx, entries, m); err != nil {
			return err
		}
	}
	if !opts.Worktree {
		return nil
	}
	if len(entries) == 0 {
		// NO entries
		return nil
	}
	bar := progress.NewIndicators("Restore files", "Restore files completed", uint64(len(entries)), w.quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.resetWorktreeEntries(ctx, entries, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	return nil
}
