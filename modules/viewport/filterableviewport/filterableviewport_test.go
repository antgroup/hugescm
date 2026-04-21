package filterableviewport

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

type object struct {
	item item.Item
}

func (i object) GetItem() item.Item {
	return i.item
}

var _ viewport.Object = object{}

var (
	filterKeyMsg                = internal.MakeKeyMsg('/')
	regexFilterKeyMsg           = internal.MakeKeyMsg('r')
	caseInsensitiveFilterKeyMsg = internal.MakeKeyMsg('i')
	applyFilterKeyMsg           = tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"}
	cancelFilterKeyMsg          = tea.KeyPressMsg{Code: tea.KeyEscape, Text: "esc"}
	toggleMatchesKeyMsg         = internal.MakeKeyMsg('o')
	nextMatchKeyMsg             = internal.MakeKeyMsg('n')
	prevMatchKeyMsg             = internal.MakeKeyMsg('N')
	downKeyMsg                  = tea.KeyPressMsg{Code: tea.KeyDown, Text: "down"}

	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	viewportStyles    = viewport.Styles{
		FooterStyle:       footerStyle,
		SelectedItemStyle: selectedItemStyle,
	}

	// cursorStyle matches the default virtual cursor rendering from textinput v2:
	// cursor.Model.View() renders Style.Inline(true).Reverse(true).Render(char)
	// where Style = lipgloss.NewStyle().Foreground(cursorColor) and cursorColor defaults to "7"
	cursorStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Reverse(true)
	focusedStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))
	focusedIfSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	unfocusedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Background(lipgloss.Color("12"))
	matchStyles            = MatchStyles{
		Focused:           focusedStyle,
		FocusedIfSelected: focusedStyle,
		Unfocused:         unfocusedStyle,
	}
	filterableViewportStyles = Styles{
		Match: matchStyles,
	}
)

func makeFilterableViewport(
	width int,
	height int,
	vpOptions []viewport.Option[object],
	fvOptions []Option[object],
) *Model[object] {
	// use default viewport test styles, will be overridden by options if passed in
	defaultTestVpStylesOption := viewport.WithStyles[object](viewportStyles)
	vpOptions = append([]viewport.Option[object]{defaultTestVpStylesOption}, vpOptions...)

	// use default filterable viewport test styles and item descriptor, will be overridden by options if passed in
	defaultTestFvStylesOption := WithStyles[object](filterableViewportStyles)
	defaultTestItemDescriptorOption := WithItemDescriptor[object]("items")
	fvOptions = append([]Option[object]{defaultTestFvStylesOption, defaultTestItemDescriptorOption}, fvOptions...)

	vp := viewport.New[object](width, height, vpOptions...)
	return New[object](vp, fvOptions...)
}

