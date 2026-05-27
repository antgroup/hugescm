package patchview

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/antgroup/hugescm/modules/diferenco"
)

// StatusBar is the interface for rendering a status bar in the patch view.
//
// Contract (locked down by view_test.go):
//   - lipgloss.Width(View(w))  == w
//   - lipgloss.Height(View(w)) == Height()
//
// Violating either side causes JoinVertical(header, mainContent, footer) to
// either misalign columns or push the diff pane into the file list region.
type StatusBar interface {
	View(width int) string
	Height() int
}

// CursorSetter is an optional interface for StatusBar implementations
// that need to be notified when the cursor changes.
type CursorSetter interface {
	SetCursor(idx int)
}

// PatchesSetter is an optional interface for StatusBar implementations
// that need access to the patches data.
type PatchesSetter interface {
	SetPatches(patches []*diferenco.Patch)
}

// statusBarBorderSize is the horizontal column overhead of the rounded
// border (1 column on each side).
//
// IMPORTANT — lipgloss v2 Style.Width() semantics:
// Style.Width(w) sets the TOTAL block width (including the border). The
// content area inside the border is therefore (w - statusBarBorderSize).
// We use this constant to derive the inner content width that callers
// must fit into; the box-level Width is simply `width` itself.
const statusBarBorderSize = 2

// fallbackHeaderBorder is used when the active style does not set
// HeaderBorder (e.g. tests that build a zero-value PatchViewStyle).
var fallbackHeaderBorder color.Color = lipgloss.Color("8")

// DefaultStatusBar is the default status bar implementation.
//
// Layout (single content line inside a rounded border):
//
//	┌────────────────────────────────────────────────────────────┐
//	│ M │ path/to/file.go               +12 -3            2/8    │
//	└────────────────────────────────────────────────────────────┘
//
// Total height is 3 (1 content line + 2 border lines).
type DefaultStatusBar struct {
	patches []*diferenco.Patch
	cursor  int
	style   PatchViewStyle
}

// NewDefaultStatusBar creates a new DefaultStatusBar.
func NewDefaultStatusBar() *DefaultStatusBar {
	return &DefaultStatusBar{
		style: DefaultStyle(),
	}
}

// SetStyle sets the style for the status bar.
func (s *DefaultStatusBar) SetStyle(style PatchViewStyle) {
	s.style = style
}

// SetPatches sets the patches data.
func (s *DefaultStatusBar) SetPatches(patches []*diferenco.Patch) {
	s.patches = patches
}

// SetCursor sets the current cursor position.
func (s *DefaultStatusBar) SetCursor(idx int) {
	s.cursor = idx
}

// Height returns the total height of the status bar including its rounded
// border (1 content line + 2 border lines = 3).
func (s *DefaultStatusBar) Height() int {
	return 3
}

// View renders the status bar as a rounded-border card of the requested
// total width. The contract is:
//
//	lipgloss.Width(View(w))  == w
//	lipgloss.Height(View(w)) == 3
//
// In lipgloss v2 Style.Width(w) is the TOTAL block width (border + content),
// so we pass `w` directly. The inner content width fed into renderContent
// is (w - 2) so the inner text does not collide with the side borders.
func (s *DefaultStatusBar) View(width int) string {
	if width <= 0 {
		return ""
	}

	if len(s.patches) == 0 {
		return s.renderBox(width, " No changes")
	}

	// Guard against a stale cursor (e.g. cursor outside a freshly-trimmed
	// patches slice). Pin to a valid index.
	cursor := s.cursor
	if cursor < 0 || cursor >= len(s.patches) {
		cursor = 0
	}

	contentWidth := max(width-statusBarBorderSize, 0)
	content := s.renderContent(cursor, contentWidth)
	return s.renderBox(width, content)
}

// renderBox wraps content in a rounded border whose total rendered width
// is exactly `width`. lipgloss v2 Width(w) = total block width.
func (s *DefaultStatusBar) renderBox(width int, content string) string {
	boxStyle := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.borderColor())
	return boxStyle.Render(content)
}

