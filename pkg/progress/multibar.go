// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package progress provides terminal progress bar utilities.
// This file implements a concurrent multi-bar renderer that uses
// bubbles/progress for bar styling and a lightweight inline render loop
// for multi-line in-place refresh — no bubbletea Program overhead.
package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

// ── colour palette ────────────────────────────────────────────────────────────

var (
	colorBarFill  = lipgloss.Color("#2BC0FE") // cyan
	colorBarTrail = lipgloss.Color("#4F6EF7") // indigo
	styleLabel    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8B5C8"))
	styleDone     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80")) // green
	styleFail     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")) // red
	styleSpeed    = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")) // slate
)

// ── bar lifecycle ─────────────────────────────────────────────────────────────

type barState int32

const (
	stateRunning barState = iota
	stateDone
	stateFailed
)

// taskEntry holds the mutable state for one download task.
// Written by download goroutines, read by the render loop.
type taskEntry struct {
	label   string
	total   atomic.Int64
	current atomic.Int64
	state   atomic.Int32 // barState

	// EWMA speed tracking (guarded by mu)
	mu        sync.Mutex
	lastBytes int64
	lastTime  time.Time
	speedBps  float64
}

func newTaskEntry(label string) *taskEntry {
	return &taskEntry{label: label, lastTime: time.Now()}
}

func (e *taskEntry) sampleSpeed() {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	cur := e.current.Load()
	dt := now.Sub(e.lastTime).Seconds()
	if dt > 0 {
		instant := float64(cur-e.lastBytes) / dt
		e.speedBps = 0.8*e.speedBps + 0.2*instant // α = 0.2 EWMA
	}
	e.lastBytes = cur
	e.lastTime = now
}

func (e *taskEntry) readSpeed() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.speedBps
}

// ── TransferBar ───────────────────────────────────────────────────────────────

// TransferBar is the per-task progress handle returned by [MultiBar.AddBar].
// It is safe for concurrent use from download goroutines.
type TransferBar struct {
	entry *taskEntry
}

// SetTotal sets the known total size in bytes.
// May be called more than once (e.g. after a redirect reveals Content-Length).
func (b *TransferBar) SetTotal(total int64) {
	b.entry.total.Store(total)
}

// SetCurrent sets the number of bytes already transferred.
// Useful when resuming a partial download.
func (b *TransferBar) SetCurrent(current int64) {
	b.entry.current.Store(current)
}

// ProxyReader wraps r so that every Read advances the bar automatically.
// It returns an (io.Reader, io.Closer) pair to match the odb.DoTransfer
// callback signature; the Closer is a harmless no-op.
func (b *TransferBar) ProxyReader(r io.Reader) (io.Reader, io.Closer) {
	cr := &countingReader{r: r, entry: b.entry}
	return cr, cr
}

// Complete marks the task as successfully finished.
func (b *TransferBar) Complete() {
	b.entry.state.Store(int32(stateDone))
}

// Fail marks the task as failed.
func (b *TransferBar) Fail() {
	b.entry.state.Store(int32(stateFailed))
}

// countingReader increments the task's byte counter on every Read.
type countingReader struct {
	r     io.Reader
	entry *taskEntry
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.entry.current.Add(int64(n))
	}
	return n, err
}

func (c *countingReader) Close() error { return nil }

// ── MultiBar ──────────────────────────────────────────────────────────────────

const (
	refreshInterval = 100 * time.Millisecond
	labelWidth      = 20

	// ANSI sequences used for inline multi-line refresh
	ansiClearLine  = "\x1b[2K"  // erase entire current line
	ansiCursorUp   = "\x1b[%dA" // move cursor up N lines
	ansiCursorHome = "\r"       // carriage return
)

// MultiBar manages a set of concurrent download progress bars.
// It renders them inline (in-place, multi-line) using only ANSI cursor
// sequences — no bubbletea Program, no terminal capability queries.
//
// Typical usage:
//
//	mb := progress.NewMultiBar(width)
//	b1 := mb.AddBar("Downloading abc123")
//	b2 := mb.AddBar("Downloading def456")
//	go func() { /* use b1 */ }()
//	go func() { /* use b2 */ }()
//	_ = mb.Run(os.Stderr)
type MultiBar struct {
	mu    sync.Mutex
	tasks []*taskEntry
	width int
}

// NewMultiBar creates a MultiBar using the given terminal width.
// Pass 0 to use the default fallback width (80 columns).
func NewMultiBar(width int) *MultiBar {
	if width <= 0 {
		width = 80
	}
	return &MultiBar{width: width}
}

