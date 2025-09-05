// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

type CherryPickOptions struct {
	From     string // From commit
	FF       bool
	Abort    bool
	Skip     bool
	Continue bool
}

type RevertOptions struct {
	From     string // From commit
	FF       bool
	Abort    bool
	Skip     bool
	Continue bool
}

// pick or revert -->
type ReplayMD struct {
	BASE       plumbing.Hash          `toml:"BASE"`
	LAST       plumbing.Hash          `toml:"LAST"`       // LAST
	MERGE_TREE plumbing.Hash          `toml:"MERGE_TREE"` // MERGE_TREE
	HEAD       plumbing.ReferenceName `toml:"HEAD"`       // HEAD aka CURRENT --> branch
}

const (
	REPLAY_MD = "REPLAY-MD"
)

func (w *Worktree) replayMD() (*ReplayMD, error) {
	var md ReplayMD
	_, err := toml.DecodeFile(filepath.Join(w.odb.Root(), REPLAY_MD), &md)
	if err != nil {
		return nil, err
	}
	return &md, nil
}

func (w *Worktree) CherryPick(ctx context.Context, opts *CherryPickOptions) error {
	if opts.Abort {
		return w.cherryPickAbort(ctx)
	}
	if opts.Continue {
		return w.cherryPickContinue(ctx)
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
		die_error("resolve HEAD: %v", err)
		return err
	}
	currentName := current.Name()
	if !currentName.IsBranch() {
		die_error("reference '%s' not branch", currentName)
		return errors.New("reference not branch")
	}
	branchName := currentName.BranchName()
	pickRev, err := w.Revision(ctx, opts.From)
	if err != nil {
		die_error("unable resolve onto %v", err)
		return err
	}
	trace.DbgPrint("Branch: %s pick %s", branchName, pickRev)
	ac, err := w.odb.Commit(ctx, current.Hash())
	if err != nil {
		die_error("zeta cherry-pick resolve 'HEAD' error: %v", err)
		return err
	}
	a, err := ac.Root(ctx)
	if err != nil {
		die_error("zeta cherry-pick resolve 'HEAD' root error: %v", err)
		return err
	}
	bc, err := w.odb.Commit(ctx, pickRev)
	if err != nil {
		die_error("zeta cherry-pick resolve '%s' error: %v", pickRev, err)
		return err
	}
	b, err := bc.Root(ctx)
	if err != nil {
		die_error("zeta cherry-pick resolve 'PICK' root error: %v", err)
		return err
	}
	var o *object.Tree
	if len(bc.Parents) != 0 {
		oc, err := w.odb.Commit(ctx, bc.Parents[0])
		if err != nil {
			die_error("zeta cherry-pick resolve base error: %v", err)
			return err
		}
		if o, err = oc.Root(ctx); err != nil {
			die_error("zeta cherry-pick resolve root tree error: %v", err)
			return err
		}
	} else {
		o = w.odb.EmptyTree()
	}
	result, err := w.odb.MergeTree(ctx, o, a, b, &odb.MergeOptions{
		Branch1:       currentName.BranchName(),
		Branch2:       opts.From,
		DetectRenames: true,
		Textconv:      false,
		MergeDriver:   w.resolveMergeDriver(),
		TextGetter:    w.readMissingText,
	})
	if err != nil {
		die_error("merge-tree: %v", err)
		return err
	}
	if len(result.Conflicts) != 0 {
		_ = w.checkoutReplayConflicts(ctx, &ReplayMD{
			BASE:       current.Hash(),
			LAST:       pickRev,
			MERGE_TREE: result.NewTree,
			HEAD:       current.Name(),
		}, result.Conflicts)
		// format error:
		fmt.Fprintln(os.Stderr, W("Cherry-pick failed; fix conflicts and then commit the result."))
		return ErrHasConflicts
	}
	committer := w.NewCommitter()
	cc := &object.Commit{
		Author:       bc.Author,
		Committer:    *committer,
		Parents:      []plumbing.Hash{ac.Hash},
		Tree:         result.NewTree,
		ExtraHeaders: bc.ExtraHeaders,
		Message:      bc.Message,
	}
	newRev, err := w.odb.WriteEncoded(cc)
	if err != nil {
		die_error("unable encode commit: %v", err)
		return err
	}
	messagePrefix := fmt.Sprintf("Cherry pick '%s' to %s", pickRev, branchName)
	if err := w.DoUpdate(ctx, current.Name(), bc.Hash, newRev, committer, "cherry-pick: "+messagePrefix); err != nil {
		die_error("update rebase: %v", err)
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: newRev, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s..%s\n", W("Updating"), shortHash(bc.Hash), shortHash(newRev))
	fmt.Fprintf(os.Stderr, W("Successfully cherry-pick and updated %s.\n"), current.Name())
	return nil
}

func (w *Worktree) cherryPickAbort(ctx context.Context) error {
	md, err := w.replayMD()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		die_error("zeta cherry-pick --abort: read 'REPLAY-MD': %v", err)
		return err
	}
	trace.DbgPrint("BASE: %s", md.BASE)
	HEAD := plumbing.NewSymbolicReference(plumbing.HEAD, md.HEAD)
	if err := w.Update(HEAD, nil); err != nil {
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: md.BASE, Mode: HardReset}); err != nil {
		die_error("zeta cherry-pick --abort: reset worktree error: %v", err)
		return err
	}
	_ = os.Remove(filepath.Join(w.odb.Root(), REPLAY_MD))
	return nil
}