// renderContent renders the single-line status content, fitted to
// contentWidth and right-padded with spaces so the file count sits flush
// against the right edge. The caller is responsible for wrapping the
// returned string in the border box.
func (s *DefaultStatusBar) renderContent(cursor, contentWidth int) string {
	p := s.patches[cursor]
	stat := p.Stat()
	ps := patchStatus(p)

	// Status indicator (single bold letter)
	status := s.statusStyle(ps).Render(ps)

	// Stats string ("+12 -3" / "+12" / "-3" / "")
	var stats string
	switch {
	case stat.Addition > 0 && stat.Deletion > 0:
		stats = s.style.Addition.Render("+"+strconv.Itoa(stat.Addition)) + " " +
			s.style.Deletion.Render("-"+strconv.Itoa(stat.Deletion))
	case stat.Addition > 0:
		stats = s.style.Addition.Render("+" + strconv.Itoa(stat.Addition))
	case stat.Deletion > 0:
		stats = s.style.Deletion.Render("-" + strconv.Itoa(stat.Deletion))
	}

	// File count "i/N"
	fileCount := s.style.FileCount.Render(
		strconv.Itoa(cursor+1) + "/" + strconv.Itoa(len(s.patches)))

	// Separator glyph
	sep := s.style.Separator.Render("│")

	// Measured column widths
	statusWidth := lipgloss.Width(status)
	sepWidth := lipgloss.Width(sep)
	statsWidth := lipgloss.Width(stats)
	fileCountWidth := lipgloss.Width(fileCount)

	// Layout: " {status} {sep} {path} {stats}  {fileCount} "
	// fixedWidth covers everything except the path itself.
	//   1 (lead space) + status + 1 (space) + sep + 1 (space)
	//   + (statsWidth>0 ? 1 (space) + stats : 0)
	//   + 2 (min gap before fileCount)
	//   + fileCount
	//   + 1 (trailing space)
	fixedWidth := 1 + statusWidth + 1 + sepWidth + 1 + 2 + fileCountWidth + 1
	if statsWidth > 0 {
		fixedWidth += 1 + statsWidth
	}

	// Path display, left-truncated to fit. If even the truncated path
	// would not fit we drop stats first, then drop more path width.
	pathDisplay := patchName(p)
	pathWidth := max(contentWidth-fixedWidth, 0)

	// If stats does not fit, drop it and recompute.
	showStats := statsWidth > 0 && pathWidth >= 1
	if !showStats {
		// Recompute fixedWidth without stats.
		fixedWidth = 1 + statusWidth + 1 + sepWidth + 1 + 2 + fileCountWidth + 1
		pathWidth = max(contentWidth-fixedWidth, 0)
	}

	if pathWidth > 0 && lipgloss.Width(pathDisplay) > pathWidth {
		// TruncateLeftWc keeps the rightmost characters (the file name)
		// which is what users care about; leading directories collapse to "…".
		remove := lipgloss.Width(pathDisplay) - pathWidth + 1
		pathDisplay = ansi.TruncateLeftWc(pathDisplay, remove, "…")
	}
	pathDisplay = s.style.PathDisplay.Render(pathDisplay)

	// Compose left segment (status + sep + path [+ stats]).
	var left string
	if showStats {
		left = fmt.Sprintf(" %s %s %s %s", status, sep, pathDisplay, stats)
	} else {
		left = fmt.Sprintf(" %s %s %s", status, sep, pathDisplay)
	}

	// Right-align fileCount with space fill; clamp to keep total at
	// contentWidth.
	leftWidth := lipgloss.Width(left)
	spaceWidth := max(contentWidth-leftWidth-fileCountWidth-1, 1)
	content := left + strings.Repeat(" ", spaceWidth) + fileCount + " "

	// Safety net: if anything still overflows (e.g. ambiguous-width chars
	// in the path), hard-truncate. Without this, the box border would
	// wrap to a second row.
	if cw := lipgloss.Width(content); cw > contentWidth {
		content = ansi.TruncateWc(content, contentWidth, "")
	}
	return content
}

// borderColor returns the rounded-border color for the status bar card.
// Falls back to a neutral grey when the active style leaves HeaderBorder
// nil (zero-value PatchViewStyle in tests, etc.).
func (s *DefaultStatusBar) borderColor() color.Color {
	if s.style.HeaderBorder != nil {
		return s.style.HeaderBorder
	}
	return fallbackHeaderBorder
}

// statusStyle returns the style for a status character.
func (s *DefaultStatusBar) statusStyle(status string) lipgloss.Style {
	switch status {
	case "A":
		return s.style.StatusAdded
	case "D":
		return s.style.StatusDeleted
	case "R":
		return s.style.StatusRenamed
	default:
		return s.style.StatusModified
	}
}

// patchName returns the display name for a patch.
func patchName(p *diferenco.Patch) string {
	if p == nil {
		return ""
	}
	switch {
	case p.From == nil && p.To != nil:
		return p.To.Name
	case p.From != nil && p.To == nil:
		return p.From.Name
	case p.From != nil && p.To != nil && p.From.Name != p.To.Name:
		return p.From.Name + " → " + p.To.Name
	case p.To != nil:
		return p.To.Name
	case p.From != nil:
		return p.From.Name
	default:
		return ""
	}
}

// patchStatus returns the status character for a patch.
func patchStatus(p *diferenco.Patch) string {
	if p == nil {
		return "M"
	}
	switch {
	case p.From == nil:
		return "A"
	case p.To == nil:
		return "D"
	case p.From != nil && p.To != nil && p.From.Name != p.To.Name:
		return "R"
	default:
		return "M"
	}
}
