// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

const (
	MAX_SHOW_BINARY_BLOB = 10<<20 - 8
)

type CatOptions struct {
	Object    string
	Limit     int64 // blob limit size
	Type      bool  // object type
	PrintSize bool
	PrintJSON bool
	Verify    bool
	Textconv  bool
	Direct    bool
	Output    string
}

func (opts *CatOptions) Println(a ...any) error {
	fd, _, err := opts.NewFD()
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fmt.Fprintln(fd, a...)
	return err
}

func (opts *CatOptions) NewFD() (io.WriteCloser, term.Level, error) {
	if len(opts.Output) == 0 {
		return &NopWriteCloser{Writer: os.Stdout}, term.StdoutLevel, nil
	}
	fd, err := os.Create(opts.Output)
	return fd, term.LevelNone, err
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

func (r *Repository) fetchMissingBlob(ctx context.Context, o *promiseObject) error {
	if r.odb.Exists(o.oid, false) {
		return nil
	}
	if !r.promisorEnabled() {
		return plumbing.NoSuchObject(o.oid)
	}
	return r.promiseMissingFetch(ctx, o)
}

func (r *Repository) catMissingObject(ctx context.Context, o *promiseObject) (*object.Blob, error) {
	if err := r.fetchMissingBlob(ctx, o); err != nil {
		return nil, err
	}
	return r.odb.Blob(ctx, o.oid)
}

func objectSize(a object.Encoder) int {
	var b bytes.Buffer
	_ = a.Encode(&b)
	return b.Len()
}

func (r *Repository) printSize(ctx context.Context, opts *CatOptions, o *promiseObject) error {
	var a any
	var err error
	if a, err = r.odb.Object(ctx, o.oid); err == nil {
		if v, ok := a.(object.Encoder); !ok {
			return opts.Println(objectSize(v))
		}
		// unreachable
		return nil
	}
	if !plumbing.IsNoSuchObject(err) {
		fmt.Fprintf(os.Stderr, "cat-file: resolve object '%s' error: %v\n", o.oid, err)
		return err
	}
	var b *object.Blob
	if b, err = r.catMissingObject(ctx, o); err != nil {
		return catShowError(o.oid.String(), err)
	}
	defer b.Close()
	return opts.Println(b.Size)
}

func (r *Repository) printType(ctx context.Context, opts *CatOptions, o *promiseObject) error {
	a, err := r.odb.Object(ctx, o.oid)
	if plumbing.IsNoSuchObject(err) {
		if err := r.fetchMissingBlob(ctx, o); err == nil {
			return opts.Println("blob")
		}
	}
	if err != nil {
		return catShowError(o.oid.String(), err)
	}
	switch a.(type) {
	case *object.Commit:
		return opts.Println("commit")
	case *object.Tag:
		return opts.Println("tag")
	case *object.Tree:
		return opts.Println("tree")
	case *object.Fragments:
		return opts.Println("fragments")
	}
	return nil
}

const (
	binaryTruncated = "*** Binary truncated ***"
)

func (r *Repository) catBlob(ctx context.Context, opts *CatOptions, o *promiseObject) error {
	if o.oid == backend.BLANK_BLOB_HASH {
		return nil // empty blob, skip
	}
	b, err := r.catMissingObject(ctx, o)
	if err != nil {
		return err
	}
	defer b.Close()
	fd, colorMode, err := opts.NewFD()
	if err != nil {
		return err
	}
	if opts.Verify {
		h := plumbing.NewHasher()
		if _, err := io.Copy(h, b.Contents); err != nil {
			return err
		}
		fmt.Fprintln(fd, h.Sum())
		return nil
	}
	reader, charset, err := diferenco.NewUnifiedReaderEx(b.Contents, opts.Textconv)
	if err != nil {
		return err
	}
	if opts.Limit < 0 {
		opts.Limit = b.Size
	}
	if colorMode != term.LevelNone && charset == diferenco.BINARY {
		if opts.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			opts.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}
		return processColor(reader, fd, opts.Limit, colorMode)
	}
	if _, err = io.Copy(fd, io.LimitReader(reader, opts.Limit)); err != nil {
		return err
	}
	return nil
}

