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

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"
)

// Theme defines colors for the patch view.
type Theme struct {
	// List colors
	Addition lipgloss.Style
	Deletion lipgloss.Style
	Selected lipgloss.Style

	// Diff content colors
	AdditionLine lipgloss.Style
	DeletionLine lipgloss.Style
	ContextLine  lipgloss.Style
	MetaLine     lipgloss.Style
	FragLine     lipgloss.Style
}

// defaultTheme is the cached default theme instance.
// Colors are designed to be consistent with GitHub's diff styling.
var defaultTheme = Theme{
	Addition: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#22863A"), Dark: lipgloss.Color("#85E89D"),
	}),
	Deletion: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#CB2431"), Dark: lipgloss.Color("#F97583"),
	}),
	Selected: lipgloss.NewStyle().Background(compat.AdaptiveColor{
		Light: lipgloss.Color("#ebf1fc"), Dark: lipgloss.Color("#282a38"),
	}),
	AdditionLine: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#22863A"), Dark: lipgloss.Color("#85E89D"),
	}),
	DeletionLine: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#CB2431"), Dark: lipgloss.Color("#F97583"),
	}),
	ContextLine: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#24292E"), Dark: lipgloss.Color("#E1E4E8"),
	}),
	// MetaLine: diff header lines (diff --zeta, index, new file mode, etc.)
	// Uses a dim/subtle color to not distract from the actual diff content.
	MetaLine: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#6E7781"), Dark: lipgloss.Color("#8B949E"),
	}),
	// FragLine: fragment header lines (---, +++, @@ ... @@)
	// Uses a distinctive but not overwhelming color (cyan/teal tone).
	FragLine: lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#0550AE"), Dark: lipgloss.Color("#58A6FF"),
	}),
}

// DefaultTheme returns the default theme (GitHub-style colors).
// Returns a copy to prevent modification of the cached instance.
func DefaultTheme() Theme {
	return defaultTheme
}

// UI styles - cached to avoid repeated allocations.
var (
	styleHeaderBg    = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	styleFileCount   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSeparator   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stylePathDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	styleFilesTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleFooterBg    = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("7")).
				Padding(0, 1)

	// Status styles for header
	statusStyles = map[string]lipgloss.Style{
		"A": lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		"D": lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		"R": lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		"M": lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
	}
)

type model struct {
	patches []*diferenco.Patch
	cursor  int

	vp      viewport.Model
	width   int
	height  int
	hScroll int

	listWidthPct int
	focusRight   bool
	listYOffset  int
	theme        Theme

	// Cache for formatted diff content.
	// IMPORTANT: This cache assumes that:
	// 1. Patch content is immutable during the viewer's lifetime
	// 2. Theme is static and won't change after initialization
	// If either assumption is violated, the cache must be invalidated.
	cachedPatch    *diferenco.Patch
	cachedContent  string
	cachedMaxWidth int
}

const (
	headerHeight = 1
	footerHeight = 1
	gapWidth     = 1
	borderSize   = 2
	titleHeight  = 1
)

// Run starts the interactive patch navigation view.
func Run(patches []*diferenco.Patch, opts ...Option) error {
	if len(patches) == 0 {
		return nil
	}
	m := &model{
		patches:      patches,
		vp:           viewport.New(),
		listWidthPct: 20,
		theme:        DefaultTheme(),
	}
	for _, opt := range opts {
		opt(m)
	}
	p := tea.NewProgram(m, tea.WithOutput(os.Stdout))
	_, err := p.Run()
	return err
}

// Option configures the patch view.
type Option func(*model)

// WithTheme sets a custom theme.
func WithTheme(theme Theme) Option {
	return func(m *model) {
		m.theme = theme
	}
}

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
	runes := []rune(path)

	width := 0
	cut := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		w := displaywidth.Rune(runes[i])
		if width+w > target {
			break
		}
		width += w
		cut = i
	}
	return "…" + string(runes[cut:])
}

