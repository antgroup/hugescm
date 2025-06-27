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
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

type RebaseOptions struct {
	Branch   string
	Upstream string
	Onto     string
	Abort    bool
	Continue bool
}

func (w *Worktree) Rebase(ctx context.Context, opts *RebaseOptions) error {
	if opts.Abort {
		return w.rebaseAbort(ctx)
	}
	if opts.Continue {
		return w.rebaseContinue(ctx)
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

	if opts.Branch != "HEAD" && opts.Branch != branchName {
		if err := w.SwitchBranch(ctx, opts.Branch, &SwitchOptions{Force: false}); err != nil {
			die_error("can not switch branch %s", err)
			return err
		}
	}

	var newRev plumbing.Hash
	var messagePrefix string
	if opts.Onto != "" {
		ontoRev, err := w.Revision(ctx, opts.Onto)
		if err != nil {
			die_error("unable resolve onto %v", err)
			return err
		}
		upsRev, err := w.Revision(ctx, opts.Upstream)
		if err != nil {
			die_error("unable resolve upstream %v", err)
			return err
		}
		messagePrefix = fmt.Sprintf("Rebase branch '%s' with upstream %s onto %s", branchName, opts.Branch, ontoRev)
		newRev, err = w.rebaseWithUpstream(ctx, current.Hash(), upsRev, ontoRev, currentName, plumbing.ReferenceName(opts.Onto), false)
		if err != nil {
			die_error("rebase: %v", err)
			return err
		}
	} else {
		ontoRev, err := w.Revision(ctx, opts.Upstream)
		if err != nil {
			die_error("unable resolve onto %v", err)
			return err
		}
		messagePrefix = fmt.Sprintf("Rebase branch '%s' onto %s", branchName, ontoRev)
		newRev, err = w.rebaseInternal(ctx, current.Hash(), ontoRev, currentName, plumbing.ReferenceName(opts.Onto), false)
		if err != nil {
			return err
		}
	}

	if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "rebase: "+messagePrefix); err != nil {
		die_error("update rebase: %v", err)
		return err
	}

	if err := w.Reset(ctx, &ResetOptions{Commit: newRev, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s..%s\n", W("Updating"), shortHash(current.Hash()), shortHash(newRev))
	fmt.Fprintf(os.Stderr, W("Successfully rebased and updated %s.\n"), currentName)
	return nil
}

// rebaseInternal:
//
//	A-->B-->C-->D-->E (our)
//	A-->B-->G-->H-->K (onto)
//
// Rebase:
//
//	 A-->B-->G-->H-->K-->C-->D-->E (our rebased)
//
//	merge K & C  merge-base; B, parent K;    A-->B-->G-->H-->K-->C(n)
//	merge K & D  merge-base: B, parent C(n); A-->B-->G-->H-->K-->C(n)-->D(n)
//	merge K & E  merge-base: B, parent D(n); A-->B-->G-->H-->K-->C(n)-->D(n)-->E(n)
func (w *Worktree) rebaseInternal(ctx context.Context, our, onto plumbing.Hash, ourBranch, ontoBranch plumbing.ReferenceName, textconv bool) (plumbing.Hash, error) {
	oursCommit, err := w.odb.Commit(ctx, our)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	ontoCommit, err := w.odb.Commit(ctx, onto)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	bases, err := oursCommit.MergeBase(ctx, ontoCommit)
	if err != nil {
		die_error("rebase %s onto %s: %v", our, onto, err)
		return plumbing.ZeroHash, err
	}
	if len(bases) == 0 {
		fmt.Fprintf(os.Stderr, "rebase: %s\n", W("refusing to merge unrelated histories"))
		return plumbing.ZeroHash, ErrUnrelatedHistories
	}
	ignore := make([]plumbing.Hash, 0, 2)
	for _, c := range bases {
		ignore = append(ignore, c.Hash)
	}
	baseTree, err := bases[0].Root(ctx)
	if err != nil {
		die_error("unable resolve root tree: %v", err)
		return plumbing.ZeroHash, err
	}
	ontoTree, err := ontoCommit.Root(ctx)
	if err != nil {
		die_error("resolve onto tree: %v", err)
		return plumbing.ZeroHash, err
	}
	// TODO: rebase: merge commits should be avoided as much as possible
	commits, err := w.revList(ctx, our, ignore, LogOrderTopo, nil)
	if err != nil {
		die_error("log range base error: %v", err)
		return plumbing.ZeroHash, err
	}
	lastCommitID := onto
	mergeDriver := w.resolveMergeDriver()
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		if len(c.Parents) == 2 {
			// skip merge commit
			continue
		}
		t, err := c.Root(ctx)
		if err != nil {
			die_error("resolve %s tree: %v", c.Hash, err)
			return plumbing.ZeroHash, err
		}
		result, err := w.odb.MergeTree(ctx, baseTree, t, ontoTree, &odb.MergeOptions{
			Branch1:       ourBranch.BranchName(),
			Branch2:       ontoBranch.Short(),
			DetectRenames: true,
			Textconv:      textconv,
			MergeDriver:   mergeDriver,
			TextGetter:    w.readMissingText,
		})
		if err != nil {
			die_error("merge-tree: %v", err)
			return plumbing.ZeroHash, err
		}
		if len(result.Conflicts) != 0 {
			err = w.checkoutRebaseConflicts(ctx, &RebaseMD{
				REBASE_HEAD: our,
				ONTO:        onto,
				STOPPED:     c.Hash,
				LAST:        lastCommitID,
				MERGE_TREE:  result.NewTree,
				HEAD:        ourBranch,
			}, result.Conflicts)
			if err != nil {
				die_error("unable checkout conflicts: %v", err)
			}
			return plumbing.ZeroHash, ErrHasConflicts
		}
		cc := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Parents:      []plumbing.Hash{lastCommitID},
			Tree:         result.NewTree,
			ExtraHeaders: c.ExtraHeaders,
			Message:      c.Message,
		}
		newRev, err := w.odb.WriteEncoded(cc)
		if err != nil {
			die_error("unable encode commit: %v", err)
			return plumbing.ZeroHash, err
		}
		lastCommitID = newRev
	}
	return lastCommitID, nil
}

