// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

type RunCtx struct {
	S       *Server
	Session *Session
}

var (
	ErrPathNecessary = errors.New("path is necessary")
)

type Command interface {
	ParseArgs(args []string) error
	Exec(ctx *RunCtx) int
}

var (
	commandProvider = map[string]func() Command{
		"ls-remote": func() Command {
			return &LsRemote{}
		},
		"metadata": func() Command {
			return &Metadata{}
		},
		"objects": func() Command {
			return &Objects{}
		},
		"push": func() Command {
			return &Push{}
		},
	}
)

func NewCommand(args []string) (Command, error) {
	if len(args) < 1 {
		return nil, errors.New("missing args")
	}
	cmdFunc, ok := commandProvider[args[0]]
	if !ok {
		return nil, fmt.Errorf("unregister sub command: %s", args[0])
	}
	cmd := cmdFunc()
	if err := cmd.ParseArgs(args[1:]); err != nil {
		return nil, err
	}
	return cmd, nil
}

func ZetaEncodeVND(w io.Writer, a any) {
	if err := json.NewEncoder(w).Encode(a); err != nil {
		logrus.Errorf("encode response error: %v", err)
	}
}