// truncateLeftToWidth truncates a string from the left to fit within maxWidth.
// It preserves ANSI escape codes and uses ellipsis to indicate truncation.
// Example: truncateLeftToWidth("abcde", 3) → "…de"
func truncateLeftToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	width := lipgloss.Width(s)
	if width <= maxWidth {
		return s
	}
	// Calculate how many characters to remove from the left
	remove := width - maxWidth + 1 // +1 for ellipsis
	return ansi.TruncateLeftWc(s, remove, "…")
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
			if !m.focusRight && m.cursor < len(m.patches)-1 {
				m.selectFile(m.cursor + 1)
			}
			return m, nil
		case "n":
			if m.cursor < len(m.patches)-1 {
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
	if len(m.patches) == 0 {
		return
	}
	idx = max(0, min(idx, len(m.patches)-1))

	changed := idx != m.cursor
	if changed {
		m.cursor = idx
		m.hScroll = 0
		m.vp.GotoTop()
	}

	// Always sync state, even if cursor didn't change
	m.syncListYOffset()
	m.updateContent()
}

// syncListYOffset ensures the cursor is visible in the file list.
// This should be called whenever the cursor or list dimensions change.
func (m *model) syncListYOffset() {
	if len(m.patches) == 0 {
		m.listYOffset = 0
		return
	}
	contentHeight := m.listContentHeight()
	totalFiles := len(m.patches)

	startIdx := m.listYOffset
	endIdx := min(startIdx+contentHeight, totalFiles)

	if m.cursor < startIdx {
		m.listYOffset = m.cursor
	} else if m.cursor >= endIdx {
		m.listYOffset = max(0, m.cursor-contentHeight+1)
	}

	// Clamp offset to valid range
	maxOffset := max(0, totalFiles-contentHeight)
	m.listYOffset = min(max(m.listYOffset, 0), maxOffset)
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
	m.syncListYOffset()
	m.updateContent()
}

func (m *model) updateContent() {
	if len(m.patches) == 0 {
		m.vp.SetContent("No changes")
		return
	}

	// Use cached content if available
	currentPatch := m.patches[m.cursor]
	var raw string
	var maxLineWidth int

	if m.cachedPatch == currentPatch {
		// Cache hit: use cached formatted content
		raw = m.cachedContent
		maxLineWidth = m.cachedMaxWidth
	} else {
		// Cache miss: format and cache the content
		raw = m.formatPatch(currentPatch)
		maxLineWidth = 0
		for line := range strings.SplitSeq(raw, "\n") {
			if w := lipgloss.Width(line); w > maxLineWidth {
				maxLineWidth = w
			}
		}
		// Update cache
		m.cachedPatch = currentPatch
		m.cachedContent = raw
		m.cachedMaxWidth = maxLineWidth
	}

	vpWidth := m.vp.Width()
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
	listStyle := paneStyle(m.listWidth(), listHeight, !m.focusRight)

	if len(m.patches) == 0 {
		return listStyle.Render(" No changes")
	}

	// Compute stats width once for the entire list
	statsWidth := m.computeStatsWidth()

	startIdx := m.listYOffset
	endIdx := min(startIdx+contentHeight, len(m.patches))
	lines := make([]string, 0, contentHeight)
	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, m.renderFileLine(m.patches[i], i == m.cursor, innerWidth, statsWidth))
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	title := styleFilesTitle.Render(" Files ")
	return listStyle.Render(title + "\n" + strings.Join(lines, "\n"))
}

