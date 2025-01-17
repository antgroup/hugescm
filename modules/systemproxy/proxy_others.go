//go:build !windows && !darwin

package systemproxy

import (
	"net"
	"os"

	"golang.org/x/net/http/httpproxy"
)

func NewSystemDialer(forward *net.Dialer) Dialer {
	allProxy := getEnvAny("ALL_PROXY", "all_proxy")
	noProxy := getEnvAny("NO_PROXY", "no_proxy")
	if len(allProxy) == 0 {
		return forward
	}
	proxyURL, err := ParseURL(allProxy, "http://")
	if err != nil {
		return forward
	}
	return newDialer(proxyURL, forward, noProxy)
}

func systemProxyConfig() *httpproxy.Config {
	return &httpproxy.Config{
		HTTPProxy:  getEnvAny("HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"),
		HTTPSProxy: getEnvAny("HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"),
		NoProxy:    getEnvAny("NO_PROXY", "no_proxy"),
		CGI:        os.Getenv("REQUEST_METHOD") != "",
	}
}
