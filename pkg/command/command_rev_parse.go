// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type RevParse struct {
	ShowToplevel bool `name:"show-toplevel" help:"Show the working tree's root path (absolute by default)"`
	ZetaDir      bool `name:"zeta-dir" help:"Show the path to the .zeta directory"`
}

func (c *RevParse) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	switch {
	case c.ShowToplevel:
		fmt.Fprintln(os.Stdout, r.BaseDir())
		return nil
	case c.ZetaDir:
		fmt.Fprintln(os.Stdout, r.ZetaDir())
		return nil
	}
	return nil
}
