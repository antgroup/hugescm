package viewport

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

// Terminology:
// - object: an object of type T that implements the Object interface, i.e. has an Item() method
// - item: the item.Item returned by an object's Item() method. A single item may span multiple viewport lines.
//         if selection is enabled, the item is the selectable unit
// - line: a line of text on one row of terminal cells
// - visible: in the vertical sense, a line is visible if it is within the viewport
// - truncated: in the horizontal sense, a line is truncated if it is too long to fit in the viewport
//
// wrap disabled, wide enough viewport:
//                           item index      line index
// this is the first line    0               0
// this is the second line   1               1
//
// wrap disabled, overflows viewport width:
//                           item index      line index
// this is the first...      0               0
// this is the secon...      1               1
//
// wrap enabled:
//               item index      line index
// this is the   0               0
// first line    0               1
// this is the   1               2
// second line   1               3

var surroundingAnsiRegex = regexp.MustCompile(`(\x1b\[[0-9;]*m.*?\x1b\[0?m)`)

// CompareFn is a function type for comparing two items of type T.
type CompareFn[T any] func(a, b T) bool

// Option is a functional option for configuring the viewport
type Option[T Object] func(*Model[T])

// WithKeyMap sets the key mapping for the viewport
func WithKeyMap[T Object](keyMap KeyMap) Option[T] {
	return func(m *Model[T]) {
		m.navigation.keyMap = keyMap
	}
}

// WithStyles sets the styling for the viewport
func WithStyles[T Object](styles Styles) Option[T] {
	return func(m *Model[T]) {
		m.display.styles = styles
	}
}

// WithWrapText sets whether the viewport wraps text
func WithWrapText[T Object](wrap bool) Option[T] {
	return func(m *Model[T]) {
		m.SetWrapText(wrap)
	}
}

// WithSelectionEnabled sets whether the viewport allows selection
func WithSelectionEnabled[T Object](enabled bool) Option[T] {
	return func(m *Model[T]) {
		m.SetSelectionEnabled(enabled)
	}
}

// WithFooterEnabled sets whether the viewport shows the footer
func WithFooterEnabled[T Object](enabled bool) Option[T] {
	return func(m *Model[T]) {
		m.SetFooterEnabled(enabled)
	}
}

// WithProgressBarEnabled sets whether the footer displays a Unicode progress bar
func WithProgressBarEnabled[T Object](enabled bool) Option[T] {
	return func(m *Model[T]) {
		m.SetProgressBarEnabled(enabled)
	}
}

// WithStickyTop sets whether to automatically scroll to the top when content changes
func WithStickyTop[T Object](stickyTop bool) Option[T] {
	return func(m *Model[T]) {
		m.SetTopSticky(stickyTop)
	}
}

// WithStickyBottom sets whether to automatically scroll to the bottom when content changes
func WithStickyBottom[T Object](stickyBottom bool) Option[T] {
	return func(m *Model[T]) {
		m.SetBottomSticky(stickyBottom)
	}
}

// WithSelectionStyleOverridesItemStyle controls whether the selection style replaces the item's
// existing ANSI styling. When true (default), the selected item is stripped of its original
// styling and the selection style is applied to all non-highlighted regions. When false,
// the item keeps its original styling and the selection style is applied only to unstyled regions.
func WithSelectionStyleOverridesItemStyle[T Object](overrides bool) Option[T] {
	return func(m *Model[T]) {
		m.config.selectionStyleOverridesItemStyle = overrides
	}
}

// WithFileSaving configures automatic file saving when a hotkey is pressed.
// Files are saved to the specified directory with timestamp-based names.
func WithFileSaving[T Object](saveDir string, saveKey key.Binding) Option[T] {
	return func(m *Model[T]) {
		m.config.saveDir = saveDir
		m.config.saveKey = saveKey
	}
}

// Model represents a viewport component
type Model[T Object] struct {
	// content manages the content and selection state
	content *contentManager[T]

	// display handles rendering
	display *displayManager

	// navigation manages keyboard input and navigation logic
	navigation *navigationManager

	// config manages configuration options
	config *configuration
}

// New creates a new viewport model with reasonable defaults
func New[T Object](width, height int, opts ...Option[T]) (m *Model[T]) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	m = &Model[T]{}
	m.content = newContentManager[T]()
	m.display = newDisplayManager(width, height, DefaultStyles())
	m.navigation = newNavigationManager(DefaultKeyMap())
	m.config = newConfiguration()

	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}

	return m
}

// Update processes messages and updates the model
func (m *Model[T]) Update(msg tea.Msg) (*Model[T], tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// route all messages to filename textinput when actively entering filename
	if m.config.saveState.enteringFilename {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.Code {
			case tea.KeyEnter:
				filename := m.config.saveState.filenameInput.Value()
				if filename == "" {
					filename = time.Now().Format("20060102-150405") + ".txt"
				} else if !strings.HasSuffix(filename, ".txt") {
					filename += ".txt"
				}
				m.config.saveState.enteringFilename = false
				m.config.saveState.saving = true
				return m, m.saveToFile(filename)
			case tea.KeyEscape:
				m.config.saveState.enteringFilename = false
				return m, nil
			}
		}
		// forward all non-KeyMsg messages to textinput (e.g. cursor blink)
		m.config.saveState.filenameInput, cmd = m.config.saveState.filenameInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.config.saveKey) {
			saveDirDefined := m.config.saveDir != ""
			saving := m.config.saveState.saving
			showingResult := m.config.saveState.showingResult
			enteringFilename := m.config.saveState.enteringFilename
			if !saveDirDefined || saving || showingResult || enteringFilename {
				return m, nil
			}
			ti := textinput.New()
			ti.Placeholder = time.Now().Format("20060102-150405") + ".txt"
			ti.Focus()
			ti.CharLimit = 256
			ti.SetWidth(m.display.bounds.width - 20)
			m.config.saveState.filenameInput = ti
			m.config.saveState.enteringFilename = true
			return m, textinput.Blink
		}

	case fileSavedMsg:
		// update save state with result
		m.config.saveState.saving = false
		m.config.saveState.showingResult = true
		if msg.err != nil {
			m.config.saveState.isError = true
			m.config.saveState.resultMsg = fmt.Sprintf("Save failed: %v", msg.err)
		} else {
			m.config.saveState.isError = false
			m.config.saveState.resultMsg = fmt.Sprintf("Saved to %s", msg.filename)
		}
		// start 4 second timer to clear result
		cmd = func() tea.Msg {
			time.Sleep(4 * time.Second)
			return clearSaveResultMsg{}
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case clearSaveResultMsg:
		// clear the save result display
		m.config.saveState.showingResult = false
		m.config.saveState.resultMsg = ""
		m.config.saveState.isError = false
		return m, nil
	}

	// handle navigation for KeyMsg
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		navCtx := navigationContext{
			wrapText:        m.config.wrapText,
			dimensions:      m.display.bounds,
			numContentLines: m.getNumContentLines(),
			numVisibleItems: m.getNumVisibleItems(),
		}
		navResult := m.navigation.processKeyMsg(keyMsg, navCtx)

		switch navResult.action {
		case actionUp:
			if m.navigation.selectionEnabled {
				m.SetSelectedItemIdx(m.content.getSelectedIdx() - navResult.selectionAmount)
			} else {
				m.scrollDownLines(-navResult.scrollAmount)
			}

		case actionDown:
			if m.navigation.selectionEnabled {
				m.SetSelectedItemIdx(m.content.getSelectedIdx() + navResult.selectionAmount)
			} else {
				m.scrollDownLines(navResult.scrollAmount)
			}

		case actionLeft:
			if !m.config.wrapText {
				m.SetXOffset(m.display.xOffset - navResult.scrollAmount)
			}

		case actionRight:
			if !m.config.wrapText {
				m.SetXOffset(m.display.xOffset + navResult.scrollAmount)
			}

		case actionHalfPageUp, actionPageUp:
			m.scrollDownLines(-navResult.scrollAmount)
			if m.navigation.selectionEnabled {
				m.SetSelectedItemIdx(m.content.getSelectedIdx() - navResult.selectionAmount)
			}

		case actionHalfPageDown, actionPageDown:
			m.scrollDownLines(navResult.scrollAmount)
			if m.navigation.selectionEnabled {
				m.SetSelectedItemIdx(m.content.getSelectedIdx() + navResult.selectionAmount)
			}

		case actionTop:
			if m.navigation.selectionEnabled {
				m.SetSelectedItemIdx(0)
			} else {
				m.display.topItemIdx = 0
				m.display.topItemLineOffset = 0
			}

		case actionBottom:
			if m.navigation.selectionEnabled {
				m.SetSelectedItemIdx(m.content.getSelectedIdx() + m.content.numItems())
			} else {
				maxItemIdx, maxTopLineOffset := m.maxItemIdxAndMaxTopLineOffset()
				m.display.setTopItemIdxAndOffset(maxItemIdx, maxTopLineOffset)
			}

		default:
			// no-op on keypress that doesn't produce a selection action
		}
	}

	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// View renders the viewport
