// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/httpserver"
	"github.com/antgroup/hugescm/pkg/serve/sshserver"
	"github.com/sirupsen/logrus"
)

type Serve struct {
	Config string `short:"c" name:"config" help:"Location of shared server config file (parsed by both httpd and sshd)" default:"~/config/zeta-serve.toml" type:"path"`
}

func (c *Serve) Run(globals *Globals) error {
	httpCfg, err := httpserver.NewServerConfig(c.Config, globals.ExpandEnv)
	if err != nil {
		logrus.Errorf("zeta-serve serve: http config error: %v", err)
		return err
	}
	httpSrv, err := httpserver.NewServer(httpCfg)
	if err != nil {
		logrus.Errorf("zeta-serve serve: new httpd error: %v", err)
		return err
	}
	sshCfg, err := sshserver.NewServerConfig(c.Config, globals.ExpandEnv)
	if err != nil {
		logrus.Errorf("zeta-serve serve: ssh config error: %v", err)
		return err
	}
	sshSrv, err := sshserver.NewServer(sshCfg)
	if err != nil {
		logrus.Errorf("zeta-serve serve: new sshd error: %v", err)
		return err
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); errCh <- httpSrv.ListenAndServe() }()
	go func() { defer wg.Done(); errCh <- sshSrv.ListenAndServe() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var exitErr error
	select {
	case sig := <-quit:
		logrus.Infof("zeta-serve received signal: %v, shutting down ...", sig)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Errorf("zeta-serve serve: %v", err)
			exitErr = err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	go func() { _ = httpSrv.Shutdown(ctx) }()
	go func() { _ = sshSrv.Shutdown(ctx) }()

	wg.Wait()

	logrus.Infof("zeta-serve exited")
	return exitErr
}