/*
First let’s assume your topic is based on branch next. For example, a feature developed
in topic depends on some functionality which is found in next.

	o---o---o---o---o  master
		\
			o---o---o---o---o  next
							\
							o---o---o  topic

We want to make topic forked from branch master; for example, because the functionality
on which topic depends was merged into the more stable master branch. We want our tree to
look like this:

	o---o---o---o---o  master
		|            \
		|             o'--o'--o'  topic
		\
			o---o---o---o---o  next

We can get this using the following command:

	git rebase --onto master next topic

Another example of --onto option is to rebase part of a branch. If we have the following
situation:

							H---I---J topicB
							/
					E---F---G  topicA
				/
	A---B---C---D  master

then the command

	git rebase --onto master topicA topicB

would result in:

				H'--I'--J'  topicB
				/
				| E---F---G  topicA
				|/
	A---B---C---D  master

This is useful when topicB does not depend on topicA.

A range of commits could also be removed with rebase. If we have the following situation:

	E---F---G---H---I---J  topicA

then the command

	git rebase --onto topicA~5 topicA~3 topicA

would result in the removal of commits F and G:

	E---H'---I'---J'  topicA

This is useful if F and G were flawed in some way, or should not be part of topicA. Note
that the argument to --onto and the <upstream> parameter can be any valid commit-ish.
*/
func (w *Worktree) rebaseWithUpstream(ctx context.Context, our, upstream, onto plumbing.Hash, ourBranch, ontoBranch plumbing.ReferenceName, textconv bool) (plumbing.Hash, error) {
	oursCommit, err := w.ODB().Commit(ctx, our)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	upsCommit, err := w.ODB().Commit(ctx, upstream)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	ontoCommit, err := w.ODB().Commit(ctx, onto)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	ourBases, err := oursCommit.MergeBase(ctx, upsCommit)
	if err != nil {
		die_error("calc base of upstream with branch error: %v", err)
		return plumbing.ZeroHash, err
	}
	if len(ourBases) == 0 {
		fmt.Fprintln(os.Stderr, "rebase: refusing to use unrelated histories upstream")
		return plumbing.ZeroHash, err
	}
	ontoBases, err := ontoCommit.MergeBase(ctx, ourBases[0])
	if err != nil {
		die_error("calc base of branch with onto error: %v", err)
		return plumbing.ZeroHash, err
	}
	if len(ontoBases) == 0 {
		fmt.Fprintln(os.Stderr, "rebase: refusing to use unrelated histories onto")
		return plumbing.ZeroHash, err
	}

	ontoBaseTree, err := ontoBases[0].Root(ctx)
	if err != nil {
		die_error("unable resolve root tree: %v", err)
		return plumbing.ZeroHash, err

	}
	ontoTree, err := ontoCommit.Root(ctx)
	if err != nil {
		die_error("resolve onto tree: %v", err)
		return plumbing.ZeroHash, err
	}

	ignore := make([]plumbing.Hash, 0, 2)
	for _, c := range ourBases {
		ignore = append(ignore, c.Hash)
	}
	// TODO: rebase: merge commits should be avoided as much as possible
	commits, err := w.revList(ctx, our, ignore, LogOrderTopo, nil)
	if err != nil {
		die_error("log range base error: %v", err)
		return plumbing.ZeroHash, err
	}
	lastCommitID := onto
	mergeDriver := w.resolveMergeDriver()
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		if len(c.Parents) == 2 {
			// skip merge commit
			continue
		}
		t, err := c.Root(ctx)
		if err != nil {
			die_error("resolve %s tree: %v", c.Hash, err)
			return plumbing.ZeroHash, err

		}
		result, err := w.ODB().MergeTree(ctx, ontoBaseTree, t, ontoTree, &odb.MergeOptions{
			Branch1:       ourBranch.BranchName(),
			Branch2:       ontoBranch.Short(),
			DetectRenames: true,
			Textconv:      textconv,
			MergeDriver:   mergeDriver,
			TextGetter:    w.readMissingText,
		})
		if err != nil {
			die_error("merge-tree: %v", err)
			return plumbing.ZeroHash, err
		}
		if len(result.Conflicts) != 0 {
			err = w.checkoutRebaseConflicts(ctx, &RebaseMD{
				REBASE_HEAD: our,
				ONTO:        onto,
				STOPPED:     c.Hash,
				LAST:        lastCommitID,
				MERGE_TREE:  result.NewTree,
				HEAD:        ourBranch,
			}, result.Conflicts)
			if err != nil {
				die_error("unable checkout conflicts: %v", err)
			}
			return plumbing.ZeroHash, ErrHasConflicts
		}
		cc := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Parents:      []plumbing.Hash{lastCommitID},
			Tree:         result.NewTree,
			ExtraHeaders: c.ExtraHeaders,
			Message:      c.Message,
		}
		newRev, err := w.ODB().WriteEncoded(cc)
		if err != nil {
			die_error("unable encode commit: %v", err)
			return plumbing.ZeroHash, err
		}
		lastCommitID = newRev
	}
	return lastCommitID, nil
}