func TestNew(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"Line 1",
		"Line 2",
		"Line 3",
	}))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"Line 1",
		"Line 2",
		"No Filter",
		footerStyle.Render("66% (2/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestNewLongText(t *testing.T) {
	fv := makeFilterableViewport(
		10, // emptyText is longer than this
		5,  // increased height
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("Nada Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"Line 1",
		"Line 2",
		"Line 3",
	}))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"Line 1",
		"Line 2",
		"Line 3",
		"Nada Fi...",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestNewWidthHeight(t *testing.T) {
	fv := makeFilterableViewport(
		25,
		8,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	if fv.GetWidth() != 25 {
		t.Errorf("expected width 25, got %d", fv.GetWidth())
	}
	if fv.GetHeight() != 8 {
		t.Errorf("expected height 8, got %d", fv.GetHeight())
	}
}

func TestZeroDimensions(t *testing.T) {
	fv := makeFilterableViewport(
		0,
		0,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	if fv.GetWidth() != 0 {
		t.Errorf("expected width 0, got %d", fv.GetWidth())
	}
	if fv.GetHeight() != 0 {
		t.Errorf("expected height 0, got %d", fv.GetHeight())
	}
	internal.CmpStr(t, "", fv.View())
}

func TestNegativeDimensions(t *testing.T) {
	fv := makeFilterableViewport(
		-5,
		-3,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	if fv.GetWidth() != 0 {
		t.Errorf("expected width 0 for negative input, got %d", fv.GetWidth())
	}
	if fv.GetHeight() != 0 {
		t.Errorf("expected height 0 for negative input, got %d", fv.GetHeight())
	}
	internal.CmpStr(t, "", fv.View())
}

func TestSetWidthSetHeight(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetWidth(30)
	if fv.GetWidth() != 30 {
		t.Errorf("expected width 30, got %d", fv.GetWidth())
	}

	fv.SetHeight(6)
	if fv.GetHeight() != 6 {
		t.Errorf("expected height 6, got %d", fv.GetHeight())
	}
}

func TestFilterFocusedInitial(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	if fv.FilterFocused() {
		t.Error("filter should not be focused initially")
	}
}

func TestEmptyContent(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("No filter"),
		},
	)
	fv.SetObjects([]object{})
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"",
		"",
		"No filter",
		"",
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestWithMatchesOnlyTrue(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithMatchingItemsOnly[object](true),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"cherry",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a" + focusedStyle.Render("p") + unfocusedStyle.Render("p") + "le",
		"",
		"[exact] Filter: p" + cursorStyle.Render(" ") + " (1/2 matches on 1 items) showing matches only",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestWithMatchesOnlyFalse(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5, // increased height
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithMatchingItemsOnly[object](false),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"cherry",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a" + focusedStyle.Render("p") + unfocusedStyle.Render("p") + "le",
		"banana",
		"cherry",
		"[exact] Filter: p" + cursorStyle.Render(" ") + " (1/2 matches on 1 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestNoItemDescriptor(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithItemDescriptor[object](""), // override the test default
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"cherry",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a" + focusedStyle.Render("p") + unfocusedStyle.Render("p") + "le",
		"banana",
		"cherry",
		"[exact] p  (1/2 matches)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestWithCanToggleMatchesOnlyTrue(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithCanToggleMatchingItemsOnly[object](true),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"cherry",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a" + focusedStyle.Render("p") + unfocusedStyle.Render("p") + "le",
		"banana",
		"[exact] p  (1/2 matches on 1 items)",
		footerStyle.Render("66% (2/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	fv, _ = fv.Update(toggleMatchesKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a" + focusedStyle.Render("p") + unfocusedStyle.Render("p") + "le",
		"",
		"[exact] p  (1/2 matches on 1 items) showing matches only",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestWithCanToggleMatchesOnlyFalse(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithCanToggleMatchingItemsOnly[object](false),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"cherry",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a" + focusedStyle.Render("p") + unfocusedStyle.Render("p") + "le",
		"banana",
		"[exact] p  (1/2 matches on 1 items)",
		footerStyle.Render("66% (2/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	fv, _ = fv.Update(toggleMatchesKeyMsg)
	internal.CmpStr(t, expectedView, fv.View())
}

func TestNilContent(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(nil)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"",
		"",
		"No Filter",
		"",
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestDefaultText(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"test"}))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"test",
		"",
		"No Filter",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"test",
		"",
		"[exact] p  (no matches)",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterKeyFocus(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	if !fv.FilterFocused() {
		t.Error("filter should be focused after pressing filter key")
	}
}

func TestRegexFilterKeyFocus(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(regexFilterKeyMsg)
	if !fv.FilterFocused() {
		t.Error("filter should be focused after pressing regex filter key")
	}
}

func TestCaseInsensitiveFilterKeyEmpty(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"Apple", "banana"}))
	fv, _ = fv.Update(caseInsensitiveFilterKeyMsg)
	if !fv.FilterFocused() {
		t.Error("filter should be focused after pressing case insensitive filter key")
	}

	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	// 'a' matches 'A' in Apple and 3 'a's in banana = 4 matches on 2 items
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("A") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[iregex] Filter: a  (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSwitchFromExactToCaseInsensitive(t *testing.T) {
	fv := makeFilterableViewport(
		60,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"Apple", "banana"}))

	// exact filter
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// exact filter matches only lowercase 'a'
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"Apple",
		"b" + focusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[exact] Filter: a  (1/3 matches on 1 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// 'i' to switch to case-insensitive mode
	fv, _ = fv.Update(caseInsensitiveFilterKeyMsg)

	// now matches both cases, no (?i) in text, label is [iregex]
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("A") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[iregex] Filter: a" + cursorStyle.Render(" ") + " (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSwitchFromCaseInsensitiveToExact(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"Apple", "banana"}))

	// start case-insensitive filter
	fv, _ = fv.Update(caseInsensitiveFilterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// case-insensitive matching (matches both 'A' and 'a')
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("A") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[iregex] Filter: a  (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// switch to exact mode with '/'
	fv, _ = fv.Update(filterKeyMsg)

	// filter text preserved as-is, just switches to exact mode
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"Apple",
		"b" + focusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[exact] Filter: a" + cursorStyle.Render(" ") + " (1/3 matches on 1 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestCaseInsensitiveKeyReEntersEditingMode(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"Apple", "banana"}))

	// start case-insensitive filter
	fv, _ = fv.Update(caseInsensitiveFilterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// case-insensitive matching
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("A") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[iregex] Filter: a  (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// press 'i' again - should just re-enter editing mode
	fv, _ = fv.Update(caseInsensitiveFilterKeyMsg)

	// still case-insensitive, filter should be focused for editing
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("A") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[iregex] Filter: a" + cursorStyle.Render(" ") + " (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestApplyFilterKey(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	if fv.FilterFocused() {
		t.Error("filter should not be focused after applying filter")
	}
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("a") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"[exact] a  (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestCancelFilterKey(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(cancelFilterKeyMsg)
	if fv.FilterFocused() {
		t.Error("filter should not be focused after canceling")
	}
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"No Filter",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestRegexFilterValidPattern(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana", "apricot"}))
	fv, _ = fv.Update(regexFilterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	fv, _ = fv.Update(internal.MakeKeyMsg('+'))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("app") + "le",
		"banana",
		"[regex] Filter: ap+" + cursorStyle.Render(" ") + " (1/2 matches on 2 items)",
		footerStyle.Render("66% (2/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestRegexFilterInvalidPattern(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(regexFilterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('['))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"[regex] Filter: [" + cursorStyle.Render(" ") + " (no matches)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestStyleOverlay(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetSelectionEnabled(true)

	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		internal.RedFg.Render("apple") + " pie " + internal.BlueFg.Render("yum"),
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "apple pie" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// on selected lines, match highlights keep their original styles and selection fills gaps
	// first item is selected, has focused match covering entire content "apple pie"
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple pie"),
		unfocusedStyle.Render("apple pie") + " " + internal.BlueFg.Render("yum"),
		"[exact] apple pie  (1/2 matches on 2 items)",
		footerStyle.Render("50% (1/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// move selection down to second item: match keeps unfocused style, selection fills " yum"
	fv, _ = fv.Update(downKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple pie"),
		unfocusedStyle.Render("apple pie") + selectedItemStyle.Render(" yum"),
		"[exact] apple pie  (1/2 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestRegexFilterMultipleMatchesInSingleLine(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"the cat sat on the mat",
		"dog",
		"another the and the end",
	}))
	fv, _ = fv.Update(regexFilterKeyMsg)
	// use regex pattern \bthe\b to match whole word "the"
	for _, c := range "\\bthe\\b" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// should focus on first match in first line
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("the") + " cat sat on " + unfocusedStyle.Render("the") + " mat",
		"dog",
		"another " + unfocusedStyle.Render("the") + " and " + unfocusedStyle.Render("the") + " end",
		"",
		"[regex] Filter: \\bthe\\b  (1/4 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	// navigate to second match (still in first line)
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedSecondMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("the") + " cat sat on " + focusedStyle.Render("the") + " mat",
		"dog",
		"another " + unfocusedStyle.Render("the") + " and " + unfocusedStyle.Render("the") + " end",
		"",
		"[regex] Filter: \\bthe\\b  (2/4 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	// navigate to third match (third line, first match)
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedThirdMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("the") + " cat sat on " + unfocusedStyle.Render("the") + " mat",
		"dog",
		"another " + focusedStyle.Render("the") + " and " + unfocusedStyle.Render("the") + " end",
		"",
		"[regex] Filter: \\bthe\\b  (3/4 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedThirdMatch, fv.View())

	// navigate to fourth match (third line, second match)
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedFourthMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("the") + " cat sat on " + unfocusedStyle.Render("the") + " mat",
		"dog",
		"another " + unfocusedStyle.Render("the") + " and " + focusedStyle.Render("the") + " end",
		"",
		"[regex] Filter: \\bthe\\b  (4/4 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedFourthMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedFourthMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedThirdMatch, fv.View())
}

func TestNoMatchesShowsNoMatchesText(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('x'))
	fv, _ = fv.Update(internal.MakeKeyMsg('y'))
	fv, _ = fv.Update(internal.MakeKeyMsg('z'))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"[exact] xyz" + cursorStyle.Render(" ") + " (no matches)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestWithFilterModes(t *testing.T) {
	customModes := []FilterMode{
		ExactFilterMode(key.NewBinding(key.WithKeys("g"))),
	}
	fv := makeFilterableViewport(
		20,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object](customModes),
		},
	)
	fv.SetObjects(stringsToItems([]string{"test"}))
	fv, _ = fv.Update(filterKeyMsg) // '/' should not match custom key 'g'
	if fv.FilterFocused() {
		t.Error("filter should not be focused with custom filter modes")
	}
}

func TestViewportControls(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		3,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"line1", "line2", "line3"}))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line1",
		"No Filter",
		footerStyle.Render("33% (1/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	fv, _ = fv.Update(downKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line2",
		"No Filter",
		footerStyle.Render("66% (2/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestApplyEmptyFilterShowsWhenEmptyText(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No filter applied"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"No filter applied",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestEditingEmptyFilterShowsEditingMessage(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"[exact] Filter: " + cursorStyle.Render(" ") + " type to filter",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSpecialKeysWhileFiltering(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithCanToggleMatchingItemsOnly[object](true),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"book",
		"food",
		"cherry",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	fv, _ = fv.Update(toggleMatchesKeyMsg) // 'o'
	fv, _ = fv.Update(nextMatchKeyMsg)     // 'n'
	fv, _ = fv.Update(prevMatchKeyMsg)     // 'N'
	fv, _ = fv.Update(filterKeyMsg)        // '/'
	fv, _ = fv.Update(regexFilterKeyMsg)   // 'r'
	expectedViewAfterO := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"book",
		"[exact] ponN/r" + cursorStyle.Render(" ") + " (no matches)",
		footerStyle.Render("50% (2/4)"),
	})
	internal.CmpStr(t, expectedViewAfterO, fv.View())
}

func TestAnsiEscapeCodesNotMatched(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		internal.RedFg.Render("apple"),
		internal.RedFg.Render("book"),
		internal.RedFg.Render("food"),
		internal.RedFg.Render("cherry"),
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "x1b" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		internal.RedFg.Render("apple"),
		internal.RedFg.Render("book"),
		"[exact] x1b  (no matches)",
		footerStyle.Render("50% (2/4)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestMatchNavigationWithNoMatches(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('x'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"[exact] x  (no matches)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedView, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedView, fv.View())
}

func TestMatchNavigationWithOverlappingMatches(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{"aaa"}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "aa" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aa") + "a",
		"",
		"[exact] aa  (1/1 matches on 1 items)",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())
}

func TestMatchNavigationWithAllItemsWrap(t *testing.T) {
	fv := makeFilterableViewport(
		7,
		6,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{
			WithStyles[object](Styles{
				Match: matchStyles,
			}),
			WithMatchingItemsOnly[object](false),
			WithEmptyText[object]("None"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"hi there",
		"hi over there",
		"no match",
	}))
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hi ther",
		"e",
		"hi over",
		" there",
		"None",
		footerStyle.Render("66% ..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "there" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hi " + focusedStyle.Render("ther"),
		focusedStyle.Render("e"),
		"hi over",
		" " + unfocusedStyle.Render("there"),
		"[exa...",
		footerStyle.Render("66% ..."),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedSecondMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hi " + unfocusedStyle.Render("ther"),
		unfocusedStyle.Render("e"),
		"hi over",
		" " + focusedStyle.Render("there"),
		"[exa...",
		footerStyle.Render("66% ..."),
	})
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())
}

func TestMatchNavigationWithMatchingItemsOnlyWrap(t *testing.T) {
	fv := makeFilterableViewport(
		7,
		6,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{
			WithStyles[object](Styles{
				Match: matchStyles,
			}),
			WithMatchingItemsOnly[object](true),
			WithEmptyText[object]("None"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"hi there",
		"hi over there",
		"no match",
	}))
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hi ther",
		"e",
		"hi over",
		" there",
		"None",
		footerStyle.Render("66% ..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "there" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hi " + focusedStyle.Render("ther"),
		focusedStyle.Render("e"),
		"hi over",
		" " + unfocusedStyle.Render("there"),
		"[exa...",
		footerStyle.Render("100%..."),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedSecondMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hi " + unfocusedStyle.Render("ther"),
		unfocusedStyle.Render("e"),
		"hi over",
		" " + focusedStyle.Render("there"),
		"[exa...",
		footerStyle.Render("100%..."),
	})
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())
}

func TestMatchNavigationWrapLineOffset(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		strings.Repeat("a", 100) + "goose" + strings.Repeat("a", 100),
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "goose" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		strings.Repeat("a", 20),
		strings.Repeat("a", 20),
		focusedStyle.Render("goose") + strings.Repeat("a", 15),
		"[exact] goose  (1...",
		footerStyle.Render("99% (1/1)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestMatchNavigationWrappedLinesWithMatches(t *testing.T) {
	fv := makeFilterableViewport(
		4,
		6,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		strings.Repeat("a", 10),
		strings.Repeat("b", 15),
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "aaa" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aaa") + unfocusedStyle.Render("a"),
		unfocusedStyle.Render("aa") + unfocusedStyle.Render("aa"),
		unfocusedStyle.Render("a") + "a",
		"bbbb",
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("aaa") + focusedStyle.Render("a"),
		focusedStyle.Render("aa") + unfocusedStyle.Render("aa"),
		unfocusedStyle.Render("a") + "a",
		"bbbb",
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(cancelFilterKeyMsg)
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "bbb" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"aaaa",
		"aaaa",
		"aa",
		focusedStyle.Render("bbb") + unfocusedStyle.Render("b"),
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"aaaa",
		"aa",
		unfocusedStyle.Render("bbb") + focusedStyle.Render("b"),
		focusedStyle.Render("bb") + unfocusedStyle.Render("bb"),
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestMatchNavigationWrappedLinesWithWrappedMatches(t *testing.T) {
	fv := makeFilterableViewport(
		4,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		strings.Repeat("a", 10),
		strings.Repeat("a", 15),
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for range 5 {
		fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aaaa"),
		focusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		unfocusedStyle.Render("aa"),
		"[...",
		footerStyle.Render("5..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("aaaa"),
		unfocusedStyle.Render("a") + focusedStyle.Render("aaa"),
		focusedStyle.Render("aa"),
		"[...",
		footerStyle.Render("5..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("aa"),
		focusedStyle.Render("aaaa"),
		focusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("aaaa"),
		unfocusedStyle.Render("a") + focusedStyle.Render("aaa"),
		focusedStyle.Render("aa") + unfocusedStyle.Render("aa"),
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		unfocusedStyle.Render("aa") + focusedStyle.Render("aa"),
		focusedStyle.Render("aaa"),
		"[...",
		footerStyle.Render("1..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("a") + focusedStyle.Render("aaa"),
		focusedStyle.Render("aa") + unfocusedStyle.Render("aa"),
		unfocusedStyle.Render("aaa"),
		"[...",
		footerStyle.Render("1..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aaaa"),
		focusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		unfocusedStyle.Render("aa") + unfocusedStyle.Render("aa"),
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("a") + focusedStyle.Render("aaa"),
		focusedStyle.Render("aa"),
		unfocusedStyle.Render("aaaa"),
		"[...",
		footerStyle.Render("9..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aaaa"),
		focusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		unfocusedStyle.Render("aa"),
		"[...",
		footerStyle.Render("5..."),
	})
	internal.CmpStr(t, expected, fv.View())

	// rollover
	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		unfocusedStyle.Render("aa") + focusedStyle.Render("aa"),
		focusedStyle.Render("aaa"),
		"[...",
		footerStyle.Render("1..."),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aaaa"),
		focusedStyle.Render("a") + unfocusedStyle.Render("aaa"),
		unfocusedStyle.Render("aa"),
		"[...",
		footerStyle.Render("5..."),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestMatchNavigationNoWrap(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		6,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"duck duck duck duck duck duck duck duck duck duck goose",
		"duck duck duck duck duck goose duck duck duck duck duck",
		"goose duck duck duck duck duck duck duck duck duck duck",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "goose" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"...k duck duck duck duck " + focusedStyle.Render("goose"),
		unfocusedStyle.Render("...se") + " duck duck duck duck duck",
		"...ck duck duck duck duck duck",
		"",
		"[exact] goose  (1/3 matches...",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedSecondMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"...k duck duck duck duck " + unfocusedStyle.Render("goose"),
		focusedStyle.Render("...se") + " duck duck duck duck duck",
		"...ck duck duck duck duck duck",
		"",
		"[exact] goose  (2/3 matches...",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedThirdMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"duck duck duck duck duck du...",
		"duck duck duck duck duck " + unfocusedStyle.Render("go..."),
		focusedStyle.Render("goose") + " duck duck duck duck d...",
		"",
		"[exact] goose  (3/3 matches...",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedThirdMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())
}

func TestMatchNavigationNoWrapPanning(t *testing.T) {
	fv := makeFilterableViewport(
		10,
		3,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		strings.Repeat("a", 32),
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for range 4 {
		fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedLeftmostMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("aaaa") + unfocusedStyle.Render("aaa.") + unfocusedStyle.Render(".."),
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})

	internal.CmpStr(t, expectedLeftmostMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("aaaa") + focusedStyle.Render("aaa.") + unfocusedStyle.Render(".."),
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedTravelingRight := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("..") + unfocusedStyle.Render(".aaa") + focusedStyle.Render("a..."),
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedTravelingRight, fv.View())

	for range 4 {
		fv, _ = fv.Update(nextMatchKeyMsg)
		internal.CmpStr(t, expectedTravelingRight, fv.View())
	}

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedRightmostMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("..") + unfocusedStyle.Render(".aaa") + focusedStyle.Render("aaaa"),
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedRightmostMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("..") + focusedStyle.Render(".aaa") + unfocusedStyle.Render("aaaa"),
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expectedTravelingLeft := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("...a") + unfocusedStyle.Render("aaa.") + unfocusedStyle.Render(".."),
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedTravelingLeft, fv.View())

	for range 4 {
		fv, _ = fv.Update(prevMatchKeyMsg)
		internal.CmpStr(t, expectedTravelingLeft, fv.View())
	}

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedLeftmostMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedRightmostMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedLeftmostMatch, fv.View())
}

func TestMatchNavigationNoWrapUnicode(t *testing.T) {
	fv := makeFilterableViewport(
		32,
		3,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		// a (1w, 1b), 💖 (2w, 4b)
		"💖💖💖💖💖💖💖💖 hi aaaaaaaaaaaaaaaa",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "hi" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"💖💖💖💖💖💖💖💖 " + focusedStyle.Render("hi") + " aaaaaaaaa...",
		"[exact] hi  (1/1 matches on 1...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())
}

func TestMatchNavigationManyMatchesWrap(t *testing.T) {
	fv := makeFilterableViewport(
		100,
		50,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{},
	)
	numAs := 10000
	fv.SetObjects(stringsToItems([]string{
		internal.RedFg.Render(strings.Repeat("a", numAs)),
	}))
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)
	firstRows := []string{
		focusedStyle.Render("a") + strings.Repeat(unfocusedStyle.Render("a"), fv.GetWidth()-1),
	}
	rest := make([]string, fv.GetHeight()-3) // -3 for first row, filter, footer
	for i := range rest {
		rest[i] = strings.Repeat(unfocusedStyle.Render("a"), fv.GetWidth())
	}
	rest = append(rest, fmt.Sprintf("[exact] a  (1/%d matches on 1 items)", numAs))
	rest = append(rest, footerStyle.Render("99% (1/1)"))
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), append(firstRows, rest...))
	internal.CmpStr(t, expected, fv.View())
}

func TestMatchNavigationManyMatchesWrapPerformance(t *testing.T) {
	runTest := func(t *testing.T) {
		fv := makeFilterableViewport(
			100,
			50,
			[]viewport.Option[object]{
				viewport.WithWrapText[object](true),
			},
			[]Option[object]{},
		)
		numAs := 5000
		fv.SetObjects(stringsToItems([]string{
			internal.RedFg.Render(strings.Repeat("a", numAs)),
		}))
		fv, _ = fv.Update(filterKeyMsg)
		fv, _ = fv.Update(internal.MakeKeyMsg('a'))
		fv, _ = fv.Update(applyFilterKeyMsg)
		firstRows := []string{
			focusedStyle.Render("a") + strings.Repeat(unfocusedStyle.Render("a"), fv.GetWidth()-1),
		}
		rest := make([]string, fv.GetHeight()-3) // -3 for first row, filter, footer
		for i := range rest {
			rest[i] = strings.Repeat(unfocusedStyle.Render("a"), fv.GetWidth())
		}
		rest = append(rest, fmt.Sprintf("[exact] a  (1/%d matches on 1 items)", numAs))
		rest = append(rest, footerStyle.Render("99% (1/1)"))
		expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), append(firstRows, rest...))
		internal.CmpStr(t, expected, fv.View())

		numNext := 40
		for range numNext {
			fv, _ = fv.Update(nextMatchKeyMsg)
		}
		expectedAfterNext := []string{
			strings.Repeat(unfocusedStyle.Render("a"), numNext) + focusedStyle.Render("a") + strings.Repeat(unfocusedStyle.Render("a"), fv.GetWidth()-numNext-1),
		}
		restAfterNext := make([]string, fv.GetHeight()-3) // -3 for first row, filter, footer
		for i := range restAfterNext {
			restAfterNext[i] = strings.Repeat(unfocusedStyle.Render("a"), fv.GetWidth())
		}
		restAfterNext = append(restAfterNext, fmt.Sprintf("[exact] a  (%d/%d matches on 1 items)", numNext+1, numAs))
		restAfterNext = append(restAfterNext, footerStyle.Render("99% (1/1)"))
		expectedAfterNextView := internal.Pad(fv.GetWidth(), fv.GetHeight(), append(expectedAfterNext, restAfterNext...))
		internal.CmpStr(t, expectedAfterNextView, fv.View())
	}
	internal.RunWithTimeout(t, runTest, 200*time.Millisecond)
}

func TestScrollingWithManyHighlightedMatchesPerformance(t *testing.T) {
	runTest := func(t *testing.T) {
		width := 80
		height := 20
		fv := makeFilterableViewport(
			width,
			height,
			[]viewport.Option[object]{
				viewport.WithWrapText[object](false),
			},
			[]Option[object]{},
		)

		numItems := height * 5
		items := make([]string, numItems)
		for i := range items {
			items[i] = strings.Repeat("a", width)
		}
		fv.SetObjects(stringsToItems(items))

		// everything on screen highlighted
		fv, _ = fv.Update(filterKeyMsg)
		fv, _ = fv.Update(internal.MakeKeyMsg('a'))
		fv, _ = fv.Update(applyFilterKeyMsg)

		firstView := fv.View()
		if !strings.Contains(firstView, focusedStyle.Render("a")) {
			t.Fatal("expected focused match in initial view")
		}

		for i := range height {
			fv, _ = fv.Update(downKeyMsg)
			view := fv.View()

			// after first scroll, focused match should go out of view
			// but unfocused matches should still be visible
			if i > 0 && strings.Contains(view, focusedStyle.Render("a")) {
				t.Errorf("focused match should be out of view after scrolling %d times", i+1)
			}
			if !strings.Contains(view, unfocusedStyle.Render("a")) {
				t.Errorf("unfocused matches should still be visible after scrolling %d times", i+1)
			}
		}
	}
	internal.RunWithTimeout(t, runTest, 220*time.Millisecond)
}

func TestScrollingWithManyHighlightedMatchesPerformanceSelectionEnabled(t *testing.T) {
	runTest := func(t *testing.T) {
		width := 80
		height := 20
		fv := makeFilterableViewport(
			width,
			height,
			[]viewport.Option[object]{
				viewport.WithWrapText[object](false),
				viewport.WithSelectionEnabled[object](true),
			},
			[]Option[object]{},
		)

		numItems := height * 5
		items := make([]string, numItems)
		for i := range items {
			items[i] = strings.Repeat("a", width)
		}
		fv.SetObjects(stringsToItems(items))

		// everything on screen highlighted
		fv, _ = fv.Update(filterKeyMsg)
		fv, _ = fv.Update(internal.MakeKeyMsg('a'))
		fv, _ = fv.Update(applyFilterKeyMsg)

		firstView := fv.View()
		if !strings.Contains(firstView, focusedStyle.Render("a")) {
			t.Fatal("expected focused match in initial view")
		}

		// with selection enabled, the viewport keeps the selected item (with focused match) in view
		// height - 2 accounts for header and footer lines, leaving content lines
		contentLines := height - 2
		for i := range height {
			fv, _ = fv.Update(downKeyMsg)
			view := fv.View()

			// for first (contentLines - 1) scrolls, focused match stays in view
			// after that, selection scrolls past visible area
			if i < contentLines-1 {
				if !strings.Contains(view, focusedStyle.Render("a")) {
					t.Errorf("focused match should stay in view after moving selection down %d times", i+1)
				}
			} else {
				if strings.Contains(view, focusedStyle.Render("a")) {
					t.Errorf("focused match should be out of view after moving selection down %d times", i+1)
				}
			}

			// unfocused matches should always be visible
			if !strings.Contains(view, unfocusedStyle.Render("a")) {
				t.Errorf("unfocused matches should still be visible after moving selection down %d times", i+1)
			}
		}
	}
	internal.RunWithTimeout(t, runTest, 200*time.Millisecond)
}

func TestMatchNavigationWithSelectionEnabled(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
			viewport.WithSelectionEnabled[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "apple" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + selectedItemStyle.Render(" pie"),
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("33% (1/3)"),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedSecondMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("apple") + " pie",
		"banana bread",
		focusedStyle.Render("apple") + selectedItemStyle.Render(" cake"),
		"[exact] apple  (2/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedFirstMatch, fv.View())
}

func TestFocusedIfSelectedMatchStyle(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
			viewport.WithSelectionEnabled[object](true),
		},
		[]Option[object]{
			WithStyles[object](Styles{
				Match: MatchStyles{
					Focused:           focusedStyle,
					FocusedIfSelected: focusedIfSelectedStyle,
					Unfocused:         unfocusedStyle,
				},
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))

	// start filtering for "apple"
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "apple" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// focused match is on item 0 (selected) — should use focusedIfSelectedStyle
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedIfSelectedStyle.Render("apple") + selectedItemStyle.Render(" pie"),
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("33% (1/3)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// navigate to next match — focused match moves to item 2 (now selected),
	// item 0 becomes unfocused
	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("apple") + " pie",
		"banana bread",
		focusedIfSelectedStyle.Render("apple") + selectedItemStyle.Render(" cake"),
		"[exact] apple  (2/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// navigate back — focused match on item 0 again (selected),
	// uses focusedIfSelectedStyle again
	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedIfSelectedStyle.Render("apple") + selectedItemStyle.Render(" pie"),
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("33% (1/3)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestFocusedIfSelectedWithReverseSelection(t *testing.T) {
	reverseStyle := lipgloss.NewStyle().Reverse(true)
	cyanFgStyle := lipgloss.NewStyle().Foreground(lipgloss.Cyan)
	reverseCyanStyle := lipgloss.NewStyle().Reverse(true).Foreground(lipgloss.Cyan)
	brightRedStyle := lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	fv := makeFilterableViewport(
		40,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
			viewport.WithSelectionEnabled[object](true),
			viewport.WithStyles[object](viewport.Styles{
				SelectedItemStyle: reverseStyle,
			}),
		},
		[]Option[object]{
			WithStyles[object](Styles{
				Match: MatchStyles{
					Focused:           reverseCyanStyle,
					FocusedIfSelected: cyanFgStyle,
					Unfocused:         brightRedStyle,
				},
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))

	// Apply filter
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "apple" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// After apply: focused match (1/2) on item 0 which IS selected
	// FocusedIfSelected should be used for "apple", SelectedItemStyle for " pie"
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		cyanFgStyle.Render("apple") + reverseStyle.Render(" pie"),
		"banana bread",
		brightRedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		"33% (1/3)",
	})
	internal.CmpStr(t, expected, fv.View())

	// Press n — focused match moves to item 2, selection follows
	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		brightRedStyle.Render("apple") + " pie",
		"banana bread",
		cyanFgStyle.Render("apple") + reverseStyle.Render(" cake"),
		"[exact] apple  (2/2 matches on 2 items)",
		"100% (3/3)",
	})
	internal.CmpStr(t, expected, fv.View())

	// Move selection up — focused match stays on item 2 but selection moves to item 1,
	// so focused match should now use Focused (reverse+cyan) instead of FocusedIfSelected
	fv, _ = fv.Update(internal.MakeKeyMsg('k'))
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		brightRedStyle.Render("apple") + " pie",
		reverseStyle.Render("banana bread"),
		reverseCyanStyle.Render("apple") + " cake",
		"[exact] apple  (2/2 matches on 2 items)",
		"66% (2/3)",
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestMatchNavigationWithSelectionEnabledWrap(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		6,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
			viewport.WithSelectionEnabled[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"the quick brown fox",
		"jumped over the lazy dog",
		"the end",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "the" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedFirstMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("the") + selectedItemStyle.Render(" quick brown fox"),
		"jumped over " + unfocusedStyle.Render("the") + " lazy",
		" dog",
		unfocusedStyle.Render("the") + " end",
		"[exact] the  (1/3...",
		footerStyle.Render("33% (1/3)"),
	})
	internal.CmpStr(t, expectedFirstMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedSecondMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("the") + " quick brown fox",
		selectedItemStyle.Render("jumped over ") + focusedStyle.Render("the") + selectedItemStyle.Render(" lazy"),
		selectedItemStyle.Render(" dog"),
		unfocusedStyle.Render("the") + " end",
		"[exact] the  (2/3...",
		footerStyle.Render("66% (2/3)"),
	})
	internal.CmpStr(t, expectedSecondMatch, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedThirdMatch := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("the") + " quick brown fox",
		"jumped over " + unfocusedStyle.Render("the") + " lazy",
		" dog",
		focusedStyle.Render("the") + selectedItemStyle.Render(" end"),
		"[exact] the  (3/3...",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedThirdMatch, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedSecondMatch, fv.View())
}

func TestMatchNavigationWithSelectionEnabledWrapScrolling(t *testing.T) {
	fv := makeFilterableViewport(
		5,
		4,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
			viewport.WithSelectionEnabled[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"long long long long ",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "long " {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedTopFocused := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("long "),
		unfocusedStyle.Render("long "),
		"[e...",
		footerStyle.Render("10..."),
	})
	internal.CmpStr(t, expectedTopFocused, fv.View())

	for range 2 {
		fv, _ = fv.Update(nextMatchKeyMsg)
		expectedBottomFocused := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
			unfocusedStyle.Render("long "),
			focusedStyle.Render("long "),
			"[e...",
			footerStyle.Render("10..."),
		})
		internal.CmpStr(t, expectedBottomFocused, fv.View())
	}

	fv, _ = fv.Update(prevMatchKeyMsg)
	internal.CmpStr(t, expectedTopFocused, fv.View())
}

