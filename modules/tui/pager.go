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
	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

// Compile-time interface assertion
var _ io.WriteCloser = &Pager{}

// StringObject implements viewport.Object for a single line
type StringObject string

func (s StringObject) GetItem() item.Item {
	return item.NewItem(string(s))
}

// Pager represents a simple terminal pager built with viewport.
// It implements io.Writer and must be closed to display content.
type Pager struct {
	buf          bytes.Buffer
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
	return p.buf.Write(data)
}

// Close finalizes the pager and displays the content.
// For short content that fits in the terminal, it outputs directly without starting the pager.
// For longer content, it starts an interactive pager with viewport.
// Close is idempotent - calling it multiple times is safe.
func (p *Pager) Close() error {
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
	_, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termHeight <= 0 {
		return false
	}

	lineCount := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") && content != "" {
		lineCount++
	}

	return lineCount <= termHeight-4
}

// run starts the interactive pager with the given content using viewport.
func (p *Pager) run(content string) error {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	objects := make([]StringObject, len(lines))
	for i, line := range lines {
		objects[i] = StringObject(line)
	}

	model := &pagerModel{
		vp:        newViewport(objects),
		useAlt:    p.useAltScreen,
		width:     80,
		height:    24,
		totalLine: len(objects),
	}
	program := tea.NewProgram(model, tea.WithOutput(os.Stderr))
	_, err := program.Run()
	return err
}

// ColorMode returns the color mode of the pager.
func (p *Pager) ColorMode() term.Level {
	return p.colorMode
}

func newViewport(objects []StringObject) *viewport.Model[StringObject] {
	styles := viewport.DefaultStyles()
	styles.FooterStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	opts := []viewport.Option[StringObject]{
		viewport.WithFooterEnabled[StringObject](false),
		viewport.WithWrapText[StringObject](false),
		viewport.WithStyles[StringObject](styles),
	}
	vp := viewport.New(80, 24, opts...)
	vp.SetObjects(objects)
	return vp
}

// pagerModel is the bubbletea model for the pager using viewport
type pagerModel struct {
	vp        *viewport.Model[StringObject]
	useAlt    bool
	ready     bool
	width     int
	height    int
	totalLine int
}

// Init initializes the pager model.
func (m *pagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.vp.SetWidth(m.width)
		m.vp.SetHeight(m.height - 1)
		return m, nil

	case tea.KeyPressMsg:
		// Handle quit keys ourselves (viewport doesn't handle these)
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	// Let viewport handle navigation
	vp, cmd := m.vp.Update(msg)
	m.vp = vp
	return m, cmd
}

// View renders the pager UI
func (m *pagerModel) View() tea.View {
	if !m.ready {
		return tea.NewView("Loading...")
	}

	content := m.vp.View()
	statusBar := m.renderStatusBar()
	fullView := lipgloss.JoinVertical(lipgloss.Left, content, statusBar)

	v := tea.NewView(fullView)
	v.AltScreen = m.useAlt
	return v
}

// renderStatusBar creates a status bar with line numbers and progress percentage
func (m *pagerModel) renderStatusBar() string {
	if m.totalLine == 0 {
		return ""
	}

	topIdx, _ := m.vp.GetTopItemIdxAndLineOffset()
	vpHeight := m.vp.GetHeight()
	bottomLine := min(m.totalLine, topIdx+vpHeight)

	var percentage int
	if m.totalLine > 0 {
		percentage = min(100, bottomLine*100/m.totalLine)
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	statusText := fmt.Sprintf("Lines: %d-%d/%d (%d%%) | ↑/k up | ↓/j down | g top | G bottom | space/f page down | b page up | q quit",
		topIdx+1, bottomLine, m.totalLine, percentage)

	return statusStyle.Render(statusText)
}