// AddBar registers a new download task and returns its [TransferBar].
// Must be called before [MultiBar.Run].
func (mb *MultiBar) AddBar(label string) *TransferBar {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	t := newTaskEntry(label)
	mb.tasks = append(mb.tasks, t)
	return &TransferBar{entry: t}
}

// Run starts the inline render loop and blocks until every bar has been
// marked [TransferBar.Complete] or [TransferBar.Fail].
// w is typically os.Stderr.
func (mb *MultiBar) Run(w io.Writer) error {
	mb.mu.Lock()
	tasks := mb.tasks
	width := mb.width
	mb.mu.Unlock()

	if len(tasks) == 0 {
		return nil
	}

	renderer := newInlineRenderer(tasks, width)
	return renderer.loop(w)
}

// ── inlineRenderer ────────────────────────────────────────────────────────────

// inlineRenderer owns the progress.Model instances and drives the render loop.
type inlineRenderer struct {
	tasks     []*taskEntry
	bars      []progress.Model
	termWidth int
	// track whether we have already printed the initial block
	firstDraw bool
}

func newInlineRenderer(tasks []*taskEntry, termWidth int) *inlineRenderer {
	r := &inlineRenderer{tasks: tasks, termWidth: termWidth, firstDraw: true}
	r.rebuildBars()
	return r
}

func (r *inlineRenderer) rebuildBars() {
	bw := barContentWidth(r.termWidth)
	bars := make([]progress.Model, len(r.tasks))
	for i := range r.tasks {
		bars[i] = progress.New(
			progress.WithWidth(bw),
			progress.WithColors(colorBarFill, colorBarTrail),
			progress.WithoutPercentage(),
		)
	}
	r.bars = bars
}

// loop ticks at refreshInterval, redraws all rows in-place, and returns when
// every task has settled.
func (r *inlineRenderer) loop(w io.Writer) error {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		// sample speeds for running tasks
		for _, t := range r.tasks {
			if barState(t.state.Load()) == stateRunning {
				t.sampleSpeed()
			}
		}

		r.redraw(w)

		if r.allSettled() {
			// final newline so the shell prompt appears below the bars
			_, _ = fmt.Fprintln(w)
			return nil
		}
	}
}

// redraw erases and rewrites all bar rows in-place.
func (r *inlineRenderer) redraw(w io.Writer) {
	n := len(r.tasks)
	var sb strings.Builder

	if r.firstDraw {
		// first time: just print the block (cursor is already at the right spot)
		r.firstDraw = false
	} else {
		// move cursor back up to the first bar row
		fmt.Fprintf(&sb, ansiCursorUp, n)
	}

	for i, t := range r.tasks {
		sb.WriteString(ansiCursorHome)
		sb.WriteString(ansiClearLine)
		sb.WriteString(r.renderRow(i, t))
		sb.WriteByte('\n')
	}

	_, _ = fmt.Fprint(w, sb.String())
}

func (r *inlineRenderer) renderRow(i int, t *taskEntry) string {
	state := barState(t.state.Load())
	total := t.total.Load()
	current := t.current.Load()

	label := styleLabel.Render(fmt.Sprintf("%-*s", labelWidth, truncate(t.label, labelWidth)))
	bar := r.bars[i].ViewAs(pct(current, total))
	size := formatBytes(current)

	switch state {
	case stateDone:
		return fmt.Sprintf("%s %s  %s  %s", label, bar, size, styleDone.Render("done"))
	case stateFailed:
		return fmt.Sprintf("%s %s  %s  %s", label, bar, size, styleFail.Render("failed"))
	default:
		spd := styleSpeed.Render(formatSpeed(t.readSpeed()))
		return fmt.Sprintf("%s %s  %s  %s", label, bar, size, spd)
	}
}

func (r *inlineRenderer) allSettled() bool {
	for _, t := range r.tasks {
		if barState(t.state.Load()) == stateRunning {
			return false
		}
	}
	return true
}

// ── helpers ───────────────────────────────────────────────────────────────────

// barContentWidth computes the width to pass to progress.WithWidth, leaving
// room for the label, size, and speed/status columns.
func barContentWidth(termWidth int) int {
	// label(20) + space(1) + bar + space(2) + size(9) + space(2) + suffix(14)
	const overhead = labelWidth + 1 + 2 + 9 + 2 + 14
	return max(termWidth-overhead, 10)
}

func pct(current, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(current) / float64(total)
	if p > 1 {
		return 1
	}
	return p
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%6d  B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%6.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatSpeed(bps float64) string {
	if bps < 1 {
		return "       ---    "
	}
	const unit = 1024.0
	if bps < unit {
		return fmt.Sprintf("%6.1f  B/s", bps)
	}
	div, exp := unit, 0
	for n := bps / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%6.1f %ciB/s", bps/div, "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