func TestToggleWrap(t *testing.T) {
	fv := makeFilterableViewport(
		20,
		6,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"the quick brown fox jumped over the lazy dog",
	}))
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "lazy" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// at first the match is in view
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"...ped over the " + focusedStyle.Render("l..."),
		"",
		"",
		"",
		"[exact] lazy  (1/...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// when we toggle wrapping here, the match happens to still be in view, but we don't force that
	// otherwise there would be surprising jumps if the user is scrolled away from the current match and toggles wrap
	fv.SetWrapText(true)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"the quick brown fox ",
		"jumped over the " + focusedStyle.Render("lazy"),
		" dog",
		"",
		"[exact] lazy  (1/...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// the match is out of view here, demonstrating the above comment
	fv.SetWrapText(false)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"the quick brown f...",
		"",
		"",
		"",
		"[exact] lazy  (1/...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestApplyFilterScrollsToFirstMatch(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"match here",
		"line 8",
	}))
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"No Filter",
		footerStyle.Render("37% (3/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 5",
		"line 6",
		focusedStyle.Render("match") + " here",
		"[exact] match  (1/1 matches...",
		footerStyle.Render("87% (7/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(cancelFilterKeyMsg)
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "lin" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("lin") + "e 1",
		unfocusedStyle.Render("lin") + "e 2",
		unfocusedStyle.Render("lin") + "e 3",
		"[exact] lin  (1/7 matches o...",
		footerStyle.Render("37% (3/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("lin") + "e 1",
		focusedStyle.Render("lin") + "e 2",
		unfocusedStyle.Render("lin") + "e 3",
		"[exact] lin  (2/7 matches o...",
		footerStyle.Render("37% (3/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('e'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("line") + " 1",
		unfocusedStyle.Render("line") + " 2",
		unfocusedStyle.Render("line") + " 3",
		"[exact] line  (1/7 matches ...",
		footerStyle.Render("37% (3/8)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestSetObjectsPreservesMatchIndex(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"match one",
		"match two",
		"match three",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("match") + " one",
		focusedStyle.Render("match") + " two",
		unfocusedStyle.Render("match") + " three",
		"[exact] match  (2/3 matches...",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// add a new item - should stay on match 2, now 2/4
	fv.SetObjects(stringsToItems([]string{
		"match one",
		"match new",
		"match two",
		"match three",
	}))

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("match") + " one",
		focusedStyle.Render("match") + " new",
		unfocusedStyle.Render("match") + " two",
		"[exact] match  (2/4 matches...",
		footerStyle.Render("75% (3/4)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestAppendObjectsPreservesMatchIndex(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"match one",
		"match two",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	fv, _ = fv.Update(nextMatchKeyMsg)
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("match") + " one",
		focusedStyle.Render("match") + " two",
		"",
		"[exact] match  (2/2 matches...",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// append new items - should stay on match 2, now 2/4
	fv.AppendObjects(stringsToItems([]string{
		"match three",
		"match four",
	}))

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("match") + " one",
		focusedStyle.Render("match") + " two",
		unfocusedStyle.Render("match") + " three",
		"[exact] match  (2/4 matches...",
		footerStyle.Render("75% (3/4)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestAppendObjectsWithNil(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"item one",
		"item two",
	}))

	// appending nil should not crash or change objects
	fv.AppendObjects(nil)

	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"item one",
		"item two",
		"",
		"No Filter",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestAppendObjectsRespectsMatchLimit(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMaxMatchLimit[object](5),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"match one",
		"match two",
		"match three",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// 3 matches, under limit
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("match") + " one",
		unfocusedStyle.Render("match") + " two",
		unfocusedStyle.Render("match") + " three",
		"[exact] match  (1/3 matches on 3 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// append 3 more items, which will exceed the limit of 5
	fv.AppendObjects(stringsToItems([]string{
		"match four",
		"match five",
		"match six",
	}))

	// should now show limit exceeded message and all items
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"match one",
		"match two",
		"match three",
		"[exact] match  (5+ matches on 6+ items)",
		footerStyle.Render("50% (3/6)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestAppendObjectsIncrementalWithMatchingItemsOnly(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](true),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"match one",
		"nothing here",
		"match two",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// should show only matching items
	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("match") + " one",
		unfocusedStyle.Render("match") + " two",
		"",
		"",
		"[exact] match  (1/2 matches on 2 item...",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// append mixed items (some matching, some not)
	fv.AppendObjects(stringsToItems([]string{
		"nothing",
		"match three",
		"also nothing",
		"match four",
	}))

	// should show only matching items, including new matches
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("match") + " one",
		unfocusedStyle.Render("match") + " two",
		unfocusedStyle.Render("match") + " three",
		unfocusedStyle.Render("match") + " four",
		"[exact] match  (1/4 matches on 4 item...",
		footerStyle.Render("100% (4/4)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestVerticalPadding(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		10,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{
			WithVerticalPad[object](2),
		},
	)

	// create many items so we can test padding
	items := make([]string, 50)
	for i := range 50 {
		if i == 10 || i == 20 || i == 30 {
			items[i] = fmt.Sprintf("match item %d", i)
		} else {
			items[i] = fmt.Sprintf("item %d", i)
		}
	}
	fv.SetObjects(stringsToItems(items))

	// apply filter to find "match"
	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// first match at item 10 should have at least 2 lines above and below
	// with 8 content lines and verticalPad=2, it shows items 5-12
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"item 5",
		"item 6",
		"item 7",
		"item 8",
		"item 9",
		focusedStyle.Render("match") + " item 10",
		"item 11",
		"item 12",
		"[exact] match  (1/3 matches...",
		footerStyle.Render("26% (13/50)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// navigate to second match at item 20
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"item 15",
		"item 16",
		"item 17",
		"item 18",
		"item 19",
		focusedStyle.Render("match") + " item 20",
		"item 21",
		"item 22",
		"[exact] match  (2/3 matches...",
		footerStyle.Render("46% (23/50)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestHorizontalPadding(t *testing.T) {
	fv := makeFilterableViewport(
		10,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{
			WithHorizontalPad[object](3),
		},
	)

	fv.SetObjects(stringsToItems([]string{
		"short goose text with some more words here",
		"another goose line with extra padding test",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "goose" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// first match attempted padding of 3 on each side
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		".." + focusedStyle.Render(".oose") + "...",
		"... " + unfocusedStyle.Render("goo..") + ".",
		"",
		"[exact]...",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// second match attempted padding of 3 on each side
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("...se") + " t...",
		".." + focusedStyle.Render(".oose") + "...",
		"",
		"[exact]...",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestMatchNavigationWithVerticalPadding(t *testing.T) {
	h := 34
	fv := makeFilterableViewport(
		100,
		h,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{
			WithVerticalPad[object](10),
		},
	)

	nItems := 50
	items := make([]string, nItems)
	for i := range nItems {
		items[i] = "hi"
	}
	fv.SetObjects(stringsToItems(items))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "hi" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedStrings := []string{
		focusedStyle.Render("hi"),
	}
	for i := 0; i < h-3; i++ { // -3 for filter line, focused line, & footer
		expectedStrings = append(expectedStrings, unfocusedStyle.Render("hi"))
	}
	expectedStrings = append(expectedStrings, "[exact] hi  (1/50 matches on 50 items)")
	expectedStrings = append(expectedStrings, footerStyle.Render("64% (32/50)"))
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), expectedStrings)
	internal.CmpStr(t, expectedView, fv.View())

	// go to bottom match, then previous match 21 times to reach the 10 padding above
	fv, _ = fv.Update(prevMatchKeyMsg)
	nPrev := 21
	for range nPrev {
		fv, _ = fv.Update(prevMatchKeyMsg)
	}
	expectedStrings = []string{}
	for range 10 {
		expectedStrings = append(expectedStrings, unfocusedStyle.Render("hi"))
	}
	expectedStrings = append(expectedStrings, focusedStyle.Render("hi"))
	for i := 0; i < h-10-3; i++ {
		expectedStrings = append(expectedStrings, unfocusedStyle.Render("hi"))
	}
	expectedStrings = append(expectedStrings, "[exact] hi  (29/50 matches on 50 items)")
	expectedStrings = append(expectedStrings, footerStyle.Render("100% (50/50)"))
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), expectedStrings)
	internal.CmpStr(t, expectedView, fv.View())

	// next previous match should keep 10 lines above and scroll one up
	fv, _ = fv.Update(prevMatchKeyMsg)
	expectedStrings = []string{}
	for range 10 {
		expectedStrings = append(expectedStrings, unfocusedStyle.Render("hi"))
	}
	expectedStrings = append(expectedStrings, focusedStyle.Render("hi"))
	for i := 0; i < h-10-3; i++ {
		expectedStrings = append(expectedStrings, unfocusedStyle.Render("hi"))
	}
	expectedStrings = append(expectedStrings, "[exact] hi  (28/50 matches on 50 items)")
	expectedStrings = append(expectedStrings, footerStyle.Render("98% (49/50)"))
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), expectedStrings)
	internal.CmpStr(t, expectedView, fv.View())
}

func TestMatchNavigationRolloverWithVerticalPadding(t *testing.T) {
	fv := makeFilterableViewport(
		100,
		10,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{
			WithVerticalPad[object](10),
		},
	)
	fv.SetSelectionEnabled(true)

	nItems := 20
	items := make([]string, nItems)
	for i := range nItems {
		items[i] = "hi"
	}
	fv.SetObjects(stringsToItems(items))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "hi" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		"[exact] hi  (1/20 matches on 20 items)",
		footerStyle.Render("5% (1/20)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// previous match (last one)
	fv, _ = fv.Update(prevMatchKeyMsg)
	expectedViewAfterScroll := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		unfocusedStyle.Render("hi"),
		focusedStyle.Render("hi"),
		"[exact] hi  (20/20 matches on 20 items)",
		footerStyle.Render("100% (20/20)"),
	})
	internal.CmpStr(t, expectedViewAfterScroll, fv.View())
}

func stringsToItems(vals []string) []object {
	items := make([]object, len(vals))
	for i, s := range vals {
		items[i] = object{item: item.NewItem(s)}
	}
	return items
}

func TestSelectionAndFocusedMatchAfterItemsChange(t *testing.T) {
	fv := makeFilterableViewport(
		100,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
			viewport.WithSelectionEnabled[object](true),
		},
		[]Option[object]{},
	)

	initialItems := []string{
		"1 2",
		"1 2",
		"1 2",
		"1 2",
		"1 2",
	}
	fv.SetObjects(stringsToItems(initialItems))

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('1'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// focus second match
	fv, _ = fv.Update(nextMatchKeyMsg)

	// move selection to third item
	fv, _ = fv.Update(downKeyMsg)

	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("1") + " 2",
		focusedStyle.Render("1") + " 2",
		unfocusedStyle.Render("1") + selectedItemStyle.Render(" 2"),
		"[exact] 1  (2/5 matches on 5 items)",
		footerStyle.Render("60% (3/5)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// add a new item
	initialItems = append(initialItems, "1 2")
	fv.SetObjects(stringsToItems(initialItems))

	// neither match nor selection should change
	expected = strings.ReplaceAll(expected, "2/5 matches on 5", "2/6 matches on 6")
	expected = strings.ReplaceAll(expected, "60% (3/5)", "50% (3/6)")
	internal.CmpStr(t, expected, fv.View())

	// changing match should change selection too
	fv, _ = fv.Update(nextMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("1") + " 2",
		unfocusedStyle.Render("1") + " 2",
		focusedStyle.Render("1") + selectedItemStyle.Render(" 2"),
		"[exact] 1  (3/6 matches on 6 items)",
		footerStyle.Render("50% (3/6)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(prevMatchKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("1") + " 2",
		focusedStyle.Render("1") + selectedItemStyle.Render(" 2"),
		unfocusedStyle.Render("1") + " 2",
		"[exact] 1  (2/6 matches on 6 items)",
		footerStyle.Render("33% (2/6)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestCurrentMatchNotCenteredAfterItemsChange(t *testing.T) {
	fv := makeFilterableViewport(
		100,
		4,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)

	initialItems := []string{
		"1",
		"2",
		"3",
		"4",
		"5",
		"6",
	}
	fv.SetObjects(stringsToItems(initialItems))

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('1'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("1"),
		"2",
		"[exact] 1  (1/1 matches on 1 items)",
		footerStyle.Render("33% (2/6)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// scroll so focused match out of view
	fv, _ = fv.Update(downKeyMsg)
	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"2",
		"3",
		"[exact] 1  (1/1 matches on 1 items)",
		footerStyle.Render("50% (3/6)"),
	})
	internal.CmpStr(t, expected, fv.View())

	initialItems = append(initialItems, "7", "8", "9")
	fv.SetObjects(stringsToItems(initialItems))

	newExpected := strings.ReplaceAll(expected, "50% (3/6)", "33% (3/9)")
	internal.CmpStr(t, newExpected, fv.View())
}

func TestMaxMatchLimit(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithMaxMatchLimit[object](5),
			WithMatchingItemsOnly[object](true), // Should be ignored when limit exceeded
		},
	)

	items := []string{
		"apple apple",
		"apple apple",
		"apple apple",
		"apple apple",
		"apple apple",
		"banana",
	}
	fv.SetObjects(stringsToItems(items))

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "app" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple apple",
		"apple apple",
		"apple apple",
		"apple apple",
		"[exact] Filter: app  (5+ matches on 3+ items)",
		footerStyle.Render("66% (4/6)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// view should be unchanged by navigating matches when limit exceeded
	fv, _ = fv.Update(nextMatchKeyMsg)
	internal.CmpStr(t, expectedView, fv.View())

	// clear search filter
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(cancelFilterKeyMsg)

	if fv.matchLimitExceeded {
		t.Error("matchLimitExceeded should be false after clearing filter")
	}

	// filter that doesn't exceed limit
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('b'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("b") + "anana",
		"",
		"",
		"",
		"[exact] Filter: b  (1/1 matches on 1 items) showing matches only",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestMaxMatchLimitWithAppendObjects(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		3,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithMaxMatchLimit[object](3),
		},
	)

	items := []string{
		"a",
		"bbb",
	}
	fv.SetObjects(stringsToItems(items))

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("a"),
		"[exact] Filter: a  (1/1 matches on 1 items)",
		footerStyle.Render("50% (1/2)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// append new items that cause match limit to be exceeded
	fv.AppendObjects(stringsToItems([]string{"aaa", "aaa"}))

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"a",
		"[exact] Filter: a  (3+ matches on 2+ items)",
		footerStyle.Render("25% (1/4)"),
	})
	internal.CmpStr(t, expected, fv.View())
}

func TestMaxMatchLimitUnlimited(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithMaxMatchLimit[object](0), // unlimited
		},
	)

	fv.SetObjects(stringsToItems([]string{
		"apple apple",
	}))

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("a") + "pple " + unfocusedStyle.Render("a") + "pple",
		"",
		"",
		"",
		"[exact] Filter: a  (1/2 matches on 1 items)",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestToggleWrap_DoesNotJumpToMatchWhenScrolledAway(t *testing.T) {
	fv := makeFilterableViewport(
		30,
		5,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
			viewport.WithSelectionEnabled[object](true),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"match here",
		"line 8",
	}))

	expected := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		selectedItemStyle.Render("line 1"),
		"line 2",
		"line 3",
		"No Filter",
		footerStyle.Render("12% (1/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(filterKeyMsg)
	for _, c := range "match" {
		fv, _ = fv.Update(internal.MakeKeyMsg(c))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 5",
		"line 6",
		focusedStyle.Render("match") + selectedItemStyle.Render(" here"),
		"[exact] match  (1/1 matches...",
		footerStyle.Render("87% (7/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	fv, _ = fv.Update(internal.MakeKeyMsg('g'))

	expected = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		selectedItemStyle.Render("line 1"),
		"line 2",
		"line 3",
		"[exact] match  (1/1 matches...",
		footerStyle.Render("12% (1/8)"),
	})
	internal.CmpStr(t, expected, fv.View())

	// toggling wrap should not change view
	fv.SetWrapText(true)
	internal.CmpStr(t, expected, fv.View())
	fv.SetWrapText(false)
	internal.CmpStr(t, expected, fv.View())
}

func TestFilterLineAtBottom(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Filter line should appear just above footer, not at top
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"No Filter",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Apply a filter - filter line still at bottom
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('l'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("l") + "ine 1",
		unfocusedStyle.Render("l") + "ine 2",
		unfocusedStyle.Render("l") + "ine 3",
		"[exact] Filter: l  (1/3 matches on 3 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestEmptyTextAtBottom(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No active filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Empty text should appear just above footer when filter mode is off
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"No active filter",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionWithWrap(t *testing.T) {
	fv := makeFilterableViewport(
		15,
		7,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{
			WithEmptyText[object]("None"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"short",
		"longer text that wraps",
	}))

	// Filter line should appear just above footer, after wrapped content
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"short",
		"longer text tha",
		"t wraps",
		"",
		"",
		"None",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionDuringEditing(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Enter filter editing mode
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('t'))
	fv, _ = fv.Update(internal.MakeKeyMsg('e'))
	fv, _ = fv.Update(internal.MakeKeyMsg('s'))
	fv, _ = fv.Update(internal.MakeKeyMsg('t'))

	// Cursor should appear in filter line at bottom
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"[exact] Filter: test" + cursorStyle.Render(" ") + " (no matches)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestHeightConsistencyAfterRefactor(t *testing.T) {
	widths := []int{10, 20, 50}
	heights := []int{3, 5, 10, 20}

	for _, w := range widths {
		for _, h := range heights {
			fv := makeFilterableViewport(
				w,
				h,
				[]viewport.Option[object]{},
				[]Option[object]{},
			)

			// Verify GetHeight returns same value as SetHeight input
			if got := fv.GetHeight(); got != h {
				t.Errorf("width=%d height=%d: GetHeight() = %d, want %d", w, h, got, h)
			}

			// Verify GetWidth returns same value
			if got := fv.GetWidth(); got != w {
				t.Errorf("width=%d height=%d: GetWidth() = %d, want %d", w, h, got, w)
			}

			// Set new dimensions and verify
			newH := h + 5
			fv.SetHeight(newH)
			if got := fv.GetHeight(); got != newH {
				t.Errorf("after SetHeight(%d): GetHeight() = %d, want %d", newH, got, newH)
			}

			newW := w + 10
			fv.SetWidth(newW)
			if got := fv.GetWidth(); got != newW {
				t.Errorf("after SetWidth(%d): GetWidth() = %d, want %d", newW, got, newW)
			}
		}
	}
}

func TestContentStartsAtTop(t *testing.T) {
	fv := makeFilterableViewport(
		40,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
	}))

	// Content should start at the very top of the viewport
	// not shifted down by any filter header
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 1", // Content starts at top
		"line 2",
		"line 3",
		"line 4",
		"No Filter",                      // Filter line just above footer
		footerStyle.Render("100% (4/4)"), // Footer at bottom
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_ExactMode(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))

	fv.SetFilter("apple", FilterExact)

	if fv.GetFilterText() != "apple" {
		t.Errorf("expected filter text 'apple', got '%s'", fv.GetFilterText())
	}
	if fv.GetActiveFilterMode().Name != FilterExact {
		t.Errorf("expected active filter mode %q, got %q", FilterExact, fv.GetActiveFilterMode().Name)
	}

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_RegexMode(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apricot tart",
	}))

	fv.SetFilter("ap.*e", FilterRegex)

	if fv.GetFilterText() != "ap.*e" {
		t.Errorf("expected filter text 'ap.*e', got '%s'", fv.GetFilterText())
	}
	if fv.GetActiveFilterMode().Name != FilterRegex {
		t.Errorf("expected active filter mode %q, got %q", FilterRegex, fv.GetActiveFilterMode().Name)
	}

	// regex ap.*e matches "apple pie" (greedy match to the last 'e')
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple pie"),
		"banana bread",
		"apricot tart",
		"[regex] ap.*e  (1/1 matches on 1 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_ClearsFilterWhenEmpty(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
	}))

	// First set a filter
	fv.SetFilter("apple", FilterExact)
	if fv.GetFilterText() != "apple" {
		t.Errorf("expected filter text 'apple', got '%s'", fv.GetFilterText())
	}

	// Then clear it
	fv.SetFilter("", "")
	if fv.GetFilterText() != "" {
		t.Errorf("expected empty filter text, got '%s'", fv.GetFilterText())
	}

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple pie",
		"banana bread",
		"",
		"No Filter",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_SwitchBetweenModes(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"test123",
		"test456",
	}))

	// Start with exact mode
	fv.SetFilter("test", FilterExact)
	if fv.GetActiveFilterMode().Name != FilterExact {
		t.Errorf("expected active filter mode %q, got %q", FilterExact, fv.GetActiveFilterMode().Name)
	}

	// Switch to regex mode with same filter
	fv.SetFilter("test\\d+", FilterRegex)
	if fv.GetActiveFilterMode().Name != FilterRegex {
		t.Errorf("expected active filter mode %q, got %q", FilterRegex, fv.GetActiveFilterMode().Name)
	}
	if fv.GetFilterText() != "test\\d+" {
		t.Errorf("expected filter text 'test\\d+', got '%s'", fv.GetFilterText())
	}

	// Both lines should match the regex
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("test123"),
		unfocusedStyle.Render("test456"),
		"",
		"[regex] test\\d+ (1/2 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_WithMatchingItemsOnly(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](true),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))

	fv.SetFilter("apple", FilterExact)

	// Only matching items should be shown
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		unfocusedStyle.Render("apple") + " cake",
		"",
		"[exact] apple  (1/2 matches on 2 items) showing matches only",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetMatchingItemsOnly_EnableShowsOnlyMatches(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](false), // start with all items shown
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))
	fv.SetFilter("apple", FilterExact)

	// Initially all items shown
	if fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be false initially")
	}
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Enable matching items only
	fv.SetMatchingItemsOnly(true)

	if !fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be true after SetMatchingItemsOnly(true)")
	}
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		unfocusedStyle.Render("apple") + " cake",
		"",
		"[exact] apple  (1/2 matches on 2 items) showing matches only",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetMatchingItemsOnly_DisableShowsAllItems(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](true), // start with matches only
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))
	fv.SetFilter("apple", FilterExact)

	// Initially only matching items shown
	if !fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be true initially")
	}
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		unfocusedStyle.Render("apple") + " cake",
		"",
		"[exact] apple  (1/2 matches on 2 items) showing matches only",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Disable matching items only
	fv.SetMatchingItemsOnly(false)

	if fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be false after SetMatchingItemsOnly(false)")
	}
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetMatchingItemsOnly_ToggleBackAndForth(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"apricot",
	}))
	fv.SetFilter("a", FilterExact)

	// Default is false
	if fv.GetMatchingItemsOnly() {
		t.Error("expected default GetMatchingItemsOnly to be false")
	}

	// Toggle to true
	fv.SetMatchingItemsOnly(true)
	if !fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be true")
	}

	// Toggle back to false
	fv.SetMatchingItemsOnly(false)
	if fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be false")
	}

	// Toggle to true again
	fv.SetMatchingItemsOnly(true)
	if !fv.GetMatchingItemsOnly() {
		t.Error("expected GetMatchingItemsOnly to be true")
	}
}