func (w *Worktree) cherryPickContinue(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	md, err := w.replayMD()
	if err != nil {
		if os.IsNotExist(err) {
			die_error("zeta cherry-pick --continue: metadata 'REPLAY-MD' not found")
			return err
		}
		die_error("zeta cherry-pick --continue: read 'REPLAY-MD': %v", err)
		return err
	}
	trace.DbgPrint("%s --> %s", md.LAST, md.BASE)
	pc, err := w.odb.Commit(ctx, md.BASE)
	if err != nil {
		die_error("zeta cherry-pick --continue: unable resolve HEAD: %v", err)
		return err
	}
	last, err := w.odb.Commit(ctx, md.LAST)
	if err != nil {
		die_error("unable open last tree: %v", err)
		return err
	}
	resolvedTree, err := w.writeIndexAsTree(ctx, last.Tree, false)
	if err != nil {
		die_error("unable write resolved tree: %v", err)
		return err
	}
	trace.DbgPrint("conflicts resolved: %s", resolvedTree)
	committer := w.NewCommitter()
	cc := &object.Commit{
		Author:       last.Author,
		Committer:    *committer,
		Parents:      []plumbing.Hash{pc.Hash},
		Tree:         resolvedTree,
		ExtraHeaders: last.ExtraHeaders,
		Message:      last.Message,
	}
	lastCommitID, err := w.odb.WriteEncoded(cc)
	if err != nil {
		die_error("unable encode commit: %v", err)
		return err
	}
	branchName := md.HEAD.BranchName()
	messagePrefix := fmt.Sprintf("Cherry pick '%s' to %s", md.LAST, branchName)
	if err := w.DoUpdate(ctx, md.HEAD, pc.Hash, lastCommitID, committer, "cherry-pick: "+messagePrefix); err != nil {
		die_error("update rebase: %v", err)
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: lastCommitID, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s..%s\n", W("Updating"), shortHash(pc.Hash), shortHash(lastCommitID))
	fmt.Fprintf(os.Stderr, W("Successfully cherry-pick and updated %s.\n"), md.HEAD)
	_ = os.Remove(filepath.Join(w.odb.Root(), REPLAY_MD))
	return nil
}

func (w *Worktree) Revert(ctx context.Context, opts *RevertOptions) error {
	if opts.Abort {
		return w.revertAbort(ctx)
	}
	if opts.Continue {
		return w.revertContinue(ctx)
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
		die_error("resolve HEAD: %v", err)
		return err
	}
	currentName := current.Name()
	if !currentName.IsBranch() {
		die_error("reference '%s' not branch", currentName)
		return errors.New("reference not branch")
	}
	branchName := currentName.BranchName()
	revertRev, err := w.Revision(ctx, opts.From)
	if err != nil {
		die_error("unable resolve onto %v", err)
		return err
	}
	trace.DbgPrint("Branch: %s revert %s", branchName, revertRev)
	ac, err := w.odb.Commit(ctx, current.Hash())
	if err != nil {
		die_error("zeta revert resolve 'HEAD' error: %v", err)
		return err
	}
	a, err := ac.Root(ctx)
	if err != nil {
		die_error("zeta revert resolve 'HEAD' root error: %v", err)
		return err
	}
	oc, err := w.odb.Commit(ctx, revertRev)
	if err != nil {
		die_error("zeta revert resolve '%s' error: %v", revertRev, err)
		return err
	}
	o, err := oc.Root(ctx)
	if err != nil {
		die_error("zeta revert resolve 'REVERT' root error: %v", err)
		return err
	}
	var b *object.Tree
	if len(oc.Parents) != 0 {
		bc, err := w.odb.Commit(ctx, oc.Parents[0])
		if err != nil {
			die_error("zeta revert resolve base error: %v", err)
			return err
		}
		if b, err = bc.Root(ctx); err != nil {
			die_error("zeta revert resolve root tree error: %v", err)
			return err
		}
	} else {
		b = w.odb.EmptyTree()
	}
	result, err := w.odb.MergeTree(ctx, o, a, b, &odb.MergeOptions{
		Branch1:       currentName.BranchName(),
		Branch2:       opts.From,
		DetectRenames: true,
		Textconv:      false,
		MergeDriver:   w.resolveMergeDriver(),
		TextGetter:    w.readMissingText,
	})
	if err != nil {
		die_error("merge-tree: %v", err)
		return err
	}
	if len(result.Conflicts) != 0 {
		_ = w.checkoutReplayConflicts(ctx, &ReplayMD{
			BASE:       current.Hash(),
			LAST:       revertRev,
			MERGE_TREE: result.NewTree,
			HEAD:       current.Name(),
		}, result.Conflicts)
		// format error:
		fmt.Fprintln(os.Stderr, W("Revert failed; fix conflicts and then commit the result."))
		return ErrHasConflicts
	}
	committer := w.NewCommitter()
	cc := &object.Commit{
		Author:       *committer,
		Committer:    *committer,
		Parents:      []plumbing.Hash{ac.Hash},
		Tree:         result.NewTree,
		ExtraHeaders: oc.ExtraHeaders,
		Message:      "revert: " + oc.Subject(),
	}
	newRev, err := w.odb.WriteEncoded(cc)
	if err != nil {
		die_error("unable encode commit: %v", err)
		return err
	}
	messagePrefix := fmt.Sprintf("Revert '%s' on %s", revertRev, branchName)
	if err := w.DoUpdate(ctx, current.Name(), oc.Hash, newRev, committer, "revert: "+messagePrefix); err != nil {
		die_error("update rebase: %v", err)
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: newRev, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s..%s\n", W("Updating"), shortHash(oc.Hash), shortHash(newRev))
	fmt.Fprintf(os.Stderr, W("Successfully revert and updated %s.\n"), current.Name())
	return nil
}

func (w *Worktree) revertAbort(ctx context.Context) error {
	md, err := w.replayMD()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		die_error("zeta revert --abort: read 'REPLAY-MD': %v", err)
		return err
	}
	trace.DbgPrint("BASE: %s", md.BASE)
	HEAD := plumbing.NewSymbolicReference(plumbing.HEAD, md.HEAD)
	if err := w.Update(HEAD, nil); err != nil {
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: md.BASE, Mode: HardReset}); err != nil {
		die_error("zeta revert --abort: reset worktree error: %v", err)
		return err
	}
	_ = os.Remove(filepath.Join(w.odb.Root(), REPLAY_MD))
	return nil
}

