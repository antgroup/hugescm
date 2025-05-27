// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/modules/zeta/reflog"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

const (
	StashName plumbing.ReferenceName = "refs/stash"
)

func (w *Worktree) doStashUpdate(ro *reflog.Reflog) error {
	if ro.Empty() {
		if err := w.ReferenceRemove(plumbing.NewHashReference(StashName, plumbing.ZeroHash)); err != nil {
			return err
		}
		return w.rdb.Delete(StashName)
	}
	N := ro.Entries[0].N
	if N.IsZero() {
		return fmt.Errorf("reflog: bad commit %s", N)
	}
	if err := w.ReferenceUpdate(plumbing.NewHashReference(StashName, N), nil); err != nil {
		return err
	}
	return w.rdb.Write(ro)
}

func (w *Worktree) checkStashRev(stashRev string) (int, error) {
	_, index, err := parseReflogRev(stashRev)
	if err != nil || index < 0 {
		die_error("%s is not a valid reference", stashRev)
		return 0, err
	}
	if !w.rdb.Exists(StashName) {
		fmt.Fprintln(os.Stderr, W("No stash entries found."))
		return 0, errors.New("no stash entries found")
	}
	return index, nil
}

func (w *Worktree) readStashRev(stashRev string) (*reflog.Entry, error) {
	index, err := w.checkStashRev(stashRev)
	if err != nil {
		return nil, err
	}
	ro, err := w.rdb.Read(StashName)
	if err != nil {
		die("read reflog: %v", err)
		return nil, err
	}
	if index >= len(ro.Entries) {
		fmt.Fprintln(os.Stderr, W("No stash entries found."))
		return nil, errors.New("no stash entries found")
	}
	return ro.Entries[index], nil
}

// Stash feature

type StashPushOptions struct {
	U bool
}

func (w *Worktree) restoreIndex(ctx context.Context, treeOID plumbing.Hash) error {
	tree, err := w.odb.Tree(ctx, treeOID)
	if err != nil {
		return err
	}
	_, err = w.resetIndex(ctx, tree)
	return err
}

type stashStoreResult struct {
	stashIndexTree    plumbing.Hash
	stashWorktreeTree plumbing.Hash
	stashIndex        plumbing.Hash
	stashWorktree     plumbing.Hash
}

func (w *Worktree) stashStore(ctx context.Context, base *object.Commit, committer *object.Signature, includeUntracked bool, messageIndex, messageWorktree string) (*stashStoreResult, error) {
	stashIndexTree, err := w.writeIndexAsTree(ctx, base.Tree, false)
	if err != nil {
		die_error("write index as tree: %v", err)
		return nil, err
	}
	stash0, err := w.commitTree(ctx, &CommitTreeOptions{
		Tree:      stashIndexTree,
		Author:    *committer,
		Committer: *committer,
		Parents:   []plumbing.Hash{base.Hash},
		Message:   messageIndex,
	})
	if err != nil {
		die("create index commit: %v", err)
		return nil, err
	}
	w.DbgPrint("new stash commit: %s", stash0)
	if includeUntracked {
		if _, err = w.doAdd(ctx, ".", w.Excludes, false, false); err != nil {
			die("add all: %v", err)
			return nil, err
		}
	} else {
		if err = w.autoAddModifiedAndDeleted(ctx); err != nil {
			die("add: %v", err)
			return nil, err
		}
	}
	stashWorktree, err := w.writeIndexAsTree(ctx, base.Hash, false)
	if err != nil {
		die("restore index. commit unstaged changes error: %v", err)
		_ = w.restoreIndex(ctx, stashIndexTree)
		return nil, err
	}
	newRev, err := w.commitTree(ctx, &CommitTreeOptions{
		Tree:      stashWorktree,
		Author:    *committer,
		Committer: *committer,
		Parents:   []plumbing.Hash{base.Hash, stash0},
		Message:   messageWorktree,
	})
	if err != nil {
		die("restore index. create commit error: %v", err)
		_ = w.restoreIndex(ctx, stashIndexTree)
		return nil, err
	}
	return &stashStoreResult{stashIndexTree: stashIndexTree, stashWorktreeTree: stashWorktree, stashIndex: stash0, stashWorktree: newRev}, nil
}

