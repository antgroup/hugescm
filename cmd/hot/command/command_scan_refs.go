package command

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/cmd/hot/pkg/refs"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/fnmatch"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
)

func newModel(references *refs.References) model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = 20
	p.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
	p.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
	p.SetTotalPages(len(references.Items))

	return model{
		paginator:  p,
		references: references,
	}
}

type model struct {
	references *refs.References
	paginator  paginator.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	m.paginator, cmd = m.paginator.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s\x1b[38;2;32;225;215m%d\x1b[0m\n\n", tr.W("Matched references: "), len(m.references.Items))
	start, end := m.paginator.GetSliceBounds(len(m.references.Items))
	t := table.NewWriter()
	t.SetStyle(table.StyleColoredBlackOnCyanWhite)
	t.AppendHeader(table.Row{tr.W("Hash"), tr.W("Date"), tr.W("Reference Name"), tr.W("Leading"), tr.W("Lagging")})
	for _, item := range m.references.Items[start:end] {
		if item.Broken {
			t.AppendRow(table.Row{item.Hash, "", item.Name, tr.W("reference is broken")})
			continue
		}
		date := item.Committer.When.Local().Format(time.RFC3339)
		if item.Name == m.references.Current || !item.IsBranch() {
			t.AppendRow(table.Row{item.Hash, date, item.ShortName, "", ""})
			continue
		}
		if item.Leading == 0 {
			t.AppendRow(table.Row{item.Hash, date, item.ShortName, "*merged", strconv.Itoa(item.Lagging)})
			continue
		}
		t.AppendRow(table.Row{item.Hash, date, item.ShortName, strconv.Itoa(item.Leading), strconv.Itoa(item.Lagging)})
	}
	b.WriteString(t.Render())
	b.WriteString("\n\n")
	b.WriteString("  " + m.paginator.View())
	b.WriteString("\n\n  h/l ←/→ page • q: quit\n")
	return b.String()
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