func TestSetMatchingItemsOnly_NoEffectWithoutFilter(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"cherry",
	}))

	// Set matching items only without a filter - all items should still show
	fv.SetMatchingItemsOnly(true)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"cherry",
		"No Filter",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilterableViewportStyles_ChangesMatchStyles(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple pie",
		"banana bread",
		"apple cake",
	}))
	fv.SetFilter("apple", FilterExact)

	// Verify initial styles are applied
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("apple") + " pie",
		"banana bread",
		unfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Change to new styles
	newFocusedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("2"))
	newUnfocusedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Background(lipgloss.Color("4"))
	fv.SetFilterableViewportStyles(Styles{
		Match: MatchStyles{
			Focused:   newFocusedStyle,
			Unfocused: newUnfocusedStyle,
		},
	})

	// Verify new styles are applied
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		newFocusedStyle.Render("apple") + " pie",
		"banana bread",
		newUnfocusedStyle.Render("apple") + " cake",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilterableViewportStyles_UpdatesExistingHighlights(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"test one",
		"test two",
		"test three",
	}))
	fv.SetFilter("test", FilterExact)

	// Navigate to second match
	fv, _ = fv.Update(nextMatchKeyMsg)

	// Now second match should be focused
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		unfocusedStyle.Render("test") + " one",
		focusedStyle.Render("test") + " two",
		unfocusedStyle.Render("test") + " three",
		"[exact] test  (2/3 matches on 3 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Change styles - should update all highlights including the focused one
	newFocusedStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	newUnfocusedStyle := lipgloss.NewStyle().Italic(true)
	fv.SetFilterableViewportStyles(Styles{
		Match: MatchStyles{
			Focused:   newFocusedStyle,
			Unfocused: newUnfocusedStyle,
		},
	})

	// Verify new styles applied with correct focus
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		newUnfocusedStyle.Render("test") + " one",
		newFocusedStyle.Render("test") + " two",
		newUnfocusedStyle.Render("test") + " three",
		"[exact] test  (2/3 matches on 3 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestAdjustObjectsForFilter_CalledOnFilterChange(t *testing.T) {
	var hookCalls []struct {
		filterText string
		mode       FilterModeName
	}

	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithAdjustObjectsForFilter[object](func(filterText string, mode FilterModeName) []object {
				hookCalls = append(hookCalls, struct {
					filterText string
					mode       FilterModeName
				}{filterText, mode})
				return nil // return nil to keep existing objects
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))

	// Start filter mode and type
	fv, _ = fv.Update(filterKeyMsg)
	_, _ = fv.Update(internal.MakeKeyMsg('a'))

	if len(hookCalls) < 1 {
		t.Fatal("expected hook to be called at least once")
	}

	// Check last call has correct filter text and mode
	lastCall := hookCalls[len(hookCalls)-1]
	if lastCall.filterText != "a" {
		t.Errorf("expected filterText 'a', got %q", lastCall.filterText)
	}
	if lastCall.mode != FilterExact {
		t.Errorf("expected mode %q (exact), got %q", FilterExact, lastCall.mode)
	}
}

