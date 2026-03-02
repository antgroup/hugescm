package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
)

func AskConfirm(confirm *bool, format string, a ...any) error {
	c := huh.NewConfirm().Title("? " + fmt.Sprintf(format, a...)).Inline(true).Value(confirm)
	c.WithTheme(baseTheme())
	return c.RunAccessible(os.Stdout, os.Stdin)
}
