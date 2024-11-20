// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/refs"
)

var (
	ErrNotAllowedRemoveCurrent = errors.New("not allowed remove HEAD")
)

func (r *Repository) ShowCurrent(w io.Writer) error {
	current, err := r.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve HEAD error: %v\n", err)
		return err
	}
	if current.Name() == plumbing.HEAD {
		// detach checkout
		return nil
	}
	fmt.Fprintf(w, "%s\n", current.Name().Short())
	return nil
}

func (r *Repository) MoveBranch(from, to string, force bool) error {
	if !plumbing.ValidateBranchName([]byte(to)) {
		die("'%s' is not a valid branch name", to)
		return &plumbing.ErrBadReferenceName{Name: to}
	}
	head, err := r.HEAD()
	if err != nil {
		die("current branch not found: %v", err)
		return err
	}
	if head == nil {
		die_error("current reference not found")
		return plumbing.ErrReferenceNotFound
	}
	fromRef, err := r.Reference(plumbing.NewBranchReferenceName(from))
	if err == plumbing.ErrReferenceNotFound {
		die_error("'%s' not found.", from)
		return err
	}
	if err != nil {
		die_error("resolve branch '%s': %v", from, err)
		return err
	}
	if err := r.ReferenceRemove(fromRef); err != nil {
		die_error("update target error: %v", err)
		return err
	}
	restoreBranch := func() {
		_ = r.ReferenceUpdate(fromRef, nil)
	}
	target := plumbing.NewBranchReferenceName(to)
	var toRef *plumbing.Reference
	if toRef, err = r.ReferencePrefixMatch(target); err != nil && err != plumbing.ErrReferenceNotFound {
		restoreBranch()
		die_error("resolve branch '%s' error: %v", target, err)
		return err
	}
	if toRef != nil {
		if toRef.Name() != target {
			restoreBranch()
			die("'%s' exists; cannot create '%s'", toRef.Name(), target)
			return errors.New("move branch denied")
		}
		if !force {
			restoreBranch()
			die_error("'%s' already exists, hash: %s", to, toRef.Hash())
			return errors.New("move branch denied")
		}
	}
	newRef := plumbing.NewHashReference(target, fromRef.Hash())
	if err := r.ReferenceUpdate(newRef, toRef); err != nil {
		restoreBranch()
		die_error("update target error: %v", err)
		return err
	}

	if head.Target() == fromRef.Name() {
		if err := r.ReferenceUpdate(plumbing.NewSymbolicReference(plumbing.HEAD, target), nil); err != nil {
			die_error("update HEAD error: %v", err)
			return err
		}
	}
	fmt.Fprintf(os.Stderr, W("Branch '%s' has been moved to '%s'\n"), from, to)
	return nil
}

func (r *Repository) RemoveBranch(branches []string, force bool) error {
	head, err := r.HEAD()
	if err != nil {
		die("current branch not found: %v", err)
		return err
	}
	for _, b := range branches {
		ref, err := r.Reference(plumbing.NewBranchReferenceName(b))
		if err == plumbing.ErrReferenceNotFound {
			die_error("branch '%s' not found", b)
			return err
		}
		if err != nil {
			die_error("resolve branch '%s': %v", b, err)
			return err
		}
		if head.Target() == ref.Name() {
			die_error("cannot delete branch '%s' used by worktree at '%s'", b, r.baseDir)
			return ErrNotAllowedRemoveCurrent
		}
		if err := r.ReferenceRemove(ref); err != nil {
			die_error("remove branch error: %v", err)
			return err
		}
		fmt.Fprintf(os.Stderr, W("Deleted branch %s (was %s).\n"), b, shortHash(ref.Hash()))
	}
	return nil
}

func (r *Repository) ListBranch(ctx context.Context, pattern []string) error {
	db, err := refs.ReferencesDB(r.zetaDir)
	if err != nil {
		die_error("open references db error: %v", err)
		return err
	}
	m := NewMatcher(pattern)
	w := NewPrinter(ctx)
	defer w.Close()
	target := db.HEAD().Target()
	for _, r := range db.References() {
		if !r.Name().IsBranch() {
			continue
		}
		branchName := r.Name().BranchName()
		if !m.Match(branchName) {
			continue
		}
		if target == r.Name() {
			if isTrueColorTerminal {
				fmt.Fprintf(w, "\x1b[38;2;67;233;123m* %s\x1b[0m\n", branchName)
				continue
			}
			fmt.Fprintf(w, " %s\n", branchName)
			continue
		}
		fmt.Fprintf(w, "  %s\n", branchName)
	}
	return nil
}

func (r *Repository) CreateBranch(ctx context.Context, newBranch, from string, force bool, fetchMissing bool) error {
	if !plumbing.ValidateBranchName([]byte(newBranch)) {
		die("'%s' is not a valid branch name", newBranch)
		return &plumbing.ErrBadReferenceName{Name: newBranch}
	}
	newRefName := plumbing.NewBranchReferenceName(newBranch)
	if ref, err := r.ReferencePrefixMatch(newRefName); err == nil {
		if ref.Name() != newRefName {
			die("'%s' exists; cannot create '%s'", ref.Name(), newRefName)
			return errors.New("cannot create ref")
		}
		if !force {
			die_error("branch '%s' exists, commit: %s", newBranch, ref.Hash())
			return errors.New("cannot create ref")
		}
	}
	target, err := r.promiseFetch(ctx, from, fetchMissing)
	if err != nil {
		die_error("resolve '%s': %v", from, err)
		return err
	}
	cc, err := r.odb.Commit(ctx, target)
	if err != nil {
		die_error("open commit: %v", err)
		return err
	}
	if err := r.DoUpdate(ctx, newRefName, plumbing.ZeroHash, cc.Hash, r.NewCommitter(), "branch: Created from "+from); err != nil {
		die_error("update-ref: %v", err)
		return err
	}
	return nil
}
