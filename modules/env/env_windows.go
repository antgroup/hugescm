//go:build windows

package env

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func searchGitForWindows() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\GitForWindows`, registry.QUERY_VALUE)
	if err != nil {
		return "", nil
	}
	defer k.Close() // nolint
	installPath, _, err := k.GetStringValue("InstallPath")
	if err != nil {
		return "", err
	}
	return installPath, nil
}

func hasGitExe(installDir string) bool {
	gitExe := filepath.Join(installDir, "cmd", "git.exe")
	if _, err := os.Stat(gitExe); err != nil {
		return false
	}
	return true
}

func checkLessExe(installDir string) {
	lessExe := filepath.Join(installDir, "usr", "bin", "less.exe")
	if _, err := os.Stat(lessExe); err == nil {
		_ = os.Setenv(ZETA_LESS_EXE_HIJACK, lessExe)
	}
}

func cleanupEnv(pathList []string) {
	pathNewList := make([]string, 0, len(pathList)+2)
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
	_ = os.Setenv("PATH", strings.Join(pathNewList, string(os.PathListSeparator)))
}

// InitializeEnv: initialize path env
func InitializeEnv() error {
	gitInstallDir, err := searchGitForWindows()
	if err != nil {
		cleanupEnv(strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)))
		return nil
	}
	checkLessExe(gitInstallDir)
	pathEnv := os.Getenv("PATH")
	pathList := strings.Split(pathEnv, string(os.PathListSeparator))
	pathNewList := make([]string, 0, len(pathList)+2)
	if _, err := exec.LookPath("git"); err != nil && hasGitExe(gitInstallDir) {
		pathNewList = append(pathNewList, filepath.Join(gitInstallDir, "cmd"))
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
	_ = os.Setenv("PATH", strings.Join(pathNewList, string(os.PathListSeparator)))
	return nil
}
