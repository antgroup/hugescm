package zeta

import (
	"context"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type ShowOptions struct {
	Objects   []string
	Textconv  bool
	Algorithm diferenco.Algorithm
	Limit     int64
	w         io.Writer
	useColor  bool
}

type showObject struct {
	name string
	oid  plumbing.Hash
}

func (r *Repository) parseObject(ctx context.Context, name string) (plumbing.Hash, error) {
	prefix, p, ok := strings.Cut(name, ":")
	oid, err := r.Revision(ctx, prefix)
	if !ok || err != nil {
		return oid, err
	}
	e, err := r.parseEntry(ctx, oid, p)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return e.Hash, nil
}

func (r *Repository) showFetch(ctx context.Context, oid plumbing.Hash) error {
	if r.odb.Exists(oid, false) || r.odb.Exists(oid, true) {
		return nil
	}
	if !r.promisorEnabled() {
		return plumbing.NoSuchObject(oid)
	}
	return r.promiseMissingFetch(ctx, oid)
}

func (r *Repository) Show(ctx context.Context, opts *ShowOptions) error {
	objects := make([]*showObject, 0, len(opts.Objects))
	for _, o := range opts.Objects {
		oid, err := r.parseObject(ctx, o)
		if err != nil {
			die_error("parse object %s error: %v", o, err)
			return err
		}
		if err := r.showFetch(ctx, oid); err != nil {
			die_error("search object %s error: %v", oid, err)
			return err
		}
		objects = append(objects, &showObject{name: o, oid: oid})
	}
	p := NewPrinter(ctx)
	defer p.Close()
	opts.w = p
	opts.useColor = p.useColor
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
	case *object.Commit:
		return r.showCommit(ctx, opts, so, a)
	case *object.Tag:
		return r.showTag(ctx, opts, so, a)
	case *object.Fragments:
	}
	return nil
}

func (r *Repository) showBlob(ctx context.Context, opts *ShowOptions, so *showObject) error {
	b, err := r.catMissingObject(ctx, so.oid)
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
	if opts.useColor && charset == diferenco.BINARY {
		if opts.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			opts.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}
		return processColor(reader, opts.w, opts.Limit)
	}
	_, err = io.Copy(opts.w, io.LimitReader(reader, opts.Limit))
	return err
}

func (r *Repository) showCommit(ctx context.Context, opts *ShowOptions, so *showObject, t *object.Commit) error {
	return nil
}

func (r *Repository) showTag(ctx context.Context, opts *ShowOptions, so *showObject, t *object.Tag) error {

	return nil
}
