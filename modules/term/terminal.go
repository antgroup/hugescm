package term

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// Level represents the color support level of a terminal.
//
// The levels are:
//   - LevelNone: No color support, ANSI codes are stripped
//   - Level256: 256-color palette support (standard ANSI colors)
//   - Level16M: 16 million colors (24-bit truecolor/RGB) support
type Level int

const (
	LevelNone Level = iota
	Level256
	Level16M
)

// SupportColor returns true if the terminal supports any color output.
func (level Level) SupportColor() bool {
	return level > LevelNone
}

var (
	// StderrLevel is the detected color support level for stderr.
	StderrLevel Level
	// StdoutLevel is the detected color support level for stdout.
	StdoutLevel Level
)

// isFalse checks if a string value represents a false/negative value.
// Recognized false values: false, off, 0, no (case-insensitive).
func isFalse(s string) bool {
	s = strings.ToLower(s)
	return s == "false" || s == "off" || s == "0" || s == "no"
}

// detectForceColor checks the FORCE_COLOR environment variable and returns
// the forced color level along with a boolean indicating if forcing is enabled.
//
// FORCE_COLOR values:
//   - 0, false, off, no: No color (LevelNone)
//   - 3: Truecolor (Level16M)
//   - any other value: 256-color (Level256)
func detectForceColor() (Level, bool) {
	forceColorEnv, ok := os.LookupEnv("FORCE_COLOR")
	if !ok {
		return LevelNone, false
	}
	if isFalse(forceColorEnv) {
		return LevelNone, true
	}
	if forceColorEnv == "3" {
		return Level16M, true
	}
	return Level256, true
}

// https://github.com/gui-cs/Terminal.Gui/issues/48
// https://github.com/termstandard/colors
// https://github.com/microsoft/terminal/issues/11057
// https://marvinh.dev/blog/terminal-colors/
// https://github.com/microsoft/terminal/issues/13006
// https://github.com/termstandard/colors/issues/69 Terminal.app for macOS Tahoe supports truecolor

var (
	// termSupports maps terminal program names to their color capabilities.
	// This list includes known terminals that support 16M colors.
	termSupports = map[string]Level{
		"mintty":    Level16M,
		"iTerm.app": Level16M,
		"WezTerm":   Level16M,
	}
)

// detectColorLevel detects the terminal's color support capability by checking
// various environment variables and terminal type indicators.
//
// Detection order:
//  1. Windows Terminal (WT_SESSION env var)
//  2. Known terminal programs (TERM_PROGRAM env var)
//  3. COLORTERM and TERM env vars for truecolor/256color keywords
//  4. Platform-specific detection (Cygwin/Windows console)
func detectColorLevel() Level {
	// detect Windows Terminal
	if _, ok := os.LookupEnv("WT_SESSION"); ok {
		return Level16M
	}
	if termApp, ok := os.LookupEnv("TERM_PROGRAM"); ok {
		if colorLevel, ok := termSupports[termApp]; ok {
			return colorLevel
		}
	}
	colorTermEnv := os.Getenv("COLORTERM")
	termEnv := os.Getenv("TERM")
	if strings.Contains(termEnv, "24bit") ||
		strings.Contains(termEnv, "truecolor") ||
		strings.Contains(colorTermEnv, "24bit") ||
		strings.Contains(colorTermEnv, "truecolor") {
		return Level16M
	}
	if strings.Contains(termEnv, "256") || strings.Contains(colorTermEnv, "256") {
		return Level256
	}
	return detectColorLevelHijack()
}

func init() {
	// Detect FORCE_COLOR and override detection
	if colorLevel, ok := detectForceColor(); ok {
		StderrLevel = colorLevel
		StdoutLevel = colorLevel
		return
	}
	// Detect NO_COLOR (https://no-color.org/)
	if noColor, ok := os.LookupEnv("NO_COLOR"); ok && !isFalse(noColor) {
		return
	}
	// Auto-detect color level from environment
	colorLevel := detectColorLevel()
	if IsTerminal(os.Stderr.Fd()) {
		StderrLevel = colorLevel
	}
	if IsTerminal(os.Stdout.Fd()) {
		StdoutLevel = colorLevel
	}
}

// IsTerminal returns true if the given file descriptor is connected to a terminal.
// This works for both native terminals and Cygwin/MSYS2 pseudo-terminals.
func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd)) || IsCygwinTerminal(fd)
}

// IsNativeTerminal returns true if the given file descriptor is a native terminal
// (not a Cygwin/MSYS2 pseudo-terminal).
func IsNativeTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// GetSize returns the dimensions of the terminal for the given file descriptor.
// Returns width, height in characters, and any error encountered.
func GetSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}
