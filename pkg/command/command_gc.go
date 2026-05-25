// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"time"

	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type GC struct {
	Prune time.Duration `name:"prune" help:"Pruning objects older than specified date (default is 2 weeks ago, configurable with gc.pruneExpire)" type:"expire" default:"2.weeks.ago"`
	Quiet bool          `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
}

func (c *GC) Run(ctx context.Context, g *Globals) error {
	trace.DbgPrint("prune: %v", c.Prune)
	r, err := zeta.Open(ctx, &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
		Quiet:    c.Quiet,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	return r.Gc(ctx, &zeta.GcOptions{Prune: c.Prune})
}
