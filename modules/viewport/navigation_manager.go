package viewport

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// navigationManager manages keyboard input and navigation logic
type navigationManager struct {
	// keyMap is the keymap for the viewport
	keyMap KeyMap

	// selectionEnabled is true if the viewport allows individual line selection
	selectionEnabled bool

	// topSticky is true when selection should remain at the top until user manually scrolls down
	topSticky bool

	// bottomSticky is true when selection should remain at the bottom until user manually scrolls up
	bottomSticky bool
}

// newNavigationManager creates a new navigationManager with the specified key mappings.
func newNavigationManager(keyMap KeyMap) *navigationManager {
	return &navigationManager{
		keyMap:           keyMap,
		selectionEnabled: false,
		topSticky:        false,
		bottomSticky:     false,
	}
}

// navigationAction represents a navigation command
type navigationAction int

const (
	// actionNone represents no navigation action.
	actionNone navigationAction = iota
	// actionUp represents moving up one item.
	actionUp
	// actionDown represents moving down one item.
	actionDown
	// actionLeft represents moving left horizontally.
	actionLeft
	// actionRight represents moving right horizontally.
	actionRight
	// actionHalfPageUp represents moving up half a page.
	actionHalfPageUp
	// actionHalfPageDown represents moving down half a page.
	actionHalfPageDown
	// actionPageUp represents moving up one page.
	actionPageUp
	// actionPageDown represents moving down one page.
	actionPageDown
	// actionTop represents moving to the top.
	actionTop
	// actionBottom represents moving to the bottom.
	actionBottom
)

// navigationContext contains the context needed for navigation calculations
type navigationContext struct {
	wrapText        bool
	dimensions      rectangle
	numContentLines int
	numVisibleItems int
}

// navigationResult contains the result of processing a navigation action
type navigationResult struct {
	action          navigationAction
	scrollAmount    int // lines to scroll
	selectionAmount int // items to move selection
}

// processKeyMsg processes a keyboard message and returns the corresponding navigation action
func (nm navigationManager) processKeyMsg(msg tea.KeyMsg, ctx navigationContext) navigationResult {
	switch {
	case key.Matches(msg, nm.keyMap.Up):
		return navigationResult{action: actionUp, scrollAmount: 1, selectionAmount: 1}

	case key.Matches(msg, nm.keyMap.Down):
		return navigationResult{action: actionDown, scrollAmount: 1, selectionAmount: 1}

	case key.Matches(msg, nm.keyMap.Left):
		if !ctx.wrapText {
			return navigationResult{action: actionLeft, scrollAmount: ctx.dimensions.width / 4}
		}

	case key.Matches(msg, nm.keyMap.Right):
		if !ctx.wrapText {
			return navigationResult{action: actionRight, scrollAmount: ctx.dimensions.width / 4}
		}

	case key.Matches(msg, nm.keyMap.HalfPageUp):
		scrollAmount := ctx.numContentLines / 2
		selectionAmount := max(1, ctx.numVisibleItems/2)
		return navigationResult{action: actionHalfPageUp, scrollAmount: scrollAmount, selectionAmount: selectionAmount}

	case key.Matches(msg, nm.keyMap.HalfPageDown):
		scrollAmount := ctx.numContentLines / 2
		selectionAmount := max(1, ctx.numVisibleItems/2)
		return navigationResult{action: actionHalfPageDown, scrollAmount: scrollAmount, selectionAmount: selectionAmount}

	case key.Matches(msg, nm.keyMap.PageUp):
		scrollAmount := ctx.numContentLines
		selectionAmount := ctx.numVisibleItems
		return navigationResult{action: actionPageUp, scrollAmount: scrollAmount, selectionAmount: selectionAmount}

	case key.Matches(msg, nm.keyMap.PageDown):
		scrollAmount := ctx.numContentLines
		selectionAmount := ctx.numVisibleItems
		return navigationResult{action: actionPageDown, scrollAmount: scrollAmount, selectionAmount: selectionAmount}

	case key.Matches(msg, nm.keyMap.Top):
		return navigationResult{action: actionTop}

	case key.Matches(msg, nm.keyMap.Bottom):
		return navigationResult{action: actionBottom}
	}

	return navigationResult{action: actionNone}
}
