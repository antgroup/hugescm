// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

const (
	AnySize = -1
)

type FetchOptions struct {
	Target     plumbing.Hash
	DeepenFrom plumbing.Hash
	Have       plumbing.Hash
	Deepen     int
	Depth      int
	SizeLimit  int64
	SkipLarges bool
}

type FetchResult struct {
	*transport.Reference
	FETCH_HEAD plumbing.Hash
}

func (r *Repository) batch(ctx context.Context, t transport.Transport, oids []plumbing.Hash) error {
	if len(oids) == 0 {
		return nil
	}
	rc, err := t.BatchObjects(ctx, oids)
	if err != nil {
		return err
	}
	if err := r.odb.Unpack(rc, len(oids), r.quiet); err != nil {
		_ = rc.Close()
		if lastErr := rc.LastError(); lastErr != nil {
			return lastErr
		}
		return err
	}
	_ = rc.Close()
	return nil
}

func (r *Repository) fetch(ctx context.Context, t transport.Transport, opts *FetchOptions) error {
	metaOpts := &transport.MetadataOptions{
		DeepenFrom: opts.DeepenFrom,
		Have:       opts.Have,
		Deepen:     opts.Deepen,
		Depth:      opts.Depth,
	}
	if r.Core.Snapshot {
		metaOpts.SparseDirs = r.Core.SparseDirs
	}
	rc, err := t.FetchMetadata(ctx, opts.Target, metaOpts)
	if err != nil {
		return err
	}
	if err := r.odb.MetadataUnpack(rc, r.quiet); err != nil {
		_ = rc.Close()
		if lastErr := rc.LastError(); lastErr != nil {
			return lastErr
		}
		return err
	}
	_ = rc.Close()
	if err := r.odb.Reload(); err != nil {
		return err
	}
	return r.fetchObjects(ctx, t, opts.Target, opts.SizeLimit, opts.SkipLarges)
}

func (r *Repository) fetchAny(ctx context.Context, opts *FetchOptions) error {
	shallow, err := r.odb.DeepenFrom()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable resolve shallow commit %s error: %v\n", shallow, err)
		return err
	}
	opts.DeepenFrom = shallow
	t, err := r.newTransport(ctx, transport.DOWNLOAD)
	if err != nil {
		return err
	}
	if err := r.fetch(ctx, t, opts); err != nil {
		fmt.Fprintf(os.Stderr, "fetch metadata error: %v\n", err)
		return err
	}
	return nil
}

type DoFetchOptions struct {
	Name        string
	Unshallow   bool
	Limit       int64
	Tag         bool
	Force       bool
	FetchAlways bool
	SkipLarges  bool
}

func (opts *DoFetchOptions) ReferenceName() plumbing.ReferenceName {
	if len(opts.Name) == 0 || opts.Name == string(plumbing.HEAD) {
		return plumbing.HEAD
	}
	if strings.HasPrefix(opts.Name, plumbing.ReferencePrefix) {
		return plumbing.ReferenceName(opts.Name)
	}
	if opts.Tag {
		return plumbing.NewTagReferenceName(opts.Name)
	}
	return plumbing.NewBranchReferenceName(opts.Name)
}

func (r *Repository) updateTagReference(ctx context.Context, refname plumbing.ReferenceName, target plumbing.Hash, force bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	old, err := r.Reference(refname)
	if err != nil && err != plumbing.ErrReferenceNotFound {
		die_error("resolve %s: %v", refname, err)
		return err
	}
	tagName := refname.TagName()
	if old != nil && old.Hash() != target {
		if !force {
			fmt.Fprintf(os.Stderr, " ! %s %s -> %s (%s)\n", W("[rejected]"), tagName, tagName, W("would clobber existing tag"))
			return ErrAborting
		}
	}
	if err := r.ReferenceUpdate(plumbing.NewHashReference(refname, target), old); err != nil {
		die_error("update-ref '%s' error: %v", refname, err)
		return err
	}

	if old == nil {
		fmt.Fprintf(os.Stderr, " * %s %s -> %s\n", W("[new tag]"), tagName, tagName)
		return nil
	}
	fmt.Fprintf(os.Stderr, " t %s %s -> %s\n", W("[tag update]"), tagName, tagName)
	return nil
}

func (r *Repository) resolveRef(refname plumbing.ReferenceName) (*plumbing.Reference, plumbing.ReferenceName, error) {
	if refname == plumbing.HEAD {
		current, err := r.Current()
		if err != nil {
			die_error("resolve HEAD: %v", err)
			return nil, "", err
		}
		return current, current.Name(), nil
	}
	current, err := r.Reference(refname)
	if err != nil && err != plumbing.ErrReferenceNotFound {
		die_error("resolve %s: %v", refname, err)
		return nil, "", err
	}
	return current, refname, nil
}

var (
	ErrHaveCommits = errors.New("have commits")
)

