package stat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/rivo/uniseg"
)

type Item struct {
	Path  string
	Total int64
	Count int
}

// Exports support sort
type Items []Item

// Len len exports
func (m Items) Len() int { return len(m) }

// Less less
func (m Items) Less(i, j int) bool { return m[i].Total > m[j].Total }

// Swap function
func (m Items) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

type sizeCounter struct {
	sum   int64
	count int
}

type summer struct {
	files    map[string]*sizeCounter
	total    int64
	count    int
	fullPath bool
}

func truncatedName(s string, maxWidth int) string {
	w := uniseg.StringWidth(s)
	if w < maxWidth {
		return s
	}
	vv := strengthen.SplitPath(s)
	if len(vv) <= 1 {
		return s
	}
	for i := 1; i < len(vv); i++ {
		ss := ".../" + path.Join(vv[i:]...)
		if uniseg.StringWidth(ss) <= maxWidth {
			return ss
		}
	}
	return vv[len(vv)-1]
}

func newSummer(fullPath bool) *summer {
	return &summer{files: make(map[string]*sizeCounter), fullPath: fullPath}
}

func (s *summer) add(file string, size int64) {
	s.total += size
	s.count++
	if sz, ok := s.files[file]; ok {
		sz.sum += size
		sz.count++
		return
	}
	s.files[file] = &sizeCounter{sum: size, count: 1}
}

func (s *summer) draw(w io.Writer) {
	if len(s.files) == 0 {
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleColoredBlackOnCyanWhite)
	t.AppendHeader(table.Row{"#", tr.W("Path"), tr.W("Modifications"), tr.W("Cumulative Size")})
	items := make([]Item, 0, len(s.files))
	for n, i := range s.files {
		items = append(items, Item{Path: n, Total: i.sum, Count: i.count})
	}
	sort.Sort(Items(items))
	for i, item := range items {
		t.AppendRow(table.Row{i + 1, truncatedName(item.Path, 100), strconv.Itoa(item.Count), strengthen.FormatSize(item.Total)})
	}
	t.AppendRow(table.Row{strings.ToUpper(tr.W("total")), "", strconv.Itoa(s.count), strengthen.FormatSize(s.total)})
	t.Render()
}

type Printer func(string, string, int64)

func (s *summer) printName(name, oid string, size int64) {
	if len(name) == 0 {
		fmt.Fprintf(os.Stderr, "%s <%s> %s: %s\n", yellow(oid), blue("dangle"), tr.W("size"), red(strengthen.FormatSize(size)))
		return
	}
	if !s.fullPath {
		name = truncatedName(name, 100)
	}
	fmt.Fprintf(os.Stderr, "%s [%s] %s: %s\n", yellow(oid), blue(name), tr.W("size"), red(strengthen.FormatSize(size)))
}

func (s *summer) resolveName(ctx context.Context, repoPath string, seen map[string]int64, psArgs []string, fn Printer) error {
	if git.IsGitVersionAtLeast(git.NewVersion(2, 35, 0)) {
		psArgs = append(psArgs, "--filter=object:type=blob")
	}
	cmd := command.NewFromOptions(ctx,
		&command.RunOpts{
			RepoPath: repoPath,
			Environ:  os.Environ(),
		}, "git", psArgs...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer out.Close() // nolint
	if err := cmd.Start(); err != nil {
		return err
	}
	br := bufio.NewScanner(out)
	for br.Scan() {
		oid, name, _ := strings.Cut(br.Text(), " ")
		if size, ok := seen[oid]; ok {
			if fn != nil {
				fn(name, oid, size)
			}
			s.add(name, size)
		}
	}
	return nil
}
