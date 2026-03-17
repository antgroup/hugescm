package command

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/antgroup/hugescm/cmd/hot/pkg/refs"
	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/fnmatch"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/trace"
)

func newModel(references *refs.References) model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = 20
	p.ActiveDot = lipgloss.NewStyle().Foreground(compat.AdaptiveColor{Light: lipgloss.Color("235"), Dark: lipgloss.Color("252")}).Render("•")
	p.InactiveDot = lipgloss.NewStyle().Foreground(compat.AdaptiveColor{Light: lipgloss.Color("250"), Dark: lipgloss.Color("238")}).Render("•")
	p.SetTotalPages(len(references.Items))

	return model{
		paginator:  p,
		references: references,
	}
}

type model struct {
	references *refs.References
	paginator  paginator.Model
	table      table.Model
	ready      bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "h", "left":
			// Previous page
			if m.paginator.Page > 0 {
				m.paginator.PrevPage()
				m.ready = false
			}
		case "l", "right":
			// Next page
			if m.paginator.Page < m.paginator.TotalPages-1 {
				m.paginator.NextPage()
				m.ready = false
			}
		}
	}

	// Update table
	if m.ready {
		m.table, cmd = m.table.Update(msg)
	}

	// Build table on first render or page change
	if !m.ready {
		m.table = m.buildTable()
		m.ready = true
	}

	return m, cmd
}

func (m model) buildTable() table.Model {
	start, end := m.paginator.GetSliceBounds(len(m.references.Items))

	// Build table columns with proper widths
	termWidth := getTerminalWidth()
	colWidths := struct {
		hash    int
		date    int
		name    int
		leading int
		lagging int
	}{
		hash:    40, // Full commit hash
		date:    25,
		leading: 8,
		lagging: 8,
	}
	// Width calculation:
	// terminal = table + lipgloss borders(2)
	// table = sum(colWidths) + padding + separators
	// For 5 columns with padding=1: sum(col) + 2*5 + 4 = sum(col) + 14
	fixedWidth := colWidths.hash + colWidths.date + colWidths.leading + colWidths.lagging + 16 // 16 = padding + separators + lipgloss borders
	colWidths.name = max(30, min(80, termWidth-fixedWidth))

	columns := []table.Column{
		{Title: tr.W("Hash"), Width: colWidths.hash},
		{Title: tr.W("Date"), Width: colWidths.date},
		{Title: tr.W("Reference Name"), Width: colWidths.name},
		{Title: tr.W("Leading"), Width: colWidths.leading},
		{Title: tr.W("Lagging"), Width: colWidths.lagging},
	}

	// Build table rows
	rows := make([]table.Row, 0, end-start)
	for _, item := range m.references.Items[start:end] {
		if item.Broken {
			rows = append(rows, table.Row{item.Hash, "", item.Name, tr.W("reference is broken"), ""})
			continue
		}
		date := item.Committer.When.Local().Format(time.RFC3339)
		if item.Name == m.references.Current || !item.IsBranch() {
			rows = append(rows, table.Row{item.Hash, date, item.ShortName, "", ""})
			continue
		}
		if item.Leading == 0 {
			rows = append(rows, table.Row{item.Hash, date, item.ShortName, "*merged", strconv.Itoa(item.Lagging)})
			continue
		}
		rows = append(rows, table.Row{item.Hash, date, item.ShortName, strconv.Itoa(item.Leading), strconv.Itoa(item.Lagging)})
	}

	// Create table
	// Total width must not exceed terminal width - lipgloss borders (2)
	totalWidth := termWidth - 2
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(20, len(rows)+2)),
		table.WithWidth(totalWidth),
	)

	// Apply styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("243")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("173")).
		Padding(0, 1)
	s.Cell = s.Cell.Padding(0, 1)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return t
}

func (m model) View() tea.View {
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s\x1b[38;2;32;225;215m%d\x1b[0m\n\n", tr.W("Matched references: "), len(m.references.Items))

	if m.ready {
		// Wrap table with lipgloss to add complete borders
		tableStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("243"))
		b.WriteString(tableStyle.Render(m.table.View()))
		b.WriteString("\n\n")
		b.WriteString("  " + m.paginator.View())
		b.WriteString("\n\n  ↑/k ↓/j: navigate • h/l ←/→: page • q: quit\n")
	}

	return tea.NewView(b.String())
}

// getTerminalWidth returns the terminal width with a default fallback
func getTerminalWidth() int {
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	return 80
}

type ScanRefs struct {
	Pattern []string `arg:"" optional:"" name:"pattern" help:"Matching pattern, all references are displayed by default"`
	CWD     string   `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Oldest  bool     `short:"O" name:"oldest" help:"Sort by time from oldest to newest"`
}

func (c *ScanRefs) fixup() {
	for i, pattern := range c.Pattern {
		if strings.HasSuffix(pattern, "/") {
			c.Pattern[i] = pattern + "*"
		}
	}
}

func (c *ScanRefs) Match(name string) bool {
	if len(c.Pattern) == 0 {
		return true
	}
	for _, pattern := range c.Pattern {
		if fnmatch.Match(pattern, name, 0) {
			return true
		}
	}
	return false
}

func (c *ScanRefs) Run(g *Globals) error {
	c.fixup()
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)
	order := git.OrderNewest
	if c.Oldest {
		order = git.OrderOldest
	}
	references, err := refs.ScanReferences(context.Background(), repoPath, c, order)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan references error: %v\n", err)
		return err
	}
	if len(references.Items) == 0 {
		return nil
	}
	p := tea.NewProgram(newModel(references))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "show references error: %v\n", err)
		return err
	}
	return nil
}
