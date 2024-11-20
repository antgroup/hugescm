//go:build !windows

package proxy

import (
	"os"

	"golang.org/x/net/http/httpproxy"
)

func FromEnvironment() *httpproxy.Config {
	return &httpproxy.Config{
		HTTPProxy:  getEnvAny("HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"),
		HTTPSProxy: getEnvAny("HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"),
		NoProxy:    getEnvAny("NO_PROXY", "no_proxy"),
		CGI:        os.Getenv("REQUEST_METHOD") != "",
	}
}
