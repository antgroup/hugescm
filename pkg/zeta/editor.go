// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/shlex"
)

const (
	COMMIT_EDITMSG = "COMMIT_EDITMSG"
	TAG_EDITMSG    = "TAG_EDITMSG"
	MERGE_MSG      = "MERGE_MSG"
	defaultEditor  = "vi"
)

var (
	windowsEditor = []string{
		"vim",
		"nvim",
		"vi",
	}
)

// searchEditor: search editor
//
//	see: https://github.com/microsoft/terminal/discussions/16440
//	windows fallback: use git-for-windows vim ??
func searchEditor() string {
	if runtime.GOOS == "windows" {
		for _, e := range windowsEditor {
			if p, err := exec.LookPath(e); err == nil {
				return p
			}
		}
	}
	return "vi"
}

func fallbackEditor() string {
	if e, ok := os.LookupEnv("GIT_EDITOR"); ok {
		return e
	}
	if e, ok := os.LookupEnv("EDITOR"); ok {
		return e
	}
	return searchEditor()
}

// See: https://docs.github.com/en/get-started/getting-started-with-git/associating-text-editors-with-git
// vscode: zeta config --global core.editor "code --wait"
// sublime text: zeta config --global core.editor "subl -n -w"
// textmate: zeta config --global core.editor "mate -w"
func launchEditor(ctx context.Context, editor, path string, extraEnv []string) error {
	editorArgs := make([]string, 0, 10)
	if len(editor) == 0 {
		editor = fallbackEditor()
	}
	if cmdArgs, _ := shlex.Split(editor, true); len(cmdArgs) > 0 {
		editor = cmdArgs[0]
		editorArgs = append(editorArgs, cmdArgs[1:]...)
	}
	editorArgs = append(editorArgs, path)
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ:   os.Environ(),
		ExtraEnv:  extraEnv,
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		Stdin:     os.Stdin,
		NoSetpgid: true,
	}, editor, editorArgs...)
	return cmd.RunEx()
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
	defer fd.Close()
	return messageReadFrom(fd)
}
