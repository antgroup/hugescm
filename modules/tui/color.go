package tui

import (
	"maps"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/diferenco/color"
	"github.com/antgroup/hugescm/modules/term"
)

// DiffTheme defines color scheme for diff output.
type DiffTheme struct {
	Dark  map[color.ColorKey]string
	Light map[color.ColorKey]string
}

// Predefined diff themes.
var (
	// GitHub theme (default).
	GitHub = DiffTheme{
		Dark: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;248;81;73m",  // #f85149 red
			color.New:    "\x1b[38;2;63;185;80m",  // #3fb950 green
			color.Frag:   "\x1b[38;2;88;166;255m", // #58a6ff blue
			color.Commit: "\x1b[38;2;210;153;34m", // #d29922 yellow
		},
		Light: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;215;58;73m", // #d73a49 red
			color.New:    "\x1b[38;2;40;167;69m", // #28a745 green
			color.Frag:   "\x1b[38;2;0;92;197m",  // #005cc5 blue
			color.Commit: "\x1b[38;2;176;136;0m", // #b08800 yellow
		},
	}

	// Dracula theme.
	Dracula = DiffTheme{
		Dark: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;255;85;85m",   // #ff5555 red
			color.New:    "\x1b[38;2;80;250;123m",  // #50fa7b green
			color.Frag:   "\x1b[38;2;139;233;253m", // #8be9fd cyan
			color.Commit: "\x1b[38;2;241;250;140m", // #f1fa8c yellow
		},
		Light: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;215;58;73m", // same as GitHub light
			color.New:    "\x1b[38;2;40;167;69m",
			color.Frag:   "\x1b[38;2;0;92;197m",
			color.Commit: "\x1b[38;2;176;136;0m",
		},
	}

	// OneDark theme.
	OneDark = DiffTheme{
		Dark: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;224;108;117m", // #e06c75 red
			color.New:    "\x1b[38;2;152;195;121m", // #98c379 green
			color.Frag:   "\x1b[38;2;97;175;239m",  // #61afef blue
			color.Commit: "\x1b[38;2;209;154;102m", // #d19a66 orange
		},
		Light: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;228;86;73m",  // #e45649 red
			color.New:    "\x1b[38;2;80;161;79m",  // #50a14f green
			color.Frag:   "\x1b[38;2;56;125;203m", // #387dcb blue
			color.Commit: "\x1b[38;2;188;122;0m",  // #bc7a00 orange
		},
	}

	// Catppuccin theme.
	Catppuccin = DiffTheme{
		Dark: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;243;139;168m", // #f38ba8 red
			color.New:    "\x1b[38;2;166;227;161m", // #a6e3a1 green
			color.Frag:   "\x1b[38;2;137;180;250m", // #89b4fa blue
			color.Commit: "\x1b[38;2;249;226;175m", // #f9e2af yellow
		},
		Light: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;210;15;57m",  // #d20f39 red
			color.New:    "\x1b[38;2;64;160;43m",  // #40a02b green
			color.Frag:   "\x1b[38;2;30;102;245m", // #1e66f5 blue
			color.Commit: "\x1b[38;2;223;142;29m", // #df8e1d yellow
		},
	}

	// Nord theme.
	Nord = DiffTheme{
		Dark: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;191;97;106m",  // #bf616a red
			color.New:    "\x1b[38;2;163;190;140m", // #a3be8c green
			color.Frag:   "\x1b[38;2;136;192;208m", // #88c0d0 cyan
			color.Commit: "\x1b[38;2;235;203;139m", // #ebcb8b yellow
		},
		Light: map[color.ColorKey]string{
			color.Old:    "\x1b[38;2;191;97;106m", // same as dark
			color.New:    "\x1b[38;2;163;190;140m",
			color.Frag:   "\x1b[38;2;136;192;208m",
			color.Commit: "\x1b[38;2;235;203;139m",
		},
	}

	// Current theme (can be changed).
	currentTheme = Dracula
)

// SetDiffTheme sets the current diff theme.
func SetDiffTheme(theme DiffTheme) {
	currentTheme = theme
}

// EncoderOptions returns diferenco.EncoderOption slice with appropriate color
// configuration based on the terminal's color level.
func EncoderOptions(level term.Level) []diferenco.EncoderOption {
	cc := color.ColorConfig{
		color.Context:                   color.Normal,
		color.Meta:                      color.Bold,
		color.Whitespace:                color.BgRed,
		color.Func:                      color.Normal,
		color.OldMoved:                  color.BoldMagenta,
		color.OldMovedAlternative:       color.BoldBlue,
		color.OldMovedDimmed:            color.Faint,
		color.OldMovedAlternativeDimmed: color.FaintItalic,
		color.NewMoved:                  color.BoldCyan,
		color.NewMovedAlternative:       color.BoldYellow,
		color.NewMovedDimmed:            color.Faint,
		color.NewMovedAlternativeDimmed: color.FaintItalic,
		color.ContextDimmed:             color.Faint,
		color.OldDimmed:                 color.FaintRed,
		color.NewDimmed:                 color.FaintGreen,
		color.ContextBold:               color.Bold,
		color.OldBold:                   color.BoldRed,
		color.NewBold:                   color.BoldGreen,
	}

	switch level {
	case term.Level16M:
		// Use truecolor with current theme based on background
		theme := currentTheme.Dark
		if !lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
			theme = currentTheme.Light
		}
		maps.Copy(cc, theme)
	case term.Level256:
		cc[color.Old] = color.Red
		cc[color.New] = color.Green
		cc[color.Frag] = color.Cyan
		cc[color.Commit] = color.Yellow
	default:
		return nil
	}

	return []diferenco.EncoderOption{diferenco.WithColor(cc)}
}
