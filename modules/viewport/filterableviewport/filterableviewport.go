package filterableviewport

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

type filterMode int

const (
	filterModeOff filterMode = iota
	filterModeEditing
	filterModeApplied
)

// FilterLinePosition controls where the filter line is rendered
type FilterLinePosition int

const (
	// FilterLineBottom renders the filter line just above the footer (default)
	FilterLineBottom FilterLinePosition = iota

	// FilterLineTop renders the filter line just below the header
	FilterLineTop
)

// Option is a functional option for configuring the filterable viewport
type Option[T viewport.Object] func(*Model[T])

// WithKeyMap sets the key mapping for the viewport
func WithKeyMap[T viewport.Object](keyMap KeyMap) Option[T] {
	return func(m *Model[T]) {
		m.keyMap = keyMap
	}
}

// WithStyles sets the styles for the filterable viewport
func WithStyles[T viewport.Object](styles Styles) Option[T] {
	return func(m *Model[T]) {
		m.styles = styles
	}
}

// WithPrefixText sets the prefix text for the filter line
func WithPrefixText[T viewport.Object](prefix string) Option[T] {
	return func(m *Model[T]) {
		m.prefixText = prefix
	}
}

// WithEmptyText sets the text to display when the filter is empty
func WithEmptyText[T viewport.Object](whenEmpty string) Option[T] {
	return func(m *Model[T]) {
		m.emptyText = whenEmpty
	}
}

// WithMatchingItemsOnly sets whether to show only the matching items
func WithMatchingItemsOnly[T viewport.Object](matchingItemsOnly bool) Option[T] {
	return func(m *Model[T]) {
		m.matchingItemsOnly = matchingItemsOnly
	}
}

// WithCanToggleMatchingItemsOnly sets whether this viewport can toggle matching items only mode
func WithCanToggleMatchingItemsOnly[T viewport.Object](canToggleMatchingItemsOnly bool) Option[T] {
	return func(m *Model[T]) {
		m.canToggleMatchingItemsOnly = canToggleMatchingItemsOnly
	}
}

// WithVerticalPad sets the number of lines of context to keep above/below the focused match (scrolloff)
func WithVerticalPad[T viewport.Object](verticalPad int) Option[T] {
	return func(m *Model[T]) {
		m.verticalPad = verticalPad
	}
}

// WithHorizontalPad sets the number of columns of context to keep left/right of the focused match (panoff)
func WithHorizontalPad[T viewport.Object](horizontalPad int) Option[T] {
	return func(m *Model[T]) {
		m.horizontalPad = horizontalPad
	}
}

// WithMaxMatchLimit sets the maximum number of matches when searching.
// When this limit is exceeded, match highlighting and navigation are disabled
// and all items are shown regardless of matchingItemsOnly setting.
// Set to 0 for unlimited matches. Default is 30000.
func WithMaxMatchLimit[T viewport.Object](maxMatchLimit int) Option[T] {
	return func(m *Model[T]) {
		m.maxMatchLimit = maxMatchLimit
	}
}

// WithAdjustObjectsForFilter sets a function that returns the visible filterable viewport objects
// based on the current filter. It's called internally whenever the filter changes. Use this when
// your visible objects depend on the filter in complex ways—for example, a tree view where matching
// one node should also show parent and child nodes. Return nil to keep the current objects unmodified.
// This is independent behavior from SetMatchingItemsOnly - when showing matching items only, the filterable viewport
// will still call this function to determine which items to show, but it will also filter that list down to matching
// items only. See tests for concrete examples of use.
func WithAdjustObjectsForFilter[T viewport.Object](fn func(filterText string, mode FilterModeName) []T) Option[T] {
	return func(m *Model[T]) {
		m.adjustObjectsForFilter = fn
	}
}

// WithFilterModes sets the filter modes for the filterable viewport.
// If not provided, New() defaults to DefaultFilterModes().
func WithFilterModes[T viewport.Object](modes []FilterMode) Option[T] {
	return func(m *Model[T]) {
		m.filterModes = modes
	}
}

// WithFilterLinePosition sets whether the filter line renders at the top (below header) or bottom (above footer)
func WithFilterLinePosition[T viewport.Object](position FilterLinePosition) Option[T] {
	return func(m *Model[T]) {
		m.filterLinePosition = position
	}
}

// WithFilterLinePrefix sets a string that is always prepended to the filter line, regardless of filter state.
func WithFilterLinePrefix[T viewport.Object](prefix string) Option[T] {
	return func(m *Model[T]) {
		m.filterLinePrefix = prefix
	}
}

// WithItemDescriptor sets a word describing the items (e.g. "logs", "events").
// When set, match count text includes the total item count: "4/5 matches on 10 logs".
// When empty (default), just "4/5 matches" is shown.
func WithItemDescriptor[T viewport.Object](descriptor string) Option[T] {
	return func(m *Model[T]) {
		m.itemDescriptor = descriptor
	}
}

