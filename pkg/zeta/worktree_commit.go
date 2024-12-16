// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/mattn/go-isatty"
)

var (
	// ErrNoChanges occurs when a commit is attempted using a clean
	// working tree, with no changes to be committed.
	ErrNoChanges            = errors.New("clean working tree")
	ErrNotAllowEmptyMessage = errors.New("not allow empty message")
	ErrNothingToCommit      = errors.New("nothing to commit")
)

func (w *Worktree) genAmendMessageTemplate(ctx context.Context, p string) error {
	current, err := w.Current()
	if err != nil {
		return err
	}
	cc, err := w.odb.Commit(ctx, current.Hash())
	if err != nil {
		fmt.Fprintf(os.Stderr, "open head commit error: %v\n", err)
		return err
	}
	root, err := cc.Root(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open head tree error: %v\n", err)
		return err
	}
	var parentRoot *object.Tree
	if len(cc.Parents) != 0 {
		if pc, err := w.odb.Commit(ctx, cc.Parents[0]); err == nil {
			if parentRoot, err = pc.Root(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "open parent tree error: %v\n", err)
				return err
			}
		}
	}
	changes, err := root.DiffContext(ctx, parentRoot, noder.NewSparseTreeMatcher(w.Core.SparseDirs))
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff changes tree error: %v\n", err)
		return err
	}

	var b bytes.Buffer
	lines := strings.Split(cc.Message, "\n")
	for _, line := range lines {
		fmt.Fprintf(&b, "%s\n", line)
	}
	prefix := tr.Sprintf("Please enter the commit message for your changes. Lines starting\nwith '%c' will be ignored, and an empty message aborts the commit.", '#')
	for _, s := range strings.Split(prefix, "\n") {
		fmt.Fprintf(&b, "# %s\n", s)
	}
	fmt.Fprintf(&b, "#\n# %s %s\n# %s\n", W("On branch"), current.Name().BranchName(), W("Changes to be committed:"))

	for _, ch := range changes {
		a, err := ch.Action()
		if err != nil {
			fmt.Fprintf(os.Stderr, "change action error: %s\n", err)
			return err
		}
		switch a {
		case merkletrie.Delete:
			fmt.Fprintf(&b, "#    %s\t%s\n", W("deleted:"), ch.From.Name)
		case merkletrie.Insert:
			fmt.Fprintf(&b, "#    %s\t%s\n", W("new file:"), ch.To.Name)
		case merkletrie.Modify:
			fmt.Fprintf(&b, "#    %s\t%s\n", W("modified:"), ch.To.Name)
		default:
		}
	}
	fmt.Fprintf(&b, "#\n")
	return os.WriteFile(p, b.Bytes(), 0644)
}

func (w *Worktree) genMessageTemplate(ctx context.Context, opts *CommitOptions, branchName, p string, status Status) error {
	if opts.Amend {
		return w.genAmendMessageTemplate(ctx, p)
	}
	var b bytes.Buffer
	b.WriteByte('\n')
	prefix := tr.Sprintf("Please enter the commit message for your changes. Lines starting\nwith '%c' will be ignored, and an empty message aborts the commit.", '#')
	for _, s := range strings.Split(prefix, "\n") {
		fmt.Fprintf(&b, "# %s\n", s)
	}
	fmt.Fprintf(&b, "#\n# %s %s\n# %s\n", W("On branch"), branchName, W("Changes to be committed:"))
	for p, s := range status {
		if s.Worktree == Untracked {
			continue
		}
		if s.Staging != Unmodified {
			fmt.Fprintf(&b, "#    %s\t%s\n", W(StatusName(s.Staging)), p)
			continue
		}
		if !opts.All {
			continue
		}
		if s.Worktree != Unmodified {
			fmt.Fprintf(&b, "#    %s\t%s\n", W(StatusName(s.Worktree)), p)
		}
	}
	fmt.Fprintf(&b, "#\n")
	return os.WriteFile(p, b.Bytes(), 0644)
}

func (w *Worktree) messageFromPrompt(ctx context.Context, opts *CommitOptions, branchName string, status Status) (string, error) {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) && !env.ZETA_TERMINAL_PROMPT.SimpleAtob(true) {
		return "", nil
	}
	p := filepath.Join(w.odb.Root(), COMMIT_EDITMSG)
	if err := w.genMessageTemplate(ctx, opts, branchName, p, status); err != nil {
		return "", err
	}
	if err := launchEditor(ctx, w.coreEditor(), p, nil); err != nil {
		return "", nil
	}
	return messageReadFromPath(p)
}

func messageSubject(message string) string {
	if i := strings.IndexAny(message, "\r\n"); i != -1 {
		return message[0:i]
	}
	return message
}

