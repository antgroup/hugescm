// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/antgroup/hugescm/modules/diferenco"
)

// diffColors defines colors for diff output (GitHub-style).
type diffColors struct {
	addition lipgloss.Style
	deletion lipgloss.Style
	context  lipgloss.Style
}

// diffHighlight defines word-diff highlight styles.
type diffHighlight struct {
	deletion lipgloss.Style
	addition lipgloss.Style
}

// defaultDiffColors returns default diff colors with adaptive light/dark theme.
func defaultDiffColors() diffColors {
	return diffColors{
		addition: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipgloss.Color("#22863A"), Dark: lipgloss.Color("#85E89D"),
		}),
		deletion: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipgloss.Color("#CB2431"), Dark: lipgloss.Color("#F97583"),
		}),
		context: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipgloss.Color("#24292E"), Dark: lipgloss.Color("#E1E4E8"),
		}),
	}
}

// defaultDiffHighlight returns default word-diff highlight styles.
func defaultDiffHighlight() diffHighlight {
	return diffHighlight{
		deletion: lipgloss.NewStyle().
			Background(compat.AdaptiveColor{Light: lipgloss.Color("#ffc0ba"), Dark: lipgloss.Color("#5a1d1e")}).
			Foreground(compat.AdaptiveColor{Light: lipgloss.Color("#82071e"), Dark: lipgloss.Color("#ffa6a8")}),
		addition: lipgloss.NewStyle().
			Background(compat.AdaptiveColor{Light: lipgloss.Color("#a6f2b5"), Dark: lipgloss.Color("#2a6e3b")}).
			Foreground(compat.AdaptiveColor{Light: lipgloss.Color("#116329"), Dark: lipgloss.Color("#7ee691")}),
	}
}

// diffFormatter formats diff output with optional word-diff highlighting.
type diffFormatter struct {
	colors    diffColors
	highlight diffHighlight
	wordDiff  bool
}

// newDiffFormatter creates a new diffFormatter.
func newDiffFormatter(wordDiff bool) *diffFormatter {
	return &diffFormatter{
		colors:    defaultDiffColors(),
		highlight: defaultDiffHighlight(),
		wordDiff:  wordDiff,
	}
}

const (
	srcPrefix = "a/"
	dstPrefix = "b/"
	zeroOID   = "0000000000000000000000000000000000000000000000000000000000000000"
	// Guardrails for word-diff to avoid pathological latency on very long lines.
	maxWordDiffLineBytes = 8 * 1024
	maxWordDiffTotal     = 16 * 1024
)

// formatPatch formats a single patch.
func (f *diffFormatter) formatPatch(patch *diferenco.Patch) string {
	var sb strings.Builder
	if n := estimatePatchSize(patch); n > 0 {
		sb.Grow(n)
	}

	// Write file patch header
	f.writeFilePatchHeader(&sb, patch)

	if len(patch.Hunks) == 0 {
		return sb.String()
	}

	for _, hunk := range patch.Hunks {
		f.formatHunk(&sb, hunk)
	}
	return sb.String()
}

func estimatePatchSize(patch *diferenco.Patch) int {
	if patch == nil {
		return 0
	}
	// Header + hunk metadata baseline.
	size := 256 + len(patch.Hunks)*64
	for _, hunk := range patch.Hunks {
		for _, line := range hunk.Lines {
			// Prefix byte + payload + newline. Colored output adds ANSI overhead,
			// so reserve a little extra to reduce reallocations.
			size += len(line.Content) + 12
		}
	}
	if size < 0 {
		return 0
	}
	return size
}

