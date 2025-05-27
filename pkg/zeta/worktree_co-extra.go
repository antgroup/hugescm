// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/antgroup/hugescm/modules/merkletrie"
	mindex "github.com/antgroup/hugescm/modules/merkletrie/index"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

// TODO: rename detect ???

var (
	ErrNotTreeNoder = errors.New("not tree noder")
	ErrNotIndexNode = errors.New("not index node")
)

func (w *Worktree) resolveTreeEntry(p noder.Path) (*object.TreeEntry, error) {
	n, ok := p.Last().(*object.TreeNoder)
	if !ok {
		return nil, ErrNotTreeNoder
	}
	return &object.TreeEntry{
		Name: n.Name(),
		Size: n.Size(),
		Mode: n.TrueMode(),
		Hash: n.HashRaw(),
	}, nil
}

func (w *Worktree) resolveIndexEntry(p noder.Path) (*object.TreeEntry, error) {
	n, ok := p.Last().(*mindex.Node)
	if !ok {
		return nil, ErrNotIndexNode
	}
	return &object.TreeEntry{
		Name: n.Name(),
		Size: n.Size(),
		Mode: n.TrueMode(), //
		Hash: n.HashRaw(),
	}, nil
}

func (w *Worktree) checkoutOne(ctx context.Context, t transport.Transport, name string, e *object.TreeEntry) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)
	bar := &nonProgressBar{}
	larges := make([]*odb.Entry, 0, 10)
	switch e.Type() {
	case object.BlobObject:
		larges = append(larges, &odb.Entry{Hash: e.Hash, Size: e.Size})
	case object.FragmentsObject:
		ff, err := w.odb.Fragments(ctx, e.Hash)
		if err != nil {
			return err
		}
		for _, fe := range ff.Entries {
			larges = append(larges, &odb.Entry{Hash: fe.Hash, Size: int64(fe.Size)})
		}
	default:
		return nil
	}
	if err = w.transfer(ctx, t, larges); err != nil {
		return err
	}
	if err := w.checkoutFile(ctx, name, e, bar); err != nil {
		if plumbing.IsNoSuchObject(err) && w.missingNotFailure {
			w.addPseudoIndex(name, e, b)
			return nil
		}
		if filemode.IsErrMalformedMode(err) {
			_, _ = term.Fprintf(os.Stderr, "\x1b[2K\rskip checkout '\x1b[31m%s\x1b[0m': malformed mode '%s'\n", name, e.Mode)
			w.addPseudoIndex(name, e, b)
			return nil
		}
		return err
	}
	if err := w.addIndexFromFile(name, e.Hash, e.Mode, b); err != nil {
		return err
	}
	for _, e := range larges {
		_ = w.odb.PruneObject(ctx, e.Hash, false)
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) checkoutOneAfterAnother0(ctx context.Context, entries []*odb.TreeEntry) error {
	_, _ = tr.Fprintf(os.Stderr, "Start checkout large files, total: %d\n", len(entries))
	t, err := w.newTransport(ctx, transport.DOWNLOAD)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := w.checkoutOne(ctx, t, e.Path, e.TreeEntry); err != nil {
			return err
		}
		_, _ = tr.Fprintf(os.Stderr, "Checkout '%s' success.\n", e.Path)
	}
	_, _ = tr.Fprintf(os.Stderr, "Checkout one after another, total: %d\n", len(entries))
	return nil
}

func (w *Worktree) checkoutOneAfterAnother(ctx context.Context) error {
	cc, err := w.parseRevExhaustive(ctx, "HEAD")
	if err != nil {
		return err
	}
	root, err := cc.Root(ctx)
	if err != nil {
		return err
	}
	changes, err := w.diffTreeWithWorktree(ctx, root, false)
	if err != nil {
		return err
	}
	entries := make([]*odb.TreeEntry, 0, len(changes))
	for _, ch := range changes {
		a, err := ch.Action()
		if err != nil {
			return err
		}
		if a == merkletrie.Insert {
			// ignore insert files
			continue
		}
		name := ch.From.String()
		e, err := w.resolveTreeEntry(ch.From)
		if err != nil {
			return err
		}
		w.DbgPrint("resolve entry: %v", e)
		entries = append(entries, &odb.TreeEntry{Path: name, TreeEntry: e})
	}
	return w.checkoutOneAfterAnother0(ctx, entries)
}