func (m *model) renderFileLine(p *diferenco.Patch, selected bool, width, statsWidth int) string {
	stat := p.Stat()
	path := patchName(p)

	// Fixed column layout:
	// [prefix: 2][path: variable][padding: 1][stats: right-aligned]
	const (
		prefixWidth = 2
		padding     = 1
	)

	// Calculate path width
	pathWidth := max(width-prefixWidth-statsWidth-padding, 0)

	// Truncate path if needed
	if pathWidth > 0 && displaywidth.String(path) > pathWidth {
		path = truncatePath(path, pathWidth)
	}
	// Pad path to exact display width (important for wide characters)
	pathDisplay := padRightDisplayWidth(path, pathWidth)

	// Build the complete line
	var line strings.Builder

	if selected {
		// Selected: apply background to entire line, but preserve stats colors
		prefix := "▌ "
		line.WriteString(m.theme.Selected.Render(prefix + pathDisplay + strings.Repeat(" ", padding)))

		// Render stats with selected background but original foreground colors
		if stat.Addition > 0 && stat.Deletion > 0 {
			addedText := fmt.Sprintf("+%d", stat.Addition)
			deletedText := fmt.Sprintf("-%d", stat.Deletion)
			addedStyle := m.theme.Selected.Foreground(m.theme.Addition.GetForeground())
			deletedStyle := m.theme.Selected.Foreground(m.theme.Deletion.GetForeground())
			statsRendered := addedStyle.Render(addedText) + " " + deletedStyle.Render(deletedText)
			statsRenderedWidth := displaywidth.String(addedText) + 1 + displaywidth.String(deletedText)
			paddingNeeded := statsWidth - statsRenderedWidth
			if paddingNeeded > 0 {
				line.WriteString(statsRendered + m.theme.Selected.Render(strings.Repeat(" ", paddingNeeded)))
			} else {
				line.WriteString(statsRendered)
			}
		} else if stat.Addition > 0 {
			addedText := fmt.Sprintf("+%d", stat.Addition)
			addedStyle := m.theme.Selected.Foreground(m.theme.Addition.GetForeground())
			statsRendered := addedStyle.Render(addedText)
			paddingNeeded := statsWidth - displaywidth.String(addedText)
			if paddingNeeded > 0 {
				line.WriteString(statsRendered + m.theme.Selected.Render(strings.Repeat(" ", paddingNeeded)))
			} else {
				line.WriteString(statsRendered)
			}
		} else if stat.Deletion > 0 {
			deletedText := fmt.Sprintf("-%d", stat.Deletion)
			deletedStyle := m.theme.Selected.Foreground(m.theme.Deletion.GetForeground())
			statsRendered := deletedStyle.Render(deletedText)
			paddingNeeded := statsWidth - displaywidth.String(deletedText)
			if paddingNeeded > 0 {
				line.WriteString(statsRendered + m.theme.Selected.Render(strings.Repeat(" ", paddingNeeded)))
			} else {
				line.WriteString(statsRendered)
			}
		} else if statsWidth > 0 {
			line.WriteString(m.theme.Selected.Render(strings.Repeat(" ", statsWidth)))
		}
	} else {
		// Not selected: prefix + path + stats with colors
		line.WriteString("  ")
		line.WriteString(pathDisplay)
		line.WriteString(strings.Repeat(" ", padding))

		// Render stats with colors
		if stat.Addition > 0 && stat.Deletion > 0 {
			addedText := fmt.Sprintf("+%d", stat.Addition)
			deletedText := fmt.Sprintf("-%d", stat.Deletion)
			statsRendered := m.theme.Addition.Render(addedText) + " " + m.theme.Deletion.Render(deletedText)
			statsRenderedWidth := displaywidth.String(addedText) + 1 + displaywidth.String(deletedText)
			paddingNeeded := statsWidth - statsRenderedWidth
			line.WriteString(statsRendered)
			if paddingNeeded > 0 {
				line.WriteString(strings.Repeat(" ", paddingNeeded))
			}
		} else if stat.Addition > 0 {
			addedText := fmt.Sprintf("+%d", stat.Addition)
			line.WriteString(m.theme.Addition.Render(addedText))
			paddingNeeded := statsWidth - displaywidth.String(addedText)
			if paddingNeeded > 0 {
				line.WriteString(strings.Repeat(" ", paddingNeeded))
			}
		} else if stat.Deletion > 0 {
			deletedText := fmt.Sprintf("-%d", stat.Deletion)
			line.WriteString(m.theme.Deletion.Render(deletedText))
			paddingNeeded := statsWidth - displaywidth.String(deletedText)
			if paddingNeeded > 0 {
				line.WriteString(strings.Repeat(" ", paddingNeeded))
			}
		} else if statsWidth > 0 {
			line.WriteString(strings.Repeat(" ", statsWidth))
		}
	}

	return line.String()
}