type RebaseMD struct {
	REBASE_HEAD plumbing.Hash          `toml:"REBASE_HEAD"` // REBASE_HEAD
	ONTO        plumbing.Hash          `toml:"ONTO"`        // ONTO Hash
	STOPPED     plumbing.Hash          `toml:"STOPPED"`     // STOPPED Hash
	LAST        plumbing.Hash          `toml:"LAST"`        // LAST
	MERGE_TREE  plumbing.Hash          `toml:"MERGE_TREE"`  // MERGE_TREE
	HEAD        plumbing.ReferenceName `toml:"HEAD"`        // HEAD aka CURRENT
}

const (
	REBASE_MD = "REBASE-MD"
)

func (w *Worktree) rebaseMD() (*RebaseMD, error) {
	var md RebaseMD
	_, err := toml.DecodeFile(filepath.Join(w.odb.Root(), REBASE_MD), &md)
	if err != nil {
		return nil, err
	}
	return &md, nil
}

func (w *Worktree) checkoutRebaseConflicts(ctx context.Context, md *RebaseMD, conflicts []*odb.Conflict) error {
	mPath := filepath.Join(w.odb.Root(), REBASE_MD)
	if _, err := os.Stat(mPath); err == nil {
		die_error("unable hold rebase conflicts: REBASE-MD exists")
		return errors.New("unable hold rebase conflicts: REBASE-MD exists")
	}
	cc, err := w.odb.Commit(ctx, md.LAST)
	if err != nil {
		die_error("unable read last commit: %v", err)
		return err
	}
	lastTree, err := cc.Root(ctx)
	if err != nil {
		die_error("unable read last tree: %v", err)
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
		die_error("unable encode rebase metadata: %v", err)
		return err
	}
	if err := w.checkoutConflicts(ctx, lastTree, newTree, conflicts); err != nil {
		die_error("unable checkout conflicts: %v", err)
		return err
	}
	HEAD := plumbing.NewHashReference(plumbing.HEAD, md.LAST)
	if err := w.ReferenceUpdate(HEAD, nil); err != nil {
		die_error("unable set HEAD to last: %v", err)
		return err
	}
	return nil
}

func (w *Worktree) rebaseAbort(ctx context.Context) error {
	md, err := w.rebaseMD()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		die_error("zeta rebase --continue: read 'REBASE-MD': %v", err)
		return err
	}
	w.DbgPrint("REBASE_HEAD: %s", md.REBASE_HEAD)
	HEAD := plumbing.NewSymbolicReference(plumbing.HEAD, md.HEAD)
	if err := w.ReferenceUpdate(HEAD, nil); err != nil {
		return err
	}
	if err := w.Reset(ctx, &ResetOptions{Commit: md.REBASE_HEAD, Mode: HardReset}); err != nil {
		die_error("zeta rebase --abort: reset worktree error: %v", err)
		return err
	}
	_ = os.Remove(filepath.Join(w.odb.Root(), REBASE_MD))
	return nil
}

