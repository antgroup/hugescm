// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

//  Create, list, delete tag

type Tag struct {
	Annotate bool     `name:"annotate" short:"a" help:"Annotated tag, needs a message"`
	File     string   `name:"file" short:"F" help:"Take the tag message from the given file. Use - to read the message from the standard input"`
	Message  []string `name:"message" short:"m" help:"Use the given tag message (instead of prompting)"`
	List     bool     `name:"list" short:"l" help:"List tags. With optional <pattern>..."`
	Delete   bool     `name:"delete" short:"d" help:"Delete tags"`
	Force    bool     `name:"force" short:"f" help:"Replace the tag if exists"`
	Args     []string `arg:"" optional:"" name:"args" help:"Tag args: <tagname>, <pattern>, <start-point>"`
}

const (
	tagSumaryFormat = `%szeta tag [<options>] [-a] [-f] [-m <msg>] <tagname> [<start-point>]
%szeta tag [<options>] [-l] [<pattern>...]
%szeta tag [<options>] -d <tagname>...`
)

func (t *Tag) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(tagSumaryFormat, W("Usage: "), or, or)
}

func (t *Tag) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		diev("open repo: %v", err)
		return err
	}
	if t.List {
		return r.ListTag(context.Background(), t.Args)
	}
	if t.Delete {
		return r.RemoveTag(t.Args)
	}

	switch len(t.Args) {
	case 0:
		return r.ListTag(context.Background(), nil)
	case 1:
		return r.NewTag(context.Background(), &zeta.NewTagOptions{
			Name:     t.Args[0],
			Target:   "HEAD",
			Message:  t.Message,
			File:     t.File,
			Annotate: t.Annotate,
			Force:    t.Force,
		})
	default:
	}
	return r.NewTag(context.Background(), &zeta.NewTagOptions{
		Name:     t.Args[0],
		Target:   t.Args[1],
		Message:  t.Message,
		File:     t.File,
		Annotate: t.Annotate,
		Force:    t.Force,
	})
}
