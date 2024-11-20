// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/antgroup/hugescm/modules/diff"
	dmp "github.com/antgroup/hugescm/modules/diffmatchpatch"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/filesystem"
	mindex "github.com/antgroup/hugescm/modules/merkletrie/index"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	fdiff "github.com/antgroup/hugescm/modules/plumbing/format/diff"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type fileWrapper struct {
	name string
	hash plumbing.Hash
	mode filemode.FileMode
}

func (f *fileWrapper) Path() string {
	return f.name
}

func (f *fileWrapper) Hash() plumbing.Hash {
	return f.hash
}

func (f *fileWrapper) Mode() filemode.FileMode {
	return f.mode
}

var (
	_ fdiff.File = &fileWrapper{}
)

func (w *Worktree) openText(p string, size int64, textConv bool) (string, error) {
	fd, err := w.fs.Open(p)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	content, _, err := object.GetUnifiedText(fd, size, textConv)
	return content, err
}

func (w *Worktree) openBlobText(ctx context.Context, oid plumbing.Hash, textConv bool) (string, error) {
	br, err := w.odb.Blob(ctx, oid)
	if err != nil {
		return "", err
	}
	defer br.Close()
	content, _, err := object.GetUnifiedText(br.Contents, br.Size, textConv)
	return content, err
}

const (
	diffSizeLimit = 50 * 1024 * 1024 // 50M
)

func (w *Worktree) resolveContent(ctx context.Context, p noder.Path, textconv bool) (f fdiff.File, content string, fragments bool, bin bool, err error) {
	if p == nil {
		return nil, "", false, false, nil
	}
	name := p.String()
	switch a := p.Last().(type) {
	case *filesystem.Node:
		f = &fileWrapper{name: name, hash: a.HashRaw(), mode: a.Mode()}
		if a.Size() > diffSizeLimit {
			return f, "", false, true, nil
		}
		content, err = w.openText(name, a.Size(), textconv)
		if err == object.ErrNotTextContent {
			return f, "", false, true, nil
		}
		return f, content, false, false, nil
	case *mindex.Node:
		f = &fileWrapper{name: name, hash: a.HashRaw(), mode: a.Mode()}
		if a.IsFragments() {
			return f, "", true, false, err
		}
		if a.Size() > diffSizeLimit {
			return f, "", false, true, nil
		}
		content, err = w.openBlobText(ctx, a.HashRaw(), textconv)
		// When the current repository uses an incomplete checkout mechanism, we treat these files as binary files, i.e. no differences can be calculated.
		if err == object.ErrNotTextContent || plumbing.IsNoSuchObject(err) {
			return f, "", false, true, nil
		}
		return f, content, false, false, nil
	case *object.TreeNoder:
		f = &fileWrapper{name: name, hash: a.HashRaw(), mode: a.Mode()}
		if a.IsFragments() {
			return f, "", true, false, err
		}
		if a.Size() > diffSizeLimit {
			return f, "", false, true, nil
		}
		content, err = w.openBlobText(ctx, a.HashRaw(), textconv)
		if err == object.ErrNotTextContent || plumbing.IsNoSuchObject(err) {
			return f, "", false, true, nil
		}
		return f, content, a.IsFragments(), false, nil
	default:
	}
	return nil, "", false, false, errors.New("unsupport noder type")
}

func (w *Worktree) filePatchWithContext(ctx context.Context, c *merkletrie.Change, textconv bool) (fdiff.FilePatch, error) {
	if c.From == nil && c.To == nil {
		return nil, errors.New("malformed change: nil from and to")
	}
	from, fromContent, isFragmentsA, isBinA, err := w.resolveContent(ctx, c.From, textconv)
	if err != nil {
		return nil, err
	}
	to, toContent, isFragmentsB, isBinB, err := w.resolveContent(ctx, c.To, textconv)
	if err != nil {
		return nil, err
	}
	if isFragmentsA || isFragmentsB {
		return object.NewFilePatchWrapper(nil, from, to, true), nil
	}
	if isBinA || isBinB {
		return object.NewFilePatchWrapper(nil, from, to, false), nil
	}
	diffs := diff.Do(fromContent, toContent)

	var chunks []fdiff.Chunk
	for _, d := range diffs {
		select {
		case <-ctx.Done():
			return nil, object.ErrCanceled
		default:
		}

		var op fdiff.Operation
		switch d.Type {
		case dmp.DiffEqual:
			op = fdiff.Equal
		case dmp.DiffDelete:
			op = fdiff.Delete
		case dmp.DiffInsert:
			op = fdiff.Add
		}

		chunks = append(chunks, object.NewTextChunk(d.Text, op))
	}
	return object.NewFilePatchWrapper(chunks, from, to, false), nil
}

