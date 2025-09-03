package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

const (
	snapshotSummaryFormat = `%shot snapshot [<options>]
%shot snapshot [<options>] --push [reference]
%shot snapshot [<options>] --push [<remote>] [<reference>]
`
)

type Snapshot struct {
	Message        []string `name:"message" short:"m" help:"Use the given as the commit message. Concatenate multiple -m options as separate paragraphs"`
	File           string   `name:"file" short:"F" help:"Take the commit message from the given file. Use - to read the message from the standard input"`
	Parents        []string `name:"parents" short:"p" help:"ID of a parent commit object"`
	CWD            string   `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Orphan         bool     `name:"orphan" help:"Create an orphan commit"`
	Push           bool     `name:"push" short:"P" help:"Push the worktree snapshot commit to the remote"`
	Force          bool     `name:"force" short:"f" help:"Force updates"`
	UnresolvedArgs []string `arg:"" optional:"" hidden:""`
	repoPath       string   `kong:"-"`
	worktree       string   `kong:"-"`
}

func (c *Snapshot) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(snapshotSummaryFormat, W("Usage: "), or, or)
}

func (c *Snapshot) Passthrough(paths []string) {
	c.UnresolvedArgs = append(c.UnresolvedArgs, paths...)
}

func messageReadFrom(r io.Reader) (string, error) {
	br := bufio.NewScanner(r)
	lines := make([]string, 0, 10)
	for br.Scan() {
		line := strings.TrimRightFunc(br.Text(), unicode.IsSpace)
		if strings.HasPrefix(line, "#") {
			break
		}
		lines = append(lines, line)
	}
	if br.Err() != nil {
		return "", br.Err()
	}
	var pos int
	for i, n := range lines {
		if len(n) != 0 {
			pos = i
			break
		}
	}
	lines = lines[pos:]
	if len(lines) == 0 {
		return "", nil
	}
	lines[0] = strings.TrimSpace(lines[0])
	if lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n"), nil
}

func messageReadFromPath(p string) (string, error) {
	fd, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer fd.Close() // nolint
	return messageReadFrom(fd)
}

func genMessage(message []string) string {
	if len(message) == 0 {
		return ""
	}
	lines := make([]string, 0, 10)
	lines = append(lines, strings.Split(message[0], "\n")...)
	if len(message) > 1 {
		lines = append(lines, message[1:]...)
	}
	var pos int
	for i, n := range lines {
		if len(n) != 0 {
			pos = i
			break
		}
	}
	lines = lines[pos:]
	if len(lines) == 0 {
		return ""
	}
	lines[0] = strings.TrimSpace(lines[0])
	if lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (c *Snapshot) genMessage() (message string, err error) {
	switch {
	case c.File == "-":
		if message, err = messageReadFrom(os.Stdin); err != nil {
			die("read messsage from stdin: %v", err)
			return
		}
	case len(c.File) != 0:
		if message, err = messageReadFromPath(c.File); err != nil {
			die("read messsage from %s: %v", c.File, err)
			return
		}
	default:
		message = genMessage(c.Message)
	}
	if len(message) == 0 {
		fmt.Fprintln(os.Stderr, W("Aborting commit due to empty commit message."))
		return "", errors.New("not allow empty message")
	}
	return
}

func (c *Snapshot) snapshotWriteIndex(ctx context.Context, snapshotEnv []string, treeish string) error {
	psArgs := []string{"read-tree"}
	if len(treeish) != 0 && !git.IsHashZero(treeish) {
		if !git.ValidateReferenceName([]byte(treeish)) {
			return fmt.Errorf("bad revision name '%s'", treeish)
		}
		psArgs = append(psArgs, "--", treeish)
	} else {
		psArgs = append(psArgs, "--empty")
	}

	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  c.repoPath,
		Environ:   snapshotEnv,
		Stderr:    os.Stderr,
		NoSetpgid: true,
	}, "git", psArgs...)
	return cmd.RunEx()
}

func (c *Snapshot) addALL(ctx context.Context, snapshotEnv []string) error {
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  c.worktree,
		Environ:   snapshotEnv,
		Stderr:    os.Stderr,
		NoSetpgid: true,
	}, "git", "add", "-A")
	return cmd.RunEx()
}

func (c *Snapshot) writeTree(ctx context.Context, snapshotEnv []string) (string, error) {
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  c.repoPath,
		Stderr:    os.Stderr,
		Environ:   snapshotEnv,
		NoSetpgid: true,
	}, "git", "write-tree")
	treeID, err := cmd.OneLine()
	if err != nil {
		return "", err
	}
	return treeID, nil
}

func (c *Snapshot) doSnapshot(ctx context.Context, basePoint string) (string, error) {
	snapshotIndex := filepath.Join(c.repoPath, "snapshot.index") // INDEX file
	snapshotEnv := env.SanitizerEnv("GIT_INDEX_VERSION", "GIT_INDEX_FILE")
	snapshotEnv = append(snapshotEnv,
		"GIT_INDEX_VERSION=4",
		"GIT_INDEX_FILE="+snapshotIndex,
	)
	if err := c.snapshotWriteIndex(ctx, snapshotEnv, basePoint); err != nil {
		die("git read-tree error: %v", err)
		return "", err
	}
	if err := c.addALL(ctx, snapshotEnv); err != nil {
		die("git add error: %v", err)
		return "", err
	}
	treeOID, err := c.writeTree(ctx, snapshotEnv)
	if err != nil {
		die("git write-tree: %v", err)
		return "", err
	}
	trace.DbgPrint("new tree: %s", treeOID)
	message, err := c.genMessage()
	if err != nil {
		return "", err
	}
	psArgs := []string{
		"commit-tree",
		"-F",
		"-",
	}
	parents := c.Parents
	if len(parents) == 0 && !c.Orphan {
		parents = append(parents, basePoint)
	}
	for _, parent := range parents {
		if parent == "" || git.IsHashZero(parent) {
			continue
		}

		psArgs = append(psArgs, "-p", parent)
	}
	psArgs = append(psArgs, treeOID)
	stdin := strings.NewReader(message)
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath:  c.repoPath,
		Stdin:     stdin,
		Stderr:    os.Stderr,
		Environ:   snapshotEnv,
		NoSetpgid: true,
	}, "git", psArgs...)
	commitID, err := cmd.OneLine()
	if err != nil {
		die("git commit-tree error: %v", err)
		return "", err
	}
	return commitID, nil
}

func (c *Snapshot) Run(g *Globals) error {
	var remote, refname string
	if c.Push {
		switch len(c.UnresolvedArgs) {
		case 0:
			die("hot snapshot --push require remote refname")
			return errors.New("missing args")
		case 1:
			remote = "origin"
			refname = c.UnresolvedArgs[0]
		default:
			remote = c.UnresolvedArgs[0]
			refname = c.UnresolvedArgs[1]
		}
	}
	var err error
	if c.worktree, err = git.RevParseWorktree(context.Background(), c.CWD); err != nil {
		die("can only be run on non-bare repositories, error: %v", err)
		return err
	}
	c.repoPath = git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v", c.repoPath)
	basePoint, current, err := git.RevParseCurrentEx(context.Background(), os.Environ(), c.repoPath)
	if err != nil {
		die("rev-parse HEAD: %v", err)
		return err
	}
	trace.DbgPrint("current '%s' commit: %s", current, basePoint)
	commit, err := c.doSnapshot(context.Background(), basePoint)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, W("new snapshot commit:"))
	_, _ = fmt.Fprintln(os.Stdout, commit)
	if !c.Push {
		return nil
	}
	trace.DbgPrint("remote %s reference: %s", remote, refname)
	psArgs := []string{"push"}
	if c.Force {
		psArgs = append(psArgs, "-f")
	}
	psArgs = append(psArgs, remote, fmt.Sprintf("%s:%s", commit, refname))
	cmd := command.NewFromOptions(context.Background(), &command.RunOpts{
		RepoPath:  c.repoPath,
		Environ:   os.Environ(),
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		NoSetpgid: true,
	}, "git", psArgs...)
	if err := cmd.RunEx(); err != nil {
		return err
	}
	return nil
}
