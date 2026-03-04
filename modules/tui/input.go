package tui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func baseTheme() *huh.Theme {
	t := huh.ThemeBase()

	var (
		normalFg = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
		blue     = lipgloss.AdaptiveColor{Light: "#ace0f9", Dark: "#ace0f9"}
		cream    = lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}
		fuchsia  = lipgloss.Color("#F780E2")
		green    = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
		red      = lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}
	)

	t.Focused.Base = t.Focused.Base.BorderForeground(lipgloss.Color("238"))
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(blue).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(blue).Bold(true).MarginBottom(1)
	t.Focused.Directory = t.Focused.Directory.Foreground(blue)
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.AdaptiveColor{Light: "", Dark: "243"})
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(fuchsia)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(fuchsia)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(fuchsia)
	t.Focused.Option = t.Focused.Option.Foreground(normalFg)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(fuchsia)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(green)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#02CF92", Dark: "#02A877"}).SetString("✓ ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "", Dark: "243"}).SetString("• ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(normalFg)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(cream).Background(fuchsia)
	t.Focused.Next = t.Focused.FocusedButton
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(normalFg).Background(lipgloss.AdaptiveColor{Light: "252", Dark: "237"})

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(green)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(lipgloss.AdaptiveColor{Light: "248", Dark: "238"})
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(fuchsia)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}

func askTitle(format string, a ...any) string {
	return "? " + fmt.Sprintf(format, a...)
}

func AskInput(value *string, format string, a ...any) error {
	i := huh.NewInput().Title(askTitle(format, a...)).Inline(true).Value(value).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("input cannot be empty")
			}
			return nil
		})
	i.WithTheme(baseTheme())
	return i.RunAccessible(os.Stdout, os.Stdin)
}

// AskPassword prompts for a password input with asterisk masking.
// It uses the huh theme for styling and displays asterisks (*) for each character entered.
// Unlike RunAccessible, this implementation shows visual feedback while still preserving
// the terminal output (no screen refresh on Enter).
// It properly handles non-ANSI characters (UTF-8, Chinese, Emoji) by using rune-level processing.
// Cross-platform support: Windows, Linux, macOS (via golang.org/x/term).
func AskPassword(password *string, format string, a ...any) error {
	theme := baseTheme()
	styles := theme.Focused

	validator := func(input string) error {
		if input == "" {
			return fmt.Errorf("password cannot be empty")
		}
		return nil
	}

	for {
		title := styles.Title.PaddingRight(1).Render(askTitle(format, a...))
		fmt.Print(title)

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
				fmt.Println()

				input := string(passwordRunes)
				if err := validator(input); err != nil {
					fmt.Println(styles.ErrorMessage.Render(err.Error()))
					break
				}

				*password = input
				return nil

			case 127, 8: // Backspace/Delete (DEL=127 Unix, Backspace=8 Windows)
				if len(passwordRunes) > 0 {
					passwordRunes = passwordRunes[:len(passwordRunes)-1]
					fmt.Print("\b \b")
				}

			case 3: // Ctrl+C
				_ = term.Restore(fd, oldState)
				fmt.Println()
				return errors.New("input cancelled")

			default:
				if r == utf8.RuneError {
					// Invalid UTF-8 sequence, skip it
					continue
				}
				if !unicode.IsControl(r) {
					passwordRunes = append(passwordRunes, r)
					fmt.Print("*")
				}
			}
		}
	}
}
