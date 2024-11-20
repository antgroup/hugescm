// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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

func (r *Repository) logPrint(ctx context.Context, opts *LogOptions, latest plumbing.Hash, formatJSON bool) error {
	if formatJSON {
		iter, err := r.logInter(ctx, opts)
		if err != nil {
			return err
		}
		defer iter.Close()
		commits := make([]*object.Commit, 0, 20)
		var cc *object.Commit
		for {
			if cc, err = iter.Next(ctx); err != nil {
				break
			}
			if cc.Hash == latest {
				break
			}
			commits = append(commits, cc)
		}
		if err != io.EOF && err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(commits)
	}
	rdb, err := r.ReferencesEx(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve references error: %v\n", err)
		return err
	}
	iter, err := r.logInter(ctx, opts)
	if err != nil {
		return err
	}
	defer iter.Close()
	p := NewPrinter(ctx)
	var cc *object.Commit
	for {
		if cc, err = iter.Next(ctx); err != nil {
			break
		}
		if cc.Hash == latest {
			break
		}
		if err := p.LogOne(cc, rdb.M[cc.Hash]); err != nil {
			_ = p.Close()
			return err
		}
	}
	_ = p.Close()
	if err == io.EOF {
		err = nil
	}
	return err
}

type commitsGroup struct {
	commits []*object.Commit
	seen    map[plumbing.Hash]bool
}

func (r *Repository) revList0(ctx context.Context, start, end plumbing.Hash, paths []string, cg *commitsGroup) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	opts := &LogOptions{
		Order: LogOrderBSF,
		From:  start,
	}
	if len(paths) != 0 {
		m := NewMatcher(paths)
		opts.PathFilter = m.Match
	}
	iter, err := r.logInter(ctx, opts)
	if err != nil {
		return err
	}
	defer iter.Close()
	var cc *object.Commit
	for {
		if cc, err = iter.Next(ctx); err != nil {
			break
		}
		if cc.Hash == end {
			break
		}
		if cg.seen[cc.Hash] {
			continue
		}
		cg.commits = append(cg.commits, cc)
	}
	if err == io.EOF {
		err = nil
	}
	return err
}

func (r *Repository) revList(ctx context.Context, start, end plumbing.Hash, paths []string) ([]*object.Commit, error) {
	cg := &commitsGroup{
		commits: make([]*object.Commit, 0, 100),
		seen:    make(map[plumbing.Hash]bool),
	}
	if err := r.revList0(ctx, start, end, paths, cg); err != nil {
		return nil, err
	}
	return cg.commits, nil
}

// logFromMergeBase: a...b  a from merge-base and b from merge-base changes
func (r *Repository) logFromMergeBase(ctx context.Context, a, b plumbing.Hash, paths []string, formatJSON bool) error {
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
	var end plumbing.Hash
	if len(bases) != 0 {
		end = bases[0].Hash
	}
	cg := &commitsGroup{
		commits: make([]*object.Commit, 0, 100),
		seen:    make(map[plumbing.Hash]bool),
	}
	if err := r.revList0(ctx, ac.Hash, end, paths, cg); err != nil {
		fmt.Fprintf(os.Stderr, "log commit '%s' error: %v\n", a, err)
		return err
	}
	if err := r.revList0(ctx, bc.Hash, end, paths, cg); err != nil {
		fmt.Fprintf(os.Stderr, "log commit '%s' error: %v\n", b, err)
		return err
	}
	if formatJSON {
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
func (r *Repository) logRevFromTo(ctx context.Context, from, to plumbing.Hash, paths []string, formatJSON bool) error {
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
	mergeBases, err := newRev.MergeBase(ctx, oldRev)
	if err != nil {
		die_error("resolve merge-base error: %v", err)
		return err
	}
	if len(mergeBases) == 0 {
		opts := &LogOptions{
			Order: LogOrderBSF,
			From:  newRev.Hash,
		}
		if len(paths) != 0 {
			m := NewMatcher(paths)
			opts.PathFilter = m.Match
		}
		return r.logPrint(ctx, opts, plumbing.ZeroHash, formatJSON)
	}
	mergeBase := mergeBases[0]
	if mergeBase.Hash == newRev.Hash {
		// newRev is old rev parents
		return nil
	}
	opts := &LogOptions{
		Order: LogOrderBSF,
		From:  newRev.Hash,
	}
	if len(paths) != 0 {
		m := NewMatcher(paths)
		opts.PathFilter = m.Match
	}
	return r.logPrint(ctx, opts, mergeBase.Hash, formatJSON)
}

func (r *Repository) Log(ctx context.Context, revRange string, paths []string, formatJSON bool) error {
	if aRev, bRev, ok := strings.Cut(revRange, "..."); ok {
		a, err := r.resolveRevision(ctx, aRev)
		if err != nil {
			dieln(err)
			return err
		}
		b, err := r.resolveRevision(ctx, bRev)
		if err != nil {
			dieln(err)
			return err
		}
		return r.logFromMergeBase(ctx, a, b, paths, formatJSON)
	}
	if fromRev, toRev, ok := strings.Cut(revRange, ".."); ok {
		from, err := r.resolveRevision(ctx, fromRev)
		if err != nil {
			dieln(err)
			return err
		}
		to, err := r.resolveRevision(ctx, toRev)
		if err != nil {
			dieln(err)
			return err
		}
		return r.logRevFromTo(ctx, from, to, paths, formatJSON)
	}
	if revRange == "" {
		revRange = "HEAD"
	}
	rev, err := r.resolveRevision(ctx, revRange)
	if err != nil {
		dieln(err)
		return err
	}
	opts := &LogOptions{
		Order: LogOrderBSF,
		From:  rev,
	}
	if len(paths) != 0 {
		m := NewMatcher(paths)
		opts.PathFilter = m.Match
	}
	return r.logPrint(ctx, opts, plumbing.ZeroHash, formatJSON)
}

// logInter returns the commit history from the given LogOptions.
func (r *Repository) logInter(ctx context.Context, o *LogOptions) (object.CommitIter, error) {
	fn := commitIterFunc(o.Order)
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
	if from == plumbing.ZeroHash {
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

func commitIterFunc(order LogOrder) func(c *object.Commit) object.CommitIter {
	switch order {
	case LogOrderDefault:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitPreorderIter(c, nil, nil)
		}
	case LogOrderDFS:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitPreorderIter(c, nil, nil)
		}
	case LogOrderDFSPost:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitPostorderIter(c, nil)
		}
	case LogOrderBSF:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitIterBSF(c, nil, nil)
		}
	case LogOrderCommitterTime:
		return func(c *object.Commit) object.CommitIter {
			return object.NewCommitIterCTime(c, nil, nil)
		}
	}
	return nil
}
