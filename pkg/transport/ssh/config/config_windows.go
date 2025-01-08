//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"
)

// systemConfigFinder return C:\ProgramData\ssh\ssh_config on windows
func systemConfigFinder() string {
	return os.ExpandEnv("${ProgramData}\\ssh\\ssh_config")
}

func systemConfigDir() string {
	return os.ExpandEnv("${ProgramData}\\ssh")
}

func isSystem(filename string) bool {
	programData := os.Getenv("ProgramData")
	if strings.HasSuffix(programData, "\\") {
		programData += "\\"
	}
	return strings.HasPrefix(filepath.Clean(filename), programData)
}
