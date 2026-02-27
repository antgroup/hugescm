// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// bfsCommitIterator implements a breadth-first search (BFS) traversal of the commit graph.
// It uses a queue to process commits level by level, visiting all commits at depth n
// before moving to depth n+1. This is useful when you want to process commits in
// chronological order (newest to oldest by generation).
type bfsCommitIterator struct {
	// seenExternal contains commits that have been seen in other iterators and should be skipped
	seenExternal map[plumbing.Hash]bool
	// seen tracks commits that have already been processed to avoid duplicates
	seen map[plumbing.Hash]bool
	// queue holds the commits to be processed in BFS order (FIFO)
	queue []*Commit
}

// NewCommitIterBFS returns a CommitIter that walks the commit history,
// starting at the given commit and visiting its parents in pre-order.
// The given callback will be called for each visited commit. Each commit will
// be visited only once. If the callback returns an error, walking will stop
// and will return the error. Other errors might be returned if the history
// cannot be traversed (e.g. missing objects). Ignore allows to skip some
// commits from being iterated.
func NewCommitIterBFS(
	c *Commit,
	seenExternal map[plumbing.Hash]bool,
	ignore []plumbing.Hash,
) CommitIter {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}

	return &bfsCommitIterator{
		seenExternal: seenExternal,
		seen:         seen,
		queue:        []*Commit{c},
	}
}

// appendHash adds a commit hash to the BFS queue if it hasn't been seen before.
// If the commit is not found in the backend (shallow clone scenario), it's silently skipped.
func (w *bfsCommitIterator) appendHash(ctx context.Context, b Backend, h plumbing.Hash) error {
	if w.seen[h] || w.seenExternal[h] {
		return nil
	}
	c, err := b.Commit(ctx, h)
	if err != nil {
		return err
	}
	w.queue = append(w.queue, c)
	return nil
}

// Next returns the next commit in BFS order. It processes commits by dequeueing
// from the front of the queue and enqueuing all unseen parents at the back.
// Missing commits (in shallow clones) are silently skipped.
func (w *bfsCommitIterator) Next(ctx context.Context) (*Commit, error) {
	var c *Commit
	for {
		if len(w.queue) == 0 {
			return nil, io.EOF
		}
		c = w.queue[0]
		w.queue = w.queue[1:]

		if w.seen[c.Hash] || w.seenExternal[c.Hash] {
			continue
		}

		w.seen[c.Hash] = true

		// Add all parent commits to the queue for later processing
		for _, h := range c.Parents {
			err := w.appendHash(ctx, c.b, h)
			if plumbing.IsNoSuchObject(err) {
				// Skip missing commits in shallow clone scenarios
				continue
			}
			if err != nil {
				return nil, err
			}
		}

		return c, nil
	}
}

// ForEach iterates through all commits in BFS order, calling the callback for each one.
// Iteration stops if the callback returns an error or ErrStop.
func (w *bfsCommitIterator) ForEach(ctx context.Context, cb func(*Commit) error) error {
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

// Close is a no-op for the BFS iterator as it doesn't hold any external resources.
func (w *bfsCommitIterator) Close() {}
