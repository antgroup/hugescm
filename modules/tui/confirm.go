package tui

import (
	"os"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Color definitions for huh theme
var (
	normalFg = compat.AdaptiveColor{Light: lipgloss.Color("235"), Dark: lipgloss.Color("252")}
	fuchsia  = lipgloss.Color("#F780E2")
	green    = compat.AdaptiveColor{Light: lipgloss.Color("#02BA84"), Dark: lipgloss.Color("#02BF87")}
)

// baseTheme returns a custom theme for huh widgets.
func baseTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		t := huh.ThemeBase(isDark)

		t.Focused.Title = t.Focused.Title.Foreground(blue).Bold(true)
		t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
		t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)
		t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(green)
		t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(compat.AdaptiveColor{Light: lipgloss.Color("248"), Dark: lipgloss.Color("238")})
		t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(fuchsia)
		t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(normalFg)

		t.Blurred = t.Focused
		t.Blurred.TextInput.Cursor = lipgloss.NewStyle()

		return t
	})
}

// AskConfirm prompts for a confirmation using huh library.
// It provides a user-friendly yes/no confirmation dialog.
//
// Note: Output goes to stderr to avoid interfering with stdout piping.
func AskConfirm(confirm *bool, format string, a ...any) error {
	c := huh.NewConfirm().Title(askTitle(format, a...)).Inline(true).Value(confirm).WithTheme(baseTheme())
	return c.RunAccessible(os.Stderr, os.Stdin)
}