func (w *Worktree) current() (plumbing.ReferenceName, plumbing.Hash, error) {
	ref, err := w.HEAD()
	if err != nil {
		return "", plumbing.ZeroHash, err
	}
	if ref == nil {
		return "", plumbing.ZeroHash, plumbing.ErrReferenceNotFound
	}
	if ref.Type() == plumbing.HashReference {
		return ref.Name(), ref.Hash(), nil
	}
	t, err := w.Reference(ref.Target())
	switch err {
	case nil:
		return t.Name(), t.Hash(), nil
	case plumbing.ErrReferenceNotFound:
		return ref.Target(), plumbing.ZeroHash, nil
	}
	return "", plumbing.ZeroHash, err
}

// Commit stores the current contents of the index in a new commit along with
// a log message from the user describing the changes.
func (w *Worktree) Commit(ctx context.Context, opts *CommitOptions) (plumbing.Hash, error) {
	if err := opts.Validate(w.Repository); err != nil {
		return plumbing.ZeroHash, err
	}
	current, oldRev, err := w.current()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	var status Status
	if !opts.Amend {
		if status, err = w.status(context.Background(), oldRev); err != nil {
			return plumbing.ZeroHash, err
		}
		if status.IsClean() {
			return plumbing.ZeroHash, ErrNoChanges
		}
		cs := newChanges(status, w.baseDir)
		if !cs.hasStagedChanges(opts.All) {
			cs.show()
			return plumbing.ZeroHash, ErrNothingToCommit
		}
	}

	var message string
	switch {
	case opts.File == "-":
		if message, err = messageReadFrom(os.Stdin); err != nil {
			return plumbing.ZeroHash, err
		}
	case len(opts.File) != 0:
		if message, err = messageReadFromPath(opts.File); err != nil {
			return plumbing.ZeroHash, err
		}
	case len(opts.Message) == 0:
		if message, err = w.messageFromPrompt(ctx, opts, current.BranchName(), status); err != nil {
			return plumbing.ZeroHash, err
		}
	default:
		message = genMessage(opts.Message)
	}

	if len(message) == 0 && !opts.AllowEmptyMessage {
		return plumbing.ZeroHash, ErrNotAllowEmptyMessage
	}

	if opts.All {
		if err := w.autoAddModifiedAndDeleted(ctx); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	var newTree plumbing.Hash
	if oldRev.IsZero() {
		if newTree, err = w.writeIndexAsTree(ctx, plumbing.ZeroHash, opts.AllowEmptyCommits); err != nil {
			return plumbing.ZeroHash, err
		}
	} else {
		cc, err := w.odb.Commit(ctx, oldRev)
		if err != nil {
			return plumbing.ZeroHash, err
		}

		if opts.Amend {
			newTree = cc.Tree
			opts.Parents = cc.Parents
		} else {
			if newTree, err = w.writeIndexAsTree(ctx, cc.Tree, opts.AllowEmptyCommits); err != nil {
				return plumbing.ZeroHash, err
			}
		}
	}

	commit, err := w.commitTree(ctx, &CommitTreeOptions{
		Tree:      newTree,
		Author:    opts.Author,
		Committer: opts.Committer,
		Parents:   opts.Parents,
		SignKey:   opts.SignKey,
		Message:   message,
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}
	reflogMessage := "commit: " + messageSubject(message)
	if current.IsBranch() {
		_ = w.writeHEADReflog(commit, &opts.Committer, reflogMessage)
	}
	// Allow creating commits to detached HEAD
	if err := w.DoUpdate(ctx, current, oldRev, commit, &opts.Committer, reflogMessage); err != nil {
		return plumbing.ZeroHash, err
	}
	return commit, nil
}

func (w *Worktree) autoAddModifiedAndDeleted(ctx context.Context) error {
	s, err := w.Status(ctx, false)
	if err != nil {
		return err
	}

	idx, err := w.odb.Index()
	if err != nil {
		return err
	}

	for path, fs := range s {
		if fs.Worktree != Modified && fs.Worktree != Deleted {
			continue
		}

		if _, _, err := w.doAddFile(ctx, idx, s, path, nil, false); err != nil {
			return err
		}

	}

	return w.odb.SetIndex(idx)
}

func (w *Worktree) Stats(ctx context.Context) error {
	current, err := w.Current()
	if err != nil {
		die_error("unable resolve current branch: %v", err)
		return err
	}
	cc, err := w.odb.Commit(ctx, current.Hash())
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
	fmt.Fprintf(os.Stdout, "[%s %s] %s\n %d files changed, %d insertions(+), %d deletions(-)\n",
		current.Name().Short(), shortHash(current.Hash()), cc.Subject(),
		len(stats), added, deleted)
	return nil
}
