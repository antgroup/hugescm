package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

func getEnvAny(names ...string) string {
	for _, n := range names {
		if val, ok := os.LookupEnv(n); ok && val != "" {
			return val
		}
	}
	return ""
}

var (
	envProxyOnce      sync.Once
	envProxyFuncValue func(*url.URL) (*url.URL, error)
)

// envProxyFunc returns a function that reads the
// environment variable to determine the proxy address.
func envProxyFunc() func(*url.URL) (*url.URL, error) {
	envProxyOnce.Do(func() {
		envProxyFuncValue = FromEnvironment().ProxyFunc()
	})
	return envProxyFuncValue
}

func ProxyFromEnvironment(req *http.Request) (*url.URL, error) {
	return envProxyFunc()(req.URL)
}

type ProxyConfig struct {
	ProxyURL *url.URL
	NoProxy  []string
}

func ProxyFromEnv() *ProxyConfig {
	cfg := FromEnvironment()
	proxyEnv := cfg.HTTPProxy
	if len(proxyEnv) == 0 {
		return nil
	}
	u, err := url.Parse(proxyEnv)
	if err != nil || u.Scheme == "" {
		if u, err = url.Parse("http://" + proxyEnv); err != nil {
			return nil
		}
	}
	return &ProxyConfig{ProxyURL: u, NoProxy: strings.Split(cfg.NoProxy, ",")}
}

func (pc *ProxyConfig) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer, err := proxy.FromURL(pc.ProxyURL, proxy.Direct)
	if err != nil {
		return nil, err
	}
	return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
}

var coordDialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 15 * time.Second,
}

func DialServerViaCONNECT(ctx context.Context, addr string, proxy *url.URL) (net.Conn, error) {
	proxyAddr := proxy.Host
	var d net.Dialer
	var c net.Conn
	var err error
	switch proxy.Scheme {
	case "http":
		if proxy.Port() == "" {
			proxyAddr = net.JoinHostPort(proxyAddr, "80")
		}
		if c, err = d.DialContext(ctx, "tcp", proxyAddr); err != nil {
			return nil, fmt.Errorf("dialing proxy %q failed: %w", proxyAddr, err)
		}
	case "https":
		if proxy.Port() == "" {
			proxyAddr = net.JoinHostPort(proxyAddr, "443")
		}
		if c, err = tls.DialWithDialer(coordDialer, "tcp", proxyAddr, nil); err != nil {
			return nil, fmt.Errorf("dialing proxy %q failed: %w", proxyAddr, err)
		}
	}

	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, proxy.Hostname())
	br := bufio.NewReader(c)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response from CONNECT to %s via proxy %s failed: %w",
			addr, proxyAddr, err)
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("proxy error from %s while dialing %s: %v", proxyAddr, addr, res.Status)
	}

	// It's safe to discard the bufio.Reader here and return the
	// original TCP conn directly because we only use this for
	// TLS, and in TLS the client speaks first, so we know there's
	// no unbuffered data. But we can double-check.
	if br.Buffered() > 0 {
		return nil, fmt.Errorf("unexpected %d bytes of buffered data from CONNECT proxy %q",
			br.Buffered(), proxyAddr)
	}
	return c, nil
}
