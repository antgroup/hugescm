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

// Compile-time interface assertion
var _ io.WriteCloser = &Pager{}

// Pager represents a simple terminal pager built with bubbletea.
// It implements io.Writer and must be closed to display content.
type Pager struct {
	buf          bytes.Buffer
	closed       bool
	colorMode    term.Level
	useAltScreen bool
}

// NewPager creates a new pager with given color mode and alt screen setting.
// The pager implements io.Writer, content is accumulated via Write calls.
// Close must be called to display the content in the pager.
// useAltScreen controls whether to use alternate screen buffer (default true).
func NewPager(colorMode term.Level, useAltScreen bool) *Pager {
	return &Pager{
		colorMode:    colorMode,
		useAltScreen: useAltScreen,
	}
}

// Write implements io.Writer interface for the pager.
// It appends data to the internal buffer and returns an error if the pager is closed.
func (p *Pager) Write(data []byte) (int, error) {
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	return p.buf.Write(data)
}

// Close finalizes the pager and displays the content.
// For short content that fits in the terminal, it outputs directly without starting the pager.
// For longer content, it starts an interactive pager with bubbletea.
// Close is idempotent - calling it multiple times is safe.
func (p *Pager) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true

	content := p.buf.String()
	if content == "" {
		return nil
	}

	// If color is disabled (e.g. NO_COLOR or non-interactive terminal),
	// we also disable the pager and print directly.
	if p.colorMode == term.LevelNone {
		_, err := io.WriteString(os.Stdout, content)
		return err
	}

	// If content fits in one screen, output directly without starting pager
	if p.shouldSkipPager(content) {
		_, err := io.WriteString(os.Stdout, content)
		return err
	}

	return p.run(content)
}

// shouldSkipPager checks if the content is short enough to display without a pager.
func (p *Pager) shouldSkipPager(content string) bool {
	// Get terminal height
	_, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termHeight <= 0 {
		return false
	}

	// Count lines in content
	// Reserve 2 lines for status bar and 1 for prompt
	lineCount := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") && content != "" {
		lineCount++ // Count the last line if it doesn't end with newline
	}

	// If content fits in terminal (minus status bar and some margin), skip pager
	return lineCount <= termHeight-4
}

// run starts the interactive pager with the given content.
func (p *Pager) run(content string) error {
	model := newPagerModel(content, p.colorMode, p.useAltScreen)
	program := tea.NewProgram(model, tea.WithOutput(os.Stderr))
	_, err := program.Run()
	return err
}

// ColorMode returns the color mode of the pager.
func (p *Pager) ColorMode() term.Level {
	return p.colorMode
}

// pagerModel is the bubbletea model for the pager
type pagerModel struct {
	scrollPos    int
	colorMode    term.Level
	useAltScreen bool
	ready        bool
	height       int
	width        int

	// Cached data for performance
	lines []string
}

// newPagerModel creates a new pager model with the given content.
func newPagerModel(content string, colorMode term.Level, useAltScreen bool) *pagerModel {
	return &pagerModel{
		colorMode:    colorMode,
		useAltScreen: useAltScreen,
		height:       20, // default height, will be updated
		width:        80, // default width, will be updated
		lines:        strings.Split(strings.TrimSuffix(content, "\n"), "\n"),
	}
}

// Init initializes the pager model.
func (m *pagerModel) Init() tea.Cmd {
	return nil
}

// maxScroll returns the maximum scroll position.
func (m *pagerModel) maxScroll() int {
	return max(0, len(m.lines)-m.height)
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
			m.scrollPos = m.maxScroll()
		case "j", "down", "↓", "enter":
			m.scrollPos = min(m.scrollPos+1, m.maxScroll())
		case "k", "up", "↑":
			m.scrollPos = max(0, m.scrollPos-1)
		case "d", "ctrl+d":
			// Half page down
			m.scrollPos = min(m.scrollPos+m.height/2, m.maxScroll())
		case "u", "ctrl+u":
			// Half page up
			m.scrollPos = max(0, m.scrollPos-m.height/2)
		case "f", "ctrl+f", "pagedown", " ":
			// Full page down
			m.scrollPos = min(m.scrollPos+m.height, m.maxScroll())
		case "b", "ctrl+b", "pageup":
			// Full page up
			m.scrollPos = max(0, m.scrollPos-m.height)
		case "home":
			m.scrollPos = 0
		case "end":
			m.scrollPos = m.maxScroll()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 2 // leave space for status bar
		m.ready = true
		m.scrollPos = min(m.scrollPos, m.maxScroll())
	}

	return m, nil
}

// View renders the pager UI
func (m *pagerModel) View() tea.View {
	if !m.ready {
		return tea.NewView("Loading...")
	}

	// Show visible lines
	visibleContent := m.getVisibleContent()

	// Render status bar with line numbers and progress
	statusBar := m.renderStatusBar()

	// Use lipgloss.JoinVertical to preserve all whitespace including spaces, tabs, and empty lines
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, visibleContent, statusBar))
	v.AltScreen = m.useAltScreen // Use alternate screen buffer for clean exit
	return v
}

// renderStatusBar creates a status bar with line numbers and progress percentage
func (m *pagerModel) renderStatusBar() string {
	if len(m.lines) == 0 {
		return ""
	}

	// Calculate visible line range
	totalLines := len(m.lines)
	topLine := m.scrollPos + 1
	bottomLine := min(totalLines, m.scrollPos+m.height)

	// Calculate percentage based on bottom line position (more intuitive)
	// This shows how much of the content has been viewed
	var percentage int
	if totalLines > 0 {
		percentage = bottomLine * 100 / totalLines
		if percentage > 100 {
			percentage = 100
		}
	}

	// Style status bar with gray background and light gray foreground
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	// Build status text - show the visible line range
	statusText := fmt.Sprintf("Lines: %d-%d/%d (%d%%) | ↑/k up | ↓/j/enter down | g/home top | G/end bottom | f/b page | q quit",
		topLine, bottomLine, totalLines, percentage)

	return statusStyle.Render(statusText)
}

// getVisibleContent returns the content that should be visible, preserving ANSI codes
func (m *pagerModel) getVisibleContent() string {
	if len(m.lines) == 0 {
		return ""
	}

	start := max(0, m.scrollPos)
	end := min(len(m.lines), start+m.height)
	visibleLines := m.lines[start:end]

	return strings.Join(visibleLines, "\n")
}
