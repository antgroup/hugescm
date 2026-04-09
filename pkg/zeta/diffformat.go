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
	meta     lipgloss.Style
	frag     lipgloss.Style
}

// diffHighlight defines word-diff highlight styles.
type diffHighlight struct {
	deletion lipgloss.Style
	addition lipgloss.Style
}

// defaultDiffColors returns default diff colors with adaptive light/dark theme.
// Colors are designed to be consistent with GitHub's diff styling.
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
		// Meta: diff header lines (diff --zeta, index, new file mode, etc.)
		// Uses a dim/subtle gray to not distract from the actual diff content.
		meta: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipgloss.Color("#6E7781"), Dark: lipgloss.Color("#8B949E"),
		}),
		// Frag: fragment header lines (---, +++, @@ ... @@)
		// Uses GitHub's link blue color for consistency.
		frag: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipgloss.Color("#0550AE"), Dark: lipgloss.Color("#58A6FF"),
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

	// Three cases: modification, new file, or deleted file
	switch {
	case from != nil && to != nil:
		f.writeModifyHeader(sb, p, from, to)
	case from == nil:
		f.writeNewFileHeader(sb, p, to)
	case to == nil:
		f.writeDeleteHeader(sb, p, from)
	}
}

// writeModifyHeader writes diff header for file modification.
func (f *diffFormatter) writeModifyHeader(sb *strings.Builder, p *diferenco.Patch, from, to *diferenco.File) {
	hashEquals := from.Hash == to.Hash

	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("diff --zeta %s%s %s%s", srcPrefix, from.Name, dstPrefix, to.Name)))
	sb.WriteByte('\n')

	if from.Mode != to.Mode {
		sb.WriteString(f.colors.meta.Render(fmt.Sprintf("old mode %o", from.Mode)))
		sb.WriteByte('\n')
		sb.WriteString(f.colors.meta.Render(fmt.Sprintf("new mode %o", to.Mode)))
		sb.WriteByte('\n')
	}

	if from.Name != to.Name {
		sb.WriteString(f.colors.meta.Render("rename from " + from.Name))
		sb.WriteByte('\n')
		sb.WriteString(f.colors.meta.Render("rename to " + to.Name))
		sb.WriteByte('\n')
	}

	if !hashEquals {
		switch {
		case from.Mode != to.Mode:
			sb.WriteString(f.colors.meta.Render(fmt.Sprintf("index %s..%s", from.Hash, to.Hash)))
		default:
			sb.WriteString(f.colors.meta.Render(fmt.Sprintf("index %s..%s %o", from.Hash, to.Hash, from.Mode)))
		}
		sb.WriteByte('\n')
	}

	if !hashEquals {
		f.writeFileMarkers(sb, p, srcPrefix+from.Name, dstPrefix+to.Name)
	}
}

// writeNewFileHeader writes diff header for new file.
func (f *diffFormatter) writeNewFileHeader(sb *strings.Builder, p *diferenco.Patch, to *diferenco.File) {
	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("diff --zeta %s %s", srcPrefix+to.Name, dstPrefix+to.Name)))
	sb.WriteByte('\n')
	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("new file mode %o", to.Mode)))
	sb.WriteByte('\n')
	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("index %s..%s", zeroOID, to.Hash)))
	sb.WriteByte('\n')

	f.writeFileMarkers(sb, p, "/dev/null", dstPrefix+to.Name)
}

// writeDeleteHeader writes diff header for deleted file.
func (f *diffFormatter) writeDeleteHeader(sb *strings.Builder, p *diferenco.Patch, from *diferenco.File) {
	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("diff --zeta %s %s", srcPrefix+from.Name, dstPrefix+from.Name)))
	sb.WriteByte('\n')
	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("deleted file mode %o", from.Mode)))
	sb.WriteByte('\n')
	sb.WriteString(f.colors.meta.Render(fmt.Sprintf("index %s..%s", from.Hash, zeroOID)))
	sb.WriteByte('\n')

	f.writeFileMarkers(sb, p, srcPrefix+from.Name, "/dev/null")
}

// writeFileMarkers writes file markers based on patch type (binary/fragments/text).
// For text files: writes "--- fromPath" and "+++ toPath"
// For binary/fragments: writes "Binary/Fragments files ... differ"
func (f *diffFormatter) writeFileMarkers(sb *strings.Builder, p *diferenco.Patch, fromPath, toPath string) {
	switch {
	case p.IsBinary:
		sb.WriteString(f.colors.meta.Render(fmt.Sprintf("Binary files %s and %s differ", fromPath, toPath)))
		sb.WriteByte('\n')
	case p.IsFragments:
		sb.WriteString(f.colors.meta.Render(fmt.Sprintf("Fragments files %s and %s differ", fromPath, toPath)))
		sb.WriteByte('\n')
	default:
		sb.WriteString(f.colors.frag.Render("--- " + fromPath))
		sb.WriteByte('\n')
		sb.WriteString(f.colors.frag.Render("+++ " + toPath))
		sb.WriteByte('\n')
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

	var header strings.Builder
	header.WriteString("@@")

	// Format from line range
	switch {
	case fromCount > 1:
		fmt.Fprintf(&header, " -%d,%d", hunk.FromLine, fromCount)
	case hunk.FromLine == 1 && fromCount == 0:
		header.WriteString(" -0,0")
	default:
		fmt.Fprintf(&header, " -%d", hunk.FromLine)
	}

	// Format to line range
	switch {
	case toCount > 1:
		fmt.Fprintf(&header, " +%d,%d", hunk.ToLine, toCount)
	case hunk.ToLine == 1 && toCount == 0:
		header.WriteString(" +0,0")
	default:
		fmt.Fprintf(&header, " +%d", hunk.ToLine)
	}

	header.WriteString(" @@")

	sb.WriteString(f.colors.frag.Render(header.String()))
	sb.WriteByte('\n')

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