// SetFilterLinePrefix updates the string prepended to the filter line and re-renders it.
func (m *Model[T]) SetFilterLinePrefix(prefix string) {
	m.filterLinePrefix = prefix
	m.setFilterLine(m.renderFilterLine())
}

// SetAdjustObjectsForFilter updates the function used to adjust visible objects when the filter changes.
func (m *Model[T]) SetAdjustObjectsForFilter(fn func(filterText string, mode FilterModeName) []T) {
	m.adjustObjectsForFilter = fn
}

// Model is the state and logic for a filterable viewport
type Model[T viewport.Object] struct {
	vp *viewport.Model[T]

	keyMap                   KeyMap
	filterTextInput          textinput.Model
	filterMode               filterMode
	prefixText               string
	emptyText                string
	filterLinePosition       FilterLinePosition
	filterLinePrefix         string
	objects                  []T
	filterModes              []FilterMode
	filterModesByName        map[FilterModeName]int // name -> index in filterModes
	activeFilterModeName     FilterModeName         // "" when no mode active
	lastActiveFilterModeName FilterModeName
	styles                   Styles

	itemDescriptor             string
	matchingItemsOnly          bool
	canToggleMatchingItemsOnly bool
	allMatches                 []viewport.Highlight
	numMatchingItems           int
	focusedMatchIdx            int
	previousFocusedMatchIdx    int
	totalMatchesOnAllItems     int
	itemIdxToFilteredIdx       map[int]int
	matchWidthsByMatchIdx      map[int]item.WidthRange
	lastFilterValue            string
	maxMatchLimit              int // 0 = unlimited
	matchLimitExceeded         bool
	adjustObjectsForFilter     func(filterText string, mode FilterModeName) []T

	verticalPad   int
	horizontalPad int

	searchHistory      []string // oldest at 0, newest at end
	searchHistoryIdx   int      // index into searchHistory; == len(searchHistory) means "at draft"
	searchHistoryDraft string   // current unsaved input preserved while browsing
}

// New creates a new filterable viewport model with default configuration
func New[T viewport.Object](vp *viewport.Model[T], opts ...Option[T]) *Model[T] {
	ti := textinput.New()
	ti.CharLimit = 0
	ti.Prompt = ""
	// Use unstyled text so the filter line doesn't include ANSI color codes
	// from the textinput's default dark theme styling.
	tiStyles := ti.Styles()
	tiStyles.Focused.Text = lipgloss.NewStyle()
	tiStyles.Blurred.Text = lipgloss.NewStyle()
	tiStyles.Focused.Placeholder = lipgloss.NewStyle()
	tiStyles.Blurred.Placeholder = lipgloss.NewStyle()
	ti.SetStyles(tiStyles)

	defaultKeyMap := DefaultKeyMap()
	defaultStyles := DefaultStyles()

	m := &Model[T]{
		vp:                         vp,
		keyMap:                     defaultKeyMap,
		filterTextInput:            ti,
		filterMode:                 filterModeOff,
		prefixText:                 "",
		emptyText:                  "No Filter",
		objects:                    []T{},
		filterModes:                DefaultFilterModes(),
		activeFilterModeName:       "",
		lastActiveFilterModeName:   "",
		styles:                     defaultStyles,
		matchingItemsOnly:          false,
		canToggleMatchingItemsOnly: true,
		allMatches:                 []viewport.Highlight{},
		numMatchingItems:           0,
		focusedMatchIdx:            -1,
		previousFocusedMatchIdx:    -1,
		totalMatchesOnAllItems:     0,
		itemIdxToFilteredIdx:       make(map[int]int),
		matchWidthsByMatchIdx:      make(map[int]item.WidthRange),
		lastFilterValue:            "",
		maxMatchLimit:              30000, // reasonable default
		matchLimitExceeded:         false,
		verticalPad:                0,
		horizontalPad:              0,
		searchHistory:              []string{},
		searchHistoryIdx:           0,
	}
	m.SetHeight(vp.GetHeight())

	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}

	// validate that at least one filter mode is set
	if len(m.filterModes) == 0 {
		panic("filterableviewport: no filter modes set; use viewport.Model directly if filtering is not needed")
	}

	// build name -> index lookup and validate uniqueness
	m.filterModesByName = make(map[FilterModeName]int, len(m.filterModes))
	for i, mode := range m.filterModes {
		if mode.Name == "" {
			panic(fmt.Sprintf("filterableviewport: FilterMode at index %d has empty Name", i))
		}
		if _, exists := m.filterModesByName[mode.Name]; exists {
			panic(fmt.Sprintf("filterableviewport: duplicate FilterModeName %q", mode.Name))
		}
		m.filterModesByName[mode.Name] = i
	}

	// set initial pre-footer line
	m.setFilterLine(m.renderFilterLine())

	return m
}

