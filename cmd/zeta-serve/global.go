// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

type Globals struct {
	Verbose   bool        `short:"V" name:"verbose" help:"Make the operation more talkative"`
	ExpandEnv bool        `short:"E" name:"expand-env" help:"Replaces $${var} or $$var in the config file according to the values of the current environment variables."`
	Version   VersionFlag `short:"v" name:"version" help:"Show version number and quit"`
}

type VersionFlag bool

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(version.GetVersionString())
	app.Exit(0)
	return nil
}
