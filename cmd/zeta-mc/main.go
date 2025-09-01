// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/version"
)

func main() {
	// delay initialize git env
	_ = env.InitializeEnv()
	// initialize locale
	_ = tr.Initialize()
	kong.BindW(tr.W) // replace W
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
	)
	m := strengthen.NewMeasurer("zeta-mc", app.Debug)
	defer m.Close()
	err := ctx.Run(&app.Globals)
	if err != nil {
		os.Exit(1)
	}
}
