//go:build darwin

package systemproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestFindSystemProxy(t *testing.T) {
	settings, err := findSystemProxy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(settings)
}

func TestSystemProxyConfig(t *testing.T) {
	cfg := systemProxyConfig()
	fmt.Fprintf(os.Stderr, "%v\n", cfg)
}

func TestConnectHackNews(t *testing.T) {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 NewSystemProxy(""),
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	resp, err := client.Get("https://news.ycombinator.com/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Fprintf(os.Stderr, "%d %s\n", resp.StatusCode, resp.Status)
	for k, v := range resp.Header {
		if len(v) != 0 {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, v[0])
		}
	}
}
