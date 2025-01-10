// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/progressbar"
	"github.com/antgroup/hugescm/modules/term"
)

var (
	blueColorMap = map[term.Level]string{
		term.Level256: "\x1b[36m",
		term.Level16M: "\x1b[38;2;72;198;239m",
	}
	endColorMap = map[term.Level]string{
		term.Level256: "\x1b[0m",
		term.Level16M: "\x1b[0m",
	}
)

type Bar struct {
	bar   *progressbar.ProgressBar
	total int
}

func MakeTheme() progressbar.Theme {
	switch term.StderrLevel {
	case term.Level256:
		return progressbar.Theme{
			Saucer:        "\x1b[36m#\x1b[0m",
			SaucerHead:    "\x1b[36m>\x1b[0m",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}
	case term.Level16M:
		return progressbar.Theme{
			Saucer:        "\x1b[38;2;72;198;239m#\x1b[0m",
			SaucerHead:    "\x1b[38;2;72;198;239m>\x1b[0m",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}
	default:
	}
	return progressbar.Theme{
		Saucer:        "#",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	}
}

func wrapDescription(description string) string {
	if term.StderrLevel != term.LevelNone {
		return fmt.Sprintf("\x1b[0m%s...", description)
	}
	return description + "..."
}

func NewBar(description string, total int, quiet bool) *Bar {
	if quiet {
		return &Bar{}
	}
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetDescription(wrapDescription(description)),
		progressbar.OptionFullWidth(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintf(os.Stderr, "%s\n", endColorMap[term.StderrLevel])
		}),
		progressbar.OptionSetTheme(MakeTheme()))
	return &Bar{bar: bar, total: total}
}

func NewUnknownBar(description string, quiet bool) *Bar {
	if quiet {
		return &Bar{}
	}
	bar := progressbar.NewOptions64(
		-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowTotalBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)
	return &Bar{bar: bar}
}

func (b *Bar) NewTeeReader(r io.Reader) io.Reader {
	if b.bar == nil {
		return r
	}
	return io.TeeReader(r, b.bar)
}

func (b *Bar) Add(n int) {
	if b.bar != nil {
		_ = b.bar.Add(n)
	}
}

func (b *Bar) Finish() {
	if b.bar != nil {
		_ = b.bar.Finish()
	}
}

func (b *Bar) Exit() {
	if b.bar != nil {
		_ = b.bar.Exit()
	}
}
