package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

func IsBareRepository(ctx context.Context, repoPath string) bool {
	cmd := command.New(ctx, command.NoDir, "git", "--git-dir", repoPath, "config", "--get", "core.bare")
	v, err := cmd.OneLine()
	if err != nil {
		return false
	}
	return strings.EqualFold(v, "true")
}

const (
	differentHashErr     = "fatal: attempt to reinitialize repository with different hash"
	invalidBranchNameErr = "fatal: invalid initial branch name"
)

var (
	ErrDifferentHash     = errors.New("attempt to reinitialize repository with different hash")
	ErrInvalidBranchName = errors.New("invalid initial branch name")
)

func NewRepo(ctx context.Context, repoPath, branch string, bare bool, shaFormat HashFormat) error {
	branch = strings.TrimPrefix(branch, refHeadPrefix)
	stderr := command.NewStderr()
	psArgs := []string{"init", "--initial-branch=" + branch, "--object-format=" + shaFormat.String()}

	if bare {
		psArgs = append(psArgs, "--bare")
	}
	psArgs = append(psArgs, repoPath)
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Stderr: stderr,
	}, "git", psArgs...)
	if err := cmd.RunEx(); err != nil {
		message := stderr.String()
		if strings.HasPrefix(message, differentHashErr) {
			return ErrDifferentHash
		}
		if strings.HasPrefix(message, invalidBranchNameErr) {
			return ErrInvalidBranchName
		}
		return fmt.Errorf("initialize repo %s error %v stderr: %s", repoPath, err, message)
	}
	return nil
}
