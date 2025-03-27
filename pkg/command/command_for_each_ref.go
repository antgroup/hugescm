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

func (c *ForEachRef) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint

	return r.ForEachReference(context.Background(), &zeta.ForEachReferenceOptions{
		FormatJSON: c.JSON,
		Order:      c.Sort,
		Pattern:    c.Pattern,
	})
}
