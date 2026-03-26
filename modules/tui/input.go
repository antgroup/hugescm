package tui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"golang.org/x/term"
)

// ErrInterrupted is returned when user presses Ctrl+C or Ctrl+D.
var ErrInterrupted = errors.New("interrupted")

const maxAttempts = 3

// Color definitions (package-level to avoid recreation)
var (
	blue = compat.AdaptiveColor{Light: lipgloss.Color("#ace0f9"), Dark: lipgloss.Color("#ace0f9")}
	red  = compat.AdaptiveColor{Light: lipgloss.Color("#FF4672"), Dark: lipgloss.Color("#ED567A")}
)

var (
	errorStyle = lipgloss.NewStyle().Foreground(red)
	titleStyle = lipgloss.NewStyle().Foreground(blue).Bold(true)
)

// askTitle formats a title with a prefix.
func askTitle(format string, a ...any) string {
	return "? " + fmt.Sprintf(format, a...)
}

// readLine reads a line with proper CJK/emoji backspace handling.
// mask=0 shows input directly; otherwise each rune is replaced by mask.
func readLine(mask rune, format string, a ...any) (string, error) {
	title := titleStyle.Render(askTitle(format, a...))
	_, _ = lipgloss.Fprint(os.Stderr, title)
	fd := int(os.Stdin.Fd())

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(fd, oldState) // nolint

	var inputRunes []rune
	reader := bufio.NewReader(os.Stdin)

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Fprint(os.Stderr, "\r\n")
				return "", ErrInterrupted
			}
			return "", fmt.Errorf("read error: %w", err)
		}

		switch r {
		case '\r', '\n':
			fmt.Fprint(os.Stderr, "\r\n")
			return string(inputRunes), nil
		case 127, 8: // Backspace
			if len(inputRunes) > 0 {
				inputRunes = inputRunes[:len(inputRunes)-1]
				redrawLine(title, inputRunes, mask)
			}
		case 3: // Ctrl+C
			fmt.Fprint(os.Stderr, "\r\n")
			return "", ErrInterrupted
		case 4: // Ctrl+D
			fmt.Fprint(os.Stderr, "\r\n")
			return "", ErrInterrupted
		case 27: // ESC sequence (arrow keys, etc.)
			for {
				b, err := reader.ReadByte()
				if err != nil {
					break
				}
				if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
					break
				}
			}
			continue
		default:
			if r == utf8.RuneError {
				continue
			}
			if !unicode.IsControl(r) {
				inputRunes = append(inputRunes, r)
				if mask != 0 {
					fmt.Fprint(os.Stderr, string(mask))
				} else {
					fmt.Fprint(os.Stderr, string(r))
				}
			}
		}
	}
}

// redrawLine redraws the input line, correctly handling CJK/emoji characters.
func redrawLine(title string, runes []rune, mask rune) {
	fmt.Fprint(os.Stderr, "\r")
	fmt.Fprint(os.Stderr, title)
	if mask != 0 {
		fmt.Fprint(os.Stderr, strings.Repeat(string(mask), len(runes)))
	} else {
		fmt.Fprint(os.Stderr, string(runes))
	}
	fmt.Fprint(os.Stderr, "\x1b[K")
}

// AskInput prompts for a text input with proper CJK/emoji backspace handling.
//
// Note: Output goes to stderr to avoid interfering with stdout piping.
func AskInput(value *string, format string, a ...any) error {
	input, err := readLine(0, format, a...)
	if err != nil {
		return err
	}

	*value = input
	return nil
}

// AskPassword prompts for a password input with asterisk masking.
// It properly handles UTF-8, CJK characters, emoji, and terminal control sequences.
// Cross-platform support: Windows, Linux, macOS (via golang.org/x/term).
//
// Note: Output goes to stderr to avoid interfering with stdout piping.
func AskPassword(password *string, format string, a ...any) error {
	for range maxAttempts {
		input, err := readLine('*', format, a...)
		if err != nil {
			return err
		}

		if input = strings.TrimSpace(input); input == "" {
			_, _ = lipgloss.Fprintln(os.Stderr, errorStyle.Render("password cannot be empty"))
			continue
		}

		*password = input
		return nil
	}
	return fmt.Errorf("failed to get password after %d attempts", maxAttempts)
}
