// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

func (c *Diff) Run(ctx context.Context, g *Globals) error {
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

	// Top header describes the diff scope plus a one-line stats summary,
	// mirroring how the user invoked the command:
	//   Diff:  --cached HEAD
	//   Files: 3 files changed, +5 -2
	title := c.headerTitle()
	summary := patchview.ColorizedPatchSummary(patchview.DefaultStyle(), patches)

	var entries []patchview.HeaderEntry
	if title != "" {
		entries = append(entries, patchview.HeaderEntry{Key: "Diff", Value: title})
	}
	if summary != "" {
		entries = append(entries, patchview.HeaderEntry{Key: "Files", Value: summary})
	}

	var runOpts []patchview.Option
	if len(entries) > 0 {
		runOpts = append(runOpts, patchview.WithHeaderEntries(entries...))
	}
	return patchview.Run(patches, runOpts...)
}

// headerTitle returns the patchview header value describing the diff
// scope as the user invoked it. The "Diff:" key is added by the caller
// when assembling HeaderEntries, so this returns only the value:
//
//	"--cached"                 -- staged changes
//	"--cached <args...>"       -- staged + extra args
//	"<args...>"                -- when the user passed an explicit range/paths
//	"worktree"                 -- worktree vs. index, no args
//
// We deliberately echo back the user-provided args verbatim so the
// header matches what they typed.
func (c *Diff) headerTitle() string {
	switch {
	case c.Cached || c.Staged:
		if len(c.Args) > 0 {
			return "--cached " + strings.Join(c.Args, " ")
		}
		return "--cached"
	case len(c.Args) > 0:
		return strings.Join(c.Args, " ")
	default:
		return "worktree"
	}
}
