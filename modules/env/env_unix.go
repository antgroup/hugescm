//go:build !windows

package env

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

var (
	allowedEnv = map[string]bool{
		"HOME":            true,
		"PATH":            true,
		"TZ":              true,
		"LANG":            true, //Replace by en_US.UTF-8
		"TEMP":            true,
		"LD_LIBRARY_PATH": true,
		// Git HTTP proxy settings: https://git-scm.com/docs/git-config#git-config-httpproxy
		"all_proxy":   true,
		"http_proxy":  true,
		"HTTP_PROXY":  true,
		"https_proxy": true,
		"HTTPS_PROXY": true,
		// libcurl settings: https://curl.haxx.se/libcurl/c/CURLOPT_NOPROXY.html
		"no_proxy": true,
		"NO_PROXY": true,
		// Environment variables to tell git to use custom SSH executable or command
		"GIT_SSH":         true,
		"GIT_SSH_COMMAND": true,
		// Environment variables need for ssh-agent based authentication
		"SSH_AUTH_SOCK": true,
		"SSH_AGENT_PID": true,

		// Export git tracing variables for easier debugging
		"GIT_TRACE":             true,
		"GIT_TRACE_PACK_ACCESS": true,
		"GIT_TRACE_PACKET":      true,
		"GIT_TRACE_PERFORMANCE": true,
		"GIT_TRACE_SETUP":       true,
	}
)

var (
	Environ = sync.OnceValue(func() []string {
		origin := os.Environ()
		cleanEnv := make([]string, 0, len(origin))
		for _, s := range origin {
			k, _, ok := strings.Cut(s, "=")
			if !ok {
				continue
			}
			if !allowedEnv[k] {
				continue
			}
			cleanEnv = append(cleanEnv, s)
		}
		slices.Sort(cleanEnv) // order by
		return cleanEnv
	})
)

func DelayInitializeEnv() error {
	pathEnv := os.Getenv("PATH")
	pathList := strings.Split(pathEnv, string(os.PathListSeparator))
	pathNewList := make([]string, 0, len(pathList))
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
	return exec.LookPath(name)
}
