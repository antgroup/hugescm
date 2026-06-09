// Copyright (c) Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package term

import (
	"image/color"
	"os"
	"sync"

	"charm.land/lipgloss/v2"
)

// supportsOSCQuery returns true if the current terminal is likely to
// support OSC 11 background color queries without leaking escape responses.
//
// The heuristic: if detectColorLevel (environment-based, ignoring FORCE_COLOR)
// reports Level16M, the terminal is modern enough to handle OSC 11.
// Additionally, VTE_VERSION is checked as a supplementary signal for terminals
// that may not advertise truecolor via COLORTERM/TERM.
func supportsOSCQuery() bool {
	if detectColorLevel() >= Level16M {
		return true
	}
	// VTE-based terminals (GNOME Terminal, Tilix, Xfce Terminal, etc.)
	if _, ok := os.LookupEnv("VTE_VERSION"); ok {
		return true
	}
	return false
}

var (
	detectOnce sync.Once
	hasDarkBg  = true // default to dark when detection is skipped or fails
)

func detectBackground() {
	if !supportsOSCQuery() {
		return // keep default (dark)
	}
	if !IsTerminal(os.Stdin.Fd()) || !IsTerminal(os.Stdout.Fd()) {
		return // not a terminal, keep default
	}
	hasDarkBg = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}

// HasDarkBackground reports whether the terminal has a dark background.
// Detection is lazy (executed at most once) and only queries the terminal
// if it is known to support OSC 11 reliably. Otherwise it defaults to true.
func HasDarkBackground() bool {
	detectOnce.Do(detectBackground)
	return hasDarkBg
}

// AdaptiveColor holds a pair of colors for light and dark backgrounds.
// It implements [image/color.Color] and selects the appropriate variant
// based on the terminal's background at runtime.
type AdaptiveColor struct {
	Light color.Color
	Dark  color.Color
}

// RGBA implements [image/color.Color].
func (c AdaptiveColor) RGBA() (uint32, uint32, uint32, uint32) {
	if HasDarkBackground() {
		return c.Dark.RGBA()
	}
	return c.Light.RGBA()
}
