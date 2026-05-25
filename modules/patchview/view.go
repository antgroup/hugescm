package patchview

import (
	"fmt"
	"image/color"
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
	listItems []*patchItem
	statusBar StatusBar

	width        int
	height       int
	listWidthPct int
	focusRight   bool

	yOffset int
	xOffset int

	// darkBackground is the resolved theme used by the view. If
	// darkBackgroundExplicit is false the view will probe the terminal
	// on Run.
	darkBackground         bool
	darkBackgroundExplicit bool

	style         PatchViewStyle
	styleExplicit bool
}

// Ensure patchItem implements viewport.Object
var _ viewport.Object = (*patchItem)(nil)

// patchItem wraps a patch for the viewport. The selected status is read
// dynamically from a shared cursor pointer so that changing the cursor does
// not require rebuilding the underlying object slice.
type patchItem struct {
	patch     *diferenco.Patch
	index     int
	cursorPtr *int
	width     int
	style     PatchViewStyle
}

func newPatchItem(p *diferenco.Patch, index int, cursorPtr *int, width int, style PatchViewStyle) *patchItem {
	return &patchItem{patch: p, index: index, cursorPtr: cursorPtr, width: width, style: style}
}

func (p *patchItem) isSelected() bool {
	if p.cursorPtr == nil {
		return false
	}
	return *p.cursorPtr == p.index
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
	selected := p.isSelected()
	if selected {
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

// WithStyle sets a custom style. Setting a custom style suppresses the
// automatic theme switch normally performed by Run.
func WithStyle(style PatchViewStyle) Option {
	return func(pv *PatchView) {
		pv.style = style
		pv.styleExplicit = true
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

// WithDarkBackground forces the dark/light theme. When not provided the
// theme is probed from the terminal once inside Run. The actual theme is
// applied by NewPatchView after all options have run, so the option order
// relative to WithStyle / WithStatusBar does not matter.
func WithDarkBackground(dark bool) Option {
	return func(pv *PatchView) {
		pv.darkBackground = dark
		pv.darkBackgroundExplicit = true
	}
}

// Run starts the interactive patch navigation view.
func Run(patches []*diferenco.Patch, opts ...Option) error {
	if len(patches) == 0 {
		return nil
	}
	pv := NewPatchView(patches, opts...)
	// Probe the terminal exactly once if the caller did not specify a
	// theme. We deliberately keep this IO outside of NewPatchView so tests
	// and library consumers can construct PatchView without touching the
	// terminal.
	if !pv.darkBackgroundExplicit {
		pv.applyTheme(hasDarkBackground())
	}
	p := tea.NewProgram(pv, tea.WithOutput(os.Stdout))
	_, err := p.Run()
	return err
}

// NewPatchView creates a new PatchView. It performs no terminal IO.
//
// Theme handling:
//   - If the caller passes WithDarkBackground, that value is honoured.
//   - If the caller passes WithStyle, the provided style is preserved and
//     subsequent theme switches only update the renderer's dark flag for
//     syntax highlighting.
//   - Otherwise the dark theme is used as the construction-time default to
//     keep this function side-effect free. The Run helper additionally
//     probes the actual terminal once before entering the TUI loop. Callers
//     that drive the model themselves (e.g. embedding it in their own
//     bubbletea program) are responsible for probing the terminal and
//     calling WithDarkBackground; otherwise the dark theme will be used.
func NewPatchView(patches []*diferenco.Patch, opts ...Option) *PatchView {
	pv := &PatchView{
		patches:        patches,
		renderer:       NewPatchRenderer(true),
		listVp:         viewport.New(0, 0, viewport.WithSelectionEnabled[*patchItem](true)),
		listWidthPct:   20,
		style:          DefaultStyle(),
		darkBackground: true,
	}

	for _, opt := range opts {
		opt(pv)
	}

	// Set up default status bar if not provided
	if pv.statusBar == nil {
		pv.statusBar = NewDefaultStatusBar()
	}

	// Apply the resolved theme once, after every component exists. This
	// fixes the option-ordering bug where WithDarkBackground used to call
	// applyTheme before statusBar was constructed.
	pv.applyTheme(pv.darkBackground)

	if sb, ok := pv.statusBar.(PatchesSetter); ok {
		sb.SetPatches(patches)
	}

	return pv
}

// applyTheme rebuilds the default style for the given theme and propagates
// it to the renderer and status bar. If the caller provided a custom style
// via WithStyle we keep that style but still update the renderer's dark
// mode so syntax highlighting picks the matching palette. Either way the
// current pv.style is pushed to the renderer and status bar so callers do
// not need to invoke the setters themselves.
func (pv *PatchView) applyTheme(dark bool) {
	pv.darkBackground = dark
	if !pv.styleExplicit {
		pv.style = DefaultStyleFor(dark)
		for _, it := range pv.listItems {
			it.style = pv.style
		}
	}
	pv.renderer.SetStyle(pv.style)
	pv.renderer.SetDarkBackground(dark)
	if pv.statusBar != nil {
		if sb, ok := pv.statusBar.(StyleSetter); ok {
			sb.SetStyle(pv.style)
		}
	}
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
			pv.syncListSelection()
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
	pv.syncListSelection()
}

func (pv *PatchView) setupLayout() {
	listWidth := pv.listWidth() - borderSize
	listHeight := pv.listContentHeight()

	if listWidth > 0 && listHeight > 0 {
		pv.listVp.SetWidth(listWidth)
		pv.listVp.SetHeight(listHeight)
		pv.rebuildFileList()
	}

	vpWidth := pv.diffViewportWidth()
	vpHeight := pv.diffViewportHeight()
	pv.renderer.SetSize(vpWidth, vpHeight)

	if len(pv.patches) > 0 && pv.renderer.patch == nil {
		pv.renderer.SetPatch(pv.patches[pv.cursor])
	}
}

// rebuildFileList creates the list of patch items. The selected state is
// not baked into the items: each item reads pv.cursor via a shared pointer
// at render time, so navigating between files does not require rebuilding
// the slice (see syncListSelection).
func (pv *PatchView) rebuildFileList() {
	if len(pv.patches) == 0 {
		pv.listItems = nil
		pv.listVp.SetObjects(nil)
		return
	}

	width := pv.listVp.GetWidth()
	items := make([]*patchItem, len(pv.patches))
	for i, p := range pv.patches {
		items[i] = newPatchItem(p, i, &pv.cursor, width, pv.style)
	}
	pv.listItems = items
	pv.listVp.SetObjects(items)
	pv.listVp.SetSelectedItemIdx(pv.cursor)
}

// syncListSelection only updates the viewport's selected index. The
// per-item rendering of the selected state is handled by patchItem itself
// via the shared cursor pointer, so no slice rebuild is needed.
func (pv *PatchView) syncListSelection() {
	if len(pv.patches) == 0 {
		return
	}
	pv.listVp.SetSelectedItemIdx(pv.cursor)
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

	borderColor := pv.borderColor(!pv.focusRight)

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

	borderColor := pv.borderColor(pv.focusRight)

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
			pctText = fmt.Sprintf(" (%d%%)", scrollPercent(pv.yOffset, pv.diffViewportHeight(), total))
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
		pct := scrollPercent(pv.yOffset, vpH, total)
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

// Fallback border colors used when the active style does not provide them.
// Using ANSI 8/12 keeps the look in line with the bundled dark/light themes
// and avoids depending on a specific true-colour palette.
var (
	fallbackBorderActive   color.Color = lipgloss.Color("12")
	fallbackBorderInactive color.Color = lipgloss.Color("8")
)

// borderColor picks the border colour for a pane. When focused is true the
// active colour is used; otherwise the inactive one. A nil entry in the
// style falls back to a safe default so callers do not have to nil-check.
func (pv *PatchView) borderColor(focused bool) color.Color {
	if focused {
		if pv.style.BorderActive != nil {
			return pv.style.BorderActive
		}
		return fallbackBorderActive
	}
	if pv.style.BorderInactive != nil {
		return pv.style.BorderInactive
	}
	return fallbackBorderInactive
}

// scrollPercent returns a 0..100 indicator of how much of the content has
// been scrolled past. The formula is:
//
//	pct = (yOffset + viewportHeight) * 100 / total
//
// which models "the bottom edge of the viewport". Consequences:
//
//   - When the viewport bottom reaches the last line (yOffset+vpH==total),
//     the result is exactly 100.
//   - When the viewport is taller than the total content, the result is
//     clamped to 100 (the user already sees everything).
//   - A non-positive total returns 0 so callers can avoid a separate guard.
//   - Negative offsets are also clamped to 0; the renderer should never pass
//     them but the helper stays total to keep tests cheap.
func scrollPercent(yOffset, viewportHeight, total int) int {
	if total <= 0 {
		return 0
	}
	pct := (yOffset + viewportHeight) * 100 / total
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
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
