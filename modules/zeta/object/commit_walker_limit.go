// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"io"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type commitLimitIter struct {
	sourceIter   CommitIter
	limitOptions LogLimitOptions
}

type LogLimitOptions struct {
	Since *time.Time
	Until *time.Time
}

func NewCommitLimitIterFromIter(commitIter CommitIter, limitOptions LogLimitOptions) CommitIter {
	iterator := new(commitLimitIter)
	iterator.sourceIter = commitIter
	iterator.limitOptions = limitOptions
	return iterator
}

func (c *commitLimitIter) Next(ctx context.Context) (*Commit, error) {
	for {
		commit, err := c.sourceIter.Next(ctx)
		if err != nil {
			return nil, err
		}

		if c.limitOptions.Since != nil && commit.Committer.When.Before(*c.limitOptions.Since) {
			continue
		}
		if c.limitOptions.Until != nil && commit.Committer.When.After(*c.limitOptions.Until) {
			continue
		}
		return commit, nil
	}
}

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

func (c *commitLimitIter) Close() {
	c.sourceIter.Close()
}
