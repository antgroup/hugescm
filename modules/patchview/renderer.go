package patchview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/antgroup/hugescm/modules/diferenco"
)

const (
	minCodeWidth = 10
)

// PatchRenderer renders a diferenco.Patch for display.
// It handles line numbers, syntax highlighting, and horizontal scrolling.
type PatchRenderer struct {
	patch   *diferenco.Patch
	style   PatchViewStyle
	width   int
	height  int
	xOffset int
	yOffset int

	// Precomputed values
	totalLines      int
	hunkLineOffsets []int
	beforeNumDigits int
	afterNumDigits  int

	// Options
	lineNumbers bool

	// Syntax highlighting
	highlighter     *SyntaxHighlighter
	syntaxHighlight bool
	isDark          bool
}

// NewPatchRenderer creates a new PatchRenderer with the default style for
// the given theme. No terminal IO is performed; callers (typically
// PatchView) decide the theme.
func NewPatchRenderer(isDark bool) *PatchRenderer {
	return &PatchRenderer{
		style:           DefaultStyleFor(isDark),
		lineNumbers:     true,
		syntaxHighlight: true,
		isDark:          isDark,
	}
}

// SetPatch sets the patch to render.
func (r *PatchRenderer) SetPatch(p *diferenco.Patch) {
	r.patch = p
	r.xOffset = 0
	r.yOffset = 0
	r.computeMetadata()
	r.initHighlighter()
}

// SetSize sets the rendering area size.
func (r *PatchRenderer) SetSize(width, height int) {
	r.width = width
	r.height = height
}

// SetStyle sets the style for rendering.
func (r *PatchRenderer) SetStyle(style PatchViewStyle) {
	r.style = style
}

// SetLineNumbers sets whether to show line numbers.
func (r *PatchRenderer) SetLineNumbers(enabled bool) {
	r.lineNumbers = enabled
}

// SetSyntaxHighlight sets whether to enable syntax highlighting.
func (r *PatchRenderer) SetSyntaxHighlight(enabled bool) {
	r.syntaxHighlight = enabled
	if !enabled {
		r.highlighter = nil
	}
}

// SetDarkBackground sets the terminal background mode.
func (r *PatchRenderer) SetDarkBackground(dark bool) {
	r.isDark = dark
	r.initHighlighter()
}

// initHighlighter initializes the syntax highlighter.
func (r *PatchRenderer) initHighlighter() {
	if r.patch == nil || !r.syntaxHighlight {
		r.highlighter = nil
		return
	}

	filename := r.patch.Name()
	if filename == "" {
		r.highlighter = nil
		return
	}

	r.highlighter = NewSyntaxHighlighter(filename, r.isDark)
}

// SetYOffset sets the vertical scroll offset.
func (r *PatchRenderer) SetYOffset(offset int) {
	r.yOffset = max(0, min(offset, r.maxYOffset()))
}

// SetXOffset sets the horizontal scroll offset.
// Note: Unlike SetYOffset, there's no upper bound because line widths vary
// and may contain ANSI escape sequences. The render function handles
// out-of-bounds offsets gracefully by showing empty content.
func (r *PatchRenderer) SetXOffset(offset int) {
	r.xOffset = max(0, offset)
}

// YOffset returns the current vertical offset.
func (r *PatchRenderer) YOffset() int {
	return r.yOffset
}

// XOffset returns the current horizontal offset.
func (r *PatchRenderer) XOffset() int {
	return r.xOffset
}

// TotalLines returns the total number of lines in the patch.
func (r *PatchRenderer) TotalLines() int {
	return r.totalLines
}

// HunkOffsets returns the starting line offset for each hunk.
// This is used for [ and ] navigation between hunks.
func (r *PatchRenderer) HunkOffsets() []int {
	return r.hunkLineOffsets
}

