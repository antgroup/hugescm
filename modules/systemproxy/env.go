package systemproxy

import (
	"os"
)

// https://about.gitlab.com/blog/2021/01/27/we-need-to-talk-no-proxy/

func getEnvAny(names ...string) string {
	for _, n := range names {
		if val, ok := os.LookupEnv(n); ok && val != "" {
			return val
		}
	}
	return ""
}
