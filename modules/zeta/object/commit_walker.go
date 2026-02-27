// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"container/list"
	"context"
	"errors"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/refs"
)

// lookupIter implements CommitIter by looking up commits from a Backend
// based on a predefined list of commit hashes. This is useful when you already
// know the exact commit hashes you want to traverse and don't need to discover
// the commit graph dynamically.
type lookupIter struct {
	b      Backend         // Backend to fetch commits from
	series []plumbing.Hash // List of commit hashes to iterate over
	pos    int             // Current position in the series
}

// NewCommitIter creates a new CommitIter that iterates over commits with the
// given hashes in the specified order. This is a simple iterator that directly
// fetches commits from the backend without any graph traversal logic.
//
// Parameters:
//   - b: Backend to fetch commits from
//   - hashes: Ordered list of commit hashes to iterate over
//
// Returns:
//   - CommitIter that yields commits in the order provided
func NewCommitIter(b Backend, hashes []plumbing.Hash) CommitIter {
	return &lookupIter{b: b, series: hashes}
}

// Next returns the next commit in the series. If all commits have been returned
// or a commit cannot be found in the backend (ErrNoSuchObject), it returns io.EOF.
//
// This method is designed to be called repeatedly until io.EOF is returned,
// indicating that there are no more commits to iterate over.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//
// Returns:
//   - *Commit: The next commit in the series
//   - error: io.EOF if no more commits, or an error if the commit cannot be fetched
func (iter *lookupIter) Next(ctx context.Context) (*Commit, error) {
	if iter.pos >= len(iter.series) {
		return nil, io.EOF
	}
	oid := iter.series[iter.pos]
	cc, err := iter.b.Commit(ctx, oid)
	if plumbing.IsNoSuchObject(err) {
		// If the commit doesn't exist in the backend, treat it as EOF
		// This is important for shallow clone scenarios where some commits
		// may be missing
		return nil, io.EOF
	}
	if err == nil {
		iter.pos++
	}
	return cc, err
}

// ForEach iterates over all commits in the series, calling the provided callback
// function for each commit. The iteration stops when the callback returns an error
// or when all commits have been processed.
//
// Special handling for error returns:
//   - plumbing.ErrStop: Stops iteration without error
//   - io.EOF: Marks the end of iteration, not an error
//   - Other errors: Stops iteration and returns the error
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - cb: Callback function called for each commit
//
// Returns:
//   - error: Any error returned by the callback, or nil if iteration completes
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

// Close marks the iterator as closed by advancing the position to the end
// of the series. After calling Close, subsequent calls to Next will return io.EOF.
func (iter *lookupIter) Close() {
	iter.pos = len(iter.series)
}

// commitPreIterator implements CommitIter with pre-order traversal of the commit graph.
// Pre-order means that a commit is visited before its parents. This iterator uses
// a depth-first search (DFS) approach with an explicit stack to avoid recursion.
//
// Deduplication: Each commit is visited at most once using two seen maps:
//   - seen: Commits already visited by this iterator
//   - seenExternal: Commits already visited by other iterators (for complex traversals)
//
// Shallow clone support: Missing commits (ErrNoSuchObject) are handled gracefully,
// allowing the traversal to continue with available commits.
type commitPreIterator struct {
	seenExternal map[plumbing.Hash]bool // Commits seen by external iterators
	seen         map[plumbing.Hash]bool // Commits already visited by this iterator
	stack        []CommitIter           // Stack for DFS traversal
	start        *Commit                // Starting commit to process first
}

// NewCommitPreorderIter creates a new CommitIter that walks the commit history
// in pre-order (depth-first), starting at the given commit and visiting its parents.
//
// Pre-order traversal characteristics:
//   - Commits are visited before their parents
//   - Uses depth-first search with explicit stack
//   - Each commit is visited exactly once (deduplication)
//   - Handles missing commits gracefully (shallow clone support)
//
// Parameters:
//   - c: Starting commit for the traversal
//   - seenExternal: Map of commits already seen by other iterators (can be nil)
//   - ignore: List of commit hashes to skip during traversal
//
// Returns:
//   - CommitIter that yields commits in pre-order
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

