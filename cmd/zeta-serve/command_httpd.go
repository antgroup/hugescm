// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"

	"github.com/antgroup/hugescm/pkg/serve/httpserver"
	"github.com/sirupsen/logrus"
)

type HTTPD struct {
	Config string `short:"c" name:"config" help:"Location of server config file" default:"~/config/zeta-serve-httpd.toml" type:"path"`
}

func (c *HTTPD) Run(globals *Globals) error {
	sc, err := httpserver.NewServerConfig(c.Config, globals.ExpandEnv)
	if err != nil {
		logrus.Errorf("zeta-seve httpd load server config error: %v", err)
		return err
	}
	srv, err := httpserver.NewServer(sc)
	if err != nil {
		logrus.Errorf("zeta-seve httpd new httpd server error: %v", err)
		return err
	}
	closer := newCloser()
	go closer.listenSignal(context.Background(), srv)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.Errorf("zeta-seve httpd listen server error: %v", err)
		return err
	}
	<-closer.ch
	logrus.Infof("zeta-seve httpd exited")
	return nil
}
