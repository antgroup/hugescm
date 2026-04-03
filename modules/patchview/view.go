package patchview

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/x/ansi"
)

// Entry represents one file patch item in the navigation view.
type Entry struct {
	Name     string
	Status   string
	Addition int
	Deletion int
	Content  string
}

var (
	patchAdd = lipgloss.NewStyle().
			Foreground(compat.AdaptiveColor{Light: lipgloss.Color("#22863A"), Dark: lipgloss.Color("#85E89D")})
	patchDel = lipgloss.NewStyle().
			Foreground(compat.AdaptiveColor{Light: lipgloss.Color("#CB2431"), Dark: lipgloss.Color("#F97583")})
	patchSel = lipgloss.NewStyle().
			Background(compat.AdaptiveColor{Light: lipgloss.Color("#ebf1fc"), Dark: lipgloss.Color("#282a38")})
	patchAddSel = lipgloss.NewStyle().
			Foreground(compat.AdaptiveColor{Light: lipgloss.Color("#22863A"), Dark: lipgloss.Color("#85E89D")}).
			Background(compat.AdaptiveColor{Light: lipgloss.Color("#ebf1fc"), Dark: lipgloss.Color("#282a38")})
	patchDelSel = lipgloss.NewStyle().
			Foreground(compat.AdaptiveColor{Light: lipgloss.Color("#CB2431"), Dark: lipgloss.Color("#F97583")}).
			Background(compat.AdaptiveColor{Light: lipgloss.Color("#ebf1fc"), Dark: lipgloss.Color("#282a38")})
)

type model struct {
	entries []Entry
	cursor  int

	vp      viewport.Model
	width   int
	height  int
	hScroll int

	listWidthPct int
	focusRight   bool
	listYOffset  int
}

const (
	headerHeight = 1
	footerHeight = 1
	gapWidth     = 1
	borderSize   = 2
	titleHeight  = 1
)

// Run starts the interactive patch navigation view.
func Run(entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	m := &model{
		entries:      entries,
		vp:           viewport.New(),
		listWidthPct: 20,
	}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	_, err := p.Run()
	return err
}

func truncatePath(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= width {
		return path
	}
	if width == 1 {
		return "…"
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ansi.Truncate(path, width, "…")
	}
	tail := parts[len(parts)-1]
	prefix := "…/"
	remain := width - lipgloss.Width(prefix)
	if remain <= 0 {
		return ansi.Truncate(path, width, "…")
	}
	if lipgloss.Width(tail) > remain {
		remove := lipgloss.Width(tail) - remain
		tail = ansi.TruncateLeftWc(tail, remove, "")
	}
	return prefix + tail
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.setupLayout()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if !m.focusRight && m.cursor > 0 {
				m.selectFile(m.cursor - 1)
			}
			return m, nil
		case "down", "j":
			if !m.focusRight && m.cursor < len(m.entries)-1 {
				m.selectFile(m.cursor + 1)
			}
			return m, nil
		case "n":
			if m.cursor < len(m.entries)-1 {
				m.selectFile(m.cursor + 1)
			}
			return m, nil
		case "p":
			if m.cursor > 0 {
				m.selectFile(m.cursor - 1)
			}
			return m, nil
		case "tab":
			m.focusRight = !m.focusRight
			return m, nil
		case "left":
			if m.focusRight {
				m.focusRight = false
			}
			return m, nil
		case "right":
			if !m.focusRight {
				m.focusRight = true
			}
			return m, nil
		case "h":
			if m.focusRight {
				m.hScroll = max(0, m.hScroll-10)
				m.updateContent()
			}
			return m, nil
		case "l":
			if m.focusRight {
				m.hScroll += 10
				m.updateContent()
			} else {
				m.focusRight = true
			}
			return m, nil
		case "ctrl+h", "ctrl+left":
			if m.focusRight {
				m.hScroll = max(0, m.hScroll-20)
				m.updateContent()
			}
			return m, nil
		case "ctrl+l", "ctrl+right":
			if m.focusRight {
				m.hScroll += 20
				m.updateContent()
			}
			return m, nil
		}
		if m.focusRight {
			vp, cmd := m.vp.Update(msg)
			m.vp = vp
			return m, cmd
		}
		return m, nil
	}
	vp, cmd := m.vp.Update(msg)
	m.vp = vp
	return m, cmd
}

func (m *model) View() tea.View {
	if m.width <= 0 || m.height <= 0 {
		return tea.NewView("")
	}
	header := m.renderHeader()
	fileList := m.renderFileList()
	diffContent := m.renderDiffContent()
	footer := m.renderFooter()
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, fileList, " ", diffContent)
	fullView := lipgloss.JoinVertical(lipgloss.Left, header, mainContent, footer)
	view := tea.NewView(fullView)
	view.AltScreen = true
	return view
}

func (m *model) selectFile(idx int) {
	if len(m.entries) == 0 {
		return
	}
	idx = max(0, min(idx, len(m.entries)-1))
	if idx == m.cursor {
		return
	}
	m.cursor = idx
	m.hScroll = 0
	m.vp.GotoTop()
	m.updateContent()
}