type treeEntries struct {
	larges []*odb.TreeEntry
	small  []*odb.TreeEntry
}

func (t *treeEntries) append(e *odb.TreeEntry, largeSize int64) {
	if e.Size > largeSize {
		t.larges = append(t.larges, e)
		return
	}
	t.small = append(t.small, e)
}

func (w *Worktree) resolveBatchObjects(ctx context.Context, root *object.Tree, r io.Reader) (*missingFetcher, *treeEntries, error) {
	br := bufio.NewScanner(r)
	m := newMissingFetcher()
	matcher := noder.NewSparseMatcher(w.Core.SparseDirs)
	entries := &treeEntries{
		larges: make([]*odb.TreeEntry, 0, 100),
		small:  make([]*odb.TreeEntry, 0, 100),
	}
	largeSize := w.largeSize()
	for br.Scan() {
		p := path.Clean(strings.TrimSpace(br.Text()))
		if p == "." || !matcher.Match(p) {
			// NOT match sparse rules
			continue
		}
		e, err := root.FindEntry(ctx, p)
		if err != nil {
			return nil, nil, err
		}
		entries.append(&odb.TreeEntry{Path: p, TreeEntry: e}, largeSize)
		if e.Type() == object.BlobObject {
			m.store(w.odb, e.Hash, e.Size, largeSize)
			continue
		}
		if e.Type() == object.FragmentsObject {
			fe, err := w.odb.Fragments(ctx, e.Hash)
			if err != nil {
				return nil, nil, err
			}
			for _, ee := range fe.Entries {
				m.store(w.odb, ee.Hash, int64(ee.Size), largeSize)
			}
			continue
		}
	}
	if br.Err() != nil {
		return nil, nil, br.Err()
	}
	return m, entries, nil
}

func (w *Worktree) DoBatchCo(ctx context.Context, oneByOne bool, revision string, r io.Reader) error {
	oid, err := w.resolveRevision(ctx, revision)
	if err != nil {
		return err
	}
	cc, err := w.odb.Commit(ctx, oid)
	if err != nil {
		return err
	}
	root, err := cc.Root(ctx)
	if err != nil {
		return err
	}
	m, t, err := w.resolveBatchObjects(ctx, root, r)
	if err != nil {
		return err
	}
	if err := w.fetchMissingObjects(ctx, m, oneByOne); err != nil {
		return err
	}
	entries := t.small
	if !oneByOne {
		entries = append(entries, t.larges...)
	}
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", uint64(len(entries)), w.quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.resetWorktreeEntries(ctx, entries, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	if oneByOne {
		return w.checkoutOneAfterAnother0(ctx, t.larges)
	}
	return nil
}

