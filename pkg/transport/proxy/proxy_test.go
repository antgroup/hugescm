package proxy

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestProxyFromEnvironment(t *testing.T) {
	req, err := http.NewRequest("GET", "https://github.com", nil)
	if err != nil {
		return
	}
	u, err := ProxyFromEnvironment(req)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", u)
}

func TestProxyFromEnvironmentEnv(t *testing.T) {
	os.Setenv("ALL_PROXY", "socks5://127.0.0.1:13659")
	req, err := http.NewRequest("GET", "https://github.com", nil)
	if err != nil {
		return
	}
	u, err := ProxyFromEnvironment(req)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", u)
}