func (m *model) setupLayout() {
	vpWidth := m.diffViewportWidth()
	vpHeight := m.diffViewportHeight()
	if vpWidth > 0 {
		m.vp.SetWidth(vpWidth)
	}
	if vpHeight > 0 {
		m.vp.SetHeight(vpHeight)
	}
	m.updateContent()
}

func (m *model) updateContent() {
	if len(m.entries) == 0 {
		m.vp.SetContent("No changes")
		return
	}
	raw := m.entries[m.cursor].Content
	vpWidth := m.vp.Width()
	maxLineWidth := 0
	for line := range strings.SplitSeq(raw, "\n") {
		if w := lipgloss.Width(line); w > maxLineWidth {
			maxLineWidth = w
		}
	}
	maxHScroll := max(0, maxLineWidth-vpWidth)
	m.hScroll = min(m.hScroll, maxHScroll)

	content := raw
	if vpWidth > 0 && m.hScroll > 0 {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lineWidth := lipgloss.Width(line)
			if lineWidth > m.hScroll {
				lines[i] = ansi.TruncateLeftWc(line, m.hScroll, "")
			} else {
				lines[i] = ""
			}
		}
		content = strings.Join(lines, "\n")
	}
	m.vp.SetContent(content)
}

func (m *model) listPaneHeight() int {
	return max(m.height-headerHeight-footerHeight, 0)
}
func (m *model) listContentHeight() int {
	return max(m.listPaneHeight()-borderSize-titleHeight, 1)
}
func (m *model) listWidth() int     { return max(m.width*m.listWidthPct/100, 1) }
func (m *model) diffPaneWidth() int { return max(m.width-m.listWidth()-gapWidth, 0) }
func (m *model) diffPaneHeight() int {
	return max(m.height-headerHeight-footerHeight, 0)
}
func (m *model) diffViewportWidth() int { return max(m.diffPaneWidth()-borderSize, 0) }
func (m *model) diffViewportHeight() int {
	return max(m.diffPaneHeight()-borderSize-titleHeight, 0)
}

func (m *model) renderFileList() string {
	innerWidth := m.listWidth() - borderSize
	listHeight := m.listPaneHeight()
	contentHeight := m.listContentHeight()
	borderColor := lipgloss.Color("8")
	if !m.focusRight {
		borderColor = lipgloss.Color("12")
	}
	listStyle := lipgloss.NewStyle().
		Width(m.listWidth()).
		Height(listHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)
	if len(m.entries) == 0 {
		return listStyle.Render(" No changes")
	}
	totalFiles := len(m.entries)
	startIdx := m.listYOffset
	endIdx := min(startIdx+contentHeight, totalFiles)
	if m.cursor < startIdx {
		startIdx = m.cursor
		endIdx = min(startIdx+contentHeight, totalFiles)
		m.listYOffset = startIdx
	} else if m.cursor >= endIdx {
		endIdx = m.cursor + 1
		startIdx = max(0, endIdx-contentHeight)
		m.listYOffset = startIdx
	}
	var lines []string
	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, m.renderFileLine(m.entries[i], i == m.cursor, innerWidth))
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Render(" Files ")
	return listStyle.Render(title + "\n" + strings.Join(lines, "\n"))
}

func (m *model) renderFileLine(e Entry, selected bool, width int) string {
	path := e.Name
	added := strconv.Itoa(e.Addition)
	deleted := strconv.Itoa(e.Deletion)
	var statsWidth int
	switch {
	case e.Addition > 0 && e.Deletion > 0:
		statsWidth = len(added) + 3 + len(deleted)
	case e.Addition != 0:
		statsWidth = len(added) + 1
	case e.Deletion != 0:
		statsWidth = len(deleted) + 1
	}
	reserved := 2 + statsWidth + 1
	availableForPath := max(width-reserved, 0)
	var line strings.Builder
	if selected {
		_, _ = line.WriteString(patchSel.Render("▌ "))
		switch {
		case availableForPath == 0:
		case lipgloss.Width(path) > availableForPath:
			_, _ = line.WriteString(patchSel.Render(truncatePath(path, availableForPath)))
		default:
			_, _ = line.WriteString(patchSel.Render(path))
		}
		_, _ = line.WriteString(patchSel.Render(" "))
		switch {
		case e.Addition > 0 && e.Deletion > 0:
			_, _ = line.WriteString(patchAddSel.Render("+" + added))
			_, _ = line.WriteString(patchSel.Render(" "))
			_, _ = line.WriteString(patchDelSel.Render("-" + deleted))
		case e.Addition != 0:
			_, _ = line.WriteString(patchAddSel.Render("+" + added))
		case e.Deletion != 0:
			_, _ = line.WriteString(patchDelSel.Render("-" + deleted))
		}
		return line.String()
	}
	_, _ = line.WriteString("  ")
	switch {
	case availableForPath == 0:
	case lipgloss.Width(path) > availableForPath:
		_, _ = line.WriteString(truncatePath(path, availableForPath))
	default:
		_, _ = line.WriteString(path)
	}
	_ = line.WriteByte(' ')
	switch {
	case e.Addition > 0 && e.Deletion > 0:
		_, _ = line.WriteString(patchAdd.Render("+" + added))
		_ = line.WriteByte(' ')
		_, _ = line.WriteString(patchDel.Render("-" + deleted))
	case e.Addition != 0:
		_, _ = line.WriteString(patchAdd.Render("+" + added))
	case e.Deletion != 0:
		_, _ = line.WriteString(patchDel.Render("-" + deleted))
	}
	return line.String()
}