func (w *Worktree) StashPush(ctx context.Context, opts *StashPushOptions) error {
	status, err := w.Status(context.Background(), false)
	if err != nil {
		die_error("status: %v", err)
		return err
	}
	if status.IsClean() {
		fmt.Fprintln(os.Stderr, W("No local changes to save"))
		return nil
	}
	current, err := w.Current()
	if err != nil {
		die("resolve current %s", err)
		return err
	}
	cc, err := w.odb.Commit(ctx, current.Hash())
	if err != nil {
		die("resolve current commit: %v", err)
		return err
	}
	committer := w.NewCommitter()
	messageIndex := fmt.Sprintf("index on %s: %s %s\n", current.Name().Short(), shortHash(cc.Hash), cc.Subject())
	messageWorktree := fmt.Sprintf("WIP on %s: %s %s\n", current.Name().Short(), shortHash(cc.Hash), cc.Subject())
	result, err := w.stashStore(ctx, cc, committer, opts.U, messageIndex, messageWorktree)
	if err != nil {
		return err
	}
	var oldRev plumbing.Hash
	old, err := w.Reference(StashName)
	if err != nil && err != plumbing.ErrReferenceNotFound {
		die("resolve refs/stash: %v", err)
		return err
	}
	if old != nil {
		oldRev = old.Hash()
	}
	if err := w.DoUpdate(ctx, StashName, oldRev, result.stashWorktree, committer, messageWorktree); err != nil {
		die("update-ref refs/stash: %v", err)
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: cc.Hash, Mode: MergeReset, Quiet: w.quiet}); err != nil {
		die_error("reset worktree error: %v", err)
		return err
	}
	return nil
}

func (w *Worktree) StashList(ctx context.Context) error {
	if !w.rdb.Exists(StashName) {
		return nil
	}
	writer := NewPrinter(ctx)
	defer writer.Close() // nolint
	ro, err := w.rdb.Read(StashName)
	if err != nil {
		return err
	}
	for i, e := range ro.Entries {
		fmt.Fprintf(writer, "stash@{%d}: %s\n", i, e.Message)
	}
	return nil
}

func (w *Worktree) StashShow(ctx context.Context, stashRev string) error {
	e, err := w.readStashRev(stashRev)
	if err != nil {
		return err
	}
	w.DbgPrint("new checksum %v", e.N)
	cc, err := w.odb.Commit(ctx, e.N)
	if err != nil {
		die_error("open HEAD: %v", err)
		return err
	}
	stats, err := cc.StatsContext(ctx, noder.NewSparseTreeMatcher(w.Core.SparseDirs), &object.PatchOptions{})
	if plumbing.IsNoSuchObject(err) {
		fmt.Fprintf(os.Stderr, "incomplete checkout, skipping change line count statistics\n")
		return nil
	}
	if err != nil {
		die_error("stats: %v", err)
		return err
	}
	var added, deleted int
	for _, s := range stats {
		added += s.Addition
		deleted += s.Deletion
	}
	p := NewPrinter(ctx)
	defer p.Close() // nolint
	object.StatsWriteTo(p, stats, p.ColorMode() != term.LevelNone)
	fmt.Fprintf(p, "%d files changed, %d insertions(+), %d deletions(-)\n", len(stats), added, deleted)
	return nil
}

func (w *Worktree) stashApplyTree(ctx context.Context, I, W plumbing.Hash) error {
	// restore index
	treeI, err := w.odb.Tree(ctx, I)
	if err != nil {
		return err
	}
	if _, err := w.resetIndex(ctx, treeI); err != nil {
		return err
	}
	treeW, err := w.odb.Tree(ctx, W)
	if err != nil {
		return err
	}

	removedFiles := []string{}
	changes, err := w.diffTreeWithTree(ctx, treeI, treeW, false)
	if err != nil {
		return err
	}
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			return err
		}
		name := nameFromAction(&ch)
		if action == merkletrie.Delete {
			removedFiles = append(removedFiles, name)
		}
	}

	if err := w.checkoutWorktreeOnly(ctx, treeW, removedFiles, nonProgressBar{}); err != nil {
		return err
	}
	return nil
}

var (
	ErrNotAStashLikeCommit = errors.New("not a stash-like commit")
)

