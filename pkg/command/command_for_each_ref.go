// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// Output information on each ref

type ForEachRef struct {
	JSON    bool     `name:"json" short:"j" help:"Data will be returned in JSON format"`
	Sort    string   `name:"sort" help:"Field name to sort on" placeholder:"<order>"`
	Pattern []string `arg:"" optional:"" name:"pattern" help:"If given, only refs matching at least one pattern are shown"`
}

func (c *ForEachRef) Run(ctx context.Context, g *Globals) error {
	r, err := zeta.Open(ctx, &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint

	return r.ForEachReference(ctx, &zeta.ForEachReferenceOptions{
		FormatJSON: c.JSON,
		Order:      c.Sort,
		Pattern:    c.Pattern,
	})
}
