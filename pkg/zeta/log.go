// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/modules/zeta/refs"
)

type ReferenceLite struct {
	Name      plumbing.ReferenceName
	ShortName plumbing.ReferenceName
	Target    plumbing.ReferenceName
}

func (r *ReferenceLite) colorFormat() string {
	if r.Name.IsBranch() {
		return "\x1b[1;32m" + string(r.ShortName) + "\x1b[0m"
	}
	if r.Name.IsRemote() {
		return "\x1b[1;31m" + string(r.ShortName) + "\x1b[0m"
	}
	if r.Name.IsTag() {
		return "\x1b[1;33mtag: " + string(r.ShortName) + "\x1b[0m"
	}
	return "\x1b[1;36m" + string(r.ShortName) + "\x1b[0m"
}

type ReferencesEx struct {
	*refs.DB
	M map[plumbing.Hash][]*ReferenceLite
}

func (r *Repository) ReferencesEx(ctx context.Context) (*ReferencesEx, error) {
	rdb, err := r.References()
	if err != nil {
		return nil, err
	}
	m := make(map[plumbing.Hash][]*ReferenceLite)
	hr := rdb.HEAD()
	switch hr.Type() {
	case plumbing.HashReference:
		m[hr.Hash()] = []*ReferenceLite{
			{
				Name:      plumbing.HEAD,
				ShortName: plumbing.HEAD,
			},
		}
	case plumbing.SymbolicReference:
		if target, err := rdb.Resolve(hr.Target()); err == nil {
			m[target.Hash()] = []*ReferenceLite{
				{
					Name:      plumbing.HEAD,
					ShortName: plumbing.HEAD,
					Target:    plumbing.ReferenceName(rdb.ShortName(target.Name(), true)),
				},
			}
		}
	}

	for _, ref := range rdb.References() {
		oid := ref.Hash()
		if ref.Name().IsTag() {
			if t, err := r.odb.Tag(ctx, oid); err == nil {
				oid = t.Object
			}
		}
		rr := &ReferenceLite{
			Name:      ref.Name(),
			ShortName: plumbing.ReferenceName(rdb.ShortName(ref.Name(), true)),
		}
		if refs, ok := m[oid]; ok {
			newRefs := make([]*ReferenceLite, 0, len(refs)+1)
			newRefs = append(newRefs, refs...)
			newRefs = append(newRefs, rr)
			m[oid] = newRefs
			continue
		}
		m[oid] = []*ReferenceLite{rr}
	}
	return &ReferencesEx{DB: rdb, M: m}, nil
}

func (r *Repository) logPrint(ctx context.Context, opts *LogOptions, ignore []plumbing.Hash, formatJSON bool) error {
	if formatJSON {
		iter, err := r.newCommitIter(ctx, opts, ignore)
		if err != nil {
			return err
		}
		defer iter.Close()
		commits := make([]*object.Commit, 0, 20)
		var cc *object.Commit
		for {
			cc, err = iter.Next(ctx)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			commits = append(commits, cc)
		}
		if opts.Reverse {
			slices.Reverse(commits)
		}
		return json.NewEncoder(os.Stdout).Encode(commits)
	}
	rdb, err := r.ReferencesEx(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve references error: %v\n", err)
		return err
	}
	iter, err := r.newCommitIter(ctx, opts, ignore)
	if err != nil {
		return err
	}
	defer iter.Close()
	if opts.Reverse {
		commits := make([]*object.Commit, 0, 100)
		for {
			cc, err := iter.Next(ctx)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			commits = append(commits, cc)
		}
		slices.Reverse(commits)
		p := NewPrinter(ctx)
		for _, cc := range commits {
			if err := p.LogOne(cc, rdb.M[cc.Hash]); err != nil {
				_ = p.Close()
				return err
			}
		}
		_ = p.Close()
		return nil
	}
	p := NewPrinter(ctx)
	for {
		cc, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = p.Close()
			return err
		}
		if err := p.LogOne(cc, rdb.M[cc.Hash]); err != nil {
			_ = p.Close()
			return err
		}
	}
	_ = p.Close()
	return nil
}

type commitsGroup struct {
	commits []*object.Commit
	seen    map[plumbing.Hash]bool
}

