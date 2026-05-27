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
	"github.com/charmbracelet/x/ansi"
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

	// topHeaderKeyColumn is the fixed width (in display columns) reserved
	// for "Key: " prefixes in the top header. It is sized to comfortably
	// hold the longest canonical key ("Subject:") plus a trailing space.
	topHeaderKeyColumn = 9
)

// HeaderEntry is a single "Key: value" row in the top header block.
//
// When Key == "", the row is rendered as a plain continuation line
// indented to topHeaderKeyColumn (used by legacy WithHeader callers that
// pass a free-form multi-line string).
//
// Highlight = true asks the view to render Value with the bold accent
// style (style.HeaderHash) — used for commit hashes.
type HeaderEntry struct {
	Key       string // canonical key text without the trailing ":" (e.g. "Commit").
	Value     string
	Highlight bool // if true, render Value with style.HeaderHash (bold + accent).
}

// PatchView is an interactive patch navigation view.
type PatchView struct {
	patches []*diferenco.Patch
	cursor  int

	renderer  *PatchRenderer
	listVp    *viewport.Model[*patchItem]
	statusBar StatusBar

	// headerEntries is the structured top header (rendered as a borderless
	// "Key: value" block above the status bar). Empty means no top header
	// is shown — the view collapses to just status bar + main + footer.
	headerEntries []HeaderEntry

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

// WithHeader sets a free-form multi-line top header. Each "\n"-separated
// segment becomes its own row, rendered as a borderless continuation line
// (no "Key:" prefix, indented to align with the value column used by
// WithCommitHeader).
//
// Pass an empty string (or do not call this option) to suppress the top
// header entirely — the view will collapse back to just the status bar.
//
// For commit-style metadata prefer WithCommitHeader, which produces the
// canonical 4-row "Commit/Author/Date/Subject" layout with a highlighted
// hash; use WithHeader for ad-hoc diff range descriptions and similar.
func WithHeader(text string) Option {
	return func(pv *PatchView) {
		if text == "" {
			pv.headerEntries = nil
			return
		}
		var entries []HeaderEntry
		for line := range strings.SplitSeq(text, "\n") {
			entries = append(entries, HeaderEntry{Value: line})
		}
		pv.headerEntries = entries
	}
}

// WithHeaderEntries sets a structured top header, one entry per row, of
// the form "Key: value". A nil key argument produces a continuation row
// (no key prefix) that still aligns under the value column.
//
// Use this when you need full control (e.g. mixing highlighted and plain
// values); prefer WithCommitHeader for the common commit-metadata case.
func WithHeaderEntries(entries ...HeaderEntry) Option {
	return func(pv *PatchView) {
		if len(entries) == 0 {
			pv.headerEntries = nil
			return
		}
		pv.headerEntries = append([]HeaderEntry(nil), entries...)
	}
}

// WithCommitHeader produces the canonical commit-style top header:
//
//	Commit:  <hash>            (hash highlighted via style.HeaderHash)
//	Author:  <author>
//	Date:    <date>
//	Subject: <subject>
//	Files:   <files>           (only included when non-empty)
//
// Empty fields drop their entire row, so callers can pass "" for missing
// metadata without producing blank lines.
func WithCommitHeader(hash, author, date, subject string) Option {
	return func(pv *PatchView) {
		pv.headerEntries = commitHeaderEntries(hash, author, date, subject, "")
	}
}

// WithCommitHeaderWithFiles is like WithCommitHeader but appends an extra
// "Files: <summary>" row (typically produced by SummarizePatches).
func WithCommitHeaderWithFiles(hash, author, date, subject, files string) Option {
	return func(pv *PatchView) {
		pv.headerEntries = commitHeaderEntries(hash, author, date, subject, files)
	}
}

func commitHeaderEntries(hash, author, date, subject, files string) []HeaderEntry {
	var entries []HeaderEntry
	if hash != "" {
		entries = append(entries, HeaderEntry{Key: "Commit", Value: hash, Highlight: true})
	}
	if author != "" {
		entries = append(entries, HeaderEntry{Key: "Author", Value: author})
	}
	if date != "" {
		entries = append(entries, HeaderEntry{Key: "Date", Value: date})
	}
	if subject != "" {
		entries = append(entries, HeaderEntry{Key: "Subject", Value: subject})
	}
	if files != "" {
		entries = append(entries, HeaderEntry{Key: "Files", Value: files})
	}
	return entries
}

// Run starts the interactive patch navigation view.
//
// When patches is empty the TUI is skipped — there is nothing meaningful
// to navigate. If the caller configured a top header (e.g. via
// WithCommitHeader) we still print it to stdout followed by a
// "No changes" line, so callers like `hot show` keep showing commit
// metadata for merge / empty commits instead of degenerating to a bare
// "No changes" message.
func Run(patches []*diferenco.Patch, opts ...Option) error {
	if len(patches) == 0 {
		// Build a throw-away PatchView purely to resolve the configured
		// header entries + style without duplicating option plumbing.
		pv := NewPatchView(patches, opts...)
		if header := RenderHeaderEntries(pv.style, pv.headerEntries); header != "" {
			_, _ = fmt.Fprintln(os.Stdout, header)
		}
		_, _ = fmt.Fprintln(os.Stdout, "No changes")
		return nil
	}
	pv := NewPatchView(patches, opts...)
	p := tea.NewProgram(pv, tea.WithOutput(os.Stdout))
	_, err := p.Run()
	return err
}

// RenderCommitHeader renders the canonical commit-style header block
// (Commit / Author / Date / Subject [/ Files]) to a plain (but ANSI-
// colored) string with one row per non-empty field. Unlike the TUI top
// header it does not truncate to terminal width — callers print it
// directly to stdout.
//
// Use this when you want to surface commit metadata outside the
// interactive view (e.g. when there are no patches to navigate and the
// TUI is skipped entirely).
func RenderCommitHeader(style PatchViewStyle, hash, author, date, subject, files string) string {
	return RenderHeaderEntries(style, commitHeaderEntries(hash, author, date, subject, files))
}

// RenderHeaderEntries renders a list of HeaderEntry rows as a plain
// (ANSI-colored) string, using the same key padding + highlight rules
// as the in-TUI top header. Returns "" for an empty input so callers
// can cheaply skip the row.
func RenderHeaderEntries(style PatchViewStyle, entries []HeaderEntry) string {
	if len(entries) == 0 {
		return ""
	}
	rows := make([]string, len(entries))
	for i, e := range entries {
		var prefix string
		if e.Key != "" {
			prefix = e.Key + ":"
			if displaywidth.String(prefix) < topHeaderKeyColumn {
				prefix += strings.Repeat(" ", topHeaderKeyColumn-displaywidth.String(prefix))
			}
			prefix = style.HeaderMeta.Render(prefix)
		} else {
			prefix = strings.Repeat(" ", topHeaderKeyColumn)
		}
		value := e.Value
		if e.Highlight {
			value = style.HeaderHash.Render(value)
		}
		rows[i] = prefix + value
	}
	return strings.Join(rows, "\n")
}

// SummarizePatches returns a compact one-line summary of a patch set in
// the form "N files changed, +A -D". Empty input returns "". This is
// shared by command_diff / command_show / showdiff / show so the header
// subtitle stays consistent across entry points.
//
// The returned string is plain text (no ANSI). For a colorized variant
// suitable for direct use as a HeaderEntry value, use
// ColorizedPatchSummary.
func SummarizePatches(patches []*diferenco.Patch) string {
	if len(patches) == 0 {
		return ""
	}
	add, del := patchTotals(patches)
	noun := "files"
	if len(patches) == 1 {
		noun = "file"
	}
	return fmt.Sprintf("%d %s changed, +%d -%d", len(patches), noun, add, del)
}

// ColorizedPatchSummary is like SummarizePatches but renders the +A/-D
// segments with the same Addition/Deletion colors used in the file list,
// so the Files: row of the top header matches the per-file stats visually.
//
// Pass DefaultStyle() (or the same style you pass via WithStyle) so the
// dark/light theme stays consistent. Empty input returns "".
//
// The rest of the string ("N files changed, ") uses the caller's default
// foreground — we intentionally do NOT wrap it with HeaderMeta because
// the header renderer already applies HeaderMeta-like keying via the
// "Files:" prefix and double-styling would dim the +A/-D too much.
func ColorizedPatchSummary(style PatchViewStyle, patches []*diferenco.Patch) string {
	if len(patches) == 0 {
		return ""
	}
	add, del := patchTotals(patches)
	noun := "files"
	if len(patches) == 1 {
		noun = "file"
	}
	return fmt.Sprintf("%d %s changed, %s %s",
		len(patches), noun,
		style.Addition.Render(fmt.Sprintf("+%d", add)),
		style.Deletion.Render(fmt.Sprintf("-%d", del)),
	)
}

func patchTotals(patches []*diferenco.Patch) (add, del int) {
	for _, p := range patches {
		if p == nil {
			continue
		}
		st := p.Stat()
		add += st.Addition
		del += st.Deletion
	}
	return add, del
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

	statusBar := pv.renderHeader()
	fileList := pv.renderFileList()
	gap := " "
	diffContent := pv.renderDiffContent()
	footer := pv.renderFooter()

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, fileList, gap, diffContent)

	var fullView string
	if topHeader := pv.renderTopHeader(); topHeader != "" {
		fullView = lipgloss.JoinVertical(lipgloss.Left, topHeader, statusBar, mainContent, footer)
	} else {
		fullView = lipgloss.JoinVertical(lipgloss.Left, statusBar, mainContent, footer)
	}

	view := tea.NewView(fullView)
	view.AltScreen = true
	return view
}

