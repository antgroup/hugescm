package stat

import (
	"fmt"
	"os"
	"path"
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
	sort.Sort(Items(items))

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
			displayPath = truncateName(item.Path, pathWidth)
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
		fmt.Println()
		fmt.Println(titleStyle.Render(title))
		fmt.Println()
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

// truncateName safely truncates a path string while preserving path structure
func truncateName(s string, maxWidth int) string {
	if maxWidth <= 0 || displaywidth.String(s) <= maxWidth {
		return s
	}

	vv := strengthen.SplitPath(s)
	var lastPart string
	if len(vv) > 1 {
		// Try to preserve path segments from the end
		for i := 1; i <= len(vv); i++ {
			if ss := ".../" + path.Join(vv[len(vv)-i:]...); displaywidth.String(ss) <= maxWidth {
				return ss
			}
		}
		lastPart = vv[len(vv)-1]
	} else {
		lastPart = s
	}

	// Truncate last part with ellipsis
	if maxWidth <= 3 {
		return "..."[:maxWidth]
	}
	return displaywidth.TruncateString(lastPart, maxWidth, "...")
}
