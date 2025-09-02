// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

// MergeOptions describes how a merge should be performed
type MergeOptions struct {
	From string // From branch
	// Requires a merge to be fast forward only. If this is true, then a merge will
	// throw an error if ff is not possible.
	FFOnly                            bool
	FF                                bool
	Squash                            bool
	Signoff                           bool
	AllowUnrelatedHistories, Textconv bool
	Abort                             bool
	Continue                          bool
	Message                           []string
	File                              string
}

// 1 Merge branch 'dev-1' into dev-2
/*
Merge branch 'dev-1' into dev-2
# 请输入一个提交信息以解释此合并的必要性，尤其是将一个更新后的上游分支
# 合并到主题分支。
#
# 以 '#' 开始的行将被忽略，而空的提交说明将终止提交。
*/
func (w *Worktree) mergeMessageFromPrompt(ctx context.Context, messagePrefix string) (string, error) {
	if !term.IsTerminal(os.Stdin.Fd()) || !env.ZETA_TERMINAL_PROMPT.SimpleAtob(true) {
		return "", nil
	}
	p := filepath.Join(w.odb.Root(), MERGE_MSG)
	message := strengthen.StrCat(messagePrefix,
		"\n# ", W("Please enter a commit message to explain why this merge is necessary,"),
		"\n# ", W("especially if it merges an updated upstream into a topic branch."),
		"\n#\n# ", tr.Sprintf("Lines starting with '%c' will be ignored, and an empty message aborts.", '#'),
	)
	if err := os.WriteFile(p, []byte(message), 0644); err != nil {
		return "", err
	}
	if err := launchEditor(ctx, w.coreEditor(), p, nil); err != nil {
		return "", nil
	}
	return messageReadFromPath(p)
}

func (w *Worktree) mergeMessageGen(ctx context.Context, opts *MergeOptions, branchName string) (string, error) {
	switch {
	case opts.File == "-":
		return messageReadFrom(os.Stdin)
	case len(opts.File) != 0:
		return messageReadFromPath(opts.File)
	case len(opts.Message) == 0:
		messagePrefix := fmt.Sprintf("Merge branch '%s' into %s", opts.From, branchName)
		return w.mergeMessageFromPrompt(ctx, messagePrefix)
	}
	return genMessage(opts.Message), nil
}

func (w *Worktree) mergeFF(ctx context.Context, parent1, parent2 plumbing.Hash, message string) (plumbing.Hash, error) {
	select {
	case <-ctx.Done():
		return plumbing.ZeroHash, ctx.Err()
	default:
	}
	c0, err := w.odb.Commit(ctx, parent1)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	c1, err := w.odb.Commit(ctx, parent2)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	committer := w.NewCommitter()
	cc := &object.Commit{
		Tree:      c0.Tree,
		Author:    *committer,
		Committer: *committer,
		Parents:   []plumbing.Hash{c0.Hash, c1.Hash},
		Message:   message,
	}
	return w.odb.WriteEncoded(cc)
}

func (w *Worktree) Merge(ctx context.Context, opts *MergeOptions) error {
	if opts.Abort {
		return w.mergeAbort(ctx)
	}
	if opts.Continue {
		return w.mergeContinue(ctx)
	}
	if len(opts.From) == 0 {
		die_error("zeta merge require revision argument")
		return ErrAborting
	}

	s, err := w.Status(ctx, false)
	if err != nil {
		die_error("status: %v", err)
		return err
	}
	if !s.IsClean() {
		fmt.Fprintln(os.Stderr, W("Please commit or stash them."))
		return ErrAborting
	}

	current, err := w.Current()
	if err != nil {
		die("resolve current %s", err)
		return err
	}
	currentName := current.Name()
	if !currentName.IsBranch() {
		die_error("reference '%s' not branch", currentName)
		return errors.New("reference not branch")
	}
	branchName := currentName.BranchName()
	from, err := w.Revision(ctx, opts.From)
	if err != nil {
		die_error("rev-parse %s error: %v", opts.From, err)
		return err
	}
	var ignoreParents []plumbing.Hash
	deepenFrom, err := w.odb.DeepenFrom()
	if err != nil && !os.IsNotExist(err) {
		die_error("check shallow: %v", err)
		return err
	}
	if !deepenFrom.IsZero() {
		d, err := w.odb.Commit(ctx, deepenFrom)
		if err != nil {
			die_error("resolve shallow commit %s: %v", deepenFrom, err)
			return err
		}
		ignoreParents = append(ignoreParents, d.Parents...)
	}
	headAheadOfRef, err := w.isFastForward(ctx, from, current.Hash(), ignoreParents)
	if err != nil {
		die_error("check fast-forward error: %v", err)
		return err
	}
	// already up to date
	if headAheadOfRef {
		fmt.Fprintln(os.Stderr, W("Already up to date."))
		return nil
	}

	fastForward, err := w.isFastForward(ctx, current.Hash(), from, ignoreParents)
	if err != nil {
		die_error("check fast-forward error: %v", err)
		return err
	}
	if fastForward {
		newRev := from
		if !opts.FF {
			message, err := w.mergeMessageGen(ctx, opts, branchName)
			if err != nil {
				die_error("unable resolve merge message")
				return err
			}
			if len(message) == 0 {
				return ErrAborting
			}
			if newRev, err = w.mergeFF(ctx, newRev, current.Hash(), message); err != nil {
				die_error("merge FF error")
				return err
			}
		}
		trace.DbgPrint("update-ref %s %s", current.Hash(), newRev)
		if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "merge: Fast-forward"); err != nil {
			die_error("update fast forward: %v", err)
			return err
		}
		if err := w.Reset(ctx, &ResetOptions{Commit: newRev, Mode: MergeReset}); err != nil {
			die_error("reset worktree: %v", err)
			return err
		}
		fmt.Fprintf(os.Stderr, "%s %s..%s\nFast-forward\n", W("Updating"), shortHash(current.Hash()), shortHash(newRev))
		_ = w.mergeStat(ctx, current.Hash(), newRev)
		return nil
	}
	if opts.FFOnly {
		fmt.Fprintln(os.Stderr, W("Not possible to fast-forward, aborting."))
		return ErrNonFastForwardUpdate
	}
	newRev, err := w.mergeInternal(ctx, current.Hash(), from, branchName, opts.From, opts.Squash, opts.AllowUnrelatedHistories, opts.Textconv, opts.Signoff, func() string {
		message, _ := w.mergeMessageGen(ctx, opts, branchName)
		return message
	})
	if err != nil {
		return err
	}
	messagePrefix := fmt.Sprintf("Merge branch '%s' into %s", opts.From, branchName)
	if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "merge: "+messagePrefix); err != nil {
		die_error("update fast forward: %v", err)
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: newRev, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s..%s\nMerge completed\n", W("Updating"), shortHash(current.Hash()), shortHash(newRev))
	_ = w.mergeStat(ctx, current.Hash(), newRev)
	return nil
}