// getPatchContext: In the object package, there is no patch implementation for worktree diff, so we need
func (w *Worktree) getPatchContext(ctx context.Context, changes merkletrie.Changes, m *Matcher, textconv bool) ([]fdiff.FilePatch, error) {
	var filePatches []fdiff.FilePatch
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
		fp, err := w.filePatchWithContext(ctx, &c, textconv)
		if err != nil {
			return nil, err
		}

		filePatches = append(filePatches, fp)
	}
	return filePatches, nil
}

func (w *Worktree) diffWorktree(ctx context.Context, opts *DiffContextOptions, writer io.Writer) error {
	changes, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return err
	}
	if opts.NameOnly || opts.NameStatus {
		return opts.formatChanges(changes, writer)
	}
	m := NewMatcher(opts.PathSpec)
	filePatchs, err := w.getPatchContext(ctx, changes, m, opts.Textconv)
	if err != nil {
		return err
	}
	return opts.format(object.NewPatch("", filePatchs), writer)
}

func (w *Worktree) readBaseTree(ctx context.Context, oid plumbing.Hash, opts *DiffContextOptions) (*object.Tree, error) {
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

func (w *Worktree) DiffTreeWithIndex(ctx context.Context, oid plumbing.Hash, opts *DiffContextOptions, writer io.Writer) error {
	tree, err := w.readBaseTree(ctx, oid, opts)
	if err != nil {
		return err
	}
	changes, err := w.diffTreeWithStaging(ctx, tree, false)
	if err != nil {
		return err
	}
	if opts.NameOnly || opts.NameStatus {
		return opts.formatChanges(changes, writer)
	}
	m := NewMatcher(opts.PathSpec)
	filePatchs, err := w.getPatchContext(ctx, changes, m, opts.Textconv)
	if err != nil {
		return err
	}
	return opts.format(object.NewPatch("", filePatchs), writer)
}

func (w *Worktree) DiffTreeWithWorktree(ctx context.Context, oid plumbing.Hash, opts *DiffContextOptions, writer io.Writer) error {
	tree, err := w.readBaseTree(ctx, oid, opts)
	if err != nil {
		return err
	}
	rawChanges, err := w.diffTreeWithWorktree(ctx, tree, false)
	if err != nil {
		return err
	}
	changes := w.excludeIgnoredChanges(rawChanges)
	if opts.NameOnly || opts.NameStatus {
		return opts.formatChanges(changes, writer)
	}
	m := NewMatcher(opts.PathSpec)
	filePatchs, err := w.getPatchContext(ctx, changes, m, opts.Textconv)
	if err != nil {
		return err
	}
	return opts.format(object.NewPatch("", filePatchs), writer)
}

func (w *Worktree) resolveBetweenTree(ctx context.Context, opts *DiffContextOptions) (oldTree *object.Tree, newTree *object.Tree, err error) {
	if !opts.ThreeWayCompare {
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

func (w *Worktree) between(ctx context.Context, opts *DiffContextOptions, writer io.Writer) error {
	oldTree, newTree, err := w.resolveBetweenTree(ctx, opts)
	if err != nil {
		return err
	}
	o := &object.DiffTreeOptions{
		DetectRenames:    true,
		OnlyExactRenames: true,
	}
	changes, err := object.DiffTreeWithOptions(ctx, oldTree, newTree, o, noder.NewSparseTreeMatcher(w.Core.SparseDirs))
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff tree error: %v\n", err)
		return err
	}
	patch, err := opts.PatchContext(ctx, changes)
	if err != nil {
		die_error("patch %v", err)
		return err
	}
	return opts.formatEx(patch, writer)
}

func (w *Worktree) DiffContext(ctx context.Context, opts *DiffContextOptions, writer io.Writer) error {
	if len(opts.From) != 0 && len(opts.To) != 0 {
		w.DbgPrint("from %s to %s", opts.From, opts.To)
		return w.between(ctx, opts, writer)
	}
	if len(opts.From) != 0 {
		oid, err := w.Revision(ctx, opts.From)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve revision %s error: %v\n", opts.From, err)
			return err
		}
		if opts.Staged {
			if err := w.DiffTreeWithIndex(ctx, oid, opts, writer); err != nil {
				fmt.Fprintf(os.Stderr, "zeta diff --cached error: %v\n", err)
				return err
			}
			return nil
		}
		w.DbgPrint("from %s to worktree", oid)
		if err := w.DiffTreeWithWorktree(ctx, oid, opts, writer); err != nil {
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
		if err := w.DiffTreeWithIndex(ctx, ref.Hash(), opts, writer); err != nil {
			fmt.Fprintf(os.Stderr, "zeta diff --cached error: %v\n", err)
			return err
		}
		return nil
	}
	if err := w.diffWorktree(ctx, opts, writer); err != nil {
		fmt.Fprintf(os.Stderr, "zeta diff error: %v\n", err)
		return err
	}

	return nil
}
