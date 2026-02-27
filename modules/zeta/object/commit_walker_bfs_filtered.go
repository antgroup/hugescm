// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// NewFilterCommitIter returns a CommitIter that walks the commit history,
// starting at the passed commit and visiting its parents in Breadth-first order.
// The commits returned by the CommitIter will validate the passed CommitFilter.
// The history won't be traversed beyond a commit if isLimit is true for it.
// Each commit will be visited only once.
// If the commit history can not be traversed, or the Close() method is called,
// the CommitIter won't return more commits.
// If no isValid is passed, all ancestors of from commit will be valid.
// If no isLimit is limit, all ancestors of all commits will be visited.
func NewFilterCommitIter(
	from *Commit,
	isValid *CommitFilter,
	isLimit *CommitFilter,
) CommitIter {
	var validFilter CommitFilter
	if isValid == nil {
		validFilter = func(_ *Commit) bool {
			return true
		}
	} else {
		validFilter = *isValid
	}

	var limitFilter CommitFilter
	if isLimit == nil {
		limitFilter = func(_ *Commit) bool {
			return false
		}
	} else {
		limitFilter = *isLimit
	}

	return &filterCommitIter{
		isValid: validFilter,
		isLimit: limitFilter,
		visited: map[plumbing.Hash]struct{}{},
		queue:   []*Commit{from},
	}
}

// CommitFilter is a predicate function that determines whether a commit should be
// included in iteration results. Returns true if the commit passes the filter.
type CommitFilter func(*Commit) bool

// filterCommitIter implements CommitIter with BFS traversal and custom filtering.
// It supports two types of filters:
//   - isValid: Determines if a commit should be yielded to the caller
//   - isLimit: Determines if traversal should stop at a commit (don't visit its parents)
//
// This is used to implement commands like "git log --merges-only" or "git log --no-merges".
type filterCommitIter struct {
	// isValid determines if a commit should be yielded to the caller
	isValid CommitFilter
	// isLimit determines if traversal should stop at a commit (don't visit parents)
	isLimit CommitFilter
	// visited tracks commits that have already been processed to avoid duplicates
	visited map[plumbing.Hash]struct{}
	// queue holds commits to be processed in BFS order (FIFO)
	queue []*Commit
	// lastErr stores the last error encountered during iteration
	lastErr error
}

// Next returns the next commit of the CommitIter.
// It will return io.EOF if there are no more commits to visit,
// or an error if the history could not be traversed.
func (w *filterCommitIter) Next(ctx context.Context) (*Commit, error) {
	var commit *Commit
	var err error
	for {
		commit, err = w.popNewFromQueue()
		if err != nil {
			return nil, w.close(err)
		}

		w.visited[commit.Hash] = struct{}{}

		if !w.isLimit(commit) {
			err = w.addToQueue(ctx, commit.b, commit.Parents...)
			if err != nil {
				return nil, w.close(err)
			}
		}

		if w.isValid(commit) {
			return commit, nil
		}
	}
}

// ForEach runs the passed callback over each Commit returned by the CommitIter
// until the callback returns an error or there is no more commits to traverse.
func (w *filterCommitIter) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		commit, err := w.Next(ctx)
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if err := cb(commit); err == plumbing.ErrStop {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

// Error returns the error that caused that the CommitIter is no longer returning commits
func (w *filterCommitIter) Error() error {
	return w.lastErr
}

// Close cleans up the iterator's internal state, releasing references to commits
// and filters. After calling Close, the iterator cannot be used further.
func (w *filterCommitIter) Close() {
	w.visited = map[plumbing.Hash]struct{}{}
	w.queue = []*Commit{}
	w.isLimit = nil
	w.isValid = nil
}

// close is an internal helper that closes the iterator and records an error.
// This is used when an error occurs during iteration.
//
// Parameters:
//   - err: The error to record
//
// Returns:
//   - error: The same error passed in
func (w *filterCommitIter) close(err error) error {
	w.Close()
	w.lastErr = err
	return err
}

// popNewFromQueue removes and returns the first unvisited commit from the FIFO queue.
//
// This method implements the FIFO queue behavior for BFS traversal:
//   - Returns the first commit in the queue (oldest)
//   - Skips commits that have already been visited (deduplication)
//   - Returns io.EOF when the queue is empty
//
// Returns:
//   - *Commit: The first unvisited commit
//   - error: io.EOF if queue is empty, or the last error if one occurred
func (w *filterCommitIter) popNewFromQueue() (*Commit, error) {
	var first *Commit
	for {
		if len(w.queue) == 0 {
			if w.lastErr != nil {
				return nil, w.lastErr
			}

			return nil, io.EOF
		}

		first = w.queue[0]
		w.queue = w.queue[1:]
		if _, ok := w.visited[first.Hash]; ok {
			continue
		}

		return first, nil
	}
}

// addToQueue adds the passed commits to the internal fifo queue if they weren't seen
// or returns an error if the passed hashes could not be used to get valid commits
// In shallow clone scenarios (where some commits are missing), missing commits are
// skipped instead of returning an error, allowing the traversal to continue.
func (w *filterCommitIter) addToQueue(
	ctx context.Context,
	b Backend,
	hashes ...plumbing.Hash,
) error {
	for _, hash := range hashes {
		if _, ok := w.visited[hash]; ok {
			continue
		}

		commit, err := b.Commit(ctx, hash)
		if plumbing.IsNoSuchObject(err) {
			// In shallow clone scenarios, missing commits are skipped
			// instead of returning an error. This allows the traversal
			// to continue with available commits.
			continue
		}
		if err != nil {
			return err
		}

		w.queue = append(w.queue, commit)
	}

	return nil
}