func (m *Model[T]) View() string {
	var builder strings.Builder
	wrap := m.config.wrapText

	visibleHeaderLines := m.getVisibleHeaderLines()
	itemIndexes := m.getVisibleContentItemIndexes()

	// pre-allocate capacity based on estimated size
	estimatedSize := (len(visibleHeaderLines) + len(itemIndexes) + 10) * (m.display.bounds.width + 1)
	builder.Grow(estimatedSize)

	// header lines
	for i := range visibleHeaderLines {
		headerItem := item.NewItem(visibleHeaderLines[i])
		line, _ := headerItem.Take(0, m.display.bounds.width, m.config.continuationIndicator, []item.Highlight{})
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	// render post-header line if set
	if m.config.postHeaderLine != "" {
		postHeaderItem := item.NewItem(m.config.postHeaderLine)
		truncated, _ := postHeaderItem.Take(0, m.display.bounds.width, m.config.continuationIndicator, []item.Highlight{})
		builder.WriteString(truncated)
		builder.WriteByte('\n')
	}

	// content lines — render each visible line using segment-aware logic.
	// An item may have multiple line-broken segments (via LineBrokenItems()), each rendered
	// on a separate terminal line and wrapping independently.
	truncatedVisibleContentLines := make([]string, len(itemIndexes))

	// selection prefix: when selection is enabled and a prefix is configured,
	// prepend the prefix to selected lines and equivalent padding to others
	cw := m.contentWidth()
	hasPrefix := m.navigation.selectionEnabled && m.display.styles.SelectionPrefix != ""
	prefixPad := m.selectionPrefixPadding()

	// segment tracking state for multi-line items
	var currentSegments []item.Item
	currentSegIdx := 0
	currentCellsToLeft := 0
	prevItemIdx := -1

	// initialize segment state for the first visible item
	if wrap && len(itemIndexes) > 0 {
		topItem := m.content.objects[itemIndexes[0]].GetItem()
		currentSegments = topItem.LineBrokenItems()
		var wrapOffset int
		currentSegIdx, wrapOffset = decomposeLineOffset(currentSegments, m.display.topItemLineOffset, cw)
		currentCellsToLeft = wrapOffset * cw
		prevItemIdx = itemIndexes[0]
	}

	for idx, itemIdx := range itemIndexes {
		// when we encounter a new item, refresh segment tracking
		if itemIdx != prevItemIdx {
			fullItem := m.content.objects[itemIdx].GetItem()
			currentSegments = fullItem.LineBrokenItems()
			currentSegIdx = 0
			currentCellsToLeft = 0
			prevItemIdx = itemIdx
		}

		var truncated string
		isSelection := m.navigation.selectionEnabled && itemIdx == m.content.getSelectedIdx()

		// get highlights for this item and remap to current segment
		highlights := m.getHighlightsForItem(itemIdx)
		if isSelection && m.config.selectionStyleOverridesItemStyle {
			highlights = m.selectionHighlights(itemIdx, highlights)
		}
		highlights = remapHighlightsForSegment(highlights, currentSegments, currentSegIdx)

		// get the current segment to render
		segment := currentSegments[currentSegIdx]

		// when selection style overrides item style, use a stripped segment (no ANSI) so only
		// highlight styling applies, preventing original content styling from leaking through
		if isSelection && m.config.selectionStyleOverridesItemStyle {
			segment = item.NewItem(segment.ContentNoAnsi())
		}

		if wrap {
			var widthTaken int
			truncated, widthTaken = segment.Take(
				currentCellsToLeft,
				cw,
				"",
				highlights,
			)
			// advance segment tracking for next iteration
			if idx+1 < len(itemIndexes) && itemIndexes[idx+1] == itemIdx {
				currentCellsToLeft += widthTaken
				if currentCellsToLeft >= segment.Width() {
					currentSegIdx++
					currentCellsToLeft = 0
				}
			}
		} else {
			// non-wrapped: render segment with horizontal panning
			truncated, _ = segment.Take(
				m.display.xOffset,
				cw,
				m.config.continuationIndicator,
				highlights,
			)
		}

		if isSelection && !m.config.selectionStyleOverridesItemStyle {
			truncated = m.styleSelection(truncated)
		}

		pannedRight := m.display.xOffset > 0
		segmentHasWidth := segment.Width() > 0
		pannedPastAllWidth := lipgloss.Width(truncated) == 0
		if !wrap && pannedRight && segmentHasWidth && pannedPastAllWidth {
			// if panned right past where line ends, show continuation indicator
			continuation := item.NewItem(m.config.continuationIndicator)
			truncated, _ = continuation.Take(0, cw, "", []item.Highlight{})
			if isSelection {
				truncated = m.display.styles.SelectedItemStyle.Render(item.StripAnsi(truncated))
			}
		}

		if isSelection && lipgloss.Width(truncated) == 0 {
			// ensure selection is visible even if line empty
			truncated = m.display.styles.SelectedItemStyle.Render(" ")
		}

		// prepend selection prefix or padding
		if hasPrefix {
			if isSelection {
				truncated = m.display.styles.SelectionPrefix + truncated
			} else {
				truncated = prefixPad + truncated
			}
		}

		truncatedVisibleContentLines[idx] = truncated
	}

	for i := range truncatedVisibleContentLines {
		builder.WriteString(truncatedVisibleContentLines[i])
		builder.WriteByte('\n')
	}

	nVisibleLines := len(itemIndexes)
	padCount := max(0, m.getNumContentLines()-nVisibleLines)
	for range padCount {
		builder.WriteByte('\n')
	}

	// render pre-footer line if set
	if m.config.preFooterLine != "" {
		preFooterItem := item.NewItem(m.config.preFooterLine)
		truncated, _ := preFooterItem.Take(0, m.display.bounds.width, m.config.continuationIndicator, []item.Highlight{})
		builder.WriteString(truncated)
		builder.WriteByte('\n')
	}

	if m.config.saveState.enteringFilename {
		// show filename input in footer
		prompt := "Save as: "
		inputView := m.config.saveState.filenameInput.View()
		footerContent := prompt + inputView
		footerItem := item.NewItem(footerContent)
		truncated, _ := footerItem.Take(0, m.display.bounds.width, m.config.continuationIndicator, []item.Highlight{})
		builder.WriteString(m.display.styles.FooterStyle.Render(truncated))
	} else if m.config.saveState.saving || m.config.saveState.showingResult {
		// show save status footer
		var statusMsg string
		if m.config.saveState.saving {
			statusMsg = "Saving..."
		} else if m.config.saveState.showingResult {
			statusMsg = m.config.saveState.resultMsg
		}
		statusItem := item.NewItem(statusMsg)
		truncated, _ := statusItem.Take(0, m.display.bounds.width, m.config.continuationIndicator, []item.Highlight{})
		styledMsg := m.display.styles.FooterStyle.Render(truncated)
		builder.WriteString(styledMsg)
	} else if m.config.footerEnabled {
		// pad so footer shows up at bottom
		builder.WriteString(m.getTruncatedFooterLine(itemIndexes))
	}

	return m.display.render(strings.TrimSuffix(builder.String(), "\n"))
}

// SetObjects sets the objects
func (m *Model[T]) SetObjects(objects []T) {
	var initialNumLinesAboveSelection int
	var stayAtTop, stayAtBottom bool
	var prevSelection T
	if m.navigation.selectionEnabled {
		if inView := m.selectionInViewInfo(); inView.numLinesSelectionInView > 0 {
			initialNumLinesAboveSelection = inView.numLinesAboveSelection
		}
		currentItems := m.content.objects
		selectedIdx := m.content.getSelectedIdx()
		if m.navigation.topSticky && len(currentItems) > 0 && selectedIdx == 0 {
			stayAtTop = true
		} else if m.navigation.bottomSticky && (len(currentItems) == 0 || (selectedIdx == len(currentItems)-1)) {
			stayAtBottom = true
		} else if m.content.compareFn != nil && 0 <= selectedIdx && selectedIdx < len(currentItems) {
			prevSelection = currentItems[selectedIdx]
		}
	} else {
		if m.navigation.topSticky && m.isScrolledToTop() {
			stayAtTop = true
		} else if m.navigation.bottomSticky && m.isScrolledToBottom() {
			stayAtBottom = true
		}
	}

	m.content.objects = objects
	// ensure scroll position is valid given new Item
	m.safelySetTopItemIdxAndOffset(m.display.topItemIdx, m.display.topItemLineOffset)

	// ensure xOffset is valid given new Item
	m.SetXOffset(m.display.xOffset)

	if m.navigation.selectionEnabled {
		if stayAtTop {
			m.content.setSelectedIdx(0)
		} else if stayAtBottom {
			m.content.setSelectedIdx(max(0, m.content.numItems()-1))
			m.scrollSoSelectionInView()
		} else if m.content.compareFn != nil {
			// TODO: could flag when items are sorted & comparable and use binary search instead
			found := false
			items := m.content.objects
			for i := range items {
				if m.content.compareFn(items[i], prevSelection) {
					m.content.setSelectedIdx(i)
					found = true
					break
				}
			}
			if !found {
				m.content.setSelectedIdx(0)
			}
		}

		// when staying at bottom, just want to scroll so selection in view, which is done above
		if !stayAtBottom {
			m.content.selectedIdx = clampValZeroToMax(m.content.selectedIdx, len(m.content.objects)-1)
			m.scrollSoSelectionInView()
			if inView := m.selectionInViewInfo(); inView.numLinesSelectionInView > 0 {
				deltaLinesAbove := initialNumLinesAboveSelection - inView.numLinesAboveSelection
				m.scrollDownLines(-deltaLinesAbove)
			}
		}
	} else {
		if stayAtTop {
			m.display.setTopItemIdxAndOffset(0, 0)
		} else if stayAtBottom {
			maxItemIdx, maxTopLineOffset := m.maxItemIdxAndMaxTopLineOffset()
			m.display.setTopItemIdxAndOffset(maxItemIdx, maxTopLineOffset)
		}
	}
}

// SetTopSticky sets whether selection should stay at top when new Item added and selection is at the top
func (m *Model[T]) SetTopSticky(topSticky bool) {
	m.navigation.topSticky = topSticky
}

// SetBottomSticky sets whether selection should stay at bottom when new Item added and selection is at the bottom
func (m *Model[T]) SetBottomSticky(bottomSticky bool) {
	m.navigation.bottomSticky = bottomSticky
}

// SetSelectionEnabled sets whether the viewport allows line selection
func (m *Model[T]) SetSelectionEnabled(selectionEnabled bool) {
	wasEnabled := m.navigation.selectionEnabled
	m.navigation.selectionEnabled = selectionEnabled

	// when enabling selection, set the selected item to the top visible item and ensure the top line is in view
	if selectionEnabled && !wasEnabled && !m.content.isEmpty() {
		topVisibleItemIdx := clampValZeroToMax(m.display.topItemIdx, m.content.numItems()-1)
		m.content.setSelectedIdx(topVisibleItemIdx)
		m.scrollSoSelectionInView()
	}
}

// SetFooterEnabled sets whether the viewport shows the footer when it overflows
func (m *Model[T]) SetFooterEnabled(footerEnabled bool) {
	m.config.footerEnabled = footerEnabled
}

// SetProgressBarEnabled sets whether the footer displays a Unicode progress bar in the footer
func (m *Model[T]) SetProgressBarEnabled(enabled bool) {
	m.config.progressBarEnabled = enabled
}

// SetPostHeaderLine sets a line to render just below the header.
// Pass empty string to disable. The line will be truncated to viewport width.
func (m *Model[T]) SetPostHeaderLine(line string) {
	m.config.postHeaderLine = line
}

// SetPreFooterLine sets a line to render just above the footer.
// Pass empty string to disable. The line will be truncated to viewport width.
func (m *Model[T]) SetPreFooterLine(line string) {
	m.config.preFooterLine = line
}

// GetPreFooterLine returns the current pre-footer line.
func (m *Model[T]) GetPreFooterLine() string {
	return m.config.preFooterLine
}

// SetSelectionComparator sets the comparator function for maintaining the current selection when Item changes.
// If compareFn is non-nil, the viewport will try to maintain the current selection when Item changes.
func (m *Model[T]) SetSelectionComparator(compareFn CompareFn[T]) {
	m.content.compareFn = compareFn
}

// GetSelectionEnabled returns whether the viewport allows line selection
func (m *Model[T]) GetSelectionEnabled() bool {
	return m.navigation.selectionEnabled
}

// IsCapturingInput returns true when the viewport is in a mode that should capture all input
// (e.g., filename entry for saving). Callers should forward all messages to the viewport
// without processing them when this returns true.
func (m *Model[T]) IsCapturingInput() bool {
	return m.config.saveState.enteringFilename
}

// SetWrapText sets whether the viewport wraps text
func (m *Model[T]) SetWrapText(wrapText bool) {
	var initialNumLinesAboveSelection int
	if m.navigation.selectionEnabled {
		if inView := m.selectionInViewInfo(); inView.numLinesSelectionInView > 0 {
			initialNumLinesAboveSelection = inView.numLinesAboveSelection
		}
	}
	m.config.wrapText = wrapText
	m.display.topItemLineOffset = 0
	m.display.xOffset = 0
	if m.navigation.selectionEnabled {
		m.scrollSoSelectionInView()
		if inView := m.selectionInViewInfo(); inView.numLinesSelectionInView > 0 {
			deltaLinesAbove := initialNumLinesAboveSelection - inView.numLinesAboveSelection
			m.scrollDownLines(-deltaLinesAbove)
			m.scrollSoSelectionInView()
		}
	}
	m.safelySetTopItemIdxAndOffset(m.display.topItemIdx, m.display.topItemLineOffset)
}

// GetWrapText returns whether the viewport wraps text
func (m *Model[T]) GetWrapText() bool {
	return m.config.wrapText
}

// SetWidth sets the viewport's width
func (m *Model[T]) SetWidth(width int) {
	m.setWidthHeight(width, m.display.bounds.height)
}

// GetWidth returns the viewport width
func (m *Model[T]) GetWidth() int {
	return m.display.bounds.width
}

// SetHeight sets the viewport's height, including header and footer
func (m *Model[T]) SetHeight(height int) {
	m.setWidthHeight(m.display.bounds.width, height)
}

// GetHeight returns the viewport height
func (m *Model[T]) GetHeight() int {
	return m.display.bounds.height
}

// SetStyles sets the styling for the viewport
func (m *Model[T]) SetStyles(styles Styles) {
	m.display.styles = styles
}

// GetTopItemIdxAndLineOffset returns the current top item index and line offset within that item
func (m *Model[T]) GetTopItemIdxAndLineOffset() (int, int) {
	return m.display.topItemIdx, m.display.topItemLineOffset
}

// SetSelectedItemIdx sets the selected context index. Automatically puts selection in view as necessary
func (m *Model[T]) SetSelectedItemIdx(selectedItemIdx int) {
	if !m.navigation.selectionEnabled {
		return
	}
	m.content.setSelectedIdx(selectedItemIdx)
	m.scrollSoSelectionInView()
}

// GetSelectedItemIdx returns the currently selected item index
func (m *Model[T]) GetSelectedItemIdx() int {
	if !m.navigation.selectionEnabled {
		return 0
	}
	return m.content.getSelectedIdx()
}

// GetSelectedItem returns a pointer to the currently selected item
func (m *Model[T]) GetSelectedItem() *T {
	if !m.navigation.selectionEnabled {
		return nil
	}
	return m.content.getSelectedItem()
}

// SetHeader sets the header, an unselectable set of lines at the top of the viewport
func (m *Model[T]) SetHeader(header []string) {
	m.content.header = header
}

// EnsureItemInView scrolls or pans the viewport so that the specified portion of an item is visible.
// If the desired item portion is above or below the current view, it scrolls vertically to bring it into view, leaving
// verticalPad number of lines of context if possible.
// If the desired item portion is to the left or right of the current view, it pans horizontally to bring it into view,
// leaving horizontalPad number of columns of context if possible.
// Afterwards, it's possible that the selection is out of view of the viewport.
func (m *Model[T]) EnsureItemInView(itemIdx, startWidth, endWidth, verticalPad, horizontalPad int) {
	if m.display.bounds.width == 0 {
		return
	}
	if m.content.isEmpty() {
		m.safelySetTopItemIdxAndOffset(0, 0)
		return
	}

	itemIdx, startWidth, endWidth = m.clampItemAndWidthParams(itemIdx, startWidth, endWidth)

	if m.config.wrapText {
		m.ensureWrappedPortionInView(itemIdx, startWidth, endWidth, verticalPad)
	} else {
		m.ensureUnwrappedItemVerticallyInView(itemIdx, verticalPad)
		m.ensureUnwrappedPortionHorizontallyInView(startWidth, endWidth, horizontalPad)
	}
}

// clampItemAndWidthParams clamps itemIdx, startWidth, and endWidth to valid ranges
func (m *Model[T]) clampItemAndWidthParams(itemIdx, startWidth, endWidth int) (int, int, int) {
	itemIdx = max(0, min(itemIdx, m.content.numItems()-1))
	itemWidth := m.content.objects[itemIdx].GetItem().Width()
	startWidth = max(0, min(startWidth, itemWidth))
	endWidth = max(startWidth, min(endWidth, itemWidth))
	return itemIdx, startWidth, endWidth
}

// ensureWrappedPortionInView ensures the specified portion is visible in wrapped mode
func (m *Model[T]) ensureWrappedPortionInView(itemIdx, startWidth, endWidth, verticalPad int) {
	if !m.config.wrapText {
		panic("ensureWrappedPortionInView called when wrapText is false")
	}
	viewportWidth := m.contentWidth()
	segments := m.content.objects[itemIdx].GetItem().LineBrokenItems()
	startLineOffset := lineOffsetForCellPosition(segments, startWidth, viewportWidth)
	endLineOffset := lineOffsetForCellPosition(segments, max(0, endWidth-1), viewportWidth)
	if endWidth == 0 {
		endLineOffset = 0
	}

	numLinesInPortion := endLineOffset - startLineOffset + 1
	numContentLines := m.getNumContentLines()

	// portion larger than viewport: align top with padding if possible
	if numLinesInPortion >= numContentLines {
		desiredLinesAbove := min(verticalPad, numContentLines-1)
		if startLineOffset >= desiredLinesAbove {
			m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset-desiredLinesAbove)
		} else {
			// need to scroll up to previous items to get padding
			m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset)
			m.scrollDownLines(-desiredLinesAbove)
		}
		return
	}

	// check if already in view before any scroll-direction-based positioning
	// this prevents oscillation when scrollingDown changes between calls
	portionStartInView, portionEndInView, linesAbovePortion, linesBelowPortion := m.getWrappedPortionViewInfo(itemIdx, startLineOffset, endLineOffset)

	// if fully visible, check if position is already acceptable
	if portionStartInView && portionEndInView {
		// when padding can't be satisfied on both sides, check if already centered
		if verticalPad*2+numLinesInPortion > numContentLines {
			// only skip repositioning if already approximately centered (within 1 line)
			// this prevents oscillation while still allowing initial centering
			desiredPadding := numContentLines / 2
			paddingDiff := linesAbovePortion - linesBelowPortion
			if paddingDiff < 0 {
				paddingDiff = -paddingDiff
			}
			if paddingDiff <= 1 ||
				(linesAbovePortion >= desiredPadding-1 && linesBelowPortion >= desiredPadding-1) {
				return
			}
			// not centered, fall through to scroll-direction-based repositioning below
		} else {
			// padding can be satisfied on both sides
			desiredPad := min(verticalPad, numContentLines-numLinesInPortion)
			// already fully visible, check if padding is respected
			if linesAbovePortion >= desiredPad && linesBelowPortion >= desiredPad {
				return
			}

			// adjust position to ensure padding on the side that needs it
			if linesBelowPortion < desiredPad {
				// insufficient padding below, position to add more padding below
				linesToGoBack := numContentLines - 1 - desiredPad
				if endLineOffset >= linesToGoBack {
					m.safelySetTopItemIdxAndOffset(itemIdx, endLineOffset-linesToGoBack)
				} else {
					targetItemIdx, targetOffset := m.getItemIdxAbove(itemIdx, endLineOffset, linesToGoBack-endLineOffset)
					m.safelySetTopItemIdxAndOffset(targetItemIdx, targetOffset)
				}
			} else {
				// insufficient padding above, position to add more padding above
				if startLineOffset >= desiredPad {
					m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset-desiredPad)
				} else {
					targetItemIdx, targetOffset := m.getItemIdxAbove(itemIdx, startLineOffset, desiredPad-startLineOffset)
					m.safelySetTopItemIdxAndOffset(targetItemIdx, targetOffset)
				}
			}
			return
		}
	}

	// not visible, position based on scrolling direction
	scrollingDown := m.targetBelowTop(itemIdx, startLineOffset)

	// when padding can't be satisfied on both sides, center based on scroll direction
	if verticalPad*2+numLinesInPortion > numContentLines {
		desiredPadding := numContentLines / 2
		if scrollingDown {
			// scrolling down: leave desiredPadding lines below
			m.safelySetTopItemIdxAndOffset(itemIdx, endLineOffset)
			linesFromTarget := m.linesBetweenCurrentTopAndTarget(itemIdx, endLineOffset)
			linesToScrollUp := max(0, numContentLines-1-desiredPadding-linesFromTarget)
			m.scrollDownLines(-linesToScrollUp)
		} else {
			// scrolling up: leave desiredPadding lines above
			if startLineOffset >= desiredPadding {
				m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset-desiredPadding)
			} else {
				m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset)
				m.scrollDownLines(-desiredPadding)
			}
		}
		return
	}

	desiredPad := min(verticalPad, numContentLines-numLinesInPortion)
	if scrollingDown {
		// scrolling down: leave desiredPad lines below
		m.safelySetTopItemIdxAndOffset(itemIdx, endLineOffset)
		linesFromTarget := m.linesBetweenCurrentTopAndTarget(itemIdx, endLineOffset)
		linesToScrollUp := max(0, numContentLines-1-desiredPad-linesFromTarget)
		m.scrollDownLines(-linesToScrollUp)
	} else {
		// scrolling up: leave desiredPad lines above
		if startLineOffset >= desiredPad {
			m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset-desiredPad)
		} else {
			m.safelySetTopItemIdxAndOffset(itemIdx, startLineOffset)
			m.scrollDownLines(-desiredPad)
		}
	}
}

