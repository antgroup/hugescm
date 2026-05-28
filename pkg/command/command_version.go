// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
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

type versionInfo struct {
	Version    string            `json:"version"`
	Commit     string            `json:"commit"`
	Time       string            `json:"time"`
	Arch       string            `json:"arch"`
	OS         string            `json:"os"`
	GoVersion  string            `json:"go_version,omitempty"`
	BuildFlags map[string]string `json:"build_flags,omitempty"`
}

func (c *Version) formatJSON() error {
	info := versionInfo{
		Version: version.GetVersion(),
		Commit:  version.GetBuildCommit(),
		Time:    version.GetBuildTime(),
		Arch:    runtime.GOARCH,
		OS:      runtime.GOOS,
	}
	if c.BuildOptions {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			info.GoVersion = strings.TrimPrefix(buildInfo.GoVersion, "go")
			for _, s := range buildInfo.Settings {
				if len(s.Value) == 0 {
					continue
				}
				if info.BuildFlags == nil {
					info.BuildFlags = make(map[string]string)
				}
				info.BuildFlags[s.Key] = s.Value
			}
		}
	}
	return json.NewEncoder(os.Stdout).Encode(info)
}

func (c *Version) Run(ctx context.Context, g *Globals) error {
	if c.JSON {
		return c.formatJSON()
	}
	_, _ = fmt.Fprintf(os.Stdout, "zeta %s (%s), built %v\n", version.GetVersion(), version.GetBuildCommit(), version.GetBuildTime())
	if !c.BuildOptions {
		return nil
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	_, _ = fmt.Fprintf(os.Stdout, "arch: %s\nos:   %s\ngo:   %s\n", runtime.GOARCH, runtime.GOOS, strings.TrimPrefix(info.GoVersion, "go"))
	for _, s := range info.Settings {
		if len(s.Value) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s:\n  %s\n", s.Key, s.Value)
	}
	return nil
}
