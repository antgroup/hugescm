// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/diff"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/patchview"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/trace"
)

type Diff struct {
	CWD    string   `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Cached bool     `name:"cached" help:"Show staged changes"`
	Staged bool     `name:"staged" help:"Same as --cached"`
	JSON   bool     `name:"json" short:"j" help:"Output patches in JSON format"`
	Args   []string `arg:"" optional:"" name:"args" help:"Commit range or paths"`
}

func (c *Diff) Run(g *Globals) error {
	ctx := context.Background()
	repoPath := git.RevParseRepoPath(ctx, c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)

	// Get hash format from repository
	formatName, err := git.RevParseHashFormat(ctx, repoPath)
	if err != nil {
		die("detect hash format: %v", err)
		return err
	}
	hashFormat := git.HashFormatFromName(formatName)
	trace.DbgPrint("hash format: %s, abbrev: %d", formatName, hashFormat.HexSize())

	// Build git diff arguments
	args := []string{
		"diff",
		"--patch",
		"--raw",
		fmt.Sprintf("--abbrev=%d", hashFormat.HexSize()),
		"--full-index",
		"--find-renames=50%",
	}

	if c.Cached || c.Staged {
		args = append(args, "--cached")
	}

	// Append user-provided arguments (commit range, paths, etc.)
	if len(c.Args) > 0 {
		args = append(args, c.Args...)
	}

	// Create and start command
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ: os.Environ(),
	}, "git", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		die("create stdout pipe: %v", err)
		return err
	}
	defer stdout.Close() // nolint: errcheck

	if err := cmd.Start(); err != nil {
		die("start git diff: %v", err)
		return err
	}

	// Parse diff output
	parser := diff.NewParser(hashFormat, stdout, diff.Limits{})
	var patches []*diferenco.Patch

	for parser.Parse() {
		p := parser.Patch()
		if p.Patch != nil {
			patches = append(patches, p.Patch)
		}
	}

	if err := cmd.Wait(); err != nil {
		die("git diff: %v", command.FromError(err))
		return err
	}

	if perr := parser.Err(); perr != nil {
		die("parse diff: %v", perr)
		return perr
	}

	trace.DbgPrint("parsed %d patches", len(patches))

	// Display using patchview
	if len(patches) == 0 {
		fmt.Println("No changes")
		return nil
	}

	// JSON output
	if c.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(patches)
	}

	// Terminal not supported: fallback to plain text output
	if !term.IsTerminal(os.Stdout.Fd()) {
		encoder := diferenco.NewUnifiedEncoder(os.Stdout, diferenco.WithVCS("git"))
		return encoder.Encode(patches)
	}

	return patchview.Run(patches)
}