// writeFilePatchHeader writes the diff header for a patch.
func (f *diffFormatter) writeFilePatchHeader(sb *strings.Builder, p *diferenco.Patch) {
	from, to := p.From, p.To
	if from == nil && to == nil {
		return
	}

	switch {
	case from != nil && to != nil:
		hashEquals := from.Hash == to.Hash
		fmt.Fprintf(sb, "diff --zeta %s%s %s%s\n", srcPrefix, from.Name, dstPrefix, to.Name)
		if from.Mode != to.Mode {
			fmt.Fprintf(sb, "old mode %o\n", from.Mode)
			fmt.Fprintf(sb, "new mode %o\n", to.Mode)
		}
		if from.Name != to.Name {
			fmt.Fprintf(sb, "rename from %s\n", from.Name)
			fmt.Fprintf(sb, "rename to %s\n", to.Name)
		}
		if from.Mode != to.Mode && !hashEquals {
			fmt.Fprintf(sb, "index %s..%s\n", from.Hash, to.Hash)
		} else if !hashEquals {
			fmt.Fprintf(sb, "index %s..%s %o\n", from.Hash, to.Hash, from.Mode)
		}
		if !hashEquals {
			if p.IsBinary {
				fmt.Fprintf(sb, "Binary files %s%s and %s%s differ\n", srcPrefix, from.Name, dstPrefix, to.Name)
			} else if p.IsFragments {
				fmt.Fprintf(sb, "Fragments files %s%s and %s%s differ\n", srcPrefix, from.Name, dstPrefix, to.Name)
			} else {
				fmt.Fprintf(sb, "--- %s%s\n", srcPrefix, from.Name)
				fmt.Fprintf(sb, "+++ %s%s\n", dstPrefix, to.Name)
			}
		}
	case from == nil:
		fmt.Fprintf(sb, "diff --zeta %s%s %s%s\n", srcPrefix, to.Name, dstPrefix, to.Name)
		fmt.Fprintf(sb, "new file mode %o\n", to.Mode)
		fmt.Fprintf(sb, "index %s..%s\n", zeroOID, to.Hash)
		if p.IsBinary {
			sb.WriteString("Binary files /dev/null and " + dstPrefix + to.Name + " differ\n")
		} else if p.IsFragments {
			sb.WriteString("Fragments files /dev/null and " + dstPrefix + to.Name + " differ\n")
		} else {
			sb.WriteString("--- /dev/null\n")
			fmt.Fprintf(sb, "+++ %s%s\n", dstPrefix, to.Name)
		}
	case to == nil:
		fmt.Fprintf(sb, "diff --zeta %s%s %s%s\n", srcPrefix, from.Name, dstPrefix, from.Name)
		fmt.Fprintf(sb, "deleted file mode %o\n", from.Mode)
		fmt.Fprintf(sb, "index %s..%s\n", from.Hash, zeroOID)
		if p.IsBinary {
			sb.WriteString("Binary files " + srcPrefix + from.Name + " and /dev/null differ\n")
		} else if p.IsFragments {
			sb.WriteString("Fragments files " + srcPrefix + from.Name + " and /dev/null differ\n")
		} else {
			fmt.Fprintf(sb, "--- %s%s\n", srcPrefix, from.Name)
			sb.WriteString("+++ /dev/null\n")
		}
	}
}

// formatHunk formats a hunk with optional word-diff.
func (f *diffFormatter) formatHunk(sb *strings.Builder, hunk *diferenco.Hunk) {
	// Hunk header
	fromCount, toCount := 0, 0
	for _, l := range hunk.Lines {
		switch l.Kind {
		case diferenco.Delete:
			fromCount++
		case diferenco.Insert:
			toCount++
		default:
			fromCount++
			toCount++
		}
	}

	sb.WriteString("@@")
	if fromCount > 1 {
		fmt.Fprintf(sb, " -%d,%d", hunk.FromLine, fromCount)
	} else if hunk.FromLine == 1 && fromCount == 0 {
		sb.WriteString(" -0,0")
	} else {
		fmt.Fprintf(sb, " -%d", hunk.FromLine)
	}
	if toCount > 1 {
		fmt.Fprintf(sb, " +%d,%d", hunk.ToLine, toCount)
	} else if hunk.ToLine == 1 && toCount == 0 {
		sb.WriteString(" +0,0")
	} else {
		fmt.Fprintf(sb, " +%d", hunk.ToLine)
	}
	sb.WriteString(" @@\n")

	lines := hunk.Lines
	for i := 0; i < len(lines); i++ {
		// Collect consecutive deletions
		if lines[i].Kind == diferenco.Delete {
			delStart := i
			for i < len(lines) && lines[i].Kind == diferenco.Delete {
				i++
			}
			delLines := lines[delStart:i]

			// Collect consecutive insertions
			insStart := i
			for i < len(lines) && lines[i].Kind == diferenco.Insert {
				i++
			}
			insLines := lines[insStart:i]

			// Format deletion/insertion pairs
			f.formatChangeBlock(sb, delLines, insLines)
			i-- // adjust for loop increment
			continue
		}
		f.formatLine(sb, lines[i])
	}
}

