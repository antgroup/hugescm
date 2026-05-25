// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/version"
)

func main() {
	// delay initialize git env
	_ = env.DelayInitializeEnv()
	// initialize locale
	_ = tr.Initialize()
	kong.BindW(tr.W) // replace W

	// A cancellable root context lets long-running operations (migrate,
	// etc.) react to Ctrl+C / SIGTERM. stop() detaches the signal
	// handlers when main returns.
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var app App
	ctx := kong.Parse(&app,
		kong.Name("zeta-mc"),
		kong.Description(tr.W("zeta-mc - Migrate Git repository to zeta")),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
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
	m := strengthen.NewMeasurer("zeta-mc", app.Debug)
	defer m.Close()
	err := ctx.Run(&app.Globals)
	if err != nil {
		os.Exit(1)
	}
}
