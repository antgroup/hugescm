package systemproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

type coordDialer struct {
	proxyURL *url.URL
	forward  *net.Dialer
}

func (d *coordDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	return DialServerViaCONNECT(ctx, address, d.proxyURL, d.forward)
}

// DialServerViaCONNECT: SSH protocol should use socks5 protocol as much as possible
func DialServerViaCONNECT(ctx context.Context, addr string, proxy *url.URL, forward *net.Dialer) (net.Conn, error) {
	proxyAddr := proxy.Host
	var c net.Conn
	var err error
	switch proxy.Scheme {
	case "http":
		if proxy.Port() == "" {
			proxyAddr = net.JoinHostPort(proxyAddr, "80")
		}
		if c, err = forward.DialContext(ctx, "tcp", proxyAddr); err != nil {
			return nil, err
		}
	case "https":
		if proxy.Port() == "" {
			proxyAddr = net.JoinHostPort(proxyAddr, "443")
		}
		d := &tls.Dialer{NetDialer: forward}
		if c, err = d.DialContext(ctx, "tcp", proxyAddr); err != nil {
			return nil, err
		}
	}
	h := make(http.Header)
	if u := proxy.User; u != nil {
		h.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u.String())))
	}
	h.Set("Proxy-Connection", "Keep-Alive")
	connect := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: h,
	}
	if err := connect.Write(c); err != nil {
		_ = c.Close()
		return nil, err
	}
	br := bufio.NewReader(c)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("reading HTTP response from CONNECT to %s via proxy %s failed: %w",
			addr, proxyAddr, err)
	}
	if res.StatusCode != 200 {
		_ = c.Close()
		return nil, fmt.Errorf("proxy error from %s while dialing %s: %v", proxyAddr, addr, res.Status)
	}

	// It's safe to discard the bufio.Reader here and return the
	// original TCP conn directly because we only use this for
	// TLS, and in TLS the client speaks first, so we know there's
	// no unbuffered data. But we can double-check.
	if br.Buffered() > 0 {
		_ = c.Close()
		return nil, fmt.Errorf("unexpected %d bytes of buffered data from CONNECT proxy %q",
			br.Buffered(), proxyAddr)
	}
	return c, nil
}
