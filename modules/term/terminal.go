package term

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// Level color level
type Level int

const (
	LevelNone Level = iota
	Level256
	Level16M
)

func (level Level) SupportColor() bool {
	return level > LevelNone
}

var (
	StderrLevel Level
	StdoutLevel Level
)

func isFalse(s string) bool {
	s = strings.ToLower(s)
	return s == "false" || s == "on" || s == "off" || s == "0"
}

func detectForceColor() (Level, bool) {
	forceColorEnv, ok := os.LookupEnv("FORCE_COLOR")
	if !ok {
		return LevelNone, false
	}
	if isFalse(forceColorEnv) {
		return LevelNone, false
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

var (
	termSupports = map[string]Level{
		"mintty":         Level16M,
		"iTerm.app":      Level16M,
		"WezTerm":        Level16M,
		"Apple_Terminal": Level256,
	}
)

func detectColorLevel() Level {
	// detect FORCE_COLOR and level
	if level, ok := detectForceColor(); ok {
		return level
	}
	// detect NO_COLOR
	if noColor, ok := os.LookupEnv("NO_COLOR"); ok && !isFalse(noColor) {
		return LevelNone
	}
	// detect Windows Terminal
	if _, ok := os.LookupEnv("WT_SESSION"); ok {
		return Level16M
	}
	if termApp, ok := os.LookupEnv("TERM_PROGRAM"); ok {
		if level, ok := termSupports[termApp]; ok {
			return level
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
	colorLevel := detectColorLevel()
	if IsTerminal(os.Stderr.Fd()) {
		StderrLevel = colorLevel
	}
	if IsTerminal(os.Stdout.Fd()) {
		StdoutLevel = colorLevel
	}
}

func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd)) || IsCygwinTerminal(fd)
}

func IsNativeTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

func GetSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}