// getWrappedPortionViewInfo returns whether the portion is in view and padding information
func (m *Model[T]) getWrappedPortionViewInfo(itemIdx, startLineOffset, endLineOffset int) (portionStartInView, portionEndInView bool, linesAbove, linesBelow int) {
	if !m.config.wrapText {
		panic("getWrappedPortionViewInfo called when wrapText is false")
	}
	itemIndexes := m.getVisibleContentItemIndexes()
	itemFirstSeenAt := -1
	portionStartPos := -1
	portionEndPos := -1

	for i, visibleItemIdx := range itemIndexes {
		if visibleItemIdx == itemIdx {
			if itemFirstSeenAt == -1 {
				itemFirstSeenAt = i
			}
			lineOffsetInItem := i - itemFirstSeenAt
			if m.display.topItemIdx == itemIdx && itemFirstSeenAt == 0 {
				lineOffsetInItem += m.display.topItemLineOffset
			}
			if lineOffsetInItem == startLineOffset {
				portionStartInView = true
				portionStartPos = i
			}
			if lineOffsetInItem == endLineOffset {
				portionEndInView = true
				portionEndPos = i
			}
		}
	}

	if portionStartInView {
		linesAbove = portionStartPos
	}
	if portionEndInView {
		linesBelow = len(itemIndexes) - portionEndPos - 1
	}

	return portionStartInView, portionEndInView, linesAbove, linesBelow
}

