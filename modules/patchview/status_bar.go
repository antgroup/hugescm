package patchview

import (
	"fmt"
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

// Height returns the height of the status bar (always 1).
func (s *DefaultStatusBar) Height() int {
	return 1
}

// View renders the status bar.
func (s *DefaultStatusBar) View(width int) string {
	if len(s.patches) == 0 {
		return s.style.HeaderBg.Width(width).Render(" No changes")
	}

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

	// Path display
	pathDisplay := patchName(p)
	fileCountWidth := lipgloss.Width(fileCount)
	statsWidth := lipgloss.Width(stats)
	fixedWidth := 1 + 1 + 3 + fileCountWidth + 2 // space + status + space + sep + space + count + padding

	availableForPathAndStats := width - fixedWidth
	showStats := availableForPathAndStats > statsWidth+10

	var pathWidth int
	if showStats {
		pathWidth = availableForPathAndStats - statsWidth - 1
	} else {
		pathWidth = availableForPathAndStats
	}
	pathWidth = max(pathWidth, 0)

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

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(fileCount)
	spaceWidth := max(width-leftWidth-rightWidth, 0)

	return s.style.HeaderBg.Width(width).Render(
		left + strings.Repeat(" ", spaceWidth) + fileCount)
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
