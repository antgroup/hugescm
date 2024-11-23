//go:build windows

package env

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// initializeGW: detect git for windows installation
func initializeGW() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\GitForWindows`, registry.QUERY_VALUE)
	if err != nil {
		return "", nil
	}
	defer k.Close()
	installPath, _, err := k.GetStringValue("InstallPath")
	if err != nil {
		return "", err
	}
	gitForWindowsBinDir := filepath.Clean(filepath.Join(installPath, "cmd"))
	if _, err := os.Stat(filepath.Join(gitForWindowsBinDir, "git.exe")); err != nil {
		return "", err
	}
	return gitForWindowsBinDir, nil
}

// InitializeEnv: initialize path env
func InitializeEnv() error {
	pathEnv := os.Getenv("PATH")
	pathList := strings.Split(pathEnv, string(os.PathListSeparator))
	pathNewList := make([]string, 0, len(pathList)+2)
	if _, err := exec.LookPath("git"); err != nil {
		gitForWindowsBinDir, err := initializeGW()
		if err != nil {
			return err
		}
		pathNewList = append(pathNewList, filepath.Clean(gitForWindowsBinDir))
	}
	seen := make(map[string]bool)
	for _, p := range pathList {
		cleanedPath := filepath.Clean(p)
		if cleanedPath == "." {
			continue
		}
		u := strings.ToLower(cleanedPath)
		if seen[u] {
			continue
		}
		seen[u] = true
		pathNewList = append(pathNewList, cleanedPath)
	}
	os.Setenv("PATH", strings.Join(pathNewList, string(os.PathListSeparator)))
	return nil
}