func TestAdjustObjectsForFilter_CalledWithRegexMode(t *testing.T) {
	var lastMode FilterModeName

	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithAdjustObjectsForFilter[object](func(_ string, mode FilterModeName) []object {
				lastMode = mode
				return nil
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))

	// Start regex filter mode
	fv, _ = fv.Update(regexFilterKeyMsg)
	_, _ = fv.Update(internal.MakeKeyMsg('a'))

	if lastMode != FilterRegex {
		t.Errorf("expected mode %q (regex), got %q", FilterRegex, lastMode)
	}
}

func TestAdjustObjectsForFilter_ReplacesObjects(t *testing.T) {
	// Hook returns a different set of objects based on filter
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](false),
			WithAdjustObjectsForFilter[object](func(filterText string, _ FilterModeName) []object {
				if filterText == "" {
					return stringsToItems([]string{"apple", "banana", "cherry"})
				}
				// When filtering, return parent + matching child (like a tree)
				return stringsToItems([]string{"parent", "child-apple"})
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana", "cherry"}))

	// Before filter: should show original objects
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"apple",
		"banana",
		"cherry",
		"",
		"No Filter",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Apply filter with "a"
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// After filter: should show hook's objects with "a" highlighted
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"p" + focusedStyle.Render("a") + "rent",
		"child-" + unfocusedStyle.Render("a") + "pple",
		"",
		"",
		"[exact] a  (1/2 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestAdjustObjectsForFilter_NilKeepsExistingObjects(t *testing.T) {
	hookCallCount := 0

	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](false),
			WithAdjustObjectsForFilter[object](func(_ string, _ FilterModeName) []object {
				hookCallCount++
				return nil // explicitly return nil
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))

	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// hook is called twice: once when mode activates (empty text), once when text changes to "a"
	if hookCallCount != 2 {
		t.Errorf("hook should have been called twice, got %d", hookCallCount)
	}

	// Original objects should still be shown, with "a" highlighted
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("a") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		"",
		"[exact] a  (1/4 matches on 2 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestAdjustObjectsForFilter_WithMatchingItemsOnlyTrue(t *testing.T) {
	// Hook provides objects, but only matching ones should be shown
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](true),
			WithAdjustObjectsForFilter[object](func(_ string, _ FilterModeName) []object {
				// Return parent + child, but only child matches "apple"
				return stringsToItems([]string{"parent-node", "child-apple"})
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"initial"}))
	fv.SetFilter("apple", FilterExact)

	// Only child-apple matches "apple", so only it should be shown
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"child-" + focusedStyle.Render("apple"),
		"",
		"",
		"[exact] apple  (1/1 matches on 1 items) showing matches only",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestAdjustObjectsForFilter_WithMatchingItemsOnlyFalse(t *testing.T) {
	// Hook provides objects, all should be shown (matches highlighted)
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](false),
			WithAdjustObjectsForFilter[object](func(_ string, _ FilterModeName) []object {
				return stringsToItems([]string{"parent-node", "child-apple"})
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"initial"}))
	fv.SetFilter("apple", FilterExact)

	// Both should be visible, child-apple has match highlighted
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"parent-node",
		"child-" + focusedStyle.Render("apple"),
		"",
		"[exact] apple  (1/1 matches on 1 items)",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestAdjustObjectsForFilter_MatchNavigationWorks(t *testing.T) {
	// Verify n/N navigation works with hook-provided objects
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithMatchingItemsOnly[object](false),
			WithAdjustObjectsForFilter[object](func(_ string, _ FilterModeName) []object {
				return stringsToItems([]string{
					"first-apple",
					"no-match-here",
					"second-apple",
				})
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"initial"}))
	fv.SetFilter("apple", FilterExact)

	// Should show "1/2 matches" (two items contain "apple"), first match focused
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"first-" + focusedStyle.Render("apple"),
		"no-match-here",
		"second-" + unfocusedStyle.Render("apple"),
		"",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Navigate to next match
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"first-" + unfocusedStyle.Render("apple"),
		"no-match-here",
		"second-" + focusedStyle.Render("apple"),
		"",
		"[exact] apple  (2/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Navigate to previous match
	fv, _ = fv.Update(prevMatchKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"first-" + focusedStyle.Render("apple"),
		"no-match-here",
		"second-" + unfocusedStyle.Render("apple"),
		"",
		"[exact] apple  (1/2 matches on 2 items)",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestAdjustObjectsForFilter_ClearFilterRestoresOriginalBehavior(t *testing.T) {
	callCount := 0
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithAdjustObjectsForFilter[object](func(filterText string, _ FilterModeName) []object {
				callCount++
				if filterText != "" {
					return stringsToItems([]string{"hook-provided"})
				}
				return stringsToItems([]string{"original-a", "original-b"})
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"original-a", "original-b"}))

	// Apply a filter
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('x'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"hook-provided",
		"",
		"",
		"[exact] x  (no matches)",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Clear filter
	fv, _ = fv.Update(cancelFilterKeyMsg)

	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"original-a",
		"original-b",
		"",
		"No Filter",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_SelectionAtBottomWithBottomSticky(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{
			viewport.WithSelectionEnabled[object](true),
			viewport.WithStickyBottom[object](true),
		},
		[]Option[object]{},
	)

	items := stringsToItems([]string{
		"error: something broke",
		"info: all good",
		"info: still good",
		"info: yep good",
		"error: another problem",
		"info: fine",
		"info: ok",
		"info: last line",
	})
	fv.SetObjects(items)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"error: another problem",
		"info: fine",
		"info: ok",
		selectedItemStyle.Render("info: last line"),
		"No Filter",
		footerStyle.Render("100% (8/8)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// apply filter - should move selection to the first match
	fv.SetFilter("error", FilterExact)

	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("error") + selectedItemStyle.Render(": something broke"),
		"info: all good",
		"info: still good",
		"info: yep good",
		"[exact] error  (1/2 matches on 2 items)",
		footerStyle.Render("12% (1/8)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestSetFilter_SelectionAtBottomWithBottomSticky_AppendDoesNotJump(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{
			viewport.WithSelectionEnabled[object](true),
			viewport.WithStickyBottom[object](true),
		},
		[]Option[object]{},
	)

	items := stringsToItems([]string{
		"error: something broke",
		"info: all good",
		"info: still good",
		"info: yep good",
		"error: another problem",
		"info: fine",
		"info: ok",
		"info: last line",
	})
	fv.SetObjects(items)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"error: another problem",
		"info: fine",
		"info: ok",
		selectedItemStyle.Render("info: last line"),
		"No Filter",
		footerStyle.Render("100% (8/8)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// apply filter while selection is at bottom
	fv.SetFilter("error", FilterExact)

	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("error") + selectedItemStyle.Render(": something broke"),
		"info: all good",
		"info: still good",
		"info: yep good",
		"[exact] error  (1/2 matches on 2 items)",
		footerStyle.Render("12% (1/8)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// append new logs - selection should stay at the first match, not jump to bottom
	fv.AppendObjects(stringsToItems([]string{
		"error: whoops",
	}))
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		focusedStyle.Render("error") + selectedItemStyle.Render(": something broke"),
		"info: all good",
		"info: still good",
		"info: yep good",
		"[exact] error  (1/3 matches on 3 items)",
		footerStyle.Render("11% (1/9)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

// TestCustomFilterMode verifies that a custom filter mode with a custom MatchFunc works correctly.
func TestCustomFilterMode(t *testing.T) {
	// Custom filter mode: matches only lines that start with the filter text
	prefixMode := FilterMode{
		Name: "prefix",
		Key: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prefix filter"),
		),
		Label: "[prefix]",
		GetMatchFunc: func(filterText string) (MatchFunc, error) {
			return func(content string) []item.ByteRange {
				if strings.HasPrefix(content, filterText) {
					return []item.ByteRange{{Start: 0, End: len(filterText)}}
				}
				return nil
			}, nil
		},
	}

	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object]([]FilterMode{prefixMode}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"alpha one",
		"beta two alpha",
		"alpha three",
	}))

	// Activate custom mode with 'p'
	fv, _ = fv.Update(internal.MakeKeyMsg('p'))
	if fv.GetActiveFilterMode().Name != "prefix" {
		t.Fatalf("expected active mode 'prefix', got %q", fv.GetActiveFilterMode().Name)
	}
	if fv.GetActiveFilterMode().Label != "[prefix]" {
		t.Fatalf("expected label '[prefix]', got %q", fv.GetActiveFilterMode().Label)
	}

	// Type "alpha"
	for _, ch := range "alpha" {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// Should have 2 matches (alpha one and alpha three)
	if fv.totalMatchesOnAllItems != 2 {
		t.Errorf("expected 2 total matches, got %d", fv.totalMatchesOnAllItems)
	}
	if fv.numMatchingItems != 2 {
		t.Errorf("expected 2 matching items, got %d", fv.numMatchingItems)
	}
}

// TestCustomFilterModeWithError verifies that a custom filter mode returning an error shows no matches.
func TestCustomFilterModeWithError(t *testing.T) {
	errorMode := FilterMode{
		Name: "error",
		Key: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "error filter"),
		),
		Label: "[error]",
		GetMatchFunc: func(_ string) (MatchFunc, error) {
			return nil, fmt.Errorf("always fails")
		},
	}

	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object]([]FilterMode{errorMode}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
	}))

	fv, _ = fv.Update(internal.MakeKeyMsg('e'))
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// Error mode should result in 0 matches
	if fv.totalMatchesOnAllItems != 0 {
		t.Errorf("expected 0 matches with error mode, got %d", fv.totalMatchesOnAllItems)
	}
}