// padRightDisplayWidth pads a string with spaces on the right to reach the target display width.
// This correctly handles strings containing wide characters (CJK, emoji, etc.).
func padRightDisplayWidth(s string, width int) string {
	w := displaywidth.String(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// computeStatsWidth calculates the display width needed for the stats column.
// It computes the maximum stats width across all patches to ensure consistent
// column width during scrolling. A minimum width is enforced for visual stability,
// and a maximum width prevents excessive space usage for very large stats.
func (m *model) computeStatsWidth() int {
	const (
		minStatsWidth = 4  // keeps a small stable column even for tiny stats
		maxStatsWidth = 16 // prevents excessive space for very large stats
	)

	maxWidth := minStatsWidth
	for _, p := range m.patches {
		stat := p.Stat()
		statsText := ""
		if stat.Addition > 0 && stat.Deletion > 0 {
			statsText = fmt.Sprintf("+%d -%d", stat.Addition, stat.Deletion)
		} else if stat.Addition > 0 {
			statsText = fmt.Sprintf("+%d", stat.Addition)
		} else if stat.Deletion > 0 {
			statsText = fmt.Sprintf("-%d", stat.Deletion)
		}
		if w := displaywidth.String(statsText); w > maxWidth {
			maxWidth = w
		}
	}
	return min(maxWidth, maxStatsWidth)
}

func (m *model) renderDiffContent() string {
	paneWidth := m.diffPaneWidth()
	paneHeight := m.diffPaneHeight()
	diffStyle := paneStyle(paneWidth, paneHeight, m.focusRight)

	if len(m.patches) == 0 {
		return diffStyle.Render(" No diff content")
	}
	var pctText string
	if m.vp.TotalLineCount() > 0 {
		percentage := min(100, (m.vp.YOffset()+m.vp.Height())*100/m.vp.TotalLineCount())
		pctText = fmt.Sprintf(" (%d%%)", percentage)
	}
	title := styleFilesTitle.Render(fmt.Sprintf(" Diff%s ", pctText))
	return diffStyle.Render(title + "\n" + m.vp.View())
}

func statusStyle(status string) lipgloss.Style {
	if s, ok := statusStyles[status]; ok {
		return s
	}
	return statusStyles["M"]
}

// paneStyle returns a styled border for panes, with different colors based on focus state.
func paneStyle(width, height int, focused bool) lipgloss.Style {
	borderColor := lipgloss.Color("8")
	if focused {
		borderColor = lipgloss.Color("12")
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)
}

func (m *model) renderHeader() string {
	if len(m.patches) == 0 {
		return styleHeaderBg.Width(m.width).Render(" No changes")
	}
	p := m.patches[m.cursor]
	stat := p.Stat()
	ps := patchStatus(p)
	status := statusStyle(ps).Render(ps)
	var stats string
	switch {
	case stat.Addition > 0 && stat.Deletion > 0:
		stats = m.theme.Addition.Render("+"+strconv.Itoa(stat.Addition)) + " " + m.theme.Deletion.Render("-"+strconv.Itoa(stat.Deletion))
	case stat.Addition != 0:
		stats = m.theme.Addition.Render("+" + strconv.Itoa(stat.Addition))
	case stat.Deletion != 0:
		stats = m.theme.Deletion.Render("-" + strconv.Itoa(stat.Deletion))
	}
	fileCount := styleFileCount.Render(fmt.Sprintf("%d/%d", m.cursor+1, len(m.patches)))
	sep := styleSeparator.Render("│")
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
	pathDisplay := patchName(p)
	if pathWidth > 0 && lipgloss.Width(pathDisplay) > pathWidth {
		pathDisplay = truncateLeftToWidth(pathDisplay, pathWidth)
	}
	pathDisplay = stylePathDisplay.Render(pathDisplay)
	left := fmt.Sprintf(" %s %s %s", status, sep, pathDisplay)
	if showStats {
		left = fmt.Sprintf(" %s %s %s %s", status, sep, pathDisplay, stats)
	}
	spaceWidth := max(m.width-lipgloss.Width(left)-lipgloss.Width(fileCount), 0)
	return styleHeaderBg.Width(m.width).Render(left + strings.Repeat(" ", spaceWidth) + fileCount)
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
	return styleFooterBg.Width(m.width).Render(content)
}

// patchName returns the display name for a patch.
func patchName(p *diferenco.Patch) string {
	if p == nil {
		return ""
	}
	switch {
	case p.From == nil && p.To != nil:
		return p.To.Name
	case p.From != nil && p.To == nil:
		return p.From.Name
	case p.From != nil && p.To != nil && p.From.Name != p.To.Name:
		return p.From.Name + " → " + p.To.Name
	case p.To != nil:
		return p.To.Name
	case p.From != nil:
		return p.From.Name
	default:
		return ""
	}
}

// patchStatus returns the status character for a patch.
func patchStatus(p *diferenco.Patch) string {
	if p == nil {
		return "M"
	}
	switch {
	case p.From == nil:
		return "A"
	case p.To == nil:
		return "D"
	case p.From != nil && p.To != nil && p.From.Name != p.To.Name:
		return "R"
	default:
		return "M"
	}
}

const (
	srcPrefix = "a/"
	dstPrefix = "b/"
	zeroOID   = "0000000000000000000000000000000000000000000000000000000000000000"
)

// formatPatch formats a single patch using the model's theme.
func (m *model) formatPatch(p *diferenco.Patch) string {
	var sb strings.Builder

	// Write file patch header
	m.writeFilePatchHeader(&sb, p)

	if len(p.Hunks) == 0 {
		return sb.String()
	}

	for _, hunk := range p.Hunks {
		m.formatHunk(&sb, hunk)
	}
	return sb.String()
}

func (m *model) writeFilePatchHeader(sb *strings.Builder, p *diferenco.Patch) {
	from, to := p.From, p.To
	if from == nil && to == nil {
		return
	}

	switch {
	case from != nil && to != nil:
		hashEquals := from.Hash == to.Hash
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("diff --zeta %s%s %s%s", srcPrefix, from.Name, dstPrefix, to.Name)))
		sb.WriteByte('\n')
		if from.Mode != to.Mode {
			sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("old mode %o", from.Mode)))
			sb.WriteByte('\n')
			sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("new mode %o", to.Mode)))
			sb.WriteByte('\n')
		}
		if from.Name != to.Name {
			sb.WriteString(m.theme.MetaLine.Render("rename from " + from.Name))
			sb.WriteByte('\n')
			sb.WriteString(m.theme.MetaLine.Render("rename to " + to.Name))
			sb.WriteByte('\n')
		}
		if from.Mode != to.Mode && !hashEquals {
			sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("index %s..%s", from.Hash, to.Hash)))
			sb.WriteByte('\n')
		} else if !hashEquals {
			sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("index %s..%s %o", from.Hash, to.Hash, from.Mode)))
			sb.WriteByte('\n')
		}
		if !hashEquals {
			if p.IsBinary {
				sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("Binary files %s%s and %s%s differ", srcPrefix, from.Name, dstPrefix, to.Name)))
				sb.WriteByte('\n')
			} else if p.IsFragments {
				sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("Fragments files %s%s and %s%s differ", srcPrefix, from.Name, dstPrefix, to.Name)))
				sb.WriteByte('\n')
			} else {
				sb.WriteString(m.theme.FragLine.Render("--- " + srcPrefix + from.Name))
				sb.WriteByte('\n')
				sb.WriteString(m.theme.FragLine.Render("+++ " + dstPrefix + to.Name))
				sb.WriteByte('\n')
			}
		}
	case from == nil:
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("diff --zeta %s %s", srcPrefix+to.Name, dstPrefix+to.Name)))
		sb.WriteByte('\n')
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("new file mode %o", to.Mode)))
		sb.WriteByte('\n')
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("index %s..%s", zeroOID, to.Hash)))
		sb.WriteByte('\n')
		if p.IsBinary {
			sb.WriteString(m.theme.MetaLine.Render("Binary files /dev/null and " + dstPrefix + to.Name + " differ"))
			sb.WriteByte('\n')
		} else if p.IsFragments {
			sb.WriteString(m.theme.MetaLine.Render("Fragments files /dev/null and " + dstPrefix + to.Name + " differ"))
			sb.WriteByte('\n')
		} else {
			sb.WriteString(m.theme.FragLine.Render("--- /dev/null"))
			sb.WriteByte('\n')
			sb.WriteString(m.theme.FragLine.Render("+++ " + dstPrefix + to.Name))
			sb.WriteByte('\n')
		}
	case to == nil:
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("diff --zeta %s %s", srcPrefix+from.Name, dstPrefix+from.Name)))
		sb.WriteByte('\n')
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("deleted file mode %o", from.Mode)))
		sb.WriteByte('\n')
		sb.WriteString(m.theme.MetaLine.Render(fmt.Sprintf("index %s..%s", from.Hash, zeroOID)))
		sb.WriteByte('\n')
		if p.IsBinary {
			sb.WriteString(m.theme.MetaLine.Render("Binary files " + srcPrefix + from.Name + " and /dev/null differ"))
			sb.WriteByte('\n')
		} else if p.IsFragments {
			sb.WriteString(m.theme.MetaLine.Render("Fragments files " + srcPrefix + from.Name + " and /dev/null differ"))
			sb.WriteByte('\n')
		} else {
			sb.WriteString(m.theme.FragLine.Render("--- " + srcPrefix + from.Name))
			sb.WriteByte('\n')
			sb.WriteString(m.theme.FragLine.Render("+++ /dev/null"))
			sb.WriteByte('\n')
		}
	}
}

