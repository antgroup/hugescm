package viewport

import (
	"charm.land/lipgloss/v2"
)

// displayManager handles all display/rendering concerns
type displayManager struct {
	// bounds contains the viewport dimensions in terminal cells
	bounds rectangle

	// topItemIdx is the index of the topmost visible item
	topItemIdx int

	// topItemLineOffset is the number of lines in the top item that are above the first visible line
	// Only non-zero when wrapped
	topItemLineOffset int

	// xOffset is the number of terminal cells (width) scrolled right when lines overflow and wrapping is off
	xOffset int

	// styles contains the styling configuration
	styles Styles
}

// newDisplayManager creates a new displayManager with the specified dimensions and styles
func newDisplayManager(width, height int, styles Styles) *displayManager {
	return &displayManager{
		bounds: rectangle{
			width:  max(0, width),
			height: max(0, height),
		},
		topItemIdx:        0,
		topItemLineOffset: 0,
		xOffset:           0,
		styles:            styles,
	}
}

// setBounds sets the viewport dimensions with validation
func (dm *displayManager) setBounds(r rectangle) {
	r.width, r.height = max(0, r.width), max(0, r.height)
	dm.bounds = r
}

// setTopItemIdxAndOffset sets the top item index and line offset
func (dm *displayManager) setTopItemIdxAndOffset(topItemIdx, topItemLineOffset int) {
	dm.topItemIdx, dm.topItemLineOffset = topItemIdx, topItemLineOffset
}

// getNumContentLines returns the number of lines in the content
func (dm *displayManager) getNumContentLines(headerLines int, hasPostHeader bool, hasPreFooter bool, showFooter bool) int {
	contentHeight := dm.bounds.height - headerLines
	if hasPostHeader {
		contentHeight-- // one for post-header
	}
	if hasPreFooter {
		contentHeight-- // one for pre-footer
	}
	if showFooter {
		contentHeight-- // one for footer
	}
	return max(0, contentHeight)
}

// render applies final styling to the display
func (dm *displayManager) render(display string) string {
	return lipgloss.NewStyle().Width(dm.bounds.width).Height(dm.bounds.height).Render(display)
}

// rectangle represents a rectangular area
type rectangle struct {
	width, height int
}
