// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type LsTree struct {
	OnlyTrees bool     `short:"d" shortonly:"" help:"Only show trees"`
	Recurse   bool     `short:"r" shortonly:"" help:"Recurse into subtrees"`
	Tree      bool     `short:"t" shortonly:"" help:"Show trees when recursing"`
	Z         bool     `short:"z" shortonly:"" help:"Terminate entries with NUL byte"`
	Long      bool     `name:"long" short:"l" help:"Include object size"`
	NameOnly  bool     `name:"name-only" alias:"name-status" help:"List only filenames"`
	Abbrev    int      `name:"abbrev" help:"Use <n> digits to display object names" placeholder:"<n>"`
	JSON      bool     `name:"json" short:"j" help:"Data will be returned in JSON format"`
	Revision  string   `arg:"" name:"tree-ish" help:"ID of a tree-ish"`
	Paths     []string `arg:"" name:"path" optional:"" help:"Given paths, show as match patterns; else, use root as sole argument"`
}

func (c *LsTree) NewLine() byte {
	if c.Z {
		return '\x00'
	}
	return '\n'
}

// List the contents of a tree object
func (c *LsTree) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint

	if err := r.LsTree(context.Background(), &zeta.LsTreeOptions{
		OnlyTrees: c.OnlyTrees,
		Recurse:   c.Recurse,
		Tree:      c.Tree,
		NewLine:   c.NewLine(),
		Long:      c.Long,
		NameOnly:  c.NameOnly,
		Abbrev:    c.Abbrev,
		Revision:  c.Revision,
		Paths:     slashPaths(c.Paths),
		JSON:      c.JSON,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "zeta ls-tree error: %v\n", err)
		return err
	}
	return nil
}
