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

// StyleSetter is an optional interface for StatusBar implementations that
// support style propagation from the host PatchView.
type StyleSetter interface {
	SetStyle(style PatchViewStyle)
}

// DefaultStatusBar is the default status bar implementation.
// It displays: status + separator + path + stats + file count.
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

// Height returns the height of the status bar.
// The bar is wrapped in a rounded border (top + bottom = 2) plus 1 line of
// content, giving a total height of 3.
func (s *DefaultStatusBar) Height() int {
	return 3
}

// View renders the status bar.
func (s *DefaultStatusBar) View(width int) string {
	if len(s.patches) == 0 {
		boxStyle := lipgloss.NewStyle().
			Width(width).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(s.borderColor())
		return boxStyle.Render(" No changes")
	}

	// In lipgloss v2, Width sets the total block width including borders.
	// The rounded border consumes 2 columns, so the usable content width
	// is (width - 2).
	contentWidth := max(width-2, 0)

	p := s.patches[s.cursor]
	stat := p.Stat()
	ps := patchStatus(p)

	// Status indicator
	status := s.statusStyle(ps).Render(ps)

	// Stats
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

	// File count
	fileCount := s.style.FileCount.Render(
		strconv.Itoa(s.cursor+1) + "/" + strconv.Itoa(len(s.patches)))

	// Separator
	sep := s.style.Separator.Render("│")
	sepWidth := lipgloss.Width(sep)

	// Path display – compute available space from measured widths.
	pathDisplay := patchName(p)
	statusWidth := lipgloss.Width(status)
	fileCountWidth := lipgloss.Width(fileCount)
	statsWidth := lipgloss.Width(stats)

	// Layout: " {status} {sep} {path} {stats}  {fileCount}"
	// Fixed columns that are always present (excluding path and stats):
	//   1 (leading space) + statusWidth + 1 (space) + sepWidth + 1 (space)
	//   + 1 (min gap before fileCount) + fileCountWidth
	fixedWidth := 1 + statusWidth + 1 + sepWidth + 1 + 1 + fileCountWidth
	availableForPathAndStats := max(contentWidth-fixedWidth, 0)

	showStats := statsWidth > 0 && availableForPathAndStats > statsWidth+10

	var pathWidth int
	if showStats {
		// path + " " + stats
		pathWidth = max(availableForPathAndStats-statsWidth-1, 0)
	} else {
		pathWidth = availableForPathAndStats
	}

	if pathWidth > 0 && lipgloss.Width(pathDisplay) > pathWidth {
		remove := lipgloss.Width(pathDisplay) - pathWidth + 1
		pathDisplay = ansi.TruncateLeftWc(pathDisplay, remove, "…")
	}
	pathDisplay = s.style.PathDisplay.Render(pathDisplay)

	// Build left side
	var left string
	if showStats {
		left = fmt.Sprintf(" %s %s %s %s", status, sep, pathDisplay, stats)
	} else {
		left = fmt.Sprintf(" %s %s %s", status, sep, pathDisplay)
	}

	// Right-align fileCount with space fill; clamp to avoid overflow.
	leftWidth := lipgloss.Width(left)
	spaceWidth := max(contentWidth-leftWidth-fileCountWidth, 1)

	content := left + strings.Repeat(" ", spaceWidth) + fileCount

	// Safety: if the content is still wider than contentWidth (e.g. due to
	// ambiguous-width characters), hard-truncate to prevent line wrapping
	// inside the border box.
	if cw := lipgloss.Width(content); cw > contentWidth {
		content = ansi.TruncateWc(content, contentWidth, "")
	}

	// Wrap the status line in a rounded border. Width is the total block
	// width including the border itself.
	boxStyle := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.borderColor())

	return boxStyle.Render(content)
}

// borderColor returns the border colour for the status bar box.
func (s *DefaultStatusBar) borderColor() color.Color {
	if s.style.BorderInactive != nil {
		return s.style.BorderInactive
	}
	return fallbackBorderInactive
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