type mergeStashResult struct {
	newIndexTree    plumbing.Hash
	newWorktreeTree plumbing.Hash
	conflicts       []*odb.Conflict
}

func (r *mergeStashResult) format() {
	for _, e := range r.conflicts {
		if e.Ancestor.Path != "" {
			fmt.Fprintf(os.Stdout, "conflict: %s\n", e.Ancestor.Path)
			continue
		}
		if e.Our.Path != "" {
			fmt.Fprintf(os.Stdout, "conflict: %s\n", e.Our.Path)
			continue
		}
		if e.Their.Path != "" {
			fmt.Fprintf(os.Stdout, "conflict: %s\n", e.Their.Path)
			continue
		}
	}
}

func (w *Worktree) mergeStash(ctx context.Context, stashIndex, stashWorktree, currentIndex, currentWorktree *object.Commit) (*mergeStashResult, error) {
	indexBases, err := currentIndex.MergeBase(ctx, stashIndex)
	if err != nil {
		return nil, err
	}
	if len(indexBases) == 0 {
		return nil, ErrUnrelatedHistories
	}
	o, err := indexBases[0].Root(ctx)
	if err != nil {
		return nil, err
	}
	a, err := currentIndex.Root(ctx)
	if err != nil {
		return nil, err
	}
	b, err := stashIndex.Root(ctx)
	if err != nil {
		return nil, err
	}
	mergeDriver := w.resolveMergeDriver()
	mr, err := w.odb.MergeTree(ctx, o, a, b, &odb.MergeOptions{
		Branch1:       "CurrentIndex",
		Branch2:       "StashIndex",
		DetectRenames: true,
		Textconv:      false,
		MergeDriver:   mergeDriver,
	})
	if err != nil {
		return nil, err
	}
	if len(mr.Conflicts) != 0 {
		return nil, ErrHasConflicts
	}
	worktreeBases, err := currentWorktree.MergeBase(ctx, stashWorktree)
	if err != nil {
		return nil, err
	}
	if len(worktreeBases) == 0 {
		return nil, ErrUnrelatedHistories
	}
	o2, err := worktreeBases[0].Root(ctx)
	if err != nil {
		return nil, err
	}
	a2, err := currentWorktree.Root(ctx)
	if err != nil {
		return nil, err
	}
	b2, err := stashWorktree.Root(ctx)
	if err != nil {
		return nil, err
	}
	mr1, err := w.odb.MergeTree(ctx, o2, a2, b2, &odb.MergeOptions{
		Branch1:       "CurrentWorktree",
		Branch2:       "StashWorktree",
		DetectRenames: true,
		Textconv:      false,
		MergeDriver:   mergeDriver,
	})
	if err != nil {
		return nil, err
	}
	return &mergeStashResult{newIndexTree: mr.NewTree, newWorktreeTree: mr1.NewTree, conflicts: mr1.Conflicts}, nil
}

