package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/jedib0t/go-pretty/v6/table"
)

var ErrorBlobNotFound = errors.New("blob not found")

type FileItem struct {
	Path          string
	Size          int64
	Modifications int
}

// Exports support sort
type FileItems []FileItem

// Len len exports
func (m FileItems) Len() int { return len(m) }

// Less less
func (m FileItems) Less(i, j int) bool { return m[i].Size > m[j].Size }

// Swap function
func (m FileItems) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

type pathSize struct {
	size          int64
	modifications int
}

type summer struct {
	files map[string]*pathSize
}

func newSummer() *summer {
	return &summer{files: make(map[string]*pathSize)}
}

func (s *summer) add(file string, size int64) {
	if p, ok := s.files[file]; ok {
		p.size += size
		p.modifications++
		return
	}
	s.files[file] = &pathSize{size: size, modifications: 1}
}

func (s *summer) draw(w io.Writer) {
	if len(s.files) == 0 {
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleColoredBlackOnCyanWhite)
	t.AppendHeader(table.Row{"#", "Path", "Modifications", "Size"})
	var largeSize int64
	items := make([]FileItem, 0, len(s.files))
	for n, p := range s.files {
		items = append(items, FileItem{Path: n, Size: p.size, Modifications: p.modifications})
		largeSize += p.size
	}
	sort.Sort(FileItems(items))
	for i, item := range items {
		t.AppendRow(table.Row{i + 1, item.Path, item.Modifications, strengthen.FormatSize(item.Size)})
	}
	t.AppendRow(table.Row{"TOTAL", "", "", strengthen.FormatSize(largeSize)})
	fmt.Fprintf(w, "Descending order by path size:\n")
	t.Render()
}

func (s *summer) resolveName(ctx context.Context, repoPath string, blobs map[string]int64) error {
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: os.Environ()},
		"git", "rev-list", "--objects", "--all", "--filter=object:type=blob")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer out.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait() // nolint
	br := bufio.NewScanner(out)
	for br.Scan() {
		oid, name, _ := strings.Cut(br.Text(), " ")
		if size, ok := blobs[oid]; ok {
			if len(name) == 0 {
				fmt.Fprintf(os.Stderr, "\x1b[38;2;254;225;64m%s\x1b[0m <\x1b[38;2;72;198;239mdangling\x1b[0m> size: \x1b[38;2;247;112;98m%s\x1b[0m\n", oid, strengthen.FormatSize(size))
				continue
			}
			fmt.Fprintf(os.Stderr, "\x1b[38;2;254;225;64m%s\x1b[0m [\x1b[38;2;72;198;239m%s\x1b[0m] size: \x1b[38;2;247;112;98m%s\x1b[0m\n", oid, name, strengthen.FormatSize(size))
			s.add(name, size)
		}
	}
	return nil
}

type Az struct {
	Paths []string `arg:"" name:"path" help:"Path to repositories" default:"." type:"path"`
	Limit int64    `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB,MB,GB,K,M,G" default:"10m" type:"size"`
}

func (a *Az) Run(g *Globals) error {
	for _, p := range a.Paths {
		if err := a.azOnce(p); err != nil {
			return err
		}
	}
	return nil
}

// git cat-file --batch-check --batch-all-objects
func (a *Az) azOnce(p string) error {
	repoPath := git.RevParseRepoPath(context.Background(), p)
	trace.DbgPrint("begin analysis repository: %v large file: %v", repoPath, strengthen.FormatSize(a.Limit))
	sum := newSummer()
	blobs := make(map[string]int64)
	filter, err := deflect.NewFilter(repoPath, git.HashFormatOK(repoPath), &deflect.FilterOption{
		Limit: a.Limit,
		Rejector: func(oid string, size int64) error {
			blobs[oid] = size
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "backset az: new filter: %v\n", err)
		return err
	}
	if err := filter.Execute(nil); err != nil {
		fmt.Fprintf(os.Stderr, "backset az: check large file: %v\n", err)
		return err
	}
	if err := sum.resolveName(context.Background(), repoPath, blobs); err != nil {
		fmt.Fprintf(os.Stderr, "backset az: resolve file name error: %v\n", err)
		return err
	}
	sum.draw(os.Stdout)
	fmt.Fprintf(os.Stderr, "Repo: \x1b[38;2;32;225;215m%s\x1b[0m size: \x1b[38;2;72;198;239m%s\x1b[0m\n", repoPath, strengthen.FormatSize(filter.Size()))
	return nil
}
