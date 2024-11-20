// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
)

type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

type closer struct {
	ch chan bool
}

func newCloser() *closer {
	return &closer{ch: make(chan bool, 1)}
}
