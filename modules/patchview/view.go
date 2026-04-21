package patchview

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/item"
	"github.com/clipperhouse/displaywidth"
)

const (
	headerHeight    = 1
	footerHeight    = 1
	gapWidth        = 1
	borderSize      = 2
	titleHeight     = 1
	hScrollStep     = 10
	hScrollFastStep = 20
)

// PatchView is an interactive patch navigation view.
type PatchView struct {
	patches []*diferenco.Patch
	cursor  int

	renderer  *PatchRenderer
	listVp    *viewport.Model[*patchItem]
	statusBar StatusBar

	width        int
	height       int
	listWidthPct int
	focusRight   bool

	yOffset int
	xOffset int

	style PatchViewStyle
}

// Ensure patchItem implements viewport.Object
var _ viewport.Object = (*patchItem)(nil)

// patchItem wraps a patch for the viewport.
type patchItem struct {
	patch    *diferenco.Patch
	selected bool
	width    int
	style    PatchViewStyle
}

func newPatchItem(p *diferenco.Patch, selected bool, width int, style PatchViewStyle) *patchItem {
	return &patchItem{patch: p, selected: selected, width: width, style: style}
}

func (p *patchItem) GetItem() item.Item {
	return item.NewItem(p.render())
}

func (p *patchItem) render() string {
	path := patchName(p.patch)
	stat := p.patch.Stat()
	additions := stat.Addition
	deletions := stat.Deletion

	added := strconv.Itoa(additions)
	deleted := strconv.Itoa(deletions)

	var statsWidth int
	switch {
	case additions > 0 && deletions > 0:
		statsWidth = len(added) + 3 + len(deleted)
	case additions != 0:
		statsWidth = len(added) + 1
	case deletions != 0:
		statsWidth = len(deleted) + 1
	}

	reserved := 2 + statsWidth + 1
	availableForPath := max(p.width-reserved, 0)

	var line strings.Builder
	if p.selected {
		line.WriteString(p.style.Selected.Render("▌ "))
		if availableForPath > 0 {
			if displaywidth.String(path) > availableForPath {
				line.WriteString(p.style.Selected.Render(truncatePath(path, availableForPath)))
			} else {
				line.WriteString(p.style.Selected.Render(path))
			}
		}
		line.WriteString(p.style.Selected.Render(" "))
		switch {
		case additions > 0 && deletions > 0:
			addStyle := p.style.Selected.Foreground(p.style.Addition.GetForeground())
			delStyle := p.style.Selected.Foreground(p.style.Deletion.GetForeground())
			line.WriteString(addStyle.Render("+" + added))
			line.WriteString(p.style.Selected.Render(" "))
			line.WriteString(delStyle.Render("-" + deleted))
		case additions != 0:
			addStyle := p.style.Selected.Foreground(p.style.Addition.GetForeground())
			line.WriteString(addStyle.Render("+" + added))
		case deletions != 0:
			delStyle := p.style.Selected.Foreground(p.style.Deletion.GetForeground())
			line.WriteString(delStyle.Render("-" + deleted))
		}
	} else {
		line.WriteString("  ")
		if availableForPath > 0 {
			if displaywidth.String(path) > availableForPath {
				line.WriteString(truncatePath(path, availableForPath))
			} else {
				line.WriteString(path)
			}
		}
		line.WriteString(" ")
		switch {
		case additions > 0 && deletions > 0:
			line.WriteString(p.style.Addition.Render("+" + added + " "))
			line.WriteString(p.style.Deletion.Render("-" + deleted))
		case additions != 0:
			line.WriteString(p.style.Addition.Render("+" + added))
		case deletions != 0:
			line.WriteString(p.style.Deletion.Render("-" + deleted))
		}
	}

	return line.String()
}

// Option configures the patch view.
type Option func(*PatchView)

// WithStyle sets a custom style.
func WithStyle(style PatchViewStyle) Option {
	return func(pv *PatchView) {
		pv.style = style
	}
}

