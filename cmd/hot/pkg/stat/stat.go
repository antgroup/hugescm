package stat

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
)

type StatOptions struct {
	RepoPath string
}

func scanIdentity(ctx context.Context, repoPath string) {
	cmd := command.New(ctx, repoPath, "git", "config", "--get", "user.name")
	var err error
	var name, email string
	if name, err = cmd.OneLine(); err != nil || len(name) == 0 {
		fmt.Fprintf(os.Stderr, "missing configure 'user.name'\n")
	} else {
		fmt.Fprintf(os.Stderr, "Found 'user.name': %s\n", name)
	}
	cmd2 := command.New(ctx, repoPath, "git", "config", "--get", "user.email")
	if email, err = cmd2.OneLine(); err != nil || len(email) == 0 {
		fmt.Fprintf(os.Stderr, "missing configure 'user.email'\n")
		return
	}
}

func Stat(ctx context.Context, o *StatOptions) error {
	_, _ = tr.Fprintf(os.Stderr, "Location: %s\n", o.RepoPath)
	scanIdentity(ctx, o.RepoPath)
	shaFormat, refFormat := git.ExtensionsFormat(o.RepoPath)
	fmt.Fprintf(os.Stderr, "%s %s\n", shaFormat, refFormat)
	return nil
}
