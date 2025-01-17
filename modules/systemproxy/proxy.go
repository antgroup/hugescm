// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package proxy provides support for a variety of protocols to proxy network
// data.
package systemproxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
)

// A Dialer is a means to establish a connection.
// Custom dialers should also implement ContextDialer.
type Dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

// Auth contains authentication parameters that specific Dialers may require.
type Auth struct {
	User, Password string
}

func NewDialerFromURL(u *url.URL, forward *net.Dialer) (Dialer, error) {
	switch u.Scheme {
	case "socks5", "socks5h":
		addr := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "1080"
		}
		var auth *Auth
		if u.User != nil {
			auth = new(Auth)
			auth.User = u.User.Username()
			if p, ok := u.User.Password(); ok {
				auth.Password = p
			}
		}
		return SOCKS5("tcp", net.JoinHostPort(addr, port), auth, forward)
	case "http", "https":
		d := &coordDialer{
			proxyURL: u,
			forward:  forward,
		}
		return d, nil
	}
	return nil, errors.New("systemproxy: unknown scheme: " + u.Scheme)
}

type ProxyFuncValue func(*url.URL) (*url.URL, error)

// systemProxyFunc returns a function that reads the
// environment variable or system config to determine the proxy address.
var (
	systemProxyFunc = sync.OnceValue(func() ProxyFuncValue {
		return systemProxyConfig().ProxyFunc()
	})
)

func NewSystemProxy(proxyURL string) func(*http.Request) (*url.URL, error) {
	if len(proxyURL) != 0 {
		if u, err := url.Parse(proxyURL); err == nil {
			// Use proxyURL
			return http.ProxyURL(u)
		}
	}
	return func(r *http.Request) (*url.URL, error) {
		return systemProxyFunc()(r.URL)
	}
}