func (w *Worktree) rebaseContinue(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	md, err := w.rebaseMD()
	if err != nil {
		if os.IsNotExist(err) {
			die_error("zeta rebase --continue: metadata 'REBASE-MD' not found")
			return err
		}
		die_error("zeta rebase --continue: read 'REBASE-MD': %v", err)
		return err
	}
	w.DbgPrint("%s", md.REBASE_HEAD)
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
	newBaseTree, err := w.odb.Tree(ctx, resolvedTree)
	if err != nil {
		die_error("unable write resolved tree: %v", err)
		return err
	}
	w.DbgPrint("conflicts resolved: %s", resolvedTree)
	stoppedCC, err := w.odb.Commit(ctx, md.STOPPED)
	if err != nil {
		die_error("unable resolve stopped commit: %v", err)
		return err
	}
	cc := &object.Commit{
		Author:       stoppedCC.Author,
		Committer:    stoppedCC.Committer,
		Parents:      []plumbing.Hash{md.LAST},
		Tree:         resolvedTree,
		ExtraHeaders: stoppedCC.ExtraHeaders,
		Message:      stoppedCC.Message,
	}
	lastCommitID, err := w.odb.WriteEncoded(cc)
	if err != nil {
		die_error("unable encode commit: %v", err)
		return err
	}
	ontoTree, err := w.getTreeFromCommitHash(ctx, md.ONTO)
	if err != nil {
		die_error("unable resolve onto: %v", err)
		return err
	}
	mergeDriver := w.resolveMergeDriver()
	// TODO: rebase: merge commits should be avoided as much as possible
	commits, err := w.revList(ctx, md.REBASE_HEAD, []plumbing.Hash{md.STOPPED}, LogOrderTopo, nil)
	if err != nil {
		die_error("log range base error: %v", err)
		return err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		if len(c.Parents) == 2 {
			// skip merge commit
			continue
		}
		t, err := c.Root(ctx)
		if err != nil {
			die_error("resolve %s tree: %v", c.Hash, err)
			return err
		}
		result, err := w.odb.MergeTree(ctx, newBaseTree, t, ontoTree, &odb.MergeOptions{
			Branch1:       "rebase-HEAD",
			Branch2:       "rebase-ONTO",
			DetectRenames: true,
			Textconv:      false,
			MergeDriver:   mergeDriver,
			TextGetter:    w.readMissingText,
		})
		if err != nil {
			die_error("merge-tree: %v", err)
			return err
		}
		if len(result.Conflicts) != 0 {
			err = w.checkoutRebaseConflicts(ctx, &RebaseMD{
				REBASE_HEAD: md.REBASE_HEAD,
				ONTO:        md.ONTO,
				STOPPED:     c.Hash,
				LAST:        lastCommitID,
				MERGE_TREE:  result.NewTree,
				HEAD:        md.HEAD,
			}, result.Conflicts)
			if err != nil {
				die_error("unable checkout conflicts: %v", err)
			}
			return ErrHasConflicts
		}
		cc := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Parents:      []plumbing.Hash{lastCommitID},
			Tree:         result.NewTree,
			ExtraHeaders: c.ExtraHeaders,
			Message:      c.Message,
		}
		newRev, err := w.odb.WriteEncoded(cc)
		if err != nil {
			die_error("unable encode commit: %v", err)
			return err
		}
		lastCommitID = newRev
	}
	branchName := md.HEAD.BranchName()
	messagePrefix := fmt.Sprintf("Rebase branch '%s' onto %s", branchName, md.ONTO)
	if err := w.DoUpdate(ctx, md.HEAD, md.REBASE_HEAD, lastCommitID, w.NewCommitter(), "rebase: "+messagePrefix); err != nil {
		die_error("update rebase: %v", err)
		return err
	}
	//Reset HEAD
	w.DbgPrint("REBASE_HEAD: %s", md.REBASE_HEAD)
	HEAD := plumbing.NewSymbolicReference(plumbing.HEAD, md.HEAD)
	if err := w.ReferenceUpdate(HEAD, nil); err != nil {
		return err
	}

	if err := w.Reset(ctx, &ResetOptions{Commit: lastCommitID, Mode: MergeReset}); err != nil {
		die_error("reset worktree: %v", err)
		return err
	}
	_ = os.Remove(filepath.Join(w.ODB().Root(), REBASE_MD))
	fmt.Fprintf(os.Stderr, "%s %s..%s\n", W("Updating"), shortHash(md.REBASE_HEAD), shortHash(lastCommitID))
	fmt.Fprintf(os.Stderr, W("Successfully rebased and updated %s.\n"), md.HEAD)
	return nil
}
