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

var allowedEnv = map[string]bool{
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

func LookupEnv(key string) (string, bool) {
	// if key == "LANG" {
	// 	return "en_US.UTF-8", true
	// }
	return os.LookupEnv(key)
}

var (
	Environ = sync.OnceValue(func() []string {
		origin := os.Environ()
		cleanEnv := make([]string, 0, len(origin))
		for _, s := range origin {
			k, _, ok := strings.Cut(s, "=")
			if !ok {
				continue
			}
			if strings.HasPrefix(k, "GIT_") && !allowedEnv[k] {
				continue
			}
			cleanEnv = append(cleanEnv, s)
		}
		slices.Sort(cleanEnv) // order by
		return cleanEnv
	})
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