func TestFuzzyFilterMode(t *testing.T) {
	fuzzyMode := FuzzyFilterMode(key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fuzzy filter"),
	))

	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object]([]FilterMode{fuzzyMode}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"hello world",
		"help wanted",
		"goodbye",
		"hxexlxlxo",
	}))

	// Activate fuzzy mode
	fv, _ = fv.Update(internal.MakeKeyMsg('f'))
	if fv.GetActiveFilterMode().Label != "[fuzzy]" {
		t.Fatalf("expected label '[fuzzy]', got %q", fv.GetActiveFilterMode().Label)
	}

	// Type "hlo" — should match "hello world" (h-e-l-l-o), "hxexlxlxo" (h-x-e-x-l-x-l-x-o)
	// but not "help wanted" (no 'o' after 'l') or "goodbye" (no 'h')
	for _, ch := range "hlo" {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	if fv.numMatchingItems != 2 {
		t.Errorf("expected 2 matching items, got %d", fv.numMatchingItems)
	}
}

func TestFuzzyFilterModeNoMatch(t *testing.T) {
	fuzzyMode := FuzzyFilterMode(key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fuzzy filter"),
	))

	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object]([]FilterMode{fuzzyMode}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"abc",
		"def",
	}))

	fv, _ = fv.Update(internal.MakeKeyMsg('f'))
	for _, ch := range "xyz" {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	if fv.numMatchingItems != 0 {
		t.Errorf("expected 0 matching items, got %d", fv.numMatchingItems)
	}
}

