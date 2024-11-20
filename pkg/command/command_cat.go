// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Cat struct {
	Hash        string `arg:"" name:"object" help:"The name of the object to show"`
	WriteMax    int64  `name:"max" short:"m" optional:"" help:"Blob output size limit; use 0 for unlimited. Units: KB, MB, GB, K, M, G" default:"20m" type:"size"`
	T           bool   `name:"type" short:"t" help:"Show object type"`
	DisplaySize bool   `name:":" short:"s" help:"Show object size"`
	JSON        bool   `name:"json" short:"j" help:"Returns data as JSON; limited to commits, trees, fragments, and tags"`
	Textconv    bool   `name:"textconv" help:"Output text as Unicode; blobs only"`
	Verify      bool   `name:"verify" help:"Verify object hash"`
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
		Hash:        c.Hash,
		SizeMax:     c.WriteMax,
		Type:        c.T,
		DisplaySize: c.DisplaySize,
		Textconv:    c.Textconv,
		FormatJSON:  c.JSON,
		Verify:      c.Verify,
	})
}
