package filterableviewport

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines the key bindings for the filterable viewport.
// Filter mode activation keys (exact, regex, case-insensitive) are defined on
// each FilterMode.Key — see DefaultFilterModes() and WithFilterModes().
type KeyMap struct {
	ApplyFilterKey             key.Binding
	CancelFilterKey            key.Binding
	ToggleMatchingItemsOnlyKey key.Binding
	NextMatchKey               key.Binding
	PrevMatchKey               key.Binding
	SearchHistoryPrevKey       key.Binding
	SearchHistoryNextKey       key.Binding
}

// DefaultKeyMap returns a default keymap for the filterable viewport
func DefaultKeyMap() KeyMap {
	return KeyMap{
		ApplyFilterKey: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "apply filter"),
		),
		CancelFilterKey: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel filter"),
		),
		ToggleMatchingItemsOnlyKey: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "toggle matches only"),
		),
		NextMatchKey: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		PrevMatchKey: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "previous match"),
		),
		SearchHistoryPrevKey: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "previous search"),
		),
		SearchHistoryNextKey: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "next search"),
		),
	}
}
