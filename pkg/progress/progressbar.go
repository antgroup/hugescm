// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/progressbar"
)

type ProgressBar struct {
	bar   *progressbar.ProgressBar
	total int
}

func NewBar(description string, total int, quiet bool) *ProgressBar {
	if quiet {
		return &ProgressBar{}
	}
	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription(fmt.Sprintf("\x1b[0m%s...", description)),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "\x1b[38;2;72;198;239m#\x1b[0m",
			SaucerHead:    "\x1b[38;2;72;198;239m>\x1b[0m",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	return &ProgressBar{bar: bar, total: total}
}

func (b *ProgressBar) Add(n int) {
	if b.bar != nil {
		_ = b.bar.Add(n)
	}
}

func (b *ProgressBar) Done() int {
	if b.bar == nil {
		return 0
	}
	_ = b.bar.Finish()
	fmt.Fprintln(os.Stderr, "\x1b[0m")
	if b.total <= 0 {
		return 0
	}
	return b.total
}
