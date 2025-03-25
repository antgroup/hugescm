// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type PullOptions struct {
	FF, FFOnly, Rebase, Squash, Unshallow, One bool
	Limit                                      int64
}

func (w *Worktree) Pull(ctx context.Context, opts *PullOptions) error {
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
	fo, err := w.DoFetch(ctx, &DoFetchOptions{Name: currentName.String(), Unshallow: opts.Unshallow, Limit: opts.Limit, FetchAlways: true, SkipLarges: opts.One})
	if err != nil {
		return err
	}
	if fo.FETCH_HEAD == current.Hash() {
		fmt.Fprintln(os.Stderr, W("Already up to date."))
		return nil
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
	headAheadOfRef, err := w.isFastForward(ctx, fo.FETCH_HEAD, current.Hash(), ignoreParents)
	if err != nil {
		die_error("check fast-forward error: %v", err)
		return err
	}
	// already up to date
	if headAheadOfRef {
		fmt.Fprintln(os.Stderr, W("Already up to date."))
		return nil
	}

	fastForward, err := w.isFastForward(ctx, current.Hash(), fo.FETCH_HEAD, ignoreParents)
	if err != nil {
		die_error("check fast-forward error: %v", err)
		return err
	}
	branchName := currentName.BranchName()
	if fastForward {
		newRev := fo.FETCH_HEAD
		if !opts.FF {
			messagePrefix := fmt.Sprintf("Merge branch '%s of %s' into %s", branchName, w.cleanedRemote(), branchName)
			message, err := w.mergeMessageFromPrompt(ctx, messagePrefix)
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
		if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "pull: Fast-forward"); err != nil {
			die_error("update fast forward: %v", err)
			return err
		}
		fmt.Fprintf(os.Stderr, "%s %s..%s\nFast-forward\n", W("Updating"), shortHash(current.Hash()), shortHash(newRev))
		if err := w.Reset(ctx, &ResetOptions{Commit: newRev, Mode: MergeReset}); err != nil {
			die_error("reset worktree: %v", err)
			return err
		}
		_ = w.mergeStat(ctx, current.Hash(), newRev)
		return nil
	}
	if opts.FFOnly {
		fmt.Fprintln(os.Stderr, W("Not possible to fast-forward, aborting."))
		return ErrNonFastForwardUpdate
	}
	remoteRefName := plumbing.NewRemoteReferenceName("origin", branchName)
	if opts.Rebase {
		messagePrefix := fmt.Sprintf("Rebase branch '%s of %s' into %s", branchName, w.cleanedRemote(), branchName)
		newRev, err := w.rebaseInternal(ctx, current.Hash(), fo.FETCH_HEAD, currentName, remoteRefName, false)
		if err != nil {
			return err
		}
		if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "pull: "+messagePrefix); err != nil {
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
	messagePrefix := fmt.Sprintf("Merge branch '%s of %s' into %s", branchName, w.cleanedRemote(), branchName)
	newRev, err := w.mergeInternal(ctx, current.Hash(), fo.FETCH_HEAD, branchName, string(remoteRefName), opts.Squash, false, false, false, func() string {
		message, _ := w.mergeMessageFromPrompt(ctx, messagePrefix)
		return message
	})
	if err != nil {
		return err
	}
	if err := w.DoUpdate(ctx, current.Name(), current.Hash(), newRev, w.NewCommitter(), "pull: "+messagePrefix); err != nil {
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