// WithListWidth sets the file list width percentage (default 20).
func WithListWidth(pct int) Option {
	return func(pv *PatchView) {
		pv.listWidthPct = pct
	}
}

// WithStatusBar sets a custom status bar.
func WithStatusBar(sb StatusBar) Option {
	return func(pv *PatchView) {
		pv.statusBar = sb
	}
}

// Run starts the interactive patch navigation view.
func Run(patches []*diferenco.Patch, opts ...Option) error {
	if len(patches) == 0 {
		return nil
	}
	pv := NewPatchView(patches, opts...)
	p := tea.NewProgram(pv, tea.WithOutput(os.Stdout))
	_, err := p.Run()
	return err
}

// NewPatchView creates a new PatchView.
func NewPatchView(patches []*diferenco.Patch, opts ...Option) *PatchView {
	pv := &PatchView{
		patches:      patches,
		renderer:     NewPatchRenderer(),
		listVp:       viewport.New(0, 0, viewport.WithSelectionEnabled[*patchItem](true)),
		listWidthPct: 20,
		style:        DefaultStyle(),
	}

	for _, opt := range opts {
		opt(pv)
	}

	// Set up default status bar if not provided
	if pv.statusBar == nil {
		pv.statusBar = NewDefaultStatusBar()
	}

	// Apply style to components
	pv.renderer.SetStyle(pv.style)
	if sb, ok := pv.statusBar.(interface{ SetStyle(PatchViewStyle) }); ok {
		sb.SetStyle(pv.style)
	}
	if sb, ok := pv.statusBar.(PatchesSetter); ok {
		sb.SetPatches(patches)
	}

	return pv
}

func (pv *PatchView) Init() tea.Cmd {
	return nil
}

func (pv *PatchView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		pv.width = msg.Width
		pv.height = msg.Height
		pv.setupLayout()
		return pv, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return pv, tea.Quit

		case "n":
			if pv.cursor < len(pv.patches)-1 {
				pv.selectFile(pv.cursor + 1)
			}
			return pv, nil

		case "p":
			if pv.cursor > 0 {
				pv.selectFile(pv.cursor - 1)
			}
			return pv, nil

		case "tab":
			pv.focusRight = !pv.focusRight
			return pv, nil

		case "left":
			if pv.focusRight {
				pv.focusRight = false
			}
			return pv, nil

		case "right":
			if !pv.focusRight {
				pv.focusRight = true
			}
			return pv, nil
		}

		// Right panel focus: handle diff scrolling
		if pv.focusRight {
			switch msg.String() {
			case "j", "down":
				pv.yOffset++
				pv.clampYOffset()
			case "k", "up":
				pv.yOffset--
				pv.clampYOffset()
			case "h":
				pv.xOffset = max(0, pv.xOffset-hScrollStep)
			case "l":
				pv.xOffset += hScrollStep
			case "ctrl+h", "ctrl+left":
				pv.xOffset = max(0, pv.xOffset-hScrollFastStep)
			case "ctrl+l", "ctrl+right":
				pv.xOffset += hScrollFastStep
			case "ctrl+d":
				pv.yOffset += pv.diffViewportHeight() / 2
				pv.clampYOffset()
			case "ctrl+u":
				pv.yOffset -= pv.diffViewportHeight() / 2
				pv.clampYOffset()
			case "g", "home":
				pv.yOffset = 0
			case "G", "end":
				pv.yOffset = pv.renderer.TotalLines() - pv.diffViewportHeight()
				pv.clampYOffset()
			case "]":
				pv.jumpToNextHunk()
			case "[":
				pv.jumpToPrevHunk()
			}
			return pv, nil
		}

		// Left panel focus: 'l' switches to right panel
		if msg.String() == "l" {
			pv.focusRight = true
			return pv, nil
		}

		// Forward to list viewport
		vp, cmd := pv.listVp.Update(msg)
		pv.listVp = vp
		newCursor := pv.listVp.GetSelectedItemIdx()
		if newCursor != pv.cursor && newCursor >= 0 && newCursor < len(pv.patches) {
			pv.cursor = newCursor
			pv.renderer.SetPatch(pv.patches[newCursor])
			pv.yOffset = 0
			pv.xOffset = 0
			if sb, ok := pv.statusBar.(CursorSetter); ok {
				sb.SetCursor(newCursor)
			}
			pv.updateFileListSelection()
		}
		return pv, cmd
	}

	return pv, nil
}