func (r *Repository) revList0(ctx context.Context, want plumbing.Hash, ignore []plumbing.Hash, order LogOrder, paths []string, cg *commitsGroup) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	iter, err := r.newCommitIter(ctx, &LogOptions{
		Order:      order,
		From:       want,
		PathFilter: newLogPathFilter(paths),
	}, ignore)
	if err != nil {
		return err
	}
	defer iter.Close()
	for {
		cc, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if cg.seen[cc.Hash] {
			continue
		}
		cg.commits = append(cg.commits, cc)
	}
	return nil
}

func (r *Repository) revList(ctx context.Context, want plumbing.Hash, ignore []plumbing.Hash, paths []string) ([]*object.Commit, error) {
	cg := &commitsGroup{
		commits: make([]*object.Commit, 0, 100),
		seen:    make(map[plumbing.Hash]bool),
	}
	if err := r.revList0(ctx, want, ignore, LogOrderBFS, paths, cg); err != nil {
		return nil, err
	}
	return cg.commits, nil
}

// logFromMergeBase: a...b  a from merge-base and b from merge-base changes
func (r *Repository) logFromMergeBase(ctx context.Context, a, b plumbing.Hash, opts *LogCommandOptions) error {
	ac, err := r.odb.ParseRevExhaustive(ctx, a)
	if err != nil {
		die("open %s: %v", a, err)
		return err
	}
	bc, err := r.odb.ParseRevExhaustive(ctx, b)
	if err != nil {
		die("open %s: %v", b, err)
		return err
	}
	bases, err := ac.MergeBase(ctx, bc)
	if err != nil {
		die("open merge-base %s...%s: %v", a, b, err)
		return err
	}
	ignore := make([]plumbing.Hash, 0, 2)
	for _, b := range bases {
		ignore = append(ignore, b.Hash)
	}
	cg := &commitsGroup{
		commits: make([]*object.Commit, 0, 100),
		seen:    make(map[plumbing.Hash]bool),
	}
	if err := r.revList0(ctx, ac.Hash, ignore, opts.Order, opts.Paths, cg); err != nil {
		fmt.Fprintf(os.Stderr, "log commit '%s' error: %v\n", a, err)
		return err
	}
	if err := r.revList0(ctx, bc.Hash, ignore, opts.Order, opts.Paths, cg); err != nil {
		fmt.Fprintf(os.Stderr, "log commit '%s' error: %v\n", b, err)
		return err
	}
	if opts.Reverse {
		slices.Reverse(cg.commits) // reverse
	}
	if opts.FormatJSON {
		return json.NewEncoder(os.Stdout).Encode(cg.commits)
	}
	rdb, err := r.ReferencesEx(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve references error: %v\n", err)
		return err
	}
	p := NewPrinter(ctx)
	for _, cc := range cg.commits {
		if err := p.LogOne(cc, rdb.M[cc.Hash]); err != nil {
			_ = p.Close()
			return err
		}
	}
	_ = p.Close()
	return nil
}

// logRevFromTo: a..b shows the change from a to b.
// if a not b ancestor, show both merge-base to b.
func (r *Repository) logRevFromTo(ctx context.Context, from, to plumbing.Hash, opts *LogCommandOptions) error {
	oldRev, err := r.odb.ParseRevExhaustive(ctx, from)
	if err != nil {
		die_error("open commit '%s' error: %v", from, err)
		return err
	}
	newRev, err := r.odb.ParseRevExhaustive(ctx, to)
	if err != nil {
		die_error("open commit '%s' error: %v", to, err)
		return err
	}
	// Equal
	if newRev.Hash == oldRev.Hash {
		return nil
	}
	bases, err := newRev.MergeBase(ctx, oldRev)
	if err != nil {
		die_error("resolve merge-base error: %v", err)
		return err
	}
	if len(bases) == 0 {
		return r.logPrint(ctx, &LogOptions{
			From:       newRev.Hash,
			Order:      opts.Order,
			PathFilter: newLogPathFilter(opts.Paths),
			Reverse:    opts.Reverse,
		}, nil, opts.FormatJSON)
	}
	ignore := make([]plumbing.Hash, 0, 2)
	for _, cc := range bases {
		if cc.Hash == newRev.Hash {
			// newRev is old rev parents
			return nil
		}
		ignore = append(ignore, cc.Hash)
	}
	return r.logPrint(ctx, &LogOptions{
		From:       newRev.Hash,
		Order:      opts.Order,
		PathFilter: newLogPathFilter(opts.Paths),
		Reverse:    opts.Reverse,
	}, ignore, opts.FormatJSON)
}

