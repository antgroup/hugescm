// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"time"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type GC struct {
	Prune int64 `name:"prune" help:"Pruning objects older than specified date (default is 2 weeks ago, configurable with gc.pruneExpire)" type:"expiry-date" default:"2.weeks.ago"`
	Quiet bool  `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
}

func (c *GC) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
		Quiet:    c.Quiet,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	return r.Gc(context.Background(), &zeta.GcOptions{Prune: time.Second * time.Duration(c.Prune)})
}