func (m *model) renderDiffContent() string {
	paneWidth := m.diffPaneWidth()
	paneHeight := m.diffPaneHeight()
	borderColor := lipgloss.Color("8")
	if m.focusRight {
		borderColor = lipgloss.Color("12")
	}
	diffStyle := lipgloss.NewStyle().
		Width(paneWidth).
		Height(paneHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	if len(m.entries) == 0 {
		return diffStyle.Render(" No diff content")
	}
	var pctText string
	if m.vp.TotalLineCount() > 0 {
		percentage := min(100, (m.vp.YOffset()+m.vp.Height())*100/m.vp.TotalLineCount())
		pctText = fmt.Sprintf(" (%d%%)", percentage)
	}
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Render(fmt.Sprintf(" Diff%s ", pctText))
	return diffStyle.Render(title + "\n" + m.vp.View())
}

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "A":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	case "D":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	case "R":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	}
}

func (m *model) renderHeader() string {
	if len(m.entries) == 0 {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Width(m.width).
			Render(" No changes")
	}
	e := m.entries[m.cursor]
	status := e.Status
	if len(status) == 0 {
		status = "M"
	}
	status = statusStyle(status).Render(status)
	var stats string
	switch {
	case e.Addition > 0 && e.Deletion > 0:
		stats = patchAdd.Render("+"+strconv.Itoa(e.Addition)) + " " + patchDel.Render("-"+strconv.Itoa(e.Deletion))
	case e.Addition != 0:
		stats = patchAdd.Render("+" + strconv.Itoa(e.Addition))
	case e.Deletion != 0:
		stats = patchDel.Render("-" + strconv.Itoa(e.Deletion))
	}
	fileCount := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render(fmt.Sprintf("%d/%d", m.cursor+1, len(m.entries)))
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("│")
	fileCountWidth := lipgloss.Width(fileCount)
	statsWidth := lipgloss.Width(stats)
	fixedWidth := 1 + 1 + 3 + fileCountWidth + 2
	availableForPathAndStats := m.width - fixedWidth
	showStats := availableForPathAndStats > statsWidth+10
	pathWidth := availableForPathAndStats
	if showStats {
		pathWidth = availableForPathAndStats - statsWidth - 1
	}
	pathWidth = max(pathWidth, 0)
	pathDisplay := e.Name
	if pathWidth > 0 && lipgloss.Width(pathDisplay) > pathWidth {
		remove := lipgloss.Width(pathDisplay) - pathWidth + 1
		pathDisplay = ansi.TruncateLeftWc(pathDisplay, remove, "…")
	}
	pathDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).Render(pathDisplay)
	left := fmt.Sprintf(" %s %s %s", status, sep, pathDisplay)
	if showStats {
		left = fmt.Sprintf(" %s %s %s %s", status, sep, pathDisplay, stats)
	}
	spaceWidth := max(m.width-lipgloss.Width(left)-lipgloss.Width(fileCount), 0)
	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Width(m.width).
		Render(left + strings.Repeat(" ", spaceWidth) + fileCount)
}

func (m *model) renderFooter() string {
	var scrollInfo string
	if m.vp.TotalLineCount() > 0 {
		percentage := min(100, (m.vp.YOffset()+m.vp.Height())*100/m.vp.TotalLineCount())
		scrollInfo = fmt.Sprintf("Lines: %d-%d/%d (%d%%)",
			m.vp.YOffset()+1,
			min(m.vp.YOffset()+m.vp.Height(), m.vp.TotalLineCount()),
			m.vp.TotalLineCount(),
			percentage)
		if m.hScroll > 0 {
			scrollInfo += fmt.Sprintf("  Col: %d+", m.hScroll)
		}
	}
	keys := "j/k:navigate  l/→:diff  tab:diff  n/p:file  q:quit"
	if m.focusRight {
		keys = "j/k:scroll  h/l:hscroll  ←→:focus  tab:files  n/p:file  q:quit"
	}
	spaceWidth := max(m.width-lipgloss.Width(scrollInfo)-lipgloss.Width(keys)-2, 0)
	content := scrollInfo + " " + strings.Repeat(" ", spaceWidth) + keys
	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("7")).
		Padding(0, 1).
		Width(m.width).
		Render(content)
}
