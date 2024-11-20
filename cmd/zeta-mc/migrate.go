// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/pkg/migrate"
	"github.com/antgroup/hugescm/pkg/tr"
)

type App struct {
	Globals
	From        string   `arg:"" name:"from" help:"Original repository remote URL (or filesystem path)" type:"string"`
	Destination string   `arg:"" optional:"" name:"destination" help:"Destination where the repository is migrated" type:"path"`
	Values      []string `short:"X" name:":config" help:"Override default configuration, format: <key>=<value>"`
	Squeeze     bool     `name:"squeeze" short:"s" help:"Squeeze mode, compressed metadata"`
	LFS         bool     `name:"lfs" help:"Migrate all LFS objects to zeta"`
	Quiet       bool     `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
	Debug       bool     `name:"debug" help:"Enable debug mode; analyze timing"`
}

func die_error(format string, a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString(tr.W("error: "))
	fmt.Fprintf(&b, tr.W(format), a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

func (c *App) concatDestination(baseName string) (string, error) {
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
	dirs, err := os.ReadDir(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return destination, nil
		}
		fmt.Fprintf(os.Stderr, "readdir %s error: %v\n", destination, err)
		return "", err
	}
	if len(dirs) != 0 {
		die_error("destination path '%s' already exists and is not an empty directory.", filepath.Base(destination))
		return "", ErrWorktreeNotEmpty
	}
	return destination, nil
}

func (c *App) cloneAndMigrate(g *Globals, uri string) error {
	destination, err := c.concatDestination(path.Base(uri))
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(os.TempDir(), "clone")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return err
	}
	defer os.RemoveAll(tempDir)
	if err := g.RunEx(command.NoDir, "git", "clone", "--bare", c.From, tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "clone error: %v", err)
		return err
	}
	return c.migrateFrom(g, tempDir, destination)
}

func (c *App) Run(g *Globals) error {
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
	destination, err := c.concatDestination(filepath.Base(c.From) + "-zeta")
	if err != nil {
		return err
	}
	return c.migrateFrom(g, absFrom, destination)
}

func (c *App) migrateFrom(g *Globals, from, to string) error {
	if c.LFS {
		fmt.Fprintf(os.Stderr, "Fetch all lfs objects ...\n")
		if err := g.RunEx(from, "git", "lfs", "fetch", "--all"); err != nil {
			fmt.Fprintf(os.Stderr, "git lfs fetch error: %v", err)
		}
	}
	now := time.Now()
	r, err := migrate.NewMigrator(context.Background(), &migrate.MigrateOptions{
		Environ: os.Environ(),
		From:    from,
		To:      to,
		Squeeze: c.Squeeze,
		LFS:     c.LFS,
		StepEnd: 4,
		Values:  c.Values,
		Quiet:   c.Quiet,
		Verbose: g.Verbose,
		Debuger: g,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewRewriter error: %v\n", err)
		return err
	}
	defer r.Close()
	if err := r.Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Execute error: %v\n", err)
		return err
	}
	tr.Fprintf(os.Stderr, "Migrate '%s' from git to zeta success, spent: %v\n", c.From, time.Since(now))
	return nil
}
