// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

type Globals struct {
	Verbose   bool        `short:"V" name:"verbose" help:"Make the operation more talkative"`
	ExpandEnv bool        `short:"E" name:"expand-env" help:"Replaces $${var} or $$var in the config file according to the values of the current environment variables."`
	Version   VersionFlag `short:"v" name:"version" help:"Show version number and quit"`
}

func (g *Globals) DbgPrint(format string, args ...any) {
	if !g.Verbose {
		return
	}
	message := strings.TrimSuffix(fmt.Sprintf(format, args...), "\n")
	var buffer bytes.Buffer
	for _, s := range strings.Split(message, "\n") {
		_, _ = buffer.WriteString("\x1b[33m* ")
		_, _ = buffer.WriteString(s)
		_, _ = buffer.WriteString("\x1b[0m\n")
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}

type VersionFlag bool

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(version.GetVersionString())
	app.Exit(0)
	return nil
}

type Debuger interface {
	DbgPrint(format string, args ...any)
}
