// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

type App struct {
	Globals
	HTTPD   HTTPD   `cmd:"httpd" help:"start zeta-serve httpd server"`
	SSHD    SSHD    `cmd:"sshd" help:"start zeta-serve sshd server"`
	Keygen  Keygen  `cmd:"keygen" help:"Generates a random private key"`
	Encrypt Encrypt `cmd:"encrypt" help:"Encrypting Data Using RSA Key"`
}

func main() {
	var app App
	ctx := kong.Parse(&app,
		kong.Name("zeta-serve"),
		kong.Description("HugeSCM - A next generation cloud-based version control system"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version.GetVersionString(),
		},
	)
	now := time.Now()
	if app.Verbose {
		trace.EnableDebugMode()
	}
	err := ctx.Run(&app.Globals)
	if app.Verbose {
		trace.DbgPrint("time spent: %v", time.Since(now))
	}
	if err != nil {
		//fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
