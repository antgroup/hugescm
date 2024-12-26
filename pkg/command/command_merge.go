// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// Join two or more development histories together
type Merge struct {
	Revision                string   `arg:"" optional:"" name:"revision" help:"Merge specific revision into HEAD"`
	FF                      bool     `name:"ff" negatable:"" help:"Allow fast-forward" default:"true"`
	FFOnly                  bool     `name:"ff-only" help:"Abort if fast-forward is not possible"`
	Squash                  bool     `name:"squash" help:"Create a single commit instead of doing a merge"`
	AllowUnrelatedHistories bool     `name:"allow-unrelated-histories" help:"Allow merging unrelated histories"`
	Textconv                bool     `name:"textconv" help:"Converting text to Unicode"`
	Message                 []string `name:"message" short:"m" help:"Merge commit message (for a non-fast-forward merge)" placeholder:"<message>"`
	File                    string   `name:"file" short:"F" help:"Read message from file" placeholder:"<file>"`
	Signoff                 bool     `name:"signoff" negatable:"" help:"Add a Signed-off-by trailer" default:"false"`
	Abort                   bool     `name:"abort" help:"Abort a conflicting merge"`
	Continue                bool     `name:"continue" help:"Continue a merge with resolved conflicts"`
}

const (
	mergeSummaryFormat = `%szeta merge [<options>] [<revision>]
%szeta merge --abort
%szeta merge --continue`
)

func (c *Merge) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(mergeSummaryFormat, W("Usage: "), or, or)
}

func (c *Merge) Run(g *Globals) error {
	if c.FFOnly && c.Squash {
		diev("--ff-only is not compatible with --squash")
		return ErrFlagsIncompatible
	}
	if c.Abort && c.Continue {
		diev("--abort is not compatible with --continue")
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
	defer r.Close()
	w := r.Worktree()
	if err := w.Merge(context.Background(), &zeta.MergeOptions{
		From:                    c.Revision,
		FF:                      c.FF,
		FFOnly:                  c.FFOnly,
		Squash:                  c.Squash,
		Signoff:                 c.Signoff,
		Message:                 c.Message,
		File:                    c.File,
		AllowUnrelatedHistories: c.AllowUnrelatedHistories,
		Textconv:                c.Textconv,
		Abort:                   c.Abort,
		Continue:                c.Continue,
	}); err != nil {
		return err
	}
	return nil
}
