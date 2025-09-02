// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package bar

import (
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/progressbar"
)

type ProgressBar struct {
	bar         *progressbar.ProgressBar
	total       int
	stepCurrent int
	stepEnd     int
}

func NewBar(description string, total int, stepCurrent, stepEnd int, verbose bool) *ProgressBar {
	if verbose {
		return &ProgressBar{}
	}
	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription(fmt.Sprintf("\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s...", stepCurrent, stepEnd, description)),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "\x1b[38;2;72;198;239m#\x1b[0m",
			SaucerHead:    "\x1b[38;2;72;198;239m>\x1b[0m",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	return &ProgressBar{bar: bar, total: total, stepCurrent: stepCurrent, stepEnd: stepEnd}
}

func (b *ProgressBar) Add(n int) {
	if b.bar != nil {
		b.bar.Add(n)
	}
}

func (b *ProgressBar) Done() {
	if b.bar == nil {
		return
	}
	_ = b.bar.Finish()
	if b.total <= 0 {
		fmt.Fprintf(os.Stderr, "\n\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s.\n", b.stepCurrent, b.stepEnd, tr.W("processing completed"))
		return
	}
	fmt.Fprintf(os.Stderr, "\n\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s, %s: %d\n", b.stepCurrent, b.stepEnd, tr.W("processing completed"), tr.W("total"), b.total)
}
