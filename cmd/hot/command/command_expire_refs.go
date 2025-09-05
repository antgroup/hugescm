package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antgroup/hugescm/cmd/hot/pkg/refs"
	"github.com/antgroup/hugescm/modules/fnmatch"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type ExpireRefs struct {
	Pattern []string      `arg:"" optional:"" name:"pattern" help:"Matching pattern, all references are displayed by default"`
	CWD     string        `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Merged  bool          `short:"M" name:"merged" help:"Only clean up merged branches, ignoring expiration times"`
	Tag     bool          `short:"T" name:"tag" help:"Clean up expired Tags, off by default"`
	Expires time.Duration `short:"E" name:"expires" help:"Reference expiration time, support: m, h, d, w" type:"expire" default:"90d"`
}

func (c *ExpireRefs) fixup() {
	for i, pattern := range c.Pattern {
		if strings.HasSuffix(pattern, "/") {
			c.Pattern[i] = pattern + "*"
		}
	}
}

func (c *ExpireRefs) Match(name string) bool {
	if len(c.Pattern) == 0 {
		return true
	}
	for _, pattern := range c.Pattern {
		if fnmatch.Match(pattern, name, 0) {
			return true
		}
	}
	return false
}

func (c *ExpireRefs) Expire(ref *refs.Reference) bool {
	if strings.HasPrefix(ref.Name, "refs/tmp/") {
		return true
	}
	if c.Merged {
		return ref.Merged()
	}
	// check ref is tag and cleanup tag
	if ref.IsTag() && !c.Tag {
		return false
	}
	return time.Since(ref.Committer.When) > c.Expires
}

func (c *ExpireRefs) Run(g *Globals) error {
	c.fixup()
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v expires: %v", repoPath, c.Expires)
	references, err := refs.ScanReferences(context.Background(), repoPath, c, git.OrderNone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo references error: %v\n", err)
		return err
	}
	if len(references.Items) == 0 {
		return nil
	}
	target := filepath.Join(repoPath, "logs/expire-refs.log")
	_ = os.MkdirAll(filepath.Dir(target), 0755)
	fd, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open logs error: %v\n", err)
		return err
	}
	defer fd.Close() // nolint
	_, _ = fmt.Fprintf(fd, "CLEANUP START TIME: %v\n", time.Now().Format(time.RFC3339))

	u, err := git.NewRefUpdater(context.Background(), repoPath, os.Environ(), false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RefUpdater: new ref updater error: %v\n", err)
		return err
	}
	defer u.Close() // nolint
	if err := u.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "RefUpdater: Start ref updater error: %v\n", err)
		return err
	}
	var total int
	for _, ref := range references.Items {
		if ref.Name == references.Current {
			continue
		}
		if ref.Broken {
			_ = refs.RemoveBrokenRef(repoPath, ref.Name)
			continue
		}
		if !c.Expire(ref) {
			continue
		}
		if err := u.Delete(git.ReferenceName(ref.Name)); err != nil {
			fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Delete %s error: %v\n", ref.Name, err)
			return err
		}
		total++
		date := ref.Committer.When.Format(time.RFC3339)
		_, _ = fmt.Fprintf(fd, "%s %s %s removed\n", ref.Hash, date, ref.Name)
		fmt.Fprintf(os.Stderr, "\x1b[2K\rDELETE '%s' (OID: %s)", ref.ShortName, ref.Hash)
	}
	if err := u.Prepare(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Prepare error: %v\n", err)
		return err
	}
	if err := u.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Commit error: %v\n", err)
		return err
	}
	if total != 0 {
		fmt.Fprintf(os.Stderr, "\nExpire refs success, total: %d\n", total)
	}
	return nil
}
