// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"

	"github.com/antgroup/hugescm/pkg/serve/sshserver"
	"github.com/sirupsen/logrus"
)

type SSHD struct {
	Config string `short:"c" name:"config" help:"Location of server config file" default:"~/config/zeta-serve-sshd.toml" type:"path"`
}

func (c *SSHD) Run(globals *Globals) error {
	sc, err := sshserver.NewServerConfig(c.Config, globals.ExpandEnv)
	if err != nil {
		logrus.Errorf("zeta-seve sshd load server config error: %v", err)
		return err
	}
	srv, err := sshserver.NewServer(sc)
	if err != nil {
		logrus.Errorf("zeta-seve sshd new sshd server error: %v", err)
		return err
	}
	closer := newCloser()
	go closer.listenSignal(context.Background(), srv)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.Errorf("zeta-seve sshd listen server error: %v", err)
		return err
	}
	<-closer.ch
	logrus.Infof("zeta-seve sshd exited")
	return nil
}
