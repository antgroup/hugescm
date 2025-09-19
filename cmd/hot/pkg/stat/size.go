// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package stat

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

	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/strengthen"
)

type SizeExecutor struct {
	limit    int64
	paths    []string
	objects  map[string]int64
	fullPath bool
}

func NewSizeExecutor(size int64, fullPath bool) *SizeExecutor {
	return &SizeExecutor{limit: size, objects: make(map[string]int64), fullPath: fullPath}
}

// BLOB filter
func (e *SizeExecutor) Match(entry *gitobj.TreeEntry, absPath string) bool {
	if _, ok := e.objects[hex.EncodeToString(entry.Oid)]; ok {
		return true
	}
	return false
}

func (e *SizeExecutor) Paths() []string {
	return e.paths
}

// git cat-file --batch-check --batch-all-objects
func (e *SizeExecutor) Run(ctx context.Context, repoPath string, extract bool) error {
	if !git.IsGitVersionAtLeast(git.NewVersion(2, 35, 0)) {
		return fmt.Errorf("require Git 2.28 or later")
	}
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: repoPath}, "cat-file", "--batch-check", "--batch-all-objects", "--unordered")
	if err != nil {
		return fmt.Errorf("start git cat-file error %w", err)
	}
	defer reader.Close() // nolint
	br := bufio.NewReader(reader)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF { // always endswith '\n'
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
	su := newSummer(e.fullPath)
	psArgs := []string{"rev-list", "--objects", "--all"}
	if err := su.resolveName(ctx, repoPath, e.objects, psArgs, su.printName); err != nil {
		fmt.Fprintf(os.Stderr, "hot size: resolve file name error: %v", err)
		return err
	}
	if len(su.files) != 0 {
		_, _ = fmt.Fprintf(os.Stdout, "%s - %s:\n", tr.W("Descending order by total size"), tr.W("All Branches and Tags"))
	}
	su.draw(os.Stdout)
	if extract {
		e.currentCheck(ctx, repoPath, e.objects)
	}
	// COPY to files
	for p := range su.files {
		e.paths = append(e.paths, p)
	}
	diskSize, err := strengthen.Du(filepath.Join(repoPath, "objects"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "hot size: check repo disk usage error: %v", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s: %s %s: %s\n", tr.W("Repository"), green2(repoPath), tr.W("size"), blue(strengthen.FormatSize(diskSize)))
	return nil
}

func (e *SizeExecutor) currentCheck(ctx context.Context, repoPath string, objects map[string]int64) {
	su := newSummer(e.fullPath)
	psArgs := []string{"rev-list", "--objects", "HEAD"}
	if err := su.resolveName(ctx, repoPath, objects, psArgs, nil); err != nil {
		fmt.Fprintf(os.Stderr, "hot size: resolve file name error: %v", err)
		return
	}
	if len(su.files) != 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\n%s - %s:\n", tr.W("Descending order by total size"), tr.W("Default Branch"))
	}
	su.draw(os.Stdout)
}
