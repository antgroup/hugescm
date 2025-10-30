package command

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/co"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type Co struct {
	From        string   `arg:"" name:"from" help:"Original repository remote URL" type:"string"`
	Destination string   `arg:"" optional:"" name:"destination" help:"Destination for the new repository" type:"path"`
	Branch      string   `name:"branch" short:"b" help:"Instead of pointing the newly created HEAD to the branch pointed to by the cloned repository’s HEAD, point to <name> branch instead"`
	Commit      string   `name:"commit" short:"c" help:"Instead of pointing the newly created HEAD to the branch pointed to by the cloned repository’s HEAD, point to <name> commit instead"`
	Sparse      []string `name:"sparse" short:"s" help:"A subset of repository files, all files are checked out by default" type:"string"`
	Depth       int      `name:"depth" short:"d" help:"Create a shallow clone with a history truncated to the specified number of commits" default:"5"`
	Limit       int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Recursive   bool     `name:"recursive" short:"r" help:"After the clone is created, initialize and clone submodules within based on the provided pathspec"`
	Values      []string `short:"X" shortonly:"" help:"Override default clone/fetch configuration, format: <key>=<value>"`
}

func (c *Co) concatDestination(baseName string) (string, error) {
	destination := c.Destination
	if len(destination) == 0 {
		destination = strings.TrimSuffix(baseName, ".git")
	}
	if !filepath.IsAbs(destination) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Get current workdir error: %v\n", err)
			return "", err
		}
		destination = filepath.Join(cwd, destination)
	}
	dirs, err := os.ReadDir(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return destination, nil
		}
		fmt.Fprintf(os.Stderr, "readdir %s error: %v\n", destination, err)
		return "", err
	}
	if len(dirs) != 0 {
		fmt.Fprintf(os.Stderr, "fatal: destination path '%s' already exists and is not an empty directory.\n", filepath.Base(destination))
		return "", ErrWorktreeNotEmpty
	}
	return destination, nil
}

func (c *Co) decodeRemote() (remote string, uri string, err error) {
	remote = c.From
	if git.MatchesScpLike(remote) {
		_, _, _, uri = git.FindScpLikeComponents(remote)
		return
	}
	if git.MatchesScheme(remote) {
		u, err := url.Parse(remote)
		if err != nil {
			return "", "", err
		}
		return remote, u.Path, nil
	}
	return remote, remote, nil
}

func (c *Co) Run(g *Globals) error {
	remote, uri, err := c.decodeRemote()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad remote '%s' error: '%v'\n", c.From, err)
		return err
	}
	destination, err := c.concatDestination(path.Base(uri))
	if err != nil {
		return err
	}
	trace.DbgPrint("%s --> %s", remote, destination)
	return co.Co(context.Background(), &co.CoOptions{
		Remote:      remote,
		Destination: destination,
		Branch:      c.Branch,
		Commit:      c.Commit,
		Sparse:      c.Sparse,
		Depth:       c.Depth,
		Limit:       c.Limit,
		Recursive:   c.Recursive,
		Values:      c.Values,
	})
}
