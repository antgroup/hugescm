//go:build !windows

package env

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

var allowedEnv = []string{
	"HOME",
	"PATH",
	"TZ",
	"LANG", //Replace by en_US.UTF-8
	"TEMP",
	"LD_LIBRARY_PATH",
	// Git HTTP proxy settings: https://git-scm.com/docs/git-config#git-config-httpproxy
	"all_proxy",
	"http_proxy",
	"HTTP_PROXY",
	"https_proxy",
	"HTTPS_PROXY",
	// libcurl settings: https://curl.haxx.se/libcurl/c/CURLOPT_NOPROXY.html
	"no_proxy",
	"NO_PROXY",
	// Environment variables to tell git to use custom SSH executable or command
	"GIT_SSH",
	"GIT_SSH_COMMAND",
	// Environment variables need for ssh-agent based authentication
	"SSH_AUTH_SOCK",
	"SSH_AGENT_PID",

	// Export git tracing variables for easier debugging
	"GIT_TRACE",
	"GIT_TRACE_PACK_ACCESS",
	"GIT_TRACE_PACKET",
	"GIT_TRACE_PERFORMANCE",
	"GIT_TRACE_SETUP",
}

func LookupEnv(key string) (string, bool) {
	// if key == "LANG" {
	// 	return "en_US.UTF-8", true
	// }
	return os.LookupEnv(key)
}

var (
	Environ = sync.OnceValue(func() []string {
		cleanEnv := make([]string, 0, len(allowedEnv))
		for _, e := range allowedEnv {
			if v, ok := LookupEnv(e); ok {
				cleanEnv = append(cleanEnv, e+"="+v)
			}
		}
		slices.Sort(cleanEnv) // order by
		return cleanEnv
	})
)

func InitializeEnv() error {
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
