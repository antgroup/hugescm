// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/replay"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type Unbranch struct {
	Revision string `arg:"" optional:"" name:"revision" help:"Linearize the specified revision history"`
	CWD      string `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Confirm  bool   `short:"Y" name:"confirm" help:"Confirm rewriting local branches and tags"`
	Prune    bool   `short:"P" name:"prune" help:"Prune repository when commits are rewritten"`
	Target   string `short:"T" name:"target" help:"Save linearized branches to new target"`
	Keep     int    `short:"K" name:"keep" help:"Keep the number of commits, 0 keeps all commits"`
}

func (c *Unbranch) Run(g *Globals) error {
	if len(c.Revision) == 0 && c.Keep != 0 {
		fmt.Fprintf(os.Stderr, "%s\n", tr.W("unbranch unspecified branch mode is incompatible with --keep"))
		return errors.New("unbranch unspecified branch mode is incompatible with --keep")
	}
	if len(c.Target) != 0 {
		if !git.ValidateBranchName([]byte(c.Target)) {
			fmt.Fprintf(os.Stderr, "invalid branch name '%s'\n", c.Target)
			return errors.New("bad branch name")
		}
	}
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)
	r, err := replay.NewReplayer(context.Background(), repoPath, 2, g.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new replayer error: %v\n", err)
		return err
	}
	defer r.Close() // nolint
	if err := r.Unbranch(&replay.UnbranchOptions{
		Branch:  c.Revision,
		Target:  c.Target,
		Confirm: c.Confirm,
		Prune:   c.Prune,
		Keep:    c.Keep,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Linearize repo history error: %v\n", err)
		return err
	}
	return nil
}
