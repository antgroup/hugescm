// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package term

import (
	"os"
	"strconv"
	"strings"
)

// ImageProtocol represents an inline-image protocol supported by the
// hosting terminal.
type ImageProtocol int

const (
	// ImageNone means the terminal is not known to support any inline image
	// protocol; callers should fall back to a textual representation
	// (e.g. a hex dump).
	ImageNone ImageProtocol = iota
	// ImageITerm2 is the iTerm2 OSC 1337 inline image protocol, also
	// implemented by WezTerm, mintty, Ghostty and Otty.
	ImageITerm2
	// ImageKitty is the Kitty graphics protocol, also implemented by WezTerm,
	// Ghostty, recent Konsole builds, Windows Terminal and VS Code.
	ImageKitty
)

// String returns a short, lower-case name for the protocol, suitable for
// logging and configuration values.
func (p ImageProtocol) String() string {
	switch p {
	case ImageITerm2:
		return "iterm2"
	case ImageKitty:
		return "kitty"
	default:
		return "none"
	}
}

// Supported reports whether the protocol is a real inline image protocol.
func (p ImageProtocol) Supported() bool {
	return p != ImageNone
}

// minKonsoleKittyVersion is the first Konsole release that ships a
// usable Kitty graphics implementation. Earlier versions either lack
// support entirely or have severe rendering bugs, so we gate detection
// on the reported KONSOLE_VERSION (encoded as MAJOR*10000 + MINOR*100).
const minKonsoleKittyVersion = 220400 // Konsole 22.04

// DetectImageProtocol inspects environment variables to determine which
// inline-image protocol (if any) is supported by the current terminal.
//
// This is intentionally an on-demand check rather than a package-init side
// effect: image rendering is the exception rather than the rule, so the
// environment probe should only run once the caller has decided it actually
// has an image to display.
//
// Detection order:
//  1. Kitty graphics: TERM=xterm-kitty, KITTY_WINDOW_ID set,
//     KONSOLE_VERSION >= 22.04, TERM_PROGRAM in {ghostty, WezTerm, kitty,
//     vscode}, or WT_SESSION (Windows Terminal).
//  2. iTerm2 OSC 1337: TERM_PROGRAM in {iTerm.app, mintty, otty}, or
//     LC_TERMINAL=iTerm2.
//
// WezTerm also speaks iTerm2's protocol and Sixel, but Kitty graphics is
// strictly richer (24-bit colour, no quantisation, chunked transfers), so
// it wins the tie.
//
// VS Code's integrated terminal implements both the iTerm2 OSC 1337
// inline-image protocol and the Kitty graphics protocol (since VS Code
// 1.110 — microsoft/vscode#286141, xtermjs/xterm.js#5592). Kitty graphics
// is strictly richer, so VS Code is mapped there unconditionally —
// matching the precedent set by WezTerm — betting that the bulk of VS
// Code users will be on a Stable release >= 1.110 by the time zeta
// ships broadly. Image rendering in VS Code is also opt-in: callers must
// set terminal.integrated.enableImages=true in settings.json or neither
// protocol will produce visible output.
//
// Caveat: current Stable builds of VS Code can show blurry text cell
// antialiasing next to Kitty-rendered images, and pre-1.110 builds will
// silently drop Kitty APC codes (treating them as ASCII passthrough).
// Both are transient issues; we accept the trade-off betting they will
// be resolved before zeta ships broadly. Users who need to force iTerm2
// can do so by overriding TERM_PROGRAM before invoking zeta.
//
// Anything else returns ImageNone. Terminals with unreliable support
// (Warp) are intentionally excluded from the auto whitelist; users can
// force rendering via --image=on.
func DetectImageProtocol() ImageProtocol {
	if os.Getenv("TERM") == "xterm-kitty" {
		return ImageKitty
	}
	if _, ok := os.LookupEnv("KITTY_WINDOW_ID"); ok {
		return ImageKitty
	}
	if v, ok := os.LookupEnv("KONSOLE_VERSION"); ok {
		if n, err := strconv.Atoi(v); err == nil && n >= minKonsoleKittyVersion {
			return ImageKitty
		}
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "WezTerm", "kitty", "vscode":
		return ImageKitty
	case "iTerm.app", "mintty", "otty":
		return ImageITerm2
	}
	if strings.EqualFold(os.Getenv("LC_TERMINAL"), "iTerm2") {
		return ImageITerm2
	}
	// Windows Terminal supports the Kitty graphics protocol.
	// It is detected via the WT_SESSION environment variable.
	if os.Getenv("WT_SESSION") != "" {
		return ImageKitty
	}
	return ImageNone
}

// ImageMode represents a tri-state preference for inline image rendering,
// typically wired to a CLI flag like --image=auto|on|off.
type ImageMode int

const (
	// ImageModeAuto honours the detected terminal capability: render images
	// only when DetectImageProtocol reports a supported protocol.
	ImageModeAuto ImageMode = iota
	// ImageModeOn forces image rendering even when detection returned
	// ImageNone (useful for terminals not on our whitelist, or for tmux
	// passthrough setups). When detection has no protocol, iTerm2 is used as
	// the most widely compatible default.
	ImageModeOn
	// ImageModeOff disables image rendering unconditionally.
	ImageModeOff
)

// ParseImageMode parses a CLI/config string into an ImageMode. Recognised
// values are "auto", "on" (synonyms: "yes", "true", "1"), and "off"
// (synonyms: "no", "false", "0"). Unknown values fall back to ImageModeAuto
// so that misconfiguration never silently disables a feature the user wanted.
func ParseImageMode(s string) ImageMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ImageModeAuto
	case "on", "yes", "true", "1":
		return ImageModeOn
	case "off", "no", "false", "0":
		return ImageModeOff
	default:
		return ImageModeAuto
	}
}

// ResolveImage parses a user-supplied mode string and returns the inline
// image protocol the caller should use.
//
//   - off: always returns ImageNone (environment probe is skipped).
//   - on:  returns the detected protocol, or ImageITerm2 when nothing was
//     detected (most widely compatible default for tmux passthrough and
//     terminals not on our whitelist).
//   - auto (default): returns the detected protocol as-is.
func ResolveImage(mode string) ImageProtocol {
	m := ParseImageMode(mode)
	if m == ImageModeOff {
		return ImageNone
	}
	detected := DetectImageProtocol()
	if m == ImageModeOn && detected == ImageNone {
		return ImageITerm2
	}
	return detected
}