// Render renders the patch content for the current viewport.
func (r *PatchRenderer) Render() string {
	if r.patch == nil || r.width <= 0 || r.height <= 0 {
		return ""
	}

	if r.patch.IsBinary {
		return r.style.DiffStyle.FileName.Render("Binary file differs")
	}

	if len(r.patch.Hunks) == 0 {
		return r.style.DiffStyle.FileMeta.Render("No changes")
	}

	layout := r.layoutDecision()
	showLineNums := layout.showLineNums
	codeW := layout.codeWidth

	var sb strings.Builder
	sb.Grow(r.width * r.height)

	lineIdx := 0
	printed := 0

	for _, hunk := range r.patch.Hunks {
		// Hunk header line
		if lineIdx >= r.yOffset && printed < r.height {
			line := r.renderHunkHeader(hunk, showLineNums, codeW)
			if printed > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(line)
			printed++
		}
		lineIdx++

		if printed >= r.height {
			break
		}

		// Hunk content lines
		beforeLine := hunk.FromLine
		afterLine := hunk.ToLine

		for _, l := range hunk.Lines {
			if lineIdx >= r.yOffset && printed < r.height {
				line := r.renderLine(l, beforeLine, afterLine, showLineNums, codeW)
				if printed > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(line)
				printed++
			}

			switch l.Kind {
			case diferenco.Delete:
				beforeLine++
			case diferenco.Insert:
				afterLine++
			default:
				beforeLine++
				afterLine++
			}

			lineIdx++
			if printed >= r.height {
				break
			}
		}

		if printed >= r.height {
			break
		}
	}

	// Fill remaining lines
	for printed < r.height {
		if printed > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(r.renderEmptyLine(showLineNums, codeW))
		printed++
	}

	return sb.String()
}

// computeMetadata precomputes line counts and hunk offsets.
func (r *PatchRenderer) computeMetadata() {
	if r.patch == nil || len(r.patch.Hunks) == 0 {
		r.totalLines = 0
		r.hunkLineOffsets = nil
		r.beforeNumDigits = 1
		r.afterNumDigits = 1
		return
	}

	maxBefore, maxAfter := 0, 0
	r.totalLines = 0
	r.hunkLineOffsets = make([]int, 0, len(r.patch.Hunks))

	for _, h := range r.patch.Hunks {
		r.hunkLineOffsets = append(r.hunkLineOffsets, r.totalLines)

		beforeLine := h.FromLine
		afterLine := h.ToLine
		for _, l := range h.Lines {
			switch l.Kind {
			case diferenco.Delete:
				beforeLine++
			case diferenco.Insert:
				afterLine++
			default:
				beforeLine++
				afterLine++
			}
		}
		maxBefore = max(maxBefore, beforeLine)
		maxAfter = max(maxAfter, afterLine)
		r.totalLines += 1 + len(h.Lines) // 1 for hunk header
	}

	r.beforeNumDigits = digitCount(maxBefore)
	r.afterNumDigits = digitCount(maxAfter)
}

// maxYOffset returns the maximum vertical scroll offset.
func (r *PatchRenderer) maxYOffset() int {
	return max(0, r.totalLines-r.height)
}

// layout captures the resolved layout decision for a single render pass:
// whether line numbers are shown and how many columns are available for
// code. Centralising this avoids the two-place threshold drift between
// shouldShowLineNumbers and codeWidth.
type layout struct {
	showLineNums bool
	lineNumWidth int
	codeWidth    int
}

// layoutDecision computes a single, internally consistent layout decision
// for the current renderer state. Centralising this avoids the two-place
// threshold drift that existed when shouldShowLineNumbers and codeWidth
// were separate methods.
func (r *PatchRenderer) layoutDecision() layout {
	if r.width <= 0 {
		return layout{}
	}
	if !r.lineNumbers {
		return layout{
			showLineNums: false,
			lineNumWidth: 0,
			codeWidth:    r.width,
		}
	}
	lnW := (r.beforeNumDigits + lineNumPadding*2) + (r.afterNumDigits + lineNumPadding*2)
	if r.width-lnW < minCodeWidth {
		// Not enough room for line numbers; hide them and give all of
		// the width to code.
		return layout{
			showLineNums: false,
			lineNumWidth: 0,
			codeWidth:    r.width,
		}
	}
	return layout{
		showLineNums: true,
		lineNumWidth: lnW,
		codeWidth:    r.width - lnW,
	}
}

// renderHunkHeader renders a hunk header line (@@ -1,3 +1,4 @@).
func (r *PatchRenderer) renderHunkHeader(hunk *diferenco.Hunk, showLineNums bool, codeW int) string {
	style := &r.style.DiffStyle.DividerLine

	// Build hunk header with section
	fromCount := hunkFromCount(hunk)
	toCount := hunkToCount(hunk)
	header := formatHunkHeader(hunk.FromLine, fromCount, hunk.ToLine, toCount, hunk.Section)

	// Drop the leading "@@" because the divider line style already
	// provides the visual separator.
	headerContent := strings.TrimPrefix(header, "@@")

	var sb strings.Builder

	if showLineNums {
		sb.WriteString(style.LineNumber.Render(pad("…", r.beforeNumDigits)))
		sb.WriteString(style.LineNumber.Render(pad("…", r.afterNumDigits)))
	}

	sb.WriteString(style.Code.Width(codeW).Render(headerContent))
	return sb.String()
}

