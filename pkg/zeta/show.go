package zeta

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/diferenco/color"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type ShowOptions struct {
	Objects          []string
	Textconv         bool
	Algorithm        diferenco.Algorithm
	Limit            int64
	w                *printer
	isColorSupported bool
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
	defer p.Close()
	opts.w = p
	fmt.Fprintf(os.Stderr, "%v %v\n", p.is256ColorSupported, p.isTrueColorSupported)
	opts.isColorSupported = p.is256ColorSupported
	for _, o := range objects {
		if err := r.showOne(ctx, opts, o); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) showOne(ctx context.Context, opts *ShowOptions, so *showObject) error {
	var o any
	var err error
	if o, err = r.odb.Object(ctx, so.oid); err != nil {
		if plumbing.IsNoSuchObject(err) {
			return r.showBlob(ctx, opts, so)
		}
		return catShowError(so.oid.String(), err)
	}
	switch a := o.(type) {
	case *object.Tree:
		return r.showTree(ctx, opts, so, a)
	case *object.Commit:
		return r.showCommit(ctx, opts, a)
	case *object.Tag:
		return r.showTag(ctx, opts, a)
	case *object.Fragments:
		return r.showFragments(ctx, opts, so, a)
	}
	return nil
}

func (r *Repository) showBlob(ctx context.Context, opts *ShowOptions, so *showObject) error {
	b, err := r.catMissingObject(ctx, &promiseObject{oid: so.oid})
	if err != nil {
		return err
	}
	defer b.Close()
	if opts.Limit < 0 {
		opts.Limit = b.Size
	}
	reader, charset, err := diferenco.NewUnifiedReaderEx(b.Contents, opts.Textconv)
	if err != nil {
		return err
	}
	if opts.isColorSupported && charset == diferenco.BINARY {
		if opts.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			opts.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}
		return processColor(reader, opts.w, opts.Limit)
	}
	_, err = io.Copy(opts.w, io.LimitReader(reader, opts.Limit))
	return err
}

func (r *Repository) showCommit(ctx context.Context, opts *ShowOptions, cc *object.Commit) error {
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
	if err := opts.w.LogOne(cc, rdb.M[cc.Hash]); err != nil {
		return err
	}
	if len(cc.Parents) == 2 {
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
	e := diferenco.NewUnifiedEncoder(opts.w)
	if opts.isColorSupported {
		e.SetColor(color.NewColorConfig())
	}
	_ = e.Encode(patch)
	return nil
}

func (r *Repository) showTag(ctx context.Context, opts *ShowOptions, tag *object.Tag) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if opts.isColorSupported {
		fmt.Fprintf(opts.w, "\x1b[33mtag %s\x1b[0m\n", tag.Name)
	} else {
		fmt.Fprintf(opts.w, "tag %s\n", tag.Name)
	}
	fmt.Fprintf(opts.w, "Tagger: %s <%s>\nDate:   %s\n\n%s\n", tag.Tagger.Name, tag.Tagger.Email, tag.Tagger.When.Format(time.RFC3339), tag.Content)
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
	return r.showCommit(ctx, opts, cc)
}

func (r *Repository) showTree(ctx context.Context, opts *ShowOptions, so *showObject, tree *object.Tree) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if opts.isColorSupported {
		fmt.Fprintf(opts.w, "\x1b[33mtree %s\x1b[0m\n\n", so.name)
	} else {
		fmt.Fprintf(opts.w, "tree %s\n\n", so.name)
	}
	for _, e := range tree.Entries {
		t := e.Type()
		if t == object.TreeObject {
			fmt.Fprintf(opts.w, "%s/\n", e.Name)
			continue
		}
		if t == object.FragmentsObject && opts.isColorSupported {
			fmt.Fprintf(opts.w, "\x1b[36m%s\x1b[0m\n", e.Name)
		}
		fmt.Fprintln(opts.w, e.Name)
	}
	return nil
}

func (r *Repository) showFragments(ctx context.Context, opts *ShowOptions, so *showObject, ff *object.Fragments) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if opts.isColorSupported {
		fmt.Fprintf(opts.w, "\x1b[33mfragments %s\x1b[0m\nraw:  %s\nsize: %d\n\n", so.oid, ff.Origin, ff.Size)
	} else {
		fmt.Fprintf(opts.w, "fragments %s\nraw:  %s\nsize: %d\n", so.oid, ff.Origin, ff.Size)
	}
	for _, e := range ff.Entries {
		fmt.Fprintf(opts.w, "%d\t%s %d\n", e.Index, e.Hash, e.Size)
	}
	return nil
}
