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
	"github.com/clipperhouse/displaywidth"
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
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termHeight <= 0 {
		return false
	}
	if termWidth <= 0 {
		termWidth = 80
	}

	// Count rendered screen lines, accounting for ANSI codes and soft wraps.
	// Reserve 2 lines for status bar and 1 for prompt.
	lineCount := countRenderedLines(content, termWidth)

	// If content fits in terminal (minus status bar and some margin), skip pager
	return lineCount <= termHeight-4
}

func countRenderedLines(content string, width int) int {
	if content == "" {
		return 0
	}
	lineCount := 0
	start := 0
	for {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			break
		}
		lineCount += renderedLineHeight(content[start:start+idx], width)
		start += idx + 1
	}
	// Match previous behavior: trailing newline doesn't add an extra terminal line.
	if start < len(content) {
		lineCount += renderedLineHeight(content[start:], width)
	}
	return lineCount
}

func renderedLineHeight(line string, width int) int {
	if width <= 0 {
		return 1
	}
	if strings.IndexByte(line, '\x1b') >= 0 {
		line = term.StripANSI(line)
	}
	w := displaywidth.String(line)
	if w <= 0 {
		return 1
	}
	return (w + width - 1) / width
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

	// Content is kept as a single string with line start offsets to avoid
	// repeatedly joining line slices on every render.
	content    string
	lineStarts []int

	// Cached status style to avoid rebuilding it every frame.
	statusStyle lipgloss.Style
	statusWidth int
}

// newPagerModel creates a new pager model with the given content.
func newPagerModel(content string, colorMode term.Level, useAltScreen bool) *pagerModel {
	trimmed := strings.TrimSuffix(content, "\n")
	return &pagerModel{
		colorMode:    colorMode,
		useAltScreen: useAltScreen,
		height:       20, // default height, will be updated
		width:        80, // default width, will be updated
		content:      trimmed,
		lineStarts:   buildLineStarts(trimmed),
	}
}

// Init initializes the pager model.
func (m *pagerModel) Init() tea.Cmd {
	return nil
}

// maxScroll returns the maximum scroll position.
func (m *pagerModel) maxScroll() int {
	return max(0, m.lineCount()-m.height)
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
		case "f", "ctrl+f", "pagedown", "space":
			// Full page down (space is standard in less and other pagers)
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
		m.height = msg.Height - 1 // leave space for status bar
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
	totalLines := m.lineCount()
	if totalLines == 0 {
		return ""
	}

	// Calculate visible line range
	topLine := m.scrollPos + 1
	bottomLine := min(totalLines, m.scrollPos+m.height)

	// Calculate percentage based on bottom line position (more intuitive)
	// This shows how much of the content has been viewed
	var percentage int
	if totalLines > 0 {
		percentage = min(bottomLine*100/totalLines, 100)
	}

	// Style status bar with gray background and light gray foreground
	// Build status text - show the visible line range
	statusText := fmt.Sprintf("Lines: %d-%d/%d (%d%%) | ↑/k up | ↓/j/enter down | g/home top | G/end bottom | space/f page down | b page up | q quit",
		topLine, bottomLine, totalLines, percentage)

	return m.getStatusStyle().Render(statusText)
}

// getVisibleContent returns the content that should be visible, preserving ANSI codes
func (m *pagerModel) getVisibleContent() string {
	totalLines := m.lineCount()
	if totalLines == 0 {
		return ""
	}

	start := max(0, m.scrollPos)
	end := min(totalLines, start+m.height)
	if start >= end {
		return ""
	}
	startByte := m.lineStarts[start]
	endByte := len(m.content)
	if end < totalLines {
		endByte = m.lineStarts[end] - 1 // Exclude trailing newline after the last visible line.
	}
	if endByte < startByte {
		return ""
	}
	return m.content[startByte:endByte]
}

func (m *pagerModel) lineCount() int {
	return len(m.lineStarts)
}

func (m *pagerModel) getStatusStyle() lipgloss.Style {
	if m.statusWidth != m.width {
		m.statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("235")).
			Padding(0, 1).
			Width(m.width)
		m.statusWidth = m.width
	}
	return m.statusStyle
}

func buildLineStarts(content string) []int {
	// Match strings.Split("", "\n") behavior used previously.
	if content == "" {
		return []int{0}
	}
	starts := make([]int, 1, strings.Count(content, "\n")+1)
	starts[0] = 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' && i+1 < len(content) {
			starts = append(starts, i+1)
		}
	}
	return starts
}
