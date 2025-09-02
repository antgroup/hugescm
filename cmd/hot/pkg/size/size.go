// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package size

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/strengthen"
)

type Executor struct {
	limit   int64
	paths   []string
	objects map[string]int64
}

func NewExecutor(size int64) *Executor {
	return &Executor{limit: size, objects: make(map[string]int64)}
}

// BLOB filter
func (e *Executor) Match(entry *gitobj.TreeEntry, absPath string) bool {
	if _, ok := e.objects[hex.EncodeToString(entry.Oid)]; ok {
		return true
	}
	return false
}

func (e *Executor) Paths() []string {
	return e.paths
}

// git cat-file --batch-check --batch-all-objects
func (e *Executor) Run(ctx context.Context, repoPath string, extract bool) error {
	if !git.IsGitVersionAtLeast("2.28.0") {
		return fmt.Errorf("require Git 2.28 or later")
	}
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: repoPath}, "cat-file", "--batch-check", "--batch-all-objects", "--unordered")
	if err != nil {
		return fmt.Errorf("start git cat-file error %w", err)
	}
	defer reader.Close()
	br := bufio.NewReader(reader)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("git cat-file readline error %w", err)
		}
		line = line[:len(line)-1]
		sv := strings.Split(line, " ")
		if len(sv) < 3 {
			continue
		}
		if sv[1] != "blob" {
			continue
		}
		sz, err := strconv.ParseInt(sv[2], 10, 64)
		if err != nil {
			continue
		}
		if sz >= e.limit {
			e.objects[sv[0]] = sz
		}
	}
	sum := newSummer()
	psArgs := []string{"rev-list", "--objects", "--all"}
	if err := sum.resolveName(ctx, repoPath, e.objects, psArgs, sum.printName); err != nil {
		fmt.Fprintf(os.Stderr, "blat size: resolve file name error: %v", err)
		return err
	}
	if len(sum.files) != 0 {
		fmt.Fprintf(os.Stdout, "%s - %s:\n", tr.W("Descending order by total size"), tr.W("All Branches and Tags"))
	}
	sum.draw(os.Stdout)
	if extract {
		e.currentCheck(ctx, repoPath, e.objects)
	}
	// COPY to files
	for p := range sum.files {
		e.paths = append(e.paths, p)
	}
	diskSize, err := strengthen.Du(filepath.Join(repoPath, "objects"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "blat size: check repo disk usage error: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s: \x1b[38;2;32;225;215m%s\x1b[0m %s: \x1b[38;2;72;198;239m%s\x1b[0m\n", tr.W("Repository"), repoPath, tr.W("size"), strengthen.FormatSize(diskSize))
	return nil
}

func (e *Executor) currentCheck(ctx context.Context, repoPath string, objects map[string]int64) {
	sum := newSummer()
	psArgs := []string{"rev-list", "--objects", "HEAD"}
	if err := sum.resolveName(ctx, repoPath, objects, psArgs, nil); err != nil {
		fmt.Fprintf(os.Stderr, "blat size: resolve file name error: %v", err)
		return
	}
	if len(sum.files) != 0 {
		fmt.Fprintf(os.Stdout, "\n%s - %s:\n", tr.W("Descending order by total size"), tr.W("Default Branch"))
	}
	sum.draw(os.Stdout)
}
