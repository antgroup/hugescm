// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/filesystem"
	mindex "github.com/antgroup/hugescm/modules/merkletrie/index"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func (w *Worktree) openText(p string, size int64, textConv bool) (string, error) {
	fd, err := w.fs.Open(p)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	content, _, err := diferenco.ReadUnifiedText(fd, size, textConv)
	return content, err
}

func (w *Worktree) openBlobText(ctx context.Context, oid plumbing.Hash, textConv bool) (string, error) {
	br, err := w.odb.Blob(ctx, oid)
	if err != nil {
		return "", err
	}
	defer br.Close()
	content, _, err := diferenco.ReadUnifiedText(br.Contents, br.Size, textConv)
	return content, err
}

func (w *Worktree) readContent(ctx context.Context, p noder.Path, textConv bool) (f *diferenco.File, content string, fragments bool, bin bool, err error) {
	if p == nil {
		return nil, "", false, false, nil
	}
	name := p.String()
	switch a := p.Last().(type) {
	case *filesystem.Node:
		f = &diferenco.File{Name: name, Hash: a.HashRaw().String(), Mode: uint32(a.Mode())}
		if a.Size() > diferenco.MAX_DIFF_SIZE {
			return f, "", false, true, nil
		}
		content, err = w.openText(name, a.Size(), textConv)
		if err == diferenco.ErrNonTextContent {
			return f, "", false, true, nil
		}
		return f, content, false, false, nil
	case *mindex.Node:
		f = &diferenco.File{Name: name, Hash: a.HashRaw().String(), Mode: uint32(a.Mode())}
		if a.IsFragments() {
			return f, "", true, false, err
		}
		if a.Size() > diferenco.MAX_DIFF_SIZE {
			return f, "", false, true, nil
		}
		content, err = w.openBlobText(ctx, a.HashRaw(), textConv)
		// When the current repository uses an incomplete checkout mechanism, we treat these files as binary files, i.e. no differences can be calculated.
		if err == diferenco.ErrNonTextContent || plumbing.IsNoSuchObject(err) {
			return f, "", false, true, nil
		}
		return f, content, false, false, nil
	case *object.TreeNoder:
		f = &diferenco.File{Name: name, Hash: a.HashRaw().String(), Mode: uint32(a.Mode())}
		if a.IsFragments() {
			return f, "", true, false, err
		}
		if a.Size() > diferenco.MAX_DIFF_SIZE {
			return f, "", false, true, nil
		}
		content, err = w.openBlobText(ctx, a.HashRaw(), textConv)
		if err == diferenco.ErrNonTextContent || plumbing.IsNoSuchObject(err) {
			return f, "", false, true, nil
		}
		return f, content, a.IsFragments(), false, nil
	default:
	}
	return nil, "", false, false, errors.New("unsupport noder type")
}

func (w *Worktree) filePatchWithContext(ctx context.Context, c *merkletrie.Change, textconv bool) (*diferenco.Unified, error) {
	if c.From == nil && c.To == nil {
		return nil, errors.New("malformed change: nil from and to")
	}
	from, fromContent, isFragmentsA, isBinA, err := w.readContent(ctx, c.From, textconv)
	if err != nil {
		return nil, err
	}
	to, toContent, isFragmentsB, isBinB, err := w.readContent(ctx, c.To, textconv)
	if err != nil {
		return nil, err
	}
	if isFragmentsA || isFragmentsB {
		return &diferenco.Unified{From: from, To: to, IsFragments: true}, nil
	}
	if isBinA || isBinB {
		return &diferenco.Unified{From: from, To: to, IsBinary: true}, nil
	}
	return diferenco.DoUnified(ctx, &diferenco.Options{From: from, To: to, S1: fromContent, S2: toContent})
}

// getPatchContext: In the object package, there is no patch implementation for worktree diff, so we need
func (w *Worktree) getPatchContext(ctx context.Context, changes merkletrie.Changes, m *Matcher, textConv bool) ([]*diferenco.Unified, error) {
	var filePatches []*diferenco.Unified
	for _, c := range changes {
		select {
		case <-ctx.Done():
			return nil, object.ErrCanceled
		default:
		}
		name := nameFromAction(&c)
		if !m.Match(name) {
			continue
		}
		p, err := w.filePatchWithContext(ctx, &c, textConv)
		if err != nil {
			return nil, err
		}

		filePatches = append(filePatches, p)
	}
	return filePatches, nil
}

