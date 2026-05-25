package patchview

import (
	"fmt"
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

const lineNumPadding = 1

// LineStyle defines the style for a single line.
type LineStyle struct {
	LineNumber lipgloss.Style // Line number style
	Code       lipgloss.Style // Code content style
}

// DiffViewStyle defines the complete style for DiffView.
type DiffViewStyle struct {
	DividerLine LineStyle      // Hunk divider line style (@@ -1,3 +1,4 @@)
	MissingLine LineStyle      // Missing line style (used in Split view)
	EqualLine   LineStyle      // Unchanged line style
	InsertLine  LineStyle      // Inserted line style
	DeleteLine  LineStyle      // Deleted line style
	FileName    lipgloss.Style // File name style
	FileMeta    lipgloss.Style // File metadata style
}

// PatchViewStyle defines the visual style for the patch view.
type PatchViewStyle struct {
	// File list styles
	Addition lipgloss.Style
	Deletion lipgloss.Style
	Selected lipgloss.Style

	// Diff view styles (using LineStyle for background fill)
	DiffStyle DiffViewStyle

	// UI styles
	HeaderBg    lipgloss.Style
	FileCount   lipgloss.Style
	Separator   lipgloss.Style
	PathDisplay lipgloss.Style
	FilesTitle  lipgloss.Style
	FooterBg    lipgloss.Style

	// Pane border colors. BorderActive is used for the focused pane,
	// BorderInactive for the other pane.
	BorderActive   color.Color
	BorderInactive color.Color

	// Status styles for header
	StatusAdded    lipgloss.Style
	StatusDeleted  lipgloss.Style
	StatusRenamed  lipgloss.Style
	StatusModified lipgloss.Style
}

// DefaultDarkDiffViewStyle returns the dark theme style.
func DefaultDarkDiffViewStyle() DiffViewStyle {
	setPadding := func(s lipgloss.Style) lipgloss.Style {
		return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
	}

	return DiffViewStyle{
		DividerLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.BBQ)),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.BBQ),
		},
		MissingLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Background(charmtone.BBQ)),
			Code: lipgloss.NewStyle().
				Background(charmtone.BBQ),
		},
		EqualLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(charmtone.Squid).
				Background(charmtone.Pepper)),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Squid).
				Background(charmtone.Pepper),
		},
		InsertLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#629657")).
				Background(lipgloss.Color("#2b322a"))),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#323931")),
		},
		DeleteLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a45c59")).
				Background(lipgloss.Color("#312929"))),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#383030")),
		},
		FileName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#79B8FF")),
		FileMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#959DA5")),
	}
}

// DefaultLightDiffViewStyle returns the light theme style.
// Color scheme based on One Light Pro (clear, bright, moderate contrast).
func DefaultLightDiffViewStyle() DiffViewStyle {
	setPadding := func(s lipgloss.Style) lipgloss.Style {
		return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
	}

	return DiffViewStyle{
		DividerLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#696C77")).
				Background(lipgloss.Color("#E5E5E6"))),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#696C77")).
				Background(lipgloss.Color("#E5E5E6")),
		},
		MissingLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Background(lipgloss.Color("#F0F0F0"))),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#F5F5F5")),
		},
		EqualLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9D9D9F")).
				Background(lipgloss.Color("#F0F0F0"))),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#383A42")).
				Background(lipgloss.Color("#F5F5F5")),
		},
		InsertLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50A14F")).
				Background(lipgloss.Color("#E0F0E0"))),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#383A42")).
				Background(lipgloss.Color("#D4EDD4")),
		},
		DeleteLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E45649")).
				Background(lipgloss.Color("#FAE8E6"))),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#383A42")).
				Background(lipgloss.Color("#F5D4D1")),
		},
		FileName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4078F2")),
		FileMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#696C77")),
	}
}

// DefaultDiffViewStyle returns a diff view style for the given theme.
func DefaultDiffViewStyle(isDark bool) DiffViewStyle {
	if isDark {
		return DefaultDarkDiffViewStyle()
	}
	return DefaultLightDiffViewStyle()
}

// hasDarkBackground detects terminal background color.
// This performs terminal IO and should only be called when a real TTY is
// available (e.g. inside Run). Other call sites should pass the desired
// theme explicitly or accept the default (dark).
func hasDarkBackground() bool {
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}

// DefaultStyle returns the dark theme by default. It does NOT probe the
// terminal so it is safe to call from tests and library init paths. To pick
// a theme based on the actual terminal background, use DefaultStyleFor with
// the result of probing the terminal (e.g. inside Run).
func DefaultStyle() PatchViewStyle {
	return DefaultDarkStyle()
}

// DefaultStyleFor returns the default style for the requested theme.
func DefaultStyleFor(isDark bool) PatchViewStyle {
	if isDark {
		return DefaultDarkStyle()
	}
	return DefaultLightStyle()
}

// DefaultDarkStyle returns the dark theme style.
func DefaultDarkStyle() PatchViewStyle {
	return PatchViewStyle{
		Addition: lipgloss.NewStyle().Foreground(lipgloss.Color("#85E89D")),
		Deletion: lipgloss.NewStyle().Foreground(lipgloss.Color("#F97583")),
		Selected: lipgloss.NewStyle().Background(lipgloss.Color("#282a38")),

		DiffStyle: DefaultDarkDiffViewStyle(),

		HeaderBg:       lipgloss.NewStyle(),
		FileCount:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Separator:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		PathDisplay:    lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true),
		FilesTitle:     lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		FooterBg:       lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1),
		BorderActive:   lipgloss.Color("12"),
		BorderInactive: lipgloss.Color("8"),
		StatusAdded:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		StatusDeleted:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		StatusRenamed:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		StatusModified: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
	}
}

// DefaultLightStyle returns the light theme style.
func DefaultLightStyle() PatchViewStyle {
	return PatchViewStyle{
		Addition: lipgloss.NewStyle().Foreground(lipgloss.Color("#22863A")),
		Deletion: lipgloss.NewStyle().Foreground(lipgloss.Color("#CB2431")),
		Selected: lipgloss.NewStyle().Background(lipgloss.Color("#ebf1fc")),

		DiffStyle: DefaultLightDiffViewStyle(),

		HeaderBg:       lipgloss.NewStyle(),
		FileCount:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Separator:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		PathDisplay:    lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Bold(true),
		FilesTitle:     lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true),
		FooterBg:       lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Padding(0, 1),
		BorderActive:   lipgloss.Color("4"),
		BorderInactive: lipgloss.Color("8"),
		StatusAdded:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		StatusDeleted:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		StatusRenamed:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		StatusModified: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
	}
}

// extractBgColor extracts background color hex value from lipgloss.Style.
func extractBgColor(s lipgloss.Style) string {
	bg := s.GetBackground()
	if bg == nil {
		return ""
	}
	r, g, b, a := bg.RGBA()
	if a == 0 {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
