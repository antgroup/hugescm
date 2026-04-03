package stat

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
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

type Printer func(string, string, int64)

func (s *summer) printName(name, oid string, size int64) {
	if len(name) == 0 {
		fmt.Fprintf(os.Stderr, "%s <%s> %s: %s\n", yellow(oid), blue("dangle"), tr.W("size"), red(strengthen.FormatSize(size)))
		return
	}
	displayName := name
	if !s.fullPath {
		displayName = truncatePath(name, 100)
	}
	fmt.Fprintf(os.Stderr, "%s [%s] %s: %s\n", yellow(oid), blue(displayName), tr.W("size"), red(strengthen.FormatSize(size)))
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
