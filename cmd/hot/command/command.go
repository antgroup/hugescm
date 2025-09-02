// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/version"
)

var (
	ErrLocalEndpoint    = errors.New("local endpoint")
	ErrWorktreeNotEmpty = errors.New("worktree not empty")
)

type Globals struct {
	Verbose bool        `short:"V" help:"Make the operation more talkative"`
	Version VersionFlag `short:"v" name:"version" help:"Show version number and quit"`
}

type VersionFlag bool

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(version.GetVersionString())
	app.Exit(0)
	return nil
}

func pickURI(rawURL string) (string, error) {
	if git.MatchesScpLike(rawURL) {
		_, _, _, p := git.FindScpLikeComponents(rawURL)
		return p, nil
	}
	if git.MatchesScheme(rawURL) {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", err
		}
		return u.Path, nil
	}
	return "", ErrLocalEndpoint
}

func (g *Globals) RunEx(ctx context.Context, repoPath string, cmdArg0 string, args ...string) error {
	now := time.Now()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  repoPath,
		Environ:   os.Environ(),
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
