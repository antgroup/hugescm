// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/zeta/config"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Remote struct {
	Show ShowRemote `cmd:"show" help:"Gives some information about the remote" default:"1"`
	Set  SetRemote  `cmd:"set" help:"Set URL for the remote"`
}

type ShowRemote struct {
	JSON bool `name:"json" short:"j" help:"Data will be returned in JSON format"`
}

func (c *ShowRemote) Run(ctx context.Context, g *Globals) error {
	r, err := zeta.Open(ctx, &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	return r.ShowRemote(&zeta.ShowRemoteOptions{JSON: c.JSON})
}

// Set or replace remote
type SetRemote struct {
	URL string `arg:"" name:"url" help:"URL for the remote"`
}

func (c *SetRemote) Run(ctx context.Context, g *Globals) error {
	r, err := zeta.Open(ctx, &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	e, err := transport.NewEndpoint(c.URL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta remote set remote to '%s' error: %v\n", c.URL, err)
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
	_, _ = fmt.Fprintf(os.Stdout, "remote: %s\n", newRemote)
	return nil
}
