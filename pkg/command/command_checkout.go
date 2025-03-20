// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Checkout struct {
	Branch          string   `name:"branch" short:"b" help:"Direct the new HEAD to the <name> branch after checkout" placeholder:"<branch>"`
	TagName         string   `name:"tag" short:"t" help:"Direct the new HEAD to the <name> tag's commit after checkout" placeholder:"<tag>"`
	Refname         string   `name:"refname" help:"Direct the new HEAD to the <name> ref's commit after checkout" placeholder:"<tag>"`
	Commit          string   `name:"commit" help:"Direct the new HEAD to the <commit> branch after checkout" placeholder:"<commit>"`
	Sparse          []string `name:"sparse" short:"s" help:"A subset of repository files, all files are checked out by default" placeholder:"<dir>"`
	Limit           int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Batch           bool     `name:"batch" help:"Get and checkout files for each provided on stdin"`
	Snapshot        bool     `name:"snapshot" help:"Checkout a non-editable snapshot"`
	Depth           int      `name:"depth" help:"Create a shallow clone with a history truncated to the specified number of commits" default:"1"`
	One             bool     `name:"one" help:"Checkout large files one after another"`
	Quiet           bool     `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
	Args            []string `arg:"" optional:""`
	passthroughArgs []string `kong:"-"`
}

const (
	coSummaryFormat = `%szeta checkout (co) [--branch|--tag] [--commit] [--sparse] [--limit] <url> [<destination>]
%szeta checkout (co) <branch>
%szeta checkout (co) [<branch>] -- <file>...
%szeta checkout (co) --batch [<branch>]
%szeta checkout (co) <something> [<paths>]`
)

func (c *Checkout) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(coSummaryFormat, W("Usage: "), or, or, or, or)
}

func (c *Checkout) Passthrough(paths []string) {
	c.passthroughArgs = append(c.passthroughArgs, paths...)
}

func (c *Checkout) doRemote(g *Globals, remote, destination string) error {
	if c.One && c.Limit != -1 {
		diev("--one is not compatible with --limit N")
		return ErrFlagsIncompatible
	}
	if len(c.TagName) != 0 && (len(c.Branch) != 0 || len(c.Commit) != 0) {
		diev("--tag is not compatible with --branch or --commit")
		return ErrFlagsIncompatible

	}
	r, err := zeta.New(context.Background(), &zeta.NewOptions{
		Remote:      remote,
		Branch:      c.Branch,
		TagName:     c.TagName,
		Refname:     c.Refname,
		Commit:      c.Commit,
		Destination: destination,
		SparseDirs:  c.Sparse,
		Snapshot:    c.Snapshot,
		SizeLimit:   c.Limit,
		Values:      g.Values,
		One:         c.One,
		Depth:       c.Depth,
		Quiet:       c.Quiet,
		Verbose:     g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.Postflight(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "postflight: prune objects error: %v\n", err)
		return err
	}
	return nil
}

func (c *Checkout) destination() string {
	if len(c.Args) >= 2 {
		return c.Args[1]
	}
	if len(c.passthroughArgs) > 0 {
		return c.passthroughArgs[0]
	}
	return ""
}

func (c *Checkout) revision() string {
	if len(c.Args) != 0 {
		return c.Args[0]
	}
	return "HEAD"
}

func (c *Checkout) runCompatibleCheckout0(g *Globals, r *zeta.Repository, worktreeOnly bool, branchName plumbing.ReferenceName, oid plumbing.Hash, pathSpec []string) error {
	w := r.Worktree()
	if len(pathSpec) != 0 {
		if err := w.DoPathCo(context.Background(), worktreeOnly, oid, pathSpec); err != nil {
			if oid, ok := plumbing.ExtractNoSuchObject(err); ok {
				fmt.Fprintf(os.Stderr, "zeta checkout: missing object: %s\ntry download it: zeta cat -t %s\n", oid, oid)
				return err
			}
			fmt.Fprintf(os.Stderr, "zeta checkout: checkout files error: %v\n", err)
			return err
		}
		return nil
	}
	g.DbgPrint("compatible checkout")
	opts := &zeta.CheckoutOptions{Branch: branchName, Merge: false, Force: false}
	if len(branchName) == 0 {
		opts.Hash = oid
	}
	if err := w.Checkout(context.Background(), opts); err != nil {
		if err != zeta.ErrAborting {
			target := string(branchName)
			if len(target) == 0 {
				target = oid.String()
			}
			diev("checkout to '%s' error: %v", target, err)
		}
		return err
	}
	return nil
}

func (c *Checkout) runCompatibleCheckout(g *Globals, r *zeta.Repository) error {
	pathSpec := make([]string, 0, len(c.Args))
	// zeta checkout <something> [<paths>]
	if len(c.Args) == 0 {
		pathSpec = append(pathSpec, c.passthroughArgs...)
		head, err := r.Current()
		if err != nil {
			diev("checkout resolve HEAD error: %v", err)
			return err
		}
		return c.runCompatibleCheckout0(g, r, true, head.Name(), head.Hash(), pathSpec)
	}
	rev, refname, err := r.RevisionEx(context.Background(), c.Args[0])
	if zeta.IsErrUnknownRevision(err) {
		pathSpec = append(pathSpec, c.Args...)
		pathSpec = append(pathSpec, c.passthroughArgs...)
		head, err := r.Current()
		if err != nil {
			return err
		}
		g.DbgPrint("resolve HEAD: %s", head.Name())
		return c.runCompatibleCheckout0(g, r, true, head.Name(), head.Hash(), slashPaths(pathSpec))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta checkout: resolve revision error: %v\n", err)
		return err
	}
	// zeta checkout <something> [<paths>]
	g.DbgPrint("resolve revision: %s", rev)
	pathSpec = append(pathSpec, c.Args[1:]...)
	pathSpec = append(pathSpec, c.passthroughArgs...)
	var worktreeOnly bool
	if len(pathSpec) != 0 {
		worktreeOnly = r.IsCurrent(refname)
	}
	return c.runCompatibleCheckout0(g, r, worktreeOnly, refname, rev, slashPaths(pathSpec))
}

func (c *Checkout) Run(g *Globals) error {
	if len(c.Args) > 0 && transport.IsRemoteEndpoint(c.Args[0]) {
		return c.doRemote(g, c.Args[0], c.destination())
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Verbose:  g.Verbose,
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
	if c.Batch {
		w := r.Worktree()
		if err := w.DoBatchCo(context.Background(), c.One, c.revision(), os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "zeta checkout --batch error: %v\n", err)
			return err
		}
		return nil
	}
	if c.One {
		diev("--one is not compatible with checkout revision or files")
		return ErrFlagsIncompatible
	}
	if err := c.runCompatibleCheckout(g, r); err != nil {
		return err
	}
	return nil
}
