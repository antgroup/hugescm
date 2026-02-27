// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"
	"slices"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// commitPathIter implements a commit iterator that filters commits by file path.
// It performs tree diffing between consecutive commits to find commits that modified
// specific files matching a path filter. This is similar to "git log -- <path>".
type commitPathIter struct {
	// pathFilter is a function that returns true for file paths we're interested in
	pathFilter func(string) bool
	// sourceIter is the underlying commit iterator providing commits in chronological order
	sourceIter CommitIter
	// currentCommit is the commit currently being processed
	currentCommit *Commit
	// checkParent if true, verifies that the parent commit is actually in the commit tree
	// This is used for "git log --all" to filter commits that are not ancestors
	checkParent bool
}

// NewCommitPathIterFromIter returns a commit iterator which performs diffTree between
// successive trees returned from the commit iterator. The purpose of this is to find
// the commits that explain how the files that match the path came to be.
//
// If checkParent is true, the function double checks if the potential parent (next commit in a path)
// is one of the parents in the commit tree (used by "git log --all").
//
// Parameters:
//   - pathFilter: A function that takes a file path and returns true if we want commits that modified it
//   - commitIter: The source commit iterator to filter
//   - checkParent: If true, verify parent relationship (for "git log --all")
func NewCommitPathIterFromIter(pathFilter func(string) bool, commitIter CommitIter, checkParent bool) CommitIter {
	iterator := new(commitPathIter)
	iterator.sourceIter = commitIter
	iterator.pathFilter = pathFilter
	iterator.checkParent = checkParent
	return iterator
}

// NewCommitFileIterFromIter is kept for backward compatibility.
// It creates a path iterator that filters for a single specific file.
// Can be replaced with NewCommitPathIterFromIter.
func NewCommitFileIterFromIter(fileName string, commitIter CommitIter, checkParent bool) CommitIter {
	return NewCommitPathIterFromIter(
		func(path string) bool {
			return path == fileName
		},
		commitIter,
		checkParent,
	)
}

func (c *commitPathIter) Next(ctx context.Context) (*Commit, error) {
	if c.currentCommit == nil {
		var err error
		c.currentCommit, err = c.sourceIter.Next(ctx)
		if err != nil {
			return nil, err
		}
	}
	commit, commitErr := c.getNextFileCommit(ctx)

	// Setting current-commit to nil to prevent unwanted states when errors are raised
	if commitErr != nil {
		c.currentCommit = nil
	}
	return commit, commitErr
}

func (c *commitPathIter) getNextFileCommit(ctx context.Context) (*Commit, error) {
	var parentTree, currentTree *Tree

	for {
		// Parent-commit can be nil if the current-commit is the initial commit
		parentCommit, parentCommitErr := c.sourceIter.Next(ctx)
		if parentCommitErr != nil {
			// If the parent-commit is beyond the initial commit, keep it nil
			if parentCommitErr != io.EOF {
				return nil, parentCommitErr
			}
			parentCommit = nil
		}

		if parentTree == nil {
			var currTreeErr error
			currentTree, currTreeErr = c.currentCommit.Root(ctx)
			if currTreeErr != nil {
				return nil, currTreeErr
			}
		} else {
			currentTree = parentTree
			parentTree = nil
		}

		if parentCommit != nil {
			var parentTreeErr error
			parentTree, parentTreeErr = parentCommit.Root(ctx)
			if parentTreeErr != nil {
				return nil, parentTreeErr
			}
		}

		// Find diff between current and parent trees
		changes, diffErr := DiffTreeContext(ctx, currentTree, parentTree, nil)
		if diffErr != nil {
			return nil, diffErr
		}

		// Check if any changes match our path filter
		found := c.hasFileChange(changes, parentCommit)

		// Save current commit for return, update for next iteration
		prevCommit := c.currentCommit
		c.currentCommit = parentCommit

		if found {
			return prevCommit, nil
		}

		// If no match and no more parent commits, we're done
		if parentCommit == nil {
			return nil, io.EOF
		}
	}
}

// hasFileChange checks if any of the changes match the path filter and, if checkParent is true,
// verifies the parent relationship.
func (c *commitPathIter) hasFileChange(changes Changes, parent *Commit) bool {
	for _, change := range changes {
		if !c.pathFilter(change.name()) {
			continue
		}

		// File path matches, now verify parent if needed
		if c.checkParent {
			// Check if parent is beyond the initial commit or is an actual parent
			if parent == nil || isParentHash(parent.Hash, c.currentCommit) {
				return true
			}
			continue
		}

		return true
	}

	return false
}

// isParentHash checks if the given hash is one of the commit's parent hashes.
func isParentHash(hash plumbing.Hash, commit *Commit) bool {
	return slices.Contains(commit.Parents, hash)
}

// ForEach iterates through all commits that modified files matching the path filter,
// calling the callback for each one. Iteration stops if the callback returns an error or ErrStop.
func (c *commitPathIter) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		commit, nextErr := c.Next(ctx)
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nextErr
		}
		err := cb(commit)
		if err == plumbing.ErrStop {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

// Close closes the underlying source iterator.
func (c *commitPathIter) Close() {
	c.sourceIter.Close()
}
