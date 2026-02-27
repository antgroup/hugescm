// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// commitLimitIter implements a commit iterator that filters commits by time range.
// This is similar to "git log --since=... --until=...".
type commitLimitIter struct {
	// sourceIter is the underlying commit iterator providing commits
	sourceIter CommitIter
	// limitOptions contains the time range constraints for filtering commits
	limitOptions LogLimitOptions
}

// LogLimitOptions defines time-based filtering options for commit iteration.
type LogLimitOptions struct {
	// Only include commits after this timestamp (inclusive)
	Since *time.Time
	// Only include commits before this timestamp (inclusive)
	Until *time.Time
}

// NewCommitLimitIterFromIter creates a new commit iterator that filters commits
// by the specified time range. This is used to implement "git log --since=... --until=...".
func NewCommitLimitIterFromIter(commitIter CommitIter, limitOptions LogLimitOptions) CommitIter {
	iterator := new(commitLimitIter)
	iterator.sourceIter = commitIter
	iterator.limitOptions = limitOptions
	return iterator
}

// Next returns the next commit that falls within the specified time range.
// Commits outside the time range are silently skipped.
func (c *commitLimitIter) Next(ctx context.Context) (*Commit, error) {
	for {
		commit, err := c.sourceIter.Next(ctx)
		if err != nil {
			return nil, err
		}

		// Skip commits before the Since time
		if c.limitOptions.Since != nil && commit.Committer.When.Before(*c.limitOptions.Since) {
			continue
		}
		// Skip commits after the Until time
		if c.limitOptions.Until != nil && commit.Committer.When.After(*c.limitOptions.Until) {
			continue
		}
		return commit, nil
	}
}

// ForEach iterates through all commits within the time range, calling the callback for each one.
// Iteration stops if the callback returns an error or ErrStop.
func (c *commitLimitIter) ForEach(ctx context.Context, cb func(*Commit) error) error {
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
func (c *commitLimitIter) Close() {
	c.sourceIter.Close()
}