func (w *Worktree) revertContinue(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	md, err := w.replayMD()
	if err != nil {
		if os.IsNotExist(err) {
			die_error("zeta revert --continue: metadata 'REPLAY-MD' not found")
			return err
		}
		die_error("zeta revert --continue: read 'REPLAY-MD': %v", err)
		return err
	}
	trace.DbgPrint("%s --> %s", md.LAST, md.BASE)
	pc, err := w.odb.Commit(ctx, md.BASE)
	if err != nil {
		die_error("zeta revert --continue: unable resolve HEAD: %v", err)
		return err
	}
	last, err := w.odb.Commit(ctx, md.LAST)
	if err != nil {
		die_error("unable open last tree: %v", err)
		return err
	}
	resolvedTree, err := w.writeIndexAsTree(ctx, last.Tree, false)
	if err != nil {
		die_error("unable write resolved tree: %v", err)
		return err
	}
	trace.DbgPrint("conflicts resolved: %s", resolvedTree)
	committer := w.NewCommitter()
	cc := &object.Commit{
		Author:       *committer,
		Committer:    *committer,
		Parents:      []plumbing.Hash{pc.Hash},
		Tree:         resolvedTree,
		ExtraHeaders: last.ExtraHeaders,
		Message:      "revert: " + last.Subject(),
	}
	lastCommitID, err := w.odb.WriteEncoded(cc)
	if err != nil {
		die_error("unable encode commit: %v", err)
		return err
	}
	messagePrefix := fmt.Sprintf("Revert '%s'", md.LAST)
	if err := w.DoUpdate(ctx, md.HEAD, pc.Hash, lastCommitID, committer, "revert: "+messagePrefix); err != nil {
		die_error("update revert: %v", err)
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: lastCommitID, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s..%s\n", W("Updating"), shortHash(pc.Hash), shortHash(lastCommitID))
	fmt.Fprintf(os.Stderr, W("Successfully cherry-pick and updated %s.\n"), md.HEAD)
	_ = os.Remove(filepath.Join(w.odb.Root(), REPLAY_MD))
	return nil
}

func (w *Worktree) checkoutReplayConflicts(ctx context.Context, md *ReplayMD, conflicts []*odb.Conflict) error {
	mPath := filepath.Join(w.odb.Root(), REPLAY_MD)
	if _, err := os.Stat(mPath); err == nil {
		die_error("unable hold rebase conflicts: REPLAY-MD exists")
		return errors.New("unable hold rebase conflicts: REPLAY-MD exists")
	}
	cc, err := w.odb.Commit(ctx, md.BASE)
	if err != nil {
		die_error("unable read base commit: %v", err)
		return err
	}
	baseTree, err := cc.Root(ctx)
	if err != nil {
		die_error("unable read base tree: %v", err)
		return err
	}
	newTree, err := w.odb.Tree(ctx, md.MERGE_TREE)
	if err != nil {
		die_error("unable open merge tree: %v", err)
		return err
	}
	fd, err := os.Create(mPath)
	if err != nil {
		die_error("unable create REBASE-MD: %v", err)
		return err
	}
	err = toml.NewEncoder(fd).Encode(md)
	_ = fd.Close()
	if err != nil {
		die_error("unable encode replay metadata: %v", err)
		return err
	}
	if err := w.checkoutConflicts(ctx, baseTree, newTree, conflicts); err != nil {
		die_error("unable checkout conflicts: %v", err)
		return err
	}
	HEAD := plumbing.NewHashReference(plumbing.HEAD, md.LAST)
	if err := w.Update(HEAD, nil); err != nil {
		die_error("unable set HEAD to last: %v", err)
		return err
	}
	return nil
}
