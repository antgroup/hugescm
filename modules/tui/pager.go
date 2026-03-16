package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/term"
)

// Pager represents a simple terminal pager built with bubbletea
type Pager struct {
	program   *tea.Program
	content   string
	writer    *pagerWriter
	colorMode term.Level
}

// NewPager creates a new pager with given color mode
func NewPager(colorMode term.Level) *Pager {
	p := &Pager{
		colorMode: colorMode,
		writer:    &pagerWriter{},
	}
	return p
}

// Write implements io.Writer interface for the pager
func (p *Pager) Write(data []byte) (int, error) {
	return p.writer.Write(data)
}

// Close starts the pager and waits for it to finish
func (p *Pager) Close() error {
	if p.writer.closed {
		return nil
	}
	p.writer.closed = true
	p.content = p.writer.String()

	// If content is empty, skip starting the pager
	if p.content == "" {
		return nil
	}

	// If no color support, just output directly
	if p.colorMode == term.LevelNone {
		_, err := os.Stdout.Write([]byte(p.content))
		return err
	}

	// Start the pager program with accumulated content
	model := &pagerModel{
		content:        p.content,
		scrollPos:      0,
		colorMode:      p.colorMode,
		ready:          false,
		height:         20,   // default height, will be updated
		width:          80,   // default width, will be updated
		contentChanged: true, // Mark content as changed to trigger caching
	}

	p.program = tea.NewProgram(model, tea.WithOutput(os.Stderr))
	_, err := p.program.Run()
	return err
}

// ColorMode returns the color mode of the pager
func (p *Pager) ColorMode() term.Level {
	return p.colorMode
}

// EnableColor returns whether color is enabled
func (p *Pager) EnableColor() bool {
	return p.colorMode != term.LevelNone
}

// pagerModel is the bubbletea model for the pager
type pagerModel struct {
	content   string
	scrollPos int
	colorMode term.Level
	ready     bool
	height    int
	width     int

	// Cached data for performance
	cachedLines    []string
	contentChanged bool
}

// Init initializes the pager model
func (m *pagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "g":
			m.scrollPos = 0
		case "G":
			m.scrollPos = max(0, len(m.cachedLines)-m.height)
		case "j", "down", "↓", "enter":
			m.scrollPos = min(m.scrollPos+1, max(0, len(m.cachedLines)-m.height))
		case "k", "up", "↑":
			m.scrollPos = max(0, m.scrollPos-1)
		case "d", "ctrl+d":
			// Half page down
			m.scrollPos = min(m.scrollPos+m.height/2, max(0, len(m.cachedLines)-m.height))
		case "u", "ctrl+u":
			// Half page up
			m.scrollPos = max(0, m.scrollPos-m.height/2)
		case "f", "ctrl+f", "pagedown", " ":
			// Full page down
			m.scrollPos = min(m.scrollPos+m.height, max(0, len(m.cachedLines)-m.height))
		case "b", "ctrl+b", "pageup":
			// Full page up
			m.scrollPos = max(0, m.scrollPos-m.height)
		case "home":
			m.scrollPos = 0
		case "end":
			m.scrollPos = max(0, len(m.cachedLines)-m.height)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 2 // leave space for status bar
		m.ready = true
		// Split lines only if content has changed or cache is empty
		if m.contentChanged || len(m.cachedLines) == 0 {
			m.cachedLines = m.splitLinesWithANSI()
			m.contentChanged = false
		}
		m.scrollPos = min(m.scrollPos, max(0, len(m.cachedLines)-m.height))
	}

	return m, nil
}

// View renders the pager UI
func (m *pagerModel) View() tea.View {
	if !m.ready {
		return tea.NewView("Loading...")
	}

	// Show visible lines with proper ANSI handling
	visibleContent := m.getVisibleContent()

	// Render status bar with line numbers and progress
	statusBar := m.renderStatusBar()

	// Use lipgloss.JoinVertical to preserve all whitespace including spaces, tabs, and empty lines
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, visibleContent, statusBar))
}

// renderStatusBar creates a status bar with line numbers and progress percentage
func (m *pagerModel) renderStatusBar() string {
	if len(m.cachedLines) == 0 {
		return ""
	}

	// Calculate visible line range
	totalLines := len(m.cachedLines)
	topLine := m.scrollPos + 1
	bottomLine := min(totalLines, m.scrollPos+m.height)

	// Calculate percentage based on bottom line position (more intuitive)
	// This shows how much of the content has been viewed
	var percentage float64
	if totalLines > 0 {
		percentage = float64(bottomLine) / float64(totalLines) * 100
		// Clamp to 100% to avoid showing >100%
		if percentage > 100 {
			percentage = 100
		}
	} else {
		percentage = 0
	}

	// Style status bar with gray background and light gray foreground
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	// Build status text - show the visible line range
	statusText := fmt.Sprintf("Lines: %d-%d/%d (%.1f%%) | ↑/k up | ↓/j/enter down | g/home top | G/end bottom | f/b page | q quit",
		topLine, bottomLine, totalLines, percentage)

	return statusStyle.Render(statusText)
}

// getVisibleContent returns the content that should be visible, preserving ANSI codes
func (m *pagerModel) getVisibleContent() string {
	if len(m.cachedLines) == 0 {
		return ""
	}

	start := max(0, m.scrollPos)
	end := min(len(m.cachedLines), start+m.height)
	visibleLines := m.cachedLines[start:end]

	return strings.Join(visibleLines, "\n")
}

// splitLinesWithANSI splits content into lines while preserving ANSI escape sequences
// Optimized version with better ANSI sequence handling
func (m *pagerModel) splitLinesWithANSI() []string {
	var lines []string
	var currentLine strings.Builder
	inANSI := false
	ansiBuffer := strings.Builder{}

	for _, r := range m.content {
		if r == '\x1b' {
			// Start of ANSI escape sequence
			ansiBuffer.WriteRune(r)
			inANSI = true
			continue
		}

		if inANSI {
			ansiBuffer.WriteRune(r)
			// ANSI CSI sequences (Control Sequence Introducer): ESC[... + terminator
			// Terminators: letters (a-z, A-Z) or some special characters
			// ANSI OSC sequences: ESC]... followed by BEL (\a) or ST (ESC\)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '\a' {
				// End of ANSI sequence
				currentLine.WriteString(ansiBuffer.String())
				ansiBuffer.Reset()
				inANSI = false
			}
			continue
		}

		currentLine.WriteRune(r)

		if r == '\n' {
			lines = append(lines, strings.TrimSuffix(currentLine.String(), "\n"))
			currentLine.Reset()
		}
	}

	// Add the last line if it exists
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// pagerWriter is a writer that accumulates content before starting the pager
type pagerWriter struct {
	bytes.Buffer
	closed bool
}

// Write appends data to the writer
func (w *pagerWriter) Write(data []byte) (int, error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(data)
}
