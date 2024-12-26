// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Switch struct {
	Args           []string `arg:"" optional:"" help:"Branch to switch to and start-point"`
	Create         bool     `name:"create" short:"c" help:"Create a new branch named <branch> starting at <start-point> before switching to the branch"`
	ForceCreate    bool     `name:"force-create" short:"C" help:"Similar to --create except that if <branch> already exists, it will be reset to <start-point>"`
	Detach         bool     `name:"detach" help:"Switch to a commit for inspection and discardable experiments"`
	Orphan         bool     `name:"orphan" help:"Create a new orphan branch, named <new-branch>. All tracked files are removed"`
	DiscardChanges bool     `name:"discard-changes" help:"Proceed even if the index or the working tree differs from HEAD"`
	Force          bool     `name:"force" short:"f" help:"An alias for --discard-changes"`
	Merge          bool     `name:"merge" short:"m" negatable:"" default:"true" help:"Perform a 3-way merge with the new branch"`
	Remote         bool     `name:"remote" help:"Attempt to checkout from remote when branch is absent"`
	Limit          int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Quiet          bool     `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
}

const (
	switchSummaryFormat = `%szeta switch [<options>] <branch> [--remote]
%szeta switch [<options>] --detach [<start-point>]
%szeta switch [<options>] (-c|-C) <new-branch> [<start-point>]
%szeta switch [<options>] --orphan <new-branch>
%szeta switch [<options>] --remote <new-branch> <start-point>`
)

func (s *Switch) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(switchSummaryFormat, W("Usage: "), or, or, or, or)
}

func (s *Switch) Discard() bool {
	return s.Force || s.DiscardChanges
}

func (s *Switch) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
		Quiet:    s.Quiet,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	if len(s.Args) == 0 {
		die("missing branch or commit argument")
		return ErrArgRequired
	}
	branchOrBasePoint := s.Args[0]
	basePoint := "HEAD"
	if len(s.Args) >= 2 {
		basePoint = s.Args[1]
	}
	so := &zeta.SwitchOptions{Force: s.Discard(), Merge: s.Merge, ForceCreate: s.ForceCreate, Remote: s.Remote, Limit: s.Limit}
	if err := so.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "zeta switch to '%s' error: %v\n", basePoint, err)
		return err
	}
	if s.Create || s.ForceCreate {
		return r.SwitchNewBranch(context.Background(), branchOrBasePoint, basePoint, so)
	}
	if s.Detach {
		return r.SwitchDetach(context.Background(), branchOrBasePoint, so)
	}
	if s.Orphan {
		return r.SwitchOrphan(context.Background(), branchOrBasePoint, so)
	}
	return r.SwitchBranch(context.Background(), branchOrBasePoint, so)
}