func (pv *PatchView) View() tea.View {
	if pv.width <= 0 || pv.height <= 0 {
		return tea.NewView("")
	}

	header := pv.renderHeader()
	fileList := pv.renderFileList()
	gap := " "
	diffContent := pv.renderDiffContent()
	footer := pv.renderFooter()

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, fileList, gap, diffContent)
	fullView := lipgloss.JoinVertical(lipgloss.Left, header, mainContent, footer)

	view := tea.NewView(fullView)
	view.AltScreen = true
	return view
}

// Layout calculations

func (pv *PatchView) headerHeight() int {
	if pv.statusBar != nil {
		return pv.statusBar.Height()
	}
	return headerHeight
}

func (pv *PatchView) listPaneHeight() int {
	return max(pv.height-pv.headerHeight()-footerHeight, 0)
}

func (pv *PatchView) listContentHeight() int {
	return max(pv.listPaneHeight()-borderSize-titleHeight, 1)
}

func (pv *PatchView) listWidth() int {
	return max(pv.width*pv.listWidthPct/100, 1)
}

func (pv *PatchView) diffPaneWidth() int {
	return max(pv.width-pv.listWidth()-gapWidth, 0)
}

func (pv *PatchView) diffPaneHeight() int {
	return max(pv.height-pv.headerHeight()-footerHeight, 0)
}

func (pv *PatchView) diffViewportWidth() int {
	return max(pv.diffPaneWidth()-borderSize, 0)
}

func (pv *PatchView) diffViewportHeight() int {
	return max(pv.diffPaneHeight()-borderSize-titleHeight, 0)
}

// Actions

func (pv *PatchView) selectFile(idx int) {
	if len(pv.patches) == 0 {
		return
	}
	idx = max(0, min(idx, len(pv.patches)-1))
	if idx == pv.cursor {
		return
	}
	pv.cursor = idx
	pv.renderer.SetPatch(pv.patches[idx])
	pv.yOffset = 0
	pv.xOffset = 0

	if sb, ok := pv.statusBar.(CursorSetter); ok {
		sb.SetCursor(idx)
	}
}

func (pv *PatchView) setupLayout() {
	listWidth := pv.listWidth() - borderSize
	listHeight := pv.listContentHeight()

	if listWidth > 0 && listHeight > 0 {
		pv.listVp.SetWidth(listWidth)
		pv.listVp.SetHeight(listHeight)
		pv.updateFileList()
	}

	vpWidth := pv.diffViewportWidth()
	vpHeight := pv.diffViewportHeight()
	pv.renderer.SetSize(vpWidth, vpHeight)

	if len(pv.patches) > 0 && pv.renderer.patch == nil {
		pv.renderer.SetPatch(pv.patches[pv.cursor])
	}
}

func (pv *PatchView) updateFileList() {
	if len(pv.patches) == 0 {
		pv.listVp.SetObjects(nil)
		return
	}

	width := pv.listVp.GetWidth()
	items := make([]*patchItem, len(pv.patches))
	for i, p := range pv.patches {
		items[i] = newPatchItem(p, i == pv.cursor, width, pv.style)
	}
	pv.listVp.SetObjects(items)
	pv.listVp.SetSelectedItemIdx(pv.cursor)
}

func (pv *PatchView) updateFileListSelection() {
	if len(pv.patches) == 0 {
		return
	}

	width := pv.listVp.GetWidth()
	items := make([]*patchItem, len(pv.patches))
	for i, p := range pv.patches {
		items[i] = newPatchItem(p, i == pv.cursor, width, pv.style)
	}
	pv.listVp.SetObjects(items)
}

