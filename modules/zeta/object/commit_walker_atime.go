// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/emirpasic/gods/trees/binaryheap"
)

type commitIteratorByATime struct {
	seenExternal map[plumbing.Hash]bool
	seen         map[plumbing.Hash]bool
	heap         *binaryheap.Heap
}

// NewCommitIterATime returns a CommitIter that walks the commit history,
// starting at the given commit and visiting its parents while preserving Author Time order.
// this appears to be the closest order to `git log`
// The given callback will be called for each visited commit. Each commit will
// be visited only once. If the callback returns an error, walking will stop
// and will return the error. Other errors might be returned if the history
// cannot be traversed (e.g. missing objects). Ignore allows to skip some
// commits from being iterated.
func NewCommitIterATime(
	c *Commit,
	seenExternal map[plumbing.Hash]bool,
	ignore []plumbing.Hash,
) CommitIter {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}

	heap := binaryheap.NewWith(func(a, b any) int {
		if a.(*Commit).Author.When.Before(b.(*Commit).Author.When) {
			return 1
		}
		return -1
	})
	heap.Push(c)

	return &commitIteratorByATime{
		seenExternal: seenExternal,
		seen:         seen,
		heap:         heap,
	}
}

func (w *commitIteratorByATime) Next(ctx context.Context) (*Commit, error) {
	var c *Commit
	for {
		cIn, ok := w.heap.Pop()
		if !ok {
			return nil, io.EOF
		}
		c = cIn.(*Commit)

		if w.seen[c.Hash] || w.seenExternal[c.Hash] {
			continue
		}

		w.seen[c.Hash] = true

		for _, h := range c.Parents {
			if w.seen[h] || w.seenExternal[h] {
				continue
			}
			pc, err := c.b.Commit(ctx, h)
			if plumbing.IsNoSuchObject(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			w.heap.Push(pc)
		}

		return c, nil
	}
}

func (w *commitIteratorByATime) ForEach(ctx context.Context, cb func(*Commit) error) error {
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

func (w *commitIteratorByATime) Close() {}
