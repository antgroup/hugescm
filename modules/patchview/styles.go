package patchview

import (
	"fmt"
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/antgroup/hugescm/modules/term"
	"github.com/charmbracelet/x/exp/charmtone"
)

const lineNumPadding = 1

// LineStyle defines the style for a single line.
type LineStyle struct {
	LineNumber lipgloss.Style // Line number style
	Symbol     lipgloss.Style // Leading symbol style (+/-)
	Code       lipgloss.Style // Code content style
}

// Style defines the complete visual style for the patch view.
type Style struct {
	// Diff line styles
	DividerLine LineStyle      // Hunk divider line style (@@ -1,3 +1,4 @@)
	MissingLine LineStyle      // Missing line style (used in Split view)
	EqualLine   LineStyle      // Unchanged line style
	InsertLine  LineStyle      // Inserted line style
	DeleteLine  LineStyle      // Deleted line style
	FileName    lipgloss.Style // File name style
	FileMeta    lipgloss.Style // File metadata style

	// File list styles
	Addition lipgloss.Style
	Deletion lipgloss.Style
	Selected lipgloss.Style

	// UI styles
	HeaderBg    lipgloss.Style
	FileCount   lipgloss.Style
	Separator   lipgloss.Style
	PathDisplay lipgloss.Style
	FilesTitle  lipgloss.Style
	FooterBg    lipgloss.Style

	// Border colors for the top header / status bar cards.
	// These are intentionally neutral (no focus highlight) so the cards
	// stay visually stable while the user navigates panes.
	HeaderBorder color.Color

	// Subdued text used inside the top header card (e.g. commit metadata).
	HeaderMeta lipgloss.Style

	// Highlight style for the commit hash shown in the top header.
	HeaderHash lipgloss.Style

	// Status styles for header
	StatusAdded    lipgloss.Style
	StatusDeleted  lipgloss.Style
	StatusRenamed  lipgloss.Style
	StatusModified lipgloss.Style
}

// DefaultStyle returns the default style with auto-detected theme.
func DefaultStyle() Style {
	if term.HasDarkBackground() {
		return DefaultDarkStyle()
	}
	return DefaultLightStyle()
}

// DefaultDarkStyle returns the dark theme style.
func DefaultDarkStyle() Style {
	setPadding := func(s lipgloss.Style) lipgloss.Style {
		return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
	}

	return Style{
		// Diff line styles
		DividerLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.BBQ)),
			Symbol: lipgloss.NewStyle().
				Background(charmtone.BBQ),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.BBQ),
		},
		MissingLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Background(charmtone.BBQ)),
			Symbol: lipgloss.NewStyle().
				Background(charmtone.BBQ),
			Code: lipgloss.NewStyle().
				Background(charmtone.BBQ),
		},
		EqualLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(charmtone.Squid).
				Background(charmtone.Pepper)),
			Symbol: lipgloss.NewStyle().
				Background(charmtone.Pepper),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Squid).
				Background(charmtone.Pepper),
		},
		InsertLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#629657")).
				Background(lipgloss.Color("#2b322a"))),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#629657")).
				Background(lipgloss.Color("#2f362d")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#323931")),
		},
		DeleteLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a45c59")).
				Background(lipgloss.Color("#312929"))),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a45c59")).
				Background(lipgloss.Color("#352d2d")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#383030")),
		},
		FileName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#79B8FF")),
		FileMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#959DA5")),

		// File list styles
		Addition: lipgloss.NewStyle().Foreground(lipgloss.Color("#85E89D")),
		Deletion: lipgloss.NewStyle().Foreground(lipgloss.Color("#F97583")),
		Selected: lipgloss.NewStyle().Background(lipgloss.Color("#282a38")),

		// UI styles
		HeaderBg:       lipgloss.NewStyle(),
		FileCount:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Separator:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		PathDisplay:    lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true),
		FilesTitle:     lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		FooterBg:       lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1),
		HeaderBorder:   lipgloss.Color("8"),
		HeaderMeta:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		HeaderHash:     lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
		StatusAdded:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		StatusDeleted:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		StatusRenamed:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		StatusModified: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
	}
}

// DefaultLightStyle returns the light theme style.
func DefaultLightStyle() Style {
	setPadding := func(s lipgloss.Style) lipgloss.Style {
		return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
	}

	return Style{
		// Diff line styles
		DividerLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#696C77")).
				Background(lipgloss.Color("#E5E5E6"))),
			Symbol: lipgloss.NewStyle().
				Background(lipgloss.Color("#E5E5E6")),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#696C77")).
				Background(lipgloss.Color("#E5E5E6")),
		},
		MissingLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Background(lipgloss.Color("#F0F0F0"))),
			Symbol: lipgloss.NewStyle().
				Background(lipgloss.Color("#F5F5F5")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#F5F5F5")),
		},
		EqualLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9D9D9F")).
				Background(lipgloss.Color("#F0F0F0"))),
			Symbol: lipgloss.NewStyle().
				Background(lipgloss.Color("#F5F5F5")),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#383A42")).
				Background(lipgloss.Color("#F5F5F5")),
		},
		InsertLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50A14F")).
				Background(lipgloss.Color("#E0F0E0"))),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50A14F")).
				Background(lipgloss.Color("#DAF0DA")),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#383A42")).
				Background(lipgloss.Color("#D4EDD4")),
		},
		DeleteLine: LineStyle{
			LineNumber: setPadding(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E45649")).
				Background(lipgloss.Color("#FAE8E6"))),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E45649")).
				Background(lipgloss.Color("#F8DEDA")),
			Code: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#383A42")).
				Background(lipgloss.Color("#F5D4D1")),
		},
		FileName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4078F2")),
		FileMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#696C77")),

		// File list styles
		Addition: lipgloss.NewStyle().Foreground(lipgloss.Color("#22863A")),
		Deletion: lipgloss.NewStyle().Foreground(lipgloss.Color("#CB2431")),
		Selected: lipgloss.NewStyle().Background(lipgloss.Color("#ebf1fc")),

		// UI styles
		HeaderBg:       lipgloss.NewStyle(),
		FileCount:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Separator:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		PathDisplay:    lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Bold(true),
		FilesTitle:     lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true),
		FooterBg:       lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Padding(0, 1),
		HeaderBorder:   lipgloss.Color("7"),
		HeaderMeta:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		HeaderHash:     lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
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
