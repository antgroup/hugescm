// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/survey"
)

func (r *Replayer) cleanup(prune bool) error {
	if !prune {
		prompt := &survey.Confirm{
			Message: tr.W("Do you want to prune the repository right away"),
		}
		_ = survey.AskOne(prompt, &prune)
		if !prune {
			return nil
		}
	}
	cmd := command.NewFromOptions(r.ctx, &command.RunOpts{
		Environ:   os.Environ(),
		RepoPath:  r.repoPath,
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		Stdin:     os.Stdin,
		NoSetpgid: true,
	}, "git", "-c", "repack.writeBitmaps=true", "-c", "pack.packSizeLimit=16g", "gc", "--prune=now", "--aggressive")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run git gc error: %w", err)
	}
	diskSize, err := strengthen.Du(filepath.Join(r.repoPath, "objects"))
	if err != nil {
		return fmt.Errorf("du repo size error: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s: \x1b[38;2;32;225;215m%s\x1b[0m %s: \x1b[38;2;72;198;239m%s\x1b[0m\n",
		r.stepCurrent, r.stepEnd, tr.W("Repository"), r.repoPath, tr.W("size"), strengthen.FormatSize(diskSize))
	return nil
}
