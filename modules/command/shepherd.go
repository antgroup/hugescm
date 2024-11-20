package command

import (
	"context"
	"io"
	"os/exec"
	"sync/atomic"

	"github.com/antgroup/hugescm/modules/env"
)

type RunOpts struct {
	Environ   []string  // As environ
	ExtraEnv  []string  // append to env
	RepoPath  string    // dir
	Stderr    io.Writer // stderr
	Stdout    io.Writer // stdout
	Stdin     io.Reader // stdin
	Detached  bool      //Detached If true, the child process will not be terminated when the parent process ends
	NoSetpgid bool
}

type Shepherder interface {
	// NewFromOptions: Create command with options
	NewFromOptions(ctx context.Context, opt *RunOpts, name string, arg ...string) *Command
	// New: Create a process with environment variable isolation
	New(ctx context.Context, repoPath string, name string, arg ...string) *Command
	// ProcessesCount: Get the number of child processes
	ProcessesCount() int32
}

type shepherder struct {
	env.Builder
	count int32
}

func (s *shepherder) inc() int32 {
	return atomic.AddInt32(&s.count, 1)
}

func (s *shepherder) dec() int32 {
	return atomic.AddInt32(&s.count, -1)
}

func (s *shepherder) ProcessesCount() int32 {
	return atomic.LoadInt32(&s.count)
}

func NewShepherder(b env.Builder) Shepherder {
	return &shepherder{Builder: b}
}

// New new command:
func (s *shepherder) New(ctx context.Context, repoPath string, name string, arg ...string) *Command {
	return s.NewFromOptions(ctx, &RunOpts{RepoPath: repoPath}, name, arg...)
}

func (s *shepherder) NewFromOptions(ctx context.Context, opt *RunOpts, name string, arg ...string) *Command {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Dir = opt.RepoPath
	if len(opt.Environ) == 0 {
		cmd.Env = append(cmd.Env, s.Environ()...)
	} else {
		cmd.Env = append(cmd.Env, opt.Environ...)
	}
	if len(opt.ExtraEnv) != 0 {
		cmd.Env = append(cmd.Env, opt.ExtraEnv...)
	}
	cmd.Stderr = opt.Stderr
	cmd.Stdout = opt.Stdout
	cmd.Stdin = opt.Stdin
	c := &Command{rawCmd: cmd, context: ctx, s: s, detached: opt.Detached}
	if !opt.NoSetpgid {
		setSysProcAttribute(cmd, c.detached)
	}
	return c
}

var (
	shepherd = NewShepherder(env.NewBuilder())
)

// Create an isolated process based on shepherd
func NewFromOptions(ctx context.Context, opt *RunOpts, name string, arg ...string) *Command {
	return shepherd.NewFromOptions(ctx, opt, name, arg...)
}

// Create an isolated process based on shepherd
func New(ctx context.Context, repoPath string, name string, arg ...string) *Command {
	return shepherd.New(ctx, repoPath, name, arg...)
}

// ProcessesCount: Get the number of child processes of the default shepherd
func ProcessesCount() int32 {
	return shepherd.ProcessesCount()
}
