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
)

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

func GetUserAgent() string {
	return "Zeta/" + version
}

func GetBannerVersion() string {
	return "Zeta-" + version
}

// GetBuildTime returns the time at which the build took place
func GetBuildTime() string {
	return buildTime
}