// Init initializes the filterable viewport model
func (m *Model[T]) Init() tea.Cmd {
	return nil
}

// Update processes messages and updates the model state
func (m *Model[T]) Update(msg tea.Msg) (*Model[T], tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.vp.IsCapturingInput() {
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// check if any filter mode key matches
		if m.filterMode != filterModeEditing {
			for i := range m.filterModes {
				if key.Matches(msg, m.filterModes[i].Key) {
					m.activeFilterModeName = m.filterModes[i].Name
					m.filterTextInput.Focus()
					m.filterMode = filterModeEditing
					m.resetSearchHistoryBrowsing()
					m.updateMatchingItems()
					m.ensureCurrentMatchInView()
					return m, textinput.Blink
				}
			}
		}

		switch {
		case key.Matches(msg, m.keyMap.ApplyFilterKey):
			if m.filterMode == filterModeEditing {
				m.addToSearchHistory(m.filterTextInput.Value())
				m.filterTextInput.Blur()
				m.filterMode = filterModeApplied
				m.resetSearchHistoryBrowsing()
				m.updateMatchingItems()
				m.ensureCurrentMatchInView()
				return m, nil
			}
		case key.Matches(msg, m.keyMap.ToggleMatchingItemsOnlyKey):
			if m.filterMode != filterModeEditing && m.canToggleMatchingItemsOnly {
				m.matchingItemsOnly = !m.matchingItemsOnly
				m.updateMatchingItems()
				m.ensureCurrentMatchInView()
				return m, nil
			}
		case key.Matches(msg, m.keyMap.NextMatchKey):
			if m.filterMode != filterModeEditing && m.filterMode != filterModeOff && len(m.allMatches) > 0 {
				m.navigateToNextMatch()
				return m, nil
			}
		case key.Matches(msg, m.keyMap.PrevMatchKey):
			if m.filterMode != filterModeEditing && m.filterMode != filterModeOff && len(m.allMatches) > 0 {
				m.navigateToPrevMatch()
				return m, nil
			}
		case key.Matches(msg, m.keyMap.CancelFilterKey):
			m.filterMode = filterModeOff
			m.activeFilterModeName = ""
			m.filterTextInput.Blur()
			m.filterTextInput.SetValue("")
			m.resetSearchHistoryBrowsing()
			m.updateMatchingItems()
			m.ensureCurrentMatchInView()
			return m, nil
		case key.Matches(msg, m.keyMap.SearchHistoryPrevKey):
			if m.filterMode == filterModeEditing && len(m.searchHistory) > 0 {
				m.navigateSearchHistoryPrev()
				m.updateMatchingItems()
				m.ensureCurrentMatchInView()
				return m, nil
			}
		case key.Matches(msg, m.keyMap.SearchHistoryNextKey):
			if m.filterMode == filterModeEditing && m.searchHistoryIdx < len(m.searchHistory) {
				m.navigateSearchHistoryNext()
				m.updateMatchingItems()
				m.ensureCurrentMatchInView()
				return m, nil
			}
		}
	}

	if m.filterMode != filterModeEditing {
		prevSelectedIdx := m.vp.GetSelectedItemIdx()
		m.vp, cmd = m.vp.Update(msg)
		cmds = append(cmds, cmd)
		// when the selection moves, re-evaluate focused match highlight style
		// since it differs depending on whether the focused match is on the selected item
		if m.vp.GetSelectedItemIdx() != prevSelectedIdx && len(m.allMatches) > 0 {
			m.updateFocusedMatchHighlight()
		}
	} else {
		m.filterTextInput, cmd = m.filterTextInput.Update(msg)
		m.updateMatchingItems()
		m.ensureCurrentMatchInView()
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the filterable viewport model as a string
func (m *Model[T]) View() string {
	return m.vp.View()
}

// GetWidth returns the width of the filterable viewport
func (m *Model[T]) GetWidth() int {
	return m.vp.GetWidth()
}

// SetWidth updates the width of both the viewport and textinput
func (m *Model[T]) SetWidth(width int) {
	m.vp.SetWidth(width)
	m.setFilterLine(m.renderFilterLine())
}

// GetHeight returns the height of the filterable viewport
func (m *Model[T]) GetHeight() int {
	return m.vp.GetHeight()
}

// SetHeight updates the height of the filterable viewport
func (m *Model[T]) SetHeight(height int) {
	m.vp.SetHeight(height)
}

// SetObjects sets the viewport objects
func (m *Model[T]) SetObjects(objects []T) {
	if objects == nil {
		objects = []T{}
	}
	m.objects = objects
	m.updateMatchingItems()
}

// AppendObjects appends objects to the viewport's existing objects
func (m *Model[T]) AppendObjects(objects []T) {
	if objects == nil {
		return
	}
	startIdx := len(m.objects)
	m.objects = append(m.objects, objects...)

	// if filter active and not at limit, do incremental update
	if m.filterMode != filterModeOff &&
		m.filterTextInput.Value() != "" &&
		!m.matchLimitExceeded {
		m.appendMatchesForNewObjects(startIdx, objects)
	} else if m.matchLimitExceeded {
		// already at limit, just update viewport with all objects
		m.vp.SetObjects(m.objects)
	} else {
		m.updateMatchingItems()
	}
}

// FilterFocused returns true if the filter text input is focused
func (m *Model[T]) FilterFocused() bool {
	return m.filterTextInput.Focused()
}

// IsCapturingInput returns true when the filterableviewport or its underlying
// viewport is capturing input (e.g., filter entry, filename entry). Callers
// should check this before processing their own key bindings.
func (m *Model[T]) IsCapturingInput() bool {
	return m.filterTextInput.Focused() || m.vp.IsCapturingInput()
}

// GetWrapText returns whether text wrapping is enabled in the viewport
func (m *Model[T]) GetWrapText() bool {
	return m.vp.GetWrapText()
}

// SetWrapText sets whether text wrapping is enabled in the viewport
func (m *Model[T]) SetWrapText(wrapText bool) {
	m.vp.SetWrapText(wrapText)
}

// GetSelectionEnabled returns whether selection is enabled in the viewport
func (m *Model[T]) GetSelectionEnabled() bool {
	return m.vp.GetSelectionEnabled()
}

// SetSelectionEnabled sets whether selection is enabled in the viewport
func (m *Model[T]) SetSelectionEnabled(selectionEnabled bool) {
	m.vp.SetSelectionEnabled(selectionEnabled)
}

// GetFilterText returns the current filter text
func (m *Model[T]) GetFilterText() string {
	return m.filterTextInput.Value()
}

// GetActiveFilterMode returns the currently active filter mode, or nil if none.
func (m *Model[T]) GetActiveFilterMode() *FilterMode {
	idx, ok := m.filterModesByName[m.activeFilterModeName]
	if !ok {
		return nil
	}
	return &m.filterModes[idx]
}

// FilterModes returns the configured filter modes.
func (m *Model[T]) FilterModes() []FilterMode {
	return m.filterModes
}

// GetSelectedItem returns the currently selected item, or nil if no selection
func (m *Model[T]) GetSelectedItem() *T {
	return m.vp.GetSelectedItem()
}

// GetSelectedItemIdx returns the index of the currently selected item
func (m *Model[T]) GetSelectedItemIdx() int {
	return m.vp.GetSelectedItemIdx()
}

// SetSelectedItemIdx sets the selected item index
func (m *Model[T]) SetSelectedItemIdx(idx int) {
	m.vp.SetSelectedItemIdx(idx)
}

// SetTopSticky sets whether selection sticks to the top
func (m *Model[T]) SetTopSticky(topSticky bool) {
	m.vp.SetTopSticky(topSticky)
}

// SetBottomSticky sets whether selection sticks to the bottom
func (m *Model[T]) SetBottomSticky(bottomSticky bool) {
	m.vp.SetBottomSticky(bottomSticky)
}

// SetHeader sets the viewport header lines
func (m *Model[T]) SetHeader(header []string) {
	m.vp.SetHeader(header)
}

// SetSelectionComparator sets the function used to maintain selection across object updates
func (m *Model[T]) SetSelectionComparator(compareFn viewport.CompareFn[T]) {
	m.vp.SetSelectionComparator(compareFn)
}

// SetFilter sets the filter text and mode programmatically.
// Use the FilterModeName constants (e.g. FilterExact, FilterRegex) or your own custom names.
func (m *Model[T]) SetFilter(value string, mode FilterModeName) {
	m.filterTextInput.SetValue(value)
	if _, ok := m.filterModesByName[mode]; ok {
		m.activeFilterModeName = mode
	}
	if value != "" && m.filterMode == filterModeOff {
		m.filterMode = filterModeApplied
	} else if value == "" {
		m.filterMode = filterModeOff
		m.activeFilterModeName = ""
	}
	m.updateMatchingItems()
	m.ensureCurrentMatchInView()
}

// GetMatchingItemsOnly returns whether only matching items are shown
func (m *Model[T]) GetMatchingItemsOnly() bool {
	return m.matchingItemsOnly
}

// SetMatchingItemsOnly sets whether to show only matching items
func (m *Model[T]) SetMatchingItemsOnly(matchingItemsOnly bool) {
	m.matchingItemsOnly = matchingItemsOnly
	m.updateMatchingItems()
}

// SetFilterableViewportStyles sets the styles for the filterable viewport
func (m *Model[T]) SetFilterableViewportStyles(styles Styles) {
	m.styles = styles
	// re-apply highlights with new styles
	m.updateFocusedMatchHighlight()
}

// SetViewportStyles sets styles on the underlying viewport
func (m *Model[T]) SetViewportStyles(styles viewport.Styles) {
	m.vp.SetStyles(styles)
}

// updateMatchingItems recalculates the matching items and updates match tracking
func (m *Model[T]) updateMatchingItems() {
	matchingObjects, filterChanged := m.getMatchingObjectsAndUpdateMatches()

	if !m.matchLimitExceeded {
		m.numMatchingItems = len(matchingObjects)
	}

	// when match limit exceeded, show all objects
	if m.showMatchesOnly() {
		m.vp.SetObjects(matchingObjects)
	} else {
		m.vp.SetObjects(m.objects)
	}

	// when no matches found with an active filter and items are unwrapped, reset horizontal scroll
	if m.totalMatchesOnAllItems == 0 && m.filterMode != filterModeOff && m.filterTextInput.Value() != "" && !m.vp.GetWrapText() {
		m.vp.SetXOffset(0)
	}

	// when the filter changed, move selection to the focused match
	if filterChanged {
		m.setSelectionToCurrentMatch()
	}
	m.updateFocusedMatchHighlight()

	// update the pre-footer line with the current filter state
	m.setFilterLine(m.renderFilterLine())
}

// updateFocusedMatchHighlight sets a specific highlight for the currently focused match
func (m *Model[T]) updateFocusedMatchHighlight() {
	if m.focusedMatchIdx < 0 || m.focusedMatchIdx >= len(m.allMatches) {
		m.vp.SetHighlights(nil)
		return
	}

	selectedIdx := m.vp.GetSelectedItemIdx()

	// if only focus changed, update only the affected highlights
	if m.previousFocusedMatchIdx >= 0 && m.previousFocusedMatchIdx < len(m.allMatches) &&
		m.focusedMatchIdx != m.previousFocusedMatchIdx &&
		len(m.allMatches) > 0 {
		currentHighlights := m.vp.GetHighlights()
		if len(currentHighlights) == len(m.allMatches) {
			if m.previousFocusedMatchIdx < len(currentHighlights) {
				currentHighlights[m.previousFocusedMatchIdx].ItemHighlight.Style = m.styles.Match.Unfocused
			}
			if m.focusedMatchIdx < len(currentHighlights) {
				focusedItemIdx := m.allMatches[m.focusedMatchIdx].ItemIndex
				if m.matchingItemsOnly {
					if filteredIdx, ok := m.itemIdxToFilteredIdx[focusedItemIdx]; ok {
						focusedItemIdx = filteredIdx
					}
				}
				if m.vp.GetSelectionEnabled() && focusedItemIdx == selectedIdx {
					currentHighlights[m.focusedMatchIdx].ItemHighlight.Style = m.styles.Match.FocusedIfSelected
				} else {
					currentHighlights[m.focusedMatchIdx].ItemHighlight.Style = m.styles.Match.Focused
				}
			}
			m.vp.SetHighlights(currentHighlights)
			m.previousFocusedMatchIdx = m.focusedMatchIdx
			return
		}
	}

	// otherwise, rebuild all highlights
	highlights := make([]viewport.Highlight, len(m.allMatches))
	for matchIdx, match := range m.allMatches {
		itemIdx := match.ItemIndex
		if m.matchingItemsOnly {
			if filteredIdx, ok := m.itemIdxToFilteredIdx[itemIdx]; ok {
				itemIdx = filteredIdx
			} else {
				panic("focused match item index not found in filtered items")
			}
		}
		style := m.styles.Match.Unfocused
		if matchIdx == m.focusedMatchIdx {
			if m.vp.GetSelectionEnabled() && itemIdx == selectedIdx {
				style = m.styles.Match.FocusedIfSelected
			} else {
				style = m.styles.Match.Focused
			}
		}
		highlight := viewport.Highlight{
			ItemIndex: itemIdx,
			ItemHighlight: item.Highlight{
				Style:                    style,
				ByteRangeUnstyledContent: match.ItemHighlight.ByteRangeUnstyledContent,
			},
		}
		highlights[matchIdx] = highlight
	}

	m.vp.SetHighlights(highlights)
	m.previousFocusedMatchIdx = m.focusedMatchIdx
}

func (m *Model[T]) renderFilterLine() string {
	var filterContent string

	switch m.filterMode {
	case filterModeOff:
		filterContent = m.emptyText
	case filterModeEditing, filterModeApplied:
		if m.filterTextInput.Value() == "" && m.filterMode == filterModeApplied {
			filterContent = m.emptyText
		} else {
			filterContent = strings.Join(removeEmpty([]string{
				m.getModeIndicator(),
				m.prefixText,
				m.filterTextInput.View(),
				m.getTextAfterFilter(),
				matchingItemsOnlyText(m.showMatchesOnly()),
			}),
				" ",
			)
		}
	default:
		panic(fmt.Sprintf("invalid filter mode: %d", m.filterMode))
	}

	filterLine := strings.Join(removeEmpty([]string{m.filterLinePrefix, filterContent}), " ")
	filterItem := item.NewItem(filterLine)
	res, _ := filterItem.Take(0, m.GetWidth(), "...", []item.Highlight{})
	return res
}

// setFilterLine sets the rendered filter line on the appropriate viewport line based on position
func (m *Model[T]) setFilterLine(line string) {
	switch m.filterLinePosition {
	case FilterLineBottom:
		m.vp.SetPreFooterLine(line)
	case FilterLineTop:
		m.vp.SetPostHeaderLine(line)
	}
}

func (m *Model[T]) getModeIndicator() string {
	if mode := m.GetActiveFilterMode(); mode != nil {
		return mode.Label
	}
	return ""
}

// getMatchingObjectsAndUpdateMatches filters objects and updates match tracking.
// Returns the matching objects and whether the filter value changed.
func (m *Model[T]) getMatchingObjectsAndUpdateMatches() ([]T, bool) {
	filterValue := m.filterTextInput.Value()
	filterChanged := filterValue != m.lastFilterValue || m.activeFilterModeName != m.lastActiveFilterModeName
	m.lastFilterValue = filterValue
	m.lastActiveFilterModeName = m.activeFilterModeName

	if filterChanged && m.adjustObjectsForFilter != nil {
		modeName := m.activeFilterModeName
		if modeName == "" && len(m.filterModes) > 0 {
			modeName = m.filterModes[0].Name
		}
		if newObjects := m.adjustObjectsForFilter(filterValue, modeName); newObjects != nil {
			m.objects = newObjects
		}
	}

	m.allMatches = []viewport.Highlight{}
	prevFocusedMatchIdx := m.focusedMatchIdx
	m.focusedMatchIdx = -1
	m.totalMatchesOnAllItems = 0
	m.itemIdxToFilteredIdx = make(map[int]int)
	m.matchLimitExceeded = false

	if m.filterMode == filterModeOff || filterValue == "" {
		return m.objects, filterChanged
	}

	// get the MatchFunc from the active mode
	var matchFn MatchFunc
	if mode := m.GetActiveFilterMode(); mode != nil {
		var err error
		matchFn, err = mode.GetMatchFunc(filterValue)
		if err != nil {
			return []T{}, filterChanged
		}
	}
	if matchFn == nil {
		return m.objects, filterChanged
	}

	var highlights []viewport.Highlight
	matchIdx := 0
	totalMatchCount := 0
	maxReached := false
	itemsWithMatchesSet := make(map[int]bool)

	for itemIdx := range m.objects {
		matches := m.extractMatches(m.objects[itemIdx], matchFn)

		if len(matches) > 0 {
			itemsWithMatchesSet[itemIdx] = true
		}

		if m.maxMatchLimit > 0 && totalMatchCount+len(matches) > m.maxMatchLimit {
			maxReached = true
			break
		}

		totalMatchCount += len(matches)

		newHighlights := m.buildHighlightsFromMatches(itemIdx, matches, matchIdx)
		matchIdx += len(matches)
		highlights = append(highlights, newHighlights...)
	}

	m.matchLimitExceeded = maxReached

	if maxReached {
		// clear match state and return all objects - no highlighting or navigation when limit exceeded
		m.allMatches = []viewport.Highlight{}
		m.focusedMatchIdx = -1
		m.totalMatchesOnAllItems = totalMatchCount
		// count of items with matches up to the limit
		m.numMatchingItems = len(itemsWithMatchesSet)
		return m.objects, filterChanged
	}

	filteredObjects := make([]T, 0, len(m.objects))
	itemsWithMatches := make(map[int]bool)

	for _, highlight := range highlights {
		itemIdx := highlight.ItemIndex
		if !itemsWithMatches[itemIdx] {
			filteredObjects = append(filteredObjects, m.objects[itemIdx])
			m.itemIdxToFilteredIdx[itemIdx] = len(filteredObjects) - 1
			itemsWithMatches[itemIdx] = true
		}
		m.allMatches = append(m.allMatches, highlight)
	}

	m.totalMatchesOnAllItems = len(m.allMatches)

	if filterChanged {
		if m.totalMatchesOnAllItems > 0 {
			m.focusedMatchIdx = 0
		} else {
			m.focusedMatchIdx = -1
		}
	} else {
		if prevFocusedMatchIdx >= 0 && prevFocusedMatchIdx < len(m.allMatches) {
			m.focusedMatchIdx = prevFocusedMatchIdx
		} else if m.totalMatchesOnAllItems > 0 {
			m.focusedMatchIdx = 0
		} else {
			m.focusedMatchIdx = -1
		}
	}

	return filteredObjects, filterChanged
}

// appendMatchesForNewObjects processes only newly appended objects for matches
// and incrementally updates match state without rescanning existing objects
func (m *Model[T]) appendMatchesForNewObjects(startIdx int, newObjects []T) {
	filterValue := m.filterTextInput.Value()

	var matchFn MatchFunc
	if mode := m.GetActiveFilterMode(); mode != nil {
		var err error
		matchFn, err = mode.GetMatchFunc(filterValue)
		if err != nil {
			// invalid match (e.g. bad regex), fallback to full update
			m.updateMatchingItems()
			return
		}
	}
	if matchFn == nil {
		m.updateMatchingItems()
		return
	}

	matchIdx := len(m.allMatches)
	totalMatchCount := m.totalMatchesOnAllItems
	prevNumMatchingItems := m.numMatchingItems
	itemsWithMatchesSet := make(map[int]bool)
	var newHighlights []viewport.Highlight

	for i, obj := range newObjects {
		itemIdx := startIdx + i
		matches := m.extractMatches(obj, matchFn)

		if len(matches) > 0 {
			itemsWithMatchesSet[itemIdx] = true
		}

		if m.maxMatchLimit > 0 && totalMatchCount+len(matches) > m.maxMatchLimit {
			// transition to match limit exceeded
			m.matchLimitExceeded = true
			m.allMatches = []viewport.Highlight{}
			m.focusedMatchIdx = -1
			m.totalMatchesOnAllItems = totalMatchCount
			m.numMatchingItems = prevNumMatchingItems + len(itemsWithMatchesSet)
			m.vp.SetObjects(m.objects)
			m.updateFocusedMatchHighlight()
			// update the pre-footer line with the current filter state
			m.setFilterLine(m.renderFilterLine())
			return
		}

		totalMatchCount += len(matches)

		highlights := m.buildHighlightsFromMatches(itemIdx, matches, matchIdx)
		matchIdx += len(matches)
		newHighlights = append(newHighlights, highlights...)
	}

	// append new matches to existing
	m.allMatches = append(m.allMatches, newHighlights...)
	m.totalMatchesOnAllItems = totalMatchCount
	m.numMatchingItems = prevNumMatchingItems + len(itemsWithMatchesSet)

	// update viewport objects
	if m.showMatchesOnly() {
		// build filtered objects list including new matching items
		filteredObjects := make([]T, 0, m.numMatchingItems)
		itemsWithMatches := make(map[int]bool)

		for _, highlight := range m.allMatches {
			itemIdx := highlight.ItemIndex
			if !itemsWithMatches[itemIdx] {
				filteredObjects = append(filteredObjects, m.objects[itemIdx])
				m.itemIdxToFilteredIdx[itemIdx] = len(filteredObjects) - 1
				itemsWithMatches[itemIdx] = true
			}
		}
		m.vp.SetObjects(filteredObjects)
	} else {
		// already updated by append to m.objects
		m.vp.SetObjects(m.objects)
	}

	m.updateFocusedMatchHighlight()
	// update the pre-footer line with the current filter state
	m.setFilterLine(m.renderFilterLine())
}

// extractMatches extracts matches from an object using the provided MatchFunc
func (m *Model[T]) extractMatches(obj T, matchFn MatchFunc) []item.Match {
	itm := obj.GetItem()
	byteRanges := matchFn(itm.ContentNoAnsi())
	return itm.ByteRangesToMatches(byteRanges)
}

// buildHighlightsFromMatches creates viewport highlights from item matches
func (m *Model[T]) buildHighlightsFromMatches(itemIdx int, matches []item.Match, startMatchIdx int) []viewport.Highlight {
	highlights := make([]viewport.Highlight, 0, len(matches))
	matchIdx := startMatchIdx

	for i := range matches {
		m.matchWidthsByMatchIdx[matchIdx] = matches[i].WidthRange
		matchIdx++

		highlight := viewport.Highlight{
			ItemIndex: itemIdx,
			ItemHighlight: item.Highlight{
				Style:                    m.styles.Match.Unfocused,
				ByteRangeUnstyledContent: matches[i].ByteRange,
			},
		}
		highlights = append(highlights, highlight)
	}

	return highlights
}

func (m *Model[T]) showMatchesOnly() bool {
	return m.matchingItemsOnly && !m.matchLimitExceeded
}

// matchingItemsOnlyText returns the text to display when showing matching items only
func matchingItemsOnlyText(matchingItemsOnly bool) string {
	if matchingItemsOnly {
		return "showing matches only"
	}
	return ""
}

// removeEmpty removes empty strings from a slice
func removeEmpty(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

// getTextAfterFilter returns the text to display after the filter input
func (m *Model[T]) getTextAfterFilter() string {
	if m.filterTextInput.Value() == "" {
		return "type to filter"
	}
	return m.getMatchCountText()
}

// getMatchCountText returns the formatted match count text
func (m *Model[T]) getMatchCountText() string {
	if m.matchLimitExceeded {
		if m.itemDescriptor != "" {
			return fmt.Sprintf("(%d+ matches on %d+ %s)", m.maxMatchLimit, m.numMatchingItems, m.itemDescriptor)
		}
		return fmt.Sprintf("(%d+ matches)", m.maxMatchLimit)
	}
	if m.totalMatchesOnAllItems == 0 {
		return "(no matches)"
	}
	currentMatch := m.focusedMatchIdx + 1
	if m.focusedMatchIdx < 0 {
		currentMatch = 0
	}
	if m.itemDescriptor != "" {
		return fmt.Sprintf("(%d/%d matches on %d %s)", currentMatch, m.totalMatchesOnAllItems, m.numMatchingItems, m.itemDescriptor)
	}
	return fmt.Sprintf("(%d/%d matches)", currentMatch, m.totalMatchesOnAllItems)
}

func (m *Model[T]) navigateToNextMatch() {
	if len(m.allMatches) == 0 {
		return
	}
	m.focusedMatchIdx = (m.focusedMatchIdx + 1) % len(m.allMatches)
	m.afterMatchNavigation()
}

func (m *Model[T]) navigateToPrevMatch() {
	if len(m.allMatches) == 0 {
		return
	}
	m.focusedMatchIdx--
	if m.focusedMatchIdx < 0 {
		m.focusedMatchIdx = len(m.allMatches) - 1
	}
	m.afterMatchNavigation()
}

func (m *Model[T]) afterMatchNavigation() {
	m.ensureCurrentMatchInView()
	m.setSelectionToCurrentMatch()
	m.updateFocusedMatchHighlight()
	m.setFilterLine(m.renderFilterLine())
}

const maxSearchHistorySize = 100

func (m *Model[T]) addToSearchHistory(text string) {
	if text == "" {
		return
	}
	if len(m.searchHistory) > 0 && m.searchHistory[len(m.searchHistory)-1] == text {
		return
	}
	m.searchHistory = append(m.searchHistory, text)
	if len(m.searchHistory) > maxSearchHistorySize {
		m.searchHistory = m.searchHistory[len(m.searchHistory)-maxSearchHistorySize:]
	}
}

func (m *Model[T]) resetSearchHistoryBrowsing() {
	m.searchHistoryIdx = len(m.searchHistory)
	m.searchHistoryDraft = ""
}

func (m *Model[T]) navigateSearchHistoryPrev() {
	if len(m.searchHistory) == 0 {
		return
	}
	if m.searchHistoryIdx == len(m.searchHistory) {
		m.searchHistoryDraft = m.filterTextInput.Value()
	}
	if m.searchHistoryIdx > 0 {
		m.searchHistoryIdx--
	}
	text := m.searchHistory[m.searchHistoryIdx]
	m.filterTextInput.SetValue(text)
	m.filterTextInput.SetCursor(len(text))
}

func (m *Model[T]) navigateSearchHistoryNext() {
	if m.searchHistoryIdx >= len(m.searchHistory) {
		return
	}
	m.searchHistoryIdx++
	if m.searchHistoryIdx == len(m.searchHistory) {
		m.filterTextInput.SetValue(m.searchHistoryDraft)
		m.filterTextInput.SetCursor(len(m.searchHistoryDraft))
	} else {
		text := m.searchHistory[m.searchHistoryIdx]
		m.filterTextInput.SetValue(text)
		m.filterTextInput.SetCursor(len(text))
	}
}

func (m *Model[T]) getFocusedMatch() *viewport.Highlight {
	if m.focusedMatchIdx < 0 || m.focusedMatchIdx >= len(m.allMatches) {
		return nil
	}
	return &m.allMatches[m.focusedMatchIdx]
}

// getItemIdx returns the viewport item index for a match, remapping when showing matches only
func (m *Model[T]) getItemIdx(match *viewport.Highlight) int {
	itemIdx := match.ItemIndex
	if m.showMatchesOnly() {
		if filteredIdx, ok := m.itemIdxToFilteredIdx[itemIdx]; ok {
			return filteredIdx
		}
	}
	return itemIdx
}

func (m *Model[T]) ensureCurrentMatchInView() {
	currentMatch := m.getFocusedMatch()
	if currentMatch == nil {
		return
	}
	widthRange := m.matchWidthsByMatchIdx[m.focusedMatchIdx]
	m.vp.EnsureItemInView(m.getItemIdx(currentMatch), widthRange.Start, widthRange.End, m.verticalPad, m.horizontalPad)
}

func (m *Model[T]) setSelectionToCurrentMatch() {
	if !m.vp.GetSelectionEnabled() {
		return
	}
	currentMatch := m.getFocusedMatch()
	if currentMatch == nil {
		return
	}
	itemIdx := m.getItemIdx(currentMatch)
	if m.vp.GetSelectedItemIdx() != itemIdx {
		m.vp.SetSelectedItemIdx(itemIdx)
	}
}
