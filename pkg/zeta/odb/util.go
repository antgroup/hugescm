// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"os"
	"path/filepath"
)

var (
	subDirs = []struct {
		name  string
		isDir bool
	}{
		{"zeta.toml", false},
		{"metadata", true},
		//{"blob", true},
	}
)

func IsZetaDir(dir string) bool {
	for _, d := range subDirs {
		si, err := os.Stat(filepath.Join(dir, d.name))
		if err != nil {
			return false
		}
		if si.IsDir() != d.isDir {
			return false
		}
	}
	return true
}
