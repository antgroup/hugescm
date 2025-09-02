// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// TODO:
// Switch to a specified branch. The working tree and the index are updated to match the branch. All new commits will be added to the tip of this branch.
// Optionally a new branch could be created with either , , automatically from a remote branch of same name (see ),
// or detach the working tree from any branch with , along with switching.-c-C--guess--detach
// Switching branches does not require a clean index and working tree (i.e. no differences compared to ).
// The operation is aborted however if the operation leads to loss of local changes, unless told otherwise with or .HEAD--discard-changes--merge

type SwitchOptions struct {
	Force       bool // aka discardChanges
	Merge       bool
	ForceCreate bool
	Remote      bool
	Limit       int64
	firstSwitch bool
	one         bool
}

func (so *SwitchOptions) Validate() error {
	if so.Force && so.Merge {
		return errors.New("force and merge cannot be used together")
	}
	return nil
}

func switchError(target string, err error) {
	if err == ErrAborting {
		return
	}
	die("switch to '%s' error: %v", target, err)
}

func (r *Repository) switchBranchFromRemote(ctx context.Context, branch string, so *SwitchOptions) error {
	fo, err := r.DoFetch(ctx, &DoFetchOptions{Name: branch, FetchAlways: true, Limit: so.Limit})
	if err != nil {
		return err
	}
	opts := &CheckoutOptions{Merge: so.Merge, Force: so.Force, First: false, One: so.one}
	if fo.Reference != nil && fo.Name.IsBranch() {
		if err := r.CreateBranch(ctx, branch, fo.FETCH_HEAD.String(), so.ForceCreate, true); err != nil {
			return err
		}
		opts.Branch = plumbing.NewBranchReferenceName(branch)
	} else {
		opts.Hash = fo.FETCH_HEAD
	}
	w := r.Worktree()
	w.missingNotFailure = true
	if err := w.Checkout(ctx, opts); err != nil {
		switchError(branch, err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s '%s' %s 'origin/%s'", W("branch"), W("set up to track"), branch, branch)
	fmt.Fprintf(os.Stderr, "%s '%s'\n", W("Switched to branch"), branch)
	return nil
}

func (r *Repository) SwitchBranch(ctx context.Context, branch string, so *SwitchOptions) error {
	refname := plumbing.NewBranchReferenceName(branch)
	ref, err := r.Reference(refname)
	if err == plumbing.ErrReferenceNotFound {
		if !so.Remote {
			die("couldn't find branch '%s', add '--remote' download and switch to this branch", refname)
			return err
		}
		trace.DbgPrint("switch branch from remote: %v", branch)
		return r.switchBranchFromRemote(ctx, branch, so)
	}
	if err != nil {
		die_error("find branch '%s': %v", branch, err)
		return err
	}
	if ref.Type() != plumbing.HashReference {
		die("reference %s not branch", branch)
		return err
	}
	trace.DbgPrint("switch branch from local: %v", branch)
	w := r.Worktree()
	if err := w.Checkout(ctx, &CheckoutOptions{Branch: refname, Merge: so.Merge, Force: so.Force, First: so.firstSwitch, One: so.one}); err != nil {
		switchError(branch, err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s '%s'\n", W("Switched to branch"), branch)
	return nil
}

func (r *Repository) SwitchDetach(ctx context.Context, basePoint string, so *SwitchOptions) error {
	oid, err := r.promiseFetch(ctx, basePoint, true)
	if err != nil {
		die_error("resolve %s: %v", basePoint, err)
		return err
	}
	w := r.Worktree()
	if err := w.Checkout(ctx, &CheckoutOptions{Hash: oid, Merge: so.Merge, Force: so.Force, First: so.firstSwitch, One: so.one}); err != nil {
		switchError(basePoint, err)
		return err
	}
	cc, err := w.parseRevExhaustive(ctx, basePoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve HEAD commit error: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "HEAD %s %s %s\n", W("is now at"), shortHash(cc.Hash), cc.Subject())
	return nil
}

func (r *Repository) SwitchOrphan(ctx context.Context, newBranch string, so *SwitchOptions) error {
	refname := plumbing.NewBranchReferenceName(newBranch)
	ref, err := r.ReferencePrefixMatch(refname)
	if err != nil && err != plumbing.ErrReferenceNotFound {
		die_error("zeta switch: from %s error: %v", newBranch, err)
		return err
	}
	if ref != nil {
		die("a branch named '%s' already exists", newBranch)
		return errors.New("branch already exists")
	}
	cc, err := r.parseRevExhaustive(ctx, "HEAD")
	if err != nil {
		die_error("zeta switch: parse HEAD: %v", err)
		return err
	}
	orphanCommit := &object.Commit{
		Author:       cc.Author,
		Committer:    cc.Committer,
		Tree:         cc.Tree,
		ExtraHeaders: cc.ExtraHeaders,
		Message:      cc.Message,
	}
	newOID, err := r.odb.WriteEncoded(orphanCommit)
	if err != nil {
		die("zeta switch: encode new commit: %v", err)
		return err
	}
	if err := r.DoUpdate(ctx, refname, plumbing.ZeroHash, newOID, r.NewCommitter(), "branch: Create orphan from"); err != nil {
		die_error("update-ref '%s': %v", refname, err)
		return err
	}
	w := r.Worktree()
	if err := w.Checkout(ctx, &CheckoutOptions{Branch: refname, Merge: so.Merge, Force: so.Force, First: so.firstSwitch, One: so.one}); err != nil {
		switchError(newBranch, err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s '%s'\n", W("Switched to a new branch"), newOID.String())
	return nil
}

func (r *Repository) SwitchNewBranch(ctx context.Context, newBranch string, basePoint string, so *SwitchOptions) error {
	if err := r.CreateBranch(ctx, newBranch, basePoint, so.ForceCreate, true); err != nil {
		return err
	}
	w := r.Worktree()
	if err := w.Checkout(ctx, &CheckoutOptions{Branch: plumbing.NewBranchReferenceName(newBranch), Merge: so.Merge, Force: so.Force, First: so.firstSwitch, One: so.one}); err != nil {
		switchError(newBranch, err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s '%s'\n", W("Switched to a new branch"), newBranch)
	return nil
}
