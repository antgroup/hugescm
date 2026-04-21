package viewport

import (
	"charm.land/lipgloss/v2"
)

// Styles contains styling configuration for the viewport
type Styles struct {
	// SelectionPrefix is prepended to each visible line of the selected item.
	// Non-selected lines get equivalent-width blank padding to maintain alignment.
	// Only applied when selection is enabled and this string is non-empty.
	// This is the primary mechanism for selection visibility under NO_COLOR.
	SelectionPrefix string

	FooterStyle       lipgloss.Style
	SelectedItemStyle lipgloss.Style
}

// DefaultStyles returns a set of default styles for the viewport.
// Uses only reverse video — no 256-color or true-color values.
func DefaultStyles() Styles {
	return Styles{
		SelectionPrefix:   "",
		FooterStyle:       lipgloss.NewStyle(),
		SelectedItemStyle: lipgloss.NewStyle().Reverse(true),
	}
}
