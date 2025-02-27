package env

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var allowedEnv = []string{
	"HOME",
	"PATH",
	"TZ",
	"LANG", //Replace by en_US.UTF-8
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

func SanitizerEnv(removeKey ...string) []string {
	removeMap := make(map[string]bool)
	for _, k := range removeKey {
		removeMap[k] = true
	}
	originEnv := os.Environ()
	env := make([]string, 0, len(originEnv))
	for _, e := range originEnv {
		k, _, ok := strings.Cut(e, "=")
		if !ok {
			// BAD env
			continue
		}
		if removeMap[k] {
			continue
		}
		env = append(env, e)
	}
	return env
}

// GetBool fetches and parses a boolean typed environment variable
//
// If the variable is empty, returns `fallback` and no error.
// If there is an error, returns `fallback` and the error.
func GetBool(name string, fallback bool) (bool, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return fallback, fmt.Errorf("get bool %s: %w", name, err)
	}
	return v, nil
}

// GetInt fetches and parses an integer typed environment variable
//
// If the variable is empty, returns `fallback` and no error.
// If there is an error, returns `fallback` and the error.
func GetInt(name string, fallback int) (int, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback, fmt.Errorf("get int %s: %w", name, err)
	}
	return v, nil
}

// GetDuration fetches and parses a duration typed environment variable
func GetDuration(name string, fallback time.Duration) (time.Duration, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fallback, fmt.Errorf("get duration %s: %w", name, err)
	}
	return v, nil
}

// GetString fetches a given name from the environment and falls back to a
// default value if the name is not available. The value is stripped of
// leading and trailing whitespace.
func GetString(name string, fallback string) string {
	value := os.Getenv(name)

	if value == "" {
		return fallback
	}

	return strings.TrimSpace(value)
}
