// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package systemproxy

import (
	"context"
	"net"

	"github.com/antgroup/hugescm/modules/systemproxy/internal/socks"
)

// SOCKS5 returns a Dialer that makes SOCKSv5 connections to the given
// address with an optional username and password.
// See RFC 1928 and RFC 1929.
func SOCKS5(network, address string, auth *Auth, forward Dialer) (Dialer, error) {
	d := socks.NewDialer(network, address)
	if forward != nil {
		d.ProxyDial = func(ctx context.Context, network string, address string) (net.Conn, error) {
			return forward.DialContext(ctx, network, address)
		}
	}
	if auth != nil {
		up := socks.UsernamePassword{
			Username: auth.User,
			Password: auth.Password,
		}
		d.AuthMethods = []socks.AuthMethod{
			socks.AuthMethodNotRequired,
			socks.AuthMethodUsernamePassword,
		}
		d.Authenticate = up.Authenticate
	}
	return d, nil
}
