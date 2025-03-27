// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Commit struct {
	Message           []string `name:"message" short:"m" help:"Use the given as the commit message. Concatenate multiple -m options as separate paragraphs" placeholder:"<message>"`
	File              string   `name:"file" short:"F" help:"Take the commit message from the given file. Use - to read the message from the standard input" placeholder:"<file>"`
	All               bool     `name:"all" short:"a" help:"Automatically stage modified and deleted files, but newly untracked files remain unaffected"`
	AllowEmpty        bool     `name:"allow-empty" help:"Allow creating a commit with the exact same tree structure as its parent commit"`
	AllowEmptyMessage bool     `name:"allow-empty-message" help:"Like --allow-empty this command is primarily for use by foreign SCM interface scripts"`
	Amend             bool     `name:"amend" help:"Replace the tip of the current branch by creating a new commit"`
}

func (c *Commit) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	w := r.Worktree()
	opts := &zeta.CommitOptions{
		All:               c.All,
		AllowEmptyCommits: c.AllowEmpty,
		AllowEmptyMessage: c.AllowEmptyMessage,
		Amend:             c.Amend,
		Message:           c.Message,
		File:              c.File,
	}
	oid, err := w.Commit(context.Background(), opts)
	if err != nil {
		switch err {
		case zeta.ErrMissingAuthor:
			fmt.Fprintf(os.Stderr, `zeta commit: %s
%s

%s

    zeta config --global user.email "you@example.com"
    zeta config --global user.name "Your Name"

%s
%s
`, W("Author identity unknown"),
				W("*** Please tell me who you are."),
				W("Run"),
				W("to set your account's default identity."),
				W("Omit --global to set the identity only in this repository."))
			return err
		case zeta.ErrNotAllowEmptyMessage:
			fmt.Fprintln(os.Stderr, W("Aborting commit due to empty commit message."))
			return err
		case zeta.ErrNoChanges:
			fmt.Fprintln(os.Stderr, W("nothing to commit, working tree clean"))
			return err
		case zeta.ErrNothingToCommit:
			return err
		default:
			fmt.Fprintf(os.Stderr, "zeta commit error: %v\n", err)
			return err
		}
	}
	w.DbgPrint("create commit: %s\n", oid.String())
	return w.Stats(context.Background())
}