// Layout calculations

// topHeaderHeight returns the rendered height of the optional top header
// block, or 0 when no header is configured.
//
// The header is rendered borderless (no padding rows), so the height is
// simply len(pv.headerEntries) — one terminal row per entry.
//
// This invariant (height == len(entries) == lipgloss.Height(renderTopHeader()))
// is locked down by view_test.go because any drift directly pushes the
// diff pane into the file list.
func (pv *PatchView) topHeaderHeight() int {
	return len(pv.headerEntries)
}

func (pv *PatchView) headerHeight() int {
	if pv.statusBar != nil {
		return pv.statusBar.Height()
	}
	return headerHeight
}

func (pv *PatchView) listPaneHeight() int {
	return max(pv.height-pv.topHeaderHeight()-pv.headerHeight()-footerHeight, 0)
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
	return max(pv.height-pv.topHeaderHeight()-pv.headerHeight()-footerHeight, 0)
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

// renderTopHeader renders the optional top header as a borderless block
// of "Key: value" rows. Total rendered width is at most pv.width (each
// row is hard-truncated; we never wrap, so height stays predictable).
//
// Returns "" when no header is configured, which signals the caller to
// skip the row entirely in JoinVertical.
//
// Layout:
//   - Each row is `<key padded to topHeaderKeyColumn><value>`.
//   - Keys use style.HeaderMeta (subdued).
//   - Values use the default foreground, except entries flagged
//     highlight=true which use style.HeaderHash (bold accent — used for
//     commit hashes).
//   - Continuation rows (key == "") render as `<spaces><value>` aligned
//     to the value column.
func (pv *PatchView) renderTopHeader() string {
	if len(pv.headerEntries) == 0 {
		return ""
	}

	width := pv.width
	if width <= 0 {
		return ""
	}

	keyPad := min(topHeaderKeyColumn, width)
	valueWidth := max(width-keyPad, 0)

	rows := make([]string, len(pv.headerEntries))
	for i, e := range pv.headerEntries {
		var prefix string
		if e.Key != "" {
			prefix = e.Key + ":"
			if displaywidth.String(prefix) < keyPad {
				prefix += strings.Repeat(" ", keyPad-displaywidth.String(prefix))
			}
			prefix = pv.style.HeaderMeta.Render(prefix)
		} else {
			prefix = strings.Repeat(" ", keyPad)
		}

		value := e.Value
		if valueWidth > 0 && lipgloss.Width(value) > valueWidth {
			value = ansi.TruncateWc(value, valueWidth, "…")
		}
		if e.Highlight {
			value = pv.style.HeaderHash.Render(value)
		}

		rows[i] = prefix + value
	}

	return strings.Join(rows, "\n")
}

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
