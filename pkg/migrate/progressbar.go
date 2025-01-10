// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/progressbar"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/tr"
)

type ProgressBar struct {
	bar         *progressbar.ProgressBar
	total       int
	stepCurrent int
	stepEnd     int
}

func makeProgressBarTheme() progressbar.Theme {
	switch term.StdoutLevel {
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

func makeDescription(description string, stepCurrent, stepEnd int) string {
	switch term.StdoutLevel {
	case term.Level256:
		return fmt.Sprintf("\x1b[36m[%d/%d]\x1b[0m %s...", stepCurrent, stepEnd, description)
	case term.Level16M:
		return fmt.Sprintf("\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s...", stepCurrent, stepEnd, description)
	default:
	}
	return fmt.Sprintf("[%d/%d] %s...", stepCurrent, stepEnd, description)
}

func NewBar(description string, total int, stepCurrent, stepEnd int, verbose bool) *ProgressBar {
	if verbose {
		return &ProgressBar{}
	}
	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription(makeDescription(description, stepCurrent, stepEnd)),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(makeProgressBarTheme()))

	return &ProgressBar{bar: bar, total: total, stepCurrent: stepCurrent, stepEnd: stepEnd}
}

func (b *ProgressBar) Add(n int) {
	if b.bar != nil {
		_ = b.bar.Add(n)
	}
}

var (
	startColor = map[term.Level]string{
		term.Level256: "\x1b[36m",
		term.Level16M: "\x1b[38;2;72;198;239m",
	}
	endColor = map[term.Level]string{
		term.Level256: "\x1b[0m",
		term.Level16M: "\x1b[0m",
	}
)

func (b *ProgressBar) Done() {
	if b.bar == nil {
		return
	}
	_ = b.bar.Finish()
	if b.total <= 0 {
		fmt.Fprintf(os.Stderr, "\n%s[%d/%d]%s %s.\n", startColor[term.StderrLevel], b.stepCurrent, b.stepEnd, endColor[term.StdoutLevel], tr.W("processing completed"))
		return
	}
	fmt.Fprintf(os.Stderr, "\n\x1b%s[%d/%d]%s %s, %s: %d\n", startColor[term.StderrLevel], b.stepCurrent, b.stepEnd, endColor[term.StdoutLevel], tr.W("processing completed"), tr.W("total"), b.total)
}
