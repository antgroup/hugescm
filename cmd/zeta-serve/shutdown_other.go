// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func (c *closer) listenSignal(ctx context.Context, srv Shutdowner) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	signal := <-quit
	logrus.Infof("zeta-serve receive signal: %v, exiting ...", signal)
	newCtx, cancelCtx := context.WithTimeout(ctx, time.Minute*6)
	defer cancelCtx()
	_ = srv.Shutdown(newCtx)
	c.ch <- true
}
