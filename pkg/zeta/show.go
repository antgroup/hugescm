package zeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/hexview"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/patchview"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/tui"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type ShowOptions struct {
	Nav       bool
	Objects   []string
	Textconv  bool
	Algorithm diferenco.Algorithm
	Limit     int64
}

type showObject struct {
	name string
	oid  plumbing.Hash
}

func (r *Repository) parseObject(ctx context.Context, name string) (plumbing.Hash, int64, error) {
	prefix, p, ok := strings.Cut(name, ":")
	oid, err := r.Revision(ctx, prefix)
	if !ok || err != nil {
		return oid, 0, err
	}
	e, err := r.parseEntry(ctx, oid, p)
	if err != nil {
		return plumbing.ZeroHash, 0, err
	}
	return e.Hash, e.Size, nil
}

func (r *Repository) showFetch(ctx context.Context, o *promiseObject) error {
	if r.odb.Exists(o.oid, false) || r.odb.Exists(o.oid, true) {
		return nil
	}
	if !r.promisorEnabled() {
		return plumbing.NoSuchObject(o.oid)
	}
	return r.promiseMissingFetch(ctx, o)
}

func (r *Repository) Show(ctx context.Context, opts *ShowOptions) error {
	objects := make([]*showObject, 0, len(opts.Objects))
	for _, o := range opts.Objects {
		oid, size, err := r.parseObject(ctx, o)
		if err != nil {
			die_error("parse object %s error: %v", o, err)
			return err
		}
		if err := r.showFetch(ctx, &promiseObject{oid: oid, size: size}); err != nil {
			die_error("search object %s error: %v", oid, err)
			return err
		}
		objects = append(objects, &showObject{name: o, oid: oid})
	}
	p := NewPrinter(ctx)
	if opts.Nav {
		p = NewBuiltinPrinter(ctx)
	}
	defer p.Close() // nolint
	for _, o := range objects {
		if err := r.showOne(ctx, p, opts, o); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				break
			}
			return err
		}
	}
	return nil
}

func (r *Repository) showOne(ctx context.Context, w *printer, opts *ShowOptions, so *showObject) error {
	var o any
	var err error
	if o, err = r.odb.Object(ctx, so.oid); err != nil {
		if plumbing.IsNoSuchObject(err) {
			return r.showBlob(ctx, w, opts, so)
		}
		return catShowError(so.oid.String(), err)
	}
	switch a := o.(type) {
	case *object.Tree:
		return r.showTree(ctx, w, so, a)
	case *object.Commit:
		return r.showCommit(ctx, w, opts, a)
	case *object.Tag:
		return r.showTag(ctx, w, opts, a)
	case *object.Fragments:
		return r.showFragments(ctx, w, so, a)
	}
	return nil
}

func (r *Repository) showBlob(ctx context.Context, w Printer, opts *ShowOptions, so *showObject) error {
	b, err := r.catMissingObject(ctx, &promiseObject{oid: so.oid})
	if err != nil {
		return err
	}
	defer b.Close() // nolint
	if opts.Limit < 0 {
		opts.Limit = b.Size
	}
	reader, meta, err := diferenco.NewUnifiedReader(b.Contents, opts.Textconv)
	if err != nil {
		return err
	}
	if w.EnableColor() && !meta.IsText() {
		if opts.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			opts.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}
		return hexview.Format(reader, w, opts.Limit, w.ColorMode())
	}
	_, err = io.Copy(w, io.LimitReader(reader, opts.Limit))
	return err
}