func (w *Worktree) stashApply(ctx context.Context, e *reflog.Entry) error {
	stashWorktree, err := w.odb.Commit(ctx, e.N)
	if err != nil {
		die_error("zeta stash apply: resolve '%s' error: %v", e.N, err)
		return err
	}
	if len(stashWorktree.Parents) != 2 {
		die("'%s' is not a stash-like commit", e.N)
		return ErrNotAStashLikeCommit
	}
	stashIndex, err := w.odb.Commit(ctx, stashWorktree.Parents[1])
	if err != nil {
		die_error("zeta stash apply: resolve index commit error: %v", err)
		return err
	}
	if len(stashIndex.Parents) == 0 || stashIndex.Parents[0] != stashWorktree.Parents[0] {
		die("'%s' is not a stash-like commit", e.N)
		return ErrNotAStashLikeCommit
	}
	w.DbgPrint("worktree %s index %s", stashWorktree.Hash, stashIndex.Hash)
	oid, err := w.resolveRevision(ctx, "HEAD")
	if err != nil {
		die_error("zeta stash apply: resolve 'HEAD': %v", err)
		return err
	}
	status, err := w.Status(context.Background(), false)
	if err != nil {
		die_error("status: %v", err)
		return err
	}
	if status.IsClean() && oid == stashWorktree.Parents[0] {
		if err := w.stashApplyTree(ctx, stashIndex.Tree, stashWorktree.Tree); err != nil {
			die_error("zeta stash apply: reset index worktree: %v", err)
			return err
		}
		return nil
	}
	cc, err := w.odb.Commit(ctx, oid)
	if err != nil {
		die_error("zeta stash apply: unable open '%s' error: %v", oid, err)
		return err
	}
	if status.IsClean() {
		result, err := w.mergeStash(ctx, stashIndex, stashWorktree, cc, cc)
		if err != nil {
			if err == ErrHasConflicts {
				die_error("conflicts in index.")
				return err
			}
			die_error("merge stash error: %v", err)
			return err
		}
		if err := w.stashApplyTree(ctx, result.newIndexTree, result.newWorktreeTree); err != nil {
			die_error("zeta stash apply: reset index worktree: %v", err)
			return err
		}
		if len(result.conflicts) != 0 {
			result.format()
			die_error("zeta stash apply: worktree has conflict")
			return ErrHasConflicts
		}
		return nil
	}
	committer := w.NewCommitter()
	storeResult, err := w.stashStore(ctx, cc, committer, false, "auto stash index", "auto stash worktree")
	if err != nil {
		return err
	}
	currentIndex, err := w.odb.Commit(ctx, storeResult.stashIndex)
	if err != nil {
		die_error("unable open commit: %v", err)
		return err
	}
	currentWorktree, err := w.odb.Commit(ctx, storeResult.stashWorktree)
	if err != nil {
		die_error("unable open commit: %v", err)
		return err
	}
	result, err := w.mergeStash(ctx, stashIndex, stashWorktree, currentIndex, currentWorktree)
	if err != nil {
		_ = w.stashApplyTree(ctx, storeResult.stashIndexTree, storeResult.stashWorktreeTree)
		if err == ErrHasConflicts {
			die_error("conflicts in index.")
			return err
		}
		die_error("merge stash error: %v", err)
		return err
	}
	if err := w.stashApplyTree(ctx, result.newIndexTree, result.newWorktreeTree); err != nil {
		die_error("zeta stash apply: reset index worktree: %v", err)
		return err
	}
	if len(result.conflicts) != 0 {
		result.format()
		die_error("zeta stash apply: worktree has conflict")
		return ErrHasConflicts
	}
	return nil
}

func (w *Worktree) StashApply(ctx context.Context, stashRev string) error {
	e, err := w.readStashRev(stashRev)
	if err != nil {
		return err
	}
	w.DbgPrint("new checksum %v", e.N)
	return w.stashApply(ctx, e)
}

func (w *Worktree) StashPop(ctx context.Context, stashRev string) error {
	index, err := w.checkStashRev(stashRev)
	if err != nil {
		return err
	}
	ro, err := w.rdb.Read(StashName)
	if err != nil {
		die("read reflog: %v", err)
		return err
	}
	if index >= len(ro.Entries) {
		fmt.Fprintln(os.Stderr, W("No stash entries found."))
		return errors.New("no stash entries found")
	}
	e := ro.Entries[index]
	if err := w.stashApply(ctx, e); err != nil {
		return err
	}
	if err := ro.Drop(index, true); err != nil {
		die_error("zeta stash pop: %s", err)
		return err
	}
	if err := w.doStashUpdate(ro); err != nil {
		die_error("zeta stash pop: %v", err)
		return err
	}
	return nil
}

func (w *Worktree) StashClear(ctx context.Context) error {
	if !w.rdb.Exists(StashName) {
		return nil
	}
	ro, err := w.rdb.Read(StashName)
	if err != nil {
		die("read reflog: %v", err)
		return err
	}
	ro.Clear()
	if err := w.doStashUpdate(ro); err != nil {
		die_error("zeta stash clear: %v", err)
		return err
	}
	return nil
}

func (w *Worktree) StashDrop(ctx context.Context, stashRev string) error {
	index, err := w.checkStashRev(stashRev)
	if err != nil {
		return err
	}
	ro, err := w.rdb.Read(StashName)
	if err != nil {
		die("read reflog: %v", err)
		return err
	}
	if err := ro.Drop(index, true); err != nil {
		die_error("zeta stash drop: %v", err)
		return err
	}
	if err := w.doStashUpdate(ro); err != nil {
		die_error("zeta stash drop: %v", err)
		return err
	}
	return nil
}
