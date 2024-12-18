// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type CatOptions struct {
	Hash        string
	SizeMax     int64 // blob limit size
	Type        bool  // object type
	DisplaySize bool
	Textconv    bool
	FormatJSON  bool
	Verify      bool
}

func catShowError(oid string, err error) error {
	if err == nil {
		return nil
	}
	if plumbing.IsNoSuchObject(err) {
		fmt.Fprintf(os.Stderr, "cat-file: object '%s' not found\n", oid)
		return err
	}
	fmt.Fprintf(os.Stderr, "cat-file: resolve object '%s' error: %v\n", oid, err)
	return err
}

func (r *Repository) catMissingObject(ctx context.Context, oid plumbing.Hash) (*object.Blob, error) {
	b, err := r.odb.Blob(ctx, oid)
	if err == nil {
		return b, nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return nil, err
	}
	if !r.promisorEnabled() {
		return nil, err
	}
	if err = r.promiseMissingFetch(ctx, oid); err != nil {
		return nil, err
	}
	return r.odb.Blob(ctx, oid)
}

func objectSize(a object.Encoder) int {
	var b bytes.Buffer
	_ = a.Encode(&b)
	return b.Len()
}

func (r *Repository) showSize(ctx context.Context, oid plumbing.Hash) (err error) {
	var a any
	if a, err = r.odb.Object(ctx, oid); err == nil {
		if v, ok := a.(object.Encoder); ok {
			fmt.Fprintf(os.Stdout, "%d\n", objectSize(v))
			return
		}
		// unreachable
		return
	}
	if !plumbing.IsNoSuchObject(err) {
		fmt.Fprintf(os.Stderr, "cat-file: resolve object '%s' error: %v\n", oid, err)
		return
	}
	var b *object.Blob
	if b, err = r.catMissingObject(ctx, oid); err != nil {
		return catShowError(oid.String(), err)
	}
	defer b.Close()
	fmt.Fprintf(os.Stdout, "%d\n", b.Size)
	return nil
}

func (r *Repository) catBlob(ctx context.Context, w io.Writer, oid plumbing.Hash, n int64, textconv, verify bool) error {
	if oid == backend.BLANK_BLOB_HASH {
		return nil // empty blob, skip
	}
	if n <= 0 {
		n = math.MaxInt64
	}
	b, err := r.catMissingObject(ctx, oid)
	if err != nil {
		return err
	}
	defer b.Close()
	if verify {
		h := plumbing.NewHasher()
		if _, err := io.Copy(h, b.Contents); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, h.Sum())
		return nil
	}
	reader := b.Contents
	if textconv {
		if reader, err = diferenco.NewUnifiedReader(b.Contents); err != nil {
			return err
		}
	}
	if _, err = io.Copy(w, io.LimitReader(reader, n)); err != nil {
		return err
	}
	return nil
}

func (r *Repository) showType(ctx context.Context, oid plumbing.Hash) error {
	a, err := r.odb.Object(ctx, oid)
	if plumbing.IsNoSuchObject(err) {
		b, err := r.catMissingObject(ctx, oid)
		if err != nil {
			return catShowError(oid.String(), err)
		}
		defer b.Close()
		fmt.Fprintln(os.Stdout, "blob")
		return nil
	}
	if err != nil {
		return catShowError(oid.String(), err)
	}
	switch a.(type) {
	case *object.Commit:
		fmt.Fprintln(os.Stdout, "commit")
	case *object.Tag:
		fmt.Fprintln(os.Stdout, "tag")
	case *object.Tree:
		fmt.Fprintln(os.Stdout, "tree")
	case *object.Fragments:
		fmt.Fprintln(os.Stdout, "fragments")
	}
	return nil
}

func (r *Repository) catObject(ctx context.Context, opts *CatOptions, oid plumbing.Hash) error {
	if opts.DisplaySize {
		return r.showSize(ctx, oid)
	}
	if opts.Type {
		return r.showType(ctx, oid)
	}
	a, err := r.odb.Object(ctx, oid)
	if plumbing.IsNoSuchObject(err) {
		return catShowError(oid.String(), r.catBlob(ctx, os.Stdout, oid, opts.SizeMax, opts.Textconv, opts.Verify))
	}
	if err != nil {
		return catShowError(oid.String(), err)
	}
	if opts.Verify {
		if w, ok := a.(object.Encoder); ok {
			h := plumbing.NewHasher()
			_ = w.Encode(h)
			fmt.Fprintln(os.Stdout, h.Sum())
		}
		return nil
	}
	if opts.FormatJSON {
		return json.NewEncoder(os.Stdout).Encode(a)
	}
	if w, ok := a.(object.Printer); ok {
		_ = w.Pretty(os.Stdout)
	}
	return nil
}

func (r *Repository) catBranchOrTag(ctx context.Context, opts *CatOptions, branchOrTag string) (err error) {
	var oid plumbing.Hash
	if oid, err = r.Revision(ctx, branchOrTag); err != nil {
		return catShowError(branchOrTag, err)
	}
	r.DbgPrint("resolve object '%s'", oid)
	return r.catObject(ctx, opts, oid)
}

func (r *Repository) Cat(ctx context.Context, opts *CatOptions) error {
	k, v, ok := strings.Cut(opts.Hash, ":")
	if !ok {
		return r.catBranchOrTag(ctx, opts, k)
	}
	if len(k) == 0 {
		k = string(plumbing.HEAD) // default --> HEAD
	}
	oid, err := r.Revision(ctx, k)
	if err != nil {
		return catShowError(k, err)
	}
	var o any
	if o, err = r.odb.Object(ctx, oid); err != nil {
		return catShowError(oid.String(), err)
	}
	switch a := o.(type) {
	case *object.Tree:
		if len(v) == 0 {
			// self
			return r.catObject(ctx, opts, a.Hash)
		}
		e, err := a.FindEntry(ctx, v)
		if err != nil {
			return catShowError(v, err)
		}
		return r.catObject(ctx, opts, e.Hash)
	case *object.Commit:
		if len(v) == 0 {
			// root tree
			return r.catObject(ctx, opts, a.Tree)
		}
		root, err := r.odb.Tree(ctx, a.Tree)
		if err != nil {
			return catShowError(v, err)
		}
		e, err := root.FindEntry(ctx, v)
		if err != nil {
			return catShowError(v, err)
		}
		return r.catObject(ctx, opts, e.Hash)
	case *object.Tag:
		cc, err := r.odb.ParseRevExhaustive(ctx, a.Hash)
		if err != nil {
			return catShowError(v, err)
		}
		if len(v) == 0 {
			// root tree
			return r.catObject(ctx, opts, cc.Tree)
		}
		root, err := r.odb.Tree(ctx, cc.Tree)
		if err != nil {
			return catShowError(v, err)
		}
		e, err := root.FindEntry(ctx, v)
		if err != nil {
			return catShowError(v, err)
		}
		return r.catObject(ctx, opts, e.Hash)
	default:
	}
	return r.catObject(ctx, opts, oid)
}
