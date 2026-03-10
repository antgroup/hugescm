package tui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"unicode"
	"unicode/utf8"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"golang.org/x/term"
)

// askTitle formats a title with a prefix.
func askTitle(format string, a ...any) string {
	return "? " + fmt.Sprintf(format, a...)
}

// baseTheme returns a custom theme for huh input fields.
func baseTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		t := huh.ThemeBase(isDark)

		var (
			blue     = compat.AdaptiveColor{Light: lipgloss.Color("#ace0f9"), Dark: lipgloss.Color("#ace0f9")}
			red      = compat.AdaptiveColor{Light: lipgloss.Color("#FF4672"), Dark: lipgloss.Color("#ED567A")}
			normalFg = compat.AdaptiveColor{Light: lipgloss.Color("235"), Dark: lipgloss.Color("252")}
			fuchsia  = lipgloss.Color("#F780E2")
			green    = compat.AdaptiveColor{Light: lipgloss.Color("#02BA84"), Dark: lipgloss.Color("#02BF87")}
		)

		// Title styling
		t.Focused.Title = t.Focused.Title.Foreground(blue).Bold(true)
		t.Focused.Description = t.Focused.Description.Foreground(compat.AdaptiveColor{Light: lipgloss.Color(""), Dark: lipgloss.Color("243")})
		t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
		t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)

		// Text input styling
		t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(green)
		t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(compat.AdaptiveColor{Light: lipgloss.Color("248"), Dark: lipgloss.Color("238")})
		t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(fuchsia)
		t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(normalFg)

		// Copy to blurred state
		t.Blurred = t.Focused
		t.Blurred.TextInput.Cursor = lipgloss.NewStyle()

		return t
	})
}

// AskInput prompts for a text input using huh library.
// It provides a user-friendly input interface with validation.
//
// Note: Output goes to stderr to avoid interfering with stdout piping.
// This also allows colors to work correctly when stdout is piped but stderr is a TTY.
func AskInput(value *string, format string, a ...any) error {
	i := huh.NewInput().Title(askTitle(format, a...)).Inline(true).Value(value).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("input cannot be empty")
			}
			return nil
		}).WithTheme(baseTheme())
	return i.RunAccessible(os.Stderr, os.Stdin)
}

// AskPassword prompts for a password input with asterisk masking.
// It uses lipgloss for styling and displays asterisks (*) for each character entered.
// This implementation shows visual feedback while preserving the terminal output.
// It properly handles non-ANSI characters (UTF-8, Chinese, Emoji) by using rune-level processing.
// Cross-platform support: Windows, Linux, macOS (via golang.org/x/term).
//
// Note: Output goes to stderr to avoid interfering with stdout piping.
// This also allows colors to work correctly when stdout is piped but stderr is a TTY.
func AskPassword(password *string, format string, a ...any) error {
	// Color definitions
	blue := compat.AdaptiveColor{Light: lipgloss.Color("#ace0f9"), Dark: lipgloss.Color("#ace0f9")}
	red := compat.AdaptiveColor{Light: lipgloss.Color("#FF4672"), Dark: lipgloss.Color("#ED567A")}

	// Use lipgloss styles - no renderer needed in v2
	titleStyle := lipgloss.NewStyle().Foreground(blue).Bold(true).PaddingRight(1)
	errorStyle := lipgloss.NewStyle().Foreground(red)

	validator := func(input string) error {
		if input == "" {
			return fmt.Errorf("password cannot be empty")
		}
		return nil
	}

	for {
		title := titleStyle.Render(askTitle(format, a...))
		fmt.Fprint(os.Stderr, title)

		fd := int(os.Stdin.Fd())
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return fmt.Errorf("failed to set raw mode: %w", err)
		}

		var passwordRunes []rune
		reader := bufio.NewReader(os.Stdin)

		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				_ = term.Restore(fd, oldState)
				return fmt.Errorf("read error: %w", err)
			}

			switch r {
			case '\r', '\n':
				_ = term.Restore(fd, oldState)
				fmt.Fprintln(os.Stderr)

				input := string(passwordRunes)
				if err := validator(input); err != nil {
					fmt.Fprintln(os.Stderr, errorStyle.Render(err.Error()))
					break
				}

				*password = input
				return nil

			case 127, 8: // Backspace/Delete (DEL=127 Unix, Backspace=8 Windows)
				if len(passwordRunes) > 0 {
					passwordRunes = passwordRunes[:len(passwordRunes)-1]
					fmt.Fprint(os.Stderr, "\b \b")
				}

			case 3: // Ctrl+C
				_ = term.Restore(fd, oldState)
				fmt.Fprintln(os.Stderr)
				return errors.New("input cancelled")

			default:
				if r == utf8.RuneError {
					// Invalid UTF-8 sequence, skip it
					continue
				}
				if !unicode.IsControl(r) {
					passwordRunes = append(passwordRunes, r)
					fmt.Fprint(os.Stderr, "*")
				}
			}
		}
	}
}
