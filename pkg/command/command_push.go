// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"errors"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Push struct {
	Refspec     string   `arg:"" optional:"" name:"refspec" default:"" help:"Specify what destination ref to update with what source object"`
	PushOptions []string `name:"push-option" short:"o" help:"Option to transmit" placeholder:"<option>"`
	Tag         bool     `name:"tag" short:"t" help:"Update remote tag reference"`
	Force       bool     `name:"force" short:"f" help:"force updates"`
}

func (c *Push) Run(g *Globals) error {
	if len(c.Refspec) == 0 && c.Tag {
		diev("--tag is not compatible with blank refspec")
		return errors.New("flags incompatible")
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	if err := r.Push(context.Background(), &zeta.PushOptions{
		Refspec:     c.Refspec,
		PushOptions: c.PushOptions,
		Tag:         c.Tag,
		Force:       c.Force,
	}); err != nil {
		return err
	}
	return nil
}