func nameFromDifeName(from, to *diferenco.File) string {
	if from == nil && to == nil {
		return ""
	}
	if from == nil {
		return to.Name
	}
	if to == nil {
		return from.Name
	}
	if from.Name != to.Name {
		return fmt.Sprintf("%s => %s", from.Name, to.Name)
	}
	return from.Name
}

func (w *Worktree) fileStatWithContext(ctx context.Context, c *merkletrie.Change, textconv bool) (*object.FileStat, error) {
	if c.From == nil && c.To == nil {
		return nil, errors.New("malformed change: nil from and to")
	}
	from, fromContent, isFragmentsA, isBinA, err := w.readContent(ctx, c.From, textconv)
	if err != nil {
		return nil, err
	}
	to, toContent, isFragmentsB, isBinB, err := w.readContent(ctx, c.To, textconv)
	if err != nil {
		return nil, err
	}
	s := &object.FileStat{Name: nameFromDifeName(from, to)}
	if isFragmentsA || isFragmentsB {
		return s, nil
	}
	if isBinA || isBinB {
		return s, nil
	}
	stat, err := diferenco.Stat(ctx, &diferenco.Options{From: from, To: to, S1: fromContent, S2: toContent})
	if err != nil {
		return nil, err
	}
	s.Addition = stat.Addition
	s.Deletion = stat.Deletion
	return s, nil
}

func (w *Worktree) getStatsContext(ctx context.Context, changes merkletrie.Changes, m *Matcher, textConv bool) (object.FileStats, error) {
	var fileStats []object.FileStat
	for _, c := range changes {
		select {
		case <-ctx.Done():
			return nil, object.ErrCanceled
		default:
		}
		name := nameFromAction(&c)
		if !m.Match(name) {
			continue
		}
		s, err := w.fileStatWithContext(ctx, &c, textConv)
		if err != nil {
			return nil, err
		}

		fileStats = append(fileStats, *s)
	}
	return fileStats, nil
}

func (w *Worktree) showChanges(ctx context.Context, opts *DiffOptions, changes merkletrie.Changes) error {
	if opts.NameOnly || opts.NameStatus {
		return opts.showChangesStatus(ctx, changes)
	}
	m := NewMatcher(opts.PathSpec)
	if opts.showStatsOnly() {
		fileStats, err := w.getStatsContext(ctx, changes, m, opts.Textconv)
		if err != nil {
			return err
		}
		return opts.showStats(ctx, fileStats)
	}

	filePatchs, err := w.getPatchContext(ctx, changes, m, opts.Textconv)
	if err != nil {
		return err
	}
	return opts.showPatch(ctx, filePatchs)
}

func (w *Worktree) diffWorktree(ctx context.Context, opts *DiffOptions) error {
	changes, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return err
	}
	return w.showChanges(ctx, opts, changes)
}

func (w *Worktree) readBaseTree(ctx context.Context, oid plumbing.Hash, opts *DiffOptions) (*object.Tree, error) {
	if len(opts.MergeBase) == 0 {
		return w.readTree(ctx, oid, "")
	}
	var err error
	var a, b *object.Commit
	if a, err = w.odb.ParseRevExhaustive(ctx, oid); err != nil {
		return nil, err
	}
	if b, err = w.parseRevExhaustive(ctx, opts.MergeBase); err != nil {
		return nil, err
	}
	bases, err := a.MergeBase(ctx, b)
	if err != nil {
		return nil, err
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("merge-base: %s and %s have no common ancestor", opts.MergeBase, oid)
	}
	return bases[0].Root(ctx)
}

func (w *Worktree) DiffTreeWithIndex(ctx context.Context, oid plumbing.Hash, opts *DiffOptions) error {
	tree, err := w.readBaseTree(ctx, oid, opts)
	if err != nil {
		return err
	}
	changes, err := w.diffTreeWithStaging(ctx, tree, false)
	if err != nil {
		return err
	}
	return w.showChanges(ctx, opts, changes)
}

