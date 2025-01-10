//go:build !windows

package term

func IsCygwinTerminal(fd uintptr) bool {
	return false
}

func detectColorLevelHijack() Level {
	return LevelNone
}