func (r *Repository) prepareFetch(ctx context.Context, current *plumbing.Reference, want plumbing.Hash, opts *DoFetchOptions) (*FetchOptions, error) {
	o := &FetchOptions{
		Target:     want,
		Deepen:     transport.Shallow,
		Depth:      transport.AnyDepth,
		SizeLimit:  opts.Limit,
		SkipLarges: opts.SkipLarges,
	}
	var commits []*object.Commit
	var err error
	if current != nil {
		// Full history check
		if commits, err = r.revList(ctx, current.Hash(), nil, nil); err != nil {
			die_error("log commits error: %v", err)
			return nil, err
		}
	}
	var deepenFrom plumbing.Hash
	if !opts.Unshallow {
		if deepenFrom, err = r.odb.DeepenFrom(); err != nil && !os.IsNotExist(err) {
			die_error("resolve shallow: %v", err)
			return nil, err
		}
	}
	// Fetch new reference
	if len(commits) == 0 {
		if opts.Unshallow || o.DeepenFrom.IsZero() {
			// unshallow
			// shallow file not found equal unshallow
			o.Deepen = transport.AnyDeepen
			return o, nil
		}
		// shallow: say deepen-from
		o.DeepenFrom = deepenFrom
		return o, nil
	}
	basePoint := commits[len(commits)-1]
	if len(basePoint.Parents) == 0 {
		// Full history checkout
		o.Have = current.Hash()
		return o, nil
	}
	if opts.Unshallow {
		// Incomplete checkout, --unshallow needs to get all commits.
		o.Deepen = -1
		return o, nil
	}
	// unshallow repo ,fetch all
	if o.DeepenFrom.IsZero() {
		o.Deepen = -1
	}
	// Incomplete history, keep shallow strategy
	o.DeepenFrom = deepenFrom
	o.Have = current.Hash()
	return o, nil
}

// DoFetch: Fetch reference or commit
func (r *Repository) DoFetch(ctx context.Context, opts *DoFetchOptions) (*FetchResult, error) {
	current, refname, err := r.resolveRef(opts.ReferenceName())
	if err != nil {
		return nil, err
	}
	t, err := r.newTransport(ctx, transport.DOWNLOAD)
	if err != nil {
		return nil, err
	}
	var want plumbing.Hash
	ref, err := t.FetchReference(ctx, refname)
	switch {
	case err == transport.ErrReferenceNotExist:
		if !plumbing.ValidateHashHex(string(opts.Name)) {
			die_error("couldn't find remote ref %s", opts.Name)
			return nil, err
		}
		refname = plumbing.ReferenceName(opts.Name)
		want = plumbing.NewHash(string(opts.Name))
	case err != nil:
		die("fetch remote reference '%s' error: %v", opts.Name, err)
		return nil, err
	default:
		want = plumbing.NewHash(ref.Hash)
	}

	o, err := r.prepareFetch(ctx, current, want, opts)
	if err != nil {
		return nil, err
	}

	// Unless the user modifies the sparse checkout configuration, we do not have to repeat the fetch metadata.
	// Once the user modifies the relevant configuration, we need to use the forced fetch operation.
	if r.odb.Exists(o.Target, true) && o.Deepen != -1 {
		if opts.FetchAlways {
			if err := r.fetchObjects(ctx, t, o.Target, o.SizeLimit, o.SkipLarges); err != nil {
				die_error("fetch missing object error: %v", err)
				return nil, err
			}
		}
		return &FetchResult{Reference: ref, FETCH_HEAD: o.Target}, nil
	}

	if err := r.fetch(ctx, t, o); err != nil {
		die_error("fetch target '%s' error: %v", o.Target, err)
		return nil, err
	}
	if err := r.odb.SpecReferenceUpdate(odb.FETCH_HEAD, o.Target); err != nil {
		die_error("update FETCH_HEAD: %v", err)
		return nil, err
	}
	if opts.Unshallow {
		_ = r.odb.Unshallow()
	}
	fmt.Fprintf(os.Stderr, "From: %s\n", r.cleanedRemote())
	switch {
	case refname.IsBranch():
		originBranch := plumbing.NewRemoteReferenceName(plumbing.Origin, refname.BranchName())
		if err := r.ReferenceUpdate(plumbing.NewHashReference(originBranch, o.Target), nil); err != nil {
			die_error("update-ref '%s' error: %v", originBranch, err)
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "* branch %s -> FETCH_HEAD\n", refname.BranchName())
	case refname.IsTag():
		if err := r.updateTagReference(ctx, refname, o.Target, opts.Force); err != nil {
			return nil, nil
		}
	default:
		fmt.Fprintf(os.Stderr, "* %s -> FETCH_HEAD\n", refname)
	}
	return &FetchResult{Reference: ref, FETCH_HEAD: o.Target}, nil
}
