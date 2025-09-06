package stat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
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
	_, _ = fmt.Fprintf(w, "Descending order by path size:\n")
	t.Render()
}

func (s *summer) resolveName(ctx context.Context, repoPath string, objects map[string]int64) error {
	cmd := command.NewFromOptions(ctx,
		&command.RunOpts{
			RepoPath: repoPath,
			Environ:  os.Environ(),
		}, "git", "rev-list", "--objects", "--all", "--filter=object:type=blob")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer out.Close() // nolint
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait() // nolint
	br := bufio.NewScanner(out)
	for br.Scan() {
		oid, name, _ := strings.Cut(br.Text(), " ")
		if size, ok := objects[oid]; ok {
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

func showHugeObjects(ctx context.Context, repoPath string, objects map[string]int64) error {
	sum := newSummer()
	if err := sum.resolveName(ctx, repoPath, objects); err != nil {
		fmt.Fprintf(os.Stderr, "hot az: resolve file name error: %v\n", err)
		return err
	}
	sum.draw(os.Stdout)
	return nil
}

func Az(ctx context.Context, repoPath string, limit int64) error {
	objects := make(map[string]int64)
	filter, err := deflect.NewFilter(repoPath, git.HashFormatOK(repoPath), &deflect.FilterOption{
		Limit: limit,
		Rejector: func(oid string, size int64) error {
			objects[oid] = size
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "hot az: new filter: %v\n", err)
		return err
	}
	if err := filter.Execute(nil); err != nil {
		fmt.Fprintf(os.Stderr, "hot az: check large file: %v\n", err)
		return err
	}
	_ = showHugeObjects(ctx, repoPath, objects)
	fmt.Fprintf(os.Stderr, "%s\x1b[38;2;72;198;239m%s\x1b[0m\n", tr.W("Size: "), strengthen.FormatSize(filter.Size()))
	return nil
}
