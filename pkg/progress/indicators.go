// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/tr"
)

var (
	selectedSpinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

type Indicators struct {
	description string
	completed   string
	quiet       bool
	current     uint64
	total       uint64
	wg          sync.WaitGroup
}

func NewIndicators(description, completed string, total uint64, quiet bool) *Indicators {
	return &Indicators{description: tr.W(description), completed: tr.W(completed), total: total, quiet: quiet}
}

func (i *Indicators) Add(n int) {
	atomic.AddUint64(&i.current, uint64(n))
}

func (i *Indicators) Wait() {
	i.wg.Wait()
}

func (i *Indicators) Run(ctx context.Context) {
	if i.quiet {
		return
	}
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		blue := blueColorMap[term.StderrLevel]
		end := endColorMap[term.StderrLevel]
		startTime := time.Now()
		tick := time.NewTicker(time.Millisecond * 100)
		for {
			select {
			case <-ctx.Done():
				current := atomic.LoadUint64(&i.current)
				if err := context.Cause(ctx); errors.Is(err, context.Canceled) {
					if i.total == 0 {
						fmt.Fprintf(os.Stderr, "\x1b[2K\r%s, %s: %d, %s: %v%s\n",
							i.completed, tr.W("total"), current, tr.W("time spent"), time.Since(startTime).Truncate(time.Millisecond), end)
						return
					}
					fmt.Fprintf(os.Stderr, "\x1b[2K\r%s: %d%% (%d/%d) %s, %s: %v%s\n",
						i.description, 100*current/i.total, current, i.total, tr.W("completed"), tr.W("time spent"), time.Since(startTime).Truncate(time.Millisecond), end)
					return
				}
				return
			case <-tick.C:
				current := atomic.LoadUint64(&i.current)
				spinner := selectedSpinner[int(math.Round(math.Mod(float64(time.Since(startTime).Milliseconds()/100), float64(len(selectedSpinner)))))]
				if i.total == 0 {
					fmt.Fprintf(os.Stderr, "\x1b[2K\r%s %s... %s%d%s", blue, spinner, i.description, current, end)
				} else {
					fmt.Fprintf(os.Stderr, "\x1b[2K\r%s %s... %s%d%% (%d/%d)%s", blue, spinner, i.description, 100*current/i.total, current, i.total, end)
				}
			}
		}
	}()
}
