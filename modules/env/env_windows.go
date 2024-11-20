//go:build windows

package env

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// initializeGW todo
func initializeGW() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\GitForWindows`, registry.QUERY_VALUE)
	if err != nil {
		if k, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\GitForWindows`, registry.QUERY_VALUE); err != nil {
			return "", err
		}
	}
	defer k.Close()
	installPath, _, err := k.GetStringValue("InstallPath")
	if err != nil {
		return "", err
	}
	installPath = filepath.Clean(installPath)
	git := filepath.Join(installPath, "cmd\\git.exe")
	if _, err := os.Stat(git); err != nil {
		return "", err
	}
	return filepath.Join(installPath, "cmd"), nil
}

// InitializeEnv todo
func InitializeEnv() error {
	if _, err := exec.LookPath("git"); err == nil {
		return nil
	}
	gitBinDir, err := initializeGW()
	if err != nil {
		return err
	}
	pathEnv := os.Getenv("PATH")
	pathList := strings.Split(pathEnv, string(os.PathListSeparator))
	pathNewList := make([]string, 0, len(pathList)+2)
	pathNewList = append(pathNewList, filepath.Clean(gitBinDir))
	for _, s := range pathList {
		cleanedPath := filepath.Clean(s)
		if cleanedPath == "." {
			continue
		}
		pathNewList = append(pathNewList, cleanedPath)
	}
	os.Setenv("PATH", strings.Join(pathNewList, string(os.PathListSeparator)))
	return nil
}
