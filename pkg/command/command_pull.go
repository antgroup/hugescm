// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Pull struct {
	FF        bool  `name:"ff" negatable:"" help:"Allow fast-forward" default:"true"`
	FFOnly    bool  `name:"ff-only" help:"Abort if fast-forward is not possible"`
	Rebase    bool  `name:"rebase" help:"Incorporate changes by rebasing rather than merging"`
	Squash    bool  `name:"squash" help:"Create a single commit instead of doing a merge"`
	Unshallow bool  `name:"unshallow" help:"Get complete history"`
	One       bool  `name:"one" help:"Checkout large files one after another"`
	Limit     int64 `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
}

func (c *Pull) Run(g *Globals) error {
	if c.FFOnly && c.Rebase {
		diev("--ff-only is not compatible with --rebase")
		return ErrFlagsIncompatible
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	w := r.Worktree()
	if err := w.Pull(context.Background(), &zeta.PullOptions{
		FF:        c.FF,
		FFOnly:    c.FFOnly,
		Rebase:    c.Rebase,
		Squash:    c.Squash,
		Unshallow: c.Unshallow,
		One:       c.One,
		Limit:     c.Limit,
	}); err != nil {
		return err
	}
	return nil
}
