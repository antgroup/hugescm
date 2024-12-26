// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Clean struct {
	DryRun bool `name:"dry-run" short:"n" help:"dry run"`
	Force  bool `name:"force" short:"f" help:"force"`
	Dir    bool `short:"d" shortonly:"" help:"Remove whole directories"`
	ALL    bool `short:"x" shortonly:"" help:"Remove ignored files, too"`
}

func (c *Clean) Run(g *Globals) error {
	if !c.DryRun && !c.Force {
		die("refusing to clean, please specify at least -f or -n")
		return errors.New("refusing to clean")
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
	if err := w.Clean(context.Background(), &zeta.CleanOptions{DryRun: c.DryRun, Dir: c.Dir, All: c.ALL}); err != nil {
		fmt.Fprintf(os.Stderr, "zeta clean error: %v\n", err)
		return err
	}
	return nil
}
