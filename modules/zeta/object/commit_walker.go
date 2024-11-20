// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"container/list"
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/refs"
)

type lookupIter struct {
	b      Backend
	series []plumbing.Hash
	pos    int
}

func NewCommitIter(b Backend, hashes []plumbing.Hash) CommitIter {
	return &lookupIter{b: b, series: hashes}
}

func (iter *lookupIter) Next(ctx context.Context) (*Commit, error) {
	if iter.pos >= len(iter.series) {
		return nil, io.EOF
	}
	oid := iter.series[iter.pos]
	cc, err := iter.b.Commit(ctx, oid)
	if plumbing.IsNoSuchObject(err) {
		return nil, io.EOF
	}
	if err == nil {
		iter.pos++
	}
	return cc, err
}

func (iter *lookupIter) ForEach(ctx context.Context, cb func(*Commit) error) error {
	defer iter.Close()
	for {
		cc, err := iter.Next(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := cb(cc); err != nil {
			if err == plumbing.ErrStop {
				return nil
			}
			return err
		}
	}
}

func (iter *lookupIter) Close() {
	iter.pos = len(iter.series)
}

type commitPreIterator struct {
	seenExternal map[plumbing.Hash]bool
	seen         map[plumbing.Hash]bool
	stack        []CommitIter
	start        *Commit
}

// NewCommitPreorderIter returns a CommitIter that walks the commit history,
// starting at the given commit and visiting its parents in pre-order.
// The given callback will be called for each visited commit. Each commit will
// be visited only once. If the callback returns an error, walking will stop
// and will return the error. Other errors might be returned if the history
// cannot be traversed (e.g. missing objects). Ignore allows to skip some
// commits from being iterated.
func NewCommitPreorderIter(
	c *Commit,
	seenExternal map[plumbing.Hash]bool,
	ignore []plumbing.Hash,
) CommitIter {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}

	return &commitPreIterator{
		seenExternal: seenExternal,
		seen:         seen,
		stack:        make([]CommitIter, 0),
		start:        c,
	}
}

func (w *commitPreIterator) Next(ctx context.Context) (*Commit, error) {
	var c *Commit
	for {
		if w.start != nil {
			c = w.start
			w.start = nil
		} else {
			current := len(w.stack) - 1
			if current < 0 {
				return nil, io.EOF
			}

			var err error
			c, err = w.stack[current].Next(ctx)
			if err == io.EOF {
				w.stack = w.stack[:current]
				continue
			}

			if err != nil {
				return nil, err
			}
		}

		if w.seen[c.Hash] || w.seenExternal[c.Hash] {
			continue
		}

		w.seen[c.Hash] = true

		if c.NumParents() > 0 {
			w.stack = append(w.stack, filteredParentIter(c, w.seen))
		}

		return c, nil
	}
}

func filteredParentIter(c *Commit, seen map[plumbing.Hash]bool) CommitIter {
	var hashes []plumbing.Hash
	for _, h := range c.Parents {
		if !seen[h] {
			hashes = append(hashes, h)
		}
	}

	return NewCommitIter(c.b, hashes)
}

func (w *commitPreIterator) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		c, err := w.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = cb(c)
		if err == plumbing.ErrStop {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *commitPreIterator) Close() {}

type commitPostIterator struct {
	stack []*Commit
	seen  map[plumbing.Hash]bool
}

// NewCommitPostorderIter returns a CommitIter that walks the commit
// history like WalkCommitHistory but in post-order. This means that after
// walking a merge commit, the merged commit will be walked before the base
// it was merged on. This can be useful if you wish to see the history in
// chronological order. Ignore allows to skip some commits from being iterated.
func NewCommitPostorderIter(c *Commit, ignore []plumbing.Hash) CommitIter {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}

	return &commitPostIterator{
		stack: []*Commit{c},
		seen:  seen,
	}
}

