// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/progress"
)

var (
	ErrHasUnstaged = errors.New("has unstaged")
)

func (w *Worktree) Checkout(ctx context.Context, opts *CheckoutOptions) error {
	if opts.First {
		return w.checkoutFirstTime(ctx, opts)
	}
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", 0, opts.Quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.checkout(ctx, opts, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	return nil
}

func (w *Worktree) checkoutSlow(ctx context.Context, opts *CheckoutOptions, bar ProgressBar) error {
	if bar == nil {
		bar = &nonProgressBar{}
	}
	c, err := w.getCommitFromCheckoutOptions(ctx, opts)
	if err != nil {
		return err
	}

	if !opts.Hash.IsZero() && !opts.Create {
		err = w.setHEADToCommit(opts.Hash)
	} else {
		err = w.setHEADToBranch(opts.Branch, c)
	}

	if err != nil {
		return err
	}
	t, err := w.getTreeFromCommitHash(ctx, c)
	if err != nil {
		return err
	}
	if _, err := w.resetIndex(ctx, t); err != nil {
		return err
	}
	if err := w.checkoutWorktree(ctx, t, bar); err != nil {
		return err
	}
	return nil
}

func (w *Worktree) validChangeIgnore(ch merkletrie.Change, ignore map[string]bool) (bool, error) {
	action, err := ch.Action()
	if err != nil {
		return false, nil
	}

	switch action {
	case merkletrie.Delete:
		name := ch.From.String()
		return ignore[name], validPath(name)
	case merkletrie.Insert:
		name := ch.To.String()
		return ignore[name], validPath(name)
	case merkletrie.Modify:
		name := ch.From.String()
		return ignore[name], validPath(name, ch.To.String())
	}

	return false, nil
}

func (w *Worktree) resetIndexIgnoreFiles(ctx context.Context, t *object.Tree, doNotCheckouts map[string]bool) error {
	idx, err := w.odb.Index()

	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)

	changes, err := w.diffTreeWithStaging(ctx, t, true)
	if err != nil {
		return err
	}

	for _, ch := range changes {
		a, err := ch.Action()
		if err != nil {
			return err
		}

		var name string
		var e *object.TreeEntry

		switch a {
		case merkletrie.Modify, merkletrie.Insert:
			name = ch.To.String()
			e, err = t.FindEntry(ctx, name)
			if err != nil {
				return err
			}
		case merkletrie.Delete:
			name = ch.From.String()
		}
		if doNotCheckouts[name] {
			continue
		}
		b.Remove(name)
		if e == nil {
			continue
		}
		b.Add(&index.Entry{
			Name: name,
			Hash: e.Hash,
			Mode: e.Mode,
			Size: uint64(e.Size),
		})
	}

	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) checkoutIgnoreFiles(ctx context.Context, t *object.Tree, doNotCheckouts map[string]bool, bar ProgressBar) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)

	changes, err := w.diffStagingWithWorktree(ctx, true, false)
	if err != nil {
		return err
	}
	changes = rearrangeChanges(changes)
	for _, ch := range changes {
		skip, err := w.validChangeIgnore(ch, doNotCheckouts)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if err := w.checkoutChange(ctx, ch, t, b, bar); err != nil {
			return err
		}
	}

	b.Write(idx)
	return w.odb.SetIndex(idx)
}

var (
	ErrAborting = errors.New("aborting")
)

