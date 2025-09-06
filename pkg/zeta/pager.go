// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/shlex"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type Printer interface {
	io.WriteCloser
	ColorMode() term.Level
	EnableColor() bool
}

type WrapPrinter struct {
	io.WriteCloser
}

func (WrapPrinter) ColorMode() term.Level {
	return term.LevelNone
}

func (WrapPrinter) EnableColor() bool {
	return false
}

// https://github.com/sharkdp/bat/blob/master/src/less.rs

func lookupPager() (string, bool) {
	pager, ok := os.LookupEnv("ZETA_PAGER")
	if ok {
		return pager, ok
	}
	return os.LookupEnv("PAGER")
}

type printer struct {
	w         io.Writer
	colorMode term.Level
	closeFn   func() error
}

func (p *printer) EnableColor() bool {
	return p.colorMode != term.LevelNone
}

func (p *printer) ColorMode() term.Level {
	return p.colorMode
}

func (p *printer) Write(b []byte) (n int, err error) {
	return p.w.Write(b)
}

func (p *printer) Close() error {
	if p.closeFn == nil {
		return nil
	}
	return p.closeFn()
}

func indent(t string) string {
	var output []string
	lines := strings.Split(t, "\n")
	for _, line := range lines {
		output = append(output, "    "+line)
	}
	if len(lines) > 0 && len(lines[len(lines)-1]) != 0 {
		output = append(output, "")
	}
	return strings.Join(output, "\n")
}

func (p *printer) logOne(c *object.Commit) (err error) {
	if p.colorMode != term.LevelNone {
		_, err = fmt.Fprintf(p.w, "\x1b[33mcommit %s\x1b[0m\nAuthor: %s <%s>\nDate:   %s\n\n%s\n",
			c.Hash, c.Author.Name, c.Author.Email, c.Author.When.Format(time.RFC3339), indent(c.Message))
		return
	}
	_, err = fmt.Fprintf(p.w, "commit %s\nAuthor: %s <%s>\nDate:   %s\n\n%s\n",
		c.Hash, c.Author.Name, c.Author.Email, c.Author.When.Format(time.RFC3339), indent(c.Message))
	return
}

func (p *printer) logOneNoColor(c *object.Commit, refs []*ReferenceLite) (err error) {
	var w bytes.Buffer
	var target plumbing.ReferenceName
	fmt.Fprintf(&w, "commit %s (", c.Hash)
	for i, r := range refs {
		if i != 0 {
			_, _ = w.WriteString(", ")
		}
		if r.Name == plumbing.HEAD {
			if len(r.Target) == 0 {
				_, _ = w.WriteString("HEAD")
				continue
			}
			fmt.Fprintf(&w, "HEAD -> %s", r.Target)
			target = r.Target
			continue
		}
		if target == r.Name {
			continue
		}
		_, _ = w.WriteString(string(r.ShortName))
	}
	_, _ = w.WriteString(")\n")
	if _, err = p.w.Write(w.Bytes()); err != nil {
		return
	}
	_, err = fmt.Fprintf(p.w, "Author: %s <%s>\nDate:   %s\n\n%s\n", c.Author.Name, c.Author.Email, c.Author.When.Format(time.RFC3339), indent(c.Message))
	return
}

func (p *printer) LogOne(c *object.Commit, refs []*ReferenceLite) (err error) {
	if len(refs) == 0 {
		return p.logOne(c)
	}
	if p.colorMode == term.LevelNone {
		return p.logOneNoColor(c, refs)
	}
	var w bytes.Buffer
	var target plumbing.ReferenceName
	fmt.Fprintf(&w, "\x1b[33mcommit %s (", c.Hash)
	for i, r := range refs {
		if r.ShortName == target {
			continue
		}
		if i != 0 {
			_, _ = w.WriteString("\x1b[1;33m,\x1b[0m ")
		}
		if r.Name == plumbing.HEAD {
			if len(r.Target) == 0 {
				_, _ = w.WriteString("\x1b[1;36mHEAD\x1b[0m")
				continue
			}
			fmt.Fprintf(&w, "\x1b[1;36mHEAD -> \x1b[1;32m%s\x1b[33m\x1b[0m", r.Target)
			target = r.Target
			continue
		}
		_, _ = w.WriteString(r.colorFormat())
	}
	_, _ = w.WriteString("\x1b[33m)\x1b[0m\n")
	if _, err = p.w.Write(w.Bytes()); err != nil {
		return
	}
	_, err = fmt.Fprintf(p.w, "Author: %s <%s>\nDate:   %s\n\n%s\n", c.Author.Name, c.Author.Email, c.Author.When.Format(time.RFC3339), indent(c.Message))
	return
}

func NewPrinter(ctx context.Context) *printer {
	if term.StdoutLevel == term.LevelNone {
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	pager, ok := lookupPager()
	if ok && len(pager) == 0 {
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	if len(pager) == 0 {
		pager = "less"
	}
	pagerArgs := make([]string, 0, 4)
	if cmdArgs, _ := shlex.Split(pager, true); len(cmdArgs) > 0 {
		pager = cmdArgs[0]
		pagerArgs = append(pagerArgs, cmdArgs[1:]...)
	}
	pagerExe, err := env.LookupPager(pager)
	if err != nil {
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	cmd := exec.CommandContext(ctx, pagerExe, pagerArgs...)
	cmd.Env = env.SanitizerEnv("PAGER", "LESS", "LV") // AVOID PAGER ENV
	// PAGER_ENV: LESS=FRX LV=-c
	cmd.Env = append(cmd.Env, "LESS=FRX", "LV=-c")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	return &printer{w: stdin, colorMode: term.StdoutLevel, closeFn: func() error {
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			return err
		}
		return nil
	}}
}
