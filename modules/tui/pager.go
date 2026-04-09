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
	"github.com/charmbracelet/x/ansi"
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
// It uses early termination to avoid counting all lines when the content is long.
func (p *Pager) shouldSkipPager(content string) bool {
	// Get terminal height
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termHeight <= 0 {
		return false
	}
	if termWidth <= 0 {
		termWidth = 80
	}

	// Maximum lines that fit in terminal (minus status bar and margin)
	maxLines := termHeight - 4
	if maxLines <= 0 {
		return false
	}

	// Count rendered screen lines with early termination
	// Stop counting once we exceed the maximum to avoid unnecessary work
	lineCount := countRenderedLinesLimit(content, termWidth, maxLines+1)

	// If content fits in terminal, skip pager
	return lineCount <= maxLines
}

// countRenderedLinesLimit counts rendered lines but stops early if maxLines is exceeded.
// This is more efficient than counting all lines when we just need to know if content exceeds a threshold.
func countRenderedLinesLimit(content string, width int, maxLines int) int {
	if content == "" {
		return 0
	}

	lineCount := 0
	lineStart := 0

	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			// Calculate height for this line without creating a substring
			lineCount += renderedLineHeight(content[lineStart:i], width)
			lineStart = i + 1

			// Early termination: stop if we've exceeded the limit
			if lineCount > maxLines {
				return lineCount
			}
		}
	}

	// Handle last line (if no trailing newline)
	if lineStart < len(content) {
		lineCount += renderedLineHeight(content[lineStart:], width)
	}

	return lineCount
}

// renderedLineHeight calculates the rendered height of a line
// without requiring a string copy by accepting the line boundaries directly.
func renderedLineHeight(line string, width int) int {
	if width <= 0 {
		return 1
	}
	w := ansi.StringWidth(line)
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

// getVisibleContent returns the content that should be visible, preserving ANSI codes.
// Uses pre-computed line starts for O(1) line lookup.
func (m *pagerModel) getVisibleContent() string {
	totalLines := m.lineCount()
	if totalLines == 0 {
		return ""
	}

	// Clamp scroll position to valid range
	start := max(0, min(m.scrollPos, totalLines-1))
	end := min(totalLines, start+m.height)

	if start >= end {
		return ""
	}

	startByte := m.lineStarts[start]
	endByte := len(m.content)

	// Get the byte offset of the line after the last visible line
	// Subtract 1 to exclude the trailing newline
	if end < totalLines {
		endByte = m.lineStarts[end] - 1
	}

	// Safety check for invalid byte range
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

// buildLineStarts creates a slice of byte offsets for the start of each line.
// This enables O(1) line lookup instead of O(n) string splitting.
func buildLineStarts(content string) []int {
	if content == "" {
		return []int{0}
	}

	// Pre-allocate based on newline count for efficiency
	// Add 1 for the first line which always starts at offset 0
	newlineCount := strings.Count(content, "\n")
	starts := make([]int, 0, newlineCount+1)
	starts = append(starts, 0) // First line always starts at 0

	// Find all newline positions
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' && i+1 < len(content) {
			starts = append(starts, i+1)
		}
	}

	return starts
}
