// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

var (
	ErrUnrelatedHistories = errors.New("merge: refusing to merge unrelated histories")
	ErrHasConflicts       = errors.New("merge: there are conflicting files")
	ErrNotAncestor        = errors.New("merge-base: not ancestor")
)

type MergeTreeOptions struct {
	Branch1, Branch2, MergeBase                          string
	AllowUnrelatedHistories, Z, NameOnly, Textconv, JSON bool
}

func (r *Repository) readMissingText(ctx context.Context, oid plumbing.Hash, textconv bool) (string, string, error) {
	br, err := r.odb.Blob(ctx, oid)
	switch {
	case err == nil:
		// nothing
	case plumbing.IsNoSuchObject(err):
		if err = r.promiseMissingFetch(ctx, &promiseObject{oid: oid}); err != nil {
			return "", "", err
		}
		if br, err = r.odb.Blob(ctx, oid); err != nil {
			return "", "", err
		}
	default:
		return "", "", err
	}
	defer br.Close() // nolint
	return diferenco.ReadUnifiedText(br.Contents, br.Size, textconv)
}

func (o *MergeTreeOptions) formatJson(result *odb.MergeResult) {
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		die("format to json error: %v", err)
	}
}

func (o *MergeTreeOptions) format(result *odb.MergeResult) {
	if o.JSON {
		o.formatJson(result)
		return
	}
	NewLine := byte('\n')
	if o.Z {
		NewLine = '\x00'
	}
	fmt.Fprintf(os.Stdout, "%s%c", result.NewTree, NewLine)
	if o.NameOnly {
		for _, e := range result.Conflicts {
			if e.Ancestor.Path != "" {
				fmt.Fprintf(os.Stdout, "%s%c", e.Ancestor.Path, NewLine)
				continue
			}
			if e.Our.Path != "" {
				fmt.Fprintf(os.Stdout, "%s%c", e.Our.Path, NewLine)
				continue
			}
			if e.Their.Path != "" {
				fmt.Fprintf(os.Stdout, "%s%c", e.Their.Path, NewLine)
				continue
			}
		}
	} else {
		for _, e := range result.Conflicts {
			if e.Ancestor.Path != "" {
				fmt.Fprintf(os.Stdout, "%s %s 1 %s%c", e.Ancestor.Mode, e.Ancestor.Hash, e.Ancestor.Path, NewLine)
			}
			if e.Our.Path != "" {
				fmt.Fprintf(os.Stdout, "%s %s 2 %s%c", e.Our.Mode, e.Our.Hash, e.Our.Path, NewLine)
			}
			if e.Their.Path != "" {
				fmt.Fprintf(os.Stdout, "%s %s 3 %s%c", e.Their.Mode, e.Their.Hash, e.Their.Path, NewLine)
			}
		}
	}
	if len(result.Messages) == 0 {
		return
	}
	fmt.Fprintf(os.Stdout, "%c", NewLine)
	for _, m := range result.Messages {
		fmt.Fprintf(os.Stdout, "%s%c", m, NewLine)
	}
}

type mergeTreeResult struct {
	*odb.MergeResult
	bases []plumbing.Hash
}

