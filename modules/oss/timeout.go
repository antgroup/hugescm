// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"net"
	"time"
)

// timeoutConn handles HTTP timeout
type timeoutConn struct {
	conn    net.Conn
	timeout time.Duration
}

func newTimeoutConn(conn net.Conn, timeout time.Duration) *timeoutConn {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	return &timeoutConn{
		conn:    conn,
		timeout: timeout,
	}
}

func (c *timeoutConn) Read(b []byte) (n int, err error) {
	_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	n, err = c.conn.Read(b)
	_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	return n, err
}

func (c *timeoutConn) Write(b []byte) (n int, err error) {
	_ = c.SetWriteDeadline(time.Now().Add(c.timeout))
	n, err = c.conn.Write(b)
	_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	return n, err
}

func (c *timeoutConn) Close() error {
	return c.conn.Close()
}

func (c *timeoutConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *timeoutConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *timeoutConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *timeoutConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *timeoutConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
