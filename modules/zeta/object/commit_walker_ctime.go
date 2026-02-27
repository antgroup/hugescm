// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/emirpasic/gods/trees/binaryheap"
)

// commitIteratorByCTime implements a commit walker that orders commits by committer timestamp.
// This is the closest to "git log" default ordering, showing commits from newest to oldest.
type commitIteratorByCTime struct {
	// seenExternal contains commits that have been seen in other iterators and should be skipped
	seenExternal map[plumbing.Hash]bool
	// seen tracks commits that have already been processed to avoid duplicates
	seen map[plumbing.Hash]bool
	// heap is a max-heap ordered by committer timestamp (newest first)
	heap *binaryheap.Heap
}

// NewCommitIterCTime returns a CommitIter that walks the commit history,
// starting at the given commit and visiting its parents while preserving Committer Time order.
// This appears to be the closest order to `git log` (newest commits first).
//
// The iterator will visit each commit only once. If the callback returns an error,
// walking will stop and return the error. Missing commits (in shallow clones) are silently skipped.
//
// Parameters:
//   - c: The starting commit
//   - seenExternal: Commits already seen in other traversals
//   - ignore: List of commits to skip
func NewCommitIterCTime(
	c *Commit,
	seenExternal map[plumbing.Hash]bool,
	ignore []plumbing.Hash,
) CommitIter {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}

	// Create a max-heap ordered by committer timestamp (newest first)
	heap := binaryheap.NewWith(func(a, b any) int {
		if a.(*Commit).Committer.When.Before(b.(*Commit).Committer.When) {
			return 1
		}
		return -1
	})
	heap.Push(c)

	return &commitIteratorByCTime{
		seenExternal: seenExternal,
		seen:         seen,
		heap:         heap,
	}
}

// Next returns the next commit in committer timestamp order (newest first).
// It pops from the heap, marks the commit as seen, and pushes all unseen parents
// to the heap. Missing commits (in shallow clones) are silently skipped.
func (w *commitIteratorByCTime) Next(ctx context.Context) (*Commit, error) {
	var c *Commit
	for {
		cIn, ok := w.heap.Pop()
		if !ok {
			return nil, io.EOF
		}
		c = cIn.(*Commit)

		// Skip commits that have already been seen
		if w.seen[c.Hash] || w.seenExternal[c.Hash] {
			continue
		}

		w.seen[c.Hash] = true

		// Add all parent commits to the heap for later processing
		for _, h := range c.Parents {
			if w.seen[h] || w.seenExternal[h] {
				continue
			}
			pc, err := c.b.Commit(ctx, h)
			if plumbing.IsNoSuchObject(err) {
				// Skip missing commits in shallow clone scenarios
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

// ForEach iterates through all commits in committer timestamp order, calling the callback for each one.
// Iteration stops if the callback returns an error or ErrStop.
func (w *commitIteratorByCTime) ForEach(ctx context.Context, cb func(*Commit) error) error {
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

// Close is a no-op for the CTime iterator as it doesn't hold any external resources.
func (w *commitIteratorByCTime) Close() {}
