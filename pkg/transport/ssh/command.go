// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/antgroup/hugescm/modules/zeta"
	"golang.org/x/crypto/ssh"
)

type Command struct {
	client *ssh.Client
	*ssh.Session
	io.Reader
	stderr    io.Reader
	DbgPrint  func(format string, args ...any)
	once      sync.Once
	closer    []io.Closer
	lastError error // Do not use specific types to store errors, otherwise nil will not be equal to nil
}

func (c *Command) LastError() error {
	return c.lastError
}

func (c *Command) Setenv(name string, value string) error {
	c.DbgPrint("setting env %s = \"%s\"", name, value)
	return c.Session.Setenv(name, value)
}

func (c *Command) readStderr() {
	br := bufio.NewScanner(c.stderr)
	for br.Scan() {
		fmt.Fprintf(os.Stderr, "remote: %s\n", br.Text())
	}
}

func (c *Command) Start(cmd string) error {
	go c.readStderr()
	c.DbgPrint("Sending command: %s", cmd)
	return c.Session.Start(cmd)
}

func (c *Command) Wait() error {
	var err error
	c.once.Do(func() {
		err = c.Session.Wait()
	})
	return err
}

func (c *Command) Close() error {
	for _, cc := range c.closer {
		_ = cc.Close()
	}
	if err := c.Wait(); err != nil {
		switch a := err.(type) {
		case *ssh.ExitError:
			exitStatus := a.ExitStatus()
			c.DbgPrint("Exit status %v", exitStatus)
			c.lastError = &zeta.ErrExitCode{
				Code:    exitStatus,
				Message: a.String(),
			}
		case *ssh.ExitMissingError:
			c.lastError = &zeta.ErrExitCode{
				Code:    500,
				Message: a.Error(),
			}
		}
		_ = c.client.Close()
		return err
	}
	_ = c.client.Close()
	return nil
}

type getObjectCommand struct {
	*Command
	size   int64
	offset int64
}

func (c *getObjectCommand) Size() int64 {
	return c.size
}

func (c *getObjectCommand) Offset() int64 {
	return c.offset
}
