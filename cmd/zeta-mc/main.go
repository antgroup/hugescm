// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/version"
)

type Debugger struct {
	closeFn func()
}

func NewDebugger(debugMode bool) *Debugger {
	d := &Debugger{}
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
		fd.Close()
		fmt.Fprintf(os.Stderr, "Task operation completed\ngo tool pprof -http=\":8080\" %s\n", pprofName)
	}
	return d
}

func (d *Debugger) Close() {
	if d.closeFn != nil {
		d.closeFn()
	}
}

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
	d := NewDebugger(app.Debug)
	defer d.Close()
	err := ctx.Run(&app.Globals)
	if err != nil {
		os.Exit(1)
	}
}
