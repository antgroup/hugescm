package command

import (
	"context"

	"github.com/antgroup/hugescm/cmd/hot/pkg/stat"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type Stat struct {
	CWD   string `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Limit int64  `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB, MB, GB, K, M, G" default:"20m" type:"size"`
}

func (c *Stat) Run(ctx context.Context, g *Globals) error {
	repoPath := git.RevParseRepoPath(ctx, c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)
	return stat.Stat(ctx, &stat.StatOptions{
		RepoPath: repoPath,
		Limit:    c.Limit,
	})
}
