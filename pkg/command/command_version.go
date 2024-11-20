// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/antgroup/hugescm/pkg/version"
)

type Version struct {
	BuildOptions bool `name:"build-options" help:"Also print build options"`
	JSON         bool `short:"j" name:"json" help:"Data will be returned in JSON format"`
}

func (c *Version) formatJSON() error {
	m := map[string]string{
		"version": version.GetVersion(),
		"commit":  version.GetBuildCommit(),
		"time":    version.GetBuildTime(),
		"arch":    runtime.GOARCH,
		"os":      runtime.GOOS,
	}
	if c.BuildOptions {
		if info, ok := debug.ReadBuildInfo(); ok {
			m["go_version"] = strings.TrimPrefix(info.GoVersion, "go")
			for _, s := range info.Settings {
				if len(s.Value) == 0 {
					continue
				}
				m[s.Key] = s.Value
			}
		}
	}
	return json.NewEncoder(os.Stdout).Encode(m)
}

func (c *Version) Run(g *Globals) error {
	if c.JSON {
		return c.formatJSON()
	}
	fmt.Fprintf(os.Stdout, "zeta %s (%s), built %v\n", version.GetVersion(), version.GetBuildCommit(), version.GetBuildTime())
	if !c.BuildOptions {
		return nil
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	fmt.Fprintf(os.Stdout, "arch: %s\nos:   %s\ngo:   %s\n", runtime.GOARCH, runtime.GOOS, strings.TrimPrefix(info.GoVersion, "go"))
	for _, s := range info.Settings {
		if len(s.Value) == 0 {
			continue
		}
		fmt.Fprintf(os.Stdout, "%s:\n  %s\n", s.Key, s.Value)
	}
	return nil
}
