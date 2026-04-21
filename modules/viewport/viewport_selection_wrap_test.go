package viewport

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

func TestViewport_SelectionOn_WrapOn_Empty(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{})
	internal.CmpStr(t, expectedView, vp.View())
	vp.SetHeader([]string{"header"})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"header"})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SmolDimensions(t *testing.T) {
	w, h := 0, 0
	vp := newViewport(w, h)
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{"hi"})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{""})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(1)
	vp.SetHeight(1)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"h"})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(2)
	vp.SetHeight(2)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"he", "ad"})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(3)
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"hea", "der", ""})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(4)
	vp.SetHeight(4)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"head",
		"er",
		internal.BlueFg.Render("hi"),
		"1...",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_Basic(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_GetConfigs(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first",
		"second",
	})
	if selectionEnabled := vp.GetSelectionEnabled(); !selectionEnabled {
		t.Errorf("expected selection to be enabled, got %v", selectionEnabled)
	}
	if wrapText := vp.GetWrapText(); !wrapText {
		t.Errorf("expected text wrapping to be enabled, got %v", wrapText)
	}
	if selectedItemIdx := vp.GetSelectedItemIdx(); selectedItemIdx != 0 {
		t.Errorf("expected selected item index to be 0, got %v", selectedItemIdx)
	}
	vp, _ = vp.Update(downKeyMsg)
	if selectedItemIdx := vp.GetSelectedItemIdx(); selectedItemIdx != 1 {
		t.Errorf("expected selected item index to be 1, got %v", selectedItemIdx)
	}
	if selectedItem := vp.GetSelectedItem(); selectedItem != nil && selectedItem.GetItem().Content() != "second" {
		t.Errorf("got unexpected selected item: %v", selectedItem)
	}
}

