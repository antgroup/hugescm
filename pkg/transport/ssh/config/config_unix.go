//go:build !windows

package config

import (
	"path/filepath"
	"strings"
)

// SystemConfigFinder return ~/etc/ssh/ssh_config on unix,
func SystemConfigFinder() string {
	return filepath.Join("/", "etc", "ssh", "ssh_config")
}

func systemConfigDir() string {
	return filepath.Join("/", "etc", "ssh")
}

func isSystem(filename string) bool {
	// TODO: not sure this is the best way to detect a system repo
	return strings.HasPrefix(filepath.Clean(filename), "/etc/ssh/")
}