// targetBelowTop checks if a target item & line is below the current top of viewport
func (m *Model[T]) targetBelowTop(targetItemIdx, targetStartLineOffset int) bool {
	if m.display.topItemIdx < targetItemIdx {
		return true
	}
	if m.display.topItemIdx == targetItemIdx && m.display.topItemLineOffset < targetStartLineOffset {
		return true
	}
	return false
}

// linesBetweenCurrentTopAndTarget calculates how many lines separate current top line from target position
func (m *Model[T]) linesBetweenCurrentTopAndTarget(targetItemIdx, targetLineOffset int) int {
	if m.display.topItemIdx > targetItemIdx {
		panic("current top item index is after target item index")
	}

	if m.display.topItemIdx == targetItemIdx {
		return targetLineOffset - m.display.topItemLineOffset
	}

	// count lines from top item to target
	linesFromTarget := m.numLinesForItem(m.display.topItemIdx) - m.display.topItemLineOffset
	for idx := m.display.topItemIdx + 1; idx < targetItemIdx; idx++ {
		linesFromTarget += m.numLinesForItem(idx)
	}
	linesFromTarget += targetLineOffset

	return linesFromTarget
}

// ensureUnwrappedItemVerticallyInView scrolls vertically to bring item into view
func (m *Model[T]) ensureUnwrappedItemVerticallyInView(itemIdx, verticalPad int) {
	if m.config.wrapText {
		panic("ensureUnwrappedItemVerticallyInView called when wrapText is true")
	}
	itemIndexes := m.getVisibleContentItemIndexes()
	numContentLines := m.getNumContentLines()

	// check if already visible
	visiblePosition := -1
	for i, visibleItemIdx := range itemIndexes {
		if visibleItemIdx == itemIdx {
			visiblePosition = i
			break
		}
	}

	itemInBottomHalfOfViewport := m.display.topItemIdx+numContentLines/2 <= itemIdx

	// when padding can't be satisfied on both sides, center the item
	if verticalPad*2+1 > numContentLines {
		desiredPadding := numContentLines / 2
		if itemInBottomHalfOfViewport {
			// leave desiredPadding lines below
			targetTopItemIdx := max(0, itemIdx-numContentLines+1+desiredPadding)
			m.safelySetTopItemIdxAndOffset(targetTopItemIdx, 0)
		} else {
			// leave desiredPadding lines above
			targetTopItemIdx := max(0, itemIdx-desiredPadding)
			m.safelySetTopItemIdxAndOffset(targetTopItemIdx, 0)
		}
		return
	}

	desiredPad := min(verticalPad, numContentLines-1)

	if visiblePosition >= 0 {
		// item is visible, check if padding is respected
		linesAbove := visiblePosition
		linesBelow := len(itemIndexes) - visiblePosition - 1

		if linesAbove >= desiredPad && linesBelow >= desiredPad {
			return
		}

		if itemInBottomHalfOfViewport {
			targetTopItemIdx := max(0, itemIdx-numContentLines+1+desiredPad)
			m.safelySetTopItemIdxAndOffset(targetTopItemIdx, 0)
		} else {
			targetTopItemIdx := max(0, itemIdx-desiredPad)
			m.safelySetTopItemIdxAndOffset(targetTopItemIdx, 0)
		}
		return
	}

	// not visible, position based on item position
	if itemInBottomHalfOfViewport {
		// leave desiredPad lines below
		targetTopItemIdx := max(0, itemIdx-numContentLines+1+desiredPad)
		m.safelySetTopItemIdxAndOffset(targetTopItemIdx, 0)
	} else {
		// leave desiredPad lines above
		targetTopItemIdx := max(0, itemIdx-desiredPad)
		m.safelySetTopItemIdxAndOffset(targetTopItemIdx, 0)
	}
}

