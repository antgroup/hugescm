package git

import (
	"context"
	"errors"
	"io"

	"github.com/antgroup/hugescm/modules/command"
)

type commandReader struct {
	cmd    *command.Command
	reader io.ReadCloser
}

func (c *commandReader) Read(p []byte) (int, error) {
	if c.reader == nil {
		panic("command has no reader")
	}
	return c.reader.Read(p)
}

func (c *commandReader) Close() (err error) {
	if c.reader != nil {
		_ = c.reader.Close()
	}
	return c.cmd.Wait()
}

// NewReaderFromOptions new git command as a reader
func NewReader(ctx context.Context, opt *command.RunOpts, arg ...string) (io.ReadCloser, error) {
	if opt.Stdout != nil {
		return nil, errors.New("exec: Stdout should be nil")
	}
	cmdArgs := append([]string{"--git-dir", opt.RepoPath}, arg...)
	cmd := command.NewFromOptions(ctx, opt, "git", cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return nil, err
	}
	return &commandReader{cmd: cmd, reader: stdout}, nil
}
