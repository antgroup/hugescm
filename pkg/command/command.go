// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"errors"
	"fmt"

	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

type Globals struct {
	Verbose bool        `short:"V" name:"verbose" help:"Make the operation more talkative"`
	Version VersionFlag `short:"v" name:"version" help:"Show version number and quit"`
	Values  []string    `short:"X" shortonly:"" help:"Override default configuration, format: <key>=<value>"`
	CWD     string      `name:"cwd" help:"Set the path to the repository worktree" placeholder:"<worktree>"`
}

type VersionFlag bool

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(version.GetVersionString())
	app.Exit(0)
	return nil
}

var (
	ErrArgRequired = errors.New("arg required")
)