func (m *model) formatHunk(sb *strings.Builder, hunk *diferenco.Hunk) {
	fromCount, toCount := 0, 0
	for _, l := range hunk.Lines {
		switch l.Kind {
		case diferenco.Delete:
			fromCount++
		case diferenco.Insert:
			toCount++
		default:
			fromCount++
			toCount++
		}
	}

	var header strings.Builder
	header.WriteString("@@")
	if fromCount > 1 {
		fmt.Fprintf(&header, " -%d,%d", hunk.FromLine, fromCount)
	} else if hunk.FromLine == 1 && fromCount == 0 {
		header.WriteString(" -0,0")
	} else {
		fmt.Fprintf(&header, " -%d", hunk.FromLine)
	}
	if toCount > 1 {
		fmt.Fprintf(&header, " +%d,%d", hunk.ToLine, toCount)
	} else if hunk.ToLine == 1 && toCount == 0 {
		header.WriteString(" +0,0")
	} else {
		fmt.Fprintf(&header, " +%d", hunk.ToLine)
	}
	header.WriteString(" @@")

	sb.WriteString(m.theme.FragLine.Render(header.String()))
	sb.WriteByte('\n')

	for _, line := range hunk.Lines {
		m.formatLine(sb, line)
	}
}

func (m *model) formatLine(sb *strings.Builder, line diferenco.Line) {
	content := strings.TrimSuffix(line.Content, "\n")

	switch line.Kind {
	case diferenco.Delete:
		sb.WriteByte('-')
		sb.WriteString(m.theme.DeletionLine.Render(content))
	case diferenco.Insert:
		sb.WriteByte('+')
		sb.WriteString(m.theme.AdditionLine.Render(content))
	default:
		sb.WriteByte(' ')
		sb.WriteString(m.theme.ContextLine.Render(content))
	}
	sb.WriteByte('\n')
}