func (r *Repository) resolveAncestorTree0(ctx context.Context, into, from *object.Commit, mergeDriver odb.MergeDriver, allowUnrelatedHistories, textconv bool) (*object.Tree, error) {
	bases, err := into.MergeBase(ctx, from)
	if err != nil {
		die_error("merge-base '%s-%s': %v", from.Hash, into.Hash, err)
		return nil, err
	}
	var o *object.Tree
	switch len(bases) {
	case 0:
		if !allowUnrelatedHistories {
			trace.DbgPrint("merge: merge from %s to %s refusing to merge unrelated histories", from.Hash, into.Hash)
			fmt.Fprintf(os.Stderr, "merge: %s\n", W("refusing to merge unrelated histories"))
			return nil, ErrUnrelatedHistories
		}
		return r.odb.EmptyTree(), nil
	case 1:
		if o, err = bases[0].Root(ctx); err != nil {
			die_error("resolve bases tree: %v", err)
			return nil, err
		}
	default:
		if o, err = r.resolveAncestorTree0(ctx, bases[0], bases[1], mergeDriver, allowUnrelatedHistories, textconv); err != nil {
			return nil, err
		}
	}
	a, err := into.Root(ctx)
	if err != nil {
		return nil, err
	}
	b, err := from.Root(ctx)
	if err != nil {
		return nil, err
	}
	result, err := r.odb.MergeTree(ctx, o, a, b, &odb.MergeOptions{
		Branch1:       "Temporary merge branch 1",
		Branch2:       "Temporary merge branch 2",
		DetectRenames: true,
		Textconv:      textconv,
		MergeDriver:   mergeDriver,
		TextGetter:    r.readMissingText,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Conflicts) != 0 {
		return nil, result
	}
	trace.DbgPrint("make new merge-tree: %s", result.NewTree)
	return r.odb.Tree(ctx, result.NewTree)
}

func (r *Repository) resolveAncestorTree(ctx context.Context, into, from, base *object.Commit, mergeDriver odb.MergeDriver, allowUnrelatedHistories, textconv bool) ([]plumbing.Hash, *object.Tree, error) {
	if base != nil {
		o, err := base.Root(ctx)
		if err != nil {
			die_error("resolve bases tree: %v", err)
			return nil, nil, err
		}
		return []plumbing.Hash{base.Hash}, o, nil
	}
	bases, err := into.MergeBase(ctx, from)
	if err != nil {
		die_error("merge-base '%s-%s': %v", from.Hash, into.Hash, err)
		return nil, nil, err
	}
	if len(bases) == 0 {
		if !allowUnrelatedHistories {
			trace.DbgPrint("merge: merge from %s to %s refusing to merge unrelated histories", from.Hash, into.Hash)
			fmt.Fprintf(os.Stderr, "merge: %s\n", W("refusing to merge unrelated histories"))
			return nil, nil, ErrUnrelatedHistories
		}
		return nil, r.odb.EmptyTree(), nil
	}
	baseOIDs := make([]plumbing.Hash, 0, 2)
	for _, c := range bases {
		baseOIDs = append(baseOIDs, c.Hash)
	}
	if len(bases) == 1 {
		o, err := bases[0].Root(ctx)
		if err != nil {
			die_error("resolve bases tree: %v", err)
			return nil, nil, err
		}
		return baseOIDs, o, nil
	}
	o, err := r.resolveAncestorTree0(ctx, bases[0], bases[1], mergeDriver, allowUnrelatedHistories, textconv)
	if err != nil {
		return nil, nil, err
	}
	return baseOIDs, o, nil
}

func (r *Repository) mergeTree(ctx context.Context, into, from, base *object.Commit, branch1, branch2 string, allowUnrelatedHistories, textconv bool) (*mergeTreeResult, error) {
	mergeDriver := r.resolveMergeDriver()
	bases, o, err := r.resolveAncestorTree(ctx, into, from, base, mergeDriver, allowUnrelatedHistories, textconv)
	if err != nil {
		return nil, err
	}
	trace.DbgPrint("merge from %s to %s base: %s", from.Hash, into.Hash, bases)
	a, err := into.Root(ctx)
	if err != nil {
		die_error("read tree '%s:' %v", from.Hash, err)
		return nil, err
	}
	b, err := from.Root(ctx)
	if err != nil {
		die_error("read tree '%s:' %v", into.Hash, err)
		return nil, err
	}
	if a.Equal(b) {
		return &mergeTreeResult{MergeResult: &odb.MergeResult{NewTree: a.Hash}, bases: bases}, nil
	}
	result, err := r.odb.MergeTree(ctx, o, a, b, &odb.MergeOptions{
		Branch1:       branch1,
		Branch2:       branch2,
		DetectRenames: true,
		Textconv:      textconv,
		MergeDriver:   mergeDriver,
		TextGetter:    r.readMissingText,
	})
	if err != nil {
		die_error("merge-tree: %v", err)
		return nil, err
	}
	return &mergeTreeResult{MergeResult: result, bases: bases}, nil
}

func (r *Repository) MergeTree(ctx context.Context, opts *MergeTreeOptions) error {
	c1, err := r.parseRevExhaustive(ctx, opts.Branch1)
	if err != nil {
		die_error("parse-rev '%s': %v", opts.Branch1, err)
		return err
	}
	c2, err := r.parseRevExhaustive(ctx, opts.Branch2)
	if err != nil {
		die_error("parse-rev '%s': %v", opts.Branch1, err)
		return err
	}
	var base *object.Commit
	if len(opts.MergeBase) != 0 {
		if base, err = r.parseRevExhaustive(ctx, opts.MergeBase); err != nil {
			die_error("parse-rev '%s': %v", opts.Branch1, err)
			return err
		}
	}
	result, err := r.mergeTree(ctx, c1, c2, base, opts.Branch1, opts.Branch2, opts.AllowUnrelatedHistories, opts.Textconv)
	if err != nil {
		if mr, ok := err.(*odb.MergeResult); ok {
			opts.format(mr)
			return ErrHasConflicts
		}
		return err
	}
	opts.format(result.MergeResult)
	if len(result.Conflicts) != 0 {
		return ErrHasConflicts
	}
	return nil
}