// formatChangeBlock formats a block of deletions and insertions with word-diff.
func (f *diffFormatter) formatChangeBlock(sb *strings.Builder, delLines, insLines []diferenco.Line) {
	// Only apply word-diff when there's exactly one deletion and one insertion
	// This matches GitHub's behavior and avoids false positives
	if f.wordDiff && len(delLines) == 1 && len(insLines) == 1 && shouldWordDiff(delLines[0], insLines[0]) {
		f.formatWordDiffPair(sb, delLines[0], insLines[0])
		return
	}

	// Otherwise, output lines normally
	for _, line := range delLines {
		f.formatLine(sb, line)
	}
	for _, line := range insLines {
		f.formatLine(sb, line)
	}
}

// formatWordDiffPair formats a pair of -/+ lines with word highlighting.
func (f *diffFormatter) formatWordDiffPair(sb *strings.Builder, oldLine, newLine diferenco.Line) {
	oldContent := strings.TrimSuffix(oldLine.Content, "\n")
	newContent := strings.TrimSuffix(newLine.Content, "\n")

	diffs, err := diferenco.DiffWords(context.Background(), oldContent, newContent, pickWordDiffAlgorithm(oldContent, newContent), nil)
	if err != nil {
		// Fallback to normal formatting
		f.formatLine(sb, oldLine)
		f.formatLine(sb, newLine)
		return
	}

	// Deletion line
	sb.WriteByte('-')
	for _, d := range diffs {
		switch d.Type {
		case diferenco.Delete:
			sb.WriteString(f.highlight.deletion.Render(d.Text))
		case diferenco.Equal:
			sb.WriteString(f.colors.deletion.Render(d.Text))
		}
	}
	sb.WriteByte('\n')

	// Insertion line
	sb.WriteByte('+')
	for _, d := range diffs {
		switch d.Type {
		case diferenco.Insert:
			sb.WriteString(f.highlight.addition.Render(d.Text))
		case diferenco.Equal:
			sb.WriteString(f.colors.addition.Render(d.Text))
		}
	}
	sb.WriteByte('\n')
}

func shouldWordDiff(oldLine, newLine diferenco.Line) bool {
	oldContent := strings.TrimSuffix(oldLine.Content, "\n")
	newContent := strings.TrimSuffix(newLine.Content, "\n")
	if len(oldContent) > maxWordDiffLineBytes || len(newContent) > maxWordDiffLineBytes {
		return false
	}
	if len(oldContent)+len(newContent) > maxWordDiffTotal {
		return false
	}
	return true
}

func pickWordDiffAlgorithm(oldContent, newContent string) diferenco.Algorithm {
	combined := len(oldContent) + len(newContent)
	if combined >= 4*1024 {
		return diferenco.Histogram
	}
	return diferenco.Myers
}

// formatLine formats a single diff line.
func (f *diffFormatter) formatLine(sb *strings.Builder, line diferenco.Line) {
	content := strings.TrimSuffix(line.Content, "\n")

	switch line.Kind {
	case diferenco.Delete:
		sb.WriteByte('-')
		sb.WriteString(f.colors.deletion.Render(content))
	case diferenco.Insert:
		sb.WriteByte('+')
		sb.WriteString(f.colors.addition.Render(content))
	default:
		sb.WriteByte(' ')
		sb.WriteString(f.colors.context.Render(content))
	}
	sb.WriteByte('\n')
}
