// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"

	"github.com/antgroup/hugescm/cmd/hot/command"
	"github.com/antgroup/hugescm/cmd/hot/tr"
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
	Unbranch   command.Unbranch   `cmd:"unbranch " help:"Linearize repository history"`
	PruneRefs  command.PruneRefs  `cmd:"prune-refs" help:"Prune refs by prefix"`
	ScanRefs   command.ScanRefs   `cmd:"scan-refs" help:"Scan references in a local repository"`
	ExpireRefs command.ExpireRefs `cmd:"expire-refs" help:"Clean up expired references"`
	Snapshot   command.Snapshot   `cmd:"snapshot" help:"Create a snapshot commit for the worktree"`
	Co         command.Co         `cmd:"co" help:"Clones a repository into a newly created directory"`
	Az         command.Az         `cmd:"az" help:"Analyze repository large files"`
	Debug      bool               `name:"debug" help:"Enable debug mode; analyze timing"`
}

func main() {
	// delay initilaize git env
	_ = env.InitializeEnv()
	// initialize locale
	_ = tr.DelayInitializeLocale()
	kong.BindW(tr.W) // replace W
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
	)
	if app.Verbose {
		trace.EnableDebugMode()
	}
	m := strengthen.NewMeasurer("hot", app.Debug)
	defer m.Close()
	err := ctx.Run(&app.Globals)
	if err != nil {
		os.Exit(1)
	}
}
