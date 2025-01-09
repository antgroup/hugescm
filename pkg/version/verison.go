// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	version     string
	buildCommit string
	buildTime   string
	telemetry   string
)

func telemetryOn() bool {
	switch telemetry {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}

// GetVersionString returns a standard version header
func GetVersionString() string {
	return fmt.Sprintf("%s %v (%s), built %v", filepath.Base(os.Args[0]), version, buildCommit, buildTime)
}

func GetBuildCommit() string {
	return buildCommit
}

// GetVersion returns the semver compatible version number
func GetVersion() string {
	return version
}

func GetServerVersion() string {
	return "Zeta/" + version
}

func GetTelemeryUserAgent() string {
	if u, err := Uname(); err == nil {
		return fmt.Sprintf("Zeta/%s (%s; %s; %s)", version, u.Name, u.Machine, u.Release)
	}
	return "Zeta/" + version
}

func GetUserAgent() string {
	if telemetryOn() {
		return GetTelemeryUserAgent()
	}
	return "Zeta/" + version
}

func GetBannerVersion() string {
	if telemetryOn() {
		if u, err := Uname(); err == nil {
			// SSH-protoversion-softwareversion SP comments CR LF
			return fmt.Sprintf("ZETA-%s (%s; %s; %s)", version, u.Name, u.Machine, u.Release)
		}
	}
	return "ZETA-" + version
}

func GetServerBannerVersion() string {
	return "ZETA-" + version
}

// GetBuildTime returns the time at which the build took place
func GetBuildTime() string {
	return buildTime
}