func TestFuzzyFilterModeCaseInsensitive(t *testing.T) {
	fuzzyMode := FuzzyFilterMode(key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fuzzy filter"),
	))

	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object]([]FilterMode{fuzzyMode}),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"Hello World",
		"HELLO",
		"goodbye",
	}))

	fv, _ = fv.Update(internal.MakeKeyMsg('f'))
	for _, ch := range "helo" {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// Should match "Hello World" and "HELLO" (case-insensitive)
	if fv.numMatchingItems != 2 {
		t.Errorf("expected 2 matching items, got %d", fv.numMatchingItems)
	}
}

func TestFuzzyFilterModeEmptyFilter(t *testing.T) {
	mode := FuzzyFilterMode(key.NewBinding(key.WithKeys("f")))
	matchFn, err := mode.GetMatchFunc("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty filter should return nil (no matches highlighted)
	ranges := matchFn("hello")
	if ranges != nil {
		t.Errorf("expected nil for empty filter, got %+v", ranges)
	}
}

func TestFuzzyFilterModeHighlightRanges(t *testing.T) {
	mode := FuzzyFilterMode(key.NewBinding(key.WithKeys("f")))
	matchFn, err := mode.GetMatchFunc("hlo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "hello world" — h(0) to o(4), single span [0, 5)
	ranges := matchFn("hello world")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0] != (item.ByteRange{Start: 0, End: 5}) {
		t.Errorf("expected {0, 5}, got %+v", ranges[0])
	}

	// No match
	ranges = matchFn("goodbye")
	if ranges != nil {
		t.Errorf("expected nil for non-matching content, got %+v", ranges)
	}
}

func TestFuzzyFilterModeUnicode(t *testing.T) {
	mode := FuzzyFilterMode(key.NewBinding(key.WithKeys("f")))
	matchFn, err := mode.GetMatchFunc("über")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "ü--b--e--r" — ü is 2 bytes, so total is 11 bytes; span from ü(0) to r(10-11)
	ranges := matchFn("ü--b--e--r")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0] != (item.ByteRange{Start: 0, End: 11}) {
		t.Errorf("expected {0, 11}, got %+v", ranges[0])
	}
}

