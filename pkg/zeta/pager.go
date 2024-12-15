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
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// https://github.com/sharkdp/bat/blob/master/src/less.rs
func lookupPagerEnv() (string, bool) {
	pager, ok := os.LookupEnv("ZETA_PAGER")
	if ok {
		return pager, ok
	}
	return os.LookupEnv("PAGER")
}

type Printer struct {
	w        io.Writer
	useColor bool
	closeFn  func() error
}

func (p *Printer) Close() error {
	if p.closeFn == nil {
		return nil
	}
	return p.closeFn()
}

func (p *Printer) UseColor() bool {
	return p.useColor
}

func (p *Printer) Write(b []byte) (n int, err error) {
	return p.w.Write(b)
}

func indent(t string) string {
	var output []string
	for _, line := range strings.Split(t, "\n") {
		if len(line) != 0 {
			line = "    " + line
		}

		output = append(output, line)
	}

	return strings.Join(output, "\n")
}

func (p *Printer) logOne(c *object.Commit) (err error) {
	if p.useColor {
		_, err = fmt.Fprintf(p.w, "\x1b[33mcommit %s\x1b[0m\nAuthor: %s <%s>\nDate:   %s\n\n%s\n",
			c.Hash, c.Author.Name, c.Author.Email, c.Author.When.Format(time.RFC3339), indent(c.Message))
		return
	}
	_, err = fmt.Fprintf(p.w, "commit %s\nAuthor: %s <%s>\nDate:   %s\n\n%s\n",
		c.Hash, c.Author.Name, c.Author.Email, c.Author.When.Format(time.RFC3339), indent(c.Message))
	return
}

func (p *Printer) logOneNoColor(c *object.Commit, refs []*ReferenceLite) (err error) {
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

func (p *Printer) LogOne(c *object.Commit, refs []*ReferenceLite) (err error) {
	if len(refs) == 0 {
		return p.logOne(c)
	}
	if !p.useColor {
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

func NewPrinter(ctx context.Context) *Printer {
	if !isTrueColorTerminal {
		return &Printer{useColor: isTrueColorTerminal, w: os.Stdout}
	}
	pager, ok := lookupPagerEnv()
	if ok && len(pager) == 0 {
		// PAPER disabled
		return &Printer{useColor: isTrueColorTerminal, w: os.Stdout}
	}
	if len(pager) == 0 {
		pager = "less"
	}
	pagerExe, err := exec.LookPath(pager)
	if err != nil {
		return &Printer{useColor: isTrueColorTerminal, w: os.Stdout}
	}
	cmd := exec.CommandContext(ctx, pagerExe)

	cmd.Env = env.SanitizerEnv("PAGER", "LESS", "LV") // AVOID PAGER ENV
	// PAGER_ENV: LESS=FRX LV=-c
	cmd.Env = append(cmd.Env, "LESS=FRX", "LV=-c")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return &Printer{useColor: isTrueColorTerminal, w: os.Stdout}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return &Printer{useColor: isTrueColorTerminal, w: os.Stdout}
	}
	return &Printer{useColor: isTrueColorTerminal, w: stdin, closeFn: func() error {
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			return err
		}
		return nil
	}}
}
