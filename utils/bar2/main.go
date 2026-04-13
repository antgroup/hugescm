// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// utils/bar2 is a standalone demo that exercises the MultiBar renderer.
// It simulates three concurrent downloads with randomised speeds and shows
// the bubbles-based parallel progress bars in action.
//
// Run with:
//
//	go run ./utils/bar2
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/progress"
)

// termWidth returns the visible width of the current terminal.
// It can be replaced in tests.
var termWidth = func() (width int, err error) {
	width, _, err = term.GetSize(int(os.Stderr.Fd()))
	if err == nil {
		return width, nil
	}
	return 0, err
}

func main() {
	const numTasks = 3
	width, err := termWidth()
	if err != nil {
		width = 80
	}

	mb := progress.NewMultiBar(width)

	type task struct {
		label string
		size  int64 // simulated total bytes
	}
	tasks := []task{
		{label: "Downloading a1b2c3d4", size: 12 * 1024 * 1024},
		{label: "Downloading e5f6a7b8", size: 5 * 1024 * 1024},
		{label: "Downloading c9d0e1f2", size: 30 * 1024 * 1024},
	}

	bars := make([]*progress.TransferBar, numTasks)
	for i, t := range tasks {
		bars[i] = mb.AddBar(t.label)
	}

	// Launch one goroutine per task to simulate a download.
	for i, t := range tasks {
		bar := bars[i]
		totalBytes := t.size
		go func(bar *progress.TransferBar, total int64) {
			bar.SetTotal(total)

			var transferred int64
			// chunk size: 256 KiB ± random jitter
			const baseChunk = 256 * 1024
			for transferred < total {
				chunk := int64(baseChunk + rand.IntN(baseChunk))
				if transferred+chunk > total {
					chunk = total - transferred
				}
				// simulate network latency (10–80 ms per chunk)
				delay := time.Duration(10+rand.IntN(70)) * time.Millisecond
				time.Sleep(delay)
				transferred += chunk
				bar.SetCurrent(transferred)
			}
			bar.Complete()
		}(bar, totalBytes)
	}

	if err := mb.Run(os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "progress error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "all downloads complete.")
}
