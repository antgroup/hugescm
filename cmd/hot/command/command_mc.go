// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/antgroup/hugescm/cmd/hot/pkg/mc"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
)

type Mc struct {
	From        string `arg:"" name:"from" help:"Original repository remote URL (or filesystem path)" type:"string"`
	Destination string `arg:"" optional:"" name:"destination" help:"Destination where the repository is migrated" type:"path"`
	Format      string `name:"format" default:"sha256" help:"Specifying the object format, support only: sha1 or sha256"`
	Bare        bool   `short:"b" name:"bare" optional:"" help:"Save as a bare git repository"`
}

// Migrator
func (c *Mc) concatDestination(baseName string) (string, error) {
	destination := c.Destination
	if len(destination) == 0 {
		destination = strings.TrimSuffix(baseName, ".git")
	}
	if !filepath.IsAbs(destination) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Get current workdir error: %v\n", err)
			return "", err
		}
		destination = filepath.Join(cwd, destination)
	}
	if c.Bare {
		destination += ".git"
	}
	dirs, err := os.ReadDir(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return destination, nil
		}
		fmt.Fprintf(os.Stderr, "readdir %s error: %v\n", destination, err)
		return "", err
	}
	if len(dirs) != 0 {
		fmt.Fprintf(os.Stderr, "fatal: destination path '%s' already exists and is not an empty directory.\n", filepath.Base(destination))
		return "", ErrWorktreeNotEmpty
	}
	return destination, nil
}

func (c *Mc) cloneAndMigrate(g *Globals, uri string) error {
	destination, err := c.concatDestination(path.Base(uri))
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(os.TempDir(), "clone")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return err
	}
	defer os.RemoveAll(tempDir) // nolint
	if err := g.RunEx(context.Background(), command.NoDir, "git", "clone", "--bare", c.From, tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "clone error: %v", err)
		return err
	}
	return c.migrateFrom(g, tempDir, destination)
}

func (c *Mc) Run(g *Globals) error {
	uri, err := pickURI(c.From)
	if err == nil {
		return c.cloneAndMigrate(g, uri)
	}
	if err != ErrLocalEndpoint {
		fmt.Fprintf(os.Stderr, "bad remote '%s' %v\n", c.From, err)
		return err
	}
	absFrom, err := filepath.Abs(c.From)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad remote '%s' %v\n", c.From, err)
		return err
	}
	if _, err = os.Stat(c.From); err != nil {
		fmt.Fprintf(os.Stderr, "bad remote '%s' %v\n", c.From, err)
		return err
	}
	destination, err := c.concatDestination(filepath.Base(c.From) + "-sha256")
	if err != nil {
		return err
	}
	return c.migrateFrom(g, absFrom, destination)
}

func (c *Mc) migrateFrom(g *Globals, from, to string) error {
	now := time.Now()
	r, err := mc.NewMigrator(context.Background(), &mc.MigrateOptions{
		From:    from,
		To:      to, //  os.Environ(), from, to, c.Bare, 4, g.Verbose
		Format:  c.Format,
		Bare:    c.Bare,
		Verbose: g.Verbose,
		StepEnd: 4,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mc %s to %s error: %v\n", from, to, err)
		return err
	}
	defer r.Close() // nolint
	if err := r.Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Execute error: %v\n", err)
		return err
	}
	_, _ = tr.Fprintf(os.Stderr, "migrate repository to %s success, spent: %v\n", c.Format, time.Since(now))
	return nil
}
