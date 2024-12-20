// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type MergeTree struct {
	Branch1                 string `arg:"" name:"branch1" help:"branch1"`
	Branch2                 string `arg:"" name:"branch2" help:"branch2"`
	MergeBase               string `name:"merge-base" help:"Specify a merge-base for the merge"`
	AllowUnrelatedHistories bool   `name:"allow-unrelated-histories" help:"If branches lack common history, merge-tree errors. Use this flag to force merge"`
	NameOnly                bool   `name:"name-only" help:"Only output conflict-related file names"`
	Textconv                bool   `name:"textconv" help:"Converting text to Unicode"`
	Z                       bool   `name:":z" short:"z" help:"Terminate entries with NUL byte"`
	JSON                    bool   `name:"json" help:"Convert conflict results to JSON"`
}

func (c *MergeTree) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	err = r.MergeTree(context.Background(), &zeta.MergeTreeOptions{
		Branch1:                 c.Branch1,
		Branch2:                 c.Branch2,
		MergeBase:               c.MergeBase,
		AllowUnrelatedHistories: c.AllowUnrelatedHistories,
		NameOnly:                c.NameOnly,
		Textconv:                c.Textconv,
		Z:                       c.Z,
		JSON:                    c.JSON,
	})
	if err == zeta.ErrHasConflicts {
		return &zeta.ErrExitCode{ExitCode: 1, Message: err.Error()}
	}
	if err == zeta.ErrUnrelatedHistories {
		return &zeta.ErrExitCode{ExitCode: 2, Message: err.Error()}
	}
	if err != nil {
		return &zeta.ErrExitCode{ExitCode: 127, Message: err.Error()}
	}
	return nil
}