// TestModeSwitching verifies that switching between filter modes preserves the filter text
// and re-evaluates matches.
func TestModeSwitching(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"Hello World",
		"hello world",
		"HELLO WORLD",
	}))

	// Activate exact mode and type "hello"
	fv, _ = fv.Update(filterKeyMsg) // '/'
	for _, ch := range "hello" {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	if fv.GetActiveFilterMode().Name != FilterExact {
		t.Fatalf("expected exact mode, got %q", fv.GetActiveFilterMode().Name)
	}
	// Exact match should find only "hello world" (case-sensitive)
	exactMatchCount := fv.totalMatchesOnAllItems
	if exactMatchCount != 1 {
		t.Fatalf("expected 1 exact match, got %d", exactMatchCount)
	}

	// Cancel and switch to case-insensitive mode
	fv, _ = fv.Update(cancelFilterKeyMsg)
	fv, _ = fv.Update(caseInsensitiveFilterKeyMsg) // 'i'
	if fv.GetActiveFilterMode().Name != FilterCaseInsensitive {
		t.Fatalf("expected case-insensitive mode, got %q", fv.GetActiveFilterMode().Name)
	}

	// Type "hello" again
	for _, ch := range "hello" {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// Case-insensitive should match all 3 items
	if fv.totalMatchesOnAllItems != 3 {
		t.Errorf("expected 3 case-insensitive matches, got %d", fv.totalMatchesOnAllItems)
	}

	// Cancel and switch to regex mode
	fv, _ = fv.Update(cancelFilterKeyMsg)
	fv, _ = fv.Update(regexFilterKeyMsg) // 'r'
	if fv.GetActiveFilterMode().Name != FilterRegex {
		t.Fatalf("expected regex mode, got %q", fv.GetActiveFilterMode().Name)
	}

	// Type regex pattern
	for _, ch := range `^[hH]ello` {
		fv, _ = fv.Update(internal.MakeKeyMsg(ch))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)

	// Should match "Hello World" and "hello world" but not "HELLO WORLD"
	if fv.totalMatchesOnAllItems != 2 {
		t.Errorf("expected 2 regex matches for ^[hH]ello, got %d", fv.totalMatchesOnAllItems)
	}
}

// TestSetFilterWithVariousModes verifies that SetFilter works with different filter modes.
func TestSetFilterWithVariousModes(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"Hello World",
		"hello world",
		"HELLO WORLD",
	}))

	// SetFilter with exact mode
	fv.SetFilter("hello", FilterExact)
	if fv.GetActiveFilterMode().Name != FilterExact {
		t.Errorf("expected mode %q, got %q", FilterExact, fv.GetActiveFilterMode().Name)
	}
	if fv.GetFilterText() != "hello" {
		t.Errorf("expected filter text 'hello', got %q", fv.GetFilterText())
	}
	if fv.totalMatchesOnAllItems != 1 {
		t.Errorf("expected 1 exact match, got %d", fv.totalMatchesOnAllItems)
	}

	// SetFilter with regex mode
	fv.SetFilter("HELLO", FilterRegex)
	if fv.GetActiveFilterMode().Name != FilterRegex {
		t.Errorf("expected mode %q, got %q", FilterRegex, fv.GetActiveFilterMode().Name)
	}
	if fv.totalMatchesOnAllItems != 1 {
		t.Errorf("expected 1 regex match for 'HELLO', got %d", fv.totalMatchesOnAllItems)
	}

	// SetFilter with case-insensitive mode
	fv.SetFilter("hello", FilterCaseInsensitive)
	if fv.GetActiveFilterMode().Name != FilterCaseInsensitive {
		t.Errorf("expected mode %q, got %q", FilterCaseInsensitive, fv.GetActiveFilterMode().Name)
	}
	if fv.totalMatchesOnAllItems != 3 {
		t.Errorf("expected 3 case-insensitive matches, got %d", fv.totalMatchesOnAllItems)
	}

	// SetFilter with empty string clears filter
	fv.SetFilter("", "")
	if fv.GetActiveFilterMode() != nil {
		t.Errorf("expected nil active filter mode after empty filter, got %q", fv.GetActiveFilterMode().Name)
	}
	if fv.filterMode != filterModeOff {
		t.Errorf("expected filterModeOff after empty filter, got %d", fv.filterMode)
	}

	// SetFilter with unknown mode name should be ignored (keeps current mode)
	fv.SetFilter("test", "nonexistent")
	if fv.GetActiveFilterMode() != nil {
		t.Errorf("expected nil active filter mode for unknown mode, got %q", fv.GetActiveFilterMode().Name)
	}
}

// TestFilterModesAccessor verifies the FilterModes() accessor.
func TestFilterModesAccessor(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)

	modes := fv.FilterModes()
	if len(modes) != 3 {
		t.Fatalf("expected 3 default filter modes, got %d", len(modes))
	}

	if modes[0].Name != FilterExact || modes[0].Label != "[exact]" {
		t.Errorf("expected first mode Name=%q Label='[exact]', got Name=%q Label=%q", FilterExact, modes[0].Name, modes[0].Label)
	}
	if modes[1].Name != FilterRegex || modes[1].Label != "[regex]" {
		t.Errorf("expected second mode Name=%q Label='[regex]', got Name=%q Label=%q", FilterRegex, modes[1].Name, modes[1].Label)
	}
	if modes[2].Name != FilterCaseInsensitive || modes[2].Label != "[iregex]" {
		t.Errorf("expected third mode Name=%q Label='[iregex]', got Name=%q Label=%q", FilterCaseInsensitive, modes[2].Name, modes[2].Label)
	}
}

// TestGetActiveFilterModeNil verifies GetActiveFilterMode returns nil when no mode is active.
func TestGetActiveFilterModeNil(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)

	if fv.GetActiveFilterMode() != nil {
		t.Errorf("expected nil active filter mode initially")
	}

	// Activate mode
	fv, _ = fv.Update(filterKeyMsg)
	if fv.GetActiveFilterMode() == nil {
		t.Errorf("expected non-nil active filter mode after activation")
	}

	// Cancel
	fv, _ = fv.Update(cancelFilterKeyMsg)
	if fv.GetActiveFilterMode() != nil {
		t.Errorf("expected nil active filter mode after cancel")
	}
}

// TestWithFilterModesCustom verifies WithFilterModes overrides defaults.
func TestWithFilterModesCustom(t *testing.T) {
	customMode := FilterMode{
		Name: "custom",
		Key: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "custom"),
		),
		Label: "[custom]",
		GetMatchFunc: func(filterText string) (MatchFunc, error) {
			return func(content string) []item.ByteRange {
				// Simple: match everything
				if len(content) > 0 && filterText != "" {
					return []item.ByteRange{{Start: 0, End: len(content)}}
				}
				return nil
			}, nil
		},
	}

	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithFilterModes[object]([]FilterMode{customMode}),
		},
	)

	modes := fv.FilterModes()
	if len(modes) != 1 {
		t.Fatalf("expected 1 custom filter mode, got %d", len(modes))
	}
	if modes[0].Label != "[custom]" {
		t.Errorf("expected label '[custom]', got %q", modes[0].Label)
	}

	// Default filter key '/' should not activate anything since we replaced modes
	fv.SetObjects(stringsToItems([]string{"hello"}))
	fv, _ = fv.Update(filterKeyMsg) // '/' — should not match any mode key
	if fv.GetActiveFilterMode() != nil {
		t.Errorf("expected no mode activation from '/', got %q", fv.GetActiveFilterMode().Name)
	}

	// Custom key 'x' should work
	fv, _ = fv.Update(internal.MakeKeyMsg('x'))
	if fv.GetActiveFilterMode().Name != "custom" {
		t.Errorf("expected mode 'custom' after 'x', got %q", fv.GetActiveFilterMode().Name)
	}
}

// TestAdjustObjectsForFilter_ModeNonEmptyOnClear verifies that the callback
// always receives a valid (non-empty) mode name, even when clearing the filter.
func TestAdjustObjectsForFilter_ModeNonEmptyOnClear(t *testing.T) {
	var receivedModes []FilterModeName
	fv := makeFilterableViewport(
		80,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithAdjustObjectsForFilter[object](func(_ string, mode FilterModeName) []object {
				receivedModes = append(receivedModes, mode)
				if mode == "" {
					t.Fatalf("adjustObjectsForFilter received empty mode name")
				}
				return nil
			}),
		},
	)
	fv.SetObjects(stringsToItems([]string{"apple", "banana"}))

	// Activate filter, type, apply
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// Clear filter — this sets activeFilterModeName to "" internally,
	// but the callback should still receive a valid (non-empty) mode name
	_, _ = fv.Update(cancelFilterKeyMsg)

	if len(receivedModes) == 0 {
		t.Fatal("expected adjustObjectsForFilter to be called at least once")
	}
	for i, mode := range receivedModes {
		if mode == "" {
			t.Errorf("call %d: received empty mode name", i)
		}
	}
}

func TestModeSwitchAfterCancel(t *testing.T) {
	fv := makeFilterableViewport(
		80,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
	}))

	// Activate exact mode, type, apply
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// "apple" has 1 'a', "banana" has 3 'a's = 4 total matches
	if fv.totalMatchesOnAllItems != 4 {
		t.Fatalf("expected 4 matches for 'a', got %d", fv.totalMatchesOnAllItems)
	}

	// Cancel filter
	fv, _ = fv.Update(cancelFilterKeyMsg)
	if fv.GetActiveFilterMode() != nil {
		t.Errorf("expected nil active filter mode after cancel, got %q", fv.GetActiveFilterMode().Name)
	}
	if fv.filterMode != filterModeOff {
		t.Errorf("expected filterModeOff after cancel")
	}

	// Switch to regex mode
	fv, _ = fv.Update(regexFilterKeyMsg)
	if fv.GetActiveFilterMode().Name != FilterRegex {
		t.Errorf("expected mode %q (regex), got %q", FilterRegex, fv.GetActiveFilterMode().Name)
	}
	// Filter text should be empty (was cleared on cancel)
	if fv.GetFilterText() != "" {
		t.Errorf("expected empty filter text after cancel+mode switch, got %q", fv.GetFilterText())
	}
}

func TestDuplicateFilterModeNamePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate FilterModeName, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "duplicate FilterModeName") {
			t.Errorf("expected panic message about duplicate FilterModeName, got: %s", msg)
		}
	}()

	vp := viewport.New[object](80, 6)
	New[object](vp,
		WithFilterModes[object]([]FilterMode{
			ExactFilterMode(key.NewBinding(key.WithKeys("/"))),
			ExactFilterMode(key.NewBinding(key.WithKeys("f"))), // same Name: "exact"
		}),
	)
}

func TestNoFilterModesPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for no filter modes, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "no filter modes set") {
			t.Errorf("expected panic message about no filter modes, got: %s", msg)
		}
	}()

	vp := viewport.New[object](80, 6)
	New[object](vp,
		WithFilterModes[object]([]FilterMode{}),
	)
}

func TestEmptyFilterModeNamePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for empty FilterMode Name, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "empty Name") {
			t.Errorf("expected panic message about empty Name, got: %s", msg)
		}
	}()

	vp := viewport.New[object](80, 6)
	New[object](vp,
		WithFilterModes[object]([]FilterMode{
			{Key: key.NewBinding(key.WithKeys("x")), Label: "[x]", GetMatchFunc: func(_ string) (MatchFunc, error) { return nil, nil }},
		}),
	)
}

func TestNoMatchesResetsXOffsetWhenUnwrapped(t *testing.T) {
	fv := makeFilterableViewport(
		10,
		3,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](false),
		},
		[]Option[object]{},
	)
	fv.SetObjects(stringsToItems([]string{
		strings.Repeat("a", 32),
	}))

	// filter for "a" and navigate to a right-side match so xOffset > 0
	fv, _ = fv.Update(filterKeyMsg)
	for range 4 {
		fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	}
	fv, _ = fv.Update(applyFilterKeyMsg)
	fv, _ = fv.Update(nextMatchKeyMsg)
	fv, _ = fv.Update(nextMatchKeyMsg)
	if fv.vp.GetXOffsetWidth() == 0 {
		t.Fatal("expected xOffset > 0 after navigating to right-side match")
	}

	// cancel filter and start a new one that produces no matches
	fv, _ = fv.Update(cancelFilterKeyMsg)
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('z'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	if fv.vp.GetXOffsetWidth() != 0 {
		t.Fatalf("expected xOffset=0 when no matches and unwrapped, got %d", fv.vp.GetXOffsetWidth())
	}
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"aaaaaaa...",
		"[exact]...",
		footerStyle.Render("100% (1/1)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}
