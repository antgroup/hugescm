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

type Show struct {
	CWD    string `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Commit string `arg:"" name:"commit" help:"Commit to show" optional:"" default:"HEAD"`
	JSON   bool   `name:"json" short:"j" help:"Output patches in JSON format"`
}

func (c *Show) Run(ctx context.Context, g *Globals) error {
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

	// Build git show arguments
	args := []string{
		"show",
		"--patch",
		"--raw",
		fmt.Sprintf("--abbrev=%d", hashFormat.HexSize()),
		"--full-index",
		"--find-renames=50%",
		"--format=",
		c.Commit,
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
		die("start git show: %v", err)
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
		die("git show: %v", command.FromError(err))
		return err
	}

	if perr := parser.Err(); perr != nil {
		die("parse diff: %v", perr)
		return perr
	}

	trace.DbgPrint("parsed %d patches", len(patches))

	// JSON output: always emit the raw patch array, even when empty, so
	// callers piping `hot show --json` get stable, parseable output.
	if c.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(patches)
	}

	// Terminal not supported: fallback to plain text output. We still
	// surface a "No changes" hint when there is nothing to encode (e.g.
	// merge commits) so the pipe output is not silently empty.
	if !term.IsTerminal(os.Stdout.Fd()) {
		if len(patches) == 0 {
			fmt.Println("No changes")
			return nil
		}
		encoder := diferenco.NewUnifiedEncoder(os.Stdout, diferenco.WithVCS("git"))
		return encoder.Encode(patches)
	}

	// Build a top header with commit metadata. We run a second `git log
	// -1` for this rather than parsing the (suppressed) format output,
	// because the main parse path is strictly diff-only. Failure to
	// resolve metadata is non-fatal — we just fall back to no header.
	//
	// The header is configured unconditionally so that merge commits /
	// empty commits (which produce zero patches) still show their
	// Commit / Author / Date / Subject details instead of degenerating
	// to a bare "No changes" line. patchview.Run handles the empty-
	// patches case by printing the header + "No changes" to stdout.
	runOpts := []patchview.Option{}
	if hdr := loadShowHeaderOption(ctx, c.Commit, patches); hdr != nil {
		runOpts = append(runOpts, hdr)
	}
	return patchview.Run(patches, runOpts...)
}

// loadShowHeaderOption returns a patchview.Option populated with the
// commit metadata for `commit`, or nil if the metadata cannot be
// resolved. Errors are intentionally swallowed: a missing header is
// strictly better than failing the whole `hot show`.
func loadShowHeaderOption(ctx context.Context, commit string, patches []*diferenco.Patch) patchview.Option {
	// Use %H (full hash) to match `git show` / `git log` defaults; the
	// patchview will left-truncate if the terminal is too narrow.
	format := "--format=%H%x09%an <%ae>%x09%aD%x09%s"
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ: os.Environ(),
	}, "git", "log", "-1", format, commit)

	out, err := cmd.Output()
	if err != nil {
		trace.DbgPrint("load show header: %v", err)
		return nil
	}

	line := strings.TrimRight(string(out), "\n")
	parts := strings.SplitN(line, "\t", 4)
	if len(parts) < 4 {
		return nil
	}
	hash, author, date, subject := parts[0], parts[1], parts[2], parts[3]

	// Add the patch summary as its own "Files:" row in the header so the
	// user gets the same +X -Y info `git show --stat` would surface, with
	// the same Addition/Deletion colors used in the file list.
	files := patchview.ColorizedPatchSummary(patchview.DefaultStyle(), patches)
	return patchview.WithCommitHeaderWithFiles(hash, author, date, subject, files)
}