// renderLine renders a single diff line.
func (r *PatchRenderer) renderLine(l diferenco.Line, beforeLine, afterLine int, showLineNums bool, codeW int) string {
	var style *LineStyle
	var sym string
	var beforeNum, afterNum string

	switch l.Kind {
	case diferenco.Insert:
		style = &r.style.DiffStyle.InsertLine
		sym = "+"
		beforeNum = pad(" ", r.beforeNumDigits)
		afterNum = pad(afterLine, r.afterNumDigits)
	case diferenco.Delete:
		style = &r.style.DiffStyle.DeleteLine
		sym = "-"
		beforeNum = pad(beforeLine, r.beforeNumDigits)
		afterNum = pad(" ", r.afterNumDigits)
	default:
		style = &r.style.DiffStyle.EqualLine
		sym = " "
		beforeNum = pad(beforeLine, r.beforeNumDigits)
		afterNum = pad(afterLine, r.afterNumDigits)
	}

	var sb strings.Builder

	// Line numbers with background
	if showLineNums {
		sb.WriteString(style.LineNumber.Render(beforeNum))
		sb.WriteString(style.LineNumber.Render(afterNum))
	}

	// Get original content and remove trailing newlines (\r\n or \n)
	content := strings.TrimRight(l.Content, "\r\n")

	// Apply syntax highlighting (on full code before adding symbol)
	if r.highlighter != nil && r.syntaxHighlight && content != "" {
		bgColor := extractBgColor(style.Code)
		content = r.highlighter.Highlight(content, bgColor)
	}

	// Build full content (symbol + content)
	fullContent := sym + " " + content

	// Apply horizontal scroll
	if r.xOffset > 0 && len(fullContent) > 0 {
		contentWidth := lipgloss.Width(fullContent)
		if contentWidth > r.xOffset {
			fullContent = ansi.TruncateLeftWc(fullContent, r.xOffset, "")
		} else {
			fullContent = ""
		}
	}

	// Truncate to fit width and render with background fill
	truncated := ansi.TruncateWc(fullContent, codeW, "")
	sb.WriteString(style.Code.Width(codeW).Render(truncated))

	return sb.String()
}

// renderEmptyLine renders an empty line for padding.
func (r *PatchRenderer) renderEmptyLine(showLineNums bool, codeW int) string {
	style := &r.style.DiffStyle.EqualLine
	var sb strings.Builder

	if showLineNums {
		blank := strings.Repeat(" ", r.beforeNumDigits)
		blankAfter := strings.Repeat(" ", r.afterNumDigits)
		sb.WriteString(style.LineNumber.Render(blank))
		sb.WriteString(style.LineNumber.Render(blankAfter))
	}

	// Use Width() to fill background color
	sb.WriteString(style.Code.Width(codeW).Render(""))

	return sb.String()
}

// hunkFromCount calculates the number of lines in hunk from source.
func hunkFromCount(hunk *diferenco.Hunk) int {
	count := 0
	for _, l := range hunk.Lines {
		if l.Kind != diferenco.Insert {
			count++
		}
	}
	return count
}

// hunkToCount calculates the number of lines in hunk to target.
func hunkToCount(hunk *diferenco.Hunk) int {
	count := 0
	for _, l := range hunk.Lines {
		if l.Kind != diferenco.Delete {
			count++
		}
	}
	return count
}

// formatHunkHeader formats a hunk header.
func formatHunkHeader(fromLine, fromCount, toLine, toCount int, section string) string {
	var sb strings.Builder
	sb.WriteString("@@")
	sb.WriteString(formatHunkRange(fromLine, fromCount, "-"))
	sb.WriteString(formatHunkRange(toLine, toCount, "+"))
	sb.WriteString(" @@")
	if section != "" {
		sb.WriteString(" ")
		sb.WriteString(section)
	}
	return sb.String()
}

// formatHunkRange formats a hunk range like "-1,3" or "-1".
func formatHunkRange(start, count int, prefix string) string {
	switch count {
	case 0:
		return fmt.Sprintf(" %s%d,0", prefix, start)
	case 1:
		return fmt.Sprintf(" %s%d", prefix, start)
	default:
		return fmt.Sprintf(" %s%d,%d", prefix, start, count)
	}
}

// digitCount returns the number of digits in n.
func digitCount(n int) int {
	if n <= 0 {
		return 1
	}
	count := 0
	for n > 0 {
		count++
		n /= 10
	}
	return count
}

// pad left-pads a value to the target width (right-aligned).
func pad(v any, width int) string {
	s := fmt.Sprintf("%v", v)
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
