package term

import (
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/strengthen"
	"golang.org/x/term"
)

type ColorMode int

const (
	NO_COLOR ColorMode = iota
	HAS_256COLOR
	HAS_TRUECOLOR
)

var (
	StderrMode ColorMode
	StdoutMode ColorMode
)

func detectTermColorMode() ColorMode {
	if strengthen.SimpleAtob(os.Getenv("ZETA_FORCE_TRUECOLOR"), false) {
		return HAS_TRUECOLOR
	}
	if strengthen.SimpleAtob(os.Getenv("NO_COLOR"), false) {
		return NO_COLOR
	}
	if _, ok := os.LookupEnv("WT_SESSION"); ok {
		return HAS_TRUECOLOR
	}
	colorTermEnv := os.Getenv("COLORTERM")
	termEnv := os.Getenv("TERM")
	if strings.Contains(termEnv, "24bit") ||
		strings.Contains(termEnv, "truecolor") ||
		strings.Contains(colorTermEnv, "24bit") ||
		strings.Contains(colorTermEnv, "truecolor") {
		return HAS_TRUECOLOR
	}
	if strings.Contains(termEnv, "256") || strings.Contains(colorTermEnv, "256") {
		return HAS_256COLOR
	}
	return NO_COLOR
}

func init() {
	colorMode := detectTermColorMode()
	if IsTerminal(os.Stderr.Fd()) {
		StderrMode = colorMode
	}
	if IsTerminal(os.Stdout.Fd()) {
		StdoutMode = colorMode
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
