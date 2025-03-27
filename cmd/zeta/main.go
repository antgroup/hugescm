// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/pkg/command"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/version"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type App struct {
	command.Globals
	Checkout    command.Checkout    `cmd:"checkout" aliases:"co" help:"Checkout remote, switch branches, or restore worktree files"`
	Switch      command.Switch      `cmd:"switch" help:"Switch branches"`
	Add         command.Add         `cmd:"add" help:"Add file contents to the index"`
	Status      command.Status      `cmd:"status" help:"Show the working tree status"`
	Restore     command.Restore     `cmd:"restore" help:"Restore working tree files"`
	Fetch       command.Fetch       `cmd:"fetch" help:"Download objects and reference from remote"`
	Commit      command.Commit      `cmd:"commit" help:"Record changes to the repository"`
	Push        command.Push        `cmd:"push" help:"Update remote refs along with associated objects"`
	Branch      command.Branch      `cmd:"branch" help:"List, create, or delete branches"`
	Tag         command.Tag         `cmd:"tag" help:"List, create, or delete tags"`
	Pull        command.Pull        `cmd:"pull" help:"Fetch from and integrate with remote"`
	Merge       command.Merge       `cmd:"merge" help:"Join two development histories together"`
	Rebase      command.Rebase      `cmd:"rebase" help:"Reapply commits on top of another base tip"`
	Config      command.Config      `cmd:"config" help:"Get and set repository or global options"`
	CatFile     command.Cat         `cmd:"cat-file" aliases:"cat" help:"Provide contents or details of repository objects"`
	Log         command.Log         `cmd:"log" help:"Show commit logs"`
	GC          command.GC          `cmd:"gc" help:"Cleanup unnecessary files and optimize the local repository"`
	Reset       command.Reset       `cmd:"reset" help:"Reset current HEAD to the specified state"`
	Diff        command.Diff        `cmd:"diff" help:"Show changes between commits, commit and working tree, etc"`
	Clean       command.Clean       `cmd:"clean" help:"Remove untracked files from the working tree"`
	LsTree      command.LsTree      `cmd:"ls-tree" help:"List the contents of a tree object"`
	MergeTree   command.MergeTree   `cmd:"merge-tree" help:"Perform merge without touching index or working tree"`
	RM          command.Remove      `cmd:"rm" help:"Remove files from the working tree and from the index"`
	Stash       command.Stash       `cmd:"stash" help:"Stash the changes in a dirty working directory away"`
	RevParse    command.RevParse    `cmd:"rev-parse" help:"Pick out and massage parameters"`
	ForEachRef  command.ForEachRef  `cmd:"for-each-ref" help:"Output information on each ref"`
	Remote      command.Remote      `cmd:"remote" help:"Manage of tracked repository"`
	CheckIgnore command.CheckIgnore `cmd:"check-ignore" help:"Debug zetaignore / exclude files"`
	Init        command.Init        `cmd:"init" help:"Create an empty zeta repository"`
	MergeBase   command.MergeBase   `cmd:"merge-base" help:"Find optimal common ancestors for merge"`
	LsFiles     command.LsFiles     `cmd:"ls-files" help:"Show information about files in the index and the working tree"`
	HashObject  command.HashObject  `cmd:"hash-object" help:"Compute hash or create object"`
	MergeFile   command.MergeFile   `cmd:"merge-file" help:"Run a three-way file merge"`
	Show        command.Show        `cmd:"show" help:"Show various types of objects"`
	Version     command.Version     `cmd:"version" help:"Display version information"`
	Debug       bool                `name:"debug" help:"Enable debug mode; analyze timing"`
}

type Tracer struct {
	closeFn func()
}

func NewTracer(debugMode bool) *Tracer {
	d := &Tracer{}
	if !debugMode {
		return d
	}
	pprofName := filepath.Join(os.TempDir(), fmt.Sprintf("zeta-%d.pprof", os.Getpid()))
	fd, err := os.Create(pprofName)
	if err != nil {
		return d
	}
	if err = pprof.StartCPUProfile(fd); err != nil {
		_ = fd.Close()
		return d
	}
	d.closeFn = func() {
		pprof.StopCPUProfile()
		_ = fd.Close()
		fmt.Fprintf(os.Stderr, "Task operation completed\ngo tool pprof -http=\":8080\" %s\n", pprofName)
	}
	return d
}

func (d *Tracer) Close() {
	if d.closeFn != nil {
		d.closeFn()
	}
}

func main() {
	_ = env.InitializeEnv()
	// initialize locale
	_ = tr.Initialize()
	kong.BindW(tr.W) // replace W
	var app App
	ctx := kong.Parse(&app,
		kong.NamedMapper("size", command.SizeDecoder()),
		kong.NamedMapper("expiry-date", command.ExpiryDateDecoder()),
		kong.Name("zeta"),
		kong.Description(tr.W("HugeSCM - A next generation cloud-based version control system")),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}),
		kong.Vars{
			"version": version.GetVersionString(),
		},
	)
	now := time.Now()
	t := NewTracer(app.Debug)
	err := ctx.Run(&app.Globals)
	t.Close()
	if app.Verbose {
		app.DbgPrint("time spent: %v", time.Since(now))
	}
	if err == nil {
		return
	}
	if e, ok := err.(*zeta.ErrExitCode); ok {
		os.Exit(e.ExitCode)
	}
	os.Exit(127)
}