func (w *Worktree) DiffTreeWithWorktree(ctx context.Context, oid plumbing.Hash, opts *DiffOptions) error {
	tree, err := w.readBaseTree(ctx, oid, opts)
	if err != nil {
		return err
	}
	rawChanges, err := w.diffTreeWithWorktree(ctx, tree, false)
	if err != nil {
		return err
	}
	changes := w.excludeIgnoredChanges(rawChanges)
	return w.showChanges(ctx, opts, changes)
}

func (w *Worktree) resolveBetweenTree(ctx context.Context, opts *DiffOptions) (oldTree *object.Tree, newTree *object.Tree, err error) {
	if !opts.W3 {
		if oldTree, err = w.parseTreeExhaustive(ctx, opts.From, ""); err != nil {
			fmt.Fprintf(os.Stderr, "resolve tree: %s error: %v\n", opts.From, err)
			return
		}
		if newTree, err = w.parseTreeExhaustive(ctx, opts.To, ""); err != nil {
			fmt.Fprintf(os.Stderr, "resolve tree: %s error: %v\n", opts.To, err)
			return
		}
		return
	}
	var a, b *object.Commit
	if a, err = w.parseRevExhaustive(ctx, opts.From); err != nil {
		return nil, nil, err
	}
	if b, err = w.parseRevExhaustive(ctx, opts.To); err != nil {
		return nil, nil, err
	}
	bases, err := a.MergeBase(ctx, b)
	if err != nil {
		return nil, nil, err
	}
	if len(bases) == 0 {
		if oldTree, err = a.Root(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "resolve tree: %s error: %v\n", opts.From, err)
			return
		}
		if newTree, err = b.Root(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "resolve tree: %s error: %v\n", opts.To, err)
			return
		}
		return
	}
	if oldTree, err = bases[0].Root(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "resolve tree: %s error: %v\n", opts.From, err)
		return
	}
	if newTree, err = b.Root(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "resolve tree: %s error: %v\n", opts.To, err)
		return
	}
	return
}

func (w *Worktree) between(ctx context.Context, opts *DiffOptions) error {
	w.DbgPrint("from %s to %s", opts.From, opts.To)
	oldTree, newTree, err := w.resolveBetweenTree(ctx, opts)
	if err != nil {
		return err
	}
	o := &object.DiffTreeOptions{
		DetectRenames:    true,
		OnlyExactRenames: true,
	}
	w.DbgPrint("oldTree %s newTree %s", oldTree.Hash, newTree.Hash)
	changes, err := object.DiffTreeWithOptions(ctx, oldTree, newTree, o, noder.NewSparseTreeMatcher(w.Core.SparseDirs))
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff tree error: %v\n", err)
		return err
	}
	return opts.ShowChanges(ctx, changes)
}

func (w *Worktree) DiffContext(ctx context.Context, opts *DiffOptions) error {
	if opts.Algorithm == diferenco.Unspecified && len(w.Diff.Algorithm) != 0 {
		if a, err := diferenco.AlgorithmFromName(w.Diff.Algorithm); err != nil {
			warn("diff: bad config, key: diff.algorithm value: %s", w.Diff.Algorithm)
		} else {
			opts.Algorithm = a
		}
	}
	if len(opts.From) != 0 && len(opts.To) != 0 {
		return w.between(ctx, opts)
	}
	if len(opts.From) != 0 {
		oid, err := w.Revision(ctx, opts.From)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve revision %s error: %v\n", opts.From, err)
			return err
		}
		if opts.Staged {
			if err := w.DiffTreeWithIndex(ctx, oid, opts); err != nil {
				fmt.Fprintf(os.Stderr, "zeta diff --cached error: %v\n", err)
				return err
			}
			return nil
		}
		w.DbgPrint("from %s to worktree", oid)
		if err := w.DiffTreeWithWorktree(ctx, oid, opts); err != nil {
			fmt.Fprintf(os.Stderr, "zeta diff error: %v\n", err)
			return err
		}
		return nil
	}
	if opts.Staged {
		ref, err := w.Current()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve current branch error: %v\n", err)
			return err
		}
		if err := w.DiffTreeWithIndex(ctx, ref.Hash(), opts); err != nil {
			fmt.Fprintf(os.Stderr, "zeta diff --cached error: %v\n", err)
			return err
		}
		return nil
	}
	if err := w.diffWorktree(ctx, opts); err != nil {
		fmt.Fprintf(os.Stderr, "zeta diff error: %v\n", err)
		return err
	}

	return nil
}