// ensureUnwrappedPortionHorizontallyInView pans horizontally to bring portion into view
func (m *Model[T]) ensureUnwrappedPortionHorizontallyInView(startWidth, endWidth, horizontalPad int) {
	if m.config.wrapText {
		panic("ensureUnwrappedPortionHorizontallyInView called when wrapText is true")
	}
	viewportWidth := m.contentWidth()
	currentXOffset := m.display.xOffset

	visibleStartWidth := currentXOffset + 1
	visibleEndWidth := currentXOffset + viewportWidth

	portionStartInView := startWidth >= visibleStartWidth && startWidth <= visibleEndWidth
	portionEndInView := endWidth >= visibleStartWidth && endWidth <= visibleEndWidth

	portionWidth := endWidth - startWidth
	panningRight := startWidth > visibleStartWidth

	// portion wider than viewport: align left edge with padding
	if portionWidth > viewportWidth {
		desiredColumnsLeft := min(horizontalPad, viewportWidth-1)
		targetXOffset := max(0, startWidth-desiredColumnsLeft)
		m.SetXOffset(targetXOffset)
		return
	}

	// when padding can't be satisfied on both sides, center the portion
	if horizontalPad*2+portionWidth > viewportWidth {
		desiredColumnsLeft := (viewportWidth - portionWidth) / 2
		targetXOffset := max(0, startWidth-desiredColumnsLeft)
		m.SetXOffset(targetXOffset)
		return
	}

	desiredPad := min(horizontalPad, viewportWidth-portionWidth)

	if portionStartInView && portionEndInView {
		// already fully visible, check if padding is respected
		columnsLeft := startWidth - currentXOffset
		columnsRight := currentXOffset + viewportWidth - endWidth

		if columnsLeft >= desiredPad && columnsRight >= desiredPad {
			return
		}

		// adjust position based on panning direction
		if panningRight {
			targetXOffset := max(0, endWidth+desiredPad-viewportWidth)
			m.SetXOffset(targetXOffset)
		} else {
			targetXOffset := max(0, startWidth-desiredPad)
			m.SetXOffset(targetXOffset)
		}
		return
	}

	// not visible, position based on panning direction
	if panningRight {
		// panning right: leave desiredPad columns to the right
		targetXOffset := max(0, endWidth+desiredPad-viewportWidth)
		m.SetXOffset(targetXOffset)
	} else {
		// panning left: leave desiredPad columns to the left
		targetXOffset := max(0, startWidth-desiredPad)
		m.SetXOffset(targetXOffset)
	}
}

// SetXOffset sets the horizontal offset, in terminal cell width, for panning when text wrapping is disabled
func (m *Model[T]) SetXOffset(widthOffset int) {
	if m.config.wrapText {
		return
	}
	maxXOffset := m.maxItemWidth() - m.contentWidth()
	m.display.xOffset = max(0, min(maxXOffset, widthOffset))
}

// GetXOffsetWidth returns the horizontal offset, in terminal cell width, for panning when text wrapping is disabled
func (m *Model[T]) GetXOffsetWidth() int {
	if m.config.wrapText {
		return 0
	}
	return m.display.xOffset
}

// SetHighlights sets specific positions to highlight with custom styles in the viewport.
func (m *Model[T]) SetHighlights(highlights []Highlight) {
	m.content.setHighlights(highlights)
}

// GetHighlights returns all highlights.
func (m *Model[T]) GetHighlights() []Highlight {
	return m.content.getHighlights()
}

