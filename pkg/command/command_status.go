// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Status struct {
	Short bool `name:"short" short:"s" help:"Give the output in the short-format"`
	Z     bool `short:"z" shortonly:"" help:"Terminate entries with NUL byte"`
}

func (s *Status) NewLine() byte {
	if s.Z {
		return '\x00'
	}
	return '\n'
}

// --show-stash

func (s *Status) Run(g *Globals) error {
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
	w.ShowFs(g.Verbose)
	shortFormat := s.Short || s.Z
	status, err := w.Status(context.Background(), !shortFormat)
	if err != nil {
		diev("status: %v", err)
		return err
	}
	if shortFormat {
		w.ShowStatus(status, true, s.Z)
		return nil
	}
	if status.IsClean() {
		fmt.Fprintln(os.Stderr, W("nothing to commit, working tree clean"))
		return nil
	}
	w.ShowStatus(status, false, s.Z)
	return nil
}
