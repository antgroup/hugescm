// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/config"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Init struct {
	Branch    string `name:"branch" short:"b" help:"Override the name of the initial branch" default:"mainline" placeholder:"<branch>"`
	Remote    string `name:"remote" help:"Initialize and start tracking a new repository" placeholder:"<remote>"`
	Directory string `arg:"" name:"directory" help:"Repository directory"`
}

func (c *Init) Run(g *Globals) error {
	if len(c.Branch) != 0 {
		if !plumbing.ValidateBranchName([]byte(c.Branch)) {
			diev("'%s' is not a valid branch name", c.Branch)
			return &zeta.ErrExitCode{ExitCode: 129}
		}
	}
	if worktree, _, err := zeta.FindZetaDir(c.Directory); err == nil {
		diev("Directory '%s' is already managed by zeta", worktree)
		return &zeta.ErrExitCode{ExitCode: 127}
	}
	r, err := zeta.Init(context.Background(), &zeta.InitOptions{
		Branch:    c.Branch,
		Worktree:  c.Directory,
		MustEmpty: false,
		Verbose:   g.Verbose})
	if err != nil {
		return err
	}
	defer r.Close()
	if len(c.Remote) != 0 {
		e, err := transport.NewEndpoint(c.Remote, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta remote set remote to '%s' error: %v\n", c.Remote, err)
			return err
		}
		newRemote := e.String()
		if err := config.UpdateLocal(r.ZetaDir(), &config.UpdateOptions{
			Values: map[string]any{
				"core.remote": newRemote,
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "zeta remote set remote to '%s' error: %v\n", newRemote, err)
			return err
		}
	}
	return nil
}
