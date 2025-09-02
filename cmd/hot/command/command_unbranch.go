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
	CWD     string `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Confirm bool   `short:"Y" name:"confirm" help:"Confirm rewriting local branches and tags"`
	Prune   bool   `short:"P" name:"prune" help:"Prune repository when commits are rewritten"`
	Branch  string `short:"B" name:"branch" help:"Linearize the specified branch history"`
	Keep    int    `short:"K" name:"keep" help:"Keep the number of commits, 0 keeps all commits"`
}

func (c *Unbranch) Run(g *Globals) error {
	if len(c.Branch) == 0 && c.Keep != 0 {
		fmt.Fprintf(os.Stderr, "%s\n", tr.W("unbranch unspecified branch mode is incompatible with --keep"))
		return errors.New("unbranch unspecified branch mode is incompatible with --keep")
	}
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)
	r, err := replay.NewReplayer(context.Background(), repoPath, 2, g.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new replayer error: %v\n", err)
		return err
	}
	defer r.Close()
	if err := r.Unbranch(c.Branch, c.Confirm, c.Prune, c.Keep); err != nil {
		fmt.Fprintf(os.Stderr, "Linearize repo history error: %v\n", err)
		return err
	}
	return nil
}
