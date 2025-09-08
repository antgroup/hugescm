//go:build windows

package env

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"
)

var (
	allowedEnv = map[string]bool{
		// Environment variables to tell git to use custom SSH executable or command
		"GIT_SSH":         true,
		"GIT_SSH_COMMAND": true,
		// Export git tracing variables for easier debugging
		"GIT_TRACE":             true,
		"GIT_TRACE_PACK_ACCESS": true,
		"GIT_TRACE_PACKET":      true,
		"GIT_TRACE_PERFORMANCE": true,
		"GIT_TRACE_SETUP":       true,
		"GIT_CURL_VERBOSE":      true,
	}
)

var (
	Environ = sync.OnceValue(func() []string {
		originEnv := os.Environ()
		sanitizedEnv := make([]string, 0, len(originEnv))
		for _, s := range originEnv {
			k, _, ok := strings.Cut(s, "=")
			if !ok || strings.HasPrefix(k, "GIT_") && !allowedEnv[k] {
				continue
			}
			sanitizedEnv = append(sanitizedEnv, s)
		}
		slices.Sort(sanitizedEnv) // order by
		return sanitizedEnv
	})
)

var (
	lookupGitForWindowsInstall = sync.OnceValues(func() (string, error) {
		gitForWindowsKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\GitForWindows`, registry.QUERY_VALUE)
		if err != nil {
			return "", nil
		}
		defer gitForWindowsKey.Close() // nolint
		installPath, _, err := gitForWindowsKey.GetStringValue("InstallPath")
		if err != nil {
			return "", err
		}
		return installPath, nil
	})
)

func hasGitExe(installDir string) bool {
	gitExe := filepath.Join(installDir, "cmd", "git.exe")
	if _, err := os.Stat(gitExe); err != nil {
		return false
	}
	return true
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

// DelayInitializeEnv: initialize path env
func DelayInitializeEnv() error {
	gitForWindowsInstall, err := lookupGitForWindowsInstall()
	if err != nil {
		cleanupEnv(strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)))
		return nil
	}
	pathEnv := os.Getenv("PATH")
	pathList := strings.Split(pathEnv, string(os.PathListSeparator))
	pathNewList := make([]string, 0, len(pathList)+2)
	if _, err := exec.LookPath("git"); err != nil && hasGitExe(gitForWindowsInstall) {
		pathNewList = append(pathNewList, filepath.Join(gitForWindowsInstall, "cmd"))
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

func LookupPager(name string) (string, error) {
	pagerExe, err := exec.LookPath(name)
	if err == nil {
		return pagerExe, nil
	}
	gitForWindowsInstall, err := lookupGitForWindowsInstall()
	if err != nil {
		return "", err
	}
	// C:\Program Files\Git\usr\bin\less.exe
	lessExe := filepath.Join(gitForWindowsInstall, "usr/bin/less.exe")
	if _, err := os.Stat(lessExe); err != nil {
		return "", err
	}
	return lessExe, nil
}