const maxSizeForSquashedCommitMessage = 4 << 10

func (w *Worktree) makeSquashMessage(ctx context.Context, from plumbing.Hash, ignore []plumbing.Hash, messagePrefix string) (string, error) {
	commits, err := w.revList(ctx, from, ignore, LogOrderTopo, nil)
	if err != nil {
		die_error("log range base error: %v", err)
		return "", err
	}
	var b strings.Builder
	b.WriteString(messagePrefix)
	b.WriteString("\nSquashed commit of the following:\n")
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		subject := c.Subject()
		if b.Len()+len(subject) >= maxSizeForSquashedCommitMessage {
			fmt.Fprintf(&b, "\n...\n[ZETA] %d more commit(s) ignored to avoid oversized message\n", i)
			break
		}
		fmt.Fprintf(&b, "\n* %s: %s\n", shortHash(c.Hash), subject)
	}
	return b.String(), nil
}

func (w *Worktree) mergeInternal(ctx context.Context, into, from plumbing.Hash, branch1, branch2 string, squash, allowUnrelatedHistories, textconv, signoff bool, messageFn func() string) (plumbing.Hash, error) {
	c1, err := w.odb.Commit(ctx, into)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	c2, err := w.odb.Commit(ctx, from)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	result, err := w.mergeTree(ctx, c1, c2, nil, branch1, branch2, allowUnrelatedHistories, textconv)
	if err != nil {
		if mr, ok := err.(*odb.MergeResult); ok {
			for _, m := range mr.Messages {
				fmt.Fprintln(os.Stderr, m)
			}
			return plumbing.ZeroHash, ErrHasConflicts
		}
		return plumbing.ZeroHash, err
	}
	for _, m := range result.Messages {
		fmt.Fprintln(os.Stderr, m)
	}
	if len(result.Conflicts) != 0 {
		if err := w.checkoutMergeConflicts(ctx, c1, c2, result.MergeResult); err != nil {
			die_error("checkout conflict tree: %v", err)
		}
		fmt.Fprintln(os.Stderr, W("Automatic merge failed; fix conflicts and then commit the result."))
		return plumbing.ZeroHash, ErrHasConflicts
	}
	parents := []plumbing.Hash{into}
	message := messageFn()
	if squash {
		if message, err = w.makeSquashMessage(ctx, from, result.bases, message); err != nil {
			return plumbing.ZeroHash, err
		}
	} else {
		parents = append(parents, from)
	}

	if len(message) == 0 {
		die_error("No merge message -- not updating HEAD")
		return plumbing.ZeroHash, ErrAborting
	}
	committer := w.NewCommitter()
	if signoff {
		message = fmt.Sprintf("%s\n\nSigned-off-by: %s <%s>\n", strings.TrimRightFunc(message, unicode.IsSpace), committer.Name, committer.Email)
	}
	cc, err := w.commitTree(ctx, &CommitTreeOptions{
		Tree:      result.NewTree,
		Author:    *committer,
		Committer: *committer,
		Parents:   parents,
		Message:   message,
	})
	if err != nil {
		die_error("zeta commit-tree error: %v", err)
		return plumbing.ZeroHash, err
	}
	return cc, nil
}