func (w *commitPostIterator) Next(ctx context.Context) (*Commit, error) {
	for {
		if len(w.stack) == 0 {
			return nil, io.EOF
		}

		c := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]

		if w.seen[c.Hash] {
			continue
		}

		w.seen[c.Hash] = true

		return c, c.MakeParents().ForEach(ctx, func(p *Commit) error {
			w.stack = append(w.stack, p)
			return nil
		})
	}
}

func (w *commitPostIterator) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		c, err := w.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = cb(c)
		if err == plumbing.ErrStop {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *commitPostIterator) Close() {}

// commitAllIterator stands for commit iterator for all refs.
type commitAllIterator struct {
	// currCommit points to the current commit.
	currCommit *list.Element
}

// NewCommitAllIter returns a new commit iterator for all refs.
// repoStorer is a repo Storer used to get commits and references.
// commitIterFunc is a commit iterator function, used to iterate through ref commits in chosen order
func NewCommitAllIter(ctx context.Context, rdb refs.Backend, odb Backend, commitIterFunc func(*Commit) CommitIter) (CommitIter, error) {
	commitsPath := list.New()
	commitsLookup := make(map[plumbing.Hash]*list.Element)
	head, err := refs.ReferenceResolve(rdb, plumbing.HEAD)
	if err == nil {
		err = addReference(ctx, odb, commitIterFunc, head, commitsPath, commitsLookup)
	}

	if err != nil && err != plumbing.ErrReferenceNotFound {
		return nil, err
	}
	// add all references along with the HEAD
	refIter, err := refs.NewReferenceIter(rdb)
	if err != nil {
		return nil, err
	}
	defer refIter.Close()

	for {
		ref, err := refIter.Next()
		if err == io.EOF {
			break
		}

		if err == plumbing.ErrReferenceNotFound {
			continue
		}

		if err != nil {
			return nil, err
		}

		if err = addReference(ctx, odb, commitIterFunc, ref, commitsPath, commitsLookup); err != nil {
			return nil, err
		}
	}

	return &commitAllIterator{commitsPath.Front()}, nil
}

func addReference(
	ctx context.Context,
	b Backend,
	commitIterFunc func(*Commit) CommitIter,
	ref *plumbing.Reference,
	commitsPath *list.List,
	commitsLookup map[plumbing.Hash]*list.Element) error {

	_, exists := commitsLookup[ref.Hash()]
	if exists {
		// we already have it - skip the reference.
		return nil
	}

	refCommit, _ := GetCommit(ctx, b, ref.Hash())
	if refCommit == nil {
		// if it's not a commit - skip it.
		return nil
	}

	var (
		refCommits []*Commit
		parent     *list.Element
	)
	// collect all ref commits to add
	commitIter := commitIterFunc(refCommit)
	for c, e := commitIter.Next(ctx); e == nil; {
		parent, exists = commitsLookup[c.Hash]
		if exists {
			break
		}
		refCommits = append(refCommits, c)
		c, e = commitIter.Next(ctx)
	}
	commitIter.Close()

	if parent == nil {
		// common parent - not found
		// add all commits to the path from this ref (maybe it's a HEAD and we don't have anything, yet)
		for _, c := range refCommits {
			parent = commitsPath.PushBack(c)
			commitsLookup[c.Hash] = parent
		}
	} else {
		// add ref's commits to the path in reverse order (from the latest)
		for i := len(refCommits) - 1; i >= 0; i-- {
			c := refCommits[i]
			// insert before found common parent
			parent = commitsPath.InsertBefore(c, parent)
			commitsLookup[c.Hash] = parent
		}
	}

	return nil
}

func (it *commitAllIterator) Next(ctx context.Context) (*Commit, error) {
	if it.currCommit == nil {
		return nil, io.EOF
	}

	c := it.currCommit.Value.(*Commit)
	it.currCommit = it.currCommit.Next()

	return c, nil
}

func (it *commitAllIterator) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		c, err := it.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = cb(c)
		if err == plumbing.ErrStop {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (it *commitAllIterator) Close() {
	it.currCommit = nil
}
