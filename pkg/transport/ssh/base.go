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
	"github.com/antgroup/hugescm/pkg/transport/ssh/config"
	"github.com/antgroup/hugescm/pkg/transport/ssh/knownhosts"
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
	proxyConfig  *proxy.ProxyConfig
	hostKeyDB    *knownhosts.HostKeyDB
	Hostname     string
	Port         string
	IdentityFile string
	verbose      bool
}

var DefaultUserSettings = &config.UserSettings{
	IgnoreErrors:         false,
	IgnoreMatchDirective: true,
	SystemConfigFinder:   config.SystemConfigFinder,
	UserConfigFinder:     config.UserConfigFinder,
}

func NewTransport(ctx context.Context, endpoint *transport.Endpoint, operation transport.Operation, verbose bool) (transport.Transport, error) {
	cc := &client{
		Endpoint:    endpoint,
		proxyConfig: proxy.ProxyFromEnv(),
		verbose:     verbose,
	}
	var err error
	if cc.Hostname, err = DefaultUserSettings.GetStrict(endpoint.Host, "Hostname"); err != nil || len(cc.Hostname) == 0 {
		cc.Hostname = endpoint.Host
	}
	if cc.Port, err = DefaultUserSettings.GetStrict(endpoint.Host, "Port"); err != nil || len(cc.Port) == 0 {
		cc.Port = strconv.Itoa(endpoint.Port)
	}
	cc.IdentityFile = DefaultUserSettings.Get(endpoint.Host, "IdentityFile")
	return cc, nil
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

func (c *client) traceConn(conn net.Conn) {
	if !c.verbose {
		return
	}
	remoteAddr := conn.RemoteAddr()
	addr, port, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		return
	}
	c.DbgPrint("Connecting to %s [%s] port %s.", c.Host, addr, port)
}

func (c *client) NewBaseCommand(ctx context.Context) (*Command, error) {
	addr := net.JoinHostPort(c.Hostname, c.Port)
	conn, err := c.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	c.traceConn(conn)
	return c.newCommand(conn, addr)
}

var (
	guardEnv = map[string]bool{
		"LANG": true,
		"TERM": true,
	}
)

func isHarmlessEnv(name string) bool {
	upperKey := strings.ToUpper(name)
	return !strings.HasPrefix(upperKey, "ZETA_") && !guardEnv[upperKey]
}

func (c *client) traceSSH(cc ssh.Conn) {
	if !c.verbose {
		return
	}
	// Remote protocol version 2.0, remote software version Bassinet-7.9.9
	// SSH-2.0-HugeSCM-0.16.2
	protocolVersion, soffwareVersion, ok := strings.Cut(strings.TrimPrefix(string(cc.ServerVersion()), "SSH-"), "-")
	if ok {
		c.DbgPrint("Remote protocol version %s, remote software version %s", protocolVersion, soffwareVersion)
	}
}

func (c *client) newCommand(conn net.Conn, addr string) (*Command, error) {
	if c.hostKeyDB == nil {
		if err := c.readHostKeyDB(); err != nil {
			return nil, err
		}
	}
	auth, err := c.makeAuth()
	if err != nil {
		return nil, err
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:              c.User,
		Auth:              auth,
		HostKeyCallback:   c.HostKeyCallback,
		BannerCallback:    ssh.BannerDisplayStderr(),
		HostKeyAlgorithms: c.supportedHostKeyAlgos(),
	})
	if err != nil {
		return nil, err
	}
	c.traceSSH(cc)
	client := ssh.NewClient(cc, chans, reqs)
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	cmd := &Command{client: client, Session: session, Reader: bytes.NewReader(nil), DbgPrint: c.DbgPrint}
	session.Stderr = os.Stderr // bind stderr
	_ = cmd.Setenv("LANG", os.Getenv("LANG"))
	_ = cmd.Setenv("TERM", os.Getenv("TERM"))
	_ = cmd.Setenv("SERVER_NAME", c.Host)
	_ = cmd.Setenv("ZETA_PROTOCOL", "Z1")
	for k, v := range c.ExtraEnv {
		if isHarmlessEnv(k) {
			_ = cmd.Setenv(k, v)
		}
	}
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