func (r *Repository) catFragments(ctx context.Context, opts *CatOptions, ff *object.Fragments) error {
	fd, colorMode, err := opts.NewFD()
	if err != nil {
		return err
	}
	defer fd.Close()
	objects := make([]*object.Blob, 0, len(ff.Entries))
	defer func() {
		for _, o := range objects {
			_ = o.Close()
		}
	}()
	readers := make([]io.Reader, 0, len(ff.Entries))
	for _, e := range ff.Entries {
		o, err := r.catMissingObject(ctx, &promiseObject{oid: e.Hash, size: int64(e.Size)})
		if err != nil {
			return err
		}
		objects = append(objects, o)
		readers = append(readers, o.Contents)
	}
	if opts.Limit < 0 {
		opts.Limit = int64(ff.Size)
	}
	// fragments ignore --textconv
	reader := io.MultiReader(readers...)
	if colorMode != term.LevelNone {
		if opts.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			opts.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}
		return processColor(reader, fd, opts.Limit, colorMode)
	}
	if _, err = io.Copy(fd, io.LimitReader(reader, opts.Limit)); err != nil {
		return err
	}
	return nil
}

func (r *Repository) catObject(ctx context.Context, opts *CatOptions, o *promiseObject) error {
	if opts.PrintSize {
		return r.printSize(ctx, opts, o)
	}
	if opts.Type {
		return r.printType(ctx, opts, o)
	}
	a, err := r.odb.Object(ctx, o.oid)
	if plumbing.IsNoSuchObject(err) {
		return catShowError(o.oid.String(), r.catBlob(ctx, opts, o))
	}
	if err != nil {
		return catShowError(o.oid.String(), err)
	}
	if opts.Verify {
		if w, ok := a.(object.Encoder); ok {
			h := plumbing.NewHasher()
			_ = w.Encode(h)
			fmt.Fprintln(os.Stdout, h.Sum())
		}
		return nil
	}
	if opts.PrintJSON {
		fd, _, err := opts.NewFD()
		if err != nil {
			return err
		}
		defer fd.Close()
		return json.NewEncoder(fd).Encode(a)
	}
	if opts.Direct {
		// only fragments support direct read
		if ff, ok := a.(*object.Fragments); ok {
			return r.catFragments(ctx, opts, ff)
		}
	}
	if w, ok := a.(object.Printer); ok {
		fd, _, err := opts.NewFD()
		if err != nil {
			return err
		}
		defer fd.Close()
		_ = w.Pretty(fd)
	}
	return nil
}

func (r *Repository) catBranchOrTag(ctx context.Context, opts *CatOptions, branchOrTag string) (err error) {
	var oid plumbing.Hash
	if oid, err = r.Revision(ctx, branchOrTag); err != nil {
		return catShowError(branchOrTag, err)
	}
	r.DbgPrint("resolve object '%s'", oid)
	return r.catObject(ctx, opts, &promiseObject{oid: oid})
}

func (r *Repository) Cat(ctx context.Context, opts *CatOptions) error {
	k, v, ok := strings.Cut(opts.Object, ":")
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
			return r.catObject(ctx, opts, &promiseObject{oid: a.Hash})
		}
		e, err := a.FindEntry(ctx, v)
		if err != nil {
			return catShowError(v, err)
		}
		return r.catObject(ctx, opts, &promiseObject{oid: e.Hash, size: e.Size})
	case *object.Commit:
		if len(v) == 0 {
			// root tree
			return r.catObject(ctx, opts, &promiseObject{oid: a.Tree})
		}
		root, err := r.odb.Tree(ctx, a.Tree)
		if err != nil {
			return catShowError(v, err)
		}
		e, err := root.FindEntry(ctx, v)
		if err != nil {
			return catShowError(v, err)
		}
		return r.catObject(ctx, opts, &promiseObject{oid: e.Hash, size: e.Size})
	case *object.Tag:
		cc, err := r.odb.ParseRevExhaustive(ctx, a.Hash)
		if err != nil {
			return catShowError(v, err)
		}
		if len(v) == 0 {
			// root tree
			return r.catObject(ctx, opts, &promiseObject{oid: cc.Tree})
		}
		root, err := r.odb.Tree(ctx, cc.Tree)
		if err != nil {
			return catShowError(v, err)
		}
		e, err := root.FindEntry(ctx, v)
		if err != nil {
			return catShowError(v, err)
		}
		return r.catObject(ctx, opts, &promiseObject{oid: e.Hash, size: e.Size})
	default:
	}
	return r.catObject(ctx, opts, &promiseObject{oid: oid})
}
