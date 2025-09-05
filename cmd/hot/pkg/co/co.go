package co

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type CoOptions struct {
	Remote, Destination string
	Branch, Commit      string
	Sparse              []string
	Depth               int
	Limit               int64
	Recursive           bool
}

var (
	newEnviron = sync.OnceValue(func() []string {
		env := slices.Clone(os.Environ())
		if ua, ok := NewUserAgent(); ok {
			env = append(env, "GIT_USER_AGENT="+ua)
		}
		return env
	})
)

func run(ctx context.Context, repoPath string, cmdArg0 string, args ...string) error {
	now := time.Now()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  repoPath,
		Environ:   newEnviron(),
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		Stdin:     os.Stdin,
		NoSetpgid: true,
	}, cmdArg0, args...)
	if err := cmd.Run(); err != nil {
		return err
	}
	trace.DbgPrint("exec: %s spent: %v", cmd.String(), time.Since(now))
	return nil
}

func fetch(ctx context.Context, o *CoOptions) error {
	now := time.Now()
	if err := git.NewRepo(ctx, o.Destination, git.ReferenceNameDefault, false, git.HashFormatFromSize(len(o.Commit))); err != nil {
		fmt.Fprintf(os.Stderr, "initialize repository '%s' error: %v\n", o.Destination, err)
		return err
	}
	if err := run(ctx, o.Destination, "git", "config", "index.version", "4"); err != nil {
		fmt.Fprintf(os.Stderr, "config index v4 error: %v\n", err)
		return err
	}
	if err := run(ctx, o.Destination, "git", "remote", "add", "origin", o.Remote); err != nil {
		fmt.Fprintf(os.Stderr, "add remote error: %v\n", err)
		return err
	}
	if len(o.Sparse) != 0 {
		if err := sparseCheckout(ctx, o); err != nil {
			return err
		}
	}
	fetchArgs := make([]string, 0, 10)
	fetchArgs = append(fetchArgs, "fetch")
	if o.Depth > 0 && o.Depth < 20 {
		fetchArgs = append(fetchArgs, "--depth="+strconv.Itoa(o.Depth))
	}
	fetchArgs = append(fetchArgs, "origin", o.Commit)
	if err := run(ctx, o.Destination, "git", fetchArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "fetch error: %v", err)
		return err
	}
	// git switch [<options>] [--no-guess] <branch>
	// git switch [<options>] --detach [<start-point>]
	// git switch [<options>] (-c|-C) <new-branch> [<start-point>]
	// git switch [<options>] --orphan <new-branch>
	switchArgs := make([]string, 0, 10)
	switchArgs = append(switchArgs, "switch")
	if len(o.Branch) == 0 {
		switchArgs = append(switchArgs, "--detach", o.Commit)
	} else {
		switchArgs = append(switchArgs, "-c", o.Branch, o.Commit)
	}
	if err := run(ctx, o.Destination, "git", switchArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "switch error: %v", err)
		return err
	}
	if o.Recursive {
		if err := run(ctx, o.Destination, "git", "submodule", "update", "--init", "--recursive", "--recommend-shallow"); err != nil {
			fmt.Fprintf(os.Stderr, "switch error: %v", err)
			return err
		}
	}
	_, _ = tr.Fprintf(os.Stderr, "Cloning to '%s' completed, spent: %v.\n", o.Destination, time.Since(now))
	return nil
}

func Co(ctx context.Context, o *CoOptions) error {
	if len(o.Commit) != 0 && !git.IsGitVersionAtLeast(git.NewVersion(2, 50, 0)) {
		return fetch(ctx, o)
	}
	return clone(ctx, o)
}

func sparseCheckout(ctx context.Context, o *CoOptions) error {
	now := time.Now()
	// https://git-scm.com/docs/git-sparse-checkout#Documentation/git-sparse-checkout.txt-emsetem
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  o.Destination,
		Environ:   newEnviron(),
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		NoSetpgid: true,
	}, "git", "sparse-checkout", "set", "--cone", "--sparse-index", "--stdin")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize sparse checkout error: %v\n", err)
		return err
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "initialize sparse checkout error: %v\n", err)
		_ = stdin.Close()
		return err
	}
	// https://git-scm.com/docs/git-sparse-checkout#Documentation/git-sparse-checkout.txt-codegitsparse-checkoutsetMYDIR1SUBDIR2code
	for _, s := range o.Sparse {
		if _, err := stdin.Write([]byte(s + "\n")); err != nil {
			fmt.Fprintf(os.Stderr, "initialize sparse checkout error: %v\n", err)
			_ = stdin.Close()
			_ = cmd.Wait()
			return err
		}
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "initialize sparse checkout error: %v\n", err)
		return err
	}
	trace.DbgPrint("git space-checkout spent: %v", time.Since(now))
	return nil
}

func clone(ctx context.Context, o *CoOptions) error {
	now := time.Now()
	cloneArgs := make([]string, 0, 20)
	cloneArgs = append(cloneArgs, "-c", "index.version=4", "-c", "advice.detachedHead=false", "clone")
	switch {
	case len(o.Sparse) != 0 && o.Limit >= 0:
		cloneArgs = append(cloneArgs, "--sparse", fmt.Sprintf("--filter=blob:limit=%d", o.Limit), "--no-checkout")
	case len(o.Sparse) != 0:
		cloneArgs = append(cloneArgs, "--sparse", "--filter=blob:none", "--no-checkout")
	case o.Limit >= 0:
		cloneArgs = append(cloneArgs, fmt.Sprintf("--filter=blob:limit=%d", o.Limit))
	}
	switch {
	case len(o.Commit) != 0:
		cloneArgs = append(cloneArgs, "--revision", o.Commit)
	case len(o.Branch) != 0:
		cloneArgs = append(cloneArgs, "--single-branch", "--branch", o.Branch)
	}
	if o.Depth > 0 && o.Depth < 20 {
		cloneArgs = append(cloneArgs, "--depth="+strconv.Itoa(o.Depth))
	}
	if o.Recursive {
		cloneArgs = append(cloneArgs, "recursive", "--shallow-submodules") // submodule shallow
	}
	cloneArgs = append(cloneArgs, o.Remote, o.Destination)
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ:   newEnviron(),
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		Stdin:     os.Stdin,
		NoSetpgid: true,
	}, "git", cloneArgs...)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "clone error: %v", err)
		return err
	}
	if len(o.Branch) != 0 && len(o.Commit) != 0 {
		if err := run(ctx, o.Destination, "git", "switch", "-c", o.Branch, o.Commit); err != nil {
			fmt.Fprintf(os.Stderr, "switch error: %v", err)
			return err
		}
	}
	trace.DbgPrint("git clone spent: %v", time.Since(now))
	if len(o.Sparse) != 0 {
		if err := sparseCheckout(ctx, o); err != nil {
			return err
		}
		if err := run(ctx, o.Destination, "git", "checkout", "HEAD"); err != nil {
			fmt.Fprintf(os.Stderr, "checkout error: %v\n", err)
			return err
		}
	}
	_, _ = tr.Fprintf(os.Stderr, "Cloning to '%s' completed, spent: %v.\n", o.Destination, time.Since(now))
	return nil
}