func (r *Repository) Log(ctx context.Context, opts *LogCommandOptions) error {
	if aRev, bRev, ok := strings.Cut(opts.Revision, "..."); ok {
		a, err := r.Revision(ctx, aRev)
		if err != nil {
			dieln(err)
			return err
		}
		b, err := r.Revision(ctx, bRev)
		if err != nil {
			dieln(err)
			return err
		}
		return r.logFromMergeBase(ctx, a, b, opts)
	}
	if fromRev, toRev, ok := strings.Cut(opts.Revision, ".."); ok {
		from, err := r.Revision(ctx, fromRev)
		if err != nil {
			dieln(err)
			return err
		}
		to, err := r.Revision(ctx, toRev)
		if err != nil {
			dieln(err)
			return err
		}
		return r.logRevFromTo(ctx, from, to, opts)
	}
	if opts.Revision == "" {
		opts.Revision = "HEAD"
	}
	rev, err := r.Revision(ctx, opts.Revision)
	if err != nil {
		dieln(err)
		return err
	}
	return r.logPrint(ctx, &LogOptions{
		From:       rev,
		Order:      opts.Order,
		PathFilter: newLogPathFilter(opts.Paths),
		Reverse:    opts.Reverse,
	}, nil, opts.FormatJSON)
}

// newCommitIter returns the commit history from the given LogOptions.
func (r *Repository) newCommitIter(ctx context.Context, o *LogOptions, ignore []plumbing.Hash) (object.CommitIter, error) {
	fn := commitIterFunc(o.Order, ignore)
	if fn == nil {
		return nil, fmt.Errorf("invalid Order=%v", o.Order)
	}

	var (
		it  object.CommitIter
		err error
	)
	if o.All {
		it, err = r.logAll(ctx, fn)
	} else {
		it, err = r.log(ctx, o.From, fn)
	}

	if err != nil {
		return nil, err
	}

	if o.FileName != nil {
		// for `git log --all` also check parent (if the next commit comes from the real parent)
		it = r.logWithFile(*o.FileName, it, o.All)
	}
	if o.PathFilter != nil {
		it = r.logWithPathFilter(o.PathFilter, it, o.All)
	}

	if o.Since != nil || o.Until != nil {
		limitOptions := object.LogLimitOptions{Since: o.Since, Until: o.Until}
		it = r.logWithLimit(it, limitOptions)
	}

	return it, nil
}

func (r *Repository) log(ctx context.Context, from plumbing.Hash, commitIterFunc func(*object.Commit) object.CommitIter) (object.CommitIter, error) {
	h := from
	if from.IsZero() {
		current, err := r.Current()
		if err != nil {
			return nil, err
		}
		h = current.Hash()
	}

	commit, err := r.odb.ParseRevExhaustive(ctx, h)
	if err != nil {
		return nil, err
	}
	return commitIterFunc(commit), nil
}

func (r *Repository) logAll(ctx context.Context, commitIterFunc func(*object.Commit) object.CommitIter) (object.CommitIter, error) {
	return object.NewCommitAllIter(ctx, r.Backend, r.odb, commitIterFunc)
}

func (*Repository) logWithFile(fileName string, commitIter object.CommitIter, checkParent bool) object.CommitIter {
	return object.NewCommitPathIterFromIter(
		func(path string) bool {
			return path == fileName
		},
		commitIter,
		checkParent,
	)
}

func (*Repository) logWithPathFilter(pathFilter func(string) bool, commitIter object.CommitIter, checkParent bool) object.CommitIter {
	return object.NewCommitPathIterFromIter(
		pathFilter,
		commitIter,
		checkParent,
	)
}

func (*Repository) logWithLimit(commitIter object.CommitIter, limitOptions object.LogLimitOptions) object.CommitIter {
	return object.NewCommitLimitIterFromIter(commitIter, limitOptions)
}

func commitIterFunc(order LogOrder, ignore []plumbing.Hash) func(c *object.Commit) object.CommitIter {
	switch order {
	case LogOrderDefault:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitPreorderIter(c, nil, ignore)
		}
	case LogOrderDFS:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitPreorderIter(c, nil, ignore)
		}
	case LogOrderDFSPost:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitPostorderIter(c, ignore)
		}
	case LogOrderBFS:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitIterBSF(c, nil, ignore)
		}
	case LogOrderCommitterTime:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitIterCTime(c, nil, ignore)
		}
	case LogOrderAuthorTime:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitIterATime(c, nil, ignore)
		}
	}
	return nil
}
