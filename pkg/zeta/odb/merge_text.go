// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

// .merge_file_XXXXXX
func (d *ODB) writeMergeFileToTemp(s string) (string, error) {
	tempDir := filepath.Join(d.root, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", err
	}
	fd, err := os.CreateTemp(tempDir, ".merge_file_")
	if err != nil {
		return "", err
	}
	name := fd.Name()
	if _, err := fd.WriteString(s); err != nil {
		_ = fd.Close()
		_ = os.Remove(name)
		return "", err
	}
	_ = fd.Close()
	return name, nil
}

// ExternalMerge: use external merge tool --> aka git merge-file.
//
//	eg: git merge-file -L VERSION -L VERSION -L VERSION --no-diff3 -p VERSION.a VERSION.o VERSION.b
func (d *ODB) ExternalMerge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error) {
	var pathO, pathA, pathB string
	var err error
	defer func() {
		if len(pathO) != 0 {
			_ = os.Remove(pathO)
		}
		if len(pathA) != 0 {
			_ = os.Remove(pathA)
		}
		if len(pathB) != 0 {
			_ = os.Remove(pathB)
		}
	}()
	if pathO, err = d.writeMergeFileToTemp(o); err != nil {
		return "", false, err
	}
	if pathA, err = d.writeMergeFileToTemp(a); err != nil {
		return "", false, err
	}
	if pathB, err = d.writeMergeFileToTemp(b); err != nil {
		return "", false, err
	}
	var stdout strings.Builder
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Stderr: stderr,
		Stdout: &stdout,
	}, "git", "merge-file", "-L", labelA, "-L", labelO, "-L", labelB, "-p", pathA, pathO, pathB)
	if err = cmd.Run(); err != nil {
		if command.FromErrorCode(err) == 1 {
			return stdout.String(), true, nil
		}
		return "", true, fmt.Errorf("git merge-file error: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), false, nil
}

func (d *ODB) Diff3Merge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error) {
	var pathO, pathA, pathB string
	var err error
	defer func() {
		if len(pathO) != 0 {
			_ = os.Remove(pathO)
		}
		if len(pathA) != 0 {
			_ = os.Remove(pathA)
		}
		if len(pathB) != 0 {
			_ = os.Remove(pathB)
		}
	}()
	if pathO, err = d.writeMergeFileToTemp(o); err != nil {
		return "", false, err
	}
	if pathA, err = d.writeMergeFileToTemp(a); err != nil {
		return "", false, err
	}
	if pathB, err = d.writeMergeFileToTemp(b); err != nil {
		return "", false, err
	}
	var stdout strings.Builder
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Stderr: stderr,
		Stdout: &stdout,
	}, "diff3", "-m", "-a", "-L", labelA, "-L", labelO, "-L", labelB, "-p", pathA, pathO, pathB)
	if err = cmd.Run(); err != nil {
		if command.FromErrorCode(err) == 1 {
			return stdout.String(), true, nil
		}
		return "", true, fmt.Errorf("diff3 error: %w\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), false, nil
}
