// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type HashObject struct {
	W     bool   `short:"w" shortonly:"" help:"Write the object into the object database"`
	Stdin bool   `name:"stdin" help:"Read the object from stdin"`
	Path  string `name:"path" help:"Process file as it were from this path" placeholder:"<file>"`
}

func (c *HashObject) Run(g *Globals) error {
	if !c.W {
		return c.hashObject()
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	if c.Stdin {
		oid, err := r.ODB().HashTo(context.Background(), os.Stdin, -1)
		if err != nil {
			diev("hash-object error: %v", err)
			return err
		}
		fmt.Fprintln(os.Stdout, oid)
		return nil
	}
	if len(c.Path) == 0 {
		diev("require --stdin or --path")
		return ErrArgRequired
	}
	fd, err := os.Open(c.Path)
	if err != nil {
		diev("open %s error: %v", c.Path, err)
		return err
	}
	defer fd.Close()
	si, err := fd.Stat()
	if err != nil {
		diev("stat %s error: %v", c.Path, err)
		return err
	}
	oid, _, err := r.HashTo(context.Background(), fd, si.Size())
	if err != nil {
		diev("hash-object error: %v", err)
		return err
	}
	fmt.Fprintln(os.Stdout, oid)
	return nil
}

func (c *HashObject) hashObject() error {
	var r io.Reader
	switch {
	case c.Stdin:
		r = os.Stdin
	case len(c.Path) != 0:
		fd, err := os.Open(c.Path)
		if err != nil {
			diev("open %s error: %v", c.Path, err)
			return err
		}
		defer fd.Close()
		r = fd
	default:
		diev("require --stdin or --path")
		return ErrArgRequired
	}
	h := plumbing.NewHasher()
	if _, err := io.Copy(h, r); err != nil {
		diev("hash-object error: %v", err)
		return err
	}
	fmt.Fprintln(os.Stdout, h.Sum())
	return nil
}
