// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/antgroup/hugescm/cmd/hot/command"
	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

type App struct {
	command.Globals
	Cat        command.Cat        `cmd:"cat" help:"Provide contents or details of repository objects"`
	Stat       command.Stat       `cmd:"stat" help:"View repository status"`
	Size       command.Size       `cmd:"size" help:"Show repositories size and large files"`
	Remove     command.Remove     `cmd:"remove" help:"Remove files in repository and rewrite history"`
	Smart      command.Smart      `cmd:"smart" help:"Interactive mode to clean repository large files"`
	Graft      command.Graft      `cmd:"graft" help:"Interactive mode to clean repository large files (Grafting mode)"`
	Mc         command.Mc         `cmd:"mc" help:"Migrate a repository to the specified object format"`
	Unbranch   command.Unbranch   `cmd:"unbranch" help:"Linearize repository history"`
	PruneRefs  command.PruneRefs  `cmd:"prune-refs" help:"Prune refs by prefix"`
	ScanRefs   command.ScanRefs   `cmd:"scan-refs" help:"Scan references in a local repository"`
	ExpireRefs command.ExpireRefs `cmd:"expire-refs" help:"Clean up expired references"`
	Snapshot   command.Snapshot   `cmd:"snapshot" help:"Create a snapshot commit for the worktree"`
	Az         command.Az         `cmd:"az" help:"Analyze repository large files"`
	Co         command.Co         `cmd:"co" help:"EXPERIMENTAL: Clones a repository into a newly created directory"`
	Diff       command.Diff       `cmd:"diff" help:"Show changes between commits, commit and working tree, etc"`
	Show       command.Show       `cmd:"show" help:"Show the changes introduced by a commit"`
	Debug      bool               `name:"debug" help:"Enable debug mode; analyze timing"`
}

// run is the real entry point. It returns the process exit code so that
// deferred clean-ups (Measurer.Close, signal.stop, etc.) run before exit.
// Using os.Exit directly inside main would skip every deferred call.
func run() int {
	// delay initialize git env
	_ = env.DelayInitializeEnv()
	// initialize locale
	_ = tr.DelayInitializeLocale()
	kong.BindW(tr.W) // replace W

	// A cancellable root context lets long-running operations (replay,
	// migrate, prune) react to Ctrl+C / SIGTERM. stop() detaches the
	// signal handlers when run returns.
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var app App
	ctx := kong.Parse(&app,
		kong.NamedMapper("size", command.SizeDecoder()),
		kong.NamedMapper("expire", command.ExpireDecoder()),
		kong.Name("hot"),
		kong.Description(tr.W("hot - Git repositories maintenance tool")),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}),
		kong.Vars{
			"version": version.GetVersionString(),
		},
		// Inject rootCtx so each subcommand's Run(ctx context.Context, ...)
		// receives the cancellable context automatically.
		kong.BindTo(rootCtx, (*context.Context)(nil)),
	)
	if app.Verbose {
		trace.EnableDebugMode()
	}

	m := strengthen.NewMeasurer("hot", app.Debug)
	defer m.Close()

	if err := ctx.Run(&app.Globals); err != nil {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
