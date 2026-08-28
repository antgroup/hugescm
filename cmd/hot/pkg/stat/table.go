package stat

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/clipperhouse/displaywidth"
	"golang.org/x/term"
)

// drawInteractive renders the table statically (no interaction needed)
func (s *summer) drawInteractive(title string) error {
	if len(s.files) == 0 {
		return nil
	}

	// Build and sort items
	items := make(Items, 0, len(s.files))
	for n, i := range s.files {
		items = append(items, Item{Path: n, Total: i.sum, Count: i.count})
	}
	sort.Sort(items)

	// Get terminal width
	termWidth := getTerminalWidth()

	// Calculate path column width dynamically
	// Formula: termWidth - (# col) - (count col) - (size col) - borders - padding
	// # col: ~6 chars, count col: ~12 chars, size col: ~14 chars, borders: 8, padding: 8
	fixedWidth := 6 + 12 + 14 + 8 + 8
	pathWidth := min(max(termWidth-fixedWidth, 20), 100)

	// Build rows (including total row)
	rows := make([][]string, 0, len(items)+1)
	for i, item := range items {
		displayPath := item.Path
		if !s.fullPath {
			displayPath = truncatePath(item.Path, pathWidth)
		}
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			displayPath,
			strconv.Itoa(item.Count),
			strengthen.FormatSize(item.Total),
		})
	}

	// Add total row (bold)
	totalRow := []string{
		strings.ToUpper(tr.W("total")),
		"",
		strconv.Itoa(s.count),
		strengthen.FormatSize(s.total),
	}
	rows = append(rows, totalRow)

	// Color scheme optimized for file size statistics
	// Using warm, attention-grabbing colors while maintaining readability
	headerColor := lipgloss.Color("173") // Warm coral/salmon - stands out but not harsh
	totalColor := lipgloss.Color("215")  // Warm gold/amber - indicates summary/importance
	borderColor := lipgloss.Color("243") // Medium gray - visible but not distracting

	// Create table with warm color scheme
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(borderColor)).
		Headers("#", tr.W("Path"), tr.W("Modifications"), tr.W("Cumulative Size")).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				// Header: warm coral for clear structure
				return lipgloss.NewStyle().
					Foreground(headerColor).
					Bold(true).
					Padding(0, 1)
			case row == len(items):
				// Total row: warm gold to highlight summary
				return lipgloss.NewStyle().
					Foreground(totalColor).
					Bold(true).
					Padding(0, 1)
			default:
				// Regular rows: default terminal color
				return lipgloss.NewStyle().
					Padding(0, 1)
			}
		})

	// Print title with proper spacing
	if title != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))
		fmt.Printf("\n%s\n\n", titleStyle.Render(title))
	}

	// Print table
	fmt.Println(t)

	return nil
}

// getTerminalWidth returns the terminal width, with a sensible default
func getTerminalWidth() int {
	// Try to get terminal width
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	// Default to 80 if we can't detect
	return 80
}

// truncatePath truncates a path from the left to fit within maxWidth using
// grapheme cluster boundaries (per UAX #29, the segmenting model used by
// DECSET mode 2027 terminals). Operates on grapheme clusters rather than
// runes so multi-codepoint graphemes — ZWJ family emoji, regional indicator
// (flag) pairs, keycap sequences, combining-mark clusters — are never split
// at a rune boundary, which would otherwise produce a broken surrogate
// sequence plus a width mismatch against grapheme-aware terminal renderers.
func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if displaywidth.String(path) <= maxWidth {
		return path
	}
	if maxWidth == 1 {
		return "…"
	}

	target := maxWidth - 1

	// StringGraphemes is a forward-only iterator; collect the clusters with
	// their widths first so we can walk backward.
	type grapheme struct {
		value string
		width int
	}
	var graphemes []grapheme
	g := displaywidth.StringGraphemes(path)
	for g.Next() {
		graphemes = append(graphemes, grapheme{value: g.Value(), width: g.Width()})
	}

	width := 0
	cut := len(graphemes)
	for i, grapheme := range slices.Backward(graphemes) {
		if width+grapheme.width > target {
			break
		}
		width += grapheme.width
		cut = i
	}

	var sb strings.Builder
	sb.Grow(len(path) + 3)
	sb.WriteString("…")
	for _, gm := range graphemes[cut:] {
		sb.WriteString(gm.value)
	}
	return sb.String()
}
