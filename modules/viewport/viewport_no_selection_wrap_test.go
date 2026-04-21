package viewport

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

func TestViewport_SelectionOff_WrapOn_Empty(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetWrapText(true)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{})
	internal.CmpStr(t, expectedView, vp.View())
	vp.SetHeader([]string{"header"})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"header"})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SmolDimensions(t *testing.T) {
	w, h := 0, 0
	vp := newViewport(w, h)
	vp.SetWrapText(true)
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
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"head", "er", "hi", "1..."})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_Basic(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_GetConfigs(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first",
		"second",
	})
	if selectionEnabled := vp.GetSelectionEnabled(); selectionEnabled {
		t.Errorf("expected selection to be disabled, got %v", selectionEnabled)
	}
	if wrapText := vp.GetWrapText(); !wrapText {
		t.Errorf("expected text wrapping to be enabled, got %v", wrapText)
	}
	if selectedItemIdx := vp.GetSelectedItemIdx(); selectedItemIdx != 0 {
		t.Errorf("expected selected item index to be 0, got %v", selectedItemIdx)
	}
	if selectedItem := vp.GetSelectedItem(); selectedItem != nil {
		t.Errorf("expected selected item to be nil, got %v", selectedItem)
	}
}

func TestViewport_SelectionOff_WrapOn_ShowFooter(t *testing.T) {
	w, h := 15, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		internal.RedFg.Render("a") + " really really",
		"99% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		internal.RedFg.Render("a") + " really really",
		" long line",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(9)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		internal.RedFg.Render("a") + " really really",
		" long line",
		"",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_FooterStyle(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h, WithStyles[object](Styles{
		FooterStyle:       internal.RedFg,
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"1",
		"2",
		"3",
		"4",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"1",
		"2",
		"3",
		internal.RedFg.Render("75% (3/4)"),
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_FooterDisabled(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first line",
		"second line",
		"third line",
		"fourth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		"second line",
		"third line",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetFooterEnabled(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		"second line",
		"third line",
		"fourth line",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SpaceAround(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"    first line     ",
		"          first line          ",
		"               first line               ",
	})
	// trailing space is not trimmed
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"    first line ",
		"",
		"          first",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_MultiHeader(t *testing.T) {
	w, h := 15, 2
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header1", "header2"})
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
		"line1",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		"line2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		"line2",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_OverflowLine(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"long header overflows"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"long header ove",
		"rflows",
		"123456789012345",
		"123456789012345",
		"6",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_OverflowHeight(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"123456789012345",
		"123456789012345",
		"6",
		"123456789012345",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_Scrolling(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
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
		"first",
		"second",
		"third",
		"fourth",
		"66% (4/6)",
	})
	validate(expectedView)

	// scrolling up past top is no-op
	vp, _ = vp.Update(upKeyMsg)
	validate(expectedView)

	// scrolling down by one
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"third",
		"fourth",
		"fifth",
		"83% (5/6)",
	})
	validate(expectedView)

	// scrolling down by one again
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	validate(expectedView)

	// scrolling down past bottom when at bottom is no-op
	vp, _ = vp.Update(downKeyMsg)
	validate(expectedView)
}

func TestViewport_SelectionOff_WrapOn_EnsureItemInView(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line that is super long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first ",
		"line",
		"the second",
		" line",
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(2, 0, 9, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"the second",
		" line",
		"the third",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp, _ = vp.Update(goToBottomKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the fourth",
		" line that",
		" is super ",
		"long",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(1, len("the second"), len("the second line"), 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		" line",
		"the third ",
		"line",
		"the fourth",
		"99% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(0, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first ",
		"line",
		"the second",
		" line",
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(3, 0, len("the fourth line that is super "), 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"the fourth",
		" line that",
		" is super ",
		"99% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_EnsureItemInViewVerticalPad(t *testing.T) {
	w, h := 10, 10
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	numItems := 100
	nums := make([]string, 0, numItems)
	for i := range numItems {
		nums = append(nums, strconv.Itoa(i+1))
	}
	setContent(vp, nums)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"1",
		"2",
		"3",
		"4",
		"5",
		"6",
		"7",
		"8",
		"8% (8/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "10" with verticalPad=1
	// should leave 1 line of context below
	vp.EnsureItemInView(9, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"4",
		"5",
		"6",
		"7",
		"8",
		"9",
		"10",
		"11",
		"11% (11...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to "5" with verticalPad=1
	// should leave 1 line of context above
	vp.EnsureItemInView(4, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"4",
		"5",
		"6",
		"7",
		"8",
		"9",
		"10",
		"11",
		"11% (11...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "15" with verticalPad=2
	// should leave 2 lines of context above
	vp.EnsureItemInView(99, 0, 0, 0, 0) // reset to bottom
	vp.EnsureItemInView(14, 0, 0, 2, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"13",
		"14",
		"15",
		"16",
		"17",
		"18",
		"19",
		"20",
		"20% (20...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "99", not enough content below for verticalPad=3
	// pad below as much as possible
	vp.EnsureItemInView(0, 0, 0, 0, 0) // reset to top
	vp.EnsureItemInView(98, 0, 0, 3, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"93",
		"94",
		"95",
		"96",
		"97",
		"98",
		"99",
		"100",
		"100% (1...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "50", request more padding than is available given viewport height -> center item
	vp.EnsureItemInView(0, 0, 0, 0, 0) // reset to top
	vp.EnsureItemInView(49, 0, 0, 5, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"47",
		"48",
		"49",
		"50",
		"51",
		"52",
		"53",
		"54",
		"54% (54...",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_EnsureItemInViewHorizontalPad(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"some line that is really long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"some line ",
		"that is re",
		"ally long",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: ensure "line " is visible with horizontalPad=2
	// in wrap mode, horizontal padding ensures character ranges are visible
	vp.EnsureItemInView(0, 0, 0, 0, 0) // reset
	vp.EnsureItemInView(0, len("some line"), len("some line "), 0, 2)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"some line ",
		"that is re",
		"ally long",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: ensure "really" is visible with horizontalPad=1
	vp.EnsureItemInView(0, len("some line that is "), len("some line that is really"), 0, 1)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"some line ",
		"that is re",
		"ally long",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: ensure end of string is visible with large horizontalPad
	vp.EnsureItemInView(0, len("some line that is really lon"), len("some line that is really long"), 0, 100)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"some line ",
		"that is re",
		"ally long",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SetXOffset(t *testing.T) {
	w, h := 10, 8
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first ",
		"line",
		"the second",
		" line",
		"",
		"",
		"100% (2/2)",
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

func TestViewport_SelectionOff_WrapOn_BulkScrolling(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first",
		"line",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		" line",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page down
	vp, _ = vp.Update(halfPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		" line",
		"the third ",
		"99% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the third ",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		" line",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page up
	vp, _ = vp.Update(halfPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"the second",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first",
		"line",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to bottom
	vp, _ = vp.Update(goToBottomKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the third ",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to top
	vp, _ = vp.Update(goToTopKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first",
		"line",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_Panning(t *testing.T) {
	w, h := 10, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header long"})
	vp.SetWrapText(true)
	doSetContent := func() {
		setContent(vp, []string{
			"first line that is fairly long",
			"second line that is even much longer than the first",
			"third line that is fairly long",
			"fourth",
			"fifth line that is fairly long",
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
		"first line",
		" that is f",
		"airly long",
		"second lin",
		"33% (2/6)",
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
		" that is f",
		"airly long",
		"second lin",
		"e that is ",
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
		"airly long",
		"second lin",
		"e that is",
		"even much",
		"33% (2/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header lon",
		"g",
		"second lin",
		"e that is ",
		"even much ",
		"longer tha",
		"33% (2/6)",
	})
	validate(expectedView)
}

func TestViewport_SelectionOff_WrapOn_ChangeHeight(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first",
		"line",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to bottom
	vp, _ = vp.Update(fullPgDownKeyMsg)
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the third",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the third",
		"99% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first",
		"line",
		"the second",
		" line",
		"the third",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_ChangeContent(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first",
		"line",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to bottom
	vp, _ = vp.Update(fullPgDownKeyMsg)
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the third",
		"line",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove Item
	setContent(vp, []string{
		"the first line",
		"the second line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		" line",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the second",
		" line",
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove all Item
	setContent(vp, []string{})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SuperLongWrappedLine(t *testing.T) {
	runTest := func(t *testing.T) {
		w, h := 10, 5
		vp := newViewport(w, h)
		vp.SetHeader([]string{"header"})
		vp.SetWrapText(true)
		setContent(vp, []string{
			"smol",
			strings.Repeat("12345678", 1000000),
			"smol",
		})
		expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			"smol",
			"1234567812",
			"3456781234",
			"66% (2/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())

		vp, _ = vp.Update(downKeyMsg)
		expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			"1234567812",
			"3456781234",
			"5678123456",
			"66% (2/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())

		vp, _ = vp.Update(downKeyMsg)
		expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			"3456781234",
			"5678123456",
			"7812345678",
			"66% (2/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())

		vp, _ = vp.Update(goToBottomKeyMsg)
		expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
			"header",
			"5678123456",
			"7812345678",
			"smol",
			"100% (3/3)",
		})
		internal.CmpStr(t, expectedView, vp.View())
	}
	internal.RunWithTimeout(t, runTest, 500*time.Millisecond)
}

func TestViewport_SelectionOff_WrapOn_EnableSelectionShowsTopLineInItem(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetWrapText(true)
	setContent(vp, []string{
		"short",
		"this is a very long line",
		"another short line",
		"final line",
	})
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"very long ",
		"line",
		"another sh",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
	vp.SetSelectionEnabled(true)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		internal.BlueFg.Render("this is a "),
		internal.BlueFg.Render("very long "),
		internal.BlueFg.Render("line"),
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SetHighlights(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		"first",
		"second line that wraps",
		"third",
	})
	highlights := []Highlight{
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 0,
					End:   6,
				},
				Style: internal.RedFg,
			},
		},
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 12,
					End:   16,
				},
				Style: internal.GreenFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		internal.RedFg.Render("second") + " lin",
		"e " + internal.GreenFg.Render("that") + " wra",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SetHighlightsStyledContent(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	setContent(vp, []string{
		internal.GreenFg.Render("first"),
		internal.BlueFg.Render("second line that wraps"),
		internal.RedFg.Render("third"),
	})
	highlights := []Highlight{
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 0,
					End:   6,
				},
				Style: internal.RedFg,
			},
		},
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 12,
					End:   16,
				},
				Style: internal.GreenFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.GreenFg.Render("first"),
		internal.RedFg.Render("second") + internal.BlueFg.Render(" lin"),
		internal.BlueFg.Render("e ") + internal.GreenFg.Render("that") + internal.BlueFg.Render(" wra"),
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOn_SetHighlightsAnsiUnicode(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"A💖中é"})
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
		"A" + internal.RedFg.Render("💖中") + "é tex",
		"t that wra",
		"ps",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}