// Next returns the next commit in pre-order. This method implements depth-first
// traversal using an explicit stack to avoid recursion.
//
// Algorithm:
//  1. If this is the first call, return the start commit
//  2. Pop the top iterator from the stack and get its next commit
//  3. If the iterator is exhausted, pop it and continue
//  4. If the commit has already been seen, skip it
//  5. Mark the commit as seen and push its parents onto the stack
//  6. Return the commit
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//
// Returns:
//   - *Commit: The next commit in pre-order
//   - error: io.EOF if no more commits, or an error if traversal fails
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

// filteredParentIter creates an iterator for a commit's parents, excluding any
// commits that have already been seen. This is a key optimization for commit graph
// traversal that prevents revisiting the same commit multiple times.
//
// This function is particularly important for merge commits, which have multiple
// parents. By filtering out already-seen parents, we avoid redundant work and
// ensure that each commit is visited exactly once.
//
// Parameters:
//   - c: The commit whose parents should be iterated
//   - seen: Map of commit hashes that have already been visited
//
// Returns:
//   - CommitIter that yields the commit's unseen parents
func filteredParentIter(c *Commit, seen map[plumbing.Hash]bool) CommitIter {
	var hashes []plumbing.Hash
	for _, h := range c.Parents {
		if !seen[h] {
			hashes = append(hashes, h)
		}
	}

	return NewCommitIter(c.b, hashes)
}

// ForEach iterates over all commits reachable from the starting commit in pre-order,
// calling the provided callback function for each commit. The iteration stops when
// the callback returns an error or when all reachable commits have been processed.
//
// Special handling for error returns:
//   - plumbing.ErrStop: Stops iteration without error
//   - io.EOF: Marks the end of iteration, not an error
//   - Other errors: Stops iteration and returns the error
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - cb: Callback function called for each commit
//
// Returns:
//   - error: Any error returned by the callback, or nil if iteration completes
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

// Close is a no-op for commitPreIterator as it doesn't hold any external
// resources that need to be explicitly cleaned up.
func (w *commitPreIterator) Close() {}

// commitPostIterator implements CommitIter with post-order traversal of the commit graph.
// Post-order means that a commit is visited after all its descendants (parents in git's
// terminology). This is useful when you want to see the history in chronological order,
// where older commits are visited after newer commits.
//
// Post-order traversal characteristics:
//   - Commits are visited after their parents
//   - Uses depth-first search with explicit stack
//   - Each commit is visited exactly once (deduplication)
//   - Particularly useful for chronological history viewing
type commitPostIterator struct {
	stack []*Commit              // Stack for DFS traversal
	seen  map[plumbing.Hash]bool // Commits already visited
}

// NewCommitPostorderIter creates a new CommitIter that walks the commit history
// in post-order (depth-first), starting at the given commit.
//
// Post-order traversal characteristics:
//   - Commits are visited after their parents
//   - Useful for chronological history viewing (older commits after newer ones)
//   - Uses depth-first search with explicit stack
//   - Each commit is visited exactly once (deduplication)
//
// Example:
//
//	For a commit graph: C3 <- C2 <- C1
//	Pre-order visits: C3, C2, C1
//	Post-order visits: C1, C2, C3
//
// Parameters:
//   - c: Starting commit for the traversal
//   - ignore: List of commit hashes to skip during traversal
//
// Returns:
//   - CommitIter that yields commits in post-order
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

type commitPostIteratorFirstParent struct {
	stack []*Commit
	seen  map[plumbing.Hash]bool
}

// NewCommitPostorderIterFirstParent returns a CommitIter that walks the commit
// history like WalkCommitHistory but in post-order.
//
// This option gives a better overview when viewing the evolution of a particular
// topic branch, because merges into a topic branch tend to be only about
// adjusting to updated upstream from time to time, and this option allows
// you to ignore the individual commits brought in to your history by such
// a merge.
//
// Ignore allows to skip some commits from being iterated.
func NewCommitPostorderIterFirstParent(c *Commit, ignore []plumbing.Hash) CommitIter {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}

	return &commitPostIteratorFirstParent{
		stack: []*Commit{c},
		seen:  seen,
	}
}

func (w *commitPostIteratorFirstParent) Next(ctx context.Context) (*Commit, error) {
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
			if len(c.Parents) > 0 && p.Hash == c.Parents[0] {
				w.stack = append(w.stack, p)
			}
			return nil
		})
	}
}

func (w *commitPostIteratorFirstParent) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		c, err := w.Next(ctx)
		if errors.Is(err, io.EOF) {
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

func (w *commitPostIteratorFirstParent) Close() {}