func (m *Model[T]) maxItemWidth() int {
	if m.config.wrapText {
		panic("maxItemWidth should not be called when wrapping is enabled")
	}

	maxLineWidth := 0

	headerLines := m.getVisibleHeaderLines()
	for i := range headerLines {
		if w := lipgloss.Width(headerLines[i]); w > maxLineWidth {
			maxLineWidth = w
		}
	}

	// check content line widths without fully rendering all of them
	if !m.content.isEmpty() {
		items := m.content.objects
		startIdx := clampValZeroToMax(m.display.topItemIdx, m.content.numItems()-1)
		numItemsToCheck := min(m.content.numItems()-startIdx, m.display.bounds.height)

		for i := range numItemsToCheck {
			itemIdx := startIdx + i
			if itemIdx >= m.content.numItems() {
				break
			}
			currItem := items[itemIdx].GetItem()
			if w := currItem.Width(); w > maxLineWidth {
				maxLineWidth = w
			}
		}
	}

	return maxLineWidth
}

func (m *Model[T]) numLinesForItem(itemIdx int) int {
	if !m.config.wrapText {
		return 1
	}
	cw := m.contentWidth()
	if cw == 0 {
		return 0
	}
	if m.content.isEmpty() || itemIdx < 0 || itemIdx >= m.content.numItems() {
		return 0
	}
	items := m.content.objects
	return items[itemIdx].GetItem().NumWrappedLines(cw)
}

// contentWidth returns the width available for rendering content items.
// When selection is enabled and a SelectionPrefix is configured, the prefix
// reduces the available content width. Headers, footers, and other chrome
// use the full bounds.width instead.
func (m *Model[T]) contentWidth() int {
	if m.navigation.selectionEnabled && m.display.styles.SelectionPrefix != "" {
		pw := lipgloss.Width(m.display.styles.SelectionPrefix)
		return max(0, m.display.bounds.width-pw)
	}
	return m.display.bounds.width
}

// selectionPrefixPadding returns whitespace the same width as SelectionPrefix.
func (m *Model[T]) selectionPrefixPadding() string {
	if m.display.styles.SelectionPrefix == "" {
		return ""
	}
	return strings.Repeat(" ", lipgloss.Width(m.display.styles.SelectionPrefix))
}

func (m *Model[T]) setWidthHeight(width, height int) {
	if m.display.bounds.width == width && m.display.bounds.height == height {
		return
	}
	m.display.setBounds(rectangle{width: width, height: height})
	m.safelySetTopItemIdxAndOffset(m.display.topItemIdx, m.display.topItemLineOffset)
	if m.navigation.selectionEnabled {
		m.scrollSoSelectionInView()
	}
}

func (m *Model[T]) safelySetTopItemIdxAndOffset(topItemIdx, topItemLineOffset int) {
	maxTopItemIdx, maxTopItemLineOffset := m.maxItemIdxAndMaxTopLineOffset()
	if topItemIdx < 0 {
		topItemIdx = 0
		topItemLineOffset = 0
	}
	if topItemIdx > maxTopItemIdx {
		topItemIdx = maxTopItemIdx
		topItemLineOffset = maxTopItemLineOffset
	}
	if topItemIdx == maxTopItemIdx {
		topItemLineOffset = clampValZeroToMax(topItemLineOffset, maxTopItemLineOffset)
	}
	m.display.setTopItemIdxAndOffset(topItemIdx, topItemLineOffset)
}

// getNumContentLines returns the number of lines of between the header and footer/pre-footer
func (m *Model[T]) getNumContentLines() int {
	return m.display.getNumContentLines(len(m.getVisibleHeaderLines()), m.config.postHeaderLine != "", m.config.preFooterLine != "", true)
}

func (m *Model[T]) scrollSoSelectionInView() {
	if !m.navigation.selectionEnabled {
		panic("scrollSoSelectionInView called when selection is not enabled")
	}
	selectedItem := m.content.getSelectedItem()
	if selectedItem == nil {
		return
	}
	selectedItemWidth := (*selectedItem).GetItem().Width()
	startWidth := 0
	endWidth := selectedItemWidth
	if !m.config.wrapText && m.display.xOffset > 0 {
		if selectedItemWidth < m.display.xOffset {
			// ensure the selection is visible by scrolling, but maintain xOffset if possible
			prevXOffset := m.display.xOffset
			m.EnsureItemInView(m.content.selectedIdx, 0, 0, 0, 0)
			m.SetXOffset(prevXOffset)
			return
		}
		startWidth = m.display.xOffset
		endWidth = m.display.xOffset + m.contentWidth() - 1
	}
	m.EnsureItemInView(m.content.selectedIdx, startWidth, endWidth, 0, 0)
}

// getItemIdxAbove consumes n lines by moving up through items, returning the final item index and line offset
func (m *Model[T]) getItemIdxAbove(startItemIdx, startLineOffset, linesToConsume int) (finalItemIdx, finalLineOffset int) {
	itemIdx := startItemIdx
	lineOffset := startLineOffset
	remaining := linesToConsume

	for remaining > 0 {
		itemIdx--
		if itemIdx < 0 {
			return 0, 0
		}
		numLinesInItem := m.numLinesForItem(itemIdx)
		if remaining <= numLinesInItem {
			return itemIdx, numLinesInItem - remaining
		}
		remaining -= numLinesInItem
	}
	return itemIdx, lineOffset
}

// getItemIdxBelow consumes n lines by moving down through items, returning the final item index and line offset
func (m *Model[T]) getItemIdxBelow(startItemIdx, linesToConsume int) (finalItemIdx, finalLineOffset int) {
	itemIdx := startItemIdx
	remaining := linesToConsume

	for remaining > 0 {
		itemIdx++
		if itemIdx >= m.content.numItems() {
			return m.content.numItems() - 1, 0
		}
		numLinesInItem := m.numLinesForItem(itemIdx)
		if remaining <= numLinesInItem {
			return itemIdx, remaining - 1
		}
		remaining -= numLinesInItem
	}
	return itemIdx, 0
}

// scrollDownLines edits topItemIdx and topItemLineOffset to scroll the viewport by n lines (negative for up, positive for down)
func (m *Model[T]) scrollDownLines(numLinesDown int) {
	if numLinesDown == 0 {
		return
	}

	// scrolling down past bottom
	if numLinesDown > 0 && m.isScrolledToBottom() {
		return
	}

	// scrolling up past top
	if numLinesDown < 0 && m.isScrolledToTop() {
		return
	}

	newTopItemIdx, newTopItemLineOffset := m.display.topItemIdx, m.display.topItemLineOffset
	if !m.config.wrapText {
		newTopItemIdx = m.display.topItemIdx + numLinesDown
	} else {
		// wrapped
		if numLinesDown < 0 { // scrolling up
			if newTopItemLineOffset >= -numLinesDown {
				// same item, just change offset
				newTopItemLineOffset += numLinesDown
			} else {
				// need to scroll up through multiple items
				linesToConsume := -numLinesDown - newTopItemLineOffset
				newTopItemIdx, newTopItemLineOffset = m.getItemIdxAbove(newTopItemIdx, newTopItemLineOffset, linesToConsume)
			}
		} else { // scrolling down
			numLinesInTopItem := m.numLinesForItem(newTopItemIdx)
			if newTopItemLineOffset+numLinesDown < numLinesInTopItem {
				// same item, just change offset
				newTopItemLineOffset += numLinesDown
			} else {
				// need to scroll down through multiple items
				linesToConsume := numLinesDown - (numLinesInTopItem - (newTopItemLineOffset + 1))
				newTopItemIdx, newTopItemLineOffset = m.getItemIdxBelow(newTopItemIdx, linesToConsume)
			}
		}
	}
	m.safelySetTopItemIdxAndOffset(newTopItemIdx, newTopItemLineOffset)
	m.SetXOffset(m.display.xOffset)
}