func (r *Repository) showCommit(ctx context.Context, w *printer, opts *ShowOptions, cc *object.Commit) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	rdb, err := r.ReferencesEx(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve references error: %v\n", err)
		return err
	}
	// In non-nav mode we print the commit log header before the patch so
	// the output mirrors `git show`. Nav mode renders the same metadata
	// inside the patchview top header (see below), so emitting LogOne
	// here would just duplicate it (and corrupt the alt-screen).
	navMode := opts.Nav && term.StdoutLevel != term.LevelNone
	if !navMode {
		if err := w.LogOne(cc, rdb.M[cc.Hash]); err != nil {
			return err
		}
	}
	if len(cc.Parents) == 2 {
		if navMode {
			// Merge commits have no diff to navigate. We deliberately do
			// NOT route through the BuiltinPrinter pager here: feeding a
			// short metadata blob into the pager can still fall through
			// to the interactive bubbletea path (e.g. when term.GetSize
			// fails) and leave the terminal waiting for a keypress with
			// the prompt suppressed. Instead, render the same commit-
			// style header that we use for the diff-bearing path via
			// patchview.Run, which short-circuits to a plain stdout
			// header + "No changes" line when the patch list is empty.
			// The BuiltinPrinter created by Show stays empty and its
			// Close() is a no-op, so the prompt is restored cleanly.
			hash := cc.Hash.String()
			author := fmt.Sprintf("%s <%s>", cc.Author.Name, cc.Author.Email)
			date := cc.Author.When.Format(time.RFC1123Z)
			subject := firstLine(cc.Message)
			return patchview.Run(nil,
				patchview.WithCommitHeaderWithFiles(hash, author, date, subject, ""),
			)
		}
		return nil
	}
	oldTree := r.odb.EmptyTree()
	if len(cc.Parents) == 1 {
		pc, err := r.odb.Commit(ctx, cc.Parents[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve commit %s error: %v\n", cc.Parents[0], err)
			return err
		}
		if oldTree, err = pc.Root(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "resolve parent tree %s error: %v\n", cc.Parents[0], err)
			return err
		}
	}
	newTree, err := cc.Root(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve current tree %s error: %v\n", cc.Parents[0], err)
		return err
	}
	o := &object.DiffTreeOptions{
		DetectRenames:    true,
		OnlyExactRenames: true,
	}
	changes, err := object.DiffTreeWithOptions(ctx, oldTree, newTree, o, noder.NewSparseTreeMatcher(r.Core.SparseDirs))
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff tree error: %v\n", err)
		return err
	}
	patch, err := changes.Patch(ctx, &object.PatchOptions{
		Algorithm: opts.Algorithm,
		Textconv:  opts.Textconv,
	})
	if err != nil {
		return err
	}

	if navMode {
		// Build a top header with commit metadata so users can see what
		// they're navigating. We use the full 40-char hash to match the
		// behaviour of `git show`; the patchview will left-truncate the
		// line if the terminal is too narrow.
		hash := cc.Hash.String()
		author := fmt.Sprintf("%s <%s>", cc.Author.Name, cc.Author.Email)
		date := cc.Author.When.Format(time.RFC1123Z)
		subject := firstLine(cc.Message)
		files := patchview.ColorizedPatchSummary(patchview.DefaultStyle(), patch)
		return patchview.Run(patch,
			patchview.WithCommitHeaderWithFiles(hash, author, date, subject, files),
		)
	}

	e := diferenco.NewUnifiedEncoder(w, tui.EncoderOptions(w.ColorMode())...)
	_ = e.Encode(patch)
	return nil
}

func (r *Repository) showTag(ctx context.Context, w *printer, opts *ShowOptions, tag *object.Tag) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if w.EnableColor() {
		_, _ = fmt.Fprintf(w, "\x1b[33mtag %s\x1b[0m\n", tag.Name)
	} else {
		_, _ = fmt.Fprintf(w, "tag %s\n", tag.Name)
	}
	_, _ = fmt.Fprintf(w, "Tagger: %s <%s>\nDate:   %s\n\n%s\n", tag.Tagger.Name, tag.Tagger.Email, tag.Tagger.When.Format(time.RFC3339), tag.Content)
	var cc *object.Commit
	var err error
	switch tag.ObjectType {
	case object.TagObject:
		cc, err = r.odb.ParseRevExhaustive(ctx, tag.Object)
	case object.CommitObject:
		cc, err = r.odb.Commit(ctx, tag.Object)
	default:
		return backend.NewErrMismatchedObjectType(tag.Object, "commit")
	}
	if err != nil {
		return err
	}
	return r.showCommit(ctx, w, opts, cc)
}

func (r *Repository) showTree(ctx context.Context, w Printer, so *showObject, tree *object.Tree) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if w.EnableColor() {
		_, _ = fmt.Fprintf(w, "\x1b[33mtree %s\x1b[0m\n\n", so.name)
	} else {
		_, _ = fmt.Fprintf(w, "tree %s\n\n", so.name)
	}
	for _, e := range tree.Entries {
		t := e.Type()
		if t == object.TreeObject {
			_, _ = fmt.Fprintf(w, "%s/\n", e.Name)
			continue
		}
		if t == object.FragmentsObject && w.EnableColor() {
			_, _ = fmt.Fprintf(w, "\x1b[36m%s\x1b[0m\n", e.Name)
		}
		_, _ = fmt.Fprintln(w, e.Name)
	}
	return nil
}

func (r *Repository) showFragments(ctx context.Context, w Printer, so *showObject, ff *object.Fragments) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if w.EnableColor() {
		_, _ = fmt.Fprintf(w, "\x1b[33mfragments %s\x1b[0m\nraw:  %s\nsize: %d\n\n", so.oid, ff.Origin, ff.Size)
	} else {
		_, _ = fmt.Fprintf(w, "fragments %s\nraw:  %s\nsize: %d\n", so.oid, ff.Origin, ff.Size)
	}
	for _, e := range ff.Entries {
		_, _ = fmt.Fprintf(w, "%d\t%s %d\n", e.Index, e.Hash, e.Size)
	}
	return nil
}

// firstLine returns the first non-empty line of s, stripped of trailing
// CR/LF. Used to extract the commit subject from a full commit message
// for the patchview top header.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			return line
		}
	}
	return ""
}
