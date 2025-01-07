// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/proxy"
	"golang.org/x/crypto/ssh"
)

var (
	W      = tr.W
	dialer = net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
)

const DefaultUsername = "zeta"

type client struct {
	*transport.Endpoint
	proxyConfig     *proxy.ProxyConfig
	hostKeyCallback ssh.HostKeyCallback
	verbose         bool
}

func NewTransport(ctx context.Context, endpoint *transport.Endpoint, operation transport.Operation, verbose bool) (transport.Transport, error) {
	return &client{
		Endpoint:    endpoint,
		proxyConfig: proxy.ProxyFromEnv(),
		verbose:     verbose,
	}, nil
}

func (c *client) DialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	pc := c.proxyConfig
	if pc == nil || slices.Contains(pc.NoProxy, c.Host) {
		return dialer.DialContext(ctx, network, addr)
	}
	switch pc.ProxyURL.Scheme {
	case "http", "https":
		return proxy.DialServerViaCONNECT(ctx, addr, pc.ProxyURL)
	case "socks5", "socks5h":
		return pc.DialContext(ctx, network, addr)
	}
	return dialer.DialContext(ctx, network, addr)
}

func (c *client) NewBaseCommand(ctx context.Context) (*Command, error) {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, err := c.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return c.newCommand(conn, addr)
}

func (c *client) newCommand(conn net.Conn, addr string) (*Command, error) {
	auth := []ssh.AuthMethod{
		ssh.PublicKeysCallback(c.PublicKeys),
	}
	if len(c.Password) != 0 {
		auth = append(auth, ssh.Password(c.Password)) // no retry
	} else {
		auth = append(auth, ssh.RetryableAuthMethod(ssh.PasswordCallback(c.readPassword), 3))
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            c.User,
		Auth:            auth,
		HostKeyCallback: c.HostKeyCallback,
	})
	if err != nil {
		return nil, err
	}
	client := ssh.NewClient(cc, chans, reqs)
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	cmd := &Command{client: client, Session: session, Reader: bytes.NewReader(nil), DbgPrint: c.DbgPrint}
	session.Stderr = &cmd.stderr // bind stderr
	_ = cmd.Setenv("LANG", os.Getenv("LANG"))
	_ = cmd.Setenv("TERM", os.Getenv("TERM"))
	_ = cmd.Setenv("ZETA_PROTOCOL", "Z1")
	_ = cmd.Setenv("SERVER_NAME", c.Host)
	return cmd, nil
}

var (
	_ transport.Transport = &client{}
)

func (c *client) DbgPrint(format string, args ...any) {
	if !c.verbose {
		return
	}
	message := fmt.Sprintf(format, args...)
	var buffer bytes.Buffer
	for _, s := range strings.Split(message, "\n") {
		_, _ = buffer.WriteString("\x1b[38;2;254;225;64m* ")
		_, _ = buffer.WriteString(s)
		_, _ = buffer.WriteString("\x1b[0m\n")
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}