// getVisibleHeaderLines returns the lines of header that are visible in the viewport as strings.
// header lines will take precedence over content and footer if there is not enough vertical height
func (m *Model[T]) getVisibleHeaderLines() []string {
	if m.display.bounds.height == 0 {
		return nil
	}

	headerItems := make([]item.Item, len(m.content.header))
	for i := range m.content.header {
		headerItems[i] = item.NewItem(m.content.header[i])
	}

	itemIndexes := m.getItemIndexesSpanningLines(
		0,
		0,
		m.display.bounds.height,
		len(headerItems),
		func(idx int) item.Item { return headerItems[idx] },
		m.display.bounds.width, // headers use full viewport width
	)

	headerLines := make([]string, len(itemIndexes))
	currentItemIdxWidthToLeft := 0
	for idx, itemIdx := range itemIndexes {
		var truncated string
		if m.config.wrapText {
			currentItemIdx := itemIndexes[idx]
			var widthTaken int
			truncated, widthTaken = headerItems[itemIdx].Take(
				currentItemIdxWidthToLeft,
				m.display.bounds.width,
				"",
				[]item.Highlight{}, // no highlights for header
			)
			if idx+1 < len(itemIndexes) {
				nextItemIdx := itemIndexes[idx+1]
				if nextItemIdx != currentItemIdx {
					currentItemIdxWidthToLeft = 0
				} else {
					currentItemIdxWidthToLeft += widthTaken
				}
			}
		} else {
			// if not wrapped, items are not yet truncated or highlighted
			truncated, _ = headerItems[itemIdx].Take(
				0, // header doesn't pan horizontally
				m.display.bounds.width,
				m.config.continuationIndicator,
				[]item.Highlight{}, // no highlights for header
			)
		}
		headerLines[idx] = truncated
	}

	return headerLines
}

// getVisibleContentItemIndexes returns the item indexes of content that are visible in the viewport
func (m *Model[T]) getVisibleContentItemIndexes() []int {
	if m.display.bounds.width == 0 || m.content.isEmpty() {
		return nil
	}

	linesUsedByHeader := len(m.getVisibleHeaderLines())
	if m.config.postHeaderLine != "" {
		linesUsedByHeader++ // post-header
	}
	numLinesAfterHeader := max(0, m.display.bounds.height-linesUsedByHeader)

	itemIndexes := m.getItemIndexesSpanningLines(
		m.display.topItemIdx,
		m.display.topItemLineOffset,
		numLinesAfterHeader,
		m.content.numItems(),
		func(idx int) item.Item {
			return m.content.objects[idx].GetItem()
		},
		m.contentWidth(), // content uses narrower width when selection prefix is configured
	)
	if len(itemIndexes) == 0 {
		return nil
	}

	reservedLines := 0
	if m.config.footerEnabled {
		reservedLines++ // footer
	}
	if m.config.preFooterLine != "" {
		reservedLines++ // pre-footer
	}
	if reservedLines > 0 {
		itemIndexes = safeSliceUpToIdx(itemIndexes, numLinesAfterHeader-reservedLines)
	}
	return itemIndexes
}

// getItemIndexesSpanningLines returns the item indexes for each line given a top item index, offset and num lines.
// wrapWidth is the width used for wrapping calculations (content width for content, bounds width for headers).
func (m *Model[T]) getItemIndexesSpanningLines(
	topItemIdx int,
	topItemLineOffset int,
	totalNumLines int,
	numItems int,
	getItem func(int) item.Item,
	wrapWidth int,
) []int {
	if numItems == 0 || totalNumLines == 0 {
		return nil
	}

	var itemIndexes []int

	addLine := func(itemIndex int) bool {
		itemIndexes = append(itemIndexes, itemIndex)
		return len(itemIndexes) == totalNumLines
	}

	currItemIdx := clampValZeroToMax(topItemIdx, numItems-1)

	currItem := getItem(currItemIdx)
	done := totalNumLines == 0
	if done {
		return itemIndexes
	}

	if m.config.wrapText {
		// first item has potentially fewer lines depending on the line offset
		numLines := max(0, currItem.NumWrappedLines(wrapWidth)-topItemLineOffset)
		for range numLines {
			// adding untruncated, unstyled items
			done = addLine(currItemIdx)
			if done {
				break
			}
		}

		for !done {
			currItemIdx++
			if currItemIdx >= numItems {
				done = true
			} else {
				currItem = getItem(currItemIdx)
				numLines = currItem.NumWrappedLines(wrapWidth)
				for range numLines {
					// adding untruncated, unstyled items
					done = addLine(currItemIdx)
					if done {
						break
					}
				}
			}
		}
	} else {
		done = addLine(currItemIdx)
		for !done {
			currItemIdx++
			if currItemIdx >= numItems {
				done = true
			} else {
				done = addLine(currItemIdx)
			}
		}
	}
	return itemIndexes
}

func (m *Model[T]) getTruncatedFooterLine(visibleContentItemIndexes []int) string {
	numerator := m.content.getSelectedIdx() + 1 // 0 indexed
	denominator := m.content.numItems()
	if denominator == 0 {
		return ""
	}
	if !m.config.footerEnabled {
		panic("getTruncatedFooterLine called when footer should not be shown")
	}
	if len(visibleContentItemIndexes) == 0 {
		return ""
	}

	var footerString string
	var percentScrolled int

	// if selection is disabled, numerator should be item index of bottom visible line
	if !m.navigation.selectionEnabled {
		numerator = visibleContentItemIndexes[len(visibleContentItemIndexes)-1] + 1
		if m.config.wrapText && numerator == denominator && !m.isScrolledToBottom() {
			// if wrapped && bottom visible line is max item index, but actually not fully scrolled to bottom, show 99%
			percentScrolled = 99
			footerString = fmt.Sprintf("99%% (%d/%d)", numerator, denominator)
		}
	}

	if footerString == "" {
		percentScrolled = percent(numerator, denominator)
		footerString = fmt.Sprintf("%d%% (%d/%d)", percentScrolled, numerator, denominator)
	}

	if m.config.progressBarEnabled {
		barSpace := m.display.bounds.width - len(footerString) - 1
		if barSpace >= 3 {
			barWidth := min(10, barSpace)
			footerString = buildProgressBar(percentScrolled, barWidth) + " " + footerString
		}
	}

	footerItem := item.NewItem(footerString)
	f, _ := footerItem.Take(0, m.display.bounds.width, m.config.continuationIndicator, []item.Highlight{})
	return m.display.styles.FooterStyle.Render(f)
}

func (m *Model[T]) isScrolledToBottom() bool {
	maxItemIdx, maxTopItemLineOffset := m.maxItemIdxAndMaxTopLineOffset()
	if m.display.topItemIdx > maxItemIdx {
		return true
	}
	if m.display.topItemIdx == maxItemIdx {
		return m.display.topItemLineOffset >= maxTopItemLineOffset
	}
	return false
}

// isScrolledToTop returns true if the viewport is scrolled to the very top
func (m *Model[T]) isScrolledToTop() bool {
	return m.display.topItemIdx == 0 && m.display.topItemLineOffset == 0
}

type selectionInViewInfoResult struct {
	numLinesSelectionInView int
	numLinesAboveSelection  int
}

func (m *Model[T]) selectionInViewInfo() selectionInViewInfoResult {
	if !m.navigation.selectionEnabled {
		panic("selectionInViewInfo called when selection is disabled")
	}
	itemIndexes := m.getVisibleContentItemIndexes()
	numLinesSelectionInView := 0
	numLinesAboveSelection := 0
	assignedNumLinesAboveSelection := false
	for i := range itemIndexes {
		if itemIndexes[i] == m.content.getSelectedIdx() {
			if !assignedNumLinesAboveSelection {
				numLinesAboveSelection = i
				assignedNumLinesAboveSelection = true
			}
			numLinesSelectionInView++
		}
	}
	return selectionInViewInfoResult{
		numLinesSelectionInView: numLinesSelectionInView,
		numLinesAboveSelection:  numLinesAboveSelection,
	}
}