func TestViewport_SelectionOn_WrapOn_ShowFooter(t *testing.T) {
	w, h := 15, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		internal.RedFg.Render("a") + " really really",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		internal.RedFg.Render("a") + " really really",
		" long line",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(9)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		internal.RedFg.Render("a") + " really really",
		" long line",
		"",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_FooterStyle(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h, WithStyles[object](Styles{
		FooterStyle:       internal.RedFg,
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"1",
		"2",
		"3",
		"4",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("1"),
		"2",
		"3",
		internal.RedFg.Render("25% (1/4)"),
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_FooterDisabled(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first line",
		"second line",
		"third line",
		"fourth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		"second line",
		"third line",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetFooterEnabled(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		"second line",
		"third line",
		"fourth line",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SpaceAround(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"    first line     ",
		"          first line          ",
		"               first line               ",
	})
	// trailing space is not trimmed
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("    first line "),
		internal.BlueFg.Render("    "),
		"          first",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_MultiHeader(t *testing.T) {
	w, h := 15, 2
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header1", "header2"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"line1",
		"line2",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(4)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		internal.BlueFg.Render("line1"),
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		internal.BlueFg.Render("line2"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		internal.BlueFg.Render("line2"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		internal.BlueFg.Render("line2"),
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_OverflowLine(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"long header overflows"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"long header ove",
		"rflows",
		internal.BlueFg.Render("123456789012345"),
		"123456789012345",
		"6",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_OverflowHeight(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
	})
	vp.SetSelectedItemIdx(1)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"123456789012345",
		internal.BlueFg.Render("123456789012345"),
		internal.BlueFg.Render("6"),
		"123456789012345",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_Scrolling(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	doSetContent := func() {
		setContent(vp, []string{
			"first",
			"second",
			"third",
			"fourth",
			"fifth",
			"sixth",
		})
	}
	validate := func(expectedView string) {
		// set Item multiple times to confirm no side effects of doing it
		internal.CmpStr(t, expectedView, vp.View())
		doSetContent()
		internal.CmpStr(t, expectedView, vp.View())
	}
	doSetContent()
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"second",
		"third",
		"fourth",
		"16% (1/6)",
	})
	validate(expectedView)

	// scrolling up past top is no-op
	vp, _ = vp.Update(upKeyMsg)
	validate(expectedView)

	// scrolling down by one
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		internal.BlueFg.Render("second"),
		"third",
		"fourth",
		"33% (2/6)",
	})
	validate(expectedView)

	// scrolling down by one again
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		internal.BlueFg.Render("third"),
		"fourth",
		"50% (3/6)",
	})
	validate(expectedView)

	// scroll to bottom
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"fifth",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	validate(expectedView)

	// scrolling down past bottom when at bottom is no-op
	vp, _ = vp.Update(downKeyMsg)
	validate(expectedView)
}

func TestViewport_SelectionOn_WrapOn_EnsureItemInView(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line that is super long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"the second",
		" line",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(2, 0, 9, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("line"),
		"the second",
		" line",
		"the third",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp, _ = vp.Update(goToBottomKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the fourth"),
		internal.BlueFg.Render(" line that"),
		internal.BlueFg.Render(" is super "),
		internal.BlueFg.Render("long"),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(1, len("the second"), len("the second line"), 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		" line",
		"the third ",
		"line",
		internal.BlueFg.Render("the fourth"),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(0, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first ",
		"line",
		"the second",
		" line",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(3, 0, len("the fourth line that is super "), 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		internal.BlueFg.Render("the fourth"),
		internal.BlueFg.Render(" line that"),
		internal.BlueFg.Render(" is super "),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_EnsureItemInViewVerticalPad(t *testing.T) {
	w, h := 10, 10
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	numItems := 100
	nums := make([]string, 0, numItems)
	for i := range numItems {
		nums = append(nums, strconv.Itoa(i+1))
	}
	setContent(vp, nums)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("1"),
		"2",
		"3",
		"4",
		"5",
		"6",
		"7",
		"8",
		"1% (1/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "10" with verticalPad=1
	// should leave 1 line of context below
	vp.SetSelectedItemIdx(9)
	vp.EnsureItemInView(9, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"4",
		"5",
		"6",
		"7",
		"8",
		"9",
		selectionStyle.Render("10"),
		"11",
		"10% (10...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to "5" with verticalPad=1
	// should leave 1 line of context above
	vp.SetSelectedItemIdx(4)
	vp.EnsureItemInView(4, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"4",
		selectionStyle.Render("5"),
		"6",
		"7",
		"8",
		"9",
		"10",
		"11",
		"5% (5/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "15" with verticalPad=2
	// should leave 2 lines of context above
	vp.SetSelectedItemIdx(99) // reset to bottom
	vp.EnsureItemInView(99, 0, 0, 0, 0)
	vp.SetSelectedItemIdx(14)
	vp.EnsureItemInView(14, 0, 0, 2, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"13",
		"14",
		selectionStyle.Render("15"),
		"16",
		"17",
		"18",
		"19",
		"20",
		"15% (15...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "99", not enough content below for verticalPad=3
	// pad below as much as possible
	vp.SetSelectedItemIdx(0) // reset to top
	vp.EnsureItemInView(0, 0, 0, 0, 0)
	vp.SetSelectedItemIdx(98)
	vp.EnsureItemInView(98, 0, 0, 3, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"93",
		"94",
		"95",
		"96",
		"97",
		"98",
		selectionStyle.Render("99"),
		"100",
		"99% (99...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "50", request more padding than is available given viewport height -> center item
	vp.SetSelectedItemIdx(0) // reset to top
	vp.EnsureItemInView(0, 0, 0, 0, 0)
	vp.SetSelectedItemIdx(49)
	vp.EnsureItemInView(49, 0, 0, 5, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"47",
		"48",
		"49",
		selectionStyle.Render("50"),
		"51",
		"52",
		"53",
		"54",
		"50% (50...",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

// TestViewport_SelectionOn_WrapOn_EnsureItemInViewNoOscillation verifies that repeated calls
// to EnsureItemInView produce stable positioning. Before the fix, when padding couldn't be
// satisfied on both sides, the view would oscillate on each call because scrollingDown
// would change based on the current position. This simulates what happens during cursor
// blinks in the filterable viewport, where SetObjects and EnsureItemInView are called
// repeatedly on the same visible item.
//
// The oscillation occurs specifically when navigating FROM BELOW to an item:
// 1. First call: scrollingDown=false (coming from below), positions with padding above
// 2. After positioning, top is now ABOVE target, so scrollingDown becomes true
// 3. Second call: scrollingDown=true, positions with padding below (different position!)
// 4. This creates oscillation between the two positions
func TestViewport_SelectionOn_WrapOn_EnsureItemInViewNoOscillation(t *testing.T) {
	w, h := 10, 10
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	// create 100 items
	numItems := 100
	nums := make([]string, 0, numItems)
	for i := range numItems {
		nums = append(nums, strconv.Itoa(i+1))
	}
	setContent(vp, nums)

	// first go to the bottom, then navigate UP to item 50
	// this is the scenario that triggers oscillation: coming from below
	vp.SetSelectedItemIdx(99) // go to bottom (item 100)
	vp.EnsureItemInView(99, 0, 0, 0, 0)

	// now navigate up to item 50 with padding=5 (can't fit on both sides)
	vp.SetSelectedItemIdx(49)
	vp.EnsureItemInView(49, 0, 0, 5, 0)
	viewAfterFirstCall := vp.View()

	// item 50 should be approximately centered
	// when coming from below, scroll-up centering positions with padding above
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"46",
		"47",
		"48",
		"49",
		selectionStyle.Render("50"),
		"51",
		"52",
		"53",
		"50% (50...",
	})
	internal.CmpStr(t, expectedView, viewAfterFirstCall)

	// simulate cursor blink: call EnsureItemInView again without any navigation
	// before the fix, this would cause oscillation because:
	// - after first call, top is at item 47 (above target item 50)
	// - targetBelowTop(49, 0) now returns true (scrollingDown=true)
	// - this triggers different positioning logic, causing the view to shift
	for i := range 5 {
		vp.EnsureItemInView(49, 0, 0, 5, 0)
		viewAfterRepeat := vp.View()

		// view should remain stable - no oscillation
		if viewAfterRepeat != viewAfterFirstCall {
			t.Fatalf("View oscillated on iteration %d.\nExpected:\n%s\n\nGot:\n%s", i+1, viewAfterFirstCall, viewAfterRepeat)
		}
	}
}

func TestViewport_SelectionOn_WrapOn_EnsureItemInViewHorizontalPad(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"some line that is really long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("some line "),
		selectionStyle.Render("that is re"),
		selectionStyle.Render("ally long"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: ensure "line " is visible with horizontalPad=2
	// in wrap mode, horizontal padding ensures character ranges are visible
	vp.SetSelectedItemIdx(0) // reset
	vp.EnsureItemInView(0, 0, 0, 0, 0)
	vp.EnsureItemInView(0, len("some line"), len("some line "), 0, 2)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("some line "),
		selectionStyle.Render("that is re"),
		selectionStyle.Render("ally long"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: ensure "really" is visible with horizontalPad=1
	vp.EnsureItemInView(0, len("some line that is "), len("some line that is really"), 0, 1)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("some line "),
		selectionStyle.Render("that is re"),
		selectionStyle.Render("ally long"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: ensure end of string is visible with large horizontalPad
	vp.EnsureItemInView(0, len("some line that is really lon"), len("some line that is really long"), 0, 100)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("some line "),
		selectionStyle.Render("that is re"),
		selectionStyle.Render("ally long"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SetXOffset(t *testing.T) {
	w, h := 10, 8
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
	})

	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"the second",
		" line",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetXOffset(-1)
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetXOffset(0)
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetXOffset(4)
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetXOffset(1000)
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_BulkScrolling(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the second"),
		internal.BlueFg.Render(" line"),
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page down
	vp, _ = vp.Update(halfPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page down
	vp, _ = vp.Update(halfPgDownKeyMsg)
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the second"),
		internal.BlueFg.Render(" line"),
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page up
	vp, _ = vp.Update(halfPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page up
	vp, _ = vp.Update(halfPgUpKeyMsg)
	internal.CmpStr(t, expectedView, vp.View())

	// go to bottom
	vp, _ = vp.Update(goToBottomKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to top
	vp, _ = vp.Update(goToTopKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_Panning(t *testing.T) {
	w, h := 10, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header long"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	doSetContent := func() {
		setContent(vp, []string{
			"first line that is fairly long",
			"second line that is even much longer than the first",
			"third line that is fairly long as well",
			"fourth kinda long",
			"fifth kinda long too",
			"sixth",
		})
	}
	validate := func(expectedView string) {
		// set Item multiple times to confirm no side effects of doing it
		internal.CmpStr(t, expectedView, vp.View())
		doSetContent()
		internal.CmpStr(t, expectedView, vp.View())
	}
	doSetContent()
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("first line"),
		internal.BlueFg.Render(" that is f"),
		internal.BlueFg.Render("airly long"),
		"second lin",
		"16% (1/6)",
	})
	validate(expectedView)

	// pan right
	vp.SetXOffset(5)
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("second lin"),
		internal.BlueFg.Render("e that is "),
		internal.BlueFg.Render("even much "),
		internal.BlueFg.Render("longer tha"),
		"33% (2/6)",
	})
	validate(expectedView)

	// pan all the way right
	vp.SetXOffset(41)
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("third line"),
		internal.BlueFg.Render(" that is f"),
		internal.BlueFg.Render("airly long"),
		internal.BlueFg.Render(" as well"),
		"50% (3/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		"airly long",
		" as well",
		internal.BlueFg.Render("fourth kin"),
		internal.BlueFg.Render("da long"),
		"66% (4/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		"fourth kin",
		"da long",
		internal.BlueFg.Render("fifth kind"),
		internal.BlueFg.Render("a long too"),
		"83% (5/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		"da long",
		"fifth kind",
		"a long too",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		"da long",
		internal.BlueFg.Render("fifth kind"),
		internal.BlueFg.Render("a long too"),
		"sixth",
		"83% (5/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("fourth kin"),
		internal.BlueFg.Render("da long"),
		"fifth kind",
		"a long too",
		"66% (4/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("third line"),
		internal.BlueFg.Render(" that is f"),
		internal.BlueFg.Render("airly long"),
		internal.BlueFg.Render(" as well"),
		"50% (3/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("second lin"),
		internal.BlueFg.Render("e that is "),
		internal.BlueFg.Render("even much "),
		internal.BlueFg.Render("longer tha"),
		"33% (2/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		internal.BlueFg.Render("first line"),
		internal.BlueFg.Render(" that is f"),
		internal.BlueFg.Render("airly long"),
		"second lin",
		"16% (1/6)",
	})
	validate(expectedView)
}

func TestViewport_SelectionOn_WrapOn_MaintainSelection(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	vp.SetSelectionComparator(objectsEqual)
	setContent(vp, []string{
		"sixth item",
		"seventh item",
		"eighth item",
		"ninth item",
		"tenth item",
		"eleventh item",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("sixth item"),
		"seventh it",
		"em",
		"eighth ite",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"sixth item",
		internal.BlueFg.Render("seventh it"),
		internal.BlueFg.Render("em"),
		"eighth ite",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item above
	setContent(vp, []string{
		"first item",
		"second item",
		"third item",
		"fourth item",
		"fifth item",
		"sixth item",
		"seventh item",
		"eighth item",
		"ninth item",
		"tenth item",
		"eleventh item",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"sixth item",
		internal.BlueFg.Render("seventh it"),
		internal.BlueFg.Render("em"),
		"eighth ite",
		"63% (7/11)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item below
	setContent(vp, []string{
		"first item",
		"second item",
		"third item",
		"fourth item",
		"fifth item",
		"sixth item",
		"seventh item",
		"eighth item",
		"ninth item",
		"tenth item",
		"eleventh item",
		"twelfth item",
		"thirteenth item",
		"fourteenth item",
		"fifteenth item",
		"sixteenth item",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"sixth item",
		internal.BlueFg.Render("seventh it"),
		internal.BlueFg.Render("em"),
		"eighth ite",
		"43% (7/16)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_StickyTop(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetTopSticky(true)
	setContent(vp, []string{
		"the first line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the second line",
		"the first line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the second"),
		internal.BlueFg.Render(" line"),
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// de-activate by moving selection down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the second line",
		"the first line",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_StickyBottom(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"the first line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the second line",
		"the first line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		" line",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add longer Item at bottom
	setContent(vp, []string{
		"the second line",
		"the first line",
		"a very long line that wraps a lot",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("a very lon"),
		internal.BlueFg.Render("g line tha"),
		internal.BlueFg.Render("t wraps a "),
		internal.BlueFg.Render("lot"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// de-activate by moving selection up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"a very lon",
		"g line tha",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the second line",
		"the first line",
		"a very long line that wraps a lot",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"a very lon",
		"g line tha",
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_StickyBottomOverflowHeight(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetBottomSticky(true)

	// test covers case where first set Item to empty, then overflow height
	setContent(vp, []string{})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	setContent(vp, []string{
		"the second line",
		"the first line",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_StickyTopBottom(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetTopSticky(true)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"the first line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item, top sticky wins out arbitrarily when both set
	setContent(vp, []string{
		"the second line",
		"the first line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the second"),
		internal.BlueFg.Render(" line"),
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection to bottom
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the second line",
		"the first line",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// de-activate by moving selection up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the second line",
		"the first line",
		"the third line",
		"the fourth line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_StickyBottomLongLine(t *testing.T) {
	w, h := 10, 10
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"first line",
		"next line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.BlueFg.Render("next line"),
		"",
		"",
		"",
		"",
		"",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	setContent(vp, []string{
		"first line",
		"next line",
		"a very long line at the bottom that wraps many times",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		"next line",
		internal.BlueFg.Render("a very lon"),
		internal.BlueFg.Render("g line at "),
		internal.BlueFg.Render("the bottom"),
		internal.BlueFg.Render(" that wrap"),
		internal.BlueFg.Render("s many tim"),
		internal.BlueFg.Render("es"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_StickyTop(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(false)
	vp.SetTopSticky(true)
	setContent(vp, []string{
		"the first line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first ",
		"99% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item
	setContent(vp, []string{
		"the second line",
		"the first line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to de-activate sticky
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		" line",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item - should not return to top
	setContent(vp, []string{
		"the third line",
		"the second line",
		"the first line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_StickyBottom(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(false)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"the first line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first ",
		"line",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item
	setContent(vp, []string{
		"the first line",
		"the second line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"the second",
		" line",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add longer item at bottom
	setContent(vp, []string{
		"the first line",
		"the second line",
		"a very long line that wraps a lot",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"g line tha",
		"t wraps a ",
		"lot",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to de-activate sticky
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"a very lon",
		"g line tha",
		"t wraps a ",
		"99% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item - should not jump to bottom
	setContent(vp, []string{
		"the first line",
		"the second line",
		"a very long line that wraps a lot",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"a very lon",
		"g line tha",
		"t wraps a ",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_StickyBottomOverflowHeight(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(false)
	vp.SetBottomSticky(true)

	// test covers case where first set item to empty, then overflow height
	setContent(vp, []string{})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	setContent(vp, []string{
		"the second line",
		"the first line",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_StickyBottomLongLine(t *testing.T) {
	w, h := 10, 9
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(false)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"first line",
		"next line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		"next line",
		"",
		"",
		"",
		"",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	setContent(vp, []string{
		"first line",
		"next line",
		"a very long line at the bottom that wraps many times",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"next line",
		"a very lon",
		"g line at ",
		"the bottom",
		" that wrap",
		"s many tim",
		"es",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_RemoveLogsWhenSelectionBottom(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	setContent(vp, []string{
		"the second line",
		"the first line",
		"the third line",
		"the fourth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the second"),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection to bottom
	vp.SetSelectedItemIdx(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the fourth"),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove bottom items
	setContent(vp, []string{
		"the second line",
		"the first line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_ChangeHeight(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
		"the fifth line",
		"the sixth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"the second",
		" line",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to third line
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		" line",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the third "),
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"the fourth",
		" line",
		"the fifth ",
		"line",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to last line
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the fourth",
		" line",
		"the fifth ",
		"line",
		internal.BlueFg.Render("the sixth "),
		internal.BlueFg.Render("line"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the sixth "),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_ChangeContent(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
		"the fifth line",
		"the sixth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"the second",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to bottom
	vp.SetSelectedItemIdx(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		internal.BlueFg.Render("the sixth "),
		internal.BlueFg.Render("line"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove Item
	setContent(vp, []string{
		"the second line",
		"the third line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		" line",
		internal.BlueFg.Render("the third "),
		internal.BlueFg.Render("line"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove all Item
	setContent(vp, []string{})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
		"the fifth line",
		"the sixth line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the first "),
		internal.BlueFg.Render("line"),
		"the second",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_AnsiOnSelection(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"line with some " + internal.RedFg.Render("red") + " text",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("line with "),
		selectionStyle.Render("some red t"),
		internal.BlueFg.Render("ext"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_AnsiOnSelection_NoOverride(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h, WithSelectionStyleOverridesItemStyle[object](false))
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"line with some " + internal.RedFg.Render("red") + " text",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("line with "),
		selectionStyle.Render("some ") + internal.RedFg.Render("red") + selectionStyle.Render(" t"), // item style preserved
		internal.BlueFg.Render("ext"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SelectionEmpty(t *testing.T) {
	w, h := 20, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render(" "),
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_ExtraSlash(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"|2024|" + internal.RedFg.Render("fl..lq") + "/" + internal.RedFg.Render("flask-3") + "|",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("|2024|fl.."),
		selectionStyle.Render("lq/flask-3"),
		selectionStyle.Render("|"),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SuperLongWrappedLine(t *testing.T) {
	runTest := func(t *testing.T) {
		w, h := 10, 5
		vp := newViewport(w, h)
		vp.SetHeader([]string{"header"})
		vp.SetSelectionEnabled(true)
		vp.SetWrapText(true)
		setContent(vp, []string{
			"smol",
			strings.Repeat("12345678", 1000000),
			"smol",
		})
		expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			internal.BlueFg.Render("smol"),
			"1234567812",
			"3456781234",
			"33% (1/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())

		vp, _ = vp.Update(downKeyMsg)
		expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			internal.BlueFg.Render("1234567812"),
			internal.BlueFg.Render("3456781234"),
			internal.BlueFg.Render("5678123456"),
			"66% (2/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())

		vp, _ = vp.Update(downKeyMsg)
		expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			"5678123456",
			"7812345678",
			internal.BlueFg.Render("smol"),
			"100% (3/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())
	}
	internal.RunWithTimeout(t, runTest, 500*time.Millisecond)
}

func TestViewport_SelectionOn_WrapOn_SetHighlights(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first line that wraps",
		"second",
		"third",
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 0,
					End:   5,
				},
				Style: internal.GreenFg,
			},
		},
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 11,
					End:   15,
				},
				Style: internal.RedFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.GreenFg.Render("first") + internal.BlueFg.Render(" line"),
		internal.BlueFg.Render(" ") + internal.RedFg.Render("that") + internal.BlueFg.Render(" wrap"),
		internal.BlueFg.Render("s"),
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SetHighlightsStyledContent(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		internal.BlueFg.Render("first line that wraps"),
		internal.GreenFg.Render("second"),
		internal.RedFg.Render("third"),
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 0,
					End:   5,
				},
				Style: internal.GreenFg,
			},
		},
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 11,
					End:   15,
				},
				Style: internal.RedFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.GreenFg.Render("first") + internal.BlueFg.Render(" line"),
		internal.BlueFg.Render(" ") + internal.RedFg.Render("that") + internal.BlueFg.Render(" wrap"),
		internal.BlueFg.Render("s"),
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SetHighlightsAnsiUnicode(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"A💖中é"})
	vp.SetSelectionEnabled(true)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"A💖中é text that wraps",
		"another line",
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 1,
					End:   8,
				},
				Style: internal.RedFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"A💖中é",
		internal.BlueFg.Render("A") + internal.RedFg.Render("💖中") + internal.BlueFg.Render("é tex"),
		internal.BlueFg.Render("t that wra"),
		internal.BlueFg.Render("ps"),
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

// # OTHER

func TestViewport_StyleOverlay(t *testing.T) {
	w, h := 20, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"plain text",
		internal.RedFg.Render("red text"),
		"more plain",
	})

	// add highlight to the second item which already has red styling
	highlights := []Highlight{
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 0,
					End:   3,
				},
				Style: internal.GreenFg,
			},
		},
	}
	vp.SetHighlights(highlights)

	// first item is selected, highlight should show on second item
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("plain text"),
		internal.GreenFg.Render("red") + internal.RedFg.Render(" text"),
		"more plain",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection style on second item overrides the red content styling; highlight keeps its style
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"plain text",
		internal.GreenFg.Render("red") + selectionStyle.Render(" text"),
		"more plain",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to third item, highlight should show again on second item
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"plain text",
		internal.GreenFg.Render("red") + internal.RedFg.Render(" text"),
		internal.BlueFg.Render("more plain"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_ToggleWrap_PreserveSelection(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first line that is fairly long",
		"second line that is even much longer than the first",
		"third line that is fairly long",
		"fourth",
		"fifth line that is fairly long",
		"sixth",
	})

	// wrap off, selection on first line
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line t..."),
		"second line ...",
		"third line t...",
		"fourth",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to third line
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line t...",
		"second line ...",
		internal.BlueFg.Render("third line t..."),
		"fourth",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap on
	vp.SetWrapText(true)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"longer than the",
		" first",
		internal.BlueFg.Render("third line that"),
		internal.BlueFg.Render(" is fairly long"),
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap off
	vp.SetWrapText(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line t...",
		"second line ...",
		internal.BlueFg.Render("third line t..."),
		"fourth",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to last line
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third line t...",
		"fourth",
		"fifth line t...",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap on
	vp.SetWrapText(true)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fourth",
		"fifth line that",
		" is fairly long",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap off
	vp.SetWrapText(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third line t...",
		"fourth",
		"fifth line t...",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_ToggleWrap_PreserveSelectionInView(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"a really really really really really really really really really really really really long preamble",
		"first line that is fairly long",
		"second line that is even much longer than the first",
		"third line that is fairly long",
	})
	vp.SetSelectedItemIdx(3)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"a really rea...",
		"first line t...",
		"second line ...",
		internal.BlueFg.Render("third line t..."),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap, full wrapped selection should remain in view
	vp.SetWrapText(true)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"longer than the",
		" first",
		internal.BlueFg.Render("third line that"),
		internal.BlueFg.Render(" is fairly long"),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap
	vp.SetWrapText(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"a really rea...",
		"first line t...",
		"second line ...",
		internal.BlueFg.Render("third line t..."),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_ToggleWrap_ScrollInBounds(t *testing.T) {
	w, h := 10, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
		"the fifth line",
		"the sixth line",
	})

	// scroll to bottom with selection at top of that view
	vp.SetSelectedItemIdx(5)
	vp, _ = vp.Update(upKeyMsg)
	vp, _ = vp.Update(upKeyMsg)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the fourth"),
		internal.BlueFg.Render(" line"),
		"the fifth ",
		"line",
		"the sixth ",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// toggle wrap
	vp.SetWrapText(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the sec...",
		"the thi...",
		internal.BlueFg.Render("the fou..."),
		"the fif...",
		"the six...",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_WrapOn_Basic(t *testing.T) {
	// width=20, prefix="> " (2 chars), so content wraps at 18
	w, h := 20, 7
	prefix := "> "

	vp := newViewport(w, h, WithStyles[object](Styles{
		SelectionPrefix:   prefix,
		FooterStyle:       lipgloss.NewStyle(),
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{"short", "medium length", "third"})

	// selection on first item
	expectedView := internal.Pad(w, h, []string{
		prefix + selectionStyle.Render("short"),
		"  " + "medium length",
		"  " + "third",
		"",
		"",
		"",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(w, h, []string{
		"  " + "short",
		prefix + selectionStyle.Render("medium length"),
		"  " + "third",
		"",
		"",
		"",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_WrapOn_LongItemWraps(t *testing.T) {
	// width=12, prefix="> " (2 chars), so content wraps at 10
	w, h := 12, 7
	prefix := "> "

	vp := newViewport(w, h, WithStyles[object](Styles{
		SelectionPrefix:   prefix,
		FooterStyle:       lipgloss.NewStyle(),
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	// "hello world!!" is 13 chars, wraps at content width 10 into 2 lines
	setContent(vp, []string{"hello world!!", "short"})

	// selected item wraps: prefix on both wrapped lines
	expectedView := internal.Pad(w, h, []string{
		prefix + selectionStyle.Render("hello worl"),
		prefix + selectionStyle.Render("d!!"),
		"  " + "short",
		"",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move down - unselected item still wraps, gets padding on both lines
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(w, h, []string{
		"  " + "hello worl",
		"  " + "d!!",
		prefix + selectionStyle.Render("short"),
		"",
		"",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_WrapOn_WithHeader(t *testing.T) {
	// header uses full width (no prefix), content uses contentWidth
	w, h := 20, 6
	prefix := "> "

	vp := newViewport(w, h, WithStyles[object](Styles{
		SelectionPrefix:   prefix,
		FooterStyle:       lipgloss.NewStyle(),
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetHeader([]string{"header line"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{"alpha", "beta"})

	expectedView := internal.Pad(w, h, []string{
		"header line",
		prefix + selectionStyle.Render("alpha"),
		"  " + "beta",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_WrapOn_NoColor(t *testing.T) {
	// all styles empty, only prefix distinguishes selection
	w, h := 16, 6
	prefix := "> "
	emptyStyle := lipgloss.NewStyle()

	vp := newViewport(w, h, WithStyles[object](Styles{
		SelectionPrefix:   prefix,
		FooterStyle:       emptyStyle,
		SelectedItemStyle: emptyStyle,
	}))
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	// "a]long item here" is 16 chars, wraps at content width 14 into 2 lines
	setContent(vp, []string{"a long item here!", "other"})

	expectedView := internal.Pad(w, h, []string{
		prefix + "a long item he",
		prefix + "re!",
		"  " + "other",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_SetSameDimensionsPreservesScrollPosition(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
		"the fifth line",
		"the sixth line",
		"the seventh line",
		"the eighth line",
	})

	// move selection to fifth item, causing a scroll
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView := internal.Pad(w, h, []string{
		"header",
		" line",
		internal.BlueFg.Render("the fifth "),
		internal.BlueFg.Render("line"),
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// setting the same width and height should not change the scroll position
	vp.SetWidth(w)
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(h)
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOn_ChangeHeightPreservesSelectionPosition(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
		"the fifth line",
		"the sixth line",
		"the seventh line",
		"the eighth line",
	})

	// move selection to fifth item
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView := internal.Pad(w, h, []string{
		"header",
		"the fourth",
		" line",
		internal.BlueFg.Render("the fifth "),
		internal.BlueFg.Render("line"),
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height - selection should remain visible and not jump to the top
	vp.SetHeight(10)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the fourth",
		" line",
		internal.BlueFg.Render("the fifth "),
		internal.BlueFg.Render("line"),
		"the sixth ",
		"line",
		"the sevent",
		"h line",
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height - selection should still be visible
	vp.SetHeight(4)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the fifth "),
		internal.BlueFg.Render("line"),
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func setContent(vp *Model[object], content []string) {
	renderableStrings := make([]object, len(content))
	for i := range content {
		renderableStrings[i] = object{item: item.NewItem(content[i])}
	}
	vp.SetObjects(renderableStrings)
}
