package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	STDERR_BUFFER_LIMIT = 8 * 1024
	STDERR_BUFFER_GROUP = 512
)

type LimitStderr struct {
	*strings.Builder
	limit int
}

func NewStderr() *LimitStderr {
	b := &strings.Builder{}
	b.Grow(STDERR_BUFFER_GROUP)
	return &LimitStderr{Builder: b, limit: STDERR_BUFFER_LIMIT}
}

func (w *LimitStderr) Bytes() []byte {
	return []byte(w.String())
}

func (w *LimitStderr) Write(p []byte) (int, error) {
	n := len(p)
	var err error
	if w.limit > 0 {
		if n > w.limit {
			p = p[:w.limit]
		}
		w.limit -= len(p)
		_, err = w.Builder.Write(p)
	}
	return n, err
}

type Command struct {
	rawCmd    *exec.Cmd
	context   context.Context
	startTime time.Time
	s         *shepherd
	detached  bool
	once      sync.Once
	waitError error
}

func (c *Command) Start() error {
	c.startTime = time.Now()
	if c.rawCmd.Stderr == nil {
		c.rawCmd.Stderr = os.Stderr
	}
	if err := c.rawCmd.Start(); err != nil {
		return err
	}
	c.s.inc()
	return nil
}

func (c *Command) wait() {
	if err := c.rawCmd.Wait(); err != nil && c.context.Err() != context.DeadlineExceeded {
		c.waitError = err
		return
	}
	c.waitError = c.context.Err()
}

func (c *Command) Wait() error {
	c.once.Do(func() {
		if c.rawCmd == nil {
			return
		}
		c.wait()
		c.s.dec()
	})
	return c.waitError
}

func (c *Command) UseTime() time.Duration {
	return time.Since(c.startTime)
}

func (c *Command) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// prefixSuffixSaver is an io.Writer which retains the first N bytes
// and the last N bytes written to it. The Bytes() methods reconstructs
// it with a pretty error message.
type prefixSuffixSaver struct {
	N         int // max size of prefix or suffix
	prefix    []byte
	suffix    []byte // ring buffer once len(suffix) == N
	suffixOff int    // offset to write into suffix
	skipped   int64

	// TODO(bradfitz): we could keep one large []byte and use part of it for
	// the prefix, reserve space for the '... Omitting N bytes ...' message,
	// then the ring buffer suffix, and just rearrange the ring buffer
	// suffix when Bytes() is called, but it doesn't seem worth it for
	// now just for error messages. It's only ~64KB anyway.
}

func (w *prefixSuffixSaver) Write(p []byte) (n int, err error) {
	lenp := len(p)
	p = w.fill(&w.prefix, p)

	// Only keep the last w.N bytes of suffix data.
	if overage := len(p) - w.N; overage > 0 {
		p = p[overage:]
		w.skipped += int64(overage)
	}
	p = w.fill(&w.suffix, p)

	// w.suffix is full now if p is non-empty. Overwrite it in a circle.
	for len(p) > 0 { // 0, 1, or 2 iterations.
		n := copy(w.suffix[w.suffixOff:], p)
		p = p[n:]
		w.skipped += int64(n)
		w.suffixOff += n
		if w.suffixOff == w.N {
			w.suffixOff = 0
		}
	}
	return lenp, nil
}

// fill appends up to len(p) bytes of p to *dst, such that *dst does not
// grow larger than w.N. It returns the un-appended suffix of p.
func (w *prefixSuffixSaver) fill(dst *[]byte, p []byte) (pRemain []byte) {
	if remain := w.N - len(*dst); remain > 0 {
		add := minInt(len(p), remain)
		*dst = append(*dst, p[:add]...)
		p = p[add:]
	}
	return p
}

func (w *prefixSuffixSaver) Bytes() []byte {
	if w.suffix == nil {
		return w.prefix
	}
	if w.skipped == 0 {
		return append(w.prefix, w.suffix...)
	}
	var buf bytes.Buffer
	buf.Grow(len(w.prefix) + len(w.suffix) + 50)
	buf.Write(w.prefix)
	buf.WriteString("\n... omitting ")
	buf.WriteString(strconv.FormatInt(w.skipped, 10))
	buf.WriteString(" bytes ...\n")
	buf.Write(w.suffix[w.suffixOff:])
	buf.Write(w.suffix[:w.suffixOff])
	return buf.Bytes()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Command) Environ() []string {
	return c.rawCmd.Environ()
}

func (c *Command) StdoutPipe() (io.ReadCloser, error) {
	return c.rawCmd.StdoutPipe()
}

func (c *Command) StderrPipe() (io.ReadCloser, error) {
	return c.rawCmd.StderrPipe()
}

func (c *Command) StdinPipe() (io.WriteCloser, error) {
	return c.rawCmd.StdinPipe()
}

func (c *Command) Output() ([]byte, error) {
	if c.rawCmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	c.rawCmd.Stdout = &stdout

	captureErr := c.rawCmd.Stderr == nil
	if captureErr {
		c.rawCmd.Stderr = &prefixSuffixSaver{N: 32 << 10}
	}

	err := c.Run()
	if err != nil && captureErr {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = c.rawCmd.Stderr.(*prefixSuffixSaver).Bytes()
		}
	}
	return stdout.Bytes(), err
}

func (c *Command) OneLine() (string, error) {
	b, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (c *Command) RunEx() error {
	captureErr := c.rawCmd.Stderr == nil
	if captureErr {
		c.rawCmd.Stderr = &prefixSuffixSaver{N: 32 << 10}
	}

	err := c.Run()
	if err != nil && captureErr {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = c.rawCmd.Stderr.(*prefixSuffixSaver).Bytes()
		}
	}
	return err
}

func (c *Command) String() string {
	b := new(strings.Builder)
	b.WriteString("[")
	b.WriteString(c.rawCmd.Dir)
	b.WriteString("] ")
	b.WriteString(c.rawCmd.Path)
	for _, a := range c.rawCmd.Args[1:] {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	return b.String()
}

func (c *Command) Exit() error {
	cleanExit(c.rawCmd, c.detached)
	return c.Wait()
}