func (m *Model[T]) maxItemIdxAndMaxTopLineOffset() (int, int) {
	numItems := m.content.numItems()
	if numItems == 0 {
		return 0, 0
	}

	headerLines := len(m.getVisibleHeaderLines())
	if m.config.postHeaderLine != "" {
		headerLines++ // post-header
	}
	reservedLines := 1 // footer
	if m.config.preFooterLine != "" {
		reservedLines++ // pre-footer
	}
	numContentLines := max(0, m.display.bounds.height-headerLines-reservedLines)

	if !m.config.wrapText {
		return max(0, numItems-numContentLines), 0
	}

	// wrapped
	maxTopItemIdx, maxTopItemLineOffset := numItems-1, 0
	numLinesLastItem := m.numLinesForItem(numItems - 1)
	if numContentLines <= numLinesLastItem {
		// last item takes up whole screen or more, adjust offset accordingly
		maxTopItemLineOffset = numLinesLastItem - numContentLines
	} else {
		// need to scroll up through multiple items to fill the screen
		linesToConsume := numContentLines - numLinesLastItem
		maxTopItemIdx, maxTopItemLineOffset = m.getItemIdxAbove(maxTopItemIdx, maxTopItemLineOffset, linesToConsume)
	}
	return max(0, maxTopItemIdx), max(0, maxTopItemLineOffset)
}

// getHighlightsForItem returns highlights for the specific item index
func (m *Model[T]) getHighlightsForItem(itemIndex int) []item.Highlight {
	return m.content.getItemHighlightsForItem(itemIndex)
}

func (m *Model[T]) getNumVisibleItems() int {
	if !m.config.wrapText {
		return m.getNumContentLines()
	}
	itemIndexes := m.getVisibleContentItemIndexes()
	// return distinct number of items
	itemIndexSet := make(map[int]struct{})
	for _, i := range itemIndexes {
		itemIndexSet[i] = struct{}{}
	}
	return len(itemIndexSet)
}

// selectionHighlights returns highlights that fill gaps between existing match
// highlights with the selection style, so that the selection background covers
// the entire item while match highlights remain visible on top.
func (m *Model[T]) selectionHighlights(itemIdx int, matchHighlights []item.Highlight) []item.Highlight {
	itemLen := len(m.content.objects[itemIdx].GetItem().ContentNoAnsi())
	if itemLen == 0 {
		return matchHighlights
	}

	// sort match highlights by start position
	sorted := make([]item.Highlight, len(matchHighlights))
	copy(sorted, matchHighlights)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].ByteRangeUnstyledContent.Start < sorted[i].ByteRangeUnstyledContent.Start {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// fill gaps between match highlights with selection style
	var result []item.Highlight
	pos := 0
	for _, h := range sorted {
		if h.ByteRangeUnstyledContent.Start > pos {
			result = append(result, item.Highlight{
				Style:                    m.display.styles.SelectedItemStyle,
				ByteRangeUnstyledContent: item.ByteRange{Start: pos, End: h.ByteRangeUnstyledContent.Start},
			})
		}
		result = append(result, h)
		pos = h.ByteRangeUnstyledContent.End
	}
	if pos < itemLen {
		result = append(result, item.Highlight{
			Style:                    m.display.styles.SelectedItemStyle,
			ByteRangeUnstyledContent: item.ByteRange{Start: pos, End: itemLen},
		})
	}
	return result
}

// styleSelection applies the selection style to unstyled portions of the string,
// preserving any existing ANSI styling. Used when selectionStyleOverridesItemStyle is false.
func (m *Model[T]) styleSelection(selection string) string {
	split := surroundingAnsiRegex.Split(selection, -1)
	matches := surroundingAnsiRegex.FindAllString(selection, -1)
	var builder strings.Builder
	builder.Grow(len(selection))

	for i, section := range split {
		if section != "" {
			builder.WriteString(m.display.styles.SelectedItemStyle.Render(section))
		}
		if i < len(split)-1 && i < len(matches) {
			builder.WriteString(matches[i])
		}
	}
	return builder.String()
}

// fileSavedMsg is returned when file saving completes.
type fileSavedMsg struct {
	filename string // full path to saved file
	err      error  // error if save failed, nil on success
}

// clearSaveResultMsg is sent after some seconds to clear the save result display
type clearSaveResultMsg struct{}

// saveToFile saves all viewport objects to a file with the given filename.
func (m *Model[T]) saveToFile(filename string) tea.Cmd {
	return func() tea.Msg {
		// create directory if needed
		if err := os.MkdirAll(m.config.saveDir, 0750); err != nil {
			return fileSavedMsg{err: fmt.Errorf("failed to create directory %s: %w", m.config.saveDir, err)}
		}

		fullPath := filepath.Join(m.config.saveDir, filename)

		// collect content without ANSI codes
		var content strings.Builder
		for _, obj := range m.content.objects {
			content.WriteString(obj.GetItem().ContentNoAnsi())
			content.WriteString("\n")
		}

		if err := os.WriteFile(fullPath, []byte(content.String()), 0600); err != nil {
			return fileSavedMsg{err: fmt.Errorf("failed to write file: %w", err)}
		}

		return fileSavedMsg{filename: fullPath, err: nil}
	}
}

// decomposeLineOffset converts a line offset within an item into
// (segmentIdx, wrapOffset) given the item's line-broken items.
// segmentIdx is which line-broken item, wrapOffset is how many wrapped lines
// into that segment. For single-line items: returns (0, lineOffset).
func decomposeLineOffset(segments []item.Item, lineOffset, wrapWidth int) (segmentIdx, wrapOffset int) {
	remaining := lineOffset
	for i, seg := range segments {
		n := seg.NumWrappedLines(wrapWidth)
		if remaining < n {
			return i, remaining
		}
		remaining -= n
	}
	if len(segments) == 0 {
		return 0, 0
	}
	return len(segments) - 1, 0
}

// remapHighlightsForSegment clips and adjusts highlight byte ranges from the full
// item's content space to a specific line-broken item's content space.
// Highlights that don't overlap the segment are dropped.
func remapHighlightsForSegment(highlights []item.Highlight, segments []item.Item, segIdx int) []item.Highlight {
	if len(segments) <= 1 {
		// single-segment item: highlights are already in the right space
		return highlights
	}

	// compute byte offset of this segment in the full concatenated content
	startByte := 0
	for i := range segIdx {
		startByte += len(segments[i].ContentNoAnsi())
		startByte++ // \n separator
	}
	endByte := startByte + len(segments[segIdx].ContentNoAnsi())

	var result []item.Highlight
	for _, h := range highlights {
		br := h.ByteRangeUnstyledContent
		if br.End <= startByte || br.Start >= endByte {
			continue
		}
		adjusted := h
		adjusted.ByteRangeUnstyledContent.Start = max(0, br.Start-startByte)
		adjusted.ByteRangeUnstyledContent.End = min(endByte-startByte, br.End-startByte)
		result = append(result, adjusted)
	}
	return result
}

// lineOffsetForCellPosition converts a cumulative cell position across
// line-broken items into a line offset. For single-line items: cellPos / wrapWidth.
func lineOffsetForCellPosition(segments []item.Item, cellPos, wrapWidth int) int {
	if len(segments) <= 1 || wrapWidth <= 0 {
		if wrapWidth <= 0 {
			return 0
		}
		return cellPos / wrapWidth
	}
	cumCells := 0
	lineOffset := 0
	for _, seg := range segments {
		segWidth := seg.Width()
		if cumCells+segWidth > cellPos {
			if wrapWidth > 0 {
				lineOffset += (cellPos - cumCells) / wrapWidth
			}
			return lineOffset
		}
		cumCells += segWidth
		lineOffset += seg.NumWrappedLines(wrapWidth)
	}
	return max(0, lineOffset-1)
}

func percent(a, b int) int {
	if b == 0 {
		return 100
	}
	return int(float32(a) / float32(b) * 100)
}

// buildProgressBar returns a string of exactly barWidth cells using U+2588 (█)
// for the filled portion and U+2591 (░) for the empty portion.
func buildProgressBar(percentScrolled, barWidth int) string {
	if barWidth <= 0 {
		return ""
	}
	filled := min(int(float64(barWidth)*float64(percentScrolled)/100.0), barWidth)
	return strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
}

func safeSliceUpToIdx[T any](s []T, i int) []T {
	if i > len(s) {
		return s
	}
	if i < 0 {
		return []T{}
	}
	return s[:i]
}

func clampValZeroToMax(v, maximum int) int {
	return max(0, min(maximum, v))
}