func (w *Worktree) checkoutConflicts(ctx context.Context, tree, newTree *object.Tree, conflicts []*odb.Conflict) error {
	if _, err := w.resetIndex(ctx, tree); err != nil {
		return err
	}
	conflictPaths := make(map[string]bool)
	for _, c := range conflicts {
		if len(c.Our.Path) != 0 {
			conflictPaths[c.Our.Path] = true
		}
		if len(c.Ancestor.Path) != 0 {
			conflictPaths[c.Ancestor.Path] = true
		}
		if len(c.Their.Path) != 0 {
			conflictPaths[c.Their.Path] = true
		}
	}
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)
	changes, err := w.diffTreeWithWorktree(ctx, newTree, false)
	if err != nil {
		return err
	}
	bar := nonProgressBar{}
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
		if conflictPaths[name] {
			continue
		}
		if err := w.addIndexFromFile(name, e.Hash, e.Mode, b); err != nil {
			return err
		}
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) checkoutMergeConflicts(ctx context.Context, into, from *object.Commit, result *odb.MergeResult) error {
	tree0, err := into.Root(ctx)
	if err != nil {
		return err
	}
	if err := w.odb.SpecReferenceUpdate(odb.MERGE_HEAD, from.Hash); err != nil {
		return err
	}
	root, err := w.odb.Tree(ctx, result.NewTree)
	if err != nil {
		return err
	}
	return w.checkoutConflicts(ctx, tree0, root, result.Conflicts)
}

func (w *Worktree) mergeAbort(ctx context.Context) error {
	mergeHEAD, err := w.odb.ResolveSpecReference(odb.MERGE_HEAD)
	if err != nil {
		if os.IsNotExist(err) {
			die("zeta merge --abort: no valid merge found")
			return err
		}
		die("zeta merge --abort: %v", err)
		return err
	}
	trace.DbgPrint("MERGE_HEAD: %s", mergeHEAD)
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
	if err := w.Reset(ctx, &ResetOptions{Commit: cc.Hash, Mode: HardReset}); err != nil {
		die_error("zeta merge --abort: reset worktree error: %v", err)
		return err
	}
	_ = w.odb.SpecReferenceRemove(odb.MERGE_HEAD)
	return nil
}

func (w *Worktree) mergeContinue(ctx context.Context) error {
	mergeHEAD, err := w.odb.ResolveSpecReference(odb.MERGE_HEAD)
	if err != nil {
		if os.IsNotExist(err) {
			die("zeta merge --abort: no valid merge found")
			return err
		}
		die("zeta merge --abort: %v", err)
		return err
	}
	trace.DbgPrint("MERGE_HEAD: %s", mergeHEAD)
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
	mergeTree, err := w.writeIndexAsTree(ctx, cc.Hash, false)
	if err != nil {
		die_error("write index as tree: %v", err)
		return err
	}
	messagePrefix := fmt.Sprintf("Merge branch '%s' into %s", mergeHEAD, current.Name().Short())
	message, err := w.mergeMessageFromPrompt(ctx, messagePrefix)
	if err != nil {
		return err
	}
	if len(message) == 0 {
		return ErrAborting
	}
	committer := w.NewCommitter()
	newRev, err := w.commitTree(ctx, &CommitTreeOptions{
		Tree:      mergeTree,
		Author:    *committer,
		Committer: *committer,
		Parents:   []plumbing.Hash{cc.Hash, mergeHEAD},
		Message:   message,
	})
	if err != nil {
		die_error("zeta commit-tree error: %v", err)
		return err
	}
	if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "merge: "+messagePrefix); err != nil {
		die_error("update fast forward: %v", err)
		return err
	}
	_ = w.odb.SpecReferenceRemove(odb.MERGE_HEAD)
	return nil
}

func (w *Worktree) mergeStat(ctx context.Context, oldRev, newRev plumbing.Hash) error {
	if w.quiet {
		return nil
	}
	oldTree, err := w.getTreeFromHash(ctx, oldRev)
	if err != nil {
		die_error("unable read tree: %v error: %v", oldRev, err)
		return err
	}
	newTree, err := w.getTreeFromHash(ctx, newRev)
	if err != nil {
		die_error("unable read tree: %v error: %v", newRev, err)
		return err
	}
	o := &object.DiffTreeOptions{
		DetectRenames:    true,
		OnlyExactRenames: true,
	}
	changes, err := object.DiffTreeWithOptions(ctx, oldTree, newTree, o, noder.NewSparseTreeMatcher(w.Core.SparseDirs))
	if err != nil {
		die_error("unable diff tree: old %v new %v: %v", oldRev, newRev, err)
		return err
	}
	stats, err := changes.Stats(ctx, &object.PatchOptions{})
	if err != nil {
		return err
	}
	var added, deleted int
	for _, s := range stats {
		added += s.Addition
		deleted += s.Deletion
	}
	object.StatsWriteTo(os.Stderr, stats, term.StdoutLevel != term.LevelNone)
	fmt.Fprintf(os.Stdout, "%d files changed, %d insertions(+), %d deletions(-)\n", len(stats), added, deleted)
	return nil
}
