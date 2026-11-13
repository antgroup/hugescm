// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"errors"
	"fmt"

	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

type Globals struct {
	Verbose bool        `short:"V" name:"verbose" help:"Make the operation more talkative"`
	Version VersionFlag `short:"v" name:"version" help:"Show version number and quit"`
	Values  []string    `short:"X" shortonly:"" help:"Override default configuration, format: <key>=<value>"`
	CWD     string      `name:"cwd" help:"Set the path to the repository worktree" placeholder:"<worktree>"`
	Color   string      `name:"color" default:"auto" placeholder:"<when>" help:"When to colorize output: \"auto\" (detect from the terminal, default), \"always\" (force colors even when piped) or \"never\""`
}

// ApplyColorOverride applies the --color tri-state to the detected terminal
// color levels. It must be called after flag parsing (so g.Color is set)
// and before any output-producing command runs, since the levels are read
// lazily by the printers, the diff encoders and the log renderer.
//
// Resolution order: an explicit --color flag overrides the FORCE_COLOR and
// NO_COLOR environment variables (those are applied during modules/term
// package initialization, which runs before flag parsing).
func (g *Globals) ApplyColorOverride() error {
	switch g.Color {
	case "", "auto":
		return nil
	case "always":
		// Follow the environment when it advertises a level, but never
		// sink below 256 colors: the user explicitly asked for colors,
		// e.g. to keep them through a pager or into a CI log.
		level := term.DetectColorLevel()
		if level < term.Level256 {
			level = term.Level256
		}
		term.StdoutLevel = level
		term.StderrLevel = level
	case "never":
		term.StdoutLevel = term.LevelNone
		term.StderrLevel = term.LevelNone
	default:
		return fmt.Errorf("unsupported --color value %q (supported: always, never, auto)", g.Color)
	}
	return nil
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