// Checkout switch branches or restore working tree files.
func (w *Worktree) checkout(ctx context.Context, opts *CheckoutOptions, bar ProgressBar) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	if opts.Create {
		if err := w.createBranch(opts); err != nil {
			return err
		}
	}
	if opts.Force {
		return w.checkoutSlow(ctx, opts, bar)
	}
	current, err := w.Current()
	if err == plumbing.ErrReferenceNotFound {
		return w.checkoutSlow(ctx, opts, bar)
	}
	if err != nil {
		return err
	}
	commit := current.Hash()
	indexChanges, err := w.diffCommitWithStaging(ctx, commit, false)
	if err != nil {
		return err
	}
	status := make(Status)
	for _, ch := range indexChanges {
		a, err := ch.Action()
		if err != nil {
			return err
		}

		fs := status.File(nameFromAction(&ch))
		fs.Worktree = Unmodified

		switch a {
		case merkletrie.Delete:
			status.File(ch.From.String()).Staging = Deleted
		case merkletrie.Insert:
			status.File(ch.To.String()).Staging = Added
		case merkletrie.Modify:
			status.File(ch.To.String()).Staging = Modified
		}
	}

	rawChanges, err := w.diffStagingWithWorktree(ctx, false, false)
	if err != nil {
		return err
	}
	worktreeChanges := w.excludeIgnoredChanges(rawChanges)
	if len(indexChanges) == 0 && len(worktreeChanges) == 0 {
		// no changes: checkout
		return w.checkoutSlow(ctx, opts, bar)
	}
	worktreeChanges = rearrangeChanges(worktreeChanges)
	for _, ch := range worktreeChanges {
		a, err := ch.Action()
		if err != nil {
			return err
		}

		fs := status.File(nameFromAction(&ch))
		if fs.Staging == Untracked {
			fs.Staging = Unmodified
		}

		switch a {
		case merkletrie.Delete:
			fs.Worktree = Deleted
		case merkletrie.Insert:
			fs.Worktree = Untracked
			fs.Staging = Untracked
		case merkletrie.Modify:
			fs.Worktree = Modified
		}
	}
	overwrites := make([]string, 0, len(status))
	doNotCheckouts := make(map[string]bool)
	oldTree, err := w.readTree(ctx, current.Hash(), "")
	if err != nil {
		return err
	}
	cc, err := w.getCommitFromCheckoutOptions(ctx, opts)
	if err != nil {
		return err
	}
	newTree, err := w.readTree(ctx, cc, "")
	if err != nil {
		return err
	}
	for p, s := range status {
		if s.Worktree == Unmodified && s.Staging == Unmodified {
			continue
		}
		a, aErr := oldTree.FindEntry(ctx, p)
		if aErr != nil && !object.IsErrEntryNotFound(aErr) {
			return aErr
		}
		b, bErr := newTree.FindEntry(ctx, p)
		if bErr != nil && !object.IsErrEntryNotFound(bErr) {
			return bErr
		}
		if a.Equal(b) {
			doNotCheckouts[p] = true
			continue
		}
		overwrites = append(overwrites, p)
	}
	if len(overwrites) != 0 {
		die_error("Your local changes to the following files would be overwritten by checkout:")
		for _, s := range overwrites {
			fmt.Fprintf(os.Stderr, "    %s\n", s)
		}

		fmt.Fprintf(os.Stderr, "%s\n%s\n", W("Please commit your changes or stash them before you switch branches."), W("Aborting"))
		return ErrAborting
	}

	if !opts.Hash.IsZero() && !opts.Create {
		err = w.setHEADToCommit(opts.Hash)
	} else {
		err = w.setHEADToBranch(opts.Branch, cc)
	}
	if err != nil {
		return err
	}
	if err := w.resetIndexIgnoreFiles(ctx, newTree, doNotCheckouts); err != nil {
		return err
	}
	if err := w.checkoutIgnoreFiles(ctx, newTree, doNotCheckouts, bar); err != nil {
		return err
	}
	return nil
}

// Only call zeta checkout or migrate
func (w *Worktree) checkoutFirstTime(ctx context.Context, opts *CheckoutOptions) error {
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", 0, opts.Quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.checkoutFirstTimeInternal(ctx, opts, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	if opts.One {
		return w.checkoutOneAfterAnother(ctx)
	}
	return nil
}

func (w *Worktree) checkoutFirstTimeInternal(ctx context.Context, opts *CheckoutOptions, bar ProgressBar) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	if opts.Create {
		if err := w.createBranch(opts); err != nil {
			return err
		}
	}
	c, err := w.getCommitFromCheckoutOptions(ctx, opts)
	if err != nil {
		return err
	}
	if !opts.Hash.IsZero() && !opts.Create {
		err = w.setHEADToCommit(opts.Hash)
	} else {
		err = w.setHEADToBranch(opts.Branch, c)
	}
	if err != nil {
		return err
	}

	t, err := w.getTreeFromCommitHash(ctx, c)
	if err != nil {
		return err
	}
	if _, err := w.resetIndex(ctx, t); err != nil {
		return err
	}
	if err := w.resetWorktreeFast(ctx, t, bar); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rreset worktree error: %v\n", err)
		return err
	}
	return nil
}

type checkoutEntry struct {
	name  string
	entry *object.TreeEntry
}

type indexRecv func(*index.Entry)

type checkoutGroup struct {
	ch     chan *checkoutEntry
	errors chan error
	wg     sync.WaitGroup
	recv   indexRecv
}

func (cg *checkoutGroup) waitClose() {
	close(cg.ch)
	cg.wg.Wait()
}

