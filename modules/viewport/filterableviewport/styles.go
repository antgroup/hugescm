package filterableviewport

import (
	"charm.land/lipgloss/v2"
)

// Styles contains styling configuration for the filterable viewport
type Styles struct {
	Match MatchStyles
}

// MatchStyles contains styles for matches in the filterable viewport
type MatchStyles struct {
	Focused           lipgloss.Style
	FocusedIfSelected lipgloss.Style // used when the focused match is on the selected item
	Unfocused         lipgloss.Style
}

// DefaultMatchStyles returns a set of default styles for matches.
// Uses only reverse video and safe ANSI colors — no 256-color or true-color values.
func DefaultMatchStyles() MatchStyles {
	return MatchStyles{
		Focused:           lipgloss.NewStyle().Reverse(true).Foreground(lipgloss.Cyan),
		FocusedIfSelected: lipgloss.NewStyle().Reverse(true).Foreground(lipgloss.Cyan),
		Unfocused:         lipgloss.NewStyle().Reverse(true).Foreground(lipgloss.BrightRed),
	}
}

// DefaultStyles returns a set of default styles for the filterable viewport
func DefaultStyles() Styles {
	return Styles{
		Match: DefaultMatchStyles(),
	}
}
