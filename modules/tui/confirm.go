package tui

import (
	"fmt"
	"os"

	"charm.land/huh/v2"
)

// AskConfirm prompts for a confirmation using huh library.
// It provides a user-friendly yes/no confirmation dialog.
//
// Note: Output goes to stderr to avoid interfering with stdout piping.
// This also allows colors to work correctly when stdout is piped but stderr is a TTY.
func AskConfirm(confirm *bool, format string, a ...any) error {
	c := huh.NewConfirm().Title("? " + fmt.Sprintf(format, a...)).Inline(true).Value(confirm)
	return c.RunAccessible(os.Stderr, os.Stdin)
}