func (cg *checkoutGroup) submit(ctx context.Context, e *checkoutEntry) error {
	// In case the context has been cancelled, we have a race between observing an error from
	// the killed Git process and observing the context cancellation itself. But if we end up
	// here because of cancellation of the Git process, we don't want to pass that one down the
	// pipeline but instead just stop the pipeline gracefully. We thus have this check here up
	// front to error messages from the Git process.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cg.errors:
		return err
	default:
	}

	select {
	case cg.ch <- e:
		return nil
	case err := <-cg.errors:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cg *checkoutGroup) coco(ctx context.Context, w *Worktree, bar ProgressBar) error {
	for e := range cg.ch {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}
		if err := w.checkoutFile(ctx, e.name, e.entry, bar); err != nil {
			if plumbing.IsNoSuchObject(err) && w.missingNotFailure {
				w.addPseudoIndexRecv(e.name, e.entry, cg.recv)
				continue
			}
			if filemode.IsErrMalformedMode(err) {
				_, _ = term.Fprintf(os.Stderr, "\x1b[2K\rskip checkout '\x1b[31m%s\x1b[0m': malformed mode '%s'\n", e.name, e.entry.Mode)
				w.addPseudoIndexRecv(e.name, e.entry, cg.recv)
				continue
			}
			fmt.Fprintf(os.Stderr, "\x1b[2K\rcheckout file %s error: %v\n", e.name, err)
			return err
		}
		if err := w.addIndex(e.name, e.entry, cg.recv); err != nil {
			fmt.Fprintf(os.Stderr, "\x1b[2K\rreset file %s index error: %v\n", e.name, err)
			return err
		}
	}
	return nil
}

func (cg *checkoutGroup) run(ctx context.Context, w *Worktree, bar ProgressBar) {
	cg.wg.Add(1)
	go func() {
		defer cg.wg.Done()
		err := cg.coco(ctx, w, bar)
		cg.errors <- err
	}()
}

func (w *Worktree) addIndex(name string, entry *object.TreeEntry, recv indexRecv) error {
	fi, err := w.fs.Lstat(name)
	if err != nil {
		return err
	}
	e := &index.Entry{
		Hash:       entry.Hash,
		Name:       name,
		Mode:       entry.Mode,
		ModifiedAt: fi.ModTime(),
		Size:       uint64(fi.Size()),
	}

	// if the FileInfo.Sys() comes from os the ctime, dev, inode, uid and gid
	// can be retrieved, otherwise this doesn't apply
	if fillSystemInfo != nil {
		fillSystemInfo(e, fi.Sys())
	}
	recv(e)
	return nil
}

func (w *Worktree) addPseudoIndexRecv(name string, entry *object.TreeEntry, recv indexRecv) {
	now := time.Now()
	e := &index.Entry{
		Hash:       entry.Hash,
		Name:       name,
		Mode:       entry.Mode,
		ModifiedAt: now,
		CreatedAt:  now,
		Size:       uint64(entry.Size),
	}
	recv(e)
}

const (
	batchLimit = 8
)

func (w *Worktree) resetWorktreeFast(ctx context.Context, t *object.Tree, bar ProgressBar) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}

	newCtx, cancelCtx := context.WithCancelCause(ctx)
	defer cancelCtx(nil)
	b := newIndexBuilder(idx)
	var mu sync.Mutex
	recv := func(e *index.Entry) {
		mu.Lock()
		defer mu.Unlock()
		b.Remove(e.Name)
		b.Add(e)
	}
	cg := &checkoutGroup{
		ch:     make(chan *checkoutEntry, 20), // 4 goroutine
		errors: make(chan error, batchLimit),
		recv:   recv,
	}
	for range batchLimit {
		cg.run(newCtx, w, bar)
	}
	for _, e := range idx.Entries {
		var entry *object.TreeEntry
		if entry, err = t.FindEntry(ctx, e.Name); err != nil {
			cg.waitClose()
			return err
		}
		if err := cg.submit(newCtx, &checkoutEntry{name: e.Name, entry: entry}); err != nil {
			cg.waitClose()
			return err
		}
	}
	cg.waitClose()
	close(cg.errors)
	for err = range cg.errors {
		if err != nil {
			return err
		}
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) unstagedChanges(ctx context.Context) (merkletrie.Changes, error) {
	ch, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return nil, err
	}
	var changes merkletrie.Changes
	for _, c := range ch {
		a, err := c.Action()
		if err != nil {
			return nil, err
		}

		if a == merkletrie.Insert {
			continue
		}
		changes = append(changes, c)
	}

	return changes, nil
}

func (w *Worktree) checkoutWorktree(ctx context.Context, t *object.Tree, bar ProgressBar) error {
	changes, err := w.diffStagingWithWorktree(ctx, true, false)
	if err != nil {
		return err
	}

	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)
	// Rearrange
	changes = rearrangeChanges(changes)
	for _, ch := range changes {
		if err := w.validChange(ch); err != nil {
			return err
		}
		if err := w.checkoutChange(ctx, ch, t, b, bar); err != nil {
			return err
		}
	}

	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func rearrangeChanges(changes merkletrie.Changes) merkletrie.Changes {
	recs := make([]merkletrie.Change, 0, len(changes))
	var modified merkletrie.Changes
	for _, ch := range changes {
		if a, err := ch.Action(); err == nil && a == merkletrie.Delete {
			recs = append(recs, ch)
			continue
		}
		modified = append(modified, ch)
	}
	recs = append(recs, modified...)
	return recs
}
