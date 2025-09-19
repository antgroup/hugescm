package stat

import (
	"fmt"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
)

func red(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;247;112;98m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[31m" + s + "\x1b[0m"
	}
	return s
}

func yellow(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;254;225;64m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[33m" + s + "\x1b[0m"
	default:
	}
	return s
}

func green(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;67;233;123m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[32m" + s + "\x1b[0m"
	default:
	}
	return s
}

func colorE(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;250;112;154m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[31m" + s + "\x1b[0m"
	default:
	}
	return s
}

func blue(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;0;201;255m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[34m" + s + "\x1b[0m"
	default:
	}
	return s
}

func green2(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;32;225;215m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[32m" + s + "\x1b[0m"
	default:
	}
	return s
}

func colorSize(i int64) string {
	return blue(strengthen.FormatSize(i))
}

func colorSizeU(i uint64) string {
	return blue(strengthen.FormatSizeU(i))
}

func colorInt[I int | uint64 | int64](i I) string {
	return blue(fmt.Sprintf("%d", i))
}