func (w *Worktree) checkoutDotWorktreeOnly(ctx context.Context, bar ProgressBar) error {
	changes, err := w.diffStagingWithWorktree(ctx, false, false)
	if err != nil {
		return err
	}
	changes = rearrangeChanges(changes)
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			return err
		}
		if action == merkletrie.Insert {
			continue
		}
		name := ch.From.String()
		e, err := w.resolveIndexEntry(ch.From)
		if err != nil {
			return err
		}
		if err = w.checkoutFile(ctx, name, e, bar); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worktree) checkoutWorktreeOnly(ctx context.Context, root *object.Tree, removedFiles []string, bar ProgressBar) error {
	changes, err := w.diffTreeWithWorktree(ctx, root, false)
	if err != nil {
		return err
	}
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			return err
		}
		name := nameFromAction(&ch)

		switch action {
		case merkletrie.Insert:
			//only remove files what index tracked before
			if !slices.ContainsFunc(removedFiles, func(s string) bool { return systemCaseEqual(s, name) }) {
				continue
			}
			w.fs.Remove(name)
		default:
			//checkout deleted and modified file
			e, err := w.resolveTreeEntry(ch.From)
			if err != nil {
				return err
			}
			if err = w.checkoutFile(ctx, name, e, bar); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Worktree) checkoutDot0(ctx context.Context, worktreeOnly bool, root *object.Tree, bar ProgressBar) error {
	if worktreeOnly {
		return w.checkoutDotWorktreeOnly(ctx, bar)
	}
	changes, err := w.diffTreeWithWorktree(ctx, root, false)
	if err != nil {
		return err
	}
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			return err
		}
		name := nameFromAction(&ch)
		// only checkout deleted and modified file
		if action == merkletrie.Insert {
			continue
		}
		e, err := w.resolveTreeEntry(ch.From)
		if err != nil {
			return err
		}
		if err = w.checkoutFile(ctx, name, e, bar); err != nil {
			return err
		}
		if err := w.addIndexFromFile(name, e.Hash, e.Mode, b); err != nil {
			return err
		}
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) checkoutDot(ctx context.Context, worktreeOnly bool, root *object.Tree) error {
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", 0, w.quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.checkoutDot0(ctx, worktreeOnly, root, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	return nil
}

func (w *Worktree) doPathCheckoutWorktreeOnly(ctx context.Context, patterns []string) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	m := NewMatcher(patterns)
	entries := make([]*odb.TreeEntry, 0, 100)
	for _, e := range idx.Entries {
		if !m.Match(e.Name) {
			continue
		}
		entries = append(entries,
			&odb.TreeEntry{
				Path: e.Name,
				TreeEntry: &object.TreeEntry{
					Name: filepath.Base(e.Name),
					Size: int64(e.Size),
					Mode: e.Mode,
					Hash: e.Hash,
				}})
	}
	ci := newMissingFetcher()
	largeSize := w.largeSize()
	for _, e := range entries {
		switch e.Type() {
		case object.BlobObject:
			ci.store(w.odb, e.Hash, e.Size, largeSize)
		case object.FragmentsObject:
			fragmentEntry, err := w.odb.Fragments(ctx, e.Hash)
			if err != nil {
				return fmt.Errorf("open fragments: %w", err)
			}
			for _, ee := range fragmentEntry.Entries {
				ci.store(w.odb, ee.Hash, int64(ee.Size), largeSize)
			}
		default:
		}
	}
	if err := w.fetchMissingObjects(ctx, ci, false); err != nil {
		return err
	}
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", uint64(len(entries)), w.quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.resetWorktreeEntriesWorktreeOnly(ctx, entries, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	return nil
}

func (w *Worktree) DoPathCo(ctx context.Context, worktreeOnly bool, oid plumbing.Hash, pathSpec []string) error {
	cc, err := w.odb.ParseRevExhaustive(ctx, oid)
	if err != nil {
		return err
	}
	root, err := cc.Root(ctx)
	if err != nil {
		return err
	}
	patterns, hasDot, err := w.cleanPatterns(pathSpec)
	if err != nil {
		return err
	}
	if hasDot {
		w.DbgPrint("checkout all files")
		return w.checkoutDot(ctx, worktreeOnly, root)
	}
	if worktreeOnly {
		return w.doPathCheckoutWorktreeOnly(ctx, pathSpec)
	}
	entries, err := w.lsTreeRecurseFilter(ctx, root, NewMatcher(patterns))
	if err != nil {
		return err
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
				return fmt.Errorf("open fragments: %w", err)
			}
			for _, ee := range fragmentEntry.Entries {
				ci.store(w.odb, ee.Hash, int64(ee.Size), largeSize)
			}
		default:
		}
	}
	if err := w.fetchMissingObjects(ctx, ci, false); err != nil {
		return err
	}
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", 0, w.quiet)
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
