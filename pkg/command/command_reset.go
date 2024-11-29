// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/zeta"
)

// Reset current HEAD to the specified state
type Reset struct {
	Revision string   `arg:"" optional:"" name:"commit" help:"Resets the current branch head to <commit>"`
	Mixed    bool     `name:"mixed" help:"Reset HEAD and index"`
	Soft     bool     `name:"soft" help:"Reset only HEAD"`
	Hard     bool     `name:"hard" help:"Reset HEAD, index and working tree, changes discarded"`
	Merge    bool     `name:"merge" help:"Reset HEAD, index and working tree"`
	Keep     bool     `name:"keep" help:"Reset HEAD but keep local changes"`
	Fetch    bool     `name:"fetch" help:"Fetch missing objects"`
	One      bool     `name:"one" help:"Checkout large files one after another, --hard mode only"`
	Limit    int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Quiet    bool     `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
	paths    []string `kong:"-"`
}

// SYNOPSIS
//        zeta reset [-q] [<tree-ish>] -- <pathspec>...
//        zeta reset [--soft | --mixed [-N] | --hard | --merge | --keep] [-q] [<commit>]

const (
	resetSummaryFormat = `%szeta reset [-q] [<tree-ish>] -- <pathspec>...
%szeta reset [--soft | --mixed [-N] | --hard | --merge | --keep] [--fetch] [-q] [<commit>]`
)

func (c *Reset) Summary() string {
	return fmt.Sprintf(resetSummaryFormat, W("Usage: "), W("   or: "))
}

func (c *Reset) ResetMode() zeta.ResetMode {
	if c.Soft || c.Keep {
		return zeta.SoftReset
	}
	if c.Hard {
		return zeta.HardReset
	}
	if c.Merge {
		return zeta.MergeReset
	}
	return zeta.MixedReset
}

func (c *Reset) Passthrough(paths []string) {
	c.paths = append(c.paths, paths...)
}

func (c *Reset) validateFlags() string {
	if len(c.paths) == 0 {
		return ""
	}
	if c.Hard {
		return "hard"
	}
	if c.Keep {
		return "keep"
	}
	if c.Merge {
		return "merge"
	}
	if c.Mixed {
		return "mixed"
	}
	if c.Soft {
		return "soft"
	}
	return ""
}

func (c *Reset) resetAutoFetch(w *zeta.Worktree) error {
	oid, err := w.Prefetch(context.Background(), c.Revision, c.Limit, c.One)
	if err != nil {
		return err
	}
	if err := w.Reset(context.Background(), &zeta.ResetOptions{Commit: oid, Mode: c.ResetMode(), Fetch: c.Fetch, One: c.One, Quiet: c.Quiet}); err != nil {
		fmt.Fprintf(os.Stderr, "zeta reset to %s error: %v\n", oid, err)
		return err
	}
	return nil
}

func (c *Reset) Run(g *Globals) error {
	if action := c.validateFlags(); len(action) != 0 {
		diev("cannot %s reset with paths.", action)
		return ErrFlagsIncompatible
	}
	if c.One && !c.Hard {
		diev("--one required --hard")
		return ErrArgRequired
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
		Quiet:    c.Quiet,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Postflight(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "postflight: prune objects error: %v\n", err)
		}
		r.Close()
	}()
	w := r.Worktree()

	if len(c.Revision) == 0 {
		c.Revision = string(plumbing.HEAD)
	}

	if c.Fetch || c.One {
		return c.resetAutoFetch(w)
	}

	oid, err := r.Revision(context.Background(), c.Revision)
	if plumbing.IsNoSuchObject(err) {
		fmt.Fprintf(os.Stderr, "zeta reset: %s not found\n", c.Revision)
		return err
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta reset: resolve [%s] error: %v\n", c.Revision, err)
		return err
	}
	if oid.IsZero() {
		fmt.Fprintf(os.Stderr, "zeta reset: resolve [%s] error: no such revision\n", c.Revision)
		return errors.New("no such revision")
	}
	if len(c.paths) != 0 {
		if err := w.ResetSpec(context.Background(), oid, slashPaths(c.paths)); err != nil {
			fmt.Fprintf(os.Stderr, "zeta reset: error: %v\n", err)
			return err
		}
		return nil
	}
	if err := w.Reset(context.Background(), &zeta.ResetOptions{Commit: oid, Mode: c.ResetMode(), Fetch: c.Fetch, One: c.One, Quiet: c.Quiet}); err != nil {
		fmt.Fprintf(os.Stderr, "zeta reset to %s error: %v\n", oid, err)
		return err
	}
	return nil
}
