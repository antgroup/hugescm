// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// Rename a file
type Rename struct {
	DryRun      bool   `name:"dry-run" short:"n" help:"Dry run"`
	Force       bool   `name:"force" short:"f" help:"Force rename even if target exists"`
	K           bool   `short:"k" shortonly:"" help:"Skip rename errors"`
	Source      string `arg:"" name:"source" help:"Source"`
	Destination string `arg:"" name:"destination" help:"Destination"`
}

const (
	moveSummaryFormat = `%szeta rename [<options>] <source> <destination>`
)

func (c *Rename) Summary() string {
	return fmt.Sprintf(moveSummaryFormat, W("Usage: "))
}

func (c *Rename) Run(g *Globals) error {
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
	if err := w.Rename(context.Background(), c.Source, c.Destination, &zeta.RenameOptions{
		DryRun: c.DryRun,
		Force:  c.Force,
	}); err != nil {
		// ignore error
		if c.K {
			return nil
		}
		return err
	}
	return nil
}
