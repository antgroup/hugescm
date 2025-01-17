// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/systemproxy"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/ssh/config"
	"github.com/antgroup/hugescm/pkg/transport/ssh/knownhosts"
	"github.com/antgroup/hugescm/pkg/version"
	"golang.org/x/crypto/ssh"
)

const (
	protocolVersionPrefix = "SSH-2.0-"
)

var (
	W      = tr.W
	direct = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
)

const DefaultUsername = "zeta"

type client struct {
	*transport.Endpoint
	dialer       systemproxy.Dialer
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
		Endpoint: endpoint,
		dialer:   systemproxy.NewSystemDialer(direct),
		verbose:  verbose,
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
	conn, err := c.dialer.DialContext(ctx, network, addr)
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) && direct != c.dialer {
			c.DbgPrint("Connect proxy server error: %v", err)
			return direct.DialContext(ctx, network, addr)
		}
		return nil, err
	}
	return conn, nil
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
	auth, err := c.prepareAuthMethod()
	if err != nil {
		return nil, err
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:              c.User,
		Auth:              auth,
		ClientVersion:     protocolVersionPrefix + version.GetBannerVersion(),
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
	trace.DbgPrint(format, args...)
}
