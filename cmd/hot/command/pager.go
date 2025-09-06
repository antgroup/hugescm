package command

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/shlex"
	"github.com/antgroup/hugescm/modules/term"
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
	pager, ok := os.LookupEnv("GIT_PAGER")
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

func NewPrinter(ctx context.Context) *printer {
	if term.StdoutLevel == term.LevelNone {
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	pager, ok := lookupPager()
	if ok && len(pager) == 0 {
		// PAGER disabled
		return &printer{w: os.Stdout, colorMode: term.StdoutLevel}
	}
	if len(pager) == 0 {
		pager = "less" // search pager
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
