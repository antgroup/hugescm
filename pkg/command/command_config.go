// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Config struct {
	Args   []string `arg:"" name:"args" optional:"" help:"Name and value, support: <name value> appears in pairs or <name=value ...>, eg: zeta config K1=V1 K2=V2"`
	System bool     `name:"system" help:"Use system config file"`
	Global bool     `name:"global" help:"Only read or write to global ~/.zeta.toml"`
	Local  bool     `name:"local" help:"Only read or write to repository .zeta/zeta.toml, which is the default behavior when writing"`
	Unset  bool     `name:"unset" short:"u" help:"Remove the line matching the key from config file"`
	List   bool     `name:"list" short:"l" help:"List all variables set in config file, along with their values"`
	Get    bool     `name:"get" help:"Get the value for a given Key"`
	GetALL bool     `name:"get-all" help:"Get all values for a given Key"`
	Add    bool     `name:"add" help:"Add a new variable: name value"`
	Z      bool     `short:"z" shortonly:"" help:"Terminate values with NUL byte"`
	Type   string   `name:"type" short:"T" help:"zeta config will ensure that any input or output is valid under the given type constraint(s), support: bool, int, float, date" placeholder:"<type>"`
}

func (c *Config) Run(g *Globals) error {
	if c.List {
		if len(c.Args) != 0 {
			fmt.Fprintf(os.Stderr, "error: wrong number of arguments, should be 0\n")
			return errors.New("wrong number of arguments, should be 0")
		}
		return zeta.ListConfig(&zeta.ListConfigOptions{
			System: c.System,
			Global: c.Global,
			Local:  c.Local,
			Z:      c.Z,
			CWD:    g.CWD,
			Values: g.Values,
		})
	}
	if c.Get {
		return zeta.GetConfig(&zeta.GetConfigOptions{
			System: c.System,
			Global: c.Global,
			Local:  c.Local,
			Z:      c.Z,
			Keys:   c.Args,
			CWD:    g.CWD,
			Values: g.Values,
		})
	}
	if c.GetALL {
		return zeta.GetConfig(&zeta.GetConfigOptions{
			System: c.System,
			Global: c.Global,
			Local:  c.Local,
			ALL:    true,
			Z:      c.Z,
			Keys:   c.Args,
			CWD:    g.CWD,
			Values: g.Values,
		})
	}
	if c.Unset {
		return zeta.UnsetConfig(&zeta.UnsetConfigOptions{
			System: c.System,
			Global: c.Global,
			Keys:   c.Args,
			CWD:    g.CWD,
		})
	}
	if len(c.Args) == 1 {
		kv := c.Args[0]
		if strings.IndexByte(kv, '=') == -1 {
			return zeta.GetConfig(&zeta.GetConfigOptions{
				System: c.System,
				Global: c.Global,
				Local:  c.Local,
				Z:      c.Z,
				Keys:   c.Args,
				CWD:    g.CWD,
				Values: g.Values,
			})
		}
	}
	return zeta.UpdateConfig(&zeta.UpdateConfigOptions{
		System:        c.System,
		Global:        c.Global,
		Add:           c.Add,
		NameAndValues: c.Args,
		Type:          c.Type,
		CWD:           g.CWD,
	})
}
