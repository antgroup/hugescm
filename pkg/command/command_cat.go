// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Cat struct {
	Object      string `arg:"" name:"object" help:"The name of the object to show"`
	T           bool   `name:"type" short:"t" help:"Show object type"`
	DisplaySize bool   `name:":" short:"s" help:"Show object size"`
	JSON        bool   `name:"json" short:"j" help:"Returns data as JSON; limited to commits, trees, fragments, and tags"`
	Textconv    bool   `name:"textconv" help:"Converting text to Unicode"`
	Direct      bool   `name:"direct" help:"View files directly"`
	Verify      bool   `name:"verify" help:"Verify object hash"`
	Limit       int64  `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Output      string `name:"output" help:"Output to a specific file instead of stdout" placeholder:"<file>"`
}

func (c *Cat) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	return r.Cat(context.Background(), &zeta.CatOptions{
		Object:    c.Object,
		Limit:     c.Limit,
		Type:      c.T,
		PrintSize: c.DisplaySize,
		Textconv:  c.Textconv,
		Direct:    c.Direct,
		PrintJSON: c.JSON,
		Verify:    c.Verify,
		Output:    c.Output,
	})
}
