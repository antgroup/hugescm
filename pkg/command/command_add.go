// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/zeta"
)

// --chmod=(+|-)x

// Add file contents to the index
type Add struct {
	ALL      bool     `name:"all" short:"A" help:"Add changes from all tracked and untracked files"`
	DryRun   bool     `name:"dry-run" short:"n" help:"Dry run"`
	Update   bool     `name:"update" short:"u" help:"Update tracked files"`
	Chmod    string   `name:"chmod" help:"Override the executable bit of the listed files" placeholder:"(+|-)x"`
	PathSpec []string `arg:"" optional:"" name:"pathspec" help:"Path specification, similar to Git path matching mode"`
}

func (a *Add) Run(g *Globals) error {
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
	if a.ALL {
		if err := w.AddWithOptions(context.Background(), &zeta.AddOptions{All: true, DryRun: a.DryRun}); err != nil {
			diev("zeta add all error: %v\n", err)
			return err
		}
		return nil
	}
	switch a.Chmod {
	case "": // ignore
	case "+x":
		return w.Chmod(context.Background(), a.PathSpec, true, a.DryRun)
	case "-x":
		return w.Chmod(context.Background(), a.PathSpec, false, a.DryRun)
	default:
		diev("--chmod param '%s' must be either -x or +x\n", a.Chmod)
		return errors.New("bad chmod")
	}
	if a.Update {
		if err := w.AddTracked(context.Background(), slashPaths(a.PathSpec), a.DryRun); err != nil {
			diev("zeta add --update error: %v", err)
			return err
		}
		return nil
	}
	if len(a.PathSpec) == 0 {
		_, _ = term.Fprintf(os.Stderr, "%s\n\x1b[33m%s\x1b[0m\n",
			W("Nothing specified, nothing added."),
			W("hint: Maybe you wanted to say 'zeta add .'?"))
		return errors.New("nothing specified, nothing added")
	}
	if err := w.Add(context.Background(), slashPaths(a.PathSpec), a.DryRun); err != nil {
		fmt.Fprintf(os.Stderr, "zeta add error: %v\n", err)
		return err
	}
	return nil
}