func (pv *PatchView) clampYOffset() {
	maxY := max(0, pv.renderer.TotalLines()-pv.diffViewportHeight())
	pv.yOffset = max(0, min(pv.yOffset, maxY))
}

func (pv *PatchView) jumpToNextHunk() {
	offsets := pv.renderer.HunkOffsets()
	for _, off := range offsets {
		if off > pv.yOffset {
			pv.yOffset = off
			pv.clampYOffset()
			return
		}
	}
}

func (pv *PatchView) jumpToPrevHunk() {
	offsets := pv.renderer.HunkOffsets()
	for i := len(offsets) - 1; i >= 0; i-- {
		if offsets[i] < pv.yOffset {
			pv.yOffset = offsets[i]
			pv.clampYOffset()
			return
		}
	}
}

// Rendering

func (pv *PatchView) renderHeader() string {
	if pv.statusBar != nil {
		return pv.statusBar.View(pv.width)
	}
	return pv.style.HeaderBg.Width(pv.width).Render(" No changes")
}

func (pv *PatchView) renderFileList() string {
	listHeight := pv.listPaneHeight()

	borderColor := lipgloss.Color("8")
	if !pv.focusRight {
		borderColor = lipgloss.Color("12")
	}

	listStyle := lipgloss.NewStyle().
		Width(pv.listWidth()).
		Height(listHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if len(pv.patches) == 0 {
		return listStyle.Render(" No changes")
	}

	title := pv.style.FilesTitle.Render(" Files ")
	content := pv.listVp.View()
	return listStyle.Render(title + "\n" + content)
}

func (pv *PatchView) renderDiffContent() string {
	paneWidth := pv.diffPaneWidth()
	paneHeight := pv.diffPaneHeight()

	borderColor := lipgloss.Color("8")
	if pv.focusRight {
		borderColor = lipgloss.Color("12")
	}

	diffStyle := lipgloss.NewStyle().
		Width(paneWidth).
		Height(paneHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	if len(pv.patches) > 0 {
		pv.renderer.SetYOffset(pv.yOffset)
		pv.renderer.SetXOffset(pv.xOffset)
		content := pv.renderer.Render()

		pctText := ""
		total := pv.renderer.TotalLines()
		if total > 0 {
			vpH := pv.diffViewportHeight()
			pct := min(100, (pv.yOffset+vpH)*100/max(total, 1))
			pctText = fmt.Sprintf(" (%d%%)", pct)
		}

		title := pv.style.FilesTitle.Render(fmt.Sprintf(" Diff%s ", pctText))
		return diffStyle.Render(title + "\n" + content)
	}

	return diffStyle.Render(" No diff content")
}

func (pv *PatchView) renderFooter() string {
	var scrollInfo string
	total := pv.renderer.TotalLines()
	if total > 0 {
		vpH := pv.diffViewportHeight()
		pct := min(100, (pv.yOffset+vpH)*100/max(total, 1))
		scrollInfo = fmt.Sprintf("Lines: %d-%d/%d (%d%%)",
			pv.yOffset+1,
			min(pv.yOffset+vpH, total),
			total,
			pct)

		if pv.xOffset > 0 {
			scrollInfo += fmt.Sprintf("  Col: %d+", pv.xOffset)
		}
	}

	var keys string
	if pv.focusRight {
		keys = "j/k:scroll  h/l:hscroll  [/]:hunk  g/G:top/bottom  tab:files  n/p:file  q:quit"
	} else {
		keys = "j/k:navigate  l/→:diff  tab:diff  n/p:file  q:quit"
	}

	leftWidth := lipgloss.Width(scrollInfo)
	rightWidth := lipgloss.Width(keys)
	spaceWidth := max(pv.width-leftWidth-rightWidth-2, 0)

	content := scrollInfo + " " + strings.Repeat(" ", spaceWidth) + keys

	return pv.style.FooterBg.Width(pv.width).Render(content)
}

// truncatePath truncates a path from the left to fit within maxWidth.
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
